package recordcarrier

import (
	"fmt"
	"strings"

	"github.com/m0n0x41d/haft/internal/recordmapping"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	ProjectRecordCarrierSchemaVersionV1       = "haft.project-record-carrier/v1"
	EntityRecordCarrierBindingSchemaVersionV1 = "haft.entity-record-carrier-binding/v1"
	RecordMembershipSourceSchemaVersionV1     = "haft.record-membership-source/v1"
	RecordClassificationSourceSchemaVersionV1 = "haft.record-classification-source/v1"
	ProjectRecordCarrierEditionV1             = "1"
)

type MappingManifestRef = recordmapping.MappingManifestRef

func NewMappingManifestRef(
	id string,
	version string,
	digest typedmemory.SHA256Digest,
) (MappingManifestRef, error) {
	return recordmapping.NewMappingManifestRef(id, version, digest)
}

func ParseMappingManifestRef(raw string) (MappingManifestRef, error) {
	return recordmapping.ParseMappingManifestRef(raw)
}

type AdapterVersion = recordmapping.AdapterVersion

func NewAdapterVersion(raw string) (AdapterVersion, error) {
	return recordmapping.NewAdapterVersion(raw)
}

type EntityRecordCarrierBindingRef struct {
	digest typedmemory.SHA256Digest
}

func ParseEntityRecordCarrierBindingRef(raw string) (EntityRecordCarrierBindingRef, error) {
	digestRaw, found := strings.CutPrefix(raw, "entity-record-carrier-binding:")
	if !found {
		return EntityRecordCarrierBindingRef{}, fmt.Errorf("entity-record carrier binding reference is malformed")
	}
	digest, err := typedmemory.NewSHA256Digest(digestRaw)
	if err != nil {
		return EntityRecordCarrierBindingRef{}, fmt.Errorf("entity-record carrier binding reference: %w", err)
	}
	ref := EntityRecordCarrierBindingRef{digest: digest}
	if ref.String() != raw {
		return EntityRecordCarrierBindingRef{}, fmt.Errorf("entity-record carrier binding reference is not canonical")
	}
	return ref, nil
}

func (ref EntityRecordCarrierBindingRef) Digest() typedmemory.SHA256Digest { return ref.digest }

func (ref EntityRecordCarrierBindingRef) String() string {
	return "entity-record-carrier-binding:" + ref.digest.String()
}

func (ref EntityRecordCarrierBindingRef) valid() bool {
	parsed, err := ParseEntityRecordCarrierBindingRef(ref.String())
	return err == nil && parsed == ref
}

func parseExactEntityID(raw string) (typedmemory.EntityID, error) {
	value, err := typedmemory.NewEntityID(raw)
	if err != nil {
		return typedmemory.EntityID{}, err
	}
	if value.String() != raw {
		return typedmemory.EntityID{}, fmt.Errorf("EntityID is not canonical")
	}
	return value, nil
}

func parseExactBoundedContext(
	raw string,
) (typedmemory.BoundedContextRef, error) {
	value, err := typedmemory.NewBoundedContextRef(raw)
	if err != nil {
		return typedmemory.BoundedContextRef{}, err
	}
	if value.String() != raw {
		return typedmemory.BoundedContextRef{}, fmt.Errorf("bounded context is not canonical")
	}
	return value, nil
}

func parseExactCarrierRef(raw string) (typedmemory.CarrierRef, error) {
	value, err := typedmemory.NewCarrierRef(raw)
	if err != nil {
		return typedmemory.CarrierRef{}, err
	}
	if value.String() != raw {
		return typedmemory.CarrierRef{}, fmt.Errorf("carrier reference is not canonical")
	}
	return value, nil
}

func parseExactCarrierEdition(raw string) (typedmemory.CarrierEdition, error) {
	value, err := typedmemory.NewCarrierEdition(raw)
	if err != nil {
		return typedmemory.CarrierEdition{}, err
	}
	if value.String() != raw || value.String() != ProjectRecordCarrierEditionV1 {
		return typedmemory.CarrierEdition{}, fmt.Errorf("carrier edition is not exact and canonical")
	}
	return value, nil
}

func parseExactDigest(raw string) (typedmemory.SHA256Digest, error) {
	value, err := typedmemory.NewSHA256Digest(raw)
	if err != nil {
		return typedmemory.SHA256Digest{}, err
	}
	if value.String() != raw {
		return typedmemory.SHA256Digest{}, fmt.Errorf("digest is not canonical")
	}
	return value, nil
}
