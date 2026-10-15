package rawjson

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const maxSafeInteger int64 = 9007199254740991

// MaxLengthError identifies a string limit without retaining any submitted value.
// Path is a JSON Pointer relative to the validated component or HTTP envelope.
type MaxLengthError struct {
	Path  string
	Limit int64
}

func (err *MaxLengthError) Error() string {
	return "raw JSON: value exceeds schema limits"
}

type schema struct {
	Ref                  string             `json:"$ref"`
	Type                 string             `json:"type"`
	Properties           map[string]*schema `json:"properties"`
	Required             []string           `json:"required"`
	AdditionalProperties *bool              `json:"additionalProperties"`
	Nullable             *bool              `json:"nullable"`
	Items                *schema            `json:"items"`
	MinItems             *int64             `json:"minItems"`
	MaxItems             *int64             `json:"maxItems"`
	MinLength            *int64             `json:"minLength"`
	MaxLength            *int64             `json:"maxLength"`
	Enum                 []json.RawMessage  `json:"enum"`
	Format               string             `json:"format"`
	Minimum              *int64             `json:"minimum"`
	Maximum              *int64             `json:"maximum"`
	OneOf                []*schema          `json:"oneOf"`
	Discriminator        *struct {
		PropertyName string `json:"propertyName"`
	} `json:"discriminator"`
	fields      map[string]json.RawMessage
	target      *schema
	stringEnum  []string
	integerEnum []int64
	branches    map[string]*schema
}

func (definition *schema) UnmarshalJSON(data []byte) error {
	type wire schema
	var value wire
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	for _, raw := range fields {
		if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return fmt.Errorf("raw JSON contract: descriptor rules cannot be null")
		}
	}
	*definition = schema(value)
	definition.fields = fields
	return nil
}

func checkSchema(definition *schema, schemas map[string]*schema, visiting map[*schema]bool) error {
	if definition == nil {
		return fmt.Errorf("raw JSON contract: missing schema")
	}
	if visiting[definition] {
		return fmt.Errorf("raw JSON contract: reference cycle")
	}
	visiting[definition] = true
	defer delete(visiting, definition)
	if definition.Nullable != nil && !*definition.Nullable {
		return fmt.Errorf("raw JSON contract: nullable must be true or omitted")
	}
	if _, ok := definition.fields["$ref"]; ok {
		if err := checkKeys(definition.fields, "$ref"); err != nil {
			return err
		}
		if !strings.HasPrefix(definition.Ref, "#/schemas/") {
			return fmt.Errorf("raw JSON contract: unsupported reference")
		}
		name := strings.TrimPrefix(definition.Ref, "#/schemas/")
		target, ok := schemas[name]
		if !ok {
			return fmt.Errorf("raw JSON contract: unknown reference")
		}
		definition.target = target
		return checkSchema(target, schemas, visiting)
	}
	if _, ok := definition.fields["oneOf"]; ok {
		if err := checkKeys(definition.fields, "oneOf", "discriminator", "nullable"); err != nil {
			return err
		}
		if len(definition.OneOf) < 2 || definition.Discriminator == nil || definition.Discriminator.PropertyName == "" {
			return fmt.Errorf("raw JSON contract: invalid discriminator")
		}
		property := definition.Discriminator.PropertyName
		var discriminatorFields map[string]json.RawMessage
		if err := json.Unmarshal(definition.fields["discriminator"], &discriminatorFields); err != nil {
			return err
		}
		if err := checkKeys(discriminatorFields, "propertyName"); err != nil {
			return err
		}
		definition.branches = make(map[string]*schema)
		for _, branch := range definition.OneOf {
			if err := checkSchema(branch, schemas, visiting); err != nil {
				return err
			}
			if branch.Type != "object" || branch.Nullable != nil {
				return fmt.Errorf("raw JSON contract: discriminator branch must be a non-null object")
			}
			tag := branch.Properties[property]
			if !slices.Contains(branch.Required, property) || tag == nil || tag.Type != "string" || tag.Nullable != nil || len(tag.stringEnum) != 1 {
				return fmt.Errorf("raw JSON contract: invalid discriminator property")
			}
			literal := tag.stringEnum[0]
			if _, exists := definition.branches[literal]; exists {
				return fmt.Errorf("raw JSON contract: duplicate discriminator literal")
			}
			definition.branches[literal] = branch
		}
		return nil
	}
	switch definition.Type {
	case "object":
		if err := checkKeys(definition.fields, "type", "properties", "required", "additionalProperties", "nullable"); err != nil {
			return err
		}
		if definition.Properties == nil || definition.Required == nil || definition.AdditionalProperties == nil || *definition.AdditionalProperties {
			return fmt.Errorf("raw JSON contract: object must have explicit closed properties and required fields")
		}
		seen := make(map[string]bool)
		var requiredValues []json.RawMessage
		if err := json.Unmarshal(definition.fields["required"], &requiredValues); err != nil {
			return err
		}
		for _, raw := range requiredValues {
			if len(raw) == 0 || raw[0] != '"' {
				return fmt.Errorf("raw JSON contract: required property names must be strings")
			}
		}
		for _, name := range definition.Required {
			if _, exists := definition.Properties[name]; !exists || seen[name] {
				return fmt.Errorf("raw JSON contract: invalid required property")
			}
			seen[name] = true
		}
		for _, child := range definition.Properties {
			if err := checkSchema(child, schemas, visiting); err != nil {
				return err
			}
		}
	case "array":
		if err := checkKeys(definition.fields, "type", "items", "minItems", "maxItems", "nullable"); err != nil {
			return err
		}
		if err := checkLimits(definition.MinItems, definition.MaxItems, true); err != nil {
			return err
		}
		if err := checkSchema(definition.Items, schemas, visiting); err != nil {
			return err
		}
	case "string":
		if err := checkKeys(definition.fields, "type", "enum", "minLength", "maxLength", "format", "nullable"); err != nil {
			return err
		}
		if err := checkLimits(definition.MinLength, definition.MaxLength, true); err != nil {
			return err
		}
		if _, exists := definition.fields["format"]; exists && !slices.Contains([]string{"uuid", "ulid", "date-time"}, definition.Format) {
			return fmt.Errorf("raw JSON contract: unsupported string format")
		}
		if _, exists := definition.fields["enum"]; exists {
			if len(definition.Enum) == 0 {
				return fmt.Errorf("raw JSON contract: empty string enum")
			}
			definition.stringEnum = nil
			for _, raw := range definition.Enum {
				var value string
				if err := json.Unmarshal(raw, &value); err != nil || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) || slices.Contains(definition.stringEnum, value) {
					return fmt.Errorf("raw JSON contract: invalid string enum")
				}
				definition.stringEnum = append(definition.stringEnum, value)
			}
		}
	case "integer":
		if err := checkKeys(definition.fields, "type", "enum", "minimum", "maximum", "format", "nullable"); err != nil {
			return err
		}
		if err := checkLimits(definition.Minimum, definition.Maximum, false); err != nil {
			return err
		}
		if _, exists := definition.fields["format"]; exists && !slices.Contains([]string{"int32", "int64"}, definition.Format) {
			return fmt.Errorf("raw JSON contract: unsupported integer format")
		}
		if _, exists := definition.fields["enum"]; exists {
			if len(definition.Enum) == 0 {
				return fmt.Errorf("raw JSON contract: empty integer enum")
			}
			definition.integerEnum = nil
			for _, raw := range definition.Enum {
				value, err := json.Number(string(raw)).Int64()
				if err != nil || value < -maxSafeInteger || value > maxSafeInteger || slices.Contains(definition.integerEnum, value) {
					return fmt.Errorf("raw JSON contract: invalid integer enum")
				}
				definition.integerEnum = append(definition.integerEnum, value)
			}
		}
	case "boolean":
		if err := checkKeys(definition.fields, "type", "nullable"); err != nil {
			return err
		}
	default:
		return fmt.Errorf("raw JSON contract: unsupported schema type")
	}
	return nil
}

func checkLimits(minimum, maximum *int64, nonnegative bool) error {
	for _, value := range []*int64{minimum, maximum} {
		if value != nil && (*value < -maxSafeInteger || *value > maxSafeInteger || nonnegative && *value < 0) {
			return fmt.Errorf("raw JSON contract: invalid schema limit")
		}
	}
	if minimum != nil && maximum != nil && *minimum > *maximum {
		return fmt.Errorf("raw JSON contract: inverted schema limits")
	}
	return nil
}

func (definition *schema) validate(value any) error {
	if definition.target != nil {
		return definition.target.validate(value)
	}
	if value == nil {
		if definition.Nullable != nil && *definition.Nullable {
			return nil
		}
		return fmt.Errorf("raw JSON: null is not allowed")
	}
	if definition.branches != nil {
		fields, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("raw JSON: discriminator requires an object")
		}
		tag, ok := fields[definition.Discriminator.PropertyName].(string)
		if !ok {
			return fmt.Errorf("raw JSON: missing or invalid discriminator")
		}
		branch, exists := definition.branches[tag]
		if !exists {
			return fmt.Errorf("raw JSON: unknown discriminator value")
		}
		return branch.validate(value)
	}
	switch definition.Type {
	case "object":
		fields, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("raw JSON: expected object")
		}
		for _, name := range definition.Required {
			if _, exists := fields[name]; !exists {
				return fmt.Errorf("raw JSON: required property is missing")
			}
		}
		for name, child := range fields {
			property, exists := definition.Properties[name]
			if !exists {
				return fmt.Errorf("raw JSON: unknown property")
			}
			if err := property.validate(child); err != nil {
				return prependMaxLengthPath(err, name)
			}
		}
	case "array":
		values, ok := value.([]any)
		if !ok {
			return fmt.Errorf("raw JSON: expected array")
		}
		if err := checkValueLimits(int64(len(values)), definition.MinItems, definition.MaxItems); err != nil {
			return err
		}
		for index, child := range values {
			if err := definition.Items.validate(child); err != nil {
				return prependMaxLengthPath(err, strconv.Itoa(index))
			}
		}
	case "string":
		text, ok := value.(string)
		if !ok {
			return fmt.Errorf("raw JSON: expected string")
		}
		length := int64(utf8.RuneCountInString(text))
		if definition.MaxLength != nil && length > *definition.MaxLength {
			return &MaxLengthError{Limit: *definition.MaxLength}
		}
		if err := checkValueLimits(length, definition.MinLength, nil); err != nil {
			return err
		}
		if definition.stringEnum != nil && !slices.Contains(definition.stringEnum, text) {
			return fmt.Errorf("raw JSON: string is outside enum")
		}
		if err := validateFormat(definition.Format, text); err != nil {
			return err
		}
	case "integer":
		number, ok := value.(json.Number)
		if !ok {
			return fmt.Errorf("raw JSON: expected integer")
		}
		integer, err := number.Int64()
		if err != nil {
			return fmt.Errorf("raw JSON: integer is outside signed 64-bit decimal representation")
		}
		if definition.Format == "int32" && (integer < -2147483648 || integer > 2147483647) {
			return fmt.Errorf("raw JSON: integer is outside signed 32-bit range")
		}
		if err := checkValueLimits(integer, definition.Minimum, definition.Maximum); err != nil {
			return err
		}
		if definition.integerEnum != nil && !slices.Contains(definition.integerEnum, integer) {
			return fmt.Errorf("raw JSON: integer is outside enum")
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("raw JSON: expected boolean")
		}
	default:
		return fmt.Errorf("raw JSON: unsupported schema")
	}
	return nil
}

// Only descriptor-approved property names or actual array positions reach here.
// Each rejected value creates its own error; the immutable schema is never changed.
func prependMaxLengthPath(err error, segment string) error {
	var limit *MaxLengthError
	if errors.As(err, &limit) {
		escaped := strings.ReplaceAll(strings.ReplaceAll(segment, "~", "~0"), "/", "~1")
		limit.Path = "/" + escaped + limit.Path
	}
	return err
}

func checkValueLimits(value int64, minimum, maximum *int64) error {
	if minimum != nil && value < *minimum || maximum != nil && value > *maximum {
		return fmt.Errorf("raw JSON: value exceeds schema limits")
	}
	return nil
}

var uuidPattern = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
var ulidPattern = regexp.MustCompile(`(?i)^[0-7][0-9A-HJKMNP-TV-Z]{25}$`)
var dateTimePattern = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\.[0-9]+)?(Z|[+-]([01][0-9]|2[0-3]):[0-5][0-9])$`)

func validateFormat(format, value string) error {
	valid := true
	switch format {
	case "":
	case "uuid":
		valid = uuidPattern.MatchString(value)
	case "ulid":
		valid = len(value) == 26 && ulidPattern.MatchString(value)
	case "date-time":
		_, err := time.Parse(time.RFC3339Nano, value)
		valid = dateTimePattern.MatchString(value) && err == nil
	default:
		valid = false
	}
	if !valid {
		return fmt.Errorf("raw JSON: string does not match format")
	}
	return nil
}
