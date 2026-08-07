package sqlite

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projectmemory"
	"github.com/m0n0x41d/haft/internal/projectmemory/carrierfamily"
	"github.com/m0n0x41d/haft/internal/projectmemory/codeanchoradapter"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselectioneffect"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorycandidatecodec"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
	"github.com/m0n0x41d/haft/internal/typedmemoryvalidation"
	"github.com/m0n0x41d/haft/internal/typedmemorywire"
)

func TestProductionCodeAnchorAdapterAdmitsExplicitClaimLinkWithoutInference(
	t *testing.T,
) {
	fixture := newProductionNoteSelectedFixture(t)
	ctx := context.Background()
	selection, err := fixture.service.SelectGenesis(
		ctx,
		genesisSelectionInput(fixture),
	)
	mustProductionNoteNoError(t, err)
	if _, ok := selection.(projecttypeenvselectioneffect.FreshlyCommitted); !ok {
		t.Fatalf(
			"production TypeEnv selection = %T, want FreshlyCommitted",
			selection,
		)
	}
	resolver := genesisE2EProjectRuntimeResolver(t, fixture)
	baseLoader, err :=
		typedmemorystore.NewProjectAwareSQLiteCurrentProjectSnapshotLoader(
			fixture.database,
			projectmemory.NewBaseTypeEnvLoader(),
			resolver,
		)
	mustProductionNoteNoError(t, err)
	clock := &genesisE2EClock{
		value: time.Date(2026, 7, 18, 13, 0, 0, 0, time.UTC),
	}
	contextRef, err := typedmemory.NewBoundedContextRef("haft-project")
	mustProductionNoteNoError(t, err)
	concern := productionNoteConcernDeclaration(t, contextRef)
	admitProductionPortfolioConcern(
		t,
		ctx,
		fixture,
		resolver,
		baseLoader,
		clock,
		concern,
	)

	current := loadProductionPortfolioSnapshot(
		t,
		ctx,
		baseLoader,
		fixture.project,
	)
	claim := productionProjectClaimAtConcernCandidate(
		t,
		fixture.project,
		current,
		concern,
		contextRef,
	)
	admitProductionCarrierCandidate(
		t,
		ctx,
		fixture,
		resolver,
		baseLoader,
		clock,
		claim.changeSet,
		claim.stage,
		"production-code-anchor-target-claim",
	)

	current = loadProductionPortfolioSnapshot(
		t,
		ctx,
		baseLoader,
		fixture.project,
	)
	claimBinding := productionCodeAnchorClaimBinding(
		t,
		current,
		claim.entity,
		contextRef,
	)
	draft := productionCodeAnchorDraft(
		t,
		fixture.project,
		contextRef,
		claimBinding,
	)
	result := codeanchoradapter.Adapt(
		draft,
		productionCodeAnchorExactRuntime(t, fixture, current),
	)
	candidate, ok := result.(codeanchoradapter.ValidCandidate)
	if !ok {
		t.Fatalf("CodeAnchor adapter result = %T, want ValidCandidate", result)
	}
	assertProductionCodeAnchorCandidate(t, candidate)
	receipt := admitProductionCodeAnchorCandidate(
		t,
		ctx,
		fixture,
		resolver,
		baseLoader,
		clock,
		candidate,
	)
	assertProductionCodeAnchorReread(
		t,
		ctx,
		fixture,
		receipt.GraphRevision(),
	)
}

type productionCarrierFamilySourceStage struct {
	project    projectidentity.ProjectID
	observable typedmemory.MemberOfObservableInput
	blob       typedmemorystore.ObservableInputBlob
}

var _ typedmemorystore.ObservableInputContentProvider = productionCarrierFamilySourceStage{}
var _ typedmemorystore.SnapshotObservableInputOverlay = productionCarrierFamilySourceStage{}

func newProductionCarrierFamilySourceStage(
	t *testing.T,
	source carrierfamily.MembershipSourceV1,
) productionCarrierFamilySourceStage {
	t.Helper()
	observable := source.ObservableInput()
	blob, err := typedmemorystore.NewObservableInputBlob(
		observable.Reference(),
		observable.Digest(),
		source.CanonicalBytes(),
	)
	mustProductionNoteNoError(t, err)
	return productionCarrierFamilySourceStage{
		project:    source.ProjectID(),
		observable: observable,
		blob:       blob,
	}
}

func (stage productionCarrierFamilySourceStage) LoadObservableInput(
	ctx context.Context,
	project projectidentity.ProjectID,
	reference typedmemory.ObservableInputRef,
	digest typedmemory.SHA256Digest,
) (typedmemorystore.ObservableInputBlob, error) {
	if err := ctx.Err(); err != nil {
		return typedmemorystore.ObservableInputBlob{}, err
	}
	if project != stage.project ||
		reference != stage.observable.Reference() ||
		digest != stage.observable.Digest() {
		return typedmemorystore.ObservableInputBlob{}, fmt.Errorf(
			"production carrier-family source is unavailable",
		)
	}
	return typedmemorystore.NewObservableInputBlob(
		stage.blob.Reference(),
		stage.blob.Digest(),
		stage.blob.Bytes(),
	)
}

func (stage productionCarrierFamilySourceStage) LoadSnapshotObservableInputs(
	ctx context.Context,
	project projectidentity.ProjectID,
) ([]typedmemorystore.ObservableInputBlob, error) {
	blob, err := stage.LoadObservableInput(
		ctx,
		project,
		stage.observable.Reference(),
		stage.observable.Digest(),
	)
	if err != nil {
		return nil, err
	}
	return []typedmemorystore.ObservableInputBlob{blob}, nil
}

type productionProjectClaimCandidate struct {
	entity    typedmemory.EntityID
	changeSet typedmemory.MemoryChangeSet
	stage     productionCarrierFamilySourceStage
	source    carrierfamily.MembershipSourceV1
}

func productionProjectClaimAtConcernCandidate(
	t *testing.T,
	project projectidentity.ProjectID,
	current typedmemorystore.CurrentProjectSnapshot,
	concern typedmemory.DeclareEntity,
	contextRef typedmemory.BoundedContextRef,
) productionProjectClaimCandidate {
	t.Helper()
	entity, err := typedmemory.NewEntityID("claim:production-code-anchor-target")
	mustProductionNoteNoError(t, err)
	local, err := typedmemory.NewBatchLocalRef(
		"claim:production-code-anchor-target",
	)
	mustProductionNoteNoError(t, err)
	label, err := typedmemory.NewEntityLabel(
		"Claim explicitly realized by production CodeAnchor",
	)
	mustProductionNoteNoError(t, err)
	provenance, err := typedmemory.NewProvenanceRef(
		"memory:test:production-code-anchor-target-claim",
	)
	mustProductionNoteNoError(t, err)
	declaration, err := typedmemory.NewDeclareEntity(
		entity,
		local,
		contextRef,
		label,
		provenance,
	)
	mustProductionNoteNoError(t, err)
	contextSlice := productionCodeAnchorContextSlice(t, contextRef)
	_, graphCandidate := productionProjectClaimGraph(
		t,
		current,
		"CodeAnchor relation is supplied explicitly by the caller",
	)
	relation := productionProjectClaimAtConcernRelation(
		t,
		current,
		declaration,
		concern,
		contextSlice,
		graphCandidate,
		provenance,
	)
	changeSet, err := typedmemory.NewMemoryChangeSet(
		[]typedmemory.MemoryChange{declaration, relation},
	)
	mustProductionNoteNoError(t, err)
	source := productionProjectClaimSource(
		t,
		project,
		entity,
		contextRef,
		graphCandidate.InputBytes(),
	)
	return productionProjectClaimCandidate{
		entity:    entity,
		changeSet: changeSet,
		stage:     newProductionCarrierFamilySourceStage(t, source),
		source:    source,
	}
}

func productionProjectClaimGraph(
	t *testing.T,
	current typedmemorystore.CurrentProjectSnapshot,
	text string,
) (typedmemory.ClaimGraphValue, typedmemory.TypedValueCandidate) {
	t.Helper()
	textKindID, err := typedmemory.NewKindID("Haft.Text")
	mustProductionNoteNoError(t, err)
	textKind, err := typedmemory.NewValueKindRef(
		current.Environment().Ref(),
		textKindID,
	)
	mustProductionNoteNoError(t, err)
	nodeID, err := typedmemory.NewClaimNodeID(
		"claim:production-code-anchor-explicit-link",
	)
	mustProductionNoteNoError(t, err)
	node, err := typedmemory.NewClaimNode(
		nodeID,
		textKind,
		typedmemory.NewTextValue(text),
	)
	mustProductionNoteNoError(t, err)
	graph, err := typedmemory.NewClaimGraphValue(
		[]typedmemory.ClaimNode{node},
		nil,
	)
	mustProductionNoteNoError(t, err)
	kindID, err := typedmemory.NewKindID("U.ClaimGraph")
	mustProductionNoteNoError(t, err)
	kind, err := typedmemory.NewValueKindRef(
		current.Environment().Ref(),
		kindID,
	)
	mustProductionNoteNoError(t, err)
	binding, found := current.Environment().ValueBinding(kind)
	if !found {
		t.Fatal("selected TypeEnv has no U.ClaimGraph value binding")
	}
	core, err := typedmemory.NewClaimGraphCodecV1(binding.ValueShape())
	mustProductionNoteNoError(t, err)
	encoded := core.EncodeInput(graph)
	canonical, ok := encoded.(typedmemory.CanonicalizedCodecValue)
	if !ok {
		t.Fatalf("ClaimGraph codec result = %T, want canonical value", encoded)
	}
	selected, found := current.Codecs().Resolve(binding.Codec())
	if !found {
		t.Fatal("selected runtime has no U.ClaimGraph codec")
	}
	replayed := selected.Canonicalize(
		binding.ValueShape(),
		canonical.CanonicalBytes(),
	)
	exact, ok := replayed.(typedmemory.CanonicalizedCodecValue)
	if !ok {
		t.Fatalf("selected ClaimGraph replay = %T, want canonical value", replayed)
	}
	candidate, err := typedmemory.NewTypedValueCandidate(
		kind,
		binding.ValueShape(),
		binding.Codec(),
		exact.CanonicalBytes(),
		typedmemory.NoAssertedDigest{},
	)
	mustProductionNoteNoError(t, err)
	return graph, candidate
}

func productionProjectClaimAtConcernRelation(
	t *testing.T,
	current typedmemorystore.CurrentProjectSnapshot,
	claim typedmemory.DeclareEntity,
	concern typedmemory.DeclareEntity,
	contextSlice typedmemory.ContextSlice,
	graph typedmemory.TypedValueCandidate,
	provenance typedmemory.ProvenanceRef,
) typedmemory.AssertRelation {
	t.Helper()
	signatureID, err := typedmemory.NewSignatureID(
		"Haft.ProjectClaimAtConcern",
	)
	mustProductionNoteNoError(t, err)
	signatureRef, err := typedmemory.NewRelationSignatureRef(
		current.Environment().Ref(),
		signatureID,
	)
	mustProductionNoteNoError(t, err)
	signature, found := current.Environment().RelationSignature(signatureRef)
	if !found {
		t.Fatal("selected TypeEnv has no Haft.ProjectClaimAtConcern")
	}
	claimReference := productionLocalReference(
		t,
		current.Environment(),
		"Haft.ProjectClaimRef",
		claim.LocalRef(),
	)
	concernBinding := productionNoteConcernBinding(
		t,
		current,
		concern.Entity(),
		contextSlice.Context(),
	)
	bindings := []typedmemory.CandidateSlotBinding{
		productionReferenceBinding(
			t,
			"Haft.ProjectClaimAtConcern.ProjectClaimSlot",
			claimReference,
		),
		productionReferenceBinding(
			t,
			"Haft.ProjectClaimAtConcern.EntityOfConcernSlot",
			concernBinding.Reference(),
		),
		productionValueBinding(
			t,
			"Haft.ProjectClaimAtConcern.ClaimGraphSlot",
			graph,
		),
	}
	assertion, err := typedmemory.NewAssertionID(
		"assertion:production-project-claim-at-concern",
	)
	mustProductionNoteNoError(t, err)
	modality := typedmemory.NewAffirmsObtaining()
	relation, err := typedmemory.NewRelationalAssertionCandidate(
		typedmemory.RelationalAssertionCandidateInput{
			Assertion:  assertion,
			Signature:  signature.Ref(),
			Slice:      contextSlice,
			Modality:   modality,
			Bindings:   bindings,
			Provenance: provenance,
		},
	)
	mustProductionNoteNoError(t, err)
	change, err := typedmemory.NewAssertRelation(relation)
	mustProductionNoteNoError(t, err)
	return change
}

func productionLocalReference(
	t *testing.T,
	environment typedmemory.TypeEnv,
	refKind string,
	local typedmemory.BatchLocalRef,
) typedmemory.LocalRef {
	t.Helper()
	refKindID, err := typedmemory.NewRefKindID(refKind)
	mustProductionNoteNoError(t, err)
	ref, err := typedmemory.NewRefKindRef(environment.Ref(), refKindID)
	mustProductionNoteNoError(t, err)
	value, err := typedmemory.NewLocalRef(ref, local)
	mustProductionNoteNoError(t, err)
	return value
}

func productionReferenceBinding(
	t *testing.T,
	slot string,
	reference typedmemory.StrongRef,
) typedmemory.CandidateSlotBinding {
	t.Helper()
	slotID, err := typedmemory.NewSlotKindID(slot)
	mustProductionNoteNoError(t, err)
	filler, err := typedmemory.NewByReferenceCandidate(reference)
	mustProductionNoteNoError(t, err)
	binding, err := typedmemory.NewCandidateSlotBinding(
		slotID,
		[]typedmemory.CandidateSlotFiller{filler},
	)
	mustProductionNoteNoError(t, err)
	return binding
}

func productionValueBinding(
	t *testing.T,
	slot string,
	value typedmemory.TypedValueCandidate,
) typedmemory.CandidateSlotBinding {
	t.Helper()
	slotID, err := typedmemory.NewSlotKindID(slot)
	mustProductionNoteNoError(t, err)
	filler, err := typedmemory.NewByValueCandidate(value)
	mustProductionNoteNoError(t, err)
	binding, err := typedmemory.NewCandidateSlotBinding(
		slotID,
		[]typedmemory.CandidateSlotFiller{filler},
	)
	mustProductionNoteNoError(t, err)
	return binding
}

func productionProjectClaimSource(
	t *testing.T,
	project projectidentity.ProjectID,
	entity typedmemory.EntityID,
	contextRef typedmemory.BoundedContextRef,
	canonical []byte,
) carrierfamily.MembershipSourceV1 {
	t.Helper()
	digest := productionSHA256Digest(t, canonical)
	ref, err := typedmemory.NewCarrierRef(
		"project-claim-payload:" + digest.String(),
	)
	mustProductionNoteNoError(t, err)
	edition, err := typedmemory.NewCarrierEdition("1.0.0")
	mustProductionNoteNoError(t, err)
	payload, err := carrierfamily.NewSourcePayloadV1(
		ref,
		edition,
		digest,
		"haft.project-claim-payload/v1",
		canonical,
	)
	mustProductionNoteNoError(t, err)
	carrier, err := carrierfamily.SealProjectClaimCarrierV1(
		entity,
		contextRef,
		payload,
	)
	mustProductionNoteNoError(t, err)
	manifest, err := carrierfamily.CurrentProjectClaimMappingManifestV1()
	mustProductionNoteNoError(t, err)
	binding, err := carrierfamily.SealEntityCarrierBindingV1(
		project,
		carrier,
		manifest.Ref(),
		manifest.AdapterVersion(),
	)
	mustProductionNoteNoError(t, err)
	source, err := carrierfamily.SealMembershipSourceV1(
		project,
		entity,
		contextRef,
		carrier,
		binding,
	)
	mustProductionNoteNoError(t, err)
	return source
}

func productionSHA256Digest(
	t *testing.T,
	canonical []byte,
) typedmemory.SHA256Digest {
	t.Helper()
	sum := sha256.Sum256(canonical)
	digest, err := typedmemory.NewSHA256Digest(
		"sha256:" + hex.EncodeToString(sum[:]),
	)
	mustProductionNoteNoError(t, err)
	return digest
}

func admitProductionCarrierCandidate(
	t *testing.T,
	ctx context.Context,
	fixture genesisE2EFixture,
	resolver typedmemorystore.SelectedProjectTypeEnvRuntimeResolver,
	baseLoader typedmemorystore.CurrentProjectSnapshotLoader,
	clock typedmemorystore.Clock,
	changeSet typedmemory.MemoryChangeSet,
	stage productionCarrierFamilySourceStage,
	token string,
) typedmemorystore.CommitReceipt {
	t.Helper()
	overlay, err :=
		typedmemorystore.NewCurrentProjectSnapshotLoaderWithObservableInputOverlay(
			baseLoader,
			stage,
		)
	mustProductionNoteNoError(t, err)
	source := newGenesisE2ECurrentProjectBasisSource(t, overlay)
	adapter := newProductionNoteCommitAdapter(
		t,
		fixture,
		resolver,
		clock,
		stage,
	)
	validation, err := projectmemory.NewValidationRuntime(
		fixture.project,
		source,
	)
	mustProductionNoteNoError(t, err)
	outcome, err := validation.EvaluateCandidate(
		ctx,
		typedmemorywire.ProjectCurrentSelector{},
		changeSet,
	)
	mustProductionNoteNoError(t, err)
	valid, ok := outcome.(typedmemoryvalidation.ValidOutcome)
	if !ok {
		t.Fatalf(
			"%s validation = %T/%s diagnostics=%#v",
			token,
			outcome,
			outcome.Verdict(),
			outcome.Diagnostics(),
		)
	}
	runtime, err := projectmemory.NewAdmissionRuntime(
		fixture.project,
		source,
		adapter,
	)
	mustProductionNoteNoError(t, err)
	key, err := typedmemorystore.NewIdempotencyKey(token)
	mustProductionNoteNoError(t, err)
	provenance, err := typedmemory.NewProvenanceRef("memory:test:" + token)
	mustProductionNoteNoError(t, err)
	receipt, err := runtime.AdmitValidated(
		ctx,
		valid,
		key,
		provenance,
	)
	mustProductionNoteNoError(t, err)
	if receipt.Disposition() != typedmemorystore.CommitApplied {
		t.Fatalf(
			"%s disposition = %s, want applied",
			token,
			receipt.Disposition(),
		)
	}
	return receipt
}

func productionCodeAnchorClaimBinding(
	t *testing.T,
	current typedmemorystore.CurrentProjectSnapshot,
	entity typedmemory.EntityID,
	contextRef typedmemory.BoundedContextRef,
) codeanchoradapter.ExactReferenceBinding {
	t.Helper()
	refKindID, err := typedmemory.NewRefKindID("Haft.ProjectClaimRef")
	mustProductionNoteNoError(t, err)
	refKind, err := typedmemory.NewRefKindRef(
		current.Environment().Ref(),
		refKindID,
	)
	mustProductionNoteNoError(t, err)
	referenceID, err := typedmemory.NewReferenceID(entity.String())
	mustProductionNoteNoError(t, err)
	reference, err := typedmemory.NewPersistedRef(refKind, referenceID)
	mustProductionNoteNoError(t, err)
	resolution := current.Snapshot().ResolveReference(reference, contextRef)
	resolved, ok := resolution.(typedmemory.ResolvedStrongReference)
	if !ok {
		t.Fatalf(
			"ProjectClaim target resolution = %T, want ResolvedStrongReference",
			resolution,
		)
	}
	binding, err := codeanchoradapter.NewExactReferenceBinding(resolved)
	mustProductionNoteNoError(t, err)
	return binding
}

func productionCodeAnchorDraft(
	t *testing.T,
	project projectidentity.ProjectID,
	contextRef typedmemory.BoundedContextRef,
	claim codeanchoradapter.ExactReferenceBinding,
) codeanchoradapter.Draft {
	t.Helper()
	target, err := typedmemorycandidatecodec.NewSymbolCodeAnchorTarget(
		"internal/projectmemory/codeanchoradapter/adapter.go",
		"Adapt",
	)
	mustProductionNoteNoError(t, err)
	locator, err := typedmemorycandidatecodec.NewCodeAnchorLocator(
		"github.com/m0n0x41d/haft",
		"0f9c64ef",
		target,
	)
	mustProductionNoteNoError(t, err)
	linkAssertion, err := typedmemory.NewAssertionID(
		"assertion:production-code-anchor-realizes-claim",
	)
	mustProductionNoteNoError(t, err)
	link, err := codeanchoradapter.NewClaimLink(linkAssertion, claim)
	mustProductionNoteNoError(t, err)
	entity, err := typedmemory.NewEntityID(
		"code-anchor:production-code-anchor-adapt",
	)
	mustProductionNoteNoError(t, err)
	local, err := typedmemory.NewBatchLocalRef(
		"code-anchor:production-code-anchor-adapt",
	)
	mustProductionNoteNoError(t, err)
	label, err := typedmemory.NewEntityLabel(
		"Adapt symbol at exact repository revision",
	)
	mustProductionNoteNoError(t, err)
	definition, err := typedmemory.NewAssertionID(
		"assertion:production-code-anchor-definition",
	)
	mustProductionNoteNoError(t, err)
	provenance, err := typedmemory.NewProvenanceRef(
		"memory:test:production-code-anchor",
	)
	mustProductionNoteNoError(t, err)
	draft, err := codeanchoradapter.NewDraft(codeanchoradapter.DraftInput{
		ProjectID:             project,
		AnchorEntity:          entity,
		AnchorLocalRef:        local,
		AnchorLabel:           label,
		DefinitionAssertionID: definition,
		ContextSlice:          productionCodeAnchorContextSlice(t, contextRef),
		Locator:               codeanchoradapter.NewExactLocator(locator),
		Links:                 []codeanchoradapter.SemanticLink{link},
		Provenance:            provenance,
	})
	mustProductionNoteNoError(t, err)
	return draft
}

func productionCodeAnchorContextSlice(
	t *testing.T,
	contextRef typedmemory.BoundedContextRef,
) typedmemory.ContextSlice {
	t.Helper()
	gamma, err := typedmemory.NewGammaPoint(
		time.Date(2026, 7, 18, 13, 5, 0, 0, time.UTC),
	)
	mustProductionNoteNoError(t, err)
	slice, err := typedmemory.NewContextSlice(
		typedmemory.ContextSliceInput{
			Context:   contextRef,
			GammaTime: gamma,
		},
	)
	mustProductionNoteNoError(t, err)
	return slice
}

func productionCodeAnchorExactRuntime(
	t *testing.T,
	fixture genesisE2EFixture,
	current typedmemorystore.CurrentProjectSnapshot,
) codeanchoradapter.ExactRuntimeBasis {
	t.Helper()
	runtimeDigest, err := typedmemorystore.NewSelectedRuntimeBasisDigest(
		fixture.target.runtime.Digest(),
	)
	mustProductionNoteNoError(t, err)
	coordinate, found := fixture.target.registry.CoordinateDigest()
	if !found {
		t.Fatal("production CodeAnchor runtime has no exact registry coordinate")
	}
	registryDigest, err :=
		typedmemorystore.NewExactTargetRegistryCoordinateDigest(coordinate)
	mustProductionNoteNoError(t, err)
	registration := productionPolicyForRule(
		t,
		fixture.target.installed.RegistrationPolicies,
		carrierfamily.CodeAnchorEvaluatorRuleV1(),
	)
	runtime, err := codeanchoradapter.NewExactRuntimeBasisBuilder(
		fixture.project,
	).
		SetGraphRevision(current.Snapshot().GraphRevision()).
		SetEnvironment(current.Environment()).
		SetCodecs(current.Codecs()).
		SetSelectedRuntimeCoordinates(runtimeDigest, registryDigest).
		SetRegistrationPolicy(registration).
		Build()
	mustProductionNoteNoError(t, err)
	return runtime
}

func assertProductionCodeAnchorCandidate(
	t *testing.T,
	candidate codeanchoradapter.ValidCandidate,
) {
	t.Helper()
	changes := candidate.ChangeSet().Changes()
	if len(changes) != 3 {
		t.Fatalf(
			"CodeAnchor changes = %d, want declaration + definition + explicit claim link",
			len(changes),
		)
	}
	counts := map[string]int{}
	for _, change := range changes {
		relation, ok := change.(typedmemory.AssertRelation)
		if !ok {
			continue
		}
		counts[relation.Assertion().Signature().ID().String()]++
	}
	if counts["Haft.CodeAnchorDefinition"] != 1 ||
		counts["Haft.CodeRealizesClaim"] != 1 ||
		counts["Haft.CodeChangedByWork"] != 0 {
		t.Fatalf(
			"CodeAnchor relation counts = %#v, want definition + explicit claim only",
			counts,
		)
	}
}

func admitProductionCodeAnchorCandidate(
	t *testing.T,
	ctx context.Context,
	fixture genesisE2EFixture,
	resolver typedmemorystore.SelectedProjectTypeEnvRuntimeResolver,
	baseLoader typedmemorystore.CurrentProjectSnapshotLoader,
	clock typedmemorystore.Clock,
	candidate codeanchoradapter.ValidCandidate,
) typedmemorystore.CommitReceipt {
	t.Helper()
	stage, err := codeanchoradapter.SealPreAdmissionSourceStage(candidate)
	mustProductionNoteNoError(t, err)
	overlay, err :=
		typedmemorystore.NewCurrentProjectSnapshotLoaderWithObservableInputOverlay(
			baseLoader,
			stage,
		)
	mustProductionNoteNoError(t, err)
	source := newGenesisE2ECurrentProjectBasisSource(t, overlay)
	adapter := newProductionNoteCommitAdapter(
		t,
		fixture,
		resolver,
		clock,
		stage,
	)
	validation, err := projectmemory.NewValidationRuntime(
		fixture.project,
		source,
	)
	mustProductionNoteNoError(t, err)
	outcome, err := validation.EvaluateCandidate(
		ctx,
		typedmemorywire.ProjectCurrentSelector{},
		candidate.ChangeSet(),
	)
	mustProductionNoteNoError(t, err)
	valid, ok := outcome.(typedmemoryvalidation.ValidOutcome)
	if !ok {
		t.Fatalf(
			"CodeAnchor validation = %T/%s diagnostics=%#v",
			outcome,
			outcome.Verdict(),
			outcome.Diagnostics(),
		)
	}
	runtime, err := projectmemory.NewAdmissionRuntime(
		fixture.project,
		source,
		adapter,
	)
	mustProductionNoteNoError(t, err)
	key, err := typedmemorystore.NewIdempotencyKey(
		"production-code-anchor-admission",
	)
	mustProductionNoteNoError(t, err)
	provenance, err := typedmemory.NewProvenanceRef(
		"memory:test:production-code-anchor-admission",
	)
	mustProductionNoteNoError(t, err)
	receipt, err := runtime.AdmitValidated(
		ctx,
		valid,
		key,
		provenance,
	)
	mustProductionNoteNoError(t, err)
	if receipt.Disposition() != typedmemorystore.CommitApplied {
		t.Fatalf(
			"CodeAnchor admission = %s, want applied",
			receipt.Disposition(),
		)
	}
	replay, err := runtime.AdmitValidated(
		ctx,
		valid,
		key,
		provenance,
	)
	mustProductionNoteNoError(t, err)
	if replay.Disposition() != typedmemorystore.CommitReplay ||
		replay.EventRef() != receipt.EventRef() ||
		replay.ResultDigest() != receipt.ResultDigest() {
		t.Fatal("CodeAnchor replay changed the exact committed result")
	}
	return receipt
}

func assertProductionCodeAnchorReread(
	t *testing.T,
	ctx context.Context,
	fixture genesisE2EFixture,
	revision typedmemory.GraphRevision,
) {
	t.Helper()
	transaction, err := sqlitetransaction.BeginRead(ctx, fixture.database)
	mustProductionNoteNoError(t, err)
	observation, err := typedmemorystore.LoadCurrentGraphRevalidationBasisTx(
		ctx,
		transaction,
		fixture.project,
	)
	mustProductionNoteNoError(t, err)
	finish := transaction.Commit(ctx)
	if !finish.Succeeded() {
		t.Fatalf("commit CodeAnchor durable read: %v", finish.Err())
	}
	if observation.GraphSnapshotBasis().GraphRevision() != revision {
		t.Fatal("CodeAnchor durable reread observed another graph revision")
	}
	counts := map[string]int{}
	for _, active := range observation.ActiveAssertions().Relations() {
		carrier := productionFreshCurrentAssertionCarrier(t, active)
		counts[carrier.Signature().ID().String()]++
	}
	if counts["Haft.ProjectClaimAtConcern"] != 1 ||
		counts["Haft.CodeAnchorDefinition"] != 1 ||
		counts["Haft.CodeRealizesClaim"] != 1 ||
		counts["Haft.CodeChangedByWork"] != 0 {
		t.Fatalf(
			"durable CodeAnchor relation counts = %#v, want explicit claim path only",
			counts,
		)
	}
}
