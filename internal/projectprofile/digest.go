package projectprofile

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash"
	"strconv"
	"time"
)

const (
	observedBasisDigestDomain = "haft.project-profile.observed-basis/v1"
	scopePayloadDigestDomain  = "haft.project-profile.scope-payload/v1"
)

type canonicalDigestWriter struct {
	hash hash.Hash
}

func newCanonicalDigestWriter(domain string) canonicalDigestWriter {
	writer := canonicalDigestWriter{hash: sha256.New()}
	writer.add(domain)
	return writer
}

func (writer canonicalDigestWriter) add(value string) {
	var size [8]byte
	binary.BigEndian.PutUint64(size[:], uint64(len(value)))
	_, _ = writer.hash.Write(size[:])
	_, _ = writer.hash.Write([]byte(value))
}

func (writer canonicalDigestWriter) digest() ContentDigest {
	value := "sha256:" + hex.EncodeToString(writer.hash.Sum(nil))
	return ContentDigest{value: value}
}

func DigestObservedBasis(values []ObservedBasis) (ContentDigest, error) {
	if err := validateObservedBasis(values); err != nil {
		return ContentDigest{}, err
	}
	writer := newCanonicalDigestWriter(observedBasisDigestDomain)
	addObservedBasis(writer, values)
	return writer.digest(), nil
}

func DigestScopePayload(scopes ScopeSet) (ContentDigest, error) {
	if !scopes.valid() {
		return ContentDigest{}, fmt.Errorf("scope payload is invalid")
	}
	writer := newCanonicalDigestWriter(scopePayloadDigestDomain)
	if err := addScopes(writer, scopes.Values()); err != nil {
		return ContentDigest{}, err
	}
	return writer.digest(), nil
}

func addScopes(writer canonicalDigestWriter, values []RealizationScope) error {
	writer.add(strconv.Itoa(len(values)))
	for _, value := range values {
		if err := addScope(writer, value); err != nil {
			return err
		}
	}
	return nil
}

func addScope(writer canonicalDigestWriter, scope RealizationScope) error {
	switch value := scope.(type) {
	case SoftwareRealization:
		writer.add("software")
		writer.add(value.ScopeID().String())
		return addEntityReference(writer, value.EntityReference())
	case NonSoftwareRealization:
		writer.add("non_software")
		writer.add(value.ScopeID().String())
		if err := addEntityReference(writer, value.EntityReference()); err != nil {
			return err
		}
		if err := addKindOrientation(writer, value.KindOrientation()); err != nil {
			return err
		}
		addSourceUnitRefs(writer, value.GoverningPatternRefs())
		addSpecSectionRefs(writer, value.ContractRefs())
		return nil
	default:
		return fmt.Errorf("cannot digest unknown realization scope variant")
	}
}

func addEntityReference(writer canonicalDigestWriter, reference EntityReference) error {
	switch value := reference.(type) {
	case NoEntityReference:
		writer.add("entity_ref:none")
		return nil
	case ReferencedEntity:
		writer.add("entity_ref:some")
		writer.add(value.Ref().String())
		return nil
	default:
		return fmt.Errorf("cannot digest unknown entity-reference variant")
	}
}

func addKindOrientation(writer canonicalDigestWriter, orientation KindOrientation) error {
	switch value := orientation.(type) {
	case UnspecifiedKindOrientation:
		// Keep the v1 digest vocabulary stable across the domain-only rename.
		writer.add("kind_ref:none")
		return nil
	case ReferencedKindOrientation:
		writer.add("kind_ref:some")
		writer.add(value.Ref().String())
		return nil
	default:
		return fmt.Errorf("cannot digest unknown kind-orientation variant")
	}
}

func addSourceUnitRefs(writer canonicalDigestWriter, values []SourceUnitRef) {
	writer.add(strconv.Itoa(len(values)))
	for _, value := range values {
		writer.add(value.String())
	}
}

func addSpecSectionRefs(writer canonicalDigestWriter, values []SpecSectionRef) {
	writer.add(strconv.Itoa(len(values)))
	for _, value := range values {
		writer.add(value.String())
	}
}

func addObservedBasis(writer canonicalDigestWriter, values []ObservedBasis) {
	writer.add(strconv.Itoa(len(values)))
	for _, value := range values {
		writer.add(value.Source())
		writer.add(value.Observation())
	}
}

func canonicalTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}
