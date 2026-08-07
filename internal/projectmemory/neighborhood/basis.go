package neighborhood

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const ProjectionBasisSchemaV1 = "haft.projection-basis/v1"

type ProjectionInputRef struct{ value string }
type ProjectionRef struct{ value string }
type ProjectionVersion struct{ value string }
type ProjectionCorrespondenceManifestRef struct{ value string }

func NewProjectionInputRef(raw string) (ProjectionInputRef, error) {
	value, err := exactReference("projection input", raw)
	if err != nil {
		return ProjectionInputRef{}, err
	}
	return ProjectionInputRef{value: value}, nil
}

func NewProjectionRef(raw string) (ProjectionRef, error) {
	value, err := exactReference("derived projection", raw)
	if err != nil {
		return ProjectionRef{}, err
	}
	return ProjectionRef{value: value}, nil
}

func NewProjectionVersion(raw string) (ProjectionVersion, error) {
	value, err := exactReference("derived projection version", raw)
	if err != nil {
		return ProjectionVersion{}, err
	}
	return ProjectionVersion{value: value}, nil
}

func NewProjectionCorrespondenceManifestRef(
	raw string,
) (ProjectionCorrespondenceManifestRef, error) {
	value, err := exactReference("projection correspondence manifest", raw)
	if err != nil {
		return ProjectionCorrespondenceManifestRef{}, err
	}
	return ProjectionCorrespondenceManifestRef{value: value}, nil
}

func (ref ProjectionInputRef) String() string { return ref.value }
func (ref ProjectionRef) String() string      { return ref.value }
func (version ProjectionVersion) String() string {
	return version.value
}
func (ref ProjectionCorrespondenceManifestRef) String() string {
	return ref.value
}

// CanonicalInputCoordinate identifies exact already-loaded canonical bytes.
type CanonicalInputCoordinate struct {
	ref    ProjectionInputRef
	digest typedmemory.SHA256Digest
}

func NewCanonicalInputCoordinate(
	ref ProjectionInputRef,
	digest typedmemory.SHA256Digest,
) (CanonicalInputCoordinate, error) {
	coordinate := CanonicalInputCoordinate{
		ref:    ref,
		digest: digest,
	}
	if !coordinate.Valid() {
		return CanonicalInputCoordinate{}, fmt.Errorf(
			"canonical projection input coordinate is invalid",
		)
	}
	return coordinate, nil
}

func (coordinate CanonicalInputCoordinate) Ref() ProjectionInputRef {
	return coordinate.ref
}

func (coordinate CanonicalInputCoordinate) Digest() typedmemory.SHA256Digest {
	return coordinate.digest
}

func (coordinate CanonicalInputCoordinate) Valid() bool {
	parsed, err := NewProjectionInputRef(coordinate.ref.String())
	digest, digestErr := typedmemory.NewSHA256Digest(coordinate.digest.String())
	return err == nil &&
		digestErr == nil &&
		parsed == coordinate.ref &&
		digest == coordinate.digest
}

func (coordinate CanonicalInputCoordinate) key() string {
	return "canonical:" + coordinate.ref.String() + "@" + coordinate.digest.String()
}

// DerivedProjectionCoordinate pins one replaceable read projection.
type DerivedProjectionCoordinate struct {
	ref     ProjectionRef
	version ProjectionVersion
	epoch   uint64
	digest  typedmemory.SHA256Digest
}

func NewDerivedProjectionCoordinate(
	ref ProjectionRef,
	version ProjectionVersion,
	epoch uint64,
	digest typedmemory.SHA256Digest,
) (DerivedProjectionCoordinate, error) {
	coordinate := DerivedProjectionCoordinate{
		ref:     ref,
		version: version,
		epoch:   epoch,
		digest:  digest,
	}
	if !coordinate.Valid() {
		return DerivedProjectionCoordinate{}, fmt.Errorf(
			"derived projection coordinate is invalid",
		)
	}
	return coordinate, nil
}

func (coordinate DerivedProjectionCoordinate) Ref() ProjectionRef {
	return coordinate.ref
}

func (coordinate DerivedProjectionCoordinate) Version() ProjectionVersion {
	return coordinate.version
}

func (coordinate DerivedProjectionCoordinate) Epoch() uint64 {
	return coordinate.epoch
}

func (coordinate DerivedProjectionCoordinate) Digest() typedmemory.SHA256Digest {
	return coordinate.digest
}

func (coordinate DerivedProjectionCoordinate) Valid() bool {
	ref, refErr := NewProjectionRef(coordinate.ref.String())
	version, versionErr := NewProjectionVersion(coordinate.version.String())
	digest, digestErr := typedmemory.NewSHA256Digest(coordinate.digest.String())
	return refErr == nil &&
		versionErr == nil &&
		digestErr == nil &&
		coordinate.epoch > 0 &&
		ref == coordinate.ref &&
		version == coordinate.version &&
		digest == coordinate.digest
}

func (coordinate DerivedProjectionCoordinate) key() string {
	return strings.Join(
		[]string{
			"derived",
			coordinate.ref.String(),
			coordinate.version.String(),
			fmt.Sprintf("%d", coordinate.epoch),
			coordinate.digest.String(),
		},
		":",
	)
}

// ProjectionInputCoordinate is the exact item-local coordinate read by one
// conservative transform. Its key must occur in the whole ProjectionBasis.
type ProjectionInputCoordinate struct {
	ref    ProjectionInputRef
	digest typedmemory.SHA256Digest
}

func NewProjectionInputCoordinate(
	ref ProjectionInputRef,
	digest typedmemory.SHA256Digest,
) (ProjectionInputCoordinate, error) {
	coordinate := ProjectionInputCoordinate{
		ref:    ref,
		digest: digest,
	}
	if !coordinate.Valid() {
		return ProjectionInputCoordinate{}, fmt.Errorf(
			"projection item input coordinate is invalid",
		)
	}
	return coordinate, nil
}

func (coordinate ProjectionInputCoordinate) Ref() ProjectionInputRef {
	return coordinate.ref
}

func (coordinate ProjectionInputCoordinate) Digest() typedmemory.SHA256Digest {
	return coordinate.digest
}

func (coordinate ProjectionInputCoordinate) Valid() bool {
	canonical, err := NewCanonicalInputCoordinate(
		coordinate.ref,
		coordinate.digest,
	)
	return err == nil && canonical.Valid()
}

func (coordinate ProjectionInputCoordinate) key() string {
	return coordinate.ref.String() + "@" + coordinate.digest.String()
}

type OutputCoordinateScope string

const (
	OutputRootItem  OutputCoordinateScope = "root"
	OutputFacetItem OutputCoordinateScope = "facet_item"
)

// OutputItemCoordinate keeps the root and facet-item namespaces disjoint.
type OutputItemCoordinate struct {
	scope     OutputCoordinateScope
	facet     FacetKind
	reference typedmemory.PersistedRef
}

func NewRootOutputCoordinate(
	reference typedmemory.PersistedRef,
) (OutputItemCoordinate, error) {
	coordinate := OutputItemCoordinate{
		scope:     OutputRootItem,
		reference: reference,
	}
	if !coordinate.Valid() {
		return OutputItemCoordinate{}, fmt.Errorf(
			"root output coordinate is invalid",
		)
	}
	return coordinate, nil
}

func NewFacetOutputCoordinate(
	facet FacetKind,
	reference typedmemory.PersistedRef,
) (OutputItemCoordinate, error) {
	coordinate := OutputItemCoordinate{
		scope:     OutputFacetItem,
		facet:     facet,
		reference: reference,
	}
	if !coordinate.Valid() {
		return OutputItemCoordinate{}, fmt.Errorf(
			"facet output coordinate is invalid",
		)
	}
	return coordinate, nil
}

func (coordinate OutputItemCoordinate) Scope() OutputCoordinateScope {
	return coordinate.scope
}

func (coordinate OutputItemCoordinate) Facet() (FacetKind, bool) {
	return coordinate.facet, coordinate.scope == OutputFacetItem
}

func (coordinate OutputItemCoordinate) Reference() typedmemory.PersistedRef {
	return coordinate.reference
}

func (coordinate OutputItemCoordinate) Valid() bool {
	referenceValid := validPersistedRef(coordinate.reference)
	rootValid := coordinate.scope == OutputRootItem && coordinate.facet == ""
	facetValid := coordinate.scope == OutputFacetItem && coordinate.facet.Valid()
	return referenceValid && (rootValid || facetValid)
}

func (coordinate OutputItemCoordinate) key() string {
	facet := "-"
	if coordinate.scope == OutputFacetItem {
		facet = string(coordinate.facet)
	}
	return strings.Join(
		[]string{
			string(coordinate.scope),
			facet,
			coordinate.reference.RefKind().String(),
			coordinate.reference.ReferenceID().String(),
		},
		":",
	)
}

type ConservativeTransformKind string

const (
	TransformIdentity                   ConservativeTransformKind = "identity"
	TransformFieldSelection             ConservativeTransformKind = "field_selection"
	TransformExactSourceExcerpt         ConservativeTransformKind = "exact_source_excerpt"
	TransformStablePresentationOrdering ConservativeTransformKind = "stable_presentation_ordering"
	TransformExactCountMetadata         ConservativeTransformKind = "exact_count_metadata"
	TransformExactGroupMetadata         ConservativeTransformKind = "exact_group_metadata"
)

var knownConservativeTransforms = []ConservativeTransformKind{
	TransformExactCountMetadata,
	TransformExactGroupMetadata,
	TransformExactSourceExcerpt,
	TransformFieldSelection,
	TransformIdentity,
	TransformStablePresentationOrdering,
}

func (kind ConservativeTransformKind) Valid() bool {
	return slices.Contains(knownConservativeTransforms, kind)
}

type ProjectionItemBasisKind string

const (
	ItemBasisDirect         ProjectionItemBasisKind = "direct"
	ItemBasisCorrespondence ProjectionItemBasisKind = "correspondence"
)

type ProjectionItemBasis interface {
	Kind() ProjectionItemBasisKind
	Output() OutputItemCoordinate
	Inputs() []ProjectionInputCoordinate
	Transform() ConservativeTransformKind
	IntentionalLosses() []IntentionalLossKind
	isProjectionItemBasis()
}

type DirectProjectionItemBasis struct {
	output    OutputItemCoordinate
	inputs    []ProjectionInputCoordinate
	transform ConservativeTransformKind
	losses    []IntentionalLossKind
}

func NewDirectProjectionItemBasis(
	output OutputItemCoordinate,
	inputs []ProjectionInputCoordinate,
	transform ConservativeTransformKind,
	losses []IntentionalLossKind,
) (DirectProjectionItemBasis, error) {
	basis := DirectProjectionItemBasis{
		output:    output,
		inputs:    canonicalProjectionInputs(inputs),
		transform: transform,
		losses:    canonicalIntentionalLosses(losses),
	}
	if !basis.valid() {
		return DirectProjectionItemBasis{}, fmt.Errorf(
			"direct projection item basis is invalid",
		)
	}
	return basis, nil
}

func (basis DirectProjectionItemBasis) Kind() ProjectionItemBasisKind {
	return ItemBasisDirect
}

func (basis DirectProjectionItemBasis) Output() OutputItemCoordinate {
	return basis.output
}

func (basis DirectProjectionItemBasis) Inputs() []ProjectionInputCoordinate {
	return append([]ProjectionInputCoordinate{}, basis.inputs...)
}

func (basis DirectProjectionItemBasis) Transform() ConservativeTransformKind {
	return basis.transform
}

func (basis DirectProjectionItemBasis) IntentionalLosses() []IntentionalLossKind {
	return append([]IntentionalLossKind{}, basis.losses...)
}

func (DirectProjectionItemBasis) isProjectionItemBasis() {}

func (basis DirectProjectionItemBasis) valid() bool {
	return basis.output.Valid() &&
		len(basis.inputs) > 0 &&
		allProjectionInputsValid(basis.inputs) &&
		basis.transform.Valid() &&
		allIntentionalLossesKnown(basis.losses)
}

type RelationPathWitness struct {
	assertion         typedmemory.AssertionID
	signature         typedmemory.SignatureID
	context           typedmemory.BoundedContextRef
	slot              typedmemory.SlotKindID
	target            typedmemory.PersistedRef
	provenance        typedmemory.ProvenanceRef
	admissionEventRef string
	posture           RelationalRecordItemPosture
}

func NewRelationPathWitness(
	assertion typedmemory.AssertionID,
	signature typedmemory.SignatureID,
	context typedmemory.BoundedContextRef,
	slot typedmemory.SlotKindID,
	target typedmemory.PersistedRef,
	provenance typedmemory.ProvenanceRef,
	admissionEventRef string,
) (RelationPathWitness, error) {
	// The historical constructor remains a legacy assertion-shaped path. It
	// never upgrades a v2 RelationInstance into an exact v3 assertion or an
	// occurrence.
	return newRelationPathWitness(
		assertion,
		signature,
		context,
		slot,
		target,
		provenance,
		admissionEventRef,
		legacyUnqualifiedAssertionItemPosture(),
	)
}

// NewRelationalAssertionPathWitness preserves the exact v3 assertion posture.
// Even AffirmsObtaining remains assertion content here; this constructor does
// not mint an occurrence witness.
func NewRelationalAssertionPathWitness(
	assertion typedmemory.AssertionID,
	signature typedmemory.SignatureID,
	context typedmemory.BoundedContextRef,
	slot typedmemory.SlotKindID,
	target typedmemory.PersistedRef,
	provenance typedmemory.ProvenanceRef,
	admissionEventRef string,
	modality typedmemory.AssertionModalityKind,
) (RelationPathWitness, error) {
	return newRelationPathWitness(
		assertion,
		signature,
		context,
		slot,
		target,
		provenance,
		admissionEventRef,
		exactAssertionItemPosture(modality),
	)
}

func newRelationPathWitness(
	assertion typedmemory.AssertionID,
	signature typedmemory.SignatureID,
	context typedmemory.BoundedContextRef,
	slot typedmemory.SlotKindID,
	target typedmemory.PersistedRef,
	provenance typedmemory.ProvenanceRef,
	admissionEventRef string,
	posture RelationalRecordItemPosture,
) (RelationPathWitness, error) {
	witness := RelationPathWitness{
		assertion:         assertion,
		signature:         signature,
		context:           context,
		slot:              slot,
		target:            target,
		provenance:        provenance,
		admissionEventRef: admissionEventRef,
		posture:           posture,
	}
	if !witness.Valid() {
		return RelationPathWitness{}, fmt.Errorf(
			"projection relation-path witness is invalid",
		)
	}
	return witness, nil
}

func (witness RelationPathWitness) Assertion() typedmemory.AssertionID {
	return witness.assertion
}

func (witness RelationPathWitness) Signature() typedmemory.SignatureID {
	return witness.signature
}

// RelationDeclarationFragmentID is the current semantic reading of the
// edition-bound identifier. Signature remains the sealed v1 compatibility
// accessor and does not claim a complete FPF RelationSignature.
func (witness RelationPathWitness) RelationDeclarationFragmentID() typedmemory.SignatureID {
	return witness.signature
}

func (witness RelationPathWitness) Context() typedmemory.BoundedContextRef {
	return witness.context
}

func (witness RelationPathWitness) Slot() typedmemory.SlotKindID {
	return witness.slot
}

func (witness RelationPathWitness) Target() typedmemory.PersistedRef {
	return witness.target
}

func (witness RelationPathWitness) Provenance() typedmemory.ProvenanceRef {
	return witness.provenance
}

func (witness RelationPathWitness) AdmissionEventRef() string {
	return witness.admissionEventRef
}

func (witness RelationPathWitness) RelationalRecordPosture() RelationalRecordItemPosture {
	return witness.posture
}

// RelationDeclarationPosture states what the referenced schema carrier can
// establish. Current project-memory relation paths are checked against Haft's
// structural declaration fragments; they are not witnesses of a complete FPF
// RelationSignature declaration episteme.
func (RelationPathWitness) RelationDeclarationPosture() typedmemory.RelationDeclarationPosture {
	return typedmemory.RelationDeclarationTypedFragment
}

func (witness RelationPathWitness) Valid() bool {
	assertion, assertionErr := typedmemory.NewAssertionID(
		witness.assertion.String(),
	)
	signature, signatureErr := typedmemory.NewSignatureID(
		witness.signature.String(),
	)
	context, contextErr := typedmemory.NewBoundedContextRef(
		witness.context.String(),
	)
	slot, slotErr := typedmemory.NewSlotKindID(witness.slot.String())
	provenance, provenanceErr := typedmemory.NewProvenanceRef(
		witness.provenance.String(),
	)
	event, eventErr := exactReference(
		"relation-path admission event",
		witness.admissionEventRef,
	)
	return assertionErr == nil &&
		signatureErr == nil &&
		contextErr == nil &&
		slotErr == nil &&
		provenanceErr == nil &&
		eventErr == nil &&
		assertion == witness.assertion &&
		signature == witness.signature &&
		context == witness.context &&
		slot == witness.slot &&
		provenance == witness.provenance &&
		event == witness.admissionEventRef &&
		witness.RelationDeclarationPosture() == typedmemory.RelationDeclarationTypedFragment &&
		witness.posture.Valid() &&
		validPersistedRef(witness.target)
}

func (witness RelationPathWitness) key() string {
	return strings.Join(
		[]string{
			witness.assertion.String(),
			witness.signature.String(),
			witness.context.String(),
			witness.slot.String(),
			witness.target.RefKind().String(),
			witness.target.ReferenceID().String(),
			witness.provenance.String(),
			witness.admissionEventRef,
			string(witness.RelationDeclarationPosture()),
			string(witness.posture.Kind()),
			witnessModalityKey(witness.posture),
		},
		":",
	)
}

func witnessModalityKey(posture RelationalRecordItemPosture) string {
	modality, explicit := posture.ExplicitModality()
	if !explicit {
		return ""
	}
	return modality.String()
}

type ProjectionCorrespondenceManifest struct {
	ref       ProjectionCorrespondenceManifestRef
	inputs    []ProjectionInputCoordinate
	output    OutputItemCoordinate
	witnesses []RelationPathWitness
	transform ConservativeTransformKind
	losses    []IntentionalLossKind
	digest    typedmemory.SHA256Digest
}

func NewProjectionCorrespondenceManifest(
	ref ProjectionCorrespondenceManifestRef,
	inputs []ProjectionInputCoordinate,
	output OutputItemCoordinate,
	witnesses []RelationPathWitness,
	transform ConservativeTransformKind,
	losses []IntentionalLossKind,
) (ProjectionCorrespondenceManifest, error) {
	manifest := ProjectionCorrespondenceManifest{
		ref:       ref,
		inputs:    canonicalProjectionInputs(inputs),
		output:    output,
		witnesses: canonicalRelationWitnesses(witnesses),
		transform: transform,
		losses:    canonicalIntentionalLosses(losses),
	}
	digest, err := projectionCorrespondenceManifestDigest(manifest)
	if err != nil {
		return ProjectionCorrespondenceManifest{}, err
	}
	manifest.digest = digest
	if !manifest.Valid() {
		return ProjectionCorrespondenceManifest{}, fmt.Errorf(
			"projection correspondence manifest is invalid",
		)
	}
	return manifest, nil
}

func (manifest ProjectionCorrespondenceManifest) Ref() ProjectionCorrespondenceManifestRef {
	return manifest.ref
}

func (manifest ProjectionCorrespondenceManifest) Inputs() []ProjectionInputCoordinate {
	return append([]ProjectionInputCoordinate{}, manifest.inputs...)
}

func (manifest ProjectionCorrespondenceManifest) Output() OutputItemCoordinate {
	return manifest.output
}

func (manifest ProjectionCorrespondenceManifest) Witnesses() []RelationPathWitness {
	return append([]RelationPathWitness{}, manifest.witnesses...)
}

func (manifest ProjectionCorrespondenceManifest) Transform() ConservativeTransformKind {
	return manifest.transform
}

func (manifest ProjectionCorrespondenceManifest) IntentionalLosses() []IntentionalLossKind {
	return append([]IntentionalLossKind{}, manifest.losses...)
}

func (manifest ProjectionCorrespondenceManifest) Digest() typedmemory.SHA256Digest {
	return manifest.digest
}

func (manifest ProjectionCorrespondenceManifest) Valid() bool {
	if manifest.ref.String() == "" ||
		len(manifest.inputs) < 2 ||
		len(manifest.witnesses) == 0 ||
		!manifest.output.Valid() ||
		!allProjectionInputsValid(manifest.inputs) ||
		!allRelationWitnessesValid(manifest.witnesses) ||
		!manifest.transform.Valid() ||
		!allIntentionalLossesKnown(manifest.losses) {
		return false
	}
	digest, err := projectionCorrespondenceManifestDigest(manifest)
	return err == nil && digest == manifest.digest
}

type CorrespondenceProjectionItemBasis struct {
	output      OutputItemCoordinate
	inputs      []ProjectionInputCoordinate
	manifestRef ProjectionCorrespondenceManifestRef
	transform   ConservativeTransformKind
	losses      []IntentionalLossKind
}

func NewCorrespondenceProjectionItemBasis(
	manifest ProjectionCorrespondenceManifest,
) (CorrespondenceProjectionItemBasis, error) {
	if !manifest.Valid() {
		return CorrespondenceProjectionItemBasis{}, fmt.Errorf(
			"correspondence item basis requires a valid manifest",
		)
	}
	return CorrespondenceProjectionItemBasis{
		output:      manifest.Output(),
		inputs:      manifest.Inputs(),
		manifestRef: manifest.Ref(),
		transform:   manifest.Transform(),
		losses:      manifest.IntentionalLosses(),
	}, nil
}

func (basis CorrespondenceProjectionItemBasis) Kind() ProjectionItemBasisKind {
	return ItemBasisCorrespondence
}

func (basis CorrespondenceProjectionItemBasis) Output() OutputItemCoordinate {
	return basis.output
}

func (basis CorrespondenceProjectionItemBasis) Inputs() []ProjectionInputCoordinate {
	return append([]ProjectionInputCoordinate{}, basis.inputs...)
}

func (basis CorrespondenceProjectionItemBasis) ManifestRef() ProjectionCorrespondenceManifestRef {
	return basis.manifestRef
}

func (basis CorrespondenceProjectionItemBasis) Transform() ConservativeTransformKind {
	return basis.transform
}

func (basis CorrespondenceProjectionItemBasis) IntentionalLosses() []IntentionalLossKind {
	return append([]IntentionalLossKind{}, basis.losses...)
}

func (CorrespondenceProjectionItemBasis) isProjectionItemBasis() {}

func (basis CorrespondenceProjectionItemBasis) valid() bool {
	return basis.output.Valid() &&
		len(basis.inputs) >= 2 &&
		basis.manifestRef.String() != "" &&
		allProjectionInputsValid(basis.inputs) &&
		basis.transform.Valid() &&
		allIntentionalLossesKnown(basis.losses)
}

type DeclaredReadSet struct {
	inputs []ProfileInputKind
	slots  []typedmemory.SlotKindID
}

func newDeclaredReadSet(
	profile ProjectionProfileDefinition,
) DeclaredReadSet {
	return DeclaredReadSet{
		inputs: profile.Inputs(),
		slots:  profile.SlotReads(),
	}
}

func (set DeclaredReadSet) Inputs() []ProfileInputKind {
	return append([]ProfileInputKind{}, set.inputs...)
}

func (set DeclaredReadSet) SlotKinds() []typedmemory.SlotKindID {
	return append([]typedmemory.SlotKindID{}, set.slots...)
}

type ProjectionBasis struct {
	profileRef       ProjectionProfileRef
	profileEdition   uint32
	profileDigest    typedmemory.SHA256Digest
	projectionSchema string
	canonicalInputs  []CanonicalInputCoordinate
	derivedInputs    []DerivedProjectionCoordinate
	readSet          DeclaredReadSet
	manifests        []ProjectionCorrespondenceManifest
	itemBases        []ProjectionItemBasis
	digest           typedmemory.SHA256Digest
}

func (basis ProjectionBasis) ProfileRef() ProjectionProfileRef {
	return basis.profileRef
}

func (basis ProjectionBasis) ProfileEdition() uint32 {
	return basis.profileEdition
}

func (basis ProjectionBasis) ProfileDigest() typedmemory.SHA256Digest {
	return basis.profileDigest
}

func (basis ProjectionBasis) ProjectionSchemaVersion() string {
	return basis.projectionSchema
}

func (basis ProjectionBasis) CanonicalInputs() []CanonicalInputCoordinate {
	return append([]CanonicalInputCoordinate{}, basis.canonicalInputs...)
}

func (basis ProjectionBasis) DerivedInputs() []DerivedProjectionCoordinate {
	return append([]DerivedProjectionCoordinate{}, basis.derivedInputs...)
}

func (basis ProjectionBasis) DeclaredReadSet() DeclaredReadSet {
	return DeclaredReadSet{
		inputs: basis.readSet.Inputs(),
		slots:  basis.readSet.SlotKinds(),
	}
}

func (basis ProjectionBasis) CorrespondenceManifests() []ProjectionCorrespondenceManifest {
	return append([]ProjectionCorrespondenceManifest{}, basis.manifests...)
}

func (basis ProjectionBasis) ItemBases() []ProjectionItemBasis {
	return append([]ProjectionItemBasis{}, basis.itemBases...)
}

func (basis ProjectionBasis) Digest() typedmemory.SHA256Digest {
	return basis.digest
}

func (basis ProjectionBasis) ItemBasisFor(
	coordinate OutputItemCoordinate,
) (ProjectionItemBasis, bool) {
	key := coordinate.key()
	index, found := slices.BinarySearchFunc(
		basis.itemBases,
		key,
		func(candidate ProjectionItemBasis, sought string) int {
			return strings.Compare(candidate.Output().key(), sought)
		},
	)
	if !found {
		return nil, false
	}
	return basis.itemBases[index], true
}

func (basis ProjectionBasis) Valid() bool {
	return validateProjectionBasis(basis) == nil
}

type ProjectionBasisBuilder struct {
	profile         ProjectionProfileDefinition
	canonicalInputs []CanonicalInputCoordinate
	derivedInputs   []DerivedProjectionCoordinate
	manifests       []ProjectionCorrespondenceManifest
	itemBases       []ProjectionItemBasis
}

func NewProjectionBasisBuilder(
	profile ProjectionProfileDefinition,
) *ProjectionBasisBuilder {
	return &ProjectionBasisBuilder{profile: profile}
}

func (builder *ProjectionBasisBuilder) AddCanonicalInput(
	value CanonicalInputCoordinate,
) *ProjectionBasisBuilder {
	builder.canonicalInputs = append(builder.canonicalInputs, value)
	return builder
}

func (builder *ProjectionBasisBuilder) AddDerivedInput(
	value DerivedProjectionCoordinate,
) *ProjectionBasisBuilder {
	builder.derivedInputs = append(builder.derivedInputs, value)
	return builder
}

func (builder *ProjectionBasisBuilder) AddCorrespondenceManifest(
	value ProjectionCorrespondenceManifest,
) *ProjectionBasisBuilder {
	builder.manifests = append(builder.manifests, value)
	return builder
}

func (builder *ProjectionBasisBuilder) AddItemBasis(
	value ProjectionItemBasis,
) *ProjectionBasisBuilder {
	builder.itemBases = append(builder.itemBases, value)
	return builder
}

func (builder *ProjectionBasisBuilder) Build() (ProjectionBasis, error) {
	if builder == nil || !builder.profile.Valid() {
		return ProjectionBasis{}, fmt.Errorf(
			"projection basis requires an immutable profile",
		)
	}
	basis := ProjectionBasis{
		profileRef:       builder.profile.Ref(),
		profileEdition:   builder.profile.Edition(),
		profileDigest:    builder.profile.Digest(),
		projectionSchema: builder.profile.SchemaVersion(),
		canonicalInputs:  canonicalCanonicalInputs(builder.canonicalInputs),
		derivedInputs:    canonicalDerivedInputs(builder.derivedInputs),
		readSet:          newDeclaredReadSet(builder.profile),
		manifests:        canonicalCorrespondenceManifests(builder.manifests),
		itemBases:        canonicalItemBases(builder.itemBases),
	}
	digest, err := projectionBasisDigest(basis)
	if err != nil {
		return ProjectionBasis{}, err
	}
	basis.digest = digest
	if err := validateProjectionBasis(basis); err != nil {
		return ProjectionBasis{}, err
	}
	return basis, nil
}

func validateProjectionBasis(basis ProjectionBasis) error {
	profile, found := LookupProjectionProfile(basis.profileRef)
	if !found ||
		profile.Edition() != basis.profileEdition ||
		profile.Digest() != basis.profileDigest ||
		profile.SchemaVersion() != basis.projectionSchema {
		return fmt.Errorf("projection basis profile identity is not current")
	}
	if len(basis.canonicalInputs) == 0 {
		return fmt.Errorf("projection basis requires canonical input")
	}
	if len(basis.itemBases) == 0 {
		return fmt.Errorf("projection basis requires total item basis")
	}
	if !allCanonicalInputsValid(basis.canonicalInputs) ||
		!allDerivedInputsValid(basis.derivedInputs) ||
		!allCorrespondenceManifestsValid(basis.manifests) ||
		!allItemBasesValid(basis.itemBases) {
		return fmt.Errorf("projection basis contains invalid coordinates")
	}
	if !slices.Equal(basis.readSet.inputs, profile.Inputs()) ||
		!slices.Equal(basis.readSet.slots, profile.SlotReads()) {
		return fmt.Errorf("projection basis declared read set differs from profile")
	}
	if hasDuplicateCanonicalInputRefs(basis.canonicalInputs) ||
		hasDuplicateDerivedProjectionRefs(basis.derivedInputs) ||
		hasDuplicateManifestRefs(basis.manifests) ||
		hasDuplicateOutputCoordinates(basis.itemBases) {
		return fmt.Errorf("projection basis repeats an exact coordinate")
	}
	if err := validateItemInputClosure(basis); err != nil {
		return err
	}
	if err := validateCorrespondenceClosure(basis); err != nil {
		return err
	}
	digest, err := projectionBasisDigest(basis)
	if err != nil {
		return err
	}
	if digest != basis.digest {
		return fmt.Errorf("projection basis digest is not canonical")
	}
	return nil
}

func validateItemInputClosure(basis ProjectionBasis) error {
	declared := make(map[string]struct{}, len(basis.canonicalInputs))
	for _, input := range basis.canonicalInputs {
		key := input.ref.String() + "@" + input.digest.String()
		declared[key] = struct{}{}
	}
	for _, item := range basis.itemBases {
		for _, input := range item.Inputs() {
			if _, found := declared[input.key()]; found {
				continue
			}
			return fmt.Errorf(
				"projection item %q reads undeclared input %q",
				item.Output().key(),
				input.key(),
			)
		}
	}
	return nil
}

func validateCorrespondenceClosure(basis ProjectionBasis) error {
	manifests := make(
		map[string]ProjectionCorrespondenceManifest,
		len(basis.manifests),
	)
	for _, manifest := range basis.manifests {
		manifests[manifest.Ref().String()] = manifest
	}
	for _, item := range basis.itemBases {
		correspondence, ok := item.(CorrespondenceProjectionItemBasis)
		if !ok {
			continue
		}
		manifest, found := manifests[correspondence.ManifestRef().String()]
		if !found {
			return fmt.Errorf(
				"projection item %q has no exact correspondence manifest",
				item.Output().key(),
			)
		}
		if manifest.Output() != item.Output() ||
			!slices.Equal(manifest.Inputs(), item.Inputs()) ||
			manifest.Transform() != item.Transform() ||
			!slices.Equal(
				manifest.IntentionalLosses(),
				item.IntentionalLosses(),
			) {
			return fmt.Errorf(
				"projection item %q differs from its correspondence manifest",
				item.Output().key(),
			)
		}
	}
	return nil
}

type projectionCorrespondenceManifestCanonicalV1 struct {
	Ref       string                           `json:"manifest_ref"`
	Inputs    []projectionInputCanonicalV1     `json:"inputs"`
	Output    outputItemCoordinateCanonicalV1  `json:"output"`
	Witnesses []relationPathWitnessCanonicalV1 `json:"relation_witnesses"`
	Transform string                           `json:"transform"`
	Losses    []string                         `json:"intentional_loss"`
}

type projectionBasisCanonicalV1 struct {
	Schema           string                                        `json:"schema"`
	ProfileRef       string                                        `json:"profile_ref"`
	ProfileEdition   uint32                                        `json:"profile_edition"`
	ProfileDigest    string                                        `json:"profile_digest"`
	ProjectionSchema string                                        `json:"projection_schema_version"`
	CanonicalInputs  []canonicalInputCanonicalV1                   `json:"canonical_inputs"`
	DerivedInputs    []derivedProjectionCanonicalV1                `json:"derived_projection_inputs"`
	ReadInputs       []string                                      `json:"declared_input_families"`
	ReadSlots        []string                                      `json:"declared_slot_kinds"`
	Manifests        []projectionCorrespondenceManifestCanonicalV1 `json:"correspondence_manifests"`
	ItemBases        []projectionItemBasisCanonicalV1              `json:"item_basis"`
}

type canonicalInputCanonicalV1 struct {
	Ref    string `json:"ref"`
	Digest string `json:"digest"`
}

type derivedProjectionCanonicalV1 struct {
	Ref     string `json:"ref"`
	Version string `json:"version"`
	Epoch   uint64 `json:"epoch"`
	Digest  string `json:"digest"`
}

type projectionInputCanonicalV1 struct {
	Ref    string `json:"ref"`
	Digest string `json:"digest"`
}

type outputItemCoordinateCanonicalV1 struct {
	Scope         string `json:"scope"`
	Facet         string `json:"facet,omitempty"`
	ReferenceKind string `json:"reference_kind"`
	ReferenceID   string `json:"reference_id"`
}

type relationPathWitnessCanonicalV1 struct {
	Assertion                  string `json:"assertion_id"`
	Signature                  string `json:"signature_id"`
	RelationDeclarationPosture string `json:"relation_declaration_posture"`
	Context                    string `json:"bounded_context_ref"`
	Slot                       string `json:"slot_kind_id"`
	TargetRefKind              string `json:"target_reference_kind"`
	TargetReferenceID          string `json:"target_reference_id"`
	Provenance                 string `json:"provenance_ref"`
	AdmissionEventRef          string `json:"admission_event_ref"`
	RelationalPosture          string `json:"relational_record_posture"`
	ExplicitModality           string `json:"explicit_modality,omitempty"`
}

type projectionItemBasisCanonicalV1 struct {
	Kind        string                          `json:"kind"`
	Output      outputItemCoordinateCanonicalV1 `json:"output"`
	Inputs      []projectionInputCanonicalV1    `json:"inputs"`
	ManifestRef string                          `json:"correspondence_manifest_ref,omitempty"`
	Transform   string                          `json:"transform"`
	Losses      []string                        `json:"intentional_loss"`
}

func projectionCorrespondenceManifestDigest(
	manifest ProjectionCorrespondenceManifest,
) (typedmemory.SHA256Digest, error) {
	carrier := encodeCorrespondenceManifest(manifest)
	return digestCanonicalJSON(carrier)
}

func projectionBasisDigest(
	basis ProjectionBasis,
) (typedmemory.SHA256Digest, error) {
	readInputs := make([]string, 0, len(basis.readSet.inputs))
	for _, input := range basis.readSet.inputs {
		readInputs = append(readInputs, string(input))
	}
	readSlots := make([]string, 0, len(basis.readSet.slots))
	for _, slot := range basis.readSet.slots {
		readSlots = append(readSlots, slot.String())
	}
	carrier := projectionBasisCanonicalV1{
		Schema:           ProjectionBasisSchemaV1,
		ProfileRef:       basis.profileRef.String(),
		ProfileEdition:   basis.profileEdition,
		ProfileDigest:    basis.profileDigest.String(),
		ProjectionSchema: basis.projectionSchema,
		CanonicalInputs:  encodeCanonicalInputs(basis.canonicalInputs),
		DerivedInputs:    encodeDerivedInputs(basis.derivedInputs),
		ReadInputs:       readInputs,
		ReadSlots:        readSlots,
		Manifests:        encodeCorrespondenceManifests(basis.manifests),
		ItemBases:        encodeItemBases(basis.itemBases),
	}
	return digestCanonicalJSON(carrier)
}

func digestCanonicalJSON(value any) (typedmemory.SHA256Digest, error) {
	canonical, err := json.Marshal(value)
	if err != nil {
		return typedmemory.SHA256Digest{}, fmt.Errorf(
			"encode projection basis canonical JSON: %w",
			err,
		)
	}
	sum := sha256.Sum256(canonical)
	raw := "sha256:" + hex.EncodeToString(sum[:])
	return typedmemory.NewSHA256Digest(raw)
}

func encodeCanonicalInputs(
	values []CanonicalInputCoordinate,
) []canonicalInputCanonicalV1 {
	result := make([]canonicalInputCanonicalV1, 0, len(values))
	for _, value := range values {
		result = append(result, canonicalInputCanonicalV1{
			Ref:    value.Ref().String(),
			Digest: value.Digest().String(),
		})
	}
	return result
}

func encodeDerivedInputs(
	values []DerivedProjectionCoordinate,
) []derivedProjectionCanonicalV1 {
	result := make([]derivedProjectionCanonicalV1, 0, len(values))
	for _, value := range values {
		result = append(result, derivedProjectionCanonicalV1{
			Ref:     value.Ref().String(),
			Version: value.Version().String(),
			Epoch:   value.Epoch(),
			Digest:  value.Digest().String(),
		})
	}
	return result
}

func encodeProjectionInputs(
	values []ProjectionInputCoordinate,
) []projectionInputCanonicalV1 {
	result := make([]projectionInputCanonicalV1, 0, len(values))
	for _, value := range values {
		result = append(result, projectionInputCanonicalV1{
			Ref:    value.Ref().String(),
			Digest: value.Digest().String(),
		})
	}
	return result
}

func encodeOutputCoordinate(
	value OutputItemCoordinate,
) outputItemCoordinateCanonicalV1 {
	facet, facetSet := value.Facet()
	facetValue := ""
	if facetSet {
		facetValue = string(facet)
	}
	return outputItemCoordinateCanonicalV1{
		Scope:         string(value.Scope()),
		Facet:         facetValue,
		ReferenceKind: value.Reference().RefKind().String(),
		ReferenceID:   value.Reference().ReferenceID().String(),
	}
}

func encodeRelationWitnesses(
	values []RelationPathWitness,
) []relationPathWitnessCanonicalV1 {
	result := make([]relationPathWitnessCanonicalV1, 0, len(values))
	for _, value := range values {
		result = append(result, relationPathWitnessCanonicalV1{
			Assertion:                  value.Assertion().String(),
			Signature:                  value.Signature().String(),
			RelationDeclarationPosture: string(value.RelationDeclarationPosture()),
			Context:                    value.Context().String(),
			Slot:                       value.Slot().String(),
			TargetRefKind:              value.Target().RefKind().String(),
			TargetReferenceID:          value.Target().ReferenceID().String(),
			Provenance:                 value.Provenance().String(),
			AdmissionEventRef:          value.AdmissionEventRef(),
			RelationalPosture: string(
				value.RelationalRecordPosture().Kind(),
			),
			ExplicitModality: witnessModalityKey(
				value.RelationalRecordPosture(),
			),
		})
	}
	return result
}

func encodeCorrespondenceManifest(
	manifest ProjectionCorrespondenceManifest,
) projectionCorrespondenceManifestCanonicalV1 {
	return projectionCorrespondenceManifestCanonicalV1{
		Ref:       manifest.Ref().String(),
		Inputs:    encodeProjectionInputs(manifest.Inputs()),
		Output:    encodeOutputCoordinate(manifest.Output()),
		Witnesses: encodeRelationWitnesses(manifest.Witnesses()),
		Transform: string(manifest.Transform()),
		Losses:    encodeIntentionalLosses(manifest.IntentionalLosses()),
	}
}

func encodeCorrespondenceManifests(
	values []ProjectionCorrespondenceManifest,
) []projectionCorrespondenceManifestCanonicalV1 {
	result := make(
		[]projectionCorrespondenceManifestCanonicalV1,
		0,
		len(values),
	)
	for _, value := range values {
		result = append(result, encodeCorrespondenceManifest(value))
	}
	return result
}

func encodeItemBases(
	values []ProjectionItemBasis,
) []projectionItemBasisCanonicalV1 {
	result := make([]projectionItemBasisCanonicalV1, 0, len(values))
	for _, value := range values {
		manifestRef := ""
		correspondence, ok := value.(CorrespondenceProjectionItemBasis)
		if ok {
			manifestRef = correspondence.ManifestRef().String()
		}
		result = append(result, projectionItemBasisCanonicalV1{
			Kind:        string(value.Kind()),
			Output:      encodeOutputCoordinate(value.Output()),
			Inputs:      encodeProjectionInputs(value.Inputs()),
			ManifestRef: manifestRef,
			Transform:   string(value.Transform()),
			Losses:      encodeIntentionalLosses(value.IntentionalLosses()),
		})
	}
	return result
}

func encodeIntentionalLosses(values []IntentionalLossKind) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, string(value))
	}
	return result
}

func canonicalCanonicalInputs(
	values []CanonicalInputCoordinate,
) []CanonicalInputCoordinate {
	result := append([]CanonicalInputCoordinate{}, values...)
	sort.Slice(result, func(left int, right int) bool {
		return result[left].key() < result[right].key()
	})
	return result
}

func canonicalDerivedInputs(
	values []DerivedProjectionCoordinate,
) []DerivedProjectionCoordinate {
	result := append([]DerivedProjectionCoordinate{}, values...)
	sort.Slice(result, func(left int, right int) bool {
		return result[left].key() < result[right].key()
	})
	return result
}

func canonicalProjectionInputs(
	values []ProjectionInputCoordinate,
) []ProjectionInputCoordinate {
	result := append([]ProjectionInputCoordinate{}, values...)
	sort.Slice(result, func(left int, right int) bool {
		return result[left].key() < result[right].key()
	})
	return result
}

func canonicalRelationWitnesses(
	values []RelationPathWitness,
) []RelationPathWitness {
	result := append([]RelationPathWitness{}, values...)
	sort.Slice(result, func(left int, right int) bool {
		return result[left].key() < result[right].key()
	})
	return result
}

func canonicalCorrespondenceManifests(
	values []ProjectionCorrespondenceManifest,
) []ProjectionCorrespondenceManifest {
	result := append([]ProjectionCorrespondenceManifest{}, values...)
	sort.Slice(result, func(left int, right int) bool {
		return result[left].Ref().String() < result[right].Ref().String()
	})
	return result
}

func canonicalItemBases(values []ProjectionItemBasis) []ProjectionItemBasis {
	result := append([]ProjectionItemBasis{}, values...)
	sort.Slice(result, func(left int, right int) bool {
		return result[left].Output().key() < result[right].Output().key()
	})
	return result
}

func canonicalIntentionalLosses(
	values []IntentionalLossKind,
) []IntentionalLossKind {
	result := append([]IntentionalLossKind{}, values...)
	sort.Slice(result, func(left int, right int) bool {
		return result[left] < result[right]
	})
	return slices.Compact(result)
}

func allCanonicalInputsValid(values []CanonicalInputCoordinate) bool {
	for _, value := range values {
		if !value.Valid() {
			return false
		}
	}
	return true
}

func allDerivedInputsValid(values []DerivedProjectionCoordinate) bool {
	for _, value := range values {
		if !value.Valid() {
			return false
		}
	}
	return true
}

func allProjectionInputsValid(values []ProjectionInputCoordinate) bool {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if !value.Valid() {
			return false
		}
		if _, found := seen[value.key()]; found {
			return false
		}
		seen[value.key()] = struct{}{}
	}
	return true
}

func allRelationWitnessesValid(values []RelationPathWitness) bool {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if !value.Valid() {
			return false
		}
		if _, found := seen[value.key()]; found {
			return false
		}
		seen[value.key()] = struct{}{}
	}
	return true
}

func allCorrespondenceManifestsValid(
	values []ProjectionCorrespondenceManifest,
) bool {
	for _, value := range values {
		if !value.Valid() {
			return false
		}
	}
	return true
}

func allItemBasesValid(values []ProjectionItemBasis) bool {
	for _, value := range values {
		if value == nil ||
			!value.Output().Valid() ||
			!value.Transform().Valid() ||
			!allProjectionInputsValid(value.Inputs()) ||
			!allIntentionalLossesKnown(value.IntentionalLosses()) {
			return false
		}
		direct, directOK := value.(DirectProjectionItemBasis)
		correspondence, correspondenceOK := value.(CorrespondenceProjectionItemBasis)
		if directOK == correspondenceOK {
			return false
		}
		if directOK && !direct.valid() {
			return false
		}
		if correspondenceOK && !correspondence.valid() {
			return false
		}
	}
	return true
}

func hasDuplicateCanonicalInputRefs(
	values []CanonicalInputCoordinate,
) bool {
	return hasDuplicateStrings(
		values,
		func(value CanonicalInputCoordinate) string {
			return value.Ref().String()
		},
	)
}

func hasDuplicateDerivedProjectionRefs(
	values []DerivedProjectionCoordinate,
) bool {
	return hasDuplicateStrings(
		values,
		func(value DerivedProjectionCoordinate) string {
			return value.Ref().String()
		},
	)
}

func hasDuplicateManifestRefs(
	values []ProjectionCorrespondenceManifest,
) bool {
	return hasDuplicateStrings(
		values,
		func(value ProjectionCorrespondenceManifest) string {
			return value.Ref().String()
		},
	)
}

func hasDuplicateOutputCoordinates(values []ProjectionItemBasis) bool {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		key := value.Output().key()
		if _, found := seen[key]; found {
			return true
		}
		seen[key] = struct{}{}
	}
	return false
}

func hasDuplicateStrings[T any](
	values []T,
	key func(T) string,
) bool {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		current := key(value)
		if _, found := seen[current]; found {
			return true
		}
		seen[current] = struct{}{}
	}
	return false
}

func validPersistedRef(ref typedmemory.PersistedRef) bool {
	refKind, refKindErr := typedmemory.NewRefKindRef(
		ref.RefKind().TypeEnv(),
		ref.RefKind().ID(),
	)
	referenceID, referenceErr := typedmemory.NewReferenceID(
		ref.ReferenceID().String(),
	)
	canonical, canonicalErr := typedmemory.NewPersistedRef(
		refKind,
		referenceID,
	)
	return refKindErr == nil &&
		referenceErr == nil &&
		canonicalErr == nil &&
		canonical.RefKind() == ref.RefKind() &&
		canonical.ReferenceID() == ref.ReferenceID()
}
