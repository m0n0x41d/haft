// Package code indexes explicitly captured source bytes. It neither runs a
// compiler nor treats syntactic relations as evidence of a claim's correctness.
package code

import "github.com/m0n0x41d/haft/internal/core/carrier"

const Version = "haft.go-index/1"

type Config struct {
	GOOS           string   `json:"goos"`
	GOARCH         string   `json:"goarch"`
	Toolchain      string   `json:"toolchain"`
	BuildTags      []string `json:"build_tags,omitempty"`
	ToolTags       []string `json:"tool_tags,omitempty"`
	CGOEnabled     bool     `json:"cgo_enabled"`
	IncludeTests   bool     `json:"include_tests"`
	IgnorePatterns []string `json:"ignore_patterns,omitempty"`
}
type Diagnostic = carrier.Diagnostic
type File struct {
	Path         string   `json:"path"`
	Digest       string   `json:"digest"`
	SyntaxDigest string   `json:"syntax_digest,omitempty"`
	Package      string   `json:"package,omitempty"`
	Imports      []string `json:"imports,omitempty"`
	Status       string   `json:"status"`
	Reason       string   `json:"reason,omitempty"`
	Raw          []byte   `json:"-"`
}
type Symbol struct {
	Anchor          string `json:"anchor"`
	Path            string `json:"path"`
	Name            string `json:"name"`
	QualifiedName   string `json:"qualified_name"`
	Kind            string `json:"kind"`
	SignatureDigest string `json:"signature_digest"`
	SyntaxDigest    string `json:"syntax_digest"`
	RawDigest       string `json:"raw_digest"`
	StartByte       int    `json:"start_byte"`
	EndByte         int    `json:"end_byte"`
	Line            int    `json:"line"`
	EndLine         int    `json:"end_line"`
}
type Package struct {
	ID                   string   `json:"id"`
	Directory            string   `json:"directory"`
	Name                 string   `json:"name"`
	ImportPath           string   `json:"import_path"`
	Files                []string `json:"files"`
	Imports              []string `json:"imports"`
	LocalDependencies    []string `json:"local_dependencies"`
	ExternalDependencies []string `json:"external_dependencies"`
	ReverseImports       []string `json:"reverse_imports"`
}
type Index struct {
	Version     string       `json:"version"`
	Basis       string       `json:"basis"`
	Config      Config       `json:"config"`
	Complete    bool         `json:"complete"`
	Files       []File       `json:"files"`
	Symbols     []Symbol     `json:"symbols"`
	Packages    []Package    `json:"packages"`
	Diagnostics []Diagnostic `json:"diagnostics,omitempty"`
}
type Resolution struct {
	CodeSelector string       `json:"code_selector,omitempty"`
	Kind         string       `json:"kind"`
	Selector     string       `json:"selector"`
	Basis        string       `json:"basis"`
	Complete     bool         `json:"complete"`
	Files        []string     `json:"files,omitempty"`
	Candidates   []Symbol     `json:"candidates,omitempty"`
	Diagnostics  []Diagnostic `json:"diagnostics,omitempty"`
}
type RelatedResult struct {
	Resolution           Resolution `json:"resolution"`
	Basis                string     `json:"basis"`
	Complete             bool       `json:"complete"`
	DependencyFiles      []string   `json:"dependency_files"`
	AffectedFiles        []string   `json:"affected_files"`
	ExternalDependencies []string   `json:"external_dependencies"`
	Limits               []string   `json:"limits"`
}
type Binding struct {
	MatchKind  string     `json:"match_kind,omitempty"`
	Claim      string     `json:"claim"`
	Kind       string     `json:"kind"`
	Ref        string     `json:"ref"`
	Covers     string     `json:"covers"`
	Conditions string     `json:"conditions,omitempty"`
	Resolution Resolution `json:"resolution"`
}
type Change struct {
	Selector             string        `json:"selector"`
	Kind                 string        `json:"kind"`
	Before               Resolution    `json:"before"`
	After                Resolution    `json:"after"`
	EvidenceBasisChanged bool          `json:"evidence_basis_changed"`
	Affected             RelatedResult `json:"affected"`
}
