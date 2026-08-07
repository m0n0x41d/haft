package typedmemory

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"time"
)

const contextSliceCanonicalDomain = "haft.typedmemory.context-slice.v1"

// VersionedPin is the common exact pin carried by Standard, vocabulary, and
// role-set selections. It has no public constructor: callers must state which
// semantic position the pin fills through one of the strong wrappers below.
type VersionedPin struct {
	reference CarrierRef
	edition   CarrierEdition
	digest    SHA256Digest
}

func newVersionedPin(
	reference CarrierRef,
	edition CarrierEdition,
	digest SHA256Digest,
) (VersionedPin, error) {
	if !reference.valid() {
		return VersionedPin{}, fmt.Errorf("versioned pin reference is required")
	}
	if !edition.valid() {
		return VersionedPin{}, fmt.Errorf("versioned pin edition is required")
	}
	if implicitContextSelector(edition.String()) {
		return VersionedPin{}, fmt.Errorf("versioned pin edition must be exact, not %q", edition.String())
	}
	if !digest.valid() {
		return VersionedPin{}, fmt.Errorf("versioned pin content digest is required")
	}
	return VersionedPin{reference: reference, edition: edition, digest: digest}, nil
}

func (pin VersionedPin) Reference() CarrierRef { return pin.reference }

func (pin VersionedPin) Edition() CarrierEdition { return pin.edition }

func (pin VersionedPin) Digest() SHA256Digest { return pin.digest }

func (pin VersionedPin) valid() bool {
	return pin.reference.valid() &&
		pin.edition.valid() &&
		!implicitContextSelector(pin.edition.String()) &&
		pin.digest.valid()
}

func (pin VersionedPin) canonicalBytes(domain string) []byte {
	writer := newCanonicalWriter(domain)
	writer.addString(pin.reference.String())
	writer.addString(pin.edition.String())
	writer.addString(pin.digest.String())
	return writer.bytes()
}

type StandardPin struct {
	versioned VersionedPin
}

func NewStandardPin(
	reference CarrierRef,
	edition CarrierEdition,
	digest SHA256Digest,
) (StandardPin, error) {
	pin, err := newVersionedPin(reference, edition, digest)
	if err != nil {
		return StandardPin{}, fmt.Errorf("%s pin: %w", "Standard", err)
	}
	return StandardPin{versioned: pin}, nil
}

func (pin StandardPin) VersionedPin() VersionedPin { return pin.versioned }

func (pin StandardPin) contextPinReference() CarrierRef { return pin.versioned.reference }

func (pin StandardPin) contextPinBytes() []byte {
	return pin.versioned.canonicalBytes("context-slice-standard-pin.v1")
}

func (pin StandardPin) validContextPin() bool { return pin.versioned.valid() }

type VocabularyPin struct {
	versioned VersionedPin
}

func NewVocabularyPin(
	reference CarrierRef,
	edition CarrierEdition,
	digest SHA256Digest,
) (VocabularyPin, error) {
	pin, err := newVersionedPin(reference, edition, digest)
	if err != nil {
		return VocabularyPin{}, fmt.Errorf("vocabulary pin: %w", err)
	}
	return VocabularyPin{versioned: pin}, nil
}

func (pin VocabularyPin) VersionedPin() VersionedPin { return pin.versioned }

func (pin VocabularyPin) contextPinReference() CarrierRef { return pin.versioned.reference }

func (pin VocabularyPin) contextPinBytes() []byte {
	return pin.versioned.canonicalBytes("context-slice-vocabulary-pin.v1")
}

func (pin VocabularyPin) validContextPin() bool { return pin.versioned.valid() }

type RoleSetPin struct {
	versioned VersionedPin
}

func NewRoleSetPin(
	reference CarrierRef,
	edition CarrierEdition,
	digest SHA256Digest,
) (RoleSetPin, error) {
	pin, err := newVersionedPin(reference, edition, digest)
	if err != nil {
		return RoleSetPin{}, fmt.Errorf("role-set pin: %w", err)
	}
	return RoleSetPin{versioned: pin}, nil
}

func (pin RoleSetPin) VersionedPin() VersionedPin { return pin.versioned }

func (pin RoleSetPin) contextPinReference() CarrierRef { return pin.versioned.reference }

func (pin RoleSetPin) contextPinBytes() []byte {
	return pin.versioned.canonicalBytes("context-slice-role-set-pin.v1")
}

func (pin RoleSetPin) validContextPin() bool { return pin.versioned.valid() }

type contextVersionedPin interface {
	contextPinReference() CarrierRef
	contextPinBytes() []byte
	validContextPin() bool
}

type EnvironmentSelectorKey struct {
	value string
}

func NewEnvironmentSelectorKey(raw string) (EnvironmentSelectorKey, error) {
	value, err := parseQualifiedIdentifier("environment selector key", raw)
	if err != nil {
		return EnvironmentSelectorKey{}, err
	}
	return EnvironmentSelectorKey{value: value}, nil
}

func (key EnvironmentSelectorKey) String() string { return key.value }

func (key EnvironmentSelectorKey) valid() bool { return key.value != "" }

type EnvironmentSelectorValue struct {
	value string
}

func NewEnvironmentSelectorValue(raw string) (EnvironmentSelectorValue, error) {
	value, err := parseOpaqueIdentifier("environment selector value", raw)
	if err != nil {
		return EnvironmentSelectorValue{}, err
	}
	if implicitContextSelector(value) {
		return EnvironmentSelectorValue{}, fmt.Errorf("environment selector value must be exact, not %q", value)
	}
	return EnvironmentSelectorValue{value: value}, nil
}

func (value EnvironmentSelectorValue) String() string { return value.value }

func (value EnvironmentSelectorValue) valid() bool {
	return value.value != "" && !implicitContextSelector(value.value)
}

// EnvironmentSelector retains the exact source digest of the selector
// expression. Its value may be a concrete scalar, set, or range expression;
// Haft does not silently reinterpret that expression while addressing a slice.
type EnvironmentSelector struct {
	key          EnvironmentSelectorKey
	value        EnvironmentSelectorValue
	sourceDigest SHA256Digest
}

func NewEnvironmentSelector(
	key EnvironmentSelectorKey,
	value EnvironmentSelectorValue,
	sourceDigest SHA256Digest,
) (EnvironmentSelector, error) {
	if !key.valid() {
		return EnvironmentSelector{}, fmt.Errorf("environment selector key is required")
	}
	if !value.valid() {
		return EnvironmentSelector{}, fmt.Errorf("environment selector value is required and must be exact")
	}
	if !sourceDigest.valid() {
		return EnvironmentSelector{}, fmt.Errorf("environment selector source digest is required")
	}
	return EnvironmentSelector{key: key, value: value, sourceDigest: sourceDigest}, nil
}

func (selector EnvironmentSelector) Key() EnvironmentSelectorKey { return selector.key }

func (selector EnvironmentSelector) Value() EnvironmentSelectorValue { return selector.value }

func (selector EnvironmentSelector) SourceDigest() SHA256Digest { return selector.sourceDigest }

func (selector EnvironmentSelector) valid() bool {
	return selector.key.valid() && selector.value.valid() && selector.sourceDigest.valid()
}

func (selector EnvironmentSelector) canonicalBytes() []byte {
	writer := newCanonicalWriter("context-slice-environment-selector.v1")
	writer.addString(selector.key.String())
	writer.addString(selector.value.String())
	writer.addString(selector.sourceDigest.String())
	return writer.bytes()
}

// GammaTimeSelector is a closed temporal selector. The unexported variant
// method prevents callers from adding an implicit now/latest representation.
type GammaTimeSelector interface {
	CanonicalBytes() []byte
	gammaTimeSelectorVariant()
}

// ResolvedGammaTimeSelector excludes policy applications. A policy must
// resolve to a concrete point or window and cannot recursively defer to policy.
type ResolvedGammaTimeSelector interface {
	GammaTimeSelector
	resolvedGammaTimeSelectorVariant()
}

type GammaPoint struct {
	at time.Time
}

func NewGammaPoint(at time.Time) (GammaPoint, error) {
	instant, err := normalizeGammaInstant(at)
	if err != nil {
		return GammaPoint{}, err
	}
	return GammaPoint{at: instant}, nil
}

func (point GammaPoint) At() time.Time { return point.at }

func (point GammaPoint) CanonicalBytes() []byte {
	writer := newCanonicalWriter("context-slice-gamma-point.v1")
	writer.addString(point.at.Format(time.RFC3339Nano))
	return writer.bytes()
}

func (GammaPoint) gammaTimeSelectorVariant() {}

func (GammaPoint) resolvedGammaTimeSelectorVariant() {}

func (point GammaPoint) valid() bool { return validGammaInstant(point.at) }

type GammaBoundary uint8

const (
	GammaBoundaryInclusive GammaBoundary = iota + 1
	GammaBoundaryExclusive
)

func (boundary GammaBoundary) String() string {
	switch boundary {
	case GammaBoundaryInclusive:
		return "inclusive"
	case GammaBoundaryExclusive:
		return "exclusive"
	default:
		return ""
	}
}

func (boundary GammaBoundary) valid() bool { return boundary.String() != "" }

type GammaWindow struct {
	start         time.Time
	end           time.Time
	startBoundary GammaBoundary
	endBoundary   GammaBoundary
}

func NewGammaWindow(
	start time.Time,
	end time.Time,
	startBoundary GammaBoundary,
	endBoundary GammaBoundary,
) (GammaWindow, error) {
	startInstant, err := normalizeGammaInstant(start)
	if err != nil {
		return GammaWindow{}, fmt.Errorf("%s window start: %w", "Gamma", err)
	}
	endInstant, err := normalizeGammaInstant(end)
	if err != nil {
		return GammaWindow{}, fmt.Errorf("%s window end: %w", "Gamma", err)
	}
	if !startInstant.Before(endInstant) {
		return GammaWindow{}, fmt.Errorf(
			"%s window start must precede end; use GammaPoint for one instant",
			"Gamma",
		)
	}
	if !startBoundary.valid() || !endBoundary.valid() {
		return GammaWindow{}, fmt.Errorf(
			"%s window requires explicit start and end boundary semantics",
			"Gamma",
		)
	}
	return GammaWindow{
		start:         startInstant,
		end:           endInstant,
		startBoundary: startBoundary,
		endBoundary:   endBoundary,
	}, nil
}

func (window GammaWindow) Start() time.Time { return window.start }

func (window GammaWindow) End() time.Time { return window.end }

func (window GammaWindow) StartBoundary() GammaBoundary { return window.startBoundary }

func (window GammaWindow) EndBoundary() GammaBoundary { return window.endBoundary }

func (window GammaWindow) CanonicalBytes() []byte {
	writer := newCanonicalWriter("context-slice-gamma-window.v1")
	writer.addString(window.start.Format(time.RFC3339Nano))
	writer.addString(window.end.Format(time.RFC3339Nano))
	writer.addString(window.startBoundary.String())
	writer.addString(window.endBoundary.String())
	return writer.bytes()
}

func (GammaWindow) gammaTimeSelectorVariant() {}

func (GammaWindow) resolvedGammaTimeSelectorVariant() {}

func (window GammaWindow) valid() bool {
	return validGammaInstant(window.start) &&
		validGammaInstant(window.end) &&
		window.start.Before(window.end) &&
		window.startBoundary.valid() &&
		window.endBoundary.valid()
}

type GammaPolicyApplication struct {
	policyRef        CarrierRef
	policyEdition    CarrierEdition
	policyDigest     SHA256Digest
	evaluationAnchor GammaPoint
	resolved         ResolvedGammaTimeSelector
}

func NewGammaPolicyApplication(
	policyRef CarrierRef,
	policyEdition CarrierEdition,
	policyDigest SHA256Digest,
	evaluationAnchor GammaPoint,
	resolved ResolvedGammaTimeSelector,
) (GammaPolicyApplication, error) {
	if !policyRef.valid() {
		return GammaPolicyApplication{}, fmt.Errorf("%s policy reference is required", "Gamma")
	}
	if !policyEdition.valid() {
		return GammaPolicyApplication{}, fmt.Errorf("%s policy edition is required", "Gamma")
	}
	if implicitContextSelector(policyEdition.String()) {
		return GammaPolicyApplication{}, fmt.Errorf(
			"%s policy edition must be exact, not %q",
			"Gamma",
			policyEdition.String(),
		)
	}
	if !policyDigest.valid() {
		return GammaPolicyApplication{}, fmt.Errorf("%s policy digest is required", "Gamma")
	}
	if !evaluationAnchor.valid() {
		return GammaPolicyApplication{}, fmt.Errorf(
			"%s policy evaluation anchor is required",
			"Gamma",
		)
	}
	if !validResolvedGammaTimeSelector(resolved) {
		return GammaPolicyApplication{}, fmt.Errorf(
			"%s policy must resolve to an exact GammaPoint or GammaWindow",
			"Gamma",
		)
	}
	return GammaPolicyApplication{
		policyRef:        policyRef,
		policyEdition:    policyEdition,
		policyDigest:     policyDigest,
		evaluationAnchor: evaluationAnchor,
		resolved:         resolved,
	}, nil
}

func (application GammaPolicyApplication) PolicyRef() CarrierRef { return application.policyRef }

func (application GammaPolicyApplication) PolicyEdition() CarrierEdition {
	return application.policyEdition
}

func (application GammaPolicyApplication) PolicyDigest() SHA256Digest {
	return application.policyDigest
}

func (application GammaPolicyApplication) EvaluationAnchor() GammaPoint {
	return application.evaluationAnchor
}

func (application GammaPolicyApplication) Resolved() ResolvedGammaTimeSelector {
	return application.resolved
}

func (application GammaPolicyApplication) CanonicalBytes() []byte {
	writer := newCanonicalWriter("context-slice-gamma-policy-application.v1")
	writer.addString(application.policyRef.String())
	writer.addString(application.policyEdition.String())
	writer.addString(application.policyDigest.String())
	writer.addBytes(application.evaluationAnchor.CanonicalBytes())
	writer.addBytes(application.resolved.CanonicalBytes())
	return writer.bytes()
}

func (GammaPolicyApplication) gammaTimeSelectorVariant() {}

func (application GammaPolicyApplication) valid() bool {
	return application.policyRef.valid() &&
		application.policyEdition.valid() &&
		!implicitContextSelector(application.policyEdition.String()) &&
		application.policyDigest.valid() &&
		application.evaluationAnchor.valid() &&
		validResolvedGammaTimeSelector(application.resolved)
}

type ContextSliceInput struct {
	Context              BoundedContextRef
	StandardPins         []StandardPin
	EnvironmentSelectors []EnvironmentSelector
	VocabularyPins       []VocabularyPin
	RoleSetPins          []RoleSetPin
	GammaTime            GammaTimeSelector
}

type ContextSliceRef struct {
	digest SHA256Digest
}

func NewContextSliceRef(digest SHA256Digest) (ContextSliceRef, error) {
	if !digest.valid() {
		return ContextSliceRef{}, fmt.Errorf("ContextSlice digest is required")
	}
	return ContextSliceRef{digest: digest}, nil
}

func (ref ContextSliceRef) Digest() SHA256Digest { return ref.digest }

func (ref ContextSliceRef) String() string { return "context-slice:" + ref.digest.String() }

func (ref ContextSliceRef) valid() bool { return ref.digest.valid() }

// ContextSlice is a content-addressed, context-local selection. Slice order is
// never project or causal order; all set-like inputs are canonicalized here.
type ContextSlice struct {
	context              BoundedContextRef
	standardPins         []StandardPin
	environmentSelectors []EnvironmentSelector
	vocabularyPins       []VocabularyPin
	roleSetPins          []RoleSetPin
	gammaTime            GammaTimeSelector
	canonicalBytes       []byte
	reference            ContextSliceRef
}

func NewContextSlice(input ContextSliceInput) (ContextSlice, error) {
	if !input.Context.valid() {
		return ContextSlice{}, fmt.Errorf("ContextSlice bounded context is required")
	}
	if !validGammaTimeSelector(input.GammaTime) {
		return ContextSlice{}, fmt.Errorf("ContextSlice requires an explicit Gamma point, window, or resolved policy application")
	}
	standards, err := normalizeContextPins("Standard", input.StandardPins)
	if err != nil {
		return ContextSlice{}, err
	}
	environment, err := normalizeEnvironmentSelectors(input.EnvironmentSelectors)
	if err != nil {
		return ContextSlice{}, err
	}
	vocabularies, err := normalizeContextPins("vocabulary", input.VocabularyPins)
	if err != nil {
		return ContextSlice{}, err
	}
	roleSets, err := normalizeContextPins("role-set", input.RoleSetPins)
	if err != nil {
		return ContextSlice{}, err
	}
	writer := canonicalContextSlice(
		input.Context,
		standards,
		environment,
		vocabularies,
		roleSets,
		input.GammaTime,
	)
	reference, err := NewContextSliceRef(writer.digest())
	if err != nil {
		return ContextSlice{}, err
	}
	return ContextSlice{
		context:              input.Context,
		standardPins:         standards,
		environmentSelectors: environment,
		vocabularyPins:       vocabularies,
		roleSetPins:          roleSets,
		gammaTime:            input.GammaTime,
		canonicalBytes:       writer.bytes(),
		reference:            reference,
	}, nil
}

func (slice ContextSlice) Context() BoundedContextRef { return slice.context }

func (slice ContextSlice) StandardPins() []StandardPin {
	return append([]StandardPin(nil), slice.standardPins...)
}

func (slice ContextSlice) EnvironmentSelectors() []EnvironmentSelector {
	return append([]EnvironmentSelector(nil), slice.environmentSelectors...)
}

func (slice ContextSlice) VocabularyPins() []VocabularyPin {
	return append([]VocabularyPin(nil), slice.vocabularyPins...)
}

func (slice ContextSlice) RoleSetPins() []RoleSetPin {
	return append([]RoleSetPin(nil), slice.roleSetPins...)
}

func (slice ContextSlice) GammaTime() GammaTimeSelector { return slice.gammaTime }

func (slice ContextSlice) CanonicalBytes() []byte {
	return append([]byte(nil), slice.canonicalBytes...)
}

func (slice ContextSlice) Digest() SHA256Digest { return slice.reference.digest }

func (slice ContextSlice) Ref() ContextSliceRef { return slice.reference }

func (slice ContextSlice) valid() bool {
	if !slice.context.valid() ||
		!validGammaTimeSelector(slice.gammaTime) ||
		!slice.reference.valid() ||
		len(slice.canonicalBytes) == 0 {
		return false
	}
	standards, err := normalizeContextPins("Standard", slice.standardPins)
	if err != nil || !exactContextPins(standards, slice.standardPins) {
		return false
	}
	environment, err := normalizeEnvironmentSelectors(slice.environmentSelectors)
	if err != nil || !exactEnvironmentSelectors(environment, slice.environmentSelectors) {
		return false
	}
	vocabularies, err := normalizeContextPins("vocabulary", slice.vocabularyPins)
	if err != nil || !exactContextPins(vocabularies, slice.vocabularyPins) {
		return false
	}
	roleSets, err := normalizeContextPins("role-set", slice.roleSetPins)
	if err != nil || !exactContextPins(roleSets, slice.roleSetPins) {
		return false
	}
	writer := canonicalContextSlice(
		slice.context,
		standards,
		environment,
		vocabularies,
		roleSets,
		slice.gammaTime,
	)
	return bytes.Equal(writer.bytes(), slice.canonicalBytes) &&
		writer.digest() == slice.reference.digest
}

func canonicalContextSlice(
	context BoundedContextRef,
	standards []StandardPin,
	environment []EnvironmentSelector,
	vocabularies []VocabularyPin,
	roleSets []RoleSetPin,
	gamma GammaTimeSelector,
) canonicalWriter {
	writer := newCanonicalWriter(contextSliceCanonicalDomain)
	writer.addString(context.String())
	writer.addUint64(uint64(len(standards)))
	for _, pin := range standards {
		writer.addBytes(pin.contextPinBytes())
	}
	writer.addUint64(uint64(len(environment)))
	for _, selector := range environment {
		writer.addBytes(selector.canonicalBytes())
	}
	writer.addUint64(uint64(len(vocabularies)))
	for _, pin := range vocabularies {
		writer.addBytes(pin.contextPinBytes())
	}
	writer.addUint64(uint64(len(roleSets)))
	for _, pin := range roleSets {
		writer.addBytes(pin.contextPinBytes())
	}
	writer.addBytes(gamma.CanonicalBytes())
	return writer
}

func normalizeContextPins[T contextVersionedPin](label string, values []T) ([]T, error) {
	owned := append([]T(nil), values...)
	sort.Slice(owned, func(left, right int) bool {
		leftPin := owned[left]
		rightPin := owned[right]
		leftRef := leftPin.contextPinReference().String()
		rightRef := rightPin.contextPinReference().String()
		if leftRef != rightRef {
			return leftRef < rightRef
		}
		leftBytes := leftPin.contextPinBytes()
		rightBytes := rightPin.contextPinBytes()
		return bytes.Compare(leftBytes, rightBytes) < 0
	})
	result := make([]T, 0, len(owned))
	seen := make(map[string][]byte, len(owned))
	for index, pin := range owned {
		if !pin.validContextPin() {
			return nil, fmt.Errorf("ContextSlice %s pin at index %d is invalid", label, index)
		}
		reference := pin.contextPinReference().String()
		encoded := pin.contextPinBytes()
		prior, exists := seen[reference]
		if !exists {
			seen[reference] = encoded
			result = append(result, pin)
			continue
		}
		if bytes.Equal(prior, encoded) {
			continue
		}
		return nil, fmt.Errorf("ContextSlice %s pin %q has conflicting editions or digests", label, reference)
	}
	return result, nil
}

func normalizeEnvironmentSelectors(values []EnvironmentSelector) ([]EnvironmentSelector, error) {
	owned := append([]EnvironmentSelector(nil), values...)
	sort.Slice(owned, func(left, right int) bool {
		leftSelector := owned[left]
		rightSelector := owned[right]
		leftKey := leftSelector.key.String()
		rightKey := rightSelector.key.String()
		if leftKey != rightKey {
			return leftKey < rightKey
		}
		leftBytes := leftSelector.canonicalBytes()
		rightBytes := rightSelector.canonicalBytes()
		return bytes.Compare(leftBytes, rightBytes) < 0
	})
	result := make([]EnvironmentSelector, 0, len(owned))
	seen := make(map[string][]byte, len(owned))
	for index, selector := range owned {
		if !selector.valid() {
			return nil, fmt.Errorf("ContextSlice environment selector at index %d is invalid", index)
		}
		key := selector.key.String()
		encoded := selector.canonicalBytes()
		prior, exists := seen[key]
		if !exists {
			seen[key] = encoded
			result = append(result, selector)
			continue
		}
		if bytes.Equal(prior, encoded) {
			continue
		}
		return nil, fmt.Errorf("ContextSlice environment selector %q has conflicting exact values", key)
	}
	return result, nil
}

func exactContextPins[T contextVersionedPin](left []T, right []T) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if !bytes.Equal(left[index].contextPinBytes(), right[index].contextPinBytes()) {
			return false
		}
	}
	return true
}

func exactEnvironmentSelectors(
	left []EnvironmentSelector,
	right []EnvironmentSelector,
) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if !bytes.Equal(left[index].canonicalBytes(), right[index].canonicalBytes()) {
			return false
		}
	}
	return true
}

func validGammaTimeSelector(selector GammaTimeSelector) bool {
	switch value := selector.(type) {
	case GammaPoint:
		return value.valid()
	case GammaWindow:
		return value.valid()
	case GammaPolicyApplication:
		return value.valid()
	default:
		return false
	}
}

func validResolvedGammaTimeSelector(selector ResolvedGammaTimeSelector) bool {
	switch value := selector.(type) {
	case GammaPoint:
		return value.valid()
	case GammaWindow:
		return value.valid()
	default:
		return false
	}
}

func normalizeGammaInstant(value time.Time) (time.Time, error) {
	if value.IsZero() {
		return time.Time{}, fmt.Errorf("%s time instant is required", "Gamma")
	}
	normalized := value.Round(0)
	normalized = normalized.UTC()
	if !validGammaInstant(normalized) {
		return time.Time{}, fmt.Errorf(
			"%s time instant must be representable as an exact RFC3339 UTC timestamp",
			"Gamma",
		)
	}
	return normalized, nil
}

func validGammaInstant(value time.Time) bool {
	if value.IsZero() {
		return false
	}
	if value.Location() != time.UTC {
		return false
	}
	year := value.Year()
	return year >= 0 && year <= 9999
}

func implicitContextSelector(raw string) bool {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "now", "latest", "current", "implicit", "head":
		return true
	default:
		return false
	}
}
