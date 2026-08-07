package projectmemory

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/projectmemory/memoryresolve"
	"github.com/m0n0x41d/haft/internal/projectmemory/neighborhood"
	"github.com/m0n0x41d/haft/internal/projectmemory/scopedrecall"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorywire"
)

func TestMemoryReadResponseRejectsNilAndZeroDomainResults(t *testing.T) {
	t.Parallel()

	if _, err := EncodeResolutionReadResponse(nil); err == nil {
		t.Fatal("resolution response accepted a nil domain result")
	}
	if _, err := EncodeResolutionReadResponse(
		memoryresolve.ExactEntity{},
	); err == nil {
		t.Fatal("resolution response accepted a zero exact result")
	}
	if _, err := EncodeNeighborhoodReadResponse(nil); err == nil {
		t.Fatal("neighborhood response accepted a nil domain result")
	}
	if _, err := EncodeNeighborhoodReadResponse(
		neighborhood.RetryRequiredResult{},
	); err == nil {
		t.Fatal("neighborhood response accepted a zero retry result")
	}
	if _, err := EncodeScopedRecallReadResponse(nil); err == nil {
		t.Fatal("scoped recall response accepted a nil domain result")
	}
	if _, err := EncodeScopedRecallReadResponse(
		scopedrecall.ScopedRetryRequired{},
	); err == nil {
		t.Fatal("scoped recall response accepted a zero retry result")
	}
}

func TestPersistedEntityRefPayloadComposesThroughPublicNeighborhoodAndRecall(
	t *testing.T,
) {
	t.Parallel()

	digest, err := typedmemory.NewSHA256Digest(
		"sha256:" + strings.Repeat("a", 64),
	)
	if err != nil {
		t.Fatal(err)
	}
	typeEnv, err := typedmemory.NewTypeEnvRef(digest)
	if err != nil {
		t.Fatal(err)
	}
	refKindID, err := typedmemory.NewRefKindID("U.EntityRef")
	if err != nil {
		t.Fatal(err)
	}
	refKind, err := typedmemory.NewRefKindRef(typeEnv, refKindID)
	if err != nil {
		t.Fatal(err)
	}
	referenceID, err := typedmemory.NewReferenceID("service:auth")
	if err != nil {
		t.Fatal(err)
	}
	entity, err := typedmemory.NewPersistedRef(refKind, referenceID)
	if err != nil {
		t.Fatal(err)
	}

	context, err := typedmemory.NewBoundedContextRef("context:project")
	if err != nil {
		t.Fatal(err)
	}
	label, err := neighborhood.NewReadableItemText("Authorization service")
	if err != nil {
		t.Fatal(err)
	}
	provenance, err := typedmemory.NewProvenanceRef("record:auth")
	if err != nil {
		t.Fatal(err)
	}
	resolutionBasis, err := typedmemory.NewResolutionBasisRef(
		"resolution:exact-auth",
	)
	if err != nil {
		t.Fatal(err)
	}
	resolutionUnit, err := memoryresolve.NewResolutionUnit(
		entity,
		context,
		label,
		nil,
		provenance,
		resolutionBasis,
	)
	if err != nil {
		t.Fatal(err)
	}
	resolveEntityPayload := resolutionUnitPayload(resolutionUnit)
	entityPayload, ok := resolveEntityPayload["entity_ref"].(map[string]any)
	if !ok {
		t.Fatalf(
			"resolve response entity_ref = %#v",
			resolveEntityPayload["entity_ref"],
		)
	}
	if entityPayload["ref_kind_id"] != "U.EntityRef" ||
		entityPayload["reference_id"] != "service:auth" {
		t.Fatalf("entity_ref payload = %#v", entityPayload)
	}
	if _, leaked := entityPayload["ref_kind"]; leaked {
		t.Fatalf("entity_ref leaked non-input field ref_kind: %#v", entityPayload)
	}

	entityBytes, err := json.Marshal(entityPayload)
	if err != nil {
		t.Fatal(err)
	}
	view := map[string]any{
		"projection_profile_ref": "agent_orientation.v2",
		"requested_facets":       []string{"problems"},
		"detail":                 "standard",
		"include_history":        false,
	}
	readBudget := map[string]any{
		"max_facets":                     1,
		"max_items_per_facet":            1,
		"max_relation_paths_per_item":    1,
		"max_carrier_excerpt_characters": 256,
		"max_provenance_depth":           1,
	}
	neighborhoodPayload, err := json.Marshal(map[string]any{
		"action": typedmemorywire.QueryActionMemory,
		"memory_request": map[string]any{
			"contract_version": typedmemorywire.ContractVersion,
			"mode":             typedmemorywire.ActionNeighborhood,
			"basis": map[string]any{
				"kind": string(typedmemorywire.BasisProjectCurrent),
			},
			"entity_ref":          entityPayload,
			"bounded_context_ref": resolveEntityPayload["bounded_context_ref"],
			"view":                view,
			"read_budget":         readBudget,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	request, err := typedmemorywire.DecodeQueryReadRequest(
		neighborhoodPayload,
	)
	if err != nil {
		t.Fatalf("public neighborhood round-trip: %v", err)
	}
	neighborhoodRequest, ok :=
		request.(typedmemorywire.NeighborhoodReadRequest)
	if !ok ||
		neighborhoodRequest.Entity().RefKindID().String() != "U.EntityRef" ||
		neighborhoodRequest.Entity().ReferenceID().String() != "service:auth" {
		t.Fatalf("public neighborhood entity_ref = %#v", request)
	}

	recallPayload, err := json.Marshal(map[string]any{
		"action": typedmemorywire.QueryActionMemory,
		"memory_request": map[string]any{
			"contract_version": typedmemorywire.ContractVersion,
			"mode":             typedmemorywire.ActionRecall,
			"basis": map[string]any{
				"kind": string(typedmemorywire.BasisProjectCurrent),
			},
			"entity_ref":          entityPayload,
			"bounded_context_ref": "context:project",
			"view":                view,
			"read_budget":         readBudget,
			"query":               "authorization decisions",
			"candidate_budget": map[string]any{
				"max_candidates": 8,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	request, err = typedmemorywire.DecodeQueryReadRequest(recallPayload)
	if err != nil {
		t.Fatalf("public recall round-trip: %v", err)
	}
	recallRequest, ok := request.(typedmemorywire.RecallReadRequest)
	if !ok ||
		recallRequest.Entity().RefKindID().String() != "U.EntityRef" ||
		recallRequest.Entity().ReferenceID().String() != "service:auth" {
		t.Fatalf("public recall entity_ref = %#v", request)
	}
	reusedEntityBytes, err := json.Marshal(entityPayload)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(entityBytes, reusedEntityBytes) {
		t.Fatalf(
			"resolve entity_ref changed during public composition\n got: %s\nwant: %s",
			reusedEntityBytes,
			entityBytes,
		)
	}
}

func TestRelationWitnessPayloadUsesFragmentAsCurrentCoordinate(t *testing.T) {
	t.Parallel()

	assertion, err := typedmemory.NewAssertionID("assertion:test")
	if err != nil {
		t.Fatal(err)
	}
	fragment, err := typedmemory.NewSignatureID("Haft.RecordAtConcern")
	if err != nil {
		t.Fatal(err)
	}
	contextRef, err := typedmemory.NewBoundedContextRef("context:test")
	if err != nil {
		t.Fatal(err)
	}
	slot, err := typedmemory.NewSlotKindID("Haft.RecordSlot")
	if err != nil {
		t.Fatal(err)
	}
	digest, err := typedmemory.NewSHA256Digest("sha256:" + strings.Repeat("a", 64))
	if err != nil {
		t.Fatal(err)
	}
	typeEnv, err := typedmemory.NewTypeEnvRef(digest)
	if err != nil {
		t.Fatal(err)
	}
	refKindID, err := typedmemory.NewRefKindID("U.EntityRef")
	if err != nil {
		t.Fatal(err)
	}
	refKind, err := typedmemory.NewRefKindRef(typeEnv, refKindID)
	if err != nil {
		t.Fatal(err)
	}
	referenceID, err := typedmemory.NewReferenceID("record:test")
	if err != nil {
		t.Fatal(err)
	}
	target, err := typedmemory.NewPersistedRef(refKind, referenceID)
	if err != nil {
		t.Fatal(err)
	}
	provenance, err := typedmemory.NewProvenanceRef("event:test")
	if err != nil {
		t.Fatal(err)
	}
	witness, err := neighborhood.NewRelationPathWitness(
		assertion,
		fragment,
		contextRef,
		slot,
		target,
		provenance,
		"admission:test",
	)
	if err != nil {
		t.Fatal(err)
	}

	payload := relationWitnessesPayload([]neighborhood.RelationPathWitness{witness})
	if len(payload) != 1 ||
		payload[0]["relation_declaration_fragment_id"] != fragment.String() ||
		payload[0]["signature_id"] != fragment.String() ||
		payload[0]["relation_declaration_posture"] !=
			string(typedmemory.RelationDeclarationTypedFragment) {
		t.Fatalf("relation witness payload = %#v", payload)
	}
}

func TestNeighborhoodRetryResponseKeepsTypedRetryWithoutAuthority(
	t *testing.T,
) {
	t.Parallel()

	observed := responseTestSnapshot(t, 7, "a")
	required := responseTestSnapshot(t, 8, "b")
	cause, err := neighborhood.NewStaleSnapshotCause(observed, required)
	if err != nil {
		t.Fatal(err)
	}
	result, err := neighborhood.NewRetryRequiredResult(cause, required)
	if err != nil {
		t.Fatal(err)
	}

	encoded, err := EncodeNeighborhoodReadResponse(result)
	if err != nil {
		t.Fatal(err)
	}
	envelope := decodeResponseEnvelope(t, encoded)
	assertResponseHeader(
		t,
		envelope,
		typedmemorywire.ActionNeighborhood,
		string(neighborhood.ResultRetryRequired),
	)
	payload := responseObject(t, envelope, "result")
	retryCause := responseObject(t, payload, "cause")
	interpretation := responseObject(
		t,
		payload,
		"interpretation_contract",
	)
	if retryCause["kind"] != string(neighborhood.RetryStaleSnapshot) {
		t.Fatalf("retry cause = %#v", retryCause)
	}
	if interpretation["authority"] !=
		string(neighborhood.AuthorityNotGranted) {
		t.Fatalf("retry interpretation = %#v", interpretation)
	}
	if interpretation["work_order"] !=
		string(neighborhood.WorkOrderNotImplied) {
		t.Fatalf("retry work-order posture = %#v", interpretation)
	}
	if interpretation["relational_records"] !=
		string(neighborhood.RelationalRecordsUnavailable) {
		t.Fatalf("retry relational-record posture = %#v", interpretation)
	}
	if _, ambiguous := interpretation["relations"]; ambiguous {
		t.Fatalf("ambiguous relations field survived: %#v", interpretation)
	}
}

func TestNeighborhoodAbstentionResponseKeepsInspectedBasis(
	t *testing.T,
) {
	t.Parallel()

	required, err := neighborhood.NewRequiredBasisRef(
		"typeenv:missing-kind:Haft.CodeAnchor",
	)
	if err != nil {
		t.Fatal(err)
	}
	issue, err := neighborhood.NewMissingTypeBasisIssue(
		neighborhood.FacetImplementation,
		required,
	)
	if err != nil {
		t.Fatal(err)
	}
	basis, err := neighborhood.NewNoAdmissibleFacetBasis(
		[]neighborhood.FacetBasisIssue{issue},
	)
	if err != nil {
		t.Fatal(err)
	}
	source, err := neighborhood.NewInspectedSourceRef(
		"canonical:typed-memory@revision:8",
	)
	if err != nil {
		t.Fatal(err)
	}
	result, err := neighborhood.NewAbstainedResult(
		basis,
		[]neighborhood.InspectedSourceRef{source},
	)
	if err != nil {
		t.Fatal(err)
	}

	encoded, err := EncodeNeighborhoodReadResponse(result)
	if err != nil {
		t.Fatal(err)
	}
	envelope := decodeResponseEnvelope(t, encoded)
	assertResponseHeader(
		t,
		envelope,
		typedmemorywire.ActionNeighborhood,
		string(neighborhood.ResultAbstained),
	)
	payload := responseObject(t, envelope, "result")
	abstentionBasis := responseObject(t, payload, "basis")
	sources, ok := payload["inspected_sources"].([]any)
	if !ok || len(sources) != 1 ||
		sources[0] != source.String() {
		t.Fatalf("inspected sources = %#v", payload["inspected_sources"])
	}
	if abstentionBasis["kind"] !=
		string(neighborhood.AbstainNoAdmissibleFacet) {
		t.Fatalf("abstention basis = %#v", abstentionBasis)
	}
}

func TestNeighborhoodAbstentionBridgeIssueUsesCanonicalWireShape(
	t *testing.T,
) {
	t.Parallel()

	source, err := typedmemory.NewBoundedContextRef("context:source")
	if err != nil {
		t.Fatal(err)
	}
	target, err := typedmemory.NewBoundedContextRef("context:target")
	if err != nil {
		t.Fatal(err)
	}
	bridgeRef, err := neighborhood.NewContextBridgeRef(
		"bridge:source-target",
	)
	if err != nil {
		t.Fatal(err)
	}
	bridge, err := neighborhood.NewKnownBridge(bridgeRef)
	if err != nil {
		t.Fatal(err)
	}
	issue, err := neighborhood.NewExplicitBridgeRequiredIssue(
		neighborhood.FacetSpecifications,
		source,
		target,
		bridge,
	)
	if err != nil {
		t.Fatal(err)
	}
	basis, err := neighborhood.NewNoAdmissibleFacetBasis(
		[]neighborhood.FacetBasisIssue{issue},
	)
	if err != nil {
		t.Fatal(err)
	}
	inspected, err := neighborhood.NewInspectedSourceRef(
		"canonical:typed-memory@revision:8",
	)
	if err != nil {
		t.Fatal(err)
	}
	result, err := neighborhood.NewAbstainedResult(
		basis,
		[]neighborhood.InspectedSourceRef{inspected},
	)
	if err != nil {
		t.Fatal(err)
	}

	encoded, err := EncodeNeighborhoodReadResponse(result)
	if err != nil {
		t.Fatal(err)
	}
	envelope := decodeResponseEnvelope(t, encoded)
	payload := responseObject(t, envelope, "result")
	abstention := responseObject(t, payload, "basis")
	issues, ok := abstention["issues"].([]any)
	if !ok || len(issues) != 1 {
		t.Fatalf("facet issues = %#v", abstention["issues"])
	}
	presented, ok := issues[0].(map[string]any)
	if !ok {
		t.Fatalf("facet issue = %#v", issues[0])
	}
	if presented["bridge"] != string(neighborhood.BridgeKnown) ||
		presented["known_bridge_ref"] != bridgeRef.String() {
		t.Fatalf("explicit bridge issue = %#v", presented)
	}
	if _, nested := presented["bridge"].(map[string]any); nested {
		t.Fatalf("explicit bridge issue retained competing wire shape: %#v", presented)
	}
}

func TestNeighborhoodRetryResponseEncodesEveryClosedCauseVariant(
	t *testing.T,
) {
	t.Parallel()

	observed := responseTestSnapshot(t, 17, "c")
	required := responseTestSnapshot(t, 18, "d")
	profileRef, err := neighborhood.ParseProjectionProfileRef(
		"agent_orientation.v1",
	)
	if err != nil {
		t.Fatal(err)
	}
	profile, found := neighborhood.LookupProjectionProfile(profileRef)
	if !found {
		t.Fatal("agent-orientation projection profile is unavailable")
	}
	cursor, err := neighborhood.NewSnapshotCursor(
		observed.GraphRevision(),
		observed.TypeEnv(),
		profile,
		neighborhood.FacetEvidence,
		4,
	)
	if err != nil {
		t.Fatal(err)
	}
	projection, err := neighborhood.NewProjectionRef(
		"projection:spec-index:v2",
	)
	if err != nil {
		t.Fatal(err)
	}
	staleSnapshot, err := neighborhood.NewStaleSnapshotCause(
		observed,
		required,
	)
	if err != nil {
		t.Fatal(err)
	}
	staleCursor, err := neighborhood.NewStaleCursorCause(cursor, required)
	if err != nil {
		t.Fatal(err)
	}
	projectionRebuild, err :=
		neighborhood.NewProjectionRebuildRequiredCause(
			projection,
			3,
			4,
		)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		cause     neighborhood.WholeReadRetryCause
		kind      neighborhood.WholeReadRetryCauseKind
		operation neighborhood.RetryOperation
		field     string
	}{
		{
			cause:     staleSnapshot,
			kind:      neighborhood.RetryStaleSnapshot,
			operation: neighborhood.RetryReloadSnapshot,
			field:     "observed_snapshot",
		},
		{
			cause:     staleCursor,
			kind:      neighborhood.RetryStaleCursor,
			operation: neighborhood.RetryRestartFromCursor,
			field:     "cursor",
		},
		{
			cause:     projectionRebuild,
			kind:      neighborhood.RetryProjectionRebuildRequired,
			operation: neighborhood.RetryRebuildProjection,
			field:     "projection_ref",
		},
	}
	for _, testCase := range cases {
		result, resultErr := neighborhood.NewRetryRequiredResult(
			testCase.cause,
			required,
		)
		if resultErr != nil {
			t.Fatal(resultErr)
		}
		encoded, encodeErr := EncodeNeighborhoodReadResponse(result)
		if encodeErr != nil {
			t.Fatal(encodeErr)
		}
		envelope := decodeResponseEnvelope(t, encoded)
		payload := responseObject(t, envelope, "result")
		cause := responseObject(t, payload, "cause")
		if cause["kind"] != string(testCase.kind) ||
			payload["retry_operation"] != string(testCase.operation) {
			t.Fatalf(
				"retry payload = %#v, want %q/%q",
				payload,
				testCase.kind,
				testCase.operation,
			)
		}
		if _, found := cause[testCase.field]; !found {
			t.Fatalf(
				"retry cause %q omitted field %q: %#v",
				testCase.kind,
				testCase.field,
				cause,
			)
		}
	}
}

func TestNeighborhoodAbstentionResponseEncodesEveryFacetIssueVariant(
	t *testing.T,
) {
	t.Parallel()

	requiredType, err := neighborhood.NewRequiredBasisRef(
		"typeenv:missing-kind:Haft.CodeAnchor",
	)
	if err != nil {
		t.Fatal(err)
	}
	correspondence, err :=
		neighborhood.NewProjectionCorrespondenceManifestRef(
			"correspondence:code-anchor:v1",
		)
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := neighborhood.NewLegacyRecordRef("legacy:decision:42")
	if err != nil {
		t.Fatal(err)
	}
	resolution, err := neighborhood.NewIdentityResolutionRef(
		"identity-resolution:decision:42",
	)
	if err != nil {
		t.Fatal(err)
	}
	projection, err := neighborhood.NewProjectionRef(
		"projection:spec-index",
	)
	if err != nil {
		t.Fatal(err)
	}
	observedVersion, err := neighborhood.NewProjectionVersion(
		"projection-version:1",
	)
	if err != nil {
		t.Fatal(err)
	}
	requiredVersion, err := neighborhood.NewProjectionVersion(
		"projection-version:2",
	)
	if err != nil {
		t.Fatal(err)
	}
	sourceContext, err := typedmemory.NewBoundedContextRef("context:source")
	if err != nil {
		t.Fatal(err)
	}
	targetContext, err := typedmemory.NewBoundedContextRef("context:target")
	if err != nil {
		t.Fatal(err)
	}
	missingType, err := neighborhood.NewMissingTypeBasisIssue(
		neighborhood.FacetImplementation,
		requiredType,
	)
	if err != nil {
		t.Fatal(err)
	}
	missingCorrespondence, err :=
		neighborhood.NewMissingCorrespondenceBasisIssue(
			neighborhood.FacetImplementation,
			correspondence,
		)
	if err != nil {
		t.Fatal(err)
	}
	unresolvedIdentity, err :=
		neighborhood.NewUnresolvedLegacyIdentityIssue(
			neighborhood.FacetDecisions,
			legacy,
			resolution,
		)
	if err != nil {
		t.Fatal(err)
	}
	staleProjection, err := neighborhood.NewStaleDerivedProjectionIssue(
		neighborhood.FacetSpecifications,
		projection,
		observedVersion,
		requiredVersion,
	)
	if err != nil {
		t.Fatal(err)
	}
	explicitBridge, err := neighborhood.NewExplicitBridgeRequiredIssue(
		neighborhood.FacetSpecifications,
		sourceContext,
		targetContext,
		neighborhood.UnknownBridge{},
	)
	if err != nil {
		t.Fatal(err)
	}
	basis, err := neighborhood.NewNoAdmissibleFacetBasis(
		[]neighborhood.FacetBasisIssue{
			missingType,
			missingCorrespondence,
			unresolvedIdentity,
			staleProjection,
			explicitBridge,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	inspected, err := neighborhood.NewInspectedSourceRef(
		"canonical:typed-memory@revision:18",
	)
	if err != nil {
		t.Fatal(err)
	}
	result, err := neighborhood.NewAbstainedResult(
		basis,
		[]neighborhood.InspectedSourceRef{inspected},
	)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := EncodeNeighborhoodReadResponse(result)
	if err != nil {
		t.Fatal(err)
	}
	envelope := decodeResponseEnvelope(t, encoded)
	payload := responseObject(t, envelope, "result")
	abstention := responseObject(t, payload, "basis")
	issues, ok := abstention["issues"].([]any)
	if !ok || len(issues) != 5 {
		t.Fatalf("facet issues = %#v, want five variants", abstention["issues"])
	}
	wantKinds := []neighborhood.FacetBasisIssueKind{
		neighborhood.IssueMissingTypeBasis,
		neighborhood.IssueMissingCorrespondenceBasis,
		neighborhood.IssueUnresolvedLegacyIdentity,
		neighborhood.IssueStaleDerivedProjection,
		neighborhood.IssueExplicitBridgeRequired,
	}
	for index, raw := range issues {
		presented, found := raw.(map[string]any)
		if !found || presented["kind"] != string(wantKinds[index]) {
			t.Fatalf(
				"facet issue %d = %#v, want kind %q",
				index,
				raw,
				wantKinds[index],
			)
		}
	}
}

func responseTestSnapshot(
	t *testing.T,
	revision uint64,
	fill string,
) neighborhood.SnapshotBasis {
	t.Helper()
	rawDigest := "sha256:" + strings.Repeat(fill, 64)
	digest, err := typedmemory.NewSHA256Digest(rawDigest)
	if err != nil {
		t.Fatal(err)
	}
	typeEnv, err := typedmemory.NewTypeEnvRef(digest)
	if err != nil {
		t.Fatal(err)
	}
	basis, err := neighborhood.NewSnapshotBasis(
		typedmemory.NewGraphRevision(revision),
		typeEnv,
		typeEnv.Digest(),
	)
	if err != nil {
		t.Fatal(err)
	}
	return basis
}

func decodeResponseEnvelope(
	t *testing.T,
	encoded []byte,
) map[string]any {
	t.Helper()
	var envelope map[string]any
	if err := json.Unmarshal(encoded, &envelope); err != nil {
		t.Fatal(err)
	}
	return envelope
}

func assertResponseHeader(
	t *testing.T,
	envelope map[string]any,
	action string,
	resultKind string,
) {
	t.Helper()
	if envelope["contract_version"] != typedmemorywire.ContractVersion ||
		envelope["action"] != action ||
		envelope["result_kind"] != resultKind {
		t.Fatalf("memory-read envelope = %#v", envelope)
	}
}

func responseObject(
	t *testing.T,
	value map[string]any,
	key string,
) map[string]any {
	t.Helper()
	object, ok := value[key].(map[string]any)
	if !ok {
		t.Fatalf("%s = %#v, want object", key, value[key])
	}
	return object
}
