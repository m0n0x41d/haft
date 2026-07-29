package typedmemorywire

import (
	"fmt"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

// DiagnosticChangeKind identifies the strict wire variant at one change
// ordinal. It is presentation metadata only: it grants no validation or
// admission capability.
type DiagnosticChangeKind uint8

const (
	DiagnosticChangeDeclareEntity DiagnosticChangeKind = iota + 1
	DiagnosticChangeIdentity
	DiagnosticChangeInstantiateRelation
	DiagnosticChangeRetractAssertion
	DiagnosticChangeAssertRelation
)

// DiagnosticIdentityKind identifies the nested identity-change wire variant.
type DiagnosticIdentityKind uint8

const (
	DiagnosticIdentityAdmitAlias DiagnosticIdentityKind = iota + 1
	DiagnosticIdentitySupersedeAlias
	DiagnosticIdentityMergeEntities
	DiagnosticIdentitySplitEntity
)

type diagnosticBindingCoordinate struct {
	slotKind    typedmemory.SlotKindID
	ordinal     uint64
	fillerCount uint64
}

type diagnosticChangeCoordinate struct {
	kind         DiagnosticChangeKind
	identityKind DiagnosticIdentityKind
	bindings     []diagnosticBindingCoordinate
}

// DiagnosticCoordinateIndex is an immutable, decoder-owned projection of the
// original request coordinates. In particular, relation binding ordinals are
// captured before typedmemory normalizes bindings into semantic order.
//
// The index deliberately carries no request values, raw bytes, validation
// evidence, or admission authority.
type DiagnosticCoordinateIndex struct {
	changes []diagnosticChangeCoordinate
}

func (index DiagnosticCoordinateIndex) ChangeCount() int {
	return len(index.changes)
}

func (index DiagnosticCoordinateIndex) ChangeKind(
	changeOrdinal uint64,
) (DiagnosticChangeKind, bool) {
	coordinate, found := index.change(changeOrdinal)
	if !found {
		return 0, false
	}
	return coordinate.kind, true
}

func (index DiagnosticCoordinateIndex) IdentityKind(
	changeOrdinal uint64,
) (DiagnosticIdentityKind, bool) {
	coordinate, found := index.change(changeOrdinal)
	if !found || coordinate.kind != DiagnosticChangeIdentity {
		return 0, false
	}
	if coordinate.identityKind == 0 {
		return 0, false
	}
	return coordinate.identityKind, true
}

func (index DiagnosticCoordinateIndex) BindingOrdinal(
	changeOrdinal uint64,
	slotKind typedmemory.SlotKindID,
) (uint64, bool) {
	coordinate, found := index.change(changeOrdinal)
	if !found || !diagnosticRelationChangeKind(coordinate.kind) {
		return 0, false
	}
	for _, binding := range coordinate.bindings {
		if binding.slotKind == slotKind {
			return binding.ordinal, true
		}
	}
	return 0, false
}

func (index DiagnosticCoordinateIndex) FillerCount(
	changeOrdinal uint64,
	slotKind typedmemory.SlotKindID,
) (uint64, bool) {
	coordinate, found := index.change(changeOrdinal)
	if !found || !diagnosticRelationChangeKind(coordinate.kind) {
		return 0, false
	}
	for _, binding := range coordinate.bindings {
		if binding.slotKind == slotKind {
			return binding.fillerCount, true
		}
	}
	return 0, false
}

func (index DiagnosticCoordinateIndex) change(
	changeOrdinal uint64,
) (diagnosticChangeCoordinate, bool) {
	if changeOrdinal >= uint64(len(index.changes)) {
		return diagnosticChangeCoordinate{}, false
	}
	return index.changes[changeOrdinal], true
}

func (index DiagnosticCoordinateIndex) validFor(changeCount int) bool {
	if changeCount <= 0 || len(index.changes) != changeCount {
		return false
	}
	for _, change := range index.changes {
		if !change.valid() {
			return false
		}
	}
	return true
}

func (coordinate diagnosticChangeCoordinate) valid() bool {
	switch coordinate.kind {
	case DiagnosticChangeDeclareEntity,
		DiagnosticChangeRetractAssertion:
		return coordinate.identityKind == 0 &&
			len(coordinate.bindings) == 0
	case DiagnosticChangeIdentity:
		return coordinate.identityKind != 0 && len(coordinate.bindings) == 0
	case DiagnosticChangeInstantiateRelation:
		return coordinate.identityKind == 0 &&
			validBindingCoordinates(coordinate.bindings)
	case DiagnosticChangeAssertRelation:
		return coordinate.identityKind == 0 &&
			validBindingCoordinates(coordinate.bindings)
	default:
		return false
	}
}

func diagnosticRelationChangeKind(kind DiagnosticChangeKind) bool {
	return kind == DiagnosticChangeInstantiateRelation ||
		kind == DiagnosticChangeAssertRelation
}

func validBindingCoordinates(values []diagnosticBindingCoordinate) bool {
	if len(values) == 0 {
		return false
	}
	seen := make(map[string]struct{}, len(values))
	for index, value := range values {
		if value.slotKind.String() == "" ||
			value.ordinal != uint64(index) ||
			value.fillerCount == 0 {
			return false
		}
		key := value.slotKind.String()
		if _, duplicate := seen[key]; duplicate {
			return false
		}
		seen[key] = struct{}{}
	}
	return true
}

func copyDiagnosticCoordinateIndex(
	index DiagnosticCoordinateIndex,
) DiagnosticCoordinateIndex {
	changes := make([]diagnosticChangeCoordinate, 0, len(index.changes))
	for _, change := range index.changes {
		copyValue := change
		copyValue.bindings = append([]diagnosticBindingCoordinate(nil), change.bindings...)
		changes = append(changes, copyValue)
	}
	return DiagnosticCoordinateIndex{changes: changes}
}

func diagnosticCoordinateForCandidate(
	candidate memoryChangeCandidate,
) (diagnosticChangeCoordinate, error) {
	switch value := candidate.(type) {
	case exactMemoryChangeCandidate:
		return diagnosticCoordinateForExactChange(value.change)
	case relationMemoryChangeCandidate:
		bindings := make([]diagnosticBindingCoordinate, 0, len(value.bindings))
		for ordinal, binding := range value.bindings {
			bindings = append(bindings, diagnosticBindingCoordinate{
				slotKind:    binding.slotKind,
				ordinal:     uint64(ordinal),
				fillerCount: uint64(len(binding.fillers)),
			})
		}
		return diagnosticChangeCoordinate{
			kind:     DiagnosticChangeInstantiateRelation,
			bindings: bindings,
		}, nil
	case relationalAssertionMemoryChangeCandidate:
		bindings := make([]diagnosticBindingCoordinate, 0, len(value.bindings))
		for ordinal, binding := range value.bindings {
			bindings = append(bindings, diagnosticBindingCoordinate{
				slotKind:    binding.slotKind,
				ordinal:     uint64(ordinal),
				fillerCount: uint64(len(binding.fillers)),
			})
		}
		return diagnosticChangeCoordinate{
			kind:     DiagnosticChangeAssertRelation,
			bindings: bindings,
		}, nil
	default:
		return diagnosticChangeCoordinate{}, fmt.Errorf(
			"unknown strict MemoryChange candidate %T",
			candidate,
		)
	}
}

func diagnosticCoordinateForExactChange(
	change typedmemory.MemoryChange,
) (diagnosticChangeCoordinate, error) {
	switch value := change.(type) {
	case typedmemory.DeclareEntity:
		return diagnosticChangeCoordinate{kind: DiagnosticChangeDeclareEntity}, nil
	case typedmemory.ApplyIdentityChange:
		identityKind, err := diagnosticIdentityKind(value.Change())
		if err != nil {
			return diagnosticChangeCoordinate{}, err
		}
		return diagnosticChangeCoordinate{
			kind:         DiagnosticChangeIdentity,
			identityKind: identityKind,
		}, nil
	case typedmemory.RetractAssertion:
		return diagnosticChangeCoordinate{kind: DiagnosticChangeRetractAssertion}, nil
	default:
		return diagnosticChangeCoordinate{}, fmt.Errorf(
			"unknown exact MemoryChange candidate %T",
			change,
		)
	}
}

func diagnosticIdentityKind(
	change typedmemory.IdentityChange,
) (DiagnosticIdentityKind, error) {
	switch change.(type) {
	case typedmemory.AdmitAlias:
		return DiagnosticIdentityAdmitAlias, nil
	case typedmemory.SupersedeAlias:
		return DiagnosticIdentitySupersedeAlias, nil
	case typedmemory.MergeEntities:
		return DiagnosticIdentityMergeEntities, nil
	case typedmemory.SplitEntity:
		return DiagnosticIdentitySplitEntity, nil
	default:
		return 0, fmt.Errorf("unknown strict IdentityChange candidate %T", change)
	}
}
