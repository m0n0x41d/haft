package typedmemorystore

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strconv"

	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

type semanticMaterializationBuilder struct {
	ctx                        context.Context
	transaction                *sqlitetransaction.Transaction
	request                    CommitRequest
	admission                  revalidatedAdmission
	environment                typedmemory.TypeEnv
	identity                   genericEventIdentity
	recordedAt                 string
	statements                 []statement
	footprint                  genericMaterializationFootprint
	rowDigests                 []string
	contexts                   map[string]struct{}
	values                     map[string]struct{}
	observables                map[string]struct{}
	evaluations                map[string]struct{}
	evalInputs                 map[string]struct{}
	prefixes                   map[uint64]typedmemory.SHA256Digest
	admissionUse               map[string]typedmemory.ReferenceFillerAdmissionUse
	classificationAdmissionUse map[string]typedmemory.ClassificationReferenceFillerAdmissionUse
	classificationSources      map[string]KindClassificationSourceBlob
}

type sqliteRelationFillerCoordinate struct {
	changeOrdinal int64
	slotOrdinal   int64
	fillerOrdinal int64
}

func newSQLiteRelationFillerCoordinate(
	coordinate typedmemory.RelationFillerCoordinate,
	slotOrdinal uint64,
) (sqliteRelationFillerCoordinate, error) {
	change, err := exactSQLiteCoordinate(
		coordinate.ChangeOrdinal(),
		"relation-filler use change ordinal",
	)
	if err != nil {
		return sqliteRelationFillerCoordinate{}, err
	}
	slot, err := exactSQLiteCoordinate(
		slotOrdinal,
		"relation-filler use slot ordinal",
	)
	if err != nil {
		return sqliteRelationFillerCoordinate{}, err
	}
	filler, err := exactSQLiteCoordinate(
		coordinate.FillerOrdinal(),
		"relation-filler use filler ordinal",
	)
	if err != nil {
		return sqliteRelationFillerCoordinate{}, err
	}
	return sqliteRelationFillerCoordinate{
		changeOrdinal: change,
		slotOrdinal:   slot,
		fillerOrdinal: filler,
	}, nil
}

type materializedRelation interface {
	Assertion() typedmemory.AssertionID
	Signature() typedmemory.RelationSignatureRef
	Slice() typedmemory.ContextSlice
	Bindings() []typedmemory.SlotBinding
	Provenance() typedmemory.ProvenanceRef
}

type relationStorageFamily struct {
	slotTable              string
	fillerTable            string
	resolutionTable        string
	memberOfUseTable       string
	classificationUseTable string
	disjointnessUseTable   string
	assertionDigestTag     string
	slotDigestTag          string
	fillerDigestTag        string
	resolutionDigestTag    string
	memberOfUseDigestTag   string
	disjointnessDigestTag  string
	assertionRowKind       string
	slotRowKind            string
	fillerRowKind          string
	resolutionRowKind      string
	memberOfUseRowKind     string
	disjointnessUseRowKind string
}

var legacyRelationStorageFamily = relationStorageFamily{
	slotTable:              "typed_memory_relation_slots",
	fillerTable:            "typed_memory_relation_fillers",
	resolutionTable:        "typed_memory_reference_resolution_uses",
	memberOfUseTable:       "typed_memory_relation_filler_memberof_uses",
	classificationUseTable: kindClassificationUseTable54,
	disjointnessUseTable:   "typed_memory_relation_filler_disjoint_entailment_uses",
	assertionDigestTag:     "relation:",
	slotDigestTag:          "slot:",
	fillerDigestTag:        "filler:",
	resolutionDigestTag:    "resolution:",
	memberOfUseDigestTag:   "memberof-use:",
	disjointnessDigestTag:  "disjoint-entailment-use:",
	assertionRowKind:       "relation_instance",
	slotRowKind:            "relation_slot",
	fillerRowKind:          "relation_filler",
	resolutionRowKind:      "reference_resolution_use",
	memberOfUseRowKind:     "relation_filler_memberof_use",
	disjointnessUseRowKind: "relation_filler_disjoint_entailment_use",
}

var relationalAssertionStorageFamily = relationStorageFamily{
	slotTable:              "typed_memory_relational_assertion_slots_v3",
	fillerTable:            "typed_memory_relational_assertion_fillers_v3",
	resolutionTable:        "typed_memory_relational_assertion_reference_resolution_uses_v3",
	memberOfUseTable:       "typed_memory_relational_assertion_memberof_uses_v3",
	classificationUseTable: kindClassificationUseTable54,
	disjointnessUseTable:   "typed_memory_relational_assertion_disjointness_uses_v3",
	assertionDigestTag:     "relational-assertion-v3:",
	slotDigestTag:          "relational-assertion-slot-v3:",
	fillerDigestTag:        "relational-assertion-filler-v3:",
	resolutionDigestTag:    "relational-assertion-resolution-v3:",
	memberOfUseDigestTag:   "relational-assertion-memberof-use-v3:",
	disjointnessDigestTag:  "relational-assertion-disjointness-use-v3:",
	assertionRowKind:       "relational_assertion_v3",
	slotRowKind:            "relational_assertion_slot_v3",
	fillerRowKind:          "relational_assertion_filler_v3",
	resolutionRowKind:      "relational_assertion_reference_resolution_use_v3",
	memberOfUseRowKind:     "relational_assertion_memberof_use_v3",
	disjointnessUseRowKind: "relational_assertion_disjointness_use_v3",
}

func relationStorageFamilyForAdmissionContract(
	version AdmissionContractVersion,
) (relationStorageFamily, error) {
	if version.IsV1() {
		return legacyRelationStorageFamily, nil
	}
	if version.IsV2() {
		return relationalAssertionStorageFamily, nil
	}
	return relationStorageFamily{}, fmt.Errorf(
		"%w: unsupported admission contract version %q",
		ErrInvalidAdmissionBatch,
		version.String(),
	)
}

func buildGenericSemanticMaterialization(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	request CommitRequest,
	admission revalidatedAdmission,
	environment typedmemory.TypeEnv,
	identity genericEventIdentity,
	recordedAt string,
) (genericSemanticMaterialization, error) {
	builder := &semanticMaterializationBuilder{
		ctx:          ctx,
		transaction:  transaction,
		request:      request,
		admission:    admission,
		environment:  environment,
		identity:     identity,
		recordedAt:   recordedAt,
		contexts:     make(map[string]struct{}),
		values:       make(map[string]struct{}),
		observables:  make(map[string]struct{}),
		evaluations:  make(map[string]struct{}),
		evalInputs:   make(map[string]struct{}),
		prefixes:     make(map[uint64]typedmemory.SHA256Digest),
		admissionUse: make(map[string]typedmemory.ReferenceFillerAdmissionUse),
		classificationAdmissionUse: make(
			map[string]typedmemory.ClassificationReferenceFillerAdmissionUse,
		),
		classificationSources: make(map[string]KindClassificationSourceBlob),
	}
	if err := builder.indexAdmissionUses(); err != nil {
		return genericSemanticMaterialization{}, err
	}
	if err := builder.appendObservableBlobs(); err != nil {
		return genericSemanticMaterialization{}, err
	}
	if err := builder.appendKindClassificationSourceBlobs(); err != nil {
		return genericSemanticMaterialization{}, err
	}
	for _, change := range admission.prepared.changes {
		if err := builder.appendChange(change); err != nil {
			return genericSemanticMaterialization{}, err
		}
	}
	sort.Strings(builder.rowDigests)
	return genericSemanticMaterialization{
		statements: builder.statements,
		footprint:  builder.footprint,
		rowDigests: builder.rowDigests,
	}, nil
}

func (builder *semanticMaterializationBuilder) indexAdmissionUses() error {
	switch basis := builder.admission.prepared.basis.(type) {
	case typedmemory.SnapshotOnlyBasis:
		return nil
	case typedmemory.ContextSliceMembershipBasis:
		for _, use := range basis.ReferenceFillerAdmissionUses() {
			key := admissionUseCoordinateKey(use.Coordinate())
			if _, exists := builder.admissionUse[key]; exists {
				return ErrInvalidAdmissionBatch
			}
			builder.admissionUse[key] = use
		}
		return nil
	case typedmemory.ContextSliceClassificationBasis:
		for _, use := range basis.ClassificationReferenceFillerAdmissionUses() {
			key := admissionUseCoordinateKey(use.Coordinate())
			if _, exists := builder.classificationAdmissionUse[key]; exists {
				return ErrInvalidAdmissionBatch
			}
			builder.classificationAdmissionUse[key] = use
		}
		return nil
	default:
		return ErrInvalidAdmissionBatch
	}
}

func (builder *semanticMaterializationBuilder) appendObservableBlobs() error {
	for _, blob := range builder.admission.observableBlobs {
		key := blob.Reference().String()
		if _, exists := builder.observables[key]; exists {
			continue
		}
		builder.observables[key] = struct{}{}
		builder.statements = append(builder.statements, statement{
			query: `INSERT INTO typed_memory_observable_input_blobs (
				project_id, event_ref, observable_input_ref,
				observable_input_digest, canonical_observable_input_bytes
			) VALUES (?, ?, ?, ?, ?)`,
			arguments: []any{
				builder.request.project.String(), builder.identity.eventRef,
				blob.Reference().String(), blob.Digest().String(), blob.Bytes(),
			},
		})
		builder.footprint.observableInputBlobCount++
		builder.rowDigests = append(builder.rowDigests, "observable:"+blob.Digest().String())
	}
	return nil
}

func (builder *semanticMaterializationBuilder) appendKindClassificationSourceBlobs() error {
	for _, blob := range builder.admission.classificationSources {
		key := blob.Reference().String()
		if _, exists := builder.classificationSources[key]; exists {
			return ErrInvalidAdmissionBatch
		}
		builder.classificationSources[key] = blob
		builder.statements = append(builder.statements, statement{
			query: `INSERT INTO ` + kindClassificationSourceBlobTable54 + ` (
				project_id, event_ref, source_ref, source_digest,
				canonical_source_bytes
			) VALUES (?, ?, ?, ?, ?)`,
			arguments: []any{
				builder.request.project.String(),
				builder.identity.eventRef,
				blob.Reference().String(),
				blob.Digest().String(),
				blob.Bytes(),
			},
		})
		builder.footprint.kindClassificationSourceBlobCount++
		builder.rowDigests = append(
			builder.rowDigests,
			kindClassificationSourceBlobDigestTag54+blob.Digest().String(),
		)
	}
	return nil
}

func (builder *semanticMaterializationBuilder) appendChange(
	prepared preparedAdmissionChange,
) error {
	switch value := prepared.change.(type) {
	case typedmemory.ValidatedDeclareEntity:
		return builder.appendDeclaration(prepared.ordinal, value.Change())
	case typedmemory.ValidatedIdentityChange:
		return builder.appendIdentityChange(prepared.ordinal, value.Change())
	case typedmemory.ValidatedRelationInstance:
		return builder.appendRelation(prepared.ordinal, value.Relation())
	case typedmemory.ValidatedRelationalAssertion:
		return builder.appendRelationalAssertion(prepared.ordinal, value.Assertion())
	case typedmemory.ValidatedRetraction:
		return builder.appendRetraction(prepared.ordinal, value.Change())
	default:
		return fmt.Errorf("%w: unsupported generic materialization %T", ErrUnsupportedBatch, prepared.change)
	}
}

func (builder *semanticMaterializationBuilder) appendDeclaration(
	ordinal uint64,
	declaration typedmemory.AdmittedEntityDeclaration,
) error {
	candidate, err := builder.candidateDeclaration(ordinal, declaration)
	if err != nil {
		return err
	}
	canonicalCandidate, err := candidate.CanonicalBytes()
	if err != nil {
		return fmt.Errorf("canonicalize candidate entity declaration: %w", err)
	}
	candidateDigest, err := candidate.Digest()
	if err != nil {
		return fmt.Errorf("digest candidate entity declaration: %w", err)
	}
	sqliteOrdinal, err := exactSQLiteCoordinate(ordinal, "generic declaration ordinal")
	if err != nil {
		return err
	}
	sqliteRevision, exact := sqliteIntegerFromUint64(
		builder.request.expectedRevision.Value(),
	)
	if !exact {
		return ErrRevisionOverflow
	}
	var globalCount int64
	err = builder.transaction.ScanOne(
		builder.ctx,
		`SELECT COUNT(*)
		FROM typed_memory_entities entity
		JOIN typed_memory_graph_events event
			ON event.project_id = entity.project_id
			AND event.event_ref = entity.first_declared_event_ref
		JOIN typed_memory_graph_commits commit_record
			ON commit_record.project_id = event.project_id
			AND commit_record.event_ref = event.event_ref
		WHERE entity.project_id = ? AND entity.entity_id = ?
			AND entity.first_declared_revision <= ?`,
		[]any{
			builder.request.project.String(),
			declaration.Entity().String(),
			sqliteRevision,
		},
		[]any{&globalCount},
	)
	if err != nil {
		return fmt.Errorf("inspect global entity identity: %w", err)
	}
	if globalCount > 1 {
		return ErrRevalidationRejected
	}
	if globalCount == 0 {
		builder.statements = append(builder.statements, statement{
			query: `INSERT INTO typed_memory_entities (
				project_id, entity_id, first_declared_event_ref,
				first_declared_revision, recorded_at
			) VALUES (?, ?, ?, ?, ?)`,
			arguments: []any{
				builder.request.project.String(), declaration.Entity().String(),
				builder.identity.eventRef, builder.identity.nextSQLiteRevision,
				builder.recordedAt,
			},
		})
		builder.footprint.entityCount++
	}
	builder.statements = append(builder.statements, statement{
		query: `INSERT INTO typed_memory_entity_contexts (
			project_id, entity_id, bounded_context_ref, label, provenance_ref,
			declared_event_ref, declared_revision, recorded_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		arguments: []any{
			builder.request.project.String(), declaration.Entity().String(),
			declaration.Context().String(), declaration.Label().String(),
			declaration.Provenance().String(), builder.identity.eventRef,
			builder.identity.nextSQLiteRevision, builder.recordedAt,
		},
	})
	builder.footprint.entityContextCount++
	builder.statements = append(builder.statements, statement{
		query: `INSERT INTO typed_memory_entity_declarations (
			project_id, event_ref, change_ordinal, entity_id,
			batch_local_ref, bounded_context_ref, label, provenance_ref,
			declaration_digest, canonical_declaration_bytes
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		arguments: []any{
			builder.request.project.String(), builder.identity.eventRef, sqliteOrdinal,
			candidate.Entity().String(), candidate.LocalRef().String(),
			candidate.Context().String(), candidate.Label().String(),
			candidate.Provenance().String(), candidateDigest.String(), canonicalCandidate,
		},
	})
	builder.footprint.entityDeclarationCount++
	builder.rowDigests = append(
		builder.rowDigests,
		"entity-declaration:"+candidateDigest.String(),
	)
	digest, err := digestFields(
		"typed-memory-entity-context-row.v1",
		declaration.Entity().String(),
		declaration.Context().String(),
		declaration.Label().String(),
		declaration.Provenance().String(),
	)
	if err != nil {
		return err
	}
	builder.rowDigests = append(builder.rowDigests, "entity-context:"+digest.String())
	return nil
}

func (builder *semanticMaterializationBuilder) candidateDeclaration(
	ordinal uint64,
	admitted typedmemory.AdmittedEntityDeclaration,
) (typedmemory.DeclareEntity, error) {
	changes := builder.request.candidate.Changes()
	if ordinal >= uint64(len(changes)) {
		return typedmemory.DeclareEntity{}, ErrInvalidAdmissionBatch
	}
	candidate, ok := changes[ordinal].(typedmemory.DeclareEntity)
	if !ok {
		return typedmemory.DeclareEntity{}, ErrInvalidAdmissionBatch
	}
	matches := candidate.Entity() == admitted.Entity() &&
		candidate.Context() == admitted.Context() &&
		candidate.Label() == admitted.Label() &&
		candidate.Provenance() == admitted.Provenance()
	if !matches {
		return typedmemory.DeclareEntity{}, ErrInvalidAdmissionBatch
	}
	return candidate, nil
}

func (builder *semanticMaterializationBuilder) appendIdentityChange(
	ordinal uint64,
	change typedmemory.IdentityChange,
) error {
	switch value := change.(type) {
	case typedmemory.AdmitAlias:
		canonical, err := value.CanonicalBytes()
		if err != nil {
			return err
		}
		digest, err := value.Digest()
		if err != nil {
			return err
		}
		return builder.appendAliasRow(
			ordinal,
			"admit_alias",
			value.Entity(),
			value.Alias(),
			"",
			value.Context(),
			value.Provenance(),
			"",
			canonical,
			digest,
		)
	case typedmemory.SupersedeAlias:
		previous, err := builder.loadActiveAliasChangeRef(
			value.Entity(),
			value.OldAlias(),
			value.Context(),
		)
		if err != nil {
			return err
		}
		canonical, err := value.CanonicalBytes()
		if err != nil {
			return err
		}
		digest, err := value.Digest()
		if err != nil {
			return err
		}
		return builder.appendAliasRow(
			ordinal,
			"supersede_alias",
			value.Entity(),
			value.OldAlias(),
			value.Replacement().String(),
			value.Context(),
			value.Provenance(),
			previous,
			canonical,
			digest,
		)
	case typedmemory.MergeEntities, typedmemory.SplitEntity:
		return ErrManualIdentityReconciliationRequired
	default:
		return ErrUnsupportedBatch
	}
}

func (builder *semanticMaterializationBuilder) appendAliasRow(
	ordinal uint64,
	kind string,
	entity typedmemory.EntityID,
	alias typedmemory.EntityAlias,
	replacement string,
	contextRef typedmemory.BoundedContextRef,
	provenance typedmemory.ProvenanceRef,
	supersedes string,
	canonical []byte,
	digest typedmemory.SHA256Digest,
) error {
	sqliteOrdinal, err := exactSQLiteCoordinate(ordinal, "generic alias-change ordinal")
	if err != nil {
		return err
	}
	aliasRef := derivedRef(
		"typed-memory-alias-change",
		builder.request.project.String(),
		builder.identity.eventRef,
		strconv.FormatUint(ordinal, 10),
		digest.String(),
	)
	var replacementValue any
	if replacement != "" {
		replacementValue = replacement
	}
	builder.statements = append(builder.statements, statement{
		query: `INSERT INTO typed_memory_alias_changes (
			project_id, event_ref, change_ordinal, alias_change_ref,
			change_kind, bounded_context_ref, alias, replacement_alias,
			entity_id, supersedes_alias_change_ref,
			alias_change_digest, canonical_alias_change_bytes, provenance_ref
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		arguments: []any{
			builder.request.project.String(), builder.identity.eventRef, sqliteOrdinal,
			aliasRef, kind, contextRef.String(), alias.String(), replacementValue,
			entity.String(), supersedes, digest.String(), canonical, provenance.String(),
		},
	})
	builder.footprint.aliasChangeCount++
	builder.rowDigests = append(builder.rowDigests, "alias:"+digest.String())
	return nil
}

func (builder *semanticMaterializationBuilder) loadActiveAliasChangeRef(
	entity typedmemory.EntityID,
	alias typedmemory.EntityAlias,
	contextRef typedmemory.BoundedContextRef,
) (string, error) {
	var ref string
	err := builder.transaction.ScanOne(
		builder.ctx,
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
		SELECT alias_change_ref FROM active
		WHERE entity_id = ?
			AND CASE change_kind WHEN 'admit_alias' THEN alias ELSE replacement_alias END = ?`,
		[]any{
			builder.request.project.String(), contextRef.String(),
			builder.identity.expectedSQLiteRevision, entity.String(), alias.String(),
		},
		[]any{&ref},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrRevalidationRejected
	}
	if err != nil {
		return "", fmt.Errorf("load exact superseded alias row: %w", err)
	}
	return ref, nil
}

func (builder *semanticMaterializationBuilder) appendRelation(
	ordinal uint64,
	relation typedmemory.RelationInstance,
) error {
	sqliteOrdinal, err := exactSQLiteCoordinate(ordinal, "generic relation ordinal")
	if err != nil {
		return err
	}
	if err := builder.appendContextSlice(relation.Slice()); err != nil {
		return err
	}
	canonical, err := relation.CanonicalBytes()
	if err != nil {
		return err
	}
	digest, err := relation.Digest()
	if err != nil {
		return err
	}
	builder.statements = append(builder.statements, statement{
		query: `INSERT INTO typed_memory_relation_instances (
			project_id, event_ref, change_ordinal, assertion_id,
			signature_ref, context_slice_ref, relation_digest,
			canonical_relation_bytes, provenance_ref
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		arguments: []any{
			builder.request.project.String(), builder.identity.eventRef, sqliteOrdinal,
			relation.Assertion().String(), relation.Signature().String(),
			relation.Slice().Ref().String(), digest.String(), canonical,
			relation.Provenance().String(),
		},
	})
	// The historical footprint column counts stored relational records across
	// both lanes. It does not assert that the direct relation obtains and does
	// not manufacture a world-side occurrence.
	builder.footprint.relationCount++
	builder.rowDigests = append(
		builder.rowDigests,
		legacyRelationStorageFamily.assertionDigestTag+digest.String(),
	)
	return builder.appendRelationBindings(ordinal, relation, legacyRelationStorageFamily)
}

func (builder *semanticMaterializationBuilder) appendRelationalAssertion(
	ordinal uint64,
	assertion typedmemory.RelationalAssertion,
) error {
	sqliteOrdinal, err := exactSQLiteCoordinate(ordinal, "generic relational-assertion ordinal")
	if err != nil {
		return err
	}
	if err := builder.appendContextSlice(assertion.Slice()); err != nil {
		return err
	}
	canonical, err := assertion.CanonicalBytes()
	if err != nil {
		return err
	}
	digest, err := assertion.Digest()
	if err != nil {
		return err
	}
	builder.statements = append(builder.statements, statement{
		query: `INSERT INTO typed_memory_relational_assertions_v3 (
			project_id, event_ref, change_ordinal, assertion_id,
			signature_ref, context_slice_ref, modality, assertion_digest,
			canonical_assertion_bytes, provenance_ref
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		arguments: []any{
			builder.request.project.String(), builder.identity.eventRef, sqliteOrdinal,
			assertion.Assertion().String(), assertion.Signature().String(),
			assertion.Slice().Ref().String(), assertion.Modality().Kind().String(),
			digest.String(), canonical, assertion.Provenance().String(),
		},
	})
	// This shared footprint counter records one stored assertion row. Its name
	// is historical; modality remains claim content and no occurrence is
	// inferred here.
	builder.footprint.relationCount++
	builder.rowDigests = append(
		builder.rowDigests,
		relationalAssertionStorageFamily.assertionDigestTag+digest.String(),
	)
	return builder.appendRelationBindings(
		ordinal,
		assertion,
		relationalAssertionStorageFamily,
	)
}

func (builder *semanticMaterializationBuilder) appendRelationBindings(
	ordinal uint64,
	relation materializedRelation,
	family relationStorageFamily,
) error {
	fragment, exists := builder.environment.TypedRelationDeclarationFragment(
		relation.Signature(),
	)
	if !exists {
		return ErrRevalidationRejected
	}
	for slotOrdinal, binding := range relation.Bindings() {
		slot, exists := fragment.Slot(binding.Name())
		if !exists {
			return ErrRevalidationRejected
		}
		if err := builder.appendSlot(
			ordinal,
			uint64(slotOrdinal),
			relation,
			binding,
			slot,
			family,
		); err != nil {
			return err
		}
	}
	return nil
}

func (builder *semanticMaterializationBuilder) appendContextSlice(
	slice typedmemory.ContextSlice,
) error {
	key := slice.Ref().String()
	if _, exists := builder.contexts[key]; exists {
		return nil
	}
	builder.contexts[key] = struct{}{}
	var catalogDigest string
	var catalogContext string
	var catalogBytes []byte
	err := builder.transaction.ScanOne(
		builder.ctx,
		`SELECT context_slice_digest, bounded_context_ref, canonical_context_slice_bytes
		FROM typed_memory_context_slice_catalog
		WHERE project_id = ? AND context_slice_ref = ?`,
		[]any{builder.request.project.String(), slice.Ref().String()},
		[]any{&catalogDigest, &catalogContext, &catalogBytes},
	)
	if errors.Is(err, sql.ErrNoRows) {
		builder.statements = append(builder.statements, statement{
			query: `INSERT INTO typed_memory_context_slice_catalog (
				project_id, event_ref, context_slice_ref, context_slice_digest,
				bounded_context_ref, canonical_context_slice_bytes
			) VALUES (?, ?, ?, ?, ?, ?)`,
			arguments: []any{
				builder.request.project.String(), builder.identity.eventRef,
				slice.Ref().String(), slice.Digest().String(), slice.Context().String(),
				slice.CanonicalBytes(),
			},
		})
		builder.footprint.contextSliceCatalogCount++
		builder.rowDigests = append(
			builder.rowDigests,
			"context-slice-catalog:"+slice.Digest().String(),
		)
	} else if err != nil {
		return fmt.Errorf("inspect immutable ContextSlice catalog: %w", err)
	} else if catalogDigest != slice.Digest().String() ||
		catalogContext != slice.Context().String() ||
		!bytes.Equal(catalogBytes, slice.CanonicalBytes()) {
		return fmt.Errorf("%w: ContextSlice ref conflicts with immutable catalog", ErrRevalidationRejected)
	}
	builder.statements = append(builder.statements, statement{
		query: `INSERT INTO typed_memory_context_slices (
			project_id, event_ref, context_slice_ref, context_slice_digest,
			bounded_context_ref, canonical_context_slice_bytes
		) VALUES (?, ?, ?, ?, ?, ?)`,
		arguments: []any{
			builder.request.project.String(), builder.identity.eventRef,
			slice.Ref().String(), slice.Digest().String(), slice.Context().String(),
			slice.CanonicalBytes(),
		},
	})
	builder.footprint.contextSliceCount++
	builder.rowDigests = append(builder.rowDigests, "context-slice:"+slice.Digest().String())
	return nil
}

func (builder *semanticMaterializationBuilder) appendSlot(
	changeOrdinal uint64,
	slotOrdinal uint64,
	relation materializedRelation,
	binding typedmemory.SlotBinding,
	slot typedmemory.SlotSpec,
	family relationStorageFamily,
) error {
	sqliteChangeOrdinal, err := exactSQLiteCoordinate(
		changeOrdinal,
		"generic relation change ordinal",
	)
	if err != nil {
		return err
	}
	sqliteSlotOrdinal, err := exactSQLiteCoordinate(
		slotOrdinal,
		"generic relation slot ordinal",
	)
	if err != nil {
		return err
	}
	builder.statements = append(builder.statements, statement{
		query: `INSERT INTO ` + family.slotTable + ` (
			project_id, event_ref, change_ordinal, assertion_id,
			slot_ordinal, slot_kind_ref, slot_digest, canonical_slot_bytes
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		arguments: []any{
			builder.request.project.String(), builder.identity.eventRef,
			sqliteChangeOrdinal, relation.Assertion().String(), sqliteSlotOrdinal,
			binding.Name().String(), binding.Digest().String(), binding.CanonicalBytes(),
		},
	})
	builder.footprint.relationSlotCount++
	builder.rowDigests = append(builder.rowDigests, family.slotDigestTag+binding.Digest().String())
	for fillerOrdinal, filler := range binding.Fillers() {
		if err := builder.appendFiller(
			changeOrdinal,
			slotOrdinal,
			uint64(fillerOrdinal),
			relation,
			binding.Name(),
			slot,
			filler,
			family,
		); err != nil {
			return err
		}
	}
	return nil
}

func (builder *semanticMaterializationBuilder) appendFiller(
	changeOrdinal uint64,
	slotOrdinal uint64,
	fillerOrdinal uint64,
	relation materializedRelation,
	slotKind typedmemory.SlotKindID,
	slot typedmemory.SlotSpec,
	filler typedmemory.SlotFiller,
	family relationStorageFamily,
) error {
	sqliteChangeOrdinal, err := exactSQLiteCoordinate(
		changeOrdinal,
		"generic relation-filler change ordinal",
	)
	if err != nil {
		return err
	}
	sqliteSlotOrdinal, err := exactSQLiteCoordinate(
		slotOrdinal,
		"generic relation-filler slot ordinal",
	)
	if err != nil {
		return err
	}
	sqliteFillerOrdinal, err := exactSQLiteCoordinate(
		fillerOrdinal,
		"generic relation-filler ordinal",
	)
	if err != nil {
		return err
	}
	switch value := filler.(type) {
	case typedmemory.ReferenceFiller:
		target, ok := slot.Target().(typedmemory.ReferenceSlotTarget)
		if !ok {
			return ErrRevalidationRejected
		}
		digest := value.Digest()
		builder.statements = append(builder.statements, statement{
			query: `INSERT INTO ` + family.fillerTable + ` (
				project_id, event_ref, change_ordinal, assertion_id,
				slot_ordinal, filler_ordinal, filler_kind,
				reference_kind_ref, reference_id, entity_id,
				required_value_kind_ref, value_ref,
				filler_digest, canonical_filler_bytes
			) VALUES (?, ?, ?, ?, ?, ?, 'by_reference', ?, ?, ?, ?, '', ?, ?)`,
			arguments: []any{
				builder.request.project.String(), builder.identity.eventRef,
				sqliteChangeOrdinal, relation.Assertion().String(), sqliteSlotOrdinal,
				sqliteFillerOrdinal, value.Reference().RefKind().String(),
				value.Reference().ReferenceID().String(), value.Entity().String(),
				target.ValueKind().String(), digest.String(), value.CanonicalBytes(),
			},
		})
		builder.footprint.relationFillerCount++
		builder.rowDigests = append(builder.rowDigests, family.fillerDigestTag+digest.String())
		key := relationFillerKey(
			changeOrdinal,
			relation.Assertion(),
			slotKind,
			fillerOrdinal,
		)
		if use, exists := builder.admissionUse[key]; exists {
			if use.Coordinate().FillerDigest() != digest {
				return ErrInvalidAdmissionBatch
			}
			return builder.appendReferenceAdmissionUse(slotOrdinal, use, family)
		}
		use, exists := builder.classificationAdmissionUse[key]
		if !exists || use.Coordinate().FillerDigest() != digest ||
			family != relationalAssertionStorageFamily {
			return ErrInvalidAdmissionBatch
		}
		return builder.appendClassificationReferenceAdmissionUse(
			slotOrdinal,
			use,
			family,
		)
	case typedmemory.ValueFiller:
		valueBytes := value.CanonicalBytes()
		valueDigest := value.Digest()
		verified := value.Value()
		valueRef := derivedRef(
			"typed-memory-value",
			verified.ValueKind().String(),
			verified.ValueShape().String(),
			verified.Codec().String(),
			verified.Digest().String(),
		)
		if _, exists := builder.values[valueRef]; !exists {
			builder.values[valueRef] = struct{}{}
			builder.statements = append(builder.statements, statement{
				query: `INSERT INTO typed_memory_value_blobs (
					project_id, event_ref, value_ref, value_kind_ref,
					value_shape_ref, codec_ref, value_digest, canonical_value_bytes
				) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
				arguments: []any{
					builder.request.project.String(), builder.identity.eventRef, valueRef,
					verified.ValueKind().String(), verified.ValueShape().String(),
					verified.Codec().String(), verified.Digest().String(), verified.CanonicalBytes(),
				},
			})
			builder.footprint.valueBlobCount++
			builder.rowDigests = append(builder.rowDigests, "value:"+verified.Digest().String())
		}
		builder.statements = append(builder.statements, statement{
			query: `INSERT INTO ` + family.fillerTable + ` (
				project_id, event_ref, change_ordinal, assertion_id,
				slot_ordinal, filler_ordinal, filler_kind,
				reference_kind_ref, reference_id, entity_id,
				required_value_kind_ref, value_ref,
				filler_digest, canonical_filler_bytes
			) VALUES (?, ?, ?, ?, ?, ?, 'by_value', '', '', '', '', ?, ?, ?)`,
			arguments: []any{
				builder.request.project.String(), builder.identity.eventRef,
				sqliteChangeOrdinal, relation.Assertion().String(), sqliteSlotOrdinal,
				sqliteFillerOrdinal, valueRef, valueDigest.String(), valueBytes,
			},
		})
		builder.footprint.relationFillerCount++
		builder.rowDigests = append(
			builder.rowDigests,
			family.fillerDigestTag+valueDigest.String(),
		)
		return nil
	default:
		return ErrUnsupportedBatch
	}
}

func (builder *semanticMaterializationBuilder) appendReferenceAdmissionUse(
	slotOrdinal uint64,
	use typedmemory.ReferenceFillerAdmissionUse,
	family relationStorageFamily,
) error {
	coordinate := use.Coordinate()
	sqliteCoordinate, err := newSQLiteRelationFillerCoordinate(
		coordinate,
		slotOrdinal,
	)
	if err != nil {
		return err
	}
	resolution := use.Resolution()
	var resolutionKind string
	var resolutionBasis any
	var declarationOrdinal any
	var localReferenceKind any
	var batchLocalReference any
	var declarationDigest any
	var orderedPrefixDigest any
	switch value := resolution.(type) {
	case typedmemory.SnapshotReferenceResolution:
		resolutionKind = "snapshot_reference"
		resolutionBasis = value.ResolutionBasis().String()
		if _, prospective := use.RequiredMembership().EvaluationView().(typedmemory.ProspectiveBatchView); prospective {
			return ErrInvalidAdmissionBatch
		}
	case typedmemory.SameBatchDeclarationResolution:
		resolutionKind = "same_batch_declaration"
		exactDeclarationOrdinal, conversionErr := exactSQLiteCoordinate(
			value.DeclarationChangeOrdinal(),
			"same-batch declaration ordinal",
		)
		if conversionErr != nil {
			return conversionErr
		}
		declarationOrdinal = exactDeclarationOrdinal
		localReferenceKind = value.LocalReference().RefKind().String()
		batchLocalReference = value.LocalReference().BatchLocalRef().String()
		declarationDigest = value.DeclarationDigest().String()
		prefix, err := builder.appendOrderedCandidatePrefix(coordinate.ChangeOrdinal())
		if err != nil {
			return err
		}
		orderedPrefixDigest = prefix.Digest().String()
		view, prospective := use.RequiredMembership().EvaluationView().(typedmemory.ProspectiveBatchView)
		if !prospective ||
			view.DeclarationChangeOrdinal() != value.DeclarationChangeOrdinal() ||
			view.DeclarationDigest() != value.DeclarationDigest() ||
			view.LocalReference() != value.LocalReference() ||
			view.PersistedReference() != value.PersistedReference() ||
			view.OrderedCandidatePrefix().Digest() != prefix.Digest() {
			return ErrInvalidAdmissionBatch
		}
	default:
		return ErrInvalidAdmissionBatch
	}
	builder.statements = append(builder.statements, statement{
		query: `INSERT INTO ` + family.resolutionTable + ` (
			project_id, event_ref, change_ordinal, assertion_id,
			slot_ordinal, filler_ordinal, filler_digest, entity_id,
			resolution_kind, resolution_basis_ref, declaration_change_ordinal,
			local_reference_kind_ref, batch_local_ref, declaration_digest,
			ordered_candidate_prefix_digest,
			resolution_digest, canonical_resolution_bytes
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		arguments: []any{
			builder.request.project.String(), builder.identity.eventRef,
			sqliteCoordinate.changeOrdinal, coordinate.Assertion().String(),
			sqliteCoordinate.slotOrdinal, sqliteCoordinate.fillerOrdinal,
			coordinate.FillerDigest().String(), coordinate.Entity().String(),
			resolutionKind, resolutionBasis, declarationOrdinal,
			localReferenceKind, batchLocalReference, declarationDigest,
			orderedPrefixDigest,
			resolution.Digest().String(), resolution.CanonicalBytes(),
		},
	})
	builder.footprint.referenceResolutionCount++
	builder.rowDigests = append(
		builder.rowDigests,
		family.resolutionDigestTag+resolution.Digest().String(),
	)
	if err := builder.appendMembershipEvaluation(use.RequiredMembership()); err != nil {
		return err
	}
	if err := builder.appendMembershipUse(slotOrdinal, use, nil, family); err != nil {
		return err
	}
	for _, disjoint := range use.DisjointMemberships() {
		switch exact := disjoint.(type) {
		case typedmemory.DirectNotMemberUse:
			if err := builder.appendMembershipEvaluation(exact.Judgement()); err != nil {
				return err
			}
			if err := builder.appendMembershipUse(slotOrdinal, use, exact, family); err != nil {
				return err
			}
		case typedmemory.DisjointEntailmentUse:
			if err := builder.appendDisjointEntailmentUse(
				slotOrdinal,
				use,
				exact,
				family,
			); err != nil {
				return err
			}
		default:
			return ErrInvalidAdmissionBatch
		}
	}
	return nil
}

func (builder *semanticMaterializationBuilder) appendClassificationReferenceAdmissionUse(
	slotOrdinal uint64,
	use typedmemory.ClassificationReferenceFillerAdmissionUse,
	family relationStorageFamily,
) error {
	coordinate := use.Coordinate()
	sqliteCoordinate, err := newSQLiteRelationFillerCoordinate(
		coordinate,
		slotOrdinal,
	)
	if err != nil {
		return err
	}
	resolution := use.Resolution()
	var resolutionKind string
	var resolutionBasis any
	var declarationOrdinal any
	var localReferenceKind any
	var batchLocalReference any
	var declarationDigest any
	var orderedPrefixDigest any
	switch value := resolution.(type) {
	case typedmemory.SnapshotReferenceResolution:
		resolutionKind = "snapshot_reference"
		resolutionBasis = value.ResolutionBasis().String()
	case typedmemory.SameBatchDeclarationResolution:
		resolutionKind = "same_batch_declaration"
		exactDeclarationOrdinal, conversionErr := exactSQLiteCoordinate(
			value.DeclarationChangeOrdinal(),
			"same-batch classification declaration ordinal",
		)
		if conversionErr != nil {
			return conversionErr
		}
		declarationOrdinal = exactDeclarationOrdinal
		localReferenceKind = value.LocalReference().RefKind().String()
		batchLocalReference = value.LocalReference().BatchLocalRef().String()
		declarationDigest = value.DeclarationDigest().String()
		prefix, err := builder.appendOrderedCandidatePrefix(coordinate.ChangeOrdinal())
		if err != nil {
			return err
		}
		orderedPrefixDigest = prefix.Digest().String()
	default:
		return ErrInvalidAdmissionBatch
	}
	builder.statements = append(builder.statements, statement{
		query: `INSERT INTO ` + family.resolutionTable + ` (
			project_id, event_ref, change_ordinal, assertion_id,
			slot_ordinal, filler_ordinal, filler_digest, entity_id,
			resolution_kind, resolution_basis_ref, declaration_change_ordinal,
			local_reference_kind_ref, batch_local_ref, declaration_digest,
			ordered_candidate_prefix_digest,
			resolution_digest, canonical_resolution_bytes
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		arguments: []any{
			builder.request.project.String(), builder.identity.eventRef,
			sqliteCoordinate.changeOrdinal, coordinate.Assertion().String(),
			sqliteCoordinate.slotOrdinal, sqliteCoordinate.fillerOrdinal,
			coordinate.FillerDigest().String(), coordinate.Entity().String(),
			resolutionKind, resolutionBasis, declarationOrdinal,
			localReferenceKind, batchLocalReference, declarationDigest,
			orderedPrefixDigest,
			resolution.Digest().String(), resolution.CanonicalBytes(),
		},
	})
	builder.footprint.referenceResolutionCount++
	builder.rowDigests = append(
		builder.rowDigests,
		family.resolutionDigestTag+resolution.Digest().String(),
	)
	required := use.RequiredClassification()
	if err := builder.appendKindClassificationEvaluation(required); err != nil {
		return err
	}
	if err := builder.appendKindClassificationUse(
		slotOrdinal,
		use,
		"required_true",
		"",
		required,
		required.Digest(),
		required.CanonicalBytes(),
	); err != nil {
		return err
	}
	for _, disjoint := range use.DisjointClassifications() {
		judgement := disjoint.Judgement()
		if err := builder.appendKindClassificationEvaluation(judgement); err != nil {
			return err
		}
		if err := builder.appendKindClassificationUse(
			slotOrdinal,
			use,
			"disjoint_false",
			disjoint.Constraint().String(),
			judgement,
			disjoint.Digest(),
			disjoint.CanonicalBytes(),
		); err != nil {
			return err
		}
	}
	return nil
}

func (builder *semanticMaterializationBuilder) appendKindClassificationEvaluation(
	judgement typedmemory.KindClassificationJudgement,
) error {
	key := judgement.Digest().String()
	if _, exists := builder.evaluations[key]; exists {
		return nil
	}
	settled, err := requireSettledKindClassification(judgement)
	if err != nil {
		return err
	}
	request := judgement.Request()
	candidate, exactEntity := request.Candidate().(typedmemory.ExactKindEntityCandidate)
	if !exactEntity {
		return ErrInvalidAdmissionBatch
	}
	if err := builder.appendContextSlice(request.ContextSlice()); err != nil {
		return err
	}
	featureSet := settled.basis.FeatureSet()
	evaluationRef := kindClassificationEvaluationRef(judgement)
	builder.statements = append(builder.statements, statement{
		query: `INSERT INTO ` + kindClassificationEvaluationTable54 + ` (
			project_id, event_ref, evaluation_ref, judgement_kind,
			entity_id, candidate_value_kind_ref, local_value_kind_ref,
			signature_ref, context_slice_ref, criterion_rule_ref,
			feature_set_digest, request_digest, canonical_request_bytes,
			basis_digest, canonical_basis_bytes,
			judgement_digest, canonical_judgement_bytes
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		arguments: []any{
			builder.request.project.String(), builder.identity.eventRef,
			evaluationRef, judgement.Kind().String(), candidate.EntityID().String(),
			request.Candidate().ValueKind().String(),
			request.LocalKind().ValueKind().String(),
			request.SignatureEdition().String(), request.ContextSlice().Ref().String(),
			settled.basis.Criterion().String(), featureSet.Digest().String(),
			request.Digest().String(), request.CanonicalBytes(),
			settled.basis.Digest().String(), settled.basis.CanonicalBytes(),
			judgement.Digest().String(), judgement.CanonicalBytes(),
		},
	})
	builder.footprint.kindClassificationEvaluationCount++
	builder.rowDigests = append(
		builder.rowDigests,
		kindClassificationEvaluationDigestTag54+judgement.Digest().String(),
	)
	for ordinal, feature := range featureSet.Features() {
		sourceKind := kindClassificationFeatureSourceKind(feature)
		if sourceKind == "external_blob" {
			blob, exists := builder.classificationSources[feature.Source().String()]
			if !exists || blob.Digest() != feature.SourceDigest() {
				return ErrInvalidAdmissionBatch
			}
		}
		builder.statements = append(builder.statements, statement{
			query: `INSERT INTO ` + kindClassificationFeatureTable54 + ` (
				project_id, event_ref, evaluation_ref, feature_ordinal,
				source_kind, source_ref, source_digest,
				feature_key, governor_rule_ref,
				feature_digest, canonical_feature_bytes
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			arguments: []any{
				builder.request.project.String(), builder.identity.eventRef,
				evaluationRef, int64(ordinal), sourceKind,
				feature.Source().String(), feature.SourceDigest().String(),
				feature.Key().String(), feature.Governor().String(),
				feature.Digest().String(), feature.CanonicalBytes(),
			},
		})
		builder.footprint.kindClassificationFeatureCount++
		builder.rowDigests = append(
			builder.rowDigests,
			kindClassificationFeatureDigestTag54+feature.Digest().String(),
		)
	}
	builder.evaluations[key] = struct{}{}
	return nil
}

func (builder *semanticMaterializationBuilder) appendKindClassificationUse(
	slotOrdinal uint64,
	admissionUse typedmemory.ClassificationReferenceFillerAdmissionUse,
	useKind string,
	constraint string,
	judgement typedmemory.KindClassificationJudgement,
	useDigest typedmemory.SHA256Digest,
	useBytes []byte,
) error {
	coordinate := admissionUse.Coordinate()
	sqliteCoordinate, err := newSQLiteRelationFillerCoordinate(
		coordinate,
		slotOrdinal,
	)
	if err != nil {
		return err
	}
	request := judgement.Request()
	builder.statements = append(builder.statements, statement{
		query: `INSERT INTO ` + kindClassificationUseTable54 + ` (
			project_id, event_ref, change_ordinal, assertion_id,
			slot_ordinal, filler_ordinal, filler_digest,
			use_kind, constraint_id, queried_value_kind_ref,
			request_digest, evaluation_ref, expected_judgement_kind,
			use_digest, canonical_use_bytes
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		arguments: []any{
			builder.request.project.String(), builder.identity.eventRef,
			sqliteCoordinate.changeOrdinal, coordinate.Assertion().String(),
			sqliteCoordinate.slotOrdinal, sqliteCoordinate.fillerOrdinal,
			coordinate.FillerDigest().String(), useKind, constraint,
			request.LocalKind().ValueKind().String(), request.Digest().String(),
			kindClassificationEvaluationRef(judgement), judgement.Kind().String(),
			useDigest.String(), useBytes,
		},
	})
	builder.footprint.kindClassificationUseCount++
	builder.rowDigests = append(
		builder.rowDigests,
		kindClassificationUseDigestTag54+useDigest.String(),
	)
	return nil
}

func (builder *semanticMaterializationBuilder) appendOrderedCandidatePrefix(
	endExclusive uint64,
) (typedmemory.OrderedCandidatePrefix, error) {
	sqliteEnd, err := exactSQLiteCoordinate(
		endExclusive,
		"ordered-candidate prefix end ordinal",
	)
	if err != nil {
		return typedmemory.OrderedCandidatePrefix{}, err
	}
	prefix, err := typedmemory.ComputeOrderedCandidatePrefix(
		builder.request.candidate,
		endExclusive,
	)
	if err != nil {
		return typedmemory.OrderedCandidatePrefix{}, fmt.Errorf(
			"compute exact ordered candidate prefix: %w",
			err,
		)
	}
	if existing, exists := builder.prefixes[endExclusive]; exists {
		if existing != prefix.Digest() {
			return typedmemory.OrderedCandidatePrefix{}, ErrInvalidAdmissionBatch
		}
		return prefix, nil
	}
	builder.prefixes[endExclusive] = prefix.Digest()
	builder.statements = append(builder.statements, statement{
		query: `INSERT INTO typed_memory_ordered_candidate_prefixes (
			project_id, event_ref, prefix_end_ordinal, request_digest, prefix_digest
		) VALUES (?, ?, ?, ?, ?)`,
		arguments: []any{
			builder.request.project.String(), builder.identity.eventRef,
			sqliteEnd, builder.admission.prepared.requestDigest.String(),
			prefix.Digest().String(),
		},
	})
	builder.footprint.orderedCandidatePrefixCount++
	builder.rowDigests = append(
		builder.rowDigests,
		"ordered-prefix:"+prefix.Digest().String(),
	)
	return prefix, nil
}

func (builder *semanticMaterializationBuilder) appendMembershipEvaluation(
	judgement typedmemory.DefinedMemberOfJudgement,
) error {
	key := judgement.Digest().String()
	if _, exists := builder.evaluations[key]; exists {
		return nil
	}
	builder.evaluations[key] = struct{}{}
	basis := judgement.Basis()
	query := judgement.Query()
	view := judgement.EvaluationView()
	inputs := basis.ObservableInputs()
	inputSetDigest, err := typedmemory.ComputeMemberOfObservableInputSetDigest(inputs)
	if err != nil {
		return fmt.Errorf("digest exact MemberOf observable-input set: %w", err)
	}
	var viewDeclarationOrdinal any
	var viewLocalReferenceKind any
	var viewBatchLocalReference any
	var viewDeclarationDigest any
	var viewPrefixEndOrdinal any
	var viewOrderedPrefixDigest any
	switch exact := view.(type) {
	case typedmemory.PersistedSnapshotView:
	case typedmemory.ProspectiveBatchView:
		prefix, prefixErr := builder.appendOrderedCandidatePrefix(
			exact.EvaluationChangeOrdinal(),
		)
		if prefixErr != nil {
			return prefixErr
		}
		if prefix.Digest() != exact.OrderedCandidatePrefix().Digest() {
			return ErrInvalidAdmissionBatch
		}
		sqliteDeclarationOrdinal, conversionErr := exactSQLiteCoordinate(
			exact.DeclarationChangeOrdinal(),
			"prospective view declaration ordinal",
		)
		if conversionErr != nil {
			return conversionErr
		}
		sqliteEvaluationOrdinal, conversionErr := exactSQLiteCoordinate(
			exact.EvaluationChangeOrdinal(),
			"prospective view evaluation ordinal",
		)
		if conversionErr != nil {
			return conversionErr
		}
		viewDeclarationOrdinal = sqliteDeclarationOrdinal
		viewLocalReferenceKind = exact.LocalReference().RefKind().String()
		viewBatchLocalReference = exact.LocalReference().BatchLocalRef().String()
		viewDeclarationDigest = exact.DeclarationDigest().String()
		viewPrefixEndOrdinal = sqliteEvaluationOrdinal
		viewOrderedPrefixDigest = exact.OrderedCandidatePrefix().Digest().String()
	default:
		return ErrInvalidAdmissionBatch
	}
	if err := builder.appendContextSlice(query.ContextSlice()); err != nil {
		return err
	}
	evaluationRef := derivedRef("typed-memory-memberof-evaluation", key)
	builder.statements = append(builder.statements, statement{
		query: `INSERT INTO typed_memory_memberof_evaluations (
			project_id, event_ref, evaluation_ref, judgement_kind,
			entity_id, value_kind_ref, context_slice_ref,
			evaluator_rule_ref, evaluation_provenance_ref,
			evaluation_view_kind, evaluation_view_digest,
			canonical_evaluation_view_bytes,
			view_declaration_change_ordinal, view_local_reference_kind_ref,
			view_batch_local_ref, view_declaration_digest,
			view_prefix_end_ordinal, view_ordered_candidate_prefix_digest,
			observable_input_count, observable_input_set_digest,
			query_digest, canonical_query_bytes,
			basis_digest, canonical_basis_bytes,
			judgement_digest, canonical_judgement_bytes
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		arguments: []any{
			builder.request.project.String(), builder.identity.eventRef, evaluationRef,
			judgement.Kind().String(), query.EntityID().String(), query.ValueKind().String(),
			query.ContextSlice().Ref().String(), basis.Evaluator().String(),
			basis.EvaluationProvenance().Reference().String(), view.Kind().String(),
			view.Digest().String(), view.CanonicalBytes(),
			viewDeclarationOrdinal, viewLocalReferenceKind, viewBatchLocalReference,
			viewDeclarationDigest, viewPrefixEndOrdinal, viewOrderedPrefixDigest,
			int64(len(inputs)), inputSetDigest.String(), query.Digest().String(),
			query.CanonicalBytes(), basis.Digest().String(), basis.CanonicalBytes(),
			judgement.Digest().String(), judgement.CanonicalBytes(),
		},
	})
	builder.footprint.memberOfEvaluationCount++
	builder.rowDigests = append(builder.rowDigests, "memberof:"+judgement.Digest().String())
	for ordinal, observable := range inputs {
		inputKey := evaluationRef + "\x00" + observable.Reference().String()
		if _, exists := builder.evalInputs[inputKey]; exists {
			continue
		}
		builder.evalInputs[inputKey] = struct{}{}
		builder.statements = append(builder.statements, statement{
			query: `INSERT INTO typed_memory_memberof_observable_inputs (
				project_id, event_ref, evaluation_ref, input_ordinal,
				observable_input_ref, observable_input_digest
			) VALUES (?, ?, ?, ?, ?, ?)`,
			arguments: []any{
				builder.request.project.String(), builder.identity.eventRef,
				evaluationRef, int64(ordinal), observable.Reference().String(),
				observable.Digest().String(),
			},
		})
		builder.footprint.memberOfInputCount++
	}
	return nil
}

func (builder *semanticMaterializationBuilder) appendMembershipUse(
	slotOrdinal uint64,
	use typedmemory.ReferenceFillerAdmissionUse,
	disjoint typedmemory.DisjointNotMemberUse,
	family relationStorageFamily,
) error {
	coordinate := use.Coordinate()
	sqliteCoordinate, err := newSQLiteRelationFillerCoordinate(
		coordinate,
		slotOrdinal,
	)
	if err != nil {
		return err
	}
	judgement := typedmemory.DefinedMemberOfJudgement(use.RequiredMembership())
	useKind := "required_member"
	constraint := ""
	useDigest := use.RequiredMembership().Digest()
	useBytes := use.RequiredMembership().CanonicalBytes()
	if disjoint != nil {
		judgement = disjoint.Judgement()
		useKind = "disjoint_not_member"
		constraint = disjoint.Constraint().String()
		useDigest = disjoint.Digest()
		useBytes = disjoint.CanonicalBytes()
	}
	evaluationRef := derivedRef("typed-memory-memberof-evaluation", judgement.Digest().String())
	builder.statements = append(builder.statements, statement{
		query: `INSERT INTO ` + family.memberOfUseTable + ` (
			project_id, event_ref, change_ordinal, assertion_id,
			slot_ordinal, filler_ordinal, filler_digest,
			use_kind, constraint_id, queried_value_kind_ref,
			query_digest, evaluation_ref, expected_judgement_kind,
			use_digest, canonical_use_bytes
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		arguments: []any{
			builder.request.project.String(), builder.identity.eventRef,
			sqliteCoordinate.changeOrdinal, coordinate.Assertion().String(),
			sqliteCoordinate.slotOrdinal, sqliteCoordinate.fillerOrdinal,
			coordinate.FillerDigest().String(), useKind, constraint,
			judgement.Query().ValueKind().String(), judgement.Query().Digest().String(),
			evaluationRef, judgement.Kind().String(), useDigest.String(), useBytes,
		},
	})
	builder.footprint.memberOfUseCount++
	builder.rowDigests = append(builder.rowDigests, family.memberOfUseDigestTag+useDigest.String())
	return nil
}

func (builder *semanticMaterializationBuilder) appendDisjointEntailmentUse(
	slotOrdinal uint64,
	admissionUse typedmemory.ReferenceFillerAdmissionUse,
	entailment typedmemory.DisjointEntailmentUse,
	family relationStorageFamily,
) error {
	required := admissionUse.RequiredMembership()
	supporting := entailment.SupportingMembership()
	if required.Digest() != supporting.Digest() ||
		!bytes.Equal(required.CanonicalBytes(), supporting.CanonicalBytes()) {
		return ErrInvalidAdmissionBatch
	}
	coordinate := admissionUse.Coordinate()
	sqliteCoordinate, err := newSQLiteRelationFillerCoordinate(
		coordinate,
		slotOrdinal,
	)
	if err != nil {
		return err
	}
	constraint := entailment.ConstraintRule()
	counterQuery := entailment.CounterQuery()
	supportingEvaluationRef := derivedRef(
		"typed-memory-memberof-evaluation",
		required.Digest().String(),
	)
	builder.statements = append(builder.statements, statement{
		query: `INSERT INTO ` + family.disjointnessUseTable + ` (
			project_id, event_ref, change_ordinal, assertion_id,
			slot_ordinal, filler_ordinal, filler_digest,
			constraint_id, constraint_digest, canonical_constraint_bytes,
			matched_operand_kind_id, excluded_operand_kind_id,
			counter_value_kind_ref, counter_query_digest,
			canonical_counter_query_bytes, supporting_evaluation_ref,
			use_digest, canonical_use_bytes
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		arguments: []any{
			builder.request.project.String(), builder.identity.eventRef,
			sqliteCoordinate.changeOrdinal, coordinate.Assertion().String(),
			sqliteCoordinate.slotOrdinal, sqliteCoordinate.fillerOrdinal,
			coordinate.FillerDigest().String(), constraint.ID().String(),
			entailment.ConstraintDigest().String(), constraint.CanonicalBytes(),
			entailment.MatchedOperand().String(), entailment.ExcludedOperand().String(),
			counterQuery.ValueKind().String(), counterQuery.Digest().String(),
			counterQuery.CanonicalBytes(), supportingEvaluationRef,
			entailment.Digest().String(), entailment.CanonicalBytes(),
		},
	})
	builder.footprint.memberOfUseCount++
	builder.rowDigests = append(
		builder.rowDigests,
		family.disjointnessDigestTag+entailment.Digest().String(),
	)
	return nil
}

func (builder *semanticMaterializationBuilder) appendRetraction(
	ordinal uint64,
	retraction typedmemory.RetractAssertion,
) error {
	sqliteOrdinal, err := exactSQLiteCoordinate(
		ordinal,
		"assertion retraction ordinal",
	)
	if err != nil {
		return err
	}
	canonical, err := retraction.CanonicalBytes()
	if err != nil {
		return err
	}
	digest, err := retraction.Digest()
	if err != nil {
		return err
	}
	ref := derivedRef(
		"typed-memory-assertion-retraction",
		builder.request.project.String(),
		builder.identity.eventRef,
		strconv.FormatUint(ordinal, 10),
		digest.String(),
	)
	builder.statements = append(builder.statements, statement{
		query: `INSERT INTO typed_memory_assertion_retractions (
			project_id, event_ref, change_ordinal, retraction_ref,
			assertion_id, reason, provenance_ref,
			retraction_digest, canonical_retraction_bytes
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		arguments: []any{
			builder.request.project.String(), builder.identity.eventRef, sqliteOrdinal, ref,
			retraction.Assertion().String(), retraction.Reason().String(),
			retraction.Provenance().String(), digest.String(), canonical,
		},
	})
	builder.footprint.retractionCount++
	builder.rowDigests = append(builder.rowDigests, "retraction:"+digest.String())
	return nil
}

func admissionUseCoordinateKey(coordinate typedmemory.RelationFillerCoordinate) string {
	return relationFillerKey(
		coordinate.ChangeOrdinal(),
		coordinate.Assertion(),
		coordinate.Slot(),
		coordinate.FillerOrdinal(),
	)
}

func relationFillerKey(
	changeOrdinal uint64,
	assertion typedmemory.AssertionID,
	slot typedmemory.SlotKindID,
	fillerOrdinal uint64,
) string {
	return strconv.FormatUint(changeOrdinal, 10) + "\x00" +
		assertion.String() + "\x00" + slot.String() + "\x00" +
		strconv.FormatUint(fillerOrdinal, 10)
}
