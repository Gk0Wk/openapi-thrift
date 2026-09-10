package openapithrift

import (
	"encoding/json"
	"errors"
	"os"
	"reflect"
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
