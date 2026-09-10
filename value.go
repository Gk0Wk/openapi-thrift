package openapithrift

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Value preserves object order, including ECMAScript's numeric-key ordering.
// This is part of the existing Thrift field-number contract, not cosmetic JSON
// formatting. Unknown OpenAPI extensions remain available to profile validation.
type Value struct {
	Kind   Kind
	Text   string
	Number float64
	Bool   bool
	Items  []*Value
	Fields []Property
	index  map[string]int
}

type Kind uint8

const (
	Null Kind = iota
	Object
	Array
	String
	Number
	Boolean
)

type Property struct {
	Name  string
	Value *Value
}

func (v *Value) Get(key string) *Value {
	if v != nil {
		if v.index != nil {
			if i, ok := v.index[key]; ok {
				return v.Fields[i].Value
			}
			return nil
		}
		for _, field := range v.Fields {
			if field.Name == key {
				return field.Value
			}
		}
	}
	return nil
}
func (v *Value) Has(key string) bool { return v.Get(key) != nil }
func (v *Value) Str() string {
	if v != nil && v.Kind == String {
		return v.Text
	}
	return ""
}
func (v *Value) Is(kind Kind) bool { return v != nil && v.Kind == kind }
func (v *Value) Truth() bool {
	return v != nil && v.Kind != Null && (v.Kind != Boolean || v.Bool) && (v.Kind != String || v.Text != "") && (v.Kind != Number || v.Number != 0)
}
func (v *Value) List() []*Value {
	if v != nil {
		return v.Items
	}
	return nil
}
func (v *Value) Entries() []Property {
	if v != nil {
		return v.Fields
	}
	return nil
}
func (v *Value) Set(name string, value *Value) {
	v.put(name, value)
	if _, err := strconv.ParseUint(name, 10, 32); err == nil {
		v.orderNumericKeys()
	}
}

func (v *Value) put(name string, value *Value) {
	if v.index == nil {
		v.index = make(map[string]int, len(v.Fields))
		for i, field := range v.Fields {
			v.index[field.Name] = i
		}
	}
	if i, ok := v.index[name]; ok {
		v.Fields[i].Value = value
		return
	}
	v.index[name] = len(v.Fields)
	v.Fields = append(v.Fields, Property{name, value})
}
func (v *Value) Clone() *Value {
	if v == nil {
		return nil
	}
	result := *v
	result.Fields = append([]Property(nil), v.Fields...)
	result.Items = append([]*Value(nil), v.Items...)
	result.index = nil
	return &result
}
func str(value string) *Value   { return &Value{Kind: String, Text: value} }
func boolean(value bool) *Value { return &Value{Kind: Boolean, Bool: value} }

// Parse accepts one JSON or YAML document, with a finite, bounded JSON data
// model. YAML aliases/merges resolve before validation; duplicate explicit keys
// and alias cycles are rejected instead of silently changing the contract.
func Parse(input []byte) (*Value, error) {
	if len(input) > 16<<20 {
		return nil, errors.New("OpenAPI input exceeds 16 MiB")
	}
	input = bytes.TrimPrefix(input, []byte{0xef, 0xbb, 0xbf})
	trimmed := bytes.TrimSpace(input)
	if len(trimmed) == 0 {
		return nil, errors.New("OpenAPI input is empty")
	}
	if trimmed[0] == '{' || trimmed[0] == '[' {
		decoder := json.NewDecoder(bytes.NewReader(trimmed))
		decoder.UseNumber()
		budget := 1_000_000
		value, err := readJSON(decoder, 0, &budget)
		if err != nil {
			return nil, err
		}
		if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
			return nil, errors.New("OpenAPI input must contain one JSON document")
		}
		return value, nil
	}
	var document yaml.Node
	decoder := yaml.NewDecoder(bytes.NewReader(trimmed))
	if err := decoder.Decode(&document); err != nil {
		return nil, fmt.Errorf("invalid OpenAPI YAML: %w", err)
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, errors.New("OpenAPI input must contain one YAML document")
	}
	budget := 1_000_000
	return fromYAML(&document, 0, &budget, make(map[*yaml.Node]bool))
}

func readJSON(decoder *json.Decoder, depth int, budget *int) (*Value, error) {
	if depth > 128 || *budget <= 0 {
		return nil, errors.New("OpenAPI document nesting or node limit exceeded")
	}
	*budget--
	token, err := decoder.Token()
	if err != nil {
		return nil, fmt.Errorf("invalid OpenAPI JSON: %w", err)
	}
	value := &Value{}
	switch token := token.(type) {
	case nil:
		return value, nil
	case string:
		return str(token), nil
	case bool:
		return boolean(token), nil
	case json.Number:
		number, err := token.Float64()
		if err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
			return nil, errors.New("OpenAPI numbers must be finite")
		}
		return &Value{Kind: Number, Number: number}, nil
	case json.Delim:
		switch token {
		case '{':
			value.Kind = Object
			seen := make(map[string]bool)
			for decoder.More() {
				key, err := decoder.Token()
				if err != nil {
					return nil, err
				}
				name, ok := key.(string)
				if !ok {
					return nil, errors.New("JSON object key must be a string")
				}
				if seen[name] {
					return nil, fmt.Errorf("duplicate JSON key %q", name)
				}
				seen[name] = true
				child, err := readJSON(decoder, depth+1, budget)
				if err != nil {
					return nil, err
				}
				value.Fields = append(value.Fields, Property{name, child})
			}
			value.orderNumericKeys()
		case '[':
			value.Kind = Array
			for decoder.More() {
				child, err := readJSON(decoder, depth+1, budget)
				if err != nil {
					return nil, err
				}
				value.Items = append(value.Items, child)
			}
		default:
			return nil, errors.New("unexpected JSON delimiter")
		}
		if _, err := decoder.Token(); err != nil {
			return nil, err
		}
		return value, nil
	default:
		return nil, errors.New("unsupported JSON value")
	}
}

func fromYAML(node *yaml.Node, depth int, budget *int, active map[*yaml.Node]bool) (*Value, error) {
	if node == nil || depth > 128 || *budget <= 0 || active[node] {
		return nil, errors.New("YAML alias cycle, nesting or node limit exceeded")
	}
	*budget--
	active[node] = true
	defer delete(active, node)
	value := &Value{}
	switch node.Kind {
	case yaml.DocumentNode:
		if len(node.Content) != 1 {
			return nil, errors.New("empty YAML document")
		}
		return fromYAML(node.Content[0], depth+1, budget, active)
	case yaml.AliasNode:
		return fromYAML(node.Alias, depth+1, budget, active)
	case yaml.SequenceNode:
		value.Kind = Array
		for _, child := range node.Content {
			item, err := fromYAML(child, depth+1, budget, active)
			if err != nil {
				return nil, err
			}
			value.Items = append(value.Items, item)
		}
	case yaml.MappingNode:
		value.Kind = Object
		// Merge sources provide defaults; explicit fields always take precedence.
		for i := 0; i < len(node.Content); i += 2 {
			if node.Content[i].Tag == "!!merge" {
				merged, err := fromYAML(node.Content[i+1], depth+1, budget, active)
				if err != nil {
					return nil, err
				}
				sources := []*Value{merged}
				if merged.Is(Array) {
					sources = merged.Items
				}
				for _, source := range sources {
					if !source.Is(Object) {
						return nil, errors.New("YAML merge must reference objects")
					}
					for _, field := range source.Fields {
						if !value.Has(field.Name) {
							value.put(field.Name, field.Value)
						}
					}
				}
			}
		}
		explicit := map[string]bool{}
		for i := 0; i < len(node.Content); i += 2 {
			key := node.Content[i]
			if key.Tag == "!!merge" {
				continue
			}
			if key.Kind != yaml.ScalarNode || (key.Tag != "!!str" && key.Tag != "!!int") {
				return nil, errors.New("YAML mapping keys must be strings or response codes")
			}
			name := key.Value
			if explicit[name] {
				return nil, fmt.Errorf("duplicate YAML key %q", name)
			}
			explicit[name] = true
			child, err := fromYAML(node.Content[i+1], depth+1, budget, active)
			if err != nil {
				return nil, err
			}
			value.put(name, child)
		}
		value.orderNumericKeys()
	case yaml.ScalarNode:
		switch node.Tag {
		case "!!null":
		case "!!str":
			return str(node.Value), nil
		case "!!bool":
			var decoded bool
			if err := node.Decode(&decoded); err != nil {
				return nil, err
			}
			return boolean(decoded), nil
		case "!!int", "!!float":
			var decoded float64
			if err := node.Decode(&decoded); err != nil {
				return nil, err
			}
			if math.IsNaN(decoded) || math.IsInf(decoded, 0) {
				return nil, errors.New("OpenAPI numbers must be finite")
			}
			return &Value{Kind: Number, Number: decoded}, nil
		default:
			return nil, fmt.Errorf("YAML tag %q has no JSON representation; quote the value", node.Tag)
		}
	default:
		return nil, errors.New("unsupported YAML node")
	}
	return value, nil
}

func (v *Value) orderNumericKeys() {
	index := func(key string) (uint64, bool) {
		n, err := strconv.ParseUint(key, 10, 32)
		return n, err == nil && n < math.MaxUint32 && strconv.FormatUint(n, 10) == key
	}
	sort.SliceStable(v.Fields, func(i, j int) bool {
		a, ai := index(v.Fields[i].Name)
		b, bi := index(v.Fields[j].Name)
		if ai && bi {
			return a < b
		}
		return ai && !bi
	})
	v.index = make(map[string]int, len(v.Fields))
	for i, field := range v.Fields {
		v.index[field.Name] = i
	}
}

func (v *Value) MarshalJSON() ([]byte, error) {
	if v == nil {
		return []byte("null"), nil
	}
	switch v.Kind {
	case Null:
		return []byte("null"), nil
	case String:
		return json.Marshal(v.Text)
	case Number:
		return json.Marshal(v.Number)
	case Boolean:
		return json.Marshal(v.Bool)
	case Array:
		if v.Items == nil {
			return []byte("[]"), nil
		}
		return json.Marshal(v.Items)
	case Object:
		var b strings.Builder
		b.WriteByte('{')
		for i, field := range v.Fields {
			if i > 0 {
				b.WriteByte(',')
			}
			name, _ := json.Marshal(field.Name)
			body, err := json.Marshal(field.Value)
			if err != nil {
				return nil, err
			}
			b.Write(name)
			b.WriteByte(':')
			b.Write(body)
		}
		b.WriteByte('}')
		return []byte(b.String()), nil
	default:
		return nil, errors.New("invalid JSON value kind")
	}
}
