package authority

import (
	"encoding/json"
	"fmt"
	"time"
)

const (
	authorityBasisPresentationDigestDomain = "haft.authority.basis-presentation/v2"
	authorityBasisResolutionDigestDomain   = "haft.authority.basis-resolution/v2"
)

type canonicalAuthorityBasisPresentation struct {
	id                     PresentationID
	digest                 Digest
	projectRoot            ProjectRoot
	contextPolicyRef       ContextPolicyRef
	contextPolicyDigest    Digest
	contentRef             AuthorizationContentRef
	contentDigest          Digest
	captureRef             CarrierRef
	captureDigest          Digest
	authorizerRef          RoleAssignmentRef
	authorizerDigest       Digest
	speechActRef           SpeechActRef
	speechActDigest        Digest
	permissionRef          PermissionRef
	permissionDigest       Digest
	institutedEffectDigest Digest
	legacyProjectionDigest Digest
	canonicalJSON          []byte
}

type canonicalAuthorityBasisResolution struct {
	id                       AuthorityResolutionID
	digest                   Digest
	projectRoot              ProjectRoot
	presentationID           PresentationID
	presentationDigest       Digest
	verifierIdentity         VerifierIdentity
	verifierVersion          VerifierVersion
	verificationPolicyRef    VerificationPolicyRef
	verificationPolicyDigest Digest
	resolvedAt               time.Time
	authorityValidFrom       time.Time
	validUntil               time.Time
	legacyProjectionDigest   Digest
	canonicalJSON            []byte
}

func newCanonicalAuthorityBasisGraph(
	act VerifiedAuthorityAct,
	legacyPresentation canonicalPresentation,
	legacyResolution canonicalAuthorityResolution,
) (canonicalAuthorityBasisPresentation, canonicalAuthorityBasisResolution, error) {
	if act.state == nil ||
		validateCanonicalPresentation(legacyPresentation) != nil ||
		validateCanonicalAuthorityResolution(legacyResolution, legacyPresentation) != nil {
		return canonicalAuthorityBasisPresentation{}, canonicalAuthorityBasisResolution{}, fmt.Errorf("authority basis graph requires a verified act")
	}
	state := act.state
	policy := state.intent.state.contextPolicy.state
	content := state.intent.state.content.state
	capture := state.capture.state
	authorizer := state.authorizer.state
	speechAct := state.speechAct.state
	permission := state.permission.state
	effect := state.effect.state
	presentationProjection := struct {
		Schema                  string `json:"schema"`
		PresentationID          string `json:"presentation_id"`
		ProjectRoot             string `json:"project_root"`
		ContextPolicyRef        string `json:"context_policy_ref"`
		ContextPolicyDigest     string `json:"context_policy_digest"`
		AuthorizationContentRef string `json:"authorization_content_ref"`
		AuthorizationContentDig string `json:"authorization_content_digest"`
		TerminalCaptureRef      string `json:"terminal_capture_carrier_ref"`
		TerminalCaptureDigest   string `json:"terminal_capture_carrier_digest"`
		AuthorizerRef           string `json:"authorizer_role_assignment_ref"`
		AuthorizerDigest        string `json:"authorizer_role_assignment_digest"`
		SpeechActRef            string `json:"speech_act_ref"`
		SpeechActDigest         string `json:"speech_act_digest"`
		PermissionRef           string `json:"permission_ref"`
		PermissionDigest        string `json:"permission_digest"`
		InstitutedEffectDigest  string `json:"instituted_effect_digest"`
		LegacyProjectionDigest  string `json:"legacy_projection_digest"`
	}{
		Schema:                  "haft.authority.basis-presentation/v2",
		PresentationID:          legacyPresentation.id.String(),
		ProjectRoot:             content.envelope.projectRoot.String(),
		ContextPolicyRef:        policy.ref.String(),
		ContextPolicyDigest:     policy.digest.String(),
		AuthorizationContentRef: content.ref.String(),
		AuthorizationContentDig: content.digest.String(),
		TerminalCaptureRef:      capture.carrierRef.String(),
		TerminalCaptureDigest:   capture.carrierDigest.String(),
		AuthorizerRef:           authorizer.ref.String(),
		AuthorizerDigest:        authorizer.digest.String(),
		SpeechActRef:            speechAct.ref.String(),
		SpeechActDigest:         speechAct.digest.String(),
		PermissionRef:           permission.ref.String(),
		PermissionDigest:        permission.digest.String(),
		InstitutedEffectDigest:  effect.digest.String(),
		LegacyProjectionDigest:  legacyPresentation.digest.String(),
	}
	presentationJSON, err := json.Marshal(presentationProjection)
	if err != nil {
		return canonicalAuthorityBasisPresentation{}, canonicalAuthorityBasisResolution{}, fmt.Errorf("encode authority basis presentation: %w", err)
	}
	presentationWriter := newAuthorityDigestWriter(authorityBasisPresentationDigestDomain)
	presentationWriter.add(string(presentationJSON))
	presentation := canonicalAuthorityBasisPresentation{
		id:                     legacyPresentation.id,
		digest:                 presentationWriter.digest(),
		projectRoot:            content.envelope.projectRoot,
		contextPolicyRef:       policy.ref,
		contextPolicyDigest:    policy.digest,
		contentRef:             content.ref,
		contentDigest:          content.digest,
		captureRef:             capture.carrierRef,
		captureDigest:          capture.carrierDigest,
		authorizerRef:          authorizer.ref,
		authorizerDigest:       authorizer.digest,
		speechActRef:           speechAct.ref,
		speechActDigest:        speechAct.digest,
		permissionRef:          permission.ref,
		permissionDigest:       permission.digest,
		institutedEffectDigest: effect.digest,
		legacyProjectionDigest: legacyPresentation.digest,
		canonicalJSON:          presentationJSON,
	}
	resolutionProjection := struct {
		Schema                   string `json:"schema"`
		ResolutionID             string `json:"authority_resolution_id"`
		ProjectRoot              string `json:"project_root"`
		PresentationID           string `json:"presentation_id"`
		PresentationDigest       string `json:"presentation_digest"`
		VerifierIdentity         string `json:"verifier_identity"`
		VerifierVersion          string `json:"verifier_version"`
		VerificationPolicyRef    string `json:"verification_policy_ref"`
		VerificationPolicyDigest string `json:"verification_policy_digest"`
		ResolvedAt               string `json:"resolved_at"`
		AuthorityValidFrom       string `json:"authority_valid_from"`
		ValidUntil               string `json:"valid_until"`
		LegacyProjectionDigest   string `json:"legacy_projection_digest"`
	}{
		Schema:                   "haft.authority.basis-resolution/v2",
		ResolutionID:             legacyResolution.id.String(),
		ProjectRoot:              content.envelope.projectRoot.String(),
		PresentationID:           presentation.id.String(),
		PresentationDigest:       presentation.digest.String(),
		VerifierIdentity:         state.intent.state.verifierIdentity.String(),
		VerifierVersion:          state.intent.state.verifierVersion.String(),
		VerificationPolicyRef:    state.intent.state.verificationPolicyRef.String(),
		VerificationPolicyDigest: state.intent.state.verificationPolicyDigest.String(),
		ResolvedAt:               formatAuthorityTime(legacyResolution.resolvedAt),
		AuthorityValidFrom:       formatAuthorityTime(state.intent.state.resolutionWindow.from),
		ValidUntil:               formatAuthorityTime(legacyResolution.validUntil),
		LegacyProjectionDigest:   legacyResolution.digest.String(),
	}
	resolutionJSON, err := json.Marshal(resolutionProjection)
	if err != nil {
		return canonicalAuthorityBasisPresentation{}, canonicalAuthorityBasisResolution{}, fmt.Errorf("encode authority basis resolution: %w", err)
	}
	resolutionWriter := newAuthorityDigestWriter(authorityBasisResolutionDigestDomain)
	resolutionWriter.add(string(resolutionJSON))
	resolution := canonicalAuthorityBasisResolution{
		id:                       legacyResolution.id,
		digest:                   resolutionWriter.digest(),
		projectRoot:              content.envelope.projectRoot,
		presentationID:           presentation.id,
		presentationDigest:       presentation.digest,
		verifierIdentity:         state.intent.state.verifierIdentity,
		verifierVersion:          state.intent.state.verifierVersion,
		verificationPolicyRef:    state.intent.state.verificationPolicyRef,
		verificationPolicyDigest: state.intent.state.verificationPolicyDigest,
		resolvedAt:               legacyResolution.resolvedAt,
		authorityValidFrom:       state.intent.state.resolutionWindow.from,
		validUntil:               legacyResolution.validUntil,
		legacyProjectionDigest:   legacyResolution.digest,
		canonicalJSON:            resolutionJSON,
	}
	return presentation, resolution, nil
}

type checkedAuthorityBasis struct {
	act                VerifiedAuthorityAct
	presentation       canonicalAuthorityBasisPresentation
	resolution         canonicalAuthorityBasisResolution
	legacyPresentation canonicalPresentation
	legacyResolution   canonicalAuthorityResolution
}

func newCheckedAuthorityBasis(
	act VerifiedAuthorityAct,
	checkedAt time.Time,
) (checkedAuthorityBasis, error) {
	if !act.valid() {
		return checkedAuthorityBasis{}, fmt.Errorf("checked authority basis requires a verified act")
	}
	legacyPresentation, legacyResolution, err := authorityPairFromCheckedAct(
		act.state.intent,
		act.state.basis,
		act.state.capture.state.endedAt,
		checkedAt,
	)
	if err != nil {
		return checkedAuthorityBasis{}, err
	}
	presentation, resolution, err := newCanonicalAuthorityBasisGraph(
		act,
		legacyPresentation,
		legacyResolution,
	)
	if err != nil {
		return checkedAuthorityBasis{}, err
	}
	return checkedAuthorityBasis{
		act:                act,
		presentation:       presentation,
		resolution:         resolution,
		legacyPresentation: legacyPresentation,
		legacyResolution:   legacyResolution,
	}, nil
}
