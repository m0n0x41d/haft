package projectprofile

import (
	"bytes"
	"cmp"
	"fmt"
	"slices"
)

const (
	observedProjectBasisJSONSchemaV1 = "haft.project-profile.observed-project-basis/v1"
	observedProjectBasisDigestV1     = "haft.project-profile.observed-project-basis/v1"
)

// ObservedProjectBasisRefV1 identifies one project-bound observation relation.
// The identity is deliberately separate from the relation's content digest.
type ObservedProjectBasisRefV1 struct{ v1Reference }

func NewObservedProjectBasisRefV1(raw string) (ObservedProjectBasisRefV1, error) {
	ref, err := newV1Reference("ObservedProjectBasis ref", raw)
	return ObservedProjectBasisRefV1{v1Reference: ref}, err
}

// SourceCarrierRefV1 identifies the carrier from which an observed signal was
// recovered. Carrier identity is not evidence truth and is not an
// EvidenceProvenancePathRefV1.
type SourceCarrierRefV1 struct{ v1Reference }

func NewSourceCarrierRefV1(raw string) (SourceCarrierRefV1, error) {
	ref, err := newV1Reference("source carrier ref", raw)
	return SourceCarrierRefV1{v1Reference: ref}, err
}

// EvidenceProvenancePathRefV1 identifies the claim-bound A.10 because-graph
// relation used for one signal or assessment. It does not identify the
// evidence carrier itself.
type EvidenceProvenancePathRefV1 struct{ v1Reference }

func NewEvidenceProvenancePathRefV1(raw string) (EvidenceProvenancePathRefV1, error) {
	ref, err := newV1Reference("evidence-provenance path ref", raw)
	return EvidenceProvenancePathRefV1{v1Reference: ref}, err
}

type ObservedProjectSignalKindV1 struct{ value string }

func NewObservedProjectSignalKindV1(raw string) (ObservedProjectSignalKindV1, error) {
	value, err := requireText("observed project signal kind", raw)
	return ObservedProjectSignalKindV1{value: value}, err
}

func (kind ObservedProjectSignalKindV1) String() string { return kind.value }

func (kind ObservedProjectSignalKindV1) valid() bool {
	_, err := requireText("observed project signal kind", kind.value)
	return err == nil
}

type ObservedProjectSignalValueV1 struct{ value string }

func NewObservedProjectSignalValueV1(raw string) (ObservedProjectSignalValueV1, error) {
	value, err := requireText("observed project signal value", raw)
	return ObservedProjectSignalValueV1{value: value}, err
}

func (value ObservedProjectSignalValueV1) String() string { return value.value }

func (value ObservedProjectSignalValueV1) valid() bool {
	_, err := requireText("observed project signal value", value.value)
	return err == nil
}

type ObservedProjectDetectorVersionV1 struct{ value string }

func NewObservedProjectDetectorVersionV1(raw string) (ObservedProjectDetectorVersionV1, error) {
	value, err := requireText("observed project detector version", raw)
	return ObservedProjectDetectorVersionV1{value: value}, err
}

func (version ObservedProjectDetectorVersionV1) String() string { return version.value }

func (version ObservedProjectDetectorVersionV1) valid() bool {
	_, err := requireText("observed project detector version", version.value)
	return err == nil
}

// ObservedProjectSignalV1 is a local claim about the project together with the
// distinct carrier and A.10 evidence-provenance paths on which that claim
// relies. The signal is neither the project nor its evidence carrier.
type ObservedProjectSignalV1 struct {
	kind             ObservedProjectSignalKindV1
	value            ObservedProjectSignalValueV1
	sourceCarrierRef SourceCarrierRefV1
	evidencePathRefs []EvidenceProvenancePathRefV1
}

func NewObservedProjectSignalV1(
	kind ObservedProjectSignalKindV1,
	value ObservedProjectSignalValueV1,
	sourceCarrierRef SourceCarrierRefV1,
	evidencePathRefs []EvidenceProvenancePathRefV1,
) (ObservedProjectSignalV1, error) {
	signal := ObservedProjectSignalV1{
		kind:             kind,
		value:            value,
		sourceCarrierRef: sourceCarrierRef,
		evidencePathRefs: append([]EvidenceProvenancePathRefV1{}, evidencePathRefs...),
	}
	return canonicalObservedProjectSignalV1(signal)
}

func (signal ObservedProjectSignalV1) Kind() ObservedProjectSignalKindV1 {
	return signal.kind
}

func (signal ObservedProjectSignalV1) Value() ObservedProjectSignalValueV1 {
	return signal.value
}

func (signal ObservedProjectSignalV1) SourceCarrierRef() SourceCarrierRefV1 {
	return signal.sourceCarrierRef
}

func (signal ObservedProjectSignalV1) EvidencePathRefs() []EvidenceProvenancePathRefV1 {
	return append([]EvidenceProvenancePathRefV1{}, signal.evidencePathRefs...)
}

// ObservedProjectBasisV1 is the immutable, relation-bearing input episteme for
// ProfileOnboardingMethod v1. It records project-bound observations; it does
// not turn a carrier, detector output, or digest into the observed project.
type ObservedProjectBasisV1 interface {
	Ref() ObservedProjectBasisRefV1
	ProjectRoot() ProjectRootV1
	ObservationWindow() BasisObservationWindowV1
	Signals() []ObservedProjectSignalV1
	DetectorVersion() ObservedProjectDetectorVersionV1
	ClassifierVersion() ClassifierVersion
	observedProjectBasisV1()
}

type observedProjectBasisV1 struct {
	ref               ObservedProjectBasisRefV1
	projectRoot       ProjectRootV1
	observationWindow BasisObservationWindowV1
	signals           []ObservedProjectSignalV1
	detectorVersion   ObservedProjectDetectorVersionV1
	classifierVersion ClassifierVersion
}

func (observedProjectBasisV1) observedProjectBasisV1() {}

func NewObservedProjectBasisV1(
	ref ObservedProjectBasisRefV1,
	projectRoot ProjectRootV1,
	observationWindow BasisObservationWindowV1,
	signals []ObservedProjectSignalV1,
	detectorVersion ObservedProjectDetectorVersionV1,
	classifierVersion ClassifierVersion,
) (ObservedProjectBasisV1, error) {
	value := observedProjectBasisV1{
		ref:               ref,
		projectRoot:       projectRoot,
		observationWindow: observationWindow,
		signals:           append([]ObservedProjectSignalV1{}, signals...),
		detectorVersion:   detectorVersion,
		classifierVersion: classifierVersion,
	}
	return canonicalObservedProjectBasisV1(value)
}

func (basis observedProjectBasisV1) Ref() ObservedProjectBasisRefV1 {
	return basis.ref
}

func (basis observedProjectBasisV1) ProjectRoot() ProjectRootV1 {
	return basis.projectRoot
}

func (basis observedProjectBasisV1) ObservationWindow() BasisObservationWindowV1 {
	return basis.observationWindow
}

func (basis observedProjectBasisV1) Signals() []ObservedProjectSignalV1 {
	return append([]ObservedProjectSignalV1{}, basis.signals...)
}

func (basis observedProjectBasisV1) DetectorVersion() ObservedProjectDetectorVersionV1 {
	return basis.detectorVersion
}

func (basis observedProjectBasisV1) ClassifierVersion() ClassifierVersion {
	return basis.classifierVersion
}

func EncodeObservedProjectBasisV1CanonicalJSON(
	value ObservedProjectBasisV1,
) ([]byte, error) {
	exact, err := exactObservedProjectBasisV1(value)
	if err != nil {
		return nil, err
	}
	dto := observedProjectBasisToJSONV1(exact)
	return marshalCanonicalJSONV1(dto)
}

func DecodeObservedProjectBasisV1CanonicalJSON(
	data []byte,
) (ObservedProjectBasisV1, error) {
	var dto observedProjectBasisJSONV1
	err := decodeJSONV1(data, &dto)
	if err != nil {
		return nil, err
	}
	value, err := observedProjectBasisFromJSONV1(dto)
	if err != nil {
		return nil, err
	}
	canonical, err := EncodeObservedProjectBasisV1CanonicalJSON(value)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(data, canonical) {
		return nil, fmt.Errorf("ObservedProjectBasis JSON is not canonical")
	}
	return value, nil
}

func DigestObservedProjectBasisV1(
	value ObservedProjectBasisV1,
) (ContentDigest, error) {
	canonical, err := EncodeObservedProjectBasisV1CanonicalJSON(value)
	if err != nil {
		return ContentDigest{}, err
	}
	return digestObservedProjectBasisCanonicalJSONV1(canonical), nil
}

// ObservedProjectBasisJSONCarrierV1 is an explicit carrier for the basis
// episteme. It is neither the basis relation nor any evidence it references.
type ObservedProjectBasisJSONCarrierV1 interface {
	Schema() string
	MediaType() string
	CanonicalJSON() []byte
	ContentDigest() ContentDigest
	observedProjectBasisJSONCarrierV1()
}

type observedProjectBasisJSONCarrierV1 struct {
	canonicalJSON []byte
	digest        ContentDigest
}

func (observedProjectBasisJSONCarrierV1) observedProjectBasisJSONCarrierV1() {}
func (observedProjectBasisJSONCarrierV1) Schema() string {
	return observedProjectBasisJSONSchemaV1
}
func (observedProjectBasisJSONCarrierV1) MediaType() string { return "application/json" }
func (carrier observedProjectBasisJSONCarrierV1) CanonicalJSON() []byte {
	return append([]byte{}, carrier.canonicalJSON...)
}
func (carrier observedProjectBasisJSONCarrierV1) ContentDigest() ContentDigest {
	return carrier.digest
}

func CarryObservedProjectBasisV1(
	value ObservedProjectBasisV1,
) (ObservedProjectBasisJSONCarrierV1, error) {
	canonical, err := EncodeObservedProjectBasisV1CanonicalJSON(value)
	if err != nil {
		return nil, err
	}
	digest := digestObservedProjectBasisCanonicalJSONV1(canonical)
	return observedProjectBasisJSONCarrierV1{
		canonicalJSON: append([]byte{}, canonical...),
		digest:        digest,
	}, nil
}

// ValidateObservedProjectBasisV1AgainstWorkRecord checks the direct input
// relation available without inventing evidence truth: project, classifier,
// observation window, and input ref must be the values named by the Work.
func ValidateObservedProjectBasisV1AgainstWorkRecord(
	value ObservedProjectBasisV1,
	record ProfileOnboardingWorkRecord,
) error {
	exact, err := exactObservedProjectBasisV1(value)
	if err != nil {
		return err
	}
	canonicalRecord, err := canonicalizeProfileOnboardingWorkRecord(record)
	if err != nil {
		return err
	}
	projectRoot, found := canonicalRecord.parameterBindings.ValueFor(
		profileOnboardingProjectRootParameterV1,
	)
	if !found || projectRoot != exact.projectRoot.String() {
		return fmt.Errorf("ObservedProjectBasis project root does not match Work parameter")
	}
	classifierVersion, found := canonicalRecord.parameterBindings.ValueFor(
		profileOnboardingClassifierParameterV1,
	)
	if !found || classifierVersion != exact.classifierVersion.String() {
		return fmt.Errorf("ObservedProjectBasis classifier version does not match Work parameter")
	}
	if !sameClosedIntervalV1(
		exact.observationWindow.closedIntervalV1,
		canonicalRecord.basisObservationWindow.closedIntervalV1,
	) {
		return fmt.Errorf("ObservedProjectBasis window does not match Work basis-observation window")
	}
	basisRef := exact.ref.String()
	inputRefs := workInputStrings(canonicalRecord.inputRefs)
	if !slices.Contains(inputRefs, basisRef) {
		return fmt.Errorf("work inputs do not reference ObservedProjectBasis")
	}
	return nil
}

type observedProjectSignalJSONV1 struct {
	Kind                       string   `json:"kind"`
	Value                      string   `json:"value"`
	SourceCarrierRef           string   `json:"source_carrier_ref"`
	EvidenceProvenancePathRefs []string `json:"evidence_provenance_path_refs"`
}

type observedProjectBasisJSONV1 struct {
	Schema            string                        `json:"schema"`
	Ref               string                        `json:"ref"`
	ProjectRoot       string                        `json:"project_root"`
	ObservationWindow closedIntervalJSONV1          `json:"observation_window"`
	Signals           []observedProjectSignalJSONV1 `json:"signals"`
	DetectorVersion   string                        `json:"detector_version"`
	ClassifierVersion string                        `json:"classifier_version"`
}

func canonicalObservedProjectSignalV1(
	signal ObservedProjectSignalV1,
) (ObservedProjectSignalV1, error) {
	if !signal.kind.valid() || !signal.value.valid() {
		return ObservedProjectSignalV1{}, fmt.Errorf("observed project signal kind and value are required")
	}
	if !signal.sourceCarrierRef.valid() {
		return ObservedProjectSignalV1{}, fmt.Errorf("observed project signal source carrier ref is required")
	}
	evidenceRefs := append([]EvidenceProvenancePathRefV1{}, signal.evidencePathRefs...)
	err := canonicalizeV1Refs(
		"observed project signal evidence-provenance path refs",
		evidenceRefs,
		func(value EvidenceProvenancePathRefV1) string { return value.String() },
		func(value EvidenceProvenancePathRefV1) bool { return value.valid() },
	)
	if err != nil {
		return ObservedProjectSignalV1{}, err
	}
	signal.evidencePathRefs = evidenceRefs
	return signal, nil
}

func canonicalObservedProjectBasisV1(
	value observedProjectBasisV1,
) (observedProjectBasisV1, error) {
	if !value.ref.valid() || !value.projectRoot.valid() {
		return observedProjectBasisV1{}, fmt.Errorf("ObservedProjectBasis ref and project root are required")
	}
	if !value.observationWindow.valid() {
		return observedProjectBasisV1{}, fmt.Errorf("ObservedProjectBasis observation window is invalid")
	}
	if !value.detectorVersion.valid() || !value.classifierVersion.valid() {
		return observedProjectBasisV1{}, fmt.Errorf("ObservedProjectBasis detector and classifier versions are required")
	}
	if len(value.signals) == 0 {
		return observedProjectBasisV1{}, fmt.Errorf("ObservedProjectBasis signals must not be empty")
	}
	signals, err := mapSliceV1(
		value.signals,
		func(index int, signal ObservedProjectSignalV1) (ObservedProjectSignalV1, error) {
			canonical, signalErr := canonicalObservedProjectSignalV1(signal)
			if signalErr != nil {
				return ObservedProjectSignalV1{}, fmt.Errorf("observed project signal %d: %w", index, signalErr)
			}
			return canonical, nil
		},
	)
	if err != nil {
		return observedProjectBasisV1{}, err
	}
	slices.SortFunc(signals, compareObservedProjectSignalsV1)
	err = visitAdjacentV1(signals, func(previous ObservedProjectSignalV1, current ObservedProjectSignalV1) error {
		if compareObservedProjectSignalsV1(previous, current) == 0 {
			return fmt.Errorf("ObservedProjectBasis contains a duplicate signal")
		}
		return nil
	})
	if err != nil {
		return observedProjectBasisV1{}, err
	}
	value.signals = signals
	return value, nil
}

func exactObservedProjectBasisV1(
	value ObservedProjectBasisV1,
) (observedProjectBasisV1, error) {
	exact, ok := value.(observedProjectBasisV1)
	if !ok {
		return observedProjectBasisV1{}, fmt.Errorf("ObservedProjectBasis must be the package-owned v1 value")
	}
	return canonicalObservedProjectBasisV1(exact)
}

func compareObservedProjectSignalsV1(
	left ObservedProjectSignalV1,
	right ObservedProjectSignalV1,
) int {
	byKind := cmp.Compare(left.kind.String(), right.kind.String())
	if byKind != 0 {
		return byKind
	}
	bySource := cmp.Compare(left.sourceCarrierRef.String(), right.sourceCarrierRef.String())
	if bySource != 0 {
		return bySource
	}
	byValue := cmp.Compare(left.value.String(), right.value.String())
	if byValue != 0 {
		return byValue
	}
	leftEvidence := evidenceProvenancePathRefStringsV1(left.evidencePathRefs)
	rightEvidence := evidenceProvenancePathRefStringsV1(right.evidencePathRefs)
	return slices.Compare(leftEvidence, rightEvidence)
}

func observedProjectBasisToJSONV1(
	value observedProjectBasisV1,
) observedProjectBasisJSONV1 {
	signals := mapSliceV1Pure(value.signals, observedProjectSignalToJSONV1)
	return observedProjectBasisJSONV1{
		Schema:            observedProjectBasisJSONSchemaV1,
		Ref:               value.ref.String(),
		ProjectRoot:       value.projectRoot.String(),
		ObservationWindow: closedIntervalToJSONV1(value.observationWindow.closedIntervalV1),
		Signals:           signals,
		DetectorVersion:   value.detectorVersion.String(),
		ClassifierVersion: value.classifierVersion.String(),
	}
}

func observedProjectSignalToJSONV1(
	value ObservedProjectSignalV1,
) observedProjectSignalJSONV1 {
	return observedProjectSignalJSONV1{
		Kind:                       value.kind.String(),
		Value:                      value.value.String(),
		SourceCarrierRef:           value.sourceCarrierRef.String(),
		EvidenceProvenancePathRefs: evidenceProvenancePathRefStringsV1(value.evidencePathRefs),
	}
}

func observedProjectBasisFromJSONV1(
	dto observedProjectBasisJSONV1,
) (ObservedProjectBasisV1, error) {
	if dto.Schema != observedProjectBasisJSONSchemaV1 {
		return nil, fmt.Errorf("unsupported ObservedProjectBasis JSON schema %q", dto.Schema)
	}
	if dto.Signals == nil {
		return nil, fmt.Errorf("ObservedProjectBasis signals must be an explicit array")
	}
	ref, err := NewObservedProjectBasisRefV1(dto.Ref)
	if err != nil {
		return nil, err
	}
	projectRoot, err := NewProjectRootV1(dto.ProjectRoot)
	if err != nil {
		return nil, err
	}
	window, err := closedIntervalFromJSONV1("ObservedProjectBasis observation window", dto.ObservationWindow)
	if err != nil {
		return nil, err
	}
	signals, err := mapSliceV1(dto.Signals, func(index int, signalDTO observedProjectSignalJSONV1) (ObservedProjectSignalV1, error) {
		signal, signalErr := observedProjectSignalFromJSONV1(signalDTO)
		if signalErr != nil {
			return ObservedProjectSignalV1{}, fmt.Errorf("observed project signal %d: %w", index, signalErr)
		}
		return signal, nil
	})
	if err != nil {
		return nil, err
	}
	detectorVersion, err := NewObservedProjectDetectorVersionV1(dto.DetectorVersion)
	if err != nil {
		return nil, err
	}
	classifierVersion, err := NewClassifierVersion(dto.ClassifierVersion)
	if err != nil {
		return nil, err
	}
	observationWindow := BasisObservationWindowV1{closedIntervalV1: window}
	return NewObservedProjectBasisV1(
		ref,
		projectRoot,
		observationWindow,
		signals,
		detectorVersion,
		classifierVersion,
	)
}

func observedProjectSignalFromJSONV1(
	dto observedProjectSignalJSONV1,
) (ObservedProjectSignalV1, error) {
	if dto.EvidenceProvenancePathRefs == nil {
		return ObservedProjectSignalV1{}, fmt.Errorf("signal evidence-provenance path refs must be an explicit array")
	}
	kind, err := NewObservedProjectSignalKindV1(dto.Kind)
	if err != nil {
		return ObservedProjectSignalV1{}, err
	}
	value, err := NewObservedProjectSignalValueV1(dto.Value)
	if err != nil {
		return ObservedProjectSignalV1{}, err
	}
	sourceCarrierRef, err := NewSourceCarrierRefV1(dto.SourceCarrierRef)
	if err != nil {
		return ObservedProjectSignalV1{}, err
	}
	evidenceRefs, err := refsFromStringsV1(
		dto.EvidenceProvenancePathRefs,
		NewEvidenceProvenancePathRefV1,
	)
	if err != nil {
		return ObservedProjectSignalV1{}, err
	}
	return NewObservedProjectSignalV1(kind, value, sourceCarrierRef, evidenceRefs)
}

func digestObservedProjectBasisCanonicalJSONV1(canonical []byte) ContentDigest {
	writer := newCanonicalDigestWriter(observedProjectBasisDigestV1)
	writer.add(string(canonical))
	return writer.digest()
}

func evidenceProvenancePathRefStringsV1(
	values []EvidenceProvenancePathRefV1,
) []string {
	return mapSliceV1Pure(values, func(value EvidenceProvenancePathRefV1) string {
		return value.String()
	})
}

func sameClosedIntervalV1(left closedIntervalV1, right closedIntervalV1) bool {
	return left.from.Equal(right.from) && left.until.Equal(right.until)
}
