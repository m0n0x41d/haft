package projecttypeenvselectionauthority

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projecttypeenvprofilefit"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
)

const (
	authorizationContentSchema       = "haft.project-typeenv.head-selection-authorization-content/v2"
	legacyAuthorizationContentSchema = "haft.project-typeenv.head-selection-authorization-content/v1"
	maximumContentCanonicalBytes     = 512 * 1024
)

// ProjectTypeEnvHeadSelectionAuthorizationContent is the immutable reviewed
// utterance description. Its DescriptionRef identifies the description; its
// digest authenticates these exact canonical bytes. It is neither a
// SpeechAct occurrence nor a current authority judgement.
type ProjectTypeEnvHeadSelectionAuthorizationContent struct {
	schema         string
	descriptionRef authority.DescriptionRef
	digest         authority.Digest
	request        projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest
	stage          projecttypeenvselection.ProjectTypeEnvStage
	context        authority.BoundedContextRef
	validity       authority.TimeWindow
	intendedUpdate ProjectTypeEnvHeadUpdate
	canonicalJSON  []byte
}

type ProjectTypeEnvHeadSelectionAuthorizationContentInput struct {
	DescriptionRef   authority.DescriptionRef
	Request          projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest
	Stage            projecttypeenvselection.ProjectTypeEnvStage
	JudgementContext authority.BoundedContextRef
	ValidityWindow   authority.TimeWindow
}

type authorizationContentProjection struct {
	Schema                string                   `json:"schema"`
	DescriptionKind       string                   `json:"description_ref_kind"`
	DescriptionRef        string                   `json:"description_ref"`
	RequestRef            string                   `json:"request_ref"`
	RequestDigest         string                   `json:"request_digest"`
	Project               string                   `json:"project_id"`
	Predecessor           predecessorProjection    `json:"predecessor"`
	Base                  string                   `json:"base_type_env_ref"`
	OrderedExtensions     []string                 `json:"ordered_extension_refs"`
	RuntimeBasis          string                   `json:"runtime_evaluation_basis_ref"`
	VerifiedComposite     string                   `json:"verified_composite_ref"`
	Stage                 string                   `json:"stage_ref"`
	StageDigest           string                   `json:"stage_digest"`
	ExpectedGraphRevision uint64                   `json:"expected_graph_revision"`
	CompatibilityPosture  string                   `json:"compatibility_posture"`
	CompatibilityRef      string                   `json:"compatibility_ref"`
	CompatibilityDigest   string                   `json:"compatibility_digest"`
	RevalidationPosture   string                   `json:"revalidation_posture"`
	RevalidationRef       string                   `json:"revalidation_ref"`
	RevalidationDigest    string                   `json:"revalidation_digest"`
	ProfilePosture        string                   `json:"profile_posture"`
	ProfileFitRef         string                   `json:"profile_fit_ref"`
	ProfileFitDigest      string                   `json:"profile_fit_digest"`
	JudgementContext      string                   `json:"judgement_context_ref"`
	Action                string                   `json:"action"`
	ValidityFrom          string                   `json:"validity_from"`
	ValidityUntil         string                   `json:"validity_until"`
	IdempotencyKey        string                   `json:"idempotency_key"`
	IntendedUpdate        intendedUpdateProjection `json:"intended_head_update"`
}

type predecessorProjection struct {
	Kind                   string `json:"kind"`
	NoPriorHeadProofRef    string `json:"no_prior_head_proof_ref,omitempty"`
	PriorHeadRef           string `json:"prior_head_ref,omitempty"`
	PriorHeadRevision      uint64 `json:"prior_head_revision,omitempty"`
	PriorSelectedComposite string `json:"prior_selected_composite_ref,omitempty"`
}

type intendedUpdateProjection struct {
	Kind                   string `json:"kind"`
	HeadRef                string `json:"head_ref"`
	PriorHeadRevision      uint64 `json:"prior_head_revision,omitempty"`
	PriorSelectedComposite string `json:"prior_selected_composite_ref,omitempty"`
	SelectedComposite      string `json:"selected_composite_ref"`
	SuccessorHeadRevision  uint64 `json:"successor_head_revision"`
}

func SealProjectTypeEnvHeadSelectionAuthorizationContent(
	input ProjectTypeEnvHeadSelectionAuthorizationContentInput,
) (ProjectTypeEnvHeadSelectionAuthorizationContent, error) {
	return sealProjectTypeEnvHeadSelectionAuthorizationContent(
		input,
		authorizationContentSchema,
	)
}

func sealProjectTypeEnvHeadSelectionAuthorizationContent(
	input ProjectTypeEnvHeadSelectionAuthorizationContentInput,
	schema string,
) (ProjectTypeEnvHeadSelectionAuthorizationContent, error) {
	if schema != authorizationContentSchema &&
		schema != legacyAuthorizationContentSchema {
		return ProjectTypeEnvHeadSelectionAuthorizationContent{}, fmt.Errorf(
			"head-selection authorization content schema is unsupported",
		)
	}
	request := input.Request
	if err := projecttypeenvselection.VerifyProjectTypeEnvHeadSelectionRequestAgainstStage(
		request,
		input.Stage,
	); err != nil {
		return ProjectTypeEnvHeadSelectionAuthorizationContent{}, fmt.Errorf(
			"authorization content Stage binding: %w",
			err,
		)
	}
	if _, err := parseDescriptionRef(input.DescriptionRef.Kind(), input.DescriptionRef.String()); err != nil {
		return ProjectTypeEnvHeadSelectionAuthorizationContent{}, err
	}
	context, err := authority.NewBoundedContextRef(input.JudgementContext.String())
	if err != nil || context != input.JudgementContext {
		return ProjectTypeEnvHeadSelectionAuthorizationContent{}, fmt.Errorf(
			"authorization content judgement context is invalid",
		)
	}
	validity, err := authority.NewTimeWindow(
		input.ValidityWindow.From(),
		input.ValidityWindow.Until(),
	)
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorizationContent{}, fmt.Errorf(
			"authorization content validity window: %w",
			err,
		)
	}
	update, err := deriveHeadUpdate(request)
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorizationContent{}, err
	}
	content := ProjectTypeEnvHeadSelectionAuthorizationContent{
		schema:         schema,
		descriptionRef: input.DescriptionRef,
		request:        request,
		stage:          input.Stage,
		context:        context,
		validity:       validity,
		intendedUpdate: update,
	}
	projection, err := projectAuthorizationContent(content)
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorizationContent{}, err
	}
	canonical, err := json.Marshal(projection)
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorizationContent{}, fmt.Errorf(
			"encode head-selection authorization content: %w",
			err,
		)
	}
	if len(canonical) > maximumContentCanonicalBytes {
		return ProjectTypeEnvHeadSelectionAuthorizationContent{}, fmt.Errorf(
			"head-selection authorization content exceeds %d bytes",
			maximumContentCanonicalBytes,
		)
	}
	digest, err := digestCanonical(schema, canonical)
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorizationContent{}, err
	}
	content.digest = digest
	content.canonicalJSON = canonical
	return content, nil
}

func projectAuthorizationContent(
	content ProjectTypeEnvHeadSelectionAuthorizationContent,
) (authorizationContentProjection, error) {
	request := content.request
	target := request.Target()
	predecessor, err := projectPredecessor(request.Predecessor())
	if err != nil {
		return authorizationContentProjection{}, err
	}
	extensions := target.OrderedExtensions()
	extensionRefs := make([]string, 0, len(extensions))
	for _, extension := range extensions {
		extensionRefs = append(extensionRefs, extension.String())
	}
	compatibilityPosture, err := stageCompatibilityPosture(content.stage)
	if err != nil {
		return authorizationContentProjection{}, err
	}
	profilePosture, err := stageProfilePosture(content.stage)
	if err != nil {
		return authorizationContentProjection{}, err
	}
	update := projectIntendedUpdate(content.intendedUpdate)
	return authorizationContentProjection{
		Schema:                content.schema,
		DescriptionKind:       string(content.descriptionRef.Kind()),
		DescriptionRef:        content.descriptionRef.String(),
		RequestRef:            request.Ref().String(),
		RequestDigest:         request.Ref().Digest().String(),
		Project:               request.Project().String(),
		Predecessor:           predecessor,
		Base:                  target.Base().String(),
		OrderedExtensions:     extensionRefs,
		RuntimeBasis:          target.RuntimeBasis().String(),
		VerifiedComposite:     target.VerifiedComposite().String(),
		Stage:                 target.Stage().String(),
		StageDigest:           target.Stage().Digest().String(),
		ExpectedGraphRevision: request.ExpectedGraphRevision().Value(),
		CompatibilityPosture:  compatibilityPosture,
		CompatibilityRef:      content.stage.CompatibilityRef().String(),
		CompatibilityDigest:   content.stage.CompatibilityDigest().String(),
		RevalidationPosture:   content.stage.ExistingAssertionRevalidation().Posture().String(),
		RevalidationRef:       content.stage.ExistingAssertionRevalidationRef().String(),
		RevalidationDigest:    content.stage.ExistingAssertionRevalidationDigest().String(),
		ProfilePosture:        profilePosture,
		ProfileFitRef:         content.stage.ProfileFitRef().String(),
		ProfileFitDigest:      content.stage.ProfileFitDigest().String(),
		JudgementContext:      content.context.String(),
		Action:                content.intendedUpdate.Action().String(),
		ValidityFrom:          formatTime(content.validity.From()),
		ValidityUntil:         formatTime(content.validity.Until()),
		IdempotencyKey:        request.IdempotencyKey().String(),
		IntendedUpdate:        update,
	}, nil
}

func projectPredecessor(
	predecessor projecttypeenvselection.ProjectTypeEnvHeadSelectionPredecessor,
) (predecessorProjection, error) {
	switch value := predecessor.(type) {
	case projecttypeenvselection.GenesisStagePredecessor:
		return predecessorProjection{
			Kind: "genesis",
		}, nil
	case projecttypeenvselection.TransitionStagePredecessor:
		return predecessorProjection{
			Kind:                   "transition",
			PriorHeadRef:           value.Head().String(),
			PriorHeadRevision:      value.HeadRevision().Value(),
			PriorSelectedComposite: value.SelectedComposite().String(),
		}, nil
	default:
		if proof, ok := projecttypeenvselection.LegacyGenesisNoPriorHeadProof(
			predecessor,
		); ok {
			return predecessorProjection{
				Kind:                "genesis",
				NoPriorHeadProofRef: proof.String(),
			}, nil
		}
		return predecessorProjection{}, fmt.Errorf("head-selection predecessor is invalid")
	}
}

func projectIntendedUpdate(update ProjectTypeEnvHeadUpdate) intendedUpdateProjection {
	projection := intendedUpdateProjection{
		Kind:                  update.Action().String(),
		HeadRef:               update.Head().String(),
		SelectedComposite:     update.SelectedComposite().String(),
		SuccessorHeadRevision: update.SuccessorRevision().Value(),
	}
	priorComposite, hasPrior := update.PriorComposite()
	priorRevision, _ := update.PriorRevision()
	if hasPrior {
		projection.PriorSelectedComposite = priorComposite.String()
		projection.PriorHeadRevision = priorRevision.Value()
	}
	return projection
}

func stageCompatibilityPosture(
	stage projecttypeenvselection.ProjectTypeEnvStage,
) (string, error) {
	switch stage.Compatibility().(type) {
	case projecttypeenvselection.InitialStageCompatibility:
		return "initial", nil
	case projecttypeenvselection.ComparedStageCompatibility:
		return "compared", nil
	default:
		return "", fmt.Errorf("Stage compatibility posture is invalid")
	}
}

func stageProfilePosture(stage projecttypeenvselection.ProjectTypeEnvStage) (string, error) {
	return projectProfileFitPosture(stage.ProfileCompatibility())
}

func projectProfileFitPosture(
	assessment projecttypeenvprofilefit.Assessment,
) (string, error) {
	switch assessment.(type) {
	case projecttypeenvprofilefit.Compatible:
		return "compatible", nil
	case projecttypeenvprofilefit.Incompatible:
		return "incompatible", nil
	case projecttypeenvprofilefit.Underdetermined:
		return "underdetermined", nil
	case projecttypeenvprofilefit.Unavailable:
		return "unavailable", nil
	default:
		return "", fmt.Errorf("Stage project-profile posture is invalid")
	}
}

func DecodeProjectTypeEnvHeadSelectionAuthorizationContent(
	request projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest,
	stage projecttypeenvselection.ProjectTypeEnvStage,
	canonical []byte,
	digest authority.Digest,
) (ProjectTypeEnvHeadSelectionAuthorizationContent, error) {
	if len(canonical) == 0 || len(canonical) > maximumContentCanonicalBytes {
		return ProjectTypeEnvHeadSelectionAuthorizationContent{}, fmt.Errorf(
			"head-selection authorization content has invalid canonical size",
		)
	}
	projection := authorizationContentProjection{}
	if err := decodeStrictJSON(canonical, &projection); err != nil {
		return ProjectTypeEnvHeadSelectionAuthorizationContent{}, err
	}
	descriptionKind := authority.DescriptionRefKind(projection.DescriptionKind)
	description, err := parseDescriptionRef(descriptionKind, projection.DescriptionRef)
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorizationContent{}, err
	}
	from, err := parseTime(projection.ValidityFrom)
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorizationContent{}, err
	}
	until, err := parseTime(projection.ValidityUntil)
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorizationContent{}, err
	}
	validity, err := authority.NewTimeWindow(from, until)
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorizationContent{}, err
	}
	context, err := authority.NewBoundedContextRef(projection.JudgementContext)
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorizationContent{}, err
	}
	rebuilt, err := sealProjectTypeEnvHeadSelectionAuthorizationContent(
		ProjectTypeEnvHeadSelectionAuthorizationContentInput{
			DescriptionRef:   description,
			Request:          request,
			Stage:            stage,
			JudgementContext: context,
			ValidityWindow:   validity,
		},
		projection.Schema,
	)
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorizationContent{}, err
	}
	if rebuilt.digest != digest || !bytes.Equal(rebuilt.canonicalJSON, canonical) {
		return ProjectTypeEnvHeadSelectionAuthorizationContent{}, fmt.Errorf(
			"head-selection authorization content is not exact canonical material",
		)
	}
	return rebuilt, nil
}

func parseDescriptionRef(
	kind authority.DescriptionRefKind,
	raw string,
) (authority.DescriptionRef, error) {
	switch kind {
	case authority.DescriptionRefClaimID:
		return authority.NewClaimIDDescriptionRef(raw)
	case authority.DescriptionRefEpisteme:
		return authority.NewEpistemeDescriptionRef(raw)
	default:
		return authority.DescriptionRef{}, fmt.Errorf("authorization content DescriptionRef kind is invalid")
	}
}

func decodeStrictJSON(canonical []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(canonical))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode canonical JSON: %w", err)
	}
	if decoder.More() {
		return fmt.Errorf("canonical JSON has trailing material")
	}
	var trailing any
	err := decoder.Decode(&trailing)
	if err != io.EOF {
		return fmt.Errorf("canonical JSON has trailing material")
	}
	return nil
}

func formatTime(value time.Time) string {
	return value.Round(0).UTC().Format(time.RFC3339Nano)
}

func parseTime(raw string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil || formatTime(parsed) != raw {
		return time.Time{}, fmt.Errorf("authority time %q is not canonical", raw)
	}
	return parsed, nil
}

func (content ProjectTypeEnvHeadSelectionAuthorizationContent) DescriptionRef() authority.DescriptionRef {
	return content.descriptionRef
}

func (content ProjectTypeEnvHeadSelectionAuthorizationContent) Digest() authority.Digest {
	return content.digest
}

func (content ProjectTypeEnvHeadSelectionAuthorizationContent) Request() projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest {
	return content.request
}

func (content ProjectTypeEnvHeadSelectionAuthorizationContent) Project() projectidentity.ProjectID {
	return content.request.Project()
}

func (content ProjectTypeEnvHeadSelectionAuthorizationContent) Stage() projecttypeenvselection.ProjectTypeEnvStage {
	return content.stage
}

func (content ProjectTypeEnvHeadSelectionAuthorizationContent) JudgementContext() authority.BoundedContextRef {
	return content.context
}

func (content ProjectTypeEnvHeadSelectionAuthorizationContent) ValidityWindow() authority.TimeWindow {
	return content.validity
}

func (content ProjectTypeEnvHeadSelectionAuthorizationContent) Action() ProjectTypeEnvHeadSelectionAction {
	return content.intendedUpdate.Action()
}

func (content ProjectTypeEnvHeadSelectionAuthorizationContent) IntendedUpdate() ProjectTypeEnvHeadUpdate {
	return content.intendedUpdate
}

func (content ProjectTypeEnvHeadSelectionAuthorizationContent) CanonicalJSON() []byte {
	return append([]byte(nil), content.canonicalJSON...)
}

func (content ProjectTypeEnvHeadSelectionAuthorizationContent) Verify() error {
	rebuilt, err := sealProjectTypeEnvHeadSelectionAuthorizationContent(
		ProjectTypeEnvHeadSelectionAuthorizationContentInput{
			DescriptionRef:   content.descriptionRef,
			Request:          content.request,
			Stage:            content.stage,
			JudgementContext: content.context,
			ValidityWindow:   content.validity,
		},
		content.schema,
	)
	if err != nil {
		return err
	}
	if rebuilt.digest != content.digest || !bytes.Equal(rebuilt.canonicalJSON, content.canonicalJSON) {
		return fmt.Errorf("head-selection authorization content stored state differs from canonical bytes")
	}
	return nil
}

// ExactAgainst revalidates stored content against the non-serializable request
// witness that must be recovered again at a reliance boundary.
func (content ProjectTypeEnvHeadSelectionAuthorizationContent) ExactAgainst(
	request projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest,
) error {
	rebuilt, err := sealProjectTypeEnvHeadSelectionAuthorizationContent(
		ProjectTypeEnvHeadSelectionAuthorizationContentInput{
			DescriptionRef:   content.descriptionRef,
			Request:          request,
			Stage:            content.stage,
			JudgementContext: content.context,
			ValidityWindow:   content.validity,
		},
		content.schema,
	)
	if err != nil {
		return err
	}
	if rebuilt.digest != content.digest || !bytes.Equal(rebuilt.canonicalJSON, content.canonicalJSON) {
		return fmt.Errorf("authorization content does not match verified request")
	}
	return nil
}

func requestDigest(request projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest) string {
	return request.Ref().Digest().String()
}

func sameRequest(
	left projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest,
	right projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest,
) bool {
	return left.Ref() == right.Ref() && bytes.Equal(left.CanonicalBytes(), right.CanonicalBytes())
}

func sameStage(
	left projecttypeenvselection.ProjectTypeEnvStage,
	right projecttypeenvselection.ProjectTypeEnvStage,
) bool {
	return left.Ref() == right.Ref() && bytes.Equal(left.CanonicalBytes(), right.CanonicalBytes())
}
