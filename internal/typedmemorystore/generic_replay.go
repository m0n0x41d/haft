package typedmemorystore

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	genericStorageWriterGeneration      = int64(46)
	genericStorageCapabilityKey         = "typed_memory_writer_generation"
	genericStorageCapabilityDigest      = "sha256:ef94ecbd2590981b016eb89ba3f1c4f9a101ebbbf779355e6c3f7e13a5cdd58a"
	genericStorageCapabilityBytes       = "haft.typed-memory.storage.writer-generation=46"
	relationalAssertionWriterGeneration = int64(53)
	relationalAssertionCapabilityKey    = "typed_memory_assert_relation_writer_generation"
	relationalAssertionCapabilityDigest = "sha256:a2445bb17e50f89d3c943fd37c7b50203ce38a7d3d303b1cabab45ee57d12a0d"
	relationalAssertionCapabilityBytes  = "haft.typed-memory.storage.assert-relation-writer-generation=53"
	kindClassificationWriterGeneration  = int64(54)
	kindClassificationCapabilityKey     = "typed_memory_kind_classification_writer_generation"
	kindClassificationCapabilityDigest  = "sha256:1395bb80205b84b5b6a57e1e4a9b71ec559f93b0c50e518847a408a68ffcbe37"
	kindClassificationCapabilityBytes   = "haft.typed-memory.storage.kind-classification-writer-generation=54"
)

// durableGenericAdmissionExpectation is the exact immutable identity of one
// generic admission attempt. Replay compares all four independently sealed
// carriers: request, semantic effect, admission envelope, and admission basis.
// It intentionally contains no materialized graph rows because those are
// verified against the durable closure rather than trusted from the caller.
type durableGenericAdmissionExpectation struct {
	contractVersion   AdmissionContractVersion
	project           projectledger.ProjectID
	idempotencyKey    IdempotencyKey
	expectedRevision  typedmemory.GraphRevision
	basisTypeEnv      typedmemory.TypeEnvRef
	requestDigest     typedmemory.SHA256Digest
	requestBytes      []byte
	semanticDigest    typedmemory.SHA256Digest
	semanticBytes     []byte
	envelopeDigest    typedmemory.SHA256Digest
	envelopeBytes     []byte
	basisKind         typedmemory.AdmissionBasisKind
	basisDigest       typedmemory.SHA256Digest
	basisBytes        []byte
	eventKind         string
	authorityClass    string
	requestProvenance string
	changeCount       int64
	requiredEventRef  string
	requiredCommitRef string
	prepared          preparedAdmission
	manifest          expectedMaterializationManifest
}

func newDurableGenericAdmissionExpectation(
	request CommitRequest,
	prepared preparedAdmission,
) (durableGenericAdmissionExpectation, error) {
	if prepared.basis == nil || !prepared.batch.IsValid() {
		return durableGenericAdmissionExpectation{}, ErrInvalidAdmissionBatch
	}
	changeCount := len(prepared.changes)
	if changeCount == 0 {
		return durableGenericAdmissionExpectation{}, ErrInvalidAdmissionBatch
	}
	manifest, err := buildExpectedMaterializationManifest(prepared)
	if err != nil {
		return durableGenericAdmissionExpectation{}, fmt.Errorf(
			"build exact expected materialization manifest: %w",
			err,
		)
	}
	return durableGenericAdmissionExpectation{
		contractVersion:   request.ContractVersion(),
		project:           request.project,
		idempotencyKey:    request.idempotencyKey,
		expectedRevision:  request.expectedRevision,
		basisTypeEnv:      request.expectedTypeEnv,
		requestDigest:     prepared.requestDigest,
		requestBytes:      append([]byte(nil), prepared.requestBytes...),
		semanticDigest:    prepared.semanticDigest,
		semanticBytes:     append([]byte(nil), prepared.semanticBytes...),
		envelopeDigest:    prepared.envelopeDigest,
		envelopeBytes:     append([]byte(nil), prepared.envelopeBytes...),
		basisKind:         prepared.basis.Kind(),
		basisDigest:       prepared.basis.Digest(),
		basisBytes:        prepared.basis.CanonicalBytes(),
		eventKind:         prepared.eventKind,
		authorityClass:    "non_binding_semantic_assertion",
		requestProvenance: prepared.requestProvenanceRef,
		changeCount:       int64(changeCount),
		prepared: preparedAdmission{
			batch:          prepared.batch,
			basis:          prepared.basis,
			candidate:      prepared.candidate,
			requestDigest:  prepared.requestDigest,
			semanticDigest: prepared.semanticDigest,
			envelopeDigest: prepared.envelopeDigest,
			changes:        append([]preparedAdmissionChange(nil), prepared.changes...),
		},
		manifest: manifest,
	}, nil
}

func durableGenericExpectationFromPrepared(
	request CommitRequest,
	prepared preparedAdmission,
) (durableGenericAdmissionExpectation, error) {
	if prepared.basis == nil {
		return durableGenericAdmissionExpectation{}, ErrInvalidAdmissionBatch
	}
	if request.ContractVersion().String() == "" {
		return durableGenericAdmissionExpectation{}, ErrInvalidAdmissionBatch
	}
	return newDurableGenericAdmissionExpectation(request, prepared)
}

type durableGenericCommonRow struct {
	idempotencyChangeDigest  string
	idempotencyEventRef      string
	idempotencyRevision      int64
	idempotencyResultDigest  string
	eventCommitRef           string
	eventDigest              string
	eventExpectedRevision    int64
	eventRevision            int64
	eventBasisTypeEnv        string
	eventResultTypeEnv       string
	eventChangeDigest        string
	eventCanonicalBytes      []byte
	eventChangeCount         int64
	eventKind                string
	eventAuthorityClass      string
	eventProvenance          string
	eventRecordedAt          string
	commitRef                string
	commitEventRef           string
	commitEventDigest        string
	commitExpectedRevision   int64
	commitRevision           int64
	commitChangeDigest       string
	commitIdempotencyKey     string
	commitProjectionJobRef   string
	commitEntityCount        int64
	commitEntityContextCount int64
}

// durableGenericAdmissionDetail is a closed storage-generation union. A v45
// event remains readable as common event/commit/idempotency history, but has no
// admission basis. Only the v46 variant can satisfy generic exact replay.
type durableGenericAdmissionDetail interface {
	commonRow() durableGenericCommonRow
	durableGenericAdmissionDetailVariant()
}

type durableLegacyV45AdmissionDetail struct {
	common durableGenericCommonRow
}

func (detail durableLegacyV45AdmissionDetail) commonRow() durableGenericCommonRow {
	return detail.common
}

func (durableLegacyV45AdmissionDetail) durableGenericAdmissionDetailVariant() {}

type durableV46AdmissionDetail struct {
	common          durableGenericCommonRow
	writer          eventWriterGeneration
	admission       durableV46AdmissionRow
	closure         durableV46ClosureRow
	actualFootprint durableV46FootprintRow
	rowDigests      []string
}

func (detail durableV46AdmissionDetail) commonRow() durableGenericCommonRow {
	return detail.common
}

func (durableV46AdmissionDetail) durableGenericAdmissionDetailVariant() {}

type durableV46AdmissionRow struct {
	eventDigest    string
	basisKind      string
	typeEnvRef     string
	basisRevision  int64
	requestDigest  string
	requestBytes   []byte
	semanticDigest string
	semanticBytes  []byte
	envelopeDigest string
	envelopeBytes  []byte
	basisDigest    string
	basisBytes     []byte
	manifestDigest string
	manifestBytes  []byte
	recordedAt     string
}

type durableV46ClosureRow struct {
	commitRef                         string
	eventDigest                       string
	basisKind                         string
	requestDigest                     string
	semanticDigest                    string
	envelopeDigest                    string
	basisDigest                       string
	manifestDigest                    string
	materializationDigest             string
	materializationBytes              []byte
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
	recordedAt                        string
}

type durableV46FootprintRow struct {
	footprint           genericMaterializationFootprint
	topLevelChangeCount int64
}

type eventWriterGeneration struct {
	generation int64
	provenance string
}

func loadDurableGenericAdmissionDetail(
	ctx context.Context,
	source scanner,
	expectation durableGenericAdmissionExpectation,
) (durableGenericAdmissionDetail, bool, error) {
	project := expectation.project
	key := expectation.idempotencyKey
	common, found, err := loadDurableGenericCommonRow(ctx, source, project, key)
	if err != nil || !found {
		return nil, found, err
	}
	if err := verifyStoredGenericEventIdentity(project, common); err != nil {
		return nil, true, err
	}
	if err := requireExpectedStorageAvailability(
		ctx,
		source,
		expectation.contractVersion,
		expectation.basisKind,
	); err != nil {
		return nil, false, err
	}
	admission, admissionFound, err := loadDurableV46AdmissionRow(
		ctx,
		source,
		project,
		common.idempotencyEventRef,
	)
	if err != nil {
		return nil, false, err
	}
	closure, closureFound, err := loadDurableV46ClosureRow(
		ctx,
		source,
		project,
		common.idempotencyEventRef,
	)
	if err != nil {
		return nil, false, err
	}
	writer, writerFound, err := loadEventWriterGeneration(
		ctx,
		source,
		project,
		common.idempotencyEventRef,
	)
	if err != nil {
		return nil, false, err
	}
	if err := verifyExpectedReplayWriter(
		expectation.contractVersion,
		expectation.basisKind,
		writer,
		writerFound,
	); err != nil {
		return nil, true, err
	}
	if !admissionFound || !closureFound {
		return nil, true, storedAdmissionIntegrity(
			"event writer generation companion completeness",
			nil,
		)
	}
	if err := verifyStoredGenericAdmissionCarriers(admission); err != nil {
		return nil, true, err
	}
	if err := verifyStoredGenericAdmissionLinks(common, admission, closure); err != nil {
		return nil, true, err
	}
	if !bytes.Equal(admission.requestBytes, expectation.requestBytes) {
		return nil, true, genericReplayConflict("request bytes", nil)
	}
	if admission.requestDigest != expectation.requestDigest.String() {
		return nil, true, genericReplayConflict("request digest", nil)
	}
	if !bytes.Equal(admission.semanticBytes, expectation.semanticBytes) {
		return nil, true, genericReplayConflict("semantic bytes", nil)
	}
	if admission.semanticDigest != expectation.semanticDigest.String() {
		return nil, true, genericReplayConflict("semantic digest", nil)
	}
	if common.eventProvenance != expectation.requestProvenance {
		return nil, true, genericReplayConflict("request provenance", nil)
	}
	if !bytes.Equal(admission.envelopeBytes, expectation.envelopeBytes) {
		return nil, true, genericReplayConflict("admission envelope bytes", nil)
	}
	if admission.envelopeDigest != expectation.envelopeDigest.String() {
		return nil, true, genericReplayConflict("admission envelope digest", nil)
	}
	if !bytes.Equal(admission.basisBytes, expectation.basisBytes) {
		return nil, true, genericReplayConflict("admission basis bytes", nil)
	}
	if admission.basisKind != expectation.basisKind.String() ||
		admission.basisDigest != expectation.basisDigest.String() {
		return nil, true, genericReplayConflict("admission basis identity", nil)
	}
	actualFootprint, err := loadDurableV46FootprintRow(
		ctx,
		source,
		project,
		common.idempotencyEventRef,
	)
	if err != nil {
		return nil, false, err
	}
	rowDigests, err := loadDurableV46RowDigests(
		ctx,
		source,
		project,
		common.idempotencyEventRef,
		expectation.prepared,
		expectation.contractVersion,
	)
	if err != nil {
		return nil, false, err
	}
	if err := verifyActualMaterializationManifest(
		ctx,
		source,
		project,
		common.idempotencyEventRef,
		expectation.manifest,
	); err != nil {
		return nil, true, err
	}
	return durableV46AdmissionDetail{
		common:          common,
		writer:          writer,
		admission:       admission,
		closure:         closure,
		actualFootprint: actualFootprint,
		rowDigests:      rowDigests,
	}, true, nil
}

func verifyStoredGenericEventIdentity(
	project projectledger.ProjectID,
	common durableGenericCommonRow,
) error {
	if _, err := parseCanonicalGenericRecordedAt(common.eventRecordedAt); err != nil {
		return storedAdmissionIntegrity("stored event recorded_at", err)
	}
	expectedRevision, err := graphRevisionFromSQLite(common.eventExpectedRevision)
	if err != nil {
		return storedAdmissionIntegrity("stored event expected revision", err)
	}
	graphRevision, err := graphRevisionFromSQLite(common.eventRevision)
	if err != nil {
		return storedAdmissionIntegrity("stored event graph revision", err)
	}
	if graphRevision.Value() != expectedRevision.Value()+1 {
		return storedAdmissionIntegrity("stored event revision is not contiguous", nil)
	}
	basisTypeEnv, err := parseTypeEnvRef(common.eventBasisTypeEnv)
	if err != nil {
		return storedAdmissionIntegrity("stored event basis TypeEnv", err)
	}
	changeDigest, err := verifyStoredGenericCanonicalCarrier(
		"stored event semantic carrier",
		common.eventCanonicalBytes,
		common.eventChangeDigest,
	)
	if err != nil {
		return err
	}
	if common.idempotencyChangeDigest != changeDigest.String() ||
		common.commitChangeDigest != changeDigest.String() {
		return storedAdmissionIntegrity("stored event semantic digest copies", nil)
	}
	recomputed, err := digestFields(
		"typed-memory-graph-event.v1",
		project.String(),
		common.eventCommitRef,
		strconv.FormatUint(expectedRevision.Value(), 10),
		strconv.FormatUint(graphRevision.Value(), 10),
		basisTypeEnv.String(),
		changeDigest.String(),
		string(common.eventCanonicalBytes),
		common.eventKind,
		common.eventAuthorityClass,
		common.eventProvenance,
	)
	if err != nil {
		return storedAdmissionIntegrity("stored event digest", err)
	}
	if common.eventDigest != recomputed.String() ||
		common.idempotencyResultDigest != common.eventDigest ||
		common.commitEventDigest != common.eventDigest {
		return storedAdmissionIntegrity("stored event digest", nil)
	}
	expectedEventRef := derivedRef("typed-memory-event", recomputed.String())
	if common.idempotencyEventRef != expectedEventRef ||
		common.commitEventRef != expectedEventRef {
		return storedAdmissionIntegrity("stored event ref", nil)
	}
	return nil
}

func verifyStoredGenericAdmissionCarriers(admission durableV46AdmissionRow) error {
	requestDigest, err := verifyStoredGenericCanonicalCarrier(
		"stored request carrier",
		admission.requestBytes,
		admission.requestDigest,
	)
	if err != nil {
		return err
	}
	semanticDigest, err := verifyStoredGenericCanonicalCarrier(
		"stored semantic carrier",
		admission.semanticBytes,
		admission.semanticDigest,
	)
	if err != nil {
		return err
	}
	basisDigest, err := verifyStoredGenericCanonicalCarrier(
		"stored admission-basis carrier",
		admission.basisBytes,
		admission.basisDigest,
	)
	if err != nil {
		return err
	}
	manifestDigest, err := verifyStoredGenericCanonicalCarrier(
		"stored expected-materialization manifest carrier",
		admission.manifestBytes,
		admission.manifestDigest,
	)
	if err != nil {
		return err
	}
	envelopeDigest, err := verifyStoredGenericCanonicalCarrier(
		"stored admission-envelope carrier",
		admission.envelopeBytes,
		admission.envelopeDigest,
	)
	if err != nil {
		return err
	}
	basisKind, err := typedmemory.ParseAdmissionBasisKind(admission.basisKind)
	if err != nil {
		return storedAdmissionIntegrity("stored admission-basis kind", err)
	}
	basisTypeEnv, err := typedmemory.ParseTypeEnvRef(admission.typeEnvRef)
	if err != nil {
		return storedAdmissionIntegrity("stored admission-basis TypeEnv", err)
	}
	if admission.basisRevision < 0 {
		return storedAdmissionIntegrity("stored admission-basis revision", nil)
	}
	basisRevision := typedmemory.NewGraphRevision(uint64(admission.basisRevision))
	coordinateErr := typedmemory.VerifyStoredAdmissionBasisCoordinates(
		basisKind,
		basisTypeEnv,
		basisRevision,
		admission.basisBytes,
	)
	if coordinateErr != nil {
		return storedAdmissionIntegrity("stored admission-basis coordinates", coordinateErr)
	}
	manifest, err := decodeExpectedMaterializationManifest(
		admission.manifestBytes,
		manifestDigest,
		uint64(admission.basisRevision),
	)
	if err != nil {
		return err
	}
	manifestMatchesAdmission := manifest.requestDigest == requestDigest &&
		manifest.semanticDigest == semanticDigest &&
		manifest.basisDigest == basisDigest
	if !manifestMatchesAdmission {
		return storedAdmissionIntegrity(
			"stored expected-materialization manifest correlation",
			nil,
		)
	}
	verifyErr := typedmemory.VerifyStoredAdmissionEnvelope(
		typedmemory.StoredAdmissionEnvelopeInput{
			RequestDigest:  requestDigest,
			SemanticDigest: semanticDigest,
			SemanticBytes:  admission.semanticBytes,
			BasisKind:      basisKind,
			BasisDigest:    basisDigest,
			BasisBytes:     admission.basisBytes,
			EnvelopeDigest: envelopeDigest,
			EnvelopeBytes:  admission.envelopeBytes,
		},
	)
	if verifyErr != nil {
		return storedAdmissionIntegrity("stored admission-envelope correlation", verifyErr)
	}
	return nil
}

func verifyStoredGenericCanonicalCarrier(
	detail string,
	canonical []byte,
	storedDigestText string,
) (typedmemory.SHA256Digest, error) {
	storedDigest, err := typedmemory.NewSHA256Digest(storedDigestText)
	if err != nil {
		return typedmemory.SHA256Digest{}, storedAdmissionIntegrity(detail+" digest", err)
	}
	recomputed, err := digestBytes(canonical)
	if err != nil {
		return typedmemory.SHA256Digest{}, storedAdmissionIntegrity(detail+" digest", err)
	}
	if recomputed != storedDigest {
		return typedmemory.SHA256Digest{}, storedAdmissionIntegrity(detail+" bytes", nil)
	}
	return storedDigest, nil
}

func verifyStoredGenericAdmissionLinks(
	common durableGenericCommonRow,
	admission durableV46AdmissionRow,
	closure durableV46ClosureRow,
) error {
	if err := verifyStoredGenericRecordedAtLinks(common, admission, closure); err != nil {
		return err
	}
	checks := []struct {
		matches bool
		detail  string
	}{
		{admission.eventDigest == common.eventDigest, "admission event digest"},
		{admission.typeEnvRef == common.eventBasisTypeEnv, "admission basis TypeEnv"},
		{admission.basisRevision == common.eventExpectedRevision, "admission basis revision"},
		{admission.semanticDigest == common.eventChangeDigest, "admission semantic digest"},
		{bytes.Equal(admission.semanticBytes, common.eventCanonicalBytes), "admission semantic bytes"},
		{closure.commitRef == common.eventCommitRef, "closure commit ref"},
		{closure.eventDigest == common.eventDigest, "closure event digest"},
		{closure.basisKind == admission.basisKind, "closure admission-basis kind"},
		{closure.requestDigest == admission.requestDigest, "closure request digest"},
		{closure.semanticDigest == admission.semanticDigest, "closure semantic digest"},
		{closure.envelopeDigest == admission.envelopeDigest, "closure admission-envelope digest"},
		{closure.basisDigest == admission.basisDigest, "closure admission-basis digest"},
		{closure.manifestDigest == admission.manifestDigest, "closure expected-materialization manifest digest"},
	}
	for _, check := range checks {
		if !check.matches {
			return storedAdmissionIntegrity("stored "+check.detail, nil)
		}
	}
	return nil
}

func verifyStoredGenericRecordedAtLinks(
	common durableGenericCommonRow,
	admission durableV46AdmissionRow,
	closure durableV46ClosureRow,
) error {
	eventRecordedAt, err := parseCanonicalGenericRecordedAt(common.eventRecordedAt)
	if err != nil {
		return storedAdmissionIntegrity("stored event recorded_at", err)
	}
	admissionRecordedAt, err := parseCanonicalGenericRecordedAt(admission.recordedAt)
	if err != nil {
		return storedAdmissionIntegrity("stored admission recorded_at", err)
	}
	closureRecordedAt, err := parseCanonicalGenericRecordedAt(closure.recordedAt)
	if err != nil {
		return storedAdmissionIntegrity("stored closure recorded_at", err)
	}
	if !admissionRecordedAt.Equal(eventRecordedAt) {
		return storedAdmissionIntegrity("stored admission recorded_at link", nil)
	}
	if !closureRecordedAt.Equal(eventRecordedAt) {
		return storedAdmissionIntegrity("stored closure recorded_at link", nil)
	}
	return nil
}

func parseCanonicalGenericRecordedAt(raw string) (time.Time, error) {
	value, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse canonical generic recorded_at: %w", err)
	}
	if canonicalTime(value) != raw {
		return time.Time{}, fmt.Errorf(
			"generic recorded_at must use canonical UTC RFC3339Nano form",
		)
	}
	return value.UTC(), nil
}

func loadDurableGenericCommonRow(
	ctx context.Context,
	source scanner,
	project projectledger.ProjectID,
	key IdempotencyKey,
) (durableGenericCommonRow, bool, error) {
	row := durableGenericCommonRow{}
	err := source.ScanOne(
		ctx,
		`SELECT
			idempotency.change_set_digest, idempotency.event_ref,
			idempotency.graph_revision, idempotency.result_digest,
			event.commit_ref, event.event_digest, event.expected_revision,
			event.graph_revision, event.basis_type_env_ref,
			event.result_type_env_ref, event.change_set_digest,
			event.canonical_change_set_bytes, event.change_count,
				event.event_kind, event.authority_class, event.request_provenance_ref,
				event.recorded_at,
				commit_record.commit_ref, commit_record.event_ref,
			commit_record.event_digest, commit_record.expected_revision,
			commit_record.graph_revision, commit_record.change_set_digest,
			commit_record.idempotency_key, commit_record.projection_job_ref,
			commit_record.entity_count, commit_record.entity_context_count
		FROM typed_memory_idempotency_history idempotency
		JOIN typed_memory_graph_events event
			ON event.project_id = idempotency.project_id
			AND event.event_ref = idempotency.event_ref
		JOIN typed_memory_graph_commits commit_record
			ON commit_record.project_id = event.project_id
			AND commit_record.event_ref = event.event_ref
		WHERE idempotency.project_id = ? AND idempotency.idempotency_key = ?`,
		[]any{project.String(), key.String()},
		[]any{
			&row.idempotencyChangeDigest,
			&row.idempotencyEventRef,
			&row.idempotencyRevision,
			&row.idempotencyResultDigest,
			&row.eventCommitRef,
			&row.eventDigest,
			&row.eventExpectedRevision,
			&row.eventRevision,
			&row.eventBasisTypeEnv,
			&row.eventResultTypeEnv,
			&row.eventChangeDigest,
			&row.eventCanonicalBytes,
			&row.eventChangeCount,
			&row.eventKind,
			&row.eventAuthorityClass,
			&row.eventProvenance,
			&row.eventRecordedAt,
			&row.commitRef,
			&row.commitEventRef,
			&row.commitEventDigest,
			&row.commitExpectedRevision,
			&row.commitRevision,
			&row.commitChangeDigest,
			&row.commitIdempotencyKey,
			&row.commitProjectionJobRef,
			&row.commitEntityCount,
			&row.commitEntityContextCount,
		},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return durableGenericCommonRow{}, false, nil
	}
	if err != nil {
		return durableGenericCommonRow{}, false, fmt.Errorf("load generic typed-memory idempotency history: %w", err)
	}
	return row, true, nil
}

type genericStorageAvailability uint8

const (
	genericStorageAbsent genericStorageAvailability = iota + 1
	genericStoragePartial
	genericStorageExact
)

func loadGenericStorageAvailability(
	ctx context.Context,
	source scanner,
) (genericStorageAvailability, error) {
	var tableCount int64
	err := source.ScanOne(
		ctx,
		`SELECT COUNT(*) FROM sqlite_master
		WHERE type = 'table' AND name IN (
			'typed_memory_storage_capabilities',
			'typed_memory_event_writer_generations',
			'typed_memory_event_admission_bases',
			'typed_memory_commit_materialization_closures'
		)`,
		[]any{},
		[]any{&tableCount},
	)
	if err != nil {
		return 0, fmt.Errorf("inspect typed-memory storage generation: %w", err)
	}
	if tableCount == 0 {
		var installedGenerationCount int64
		err := source.ScanOne(
			ctx,
			`SELECT COUNT(*) FROM schema_version WHERE version >= ?`,
			[]any{genericStorageWriterGeneration},
			[]any{&installedGenerationCount},
		)
		if err != nil {
			return 0, fmt.Errorf("inspect typed-memory storage schema generation: %w", err)
		}
		if installedGenerationCount == 0 {
			return genericStorageAbsent, nil
		}
		return genericStoragePartial, nil
	}
	if tableCount != 4 {
		return genericStoragePartial, nil
	}
	var generation int64
	var digest string
	var canonical []byte
	err = source.ScanOne(
		ctx,
		`SELECT writer_generation, capability_digest, canonical_bytes
		FROM typed_memory_storage_capabilities WHERE capability_key = ?`,
		[]any{genericStorageCapabilityKey},
		[]any{&generation, &digest, &canonical},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return genericStoragePartial, nil
	}
	if err != nil {
		return 0, fmt.Errorf("load typed-memory storage generation: %w", err)
	}
	matches := generation == genericStorageWriterGeneration &&
		digest == genericStorageCapabilityDigest &&
		bytes.Equal(canonical, []byte(genericStorageCapabilityBytes))
	if !matches {
		return genericStoragePartial, nil
	}
	return genericStorageExact, nil
}

func loadRelationalAssertionStorageAvailability(
	ctx context.Context,
	source scanner,
) (genericStorageAvailability, error) {
	var tableCount int64
	err := source.ScanOne(
		ctx,
		`SELECT COUNT(*) FROM sqlite_master
		WHERE type = 'table' AND name = 'typed_memory_writer_capabilities_v53'`,
		[]any{},
		[]any{&tableCount},
	)
	if err != nil {
		return 0, fmt.Errorf(
			"inspect typed-memory relational-assertion storage generation: %w",
			err,
		)
	}
	if tableCount == 0 {
		var installedGenerationCount int64
		err := source.ScanOne(
			ctx,
			`SELECT COUNT(*) FROM schema_version WHERE version >= ?`,
			[]any{relationalAssertionWriterGeneration},
			[]any{&installedGenerationCount},
		)
		if err != nil {
			return 0, fmt.Errorf(
				"inspect typed-memory relational-assertion schema generation: %w",
				err,
			)
		}
		if installedGenerationCount == 0 {
			return genericStorageAbsent, nil
		}
		return genericStoragePartial, nil
	}
	var generation int64
	var digest string
	var canonical []byte
	err = source.ScanOne(
		ctx,
		`SELECT writer_generation, capability_digest, canonical_bytes
		FROM typed_memory_writer_capabilities_v53 WHERE capability_key = ?`,
		[]any{relationalAssertionCapabilityKey},
		[]any{&generation, &digest, &canonical},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return genericStoragePartial, nil
	}
	if err != nil {
		return 0, fmt.Errorf(
			"load typed-memory relational-assertion storage generation: %w",
			err,
		)
	}
	matches := generation == relationalAssertionWriterGeneration &&
		digest == relationalAssertionCapabilityDigest &&
		bytes.Equal(canonical, []byte(relationalAssertionCapabilityBytes))
	if !matches {
		return genericStoragePartial, nil
	}
	return genericStorageExact, nil
}

func loadKindClassificationStorageAvailability(
	ctx context.Context,
	source scanner,
) (genericStorageAvailability, error) {
	var tableCount int64
	err := source.ScanOne(
		ctx,
		`SELECT COUNT(*) FROM sqlite_master
		WHERE type = 'table' AND name IN (
			'typed_memory_writer_capabilities_v54',
			'typed_memory_kind_classification_source_blobs_v54',
			'typed_memory_kind_classification_evaluations_v54',
			'typed_memory_kind_classification_features_v54',
			'typed_memory_relational_assertion_classification_uses_v54'
		)`,
		[]any{},
		[]any{&tableCount},
	)
	if err != nil {
		return 0, fmt.Errorf(
			"inspect typed-memory kind-classification storage generation: %w",
			err,
		)
	}
	if tableCount == 0 {
		var installedGenerationCount int64
		err := source.ScanOne(
			ctx,
			`SELECT COUNT(*) FROM schema_version WHERE version >= ?`,
			[]any{kindClassificationWriterGeneration},
			[]any{&installedGenerationCount},
		)
		if err != nil {
			return 0, fmt.Errorf(
				"inspect typed-memory kind-classification schema generation: %w",
				err,
			)
		}
		if installedGenerationCount == 0 {
			return genericStorageAbsent, nil
		}
		return genericStoragePartial, nil
	}
	if tableCount != 5 {
		return genericStoragePartial, nil
	}
	var generation int64
	var digest string
	var canonical []byte
	err = source.ScanOne(
		ctx,
		`SELECT writer_generation, capability_digest, canonical_bytes
		FROM typed_memory_writer_capabilities_v54 WHERE capability_key = ?`,
		[]any{kindClassificationCapabilityKey},
		[]any{&generation, &digest, &canonical},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return genericStoragePartial, nil
	}
	if err != nil {
		return 0, fmt.Errorf(
			"load typed-memory kind-classification storage generation: %w",
			err,
		)
	}
	matches := generation == kindClassificationWriterGeneration
	matches = matches && digest == kindClassificationCapabilityDigest
	matches = matches && bytes.Equal(canonical, []byte(kindClassificationCapabilityBytes))
	if !matches {
		return genericStoragePartial, nil
	}
	return genericStorageExact, nil
}

func requireExpectedStorageAvailability(
	ctx context.Context,
	source scanner,
	version AdmissionContractVersion,
	basisKind typedmemory.AdmissionBasisKind,
) error {
	availability, err := loadGenericStorageAvailability(ctx, source)
	if err != nil {
		return err
	}
	if availability != genericStorageExact {
		return ErrStorageGenerationUnavailable
	}
	if version.IsV1() {
		return nil
	}
	if !version.IsV2() {
		return ErrStorageGenerationUnavailable
	}
	relational, err := loadRelationalAssertionStorageAvailability(ctx, source)
	if err != nil {
		return err
	}
	if relational != genericStorageExact {
		return ErrStorageGenerationUnavailable
	}
	if basisKind != typedmemory.ContextSliceClassificationAdmissionBasis {
		return nil
	}
	classification, err := loadKindClassificationStorageAvailability(ctx, source)
	if err != nil {
		return err
	}
	if classification != genericStorageExact {
		return ErrStorageGenerationUnavailable
	}
	return nil
}

func admissionWriterGeneration(
	version AdmissionContractVersion,
	basisKind typedmemory.AdmissionBasisKind,
) (int64, string) {
	if basisKind == typedmemory.ContextSliceClassificationAdmissionBasis {
		return kindClassificationWriterGeneration, "writer_v54"
	}
	if version.IsV2() {
		return relationalAssertionWriterGeneration, "writer_v53"
	}
	return genericStorageWriterGeneration, "writer_v46"
}

func verifyExpectedReplayWriter(
	version AdmissionContractVersion,
	basisKind typedmemory.AdmissionBasisKind,
	writer eventWriterGeneration,
	found bool,
) error {
	if !found {
		return storedAdmissionIntegrity("event writer generation marker", nil)
	}
	validMarker := (writer.generation == 45 && writer.provenance == "migration_v45_backfill") ||
		(writer.generation == genericStorageWriterGeneration && writer.provenance == "writer_v46") ||
		(writer.generation == relationalAssertionWriterGeneration && writer.provenance == "writer_v53") ||
		(writer.generation == kindClassificationWriterGeneration && writer.provenance == "writer_v54")
	if !validMarker {
		return storedAdmissionIntegrity("event writer generation marker", nil)
	}
	expectedGeneration, expectedProvenance := admissionWriterGeneration(version, basisKind)
	if writer.generation != expectedGeneration || writer.provenance != expectedProvenance {
		return genericReplayConflict("occupied idempotency coordinate belongs to another admission generation", nil)
	}
	return nil
}

func loadEventWriterGeneration(
	ctx context.Context,
	source scanner,
	project projectledger.ProjectID,
	eventRef string,
) (eventWriterGeneration, bool, error) {
	row := eventWriterGeneration{}
	err := source.ScanOne(
		ctx,
		`SELECT writer_generation, provenance_kind
		FROM typed_memory_event_writer_generations
		WHERE project_id = ? AND event_ref = ?`,
		[]any{project.String(), eventRef},
		[]any{&row.generation, &row.provenance},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return eventWriterGeneration{}, false, nil
	}
	if err != nil {
		return eventWriterGeneration{}, false, fmt.Errorf(
			"load typed-memory event writer generation: %w",
			err,
		)
	}
	return row, true, nil
}

func loadDurableV46AdmissionRow(
	ctx context.Context,
	source scanner,
	project projectledger.ProjectID,
	eventRef string,
) (durableV46AdmissionRow, bool, error) {
	row := durableV46AdmissionRow{}
	err := source.ScanOne(
		ctx,
		`SELECT event_digest, admission_basis_kind, type_env_ref,
			basis_graph_revision, request_digest, canonical_request_bytes,
			semantic_digest, canonical_semantic_bytes,
			admission_envelope_digest, canonical_admission_envelope_bytes,
			admission_basis_digest, canonical_admission_basis_bytes,
			materialization_manifest_digest,
			canonical_materialization_manifest_bytes, recorded_at
		FROM typed_memory_event_admission_bases
		WHERE project_id = ? AND event_ref = ?`,
		[]any{project.String(), eventRef},
		[]any{
			&row.eventDigest,
			&row.basisKind,
			&row.typeEnvRef,
			&row.basisRevision,
			&row.requestDigest,
			&row.requestBytes,
			&row.semanticDigest,
			&row.semanticBytes,
			&row.envelopeDigest,
			&row.envelopeBytes,
			&row.basisDigest,
			&row.basisBytes,
			&row.manifestDigest,
			&row.manifestBytes,
			&row.recordedAt,
		},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return durableV46AdmissionRow{}, false, nil
	}
	if err != nil {
		return durableV46AdmissionRow{}, false, fmt.Errorf("load v46 typed-memory admission basis: %w", err)
	}
	return row, true, nil
}

func loadDurableV46ClosureRow(
	ctx context.Context,
	source scanner,
	project projectledger.ProjectID,
	eventRef string,
) (durableV46ClosureRow, bool, error) {
	row := durableV46ClosureRow{}
	err := source.ScanOne(
		ctx,
		`SELECT commit_ref, event_digest, admission_basis_kind,
			request_digest, semantic_digest, admission_envelope_digest,
			admission_basis_digest, materialization_manifest_digest,
			materialization_digest,
			canonical_materialization_bytes, entity_count,
			entity_context_count, entity_declaration_count,
			context_slice_catalog_count, context_slice_count, value_blob_count,
			observable_input_blob_count, relation_count, relation_slot_count,
			relation_filler_count, ordered_candidate_prefix_count,
			reference_resolution_use_count,
			memberof_evaluation_count, memberof_input_count,
			memberof_use_count,
			kind_classification_source_blob_count,
			kind_classification_evaluation_count,
			kind_classification_feature_count,
			kind_classification_use_count,
			alias_change_count, retraction_count, recorded_at
		FROM typed_memory_commit_materialization_closures
		WHERE project_id = ? AND event_ref = ?`,
		[]any{project.String(), eventRef},
		[]any{
			&row.commitRef,
			&row.eventDigest,
			&row.basisKind,
			&row.requestDigest,
			&row.semanticDigest,
			&row.envelopeDigest,
			&row.basisDigest,
			&row.manifestDigest,
			&row.materializationDigest,
			&row.materializationBytes,
			&row.entityCount,
			&row.entityContextCount,
			&row.entityDeclarationCount,
			&row.contextSliceCatalogCount,
			&row.contextSliceCount,
			&row.valueBlobCount,
			&row.observableInputBlobCount,
			&row.relationCount,
			&row.relationSlotCount,
			&row.relationFillerCount,
			&row.orderedCandidatePrefixCount,
			&row.referenceResolutionCount,
			&row.memberOfEvaluationCount,
			&row.memberOfInputCount,
			&row.memberOfUseCount,
			&row.kindClassificationSourceBlobCount,
			&row.kindClassificationEvaluationCount,
			&row.kindClassificationFeatureCount,
			&row.kindClassificationUseCount,
			&row.aliasChangeCount,
			&row.retractionCount,
			&row.recordedAt,
		},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return durableV46ClosureRow{}, false, nil
	}
	if err != nil {
		return durableV46ClosureRow{}, false, fmt.Errorf("load v46 typed-memory materialization closure: %w", err)
	}
	return row, true, nil
}

func loadDurableV46FootprintRow(
	ctx context.Context,
	source scanner,
	project projectledger.ProjectID,
	eventRef string,
) (durableV46FootprintRow, error) {
	row := durableV46FootprintRow{}
	err := source.ScanOne(
		ctx,
		`SELECT entity_count, entity_context_count, entity_declaration_count,
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
			retraction_count, top_level_change_count
		FROM typed_memory_event_materialization_footprints_v46
		WHERE project_id = ? AND event_ref = ?`,
		[]any{project.String(), eventRef},
		[]any{
			&row.footprint.entityCount,
			&row.footprint.entityContextCount,
			&row.footprint.entityDeclarationCount,
			&row.footprint.contextSliceCatalogCount,
			&row.footprint.contextSliceCount,
			&row.footprint.valueBlobCount,
			&row.footprint.observableInputBlobCount,
			&row.footprint.relationCount,
			&row.footprint.relationSlotCount,
			&row.footprint.relationFillerCount,
			&row.footprint.orderedCandidatePrefixCount,
			&row.footprint.referenceResolutionCount,
			&row.footprint.memberOfEvaluationCount,
			&row.footprint.memberOfInputCount,
			&row.footprint.memberOfUseCount,
			&row.footprint.kindClassificationSourceBlobCount,
			&row.footprint.kindClassificationEvaluationCount,
			&row.footprint.kindClassificationFeatureCount,
			&row.footprint.kindClassificationUseCount,
			&row.footprint.aliasChangeCount,
			&row.footprint.retractionCount,
			&row.topLevelChangeCount,
		},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return durableV46FootprintRow{}, storedAdmissionIntegrity(
			"v46 typed-memory materialization footprint is missing",
			nil,
		)
	}
	if err != nil {
		return durableV46FootprintRow{}, fmt.Errorf("load v46 typed-memory materialization footprint: %w", err)
	}
	return row, nil
}

func loadDurableV46RowDigests(
	ctx context.Context,
	source scanner,
	project projectledger.ProjectID,
	eventRef string,
	prepared preparedAdmission,
	version AdmissionContractVersion,
) ([]string, error) {
	rowDigests, err := loadStoredV46DigestRows(ctx, source, project, eventRef)
	if err != nil {
		return nil, err
	}
	typedValueDigests, err := loadStoredV46TypedValueDigests(
		ctx,
		source,
		project,
		eventRef,
		prepared,
	)
	if err != nil {
		return nil, err
	}
	entityDeclarationDigests, err := loadStoredV46EntityDeclarationDigests(
		ctx,
		source,
		project,
		eventRef,
		prepared,
	)
	if err != nil {
		return nil, err
	}
	orderedPrefixDigests, err := loadStoredV46OrderedPrefixDigests(
		ctx,
		source,
		project,
		eventRef,
		prepared,
	)
	if err != nil {
		return nil, err
	}
	if err := verifyStoredV46MemberOfInputCoordinates(
		ctx,
		source,
		project,
		eventRef,
		prepared,
	); err != nil {
		return nil, err
	}
	if err := verifyStoredV46MemberOfEvaluationProjections(
		ctx,
		source,
		project,
		eventRef,
		prepared,
	); err != nil {
		return nil, err
	}
	if err := verifyStoredV46DisjointEntailmentProjections(
		ctx,
		source,
		project,
		eventRef,
		prepared,
		version,
	); err != nil {
		return nil, err
	}
	entityContextDigests, err := loadStoredV46EntityContextDigests(
		ctx,
		source,
		project,
		eventRef,
	)
	if err != nil {
		return nil, err
	}
	rowDigests = append(rowDigests, typedValueDigests...)
	rowDigests = append(rowDigests, entityDeclarationDigests...)
	rowDigests = append(rowDigests, orderedPrefixDigests...)
	rowDigests = append(rowDigests, entityContextDigests...)
	sort.Strings(rowDigests)
	return rowDigests, nil
}

func loadStoredV46DigestRows(
	ctx context.Context,
	source scanner,
	project projectledger.ProjectID,
	eventRef string,
) ([]string, error) {
	var count int64
	var joined string
	err := source.ScanOne(
		ctx,
		`SELECT COUNT(*), COALESCE(group_concat(encoded_row, char(10)), '')
		FROM (
			SELECT 'observable:' || observable_input_digest || ',' ||
				hex(canonical_observable_input_bytes) AS encoded_row
			FROM typed_memory_observable_input_blobs
			WHERE project_id = ? AND event_ref = ?
			UNION ALL
			SELECT 'alias:' || alias_change_digest || ',' ||
				hex(canonical_alias_change_bytes)
			FROM typed_memory_alias_changes
			WHERE project_id = ? AND event_ref = ?
			UNION ALL
			SELECT 'relation:' || relation_digest || ',' ||
				hex(canonical_relation_bytes)
			FROM typed_memory_relation_instances
			WHERE project_id = ? AND event_ref = ?
			UNION ALL
			SELECT 'relational-assertion-v3:' || assertion_digest || ',' ||
				hex(canonical_assertion_bytes)
			FROM typed_memory_relational_assertions_v3
			WHERE project_id = ? AND event_ref = ?
			UNION ALL
			SELECT 'context-slice:' || context_slice_digest || ',' ||
				hex(canonical_context_slice_bytes)
			FROM typed_memory_context_slices
			WHERE project_id = ? AND event_ref = ?
			UNION ALL
			SELECT 'context-slice-catalog:' || context_slice_digest || ',' ||
				hex(canonical_context_slice_bytes)
			FROM typed_memory_context_slice_catalog
			WHERE project_id = ? AND event_ref = ?
			UNION ALL
			SELECT 'slot:' || slot_digest || ',' || hex(canonical_slot_bytes)
			FROM typed_memory_relation_slots
			WHERE project_id = ? AND event_ref = ?
			UNION ALL
			SELECT 'relational-assertion-slot-v3:' || slot_digest || ',' ||
				hex(canonical_slot_bytes)
			FROM typed_memory_relational_assertion_slots_v3
			WHERE project_id = ? AND event_ref = ?
			UNION ALL
			SELECT 'filler:' || filler_digest || ',' || hex(canonical_filler_bytes)
			FROM typed_memory_relation_fillers
			WHERE project_id = ? AND event_ref = ?
			UNION ALL
			SELECT 'relational-assertion-filler-v3:' || filler_digest || ',' ||
				hex(canonical_filler_bytes)
			FROM typed_memory_relational_assertion_fillers_v3
			WHERE project_id = ? AND event_ref = ?
			UNION ALL
			SELECT 'resolution:' || resolution_digest || ',' ||
				hex(canonical_resolution_bytes)
			FROM typed_memory_reference_resolution_uses
			WHERE project_id = ? AND event_ref = ?
			UNION ALL
			SELECT 'relational-assertion-resolution-v3:' || resolution_digest || ',' ||
				hex(canonical_resolution_bytes)
			FROM typed_memory_relational_assertion_reference_resolution_uses_v3
			WHERE project_id = ? AND event_ref = ?
			UNION ALL
			SELECT 'memberof:' || judgement_digest || ',' ||
				hex(canonical_judgement_bytes)
			FROM typed_memory_memberof_evaluations
			WHERE project_id = ? AND event_ref = ?
			UNION ALL
			SELECT 'memberof-use:' || use_digest || ',' || hex(canonical_use_bytes)
			FROM typed_memory_relation_filler_memberof_uses
			WHERE project_id = ? AND event_ref = ?
			UNION ALL
			SELECT 'relational-assertion-memberof-use-v3:' || use_digest || ',' ||
				hex(canonical_use_bytes)
			FROM typed_memory_relational_assertion_memberof_uses_v3
			WHERE project_id = ? AND event_ref = ?
			UNION ALL
			SELECT 'disjoint-entailment-use:' || use_digest || ',' ||
				hex(canonical_use_bytes)
			FROM typed_memory_relation_filler_disjoint_entailment_uses
			WHERE project_id = ? AND event_ref = ?
			UNION ALL
			SELECT 'relational-assertion-disjointness-use-v3:' || use_digest || ',' ||
				hex(canonical_use_bytes)
			FROM typed_memory_relational_assertion_disjointness_uses_v3
			WHERE project_id = ? AND event_ref = ?
			UNION ALL
			SELECT 'kind-classification-source-v54:' || source_digest || ',' ||
				hex(canonical_source_bytes)
			FROM typed_memory_kind_classification_source_blobs_v54
			WHERE project_id = ? AND event_ref = ?
			UNION ALL
			SELECT 'kind-classification-evaluation-v54:' || judgement_digest || ',' ||
				hex(canonical_judgement_bytes)
			FROM typed_memory_kind_classification_evaluations_v54
			WHERE project_id = ? AND event_ref = ?
			UNION ALL
			SELECT 'kind-classification-feature-v54:' || feature_digest || ',' ||
				hex(canonical_feature_bytes)
			FROM typed_memory_kind_classification_features_v54
			WHERE project_id = ? AND event_ref = ?
			UNION ALL
			SELECT 'relational-assertion-classification-use-v54:' || use_digest || ',' ||
				hex(canonical_use_bytes)
			FROM typed_memory_relational_assertion_classification_uses_v54
			WHERE project_id = ? AND event_ref = ?
			UNION ALL
			SELECT 'retraction:' || retraction_digest || ',' ||
				hex(canonical_retraction_bytes)
			FROM typed_memory_assertion_retractions
			WHERE project_id = ? AND event_ref = ?
		)`,
		repeatedProjectEventArguments(project, eventRef, 22),
		[]any{&count, &joined},
	)
	if err != nil {
		return nil, fmt.Errorf("load v46 typed-memory row digests: %w", err)
	}
	encodedRows := splitDurableAggregate(joined)
	if int64(len(encodedRows)) != count {
		return nil, storedAdmissionIntegrity(
			"v46 typed-memory row digest aggregate count",
			nil,
		)
	}
	rowDigests := make([]string, 0, len(encodedRows))
	for _, encodedRow := range encodedRows {
		rowDigest, err := decodeStoredV46DigestRow(encodedRow)
		if err != nil {
			return nil, err
		}
		rowDigests = append(rowDigests, rowDigest)
	}
	return rowDigests, nil
}

func decodeStoredV46DigestRow(encodedRow string) (string, error) {
	separator := strings.LastIndex(encodedRow, ",")
	if separator <= 0 || separator == len(encodedRow)-1 {
		return "", storedAdmissionIntegrity(
			"v46 typed-memory canonical row aggregate shape",
			nil,
		)
	}
	rowDigest := encodedRow[:separator]
	if err := validateDurableV46RowDigest(rowDigest); err != nil {
		return "", storedAdmissionIntegrity("v46 typed-memory row digest", err)
	}
	canonicalBytes, err := hex.DecodeString(encodedRow[separator+1:])
	if err != nil {
		return "", storedAdmissionIntegrity("v46 typed-memory canonical row bytes", err)
	}
	storedDigest := rowDigest[strings.LastIndex(rowDigest, ":sha256:")+1:]
	recomputedDigest, err := digestBytes(canonicalBytes)
	if err != nil {
		return "", storedAdmissionIntegrity("v46 typed-memory canonical row digest", err)
	}
	if recomputedDigest.String() != storedDigest {
		return "", storedAdmissionIntegrity(
			"v46 typed-memory canonical row digest mismatch",
			nil,
		)
	}
	return rowDigest, nil
}

func repeatedProjectEventArguments(
	project projectledger.ProjectID,
	eventRef string,
	count int,
) []any {
	arguments := make([]any, 0, count*2)
	for range count {
		arguments = append(arguments, project.String(), eventRef)
	}
	return arguments
}

func splitDurableAggregate(joined string) []string {
	if joined == "" {
		return nil
	}
	return strings.Split(joined, "\n")
}

func loadStoredV46TypedValueDigests(
	ctx context.Context,
	source scanner,
	project projectledger.ProjectID,
	eventRef string,
	_ preparedAdmission,
) ([]string, error) {
	var count int64
	var joined string
	err := source.ScanOne(
		ctx,
		`SELECT COUNT(*), COALESCE(group_concat(encoded_row, '|'), '')
		FROM (
			SELECT hex(value_digest) || ',' || hex(value_kind_ref) || ',' ||
				hex(value_shape_ref) || ',' || hex(codec_ref) || ',' ||
				hex(canonical_value_bytes) AS encoded_row
			FROM typed_memory_value_blobs
			WHERE project_id = ? AND event_ref = ?
		)`,
		[]any{project.String(), eventRef},
		[]any{&count, &joined},
	)
	if err != nil {
		return nil, fmt.Errorf("load v46 typed-memory value rows: %w", err)
	}
	rows := splitEntityContextAggregate(joined)
	if int64(len(rows)) != count {
		return nil, storedAdmissionIntegrity(
			"v46 typed-memory value-row aggregate count",
			nil,
		)
	}
	digests := make([]string, 0, len(rows))
	for _, encoded := range rows {
		fields := strings.Split(encoded, ",")
		if len(fields) != 5 {
			return nil, storedAdmissionIntegrity(
				"v46 typed-memory value-row aggregate shape",
				nil,
			)
		}
		decoded := make([][]byte, 0, len(fields))
		for _, field := range fields {
			value, decodeErr := hex.DecodeString(field)
			if decodeErr != nil {
				return nil, storedAdmissionIntegrity(
					"v46 typed-memory value-row field",
					decodeErr,
				)
			}
			decoded = append(decoded, value)
		}
		storedDigest, digestErr := typedmemory.NewSHA256Digest(string(decoded[0]))
		if digestErr != nil {
			return nil, storedAdmissionIntegrity(
				"v46 typed-memory value digest",
				digestErr,
			)
		}
		verifyErr := typedmemory.VerifyStoredTypedValueDigest(
			string(decoded[1]),
			string(decoded[2]),
			string(decoded[3]),
			decoded[4],
			storedDigest,
		)
		if verifyErr != nil {
			return nil, storedAdmissionIntegrity(
				"v46 typed-memory value-row envelope",
				verifyErr,
			)
		}
		rowDigest := "value:" + storedDigest.String()
		if validateErr := validateDurableV46RowDigest(rowDigest); validateErr != nil {
			return nil, storedAdmissionIntegrity(
				"v46 typed-memory value-row digest",
				validateErr,
			)
		}
		digests = append(digests, rowDigest)
	}
	return digests, nil
}

func loadStoredV46EntityDeclarationDigests(
	ctx context.Context,
	source scanner,
	project projectledger.ProjectID,
	eventRef string,
	prepared preparedAdmission,
) ([]string, error) {
	var count int64
	var joined string
	err := source.ScanOne(
		ctx,
		`SELECT COUNT(*), COALESCE(group_concat(encoded_row, '|'), '')
		FROM (
			SELECT CAST(change_ordinal AS TEXT) || ',' || hex(entity_id) || ',' ||
				hex(batch_local_ref) || ',' || hex(bounded_context_ref) || ',' ||
				hex(label) || ',' || hex(provenance_ref) || ',' ||
				hex(declaration_digest) || ',' || hex(canonical_declaration_bytes) AS encoded_row
			FROM typed_memory_entity_declarations
			WHERE project_id = ? AND event_ref = ?
		)`,
		[]any{project.String(), eventRef},
		[]any{&count, &joined},
	)
	if err != nil {
		return nil, fmt.Errorf("load v46 typed-memory entity declarations: %w", err)
	}
	rows := splitEntityContextAggregate(joined)
	if int64(len(rows)) != count {
		return nil, storedAdmissionIntegrity(
			"v46 typed-memory entity-declaration aggregate count",
			nil,
		)
	}
	candidateChanges := prepared.candidate.Changes()
	digests := make([]string, 0, len(rows))
	for _, encoded := range rows {
		fields := strings.Split(encoded, ",")
		if len(fields) != 8 {
			return nil, storedAdmissionIntegrity(
				"v46 typed-memory entity-declaration aggregate shape",
				nil,
			)
		}
		ordinal, ordinalErr := strconv.ParseUint(fields[0], 10, 64)
		if ordinalErr != nil || ordinal >= uint64(len(candidateChanges)) {
			return nil, storedAdmissionIntegrity(
				"v46 typed-memory entity-declaration ordinal",
				ordinalErr,
			)
		}
		candidate, declared := candidateChanges[ordinal].(typedmemory.DeclareEntity)
		if !declared {
			return nil, storedAdmissionIntegrity(
				"v46 typed-memory entity-declaration change kind",
				nil,
			)
		}
		decoded := make([][]byte, 0, len(fields)-1)
		for _, field := range fields[1:] {
			value, decodeErr := hex.DecodeString(field)
			if decodeErr != nil {
				return nil, storedAdmissionIntegrity(
					"v46 typed-memory entity-declaration field",
					decodeErr,
				)
			}
			decoded = append(decoded, value)
		}
		canonical, canonicalErr := candidate.CanonicalBytes()
		if canonicalErr != nil {
			return nil, canonicalErr
		}
		digest, digestErr := candidate.Digest()
		if digestErr != nil {
			return nil, digestErr
		}
		matches := string(decoded[0]) == candidate.Entity().String() &&
			string(decoded[1]) == candidate.LocalRef().String() &&
			string(decoded[2]) == candidate.Context().String() &&
			string(decoded[3]) == candidate.Label().String() &&
			string(decoded[4]) == candidate.Provenance().String() &&
			string(decoded[5]) == digest.String() &&
			bytes.Equal(decoded[6], canonical)
		if !matches {
			return nil, storedAdmissionIntegrity(
				"v46 typed-memory entity declaration does not match the sealed request",
				nil,
			)
		}
		rowDigest := "entity-declaration:" + digest.String()
		if validateErr := validateDurableV46RowDigest(rowDigest); validateErr != nil {
			return nil, storedAdmissionIntegrity(
				"v46 typed-memory entity-declaration digest",
				validateErr,
			)
		}
		digests = append(digests, rowDigest)
	}
	return digests, nil
}

func loadStoredV46OrderedPrefixDigests(
	ctx context.Context,
	source scanner,
	project projectledger.ProjectID,
	eventRef string,
	prepared preparedAdmission,
) ([]string, error) {
	var count int64
	var joined string
	err := source.ScanOne(
		ctx,
		`SELECT COUNT(*), COALESCE(group_concat(encoded_row, '|'), '')
		FROM (
			SELECT CAST(prefix_end_ordinal AS TEXT) || ',' ||
				hex(request_digest) || ',' || hex(prefix_digest) AS encoded_row
			FROM typed_memory_ordered_candidate_prefixes
			WHERE project_id = ? AND event_ref = ?
		)`,
		[]any{project.String(), eventRef},
		[]any{&count, &joined},
	)
	if err != nil {
		return nil, fmt.Errorf("load v46 typed-memory ordered prefixes: %w", err)
	}
	rows := splitEntityContextAggregate(joined)
	if int64(len(rows)) != count {
		return nil, storedAdmissionIntegrity(
			"v46 typed-memory ordered-prefix aggregate count",
			nil,
		)
	}
	digests := make([]string, 0, len(rows))
	for _, encoded := range rows {
		fields := strings.Split(encoded, ",")
		if len(fields) != 3 {
			return nil, storedAdmissionIntegrity(
				"v46 typed-memory ordered-prefix aggregate shape",
				nil,
			)
		}
		endExclusive, ordinalErr := strconv.ParseUint(fields[0], 10, 64)
		if ordinalErr != nil {
			return nil, storedAdmissionIntegrity(
				"v46 typed-memory ordered-prefix end ordinal",
				ordinalErr,
			)
		}
		requestDigest, requestErr := hex.DecodeString(fields[1])
		if requestErr != nil || string(requestDigest) != prepared.requestDigest.String() {
			return nil, storedAdmissionIntegrity(
				"v46 typed-memory ordered prefix does not bind the sealed request",
				requestErr,
			)
		}
		storedDigestBytes, digestDecodeErr := hex.DecodeString(fields[2])
		if digestDecodeErr != nil {
			return nil, storedAdmissionIntegrity(
				"v46 typed-memory ordered-prefix digest bytes",
				digestDecodeErr,
			)
		}
		storedDigest, digestErr := typedmemory.NewSHA256Digest(string(storedDigestBytes))
		if digestErr != nil {
			return nil, storedAdmissionIntegrity(
				"v46 typed-memory ordered-prefix digest",
				digestErr,
			)
		}
		prefix, prefixErr := typedmemory.ComputeOrderedCandidatePrefix(
			prepared.candidate,
			endExclusive,
		)
		if prefixErr != nil || prefix.Digest() != storedDigest {
			return nil, storedAdmissionIntegrity(
				"v46 typed-memory ordered prefix does not match the sealed request",
				prefixErr,
			)
		}
		rowDigest := "ordered-prefix:" + storedDigest.String()
		if validateErr := validateDurableV46RowDigest(rowDigest); validateErr != nil {
			return nil, storedAdmissionIntegrity(
				"v46 typed-memory ordered-prefix row digest",
				validateErr,
			)
		}
		digests = append(digests, rowDigest)
	}
	return digests, nil
}

func verifyStoredV46MemberOfInputCoordinates(
	ctx context.Context,
	source scanner,
	project projectledger.ProjectID,
	eventRef string,
	prepared preparedAdmission,
) error {
	expected, err := expectedMemberOfInputCoordinates(prepared)
	if err != nil {
		return err
	}
	var count int64
	var joined string
	err = source.ScanOne(
		ctx,
		`SELECT COUNT(*), COALESCE(group_concat(encoded_row, '|'), '')
		FROM (
			SELECT hex(evaluation_ref) || ',' || CAST(input_ordinal AS TEXT) || ',' ||
				hex(observable_input_ref) || ',' || hex(observable_input_digest) AS encoded_row
			FROM typed_memory_memberof_observable_inputs
			WHERE project_id = ? AND event_ref = ?
		)`,
		[]any{project.String(), eventRef},
		[]any{&count, &joined},
	)
	if err != nil {
		return fmt.Errorf("load v46 typed-memory MemberOf input coordinates: %w", err)
	}
	rows := splitEntityContextAggregate(joined)
	if int64(len(rows)) != count {
		return storedAdmissionIntegrity(
			"v46 typed-memory MemberOf input-coordinate aggregate count",
			nil,
		)
	}
	actual := make([]string, 0, len(rows))
	for _, encoded := range rows {
		fields := strings.Split(encoded, ",")
		if len(fields) != 4 {
			return storedAdmissionIntegrity(
				"v46 typed-memory MemberOf input-coordinate shape",
				nil,
			)
		}
		evaluationRef, evaluationErr := hex.DecodeString(fields[0])
		if evaluationErr != nil {
			return storedAdmissionIntegrity(
				"v46 typed-memory MemberOf evaluation ref",
				evaluationErr,
			)
		}
		ordinal, ordinalErr := strconv.ParseUint(fields[1], 10, 64)
		if ordinalErr != nil {
			return storedAdmissionIntegrity(
				"v46 typed-memory MemberOf input ordinal",
				ordinalErr,
			)
		}
		observableRef, referenceErr := hex.DecodeString(fields[2])
		if referenceErr != nil {
			return storedAdmissionIntegrity(
				"v46 typed-memory MemberOf observable ref",
				referenceErr,
			)
		}
		observableDigest, digestErr := hex.DecodeString(fields[3])
		if digestErr != nil {
			return storedAdmissionIntegrity(
				"v46 typed-memory MemberOf observable digest",
				digestErr,
			)
		}
		actual = append(actual, materializationCoordinateKey(
			"typed-memory-memberof-input-coordinate.v1",
			string(evaluationRef),
			strconv.FormatUint(ordinal, 10),
			string(observableRef),
			string(observableDigest),
		))
	}
	sort.Strings(expected)
	sort.Strings(actual)
	if len(expected) != len(actual) {
		return storedAdmissionIntegrity("MemberOf observable-input coordinate count", nil)
	}
	for index := range expected {
		if expected[index] != actual[index] {
			return storedAdmissionIntegrity("MemberOf observable-input coordinates", nil)
		}
	}
	return nil
}

func expectedMemberOfInputCoordinates(
	prepared preparedAdmission,
) ([]string, error) {
	basis, membership := prepared.basis.(typedmemory.ContextSliceMembershipBasis)
	if !membership {
		return nil, nil
	}
	evaluations := make(map[string]typedmemory.DefinedMemberOfJudgement)
	for _, use := range basis.ReferenceFillerAdmissionUses() {
		required := typedmemory.DefinedMemberOfJudgement(use.RequiredMembership())
		evaluations[required.Digest().String()] = required
		for _, disjoint := range use.DisjointMemberships() {
			direct, ok := disjoint.(typedmemory.DirectNotMemberUse)
			if !ok {
				continue
			}
			judgement := typedmemory.DefinedMemberOfJudgement(direct.Judgement())
			evaluations[judgement.Digest().String()] = judgement
		}
	}
	coordinates := make([]string, 0)
	for digest, judgement := range evaluations {
		evaluationRef := derivedRef("typed-memory-memberof-evaluation", digest)
		for ordinal, observable := range judgement.Basis().ObservableInputs() {
			coordinates = append(coordinates, materializationCoordinateKey(
				"typed-memory-memberof-input-coordinate.v1",
				evaluationRef,
				strconv.Itoa(ordinal),
				observable.Reference().String(),
				observable.Digest().String(),
			))
		}
	}
	return coordinates, nil
}

func materializationCoordinateKey(domain string, fields ...string) string {
	return string(canonicalStorageFields(domain, fields))
}

func verifyStoredV46MemberOfEvaluationProjections(
	ctx context.Context,
	source scanner,
	project projectledger.ProjectID,
	eventRef string,
	prepared preparedAdmission,
) error {
	expected, err := expectedMemberOfEvaluationProjectionRows(prepared)
	if err != nil {
		return err
	}
	var count int64
	var joined string
	err = source.ScanOne(
		ctx,
		`SELECT COUNT(*), COALESCE(group_concat(encoded_row, '|'), '')
		FROM (
			SELECT hex(evaluation_ref) || ',' || hex(judgement_kind) || ',' ||
				hex(entity_id) || ',' || hex(value_kind_ref) || ',' ||
				hex(context_slice_ref) || ',' || hex(evaluator_rule_ref) || ',' ||
				hex(evaluation_provenance_ref) || ',' || hex(evaluation_view_kind) || ',' ||
				hex(evaluation_view_digest) || ',' || hex(canonical_evaluation_view_bytes) || ',' ||
				CASE WHEN view_declaration_change_ordinal IS NULL THEN 'N'
					ELSE 'S' || hex(CAST(view_declaration_change_ordinal AS TEXT)) END || ',' ||
				CASE WHEN view_local_reference_kind_ref IS NULL THEN 'N'
					ELSE 'S' || hex(view_local_reference_kind_ref) END || ',' ||
				CASE WHEN view_batch_local_ref IS NULL THEN 'N'
					ELSE 'S' || hex(view_batch_local_ref) END || ',' ||
				CASE WHEN view_declaration_digest IS NULL THEN 'N'
					ELSE 'S' || hex(view_declaration_digest) END || ',' ||
				CASE WHEN view_prefix_end_ordinal IS NULL THEN 'N'
					ELSE 'S' || hex(CAST(view_prefix_end_ordinal AS TEXT)) END || ',' ||
				CASE WHEN view_ordered_candidate_prefix_digest IS NULL THEN 'N'
					ELSE 'S' || hex(view_ordered_candidate_prefix_digest) END || ',' ||
				hex(CAST(observable_input_count AS TEXT)) || ',' ||
				hex(observable_input_set_digest) || ',' || hex(query_digest) || ',' ||
				hex(canonical_query_bytes) || ',' || hex(basis_digest) || ',' ||
				hex(canonical_basis_bytes) || ',' || hex(judgement_digest) || ',' ||
				hex(canonical_judgement_bytes) AS encoded_row
			FROM typed_memory_memberof_evaluations
			WHERE project_id = ? AND event_ref = ?
		)`,
		[]any{project.String(), eventRef},
		[]any{&count, &joined},
	)
	if err != nil {
		return fmt.Errorf("load v46 typed-memory MemberOf evaluation projections: %w", err)
	}
	actual := splitEntityContextAggregate(joined)
	if int64(len(actual)) != count {
		return storedAdmissionIntegrity(
			"v46 typed-memory MemberOf evaluation aggregate count",
			nil,
		)
	}
	sort.Strings(expected)
	sort.Strings(actual)
	if len(expected) != len(actual) {
		return storedAdmissionIntegrity("MemberOf evaluation projection count", nil)
	}
	for index := range expected {
		if expected[index] != actual[index] {
			return storedAdmissionIntegrity("MemberOf evaluation projections", nil)
		}
	}
	return nil
}

func expectedMemberOfEvaluationProjectionRows(
	prepared preparedAdmission,
) ([]string, error) {
	judgements := expectedDefinedMemberOfJudgements(prepared)
	rows := make([]string, 0, len(judgements))
	for _, judgement := range judgements {
		basis := judgement.Basis()
		query := judgement.Query()
		view := judgement.EvaluationView()
		inputSetDigest, err := typedmemory.ComputeMemberOfObservableInputSetDigest(
			basis.ObservableInputs(),
		)
		if err != nil {
			return nil, err
		}
		viewDeclarationOrdinal := projectionNone()
		viewLocalReferenceKind := projectionNone()
		viewBatchLocalReference := projectionNone()
		viewDeclarationDigest := projectionNone()
		viewPrefixEndOrdinal := projectionNone()
		viewOrderedPrefixDigest := projectionNone()
		if prospective, ok := view.(typedmemory.ProspectiveBatchView); ok {
			viewDeclarationOrdinal = projectionSomeString(
				strconv.FormatUint(prospective.DeclarationChangeOrdinal(), 10),
			)
			viewLocalReferenceKind = projectionSomeString(
				prospective.LocalReference().RefKind().String(),
			)
			viewBatchLocalReference = projectionSomeString(
				prospective.LocalReference().BatchLocalRef().String(),
			)
			viewDeclarationDigest = projectionSomeString(
				prospective.DeclarationDigest().String(),
			)
			viewPrefixEndOrdinal = projectionSomeString(
				strconv.FormatUint(prospective.EvaluationChangeOrdinal(), 10),
			)
			viewOrderedPrefixDigest = projectionSomeString(
				prospective.OrderedCandidatePrefix().Digest().String(),
			)
		}
		fields := []string{
			projectionHexString(derivedRef("typed-memory-memberof-evaluation", judgement.Digest().String())),
			projectionHexString(judgement.Kind().String()),
			projectionHexString(query.EntityID().String()),
			projectionHexString(query.ValueKind().String()),
			projectionHexString(query.ContextSlice().Ref().String()),
			projectionHexString(basis.Evaluator().String()),
			projectionHexString(basis.EvaluationProvenance().Reference().String()),
			projectionHexString(view.Kind().String()),
			projectionHexString(view.Digest().String()),
			projectionHexBytes(view.CanonicalBytes()),
			viewDeclarationOrdinal,
			viewLocalReferenceKind,
			viewBatchLocalReference,
			viewDeclarationDigest,
			viewPrefixEndOrdinal,
			viewOrderedPrefixDigest,
			projectionHexString(strconv.Itoa(len(basis.ObservableInputs()))),
			projectionHexString(inputSetDigest.String()),
			projectionHexString(query.Digest().String()),
			projectionHexBytes(query.CanonicalBytes()),
			projectionHexString(basis.Digest().String()),
			projectionHexBytes(basis.CanonicalBytes()),
			projectionHexString(judgement.Digest().String()),
			projectionHexBytes(judgement.CanonicalBytes()),
		}
		rows = append(rows, strings.Join(fields, ","))
	}
	return rows, nil
}

func verifyStoredV46DisjointEntailmentProjections(
	ctx context.Context,
	source scanner,
	project projectledger.ProjectID,
	eventRef string,
	prepared preparedAdmission,
	version AdmissionContractVersion,
) error {
	expected, err := expectedDisjointEntailmentProjectionRows(prepared, version)
	if err != nil {
		return err
	}
	var count int64
	var joined string
	family, err := relationStorageFamilyForAdmissionContract(version)
	if err != nil {
		return err
	}
	table := family.disjointnessUseTable
	err = source.ScanOne(
		ctx,
		`SELECT COUNT(*), COALESCE(group_concat(encoded_row, '|'), '')
		FROM (
			SELECT hex(CAST(change_ordinal AS TEXT)) || ',' || hex(assertion_id) || ',' ||
				hex(CAST(slot_ordinal AS TEXT)) || ',' ||
				hex(CAST(filler_ordinal AS TEXT)) || ',' || hex(filler_digest) || ',' ||
				hex(constraint_id) || ',' || hex(constraint_digest) || ',' ||
				hex(canonical_constraint_bytes) || ',' ||
				hex(matched_operand_kind_id) || ',' || hex(excluded_operand_kind_id) || ',' ||
				hex(counter_value_kind_ref) || ',' || hex(counter_query_digest) || ',' ||
				hex(canonical_counter_query_bytes) || ',' ||
				hex(supporting_evaluation_ref) || ',' || hex(use_digest) || ',' ||
				hex(canonical_use_bytes) AS encoded_row
			FROM `+table+`
			WHERE project_id = ? AND event_ref = ?
		)`,
		[]any{project.String(), eventRef},
		[]any{&count, &joined},
	)
	if err != nil {
		return fmt.Errorf(
			"load v46 typed-memory disjoint-entailment projections: %w",
			err,
		)
	}
	actual := splitEntityContextAggregate(joined)
	if int64(len(actual)) != count {
		return storedAdmissionIntegrity(
			"v46 typed-memory disjoint-entailment aggregate count",
			nil,
		)
	}
	sort.Strings(expected)
	sort.Strings(actual)
	if len(expected) != len(actual) {
		return storedAdmissionIntegrity(
			"disjoint-entailment projection count",
			nil,
		)
	}
	for index := range expected {
		if expected[index] != actual[index] {
			return storedAdmissionIntegrity(
				"disjoint-entailment projections",
				nil,
			)
		}
	}
	return nil
}

type expectedDisjointEntailmentProjection struct {
	use      typedmemory.DisjointEntailmentUse
	required typedmemory.MemberOfMember
}

func expectedDisjointEntailmentProjectionRows(
	prepared preparedAdmission,
	version AdmissionContractVersion,
) ([]string, error) {
	// The prepared membership basis is the current exact-TypeEnv admission
	// result. Replay projects its sealed entailment and its already-required
	// positive MemberOf support; it must not invent a negative evaluation.
	membership, ok := prepared.basis.(typedmemory.ContextSliceMembershipBasis)
	if !ok {
		return nil, nil
	}
	manifest, err := buildExpectedMaterializationManifest(prepared)
	if err != nil {
		return nil, err
	}
	family, err := relationStorageFamilyForAdmissionContract(version)
	if err != nil {
		return nil, err
	}
	byDigest, expectedCount, err := indexExpectedDisjointEntailments(
		membership,
	)
	if err != nil {
		return nil, err
	}
	rows := make([]string, 0, expectedCount)
	for _, semantic := range manifest.semanticRows {
		if semantic.rowKind != family.disjointnessUseRowKind {
			continue
		}
		projection, takeErr := takeExpectedDisjointEntailment(
			byDigest,
			semantic,
		)
		if takeErr != nil {
			return nil, takeErr
		}
		row, rowErr := expectedDisjointEntailmentProjectionRow(
			prepared.basis.TypeEnv(),
			semantic,
			projection,
		)
		if rowErr != nil {
			return nil, rowErr
		}
		rows = append(rows, row)
	}
	if len(rows) != expectedCount || remainingExpectedDisjointEntailments(byDigest) != 0 {
		return nil, ErrInvalidAdmissionBatch
	}
	return rows, nil
}

func indexExpectedDisjointEntailments(
	membership typedmemory.ContextSliceMembershipBasis,
) (map[string][]expectedDisjointEntailmentProjection, int, error) {
	byDigest := make(map[string][]expectedDisjointEntailmentProjection)
	count := 0
	for _, admissionUse := range membership.ReferenceFillerAdmissionUses() {
		required := admissionUse.RequiredMembership()
		for _, disjoint := range admissionUse.DisjointMemberships() {
			entailment, ok := disjoint.(typedmemory.DisjointEntailmentUse)
			if !ok {
				continue
			}
			supporting := entailment.SupportingMembership()
			if supporting.Digest() != required.Digest() ||
				!bytes.Equal(supporting.CanonicalBytes(), required.CanonicalBytes()) {
				return nil, 0, ErrInvalidAdmissionBatch
			}
			key := entailment.Digest().String()
			byDigest[key] = append(
				byDigest[key],
				expectedDisjointEntailmentProjection{
					use:      entailment,
					required: required,
				},
			)
			count++
		}
	}
	return byDigest, count, nil
}

func takeExpectedDisjointEntailment(
	byDigest map[string][]expectedDisjointEntailmentProjection,
	semantic expectedSemanticRowIdentity,
) (expectedDisjointEntailmentProjection, error) {
	values := byDigest[semantic.semanticDigest.String()]
	for index, value := range values {
		if !bytes.Equal(value.use.CanonicalBytes(), semantic.semanticBytes) {
			continue
		}
		remaining := make(
			[]expectedDisjointEntailmentProjection,
			0,
			len(values)-1,
		)
		remaining = append(remaining, values[:index]...)
		remaining = append(remaining, values[index+1:]...)
		byDigest[semantic.semanticDigest.String()] = remaining
		return value, nil
	}
	return expectedDisjointEntailmentProjection{}, ErrInvalidAdmissionBatch
}

func remainingExpectedDisjointEntailments(
	byDigest map[string][]expectedDisjointEntailmentProjection,
) int {
	count := 0
	for _, values := range byDigest {
		count += len(values)
	}
	return count
}

func expectedDisjointEntailmentProjectionRow(
	typeEnv typedmemory.TypeEnvRef,
	semantic expectedSemanticRowIdentity,
	projection expectedDisjointEntailmentProjection,
) (string, error) {
	if len(semantic.coordinate) != 14 {
		return "", ErrInvalidAdmissionBatch
	}
	entailment := projection.use
	required := projection.required
	counterQuery := entailment.CounterQuery()
	supporting := entailment.SupportingMembership()
	if required.Query().ValueKind().TypeEnv() != typeEnv ||
		required.EvaluationView().TypeEnv() != typeEnv ||
		supporting.Query().ValueKind().TypeEnv() != typeEnv ||
		supporting.EvaluationView().TypeEnv() != typeEnv ||
		counterQuery.ValueKind().TypeEnv() != typeEnv {
		return "", ErrInvalidAdmissionBatch
	}
	constraint := entailment.ConstraintRule()
	supportingEvaluationRef := derivedRef(
		"typed-memory-memberof-evaluation",
		supporting.Digest().String(),
	)
	wantCoordinate := []string{
		semantic.coordinate[0],
		semantic.coordinate[1],
		semantic.coordinate[2],
		semantic.coordinate[3],
		semantic.coordinate[4],
		constraint.ID().String(),
		entailment.ConstraintDigest().String(),
		string(constraint.CanonicalBytes()),
		entailment.MatchedOperand().String(),
		entailment.ExcludedOperand().String(),
		counterQuery.ValueKind().String(),
		counterQuery.Digest().String(),
		string(counterQuery.CanonicalBytes()),
		supportingEvaluationRef,
	}
	if !slices.Equal(semantic.coordinate, wantCoordinate) {
		return "", ErrInvalidAdmissionBatch
	}
	fields := []string{
		projectionHexString(semantic.coordinate[0]),
		projectionHexString(semantic.coordinate[1]),
		projectionHexString(semantic.coordinate[2]),
		projectionHexString(semantic.coordinate[3]),
		projectionHexString(semantic.coordinate[4]),
		projectionHexString(constraint.ID().String()),
		projectionHexString(entailment.ConstraintDigest().String()),
		projectionHexBytes(constraint.CanonicalBytes()),
		projectionHexString(entailment.MatchedOperand().String()),
		projectionHexString(entailment.ExcludedOperand().String()),
		projectionHexString(counterQuery.ValueKind().String()),
		projectionHexString(counterQuery.Digest().String()),
		projectionHexBytes(counterQuery.CanonicalBytes()),
		projectionHexString(supportingEvaluationRef),
		projectionHexString(entailment.Digest().String()),
		projectionHexBytes(entailment.CanonicalBytes()),
	}
	return strings.Join(fields, ","), nil
}

func expectedDefinedMemberOfJudgements(
	prepared preparedAdmission,
) map[string]typedmemory.DefinedMemberOfJudgement {
	basis, membership := prepared.basis.(typedmemory.ContextSliceMembershipBasis)
	if !membership {
		return nil
	}
	judgements := make(map[string]typedmemory.DefinedMemberOfJudgement)
	for _, use := range basis.ReferenceFillerAdmissionUses() {
		required := typedmemory.DefinedMemberOfJudgement(use.RequiredMembership())
		judgements[required.Digest().String()] = required
		for _, disjoint := range use.DisjointMemberships() {
			direct, ok := disjoint.(typedmemory.DirectNotMemberUse)
			if !ok {
				continue
			}
			judgement := typedmemory.DefinedMemberOfJudgement(direct.Judgement())
			judgements[judgement.Digest().String()] = judgement
		}
	}
	return judgements
}

func projectionHexString(value string) string {
	return projectionHexBytes([]byte(value))
}

func projectionHexBytes(value []byte) string {
	return strings.ToUpper(hex.EncodeToString(value))
}

func projectionNone() string { return "N" }

func projectionSomeString(value string) string {
	return "S" + projectionHexString(value)
}

func loadStoredV46EntityContextDigests(
	ctx context.Context,
	source scanner,
	project projectledger.ProjectID,
	eventRef string,
) ([]string, error) {
	var count int64
	var joined string
	err := source.ScanOne(
		ctx,
		`SELECT COUNT(*), COALESCE(group_concat(encoded_row, '|'), '')
		FROM (
			SELECT hex(entity_id) || ',' || hex(bounded_context_ref) || ',' ||
				hex(label) || ',' || hex(provenance_ref) AS encoded_row
			FROM typed_memory_entity_contexts
			WHERE project_id = ? AND declared_event_ref = ?
		)`,
		[]any{project.String(), eventRef},
		[]any{&count, &joined},
	)
	if err != nil {
		return nil, fmt.Errorf("load v46 typed-memory entity-context digest inputs: %w", err)
	}
	rows := splitEntityContextAggregate(joined)
	if int64(len(rows)) != count {
		return nil, storedAdmissionIntegrity(
			"v46 typed-memory entity-context aggregate count",
			nil,
		)
	}
	digests := make([]string, 0, len(rows))
	for _, encoded := range rows {
		fields := strings.Split(encoded, ",")
		if len(fields) != 4 {
			return nil, storedAdmissionIntegrity(
				"v46 typed-memory entity-context aggregate shape",
				nil,
			)
		}
		decoded := make([]string, 0, len(fields))
		for _, field := range fields {
			value, err := hex.DecodeString(field)
			if err != nil {
				return nil, storedAdmissionIntegrity(
					"v46 typed-memory entity-context field",
					err,
				)
			}
			decoded = append(decoded, string(value))
		}
		digest, err := digestFields("typed-memory-entity-context-row.v1", decoded...)
		if err != nil {
			return nil, storedAdmissionIntegrity(
				"v46 typed-memory entity-context digest",
				err,
			)
		}
		digests = append(digests, "entity-context:"+digest.String())
	}
	for _, rowDigest := range digests {
		if err := validateDurableV46RowDigest(rowDigest); err != nil {
			return nil, storedAdmissionIntegrity(
				"v46 typed-memory entity-context row digest",
				err,
			)
		}
	}
	return digests, nil
}

func splitEntityContextAggregate(joined string) []string {
	if joined == "" {
		return nil
	}
	return strings.Split(joined, "|")
}

func validateDurableV46RowDigest(rowDigest string) error {
	separator := strings.LastIndex(rowDigest, ":sha256:")
	if separator <= 0 {
		return fmt.Errorf("v46 typed-memory row digest is malformed")
	}
	prefix := rowDigest[:separator]
	allowed := map[string]struct{}{
		"observable":                           {},
		"entity-context":                       {},
		"entity-declaration":                   {},
		"alias":                                {},
		"relation":                             {},
		"context-slice":                        {},
		"context-slice-catalog":                {},
		"slot":                                 {},
		"filler":                               {},
		"value":                                {},
		"resolution":                           {},
		"memberof":                             {},
		"memberof-use":                         {},
		"disjoint-entailment-use":              {},
		"retraction":                           {},
		"ordered-prefix":                       {},
		"relational-assertion-v3":              {},
		"relational-assertion-slot-v3":         {},
		"relational-assertion-filler-v3":       {},
		"relational-assertion-resolution-v3":   {},
		"relational-assertion-memberof-use-v3": {},
		"relational-assertion-disjointness-use-v3":    {},
		"kind-classification-source-v54":              {},
		"kind-classification-evaluation-v54":          {},
		"kind-classification-feature-v54":             {},
		"relational-assertion-classification-use-v54": {},
	}
	if _, exists := allowed[prefix]; !exists {
		return fmt.Errorf("v46 typed-memory row digest has an unknown kind")
	}
	_, err := typedmemory.NewSHA256Digest(rowDigest[separator+1:])
	if err != nil {
		return fmt.Errorf("v46 typed-memory row digest payload is malformed: %w", err)
	}
	return nil
}

func verifyDurableGenericAdmissionDetail(
	detail durableGenericAdmissionDetail,
	expectation durableGenericAdmissionExpectation,
	disposition CommitDisposition,
) (CommitReceipt, error) {
	if detail == nil {
		return CommitReceipt{}, ErrStorageGenerationUnavailable
	}
	switch exact := detail.(type) {
	case durableLegacyV45AdmissionDetail:
		return CommitReceipt{}, ErrStorageGenerationUnavailable
	case durableV46AdmissionDetail:
		return verifyDurableV46Admission(exact, expectation, disposition)
	default:
		return CommitReceipt{}, ErrStorageGenerationUnavailable
	}
}

func verifyDurableV46Admission(
	detail durableV46AdmissionDetail,
	expectation durableGenericAdmissionExpectation,
	disposition CommitDisposition,
) (CommitReceipt, error) {
	if err := verifyExpectedReplayWriter(
		expectation.contractVersion,
		expectation.basisKind,
		detail.writer,
		true,
	); err != nil {
		return CommitReceipt{}, err
	}
	common := detail.common
	admission := detail.admission
	closure := detail.closure
	expectedRevision, err := graphRevisionFromSQLite(common.eventExpectedRevision)
	if err != nil {
		return CommitReceipt{}, storedAdmissionIntegrity("event expected revision", err)
	}
	graphRevision, err := graphRevisionFromSQLite(common.eventRevision)
	if err != nil {
		return CommitReceipt{}, storedAdmissionIntegrity("event graph revision", err)
	}
	expectedAdmissionRevision, exact := sqliteIntegerFromUint64(
		expectation.expectedRevision.Value(),
	)
	if !exact {
		return CommitReceipt{}, storedAdmissionIntegrity(
			"expected admission graph revision",
			ErrRevisionOverflow,
		)
	}
	if graphRevision.Value() != expectedRevision.Value()+1 {
		return CommitReceipt{}, storedAdmissionIntegrity("event revision is not contiguous", nil)
	}
	basisTypeEnv, err := parseTypeEnvRef(common.eventBasisTypeEnv)
	if err != nil {
		return CommitReceipt{}, storedAdmissionIntegrity("event basis TypeEnv", err)
	}
	resultTypeEnv, err := parseTypeEnvRef(common.eventResultTypeEnv)
	if err != nil {
		return CommitReceipt{}, storedAdmissionIntegrity("event result TypeEnv", err)
	}
	eventDigest, err := digestFields(
		"typed-memory-graph-event.v1",
		expectation.project.String(),
		common.eventCommitRef,
		strconv.FormatUint(expectedRevision.Value(), 10),
		strconv.FormatUint(graphRevision.Value(), 10),
		basisTypeEnv.String(),
		expectation.semanticDigest.String(),
		string(expectation.semanticBytes),
		expectation.eventKind,
		expectation.authorityClass,
		expectation.requestProvenance,
	)
	if err != nil {
		return CommitReceipt{}, storedAdmissionIntegrity("event digest", err)
	}
	expectedCommitRef := derivedRef(
		"typed-memory-commit",
		expectation.project.String(),
		expectation.idempotencyKey.String(),
		expectation.semanticDigest.String(),
		strconv.FormatUint(graphRevision.Value(), 10),
	)
	expectedEventRef := derivedRef("typed-memory-event", eventDigest.String())
	expectedProjectionRef := derivedRef("typed-memory-projection-job", expectedCommitRef)
	identity := genericEventIdentity{
		nextRevision:     graphRevision,
		commitRef:        expectedCommitRef,
		eventRef:         expectedEventRef,
		eventDigest:      eventDigest,
		projectionJobRef: expectedProjectionRef,
	}
	actualFootprint := detail.actualFootprint.footprint
	closureFootprint := materializationFootprintFromClosure(closure)
	expectedMaterializationBytes := canonicalMaterializationClosure(
		identity,
		expectation.prepared,
		expectation.manifest,
		actualFootprint,
		detail.rowDigests,
	)
	materializationDigest, materializationErr := digestBytes(expectedMaterializationBytes)
	if materializationErr != nil {
		return CommitReceipt{}, storedAdmissionIntegrity("materialization digest", materializationErr)
	}
	checks := []struct {
		matches bool
		name    string
	}{
		{expectedRevision == expectation.expectedRevision, "expected revision"},
		{basisTypeEnv == expectation.basisTypeEnv, "basis TypeEnv"},
		{basisTypeEnv == resultTypeEnv, "non-activation result TypeEnv"},
		{common.idempotencyRevision == common.eventRevision, "idempotency revision"},
		{common.idempotencyResultDigest == common.eventDigest, "idempotency result digest"},
		{common.idempotencyChangeDigest == expectation.semanticDigest.String(), "idempotency semantic digest"},
		{common.eventChangeDigest == expectation.semanticDigest.String(), "event semantic digest"},
		{bytes.Equal(common.eventCanonicalBytes, expectation.semanticBytes), "event semantic bytes"},
		{common.eventChangeCount == expectation.changeCount, "event change count"},
		{common.eventKind == expectation.eventKind, "event kind"},
		{common.eventAuthorityClass == expectation.authorityClass, "event authority class"},
		{common.eventProvenance == expectation.requestProvenance, "event provenance"},
		{common.eventDigest == eventDigest.String(), "event digest"},
		{common.eventCommitRef == expectedCommitRef, "event commit ref"},
		{common.idempotencyEventRef == expectedEventRef, "idempotency event ref"},
		{common.commitRef == expectedCommitRef, "commit ref"},
		{common.commitEventRef == expectedEventRef, "commit event ref"},
		{common.commitEventDigest == eventDigest.String(), "commit event digest"},
		{common.commitExpectedRevision == common.eventExpectedRevision, "commit expected revision"},
		{common.commitRevision == common.eventRevision, "commit graph revision"},
		{common.commitChangeDigest == expectation.semanticDigest.String(), "commit semantic digest"},
		{common.commitIdempotencyKey == expectation.idempotencyKey.String(), "commit idempotency key"},
		{common.commitProjectionJobRef == expectedProjectionRef, "commit projection ref"},
		{expectation.requiredEventRef == "" || expectation.requiredEventRef == expectedEventRef, "required event ref"},
		{expectation.requiredCommitRef == "" || expectation.requiredCommitRef == expectedCommitRef, "required commit ref"},
		{admission.eventDigest == eventDigest.String(), "admission event digest"},
		{admission.basisKind == expectation.basisKind.String(), "admission basis kind"},
		{admission.typeEnvRef == expectation.basisTypeEnv.String(), "admission TypeEnv"},
		{admission.basisRevision == expectedAdmissionRevision, "admission graph revision"},
		{admission.requestDigest == expectation.requestDigest.String(), "admission request digest"},
		{bytes.Equal(admission.requestBytes, expectation.requestBytes), "admission request bytes"},
		{admission.semanticDigest == expectation.semanticDigest.String(), "admission semantic digest"},
		{bytes.Equal(admission.semanticBytes, expectation.semanticBytes), "admission semantic bytes"},
		{admission.envelopeDigest == expectation.envelopeDigest.String(), "admission envelope digest"},
		{bytes.Equal(admission.envelopeBytes, expectation.envelopeBytes), "admission envelope bytes"},
		{admission.basisDigest == expectation.basisDigest.String(), "admission basis digest"},
		{bytes.Equal(admission.basisBytes, expectation.basisBytes), "admission basis bytes"},
		{admission.manifestDigest == expectation.manifest.Digest().String(), "admission expected-materialization manifest digest"},
		{bytes.Equal(admission.manifestBytes, expectation.manifest.CanonicalBytes()), "admission expected-materialization manifest bytes"},
		{admission.recordedAt == common.eventRecordedAt, "admission recorded_at"},
		{closure.commitRef == expectedCommitRef, "closure commit ref"},
		{closure.eventDigest == eventDigest.String(), "closure event digest"},
		{closure.basisKind == expectation.basisKind.String(), "closure basis kind"},
		{closure.requestDigest == expectation.requestDigest.String(), "closure request digest"},
		{closure.semanticDigest == expectation.semanticDigest.String(), "closure semantic digest"},
		{closure.envelopeDigest == expectation.envelopeDigest.String(), "closure envelope digest"},
		{closure.basisDigest == expectation.basisDigest.String(), "closure basis digest"},
		{closure.manifestDigest == expectation.manifest.Digest().String(), "closure expected-materialization manifest digest"},
		{closure.recordedAt == common.eventRecordedAt, "closure recorded_at"},
		{closureFootprint == actualFootprint, "closure materialization footprint"},
		{detail.actualFootprint.topLevelChangeCount == expectation.changeCount, "materialization top-level change count"},
		{bytes.Equal(closure.materializationBytes, expectedMaterializationBytes), "closure materialization bytes"},
		{closure.materializationDigest == materializationDigest.String(), "closure materialization digest"},
		{closure.entityCount == common.commitEntityCount, "closure entity count"},
		{closure.entityContextCount == common.commitEntityContextCount, "closure entity-context count"},
	}
	for _, check := range checks {
		if !check.matches {
			return CommitReceipt{}, storedAdmissionIntegrity(check.name, nil)
		}
	}
	return CommitReceipt{
		disposition: disposition,
		eventRef:    expectedEventRef,
		commitRef:   expectedCommitRef,
		revision:    graphRevision,
		digest:      eventDigest,
	}, nil
}

func materializationFootprintFromClosure(
	closure durableV46ClosureRow,
) genericMaterializationFootprint {
	return genericMaterializationFootprint{
		entityCount:                       closure.entityCount,
		entityContextCount:                closure.entityContextCount,
		entityDeclarationCount:            closure.entityDeclarationCount,
		contextSliceCatalogCount:          closure.contextSliceCatalogCount,
		contextSliceCount:                 closure.contextSliceCount,
		valueBlobCount:                    closure.valueBlobCount,
		observableInputBlobCount:          closure.observableInputBlobCount,
		relationCount:                     closure.relationCount,
		relationSlotCount:                 closure.relationSlotCount,
		relationFillerCount:               closure.relationFillerCount,
		orderedCandidatePrefixCount:       closure.orderedCandidatePrefixCount,
		referenceResolutionCount:          closure.referenceResolutionCount,
		memberOfEvaluationCount:           closure.memberOfEvaluationCount,
		memberOfInputCount:                closure.memberOfInputCount,
		memberOfUseCount:                  closure.memberOfUseCount,
		kindClassificationSourceBlobCount: closure.kindClassificationSourceBlobCount,
		kindClassificationEvaluationCount: closure.kindClassificationEvaluationCount,
		kindClassificationFeatureCount:    closure.kindClassificationFeatureCount,
		kindClassificationUseCount:        closure.kindClassificationUseCount,
		aliasChangeCount:                  closure.aliasChangeCount,
		retractionCount:                   closure.retractionCount,
	}
}

// loadGenericReplay is the transaction-time exact replay entry point. A
// missing idempotency key is not an error. A legacy v45 event is found but
// cannot be replayed as a generic admission because no exact admission basis
// exists to compare.
func loadGenericReplay(
	ctx context.Context,
	source scanner,
	request CommitRequest,
	prepared preparedAdmission,
) (CommitReceipt, bool, error) {
	expectation, err := durableGenericExpectationFromPrepared(request, prepared)
	if err != nil {
		return CommitReceipt{}, false, err
	}
	return resolveGenericIdempotencyReplay(
		ctx,
		source,
		expectation,
		CommitReplay,
	)
}

// resolveDurableGeneric is the post-COMMIT reread entry point. It verifies the
// refs returned by the write path in addition to reconstructing them from the
// durable event identity. Absence after an ambiguous COMMIT remains unknown;
// it is never downgraded to an ordinary retry.
func resolveDurableGeneric(
	ctx context.Context,
	database *sql.DB,
	request CommitRequest,
	prepared preparedAdmission,
	eventRef string,
	commitRef string,
	disposition CommitDisposition,
) (CommitReceipt, error) {
	if ctx == nil {
		return CommitReceipt{}, fmt.Errorf("resolve generic typed-memory durable replay: context is required")
	}
	if database == nil {
		return CommitReceipt{}, ErrDatabaseRequired
	}
	if eventRef == "" || commitRef == "" {
		return CommitReceipt{}, fmt.Errorf("%w: durable event and commit refs are required", ErrCommitOutcomeUnknown)
	}
	expectation, err := durableGenericExpectationFromPrepared(request, prepared)
	if err != nil {
		return CommitReceipt{}, err
	}
	expectation.requiredEventRef = eventRef
	expectation.requiredCommitRef = commitRef
	transaction, err := sqlitetransaction.BeginRead(ctx, database)
	if err != nil {
		return CommitReceipt{}, fmt.Errorf("begin generic typed-memory durable reread: %w", err)
	}
	receipt, found, readErr := resolveGenericIdempotencyReplay(
		ctx,
		transaction,
		expectation,
		disposition,
	)
	finish := transaction.Rollback(ctx)
	if readErr != nil {
		return CommitReceipt{}, errors.Join(readErr, finish.Err())
	}
	if !finish.Succeeded() {
		return CommitReceipt{}, fmt.Errorf("finish generic typed-memory durable reread: %w", finish.Err())
	}
	if !found {
		return CommitReceipt{}, ErrCommitOutcomeUnknown
	}
	return receipt, nil
}

func (adapter *SQLiteAdapter) resolveDurableGeneric(
	request CommitRequest,
	prepared preparedAdmission,
	eventRef string,
	commitRef string,
	disposition CommitDisposition,
) (CommitReceipt, error) {
	readCtx, cancel := context.WithTimeout(context.Background(), durableRereadTimeout)
	defer cancel()
	return resolveDurableGeneric(
		readCtx,
		adapter.database,
		request,
		prepared,
		eventRef,
		commitRef,
		disposition,
	)
}

func genericReplayConflict(detail string, cause error) error {
	message := fmt.Errorf("%w: generic replay mismatch: %s", ErrIdempotencyConflict, detail)
	if cause == nil {
		return message
	}
	return errors.Join(message, cause)
}

func storedAdmissionIntegrity(detail string, cause error) error {
	message := fmt.Errorf("%w: %s", ErrStoredAdmissionIntegrity, detail)
	if cause == nil {
		return message
	}
	return errors.Join(message, cause)
}

func resolveGenericIdempotencyReplay(
	ctx context.Context,
	source scanner,
	expectation durableGenericAdmissionExpectation,
	disposition CommitDisposition,
) (CommitReceipt, bool, error) {
	detail, found, err := loadDurableGenericAdmissionDetail(
		ctx,
		source,
		expectation,
	)
	if err != nil || !found {
		return CommitReceipt{}, found, err
	}
	receipt, err := verifyDurableGenericAdmissionDetail(detail, expectation, disposition)
	if err != nil {
		return CommitReceipt{}, true, err
	}
	return receipt, true, nil
}
