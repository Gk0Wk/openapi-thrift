package openapithrift

import (
	"os"
	"strings"
	"testing"
)

func TestRouteIndexUsesCanonicalPathsAndDeclaredPrecedence(t *testing.T) {
	files := []ThriftSourceFile{{Path: "first.thrift", Content: `Result Get(1: Request req) (api.get="/api/v1/items/:id")`}, {Path: "second.thrift", Content: `Result GetItem(1: Request req) (api.get="/api/v1/items/{id}")`}}
	index := ExtractRouteMethodNames(files)
	if len(index) != 1 || index[BuildRouteKey("get", "/api/v1/items/{id}")] != "GetItem" {
		t.Fatalf("incorrect route index: %v", index)
	}
	if fallbackMethod("get", "/api/v1/items/{id}") != "GetItemsById" || snake("HTTPResultID") != "httpresult_id" {
		t.Fatal("naming contract changed")
	}
}

func TestRenderRetainsFixtureHeaderAndFinalNewline(t *testing.T) {
	fixture, err := os.ReadFile("tests/fixtures/apifox-boundary-lab.supported.thrift")
	if err != nil {
		t.Fatal(err)
	}
	rendered := RenderThrift(ThriftDocument{Namespace: "ispark.openapi", ServiceName: "ExampleService", Definitions: []ThriftStruct{{Name: "Empty", Fields: []ThriftField{}}}, Methods: []ThriftMethod{}})
	if !strings.HasPrefix(rendered, strings.Split(string(fixture), "namespace go")[0]) || !strings.HasSuffix(rendered, "}\n") {
		t.Fatalf("renderer framing changed: %s", rendered)
	}
}
