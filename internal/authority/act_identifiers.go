package authority

type BoundedContextRef struct{ value string }

func NewBoundedContextRef(raw string) (BoundedContextRef, error) {
	value, err := parseAuthorityReference("bounded-context ref", raw)
	return BoundedContextRef{value: value}, err
}

func (value BoundedContextRef) String() string { return value.value }
func (value BoundedContextRef) valid() bool    { return validAuthorityReference(value.value) }

type SpeechActTypeRef struct{ value string }

func NewSpeechActTypeRef(raw string) (SpeechActTypeRef, error) {
	value, err := parseAuthorityReference("SpeechAct type ref", raw)
	return SpeechActTypeRef{value: value}, err
}

func (value SpeechActTypeRef) String() string { return value.value }
func (value SpeechActTypeRef) valid() bool    { return validAuthorityReference(value.value) }

type MethodRef struct{ value string }

func NewMethodRef(raw string) (MethodRef, error) {
	value, err := parseAuthorityReference("Method ref", raw)
	return MethodRef{value: value}, err
}

func (value MethodRef) String() string { return value.value }
func (value MethodRef) valid() bool    { return validAuthorityReference(value.value) }

type SystemRef struct{ value string }

func NewSystemRef(raw string) (SystemRef, error) {
	value, err := parseAuthorityReference("System ref", raw)
	return SystemRef{value: value}, err
}

func (value SystemRef) String() string { return value.value }
func (value SystemRef) valid() bool    { return validAuthorityReference(value.value) }

type StatePlaneRef struct{ value string }

func NewStatePlaneRef(raw string) (StatePlaneRef, error) {
	value, err := parseAuthorityReference("state-plane ref", raw)
	return StatePlaneRef{value: value}, err
}

func (value StatePlaneRef) String() string { return value.value }
func (value StatePlaneRef) valid() bool    { return validAuthorityReference(value.value) }

type DeltaPredicateRef struct{ value string }

func NewDeltaPredicateRef(raw string) (DeltaPredicateRef, error) {
	value, err := parseAuthorityReference("delta-predicate ref", raw)
	return DeltaPredicateRef{value: value}, err
}

func (value DeltaPredicateRef) String() string { return value.value }
func (value DeltaPredicateRef) valid() bool    { return validAuthorityReference(value.value) }

type WorkResourceRef struct{ value string }

func NewWorkResourceRef(raw string) (WorkResourceRef, error) {
	value, err := parseAuthorityReference("Work resource ref", raw)
	return WorkResourceRef{value: value}, err
}

func (value WorkResourceRef) String() string { return value.value }
func (value WorkResourceRef) valid() bool    { return validAuthorityReference(value.value) }

type AffectedRef struct{ value string }

func NewAffectedRef(raw string) (AffectedRef, error) {
	value, err := parseAuthorityReference("affected ref", raw)
	return AffectedRef{value: value}, err
}

func (value AffectedRef) String() string { return value.value }
func (value AffectedRef) valid() bool    { return validAuthorityReference(value.value) }

type WorkOutcomeRef struct{ value string }

func NewWorkOutcomeRef(raw string) (WorkOutcomeRef, error) {
	value, err := parseAuthorityReference("Work outcome ref", raw)
	return WorkOutcomeRef{value: value}, err
}

func (value WorkOutcomeRef) String() string { return value.value }
func (value WorkOutcomeRef) valid() bool    { return validAuthorityReference(value.value) }

type UtteranceRef struct{ value string }

func NewUtteranceRef(raw string) (UtteranceRef, error) {
	value, err := parseAuthorityReference("utterance-description ref", raw)
	return UtteranceRef{value: value}, err
}

func (value UtteranceRef) String() string { return value.value }
func (value UtteranceRef) valid() bool    { return validAuthorityReference(value.value) }

type CarrierRef struct{ value string }

func NewCarrierRef(raw string) (CarrierRef, error) {
	value, err := parseAuthorityReference("carrier ref", raw)
	return CarrierRef{value: value}, err
}

func (value CarrierRef) String() string { return value.value }
func (value CarrierRef) valid() bool    { return validAuthorityReference(value.value) }

// VerificationEvidenceRelationRef identifies the relation by which later
// evidence may adjudicate whether a permission was used within its scope. It
// is deliberately separate from the permission's domain referents.
type VerificationEvidenceRelationRef struct{ value string }

func NewVerificationEvidenceRelationRef(raw string) (VerificationEvidenceRelationRef, error) {
	value, err := parseAuthorityReference("verification-evidence relation ref", raw)
	return VerificationEvidenceRelationRef{value: value}, err
}

func (value VerificationEvidenceRelationRef) String() string { return value.value }
func (value VerificationEvidenceRelationRef) valid() bool {
	return validAuthorityReference(value.value)
}

// VerificationCarrierExpectationRef identifies the carrier contract expected
// by adjudication. The actual terminal carrier is bound separately after the
// SpeechAct occurs.
type VerificationCarrierExpectationRef struct{ value string }

func NewVerificationCarrierExpectationRef(raw string) (VerificationCarrierExpectationRef, error) {
	value, err := parseAuthorityReference("verification-carrier expectation ref", raw)
	return VerificationCarrierExpectationRef{value: value}, err
}

func (value VerificationCarrierExpectationRef) String() string { return value.value }
func (value VerificationCarrierExpectationRef) valid() bool {
	return validAuthorityReference(value.value)
}

type SpeechActReviewSubjectRef struct{ value string }

func NewSpeechActReviewSubjectRef(raw string) (SpeechActReviewSubjectRef, error) {
	value, err := parseAuthorityReference("SpeechAct review-subject ref", raw)
	return SpeechActReviewSubjectRef{value: value}, err
}

func (value SpeechActReviewSubjectRef) String() string { return value.value }
func (value SpeechActReviewSubjectRef) valid() bool {
	return validAuthorityReference(value.value)
}

type InstitutedObjectRef struct{ value string }

func NewInstitutedObjectRef(raw string) (InstitutedObjectRef, error) {
	value, err := parseAuthorityReference("instituted-object ref", raw)
	return InstitutedObjectRef{value: value}, err
}

func (value InstitutedObjectRef) String() string { return value.value }
func (value InstitutedObjectRef) valid() bool    { return validAuthorityReference(value.value) }

type InstitutionalEffectRuleRef struct{ value string }

func NewInstitutionalEffectRuleRef(raw string) (InstitutionalEffectRuleRef, error) {
	value, err := parseAuthorityReference("institutional-effect rule ref", raw)
	return InstitutionalEffectRuleRef{value: value}, err
}

func (value InstitutionalEffectRuleRef) String() string { return value.value }
func (value InstitutionalEffectRuleRef) valid() bool {
	return validAuthorityReference(value.value)
}

type MethodProcedureRef struct{ value string }

func NewMethodProcedureRef(raw string) (MethodProcedureRef, error) {
	value, err := parseAuthorityReference("Method procedure ref", raw)
	return MethodProcedureRef{value: value}, err
}

func (value MethodProcedureRef) String() string { return value.value }
func (value MethodProcedureRef) valid() bool    { return validAuthorityReference(value.value) }

type ClaimScopeRef struct{ value string }

func NewClaimScopeRef(raw string) (ClaimScopeRef, error) {
	value, err := parseAuthorityReference("claim-scope ref", raw)
	return ClaimScopeRef{value: value}, err
}

func (value ClaimScopeRef) String() string { return value.value }
func (value ClaimScopeRef) valid() bool    { return validAuthorityReference(value.value) }

type WorkParameterBinding struct {
	name  string
	value string
}

func NewWorkParameterBinding(name string, value string) (WorkParameterBinding, error) {
	canonicalName, err := parseAuthorityReference("Work parameter name", name)
	if err != nil {
		return WorkParameterBinding{}, err
	}
	canonicalValue, err := parseAuthorityReference("Work parameter value", value)
	if err != nil {
		return WorkParameterBinding{}, err
	}
	return WorkParameterBinding{name: canonicalName, value: canonicalValue}, nil
}

func (binding WorkParameterBinding) Name() string  { return binding.name }
func (binding WorkParameterBinding) Value() string { return binding.value }
func (binding WorkParameterBinding) valid() bool {
	return validAuthorityReference(binding.name) && validAuthorityReference(binding.value)
}
