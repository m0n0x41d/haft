package carrier

import (
	"bytes"
	"fmt"
	"io"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

type Document struct {
	Raw    []byte `json:"raw"`
	Body   []byte `json:"body"`
	Record Record `json:"record"`
	// Edition is supplied by the capture layer when auxiliary interpretation
	// bases participate. An empty value denotes the default absent-basis edition.
	Edition     string       `json:"edition,omitempty"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

func HasErrors(ds []Diagnostic) bool {
	for _, d := range ds {
		if d.Severity == "error" {
			return true
		}
	}
	return false
}
func (d Document) Valid() bool { return !HasErrors(d.Diagnostics) }

// Bytes returns original bytes, including comments, whitespace and unknown data.
// To mutate content callers explicitly use Encode or Normalize.
func (d Document) Bytes() []byte { return bytes.Clone(d.Raw) }

func Parse(raw []byte) Document {
	d := Document{Raw: bytes.Clone(raw)}
	front, body, err := SplitFrontmatter(raw)
	d.Body = bytes.Clone(body)
	if err != nil {
		d.Diagnostics = append(d.Diagnostics, diagnostic("frontmatter", "", err.Error()))
		return d
	}
	if !utf8.Valid(raw) {
		d.Diagnostics = append(d.Diagnostics, diagnostic("invalid_utf8", "", "Carrier bytes are not UTF-8; preserved for historical reading"))
		return d
	}
	n, ds := ParseYAML(front)
	d.Diagnostics = append(d.Diagnostics, ds...)
	if HasErrors(ds) {
		return d
	}
	if err := n.Decode(&d.Record); err != nil {
		d.Diagnostics = append(d.Diagnostics, diagnostic("invalid_fields", "", err.Error()))
		return d
	}
	d.Diagnostics = append(d.Diagnostics, Validate(d.Record, false)...)
	return d
}

// SplitFrontmatter only recognizes a delimiter occupying a complete line.
func SplitFrontmatter(raw []byte) ([]byte, []byte, error) {
	lineEnd := bytes.IndexByte(raw, '\n')
	if lineEnd < 0 || string(bytes.TrimSuffix(raw[:lineEnd], []byte{'\r'})) != "---" {
		return nil, raw, fmt.Errorf("missing opening frontmatter delimiter")
	}
	start := lineEnd + 1
	for offset := start; offset <= len(raw); {
		n := bytes.IndexByte(raw[offset:], '\n')
		end := len(raw)
		if n >= 0 {
			end = offset + n
		}
		line := bytes.TrimSuffix(raw[offset:end], []byte{'\r'})
		if bytes.Equal(line, []byte("---")) {
			bodyStart := end
			if n >= 0 {
				bodyStart++
			}
			return raw[start:offset], raw[bodyStart:], nil
		}
		if n < 0 {
			break
		}
		offset = end + 1
	}
	return nil, raw, fmt.Errorf("missing closing frontmatter delimiter")
}

// ParseYAML rejects duplicate keys, aliases and multiple documents before typed
// decoding; a merge key cannot secretly supply controlling fields.
func ParseYAML(raw []byte) (*yaml.Node, []Diagnostic) {
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	var n yaml.Node
	if err := dec.Decode(&n); err != nil {
		return nil, []Diagnostic{diagnostic("invalid_yaml", "", err.Error())}
	}
	var next yaml.Node
	if err := dec.Decode(&next); err != io.EOF {
		return nil, []Diagnostic{diagnostic("multiple_yaml_documents", "", "Exactly one frontmatter mapping is required")}
	}
	if len(n.Content) != 1 || n.Content[0].Kind != yaml.MappingNode {
		return nil, []Diagnostic{diagnostic("invalid_yaml", "", "Frontmatter must be a mapping")}
	}
	var ds []Diagnostic
	var walk func(*yaml.Node, string)
	walk = func(node *yaml.Node, p string) {
		if node.Kind == yaml.AliasNode {
			ds = append(ds, diagnostic("yaml_alias", p, "YAML aliases are not interpreted"))
			return
		}
		if node.Kind == yaml.MappingNode {
			seen := map[string]bool{}
			for i := 0; i < len(node.Content); i += 2 {
				k, v := node.Content[i], node.Content[i+1]
				kp := k.Value
				if p != "" {
					kp = p + "." + kp
				}
				if k.Kind != yaml.ScalarNode || k.Tag != "!!str" {
					ds = append(ds, diagnostic("invalid_yaml_key", kp, "Mapping keys must be strings"))
				}
				if seen[k.Value] {
					ds = append(ds, diagnostic("duplicate_key", kp, "Duplicate YAML key"))
				}
				seen[k.Value] = true
				if k.Value == "<<" {
					ds = append(ds, diagnostic("yaml_merge", kp, "YAML merge keys are not interpreted"))
				}
				walk(v, kp)
			}
		} else {
			for i, child := range node.Content {
				walk(child, fmt.Sprintf("%s[%d]", p, i))
			}
		}
	}
	walk(n.Content[0], "")
	return &n, ds
}

// Encode is an explicit normalization/mutation. Unknown fields are preserved in
// Extra maps; original formatting is available only through Document.Bytes.
func Encode(r Record, body []byte) (raw []byte, err error) {
	// yaml.v3 panics for colliding inline keys. Convert that into a normal error.
	defer func() {
		if v := recover(); v != nil {
			raw = nil
			err = fmt.Errorf("encode: %v", v)
		}
	}()
	front, err := yaml.Marshal(r)
	if err != nil {
		return nil, err
	}
	return JoinFrontmatter(front, body), nil
}
func JoinFrontmatter(front, body []byte) []byte {
	var b bytes.Buffer
	b.WriteString("---\n")
	b.Write(front)
	if len(front) > 0 && front[len(front)-1] != '\n' {
		b.WriteByte('\n')
	}
	b.WriteString("---\n")
	b.Write(body)
	return b.Bytes()
}
func Normalize(d Document) ([]byte, error) {
	if !d.Valid() {
		return nil, fmt.Errorf("invalid document cannot be normalized")
	}
	return Encode(d.Record, d.Body)
}
func diagnostic(code, path, message string) Diagnostic {
	return Diagnostic{Code: code, Path: path, Message: message, Severity: "error"}
}
