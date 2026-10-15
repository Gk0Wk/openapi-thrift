package openapithrift

import (
	"encoding/json"
	"github.com/Gk0Wk/openapi-thrift/rawjson"
	"testing"
)

func TestContractIRIncludesTransportMetadata(t *testing.T) {
	c, err := CompileRawJSONContract(rawFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	var wire struct {
		Operations []struct {
			SuccessStatus int                         `json:"successStatus"`
			Parameters    []struct{ In, Name string } `json:"parameters"`
		} `json:"operations"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		t.Fatal(err)
	}
	op := wire.Operations[0]
	if op.SuccessStatus != 200 || len(op.Parameters) != 1 || op.Parameters[0].In != "path" || op.Parameters[0].Name != "draft_id" {
		t.Fatalf("transport metadata missing: %s", data)
	}
}

func TestTypedIRPreservesEmptyClosedObject(t *testing.T) {
	doc := rawFixture(t)
	empty := &Value{Kind: Object}
	empty.Set("type", str("object"))
	empty.Set("properties", &Value{Kind: Object})
	empty.Set("additionalProperties", boolean(false))
	doc.Get("components").Get("schemas").Get("SaveEnvelope").Get("properties").Set("empty", empty)
	contract, err := CompileRawJSONContract(doc)
	if err != nil {
		t.Fatal(err)
	}
	wire, err := json.Marshal(contract)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rawjson.LoadContract(wire); err != nil {
		t.Fatalf("IR lost empty object semantics: %v", err)
	}
}
