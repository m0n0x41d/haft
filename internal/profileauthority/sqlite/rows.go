package sqlite

type authorizationContentJSON struct {
	Schema                  string `json:"schema"`
	Ref                     string `json:"authorization_content_ref"`
	ProjectRoot             string `json:"project_root"`
	ActionKind              string `json:"action_kind"`
	ProfileAuthorRef        string `json:"profile_author_role_assignment_ref"`
	ProfileAuthorDigest     string `json:"profile_author_role_assignment_digest"`
	MethodDescriptionRef    string `json:"method_description_ref"`
	MethodDescriptionDigest string `json:"method_description_digest"`
	MethodContractRef       string `json:"method_contract_ref"`
	MethodContractDigest    string `json:"method_contract_digest"`
	ClassifierVersion       string `json:"classifier_version"`
	PolicyVersion           string `json:"policy_version"`
	SessionRef              string `json:"session_ref"`
	AllowedWorkFrom         string `json:"allowed_work_from"`
	AllowedWorkUntil        string `json:"allowed_work_until"`
	BasisObservationFrom    string `json:"basis_observation_from"`
	BasisObservationUntil   string `json:"basis_observation_until"`
	AuthorizationValidFrom  string `json:"authorization_valid_from"`
	AuthorizationValidUntil string `json:"authorization_valid_until"`
	SingleUseKey            string `json:"single_use_key"`
}

type contentRow struct {
	ref                     string
	digest                  string
	projectRoot             string
	actionKind              string
	profileAuthorRef        string
	profileAuthorDigest     string
	methodDescriptionRef    string
	methodDescriptionDigest string
	methodContractRef       string
	methodContractDigest    string
	classifierVersion       string
	policyVersion           string
	sessionRef              string
	allowedWorkFrom         string
	allowedWorkUntil        string
	basisObservationFrom    string
	basisObservationUntil   string
	authorizationValidFrom  string
	authorizationValidUntil string
	singleUseKey            string
	canonical               string
	recordedAt              string
}

func (row *contentRow) scanTargets() []any {
	return []any{
		&row.ref,
		&row.digest,
		&row.projectRoot,
		&row.actionKind,
		&row.profileAuthorRef,
		&row.profileAuthorDigest,
		&row.methodDescriptionRef,
		&row.methodDescriptionDigest,
		&row.methodContractRef,
		&row.methodContractDigest,
		&row.classifierVersion,
		&row.policyVersion,
		&row.sessionRef,
		&row.allowedWorkFrom,
		&row.allowedWorkUntil,
		&row.basisObservationFrom,
		&row.basisObservationUntil,
		&row.authorizationValidFrom,
		&row.authorizationValidUntil,
		&row.singleUseKey,
		&row.canonical,
		&row.recordedAt,
	}
}

type preparedAuthorizationJSON struct {
	Schema                   string `json:"schema"`
	ContentRef               string `json:"authorization_content_ref"`
	ContentDigest            string `json:"authorization_content_digest"`
	PermissionRef            string `json:"permission_ref"`
	SpeechActRef             string `json:"speech_act_ref"`
	CaptureRef               string `json:"capture_carrier_ref"`
	SpeechActSession         string `json:"speech_act_session_ref"`
	ClaimScopeRef            string `json:"claim_scope_ref"`
	EnactabilityPredicateRef string `json:"enactability_predicate_ref"`
	EvidenceClaimRef         string `json:"evidence_claim_ref"`
	CarrierClassRef          string `json:"carrier_class_ref"`
	VerifierIdentity         string `json:"verifier_identity"`
	VerifierVersion          string `json:"verifier_version"`
	VerificationPolicyRef    string `json:"verification_policy_ref"`
	VerificationPolicyDigest string `json:"verification_policy_digest"`
	BasisRef                 string `json:"basis_ref"`
	ContextPolicyRef         string `json:"context_policy_ref"`
	ContextPolicyDigest      string `json:"context_policy_digest"`
	SpeechActIntentDigest    string `json:"speech_act_intent_digest"`
}

type preparationRow struct {
	digest                   string
	projectRoot              string
	contentRef               string
	contentDigest            string
	permissionRef            string
	speechActRef             string
	captureRef               string
	speechActSession         string
	claimScopeRef            string
	enactabilityPredicateRef string
	evidenceClaimRef         string
	carrierClassRef          string
	verifierIdentity         string
	verifierVersion          string
	verificationPolicyRef    string
	verificationPolicyDigest string
	basisRef                 string
	contextPolicyRef         string
	contextPolicyDigest      string
	speechActIntentDigest    string
	canonical                string
	recordedAt               string
}

func (row *preparationRow) scanTargets() []any {
	return []any{
		&row.digest,
		&row.projectRoot,
		&row.contentRef,
		&row.contentDigest,
		&row.permissionRef,
		&row.speechActRef,
		&row.captureRef,
		&row.speechActSession,
		&row.claimScopeRef,
		&row.enactabilityPredicateRef,
		&row.evidenceClaimRef,
		&row.carrierClassRef,
		&row.verifierIdentity,
		&row.verifierVersion,
		&row.verificationPolicyRef,
		&row.verificationPolicyDigest,
		&row.basisRef,
		&row.contextPolicyRef,
		&row.contextPolicyDigest,
		&row.speechActIntentDigest,
		&row.canonical,
		&row.recordedAt,
	}
}

type permissionJSON struct {
	Schema                   string   `json:"schema"`
	PermissionRef            string   `json:"permission_ref"`
	ProjectRoot              string   `json:"project_root"`
	SubjectRef               string   `json:"subject_role_assignment_ref"`
	SubjectDigest            string   `json:"subject_role_assignment_digest"`
	Modality                 string   `json:"modality"`
	ActionKind               string   `json:"action_kind"`
	ClaimScopeRef            string   `json:"claim_scope_ref"`
	BoundedContextRef        string   `json:"bounded_context_ref"`
	ValidFrom                string   `json:"valid_from"`
	ValidUntil               string   `json:"valid_until"`
	Referents                []string `json:"referents"`
	AuthorizationContentRef  string   `json:"authorization_content_ref"`
	AuthorizationContentDig  string   `json:"authorization_content_digest"`
	MethodDescriptionRef     string   `json:"method_description_ref"`
	MethodDescriptionDigest  string   `json:"method_description_digest"`
	EnactabilityPredicateRef string   `json:"enactability_predicate_ref"`
	EvidenceClaimRefs        []string `json:"adjudication_evidence_claim_refs"`
	CarrierClassRefs         []string `json:"adjudication_carrier_class_refs"`
	VerifierIdentity         string   `json:"adjudication_verifier_identity"`
	VerifierVersion          string   `json:"adjudication_verifier_version"`
	EvaluationPolicyRef      string   `json:"adjudication_evaluation_policy_ref"`
	EvaluationPolicyDigest   string   `json:"adjudication_evaluation_policy_digest"`
	SourceSpeechActRef       string   `json:"source_speech_act_ref"`
	SourceSpeechActDigest    string   `json:"source_speech_act_digest"`
	ContextPolicyRef         string   `json:"context_policy_ref"`
	ContextPolicyDigest      string   `json:"context_policy_digest"`
	CaptureCarrierRef        string   `json:"instituting_terminal_capture_carrier_ref"`
	CaptureCarrierDigest     string   `json:"instituting_terminal_capture_carrier_digest"`
}

type permissionRow struct {
	ref                      string
	digest                   string
	preparedDigest           string
	projectRoot              string
	permissionKind           string
	subjectRef               string
	subjectDigest            string
	modality                 string
	actionKind               string
	claimScopeRef            string
	boundedContextRef        string
	validFrom                string
	validUntil               string
	referentsJSON            string
	contentRef               string
	contentDigest            string
	methodDescriptionRef     string
	methodDescriptionDigest  string
	enactabilityPredicateRef string
	evidenceClaimRefsJSON    string
	carrierClassRefsJSON     string
	verifierIdentity         string
	verifierVersion          string
	evaluationPolicyRef      string
	evaluationPolicyDigest   string
	sourceSpeechActRef       string
	sourceSpeechActDigest    string
	contextPolicyRef         string
	contextPolicyDigest      string
	captureRef               string
	captureDigest            string
	canonical                string
	recordedAt               string
}

func (row *permissionRow) scanTargets() []any {
	return []any{
		&row.ref,
		&row.digest,
		&row.preparedDigest,
		&row.projectRoot,
		&row.permissionKind,
		&row.subjectRef,
		&row.subjectDigest,
		&row.modality,
		&row.actionKind,
		&row.claimScopeRef,
		&row.boundedContextRef,
		&row.validFrom,
		&row.validUntil,
		&row.referentsJSON,
		&row.contentRef,
		&row.contentDigest,
		&row.methodDescriptionRef,
		&row.methodDescriptionDigest,
		&row.enactabilityPredicateRef,
		&row.evidenceClaimRefsJSON,
		&row.carrierClassRefsJSON,
		&row.verifierIdentity,
		&row.verifierVersion,
		&row.evaluationPolicyRef,
		&row.evaluationPolicyDigest,
		&row.sourceSpeechActRef,
		&row.sourceSpeechActDigest,
		&row.contextPolicyRef,
		&row.contextPolicyDigest,
		&row.captureRef,
		&row.captureDigest,
		&row.canonical,
		&row.recordedAt,
	}
}

type effectJSON struct {
	Schema           string `json:"schema"`
	ProjectRoot      string `json:"project_root"`
	SpeechActRef     string `json:"speech_act_ref"`
	SpeechActDigest  string `json:"speech_act_digest"`
	PermissionRef    string `json:"permission_ref"`
	PermissionDigest string `json:"permission_digest"`
}

type effectRow struct {
	digest           string
	projectRoot      string
	speechActRef     string
	speechActDigest  string
	permissionRef    string
	permissionDigest string
	canonical        string
	recordedAt       string
}

func (row *effectRow) scanTargets() []any {
	return []any{
		&row.digest,
		&row.projectRoot,
		&row.speechActRef,
		&row.speechActDigest,
		&row.permissionRef,
		&row.permissionDigest,
		&row.canonical,
		&row.recordedAt,
	}
}

type basisJSON struct {
	Schema              string `json:"schema"`
	BasisRef            string `json:"basis_ref"`
	ProjectRoot         string `json:"project_root"`
	SpeechActRef        string `json:"speech_act_ref"`
	SpeechActDigest     string `json:"speech_act_digest"`
	ContentRef          string `json:"authorization_content_ref"`
	ContentDigest       string `json:"authorization_content_digest"`
	PermissionRef       string `json:"permission_ref"`
	PermissionDigest    string `json:"permission_digest"`
	ContextPolicyRef    string `json:"context_policy_ref"`
	ContextPolicyDigest string `json:"context_policy_digest"`
}

type basisRow struct {
	ref                    string
	digest                 string
	projectRoot            string
	speechActRef           string
	speechActDigest        string
	contentRef             string
	contentDigest          string
	permissionRef          string
	permissionDigest       string
	contextPolicyRef       string
	contextPolicyDigest    string
	institutedEffectDigest string
	canonical              string
	recordedAt             string
}

func (row *basisRow) scanTargets() []any {
	return []any{
		&row.ref,
		&row.digest,
		&row.projectRoot,
		&row.speechActRef,
		&row.speechActDigest,
		&row.contentRef,
		&row.contentDigest,
		&row.permissionRef,
		&row.permissionDigest,
		&row.contextPolicyRef,
		&row.contextPolicyDigest,
		&row.institutedEffectDigest,
		&row.canonical,
		&row.recordedAt,
	}
}
