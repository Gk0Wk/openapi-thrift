package rawjson

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestMaxLengthErrorRequestAndResponseKeepTheirBoundaries(t *testing.T) {
	contract := loadTestContract(t)
	type envelope struct {
		Version int32           `json:"version"`
		Body    json.RawMessage `json:"body"`
	}
	original := envelope{Version: 7, Body: json.RawMessage(`{"text":"old"}`)}
	target := original
	input := []byte(`{"version":1,"body":{"text":"private-value"}}`)
	err := contract.DecodeRequest("post", "/entries/{id}", input, &target)
	var limit *MaxLengthError
	if !errors.As(err, &limit) || limit.Path != "/body/text" || limit.Limit != 3 {
		t.Fatalf("request error lacks envelope path: %T %v", err, err)
	}
	if target.Version != original.Version || string(target.Body) != string(original.Body) {
		t.Fatal("invalid request reached typed decode")
	}
	output, err := contract.MarshalResponse("post", "/entries/{id}", envelope{1, json.RawMessage(`{"text":"private-value"}`)})
	if output != nil || !errors.As(err, &limit) || limit.Path != "/body/text" || limit.Limit != 3 {
		t.Fatalf("invalid response emitted bytes or lost error metadata: %T %v", err, err)
	}
	if strings.Contains(err.Error(), "private-value") {
		t.Fatal("invalid response contents leaked")
	}
}

const testDescriptor = `{
  "schemaVersion":1,
  "schemas":{"Draft":{"type":"object","properties":{
    "text":{"type":"string","nullable":true,"maxLength":3},
    "items":{"type":"array","items":{"type":"string"},"maxItems":2},
    "enabled":{"type":"boolean"},
    "value":{"type":"integer","format":"int64"}
  },"required":[],"additionalProperties":false,"nullable":true}},
  "operations":[{"method":"post","path":"/entries/{id}","requestBodyRequired":true,
    "requestBody":{"type":"object","properties":{
      "version":{"type":"integer","enum":[1]},
      "body":{"$ref":"#/schemas/Draft"}
    },"required":["version","body"],"additionalProperties":false},
    "responseBody":{"type":"object","properties":{
      "version":{"type":"integer","enum":[1]},
      "body":{"$ref":"#/schemas/Draft"}
    },"required":["version","body"],"additionalProperties":false}
  }]
}`

func loadTestContract(t *testing.T) *Contract {
	t.Helper()
	contract, err := LoadContract([]byte(testDescriptor))
	if err != nil {
		t.Fatal(err)
	}
	return contract
}

func TestDescriptorValidatesCompleteEnvelopeAndPreservesRawValues(t *testing.T) {
	contract := loadTestContract(t)
	type envelope struct {
		Version int32           `json:"version"`
		Body    json.RawMessage `json:"body"`
	}
	for _, input := range []string{
		`{"version":1,"body":{}}`,
		`{"version":1,"body":null}`,
		`{"version":1,"body":{"text":null}}`,
		`{"version":1,"body":{"text":"  "}}`,
		`{"version":1,"body":{"text":"\ud83d\ude00","enabled":false,"items":[],"value":9223372036854775807}}`,
	} {
		t.Run(input, func(t *testing.T) {
			var target envelope
			if err := contract.DecodeRequest("post", "/entries/{id}", []byte(input), &target); err != nil {
				t.Fatal(err)
			}
			output, err := contract.MarshalResponse("post", "/entries/{id}", target)
			if err != nil {
				t.Fatal(err)
			}
			if string(output) != input {
				t.Fatalf("output = %s, want %s", output, input)
			}
		})
	}
	if err := contract.ValidateComponent("Draft", []byte(`{"value":9007199254740993}`)); err != nil {
		t.Fatal(err)
	}
}

func TestDescriptorRejectsInvalidEnvelopeAndRawSubtree(t *testing.T) {
	contract := loadTestContract(t)
	cases := []string{
		`{}`, `{"version":1}`, `{"version":null,"body":{}}`, `{"version":2,"body":{}}`,
		`{"Version":1,"body":{}}`, `{"version":1,"body":{},"unknown":true}`,
		`{"version":1,"version":1,"body":{}}`, `{"version":1,"body":{"unknown":1}}`,
		`{"version":1,"body":{"text":"abcd"}}`, `{"version":1,"body":{"text":1}}`,
		`{"version":1,"body":{"items":["a","b","c"]}}`, `{"version":1,"body":{"items":[null]}}`,
		`{"version":1,"body":{"enabled":1}}`, `{"version":1,"body":{"value":9223372036854775808}}`,
		`{"version":1,"body":{"value":1.5}}`, `{"version":1,"body":{"text":"\ud800"}}`,
		`{"version":1,"body":{"text":"a","text":"b"}}`, `{"version":1,"body":{}} null`, "",
	}
	for _, input := range cases {
		t.Run(input, func(t *testing.T) {
			var target struct {
				Version int32           `json:"version"`
				Body    json.RawMessage `json:"body"`
			}
			if err := contract.DecodeRequest("post", "/entries/{id}", []byte(input), &target); err == nil {
				t.Fatal("expected rejection")
			}
		})
	}
}

func TestDescriptorRejectsUnknownRulesAndReferenceCycles(t *testing.T) {
	cases := []string{
		strings.Replace(testDescriptor, `"schemaVersion":1`, `"schemaVersion":2`, 1),
		strings.Replace(testDescriptor, `"schemaVersion":1`, `"schemaVersion":1,"future":true`, 1),
		strings.Replace(testDescriptor, `"maxLength":3`, `"maxLength":3,"pattern":".*"`, 1),
		strings.Replace(testDescriptor, `"maxLength":3`, `"maxLength":-1`, 1),
		strings.Replace(testDescriptor, `"format":"int64"`, `"format":"float"`, 1),
		strings.Replace(testDescriptor, `"type":"boolean"`, `"type":"number"`, 1),
		strings.Replace(testDescriptor, `"additionalProperties":false`, `"additionalProperties":true`, 1),
		strings.Replace(testDescriptor, `"enum":[1]`, `"enum":[1,1]`, 1),
		strings.Replace(testDescriptor, `"enum":[1]`, `"enum":[9007199254740992]`, 1),
		strings.Replace(testDescriptor, `"#/schemas/Draft"`, `"#/schemas/Missing"`, 1),
		strings.Replace(testDescriptor, `"type":"boolean"`, `"$ref":"#/schemas/Draft"`, 1),
		strings.Replace(testDescriptor, `"type":"boolean"`, `"type":"boolean","maxLength":3`, 1),
	}
	for index, input := range cases {
		if _, err := LoadContract([]byte(input)); err == nil {
			t.Errorf("case %d: expected invalid descriptor rejection", index)
		}
	}
}

func TestDescriptorRejectsResponseBeforeUnicodeRepairOrRawNormalization(t *testing.T) {
	contract := loadTestContract(t)
	for _, body := range []any{
		map[string]string{"text": string([]byte{0xff})},
		json.RawMessage(`{"text":"\ud800"}`),
		json.RawMessage(`{"text":"a","text":"b"}`),
		json.RawMessage(`{"unknown":1}`),
		json.RawMessage(`{"value":9223372036854775808}`),
	} {
		value := struct {
			Version int32 `json:"version"`
			Body    any   `json:"body"`
		}{Version: 1, Body: body}
		if output, err := contract.MarshalResponse("post", "/entries/{id}", value); err == nil || output != nil {
			t.Errorf("expected response rejection, output=%s err=%v", output, err)
		}
	}
}

func TestResponseUnicodeValidationChecksSiblingSlicesSharingStorage(t *testing.T) {
	input := strings.Replace(testDescriptor, `"enabled":{"type":"boolean"}`, `"enabled":{"type":"boolean"},"more_items":{"type":"array","items":{"type":"string"}}`, 1)
	contract, err := LoadContract([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	shared := []string{string([]byte{0xff})}
	body := struct {
		Items     []string `json:"items"`
		MoreItems []string `json:"more_items"`
	}{Items: shared[:0], MoreItems: shared[:1]}
	response := struct {
		Version int32 `json:"version"`
		Body    any   `json:"body"`
	}{Version: 1, Body: body}
	if output, err := contract.MarshalResponse("post", "/entries/{id}", response); err == nil || output != nil {
		t.Fatalf("shared slice bypassed Unicode validation: output=%s err=%v", output, err)
	}
}

type hiddenResponseFields struct {
	Text string `json:"text"`
}

func TestResponseUnicodeValidationIncludesAnonymousAndHyphenNamedFields(t *testing.T) {
	input := strings.Replace(testDescriptor, `"enabled":{"type":"boolean"}`, `"enabled":{"type":"boolean"},"-":{"type":"string"}`, 1)
	contract, err := LoadContract([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	for _, body := range []any{
		struct{ hiddenResponseFields }{hiddenResponseFields{Text: string([]byte{0xff})}},
		struct {
			Text string `json:"-,omitempty"`
		}{Text: string([]byte{0xff})},
	} {
		response := struct {
			Version int32 `json:"version"`
			Body    any   `json:"body"`
		}{Version: 1, Body: body}
		if output, err := contract.MarshalResponse("post", "/entries/{id}", response); err == nil || output != nil {
			t.Errorf("encoded field bypassed Unicode validation: output=%s err=%v", output, err)
		}
	}
}

func TestDescriptorRejectsNullableOperationEnvelopeRoots(t *testing.T) {
	for _, location := range []string{"requestBody", "responseBody"} {
		input := strings.Replace(testDescriptor, `"`+location+`":{"type":"object"`, `"`+location+`":{"type":"object","nullable":true`, 1)
		if _, err := LoadContract([]byte(input)); err == nil {
			t.Errorf("nullable %s root accepted", location)
		}
	}
}
