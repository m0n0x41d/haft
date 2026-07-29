package typedmemoryvalidation

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorywire"
)

type DiagnosticPathKind string

const (
	// DiagnosticPathRequestJSON identifies a path into the exact strict request
	// that produced the candidate.
	DiagnosticPathRequestJSON DiagnosticPathKind = "request_json_path"
	// DiagnosticPathResponseJSON identifies a path into the validation response
	// when failure occurred while projecting that response rather than at an
	// addressable request value.
	DiagnosticPathResponseJSON DiagnosticPathKind = "response_json_path"
	// DiagnosticPathTypedMemorySemantic identifies an internal semantic
	// coordinate with no corresponding input location. Its rendered value is
	// explicitly prefixed and must not be interpreted as JSONPath.
	DiagnosticPathTypedMemorySemantic DiagnosticPathKind = "typed_memory_semantic_path"
)

const typedMemorySemanticPathPrefix = "typed-memory-semantic:"

type projectedDiagnosticPath struct {
	kind  DiagnosticPathKind
	value string
}

type diagnosticCoordinateCarrier interface {
	DiagnosticCoordinates() typedmemorywire.DiagnosticCoordinateIndex
}

type diagnosticPathProjector struct {
	coordinates typedmemorywire.DiagnosticCoordinateIndex
}

type coreDiagnosticPathProjector interface {
	project(
		typedmemory.DiagnosticCode,
		string,
		typedmemory.DiagnosticPath,
	) (projectedDiagnosticPath, error)
}

// semanticDiagnosticPathProjector is used only for an already-typed internal
// candidate. Such a candidate has no originating JSON carrier whose field
// ordinals could be reported honestly, so core paths remain explicitly typed
// memory semantic paths.
type semanticDiagnosticPathProjector struct{}

func (semanticDiagnosticPathProjector) project(
	_ typedmemory.DiagnosticCode,
	_ string,
	path typedmemory.DiagnosticPath,
) (projectedDiagnosticPath, error) {
	return semanticDiagnosticPath(path.String()), nil
}

func newDiagnosticPathProjector(
	request validationRequest,
) (diagnosticPathProjector, error) {
	carrier, ok := request.(diagnosticCoordinateCarrier)
	if !ok {
		return diagnosticPathProjector{}, fmt.Errorf(
			"strict request diagnostic coordinates are unavailable",
		)
	}
	coordinates := carrier.DiagnosticCoordinates()
	if coordinates.ChangeCount() != request.ChangeCount() {
		return diagnosticPathProjector{}, fmt.Errorf(
			"strict request diagnostic coordinate count %d does not match change count %d",
			coordinates.ChangeCount(),
			request.ChangeCount(),
		)
	}
	return diagnosticPathProjector{coordinates: coordinates}, nil
}

func (projector diagnosticPathProjector) project(
	code typedmemory.DiagnosticCode,
	message string,
	path typedmemory.DiagnosticPath,
) (projectedDiagnosticPath, error) {
	relative := path.String()
	if normalizedFillerSemanticDiagnostic(code, message, relative) {
		return semanticDiagnosticPath(relative), nil
	}
	if relative == "changes" {
		return requestJSONDiagnosticPath("$.change_set.changes"), nil
	}
	changeOrdinal, remainder, parsed := parseChangeDiagnosticPath(relative)
	if !parsed {
		return semanticDiagnosticPath(relative), nil
	}
	kind, found := projector.coordinates.ChangeKind(changeOrdinal)
	if !found {
		return projectedDiagnosticPath{}, fmt.Errorf(
			"core diagnostic path %q addresses absent change ordinal %d",
			relative,
			changeOrdinal,
		)
	}
	base := fmt.Sprintf("$.change_set.changes[%d]", changeOrdinal)
	if remainder == "" {
		return requestJSONDiagnosticPath(base), nil
	}

	switch kind {
	case typedmemorywire.DiagnosticChangeDeclareEntity:
		return projectDeclareEntityDiagnosticPath(relative, base, remainder), nil
	case typedmemorywire.DiagnosticChangeIdentity:
		return projector.projectIdentityDiagnosticPath(
			changeOrdinal,
			relative,
			base,
			remainder,
		)
	case typedmemorywire.DiagnosticChangeInstantiateRelation:
		return projector.projectRelationDiagnosticPath(
			changeOrdinal,
			code,
			relative,
			base,
			remainder,
		)
	case typedmemorywire.DiagnosticChangeRetractAssertion:
		return projectRetractionDiagnosticPath(relative, base, remainder), nil
	default:
		return projectedDiagnosticPath{}, fmt.Errorf(
			"strict request has unknown diagnostic change kind %d",
			kind,
		)
	}
}

func projectDeclareEntityDiagnosticPath(
	semantic string,
	base string,
	remainder string,
) projectedDiagnosticPath {
	field := remainder
	if remainder == "bounded_context" {
		field = "context"
	}
	if exactField(field, "kind", "entity_id", "local_ref", "context", "label", "provenance") {
		return requestJSONDiagnosticPath(base + "." + field)
	}
	return semanticDiagnosticPath(semantic)
}

func (projector diagnosticPathProjector) projectIdentityDiagnosticPath(
	changeOrdinal uint64,
	semantic string,
	base string,
	remainder string,
) (projectedDiagnosticPath, error) {
	identityKind, found := projector.coordinates.IdentityKind(changeOrdinal)
	if !found {
		return projectedDiagnosticPath{}, fmt.Errorf(
			"identity diagnostic path %q has no nested identity coordinate",
			semantic,
		)
	}
	field := identityWireField(remainder)
	if identityFieldAllowed(identityKind, field) {
		return requestJSONDiagnosticPath(base + ".change." + field), nil
	}
	if remainder == "identity_change" || remainder == "change" {
		return requestJSONDiagnosticPath(base + ".change"), nil
	}
	return semanticDiagnosticPath(semantic), nil
}

func identityWireField(semantic string) string {
	switch semantic {
	case "bounded_context":
		return "context"
	case "entity":
		return "entity_id"
	case "reconciliation_basis_ref":
		return "basis"
	default:
		return semantic
	}
}

func identityFieldAllowed(
	kind typedmemorywire.DiagnosticIdentityKind,
	field string,
) bool {
	if field == "kind" || field == "context" {
		return true
	}
	switch kind {
	case typedmemorywire.DiagnosticIdentityAdmitAlias:
		return exactField(field, "entity_id", "alias", "provenance")
	case typedmemorywire.DiagnosticIdentitySupersedeAlias:
		return exactField(
			field,
			"entity_id",
			"old_alias",
			"replacement",
			"provenance",
		)
	case typedmemorywire.DiagnosticIdentityMergeEntities:
		return field == "survivor" ||
			field == "merged" ||
			field == "basis"
	case typedmemorywire.DiagnosticIdentitySplitEntity:
		return field == "source" ||
			field == "targets" ||
			field == "basis"
	default:
		return false
	}
}

func (projector diagnosticPathProjector) projectRelationDiagnosticPath(
	changeOrdinal uint64,
	code typedmemory.DiagnosticCode,
	semantic string,
	base string,
	remainder string,
) (projectedDiagnosticPath, error) {
	switch remainder {
	case "kind", "assertion_id", "provenance", "bindings":
		return requestJSONDiagnosticPath(base + "." + remainder), nil
	case "signature", "signature_id":
		return requestJSONDiagnosticPath(base + ".signature_id"), nil
	case "bounded_context", "context", "context_slice.context":
		return requestJSONDiagnosticPath(base + ".context_slice.context"), nil
	case "slice", "context_slice":
		return requestJSONDiagnosticPath(base + ".context_slice"), nil
	case "slots":
		return requestJSONDiagnosticPath(base + ".bindings"), nil
	}
	if strings.HasPrefix(remainder, "context_slice.") {
		return requestJSONDiagnosticPath(base + "." + remainder), nil
	}
	if strings.HasPrefix(remainder, "slice.") {
		suffix := strings.TrimPrefix(remainder, "slice.")
		return requestJSONDiagnosticPath(base + ".context_slice." + suffix), nil
	}
	if !strings.HasPrefix(remainder, "slots.") {
		return semanticDiagnosticPath(semantic), nil
	}
	return projector.projectRelationSlotDiagnosticPath(
		changeOrdinal,
		code,
		semantic,
		base,
		strings.TrimPrefix(remainder, "slots."),
	)
}

func (projector diagnosticPathProjector) projectRelationSlotDiagnosticPath(
	changeOrdinal uint64,
	code typedmemory.DiagnosticCode,
	semantic string,
	base string,
	slotRemainder string,
) (projectedDiagnosticPath, error) {
	slotName, fillerOrdinal, fillerSuffix, hasFiller, parsed := parseSlotDiagnosticPath(
		slotRemainder,
	)
	if !parsed {
		return semanticDiagnosticPath(semantic), nil
	}
	slotKind, err := typedmemory.NewSlotKindID(slotName)
	if err != nil {
		return projectedDiagnosticPath{}, fmt.Errorf(
			"core diagnostic path %q has invalid SlotKind: %w",
			semantic,
			err,
		)
	}
	bindingOrdinal, found := projector.coordinates.BindingOrdinal(changeOrdinal, slotKind)
	if !found {
		// A required but missing semantic slot has no coordinate in the request.
		return semanticDiagnosticPath(semantic), nil
	}
	bindingBase := fmt.Sprintf("%s.bindings[%d]", base, bindingOrdinal)
	if !hasFiller {
		switch code {
		case typedmemory.DiagnosticUnknownSlot:
			return requestJSONDiagnosticPath(bindingBase + ".slot_kind"), nil
		case typedmemory.DiagnosticCardinalityMismatch:
			return requestJSONDiagnosticPath(bindingBase + ".fillers"), nil
		case typedmemory.DiagnosticMissingSlot:
			return semanticDiagnosticPath(semantic), nil
		default:
			return requestJSONDiagnosticPath(bindingBase), nil
		}
	}
	fillerCount, found := projector.coordinates.FillerCount(changeOrdinal, slotKind)
	if !found || fillerOrdinal >= fillerCount {
		return projectedDiagnosticPath{}, fmt.Errorf(
			"core diagnostic path %q addresses absent filler ordinal %d",
			semantic,
			fillerOrdinal,
		)
	}
	fillerBase := fmt.Sprintf("%s.fillers[%d]", bindingBase, fillerOrdinal)
	return projectRelationFillerDiagnosticPath(semantic, fillerBase, fillerSuffix), nil
}

func projectRelationFillerDiagnosticPath(
	semantic string,
	fillerBase string,
	suffix string,
) projectedDiagnosticPath {
	if suffix == "" {
		return requestJSONDiagnosticPath(fillerBase)
	}
	switch suffix {
	case "reference":
		return requestJSONDiagnosticPath(fillerBase + ".reference")
	case "ref_kind":
		return requestJSONDiagnosticPath(fillerBase + ".reference.ref_kind")
	case "reference.ref_kind", "reference.id", "reference.local_ref":
		return requestJSONDiagnosticPath(fillerBase + "." + suffix)
	case "value_kind_ref", "value.value_kind_ref":
		return requestJSONDiagnosticPath(fillerBase + ".value.value_kind")
	case "value", "value.candidate":
		return requestJSONDiagnosticPath(fillerBase + ".value")
	case "value.value_shape_ref":
		return requestJSONDiagnosticPath(fillerBase + ".value.value_shape")
	case "value.codec_ref", "value.codec":
		return requestJSONDiagnosticPath(fillerBase + ".value.codec")
	case "value.canonical_bytes":
		return requestJSONDiagnosticPath(fillerBase + ".value.input_base64")
	case "value.digest":
		return requestJSONDiagnosticPath(fillerBase + ".value.asserted_digest")
	case "kind":
		return requestJSONDiagnosticPath(fillerBase + ".kind")
	default:
		return semanticDiagnosticPath(semantic)
	}
}

func projectRetractionDiagnosticPath(
	semantic string,
	base string,
	remainder string,
) projectedDiagnosticPath {
	if exactField(remainder, "kind", "assertion_id", "reason", "provenance") {
		return requestJSONDiagnosticPath(base + "." + remainder)
	}
	return semanticDiagnosticPath(semantic)
}

func parseChangeDiagnosticPath(raw string) (uint64, string, bool) {
	const prefix = "changes["
	if !strings.HasPrefix(raw, prefix) {
		return 0, "", false
	}
	closeOffset := strings.IndexByte(raw[len(prefix):], ']')
	if closeOffset < 0 {
		return 0, "", false
	}
	closeIndex := len(prefix) + closeOffset
	ordinalRaw := raw[len(prefix):closeIndex]
	ordinal, err := strconv.ParseUint(ordinalRaw, 10, 64)
	if err != nil || strconv.FormatUint(ordinal, 10) != ordinalRaw {
		return 0, "", false
	}
	tail := raw[closeIndex+1:]
	if tail == "" {
		return ordinal, "", true
	}
	if !strings.HasPrefix(tail, ".") || len(tail) == 1 {
		return 0, "", false
	}
	return ordinal, strings.TrimPrefix(tail, "."), true
}

func parseSlotDiagnosticPath(
	raw string,
) (string, uint64, string, bool, bool) {
	const fillerMarker = ".fillers["
	markerIndex := strings.Index(raw, fillerMarker)
	if markerIndex < 0 {
		if raw == "" {
			return "", 0, "", false, false
		}
		return raw, 0, "", false, true
	}
	slotName := raw[:markerIndex]
	fillerStart := markerIndex + len(fillerMarker)
	closeOffset := strings.IndexByte(raw[fillerStart:], ']')
	if slotName == "" || closeOffset < 0 {
		return "", 0, "", false, false
	}
	closeIndex := fillerStart + closeOffset
	ordinalRaw := raw[fillerStart:closeIndex]
	ordinal, err := strconv.ParseUint(ordinalRaw, 10, 64)
	if err != nil || strconv.FormatUint(ordinal, 10) != ordinalRaw {
		return "", 0, "", false, false
	}
	tail := raw[closeIndex+1:]
	if tail == "" {
		return slotName, ordinal, "", true, true
	}
	if !strings.HasPrefix(tail, ".") || len(tail) == 1 {
		return "", 0, "", false, false
	}
	return slotName, ordinal, strings.TrimPrefix(tail, "."), true, true
}

func exactField(candidate string, values ...string) bool {
	for _, value := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func requestJSONDiagnosticPath(value string) projectedDiagnosticPath {
	return projectedDiagnosticPath{
		kind:  DiagnosticPathRequestJSON,
		value: value,
	}
}

func semanticDiagnosticPath(value string) projectedDiagnosticPath {
	return projectedDiagnosticPath{
		kind:  DiagnosticPathTypedMemorySemantic,
		value: typedMemorySemanticPathPrefix + value,
	}
}

func serviceDiagnosticPathKind(value string) DiagnosticPathKind {
	if value == "$.diagnostics" || strings.HasPrefix(value, "$.diagnostics.") {
		return DiagnosticPathResponseJSON
	}
	return DiagnosticPathRequestJSON
}

func normalizedFillerSemanticDiagnostic(
	code typedmemory.DiagnosticCode,
	message string,
	path string,
) bool {
	if code != typedmemory.DiagnosticTypeRuleUnavailable ||
		!strings.Contains(path, ".fillers[") {
		return false
	}
	return strings.HasPrefix(
		message,
		"admitted reference filler has no exact validation evidence",
	) || strings.HasPrefix(
		message,
		"validator could not seal exact reference-filler admission evidence",
	)
}
