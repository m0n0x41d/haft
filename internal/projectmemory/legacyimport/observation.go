package legacyimport

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

var ErrUnknownCarrier = fmt.Errorf("legacy import observation names an unknown carrier")

type SubjectObservation interface {
	Subject() SemanticSubjectRef
	Carrier() CarrierLocator
	canonicalBytes() []byte
	subjectObservationVariant()
}

type CarrierObservation struct {
	subject SemanticSubjectRef
	carrier CarrierLocator
}

func NewCarrierObservation(
	subject SemanticSubjectRef,
	carrier CarrierSnapshot,
) (CarrierObservation, error) {
	if !subject.valid() {
		return CarrierObservation{}, fmt.Errorf("carrier observation subject is required")
	}
	if !carrier.valid() {
		return CarrierObservation{}, fmt.Errorf("carrier observation snapshot is invalid")
	}
	return CarrierObservation{
		subject: subject,
		carrier: carrier.Locator(),
	}, nil
}

func (observation CarrierObservation) Subject() SemanticSubjectRef { return observation.subject }

func (observation CarrierObservation) Carrier() CarrierLocator { return observation.carrier }

func (CarrierObservation) subjectObservationVariant() {}

func (observation CarrierObservation) valid() bool {
	return observation.subject.valid() && observation.carrier.valid()
}

func (observation CarrierObservation) canonicalBytes() []byte {
	encoded, _ := json.Marshal(observationDTOOf(observation))
	return encoded
}

type AssociationObservation struct {
	subject SemanticSubjectRef
	carrier CarrierLocator
	source  LegacyIdentityRef
	target  LegacyIdentityRef
	label   AssociationLabel
}

func NewAssociationObservation(
	subject SemanticSubjectRef,
	carrier CarrierSnapshot,
	source LegacyIdentityRef,
	target LegacyIdentityRef,
	label AssociationLabel,
) (AssociationObservation, error) {
	if !subject.valid() {
		return AssociationObservation{}, fmt.Errorf("association observation subject is required")
	}
	if !carrier.valid() {
		return AssociationObservation{}, fmt.Errorf("association observation carrier is invalid")
	}
	if !source.valid() || !target.valid() {
		return AssociationObservation{}, fmt.Errorf("association observation requires exact source and target legacy identities")
	}
	if !label.valid() {
		return AssociationObservation{}, fmt.Errorf("association observation label is required")
	}
	return AssociationObservation{
		subject: subject,
		carrier: carrier.Locator(),
		source:  source,
		target:  target,
		label:   label,
	}, nil
}

func (observation AssociationObservation) Subject() SemanticSubjectRef {
	return observation.subject
}

func (observation AssociationObservation) Carrier() CarrierLocator { return observation.carrier }

func (observation AssociationObservation) Source() LegacyIdentityRef { return observation.source }

func (observation AssociationObservation) Target() LegacyIdentityRef { return observation.target }

func (observation AssociationObservation) Label() AssociationLabel { return observation.label }

func (AssociationObservation) subjectObservationVariant() {}

func (observation AssociationObservation) valid() bool {
	return observation.subject.valid() &&
		observation.carrier.valid() &&
		observation.source.valid() &&
		observation.target.valid() &&
		observation.label.valid()
}

func (observation AssociationObservation) canonicalBytes() []byte {
	encoded, _ := json.Marshal(observationDTOOf(observation))
	return encoded
}

type ObservationSet struct {
	values         []SubjectObservation
	canonicalBytes []byte
}

func NewObservationSet(values []SubjectObservation) (ObservationSet, error) {
	owned := append([]SubjectObservation(nil), values...)
	for index, observation := range owned {
		if err := validateObservation(observation); err != nil {
			return ObservationSet{}, fmt.Errorf("observation %d: %w", index, err)
		}
	}
	sort.Slice(owned, func(left, right int) bool {
		return bytes.Compare(owned[left].canonicalBytes(), owned[right].canonicalBytes()) < 0
	})
	unique := make([]SubjectObservation, 0, len(owned))
	seen := map[string]struct{}{}
	for _, observation := range owned {
		key := string(observation.canonicalBytes())
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, observation)
	}
	dto := make([]observationDTO, 0, len(unique))
	for _, observation := range unique {
		dto = append(dto, observationDTOOf(observation))
	}
	canonical, err := json.Marshal(dto)
	if err != nil {
		return ObservationSet{}, fmt.Errorf("encode observation set: %w", err)
	}
	return ObservationSet{
		values:         unique,
		canonicalBytes: canonical,
	}, nil
}

func (set ObservationSet) Values() []SubjectObservation {
	return append([]SubjectObservation(nil), set.values...)
}

func (set ObservationSet) CanonicalBytes() []byte {
	return append([]byte(nil), set.canonicalBytes...)
}

func (set ObservationSet) Digest() typedmemory.SHA256Digest {
	return digestBytes(set.canonicalBytes)
}

func validateObservation(observation SubjectObservation) error {
	if observation == nil {
		return fmt.Errorf("subject observation is required")
	}
	switch observed := observation.(type) {
	case CarrierObservation:
		if !observed.valid() {
			return fmt.Errorf("carrier observation is invalid")
		}
	case AssociationObservation:
		if !observed.valid() {
			return fmt.Errorf("association observation is invalid")
		}
	default:
		return fmt.Errorf("subject observation variant is unsupported")
	}
	return nil
}

type observationDTO struct {
	Kind       string `json:"kind"`
	Subject    string `json:"subject"`
	CarrierRef string `json:"carrier_ref"`
	Edition    string `json:"edition"`
	Digest     string `json:"digest"`
	Source     string `json:"source,omitempty"`
	Target     string `json:"target,omitempty"`
	Label      string `json:"label,omitempty"`
}

func observationDTOOf(observation SubjectObservation) observationDTO {
	base := observationDTO{
		Subject:    observation.Subject().String(),
		CarrierRef: observation.Carrier().Ref().String(),
		Edition:    observation.Carrier().Edition().String(),
		Digest:     observation.Carrier().Digest().String(),
	}
	switch observed := observation.(type) {
	case CarrierObservation:
		base.Kind = "carrier"
	case AssociationObservation:
		base.Kind = "association"
		base.Source = observed.Source().String()
		base.Target = observed.Target().String()
		base.Label = observed.Label().String()
	}
	return base
}

type LegacySourceSnapshot struct {
	catalog        CarrierCatalog
	observations   ObservationSet
	canonicalBytes []byte
}

func NewLegacySourceSnapshot(
	catalog CarrierCatalog,
	observations ObservationSet,
) (LegacySourceSnapshot, error) {
	if len(catalog.canonicalBytes) == 0 {
		return LegacySourceSnapshot{}, fmt.Errorf("legacy source carrier catalog is required")
	}
	if len(observations.canonicalBytes) == 0 {
		return LegacySourceSnapshot{}, fmt.Errorf("legacy source observation set is required")
	}
	for _, observation := range observations.values {
		if catalog.contains(observation.Carrier()) {
			continue
		}
		return LegacySourceSnapshot{}, fmt.Errorf(
			"%w: %s edition %s digest %s",
			ErrUnknownCarrier,
			observation.Carrier().Ref().String(),
			observation.Carrier().Edition().String(),
			observation.Carrier().Digest().String(),
		)
	}
	payload := legacySourceSnapshotDTO{
		CarrierCatalog: json.RawMessage(catalog.CanonicalBytes()),
		Observations:   json.RawMessage(observations.CanonicalBytes()),
	}
	canonical, err := json.Marshal(payload)
	if err != nil {
		return LegacySourceSnapshot{}, fmt.Errorf("encode legacy source snapshot: %w", err)
	}
	return LegacySourceSnapshot{
		catalog:        catalog,
		observations:   observations,
		canonicalBytes: canonical,
	}, nil
}

func (snapshot LegacySourceSnapshot) Catalog() CarrierCatalog { return snapshot.catalog }

func (snapshot LegacySourceSnapshot) Observations() ObservationSet { return snapshot.observations }

func (snapshot LegacySourceSnapshot) CanonicalBytes() []byte {
	return append([]byte(nil), snapshot.canonicalBytes...)
}

func (snapshot LegacySourceSnapshot) Digest() typedmemory.SHA256Digest {
	return digestBytes(snapshot.canonicalBytes)
}

func (snapshot LegacySourceSnapshot) valid() bool {
	return len(snapshot.canonicalBytes) > 0 &&
		len(snapshot.catalog.canonicalBytes) > 0 &&
		len(snapshot.observations.canonicalBytes) > 0
}

type legacySourceSnapshotDTO struct {
	CarrierCatalog json.RawMessage `json:"carrier_catalog"`
	Observations   json.RawMessage `json:"observations"`
}
