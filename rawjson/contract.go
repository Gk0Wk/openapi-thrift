package rawjson

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"unicode/utf8"
)

// Contract is an immutable, validated renderer descriptor used by generated transport.
type Contract struct {
	schemas    map[string]*schema
	operations map[string]operation
}

type wireContract struct {
	SchemaVersion int                `json:"schemaVersion"`
	Schemas       map[string]*schema `json:"schemas"`
	Operations    []operation        `json:"operations"`
}

type operation struct {
	SuccessStatus int `json:"successStatus,omitempty"`
	Parameters    []struct {
		In   string `json:"in"`
		Name string `json:"name"`
	} `json:"parameters,omitempty"`
	Method              string  `json:"method"`
	Path                string  `json:"path"`
	RequestBody         *schema `json:"requestBody"`
	RequestBodyRequired *bool   `json:"requestBodyRequired"`
	ResponseBody        *schema `json:"responseBody"`
}

// LoadContract rejects descriptor versions or rules outside the supported profile.
func LoadContract(data []byte) (*Contract, error) {
	parsed, err := parseStrict(data)
	if err != nil {
		return nil, err
	}
	root, ok := parsed.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("raw JSON contract: expected object")
	}
	if err := checkKeys(root, "schemaVersion", "schemas", "operations"); err != nil {
		return nil, err
	}
	var wire wireContract
	if err := decodeTyped(data, &wire); err != nil {
		return nil, err
	}
	if wire.SchemaVersion != 1 || wire.Schemas == nil || wire.Operations == nil {
		return nil, fmt.Errorf("raw JSON contract: unsupported version or missing collections")
	}
	contract := &Contract{schemas: wire.Schemas, operations: make(map[string]operation)}
	for _, definition := range contract.schemas {
		if err := checkSchema(definition, contract.schemas, make(map[*schema]bool)); err != nil {
			return nil, err
		}
		if definition.Type != "object" && definition.OneOf == nil {
			return nil, fmt.Errorf("raw JSON contract: component root must be a closed object or discriminated union")
		}
	}
	operations, ok := root["operations"].([]any)
	if !ok {
		return nil, fmt.Errorf("raw JSON contract: operations must be an array")
	}
	for index, item := range wire.Operations {
		fields, ok := operations[index].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("raw JSON contract: operation must be an object")
		}
		if err := checkKeys(fields, "method", "path", "requestBody", "requestBodyRequired", "responseBody", "successStatus", "parameters"); err != nil {
			return nil, err
		}
		if _, exists := fields["successStatus"]; exists && (item.SuccessStatus < 200 || item.SuccessStatus > 299) {
			return nil, fmt.Errorf("raw JSON contract: invalid success status")
		}
		for _, value := range fields {
			if value == nil {
				return nil, fmt.Errorf("raw JSON contract: operation fields cannot be null")
			}
		}
		switch item.Method {
		case "get", "post", "put", "patch", "delete", "head", "options", "trace":
		default:
			return nil, fmt.Errorf("raw JSON contract: unsupported operation method")
		}
		if !strings.HasPrefix(item.Path, "/") {
			return nil, fmt.Errorf("raw JSON contract: operation path must be absolute")
		}
		if (item.RequestBody == nil) != (item.RequestBodyRequired == nil) {
			return nil, fmt.Errorf("raw JSON contract: request body presence rule is missing or orphaned")
		}
		for _, body := range []*schema{item.RequestBody, item.ResponseBody} {
			if body == nil {
				continue
			}
			if err := checkSchema(body, contract.schemas, make(map[*schema]bool)); err != nil {
				return nil, err
			}
			if body.Type != "object" || body.Nullable != nil {
				return nil, fmt.Errorf("raw JSON contract: operation body requires a typed object envelope")
			}
		}
		key := item.Method + " " + item.Path
		if _, exists := contract.operations[key]; exists {
			return nil, fmt.Errorf("raw JSON contract: duplicate operation")
		}
		contract.operations[key] = item
	}
	return contract, nil
}

// ValidateComponent validates one explicitly marked component without normalizing data.
func (contract *Contract) ValidateComponent(name string, data []byte) error {
	if contract == nil {
		return fmt.Errorf("raw JSON contract is unavailable")
	}
	definition, ok := contract.schemas[name]
	if !ok {
		return fmt.Errorf("raw JSON contract: unknown component")
	}
	return validateData(definition, data)
}

// ValidateSchema checks author examples with the same descriptor and rules as
// the wire runtime. It never normalizes values or executes external resources.
func (contract *Contract) ValidateSchema(descriptor, data []byte) error {
	var definition schema
	if err := json.Unmarshal(descriptor, &definition); err != nil {
		return err
	}
	if err := checkSchema(&definition, contract.schemas, make(map[*schema]bool)); err != nil {
		return err
	}
	return validateData(&definition, data)
}

// DecodeRequest validates the original JSON before a dedicated typed decode.
func (contract *Contract) DecodeRequest(method, path string, data []byte, target any) error {
	item, err := contract.operation(method, path)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		if item.RequestBodyRequired != nil && *item.RequestBodyRequired {
			return fmt.Errorf("raw JSON: request body is required")
		}
		return nil
	}
	if item.RequestBody == nil {
		return fmt.Errorf("raw JSON: unexpected request body")
	}
	if err := validateData(item.RequestBody, data); err != nil {
		return err
	}
	return decodeTyped(data, target)
}

// MarshalResponse validates the response before its bytes reach an HTTP writer.
func (contract *Contract) MarshalResponse(method, path string, value any) ([]byte, error) {
	item, err := contract.operation(method, path)
	if err != nil {
		return nil, err
	}
	if item.ResponseBody == nil {
		return nil, fmt.Errorf("raw JSON: operation has no JSON response")
	}
	if err := validateGoStrings(reflect.ValueOf(value), make(map[visit]bool)); err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("raw JSON: response encoding failed: %w", err)
	}
	if err := validateData(item.ResponseBody, encoded); err != nil {
		return nil, err
	}
	return encoded, nil
}

func (contract *Contract) operation(method, path string) (operation, error) {
	if contract == nil {
		return operation{}, fmt.Errorf("raw JSON contract is unavailable")
	}
	item, ok := contract.operations[method+" "+path]
	if !ok {
		return operation{}, fmt.Errorf("raw JSON contract: unknown operation")
	}
	return item, nil
}

func decodeTyped(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	decoder.UseNumber()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("raw JSON: typed decoding failed: %w", err)
	}
	return nil
}

func validateData(definition *schema, data []byte) error {
	value, err := parseStrict(data)
	if err != nil {
		return err
	}
	return definition.validate(value)
}

func checkKeys[T any](fields map[string]T, allowed ...string) error {
	for key := range fields {
		found := false
		for _, candidate := range allowed {
			if key == candidate {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("raw JSON contract: unsupported descriptor field %q", key)
		}
	}
	return nil
}

type visit struct {
	pointer uintptr
	kind    reflect.Kind
	typ     reflect.Type
}

// Reject invalid Go strings before encoding/json can replace bytes with U+FFFD.
func validateGoStrings(value reflect.Value, seen map[visit]bool) error {
	if !value.IsValid() {
		return nil
	}
	switch value.Kind() {
	case reflect.Interface:
		if value.IsNil() {
			return nil
		}
		return validateGoStrings(value.Elem(), seen)
	case reflect.Pointer, reflect.Map, reflect.Slice:
		if value.IsNil() {
			return nil
		}
		key := visit{value.Pointer(), value.Kind(), value.Type()}
		if seen[key] {
			return nil
		}
		seen[key] = true
		defer delete(seen, key)
	}
	switch value.Kind() {
	case reflect.Pointer:
		return validateGoStrings(value.Elem(), seen)
	case reflect.String:
		if !utf8.ValidString(value.String()) {
			return fmt.Errorf("raw JSON: response contains invalid UTF-8")
		}
	case reflect.Map:
		iter := value.MapRange()
		for iter.Next() {
			if err := validateGoStrings(iter.Key(), seen); err != nil {
				return err
			}
			if err := validateGoStrings(iter.Value(), seen); err != nil {
				return err
			}
		}
	case reflect.Array, reflect.Slice:
		if value.Type().Elem().Kind() == reflect.Uint8 {
			return nil
		}
		for i := 0; i < value.Len(); i++ {
			if err := validateGoStrings(value.Index(i), seen); err != nil {
				return err
			}
		}
	case reflect.Struct:
		typ := value.Type()
		for i := 0; i < value.NumField(); i++ {
			field := typ.Field(i)
			if field.PkgPath != "" {
				fieldType := field.Type
				if fieldType.Kind() == reflect.Pointer {
					fieldType = fieldType.Elem()
				}
				if !field.Anonymous || fieldType.Kind() != reflect.Struct {
					continue
				}
			}
			if field.Tag.Get("json") == "-" {
				continue
			}
			if err := validateGoStrings(value.Field(i), seen); err != nil {
				return err
			}
		}
	}
	return nil
}
