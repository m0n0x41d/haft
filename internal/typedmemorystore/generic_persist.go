package typedmemorystore

import (
	"context"
	"encoding/binary"
	"fmt"
	"strconv"

	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

type genericEventIdentity struct {
	nextRevision           typedmemory.GraphRevision
	expectedSQLiteRevision int64
	nextSQLiteRevision     int64
	commitRef              string
	eventRef               string
	eventDigest            typedmemory.SHA256Digest
	projectionJobRef       string
}

type genericMaterializationFootprint struct {
	entityCount                       int64
	entityContextCount                int64
	entityDeclarationCount            int64
	contextSliceCatalogCount          int64
	contextSliceCount                 int64
	valueBlobCount                    int64
	observableInputBlobCount          int64
	relationCount                     int64
	relationSlotCount                 int64
	relationFillerCount               int64
	orderedCandidatePrefixCount       int64
	referenceResolutionCount          int64
	memberOfEvaluationCount           int64
	memberOfInputCount                int64
	memberOfUseCount                  int64
	kindClassificationSourceBlobCount int64
	kindClassificationEvaluationCount int64
	kindClassificationFeatureCount    int64
	kindClassificationUseCount        int64
	aliasChangeCount                  int64
	retractionCount                   int64
}

type genericSemanticMaterialization struct {
	statements []statement
	footprint  genericMaterializationFootprint
	rowDigests []string
}

type materializationClosureInput struct {
	identity       genericEventIdentity
	basisKind      typedmemory.AdmissionBasisKind
	requestDigest  typedmemory.SHA256Digest
	semanticDigest typedmemory.SHA256Digest
	envelopeDigest typedmemory.SHA256Digest
	basisDigest    typedmemory.SHA256Digest
	manifestDigest typedmemory.SHA256Digest
	footprint      genericMaterializationFootprint
	rowDigests     []string
}

func (adapter *SQLiteAdapter) persistGenericAdmission(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	request CommitRequest,
	revalidated revalidatedAdmission,
	environment typedmemory.TypeEnv,
) (pendingGenericAdmissionRefs, error) {
	prepared := revalidated.prepared
	recordedAt := canonicalTime(adapter.clock.Now())
	identity, err := newGenericEventIdentity(request, prepared)
	if err != nil {
		return pendingGenericAdmissionRefs{}, err
	}
	semantic, err := buildGenericSemanticMaterialization(
		ctx,
		transaction,
		request,
		revalidated,
		environment,
		identity,
		recordedAt,
	)
	if err != nil {
		return pendingGenericAdmissionRefs{}, err
	}
	manifest, err := buildExpectedMaterializationManifest(prepared)
	if err != nil {
		return pendingGenericAdmissionRefs{}, err
	}
	materializationBytes := canonicalMaterializationClosure(
		identity,
		prepared,
		manifest,
		semantic.footprint,
		semantic.rowDigests,
	)
	materializationDigest, err := digestBytes(materializationBytes)
	if err != nil {
		return pendingGenericAdmissionRefs{}, err
	}

	statements := make([]statement, 0, len(semantic.statements)+7)
	statements = append(
		statements,
		genericEventStatement(request, prepared, identity, recordedAt),
		genericAdmissionBasisStatement(request, prepared, manifest, identity, recordedAt),
		genericWriterGenerationStatement(request, identity),
	)
	statements = append(statements, semantic.statements...)
	statements = append(
		statements,
		genericIdempotencyStatement(request, prepared, identity, recordedAt),
		genericProjectionStatement(request, identity, recordedAt),
		genericMaterializationClosureStatement(
			request,
			prepared,
			manifest,
			identity,
			semantic.footprint,
			materializationBytes,
			materializationDigest,
			recordedAt,
		),
		genericCommitStatement(request, prepared, identity, semantic.footprint, recordedAt),
	)
	if err := executeStatements(ctx, transaction, statements, 0); err != nil {
		return pendingGenericAdmissionRefs{}, err
	}
	advanced, err := loadHeadWithScanner(ctx, transaction, request.project)
	if err != nil {
		return pendingGenericAdmissionRefs{}, err
	}
	if advanced.Revision() != identity.nextRevision ||
		advanced.LastEventRef() != identity.eventRef ||
		advanced.LastCommitRef() != identity.commitRef {
		return pendingGenericAdmissionRefs{}, fmt.Errorf("generic typed-memory commit did not advance the exact graph head")
	}
	return pendingGenericAdmissionRefs{
		eventRef:  identity.eventRef,
		commitRef: identity.commitRef,
	}, nil
}

func newGenericEventIdentity(
	request CommitRequest,
	prepared preparedAdmission,
) (genericEventIdentity, error) {
	nextRevision := typedmemory.NewGraphRevision(request.expectedRevision.Value() + 1)
	expectedSQLiteRevision, exact := sqliteIntegerFromUint64(request.expectedRevision.Value())
	if !exact {
		return genericEventIdentity{}, ErrRevisionOverflow
	}
	nextSQLiteRevision, exact := sqliteIntegerFromUint64(nextRevision.Value())
	if !exact {
		return genericEventIdentity{}, ErrRevisionOverflow
	}
	commitRef := derivedRef(
		"typed-memory-commit",
		request.project.String(),
		request.idempotencyKey.String(),
		prepared.semanticDigest.String(),
		strconv.FormatUint(nextRevision.Value(), 10),
	)
	projectionJobRef := derivedRef("typed-memory-projection-job", commitRef)
	eventDigest, err := digestFields(
		"typed-memory-graph-event.v1",
		request.project.String(),
		commitRef,
		strconv.FormatUint(request.expectedRevision.Value(), 10),
		strconv.FormatUint(nextRevision.Value(), 10),
		request.expectedTypeEnv.String(),
		prepared.semanticDigest.String(),
		string(prepared.semanticBytes),
		prepared.eventKind,
		"non_binding_semantic_assertion",
		prepared.requestProvenanceRef,
	)
	if err != nil {
		return genericEventIdentity{}, err
	}
	return genericEventIdentity{
		nextRevision:           nextRevision,
		expectedSQLiteRevision: expectedSQLiteRevision,
		nextSQLiteRevision:     nextSQLiteRevision,
		commitRef:              commitRef,
		eventRef:               derivedRef("typed-memory-event", eventDigest.String()),
		eventDigest:            eventDigest,
		projectionJobRef:       projectionJobRef,
	}, nil
}

func genericEventStatement(
	request CommitRequest,
	prepared preparedAdmission,
	identity genericEventIdentity,
	recordedAt string,
) statement {
	return statement{
		query: `INSERT INTO typed_memory_graph_events (
			project_id, event_ref, commit_ref, event_digest,
			expected_revision, graph_revision,
			basis_type_env_ref, result_type_env_ref,
			change_set_digest, canonical_change_set_bytes, change_count,
			event_kind, authority_class, request_provenance_ref, recorded_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		arguments: []any{
			request.project.String(), identity.eventRef, identity.commitRef,
			identity.eventDigest.String(), identity.expectedSQLiteRevision,
			identity.nextSQLiteRevision, request.expectedTypeEnv.String(),
			request.expectedTypeEnv.String(), prepared.semanticDigest.String(),
			prepared.semanticBytes, int64(len(prepared.changes)), prepared.eventKind,
			"non_binding_semantic_assertion", prepared.requestProvenanceRef, recordedAt,
		},
	}
}

func genericWriterGenerationStatement(
	request CommitRequest,
	identity genericEventIdentity,
) statement {
	basis := request.admissionBatch.Basis()
	basisKind := typedmemory.SnapshotOnlyAdmissionBasis
	if basis != nil {
		basisKind = basis.Kind()
	}
	generation, provenance := admissionWriterGeneration(
		request.ContractVersion(),
		basisKind,
	)
	return statement{
		query: `INSERT INTO typed_memory_event_writer_generations (
			project_id, event_ref, writer_generation, provenance_kind
		) VALUES (?, ?, ?, ?)`,
		arguments: []any{
			request.project.String(),
			identity.eventRef,
			generation,
			provenance,
		},
	}
}

func genericAdmissionBasisStatement(
	request CommitRequest,
	prepared preparedAdmission,
	manifest expectedMaterializationManifest,
	identity genericEventIdentity,
	recordedAt string,
) statement {
	return statement{
		query: `INSERT INTO typed_memory_event_admission_bases (
			project_id, event_ref, event_digest, admission_basis_kind,
			type_env_ref, basis_graph_revision,
			request_digest, canonical_request_bytes,
			semantic_digest, canonical_semantic_bytes,
			admission_envelope_digest, canonical_admission_envelope_bytes,
			admission_basis_digest, canonical_admission_basis_bytes,
			materialization_manifest_digest,
			canonical_materialization_manifest_bytes, recorded_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		arguments: []any{
			request.project.String(), identity.eventRef, identity.eventDigest.String(),
			prepared.basis.Kind().String(), request.expectedTypeEnv.String(),
			identity.expectedSQLiteRevision, prepared.requestDigest.String(),
			prepared.requestBytes, prepared.semanticDigest.String(), prepared.semanticBytes,
			prepared.envelopeDigest.String(), prepared.envelopeBytes,
			prepared.basis.Digest().String(), prepared.basis.CanonicalBytes(),
			manifest.Digest().String(), manifest.CanonicalBytes(), recordedAt,
		},
	}
}

func genericIdempotencyStatement(
	request CommitRequest,
	prepared preparedAdmission,
	identity genericEventIdentity,
	recordedAt string,
) statement {
	return statement{
		query: `INSERT INTO typed_memory_idempotency_history (
			project_id, idempotency_key, change_set_digest,
			event_ref, graph_revision, result_digest, recorded_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		arguments: []any{
			request.project.String(), request.idempotencyKey.String(),
			prepared.semanticDigest.String(), identity.eventRef,
			identity.nextSQLiteRevision, identity.eventDigest.String(), recordedAt,
		},
	}
}

func genericProjectionStatement(
	request CommitRequest,
	identity genericEventIdentity,
	recordedAt string,
) statement {
	return statement{
		query: `INSERT INTO typed_memory_projection_jobs (
			project_id, projection_job_ref, semantic_event_ref,
			graph_revision, target_kind, input_event_digest, recorded_at
		) VALUES (?, ?, ?, ?, 'project_carriers', ?, ?)`,
		arguments: []any{
			request.project.String(), identity.projectionJobRef, identity.eventRef,
			identity.nextSQLiteRevision, identity.eventDigest.String(), recordedAt,
		},
	}
}

func genericMaterializationClosureStatement(
	request CommitRequest,
	prepared preparedAdmission,
	manifest expectedMaterializationManifest,
	identity genericEventIdentity,
	footprint genericMaterializationFootprint,
	materializationBytes []byte,
	materializationDigest typedmemory.SHA256Digest,
	recordedAt string,
) statement {
	return statement{
		query: `INSERT INTO typed_memory_commit_materialization_closures (
			project_id, event_ref, commit_ref, event_digest,
			admission_basis_kind, request_digest, semantic_digest,
			admission_envelope_digest, admission_basis_digest,
			materialization_manifest_digest,
			materialization_digest, canonical_materialization_bytes,
			entity_count, entity_context_count, entity_declaration_count,
			context_slice_catalog_count, context_slice_count,
			value_blob_count, observable_input_blob_count, relation_count,
			relation_slot_count, relation_filler_count,
			ordered_candidate_prefix_count,
			reference_resolution_use_count, memberof_evaluation_count,
			memberof_input_count, memberof_use_count,
			kind_classification_source_blob_count,
			kind_classification_evaluation_count,
			kind_classification_feature_count,
			kind_classification_use_count,
			alias_change_count,
			retraction_count, recorded_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		arguments: []any{
			request.project.String(), identity.eventRef, identity.commitRef,
			identity.eventDigest.String(), prepared.basis.Kind().String(),
			prepared.requestDigest.String(), prepared.semanticDigest.String(),
			prepared.envelopeDigest.String(), prepared.basis.Digest().String(),
			manifest.Digest().String(),
			materializationDigest.String(), materializationBytes,
			footprint.entityCount, footprint.entityContextCount,
			footprint.entityDeclarationCount, footprint.contextSliceCatalogCount,
			footprint.contextSliceCount, footprint.valueBlobCount,
			footprint.observableInputBlobCount, footprint.relationCount,
			footprint.relationSlotCount, footprint.relationFillerCount,
			footprint.orderedCandidatePrefixCount,
			footprint.referenceResolutionCount, footprint.memberOfEvaluationCount,
			footprint.memberOfInputCount, footprint.memberOfUseCount,
			footprint.kindClassificationSourceBlobCount,
			footprint.kindClassificationEvaluationCount,
			footprint.kindClassificationFeatureCount,
			footprint.kindClassificationUseCount,
			footprint.aliasChangeCount, footprint.retractionCount, recordedAt,
		},
	}
}

func genericCommitStatement(
	request CommitRequest,
	prepared preparedAdmission,
	identity genericEventIdentity,
	footprint genericMaterializationFootprint,
	recordedAt string,
) statement {
	return statement{
		query: `INSERT INTO typed_memory_graph_commits (
			project_id, commit_ref, event_ref, event_digest,
			expected_revision, graph_revision, change_set_digest,
			idempotency_key, projection_job_ref,
			entity_count, entity_context_count, recorded_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		arguments: []any{
			request.project.String(), identity.commitRef, identity.eventRef,
			identity.eventDigest.String(), identity.expectedSQLiteRevision,
			identity.nextSQLiteRevision, prepared.semanticDigest.String(),
			request.idempotencyKey.String(), identity.projectionJobRef,
			footprint.entityCount, footprint.entityContextCount, recordedAt,
		},
	}
}

func canonicalMaterializationClosure(
	identity genericEventIdentity,
	prepared preparedAdmission,
	manifest expectedMaterializationManifest,
	footprint genericMaterializationFootprint,
	rowDigests []string,
) []byte {
	input := materializationClosureInput{
		identity:       identity,
		basisKind:      prepared.basis.Kind(),
		requestDigest:  prepared.requestDigest,
		semanticDigest: prepared.semanticDigest,
		envelopeDigest: prepared.envelopeDigest,
		basisDigest:    prepared.basis.Digest(),
		manifestDigest: manifest.Digest(),
		footprint:      footprint,
		rowDigests:     rowDigests,
	}
	return canonicalMaterializationClosureFromInput(input)
}

func canonicalMaterializationClosureFromInput(
	input materializationClosureInput,
) []byte {
	if input.basisKind == typedmemory.ContextSliceClassificationAdmissionBasis {
		return canonicalClassificationMaterializationClosure(input)
	}
	fields := []string{
		input.identity.eventRef,
		input.identity.commitRef,
		input.identity.eventDigest.String(),
		input.basisKind.String(),
		input.requestDigest.String(),
		input.semanticDigest.String(),
		input.envelopeDigest.String(),
		input.basisDigest.String(),
		input.manifestDigest.String(),
		strconv.FormatInt(input.footprint.entityCount, 10),
		strconv.FormatInt(input.footprint.entityContextCount, 10),
		strconv.FormatInt(input.footprint.entityDeclarationCount, 10),
		strconv.FormatInt(input.footprint.contextSliceCatalogCount, 10),
		strconv.FormatInt(input.footprint.contextSliceCount, 10),
		strconv.FormatInt(input.footprint.valueBlobCount, 10),
		strconv.FormatInt(input.footprint.observableInputBlobCount, 10),
		strconv.FormatInt(input.footprint.relationCount, 10),
		strconv.FormatInt(input.footprint.relationSlotCount, 10),
		strconv.FormatInt(input.footprint.relationFillerCount, 10),
		strconv.FormatInt(input.footprint.orderedCandidatePrefixCount, 10),
		strconv.FormatInt(input.footprint.referenceResolutionCount, 10),
		strconv.FormatInt(input.footprint.memberOfEvaluationCount, 10),
		strconv.FormatInt(input.footprint.memberOfInputCount, 10),
		strconv.FormatInt(input.footprint.memberOfUseCount, 10),
		strconv.FormatInt(input.footprint.aliasChangeCount, 10),
		strconv.FormatInt(input.footprint.retractionCount, 10),
		strconv.Itoa(len(input.rowDigests)),
	}
	fields = append(fields, input.rowDigests...)
	return canonicalStorageFields("typed-memory-materialization-closure.v1", fields)
}

func canonicalClassificationMaterializationClosure(
	input materializationClosureInput,
) []byte {
	fields := []string{
		input.identity.eventRef,
		input.identity.commitRef,
		input.identity.eventDigest.String(),
		input.basisKind.String(),
		input.requestDigest.String(),
		input.semanticDigest.String(),
		input.envelopeDigest.String(),
		input.basisDigest.String(),
		input.manifestDigest.String(),
		strconv.FormatInt(input.footprint.entityCount, 10),
		strconv.FormatInt(input.footprint.entityContextCount, 10),
		strconv.FormatInt(input.footprint.entityDeclarationCount, 10),
		strconv.FormatInt(input.footprint.contextSliceCatalogCount, 10),
		strconv.FormatInt(input.footprint.contextSliceCount, 10),
		strconv.FormatInt(input.footprint.valueBlobCount, 10),
		strconv.FormatInt(input.footprint.observableInputBlobCount, 10),
		strconv.FormatInt(input.footprint.relationCount, 10),
		strconv.FormatInt(input.footprint.relationSlotCount, 10),
		strconv.FormatInt(input.footprint.relationFillerCount, 10),
		strconv.FormatInt(input.footprint.orderedCandidatePrefixCount, 10),
		strconv.FormatInt(input.footprint.referenceResolutionCount, 10),
		strconv.FormatInt(input.footprint.memberOfEvaluationCount, 10),
		strconv.FormatInt(input.footprint.memberOfInputCount, 10),
		strconv.FormatInt(input.footprint.memberOfUseCount, 10),
		strconv.FormatInt(input.footprint.kindClassificationSourceBlobCount, 10),
		strconv.FormatInt(input.footprint.kindClassificationEvaluationCount, 10),
		strconv.FormatInt(input.footprint.kindClassificationFeatureCount, 10),
		strconv.FormatInt(input.footprint.kindClassificationUseCount, 10),
		strconv.FormatInt(input.footprint.aliasChangeCount, 10),
		strconv.FormatInt(input.footprint.retractionCount, 10),
		strconv.Itoa(len(input.rowDigests)),
	}
	fields = append(fields, input.rowDigests...)
	return canonicalStorageFields("typed-memory-materialization-closure.v2", fields)
}

func canonicalStorageFields(domain string, fields []string) []byte {
	buffer := make([]byte, 0)
	appendField := func(value string) {
		var length [8]byte
		binary.BigEndian.PutUint64(length[:], uint64(len(value)))
		buffer = append(buffer, length[:]...)
		buffer = append(buffer, value...)
	}
	appendField("haft.typedmemorystore.canonical.v1")
	appendField(domain)
	for _, field := range fields {
		appendField(field)
	}
	return buffer
}
