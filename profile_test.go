package openapithrift

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestGoProfilePreservesPublishedReferenceCorpus(t *testing.T) {
	body, err := os.ReadFile("tests/fixtures/go-core-reference.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Cases []struct {
			Action string            `json:"action"`
			Args   []json.RawMessage `json:"args"`
			Result json.RawMessage   `json:"result"`
			Error  *struct {
				Message string `json:"message"`
			} `json:"error"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(body, &corpus); err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, item := range corpus.Cases {
		if item.Action != "validateOpenApiRenderDocument" {
			continue
		}
		count++
		input := []byte(item.Args[0])
		if len(input) > 0 && input[0] == '"' {
			var text string
			if err := json.Unmarshal(input, &text); err != nil {
				t.Fatal(err)
			}
			input = []byte(text)
		}
		options := ValidationOptions{}
		if len(item.Args) > 1 {
			if err := json.Unmarshal(item.Args[1], &options); err != nil {
				t.Fatal(err)
			}
		}
		result, err := Validate(input, options)
		if item.Error != nil {
			if err == nil || err.Error() != item.Error.Message {
				t.Errorf("case %d: got %v; want %s", count, err, item.Error.Message)
			}
			continue
		}
		if err != nil {
			t.Errorf("case %d: %v", count, err)
			continue
		}
		var expected ValidationResult
		if err := json.Unmarshal(item.Result, &expected); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(result, expected) {
			got, _ := json.MarshalIndent(result, "", "  ")
			want, _ := json.MarshalIndent(expected, "", "  ")
			t.Errorf("case %d validation differs\nGOT:\n%s\nWANT:\n%s", count, got, want)
		}
	}
	if count != 5 {
		t.Fatalf("reference profile corpus incomplete: %d", count)
	}
}

func TestProfileRecursiveObjectAndAliasBoundaries(t *testing.T) {
	base := `{"openapi":"3.0.3","paths":{"/tree":{"get":{"operationId":"getTree","tags":["trees"],"responses":{"200":{"content":{"application/json":{"schema":{"$ref":"#/components/schemas/Node"}}}}}}}},"components":{"schemas":SCHEMAS}}`
	for _, item := range []struct {
		name, schemas string
		valid         bool
	}{
		{"recursive object", `{"Node":{"type":"object","properties":{"next":{"$ref":"#/components/schemas/Node"}}}}`, true},
		{"mutual object", `{"Node":{"type":"object","properties":{"next":{"$ref":"#/components/schemas/Edge"}}},"Edge":{"type":"object","properties":{"node":{"$ref":"#/components/schemas/Node"}}}}`, true},
		{"cyclic alias", `{"Node":{"$ref":"#/components/schemas/Node"}}`, false},
		{"mutual aliases", `{"Node":{"$ref":"#/components/schemas/Edge"},"Edge":{"$ref":"#/components/schemas/Node"}}`, false},
		{"cyclic array", `{"Node":{"type":"array","items":{"$ref":"#/components/schemas/Node"}}}`, false},
	} {
		t.Run(item.name, func(t *testing.T) {
			input := []byte(strings.Replace(base, "SCHEMAS", item.schemas, 1))
			result, err := Validate(input, ValidationOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if (result.ErrorCount == 0) != item.valid {
				t.Fatalf("unexpected validity: %+v", result)
			}
			if item.valid {
				projection, err := Convert(input, ProjectionOptions{})
				if err != nil || !strings.Contains(projection.Thrift, "struct Node") {
					t.Fatalf("recursive object was not projected: %v", err)
				}
			}
		})
	}
}

func TestValidationRejectsMalformedInputWithoutPanic(t *testing.T) {
	for _, input := range []string{`null`, `[]`, `{"openapi":"3.0.3","paths":{"/bad":{"get":{"parameters":{}}}}}`, `{"openapi":"3.0.3","paths":{"/bad":{"get":{"responses":{"200":{"content":{"application/json":{"schema":false}}}}}}}}`} {
		result, err := Validate([]byte(input), ValidationOptions{})
		if err == nil && result.ErrorCount == 0 {
			t.Errorf("malformed input accepted: %s", input)
		}
	}
}
