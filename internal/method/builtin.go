package method

func verificationBeforeCompletion() Definition {
	return Definition{
		ID:      "verification-before-completion",
		Version: CatalogVersion,
		Title:   "Verification before completion",
		Summary: "Completion claims need fresh verification evidence.",
		Intent:  "Do not claim work is done until the relevant check has actually run or an explicit waiver is recorded.",
		AppliesTo: Applicability{
			TaskKinds:     []string{"feature", "bugfix", "debug", "refactor", "external_integration", "architecture"},
			ChangeIntents: []string{"add_feature", "fix_bug", "refactor", "change_behavior"},
			RiskSignals:   []string{"failing_test", "behavior_change", "external_io", "domain_boundary", "governed_file"},
		},
		DoesNotApplyTo: Applicability{
			TaskKinds: []string{TaskMechanicalEdit, TaskFormattingOnly},
		},
		HardGates: []Gate{{
			ID:               "fresh_verification_before_completion",
			Kind:             "test_evidence",
			CheckLevel:       "deterministic",
			PassCondition:    "A relevant test, build, runtime check, or diff inspection is recorded before completion.",
			RequiredEvidence: []string{"command_output", "runtime_check", "diff_inspection"},
			Waiver:           WaiverPolicy{Allowed: true, RequiresReason: true},
		}},
		Procedure: []string{
			"Identify the narrowest relevant verification before editing.",
			"Run that verification after the change.",
			"Record the command/result or explicit waiver before claiming completion.",
		},
		AntiPatterns: []string{
			"Completion claim with no fresh evidence.",
			"Assuming a check passed because the change is small.",
		},
		RequiredEvidence: []string{"command_output", "runtime_check", "diff_inspection"},
		Waiver:           WaiverPolicy{Allowed: true, RequiresReason: true},
		Priority:         90,
	}
}

func problemClosureHygiene() Definition {
	return Definition{
		ID:      "problem-closure-hygiene",
		Version: CatalogVersion,
		Title:   "Problem closure hygiene",
		Summary: "Completed work must leave linked or deliberately suspended problems.",
		Intent:  "Do not claim implementation work is done while a linked ProblemCard remains active/backlog with no SolutionPortfolio, DecisionRecord, or supporting evidence path.",
		AppliesTo: Applicability{
			RiskSignals: []string{"problem_closure_hygiene", "governed_workflow_change"},
		},
		DoesNotApplyTo: Applicability{
			TaskKinds: []string{TaskMechanicalEdit, TaskFormattingOnly},
		},
		HardGates: []Gate{{
			ID:               "problem_graph_closure_hygiene_recorded",
			Kind:             "graph_hygiene",
			CheckLevel:       "deterministic",
			PassCondition:    "Each linked active ProblemCard has a SolutionPortfolio, DecisionRecord, or supporting evidence path, or an explicit waiver records why it remains open.",
			RequiredEvidence: []string{"problem_ref", "linked_artifact_ref_or_evidence_ref", "status_or_query_ref"},
			Waiver:           WaiverPolicy{Allowed: true, RequiresReason: true},
		}},
		Procedure: []string{
			"List ProblemCards linked to the MethodRun before close.",
			"Confirm each has a graph path to a SolutionPortfolio, DecisionRecord, or supporting evidence.",
			"Waive only when the operator deliberately leaves the problem open, with a reason.",
		},
		AntiPatterns: []string{
			"Closing implementation work while the related ProblemCard still looks like untouched backlog.",
			"Treating tests or changelog text as graph closure without a link/evidence path.",
		},
		RequiredEvidence: []string{"problem_ref", "linked_artifact_ref_or_evidence_ref", "status_or_query_ref"},
		Waiver:           WaiverPolicy{Allowed: true, RequiresReason: true},
		Priority:         4,
	}
}

func graphPreflightBeforeGovernedEdit() Definition {
	return Definition{
		ID:      "graph-preflight-before-governed-edit",
		Version: CatalogVersion,
		Title:   "Graph preflight before governed edit",
		Summary: "Governed code work needs Haft graph evidence before edits.",
		Intent:  "Use the fused code/reasoning graph before editing governed files or symbols, then record how that evidence affected the plan.",
		AppliesTo: Applicability{
			RiskSignals: []string{"governed_file", "governed_symbol", "decision_governed_code"},
		},
		DoesNotApplyTo: Applicability{
			TaskKinds: []string{TaskMechanicalEdit, TaskFormattingOnly},
		},
		HardGates: []Gate{{
			ID:               "graph_preflight_recorded_before_governed_edit",
			Kind:             "graph_evidence",
			CheckLevel:       "human_review",
			PassCondition:    "The closeout cites pre-edit code_context plus impact/explore/node/callers/callees evidence and states how it changed file choice, blast-radius assessment, or the implementation plan.",
			RequiredEvidence: []string{"code_context_ref", "impact_or_explore_or_node_ref", "plan_influence_note"},
			Waiver:           WaiverPolicy{Allowed: true, RequiresReason: true},
		}},
		SoftGates: []string{"Status-only does not satisfy graph preflight; use code_context and a task-specific traversal."},
		Procedure: []string{
			"Call code_context for the intended governed file or symbol before editing.",
			"Narrow with impact, explore, node, callers, or callees for the symbol flow or blast radius.",
			"Record the graph calls and the resulting plan or risk change in method closeout.",
		},
		AntiPatterns: []string{
			"Calling only status or search and treating it as graph preflight.",
			"Running graph queries after editing just to satisfy the checklist.",
			"Recording graph evidence without saying how it affected the plan.",
		},
		RequiredEvidence: []string{"code_context_ref", "impact_or_explore_or_node_ref", "plan_influence_note"},
		Waiver:           WaiverPolicy{Allowed: true, RequiresReason: true},
		Priority:         5,
	}
}

func systematicDebuggingBeforeFix() Definition {
	return Definition{
		ID:      "systematic-debugging-before-fix",
		Version: CatalogVersion,
		Title:   "Systematic debugging before fix",
		Summary: "Bug fixes need a root-cause hypothesis before patching.",
		Intent:  "Avoid shotgun edits: reproduce or explain the failure, rank hypotheses, then patch the evidenced cause.",
		AppliesTo: Applicability{
			TaskKinds:     []string{"bugfix", "debug"},
			ChangeIntents: []string{"fix_bug"},
			RiskSignals:   []string{"failing_test", "panic", "regression", "intermittent_failure"},
		},
		HardGates: []Gate{{
			ID:               "root_cause_named_before_fix",
			Kind:             "debug_evidence",
			CheckLevel:       "human_review",
			PassCondition:    "The closeout names the root cause or the best evidenced hypothesis before presenting the patch as a fix.",
			RequiredEvidence: []string{"reproduction", "test_ref", "log_ref", "trace_ref"},
			Waiver:           WaiverPolicy{Allowed: true, RequiresReason: true},
		}},
		Procedure: []string{
			"State the observed failure.",
			"Name the strongest root-cause hypothesis.",
			"Patch only after evidence supports the hypothesis.",
		},
		AntiPatterns: []string{
			"Trying plausible fixes without reproducing or explaining the failure.",
			"Editing unrelated files during a narrow bugfix.",
		},
		RequiredEvidence: []string{"reproduction", "test_ref", "log_ref"},
		Waiver:           WaiverPolicy{Allowed: true, RequiresReason: true},
		Priority:         10,
	}
}

func behaviorFirstTesting() Definition {
	return Definition{
		ID:      "behavior-first-testing",
		Version: CatalogVersion,
		Title:   "Behavior-first testing",
		Summary: "User-visible behavior changes need behavior-level evidence.",
		Intent:  "Define the behavior being changed and verify it through the highest practical public boundary.",
		AppliesTo: Applicability{
			TaskKinds:     []string{"feature", "external_integration"},
			ChangeIntents: []string{"add_feature", "change_behavior"},
			RiskSignals:   []string{"behavior_change", "public_api", "external_io"},
		},
		HardGates: []Gate{{
			ID:               "public_behavior_evidence_recorded",
			Kind:             "test_evidence",
			CheckLevel:       "human_review",
			PassCondition:    "The closeout records a public behavior, integration, or API-level check for the changed behavior.",
			RequiredEvidence: []string{"e2e_test", "integration_test", "api_test", "runtime_check"},
			Waiver:           WaiverPolicy{Allowed: true, RequiresReason: true},
		}},
		SoftGates: []string{"Prefer E2E/API/integration evidence over unit-only proof when behavior crosses a boundary."},
		Procedure: []string{
			"Name the public behavior that changes.",
			"Add or identify a behavior-level check.",
			"Run it after implementation.",
		},
		AntiPatterns: []string{
			"Only testing private helpers for a user-visible behavior change.",
			"Changing behavior under a refactor label.",
		},
		RequiredEvidence: []string{"e2e_test", "integration_test", "api_test"},
		Waiver:           WaiverPolicy{Allowed: true, RequiresReason: true},
		Priority:         30,
	}
}

func refactorOnlyUnderTests() Definition {
	return Definition{
		ID:      "refactor-only-under-tests",
		Version: CatalogVersion,
		Title:   "Refactor only under tests",
		Summary: "Behavior-preserving refactors need a before/after test boundary.",
		Intent:  "A refactor should preserve public behavior; baseline and post-change checks make that claim concrete.",
		AppliesTo: Applicability{
			TaskKinds:     []string{"refactor"},
			ChangeIntents: []string{"refactor"},
		},
		HardGates: []Gate{{
			ID:               "baseline_and_post_refactor_checks_recorded",
			Kind:             "test_evidence",
			CheckLevel:       "deterministic",
			PassCondition:    "The closeout records baseline and post-change verification, or an explicit reason the baseline was unavailable.",
			RequiredEvidence: []string{"baseline_test_output", "post_change_test_output"},
			Waiver:           WaiverPolicy{Allowed: true, RequiresReason: true},
		}},
		Procedure: []string{
			"Run or identify the existing behavior checks before editing.",
			"Make structure-only changes.",
			"Run the same checks after editing.",
		},
		AntiPatterns: []string{
			"Adding feature semantics in a refactor.",
			"Refactoring with no behavior check.",
		},
		RequiredEvidence: []string{"baseline_test_output", "post_change_test_output"},
		Waiver:           WaiverPolicy{Allowed: true, RequiresReason: true},
		Priority:         20,
	}
}

func domainPortBeforeAdapter() Definition {
	return Definition{
		ID:      "domain-port-before-adapter",
		Version: CatalogVersion,
		Title:   "Domain port before adapter",
		Summary: "External systems are reached through domain-owned ports.",
		Intent:  "Name the domain capability before implementing transport, persistence, or vendor-specific adapter code.",
		AppliesTo: Applicability{
			TaskKinds:    []string{"external_integration"},
			RiskSignals:  []string{"external_io", "domain_boundary", "persistence", "api_client", "notification"},
			PathContains: []string{"github", "slack", "db", "persistence", "adapter"},
		},
		HardGates: []Gate{{
			ID:               "domain_port_named_before_adapter",
			Kind:             "boundary_policy",
			CheckLevel:       "human_review",
			PassCondition:    "The closeout identifies the domain port/capability before adapter implementation details.",
			RequiredEvidence: []string{"diff_ref", "test_ref"},
			Waiver:           WaiverPolicy{Allowed: true, RequiresReason: true},
		}, {
			ID:               "adapter_does_not_own_business_policy",
			Kind:             "code_shape",
			CheckLevel:       "human_review",
			PassCondition:    "Business policy remains outside the transport/vendor adapter.",
			RequiredEvidence: []string{"diff_ref", "review_note"},
			Waiver:           WaiverPolicy{Allowed: true, RequiresReason: true},
		}},
		SoftGates: []string{"Avoid transport/vendor types in the domain boundary."},
		Procedure: []string{
			"Name the domain capability needed from the external system.",
			"Define or identify the port at the domain/orchestration boundary.",
			"Implement adapter code at the effect boundary.",
		},
		AntiPatterns: []string{
			"Business decision in HTTP/DB/vendor adapter.",
			"Domain layer importing transport-specific types.",
		},
		RequiredEvidence: []string{"diff_ref", "test_ref", "review_note"},
		Waiver:           WaiverPolicy{Allowed: true, RequiresReason: true},
		Priority:         25,
	}
}

func functionalCoreImperativeShell() Definition {
	return Definition{
		ID:      "functional-core-imperative-shell",
		Version: CatalogVersion,
		Title:   "Functional core, imperative shell",
		Summary: "Business logic belongs in a pure core; side effects stay at the boundary.",
		Intent:  "Keep policy, calculation, and state transitions testable without mocks by separating them from IO orchestration.",
		AppliesTo: Applicability{
			TaskKinds:   []string{"refactor", "architecture"},
			RiskSignals: []string{"business_logic", "domain_policy", "side_effect_boundary", "io_boundary", "hard_to_test"},
		},
		DoesNotApplyTo: Applicability{
			TaskKinds: []string{TaskMechanicalEdit, TaskFormattingOnly},
		},
		HardGates: []Gate{{
			ID:               "pure_core_named_before_shell",
			Kind:             "code_shape",
			CheckLevel:       "human_review",
			PassCondition:    "The closeout identifies the pure core boundary and the side-effect shell boundary.",
			RequiredEvidence: []string{"diff_ref", "test_ref", "review_note"},
			Waiver:           WaiverPolicy{Allowed: true, RequiresReason: true},
		}, {
			ID:               "core_testable_without_external_effects",
			Kind:             "test_evidence",
			CheckLevel:       "human_review",
			PassCondition:    "Core behavior is tested or inspectable without mocking transport, filesystem, network, or database details.",
			RequiredEvidence: []string{"unit_test", "diff_ref", "review_note"},
			Waiver:           WaiverPolicy{Allowed: true, RequiresReason: true},
		}},
		SoftGates: []string{"Prefer data-in/data-out functions for the core and a thin orchestration shell for effects."},
		Procedure: []string{
			"Name the domain calculation, policy, or transition that must be pure.",
			"Keep IO and external calls in the outer shell.",
			"Verify core behavior through direct inputs and outputs where practical.",
		},
		AntiPatterns: []string{
			"Business policy hidden in a database, HTTP, CLI, or vendor adapter.",
			"Tests that require mocks because the core and shell are inseparable.",
		},
		RequiredEvidence: []string{"diff_ref", "test_ref", "review_note"},
		Waiver:           WaiverPolicy{Allowed: true, RequiresReason: true},
		Priority:         35,
	}
}

func makeIllegalStatesUnrepresentable() Definition {
	return Definition{
		ID:      "make-illegal-states-unrepresentable",
		Version: CatalogVersion,
		Title:   "Make illegal states unrepresentable",
		Summary: "Domain invariants should live in types or canonical data shapes, not only in after-the-fact checks.",
		Intent:  "Reduce invalid combinations by representing state with constrained constructors, sum types, typestates, or canonical forms.",
		AppliesTo: Applicability{
			RiskSignals: []string{"state_model", "state_machine", "invariant", "schema_change", "domain_model", "invalid_state"},
		},
		DoesNotApplyTo: Applicability{
			TaskKinds: []string{TaskMechanicalEdit, TaskFormattingOnly},
		},
		HardGates: []Gate{{
			ID:               "invariant_encoded_in_shape",
			Kind:             "domain_invariant",
			CheckLevel:       "human_review",
			PassCondition:    "The closeout names the invariant and shows where the representation makes invalid states impossible or narrower.",
			RequiredEvidence: []string{"diff_ref", "review_note", "test_ref"},
			Waiver:           WaiverPolicy{Allowed: true, RequiresReason: true},
		}, {
			ID:               "single_canonical_state_form",
			Kind:             "domain_model",
			CheckLevel:       "human_review",
			PassCondition:    "There is one canonical representation for the domain state at this layer, or the exception is recorded.",
			RequiredEvidence: []string{"diff_ref", "review_note"},
			Waiver:           WaiverPolicy{Allowed: true, RequiresReason: true},
		}},
		SoftGates: []string{"Prefer explicit variants over boolean flag combinations when the domain has named states."},
		Procedure: []string{
			"Name the invalid state or invalid transition.",
			"Choose a representation that cannot express it, or narrows where it can enter.",
			"Parse weak boundary data into the strong representation before core logic.",
		},
		AntiPatterns: []string{
			"Boolean flag clusters that can express contradictory states.",
			"Runtime validation repeated across call sites instead of a canonical constructor or type.",
		},
		RequiredEvidence: []string{"diff_ref", "review_note", "test_ref"},
		Waiver:           WaiverPolicy{Allowed: true, RequiresReason: true},
		Priority:         40,
	}
}
