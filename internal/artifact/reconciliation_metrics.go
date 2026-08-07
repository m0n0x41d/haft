package artifact

const (
	ReconciliationMetricsSchemaVersion = 1
	ReconciliationMetricsAuthority     = "read_only_reconciliation_metrics_not_binding_authority"
)

type ReconciliationMetricsPacket struct {
	SchemaVersion  int                            `json:"schema_version"`
	Authority      string                         `json:"authority"`
	CapturePolicy  string                         `json:"capture_policy"`
	Reconciliation ReconciliationPlanMetrics      `json:"reconciliation"`
	GoverningSet   ReconciliationGoverningMetrics `json:"governing_set"`
	DriftEvents    ReconciliationDriftMetrics     `json:"drift_events"`
	BeforeAfterUse ReconciliationBeforeAfterUse   `json:"before_after_use"`
}

type ReconciliationPlanMetrics struct {
	ReviewedDecisions         int `json:"reviewed_decisions"`
	Groups                    int `json:"groups"`
	WholeFileFallbackOnly     int `json:"whole_file_fallback_only"`
	MissingExplicitSubject    int `json:"missing_explicit_subject"`
	ScopeEnrichmentCandidates int `json:"scope_enrichment_candidates"`
	ConflictRequiresOperator  int `json:"conflict_requires_operator"`
}

type ReconciliationGoverningMetrics struct {
	CurrentDecisions       int `json:"current_decisions"`
	GoverningSets          int `json:"governing_sets"`
	FallbackTargetSets     int `json:"fallback_target_sets"`
	ScopeEnrichmentSets    int `json:"scope_enrichment_sets"`
	ConflictSets           int `json:"conflict_sets"`
	OverlapReviewSets      int `json:"overlap_review_sets"`
	MissingExplicitSubject int `json:"missing_explicit_subject"`
	TerminalHistoryRefs    int `json:"terminal_history_refs"`
}

type ReconciliationDriftMetrics struct {
	UniqueEvents                 int `json:"unique_events"`
	ImpactedDecisions            int `json:"impacted_decisions"`
	MaterialEvents               int `json:"material_events"`
	AuditOnlyEvents              int `json:"audit_only_events"`
	NeedsBindingResolutionEvents int `json:"needs_binding_resolution_events"`
	SemanticTargetEvents         int `json:"semantic_target_events"`
	FileFallbackEvents           int `json:"file_fallback_events"`
	UnknownHighRiskEvents        int `json:"unknown_high_risk_events"`
	MaxFanout                    int `json:"max_fanout"`
}

type ReconciliationBeforeAfterUse struct {
	BeforeCommand     string   `json:"before_command"`
	ApplyCommand      string   `json:"apply_command"`
	AfterCommand      string   `json:"after_command"`
	RequiredAuthority string   `json:"required_authority"`
	MutationBoundary  []string `json:"mutation_boundary"`
}

func BuildReconciliationMetricsPacket(
	plan DecisionReconciliationPlan,
	governing CurrentGoverningSetReport,
	driftEvents DriftEventReport,
) ReconciliationMetricsPacket {
	return ReconciliationMetricsPacket{
		SchemaVersion: ReconciliationMetricsSchemaVersion,
		Authority:     ReconciliationMetricsAuthority,
		CapturePolicy: "capture_before_and_after_operator_approved_reconciliation_apply",
		Reconciliation: ReconciliationPlanMetrics{
			ReviewedDecisions:         plan.Summary.ReviewedDecisions,
			Groups:                    plan.Summary.Groups,
			WholeFileFallbackOnly:     plan.Summary.WholeFileFallbackOnly,
			MissingExplicitSubject:    plan.Summary.MissingExplicitSubject,
			ScopeEnrichmentCandidates: plan.Summary.ScopeEnrichmentCandidates,
			ConflictRequiresOperator:  plan.Summary.ConflictRequiresOperator,
		},
		GoverningSet: ReconciliationGoverningMetrics{
			CurrentDecisions:       governing.Summary.CurrentDecisions,
			GoverningSets:          governing.Summary.GoverningSets,
			FallbackTargetSets:     governing.Summary.FallbackTargetSets,
			ScopeEnrichmentSets:    governing.Summary.ScopeEnrichmentSets,
			ConflictSets:           governing.Summary.ConflictSets,
			OverlapReviewSets:      governing.Summary.OverlapReviewSets,
			MissingExplicitSubject: governing.Summary.MissingExplicitSubject,
			TerminalHistoryRefs:    governing.Summary.TerminalHistoryRefs,
		},
		DriftEvents: ReconciliationDriftMetrics{
			UniqueEvents:                 driftEvents.Summary.UniqueEvents,
			ImpactedDecisions:            driftEvents.Summary.ImpactedDecisions,
			MaterialEvents:               driftEvents.Summary.MaterialEvents,
			AuditOnlyEvents:              driftEvents.Summary.AuditOnlyEvents,
			NeedsBindingResolutionEvents: driftEvents.Summary.NeedsBindingResolutionEvents,
			SemanticTargetEvents:         driftEvents.Summary.SemanticTargetEvents,
			FileFallbackEvents:           driftEvents.Summary.FileFallbackEvents,
			UnknownHighRiskEvents:        driftEvents.Summary.UnknownHighRiskEvents,
			MaxFanout:                    driftEvents.Summary.MaxFanout,
		},
		BeforeAfterUse: ReconciliationBeforeAfterUse{
			BeforeCommand:     "haft decision reconcile metrics --json",
			ApplyCommand:      "haft decision reconcile apply SELECTION.json --json",
			AfterCommand:      "haft decision reconcile metrics --json",
			RequiredAuthority: "operator_approved_reconciliation_selection",
			MutationBoundary: []string{
				"metrics capture is read-only",
				"selection apply remains the only mutation step",
				"metrics do not approve, supersede, retire, enrich, waive, or rebaseline decisions",
			},
		},
	}
}
