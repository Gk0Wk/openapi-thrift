package rawjson

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"unicode/utf8"
)

const maxNestingDepth = 10000

func parseStrict(data []byte) (any, error) {
	if err := validateUnicode(data); err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	value, err := readValue(decoder, 0)
	if err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, fmt.Errorf("raw JSON: expected end of input")
	}
	return value, nil
}

func readValue(decoder *json.Decoder, depth int) (any, error) {
	value, err := decoder.Token()
	if err != nil {
		return nil, fmt.Errorf("raw JSON: invalid syntax: %w", err)
	}
	delimiter, ok := value.(json.Delim)
	if !ok {
		return value, nil
	}
	if depth >= maxNestingDepth {
		return nil, fmt.Errorf("raw JSON: maximum nesting depth exceeded")
	}
	switch delimiter {
	case '{':
		fields := make(map[string]any)
		for decoder.More() {
			token, err := decoder.Token()
			if err != nil {
				return nil, fmt.Errorf("raw JSON: invalid object key: %w", err)
			}
			key, ok := token.(string)
			if !ok {
				return nil, fmt.Errorf("raw JSON: expected object key")
			}
			if _, exists := fields[key]; exists {
				return nil, fmt.Errorf("raw JSON: duplicate object key")
			}
			fields[key], err = readValue(decoder, depth+1)
			if err != nil {
				return nil, err
			}
		}
		if token, err := decoder.Token(); err != nil || token != json.Delim('}') {
			return nil, fmt.Errorf("raw JSON: unclosed object")
		}
		return fields, nil
	case '[':
		values := make([]any, 0)
		for decoder.More() {
			value, err := readValue(decoder, depth+1)
			if err != nil {
				return nil, err
			}
			values = append(values, value)
		}
		if token, err := decoder.Token(); err != nil || token != json.Delim(']') {
			return nil, fmt.Errorf("raw JSON: unclosed array")
		}
		return values, nil
	default:
		return nil, fmt.Errorf("raw JSON: unexpected delimiter")
	}
}

// encoding/json repairs invalid Unicode, so reject it before decoding tokens.
func validateUnicode(data []byte) error {
	if !utf8.Valid(data) {
		return fmt.Errorf("raw JSON: invalid UTF-8")
	}
	inString := false
	for i := 0; i < len(data); i++ {
		if data[i] == '"' {
			inString = !inString
			continue
		}
		if !inString || data[i] != '\\' {
			continue
		}
		i++
		if i >= len(data) {
			return fmt.Errorf("raw JSON: incomplete string escape")
		}
		if data[i] != 'u' {
			continue
		}
		first, err := escapedCodeUnit(data, i)
		if err != nil {
			return err
		}
		i += 4
		if first >= 0xdc00 && first <= 0xdfff {
			return fmt.Errorf("raw JSON: unpaired Unicode surrogate")
		}
		if first < 0xd800 || first > 0xdbff {
			continue
		}
		if i+2 >= len(data) || data[i+1] != '\\' || data[i+2] != 'u' {
			return fmt.Errorf("raw JSON: unpaired Unicode surrogate")
		}
		second, err := escapedCodeUnit(data, i+2)
		if err != nil {
			return err
		}
		if second < 0xdc00 || second > 0xdfff {
			return fmt.Errorf("raw JSON: unpaired Unicode surrogate")
		}
		i += 6
	}
	return nil
}

func escapedCodeUnit(data []byte, start int) (uint64, error) {
	if start+4 >= len(data) {
		return 0, fmt.Errorf("raw JSON: incomplete Unicode escape")
	}
	value, err := strconv.ParseUint(string(data[start+1:start+5]), 16, 16)
	if err != nil {
		return 0, fmt.Errorf("raw JSON: invalid Unicode escape")
	}
	return value, nil
}
