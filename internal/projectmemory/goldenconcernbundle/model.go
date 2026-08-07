// Package goldenconcernbundle owns the exact dogfood acceptance projection for
// one EntityOfConcern neighborhood. The bundle is read-only evidence assembled
// from already-admitted task-adapter candidates; it is not a graph writer,
// workflow, recommendation, authority act, or project phase.
package goldenconcernbundle

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/recordmapping"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
)

const (
	SchemaV1 = "haft.golden-concern-bundle/v1"

	freshnessPostureV1  = "present_at_exact_bundle_snapshot"
	canonicalTimeLayout = time.RFC3339Nano
)

// ItemRole is a closed acceptance role. It names why an exact typed entity is
// present in this one bundle; it does not change that entity's U.Kind.
type ItemRole uint8

const (
	ItemEntityOfConcern ItemRole = iota + 1
	ItemProblemCard
	ItemSolutionOption
	ItemSolutionPortfolio
	ItemPortfolioComparison
	ItemDecisionRecord
	ItemSpecSection
	ItemProjectClaim
	ItemEvidenceRecord
	ItemSupportingEpistemeRecord
	ItemWorkRecord
	ItemPerformedWorkOccurrence
	ItemCodeAnchor
)

func (role ItemRole) String() string {
	switch role {
	case ItemEntityOfConcern:
		return "entity_of_concern"
	case ItemProblemCard:
		return "problem_card"
	case ItemSolutionOption:
		return "solution_option"
	case ItemSolutionPortfolio:
		return "solution_portfolio"
	case ItemPortfolioComparison:
		return "portfolio_comparison"
	case ItemDecisionRecord:
		return "decision_record"
	case ItemSpecSection:
		return "spec_section"
	case ItemProjectClaim:
		return "project_claim"
	case ItemEvidenceRecord:
		return "evidence_record"
	case ItemSupportingEpistemeRecord:
		return "supporting_episteme_record"
	case ItemWorkRecord:
		return "work_record"
	case ItemPerformedWorkOccurrence:
		return "performed_work_occurrence"
	case ItemCodeAnchor:
		return "code_anchor"
	default:
		return ""
	}
}

func parseItemRole(raw string) (ItemRole, error) {
	values := []ItemRole{
		ItemEntityOfConcern,
		ItemProblemCard,
		ItemSolutionOption,
		ItemSolutionPortfolio,
		ItemPortfolioComparison,
		ItemDecisionRecord,
		ItemSpecSection,
		ItemProjectClaim,
		ItemEvidenceRecord,
		ItemSupportingEpistemeRecord,
		ItemWorkRecord,
		ItemPerformedWorkOccurrence,
		ItemCodeAnchor,
	}
	for _, value := range values {
		if value.String() == raw {
			return value, nil
		}
	}
	return 0, fmt.Errorf("unsupported GoldenConcernBundle item role %q", raw)
}

type SnapshotCoordinate struct {
	context    typedmemory.BoundedContextRef
	typeEnv    typedmemory.TypeEnvRef
	revision   typedmemory.GraphRevision
	observedAt time.Time
}

func NewSnapshotCoordinate(
	contextRef typedmemory.BoundedContextRef,
	typeEnv typedmemory.TypeEnvRef,
	revision typedmemory.GraphRevision,
	observedAt time.Time,
) (SnapshotCoordinate, error) {
	contextParsed, err := typedmemory.NewBoundedContextRef(contextRef.String())
	if err != nil || contextParsed != contextRef {
		return SnapshotCoordinate{}, fmt.Errorf(
			"GoldenConcernBundle snapshot context is invalid",
		)
	}
	typeEnvParsed, err := typedmemory.ParseTypeEnvRef(typeEnv.String())
	if err != nil || typeEnvParsed != typeEnv {
		return SnapshotCoordinate{}, fmt.Errorf(
			"GoldenConcernBundle snapshot TypeEnv is invalid",
		)
	}
	if revision.Value() == 0 {
		return SnapshotCoordinate{}, fmt.Errorf(
			"GoldenConcernBundle snapshot must follow at least one admission",
		)
	}
	point, err := typedmemory.NewGammaPoint(observedAt)
	if err != nil {
		return SnapshotCoordinate{}, fmt.Errorf(
			"GoldenConcernBundle observation time: %w",
			err,
		)
	}
	return SnapshotCoordinate{
		context:    contextRef,
		typeEnv:    typeEnv,
		revision:   revision,
		observedAt: point.At(),
	}, nil
}

func (coordinate SnapshotCoordinate) Context() typedmemory.BoundedContextRef {
	return coordinate.context
}

func (coordinate SnapshotCoordinate) TypeEnv() typedmemory.TypeEnvRef {
	return coordinate.typeEnv
}

func (coordinate SnapshotCoordinate) GraphRevision() typedmemory.GraphRevision {
	return coordinate.revision
}

func (coordinate SnapshotCoordinate) ObservedAt() time.Time {
	return coordinate.observedAt
}

type ItemSpec struct {
	role              ItemRole
	reference         typedmemory.PersistedRef
	admissionEventRef string
}

func NewItemSpec(
	role ItemRole,
	reference typedmemory.PersistedRef,
	admissionEventRef string,
) (ItemSpec, error) {
	if role.String() == "" {
		return ItemSpec{}, fmt.Errorf(
			"GoldenConcernBundle item role is required",
		)
	}
	refKind, err := typedmemory.NewRefKindRef(
		reference.RefKind().TypeEnv(),
		reference.RefKind().ID(),
	)
	if err != nil || refKind != reference.RefKind() {
		return ItemSpec{}, fmt.Errorf(
			"GoldenConcernBundle item reference is invalid",
		)
	}
	referenceID, err := typedmemory.NewReferenceID(
		reference.ReferenceID().String(),
	)
	if err != nil || referenceID != reference.ReferenceID() {
		return ItemSpec{}, fmt.Errorf(
			"GoldenConcernBundle item reference ID is invalid",
		)
	}
	event, err := exactOneLine(
		"GoldenConcernBundle item admission event",
		admissionEventRef,
	)
	if err != nil {
		return ItemSpec{}, err
	}
	return ItemSpec{
		role:              role,
		reference:         reference,
		admissionEventRef: event,
	}, nil
}

type BundleItem struct {
	role              ItemRole
	reference         typedmemory.PersistedRef
	entity            typedmemory.EntityID
	label             typedmemory.EntityLabel
	provenance        typedmemory.ProvenanceRef
	admissionEventRef string
	admittedRevision  typedmemory.GraphRevision
	observedRevision  typedmemory.GraphRevision
	observedAt        time.Time
}

func (item BundleItem) Role() ItemRole { return item.role }

func (item BundleItem) Reference() typedmemory.PersistedRef {
	return item.reference
}

func (item BundleItem) Entity() typedmemory.EntityID { return item.entity }

func (item BundleItem) Label() typedmemory.EntityLabel { return item.label }

func (item BundleItem) Provenance() typedmemory.ProvenanceRef {
	return item.provenance
}

func (item BundleItem) AdmissionEventRef() string {
	return item.admissionEventRef
}

func (item BundleItem) AdmittedGraphRevision() typedmemory.GraphRevision {
	return item.admittedRevision
}

func (item BundleItem) ObservedGraphRevision() typedmemory.GraphRevision {
	return item.observedRevision
}

func (item BundleItem) ObservedAt() time.Time { return item.observedAt }

type DeclarationWitness struct {
	entity     typedmemory.EntityID
	localRef   typedmemory.BatchLocalRef
	context    typedmemory.BoundedContextRef
	label      typedmemory.EntityLabel
	provenance typedmemory.ProvenanceRef
}

func (witness DeclarationWitness) Entity() typedmemory.EntityID {
	return witness.entity
}

func (witness DeclarationWitness) LocalRef() typedmemory.BatchLocalRef {
	return witness.localRef
}

func (witness DeclarationWitness) Context() typedmemory.BoundedContextRef {
	return witness.context
}

func (witness DeclarationWitness) Label() typedmemory.EntityLabel {
	return witness.label
}

func (witness DeclarationWitness) Provenance() typedmemory.ProvenanceRef {
	return witness.provenance
}

type RelationPath struct {
	assertion         typedmemory.AssertionID
	signature         typedmemory.SignatureID
	context           typedmemory.BoundedContextRef
	slot              typedmemory.SlotKindID
	target            typedmemory.PersistedRef
	provenance        typedmemory.ProvenanceRef
	admissionEventRef string
}

func (path RelationPath) Assertion() typedmemory.AssertionID {
	return path.assertion
}

func (path RelationPath) Signature() typedmemory.SignatureID {
	return path.signature
}

func (path RelationPath) Context() typedmemory.BoundedContextRef {
	return path.context
}

func (path RelationPath) Slot() typedmemory.SlotKindID { return path.slot }

func (path RelationPath) Target() typedmemory.PersistedRef {
	return path.target
}

func (path RelationPath) Provenance() typedmemory.ProvenanceRef {
	return path.provenance
}

func (path RelationPath) AdmissionEventRef() string {
	return path.admissionEventRef
}

type ValueWitness struct {
	assertion         typedmemory.AssertionID
	signature         typedmemory.SignatureID
	slot              typedmemory.SlotKindID
	valueKind         typedmemory.ValueKindRef
	valueShape        typedmemory.ValueShapeRef
	codec             typedmemory.CodecRef
	inputDigest       typedmemory.SHA256Digest
	admissionEventRef string
}

type receiptWitness struct {
	disposition typedmemorystore.CommitDisposition
	eventRef    string
	commitRef   string
	revision    typedmemory.GraphRevision
	result      typedmemory.SHA256Digest
}

type ConcernAdmission struct {
	project          projectidentity.ProjectID
	declaration      DeclarationWitness
	reference        typedmemory.PersistedRef
	candidateDigest  typedmemory.SHA256Digest
	receipt          receiptWitness
	canonicalChanges []byte
}

func (admission ConcernAdmission) ProjectID() projectidentity.ProjectID {
	return admission.project
}

func (admission ConcernAdmission) Reference() typedmemory.PersistedRef {
	return admission.reference
}

func (admission ConcernAdmission) Declaration() DeclarationWitness {
	return admission.declaration
}

func (admission ConcernAdmission) EventRef() string {
	return admission.receipt.eventRef
}

type AdapterAdmission struct {
	project          projectidentity.ProjectID
	manifest         recordmapping.MappingManifestRef
	adapter          recordmapping.AdapterVersion
	candidateDigest  typedmemory.SHA256Digest
	receipt          receiptWitness
	signatures       []typedmemory.SignatureID
	declarations     []DeclarationWitness
	paths            []RelationPath
	values           []ValueWitness
	canonicalChanges []byte
}

func (admission AdapterAdmission) ProjectID() projectidentity.ProjectID {
	return admission.project
}

func (admission AdapterAdmission) MappingManifestRef() recordmapping.MappingManifestRef {
	return admission.manifest
}

func (admission AdapterAdmission) AdapterVersion() recordmapping.AdapterVersion {
	return admission.adapter
}

func (admission AdapterAdmission) CandidateDigest() typedmemory.SHA256Digest {
	return admission.candidateDigest
}

func (admission AdapterAdmission) EventRef() string {
	return admission.receipt.eventRef
}

func (admission AdapterAdmission) CommitRef() string {
	return admission.receipt.commitRef
}

func (admission AdapterAdmission) GraphRevision() typedmemory.GraphRevision {
	return admission.receipt.revision
}

func (admission AdapterAdmission) ResultDigest() typedmemory.SHA256Digest {
	return admission.receipt.result
}

func (admission AdapterAdmission) Signatures() []typedmemory.SignatureID {
	return append([]typedmemory.SignatureID(nil), admission.signatures...)
}

// RelationDeclarationFragmentIDs is the current semantic reading of the
// edition-bound IDs stored under the v1 `relation_signatures` wire key.
func (admission AdapterAdmission) RelationDeclarationFragmentIDs() []typedmemory.SignatureID {
	return append([]typedmemory.SignatureID(nil), admission.signatures...)
}

func (admission AdapterAdmission) Declarations() []DeclarationWitness {
	return append([]DeclarationWitness(nil), admission.declarations...)
}

func (admission AdapterAdmission) RelationPaths() []RelationPath {
	return append([]RelationPath(nil), admission.paths...)
}

type Bundle struct {
	project    projectidentity.ProjectID
	coordinate SnapshotCoordinate
	concern    ConcernAdmission
	admissions []AdapterAdmission
	items      []BundleItem
	paths      []RelationPath
	values     []ValueWitness
	canonical  []byte
	digest     typedmemory.SHA256Digest
}

func (bundle Bundle) ProjectID() projectidentity.ProjectID {
	return bundle.project
}

func (bundle Bundle) Snapshot() SnapshotCoordinate {
	return bundle.coordinate
}

func (bundle Bundle) Concern() ConcernAdmission { return bundle.concern }

func (bundle Bundle) AdapterAdmissions() []AdapterAdmission {
	return append([]AdapterAdmission(nil), bundle.admissions...)
}

func (bundle Bundle) Items() []BundleItem {
	return append([]BundleItem(nil), bundle.items...)
}

func (bundle Bundle) ExpectedRelationPaths() []RelationPath {
	return append([]RelationPath(nil), bundle.paths...)
}

func (bundle Bundle) CanonicalBytes() []byte {
	return append([]byte(nil), bundle.canonical...)
}

func (bundle Bundle) Digest() typedmemory.SHA256Digest {
	return bundle.digest
}

func (bundle Bundle) Verify() error {
	canonical, err := encodeBundleCanonical(bundle)
	if err != nil {
		return err
	}
	if !bytes.Equal(canonical, bundle.canonical) {
		return fmt.Errorf(
			"GoldenConcernBundle canonical bytes differ from live proof",
		)
	}
	digest, err := digestBytes(canonical)
	if err != nil {
		return err
	}
	if digest != bundle.digest {
		return fmt.Errorf(
			"GoldenConcernBundle digest differs from canonical bytes",
		)
	}
	return nil
}

type Builder struct {
	project       projectidentity.ProjectID
	concern       ConcernAdmission
	concernSet    bool
	coordinate    SnapshotCoordinate
	coordinateSet bool
	admissions    []AdapterAdmission
	items         []ItemSpec
}

func NewBuilder(project projectidentity.ProjectID) *Builder {
	return &Builder{project: project}
}

func (builder *Builder) SetConcern(
	concern ConcernAdmission,
) *Builder {
	if builder == nil {
		return builder
	}
	builder.concern = concern
	builder.concernSet = true
	return builder
}

func (builder *Builder) SetSnapshot(
	coordinate SnapshotCoordinate,
) *Builder {
	if builder == nil {
		return builder
	}
	builder.coordinate = coordinate
	builder.coordinateSet = true
	return builder
}

func (builder *Builder) AddAdapterAdmission(
	admission AdapterAdmission,
) *Builder {
	if builder == nil {
		return builder
	}
	builder.admissions = append(builder.admissions, admission)
	return builder
}

func (builder *Builder) AddItem(spec ItemSpec) *Builder {
	if builder == nil {
		return builder
	}
	builder.items = append(builder.items, spec)
	return builder
}

func (builder *Builder) Build() (Bundle, error) {
	if builder == nil {
		return Bundle{}, fmt.Errorf(
			"GoldenConcernBundle builder is required",
		)
	}
	project, err := projectidentity.ParseProjectID(builder.project.String())
	if err != nil || project != builder.project {
		return Bundle{}, fmt.Errorf(
			"GoldenConcernBundle project is invalid",
		)
	}
	if !builder.concernSet || !builder.coordinateSet {
		return Bundle{}, fmt.Errorf(
			"GoldenConcernBundle concern and snapshot are required",
		)
	}
	admissions := append([]AdapterAdmission(nil), builder.admissions...)
	sortAdapterAdmissions(admissions)
	if err := validateAdmissionSet(
		builder.project,
		builder.concern,
		builder.coordinate,
		admissions,
	); err != nil {
		return Bundle{}, err
	}
	items, err := materializeItems(
		builder.concern,
		builder.coordinate,
		admissions,
		builder.items,
	)
	if err != nil {
		return Bundle{}, err
	}
	paths := collectRelationPaths(admissions)
	values := collectValueWitnesses(admissions)
	if err := validateGoldenShape(
		builder.concern.Reference(),
		items,
		paths,
	); err != nil {
		return Bundle{}, err
	}
	bundle := Bundle{
		project:    builder.project,
		coordinate: builder.coordinate,
		concern:    builder.concern,
		admissions: admissions,
		items:      items,
		paths:      paths,
		values:     values,
	}
	canonical, err := encodeBundleCanonical(bundle)
	if err != nil {
		return Bundle{}, err
	}
	digest, err := digestBytes(canonical)
	if err != nil {
		return Bundle{}, err
	}
	bundle.canonical = canonical
	bundle.digest = digest
	return bundle, nil
}

func digestBytes(canonical []byte) (typedmemory.SHA256Digest, error) {
	sum := sha256.Sum256(canonical)
	raw := "sha256:" + hex.EncodeToString(sum[:])
	return typedmemory.NewSHA256Digest(raw)
}

func exactOneLine(label string, raw string) (string, error) {
	if raw == "" || raw != strings.TrimSpace(raw) {
		return "", fmt.Errorf("%s is required and must be exact", label)
	}
	if strings.ContainsAny(raw, "\r\n\t") {
		return "", fmt.Errorf("%s must be one line", label)
	}
	return raw, nil
}

func sortAdapterAdmissions(values []AdapterAdmission) {
	// This is only canonical representation order. It does not imply causal,
	// temporal, method, project, or performed-Work order.
	sort.Slice(values, func(left int, right int) bool {
		return values[left].receipt.eventRef < values[right].receipt.eventRef
	})
}

func collectRelationPaths(admissions []AdapterAdmission) []RelationPath {
	result := make([]RelationPath, 0)
	for _, admission := range admissions {
		result = append(result, admission.paths...)
	}
	sort.Slice(result, func(left int, right int) bool {
		return relationPathKey(result[left]) < relationPathKey(result[right])
	})
	return result
}

func collectValueWitnesses(admissions []AdapterAdmission) []ValueWitness {
	result := make([]ValueWitness, 0)
	for _, admission := range admissions {
		result = append(result, admission.values...)
	}
	sort.Slice(result, func(left int, right int) bool {
		return valueWitnessKey(result[left]) < valueWitnessKey(result[right])
	})
	return result
}

func relationPathKey(path RelationPath) string {
	return path.signature.String() +
		"\x00" +
		path.assertion.String() +
		"\x00" +
		path.slot.String() +
		"\x00" +
		path.target.RefKind().String() +
		"\x00" +
		path.target.ReferenceID().String() +
		"\x00" +
		path.admissionEventRef
}

func valueWitnessKey(value ValueWitness) string {
	return value.signature.String() +
		"\x00" +
		value.assertion.String() +
		"\x00" +
		value.slot.String() +
		"\x00" +
		value.inputDigest.String() +
		"\x00" +
		value.admissionEventRef
}
