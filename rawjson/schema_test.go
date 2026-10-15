package rawjson

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestMaxLengthErrorPreservesUnicodeBoundaryAndSafePath(t *testing.T) {
	for _, unit := range []string{"a", "字", "🙂", "e\u0301"} {
		t.Run(fmt.Sprintf("%U", []rune(unit)), func(t *testing.T) {
			contract := propertyContract(t, `{"type":"string","maxLength":4}`)
			valid := strings.Repeat(unit, 4/len([]rune(unit)))
			payload, err := json.Marshal(map[string]string{"value": valid})
			if err != nil {
				t.Fatal(err)
			}
			if err := contract.ValidateComponent("Payload", payload); err != nil {
				t.Fatalf("exact Unicode boundary rejected: %v", err)
			}
			payload, err = json.Marshal(map[string]string{"value": valid + "x"})
			if err != nil {
				t.Fatal(err)
			}
			err = contract.ValidateComponent("Payload", payload)
			var limit *MaxLengthError
			if !errors.As(fmt.Errorf("request: %w", err), &limit) ||
				limit.Path != "/value" || limit.Limit != 4 {
				t.Fatalf("expected typed maxLength field: %T %v", err, err)
			}
			if err.Error() != "raw JSON: value exceeds schema limits" {
				t.Fatal("maxLength diagnostic changed or retained input")
			}
		})
	}
}

func TestMaxLengthErrorTracksArraysReferencesUnionsAndEscapedProperties(t *testing.T) {
	descriptor := `{"schemaVersion":1,"schemas":{
		"Item":{"oneOf":[
			{"type":"object","properties":{"kind":{"type":"string","enum":["text"]},"a~/b":{"type":"string","maxLength":2}},"required":["kind","a~/b"],"additionalProperties":false},
			{"type":"object","properties":{"kind":{"type":"string","enum":["flag"]},"value":{"type":"boolean"}},"required":["kind","value"],"additionalProperties":false}
		],"discriminator":{"propertyName":"kind"}},
		"Payload":{"type":"object","properties":{"items":{"type":"array","items":{"$ref":"#/schemas/Item"}}},"required":["items"],"additionalProperties":false}
	},"operations":[]}`
	contract, err := LoadContract([]byte(descriptor))
	if err != nil {
		t.Fatal(err)
	}
	err = contract.ValidateComponent("Payload", []byte(`{"items":[{"kind":"flag","value":false},{"kind":"text","a~/b":"private-value"}]}`))
	var limit *MaxLengthError
	if !errors.As(err, &limit) || limit.Path != "/items/1/a~0~1b" || limit.Limit != 2 {
		t.Fatalf("array/ref/union path was lost: %T %v", err, err)
	}
	if strings.Contains(err.Error(), "private-value") {
		t.Fatal("submitted value leaked through diagnostic")
	}
}

func TestMaxLengthErrorDoesNotReclassifyOtherRejections(t *testing.T) {
	for _, tc := range []struct{ schema, body string }{
		{`{"type":"string","minLength":2}`, `{"value":"a"}`},
		{`{"type":"array","items":{"type":"string"},"maxItems":0}`, `{"value":["private-value"]}`},
		{`{"type":"integer","maximum":0}`, `{"value":1}`},
		{`{"type":"string","maxLength":2}`, `{"private-key":"private-value"}`},
		{`{"type":"string","maxLength":2}`, `{"value":"a","value":"b"}`},
		{`{"type":"string","maxLength":2}`, `{"value":null}`},
		{`{"type":"string","maxLength":2}`, `{"value":"\ud800"}`},
	} {
		contract := propertyContract(t, tc.schema)
		err := contract.ValidateComponent("Payload", []byte(tc.body))
		var limit *MaxLengthError
		if err == nil || errors.As(err, &limit) {
			t.Fatalf("unrelated rejection became maxLength: %T %v", err, err)
		}
		if strings.Contains(err.Error(), "private-key") || strings.Contains(err.Error(), "private-value") {
			t.Fatal("unknown property or submitted value leaked")
		}
	}
}

func propertyContract(t *testing.T, property string) *Contract {
	t.Helper()
	contract, err := LoadContract([]byte(fmt.Sprintf(`{"schemaVersion":1,"schemas":{"Payload":{"type":"object","properties":{"value":%s},"required":["value"],"additionalProperties":false}},"operations":[]}`, property)))
	if err != nil {
		t.Fatal(err)
	}
	return contract
}

func TestSchemaFormatsUseTheExplicitG1Subset(t *testing.T) {
	cases := []struct {
		format         string
		valid, invalid []string
	}{
		{"uuid", []string{"00000000-0000-0000-0000-000000000000", "ABCD0123-ABCD-ABCD-ABCD-0123456789AB"}, []string{"00000000000000000000000000000000", "G0000000-0000-0000-0000-000000000000"}},
		{"ulid", []string{"01ARZ3NDEKTSV4RRFFQ69G5FAV", "01arz3ndektsv4rrffq69g5fav", "7ZZZZZZZZZZZZZZZZZZZZZZZZZ"}, []string{"81ARZ3NDEKTSV4RRFFQ69G5FAV", "01ARZ3NDEKTSV4RRFFQ69G5FAI", "01ARZ3NDEKTSV4RRFFQ69G5FAL", "01ARZ3NDEKTSV4RRFFQ69G5FAO", "01ARZ3NDEKTSV4RRFFQ69G5FAU"}},
		{"date-time", []string{"2024-02-29T12:34:56Z", "2024-02-29T12:34:56.123456789+08:00", "2024-02-29T12:34:56-23:59"}, []string{"2025-02-29T12:34:56Z", "2024-02-29t12:34:56z", "2024-02-29T12:34:60Z", "2024-02-29T24:00:00Z", "2024-02-29T12:34:56+24:00", "2024-02-29T12:34:56+00:60", "2024-02-29T12:34:56,123Z", "2024-02-29T1:34:56Z"}},
	}
	for _, tc := range cases {
		t.Run(tc.format, func(t *testing.T) {
			contract := propertyContract(t, fmt.Sprintf(`{"type":"string","format":%q}`, tc.format))
			for _, value := range tc.valid {
				if err := contract.ValidateComponent("Payload", []byte(fmt.Sprintf(`{"value":%q}`, value))); err != nil {
					t.Errorf("valid %q: %v", value, err)
				}
			}
			for _, value := range tc.invalid {
				if err := contract.ValidateComponent("Payload", []byte(fmt.Sprintf(`{"value":%q}`, value))); err == nil {
					t.Errorf("invalid %q accepted", value)
				}
			}
		})
	}
}

func TestSchemaIntegerRangesRemainExact(t *testing.T) {
	cases := []struct {
		property       string
		valid, invalid []string
	}{
		{`{"type":"integer","format":"int32"}`, []string{"-2147483648", "2147483647", "0"}, []string{"-2147483649", "2147483648", "1.5", "\"1\"", "null"}},
		{`{"type":"integer","format":"int64"}`, []string{"-9223372036854775808", "9223372036854775807", "9007199254740993", "-0"}, []string{"-9223372036854775809", "9223372036854775808", "1.0", "1e0", "1E+1", "-0.0"}},
		{`{"type":"integer","minimum":-2,"maximum":2,"enum":[-2,0,2]}`, []string{"-2", "0", "2"}, []string{"-3", "3", "1"}},
	}
	for _, tc := range cases {
		contract := propertyContract(t, tc.property)
		for _, value := range tc.valid {
			if err := contract.ValidateComponent("Payload", []byte(`{"value":`+value+`}`)); err != nil {
				t.Errorf("valid %s: %v", value, err)
			}
		}
		for _, value := range tc.invalid {
			if err := contract.ValidateComponent("Payload", []byte(`{"value":`+value+`}`)); err == nil {
				t.Errorf("invalid %s accepted", value)
			}
		}
	}
}

func TestSchemaDiscriminatorChoosesExactlyOneClosedBranch(t *testing.T) {
	property := `{"oneOf":[
        {"type":"object","properties":{"kind":{"type":"string","enum":["text"]},"text":{"type":"string","nullable":true}},"required":["kind"],"additionalProperties":false},
        {"type":"object","properties":{"kind":{"type":"string","enum":["flag"]},"flag":{"type":"boolean"}},"required":["kind","flag"],"additionalProperties":false}
    ],"discriminator":{"propertyName":"kind"},"nullable":true}`
	contract := propertyContract(t, property)
	for _, value := range []string{`null`, `{"kind":"text"}`, `{"kind":"text","text":null}`, `{"kind":"flag","flag":false}`} {
		if err := contract.ValidateComponent("Payload", []byte(`{"value":`+value+`}`)); err != nil {
			t.Errorf("valid %s: %v", value, err)
		}
	}
	for _, value := range []string{`{}`, `{"kind":null}`, `{"kind":"other"}`, `{"kind":"flag"}`, `{"kind":"text","flag":true}`, `{"kind":"text","kind":"flag","flag":true}`} {
		if err := contract.ValidateComponent("Payload", []byte(`{"value":`+value+`}`)); err == nil {
			t.Errorf("invalid %s accepted", value)
		}
	}
}

func TestSchemaRejectsNullRulesRatherThanTreatingThemAsOmitted(t *testing.T) {
	for _, input := range []string{
		strings.Replace(testDescriptor, `"maxLength":3`, `"maxLength":null`, 1),
		strings.Replace(testDescriptor, `"nullable":true`, `"nullable":null`, 1),
		strings.Replace(testDescriptor, `"requestBodyRequired":true`, `"requestBodyRequired":null`, 1),
		`{"schemaVersion":1,"schemas":{},"operations":[{"method":"post","path":"/empty","responseBody":null}]}`,
		`{"schemaVersion":1,"schemas":{},"operations":[{"method":"post","path":"/empty","requestBodyRequired":null}]}`,
	} {
		if _, err := LoadContract([]byte(input)); err == nil {
			t.Errorf("invalid null rule accepted: %s", input)
		}
	}
}

func TestSchemaULIDRejectsUnicodeCaseFoldsOutsideCrockfordASCII(t *testing.T) {
	contract := propertyContract(t, `{"type":"string","format":"ulid"}`)
	for _, value := range []string{
		strings.Replace("01ARZ3NDEKTSV4RRFFQ69G5FAV", "K", "\u212a", 1),
		strings.Replace("01ARZ3NDEKTSV4RRFFQ69G5FAV", "S", "\u017f", 1),
	} {
		if err := contract.ValidateComponent("Payload", []byte(fmt.Sprintf(`{"value":%q}`, value))); err == nil {
			t.Fatalf("non-ASCII ULID accepted: %q", value)
		}
	}
}
