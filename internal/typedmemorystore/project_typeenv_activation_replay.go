package typedmemorystore

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"

	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/projecttypeenvactivation"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

type durableProjectTypeEnvActivationRow struct {
	activationRef          string
	activationDigest       string
	activationBytes        []byte
	requestRef             string
	requestDigest          string
	contentDigest          string
	authorityUseRef        string
	workRef                string
	basisTypeEnvRef        string
	resultTypeEnvRef       string
	stageRef               string
	stageDigest            string
	headRef                string
	expectedGraphRevision  int64
	committedGraphRevision int64
	committedHeadRevision  int64
	recordedAt             string
}

type durableProjectTypeEnvActivationFootprint struct {
	normal                 genericMaterializationFootprint
	activationCount        int64
	topLevelChangeCount    int64
	closureActivationCount int64
}

func isStoredProjectTypeEnvActivation(common durableGenericCommonRow) bool {
	return common.eventKind == projecttypeenvactivation.EventKind ||
		isProjectTypeEnvActivationAuthorityClass(common.eventAuthorityClass)
}

func isProjectTypeEnvActivationAuthorityClass(value string) bool {
	switch value {
	case projecttypeenvactivation.LegacyManualAuthorityClass,
		projecttypeenvactivation.HostRoutedOperatorRequestAuthorityClass,
		projecttypeenvactivation.CompatibleSuccessorPolicyAuthorityClass:
		return true
	default:
		return false
	}
}

func verifyExactProjectTypeEnvActivationCoordinate(
	ctx context.Context,
	source scanner,
	project projectledger.ProjectID,
	coordinate storedAdmissionCoordinate,
	common durableGenericCommonRow,
	admission durableV46AdmissionRow,
	closure durableV46ClosureRow,
) (*verifiedMaterializationClosure, error) {
	delta, err := verifyStoredProjectTypeEnvActivationEvent(
		project,
		coordinate,
		common,
	)
	if err != nil {
		return nil, err
	}
	envelope, basis, manifest, err := verifyStoredProjectTypeEnvActivationCarriers(
		common,
		admission,
		delta,
	)
	if err != nil {
		return nil, err
	}
	if err := verifyStoredGenericAdmissionLinks(common, admission, closure); err != nil {
		return nil, err
	}
	if _, err := verifyStoredGenericCanonicalCarrier(
		"stored activation materialization-closure carrier",
		closure.materializationBytes,
		closure.materializationDigest,
	); err != nil {
		return nil, err
	}
	activation, err := loadDurableProjectTypeEnvActivationRow(
		ctx,
		source,
		project,
		common.idempotencyEventRef,
	)
	if err != nil {
		return nil, err
	}
	if err := verifyStoredProjectTypeEnvActivationRow(
		common,
		activation,
		delta,
	); err != nil {
		return nil, err
	}
	digest, err := verifyStoredProjectTypeEnvActivationMaterialization(
		ctx,
		source,
		project,
		common,
		admission,
		closure,
		delta,
		envelope,
		basis,
		manifest,
	)
	if err != nil {
		return nil, err
	}
	return &verifiedMaterializationClosure{
		eventRef: coordinate.EventRef,
		commit:   closure.commitRef,
		digest:   digest,
	}, nil
}

func verifyStoredProjectTypeEnvActivationEvent(
	project projectledger.ProjectID,
	coordinate storedAdmissionCoordinate,
	common durableGenericCommonRow,
) (projecttypeenvactivation.Delta, error) {
	if _, err := parseCanonicalGenericRecordedAt(common.eventRecordedAt); err != nil {
		return projecttypeenvactivation.Delta{},
			storedAdmissionIntegrity("stored activation event recorded_at", err)
	}
	expectedRevision, err := graphRevisionFromSQLite(common.eventExpectedRevision)
	if err != nil {
		return projecttypeenvactivation.Delta{},
			storedAdmissionIntegrity("stored activation expected revision", err)
	}
	graphRevision, err := graphRevisionFromSQLite(common.eventRevision)
	if err != nil {
		return projecttypeenvactivation.Delta{},
			storedAdmissionIntegrity("stored activation graph revision", err)
	}
	delta, err := projecttypeenvactivation.DecodeDelta(common.eventCanonicalBytes)
	if err != nil {
		return projecttypeenvactivation.Delta{},
			storedAdmissionIntegrity("stored activation delta", err)
	}
	basisTypeEnv, err := parseTypeEnvRef(common.eventBasisTypeEnv)
	if err != nil {
		return projecttypeenvactivation.Delta{},
			storedAdmissionIntegrity("stored activation basis TypeEnv", err)
	}
	resultTypeEnv, err := parseTypeEnvRef(common.eventResultTypeEnv)
	if err != nil {
		return projecttypeenvactivation.Delta{},
			storedAdmissionIntegrity("stored activation result TypeEnv", err)
	}
	recomputed, err := digestFields(
		"typed-memory-graph-event.v1",
		project.String(),
		common.eventCommitRef,
		strconv.FormatUint(expectedRevision.Value(), 10),
		strconv.FormatUint(graphRevision.Value(), 10),
		basisTypeEnv.String(),
		delta.Digest().String(),
		string(delta.CanonicalBytes()),
		delta.EventKind(),
		delta.AuthorityClass(),
		delta.RequestRef().String(),
	)
	if err != nil {
		return projecttypeenvactivation.Delta{},
			storedAdmissionIntegrity("stored activation event digest", err)
	}
	expectedCommit := derivedRef(
		"typed-memory-commit",
		project.String(),
		coordinate.IdempotencyKey,
		delta.Digest().String(),
		strconv.FormatUint(graphRevision.Value(), 10),
	)
	expectedEvent := derivedRef("typed-memory-event", recomputed.String())
	expectedProjection := derivedRef("typed-memory-projection-job", expectedCommit)
	matches := graphRevision.Value() == expectedRevision.Value()+1 &&
		delta.Project() == project &&
		delta.ExpectedGraphRevision() == expectedRevision &&
		delta.CommittedGraphRevision() == graphRevision &&
		delta.Target().Composite() == resultTypeEnv &&
		delta.RequestRef().String() == common.eventProvenance &&
		delta.Digest().String() == common.eventChangeDigest &&
		delta.Digest().String() == common.idempotencyChangeDigest &&
		delta.Digest().String() == common.commitChangeDigest &&
		common.eventKind == delta.EventKind() &&
		common.eventAuthorityClass == delta.AuthorityClass() &&
		common.eventChangeCount == 1 &&
		common.eventDigest == recomputed.String() &&
		common.idempotencyResultDigest == recomputed.String() &&
		common.commitEventDigest == recomputed.String() &&
		common.eventCommitRef == expectedCommit &&
		common.commitRef == expectedCommit &&
		common.idempotencyEventRef == expectedEvent &&
		common.commitEventRef == expectedEvent &&
		common.idempotencyRevision == common.eventRevision &&
		common.commitExpectedRevision == common.eventExpectedRevision &&
		common.commitRevision == common.eventRevision &&
		common.commitIdempotencyKey == coordinate.IdempotencyKey &&
		common.commitProjectionJobRef == expectedProjection &&
		common.commitEntityCount == 0 &&
		common.commitEntityContextCount == 0
	if !matches {
		return projecttypeenvactivation.Delta{},
			storedAdmissionIntegrity("stored activation event identity", nil)
	}
	return delta, nil
}

func verifyStoredProjectTypeEnvActivationCarriers(
	common durableGenericCommonRow,
	admission durableV46AdmissionRow,
	delta projecttypeenvactivation.Delta,
) (
	projecttypeenvactivation.AdmissionEnvelope,
	projecttypeenvactivation.AdmissionBasis,
	projecttypeenvactivation.MaterializationManifest,
	error,
) {
	requestRef, err := projecttypeenvselection.ParseProjectTypeEnvHeadSelectionRequestRef(
		common.eventProvenance,
	)
	if err != nil {
		return projecttypeenvactivation.AdmissionEnvelope{},
			projecttypeenvactivation.AdmissionBasis{},
			projecttypeenvactivation.MaterializationManifest{},
			storedAdmissionIntegrity("stored activation request ref", err)
	}
	request, err := projecttypeenvselection.VerifyProjectTypeEnvHeadSelectionRequest(
		requestRef,
		admission.requestBytes,
	)
	if err != nil {
		return projecttypeenvactivation.AdmissionEnvelope{},
			projecttypeenvactivation.AdmissionBasis{},
			projecttypeenvactivation.MaterializationManifest{},
			storedAdmissionIntegrity("stored activation request", err)
	}
	storedBasis, err := parseTypeEnvRef(common.eventBasisTypeEnv)
	if err != nil {
		return projecttypeenvactivation.AdmissionEnvelope{},
			projecttypeenvactivation.AdmissionBasis{},
			projecttypeenvactivation.MaterializationManifest{},
			storedAdmissionIntegrity("stored activation request basis", err)
	}
	if err := verifyActivationRequestDelta(request, storedBasis, delta); err != nil {
		return projecttypeenvactivation.AdmissionEnvelope{},
			projecttypeenvactivation.AdmissionBasis{},
			projecttypeenvactivation.MaterializationManifest{},
			storedAdmissionIntegrity("stored activation request/delta closure", err)
	}
	envelope, err := projecttypeenvactivation.DecodeAdmissionEnvelope(
		admission.envelopeBytes,
	)
	if err != nil {
		return projecttypeenvactivation.AdmissionEnvelope{},
			projecttypeenvactivation.AdmissionBasis{},
			projecttypeenvactivation.MaterializationManifest{},
			storedAdmissionIntegrity("stored activation envelope", err)
	}
	basis, err := projecttypeenvactivation.DecodeAdmissionBasis(admission.basisBytes)
	if err != nil {
		return projecttypeenvactivation.AdmissionEnvelope{},
			projecttypeenvactivation.AdmissionBasis{},
			projecttypeenvactivation.MaterializationManifest{},
			storedAdmissionIntegrity("stored activation basis", err)
	}
	manifest, err := projecttypeenvactivation.DecodeMaterializationManifest(
		admission.manifestBytes,
	)
	if err != nil {
		return projecttypeenvactivation.AdmissionEnvelope{},
			projecttypeenvactivation.AdmissionBasis{},
			projecttypeenvactivation.MaterializationManifest{},
			storedAdmissionIntegrity("stored activation manifest", err)
	}
	if err := projecttypeenvactivation.VerifyClosure(
		delta,
		envelope,
		basis,
		manifest,
	); err != nil {
		return projecttypeenvactivation.AdmissionEnvelope{},
			projecttypeenvactivation.AdmissionBasis{},
			projecttypeenvactivation.MaterializationManifest{},
			storedAdmissionIntegrity("stored activation carrier closure", err)
	}
	requestTarget := request.Target()
	deltaTarget := delta.Target()
	matches := request.Ref() == delta.RequestRef() &&
		request.Ref().Digest() == delta.RequestDigest() &&
		request.Project() == delta.Project() &&
		request.ExpectedGraphRevision() == delta.ExpectedGraphRevision() &&
		requestTarget.Base() == deltaTarget.Base() &&
		requestTarget.RuntimeBasis() == deltaTarget.RuntimeBasis() &&
		requestTarget.VerifiedComposite() == deltaTarget.Composite() &&
		requestTarget.Stage() == deltaTarget.Stage() &&
		orderedActivationExtensionsEqual(
			requestTarget.OrderedExtensions(),
			deltaTarget.OrderedExtensions(),
		) &&
		activationPredecessorsEqual(request.Predecessor(), delta.Predecessor()) &&
		admission.eventDigest == common.eventDigest &&
		admission.basisKind == projecttypeenvactivation.AdmissionKindSnapshotOnly &&
		admission.typeEnvRef == common.eventBasisTypeEnv &&
		admission.basisRevision == common.eventExpectedRevision &&
		admission.requestDigest == request.Ref().Digest().String() &&
		admission.semanticDigest == delta.Digest().String() &&
		bytes.Equal(admission.semanticBytes, delta.CanonicalBytes()) &&
		admission.envelopeDigest == envelope.Digest().String() &&
		admission.basisDigest == basis.Digest().String() &&
		admission.manifestDigest == manifest.Digest().String() &&
		manifest.EventRef().String() == common.idempotencyEventRef &&
		manifest.CommitRef().String() == common.eventCommitRef
	if !matches {
		return projecttypeenvactivation.AdmissionEnvelope{},
			projecttypeenvactivation.AdmissionBasis{},
			projecttypeenvactivation.MaterializationManifest{},
			storedAdmissionIntegrity("stored activation admission coordinates", nil)
	}
	return envelope, basis, manifest, nil
}

func loadDurableProjectTypeEnvActivationRow(
	ctx context.Context,
	source scanner,
	project projectledger.ProjectID,
	eventRef string,
) (durableProjectTypeEnvActivationRow, error) {
	row := durableProjectTypeEnvActivationRow{}
	err := source.ScanOne(
		ctx,
		`SELECT activation_ref, activation_digest, canonical_activation_bytes,
			request_ref, request_digest, content_digest, authority_use_ref,
			work_ref, basis_type_env_ref, result_type_env_ref, stage_ref,
			stage_digest, head_ref, expected_graph_revision,
			committed_graph_revision, committed_head_revision, recorded_at
		FROM typed_memory_type_env_activations
		WHERE project_id = ? AND event_ref = ? AND change_ordinal = 0`,
		[]any{project.String(), eventRef},
		[]any{
			&row.activationRef,
			&row.activationDigest,
			&row.activationBytes,
			&row.requestRef,
			&row.requestDigest,
			&row.contentDigest,
			&row.authorityUseRef,
			&row.workRef,
			&row.basisTypeEnvRef,
			&row.resultTypeEnvRef,
			&row.stageRef,
			&row.stageDigest,
			&row.headRef,
			&row.expectedGraphRevision,
			&row.committedGraphRevision,
			&row.committedHeadRevision,
			&row.recordedAt,
		},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return durableProjectTypeEnvActivationRow{},
			storedAdmissionIntegrity("stored activation row is missing", nil)
	}
	if err != nil {
		return durableProjectTypeEnvActivationRow{},
			fmt.Errorf("load stored ProjectTypeEnv activation row: %w", err)
	}
	return row, nil
}

func verifyStoredProjectTypeEnvActivationRow(
	common durableGenericCommonRow,
	row durableProjectTypeEnvActivationRow,
	delta projecttypeenvactivation.Delta,
) error {
	sqliteExpectedRevision, err := exactSQLiteCoordinate(
		delta.ExpectedGraphRevision().Value(),
		"stored activation expected graph revision",
	)
	if err != nil {
		return err
	}
	sqliteCommittedRevision, err := exactSQLiteCoordinate(
		delta.CommittedGraphRevision().Value(),
		"stored activation committed graph revision",
	)
	if err != nil {
		return err
	}
	sqliteHeadRevision, err := exactSQLiteCoordinate(
		delta.SuccessorHeadRevision().Value(),
		"stored activation committed head revision",
	)
	if err != nil {
		return err
	}
	if _, err := parseCanonicalGenericRecordedAt(row.recordedAt); err != nil {
		return storedAdmissionIntegrity("stored activation recorded_at", err)
	}
	matches := row.activationRef == delta.Ref().String() &&
		row.activationDigest == delta.Digest().String() &&
		bytes.Equal(row.activationBytes, delta.CanonicalBytes()) &&
		row.requestRef == delta.RequestRef().String() &&
		row.requestDigest == delta.RequestDigest().String() &&
		row.contentDigest == delta.ContentDigest().String() &&
		row.authorityUseRef == delta.AuthorityUseRef() &&
		row.workRef == delta.WorkRef().String() &&
		row.basisTypeEnvRef == common.eventBasisTypeEnv &&
		row.resultTypeEnvRef == delta.Target().Composite().String() &&
		row.stageRef == delta.Target().Stage().String() &&
		row.stageDigest == delta.Target().Stage().Digest().String() &&
		row.headRef == delta.Head().String() &&
		row.expectedGraphRevision == sqliteExpectedRevision &&
		row.committedGraphRevision == sqliteCommittedRevision &&
		row.committedHeadRevision == sqliteHeadRevision &&
		row.recordedAt == common.eventRecordedAt
	if !matches {
		return storedAdmissionIntegrity("stored activation row coordinates", nil)
	}
	return nil
}

func verifyStoredProjectTypeEnvActivationMaterialization(
	ctx context.Context,
	source scanner,
	project projectledger.ProjectID,
	common durableGenericCommonRow,
	admission durableV46AdmissionRow,
	closure durableV46ClosureRow,
	delta projecttypeenvactivation.Delta,
	envelope projecttypeenvactivation.AdmissionEnvelope,
	basis projecttypeenvactivation.AdmissionBasis,
	manifest projecttypeenvactivation.MaterializationManifest,
) (typedmemory.SHA256Digest, error) {
	footprint, err := loadProjectTypeEnvActivationFootprint(
		ctx,
		source,
		project,
		common.idempotencyEventRef,
	)
	if err != nil {
		return typedmemory.SHA256Digest{}, err
	}
	zero := genericMaterializationFootprint{}
	if footprint.normal != zero ||
		footprint.activationCount != 1 ||
		footprint.closureActivationCount != 1 ||
		footprint.topLevelChangeCount != 1 ||
		common.eventChangeCount != 1 {
		return typedmemory.SHA256Digest{},
			storedAdmissionIntegrity("stored activation materialization footprint", nil)
	}
	eventDigest, err := typedmemory.NewSHA256Digest(common.eventDigest)
	if err != nil {
		return typedmemory.SHA256Digest{},
			storedAdmissionIntegrity("stored activation materialization event digest", err)
	}
	requestDigest, err := typedmemory.NewSHA256Digest(admission.requestDigest)
	if err != nil {
		return typedmemory.SHA256Digest{},
			storedAdmissionIntegrity("stored activation request digest", err)
	}
	input := materializationClosureInput{
		identity: genericEventIdentity{
			nextRevision: delta.CommittedGraphRevision(),
			commitRef:    common.eventCommitRef,
			eventRef:     common.idempotencyEventRef,
			eventDigest:  eventDigest,
		},
		basisKind:      typedmemory.SnapshotOnlyAdmissionBasis,
		requestDigest:  requestDigest,
		semanticDigest: delta.Digest(),
		envelopeDigest: envelope.Digest(),
		basisDigest:    basis.Digest(),
		manifestDigest: manifest.Digest(),
		footprint:      zero,
		rowDigests: []string{
			"type-env-activation:" + delta.Digest().String(),
		},
	}
	expectedBytes := canonicalMaterializationClosureFromInput(input)
	expectedDigest, err := digestBytes(expectedBytes)
	if err != nil {
		return typedmemory.SHA256Digest{}, err
	}
	closureNormal := materializationFootprintFromClosure(closure)
	matches := closureNormal == zero &&
		closure.entityCount == common.commitEntityCount &&
		closure.entityContextCount == common.commitEntityContextCount &&
		bytes.Equal(closure.materializationBytes, expectedBytes) &&
		closure.materializationDigest == expectedDigest.String()
	if !matches {
		return typedmemory.SHA256Digest{},
			storedAdmissionIntegrity("stored activation exact materialization closure", nil)
	}
	return expectedDigest, nil
}

func loadProjectTypeEnvActivationFootprint(
	ctx context.Context,
	source scanner,
	project projectledger.ProjectID,
	eventRef string,
) (durableProjectTypeEnvActivationFootprint, error) {
	row := durableProjectTypeEnvActivationFootprint{}
	err := source.ScanOne(
		ctx,
		`SELECT footprint.entity_count, footprint.entity_context_count,
			footprint.entity_declaration_count,
			footprint.context_slice_catalog_count, footprint.context_slice_count,
			footprint.value_blob_count, footprint.observable_input_blob_count,
			footprint.relation_count, footprint.relation_slot_count,
			footprint.relation_filler_count,
			footprint.ordered_candidate_prefix_count,
			footprint.reference_resolution_use_count,
			footprint.memberof_evaluation_count, footprint.memberof_input_count,
			footprint.memberof_use_count, footprint.alias_change_count,
			footprint.retraction_count, footprint.type_env_activation_count,
			footprint.top_level_change_count, closure.type_env_activation_count
		FROM typed_memory_event_materialization_footprints_v46 footprint
		JOIN typed_memory_commit_materialization_closures closure
			ON closure.project_id = footprint.project_id
			AND closure.event_ref = footprint.event_ref
		WHERE footprint.project_id = ? AND footprint.event_ref = ?`,
		[]any{project.String(), eventRef},
		[]any{
			&row.normal.entityCount,
			&row.normal.entityContextCount,
			&row.normal.entityDeclarationCount,
			&row.normal.contextSliceCatalogCount,
			&row.normal.contextSliceCount,
			&row.normal.valueBlobCount,
			&row.normal.observableInputBlobCount,
			&row.normal.relationCount,
			&row.normal.relationSlotCount,
			&row.normal.relationFillerCount,
			&row.normal.orderedCandidatePrefixCount,
			&row.normal.referenceResolutionCount,
			&row.normal.memberOfEvaluationCount,
			&row.normal.memberOfInputCount,
			&row.normal.memberOfUseCount,
			&row.normal.aliasChangeCount,
			&row.normal.retractionCount,
			&row.activationCount,
			&row.topLevelChangeCount,
			&row.closureActivationCount,
		},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return durableProjectTypeEnvActivationFootprint{},
			storedAdmissionIntegrity("stored activation footprint is missing", nil)
	}
	if err != nil {
		return durableProjectTypeEnvActivationFootprint{},
			fmt.Errorf("load stored ProjectTypeEnv activation footprint: %w", err)
	}
	return row, nil
}
