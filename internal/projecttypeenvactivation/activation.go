// Package projecttypeenvactivation owns the pure canonical carrier algebra for
// one ProjectTypeEnv activation. It has no storage or authority-effect side
// effects; higher layers supply occurrence refs and storage graph coordinates.
package projecttypeenvactivation

import (
	"bytes"
	"fmt"
	"math"
	"regexp"
	"strings"

	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv"
	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	// Delta and AdmissionBasis change canonical shape because Genesis no
	// longer embeds a no-prior-head proof. Their v1 domains remain decode-only;
	// all constructors issue v2.
	deltaDomainV1  = "haft.project-typeenv.activation-delta.v1"
	deltaDomainV2  = "haft.project-typeenv.activation-delta.v2"
	deltaRefPrefix = "project-typeenv-activation-delta:"

	envelopeDomain    = "haft.project-typeenv.activation-admission-envelope.v1"
	envelopeRefPrefix = "project-typeenv-activation-envelope:"

	basisDomainV1  = "haft.project-typeenv.activation-admission-basis.v1"
	basisDomainV2  = "haft.project-typeenv.activation-admission-basis.v2"
	basisRefPrefix = "project-typeenv-activation-basis:"

	// Envelope and Manifest shapes are unchanged. Their existing domains stay
	// current; new instances acquire new identities through their v2
	// Delta/Basis member references.
	manifestDomain    = "haft.project-typeenv.activation-materialization-manifest.v1"
	manifestRefPrefix = "project-typeenv-activation-manifest:"

	transactionRefPrefix  = "project-typeenv-head-selection-transaction:"
	authorityUseRefPrefix = "project-typeenv-head-selection-authority-use:"
	workRecordRefPrefix   = "project-typeenv-head-cas-work-record:"
	graphKeyPrefix        = "project-typeenv-head-activation:"

	AdmissionKindSnapshotOnly = "snapshot_only"
	EventKind                 = "activate_type_env"
	AuthorityClass            = "manual_type_env_activation"
	MaterializationOrdinal    = uint32(0)

	maximumOrderedExtensions = 4096
)

type carrierEdition uint8

const (
	legacyCarrierV1 carrierEdition = iota + 1
	currentCarrierV2
)

func (edition carrierEdition) current() bool {
	return edition == currentCarrierV2
}

var stableHexPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

type DeltaRef struct {
	digest typedmemory.SHA256Digest
}

func ParseDeltaRef(raw string) (DeltaRef, error) {
	digest, err := parseDigestRef("activation delta", deltaRefPrefix, raw)
	if err != nil {
		return DeltaRef{}, err
	}
	return DeltaRef{digest: digest}, nil
}

func (ref DeltaRef) Digest() typedmemory.SHA256Digest {
	return ref.digest
}

func (ref DeltaRef) String() string {
	return deltaRefPrefix + ref.digest.String()
}

type Target struct {
	base              typedmemory.TypeEnvRef
	orderedExtensions []typedmemory.TypeEnvExtensionRef
	runtimeBasis      projecttypeenv.RuntimeEvaluationBasisRef
	composite         typedmemory.TypeEnvRef
	stage             projecttypeenvselection.ProjectTypeEnvStageRef
}

type TargetInput struct {
	Base              typedmemory.TypeEnvRef
	OrderedExtensions []typedmemory.TypeEnvExtensionRef
	RuntimeBasis      projecttypeenv.RuntimeEvaluationBasisRef
	Composite         typedmemory.TypeEnvRef
	Stage             projecttypeenvselection.ProjectTypeEnvStageRef
}

func NewTarget(input TargetInput) (Target, error) {
	base, err := typedmemory.ParseTypeEnvRef(input.Base.String())
	if err != nil || base != input.Base {
		return Target{}, fmt.Errorf("activation target base TypeEnv is required")
	}
	if len(input.OrderedExtensions) > maximumOrderedExtensions {
		return Target{}, fmt.Errorf(
			"activation target extension count exceeds %d",
			maximumOrderedExtensions,
		)
	}
	extensions := make([]typedmemory.TypeEnvExtensionRef, 0, len(input.OrderedExtensions))
	seen := make(map[string]struct{}, len(input.OrderedExtensions))
	for _, candidate := range input.OrderedExtensions {
		extension, parseErr := typedmemory.ParseTypeEnvExtensionRef(candidate.String())
		if parseErr != nil || extension != candidate {
			return Target{}, fmt.Errorf("activation target extension is invalid")
		}
		if _, exists := seen[extension.String()]; exists {
			return Target{}, fmt.Errorf("activation target extensions contain a duplicate")
		}
		seen[extension.String()] = struct{}{}
		extensions = append(extensions, extension)
	}
	runtimeBasis, err := projecttypeenv.ParseRuntimeEvaluationBasisRef(
		input.RuntimeBasis.String(),
	)
	if err != nil || runtimeBasis != input.RuntimeBasis {
		return Target{}, fmt.Errorf("activation target runtime basis is required")
	}
	composite, err := typedmemory.ParseTypeEnvRef(input.Composite.String())
	if err != nil || composite != input.Composite {
		return Target{}, fmt.Errorf("activation target composite TypeEnv is required")
	}
	stage, err := projecttypeenvselection.ParseProjectTypeEnvStageRef(input.Stage.String())
	if err != nil || stage != input.Stage {
		return Target{}, fmt.Errorf("activation target Stage is required")
	}
	return Target{
		base:              base,
		orderedExtensions: extensions,
		runtimeBasis:      runtimeBasis,
		composite:         composite,
		stage:             stage,
	}, nil
}

func (target Target) Base() typedmemory.TypeEnvRef {
	return target.base
}

func (target Target) OrderedExtensions() []typedmemory.TypeEnvExtensionRef {
	return append([]typedmemory.TypeEnvExtensionRef(nil), target.orderedExtensions...)
}

func (target Target) RuntimeBasis() projecttypeenv.RuntimeEvaluationBasisRef {
	return target.runtimeBasis
}

func (target Target) Composite() typedmemory.TypeEnvRef {
	return target.composite
}

func (target Target) Stage() projecttypeenvselection.ProjectTypeEnvStageRef {
	return target.stage
}

// activationPredecessor is the carrier-owned predecessor encoding. Current
// Genesis is a tag. Historical v1 Genesis retains its proof coordinate only so
// exact legacy bytes can be decoded and verified; no current constructor can
// produce that variant.
type activationPredecessor interface {
	activationPredecessorVariant()
	publicPredecessor() projecttypeenvselection.ProjectTypeEnvHeadSelectionPredecessor
}

type currentGenesisActivationPredecessor struct{}

func (currentGenesisActivationPredecessor) activationPredecessorVariant() {}

func (currentGenesisActivationPredecessor) publicPredecessor() projecttypeenvselection.ProjectTypeEnvHeadSelectionPredecessor {
	return projecttypeenvselection.NewGenesisStagePredecessor()
}

type legacyGenesisActivationPredecessor struct {
	proof projecttypeenvselection.NoPriorHeadProofRef
}

func (legacyGenesisActivationPredecessor) activationPredecessorVariant() {}

func (legacyGenesisActivationPredecessor) publicPredecessor() projecttypeenvselection.ProjectTypeEnvHeadSelectionPredecessor {
	return projecttypeenvselection.NewGenesisStagePredecessor()
}

type transitionActivationPredecessor struct {
	value projecttypeenvselection.TransitionStagePredecessor
}

func (transitionActivationPredecessor) activationPredecessorVariant() {}

func (predecessor transitionActivationPredecessor) publicPredecessor() projecttypeenvselection.ProjectTypeEnvHeadSelectionPredecessor {
	return predecessor.value
}

type Delta struct {
	ref                    DeltaRef
	digest                 typedmemory.SHA256Digest
	edition                carrierEdition
	transactionRef         string
	transactionDigest      typedmemory.SHA256Digest
	project                projectidentity.ProjectID
	head                   projecttypeenvselection.ProjectTypeEnvHeadRef
	requestRef             projecttypeenvselection.ProjectTypeEnvHeadSelectionRequestRef
	requestDigest          typedmemory.SHA256Digest
	contentDigest          authority.Digest
	authorityUseRef        string
	workRef                authority.WorkRef
	workRecordRef          string
	predecessor            activationPredecessor
	target                 Target
	expectedGraphRevision  typedmemory.GraphRevision
	committedGraphRevision typedmemory.GraphRevision
	successorHeadRevision  projecttypeenvselection.HeadRevision
	canonicalBytes         []byte
}

type DeltaInput struct {
	TransactionRef         string
	TransactionDigest      typedmemory.SHA256Digest
	Project                projectidentity.ProjectID
	Head                   projecttypeenvselection.ProjectTypeEnvHeadRef
	RequestRef             projecttypeenvselection.ProjectTypeEnvHeadSelectionRequestRef
	RequestDigest          typedmemory.SHA256Digest
	ContentDigest          authority.Digest
	AuthorityUseRef        string
	WorkRef                authority.WorkRef
	WorkRecordRef          string
	Predecessor            projecttypeenvselection.ProjectTypeEnvHeadSelectionPredecessor
	Target                 Target
	ExpectedGraphRevision  typedmemory.GraphRevision
	CommittedGraphRevision typedmemory.GraphRevision
	SuccessorHeadRevision  projecttypeenvselection.HeadRevision
}

func NewDelta(input DeltaInput) (Delta, error) {
	predecessor, err := newCurrentActivationPredecessor(
		input.Project,
		input.Predecessor,
	)
	if err != nil {
		return Delta{}, err
	}
	state, err := normalizeDeltaState(deltaState{
		edition:                currentCarrierV2,
		transactionRef:         input.TransactionRef,
		transactionDigest:      input.TransactionDigest,
		project:                input.Project,
		head:                   input.Head,
		requestRef:             input.RequestRef,
		requestDigest:          input.RequestDigest,
		contentDigest:          input.ContentDigest,
		authorityUseRef:        input.AuthorityUseRef,
		workRef:                input.WorkRef,
		workRecordRef:          input.WorkRecordRef,
		predecessor:            predecessor,
		target:                 input.Target,
		expectedGraphRevision:  input.ExpectedGraphRevision,
		committedGraphRevision: input.CommittedGraphRevision,
		successorHeadRevision:  input.SuccessorHeadRevision,
	})
	if err != nil {
		return Delta{}, err
	}
	canonical, err := encodeDeltaState(state)
	if err != nil {
		return Delta{}, err
	}
	return DecodeDelta(canonical)
}

func DecodeDelta(canonical []byte) (Delta, error) {
	reader, edition, domain, err := newEditionedCanonicalReader(
		canonical,
		deltaDomainV2,
		deltaDomainV1,
	)
	if err != nil {
		return Delta{}, err
	}
	state, err := decodeDeltaState(reader, edition)
	if err != nil {
		return Delta{}, err
	}
	if err := reader.requireEnd("activation delta"); err != nil {
		return Delta{}, err
	}
	normalized, err := normalizeDeltaState(state)
	if err != nil {
		return Delta{}, err
	}
	reencoded, err := encodeDeltaState(normalized)
	if err != nil {
		return Delta{}, err
	}
	if !bytes.Equal(reencoded, canonical) {
		return Delta{}, fmt.Errorf("activation delta is not canonical")
	}
	digest, err := canonicalDigest(domain, canonical)
	if err != nil {
		return Delta{}, err
	}
	return Delta{
		ref:                    DeltaRef{digest: digest},
		digest:                 digest,
		edition:                normalized.edition,
		transactionRef:         normalized.transactionRef,
		transactionDigest:      normalized.transactionDigest,
		project:                normalized.project,
		head:                   normalized.head,
		requestRef:             normalized.requestRef,
		requestDigest:          normalized.requestDigest,
		contentDigest:          normalized.contentDigest,
		authorityUseRef:        normalized.authorityUseRef,
		workRef:                normalized.workRef,
		workRecordRef:          normalized.workRecordRef,
		predecessor:            normalized.predecessor,
		target:                 normalized.target,
		expectedGraphRevision:  normalized.expectedGraphRevision,
		committedGraphRevision: normalized.committedGraphRevision,
		successorHeadRevision:  normalized.successorHeadRevision,
		canonicalBytes:         append([]byte(nil), canonical...),
	}, nil
}

func (delta Delta) Ref() DeltaRef {
	return delta.ref
}

func (delta Delta) Digest() typedmemory.SHA256Digest {
	return delta.digest
}

func (delta Delta) TransactionRef() string {
	return delta.transactionRef
}

func (delta Delta) TransactionDigest() typedmemory.SHA256Digest {
	return delta.transactionDigest
}

func (delta Delta) Project() projectidentity.ProjectID {
	return delta.project
}

func (delta Delta) Head() projecttypeenvselection.ProjectTypeEnvHeadRef {
	return delta.head
}

func (delta Delta) RequestRef() projecttypeenvselection.ProjectTypeEnvHeadSelectionRequestRef {
	return delta.requestRef
}

func (delta Delta) RequestDigest() typedmemory.SHA256Digest {
	return delta.requestDigest
}

func (delta Delta) ContentDigest() authority.Digest {
	return delta.contentDigest
}

func (delta Delta) AuthorityUseRef() string {
	return delta.authorityUseRef
}

func (delta Delta) WorkRef() authority.WorkRef {
	return delta.workRef
}

func (delta Delta) WorkRecordRef() string {
	return delta.workRecordRef
}

func (delta Delta) Predecessor() projecttypeenvselection.ProjectTypeEnvHeadSelectionPredecessor {
	if delta.predecessor == nil {
		return nil
	}
	return delta.predecessor.publicPredecessor()
}

func (delta Delta) Target() Target {
	value, _ := NewTarget(TargetInput{
		Base:              delta.target.Base(),
		OrderedExtensions: delta.target.OrderedExtensions(),
		RuntimeBasis:      delta.target.RuntimeBasis(),
		Composite:         delta.target.Composite(),
		Stage:             delta.target.Stage(),
	})
	return value
}

func (delta Delta) ExpectedGraphRevision() typedmemory.GraphRevision {
	return delta.expectedGraphRevision
}

func (delta Delta) CommittedGraphRevision() typedmemory.GraphRevision {
	return delta.committedGraphRevision
}

func (delta Delta) SuccessorHeadRevision() projecttypeenvselection.HeadRevision {
	return delta.successorHeadRevision
}

func (delta Delta) EventKind() string {
	return EventKind
}

func (delta Delta) AuthorityClass() string {
	return AuthorityClass
}

func (delta Delta) CanonicalBytes() []byte {
	return append([]byte(nil), delta.canonicalBytes...)
}

func (delta Delta) Verify() error {
	decoded, err := DecodeDelta(delta.canonicalBytes)
	if err != nil {
		return err
	}
	if decoded.ref != delta.ref || decoded.digest != delta.digest {
		return fmt.Errorf("activation delta differs from canonical bytes")
	}
	return nil
}

type deltaState struct {
	edition                carrierEdition
	transactionRef         string
	transactionDigest      typedmemory.SHA256Digest
	project                projectidentity.ProjectID
	head                   projecttypeenvselection.ProjectTypeEnvHeadRef
	requestRef             projecttypeenvselection.ProjectTypeEnvHeadSelectionRequestRef
	requestDigest          typedmemory.SHA256Digest
	contentDigest          authority.Digest
	authorityUseRef        string
	workRef                authority.WorkRef
	workRecordRef          string
	predecessor            activationPredecessor
	target                 Target
	expectedGraphRevision  typedmemory.GraphRevision
	committedGraphRevision typedmemory.GraphRevision
	successorHeadRevision  projecttypeenvselection.HeadRevision
}

func normalizeDeltaState(state deltaState) (deltaState, error) {
	if state.edition != legacyCarrierV1 &&
		state.edition != currentCarrierV2 {
		return deltaState{}, fmt.Errorf(
			"activation delta carrier edition is invalid",
		)
	}
	project, err := projectidentity.ParseProjectID(state.project.String())
	if err != nil || project != state.project {
		return deltaState{}, fmt.Errorf("activation project identity is required")
	}
	transactionDigest, err := typedmemory.NewSHA256Digest(state.transactionDigest.String())
	if err != nil ||
		transactionDigest != state.transactionDigest ||
		!exactDigestRef(state.transactionRef, transactionRefPrefix, transactionDigest) {
		return deltaState{}, fmt.Errorf("activation transaction ref/digest mismatch")
	}
	head, err := projecttypeenvselection.ParseProjectTypeEnvHeadRef(state.head.String())
	if err != nil || head != state.head || head.Project() != project {
		return deltaState{}, fmt.Errorf("activation head/project mismatch")
	}
	requestRef, err := projecttypeenvselection.ParseProjectTypeEnvHeadSelectionRequestRef(
		state.requestRef.String(),
	)
	if err != nil ||
		requestRef != state.requestRef ||
		requestRef.Digest() != state.requestDigest {
		return deltaState{}, fmt.Errorf("activation request ref/digest mismatch")
	}
	contentDigest, err := authority.NewDigest(state.contentDigest.String())
	if err != nil || contentDigest != state.contentDigest {
		return deltaState{}, fmt.Errorf("activation content digest is required")
	}
	if !validDigestRef(state.authorityUseRef, authorityUseRefPrefix) {
		return deltaState{}, fmt.Errorf("activation authority-use ref is required")
	}
	workRef, err := authority.NewWorkRef(state.workRef.String())
	if err != nil || workRef != state.workRef {
		return deltaState{}, fmt.Errorf("activation Work ref is required")
	}
	if !validDigestRef(state.workRecordRef, workRecordRefPrefix) {
		return deltaState{}, fmt.Errorf("activation Work-record ref is required")
	}
	predecessor, err := normalizeActivationPredecessor(
		project,
		state.edition,
		state.predecessor,
	)
	if err != nil {
		return deltaState{}, err
	}
	target, err := NewTarget(TargetInput{
		Base:              state.target.Base(),
		OrderedExtensions: state.target.OrderedExtensions(),
		RuntimeBasis:      state.target.RuntimeBasis(),
		Composite:         state.target.Composite(),
		Stage:             state.target.Stage(),
	})
	if err != nil {
		return deltaState{}, err
	}
	if predecessorValue, ok := predecessor.(transitionActivationPredecessor); ok &&
		predecessorValue.value.SelectedComposite() == target.Composite() {
		return deltaState{}, fmt.Errorf(
			"transition activation must change the selected TypeEnv",
		)
	}
	if state.expectedGraphRevision.Value() == math.MaxUint64 ||
		state.committedGraphRevision.Value() != state.expectedGraphRevision.Value()+1 {
		return deltaState{}, fmt.Errorf("activation GraphRevision pair is not contiguous")
	}
	successor, err := projecttypeenvselection.NewHeadRevision(
		state.successorHeadRevision.Value(),
	)
	if err != nil || successor != state.successorHeadRevision {
		return deltaState{}, fmt.Errorf("activation successor HeadRevision is required")
	}
	switch predecessorValue := predecessor.(type) {
	case currentGenesisActivationPredecessor,
		legacyGenesisActivationPredecessor:
		if successor.Value() != 1 {
			return deltaState{}, fmt.Errorf(
				"genesis activation successor HeadRevision must be one",
			)
		}
	case transitionActivationPredecessor:
		if predecessorValue.value.HeadRevision().Value() == math.MaxUint64 ||
			successor.Value() != predecessorValue.value.HeadRevision().Value()+1 {
			return deltaState{}, fmt.Errorf(
				"transition activation successor HeadRevision is not contiguous",
			)
		}
	}
	return deltaState{
		edition:                state.edition,
		transactionRef:         state.transactionRef,
		transactionDigest:      transactionDigest,
		project:                project,
		head:                   head,
		requestRef:             requestRef,
		requestDigest:          state.requestDigest,
		contentDigest:          contentDigest,
		authorityUseRef:        state.authorityUseRef,
		workRef:                workRef,
		workRecordRef:          state.workRecordRef,
		predecessor:            predecessor,
		target:                 target,
		expectedGraphRevision:  typedmemory.NewGraphRevision(state.expectedGraphRevision.Value()),
		committedGraphRevision: typedmemory.NewGraphRevision(state.committedGraphRevision.Value()),
		successorHeadRevision:  successor,
	}, nil
}

func encodeDeltaState(state deltaState) ([]byte, error) {
	domain := deltaDomainForEdition(state.edition)
	writer := newCanonicalWriter(domain)
	writer.writeString(state.transactionRef)
	writer.writeString(state.transactionDigest.String())
	writer.writeString(state.project.String())
	writer.writeString(state.head.String())
	writer.writeString(state.requestRef.String())
	writer.writeString(state.requestDigest.String())
	writer.writeString(state.contentDigest.String())
	writer.writeString(state.authorityUseRef)
	writer.writeString(state.workRef.String())
	writer.writeString(state.workRecordRef)
	encodeActivationPredecessor(&writer, state.predecessor)
	if err := encodeTarget(&writer, state.target); err != nil {
		return nil, err
	}
	writer.writeUint64(state.expectedGraphRevision.Value())
	writer.writeUint64(state.committedGraphRevision.Value())
	writer.writeUint64(state.successorHeadRevision.Value())
	writer.writeString(EventKind)
	writer.writeString(AuthorityClass)
	return writer.bytes(), nil
}

func decodeDeltaState(
	reader *canonicalReader,
	edition carrierEdition,
) (deltaState, error) {
	transactionRef, err := reader.readString("activation transaction ref")
	if err != nil {
		return deltaState{}, err
	}
	transactionDigestText, err := reader.readString("activation transaction digest")
	if err != nil {
		return deltaState{}, err
	}
	transactionDigest, err := typedmemory.NewSHA256Digest(transactionDigestText)
	if err != nil {
		return deltaState{}, err
	}
	projectText, err := reader.readString("activation project")
	if err != nil {
		return deltaState{}, err
	}
	project, err := projectidentity.ParseProjectID(projectText)
	if err != nil {
		return deltaState{}, err
	}
	headText, err := reader.readString("activation head")
	if err != nil {
		return deltaState{}, err
	}
	head, err := projecttypeenvselection.ParseProjectTypeEnvHeadRef(headText)
	if err != nil {
		return deltaState{}, err
	}
	requestText, err := reader.readString("activation request ref")
	if err != nil {
		return deltaState{}, err
	}
	requestRef, err := projecttypeenvselection.ParseProjectTypeEnvHeadSelectionRequestRef(
		requestText,
	)
	if err != nil {
		return deltaState{}, err
	}
	requestDigestText, err := reader.readString("activation request digest")
	if err != nil {
		return deltaState{}, err
	}
	requestDigest, err := typedmemory.NewSHA256Digest(requestDigestText)
	if err != nil {
		return deltaState{}, err
	}
	contentDigestText, err := reader.readString("activation content digest")
	if err != nil {
		return deltaState{}, err
	}
	contentDigest, err := authority.NewDigest(contentDigestText)
	if err != nil {
		return deltaState{}, err
	}
	authorityUseRef, err := reader.readString("activation authority-use ref")
	if err != nil {
		return deltaState{}, err
	}
	workText, err := reader.readString("activation Work ref")
	if err != nil {
		return deltaState{}, err
	}
	workRef, err := authority.NewWorkRef(workText)
	if err != nil {
		return deltaState{}, err
	}
	workRecordRef, err := reader.readString("activation Work-record ref")
	if err != nil {
		return deltaState{}, err
	}
	predecessor, err := decodeActivationPredecessor(
		reader,
		project,
		edition,
	)
	if err != nil {
		return deltaState{}, err
	}
	target, err := decodeTarget(reader)
	if err != nil {
		return deltaState{}, err
	}
	expectedValue, err := reader.readUint64("activation expected GraphRevision")
	if err != nil {
		return deltaState{}, err
	}
	committedValue, err := reader.readUint64("activation committed GraphRevision")
	if err != nil {
		return deltaState{}, err
	}
	headRevisionValue, err := reader.readUint64("activation successor HeadRevision")
	if err != nil {
		return deltaState{}, err
	}
	headRevision, err := projecttypeenvselection.NewHeadRevision(headRevisionValue)
	if err != nil {
		return deltaState{}, err
	}
	eventKind, err := reader.readString("activation event kind")
	if err != nil || eventKind != EventKind {
		return deltaState{}, fmt.Errorf("activation event kind is invalid")
	}
	authorityClass, err := reader.readString("activation authority class")
	if err != nil || authorityClass != AuthorityClass {
		return deltaState{}, fmt.Errorf("activation authority class is invalid")
	}
	return deltaState{
		edition:                edition,
		transactionRef:         transactionRef,
		transactionDigest:      transactionDigest,
		project:                project,
		head:                   head,
		requestRef:             requestRef,
		requestDigest:          requestDigest,
		contentDigest:          contentDigest,
		authorityUseRef:        authorityUseRef,
		workRef:                workRef,
		workRecordRef:          workRecordRef,
		predecessor:            predecessor,
		target:                 target,
		expectedGraphRevision:  typedmemory.NewGraphRevision(expectedValue),
		committedGraphRevision: typedmemory.NewGraphRevision(committedValue),
		successorHeadRevision:  headRevision,
	}, nil
}

type EnvelopeRef struct {
	digest typedmemory.SHA256Digest
}

func ParseEnvelopeRef(raw string) (EnvelopeRef, error) {
	digest, err := parseDigestRef("activation admission envelope", envelopeRefPrefix, raw)
	if err != nil {
		return EnvelopeRef{}, err
	}
	return EnvelopeRef{digest: digest}, nil
}

func (ref EnvelopeRef) Digest() typedmemory.SHA256Digest {
	return ref.digest
}

func (ref EnvelopeRef) String() string {
	return envelopeRefPrefix + ref.digest.String()
}

type AdmissionEnvelope struct {
	ref            EnvelopeRef
	digest         typedmemory.SHA256Digest
	deltaRef       DeltaRef
	deltaDigest    typedmemory.SHA256Digest
	requestRef     projecttypeenvselection.ProjectTypeEnvHeadSelectionRequestRef
	requestDigest  typedmemory.SHA256Digest
	target         typedmemory.TypeEnvRef
	stage          projecttypeenvselection.ProjectTypeEnvStageRef
	graphKey       string
	canonicalBytes []byte
}

func NewAdmissionEnvelope(delta Delta, graphKey string) (AdmissionEnvelope, error) {
	if err := delta.Verify(); err != nil {
		return AdmissionEnvelope{}, err
	}
	if !delta.edition.current() {
		return AdmissionEnvelope{}, fmt.Errorf(
			"legacy activation delta is read-only and cannot issue a current envelope",
		)
	}
	if !validGraphKey(graphKey) {
		return AdmissionEnvelope{}, fmt.Errorf("activation graph idempotency key is invalid")
	}
	writer := newCanonicalWriter(envelopeDomain)
	writer.writeString(AdmissionKindSnapshotOnly)
	writer.writeString(delta.Ref().String())
	writer.writeString(delta.Digest().String())
	writer.writeString(delta.RequestRef().String())
	writer.writeString(delta.RequestDigest().String())
	writer.writeString(delta.Target().Composite().String())
	writer.writeString(delta.Target().Stage().String())
	writer.writeString(graphKey)
	return DecodeAdmissionEnvelope(writer.bytes())
}

func DecodeAdmissionEnvelope(canonical []byte) (AdmissionEnvelope, error) {
	reader, err := newCanonicalReader(canonical, envelopeDomain)
	if err != nil {
		return AdmissionEnvelope{}, err
	}
	kind, err := reader.readString("activation envelope kind")
	if err != nil || kind != AdmissionKindSnapshotOnly {
		return AdmissionEnvelope{}, fmt.Errorf("activation envelope kind is invalid")
	}
	deltaText, err := reader.readString("activation envelope delta ref")
	if err != nil {
		return AdmissionEnvelope{}, err
	}
	deltaRef, err := ParseDeltaRef(deltaText)
	if err != nil {
		return AdmissionEnvelope{}, err
	}
	deltaDigestText, err := reader.readString("activation envelope delta digest")
	if err != nil {
		return AdmissionEnvelope{}, err
	}
	deltaDigest, err := typedmemory.NewSHA256Digest(deltaDigestText)
	if err != nil || deltaRef.Digest() != deltaDigest {
		return AdmissionEnvelope{}, fmt.Errorf("activation envelope delta ref/digest mismatch")
	}
	requestText, err := reader.readString("activation envelope request ref")
	if err != nil {
		return AdmissionEnvelope{}, err
	}
	requestRef, err := projecttypeenvselection.ParseProjectTypeEnvHeadSelectionRequestRef(
		requestText,
	)
	if err != nil {
		return AdmissionEnvelope{}, err
	}
	requestDigestText, err := reader.readString("activation envelope request digest")
	if err != nil {
		return AdmissionEnvelope{}, err
	}
	requestDigest, err := typedmemory.NewSHA256Digest(requestDigestText)
	if err != nil || requestRef.Digest() != requestDigest {
		return AdmissionEnvelope{}, fmt.Errorf("activation envelope request ref/digest mismatch")
	}
	targetText, err := reader.readString("activation envelope target")
	if err != nil {
		return AdmissionEnvelope{}, err
	}
	target, err := typedmemory.ParseTypeEnvRef(targetText)
	if err != nil {
		return AdmissionEnvelope{}, err
	}
	stageText, err := reader.readString("activation envelope Stage")
	if err != nil {
		return AdmissionEnvelope{}, err
	}
	stage, err := projecttypeenvselection.ParseProjectTypeEnvStageRef(stageText)
	if err != nil {
		return AdmissionEnvelope{}, err
	}
	graphKey, err := reader.readString("activation envelope graph key")
	if err != nil {
		return AdmissionEnvelope{}, err
	}
	if !validGraphKey(graphKey) {
		return AdmissionEnvelope{}, fmt.Errorf("activation envelope graph key is invalid")
	}
	if err := reader.requireEnd("activation admission envelope"); err != nil {
		return AdmissionEnvelope{}, err
	}
	digest, err := canonicalDigest(envelopeDomain, canonical)
	if err != nil {
		return AdmissionEnvelope{}, err
	}
	return AdmissionEnvelope{
		ref:            EnvelopeRef{digest: digest},
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

func (value AdmissionEnvelope) Ref() EnvelopeRef {
	return value.ref
}

func (value AdmissionEnvelope) Digest() typedmemory.SHA256Digest {
	return value.digest
}

func (value AdmissionEnvelope) DeltaRef() DeltaRef {
	return value.deltaRef
}

func (value AdmissionEnvelope) DeltaDigest() typedmemory.SHA256Digest {
	return value.deltaDigest
}

func (value AdmissionEnvelope) RequestRef() projecttypeenvselection.ProjectTypeEnvHeadSelectionRequestRef {
	return value.requestRef
}

func (value AdmissionEnvelope) RequestDigest() typedmemory.SHA256Digest {
	return value.requestDigest
}

func (value AdmissionEnvelope) TargetComposite() typedmemory.TypeEnvRef {
	return value.target
}

func (value AdmissionEnvelope) Stage() projecttypeenvselection.ProjectTypeEnvStageRef {
	return value.stage
}

func (value AdmissionEnvelope) GraphIdempotencyKey() string {
	return value.graphKey
}

func (value AdmissionEnvelope) AdmissionKind() string {
	return AdmissionKindSnapshotOnly
}

func (value AdmissionEnvelope) CanonicalBytes() []byte {
	return append([]byte(nil), value.canonicalBytes...)
}

func (value AdmissionEnvelope) Verify() error {
	decoded, err := DecodeAdmissionEnvelope(value.canonicalBytes)
	if err != nil {
		return err
	}
	if decoded.ref != value.ref || decoded.digest != value.digest {
		return fmt.Errorf("activation envelope differs from canonical bytes")
	}
	return nil
}

type BasisRef struct {
	digest typedmemory.SHA256Digest
}

func ParseBasisRef(raw string) (BasisRef, error) {
	digest, err := parseDigestRef("activation admission basis", basisRefPrefix, raw)
	if err != nil {
		return BasisRef{}, err
	}
	return BasisRef{digest: digest}, nil
}

func (ref BasisRef) Digest() typedmemory.SHA256Digest {
	return ref.digest
}

func (ref BasisRef) String() string {
	return basisRefPrefix + ref.digest.String()
}

type AdmissionBasis struct {
	ref                   BasisRef
	digest                typedmemory.SHA256Digest
	edition               carrierEdition
	envelopeRef           EnvelopeRef
	envelopeDigest        typedmemory.SHA256Digest
	project               projectidentity.ProjectID
	predecessor           activationPredecessor
	target                typedmemory.TypeEnvRef
	stage                 projecttypeenvselection.ProjectTypeEnvStageRef
	expectedGraphRevision typedmemory.GraphRevision
	canonicalBytes        []byte
}

func NewAdmissionBasis(delta Delta, envelope AdmissionEnvelope) (AdmissionBasis, error) {
	if err := VerifyEnvelopeForDelta(delta, envelope); err != nil {
		return AdmissionBasis{}, err
	}
	if !delta.edition.current() {
		return AdmissionBasis{}, fmt.Errorf(
			"legacy activation delta is read-only and cannot issue a current admission basis",
		)
	}
	domain := basisDomainForEdition(currentCarrierV2)
	writer := newCanonicalWriter(domain)
	writer.writeString(AdmissionKindSnapshotOnly)
	writer.writeString(envelope.Ref().String())
	writer.writeString(envelope.Digest().String())
	writer.writeString(delta.Project().String())
	encodeActivationPredecessor(&writer, delta.predecessor)
	writer.writeString(delta.Target().Composite().String())
	writer.writeString(delta.Target().Stage().String())
	writer.writeUint64(delta.ExpectedGraphRevision().Value())
	return DecodeAdmissionBasis(writer.bytes())
}

func DecodeAdmissionBasis(canonical []byte) (AdmissionBasis, error) {
	reader, edition, domain, err := newEditionedCanonicalReader(
		canonical,
		basisDomainV2,
		basisDomainV1,
	)
	if err != nil {
		return AdmissionBasis{}, err
	}
	kind, err := reader.readString("activation basis kind")
	if err != nil || kind != AdmissionKindSnapshotOnly {
		return AdmissionBasis{}, fmt.Errorf("activation basis kind is invalid")
	}
	envelopeText, err := reader.readString("activation basis envelope ref")
	if err != nil {
		return AdmissionBasis{}, err
	}
	envelopeRef, err := ParseEnvelopeRef(envelopeText)
	if err != nil {
		return AdmissionBasis{}, err
	}
	envelopeDigestText, err := reader.readString("activation basis envelope digest")
	if err != nil {
		return AdmissionBasis{}, err
	}
	envelopeDigest, err := typedmemory.NewSHA256Digest(envelopeDigestText)
	if err != nil || envelopeRef.Digest() != envelopeDigest {
		return AdmissionBasis{}, fmt.Errorf("activation basis envelope ref/digest mismatch")
	}
	projectText, err := reader.readString("activation basis project")
	if err != nil {
		return AdmissionBasis{}, err
	}
	project, err := projectidentity.ParseProjectID(projectText)
	if err != nil {
		return AdmissionBasis{}, err
	}
	predecessor, err := decodeActivationPredecessor(
		reader,
		project,
		edition,
	)
	if err != nil {
		return AdmissionBasis{}, err
	}
	targetText, err := reader.readString("activation basis target")
	if err != nil {
		return AdmissionBasis{}, err
	}
	target, err := typedmemory.ParseTypeEnvRef(targetText)
	if err != nil {
		return AdmissionBasis{}, err
	}
	stageText, err := reader.readString("activation basis Stage")
	if err != nil {
		return AdmissionBasis{}, err
	}
	stage, err := projecttypeenvselection.ParseProjectTypeEnvStageRef(stageText)
	if err != nil {
		return AdmissionBasis{}, err
	}
	revisionValue, err := reader.readUint64("activation basis expected GraphRevision")
	if err != nil {
		return AdmissionBasis{}, err
	}
	if err := reader.requireEnd("activation admission basis"); err != nil {
		return AdmissionBasis{}, err
	}
	digest, err := canonicalDigest(domain, canonical)
	if err != nil {
		return AdmissionBasis{}, err
	}
	return AdmissionBasis{
		ref:                   BasisRef{digest: digest},
		digest:                digest,
		edition:               edition,
		envelopeRef:           envelopeRef,
		envelopeDigest:        envelopeDigest,
		project:               project,
		predecessor:           predecessor,
		target:                target,
		stage:                 stage,
		expectedGraphRevision: typedmemory.NewGraphRevision(revisionValue),
		canonicalBytes:        append([]byte(nil), canonical...),
	}, nil
}

func (value AdmissionBasis) Ref() BasisRef {
	return value.ref
}

func (value AdmissionBasis) Digest() typedmemory.SHA256Digest {
	return value.digest
}

func (value AdmissionBasis) EnvelopeRef() EnvelopeRef {
	return value.envelopeRef
}

func (value AdmissionBasis) EnvelopeDigest() typedmemory.SHA256Digest {
	return value.envelopeDigest
}

func (value AdmissionBasis) Project() projectidentity.ProjectID {
	return value.project
}

func (value AdmissionBasis) Predecessor() projecttypeenvselection.ProjectTypeEnvHeadSelectionPredecessor {
	if value.predecessor == nil {
		return nil
	}
	return value.predecessor.publicPredecessor()
}

func (value AdmissionBasis) TargetComposite() typedmemory.TypeEnvRef {
	return value.target
}

func (value AdmissionBasis) Stage() projecttypeenvselection.ProjectTypeEnvStageRef {
	return value.stage
}

func (value AdmissionBasis) ExpectedGraphRevision() typedmemory.GraphRevision {
	return value.expectedGraphRevision
}

func (value AdmissionBasis) AdmissionKind() string {
	return AdmissionKindSnapshotOnly
}

func (value AdmissionBasis) CanonicalBytes() []byte {
	return append([]byte(nil), value.canonicalBytes...)
}

func (value AdmissionBasis) Verify() error {
	decoded, err := DecodeAdmissionBasis(value.canonicalBytes)
	if err != nil {
		return err
	}
	if decoded.ref != value.ref || decoded.digest != value.digest {
		return fmt.Errorf("activation basis differs from canonical bytes")
	}
	return nil
}

type ManifestRef struct {
	digest typedmemory.SHA256Digest
}

func ParseManifestRef(raw string) (ManifestRef, error) {
	digest, err := parseDigestRef(
		"activation materialization manifest",
		manifestRefPrefix,
		raw,
	)
	if err != nil {
		return ManifestRef{}, err
	}
	return ManifestRef{digest: digest}, nil
}

func (ref ManifestRef) Digest() typedmemory.SHA256Digest {
	return ref.digest
}

func (ref ManifestRef) String() string {
	return manifestRefPrefix + ref.digest.String()
}

type MaterializationManifest struct {
	ref            ManifestRef
	digest         typedmemory.SHA256Digest
	deltaRef       DeltaRef
	deltaDigest    typedmemory.SHA256Digest
	envelopeRef    EnvelopeRef
	envelopeDigest typedmemory.SHA256Digest
	basisRef       BasisRef
	basisDigest    typedmemory.SHA256Digest
	event          projecttypeenvselection.GraphEventRef
	commit         projecttypeenvselection.GraphCommitRef
	canonicalBytes []byte
}

func NewMaterializationManifest(
	delta Delta,
	envelope AdmissionEnvelope,
	basis AdmissionBasis,
	event projecttypeenvselection.GraphEventRef,
	commit projecttypeenvselection.GraphCommitRef,
) (MaterializationManifest, error) {
	if err := VerifyAdmission(delta, envelope, basis); err != nil {
		return MaterializationManifest{}, err
	}
	if !delta.edition.current() || !basis.edition.current() {
		return MaterializationManifest{}, fmt.Errorf(
			"legacy activation admission is read-only and cannot issue a current manifest",
		)
	}
	canonicalEvent, err := projecttypeenvselection.ParseGraphEventRef(event.String())
	if err != nil || canonicalEvent != event {
		return MaterializationManifest{}, fmt.Errorf("activation graph event ref is required")
	}
	canonicalCommit, err := projecttypeenvselection.ParseGraphCommitRef(commit.String())
	if err != nil || canonicalCommit != commit {
		return MaterializationManifest{}, fmt.Errorf("activation graph commit ref is required")
	}
	writer := newCanonicalWriter(manifestDomain)
	writer.writeString(delta.Ref().String())
	writer.writeString(delta.Digest().String())
	writer.writeString(envelope.Ref().String())
	writer.writeString(envelope.Digest().String())
	writer.writeString(basis.Ref().String())
	writer.writeString(basis.Digest().String())
	writer.writeString(event.String())
	writer.writeString(commit.String())
	writer.writeUint32(MaterializationOrdinal)
	writer.writeUint32(1)
	writer.writeUint32(1)
	writer.writeString(delta.Digest().String())
	return DecodeMaterializationManifest(writer.bytes())
}

func DecodeMaterializationManifest(
	canonical []byte,
) (MaterializationManifest, error) {
	reader, err := newCanonicalReader(canonical, manifestDomain)
	if err != nil {
		return MaterializationManifest{}, err
	}
	deltaText, err := reader.readString("activation manifest delta ref")
	if err != nil {
		return MaterializationManifest{}, err
	}
	deltaRef, err := ParseDeltaRef(deltaText)
	if err != nil {
		return MaterializationManifest{}, err
	}
	deltaDigestText, err := reader.readString("activation manifest delta digest")
	if err != nil {
		return MaterializationManifest{}, err
	}
	deltaDigest, err := typedmemory.NewSHA256Digest(deltaDigestText)
	if err != nil || deltaRef.Digest() != deltaDigest {
		return MaterializationManifest{}, fmt.Errorf("activation manifest delta ref/digest mismatch")
	}
	envelopeText, err := reader.readString("activation manifest envelope ref")
	if err != nil {
		return MaterializationManifest{}, err
	}
	envelopeRef, err := ParseEnvelopeRef(envelopeText)
	if err != nil {
		return MaterializationManifest{}, err
	}
	envelopeDigestText, err := reader.readString("activation manifest envelope digest")
	if err != nil {
		return MaterializationManifest{}, err
	}
	envelopeDigest, err := typedmemory.NewSHA256Digest(envelopeDigestText)
	if err != nil || envelopeRef.Digest() != envelopeDigest {
		return MaterializationManifest{}, fmt.Errorf(
			"activation manifest envelope ref/digest mismatch",
		)
	}
	basisText, err := reader.readString("activation manifest basis ref")
	if err != nil {
		return MaterializationManifest{}, err
	}
	basisRef, err := ParseBasisRef(basisText)
	if err != nil {
		return MaterializationManifest{}, err
	}
	basisDigestText, err := reader.readString("activation manifest basis digest")
	if err != nil {
		return MaterializationManifest{}, err
	}
	basisDigest, err := typedmemory.NewSHA256Digest(basisDigestText)
	if err != nil || basisRef.Digest() != basisDigest {
		return MaterializationManifest{}, fmt.Errorf(
			"activation manifest basis ref/digest mismatch",
		)
	}
	eventText, err := reader.readString("activation manifest event ref")
	if err != nil {
		return MaterializationManifest{}, err
	}
	event, err := projecttypeenvselection.ParseGraphEventRef(eventText)
	if err != nil {
		return MaterializationManifest{}, err
	}
	commitText, err := reader.readString("activation manifest commit ref")
	if err != nil {
		return MaterializationManifest{}, err
	}
	commit, err := projecttypeenvselection.ParseGraphCommitRef(commitText)
	if err != nil {
		return MaterializationManifest{}, err
	}
	ordinal, err := reader.readUint32("activation manifest ordinal")
	if err != nil || ordinal != MaterializationOrdinal {
		return MaterializationManifest{}, fmt.Errorf("activation manifest ordinal is invalid")
	}
	activationCount, err := reader.readUint32("activation manifest activation count")
	if err != nil || activationCount != 1 {
		return MaterializationManifest{}, fmt.Errorf(
			"activation manifest must contain exactly one activation",
		)
	}
	topLevelCount, err := reader.readUint32("activation manifest top-level count")
	if err != nil || topLevelCount != 1 {
		return MaterializationManifest{}, fmt.Errorf(
			"activation manifest top-level count must be one",
		)
	}
	rowDigest, err := reader.readString("activation manifest row digest")
	if err != nil || rowDigest != deltaDigest.String() {
		return MaterializationManifest{}, fmt.Errorf("activation manifest row digest mismatch")
	}
	if err := reader.requireEnd("activation materialization manifest"); err != nil {
		return MaterializationManifest{}, err
	}
	digest, err := canonicalDigest(manifestDomain, canonical)
	if err != nil {
		return MaterializationManifest{}, err
	}
	return MaterializationManifest{
		ref:            ManifestRef{digest: digest},
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

func (value MaterializationManifest) Ref() ManifestRef {
	return value.ref
}

func (value MaterializationManifest) Digest() typedmemory.SHA256Digest {
	return value.digest
}

func (value MaterializationManifest) DeltaRef() DeltaRef {
	return value.deltaRef
}

func (value MaterializationManifest) DeltaDigest() typedmemory.SHA256Digest {
	return value.deltaDigest
}

func (value MaterializationManifest) EnvelopeRef() EnvelopeRef {
	return value.envelopeRef
}

func (value MaterializationManifest) EnvelopeDigest() typedmemory.SHA256Digest {
	return value.envelopeDigest
}

func (value MaterializationManifest) BasisRef() BasisRef {
	return value.basisRef
}

func (value MaterializationManifest) BasisDigest() typedmemory.SHA256Digest {
	return value.basisDigest
}

func (value MaterializationManifest) EventRef() projecttypeenvselection.GraphEventRef {
	return value.event
}

func (value MaterializationManifest) CommitRef() projecttypeenvselection.GraphCommitRef {
	return value.commit
}

func (value MaterializationManifest) ActivationCount() uint32 {
	return 1
}

func (value MaterializationManifest) TopLevelChangeCount() uint32 {
	return 1
}

func (value MaterializationManifest) CanonicalBytes() []byte {
	return append([]byte(nil), value.canonicalBytes...)
}

func (value MaterializationManifest) Verify() error {
	decoded, err := DecodeMaterializationManifest(value.canonicalBytes)
	if err != nil {
		return err
	}
	if decoded.ref != value.ref || decoded.digest != value.digest {
		return fmt.Errorf("activation manifest differs from canonical bytes")
	}
	return nil
}

func VerifyEnvelopeForDelta(delta Delta, envelope AdmissionEnvelope) error {
	if err := delta.Verify(); err != nil {
		return err
	}
	if err := envelope.Verify(); err != nil {
		return err
	}
	matches := envelope.DeltaRef() == delta.Ref() &&
		envelope.DeltaDigest() == delta.Digest() &&
		envelope.RequestRef() == delta.RequestRef() &&
		envelope.RequestDigest() == delta.RequestDigest() &&
		envelope.TargetComposite() == delta.Target().Composite() &&
		envelope.Stage() == delta.Target().Stage()
	if !matches {
		return fmt.Errorf("activation envelope differs from delta")
	}
	return nil
}

func VerifyAdmission(
	delta Delta,
	envelope AdmissionEnvelope,
	basis AdmissionBasis,
) error {
	if err := VerifyEnvelopeForDelta(delta, envelope); err != nil {
		return err
	}
	if err := basis.Verify(); err != nil {
		return err
	}
	matches := basis.EnvelopeRef() == envelope.Ref() &&
		basis.EnvelopeDigest() == envelope.Digest() &&
		basis.edition == delta.edition &&
		basis.Project() == delta.Project() &&
		sameActivationPredecessor(basis.predecessor, delta.predecessor) &&
		basis.TargetComposite() == delta.Target().Composite() &&
		basis.Stage() == delta.Target().Stage() &&
		basis.ExpectedGraphRevision() == delta.ExpectedGraphRevision()
	if !matches {
		return fmt.Errorf("activation basis differs from delta and envelope")
	}
	return nil
}

func VerifyClosure(
	delta Delta,
	envelope AdmissionEnvelope,
	basis AdmissionBasis,
	manifest MaterializationManifest,
) error {
	if err := VerifyAdmission(delta, envelope, basis); err != nil {
		return err
	}
	if err := manifest.Verify(); err != nil {
		return err
	}
	matches := manifest.DeltaRef() == delta.Ref() &&
		manifest.DeltaDigest() == delta.Digest() &&
		manifest.EnvelopeRef() == envelope.Ref() &&
		manifest.EnvelopeDigest() == envelope.Digest() &&
		manifest.BasisRef() == basis.Ref() &&
		manifest.BasisDigest() == basis.Digest()
	if !matches {
		return fmt.Errorf("activation materialization manifest differs from admission")
	}
	return nil
}

func deltaDomainForEdition(edition carrierEdition) string {
	switch edition {
	case legacyCarrierV1:
		return deltaDomainV1
	case currentCarrierV2:
		return deltaDomainV2
	default:
		return ""
	}
}

func basisDomainForEdition(edition carrierEdition) string {
	switch edition {
	case legacyCarrierV1:
		return basisDomainV1
	case currentCarrierV2:
		return basisDomainV2
	default:
		return ""
	}
}

func newEditionedCanonicalReader(
	canonical []byte,
	currentDomain string,
	legacyDomain string,
) (*canonicalReader, carrierEdition, string, error) {
	current, currentErr := newCanonicalReader(canonical, currentDomain)
	if currentErr == nil {
		return current, currentCarrierV2, currentDomain, nil
	}
	legacy, legacyErr := newCanonicalReader(canonical, legacyDomain)
	if legacyErr == nil {
		return legacy, legacyCarrierV1, legacyDomain, nil
	}
	return nil, 0, "", fmt.Errorf(
		"canonical carrier domain is neither current nor supported legacy: %w",
		currentErr,
	)
}

func canonicalDigest(domain string, canonical []byte) (typedmemory.SHA256Digest, error) {
	text, err := digestCanonical(domain, canonical)
	if err != nil {
		return typedmemory.SHA256Digest{}, err
	}
	return typedmemory.NewSHA256Digest(text)
}

func parseDigestRef(
	name string,
	prefix string,
	raw string,
) (typedmemory.SHA256Digest, error) {
	value, found := strings.CutPrefix(raw, prefix)
	if !found {
		return typedmemory.SHA256Digest{}, fmt.Errorf("%s ref is malformed", name)
	}
	digest, err := typedmemory.NewSHA256Digest(value)
	if err != nil {
		return typedmemory.SHA256Digest{}, fmt.Errorf("%s ref is malformed: %w", name, err)
	}
	return digest, nil
}

func exactDigestRef(
	raw string,
	prefix string,
	digest typedmemory.SHA256Digest,
) bool {
	parsed, err := parseDigestRef("stable", prefix, raw)
	return err == nil && parsed == digest
}

func validDigestRef(raw string, prefix string) bool {
	_, err := parseDigestRef("stable", prefix, raw)
	return err == nil
}

func validGraphKey(raw string) bool {
	value, found := strings.CutPrefix(raw, graphKeyPrefix)
	return found && stableHexPattern.MatchString(value)
}

func newCurrentActivationPredecessor(
	project projectidentity.ProjectID,
	predecessor projecttypeenvselection.ProjectTypeEnvHeadSelectionPredecessor,
) (activationPredecessor, error) {
	switch value := predecessor.(type) {
	case projecttypeenvselection.GenesisStagePredecessor:
		return currentGenesisActivationPredecessor{}, nil
	case projecttypeenvselection.TransitionStagePredecessor:
		normalized, err := projecttypeenvselection.NewTransitionStagePredecessor(
			projecttypeenvselection.TransitionStagePredecessorInput{
				Project:           project,
				Head:              value.Head(),
				HeadRevision:      value.HeadRevision(),
				SelectedComposite: value.SelectedComposite(),
			},
		)
		if err != nil {
			return nil, err
		}
		return transitionActivationPredecessor{value: normalized}, nil
	default:
		return nil, fmt.Errorf("head-selection predecessor variant is required")
	}
}

func normalizeActivationPredecessor(
	project projectidentity.ProjectID,
	edition carrierEdition,
	predecessor activationPredecessor,
) (activationPredecessor, error) {
	switch value := predecessor.(type) {
	case currentGenesisActivationPredecessor:
		if edition != currentCarrierV2 {
			return nil, fmt.Errorf(
				"legacy activation Genesis predecessor requires its historical proof",
			)
		}
		return currentGenesisActivationPredecessor{}, nil
	case legacyGenesisActivationPredecessor:
		if edition != legacyCarrierV1 {
			return nil, fmt.Errorf(
				"current activation Genesis predecessor cannot carry a legacy proof",
			)
		}
		proof, err := projecttypeenvselection.ParseNoPriorHeadProofRef(
			value.proof.String(),
		)
		if err != nil || proof != value.proof {
			return nil, fmt.Errorf(
				"legacy activation Genesis predecessor proof is invalid",
			)
		}
		return legacyGenesisActivationPredecessor{proof: proof}, nil
	case transitionActivationPredecessor:
		normalized, err := projecttypeenvselection.NewTransitionStagePredecessor(
			projecttypeenvselection.TransitionStagePredecessorInput{
				Project:           project,
				Head:              value.value.Head(),
				HeadRevision:      value.value.HeadRevision(),
				SelectedComposite: value.value.SelectedComposite(),
			},
		)
		if err != nil {
			return nil, err
		}
		return transitionActivationPredecessor{value: normalized}, nil
	default:
		return nil, fmt.Errorf(
			"activation predecessor encoding variant is required",
		)
	}
}

func encodeActivationPredecessor(
	writer *canonicalWriter,
	predecessor activationPredecessor,
) {
	switch value := predecessor.(type) {
	case currentGenesisActivationPredecessor:
		writer.writeString("genesis")
	case legacyGenesisActivationPredecessor:
		writer.writeString("genesis")
		writer.writeString(value.proof.String())
	case transitionActivationPredecessor:
		writer.writeString("transition")
		writer.writeString(value.value.Project().String())
		writer.writeString(value.value.Head().String())
		writer.writeUint64(value.value.HeadRevision().Value())
		writer.writeString(value.value.SelectedComposite().String())
	}
}

func decodeActivationPredecessor(
	reader *canonicalReader,
	project projectidentity.ProjectID,
	edition carrierEdition,
) (activationPredecessor, error) {
	kind, err := reader.readString("predecessor kind")
	if err != nil {
		return nil, err
	}
	switch kind {
	case "genesis":
		if edition == currentCarrierV2 {
			return currentGenesisActivationPredecessor{}, nil
		}
		proofText, readErr := reader.readString("Genesis proof")
		if readErr != nil {
			return nil, readErr
		}
		proof, parseErr := projecttypeenvselection.ParseNoPriorHeadProofRef(proofText)
		if parseErr != nil {
			return nil, parseErr
		}
		return legacyGenesisActivationPredecessor{proof: proof}, nil
	case "transition":
		projectText, readErr := reader.readString("Transition project")
		if readErr != nil {
			return nil, readErr
		}
		priorProject, parseErr := projectidentity.ParseProjectID(projectText)
		if parseErr != nil || priorProject != project {
			return nil, fmt.Errorf("transition predecessor project mismatch")
		}
		headText, readErr := reader.readString("Transition head")
		if readErr != nil {
			return nil, readErr
		}
		head, parseErr := projecttypeenvselection.ParseProjectTypeEnvHeadRef(headText)
		if parseErr != nil {
			return nil, parseErr
		}
		revisionValue, readErr := reader.readUint64("Transition HeadRevision")
		if readErr != nil {
			return nil, readErr
		}
		revision, parseErr := projecttypeenvselection.NewHeadRevision(revisionValue)
		if parseErr != nil {
			return nil, parseErr
		}
		compositeText, readErr := reader.readString("Transition composite")
		if readErr != nil {
			return nil, readErr
		}
		composite, parseErr := typedmemory.ParseTypeEnvRef(compositeText)
		if parseErr != nil {
			return nil, parseErr
		}
		predecessor, constructionErr :=
			projecttypeenvselection.NewTransitionStagePredecessor(
				projecttypeenvselection.TransitionStagePredecessorInput{
					Project:           project,
					Head:              head,
					HeadRevision:      revision,
					SelectedComposite: composite,
				},
			)
		if constructionErr != nil {
			return nil, constructionErr
		}
		return transitionActivationPredecessor{value: predecessor}, nil
	default:
		return nil, fmt.Errorf("head-selection predecessor kind is invalid")
	}
}

func sameActivationPredecessor(
	left activationPredecessor,
	right activationPredecessor,
) bool {
	switch leftValue := left.(type) {
	case currentGenesisActivationPredecessor:
		_, ok := right.(currentGenesisActivationPredecessor)
		return ok
	case legacyGenesisActivationPredecessor:
		rightValue, ok := right.(legacyGenesisActivationPredecessor)
		return ok && leftValue.proof == rightValue.proof
	case transitionActivationPredecessor:
		rightValue, ok := right.(transitionActivationPredecessor)
		return ok &&
			leftValue.value.Project() == rightValue.value.Project() &&
			leftValue.value.Head() == rightValue.value.Head() &&
			leftValue.value.HeadRevision() == rightValue.value.HeadRevision() &&
			leftValue.value.SelectedComposite() == rightValue.value.SelectedComposite()
	default:
		return false
	}
}

func encodeTarget(writer *canonicalWriter, target Target) error {
	count := len(target.orderedExtensions)
	if count > maximumOrderedExtensions {
		return fmt.Errorf(
			"activation target extension count exceeds %d",
			maximumOrderedExtensions,
		)
	}
	writer.writeString(target.Base().String())
	writer.writeUint32(uint32(count))
	for _, extension := range target.OrderedExtensions() {
		writer.writeString(extension.String())
	}
	writer.writeString(target.RuntimeBasis().String())
	writer.writeString(target.Composite().String())
	writer.writeString(target.Stage().String())
	return nil
}

func decodeTarget(reader *canonicalReader) (Target, error) {
	baseText, err := reader.readString("target base")
	if err != nil {
		return Target{}, err
	}
	base, err := typedmemory.ParseTypeEnvRef(baseText)
	if err != nil {
		return Target{}, err
	}
	count, err := reader.readUint32("target extension count")
	if err != nil {
		return Target{}, err
	}
	if count > maximumOrderedExtensions {
		return Target{}, fmt.Errorf("target extension count is too large")
	}
	extensions := make([]typedmemory.TypeEnvExtensionRef, 0, count)
	for index := uint32(0); index < count; index++ {
		extensionText, readErr := reader.readString("target extension")
		if readErr != nil {
			return Target{}, readErr
		}
		extension, parseErr := typedmemory.ParseTypeEnvExtensionRef(extensionText)
		if parseErr != nil {
			return Target{}, parseErr
		}
		extensions = append(extensions, extension)
	}
	runtimeText, err := reader.readString("target runtime basis")
	if err != nil {
		return Target{}, err
	}
	runtimeBasis, err := projecttypeenv.ParseRuntimeEvaluationBasisRef(runtimeText)
	if err != nil {
		return Target{}, err
	}
	compositeText, err := reader.readString("target composite")
	if err != nil {
		return Target{}, err
	}
	composite, err := typedmemory.ParseTypeEnvRef(compositeText)
	if err != nil {
		return Target{}, err
	}
	stageText, err := reader.readString("target Stage")
	if err != nil {
		return Target{}, err
	}
	stage, err := projecttypeenvselection.ParseProjectTypeEnvStageRef(stageText)
	if err != nil {
		return Target{}, err
	}
	return NewTarget(TargetInput{
		Base:              base,
		OrderedExtensions: extensions,
		RuntimeBasis:      runtimeBasis,
		Composite:         composite,
		Stage:             stage,
	})
}
