// Package transport adapts the shared application API without owning its semantics.
package transport

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"
	"unicode/utf8"

	"github.com/m0n0x41d/haft/internal/core/app"
)

const MaxInputBytes = 16 << 20
const maxDepth = 64
const maxValues = 200000

// DecodeRequest rejects duplicate keys, unknown fields (including nested fields),
// case-folded aliases, invalid UTF-8, trailing data and excessive resource use.
// Map keys such as snapshot digests remain data rather than struct field names.
func DecodeRequest(raw []byte) (app.Request, error) {
	var q app.Request
	err := Decode(raw, &q)
	return q, err
}

func ReadInput(r io.Reader) ([]byte, error) {
	raw, err := io.ReadAll(io.LimitReader(r, MaxInputBytes+1))
	if err == nil && len(raw) > MaxInputBytes {
		err = fmt.Errorf("input_limit: maximum %d bytes", MaxInputBytes)
	}
	return raw, err
}

// Decode provides the same closed decoder for transport envelopes and requests.
func Decode(raw []byte, target any) error {
	if len(raw) > MaxInputBytes {
		return fmt.Errorf("input_limit: maximum %d bytes", MaxInputBytes)
	}
	if !utf8.Valid(raw) {
		return fmt.Errorf("invalid_utf8")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	count := 0
	v, err := readValue(d, "$", 0, &count)
	if err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return fmt.Errorf("trailing_json: expected one JSON object")
	}
	if _, ok := v.(map[string]any); !ok {
		return fmt.Errorf("invalid_json: expected an object")
	}
	t := reflect.TypeOf(target)
	if t == nil || t.Kind() != reflect.Pointer {
		return fmt.Errorf("invalid_decode_target")
	}
	if err := validateFields(v, t.Elem(), "$"); err != nil {
		return err
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("invalid_json: %w", err)
	}
	return nil
}

func readValue(d *json.Decoder, path string, depth int, count *int) (any, error) {
	*count++
	if depth > maxDepth || *count > maxValues {
		return nil, fmt.Errorf("input_limit: JSON nesting or value count exceeded")
	}
	token, err := d.Token()
	if err != nil {
		return nil, fmt.Errorf("invalid_json: %w", err)
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return token, nil
	}
	switch delim {
	case '{':
		m := map[string]any{}
		for d.More() {
			keyToken, err := d.Token()
			if err != nil {
				return nil, fmt.Errorf("invalid_json: %w", err)
			}
			key, ok := keyToken.(string)
			if !ok {
				return nil, fmt.Errorf("invalid_json: non-string object key")
			}
			if _, exists := m[key]; exists {
				return nil, fmt.Errorf("duplicate_field: %s.%s", path, key)
			}
			value, err := readValue(d, path+"."+key, depth+1, count)
			if err != nil {
				return nil, err
			}
			m[key] = value
		}
		if _, err := d.Token(); err != nil {
			return nil, fmt.Errorf("invalid_json: %w", err)
		}
		return m, nil
	case '[':
		var values []any
		for d.More() {
			value, err := readValue(d, path+"[]", depth+1, count)
			if err != nil {
				return nil, err
			}
			values = append(values, value)
		}
		if _, err := d.Token(); err != nil {
			return nil, fmt.Errorf("invalid_json: %w", err)
		}
		return values, nil
	default:
		return nil, fmt.Errorf("invalid_json: unexpected delimiter")
	}
}

func fieldTypes(t reflect.Type) map[string]reflect.Type {
	m := map[string]reflect.Type{}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		name := strings.Split(f.Tag.Get("json"), ",")[0]
		if name == "-" {
			continue
		}
		if name == "" {
			name = f.Name
		}
		m[name] = f.Type
	}
	return m
}
func validateFields(v any, t reflect.Type, path string) error {
	if t == reflect.TypeOf(json.RawMessage{}) {
		return nil
	}
	if t.Kind() == reflect.Pointer {
		if v == nil {
			return nil
		}
		return validateFields(v, t.Elem(), path)
	}
	if v == nil { // Null is valid only where the Go wire type can represent it.
		if t.Kind() == reflect.Map || t.Kind() == reflect.Slice || t.Kind() == reflect.Interface {
			return nil
		}
		return fmt.Errorf("invalid_json: null scalar at %s", path)
	}
	switch t.Kind() {
	case reflect.Struct:
		m, ok := v.(map[string]any)
		if !ok {
			return fmt.Errorf("invalid_json: expected object at %s", path)
		}
		fields := fieldTypes(t)
		for key, value := range m {
			ft, ok := fields[key]
			if !ok {
				return fmt.Errorf("unknown_field: %s.%s", path, key)
			}
			if err := validateFields(value, ft, path+"."+key); err != nil {
				return err
			}
		}
	case reflect.Map:
		if m, ok := v.(map[string]any); ok {
			for key, value := range m {
				if err := validateFields(value, t.Elem(), path+"."+key); err != nil {
					return err
				}
			}
		}
	case reflect.Slice, reflect.Array:
		if t.Elem().Kind() == reflect.Uint8 {
			return nil
		}
		if a, ok := v.([]any); ok {
			for _, value := range a {
				if err := validateFields(value, t.Elem(), path+"[]"); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// RequestSchema derives every field from the same versioned wire type decoded by
// CLI and MCP. Application validation remains the source of semantic checks.
func RequestSchema() map[string]any {
	s := schema(reflect.TypeOf(app.Request{}))
	p := s["properties"].(map[string]any)
	p["format"].(map[string]any)["enum"] = []string{app.Format}
	p["operation"].(map[string]any)["enum"] = []string{"remember", "recall", "context", "impact", "fpf", "source", "check", "change", "recover"}
	p["action"].(map[string]any)["description"] = "fpf/source: status, search, inspect; check: structural, prepare, observe; change: create, list, show, preview, apply, sync, archive, reopen, rebase, update; remember: terms or omitted."
	p["carrier"].(map[string]any)["description"] = "Authored Markdown with YAML frontmatter. Explicit input fields are local trusted data; the adapter does not infer operator confirmation."
	p["ref"].(map[string]any)["description"] = "Exact record/claim reference, source locator, or file:/dir:/sym: selector for the chosen operation."
	p["request_id"].(map[string]any)["description"] = "Stable caller-generated idempotency key for a write; reuse only with identical payload."
	p["expected_generation"].(map[string]any)["description"] = "Transaction basis from a previous result; required where the application requests optimistic concurrency."
	p["limit"].(map[string]any)["minimum"] = 0
	p["limit"].(map[string]any)["maximum"] = 500
	s["required"] = []string{"format", "operation"}
	return s
}
func schema(t reflect.Type) map[string]any {
	if t.Kind() == reflect.Pointer {
		return schema(t.Elem())
	}
	s := map[string]any{}
	switch t.Kind() {
	case reflect.Struct:
		p := map[string]any{}
		for name, ft := range fieldTypes(t) {
			p[name] = schema(ft)
		}
		s["type"] = "object"
		s["properties"] = p
		s["additionalProperties"] = false
	case reflect.Map:
		s["type"] = "object"
		s["additionalProperties"] = schema(t.Elem())
	case reflect.Slice, reflect.Array:
		if t.Elem().Kind() == reflect.Uint8 {
			s["type"] = "string"
			s["contentEncoding"] = "base64"
		} else {
			s["type"] = "array"
			s["items"] = schema(t.Elem())
		}
	case reflect.String:
		s["type"] = "string"
	case reflect.Bool:
		s["type"] = "boolean"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		s["type"] = "integer"
	case reflect.Float32, reflect.Float64:
		s["type"] = "number"
	}
	return s
}
