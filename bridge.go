package openapithrift

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// BridgeRequest is the browser/WASM wire boundary. All actual conversion and
// validation remains in the native core; no JavaScript projection is shipped.
type BridgeRequest struct {
	Action  string             `json:"action"`
	Input   json.RawMessage    `json:"input,omitempty"`
	Options json.RawMessage    `json:"options,omitempty"`
	Files   []ThriftSourceFile `json:"files,omitempty"`
	Method  string             `json:"method,omitempty"`
	Path    string             `json:"path,omitempty"`
	Issues  []ValidationIssue  `json:"issues,omitempty"`
}
type BridgeError struct {
	Name    string            `json:"name"`
	Message string            `json:"message"`
	Pointer string            `json:"pointer,omitempty"`
	Issues  []ValidationIssue `json:"issues,omitempty"`
}
type BridgeResponse struct {
	Result json.RawMessage `json:"result,omitempty"`
	Error  *BridgeError    `json:"error,omitempty"`
}

func DispatchJSON(body []byte) []byte {
	result, err := dispatchJSON(body)
	response := BridgeResponse{Result: result}
	if err != nil {
		response.Error = &BridgeError{Name: "Error", Message: err.Error()}
		var projection *ProjectionError
		var validation *ValidationError
		if errors.As(err, &projection) {
			response.Error = &BridgeError{Name: "OpenApiProjectionError", Message: projection.Message, Pointer: projection.Pointer}
		} else if errors.As(err, &validation) {
			response.Error = &BridgeError{Name: "OpenApiRenderValidationError", Message: err.Error(), Issues: validation.Issues}
		}
	}
	encoded, marshalErr := json.Marshal(response)
	if marshalErr != nil {
		return []byte(`{"error":{"name":"Error","message":"cannot encode core response"}}`)
	}
	return encoded
}
func dispatchJSON(body []byte) (json.RawMessage, error) {
	if len(body) > 32<<20 {
		return nil, errors.New("WASM request exceeds 32 MiB")
	}
	var request BridgeRequest
	if err := decodeBridge(body, &request); err != nil {
		return nil, err
	}
	switch request.Action {
	case "buildRouteKey":
		return json.Marshal(BuildRouteKey(request.Method, request.Path))
	case "normalizeRoutePath":
		return json.Marshal(NormalizeRoutePath(request.Path))
	case "extractRouteMethodNameMapFromThriftSources":
		return json.Marshal(ExtractRouteMethodNames(request.Files))
	case "formatOpenApiRenderValidationIssues":
		return json.Marshal(FormatValidationIssues(request.Issues))
	case "renderThriftDocument":
		var document ThriftDocument
		if err := decodeBridge(request.Input, &document); err != nil {
			return nil, err
		}
		return json.Marshal(RenderThrift(document))
	}
	input := []byte(request.Input)
	if len(input) > 0 && input[0] == '"' {
		var source string
		if err := json.Unmarshal(input, &source); err != nil {
			return nil, err
		}
		input = []byte(source)
	}
	if request.Action == "validateOpenApiRenderDocument" || request.Action == "assertOpenApiRenderDocument" {
		var options ValidationOptions
		if err := decodeOptions(request.Options, &options); err != nil {
			return nil, err
		}
		document, err := Parse(input)
		if err != nil {
			return nil, err
		}
		result, err := ValidateDocument(document, options)
		if err != nil {
			return nil, err
		}
		if request.Action == "assertOpenApiRenderDocument" && result.ErrorCount > 0 {
			return nil, &ValidationError{result.Issues}
		}
		return json.Marshal(result)
	}
	document, err := ParseOpenAPIDocument(input)
	if err != nil {
		return nil, err
	}
	if request.Action == "parseOpenApiDocument" {
		return json.Marshal(document)
	}
	var options ProjectionOptions
	if err := decodeOptions(request.Options, &options); err != nil {
		return nil, err
	}
	switch request.Action {
	case "convertOpenApiToThrift":
		result, err := ConvertDocument(document, options)
		if err != nil {
			return nil, err
		}
		return json.Marshal(result)
	case "projectDocument":
		result, err := Project(document, options)
		if err != nil {
			return nil, err
		}
		return json.Marshal(result)
	default:
		return nil, fmt.Errorf("unknown core action: %s", request.Action)
	}
}
func decodeOptions(body []byte, value interface{}) error {
	if len(body) == 0 {
		return nil
	}
	return decodeBridge(body, value)
}
func decodeBridge(body []byte, value interface{}) error {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return fmt.Errorf("invalid core request: %w", err)
	}
	if err := decoder.Decode(new(json.RawMessage)); !errors.Is(err, io.EOF) {
		return errors.New("core request must contain one JSON document")
	}
	return nil
}
