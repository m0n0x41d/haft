package artifact

import "strings"

const (
	EngineeringValueSpaceSchemaVersion = 1
	EngineeringValueSpaceRecordKind    = "haft_engineering_value_characteristic_space"
	EngineeringValueSpaceAuthority     = "read_only_value_characterization"

	EngineeringValueNoSingleScore          = "no_single_haft_or_fpf_score"
	EngineeringValueCharacteristicOnly     = "characteristic_space_only"
	EngineeringValueHealthyReopenTreatment = "healthy_reopening_not_counted_as_simple_failure"
	EngineeringValueNoMovementDisposition  = "revise_bound_or_remove_features_without_measurable_value_movement"

	EngineeringValueBoundaryNotScore        = "not_score"
	EngineeringValueBoundaryNotEvidence     = "not_evidence"
	EngineeringValueBoundaryNotApproval     = "not_approval"
	EngineeringValueBoundaryNotGateDecision = "not_gate_decision"
	EngineeringValueBoundaryNotGlobalTruth  = "not_global_truth"

	EngineeringValueSimplifyKillAuthority = "read_only_review_trigger_not_automatic_gate"
	EngineeringValueSimplifyAction        = "simplify_or_remove_capability"
	EngineeringValueKillAction            = "stop_or_retire_capability"
	EngineeringValueReviewAction          = "review_before_continuing_investment"
)

type EngineeringValueSpaceInput struct {
	BearerRef    string
	Window       string
	MethodRef    string
	EvidenceRefs []string
}

type EngineeringValueSpace struct {
	SchemaVersion        int                                     `json:"schema_version"`
	RecordKind           string                                  `json:"record_kind"`
	Authority            string                                  `json:"authority"`
	EvaluatedObject      EngineeringValueEvaluatedObject         `json:"evaluated_object"`
	ScorePolicy          EngineeringValueScorePolicy             `json:"score_policy"`
	Characteristics      []EngineeringValueCharacteristic        `json:"characteristics"`
	SimplifyKillCriteria []EngineeringValueSimplifyKillCriterion `json:"simplify_kill_criteria"`
	ProtectedTradeOffs   []string                                `json:"protected_trade_offs"`
	InterpretationRules  EngineeringValueInterpretationRules     `json:"interpretation_rules"`
	AuthorityBoundary    EngineeringValueSpaceAuthorityBoundary  `json:"authority_boundary"`
}

type EngineeringValueEvaluatedObject struct {
	BearerRef   string `json:"bearer_ref"`
	ObjectKind  string `json:"object_kind"`
	DeclaredUse string `json:"declared_use"`
}

type EngineeringValueScorePolicy struct {
	SingleScore   string `json:"single_score"`
	Aggregation   string `json:"aggregation"`
	GoodhartGuard string `json:"goodhart_guard"`
}

type EngineeringValueCharacteristic struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	BearerRef          string   `json:"bearer_ref"`
	Method             string   `json:"method"`
	Window             string   `json:"window"`
	Denominator        string   `json:"denominator"`
	EvidenceRefs       []string `json:"evidence_refs"`
	Scale              string   `json:"scale"`
	ValueMeaning       string   `json:"value_meaning"`
	EvidenceRule       string   `json:"evidence_rule"`
	Missingness        string   `json:"missingness"`
	Floor              string   `json:"floor"`
	ProtectedTradeOffs []string `json:"protected_trade_offs"`
	ReopenCondition    string   `json:"reopen_condition"`
}

type EngineeringValueSimplifyKillCriterion struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	Trigger            string   `json:"trigger"`
	ReviewAction       string   `json:"review_action"`
	EvidenceRule       string   `json:"evidence_rule"`
	AuthorityBoundary  string   `json:"authority_boundary"`
	ProtectedTradeOffs []string `json:"protected_trade_offs"`
}

type EngineeringValueInterpretationRules struct {
	HealthyReopening            string `json:"healthy_reopening"`
	FeatureWithoutValueMovement string `json:"feature_without_value_movement"`
	ComparisonMode              string `json:"comparison_mode"`
}

type EngineeringValueSpaceAuthorityBoundary struct {
	Score        string `json:"score"`
	Evidence     string `json:"evidence"`
	Approval     string `json:"approval"`
	GateDecision string `json:"gate_decision"`
	GlobalTruth  string `json:"global_truth"`
}

type engineeringValueCharacteristicTemplate struct {
	id                 string
	name               string
	method             string
	denominator        string
	scale              string
	valueMeaning       string
	evidenceRule       string
	floor              string
	protectedTradeOffs []string
	reopenCondition    string
}

func BuildEngineeringValueSpace(input EngineeringValueSpaceInput) EngineeringValueSpace {
	normalized := normalizeEngineeringValueSpaceInput(input)

	return EngineeringValueSpace{
		SchemaVersion: EngineeringValueSpaceSchemaVersion,
		RecordKind:    EngineeringValueSpaceRecordKind,
		Authority:     EngineeringValueSpaceAuthority,
		EvaluatedObject: EngineeringValueEvaluatedObject{
			BearerRef:   normalized.BearerRef,
			ObjectKind:  "haft_release_or_configuration",
			DeclaredUse: "reduce_semantic_decision_correspondence_rework_without_unacceptable_ceremony_or_false_blocking",
		},
		ScorePolicy: EngineeringValueScorePolicy{
			SingleScore:   EngineeringValueNoSingleScore,
			Aggregation:   EngineeringValueCharacteristicOnly,
			GoodhartGuard: "do_not_optimize_one_metric_as_value_truth",
		},
		Characteristics:      engineeringValueCharacteristics(normalized),
		SimplifyKillCriteria: engineeringValueSimplifyKillCriteria(),
		ProtectedTradeOffs:   engineeringValueProtectedTradeOffs(),
		InterpretationRules:  engineeringValueInterpretationRules(),
		AuthorityBoundary: EngineeringValueSpaceAuthorityBoundary{
			Score:        EngineeringValueBoundaryNotScore,
			Evidence:     EngineeringValueBoundaryNotEvidence,
			Approval:     EngineeringValueBoundaryNotApproval,
			GateDecision: EngineeringValueBoundaryNotGateDecision,
			GlobalTruth:  EngineeringValueBoundaryNotGlobalTruth,
		},
	}
}

func engineeringValueSimplifyKillCriteria() []EngineeringValueSimplifyKillCriterion {
	return []EngineeringValueSimplifyKillCriterion{
		{
			ID:                "scope_violation_not_blocked_or_surfaced",
			Name:              "Scope violation not blocked or surfaced",
			Trigger:           "feature_or_agent_surface_allows_stronger_use_outside_declared_bearer_frame_or_attempted_use",
			ReviewAction:      EngineeringValueKillAction,
			EvidenceRule:      "compare attempted use, bearer, exact source refs, and blocked-use attention records before continuing investment",
			AuthorityBoundary: EngineeringValueSimplifyKillAuthority,
			ProtectedTradeOffs: []string{
				"semantic_fidelity_vs_ceremony",
				"automation_vs_principal_control",
			},
		},
		{
			ID:                "ceremony_exceeds_value_movement",
			Name:              "Ceremony exceeds measured value movement",
			Trigger:           "governance_ceremony_time_increases_without_movement_in_at_least_one_declared_value_characteristic",
			ReviewAction:      EngineeringValueSimplifyAction,
			EvidenceRule:      "pair ceremony samples with characteristic evidence in the same bearer/window; missing value evidence keeps the trigger advisory",
			AuthorityBoundary: EngineeringValueSimplifyKillAuthority,
			ProtectedTradeOffs: []string{
				"semantic_fidelity_vs_ceremony",
				"exactness_vs_cognitive_ergonomics",
			},
		},
		{
			ID:                "false_block_rate_exceeds_tolerance",
			Name:              "False blocking exceeds tolerance",
			Trigger:           "semantic_review_false_block_rate_exceeds_operator_declared_tolerance_for_the_window",
			ReviewAction:      EngineeringValueSimplifyAction,
			EvidenceRule:      "operator-reviewed blocked findings must separate legitimate multi-view same-object cases from true high-risk blocks",
			AuthorityBoundary: EngineeringValueSimplifyKillAuthority,
			ProtectedTradeOffs: []string{
				"early_detection_vs_false_positives",
				"exactness_vs_cognitive_ergonomics",
			},
		},
		{
			ID:                "missing_equal_budget_comparison",
			Name:              "Missing equal-budget comparison",
			Trigger:           "value_claim_is_made_without_equal_budget_baseline_or_explicit_abstain",
			ReviewAction:      EngineeringValueReviewAction,
			EvidenceRule:      "compare against declared baseline under parity or label the value claim unavailable for the window",
			AuthorityBoundary: EngineeringValueSimplifyKillAuthority,
			ProtectedTradeOffs: []string{
				"durable_traceability_vs_artifact_explosion",
				"compact_views_vs_source_recoverability",
			},
		},
		{
			ID:                "evidence_refs_missing",
			Name:              "Evidence refs missing",
			Trigger:           "characteristic_or_dashboard_claim_has_no_source_refs_for_the_declared_window",
			ReviewAction:      EngineeringValueReviewAction,
			EvidenceRule:      "source refs must identify evidence records before the dashboard can support a product-value claim",
			AuthorityBoundary: EngineeringValueSimplifyKillAuthority,
			ProtectedTradeOffs: []string{
				"compact_views_vs_source_recoverability",
				"automation_vs_principal_control",
			},
		},
		{
			ID:                 "single_proxy_value_claim",
			Name:               "Single proxy value claim",
			Trigger:            "one_metric_or_scalar_score_is_presented_as_haft_or_fpf_value_truth",
			ReviewAction:       EngineeringValueKillAction,
			EvidenceRule:       "inspect the surface for scalarized value claims and recover the protected trade-offs hidden by the proxy",
			AuthorityBoundary:  EngineeringValueSimplifyKillAuthority,
			ProtectedTradeOffs: engineeringValueProtectedTradeOffs(),
		},
	}
}

func normalizeEngineeringValueSpaceInput(input EngineeringValueSpaceInput) EngineeringValueSpaceInput {
	bearerRef := strings.TrimSpace(input.BearerRef)
	if bearerRef == "" {
		bearerRef = "haft_release_or_configuration"
	}

	window := strings.TrimSpace(input.Window)
	if window == "" {
		window = "declared_window_required_before_value_claim"
	}

	methodRef := strings.TrimSpace(input.MethodRef)
	if methodRef == "" {
		methodRef = "measurement_method_required_before_value_claim"
	}

	return EngineeringValueSpaceInput{
		BearerRef:    bearerRef,
		Window:       window,
		MethodRef:    methodRef,
		EvidenceRefs: compactStrings(input.EvidenceRefs),
	}
}

func engineeringValueCharacteristics(input EngineeringValueSpaceInput) []EngineeringValueCharacteristic {
	templates := engineeringValueCharacteristicTemplates()
	characteristics := make([]EngineeringValueCharacteristic, 0, len(templates))

	for _, template := range templates {
		characteristics = append(characteristics, engineeringValueCharacteristic(template, input))
	}

	return characteristics
}

func engineeringValueCharacteristic(
	template engineeringValueCharacteristicTemplate,
	input EngineeringValueSpaceInput,
) EngineeringValueCharacteristic {
	return EngineeringValueCharacteristic{
		ID:                 template.id,
		Name:               template.name,
		BearerRef:          input.BearerRef,
		Method:             input.MethodRef + ":" + template.method,
		Window:             input.Window,
		Denominator:        template.denominator,
		EvidenceRefs:       append([]string{}, input.EvidenceRefs...),
		Scale:              template.scale,
		ValueMeaning:       template.valueMeaning,
		EvidenceRule:       template.evidenceRule,
		Missingness:        engineeringValueMissingness(input),
		Floor:              template.floor,
		ProtectedTradeOffs: append([]string(nil), template.protectedTradeOffs...),
		ReopenCondition:    template.reopenCondition,
	}
}

func engineeringValueMissingness(input EngineeringValueSpaceInput) string {
	if len(input.EvidenceRefs) == 0 {
		return "evidence_refs_missing_value_claim_blocked"
	}

	return "declared_evidence_refs_required_for_interpretation"
}

func engineeringValueInterpretationRules() EngineeringValueInterpretationRules {
	return EngineeringValueInterpretationRules{
		HealthyReopening:            EngineeringValueHealthyReopenTreatment,
		FeatureWithoutValueMovement: EngineeringValueNoMovementDisposition,
		ComparisonMode:              "compare_characteristics_under_declared_parity_not_scalar_score",
	}
}

func engineeringValueProtectedTradeOffs() []string {
	return []string{
		"semantic_fidelity_vs_ceremony",
		"exactness_vs_cognitive_ergonomics",
		"early_detection_vs_false_positives",
		"durable_traceability_vs_artifact_explosion",
		"automation_vs_principal_control",
		"compact_views_vs_source_recoverability",
	}
}

func engineeringValueCharacteristicTemplates() []engineeringValueCharacteristicTemplate {
	return []engineeringValueCharacteristicTemplate{
		{
			id:                 "rationale_recovery_time",
			name:               "Rationale recovery time",
			method:             "time_to_recover_decision_rationale",
			denominator:        "recoverable_rationale_lookup_attempts",
			scale:              "duration_lower_is_better",
			valueMeaning:       "lower time means governance memory is easier to recover without chat context",
			evidenceRule:       "measure from a request for rationale to exact DecisionRecord or source-return packet",
			floor:              "cannot claim movement without timed recovery attempts",
			protectedTradeOffs: []string{"exactness_vs_cognitive_ergonomics", "compact_views_vs_source_recoverability"},
			reopenCondition:    "reopen if recovery relies on stale chat memory or unrecoverable publication prose",
		},
		{
			id:                 "semantic_round_trip_loss_rate",
			name:               "Semantic round-trip loss rate",
			method:             "db_markdown_empty_db_round_trip_check",
			denominator:        "semantic_round_trips",
			scale:              "rate_lower_is_better",
			valueMeaning:       "lower loss means carrier sync preserves semantic identity",
			evidenceRule:       "compare semantic edition, support refs, publication refs, and carrier hashes after round trip",
			floor:              "cannot score from formatting-only diffs",
			protectedTradeOffs: []string{"semantic_fidelity_vs_ceremony", "durable_traceability_vs_artifact_explosion"},
			reopenCondition:    "reopen if a carrier-only edit changes semantic edition or support refs silently",
		},
		{
			id:                 "silent_retarget_prevention_rate",
			name:               "Silent-retarget prevention rate",
			method:             "retarget_fixture_or_drift_case_review",
			denominator:        "entity_or_context_retarget_opportunities",
			scale:              "rate_higher_is_better",
			valueMeaning:       "higher prevention means entity/context drift is surfaced before stronger use",
			evidenceRule:       "count retarget attempts blocked or routed before they became authority",
			floor:              "cannot count cases without a declared entity and context",
			protectedTradeOffs: []string{"early_detection_vs_false_positives", "semantic_fidelity_vs_ceremony"},
			reopenCondition:    "reopen if retarget drift passes through as implementation work",
		},
		{
			id:                 "semantic_review_false_block_rate",
			name:               "Semantic-review false-block rate",
			method:             "operator_review_of_blocked_findings",
			denominator:        "semantic_review_blocks_reviewed",
			scale:              "rate_lower_is_better",
			valueMeaning:       "lower false blocking preserves operator flow while keeping high-risk blocks visible",
			evidenceRule:       "classify blocked-use findings after exact source review by the operator",
			floor:              "missing operator review keeps result advisory",
			protectedTradeOffs: []string{"early_detection_vs_false_positives", "exactness_vs_cognitive_ergonomics"},
			reopenCondition:    "reopen if reviews block legitimate multi-view same-object specs",
		},
		{
			id:                 "comparison_to_choice_leakage_defect_rate",
			name:               "Comparison-to-choice leakage defect rate",
			method:             "choice_surface_defect_audit",
			denominator:        "comparison_or_recommendation_surfaces",
			scale:              "rate_lower_is_better",
			valueMeaning:       "lower leakage means recommendations do not masquerade as human choices",
			evidenceRule:       "inspect surfaces for ComparisonResult/ChoiceResult boundary violations",
			floor:              "cannot count without examples that include both comparison and decision surfaces",
			protectedTradeOffs: []string{"automation_vs_principal_control", "compact_views_vs_source_recoverability"},
			reopenCondition:    "reopen if selected_ref is consumed as a bound DecisionRecord choice",
		},
		{
			id:                 "drift_repair_routing_precision",
			name:               "Drift repair routing precision",
			method:             "drift_route_followup_judgment",
			denominator:        "drift_signals_with_judged_repair_target",
			scale:              "rate_higher_is_better",
			valueMeaning:       "higher precision means drift is routed to the right semantic layer before code repair",
			evidenceRule:       "compare suggested repair target to later accepted repair path",
			floor:              "unknown drift kinds remain abstain/block, not false precision",
			protectedTradeOffs: []string{"early_detection_vs_false_positives", "semantic_fidelity_vs_ceremony"},
			reopenCondition:    "reopen if evidence/publication drift routinely routes to code repair",
		},
		{
			id:                 "rework_loops_by_repair_target",
			name:               "Rework loops by repair target",
			method:             "engineering_change_case_loop_count",
			denominator:        "engineering_changes_grouped_by_repair_target",
			scale:              "count_lower_is_better_with_target_distribution",
			valueMeaning:       "lower repeated loops mean repair target selection is improving",
			evidenceRule:       "derive loops from EngineeringChangeCase, drift route, and repair target refs",
			floor:              "healthy reopening is separated from avoidable rework",
			protectedTradeOffs: []string{"durable_traceability_vs_artifact_explosion", "automation_vs_principal_control"},
			reopenCondition:    "reopen if the same semantic layer loops without a changed hypothesis",
		},
		{
			id:                 "time_from_drift_signal_to_correct_repair",
			name:               "Time from drift signal to correct repair",
			method:             "drift_signal_to_accepted_repair_duration",
			denominator:        "drift_signals_routed_to_repair",
			scale:              "duration_lower_is_better",
			valueMeaning:       "lower time means attention and repair routing reduce delay without hiding uncertainty",
			evidenceRule:       "measure from first drift signal to accepted repair record or explicit abstain",
			floor:              "no accepted repair or abstain means outcome unknown",
			protectedTradeOffs: []string{"early_detection_vs_false_positives", "exactness_vs_cognitive_ergonomics"},
			reopenCondition:    "reopen if speed improves by skipping exact source return",
		},
		{
			id:                 "governance_ceremony_time",
			name:               "Governance ceremony time",
			method:             "method_run_and_operator_time_sample",
			denominator:        "governance_tasks_in_window",
			scale:              "duration_lower_is_better_with_quality_floor",
			valueMeaning:       "lower ceremony time is good only while semantic fidelity and principal control hold",
			evidenceRule:       "sample MethodRuns and operator-facing governance tasks with outcome labels",
			floor:              "cannot improve by removing required evidence or human decisions",
			protectedTradeOffs: []string{"semantic_fidelity_vs_ceremony", "automation_vs_principal_control"},
			reopenCondition:    "reopen if reduced ceremony correlates with escaped invariant violations",
		},
		{
			id:                 "first_pass_acceptance_and_escaped_defects",
			name:               "First-pass acceptance and escaped defects",
			method:             "artifact_acceptance_plus_escape_audit",
			denominator:        "submitted_artifacts_and_escaped_invariant_defects",
			scale:              "paired_rate_acceptance_higher_escape_lower",
			valueMeaning:       "value improves only when acceptance rises without increasing escaped defects",
			evidenceRule:       "pair acceptance events with later defects against the same bearer/window",
			floor:              "single-sided acceptance without escape audit is insufficient",
			protectedTradeOffs: []string{"early_detection_vs_false_positives", "durable_traceability_vs_artifact_explosion"},
			reopenCondition:    "reopen if first-pass acceptance rises while escaped defects rise",
		},
		{
			id:                 "protected_tradeoff_review",
			name:               "Protected trade-off review",
			method:             "explicit_tradeoff_review_presence",
			denominator:        "changes_touching_protected_tradeoffs",
			scale:              "coverage_higher_is_better",
			valueMeaning:       "higher coverage means value trade-offs are reviewed instead of collapsed into proxy metrics",
			evidenceRule:       "check that affected trade-offs are named with source/evidence refs before the change is called done",
			floor:              "no single metric can override a protected trade-off",
			protectedTradeOffs: engineeringValueProtectedTradeOffs(),
			reopenCondition:    "reopen if a proxy metric hides a principal-control or fidelity trade-off",
		},
	}
}
