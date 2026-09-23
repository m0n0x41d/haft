// Package change computes specification deltas over exact captured editions.
// It has no filesystem, transport, clock, random-ID or execution dependencies.
package change

import "github.com/m0n0x41d/haft/internal/core/carrier"

const Format = "haft.change/1"

type Change struct {
	Format             string                `yaml:"format" json:"format"`
	ID                 string                `yaml:"id" json:"id"`
	ChangeKey          string                `yaml:"change_key" json:"change_key"`
	Title              string                `yaml:"title" json:"title"`
	Intent             string                `yaml:"intent" json:"intent"`
	State              string                `yaml:"state" json:"state"`
	CreatedAt          string                `yaml:"created_at" json:"created_at"`
	Supersedes         []string              `yaml:"supersedes,omitempty" json:"supersedes,omitempty"`
	SupersedeReason    string                `yaml:"supersede_reason,omitempty" json:"supersede_reason,omitempty"`
	Rationale          []string              `yaml:"rationale,omitempty" json:"rationale,omitempty"`
	Tasks              []Task                `yaml:"tasks,omitempty" json:"tasks,omitempty"`
	Checks             []string              `yaml:"checks,omitempty" json:"checks,omitempty"`
	Evidence           []string              `yaml:"evidence,omitempty" json:"evidence,omitempty"`
	Patches            []SectionPatch        `yaml:"patches,omitempty" json:"patches,omitempty"`
	NoSpecChangeReason string                `yaml:"no_spec_change_reason,omitempty" json:"no_spec_change_reason,omitempty"`
	WriteReceipt       *carrier.WriteReceipt `yaml:"write_receipt,omitempty" json:"write_receipt,omitempty"`
	Extra              carrier.Extra         `yaml:",inline" json:"extra,omitempty"`
}
type Task struct {
	ID      string        `yaml:"id" json:"id"`
	Text    string        `yaml:"text" json:"text"`
	Done    bool          `yaml:"done" json:"done"`
	Results []string      `yaml:"results,omitempty" json:"results,omitempty"`
	Extra   carrier.Extra `yaml:",inline" json:"extra,omitempty"`
}
type SectionPatch struct {
	Base               string        `yaml:"base" json:"base"`
	Operations         []Operation   `yaml:"operations,omitempty" json:"operations,omitempty"`
	Body               *string       `yaml:"body,omitempty" json:"body,omitempty"`
	ExpectedBodyDigest string        `yaml:"expected_body_digest,omitempty" json:"expected_body_digest,omitempty"`
	BodyChangeReason   string        `yaml:"body_change_reason,omitempty" json:"body_change_reason,omitempty"`
	RetireReason       string        `yaml:"retire_reason,omitempty" json:"retire_reason,omitempty"`
	Extra              carrier.Extra `yaml:",inline" json:"extra,omitempty"`
}
type Operation struct {
	Op             string         `yaml:"op" json:"op"`
	ClaimID        string         `yaml:"claim_id,omitempty" json:"claim_id,omitempty"`
	Claim          *carrier.Claim `yaml:"claim,omitempty" json:"claim,omitempty"`
	NewID          string         `yaml:"new_id,omitempty" json:"new_id,omitempty"`
	Reason         string         `yaml:"reason,omitempty" json:"reason,omitempty"`
	RemoveExamples []string       `yaml:"remove_examples,omitempty" json:"remove_examples,omitempty"`
	RemoveFields   []string       `yaml:"remove_fields,omitempty" json:"remove_fields,omitempty"`
	Extra          carrier.Extra  `yaml:",inline" json:"extra,omitempty"`
}
type Document struct {
	Raw         []byte               `json:"raw"`
	Body        []byte               `json:"body"`
	Change      Change               `json:"change"`
	Diagnostics []carrier.Diagnostic `json:"diagnostics,omitempty"`
}

// Basis separates the saved authored edition from the selected current edition.
// CurrentRef is supplied from a transaction-current capture, never inferred from
// a timestamp. More than one current head requires explicit resolution first.
type Basis struct {
	Snapshot   []byte   `json:"snapshot"`
	CurrentRef string   `json:"current_ref"`
	Contested  []string `json:"contested,omitempty"`
}
type Loss struct {
	Base    string `json:"base"`
	ClaimID string `json:"claim_id,omitempty"`
	Kind    string `json:"kind"`
	ID      string `json:"id,omitempty"`
	Before  any    `json:"before"`
	After   any    `json:"after,omitempty"`
	Reason  string `json:"reason"`
}
type Output struct {
	Base string `json:"base"`
	// Successor is a content prototype. ID and timestamps are absent until an
	// effect layer supplies Metadata; it must not be published directly.
	Successor carrier.Record `json:"successor"`
	Body      []byte         `json:"body"`
}
type PreviewResult struct {
	Kind        string               `json:"kind"`
	Outputs     []Output             `json:"outputs"`
	Losses      []Loss               `json:"losses"`
	Diagnostics []carrier.Diagnostic `json:"diagnostics"`
	Digest      string               `json:"digest"`
}
type Metadata struct {
	ID                string
	CreatedAt         string
	Origin            string
	Status            string
	OperatorConfirmed bool
	Supersedes        []string
	SupersedeReason   string
	Receipt           *carrier.WriteReceipt
}
