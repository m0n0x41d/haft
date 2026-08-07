package legacyimport

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
)

type ClassificationKind string

const (
	ClassificationCarrierOnly   ClassificationKind = "carrier_only"
	ClassificationLegacyUnbound ClassificationKind = "legacy_unbound"
	ClassificationUnresolved    ClassificationKind = "unresolved"
)

type SubjectClassification interface {
	Subject() SemanticSubjectRef
	Kind() ClassificationKind
	Observations() []SubjectObservation
	canonicalBytes() []byte
	subjectClassificationVariant()
}

type CarrierOnly struct {
	subject      SemanticSubjectRef
	observations []CarrierObservation
}

func NewCarrierOnly(
	subject SemanticSubjectRef,
	observations []CarrierObservation,
) (CarrierOnly, error) {
	if !subject.valid() {
		return CarrierOnly{}, fmt.Errorf("carrier-only subject is required")
	}
	owned, err := normalizeCarrierObservations(subject, observations)
	if err != nil {
		return CarrierOnly{}, err
	}
	return CarrierOnly{subject: subject, observations: owned}, nil
}

func (classification CarrierOnly) Subject() SemanticSubjectRef { return classification.subject }

func (CarrierOnly) Kind() ClassificationKind { return ClassificationCarrierOnly }

func (classification CarrierOnly) Observations() []SubjectObservation {
	result := make([]SubjectObservation, 0, len(classification.observations))
	for _, observation := range classification.observations {
		result = append(result, observation)
	}
	return result
}

func (CarrierOnly) subjectClassificationVariant() {}

func (classification CarrierOnly) canonicalBytes() []byte {
	encoded, _ := json.Marshal(classificationDTOOf(classification))
	return encoded
}

type LegacyUnbound struct {
	subject      SemanticSubjectRef
	observations []AssociationObservation
}

func NewLegacyUnbound(
	subject SemanticSubjectRef,
	observations []AssociationObservation,
) (LegacyUnbound, error) {
	if !subject.valid() {
		return LegacyUnbound{}, fmt.Errorf("legacy-unbound subject is required")
	}
	owned, err := normalizeAssociationObservations(subject, observations)
	if err != nil {
		return LegacyUnbound{}, err
	}
	return LegacyUnbound{subject: subject, observations: owned}, nil
}

func (classification LegacyUnbound) Subject() SemanticSubjectRef { return classification.subject }

func (LegacyUnbound) Kind() ClassificationKind { return ClassificationLegacyUnbound }

func (classification LegacyUnbound) Observations() []SubjectObservation {
	result := make([]SubjectObservation, 0, len(classification.observations))
	for _, observation := range classification.observations {
		result = append(result, observation)
	}
	return result
}

func (LegacyUnbound) subjectClassificationVariant() {}

func (classification LegacyUnbound) canonicalBytes() []byte {
	encoded, _ := json.Marshal(classificationDTOOf(classification))
	return encoded
}

type Unresolved struct {
	subject      SemanticSubjectRef
	reason       UnresolvedReason
	observations []SubjectObservation
}

func NewUnresolved(
	subject SemanticSubjectRef,
	reason UnresolvedReason,
	observations []SubjectObservation,
) (Unresolved, error) {
	if !subject.valid() {
		return Unresolved{}, fmt.Errorf("unresolved subject is required")
	}
	if !reason.valid() {
		return Unresolved{}, fmt.Errorf("unresolved classification requires a basis reason")
	}
	owned, err := normalizeSubjectObservations(subject, observations)
	if err != nil {
		return Unresolved{}, err
	}
	return Unresolved{
		subject:      subject,
		reason:       reason,
		observations: owned,
	}, nil
}

func (classification Unresolved) Subject() SemanticSubjectRef { return classification.subject }

func (Unresolved) Kind() ClassificationKind { return ClassificationUnresolved }

func (classification Unresolved) Reason() UnresolvedReason { return classification.reason }

func (classification Unresolved) Observations() []SubjectObservation {
	return append([]SubjectObservation(nil), classification.observations...)
}

func (Unresolved) subjectClassificationVariant() {}

func (classification Unresolved) canonicalBytes() []byte {
	encoded, _ := json.Marshal(classificationDTOOf(classification))
	return encoded
}

func validateClassification(classification SubjectClassification) error {
	if classification == nil {
		return fmt.Errorf("subject classification is required")
	}
	switch observed := classification.(type) {
	case CarrierOnly:
		_, err := NewCarrierOnly(observed.subject, observed.observations)
		return err
	case LegacyUnbound:
		_, err := NewLegacyUnbound(observed.subject, observed.observations)
		return err
	case Unresolved:
		_, err := NewUnresolved(observed.subject, observed.reason, observed.observations)
		return err
	default:
		return fmt.Errorf("subject classification variant is unsupported")
	}
}

func normalizeCarrierObservations(
	subject SemanticSubjectRef,
	values []CarrierObservation,
) ([]CarrierObservation, error) {
	if len(values) == 0 {
		return nil, fmt.Errorf("carrier-only classification requires at least one carrier observation")
	}
	owned := append([]CarrierObservation(nil), values...)
	sort.Slice(owned, func(left, right int) bool {
		return bytes.Compare(owned[left].canonicalBytes(), owned[right].canonicalBytes()) < 0
	})
	for index, observation := range owned {
		if !observation.valid() || observation.Subject() != subject {
			return nil, fmt.Errorf("carrier observation %d does not belong to subject %q", index, subject.String())
		}
		if index > 0 && bytes.Equal(owned[index-1].canonicalBytes(), observation.canonicalBytes()) {
			return nil, fmt.Errorf("carrier-only classification repeats an observation")
		}
	}
	return owned, nil
}

func normalizeAssociationObservations(
	subject SemanticSubjectRef,
	values []AssociationObservation,
) ([]AssociationObservation, error) {
	if len(values) == 0 {
		return nil, fmt.Errorf("legacy-unbound classification requires at least one association observation")
	}
	owned := append([]AssociationObservation(nil), values...)
	sort.Slice(owned, func(left, right int) bool {
		return bytes.Compare(owned[left].canonicalBytes(), owned[right].canonicalBytes()) < 0
	})
	for index, observation := range owned {
		if !observation.valid() || observation.Subject() != subject {
			return nil, fmt.Errorf("association observation %d does not belong to subject %q", index, subject.String())
		}
		if index > 0 && bytes.Equal(owned[index-1].canonicalBytes(), observation.canonicalBytes()) {
			return nil, fmt.Errorf("legacy-unbound classification repeats an observation")
		}
	}
	return owned, nil
}

func normalizeSubjectObservations(
	subject SemanticSubjectRef,
	values []SubjectObservation,
) ([]SubjectObservation, error) {
	if len(values) == 0 {
		return nil, fmt.Errorf("unresolved classification requires at least one source observation")
	}
	owned := append([]SubjectObservation(nil), values...)
	sort.Slice(owned, func(left, right int) bool {
		return bytes.Compare(owned[left].canonicalBytes(), owned[right].canonicalBytes()) < 0
	})
	for index, observation := range owned {
		if err := validateObservation(observation); err != nil {
			return nil, fmt.Errorf("unresolved observation %d: %w", index, err)
		}
		if observation.Subject() != subject {
			return nil, fmt.Errorf("unresolved observation %d does not belong to subject %q", index, subject.String())
		}
		if index > 0 && bytes.Equal(owned[index-1].canonicalBytes(), observation.canonicalBytes()) {
			return nil, fmt.Errorf("unresolved classification repeats an observation")
		}
	}
	return owned, nil
}

type classificationDTO struct {
	Subject      string           `json:"subject"`
	Kind         string           `json:"kind"`
	Reason       string           `json:"reason,omitempty"`
	Observations []observationDTO `json:"observations"`
}

func classificationDTOOf(classification SubjectClassification) classificationDTO {
	dto := classificationDTO{
		Subject: classification.Subject().String(),
		Kind:    string(classification.Kind()),
	}
	if unresolved, exists := classification.(Unresolved); exists {
		dto.Reason = unresolved.Reason().String()
	}
	for _, observation := range classification.Observations() {
		dto.Observations = append(dto.Observations, observationDTOOf(observation))
	}
	return dto
}
