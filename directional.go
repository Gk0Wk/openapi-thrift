package openapithrift

import "strings"

// componentDirections follows schema edges only, never example payloads. The
// two independent visited sets allow recursive objects without sharing DTOs.
func componentDirections(document *Value) map[string]int {
	directions := map[string]int{}
	components := document.Get("components")
	var schema func(*Value, int)
	schema = func(v *Value, direction int) {
		if !v.Is(Object) {
			return
		}
		if ref := v.Get("$ref").Str(); strings.HasPrefix(ref, "#/components/schemas/") {
			name := strings.TrimPrefix(ref, "#/components/schemas/")
			if directions[name]&direction != 0 {
				return
			}
			directions[name] |= direction
			schema(components.Get("schemas").Get(name), direction)
			return
		}
		for _, field := range v.Get("properties").Entries() {
			schema(field.Value, direction)
		}
		for _, key := range []string{"items", "additionalProperties"} {
			schema(v.Get(key), direction)
		}
		for _, key := range []string{"allOf", "anyOf", "oneOf"} {
			for _, item := range v.Get(key).List() {
				schema(item, direction)
			}
		}
	}
	var container func(*Value, string, int, map[string]bool)
	container = func(v *Value, section string, direction int, seen map[string]bool) {
		if ref := v.Get("$ref").Str(); ref != "" {
			prefix := "#/components/" + section + "/"
			if strings.HasPrefix(ref, prefix) && !seen[ref] {
				seen[ref] = true
				container(components.Get(section).Get(strings.TrimPrefix(ref, prefix)), section, direction, seen)
			}
			return
		}
		for _, media := range v.Get("content").Entries() {
			schema(media.Value.Get("schema"), direction)
		}
	}
	for _, path := range document.Get("paths").Entries() {
		for _, method := range httpMethods {
			op := path.Value.Get(method)
			if !op.Is(Object) {
				continue
			}
			for _, params := range []*Value{path.Value.Get("parameters"), op.Get("parameters")} {
				for _, param := range params.List() {
					schema(param.Get("schema"), 1)
				}
			}
			container(op.Get("requestBody"), "requestBodies", 1, map[string]bool{})
			for _, response := range op.Get("responses").Entries() {
				if successCode.MatchString(response.Name) {
					container(response.Value, "responses", 2, map[string]bool{})
				}
			}
		}
	}
	return directions
}

func (p *projector) componentName(name string) string {
	result := pascal(name)
	if p.directions[name] == 3 {
		if p.responseDirection {
			result += "Response"
		} else {
			result += "Request"
		}
	}
	return result
}

func (p *projector) reserveComponentNames() error {
	owners := map[string]string{}
	for _, source := range sortedMapKeys(p.directions) {
		for _, direction := range []int{1, 2} {
			if p.directions[source]&direction == 0 {
				continue
			}
			p.responseDirection = direction == 2
			name := p.componentName(source)
			if owner, ok := owners[name]; ok && owner != source {
				return projectionError("方向独立组件名称冲突: "+name, "#/components/schemas/"+source)
			}
			owners[name] = source
			p.used[name] = true
		}
	}
	return nil
}

func (p *projector) fieldTags(input *Value, required bool, fieldName, pointer string) ([]string, error) {
	if p.rawOperation {
		return []string{jsonFieldTag(fieldName, required)}, nil
	}
	tags, err := p.goTags(input, required, true, "optional", pointer)
	if err != nil {
		return nil, err
	}
	schema, _, err := p.resolve(input, pointer, 0)
	if err != nil {
		return nil, err
	}
	if p.responseDirection && required && (schema.Get("nullable").Truth() || schema.Get("additionalProperties").Is(Object)) {
		// Keep existing wire names for ordinary DTOs; exact OpenAPI names are part
		// of the explicit raw envelope contract, not an implicit naming migration.
		jsonTag := `json:"` + fieldName + `"`
		if len(tags) == 0 {
			tags = []string{"go.tag='" + jsonTag + "'"}
		} else {
			tags[0] = strings.TrimSuffix(tags[0], "'") + " " + jsonTag + "'"
		}
	}
	return tags, nil
}

func jsonFieldTag(name string, required bool) string {
	suffix := ",omitempty"
	if required {
		suffix = ""
	}
	return `go.tag='json:"` + name + suffix + `"'`
}
