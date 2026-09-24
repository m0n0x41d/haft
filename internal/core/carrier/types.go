// Package carrier implements Haft's byte-preserving record model. It performs no
// IO and makes no claim about the truth, authority, or semantic adequacy of text.
package carrier

type Diagnostic struct {
	Code     string `json:"code" yaml:"code"`
	Path     string `json:"path,omitempty" yaml:"path,omitempty"`
	Message  string `json:"message" yaml:"message"`
	Severity string `json:"severity" yaml:"severity"`
}

// Extra fields remain data, never implicitly interpreted control instructions.
type Extra map[string]any

type Record struct {
	Format              string         `yaml:"format" json:"format"`
	ID                  string         `yaml:"id" json:"id"`
	Kind                string         `yaml:"kind" json:"kind"`
	Title               string         `yaml:"title" json:"title"`
	Status              string         `yaml:"status" json:"status"`
	Origin              string         `yaml:"origin" json:"origin"`
	About               string         `yaml:"about" json:"about"`
	CreatedAt           string         `yaml:"created_at" json:"created_at"`
	UpdatedAt           string         `yaml:"updated_at,omitempty" json:"updated_at,omitempty"`
	ReopenWhen          string         `yaml:"reopen_when,omitempty" json:"reopen_when,omitempty"`
	OperatorConfirmed   bool           `yaml:"operator_confirmed,omitempty" json:"operator_confirmed,omitempty"`
	WriteReceipt        *WriteReceipt  `yaml:"write_receipt,omitempty" json:"write_receipt,omitempty"`
	Supersedes          []string       `yaml:"supersedes,omitempty" json:"supersedes,omitempty"`
	SupersedeReason     string         `yaml:"supersede_reason,omitempty" json:"supersede_reason,omitempty"`
	Links               []Link         `yaml:"links,omitempty" json:"links,omitempty"`
	Sources             []Source       `yaml:"sources,omitempty" json:"sources,omitempty"`
	Constrains          []string       `yaml:"constrains,omitempty" json:"constrains,omitempty"`
	Object              string         `yaml:"object,omitempty" json:"object,omitempty"`
	Question            string         `yaml:"question,omitempty" json:"question,omitempty"`
	Disposition         string         `yaml:"disposition,omitempty" json:"disposition,omitempty"`
	Chosen              string         `yaml:"chosen,omitempty" json:"chosen,omitempty"`
	Why                 string         `yaml:"why,omitempty" json:"why,omitempty"`
	Options             []Option       `yaml:"options,omitempty" json:"options,omitempty"`
	OptionsRef          string         `yaml:"options_ref,omitempty" json:"options_ref,omitempty"`
	NoAlternativeReason string         `yaml:"no_alternative_reason,omitempty" json:"no_alternative_reason,omitempty"`
	ChoiceRule          string         `yaml:"choice_rule,omitempty" json:"choice_rule,omitempty"`
	WeakestLink         string         `yaml:"weakest_link,omitempty" json:"weakest_link,omitempty"`
	Invariants          []string       `yaml:"invariants,omitempty" json:"invariants,omitempty"`
	Predictions         []string       `yaml:"predictions,omitempty" json:"predictions,omitempty"`
	Probe               string         `yaml:"probe,omitempty" json:"probe,omitempty"`
	RerouteTo           string         `yaml:"reroute_to,omitempty" json:"reroute_to,omitempty"`
	Signal              string         `yaml:"signal,omitempty" json:"signal,omitempty"`
	Acceptance          string         `yaml:"acceptance,omitempty" json:"acceptance,omitempty"`
	NextUse             string         `yaml:"next_use,omitempty" json:"next_use,omitempty"`
	Comparison          *Comparison    `yaml:"comparison,omitempty" json:"comparison,omitempty"`
	Subject             string         `yaml:"subject,omitempty" json:"subject,omitempty"`
	Slug                string         `yaml:"slug,omitempty" json:"slug,omitempty"`
	Aliases             []string       `yaml:"aliases,omitempty" json:"aliases,omitempty"`
	Terms               []string       `yaml:"terms,omitempty" json:"terms,omitempty"`
	Claims              []Claim        `yaml:"claims,omitempty" json:"claims,omitempty"`
	RetiredClaimIDs     []string       `yaml:"retired_claim_ids,omitempty" json:"retired_claim_ids,omitempty"`
	Retirement          *Retirement    `yaml:"retirement,omitempty" json:"retirement,omitempty"`
	ReceivingUse        string         `yaml:"receiving_use,omitempty" json:"receiving_use,omitempty"`
	Claim               string         `yaml:"claim,omitempty" json:"claim,omitempty"`
	ObservedAt          string         `yaml:"observed_at,omitempty" json:"observed_at,omitempty"`
	Method              string         `yaml:"method,omitempty" json:"method,omitempty"`
	Source              string         `yaml:"source,omitempty" json:"source,omitempty"`
	Basis               *EvidenceBasis `yaml:"basis,omitempty" json:"basis,omitempty"`
	Uses                []EvidenceUse  `yaml:"uses,omitempty" json:"uses,omitempty"`
	LegacyStatus        string         `yaml:"legacy_status,omitempty" json:"legacy_status,omitempty"`
	Legacy              Extra          `yaml:"legacy,omitempty" json:"legacy,omitempty"`
	Extra               Extra          `yaml:",inline" json:"extra,omitempty"`
}

type WriteReceipt struct {
	RequestID     string `yaml:"request_id" json:"request_id"`
	PayloadDigest string `yaml:"payload_digest" json:"payload_digest"`
	Extra         Extra  `yaml:",inline" json:"extra,omitempty"`
}
type Link struct {
	Kind       string `yaml:"kind" json:"kind"`
	Target     string `yaml:"target" json:"target"`
	Reason     string `yaml:"reason,omitempty" json:"reason,omitempty"`
	LegacyType string `yaml:"legacy_type,omitempty" json:"legacy_type,omitempty"`
	Extra      Extra  `yaml:",inline" json:"extra,omitempty"`
}
type Option struct {
	ID          string `yaml:"id" json:"id"`
	Summary     string `yaml:"summary,omitempty" json:"summary,omitempty"`
	Verdict     string `yaml:"verdict,omitempty" json:"verdict,omitempty"`
	Reason      string `yaml:"reason,omitempty" json:"reason,omitempty"`
	WeakestLink string `yaml:"weakest_link,omitempty" json:"weakest_link,omitempty"`
	Extra       Extra  `yaml:",inline" json:"extra,omitempty"`
}
type Comparison struct {
	Characteristics []string `yaml:"characteristics,omitempty" json:"characteristics,omitempty"`
	Basis           string   `yaml:"basis,omitempty" json:"basis,omitempty"`
	Comparator      string   `yaml:"comparator,omitempty" json:"comparator,omitempty"`
	NonDominated    []string `yaml:"non_dominated,omitempty" json:"non_dominated,omitempty"`
	Extra           Extra    `yaml:",inline" json:"extra,omitempty"`
}
type Claim struct {
	ID             string          `yaml:"id" json:"id"`
	Kind           string          `yaml:"kind" json:"kind"`
	Text           string          `yaml:"text" json:"text"`
	Refs           []string        `yaml:"refs,omitempty" json:"refs,omitempty"`
	Terms          []string        `yaml:"terms,omitempty" json:"terms,omitempty"`
	Checks         []Binding       `yaml:"checks,omitempty" json:"checks,omitempty"`
	ImplementedBy  []Binding       `yaml:"implemented_by,omitempty" json:"implemented_by,omitempty"`
	Unchecked      string          `yaml:"unchecked,omitempty" json:"unchecked,omitempty"`
	Examples       []Example       `yaml:"examples,omitempty" json:"examples,omitempty"`
	EvidenceInputs []EvidenceInput `yaml:"evidence_inputs,omitempty" json:"evidence_inputs,omitempty"`
	Extra          Extra           `yaml:",inline" json:"-"`
}
type Binding struct {
	Ref                 string `yaml:"ref" json:"ref"`
	Covers              string `yaml:"covers" json:"covers"`
	Conditions          string `yaml:"conditions,omitempty" json:"conditions,omitempty"`
	InterpretationBasis any    `yaml:"interpretation_basis,omitempty" json:"interpretation_basis,omitempty"`
	Extra               Extra  `yaml:",inline" json:"-"`
}
type Example struct {
	ID    string `yaml:"id" json:"id"`
	Given string `yaml:"given,omitempty" json:"given,omitempty"`
	When  string `yaml:"when,omitempty" json:"when,omitempty"`
	Then  string `yaml:"then,omitempty" json:"then,omitempty"`
	Text  string `yaml:"text,omitempty" json:"text,omitempty"`
	Extra Extra  `yaml:",inline" json:"-"`
}
type EvidenceInput struct {
	Ref           string `yaml:"ref" json:"ref"`
	Applicability string `yaml:"applicability" json:"applicability"`
	Extra         Extra  `yaml:",inline" json:"-"`
}
type Retirement struct {
	Reason string `yaml:"reason" json:"reason"`
	Extra  Extra  `yaml:",inline" json:"extra,omitempty"`
}
type EvidenceBasis struct {
	Kind       string `yaml:"kind" json:"kind"`
	Ref        string `yaml:"ref" json:"ref"`
	Conditions string `yaml:"conditions,omitempty" json:"conditions,omitempty"`
	Extra      Extra  `yaml:",inline" json:"extra,omitempty"`
}
type EvidenceUse struct {
	ID           string `yaml:"id" json:"id"`
	Target       string `yaml:"target" json:"target"`
	Check        string `yaml:"check,omitempty" json:"check,omitempty"`
	Polarity     string `yaml:"polarity" json:"polarity"`
	Scope        string `yaml:"scope" json:"scope"`
	Disposition  string `yaml:"disposition,omitempty" json:"disposition,omitempty"`
	ReceivingUse string `yaml:"receiving_use,omitempty" json:"receiving_use,omitempty"`
	Window       any    `yaml:"window,omitempty" json:"window,omitempty"`
	ReopenWhen   string `yaml:"reopen_when,omitempty" json:"reopen_when,omitempty"`
	Extra        Extra  `yaml:",inline" json:"extra,omitempty"`
}
type Source struct {
	Ref             string         `yaml:"ref" json:"ref"`
	SourceRevision  SourceRevision `yaml:"source_revision" json:"source_revision"`
	PublicationPath string         `yaml:"publication_path,omitempty" json:"publication_path,omitempty"`
	BodyDigest      string         `yaml:"body_digest,omitempty" json:"body_digest,omitempty"`
	Lines           []int          `yaml:"lines,omitempty" json:"lines,omitempty"`
	SnapshotRef     string         `yaml:"snapshot_ref,omitempty" json:"snapshot_ref,omitempty"`
	Extra           Extra          `yaml:",inline" json:"extra,omitempty"`
}
type SourceRevision struct {
	Kind       string `yaml:"kind" json:"kind"`
	Repository string `yaml:"repository,omitempty" json:"repository,omitempty"`
	Commit     string `yaml:"commit,omitempty" json:"commit,omitempty"`
	TreeDigest string `yaml:"tree_digest,omitempty" json:"tree_digest,omitempty"`
	Reason     string `yaml:"reason,omitempty" json:"reason,omitempty"`
	Extra      Extra  `yaml:",inline" json:"extra,omitempty"`
}
