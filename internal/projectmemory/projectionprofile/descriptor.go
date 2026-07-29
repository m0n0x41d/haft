// Package projectionprofile owns the immutable compatibility descriptors for
// every installed ProjectionProfile edition. It deliberately stays below the
// neighborhood runtime: TypeEnv transition checks need exact profile identity,
// declared SlotKind reads, and affected facets, but never graph adapters or
// read assembly.
package projectionprofile

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

var refPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*\.v[1-9][0-9]*$`)

// Ref identifies one immutable installed ProjectionProfile edition.
type Ref struct {
	value string
}

func ParseRef(raw string) (Ref, error) {
	value := strings.TrimSpace(raw)
	if value != raw || !refPattern.MatchString(value) {
		return Ref{}, fmt.Errorf(
			"projection profile reference must be canonical <name>.v<edition>",
		)
	}
	return Ref{value: value}, nil
}

func (ref Ref) String() string {
	return ref.value
}

func (ref Ref) Valid() bool {
	return ref.valid()
}

func (ref Ref) Edition() (uint32, bool) {
	return ref.edition()
}

func (ref Ref) valid() bool {
	parsed, err := ParseRef(ref.value)
	return err == nil && parsed == ref
}

func (ref Ref) edition() (uint32, bool) {
	if !ref.valid() {
		return 0, false
	}
	index := strings.LastIndex(ref.value, ".v")
	raw := ref.value[index+2:]
	value, err := strconv.ParseUint(raw, 10, 32)
	return uint32(value), err == nil && value > 0
}

// FacetKind is a presentation grouping whose compatibility can degrade or
// become blocked when one of the exact profile's declared SlotKind reads
// changes. It conveys no priority, Work order, or authority.
type FacetKind string

const (
	FacetEpistemes      FacetKind = "epistemes"
	FacetProblems       FacetKind = "problems"
	FacetAlternatives   FacetKind = "alternatives"
	FacetDecisions      FacetKind = "decisions"
	FacetSpecifications FacetKind = "specifications"
	FacetEvidence       FacetKind = "evidence"
	FacetWork           FacetKind = "work"
	FacetImplementation FacetKind = "implementation"
	FacetUnresolved     FacetKind = "unresolved"
)

var knownFacetKinds = []FacetKind{
	FacetEpistemes,
	FacetProblems,
	FacetAlternatives,
	FacetDecisions,
	FacetSpecifications,
	FacetEvidence,
	FacetWork,
	FacetImplementation,
	FacetUnresolved,
}

func (kind FacetKind) Valid() bool {
	return slices.Contains(knownFacetKinds, kind)
}

func KnownFacetKinds() []FacetKind {
	return append([]FacetKind(nil), knownFacetKinds...)
}

// Descriptor is the lower-level immutable projection of one installed
// ProjectionProfile needed by TypeEnv transition compatibility. Digest is the
// full runtime profile digest, not a digest of this reduced descriptor.
type Descriptor struct {
	ref       Ref
	edition   uint32
	digest    typedmemory.SHA256Digest
	facets    []FacetKind
	slotReads []typedmemory.SlotKindID
}

func (descriptor Descriptor) Ref() Ref {
	return descriptor.ref
}

func (descriptor Descriptor) Edition() uint32 {
	return descriptor.edition
}

func (descriptor Descriptor) Digest() typedmemory.SHA256Digest {
	return descriptor.digest
}

func (descriptor Descriptor) Facets() []FacetKind {
	return append([]FacetKind(nil), descriptor.facets...)
}

func (descriptor Descriptor) SlotReads() []typedmemory.SlotKindID {
	return append([]typedmemory.SlotKindID(nil), descriptor.slotReads...)
}

func (descriptor Descriptor) AllowsFacet(facet FacetKind) bool {
	return descriptor.Valid() && slices.Contains(descriptor.facets, facet)
}

func (descriptor Descriptor) Valid() bool {
	refEdition, found := descriptor.ref.edition()
	if !found || refEdition != descriptor.edition || descriptor.edition == 0 {
		return false
	}
	if _, err := typedmemory.NewSHA256Digest(descriptor.digest.String()); err != nil {
		return false
	}
	if !validFacets(descriptor.facets) {
		return false
	}
	return validSlotReads(descriptor.slotReads)
}

var installed = buildInstalled()

// Installed returns every exact ProjectionProfile edition covered by TypeEnv
// transition compatibility in canonical identity order.
func Installed() []Descriptor {
	return append([]Descriptor(nil), installed...)
}

func Lookup(ref Ref) (Descriptor, bool) {
	if !ref.valid() {
		return Descriptor{}, false
	}
	index, found := slices.BinarySearchFunc(
		installed,
		ref,
		func(descriptor Descriptor, candidate Ref) int {
			return strings.Compare(
				descriptor.Ref().String(),
				candidate.String(),
			)
		},
	)
	if !found {
		return Descriptor{}, false
	}
	return installed[index], true
}

func buildInstalled() []Descriptor {
	allFacets := append([]FacetKind(nil), knownFacetKinds...)
	values := []Descriptor{
		mustDescriptor(
			"agent_orientation.v1",
			1,
			"sha256:1abb9b2c0b37a1c09b26f06bea257c9ce82cb14e1e0a1c7f8741e4b3889f867b",
			allFacets,
		),
		mustDescriptor(
			"agent_orientation.v2",
			2,
			"sha256:fe4c1c013b423e5fe16fcfd454d375696e5424d9c305074097fce80879e4a06c",
			allFacets,
		),
		mustDescriptor(
			"decision_rationale.v1",
			1,
			"sha256:aa3ae9f4c1f6645bb91e3cd5120133fab0f876622655b5399c4dca349bc3aff7",
			[]FacetKind{
				FacetProblems,
				FacetAlternatives,
				FacetDecisions,
				FacetEpistemes,
				FacetEvidence,
				FacetUnresolved,
			},
		),
		mustDescriptor(
			"evidence_currentness.v1",
			1,
			"sha256:6fcf246991f9af6370324672d52f89ff51c1c1265f42cdc418f37f85f66cab7c",
			[]FacetKind{
				FacetEvidence,
				FacetWork,
				FacetDecisions,
				FacetSpecifications,
				FacetEpistemes,
				FacetUnresolved,
			},
		),
		mustDescriptor(
			"implementation_trace.v1",
			1,
			"sha256:fc58a9e8c22e9895868791e295b66d09c96c7757616e34822e43c8846acd8926",
			[]FacetKind{
				FacetSpecifications,
				FacetEpistemes,
				FacetDecisions,
				FacetWork,
				FacetEvidence,
				FacetImplementation,
				FacetUnresolved,
			},
		),
		mustDescriptor(
			"spec_impact.v1",
			1,
			"sha256:7059e102821b431ea26d4bd9026e797a46ddcbb7c03d0cbb17d784e60dace2f1",
			[]FacetKind{
				FacetSpecifications,
				FacetEpistemes,
				FacetDecisions,
				FacetEvidence,
				FacetWork,
				FacetImplementation,
				FacetUnresolved,
			},
		),
	}
	sort.Slice(values, func(left int, right int) bool {
		return values[left].Ref().String() < values[right].Ref().String()
	})
	return values
}

func mustDescriptor(
	rawRef string,
	edition uint32,
	rawDigest string,
	facets []FacetKind,
) Descriptor {
	ref, err := ParseRef(rawRef)
	if err != nil {
		panic(err)
	}
	digest, err := typedmemory.NewSHA256Digest(rawDigest)
	if err != nil {
		panic(err)
	}
	descriptor := Descriptor{
		ref:       ref,
		edition:   edition,
		digest:    digest,
		facets:    append([]FacetKind(nil), facets...),
		slotReads: commonSlotReads(),
	}
	if !descriptor.Valid() {
		panic("installed projection profile descriptor is invalid")
	}
	return descriptor
}

func commonSlotReads() []typedmemory.SlotKindID {
	raw := []string{
		"ClaimGraphSlot",
		"EntityOfConcernSlot",
		"GroundingHolonSlot",
		"ReferenceSchemeSlot",
		"RepresentationSchemeSlot",
		"ViewpointSlot",
	}
	values := make([]typedmemory.SlotKindID, 0, len(raw))
	for _, item := range raw {
		slot, err := typedmemory.NewSlotKindID(item)
		if err != nil {
			panic(err)
		}
		values = append(values, slot)
	}
	return values
}

func validFacets(values []FacetKind) bool {
	if len(values) == 0 {
		return false
	}
	seen := make(map[FacetKind]struct{}, len(values))
	for _, value := range values {
		if !value.Valid() {
			return false
		}
		if _, found := seen[value]; found {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func validSlotReads(values []typedmemory.SlotKindID) bool {
	if len(values) == 0 {
		return false
	}
	for index, value := range values {
		parsed, err := typedmemory.NewSlotKindID(value.String())
		if err != nil || parsed != value {
			return false
		}
		if index > 0 && values[index-1].String() >= value.String() {
			return false
		}
	}
	return true
}
