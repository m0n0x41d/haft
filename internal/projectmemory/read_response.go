package projectmemory

import (
	"encoding/json"
	"fmt"

	"github.com/m0n0x41d/haft/internal/projectmemory/memoryresolve"
	"github.com/m0n0x41d/haft/internal/projectmemory/neighborhood"
	"github.com/m0n0x41d/haft/internal/projectmemory/scopedrecall"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorywire"
)

type memoryReadResponseEnvelope struct {
	ContractVersion string `json:"contract_version"`
	Action          string `json:"action"`
	ResultKind      string `json:"result_kind"`
	ResultDigest    string `json:"result_digest,omitempty"`
	Result          any    `json:"result"`
}

func EncodeResolutionReadResponse(
	result memoryresolve.EntityResolutionResult,
) ([]byte, error) {
	payload, err := resolutionReadPayload(result)
	if err != nil {
		return nil, err
	}
	return encodeMemoryReadEnvelope(
		typedmemorywire.ActionResolve,
		string(result.Kind()),
		"",
		payload,
	)
}

func EncodeNeighborhoodReadResponse(
	result neighborhood.NeighborhoodResult,
) ([]byte, error) {
	payload, digest, err := neighborhoodReadPayload(result)
	if err != nil {
		return nil, err
	}
	return encodeMemoryReadEnvelope(
		typedmemorywire.ActionNeighborhood,
		string(result.Kind()),
		digest,
		payload,
	)
}

func EncodeScopedRecallReadResponse(
	result scopedrecall.ScopedRecallResult,
) ([]byte, error) {
	payload, err := scopedRecallReadPayload(result)
	if err != nil {
		return nil, err
	}
	return encodeMemoryReadEnvelope(
		typedmemorywire.ActionRecall,
		string(result.Kind()),
		"",
		payload,
	)
}

func encodeMemoryReadEnvelope(
	action string,
	resultKind string,
	resultDigest string,
	result any,
) ([]byte, error) {
	if action == "" || resultKind == "" || result == nil {
		return nil, fmt.Errorf("memory-read response envelope is incomplete")
	}
	envelope := memoryReadResponseEnvelope{
		ContractVersion: typedmemorywire.ContractVersion,
		Action:          action,
		ResultKind:      resultKind,
		ResultDigest:    resultDigest,
		Result:          result,
	}
	encoded, err := json.Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf("encode memory-read response: %w", err)
	}
	return append(encoded, '\n'), nil
}

func resolutionReadPayload(
	result memoryresolve.EntityResolutionResult,
) (any, error) {
	if result == nil {
		return nil, fmt.Errorf("EntityOfConcern resolution result is required")
	}
	if !result.ResolutionScope().Valid() ||
		!result.SnapshotBasis().Valid() ||
		!result.Interpretation().Valid() {
		return nil, fmt.Errorf(
			"EntityOfConcern resolution result is invalid",
		)
	}
	common := map[string]any{
		"resolution_scope": resolutionScopePayload(
			result.ResolutionScope(),
		),
		"snapshot_basis": snapshotBasisPayload(
			result.SnapshotBasis(),
		),
		"interpretation_contract": interpretationPayload(
			result.Interpretation(),
		),
	}
	switch exact := result.(type) {
	case memoryresolve.ExactEntity:
		common["entity"] = resolutionUnitPayload(exact.Entity())
		common["resolution_witnesses"] = resolutionWitnessesPayload(
			exact.ResolutionWitnesses(),
		)
	case memoryresolve.KnownAbsent:
		common["inspected_index"] = map[string]any{
			"index_ref":     exact.InspectedIndex().String(),
			"index_version": exact.InspectedIndexVersion().String(),
		}
		common["completeness_basis_ref"] =
			exact.CompletenessBasis().String()
	case memoryresolve.EntityCandidates:
		candidates := exact.Candidates()
		presented := make([]map[string]any, 0, len(candidates))
		for _, candidate := range candidates {
			presented = append(presented, map[string]any{
				"entity": resolutionUnitPayload(
					candidate.Entity(),
				),
				"rank": candidate.Rank(),
				"resolution_witnesses": resolutionWitnessesPayload(
					candidate.ResolutionWitnesses(),
				),
			})
		}
		common["candidates"] = presented
		common["candidate_set_coverage"] =
			resolutionCoveragePayload(exact.Coverage())
		common["applied_budget"] = map[string]any{
			"requested":        exact.AppliedBudget().Requested(),
			"included":         exact.AppliedBudget().Included(),
			"omitted_at_least": exact.AppliedBudget().OmittedAtLeast(),
		}
	case memoryresolve.ResolutionUnsettled:
		common["issues"] = resolutionIssuesPayload(exact.Issues())
	case memoryresolve.ResolutionRetryRequired:
		cause, err := retryCausePayload(exact.Cause())
		if err != nil {
			return nil, err
		}
		common["observed_snapshot"] = snapshotBasisPayload(
			exact.ObservedSnapshot(),
		)
		common["required_snapshot"] = snapshotBasisPayload(
			exact.RequiredSnapshot(),
		)
		common["cause"] = cause
		common["retry_operation"] = string(exact.RetryOperation())
	default:
		return nil, fmt.Errorf(
			"unsupported EntityOfConcern resolution result %T",
			result,
		)
	}
	return common, nil
}

func neighborhoodReadPayload(
	result neighborhood.NeighborhoodResult,
) (any, string, error) {
	if result == nil {
		return nil, "", fmt.Errorf("memory neighborhood result is required")
	}
	switch exact := result.(type) {
	case neighborhood.ExactNeighborhood:
		if !exact.Valid() {
			return nil, "", fmt.Errorf(
				"exact memory neighborhood is invalid",
			)
		}
		return json.RawMessage(exact.CanonicalBytes()),
			exact.Digest().String(),
			nil
	case neighborhood.RetryRequiredResult:
		if !exact.RequiredSnapshot().Valid() ||
			!exact.Interpretation().Valid() {
			return nil, "", fmt.Errorf(
				"memory-neighborhood retry result is invalid",
			)
		}
		cause, err := retryCausePayload(exact.Cause())
		if err != nil {
			return nil, "", err
		}
		return map[string]any{
			"cause": cause,
			"required_snapshot": snapshotBasisPayload(
				exact.RequiredSnapshot(),
			),
			"retry_operation": string(exact.RetryOperation()),
			"interpretation_contract": interpretationPayload(
				exact.Interpretation(),
			),
		}, "", nil
	case neighborhood.AbstainedResult:
		if exact.Basis() == nil ||
			len(exact.InspectedSources()) == 0 ||
			!exact.Interpretation().Valid() {
			return nil, "", fmt.Errorf(
				"memory-neighborhood abstention result is invalid",
			)
		}
		return map[string]any{
			"basis": neighborhoodAbstentionPayload(exact.Basis()),
			"inspected_sources": inspectedSourcesPayload(
				exact.InspectedSources(),
			),
			"interpretation_contract": interpretationPayload(
				exact.Interpretation(),
			),
		}, "", nil
	default:
		return nil, "", fmt.Errorf(
			"unsupported memory-neighborhood result %T",
			result,
		)
	}
}

func scopedRecallReadPayload(
	result scopedrecall.ScopedRecallResult,
) (any, error) {
	if result == nil {
		return nil, fmt.Errorf("scoped memory-recall result is required")
	}
	if !result.Scope().Valid() ||
		!result.SnapshotBasis().Valid() ||
		!result.Interpretation().Valid() {
		return nil, fmt.Errorf(
			"scoped memory-recall result is invalid",
		)
	}
	common := map[string]any{
		"scope": scopedRecallScopePayload(result.Scope()),
		"snapshot_basis": snapshotBasisPayload(
			result.SnapshotBasis(),
		),
		"interpretation_contract": interpretationPayload(
			result.Interpretation(),
		),
	}
	switch exact := result.(type) {
	case scopedrecall.ScopedMemoryCandidateSet:
		common["candidates"] = recallCandidatesPayload(
			exact.Candidates(),
		)
		common["candidate_set_coverage"] = producerCoveragePayload(
			exact.ProducerCoverage(),
		)
		common["applied_budget"] = map[string]any{
			"requested":        exact.AppliedBudget().Requested(),
			"included":         exact.AppliedBudget().Included(),
			"omitted_at_least": exact.AppliedBudget().OmittedAtLeast(),
		}
	case scopedrecall.ScopedRetryRequired:
		cause, err := retryCausePayload(exact.Cause())
		if err != nil {
			return nil, err
		}
		common["cause"] = cause
		common["required_snapshot"] = snapshotBasisPayload(
			exact.RequiredSnapshot(),
		)
		common["retry_operation"] = string(exact.RetryOperation())
	case scopedrecall.ScopedRecallAbstained:
		common["inspected_producers"] = producerRefsPayload(
			exact.InspectedProducers(),
		)
		common["basis"] = scopedRecallAbstentionPayload(exact.Basis())
	default:
		return nil, fmt.Errorf(
			"unsupported scoped memory-recall result %T",
			result,
		)
	}
	return common, nil
}

func resolutionScopePayload(
	scope memoryresolve.ResolutionScope,
) map[string]any {
	result := map[string]any{
		"query": scope.Query().Original(),
		"context_kind": string(
			scope.Context().Kind(),
		),
	}
	exact, ok := scope.Context().(memoryresolve.ExactContext)
	if ok {
		result["bounded_context_ref"] = exact.Context().String()
	}
	return result
}

func resolutionUnitPayload(
	unit memoryresolve.ResolutionUnit,
) map[string]any {
	aliases := unit.Aliases()
	presentedAliases := make([]string, 0, len(aliases))
	for _, alias := range aliases {
		presentedAliases = append(presentedAliases, alias.String())
	}
	return map[string]any{
		"entity_ref":           persistedRefPayload(unit.Entity()),
		"bounded_context_ref":  unit.Context().String(),
		"label":                unit.Label().String(),
		"aliases":              presentedAliases,
		"provenance_ref":       unit.Provenance().String(),
		"resolution_basis_ref": unit.Basis().String(),
	}
}

func resolutionWitnessesPayload(
	witnesses []memoryresolve.ResolutionWitness,
) []map[string]any {
	result := make([]map[string]any, 0, len(witnesses))
	for _, witness := range witnesses {
		result = append(result, map[string]any{
			"kind":                 string(witness.Kind()),
			"matched":              witness.Matched(),
			"resolution_basis_ref": witness.Basis().String(),
		})
	}
	return result
}

func resolutionCoveragePayload(
	coverage memoryresolve.CandidateSetCoverage,
) map[string]any {
	result := map[string]any{
		"index_ref":        coverage.IndexRef().String(),
		"index_version":    coverage.IndexVersion().String(),
		"inspected":        coverage.InspectedCount(),
		"included":         coverage.IncludedCount(),
		"omitted_at_least": coverage.OmittedAtLeast(),
	}
	cursor, found := coverage.Cursor()
	if found {
		result["continuation_cursor"] = map[string]any{
			"digest":        cursor.Digest().String(),
			"next_offset":   cursor.NextOffset(),
			"index_ref":     cursor.IndexRef().String(),
			"index_version": cursor.IndexVersion().String(),
			"snapshot_basis": snapshotBasisPayload(
				cursor.SnapshotBasis(),
			),
			"resolution_scope": resolutionScopePayload(
				cursor.ResolutionScope(),
			),
		}
	}
	return result
}

func resolutionIssuesPayload(
	issues []memoryresolve.ResolutionBasisIssue,
) []map[string]any {
	result := make([]map[string]any, 0, len(issues))
	for _, issue := range issues {
		presented := map[string]any{"kind": string(issue.Kind())}
		switch exact := issue.(type) {
		case memoryresolve.ContextNotResolvedIssue:
			presented["query_context"] = exact.QueryContext()
		case memoryresolve.AliasConflictIssue:
			presented["alias"] = exact.Alias().String()
			presented["candidate_entity_refs"] = persistedRefsPayload(
				exact.CandidateEntityRefs(),
			)
		case memoryresolve.ReviewedSplitCandidatesIssue:
			presented["historical_entity_id"] =
				exact.HistoricalEntity().String()
			presented["candidate_entity_refs"] = persistedRefsPayload(
				exact.CandidateEntityRefs(),
			)
			presented["reconciliation_ref"] =
				exact.Reconciliation().String()
			history := exact.ReconciliationHistory()
			historyRefs := make([]string, 0, len(history))
			for _, reference := range history {
				historyRefs = append(historyRefs, reference.String())
			}
			presented["reconciliation_history_refs"] = historyRefs
		case memoryresolve.LegacyIdentityUnboundIssue:
			presented["legacy_ref"] = exact.LegacyRef().String()
		case memoryresolve.IncompleteResolutionIndexIssue:
			presented["producer_ref"] = exact.ProducerRef().String()
			presented["missing_basis_ref"] =
				exact.MissingBasis().String()
		}
		result = append(result, presented)
	}
	return result
}

func scopedRecallScopePayload(
	scope scopedrecall.ExactRecallScope,
) map[string]any {
	return map[string]any{
		"entity_ref":             persistedRefPayload(scope.Entity()),
		"bounded_context_ref":    scope.Context().String(),
		"projection_profile_ref": scope.ProfileRef().String(),
	}
}

func recallCandidatesPayload(
	candidates []scopedrecall.RecallCandidate,
) []map[string]any {
	result := make([]map[string]any, 0, len(candidates))
	for _, candidate := range candidates {
		unit := candidate.Unit()
		result = append(result, map[string]any{
			"recall_unit": map[string]any{
				"unit_id": unit.ID().String(),
				"scope": scopedRecallScopePayload(
					unit.Scope(),
				),
				"snapshot_basis": snapshotBasisPayload(
					unit.SnapshotBasis(),
				),
				"projection_basis_digest": unit.
					ProjectionBasisDigest().
					String(),
				"facet":     string(unit.Facet()),
				"item_kind": string(unit.ItemKind()),
				"reference": persistedRefPayload(
					unit.Reference(),
				),
				"text":           unit.Text().String(),
				"postures":       itemPosturesPayload(unit.Postures()),
				"provenance_ref": unit.Provenance().String(),
				"inclusion_witnesses": relationWitnessesPayload(
					unit.InclusionWitnesses(),
				),
				"content_digest": unit.ContentDigest().String(),
				"projection_schema_version": unit.
					ProjectionSchemaVersion(),
			},
			"producer_ref":     candidate.ProducerRef().String(),
			"producer_version": candidate.ProducerVersion().String(),
			"rank":             candidate.Rank(),
			"matched_terms":    candidate.MatchedTerms(),
			"exact_phrase_matched": candidate.
				ExactPhraseMatched(),
		})
	}
	return result
}

func producerCoveragePayload(
	coverage []scopedrecall.ProducerCoverage,
) []map[string]any {
	result := make([]map[string]any, 0, len(coverage))
	for _, item := range coverage {
		presented := map[string]any{
			"kind":            string(item.Kind()),
			"producer_ref":    item.ProducerRef().String(),
			"inspected_count": item.InspectedCount(),
		}
		switch exact := item.(type) {
		case scopedrecall.PartialProducerCoverage:
			presented["omitted_at_least"] = exact.OmittedAtLeast()
			presented["continuation_cursor"] =
				recallCursorPayload(exact.Cursor())
		case scopedrecall.UnavailableProducerCoverage:
			presented["missing_basis_ref"] =
				exact.MissingBasis().String()
		}
		result = append(result, presented)
	}
	return result
}

func recallCursorPayload(
	cursor scopedrecall.RecallCursor,
) map[string]any {
	return map[string]any{
		"digest":         cursor.Digest(),
		"scope":          scopedRecallScopePayload(cursor.Scope()),
		"snapshot_basis": snapshotBasisPayload(cursor.SnapshotBasis()),
		"producer_ref":   cursor.ProducerRef().String(),
		"query_digest":   cursor.QueryDigest(),
		"next_offset":    cursor.NextOffset(),
	}
}

func scopedRecallAbstentionPayload(
	basis scopedrecall.ScopedRecallAbstentionBasis,
) map[string]any {
	result := map[string]any{"kind": string(basis.Kind())}
	switch exact := basis.(type) {
	case scopedrecall.NoMatchingMemoryBasis:
		result["complete_producer_refs"] = producerRefsPayload(
			exact.CompleteProducerRefs(),
		)
	case scopedrecall.NoUsableProducerBasis:
		result["unavailable_producer_refs"] = producerRefsPayload(
			exact.UnavailableProducerRefs(),
		)
		result["missing_basis_ref"] = exact.MissingBasis().String()
	}
	return result
}

func producerRefsPayload(
	refs []scopedrecall.ProducerRef,
) []string {
	result := make([]string, 0, len(refs))
	for _, ref := range refs {
		result = append(result, ref.String())
	}
	return result
}

func neighborhoodAbstentionPayload(
	basis neighborhood.ReadAbstentionBasis,
) map[string]any {
	result := map[string]any{"kind": string(basis.Kind())}
	switch exact := basis.(type) {
	case neighborhood.EntityOrContextNotFoundBasis:
		result["entity_ref"] = persistedRefPayload(exact.Entity())
		result["bounded_context_ref"] = exact.Context().String()
		result["snapshot_basis"] = snapshotBasisPayload(exact.Snapshot())
	case neighborhood.NoAdmissibleFacetBasis:
		result["issues"] = facetIssuesPayload(exact.Issues())
	}
	return result
}

func inspectedSourcesPayload(
	refs []neighborhood.InspectedSourceRef,
) []string {
	result := make([]string, 0, len(refs))
	for _, ref := range refs {
		result = append(result, ref.String())
	}
	return result
}

func facetIssuesPayload(
	issues []neighborhood.FacetBasisIssue,
) []map[string]any {
	result := make([]map[string]any, 0, len(issues))
	for _, issue := range issues {
		presented := map[string]any{
			"kind":  string(issue.Kind()),
			"facet": string(issue.Facet()),
		}
		switch exact := issue.(type) {
		case neighborhood.MissingTypeBasisIssue:
			presented["required_ref_or_kind"] = exact.Required().String()
		case neighborhood.MissingCorrespondenceBasisIssue:
			presented["required_correspondence"] =
				exact.Required().String()
		case neighborhood.UnresolvedLegacyIdentityIssue:
			presented["legacy_ref"] = exact.LegacyRef().String()
			presented["resolution_ref"] = exact.ResolutionRef().String()
		case neighborhood.StaleDerivedProjectionIssue:
			presented["projection_ref"] = exact.ProjectionRef().String()
			presented["observed_version"] =
				exact.ObservedVersion().String()
			presented["required_version"] =
				exact.RequiredVersion().String()
		case neighborhood.ExplicitBridgeRequiredIssue:
			presented["source_context_ref"] =
				exact.SourceContext().String()
			presented["target_context_ref"] =
				exact.TargetContext().String()
			presented["bridge"] = string(exact.Bridge().Kind())
			known, ok := exact.Bridge().(neighborhood.KnownBridge)
			if ok {
				presented["known_bridge_ref"] = known.Ref().String()
			}
		}
		result = append(result, presented)
	}
	return result
}

func retryCausePayload(
	cause neighborhood.WholeReadRetryCause,
) (map[string]any, error) {
	if cause == nil {
		return nil, fmt.Errorf("memory-read retry cause is required")
	}
	result := map[string]any{"kind": string(cause.Kind())}
	switch exact := cause.(type) {
	case neighborhood.StaleSnapshotCause:
		result["observed_snapshot"] = snapshotBasisPayload(exact.Observed())
		result["required_snapshot"] = snapshotBasisPayload(exact.Required())
	case neighborhood.StaleCursorCause:
		result["cursor"] = snapshotCursorPayload(exact.Cursor())
		result["required_snapshot"] = snapshotBasisPayload(exact.Required())
	case neighborhood.ProjectionRebuildRequiredCause:
		result["projection_ref"] = exact.ProjectionRef().String()
		result["observed_epoch"] = exact.ObservedEpoch()
		result["required_epoch"] = exact.RequiredEpoch()
	default:
		return nil, fmt.Errorf(
			"unsupported memory-read retry cause %T",
			cause,
		)
	}
	return result, nil
}

func snapshotBasisPayload(
	basis neighborhood.SnapshotBasis,
) map[string]any {
	return map[string]any{
		"graph_revision":  basis.GraphRevision().Value(),
		"type_env_ref":    basis.TypeEnv().String(),
		"type_env_digest": basis.TypeEnvDigest().String(),
	}
}

func snapshotCursorPayload(
	cursor neighborhood.SnapshotCursor,
) map[string]any {
	return map[string]any{
		"digest":         cursor.Digest().String(),
		"graph_revision": cursor.GraphRevision().Value(),
		"type_env_ref":   cursor.TypeEnv().String(),
		"profile_ref":    cursor.ProfileRef().String(),
		"facet":          string(cursor.Facet()),
		"next_offset":    cursor.NextOffset(),
	}
}

func interpretationPayload(
	contract neighborhood.InterpretationContract,
) map[string]any {
	return map[string]any{
		"structure":               string(contract.Structure()),
		"identity":                string(contract.Identity()),
		"relational_records":      string(contract.RelationalRecords()),
		"ranking":                 string(contract.Ranking()),
		"truth":                   string(contract.Truth()),
		"applicability":           string(contract.Applicability()),
		"authority":               string(contract.Authority()),
		"work_order":              string(contract.WorkOrder()),
		"completeness":            string(contract.Completeness()),
		"hydrate_before_reliance": contract.HydrateBeforeReliance(),
	}
}

func persistedRefPayload(
	ref typedmemory.PersistedRef,
) map[string]any {
	return map[string]any{
		"ref_kind_id":  ref.RefKind().ID().String(),
		"reference_id": ref.ReferenceID().String(),
	}
}

func persistedRefsPayload(
	refs []typedmemory.PersistedRef,
) []map[string]any {
	result := make([]map[string]any, 0, len(refs))
	for _, ref := range refs {
		result = append(result, persistedRefPayload(ref))
	}
	return result
}

func itemPosturesPayload(
	postures neighborhood.ItemPostures,
) map[string]any {
	return map[string]any{
		"semantic":             string(postures.Semantic()),
		"lifecycle":            string(postures.Lifecycle()),
		"evidence_currentness": string(postures.Evidence()),
		"projection_freshness": string(postures.Projection()),
	}
}

func relationWitnessesPayload(
	witnesses []neighborhood.RelationPathWitness,
) []map[string]any {
	result := make([]map[string]any, 0, len(witnesses))
	for _, witness := range witnesses {
		result = append(result, map[string]any{
			"assertion_id": witness.Assertion().String(),
			"relation_declaration_fragment_id": witness.
				RelationDeclarationFragmentID().
				String(),
			// signature_id is the sealed v1 compatibility alias for the
			// same coordinate, not a complete FPF RelationSignature.
			"signature_id":                 witness.Signature().String(),
			"relation_declaration_posture": string(witness.RelationDeclarationPosture()),
			"bounded_context_ref":          witness.Context().String(),
			"slot_kind_id":                 witness.Slot().String(),
			"target_ref":                   persistedRefPayload(witness.Target()),
			"provenance_ref":               witness.Provenance().String(),
			"admission_event_ref":          witness.AdmissionEventRef(),
			"relational_record_posture": string(
				witness.RelationalRecordPosture().Kind(),
			),
		})
	}
	return result
}
