package p14acceptance

import (
	"bytes"
	"fmt"
	"maps"
	"slices"
	"testing"
)

const (
	p14RuntimeIdentityBuilderID = "runtime.identity.v1"
	p14HostResumeBuilderID      = "host.supervised-resume.v1"
	p14LoopCleanupBuilderID     = "host.loop-cleanup.v1"
	p14AgentCodeGraphBuilderID  = "agent.orientation-code-graph.v1"
	p14AgentMemoryBuilderID     = "agent.orientation-typed-memory.v1"

	p14LiveProtocolSemanticSchema = "haft.p14.live-protocol-semantic/v1"
	p14LiveProtocolSurfaceSchema  = "haft.p14.live-protocol-surface/v1"
	p14LiveProtocolOracleSchema   = "haft.p14.live-protocol-local-oracle/v1"

	p14AgentMemoryEntityID       = "entity:p14-agent-memory-roundtrip-20260728"
	p14AgentMemoryLabel          = "P14 agent typed-memory round trip"
	p14AgentMemoryContext        = "haft-project"
	p14AgentMemoryQuery          = "P14 task-level entity round trip"
	p14AgentMemoryAlias          = "p14-agent-memory-roundtrip-20260728"
	p14AgentMemoryReason         = "explicit_operator_request"
	p14AgentMemoryProvenance     = "p14:agent-typed-memory-orientation:explicit-save"
	p14AgentMemoryIdempotencyKey = "p14-agent-memory-roundtrip-20260728"
)

var p14LiveProtocolBuilderIDs = []string{
	p14RuntimeIdentityBuilderID,
	p14HostResumeBuilderID,
	p14LoopCleanupBuilderID,
	p14AgentCodeGraphBuilderID,
	p14AgentMemoryBuilderID,
}

var p14AgentOrientationBuilderIDs = []string{
	p14AgentCodeGraphBuilderID,
	p14AgentMemoryBuilderID,
}

type p14LiveProtocolPolicy struct {
	ScenarioID        string
	BuilderID         string
	ExpectedEffect    string
	PredicateIDs      []string
	SurfaceObservers  map[string]string
	SurfaceChecks     map[string][]string
	SurfaceProbes     map[string]p14LiveProtocolProbe
	AgentPrompt       *p14LiveProtocolAgentPrompt
	PersistencePrompt *p14LiveProtocolAgentPrompt
}

type p14LiveProtocolSemanticRequest struct {
	Schema           string   `json:"schema"`
	ScenarioID       string   `json:"scenario_id"`
	ExpectedEffect   string   `json:"expected_effect"`
	RequiredBindings []string `json:"required_bindings"`
	PredicateIDs     []string `json:"predicate_ids"`
}

type p14LiveProtocolSurface struct {
	Schema                string                      `json:"schema"`
	SemanticRequestDigest string                      `json:"semantic_request_digest"`
	Surface               string                      `json:"surface"`
	Observer              string                      `json:"observer"`
	RequiredBindings      []string                    `json:"required_bindings"`
	CheckIDs              []string                    `json:"check_ids"`
	Probe                 *p14LiveProtocolProbe       `json:"probe,omitempty"`
	AgentPrompt           *p14LiveProtocolAgentPrompt `json:"agent_prompt,omitempty"`
	PersistencePrompt     *p14LiveProtocolAgentPrompt `json:"persistence_prompt,omitempty"`
}

type p14LiveProtocolProbe struct {
	Tool string         `json:"tool"`
	Args map[string]any `json:"args"`
}

type p14LiveProtocolAgentPrompt struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

type p14LiveProtocolLocalOracle struct {
	Schema                string   `json:"schema"`
	SemanticRequestDigest string   `json:"semantic_request_digest"`
	PredicateIDs          []string `json:"predicate_ids"`
	LocalOracleTests      []string `json:"local_oracle_tests"`
}

func p14AgentMemoryEstablishArgs() map[string]any {
	return map[string]any{
		"action":                 "establish",
		"entity_id":              p14AgentMemoryEntityID,
		"label":                  p14AgentMemoryLabel,
		"bounded_context_ref":    p14AgentMemoryContext,
		"aliases":                []string{p14AgentMemoryQuery, p14AgentMemoryAlias},
		"persistence_reason":     p14AgentMemoryReason,
		"request_provenance_ref": p14AgentMemoryProvenance,
		"idempotency_key":        p14AgentMemoryIdempotencyKey,
	}
}

func p14AgentMemoryResolveAbsentArgs() map[string]any {
	return map[string]any{
		"action": "memory",
		"memory_request": map[string]any{
			"mode":                "resolve",
			"contract_version":    "haft.memory.v1",
			"basis":               map[string]any{"kind": "project_current"},
			"query":               p14AgentMemoryQuery,
			"bounded_context_ref": p14AgentMemoryContext,
			"max_candidates":      5,
		},
	}
}

func p14AgentMemoryResolveExactArgs() map[string]any {
	return map[string]any{
		"action": "memory",
		"memory_request": map[string]any{
			"mode":                "resolve",
			"contract_version":    "haft.memory.v1",
			"basis":               map[string]any{"kind": "project_current"},
			"query":               p14AgentMemoryAlias,
			"bounded_context_ref": p14AgentMemoryContext,
			"max_candidates":      5,
		},
	}
}

func p14AgentMemoryNeighborhoodArgs() map[string]any {
	return map[string]any{
		"action": "memory",
		"memory_request": map[string]any{
			"mode":                "neighborhood",
			"contract_version":    "haft.memory.v1",
			"basis":               map[string]any{"kind": "project_current"},
			"entity_ref":          p14AgentMemoryEntityRefArgs(),
			"bounded_context_ref": p14AgentMemoryContext,
			"view":                p14AgentMemoryView(),
			"read_budget":         p14AgentMemoryReadBudget(),
		},
	}
}

func p14AgentMemoryRecallArgs() map[string]any {
	return map[string]any{
		"action": "memory",
		"memory_request": map[string]any{
			"mode":                "recall",
			"contract_version":    "haft.memory.v1",
			"basis":               map[string]any{"kind": "project_current"},
			"entity_ref":          p14AgentMemoryEntityRefArgs(),
			"bounded_context_ref": p14AgentMemoryContext,
			"view":                p14AgentMemoryView(),
			"read_budget":         p14AgentMemoryReadBudget(),
			"query":               "P14 typed-memory round-trip records",
			"candidate_budget":    map[string]any{"max_candidates": 8},
		},
	}
}

func p14AgentMemoryEntityRefArgs() map[string]any {
	return map[string]any{
		"ref_kind_id":  "U.EntityRef",
		"reference_id": p14AgentMemoryEntityID,
	}
}

func p14AgentMemoryView() map[string]any {
	return map[string]any{
		"projection_profile_ref": "agent_orientation.v2",
		"requested_facets": []string{
			"epistemes",
			"problems",
			"alternatives",
			"decisions",
			"specifications",
			"evidence",
			"work",
			"implementation",
			"unresolved",
		},
		"detail":          "standard",
		"include_history": false,
	}
}

func p14AgentMemoryReadBudget() map[string]any {
	return map[string]any{
		"max_facets":                     9,
		"max_items_per_facet":            20,
		"max_relation_paths_per_item":    8,
		"max_carrier_excerpt_characters": 4096,
		"max_provenance_depth":           4,
	}
}

func p14LiveProtocolPolicies() map[string]p14LiveProtocolPolicy {
	return map[string]p14LiveProtocolPolicy{
		p14RuntimeIdentityBuilderID: {
			ScenarioID:     "runtime_identity",
			BuilderID:      p14RuntimeIdentityBuilderID,
			ExpectedEffect: "none",
			PredicateIDs: []string{
				"p14.runtime.executable_identity.v1",
				"p14.runtime.process_generation.v1",
				"p14.runtime.project_basis.v1",
				"p14.runtime.carrier_basis.v1",
				"p14.runtime.private_nonce.v1",
			},
			SurfaceObservers: map[string]string{
				"installed_cli": "restart_checkpoint.verify.installed_cli.v1",
				"live_mcp":      "restart_checkpoint.challenge.live_mcp.v1",
				"host_process":  "restart_checkpoint.verify.host_process.v1",
			},
			SurfaceChecks: map[string][]string{
				"installed_cli": {
					"candidate_path_and_digest_match",
					"frozen_query_and_typeenv_basis_match",
				},
				"live_mcp": {
					"candidate_path_and_digest_match",
					"pid_start_time_and_project_cwd_match",
					"private_nonce_receipt_is_code_observed",
				},
				"host_process": {
					"frozen_project_basis_match",
					"frozen_carrier_digests_match",
					"new_process_generation_is_observed",
				},
			},
			SurfaceProbes: map[string]p14LiveProtocolProbe{
				"live_mcp": {
					Tool: "haft_query",
					Args: map[string]any{
						"action": "status",
						"full":   false,
					},
				},
			},
		},
		p14HostResumeBuilderID: {
			ScenarioID:     "host_resume",
			BuilderID:      p14HostResumeBuilderID,
			ExpectedEffect: "host_process_transition",
			PredicateIDs: []string{
				"p14.host.exact_task_resume.v1",
				"p14.host.exact_task_resume_once.v1",
				"p14.host.single_writer.v1",
				"p14.host.bounded_fallback.v1",
			},
			SurfaceObservers: map[string]string{
				"host_process": "restart_checkpoint.resume_and_verify.v1",
			},
			SurfaceChecks: map[string][]string{
				"host_process": {
					"exact_task_deep_link_resumed",
					"exact_task_acquired_one_resume_lease",
					"resume_lease_has_one_writer",
					"same_link_fallback_wake_count_at_most_one",
				},
			},
		},
		p14LoopCleanupBuilderID: {
			ScenarioID:     "loop_cleanup",
			BuilderID:      p14LoopCleanupBuilderID,
			ExpectedEffect: "host_process_transition",
			PredicateIDs: []string{
				"p14.cleanup.full_digest_history.v1",
				"p14.cleanup.private_state.v1",
				"p14.cleanup.stage_removal.v1",
				"p14.cleanup.launchd_absent.v1",
			},
			SurfaceObservers: map[string]string{
				"host_process": "restart_checkpoint.verify_cleanup.v1",
			},
			SurfaceChecks: map[string][]string{
				"host_process": {
					"full_candidate_digest_is_durably_reserved",
					"a_to_b_to_a_is_rejected",
					"private_receipts_and_attempt_history_are_gitignored",
					"checkpoint_stages_are_removed",
					"launchd_label_is_observed_absent",
				},
			},
		},
		p14AgentCodeGraphBuilderID: {
			ScenarioID:     "agent_code_graph_orientation",
			BuilderID:      p14AgentCodeGraphBuilderID,
			ExpectedEffect: "host_process_observation",
			PredicateIDs: []string{
				"p14.agent.code_graph.host_generation.v1",
				"p14.agent.code_graph.installed_surface.v1",
				"p14.agent.code_graph.read_only_probe.v1",
				"p14.agent.code_graph.no_identity_autoselect.v1",
			},
			SurfaceObservers: map[string]string{
				"host_process":  "restart_checkpoint.verify.agent_code_graph.v1",
				"installed_cli": "installed_cli.verify.agent_code_graph.v1",
				"live_mcp":      "actual_codex.verify.agent_code_graph.v1",
			},
			SurfaceChecks: map[string][]string{
				"host_process": {
					"frozen_project_basis_match",
					"new_process_generation_is_observed",
				},
				"installed_cli": {
					"installed_code_graph_surface_observed",
					"installed_code_graph_oracle_matched",
				},
				"live_mcp": {
					"actual_task_tool_call_projection_bound",
					"captured_response_digest_bound",
					"agent_prompt_transcript_bound",
					"closed_agent_code_graph_orientation",
				},
			},
			SurfaceProbes: map[string]p14LiveProtocolProbe{
				"live_mcp": {
					Tool: "haft_query",
					Args: map[string]any{
						"action": "explore",
						"symbol": "NeighborhoodRead",
						"file":   "internal/cli/memory_read_runtime.go",
						"view":   "working",
					},
				},
			},
			AgentPrompt: &p14LiveProtocolAgentPrompt{
				ID: "agent_code_graph_orientation_prompt",
				Text: "Using only the MCP tools available in this task, " +
					"orient the exact NeighborhoodRead implementation in " +
					"internal/cli/memory_read_runtime.go and identify its " +
					"current module-level decision context. Do not edit " +
					"files or persist project memory.",
			},
		},
		p14AgentMemoryBuilderID: {
			ScenarioID:     "agent_typed_memory_orientation",
			BuilderID:      p14AgentMemoryBuilderID,
			ExpectedEffect: "host_process_observation_and_non_binding_entity_establishment",
			PredicateIDs: []string{
				"p14.agent.memory.host_generation.v1",
				"p14.agent.memory.installed_surface.v1",
				"p14.agent.memory.read_only_probe.v1",
				"p14.agent.memory.no_implicit_admission.v1",
				"p14.agent.memory.explicit_save_gate.v1",
				"p14.agent.memory.single_establishment_commit.v1",
				"p14.agent.memory.entity_ref_round_trip.v1",
				"p14.agent.memory.non_authorizing_interpretation.v1",
			},
			SurfaceObservers: map[string]string{
				"host_process":  "restart_checkpoint.verify.agent_memory.v1",
				"installed_cli": "installed_cli.verify.agent_memory.v1",
				"live_mcp":      "actual_codex.verify.agent_memory.v1",
			},
			SurfaceChecks: map[string][]string{
				"host_process": {
					"frozen_project_basis_match",
					"new_process_generation_is_observed",
				},
				"installed_cli": {
					"installed_memory_read_surface_observed",
					"installed_memory_read_oracle_matched",
				},
				"live_mcp": {
					"actual_task_tool_call_projection_bound",
					"captured_response_digest_bound",
					"agent_prompt_transcript_bound",
					"memory_graph_unchanged_before_explicit_save",
					"explicit_save_prompt_bound",
					"single_entity_establishment_commit_and_replay",
					"verbatim_entity_ref_round_trip",
					"next_read_exactly_composable",
					"memory_results_non_authorizing",
					"closed_agent_memory_orientation",
				},
			},
			SurfaceProbes: map[string]p14LiveProtocolProbe{
				"live_mcp": {
					Tool: "haft_query",
					Args: p14AgentMemoryResolveAbsentArgs(),
				},
			},
			AgentPrompt: &p14LiveProtocolAgentPrompt{
				ID: "agent_typed_memory_orientation_prompt",
				Text: "Using only the MCP tools and installed Haft guidance " +
					"available in this task, resolve the EntityOfConcern named " +
					p14AgentMemoryQuery + " in bounded context " +
					p14AgentMemoryContext + ". Treat absence as read-only " +
					"information: do not establish an entity, admit memory, " +
					"or invoke any other persistence action.",
			},
			PersistencePrompt: &p14LiveProtocolAgentPrompt{
				ID: "agent_typed_memory_explicit_save_prompt",
				Text: "Explicitly save the existing EntityOfConcern named " +
					p14AgentMemoryLabel + " with stable identity " +
					p14AgentMemoryEntityID + " in bounded context " +
					p14AgentMemoryContext + ". Use only haft_entity establish " +
					"with aliases exactly [" + p14AgentMemoryQuery + ", " +
					p14AgentMemoryAlias + "], persistence reason " +
					p14AgentMemoryReason + ", request provenance " +
					p14AgentMemoryProvenance + ", and idempotency key " +
					p14AgentMemoryIdempotencyKey + ".",
			},
		},
	}
}

func buildP14LiveProtocolScenario(
	declared scenarioContract,
) (preparedP14Scenario, error) {
	policies := p14LiveProtocolPolicies()
	policy, present := policies[declared.RequestBuilder]
	if !present {
		return preparedP14Scenario{}, fmt.Errorf(
			"P14 live protocol builder %q is unknown",
			declared.RequestBuilder,
		)
	}
	if err := validateP14LiveProtocolPolicy(declared, policy); err != nil {
		return preparedP14Scenario{}, err
	}
	semantic := p14LiveProtocolSemanticRequest{
		Schema:           p14LiveProtocolSemanticSchema,
		ScenarioID:       declared.ID,
		ExpectedEffect:   declared.ExpectedEffect,
		RequiredBindings: slices.Clone(declared.RequiredBindings),
		PredicateIDs:     slices.Clone(policy.PredicateIDs),
	}
	semanticBytes, err := marshalP14CanonicalJSON(semantic)
	if err != nil {
		return preparedP14Scenario{}, err
	}
	semanticDigest := p14Digest(semanticBytes)
	requests := make([]preparedP14Request, 0, len(declared.Surfaces))
	for _, surface := range declared.Surfaces {
		probe, hasProbe := policy.SurfaceProbes[surface]
		var probeRef *p14LiveProtocolProbe
		if hasProbe {
			copied := p14LiveProtocolProbe{
				Tool: probe.Tool,
				Args: maps.Clone(probe.Args),
			}
			probeRef = &copied
		}
		payload := p14LiveProtocolSurface{
			Schema:                p14LiveProtocolSurfaceSchema,
			SemanticRequestDigest: semanticDigest,
			Surface:               surface,
			Observer:              policy.SurfaceObservers[surface],
			RequiredBindings:      slices.Clone(declared.RequiredBindings),
			CheckIDs:              slices.Clone(policy.SurfaceChecks[surface]),
			Probe:                 probeRef,
		}
		if surface == "live_mcp" && policy.AgentPrompt != nil {
			copiedPrompt := *policy.AgentPrompt
			payload.AgentPrompt = &copiedPrompt
		}
		if surface == "live_mcp" && policy.PersistencePrompt != nil {
			copiedPrompt := *policy.PersistencePrompt
			payload.PersistencePrompt = &copiedPrompt
		}
		payloadBytes, encodeErr := marshalP14CanonicalJSON(payload)
		if encodeErr != nil {
			return preparedP14Scenario{}, encodeErr
		}
		requests = append(requests, preparedP14Request{
			Surface:               surface,
			Builder:               declared.RequestBuilder,
			Encoding:              "observation_protocol_json",
			CanonicalPayload:      string(payloadBytes),
			PayloadDigest:         p14Digest(payloadBytes),
			SemanticRequestDigest: semanticDigest,
		})
	}
	localOracle := p14LiveProtocolLocalOracle{
		Schema:                p14LiveProtocolOracleSchema,
		SemanticRequestDigest: semanticDigest,
		PredicateIDs:          slices.Clone(policy.PredicateIDs),
		LocalOracleTests:      slices.Clone(declared.LocalOracleTests),
	}
	localOracleBytes, err := marshalP14CanonicalJSON(localOracle)
	if err != nil {
		return preparedP14Scenario{}, err
	}
	return preparedP14Scenario{
		ID:                       declared.ID,
		SemanticRequestCanonical: string(semanticBytes),
		SemanticRequestDigest:    semanticDigest,
		Requests:                 requests,
		Oracle: preparedP14Oracle{
			Kind:                    declared.OracleKind,
			PredicateIDs:            slices.Clone(policy.PredicateIDs),
			ExpectedEffect:          declared.ExpectedEffect,
			LocalOracleOutputDigest: p14Digest(localOracleBytes),
		},
	}, nil
}

func validateP14LiveProtocolPolicy(
	declared scenarioContract,
	policy p14LiveProtocolPolicy,
) error {
	if declared.ID != policy.ScenarioID ||
		declared.RequestBuilder != policy.BuilderID ||
		declared.OracleKind != "live_predicate" ||
		declared.ExpectedEffect != policy.ExpectedEffect {
		return fmt.Errorf("P14 live protocol policy differs for %q", declared.ID)
	}
	if hasBlankOrDuplicate(policy.PredicateIDs) || len(policy.PredicateIDs) == 0 {
		return fmt.Errorf("P14 live protocol predicates are invalid for %q", declared.ID)
	}
	if len(policy.SurfaceObservers) != len(declared.Surfaces) ||
		len(policy.SurfaceChecks) != len(declared.Surfaces) {
		return fmt.Errorf("P14 live protocol surfaces differ for %q", declared.ID)
	}
	for _, surface := range declared.Surfaces {
		observer := policy.SurfaceObservers[surface]
		checks := policy.SurfaceChecks[surface]
		if observer == "" || len(checks) == 0 || hasBlankOrDuplicate(checks) {
			return fmt.Errorf(
				"P14 live protocol surface %q is invalid for %q",
				surface,
				declared.ID,
			)
		}
	}
	for surface, probe := range policy.SurfaceProbes {
		if !slices.Contains(declared.Surfaces, surface) ||
			probe.Tool == "" ||
			len(probe.Args) == 0 {
			return fmt.Errorf(
				"P14 live protocol probe %q is invalid for %q",
				surface,
				declared.ID,
			)
		}
	}
	agentScenario := slices.Contains(
		p14AgentOrientationBuilderIDs,
		declared.RequestBuilder,
	)
	if agentScenario &&
		(policy.AgentPrompt == nil ||
			policy.AgentPrompt.ID == "" ||
			policy.AgentPrompt.Text == "") {
		return fmt.Errorf(
			"P14 agent live protocol prompt is absent for %q",
			declared.ID,
		)
	}
	if !agentScenario && policy.AgentPrompt != nil {
		return fmt.Errorf(
			"P14 non-agent live protocol has an agent prompt for %q",
			declared.ID,
		)
	}
	persistencePromptExpected := declared.ID ==
		"agent_typed_memory_orientation"
	if persistencePromptExpected &&
		(policy.PersistencePrompt == nil ||
			policy.PersistencePrompt.ID == "" ||
			policy.PersistencePrompt.Text == "") {
		return fmt.Errorf(
			"P14 agent memory persistence prompt is absent for %q",
			declared.ID,
		)
	}
	if !persistencePromptExpected && policy.PersistencePrompt != nil {
		return fmt.Errorf(
			"P14 unexpected persistence prompt for %q",
			declared.ID,
		)
	}
	return nil
}

func validateP14LiveProtocolPreparedScenario(
	declared scenarioContract,
	scenario preparedP14Scenario,
) error {
	expected, err := buildP14LiveProtocolScenario(declared)
	if err != nil {
		return err
	}
	expectedBytes, err := marshalP14CanonicalJSON(expected)
	if err != nil {
		return err
	}
	actualBytes, err := marshalP14CanonicalJSON(scenario)
	if err != nil {
		return err
	}
	if !bytes.Equal(actualBytes, expectedBytes) {
		return fmt.Errorf("P14 live protocol scenario %q differs", declared.ID)
	}
	return nil
}

func TestP14LiveProtocolBuildersCloseExecutionTimePredicates(t *testing.T) {
	repositoryRoot, err := p14RepositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	contract, _, err := loadRequestOracleContract(repositoryRoot)
	if err != nil {
		t.Fatal(err)
	}
	for _, builderID := range p14LiveProtocolBuilderIDs {
		declared, err := findP14ScenarioContractByBuilder(contract, builderID)
		if err != nil {
			t.Fatal(err)
		}
		scenario, err := buildP14LiveProtocolScenario(declared)
		if err != nil {
			t.Fatal(err)
		}
		if err := validateP14LiveProtocolPreparedScenario(declared, scenario); err != nil {
			t.Fatal(err)
		}
		tampered := scenario
		tampered.Oracle.PredicateIDs = slices.Clone(scenario.Oracle.PredicateIDs)
		tampered.Oracle.PredicateIDs[0] = "caller_supplied_success"
		if err := validateP14LiveProtocolPreparedScenario(declared, tampered); err == nil {
			t.Fatalf("P14 live protocol %q accepted predicate drift", declared.ID)
		}
	}
}
