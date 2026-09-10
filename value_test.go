package openapithrift

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParsePreservesThriftFieldOrderAndYAMLParity(t *testing.T) {
	inputs := []string{`{"openapi":"3.0.3","properties":{"z":{"type":"string"},"a":{"type":"integer"}},"responses":{"201":{},"200":{},"default":{}}}`, "openapi: 3.0.3\nproperties:\n  z: {type: string}\n  a: {type: integer}\nresponses:\n  201: {}\n  200: {}\n  default: {}\n"}
	var baseline string
	for _, input := range inputs {
		value, err := Parse([]byte(input))
		if err != nil {
			t.Fatal(err)
		}
		body, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if baseline == "" {
			baseline = string(body)
		} else if string(body) != baseline {
			t.Fatalf("JSON/YAML differ: %s != %s", body, baseline)
		}
		if value.Get("properties").Fields[0].Name != "z" || value.Get("responses").Fields[0].Name != "200" {
			t.Fatal("field-number order changed")
		}
	}
}

func TestParseRejectsAmbiguousOrUnboundedInput(t *testing.T) {
	for _, input := range []string{`{"a":1,"a":2}`, `{} {}`, `{"n":1e9999}`, "a: 1\na: 2", "a: &a {b: *a}", "a: 1\n---\nb: 2", "a: .inf", "a: !!binary aGVsbG8=", strings.Repeat("[", 130) + strings.Repeat("]", 130)} {
		if _, err := Parse([]byte(input)); err == nil {
			t.Errorf("accepted invalid input: %.80s", input)
		}
	}
}

func TestYAMLAliasMergeRetainsExplicitOverrides(t *testing.T) {
	value, err := Parse([]byte("base: &base {type: object, description: inherited}\nschema: {<<: *base, description: explicit}\n"))
	if err != nil {
		t.Fatal(err)
	}
	if value.Get("schema").Get("description").Str() != "explicit" || value.Get("schema").Get("type").Str() != "object" {
		t.Fatal("merge changed explicit schema")
	}
}
