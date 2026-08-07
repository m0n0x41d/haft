package p14acceptance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"
)

type p14AgentMemoryNormalizedOutput struct {
	Schema                         string                  `json:"schema"`
	ScenarioID                     string                  `json:"scenario_id"`
	PreSaveResultKind              string                  `json:"pre_save_result_kind"`
	PostSaveResultKind             string                  `json:"post_save_result_kind"`
	NeighborhoodResultKind         string                  `json:"neighborhood_result_kind"`
	RecallResultKind               string                  `json:"recall_result_kind"`
	GraphRevisionBefore            int                     `json:"graph_revision_before"`
	GraphRevisionAfter             int                     `json:"graph_revision_after"`
	TypeEnvRef                     string                  `json:"type_env_ref"`
	TypeEnvDigest                  string                  `json:"type_env_digest"`
	EntityRef                      p14AgentMemoryEntityRef `json:"entity_ref"`
	EstablishmentDelivery          string                  `json:"establishment_delivery"`
	ReplayDelivery                 string                  `json:"replay_delivery"`
	NoWriteBeforeExplicitSave      bool                    `json:"no_write_before_explicit_save"`
	ExplicitSavePromptBound        bool                    `json:"explicit_save_prompt_bound"`
	ExactlyOneEstablishmentCommit  bool                    `json:"exactly_one_establishment_commit"`
	EntityRefRoundTrip             bool                    `json:"entity_ref_round_trip"`
	NextReadExactlyComposable      bool                    `json:"next_read_exactly_composable"`
	NonAuthorizing                 bool                    `json:"non_authorizing"`
	EstablishmentArgsTaskLevelOnly bool                    `json:"establishment_args_task_level_only"`
}

type p14AgentMemoryEntityRef struct {
	RefKindID   string `json:"ref_kind_id"`
	ReferenceID string `json:"reference_id"`
}

type p14AgentMemoryReadObservation struct {
	Action         string
	ResultKind     string
	GraphRevision  int
	TypeEnvRef     string
	TypeEnvDigest  string
	EntityRef      *p14AgentMemoryEntityRef
	NonAuthorizing bool
}

type p14AgentMemoryEstablishObservation struct {
	DeliveryKind    string
	EntityRef       p14AgentMemoryEntityRef
	NextReadTool    string
	NextReadArgs    map[string]any
	PersistenceDone bool
	NonAuthorizing  bool
}

func runP14InstalledCLIAgentOrientation(
	_ context.Context,
	execution p14InstalledCLIExecutionContext,
	scenario preparedP14Scenario,
	request preparedP14Request,
) (p14InstalledCLIFamilyResult, error) {
	surface := p14LiveProtocolSurface{}
	if err := decodeP14StrictCompactJSON(
		request.CanonicalPayload,
		&surface,
		"agent orientation installed CLI surface",
	); err != nil {
		return p14InstalledCLIFamilyResult{}, err
	}
	policy, err := p14LiveProtocolPolicyForScenario(scenario.ID)
	if err != nil {
		return p14InstalledCLIFamilyResult{}, err
	}
	priorScenario, err := p14AgentOrientationPriorScenario(
		scenario.ID,
	)
	if err != nil {
		return p14InstalledCLIFamilyResult{}, err
	}
	prior, present := execution.PriorCaptures[priorScenario]
	satisfied := present &&
		prior.SurfaceObservation.Outcome ==
			p14SurfaceOutcomeObserved &&
		validP14Digest(
			prior.SurfaceObservation.NormalizedResultDigest,
		)
	checkIDs := policy.SurfaceChecks["installed_cli"]
	if surface.Schema != p14LiveProtocolSurfaceSchema ||
		surface.Surface != "installed_cli" ||
		surface.Observer != policy.SurfaceObservers["installed_cli"] ||
		surface.SemanticRequestDigest !=
			scenario.SemanticRequestDigest ||
		len(checkIDs) == 0 {
		return p14InstalledCLIFamilyResult{}, fmt.Errorf(
			"P14 agent orientation installed CLI protocol differs",
		)
	}
	checks := make([]p14InstalledCLICheckReceipt, 0, len(checkIDs))
	for _, id := range checkIDs {
		checks = append(checks, p14InstalledCLICheckReceipt{
			ID:        id,
			Satisfied: satisfied,
		})
	}
	result := p14InstalledCLIFamilyResult{
		Receipt: p14InstalledCLIExecutionReceipt{
			Schema:               p14InstalledCLIReceiptSchema,
			ScenarioID:           scenario.ID,
			Builder:              request.Builder,
			CandidateDigest:      execution.ExecutableDigest,
			RequestPayloadDigest: request.PayloadDigest,
			ObservedAt: time.Now().
				UTC().
				Format(time.RFC3339Nano),
			Checks: checks,
		},
	}
	if !satisfied {
		result.FailureCode = "agent_orientation_installed_surface_missing"
		result.FailureDetail =
			"required installed read-only scenario was absent or mismatched"
	}
	return result, nil
}

func p14AgentOrientationPriorScenario(
	scenarioID string,
) (string, error) {
	values := map[string]string{
		"agent_code_graph_orientation":   "code_graph_exact_explore",
		"agent_typed_memory_orientation": "unknown_eoc",
	}
	prior := values[scenarioID]
	if prior == "" {
		return "", fmt.Errorf(
			"P14 agent orientation scenario %q is open",
			scenarioID,
		)
	}
	return prior, nil
}

func normalizeP14CodexMCPAgentOrientation(
	_ preparedRequestOracleCarrier,
	scenario preparedP14Scenario,
	request preparedP14Request,
	evidence []p14CodexMCPCallEvidence,
) (p14CodexMCPFamilyResult, error) {
	surface := p14LiveProtocolSurface{}
	if err := decodeP14StrictCompactJSON(
		request.CanonicalPayload,
		&surface,
		"actual Codex agent orientation surface",
	); err != nil {
		return p14CodexMCPFamilyResult{}, err
	}
	if surface.Probe == nil ||
		surface.Probe.Tool != "haft_query" ||
		surface.AgentPrompt == nil {
		return p14CodexMCPNormalizedFailure(
			"agent_orientation_mismatch",
			"agent orientation protocol omitted its probe or prompt",
			"agent_prompt_transcript_bound",
			"closed_agent_orientation",
		), nil
	}
	if scenario.ID == "agent_code_graph_orientation" {
		return normalizeP14CodexMCPAgentCodeGraph(
			scenario,
			evidence,
		)
	}
	if scenario.ID == "agent_typed_memory_orientation" {
		return normalizeP14CodexMCPAgentMemory(
			scenario,
			evidence,
		)
	}
	return p14CodexMCPFamilyResult{}, fmt.Errorf(
		"P14 agent orientation scenario %q is open",
		scenario.ID,
	)
}

func normalizeP14CodexMCPAgentCodeGraph(
	scenario preparedP14Scenario,
	evidence []p14CodexMCPCallEvidence,
) (p14CodexMCPFamilyResult, error) {
	if len(evidence) != 1 ||
		evidence[0].CaseID != "orientation_probe" ||
		evidence[0].AgentPrompt == nil ||
		evidence[0].Response.IsError {
		return p14CodexMCPNormalizedFailure(
			"agent_orientation_mismatch",
			"code-graph orientation was not the first prompt-driven tool call",
			"agent_prompt_transcript_bound",
			"closed_agent_code_graph_orientation",
		), nil
	}
	body, err := p14CodexMCPResponseBody(evidence[0])
	if err != nil {
		return p14CodexMCPFamilyResult{}, err
	}
	normalized, normalizeErr := normalizeP14AgentCodeGraphBody(
		scenario,
		body,
	)
	return p14CodexMCPNormalizedResult(
		normalized,
		normalizeErr,
		"agent_prompt_transcript_bound",
		"closed_agent_code_graph_orientation",
	)
}

func normalizeP14CodexMCPAgentMemory(
	scenario preparedP14Scenario,
	evidence []p14CodexMCPCallEvidence,
) (p14CodexMCPFamilyResult, error) {
	expectedCases := []string{
		"basis_before",
		"orientation_probe",
		"basis_after",
		"explicit_save_establish",
		"establish_replay",
		"resolve_exact",
		"neighborhood",
		"recall",
	}
	if len(evidence) != len(expectedCases) {
		return p14CodexMCPNormalizedFailure(
			"agent_orientation_mismatch",
			"typed-memory orientation does not have the closed entity round-trip evidence",
			"agent_prompt_transcript_bound",
			"memory_graph_unchanged_before_explicit_save",
			"explicit_save_prompt_bound",
			"single_entity_establishment_commit_and_replay",
			"verbatim_entity_ref_round_trip",
			"next_read_exactly_composable",
			"memory_results_non_authorizing",
			"closed_agent_memory_orientation",
		), nil
	}
	for index, call := range evidence {
		promptExpected := index == 1 || index == 3
		if call.CaseID != expectedCases[index] ||
			call.Response.IsError ||
			(call.AgentPrompt != nil) != promptExpected {
			return p14CodexMCPNormalizedFailure(
				"agent_orientation_mismatch",
				"typed-memory orientation call order or prompt evidence differs",
				"agent_prompt_transcript_bound",
				"memory_graph_unchanged_before_explicit_save",
				"explicit_save_prompt_bound",
				"single_entity_establishment_commit_and_replay",
				"verbatim_entity_ref_round_trip",
				"next_read_exactly_composable",
				"memory_results_non_authorizing",
				"closed_agent_memory_orientation",
			), nil
		}
	}
	normalized, err := normalizeP14AgentMemoryRoundTrip(
		scenario,
		evidence,
	)
	return p14CodexMCPNormalizedResult(
		normalized,
		err,
		"agent_prompt_transcript_bound",
		"memory_graph_unchanged_before_explicit_save",
		"explicit_save_prompt_bound",
		"single_entity_establishment_commit_and_replay",
		"verbatim_entity_ref_round_trip",
		"next_read_exactly_composable",
		"memory_results_non_authorizing",
		"closed_agent_memory_orientation",
	)
}

func normalizeP14AgentMemoryRoundTrip(
	scenario preparedP14Scenario,
	evidence []p14CodexMCPCallEvidence,
) (p14AgentMemoryNormalizedOutput, error) {
	readIndexes := []int{0, 1, 2, 5, 6, 7}
	reads := make(map[int]p14AgentMemoryReadObservation, len(readIndexes))
	for _, index := range readIndexes {
		body, err := p14CodexMCPResponseBody(evidence[index])
		if err != nil {
			return p14AgentMemoryNormalizedOutput{}, err
		}
		reads[index], err = normalizeP14AgentMemoryReadBody(body)
		if err != nil {
			return p14AgentMemoryNormalizedOutput{}, err
		}
	}
	established, err := normalizeP14AgentMemoryEstablishCall(
		evidence[3],
		"freshly_committed",
	)
	if err != nil {
		return p14AgentMemoryNormalizedOutput{}, err
	}
	replayed, err := normalizeP14AgentMemoryEstablishCall(
		evidence[4],
		"replayed",
	)
	if err != nil {
		return p14AgentMemoryNormalizedOutput{}, err
	}
	if err := validateP14AgentMemoryCallArguments(evidence); err != nil {
		return p14AgentMemoryNormalizedOutput{}, err
	}
	before := reads[0]
	probe := reads[1]
	after := reads[2]
	resolved := reads[5]
	neighborhood := reads[6]
	recall := reads[7]
	if before.Action != "resolve" ||
		probe.Action != "resolve" ||
		after.Action != "resolve" ||
		before.ResultKind != "known_absent" ||
		probe.ResultKind != "known_absent" ||
		after.ResultKind != "known_absent" {
		return p14AgentMemoryNormalizedOutput{}, fmt.Errorf(
			"pre-save resolves did not remain known_absent",
		)
	}
	noWriteBeforeSave := before.GraphRevision == probe.GraphRevision &&
		probe.GraphRevision == after.GraphRevision &&
		before.TypeEnvRef == probe.TypeEnvRef &&
		probe.TypeEnvRef == after.TypeEnvRef &&
		before.TypeEnvDigest == probe.TypeEnvDigest &&
		probe.TypeEnvDigest == after.TypeEnvDigest
	if !noWriteBeforeSave {
		return p14AgentMemoryNormalizedOutput{}, fmt.Errorf(
			"typed-memory graph changed before the explicit save prompt",
		)
	}
	if resolved.Action != "resolve" ||
		resolved.ResultKind != "exact_entity" ||
		neighborhood.Action != "neighborhood" ||
		neighborhood.ResultKind != "exact_neighborhood" ||
		recall.Action != "recall" ||
		(recall.ResultKind != "abstained" &&
			recall.ResultKind != "scoped_memory_candidate_set") {
		return p14AgentMemoryNormalizedOutput{}, fmt.Errorf(
			"post-establishment memory reads did not return the closed exact scope",
		)
	}
	postRevisionStable := resolved.GraphRevision ==
		after.GraphRevision+1 &&
		neighborhood.GraphRevision == resolved.GraphRevision &&
		recall.GraphRevision == resolved.GraphRevision &&
		resolved.TypeEnvRef == after.TypeEnvRef &&
		neighborhood.TypeEnvRef == resolved.TypeEnvRef &&
		recall.TypeEnvRef == resolved.TypeEnvRef &&
		resolved.TypeEnvDigest == after.TypeEnvDigest &&
		neighborhood.TypeEnvDigest == resolved.TypeEnvDigest &&
		recall.TypeEnvDigest == resolved.TypeEnvDigest
	exactlyOneCommit := established.DeliveryKind ==
		"freshly_committed" &&
		replayed.DeliveryKind == "replayed" &&
		established.PersistenceDone &&
		replayed.PersistenceDone &&
		postRevisionStable
	if !exactlyOneCommit {
		return p14AgentMemoryNormalizedOutput{}, fmt.Errorf(
			"entity establishment did not produce one commit and one replay",
		)
	}
	expectedRef := p14AgentMemoryEntityRef{
		RefKindID:   "U.EntityRef",
		ReferenceID: p14AgentMemoryEntityID,
	}
	neighborhoodInputRef, err := p14AgentMemoryInputEntityRef(
		evidence[6],
	)
	if err != nil {
		return p14AgentMemoryNormalizedOutput{}, err
	}
	recallInputRef, err := p14AgentMemoryInputEntityRef(evidence[7])
	if err != nil {
		return p14AgentMemoryNormalizedOutput{}, err
	}
	establishInput := p14JSONMapFromCanonical(
		evidence[3].Transcript.ArgsCanonical,
	)
	entityRefRoundTrip := established.EntityRef == expectedRef &&
		replayed.EntityRef == expectedRef &&
		p14JSONText(establishInput["entity_id"]) ==
			expectedRef.ReferenceID &&
		resolved.EntityRef != nil &&
		*resolved.EntityRef == expectedRef &&
		neighborhoodInputRef == expectedRef &&
		neighborhood.EntityRef != nil &&
		*neighborhood.EntityRef == expectedRef &&
		recallInputRef == expectedRef &&
		recall.EntityRef != nil &&
		*recall.EntityRef == expectedRef
	if !entityRefRoundTrip {
		return p14AgentMemoryNormalizedOutput{}, fmt.Errorf(
			"canonical U.EntityRef changed across establishment and reads",
		)
	}
	nextReadComposable, err := p14AgentMemoryNextReadComposable(
		established,
		replayed,
		evidence[6],
	)
	if err != nil {
		return p14AgentMemoryNormalizedOutput{}, err
	}
	nonAuthorizing := before.NonAuthorizing &&
		probe.NonAuthorizing &&
		after.NonAuthorizing &&
		established.NonAuthorizing &&
		replayed.NonAuthorizing &&
		resolved.NonAuthorizing &&
		neighborhood.NonAuthorizing &&
		recall.NonAuthorizing
	if !nonAuthorizing {
		return p14AgentMemoryNormalizedOutput{}, fmt.Errorf(
			"entity round-trip inferred authority from persistence or reads",
		)
	}
	return p14AgentMemoryNormalizedOutput{
		Schema:                         "haft.p14.agent-memory-normalized/v2",
		ScenarioID:                     scenario.ID,
		PreSaveResultKind:              before.ResultKind,
		PostSaveResultKind:             resolved.ResultKind,
		NeighborhoodResultKind:         neighborhood.ResultKind,
		RecallResultKind:               recall.ResultKind,
		GraphRevisionBefore:            after.GraphRevision,
		GraphRevisionAfter:             resolved.GraphRevision,
		TypeEnvRef:                     resolved.TypeEnvRef,
		TypeEnvDigest:                  resolved.TypeEnvDigest,
		EntityRef:                      expectedRef,
		EstablishmentDelivery:          established.DeliveryKind,
		ReplayDelivery:                 replayed.DeliveryKind,
		NoWriteBeforeExplicitSave:      noWriteBeforeSave,
		ExplicitSavePromptBound:        evidence[3].AgentPrompt != nil,
		ExactlyOneEstablishmentCommit:  exactlyOneCommit,
		EntityRefRoundTrip:             entityRefRoundTrip,
		NextReadExactlyComposable:      nextReadComposable,
		NonAuthorizing:                 nonAuthorizing,
		EstablishmentArgsTaskLevelOnly: true,
	}, nil
}

func normalizeP14AgentCodeGraphBody(
	scenario preparedP14Scenario,
	body []byte,
) (any, error) {
	semantic := p14CodeExploreSemanticRequest{
		Schema:                    p14CodeExploreSemanticSchema,
		ScenarioID:                scenario.ID,
		RequestKind:               "symbol",
		Symbol:                    "NeighborhoodRead",
		File:                      "internal/cli/memory_read_runtime.go",
		View:                      "working",
		ExpectedKind:              "resolved",
		ExpectedSeedKind:          "resolved_seed",
		RequireModuleDecisionRef:  true,
		RequiredModuleDecisionRef: p14CodeExploreCurrentDecisionRef,
	}
	normalized, _, err := normalizeP14CodeExploreObservation(
		semantic,
		p14FPFProjectionCommandObservation{
			Stdout:   slices.Clone(body),
			ExitCode: 0,
		},
	)
	return normalized, err
}

func normalizeP14AgentMemoryReadBody(
	body []byte,
) (p14AgentMemoryReadObservation, error) {
	payload, err := p14AgentMemoryResponsePayload(body)
	if err != nil {
		return p14AgentMemoryReadObservation{}, err
	}
	action := p14JSONText(payload["action"])
	resultKind := p14JSONText(payload["result_kind"])
	result := p14JSONMap(payload["result"])
	closedKinds := map[string]map[string]struct{}{
		"resolve": {
			"exact_entity":         {},
			"known_absent":         {},
			"entity_candidates":    {},
			"resolution_unsettled": {},
			"retry_required":       {},
		},
		"neighborhood": {
			"exact_neighborhood": {},
			"abstained":          {},
			"retry_required":     {},
		},
		"recall": {
			"scoped_memory_candidate_set": {},
			"abstained":                   {},
			"retry_required":              {},
		},
	}
	actionKinds := closedKinds[action]
	_, closedKind := actionKinds[resultKind]
	if p14JSONText(payload["contract_version"]) !=
		"haft.memory.v1" ||
		!closedKind ||
		len(result) == 0 ||
		payload["persistence_disposition"] != nil ||
		payload["admission_batch"] != nil {
		return p14AgentMemoryReadObservation{}, fmt.Errorf(
			"agent memory response violates the closed read-only envelope",
		)
	}
	snapshot := p14JSONMap(result["snapshot_basis"])
	graphRevision, validGraphRevision := p14JSONInt(
		snapshot["graph_revision"],
	)
	typeEnvRef := p14JSONText(snapshot["type_env_ref"])
	typeEnvDigest := p14JSONText(snapshot["type_env_digest"])
	interpretation := p14JSONMap(result["interpretation_contract"])
	nonAuthorizing := p14AgentMemoryInterpretationIsNonAuthorizing(
		interpretation,
	)
	if !validGraphRevision ||
		graphRevision <= 0 ||
		typeEnvRef == "" ||
		!validP14Digest(typeEnvDigest) ||
		!nonAuthorizing {
		return p14AgentMemoryReadObservation{}, fmt.Errorf(
			"agent memory response basis or interpretation differs",
		)
	}
	var entityRef *p14AgentMemoryEntityRef
	entityPayload := p14AgentMemoryReadEntityPayload(
		action,
		resultKind,
		result,
	)
	if len(entityPayload) > 0 {
		parsed, parseErr := p14AgentMemoryParseEntityRef(entityPayload)
		if parseErr != nil {
			return p14AgentMemoryReadObservation{}, parseErr
		}
		entityRef = &parsed
	}
	return p14AgentMemoryReadObservation{
		Action:         action,
		ResultKind:     resultKind,
		GraphRevision:  graphRevision,
		TypeEnvRef:     typeEnvRef,
		TypeEnvDigest:  typeEnvDigest,
		EntityRef:      entityRef,
		NonAuthorizing: nonAuthorizing,
	}, nil
}

func p14AgentMemoryResponsePayload(
	body []byte,
) (map[string]any, error) {
	canonical := bytes.TrimSpace(body)
	if !canonicalCompactJSON(canonical) {
		return nil, fmt.Errorf(
			"agent memory response is not compact JSON",
		)
	}
	decoder := json.NewDecoder(bytes.NewReader(canonical))
	decoder.UseNumber()
	payload := map[string]any{}
	if err := decoder.Decode(&payload); err != nil {
		return nil, fmt.Errorf(
			"decode agent memory response: %w",
			err,
		)
	}
	return payload, nil
}

func p14AgentMemoryReadEntityPayload(
	action string,
	resultKind string,
	result map[string]any,
) map[string]any {
	switch action {
	case "resolve":
		if resultKind != "exact_entity" {
			return nil
		}
		entity := p14JSONMap(result["entity"])
		return p14JSONMap(entity["entity_ref"])
	case "neighborhood":
		if resultKind != "exact_neighborhood" {
			return nil
		}
		view := p14JSONMap(result["memory_view_context"])
		return p14JSONMap(view["entity_ref"])
	case "recall":
		scope := p14JSONMap(result["scope"])
		return p14JSONMap(scope["entity_ref"])
	default:
		return nil
	}
}

func p14AgentMemoryInterpretationIsNonAuthorizing(
	interpretation map[string]any,
) bool {
	return interpretation["applicability"] == "not_implied" &&
		interpretation["authority"] == "not_granted" &&
		interpretation["truth"] == "not_implied" &&
		interpretation["work_order"] == "not_implied"
}

func normalizeP14AgentMemoryEstablishCall(
	call p14CodexMCPCallEvidence,
	expectedDelivery string,
) (p14AgentMemoryEstablishObservation, error) {
	body, err := p14CodexMCPResponseBody(call)
	if err != nil {
		return p14AgentMemoryEstablishObservation{}, err
	}
	payload, err := p14AgentMemoryResponsePayload(body)
	if err != nil {
		return p14AgentMemoryEstablishObservation{}, err
	}
	persistence := p14JSONMap(payload["persistence"])
	entityRef, err := p14AgentMemoryParseEntityRef(
		p14JSONMap(payload["entity_ref"]),
	)
	if err != nil {
		return p14AgentMemoryEstablishObservation{}, err
	}
	nextRead := p14JSONMap(payload["next_read"])
	nextReadArgs := p14JSONMap(nextRead["arguments"])
	if p14JSONText(payload["contract_version"]) != "haft.entity.v1" ||
		p14JSONText(payload["action"]) != "establish" ||
		p14JSONText(payload["result"]) != "established" ||
		p14JSONText(payload["delivery_kind"]) != expectedDelivery ||
		p14JSONText(payload["label"]) != p14AgentMemoryLabel ||
		p14JSONText(payload["bounded_context_ref"]) !=
			p14AgentMemoryContext ||
		!slices.Equal(
			p14JSONStringSlice(payload["aliases"]),
			[]string{p14AgentMemoryQuery, p14AgentMemoryAlias},
		) ||
		persistence["performed"] != true ||
		persistence["authority_granted"] != false ||
		p14JSONText(nextRead["tool"]) != "haft_query" ||
		len(nextReadArgs) == 0 {
		return p14AgentMemoryEstablishObservation{}, fmt.Errorf(
			"agent entity establishment response differs for %s",
			expectedDelivery,
		)
	}
	return p14AgentMemoryEstablishObservation{
		DeliveryKind:    expectedDelivery,
		EntityRef:       entityRef,
		NextReadTool:    "haft_query",
		NextReadArgs:    nextReadArgs,
		PersistenceDone: true,
		NonAuthorizing:  true,
	}, nil
}

func p14AgentMemoryParseEntityRef(
	payload map[string]any,
) (p14AgentMemoryEntityRef, error) {
	result := p14AgentMemoryEntityRef{
		RefKindID:   p14JSONText(payload["ref_kind_id"]),
		ReferenceID: p14JSONText(payload["reference_id"]),
	}
	if len(payload) != 2 ||
		result.RefKindID != "U.EntityRef" ||
		result.ReferenceID == "" {
		return p14AgentMemoryEntityRef{}, fmt.Errorf(
			"agent memory response has no exact U.EntityRef",
		)
	}
	return result, nil
}

func p14JSONStringSlice(value any) []string {
	switch exact := value.(type) {
	case []string:
		return slices.Clone(exact)
	case []any:
		result := make([]string, 0, len(exact))
		for _, item := range exact {
			text, valid := item.(string)
			if !valid {
				return nil
			}
			result = append(result, text)
		}
		return result
	default:
		return nil
	}
}

func validateP14AgentMemoryCallArguments(
	evidence []p14CodexMCPCallEvidence,
) error {
	expected := []map[string]any{
		p14AgentMemoryResolveAbsentArgs(),
		p14AgentMemoryResolveAbsentArgs(),
		p14AgentMemoryResolveAbsentArgs(),
		p14AgentMemoryEstablishArgs(),
		p14AgentMemoryEstablishArgs(),
		p14AgentMemoryResolveExactArgs(),
		p14AgentMemoryNeighborhoodArgs(),
		p14AgentMemoryRecallArgs(),
	}
	for index, call := range evidence {
		actual := p14JSONMapFromCanonical(
			call.Transcript.ArgsCanonical,
		)
		actualBytes, actualErr := marshalP14CanonicalJSON(actual)
		expectedBytes, expectedErr := marshalP14CanonicalJSON(
			expected[index],
		)
		if actualErr != nil ||
			expectedErr != nil ||
			!bytes.Equal(actualBytes, expectedBytes) {
			return fmt.Errorf(
				"agent memory call %q arguments differ",
				call.CaseID,
			)
		}
	}
	if evidence[3].Transcript.ArgsCanonical !=
		evidence[4].Transcript.ArgsCanonical {
		return fmt.Errorf(
			"entity establishment replay changed the request or idempotency key",
		)
	}
	establish := p14JSONMapFromCanonical(
		evidence[3].Transcript.ArgsCanonical,
	)
	keys := make(map[string]int)
	collectP14FPFJSONKeys(establish, keys)
	for _, forbidden := range []string{
		"type_env",
		"typeenv",
		"basis",
		"ref_kind",
		"authority",
		"change_set",
		"memory_change_set",
		"graph_revision",
	} {
		if keys[forbidden] > 0 {
			return fmt.Errorf(
				"task-level entity establishment leaked %q",
				forbidden,
			)
		}
	}
	required := []string{
		"action",
		"aliases",
		"bounded_context_ref",
		"entity_id",
		"idempotency_key",
		"label",
		"persistence_reason",
		"request_provenance_ref",
	}
	actualKeys := slices.Sorted(maps.Keys(establish))
	if !slices.Equal(actualKeys, required) {
		return fmt.Errorf(
			"task-level entity establishment keys differ: %#v",
			actualKeys,
		)
	}
	aliases := p14JSONStringSlice(establish["aliases"])
	if len(aliases) != 2 ||
		!slices.IsSorted(aliases) ||
		aliases[0] == aliases[1] {
		return fmt.Errorf(
			"task-level entity aliases are not canonical and unique",
		)
	}
	return nil
}

func p14JSONMapFromCanonical(value string) map[string]any {
	decoder := json.NewDecoder(strings.NewReader(value))
	decoder.UseNumber()
	result := map[string]any{}
	if decoder.Decode(&result) != nil {
		return nil
	}
	return result
}

func p14AgentMemoryNextReadComposable(
	established p14AgentMemoryEstablishObservation,
	replayed p14AgentMemoryEstablishObservation,
	neighborhood p14CodexMCPCallEvidence,
) (bool, error) {
	planned := p14JSONMapFromCanonical(
		neighborhood.Transcript.ArgsCanonical,
	)
	plannedBytes, err := marshalP14CanonicalJSON(planned)
	if err != nil {
		return false, err
	}
	establishedBytes, err := marshalP14CanonicalJSON(
		established.NextReadArgs,
	)
	if err != nil {
		return false, err
	}
	replayedBytes, err := marshalP14CanonicalJSON(
		replayed.NextReadArgs,
	)
	if err != nil {
		return false, err
	}
	composable := established.NextReadTool == neighborhood.Transcript.Tool &&
		replayed.NextReadTool == neighborhood.Transcript.Tool &&
		bytes.Equal(establishedBytes, plannedBytes) &&
		bytes.Equal(replayedBytes, plannedBytes)
	if !composable {
		return false, fmt.Errorf(
			"entity establishment next_read is not exactly composable",
		)
	}
	return true, nil
}

func p14AgentMemoryInputEntityRef(
	call p14CodexMCPCallEvidence,
) (p14AgentMemoryEntityRef, error) {
	args := p14JSONMapFromCanonical(call.Transcript.ArgsCanonical)
	memoryRequest := p14JSONMap(args["memory_request"])
	return p14AgentMemoryParseEntityRef(
		p14JSONMap(memoryRequest["entity_ref"]),
	)
}

func TestP14AgentOrientationRequiresInstalledAndActualHostSurfaces(
	t *testing.T,
) {
	root, err := p14RepositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	contract, rawContract, err := loadRequestOracleContract(root)
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := completePreparedInputForTest(
		contract,
		p14Digest(rawContract),
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, scenarioID := range []string{
		"agent_code_graph_orientation",
		"agent_typed_memory_orientation",
	} {
		scenario, err := preparedP14ScenarioByID(
			prepared.Scenarios,
			scenarioID,
		)
		if err != nil {
			t.Fatal(err)
		}
		request, present := p14PreparedSurfaceRequest(
			scenario,
			"live_mcp",
		)
		if !present {
			t.Fatalf(
				"P14 agent scenario %q omits live MCP",
				scenarioID,
			)
		}
		calls, err := p14CodexMCPLiveProtocolCalls(
			scenario,
			request,
		)
		if err != nil {
			t.Fatal(err)
		}
		expectedCallCount := 1
		if scenarioID == "agent_typed_memory_orientation" {
			expectedCallCount = 8
		}
		if len(calls) != expectedCallCount {
			t.Fatalf(
				"P14 agent scenario %q call = %#v",
				scenarioID,
				calls,
			)
		}
		promptCalls := 0
		for _, call := range calls {
			if p14JSONText(call.Args["action"]) == "admit" ||
				call.Args["request"] != nil {
				t.Fatalf(
					"P14 agent scenario %q call = %#v",
					scenarioID,
					call,
				)
			}
			if scenarioID == "agent_typed_memory_orientation" {
				if call.Tool == "haft_entity" {
					if call.Args["action"] != "establish" ||
						call.Args["basis"] != nil ||
						call.Args["type_env"] != nil ||
						call.Args["change_set"] != nil ||
						call.Args["authority_class"] != nil {
						t.Fatalf(
							"P14 agent entity call leaked memory internals: %#v",
							call,
						)
					}
				} else if call.Tool == "haft_query" {
					memoryRequest := p14JSONMap(
						call.Args["memory_request"],
					)
					if len(call.Args) != 2 ||
						call.Args["action"] != "memory" ||
						memoryRequest["action"] != nil ||
						call.Args["mode"] != nil ||
						call.Args["basis"] != nil {
						t.Fatalf(
							"P14 agent memory call is not nested: %#v",
							call,
						)
					}
				} else {
					t.Fatalf(
						"P14 agent memory tool is open: %#v",
						call,
					)
				}
			} else if call.Tool != "haft_query" {
				t.Fatalf(
					"P14 agent code-graph tool differs: %#v",
					call,
				)
			}
			if call.AgentPrompt != nil {
				promptCalls++
				promptCase := call.CaseID == "orientation_probe" ||
					call.CaseID == "explicit_save_establish"
				if !promptCase ||
					call.AgentPrompt.ExpectedToolCallCount != 1 {
					t.Fatalf(
						"P14 agent scenario %q prompt call = %#v",
						scenarioID,
						call,
					)
				}
			}
		}
		expectedPromptCalls := 1
		if scenarioID == "agent_typed_memory_orientation" {
			expectedPromptCalls = 2
		}
		if promptCalls != expectedPromptCalls {
			t.Fatalf(
				"P14 agent scenario %q prompt count = %d",
				scenarioID,
				promptCalls,
			)
		}
	}
}

func TestP14AgentOrientationNormalizerRequiresExplicitEntityRoundTrip(
	t *testing.T,
) {
	root, err := p14RepositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	contract, rawContract, err := loadRequestOracleContract(root)
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := completePreparedInputForTest(
		contract,
		p14Digest(rawContract),
	)
	if err != nil {
		t.Fatal(err)
	}

	codeScenario, err := preparedP14ScenarioByID(
		prepared.Scenarios,
		"agent_code_graph_orientation",
	)
	if err != nil {
		t.Fatal(err)
	}
	codeRequest, present := p14PreparedSurfaceRequest(
		codeScenario,
		"live_mcp",
	)
	if !present {
		t.Fatal("agent code-graph live request is absent")
	}
	codeSemantic := p14CodeExploreSemanticRequest{
		ScenarioID:                codeScenario.ID,
		RequestKind:               "symbol",
		Symbol:                    "NeighborhoodRead",
		File:                      "internal/cli/memory_read_runtime.go",
		View:                      "working",
		ExpectedKind:              "resolved",
		ExpectedSeedKind:          "resolved_seed",
		RequireModuleDecisionRef:  true,
		RequiredModuleDecisionRef: p14CodeExploreCurrentDecisionRef,
	}
	codeBody, err := marshalP14CanonicalJSON(
		syntheticP14CodeExplorePayload(codeSemantic),
	)
	if err != nil {
		t.Fatal(err)
	}
	codeEvidence := []p14CodexMCPCallEvidence{
		p14AgentOrientationNormalizerEvidence(
			"orientation_probe",
			codeBody,
			true,
		),
	}
	codeResult, err := normalizeP14CodexMCPAgentOrientation(
		preparedRequestOracleCarrier{},
		codeScenario,
		codeRequest,
		codeEvidence,
	)
	if err != nil {
		t.Fatal(err)
	}
	assertP14CodexMCPFamilySuccess(t, codeResult, "")
	codeEvidence[0].AgentPrompt = nil
	codeResult, err = normalizeP14CodexMCPAgentOrientation(
		preparedRequestOracleCarrier{},
		codeScenario,
		codeRequest,
		codeEvidence,
	)
	if err != nil {
		t.Fatal(err)
	}
	if codeResult.FailureCode == "" {
		t.Fatal("agent code-graph normalizer accepted missing prompt evidence")
	}

	memoryScenario, err := preparedP14ScenarioByID(
		prepared.Scenarios,
		"agent_typed_memory_orientation",
	)
	if err != nil {
		t.Fatal(err)
	}
	memoryRequest, present := p14PreparedSurfaceRequest(
		memoryScenario,
		"live_mcp",
	)
	if !present {
		t.Fatal("agent typed-memory live request is absent")
	}
	memoryCalls, err := p14CodexMCPLiveProtocolCalls(
		memoryScenario,
		memoryRequest,
	)
	if err != nil {
		t.Fatal(err)
	}
	memoryBodies := [][]byte{
		p14AgentMemoryResolveBody(t, 17, "known_absent"),
		p14AgentMemoryResolveBody(t, 17, "known_absent"),
		p14AgentMemoryResolveBody(t, 17, "known_absent"),
		p14AgentMemoryEstablishBody(t, "freshly_committed"),
		p14AgentMemoryEstablishBody(t, "replayed"),
		p14AgentMemoryResolveBody(t, 18, "exact_entity"),
		p14AgentMemoryNeighborhoodBody(t, 18),
		p14AgentMemoryRecallBody(t, 18),
	}
	memoryEvidence := make(
		[]p14CodexMCPCallEvidence,
		0,
		len(memoryCalls),
	)
	for index, call := range memoryCalls {
		memoryEvidence = append(
			memoryEvidence,
			p14AgentMemoryNormalizerEvidence(
				t,
				call,
				memoryBodies[index],
			),
		)
	}
	memoryResult, err := normalizeP14CodexMCPAgentOrientation(
		preparedRequestOracleCarrier{},
		memoryScenario,
		memoryRequest,
		memoryEvidence,
	)
	if err != nil {
		t.Fatal(err)
	}
	assertP14CodexMCPFamilySuccess(t, memoryResult, "")
	normalized := p14AgentMemoryNormalizedOutput{}
	if err := decodeP14StrictCompactJSON(
		string(memoryResult.Normalized),
		&normalized,
		"agent memory round-trip normalized result",
	); err != nil {
		t.Fatal(err)
	}
	if normalized.Schema !=
		"haft.p14.agent-memory-normalized/v2" ||
		normalized.PreSaveResultKind != "known_absent" ||
		normalized.PostSaveResultKind != "exact_entity" ||
		normalized.GraphRevisionAfter !=
			normalized.GraphRevisionBefore+1 ||
		normalized.EntityRef.RefKindID != "U.EntityRef" ||
		normalized.EntityRef.ReferenceID !=
			p14AgentMemoryEntityID ||
		!normalized.NoWriteBeforeExplicitSave ||
		!normalized.ExplicitSavePromptBound ||
		!normalized.ExactlyOneEstablishmentCommit ||
		!normalized.EntityRefRoundTrip ||
		!normalized.NextReadExactlyComposable ||
		!normalized.NonAuthorizing ||
		!normalized.EstablishmentArgsTaskLevelOnly {
		t.Fatalf(
			"agent memory normalized round trip = %#v",
			normalized,
		)
	}
	memoryEvidence[2] = p14AgentMemoryNormalizerEvidence(
		t,
		memoryCalls[2],
		p14AgentMemoryResolveBody(t, 18, "known_absent"),
	)
	memoryResult, err = normalizeP14CodexMCPAgentOrientation(
		preparedRequestOracleCarrier{},
		memoryScenario,
		memoryRequest,
		memoryEvidence,
	)
	if err != nil {
		t.Fatal(err)
	}
	if memoryResult.FailureCode != "normalization_failed" {
		t.Fatalf(
			"agent memory changed-basis result = %#v",
			memoryResult,
		)
	}
	memoryEvidence[2] = p14AgentMemoryNormalizerEvidence(
		t,
		memoryCalls[2],
		memoryBodies[2],
	)
	replayDrift := slices.Clone(memoryEvidence)
	replayArgs := p14AgentMemoryEstablishArgs()
	replayArgs["idempotency_key"] = "changed-replay-key"
	replayRaw, err := marshalP14CanonicalJSON(replayArgs)
	if err != nil {
		t.Fatal(err)
	}
	replayDrift[4].Transcript.ArgsCanonical = string(replayRaw)
	replayDrift[4].Transcript.ArgsDigest = p14Digest(replayRaw)
	p14AssertAgentMemoryNormalizationFails(
		t,
		memoryScenario,
		memoryRequest,
		replayDrift,
		"changed replay request",
	)
	rawInternals := slices.Clone(memoryEvidence)
	rawEstablishArgs := p14AgentMemoryEstablishArgs()
	rawEstablishArgs["basis"] = map[string]any{
		"kind": "project_current",
	}
	rawEstablishBytes, err := marshalP14CanonicalJSON(
		rawEstablishArgs,
	)
	if err != nil {
		t.Fatal(err)
	}
	rawInternals[3].Transcript.ArgsCanonical =
		string(rawEstablishBytes)
	rawInternals[3].Transcript.ArgsDigest =
		p14Digest(rawEstablishBytes)
	p14AssertAgentMemoryNormalizationFails(
		t,
		memoryScenario,
		memoryRequest,
		rawInternals,
		"raw project basis in task-level establishment",
	)
	refDrift := slices.Clone(memoryEvidence)
	refDrift[3] = p14AgentMemoryNormalizerEvidence(
		t,
		memoryCalls[3],
		bytes.ReplaceAll(
			memoryBodies[3],
			[]byte(p14AgentMemoryEntityID),
			[]byte("entity:p14-forged-roundtrip"),
		),
	)
	p14AssertAgentMemoryNormalizationFails(
		t,
		memoryScenario,
		memoryRequest,
		refDrift,
		"changed establishment EntityRef",
	)
	nextReadDrift := slices.Clone(memoryEvidence)
	nextReadDrift[3] = p14AgentMemoryNormalizerEvidence(
		t,
		memoryCalls[3],
		bytes.Replace(
			memoryBodies[3],
			[]byte(`"max_facets":9`),
			[]byte(`"max_facets":8`),
			1,
		),
	)
	p14AssertAgentMemoryNormalizationFails(
		t,
		memoryScenario,
		memoryRequest,
		nextReadDrift,
		"non-composable next_read",
	)
	authorityDrift := slices.Clone(memoryEvidence)
	authorityDrift[3] = p14AgentMemoryNormalizerEvidence(
		t,
		memoryCalls[3],
		bytes.Replace(
			memoryBodies[3],
			[]byte(`"authority_granted":false`),
			[]byte(`"authority_granted":true`),
			1,
		),
	)
	p14AssertAgentMemoryNormalizationFails(
		t,
		memoryScenario,
		memoryRequest,
		authorityDrift,
		"authorizing persistence interpretation",
	)
	implicitSave := slices.Clone(memoryEvidence)
	implicitSave[3].AgentPrompt = nil
	p14AssertAgentMemoryNormalizationFails(
		t,
		memoryScenario,
		memoryRequest,
		implicitSave,
		"implicit save without prompt",
	)
}

func p14AssertAgentMemoryNormalizationFails(
	t *testing.T,
	scenario preparedP14Scenario,
	request preparedP14Request,
	evidence []p14CodexMCPCallEvidence,
	reason string,
) {
	t.Helper()
	result, err := normalizeP14CodexMCPAgentOrientation(
		preparedRequestOracleCarrier{},
		scenario,
		request,
		evidence,
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.FailureCode == "" {
		t.Fatalf(
			"agent memory normalizer accepted %s",
			reason,
		)
	}
}

func p14AgentOrientationNormalizerEvidence(
	caseID string,
	body []byte,
	prompt bool,
) p14CodexMCPCallEvidence {
	evidence := p14CodexMCPNormalizerEvidence(
		caseID,
		false,
		body,
	)
	if prompt {
		evidence.AgentPrompt =
			&p14CodexMCPPromptTranscriptProjection{
				PromptID: "synthetic-agent-prompt",
			}
	}
	return evidence
}

func p14AgentMemoryNormalizerEvidence(
	t *testing.T,
	call p14CodexMCPCallDefinition,
	body []byte,
) p14CodexMCPCallEvidence {
	t.Helper()
	evidence := p14AgentOrientationNormalizerEvidence(
		call.CaseID,
		body,
		call.AgentPrompt != nil,
	)
	evidence.Transcript.Tool = call.Tool
	args, err := marshalP14CanonicalJSON(call.Args)
	if err != nil {
		t.Fatal(err)
	}
	evidence.Transcript.ArgsCanonical = string(args)
	evidence.Transcript.ArgsDigest = p14Digest(args)
	return evidence
}

func p14AgentMemoryResolveBody(
	t *testing.T,
	graphRevision int,
	resultKind string,
) []byte {
	t.Helper()
	result := p14AgentMemorySyntheticReadResult(graphRevision)
	if resultKind == "exact_entity" {
		result["entity"] = map[string]any{
			"entity_ref":          p14AgentMemoryEntityRefArgs(),
			"bounded_context_ref": p14AgentMemoryContext,
			"label":               p14AgentMemoryLabel,
			"aliases": []string{
				p14AgentMemoryQuery,
				p14AgentMemoryAlias,
			},
			"provenance_ref":       p14AgentMemoryProvenance,
			"resolution_basis_ref": "resolution:p14-agent-memory",
		}
		result["resolution_witnesses"] = []any{}
	}
	payload := map[string]any{
		"contract_version": "haft.memory.v1",
		"action":           "resolve",
		"result_kind":      resultKind,
		"result":           result,
	}
	raw, err := marshalP14CanonicalJSON(payload)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func p14AgentMemoryEstablishBody(
	t *testing.T,
	delivery string,
) []byte {
	t.Helper()
	payload := map[string]any{
		"contract_version":    "haft.entity.v1",
		"action":              "establish",
		"result":              "established",
		"delivery_kind":       delivery,
		"entity_ref":          p14AgentMemoryEntityRefArgs(),
		"label":               p14AgentMemoryLabel,
		"bounded_context_ref": p14AgentMemoryContext,
		"aliases": []string{
			p14AgentMemoryQuery,
			p14AgentMemoryAlias,
		},
		"next_read": map[string]any{
			"tool":      "haft_query",
			"arguments": p14AgentMemoryNeighborhoodArgs(),
		},
		"persistence": map[string]any{
			"performed":         true,
			"authority_granted": false,
		},
	}
	raw, err := marshalP14CanonicalJSON(payload)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func p14AgentMemoryNeighborhoodBody(
	t *testing.T,
	graphRevision int,
) []byte {
	t.Helper()
	result := p14AgentMemorySyntheticReadResult(graphRevision)
	result["schema"] = "haft.memory.neighborhood.v1"
	result["memory_view_context"] = map[string]any{
		"entity_ref":             p14AgentMemoryEntityRefArgs(),
		"bounded_context_ref":    p14AgentMemoryContext,
		"projection_profile_ref": "agent_orientation.v2",
	}
	payload := map[string]any{
		"contract_version": "haft.memory.v1",
		"action":           "neighborhood",
		"result_kind":      "exact_neighborhood",
		"result_digest":    p14TestDigest("agent-memory-neighborhood"),
		"result":           result,
	}
	raw, err := marshalP14CanonicalJSON(payload)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func p14AgentMemoryRecallBody(
	t *testing.T,
	graphRevision int,
) []byte {
	t.Helper()
	result := p14AgentMemorySyntheticReadResult(graphRevision)
	result["scope"] = map[string]any{
		"entity_ref":             p14AgentMemoryEntityRefArgs(),
		"bounded_context_ref":    p14AgentMemoryContext,
		"projection_profile_ref": "agent_orientation.v2",
	}
	result["inspected_producers"] = []string{"producer:p14-agent-memory"}
	result["basis"] = map[string]any{"kind": "no_candidate_matched"}
	payload := map[string]any{
		"contract_version": "haft.memory.v1",
		"action":           "recall",
		"result_kind":      "abstained",
		"result":           result,
	}
	raw, err := marshalP14CanonicalJSON(payload)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func p14AgentMemorySyntheticReadResult(
	graphRevision int,
) map[string]any {
	return map[string]any{
		"snapshot_basis": map[string]any{
			"graph_revision": graphRevision,
			"type_env_ref": "typeenv:" +
				p14TestDigest("agent-memory-typeenv"),
			"type_env_digest": p14TestDigest(
				"agent-memory-typeenv",
			),
		},
		"interpretation_contract": map[string]any{
			"structure":               "exact_at_snapshot",
			"identity":                "exact",
			"relational_records":      "assertions_exact_at_snapshot",
			"ranking":                 "not_applicable",
			"truth":                   "not_implied",
			"applicability":           "not_implied",
			"authority":               "not_granted",
			"work_order":              "not_implied",
			"completeness":            "bounded",
			"hydrate_before_reliance": false,
		},
	}
}
