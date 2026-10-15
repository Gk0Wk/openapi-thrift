package openapithrift

import "encoding/json"

// RawJSONContract is the normalized, versioned public IR. It does not expose
// the parser's Value tree or require consumers to reinterpret OpenAPI.
type RawJSONContract struct {
	SchemaVersion int                    `json:"schemaVersion"`
	Schemas       map[string]*JSONSchema `json:"schemas"`
	Operations    []RawJSONOperation     `json:"operations"`
}

type RawJSONParameter struct {
	In   string `json:"in"`
	Name string `json:"name"`
}

type RawJSONOperation struct {
	Method              string             `json:"method"`
	Path                string             `json:"path"`
	SuccessStatus       int                `json:"successStatus"`
	Parameters          []RawJSONParameter `json:"parameters"`
	RequestBody         *JSONSchema        `json:"requestBody,omitempty"`
	RequestBodyRequired *bool              `json:"requestBodyRequired,omitempty"`
	ResponseBody        *JSONSchema        `json:"responseBody,omitempty"`
}

// JSONSchema is a restricted compiled schema, not the authoring OpenAPI model.
// References identify entries in Contract.Schemas; numbers retain exact tokens.
type JSONSchema struct {
	Ref                  string                  `json:"$ref,omitempty"`
	Type                 string                  `json:"type,omitempty"`
	Properties           *map[string]*JSONSchema `json:"properties,omitempty"`
	Required             *[]string               `json:"required,omitempty"`
	AdditionalProperties *bool                   `json:"additionalProperties,omitempty"`
	Nullable             *bool                   `json:"nullable,omitempty"`
	Items                *JSONSchema             `json:"items,omitempty"`
	MinItems             *int64                  `json:"minItems,omitempty"`
	MaxItems             *int64                  `json:"maxItems,omitempty"`
	MinLength            *int64                  `json:"minLength,omitempty"`
	MaxLength            *int64                  `json:"maxLength,omitempty"`
	Enum                 []json.RawMessage       `json:"enum,omitempty"`
	Format               string                  `json:"format,omitempty"`
	Minimum              *int64                  `json:"minimum,omitempty"`
	Maximum              *int64                  `json:"maximum,omitempty"`
	OneOf                []*JSONSchema           `json:"oneOf,omitempty"`
	Discriminator        *JSONDiscriminator      `json:"discriminator,omitempty"`
}

type JSONDiscriminator struct {
	PropertyName string `json:"propertyName"`
}
