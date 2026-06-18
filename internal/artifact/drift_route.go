package artifact

import "strings"

const (
	DriftRouteSchemaVersion = 1
	DriftRouteRecordKind    = "semantic_drift_route"
	DriftRouteAuthority     = "read_only_repair_routing"

	DriftRouteBoundaryNotMutation     = "not_mutation"
	DriftRouteBoundaryNotApproval     = "not_approval"
	DriftRouteBoundaryNotEvidence     = "not_evidence"
	DriftRouteBoundaryNotGateDecision = "not_gate_decision"
	DriftRouteBoundaryNotGlobalTruth  = "not_global_truth"

	DriftRouteUnknownKind = "unknown_drift_kind"
)

type DriftRouteInput struct {
	DriftKind  string
	BearerRef  string
	UseContext string
}

type SemanticDriftRoute struct {
	SchemaVersion             int                `json:"schema_version"`
	RecordKind                string             `json:"record_kind"`
	Authority                 string             `json:"authority"`
	DriftKind                 string             `json:"drift_kind"`
	DriftLayer                string             `json:"drift_layer"`
	BearerRef                 string             `json:"bearer_ref,omitempty"`
	UseContext                string             `json:"use_context,omitempty"`
	CandidateRepairActions    []string           `json:"candidate_repair_actions,omitempty"`
	LanguageStateMoveKinds    []string           `json:"language_state_move_kinds,omitempty"`
	EntityOfConcernChangeMode string             `json:"entity_of_concern_change_mode"`
	EvidenceNeeded            []string           `json:"evidence_needed,omitempty"`
	BlockedUses               []string           `json:"blocked_uses,omitempty"`
	NextAdmissibleMove        string             `json:"next_admissible_move"`
	AuthorityBoundary         DriftRouteBoundary `json:"authority_boundary"`
	Recognized                bool               `json:"recognized"`
	Notes                     []string           `json:"notes,omitempty"`
}

type DriftRouteBoundary struct {
	Mutation     string `json:"mutation"`
	Evidence     string `json:"evidence"`
	Approval     string `json:"approval"`
	GateDecision string `json:"gate_decision"`
	GlobalTruth  string `json:"global_truth"`
}

type driftRouteSpec struct {
	layer    string
	actions  []string
	moves    []string
	mode     string
	evidence []string
	blocked  []string
	next     string
	notes    []string
}

func BuildSemanticDriftRoute(input DriftRouteInput) SemanticDriftRoute {
	normalized := normalizeDriftRouteInput(input)
	spec, recognized := semanticDriftRouteSpec(normalized.DriftKind)
	if !recognized {
		spec = unknownSemanticDriftRouteSpec(normalized.DriftKind)
	}

	return SemanticDriftRoute{
		SchemaVersion:             DriftRouteSchemaVersion,
		RecordKind:                DriftRouteRecordKind,
		Authority:                 DriftRouteAuthority,
		DriftKind:                 normalized.DriftKind,
		DriftLayer:                spec.layer,
		BearerRef:                 normalized.BearerRef,
		UseContext:                normalized.UseContext,
		CandidateRepairActions:    append([]string(nil), spec.actions...),
		LanguageStateMoveKinds:    append([]string(nil), spec.moves...),
		EntityOfConcernChangeMode: spec.mode,
		EvidenceNeeded:            append([]string(nil), spec.evidence...),
		BlockedUses:               driftRouteBlockedUses(normalized, spec),
		NextAdmissibleMove:        spec.next,
		AuthorityBoundary: DriftRouteBoundary{
			Mutation:     DriftRouteBoundaryNotMutation,
			Evidence:     DriftRouteBoundaryNotEvidence,
			Approval:     DriftRouteBoundaryNotApproval,
			GateDecision: DriftRouteBoundaryNotGateDecision,
			GlobalTruth:  DriftRouteBoundaryNotGlobalTruth,
		},
		Recognized: recognized,
		Notes:      append([]string(nil), spec.notes...),
	}
}

func normalizeDriftRouteInput(input DriftRouteInput) DriftRouteInput {
	return DriftRouteInput{
		DriftKind:  strings.TrimSpace(input.DriftKind),
		BearerRef:  strings.TrimSpace(input.BearerRef),
		UseContext: strings.TrimSpace(input.UseContext),
	}
}

func semanticDriftRouteSpec(kind string) (driftRouteSpec, bool) {
	specs := map[string]driftRouteSpec{
		"carrier_drift": routeSpec(
			"carrier",
			[]string{"republish_or_reencode", "no_change"},
			[]string{"view", "republish"},
			"preserve",
			[]string{"carrier_hash", "publication_projection_hash"},
			"inspect carrier bytes before semantic repair",
		),
		"publication_faithfulness_drift": routeSpec(
			"publication",
			[]string{"republish_or_reencode", "repair_coarsening_contract"},
			[]string{"republish", "revise"},
			"preserve",
			[]string{"source_edition_ref", "publication_projection_hash", "loss_or_omission_refs"},
			"repair publication before changing episteme",
		),
		"coarsening_source_path_drift": routeSpec(
			"publication",
			[]string{"repair_coarsening_contract", "republish_or_reencode"},
			[]string{"view", "republish"},
			"preserve",
			[]string{"source_return_path", "coarsening_contract_ref"},
			"restore exact source-return path",
		),
		"episteme_claim_drift": routeSpec(
			"episteme",
			[]string{"repair_episteme_claim", "reopen_decision", "supersede_decision"},
			[]string{"revise", "reopen", "retire"},
			"preserve",
			[]string{"affected_claim_refs", "claim_support_refs"},
			"revise claim-bearing episteme, not implementation by default",
		),
		"episteme_retarget_drift": routeSpec(
			"episteme",
			[]string{"repair_episteme_claim", "repair_context_or_bridge", "reopen_decision"},
			[]string{"retarget", "reopen"},
			"retarget",
			[]string{"entity_of_concern_ref", "target_context_ref"},
			"retarget entity of concern explicitly",
		),
		"entity_or_context_drift": routeSpec(
			"context",
			[]string{"repair_context_or_bridge", "reopen_decision"},
			[]string{"retarget", "reopen"},
			"retarget",
			[]string{"bounded_context_ref", "entity_of_concern_ref"},
			"repair context/bridge before code repair",
		),
		"bridge_drift": routeSpec(
			"bridge",
			[]string{"repair_context_or_bridge", "repair_coarsening_contract"},
			[]string{"retarget", "republish"},
			"retarget",
			[]string{"bridge_ref", "licensed_context_refs"},
			"restore bridge/licensing relation",
		),
		"transformation_definition_drift": routeSpec(
			"transformation",
			[]string{"repair_transformation_definition", "reopen_decision"},
			[]string{"revise", "reopen"},
			"preserve",
			[]string{"transformation_ref", "initial_post_state_refs"},
			"repair definition before judging realization",
		),
		"transformation_realization_drift": routeSpec(
			"realization",
			[]string{"repair_correspondence", "repair_code", "reopen_decision"},
			[]string{"revise", "reopen"},
			"preserve",
			[]string{"transformation_ref", "observed_change_refs", "correspondence_refs"},
			"code repair is only a candidate when realization drift is actually in code",
		),
		"implementation_correspondence_drift": routeSpec(
			"correspondence",
			[]string{"repair_correspondence", "repair_code", "reopen_decision"},
			[]string{"revise", "reopen"},
			"preserve",
			[]string{"code_correspondence_refs", "observed_code_refs"},
			"repair correspondence before assuming implementation defect",
		),
		"evidence_binding_drift": routeSpec(
			"evidence",
			[]string{"refresh_evidence", "repair_correspondence"},
			[]string{"view", "revise"},
			"preserve",
			[]string{"evidence_path_refs", "affected_claim_refs"},
			"repair evidence binding, not the claim by default",
		),
		"evidence_freshness_drift": routeSpec(
			"evidence",
			[]string{"refresh_evidence", "accept_bounded_use"},
			[]string{"view", "republish"},
			"preserve",
			[]string{"valid_until", "currentness_window"},
			"expired evidence blocks stronger use until refreshed or bounded",
		),
		"decision_basis_drift": routeSpec(
			"decision",
			[]string{"reopen_decision", "supersede_decision"},
			[]string{"reopen", "retire"},
			"preserve",
			[]string{"decision_basis_refs", "counterargument_refs"},
			"decision basis drift requires human decision flow",
		),
		"commitment_validity_drift": routeSpec(
			"commitment",
			[]string{"record_or_revoke_commitment", "reopen_decision"},
			[]string{"reopen", "retire"},
			"preserve",
			[]string{"commitment_refs", "authority_source_refs"},
			"commitment repair is separate from documentation repair",
		),
		"state_assertion_drift": routeSpec(
			"state",
			[]string{"accept_bounded_use", "reopen_decision"},
			[]string{"view", "reopen"},
			"preserve",
			[]string{"state_reading_ref", "reopen_condition"},
			"qualified state reading must name bearer/frame/use",
		),
		"fpf_profile_drift": routeSpec(
			"profile",
			[]string{"repair_context_or_bridge", "repair_episteme_claim"},
			[]string{"respecify", "republish"},
			"preserve",
			[]string{"profile_ref", "profile_validity_ref"},
			"refresh governing profile before stronger interpretation",
		),
	}

	spec, ok := specs[kind]
	return spec, ok
}

func routeSpec(
	layer string,
	actions []string,
	moves []string,
	mode string,
	evidence []string,
	next string,
) driftRouteSpec {
	return driftRouteSpec{
		layer:    layer,
		actions:  actions,
		moves:    moves,
		mode:     mode,
		evidence: evidence,
		next:     next,
		notes: []string{
			"route is advisory and read-only",
			"do not execute repair without the governing decision/workflow",
		},
	}
}

func unknownSemanticDriftRouteSpec(kind string) driftRouteSpec {
	return driftRouteSpec{
		layer:    DriftRouteUnknownKind,
		actions:  []string{"no_change"},
		moves:    []string{"view"},
		mode:     "preserve",
		evidence: []string{"drift_kind_ref", "bearer_ref", "affected_relation_refs"},
		blocked:  []string{"stronger_use_until_drift_kind_is_classified"},
		next:     "classify drift kind before selecting repair",
		notes: []string{
			"unknown drift kind is fail-closed",
			"route is advisory and read-only",
		},
	}
}

func driftRouteBlockedUses(input DriftRouteInput, spec driftRouteSpec) []string {
	blocked := append([]string(nil), spec.blocked...)
	if input.UseContext != "" {
		blocked = append(blocked, input.UseContext)
	}

	return blocked
}
