package openapithrift

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/Gk0Wk/openapi-thrift/rawjson"
)

func rawFixture(t *testing.T) *Value {
	t.Helper()
	body, err := os.ReadFile("tests/fixtures/raw-json-envelope.openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := Parse(body)
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func TestRawJSONFixtureUsesOneStrictContractAcrossNativeAndBridge(t *testing.T) {
	doc := rawFixture(t)
	before, _ := json.Marshal(doc)
	contract, err := CompileRawJSONContract(doc)
	if err != nil {
		t.Fatal(err)
	}
	if len(contract.Schemas) != 1 || len(contract.Operations) != 1 {
		t.Fatalf("unexpected contract: %+v", contract)
	}
	after, _ := json.Marshal(doc)
	if string(before) != string(after) {
		t.Fatal("compiler mutated input")
	}
	wire, err := json.Marshal(contract)
	if err != nil {
		t.Fatal(err)
	}
	bridge, _ := json.Marshal(BridgeRequest{Action: "compileRawJsonContract", Input: before})
	var response BridgeResponse
	if err := json.Unmarshal(DispatchJSON(bridge), &response); err != nil {
		t.Fatal(err)
	}
	if response.Error != nil || string(response.Result) != string(wire) {
		t.Fatalf("bridge differs: %s", DispatchJSON(bridge))
	}
	result, err := ValidateDocument(doc, ValidationOptions{})
	if err != nil || result.ErrorCount != 0 {
		t.Fatalf("strict profile rejected: %v %+v", err, result.Issues)
	}
	runtime, err := rawjson.LoadContract(wire)
	if err != nil {
		t.Fatal(err)
	}
	// Required presence and nullability stay separate, even for an empty string.
	desc := []byte(`{"type":"object","properties":{"value":{"type":"string","nullable":true}},"required":["value"],"additionalProperties":false}`)
	for _, value := range []string{`{"value":null}`, `{"value":""}`} {
		if err := runtime.ValidateSchema(desc, []byte(value)); err != nil {
			t.Fatalf("valid %s: %v", value, err)
		}
	}
	for _, value := range []string{`{}`, `{"value":0}`, `{"value":"","extra":1}`, `{"value":"a","value":"b"}`} {
		if err := runtime.ValidateSchema(desc, []byte(value)); err == nil {
			t.Fatalf("invalid accepted: %s", value)
		}
	}
	intDesc := []byte(`{"type":"integer","format":"int64"}`)
	if err := runtime.ValidateSchema(intDesc, []byte(`9223372036854775807`)); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{`9223372036854775808`, `1.0`, `1e0`} {
		if runtime.ValidateSchema(intDesc, []byte(value)) == nil {
			t.Fatalf("lossy integer accepted: %s", value)
		}
	}
}

func TestRawJSONRejectsUnsupportedSemanticsAndMarkerPlacement(t *testing.T) {
	mutations := map[string]func(*Value){
		"unused parameter marker": func(d *Value) {
			schema := &Value{Kind: Object}
			schema.Set(rawMarker, boolean(true))
			param := &Value{Kind: Object}
			param.Set("schema", schema)
			params := &Value{Kind: Object}
			params.Set("Unused", param)
			d.Get("components").Set("parameters", params)
		},
		"root marker":  func(d *Value) { d.Set(rawMarker, boolean(true)) },
		"false marker": func(d *Value) { d.Get("components").Get("schemas").Get("DraftBody").Set(rawMarker, boolean(false)) },
		"inline marker": func(d *Value) {
			d.Get("components").Get("schemas").Get("DraftMetadata").Get("properties").Get("enabled").Set(rawMarker, boolean(true))
		},
		"pattern": func(d *Value) {
			d.Get("components").Get("schemas").Get("DraftMetadata").Get("properties").Get("enabled").Set("pattern", str(".*"))
		},
		"default": func(d *Value) {
			d.Get("components").Get("schemas").Get("DraftMetadata").Get("properties").Get("enabled").Set("default", boolean(false))
		},
		"open envelope": func(d *Value) {
			d.Get("components").Get("schemas").Get("SaveEnvelope").Set("additionalProperties", boolean(true))
		},
		"extra keyword": func(d *Value) {
			d.Get("components").Get("schemas").Get("SaveEnvelope").Set("unevaluatedProperties", boolean(false))
		},
		"invalid name": func(d *Value) {
			d.Get("components").Get("schemas").Get("DraftMetadata").Get("properties").Set("-", &Value{Kind: Object, Fields: []Property{{"type", str("string")}}})
		},
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			doc := rawFixture(t)
			mutate(doc)
			if _, err := CompileRawJSONContract(doc); err == nil {
				t.Fatal("unsupported contract accepted")
			}
		})
	}
}

func TestRawExamplesAreValidatedWithoutOpeningOrdinaryProfile(t *testing.T) {
	doc := rawFixture(t)
	doc.Get("components").Get("schemas").Get("DraftMetadata").Set("example", &Value{Kind: Object})
	result, err := ValidateDocument(doc, ValidationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, issue := range result.Issues {
		if issue.Code == "apifox.raw_example" {
			found = true
		}
	}
	if !found {
		t.Fatal("invalid nested raw example accepted")
	}
	doc = rawFixture(t)
	// The same closed component remains unsupported outside a raw operation.
	doc.Get("paths").Set("/ordinary", doc.Get("paths").Get("/drafts/{draft_id}").Clone())
	ordinary := doc.Get("paths").Get("/ordinary").Get("put")
	ordinary.Get("requestBody").Get("content").Get("application/json").Set("schema", &Value{Kind: Object, Fields: []Property{{"$ref", str("#/components/schemas/DraftMetadata")}}})
	ordinary.Get("responses").Set("200", &Value{Kind: Object, Fields: []Property{{"description", str("ok")}}})
	if _, err := ConvertDocument(doc, ProjectionOptions{}); err == nil || !strings.Contains(err.Error(), "additionalProperties") {
		t.Fatalf("ordinary strict binder silently admitted: %v", err)
	}
}
