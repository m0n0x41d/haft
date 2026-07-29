package projecttypeenvselection

import (
	"bytes"
	"fmt"

	"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv"
	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	projectTypeEnvHeadSelectionRequestDomainV1 = "haft.project-typeenv.head-selection-request.v1"
	projectTypeEnvHeadSelectionRequestDomain   = "haft.project-typeenv.head-selection-request.v2"
	projectTypeEnvHeadSelectionRequestPrefix   = "project-typeenv-head-selection-request:"

	maximumHeadSelectionRequestBytes  = 32 << 20
	maximumHeadSelectionKeyBytes      = 512
	maximumHeadSelectionTargetEntries = 4096
)

// ProjectTypeEnvHeadSelectionPredecessor reuses the one existing exact
// predecessor algebra used by Stage. It is an alias, not a second canonical
// representation.
type ProjectTypeEnvHeadSelectionPredecessor = ProjectTypeEnvStagePredecessor

type ProjectTypeEnvHeadSelectionIdempotencyKey struct{ value string }

func NewProjectTypeEnvHeadSelectionIdempotencyKey(
	raw string,
) (ProjectTypeEnvHeadSelectionIdempotencyKey, error) {
	value, err := normalizeStageText("head-selection idempotency key", raw)
	if err != nil {
		return ProjectTypeEnvHeadSelectionIdempotencyKey{}, err
	}
	if len(value) > maximumHeadSelectionKeyBytes {
		return ProjectTypeEnvHeadSelectionIdempotencyKey{}, fmt.Errorf(
			"head-selection idempotency key exceeds %d bytes",
			maximumHeadSelectionKeyBytes,
		)
	}
	return ProjectTypeEnvHeadSelectionIdempotencyKey{value: value}, nil
}

func (key ProjectTypeEnvHeadSelectionIdempotencyKey) String() string { return key.value }

type ProjectTypeEnvHeadSelectionRequestRef struct{ digest typedmemory.SHA256Digest }

func ParseProjectTypeEnvHeadSelectionRequestRef(
	raw string,
) (ProjectTypeEnvHeadSelectionRequestRef, error) {
	digest, err := parseStageDigestRef(
		"project TypeEnv head-selection request",
		projectTypeEnvHeadSelectionRequestPrefix,
		raw,
	)
	if err != nil {
		return ProjectTypeEnvHeadSelectionRequestRef{}, err
	}
	return ProjectTypeEnvHeadSelectionRequestRef{digest: digest}, nil
}

func (ref ProjectTypeEnvHeadSelectionRequestRef) Digest() typedmemory.SHA256Digest {
	return ref.digest
}

func (ref ProjectTypeEnvHeadSelectionRequestRef) String() string {
	return projectTypeEnvHeadSelectionRequestPrefix + ref.digest.String()
}

// ProjectTypeEnvHeadSelectionTarget keeps the exact recipe reviewable even
// though C transitively authenticates it. The ordered extension slice is the
// Stage order and is never re-sorted by this type.
type ProjectTypeEnvHeadSelectionTarget struct {
	base              typedmemory.TypeEnvRef
	orderedExtensions []typedmemory.TypeEnvExtensionRef
	runtimeBasis      projecttypeenv.RuntimeEvaluationBasisRef
	verifiedComposite typedmemory.TypeEnvRef
	stage             ProjectTypeEnvStageRef
}

func (target ProjectTypeEnvHeadSelectionTarget) Base() typedmemory.TypeEnvRef {
	return target.base
}

func (target ProjectTypeEnvHeadSelectionTarget) OrderedExtensions() []typedmemory.TypeEnvExtensionRef {
	return append([]typedmemory.TypeEnvExtensionRef(nil), target.orderedExtensions...)
}

func (target ProjectTypeEnvHeadSelectionTarget) RuntimeBasis() projecttypeenv.RuntimeEvaluationBasisRef {
	return target.runtimeBasis
}

func (target ProjectTypeEnvHeadSelectionTarget) VerifiedComposite() typedmemory.TypeEnvRef {
	return target.verifiedComposite
}

func (target ProjectTypeEnvHeadSelectionTarget) Stage() ProjectTypeEnvStageRef {
	return target.stage
}

// ProjectTypeEnvHeadSelectionRequest is pure proposal data. It performs no
// head mutation and carries no authority capability.
type ProjectTypeEnvHeadSelectionRequest struct {
	ref                   ProjectTypeEnvHeadSelectionRequestRef
	project               projectidentity.ProjectID
	predecessor           ProjectTypeEnvHeadSelectionPredecessor
	target                ProjectTypeEnvHeadSelectionTarget
	expectedGraphRevision typedmemory.GraphRevision
	idempotencyKey        ProjectTypeEnvHeadSelectionIdempotencyKey
	canonicalBytes        []byte
}

type GenesisProjectTypeEnvHeadSelectionRequestInput struct {
	Project               projectidentity.ProjectID
	Stage                 ProjectTypeEnvStage
	ExpectedGraphRevision typedmemory.GraphRevision
	IdempotencyKey        ProjectTypeEnvHeadSelectionIdempotencyKey
}

func SealGenesisProjectTypeEnvHeadSelectionRequest(
	input GenesisProjectTypeEnvHeadSelectionRequestInput,
) (ProjectTypeEnvHeadSelectionRequest, error) {
	predecessor, ok := input.Stage.Predecessor().(GenesisStagePredecessor)
	if !ok {
		return ProjectTypeEnvHeadSelectionRequest{}, fmt.Errorf("genesis request requires Genesis Stage")
	}
	return sealProjectTypeEnvHeadSelectionRequest(
		input.Project,
		predecessor,
		input.Stage,
		input.ExpectedGraphRevision,
		input.IdempotencyKey,
	)
}

type TransitionProjectTypeEnvHeadSelectionRequestInput struct {
	Project               projectidentity.ProjectID
	ExactPriorHead        ProjectTypeEnvHeadState
	Stage                 ProjectTypeEnvStage
	ExpectedGraphRevision typedmemory.GraphRevision
	IdempotencyKey        ProjectTypeEnvHeadSelectionIdempotencyKey
}

func SealTransitionProjectTypeEnvHeadSelectionRequest(
	input TransitionProjectTypeEnvHeadSelectionRequestInput,
) (ProjectTypeEnvHeadSelectionRequest, error) {
	if err := input.ExactPriorHead.Verify(); err != nil {
		return ProjectTypeEnvHeadSelectionRequest{}, fmt.Errorf("verify exact prior head: %w", err)
	}
	predecessor, ok := input.Stage.Predecessor().(TransitionStagePredecessor)
	if !ok {
		return ProjectTypeEnvHeadSelectionRequest{}, fmt.Errorf("transition request requires Transition Stage")
	}
	if input.ExactPriorHead.Project() != input.Project ||
		predecessor.Project() != input.Project {
		return ProjectTypeEnvHeadSelectionRequest{}, fmt.Errorf("transition prior-head project mismatch")
	}
	if predecessor.Head() != input.ExactPriorHead.Ref() ||
		predecessor.HeadRevision() != input.ExactPriorHead.Revision() ||
		predecessor.SelectedComposite() != input.ExactPriorHead.SelectedComposite() {
		return ProjectTypeEnvHeadSelectionRequest{}, fmt.Errorf("transition Stage prior head mismatch")
	}
	return sealProjectTypeEnvHeadSelectionRequest(
		input.Project,
		predecessor,
		input.Stage,
		input.ExpectedGraphRevision,
		input.IdempotencyKey,
	)
}

func sealProjectTypeEnvHeadSelectionRequest(
	project projectidentity.ProjectID,
	predecessor ProjectTypeEnvHeadSelectionPredecessor,
	stage ProjectTypeEnvStage,
	expectedGraphRevision typedmemory.GraphRevision,
	idempotencyKey ProjectTypeEnvHeadSelectionIdempotencyKey,
) (ProjectTypeEnvHeadSelectionRequest, error) {
	if err := stage.Verify(); err != nil {
		return ProjectTypeEnvHeadSelectionRequest{}, fmt.Errorf("verify head-selection Stage: %w", err)
	}
	target, err := projectTypeEnvHeadSelectionTargetFromStage(stage)
	if err != nil {
		return ProjectTypeEnvHeadSelectionRequest{}, err
	}
	state, err := normalizeHeadSelectionRequestState(headSelectionRequestState{
		project:               project,
		predecessor:           predecessor,
		target:                target,
		expectedGraphRevision: expectedGraphRevision,
		idempotencyKey:        idempotencyKey,
	})
	if err != nil {
		return ProjectTypeEnvHeadSelectionRequest{}, err
	}
	canonical, err := encodeHeadSelectionRequestState(state)
	if err != nil {
		return ProjectTypeEnvHeadSelectionRequest{}, err
	}
	request, err := DecodeProjectTypeEnvHeadSelectionRequest(canonical)
	if err != nil {
		return ProjectTypeEnvHeadSelectionRequest{}, err
	}
	if err := VerifyProjectTypeEnvHeadSelectionRequestAgainstStage(request, stage); err != nil {
		return ProjectTypeEnvHeadSelectionRequest{}, err
	}
	return request, nil
}

func DecodeProjectTypeEnvHeadSelectionRequest(
	canonical []byte,
) (ProjectTypeEnvHeadSelectionRequest, error) {
	if len(canonical) == 0 {
		return ProjectTypeEnvHeadSelectionRequest{}, fmt.Errorf("project TypeEnv head-selection request is empty")
	}
	if len(canonical) > maximumHeadSelectionRequestBytes {
		return ProjectTypeEnvHeadSelectionRequest{}, fmt.Errorf(
			"project TypeEnv head-selection request exceeds %d bytes",
			maximumHeadSelectionRequestBytes,
		)
	}
	reader := stageReader{value: canonical}
	domain, err := reader.readString("head-selection request domain")
	if err != nil {
		return ProjectTypeEnvHeadSelectionRequest{}, err
	}
	if domain != projectTypeEnvHeadSelectionRequestDomain &&
		domain != projectTypeEnvHeadSelectionRequestDomainV1 {
		return ProjectTypeEnvHeadSelectionRequest{}, fmt.Errorf("head-selection request domain is invalid")
	}
	state, err := decodeHeadSelectionRequestState(&reader, domain)
	if err != nil {
		return ProjectTypeEnvHeadSelectionRequest{}, err
	}
	if reader.remaining() != 0 {
		return ProjectTypeEnvHeadSelectionRequest{}, fmt.Errorf("head-selection request has trailing bytes")
	}
	normalized, err := normalizeHeadSelectionRequestState(state)
	if err != nil {
		return ProjectTypeEnvHeadSelectionRequest{}, err
	}
	reencoded, err := encodeHeadSelectionRequestStateForDomain(normalized, domain)
	if err != nil {
		return ProjectTypeEnvHeadSelectionRequest{}, err
	}
	if !bytes.Equal(reencoded, canonical) {
		return ProjectTypeEnvHeadSelectionRequest{}, fmt.Errorf("head-selection request is not canonical")
	}
	digest, err := deriveStageProjectionDigest(canonical)
	if err != nil {
		return ProjectTypeEnvHeadSelectionRequest{}, err
	}
	return ProjectTypeEnvHeadSelectionRequest{
		ref:                   ProjectTypeEnvHeadSelectionRequestRef{digest: digest},
		project:               normalized.project,
		predecessor:           normalized.predecessor,
		target:                normalized.target,
		expectedGraphRevision: normalized.expectedGraphRevision,
		idempotencyKey:        normalized.idempotencyKey,
		canonicalBytes:        append([]byte(nil), canonical...),
	}, nil
}

func VerifyProjectTypeEnvHeadSelectionRequest(
	expected ProjectTypeEnvHeadSelectionRequestRef,
	canonical []byte,
) (ProjectTypeEnvHeadSelectionRequest, error) {
	parsed, err := ParseProjectTypeEnvHeadSelectionRequestRef(expected.String())
	if err != nil || parsed != expected {
		return ProjectTypeEnvHeadSelectionRequest{}, fmt.Errorf("expected head-selection request reference is invalid")
	}
	request, err := DecodeProjectTypeEnvHeadSelectionRequest(canonical)
	if err != nil {
		return ProjectTypeEnvHeadSelectionRequest{}, err
	}
	if request.ref != expected {
		return ProjectTypeEnvHeadSelectionRequest{}, fmt.Errorf("head-selection request reference mismatch")
	}
	return request, nil
}

func VerifyProjectTypeEnvHeadSelectionRequestAgainstStage(
	request ProjectTypeEnvHeadSelectionRequest,
	stage ProjectTypeEnvStage,
) error {
	if err := request.Verify(); err != nil {
		return err
	}
	if err := stage.Verify(); err != nil {
		return err
	}
	stagePredecessor := stage.Predecessor()
	checks := []struct {
		matches bool
		label   string
	}{
		{request.project == stage.Project(), "project"},
		{sameHeadSelectionPredecessor(request.predecessor, stagePredecessor), "predecessor"},
		{request.target.base == stage.Base(), "base B"},
		{orderedExtensionRefsEqual(request.target.orderedExtensions, stage.OrderedExtensions()), "ordered E DAG"},
		{request.target.runtimeBasis == stage.RuntimeBasis(), "runtime basis X"},
		{request.target.verifiedComposite == stage.VerifiedComposite(), "verified composite C"},
		{request.target.stage == stage.Ref(), "Stage"},
		{request.expectedGraphRevision == stage.GraphRevision(), "expected graph revision"},
	}
	for _, check := range checks {
		if !check.matches {
			return fmt.Errorf("head-selection request %s mismatch", check.label)
		}
	}
	return nil
}

// VerifyGenesisProjectTypeEnvHeadSelectionRequestStructure proves canonical
// Stage/request equality only. Current head absence is an admission-time
// effect and is intentionally absent from this pure request.
func VerifyGenesisProjectTypeEnvHeadSelectionRequestStructure(
	request ProjectTypeEnvHeadSelectionRequest,
	stage ProjectTypeEnvStage,
) error {
	_, ok := request.predecessor.(GenesisStagePredecessor)
	if !ok {
		return fmt.Errorf("head-selection request is not Genesis")
	}
	if err := VerifyProjectTypeEnvHeadSelectionRequestAgainstStage(request, stage); err != nil {
		return err
	}
	return nil
}

// VerifyTransitionProjectTypeEnvHeadSelectionRequestStructure proves
// canonical cross-coordinate equality only. It is not a current-head reread or
// a CAS witness.
func VerifyTransitionProjectTypeEnvHeadSelectionRequestStructure(
	request ProjectTypeEnvHeadSelectionRequest,
	prior ProjectTypeEnvHeadState,
	stage ProjectTypeEnvStage,
) error {
	predecessor, ok := request.predecessor.(TransitionStagePredecessor)
	if !ok {
		return fmt.Errorf("head-selection request is not Transition")
	}
	if err := VerifyProjectTypeEnvHeadSelectionRequestAgainstStage(request, stage); err != nil {
		return err
	}
	if err := prior.Verify(); err != nil {
		return err
	}
	checks := []struct {
		matches bool
		label   string
	}{
		{request.project == prior.Project(), "prior-head project"},
		{predecessor.Head() == prior.Ref(), "prior-head reference"},
		{predecessor.HeadRevision() == prior.Revision(), "prior HeadRevision"},
		{predecessor.SelectedComposite() == prior.SelectedComposite(), "prior selected C"},
	}
	for _, check := range checks {
		if !check.matches {
			return fmt.Errorf(
				"transition head-selection request %s mismatch",
				check.label,
			)
		}
	}
	return nil
}

func (request ProjectTypeEnvHeadSelectionRequest) Ref() ProjectTypeEnvHeadSelectionRequestRef {
	return request.ref
}

func (request ProjectTypeEnvHeadSelectionRequest) Project() projectidentity.ProjectID {
	return request.project
}

func (request ProjectTypeEnvHeadSelectionRequest) Head() (ProjectTypeEnvHeadRef, error) {
	return ProjectTypeEnvHeadRefForProject(request.project)
}

func (request ProjectTypeEnvHeadSelectionRequest) Predecessor() ProjectTypeEnvHeadSelectionPredecessor {
	predecessor, _ := normalizeStagePredecessor(request.predecessor)
	return predecessor
}

func (request ProjectTypeEnvHeadSelectionRequest) Target() ProjectTypeEnvHeadSelectionTarget {
	target := request.target
	target.orderedExtensions = append(
		[]typedmemory.TypeEnvExtensionRef(nil),
		request.target.orderedExtensions...,
	)
	return target
}

func (request ProjectTypeEnvHeadSelectionRequest) ExpectedGraphRevision() typedmemory.GraphRevision {
	return request.expectedGraphRevision
}

func (request ProjectTypeEnvHeadSelectionRequest) IdempotencyKey() ProjectTypeEnvHeadSelectionIdempotencyKey {
	return request.idempotencyKey
}

func (request ProjectTypeEnvHeadSelectionRequest) CanonicalBytes() []byte {
	return append([]byte(nil), request.canonicalBytes...)
}

func (request ProjectTypeEnvHeadSelectionRequest) Verify() error {
	verified, err := VerifyProjectTypeEnvHeadSelectionRequest(request.ref, request.canonicalBytes)
	if err != nil {
		return err
	}
	if verified.project != request.project ||
		!sameHeadSelectionPredecessor(verified.predecessor, request.predecessor) ||
		!sameHeadSelectionTarget(verified.target, request.target) ||
		verified.expectedGraphRevision != request.expectedGraphRevision ||
		verified.idempotencyKey != request.idempotencyKey {
		return fmt.Errorf("head-selection request stored state differs from canonical bytes")
	}
	return nil
}

type headSelectionRequestState struct {
	project               projectidentity.ProjectID
	predecessor           ProjectTypeEnvHeadSelectionPredecessor
	target                ProjectTypeEnvHeadSelectionTarget
	expectedGraphRevision typedmemory.GraphRevision
	idempotencyKey        ProjectTypeEnvHeadSelectionIdempotencyKey
}

func normalizeHeadSelectionRequestState(
	state headSelectionRequestState,
) (headSelectionRequestState, error) {
	project, err := projectidentity.ParseProjectID(state.project.String())
	if err != nil || project != state.project {
		return headSelectionRequestState{}, fmt.Errorf("head-selection project is required")
	}
	predecessor, err := normalizeStagePredecessor(state.predecessor)
	if err != nil {
		return headSelectionRequestState{}, err
	}
	if transition, ok := predecessor.(TransitionStagePredecessor); ok &&
		transition.Project() != project {
		return headSelectionRequestState{}, fmt.Errorf("head-selection prior-head project mismatch")
	}
	target, err := normalizeHeadSelectionTarget(state.target)
	if err != nil {
		return headSelectionRequestState{}, err
	}
	key, err := NewProjectTypeEnvHeadSelectionIdempotencyKey(state.idempotencyKey.String())
	if err != nil || key != state.idempotencyKey {
		return headSelectionRequestState{}, fmt.Errorf("head-selection idempotency key is required")
	}
	return headSelectionRequestState{
		project:               project,
		predecessor:           predecessor,
		target:                target,
		expectedGraphRevision: state.expectedGraphRevision,
		idempotencyKey:        key,
	}, nil
}

func projectTypeEnvHeadSelectionTargetFromStage(
	stage ProjectTypeEnvStage,
) (ProjectTypeEnvHeadSelectionTarget, error) {
	if err := stage.Verify(); err != nil {
		return ProjectTypeEnvHeadSelectionTarget{}, err
	}
	return normalizeHeadSelectionTarget(ProjectTypeEnvHeadSelectionTarget{
		base:              stage.Base(),
		orderedExtensions: stage.OrderedExtensions(),
		runtimeBasis:      stage.RuntimeBasis(),
		verifiedComposite: stage.VerifiedComposite(),
		stage:             stage.Ref(),
	})
}

func normalizeHeadSelectionTarget(
	target ProjectTypeEnvHeadSelectionTarget,
) (ProjectTypeEnvHeadSelectionTarget, error) {
	base, err := normalizeTypeEnvRef("head-selection base B", target.base)
	if err != nil {
		return ProjectTypeEnvHeadSelectionTarget{}, err
	}
	extensions, err := normalizeOrderedExtensionRefs(target.orderedExtensions)
	if err != nil {
		return ProjectTypeEnvHeadSelectionTarget{}, err
	}
	runtimeBasis, err := normalizeRuntimeBasisRef(target.runtimeBasis)
	if err != nil {
		return ProjectTypeEnvHeadSelectionTarget{}, err
	}
	composite, err := normalizeTypeEnvRef("head-selection verified composite C", target.verifiedComposite)
	if err != nil {
		return ProjectTypeEnvHeadSelectionTarget{}, err
	}
	stage, err := ParseProjectTypeEnvStageRef(target.stage.String())
	if err != nil || stage != target.stage {
		return ProjectTypeEnvHeadSelectionTarget{}, fmt.Errorf("head-selection Stage is required")
	}
	return ProjectTypeEnvHeadSelectionTarget{
		base:              base,
		orderedExtensions: extensions,
		runtimeBasis:      runtimeBasis,
		verifiedComposite: composite,
		stage:             stage,
	}, nil
}

func encodeHeadSelectionRequestState(
	state headSelectionRequestState,
) ([]byte, error) {
	return encodeHeadSelectionRequestStateForDomain(
		state,
		projectTypeEnvHeadSelectionRequestDomain,
	)
}

func encodeHeadSelectionRequestStateForDomain(
	state headSelectionRequestState,
	domain string,
) ([]byte, error) {
	writer := stageWriter{}
	writer.addString(domain)
	writer.addString(state.project.String())
	if err := encodeStagePredecessor(&writer, state.predecessor, domain); err != nil {
		return nil, err
	}
	encodeHeadSelectionTarget(&writer, state.target)
	writer.addUint64(state.expectedGraphRevision.Value())
	writer.addString(state.idempotencyKey.String())
	return writer.bytes(), nil
}

func decodeHeadSelectionRequestState(
	reader *stageReader,
	domain string,
) (headSelectionRequestState, error) {
	projectText, err := reader.readString("head-selection project")
	if err != nil {
		return headSelectionRequestState{}, err
	}
	project, err := projectidentity.ParseProjectID(projectText)
	if err != nil {
		return headSelectionRequestState{}, err
	}
	predecessor, err := decodeStagePredecessor(reader, domain)
	if err != nil {
		return headSelectionRequestState{}, err
	}
	target, err := decodeHeadSelectionTarget(reader)
	if err != nil {
		return headSelectionRequestState{}, err
	}
	revision, err := reader.readUint64("head-selection expected graph revision")
	if err != nil {
		return headSelectionRequestState{}, err
	}
	keyText, err := reader.readString("head-selection idempotency key")
	if err != nil {
		return headSelectionRequestState{}, err
	}
	key, err := NewProjectTypeEnvHeadSelectionIdempotencyKey(keyText)
	if err != nil {
		return headSelectionRequestState{}, err
	}
	return headSelectionRequestState{
		project:               project,
		predecessor:           predecessor,
		target:                target,
		expectedGraphRevision: typedmemory.NewGraphRevision(revision),
		idempotencyKey:        key,
	}, nil
}

func encodeHeadSelectionTarget(writer *stageWriter, target ProjectTypeEnvHeadSelectionTarget) {
	writer.addString(target.base.String())
	writer.addUint64(uint64(len(target.orderedExtensions)))
	for _, extension := range target.orderedExtensions {
		writer.addString(extension.String())
	}
	writer.addString(target.runtimeBasis.String())
	writer.addString(target.verifiedComposite.String())
	writer.addString(target.stage.String())
}

func decodeHeadSelectionTarget(reader *stageReader) (ProjectTypeEnvHeadSelectionTarget, error) {
	baseText, err := reader.readString("head-selection base B")
	if err != nil {
		return ProjectTypeEnvHeadSelectionTarget{}, err
	}
	base, err := typedmemory.ParseTypeEnvRef(baseText)
	if err != nil {
		return ProjectTypeEnvHeadSelectionTarget{}, err
	}
	count, err := reader.readCount(
		"head-selection ordered E DAG",
		maximumHeadSelectionTargetEntries,
	)
	if err != nil {
		return ProjectTypeEnvHeadSelectionTarget{}, err
	}
	extensions := make([]typedmemory.TypeEnvExtensionRef, 0, count)
	for index := 0; index < count; index++ {
		text, readErr := reader.readString("head-selection extension")
		if readErr != nil {
			return ProjectTypeEnvHeadSelectionTarget{}, readErr
		}
		extension, parseErr := typedmemory.ParseTypeEnvExtensionRef(text)
		if parseErr != nil {
			return ProjectTypeEnvHeadSelectionTarget{}, fmt.Errorf(
				"decode head-selection extension %d: %w",
				index,
				parseErr,
			)
		}
		extensions = append(extensions, extension)
	}
	runtimeText, err := reader.readString("head-selection runtime basis X")
	if err != nil {
		return ProjectTypeEnvHeadSelectionTarget{}, err
	}
	runtimeBasis, err := projecttypeenv.ParseRuntimeEvaluationBasisRef(runtimeText)
	if err != nil {
		return ProjectTypeEnvHeadSelectionTarget{}, err
	}
	compositeText, err := reader.readString("head-selection verified composite C")
	if err != nil {
		return ProjectTypeEnvHeadSelectionTarget{}, err
	}
	composite, err := typedmemory.ParseTypeEnvRef(compositeText)
	if err != nil {
		return ProjectTypeEnvHeadSelectionTarget{}, err
	}
	stageText, err := reader.readString("head-selection Stage")
	if err != nil {
		return ProjectTypeEnvHeadSelectionTarget{}, err
	}
	stage, err := ParseProjectTypeEnvStageRef(stageText)
	if err != nil {
		return ProjectTypeEnvHeadSelectionTarget{}, err
	}
	return ProjectTypeEnvHeadSelectionTarget{
		base:              base,
		orderedExtensions: extensions,
		runtimeBasis:      runtimeBasis,
		verifiedComposite: composite,
		stage:             stage,
	}, nil
}

func sameHeadSelectionPredecessor(
	left ProjectTypeEnvHeadSelectionPredecessor,
	right ProjectTypeEnvHeadSelectionPredecessor,
) bool {
	switch leftValue := left.(type) {
	case GenesisStagePredecessor:
		_, ok := right.(GenesisStagePredecessor)
		return ok
	case legacyGenesisStagePredecessor:
		rightValue, ok := right.(legacyGenesisStagePredecessor)
		return ok && leftValue.noPriorHeadProof == rightValue.noPriorHeadProof
	case TransitionStagePredecessor:
		rightValue, ok := right.(TransitionStagePredecessor)
		return ok && leftValue.Project() == rightValue.Project() &&
			leftValue.Head() == rightValue.Head() &&
			leftValue.HeadRevision() == rightValue.HeadRevision() &&
			leftValue.SelectedComposite() == rightValue.SelectedComposite()
	default:
		return false
	}
}

func sameHeadSelectionTarget(
	left ProjectTypeEnvHeadSelectionTarget,
	right ProjectTypeEnvHeadSelectionTarget,
) bool {
	return left.base == right.base &&
		orderedExtensionRefsEqual(left.orderedExtensions, right.orderedExtensions) &&
		left.runtimeBasis == right.runtimeBasis &&
		left.verifiedComposite == right.verifiedComposite &&
		left.stage == right.stage
}
