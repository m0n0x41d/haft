package projecttypeenvselectioneffect

import (
	"fmt"
	"math"

	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projecttypeenvactivation"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	activationDeltaRefPrefix = "project-typeenv-activation-delta:"

	activationEnvelopeDomain    = "haft.project-typeenv.activation-admission-envelope.v1"
	activationEnvelopeRefPrefix = "project-typeenv-activation-envelope:"

	activationBasisDomain    = "haft.project-typeenv.activation-admission-basis.v3"
	activationBasisRefPrefix = "project-typeenv-activation-basis:"

	activationManifestDomain    = "haft.project-typeenv.activation-materialization-manifest.v1"
	activationManifestRefPrefix = "project-typeenv-activation-manifest:"

	ProjectTypeEnvActivationAdmissionKindSnapshotOnly = "snapshot_only"
	ProjectTypeEnvActivationEventKind                 = "activate_type_env"
	ProjectTypeEnvActivationLegacyAuthorityClass      = projecttypeenvactivation.LegacyManualAuthorityClass
	ProjectTypeEnvActivationHostRoutedAuthorityClass  = projecttypeenvactivation.HostRoutedOperatorRequestAuthorityClass
	ProjectTypeEnvActivationCompatibleAuthorityClass  = projecttypeenvactivation.CompatibleSuccessorPolicyAuthorityClass
	// ProjectTypeEnvActivationAuthorityClass names the decode-only v1/v2
	// carrier value retained for frozen-history tests.
	ProjectTypeEnvActivationAuthorityClass         = ProjectTypeEnvActivationLegacyAuthorityClass
	ProjectTypeEnvActivationMaterializationOrdinal = uint32(0)
)

type ProjectTypeEnvActivationDeltaRef struct {
	digest typedmemory.SHA256Digest
}

func ParseProjectTypeEnvActivationDeltaRef(
	raw string,
) (ProjectTypeEnvActivationDeltaRef, error) {
	digest, err := parseTypedDigestRef("activation delta", activationDeltaRefPrefix, raw)
	if err != nil {
		return ProjectTypeEnvActivationDeltaRef{}, err
	}
	return ProjectTypeEnvActivationDeltaRef{digest: digest}, nil
}

func (ref ProjectTypeEnvActivationDeltaRef) Digest() typedmemory.SHA256Digest {
	return ref.digest
}

func (ref ProjectTypeEnvActivationDeltaRef) String() string {
	return activationDeltaRefPrefix + ref.digest.String()
}

// ProjectTypeEnvActivationDelta is the one semantic activation row. It cannot
// express a generic entity/relation no-op and always advances GraphRevision by
// exactly one.
type ProjectTypeEnvActivationDelta struct {
	ref                    ProjectTypeEnvActivationDeltaRef
	digest                 typedmemory.SHA256Digest
	transactionRef         ProjectTypeEnvHeadSelectionTransactionRef
	transactionDigest      typedmemory.SHA256Digest
	project                projectidentity.ProjectID
	head                   projecttypeenvselection.ProjectTypeEnvHeadRef
	requestRef             projecttypeenvselection.ProjectTypeEnvHeadSelectionRequestRef
	requestDigest          typedmemory.SHA256Digest
	contentDigest          authority.Digest
	authorityUseRef        ProjectTypeEnvHeadSelectionAuthorityUseRecordRef
	workRef                authority.WorkRef
	workRecordRef          ProjectTypeEnvHeadCASWorkRecordRef
	predecessor            projecttypeenvselection.ProjectTypeEnvHeadSelectionPredecessor
	target                 ProjectTypeEnvHeadSelectionTarget
	expectedGraphRevision  typedmemory.GraphRevision
	committedGraphRevision typedmemory.GraphRevision
	successorHeadRevision  projecttypeenvselection.HeadRevision
	authorityClass         string
	canonicalBytes         []byte
}

type ProjectTypeEnvActivationDeltaInput struct {
	Identity              ProjectTypeEnvHeadSelectionTransactionIdentity
	ReferenceDAG          ProjectTypeEnvHeadSelectionReferenceDAG
	Head                  projecttypeenvselection.ProjectTypeEnvHeadRef
	Predecessor           projecttypeenvselection.ProjectTypeEnvHeadSelectionPredecessor
	Target                ProjectTypeEnvHeadSelectionTarget
	ExpectedGraphRevision typedmemory.GraphRevision
	AuthorityClass        string
}

func SealProjectTypeEnvActivationDelta(
	input ProjectTypeEnvActivationDeltaInput,
) (ProjectTypeEnvActivationDelta, error) {
	if err := input.Identity.Verify(); err != nil {
		return ProjectTypeEnvActivationDelta{}, err
	}
	if err := input.ReferenceDAG.Verify(input.Identity); err != nil {
		return ProjectTypeEnvActivationDelta{}, err
	}
	head, err := projecttypeenvselection.ParseProjectTypeEnvHeadRef(input.Head.String())
	if err != nil || head != input.Head || head.Project() != input.Identity.Project() {
		return ProjectTypeEnvActivationDelta{}, fmt.Errorf("activation head/project mismatch")
	}
	target, err := NewProjectTypeEnvHeadSelectionTarget(
		ProjectTypeEnvHeadSelectionTargetInput{
			Base:              input.Target.Base(),
			OrderedExtensions: input.Target.OrderedExtensions(),
			RuntimeBasis:      input.Target.RuntimeBasis(),
			Composite:         input.Target.Composite(),
			Stage:             input.Target.Stage(),
		},
	)
	if err != nil {
		return ProjectTypeEnvActivationDelta{}, err
	}
	expected := input.ExpectedGraphRevision
	if expected.Value() == math.MaxUint64 {
		return ProjectTypeEnvActivationDelta{}, fmt.Errorf("expected GraphRevision overflows")
	}
	committed := typedmemory.NewGraphRevision(expected.Value() + 1)
	if committed != input.Identity.CommittedGraphRevision() {
		return ProjectTypeEnvActivationDelta{}, fmt.Errorf(
			"activation committed GraphRevision differs from transaction identity",
		)
	}
	coreTarget, err := projecttypeenvactivation.NewTarget(
		projecttypeenvactivation.TargetInput{
			Base:              target.Base(),
			OrderedExtensions: target.OrderedExtensions(),
			RuntimeBasis:      target.RuntimeBasis(),
			Composite:         target.Composite(),
			Stage:             target.Stage(),
		},
	)
	if err != nil {
		return ProjectTypeEnvActivationDelta{}, err
	}
	coreDelta, err := projecttypeenvactivation.NewDelta(
		projecttypeenvactivation.DeltaInput{
			TransactionRef:         input.Identity.Ref().String(),
			TransactionDigest:      input.Identity.Digest(),
			Project:                input.Identity.Project(),
			Head:                   head,
			RequestRef:             input.Identity.RequestRef(),
			RequestDigest:          input.Identity.RequestDigest(),
			ContentDigest:          input.Identity.ContentDigest(),
			AuthorityUseRef:        input.ReferenceDAG.AuthorityUseRecordRef().String(),
			WorkRef:                input.ReferenceDAG.WorkRef(),
			WorkRecordRef:          input.ReferenceDAG.CASWorkRecordRef().String(),
			Predecessor:            input.Predecessor,
			Target:                 coreTarget,
			ExpectedGraphRevision:  expected,
			CommittedGraphRevision: committed,
			SuccessorHeadRevision:  input.Identity.SuccessorHeadRevision(),
			AuthorityClass:         input.AuthorityClass,
		},
	)
	if err != nil {
		return ProjectTypeEnvActivationDelta{}, err
	}
	return DecodeProjectTypeEnvActivationDelta(coreDelta.CanonicalBytes())
}

func DecodeProjectTypeEnvActivationDelta(
	canonical []byte,
) (ProjectTypeEnvActivationDelta, error) {
	coreDelta, err := projecttypeenvactivation.DecodeDelta(canonical)
	if err != nil {
		return ProjectTypeEnvActivationDelta{}, err
	}
	transactionRef, err := ParseProjectTypeEnvHeadSelectionTransactionRef(
		coreDelta.TransactionRef(),
	)
	if err != nil {
		return ProjectTypeEnvActivationDelta{}, err
	}
	authorityUseRef, err := ParseProjectTypeEnvHeadSelectionAuthorityUseRecordRef(
		coreDelta.AuthorityUseRef(),
	)
	if err != nil {
		return ProjectTypeEnvActivationDelta{}, err
	}
	workRecordRef, err := ParseProjectTypeEnvHeadCASWorkRecordRef(
		coreDelta.WorkRecordRef(),
	)
	if err != nil {
		return ProjectTypeEnvActivationDelta{}, err
	}
	target, err := NewProjectTypeEnvHeadSelectionTarget(
		ProjectTypeEnvHeadSelectionTargetInput{
			Base:              coreDelta.Target().Base(),
			OrderedExtensions: coreDelta.Target().OrderedExtensions(),
			RuntimeBasis:      coreDelta.Target().RuntimeBasis(),
			Composite:         coreDelta.Target().Composite(),
			Stage:             coreDelta.Target().Stage(),
		},
	)
	if err != nil {
		return ProjectTypeEnvActivationDelta{}, err
	}
	return ProjectTypeEnvActivationDelta{
		ref:                    ProjectTypeEnvActivationDeltaRef{digest: coreDelta.Digest()},
		digest:                 coreDelta.Digest(),
		transactionRef:         transactionRef,
		transactionDigest:      coreDelta.TransactionDigest(),
		project:                coreDelta.Project(),
		head:                   coreDelta.Head(),
		requestRef:             coreDelta.RequestRef(),
		requestDigest:          coreDelta.RequestDigest(),
		contentDigest:          coreDelta.ContentDigest(),
		authorityUseRef:        authorityUseRef,
		workRef:                coreDelta.WorkRef(),
		workRecordRef:          workRecordRef,
		predecessor:            coreDelta.Predecessor(),
		target:                 target,
		expectedGraphRevision:  coreDelta.ExpectedGraphRevision(),
		committedGraphRevision: coreDelta.CommittedGraphRevision(),
		successorHeadRevision:  coreDelta.SuccessorHeadRevision(),
		authorityClass:         coreDelta.AuthorityClass(),
		canonicalBytes:         coreDelta.CanonicalBytes(),
	}, nil
}

func (delta ProjectTypeEnvActivationDelta) Ref() ProjectTypeEnvActivationDeltaRef {
	return delta.ref
}

func (delta ProjectTypeEnvActivationDelta) Digest() typedmemory.SHA256Digest {
	return delta.digest
}

func (delta ProjectTypeEnvActivationDelta) TransactionRef() ProjectTypeEnvHeadSelectionTransactionRef {
	return delta.transactionRef
}

func (delta ProjectTypeEnvActivationDelta) TransactionDigest() typedmemory.SHA256Digest {
	return delta.transactionDigest
}

func (delta ProjectTypeEnvActivationDelta) Project() projectidentity.ProjectID {
	return delta.project
}

func (delta ProjectTypeEnvActivationDelta) Head() projecttypeenvselection.ProjectTypeEnvHeadRef {
	return delta.head
}

func (delta ProjectTypeEnvActivationDelta) RequestRef() projecttypeenvselection.ProjectTypeEnvHeadSelectionRequestRef {
	return delta.requestRef
}

func (delta ProjectTypeEnvActivationDelta) RequestDigest() typedmemory.SHA256Digest {
	return delta.requestDigest
}

func (delta ProjectTypeEnvActivationDelta) ContentDigest() authority.Digest {
	return delta.contentDigest
}

func (delta ProjectTypeEnvActivationDelta) AuthorityUseRecordRef() ProjectTypeEnvHeadSelectionAuthorityUseRecordRef {
	return delta.authorityUseRef
}

func (delta ProjectTypeEnvActivationDelta) WorkRef() authority.WorkRef {
	return delta.workRef
}

func (delta ProjectTypeEnvActivationDelta) CASWorkRecordRef() ProjectTypeEnvHeadCASWorkRecordRef {
	return delta.workRecordRef
}

func (delta ProjectTypeEnvActivationDelta) Predecessor() projecttypeenvselection.ProjectTypeEnvHeadSelectionPredecessor {
	return delta.predecessor
}

func (delta ProjectTypeEnvActivationDelta) Target() ProjectTypeEnvHeadSelectionTarget {
	return delta.target
}

func (delta ProjectTypeEnvActivationDelta) ExpectedGraphRevision() typedmemory.GraphRevision {
	return delta.expectedGraphRevision
}

func (delta ProjectTypeEnvActivationDelta) CommittedGraphRevision() typedmemory.GraphRevision {
	return delta.committedGraphRevision
}

func (delta ProjectTypeEnvActivationDelta) SuccessorHeadRevision() projecttypeenvselection.HeadRevision {
	return delta.successorHeadRevision
}

func (delta ProjectTypeEnvActivationDelta) EventKind() string {
	return ProjectTypeEnvActivationEventKind
}

func (delta ProjectTypeEnvActivationDelta) AuthorityClass() string {
	return delta.authorityClass
}

func (delta ProjectTypeEnvActivationDelta) CanonicalBytes() []byte {
	return append([]byte(nil), delta.canonicalBytes...)
}

func (delta ProjectTypeEnvActivationDelta) Verify() error {
	decoded, err := DecodeProjectTypeEnvActivationDelta(delta.canonicalBytes)
	if err != nil {
		return err
	}
	if decoded.ref != delta.ref || decoded.digest != delta.digest {
		return fmt.Errorf("activation delta differs from canonical bytes")
	}
	return nil
}

func (delta ProjectTypeEnvActivationDelta) ExactFor(
	identity ProjectTypeEnvHeadSelectionTransactionIdentity,
	dag ProjectTypeEnvHeadSelectionReferenceDAG,
) bool {
	if delta.Verify() != nil ||
		identity.Verify() != nil ||
		dag.Verify(identity) != nil {
		return false
	}
	return delta.transactionRef == identity.Ref() &&
		delta.transactionDigest == identity.Digest() &&
		delta.project == identity.Project() &&
		delta.requestRef == identity.RequestRef() &&
		delta.requestDigest == identity.RequestDigest() &&
		delta.contentDigest == identity.ContentDigest() &&
		delta.authorityUseRef == dag.AuthorityUseRecordRef() &&
		delta.workRef == dag.WorkRef() &&
		delta.workRecordRef == dag.CASWorkRecordRef() &&
		delta.committedGraphRevision == identity.CommittedGraphRevision() &&
		delta.successorHeadRevision == identity.SuccessorHeadRevision()
}

type ProjectTypeEnvActivationAdmissionEnvelopeRef struct {
	digest typedmemory.SHA256Digest
}

func ParseProjectTypeEnvActivationAdmissionEnvelopeRef(
	raw string,
) (ProjectTypeEnvActivationAdmissionEnvelopeRef, error) {
	digest, err := parseTypedDigestRef(
		"activation admission envelope",
		activationEnvelopeRefPrefix,
		raw,
	)
	if err != nil {
		return ProjectTypeEnvActivationAdmissionEnvelopeRef{}, err
	}
	return ProjectTypeEnvActivationAdmissionEnvelopeRef{digest: digest}, nil
}

func (ref ProjectTypeEnvActivationAdmissionEnvelopeRef) Digest() typedmemory.SHA256Digest {
	return ref.digest
}

func (ref ProjectTypeEnvActivationAdmissionEnvelopeRef) String() string {
	return activationEnvelopeRefPrefix + ref.digest.String()
}

type ProjectTypeEnvActivationAdmissionEnvelope struct {
	ref            ProjectTypeEnvActivationAdmissionEnvelopeRef
	digest         typedmemory.SHA256Digest
	deltaRef       ProjectTypeEnvActivationDeltaRef
	deltaDigest    typedmemory.SHA256Digest
	requestRef     projecttypeenvselection.ProjectTypeEnvHeadSelectionRequestRef
	requestDigest  typedmemory.SHA256Digest
	target         typedmemory.TypeEnvRef
	stage          projecttypeenvselection.ProjectTypeEnvStageRef
	graphKey       GraphActivationIdempotencyKey
	canonicalBytes []byte
}

func SealProjectTypeEnvActivationAdmissionEnvelope(
	delta ProjectTypeEnvActivationDelta,
	dag ProjectTypeEnvHeadSelectionReferenceDAG,
) (ProjectTypeEnvActivationAdmissionEnvelope, error) {
	if err := delta.Verify(); err != nil {
		return ProjectTypeEnvActivationAdmissionEnvelope{}, err
	}
	if dag.GraphIdempotencyKey().String() == "" ||
		delta.AuthorityUseRecordRef() != dag.AuthorityUseRecordRef() ||
		delta.WorkRef() != dag.WorkRef() ||
		delta.CASWorkRecordRef() != dag.CASWorkRecordRef() {
		return ProjectTypeEnvActivationAdmissionEnvelope{},
			fmt.Errorf("activation envelope reference DAG mismatch")
	}
	coreDelta, err := projecttypeenvactivation.DecodeDelta(
		delta.CanonicalBytes(),
	)
	if err != nil {
		return ProjectTypeEnvActivationAdmissionEnvelope{}, err
	}
	coreEnvelope, err := projecttypeenvactivation.NewAdmissionEnvelope(
		coreDelta,
		dag.GraphIdempotencyKey().String(),
	)
	if err != nil {
		return ProjectTypeEnvActivationAdmissionEnvelope{}, err
	}
	return DecodeProjectTypeEnvActivationAdmissionEnvelope(
		coreEnvelope.CanonicalBytes(),
	)
}

func DecodeProjectTypeEnvActivationAdmissionEnvelope(
	canonical []byte,
) (ProjectTypeEnvActivationAdmissionEnvelope, error) {
	reader, err := newCanonicalReader(canonical, activationEnvelopeDomain)
	if err != nil {
		return ProjectTypeEnvActivationAdmissionEnvelope{}, err
	}
	kind, err := reader.readString("activation envelope kind")
	if err != nil || kind != ProjectTypeEnvActivationAdmissionKindSnapshotOnly {
		return ProjectTypeEnvActivationAdmissionEnvelope{},
			fmt.Errorf("activation envelope kind is invalid")
	}
	deltaText, err := reader.readString("activation envelope delta ref")
	if err != nil {
		return ProjectTypeEnvActivationAdmissionEnvelope{}, err
	}
	deltaRef, err := ParseProjectTypeEnvActivationDeltaRef(deltaText)
	if err != nil {
		return ProjectTypeEnvActivationAdmissionEnvelope{}, err
	}
	deltaDigestText, err := reader.readString("activation envelope delta digest")
	if err != nil {
		return ProjectTypeEnvActivationAdmissionEnvelope{}, err
	}
	deltaDigest, err := typedmemory.NewSHA256Digest(deltaDigestText)
	if err != nil || deltaRef.Digest() != deltaDigest {
		return ProjectTypeEnvActivationAdmissionEnvelope{},
			fmt.Errorf("activation envelope delta ref/digest mismatch")
	}
	requestText, err := reader.readString("activation envelope request ref")
	if err != nil {
		return ProjectTypeEnvActivationAdmissionEnvelope{}, err
	}
	requestRef, err := projecttypeenvselection.ParseProjectTypeEnvHeadSelectionRequestRef(
		requestText,
	)
	if err != nil {
		return ProjectTypeEnvActivationAdmissionEnvelope{}, err
	}
	requestDigestText, err := reader.readString("activation envelope request digest")
	if err != nil {
		return ProjectTypeEnvActivationAdmissionEnvelope{}, err
	}
	requestDigest, err := typedmemory.NewSHA256Digest(requestDigestText)
	if err != nil || requestRef.Digest() != requestDigest {
		return ProjectTypeEnvActivationAdmissionEnvelope{},
			fmt.Errorf("activation envelope request ref/digest mismatch")
	}
	targetText, err := reader.readString("activation envelope target")
	if err != nil {
		return ProjectTypeEnvActivationAdmissionEnvelope{}, err
	}
	target, err := typedmemory.ParseTypeEnvRef(targetText)
	if err != nil {
		return ProjectTypeEnvActivationAdmissionEnvelope{}, err
	}
	stageText, err := reader.readString("activation envelope Stage")
	if err != nil {
		return ProjectTypeEnvActivationAdmissionEnvelope{}, err
	}
	stage, err := projecttypeenvselection.ParseProjectTypeEnvStageRef(stageText)
	if err != nil {
		return ProjectTypeEnvActivationAdmissionEnvelope{}, err
	}
	graphKeyText, err := reader.readString("activation envelope graph key")
	if err != nil {
		return ProjectTypeEnvActivationAdmissionEnvelope{}, err
	}
	graphKey, err := ParseGraphActivationIdempotencyKey(graphKeyText)
	if err != nil {
		return ProjectTypeEnvActivationAdmissionEnvelope{}, err
	}
	if err := reader.requireEnd("activation admission envelope"); err != nil {
		return ProjectTypeEnvActivationAdmissionEnvelope{}, err
	}
	digest, err := digestCanonical(activationEnvelopeDomain, canonical)
	if err != nil {
		return ProjectTypeEnvActivationAdmissionEnvelope{}, err
	}
	return ProjectTypeEnvActivationAdmissionEnvelope{
		ref:            ProjectTypeEnvActivationAdmissionEnvelopeRef{digest: digest},
		digest:         digest,
		deltaRef:       deltaRef,
		deltaDigest:    deltaDigest,
		requestRef:     requestRef,
		requestDigest:  requestDigest,
		target:         target,
		stage:          stage,
		graphKey:       graphKey,
		canonicalBytes: append([]byte(nil), canonical...),
	}, nil
}

func (value ProjectTypeEnvActivationAdmissionEnvelope) Ref() ProjectTypeEnvActivationAdmissionEnvelopeRef {
	return value.ref
}

func (value ProjectTypeEnvActivationAdmissionEnvelope) Digest() typedmemory.SHA256Digest {
	return value.digest
}

func (value ProjectTypeEnvActivationAdmissionEnvelope) DeltaRef() ProjectTypeEnvActivationDeltaRef {
	return value.deltaRef
}

func (value ProjectTypeEnvActivationAdmissionEnvelope) DeltaDigest() typedmemory.SHA256Digest {
	return value.deltaDigest
}

func (value ProjectTypeEnvActivationAdmissionEnvelope) RequestRef() projecttypeenvselection.ProjectTypeEnvHeadSelectionRequestRef {
	return value.requestRef
}

func (value ProjectTypeEnvActivationAdmissionEnvelope) RequestDigest() typedmemory.SHA256Digest {
	return value.requestDigest
}

func (value ProjectTypeEnvActivationAdmissionEnvelope) TargetComposite() typedmemory.TypeEnvRef {
	return value.target
}

func (value ProjectTypeEnvActivationAdmissionEnvelope) Stage() projecttypeenvselection.ProjectTypeEnvStageRef {
	return value.stage
}

func (value ProjectTypeEnvActivationAdmissionEnvelope) GraphIdempotencyKey() GraphActivationIdempotencyKey {
	return value.graphKey
}

func (value ProjectTypeEnvActivationAdmissionEnvelope) AdmissionKind() string {
	return ProjectTypeEnvActivationAdmissionKindSnapshotOnly
}

func (value ProjectTypeEnvActivationAdmissionEnvelope) CanonicalBytes() []byte {
	return append([]byte(nil), value.canonicalBytes...)
}

func (value ProjectTypeEnvActivationAdmissionEnvelope) Verify() error {
	decoded, err := DecodeProjectTypeEnvActivationAdmissionEnvelope(value.canonicalBytes)
	if err != nil {
		return err
	}
	if decoded.ref != value.ref || decoded.digest != value.digest {
		return fmt.Errorf("activation envelope differs from canonical bytes")
	}
	return nil
}

type ProjectTypeEnvActivationAdmissionBasisRef struct {
	digest typedmemory.SHA256Digest
}

func ParseProjectTypeEnvActivationAdmissionBasisRef(
	raw string,
) (ProjectTypeEnvActivationAdmissionBasisRef, error) {
	digest, err := parseTypedDigestRef(
		"activation admission basis",
		activationBasisRefPrefix,
		raw,
	)
	if err != nil {
		return ProjectTypeEnvActivationAdmissionBasisRef{}, err
	}
	return ProjectTypeEnvActivationAdmissionBasisRef{digest: digest}, nil
}

func (ref ProjectTypeEnvActivationAdmissionBasisRef) Digest() typedmemory.SHA256Digest {
	return ref.digest
}

func (ref ProjectTypeEnvActivationAdmissionBasisRef) String() string {
	return activationBasisRefPrefix + ref.digest.String()
}

type ProjectTypeEnvActivationAdmissionBasis struct {
	ref                   ProjectTypeEnvActivationAdmissionBasisRef
	digest                typedmemory.SHA256Digest
	envelopeRef           ProjectTypeEnvActivationAdmissionEnvelopeRef
	envelopeDigest        typedmemory.SHA256Digest
	project               projectidentity.ProjectID
	predecessor           projecttypeenvselection.ProjectTypeEnvHeadSelectionPredecessor
	target                typedmemory.TypeEnvRef
	stage                 projecttypeenvselection.ProjectTypeEnvStageRef
	expectedGraphRevision typedmemory.GraphRevision
	canonicalBytes        []byte
}

func SealProjectTypeEnvActivationAdmissionBasis(
	delta ProjectTypeEnvActivationDelta,
	envelope ProjectTypeEnvActivationAdmissionEnvelope,
) (ProjectTypeEnvActivationAdmissionBasis, error) {
	if err := delta.Verify(); err != nil {
		return ProjectTypeEnvActivationAdmissionBasis{}, err
	}
	if err := envelope.Verify(); err != nil {
		return ProjectTypeEnvActivationAdmissionBasis{}, err
	}
	if envelope.DeltaRef() != delta.Ref() ||
		envelope.DeltaDigest() != delta.Digest() ||
		envelope.TargetComposite() != delta.Target().Composite() ||
		envelope.Stage() != delta.Target().Stage() {
		return ProjectTypeEnvActivationAdmissionBasis{},
			fmt.Errorf("activation basis envelope/delta mismatch")
	}
	coreDelta, err := projecttypeenvactivation.DecodeDelta(
		delta.CanonicalBytes(),
	)
	if err != nil {
		return ProjectTypeEnvActivationAdmissionBasis{}, err
	}
	coreEnvelope, err := projecttypeenvactivation.DecodeAdmissionEnvelope(
		envelope.CanonicalBytes(),
	)
	if err != nil {
		return ProjectTypeEnvActivationAdmissionBasis{}, err
	}
	coreBasis, err := projecttypeenvactivation.NewAdmissionBasis(
		coreDelta,
		coreEnvelope,
	)
	if err != nil {
		return ProjectTypeEnvActivationAdmissionBasis{}, err
	}
	return DecodeProjectTypeEnvActivationAdmissionBasis(
		coreBasis.CanonicalBytes(),
	)
}

func DecodeProjectTypeEnvActivationAdmissionBasis(
	canonical []byte,
) (ProjectTypeEnvActivationAdmissionBasis, error) {
	coreBasis, err := projecttypeenvactivation.DecodeAdmissionBasis(canonical)
	if err != nil {
		return ProjectTypeEnvActivationAdmissionBasis{}, err
	}
	envelopeRef, err := ParseProjectTypeEnvActivationAdmissionEnvelopeRef(
		coreBasis.EnvelopeRef().String(),
	)
	if err != nil {
		return ProjectTypeEnvActivationAdmissionBasis{}, err
	}
	return ProjectTypeEnvActivationAdmissionBasis{
		ref:                   ProjectTypeEnvActivationAdmissionBasisRef{digest: coreBasis.Digest()},
		digest:                coreBasis.Digest(),
		envelopeRef:           envelopeRef,
		envelopeDigest:        coreBasis.EnvelopeDigest(),
		project:               coreBasis.Project(),
		predecessor:           coreBasis.Predecessor(),
		target:                coreBasis.TargetComposite(),
		stage:                 coreBasis.Stage(),
		expectedGraphRevision: coreBasis.ExpectedGraphRevision(),
		canonicalBytes:        coreBasis.CanonicalBytes(),
	}, nil
}

func (value ProjectTypeEnvActivationAdmissionBasis) Ref() ProjectTypeEnvActivationAdmissionBasisRef {
	return value.ref
}

func (value ProjectTypeEnvActivationAdmissionBasis) Digest() typedmemory.SHA256Digest {
	return value.digest
}

func (value ProjectTypeEnvActivationAdmissionBasis) EnvelopeRef() ProjectTypeEnvActivationAdmissionEnvelopeRef {
	return value.envelopeRef
}

func (value ProjectTypeEnvActivationAdmissionBasis) EnvelopeDigest() typedmemory.SHA256Digest {
	return value.envelopeDigest
}

func (value ProjectTypeEnvActivationAdmissionBasis) Project() projectidentity.ProjectID {
	return value.project
}

func (value ProjectTypeEnvActivationAdmissionBasis) Predecessor() projecttypeenvselection.ProjectTypeEnvHeadSelectionPredecessor {
	return value.predecessor
}

func (value ProjectTypeEnvActivationAdmissionBasis) TargetComposite() typedmemory.TypeEnvRef {
	return value.target
}

func (value ProjectTypeEnvActivationAdmissionBasis) Stage() projecttypeenvselection.ProjectTypeEnvStageRef {
	return value.stage
}

func (value ProjectTypeEnvActivationAdmissionBasis) ExpectedGraphRevision() typedmemory.GraphRevision {
	return value.expectedGraphRevision
}

func (value ProjectTypeEnvActivationAdmissionBasis) AdmissionKind() string {
	return ProjectTypeEnvActivationAdmissionKindSnapshotOnly
}

func (value ProjectTypeEnvActivationAdmissionBasis) CanonicalBytes() []byte {
	return append([]byte(nil), value.canonicalBytes...)
}

func (value ProjectTypeEnvActivationAdmissionBasis) Verify() error {
	decoded, err := DecodeProjectTypeEnvActivationAdmissionBasis(value.canonicalBytes)
	if err != nil {
		return err
	}
	if decoded.ref != value.ref || decoded.digest != value.digest {
		return fmt.Errorf("activation basis differs from canonical bytes")
	}
	return nil
}

type ProjectTypeEnvActivationMaterializationManifestRef struct {
	digest typedmemory.SHA256Digest
}

func ParseProjectTypeEnvActivationMaterializationManifestRef(
	raw string,
) (ProjectTypeEnvActivationMaterializationManifestRef, error) {
	digest, err := parseTypedDigestRef(
		"activation materialization manifest",
		activationManifestRefPrefix,
		raw,
	)
	if err != nil {
		return ProjectTypeEnvActivationMaterializationManifestRef{}, err
	}
	return ProjectTypeEnvActivationMaterializationManifestRef{digest: digest}, nil
}

func (ref ProjectTypeEnvActivationMaterializationManifestRef) Digest() typedmemory.SHA256Digest {
	return ref.digest
}

func (ref ProjectTypeEnvActivationMaterializationManifestRef) String() string {
	return activationManifestRefPrefix + ref.digest.String()
}

type ProjectTypeEnvActivationMaterializationManifest struct {
	ref            ProjectTypeEnvActivationMaterializationManifestRef
	digest         typedmemory.SHA256Digest
	deltaRef       ProjectTypeEnvActivationDeltaRef
	deltaDigest    typedmemory.SHA256Digest
	envelopeRef    ProjectTypeEnvActivationAdmissionEnvelopeRef
	envelopeDigest typedmemory.SHA256Digest
	basisRef       ProjectTypeEnvActivationAdmissionBasisRef
	basisDigest    typedmemory.SHA256Digest
	event          projecttypeenvselection.GraphEventRef
	commit         projecttypeenvselection.GraphCommitRef
	canonicalBytes []byte
}

// ProjectTypeEnvActivationGraphCoordinates are storage-owned generic graph
// identities. The effect core binds them only after typed-memory preparation;
// it never derives a competing event or commit identity domain.
type ProjectTypeEnvActivationGraphCoordinates struct {
	event  projecttypeenvselection.GraphEventRef
	commit projecttypeenvselection.GraphCommitRef
}

type ProjectTypeEnvActivationGraphCoordinatesInput struct {
	Event  projecttypeenvselection.GraphEventRef
	Commit projecttypeenvselection.GraphCommitRef
}

func NewProjectTypeEnvActivationGraphCoordinates(
	input ProjectTypeEnvActivationGraphCoordinatesInput,
) (ProjectTypeEnvActivationGraphCoordinates, error) {
	event, err := projecttypeenvselection.ParseGraphEventRef(input.Event.String())
	if err != nil || event != input.Event {
		return ProjectTypeEnvActivationGraphCoordinates{},
			fmt.Errorf("storage-owned graph event ref is required")
	}
	commit, err := projecttypeenvselection.ParseGraphCommitRef(input.Commit.String())
	if err != nil || commit != input.Commit {
		return ProjectTypeEnvActivationGraphCoordinates{},
			fmt.Errorf("storage-owned graph commit ref is required")
	}
	return ProjectTypeEnvActivationGraphCoordinates{
		event:  event,
		commit: commit,
	}, nil
}

func (value ProjectTypeEnvActivationGraphCoordinates) EventRef() projecttypeenvselection.GraphEventRef {
	return value.event
}

func (value ProjectTypeEnvActivationGraphCoordinates) CommitRef() projecttypeenvselection.GraphCommitRef {
	return value.commit
}

func SealProjectTypeEnvActivationMaterializationManifest(
	delta ProjectTypeEnvActivationDelta,
	envelope ProjectTypeEnvActivationAdmissionEnvelope,
	basis ProjectTypeEnvActivationAdmissionBasis,
	graph ProjectTypeEnvActivationGraphCoordinates,
) (ProjectTypeEnvActivationMaterializationManifest, error) {
	if err := delta.Verify(); err != nil {
		return ProjectTypeEnvActivationMaterializationManifest{}, err
	}
	if err := envelope.Verify(); err != nil {
		return ProjectTypeEnvActivationMaterializationManifest{}, err
	}
	if err := basis.Verify(); err != nil {
		return ProjectTypeEnvActivationMaterializationManifest{}, err
	}
	if envelope.DeltaRef() != delta.Ref() ||
		envelope.DeltaDigest() != delta.Digest() ||
		basis.EnvelopeRef() != envelope.Ref() ||
		basis.EnvelopeDigest() != envelope.Digest() ||
		graph.EventRef().String() == "" ||
		graph.CommitRef().String() == "" {
		return ProjectTypeEnvActivationMaterializationManifest{},
			fmt.Errorf("activation manifest member mismatch")
	}
	coreDelta, err := projecttypeenvactivation.DecodeDelta(
		delta.CanonicalBytes(),
	)
	if err != nil {
		return ProjectTypeEnvActivationMaterializationManifest{}, err
	}
	coreEnvelope, err := projecttypeenvactivation.DecodeAdmissionEnvelope(
		envelope.CanonicalBytes(),
	)
	if err != nil {
		return ProjectTypeEnvActivationMaterializationManifest{}, err
	}
	coreBasis, err := projecttypeenvactivation.DecodeAdmissionBasis(
		basis.CanonicalBytes(),
	)
	if err != nil {
		return ProjectTypeEnvActivationMaterializationManifest{}, err
	}
	coreManifest, err := projecttypeenvactivation.NewMaterializationManifest(
		coreDelta,
		coreEnvelope,
		coreBasis,
		graph.EventRef(),
		graph.CommitRef(),
	)
	if err != nil {
		return ProjectTypeEnvActivationMaterializationManifest{}, err
	}
	return DecodeProjectTypeEnvActivationMaterializationManifest(
		coreManifest.CanonicalBytes(),
	)
}

func DecodeProjectTypeEnvActivationMaterializationManifest(
	canonical []byte,
) (ProjectTypeEnvActivationMaterializationManifest, error) {
	reader, err := newCanonicalReader(canonical, activationManifestDomain)
	if err != nil {
		return ProjectTypeEnvActivationMaterializationManifest{}, err
	}
	deltaText, err := reader.readString("activation manifest delta ref")
	if err != nil {
		return ProjectTypeEnvActivationMaterializationManifest{}, err
	}
	deltaRef, err := ParseProjectTypeEnvActivationDeltaRef(deltaText)
	if err != nil {
		return ProjectTypeEnvActivationMaterializationManifest{}, err
	}
	deltaDigestText, err := reader.readString("activation manifest delta digest")
	if err != nil {
		return ProjectTypeEnvActivationMaterializationManifest{}, err
	}
	deltaDigest, err := typedmemory.NewSHA256Digest(deltaDigestText)
	if err != nil || deltaRef.Digest() != deltaDigest {
		return ProjectTypeEnvActivationMaterializationManifest{},
			fmt.Errorf("activation manifest delta ref/digest mismatch")
	}
	envelopeText, err := reader.readString("activation manifest envelope ref")
	if err != nil {
		return ProjectTypeEnvActivationMaterializationManifest{}, err
	}
	envelopeRef, err := ParseProjectTypeEnvActivationAdmissionEnvelopeRef(envelopeText)
	if err != nil {
		return ProjectTypeEnvActivationMaterializationManifest{}, err
	}
	envelopeDigestText, err := reader.readString("activation manifest envelope digest")
	if err != nil {
		return ProjectTypeEnvActivationMaterializationManifest{}, err
	}
	envelopeDigest, err := typedmemory.NewSHA256Digest(envelopeDigestText)
	if err != nil || envelopeRef.Digest() != envelopeDigest {
		return ProjectTypeEnvActivationMaterializationManifest{},
			fmt.Errorf("activation manifest envelope ref/digest mismatch")
	}
	basisText, err := reader.readString("activation manifest basis ref")
	if err != nil {
		return ProjectTypeEnvActivationMaterializationManifest{}, err
	}
	basisRef, err := ParseProjectTypeEnvActivationAdmissionBasisRef(basisText)
	if err != nil {
		return ProjectTypeEnvActivationMaterializationManifest{}, err
	}
	basisDigestText, err := reader.readString("activation manifest basis digest")
	if err != nil {
		return ProjectTypeEnvActivationMaterializationManifest{}, err
	}
	basisDigest, err := typedmemory.NewSHA256Digest(basisDigestText)
	if err != nil || basisRef.Digest() != basisDigest {
		return ProjectTypeEnvActivationMaterializationManifest{},
			fmt.Errorf("activation manifest basis ref/digest mismatch")
	}
	eventText, err := reader.readString("activation manifest event ref")
	if err != nil {
		return ProjectTypeEnvActivationMaterializationManifest{}, err
	}
	event, err := projecttypeenvselection.ParseGraphEventRef(eventText)
	if err != nil {
		return ProjectTypeEnvActivationMaterializationManifest{}, err
	}
	commitText, err := reader.readString("activation manifest commit ref")
	if err != nil {
		return ProjectTypeEnvActivationMaterializationManifest{}, err
	}
	commit, err := projecttypeenvselection.ParseGraphCommitRef(commitText)
	if err != nil {
		return ProjectTypeEnvActivationMaterializationManifest{}, err
	}
	ordinal, err := reader.readUint32("activation manifest ordinal")
	if err != nil || ordinal != ProjectTypeEnvActivationMaterializationOrdinal {
		return ProjectTypeEnvActivationMaterializationManifest{},
			fmt.Errorf("activation manifest ordinal is invalid")
	}
	activationCount, err := reader.readUint32("activation manifest activation count")
	if err != nil || activationCount != 1 {
		return ProjectTypeEnvActivationMaterializationManifest{},
			fmt.Errorf("activation manifest must contain exactly one activation")
	}
	topLevelCount, err := reader.readUint32("activation manifest top-level count")
	if err != nil || topLevelCount != 1 {
		return ProjectTypeEnvActivationMaterializationManifest{},
			fmt.Errorf("activation manifest top-level count must be one")
	}
	rowDigest, err := reader.readString("activation manifest row digest")
	if err != nil || rowDigest != deltaDigest.String() {
		return ProjectTypeEnvActivationMaterializationManifest{},
			fmt.Errorf("activation manifest row digest mismatch")
	}
	if err := reader.requireEnd("activation materialization manifest"); err != nil {
		return ProjectTypeEnvActivationMaterializationManifest{}, err
	}
	digest, err := digestCanonical(activationManifestDomain, canonical)
	if err != nil {
		return ProjectTypeEnvActivationMaterializationManifest{}, err
	}
	return ProjectTypeEnvActivationMaterializationManifest{
		ref: ProjectTypeEnvActivationMaterializationManifestRef{
			digest: digest,
		},
		digest:         digest,
		deltaRef:       deltaRef,
		deltaDigest:    deltaDigest,
		envelopeRef:    envelopeRef,
		envelopeDigest: envelopeDigest,
		basisRef:       basisRef,
		basisDigest:    basisDigest,
		event:          event,
		commit:         commit,
		canonicalBytes: append([]byte(nil), canonical...),
	}, nil
}

func (value ProjectTypeEnvActivationMaterializationManifest) Ref() ProjectTypeEnvActivationMaterializationManifestRef {
	return value.ref
}

func (value ProjectTypeEnvActivationMaterializationManifest) Digest() typedmemory.SHA256Digest {
	return value.digest
}

func (value ProjectTypeEnvActivationMaterializationManifest) DeltaRef() ProjectTypeEnvActivationDeltaRef {
	return value.deltaRef
}

func (value ProjectTypeEnvActivationMaterializationManifest) DeltaDigest() typedmemory.SHA256Digest {
	return value.deltaDigest
}

func (value ProjectTypeEnvActivationMaterializationManifest) EnvelopeRef() ProjectTypeEnvActivationAdmissionEnvelopeRef {
	return value.envelopeRef
}

func (value ProjectTypeEnvActivationMaterializationManifest) EnvelopeDigest() typedmemory.SHA256Digest {
	return value.envelopeDigest
}

func (value ProjectTypeEnvActivationMaterializationManifest) BasisRef() ProjectTypeEnvActivationAdmissionBasisRef {
	return value.basisRef
}

func (value ProjectTypeEnvActivationMaterializationManifest) BasisDigest() typedmemory.SHA256Digest {
	return value.basisDigest
}

func (value ProjectTypeEnvActivationMaterializationManifest) EventRef() projecttypeenvselection.GraphEventRef {
	return value.event
}

func (value ProjectTypeEnvActivationMaterializationManifest) CommitRef() projecttypeenvselection.GraphCommitRef {
	return value.commit
}

func (value ProjectTypeEnvActivationMaterializationManifest) ActivationCount() uint32 {
	return 1
}

func (value ProjectTypeEnvActivationMaterializationManifest) TopLevelChangeCount() uint32 {
	return 1
}

func (value ProjectTypeEnvActivationMaterializationManifest) CanonicalBytes() []byte {
	return append([]byte(nil), value.canonicalBytes...)
}

func (value ProjectTypeEnvActivationMaterializationManifest) Verify() error {
	decoded, err := DecodeProjectTypeEnvActivationMaterializationManifest(
		value.canonicalBytes,
	)
	if err != nil {
		return err
	}
	if decoded.ref != value.ref || decoded.digest != value.digest {
		return fmt.Errorf("activation manifest differs from canonical bytes")
	}
	return nil
}

// CommittedProjectTypeEnvActivation is a sealed in-memory aggregate. It binds
// the activation row to exact graph event/commit/materialization and successor
// head coordinates. It is not a storage observation and does not prove COMMIT.
type CommittedProjectTypeEnvActivation struct {
	identity              ProjectTypeEnvHeadSelectionTransactionIdentity
	dag                   ProjectTypeEnvHeadSelectionReferenceDAG
	delta                 ProjectTypeEnvActivationDelta
	envelope              ProjectTypeEnvActivationAdmissionEnvelope
	basis                 ProjectTypeEnvActivationAdmissionBasis
	manifest              ProjectTypeEnvActivationMaterializationManifest
	successorHead         projecttypeenvselection.ProjectTypeEnvHeadState
	successorHeadDigest   typedmemory.SHA256Digest
	materializationDigest typedmemory.SHA256Digest
}

type CommittedProjectTypeEnvActivationInput struct {
	Identity              ProjectTypeEnvHeadSelectionTransactionIdentity
	ReferenceDAG          ProjectTypeEnvHeadSelectionReferenceDAG
	Delta                 ProjectTypeEnvActivationDelta
	Envelope              ProjectTypeEnvActivationAdmissionEnvelope
	Basis                 ProjectTypeEnvActivationAdmissionBasis
	Manifest              ProjectTypeEnvActivationMaterializationManifest
	SuccessorHead         projecttypeenvselection.ProjectTypeEnvHeadState
	MaterializationDigest typedmemory.SHA256Digest
}

func SealCommittedProjectTypeEnvActivation(
	input CommittedProjectTypeEnvActivationInput,
) (CommittedProjectTypeEnvActivation, error) {
	if err := input.Identity.Verify(); err != nil {
		return CommittedProjectTypeEnvActivation{}, err
	}
	if err := input.ReferenceDAG.Verify(input.Identity); err != nil {
		return CommittedProjectTypeEnvActivation{}, err
	}
	if !input.Delta.ExactFor(input.Identity, input.ReferenceDAG) {
		return CommittedProjectTypeEnvActivation{}, fmt.Errorf(
			"committed activation delta differs from transaction identity",
		)
	}
	if err := input.Envelope.Verify(); err != nil {
		return CommittedProjectTypeEnvActivation{}, err
	}
	if err := input.Basis.Verify(); err != nil {
		return CommittedProjectTypeEnvActivation{}, err
	}
	if err := input.Manifest.Verify(); err != nil {
		return CommittedProjectTypeEnvActivation{}, err
	}
	if input.Envelope.DeltaRef() != input.Delta.Ref() ||
		input.Envelope.DeltaDigest() != input.Delta.Digest() ||
		input.Basis.EnvelopeRef() != input.Envelope.Ref() ||
		input.Basis.EnvelopeDigest() != input.Envelope.Digest() ||
		input.Manifest.DeltaRef() != input.Delta.Ref() ||
		input.Manifest.DeltaDigest() != input.Delta.Digest() ||
		input.Manifest.EnvelopeRef() != input.Envelope.Ref() ||
		input.Manifest.EnvelopeDigest() != input.Envelope.Digest() ||
		input.Manifest.BasisRef() != input.Basis.Ref() ||
		input.Manifest.BasisDigest() != input.Basis.Digest() {
		return CommittedProjectTypeEnvActivation{}, fmt.Errorf(
			"committed activation members do not form one exact closure",
		)
	}
	if err := input.SuccessorHead.Verify(); err != nil {
		return CommittedProjectTypeEnvActivation{}, err
	}
	if input.SuccessorHead.Project() != input.Identity.Project() ||
		input.SuccessorHead.Ref() != input.Delta.Head() ||
		input.SuccessorHead.SelectedComposite() != input.Delta.Target().Composite() ||
		input.SuccessorHead.Revision() != input.Identity.SuccessorHeadRevision() {
		return CommittedProjectTypeEnvActivation{},
			fmt.Errorf("committed activation successor head mismatch")
	}
	materializationDigest, err := typedmemory.NewSHA256Digest(
		input.MaterializationDigest.String(),
	)
	if err != nil || materializationDigest != input.MaterializationDigest {
		return CommittedProjectTypeEnvActivation{},
			fmt.Errorf("committed activation materialization digest is required")
	}
	headDigest, err := digestRaw(input.SuccessorHead.CanonicalBytes())
	if err != nil {
		return CommittedProjectTypeEnvActivation{}, err
	}
	return CommittedProjectTypeEnvActivation{
		identity:              input.Identity,
		dag:                   input.ReferenceDAG,
		delta:                 input.Delta,
		envelope:              input.Envelope,
		basis:                 input.Basis,
		manifest:              input.Manifest,
		successorHead:         input.SuccessorHead,
		successorHeadDigest:   headDigest,
		materializationDigest: materializationDigest,
	}, nil
}

func (value CommittedProjectTypeEnvActivation) Identity() ProjectTypeEnvHeadSelectionTransactionIdentity {
	return value.identity
}

func (value CommittedProjectTypeEnvActivation) ReferenceDAG() ProjectTypeEnvHeadSelectionReferenceDAG {
	return value.dag
}

func (value CommittedProjectTypeEnvActivation) Delta() ProjectTypeEnvActivationDelta {
	return value.delta
}

func (value CommittedProjectTypeEnvActivation) Envelope() ProjectTypeEnvActivationAdmissionEnvelope {
	return value.envelope
}

func (value CommittedProjectTypeEnvActivation) Basis() ProjectTypeEnvActivationAdmissionBasis {
	return value.basis
}

func (value CommittedProjectTypeEnvActivation) Manifest() ProjectTypeEnvActivationMaterializationManifest {
	return value.manifest
}

func (value CommittedProjectTypeEnvActivation) EventRef() projecttypeenvselection.GraphEventRef {
	return value.manifest.EventRef()
}

func (value CommittedProjectTypeEnvActivation) CommitRef() projecttypeenvselection.GraphCommitRef {
	return value.manifest.CommitRef()
}

func (value CommittedProjectTypeEnvActivation) MaterializationDigest() typedmemory.SHA256Digest {
	return value.materializationDigest
}

func (value CommittedProjectTypeEnvActivation) SuccessorHead() projecttypeenvselection.ProjectTypeEnvHeadState {
	return value.successorHead
}

func (value CommittedProjectTypeEnvActivation) SuccessorHeadDigest() typedmemory.SHA256Digest {
	return value.successorHeadDigest
}

func (value CommittedProjectTypeEnvActivation) Verify() error {
	_, err := SealCommittedProjectTypeEnvActivation(
		CommittedProjectTypeEnvActivationInput{
			Identity:              value.identity,
			ReferenceDAG:          value.dag,
			Delta:                 value.delta,
			Envelope:              value.envelope,
			Basis:                 value.basis,
			Manifest:              value.manifest,
			SuccessorHead:         value.successorHead,
			MaterializationDigest: value.materializationDigest,
		},
	)
	return err
}
