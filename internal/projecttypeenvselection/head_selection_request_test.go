package projecttypeenvselection

import (
	"bytes"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestNoPriorHeadProofCanonicalRoundTripAndSnapshotBinding(t *testing.T) {
	input := stageGenesisInput(t, 17)
	proof := sealNoPriorHeadProofFixture(t, input)

	if err := proof.Verify(); err != nil {
		t.Fatalf("Verify(): %v", err)
	}
	if proof.Project() != input.Project ||
		proof.Head() != mustHeadRef(t, input.Project) ||
		proof.GraphSnapshotBasis() != input.GraphSnapshotBasis.Ref() ||
		proof.GraphSnapshotBasisDigest() != input.GraphSnapshotBasis.Ref().Digest() ||
		proof.ExpectedGraphRevision() != input.GraphRevision {
		t.Fatal("proof lost exact project/head/snapshot/revision coordinates")
	}
	decoded, err := VerifyNoPriorHeadProof(proof.Ref(), proof.CanonicalBytes())
	if err != nil {
		t.Fatalf("VerifyNoPriorHeadProof(): %v", err)
	}
	if !bytes.Equal(decoded.CanonicalBytes(), proof.CanonicalBytes()) {
		t.Fatal("proof canonical roundtrip changed bytes")
	}
	if err := VerifyNoPriorHeadProofAgainstGraphSnapshot(
		proof,
		input.GraphSnapshotBasis,
	); err != nil {
		t.Fatalf("VerifyNoPriorHeadProofAgainstGraphSnapshot(): %v", err)
	}

	copyBytes := proof.CanonicalBytes()
	copyBytes[0] ^= 0xff
	if err := proof.Verify(); err != nil {
		t.Fatalf("caller mutation changed stored proof: %v", err)
	}
	if _, err := DecodeNoPriorHeadProof(append(proof.CanonicalBytes(), 0x00)); err == nil {
		t.Fatal("proof decoder accepted trailing bytes")
	}
	mutated := proof.CanonicalBytes()
	mutated[len(mutated)-1] ^= 0x01
	if _, err := VerifyNoPriorHeadProof(proof.Ref(), mutated); err == nil {
		t.Fatal("proof verifier accepted mutated bytes under the original ref")
	}
	forged := mustNoPriorProofRef(t, "f")
	if _, err := VerifyNoPriorHeadProof(forged, proof.CanonicalBytes()); err == nil {
		t.Fatal("proof verifier accepted a forged reference")
	}
	otherSnapshot := testCommittedSnapshotBasis(t, 17, "4")
	if err := VerifyNoPriorHeadProofAgainstGraphSnapshot(proof, otherSnapshot); err == nil {
		t.Fatal("proof verifier accepted a substituted graph snapshot")
	}
}

func TestNoPriorHeadProofRejectsProjectHeadAndRevisionSubstitution(t *testing.T) {
	input := stageGenesisInput(t, 17)
	foreign := mustProjectID(t, "qnt_deadbeef")

	_, err := sealNoPriorHeadProof(noPriorHeadProofInput{
		Project:               foreign,
		Head:                  mustHeadRef(t, foreign),
		GraphSnapshot:         input.GraphSnapshotBasis,
		ExpectedGraphRevision: input.GraphRevision,
	})
	if err == nil || !strings.Contains(err.Error(), "snapshot project mismatch") {
		t.Fatalf("cross-project proof error = %v", err)
	}

	_, err = sealNoPriorHeadProof(noPriorHeadProofInput{
		Project:               input.Project,
		Head:                  mustHeadRef(t, foreign),
		GraphSnapshot:         input.GraphSnapshotBasis,
		ExpectedGraphRevision: input.GraphRevision,
	})
	if err == nil || !strings.Contains(err.Error(), "head project mismatch") {
		t.Fatalf("cross-head proof error = %v", err)
	}

	_, err = sealNoPriorHeadProof(noPriorHeadProofInput{
		Project:               input.Project,
		Head:                  mustHeadRef(t, input.Project),
		GraphSnapshot:         input.GraphSnapshotBasis,
		ExpectedGraphRevision: typedmemory.NewGraphRevision(18),
	})
	if err == nil || !strings.Contains(err.Error(), "snapshot revision mismatch") {
		t.Fatalf("cross-revision proof error = %v", err)
	}
}

func TestGenesisHeadSelectionRequestCanonicalRoundTripAndSuccessor(t *testing.T) {
	_, stage, request := genesisHeadSelectionFixture(
		t,
		17,
		"head-selection-genesis",
	)

	if err := request.Verify(); err != nil {
		t.Fatalf("Verify(): %v", err)
	}
	requestBytes := request.CanonicalBytes()
	requestBytes[0] ^= 0xff
	if err := request.Verify(); err != nil {
		t.Fatalf("caller mutation changed stored request: %v", err)
	}
	if err := VerifyGenesisProjectTypeEnvHeadSelectionRequestStructure(
		request,
		stage,
	); err != nil {
		t.Fatalf("VerifyGenesisProjectTypeEnvHeadSelectionRequestStructure(): %v", err)
	}
	decoded, err := VerifyProjectTypeEnvHeadSelectionRequest(
		request.Ref(),
		request.CanonicalBytes(),
	)
	if err != nil {
		t.Fatalf("VerifyProjectTypeEnvHeadSelectionRequest(): %v", err)
	}
	if decoded.Project() != stage.Project() ||
		decoded.ExpectedGraphRevision() != stage.GraphRevision() ||
		decoded.IdempotencyKey() != request.IdempotencyKey() {
		t.Fatal("decoded Genesis request lost common coordinates")
	}
	reader := stageReader{value: decoded.CanonicalBytes()}
	domain, err := reader.readString("head-selection request domain")
	if err != nil || domain != projectTypeEnvHeadSelectionRequestDomain {
		t.Fatalf("current request domain = %q, %v; want v2", domain, err)
	}
	target := decoded.Target()
	if target.Base() != stage.Base() ||
		target.RuntimeBasis() != stage.RuntimeBasis() ||
		target.VerifiedComposite() != stage.VerifiedComposite() ||
		target.Stage() != stage.Ref() ||
		!orderedExtensionRefsEqual(target.OrderedExtensions(), stage.OrderedExtensions()) {
		t.Fatal("decoded Genesis request lost exact B/E/X/C/Stage target")
	}
	extensions := target.OrderedExtensions()
	if len(extensions) > 0 {
		extensions[0] = typedmemory.TypeEnvExtensionRef{}
	}
	if !orderedExtensionRefsEqual(
		decoded.Target().OrderedExtensions(),
		stage.OrderedExtensions(),
	) {
		t.Fatal("caller mutation changed stored target E")
	}
	_, ok := decoded.Predecessor().(GenesisStagePredecessor)
	if !ok {
		t.Fatalf("decoded predecessor = %T; want Genesis tag", decoded.Predecessor())
	}
	if bytes.Contains(
		decoded.CanonicalBytes(),
		[]byte(noPriorHeadProofRefPrefix),
	) {
		t.Fatal("current Genesis request leaked a no-prior-head proof identity")
	}
	successor, err := DeriveGenesisProjectTypeEnvHeadSuccessorCandidate(
		request,
		stage,
	)
	if err != nil {
		t.Fatalf("DeriveGenesisProjectTypeEnvHeadSuccessorCandidate(): %v", err)
	}
	if successor.Revision().Value() != 1 {
		t.Fatalf("Genesis HeadRevision = %d; want 1", successor.Revision().Value())
	}
	if successor.Project() != request.Project() ||
		successor.SelectedComposite() != request.Target().VerifiedComposite() {
		t.Fatal("Genesis successor lost project or selected C")
	}
}

func TestGenesisHeadSelectionRequestV1DecodesReadOnly(t *testing.T) {
	stage := mustLoadReconstructedStageV2Genesis(t)
	target, err := projectTypeEnvHeadSelectionTargetFromStage(stage)
	if err != nil {
		t.Fatalf("projectTypeEnvHeadSelectionTargetFromStage(): %v", err)
	}
	predecessor, ok := stage.Predecessor().(legacyGenesisStagePredecessor)
	if !ok {
		t.Fatalf(
			"frozen legacy Stage predecessor = %T",
			stage.Predecessor(),
		)
	}
	state, err := normalizeHeadSelectionRequestState(headSelectionRequestState{
		project:               stage.Project(),
		predecessor:           predecessor,
		target:                target,
		expectedGraphRevision: stage.GraphRevision(),
		idempotencyKey:        mustHeadSelectionKey(t, "legacy-genesis-v1"),
	})
	if err != nil {
		t.Fatalf("normalize legacy request fixture: %v", err)
	}
	canonical, err := encodeHeadSelectionRequestStateForDomain(
		state,
		projectTypeEnvHeadSelectionRequestDomainV1,
	)
	if err != nil {
		t.Fatalf("encode legacy request fixture: %v", err)
	}
	request, err := DecodeProjectTypeEnvHeadSelectionRequest(canonical)
	if err != nil {
		t.Fatalf("DecodeProjectTypeEnvHeadSelectionRequest(v1): %v", err)
	}
	if err := request.Verify(); err != nil {
		t.Fatalf("Verify legacy request: %v", err)
	}
	decodedPredecessor, ok := request.Predecessor().(legacyGenesisStagePredecessor)
	if !ok {
		t.Fatalf("legacy request predecessor = %T", request.Predecessor())
	}
	if decodedPredecessor.noPriorHeadProof != predecessor.noPriorHeadProof {
		t.Fatal("legacy request lost its historical proof reference")
	}
	if err := VerifyProjectTypeEnvHeadSelectionRequestAgainstStage(
		request,
		stage,
	); err != nil {
		t.Fatalf("legacy request/Stage structural match: %v", err)
	}
	if err := VerifyGenesisProjectTypeEnvHeadSelectionRequestStructure(
		request,
		stage,
	); err == nil {
		t.Fatal("current Genesis verifier admitted a legacy proof-linked request")
	}
}

func TestTransitionHeadSelectionRequestBindsExactPriorHead(t *testing.T) {
	stageInput := stageTransitionInput(t, 29)
	stage := sealStageFixture(t, stageInput)
	predecessor := stage.Predecessor().(TransitionStagePredecessor)
	prior := sealHeadStateFixture(
		t,
		predecessor.Project(),
		predecessor.SelectedComposite(),
		predecessor.HeadRevision(),
	)
	request, err := SealTransitionProjectTypeEnvHeadSelectionRequest(
		TransitionProjectTypeEnvHeadSelectionRequestInput{
			Project:               stage.Project(),
			ExactPriorHead:        prior,
			Stage:                 stage,
			ExpectedGraphRevision: stage.GraphRevision(),
			IdempotencyKey:        mustHeadSelectionKey(t, "head-selection-transition"),
		},
	)
	if err != nil {
		t.Fatalf("SealTransitionProjectTypeEnvHeadSelectionRequest(): %v", err)
	}
	if err := VerifyProjectTypeEnvHeadSelectionRequestAgainstStage(request, stage); err != nil {
		t.Fatalf("VerifyProjectTypeEnvHeadSelectionRequestAgainstStage(): %v", err)
	}
	if err := VerifyTransitionProjectTypeEnvHeadSelectionRequestStructure(
		request,
		prior,
		stage,
	); err != nil {
		t.Fatalf("VerifyTransitionProjectTypeEnvHeadSelectionRequestStructure(): %v", err)
	}
	successor, err := DeriveTransitionProjectTypeEnvHeadSuccessorCandidate(
		request,
		prior,
		stage,
	)
	if err != nil {
		t.Fatalf("DeriveTransitionProjectTypeEnvHeadSuccessorCandidate(): %v", err)
	}
	if successor.Revision().Value() != predecessor.HeadRevision().Value()+1 {
		t.Fatalf(
			"Transition HeadRevision = %d; want %d",
			successor.Revision().Value(),
			predecessor.HeadRevision().Value()+1,
		)
	}
	if successor.Ref() != predecessor.Head() {
		t.Fatal("Transition changed the stable head ref")
	}
	if successor.SelectedComposite() != stage.VerifiedComposite() {
		t.Fatal("Transition did not select target C")
	}
	if _, err := SealTransitionProjectTypeEnvHeadSelectionRequest(
		TransitionProjectTypeEnvHeadSelectionRequestInput{
			Project:               stage.Project(),
			ExactPriorHead:        prior,
			Stage:                 stage,
			ExpectedGraphRevision: typedmemory.NewGraphRevision(30),
			IdempotencyKey:        mustHeadSelectionKey(t, "transition-wrong-graph-revision"),
		},
	); err == nil {
		t.Fatal("Transition request accepted a mismatched GraphRevision")
	}
	invalidPredecessor := TransitionStagePredecessor{
		project:           prior.Project(),
		head:              prior.Ref(),
		headRevision:      HeadRevision{},
		selectedComposite: prior.SelectedComposite(),
	}
	invalidCanonical, err := encodeHeadSelectionRequestState(headSelectionRequestState{
		project:               request.Project(),
		predecessor:           invalidPredecessor,
		target:                request.Target(),
		expectedGraphRevision: request.ExpectedGraphRevision(),
		idempotencyKey:        request.IdempotencyKey(),
	})
	if err != nil {
		t.Fatalf("encode invalid request fixture: %v", err)
	}
	if _, err := DecodeProjectTypeEnvHeadSelectionRequest(invalidCanonical); err == nil {
		t.Fatal("request decoder accepted zero Transition HeadRevision")
	}
}

func TestHeadRevisionAndGraphRevisionAreIndependentStrongCoordinates(t *testing.T) {
	_, stage, request := genesisHeadSelectionFixture(
		t,
		1,
		"equal-numeric-revisions",
	)
	successor, err := DeriveGenesisProjectTypeEnvHeadSuccessorCandidate(
		request,
		stage,
	)
	if err != nil {
		t.Fatalf("DeriveGenesisProjectTypeEnvHeadSuccessorCandidate(): %v", err)
	}
	var headRevision HeadRevision = successor.Revision()
	var graphRevision typedmemory.GraphRevision = request.ExpectedGraphRevision()
	if headRevision.Value() != 1 || graphRevision.Value() != 1 {
		t.Fatalf(
			"numeric coordinates = HeadRevision %d, GraphRevision %d; want 1, 1",
			headRevision.Value(),
			graphRevision.Value(),
		)
	}
}

func TestHeadSelectionRequestsRejectCrossBinding(t *testing.T) {
	_, stage, _ := genesisHeadSelectionFixture(t, 17, "head-selection-cross-binding")
	foreign := mustProjectID(t, "qnt_deadbeef")

	_, err := SealGenesisProjectTypeEnvHeadSelectionRequest(
		GenesisProjectTypeEnvHeadSelectionRequestInput{
			Project:               foreign,
			Stage:                 stage,
			ExpectedGraphRevision: stage.GraphRevision(),
			IdempotencyKey:        mustHeadSelectionKey(t, "wrong-project"),
		},
	)
	if err == nil || !strings.Contains(err.Error(), "project mismatch") {
		t.Fatalf("cross-project Genesis error = %v", err)
	}

	_, err = SealGenesisProjectTypeEnvHeadSelectionRequest(
		GenesisProjectTypeEnvHeadSelectionRequestInput{
			Project:               stage.Project(),
			Stage:                 stage,
			ExpectedGraphRevision: typedmemory.NewGraphRevision(18),
			IdempotencyKey:        mustHeadSelectionKey(t, "wrong-revision"),
		},
	)
	if err == nil || !strings.Contains(err.Error(), "expected graph revision mismatch") {
		t.Fatalf("cross-revision Genesis error = %v", err)
	}

	transitionInput := stageTransitionInput(t, 29)
	transitionStage := sealStageFixture(t, transitionInput)
	predecessor := transitionStage.Predecessor().(TransitionStagePredecessor)
	wrongPrior := sealHeadStateFixture(
		t,
		predecessor.Project(),
		testTypeEnvRef(t, "d"),
		predecessor.HeadRevision(),
	)
	_, err = SealTransitionProjectTypeEnvHeadSelectionRequest(
		TransitionProjectTypeEnvHeadSelectionRequestInput{
			Project:               transitionStage.Project(),
			ExactPriorHead:        wrongPrior,
			Stage:                 transitionStage,
			ExpectedGraphRevision: transitionStage.GraphRevision(),
			IdempotencyKey:        mustHeadSelectionKey(t, "wrong-prior"),
		},
	)
	if err == nil || !strings.Contains(err.Error(), "prior head mismatch") {
		t.Fatalf("cross-prior Transition error = %v", err)
	}
}

func TestHeadSelectionRequestRejectsMutatedTrailingAndSubstitutedStage(t *testing.T) {
	_, stage, request := genesisHeadSelectionFixture(t, 17, "head-selection-canonical")
	canonical := request.CanonicalBytes()

	mutated := append([]byte(nil), canonical...)
	mutated[len(mutated)-1] ^= 0x01
	if _, err := VerifyProjectTypeEnvHeadSelectionRequest(request.Ref(), mutated); err == nil {
		t.Fatal("request verifier accepted mutated bytes under the original ref")
	}
	if _, err := DecodeProjectTypeEnvHeadSelectionRequest(append(canonical, 0x00)); err == nil {
		t.Fatal("request decoder accepted trailing bytes")
	}
	unknownPredecessor := bytes.Replace(
		canonical,
		[]byte("genesis"),
		[]byte("unknown"),
		1,
	)
	if _, err := DecodeProjectTypeEnvHeadSelectionRequest(unknownPredecessor); err == nil {
		t.Fatal("request decoder accepted an unknown predecessor variant")
	}
	forged := mustHeadSelectionRequestRef(t, "f")
	if _, err := VerifyProjectTypeEnvHeadSelectionRequest(forged, canonical); err == nil {
		t.Fatal("request verifier accepted forged request ref")
	}

	otherInput := stageGenesisInput(t, 18)
	otherInput.Predecessor = NewGenesisStagePredecessor()
	otherStage := sealStageFixture(t, otherInput)
	if err := VerifyProjectTypeEnvHeadSelectionRequestAgainstStage(request, otherStage); err == nil {
		t.Fatal("request verifier accepted substituted Stage")
	}
	if err := VerifyProjectTypeEnvHeadSelectionRequestAgainstStage(request, stage); err != nil {
		t.Fatalf("original Stage stopped matching: %v", err)
	}
}

func TestHeadSelectionTransitionRevisionOverflowFailsClosed(t *testing.T) {
	input := stageTransitionInput(t, 29)
	predecessor := input.Predecessor.(TransitionStagePredecessor)
	overflowRevision := mustHeadRevision(t, math.MaxUint64)
	overflowPredecessor, err := NewTransitionStagePredecessor(
		TransitionStagePredecessorInput{
			Project:           predecessor.Project(),
			Head:              predecessor.Head(),
			HeadRevision:      overflowRevision,
			SelectedComposite: predecessor.SelectedComposite(),
		},
	)
	if err != nil {
		t.Fatalf("NewTransitionStagePredecessor(): %v", err)
	}
	input.Predecessor = overflowPredecessor
	stage := sealStageFixture(t, input)
	prior := sealHeadStateFixture(
		t,
		overflowPredecessor.Project(),
		overflowPredecessor.SelectedComposite(),
		overflowRevision,
	)
	request, err := SealTransitionProjectTypeEnvHeadSelectionRequest(
		TransitionProjectTypeEnvHeadSelectionRequestInput{
			Project:               stage.Project(),
			ExactPriorHead:        prior,
			Stage:                 stage,
			ExpectedGraphRevision: stage.GraphRevision(),
			IdempotencyKey:        mustHeadSelectionKey(t, "head-selection-overflow"),
		},
	)
	if err != nil {
		t.Fatalf("SealTransitionProjectTypeEnvHeadSelectionRequest(): %v", err)
	}
	_, err = DeriveTransitionProjectTypeEnvHeadSuccessorCandidate(
		request,
		prior,
		stage,
	)
	if !errors.Is(err, ErrProjectTypeEnvHeadRevisionOverflow) {
		t.Fatalf("overflow error = %v; want ErrProjectTypeEnvHeadRevisionOverflow", err)
	}
}

func TestProjectTypeEnvHeadStateStrictCanonicalCodec(t *testing.T) {
	project := testProjectID(t)
	state := sealHeadStateFixture(
		t,
		project,
		testTypeEnvRef(t, "a"),
		mustHeadRevision(t, 7),
	)
	if err := state.Verify(); err != nil {
		t.Fatalf("Verify(): %v", err)
	}
	decoded, err := DecodeProjectTypeEnvHeadState(state.CanonicalBytes())
	if err != nil {
		t.Fatalf("DecodeProjectTypeEnvHeadState(): %v", err)
	}
	if decoded.Ref() != mustHeadRef(t, project) || decoded.Revision().Value() != 7 {
		t.Fatal("head-state roundtrip lost stable ref or HeadRevision")
	}
	stateBytes := state.CanonicalBytes()
	stateBytes[0] ^= 0xff
	if err := state.Verify(); err != nil {
		t.Fatalf("caller mutation changed stored head state: %v", err)
	}
	if _, err := NewHeadRevision(0); err == nil {
		t.Fatal("HeadRevision admitted a zero Genesis sentinel")
	}
	if _, err := DecodeProjectTypeEnvHeadState(append(state.CanonicalBytes(), 0x00)); err == nil {
		t.Fatal("head-state decoder accepted trailing bytes")
	}
}

func TestHeadSelectionIdempotencyKeyIsCanonicalAndBounded(t *testing.T) {
	invalid := []string{
		"",
		" leading",
		"trailing ",
		"control\nkey",
		strings.Repeat("x", maximumHeadSelectionKeyBytes+1),
	}
	for _, raw := range invalid {
		if _, err := NewProjectTypeEnvHeadSelectionIdempotencyKey(raw); err == nil {
			t.Fatalf("idempotency key admitted %q", raw)
		}
	}
}

func genesisHeadSelectionFixture(
	t *testing.T,
	revision uint64,
	keyText string,
) (
	NoPriorHeadProofRecord,
	ProjectTypeEnvStage,
	ProjectTypeEnvHeadSelectionRequest,
) {
	t.Helper()
	input := stageGenesisInput(t, revision)
	proof := sealNoPriorHeadProofFixture(t, input)
	input.Predecessor = NewGenesisStagePredecessor()
	stage := sealStageFixture(t, input)
	request, err := SealGenesisProjectTypeEnvHeadSelectionRequest(
		GenesisProjectTypeEnvHeadSelectionRequestInput{
			Project:               input.Project,
			Stage:                 stage,
			ExpectedGraphRevision: input.GraphRevision,
			IdempotencyKey:        mustHeadSelectionKey(t, keyText),
		},
	)
	if err != nil {
		t.Fatalf("SealGenesisProjectTypeEnvHeadSelectionRequest(): %v", err)
	}
	return proof, stage, request
}

func sealNoPriorHeadProofFixture(
	t *testing.T,
	input ProjectTypeEnvStageInput,
) NoPriorHeadProofRecord {
	t.Helper()
	proof, err := sealNoPriorHeadProof(noPriorHeadProofInput{
		Project:               input.Project,
		Head:                  mustHeadRef(t, input.Project),
		GraphSnapshot:         input.GraphSnapshotBasis,
		ExpectedGraphRevision: input.GraphRevision,
	})
	if err != nil {
		t.Fatalf("sealNoPriorHeadProof(): %v", err)
	}
	return proof
}

func sealHeadStateFixture(
	t *testing.T,
	project projectidentity.ProjectID,
	composite typedmemory.TypeEnvRef,
	revision HeadRevision,
) ProjectTypeEnvHeadState {
	t.Helper()
	state, err := SealProjectTypeEnvHeadState(ProjectTypeEnvHeadStateInput{
		Project:           project,
		SelectedComposite: composite,
		Revision:          revision,
	})
	if err != nil {
		t.Fatalf("SealProjectTypeEnvHeadState(): %v", err)
	}
	return state
}

func mustHeadSelectionKey(
	t *testing.T,
	raw string,
) ProjectTypeEnvHeadSelectionIdempotencyKey {
	t.Helper()
	key, err := NewProjectTypeEnvHeadSelectionIdempotencyKey(raw)
	if err != nil {
		t.Fatalf("NewProjectTypeEnvHeadSelectionIdempotencyKey(): %v", err)
	}
	return key
}

func mustHeadSelectionRequestRef(
	t *testing.T,
	digit string,
) ProjectTypeEnvHeadSelectionRequestRef {
	t.Helper()
	ref, err := ParseProjectTypeEnvHeadSelectionRequestRef(
		projectTypeEnvHeadSelectionRequestPrefix + "sha256:" + strings.Repeat(digit, 64),
	)
	if err != nil {
		t.Fatalf("ParseProjectTypeEnvHeadSelectionRequestRef(): %v", err)
	}
	return ref
}
