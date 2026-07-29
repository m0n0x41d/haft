package legacyimport

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

var (
	ErrCarrierCollision         = errors.New("legacy import carrier collision")
	ErrSourceCoordinateConflict = errors.New("legacy import source coordinate conflict")
)

type CarrierLegacyIdentity interface {
	carrierLegacyIdentityVariant()
}

type NoLegacyIdentity struct{}

func (NoLegacyIdentity) carrierLegacyIdentityVariant() {}

type IdentifiedLegacyCarrier struct {
	ref LegacyIdentityRef
}

func NewIdentifiedLegacyCarrier(ref LegacyIdentityRef) (IdentifiedLegacyCarrier, error) {
	if !ref.valid() {
		return IdentifiedLegacyCarrier{}, fmt.Errorf("legacy identity reference is required")
	}
	return IdentifiedLegacyCarrier{ref: ref}, nil
}

func (IdentifiedLegacyCarrier) carrierLegacyIdentityVariant() {}

func (identity IdentifiedLegacyCarrier) Ref() LegacyIdentityRef { return identity.ref }

type CarrierLocator struct {
	ref     typedmemory.CarrierRef
	edition typedmemory.CarrierEdition
	digest  typedmemory.SHA256Digest
}

type carrierLocatorKey struct {
	ref     string
	edition string
}

func (locator CarrierLocator) Ref() typedmemory.CarrierRef { return locator.ref }

func (locator CarrierLocator) Edition() typedmemory.CarrierEdition { return locator.edition }

func (locator CarrierLocator) Digest() typedmemory.SHA256Digest { return locator.digest }

func (locator CarrierLocator) valid() bool {
	return locator.ref.String() != "" && locator.edition.String() != "" && locator.digest.String() != ""
}

func (locator CarrierLocator) key() carrierLocatorKey {
	return carrierLocatorKey{
		ref:     locator.ref.String(),
		edition: locator.edition.String(),
	}
}

type CarrierSnapshot struct {
	coordinate     SourceCoordinate
	ref            typedmemory.CarrierRef
	edition        typedmemory.CarrierEdition
	digest         typedmemory.SHA256Digest
	exactBytes     []byte
	format         CarrierFormat
	legacyIdentity CarrierLegacyIdentity
}

func NewCarrierSnapshot(
	coordinate SourceCoordinate,
	ref typedmemory.CarrierRef,
	edition typedmemory.CarrierEdition,
	format CarrierFormat,
	exactBytes []byte,
	legacyIdentity CarrierLegacyIdentity,
) (CarrierSnapshot, error) {
	if !coordinate.valid() {
		return CarrierSnapshot{}, fmt.Errorf("carrier source coordinate is required")
	}
	if ref.String() == "" {
		return CarrierSnapshot{}, fmt.Errorf("carrier reference is required")
	}
	if edition.String() == "" {
		return CarrierSnapshot{}, fmt.Errorf("carrier edition is required")
	}
	if !format.valid() {
		return CarrierSnapshot{}, fmt.Errorf("carrier format is required")
	}
	if err := validateCarrierLegacyIdentity(legacyIdentity); err != nil {
		return CarrierSnapshot{}, err
	}
	ownedBytes := append([]byte(nil), exactBytes...)
	return CarrierSnapshot{
		coordinate:     coordinate,
		ref:            ref,
		edition:        edition,
		digest:         digestBytes(ownedBytes),
		exactBytes:     ownedBytes,
		format:         format,
		legacyIdentity: legacyIdentity,
	}, nil
}

func (snapshot CarrierSnapshot) SourceCoordinate() SourceCoordinate { return snapshot.coordinate }

func (snapshot CarrierSnapshot) Ref() typedmemory.CarrierRef { return snapshot.ref }

func (snapshot CarrierSnapshot) Edition() typedmemory.CarrierEdition { return snapshot.edition }

func (snapshot CarrierSnapshot) Digest() typedmemory.SHA256Digest { return snapshot.digest }

func (snapshot CarrierSnapshot) ExactBytes() []byte {
	return append([]byte(nil), snapshot.exactBytes...)
}

func (snapshot CarrierSnapshot) Format() CarrierFormat { return snapshot.format }

func (snapshot CarrierSnapshot) LegacyIdentity() CarrierLegacyIdentity {
	return snapshot.legacyIdentity
}

func (snapshot CarrierSnapshot) Locator() CarrierLocator {
	return CarrierLocator{
		ref:     snapshot.ref,
		edition: snapshot.edition,
		digest:  snapshot.digest,
	}
}

func (snapshot CarrierSnapshot) valid() bool {
	if !snapshot.coordinate.valid() || snapshot.ref.String() == "" || snapshot.edition.String() == "" {
		return false
	}
	if !snapshot.format.valid() || snapshot.digest.String() != digestBytes(snapshot.exactBytes).String() {
		return false
	}
	return validateCarrierLegacyIdentity(snapshot.legacyIdentity) == nil
}

func (snapshot CarrierSnapshot) canonicalBytes() []byte {
	encoded, _ := json.Marshal(carrierSnapshotDTOOf(snapshot))
	return encoded
}

type CarrierCatalog struct {
	values         []CarrierSnapshot
	byLocator      map[carrierLocatorKey]CarrierSnapshot
	canonicalBytes []byte
}

func NewCarrierCatalog(values []CarrierSnapshot) (CarrierCatalog, error) {
	owned := append([]CarrierSnapshot(nil), values...)
	sort.Slice(owned, func(left, right int) bool {
		return bytes.Compare(owned[left].canonicalBytes(), owned[right].canonicalBytes()) < 0
	})

	unique := make([]CarrierSnapshot, 0, len(owned))
	byLocator := make(map[carrierLocatorKey]CarrierSnapshot, len(owned))
	byCoordinate := make(map[string]CarrierSnapshot, len(owned))
	for index, snapshot := range owned {
		if !snapshot.valid() {
			return CarrierCatalog{}, fmt.Errorf("carrier snapshot %d is invalid", index)
		}
		key := snapshot.Locator().key()
		if prior, exists := byLocator[key]; exists {
			if bytes.Equal(prior.canonicalBytes(), snapshot.canonicalBytes()) {
				continue
			}
			return CarrierCatalog{}, fmt.Errorf(
				"%w: %s edition %s has digests %s and %s",
				ErrCarrierCollision,
				snapshot.Ref().String(),
				snapshot.Edition().String(),
				prior.Digest().String(),
				snapshot.Digest().String(),
			)
		}
		coordinate := snapshot.SourceCoordinate().String()
		if prior, exists := byCoordinate[coordinate]; exists {
			return CarrierCatalog{}, fmt.Errorf(
				"%w: %s names both %s and %s",
				ErrSourceCoordinateConflict,
				coordinate,
				prior.Ref().String(),
				snapshot.Ref().String(),
			)
		}
		byLocator[key] = snapshot
		byCoordinate[coordinate] = snapshot
		unique = append(unique, snapshot)
	}
	dto := make([]carrierSnapshotDTO, 0, len(unique))
	for _, snapshot := range unique {
		dto = append(dto, carrierSnapshotDTOOf(snapshot))
	}
	canonical, err := json.Marshal(dto)
	if err != nil {
		return CarrierCatalog{}, fmt.Errorf("encode carrier catalog: %w", err)
	}
	return CarrierCatalog{
		values:         unique,
		byLocator:      byLocator,
		canonicalBytes: canonical,
	}, nil
}

func (catalog CarrierCatalog) Snapshots() []CarrierSnapshot {
	return append([]CarrierSnapshot(nil), catalog.values...)
}

func (catalog CarrierCatalog) Len() int { return len(catalog.values) }

func (catalog CarrierCatalog) CanonicalBytes() []byte {
	return append([]byte(nil), catalog.canonicalBytes...)
}

func (catalog CarrierCatalog) Digest() typedmemory.SHA256Digest {
	return digestBytes(catalog.canonicalBytes)
}

func (catalog CarrierCatalog) contains(locator CarrierLocator) bool {
	observed, exists := catalog.byLocator[locator.key()]
	return exists && observed.Digest().String() == locator.Digest().String()
}

func validateCarrierLegacyIdentity(identity CarrierLegacyIdentity) error {
	switch observed := identity.(type) {
	case NoLegacyIdentity:
		return nil
	case IdentifiedLegacyCarrier:
		if !observed.ref.valid() {
			return fmt.Errorf("identified legacy carrier requires an identity reference")
		}
		return nil
	default:
		return fmt.Errorf("carrier legacy identity variant is unsupported")
	}
}

type carrierSnapshotDTO struct {
	SourceCoordinate  string `json:"source_coordinate"`
	CarrierRef        string `json:"carrier_ref"`
	Edition           string `json:"edition"`
	Digest            string `json:"digest"`
	Format            string `json:"format"`
	ExactBytes        []byte `json:"exact_bytes"`
	LegacyIdentityRef string `json:"legacy_identity_ref,omitempty"`
}

func carrierSnapshotDTOOf(snapshot CarrierSnapshot) carrierSnapshotDTO {
	legacyIdentity := ""
	identified, exists := snapshot.legacyIdentity.(IdentifiedLegacyCarrier)
	if exists {
		legacyIdentity = identified.Ref().String()
	}
	return carrierSnapshotDTO{
		SourceCoordinate:  snapshot.SourceCoordinate().String(),
		CarrierRef:        snapshot.Ref().String(),
		Edition:           snapshot.Edition().String(),
		Digest:            snapshot.Digest().String(),
		Format:            snapshot.Format().String(),
		ExactBytes:        snapshot.ExactBytes(),
		LegacyIdentityRef: legacyIdentity,
	}
}
