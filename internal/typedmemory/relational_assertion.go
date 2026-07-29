package typedmemory

import "fmt"

// AssertionModalityKind is the closed semantic posture stated by a relational
// assertion. It is claim content only: no kind in this union proves that the
// direct relation obtains or that a world-side relation occurrence exists.
type AssertionModalityKind string

const (
	AssertionModalityAffirmsObtaining AssertionModalityKind = "affirms_obtaining"
	AssertionModalityDeniesObtaining  AssertionModalityKind = "denies_obtaining"
	AssertionModalityObtainingUnknown AssertionModalityKind = "obtaining_unknown"
)

func (kind AssertionModalityKind) String() string { return string(kind) }

// AssertionModality is a sealed sum. In this first bounded v3 slice the
// positive branch explicitly carries no occurrence designation. A later
// occurrence-bearing branch must be a distinct strong variant with its own
// obtaining and occurrence-identity bases; it must not broaden this interface
// with nullable fields.
type AssertionModality interface {
	Kind() AssertionModalityKind
	assertionModalityVariant()
	validAssertionModality() bool
}

// AffirmsObtaining states a positive direct-relation claim. It does not prove
// predicate satisfaction and cannot designate an occurrence in this slice.
type AffirmsObtaining struct {
	sealed bool
}

func NewAffirmsObtaining() AffirmsObtaining { return AffirmsObtaining{sealed: true} }

func (AffirmsObtaining) Kind() AssertionModalityKind {
	return AssertionModalityAffirmsObtaining
}

func (AffirmsObtaining) HasOccurrenceDesignation() bool { return false }

func (AffirmsObtaining) assertionModalityVariant() {}

func (modality AffirmsObtaining) validAssertionModality() bool { return modality.sealed }

// DeniesObtaining states a negative direct-relation claim. This branch cannot
// carry or imply a world-side occurrence.
type DeniesObtaining struct {
	sealed bool
}

func NewDeniesObtaining() DeniesObtaining { return DeniesObtaining{sealed: true} }

func (DeniesObtaining) Kind() AssertionModalityKind {
	return AssertionModalityDeniesObtaining
}

func (DeniesObtaining) assertionModalityVariant() {}

func (modality DeniesObtaining) validAssertionModality() bool { return modality.sealed }

// ObtainingUnknown states that the direct obtaining predicate remains
// unresolved. It is not the interpretation of a legacy assertion whose bytes
// did not carry any modality.
type ObtainingUnknown struct {
	sealed bool
}

func NewObtainingUnknown() ObtainingUnknown { return ObtainingUnknown{sealed: true} }

func (ObtainingUnknown) Kind() AssertionModalityKind {
	return AssertionModalityObtainingUnknown
}

func (ObtainingUnknown) assertionModalityVariant() {}

func (modality ObtainingUnknown) validAssertionModality() bool { return modality.sealed }

func validAssertionModalityVariant(modality AssertionModality) bool {
	switch value := modality.(type) {
	case AffirmsObtaining:
		return value.validAssertionModality()
	case DeniesObtaining:
		return value.validAssertionModality()
	case ObtainingUnknown:
		return value.validAssertionModality()
	default:
		return false
	}
}

func sameAssertionModality(left, right AssertionModality) bool {
	if !validAssertionModalityVariant(left) || !validAssertionModalityVariant(right) {
		return false
	}
	return left.Kind() == right.Kind()
}

// RelationalAssertionCandidateInput is the exact v3 request-local assertion
// carrier. ContextSlice remains complete and content-addressed; reducing it to
// a BoundedContextRef would discard part of the validation basis.
type RelationalAssertionCandidateInput struct {
	Assertion  AssertionID
	Signature  RelationSignatureRef
	Slice      ContextSlice
	Modality   AssertionModality
	Bindings   []CandidateSlotBinding
	Provenance ProvenanceRef
}

// RelationalAssertionCandidate is disjoint from legacy
// RelationInstantiation. It states modality explicitly and is never decoded
// from or silently substituted for canonical v2 bytes.
type RelationalAssertionCandidate struct {
	assertion  AssertionID
	signature  RelationSignatureRef
	slice      ContextSlice
	modality   AssertionModality
	bindings   []CandidateSlotBinding
	provenance ProvenanceRef
}

func NewRelationalAssertionCandidate(
	input RelationalAssertionCandidateInput,
) (RelationalAssertionCandidate, error) {
	if !input.Assertion.valid() {
		return RelationalAssertionCandidate{}, fmt.Errorf("relational assertion requires an assertion ID")
	}
	if !input.Signature.valid() {
		return RelationalAssertionCandidate{}, fmt.Errorf("relational assertion requires a signature")
	}
	if !input.Slice.valid() {
		return RelationalAssertionCandidate{}, fmt.Errorf("relational assertion requires a complete ContextSlice")
	}
	if !validAssertionModalityVariant(input.Modality) {
		return RelationalAssertionCandidate{}, fmt.Errorf("relational assertion requires a closed explicit modality")
	}
	if !input.Provenance.valid() {
		return RelationalAssertionCandidate{}, fmt.Errorf("relational assertion requires provenance")
	}

	bindings, err := normalizeCandidateBindings(input.Bindings)
	if err != nil {
		return RelationalAssertionCandidate{}, err
	}
	return RelationalAssertionCandidate{
		assertion:  input.Assertion,
		signature:  input.Signature,
		slice:      input.Slice,
		modality:   input.Modality,
		bindings:   bindings,
		provenance: input.Provenance,
	}, nil
}

func (assertion RelationalAssertionCandidate) Assertion() AssertionID {
	return assertion.assertion
}

func (assertion RelationalAssertionCandidate) Signature() RelationSignatureRef {
	return assertion.signature
}

func (assertion RelationalAssertionCandidate) RelationDeclarationFragmentRef() TypedRelationDeclarationFragmentRef {
	return assertion.signature
}

func (RelationalAssertionCandidate) RelationDeclarationPosture() RelationDeclarationPosture {
	return RelationDeclarationTypedFragment
}

func (assertion RelationalAssertionCandidate) Slice() ContextSlice { return assertion.slice }

func (assertion RelationalAssertionCandidate) Context() BoundedContextRef {
	return assertion.slice.Context()
}

func (assertion RelationalAssertionCandidate) Modality() AssertionModality {
	return assertion.modality
}

func (assertion RelationalAssertionCandidate) Bindings() []CandidateSlotBinding {
	return append([]CandidateSlotBinding(nil), assertion.bindings...)
}

func (assertion RelationalAssertionCandidate) Provenance() ProvenanceRef {
	return assertion.provenance
}

func (assertion RelationalAssertionCandidate) valid() bool {
	if !assertion.assertion.valid() ||
		!assertion.signature.valid() ||
		!assertion.slice.valid() ||
		!validAssertionModalityVariant(assertion.modality) ||
		len(assertion.bindings) == 0 ||
		!assertion.provenance.valid() {
		return false
	}
	for index, binding := range assertion.bindings {
		if !binding.valid() {
			return false
		}
		if index > 0 && assertion.bindings[index-1].Name().String() >= binding.Name().String() {
			return false
		}
	}
	return true
}

// RelationalAssertion is the strong v3 result of structural typed-memory
// validation. Its modality is preserved as claim content. Validation does not
// convert AffirmsObtaining into predicate truth or occurrence identity.
type RelationalAssertion struct {
	assertion  AssertionID
	signature  RelationSignatureRef
	slice      ContextSlice
	modality   AssertionModality
	bindings   []SlotBinding
	provenance ProvenanceRef
}

func newRelationalAssertion(
	candidate RelationalAssertionCandidate,
	bindings []SlotBinding,
) RelationalAssertion {
	return RelationalAssertion{
		assertion:  candidate.assertion,
		signature:  candidate.signature,
		slice:      candidate.slice,
		modality:   candidate.modality,
		bindings:   append([]SlotBinding(nil), bindings...),
		provenance: candidate.provenance,
	}
}

func (assertion RelationalAssertion) Assertion() AssertionID { return assertion.assertion }

func (assertion RelationalAssertion) Signature() RelationSignatureRef {
	return assertion.signature
}

func (assertion RelationalAssertion) RelationDeclarationFragmentRef() TypedRelationDeclarationFragmentRef {
	return assertion.signature
}

func (RelationalAssertion) RelationDeclarationPosture() RelationDeclarationPosture {
	return RelationDeclarationTypedFragment
}

func (assertion RelationalAssertion) Slice() ContextSlice { return assertion.slice }

func (assertion RelationalAssertion) Context() BoundedContextRef {
	return assertion.slice.Context()
}

func (assertion RelationalAssertion) Modality() AssertionModality {
	return assertion.modality
}

func (assertion RelationalAssertion) Bindings() []SlotBinding {
	return append([]SlotBinding(nil), assertion.bindings...)
}

func (assertion RelationalAssertion) Provenance() ProvenanceRef {
	return assertion.provenance
}

func (assertion RelationalAssertion) valid() bool {
	if !assertion.assertion.valid() ||
		!assertion.signature.valid() ||
		!assertion.slice.valid() ||
		!validAssertionModalityVariant(assertion.modality) ||
		len(assertion.bindings) == 0 ||
		!assertion.provenance.valid() {
		return false
	}
	for index, binding := range assertion.bindings {
		if !binding.valid() {
			return false
		}
		if index > 0 && assertion.bindings[index-1].Name().String() >= binding.Name().String() {
			return false
		}
	}
	return true
}

type candidateRelationalCarrier interface {
	Assertion() AssertionID
	RelationDeclarationFragmentRef() TypedRelationDeclarationFragmentRef
	Signature() RelationSignatureRef
	Slice() ContextSlice
	Bindings() []CandidateSlotBinding
	Provenance() ProvenanceRef
}

type validatedRelationalCarrier interface {
	Assertion() AssertionID
	RelationDeclarationFragmentRef() TypedRelationDeclarationFragmentRef
	Signature() RelationSignatureRef
	Slice() ContextSlice
	Bindings() []SlotBinding
	Provenance() ProvenanceRef
}
