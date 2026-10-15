package openapithrift

import (
	"os"
	"strings"
	"testing"
)

func TestOrdinaryResponseFixturesPreserveRequiredKeysOnly(t *testing.T) {
	for _, fixture := range []struct {
		file, response string
		keys           []string
	}{
		{"ordinary-response-map.openapi.json", "CapabilityResponse", []string{"headers", "aliases"}},
		{"ordinary-response-nullable.openapi.json", "DraftEnvelope", []string{"draft", "text", "enabled", "count", "ratio", "rows", "labels", "union_text", "via_alias"}},
	} {
		t.Run(fixture.file, func(t *testing.T) {
			body, err := os.ReadFile("tests/fixtures/" + fixture.file)
			if err != nil {
				t.Fatal(err)
			}
			result, err := Convert(body, ProjectionOptions{})
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, def := range result.Document.Definitions {
				if def.Name != fixture.response {
					continue
				}
				found = true
				for _, key := range fixture.keys {
					matched := false
					for _, field := range def.Fields {
						if field.Name == key {
							matched = strings.Contains(strings.Join(field.Annotations, " "), `json:"`+key+`"`)
						}
					}
					if !matched {
						t.Errorf("required response key %s can be omitted", key)
					}
				}
			}
			if !found {
				t.Fatal("response missing")
			}
			for _, def := range result.Document.Definitions {
				if def.Name == "RequestOnly" {
					for _, field := range def.Fields {
						if strings.Contains(strings.Join(field.Annotations, " "), "json:") {
							t.Fatal("response tags leaked to request-only DTO")
						}
					}
				}
			}
		})
	}
}

func TestDirectionalNameCollisionFailsInsteadOfSharingDTO(t *testing.T) {
	doc := rawFixture(t)
	// DraftMetadata is shared and therefore reserves DraftMetadataResponse.
	other := &Value{Kind: Object}
	other.Set("type", str("string"))
	doc.Get("components").Get("schemas").Set("DraftMetadataResponse", other)
	ref := &Value{Kind: Object}
	ref.Set("$ref", str("#/components/schemas/DraftMetadataResponse"))
	doc.Get("components").Get("schemas").Get("SavedEnvelope").Get("properties").Set("collision", ref)
	if _, err := ConvertDocument(doc, ProjectionOptions{}); err == nil || !strings.Contains(err.Error(), "名称冲突") {
		t.Fatalf("colliding DTO identity accepted: %v", err)
	}
}

func TestRequestResponseTypesDoNotShareNestedComponents(t *testing.T) {
	input := []byte(`{"openapi":"3.0.3","paths":{"/asset":{"post":{"requestBody":{"content":{"application/json":{"schema":{"$ref":"#/components/schemas/Asset"}}}},"responses":{"200":{"content":{"application/json":{"schema":{"$ref":"#/components/schemas/Asset"}}}}}}}},"components":{"schemas":{"Asset":{"type":"object","required":["headers","child"],"properties":{"headers":{"type":"object","additionalProperties":{"type":"string"}},"child":{"$ref":"#/components/schemas/Child"}}},"Child":{"type":"object","required":["value"],"properties":{"value":{"type":"string","nullable":true}}}}}}`)
	result, err := Convert(input, ProjectionOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"PostAssetRequest", "AssetResponse", "ChildRequest", "ChildResponse"} {
		if !strings.Contains(result.Thrift, "struct "+name+" {") {
			t.Errorf("missing independent %s: %s", name, result.Thrift)
		}
	}
	var request, response ThriftStruct
	for _, def := range result.Document.Definitions {
		if def.Name == "PostAssetRequest" {
			request = def
		}
		if def.Name == "AssetResponse" {
			response = def
		}
	}
	if len(request.Fields) == 0 || len(response.Fields) == 0 {
		t.Fatal("request or response has no fields")
	}
	if strings.Contains(strings.Join(request.Fields[0].Annotations, " "), `json:`) {
		t.Fatal("response serialization changed request")
	}
	if !strings.Contains(strings.Join(response.Fields[0].Annotations, " "), `json:"headers"`) {
		t.Fatal("required response map may be omitted")
	}
}

func TestRawJSONRequiresExplicitMarkerAndClosedEnvelope(t *testing.T) {
	input := []byte(`{"openapi":"3.0.3","paths":{"/raw":{"post":{"requestBody":{"required":true,"content":{"application/json":{"schema":{"type":"object","additionalProperties":false,"required":["payload"],"properties":{"payload":{"$ref":"#/components/schemas/Payload"}}}}}},"responses":{"204":{"description":"done"}}}}},"components":{"schemas":{"Payload":{"x-ispark-raw-json":true,"type":"object","additionalProperties":false,"properties":{"count":{"type":"integer","format":"int64"}}}}}}`)
	result, err := Convert(input, ProjectionOptions{})
	if err != nil {
		t.Fatalf("explicit strict raw JSON rejected: %v", err)
	}
	if !strings.Contains(result.Thrift, "typedef binary Payload") {
		t.Fatal("raw type was not preserved")
	}
	for _, bad := range []string{
		strings.Replace(string(input), `"x-ispark-raw-json":true`, `"x-ispark-raw-json":false`, 1),
		strings.Replace(string(input), `"additionalProperties":false`, `"additionalProperties":true`, 1),
	} {
		if _, err := Convert([]byte(bad), ProjectionOptions{}); err == nil {
			t.Fatal("invalid raw contract accepted")
		}
	}
}
