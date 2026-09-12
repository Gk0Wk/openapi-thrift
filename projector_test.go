package openapithrift

import (
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestGoProjectionPreservesPublishedReferenceCorpus(t *testing.T) {
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
				Name    string `json:"name"`
				Message string `json:"message"`
				Pointer string `json:"pointer"`
			} `json:"error"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(body, &corpus); err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, item := range corpus.Cases {
		if item.Action != "convertOpenApiToThrift" {
			continue
		}
		count++
		input := []byte(item.Args[0])
		var text string
		if len(input) > 0 && input[0] == '"' {
			if err := json.Unmarshal(input, &text); err != nil {
				t.Fatal(err)
			}
			input = []byte(text)
		}
		options := ProjectionOptions{}
		if len(item.Args) > 1 {
			if err := json.Unmarshal(item.Args[1], &options); err != nil {
				t.Fatal(err)
			}
		}
		result, err := Convert(input, options)
		if item.Error != nil {
			var projection *ProjectionError
			if !errors.As(err, &projection) || err.Error() != item.Error.Message || projection.Pointer != item.Error.Pointer {
				t.Errorf("case %d error differs: got %v; want %+v", count, err, item.Error)
			}
			continue
		}
		if err != nil {
			t.Errorf("case %d rejected: %v", count, err)
			continue
		}
		var want ProjectionResult
		if err := json.Unmarshal(item.Result, &want); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(result, want) {
			t.Errorf("case %d projection differs\nGOT:\n%s\nWANT:\n%s", count, result.Thrift, want.Thrift)
		}
	}
	if count < 35 {
		t.Fatalf("reference corpus incomplete: %d conversion cases", count)
	}
}

func TestGoProjectionRejectsCyclicAliasesButAllowsRecursiveStructs(t *testing.T) {
	for _, schema := range []string{`{"A":{"$ref":"#/components/schemas/A"}}`, `{"A":{"type":"array","items":{"$ref":"#/components/schemas/A"}}}`} {
		input := []byte(`{"openapi":"3.0.3","paths":{"/example":{"get":{"responses":{"200":{"content":{"application/json":{"schema":{"$ref":"#/components/schemas/A"}}}}}}}},"components":{"schemas":` + schema + `}}`)
		if _, err := Convert(input, ProjectionOptions{}); err == nil {
			t.Error("cyclic non-struct alias accepted")
		}
	}
}

func TestGoProjectionAddsRecursiveArrayItemValidators(t *testing.T) {
	input := []byte(`{
  "openapi":"3.0.3",
  "info":{"title":"ArrayValidation"},
  "paths":{"/items":{"post":{"requestBody":{"required":true,"content":{"application/json":{"schema":{"$ref":"#/components/schemas/Request"}}}},"responses":{"200":{"description":"ok"}}}}},
  "components":{"schemas":{
    "Request":{"type":"object","required":["tags","items"],"properties":{
      "tags":{"type":"array","minItems":1,"maxItems":5,"items":{"type":"string","minLength":2,"maxLength":8}},
      "items":{"type":"array","items":{"$ref":"#/components/schemas/Item"}}
    }},
    "Item":{"type":"object","required":["name"],"properties":{"name":{"type":"string","minLength":3}}}
  }}
}`)
	result, err := Convert(input, ProjectionOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Thrift, `tags (api.body="tags", go.tag='validate:"required,min=1,max=5,dive,min=2,max=8"')`) {
		t.Fatalf("primitive array item validators missing:\n%s", result.Thrift)
	}
	if !strings.Contains(result.Thrift, `items (api.body="items", go.tag='validate:"required,dive"')`) {
		t.Fatalf("object array dive validator missing:\n%s", result.Thrift)
	}
	if !strings.Contains(result.Thrift, `name (go.tag='validate:"required,min=3"')`) {
		t.Fatalf("nested object validator missing:\n%s", result.Thrift)
	}
}

func TestGoProjectionRecursesThroughNestedArrays(t *testing.T) {
	input := []byte(`{
  "openapi":"3.0.3",
  "info":{"title":"NestedArrayValidation"},
  "paths":{"/items":{"post":{"requestBody":{"content":{"application/json":{"schema":{"$ref":"#/components/schemas/Request"}}}},"responses":{"200":{"description":"ok"}}}}},
  "components":{"schemas":{
    "Request":{"type":"object","properties":{"matrix":{"type":"array","items":{"type":"array","items":{"type":"string","minLength":2}}}}}
  }}
}`)
	result, err := Convert(input, ProjectionOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Thrift, `matrix (api.body="matrix", go.tag='validate:"dive,dive,min=2"')`) {
		t.Fatalf("nested array validators missing:\n%s", result.Thrift)
	}
}
