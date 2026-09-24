package carrier

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// Claims and their nested authoring objects use one field namespace in JSON
// and YAML. A recalled claim can therefore be copied into change frontmatter
// or decoded into a typed revision without hiding extensions in an envelope.
// Other carrier JSON encodings, notably historical source snapshots, keep
// their existing representation. A literal extension named "extra" is data.
type claimJSON Claim
type bindingJSON Binding
type exampleJSON Example
type evidenceInputJSON EvidenceInput

func (c Claim) MarshalJSON() ([]byte, error) {
	return marshalInlineJSON(claimJSON(c), c.Extra)
}
func (c *Claim) UnmarshalJSON(raw []byte) error {
	var value claimJSON
	if err := decodeInlineJSON(raw, &value); err != nil {
		return err
	}
	*c = Claim(value)
	return nil
}
func (b Binding) MarshalJSON() ([]byte, error) {
	return marshalInlineJSON(bindingJSON(b), b.Extra)
}
func (b *Binding) UnmarshalJSON(raw []byte) error {
	var value bindingJSON
	if err := decodeInlineJSON(raw, &value); err != nil {
		return err
	}
	*b = Binding(value)
	return nil
}
func (e Example) MarshalJSON() ([]byte, error) {
	return marshalInlineJSON(exampleJSON(e), e.Extra)
}
func (e *Example) UnmarshalJSON(raw []byte) error {
	var value exampleJSON
	if err := decodeInlineJSON(raw, &value); err != nil {
		return err
	}
	*e = Example(value)
	return nil
}
func (e EvidenceInput) MarshalJSON() ([]byte, error) {
	return marshalInlineJSON(evidenceInputJSON(e), e.Extra)
}
func (e *EvidenceInput) UnmarshalJSON(raw []byte) error {
	var value evidenceInputJSON
	if err := decodeInlineJSON(raw, &value); err != nil {
		return err
	}
	*e = EvidenceInput(value)
	return nil
}

func marshalInlineJSON(value any, extra Extra) ([]byte, error) {
	// Reserve even omitted known fields, just as yaml.v3's inline map does.
	// Never let an extension override an ID, selector or other typed field.
	typ := reflect.TypeOf(value)
	for i := 0; i < typ.NumField(); i++ {
		name := strings.Split(typ.Field(i).Tag.Get("json"), ",")[0]
		if name != "-" {
			if _, exists := extra[name]; exists {
				return nil, fmt.Errorf("extension collides with known field %q", name)
			}
		}
	}
	raw, err := json.Marshal(value)
	if err != nil || len(extra) == 0 {
		return raw, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, err
	}
	for name, value := range extra {
		fields[name], err = json.Marshal(value)
		if err != nil {
			return nil, err
		}
	}
	return json.Marshal(fields)
}

func decodeInlineJSON(raw []byte, target any) error {
	if !json.Valid(raw) {
		return fmt.Errorf("invalid carrier JSON")
	}
	// JSON is a supported frontmatter spelling. Decode through the same typed
	// YAML mapping so unknown fields, integer values and explicit empty lists
	// have identical meaning. ParseYAML rejects duplicates at every depth.
	node, ds := ParseYAML(raw)
	if HasErrors(ds) {
		return fmt.Errorf("carrier JSON: %v", ds)
	}
	// Keep JSON's known-field type checks; YAML otherwise permits scalar
	// coercions such as a number into a string. target is a method-free alias.
	// Filter exact names first: encoding/json's case-folded matching must not
	// promote an unrelated YAML extension (e.g. Conditions) into a typed field.
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return err
	}
	known := map[string]json.RawMessage{}
	typ := reflect.TypeOf(target).Elem()
	for i := 0; i < typ.NumField(); i++ {
		name := strings.Split(typ.Field(i).Tag.Get("json"), ",")[0]
		if value, exists := fields[name]; name != "-" && exists {
			known[name] = value
		}
	}
	checked, err := json.Marshal(known)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(checked, target); err != nil {
		return err
	}
	return node.Decode(target)
}
