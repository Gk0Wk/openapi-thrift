package openapithrift

import (
	"errors"
	"fmt"
	"math"
	"strings"
)

const ProfileApifoxHzThrift = "apifox-hz-thrift"

type ValidationIssue struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Pointer  string `json:"pointer"`
	Message  string `json:"message"`
}
type ValidationOptions struct {
	Profile            string `json:"profile,omitempty"`
	ValidateProjection *bool  `json:"validateProjection,omitempty"`
}
type ValidationResult struct {
	Profile        string            `json:"profile"`
	Issues         []ValidationIssue `json:"issues"`
	ErrorCount     int               `json:"errorCount"`
	WarningCount   int               `json:"warningCount"`
	PathCount      int               `json:"pathCount"`
	OperationCount int               `json:"operationCount"`
	SchemaCount    int               `json:"schemaCount"`
}
type ValidationError struct {
	Issues []ValidationIssue `json:"issues"`
}

func (e *ValidationError) Error() string { return FormatValidationIssues(e.Issues) }
func FormatValidationIssues(issues []ValidationIssue) string {
	lines := make([]string, len(issues))
	for i, issue := range issues {
		lines[i] = fmt.Sprintf("%s %s %s: %s", strings.ToUpper(issue.Severity), issue.Code, issue.Pointer, issue.Message)
	}
	return strings.Join(lines, "\n")
}

// Validate checks the shared APIFox/Hertz/Thrift profile. It never fetches refs,
// executes mock scripts, or reads other files. Projection is checked by default.
func Validate(input []byte, options ValidationOptions) (ValidationResult, error) {
	document, err := Parse(input)
	if err != nil {
		return ValidationResult{}, err
	}
	return ValidateDocument(document, options)
}
func AssertValid(document *Value, options ValidationOptions) (ValidationResult, error) {
	result, err := ValidateDocument(document, options)
	if err == nil && result.ErrorCount > 0 {
		err = &ValidationError{result.Issues}
	}
	return result, err
}
func ValidateDocument(document *Value, options ValidationOptions) (ValidationResult, error) {
	if !document.Is(Object) {
		return ValidationResult{}, &ValidationError{[]ValidationIssue{{"error", "openapi.root.type", "#", "OpenAPI 输入必须是 JSON object"}}}
	}
	if options.Profile != "" && options.Profile != ProfileApifoxHzThrift {
		return ValidationResult{}, fmt.Errorf("unsupported profile: %s", options.Profile)
	}
	c := &profileContext{document: document, issues: []ValidationIssue{}, operationIDs: map[string]bool{}, active: map[*Value]bool{}, budget: 100_000}
	c.schemas = document.Get("components").Get("schemas")
	c.securitySchemes = document.Get("components").Get("securitySchemes")
	c.root()
	c.authSchemes()
	c.security(document.Get("security"), "#/security")
	c.paths()
	c.components()
	c.extensions(document, "#", 0)
	if options.ValidateProjection == nil || *options.ValidateProjection {
		if _, err := ConvertDocument(document, ProjectionOptions{}); err != nil {
			pointer := "#"
			var projection *ProjectionError
			if errors.As(err, &projection) && projection.Pointer != "" {
				pointer = projection.Pointer
			}
			c.err("hz.thrift_projection", pointer, "OpenAPI 无法稳定投影为 Thrift IDL: "+err.Error())
		}
	}
	result := ValidationResult{Profile: ProfileApifoxHzThrift, Issues: c.issues, PathCount: len(document.Get("paths").Entries()), SchemaCount: len(c.schemas.Entries())}
	for _, issue := range result.Issues {
		if issue.Severity == "error" {
			result.ErrorCount++
		} else {
			result.WarningCount++
		}
	}
	for _, path := range document.Get("paths").Entries() {
		for _, method := range httpMethods {
			if path.Value.Get(method).Truth() {
				result.OperationCount++
			}
		}
	}
	return result, nil
}

type profileContext struct {
	document, schemas, securitySchemes *Value
	issues                             []ValidationIssue
	operationIDs                       map[string]bool
	active                             map[*Value]bool
	budget                             int
	limited                            bool
}

func (c *profileContext) issue(severity, code, pointer, message string) {
	if len(c.issues) < 4096 {
		c.issues = append(c.issues, ValidationIssue{severity, code, pointer, message})
	} else {
		c.limit(pointer)
	}
}
func (c *profileContext) err(code, pointer, message string) { c.issue("error", code, pointer, message) }
func (c *profileContext) warn(code, pointer, message string) {
	c.issue("warning", code, pointer, message)
}
func (c *profileContext) limit(pointer string) {
	if !c.limited {
		c.limited = true
		c.issues = append(c.issues, ValidationIssue{"error", "openapi.validation.limit", pointer, "OpenAPI validation traversal or issue limit exceeded"})
	}
}
func (c *profileContext) step(pointer string, depth int) bool {
	if c.limited || depth > 128 || c.budget <= 0 {
		c.limit(pointer)
		return false
	}
	c.budget--
	return true
}
func (c *profileContext) root() {
	version := c.document.Get("openapi").Str()
	if !strings.HasPrefix(version, "3.0") && !strings.HasPrefix(version, "3.1") {
		c.err("openapi.version", "#/openapi", "OpenAPI Render 当前只接受 OpenAPI 3.0/3.1 文档")
	}
	if paths := c.document.Get("paths"); !paths.Is(Object) || len(paths.Fields) == 0 {
		c.err("openapi.paths.missing", "#/paths", "paths 不能为空")
	}
	if !c.schemas.Is(Object) {
		c.warn("openapi.components.schemas_missing", "#/components/schemas", "建议使用 components.schemas 承载可复用数据模型，便于 APIFox 和 Thrift 共用引用")
	}
}
func (c *profileContext) authSchemes() {
	for _, entry := range c.securitySchemes.Entries() {
		scheme, pointer := entry.Value, "#/components/securitySchemes/"+escapePointer(entry.Name)
		if !scheme.Is(Object) {
			c.err("auth.scheme.type", pointer, "securityScheme 必须是 object")
			continue
		}
		switch scheme.Get("type").Str() {
		case "apiKey":
			if strings.TrimSpace(scheme.Get("name").Str()) == "" {
				c.err("auth.apikey.name", pointer+"/name", "apiKey securityScheme 必须声明 name")
			}
			if !oneOf(scheme.Get("in").Str(), "query", "header", "cookie") {
				c.err("auth.apikey.in", pointer+"/in", "apiKey securityScheme.in 只能是 query/header/cookie")
			}
		case "http":
			if strings.TrimSpace(scheme.Get("scheme").Str()) == "" {
				c.err("auth.http.scheme", pointer+"/scheme", "http securityScheme 必须声明 scheme，例如 bearer")
			}
		case "oauth2":
			if !scheme.Get("flows").Is(Object) {
				c.err("auth.oauth2.flows", pointer+"/flows", "oauth2 securityScheme 必须声明 flows")
			}
		case "openIdConnect":
			if strings.TrimSpace(scheme.Get("openIdConnectUrl").Str()) == "" {
				c.err("auth.openid.url", pointer+"/openIdConnectUrl", "openIdConnect securityScheme 必须声明 openIdConnectUrl")
			}
		default:
			c.err("auth.scheme.unsupported", pointer+"/type", "securityScheme.type 必须是 apiKey/http/oauth2/openIdConnect")
		}
	}
}
func (c *profileContext) security(requirements *Value, pointer string) {
	if requirements == nil {
		return
	}
	if !requirements.Is(Array) {
		c.err("auth.requirement.type", pointer, "security 必须是 requirement object 数组；公开接口请使用 []")
		return
	}
	for i, requirement := range requirements.Items {
		itemPointer := fmt.Sprintf("%s/%d", pointer, i)
		if !requirement.Is(Object) {
			c.err("auth.requirement.item_type", itemPointer, "security requirement 必须是 object")
			continue
		}
		for _, entry := range requirement.Fields {
			where := itemPointer + "/" + escapePointer(entry.Name)
			if !c.securitySchemes.Get(entry.Name).Truth() {
				c.err("auth.requirement.unresolved", where, "security requirement 引用了不存在的 securityScheme: "+entry.Name)
			}
			if !entry.Value.Is(Array) {
				c.err("auth.requirement.scopes", where, "security requirement 的 scopes 必须是 string[]")
			}
		}
	}
}
func (c *profileContext) paths() {
	for _, entry := range c.document.Get("paths").Entries() {
		item, pointer := entry.Value, "#/paths/"+escapePointer(entry.Name)
		if !item.Is(Object) {
			c.err("path_item.type", pointer, "path item 必须是 object")
			continue
		}
		c.parameters(item.Get("parameters"), pointer+"/parameters", "path")
		for _, method := range httpMethods {
			if op := item.Get(method); op.Truth() {
				c.operation(entry.Name, method, op, pointer+"/"+method, item)
			}
		}
	}
}
func (c *profileContext) operation(path, method string, op *Value, pointer string, pathItem *Value) {
	if !op.Is(Object) {
		c.err("operation.type", pointer, "operation 必须是 object")
		return
	}
	label, id := strings.ToUpper(method)+" "+path, op.Get("operationId").Str()
	if id == "" {
		c.err("operation.operation_id.missing", pointer+"/operationId", label+" 必须声明稳定 operationId")
	} else if c.operationIDs[id] {
		c.err("operation.operation_id.duplicate", pointer+"/operationId", "operationId 重复: "+id)
	} else {
		c.operationIDs[id] = true
	}
	if tags := op.Get("tags"); !tags.Is(Array) || len(tags.Items) == 0 {
		c.warn("operation.tags.missing", pointer+"/tags", label+" 建议声明 tags，APIFox 会用它辅助目录和过滤")
	}
	c.security(op.Get("security"), pointer+"/security")
	parameters := &Value{Kind: Array}
	for _, owner := range []*Value{pathItem, op} {
		param := owner.Get("parameters")
		if param != nil && param.Kind != Null && !param.Is(Array) {
			c.err("parameter.list.type", pointer+"/parameters", "parameters 必须是数组")
			continue
		}
		parameters.Items = append(parameters.Items, param.List()...)
	}
	c.parameters(parameters, pointer+"/parameters", "operation")
	c.requestBody(method, op.Get("requestBody"), pointer+"/requestBody")
	c.responses(op.Get("responses"), pointer+"/responses")
}
func (c *profileContext) parameters(parameters *Value, pointer, owner string) {
	if parameters != nil && parameters.Kind != Null && !parameters.Is(Array) {
		c.err("parameter.list.type", pointer, "parameters 必须是数组")
		return
	}
	for i, param := range parameters.List() {
		where := fmt.Sprintf("%s/%d", pointer, i)
		if isRef(param) {
			param = c.componentRef(param, "parameters", "parameter", "parameter", where, false)
			if param == nil {
				continue
			}
			c.err("hz.parameter_ref.unsupported", where, "Hz/Thrift 生成不支持 parameter $ref；中央契约应在进入投影前写成内联 parameter")
		}
		c.parameter(param, where, owner)
	}
}
func (c *profileContext) parameter(param *Value, pointer, owner string) {
	if !param.Is(Object) {
		c.err("parameter.type", pointer, "parameter 必须是 object")
		return
	}
	in := param.Get("in").Str()
	if !oneOf(in, "path", "query", "header", "cookie") {
		c.err("parameter.location", pointer+"/in", "parameter.in 必须是 path/query/header/cookie")
	}
	if strings.TrimSpace(param.Get("name").Str()) == "" {
		c.err("parameter.name", pointer+"/name", "parameter 必须声明非空 name")
	}
	if in == "path" && !isBool(param.Get("required"), true) {
		c.err("parameter.path.required", pointer+"/required", "path parameter 必须 required=true")
	}
	if len(param.Get("content").Entries()) > 0 {
		c.err("hz.parameter_content.unsupported", pointer+"/content", "Hz/Thrift profile 不支持 parameter.content，请改用 schema 参数")
	}
	schema := param.Get("schema")
	if !schema.Truth() {
		c.err("parameter.schema.missing", pointer+"/schema", "parameter 必须声明 schema")
		return
	}
	c.schema(schema, pointer+"/schema", 0)
	c.parameterSerialization(param, pointer+"/schema")
	hasExample := param.Has("example") || (param.Get("examples").Is(Object) && len(param.Get("examples").Fields) > 0) || c.schemaHasExample(schema, map[string]bool{}, 0)
	if !hasExample && owner == "operation" && in != "header" {
		c.warn("apifox.parameter.example_missing", pointer, "APIFox 参数建议提供 example/examples；query/path-only 接口尤其需要")
	}
	if param.Has("example") {
		c.example(schema, param.Get("example"), pointer+"/example", 0)
	}
}
func (c *profileContext) parameterSerialization(param *Value, pointer string) {
	if param.Get("in").Str() != "query" {
		return
	}
	schema := c.resolveSchema(param.Get("schema"), pointer)
	if schema == nil {
		return
	}
	style := param.Get("style")
	if style == nil || style.Kind == Null {
		style = str("form")
	}
	explode := param.Get("explode")
	if explode == nil || explode.Kind == Null {
		explode = boolean(style.Str() == "form")
	}
	if style.Str() == "deepObject" {
		c.err("hz.query.deep_object", pointer, "不支持 deepObject query 参数；请拆成简单字段")
	}
	if oneOf(style.Str(), "spaceDelimited", "pipeDelimited") {
		c.err("hz.query.delimited", pointer, "不支持 "+style.Str()+" query 参数；数组只能使用 form + explode=true")
	}
	if objectSchema(schema) {
		c.err("hz.query.object", pointer, "不支持 object query 参数自动投影；请拆成简单字段")
	}
	if schema.Get("type").Str() == "array" && (style.Str() != "form" || !explode.Truth()) {
		c.err("hz.query.array_serialization", pointer, "query array 只能使用 form + explode=true（重复 key）语义")
	}
}
func (c *profileContext) requestBody(method string, input *Value, pointer string) {
	if !input.Truth() {
		return
	}
	if noBodyMethods[method] {
		c.err("hz.request_body.method", pointer, strings.ToUpper(method)+" 不允许声明 requestBody")
	}
	body := c.componentRef(input, "requestBodies", "request_body", "requestBody", pointer, false)
	if body == nil {
		return
	}
	c.mediaContent(body.Get("content"), pointer, "request")
	if !c.hasMediaExample(body.Get("content")) {
		c.warn("apifox.request_body.example_missing", pointer, "APIFox requestBody 建议提供 media example/examples 或 schema 字段 example")
	}
}
func (c *profileContext) responses(responses *Value, pointer string) {
	if !responses.Is(Object) {
		c.err("operation.responses.missing", pointer, "operation 必须声明 responses")
		return
	}
	count := 0
	for _, entry := range responses.Fields {
		if successCode.MatchString(entry.Name) {
			count++
		}
	}
	if count == 0 {
		c.err("hz.response.success_missing", pointer, "Hz/Thrift profile 要求每个 operation 有且只有一个 2xx success response")
	}
	if count > 1 {
		c.err("hz.response.multiple_success", pointer, "Hz/Thrift profile 不支持多个 2xx success response")
	}
	for _, entry := range responses.Fields {
		where := pointer + "/" + entry.Name
		response := c.componentRef(entry.Value, "responses", "response", "response", where, false)
		if response != nil {
			owner := "error_response"
			if successCode.MatchString(entry.Name) {
				owner = "success_response"
			}
			c.mediaContent(response.Get("content"), where, owner)
		}
	}
}
func (c *profileContext) mediaContent(content *Value, pointer, owner string) {
	if len(content.Entries()) > 1 {
		c.err("hz.media.multiple_content_types", pointer+"/content", "Hz/Thrift profile 不支持同一 request/response 的多个 content-type 主线")
	}
	for _, entry := range content.Entries() {
		media, where := entry.Value, pointer+"/content/"+escapePointer(entry.Name)
		normalized := strings.ToLower(strings.TrimSpace(entry.Name))
		if owner == "request" && normalized == "json" {
			c.err("apifox.request_body.json_alias", where, "APIFox 稳定导出必须使用 application/json，不要使用 json 简写 content-type")
		}
		if owner == "request" && !profileJSONContent(normalized) && !oneOf(normalized, "multipart/form-data", "application/x-www-form-urlencoded") {
			c.err("hz.request_body.content_type", where, "请求体只支持 application/json、multipart/form-data、application/x-www-form-urlencoded")
		}
		if schema := media.Get("schema"); schema.Truth() {
			c.schema(schema, where+"/schema", 0)
			if owner == "success_response" && !profileJSONContent(normalized) {
				resolved := c.resolveSchema(schema, where)
				if resolved != nil && (objectSchema(resolved) || resolved.Get("type").Str() == "array") {
					c.err("hz.response.raw_schema", where+"/schema", "非 JSON success response 只支持 string/binary 标量或省略 schema")
				}
			}
		}
		c.mediaExamples(media, where)
	}
}
func (c *profileContext) mediaExamples(media *Value, pointer string) {
	schema := media.Get("schema")
	if !schema.Truth() {
		return
	}
	if media.Has("example") {
		c.example(schema, media.Get("example"), pointer+"/example", 0)
	}
	for _, entry := range media.Get("examples").Entries() {
		example, where := entry.Value, pointer+"/examples/"+escapePointer(entry.Name)
		if isRef(example) {
			c.localRef(example.Get("$ref").Str(), where)
			continue
		}
		if !example.Is(Object) {
			c.err("apifox.example.type", where, "media examples 的每个条目必须是 Example Object 或 $ref")
			continue
		}
		if example.Has("value") {
			c.example(schema, example.Get("value"), where+"/value", 0)
		} else if !example.Get("externalValue").Is(String) {
			c.warn("apifox.example.value_missing", where, "media example 建议提供 value；externalValue 只适合真实外链样例")
		}
	}
}
func (c *profileContext) schema(input *Value, pointer string, depth int) {
	if !c.step(pointer, depth) {
		return
	}
	schema := c.resolveSchema(input, pointer)
	if schema == nil {
		return
	}
	if !schema.Is(Object) {
		c.err("hz.schema.type", pointer, "schema 必须是 object")
		return
	}
	// Recursive object models are valid. A back edge has already been checked at
	// its first occurrence; aliases without an object are rejected by resolution.
	if c.active[schema] {
		return
	}
	c.active[schema] = true
	defer delete(c.active, schema)
	c.schemaExamples(schema, pointer)
	c.schemaMocks(schema, pointer)
	c.unsupported(schema, pointer)
	additional := schema.Get("additionalProperties")
	if isBool(additional, true) {
		c.err("hz.schema.additional_properties_true", pointer+"/additionalProperties", "Hz/Thrift profile 不支持 additionalProperties: true")
	}
	if isBool(additional, false) {
		c.err("hz.schema.additional_properties_false", pointer+"/additionalProperties", "additionalProperties:false 需要 binder/decoder 级语义，不能投影到字段 validator")
	}
	if additional != nil && (additional.Is(Object) || additional.Is(Null) || additional.Is(Array)) && len(schema.Get("properties").Entries()) > 0 {
		c.err("hz.schema.properties_plus_map", pointer, "不支持 properties 与 additionalProperties schema 混用的 object")
	}
	for _, value := range schema.Get("enum").List() {
		if value.Is(String) && whiteSpace.MatchString(value.Text) {
			c.err("hz.schema.enum_whitespace", pointer+"/enum", "string enum 值不能包含空白字符，Hertz validator oneof 无法稳定表达")
			break
		}
	}
	for _, property := range schema.Get("properties").Entries() {
		c.schema(property.Value, pointer+"/properties/"+escapePointer(property.Name), depth+1)
	}
	if schema.Get("items").Truth() {
		c.schema(schema.Get("items"), pointer+"/items", depth+1)
	}
	if additional != nil && (additional.Is(Object) || additional.Is(Null) || additional.Is(Array)) {
		c.schema(additional, pointer+"/additionalProperties", depth+1)
	}
	for _, keyword := range []string{"anyOf", "oneOf", "allOf"} {
		for i, branch := range schema.Get(keyword).List() {
			c.schema(branch, fmt.Sprintf("%s/%s/%d", pointer, keyword, i), depth+1)
		}
	}
}
func (c *profileContext) schemaExamples(schema *Value, pointer string) {
	if schema.Has("example") {
		c.example(schema, schema.Get("example"), pointer+"/example", 0)
	}
	if examples := schema.Get("examples"); examples != nil {
		if !examples.Is(Array) {
			c.err("apifox.schema.examples_type", pointer+"/examples", "schema.examples 必须是数组；media.examples 才是命名 Example Object map")
			return
		}
		for i, example := range examples.Items {
			c.example(schema, example, fmt.Sprintf("%s/examples/%d", pointer, i), 0)
		}
	}
}
func (c *profileContext) schemaMocks(schema *Value, pointer string) {
	if schema.Has("x-apifox-mock") && strings.TrimSpace(schema.Get("x-apifox-mock").Str()) == "" {
		c.err("apifox.mock.expression", pointer+"/x-apifox-mock", "x-apifox-mock 必须是非空字符串表达式，例如 @id、@date、@pick(...)")
	}
	if schema.Has("mockScript") {
		if strings.TrimSpace(schema.Get("mockScript").Str()) == "" {
			c.err("apifox.mock_script.type", pointer+"/mockScript", "mockScript 必须是非空字符串；只在需要条件分支 mock 时使用")
		} else {
			c.warn("apifox.mock_script.review", pointer+"/mockScript", "mockScript 应保持确定性；优先使用字段级 x-apifox-mock")
		}
	}
}
func (c *profileContext) unsupported(schema *Value, pointer string) {
	canonicalOverride, legacyOverride := schema.Get("x-ispark-allow-unsupported-validation"), schema.Get("x-dramawork-allow-unsupported-validation")
	canonicalValidators, legacyValidators := schema.Get("x-ispark-validate"), schema.Get("x-dramawork-validate")
	if legacyOverride != nil {
		c.warn("hz.schema.legacy_manual_override", pointer+"/x-dramawork-allow-unsupported-validation", "x-dramawork-allow-unsupported-validation 已弃用，请迁移到 x-ispark-allow-unsupported-validation")
	}
	if legacyValidators != nil {
		c.warn("hz.schema.legacy_manual_validator", pointer+"/x-dramawork-validate", "x-dramawork-validate 已弃用，请迁移到 x-ispark-validate")
	}
	if canonicalOverride != nil && legacyOverride != nil && !equalValue(canonicalOverride, legacyOverride) {
		c.err("hz.schema.manual_override_conflict", pointer+"/x-ispark-allow-unsupported-validation", "x-ispark-allow-unsupported-validation 与已弃用字段必须保持一致")
	}
	if canonicalValidators != nil && legacyValidators != nil && !equalValue(canonicalValidators, legacyValidators) {
		c.err("hz.schema.manual_validator_conflict", pointer+"/x-ispark-validate", "x-ispark-validate 与已弃用字段必须保持一致")
	}
	override := coalesce(canonicalOverride, legacyOverride).Truth()
	validators := coalesce(canonicalValidators, legacyValidators)
	if override && !hasManualValidator(validators) {
		c.err("hz.schema.manual_validator_missing", pointer+"/x-ispark-validate", "x-ispark-allow-unsupported-validation 需要同时提供 x-ispark-validate")
	}
	if format := schema.Get("format"); format.Truth() && !supportedFormat(schema) && !override {
		c.err("hz.schema.format", pointer+"/format", "不支持 format="+valueText(format)+" 自动投影，请收紧 schema 或显式 x-ispark-validate")
	}
	if len(schema.Get("oneOf").List()) > 0 {
		c.err("hz.schema.one_of", pointer+"/oneOf", "不支持 oneOf")
	}
	if len(schema.Get("anyOf").List()) > 0 && !nullableUnion(schema) {
		c.err("hz.schema.any_of", pointer+"/anyOf", "只支持 nullable anyOf [T, null]")
	}
	if branches := schema.Get("allOf").List(); len(branches) > 0 {
		composable := true
		for i, branch := range branches {
			resolved := c.resolveSchema(branch, fmt.Sprintf("%s/allOf/%d", pointer, i))
			if resolved == nil || !objectSchema(resolved) {
				composable = false
				break
			}
		}
		if !composable {
			c.err("hz.schema.all_of", pointer+"/allOf", "只支持可组合 object schema 的 allOf")
		}
	}
	if schema.Get("pattern").Truth() && !override {
		c.err("hz.schema.pattern", pointer+"/pattern", "pattern 不能自动投影；需要显式 x-ispark-validate")
	}
	if schema.Get("multipleOf").Is(Number) && !override {
		c.err("hz.schema.multiple_of", pointer+"/multipleOf", "multipleOf 不能自动投影；需要显式 x-ispark-validate")
	}
	for _, keyword := range []string{"exclusiveMinimum", "exclusiveMaximum", "uniqueItems", "minProperties", "maxProperties"} {
		if schema.Has(keyword) && !override {
			c.err("hz.schema.unsupported_validator", pointer+"/"+keyword, keyword+" 不能自动投影；需要显式 x-ispark-validate")
		}
	}
}
func (c *profileContext) components() {
	for _, entry := range c.schemas.Entries() {
		c.schema(entry.Value, "#/components/schemas/"+escapePointer(entry.Name), 0)
	}
	for _, entry := range c.document.Get("components").Get("requestBodies").Entries() {
		c.requestBody("post", entry.Value, "#/components/requestBodies/"+escapePointer(entry.Name))
	}
	for _, entry := range c.document.Get("components").Get("responses").Entries() {
		c.mediaContent(entry.Value.Get("content"), "#/components/responses/"+escapePointer(entry.Name), "success_response")
	}
	for _, entry := range c.document.Get("components").Get("parameters").Entries() {
		c.parameter(entry.Value, "#/components/parameters/"+escapePointer(entry.Name), "path")
	}
}
func (c *profileContext) example(input, value *Value, pointer string, depth int) {
	if !c.step(pointer, depth) {
		return
	}
	schema := c.resolveSchema(input, pointer)
	if schema == nil || value.Is(Null) {
		return
	}
	if nullableUnion(schema) {
		for _, branch := range schema.Get("anyOf").List() {
			if isRef(branch) || branch.Get("type").Str() != "null" {
				c.example(branch, value, pointer, depth+1)
				break
			}
		}
		return
	}
	expected := schema.Get("type").Str()
	if objectSchema(schema) {
		expected = "object"
	}
	matched := true
	switch expected {
	case "object":
		matched = value.Is(Object)
	case "array":
		matched = value.Is(Array)
	case "string":
		matched = value.Is(String)
	case "integer":
		matched = value.Is(Number) && math.Trunc(value.Number) == value.Number
	case "number":
		matched = value.Is(Number)
	case "boolean":
		matched = value.Is(Boolean)
	}
	if !matched {
		c.err("apifox.example.schema_mismatch", pointer, "example 类型与 schema 不匹配，期望 "+expected)
	}
}
func (c *profileContext) hasMediaExample(content *Value) bool {
	for _, entry := range content.Entries() {
		media := entry.Value
		if media.Has("example") || len(media.Get("examples").Entries()) > 0 || (media.Get("schema").Truth() && c.schemaHasExample(media.Get("schema"), map[string]bool{}, 0)) {
			return true
		}
	}
	return false
}
func (c *profileContext) schemaHasExample(schema *Value, seen map[string]bool, depth int) bool {
	if !c.step("#", depth) {
		return false
	}
	if isRef(schema) {
		ref := schema.Get("$ref").Str()
		match := schemaRef.FindStringSubmatch(ref)
		if seen[ref] || match == nil || !c.schemas.Get(match[1]).Truth() {
			return false
		}
		seen[ref] = true
		return c.schemaHasExample(c.schemas.Get(match[1]), seen, depth+1)
	}
	if schema.Has("example") || len(schema.Get("examples").List()) > 0 {
		return true
	}
	for _, property := range schema.Get("properties").Entries() {
		if c.schemaHasExample(property.Value, seen, depth+1) {
			return true
		}
	}
	return schema.Get("items").Truth() && c.schemaHasExample(schema.Get("items"), seen, depth+1)
}
func (c *profileContext) resolveSchema(schema *Value, pointer string) *Value {
	return c.componentRef(schema, "schemas", "schema", "schema", pointer, true)
}
func (c *profileContext) componentRef(value *Value, group, code, label, pointer string, recursive bool) *Value {
	seen := map[string]bool{}
	for isRef(value) {
		ref := value.Get("$ref").Str()
		prefix := "#/components/" + group + "/"
		if !strings.HasPrefix(ref, prefix) || len(ref) == len(prefix) || strings.Contains(ref[len(prefix):], "/") {
			c.err("openapi.ref."+code+"_scope", pointer, label+" $ref 只允许指向 "+prefix+"*")
			return nil
		}
		if seen[ref] {
			c.err("openapi.ref.cycle", pointer, "不支持循环别名引用 "+ref)
			return nil
		}
		if len(seen) >= 128 {
			c.limit(pointer)
			return nil
		}
		seen[ref] = true
		value = c.document.Get("components").Get(group).Get(ref[len(prefix):])
		if !value.Truth() {
			c.err("openapi.ref.unresolved", pointer, "找不到 "+label+" 引用 "+ref)
			return nil
		}
		if !recursive {
			break
		}
	}
	return value
}
func (c *profileContext) localRef(ref, pointer string) {
	if !strings.HasPrefix(ref, "#/") {
		c.err("openapi.ref.external", pointer, "只允许本地 OpenAPI $ref")
		return
	}
	value := c.document
	for _, part := range strings.Split(ref[2:], "/") {
		part = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
		if !value.Is(Object) || !value.Has(part) {
			c.err("openapi.ref.unresolved", pointer, "找不到 OpenAPI 引用 "+ref)
			return
		}
		value = value.Get(part)
	}
}
func (c *profileContext) extensions(value *Value, pointer string, depth int) {
	if !c.step(pointer, depth) {
		return
	}
	if value.Is(Array) {
		for i, item := range value.Items {
			c.extensions(item, fmt.Sprintf("%s/%d", pointer, i), depth+1)
		}
		return
	}
	if !value.Is(Object) {
		return
	}
	if value.Has("x-apifox-mock") && strings.TrimSpace(value.Get("x-apifox-mock").Str()) == "" {
		c.err("apifox.mock.expression", pointer+"/x-apifox-mock", "x-apifox-mock 必须是非空字符串表达式")
	}
	if value.Has("mockScript") && strings.TrimSpace(value.Get("mockScript").Str()) == "" {
		c.err("apifox.mock_script.type", pointer+"/mockScript", "mockScript 必须是非空字符串")
	}
	for _, entry := range value.Fields {
		c.extensions(entry.Value, pointer+"/"+escapePointer(entry.Name), depth+1)
	}
}
func escapePointer(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(value, "~", "~0"), "/", "~1")
}
func oneOf(value string, choices ...string) bool {
	for _, choice := range choices {
		if value == choice {
			return true
		}
	}
	return false
}
func isBool(value *Value, expected bool) bool { return value.Is(Boolean) && value.Bool == expected }
func coalesce(first, second *Value) *Value {
	if first != nil && first.Kind != Null {
		return first
	}
	return second
}
func hasManualValidator(value *Value) bool {
	if value.Is(String) {
		return strings.TrimSpace(value.Text) != ""
	}
	for _, item := range value.List() {
		if strings.TrimSpace(item.Str()) != "" {
			return true
		}
	}
	return false
}

// The public profile deliberately rejects the projector's historical "json"
// convenience alias. Stable APIFox exports must use a real media type.
func profileJSONContent(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return value == "application/json" || strings.HasPrefix(value, "application/json;")
}
