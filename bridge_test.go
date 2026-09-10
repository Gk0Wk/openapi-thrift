package openapithrift

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestBridgePreservesStructuredErrorsAndRecovers(t *testing.T) {
	for _, input := range []string{
		`{"action":"unknown"}`,
		`{"action":"convertOpenApiToThrift","input":"{"}`,
		`{"action":"convertOpenApiToThrift","input":{"openapi":"3.0.3","paths":{}},"options":{"arbitraryFlag":true}}`,
		`{"action":"assertOpenApiRenderDocument","input":{"openapi":"2.0","paths":{}}}`,
		`{"action":"parseOpenApiDocument","input":[]}`,
		`{"action":"renderThriftDocument","input":{"name":"unknown"}}`,
		`{"action":"buildRouteKey"} {}`,
	} {
		var response BridgeResponse
		if err := json.Unmarshal(DispatchJSON([]byte(input)), &response); err != nil {
			t.Fatal(err)
		}
		if response.Error == nil {
			t.Errorf("invalid request accepted: %s", input)
		}
	}
	if output := DispatchJSON([]byte(`{"action":"buildRouteKey","method":"get","path":"/items/{id}"}`)); string(output) != `{"result":"GET /items/:id"}` {
		t.Fatalf("recovery failed: %s", output)
	}
}

func FuzzDocumentBoundary(f *testing.F) {
	for _, seed := range []string{`{}`, `{"openapi":"3.0.3","paths":{}}`, "openapi: 3.0.3\npaths: {}", `{"openapi":"3.0.3","components":{"schemas":{"A":{"$ref":"#/components/schemas/A"}}}}`, "x: &x [*x]", `{"x":1,"x":2}`} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, input []byte) {
		if len(input) > 32<<10 {
			t.Skip()
		}
		document, err := Parse(input)
		if err != nil {
			return
		}
		before, err := json.Marshal(document)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = ValidateDocument(document, ValidationOptions{})
		_, _ = ConvertDocument(document, ProjectionOptions{})
		after, err := json.Marshal(document)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(before, after) {
			t.Fatal("validation/projection modified the source document")
		}
	})
}
