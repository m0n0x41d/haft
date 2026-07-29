package projecttypeenvselectionauthority

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv"
	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
	"github.com/m0n0x41d/haft/internal/fpf/typeenvsql"
	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projectprofile"
	"github.com/m0n0x41d/haft/internal/projecttypeenvassertionreport"
	"github.com/m0n0x41d/haft/internal/projecttypeenvcompatibility"
	"github.com/m0n0x41d/haft/internal/projecttypeenvprofilebasis"
	"github.com/m0n0x41d/haft/internal/projecttypeenvprofilecompatibility"
	"github.com/m0n0x41d/haft/internal/projecttypeenvprofilefit"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/runtimemechanism"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	_ "modernc.org/sqlite"
)

var (
	authorityStageTargetOnce sync.Once
	authorityStageTarget     authorityStageTargetFixture
	authorityStageTargetErr  error
)

type authorityStageTargetFixture struct {
	verification projecttypeenv.ProjectTypeEnvCompositeVerification
	snapshot     projecttypeenv.ProjectTypeEnvExecutableSnapshot
}

type authorityFixture struct {
	stage      projecttypeenvselection.ProjectTypeEnvStage
	request    projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest
	content    ProjectTypeEnvHeadSelectionAuthorizationContent
	source     authority.VerifiedSpeechActSourceV2
	contract   ProjectTypeEnvHeadSelectionSpeechActSourceContract
	adapter    ProjectTypeEnvHeadSelectionSourceAdapterPolicy
	binding    ProjectAuthorityContextBinding
	record     ProjectTypeEnvHeadSelectionSpeechActRecord
	policy     ProjectTypeEnvHeadSelectionResolverPolicy
	basis      ProjectTypeEnvHeadSelectionAuthorityResolutionBasis
	resolution StrictPermissionResolution
	evaluated  time.Time
}

func buildAuthorityFixture(t *testing.T) authorityFixture {
	t.Helper()
	stage, request := buildStageAndRequest(t, "selection-key-a")
	contextRef := mustAuthorityValue(t, authority.NewBoundedContextRef, "bounded-context:project-typeenv-head-selection")
	description := mustDescriptionRef(t, "claim:project-typeenv-head-selection:a")
	baseTime := time.Date(2026, 7, 17, 6, 0, 0, 0, time.UTC)
	validity, err := authority.NewTimeWindow(baseTime, baseTime.Add(2*time.Hour))
	if err != nil {
		t.Fatalf("NewTimeWindow(validity): %v", err)
	}
	content, err := SealProjectTypeEnvHeadSelectionAuthorizationContent(
		ProjectTypeEnvHeadSelectionAuthorizationContentInput{
			DescriptionRef:   description,
			Request:          request,
			Stage:            stage,
			JudgementContext: contextRef,
			ValidityWindow:   validity,
		},
	)
	if err != nil {
		t.Fatalf("SealProjectTypeEnvHeadSelectionAuthorizationContent: %v", err)
	}
	source := buildSpeechActSource(t, content, request, baseTime.Add(10*time.Minute), false)
	contract, err := NewProjectTypeEnvHeadSelectionSpeechActSourceContract(contextRef)
	if err != nil {
		t.Fatalf("NewProjectTypeEnvHeadSelectionSpeechActSourceContract: %v", err)
	}
	method, methodOK := source.MethodDescription()
	executedWithin, executedWithinOK := source.ExecutedWithin()
	contextPolicy, contextPolicyOK := source.ContextPolicy()
	if !methodOK || !executedWithinOK || !contextPolicyOK {
		t.Fatal("source adapter coordinates are unavailable")
	}
	adapter, err := SealProjectTypeEnvHeadSelectionSourceAdapterPolicy(
		method,
		executedWithin,
		contextPolicy,
	)
	if err != nil {
		t.Fatalf("SealProjectTypeEnvHeadSelectionSourceAdapterPolicy: %v", err)
	}
	root, rootOK := source.ProjectRoot()
	carriers, carriersOK := source.DescriptionCarriers()
	if !rootOK || !carriersOK || len(carriers) == 0 {
		t.Fatal("source project/carrier coordinates are unavailable")
	}
	binding, err := SealProjectAuthorityContextBinding(ProjectAuthorityContextBindingInput{
		Project: request.Project(),
		Root:    root,
		Context: contextRef,
		Carrier: carriers[0],
	})
	if err != nil {
		t.Fatalf("SealProjectAuthorityContextBinding: %v", err)
	}
	record, err := SealProjectTypeEnvHeadSelectionSpeechActRecord(
		ProjectTypeEnvHeadSelectionSpeechActRecordInput{
			Source:         source,
			SourceContract: contract,
			ProjectBinding: binding,
			Content:        content,
			Request:        request,
			Stage:          stage,
		},
	)
	if err != nil {
		t.Fatalf("SealProjectTypeEnvHeadSelectionSpeechActRecord: %v", err)
	}
	evaluated := baseTime.Add(20 * time.Minute)
	policyWindow, err := authority.NewTimeWindow(baseTime, baseTime.Add(time.Hour))
	if err != nil {
		t.Fatalf("NewTimeWindow(policy): %v", err)
	}
	policy, err := SealProjectTypeEnvHeadSelectionResolverPolicy(
		ProjectTypeEnvHeadSelectionResolverPolicyInput{
			Ref:             mustResolverPolicyRef(t, "policy:typeenv-head-selection:current"),
			Edition:         mustResolverPolicyEdition(t, "resolver-policy-edition:v1"),
			EffectiveWindow: policyWindow,
			Action:          content.Action(),
			SourceContract:  contract,
			SourceAdapter:   adapter,
			ProjectBinding:  binding,
		},
	)
	if err != nil {
		t.Fatalf("SealProjectTypeEnvHeadSelectionResolverPolicy: %v", err)
	}
	basis, err := SealProjectTypeEnvHeadSelectionAuthorityResolutionBasis(
		ProjectTypeEnvHeadSelectionAuthorityResolutionBasisInput{
			Policy:      policy,
			Record:      record,
			Content:     content,
			Request:     request,
			Stage:       stage,
			EvaluatedAt: evaluated,
		},
	)
	if err != nil {
		t.Fatalf("SealProjectTypeEnvHeadSelectionAuthorityResolutionBasis: %v", err)
	}
	resolution, err := SealStrictPermissionResolution(basis)
	if err != nil {
		t.Fatalf("SealStrictPermissionResolution: %v", err)
	}
	return authorityFixture{
		stage:      stage,
		request:    request,
		content:    content,
		source:     source,
		contract:   contract,
		adapter:    adapter,
		binding:    binding,
		record:     record,
		policy:     policy,
		basis:      basis,
		resolution: resolution,
		evaluated:  evaluated,
	}
}

func mustResolverPolicyRef(
	t *testing.T,
	raw string,
) ProjectTypeEnvHeadSelectionResolverPolicyRef {
	t.Helper()
	value, err := NewProjectTypeEnvHeadSelectionResolverPolicyRef(raw)
	if err != nil {
		t.Fatalf("NewProjectTypeEnvHeadSelectionResolverPolicyRef: %v", err)
	}
	return value
}

func mustResolverPolicyEdition(
	t *testing.T,
	raw string,
) ProjectTypeEnvHeadSelectionResolverPolicyEdition {
	t.Helper()
	value, err := NewProjectTypeEnvHeadSelectionResolverPolicyEdition(raw)
	if err != nil {
		t.Fatalf("NewProjectTypeEnvHeadSelectionResolverPolicyEdition: %v", err)
	}
	return value
}

func buildStageAndRequest(
	t *testing.T,
	idempotency string,
) (
	projecttypeenvselection.ProjectTypeEnvStage,
	projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest,
) {
	t.Helper()
	target := mustAuthorityStageTarget(t)
	verification := target.verification
	project := mustProjectID(t, "qnt_0123abcd")
	priorComposite := mustTypeEnvRef(t, "c")
	priorRevision, err := projecttypeenvselection.NewHeadRevision(4)
	if err != nil {
		t.Fatalf("NewHeadRevision: %v", err)
	}
	prior, err := projecttypeenvselection.SealProjectTypeEnvHeadState(
		projecttypeenvselection.ProjectTypeEnvHeadStateInput{
			Project:           project,
			SelectedComposite: priorComposite,
			Revision:          priorRevision,
		},
	)
	if err != nil {
		t.Fatalf("SealProjectTypeEnvHeadState: %v", err)
	}
	predecessor, err := prior.ExactPriorHead()
	if err != nil {
		t.Fatalf("ExactPriorHead: %v", err)
	}
	graphRevision := typedmemory.NewGraphRevision(17)
	snapshot := mustGraphSnapshot(t, project, graphRevision)
	previous := authorityTypeEnvAtRef(
		t,
		priorComposite,
		target.snapshot.Environment(),
	)
	diff, err := projecttypeenvcompatibility.Compare(
		previous,
		target.snapshot.Environment(),
	)
	if err != nil {
		t.Fatalf("projecttypeenvcompatibility.Compare: %v", err)
	}
	compatibility, err := projecttypeenvselection.NewComparedStageCompatibility(diff)
	if err != nil {
		t.Fatalf("NewComparedStageCompatibility: %v", err)
	}
	successor, err := projecttypeenvcompatibility.CompareSuccessor(
		previous,
		target.snapshot.Environment(),
	)
	if err != nil {
		t.Fatalf("projecttypeenvcompatibility.CompareSuccessor: %v", err)
	}
	transitionProfiles, err := projecttypeenvprofilecompatibility.AssessTransitionProjectionProfiles(
		successor,
	)
	if err != nil {
		t.Fatalf("AssessTransitionProjectionProfiles: %v", err)
	}
	graphRef, err := projecttypeenvassertionreport.ParseGraphSnapshotRef(
		snapshot.Ref().String(),
	)
	if err != nil {
		t.Fatalf("ParseGraphSnapshotRef: %v", err)
	}
	graphCoordinate, err := projecttypeenvassertionreport.NewGraphSnapshotCoordinate(
		graphRef,
		graphRevision,
		snapshot.Ref().Digest(),
	)
	if err != nil {
		t.Fatalf("NewGraphSnapshotCoordinate: %v", err)
	}
	revalidation, err := projecttypeenvassertionreport.NewReport(
		verification.CompositeRef(),
		graphCoordinate,
		verification.RuntimeEvaluationBasisRef(),
		verification.RuntimeEvaluationBasisRef().Digest(),
		nil,
	)
	if err != nil {
		t.Fatalf("projecttypeenvassertionreport.NewReport: %v", err)
	}
	profileRoot, err := projectprofile.NewProjectRootV1(
		"/tmp/haft-typeenv-head-selection-test",
	)
	if err != nil {
		t.Fatalf("NewProjectRootV1: %v", err)
	}
	profileBasis, err := projecttypeenvprofilebasis.NewNoCanonicalProjectProfile(
		profileRoot,
	)
	if err != nil {
		t.Fatalf("NewNoCanonicalProjectProfile: %v", err)
	}
	profile, err := projecttypeenvprofilefit.AssessProjectTypeEnvProfileFit(
		profileBasis,
		target.snapshot,
	)
	if err != nil {
		t.Fatalf("AssessProjectTypeEnvProfileFit: %v", err)
	}
	stage, err := projecttypeenvselection.SealProjectTypeEnvStage(
		projecttypeenvselection.ProjectTypeEnvStageInput{
			Project:                                  project,
			Predecessor:                              predecessor,
			Base:                                     verification.BaseTypeEnvRef(),
			OrderedExtensions:                        verification.ExtensionRefs(),
			RuntimeBasis:                             verification.RuntimeEvaluationBasisRef(),
			VerifiedComposite:                        verification,
			Composite:                                verification.CompositeRef(),
			GraphSnapshotBasis:                       snapshot,
			GraphSnapshotBasisRef:                    snapshot.Ref(),
			GraphSnapshotBasisDigest:                 snapshot.Ref().Digest(),
			GraphRevision:                            graphRevision,
			ProfileLedgerRevision:                    profileBasis.LedgerRevision(),
			ProfileLedgerDigest:                      profileBasis.ProfileLedgerDigest(),
			Compatibility:                            compatibility,
			ExistingAssertionRevalidation:            revalidation,
			ProfileCompatibility:                     profile,
			TransitionProjectionProfileCompatibility: transitionProfiles,
		},
	)
	if err != nil {
		t.Fatalf("SealProjectTypeEnvStage: %v", err)
	}
	key, err := projecttypeenvselection.NewProjectTypeEnvHeadSelectionIdempotencyKey(idempotency)
	if err != nil {
		t.Fatalf("NewProjectTypeEnvHeadSelectionIdempotencyKey: %v", err)
	}
	request, err := projecttypeenvselection.SealTransitionProjectTypeEnvHeadSelectionRequest(
		projecttypeenvselection.TransitionProjectTypeEnvHeadSelectionRequestInput{
			Project:               project,
			ExactPriorHead:        prior,
			Stage:                 stage,
			ExpectedGraphRevision: graphRevision,
			IdempotencyKey:        key,
		},
	)
	if err != nil {
		t.Fatalf("SealTransitionProjectTypeEnvHeadSelectionRequest: %v", err)
	}
	return stage, request
}

func buildSpeechActSource(
	t *testing.T,
	content ProjectTypeEnvHeadSelectionAuthorizationContent,
	request projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest,
	startedAt time.Time,
	omitLastAffected bool,
) authority.VerifiedSpeechActSourceV2 {
	t.Helper()
	variant := defaultSpeechActSourceVariant(t, content, request)
	variant.omitLastAffected = omitLastAffected
	return buildSpeechActSourceVariant(t, content, request, startedAt, variant)
}

type speechActSourceVariant struct {
	actType              string
	statePlane           string
	delta                string
	methodRef            string
	methodDescriptionRef string
	procedureRef         string
	executedWithin       string
	projectRoot          string
	institutedObject     string
	speechActRef         string
	institutedKind       string
	modality             string
	scopedAction         string
	omitLastAffected     bool
}

func defaultSpeechActSourceVariant(
	t *testing.T,
	content ProjectTypeEnvHeadSelectionAuthorizationContent,
	request projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest,
) speechActSourceVariant {
	t.Helper()
	if err := content.ExactAgainst(request); err != nil {
		t.Fatalf("default source content/request binding: %v", err)
	}
	speechActRaw := "speech-act:typeenv-head-selection:test"
	speechAct := mustAuthorityValue(
		t,
		authority.NewSpeechActRef,
		speechActRaw,
	)
	permissionRef, err := DeriveProjectTypeEnvHeadSelectionPermissionRef(content, speechAct)
	if err != nil {
		t.Fatalf("DeriveProjectTypeEnvHeadSelectionPermissionRef: %v", err)
	}
	action, err := content.Action().AuthorityActionKind()
	if err != nil {
		t.Fatalf("AuthorityActionKind: %v", err)
	}
	return speechActSourceVariant{
		actType:              typeEnvSpeechActTypeRefValue,
		statePlane:           typeEnvSpeechActStatePlaneValue,
		delta:                typeEnvSpeechActDeltaValue,
		methodRef:            "method:manual-typeenv-head-selection",
		methodDescriptionRef: "method-description:manual-typeenv-head-selection:v1",
		procedureRef:         "procedure:review-and-authorize-typeenv-head-selection:v1",
		executedWithin:       "system:haft-authority-test",
		projectRoot:          "/tmp/haft-typeenv-head-selection-test",
		institutedObject:     permissionRef.String(),
		speechActRef:         speechActRaw,
		institutedKind:       "U.Commitment",
		modality:             "MAY",
		scopedAction:         action.String(),
	}
}

func buildSpeechActSourceVariant(
	t *testing.T,
	content ProjectTypeEnvHeadSelectionAuthorizationContent,
	request projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest,
	startedAt time.Time,
	variant speechActSourceVariant,
) authority.VerifiedSpeechActSourceV2 {
	t.Helper()
	contextRef := content.JudgementContext()
	actType := mustAuthorityValue(t, authority.NewSpeechActTypeRef, variant.actType)
	actionKind, err := authority.NewActionKind(variant.scopedAction)
	if err != nil {
		t.Fatalf("AuthorityActionKind: %v", err)
	}
	effectRuleRef := mustAuthorityValue(t, authority.NewInstitutionalEffectRuleRef, "institution-rule:typeenv-head-selection:v1")
	objectKind, err := authority.NewInstitutedObjectKind(variant.institutedKind)
	if err != nil {
		t.Fatalf("NewInstitutedObjectKind: %v", err)
	}
	modality, err := authority.NewInstitutionalModality(variant.modality)
	if err != nil {
		t.Fatalf("NewInstitutionalModality: %v", err)
	}
	utterance := mustAuthorityValue(t, authority.NewUtteranceRef, "utterance:authorize-project-typeenv-head-selection")
	effectRule, err := authority.NewInstitutionalEffectRule(
		effectRuleRef,
		objectKind,
		modality,
		actionKind,
		authority.AuthorizeReviewedIntentUtteranceRule(),
		utterance,
	)
	if err != nil {
		t.Fatalf("NewInstitutionalEffectRule: %v", err)
	}
	policyRef := mustAuthorityValue(t, authority.NewContextPolicyRef, "policy:typeenv-head-selection:v1")
	policy, err := authority.NewSpeechActContextPolicy(policyRef, contextRef, actType, effectRule)
	if err != nil {
		t.Fatalf("NewSpeechActContextPolicy: %v", err)
	}
	methodRef := mustAuthorityValue(t, authority.NewMethodRef, variant.methodRef)
	methodDescriptionRef := mustAuthorityValue(t, authority.NewMethodDescriptionRef, variant.methodDescriptionRef)
	procedureRef := mustAuthorityValue(t, authority.NewMethodProcedureRef, variant.procedureRef)
	method, err := authority.NewManualControllingTTYMethodDescription(
		methodRef,
		methodDescriptionRef,
		procedureRef,
		contextRef,
	)
	if err != nil {
		t.Fatalf("NewManualControllingTTYMethodDescription: %v", err)
	}
	parameter, err := authority.NewWorkParameterBinding(
		"parameter:reviewed-authorization-content-digest",
		content.Digest().String(),
	)
	if err != nil {
		t.Fatalf("NewWorkParameterBinding: %v", err)
	}
	frameBuilder := authority.NewSpeechActExecutionFrameBuilder(method).
		ExecutedWithin(mustAuthorityValue(t, authority.NewSystemRef, variant.executedWithin)).
		OnStatePlane(
			mustAuthorityValue(t, authority.NewStatePlaneRef, variant.statePlane),
			mustAuthorityValue(t, authority.NewDeltaPredicateRef, variant.delta),
		).
		WithOutcome(mustAuthorityValue(t, authority.NewWorkOutcomeRef, "outcome:content-authorized")).
		WithUtteranceDescription(utterance).
		BindParameter(parameter).
		UseResource(mustAuthorityValue(t, authority.NewWorkResourceRef, "resource:controlling-terminal"))
	required, err := requiredAffectedReferents(request)
	if err != nil {
		t.Fatalf("requiredAffectedReferents: %v", err)
	}
	limit := len(required)
	if variant.omitLastAffected {
		limit--
	}
	for _, ref := range required[:limit] {
		frameBuilder = frameBuilder.Affect(ref)
	}
	frame, err := frameBuilder.Build()
	if err != nil {
		t.Fatalf("SpeechActExecutionFrame.Build: %v", err)
	}
	speechActRef := mustAuthorityValue(t, authority.NewSpeechActRef, variant.speechActRef)
	captureRef := mustAuthorityValue(t, authority.NewCarrierRef, "carrier:typeenv-head-selection:terminal-capture")
	root := mustAuthorityValue(t, authority.NewProjectRoot, variant.projectRoot)
	session := mustAuthorityValue(t, authority.NewSessionRef, "session:typeenv-head-selection:test")
	reviewSubject := mustAuthorityValue(t, authority.NewSpeechActReviewSubjectRef, content.DescriptionRef().String())
	instituted := mustAuthorityValue(t, authority.NewInstitutedObjectRef, variant.institutedObject)
	intent, err := authority.NewPreparedSpeechActIntentBuilder(speechActRef, captureRef).
		ForProject(root).
		InSession(session).
		Reviewing(reviewSubject, content.Digest()).
		Institutes(instituted).
		UnderContextPolicy(policy).
		WithExecutionFrame(frame).
		Build()
	if err != nil {
		t.Fatalf("PreparedSpeechActIntent.Build: %v", err)
	}
	prepared, err := authority.PrepareManualSpeechAct(
		intent,
		"Authorize the exact reviewed TypeEnv head-selection content.",
	)
	if err != nil {
		t.Fatalf("PrepareManualSpeechAct: %v", err)
	}
	basis, err := authority.CaptureVerifiedSpeechActForTestFixture(
		t,
		prepared,
		startedAt,
		startedAt.Add(time.Second),
		startedAt.Add(2*time.Second),
	)
	if err != nil {
		t.Fatalf("CaptureVerifiedSpeechActForTestFixture: %v", err)
	}
	work := mustAuthorityValue(t, authority.NewWorkRef, "work:typeenv-head-selection:speech-act:test")
	resource := mustAuthorityValue(t, authority.NewResourceLedgerRef, "resource-ledger:typeenv-head-selection:test")
	acceptance := mustAuthorityValue(t, authority.NewAcceptancePostureRef, "acceptance:typeenv-head-selection:recognized")
	audit := mustAuthorityValue(t, authority.NewAuditTraceRef, "audit:typeenv-head-selection:test")
	carrierRef := mustAuthorityValue(t, authority.NewCarrierRef, "carrier:typeenv-head-selection:reviewed-content")
	carrier, err := authority.NewObservableCarrierBinding(
		carrierRef,
		mustAuthorityDigest(t, "d"),
	)
	if err != nil {
		t.Fatalf("NewObservableCarrierBinding: %v", err)
	}
	anchors, err := authority.NewSpeechActSourceV2AnchorsBuilder(work, content.DescriptionRef()).
		WithResourceLedger(resource).
		WithAcceptancePosture(acceptance).
		WithAuditTrace(audit).
		WithDescriptionCarrier(carrier).
		Build()
	if err != nil {
		t.Fatalf("SpeechActSourceV2Anchors.Build: %v", err)
	}
	source, err := authority.NewVerifiedSpeechActSourceV2(basis, anchors)
	if err != nil {
		t.Fatalf("NewVerifiedSpeechActSourceV2: %v", err)
	}
	return source
}

func mustAuthorityStageTarget(t *testing.T) authorityStageTargetFixture {
	t.Helper()
	authorityStageTargetOnce.Do(func() {
		authorityStageTarget, authorityStageTargetErr = buildAuthorityStageTarget()
	})
	if authorityStageTargetErr != nil {
		t.Fatalf("build Stage target: %v", authorityStageTargetErr)
	}
	return authorityStageTarget
}

func buildAuthorityStageTarget() (authorityStageTargetFixture, error) {
	// Test debt: this package still reads the repository's generated fpf.db.
	// It is opened read-only/immutable, but a package-owned golden fixture is
	// needed to isolate these tests from concurrent CLI fixture regeneration.
	databasePath, err := filepath.Abs(filepath.Join("..", "cli", "fpf.db"))
	if err != nil {
		return authorityStageTargetFixture{}, err
	}
	database, err := sql.Open("sqlite", "file:"+filepath.ToSlash(databasePath)+"?mode=ro&immutable=1")
	if err != nil {
		return authorityStageTargetFixture{}, err
	}
	database.SetMaxOpenConns(1)
	defer func() { _ = database.Close() }()
	base, err := typeenvsql.LoadArtifactReadOnlyDB(context.Background(), database)
	if err != nil {
		return authorityStageTargetFixture{}, err
	}
	resolution := projecttypeenv.LinkProjectTypeEnvCompositeIR(base, nil)
	linked, exists := resolution.CompositeIR()
	if resolution.Rejected() || !exists {
		return authorityStageTargetFixture{}, fmt.Errorf("link base composite: %#v", resolution.Issues())
	}
	runtimeBasis, err := authorityRuntimeEvaluationBasis(base, linked)
	if err != nil {
		return authorityStageTargetFixture{}, err
	}
	composite, err := projecttypeenv.SealProjectTypeEnvComposite(linked, runtimeBasis)
	if err != nil {
		return authorityStageTargetFixture{}, err
	}
	preparation := projecttypeenv.PrepareProjectTypeEnvComposite(
		projecttypeenv.ProjectTypeEnvCompositePreparationInput{
			Base: base, Linked: linked, RuntimeBasis: runtimeBasis, Composite: composite,
		},
	)
	verification, exists := preparation.Verification()
	if preparation.Rejected() || !exists {
		return authorityStageTargetFixture{}, fmt.Errorf("prepare composite: %#v", preparation.Issues())
	}
	snapshot, executable := preparation.ExecutableSnapshot()
	if !executable {
		return authorityStageTargetFixture{}, fmt.Errorf(
			"prepare composite produced no executable snapshot",
		)
	}
	return authorityStageTargetFixture{
		verification: verification,
		snapshot:     snapshot,
	}, nil
}

func authorityTypeEnvAtRef(
	t *testing.T,
	ref typedmemory.TypeEnvRef,
	source typedmemory.TypeEnv,
) typedmemory.TypeEnv {
	t.Helper()
	contexts := source.BoundedContexts()
	if len(contexts) == 0 {
		t.Fatal("source executable TypeEnv has no bounded context")
	}
	environment, err := typedmemory.NewTypeEnvBuilder(ref).
		SetSourceRevision(source.SourceRevision()).
		SetCompilerSchemaVersion(source.CompilerSchemaVersion()).
		SetCoverageManifest(source.CoverageManifest()).
		AddBoundedContext(contexts[0]).
		Build()
	if err != nil {
		t.Fatalf("build prior executable TypeEnv fixture: %v", err)
	}
	return environment
}

func authorityRuntimeEvaluationBasis(
	base typeenv.BaseTypeEnvArtifact,
	linked projecttypeenv.LinkedProjectTypeEnvCompositeIR,
) (projecttypeenv.RuntimeEvaluationBasisArtifact, error) {
	empty, err := projecttypeenv.SealRuntimeEvaluationBasis(nil)
	if err != nil {
		return projecttypeenv.RuntimeEvaluationBasisArtifact{}, err
	}
	provisional, err := projecttypeenv.SealProjectTypeEnvComposite(linked, empty)
	if err != nil {
		return projecttypeenv.RuntimeEvaluationBasisArtifact{}, err
	}
	candidate, _, err := typeenv.LowerBaseTypeEnvArtifactWithCodecsAtRef(base, provisional.Ref())
	if err != nil {
		return projecttypeenv.RuntimeEvaluationBasisArtifact{}, err
	}
	resolution := projecttypeenv.ResolveProjectTypeEnvCompositeRuntimeRequirements(
		provisional, candidate, linked, empty,
	)
	requirements := resolution.RequiredSet().Requirements()
	entries := make([]runtimemechanism.RuntimeMechanismEntryV1, 0, len(requirements))
	for _, requirement := range requirements {
		entry, entryErr := authorityRuntimeEntry(requirement)
		if entryErr != nil {
			return projecttypeenv.RuntimeEvaluationBasisArtifact{}, entryErr
		}
		entries = append(entries, entry)
	}
	artifactRef, err := typedmemory.NewCarrierRef("artifact:authority-stage-runtime")
	if err != nil {
		return projecttypeenv.RuntimeEvaluationBasisArtifact{}, err
	}
	edition, err := typedmemory.NewCarrierEdition("1.0.0")
	if err != nil {
		return projecttypeenv.RuntimeEvaluationBasisArtifact{}, err
	}
	artifact, err := runtimemechanism.SealRuntimeMechanismArtifactV1(artifactRef, edition, entries)
	if err != nil {
		return projecttypeenv.RuntimeEvaluationBasisArtifact{}, err
	}
	mechanism, err := projecttypeenv.NewRuntimeMechanismArtifactPinFromArtifact(artifact)
	if err != nil {
		return projecttypeenv.RuntimeEvaluationBasisArtifact{}, err
	}
	pins := make([]projecttypeenv.RuntimeEvaluationMechanismPin, 0, len(requirements))
	for _, requirement := range requirements {
		pin, pinErr := authorityRuntimePin(requirement, mechanism, artifact)
		if pinErr != nil {
			return projecttypeenv.RuntimeEvaluationBasisArtifact{}, pinErr
		}
		pins = append(pins, pin)
	}
	return projecttypeenv.SealRuntimeEvaluationBasis(pins, artifact)
}

func authorityRuntimeEntry(
	requirement projecttypeenv.CompositeRuntimeRequirement,
) (runtimemechanism.RuntimeMechanismEntryV1, error) {
	codec, hasCodec := requirement.Codec()
	if hasCodec {
		return runtimemechanism.NewCodecCanonicalizationEntry(codec)
	}
	rule, hasRule := requirement.Rule()
	if !hasRule {
		return runtimemechanism.RuntimeMechanismEntryV1{}, fmt.Errorf("runtime requirement has no semantic ref")
	}
	switch requirement.InvocationContract() {
	case projecttypeenv.RuntimeMechanismContractEntitySetEnumeration:
		return runtimemechanism.NewEntitySetEnumerationEntry(rule)
	case projecttypeenv.RuntimeMechanismContractCandidateVisibility:
		return runtimemechanism.NewCandidateVisibilityEntry(rule)
	case projecttypeenv.RuntimeMechanismContractKindDefinedness:
		return runtimemechanism.NewKindDefinednessEntry(rule)
	case projecttypeenv.RuntimeMechanismContractMemberOf:
		return runtimemechanism.NewMemberOfEntry(rule)
	case projecttypeenv.RuntimeMechanismContractCarrierMembershipDelivery:
		return runtimemechanism.NewCarrierMembershipDeliveryEntry(rule)
	default:
		return runtimemechanism.RuntimeMechanismEntryV1{}, fmt.Errorf("unsupported runtime contract")
	}
}

func authorityRuntimePin(
	requirement projecttypeenv.CompositeRuntimeRequirement,
	mechanism projecttypeenv.RuntimeMechanismArtifactPin,
	artifact runtimemechanism.RuntimeMechanismArtifactV1,
) (projecttypeenv.RuntimeEvaluationMechanismPin, error) {
	codec, hasCodec := requirement.Codec()
	if hasCodec {
		return projecttypeenv.NewCodecRuntimeMechanismPin(
			projecttypeenv.CodecRuntimeMechanismPinInput{
				Codec: codec, Mechanism: mechanism, ResolvedArtifact: &artifact,
			},
		)
	}
	rule, hasRule := requirement.Rule()
	if !hasRule {
		return nil, fmt.Errorf("runtime requirement has no semantic ref")
	}
	if requirement.Role() == projecttypeenv.RuntimeMechanismRoleCarrierMembership {
		return projecttypeenv.NewCarrierMembershipRuntimeMechanismPin(
			projecttypeenv.CarrierMembershipRuntimeMechanismPinInput{
				Rule: rule, Mechanism: mechanism, ResolvedArtifact: &artifact,
			},
		)
	}
	return projecttypeenv.NewEvaluatorRuntimeMechanismPin(
		projecttypeenv.EvaluatorRuntimeMechanismPinInput{
			Rule: rule, Contract: requirement.InvocationContract(), Mechanism: mechanism, ResolvedArtifact: &artifact,
		},
	)
}

func mustGraphSnapshot(
	t *testing.T,
	project projectidentity.ProjectID,
	revision typedmemory.GraphRevision,
) projecttypeenvselection.ProjectGraphSnapshotBasis {
	t.Helper()
	event, err := projecttypeenvselection.ParseGraphEventRef("typed-memory-event:" + strings.Repeat("3", 64))
	if err != nil {
		t.Fatalf("ParseGraphEventRef: %v", err)
	}
	commit, err := projecttypeenvselection.ParseGraphCommitRef("typed-memory-commit:" + strings.Repeat("3", 64))
	if err != nil {
		t.Fatalf("ParseGraphCommitRef: %v", err)
	}
	closure, err := projecttypeenvselection.NewCommittedProjectGraphClosure(
		projecttypeenvselection.CommittedProjectGraphClosureInput{
			Event: event, Commit: commit, MaterializationDigest: mustTypedDigest(t, "3"),
		},
	)
	if err != nil {
		t.Fatalf("NewCommittedProjectGraphClosure: %v", err)
	}
	snapshot, err := projecttypeenvselection.SealProjectGraphSnapshotBasis(
		projecttypeenvselection.ProjectGraphSnapshotBasisInput{
			Project: project, GraphRevision: revision, Closure: closure,
		},
	)
	if err != nil {
		t.Fatalf("SealProjectGraphSnapshotBasis: %v", err)
	}
	return snapshot
}

func mustProjectID(t *testing.T, raw string) projectidentity.ProjectID {
	t.Helper()
	value, err := projectidentity.ParseProjectID(raw)
	if err != nil {
		t.Fatalf("ParseProjectID: %v", err)
	}
	return value
}

func mustTypeEnvRef(t *testing.T, digit string) typedmemory.TypeEnvRef {
	t.Helper()
	value, err := typedmemory.ParseTypeEnvRef("typeenv:sha256:" + strings.Repeat(digit, 64))
	if err != nil {
		t.Fatalf("ParseTypeEnvRef: %v", err)
	}
	return value
}

func mustTypedDigest(t *testing.T, digit string) typedmemory.SHA256Digest {
	t.Helper()
	value, err := typedmemory.NewSHA256Digest("sha256:" + strings.Repeat(digit, 64))
	if err != nil {
		t.Fatalf("NewSHA256Digest: %v", err)
	}
	return value
}

func mustAuthorityDigest(t *testing.T, digit string) authority.Digest {
	t.Helper()
	value, err := authority.NewDigest("sha256:" + strings.Repeat(digit, 64))
	if err != nil {
		t.Fatalf("NewDigest: %v", err)
	}
	return value
}

func mustDescriptionRef(t *testing.T, raw string) authority.DescriptionRef {
	t.Helper()
	value, err := authority.NewClaimIDDescriptionRef(raw)
	if err != nil {
		t.Fatalf("NewClaimIDDescriptionRef: %v", err)
	}
	return value
}

func mustAuthorityValue[T any](
	t *testing.T,
	constructor func(string) (T, error),
	raw string,
) T {
	t.Helper()
	value, err := constructor(raw)
	if err != nil {
		t.Fatalf("construct authority value %q: %v", raw, err)
	}
	return value
}
