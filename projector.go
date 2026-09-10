package openapithrift

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var httpMethods = []string{"get", "post", "put", "delete", "patch", "head", "options"}
var noBodyMethods = map[string]bool{"get": true, "delete": true, "head": true, "options": true}
var successCode = regexp.MustCompile(`^2\d\d$`)
var whiteSpace = regexp.MustCompile(`\s`)
var schemaRef = regexp.MustCompile(`^#/components/schemas/([^/]+)$`)
var bodyRef = regexp.MustCompile(`^#/components/requestBodies/([^/]+)$`)
var formatValidators = map[string]string{"base64": "base64", "date": "datetime=2006-01-02", "date-time": "datetime=2006-01-02T15:04:05Z07:00", "email": "email", "e164": "e164", "hexcolor": "hexcolor", "hostname": "hostname_rfc1123", "ipv4": "ipv4", "ipv6": "ipv6", "json": "json", "jwt": "jwt", "uri": "uri", "url": "url", "ulid": "ulid", "uuid": "uuid", "uuid3": "uuid3", "uuid4": "uuid4", "uuid5": "uuid5"}

type projector struct {
	schemas, bodies  *Value
	definitions      map[string]ThriftStruct
	order            []string
	inProgress, used map[string]bool
}

func Convert(input []byte, options ProjectionOptions) (ProjectionResult, error) {
	document, err := Parse(input)
	if err != nil {
		return ProjectionResult{}, err
	}
	return ConvertDocument(document, options)
}
func ConvertDocument(document *Value, options ProjectionOptions) (ProjectionResult, error) {
	if err := checkDocument(document); err != nil {
		return ProjectionResult{}, err
	}
	result, err := Project(document, options)
	if err != nil {
		return ProjectionResult{}, err
	}
	return ProjectionResult{Document: result, Thrift: RenderThrift(result)}, nil
}

func ParseOpenAPIDocument(input []byte) (*Value, error) {
	document, err := Parse(input)
	if err != nil {
		return nil, projectionError(err.Error(), "")
	}
	if err := checkDocument(document); err != nil {
		return nil, err
	}
	return document, nil
}

func checkDocument(document *Value) error {
	if !document.Is(Object) {
		return projectionError("OpenAPI 输入必须是 JSON object", "")
	}
	version := document.Get("openapi").Str()
	if !strings.HasPrefix(version, "3.0") && !strings.HasPrefix(version, "3.1") {
		return projectionError("当前只支持 OpenAPI 3.x JSON 文档", "")
	}
	return nil
}

func Project(document *Value, options ProjectionOptions) (ThriftDocument, error) {
	p := &projector{schemas: document.Get("components").Get("schemas"), bodies: document.Get("components").Get("requestBodies"), definitions: map[string]ThriftStruct{}, inProgress: map[string]bool{}, used: map[string]bool{}}
	service := options.ServiceName
	if service == "" {
		service = pascal(document.Get("info").Get("title").Str())
		if service == "" {
			service = "OpenApi"
		}
		if !strings.HasSuffix(service, "Service") {
			service += "Service"
		}
	}
	namespace := options.Namespace
	if namespace == "" {
		namespace = "ispark.openapi"
	}
	result := ThriftDocument{Namespace: namespace, ServiceName: service, Definitions: []ThriftStruct{}, Methods: []ThriftMethod{}}
	paths := document.Get("paths")
	names := []string{}
	for _, field := range paths.Entries() {
		names = append(names, field.Name)
	}
	sort.Strings(names)
	for _, path := range names {
		item := paths.Get(path)
		for _, method := range httpMethods {
			op := item.Get(method)
			if !op.Truth() {
				continue
			}
			name := pascal(op.Get("operationId").Str())
			if name == "" {
				name = pascal(options.RouteMethodNames[BuildRouteKey(method, path)])
			}
			if name == "" {
				name = fallbackMethod(method, path)
			}
			request, err := p.request(item, op, method, path, name)
			if err != nil {
				return result, err
			}
			response, err := p.response(op, name)
			if err != nil {
				return result, err
			}
			result.Methods = append(result.Methods, ThriftMethod{Name: name, RequestType: request, ResponseType: response, HTTPMethod: method, Path: NormalizeRoutePath(path), Comment: comments(op.Get("summary").Str(), op.Get("description").Str())})
		}
	}
	if len(result.Methods) == 0 {
		return result, projectionError("OpenAPI 文档里没有可转换的 paths/methods", "")
	}
	for _, name := range p.order {
		result.Definitions = append(result.Definitions, p.definitions[name])
	}
	return result, nil
}

func (p *projector) allocate(raw string) (string, error) {
	base := pascal(raw)
	if base == "" {
		return "", projectionError("无法生成合法的类型名", "")
	}
	if base[0] >= '0' && base[0] <= '9' {
		base = "Type" + base
	}
	name := base
	for suffix := 2; p.used[name]; suffix++ {
		name = base + strconv.Itoa(suffix)
	}
	p.used[name] = true
	return name, nil
}
func allocateField(used map[string]bool, raw string) (string, error) {
	base := snake(raw)
	if base == "" {
		return "", projectionError("无法生成合法的字段名", "")
	}
	if base[0] >= '0' && base[0] <= '9' {
		base = "field_" + base
	}
	name := base
	for suffix := 2; used[name]; suffix++ {
		name = base + "_" + strconv.Itoa(suffix)
	}
	used[name] = true
	return name, nil
}
func (p *projector) register(def ThriftStruct) {
	if _, ok := p.definitions[def.Name]; !ok {
		p.definitions[def.Name] = def
		p.order = append(p.order, def.Name)
	}
}

func (p *projector) request(item, op *Value, method, path, methodName string) (string, error) {
	name, err := p.allocate(methodName + "Request")
	if err != nil {
		return "", err
	}
	fields := []ThriftField{}
	used := map[string]bool{}
	parameters := []*Value{}
	positions := map[string]int{}
	for _, list := range [][]*Value{item.Get("parameters").List(), op.Get("parameters").List()} {
		for _, param := range list {
			key := param.Get("$ref").Str()
			if !isRef(param) {
				key = param.Get("in").Str() + ":" + param.Get("name").Str()
			}
			if i, ok := positions[key]; ok {
				parameters[i] = param
			} else {
				positions[key] = len(parameters)
				parameters = append(parameters, param)
			}
		}
	}
	for i, param := range parameters {
		if isRef(param) {
			return "", projectionError("第一版暂不支持 parameter $ref，请在导出前展开参数", fmt.Sprintf("%s#parameters[%d]", path, i))
		}
		location, paramName := param.Get("in").Str(), param.Get("name").Str()
		pointer := path + "." + location + "." + paramName
		if len(param.Get("content").Entries()) > 0 {
			return "", projectionError("当前不支持 parameter content，请改为 schema 参数或手写 transport", pointer)
		}
		schema := param.Get("schema")
		if schema == nil {
			return "", projectionError("参数缺少 schema", path+"."+paramName)
		}
		if err := p.checkParameter(param, pointer); err != nil {
			return "", err
		}
		typ, err := p.schemaType(schema, name+pascal(paramName), pointer, 0)
		if err != nil {
			return "", err
		}
		fieldName, err := allocateField(used, paramName)
		if err != nil {
			return "", err
		}
		requiredness := "optional"
		if location == "path" {
			requiredness = "required"
		}
		tags, err := goTags(schema, param.Get("required").Truth(), location != "path", requiredness, pointer)
		if err != nil {
			return "", err
		}
		fields = append(fields, ThriftField{ID: len(fields) + 1, Requiredness: requiredness, Type: typ, Name: fieldName, Annotations: append([]string{fmt.Sprintf("api.%s=\"%s\"", location, paramName)}, tags...), Comment: comments(param.Get("description").Str())})
	}
	if body := op.Get("requestBody"); body.Truth() {
		if noBodyMethods[method] {
			return "", projectionError(strings.ToUpper(method)+" 请求不允许 requestBody", path)
		}
		extra, err := p.requestBody(body, name, path+"."+method+".requestBody", len(fields)+1, used)
		if err != nil {
			return "", err
		}
		fields = append(fields, extra...)
	}
	p.register(ThriftStruct{Name: name, Comment: comments(op.Get("summary").Str(), op.Get("description").Str()), Fields: fields})
	return name, nil
}

func (p *projector) checkParameter(param *Value, pointer string) error {
	if param.Get("in").Str() != "query" || param.Get("schema") == nil {
		return nil
	}
	schema, _, err := p.resolve(param.Get("schema"), pointer, 0)
	if err != nil {
		return err
	}
	style := param.Get("style").Str()
	if !param.Has("style") {
		style = "form"
	}
	explode := style == "form"
	if param.Has("explode") {
		explode = param.Get("explode").Truth()
	}
	if style == "deepObject" {
		return projectionError("当前不支持 deepObject query 参数，请改为简单字段或手写 transport", pointer)
	}
	if style == "spaceDelimited" || style == "pipeDelimited" {
		return projectionError("当前不支持 "+style+" query 参数序列化，请改为默认 form/explode 语义", pointer)
	}
	if objectSchema(schema) {
		return projectionError("当前不支持 object query 参数自动投影，请改为简单字段或手写 transport", pointer)
	}
	if schema.Get("type").Str() == "array" && (style != "form" || !explode) {
		return projectionError("当前只支持 query array 参数的 form + explode=true（重复 key）序列化", pointer)
	}
	return nil
}

func (p *projector) requestBody(body *Value, name, pointer string, start int, used map[string]bool) ([]ThriftField, error) {
	var err error
	for depth := 0; isRef(body); depth++ {
		if depth > 128 {
			return nil, projectionError("requestBody 引用形成循环或过深", pointer)
		}
		ref := body.Get("$ref").Str()
		match := bodyRef.FindStringSubmatch(ref)
		if match == nil {
			return nil, projectionError("当前只支持 #/components/requestBodies/* 本地引用", pointer)
		}
		body = p.bodies.Get(match[1])
		if body == nil {
			return nil, projectionError("找不到 requestBody 引用 "+ref, pointer)
		}
	}
	contentType, media, err := singleContent(body.Get("content"), pointer)
	if err != nil {
		return nil, err
	}
	if media == nil {
		return []ThriftField{}, nil
	}
	normalized := strings.ToLower(strings.TrimSpace(contentType))
	annotation := "api.body"
	if normalized == "multipart/form-data" || normalized == "application/x-www-form-urlencoded" {
		annotation = "api.form"
		contentType = normalized
	} else if normalized != "json" && normalized != "application/json" {
		return nil, projectionError("当前只支持 application/json、multipart/form-data、application/x-www-form-urlencoded 请求体", pointer)
	}
	if media.Get("schema") == nil {
		return []ThriftField{}, nil
	}
	schema, _, err := p.resolve(media.Get("schema"), pointer, 0)
	if err != nil {
		return nil, err
	}
	if err = assertSupported(schema, pointer); err != nil {
		return nil, err
	}
	if !objectSchema(schema) {
		if !isJSONContent(contentType) {
			return nil, projectionError(contentType+" 请求体顶层必须是 object schema", pointer)
		}
		typ, err := p.schemaType(media.Get("schema"), name+"Body", pointer, 0)
		if err != nil {
			return nil, err
		}
		fieldName, err := allocateField(used, "body")
		if err != nil {
			return nil, err
		}
		tags, err := goTags(media.Get("schema"), body.Get("required").Truth(), true, "optional", pointer)
		if err != nil {
			return nil, err
		}
		return []ThriftField{{ID: start, Requiredness: "optional", Type: typ, Name: fieldName, Annotations: append([]string{`api.raw_body=""`}, tags...), Comment: schemaComment(media.Get("schema"))}}, nil
	}
	if schema.Get("additionalProperties").Truth() {
		return nil, projectionError("requestBody 顶层不支持 additionalProperties", pointer)
	}
	fields := []ThriftField{}
	required := stringSet(schema.Get("required"))
	for _, field := range schema.Get("properties").Entries() {
		fieldPointer := pointer + "." + field.Name
		if annotation == "api.form" {
			binary, err := containsBinary(field.Value, fieldPointer, 0)
			if err != nil {
				return nil, err
			}
			if binary {
				return nil, projectionError("当前不支持 form/multipart 文件字段（单文件或多文件）自动投影，请改为手写上传接口", fieldPointer)
			}
		}
		typ, err := p.schemaType(field.Value, name+pascal(field.Name), fieldPointer, 0)
		if err != nil {
			return nil, err
		}
		fieldName, err := allocateField(used, field.Name)
		if err != nil {
			return nil, err
		}
		tags, err := goTags(field.Value, required[field.Name], true, "optional", fieldPointer)
		if err != nil {
			return nil, err
		}
		fields = append(fields, ThriftField{ID: start + len(fields), Requiredness: "optional", Type: typ, Name: fieldName, Annotations: append([]string{fmt.Sprintf("%s=\"%s\"", annotation, field.Name)}, tags...), Comment: schemaComment(field.Value)})
	}
	return fields, nil
}

func (p *projector) response(op *Value, methodName string) (string, error) {
	responses := op.Get("responses")
	codes := []string{}
	for _, field := range responses.Entries() {
		if successCode.MatchString(field.Name) {
			codes = append(codes, field.Name)
		}
	}
	sort.Strings(codes)
	if len(codes) == 0 {
		return "", projectionError("operation 缺少 2xx success response", methodName)
	}
	if len(codes) > 1 {
		return "", projectionError("第一版暂不支持多个 2xx success response，请先在 OpenAPI profile 中收敛", methodName)
	}
	contentType, media, err := singleContent(responses.Get(codes[0]).Get("content"), methodName+".responses")
	if err != nil {
		return "", err
	}
	if media == nil || (isJSONContent(contentType) && media.Get("schema") == nil) {
		p.register(ThriftStruct{Name: "EmptyResponse", Comment: []string{"HTTP 2xx with no JSON response body."}, Fields: []ThriftField{}})
		return "EmptyResponse", nil
	}
	if isJSONContent(contentType) {
		return p.schemaType(media.Get("schema"), methodName+"Response", methodName+".responses.2xx", 0)
	}
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	name, err := p.allocate(methodName + "RawBodyResponse")
	if err != nil {
		return "", err
	}
	typ := "binary"
	if strings.HasPrefix(contentType, "text/") {
		typ = "string"
	}
	if media.Has("schema") {
		pointer := methodName + ".responses.2xx"
		schema, _, err := p.resolve(media.Get("schema"), pointer, 0)
		if err != nil {
			return "", err
		}
		if err := assertSupported(schema, pointer); err != nil {
			return "", err
		}
		if objectSchema(schema) || (schema.Get("type").Truth() && schema.Get("type").Str() != "string") {
			return "", projectionError("非 JSON success response 只支持 string/binary 标量或省略 schema", pointer)
		}
		typ, err = scalarType(schema, pointer)
		if err != nil {
			return "", err
		}
	}
	p.register(ThriftStruct{Name: name, Comment: []string{"HTTP raw success body. content-type=" + contentType + "."}, Fields: []ThriftField{{ID: 1, Requiredness: "optional", Type: typ, Name: "body", Annotations: []string{`api.raw_body=""`}}}})
	return name, nil
}

func (p *projector) schemaType(input *Value, suggested, pointer string, depth int) (string, error) {
	if depth > 128 {
		return "", projectionError("schema 引用形成循环或过深", pointer)
	}
	schema, name, err := p.resolve(input, pointer, 0)
	if err != nil {
		return "", err
	}
	if err := assertSupported(schema, pointer); err != nil {
		return "", err
	}
	if enum := schema.Get("enum").List(); len(enum) > 0 {
		if enum[0].Is(String) {
			return "string", nil
		}
		return scalarType(schema, pointer)
	}
	if additional := schema.Get("additionalProperties"); additional.Truth() {
		if additional.Is(Boolean) {
			return "", projectionError("additionalProperties: true 不在第一版支持范围内", pointer)
		}
		if len(schema.Get("properties").Entries()) > 0 {
			return "", projectionError("当前不支持 properties 与 additionalProperties 混用的 object", pointer)
		}
		typ, err := p.schemaType(additional, suggested+"Value", pointer+".additionalProperties", depth+1)
		if err != nil {
			return "", err
		}
		return "map<string, " + typ + ">", nil
	}
	if objectSchema(schema) {
		if name == "" {
			name, err = p.allocate(suggested)
			if err != nil {
				return "", err
			}
		}
		if _, ok := p.definitions[name]; ok || p.inProgress[name] {
			return name, nil
		}
		p.inProgress[name] = true
		fields := []ThriftField{}
		used := map[string]bool{}
		required := stringSet(schema.Get("required"))
		for _, field := range schema.Get("properties").Entries() {
			fieldPointer := pointer + "." + field.Name
			typ, err := p.schemaType(field.Value, name+pascal(field.Name), fieldPointer, depth+1)
			if err != nil {
				return "", err
			}
			fieldName, err := allocateField(used, field.Name)
			if err != nil {
				return "", err
			}
			tags, err := goTags(field.Value, required[field.Name], true, "optional", fieldPointer)
			if err != nil {
				return "", err
			}
			fields = append(fields, ThriftField{ID: len(fields) + 1, Requiredness: "optional", Type: typ, Name: fieldName, Annotations: tags, Comment: schemaComment(field.Value)})
		}
		delete(p.inProgress, name)
		p.register(ThriftStruct{Name: name, Comment: comments(schema.Get("description").Str()), Fields: fields})
		return name, nil
	}
	if schema.Get("type").Str() == "array" {
		if !schema.Has("items") {
			return "", projectionError("array schema 缺少 items", pointer)
		}
		typ, err := p.schemaType(schema.Get("items"), suggested+"Item", pointer+".items", depth+1)
		if err != nil {
			return "", err
		}
		return "list<" + typ + ">", nil
	}
	return scalarType(schema, pointer)
}

func (p *projector) resolve(input *Value, pointer string, depth int) (*Value, string, error) {
	if depth > 128 {
		return nil, "", projectionError("schema 引用形成循环或过深", pointer)
	}
	if !input.Is(Object) {
		return nil, "", projectionError("schema 必须是 object", pointer)
	}
	if nullableUnion(input) {
		branch, err := nullableBranch(input, pointer)
		if err != nil {
			return nil, "", err
		}
		value, name, err := p.resolve(branch, pointer, depth+1)
		if err != nil {
			return nil, "", err
		}
		value = value.Clone()
		value.Set("nullable", boolean(true))
		return value, name, nil
	}
	if isRef(input) {
		ref := input.Get("$ref").Str()
		match := schemaRef.FindStringSubmatch(ref)
		if match == nil {
			return nil, "", projectionError("当前只支持 #/components/schemas/* 本地引用", pointer)
		}
		schema := p.schemas.Get(match[1])
		if schema == nil {
			return nil, "", projectionError("找不到 schema 引用 "+ref, pointer)
		}
		value, _, err := p.resolve(schema, pointer, depth+1)
		return value, pascal(match[1]), err
	}
	if all := input.Get("allOf").List(); len(all) > 0 {
		merged := input.Clone()
		properties := input.Get("properties").Clone()
		if properties == nil {
			properties = &Value{Kind: Object}
		}
		required := []*Value{}
		seen := map[string]bool{}
		addRequired := func(items []*Value) {
			for _, item := range items {
				if !seen[item.Str()] {
					required = append(required, item)
					seen[item.Str()] = true
				}
			}
		}
		addRequired(input.Get("required").List())
		description := input.Get("description")
		nullable := input.Get("nullable").Truth()
		for i, part := range all {
			partPointer := fmt.Sprintf("%s.allOf[%d]", pointer, i)
			schema, _, err := p.resolve(part, partPointer, depth+1)
			if err != nil {
				return nil, "", err
			}
			if !objectSchema(schema) {
				return nil, "", projectionError("当前只支持可组合 object schema 的 allOf", partPointer)
			}
			for _, field := range schema.Get("properties").Entries() {
				properties.put(field.Name, field.Value)
			}
			addRequired(schema.Get("required").List())
			if description == nil {
				description = schema.Get("description")
			}
			nullable = nullable || schema.Get("nullable").Truth()
		}
		properties.orderNumericKeys()
		merged.Set("type", str("object"))
		if description != nil {
			merged.Set("description", description)
		}
		if nullable {
			merged.Set("nullable", boolean(true))
		}
		merged.Set("properties", properties)
		merged.Set("required", &Value{Kind: Array, Items: required})
		merged.Set("allOf", &Value{Kind: Array})
		return merged, "", nil
	}
	return input, "", nil
}

func singleContent(content *Value, pointer string) (string, *Value, error) {
	entries := content.Entries()
	if len(entries) == 0 {
		return "", nil, nil
	}
	if len(entries) > 1 {
		return "", nil, projectionError("当前不支持并行多 content-type 主线", pointer)
	}
	return entries[0].Name, entries[0].Value, nil
}
func isRef(v *Value) bool { return v.Is(Object) && v.Get("$ref").Is(String) }
func isJSONContent(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return value == "json" || strings.HasPrefix(value, "application/json")
}
func objectSchema(v *Value) bool {
	return v.Get("type").Str() == "object" || v.Get("properties").Truth()
}
func stringSet(v *Value) map[string]bool {
	set := map[string]bool{}
	for _, item := range v.List() {
		set[item.Str()] = true
	}
	return set
}
func schemaComment(v *Value) []string {
	if isRef(v) {
		return nil
	}
	return comments(v.Get("description").Str())
}
func nullableUnion(v *Value) bool {
	if isRef(v) || len(v.Get("anyOf").List()) != 2 {
		return false
	}
	for _, branch := range v.Get("anyOf").List() {
		if !isRef(branch) && branch.Get("type").Str() == "null" {
			return true
		}
	}
	return false
}
func nullableBranch(v *Value, pointer string) (*Value, error) {
	for _, branch := range v.Get("anyOf").List() {
		if isRef(branch) || branch.Get("type").Str() != "null" {
			return branch, nil
		}
	}
	return nil, projectionError("nullable anyOf 缺少非 null 分支", pointer)
}
func inlineSchema(v *Value, pointer string) (*Value, error) {
	if !nullableUnion(v) {
		return v, nil
	}
	branch, err := nullableBranch(v, pointer)
	if err != nil {
		return nil, err
	}
	if isRef(branch) {
		return nil, projectionError("当前不支持 nullable anyOf 的 inline $ref 分支", pointer)
	}
	branch = branch.Clone()
	branch.Set("nullable", boolean(true))
	return branch, nil
}
func scalarType(v *Value, pointer string) (string, error) {
	switch v.Get("type").Str() {
	case "string":
		if v.Get("format").Str() == "binary" {
			return "binary", nil
		}
		return "string", nil
	case "integer":
		if v.Get("format").Str() == "int64" {
			return "i64", nil
		}
		return "i32", nil
	case "number":
		return "double", nil
	case "boolean":
		return "bool", nil
	}
	return "", projectionError("不支持的 schema type: "+valueText(v.Get("type")), pointer)
}
func containsBinary(v *Value, pointer string, depth int) (bool, error) {
	if depth > 128 {
		return false, projectionError("schema 过深", pointer)
	}
	if isRef(v) {
		return false, nil
	}
	v, err := inlineSchema(v, pointer)
	if err != nil {
		return false, err
	}
	if v.Get("type").Str() == "string" && v.Get("format").Str() == "binary" {
		return true, nil
	}
	if v.Get("type").Str() == "array" && v.Has("items") {
		return containsBinary(v.Get("items"), pointer+"[]", depth+1)
	}
	if v.Get("type").Str() == "object" {
		for _, field := range v.Get("properties").Entries() {
			found, err := containsBinary(field.Value, pointer+"."+field.Name, depth+1)
			if found || err != nil {
				return found, err
			}
		}
	}
	return false, nil
}

func manualValidators(v *Value, pointer string) ([]string, error) {
	a, b := v.Get("x-ispark-validate"), v.Get("x-dramawork-validate")
	if a != nil && b != nil && !equalValue(a, b) {
		return nil, projectionError("x-ispark-validate 与已弃用的 x-dramawork-validate 必须保持一致", pointer)
	}
	raw := a
	if raw == nil || raw.Is(Null) {
		raw = b
	}
	if raw == nil {
		return []string{}, nil
	}
	if raw.Is(String) {
		value := strings.TrimSpace(raw.Str())
		if value == "" {
			return nil, projectionError("x-ispark-validate 不能为空字符串", pointer)
		}
		return []string{value}, nil
	}
	if raw.Is(Array) {
		values := []string{}
		for _, item := range raw.Items {
			if !item.Is(String) {
				return nil, projectionError("x-ispark-validate 数组只能包含字符串", pointer)
			}
			if value := strings.TrimSpace(item.Str()); value != "" {
				values = append(values, value)
			}
		}
		if len(values) == 0 {
			return nil, projectionError("x-ispark-validate 数组不能为空", pointer)
		}
		return values, nil
	}
	return nil, projectionError("x-ispark-validate 只支持 string 或 string[]", pointer)
}
func manualOverride(v *Value, pointer string) (bool, error) {
	a, b := v.Get("x-ispark-allow-unsupported-validation"), v.Get("x-dramawork-allow-unsupported-validation")
	if a != nil && b != nil && !equalValue(a, b) {
		return false, projectionError("x-ispark-allow-unsupported-validation 与已弃用的 x-dramawork-allow-unsupported-validation 必须保持一致", pointer)
	}
	raw := a
	if raw == nil || raw.Is(Null) {
		raw = b
	}
	if !raw.Truth() {
		return false, nil
	}
	validators, err := manualValidators(v, pointer)
	if err != nil {
		return false, err
	}
	if len(validators) == 0 {
		return false, projectionError("x-ispark-allow-unsupported-validation 需要同时提供 x-ispark-validate", pointer)
	}
	return true, nil
}
func supportedFormat(v *Value) bool {
	format := v.Get("format").Str()
	if format == "" {
		return true
	}
	switch v.Get("type").Str() {
	case "string":
		return format == "binary" || formatValidators[format] != ""
	case "integer":
		return format == "int32" || format == "int64"
	case "number":
		return format == "float" || format == "double"
	}
	return false
}
func assertSupported(v *Value, pointer string) error {
	allow, err := manualOverride(v, pointer)
	if err != nil {
		return err
	}
	if v.Get("format").Truth() && !supportedFormat(v) && !allow {
		return projectionError("当前不支持 format="+v.Get("format").Str()+" 自动投影，请改为手写 validator 或收紧 schema", pointer)
	}
	if len(v.Get("oneOf").List()) > 0 {
		return projectionError("当前不支持 oneOf", pointer)
	}
	if len(v.Get("anyOf").List()) > 0 && !nullableUnion(v) {
		return projectionError("当前不支持 anyOf", pointer)
	}
	if len(v.Get("allOf").List()) > 0 {
		return projectionError("当前不支持 allOf 继承拼装", pointer)
	}
	if !allow {
		for _, key := range []string{"pattern", "multipleOf", "exclusiveMinimum", "exclusiveMaximum", "uniqueItems", "minProperties", "maxProperties"} {
			item := v.Get(key)
			reject := item != nil
			switch key {
			case "pattern":
				reject = item.Is(String) && item.Truth()
			case "multipleOf", "minProperties", "maxProperties":
				reject = item.Is(Number)
			case "uniqueItems":
				reject = item.Truth()
			}
			if reject {
				return projectionError("当前不支持 "+key+" 自动投影，请改为手写 validator", pointer)
			}
		}
	}
	if item := v.Get("additionalProperties"); item.Is(Boolean) && !item.Bool {
		return projectionError("当前不支持 additionalProperties: false 自动投影；这需要自定义 binder 或改写 schema，不能通过字段 validator 接管", pointer)
	}
	return nil
}

func goTags(input *Value, required, allowDefault bool, requiredness, pointer string) ([]string, error) {
	var schema *Value
	var err error
	if !isRef(input) {
		schema, err = inlineSchema(input, pointer)
		if err != nil {
			return nil, err
		}
	}
	validators := []string{}
	if required && requiredness != "required" {
		validators = append(validators, "required")
	}
	if enum := schema.Get("enum").List(); len(enum) > 0 {
		values := []string{}
		for _, value := range enum {
			if value.Is(String) && whiteSpace.MatchString(value.Str()) {
				return nil, projectionError("第一版不支持包含空白字符的 string enum，请先手工收紧枚举值", pointer)
			}
			values = append(values, valueText(value))
		}
		validators = append(validators, "oneof="+strings.Join(values, " "))
	}
	if schema != nil {
		var bounds [][2]string
		switch schema.Get("type").Str() {
		case "integer", "number":
			bounds = [][2]string{{"minimum", "gte"}, {"maximum", "lte"}}
		case "string":
			bounds = [][2]string{{"minLength", "min"}, {"maxLength", "max"}}
		case "array":
			bounds = [][2]string{{"minItems", "min"}, {"maxItems", "max"}}
		}
		for _, bound := range bounds {
			if value := schema.Get(bound[0]); value.Is(Number) {
				validators = append(validators, bound[1]+"="+valueText(value))
			}
		}
		if validator := formatValidators[schema.Get("format").Str()]; validator != "" {
			validators = append(validators, validator)
		}
		manual, err := manualValidators(schema, pointer)
		if err != nil {
			return nil, err
		}
		validators = append(validators, manual...)
	}
	tags := []string{}
	if len(validators) > 0 {
		tags = append(tags, `validate:"`+strings.Join(validators, ",")+`"`)
	}
	if allowDefault && schema.Has("default") {
		value := schema.Get("default")
		rendered := ""
		if value.Is(String) {
			rendered = strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(value.Str())
		} else if value.Is(Number) || value.Is(Boolean) {
			rendered = valueText(value)
		}
		if rendered != "" {
			tags = append([]string{`default:"` + rendered + `"`}, tags...)
		}
	}
	if len(tags) == 0 {
		return []string{}, nil
	}
	return []string{"go.tag='" + strings.Join(tags, " ") + "'"}, nil
}
func valueText(v *Value) string {
	if v == nil {
		return "undefined"
	}
	if v.Is(String) {
		return v.Str()
	}
	body, err := json.Marshal(v)
	if err != nil {
		return "undefined"
	}
	return string(body)
}
func equalValue(a, b *Value) bool {
	return valueText(a) == valueText(b) && ((a == nil && b == nil) || (a != nil && b != nil && a.Kind == b.Kind))
}
