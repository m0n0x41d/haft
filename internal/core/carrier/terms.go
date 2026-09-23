package carrier

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

// TermMap is auxiliary interpretation data; it is deliberately not a Record.
type TermMap struct {
	Format string `yaml:"format" json:"format"`
	Terms  []Term `yaml:"terms" json:"terms"`
	Extra  Extra  `yaml:",inline" json:"extra,omitempty"`
}
type Term struct {
	ID         string   `yaml:"id" json:"id"`
	Definition string   `yaml:"definition" json:"definition"`
	Aliases    []string `yaml:"aliases,omitempty" json:"aliases,omitempty"`
	Exclusions []string `yaml:"exclusions,omitempty" json:"exclusions,omitempty"`
	Extra      Extra    `yaml:",inline" json:"extra,omitempty"`
}
type TermsDocument struct {
	Raw         []byte       `json:"raw"`
	Body        []byte       `json:"body"`
	Terms       TermMap      `json:"terms"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

var termIDPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]*(?:\.[A-Za-z][A-Za-z0-9_-]*)+$`)

func ParseTerms(raw []byte) TermsDocument {
	d := TermsDocument{Raw: bytes.Clone(raw)}
	if !utf8.Valid(raw) {
		d.Diagnostics = []Diagnostic{diagnostic("invalid_utf8", "", "Term-map bytes are not UTF-8")}
		return d
	}
	front, body, err := SplitFrontmatter(raw)
	d.Body = bytes.Clone(body)
	if err != nil {
		d.Diagnostics = []Diagnostic{diagnostic("frontmatter", "", err.Error())}
		return d
	}
	n, ds := ParseYAML(front)
	d.Diagnostics = ds
	if HasErrors(ds) {
		return d
	}
	if err := n.Decode(&d.Terms); err != nil {
		d.Diagnostics = append(d.Diagnostics, diagnostic("invalid_terms", "", err.Error()))
		return d
	}
	d.Diagnostics = append(d.Diagnostics, ValidateTerms(d.Terms)...)
	return d
}
func ValidateTerms(t TermMap) []Diagnostic {
	var ds []Diagnostic
	if t.Format != "haft.terms/1" {
		return []Diagnostic{diagnostic("unsupported_terms_format", "format", "Term-map format must be haft.terms/1")}
	}
	seen := map[string]bool{}
	for i, term := range t.Terms {
		p := fmt.Sprintf("terms[%d]", i)
		if !termIDPattern.MatchString(term.ID) || seen[term.ID] {
			ds = append(ds, diagnostic("invalid_term_id", p+".id", "Term ID must be qualified and unique"))
		}
		seen[term.ID] = true
		if strings.TrimSpace(term.Definition) == "" {
			ds = append(ds, diagnostic("required", p+".definition", "Term definition is required"))
		}
	}
	return ds
}
func EncodeTerms(t TermMap, body []byte) (raw []byte, err error) {
	defer func() {
		if v := recover(); v != nil {
			raw = nil
			err = fmt.Errorf("encode terms: %v", v)
		}
	}()
	front, err := yaml.Marshal(t)
	if err != nil {
		return nil, err
	}
	return JoinFrontmatter(front, body), nil
}
func ValidateTermRefs(r Record, t TermMap) []Diagnostic {
	var ds []Diagnostic
	known := map[string]bool{}
	for _, term := range t.Terms {
		known[term.ID] = true
	}
	check := func(refs []string, p string) {
		for i, ref := range refs {
			if !known[ref] {
				ds = append(ds, diagnostic("unresolved_term", fmt.Sprintf("%s[%d]", p, i), "Explicit term ref is absent from the captured term map: "+ref))
			}
		}
	}
	check(r.Terms, "terms")
	for i, c := range r.Claims {
		check(c.Terms, fmt.Sprintf("claims[%d].terms", i))
	}
	return ds
}
