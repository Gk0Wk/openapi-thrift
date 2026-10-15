package openapithrift

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Gk0Wk/openapi-thrift/rawjson"
)

const rawMarker = "x-ispark-raw-json"

var rawName = regexp.MustCompile(`^[A-Z][A-Za-z0-9]*$`)

// RawJSONContract is the versioned, deterministic boundary between the sole Go
// compiler and generated HTTP transports. Descriptors never resolve URLs/files.
type compiledRawContract struct {
	SchemaVersion int                    `json:"schemaVersion"`
	Schemas       map[string]*Value      `json:"schemas"`
	Operations    []compiledRawOperation `json:"operations"`
}
type compiledRawOperation struct {
	Method              string `json:"method"`
	Path                string `json:"path"`
	RequestBody         *Value `json:"requestBody,omitempty"`
	RequestBodyRequired *bool  `json:"requestBodyRequired,omitempty"`
	ResponseBody        *Value `json:"responseBody,omitempty"`
}
type rawCompiler struct {
	document  *Value
	contract  compiledRawContract
	marked    map[string]bool
	compiling map[string]bool
	index     map[string]*Value
	budget    int
}

func CompileRawJSONContract(document *Value) (RawJSONContract, error) {
	c, err := compileRawJSON(document)
	if err != nil {
		return RawJSONContract{}, err
	}
	body, err := json.Marshal(c.contract)
	if err != nil {
		return RawJSONContract{}, err
	}
	var result RawJSONContract
	if err := json.Unmarshal(body, &result); err != nil {
		return result, err
	}
	for i := range result.Operations {
		operation := &result.Operations[i]
		item := document.Get("paths").Get(operation.Path)
		source := item.Get(operation.Method)
		parameters := map[string]RawJSONParameter{}
		for _, list := range []*Value{item.Get("parameters"), source.Get("parameters")} {
			for _, param := range list.List() {
				if param.Has("$ref") {
					return result, projectionError("raw HTTP parameter refs must be expanded", operation.Path)
				}
				parameter := RawJSONParameter{In: param.Get("in").Str(), Name: param.Get("name").Str()}
				switch parameter.In {
				case "path", "query", "header", "cookie":
				default:
					return result, projectionError("unsupported parameter channel", operation.Path)
				}
				parameters[parameter.In+" "+parameter.Name] = parameter
			}
		}
		operation.Parameters = []RawJSONParameter{}
		for _, key := range sortedMapKeys(parameters) {
			operation.Parameters = append(operation.Parameters, parameters[key])
		}
		for _, response := range source.Get("responses").Entries() {
			if successCode.MatchString(response.Name) {
				operation.SuccessStatus, err = strconv.Atoi(response.Name)
				if err != nil {
					return result, err
				}
			}
		}
	}
	return result, nil
}

func compileRawJSON(document *Value) (*rawCompiler, error) {
	c := &rawCompiler{document: document, contract: compiledRawContract{1, map[string]*Value{}, []compiledRawOperation{}}, marked: map[string]bool{}, compiling: map[string]bool{}, index: map[string]*Value{}, budget: 100000}
	components := document.Get("components")
	// Marker placement is checked even when there are no valid declarations.
	var scanSchema func(*Value, string, bool) error
	rejectMarker := func(v *Value, pointer string) error {
		if v.Has(rawMarker) {
			return projectionError("raw JSON 标记仅允许放在组件 schema", pointer+"/"+rawMarker)
		}
		return nil
	}
	scanSchema = func(v *Value, pointer string, component bool) error {
		if !v.Is(Object) {
			return nil
		}
		if v.Has(rawMarker) && !component {
			return rejectMarker(v, pointer)
		}
		for _, field := range v.Get("properties").Entries() {
			if err := scanSchema(field.Value, pointer+"/properties/"+escapePointer(field.Name), false); err != nil {
				return err
			}
		}
		for _, key := range []string{"items", "additionalProperties"} {
			if err := scanSchema(v.Get(key), pointer+"/"+key, false); err != nil {
				return err
			}
		}
		for _, key := range []string{"oneOf", "anyOf", "allOf"} {
			for i, item := range v.Get(key).List() {
				if err := scanSchema(item, fmt.Sprintf("%s/%s/%d", pointer, key, i), false); err != nil {
					return err
				}
			}
		}
		return nil
	}
	scanContent := func(v *Value, pointer string) error {
		if err := rejectMarker(v, pointer); err != nil {
			return err
		}
		for _, media := range v.Get("content").Entries() {
			where := pointer + "/content/" + escapePointer(media.Name)
			if err := rejectMarker(media.Value, where); err != nil {
				return err
			}
			if err := scanSchema(media.Value.Get("schema"), where+"/schema", false); err != nil {
				return err
			}
		}
		return nil
	}
	for _, v := range []*Value{document, components} {
		if err := rejectMarker(v, "#"); err != nil {
			return nil, err
		}
	}
	for _, field := range components.Get("schemas").Entries() {
		pointer := "#/components/schemas/" + escapePointer(field.Name)
		if field.Value.Has(rawMarker) {
			if !isBool(field.Value.Get(rawMarker), true) || !rawName.MatchString(field.Name) {
				return nil, projectionError("raw JSON 标记必须为 true，组件名须为 PascalCase 标识符", pointer)
			}
			c.marked[field.Name] = true
		}
		if err := scanSchema(field.Value, pointer, true); err != nil {
			return nil, err
		}
	}
	for _, section := range []string{"requestBodies", "responses"} {
		for _, field := range components.Get(section).Entries() {
			if err := scanContent(field.Value, "#/components/"+section+"/"+escapePointer(field.Name)); err != nil {
				return nil, err
			}
		}
	}
	for _, field := range components.Get("parameters").Entries() {
		pointer := "#/components/parameters/" + escapePointer(field.Name)
		if err := rejectMarker(field.Value, pointer); err != nil {
			return nil, err
		}
		if err := scanSchema(field.Value.Get("schema"), pointer+"/schema", false); err != nil {
			return nil, err
		}
		if err := scanContent(field.Value, pointer); err != nil {
			return nil, err
		}
	}
	for _, path := range document.Get("paths").Entries() {
		if err := rejectMarker(path.Value, "#/paths/"+escapePointer(path.Name)); err != nil {
			return nil, err
		}
		for _, method := range httpMethods {
			op := path.Value.Get(method)
			if !op.Is(Object) {
				continue
			}
			pointer := "#/paths/" + escapePointer(path.Name) + "/" + method
			if err := rejectMarker(op, pointer); err != nil {
				return nil, err
			}
			for _, params := range []*Value{path.Value.Get("parameters"), op.Get("parameters")} {
				for i, param := range params.List() {
					if err := rejectMarker(param, pointer+"/parameters"); err != nil {
						return nil, err
					}
					if err := scanSchema(param.Get("schema"), fmt.Sprintf("%s/parameters/%d/schema", pointer, i), false); err != nil {
						return nil, err
					}
				}
			}
			if err := scanContent(op.Get("requestBody"), pointer+"/requestBody"); err != nil {
				return nil, err
			}
			for _, response := range op.Get("responses").Entries() {
				if err := scanContent(response.Value, pointer+"/responses/"+response.Name); err != nil {
					return nil, err
				}
			}
		}
	}
	if len(c.marked) == 0 {
		return c, nil
	}
	for _, name := range sortedMapKeys(c.marked) {
		if _, err := c.compileMarked(name); err != nil {
			return nil, err
		}
	}
	for _, path := range document.Get("paths").Entries() {
		for _, method := range httpMethods {
			op := path.Value.Get(method)
			if !op.Is(Object) {
				continue
			}
			pointer := "#/paths/" + escapePointer(path.Name) + "/" + method
			request, err := c.container(op.Get("requestBody"), "requestBodies", pointer+"/requestBody")
			if err != nil {
				return nil, err
			}
			uses := c.contentUsesRaw(request)
			responses := map[string]*Value{}
			successes := []string{}
			for _, response := range op.Get("responses").Entries() {
				value, err := c.container(response.Value, "responses", pointer+"/responses/"+response.Name)
				if err != nil {
					return nil, err
				}
				responses[response.Name] = value
				if successCode.MatchString(response.Name) {
					successes = append(successes, response.Name)
					uses = uses || c.contentUsesRaw(value)
				} else if c.contentUsesRaw(value) {
					return nil, projectionError("raw JSON 不允许用于错误响应", pointer+"/responses/"+response.Name)
				}
			}
			for _, params := range []*Value{path.Value.Get("parameters"), op.Get("parameters")} {
				for _, param := range params.List() {
					if c.usesRaw(param.Get("schema"), map[string]bool{}) {
						return nil, projectionError("raw JSON 不允许用于 HTTP 参数", pointer+"/parameters")
					}
				}
			}
			if !uses {
				continue
			}
			if len(successes) != 1 {
				return nil, projectionError("raw JSON 操作需要唯一成功响应", pointer)
			}
			for code, response := range responses {
				if (method == "head" || code == "204" || code == "205") && response.Has("content") {
					return nil, projectionError("HEAD/204/205 不允许声明响应 content", pointer+"/responses/"+code)
				}
			}
			entry := compiledRawOperation{Method: method, Path: path.Name}
			if request != nil {
				if request.Has("required") && !request.Get("required").Is(Boolean) {
					return nil, projectionError("requestBody.required 必须是 boolean", pointer)
				}
				entry.RequestBody, err = c.content(request, pointer+"/requestBody", true)
				if err != nil {
					return nil, err
				}
				required := request.Get("required").Truth()
				entry.RequestBodyRequired = &required
			}
			entry.ResponseBody, err = c.content(responses[successes[0]], pointer+"/responses/"+successes[0], false)
			if err != nil {
				return nil, err
			}
			c.contract.Operations = append(c.contract.Operations, entry)
		}
	}
	sort.Slice(c.contract.Operations, func(i, j int) bool {
		a, b := c.contract.Operations[i], c.contract.Operations[j]
		return a.Path+" "+a.Method < b.Path+" "+b.Method
	})
	for _, pointer := range sortedMapKeys(c.index) {
		c.registerIndex(valueAtPointer(document, pointer), c.index[pointer], pointer, map[string]bool{})
	}
	for _, section := range []string{"requestBodies", "responses"} {
		for _, field := range components.Get(section).Entries() {
			container, err := c.container(field.Value, section, "#/components/"+section+"/"+escapePointer(field.Name))
			if err != nil {
				return nil, err
			}
			schema := container.Get("content").Get("application/json").Get("schema")
			if c.contentUsesRaw(container) {
				desc, err := c.content(container, "#/components/"+section+"/"+escapePointer(field.Name), section == "requestBodies")
				if err != nil {
					return nil, err
				}
				if desc != nil {
					c.registerIndex(schema, desc, "#/components/"+section+"/"+escapePointer(field.Name)+"/content/application~1json/schema", map[string]bool{})
				}
			}
		}
	}
	body, err := json.Marshal(c.contract)
	if err != nil {
		return nil, err
	}
	if _, err := rawjson.LoadContract(body); err != nil {
		return nil, projectionError("严格 raw JSON 契约无效: "+err.Error(), "#")
	}
	return c, nil
}

func valueAtPointer(v *Value, pointer string) *Value {
	for _, token := range strings.Split(strings.TrimPrefix(pointer, "#/"), "/") {
		token = strings.NewReplacer("~1", "/", "~0", "~").Replace(token)
		if v.Is(Array) {
			var i int
			if _, err := fmt.Sscanf(token, "%d", &i); err != nil || i < 0 || i >= len(v.Items) {
				return nil
			}
			v = v.Items[i]
		} else {
			v = v.Get(token)
		}
	}
	return v
}

func (c *rawCompiler) registerIndex(source, descriptor *Value, pointer string, seen map[string]bool) {
	if source == nil || descriptor == nil {
		return
	}
	c.index[pointer] = descriptor
	if ref := source.Get("$ref").Str(); ref != "" {
		if seen[ref] {
			return
		}
		next := map[string]bool{}
		for k, v := range seen {
			next[k] = v
		}
		next[ref] = true
		name, target, err := c.reference(ref, "schemas", pointer)
		if err != nil {
			return
		}
		c.registerIndex(target, descriptor, "#/components/schemas/"+escapePointer(name), next)
		source = target
	}
	if ref := descriptor.Get("$ref").Str(); ref != "" {
		descriptor = c.contract.Schemas[strings.TrimPrefix(ref, "#/schemas/")]
	}
	for _, property := range source.Get("properties").Entries() {
		c.registerIndex(property.Value, descriptor.Get("properties").Get(property.Name), pointer+"/properties/"+escapePointer(property.Name), seen)
	}
	if source.Has("items") {
		c.registerIndex(source.Get("items"), descriptor.Get("items"), pointer+"/items", seen)
	}
	for i, branch := range source.Get("oneOf").List() {
		if i < len(descriptor.Get("oneOf").List()) {
			c.registerIndex(branch, descriptor.Get("oneOf").Items[i], fmt.Sprintf("%s/oneOf/%d", pointer, i), seen)
		}
	}
}

func sortedMapKeys[T any](m map[string]T) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func (c *rawCompiler) reference(ref, section, pointer string) (string, *Value, error) {
	prefix := "#/components/" + section + "/"
	if !strings.HasPrefix(ref, prefix) {
		return "", nil, projectionError("只允许本地组件引用", pointer)
	}
	token := strings.TrimPrefix(ref, prefix)
	if strings.Contains(token, "/") || regexp.MustCompile(`~([^01]|$)`).MatchString(token) {
		return "", nil, projectionError("非法组件引用", pointer)
	}
	name := strings.NewReplacer("~1", "/", "~0", "~").Replace(token)
	value := c.document.Get("components").Get(section).Get(name)
	if value == nil {
		return "", nil, projectionError("组件引用不存在", pointer)
	}
	return name, value, nil
}
func (c *rawCompiler) container(v *Value, section, pointer string) (*Value, error) {
	seen := map[string]bool{}
	for v.Has("$ref") {
		ref := v.Get("$ref").Str()
		if len(v.Entries()) != 1 || seen[ref] {
			return nil, projectionError("组件容器引用循环或含 sibling", pointer)
		}
		seen[ref] = true
		_, target, err := c.reference(ref, section, pointer)
		if err != nil {
			return nil, err
		}
		v = target
	}
	return v, nil
}
func (c *rawCompiler) compileMarked(name string) (*Value, error) {
	if v := c.contract.Schemas[name]; v != nil {
		return v, nil
	}
	if c.compiling[name] {
		return nil, projectionError("raw JSON 引用循环", "#/components/schemas/"+name)
	}
	c.compiling[name] = true
	v, err := c.compile(c.document.Get("components").Get("schemas").Get(name), "#/components/schemas/"+escapePointer(name), map[string]bool{}, false)
	if err != nil {
		return nil, err
	}
	if v.Get("type").Str() != "object" && !v.Has("oneOf") {
		return nil, projectionError("raw JSON 根必须为封闭 object 或判别 union", name)
	}
	delete(c.compiling, name)
	c.contract.Schemas[name] = v
	return v, nil
}

func (c *rawCompiler) compile(v *Value, pointer string, stack map[string]bool, typed bool) (*Value, error) {
	c.budget--
	if c.budget < 0 || len(stack) > 128 {
		return nil, projectionError("raw JSON 契约过大或过深", pointer)
	}
	if !v.Is(Object) {
		return nil, projectionError("raw JSON schema 必须是 object", pointer)
	}
	if v.Has("$ref") {
		if len(v.Entries()) != 1 {
			return nil, projectionError("raw JSON 引用不允许 sibling", pointer)
		}
		ref := v.Get("$ref").Str()
		name, target, err := c.reference(ref, "schemas", pointer)
		if err != nil {
			return nil, err
		}
		if stack[ref] {
			return nil, projectionError("raw JSON 引用循环", pointer)
		}
		next := map[string]bool{}
		for key, value := range stack {
			next[key] = value
		}
		next[ref] = true
		if c.marked[name] {
			if _, err := c.compileMarked(name); err != nil {
				return nil, err
			}
			result := &Value{Kind: Object}
			result.Set("$ref", str("#/schemas/"+name))
			c.index[pointer] = result
			return result, nil
		}
		result, err := c.compile(target, "#/components/schemas/"+escapePointer(name), next, typed)
		if err != nil {
			return nil, err
		}
		c.index[pointer] = result
		return result, nil
	}
	out := &Value{Kind: Object}
	annotations := map[string]bool{"title": true, "description": true, "example": true, "examples": true, "deprecated": true, "x-apifox-mock": true, "mockScript": true, rawMarker: true}
	for _, field := range v.Entries() {
		if annotations[field.Name] {
			continue
		}
		if field.Name == "nullable" {
			if !field.Value.Is(Boolean) {
				return nil, projectionError("nullable 必须为 boolean", pointer)
			}
			if field.Value.Bool {
				out.Set("nullable", boolean(true))
			}
			continue
		}
		switch field.Name {
		case "properties":
			if !field.Value.Is(Object) {
				return nil, projectionError("properties 必须是 object", pointer)
			}
			properties := &Value{Kind: Object}
			names := []string{}
			for _, property := range field.Value.Entries() {
				names = append(names, property.Name)
			}
			sort.Strings(names)
			for _, name := range names {
				if typed && !validJSONTagName(name) {
					return nil, projectionError("字段名无法由 Go JSON tag 精确表达", pointer+"/properties/"+escapePointer(name))
				}
				child, err := c.compile(field.Value.Get(name), pointer+"/properties/"+escapePointer(name), stack, typed)
				if err != nil {
					return nil, err
				}
				properties.Set(name, child)
			}
			out.Set("properties", properties)
		case "items":
			child, err := c.compile(field.Value, pointer+"/items", stack, typed)
			if err != nil {
				return nil, err
			}
			out.Set("items", child)
		case "oneOf":
			if !field.Value.Is(Array) {
				return nil, projectionError("oneOf 必须是数组", pointer)
			}
			values := &Value{Kind: Array, Items: []*Value{}}
			for i, branch := range field.Value.List() {
				child, err := c.compile(branch, fmt.Sprintf("%s/oneOf/%d", pointer, i), stack, false)
				if err != nil {
					return nil, err
				}
				if child.Has("$ref") {
					child = c.contract.Schemas[strings.TrimPrefix(child.Get("$ref").Str(), "#/schemas/")]
				}
				values.Items = append(values.Items, child)
			}
			out.Set("oneOf", values)
		default:
			out.Set(field.Name, field.Value.Clone())
		}
	}
	if out.Get("type").Str() == "object" {
		if !out.Has("required") {
			out.Set("required", &Value{Kind: Array, Items: []*Value{}})
		}
		if !out.Has("properties") {
			return nil, projectionError("封闭对象需要显式 properties", pointer)
		}
	}
	// Deterministic collection ordering; the runtime validates element kinds,
	// uniqueness, constraints and the closed keyword set without coercion.
	for _, key := range []string{"required", "enum"} {
		if values := out.Get(key); values.Is(Array) {
			sort.SliceStable(values.Items, func(i, j int) bool { return valueText(values.Items[i]) < valueText(values.Items[j]) })
		}
	}
	c.index[pointer] = out
	return out, nil
}
func validJSONTagName(name string) bool {
	return name != "" && name != "-" && !strings.ContainsAny(name, ",\"\\'\r\n\t") && regexp.MustCompile(`[A-Za-z0-9]`).MatchString(name) && strings.IndexFunc(name, func(r rune) bool { return r < 32 || r == 127 }) < 0
}
func (c *rawCompiler) usesRaw(v *Value, seen map[string]bool) bool {
	if !v.Is(Object) {
		return false
	}
	if ref := v.Get("$ref").Str(); ref != "" {
		name, target, err := c.reference(ref, "schemas", "#")
		if err != nil || seen[ref] {
			return false
		}
		if c.marked[name] {
			return true
		}
		seen[ref] = true
		return c.usesRaw(target, seen)
	}
	for _, field := range v.Get("properties").Entries() {
		if c.usesRaw(field.Value, seen) {
			return true
		}
	}
	for _, key := range []string{"items", "additionalProperties"} {
		if c.usesRaw(v.Get(key), seen) {
			return true
		}
	}
	for _, key := range []string{"oneOf", "anyOf", "allOf"} {
		for _, item := range v.Get(key).List() {
			if c.usesRaw(item, seen) {
				return true
			}
		}
	}
	return false
}
func (c *rawCompiler) contentUsesRaw(v *Value) bool {
	for _, media := range v.Get("content").Entries() {
		if c.usesRaw(media.Value.Get("schema"), map[string]bool{}) {
			return true
		}
	}
	return false
}
func (c *rawCompiler) content(v *Value, pointer string, required bool) (*Value, error) {
	content := v.Get("content")
	if len(content.Entries()) == 0 && !required {
		return nil, nil
	}
	if len(content.Entries()) != 1 || !content.Has("application/json") {
		return nil, projectionError("raw JSON body 仅允许单一 application/json", pointer)
	}
	input := content.Get("application/json").Get("schema")
	descriptor, err := c.compile(input, pointer+"/content/application~1json/schema", map[string]bool{}, true)
	if err != nil {
		return nil, err
	}
	if descriptor.Get("type").Str() != "object" || descriptor.Has("nullable") {
		return nil, projectionError("raw JSON body 必须为非 null 封闭对象 envelope，不能直接使用 raw 根", pointer)
	}
	return descriptor, nil
}
