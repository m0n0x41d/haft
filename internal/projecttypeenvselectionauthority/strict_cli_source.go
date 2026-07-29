package projecttypeenvselectionauthority

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
)

const (
	strictCLITypeEnvSelectionPhrase = "AUTHORIZE THIS REVIEWED TYPEENV SELECTION"

	strictCLITypeEnvSelectionReviewTitle = "Authorize the exact reviewed ProjectTypeEnvHead selection."
)

// StrictCLISpeechActPreparationInput contains descriptions and observable
// carrier bindings only. It contains no terminal observation, performed Work,
// SpeechAct occurrence, Permission, authority resolution, or head effect.
type StrictCLISpeechActPreparationInput struct {
	Request        projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest
	Content        ProjectTypeEnvHeadSelectionAuthorizationContent
	Stage          projecttypeenvselection.ProjectTypeEnvStage
	ProjectBinding ProjectAuthorityContextBinding
	ConfigCarrier  authority.ObservableCarrierBinding
	ReviewCarrier  authority.ObservableCarrierBinding
}

// StrictCLISpeechActPreparation is the sealed pre-act description used by the
// strict CLI adapter. The literal phrase is a policy-owned utterance rule; the
// reviewed content and request remain bound by canonical digests rather than
// by the human transcribing opaque identifiers.
type StrictCLISpeechActPreparation struct {
	request        projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest
	content        ProjectTypeEnvHeadSelectionAuthorizationContent
	stage          projecttypeenvselection.ProjectTypeEnvStage
	projectBinding ProjectAuthorityContextBinding
	configBasis    ProjectTypeEnvHeadSelectionConfigAuthorityBasis
	modePolicy     StrictCLISpeechActAuthorityPolicy
	resolverPolicy ProjectTypeEnvHeadSelectionResolverPolicy
	prepared       authority.PreparedManualSpeechAct
	anchors        authority.SpeechActSourceV2Anchors
	speechActRef   authority.SpeechActRef
	reviewCarrier  authority.ObservableCarrierBinding
}

// StrictCLITypeEnvSelectionPhrase is the complete canonical utterance required
// by the strict controlling-terminal adapter.
func StrictCLITypeEnvSelectionPhrase() string {
	return strictCLITypeEnvSelectionPhrase
}

func PrepareStrictCLISpeechAct(
	input StrictCLISpeechActPreparationInput,
) (StrictCLISpeechActPreparation, error) {
	if err := input.Content.ExactAgainst(input.Request); err != nil {
		return StrictCLISpeechActPreparation{}, fmt.Errorf(
			"prepare strict TypeEnv SpeechAct content: %w",
			err,
		)
	}
	if !sameStage(input.Content.Stage(), input.Stage) {
		return StrictCLISpeechActPreparation{}, fmt.Errorf(
			"prepare strict TypeEnv SpeechAct: Stage differs from reviewed content",
		)
	}
	if !input.ProjectBinding.ExactFor(
		input.Request.Project(),
		input.ProjectBinding.Root(),
		input.Content.JudgementContext(),
	) {
		return StrictCLISpeechActPreparation{}, fmt.Errorf(
			"prepare strict TypeEnv SpeechAct: project-context binding is not exact",
		)
	}
	configBasis, err := SealProjectTypeEnvHeadSelectionConfigAuthorityBasis(
		input.Request.Project(),
		ProjectTypeEnvHeadSelectionAuthorityModeStrictCLISpeechAct,
		input.ConfigCarrier,
	)
	if err != nil {
		return StrictCLISpeechActPreparation{}, err
	}
	sourceContract, err :=
		NewProjectTypeEnvHeadSelectionSpeechActSourceContract(
			input.Content.JudgementContext(),
		)
	if err != nil {
		return StrictCLISpeechActPreparation{}, err
	}
	action, err := input.Content.Action().AuthorityActionKind()
	if err != nil {
		return StrictCLISpeechActPreparation{}, err
	}
	semanticSuffix := strictCLICoordinateSuffix(
		input.Content.JudgementContext().String(),
		action.String(),
	)
	occurrenceSuffix := strings.TrimPrefix(
		input.Content.Digest().String(),
		"sha256:",
	)
	utteranceRule, err := authority.NewLiteralSpeechActUtteranceRule(
		"AUTHORIZE",
		"THIS REVIEWED TYPEENV SELECTION",
	)
	if err != nil {
		return StrictCLISpeechActPreparation{}, err
	}
	effectRuleRef, err := authority.NewInstitutionalEffectRuleRef(
		"institution-rule:project-typeenv-head-selection:strict-cli:" +
			semanticSuffix,
	)
	if err != nil {
		return StrictCLISpeechActPreparation{}, err
	}
	institutedKind, err := authority.NewInstitutedObjectKind("U.Commitment")
	if err != nil {
		return StrictCLISpeechActPreparation{}, err
	}
	modality, err := authority.NewInstitutionalModality("MAY")
	if err != nil {
		return StrictCLISpeechActPreparation{}, err
	}
	utteranceRef, err := authority.NewUtteranceRef(
		"utterance:authorize-reviewed-project-typeenv-head-selection",
	)
	if err != nil {
		return StrictCLISpeechActPreparation{}, err
	}
	effectRule, err := authority.NewInstitutionalEffectRule(
		effectRuleRef,
		institutedKind,
		modality,
		action,
		utteranceRule,
		utteranceRef,
	)
	if err != nil {
		return StrictCLISpeechActPreparation{}, err
	}
	contextPolicyRef, err := authority.NewContextPolicyRef(
		"policy:project-typeenv-head-selection:strict-cli:" +
			semanticSuffix,
	)
	if err != nil {
		return StrictCLISpeechActPreparation{}, err
	}
	contextPolicy, err := authority.NewSpeechActContextPolicy(
		contextPolicyRef,
		input.Content.JudgementContext(),
		sourceContract.ActType(),
		effectRule,
	)
	if err != nil {
		return StrictCLISpeechActPreparation{}, err
	}
	methodRef, err := authority.NewMethodRef(
		"method:manual-project-typeenv-head-selection",
	)
	if err != nil {
		return StrictCLISpeechActPreparation{}, err
	}
	methodDescriptionRef, err := authority.NewMethodDescriptionRef(
		"method-description:manual-project-typeenv-head-selection:" +
			semanticSuffix,
	)
	if err != nil {
		return StrictCLISpeechActPreparation{}, err
	}
	procedureRef, err := authority.NewMethodProcedureRef(
		"procedure:review-and-authorize-project-typeenv-head-selection:v1",
	)
	if err != nil {
		return StrictCLISpeechActPreparation{}, err
	}
	methodDescription, err := authority.NewManualControllingTTYMethodDescription(
		methodRef,
		methodDescriptionRef,
		procedureRef,
		input.Content.JudgementContext(),
	)
	if err != nil {
		return StrictCLISpeechActPreparation{}, err
	}
	executedWithin, err := authority.NewSystemRef("system:haft-cli")
	if err != nil {
		return StrictCLISpeechActPreparation{}, err
	}
	statePlane, err := authority.NewStatePlaneRef(
		typeEnvSpeechActStatePlaneValue,
	)
	if err != nil {
		return StrictCLISpeechActPreparation{}, err
	}
	delta, err := authority.NewDeltaPredicateRef(typeEnvSpeechActDeltaValue)
	if err != nil {
		return StrictCLISpeechActPreparation{}, err
	}
	outcome, err := authority.NewWorkOutcomeRef(
		"outcome:reviewed-project-typeenv-selection-authorized",
	)
	if err != nil {
		return StrictCLISpeechActPreparation{}, err
	}
	parameter, err := authority.NewWorkParameterBinding(
		"parameter:reviewed-authorization-content-digest",
		input.Content.Digest().String(),
	)
	if err != nil {
		return StrictCLISpeechActPreparation{}, err
	}
	terminalResource, err := authority.NewWorkResourceRef(
		"resource:controlling-terminal",
	)
	if err != nil {
		return StrictCLISpeechActPreparation{}, err
	}
	frameBuilder := authority.NewSpeechActExecutionFrameBuilder(
		methodDescription,
	)
	frameBuilder = frameBuilder.ExecutedWithin(executedWithin)
	frameBuilder = frameBuilder.OnStatePlane(statePlane, delta)
	frameBuilder = frameBuilder.WithOutcome(outcome)
	frameBuilder = frameBuilder.WithUtteranceDescription(utteranceRef)
	frameBuilder = frameBuilder.BindParameter(parameter)
	frameBuilder = frameBuilder.UseResource(terminalResource)
	affected, err := requiredAffectedReferents(input.Request)
	if err != nil {
		return StrictCLISpeechActPreparation{}, err
	}
	for _, ref := range affected {
		frameBuilder = frameBuilder.Affect(ref)
	}
	frame, err := frameBuilder.Build()
	if err != nil {
		return StrictCLISpeechActPreparation{}, err
	}
	speechActRef, err := authority.NewSpeechActRef(
		"speech-act:project-typeenv-head-selection:" + occurrenceSuffix,
	)
	if err != nil {
		return StrictCLISpeechActPreparation{}, err
	}
	permissionRef, err := DeriveProjectTypeEnvHeadSelectionPermissionRef(
		input.Content,
		speechActRef,
	)
	if err != nil {
		return StrictCLISpeechActPreparation{}, err
	}
	institutedObject, err := authority.NewInstitutedObjectRef(
		permissionRef.String(),
	)
	if err != nil {
		return StrictCLISpeechActPreparation{}, err
	}
	captureRef, err := authority.NewCarrierRef(
		"carrier:project-typeenv-head-selection-terminal-capture:" +
			occurrenceSuffix,
	)
	if err != nil {
		return StrictCLISpeechActPreparation{}, err
	}
	sessionRef, err := authority.NewSessionRef(
		"session:project-typeenv-head-selection:" + occurrenceSuffix,
	)
	if err != nil {
		return StrictCLISpeechActPreparation{}, err
	}
	reviewSubject, err := authority.NewSpeechActReviewSubjectRef(
		input.Content.DescriptionRef().String(),
	)
	if err != nil {
		return StrictCLISpeechActPreparation{}, err
	}
	intent, err := authority.NewPreparedSpeechActIntentBuilder(
		speechActRef,
		captureRef,
	).
		ForProject(input.ProjectBinding.Root()).
		InSession(sessionRef).
		Reviewing(reviewSubject, input.Content.Digest()).
		Institutes(institutedObject).
		UnderContextPolicy(contextPolicy).
		WithExecutionFrame(frame).
		Build()
	if err != nil {
		return StrictCLISpeechActPreparation{}, err
	}
	reviewText := strictCLITypeEnvSelectionReviewText(input)
	prepared, err := authority.PrepareManualSpeechAct(intent, reviewText)
	if err != nil {
		return StrictCLISpeechActPreparation{}, err
	}
	workRef, err := authority.NewWorkRef(
		"work:authorize-project-typeenv-head-selection:" + occurrenceSuffix,
	)
	if err != nil {
		return StrictCLISpeechActPreparation{}, err
	}
	resourceLedgerRef, err := authority.NewResourceLedgerRef(
		"resource-ledger:project-typeenv-head-selection:" + occurrenceSuffix,
	)
	if err != nil {
		return StrictCLISpeechActPreparation{}, err
	}
	acceptanceRef, err := authority.NewAcceptancePostureRef(
		"acceptance:exact-terminal-utterance-recognized",
	)
	if err != nil {
		return StrictCLISpeechActPreparation{}, err
	}
	auditRef, err := authority.NewAuditTraceRef(
		"audit:project-typeenv-head-selection:" + occurrenceSuffix,
	)
	if err != nil {
		return StrictCLISpeechActPreparation{}, err
	}
	anchors, err := authority.NewSpeechActSourceV2AnchorsBuilder(
		workRef,
		input.Content.DescriptionRef(),
	).
		WithResourceLedger(resourceLedgerRef).
		WithAcceptancePosture(acceptanceRef).
		WithAuditTrace(auditRef).
		WithDescriptionCarrier(input.ReviewCarrier).
		Build()
	if err != nil {
		return StrictCLISpeechActPreparation{}, err
	}
	sourceAdapter, err := SealProjectTypeEnvHeadSelectionSourceAdapterPolicy(
		methodDescription,
		executedWithin,
		contextPolicy,
	)
	if err != nil {
		return StrictCLISpeechActPreparation{}, err
	}
	resolverRef, err := NewProjectTypeEnvHeadSelectionResolverPolicyRef(
		"resolver-policy:project-typeenv-head-selection:strict-cli:" +
			strings.TrimPrefix(input.ProjectBinding.Digest().String(), "sha256:"),
	)
	if err != nil {
		return StrictCLISpeechActPreparation{}, err
	}
	resolverEdition, err := NewProjectTypeEnvHeadSelectionResolverPolicyEdition(
		"resolver-policy-edition:project-typeenv-head-selection:strict-cli:v1",
	)
	if err != nil {
		return StrictCLISpeechActPreparation{}, err
	}
	resolverPolicy, err := SealProjectTypeEnvHeadSelectionResolverPolicy(
		ProjectTypeEnvHeadSelectionResolverPolicyInput{
			Ref:             resolverRef,
			Edition:         resolverEdition,
			EffectiveWindow: input.Content.ValidityWindow(),
			Action:          input.Content.Action(),
			SourceContract:  sourceContract,
			SourceAdapter:   sourceAdapter,
			ProjectBinding:  input.ProjectBinding,
		},
	)
	if err != nil {
		return StrictCLISpeechActPreparation{}, err
	}
	modePolicy, err := SealStrictCLISpeechActAuthorityPolicy(
		configBasis,
		resolverPolicy,
	)
	if err != nil {
		return StrictCLISpeechActPreparation{}, err
	}
	return StrictCLISpeechActPreparation{
		request:        input.Request,
		content:        input.Content,
		stage:          input.Stage,
		projectBinding: input.ProjectBinding,
		configBasis:    configBasis,
		modePolicy:     modePolicy,
		resolverPolicy: resolverPolicy,
		prepared:       prepared,
		anchors:        anchors,
		speechActRef:   speechActRef,
		reviewCarrier:  input.ReviewCarrier,
	}, nil
}

func strictCLITypeEnvSelectionReviewText(
	input StrictCLISpeechActPreparationInput,
) string {
	lines := []string{
		strictCLITypeEnvSelectionReviewTitle,
		"Project: " + input.Request.Project().String(),
		"Request: " + input.Request.Ref().String(),
		"Reviewed content: " + input.Content.DescriptionRef().String(),
		"Content digest: " + input.Content.Digest().String(),
		"Stage: " + input.Stage.Ref().String(),
		"Action: " + input.Content.Action().String(),
		"Valid until: " + formatTime(input.Content.ValidityWindow().Until()),
		"This act institutes one bounded MAY Permission for the reviewed head selection; it does not perform the head CAS.",
	}
	return strings.Join(lines, "\n")
}

func strictCLICoordinateSuffix(values ...string) string {
	hasher := sha256.New()
	for _, value := range values {
		_, _ = hasher.Write([]byte(value))
		_, _ = hasher.Write([]byte{0})
	}
	return hex.EncodeToString(hasher.Sum(nil))
}

func (preparation StrictCLISpeechActPreparation) Verify() error {
	rebuilt, err := PrepareStrictCLISpeechAct(
		StrictCLISpeechActPreparationInput{
			Request:        preparation.request,
			Content:        preparation.content,
			Stage:          preparation.stage,
			ProjectBinding: preparation.projectBinding,
			ConfigCarrier:  preparation.configBasis.ConfigCarrier(),
			ReviewCarrier:  preparation.reviewCarrier,
		},
	)
	if err != nil {
		return err
	}
	if rebuilt.speechActRef != preparation.speechActRef ||
		rebuilt.configBasis.Digest() != preparation.configBasis.Digest() ||
		rebuilt.modePolicy.Digest() != preparation.modePolicy.Digest() ||
		rebuilt.resolverPolicy.Digest() != preparation.resolverPolicy.Digest() {
		return fmt.Errorf("strict TypeEnv SpeechAct preparation differs from exact input")
	}
	return nil
}

func (preparation StrictCLISpeechActPreparation) SealCaptured(
	basis authority.VerifiedSpeechActSource,
) (ProjectTypeEnvHeadSelectionSpeechActRecord, error) {
	if err := preparation.Verify(); err != nil {
		return ProjectTypeEnvHeadSelectionSpeechActRecord{}, err
	}
	source, err := authority.NewVerifiedSpeechActSourceV2(
		basis,
		preparation.anchors,
	)
	if err != nil {
		return ProjectTypeEnvHeadSelectionSpeechActRecord{}, err
	}
	return SealProjectTypeEnvHeadSelectionSpeechActRecord(
		ProjectTypeEnvHeadSelectionSpeechActRecordInput{
			Source:         source,
			SourceContract: preparation.resolverPolicy.SourceContract(),
			ProjectBinding: preparation.projectBinding,
			Content:        preparation.content,
			Request:        preparation.request,
			Stage:          preparation.stage,
		},
	)
}

func (preparation StrictCLISpeechActPreparation) DecodeRecorded(
	basis authority.RecordedSpeechActSource,
	canonical []byte,
	digest authority.Digest,
) (ProjectTypeEnvHeadSelectionSpeechActRecord, error) {
	if err := preparation.Verify(); err != nil {
		return ProjectTypeEnvHeadSelectionSpeechActRecord{}, err
	}
	projection := speechActRecordProjection{}
	if err := decodeStrictJSON(canonical, &projection); err != nil {
		return ProjectTypeEnvHeadSelectionSpeechActRecord{}, err
	}
	sourceDigest, err := authority.NewDigest(projection.SourceDigest)
	if err != nil {
		return ProjectTypeEnvHeadSelectionSpeechActRecord{}, err
	}
	source, err := authority.DecodeRecordedSpeechActSourceV2(
		basis,
		projection.Source,
		sourceDigest,
	)
	if err != nil {
		return ProjectTypeEnvHeadSelectionSpeechActRecord{}, err
	}
	return DecodeProjectTypeEnvHeadSelectionSpeechActRecord(
		ProjectTypeEnvHeadSelectionSpeechActRecordInput{
			Source:         source,
			SourceContract: preparation.resolverPolicy.SourceContract(),
			ProjectBinding: preparation.projectBinding,
			Content:        preparation.content,
			Request:        preparation.request,
			Stage:          preparation.stage,
		},
		canonical,
		digest,
	)
}

func (preparation StrictCLISpeechActPreparation) PreparedSpeechAct() authority.PreparedManualSpeechAct {
	return preparation.prepared
}

func (preparation StrictCLISpeechActPreparation) SpeechActRef() authority.SpeechActRef {
	return preparation.speechActRef
}

func (preparation StrictCLISpeechActPreparation) Request() projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest {
	return preparation.request
}

func (preparation StrictCLISpeechActPreparation) Content() ProjectTypeEnvHeadSelectionAuthorizationContent {
	return preparation.content
}

func (preparation StrictCLISpeechActPreparation) Stage() projecttypeenvselection.ProjectTypeEnvStage {
	return preparation.stage
}

func (preparation StrictCLISpeechActPreparation) ConfigBasis() ProjectTypeEnvHeadSelectionConfigAuthorityBasis {
	return preparation.configBasis
}

func (preparation StrictCLISpeechActPreparation) ModePolicy() StrictCLISpeechActAuthorityPolicy {
	return preparation.modePolicy
}

func (preparation StrictCLISpeechActPreparation) ResolverPolicy() ProjectTypeEnvHeadSelectionResolverPolicy {
	return preparation.resolverPolicy
}

func (preparation StrictCLISpeechActPreparation) ProjectBinding() ProjectAuthorityContextBinding {
	return preparation.projectBinding
}

func (preparation StrictCLISpeechActPreparation) ReviewCarrier() authority.ObservableCarrierBinding {
	return preparation.reviewCarrier
}
