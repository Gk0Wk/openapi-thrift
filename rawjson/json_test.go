package rawjson

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestStrictJSONRejectsLossyOrAmbiguousInput(t *testing.T) {
	cases := []struct {
		name string
		data []byte
	}{
		{"duplicate root key", []byte(`{"a":1,"a":2}`)},
		{"duplicate escaped key", []byte(`{"a":1,"\u0061":2}`)},
		{"duplicate nested key", []byte(`{"a":[{"b":1,"b":2}]}`)},
		{"trailing value", []byte(`{"a":1} {"a":2}`)},
		{"trailing garbage", []byte(`{"a":1} !`)},
		{"invalid UTF8", []byte{'"', 0xff, '"'}},
		{"unpaired high surrogate", []byte(`"\ud800"`)},
		{"unpaired low surrogate", []byte(`"\udc00"`)},
		{"high surrogate followed by non-low", []byte(`"\ud800\u0041"`)},
		{"surrogate object key", []byte(`{"\ud800":1}`)},
		{"bad syntax", []byte(`{"a":}`)},
		{"empty", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := parseStrict(tc.data); err == nil {
				t.Fatal("expected strict rejection")
			}
		})
	}
}

func TestStrictJSONUsesTheStandardLibraryNestingBoundary(t *testing.T) {
	for _, depth := range []int{10000, 10001} {
		_, err := parseStrict([]byte(strings.Repeat("[", depth) + strings.Repeat("]", depth)))
		if depth == 10000 && err != nil {
			t.Fatalf("standard maximum depth rejected: %v", err)
		}
		if depth == 10001 && err == nil {
			t.Fatal("excessive nesting accepted")
		}
	}
}

func TestStrictJSONPreservesExactIntegerAndAcceptsValidUnicode(t *testing.T) {
	parsed, err := parseStrict([]byte(`{"large":9223372036854775807,"text":"\ud83d\ude00","literal":"\\ud800","blank":"  ","nil":null}`))
	if err != nil {
		t.Fatal(err)
	}
	object := parsed.(map[string]any)
	if got := string(object["large"].(json.Number)); got != "9223372036854775807" {
		t.Fatalf("integer changed: %s", got)
	}
	if got := object["text"].(string); got != "\U0001f600" {
		t.Fatalf("valid surrogate pair = %q", got)
	}
	if object["literal"] != `\ud800` || object["blank"] != "  " || object["nil"] != nil {
		t.Fatal("JSON values changed")
	}
}
