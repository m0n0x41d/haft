package typedmemorystore

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

// expectedMaterializationManifest is the state-independent, exact projection
// of one sealed admission onto its explicit event-local semantic storage
// family. Legacy RelationInstance rows and v3 RelationalAssertion rows remain
// disjoint in both row kinds and table coordinates. The manifest carries
// coordinates as well as semantic witnesses, so a footprint with the right
// counts but shifted ordinals or substituted view fields cannot compare equal.
//
// Global entity creation and ContextSlice catalog insertion are represented as
// conditional semantic candidates. Whether either row is inserted depends on
// the committed pre-state; the actual-manifest verifier resolves that condition
// against the sealed basis revision before comparing exact rows.
//
// AdmissionSnapshotObservations deliberately remain inside the byte-exact
// sealed admission-basis carrier. They are not independently query-visible
// storage rows, so projecting a second relational copy here would create two
// canonical forms for the same evidence. The basis digest and canonical bytes
// are the exact materialization boundary for those observations.
type expectedMaterializationManifest struct {
	requestDigest    typedmemory.SHA256Digest
	semanticDigest   typedmemory.SHA256Digest
	basisDigest      typedmemory.SHA256Digest
	basisRevision    uint64
	declarations     []expectedDeclarationCoordinate
	semanticRows     []expectedSemanticRowIdentity
	resolutions      []expectedResolutionWitness
	evaluations      []expectedEvaluationWitness
	observableInputs []expectedObservableInputTuple
	memberUses       []expectedMemberUseCoordinate
	orderedPrefixes  []expectedOrderedCandidatePrefix
	canonicalBytes   []byte
	digest           typedmemory.SHA256Digest
}

type expectedDeclarationCoordinate struct {
	changeOrdinal     uint64
	entityID          string
	batchLocalRef     string
	boundedContextRef string
	label             string
	provenanceRef     string
	declarationDigest typedmemory.SHA256Digest
	declarationBytes  []byte
	canonicalBytes    []byte
}

type expectedSemanticRowIdentity struct {
	rowKind        string
	coordinate     []string
	semanticDigest typedmemory.SHA256Digest
	semanticBytes  []byte
	conditional    bool
	canonicalBytes []byte
}

type expectedFillerCoordinate struct {
	changeOrdinal  uint64
	assertionID    string
	slotOrdinal    uint64
	fillerOrdinal  uint64
	fillerDigest   typedmemory.SHA256Digest
	canonicalBytes []byte
}

type expectedResolutionWitness struct {
	coordinate                   expectedFillerCoordinate
	entityID                     string
	resolutionKind               string
	resolutionDigest             typedmemory.SHA256Digest
	resolutionBytes              []byte
	resolutionBasisRef           string
	declarationChangeOrdinal     string
	localReferenceKindRef        string
	batchLocalRef                string
	declarationDigest            string
	orderedCandidatePrefixDigest string
	canonicalBytes               []byte
}

type expectedEvaluationWitness struct {
	evaluationRef                    string
	judgementKind                    string
	entityID                         string
	valueKindRef                     string
	contextSliceRef                  string
	evaluatorRuleRef                 string
	evaluationProvenanceRef          string
	evaluationViewKind               string
	evaluationViewDigest             typedmemory.SHA256Digest
	evaluationViewBytes              []byte
	viewDeclarationChangeOrdinal     string
	viewLocalReferenceKindRef        string
	viewBatchLocalRef                string
	viewDeclarationDigest            string
	viewPrefixEndOrdinal             string
	viewOrderedCandidatePrefixDigest string
	observableInputCount             uint64
	observableInputSetDigest         typedmemory.SHA256Digest
	queryDigest                      typedmemory.SHA256Digest
	queryBytes                       []byte
	basisDigest                      typedmemory.SHA256Digest
	basisBytes                       []byte
	judgementDigest                  typedmemory.SHA256Digest
	judgementBytes                   []byte
	canonicalBytes                   []byte
}

type expectedObservableInputTuple struct {
	evaluationRef      string
	inputOrdinal       uint64
	observableInputRef string
	observableDigest   typedmemory.SHA256Digest
	canonicalBytes     []byte
}

type expectedMemberUseCoordinate struct {
	filler                expectedFillerCoordinate
	useKind               string
	constraintID          string
	queriedValueKindRef   string
	queryDigest           typedmemory.SHA256Digest
	evaluationRef         string
	expectedJudgementKind string
	useDigest             typedmemory.SHA256Digest
	useBytes              []byte
	canonicalBytes        []byte
}

type expectedOrderedCandidatePrefix struct {
	endOrdinal     uint64
	prefixDigest   typedmemory.SHA256Digest
	prefixBytes    []byte
	canonicalBytes []byte
}

func (manifest expectedMaterializationManifest) CanonicalBytes() []byte {
	return append([]byte(nil), manifest.canonicalBytes...)
}

func (manifest expectedMaterializationManifest) Digest() typedmemory.SHA256Digest {
	return manifest.digest
}

func (manifest expectedMaterializationManifest) Declarations() []expectedDeclarationCoordinate {
	return append([]expectedDeclarationCoordinate(nil), manifest.declarations...)
}

func (manifest expectedMaterializationManifest) SemanticRows() []expectedSemanticRowIdentity {
	return append([]expectedSemanticRowIdentity(nil), manifest.semanticRows...)
}

func (manifest expectedMaterializationManifest) Resolutions() []expectedResolutionWitness {
	return append([]expectedResolutionWitness(nil), manifest.resolutions...)
}

func (manifest expectedMaterializationManifest) Evaluations() []expectedEvaluationWitness {
	return append([]expectedEvaluationWitness(nil), manifest.evaluations...)
}

func (manifest expectedMaterializationManifest) ObservableInputs() []expectedObservableInputTuple {
	return append([]expectedObservableInputTuple(nil), manifest.observableInputs...)
}

func (manifest expectedMaterializationManifest) MemberUses() []expectedMemberUseCoordinate {
	return append([]expectedMemberUseCoordinate(nil), manifest.memberUses...)
}

func (manifest expectedMaterializationManifest) OrderedPrefixes() []expectedOrderedCandidatePrefix {
	return append([]expectedOrderedCandidatePrefix(nil), manifest.orderedPrefixes...)
}

type expectedManifestBuilder struct {
	prepared            preparedAdmission
	declarations        []expectedDeclarationCoordinate
	semanticRows        []expectedSemanticRowIdentity
	resolutions         []expectedResolutionWitness
	evaluations         []expectedEvaluationWitness
	observableInputs    []expectedObservableInputTuple
	memberUses          []expectedMemberUseCoordinate
	orderedPrefixes     []expectedOrderedCandidatePrefix
	semanticRowKeys     map[string]struct{}
	contextSliceDigests map[string]typedmemory.SHA256Digest
	valueDigests        map[string]typedmemory.SHA256Digest
	observableDigests   map[string]typedmemory.SHA256Digest
	evaluationDigests   map[string]typedmemory.SHA256Digest
	prefixDigests       map[uint64]typedmemory.SHA256Digest
	referenceSlots      map[string]expectedReferenceSlot
	referenceValueKinds map[string]string
}

type expectedReferenceSlot struct {
	ordinal uint64
	family  relationStorageFamily
}

func buildExpectedMaterializationManifest(
	prepared preparedAdmission,
) (expectedMaterializationManifest, error) {
	if !prepared.batch.IsValid() || prepared.basis == nil {
		return expectedMaterializationManifest{}, ErrInvalidAdmissionBatch
	}
	if len(prepared.changes) == 0 {
		return expectedMaterializationManifest{}, ErrInvalidAdmissionBatch
	}
	candidateChanges := prepared.candidate.Changes()
	if len(candidateChanges) != len(prepared.changes) {
		return expectedMaterializationManifest{}, ErrInvalidAdmissionBatch
	}
	builder := expectedManifestBuilder{
		prepared:            prepared,
		semanticRowKeys:     make(map[string]struct{}),
		contextSliceDigests: make(map[string]typedmemory.SHA256Digest),
		valueDigests:        make(map[string]typedmemory.SHA256Digest),
		observableDigests:   make(map[string]typedmemory.SHA256Digest),
		evaluationDigests:   make(map[string]typedmemory.SHA256Digest),
		prefixDigests:       make(map[uint64]typedmemory.SHA256Digest),
		referenceSlots:      make(map[string]expectedReferenceSlot),
		referenceValueKinds: make(map[string]string),
	}
	if err := builder.indexReferenceValueKinds(); err != nil {
		return expectedMaterializationManifest{}, err
	}
	for _, change := range prepared.changes {
		if err := builder.appendChange(change); err != nil {
			return expectedMaterializationManifest{}, err
		}
	}
	if err := builder.appendAdmissionBasis(); err != nil {
		return expectedMaterializationManifest{}, err
	}
	return builder.finish()
}

func (builder *expectedManifestBuilder) indexReferenceValueKinds() error {
	switch basis := builder.prepared.basis.(type) {
	case typedmemory.SnapshotOnlyBasis:
		return nil
	case typedmemory.ContextSliceMembershipBasis:
		for _, use := range basis.ReferenceFillerAdmissionUses() {
			key := admissionUseCoordinateKey(use.Coordinate())
			valueKind := use.RequiredMembership().Query().ValueKind().String()
			if err := builder.indexReferenceValueKind(key, valueKind); err != nil {
				return err
			}
		}
		return nil
	case typedmemory.ContextSliceClassificationBasis:
		for _, use := range basis.ClassificationReferenceFillerAdmissionUses() {
			key := admissionUseCoordinateKey(use.Coordinate())
			valueKind := use.RequiredClassification().Request().LocalKind().ValueKind().String()
			if err := builder.indexReferenceValueKind(key, valueKind); err != nil {
				return err
			}
		}
		return nil
	default:
		return ErrInvalidAdmissionBatch
	}
}

func (builder *expectedManifestBuilder) indexReferenceValueKind(
	key string,
	valueKind string,
) error {
	if previous, exists := builder.referenceValueKinds[key]; exists && previous != valueKind {
		return ErrInvalidAdmissionBatch
	}
	builder.referenceValueKinds[key] = valueKind
	return nil
}

func (builder *expectedManifestBuilder) appendChange(
	prepared preparedAdmissionChange,
) error {
	switch change := prepared.change.(type) {
	case typedmemory.ValidatedDeclareEntity:
		return builder.appendDeclaration(prepared.ordinal, change.Change())
	case typedmemory.ValidatedIdentityChange:
		return builder.appendIdentityChange(prepared.ordinal, change.Change())
	case typedmemory.ValidatedRelationInstance:
		return builder.appendRelation(prepared.ordinal, change.Relation())
	case typedmemory.ValidatedRelationalAssertion:
		return builder.appendRelationalAssertion(prepared.ordinal, change.Assertion())
	case typedmemory.ValidatedRetraction:
		return builder.appendRetraction(prepared.ordinal, change.Change())
	default:
		return fmt.Errorf("%w: unsupported expected materialization %T", ErrUnsupportedBatch, prepared.change)
	}
}

func (builder *expectedManifestBuilder) appendDeclaration(
	ordinal uint64,
	admitted typedmemory.AdmittedEntityDeclaration,
) error {
	candidateChanges := builder.prepared.candidate.Changes()
	if ordinal >= uint64(len(candidateChanges)) {
		return ErrInvalidAdmissionBatch
	}
	candidate, ok := candidateChanges[ordinal].(typedmemory.DeclareEntity)
	if !ok {
		return ErrInvalidAdmissionBatch
	}
	matches := candidate.Entity() == admitted.Entity() &&
		candidate.Context() == admitted.Context() &&
		candidate.Label() == admitted.Label() &&
		candidate.Provenance() == admitted.Provenance()
	if !matches {
		return ErrInvalidAdmissionBatch
	}
	canonical, err := candidate.CanonicalBytes()
	if err != nil {
		return err
	}
	digest, err := candidate.Digest()
	if err != nil {
		return err
	}
	coordinate := newExpectedDeclarationCoordinate(ordinal, candidate, digest, canonical)
	builder.declarations = append(builder.declarations, coordinate)
	builder.addSemanticRow(newExpectedSemanticRowIdentity(
		"entity_declaration",
		[]string{strconv.FormatUint(ordinal, 10)},
		digest,
		canonical,
		false,
	))
	builder.addSemanticRow(newExpectedSemanticRowIdentity(
		"entity_context",
		[]string{
			candidate.Entity().String(),
			candidate.Context().String(),
			candidate.Label().String(),
			candidate.Provenance().String(),
		},
		digest,
		canonical,
		false,
	))
	builder.addSemanticRow(newExpectedSemanticRowIdentity(
		"global_entity_candidate",
		[]string{candidate.Entity().String()},
		digest,
		canonical,
		true,
	))
	return nil
}

func (builder *expectedManifestBuilder) appendIdentityChange(
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
		builder.addSemanticRow(newExpectedSemanticRowIdentity(
			"alias_change",
			[]string{
				strconv.FormatUint(ordinal, 10),
				"admit_alias",
				value.Context().String(),
				value.Alias().String(),
				"",
				value.Entity().String(),
				value.Provenance().String(),
			},
			digest,
			canonical,
			false,
		))
		return nil
	case typedmemory.SupersedeAlias:
		canonical, err := value.CanonicalBytes()
		if err != nil {
			return err
		}
		digest, err := value.Digest()
		if err != nil {
			return err
		}
		builder.addSemanticRow(newExpectedSemanticRowIdentity(
			"alias_change",
			[]string{
				strconv.FormatUint(ordinal, 10),
				"supersede_alias",
				value.Context().String(),
				value.OldAlias().String(),
				value.Replacement().String(),
				value.Entity().String(),
				value.Provenance().String(),
			},
			digest,
			canonical,
			false,
		))
		return nil
	case typedmemory.MergeEntities, typedmemory.SplitEntity:
		return ErrManualIdentityReconciliationRequired
	default:
		return ErrUnsupportedBatch
	}
}

func (builder *expectedManifestBuilder) appendRelation(
	ordinal uint64,
	relation typedmemory.RelationInstance,
) error {
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
	builder.addSemanticRow(newExpectedSemanticRowIdentity(
		legacyRelationStorageFamily.assertionRowKind,
		[]string{
			strconv.FormatUint(ordinal, 10),
			relation.Assertion().String(),
			relation.Signature().String(),
			relation.Slice().Ref().String(),
			relation.Provenance().String(),
		},
		digest,
		canonical,
		false,
	))
	return builder.appendRelationBindings(
		ordinal,
		relation,
		legacyRelationStorageFamily,
	)
}

func (builder *expectedManifestBuilder) appendRelationalAssertion(
	ordinal uint64,
	assertion typedmemory.RelationalAssertion,
) error {
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
	builder.addSemanticRow(newExpectedSemanticRowIdentity(
		relationalAssertionStorageFamily.assertionRowKind,
		[]string{
			strconv.FormatUint(ordinal, 10),
			assertion.Assertion().String(),
			assertion.Signature().String(),
			assertion.Slice().Ref().String(),
			assertion.Modality().Kind().String(),
			assertion.Provenance().String(),
		},
		digest,
		canonical,
		false,
	))
	return builder.appendRelationBindings(
		ordinal,
		assertion,
		relationalAssertionStorageFamily,
	)
}

func (builder *expectedManifestBuilder) appendRelationBindings(
	ordinal uint64,
	relation materializedRelation,
	family relationStorageFamily,
) error {
	for slotIndex, binding := range relation.Bindings() {
		slotOrdinal := uint64(slotIndex)
		builder.addSemanticRow(newExpectedSemanticRowIdentity(
			family.slotRowKind,
			[]string{
				strconv.FormatUint(ordinal, 10),
				relation.Assertion().String(),
				strconv.FormatUint(slotOrdinal, 10),
				binding.Name().String(),
			},
			binding.Digest(),
			binding.CanonicalBytes(),
			false,
		))
		for fillerIndex, filler := range binding.Fillers() {
			fillerOrdinal := uint64(fillerIndex)
			if err := builder.appendFiller(
				ordinal,
				relation.Assertion(),
				slotOrdinal,
				binding.Name(),
				fillerOrdinal,
				filler,
				family,
			); err != nil {
				return err
			}
		}
	}
	return nil
}

func (builder *expectedManifestBuilder) appendContextSlice(
	slice typedmemory.ContextSlice,
) error {
	key := slice.Ref().String()
	digest := slice.Digest()
	if previous, exists := builder.contextSliceDigests[key]; exists {
		if previous != digest {
			return ErrInvalidAdmissionBatch
		}
		return nil
	}
	builder.contextSliceDigests[key] = digest
	coordinate := []string{key, slice.Context().String()}
	builder.addSemanticRow(newExpectedSemanticRowIdentity(
		"context_slice",
		coordinate,
		digest,
		slice.CanonicalBytes(),
		false,
	))
	builder.addSemanticRow(newExpectedSemanticRowIdentity(
		"context_slice_catalog_candidate",
		coordinate,
		digest,
		slice.CanonicalBytes(),
		true,
	))
	return nil
}

func (builder *expectedManifestBuilder) appendFiller(
	changeOrdinal uint64,
	assertion typedmemory.AssertionID,
	slotOrdinal uint64,
	slotKind typedmemory.SlotKindID,
	fillerOrdinal uint64,
	filler typedmemory.SlotFiller,
	family relationStorageFamily,
) error {
	coordinatePrefix := []string{
		strconv.FormatUint(changeOrdinal, 10),
		assertion.String(),
		strconv.FormatUint(slotOrdinal, 10),
		strconv.FormatUint(fillerOrdinal, 10),
	}
	switch value := filler.(type) {
	case typedmemory.ReferenceFiller:
		digest := value.Digest()
		key := relationFillerKey(
			changeOrdinal,
			assertion,
			slotKind,
			fillerOrdinal,
		)
		requiredValueKind, exists := builder.referenceValueKinds[key]
		if !exists {
			return ErrInvalidAdmissionBatch
		}
		coordinate := append(append([]string(nil), coordinatePrefix...),
			"by_reference",
			value.Reference().RefKind().String(),
			value.Reference().ReferenceID().String(),
			value.Entity().String(),
			requiredValueKind,
			"",
		)
		builder.addSemanticRow(newExpectedSemanticRowIdentity(
			family.fillerRowKind,
			coordinate,
			digest,
			value.CanonicalBytes(),
			false,
		))
		if _, exists := builder.referenceSlots[key]; exists {
			return ErrInvalidAdmissionBatch
		}
		builder.referenceSlots[key] = expectedReferenceSlot{
			ordinal: slotOrdinal,
			family:  family,
		}
		return nil
	case typedmemory.ValueFiller:
		digest := value.Digest()
		verified := value.Value()
		valueRef := derivedRef(
			"typed-memory-value",
			verified.ValueKind().String(),
			verified.ValueShape().String(),
			verified.Codec().String(),
			verified.Digest().String(),
		)
		coordinate := append(append([]string(nil), coordinatePrefix...),
			"by_value", "", "", "", "", valueRef,
		)
		builder.addSemanticRow(newExpectedSemanticRowIdentity(
			family.fillerRowKind,
			coordinate,
			digest,
			value.CanonicalBytes(),
			false,
		))
		valueKey := canonicalStorageFields(
			"typed-memory-expected-value-key.v1",
			[]string{
				verified.ValueKind().String(),
				verified.ValueShape().String(),
				verified.Codec().String(),
				verified.Digest().String(),
			},
		)
		key := string(valueKey)
		if previous, exists := builder.valueDigests[key]; exists {
			if previous != verified.Digest() {
				return ErrInvalidAdmissionBatch
			}
			return nil
		}
		builder.valueDigests[key] = verified.Digest()
		builder.addSemanticRow(newExpectedSemanticRowIdentity(
			"value_blob",
			[]string{
				valueRef,
				verified.ValueKind().String(),
				verified.ValueShape().String(),
				verified.Codec().String(),
				verified.Digest().String(),
			},
			verified.Digest(),
			verified.CanonicalBytes(),
			false,
		))
		return nil
	default:
		return ErrUnsupportedBatch
	}
}

func (builder *expectedManifestBuilder) appendRetraction(
	ordinal uint64,
	change typedmemory.RetractAssertion,
) error {
	canonical, err := change.CanonicalBytes()
	if err != nil {
		return err
	}
	digest, err := change.Digest()
	if err != nil {
		return err
	}
	builder.addSemanticRow(newExpectedSemanticRowIdentity(
		"assertion_retraction",
		[]string{
			strconv.FormatUint(ordinal, 10),
			change.Assertion().String(),
			change.Reason().String(),
			change.Provenance().String(),
		},
		digest,
		canonical,
		false,
	))
	return nil
}

func (builder *expectedManifestBuilder) appendAdmissionBasis() error {
	switch basis := builder.prepared.basis.(type) {
	case typedmemory.SnapshotOnlyBasis:
		if len(builder.referenceSlots) != 0 {
			return ErrInvalidAdmissionBatch
		}
		return nil
	case typedmemory.ContextSliceMembershipBasis:
		return builder.appendMembershipAdmissionBasis(basis)
	case typedmemory.ContextSliceClassificationBasis:
		return builder.appendClassificationAdmissionBasis(basis)
	default:
		return ErrInvalidAdmissionBatch
	}
}

func (builder *expectedManifestBuilder) appendMembershipAdmissionBasis(
	membership typedmemory.ContextSliceMembershipBasis,
) error {
	uses := membership.ReferenceFillerAdmissionUses()
	if len(uses) != len(builder.referenceSlots) {
		return ErrInvalidAdmissionBatch
	}
	seen := make(map[string]struct{}, len(uses))
	for _, use := range uses {
		key := admissionUseCoordinateKey(use.Coordinate())
		slot, exists := builder.referenceSlots[key]
		if !exists {
			return ErrInvalidAdmissionBatch
		}
		if _, duplicate := seen[key]; duplicate {
			return ErrInvalidAdmissionBatch
		}
		seen[key] = struct{}{}
		if err := builder.appendReferenceUse(slot.ordinal, use, slot.family); err != nil {
			return err
		}
	}
	return nil
}

func (builder *expectedManifestBuilder) appendClassificationAdmissionBasis(
	classification typedmemory.ContextSliceClassificationBasis,
) error {
	uses := classification.ClassificationReferenceFillerAdmissionUses()
	if len(uses) != len(builder.referenceSlots) {
		return ErrInvalidAdmissionBatch
	}
	seen := make(map[string]struct{}, len(uses))
	for _, use := range uses {
		key := admissionUseCoordinateKey(use.Coordinate())
		slot, exists := builder.referenceSlots[key]
		if !exists || slot.family != relationalAssertionStorageFamily {
			return ErrInvalidAdmissionBatch
		}
		if _, duplicate := seen[key]; duplicate {
			return ErrInvalidAdmissionBatch
		}
		seen[key] = struct{}{}
		if err := builder.appendClassificationReferenceUse(
			slot.ordinal,
			use,
			slot.family,
		); err != nil {
			return err
		}
	}
	return nil
}

func (builder *expectedManifestBuilder) appendReferenceUse(
	slotOrdinal uint64,
	use typedmemory.ReferenceFillerAdmissionUse,
	family relationStorageFamily,
) error {
	coordinate := use.Coordinate()
	filler := newExpectedFillerCoordinate(
		coordinate.ChangeOrdinal(),
		coordinate.Assertion().String(),
		slotOrdinal,
		coordinate.FillerOrdinal(),
		coordinate.FillerDigest(),
	)
	witness, err := builder.newResolutionWitness(filler, use.Resolution())
	if err != nil {
		return err
	}
	builder.resolutions = append(builder.resolutions, witness)
	builder.addSemanticRow(newExpectedSemanticRowIdentity(
		family.resolutionRowKind,
		fillerCoordinateFields(filler),
		witness.resolutionDigest,
		witness.resolutionBytes,
		false,
	))
	if err := builder.appendEvaluation(use.RequiredMembership()); err != nil {
		return err
	}
	builder.appendRequiredMemberUse(filler, use.RequiredMembership(), family)
	for _, disjoint := range use.DisjointMemberships() {
		switch exact := disjoint.(type) {
		case typedmemory.DirectNotMemberUse:
			if err := builder.appendEvaluation(exact.Judgement()); err != nil {
				return err
			}
			builder.appendDisjointMemberUse(filler, exact, family)
		case typedmemory.DisjointEntailmentUse:
			if err := builder.appendDisjointEntailmentUse(
				filler,
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

func (builder *expectedManifestBuilder) appendClassificationReferenceUse(
	slotOrdinal uint64,
	use typedmemory.ClassificationReferenceFillerAdmissionUse,
	family relationStorageFamily,
) error {
	coordinate := use.Coordinate()
	filler := newExpectedFillerCoordinate(
		coordinate.ChangeOrdinal(),
		coordinate.Assertion().String(),
		slotOrdinal,
		coordinate.FillerOrdinal(),
		coordinate.FillerDigest(),
	)
	witness, err := builder.newResolutionWitness(filler, use.Resolution())
	if err != nil {
		return err
	}
	builder.resolutions = append(builder.resolutions, witness)
	builder.addSemanticRow(newExpectedSemanticRowIdentity(
		family.resolutionRowKind,
		fillerCoordinateFields(filler),
		witness.resolutionDigest,
		witness.resolutionBytes,
		false,
	))
	required := use.RequiredClassification()
	if err := builder.appendClassificationEvaluation(required); err != nil {
		return err
	}
	builder.appendClassificationUse(
		filler,
		"required_true",
		"",
		required,
		required.Digest(),
		required.CanonicalBytes(),
	)
	for _, disjoint := range use.DisjointClassifications() {
		judgement := disjoint.Judgement()
		if err := builder.appendClassificationEvaluation(judgement); err != nil {
			return err
		}
		builder.appendClassificationUse(
			filler,
			"disjoint_false",
			disjoint.Constraint().String(),
			judgement,
			disjoint.Digest(),
			disjoint.CanonicalBytes(),
		)
	}
	return nil
}

func (builder *expectedManifestBuilder) appendClassificationEvaluation(
	judgement typedmemory.KindClassificationJudgement,
) error {
	settled, err := requireSettledKindClassification(judgement)
	if err != nil {
		return err
	}
	key := judgement.Digest().String()
	if previous, exists := builder.evaluationDigests[key]; exists {
		if previous != judgement.Digest() {
			return ErrInvalidAdmissionBatch
		}
		return nil
	}
	request := judgement.Request()
	candidate, exactEntity := request.Candidate().(typedmemory.ExactKindEntityCandidate)
	if !exactEntity {
		return ErrInvalidAdmissionBatch
	}
	if err := builder.appendContextSlice(request.ContextSlice()); err != nil {
		return err
	}
	evaluationRef := kindClassificationEvaluationRef(judgement)
	featureSet := settled.basis.FeatureSet()
	coordinate := []string{
		evaluationRef,
		judgement.Kind().String(),
		candidate.EntityID().String(),
		request.Candidate().ValueKind().String(),
		request.LocalKind().ValueKind().String(),
		request.SignatureEdition().String(),
		request.ContextSlice().Ref().String(),
		settled.basis.Criterion().String(),
		featureSet.Digest().String(),
		request.Digest().String(),
		settled.basis.Digest().String(),
	}
	builder.addSemanticRow(newExpectedSemanticRowIdentity(
		kindClassificationEvaluationRowKind54,
		coordinate,
		judgement.Digest(),
		judgement.CanonicalBytes(),
		false,
	))
	for ordinal, feature := range featureSet.Features() {
		sourceKind := kindClassificationFeatureSourceKind(feature)
		if sourceKind == "external_blob" {
			builder.addSemanticRow(newExpectedSemanticRowIdentity(
				kindClassificationSourceBlobRowKind54,
				[]string{feature.Source().String()},
				feature.SourceDigest(),
				nil,
				false,
			))
		}
		builder.addSemanticRow(newExpectedSemanticRowIdentity(
			kindClassificationFeatureRowKind54,
			[]string{
				evaluationRef,
				strconv.Itoa(ordinal),
				sourceKind,
				feature.Source().String(),
				feature.SourceDigest().String(),
				feature.Key().String(),
				feature.Governor().String(),
			},
			feature.Digest(),
			feature.CanonicalBytes(),
			false,
		))
	}
	builder.evaluationDigests[key] = judgement.Digest()
	return nil
}

func (builder *expectedManifestBuilder) appendClassificationUse(
	filler expectedFillerCoordinate,
	useKind string,
	constraint string,
	judgement typedmemory.KindClassificationJudgement,
	useDigest typedmemory.SHA256Digest,
	useBytes []byte,
) {
	request := judgement.Request()
	coordinate := append(
		fillerCoordinateFields(filler),
		useKind,
		constraint,
		request.LocalKind().ValueKind().String(),
		request.Digest().String(),
		kindClassificationEvaluationRef(judgement),
		judgement.Kind().String(),
	)
	builder.addSemanticRow(newExpectedSemanticRowIdentity(
		kindClassificationUseRowKind54,
		coordinate,
		useDigest,
		useBytes,
		false,
	))
}

func (builder *expectedManifestBuilder) newResolutionWitness(
	coordinate expectedFillerCoordinate,
	resolution typedmemory.AdmissionReferenceResolution,
) (expectedResolutionWitness, error) {
	witness := expectedResolutionWitness{
		coordinate:       coordinate,
		entityID:         resolution.Entity().String(),
		resolutionDigest: resolution.Digest(),
		resolutionBytes:  resolution.CanonicalBytes(),
	}
	switch value := resolution.(type) {
	case typedmemory.SnapshotReferenceResolution:
		witness.resolutionKind = "snapshot_reference"
		witness.resolutionBasisRef = value.ResolutionBasis().String()
	case typedmemory.SameBatchDeclarationResolution:
		witness.resolutionKind = "same_batch_declaration"
		witness.declarationChangeOrdinal = strconv.FormatUint(value.DeclarationChangeOrdinal(), 10)
		witness.localReferenceKindRef = value.LocalReference().RefKind().String()
		witness.batchLocalRef = value.LocalReference().BatchLocalRef().String()
		witness.declarationDigest = value.DeclarationDigest().String()
		prefix, err := builder.appendOrderedPrefix(coordinate.changeOrdinal)
		if err != nil {
			return expectedResolutionWitness{}, err
		}
		witness.orderedCandidatePrefixDigest = prefix.prefixDigest.String()
	default:
		return expectedResolutionWitness{}, ErrInvalidAdmissionBatch
	}
	witness.canonicalBytes = canonicalResolutionWitness(witness)
	return witness, nil
}

func (builder *expectedManifestBuilder) appendEvaluation(
	judgement typedmemory.DefinedMemberOfJudgement,
) error {
	key := judgement.Digest().String()
	if previous, exists := builder.evaluationDigests[key]; exists {
		if previous != judgement.Digest() {
			return ErrInvalidAdmissionBatch
		}
		return nil
	}
	basis := judgement.Basis()
	query := judgement.Query()
	view := judgement.EvaluationView()
	inputSetDigest, err := typedmemory.ComputeMemberOfObservableInputSetDigest(
		basis.ObservableInputs(),
	)
	if err != nil {
		return err
	}
	witness := expectedEvaluationWitness{
		evaluationRef:            derivedRef("typed-memory-memberof-evaluation", key),
		judgementKind:            judgement.Kind().String(),
		entityID:                 query.EntityID().String(),
		valueKindRef:             query.ValueKind().String(),
		contextSliceRef:          query.ContextSlice().Ref().String(),
		evaluatorRuleRef:         basis.Evaluator().String(),
		evaluationProvenanceRef:  basis.EvaluationProvenance().Reference().String(),
		evaluationViewKind:       view.Kind().String(),
		evaluationViewDigest:     view.Digest(),
		evaluationViewBytes:      view.CanonicalBytes(),
		observableInputCount:     uint64(len(basis.ObservableInputs())),
		observableInputSetDigest: inputSetDigest,
		queryDigest:              query.Digest(),
		queryBytes:               query.CanonicalBytes(),
		basisDigest:              basis.Digest(),
		basisBytes:               basis.CanonicalBytes(),
		judgementDigest:          judgement.Digest(),
		judgementBytes:           judgement.CanonicalBytes(),
	}
	switch exact := view.(type) {
	case typedmemory.PersistedSnapshotView:
	case typedmemory.ProspectiveBatchView:
		prefix, prefixErr := builder.appendOrderedPrefix(exact.EvaluationChangeOrdinal())
		if prefixErr != nil {
			return prefixErr
		}
		if prefix.prefixDigest != exact.OrderedCandidatePrefix().Digest() {
			return ErrInvalidAdmissionBatch
		}
		witness.viewDeclarationChangeOrdinal = strconv.FormatUint(exact.DeclarationChangeOrdinal(), 10)
		witness.viewLocalReferenceKindRef = exact.LocalReference().RefKind().String()
		witness.viewBatchLocalRef = exact.LocalReference().BatchLocalRef().String()
		witness.viewDeclarationDigest = exact.DeclarationDigest().String()
		witness.viewPrefixEndOrdinal = strconv.FormatUint(exact.EvaluationChangeOrdinal(), 10)
		witness.viewOrderedCandidatePrefixDigest = exact.OrderedCandidatePrefix().Digest().String()
	default:
		return ErrInvalidAdmissionBatch
	}
	witness.canonicalBytes = canonicalEvaluationWitness(witness)
	builder.evaluationDigests[key] = judgement.Digest()
	builder.evaluations = append(builder.evaluations, witness)
	builder.addSemanticRow(newExpectedSemanticRowIdentity(
		"memberof_evaluation",
		[]string{witness.evaluationRef},
		judgement.Digest(),
		judgement.CanonicalBytes(),
		false,
	))
	if err := builder.appendContextSlice(query.ContextSlice()); err != nil {
		return err
	}
	for ordinal, input := range basis.ObservableInputs() {
		if err := builder.appendObservableInput(witness.evaluationRef, uint64(ordinal), input); err != nil {
			return err
		}
	}
	return nil
}

func (builder *expectedManifestBuilder) appendObservableInput(
	evaluationRef string,
	ordinal uint64,
	input typedmemory.MemberOfObservableInput,
) error {
	ref := input.Reference().String()
	if previous, exists := builder.observableDigests[ref]; exists && previous != input.Digest() {
		return ErrInvalidAdmissionBatch
	}
	builder.observableDigests[ref] = input.Digest()
	tuple := newExpectedObservableInputTuple(evaluationRef, ordinal, input)
	builder.observableInputs = append(builder.observableInputs, tuple)
	builder.addSemanticRow(newExpectedSemanticRowIdentity(
		"observable_input_blob",
		[]string{ref},
		input.Digest(),
		nil,
		false,
	))
	builder.addSemanticRow(newExpectedSemanticRowIdentity(
		"memberof_observable_input",
		[]string{evaluationRef, strconv.FormatUint(ordinal, 10)},
		input.Digest(),
		input.CanonicalBytes(),
		false,
	))
	return nil
}

func (builder *expectedManifestBuilder) appendRequiredMemberUse(
	filler expectedFillerCoordinate,
	judgement typedmemory.MemberOfMember,
	family relationStorageFamily,
) {
	coordinate := newExpectedMemberUseCoordinate(
		filler,
		"required_member",
		"",
		judgement.Query().ValueKind().String(),
		judgement.Query().Digest(),
		derivedRef("typed-memory-memberof-evaluation", judgement.Digest().String()),
		judgement.Kind().String(),
		judgement.Digest(),
		judgement.CanonicalBytes(),
	)
	builder.memberUses = append(builder.memberUses, coordinate)
	builder.addMemberUseSemanticRow(coordinate, family)
}

func (builder *expectedManifestBuilder) appendDisjointMemberUse(
	filler expectedFillerCoordinate,
	use typedmemory.DisjointNotMemberUse,
	family relationStorageFamily,
) {
	judgement := use.Judgement()
	coordinate := newExpectedMemberUseCoordinate(
		filler,
		"disjoint_not_member",
		use.Constraint().String(),
		judgement.Query().ValueKind().String(),
		judgement.Query().Digest(),
		derivedRef("typed-memory-memberof-evaluation", judgement.Digest().String()),
		judgement.Kind().String(),
		use.Digest(),
		use.CanonicalBytes(),
	)
	builder.memberUses = append(builder.memberUses, coordinate)
	builder.addMemberUseSemanticRow(coordinate, family)
}

func (builder *expectedManifestBuilder) appendDisjointEntailmentUse(
	filler expectedFillerCoordinate,
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
	constraint := entailment.ConstraintRule()
	counterQuery := entailment.CounterQuery()
	supportingEvaluationRef := derivedRef(
		"typed-memory-memberof-evaluation",
		supporting.Digest().String(),
	)
	coordinate := append(
		fillerCoordinateFields(filler),
		constraint.ID().String(),
		entailment.ConstraintDigest().String(),
		string(constraint.CanonicalBytes()),
		entailment.MatchedOperand().String(),
		entailment.ExcludedOperand().String(),
		counterQuery.ValueKind().String(),
		counterQuery.Digest().String(),
		string(counterQuery.CanonicalBytes()),
		supportingEvaluationRef,
	)
	builder.addSemanticRow(newExpectedSemanticRowIdentity(
		family.disjointnessUseRowKind,
		coordinate,
		entailment.Digest(),
		entailment.CanonicalBytes(),
		false,
	))
	return nil
}

func (builder *expectedManifestBuilder) addMemberUseSemanticRow(
	use expectedMemberUseCoordinate,
	family relationStorageFamily,
) {
	coordinate := append(
		fillerCoordinateFields(use.filler),
		use.useKind,
		use.constraintID,
		use.queriedValueKindRef,
		use.queryDigest.String(),
	)
	builder.addSemanticRow(newExpectedSemanticRowIdentity(
		family.memberOfUseRowKind,
		coordinate,
		use.useDigest,
		use.useBytes,
		false,
	))
}

func (builder *expectedManifestBuilder) appendOrderedPrefix(
	endOrdinal uint64,
) (expectedOrderedCandidatePrefix, error) {
	prefix, err := typedmemory.ComputeOrderedCandidatePrefix(
		builder.prepared.candidate,
		endOrdinal,
	)
	if err != nil {
		return expectedOrderedCandidatePrefix{}, err
	}
	if previous, exists := builder.prefixDigests[endOrdinal]; exists {
		if previous != prefix.Digest() {
			return expectedOrderedCandidatePrefix{}, ErrInvalidAdmissionBatch
		}
		for _, existing := range builder.orderedPrefixes {
			if existing.endOrdinal == endOrdinal {
				return existing, nil
			}
		}
		return expectedOrderedCandidatePrefix{}, ErrInvalidAdmissionBatch
	}
	witness := newExpectedOrderedCandidatePrefix(endOrdinal, prefix)
	builder.prefixDigests[endOrdinal] = prefix.Digest()
	builder.orderedPrefixes = append(builder.orderedPrefixes, witness)
	builder.addSemanticRow(newExpectedSemanticRowIdentity(
		"ordered_candidate_prefix",
		[]string{strconv.FormatUint(endOrdinal, 10)},
		prefix.Digest(),
		prefix.CanonicalBytes(),
		false,
	))
	return witness, nil
}

func (builder *expectedManifestBuilder) addSemanticRow(
	row expectedSemanticRowIdentity,
) {
	key := string(row.canonicalBytes)
	if _, exists := builder.semanticRowKeys[key]; exists {
		return
	}
	builder.semanticRowKeys[key] = struct{}{}
	builder.semanticRows = append(builder.semanticRows, row)
}

func (builder *expectedManifestBuilder) finish() (
	expectedMaterializationManifest,
	error,
) {
	sortManifestEntries(builder.declarations, func(value expectedDeclarationCoordinate) []byte {
		return value.canonicalBytes
	})
	sortManifestEntries(builder.semanticRows, func(value expectedSemanticRowIdentity) []byte {
		return value.canonicalBytes
	})
	sortManifestEntries(builder.resolutions, func(value expectedResolutionWitness) []byte {
		return value.canonicalBytes
	})
	sortManifestEntries(builder.evaluations, func(value expectedEvaluationWitness) []byte {
		return value.canonicalBytes
	})
	sortManifestEntries(builder.observableInputs, func(value expectedObservableInputTuple) []byte {
		return value.canonicalBytes
	})
	sortManifestEntries(builder.memberUses, func(value expectedMemberUseCoordinate) []byte {
		return value.canonicalBytes
	})
	sortManifestEntries(builder.orderedPrefixes, func(value expectedOrderedCandidatePrefix) []byte {
		return value.canonicalBytes
	})
	fields := []string{
		builder.prepared.requestDigest.String(),
		builder.prepared.semanticDigest.String(),
		builder.prepared.basis.Digest().String(),
	}
	fields = appendManifestSet(fields, "declarations", builder.declarations, func(value expectedDeclarationCoordinate) []byte {
		return value.canonicalBytes
	})
	fields = appendManifestSet(fields, "semantic_rows", builder.semanticRows, func(value expectedSemanticRowIdentity) []byte {
		return value.canonicalBytes
	})
	fields = appendManifestSet(fields, "resolutions", builder.resolutions, func(value expectedResolutionWitness) []byte {
		return value.canonicalBytes
	})
	fields = appendManifestSet(fields, "evaluations", builder.evaluations, func(value expectedEvaluationWitness) []byte {
		return value.canonicalBytes
	})
	fields = appendManifestSet(fields, "observable_inputs", builder.observableInputs, func(value expectedObservableInputTuple) []byte {
		return value.canonicalBytes
	})
	fields = appendManifestSet(fields, "member_uses", builder.memberUses, func(value expectedMemberUseCoordinate) []byte {
		return value.canonicalBytes
	})
	fields = appendManifestSet(fields, "ordered_prefixes", builder.orderedPrefixes, func(value expectedOrderedCandidatePrefix) []byte {
		return value.canonicalBytes
	})
	canonical := canonicalStorageFields(
		"typed-memory-expected-materialization-manifest.v1",
		fields,
	)
	digest, err := digestBytes(canonical)
	if err != nil {
		return expectedMaterializationManifest{}, err
	}
	return expectedMaterializationManifest{
		requestDigest:    builder.prepared.requestDigest,
		semanticDigest:   builder.prepared.semanticDigest,
		basisDigest:      builder.prepared.basis.Digest(),
		basisRevision:    builder.prepared.basis.GraphRevision().Value(),
		declarations:     append([]expectedDeclarationCoordinate(nil), builder.declarations...),
		semanticRows:     append([]expectedSemanticRowIdentity(nil), builder.semanticRows...),
		resolutions:      append([]expectedResolutionWitness(nil), builder.resolutions...),
		evaluations:      append([]expectedEvaluationWitness(nil), builder.evaluations...),
		observableInputs: append([]expectedObservableInputTuple(nil), builder.observableInputs...),
		memberUses:       append([]expectedMemberUseCoordinate(nil), builder.memberUses...),
		orderedPrefixes:  append([]expectedOrderedCandidatePrefix(nil), builder.orderedPrefixes...),
		canonicalBytes:   canonical,
		digest:           digest,
	}, nil
}

func newExpectedDeclarationCoordinate(
	ordinal uint64,
	declaration typedmemory.DeclareEntity,
	digest typedmemory.SHA256Digest,
	canonical []byte,
) expectedDeclarationCoordinate {
	coordinate := expectedDeclarationCoordinate{
		changeOrdinal:     ordinal,
		entityID:          declaration.Entity().String(),
		batchLocalRef:     declaration.LocalRef().String(),
		boundedContextRef: declaration.Context().String(),
		label:             declaration.Label().String(),
		provenanceRef:     declaration.Provenance().String(),
		declarationDigest: digest,
		declarationBytes:  append([]byte(nil), canonical...),
	}
	coordinate.canonicalBytes = canonicalDeclarationCoordinate(coordinate)
	return coordinate
}

func canonicalDeclarationCoordinate(
	declaration expectedDeclarationCoordinate,
) []byte {
	fields := []string{
		strconv.FormatUint(declaration.changeOrdinal, 10),
		declaration.entityID,
		declaration.batchLocalRef,
		declaration.boundedContextRef,
		declaration.label,
		declaration.provenanceRef,
		declaration.declarationDigest.String(),
		string(declaration.declarationBytes),
	}
	return canonicalStorageFields(
		"typed-memory-expected-declaration-coordinate.v1",
		fields,
	)
}

func newExpectedSemanticRowIdentity(
	rowKind string,
	coordinate []string,
	digest typedmemory.SHA256Digest,
	semanticBytes []byte,
	conditional bool,
) expectedSemanticRowIdentity {
	condition := "required"
	if conditional {
		condition = "conditional"
	}
	fields := []string{rowKind, condition, strconv.Itoa(len(coordinate))}
	fields = append(fields, coordinate...)
	fields = append(fields, digest.String(), string(semanticBytes))
	return expectedSemanticRowIdentity{
		rowKind:        rowKind,
		coordinate:     append([]string(nil), coordinate...),
		semanticDigest: digest,
		semanticBytes:  append([]byte(nil), semanticBytes...),
		conditional:    conditional,
		canonicalBytes: canonicalStorageFields(
			"typed-memory-expected-semantic-row.v1",
			fields,
		),
	}
}

func newExpectedFillerCoordinate(
	changeOrdinal uint64,
	assertionID string,
	slotOrdinal uint64,
	fillerOrdinal uint64,
	fillerDigest typedmemory.SHA256Digest,
) expectedFillerCoordinate {
	coordinate := expectedFillerCoordinate{
		changeOrdinal: changeOrdinal,
		assertionID:   assertionID,
		slotOrdinal:   slotOrdinal,
		fillerOrdinal: fillerOrdinal,
		fillerDigest:  fillerDigest,
	}
	coordinate.canonicalBytes = canonicalStorageFields(
		"typed-memory-expected-filler-coordinate.v1",
		fillerCoordinateFields(coordinate),
	)
	return coordinate
}

func fillerCoordinateFields(coordinate expectedFillerCoordinate) []string {
	return []string{
		strconv.FormatUint(coordinate.changeOrdinal, 10),
		coordinate.assertionID,
		strconv.FormatUint(coordinate.slotOrdinal, 10),
		strconv.FormatUint(coordinate.fillerOrdinal, 10),
		coordinate.fillerDigest.String(),
	}
}

func canonicalEvaluationWitness(witness expectedEvaluationWitness) []byte {
	fields := []string{
		witness.evaluationRef,
		witness.judgementKind,
		witness.entityID,
		witness.valueKindRef,
		witness.contextSliceRef,
		witness.evaluatorRuleRef,
		witness.evaluationProvenanceRef,
		witness.evaluationViewKind,
		witness.evaluationViewDigest.String(),
		string(witness.evaluationViewBytes),
		witness.viewDeclarationChangeOrdinal,
		witness.viewLocalReferenceKindRef,
		witness.viewBatchLocalRef,
		witness.viewDeclarationDigest,
		witness.viewPrefixEndOrdinal,
		witness.viewOrderedCandidatePrefixDigest,
		strconv.FormatUint(witness.observableInputCount, 10),
		witness.observableInputSetDigest.String(),
		witness.queryDigest.String(),
		string(witness.queryBytes),
		witness.basisDigest.String(),
		string(witness.basisBytes),
		witness.judgementDigest.String(),
		string(witness.judgementBytes),
	}
	return canonicalStorageFields(
		"typed-memory-expected-evaluation-witness.v1",
		fields,
	)
}

func canonicalResolutionWitness(witness expectedResolutionWitness) []byte {
	fields := append(
		fillerCoordinateFields(witness.coordinate),
		witness.entityID,
		witness.resolutionKind,
		witness.resolutionDigest.String(),
		string(witness.resolutionBytes),
		witness.resolutionBasisRef,
		witness.declarationChangeOrdinal,
		witness.localReferenceKindRef,
		witness.batchLocalRef,
		witness.declarationDigest,
		witness.orderedCandidatePrefixDigest,
	)
	return canonicalStorageFields(
		"typed-memory-expected-resolution-witness.v1",
		fields,
	)
}

func newExpectedObservableInputTuple(
	evaluationRef string,
	ordinal uint64,
	input typedmemory.MemberOfObservableInput,
) expectedObservableInputTuple {
	fields := []string{
		evaluationRef,
		strconv.FormatUint(ordinal, 10),
		input.Reference().String(),
		input.Digest().String(),
		string(input.CanonicalBytes()),
	}
	return expectedObservableInputTuple{
		evaluationRef:      evaluationRef,
		inputOrdinal:       ordinal,
		observableInputRef: input.Reference().String(),
		observableDigest:   input.Digest(),
		canonicalBytes: canonicalStorageFields(
			"typed-memory-expected-observable-input-tuple.v1",
			fields,
		),
	}
}

func newExpectedMemberUseCoordinate(
	filler expectedFillerCoordinate,
	useKind string,
	constraintID string,
	valueKindRef string,
	queryDigest typedmemory.SHA256Digest,
	evaluationRef string,
	judgementKind string,
	useDigest typedmemory.SHA256Digest,
	useBytes []byte,
) expectedMemberUseCoordinate {
	witness := expectedMemberUseCoordinate{
		filler:                filler,
		useKind:               useKind,
		constraintID:          constraintID,
		queriedValueKindRef:   valueKindRef,
		queryDigest:           queryDigest,
		evaluationRef:         evaluationRef,
		expectedJudgementKind: judgementKind,
		useDigest:             useDigest,
		useBytes:              append([]byte(nil), useBytes...),
	}
	witness.canonicalBytes = canonicalMemberUseCoordinate(witness)
	return witness
}

func canonicalMemberUseCoordinate(use expectedMemberUseCoordinate) []byte {
	fields := append(
		fillerCoordinateFields(use.filler),
		use.useKind,
		use.constraintID,
		use.queriedValueKindRef,
		use.queryDigest.String(),
		use.evaluationRef,
		use.expectedJudgementKind,
		use.useDigest.String(),
		string(use.useBytes),
	)
	return canonicalStorageFields(
		"typed-memory-expected-member-use-coordinate.v1",
		fields,
	)
}

func newExpectedOrderedCandidatePrefix(
	endOrdinal uint64,
	prefix typedmemory.OrderedCandidatePrefix,
) expectedOrderedCandidatePrefix {
	witness := expectedOrderedCandidatePrefix{
		endOrdinal:   endOrdinal,
		prefixDigest: prefix.Digest(),
		prefixBytes:  prefix.CanonicalBytes(),
	}
	witness.canonicalBytes = canonicalOrderedCandidatePrefix(witness)
	return witness
}

func canonicalOrderedCandidatePrefix(
	prefix expectedOrderedCandidatePrefix,
) []byte {
	fields := []string{
		strconv.FormatUint(prefix.endOrdinal, 10),
		prefix.prefixDigest.String(),
		string(prefix.prefixBytes),
	}
	return canonicalStorageFields(
		"typed-memory-expected-ordered-candidate-prefix.v1",
		fields,
	)
}

func sortManifestEntries[T any](
	values []T,
	canonical func(T) []byte,
) {
	sort.Slice(values, func(left int, right int) bool {
		leftBytes := canonical(values[left])
		rightBytes := canonical(values[right])
		return bytes.Compare(leftBytes, rightBytes) < 0
	})
}

func appendManifestSet[T any](
	fields []string,
	name string,
	values []T,
	canonical func(T) []byte,
) []string {
	result := append([]string(nil), fields...)
	result = append(result, name, strconv.Itoa(len(values)))
	for _, value := range values {
		result = append(result, string(canonical(value)))
	}
	return result
}
