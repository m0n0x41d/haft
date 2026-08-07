package typedmemorystore

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/m0n0x41d/haft/internal/projectgraphobservation"
	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

type CurrentAssertionPosture = projectgraphobservation.CurrentAssertionPosture
type CurrentAssertionCarrierKind = projectgraphobservation.CurrentAssertionCarrierKind
type CurrentAssertionCarrier = projectgraphobservation.CurrentAssertionCarrier
type CurrentLegacyRelation = projectgraphobservation.CurrentLegacyRelation
type CurrentRelationalAssertion = projectgraphobservation.CurrentRelationalAssertion
type CurrentActiveAssertion = projectgraphobservation.CurrentActiveAssertion
type CurrentActiveAssertionSet = projectgraphobservation.CurrentActiveAssertionSet
type CurrentProjectGraphObservation = projectgraphobservation.CurrentProjectGraphObservation

const (
	CurrentLegacyRelationCarrier        = projectgraphobservation.CurrentLegacyRelationCarrier
	CurrentRelationalAssertionV3Carrier = projectgraphobservation.CurrentRelationalAssertionV3Carrier
)

// LoadCurrentGraphRevalidationBasisTx reconstructs one exact graph observation
// inside the caller-owned transaction. It never begins, commits, or rolls back
// a transaction and grants no selection or mutation authority.
func LoadCurrentGraphRevalidationBasisTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	project projectledger.ProjectID,
) (CurrentProjectGraphObservation, error) {
	head, basis, err := loadCurrentGraphSnapshotBasisTx(
		ctx,
		transaction,
		project,
	)
	if err != nil {
		return CurrentProjectGraphObservation{}, err
	}
	active, err := loadCurrentActiveAssertionsAtHeadTx(ctx, transaction, head)
	if err != nil {
		return CurrentProjectGraphObservation{}, err
	}
	observation, err := projectgraphobservation.NewCurrentProjectGraphObservation(
		basis,
		head.ActiveTypeEnv(),
		active,
	)
	if err != nil {
		return CurrentProjectGraphObservation{}, storedAdmissionIntegrity(
			"construct current project graph observation",
			err,
		)
	}
	return observation, nil
}

// LoadCurrentActiveAssertionsTx rereads the current closure and returns
// assertions only when the supplied basis is still the exact current basis.
func LoadCurrentActiveAssertionsTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	basis projecttypeenvselection.ProjectGraphSnapshotBasis,
) (CurrentActiveAssertionSet, error) {
	if err := basis.Verify(); err != nil {
		return CurrentActiveAssertionSet{}, fmt.Errorf(
			"load current active assertions: graph basis is invalid: %w",
			err,
		)
	}
	project := basis.Project()
	head, current, err := loadCurrentGraphSnapshotBasisTx(
		ctx,
		transaction,
		project,
	)
	if err != nil {
		return CurrentActiveAssertionSet{}, err
	}
	if current.Ref() != basis.Ref() {
		return CurrentActiveAssertionSet{}, ErrStaleGraphRevision
	}
	return loadCurrentActiveAssertionsAtHeadTx(ctx, transaction, head)
}

func loadCurrentGraphSnapshotBasisTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	project projectledger.ProjectID,
) (
	GraphHead,
	projecttypeenvselection.ProjectGraphSnapshotBasis,
	error,
) {
	if ctx == nil {
		return GraphHead{}, projecttypeenvselection.ProjectGraphSnapshotBasis{},
			fmt.Errorf("load current graph revalidation basis: context is required")
	}
	canonicalProject, err := projectledger.ParseProjectID(project.String())
	if err != nil || canonicalProject != project {
		return GraphHead{}, projecttypeenvselection.ProjectGraphSnapshotBasis{},
			fmt.Errorf("load current graph revalidation basis: project identity is required")
	}
	if err := requireGenericStorageCapability(ctx, transaction); err != nil {
		return GraphHead{}, projecttypeenvselection.ProjectGraphSnapshotBasis{}, err
	}
	head, err := loadHeadWithScanner(ctx, transaction, project)
	if err != nil {
		return GraphHead{}, projecttypeenvselection.ProjectGraphSnapshotBasis{}, err
	}
	if err := verifyCurrentHeadClosure(ctx, transaction, head); err != nil {
		return GraphHead{}, projecttypeenvselection.ProjectGraphSnapshotBasis{}, err
	}
	materialization, err := verifyExactV46AdmissionIntegrity(ctx, transaction, head)
	if err != nil {
		return GraphHead{}, projecttypeenvselection.ProjectGraphSnapshotBasis{}, err
	}
	closure, err := graphSnapshotClosure(head, materialization)
	if err != nil {
		return GraphHead{}, projecttypeenvselection.ProjectGraphSnapshotBasis{}, err
	}
	basis, err := projecttypeenvselection.SealProjectGraphSnapshotBasis(
		projecttypeenvselection.ProjectGraphSnapshotBasisInput{
			Project:       project,
			GraphRevision: head.Revision(),
			Closure:       closure,
		},
	)
	if err != nil {
		return GraphHead{}, projecttypeenvselection.ProjectGraphSnapshotBasis{},
			fmt.Errorf("seal current project graph snapshot basis: %w", err)
	}
	return head, basis, nil
}

func graphSnapshotClosure(
	head GraphHead,
	materialization *verifiedMaterializationClosure,
) (projecttypeenvselection.ProjectGraphClosure, error) {
	if head.Revision().Value() == 0 {
		if materialization != nil {
			return nil, storedAdmissionIntegrity(
				"empty graph has an exact materialization closure",
				nil,
			)
		}
		return projecttypeenvselection.EmptyProjectGraphClosure{}, nil
	}
	if materialization == nil {
		return nil, fmt.Errorf(
			"%w: current graph head has no exact materialization closure",
			ErrStorageGenerationUnavailable,
		)
	}
	if materialization.eventRef != head.LastEventRef() ||
		materialization.commit != head.LastCommitRef() {
		return nil, storedAdmissionIntegrity(
			"current graph materialization closure differs from head",
			nil,
		)
	}
	event, err := projecttypeenvselection.ParseGraphEventRef(materialization.eventRef)
	if err != nil {
		return nil, storedAdmissionIntegrity("current graph event reference", err)
	}
	commit, err := projecttypeenvselection.ParseGraphCommitRef(materialization.commit)
	if err != nil {
		return nil, storedAdmissionIntegrity("current graph commit reference", err)
	}
	committed, err := projecttypeenvselection.NewCommittedProjectGraphClosure(
		projecttypeenvselection.CommittedProjectGraphClosureInput{
			Event:                 event,
			Commit:                commit,
			MaterializationDigest: materialization.digest,
		},
	)
	if err != nil {
		return nil, storedAdmissionIntegrity(
			"current graph committed materialization closure",
			err,
		)
	}
	return committed, nil
}

type currentAssertionOriginLane string

const (
	currentLegacyRelationLane        currentAssertionOriginLane = "legacy_relation_instance"
	currentRelationalAssertionV3Lane currentAssertionOriginLane = "relational_assertion_v3"
)

type storedCurrentAssertionOrigin struct {
	StorageLane                currentAssertionOriginLane `json:"storage_lane"`
	EventRevision              int64                      `json:"event_revision"`
	EventExpectedRevision      int64                      `json:"event_expected_revision"`
	ChangeOrdinal              int64                      `json:"change_ordinal"`
	EventRef                   string                     `json:"event_ref"`
	AssertionID                string                     `json:"assertion_id"`
	SignatureRef               string                     `json:"signature_ref"`
	RelationDeclarationPosture string                     `json:"relation_declaration_posture"`
	ContextSliceRef            string                     `json:"context_slice_ref"`
	ExplicitModality           string                     `json:"explicit_modality"`
	CarrierDigest              string                     `json:"carrier_digest"`
	CanonicalCarrier           string                     `json:"canonical_carrier_hex"`
	ProvenanceRef              string                     `json:"provenance_ref"`
	EventBasisTypeEnv          string                     `json:"event_basis_type_env"`
	AdmissionTypeEnv           string                     `json:"admission_type_env"`
	AdmissionBasisRevision     int64                      `json:"admission_basis_revision"`
	WriterGeneration           int64                      `json:"writer_generation"`
	WriterProvenance           string                     `json:"writer_provenance"`
}

// storedCurrentRelationOrigin is retained as a compatibility adapter for the
// historical internal helper surface. The carried value is now the exact
// closed legacy/v3 assertion-origin union above.
type storedCurrentRelationOrigin = storedCurrentAssertionOrigin

type currentStoredRelationalCarrier interface {
	Assertion() typedmemory.AssertionID
	Signature() typedmemory.RelationSignatureRef
	Slice() typedmemory.ContextSlice
	Bindings() []typedmemory.SlotBinding
	Provenance() typedmemory.ProvenanceRef
}

type storedCurrentRelationSlot struct {
	SlotOrdinal int64  `json:"slot_ordinal"`
	AssertionID string `json:"assertion_id"`
	SlotKindRef string `json:"slot_kind_ref"`
	SlotDigest  string `json:"slot_digest"`
	Canonical   string `json:"canonical_slot_hex"`
}

type storedCurrentRelationFiller struct {
	SlotOrdinal                    int64  `json:"slot_ordinal"`
	FillerOrdinal                  int64  `json:"filler_ordinal"`
	AssertionID                    string `json:"assertion_id"`
	FillerKind                     string `json:"filler_kind"`
	ReferenceKindRef               string `json:"reference_kind_ref"`
	ReferenceID                    string `json:"reference_id"`
	EntityID                       string `json:"entity_id"`
	RequiredValueKind              string `json:"required_value_kind_ref"`
	RequiredMemberUseCount         int64  `json:"required_member_use_count"`
	AdmittedRequiredValueKind      string `json:"admitted_required_value_kind_ref"`
	RequiredClassificationUseCount int64  `json:"required_classification_use_count"`
	ClassifiedRequiredValueKind    string `json:"classified_required_value_kind_ref"`
	ValueRef                       string `json:"value_ref"`
	FillerDigest                   string `json:"filler_digest"`
	Canonical                      string `json:"canonical_filler_hex"`
}

func loadCurrentActiveAssertionsAtHeadTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	head GraphHead,
) (CurrentActiveAssertionSet, error) {
	states, err := loadCurrentAssertionStates(ctx, transaction, head)
	if err != nil {
		return CurrentActiveAssertionSet{}, err
	}
	origins, err := loadCurrentRelationOrigins(ctx, transaction, head)
	if err != nil {
		return CurrentActiveAssertionSet{}, err
	}
	if len(origins) != len(states) {
		return CurrentActiveAssertionSet{}, storedAdmissionIntegrity(
			"current assertion origin/state cardinality",
			nil,
		)
	}
	active := make([]CurrentActiveAssertion, 0, len(origins))
	for _, origin := range origins {
		assertionID, err := typedmemory.NewAssertionID(origin.AssertionID)
		if err != nil {
			return CurrentActiveAssertionSet{}, storedAdmissionIntegrity(
				"current assertion origin ID",
				err,
			)
		}
		if states[assertionID] != storedAssertionActive {
			continue
		}
		relation, err := decodeCurrentActiveAssertion(
			ctx,
			transaction,
			head,
			origin,
		)
		if err != nil {
			return CurrentActiveAssertionSet{}, err
		}
		active = append(active, relation)
	}
	set, err := projectgraphobservation.NewCurrentActiveAssertionSet(
		head.Project(),
		head.Revision(),
		active,
	)
	if err != nil {
		return CurrentActiveAssertionSet{}, storedAdmissionIntegrity(
			"construct current active assertion set",
			err,
		)
	}
	return set, nil
}

func loadCurrentRelationOrigins(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	head GraphHead,
) ([]storedCurrentRelationOrigin, error) {
	encoded, err := loadJSONAggregate(
		ctx,
		transaction,
		`WITH current_scope(project_id, graph_revision) AS (
			SELECT ?, ?
		)
		SELECT COALESCE(json_group_array(json_object(
			'storage_lane', storage_lane,
			'event_revision', event_revision,
			'event_expected_revision', event_expected_revision,
			'change_ordinal', change_ordinal,
			'event_ref', event_ref,
			'assertion_id', assertion_id,
			'signature_ref', signature_ref,
			'relation_declaration_posture', relation_declaration_posture,
			'context_slice_ref', context_slice_ref,
			'explicit_modality', explicit_modality,
			'carrier_digest', carrier_digest,
			'canonical_carrier_hex', canonical_carrier_hex,
			'provenance_ref', provenance_ref,
			'event_basis_type_env', event_basis_type_env,
			'admission_type_env', admission_type_env,
			'admission_basis_revision', admission_basis_revision,
			'writer_generation', writer_generation,
			'writer_provenance', writer_provenance
		)), '[]')
		FROM (
			SELECT 'legacy_relation_instance' AS storage_lane,
				event.graph_revision AS event_revision,
				event.expected_revision AS event_expected_revision,
				relation.change_ordinal AS change_ordinal,
				relation.event_ref AS event_ref,
				relation.assertion_id AS assertion_id,
				relation.signature_ref AS signature_ref,
				'typed_relation_declaration_fragment' AS relation_declaration_posture,
				relation.context_slice_ref AS context_slice_ref,
				'' AS explicit_modality,
				relation.relation_digest AS carrier_digest,
				hex(relation.canonical_relation_bytes) AS canonical_carrier_hex,
				relation.provenance_ref AS provenance_ref,
				event.basis_type_env_ref AS event_basis_type_env,
				basis.type_env_ref AS admission_type_env,
				basis.basis_graph_revision AS admission_basis_revision,
				generation.writer_generation AS writer_generation,
				generation.provenance_kind AS writer_provenance
			FROM typed_memory_relation_instances relation
			JOIN typed_memory_graph_events event
				ON event.project_id = relation.project_id
				AND event.event_ref = relation.event_ref
			JOIN typed_memory_graph_commits commit_row
				ON commit_row.project_id = event.project_id
				AND commit_row.event_ref = event.event_ref
				AND commit_row.commit_ref = event.commit_ref
			JOIN typed_memory_event_admission_bases basis
				ON basis.project_id = event.project_id
				AND basis.event_ref = event.event_ref
			JOIN typed_memory_event_writer_generations generation
				ON generation.project_id = event.project_id
				AND generation.event_ref = event.event_ref
			WHERE relation.project_id = (
					SELECT project_id FROM current_scope
				)
				AND event.graph_revision <= (
					SELECT graph_revision FROM current_scope
				)
			UNION ALL
			SELECT 'relational_assertion_v3' AS storage_lane,
				event.graph_revision AS event_revision,
				event.expected_revision AS event_expected_revision,
				assertion.change_ordinal AS change_ordinal,
				assertion.event_ref AS event_ref,
				assertion.assertion_id AS assertion_id,
				assertion.signature_ref AS signature_ref,
				'typed_relation_declaration_fragment' AS relation_declaration_posture,
				assertion.context_slice_ref AS context_slice_ref,
				assertion.modality AS explicit_modality,
				assertion.assertion_digest AS carrier_digest,
				hex(assertion.canonical_assertion_bytes) AS canonical_carrier_hex,
				assertion.provenance_ref AS provenance_ref,
				event.basis_type_env_ref AS event_basis_type_env,
				basis.type_env_ref AS admission_type_env,
				basis.basis_graph_revision AS admission_basis_revision,
				generation.writer_generation AS writer_generation,
				generation.provenance_kind AS writer_provenance
			FROM typed_memory_relational_assertions_v3 assertion
			JOIN typed_memory_graph_events event
				ON event.project_id = assertion.project_id
				AND event.event_ref = assertion.event_ref
			JOIN typed_memory_graph_commits commit_row
				ON commit_row.project_id = event.project_id
				AND commit_row.event_ref = event.event_ref
				AND commit_row.commit_ref = event.commit_ref
			JOIN typed_memory_event_admission_bases basis
				ON basis.project_id = event.project_id
				AND basis.event_ref = event.event_ref
			JOIN typed_memory_event_writer_generations generation
				ON generation.project_id = event.project_id
				AND generation.event_ref = event.event_ref
			WHERE assertion.project_id = (
					SELECT project_id FROM current_scope
				)
				AND event.graph_revision <= (
					SELECT graph_revision FROM current_scope
				)
			ORDER BY assertion_id, storage_lane
		)`,
		head,
	)
	if err != nil {
		return nil, err
	}
	rows := []storedCurrentRelationOrigin{}
	if err := json.Unmarshal([]byte(encoded), &rows); err != nil {
		return nil, storedAdmissionIntegrity("decode current relation origins", err)
	}
	seen := make(map[string]currentAssertionOriginLane, len(rows))
	for _, row := range rows {
		if err := verifyCurrentAssertionOriginCoordinates(row); err != nil {
			return nil, err
		}
		if lane, exists := seen[row.AssertionID]; exists {
			detail := "current assertion has duplicate origins"
			if lane != row.StorageLane {
				detail = "current assertion crosses legacy/v3 origin lanes"
			}
			return nil, storedAdmissionIntegrity(detail, nil)
		}
		seen[row.AssertionID] = row.StorageLane
	}
	return rows, nil
}

func verifyCurrentAssertionOriginCoordinates(
	row storedCurrentAssertionOrigin,
) error {
	if row.EventRevision <= 0 ||
		row.EventExpectedRevision < 0 ||
		row.ChangeOrdinal < 0 ||
		row.EventRef == "" ||
		row.AssertionID == "" ||
		row.RelationDeclarationPosture != typedmemory.RelationDeclarationTypedFragment.String() ||
		row.EventBasisTypeEnv == "" ||
		row.EventBasisTypeEnv != row.AdmissionTypeEnv ||
		row.EventExpectedRevision != row.AdmissionBasisRevision {
		return storedAdmissionIntegrity(
			"current assertion origin coordinates",
			nil,
		)
	}
	originTypeEnv, err := typedmemory.ParseTypeEnvRef(row.EventBasisTypeEnv)
	if err != nil || originTypeEnv.String() != row.EventBasisTypeEnv {
		return storedAdmissionIntegrity(
			"current assertion origin TypeEnv",
			err,
		)
	}
	if !currentAssertionOriginWriterMatchesLane(row) {
		return storedAdmissionIntegrity(
			"current assertion origin crosses its writer-generation lane",
			nil,
		)
	}
	return nil
}

func currentAssertionOriginWriterMatchesLane(
	row storedCurrentAssertionOrigin,
) bool {
	switch row.StorageLane {
	case currentLegacyRelationLane:
		return row.WriterGeneration == genericStorageWriterGeneration &&
			row.WriterProvenance == "writer_v46" &&
			row.ExplicitModality == ""
	case currentRelationalAssertionV3Lane:
		writer53 := row.WriterGeneration == relationalAssertionWriterGeneration &&
			row.WriterProvenance == "writer_v53"
		writer54 := row.WriterGeneration == kindClassificationWriterGeneration &&
			row.WriterProvenance == "writer_v54"
		return (writer53 || writer54) && row.ExplicitModality != ""
	default:
		return false
	}
}

func decodeCurrentActiveAssertion(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	head GraphHead,
	row storedCurrentRelationOrigin,
) (CurrentActiveAssertion, error) {
	canonical, err := hex.DecodeString(row.CanonicalCarrier)
	if err != nil {
		return CurrentActiveAssertion{}, storedAdmissionIntegrity(
			"decode current canonical assertion-carrier bytes",
			err,
		)
	}
	storedDigest, err := typedmemory.NewSHA256Digest(row.CarrierDigest)
	if err != nil {
		return CurrentActiveAssertion{}, storedAdmissionIntegrity(
			"current assertion-carrier digest",
			err,
		)
	}
	event, err := projecttypeenvselection.ParseGraphEventRef(row.EventRef)
	if err != nil {
		return CurrentActiveAssertion{}, storedAdmissionIntegrity(
			"current assertion origin event",
			err,
		)
	}
	revision, err := graphRevisionFromSQLite(row.EventRevision)
	if err != nil {
		return CurrentActiveAssertion{}, storedAdmissionIntegrity(
			"current assertion origin revision",
			err,
		)
	}
	active, err := decodeCurrentAssertionCarrier(
		ctx,
		transaction,
		head,
		row,
		canonical,
		storedDigest,
		event,
		revision,
	)
	if err != nil {
		return CurrentActiveAssertion{}, err
	}
	return active, nil
}

func decodeCurrentAssertionCarrier(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	head GraphHead,
	row storedCurrentAssertionOrigin,
	canonical []byte,
	storedDigest typedmemory.SHA256Digest,
	event projecttypeenvselection.GraphEventRef,
	revision typedmemory.GraphRevision,
) (CurrentActiveAssertion, error) {
	switch row.StorageLane {
	case currentLegacyRelationLane:
		return decodeCurrentLegacyRelationCarrier(
			ctx,
			transaction,
			head,
			row,
			canonical,
			storedDigest,
			event,
			revision,
		)
	case currentRelationalAssertionV3Lane:
		return decodeCurrentRelationalAssertionCarrier(
			ctx,
			transaction,
			head,
			row,
			canonical,
			storedDigest,
			event,
			revision,
		)
	default:
		return CurrentActiveAssertion{}, storedAdmissionIntegrity(
			"current assertion origin lane",
			nil,
		)
	}
}

func decodeCurrentLegacyRelationCarrier(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	head GraphHead,
	row storedCurrentAssertionOrigin,
	canonical []byte,
	storedDigest typedmemory.SHA256Digest,
	event projecttypeenvselection.GraphEventRef,
	revision typedmemory.GraphRevision,
) (CurrentActiveAssertion, error) {
	relation, err := typedmemory.DecodeCanonicalRelationInstance(canonical)
	if err != nil {
		return CurrentActiveAssertion{}, storedAdmissionIntegrity(
			"decode current canonical legacy relation",
			err,
		)
	}
	digest, err := verifyCurrentAssertionCarrierProjection(
		ctx,
		transaction,
		head,
		row,
		relation,
		storedDigest,
	)
	if err != nil {
		return CurrentActiveAssertion{}, err
	}
	changeOrdinal, exact := uint64FromSQLiteInteger(row.ChangeOrdinal)
	if !exact {
		return CurrentActiveAssertion{}, storedAdmissionIntegrity(
			"current legacy assertion change ordinal",
			nil,
		)
	}
	active, err := projectgraphobservation.NewCurrentActiveAssertion(
		projectgraphobservation.CurrentActiveAssertionInput{
			Relation:       relation,
			CanonicalBytes: canonical,
			Digest:         digest,
			OriginEvent:    event,
			OriginRevision: revision,
			ChangeOrdinal:  changeOrdinal,
		},
	)
	if err != nil {
		return CurrentActiveAssertion{}, storedAdmissionIntegrity(
			"construct current active legacy assertion",
			err,
		)
	}
	return active, nil
}

func decodeCurrentRelationalAssertionCarrier(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	head GraphHead,
	row storedCurrentAssertionOrigin,
	canonical []byte,
	storedDigest typedmemory.SHA256Digest,
	event projecttypeenvselection.GraphEventRef,
	revision typedmemory.GraphRevision,
) (CurrentActiveAssertion, error) {
	assertion, err := typedmemory.DecodeCanonicalRelationalAssertion(canonical)
	if err != nil {
		return CurrentActiveAssertion{}, storedAdmissionIntegrity(
			"decode current canonical v3 relational assertion",
			err,
		)
	}
	digest, err := verifyCurrentAssertionCarrierProjection(
		ctx,
		transaction,
		head,
		row,
		assertion,
		storedDigest,
	)
	if err != nil {
		return CurrentActiveAssertion{}, err
	}
	if assertion.Modality().Kind().String() != row.ExplicitModality {
		return CurrentActiveAssertion{}, storedAdmissionIntegrity(
			"current v3 assertion modality projection",
			nil,
		)
	}
	changeOrdinal, exact := uint64FromSQLiteInteger(row.ChangeOrdinal)
	if !exact {
		return CurrentActiveAssertion{}, storedAdmissionIntegrity(
			"current v3 assertion change ordinal",
			nil,
		)
	}
	active, err := projectgraphobservation.NewCurrentActiveRelationalAssertion(
		projectgraphobservation.CurrentActiveRelationalAssertionInput{
			Assertion:      assertion,
			CanonicalBytes: canonical,
			Digest:         digest,
			OriginEvent:    event,
			OriginRevision: revision,
			ChangeOrdinal:  changeOrdinal,
		},
	)
	if err != nil {
		return CurrentActiveAssertion{}, storedAdmissionIntegrity(
			"construct current active v3 assertion",
			err,
		)
	}
	return active, nil
}

func verifyCurrentAssertionCarrierProjection(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	head GraphHead,
	row storedCurrentAssertionOrigin,
	carrier currentStoredRelationalCarrier,
	storedDigest typedmemory.SHA256Digest,
) (typedmemory.SHA256Digest, error) {
	canonical, err := canonicalCurrentStoredRelationalCarrier(carrier)
	if err != nil {
		return typedmemory.SHA256Digest{}, storedAdmissionIntegrity(
			"canonicalize current assertion carrier",
			err,
		)
	}
	digest, err := digestCurrentStoredRelationalCarrier(carrier)
	if err != nil {
		return typedmemory.SHA256Digest{}, storedAdmissionIntegrity(
			"digest current assertion carrier",
			err,
		)
	}
	originTypeEnv, err := typedmemory.ParseTypeEnvRef(row.EventBasisTypeEnv)
	if err != nil {
		return typedmemory.SHA256Digest{}, storedAdmissionIntegrity(
			"current assertion origin TypeEnv",
			err,
		)
	}
	matches := carrier.Assertion().String() == row.AssertionID &&
		carrier.Signature().String() == row.SignatureRef &&
		carrier.Signature().TypeEnv() == originTypeEnv &&
		carrier.Slice().Ref().String() == row.ContextSliceRef &&
		carrier.Provenance().String() == row.ProvenanceRef &&
		digest == storedDigest
	if !matches {
		return typedmemory.SHA256Digest{}, storedAdmissionIntegrity(
			"current canonical assertion-carrier projection",
			nil,
		)
	}
	storedCanonical, err := hex.DecodeString(row.CanonicalCarrier)
	if err != nil || !bytes.Equal(canonical, storedCanonical) {
		return typedmemory.SHA256Digest{}, storedAdmissionIntegrity(
			"current canonical assertion-carrier bytes",
			err,
		)
	}
	if err := verifyCurrentRelationContextSlice(
		ctx,
		transaction,
		head.Project(),
		row.EventRef,
		carrier.Slice(),
	); err != nil {
		return typedmemory.SHA256Digest{}, err
	}
	if err := verifyCurrentRelationDecomposition(
		ctx,
		transaction,
		head.Project(),
		row,
		carrier,
	); err != nil {
		return typedmemory.SHA256Digest{}, err
	}
	return digest, nil
}

func canonicalCurrentStoredRelationalCarrier(
	carrier currentStoredRelationalCarrier,
) ([]byte, error) {
	switch value := carrier.(type) {
	case typedmemory.RelationInstance:
		return value.CanonicalBytes()
	case typedmemory.RelationalAssertion:
		return value.CanonicalBytes()
	default:
		return nil, fmt.Errorf("unsupported current assertion carrier %T", carrier)
	}
}

func digestCurrentStoredRelationalCarrier(
	carrier currentStoredRelationalCarrier,
) (typedmemory.SHA256Digest, error) {
	switch value := carrier.(type) {
	case typedmemory.RelationInstance:
		return value.Digest()
	case typedmemory.RelationalAssertion:
		return value.Digest()
	default:
		return typedmemory.SHA256Digest{}, fmt.Errorf(
			"unsupported current assertion carrier %T",
			carrier,
		)
	}
}

func verifyCurrentRelationContextSlice(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	project projectledger.ProjectID,
	eventRef string,
	slice typedmemory.ContextSlice,
) error {
	var eventDigest string
	var eventContext string
	var eventBytes []byte
	err := transaction.ScanOne(
		ctx,
		`SELECT context_slice_digest, bounded_context_ref,
			canonical_context_slice_bytes
		FROM typed_memory_context_slices
		WHERE project_id = ? AND event_ref = ? AND context_slice_ref = ?`,
		[]any{project.String(), eventRef, slice.Ref().String()},
		[]any{&eventDigest, &eventContext, &eventBytes},
	)
	if err != nil {
		return classifyCurrentProjectionScanError(
			"current relation ContextSlice row",
			err,
		)
	}
	var catalogDigest string
	var catalogContext string
	var catalogBytes []byte
	err = transaction.ScanOne(
		ctx,
		`SELECT context_slice_digest, bounded_context_ref,
			canonical_context_slice_bytes
		FROM typed_memory_context_slice_catalog
		WHERE project_id = ? AND context_slice_ref = ?`,
		[]any{project.String(), slice.Ref().String()},
		[]any{&catalogDigest, &catalogContext, &catalogBytes},
	)
	if err != nil {
		return classifyCurrentProjectionScanError(
			"current relation ContextSlice catalog",
			err,
		)
	}
	expectedBytes := slice.CanonicalBytes()
	matches := eventDigest == slice.Digest().String() &&
		catalogDigest == eventDigest &&
		eventContext == slice.Context().String() &&
		catalogContext == eventContext &&
		bytes.Equal(eventBytes, expectedBytes) &&
		bytes.Equal(catalogBytes, expectedBytes)
	if !matches {
		return storedAdmissionIntegrity("current relation ContextSlice projection", nil)
	}
	return nil
}

func classifyCurrentProjectionScanError(detail string, err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return storedAdmissionIntegrity(detail, err)
	}
	return fmt.Errorf("%s: %w", detail, err)
}

func verifyCurrentRelationDecomposition(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	project projectledger.ProjectID,
	origin storedCurrentRelationOrigin,
	relation currentStoredRelationalCarrier,
) error {
	slots, err := loadCurrentRelationSlots(ctx, transaction, project, origin)
	if err != nil {
		return err
	}
	bindings := relation.Bindings()
	if len(slots) != len(bindings) {
		return storedAdmissionIntegrity("current relation slot cardinality", nil)
	}
	for index, binding := range bindings {
		row := slots[index]
		canonical, err := hex.DecodeString(row.Canonical)
		if err != nil {
			return storedAdmissionIntegrity("decode current canonical slot bytes", err)
		}
		matches := row.SlotOrdinal == int64(index) &&
			row.AssertionID == relation.Assertion().String() &&
			row.SlotKindRef == binding.Name().String() &&
			row.SlotDigest == binding.Digest().String() &&
			bytes.Equal(canonical, binding.CanonicalBytes())
		if !matches {
			return storedAdmissionIntegrity("current relation slot projection", nil)
		}
	}
	fillers, err := loadCurrentRelationFillers(ctx, transaction, project, origin)
	if err != nil {
		return err
	}
	return verifyCurrentRelationFillerRows(relation, fillers)
}

func loadCurrentRelationSlots(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	project projectledger.ProjectID,
	origin storedCurrentRelationOrigin,
) ([]storedCurrentRelationSlot, error) {
	family, err := currentAssertionOriginStorageFamily(origin.StorageLane)
	if err != nil {
		return nil, err
	}
	var encoded string
	statement := `SELECT COALESCE(json_group_array(json_object(
		'slot_ordinal', slot_ordinal,
		'assertion_id', assertion_id,
		'slot_kind_ref', slot_kind_ref,
		'slot_digest', slot_digest,
		'canonical_slot_hex', canonical_slot_hex
	)), '[]')
	FROM (
		SELECT slot_ordinal, assertion_id, slot_kind_ref, slot_digest,
			hex(canonical_slot_bytes) AS canonical_slot_hex
		FROM ` + family.slotTable + `
		WHERE project_id = ? AND event_ref = ? AND change_ordinal = ?
		ORDER BY slot_ordinal
	)`
	err = transaction.ScanOne(
		ctx,
		statement,
		[]any{project.String(), origin.EventRef, origin.ChangeOrdinal},
		[]any{&encoded},
	)
	if err != nil {
		return nil, fmt.Errorf("load current relation slots: %w", err)
	}
	rows := []storedCurrentRelationSlot{}
	if err := json.Unmarshal([]byte(encoded), &rows); err != nil {
		return nil, storedAdmissionIntegrity("decode current relation slots", err)
	}
	return rows, nil
}

func loadCurrentRelationFillers(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	project projectledger.ProjectID,
	origin storedCurrentRelationOrigin,
) ([]storedCurrentRelationFiller, error) {
	family, err := currentAssertionOriginStorageFamily(origin.StorageLane)
	if err != nil {
		return nil, err
	}
	var encoded string
	statement := `SELECT COALESCE(json_group_array(json_object(
		'slot_ordinal', slot_ordinal,
		'filler_ordinal', filler_ordinal,
		'assertion_id', assertion_id,
		'filler_kind', filler_kind,
		'reference_kind_ref', reference_kind_ref,
		'reference_id', reference_id,
		'entity_id', entity_id,
		'required_value_kind_ref', required_value_kind_ref,
		'required_member_use_count', required_member_use_count,
		'admitted_required_value_kind_ref', admitted_required_value_kind_ref,
		'required_classification_use_count', required_classification_use_count,
		'classified_required_value_kind_ref', classified_required_value_kind_ref,
		'value_ref', value_ref,
		'filler_digest', filler_digest,
		'canonical_filler_hex', canonical_filler_hex
	)), '[]')
	FROM (
		SELECT filler.slot_ordinal, filler.filler_ordinal,
			filler.assertion_id, filler.filler_kind,
			filler.reference_kind_ref, filler.reference_id,
			filler.entity_id, filler.required_value_kind_ref,
			COALESCE(required_member.use_count, 0)
				AS required_member_use_count,
			COALESCE(required_member.queried_value_kind_ref, '')
				AS admitted_required_value_kind_ref,
			COALESCE(required_classification.use_count, 0)
				AS required_classification_use_count,
			COALESCE(required_classification.queried_value_kind_ref, '')
				AS classified_required_value_kind_ref,
			filler.value_ref, filler.filler_digest,
			hex(filler.canonical_filler_bytes) AS canonical_filler_hex
		FROM ` + family.fillerTable + ` filler
		LEFT JOIN (
			SELECT project_id, event_ref, change_ordinal, assertion_id,
				slot_ordinal, filler_ordinal, filler_digest,
				COUNT(*) AS use_count,
				MIN(queried_value_kind_ref) AS queried_value_kind_ref
			FROM ` + family.memberOfUseTable + `
			WHERE use_kind = 'required_member'
				AND project_id = ? AND event_ref = ?
				AND change_ordinal = ?
			GROUP BY project_id, event_ref, change_ordinal, assertion_id,
				slot_ordinal, filler_ordinal, filler_digest
		) required_member
			ON required_member.project_id = filler.project_id
			AND required_member.event_ref = filler.event_ref
			AND required_member.change_ordinal = filler.change_ordinal
			AND required_member.assertion_id = filler.assertion_id
			AND required_member.slot_ordinal = filler.slot_ordinal
			AND required_member.filler_ordinal = filler.filler_ordinal
			AND required_member.filler_digest = filler.filler_digest
		LEFT JOIN (
			SELECT project_id, event_ref, change_ordinal, assertion_id,
				slot_ordinal, filler_ordinal, filler_digest,
				COUNT(*) AS use_count,
				MIN(queried_value_kind_ref) AS queried_value_kind_ref
			FROM ` + family.classificationUseTable + `
			WHERE use_kind = 'required_true'
				AND project_id = ? AND event_ref = ?
				AND change_ordinal = ?
			GROUP BY project_id, event_ref, change_ordinal, assertion_id,
				slot_ordinal, filler_ordinal, filler_digest
		) required_classification
			ON required_classification.project_id = filler.project_id
			AND required_classification.event_ref = filler.event_ref
			AND required_classification.change_ordinal = filler.change_ordinal
			AND required_classification.assertion_id = filler.assertion_id
			AND required_classification.slot_ordinal = filler.slot_ordinal
			AND required_classification.filler_ordinal = filler.filler_ordinal
			AND required_classification.filler_digest = filler.filler_digest
		WHERE filler.project_id = ? AND filler.event_ref = ?
			AND filler.change_ordinal = ?
		ORDER BY filler.slot_ordinal, filler.filler_ordinal
	)`
	err = transaction.ScanOne(
		ctx,
		statement,
		[]any{
			project.String(),
			origin.EventRef,
			origin.ChangeOrdinal,
			project.String(),
			origin.EventRef,
			origin.ChangeOrdinal,
			project.String(),
			origin.EventRef,
			origin.ChangeOrdinal,
		},
		[]any{&encoded},
	)
	if err != nil {
		return nil, fmt.Errorf("load current relation fillers: %w", err)
	}
	rows := []storedCurrentRelationFiller{}
	if err := json.Unmarshal([]byte(encoded), &rows); err != nil {
		return nil, storedAdmissionIntegrity("decode current relation fillers", err)
	}
	return rows, nil
}

func verifyCurrentRelationFillerRows(
	relation currentStoredRelationalCarrier,
	rows []storedCurrentRelationFiller,
) error {
	expectedCount := 0
	for _, binding := range relation.Bindings() {
		expectedCount += len(binding.Fillers())
	}
	if len(rows) != expectedCount {
		return storedAdmissionIntegrity("current relation filler cardinality", nil)
	}
	rowIndex := 0
	for slotIndex, binding := range relation.Bindings() {
		for fillerIndex, filler := range binding.Fillers() {
			row := rows[rowIndex]
			rowIndex++
			if err := verifyCurrentRelationFillerRow(
				relation.Assertion(),
				slotIndex,
				fillerIndex,
				filler,
				row,
			); err != nil {
				return err
			}
		}
	}
	return nil
}

func currentAssertionOriginStorageFamily(
	lane currentAssertionOriginLane,
) (relationStorageFamily, error) {
	switch lane {
	case currentLegacyRelationLane:
		return legacyRelationStorageFamily, nil
	case currentRelationalAssertionV3Lane:
		return relationalAssertionStorageFamily, nil
	default:
		return relationStorageFamily{}, storedAdmissionIntegrity(
			"current assertion origin storage lane",
			nil,
		)
	}
}

func verifyCurrentRelationFillerRow(
	assertion typedmemory.AssertionID,
	slotIndex int,
	fillerIndex int,
	filler typedmemory.SlotFiller,
	row storedCurrentRelationFiller,
) error {
	canonical, err := hex.DecodeString(row.Canonical)
	if err != nil {
		return storedAdmissionIntegrity("decode current canonical filler bytes", err)
	}
	commonMatches := row.SlotOrdinal == int64(slotIndex) &&
		row.FillerOrdinal == int64(fillerIndex) &&
		row.AssertionID == assertion.String()
	if !commonMatches {
		return storedAdmissionIntegrity("current relation filler coordinate", nil)
	}
	switch value := filler.(type) {
	case typedmemory.ReferenceFiller:
		// The canonical ReferenceFiller does not carry the slot's required
		// ValueKind. Recover that coordinate independently from the exact one
		// required-member admission use authenticated by the v46 closure.
		matches := row.FillerKind == "by_reference" &&
			row.ReferenceKindRef == value.Reference().RefKind().String() &&
			row.ReferenceID == value.Reference().ReferenceID().String() &&
			row.EntityID == value.Entity().String() &&
			row.RequiredValueKind != "" &&
			currentReferenceFillerAdmissionProjectionMatches(row) &&
			row.ValueRef == "" &&
			row.FillerDigest == value.Digest().String() &&
			bytes.Equal(canonical, value.CanonicalBytes())
		if !matches {
			return storedAdmissionIntegrity(
				"current reference-filler projection",
				nil,
			)
		}
	case typedmemory.ValueFiller:
		verified := value.Value()
		expectedRef := derivedRef(
			"typed-memory-value",
			verified.ValueKind().String(),
			verified.ValueShape().String(),
			verified.Codec().String(),
			verified.Digest().String(),
		)
		matches := row.FillerKind == "by_value" &&
			row.ReferenceKindRef == "" &&
			row.ReferenceID == "" &&
			row.EntityID == "" &&
			row.RequiredValueKind == "" &&
			row.RequiredMemberUseCount == 0 &&
			row.AdmittedRequiredValueKind == "" &&
			row.RequiredClassificationUseCount == 0 &&
			row.ClassifiedRequiredValueKind == "" &&
			row.ValueRef == expectedRef &&
			row.FillerDigest == value.Digest().String() &&
			bytes.Equal(canonical, value.CanonicalBytes())
		if !matches {
			return storedAdmissionIntegrity("current value-filler projection", nil)
		}
	default:
		return storedAdmissionIntegrity("current relation filler variant", nil)
	}
	return nil
}

func currentReferenceFillerAdmissionProjectionMatches(
	row storedCurrentRelationFiller,
) bool {
	historical := row.RequiredMemberUseCount == 1 &&
		row.AdmittedRequiredValueKind == row.RequiredValueKind &&
		row.RequiredClassificationUseCount == 0 &&
		row.ClassifiedRequiredValueKind == ""
	current := row.RequiredMemberUseCount == 0 &&
		row.AdmittedRequiredValueKind == "" &&
		row.RequiredClassificationUseCount == 1 &&
		row.ClassifiedRequiredValueKind == row.RequiredValueKind
	return historical || current
}
