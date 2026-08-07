package projectmemoryreferencescheme

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	projectMemoryReferenceSchemeV1Domain      = "haft.project-memory.reference-scheme.canonical.v1"
	maximumCanonicalSchemeBytes               = 4 << 20
	maximumCanonicalTextBytes                 = 64 << 10
	maximumRulePins                           = 4 << 10
	rulesBranchTag                       byte = 1
	notApplicableBranchTag               byte = 2
	runtimeNotRequiredTag                byte = 1
	runtimeRequiredTag                   byte = 2
)

// ReferenceSchemeDigest is the intrinsic identity of one exact canonical
// ProjectMemoryReferenceSchemeV1 value.
type ReferenceSchemeDigest struct {
	value typedmemory.SHA256Digest
}

func ParseReferenceSchemeDigest(raw string) (ReferenceSchemeDigest, error) {
	digest, err := typedmemory.NewSHA256Digest(raw)
	if err != nil {
		return ReferenceSchemeDigest{}, fmt.Errorf("reference-scheme digest: %w", err)
	}
	return ReferenceSchemeDigest{value: digest}, nil
}

func (digest ReferenceSchemeDigest) String() string { return digest.value.String() }

func (digest ReferenceSchemeDigest) valid() bool {
	rebuilt, err := ParseReferenceSchemeDigest(digest.String())
	return err == nil && rebuilt == digest
}

// ProjectMemoryReferenceSchemeV1 is a sealed by-value scheme. Its digest is
// computed only from the four semantic rule groups below.
type ProjectMemoryReferenceSchemeV1 struct {
	designation    DesignationRules
	interpretation InterpretationRules
	measurement    MeasurementBranch
	evaluation     EvaluationBranch
	digest         ReferenceSchemeDigest
	canonical      []byte
}

func NewProjectMemoryReferenceSchemeV1(
	designation DesignationRules,
	interpretation InterpretationRules,
	measurement MeasurementBranch,
	evaluation EvaluationBranch,
) (ProjectMemoryReferenceSchemeV1, error) {
	if !designation.valid() {
		return ProjectMemoryReferenceSchemeV1{}, fmt.Errorf("designation rules are invalid")
	}
	if !interpretation.valid() {
		return ProjectMemoryReferenceSchemeV1{}, fmt.Errorf("interpretation rules are invalid")
	}
	if err := validateMeasurementBranch(measurement); err != nil {
		return ProjectMemoryReferenceSchemeV1{}, err
	}
	if err := validateEvaluationBranch(evaluation); err != nil {
		return ProjectMemoryReferenceSchemeV1{}, err
	}
	canonical, err := encodeSchemeCanonical(
		designation,
		interpretation,
		measurement,
		evaluation,
	)
	if err != nil {
		return ProjectMemoryReferenceSchemeV1{}, err
	}
	scheme, err := DecodeProjectMemoryReferenceSchemeV1(canonical)
	if err != nil {
		return ProjectMemoryReferenceSchemeV1{}, fmt.Errorf(
			"reseal project-memory reference scheme: %w",
			err,
		)
	}
	return scheme, nil
}

func (scheme ProjectMemoryReferenceSchemeV1) Designation() DesignationRules {
	return DesignationRules{pins: scheme.designation.Pins()}
}

func (scheme ProjectMemoryReferenceSchemeV1) Interpretation() InterpretationRules {
	return InterpretationRules{pins: scheme.interpretation.Pins()}
}

func (scheme ProjectMemoryReferenceSchemeV1) Measurement() MeasurementBranch {
	return cloneMeasurementBranch(scheme.measurement)
}

func (scheme ProjectMemoryReferenceSchemeV1) Evaluation() EvaluationBranch {
	return cloneEvaluationBranch(scheme.evaluation)
}

func (scheme ProjectMemoryReferenceSchemeV1) Digest() ReferenceSchemeDigest {
	return scheme.digest
}

func (scheme ProjectMemoryReferenceSchemeV1) CanonicalBytes() []byte {
	return append([]byte(nil), scheme.canonical...)
}

func (scheme ProjectMemoryReferenceSchemeV1) Verify() error {
	if !scheme.digest.valid() {
		return fmt.Errorf("reference-scheme digest is invalid")
	}
	canonical, err := encodeSchemeCanonical(
		scheme.designation,
		scheme.interpretation,
		scheme.measurement,
		scheme.evaluation,
	)
	if err != nil {
		return fmt.Errorf("verify reference-scheme fields: %w", err)
	}
	if !bytes.Equal(canonical, scheme.canonical) {
		return fmt.Errorf("reference-scheme fields do not match canonical bytes")
	}
	decoded, err := VerifyProjectMemoryReferenceSchemeV1(
		scheme.digest,
		scheme.canonical,
	)
	if err != nil {
		return err
	}
	if !bytes.Equal(decoded.canonical, scheme.canonical) {
		return fmt.Errorf("reference-scheme canonical bytes do not match")
	}
	return nil
}

// DecodeProjectMemoryReferenceSchemeV1 accepts exact canonical bytes only.
func DecodeProjectMemoryReferenceSchemeV1(
	canonical []byte,
) (ProjectMemoryReferenceSchemeV1, error) {
	reader, err := newSchemeReader(canonical)
	if err != nil {
		return ProjectMemoryReferenceSchemeV1{}, err
	}
	designationPins, err := reader.readRulePins(
		"designation rules",
		SemanticRoleDesignation,
	)
	if err != nil {
		return ProjectMemoryReferenceSchemeV1{}, err
	}
	interpretationPins, err := reader.readRulePins(
		"interpretation rules",
		SemanticRoleInterpretation,
	)
	if err != nil {
		return ProjectMemoryReferenceSchemeV1{}, err
	}
	measurement, err := reader.readMeasurementBranch()
	if err != nil {
		return ProjectMemoryReferenceSchemeV1{}, err
	}
	evaluation, err := reader.readEvaluationBranch()
	if err != nil {
		return ProjectMemoryReferenceSchemeV1{}, err
	}
	if err := reader.requireEnd(); err != nil {
		return ProjectMemoryReferenceSchemeV1{}, err
	}
	designation, err := NewDesignationRules(designationPins)
	if err != nil {
		return ProjectMemoryReferenceSchemeV1{}, err
	}
	interpretation, err := NewInterpretationRules(interpretationPins)
	if err != nil {
		return ProjectMemoryReferenceSchemeV1{}, err
	}
	reencoded, err := encodeSchemeCanonical(
		designation,
		interpretation,
		measurement,
		evaluation,
	)
	if err != nil {
		return ProjectMemoryReferenceSchemeV1{}, err
	}
	if !bytes.Equal(reencoded, canonical) {
		return ProjectMemoryReferenceSchemeV1{}, fmt.Errorf(
			"project-memory reference-scheme bytes are not canonical",
		)
	}
	digest, err := digestSchemeCanonical(reencoded)
	if err != nil {
		return ProjectMemoryReferenceSchemeV1{}, err
	}
	return ProjectMemoryReferenceSchemeV1{
		designation:    designation,
		interpretation: interpretation,
		measurement:    cloneMeasurementBranch(measurement),
		evaluation:     cloneEvaluationBranch(evaluation),
		digest:         digest,
		canonical:      append([]byte(nil), reencoded...),
	}, nil
}

func VerifyProjectMemoryReferenceSchemeV1(
	expected ReferenceSchemeDigest,
	canonical []byte,
) (ProjectMemoryReferenceSchemeV1, error) {
	if !expected.valid() {
		return ProjectMemoryReferenceSchemeV1{}, fmt.Errorf(
			"expected reference-scheme digest is invalid",
		)
	}
	scheme, err := DecodeProjectMemoryReferenceSchemeV1(canonical)
	if err != nil {
		return ProjectMemoryReferenceSchemeV1{}, err
	}
	if scheme.digest != expected {
		return ProjectMemoryReferenceSchemeV1{}, fmt.Errorf(
			"reference-scheme digest %q does not match canonical bytes %q",
			expected.String(),
			scheme.digest.String(),
		)
	}
	return scheme, nil
}

func encodeSchemeCanonical(
	designation DesignationRules,
	interpretation InterpretationRules,
	measurement MeasurementBranch,
	evaluation EvaluationBranch,
) ([]byte, error) {
	writer := newSchemeWriter()
	writer.addRulePins(designation.pins)
	writer.addRulePins(interpretation.pins)
	if err := writer.addMeasurementBranch(measurement); err != nil {
		return nil, err
	}
	if err := writer.addEvaluationBranch(evaluation); err != nil {
		return nil, err
	}
	if writer.err != nil {
		return nil, writer.err
	}
	canonical := writer.bytes()
	if len(canonical) > maximumCanonicalSchemeBytes {
		return nil, fmt.Errorf(
			"project-memory reference scheme exceeds %d bytes",
			maximumCanonicalSchemeBytes,
		)
	}
	return canonical, nil
}

func encodeRulePinCanonical(pin ExactRulePin) []byte {
	writer := canonicalWriter{}
	writer.addRulePin(pin)
	return writer.bytes()
}

func digestSchemeCanonical(
	canonical []byte,
) (ReferenceSchemeDigest, error) {
	sum := sha256.Sum256(canonical)
	encoded := hex.EncodeToString(sum[:])
	return ParseReferenceSchemeDigest("sha256:" + encoded)
}

func validateMeasurementBranch(branch MeasurementBranch) error {
	switch value := branch.(type) {
	case MeasurementRules:
		_, err := normalizeRulePins(
			"measurement rules",
			SemanticRoleMeasurement,
			value.pins,
		)
		return err
	case MeasurementNotApplicable:
		return validateRuleForRole(
			"measurement not-applicable rule",
			SemanticRoleMeasurement,
			value.rule,
		)
	default:
		return fmt.Errorf("explicit measurement Rules or NotApplicable branch is required")
	}
}

func validateEvaluationBranch(branch EvaluationBranch) error {
	switch value := branch.(type) {
	case EvaluationRules:
		_, err := normalizeRulePins(
			"evaluation rules",
			SemanticRoleEvaluation,
			value.pins,
		)
		return err
	case EvaluationNotApplicable:
		return validateRuleForRole(
			"evaluation not-applicable rule",
			SemanticRoleEvaluation,
			value.rule,
		)
	default:
		return fmt.Errorf("explicit evaluation Rules or NotApplicable branch is required")
	}
}

func cloneMeasurementBranch(branch MeasurementBranch) MeasurementBranch {
	switch value := branch.(type) {
	case MeasurementRules:
		return MeasurementRules{pins: value.Pins()}
	case MeasurementNotApplicable:
		return value
	default:
		return nil
	}
}

func cloneEvaluationBranch(branch EvaluationBranch) EvaluationBranch {
	switch value := branch.(type) {
	case EvaluationRules:
		return EvaluationRules{pins: value.Pins()}
	case EvaluationNotApplicable:
		return value
	default:
		return nil
	}
}

type canonicalWriter struct {
	value []byte
	err   error
}

func newSchemeWriter() canonicalWriter {
	writer := canonicalWriter{}
	writer.addString(projectMemoryReferenceSchemeV1Domain)
	return writer
}

func (writer *canonicalWriter) addByte(value byte) {
	if writer.err != nil {
		return
	}
	writer.value = append(writer.value, value)
}

func (writer *canonicalWriter) addUint32(value uint32) {
	if writer.err != nil {
		return
	}
	var encoded [4]byte
	binary.BigEndian.PutUint32(encoded[:], value)
	writer.value = append(writer.value, encoded[:]...)
}

func (writer *canonicalWriter) addString(value string) {
	length, err := canonicalUint32Length("canonical string", len(value))
	if err != nil {
		writer.reject(err)
		return
	}
	writer.addUint32(length)
	writer.value = append(writer.value, []byte(value)...)
}

func (writer *canonicalWriter) addRulePins(pins []ExactRulePin) {
	count, err := canonicalUint32Length("canonical rule-pin", len(pins))
	if err != nil {
		writer.reject(err)
		return
	}
	writer.addUint32(count)
	for _, pin := range pins {
		writer.addRulePin(pin)
	}
}

func (writer *canonicalWriter) addRulePin(pin ExactRulePin) {
	writer.addString(string(pin.role))
	writer.addString(pin.rule.String())
	writer.addString(pin.source.revision.String())
	writer.addString(pin.source.carrier.String())
	writer.addString(pin.source.edition.String())
	writer.addString(pin.source.digest.String())
	switch runtime := pin.runtime.(type) {
	case RuntimeNotRequired:
		writer.addByte(runtimeNotRequiredTag)
	case RuntimeRequired:
		writer.addByte(runtimeRequiredTag)
		writer.addString(runtime.mechanism.artifact.String())
		writer.addString(runtime.mechanism.edition.String())
		writer.addString(runtime.mechanism.digest.String())
	}
}

func (writer *canonicalWriter) addMeasurementBranch(
	branch MeasurementBranch,
) error {
	switch value := branch.(type) {
	case MeasurementRules:
		writer.addByte(rulesBranchTag)
		writer.addRulePins(value.pins)
		return nil
	case MeasurementNotApplicable:
		writer.addByte(notApplicableBranchTag)
		writer.addRulePin(value.rule)
		return nil
	default:
		return fmt.Errorf("explicit measurement branch is required")
	}
}

func (writer *canonicalWriter) addEvaluationBranch(
	branch EvaluationBranch,
) error {
	switch value := branch.(type) {
	case EvaluationRules:
		writer.addByte(rulesBranchTag)
		writer.addRulePins(value.pins)
		return nil
	case EvaluationNotApplicable:
		writer.addByte(notApplicableBranchTag)
		writer.addRulePin(value.rule)
		return nil
	default:
		return fmt.Errorf("explicit evaluation branch is required")
	}
}

func (writer canonicalWriter) bytes() []byte {
	if writer.err != nil {
		return nil
	}
	return append([]byte(nil), writer.value...)
}

func (writer *canonicalWriter) reject(err error) {
	if writer.err == nil {
		writer.err = err
	}
}

func canonicalUint32Length(name string, value int) (uint32, error) {
	if value < 0 || uint64(value) > uint64(math.MaxUint32) {
		return 0, fmt.Errorf("%s length is outside uint32 range", name)
	}
	return uint32(value), nil
}

type canonicalReader struct {
	value  []byte
	offset int
}

func newSchemeReader(canonical []byte) (*canonicalReader, error) {
	if len(canonical) == 0 || len(canonical) > maximumCanonicalSchemeBytes {
		return nil, fmt.Errorf("project-memory reference-scheme size is outside bounds")
	}
	reader := &canonicalReader{value: canonical}
	domain, err := reader.readString("canonical domain")
	if err != nil {
		return nil, err
	}
	if domain != projectMemoryReferenceSchemeV1Domain {
		return nil, fmt.Errorf("project-memory reference-scheme canonical domain is invalid")
	}
	return reader, nil
}

func (reader *canonicalReader) readByte(name string) (byte, error) {
	if reader.remaining() < 1 {
		return 0, fmt.Errorf("%s is truncated", name)
	}
	value := reader.value[reader.offset]
	reader.offset++
	return value, nil
}

func (reader *canonicalReader) readUint32(name string) (uint32, error) {
	if reader.remaining() < 4 {
		return 0, fmt.Errorf("%s is truncated", name)
	}
	end := reader.offset + 4
	value := binary.BigEndian.Uint32(reader.value[reader.offset:end])
	reader.offset = end
	return value, nil
}

func (reader *canonicalReader) readString(name string) (string, error) {
	length, err := reader.readUint32(name + " length")
	if err != nil {
		return "", err
	}
	if length > maximumCanonicalTextBytes {
		return "", fmt.Errorf("%s exceeds %d bytes", name, maximumCanonicalTextBytes)
	}
	lengthBytes := int(length)
	if reader.remaining() < lengthBytes {
		return "", fmt.Errorf("%s is truncated", name)
	}
	end := reader.offset + lengthBytes
	raw := reader.value[reader.offset:end]
	reader.offset = end
	if !utf8.Valid(raw) {
		return "", fmt.Errorf("%s is not valid UTF-8", name)
	}
	value := string(raw)
	if value == "" || containsControl(value) {
		return "", fmt.Errorf("%s is not canonical text", name)
	}
	return value, nil
}

func (reader *canonicalReader) readRulePins(
	name string,
	role SemanticRole,
) ([]ExactRulePin, error) {
	count, err := reader.readRuleCount(name)
	if err != nil {
		return nil, err
	}
	pins := make([]ExactRulePin, 0, count)
	for index := 0; index < count; index++ {
		pin, pinErr := reader.readRulePin(name)
		if pinErr != nil {
			return nil, fmt.Errorf("%s pin %d: %w", name, index, pinErr)
		}
		if pin.role != role {
			return nil, fmt.Errorf(
				"%s pin %d has semantic role %q",
				name,
				index,
				pin.role,
			)
		}
		pins = append(pins, pin)
	}
	return pins, nil
}

func (reader *canonicalReader) readRuleCount(name string) (int, error) {
	count, err := reader.readUint32(name + " count")
	if err != nil {
		return 0, err
	}
	if count == 0 {
		return 0, fmt.Errorf("%s require at least one exact rule pin", name)
	}
	if count > maximumRulePins {
		return 0, fmt.Errorf("%s exceed %d pins", name, maximumRulePins)
	}
	return int(count), nil
}

func (reader *canonicalReader) readRulePin(name string) (ExactRulePin, error) {
	roleRaw, err := reader.readString(name + " semantic role")
	if err != nil {
		return ExactRulePin{}, err
	}
	ruleRaw, err := reader.readString(name + " RuleRef")
	if err != nil {
		return ExactRulePin{}, err
	}
	revisionRaw, err := reader.readString(name + " source revision")
	if err != nil {
		return ExactRulePin{}, err
	}
	carrierRaw, err := reader.readString(name + " source carrier")
	if err != nil {
		return ExactRulePin{}, err
	}
	editionRaw, err := reader.readString(name + " source carrier edition")
	if err != nil {
		return ExactRulePin{}, err
	}
	digestRaw, err := reader.readString(name + " source carrier digest")
	if err != nil {
		return ExactRulePin{}, err
	}
	runtime, err := reader.readRuntimeRequirement(name)
	if err != nil {
		return ExactRulePin{}, err
	}
	role := SemanticRole(roleRaw)
	rule, err := typedmemory.NewRuleRef(ruleRaw)
	if err != nil {
		return ExactRulePin{}, fmt.Errorf("%s RuleRef: %w", name, err)
	}
	revision, err := typedmemory.NewSourceRevision(revisionRaw)
	if err != nil {
		return ExactRulePin{}, fmt.Errorf("%s source revision: %w", name, err)
	}
	carrier, err := typedmemory.NewCarrierRef(carrierRaw)
	if err != nil {
		return ExactRulePin{}, fmt.Errorf("%s source carrier: %w", name, err)
	}
	edition, err := typedmemory.NewCarrierEdition(editionRaw)
	if err != nil {
		return ExactRulePin{}, fmt.Errorf("%s source carrier edition: %w", name, err)
	}
	digest, err := typedmemory.NewSHA256Digest(digestRaw)
	if err != nil {
		return ExactRulePin{}, fmt.Errorf("%s source carrier digest: %w", name, err)
	}
	source, err := NewExactSourceCarrierPin(ExactSourceCarrierPinInput{
		SourceRevision: revision,
		Carrier:        carrier,
		Edition:        edition,
		Digest:         digest,
	})
	if err != nil {
		return ExactRulePin{}, err
	}
	return NewExactRulePin(ExactRulePinInput{
		Role:    role,
		Rule:    rule,
		Source:  source,
		Runtime: runtime,
	})
}

func (reader *canonicalReader) readRuntimeRequirement(
	name string,
) (RuntimeRequirement, error) {
	tag, err := reader.readByte(name + " runtime requirement tag")
	if err != nil {
		return nil, err
	}
	if tag == runtimeNotRequiredTag {
		return NewRuntimeNotRequired(), nil
	}
	if tag != runtimeRequiredTag {
		return nil, fmt.Errorf("%s runtime requirement tag %d is invalid", name, tag)
	}
	artifactRaw, err := reader.readString(name + " runtime artifact")
	if err != nil {
		return nil, err
	}
	editionRaw, err := reader.readString(name + " runtime edition")
	if err != nil {
		return nil, err
	}
	digestRaw, err := reader.readString(name + " runtime digest")
	if err != nil {
		return nil, err
	}
	artifact, err := typedmemory.NewCarrierRef(artifactRaw)
	if err != nil {
		return nil, err
	}
	edition, err := typedmemory.NewCarrierEdition(editionRaw)
	if err != nil {
		return nil, err
	}
	digest, err := typedmemory.NewSHA256Digest(digestRaw)
	if err != nil {
		return nil, err
	}
	mechanism, err := NewExactRuntimeMechanismPin(ExactRuntimeMechanismPinInput{
		Artifact: artifact,
		Edition:  edition,
		Digest:   digest,
	})
	if err != nil {
		return nil, err
	}
	return NewRuntimeRequired(mechanism)
}

func (reader *canonicalReader) readMeasurementBranch() (MeasurementBranch, error) {
	tag, err := reader.readByte("measurement branch tag")
	if err != nil {
		return nil, err
	}
	if tag == rulesBranchTag {
		pins, pinsErr := reader.readRulePins(
			"measurement rules",
			SemanticRoleMeasurement,
		)
		if pinsErr != nil {
			return nil, pinsErr
		}
		return NewMeasurementRules(pins)
	}
	if tag != notApplicableBranchTag {
		return nil, fmt.Errorf("measurement branch tag %d is invalid", tag)
	}
	rule, err := reader.readRulePin("measurement not-applicable rule")
	if err != nil {
		return nil, err
	}
	return NewMeasurementNotApplicable(rule)
}

func (reader *canonicalReader) readEvaluationBranch() (EvaluationBranch, error) {
	tag, err := reader.readByte("evaluation branch tag")
	if err != nil {
		return nil, err
	}
	if tag == rulesBranchTag {
		pins, pinsErr := reader.readRulePins(
			"evaluation rules",
			SemanticRoleEvaluation,
		)
		if pinsErr != nil {
			return nil, pinsErr
		}
		return NewEvaluationRules(pins)
	}
	if tag != notApplicableBranchTag {
		return nil, fmt.Errorf("evaluation branch tag %d is invalid", tag)
	}
	rule, err := reader.readRulePin("evaluation not-applicable rule")
	if err != nil {
		return nil, err
	}
	return NewEvaluationNotApplicable(rule)
}

func (reader *canonicalReader) remaining() int {
	return len(reader.value) - reader.offset
}

func (reader *canonicalReader) requireEnd() error {
	if reader.remaining() != 0 {
		return fmt.Errorf(
			"project-memory reference scheme has %d trailing bytes",
			reader.remaining(),
		)
	}
	return nil
}

func containsControl(value string) bool {
	return bytes.ContainsFunc([]byte(value), unicode.IsControl)
}

func validateCanonicalText(name string, value string) error {
	if value == "" || value != strings.TrimSpace(value) {
		return fmt.Errorf("%s is not canonical text", name)
	}
	if len(value) > maximumCanonicalTextBytes {
		return fmt.Errorf("%s exceeds %d bytes", name, maximumCanonicalTextBytes)
	}
	if !utf8.ValidString(value) || containsControl(value) {
		return fmt.Errorf("%s is not canonical text", name)
	}
	return nil
}
