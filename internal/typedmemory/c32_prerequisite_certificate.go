package typedmemory

import (
	"bytes"
	"fmt"
)

// C32EvaluationMechanismIdentity is the exact deployed-artifact coordinate
// selected for one C.3.2 prerequisite rule. It is configuration identity, not
// executable-byte attestation or evidence that evaluation occurred.
type C32EvaluationMechanismIdentity struct {
	artifact       CarrierRef
	edition        CarrierEdition
	digest         SHA256Digest
	canonicalBytes []byte
}

func NewC32EvaluationMechanismIdentity(
	artifact CarrierRef,
	edition CarrierEdition,
	digest SHA256Digest,
) (C32EvaluationMechanismIdentity, error) {
	if !artifact.valid() ||
		!edition.valid() ||
		implicitContextSelector(edition.String()) ||
		!digest.valid() {
		return C32EvaluationMechanismIdentity{}, fmt.Errorf(
			"C.3.2 mechanism identity requires exact artifact, edition, and digest",
		)
	}
	writer := canonicalC32EvaluationMechanismIdentity(artifact, edition, digest)
	return C32EvaluationMechanismIdentity{
		artifact:       artifact,
		edition:        edition,
		digest:         digest,
		canonicalBytes: writer.bytes(),
	}, nil
}

func (identity C32EvaluationMechanismIdentity) Artifact() CarrierRef {
	return identity.artifact
}

func (identity C32EvaluationMechanismIdentity) Edition() CarrierEdition {
	return identity.edition
}

func (identity C32EvaluationMechanismIdentity) Digest() SHA256Digest {
	return identity.digest
}

func (identity C32EvaluationMechanismIdentity) CanonicalBytes() []byte {
	return append([]byte(nil), identity.canonicalBytes...)
}

func (identity C32EvaluationMechanismIdentity) valid() bool {
	rebuilt, err := NewC32EvaluationMechanismIdentity(
		identity.artifact,
		identity.edition,
		identity.digest,
	)
	return err == nil && bytes.Equal(
		rebuilt.CanonicalBytes(),
		identity.CanonicalBytes(),
	)
}

func canonicalC32EvaluationMechanismIdentity(
	artifact CarrierRef,
	edition CarrierEdition,
	digest SHA256Digest,
) canonicalWriter {
	writer := newCanonicalWriter("c32-evaluation-mechanism-identity.v1")
	writer.addString(artifact.String())
	writer.addString(edition.String())
	writer.addString(digest.String())
	return writer
}

type C32CandidateVisibilityKind uint8

const (
	C32PersistedCandidateVisibility C32CandidateVisibilityKind = iota + 1
	C32ProspectiveCandidateVisibility
)

func (kind C32CandidateVisibilityKind) String() string {
	switch kind {
	case C32PersistedCandidateVisibility:
		return "persisted_snapshot"
	case C32ProspectiveCandidateVisibility:
		return "prospective_prior_batch"
	default:
		return ""
	}
}

// C32CandidateVisibilityCoordinate is a closed shape. Persisted evaluation has
// no candidate evaluator invocation; prospective evaluation must bind all of
// its request, result, basis, rule, and mechanism coordinates.
type C32CandidateVisibilityCoordinate interface {
	Kind() C32CandidateVisibilityKind
	CanonicalBytes() []byte
	Digest() SHA256Digest
	c32CandidateVisibilityCoordinateVariant()
}

type C32PersistedVisibilityCoordinate struct {
	canonicalBytes []byte
	digest         SHA256Digest
}

func NewC32PersistedVisibilityCoordinate() C32PersistedVisibilityCoordinate {
	writer := newCanonicalWriter("c32-candidate-visibility.persisted.v1")
	return C32PersistedVisibilityCoordinate{
		canonicalBytes: writer.bytes(),
		digest:         writer.digest(),
	}
}

func (C32PersistedVisibilityCoordinate) Kind() C32CandidateVisibilityKind {
	return C32PersistedCandidateVisibility
}

func (coordinate C32PersistedVisibilityCoordinate) CanonicalBytes() []byte {
	return append([]byte(nil), coordinate.canonicalBytes...)
}

func (coordinate C32PersistedVisibilityCoordinate) Digest() SHA256Digest {
	return coordinate.digest
}

func (C32PersistedVisibilityCoordinate) c32CandidateVisibilityCoordinateVariant() {}

func (coordinate C32PersistedVisibilityCoordinate) valid() bool {
	rebuilt := NewC32PersistedVisibilityCoordinate()
	return rebuilt.digest == coordinate.digest &&
		bytes.Equal(rebuilt.canonicalBytes, coordinate.canonicalBytes)
}

type C32ProspectiveVisibilityCoordinate struct {
	requestDigest  SHA256Digest
	resultDigest   SHA256Digest
	basisDigest    SHA256Digest
	rule           RuleRef
	mechanism      C32EvaluationMechanismIdentity
	canonicalBytes []byte
	digest         SHA256Digest
}

type C32ProspectiveVisibilityCoordinateInput struct {
	RequestDigest SHA256Digest
	ResultDigest  SHA256Digest
	BasisDigest   SHA256Digest
	Rule          RuleRef
	Mechanism     C32EvaluationMechanismIdentity
}

func NewC32ProspectiveVisibilityCoordinate(
	input C32ProspectiveVisibilityCoordinateInput,
) (C32ProspectiveVisibilityCoordinate, error) {
	if !input.RequestDigest.valid() ||
		!input.ResultDigest.valid() ||
		!input.BasisDigest.valid() ||
		!input.Rule.valid() ||
		!input.Mechanism.valid() {
		return C32ProspectiveVisibilityCoordinate{}, fmt.Errorf(
			"prospective C.3.2 visibility requires exact request, result, basis, rule, and mechanism",
		)
	}
	writer := canonicalC32ProspectiveVisibilityCoordinate(input)
	return C32ProspectiveVisibilityCoordinate{
		requestDigest:  input.RequestDigest,
		resultDigest:   input.ResultDigest,
		basisDigest:    input.BasisDigest,
		rule:           input.Rule,
		mechanism:      input.Mechanism,
		canonicalBytes: writer.bytes(),
		digest:         writer.digest(),
	}, nil
}

func (C32ProspectiveVisibilityCoordinate) Kind() C32CandidateVisibilityKind {
	return C32ProspectiveCandidateVisibility
}

func (coordinate C32ProspectiveVisibilityCoordinate) RequestDigest() SHA256Digest {
	return coordinate.requestDigest
}

func (coordinate C32ProspectiveVisibilityCoordinate) ResultDigest() SHA256Digest {
	return coordinate.resultDigest
}

func (coordinate C32ProspectiveVisibilityCoordinate) BasisDigest() SHA256Digest {
	return coordinate.basisDigest
}

func (coordinate C32ProspectiveVisibilityCoordinate) Rule() RuleRef {
	return coordinate.rule
}

func (coordinate C32ProspectiveVisibilityCoordinate) Mechanism() C32EvaluationMechanismIdentity {
	return coordinate.mechanism
}

func (coordinate C32ProspectiveVisibilityCoordinate) CanonicalBytes() []byte {
	return append([]byte(nil), coordinate.canonicalBytes...)
}

func (coordinate C32ProspectiveVisibilityCoordinate) Digest() SHA256Digest {
	return coordinate.digest
}

func (C32ProspectiveVisibilityCoordinate) c32CandidateVisibilityCoordinateVariant() {}

func (coordinate C32ProspectiveVisibilityCoordinate) valid() bool {
	rebuilt, err := NewC32ProspectiveVisibilityCoordinate(
		C32ProspectiveVisibilityCoordinateInput{
			RequestDigest: coordinate.requestDigest,
			ResultDigest:  coordinate.resultDigest,
			BasisDigest:   coordinate.basisDigest,
			Rule:          coordinate.rule,
			Mechanism:     coordinate.mechanism,
		},
	)
	return err == nil &&
		rebuilt.digest == coordinate.digest &&
		bytes.Equal(rebuilt.canonicalBytes, coordinate.canonicalBytes)
}

func canonicalC32ProspectiveVisibilityCoordinate(
	input C32ProspectiveVisibilityCoordinateInput,
) canonicalWriter {
	writer := newCanonicalWriter("c32-candidate-visibility.prospective.v1")
	writer.addString(input.RequestDigest.String())
	writer.addString(input.ResultDigest.String())
	writer.addString(input.BasisDigest.String())
	writer.addString(input.Rule.String())
	writer.addBytes(input.Mechanism.CanonicalBytes())
	return writer
}

// C32PrerequisiteCertificate is the content-addressed coordinate of one
// actually correlated prerequisite chain. It does not contain a membership
// verdict and cannot authorize persistence by itself.
type C32PrerequisiteCertificate struct {
	typeEnv                  TypeEnvRef
	kindSignature            KindSignatureRef
	entitySet                EntitySetDefinitionRef
	contextSlice             ContextSliceRef
	evaluationView           MemberOfEvaluationView
	memberOfRequestDigest    SHA256Digest
	enumerationRequestDigest SHA256Digest
	enumerationResultDigest  SHA256Digest
	enumerationBasisDigest   SHA256Digest
	enumerationRule          RuleRef
	enumerationMechanism     C32EvaluationMechanismIdentity
	definednessRequestDigest SHA256Digest
	definednessResultDigest  SHA256Digest
	definednessBasisDigest   SHA256Digest
	definednessRule          RuleRef
	definednessMechanism     C32EvaluationMechanismIdentity
	candidateVisibility      C32CandidateVisibilityCoordinate
	canonicalBytes           []byte
	digest                   SHA256Digest
}

type C32PrerequisiteCertificateInput struct {
	TypeEnv                  TypeEnvRef
	KindSignature            KindSignatureRef
	EntitySet                EntitySetDefinitionRef
	ContextSlice             ContextSliceRef
	EvaluationView           MemberOfEvaluationView
	MemberOfRequestDigest    SHA256Digest
	EnumerationRequestDigest SHA256Digest
	EnumerationResultDigest  SHA256Digest
	EnumerationBasisDigest   SHA256Digest
	EnumerationRule          RuleRef
	EnumerationMechanism     C32EvaluationMechanismIdentity
	DefinednessRequestDigest SHA256Digest
	DefinednessResultDigest  SHA256Digest
	DefinednessBasisDigest   SHA256Digest
	DefinednessRule          RuleRef
	DefinednessMechanism     C32EvaluationMechanismIdentity
	CandidateVisibility      C32CandidateVisibilityCoordinate
}

func NewC32PrerequisiteCertificate(
	input C32PrerequisiteCertificateInput,
) (C32PrerequisiteCertificate, error) {
	if !input.TypeEnv.valid() ||
		!input.KindSignature.valid() ||
		!input.EntitySet.valid() ||
		!input.ContextSlice.valid() ||
		!validMemberOfEvaluationView(input.EvaluationView) {
		return C32PrerequisiteCertificate{}, fmt.Errorf(
			"C.3.2 certificate requires exact TypeEnv, signature, EntitySet, slice, and view",
		)
	}
	if input.KindSignature.TypeEnv() != input.TypeEnv ||
		input.EntitySet.TypeEnv() != input.TypeEnv ||
		input.EvaluationView.TypeEnv() != input.TypeEnv ||
		input.KindSignature.Context() != input.EntitySet.Context() {
		return C32PrerequisiteCertificate{}, fmt.Errorf(
			"C.3.2 certificate semantic coordinates do not share one TypeEnv and context",
		)
	}
	if !input.MemberOfRequestDigest.valid() ||
		!input.EnumerationRequestDigest.valid() ||
		!input.EnumerationResultDigest.valid() ||
		!input.EnumerationBasisDigest.valid() ||
		!input.EnumerationRule.valid() ||
		!input.EnumerationMechanism.valid() ||
		!input.DefinednessRequestDigest.valid() ||
		!input.DefinednessResultDigest.valid() ||
		!input.DefinednessBasisDigest.valid() ||
		!input.DefinednessRule.valid() ||
		!input.DefinednessMechanism.valid() ||
		!validC32CandidateVisibilityCoordinate(input.CandidateVisibility) {
		return C32PrerequisiteCertificate{}, fmt.Errorf(
			"C.3.2 certificate requires exact request, result, basis, rule, and mechanism coordinates",
		)
	}
	if !c32VisibilityShapeMatchesView(
		input.CandidateVisibility,
		input.EvaluationView,
	) {
		return C32PrerequisiteCertificate{}, fmt.Errorf(
			"C.3.2 candidate visibility shape does not match the evaluation view",
		)
	}
	writer := canonicalC32PrerequisiteCertificate(input)
	return C32PrerequisiteCertificate{
		typeEnv:                  input.TypeEnv,
		kindSignature:            input.KindSignature,
		entitySet:                input.EntitySet,
		contextSlice:             input.ContextSlice,
		evaluationView:           input.EvaluationView,
		memberOfRequestDigest:    input.MemberOfRequestDigest,
		enumerationRequestDigest: input.EnumerationRequestDigest,
		enumerationResultDigest:  input.EnumerationResultDigest,
		enumerationBasisDigest:   input.EnumerationBasisDigest,
		enumerationRule:          input.EnumerationRule,
		enumerationMechanism:     input.EnumerationMechanism,
		definednessRequestDigest: input.DefinednessRequestDigest,
		definednessResultDigest:  input.DefinednessResultDigest,
		definednessBasisDigest:   input.DefinednessBasisDigest,
		definednessRule:          input.DefinednessRule,
		definednessMechanism:     input.DefinednessMechanism,
		candidateVisibility:      input.CandidateVisibility,
		canonicalBytes:           writer.bytes(),
		digest:                   writer.digest(),
	}, nil
}

func (certificate C32PrerequisiteCertificate) TypeEnv() TypeEnvRef {
	return certificate.typeEnv
}

func (certificate C32PrerequisiteCertificate) KindSignature() KindSignatureRef {
	return certificate.kindSignature
}

func (certificate C32PrerequisiteCertificate) EntitySet() EntitySetDefinitionRef {
	return certificate.entitySet
}

func (certificate C32PrerequisiteCertificate) ContextSlice() ContextSliceRef {
	return certificate.contextSlice
}

func (certificate C32PrerequisiteCertificate) EvaluationView() MemberOfEvaluationView {
	return certificate.evaluationView
}

func (certificate C32PrerequisiteCertificate) MemberOfRequestDigest() SHA256Digest {
	return certificate.memberOfRequestDigest
}

func (certificate C32PrerequisiteCertificate) EnumerationRequestDigest() SHA256Digest {
	return certificate.enumerationRequestDigest
}

func (certificate C32PrerequisiteCertificate) EnumerationResultDigest() SHA256Digest {
	return certificate.enumerationResultDigest
}

func (certificate C32PrerequisiteCertificate) EnumerationBasisDigest() SHA256Digest {
	return certificate.enumerationBasisDigest
}

func (certificate C32PrerequisiteCertificate) EnumerationRule() RuleRef {
	return certificate.enumerationRule
}

func (certificate C32PrerequisiteCertificate) EnumerationMechanism() C32EvaluationMechanismIdentity {
	return certificate.enumerationMechanism
}

func (certificate C32PrerequisiteCertificate) DefinednessRequestDigest() SHA256Digest {
	return certificate.definednessRequestDigest
}

func (certificate C32PrerequisiteCertificate) DefinednessResultDigest() SHA256Digest {
	return certificate.definednessResultDigest
}

func (certificate C32PrerequisiteCertificate) DefinednessBasisDigest() SHA256Digest {
	return certificate.definednessBasisDigest
}

func (certificate C32PrerequisiteCertificate) DefinednessRule() RuleRef {
	return certificate.definednessRule
}

func (certificate C32PrerequisiteCertificate) DefinednessMechanism() C32EvaluationMechanismIdentity {
	return certificate.definednessMechanism
}

func (certificate C32PrerequisiteCertificate) CandidateVisibility() C32CandidateVisibilityCoordinate {
	return cloneC32CandidateVisibilityCoordinate(certificate.candidateVisibility)
}

func (certificate C32PrerequisiteCertificate) CanonicalBytes() []byte {
	return append([]byte(nil), certificate.canonicalBytes...)
}

func (certificate C32PrerequisiteCertificate) Digest() SHA256Digest {
	return certificate.digest
}

func (certificate C32PrerequisiteCertificate) valid() bool {
	rebuilt, err := NewC32PrerequisiteCertificate(
		C32PrerequisiteCertificateInput{
			TypeEnv:                  certificate.typeEnv,
			KindSignature:            certificate.kindSignature,
			EntitySet:                certificate.entitySet,
			ContextSlice:             certificate.contextSlice,
			EvaluationView:           certificate.evaluationView,
			MemberOfRequestDigest:    certificate.memberOfRequestDigest,
			EnumerationRequestDigest: certificate.enumerationRequestDigest,
			EnumerationResultDigest:  certificate.enumerationResultDigest,
			EnumerationBasisDigest:   certificate.enumerationBasisDigest,
			EnumerationRule:          certificate.enumerationRule,
			EnumerationMechanism:     certificate.enumerationMechanism,
			DefinednessRequestDigest: certificate.definednessRequestDigest,
			DefinednessResultDigest:  certificate.definednessResultDigest,
			DefinednessBasisDigest:   certificate.definednessBasisDigest,
			DefinednessRule:          certificate.definednessRule,
			DefinednessMechanism:     certificate.definednessMechanism,
			CandidateVisibility:      certificate.candidateVisibility,
		},
	)
	return err == nil &&
		rebuilt.digest == certificate.digest &&
		bytes.Equal(rebuilt.canonicalBytes, certificate.canonicalBytes)
}

func canonicalC32PrerequisiteCertificate(
	input C32PrerequisiteCertificateInput,
) canonicalWriter {
	writer := newCanonicalWriter("c32-prerequisite-certificate.v1")
	writer.addString(input.TypeEnv.String())
	writer.addString(input.KindSignature.String())
	writer.addString(input.EntitySet.String())
	writer.addString(input.ContextSlice.String())
	writer.addBytes(input.EvaluationView.CanonicalBytes())
	writer.addString(input.MemberOfRequestDigest.String())
	writer.addString(input.EnumerationRequestDigest.String())
	writer.addString(input.EnumerationResultDigest.String())
	writer.addString(input.EnumerationBasisDigest.String())
	writer.addString(input.EnumerationRule.String())
	writer.addBytes(input.EnumerationMechanism.CanonicalBytes())
	writer.addString(input.DefinednessRequestDigest.String())
	writer.addString(input.DefinednessResultDigest.String())
	writer.addString(input.DefinednessBasisDigest.String())
	writer.addString(input.DefinednessRule.String())
	writer.addBytes(input.DefinednessMechanism.CanonicalBytes())
	writer.addBytes(input.CandidateVisibility.CanonicalBytes())
	return writer
}

func validC32CandidateVisibilityCoordinate(
	coordinate C32CandidateVisibilityCoordinate,
) bool {
	switch value := coordinate.(type) {
	case C32PersistedVisibilityCoordinate:
		return value.valid()
	case C32ProspectiveVisibilityCoordinate:
		return value.valid()
	default:
		return false
	}
}

func cloneC32CandidateVisibilityCoordinate(
	coordinate C32CandidateVisibilityCoordinate,
) C32CandidateVisibilityCoordinate {
	switch value := coordinate.(type) {
	case C32PersistedVisibilityCoordinate:
		value.canonicalBytes = value.CanonicalBytes()
		return value
	case C32ProspectiveVisibilityCoordinate:
		value.canonicalBytes = value.CanonicalBytes()
		return value
	default:
		return nil
	}
}

func c32VisibilityShapeMatchesView(
	coordinate C32CandidateVisibilityCoordinate,
	view MemberOfEvaluationView,
) bool {
	switch view.(type) {
	case PersistedSnapshotView:
		_, ok := coordinate.(C32PersistedVisibilityCoordinate)
		return ok
	case ProspectiveBatchView:
		_, ok := coordinate.(C32ProspectiveVisibilityCoordinate)
		return ok
	default:
		return false
	}
}
