package projecttypeenvselectionauthority

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/operatorrequest"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
)

const (
	hostRoutedSelectionPayloadSchema    = "haft.project-typeenv.host-routed-selection-payload/v1"
	hostRoutedSelectionResolutionSchema = "haft.project-typeenv.host-routed-selection-resolution/v1"
	hostRoutedSelectionResolutionDomain = "haft.project-typeenv.host-routed-selection-resolution/v1"
)

// HostRoutedSelectionPayload returns the exact effect payload a host must bind
// when it routes an unambiguous operator request to select a ProjectTypeEnv
// head. It is shared by the host adapter and the kernel verifier so neither
// side can silently broaden the selected request or reviewed content.
func HostRoutedSelectionPayload(
	request projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest,
	content ProjectTypeEnvHeadSelectionAuthorizationContent,
) ([]byte, error) {
	if err := request.Verify(); err != nil {
		return nil, fmt.Errorf("host-routed selection request: %w", err)
	}
	if err := content.Verify(); err != nil {
		return nil, fmt.Errorf("host-routed selection content: %w", err)
	}
	if content.Request().Ref() != request.Ref() ||
		content.Request().Ref().Digest() != request.Ref().Digest() {
		return nil, fmt.Errorf("host-routed selection content names another request")
	}
	projection := struct {
		Schema                    string `json:"schema"`
		SelectionRequestRef       string `json:"selection_request_ref"`
		SelectionRequestDigest    string `json:"selection_request_digest"`
		SelectionRequestCanonical []byte `json:"selection_request_canonical"`
		ContentRef                string `json:"content_ref"`
		ContentDigest             string `json:"content_digest"`
		ContentCanonical          []byte `json:"content_canonical"`
	}{
		Schema:                    hostRoutedSelectionPayloadSchema,
		SelectionRequestRef:       request.Ref().String(),
		SelectionRequestDigest:    request.Ref().Digest().String(),
		SelectionRequestCanonical: request.CanonicalBytes(),
		ContentRef:                content.DescriptionRef().String(),
		ContentDigest:             content.Digest().String(),
		ContentCanonical:          content.CanonicalJSON(),
	}
	payload, err := json.Marshal(projection)
	if err != nil {
		return nil, fmt.Errorf("encode host-routed selection payload: %w", err)
	}
	return payload, nil
}

type HostRoutedSelectionResolutionInput struct {
	OperatorRequest  operatorrequest.Request
	SelectionRequest projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest
	Content          ProjectTypeEnvHeadSelectionAuthorizationContent
	ProjectBinding   ProjectAuthorityContextBinding
	EvaluatedAt      time.Time
}

// HostRoutedSelectionResolution records that the kernel verified the exact
// request routed by the host against the current selection payload and project
// binding. It deliberately does not claim independent proof of U.SpeechAct.
type HostRoutedSelectionResolution struct {
	ref              ProjectTypeEnvHeadSelectionAuthorityResolutionRef
	digest           authority.Digest
	operatorRequest  operatorrequest.Request
	selectionRequest projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest
	content          ProjectTypeEnvHeadSelectionAuthorizationContent
	projectBinding   ProjectAuthorityContextBinding
	evaluatedAt      time.Time
	canonicalJSON    []byte
}

func SealHostRoutedSelectionResolution(
	input HostRoutedSelectionResolutionInput,
) (HostRoutedSelectionResolution, error) {
	payload, err := HostRoutedSelectionPayload(input.SelectionRequest, input.Content)
	if err != nil {
		return HostRoutedSelectionResolution{}, err
	}
	request := input.OperatorRequest
	if request.Provenance() != operatorrequest.HostRoutedOperatorRequest ||
		request.Effect() != operatorrequest.ProjectTypeEnvHeadSelect ||
		request.SubjectRef() != input.SelectionRequest.Ref().String() ||
		!request.MatchesPayload(payload) {
		return HostRoutedSelectionResolution{}, fmt.Errorf(
			"host-routed operator request does not bind the exact TypeEnv selection payload",
		)
	}
	projectRoot, err := authority.NewProjectRoot(input.ProjectBinding.Root().String())
	if err != nil || !input.ProjectBinding.ExactFor(
		input.SelectionRequest.Project(),
		projectRoot,
		input.Content.JudgementContext(),
	) {
		return HostRoutedSelectionResolution{}, fmt.Errorf(
			"host-routed TypeEnv selection project binding is not exact",
		)
	}
	evaluatedAt := input.EvaluatedAt.Round(0).UTC()
	if evaluatedAt.IsZero() {
		return HostRoutedSelectionResolution{}, fmt.Errorf(
			"host-routed TypeEnv selection evaluation time is required",
		)
	}
	if !input.Content.ValidityWindow().Contains(evaluatedAt) {
		return HostRoutedSelectionResolution{}, fmt.Errorf(
			"host-routed TypeEnv selection review is outside its validity window",
		)
	}
	projection := struct {
		Schema                 string `json:"schema"`
		Verdict                string `json:"verdict"`
		Provenance             string `json:"provenance"`
		OperatorRequestRef     string `json:"operator_request_ref"`
		OperatorRequestDigest  string `json:"operator_request_digest"`
		OperatorRequestSubject string `json:"operator_request_subject_ref"`
		PayloadDigest          string `json:"operator_request_payload_digest"`
		SelectionRequestRef    string `json:"selection_request_ref"`
		SelectionRequestDigest string `json:"selection_request_digest"`
		ContentRef             string `json:"content_ref"`
		ContentDigest          string `json:"content_digest"`
		ProjectBindingDigest   string `json:"project_binding_digest"`
		EvaluatedAt            string `json:"evaluated_at"`
		InterpretationLimit    string `json:"interpretation_limit"`
	}{
		Schema:                 hostRoutedSelectionResolutionSchema,
		Verdict:                "exact_host_routed_request_accepted",
		Provenance:             string(request.Provenance()),
		OperatorRequestRef:     request.Ref(),
		OperatorRequestDigest:  request.Digest(),
		OperatorRequestSubject: request.SubjectRef(),
		PayloadDigest:          request.PayloadDigest(),
		SelectionRequestRef:    input.SelectionRequest.Ref().String(),
		SelectionRequestDigest: input.SelectionRequest.Ref().Digest().String(),
		ContentRef:             input.Content.DescriptionRef().String(),
		ContentDigest:          input.Content.Digest().String(),
		ProjectBindingDigest:   input.ProjectBinding.Digest().String(),
		EvaluatedAt:            formatTime(evaluatedAt),
		InterpretationLimit:    "host_routing_provenance_only;_not_independent_speech_act_proof",
	}
	canonical, err := json.Marshal(projection)
	if err != nil {
		return HostRoutedSelectionResolution{}, err
	}
	digest, err := digestCanonical(hostRoutedSelectionResolutionDomain, canonical)
	if err != nil {
		return HostRoutedSelectionResolution{}, err
	}
	return HostRoutedSelectionResolution{
		ref:              ProjectTypeEnvHeadSelectionAuthorityResolutionRef{digest: digest},
		digest:           digest,
		operatorRequest:  request,
		selectionRequest: input.SelectionRequest,
		content:          input.Content,
		projectBinding:   input.ProjectBinding,
		evaluatedAt:      evaluatedAt,
		canonicalJSON:    canonical,
	}, nil
}

func (resolution HostRoutedSelectionResolution) Verify(
	input HostRoutedSelectionResolutionInput,
) error {
	rebuilt, err := SealHostRoutedSelectionResolution(input)
	if err != nil {
		return err
	}
	if rebuilt.ref != resolution.ref ||
		rebuilt.digest != resolution.digest ||
		!bytes.Equal(rebuilt.canonicalJSON, resolution.canonicalJSON) {
		return fmt.Errorf("host-routed TypeEnv selection resolution differs from exact input")
	}
	return nil
}

func (resolution HostRoutedSelectionResolution) Ref() ProjectTypeEnvHeadSelectionAuthorityResolutionRef {
	return resolution.ref
}

func (resolution HostRoutedSelectionResolution) Digest() authority.Digest {
	return resolution.digest
}

func (resolution HostRoutedSelectionResolution) OperatorRequest() operatorrequest.Request {
	return resolution.operatorRequest
}

func (resolution HostRoutedSelectionResolution) SelectionRequest() projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest {
	return resolution.selectionRequest
}

func (resolution HostRoutedSelectionResolution) Content() ProjectTypeEnvHeadSelectionAuthorizationContent {
	return resolution.content
}

func (resolution HostRoutedSelectionResolution) ProjectBinding() ProjectAuthorityContextBinding {
	return resolution.projectBinding
}

func (resolution HostRoutedSelectionResolution) EvaluatedAt() time.Time {
	return resolution.evaluatedAt
}

func (resolution HostRoutedSelectionResolution) CanonicalJSON() []byte {
	return append([]byte(nil), resolution.canonicalJSON...)
}
