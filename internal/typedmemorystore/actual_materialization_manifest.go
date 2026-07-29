package typedmemorystore

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

// verifyActualMaterializationManifest rereads coordinate-bearing semantic
// projections from the same scanner/transaction and compares them to the pure
// sealed-admission manifest. Count equality is insufficient: every exact
// coordinate and witness participates in the comparison.
func verifyActualMaterializationManifest(
	ctx context.Context,
	source scanner,
	project projectledger.ProjectID,
	eventRef string,
	expected expectedMaterializationManifest,
) error {
	if err := verifyConditionalSemanticRows(
		ctx,
		source,
		project,
		eventRef,
		expected,
	); err != nil {
		return err
	}
	actualResolutions, err := loadActualResolutionWitnesses(
		ctx,
		source,
		project,
		eventRef,
	)
	if err != nil {
		return err
	}
	expectedResolutions := make([][]byte, 0, len(expected.resolutions))
	for _, witness := range expected.resolutions {
		expectedResolutions = append(expectedResolutions, witness.canonicalBytes)
	}
	if err := compareActualManifestSet(
		"reference-resolution witnesses",
		expectedResolutions,
		actualResolutions,
	); err != nil {
		return err
	}
	actualEvaluations, err := loadActualEvaluationWitnesses(
		ctx,
		source,
		project,
		eventRef,
	)
	if err != nil {
		return err
	}
	expectedEvaluations := make([][]byte, 0, len(expected.evaluations))
	for _, witness := range expected.evaluations {
		expectedEvaluations = append(expectedEvaluations, witness.canonicalBytes)
	}
	if err := compareActualManifestSet(
		"MemberOf evaluation witnesses",
		expectedEvaluations,
		actualEvaluations,
	); err != nil {
		return err
	}

	actualObservableInputs, err := loadActualObservableInputTuples(
		ctx,
		source,
		project,
		eventRef,
	)
	if err != nil {
		return err
	}
	expectedObservableInputs := make([][]byte, 0, len(expected.observableInputs))
	for _, tuple := range expected.observableInputs {
		expectedObservableInputs = append(expectedObservableInputs, tuple.canonicalBytes)
	}
	if err := compareActualManifestSet(
		"MemberOf observable-input tuples",
		expectedObservableInputs,
		actualObservableInputs,
	); err != nil {
		return err
	}

	actualMemberUses, err := loadActualMemberUseCoordinates(
		ctx,
		source,
		project,
		eventRef,
	)
	if err != nil {
		return err
	}
	expectedMemberUses := make([][]byte, 0, len(expected.memberUses))
	for _, use := range expected.memberUses {
		expectedMemberUses = append(expectedMemberUses, use.canonicalBytes)
	}
	if err := compareActualManifestSet(
		"relation-filler MemberOf-use coordinates",
		expectedMemberUses,
		actualMemberUses,
	); err != nil {
		return err
	}

	actualDeclarations, err := loadActualDeclarationCoordinates(
		ctx,
		source,
		project,
		eventRef,
	)
	if err != nil {
		return err
	}
	expectedDeclarations := make([][]byte, 0, len(expected.declarations))
	for _, declaration := range expected.declarations {
		expectedDeclarations = append(expectedDeclarations, declaration.canonicalBytes)
	}
	if err := compareActualManifestSet(
		"entity-declaration coordinates",
		expectedDeclarations,
		actualDeclarations,
	); err != nil {
		return err
	}

	actualSemanticRows, err := loadActualRequiredSemanticRows(
		ctx,
		source,
		project,
		eventRef,
	)
	if err != nil {
		return err
	}
	expectedSemanticRows, err := resolveExpectedRequiredSemanticRows(
		ctx,
		source,
		project,
		eventRef,
		expected,
	)
	if err != nil {
		return err
	}
	if err := compareActualManifestSet(
		"required semantic row identities",
		expectedSemanticRows,
		actualSemanticRows,
	); err != nil {
		return err
	}

	actualPrefixes, err := loadActualOrderedPrefixes(
		ctx,
		source,
		project,
		eventRef,
		expected,
	)
	if err != nil {
		return err
	}
	expectedPrefixes := make([][]byte, 0, len(expected.orderedPrefixes))
	for _, prefix := range expected.orderedPrefixes {
		expectedPrefixes = append(expectedPrefixes, prefix.canonicalBytes)
	}
	return compareActualManifestSet(
		"ordered candidate prefixes",
		expectedPrefixes,
		actualPrefixes,
	)
}

func verifyConditionalSemanticRows(
	ctx context.Context,
	source scanner,
	project projectledger.ProjectID,
	eventRef string,
	expected expectedMaterializationManifest,
) error {
	nextRevision, recordedAt, err := loadCurrentEventMaterializationIdentity(
		ctx,
		source,
		project,
		eventRef,
	)
	if err != nil {
		return err
	}
	if nextRevision != expected.basisRevision+1 {
		return storedAdmissionIntegrity("conditional row event revision", nil)
	}
	expectedGlobals := make([][]byte, 0)
	expectedCatalog := make([][]byte, 0)
	for _, row := range expected.semanticRows {
		if !row.conditional {
			continue
		}
		switch row.rowKind {
		case "global_entity_candidate":
			resolved, resolveErr := resolveExpectedGlobalEntityCandidate(
				ctx,
				source,
				project,
				eventRef,
				expected.basisRevision,
				nextRevision,
				recordedAt,
				row,
			)
			if resolveErr != nil {
				return resolveErr
			}
			if resolved != nil {
				expectedGlobals = append(expectedGlobals, resolved)
			}
		case "context_slice_catalog_candidate":
			resolved, resolveErr := resolveExpectedContextSliceCatalogCandidate(
				ctx,
				source,
				project,
				eventRef,
				expected.basisRevision,
				row,
			)
			if resolveErr != nil {
				return resolveErr
			}
			if resolved != nil {
				expectedCatalog = append(expectedCatalog, resolved)
			}
		default:
			return storedAdmissionIntegrity("unknown conditional semantic row", nil)
		}
	}
	actualGlobals, err := loadActualCurrentGlobalEntityRows(
		ctx,
		source,
		project,
		eventRef,
	)
	if err != nil {
		return err
	}
	if err := compareActualManifestSet(
		"state-resolved global entity rows",
		expectedGlobals,
		actualGlobals,
	); err != nil {
		return err
	}
	actualCatalog, err := loadActualCurrentContextSliceCatalogRows(
		ctx,
		source,
		project,
		eventRef,
	)
	if err != nil {
		return err
	}
	return compareActualManifestSet(
		"state-resolved ContextSlice catalog rows",
		expectedCatalog,
		actualCatalog,
	)
}

func loadCurrentEventMaterializationIdentity(
	ctx context.Context,
	source scanner,
	project projectledger.ProjectID,
	eventRef string,
) (uint64, string, error) {
	var revision int64
	var recordedAt string
	err := source.ScanOne(
		ctx,
		`SELECT graph_revision, recorded_at
		FROM typed_memory_graph_events
		WHERE project_id = ? AND event_ref = ?`,
		[]any{project.String(), eventRef},
		[]any{&revision, &recordedAt},
	)
	if err != nil {
		return 0, "", fmt.Errorf("load conditional row event identity: %w", err)
	}
	if revision <= 0 || recordedAt == "" {
		return 0, "", storedAdmissionIntegrity("conditional row event identity", nil)
	}
	return uint64(revision), recordedAt, nil
}

func resolveExpectedGlobalEntityCandidate(
	ctx context.Context,
	source scanner,
	project projectledger.ProjectID,
	eventRef string,
	basisRevision uint64,
	nextRevision uint64,
	recordedAt string,
	row expectedSemanticRowIdentity,
) ([]byte, error) {
	if len(row.coordinate) != 1 {
		return nil, storedAdmissionIntegrity("expected global entity candidate", nil)
	}
	entityID := row.coordinate[0]
	owner, exists, err := loadPriorGlobalEntityOwner(
		ctx,
		source,
		project,
		entityID,
		basisRevision,
	)
	if err != nil {
		return nil, err
	}
	if exists {
		if err := verifyExistingGlobalEntityIdentity(
			ctx,
			source,
			project,
			entityID,
			owner,
		); err != nil {
			return nil, err
		}
		return nil, nil
	}
	return canonicalGlobalEntityMaterialization(
		entityID,
		eventRef,
		nextRevision,
		recordedAt,
	), nil
}

type conditionalRowOwner struct {
	eventRef   string
	revision   uint64
	recordedAt string
}

func loadPriorGlobalEntityOwner(
	ctx context.Context,
	source scanner,
	project projectledger.ProjectID,
	entityID string,
	basisRevision uint64,
) (conditionalRowOwner, bool, error) {
	owner, found, err := loadPriorGlobalEntityDeclarationOwner(
		ctx,
		source,
		project,
		entityID,
		basisRevision,
	)
	if err != nil || found {
		return owner, found, err
	}
	// A genuinely v45-only store can predate the exact declaration projection,
	// so its immutable entity-context history remains a compatibility witness.
	// Once the store advertises exact v46 capability, however, absence of v46
	// rows cannot prove legacy: it is also the observable result of correlated
	// row loss. Upgraded legacy state therefore needs explicit P9 import.
	legacyOwner, legacyFound, err := loadPriorGlobalEntityContextOwner(
		ctx,
		source,
		project,
		entityID,
		basisRevision,
	)
	if err != nil || !legacyFound {
		return legacyOwner, legacyFound, err
	}
	availability, err := loadGenericStorageAvailability(ctx, source)
	if err != nil {
		return conditionalRowOwner{}, false, err
	}
	switch availability {
	case genericStorageAbsent:
		return legacyOwner, true, nil
	case genericStoragePartial:
		return conditionalRowOwner{}, false, ErrStorageGenerationUnavailable
	case genericStorageExact:
		return conditionalRowOwner{}, false, storedAdmissionIntegrity(
			"v46 global entity declaration witness",
			nil,
		)
	default:
		return conditionalRowOwner{}, false, ErrStorageGenerationUnavailable
	}
}

func loadPriorGlobalEntityDeclarationOwner(
	ctx context.Context,
	source scanner,
	project projectledger.ProjectID,
	entityID string,
	basisRevision uint64,
) (conditionalRowOwner, bool, error) {
	return scanConditionalRowOwner(
		ctx,
		source,
		`SELECT declaration.event_ref, event.graph_revision, event.recorded_at
		FROM typed_memory_entity_declarations declaration
		JOIN typed_memory_graph_events event
			ON event.project_id = declaration.project_id
			AND event.event_ref = declaration.event_ref
		JOIN typed_memory_graph_commits commit_record
			ON commit_record.project_id = event.project_id
			AND commit_record.event_ref = event.event_ref
		WHERE declaration.project_id = ? AND declaration.entity_id = ?
			AND event.graph_revision <= ?
		ORDER BY event.graph_revision, declaration.change_ordinal
		LIMIT 1`,
		project,
		entityID,
		basisRevision,
		"global entity declaration owner",
	)
}

func loadPriorGlobalEntityContextOwner(
	ctx context.Context,
	source scanner,
	project projectledger.ProjectID,
	entityID string,
	basisRevision uint64,
) (conditionalRowOwner, bool, error) {
	return scanConditionalRowOwner(
		ctx,
		source,
		`SELECT context.declared_event_ref, event.graph_revision, event.recorded_at
		FROM typed_memory_entity_contexts context
		JOIN typed_memory_graph_events event
			ON event.project_id = context.project_id
			AND event.event_ref = context.declared_event_ref
		JOIN typed_memory_graph_commits commit_record
			ON commit_record.project_id = event.project_id
			AND commit_record.event_ref = event.event_ref
		WHERE context.project_id = ? AND context.entity_id = ?
			AND event.graph_revision <= ?
		ORDER BY event.graph_revision, context.bounded_context_ref
		LIMIT 1`,
		project,
		entityID,
		basisRevision,
		"legacy global entity context owner",
	)
}

func scanConditionalRowOwner(
	ctx context.Context,
	source scanner,
	statement string,
	project projectledger.ProjectID,
	entityID string,
	basisRevision uint64,
	detail string,
) (conditionalRowOwner, bool, error) {
	var eventRef string
	var revision int64
	var recordedAt string
	sqliteBasisRevision, exact := sqliteIntegerFromUint64(basisRevision)
	if !exact {
		return conditionalRowOwner{}, false, storedAdmissionIntegrity(
			detail+" basis revision",
			nil,
		)
	}
	err := source.ScanOne(
		ctx,
		statement,
		[]any{project.String(), entityID, sqliteBasisRevision},
		[]any{&eventRef, &revision, &recordedAt},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return conditionalRowOwner{}, false, nil
	}
	if err != nil {
		return conditionalRowOwner{}, false, fmt.Errorf("load %s: %w", detail, err)
	}
	if eventRef == "" || revision <= 0 || recordedAt == "" {
		return conditionalRowOwner{}, false, storedAdmissionIntegrity(detail, nil)
	}
	return conditionalRowOwner{
		eventRef:   eventRef,
		revision:   uint64(revision),
		recordedAt: recordedAt,
	}, true, nil
}

func verifyExistingGlobalEntityIdentity(
	ctx context.Context,
	source scanner,
	project projectledger.ProjectID,
	entityID string,
	owner conditionalRowOwner,
) error {
	var eventRef string
	var revision int64
	var recordedAt string
	err := source.ScanOne(
		ctx,
		`SELECT first_declared_event_ref, first_declared_revision, recorded_at
		FROM typed_memory_entities
		WHERE project_id = ? AND entity_id = ?`,
		[]any{project.String(), entityID},
		[]any{&eventRef, &revision, &recordedAt},
	)
	if err != nil {
		return fmt.Errorf("verify existing global entity owner: %w", err)
	}
	ownerRevision, exact := sqliteIntegerFromUint64(owner.revision)
	if !exact {
		return storedAdmissionIntegrity("existing global entity owner revision", nil)
	}
	matches := eventRef == owner.eventRef &&
		revision == ownerRevision &&
		recordedAt == owner.recordedAt
	if !matches {
		return storedAdmissionIntegrity("existing global entity owner", nil)
	}
	return nil
}

func canonicalGlobalEntityMaterialization(
	entityID string,
	eventRef string,
	revision uint64,
	recordedAt string,
) []byte {
	return canonicalStorageFields(
		"typed-memory-expected-global-entity-row.v1",
		[]string{
			entityID,
			eventRef,
			strconv.FormatUint(revision, 10),
			recordedAt,
		},
	)
}

func loadActualCurrentGlobalEntityRows(
	ctx context.Context,
	source scanner,
	project projectledger.ProjectID,
	eventRef string,
) ([][]byte, error) {
	rows, err := loadActualManifestAggregate(
		ctx,
		source,
		`SELECT COUNT(*), COALESCE(group_concat(encoded_row, '|'), '')
		FROM (
			SELECT hex(entity_id) || ',' || hex(first_declared_event_ref) || ',' ||
				hex(CAST(first_declared_revision AS TEXT)) || ',' || hex(recorded_at) AS encoded_row
			FROM typed_memory_entities
			WHERE project_id = ? AND first_declared_event_ref = ?
		)`,
		project,
		eventRef,
	)
	if err != nil {
		return nil, fmt.Errorf("load actual global entity rows: %w", err)
	}
	result := make([][]byte, 0, len(rows))
	for _, encoded := range rows {
		fields := strings.Split(encoded, ",")
		if len(fields) != 4 {
			return nil, storedAdmissionIntegrity("global entity row shape", nil)
		}
		decoded, err := decodeActualManifestRequiredFields(fields)
		if err != nil {
			return nil, storedAdmissionIntegrity("global entity row fields", err)
		}
		revision, err := strconv.ParseUint(decoded[2], 10, 64)
		if err != nil {
			return nil, storedAdmissionIntegrity("global entity revision", err)
		}
		result = append(result, canonicalGlobalEntityMaterialization(
			decoded[0],
			decoded[1],
			revision,
			decoded[3],
		))
	}
	return result, nil
}

func resolveExpectedContextSliceCatalogCandidate(
	ctx context.Context,
	source scanner,
	project projectledger.ProjectID,
	eventRef string,
	basisRevision uint64,
	row expectedSemanticRowIdentity,
) ([]byte, error) {
	if len(row.coordinate) != 2 {
		return nil, storedAdmissionIntegrity("expected ContextSlice catalog candidate", nil)
	}
	prior, exists, err := loadPriorContextSliceOwner(
		ctx,
		source,
		project,
		row.coordinate[0],
		basisRevision,
	)
	if err != nil {
		return nil, err
	}
	if exists {
		matches := prior.digest == row.semanticDigest.String() &&
			prior.contextRef == row.coordinate[1] &&
			bytes.Equal(prior.canonicalBytes, row.semanticBytes)
		if !matches {
			return nil, storedAdmissionIntegrity("ContextSlice catalog pre-state identity", nil)
		}
		if err := verifyExistingContextSliceCatalogIdentity(
			ctx,
			source,
			project,
			row,
			prior.owner,
		); err != nil {
			return nil, err
		}
		return nil, nil
	}
	coordinate := append(append([]string(nil), row.coordinate...), eventRef)
	resolved := newExpectedSemanticRowIdentity(
		"context_slice_catalog",
		coordinate,
		row.semanticDigest,
		row.semanticBytes,
		false,
	)
	return resolved.canonicalBytes, nil
}

type priorContextSliceMaterialization struct {
	owner          conditionalRowOwner
	digest         string
	contextRef     string
	canonicalBytes []byte
}

func loadPriorContextSliceOwner(
	ctx context.Context,
	source scanner,
	project projectledger.ProjectID,
	contextSliceRef string,
	basisRevision uint64,
) (priorContextSliceMaterialization, bool, error) {
	var eventRef string
	var revision int64
	var recordedAt string
	var digest string
	var contextRef string
	var canonicalBytes []byte
	sqliteBasisRevision, exact := sqliteIntegerFromUint64(basisRevision)
	if !exact {
		return priorContextSliceMaterialization{}, false, storedAdmissionIntegrity(
			"prior ContextSlice basis revision",
			nil,
		)
	}
	err := source.ScanOne(
		ctx,
		`SELECT slice.event_ref, event.graph_revision, event.recorded_at,
			slice.context_slice_digest, slice.bounded_context_ref,
			slice.canonical_context_slice_bytes
		FROM typed_memory_context_slices slice
		JOIN typed_memory_graph_events event
			ON event.project_id = slice.project_id AND event.event_ref = slice.event_ref
		JOIN typed_memory_graph_commits commit_record
			ON commit_record.project_id = event.project_id AND commit_record.event_ref = event.event_ref
		WHERE slice.project_id = ? AND slice.context_slice_ref = ?
			AND event.graph_revision <= ?
		ORDER BY event.graph_revision, slice.event_ref
		LIMIT 1`,
		[]any{project.String(), contextSliceRef, sqliteBasisRevision},
		[]any{&eventRef, &revision, &recordedAt, &digest, &contextRef, &canonicalBytes},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return priorContextSliceMaterialization{}, false, nil
	}
	if err != nil {
		return priorContextSliceMaterialization{}, false, fmt.Errorf(
			"load prior ContextSlice owner: %w",
			err,
		)
	}
	if eventRef == "" || revision <= 0 || recordedAt == "" ||
		digest == "" || contextRef == "" || len(canonicalBytes) == 0 {
		return priorContextSliceMaterialization{}, false, storedAdmissionIntegrity(
			"prior ContextSlice owner",
			nil,
		)
	}
	return priorContextSliceMaterialization{
		owner: conditionalRowOwner{
			eventRef:   eventRef,
			revision:   uint64(revision),
			recordedAt: recordedAt,
		},
		digest:         digest,
		contextRef:     contextRef,
		canonicalBytes: append([]byte(nil), canonicalBytes...),
	}, true, nil
}

func verifyExistingContextSliceCatalogIdentity(
	ctx context.Context,
	source scanner,
	project projectledger.ProjectID,
	row expectedSemanticRowIdentity,
	owner conditionalRowOwner,
) error {
	var count int64
	var eventRef string
	var digest string
	var contextRef string
	var canonicalBytes []byte
	err := source.ScanOne(
		ctx,
		`SELECT COUNT(*),
			COALESCE(MAX(event_ref), ''),
			COALESCE(MAX(context_slice_digest), ''),
			COALESCE(MAX(bounded_context_ref), ''),
			COALESCE(MAX(canonical_context_slice_bytes), X'')
		FROM typed_memory_context_slice_catalog
		WHERE project_id = ? AND context_slice_ref = ?`,
		[]any{project.String(), row.coordinate[0]},
		[]any{&count, &eventRef, &digest, &contextRef, &canonicalBytes},
	)
	if err != nil {
		return fmt.Errorf("verify existing ContextSlice catalog row: %w", err)
	}
	matches := count == 1 &&
		eventRef == owner.eventRef &&
		digest == row.semanticDigest.String() &&
		contextRef == row.coordinate[1] &&
		bytes.Equal(canonicalBytes, row.semanticBytes)
	if !matches {
		return storedAdmissionIntegrity("existing ContextSlice catalog row identity", nil)
	}
	return nil
}

func loadActualCurrentContextSliceCatalogRows(
	ctx context.Context,
	source scanner,
	project projectledger.ProjectID,
	eventRef string,
) ([][]byte, error) {
	layout := actualSemanticRowLayout{
		detail:  "ContextSlice catalog",
		rowKind: "context_slice_catalog",
		statement: `SELECT COUNT(*), COALESCE(group_concat(encoded_row, '|'), '')
		FROM (
			SELECT hex(context_slice_ref) || ',' || hex(bounded_context_ref) || ',' ||
				hex(event_ref) || ',' || hex(context_slice_digest) || ',' ||
				hex(canonical_context_slice_bytes) AS encoded_row
			FROM typed_memory_context_slice_catalog
			WHERE project_id = ? AND event_ref = ?
		)`,
		coordinateFieldCount: 3,
	}
	return loadActualSemanticRows(ctx, source, project, eventRef, layout)
}

type actualSemanticRowLayout struct {
	detail                 string
	rowKind                string
	statement              string
	coordinateFieldCount   int
	stripSemanticBytes     bool
	verifyRawContentDigest bool
}

func loadActualRequiredSemanticRows(
	ctx context.Context,
	source scanner,
	project projectledger.ProjectID,
	eventRef string,
) ([][]byte, error) {
	entityContexts, err := loadActualEntityContextSemanticRows(
		ctx,
		source,
		project,
		eventRef,
	)
	if err != nil {
		return nil, err
	}
	layouts := []actualSemanticRowLayout{
		{
			detail:  "context-slice",
			rowKind: "context_slice",
			statement: `SELECT COUNT(*), COALESCE(group_concat(encoded_row, '|'), '')
			FROM (
				SELECT hex(context_slice_ref) || ',' || hex(bounded_context_ref) || ',' ||
					hex(context_slice_digest) || ',' ||
					hex(canonical_context_slice_bytes) AS encoded_row
				FROM typed_memory_context_slices
				WHERE project_id = ? AND event_ref = ?
			)`,
			coordinateFieldCount: 2,
		},
		{
			detail:  "value blob",
			rowKind: "value_blob",
			statement: `SELECT COUNT(*), COALESCE(group_concat(encoded_row, '|'), '')
			FROM (
				SELECT hex(value_ref) || ',' || hex(value_kind_ref) || ',' ||
					hex(value_shape_ref) || ',' ||
					hex(codec_ref) || ',' || hex(value_digest) || ',' ||
					hex(value_digest) || ',' || hex(canonical_value_bytes) AS encoded_row
				FROM typed_memory_value_blobs
				WHERE project_id = ? AND event_ref = ?
			)`,
			coordinateFieldCount: 5,
		},
		{
			detail:  "observable input blob",
			rowKind: "observable_input_blob",
			statement: `SELECT COUNT(*), COALESCE(group_concat(encoded_row, '|'), '')
			FROM (
				SELECT hex(observable_input_ref) || ',' || hex(observable_input_digest) || ',' ||
					hex(canonical_observable_input_bytes) AS encoded_row
				FROM typed_memory_observable_input_blobs
				WHERE project_id = ? AND event_ref = ?
			)`,
			coordinateFieldCount:   1,
			stripSemanticBytes:     true,
			verifyRawContentDigest: true,
		},
		{
			detail:  "kind-classification source blob v54",
			rowKind: kindClassificationSourceBlobRowKind54,
			statement: `SELECT COUNT(*), COALESCE(group_concat(encoded_row, '|'), '')
			FROM (
				SELECT hex(source_ref) || ',' || hex(source_digest) || ',' ||
					hex(canonical_source_bytes) AS encoded_row
				FROM ` + kindClassificationSourceBlobTable54 + `
				WHERE project_id = ? AND event_ref = ?
			)`,
			coordinateFieldCount:   1,
			stripSemanticBytes:     true,
			verifyRawContentDigest: true,
		},
		{
			detail:  "kind-classification evaluation v54",
			rowKind: kindClassificationEvaluationRowKind54,
			statement: `SELECT COUNT(*), COALESCE(group_concat(encoded_row, '|'), '')
			FROM (
				SELECT hex(evaluation_ref) || ',' || hex(judgement_kind) || ',' ||
					hex(entity_id) || ',' || hex(candidate_value_kind_ref) || ',' ||
					hex(local_value_kind_ref) || ',' || hex(signature_ref) || ',' ||
					hex(context_slice_ref) || ',' || hex(criterion_rule_ref) || ',' ||
					hex(feature_set_digest) || ',' || hex(request_digest) || ',' ||
					hex(basis_digest) || ',' || hex(judgement_digest) || ',' ||
					hex(canonical_judgement_bytes) AS encoded_row
				FROM ` + kindClassificationEvaluationTable54 + `
				WHERE project_id = ? AND event_ref = ?
			)`,
			coordinateFieldCount: 11,
		},
		{
			detail:  "kind-classification feature v54",
			rowKind: kindClassificationFeatureRowKind54,
			statement: `SELECT COUNT(*), COALESCE(group_concat(encoded_row, '|'), '')
			FROM (
				SELECT hex(evaluation_ref) || ',' ||
					hex(CAST(feature_ordinal AS TEXT)) || ',' || hex(source_kind) || ',' ||
					hex(source_ref) || ',' || hex(source_digest) || ',' ||
					hex(feature_key) || ',' || hex(governor_rule_ref) || ',' ||
					hex(feature_digest) || ',' || hex(canonical_feature_bytes) AS encoded_row
				FROM ` + kindClassificationFeatureTable54 + `
				WHERE project_id = ? AND event_ref = ?
			)`,
			coordinateFieldCount: 7,
		},
		{
			detail:  "relational-assertion classification use v54",
			rowKind: kindClassificationUseRowKind54,
			statement: `SELECT COUNT(*), COALESCE(group_concat(encoded_row, '|'), '')
			FROM (
				SELECT hex(CAST(change_ordinal AS TEXT)) || ',' || hex(assertion_id) || ',' ||
					hex(CAST(slot_ordinal AS TEXT)) || ',' ||
					hex(CAST(filler_ordinal AS TEXT)) || ',' || hex(filler_digest) || ',' ||
					hex(use_kind) || ',' || hex(constraint_id) || ',' ||
					hex(queried_value_kind_ref) || ',' || hex(request_digest) || ',' ||
					hex(evaluation_ref) || ',' || hex(expected_judgement_kind) || ',' ||
					hex(use_digest) || ',' || hex(canonical_use_bytes) AS encoded_row
				FROM ` + kindClassificationUseTable54 + `
				WHERE project_id = ? AND event_ref = ?
			)`,
			coordinateFieldCount: 11,
		},
		{
			detail:  "relation instance",
			rowKind: "relation_instance",
			statement: `SELECT COUNT(*), COALESCE(group_concat(encoded_row, '|'), '')
			FROM (
				SELECT hex(CAST(change_ordinal AS TEXT)) || ',' || hex(assertion_id) || ',' ||
					hex(signature_ref) || ',' || hex(context_slice_ref) || ',' ||
					hex(provenance_ref) || ',' ||
					hex(relation_digest) || ',' || hex(canonical_relation_bytes) AS encoded_row
				FROM typed_memory_relation_instances
				WHERE project_id = ? AND event_ref = ?
			)`,
			coordinateFieldCount: 5,
		},
		{
			detail:  "relation slot",
			rowKind: "relation_slot",
			statement: `SELECT COUNT(*), COALESCE(group_concat(encoded_row, '|'), '')
			FROM (
				SELECT hex(CAST(change_ordinal AS TEXT)) || ',' || hex(assertion_id) || ',' ||
					hex(CAST(slot_ordinal AS TEXT)) || ',' || hex(slot_kind_ref) || ',' ||
					hex(slot_digest) || ',' ||
					hex(canonical_slot_bytes) AS encoded_row
				FROM typed_memory_relation_slots
				WHERE project_id = ? AND event_ref = ?
			)`,
			coordinateFieldCount: 4,
		},
		{
			detail:  "relation filler",
			rowKind: "relation_filler",
			statement: `SELECT COUNT(*), COALESCE(group_concat(encoded_row, '|'), '')
			FROM (
				SELECT hex(CAST(change_ordinal AS TEXT)) || ',' || hex(assertion_id) || ',' ||
					hex(CAST(slot_ordinal AS TEXT)) || ',' ||
					hex(CAST(filler_ordinal AS TEXT)) || ',' || hex(filler_kind) || ',' ||
					hex(reference_kind_ref) || ',' || hex(reference_id) || ',' ||
					hex(entity_id) || ',' || hex(required_value_kind_ref) || ',' ||
					hex(value_ref) || ',' || hex(filler_digest) || ',' ||
					hex(canonical_filler_bytes) AS encoded_row
				FROM typed_memory_relation_fillers
				WHERE project_id = ? AND event_ref = ?
			)`,
			coordinateFieldCount: 10,
		},
		{
			detail:  "relational assertion v3",
			rowKind: relationalAssertionStorageFamily.assertionRowKind,
			statement: `SELECT COUNT(*), COALESCE(group_concat(encoded_row, '|'), '')
			FROM (
				SELECT hex(CAST(change_ordinal AS TEXT)) || ',' || hex(assertion_id) || ',' ||
					hex(signature_ref) || ',' || hex(context_slice_ref) || ',' ||
					hex(modality) || ',' || hex(provenance_ref) || ',' ||
					hex(assertion_digest) || ',' || hex(canonical_assertion_bytes) AS encoded_row
				FROM typed_memory_relational_assertions_v3
				WHERE project_id = ? AND event_ref = ?
			)`,
			coordinateFieldCount: 6,
		},
		{
			detail:  "relational assertion slot v3",
			rowKind: relationalAssertionStorageFamily.slotRowKind,
			statement: `SELECT COUNT(*), COALESCE(group_concat(encoded_row, '|'), '')
			FROM (
				SELECT hex(CAST(change_ordinal AS TEXT)) || ',' || hex(assertion_id) || ',' ||
					hex(CAST(slot_ordinal AS TEXT)) || ',' || hex(slot_kind_ref) || ',' ||
					hex(slot_digest) || ',' || hex(canonical_slot_bytes) AS encoded_row
				FROM typed_memory_relational_assertion_slots_v3
				WHERE project_id = ? AND event_ref = ?
			)`,
			coordinateFieldCount: 4,
		},
		{
			detail:  "relational assertion filler v3",
			rowKind: relationalAssertionStorageFamily.fillerRowKind,
			statement: `SELECT COUNT(*), COALESCE(group_concat(encoded_row, '|'), '')
			FROM (
				SELECT hex(CAST(change_ordinal AS TEXT)) || ',' || hex(assertion_id) || ',' ||
					hex(CAST(slot_ordinal AS TEXT)) || ',' ||
					hex(CAST(filler_ordinal AS TEXT)) || ',' || hex(filler_kind) || ',' ||
					hex(reference_kind_ref) || ',' || hex(reference_id) || ',' ||
					hex(entity_id) || ',' || hex(required_value_kind_ref) || ',' ||
					hex(value_ref) || ',' || hex(filler_digest) || ',' ||
					hex(canonical_filler_bytes) AS encoded_row
				FROM typed_memory_relational_assertion_fillers_v3
				WHERE project_id = ? AND event_ref = ?
			)`,
			coordinateFieldCount: 10,
		},
	}
	result := append([][]byte(nil), entityContexts...)
	for _, layout := range layouts {
		rows, err := loadActualSemanticRows(
			ctx,
			source,
			project,
			eventRef,
			layout,
		)
		if err != nil {
			return nil, err
		}
		result = append(result, rows...)
	}
	entailmentRows, err := loadActualDisjointEntailmentSemanticRows(
		ctx,
		source,
		project,
		eventRef,
		legacyRelationStorageFamily,
	)
	if err != nil {
		return nil, err
	}
	result = append(result, entailmentRows...)
	v3EntailmentRows, err := loadActualDisjointEntailmentSemanticRows(
		ctx,
		source,
		project,
		eventRef,
		relationalAssertionStorageFamily,
	)
	if err != nil {
		return nil, err
	}
	result = append(result, v3EntailmentRows...)
	aliasRows, err := loadActualAliasSemanticRows(ctx, source, project, eventRef)
	if err != nil {
		return nil, err
	}
	result = append(result, aliasRows...)
	retractionRows, err := loadActualRetractionSemanticRows(ctx, source, project, eventRef)
	if err != nil {
		return nil, err
	}
	result = append(result, retractionRows...)
	return result, nil
}

func loadActualEntityContextSemanticRows(
	ctx context.Context,
	source scanner,
	project projectledger.ProjectID,
	eventRef string,
) ([][]byte, error) {
	rows, err := loadActualManifestAggregate(
		ctx,
		source,
		`SELECT COUNT(*), COALESCE(group_concat(encoded_row, '|'), '')
		FROM (
			SELECT hex(context.entity_id) || ',' || hex(context.bounded_context_ref) || ',' ||
				hex(context.label) || ',' || hex(context.provenance_ref) || ',' ||
				hex(context.declared_event_ref) || ',' ||
				hex(CAST(context.declared_revision AS TEXT)) || ',' ||
				hex(context.recorded_at) || ',' ||
				hex(declaration.declaration_digest) || ',' ||
				hex(declaration.canonical_declaration_bytes) AS encoded_row
			FROM typed_memory_entity_contexts context
			JOIN typed_memory_entity_declarations declaration
				ON declaration.project_id = context.project_id
				AND declaration.event_ref = context.declared_event_ref
				AND declaration.entity_id = context.entity_id
				AND declaration.bounded_context_ref = context.bounded_context_ref
			WHERE context.project_id = ? AND context.declared_event_ref = ?
		)`,
		project,
		eventRef,
	)
	if err != nil {
		return nil, fmt.Errorf("load actual entity-context manifest: %w", err)
	}
	result := make([][]byte, 0, len(rows))
	for _, encoded := range rows {
		fields := strings.Split(encoded, ",")
		if len(fields) != 9 {
			return nil, storedAdmissionIntegrity("entity-context manifest row shape", nil)
		}
		decoded, err := decodeActualManifestRequiredFields(fields[:8])
		if err != nil {
			return nil, storedAdmissionIntegrity("entity-context manifest fields", err)
		}
		digest, err := typedmemory.NewSHA256Digest(decoded[7])
		if err != nil {
			return nil, storedAdmissionIntegrity("entity-context declaration digest", err)
		}
		canonical, err := hex.DecodeString(fields[8])
		if err != nil {
			return nil, storedAdmissionIntegrity("entity-context declaration bytes", err)
		}
		// The exact declaration carrier binds label and provenance. Keep the raw
		// columns in this projection too, so a row cannot borrow another exact
		// declaration merely by sharing entity and context coordinates.
		coordinate := []string{
			decoded[0],
			decoded[1],
			decoded[2],
			decoded[3],
			decoded[4],
			decoded[5],
			decoded[6],
		}
		row := newExpectedSemanticRowIdentity(
			"entity_context",
			coordinate,
			digest,
			canonical,
			false,
		)
		result = append(result, row.canonicalBytes)
	}
	return result, nil
}

func loadActualSemanticRows(
	ctx context.Context,
	source scanner,
	project projectledger.ProjectID,
	eventRef string,
	layout actualSemanticRowLayout,
) ([][]byte, error) {
	rows, err := loadActualManifestAggregate(
		ctx,
		source,
		layout.statement,
		project,
		eventRef,
	)
	if err != nil {
		return nil, fmt.Errorf("load actual %s manifest: %w", layout.detail, err)
	}
	result := make([][]byte, 0, len(rows))
	for _, encoded := range rows {
		fields := strings.Split(encoded, ",")
		expectedFieldCount := layout.coordinateFieldCount + 2
		if len(fields) != expectedFieldCount {
			return nil, storedAdmissionIntegrity(layout.detail+" manifest row shape", nil)
		}
		coordinate, err := decodeActualManifestRequiredFields(
			fields[:layout.coordinateFieldCount],
		)
		if err != nil {
			return nil, storedAdmissionIntegrity(layout.detail+" coordinates", err)
		}
		digestText, err := decodeActualManifestHexField(
			fields[layout.coordinateFieldCount],
		)
		if err != nil {
			return nil, storedAdmissionIntegrity(layout.detail+" digest", err)
		}
		digest, err := typedmemory.NewSHA256Digest(digestText)
		if err != nil {
			return nil, storedAdmissionIntegrity(layout.detail+" digest", err)
		}
		semanticBytes, err := hex.DecodeString(fields[layout.coordinateFieldCount+1])
		if err != nil {
			return nil, storedAdmissionIntegrity(layout.detail+" canonical bytes", err)
		}
		if layout.verifyRawContentDigest {
			recomputed, digestErr := digestBytes(semanticBytes)
			if digestErr != nil || recomputed != digest {
				return nil, storedAdmissionIntegrity(layout.detail+" content digest", digestErr)
			}
		}
		if layout.stripSemanticBytes {
			semanticBytes = nil
		}
		row := newExpectedSemanticRowIdentity(
			layout.rowKind,
			coordinate,
			digest,
			semanticBytes,
			false,
		)
		result = append(result, row.canonicalBytes)
	}
	return result, nil
}

func loadActualDisjointEntailmentSemanticRows(
	ctx context.Context,
	source scanner,
	project projectledger.ProjectID,
	eventRef string,
	family relationStorageFamily,
) ([][]byte, error) {
	rows, err := loadActualManifestAggregate(
		ctx,
		source,
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
			FROM `+family.disjointnessUseTable+`
			WHERE project_id = ? AND event_ref = ?
		)`,
		project,
		eventRef,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"load actual disjoint-entailment-use manifest: %w",
			err,
		)
	}
	result := make([][]byte, 0, len(rows))
	for _, row := range rows {
		semantic, decodeErr := decodeActualDisjointEntailmentSemanticRow(
			row,
			family.disjointnessUseRowKind,
		)
		if decodeErr != nil {
			return nil, decodeErr
		}
		result = append(result, semantic)
	}
	return result, nil
}

func decodeActualDisjointEntailmentSemanticRow(
	row string,
	rowKind string,
) ([]byte, error) {
	fields := strings.Split(row, ",")
	if len(fields) != 16 {
		return nil, storedAdmissionIntegrity(
			"disjoint-entailment-use manifest row shape",
			nil,
		)
	}
	head, err := decodeActualManifestRequiredFields(fields[:7])
	if err != nil {
		return nil, storedAdmissionIntegrity(
			"disjoint-entailment-use manifest fields",
			err,
		)
	}
	constraintBytes, err := hex.DecodeString(fields[7])
	if err != nil {
		return nil, storedAdmissionIntegrity(
			"disjoint-entailment-use constraint bytes",
			err,
		)
	}
	middle, err := decodeActualManifestRequiredFields(fields[8:12])
	if err != nil {
		return nil, storedAdmissionIntegrity(
			"disjoint-entailment-use manifest fields",
			err,
		)
	}
	counterQueryBytes, err := hex.DecodeString(fields[12])
	if err != nil {
		return nil, storedAdmissionIntegrity(
			"disjoint-entailment-use counter-query bytes",
			err,
		)
	}
	tail, err := decodeActualManifestRequiredFields(fields[13:15])
	if err != nil {
		return nil, storedAdmissionIntegrity(
			"disjoint-entailment-use manifest fields",
			err,
		)
	}
	useBytes, err := hex.DecodeString(fields[15])
	if err != nil {
		return nil, storedAdmissionIntegrity(
			"disjoint-entailment-use canonical bytes",
			err,
		)
	}
	if _, err := strconv.ParseUint(head[0], 10, 64); err != nil {
		return nil, storedAdmissionIntegrity(
			"disjoint-entailment-use change ordinal",
			err,
		)
	}
	if _, err := strconv.ParseUint(head[2], 10, 64); err != nil {
		return nil, storedAdmissionIntegrity(
			"disjoint-entailment-use slot ordinal",
			err,
		)
	}
	if _, err := strconv.ParseUint(head[3], 10, 64); err != nil {
		return nil, storedAdmissionIntegrity(
			"disjoint-entailment-use filler ordinal",
			err,
		)
	}
	fillerDigest, err := typedmemory.NewSHA256Digest(head[4])
	if err != nil {
		return nil, storedAdmissionIntegrity(
			"disjoint-entailment-use filler digest",
			err,
		)
	}
	constraintDigest, err := verifyActualManifestContentDigest(
		head[6],
		constraintBytes,
		"disjoint-entailment-use constraint",
	)
	if err != nil {
		return nil, err
	}
	if middle[0] == middle[1] {
		return nil, storedAdmissionIntegrity(
			"disjoint-entailment-use operand identity",
			nil,
		)
	}
	if _, err := typedmemory.NewKindID(middle[0]); err != nil {
		return nil, storedAdmissionIntegrity(
			"disjoint-entailment-use matched operand",
			err,
		)
	}
	if _, err := typedmemory.NewKindID(middle[1]); err != nil {
		return nil, storedAdmissionIntegrity(
			"disjoint-entailment-use excluded operand",
			err,
		)
	}
	counterQueryDigest, err := verifyActualManifestContentDigest(
		middle[3],
		counterQueryBytes,
		"disjoint-entailment-use counter query",
	)
	if err != nil {
		return nil, err
	}
	useDigest, err := verifyActualManifestContentDigest(
		tail[1],
		useBytes,
		"disjoint-entailment-use canonical use",
	)
	if err != nil {
		return nil, err
	}
	coordinate := []string{
		head[0],
		head[1],
		head[2],
		head[3],
		fillerDigest.String(),
		head[5],
		constraintDigest.String(),
		string(constraintBytes),
		middle[0],
		middle[1],
		middle[2],
		counterQueryDigest.String(),
		string(counterQueryBytes),
		tail[0],
	}
	semantic := newExpectedSemanticRowIdentity(
		rowKind,
		coordinate,
		useDigest,
		useBytes,
		false,
	)
	return semantic.canonicalBytes, nil
}

func verifyActualManifestContentDigest(
	raw string,
	content []byte,
	detail string,
) (typedmemory.SHA256Digest, error) {
	digest, err := typedmemory.NewSHA256Digest(raw)
	if err != nil {
		return typedmemory.SHA256Digest{}, storedAdmissionIntegrity(
			detail+" digest",
			err,
		)
	}
	recomputed, err := digestBytes(content)
	if err != nil || recomputed != digest {
		return typedmemory.SHA256Digest{}, storedAdmissionIntegrity(
			detail+" content digest",
			err,
		)
	}
	return digest, nil
}

func loadActualAliasSemanticRows(
	ctx context.Context,
	source scanner,
	project projectledger.ProjectID,
	eventRef string,
) ([][]byte, error) {
	rows, err := loadActualManifestAggregate(
		ctx,
		source,
		`SELECT COUNT(*), COALESCE(group_concat(encoded_row, '|'), '')
		FROM (
			SELECT hex(CAST(change_ordinal AS TEXT)) || ',' || hex(alias_change_ref) || ',' ||
				hex(change_kind) || ',' || hex(bounded_context_ref) || ',' || hex(alias) || ',' ||
				CASE WHEN replacement_alias IS NULL THEN 'N'
					ELSE 'S' || hex(replacement_alias) END || ',' ||
				hex(entity_id) || ',' || hex(supersedes_alias_change_ref) || ',' ||
				hex(provenance_ref) || ',' || hex(alias_change_digest) || ',' ||
				hex(canonical_alias_change_bytes) AS encoded_row
			FROM typed_memory_alias_changes
			WHERE project_id = ? AND event_ref = ?
		)`,
		project,
		eventRef,
	)
	if err != nil {
		return nil, fmt.Errorf("load actual alias-change manifest: %w", err)
	}
	result := make([][]byte, 0, len(rows))
	for _, encoded := range rows {
		fields := strings.Split(encoded, ",")
		if len(fields) != 11 {
			return nil, storedAdmissionIntegrity("alias-change manifest row shape", nil)
		}
		head, err := decodeActualManifestRequiredFields(fields[:5])
		if err != nil {
			return nil, storedAdmissionIntegrity("alias-change manifest fields", err)
		}
		replacement, err := decodeActualManifestOptionalField(fields[5])
		if err != nil {
			return nil, storedAdmissionIntegrity("alias-change replacement", err)
		}
		tail, err := decodeActualManifestRequiredFields(fields[6:10])
		if err != nil {
			return nil, storedAdmissionIntegrity("alias-change manifest fields", err)
		}
		digest, err := typedmemory.NewSHA256Digest(tail[3])
		if err != nil {
			return nil, storedAdmissionIntegrity("alias-change digest", err)
		}
		canonical, err := hex.DecodeString(fields[10])
		if err != nil {
			return nil, storedAdmissionIntegrity("alias-change canonical bytes", err)
		}
		coordinate := []string{
			head[0],
			head[1],
			head[2],
			head[3],
			head[4],
			replacement,
			tail[0],
			tail[1],
			tail[2],
		}
		row := newExpectedSemanticRowIdentity(
			"alias_change",
			coordinate,
			digest,
			canonical,
			false,
		)
		result = append(result, row.canonicalBytes)
	}
	return result, nil
}

func loadActualRetractionSemanticRows(
	ctx context.Context,
	source scanner,
	project projectledger.ProjectID,
	eventRef string,
) ([][]byte, error) {
	layout := actualSemanticRowLayout{
		detail:  "assertion retraction",
		rowKind: "assertion_retraction",
		statement: `SELECT COUNT(*), COALESCE(group_concat(encoded_row, '|'), '')
		FROM (
			SELECT hex(CAST(change_ordinal AS TEXT)) || ',' || hex(retraction_ref) || ',' ||
				hex(assertion_id) || ',' || hex(reason) || ',' || hex(provenance_ref) || ',' ||
				hex(retraction_digest) || ',' || hex(canonical_retraction_bytes) AS encoded_row
			FROM typed_memory_assertion_retractions
			WHERE project_id = ? AND event_ref = ?
		)`,
		coordinateFieldCount: 5,
	}
	return loadActualSemanticRows(ctx, source, project, eventRef, layout)
}

func resolveExpectedRequiredSemanticRows(
	ctx context.Context,
	source scanner,
	project projectledger.ProjectID,
	eventRef string,
	expected expectedMaterializationManifest,
) ([][]byte, error) {
	result := make([][]byte, 0, len(expected.semanticRows))
	for _, row := range expected.semanticRows {
		if row.conditional || semanticRowVerifiedByExactProjection(row.rowKind) {
			continue
		}
		resolved, err := resolveExpectedRequiredSemanticRow(
			ctx,
			source,
			project,
			eventRef,
			expected.basisRevision,
			row,
		)
		if err != nil {
			return nil, err
		}
		result = append(result, resolved)
	}
	return result, nil
}

func resolveExpectedRequiredSemanticRow(
	ctx context.Context,
	source scanner,
	project projectledger.ProjectID,
	eventRef string,
	basisRevision uint64,
	row expectedSemanticRowIdentity,
) ([]byte, error) {
	switch row.rowKind {
	case "entity_context":
		return resolveExpectedEntityContextSemanticRow(
			ctx,
			source,
			project,
			eventRef,
			basisRevision,
			row,
		)
	case "alias_change":
		return resolveExpectedAliasSemanticRow(
			ctx,
			source,
			project,
			eventRef,
			basisRevision,
			row,
		)
	case "assertion_retraction":
		return resolveExpectedRetractionSemanticRow(project, eventRef, row)
	default:
		return row.canonicalBytes, nil
	}
}

func resolveExpectedEntityContextSemanticRow(
	ctx context.Context,
	source scanner,
	project projectledger.ProjectID,
	eventRef string,
	basisRevision uint64,
	row expectedSemanticRowIdentity,
) ([]byte, error) {
	if len(row.coordinate) != 4 {
		return nil, storedAdmissionIntegrity("expected entity-context coordinate", nil)
	}
	revision, recordedAt, err := loadCurrentEventMaterializationIdentity(
		ctx,
		source,
		project,
		eventRef,
	)
	if err != nil {
		return nil, err
	}
	if revision != basisRevision+1 {
		return nil, storedAdmissionIntegrity("entity-context declared revision", nil)
	}
	coordinate := append(
		append([]string(nil), row.coordinate...),
		eventRef,
		strconv.FormatUint(revision, 10),
		recordedAt,
	)
	resolved := newExpectedSemanticRowIdentity(
		row.rowKind,
		coordinate,
		row.semanticDigest,
		row.semanticBytes,
		false,
	)
	return resolved.canonicalBytes, nil
}

func resolveExpectedAliasSemanticRow(
	ctx context.Context,
	source scanner,
	project projectledger.ProjectID,
	eventRef string,
	basisRevision uint64,
	row expectedSemanticRowIdentity,
) ([]byte, error) {
	if len(row.coordinate) != 7 {
		return nil, storedAdmissionIntegrity("expected alias-change coordinate", nil)
	}
	ordinal := row.coordinate[0]
	kind := row.coordinate[1]
	contextRef := row.coordinate[2]
	alias := row.coordinate[3]
	replacement := row.coordinate[4]
	entityID := row.coordinate[5]
	provenance := row.coordinate[6]
	aliasRef := derivedRef(
		"typed-memory-alias-change",
		project.String(),
		eventRef,
		ordinal,
		row.semanticDigest.String(),
	)
	supersedes := ""
	if kind == "supersede_alias" {
		var err error
		supersedes, err = loadExpectedActiveAliasChangeRef(
			ctx,
			source,
			project,
			basisRevision,
			contextRef,
			entityID,
			alias,
		)
		if err != nil {
			return nil, err
		}
	} else if kind != "admit_alias" || replacement != "" {
		return nil, storedAdmissionIntegrity("expected alias-change kind", nil)
	}
	coordinate := []string{
		ordinal,
		aliasRef,
		kind,
		contextRef,
		alias,
		replacement,
		entityID,
		supersedes,
		provenance,
	}
	resolved := newExpectedSemanticRowIdentity(
		row.rowKind,
		coordinate,
		row.semanticDigest,
		row.semanticBytes,
		false,
	)
	return resolved.canonicalBytes, nil
}

func loadExpectedActiveAliasChangeRef(
	ctx context.Context,
	source scanner,
	project projectledger.ProjectID,
	basisRevision uint64,
	contextRef string,
	entityID string,
	alias string,
) (string, error) {
	sqliteBasisRevision, exact := sqliteIntegerFromUint64(basisRevision)
	if !exact {
		return "", storedAdmissionIntegrity("expected active alias basis revision", nil)
	}
	var count int64
	var ref string
	err := source.ScanOne(
		ctx,
		`WITH visible AS (
			SELECT change.alias_change_ref, change.supersedes_alias_change_ref,
				change.change_kind, change.alias, change.replacement_alias,
				change.entity_id
			FROM typed_memory_alias_changes change
			JOIN typed_memory_graph_events event
				ON event.project_id = change.project_id AND event.event_ref = change.event_ref
			JOIN typed_memory_graph_commits commit_record
				ON commit_record.project_id = event.project_id AND commit_record.event_ref = event.event_ref
			WHERE change.project_id = ? AND change.bounded_context_ref = ?
				AND event.graph_revision <= ?
		), active AS (
			SELECT current.* FROM visible current
			WHERE NOT EXISTS (
				SELECT 1 FROM visible successor
				WHERE successor.supersedes_alias_change_ref = current.alias_change_ref
			)
		)
		SELECT COUNT(*), COALESCE(MAX(alias_change_ref), '') FROM active
		WHERE entity_id = ?
			AND CASE change_kind WHEN 'admit_alias' THEN alias ELSE replacement_alias END = ?`,
		[]any{
			project.String(),
			contextRef,
			sqliteBasisRevision,
			entityID,
			alias,
		},
		[]any{&count, &ref},
	)
	if err != nil {
		return "", fmt.Errorf("load expected superseded alias row: %w", err)
	}
	if count != 1 || ref == "" {
		return "", storedAdmissionIntegrity("expected superseded alias row", nil)
	}
	return ref, nil
}

func resolveExpectedRetractionSemanticRow(
	project projectledger.ProjectID,
	eventRef string,
	row expectedSemanticRowIdentity,
) ([]byte, error) {
	if len(row.coordinate) != 4 {
		return nil, storedAdmissionIntegrity("expected retraction coordinate", nil)
	}
	retractionRef := derivedRef(
		"typed-memory-assertion-retraction",
		project.String(),
		eventRef,
		row.coordinate[0],
		row.semanticDigest.String(),
	)
	coordinate := []string{
		row.coordinate[0],
		retractionRef,
		row.coordinate[1],
		row.coordinate[2],
		row.coordinate[3],
	}
	resolved := newExpectedSemanticRowIdentity(
		row.rowKind,
		coordinate,
		row.semanticDigest,
		row.semanticBytes,
		false,
	)
	return resolved.canonicalBytes, nil
}

func semanticRowVerifiedByExactProjection(rowKind string) bool {
	switch rowKind {
	case "entity_declaration",
		"reference_resolution_use",
		"relational_assertion_reference_resolution_use_v3",
		"memberof_evaluation",
		"memberof_observable_input",
		"relation_filler_memberof_use",
		"relational_assertion_memberof_use_v3",
		"ordered_candidate_prefix":
		return true
	default:
		return false
	}
}

func loadActualEvaluationWitnesses(
	ctx context.Context,
	source scanner,
	project projectledger.ProjectID,
	eventRef string,
) ([][]byte, error) {
	rows, err := loadActualManifestAggregate(
		ctx,
		source,
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
		project,
		eventRef,
	)
	if err != nil {
		return nil, fmt.Errorf("load actual MemberOf evaluation manifest: %w", err)
	}
	result := make([][]byte, 0, len(rows))
	for _, row := range rows {
		fields := strings.Split(row, ",")
		if len(fields) != 24 {
			return nil, storedAdmissionIntegrity("MemberOf evaluation manifest row shape", nil)
		}
		decoded, err := decodeActualManifestRequiredFields(fields[:9])
		if err != nil {
			return nil, storedAdmissionIntegrity("MemberOf evaluation manifest fields", err)
		}
		viewBytes, err := hex.DecodeString(fields[9])
		if err != nil {
			return nil, storedAdmissionIntegrity("MemberOf evaluation view bytes", err)
		}
		optional := make([]string, 0, 6)
		for _, encoded := range fields[10:16] {
			value, decodeErr := decodeActualManifestOptionalField(encoded)
			if decodeErr != nil {
				return nil, storedAdmissionIntegrity("MemberOf evaluation optional view", decodeErr)
			}
			optional = append(optional, value)
		}
		tail, err := decodeActualManifestRequiredFields(fields[16:19])
		if err != nil {
			return nil, storedAdmissionIntegrity("MemberOf evaluation aggregate fields", err)
		}
		inputCount, err := strconv.ParseUint(tail[0], 10, 64)
		if err != nil {
			return nil, storedAdmissionIntegrity("MemberOf observable input count", err)
		}
		viewDigest, err := typedmemory.NewSHA256Digest(decoded[8])
		if err != nil {
			return nil, storedAdmissionIntegrity("MemberOf evaluation view digest", err)
		}
		inputSetDigest, err := typedmemory.NewSHA256Digest(tail[1])
		if err != nil {
			return nil, storedAdmissionIntegrity("MemberOf observable input-set digest", err)
		}
		queryDigest, err := typedmemory.NewSHA256Digest(tail[2])
		if err != nil {
			return nil, storedAdmissionIntegrity("MemberOf query digest", err)
		}
		queryBytes, err := hex.DecodeString(fields[19])
		if err != nil {
			return nil, storedAdmissionIntegrity("MemberOf query bytes", err)
		}
		basisDigestText, err := decodeActualManifestHexField(fields[20])
		if err != nil {
			return nil, storedAdmissionIntegrity("MemberOf basis digest", err)
		}
		basisDigest, err := typedmemory.NewSHA256Digest(basisDigestText)
		if err != nil {
			return nil, storedAdmissionIntegrity("MemberOf basis digest", err)
		}
		basisBytes, err := hex.DecodeString(fields[21])
		if err != nil {
			return nil, storedAdmissionIntegrity("MemberOf basis bytes", err)
		}
		judgementDigestText, err := decodeActualManifestHexField(fields[22])
		if err != nil {
			return nil, storedAdmissionIntegrity("MemberOf judgement digest", err)
		}
		judgementDigest, err := typedmemory.NewSHA256Digest(judgementDigestText)
		if err != nil {
			return nil, storedAdmissionIntegrity("MemberOf judgement digest", err)
		}
		judgementBytes, err := hex.DecodeString(fields[23])
		if err != nil {
			return nil, storedAdmissionIntegrity("MemberOf judgement bytes", err)
		}
		witness := expectedEvaluationWitness{
			evaluationRef:                    decoded[0],
			judgementKind:                    decoded[1],
			entityID:                         decoded[2],
			valueKindRef:                     decoded[3],
			contextSliceRef:                  decoded[4],
			evaluatorRuleRef:                 decoded[5],
			evaluationProvenanceRef:          decoded[6],
			evaluationViewKind:               decoded[7],
			evaluationViewDigest:             viewDigest,
			evaluationViewBytes:              viewBytes,
			viewDeclarationChangeOrdinal:     optional[0],
			viewLocalReferenceKindRef:        optional[1],
			viewBatchLocalRef:                optional[2],
			viewDeclarationDigest:            optional[3],
			viewPrefixEndOrdinal:             optional[4],
			viewOrderedCandidatePrefixDigest: optional[5],
			observableInputCount:             inputCount,
			observableInputSetDigest:         inputSetDigest,
			queryDigest:                      queryDigest,
			queryBytes:                       queryBytes,
			basisDigest:                      basisDigest,
			basisBytes:                       basisBytes,
			judgementDigest:                  judgementDigest,
			judgementBytes:                   judgementBytes,
		}
		result = append(result, canonicalEvaluationWitness(witness))
	}
	return result, nil
}

func loadActualObservableInputTuples(
	ctx context.Context,
	source scanner,
	project projectledger.ProjectID,
	eventRef string,
) ([][]byte, error) {
	rows, err := loadActualManifestAggregate(
		ctx,
		source,
		`SELECT COUNT(*), COALESCE(group_concat(encoded_row, '|'), '')
		FROM (
			SELECT hex(evaluation_ref) || ',' || hex(CAST(input_ordinal AS TEXT)) || ',' ||
				hex(observable_input_ref) || ',' || hex(observable_input_digest) AS encoded_row
			FROM typed_memory_memberof_observable_inputs
			WHERE project_id = ? AND event_ref = ?
		)`,
		project,
		eventRef,
	)
	if err != nil {
		return nil, fmt.Errorf("load actual MemberOf observable-input manifest: %w", err)
	}
	result := make([][]byte, 0, len(rows))
	for _, row := range rows {
		fields := strings.Split(row, ",")
		if len(fields) != 4 {
			return nil, storedAdmissionIntegrity("MemberOf observable-input manifest row shape", nil)
		}
		decoded, err := decodeActualManifestRequiredFields(fields)
		if err != nil {
			return nil, storedAdmissionIntegrity("MemberOf observable-input manifest fields", err)
		}
		ordinal, err := strconv.ParseUint(decoded[1], 10, 64)
		if err != nil {
			return nil, storedAdmissionIntegrity("MemberOf observable-input ordinal", err)
		}
		digest, err := typedmemory.NewSHA256Digest(decoded[3])
		if err != nil {
			return nil, storedAdmissionIntegrity("MemberOf observable-input digest", err)
		}
		reference, err := typedmemory.NewObservableInputRef(decoded[2])
		if err != nil {
			return nil, storedAdmissionIntegrity("MemberOf observable-input ref", err)
		}
		input, err := typedmemory.NewMemberOfObservableInput(reference, digest)
		if err != nil {
			return nil, storedAdmissionIntegrity("MemberOf observable input", err)
		}
		tuple := newExpectedObservableInputTuple(decoded[0], ordinal, input)
		result = append(result, tuple.canonicalBytes)
	}
	return result, nil
}

func loadActualDeclarationCoordinates(
	ctx context.Context,
	source scanner,
	project projectledger.ProjectID,
	eventRef string,
) ([][]byte, error) {
	rows, err := loadActualManifestAggregate(
		ctx,
		source,
		`SELECT COUNT(*), COALESCE(group_concat(encoded_row, '|'), '')
		FROM (
			SELECT hex(CAST(change_ordinal AS TEXT)) || ',' || hex(entity_id) || ',' ||
				hex(batch_local_ref) || ',' || hex(bounded_context_ref) || ',' ||
				hex(label) || ',' || hex(provenance_ref) || ',' ||
				hex(declaration_digest) || ',' || hex(canonical_declaration_bytes) AS encoded_row
			FROM typed_memory_entity_declarations
			WHERE project_id = ? AND event_ref = ?
		)`,
		project,
		eventRef,
	)
	if err != nil {
		return nil, fmt.Errorf("load actual entity-declaration manifest: %w", err)
	}
	result := make([][]byte, 0, len(rows))
	for _, row := range rows {
		fields := strings.Split(row, ",")
		if len(fields) != 8 {
			return nil, storedAdmissionIntegrity("entity-declaration manifest row shape", nil)
		}
		decoded, err := decodeActualManifestRequiredFields(fields[:7])
		if err != nil {
			return nil, storedAdmissionIntegrity("entity-declaration manifest fields", err)
		}
		ordinal, err := strconv.ParseUint(decoded[0], 10, 64)
		if err != nil {
			return nil, storedAdmissionIntegrity("entity-declaration ordinal", err)
		}
		digest, err := typedmemory.NewSHA256Digest(decoded[6])
		if err != nil {
			return nil, storedAdmissionIntegrity("entity-declaration digest", err)
		}
		canonical, err := hex.DecodeString(fields[7])
		if err != nil {
			return nil, storedAdmissionIntegrity("entity-declaration bytes", err)
		}
		declaration := expectedDeclarationCoordinate{
			changeOrdinal:     ordinal,
			entityID:          decoded[1],
			batchLocalRef:     decoded[2],
			boundedContextRef: decoded[3],
			label:             decoded[4],
			provenanceRef:     decoded[5],
			declarationDigest: digest,
			declarationBytes:  canonical,
		}
		result = append(result, canonicalDeclarationCoordinate(declaration))
	}
	return result, nil
}

func loadActualOrderedPrefixes(
	ctx context.Context,
	source scanner,
	project projectledger.ProjectID,
	eventRef string,
	expected expectedMaterializationManifest,
) ([][]byte, error) {
	rows, err := loadActualManifestAggregate(
		ctx,
		source,
		`SELECT COUNT(*), COALESCE(group_concat(encoded_row, '|'), '')
		FROM (
			SELECT hex(CAST(prefix_end_ordinal AS TEXT)) || ',' ||
				hex(request_digest) || ',' || hex(prefix_digest) AS encoded_row
			FROM typed_memory_ordered_candidate_prefixes
			WHERE project_id = ? AND event_ref = ?
		)`,
		project,
		eventRef,
	)
	if err != nil {
		return nil, fmt.Errorf("load actual ordered-prefix manifest: %w", err)
	}
	prefixByEnd := make(map[uint64]expectedOrderedCandidatePrefix)
	for _, prefix := range expected.orderedPrefixes {
		prefixByEnd[prefix.endOrdinal] = prefix
	}
	result := make([][]byte, 0, len(rows))
	for _, row := range rows {
		fields := strings.Split(row, ",")
		if len(fields) != 3 {
			return nil, storedAdmissionIntegrity("ordered-prefix manifest row shape", nil)
		}
		decoded, err := decodeActualManifestRequiredFields(fields)
		if err != nil {
			return nil, storedAdmissionIntegrity("ordered-prefix manifest fields", err)
		}
		endOrdinal, err := strconv.ParseUint(decoded[0], 10, 64)
		if err != nil {
			return nil, storedAdmissionIntegrity("ordered-prefix end ordinal", err)
		}
		if decoded[1] != expected.requestDigest.String() {
			return nil, storedAdmissionIntegrity("ordered-prefix request digest", nil)
		}
		digest, err := typedmemory.NewSHA256Digest(decoded[2])
		if err != nil {
			return nil, storedAdmissionIntegrity("ordered-prefix digest", err)
		}
		expectedPrefix, exists := prefixByEnd[endOrdinal]
		if !exists {
			return nil, storedAdmissionIntegrity("ordered-prefix coordinate", nil)
		}
		prefix := expectedOrderedCandidatePrefix{
			endOrdinal:   endOrdinal,
			prefixDigest: digest,
			prefixBytes:  expectedPrefix.prefixBytes,
		}
		result = append(result, canonicalOrderedCandidatePrefix(prefix))
	}
	return result, nil
}

func loadActualResolutionWitnesses(
	ctx context.Context,
	source scanner,
	project projectledger.ProjectID,
	eventRef string,
) ([][]byte, error) {
	legacy, err := loadActualResolutionWitnessesFromFamily(
		ctx,
		source,
		project,
		eventRef,
		legacyRelationStorageFamily,
	)
	if err != nil {
		return nil, err
	}
	v3, err := loadActualResolutionWitnessesFromFamily(
		ctx,
		source,
		project,
		eventRef,
		relationalAssertionStorageFamily,
	)
	if err != nil {
		return nil, err
	}
	return append(legacy, v3...), nil
}

func loadActualResolutionWitnessesFromFamily(
	ctx context.Context,
	source scanner,
	project projectledger.ProjectID,
	eventRef string,
	family relationStorageFamily,
) ([][]byte, error) {
	rows, err := loadActualManifestAggregate(
		ctx,
		source,
		`SELECT COUNT(*), COALESCE(group_concat(encoded_row, '|'), '')
		FROM (
			SELECT hex(CAST(change_ordinal AS TEXT)) || ',' ||
				hex(assertion_id) || ',' || hex(CAST(slot_ordinal AS TEXT)) || ',' ||
				hex(CAST(filler_ordinal AS TEXT)) || ',' || hex(filler_digest) || ',' ||
				hex(entity_id) || ',' || hex(resolution_kind) || ',' ||
				CASE WHEN resolution_basis_ref IS NULL THEN 'N'
					ELSE 'S' || hex(resolution_basis_ref) END || ',' ||
				CASE WHEN declaration_change_ordinal IS NULL THEN 'N'
					ELSE 'S' || hex(CAST(declaration_change_ordinal AS TEXT)) END || ',' ||
				CASE WHEN local_reference_kind_ref IS NULL THEN 'N'
					ELSE 'S' || hex(local_reference_kind_ref) END || ',' ||
				CASE WHEN batch_local_ref IS NULL THEN 'N'
					ELSE 'S' || hex(batch_local_ref) END || ',' ||
				CASE WHEN declaration_digest IS NULL THEN 'N'
					ELSE 'S' || hex(declaration_digest) END || ',' ||
				CASE WHEN ordered_candidate_prefix_digest IS NULL THEN 'N'
					ELSE 'S' || hex(ordered_candidate_prefix_digest) END || ',' ||
				hex(resolution_digest) || ',' || hex(canonical_resolution_bytes) AS encoded_row
			FROM `+family.resolutionTable+`
			WHERE project_id = ? AND event_ref = ?
		)`,
		project,
		eventRef,
	)
	if err != nil {
		return nil, fmt.Errorf("load actual reference-resolution manifest: %w", err)
	}
	result := make([][]byte, 0, len(rows))
	for _, row := range rows {
		fields := strings.Split(row, ",")
		if len(fields) != 15 {
			return nil, storedAdmissionIntegrity(
				"reference-resolution manifest row shape",
				nil,
			)
		}
		decoded, err := decodeActualManifestRequiredFields(fields[:7])
		if err != nil {
			return nil, storedAdmissionIntegrity("reference-resolution manifest fields", err)
		}
		changeOrdinal, err := strconv.ParseUint(decoded[0], 10, 64)
		if err != nil {
			return nil, storedAdmissionIntegrity("reference-resolution change ordinal", err)
		}
		slotOrdinal, err := strconv.ParseUint(decoded[2], 10, 64)
		if err != nil {
			return nil, storedAdmissionIntegrity("reference-resolution slot ordinal", err)
		}
		fillerOrdinal, err := strconv.ParseUint(decoded[3], 10, 64)
		if err != nil {
			return nil, storedAdmissionIntegrity("reference-resolution filler ordinal", err)
		}
		fillerDigest, err := typedmemory.NewSHA256Digest(decoded[4])
		if err != nil {
			return nil, storedAdmissionIntegrity("reference-resolution filler digest", err)
		}
		optional := make([]string, 0, 6)
		for _, encoded := range fields[7:13] {
			value, decodeErr := decodeActualManifestOptionalField(encoded)
			if decodeErr != nil {
				return nil, storedAdmissionIntegrity("reference-resolution optional witness", decodeErr)
			}
			optional = append(optional, value)
		}
		resolutionDigestText, err := decodeActualManifestHexField(fields[13])
		if err != nil {
			return nil, storedAdmissionIntegrity("reference-resolution digest", err)
		}
		resolutionDigest, err := typedmemory.NewSHA256Digest(resolutionDigestText)
		if err != nil {
			return nil, storedAdmissionIntegrity("reference-resolution digest", err)
		}
		resolutionBytes, err := hex.DecodeString(fields[14])
		if err != nil {
			return nil, storedAdmissionIntegrity("reference-resolution canonical bytes", err)
		}
		coordinate := newExpectedFillerCoordinate(
			changeOrdinal,
			decoded[1],
			slotOrdinal,
			fillerOrdinal,
			fillerDigest,
		)
		witness := expectedResolutionWitness{
			coordinate:                   coordinate,
			entityID:                     decoded[5],
			resolutionKind:               decoded[6],
			resolutionDigest:             resolutionDigest,
			resolutionBytes:              resolutionBytes,
			resolutionBasisRef:           optional[0],
			declarationChangeOrdinal:     optional[1],
			localReferenceKindRef:        optional[2],
			batchLocalRef:                optional[3],
			declarationDigest:            optional[4],
			orderedCandidatePrefixDigest: optional[5],
		}
		result = append(result, canonicalResolutionWitness(witness))
	}
	return result, nil
}

func loadActualMemberUseCoordinates(
	ctx context.Context,
	source scanner,
	project projectledger.ProjectID,
	eventRef string,
) ([][]byte, error) {
	legacy, err := loadActualMemberUseCoordinatesFromFamily(
		ctx,
		source,
		project,
		eventRef,
		legacyRelationStorageFamily,
	)
	if err != nil {
		return nil, err
	}
	v3, err := loadActualMemberUseCoordinatesFromFamily(
		ctx,
		source,
		project,
		eventRef,
		relationalAssertionStorageFamily,
	)
	if err != nil {
		return nil, err
	}
	return append(legacy, v3...), nil
}

func loadActualMemberUseCoordinatesFromFamily(
	ctx context.Context,
	source scanner,
	project projectledger.ProjectID,
	eventRef string,
	family relationStorageFamily,
) ([][]byte, error) {
	rows, err := loadActualManifestAggregate(
		ctx,
		source,
		`SELECT COUNT(*), COALESCE(group_concat(encoded_row, '|'), '')
		FROM (
			SELECT hex(CAST(change_ordinal AS TEXT)) || ',' ||
				hex(assertion_id) || ',' || hex(CAST(slot_ordinal AS TEXT)) || ',' ||
				hex(CAST(filler_ordinal AS TEXT)) || ',' || hex(filler_digest) || ',' ||
				hex(use_kind) || ',' || hex(constraint_id) || ',' ||
				hex(queried_value_kind_ref) || ',' || hex(query_digest) || ',' ||
				hex(evaluation_ref) || ',' || hex(expected_judgement_kind) || ',' ||
				hex(use_digest) || ',' || hex(canonical_use_bytes) AS encoded_row
			FROM `+family.memberOfUseTable+`
			WHERE project_id = ? AND event_ref = ?
		)`,
		project,
		eventRef,
	)
	if err != nil {
		return nil, fmt.Errorf("load actual MemberOf-use manifest: %w", err)
	}
	result := make([][]byte, 0, len(rows))
	for _, row := range rows {
		fields := strings.Split(row, ",")
		if len(fields) != 13 {
			return nil, storedAdmissionIntegrity("MemberOf-use manifest row shape", nil)
		}
		decoded, err := decodeActualManifestRequiredFields(fields[:12])
		if err != nil {
			return nil, storedAdmissionIntegrity("MemberOf-use manifest fields", err)
		}
		changeOrdinal, err := strconv.ParseUint(decoded[0], 10, 64)
		if err != nil {
			return nil, storedAdmissionIntegrity("MemberOf-use change ordinal", err)
		}
		slotOrdinal, err := strconv.ParseUint(decoded[2], 10, 64)
		if err != nil {
			return nil, storedAdmissionIntegrity("MemberOf-use slot ordinal", err)
		}
		fillerOrdinal, err := strconv.ParseUint(decoded[3], 10, 64)
		if err != nil {
			return nil, storedAdmissionIntegrity("MemberOf-use filler ordinal", err)
		}
		fillerDigest, err := typedmemory.NewSHA256Digest(decoded[4])
		if err != nil {
			return nil, storedAdmissionIntegrity("MemberOf-use filler digest", err)
		}
		queryDigest, err := typedmemory.NewSHA256Digest(decoded[8])
		if err != nil {
			return nil, storedAdmissionIntegrity("MemberOf-use query digest", err)
		}
		useDigest, err := typedmemory.NewSHA256Digest(decoded[11])
		if err != nil {
			return nil, storedAdmissionIntegrity("MemberOf-use digest", err)
		}
		useBytes, err := hex.DecodeString(fields[12])
		if err != nil {
			return nil, storedAdmissionIntegrity("MemberOf-use canonical bytes", err)
		}
		filler := newExpectedFillerCoordinate(
			changeOrdinal,
			decoded[1],
			slotOrdinal,
			fillerOrdinal,
			fillerDigest,
		)
		use := expectedMemberUseCoordinate{
			filler:                filler,
			useKind:               decoded[5],
			constraintID:          decoded[6],
			queriedValueKindRef:   decoded[7],
			queryDigest:           queryDigest,
			evaluationRef:         decoded[9],
			expectedJudgementKind: decoded[10],
			useDigest:             useDigest,
			useBytes:              useBytes,
		}
		result = append(result, canonicalMemberUseCoordinate(use))
	}
	return result, nil
}

func loadActualManifestAggregate(
	ctx context.Context,
	source scanner,
	statement string,
	project projectledger.ProjectID,
	eventRef string,
) ([]string, error) {
	var count int64
	var joined string
	err := source.ScanOne(
		ctx,
		statement,
		[]any{project.String(), eventRef},
		[]any{&count, &joined},
	)
	if err != nil {
		return nil, err
	}
	rows := splitActualManifestAggregate(joined)
	if int64(len(rows)) != count {
		return nil, storedAdmissionIntegrity("materialization manifest aggregate count", nil)
	}
	return rows, nil
}

func splitActualManifestAggregate(joined string) []string {
	if joined == "" {
		return nil
	}
	return strings.Split(joined, "|")
}

func decodeActualManifestRequiredFields(fields []string) ([]string, error) {
	decoded := make([]string, 0, len(fields))
	for _, field := range fields {
		value, err := decodeActualManifestHexField(field)
		if err != nil {
			return nil, err
		}
		decoded = append(decoded, value)
	}
	return decoded, nil
}

func decodeActualManifestHexField(field string) (string, error) {
	value, err := hex.DecodeString(field)
	if err != nil {
		return "", err
	}
	return string(value), nil
}

func decodeActualManifestOptionalField(field string) (string, error) {
	if field == "N" {
		return "", nil
	}
	if !strings.HasPrefix(field, "S") {
		return "", fmt.Errorf("optional field lacks exact presence marker")
	}
	return decodeActualManifestHexField(strings.TrimPrefix(field, "S"))
}

func compareActualManifestSet(
	detail string,
	expected [][]byte,
	actual [][]byte,
) error {
	expectedCopy := cloneAndSortManifestBytes(expected)
	actualCopy := cloneAndSortManifestBytes(actual)
	if len(expectedCopy) != len(actualCopy) {
		return storedAdmissionIntegrity(detail+" count", nil)
	}
	for index := range expectedCopy {
		if !bytes.Equal(expectedCopy[index], actualCopy[index]) {
			return storedAdmissionIntegrity(detail, nil)
		}
	}
	return nil
}

func cloneAndSortManifestBytes(values [][]byte) [][]byte {
	result := make([][]byte, 0, len(values))
	for _, value := range values {
		result = append(result, append([]byte(nil), value...))
	}
	sort.Slice(result, func(left int, right int) bool {
		return bytes.Compare(result[left], result[right]) < 0
	})
	return result
}
