package specflow

import (
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/project"
)

const (
	SpecificationUseRecordSchemaVersion = 1
	SpecificationUseRecordKind          = "specification_use_record"
	SpecificationUseAuthority           = "advisory_spec_use_record"

	SpecUsePolicyDocumentaryOnly                  = "documentary_only"
	SpecUsePolicyStrongerUseRequiresCurrentSource = "stronger_use_requires_current_source"
	SpecUsePolicyTemporaryWaiver                  = "temporary_waiver"

	SpecUseDispositionAdmitted = "admitted"
	SpecUseDispositionBlocked  = "blocked"
	SpecUseDispositionWaived   = "waived"

	SpecUseReadingStrongerUseAdmitted = "stronger_use_admitted_for_declared_context"
	SpecUseReadingTemporaryWaiver     = "temporary_waiver_for_declared_context"

	SpecUseBaselineUnknown = "unknown"
	SpecUseBaselineCurrent = "current"
	SpecUseBaselineMissing = "missing_baseline"
	SpecUseBaselineDrifted = "drifted"
	SpecUseBaselineError   = "baseline_lookup_error"

	SpecUseGateDecisionNotApplicable = "not_applicable_no_operational_gate"
)

type SpecificationUseInput struct {
	SectionID       string
	UseContext      string
	Policy          string
	WaiverExpiresAt string
	Now             time.Time
}

type SpecificationUseBaselineInput struct {
	ProjectID string
	Status    string
	Baseline  SectionBaseline
	Error     string
}

type SpecificationUseRecord struct {
	SchemaVersion       int                             `json:"schema_version"`
	RecordKind          string                          `json:"record_kind"`
	Authority           string                          `json:"authority"`
	SectionID           string                          `json:"section_id"`
	SourceEdition       SpecificationUseSourceEdition   `json:"source_edition"`
	UseContext          SpecificationUseContext         `json:"use_context"`
	Policy              SpecificationUsePolicy          `json:"policy"`
	BaselineCurrentness SpecificationUseCurrentness     `json:"baseline_currentness"`
	Admission           SpecificationUseAdmission       `json:"admission"`
	GateDecision        SpecificationUseGateDisposition `json:"gate_decision"`
}

type SpecificationUseSourceEdition struct {
	SectionID   string `json:"section_id"`
	Hash        string `json:"hash"`
	Status      string `json:"status"`
	ValidUntil  string `json:"valid_until,omitempty"`
	CarrierPath string `json:"carrier_path,omitempty"`
	CarrierLine int    `json:"carrier_line,omitempty"`
}

type SpecificationUseContext struct {
	Name string `json:"name"`
}

type SpecificationUsePolicy struct {
	Name            string `json:"name"`
	WaiverExpiresAt string `json:"waiver_expires_at,omitempty"`
}

type SpecificationUseCurrentness struct {
	Status       string `json:"status"`
	ProjectID    string `json:"project_id,omitempty"`
	CurrentHash  string `json:"current_hash"`
	BaselineHash string `json:"baseline_hash,omitempty"`
	CapturedAt   string `json:"captured_at,omitempty"`
	ApprovedBy   string `json:"approved_by,omitempty"`
	Error        string `json:"error,omitempty"`
}

type SpecificationUseAdmission struct {
	Disposition string `json:"disposition"`
	Reason      string `json:"reason"`
	StrongerUse string `json:"stronger_use"`
}

type SpecificationUseGateDisposition struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
}

func BuildSpecificationUseRecord(
	section project.SpecSection,
	baseline SpecificationUseBaselineInput,
	input SpecificationUseInput,
) SpecificationUseRecord {
	normalized := normalizeSpecificationUseInput(input)
	sourceEdition := specificationUseSourceEdition(section)
	currentness := specificationUseCurrentness(sourceEdition.Hash, baseline)
	policy := specificationUsePolicy(normalized)

	return SpecificationUseRecord{
		SchemaVersion:       SpecificationUseRecordSchemaVersion,
		RecordKind:          SpecificationUseRecordKind,
		Authority:           SpecificationUseAuthority,
		SectionID:           strings.TrimSpace(section.ID),
		SourceEdition:       sourceEdition,
		UseContext:          SpecificationUseContext{Name: normalized.UseContext},
		Policy:              policy,
		BaselineCurrentness: currentness,
		Admission:           specificationUseAdmission(section, normalized, policy, currentness),
		GateDecision: SpecificationUseGateDisposition{
			Status: SpecUseGateDecisionNotApplicable,
			Reason: "SpecificationUseRecord is not a GateDecision because no OperationalGate profile is attached.",
		},
	}
}

func normalizeSpecificationUseInput(input SpecificationUseInput) SpecificationUseInput {
	return SpecificationUseInput{
		SectionID:       strings.TrimSpace(input.SectionID),
		UseContext:      strings.TrimSpace(input.UseContext),
		Policy:          strings.TrimSpace(input.Policy),
		WaiverExpiresAt: strings.TrimSpace(input.WaiverExpiresAt),
		Now:             input.Now,
	}
}

func specificationUseSourceEdition(section project.SpecSection) SpecificationUseSourceEdition {
	return SpecificationUseSourceEdition{
		SectionID:   strings.TrimSpace(section.ID),
		Hash:        HashSection(section),
		Status:      strings.TrimSpace(section.Status),
		ValidUntil:  strings.TrimSpace(section.ValidUntil),
		CarrierPath: strings.TrimSpace(section.Path),
		CarrierLine: section.Line,
	}
}

func specificationUsePolicy(input SpecificationUseInput) SpecificationUsePolicy {
	return SpecificationUsePolicy{
		Name:            input.Policy,
		WaiverExpiresAt: input.WaiverExpiresAt,
	}
}

func specificationUseCurrentness(
	currentHash string,
	baseline SpecificationUseBaselineInput,
) SpecificationUseCurrentness {
	status := strings.TrimSpace(baseline.Status)
	if status == "" {
		status = SpecUseBaselineUnknown
	}

	currentness := SpecificationUseCurrentness{
		Status:      status,
		ProjectID:   strings.TrimSpace(baseline.ProjectID),
		CurrentHash: currentHash,
		Error:       strings.TrimSpace(baseline.Error),
	}
	if status == SpecUseBaselineMissing || status == SpecUseBaselineUnknown || status == SpecUseBaselineError {
		return currentness
	}

	currentness.BaselineHash = strings.TrimSpace(baseline.Baseline.Hash)
	currentness.ApprovedBy = strings.TrimSpace(baseline.Baseline.ApprovedBy)
	if !baseline.Baseline.CapturedAt.IsZero() {
		currentness.CapturedAt = baseline.Baseline.CapturedAt.UTC().Format(time.RFC3339)
	}

	return currentness
}

func specificationUseAdmission(
	section project.SpecSection,
	input SpecificationUseInput,
	policy SpecificationUsePolicy,
	currentness SpecificationUseCurrentness,
) SpecificationUseAdmission {
	if strings.TrimSpace(section.Status) != string(project.SpecSectionStateActive) {
		return blockedSpecUseAdmission("section_not_active", ReviewUseBlockedForStrongerUse)
	}
	if input.UseContext == "" {
		return blockedSpecUseAdmission("use_context_required", ReviewUseAbstainUntilClarified)
	}

	switch policy.Name {
	case SpecUsePolicyDocumentaryOnly:
		return SpecificationUseAdmission{
			Disposition: SpecUseDispositionAdmitted,
			Reason:      "documentary_only policy admits reading the section as description, not evidence, approval, gate passage, or execution authority",
			StrongerUse: ReviewUseDocumentaryReading,
		}
	case SpecUsePolicyStrongerUseRequiresCurrentSource:
		return specificationUseCurrentSourceAdmission(currentness)
	case SpecUsePolicyTemporaryWaiver:
		return specificationUseTemporaryWaiverAdmission(policy, input.Now)
	case "":
		return blockedSpecUseAdmission("policy_required", ReviewUseAbstainUntilClarified)
	default:
		return blockedSpecUseAdmission("unknown_policy", ReviewUseAbstainUntilClarified)
	}
}

func specificationUseCurrentSourceAdmission(currentness SpecificationUseCurrentness) SpecificationUseAdmission {
	if currentness.Status != SpecUseBaselineCurrent {
		return blockedSpecUseAdmission("source_edition_not_current", ReviewUseBlockedForStrongerUse)
	}

	return SpecificationUseAdmission{
		Disposition: SpecUseDispositionAdmitted,
		Reason:      "policy admits stronger use only because baseline_currentness is current; this is still not a GateDecision",
		StrongerUse: SpecUseReadingStrongerUseAdmitted,
	}
}

func specificationUseTemporaryWaiverAdmission(
	policy SpecificationUsePolicy,
	now time.Time,
) SpecificationUseAdmission {
	if !specUseWaiverActive(policy.WaiverExpiresAt, now) {
		return blockedSpecUseAdmission("waiver_expiry_required_or_expired", ReviewUseBlockedForStrongerUse)
	}

	return SpecificationUseAdmission{
		Disposition: SpecUseDispositionWaived,
		Reason:      "temporary waiver admits attempted use only until waiver_expires_at; currentness and gate decision remain separate",
		StrongerUse: SpecUseReadingTemporaryWaiver,
	}
}

func blockedSpecUseAdmission(reason string, strongerUse string) SpecificationUseAdmission {
	return SpecificationUseAdmission{
		Disposition: SpecUseDispositionBlocked,
		Reason:      reason,
		StrongerUse: strongerUse,
	}
}

func specUseWaiverActive(raw string, now time.Time) bool {
	expiresAt, ok := parseSpecUseExpiry(raw)
	if !ok {
		return false
	}
	if now.IsZero() {
		return true
	}

	return expiresAt.After(now)
}

func parseSpecUseExpiry(raw string) (time.Time, bool) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return time.Time{}, false
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed, true
	}
	if parsed, err := time.Parse("2006-01-02", value); err == nil {
		return parsed, true
	}

	return time.Time{}, false
}
