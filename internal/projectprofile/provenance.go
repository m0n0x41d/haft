package projectprofile

import (
	"fmt"
	"time"
)

type ClassifierVersion struct {
	value string
}

func NewClassifierVersion(raw string) (ClassifierVersion, error) {
	value, err := requireText("classifier version", raw)
	if err != nil {
		return ClassifierVersion{}, err
	}
	return ClassifierVersion{value: value}, nil
}

func (version ClassifierVersion) String() string {
	return version.value
}

func (version ClassifierVersion) valid() bool {
	_, err := requireText("classifier version", version.value)
	return err == nil
}

type PolicyVersion struct {
	value string
}

func NewPolicyVersion(raw string) (PolicyVersion, error) {
	value, err := requireText("profile policy version", raw)
	if err != nil {
		return PolicyVersion{}, err
	}
	return PolicyVersion{value: value}, nil
}

func (version PolicyVersion) String() string {
	return version.value
}

func (version PolicyVersion) valid() bool {
	_, err := requireText("profile policy version", version.value)
	return err == nil
}

type ObservedBasis struct {
	source      string
	observation string
}

func NewObservedBasis(source, observation string) (ObservedBasis, error) {
	parsedSource, err := requireText("profile declaration basis source", source)
	if err != nil {
		return ObservedBasis{}, err
	}
	parsedObservation, err := requireText("profile declaration basis observation", observation)
	if err != nil {
		return ObservedBasis{}, err
	}
	return ObservedBasis{source: parsedSource, observation: parsedObservation}, nil
}

func (basis ObservedBasis) Source() string {
	return basis.source
}

func (basis ObservedBasis) Observation() string {
	return basis.observation
}

func (basis ObservedBasis) valid() bool {
	_, sourceErr := requireText("profile declaration basis source", basis.source)
	_, observationErr := requireText("profile declaration basis observation", basis.observation)
	return sourceErr == nil && observationErr == nil
}

type ObservationWindow struct {
	executionContextRef string
	from                time.Time
	until               time.Time
}

func NewObservationWindow(
	executionContextRef string,
	from time.Time,
	until time.Time,
) (ObservationWindow, error) {
	parsedContextRef, err := requireText("observation execution-context ref", executionContextRef)
	if err != nil {
		return ObservationWindow{}, err
	}
	if from.IsZero() || !until.After(from) {
		return ObservationWindow{}, fmt.Errorf("observation window requires non-zero from and later until")
	}
	return ObservationWindow{
		executionContextRef: parsedContextRef,
		from:                from,
		until:               until,
	}, nil
}

func (window ObservationWindow) ExecutionContextRef() string {
	return window.executionContextRef
}

func (window ObservationWindow) From() time.Time {
	return window.from
}

func (window ObservationWindow) Until() time.Time {
	return window.until
}

func (window ObservationWindow) Contains(instant time.Time) bool {
	return !instant.Before(window.from) && instant.Before(window.until)
}

func (window ObservationWindow) valid() bool {
	_, contextErr := requireText("observation execution-context ref", window.executionContextRef)
	return contextErr == nil && !window.from.IsZero() && window.until.After(window.from)
}

// ProfileDeclarationReceipt is finalized by Haft only after admission. It is
// legacy provenance only. Reading this record does not establish final-v1
// admission, authority, Work, or a binding ConfiguredProjectProfileV1.
type ProfileDeclarationReceipt interface {
	profileDeclarationReceiptVariant()
	DeclarationAuthorityBasisRef() string
	ScopePayloadDigest() ContentDigest
	ObservedBasisDigest() ContentDigest
	CarrierRevision() CarrierRevision
}

type OperatorDeclaredRecord struct {
	declarationAuthorityBasisRef string
	declarationWorkRef           string
	projectRoot                  string
	scopePayloadDigest           ContentDigest
	observedBasisDigest          ContentDigest
	observationWindow            ObservationWindow
	carrierRevision              CarrierRevision
}

type OperatorDeclaredRecordBuilder struct {
	value OperatorDeclaredRecord
}

func NewOperatorDeclaredRecordBuilder(
	authorityBasisRef string,
	declarationWorkRef string,
) OperatorDeclaredRecordBuilder {
	return OperatorDeclaredRecordBuilder{value: OperatorDeclaredRecord{
		declarationAuthorityBasisRef: authorityBasisRef,
		declarationWorkRef:           declarationWorkRef,
	}}
}

func (builder OperatorDeclaredRecordBuilder) ForProject(root string) OperatorDeclaredRecordBuilder {
	builder.value.projectRoot = root
	return builder
}

func (builder OperatorDeclaredRecordBuilder) ForScopePayload(
	digest ContentDigest,
) OperatorDeclaredRecordBuilder {
	builder.value.scopePayloadDigest = digest
	return builder
}

func (builder OperatorDeclaredRecordBuilder) ForObservedBasis(
	digest ContentDigest,
) OperatorDeclaredRecordBuilder {
	builder.value.observedBasisDigest = digest
	return builder
}

func (builder OperatorDeclaredRecordBuilder) ObservedWithin(
	window ObservationWindow,
) OperatorDeclaredRecordBuilder {
	builder.value.observationWindow = window
	return builder
}

func (builder OperatorDeclaredRecordBuilder) AtCarrierRevision(
	revision CarrierRevision,
) OperatorDeclaredRecordBuilder {
	builder.value.carrierRevision = revision
	return builder
}

func (builder OperatorDeclaredRecordBuilder) Build() (OperatorDeclaredRecord, error) {
	if err := validateOperatorDeclaredRecord(builder.value); err != nil {
		return OperatorDeclaredRecord{}, err
	}
	return builder.value, nil
}

func (OperatorDeclaredRecord) profileDeclarationReceiptVariant() {}

func (record OperatorDeclaredRecord) DeclarationAuthorityBasisRef() string {
	return record.declarationAuthorityBasisRef
}

func (record OperatorDeclaredRecord) DeclarationWorkRef() string {
	return record.declarationWorkRef
}

func (record OperatorDeclaredRecord) ProjectRoot() string {
	return record.projectRoot
}

func (record OperatorDeclaredRecord) ScopePayloadDigest() ContentDigest {
	return record.scopePayloadDigest
}

func (record OperatorDeclaredRecord) ObservedBasisDigest() ContentDigest {
	return record.observedBasisDigest
}

func (record OperatorDeclaredRecord) ObservationWindow() ObservationWindow {
	return record.observationWindow
}

func (record OperatorDeclaredRecord) CarrierRevision() CarrierRevision {
	return record.carrierRevision
}

type OnboardingAgentDeclaredRecord struct {
	declarationAuthorityBasisRef string
	candidateProvenanceDigest    ContentDigest
	admissionEventRef            string
	projectRoot                  string
	scopePayloadDigest           ContentDigest
	observedBasisDigest          ContentDigest
	observationWindow            ObservationWindow
	carrierRevision              CarrierRevision
}

type OnboardingAgentDeclaredRecordBuilder struct {
	value OnboardingAgentDeclaredRecord
}

func NewOnboardingAgentDeclaredRecordBuilder(
	authorityBasisRef string,
	candidateProvenanceDigest ContentDigest,
	admissionEventRef string,
) OnboardingAgentDeclaredRecordBuilder {
	return OnboardingAgentDeclaredRecordBuilder{value: OnboardingAgentDeclaredRecord{
		declarationAuthorityBasisRef: authorityBasisRef,
		candidateProvenanceDigest:    candidateProvenanceDigest,
		admissionEventRef:            admissionEventRef,
	}}
}

func (builder OnboardingAgentDeclaredRecordBuilder) ForProject(
	root string,
) OnboardingAgentDeclaredRecordBuilder {
	builder.value.projectRoot = root
	return builder
}

func (builder OnboardingAgentDeclaredRecordBuilder) ForScopePayload(
	digest ContentDigest,
) OnboardingAgentDeclaredRecordBuilder {
	builder.value.scopePayloadDigest = digest
	return builder
}

func (builder OnboardingAgentDeclaredRecordBuilder) ForObservedBasis(
	digest ContentDigest,
) OnboardingAgentDeclaredRecordBuilder {
	builder.value.observedBasisDigest = digest
	return builder
}

func (builder OnboardingAgentDeclaredRecordBuilder) ObservedWithin(
	window ObservationWindow,
) OnboardingAgentDeclaredRecordBuilder {
	builder.value.observationWindow = window
	return builder
}

func (builder OnboardingAgentDeclaredRecordBuilder) AtCarrierRevision(
	revision CarrierRevision,
) OnboardingAgentDeclaredRecordBuilder {
	builder.value.carrierRevision = revision
	return builder
}

func (builder OnboardingAgentDeclaredRecordBuilder) Build() (OnboardingAgentDeclaredRecord, error) {
	if err := validateOnboardingAgentDeclaredRecord(builder.value); err != nil {
		return OnboardingAgentDeclaredRecord{}, err
	}
	return builder.value, nil
}

func (OnboardingAgentDeclaredRecord) profileDeclarationReceiptVariant() {}

func (record OnboardingAgentDeclaredRecord) DeclarationAuthorityBasisRef() string {
	return record.declarationAuthorityBasisRef
}

func (record OnboardingAgentDeclaredRecord) CandidateProvenanceDigest() ContentDigest {
	return record.candidateProvenanceDigest
}

func (record OnboardingAgentDeclaredRecord) AdmissionEventRef() string {
	return record.admissionEventRef
}

func (record OnboardingAgentDeclaredRecord) ProjectRoot() string {
	return record.projectRoot
}

func (record OnboardingAgentDeclaredRecord) ScopePayloadDigest() ContentDigest {
	return record.scopePayloadDigest
}

func (record OnboardingAgentDeclaredRecord) ObservedBasisDigest() ContentDigest {
	return record.observedBasisDigest
}

func (record OnboardingAgentDeclaredRecord) ObservationWindow() ObservationWindow {
	return record.observationWindow
}

func (record OnboardingAgentDeclaredRecord) CarrierRevision() CarrierRevision {
	return record.carrierRevision
}

type ImportedDeclarationRecord struct {
	sourceRef                    string
	sourceDigest                 ContentDigest
	declarationAuthorityBasisRef string
	scopePayloadDigest           ContentDigest
	observedBasisDigest          ContentDigest
	carrierRevision              CarrierRevision
}

type ImportedDeclarationRecordBuilder struct {
	value ImportedDeclarationRecord
}

func NewImportedDeclarationRecordBuilder(
	sourceRef string,
	sourceDigest ContentDigest,
	authorityBasisRef string,
) ImportedDeclarationRecordBuilder {
	return ImportedDeclarationRecordBuilder{value: ImportedDeclarationRecord{
		sourceRef:                    sourceRef,
		sourceDigest:                 sourceDigest,
		declarationAuthorityBasisRef: authorityBasisRef,
	}}
}

func (builder ImportedDeclarationRecordBuilder) ForScopePayload(
	digest ContentDigest,
) ImportedDeclarationRecordBuilder {
	builder.value.scopePayloadDigest = digest
	return builder
}

func (builder ImportedDeclarationRecordBuilder) ForObservedBasis(
	digest ContentDigest,
) ImportedDeclarationRecordBuilder {
	builder.value.observedBasisDigest = digest
	return builder
}

func (builder ImportedDeclarationRecordBuilder) AtCarrierRevision(
	revision CarrierRevision,
) ImportedDeclarationRecordBuilder {
	builder.value.carrierRevision = revision
	return builder
}

func (builder ImportedDeclarationRecordBuilder) Build() (ImportedDeclarationRecord, error) {
	if err := validateImportedDeclarationRecord(builder.value); err != nil {
		return ImportedDeclarationRecord{}, err
	}
	return builder.value, nil
}

func (ImportedDeclarationRecord) profileDeclarationReceiptVariant() {}

func (record ImportedDeclarationRecord) DeclarationAuthorityBasisRef() string {
	return record.declarationAuthorityBasisRef
}

func (record ImportedDeclarationRecord) SourceRef() string {
	return record.sourceRef
}

func (record ImportedDeclarationRecord) SourceDigest() ContentDigest {
	return record.sourceDigest
}

func (record ImportedDeclarationRecord) ScopePayloadDigest() ContentDigest {
	return record.scopePayloadDigest
}

func (record ImportedDeclarationRecord) ObservedBasisDigest() ContentDigest {
	return record.observedBasisDigest
}

func (record ImportedDeclarationRecord) CarrierRevision() CarrierRevision {
	return record.carrierRevision
}

func validateProfileDeclarationReceipt(receipt ProfileDeclarationReceipt) error {
	switch value := receipt.(type) {
	case OperatorDeclaredRecord:
		return validateOperatorDeclaredRecord(value)
	case OnboardingAgentDeclaredRecord:
		return validateOnboardingAgentDeclaredRecord(value)
	case ImportedDeclarationRecord:
		return validateImportedDeclarationRecord(value)
	default:
		return fmt.Errorf("profile declaration receipt must be a complete operator, onboarding-agent, or imported record")
	}
}

func validateOperatorDeclaredRecord(record OperatorDeclaredRecord) error {
	if _, err := requireText("declaration authority-basis ref", record.declarationAuthorityBasisRef); err != nil {
		return err
	}
	if _, err := requireText("declaration Work ref", record.declarationWorkRef); err != nil {
		return err
	}
	if _, err := requireText("operator declaration project root", record.projectRoot); err != nil {
		return err
	}
	return validateRecordDigestsWindowRevision(
		record.scopePayloadDigest,
		record.observedBasisDigest,
		record.observationWindow,
		record.carrierRevision,
	)
}

func validateOnboardingAgentDeclaredRecord(record OnboardingAgentDeclaredRecord) error {
	textFields := []struct {
		name  string
		value string
	}{
		{name: "declaration authority-basis ref", value: record.declarationAuthorityBasisRef},
		{name: "onboarding admission-event ref", value: record.admissionEventRef},
		{name: "onboarding project root", value: record.projectRoot},
	}
	for _, field := range textFields {
		if _, err := requireText(field.name, field.value); err != nil {
			return err
		}
	}
	if !record.candidateProvenanceDigest.valid() {
		return fmt.Errorf("onboarding candidate-provenance digest is required")
	}
	return validateRecordDigestsWindowRevision(
		record.scopePayloadDigest,
		record.observedBasisDigest,
		record.observationWindow,
		record.carrierRevision,
	)
}

func validateImportedDeclarationRecord(record ImportedDeclarationRecord) error {
	if _, err := requireText("import source ref", record.sourceRef); err != nil {
		return err
	}
	if !record.sourceDigest.valid() {
		return fmt.Errorf("import source digest is invalid")
	}
	if _, err := requireText("declaration authority-basis ref", record.declarationAuthorityBasisRef); err != nil {
		return err
	}
	if !record.scopePayloadDigest.valid() || !record.observedBasisDigest.valid() {
		return fmt.Errorf("imported record scope-payload and observed-basis digests are required")
	}
	if !record.carrierRevision.valid() {
		return fmt.Errorf("imported record carrier revision is invalid")
	}
	return nil
}

func validateRecordDigestsWindowRevision(
	scopePayloadDigest ContentDigest,
	observedBasisDigest ContentDigest,
	window ObservationWindow,
	carrierRevision CarrierRevision,
) error {
	if !scopePayloadDigest.valid() || !observedBasisDigest.valid() {
		return fmt.Errorf("record scope-payload and observed-basis digests are required")
	}
	if !window.valid() {
		return fmt.Errorf("record observation window is invalid")
	}
	if !carrierRevision.valid() {
		return fmt.Errorf("record carrier revision is invalid")
	}
	return nil
}

func validateObservedBasis(values []ObservedBasis) error {
	if len(values) == 0 {
		return fmt.Errorf("profile declaration requires at least one basis item")
	}
	for index, value := range values {
		if !value.valid() {
			return fmt.Errorf("profile declaration basis item %d is invalid", index)
		}
	}
	return nil
}
