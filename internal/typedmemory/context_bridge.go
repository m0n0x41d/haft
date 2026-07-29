package typedmemory

import (
	"bytes"
	"fmt"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	maximumKindBridgeTextItems     = 1 << 10
	maximumKindBridgeTextItemBytes = 16 << 10
	maximumKindBridgeTextSetBytes  = 4 << 20
)

// ContextEdition is one exact, non-floating vocabulary or Standard edition
// for a bounded context. It deliberately excludes moving selectors while
// leaving the source publication free to choose its exact edition syntax.
type ContextEdition struct {
	value string
}

func NewContextEdition(raw string) (ContextEdition, error) {
	value, err := parseOpaqueIdentifier("context edition", raw)
	if err != nil {
		return ContextEdition{}, err
	}
	if value != raw || !utf8.ValidString(raw) {
		return ContextEdition{}, fmt.Errorf("context edition must be exact canonical UTF-8")
	}
	switch strings.ToLower(value) {
	case "latest", "current", "head", "*":
		return ContextEdition{}, fmt.Errorf(
			"context edition must be exact rather than moving selector %q",
			value,
		)
	default:
		return ContextEdition{value: value}, nil
	}
}

func (edition ContextEdition) String() string { return edition.value }

func (edition ContextEdition) valid() bool {
	rebuilt, err := NewContextEdition(edition.value)
	return err == nil && rebuilt == edition
}

// ContextBridgeEndpoint binds one context to one exact vocabulary edition.
// The context without the edition is not a complete C.3.3 endpoint.
type ContextBridgeEndpoint struct {
	context BoundedContextRef
	edition ContextEdition
}

func NewContextBridgeEndpoint(
	context BoundedContextRef,
	edition ContextEdition,
) (ContextBridgeEndpoint, error) {
	if !context.valid() {
		return ContextBridgeEndpoint{}, fmt.Errorf("context bridge endpoint context is required")
	}
	if !edition.valid() {
		return ContextBridgeEndpoint{}, fmt.Errorf("context bridge endpoint exact edition is required")
	}
	return ContextBridgeEndpoint{context: context, edition: edition}, nil
}

func (endpoint ContextBridgeEndpoint) Context() BoundedContextRef { return endpoint.context }

func (endpoint ContextBridgeEndpoint) Edition() ContextEdition { return endpoint.edition }

func (endpoint ContextBridgeEndpoint) valid() bool {
	return endpoint.context.valid() && endpoint.edition.valid()
}

func (endpoint ContextBridgeEndpoint) canonicalBytes() []byte {
	writer := newCanonicalWriter("context-bridge-endpoint.v1")
	writer.addString(endpoint.context.String())
	writer.addString(endpoint.edition.String())
	return writer.bytes()
}

// NamedTargetKindMapping is the only supported C.3.3 v1 mapping form.
// Signature-translation rules remain inexpressible until their runtime
// lowering and evaluation contracts exist end to end.
type NamedTargetKindMapping struct {
	sourceKind KindID
	targetKind KindID
}

func NewNamedTargetKindMapping(
	sourceKind KindID,
	targetKind KindID,
) (NamedTargetKindMapping, error) {
	if !sourceKind.valid() || !targetKind.valid() {
		return NamedTargetKindMapping{}, fmt.Errorf(
			"named target kind mapping requires source and target kinds",
		)
	}
	return NamedTargetKindMapping{
		sourceKind: sourceKind,
		targetKind: targetKind,
	}, nil
}

func (mapping NamedTargetKindMapping) SourceKind() KindID { return mapping.sourceKind }

func (mapping NamedTargetKindMapping) TargetKind() KindID { return mapping.targetKind }

func (mapping NamedTargetKindMapping) valid() bool {
	return mapping.sourceKind.valid() && mapping.targetKind.valid()
}

func (mapping NamedTargetKindMapping) canonicalBytes() []byte {
	writer := newCanonicalWriter("context-bridge-mapping.named-target.v1")
	writer.addString(mapping.sourceKind.String())
	writer.addString(mapping.targetKind.String())
	return writer.bytes()
}

type BridgeDirection uint8

const (
	OneWayBridge BridgeDirection = iota + 1
	TwoWayBridge
)

func (direction BridgeDirection) String() string {
	switch direction {
	case OneWayBridge:
		return "one_way"
	case TwoWayBridge:
		return "two_way"
	default:
		return ""
	}
}

func (direction BridgeDirection) valid() bool { return direction.String() != "" }

// KindBridgeOrderCoverage states the exact source SubkindOf fragment covered
// by this runtime bridge. v1 can express only an explicitly empty fragment.
type KindBridgeOrderCoverage uint8

const NoOrderLinksCovered KindBridgeOrderCoverage = 1

func (coverage KindBridgeOrderCoverage) String() string {
	switch coverage {
	case NoOrderLinksCovered:
		return "no_links_covered"
	default:
		return ""
	}
}

func (coverage KindBridgeOrderCoverage) valid() bool { return coverage.String() != "" }

func parseKindBridgeOrderCoverage(raw string) (KindBridgeOrderCoverage, error) {
	if NoOrderLinksCovered.String() == raw {
		return NoOrderLinksCovered, nil
	}
	return 0, fmt.Errorf("unknown context-bridge order coverage %q", raw)
}

// KindCongruenceLevel is C.3.3 CL^k on the closed ordinal 0..3 ladder. Its
// zero Go representation is invalid even though the semantic value 0 is valid.
type KindCongruenceLevel uint8

func NewKindCongruenceLevel(value uint8) (KindCongruenceLevel, error) {
	if value > 3 {
		return 0, fmt.Errorf("kind-congruence level must be on the closed CL^k 0..3 ladder")
	}
	return KindCongruenceLevel(value + 1), nil
}

func (level KindCongruenceLevel) Value() uint8 {
	if !level.valid() {
		return 0
	}
	return uint8(level) - 1
}

func (level KindCongruenceLevel) valid() bool {
	return level >= KindCongruenceLevel(1) && level <= KindCongruenceLevel(4)
}

type canonicalKindBridgeTextSet struct {
	values    []string
	canonical []byte
}

// KindBridgeLossNotes is a nonempty canonical set of disclosed losses. An
// explicit "none" note is still a statement; an absent set is invalid.
type KindBridgeLossNotes struct {
	set canonicalKindBridgeTextSet
}

func NewKindBridgeLossNotes(values []string) (KindBridgeLossNotes, error) {
	set, err := newCanonicalKindBridgeTextSet(
		"context-bridge-loss-notes.v1",
		"loss note",
		values,
	)
	if err != nil {
		return KindBridgeLossNotes{}, err
	}
	return KindBridgeLossNotes{set: set}, nil
}

func (notes KindBridgeLossNotes) Values() []string {
	return append([]string(nil), notes.set.values...)
}

func (notes KindBridgeLossNotes) valid() bool {
	return validCanonicalKindBridgeTextSet(
		"context-bridge-loss-notes.v1",
		"loss note",
		notes.set,
	)
}

func (notes KindBridgeLossNotes) canonicalBytes() []byte {
	return append([]byte(nil), notes.set.canonical...)
}

func cloneKindBridgeLossNotes(notes KindBridgeLossNotes) KindBridgeLossNotes {
	notes.set = cloneCanonicalKindBridgeTextSet(notes.set)
	return notes
}

// KindBridgeDefinednessArea is the nonempty canonical set of conditions under
// which bridge-dependent guards may use the mapping. Outside it they fail
// closed; this type stores the declaration but does not evaluate conditions.
type KindBridgeDefinednessArea struct {
	set canonicalKindBridgeTextSet
}

func NewKindBridgeDefinednessArea(values []string) (KindBridgeDefinednessArea, error) {
	set, err := newCanonicalKindBridgeTextSet(
		"context-bridge-definedness-area.v1",
		"definedness condition",
		values,
	)
	if err != nil {
		return KindBridgeDefinednessArea{}, err
	}
	return KindBridgeDefinednessArea{set: set}, nil
}

func (area KindBridgeDefinednessArea) Values() []string {
	return append([]string(nil), area.set.values...)
}

func (area KindBridgeDefinednessArea) valid() bool {
	return validCanonicalKindBridgeTextSet(
		"context-bridge-definedness-area.v1",
		"definedness condition",
		area.set,
	)
}

func (area KindBridgeDefinednessArea) canonicalBytes() []byte {
	return append([]byte(nil), area.set.canonical...)
}

func cloneKindBridgeDefinednessArea(
	area KindBridgeDefinednessArea,
) KindBridgeDefinednessArea {
	area.set = cloneCanonicalKindBridgeTextSet(area.set)
	return area
}

func newCanonicalKindBridgeTextSet(
	domain string,
	label string,
	values []string,
) (canonicalKindBridgeTextSet, error) {
	if len(values) == 0 {
		return canonicalKindBridgeTextSet{}, fmt.Errorf(
			"context bridge requires at least one %s",
			label,
		)
	}
	if len(values) > maximumKindBridgeTextItems {
		return canonicalKindBridgeTextSet{}, fmt.Errorf(
			"context bridge %s count exceeds %d",
			label,
			maximumKindBridgeTextItems,
		)
	}
	owned := append([]string(nil), values...)
	for index, value := range owned {
		parsed, err := parseOpaqueIdentifier("context bridge "+label, value)
		if err != nil {
			return canonicalKindBridgeTextSet{}, err
		}
		if parsed != value || !utf8.ValidString(value) {
			return canonicalKindBridgeTextSet{}, fmt.Errorf(
				"context bridge %s %d must be exact canonical UTF-8",
				label,
				index,
			)
		}
		if len([]byte(value)) > maximumKindBridgeTextItemBytes {
			return canonicalKindBridgeTextSet{}, fmt.Errorf(
				"context bridge %s %d exceeds %d bytes",
				label,
				index,
				maximumKindBridgeTextItemBytes,
			)
		}
	}
	sort.Strings(owned)
	for index := 1; index < len(owned); index++ {
		if owned[index-1] == owned[index] {
			return canonicalKindBridgeTextSet{}, fmt.Errorf(
				"context bridge repeats %s %q",
				label,
				owned[index],
			)
		}
	}
	canonical := canonicalKindBridgeTextSetBytes(domain, owned)
	if len(canonical) > maximumKindBridgeTextSetBytes {
		return canonicalKindBridgeTextSet{}, fmt.Errorf(
			"context bridge %s set exceeds %d canonical bytes",
			label,
			maximumKindBridgeTextSetBytes,
		)
	}
	return canonicalKindBridgeTextSet{values: owned, canonical: canonical}, nil
}

func validCanonicalKindBridgeTextSet(
	domain string,
	label string,
	set canonicalKindBridgeTextSet,
) bool {
	rebuilt, err := newCanonicalKindBridgeTextSet(domain, label, set.values)
	return err == nil &&
		slices.Equal(rebuilt.values, set.values) &&
		bytes.Equal(rebuilt.canonical, set.canonical)
}

func canonicalKindBridgeTextSetBytes(domain string, values []string) []byte {
	writer := newCanonicalWriter(domain)
	writer.addUint64(uint64(len(values)))
	for _, value := range values {
		writer.addString(value)
	}
	return writer.bytes()
}

func cloneCanonicalKindBridgeTextSet(
	set canonicalKindBridgeTextSet,
) canonicalKindBridgeTextSet {
	set.values = append([]string(nil), set.values...)
	set.canonical = append([]byte(nil), set.canonical...)
	return set
}

// ContextBridgeInput is the complete supported C.3.3 v1 runtime contract.
// No field is inferred from ID, source order, or surrounding project state.
type ContextBridgeInput struct {
	ID              ContextBridgeID
	Source          ContextBridgeEndpoint
	Target          ContextBridgeEndpoint
	Mapping         NamedTargetKindMapping
	Direction       BridgeDirection
	OrderCoverage   KindBridgeOrderCoverage
	KindCongruence  KindCongruenceLevel
	LossNotes       KindBridgeLossNotes
	DefinednessArea KindBridgeDefinednessArea
	Provenance      DeclarationProvenance
}

type ContextBridge struct {
	id              ContextBridgeID
	source          ContextBridgeEndpoint
	target          ContextBridgeEndpoint
	mapping         NamedTargetKindMapping
	direction       BridgeDirection
	orderCoverage   KindBridgeOrderCoverage
	kindCongruence  KindCongruenceLevel
	lossNotes       KindBridgeLossNotes
	definednessArea KindBridgeDefinednessArea
	provenance      DeclarationProvenance
	canonical       []byte
}

func NewContextBridge(input ContextBridgeInput) (ContextBridge, error) {
	if !input.ID.valid() {
		return ContextBridge{}, fmt.Errorf("context bridge ID is required")
	}
	if !input.Source.valid() || !input.Target.valid() {
		return ContextBridge{}, fmt.Errorf("context bridge exact endpoints are required")
	}
	if input.Source.Context() == input.Target.Context() {
		return ContextBridge{}, fmt.Errorf("context bridge endpoint contexts must differ")
	}
	if !input.Mapping.valid() {
		return ContextBridge{}, fmt.Errorf("context bridge named target mapping is required")
	}
	if !input.Direction.valid() {
		return ContextBridge{}, fmt.Errorf("context bridge direction is required")
	}
	if !input.OrderCoverage.valid() {
		return ContextBridge{}, fmt.Errorf("context bridge order coverage is required")
	}
	if !input.KindCongruence.valid() {
		return ContextBridge{}, fmt.Errorf("context bridge CL^k is required")
	}
	if !input.LossNotes.valid() {
		return ContextBridge{}, fmt.Errorf("context bridge canonical loss notes are required")
	}
	if !input.DefinednessArea.valid() {
		return ContextBridge{}, fmt.Errorf("context bridge canonical definedness area is required")
	}
	if !validDeclarationProvenance(input.Provenance) {
		return ContextBridge{}, fmt.Errorf("context bridge provenance is required")
	}
	bridge := ContextBridge{
		id:              input.ID,
		source:          input.Source,
		target:          input.Target,
		mapping:         input.Mapping,
		direction:       input.Direction,
		orderCoverage:   input.OrderCoverage,
		kindCongruence:  input.KindCongruence,
		lossNotes:       cloneKindBridgeLossNotes(input.LossNotes),
		definednessArea: cloneKindBridgeDefinednessArea(input.DefinednessArea),
		provenance:      cloneDeclarationProvenance(input.Provenance),
	}
	bridge.canonical = canonicalContextBridge(bridge)
	return cloneContextBridge(bridge), nil
}

func (bridge ContextBridge) ID() ContextBridgeID { return bridge.id }

func (bridge ContextBridge) Source() ContextBridgeEndpoint { return bridge.source }

func (bridge ContextBridge) Target() ContextBridgeEndpoint { return bridge.target }

func (bridge ContextBridge) Mapping() NamedTargetKindMapping { return bridge.mapping }

func (bridge ContextBridge) Direction() BridgeDirection { return bridge.direction }

func (bridge ContextBridge) OrderCoverage() KindBridgeOrderCoverage {
	return bridge.orderCoverage
}

func (bridge ContextBridge) KindCongruence() KindCongruenceLevel {
	return bridge.kindCongruence
}

func (bridge ContextBridge) LossNotes() KindBridgeLossNotes {
	return cloneKindBridgeLossNotes(bridge.lossNotes)
}

func (bridge ContextBridge) DefinednessArea() KindBridgeDefinednessArea {
	return cloneKindBridgeDefinednessArea(bridge.definednessArea)
}

func (bridge ContextBridge) Provenance() DeclarationProvenance {
	return cloneDeclarationProvenance(bridge.provenance)
}

func (bridge ContextBridge) CanonicalBytes() []byte {
	return append([]byte(nil), bridge.canonical...)
}

func (bridge ContextBridge) Digest() SHA256Digest {
	return digestCanonicalBytes(bridge.canonical)
}

func (bridge ContextBridge) AllowsMapping(
	source BoundedContextRef,
	sourceKind KindID,
	target BoundedContextRef,
	targetKind KindID,
) bool {
	if !bridge.valid() {
		return false
	}
	forward := bridge.source.Context() == source &&
		bridge.target.Context() == target &&
		bridge.mapping.SourceKind() == sourceKind &&
		bridge.mapping.TargetKind() == targetKind
	if forward {
		return true
	}
	return bridge.direction == TwoWayBridge &&
		bridge.target.Context() == source &&
		bridge.source.Context() == target &&
		bridge.mapping.TargetKind() == sourceKind &&
		bridge.mapping.SourceKind() == targetKind
}

func (bridge ContextBridge) valid() bool {
	if !bridge.id.valid() || !bridge.source.valid() || !bridge.target.valid() {
		return false
	}
	if bridge.source.Context() == bridge.target.Context() || !bridge.mapping.valid() {
		return false
	}
	if !bridge.direction.valid() || !bridge.orderCoverage.valid() {
		return false
	}
	if !bridge.kindCongruence.valid() || !bridge.lossNotes.valid() {
		return false
	}
	if !bridge.definednessArea.valid() || !validDeclarationProvenance(bridge.provenance) {
		return false
	}
	expected := canonicalContextBridge(bridge)
	return bytes.Equal(bridge.canonical, expected)
}

func canonicalContextBridge(bridge ContextBridge) []byte {
	writer := newCanonicalWriter("context-bridge.v1")
	writer.addString(bridge.id.String())
	writer.addBytes(bridge.source.canonicalBytes())
	writer.addBytes(bridge.target.canonicalBytes())
	writer.addBytes(bridge.mapping.canonicalBytes())
	writer.addString(bridge.direction.String())
	writer.addString(bridge.orderCoverage.String())
	writer.addUint64(uint64(bridge.kindCongruence.Value()))
	writer.addBytes(bridge.lossNotes.canonicalBytes())
	writer.addBytes(bridge.definednessArea.canonicalBytes())
	writer.addBytes(bridge.provenance.CanonicalBytes())
	return writer.bytes()
}

func cloneContextBridge(bridge ContextBridge) ContextBridge {
	bridge.lossNotes = cloneKindBridgeLossNotes(bridge.lossNotes)
	bridge.definednessArea = cloneKindBridgeDefinednessArea(bridge.definednessArea)
	bridge.provenance = cloneDeclarationProvenance(bridge.provenance)
	bridge.canonical = append([]byte(nil), bridge.canonical...)
	return bridge
}

func cloneContextBridges(bridges []ContextBridge) []ContextBridge {
	result := make([]ContextBridge, 0, len(bridges))
	for _, bridge := range bridges {
		result = append(result, cloneContextBridge(bridge))
	}
	return result
}
