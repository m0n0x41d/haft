package artifact

import (
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/reff"
)

const (
	EvidencePathRecordSchemaVersion = 1
	EvidencePathRecordKind          = "evidence_path_record"
	EvidencePathAuthority           = "advisory_evidence_reliance_record"

	EvidenceRelianceBounded  = "bounded_reliance"
	EvidenceRelianceAdvisory = "advisory_only"
	EvidenceRelianceBlocked  = "blocked"

	EvidenceCurrentnessCurrent   = "current"
	EvidenceCurrentnessExpired   = "expired"
	EvidenceCurrentnessPerpetual = "perpetual"
	EvidenceCurrentnessUnknown   = "unknown"

	EvidenceClaimBindingBound        = "bound"
	EvidenceClaimBindingNotBound     = "not_bound"
	EvidenceClaimBindingNotRequested = "not_requested"

	EvidenceTraceBindingDeclared = "declared"
	EvidenceTraceBindingMissing  = "missing"

	EvidenceBoundaryNotApproval     = "not_approval"
	EvidenceBoundaryNotGateDecision = "not_gate_decision"
	EvidenceBoundaryNotGlobalTruth  = "not_global_truth"
)

type EvidencePathInput struct {
	ArtifactRef  string
	EvidenceRef  string
	ClaimRef     string
	AttemptedUse string
	ProducerRef  string
	MethodRef    string
	WorkRef      string
}

type EvidencePathRecord struct {
	SchemaVersion       int                       `json:"schema_version"`
	RecordKind          string                    `json:"record_kind"`
	Authority           string                    `json:"authority"`
	ArtifactRef         string                    `json:"artifact_ref"`
	Evidence            EvidencePathEvidence      `json:"evidence"`
	ClaimBinding        EvidenceClaimBinding      `json:"claim_binding"`
	TraceBinding        EvidenceTraceBinding      `json:"trace_binding"`
	CurrentnessWindow   EvidenceCurrentnessWindow `json:"currentness_window"`
	AttemptedUse        EvidenceAttemptedUse      `json:"attempted_use"`
	RelianceDisposition RelianceDisposition       `json:"reliance_disposition"`
	AuthorityBoundary   EvidenceAuthorityBoundary `json:"authority_boundary"`
}

type EvidencePathEvidence struct {
	ID                 string                     `json:"id"`
	Type               string                     `json:"type,omitempty"`
	Verdict            string                     `json:"verdict,omitempty"`
	CarrierRef         string                     `json:"carrier_ref,omitempty"`
	CongruenceLevel    int                        `json:"congruence_level,omitempty"`
	FormalityLevel     int                        `json:"formality_level,omitempty"`
	FormalityScale     *reff.FormalityScale       `json:"formality_scale,omitempty"`
	FormalityBridge    *reff.FormalityBridge      `json:"formality_bridge,omitempty"`
	ClaimRefs          []string                   `json:"claim_refs,omitempty"`
	ClaimScope         []string                   `json:"claim_scope,omitempty"`
	ValidUntil         string                     `json:"valid_until,omitempty"`
	CausalSupportBasis CausalEvidenceSupportBasis `json:"causal_support_basis,omitempty"`
	Provenance         string                     `json:"provenance,omitempty"`
}

type EvidenceClaimBinding struct {
	ClaimRef string `json:"claim_ref,omitempty"`
	Status   string `json:"status"`
}

type EvidenceTraceBinding struct {
	Status      string `json:"status"`
	ProducerRef string `json:"producer_ref,omitempty"`
	MethodRef   string `json:"method_ref,omitempty"`
	WorkRef     string `json:"work_ref,omitempty"`
}

type EvidenceCurrentnessWindow struct {
	Status     string `json:"status"`
	ValidUntil string `json:"valid_until,omitempty"`
	CheckedAt  string `json:"checked_at"`
}

type EvidenceAttemptedUse struct {
	Context string `json:"context,omitempty"`
}

type RelianceDisposition struct {
	Disposition string   `json:"disposition"`
	Reason      string   `json:"reason"`
	Boundaries  []string `json:"boundaries"`
}

type EvidenceAuthorityBoundary struct {
	Approval     string `json:"approval"`
	GateDecision string `json:"gate_decision"`
	GlobalTruth  string `json:"global_truth"`
}

func BuildEvidencePathRecord(
	input EvidencePathInput,
	item EvidenceItem,
	now time.Time,
) EvidencePathRecord {
	normalized := normalizeEvidencePathInput(input)
	evidence := evidencePathEvidence(item)
	claimBinding := evidenceClaimBinding(normalized.ClaimRef, item)
	traceBinding := evidenceTraceBinding(normalized)
	currentness := evidenceCurrentnessWindow(item, now)
	attemptedUse := EvidenceAttemptedUse{Context: normalized.AttemptedUse}

	return EvidencePathRecord{
		SchemaVersion:       EvidencePathRecordSchemaVersion,
		RecordKind:          EvidencePathRecordKind,
		Authority:           EvidencePathAuthority,
		ArtifactRef:         normalized.ArtifactRef,
		Evidence:            evidence,
		ClaimBinding:        claimBinding,
		TraceBinding:        traceBinding,
		CurrentnessWindow:   currentness,
		AttemptedUse:        attemptedUse,
		RelianceDisposition: evidenceRelianceDisposition(item, attemptedUse, claimBinding, traceBinding, currentness),
		AuthorityBoundary: EvidenceAuthorityBoundary{
			Approval:     EvidenceBoundaryNotApproval,
			GateDecision: EvidenceBoundaryNotGateDecision,
			GlobalTruth:  EvidenceBoundaryNotGlobalTruth,
		},
	}
}

func normalizeEvidencePathInput(input EvidencePathInput) EvidencePathInput {
	return EvidencePathInput{
		ArtifactRef:  strings.TrimSpace(input.ArtifactRef),
		EvidenceRef:  strings.TrimSpace(input.EvidenceRef),
		ClaimRef:     strings.TrimSpace(input.ClaimRef),
		AttemptedUse: strings.TrimSpace(input.AttemptedUse),
		ProducerRef:  strings.TrimSpace(input.ProducerRef),
		MethodRef:    strings.TrimSpace(input.MethodRef),
		WorkRef:      strings.TrimSpace(input.WorkRef),
	}
}

func evidencePathEvidence(item EvidenceItem) EvidencePathEvidence {
	return EvidencePathEvidence{
		ID:                 strings.TrimSpace(item.ID),
		Type:               strings.TrimSpace(item.Type),
		Verdict:            strings.TrimSpace(item.Verdict),
		CarrierRef:         strings.TrimSpace(item.CarrierRef),
		CongruenceLevel:    item.CongruenceLevel,
		FormalityLevel:     item.FormalityLevel,
		FormalityScale:     item.FormalityScale,
		FormalityBridge:    item.FormalityBridge,
		ClaimRefs:          append([]string(nil), item.ClaimRefs...),
		ClaimScope:         append([]string(nil), item.ClaimScope...),
		ValidUntil:         strings.TrimSpace(item.ValidUntil),
		CausalSupportBasis: item.CausalSupportBasis,
		Provenance:         strings.TrimSpace(item.Provenance),
	}
}

func evidenceClaimBinding(claimRef string, item EvidenceItem) EvidenceClaimBinding {
	if claimRef == "" {
		return EvidenceClaimBinding{Status: EvidenceClaimBindingNotRequested}
	}
	if evidenceItemBindsClaim(item, claimRef) {
		return EvidenceClaimBinding{ClaimRef: claimRef, Status: EvidenceClaimBindingBound}
	}

	return EvidenceClaimBinding{ClaimRef: claimRef, Status: EvidenceClaimBindingNotBound}
}

func evidenceItemBindsClaim(item EvidenceItem, claimRef string) bool {
	for _, ref := range item.ClaimRefs {
		if strings.TrimSpace(ref) == claimRef {
			return true
		}
	}
	for _, scope := range item.ClaimScope {
		if strings.TrimSpace(scope) == claimRef {
			return true
		}
	}

	return false
}

func evidenceTraceBinding(input EvidencePathInput) EvidenceTraceBinding {
	binding := EvidenceTraceBinding{
		ProducerRef: input.ProducerRef,
		MethodRef:   input.MethodRef,
		WorkRef:     input.WorkRef,
	}
	if input.ProducerRef == "" && input.MethodRef == "" && input.WorkRef == "" {
		binding.Status = EvidenceTraceBindingMissing
		return binding
	}

	binding.Status = EvidenceTraceBindingDeclared
	return binding
}

func evidenceCurrentnessWindow(item EvidenceItem, now time.Time) EvidenceCurrentnessWindow {
	checkedAt := now
	if checkedAt.IsZero() {
		checkedAt = time.Now().UTC()
	}
	window := EvidenceCurrentnessWindow{
		Status:     EvidenceCurrentnessUnknown,
		ValidUntil: strings.TrimSpace(item.ValidUntil),
		CheckedAt:  checkedAt.UTC().Format(time.RFC3339),
	}
	if window.ValidUntil == "" {
		window.Status = EvidenceCurrentnessPerpetual
		return window
	}

	expiresAt, ok := reff.ParseValidUntil(window.ValidUntil)
	if !ok {
		return window
	}
	if expiresAt.Before(checkedAt) {
		window.Status = EvidenceCurrentnessExpired
		return window
	}

	window.Status = EvidenceCurrentnessCurrent
	return window
}

func evidenceRelianceDisposition(
	item EvidenceItem,
	attemptedUse EvidenceAttemptedUse,
	claimBinding EvidenceClaimBinding,
	traceBinding EvidenceTraceBinding,
	currentness EvidenceCurrentnessWindow,
) RelianceDisposition {
	boundaries := []string{
		EvidenceBoundaryNotApproval,
		EvidenceBoundaryNotGateDecision,
		EvidenceBoundaryNotGlobalTruth,
	}
	if strings.TrimSpace(attemptedUse.Context) == "" {
		return evidenceReliance(EvidenceRelianceBlocked, "attempted_use_required", boundaries)
	}
	if traceBinding.Status == EvidenceTraceBindingMissing {
		return evidenceReliance(EvidenceRelianceAdvisory, "trace_refs_missing", boundaries)
	}
	if item.Verdict == "superseded" {
		return evidenceReliance(EvidenceRelianceBlocked, "evidence_superseded", boundaries)
	}
	if item.Verdict == "refutes" || item.Verdict == "failed" {
		return evidenceReliance(EvidenceRelianceBlocked, "evidence_refutes_attempted_use", boundaries)
	}
	if currentness.Status == EvidenceCurrentnessExpired || currentness.Status == EvidenceCurrentnessUnknown {
		return evidenceReliance(EvidenceRelianceBlocked, "evidence_not_current", boundaries)
	}
	if claimBinding.Status == EvidenceClaimBindingNotBound {
		return evidenceReliance(EvidenceRelianceBlocked, "claim_not_bound_to_evidence", boundaries)
	}
	if item.Verdict == "supports" || item.Verdict == "accepted" {
		return evidenceReliance(EvidenceRelianceBounded, "evidence_supports_declared_attempted_use_with_boundaries", boundaries)
	}

	return evidenceReliance(EvidenceRelianceAdvisory, "evidence_present_without_supporting_verdict", boundaries)
}

func evidenceReliance(disposition string, reason string, boundaries []string) RelianceDisposition {
	return RelianceDisposition{
		Disposition: disposition,
		Reason:      reason,
		Boundaries:  append([]string(nil), boundaries...),
	}
}
