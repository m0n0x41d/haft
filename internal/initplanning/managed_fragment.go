package initplanning

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"regexp"
	"slices"
	"sort"
	"strings"
)

// ManagedFragment is the desired Haft-owned semantic fragment inside a shared
// carrier. The carrier itself remains user-owned. A manifest may establish
// ownership only for the exact coordinate and fragment digest recorded here.
type ManagedFragment struct {
	coordinate ManagedFragmentCoordinate
	component  Component
	content    []byte
	digest     string
	createMode fs.FileMode
}

type ManagedFragmentKind string

const (
	ManagedJSONObjectEntry    ManagedFragmentKind = "json_object_entry"
	ManagedJSONArrayMember    ManagedFragmentKind = "json_array_member"
	ManagedTOMLTableFamily    ManagedFragmentKind = "toml_table_family"
	ManagedTOMLTableSet       ManagedFragmentKind = "toml_table_set"
	ManagedYAMLMappingEntry   ManagedFragmentKind = "yaml_mapping_entry"
	ManagedYAMLSequenceMember ManagedFragmentKind = "yaml_sequence_member"
	ManagedHTMLCommentSection ManagedFragmentKind = "html_comment_section"
)

type managedCarrierSyntax string

const (
	managedCarrierJSON managedCarrierSyntax = "json"
	managedCarrierTOML managedCarrierSyntax = "toml"
	managedCarrierYAML managedCarrierSyntax = "yaml"
	managedCarrierText managedCarrierSyntax = "text"
)

// ManagedJSONCRewriteMergeEdition selects the JSONC-aware carrier codec. It
// accepts comments and trailing commas during observation, then writes
// canonical strict JSON when a managed mutation is required.
const ManagedJSONCRewriteMergeEdition = "jsonc.semantic-rewrite.v1"

// ManagedJSONArraySourceMergeEdition owns the source identity of one JSON
// array member. The carrier may represent that identity either as a scalar
// string or as an object whose other fields remain user-owned.
const ManagedJSONArraySourceMergeEdition = "json.array-source-member.v1"

type ManagedFragmentCoordinate struct {
	carrierPath  string
	kind         ManagedFragmentKind
	selector     string
	mergeEdition string
	jsonPath     []string
	memberID     string
	tomlPrefix   string
	tomlTables   []string
	yamlPath     []string
}

var tomlBareKeyPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
var tomlTableHeaderPattern = regexp.MustCompile(
	`^\s*(\[\[?)([A-Za-z0-9_-]+(?:\.[A-Za-z0-9_-]+)*)(\]\]?)\s*(?:#.*)?$`,
)

func NewJSONObjectEntryFragment(
	carrierPath string,
	component Component,
	selector []string,
	value []byte,
	createMode fs.FileMode,
	mergeEdition string,
) (ManagedFragment, error) {
	coordinate, err := newJSONManagedFragmentCoordinate(
		carrierPath,
		ManagedJSONObjectEntry,
		selector,
		"",
		mergeEdition,
	)
	if err != nil {
		return ManagedFragment{}, err
	}
	canonical, err := canonicalJSONValue(value)
	if err != nil {
		return ManagedFragment{}, fmt.Errorf("managed JSON object entry: %w", err)
	}
	return newManagedFragment(
		coordinate,
		component,
		canonical,
		createMode,
	)
}

func NewJSONArrayMemberFragment(
	carrierPath string,
	component Component,
	selector []string,
	memberID string,
	value []byte,
	createMode fs.FileMode,
	mergeEdition string,
) (ManagedFragment, error) {
	validatedMemberID, err := validateManagedFragmentToken(
		memberID,
		"managed JSON array member identity",
	)
	if err != nil {
		return ManagedFragment{}, err
	}
	coordinate, err := newJSONManagedFragmentCoordinate(
		carrierPath,
		ManagedJSONArrayMember,
		selector,
		validatedMemberID,
		mergeEdition,
	)
	if err != nil {
		return ManagedFragment{}, err
	}
	canonical, err := canonicalJSONValue(value)
	if err != nil {
		return ManagedFragment{}, fmt.Errorf("managed JSON array member: %w", err)
	}
	return newManagedFragment(
		coordinate,
		component,
		canonical,
		createMode,
	)
}

func NewTOMLTableFamilyFragment(
	carrierPath string,
	component Component,
	prefix string,
	value []byte,
	createMode fs.FileMode,
	mergeEdition string,
) (ManagedFragment, error) {
	canonicalPath, err := parseCanonicalAbsolutePath(carrierPath)
	if err != nil {
		return ManagedFragment{}, fmt.Errorf("managed TOML carrier path: %w", err)
	}
	validatedEdition, err := validateManagedMergeEdition(mergeEdition)
	if err != nil {
		return ManagedFragment{}, err
	}
	validatedPrefix, err := validateTOMLTablePrefix(prefix)
	if err != nil {
		return ManagedFragment{}, err
	}
	canonical, err := canonicalTOMLTableFamily(
		value,
		validatedPrefix,
	)
	if err != nil {
		return ManagedFragment{}, err
	}
	coordinate := ManagedFragmentCoordinate{
		carrierPath:  canonicalPath,
		kind:         ManagedTOMLTableFamily,
		selector:     validatedPrefix,
		mergeEdition: validatedEdition,
		tomlPrefix:   validatedPrefix,
	}
	return newManagedFragment(
		coordinate,
		component,
		canonical,
		createMode,
	)
}

// NewTOMLTableSetFragment owns only the listed exact TOML tables. Descendant
// tables outside the set remain user-owned even when they share the prefix.
func NewTOMLTableSetFragment(
	carrierPath string,
	component Component,
	prefix string,
	tables []string,
	value []byte,
	createMode fs.FileMode,
	mergeEdition string,
) (ManagedFragment, error) {
	canonicalPath, err := parseCanonicalAbsolutePath(carrierPath)
	if err != nil {
		return ManagedFragment{}, fmt.Errorf("managed TOML carrier path: %w", err)
	}
	validatedEdition, err := validateManagedMergeEdition(mergeEdition)
	if err != nil {
		return ManagedFragment{}, err
	}
	validatedPrefix, err := validateTOMLTablePrefix(prefix)
	if err != nil {
		return ManagedFragment{}, err
	}
	validatedTables, err := canonicalTOMLTableNames(
		validatedPrefix,
		tables,
	)
	if err != nil {
		return ManagedFragment{}, err
	}
	canonical, err := canonicalTOMLTableSet(
		value,
		validatedTables,
	)
	if err != nil {
		return ManagedFragment{}, err
	}
	coordinate := ManagedFragmentCoordinate{
		carrierPath:  canonicalPath,
		kind:         ManagedTOMLTableSet,
		selector:     validatedPrefix,
		mergeEdition: validatedEdition,
		tomlPrefix:   validatedPrefix,
		tomlTables:   validatedTables,
	}
	return newManagedFragment(
		coordinate,
		component,
		canonical,
		createMode,
	)
}

func NewYAMLMappingEntryFragment(
	carrierPath string,
	component Component,
	selector []string,
	value []byte,
	createMode fs.FileMode,
	mergeEdition string,
) (ManagedFragment, error) {
	coordinate, err := newYAMLManagedFragmentCoordinate(
		carrierPath,
		ManagedYAMLMappingEntry,
		selector,
		"",
		mergeEdition,
	)
	if err != nil {
		return ManagedFragment{}, err
	}
	canonical, err := canonicalYAMLValue(value)
	if err != nil {
		return ManagedFragment{}, fmt.Errorf(
			"managed YAML mapping entry: %w",
			err,
		)
	}
	return newManagedFragment(
		coordinate,
		component,
		canonical,
		createMode,
	)
}

func NewYAMLSequenceMemberFragment(
	carrierPath string,
	component Component,
	selector []string,
	memberID string,
	value []byte,
	createMode fs.FileMode,
	mergeEdition string,
) (ManagedFragment, error) {
	validatedMemberID, err := validateManagedFragmentToken(
		memberID,
		"managed YAML sequence member identity",
	)
	if err != nil {
		return ManagedFragment{}, err
	}
	coordinate, err := newYAMLManagedFragmentCoordinate(
		carrierPath,
		ManagedYAMLSequenceMember,
		selector,
		validatedMemberID,
		mergeEdition,
	)
	if err != nil {
		return ManagedFragment{}, err
	}
	canonical, err := canonicalYAMLValue(value)
	if err != nil {
		return ManagedFragment{}, fmt.Errorf(
			"managed YAML sequence member: %w",
			err,
		)
	}
	return newManagedFragment(
		coordinate,
		component,
		canonical,
		createMode,
	)
}

func newJSONManagedFragmentCoordinate(
	carrierPath string,
	kind ManagedFragmentKind,
	selector []string,
	memberID string,
	mergeEdition string,
) (ManagedFragmentCoordinate, error) {
	canonicalPath, err := parseCanonicalAbsolutePath(carrierPath)
	if err != nil {
		return ManagedFragmentCoordinate{}, fmt.Errorf("managed JSON carrier path: %w", err)
	}
	validatedEdition, err := validateManagedMergeEdition(mergeEdition)
	if err != nil {
		return ManagedFragmentCoordinate{}, err
	}
	jsonPath, pointer, err := canonicalJSONPointer(selector)
	if err != nil {
		return ManagedFragmentCoordinate{}, err
	}
	return ManagedFragmentCoordinate{
		carrierPath:  canonicalPath,
		kind:         kind,
		selector:     pointer,
		mergeEdition: validatedEdition,
		jsonPath:     jsonPath,
		memberID:     memberID,
	}, nil
}

func newYAMLManagedFragmentCoordinate(
	carrierPath string,
	kind ManagedFragmentKind,
	selector []string,
	memberID string,
	mergeEdition string,
) (ManagedFragmentCoordinate, error) {
	canonicalPath, err := parseCanonicalAbsolutePath(carrierPath)
	if err != nil {
		return ManagedFragmentCoordinate{}, fmt.Errorf(
			"managed YAML carrier path: %w",
			err,
		)
	}
	validatedEdition, err := validateManagedMergeEdition(mergeEdition)
	if err != nil {
		return ManagedFragmentCoordinate{}, err
	}
	yamlPath, pointer, err := canonicalYAMLPointer(selector)
	if err != nil {
		return ManagedFragmentCoordinate{}, err
	}
	return ManagedFragmentCoordinate{
		carrierPath:  canonicalPath,
		kind:         kind,
		selector:     pointer,
		mergeEdition: validatedEdition,
		memberID:     memberID,
		yamlPath:     yamlPath,
	}, nil
}

func newManagedFragment(
	coordinate ManagedFragmentCoordinate,
	component Component,
	content []byte,
	createMode fs.FileMode,
) (ManagedFragment, error) {
	if !coordinate.valid() {
		return ManagedFragment{}, fmt.Errorf("managed fragment coordinate is invalid")
	}
	if _, known := knownComponents[component]; !known {
		return ManagedFragment{}, fmt.Errorf("managed fragment component is not closed")
	}
	if len(content) == 0 {
		return ManagedFragment{}, fmt.Errorf("managed fragment content is empty")
	}
	if !validPermissionMode(createMode) {
		return ManagedFragment{}, fmt.Errorf("managed fragment create mode is invalid")
	}
	return ManagedFragment{
		coordinate: cloneManagedFragmentCoordinate(coordinate),
		component:  component,
		content:    slices.Clone(content),
		digest:     managedFragmentDigest(content),
		createMode: createMode.Perm(),
	}, nil
}

func validateManagedMergeEdition(raw string) (string, error) {
	if !adapterEditionPattern.MatchString(raw) {
		return "", fmt.Errorf("managed fragment merge edition is invalid")
	}
	return raw, nil
}

func validateManagedFragmentToken(
	raw string,
	label string,
) (string, error) {
	if raw == "" || raw != strings.TrimSpace(raw) {
		return "", fmt.Errorf("%s is invalid", label)
	}
	return raw, nil
}

func canonicalJSONPointer(
	raw []string,
) ([]string, string, error) {
	if len(raw) == 0 {
		return nil, "", fmt.Errorf("managed JSON selector cannot be empty")
	}
	values := make([]string, len(raw))
	encoded := make([]string, len(raw))
	for index, candidate := range raw {
		value, err := validateManagedFragmentToken(
			candidate,
			"managed JSON selector token",
		)
		if err != nil {
			return nil, "", err
		}
		values[index] = value
		escaped := strings.ReplaceAll(value, "~", "~0")
		escaped = strings.ReplaceAll(escaped, "/", "~1")
		encoded[index] = escaped
	}
	return values, "/" + strings.Join(encoded, "/"), nil
}

func validateTOMLTablePrefix(raw string) (string, error) {
	if raw == "" || raw != strings.TrimSpace(raw) {
		return "", fmt.Errorf("managed TOML table-family prefix is invalid")
	}
	parts := strings.Split(raw, ".")
	for _, part := range parts {
		if !tomlBareKeyPattern.MatchString(part) {
			return "", fmt.Errorf(
				"managed TOML table-family prefix %q is not a bare-key family",
				raw,
			)
		}
	}
	return raw, nil
}

func managedFragmentDigest(value []byte) string {
	digest := sha256.Sum256(value)
	return fmt.Sprintf("sha256:%x", digest)
}

func (coordinate ManagedFragmentCoordinate) valid() bool {
	if coordinate.carrierPath == "" ||
		coordinate.selector == "" ||
		coordinate.mergeEdition == "" {
		return false
	}
	if coordinate.kind == ManagedJSONObjectEntry {
		return len(coordinate.jsonPath) > 0 &&
			coordinate.memberID == "" &&
			coordinate.tomlPrefix == "" &&
			len(coordinate.tomlTables) == 0 &&
			len(coordinate.yamlPath) == 0
	}
	if coordinate.kind == ManagedJSONArrayMember {
		return len(coordinate.jsonPath) > 0 &&
			coordinate.memberID != "" &&
			coordinate.tomlPrefix == "" &&
			len(coordinate.tomlTables) == 0 &&
			len(coordinate.yamlPath) == 0
	}
	if coordinate.kind == ManagedTOMLTableFamily {
		return len(coordinate.jsonPath) == 0 &&
			coordinate.memberID == "" &&
			coordinate.tomlPrefix == coordinate.selector &&
			len(coordinate.tomlTables) == 0 &&
			len(coordinate.yamlPath) == 0
	}
	if coordinate.kind == ManagedTOMLTableSet {
		_, err := canonicalTOMLTableNames(
			coordinate.tomlPrefix,
			coordinate.tomlTables,
		)
		return err == nil &&
			len(coordinate.jsonPath) == 0 &&
			coordinate.memberID == "" &&
			coordinate.tomlPrefix == coordinate.selector &&
			len(coordinate.tomlTables) > 0 &&
			len(coordinate.yamlPath) == 0
	}
	if coordinate.kind == ManagedYAMLMappingEntry {
		return len(coordinate.jsonPath) == 0 &&
			coordinate.memberID == "" &&
			coordinate.tomlPrefix == "" &&
			len(coordinate.tomlTables) == 0 &&
			len(coordinate.yamlPath) > 0
	}
	if coordinate.kind == ManagedYAMLSequenceMember {
		return len(coordinate.jsonPath) == 0 &&
			coordinate.memberID != "" &&
			coordinate.tomlPrefix == "" &&
			len(coordinate.tomlTables) == 0 &&
			len(coordinate.yamlPath) > 0
	}
	if coordinate.kind == ManagedHTMLCommentSection {
		_, err := validateHTMLCommentSectionNamespace(
			coordinate.selector,
		)
		return err == nil &&
			len(coordinate.jsonPath) == 0 &&
			coordinate.memberID == "" &&
			coordinate.tomlPrefix == "" &&
			len(coordinate.tomlTables) == 0 &&
			len(coordinate.yamlPath) == 0
	}
	return false
}

func cloneManagedFragmentCoordinate(
	coordinate ManagedFragmentCoordinate,
) ManagedFragmentCoordinate {
	return ManagedFragmentCoordinate{
		carrierPath:  coordinate.carrierPath,
		kind:         coordinate.kind,
		selector:     coordinate.selector,
		mergeEdition: coordinate.mergeEdition,
		jsonPath:     slices.Clone(coordinate.jsonPath),
		memberID:     coordinate.memberID,
		tomlPrefix:   coordinate.tomlPrefix,
		tomlTables:   slices.Clone(coordinate.tomlTables),
		yamlPath:     slices.Clone(coordinate.yamlPath),
	}
}

func cloneManagedFragment(fragment ManagedFragment) ManagedFragment {
	return ManagedFragment{
		coordinate: cloneManagedFragmentCoordinate(fragment.coordinate),
		component:  fragment.component,
		content:    slices.Clone(fragment.content),
		digest:     fragment.digest,
		createMode: fragment.createMode,
	}
}

func cloneManagedFragments(source []ManagedFragment) []ManagedFragment {
	result := make([]ManagedFragment, len(source))
	for index, fragment := range source {
		result[index] = cloneManagedFragment(fragment)
	}
	return result
}

func (coordinate ManagedFragmentCoordinate) CarrierPath() string {
	return coordinate.carrierPath
}

func (coordinate ManagedFragmentCoordinate) Kind() ManagedFragmentKind {
	return coordinate.kind
}

func (coordinate ManagedFragmentCoordinate) Selector() string {
	return coordinate.selector
}

func (coordinate ManagedFragmentCoordinate) MergeEdition() string {
	return coordinate.mergeEdition
}

func (coordinate ManagedFragmentCoordinate) MemberID() string {
	return coordinate.memberID
}

func (fragment ManagedFragment) Coordinate() ManagedFragmentCoordinate {
	return cloneManagedFragmentCoordinate(fragment.coordinate)
}

func (fragment ManagedFragment) Component() Component {
	return fragment.component
}

func (fragment ManagedFragment) Content() []byte {
	return slices.Clone(fragment.content)
}

func (fragment ManagedFragment) Digest() string {
	return fragment.digest
}

func (fragment ManagedFragment) CreateMode() fs.FileMode {
	return fragment.createMode
}

type ManagedFragmentRecord struct {
	coordinate ManagedFragmentCoordinate
	component  Component
	digest     string
}

func (fragment ManagedFragment) Record() ManagedFragmentRecord {
	return ManagedFragmentRecord{
		coordinate: fragment.Coordinate(),
		component:  fragment.component,
		digest:     fragment.digest,
	}
}

// NewKnownLegacyManagedFragmentRecord pairs one desired coordinate with a
// caller-supplied, observer-normalized historical shape and hashes those bytes
// verbatim. The record carries no ownership by itself; only an explicit
// known-legacy registry may use it as adoption evidence.
func NewKnownLegacyManagedFragmentRecord(
	template ManagedFragment,
	observedContent []byte,
) (ManagedFragmentRecord, error) {
	if !template.coordinate.valid() ||
		!template.Record().valid() {
		return ManagedFragmentRecord{}, fmt.Errorf(
			"known legacy managed fragment template is invalid",
		)
	}
	if len(observedContent) == 0 {
		return ManagedFragmentRecord{}, fmt.Errorf(
			"known legacy managed fragment content is empty",
		)
	}
	record := ManagedFragmentRecord{
		coordinate: template.Coordinate(),
		component:  template.component,
		digest:     managedFragmentDigest(observedContent),
	}
	if !record.valid() {
		return ManagedFragmentRecord{}, fmt.Errorf(
			"known legacy managed fragment record is invalid",
		)
	}
	return record, nil
}

func cloneManagedFragmentRecord(
	record ManagedFragmentRecord,
) ManagedFragmentRecord {
	return ManagedFragmentRecord{
		coordinate: cloneManagedFragmentCoordinate(record.coordinate),
		component:  record.component,
		digest:     record.digest,
	}
}

func cloneManagedFragmentRecords(
	source []ManagedFragmentRecord,
) []ManagedFragmentRecord {
	result := make([]ManagedFragmentRecord, len(source))
	for index, record := range source {
		result[index] = cloneManagedFragmentRecord(record)
	}
	return result
}

func (record ManagedFragmentRecord) valid() bool {
	_, componentKnown := knownComponents[record.component]
	return record.coordinate.valid() &&
		componentKnown &&
		sha256DigestPattern.MatchString(record.digest)
}

func (record ManagedFragmentRecord) Coordinate() ManagedFragmentCoordinate {
	return cloneManagedFragmentCoordinate(record.coordinate)
}

func (record ManagedFragmentRecord) Digest() string {
	return record.digest
}

func (record ManagedFragmentRecord) Component() Component {
	return record.component
}

type ManagedFragmentBaselineKind string

const (
	ManagedFragmentNoPriorManifest ManagedFragmentBaselineKind = "no_prior_manifest"
	ManagedFragmentManifest        ManagedFragmentBaselineKind = "installation_manifest"
)

type ManagedFragmentBaseline struct {
	kind    ManagedFragmentBaselineKind
	records []ManagedFragmentRecord
	basis   OwnershipBasis
}

func NoPriorManagedFragmentBaseline() ManagedFragmentBaseline {
	return ManagedFragmentBaseline{kind: ManagedFragmentNoPriorManifest}
}

func NewManagedFragmentManifestBaseline(
	records []ManagedFragmentRecord,
	basis OwnershipBasis,
) (ManagedFragmentBaseline, error) {
	if !basis.valid() || basis.kind != OwnershipManifestReceipt {
		return ManagedFragmentBaseline{}, fmt.Errorf(
			"managed fragment manifest baseline requires a manifest receipt",
		)
	}
	validated, err := canonicalManagedFragmentRecords(records)
	if err != nil {
		return ManagedFragmentBaseline{}, err
	}
	if len(validated) == 0 {
		return ManagedFragmentBaseline{}, fmt.Errorf(
			"managed fragment manifest baseline is empty",
		)
	}
	return ManagedFragmentBaseline{
		kind:    ManagedFragmentManifest,
		records: validated,
		basis:   basis,
	}, nil
}

func (baseline ManagedFragmentBaseline) Kind() ManagedFragmentBaselineKind {
	return baseline.kind
}

func (baseline ManagedFragmentBaseline) Records() []ManagedFragmentRecord {
	return cloneManagedFragmentRecords(baseline.records)
}

func (baseline ManagedFragmentBaseline) OwnershipBasis() OwnershipBasis {
	return baseline.basis
}

type ManagedFragmentLegacyRegistry struct {
	selected bool
	records  []ManagedFragmentRecord
	basis    OwnershipBasis
}

func NoManagedFragmentLegacyRegistry() ManagedFragmentLegacyRegistry {
	return ManagedFragmentLegacyRegistry{}
}

func NewManagedFragmentLegacyRegistry(
	records []ManagedFragmentRecord,
	basis OwnershipBasis,
) (ManagedFragmentLegacyRegistry, error) {
	if !basis.valid() || basis.kind != OwnershipLegacyRegistry {
		return ManagedFragmentLegacyRegistry{}, fmt.Errorf(
			"managed fragment legacy registry requires a legacy ownership basis",
		)
	}
	validated, err := canonicalManagedFragmentLegacyRecords(records)
	if err != nil {
		return ManagedFragmentLegacyRegistry{}, err
	}
	if len(validated) == 0 {
		return ManagedFragmentLegacyRegistry{}, fmt.Errorf(
			"managed fragment legacy registry is empty",
		)
	}
	return ManagedFragmentLegacyRegistry{
		selected: true,
		records:  validated,
		basis:    basis,
	}, nil
}

func canonicalManagedFragmentRecords(
	raw []ManagedFragmentRecord,
) ([]ManagedFragmentRecord, error) {
	records := cloneManagedFragmentRecords(raw)
	sort.Slice(records, func(left int, right int) bool {
		return managedFragmentCoordinateKey(records[left].coordinate) <
			managedFragmentCoordinateKey(records[right].coordinate)
	})
	previous := ""
	for _, record := range records {
		if !record.valid() {
			return nil, fmt.Errorf("managed fragment record is invalid")
		}
		key := managedFragmentCoordinateKey(record.coordinate)
		if key == previous {
			return nil, fmt.Errorf(
				"managed fragment records repeat coordinate %s",
				record.coordinate.selector,
			)
		}
		previous = key
	}
	return records, nil
}

// canonicalManagedFragmentLegacyRecords permits a closed set of historical
// digests for one semantic coordinate. A manifest still owns exactly one
// digest per coordinate; only a legacy registry can recognize several exact
// predecessor representations.
func canonicalManagedFragmentLegacyRecords(
	raw []ManagedFragmentRecord,
) ([]ManagedFragmentRecord, error) {
	records := cloneManagedFragmentRecords(raw)
	sort.Slice(records, func(left int, right int) bool {
		leftKey := managedFragmentCoordinateKey(records[left].coordinate)
		rightKey := managedFragmentCoordinateKey(records[right].coordinate)
		if leftKey != rightKey {
			return leftKey < rightKey
		}
		return records[left].digest < records[right].digest
	})
	previousKey := ""
	previousDigest := ""
	for _, record := range records {
		if !record.valid() {
			return nil, fmt.Errorf("managed fragment legacy record is invalid")
		}
		key := managedFragmentCoordinateKey(record.coordinate)
		if key == previousKey && record.digest == previousDigest {
			return nil, fmt.Errorf(
				"managed fragment legacy records repeat coordinate %s digest %s",
				record.coordinate.selector,
				record.digest,
			)
		}
		previousKey = key
		previousDigest = record.digest
	}
	return records, nil
}

func managedFragmentCoordinateKey(
	coordinate ManagedFragmentCoordinate,
) string {
	kind := coordinate.kind
	if kind == ManagedTOMLTableSet {
		// A table set is the narrower successor of the table-family
		// coordinate. Keeping one logical key lets an existing manifest
		// migrate after the observation has proved the exact owned tables
		// still match its recorded digest.
		kind = ManagedTOMLTableFamily
	}
	return strings.Join(
		[]string{
			coordinate.carrierPath,
			string(kind),
			coordinate.selector,
			coordinate.memberID,
			coordinate.mergeEdition,
		},
		"\x00",
	)
}

type ManagedCarrierInputKind string

const (
	ManagedCarrierMissing ManagedCarrierInputKind = "missing"
	ManagedCarrierPresent ManagedCarrierInputKind = "present"
)

type ManagedCarrierInput struct {
	path    string
	kind    ManagedCarrierInputKind
	content []byte
	digest  string
	mode    fs.FileMode
}

func NewMissingManagedCarrier(
	path string,
) (ManagedCarrierInput, error) {
	canonical, err := parseCanonicalAbsolutePath(path)
	if err != nil {
		return ManagedCarrierInput{}, err
	}
	return ManagedCarrierInput{
		path: canonical,
		kind: ManagedCarrierMissing,
	}, nil
}

func NewPresentManagedCarrier(
	path string,
	content []byte,
	mode fs.FileMode,
) (ManagedCarrierInput, error) {
	canonical, err := parseCanonicalAbsolutePath(path)
	if err != nil {
		return ManagedCarrierInput{}, err
	}
	if !validPermissionMode(mode) {
		return ManagedCarrierInput{}, fmt.Errorf("managed carrier mode is invalid")
	}
	return ManagedCarrierInput{
		path:    canonical,
		kind:    ManagedCarrierPresent,
		content: slices.Clone(content),
		digest:  managedFragmentDigest(content),
		mode:    mode.Perm(),
	}, nil
}

func cloneManagedCarrierInput(
	input ManagedCarrierInput,
) ManagedCarrierInput {
	return ManagedCarrierInput{
		path:    input.path,
		kind:    input.kind,
		content: slices.Clone(input.content),
		digest:  input.digest,
		mode:    input.mode,
	}
}

func (input ManagedCarrierInput) Path() string {
	return input.path
}

func (input ManagedCarrierInput) Kind() ManagedCarrierInputKind {
	return input.kind
}

func (input ManagedCarrierInput) Content() []byte {
	return slices.Clone(input.content)
}

func (input ManagedCarrierInput) Digest() string {
	return input.digest
}

func (input ManagedCarrierInput) Mode() fs.FileMode {
	return input.mode
}

type managedFragmentProbe struct {
	coordinate       ManagedFragmentCoordinate
	candidateDigests []string
}

type ManagedFragmentObservationPlan struct {
	carrierPath                       string
	syntax                            managedCarrierSyntax
	mergeEdition                      string
	createMode                        fs.FileMode
	components                        ComponentSet
	retainedManagedFragmentComponents []Component
	desired                           []ManagedFragment
	baseline                          ManagedFragmentBaseline
	legacy                            ManagedFragmentLegacyRegistry
	probes                            []managedFragmentProbe
}

func (plan ManagedFragmentObservationPlan) CarrierPath() string {
	return plan.carrierPath
}

func (plan ManagedFragmentObservationPlan) Components() ComponentSet {
	return ComponentSet{values: plan.components.Values()}
}

func BuildManagedFragmentObservationPlan(
	desired []ManagedFragment,
	baseline ManagedFragmentBaseline,
	legacy ManagedFragmentLegacyRegistry,
) (ManagedFragmentObservationPlan, error) {
	desiredValues, err := canonicalDesiredManagedFragments(desired)
	if err != nil {
		return ManagedFragmentObservationPlan{}, err
	}
	if err := validateManagedFragmentBaseline(baseline); err != nil {
		return ManagedFragmentObservationPlan{}, err
	}
	if err := validateManagedFragmentLegacyRegistry(legacy); err != nil {
		return ManagedFragmentObservationPlan{}, err
	}
	coordinates := make(map[string]ManagedFragmentCoordinate)
	candidates := make(map[string]map[string]struct{})
	addRecord := func(record ManagedFragmentRecord) {
		key := managedFragmentCoordinateKey(record.coordinate)
		coordinates[key] = cloneManagedFragmentCoordinate(record.coordinate)
		if candidates[key] == nil {
			candidates[key] = make(map[string]struct{})
		}
		candidates[key][record.digest] = struct{}{}
	}
	for _, fragment := range desiredValues {
		addRecord(fragment.Record())
	}
	for _, record := range baseline.records {
		addRecord(record)
	}
	for _, record := range legacy.records {
		addRecord(record)
	}
	if len(coordinates) == 0 {
		return ManagedFragmentObservationPlan{}, fmt.Errorf(
			"managed fragment observation plan is empty",
		)
	}
	orderedKeys := make([]string, 0, len(coordinates))
	for key := range coordinates {
		orderedKeys = append(orderedKeys, key)
	}
	sort.Strings(orderedKeys)
	probes := make([]managedFragmentProbe, len(orderedKeys))
	for index, key := range orderedKeys {
		digests := make([]string, 0, len(candidates[key]))
		for digest := range candidates[key] {
			digests = append(digests, digest)
		}
		sort.Strings(digests)
		probes[index] = managedFragmentProbe{
			coordinate:       cloneManagedFragmentCoordinate(coordinates[key]),
			candidateDigests: digests,
		}
	}
	carrierPath, syntax, edition, createMode, components, err := validateManagedFragmentGroup(
		desiredValues,
		baseline.records,
		legacy.records,
	)
	if err != nil {
		return ManagedFragmentObservationPlan{}, err
	}
	return ManagedFragmentObservationPlan{
		carrierPath:  carrierPath,
		syntax:       syntax,
		mergeEdition: edition,
		createMode:   createMode,
		components:   ComponentSet{values: components.Values()},
		desired:      desiredValues,
		baseline:     cloneManagedFragmentBaseline(baseline),
		legacy:       cloneManagedFragmentLegacyRegistry(legacy),
		probes:       cloneManagedFragmentProbes(probes),
	}, nil
}

func canonicalDesiredManagedFragments(
	raw []ManagedFragment,
) ([]ManagedFragment, error) {
	values := cloneManagedFragments(raw)
	sort.Slice(values, func(left int, right int) bool {
		return managedFragmentCoordinateKey(values[left].coordinate) <
			managedFragmentCoordinateKey(values[right].coordinate)
	})
	previous := ""
	for _, fragment := range values {
		if !fragment.coordinate.valid() ||
			!sha256DigestPattern.MatchString(fragment.digest) ||
			!validPermissionMode(fragment.createMode) {
			return nil, fmt.Errorf("desired managed fragment is invalid")
		}
		key := managedFragmentCoordinateKey(fragment.coordinate)
		if key == previous {
			return nil, fmt.Errorf(
				"desired managed fragments repeat coordinate %s",
				fragment.coordinate.selector,
			)
		}
		previous = key
	}
	return values, nil
}

func validateManagedFragmentBaseline(
	baseline ManagedFragmentBaseline,
) error {
	if baseline.kind == ManagedFragmentNoPriorManifest {
		if len(baseline.records) != 0 || baseline.basis.valid() {
			return fmt.Errorf("no-prior managed fragment baseline carries ownership")
		}
		return nil
	}
	if baseline.kind != ManagedFragmentManifest {
		return fmt.Errorf("managed fragment baseline kind is invalid")
	}
	if !baseline.basis.valid() ||
		baseline.basis.kind != OwnershipManifestReceipt {
		return fmt.Errorf("managed fragment manifest baseline basis is invalid")
	}
	records, err := canonicalManagedFragmentRecords(baseline.records)
	if err != nil {
		return err
	}
	if len(records) == 0 {
		return fmt.Errorf("managed fragment manifest baseline is empty")
	}
	return nil
}

func validateManagedFragmentLegacyRegistry(
	registry ManagedFragmentLegacyRegistry,
) error {
	if !registry.selected {
		if len(registry.records) != 0 || registry.basis.valid() {
			return fmt.Errorf("unselected managed fragment legacy registry carries ownership")
		}
		return nil
	}
	if !registry.basis.valid() ||
		registry.basis.kind != OwnershipLegacyRegistry {
		return fmt.Errorf("managed fragment legacy registry basis is invalid")
	}
	records, err := canonicalManagedFragmentLegacyRecords(registry.records)
	if err != nil {
		return err
	}
	if len(records) == 0 {
		return fmt.Errorf("managed fragment legacy registry is empty")
	}
	return nil
}

func validateManagedFragmentGroup(
	desired []ManagedFragment,
	baseline []ManagedFragmentRecord,
	legacy []ManagedFragmentRecord,
) (
	string,
	managedCarrierSyntax,
	string,
	fs.FileMode,
	ComponentSet,
	error,
) {
	var carrierPath string
	var syntax managedCarrierSyntax
	var edition string
	var createMode fs.FileMode
	components := make(map[Component]struct{})
	componentByCoordinate := make(map[string]Component)
	acceptCoordinate := func(
		coordinate ManagedFragmentCoordinate,
		candidateComponent Component,
	) error {
		candidateSyntax, err := managedFragmentSyntax(coordinate.kind)
		if err != nil {
			return err
		}
		if _, known := knownComponents[candidateComponent]; !known {
			return fmt.Errorf("managed fragment component is invalid")
		}
		key := managedFragmentCoordinateKey(coordinate)
		if prior, exists := componentByCoordinate[key]; exists &&
			prior != candidateComponent {
			return fmt.Errorf(
				"managed fragment coordinate %s changes component from %s to %s",
				coordinate.selector,
				prior,
				candidateComponent,
			)
		}
		componentByCoordinate[key] = candidateComponent
		components[candidateComponent] = struct{}{}
		if carrierPath == "" {
			carrierPath = coordinate.carrierPath
			syntax = candidateSyntax
			edition = coordinate.mergeEdition
			return nil
		}
		if carrierPath != coordinate.carrierPath {
			return fmt.Errorf("managed fragment plan spans several carrier paths")
		}
		if syntax != candidateSyntax {
			return fmt.Errorf("managed fragment plan mixes carrier syntaxes")
		}
		if edition != coordinate.mergeEdition {
			return fmt.Errorf("managed fragment plan mixes merge editions")
		}
		return nil
	}
	for _, fragment := range desired {
		if err := acceptCoordinate(
			fragment.coordinate,
			fragment.component,
		); err != nil {
			return "", "", "", 0, ComponentSet{}, err
		}
		if createMode == 0 {
			createMode = fragment.createMode
			continue
		}
		if createMode != fragment.createMode {
			return "", "", "", 0, ComponentSet{}, fmt.Errorf(
				"managed fragments disagree on carrier create mode",
			)
		}
	}
	for _, record := range baseline {
		if err := acceptCoordinate(
			record.coordinate,
			record.component,
		); err != nil {
			return "", "", "", 0, ComponentSet{}, err
		}
	}
	for _, record := range legacy {
		if err := acceptCoordinate(
			record.coordinate,
			record.component,
		); err != nil {
			return "", "", "", 0, ComponentSet{}, err
		}
	}
	rawComponents := make([]string, 0, len(components))
	for component := range components {
		rawComponents = append(rawComponents, string(component))
	}
	componentSet, err := ParseComponentSet(rawComponents)
	if err != nil {
		return "", "", "", 0, ComponentSet{}, err
	}
	return carrierPath, syntax, edition, createMode, componentSet, nil
}

func managedFragmentSyntax(
	kind ManagedFragmentKind,
) (managedCarrierSyntax, error) {
	if kind == ManagedJSONObjectEntry ||
		kind == ManagedJSONArrayMember {
		return managedCarrierJSON, nil
	}
	if kind == ManagedTOMLTableFamily {
		return managedCarrierTOML, nil
	}
	if kind == ManagedTOMLTableSet {
		return managedCarrierTOML, nil
	}
	if kind == ManagedYAMLMappingEntry ||
		kind == ManagedYAMLSequenceMember {
		return managedCarrierYAML, nil
	}
	if kind == ManagedHTMLCommentSection {
		return managedCarrierText, nil
	}
	return "", fmt.Errorf("managed fragment kind is invalid")
}

func cloneManagedFragmentBaseline(
	baseline ManagedFragmentBaseline,
) ManagedFragmentBaseline {
	return ManagedFragmentBaseline{
		kind:    baseline.kind,
		records: cloneManagedFragmentRecords(baseline.records),
		basis:   baseline.basis,
	}
}

func cloneManagedFragmentLegacyRegistry(
	registry ManagedFragmentLegacyRegistry,
) ManagedFragmentLegacyRegistry {
	return ManagedFragmentLegacyRegistry{
		selected: registry.selected,
		records:  cloneManagedFragmentRecords(registry.records),
		basis:    registry.basis,
	}
}

func cloneManagedFragmentProbes(
	source []managedFragmentProbe,
) []managedFragmentProbe {
	result := make([]managedFragmentProbe, len(source))
	for index, probe := range source {
		result[index] = managedFragmentProbe{
			coordinate:       cloneManagedFragmentCoordinate(probe.coordinate),
			candidateDigests: slices.Clone(probe.candidateDigests),
		}
	}
	return result
}

type ManagedFragmentObservationKind string

const (
	ManagedFragmentObservedMissing ManagedFragmentObservationKind = "missing"
	ManagedFragmentObservedPresent ManagedFragmentObservationKind = "present"
)

type ManagedFragmentObservation struct {
	coordinate ManagedFragmentCoordinate
	kind       ManagedFragmentObservationKind
	digest     string
}

func (observation ManagedFragmentObservation) Coordinate() ManagedFragmentCoordinate {
	return cloneManagedFragmentCoordinate(observation.coordinate)
}

func (observation ManagedFragmentObservation) Kind() ManagedFragmentObservationKind {
	return observation.kind
}

func (observation ManagedFragmentObservation) Digest() string {
	return observation.digest
}

type ManagedCarrierObservation struct {
	carrier   ManagedCarrierInput
	fragments []ManagedFragmentObservation
}

func (observation ManagedCarrierObservation) Carrier() ManagedCarrierInput {
	return cloneManagedCarrierInput(observation.carrier)
}

func (observation ManagedCarrierObservation) Fragments() []ManagedFragmentObservation {
	return cloneManagedFragmentObservations(observation.fragments)
}

func cloneManagedFragmentObservations(
	source []ManagedFragmentObservation,
) []ManagedFragmentObservation {
	result := make([]ManagedFragmentObservation, len(source))
	for index, observation := range source {
		result[index] = ManagedFragmentObservation{
			coordinate: cloneManagedFragmentCoordinate(observation.coordinate),
			kind:       observation.kind,
			digest:     observation.digest,
		}
	}
	return result
}

func ObserveManagedCarrier(
	plan ManagedFragmentObservationPlan,
	input ManagedCarrierInput,
) (ManagedCarrierObservation, error) {
	if input.path != plan.carrierPath {
		return ManagedCarrierObservation{}, fmt.Errorf(
			"managed carrier observation belongs to another path",
		)
	}
	if input.kind == ManagedCarrierMissing {
		observations := make([]ManagedFragmentObservation, len(plan.probes))
		for index, probe := range plan.probes {
			observations[index] = ManagedFragmentObservation{
				coordinate: cloneManagedFragmentCoordinate(probe.coordinate),
				kind:       ManagedFragmentObservedMissing,
			}
		}
		return ManagedCarrierObservation{
			carrier:   cloneManagedCarrierInput(input),
			fragments: observations,
		}, nil
	}
	if input.kind != ManagedCarrierPresent ||
		!sha256DigestPattern.MatchString(input.digest) ||
		!validPermissionMode(input.mode) {
		return ManagedCarrierObservation{}, fmt.Errorf(
			"managed carrier input is invalid",
		)
	}
	var observations []ManagedFragmentObservation
	var err error
	if plan.syntax == managedCarrierJSON {
		observations, err = observeManagedJSON(
			plan.probes,
			input.content,
			plan.mergeEdition,
		)
	}
	if plan.syntax == managedCarrierTOML {
		observations, err = observeManagedTOML(plan.probes, input.content)
	}
	if plan.syntax == managedCarrierYAML {
		observations, err = observeManagedYAML(plan.probes, input.content)
	}
	if plan.syntax == managedCarrierText {
		observations, err = observeManagedText(plan.probes, input.content)
	}
	if err != nil {
		return ManagedCarrierObservation{}, err
	}
	if observations == nil {
		return ManagedCarrierObservation{}, fmt.Errorf(
			"managed carrier syntax is unsupported",
		)
	}
	return ManagedCarrierObservation{
		carrier:   cloneManagedCarrierInput(input),
		fragments: observations,
	}, nil
}

func observeManagedJSON(
	probes []managedFragmentProbe,
	raw []byte,
	mergeEdition string,
) ([]ManagedFragmentObservation, error) {
	value, err := decodeManagedJSON(raw, mergeEdition)
	if err != nil {
		return nil, fmt.Errorf("observe managed JSON carrier: %w", err)
	}
	root, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("managed JSON carrier root must be an object")
	}
	observations := make([]ManagedFragmentObservation, len(probes))
	for index, probe := range probes {
		observation, err := observeManagedJSONProbe(root, probe)
		if err != nil {
			return nil, err
		}
		observations[index] = observation
	}
	return observations, nil
}

func observeManagedJSONProbe(
	root map[string]any,
	probe managedFragmentProbe,
) (ManagedFragmentObservation, error) {
	if probe.coordinate.kind == ManagedJSONObjectEntry {
		value, found, err := lookupJSONObjectPath(
			root,
			probe.coordinate.jsonPath,
		)
		if err != nil {
			return ManagedFragmentObservation{}, err
		}
		if !found {
			return missingManagedFragmentObservation(probe.coordinate), nil
		}
		canonical, err := marshalCanonicalJSONValue(value)
		if err != nil {
			return ManagedFragmentObservation{}, err
		}
		return presentManagedFragmentObservation(
			probe.coordinate,
			managedFragmentDigest(canonical),
		), nil
	}
	if probe.coordinate.kind == ManagedJSONArrayMember {
		value, found, err := lookupJSONObjectPath(
			root,
			probe.coordinate.jsonPath,
		)
		if err != nil {
			return ManagedFragmentObservation{}, err
		}
		if !found {
			return missingManagedFragmentObservation(probe.coordinate), nil
		}
		array, ok := value.([]any)
		if !ok {
			return ManagedFragmentObservation{}, fmt.Errorf(
				"managed JSON selector %s must name an array",
				probe.coordinate.selector,
			)
		}
		var digest string
		if probe.coordinate.mergeEdition ==
			ManagedJSONArraySourceMergeEdition {
			digest, found, err = findKnownJSONArraySourceMember(
				array,
				probe.candidateDigests,
				probe.coordinate,
			)
		} else {
			digest, found, err = findKnownJSONArrayMember(
				array,
				probe.candidateDigests,
				probe.coordinate,
			)
		}
		if err != nil {
			return ManagedFragmentObservation{}, err
		}
		if !found {
			return missingManagedFragmentObservation(probe.coordinate), nil
		}
		return presentManagedFragmentObservation(
			probe.coordinate,
			digest,
		), nil
	}
	return ManagedFragmentObservation{}, fmt.Errorf(
		"managed JSON probe kind is invalid",
	)
}

func lookupJSONObjectPath(
	root map[string]any,
	path []string,
) (any, bool, error) {
	current := root
	for index, token := range path {
		value, found := current[token]
		if !found {
			return nil, false, nil
		}
		if index == len(path)-1 {
			return value, true, nil
		}
		next, ok := value.(map[string]any)
		if !ok {
			return nil, false, fmt.Errorf(
				"managed JSON selector crosses non-object token %q",
				token,
			)
		}
		current = next
	}
	return nil, false, fmt.Errorf("managed JSON selector is empty")
}

// ExtractJSONObjectEntry is the adapter-facing read port for recognizing one
// exact historical object-entry representation in a shared JSON carrier.
// Parsing retains the selected merge edition's syntax rules and the domain's
// duplicate-key rejection.
func ExtractJSONObjectEntry(
	raw []byte,
	selector []string,
	mergeEdition string,
) ([]byte, bool, error) {
	path, _, err := canonicalJSONPointer(selector)
	if err != nil {
		return nil, false, err
	}
	value, err := decodeManagedJSON(raw, mergeEdition)
	if err != nil {
		return nil, false, fmt.Errorf(
			"extract managed JSON object entry: %w",
			err,
		)
	}
	root, ok := value.(map[string]any)
	if !ok {
		return nil, false, fmt.Errorf(
			"managed JSON carrier root must be an object",
		)
	}
	selected, found, err := lookupJSONObjectPath(root, path)
	if err != nil || !found {
		return nil, found, err
	}
	canonical, err := marshalCanonicalJSONValue(selected)
	if err != nil {
		return nil, false, err
	}
	return canonical, true, nil
}

func findKnownJSONArrayMember(
	array []any,
	candidates []string,
	coordinate ManagedFragmentCoordinate,
) (string, bool, error) {
	candidateSet := make(map[string]struct{}, len(candidates))
	for _, digest := range candidates {
		candidateSet[digest] = struct{}{}
	}
	matches := make([]string, 0, 1)
	for _, value := range array {
		canonical, err := marshalCanonicalJSONValue(value)
		if err != nil {
			return "", false, err
		}
		digest := managedFragmentDigest(canonical)
		if _, known := candidateSet[digest]; known {
			matches = append(matches, digest)
		}
	}
	if len(matches) > 1 {
		return "", false, fmt.Errorf(
			"managed JSON array member %s/%s is ambiguous",
			coordinate.selector,
			coordinate.memberID,
		)
	}
	if len(matches) == 0 {
		return "", false, nil
	}
	return matches[0], true, nil
}

func findKnownJSONArraySourceMember(
	array []any,
	candidates []string,
	coordinate ManagedFragmentCoordinate,
) (string, bool, error) {
	candidateSet := make(map[string]struct{}, len(candidates))
	for _, digest := range candidates {
		candidateSet[digest] = struct{}{}
	}
	matches := make([]string, 0, 1)
	for _, value := range array {
		source, found := managedJSONArraySource(value)
		if !found {
			continue
		}
		canonical, err := marshalCanonicalJSONValue(source)
		if err != nil {
			return "", false, err
		}
		digest := managedFragmentDigest(canonical)
		if _, known := candidateSet[digest]; known {
			matches = append(matches, digest)
		}
	}
	if len(matches) > 1 {
		return "", false, fmt.Errorf(
			"managed JSON array source member %s/%s is ambiguous",
			coordinate.selector,
			coordinate.memberID,
		)
	}
	if len(matches) == 0 {
		return "", false, nil
	}
	return matches[0], true, nil
}

func managedJSONArraySource(
	value any,
) (string, bool) {
	if source, ok := value.(string); ok {
		return source, true
	}
	object, ok := value.(map[string]any)
	if !ok {
		return "", false
	}
	source, ok := object["source"].(string)
	return source, ok
}

func observeManagedTOML(
	probes []managedFragmentProbe,
	raw []byte,
) ([]ManagedFragmentObservation, error) {
	sections, err := scanTOMLSections(raw)
	if err != nil {
		return nil, fmt.Errorf("observe managed TOML carrier: %w", err)
	}
	observations := make([]ManagedFragmentObservation, len(probes))
	for index, probe := range probes {
		if probe.coordinate.kind != ManagedTOMLTableFamily &&
			probe.coordinate.kind != ManagedTOMLTableSet {
			return nil, fmt.Errorf("managed TOML probe kind is invalid")
		}
		var content []byte
		var found bool
		if probe.coordinate.kind == ManagedTOMLTableFamily {
			content, found = extractTOMLTableFamily(
				raw,
				sections,
				probe.coordinate.tomlPrefix,
			)
		}
		if probe.coordinate.kind == ManagedTOMLTableSet {
			content, found, err = extractTOMLTableSet(
				raw,
				sections,
				probe.coordinate.tomlTables,
			)
			if err != nil {
				return nil, err
			}
		}
		if !found {
			observations[index] = missingManagedFragmentObservation(
				probe.coordinate,
			)
			continue
		}
		observations[index] = presentManagedFragmentObservation(
			probe.coordinate,
			managedFragmentDigest(content),
		)
	}
	return observations, nil
}

func missingManagedFragmentObservation(
	coordinate ManagedFragmentCoordinate,
) ManagedFragmentObservation {
	return ManagedFragmentObservation{
		coordinate: cloneManagedFragmentCoordinate(coordinate),
		kind:       ManagedFragmentObservedMissing,
	}
}

func presentManagedFragmentObservation(
	coordinate ManagedFragmentCoordinate,
	digest string,
) ManagedFragmentObservation {
	return ManagedFragmentObservation{
		coordinate: cloneManagedFragmentCoordinate(coordinate),
		kind:       ManagedFragmentObservedPresent,
		digest:     digest,
	}
}

type ManagedFragmentCurrentnessKind string

const (
	ManagedFragmentCurrentOwned         ManagedFragmentCurrentnessKind = "current_owned"
	ManagedFragmentOutdatedOwned        ManagedFragmentCurrentnessKind = "outdated_owned"
	ManagedFragmentLocallyModifiedOwned ManagedFragmentCurrentnessKind = "locally_modified_owned"
	ManagedFragmentKnownLegacyExact     ManagedFragmentCurrentnessKind = "known_legacy_exact"
	ManagedFragmentForeign              ManagedFragmentCurrentnessKind = "foreign"
	ManagedFragmentOrphanedOwned        ManagedFragmentCurrentnessKind = "orphaned_owned"
	ManagedFragmentMissingOwned         ManagedFragmentCurrentnessKind = "missing_owned"
)

type ManagedFragmentCurrentness struct {
	coordinate     ManagedFragmentCoordinate
	component      Component
	kind           ManagedFragmentCurrentnessKind
	observedDigest string
	manifestDigest string
	desiredDigest  string
	hasDesired     bool
	basis          OwnershipBasis
}

func (currentness ManagedFragmentCurrentness) Coordinate() ManagedFragmentCoordinate {
	return cloneManagedFragmentCoordinate(currentness.coordinate)
}

func (currentness ManagedFragmentCurrentness) Component() Component {
	return currentness.component
}

func (currentness ManagedFragmentCurrentness) Kind() ManagedFragmentCurrentnessKind {
	return currentness.kind
}

func (currentness ManagedFragmentCurrentness) ObservedDigest() string {
	return currentness.observedDigest
}

func (currentness ManagedFragmentCurrentness) ManifestDigest() string {
	return currentness.manifestDigest
}

func (currentness ManagedFragmentCurrentness) DesiredDigest() string {
	return currentness.desiredDigest
}

func (currentness ManagedFragmentCurrentness) OwnershipBasis() OwnershipBasis {
	return currentness.basis
}

type ManagedFragmentVacantTarget struct {
	fragment ManagedFragment
}

func (target ManagedFragmentVacantTarget) Coordinate() ManagedFragmentCoordinate {
	return target.fragment.Coordinate()
}

func (target ManagedFragmentVacantTarget) Desired() ManagedFragment {
	return cloneManagedFragment(target.fragment)
}

type ManagedCarrierCurrentness struct {
	plan        ManagedFragmentObservationPlan
	observation ManagedCarrierObservation
	states      []ManagedFragmentCurrentness
	vacant      []ManagedFragmentVacantTarget
}

func (currentness ManagedCarrierCurrentness) CarrierPath() string {
	return currentness.plan.carrierPath
}

func (currentness ManagedCarrierCurrentness) Components() ComponentSet {
	return currentness.plan.Components()
}

func (currentness ManagedCarrierCurrentness) Component() Component {
	component, _ := currentness.plan.components.single()
	return component
}

func (currentness ManagedCarrierCurrentness) States() []ManagedFragmentCurrentness {
	return cloneManagedFragmentCurrentness(currentness.states)
}

func (currentness ManagedCarrierCurrentness) VacantTargets() []ManagedFragmentVacantTarget {
	result := make([]ManagedFragmentVacantTarget, len(currentness.vacant))
	for index, target := range currentness.vacant {
		result[index] = ManagedFragmentVacantTarget{
			fragment: cloneManagedFragment(target.fragment),
		}
	}
	return result
}

func cloneManagedFragmentCurrentness(
	source []ManagedFragmentCurrentness,
) []ManagedFragmentCurrentness {
	result := make([]ManagedFragmentCurrentness, len(source))
	for index, state := range source {
		result[index] = ManagedFragmentCurrentness{
			coordinate:     cloneManagedFragmentCoordinate(state.coordinate),
			component:      state.component,
			kind:           state.kind,
			observedDigest: state.observedDigest,
			manifestDigest: state.manifestDigest,
			desiredDigest:  state.desiredDigest,
			hasDesired:     state.hasDesired,
			basis:          state.basis,
		}
	}
	return result
}

func ClassifyManagedFragmentCurrentness(
	plan ManagedFragmentObservationPlan,
	observation ManagedCarrierObservation,
) (ManagedCarrierCurrentness, error) {
	if observation.carrier.path != plan.carrierPath {
		return ManagedCarrierCurrentness{}, fmt.Errorf(
			"managed fragment currentness observation belongs to another carrier",
		)
	}
	observedByKey, err := managedFragmentObservationsByKey(
		observation.fragments,
	)
	if err != nil {
		return ManagedCarrierCurrentness{}, err
	}
	if len(observedByKey) != len(plan.probes) {
		return ManagedCarrierCurrentness{}, fmt.Errorf(
			"managed fragment observation coverage is incomplete",
		)
	}
	desiredByKey := managedFragmentsByKey(plan.desired)
	manifestByKey := managedFragmentRecordsByKey(plan.baseline.records)
	legacyByKey := managedFragmentRecordSetsByKey(plan.legacy.records)
	states := make([]ManagedFragmentCurrentness, 0, len(plan.probes))
	vacant := make([]ManagedFragmentVacantTarget, 0, len(plan.desired))
	for _, probe := range plan.probes {
		key := managedFragmentCoordinateKey(probe.coordinate)
		observed, exists := observedByKey[key]
		if !exists {
			return ManagedCarrierCurrentness{}, fmt.Errorf(
				"managed fragment observation lacks %s",
				probe.coordinate.selector,
			)
		}
		desired, hasDesired := desiredByKey[key]
		manifest, manifestOwned := manifestByKey[key]
		legacy := legacyByKey[key]
		state, target, emitted, err := classifyManagedFragment(
			probe.coordinate,
			observed,
			desired,
			hasDesired,
			manifest,
			manifestOwned,
			legacy,
			plan.baseline.basis,
			plan.legacy.basis,
		)
		if err != nil {
			return ManagedCarrierCurrentness{}, err
		}
		if emitted == managedFragmentStateEmitted {
			states = append(states, state)
		}
		if emitted == managedFragmentVacancyEmitted {
			vacant = append(vacant, target)
		}
	}
	return ManagedCarrierCurrentness{
		plan:        cloneManagedFragmentObservationPlan(plan),
		observation: cloneManagedCarrierObservation(observation),
		states:      states,
		vacant:      vacant,
	}, nil
}

type managedFragmentClassificationEmission string

const (
	managedFragmentNoEmission     managedFragmentClassificationEmission = "none"
	managedFragmentStateEmitted   managedFragmentClassificationEmission = "state"
	managedFragmentVacancyEmitted managedFragmentClassificationEmission = "vacancy"
)

func classifyManagedFragment(
	coordinate ManagedFragmentCoordinate,
	observed ManagedFragmentObservation,
	desired ManagedFragment,
	hasDesired bool,
	manifest ManagedFragmentRecord,
	manifestOwned bool,
	legacy []ManagedFragmentRecord,
	manifestBasis OwnershipBasis,
	legacyBasis OwnershipBasis,
) (
	ManagedFragmentCurrentness,
	ManagedFragmentVacantTarget,
	managedFragmentClassificationEmission,
	error,
) {
	component, err := managedFragmentComponent(
		desired,
		hasDesired,
		manifest,
		manifestOwned,
		legacy,
	)
	if err != nil {
		return ManagedFragmentCurrentness{}, ManagedFragmentVacantTarget{},
			managedFragmentNoEmission, err
	}
	if manifestOwned {
		state := classifyManifestOwnedManagedFragment(
			coordinate,
			component,
			observed,
			desired,
			hasDesired,
			manifest,
			manifestBasis,
		)
		return state, ManagedFragmentVacantTarget{}, managedFragmentStateEmitted, nil
	}
	if observed.kind == ManagedFragmentObservedPresent {
		if coordinate.kind == ManagedHTMLCommentSection {
			return classifyMarkerOwnedManagedFragment(
				coordinate,
				component,
				observed,
				desired,
				hasDesired,
			), ManagedFragmentVacantTarget{}, managedFragmentStateEmitted, nil
		}
		kind := ManagedFragmentForeign
		basis := OwnershipBasis{}
		if managedFragmentRecordsContainDigest(legacy, observed.digest) {
			kind = ManagedFragmentKnownLegacyExact
			basis = legacyBasis
		}
		return ManagedFragmentCurrentness{
			coordinate:     cloneManagedFragmentCoordinate(coordinate),
			component:      component,
			kind:           kind,
			observedDigest: observed.digest,
			desiredDigest:  managedDesiredDigest(desired, hasDesired),
			hasDesired:     hasDesired,
			basis:          basis,
		}, ManagedFragmentVacantTarget{}, managedFragmentStateEmitted, nil
	}
	if hasDesired {
		return ManagedFragmentCurrentness{}, ManagedFragmentVacantTarget{
			fragment: cloneManagedFragment(desired),
		}, managedFragmentVacancyEmitted, nil
	}
	return ManagedFragmentCurrentness{}, ManagedFragmentVacantTarget{},
		managedFragmentNoEmission, nil
}

func managedFragmentComponent(
	desired ManagedFragment,
	hasDesired bool,
	manifest ManagedFragmentRecord,
	manifestOwned bool,
	legacy []ManagedFragmentRecord,
) (Component, error) {
	candidates := make([]Component, 0, 2+len(legacy))
	if hasDesired {
		candidates = append(candidates, desired.component)
	}
	if manifestOwned {
		candidates = append(candidates, manifest.component)
	}
	for _, record := range legacy {
		candidates = append(candidates, record.component)
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf(
			"managed fragment coordinate has no component provenance",
		)
	}
	component := candidates[0]
	for _, candidate := range candidates {
		if candidate != component {
			return "", fmt.Errorf(
				"managed fragment coordinate has conflicting component provenance",
			)
		}
	}
	return component, nil
}

func classifyManifestOwnedManagedFragment(
	coordinate ManagedFragmentCoordinate,
	component Component,
	observed ManagedFragmentObservation,
	desired ManagedFragment,
	hasDesired bool,
	manifest ManagedFragmentRecord,
	basis OwnershipBasis,
) ManagedFragmentCurrentness {
	state := ManagedFragmentCurrentness{
		coordinate:     cloneManagedFragmentCoordinate(coordinate),
		component:      component,
		manifestDigest: manifest.digest,
		desiredDigest:  managedDesiredDigest(desired, hasDesired),
		hasDesired:     hasDesired,
		basis:          basis,
	}
	if observed.kind == ManagedFragmentObservedMissing {
		state.kind = ManagedFragmentMissingOwned
		return state
	}
	state.observedDigest = observed.digest
	if coordinate.kind == ManagedHTMLCommentSection {
		state.kind = markerOwnedManagedFragmentKind(
			observed.digest,
			desired,
			hasDesired,
		)
		return state
	}
	if observed.digest != manifest.digest {
		state.kind = ManagedFragmentLocallyModifiedOwned
		return state
	}
	if !hasDesired {
		state.kind = ManagedFragmentOrphanedOwned
		return state
	}
	if observed.digest == desired.digest {
		state.kind = ManagedFragmentCurrentOwned
		return state
	}
	state.kind = ManagedFragmentOutdatedOwned
	return state
}

func classifyMarkerOwnedManagedFragment(
	coordinate ManagedFragmentCoordinate,
	component Component,
	observed ManagedFragmentObservation,
	desired ManagedFragment,
	hasDesired bool,
) ManagedFragmentCurrentness {
	return ManagedFragmentCurrentness{
		coordinate: cloneManagedFragmentCoordinate(coordinate),
		component:  component,
		kind: markerOwnedManagedFragmentKind(
			observed.digest,
			desired,
			hasDesired,
		),
		observedDigest: observed.digest,
		desiredDigest:  managedDesiredDigest(desired, hasDesired),
		hasDesired:     hasDesired,
	}
}

func markerOwnedManagedFragmentKind(
	observedDigest string,
	desired ManagedFragment,
	hasDesired bool,
) ManagedFragmentCurrentnessKind {
	if !hasDesired {
		return ManagedFragmentOrphanedOwned
	}
	if observedDigest == desired.digest {
		return ManagedFragmentCurrentOwned
	}
	return ManagedFragmentOutdatedOwned
}

func managedDesiredDigest(
	desired ManagedFragment,
	hasDesired bool,
) string {
	if !hasDesired {
		return ""
	}
	return desired.digest
}

func managedFragmentObservationsByKey(
	observations []ManagedFragmentObservation,
) (map[string]ManagedFragmentObservation, error) {
	result := make(map[string]ManagedFragmentObservation, len(observations))
	for _, observation := range observations {
		if !observation.coordinate.valid() {
			return nil, fmt.Errorf("managed fragment observation coordinate is invalid")
		}
		if observation.kind == ManagedFragmentObservedPresent &&
			!sha256DigestPattern.MatchString(observation.digest) {
			return nil, fmt.Errorf("managed fragment observation digest is invalid")
		}
		if observation.kind != ManagedFragmentObservedPresent &&
			observation.kind != ManagedFragmentObservedMissing {
			return nil, fmt.Errorf("managed fragment observation kind is invalid")
		}
		key := managedFragmentCoordinateKey(observation.coordinate)
		if _, duplicate := result[key]; duplicate {
			return nil, fmt.Errorf("managed fragment observation is repeated")
		}
		result[key] = observation
	}
	return result, nil
}

func managedFragmentsByKey(
	fragments []ManagedFragment,
) map[string]ManagedFragment {
	result := make(map[string]ManagedFragment, len(fragments))
	for _, fragment := range fragments {
		key := managedFragmentCoordinateKey(fragment.coordinate)
		result[key] = cloneManagedFragment(fragment)
	}
	return result
}

func managedFragmentRecordsByKey(
	records []ManagedFragmentRecord,
) map[string]ManagedFragmentRecord {
	result := make(map[string]ManagedFragmentRecord, len(records))
	for _, record := range records {
		key := managedFragmentCoordinateKey(record.coordinate)
		result[key] = cloneManagedFragmentRecord(record)
	}
	return result
}

func managedFragmentRecordSetsByKey(
	records []ManagedFragmentRecord,
) map[string][]ManagedFragmentRecord {
	result := make(map[string][]ManagedFragmentRecord)
	for _, record := range records {
		key := managedFragmentCoordinateKey(record.coordinate)
		result[key] = append(
			result[key],
			cloneManagedFragmentRecord(record),
		)
	}
	return result
}

func managedFragmentRecordsContainDigest(
	records []ManagedFragmentRecord,
	digest string,
) bool {
	for _, record := range records {
		if record.digest == digest {
			return true
		}
	}
	return false
}

func cloneManagedFragmentObservationPlan(
	plan ManagedFragmentObservationPlan,
) ManagedFragmentObservationPlan {
	return ManagedFragmentObservationPlan{
		carrierPath:  plan.carrierPath,
		syntax:       plan.syntax,
		mergeEdition: plan.mergeEdition,
		createMode:   plan.createMode,
		components:   ComponentSet{values: plan.components.Values()},
		retainedManagedFragmentComponents: slices.Clone(
			plan.retainedManagedFragmentComponents,
		),
		desired:  cloneManagedFragments(plan.desired),
		baseline: cloneManagedFragmentBaseline(plan.baseline),
		legacy:   cloneManagedFragmentLegacyRegistry(plan.legacy),
		probes:   cloneManagedFragmentProbes(plan.probes),
	}
}

func materializeRetainedManagedFragments(
	plan ManagedFragmentObservationPlan,
	input ManagedCarrierInput,
) (ManagedFragmentObservationPlan, error) {
	if len(plan.retainedManagedFragmentComponents) == 0 {
		return cloneManagedFragmentObservationPlan(plan), nil
	}
	if input.path != plan.carrierPath {
		return ManagedFragmentObservationPlan{}, fmt.Errorf(
			"managed fragment retention input belongs to another carrier",
		)
	}
	desired := cloneManagedFragments(plan.desired)
	desiredByKey := managedFragmentsByKey(desired)
	for _, record := range plan.baseline.records {
		if !slices.Contains(
			plan.retainedManagedFragmentComponents,
			record.component,
		) {
			continue
		}
		key := managedFragmentCoordinateKey(record.coordinate)
		if _, alreadyDesired := desiredByKey[key]; alreadyDesired {
			continue
		}
		if record.coordinate.kind != ManagedHTMLCommentSection {
			return ManagedFragmentObservationPlan{}, fmt.Errorf(
				"installed managed-fragment retention does not support kind %s",
				record.coordinate.kind,
			)
		}
		createMode := plan.createMode
		if createMode == 0 && input.kind == ManagedCarrierPresent {
			createMode = input.mode
		}
		fragment, found, err := retainedHTMLCommentSectionFragment(
			record,
			input,
			createMode,
		)
		if err != nil {
			return ManagedFragmentObservationPlan{}, err
		}
		if !found {
			continue
		}
		desired = append(desired, fragment)
		desiredByKey[key] = fragment
	}
	next, err := BuildManagedFragmentObservationPlan(
		desired,
		plan.baseline,
		plan.legacy,
	)
	if err != nil {
		return ManagedFragmentObservationPlan{}, err
	}
	next.retainedManagedFragmentComponents = slices.Clone(
		plan.retainedManagedFragmentComponents,
	)
	return next, nil
}

func cloneManagedCarrierObservation(
	observation ManagedCarrierObservation,
) ManagedCarrierObservation {
	return ManagedCarrierObservation{
		carrier:   cloneManagedCarrierInput(observation.carrier),
		fragments: cloneManagedFragmentObservations(observation.fragments),
	}
}

type ManagedCarrierReadiness string

const (
	ManagedCarrierReady   ManagedCarrierReadiness = "ready"
	ManagedCarrierBlocked ManagedCarrierReadiness = "blocked"
)

type ManagedFragmentEffectKind string

const (
	ManagedFragmentPreserve           ManagedFragmentEffectKind = "preserve"
	ManagedFragmentAdoptLegacy        ManagedFragmentEffectKind = "adopt_known_legacy"
	ManagedFragmentCreate             ManagedFragmentEffectKind = "create"
	ManagedFragmentReplace            ManagedFragmentEffectKind = "replace"
	ManagedFragmentRemove             ManagedFragmentEffectKind = "remove"
	ManagedFragmentForgetMissingOwned ManagedFragmentEffectKind = "forget_missing_owned"
)

type ManagedFragmentEffect struct {
	kind           ManagedFragmentEffectKind
	coordinate     ManagedFragmentCoordinate
	component      Component
	expectedKind   ManagedFragmentObservationKind
	expectedDigest string
	desired        ManagedFragment
	hasDesired     bool
}

func (effect ManagedFragmentEffect) Kind() ManagedFragmentEffectKind {
	return effect.kind
}

func (effect ManagedFragmentEffect) Coordinate() ManagedFragmentCoordinate {
	return cloneManagedFragmentCoordinate(effect.coordinate)
}

func (effect ManagedFragmentEffect) Component() Component {
	return effect.component
}

func (effect ManagedFragmentEffect) ExpectedKind() ManagedFragmentObservationKind {
	return effect.expectedKind
}

func (effect ManagedFragmentEffect) ExpectedDigest() string {
	return effect.expectedDigest
}

func (effect ManagedFragmentEffect) Desired() (ManagedFragment, bool) {
	return cloneManagedFragment(effect.desired), effect.hasDesired
}

type ManagedFragmentConflictKind string

const (
	ManagedFragmentConflictLocallyModified ManagedFragmentConflictKind = "locally_modified_owned"
	ManagedFragmentConflictForeign         ManagedFragmentConflictKind = "foreign"
)

type ManagedFragmentConflict struct {
	coordinate ManagedFragmentCoordinate
	component  Component
	kind       ManagedFragmentConflictKind
	reason     string
	basis      OwnershipBasis
}

func (conflict ManagedFragmentConflict) Coordinate() ManagedFragmentCoordinate {
	return cloneManagedFragmentCoordinate(conflict.coordinate)
}

func (conflict ManagedFragmentConflict) Component() Component {
	return conflict.component
}

func (conflict ManagedFragmentConflict) Kind() ManagedFragmentConflictKind {
	return conflict.kind
}

func (conflict ManagedFragmentConflict) Reason() string {
	return conflict.reason
}

func (conflict ManagedFragmentConflict) OwnershipBasis() OwnershipBasis {
	return conflict.basis
}

type ManagedCarrierReconciliation struct {
	readiness    ManagedCarrierReadiness
	observation  ManagedCarrierObservation
	syntax       managedCarrierSyntax
	mergeEdition string
	createMode   fs.FileMode
	components   ComponentSet
	basis        OwnershipBasis
	effects      []ManagedFragmentEffect
	conflicts    []ManagedFragmentConflict
}

func CompileManagedCarrierReconciliation(
	currentness ManagedCarrierCurrentness,
) (ManagedCarrierReconciliation, error) {
	desiredByKey := managedFragmentsByKey(currentness.plan.desired)
	effects := make([]ManagedFragmentEffect, 0, len(currentness.states)+len(currentness.vacant))
	conflicts := make([]ManagedFragmentConflict, 0)
	seen := make(map[string]struct{})
	for _, state := range currentness.states {
		key := managedFragmentCoordinateKey(state.coordinate)
		desired, hasDesired := desiredByKey[key]
		effect, conflict, emitted, err := compileManagedFragmentState(
			state,
			desired,
			hasDesired,
		)
		if err != nil {
			return ManagedCarrierReconciliation{}, err
		}
		if emitted == managedFragmentEffectEmitted {
			effects = append(effects, effect)
			seen[key] = struct{}{}
		}
		if emitted == managedFragmentConflictEmitted {
			conflicts = append(conflicts, conflict)
			seen[key] = struct{}{}
		}
	}
	for _, target := range currentness.vacant {
		key := managedFragmentCoordinateKey(target.fragment.coordinate)
		if _, duplicate := seen[key]; duplicate {
			return ManagedCarrierReconciliation{}, fmt.Errorf(
				"managed fragment reconciliation repeats a vacant target",
			)
		}
		effects = append(effects, ManagedFragmentEffect{
			kind:         ManagedFragmentCreate,
			coordinate:   target.fragment.Coordinate(),
			component:    target.fragment.component,
			expectedKind: ManagedFragmentObservedMissing,
			desired:      cloneManagedFragment(target.fragment),
			hasDesired:   true,
		})
		seen[key] = struct{}{}
	}
	sort.Slice(effects, func(left int, right int) bool {
		return managedFragmentCoordinateKey(effects[left].coordinate) <
			managedFragmentCoordinateKey(effects[right].coordinate)
	})
	sort.Slice(conflicts, func(left int, right int) bool {
		return managedFragmentCoordinateKey(conflicts[left].coordinate) <
			managedFragmentCoordinateKey(conflicts[right].coordinate)
	})
	readiness := ManagedCarrierReady
	if len(conflicts) > 0 {
		readiness = ManagedCarrierBlocked
	}
	return ManagedCarrierReconciliation{
		readiness:    readiness,
		observation:  cloneManagedCarrierObservation(currentness.observation),
		syntax:       currentness.plan.syntax,
		mergeEdition: currentness.plan.mergeEdition,
		createMode:   currentness.plan.createMode,
		components:   currentness.plan.Components(),
		basis:        currentness.plan.baseline.basis,
		effects:      cloneManagedFragmentEffects(effects),
		conflicts:    cloneManagedFragmentConflicts(conflicts),
	}, nil
}

type managedFragmentCompileEmission string

const (
	managedFragmentEffectEmitted   managedFragmentCompileEmission = "effect"
	managedFragmentConflictEmitted managedFragmentCompileEmission = "conflict"
)

func compileManagedFragmentState(
	state ManagedFragmentCurrentness,
	desired ManagedFragment,
	hasDesired bool,
) (
	ManagedFragmentEffect,
	ManagedFragmentConflict,
	managedFragmentCompileEmission,
	error,
) {
	base := ManagedFragmentEffect{
		coordinate:     state.Coordinate(),
		component:      state.component,
		expectedDigest: state.observedDigest,
		desired:        cloneManagedFragment(desired),
		hasDesired:     hasDesired,
	}
	if state.observedDigest == "" {
		base.expectedKind = ManagedFragmentObservedMissing
	}
	if state.observedDigest != "" {
		base.expectedKind = ManagedFragmentObservedPresent
	}
	switch state.kind {
	case ManagedFragmentCurrentOwned:
		base.kind = ManagedFragmentPreserve
		return base, ManagedFragmentConflict{}, managedFragmentEffectEmitted, nil
	case ManagedFragmentOutdatedOwned:
		base.kind = ManagedFragmentReplace
		return base, ManagedFragmentConflict{}, managedFragmentEffectEmitted, nil
	case ManagedFragmentKnownLegacyExact:
		base.kind = ManagedFragmentAdoptLegacy
		if !hasDesired {
			base.kind = ManagedFragmentRemove
		}
		if hasDesired && state.observedDigest != desired.digest {
			base.kind = ManagedFragmentReplace
		}
		return base, ManagedFragmentConflict{}, managedFragmentEffectEmitted, nil
	case ManagedFragmentOrphanedOwned:
		base.kind = ManagedFragmentRemove
		return base, ManagedFragmentConflict{}, managedFragmentEffectEmitted, nil
	case ManagedFragmentMissingOwned:
		base.kind = ManagedFragmentForgetMissingOwned
		if hasDesired {
			base.kind = ManagedFragmentCreate
		}
		return base, ManagedFragmentConflict{}, managedFragmentEffectEmitted, nil
	case ManagedFragmentLocallyModifiedOwned:
		return ManagedFragmentEffect{}, ManagedFragmentConflict{
			coordinate: state.Coordinate(),
			component:  state.component,
			kind:       ManagedFragmentConflictLocallyModified,
			reason:     "manifest-owned managed fragment differs from its recorded digest; preserve the shared carrier until an explicit keep, export, or replace operation",
			basis:      state.basis,
		}, managedFragmentConflictEmitted, nil
	case ManagedFragmentForeign:
		return ManagedFragmentEffect{}, ManagedFragmentConflict{
			coordinate: state.Coordinate(),
			component:  state.component,
			kind:       ManagedFragmentConflictForeign,
			reason:     "unowned managed-fragment coordinate collides with the desired host projection; preserve the shared carrier",
		}, managedFragmentConflictEmitted, nil
	default:
		return ManagedFragmentEffect{}, ManagedFragmentConflict{}, "", fmt.Errorf(
			"managed fragment currentness kind is invalid",
		)
	}
}

func cloneManagedFragmentEffects(
	source []ManagedFragmentEffect,
) []ManagedFragmentEffect {
	result := make([]ManagedFragmentEffect, len(source))
	for index, effect := range source {
		result[index] = ManagedFragmentEffect{
			kind:           effect.kind,
			coordinate:     cloneManagedFragmentCoordinate(effect.coordinate),
			component:      effect.component,
			expectedKind:   effect.expectedKind,
			expectedDigest: effect.expectedDigest,
			desired:        cloneManagedFragment(effect.desired),
			hasDesired:     effect.hasDesired,
		}
	}
	return result
}

func cloneManagedFragmentConflicts(
	source []ManagedFragmentConflict,
) []ManagedFragmentConflict {
	result := make([]ManagedFragmentConflict, len(source))
	for index, conflict := range source {
		result[index] = ManagedFragmentConflict{
			coordinate: cloneManagedFragmentCoordinate(conflict.coordinate),
			component:  conflict.component,
			kind:       conflict.kind,
			reason:     conflict.reason,
			basis:      conflict.basis,
		}
	}
	return result
}

func (plan ManagedCarrierReconciliation) Readiness() ManagedCarrierReadiness {
	return plan.readiness
}

func (plan ManagedCarrierReconciliation) Effects() []ManagedFragmentEffect {
	return cloneManagedFragmentEffects(plan.effects)
}

func (plan ManagedCarrierReconciliation) Conflicts() []ManagedFragmentConflict {
	return cloneManagedFragmentConflicts(plan.conflicts)
}

func (plan ManagedCarrierReconciliation) Components() ComponentSet {
	return ComponentSet{values: plan.components.Values()}
}

func (plan ManagedCarrierReconciliation) Component() Component {
	component, _ := plan.components.single()
	return component
}

func (plan ManagedCarrierReconciliation) ManifestBasis() OwnershipBasis {
	return plan.basis
}

func cloneManagedCarrierReconciliation(
	plan ManagedCarrierReconciliation,
) ManagedCarrierReconciliation {
	return ManagedCarrierReconciliation{
		readiness:    plan.readiness,
		observation:  cloneManagedCarrierObservation(plan.observation),
		syntax:       plan.syntax,
		mergeEdition: plan.mergeEdition,
		createMode:   plan.createMode,
		components:   plan.Components(),
		basis:        plan.basis,
		effects:      cloneManagedFragmentEffects(plan.effects),
		conflicts:    cloneManagedFragmentConflicts(plan.conflicts),
	}
}

type ManagedCarrierResultKind string

const (
	ManagedCarrierUnchanged ManagedCarrierResultKind = "unchanged"
	ManagedCarrierWrite     ManagedCarrierResultKind = "write"
	ManagedCarrierAbsent    ManagedCarrierResultKind = "absent"
)

type ManagedCarrierMutationResult struct {
	kind    ManagedCarrierResultKind
	path    string
	content []byte
	digest  string
	mode    fs.FileMode
	changed bool
}

type ManagedCarrierInstallPlan struct {
	reconciliation ManagedCarrierReconciliation
	result         ManagedCarrierMutationResult
	hasResult      bool
}

func compileManagedCarrierInstallPlan(
	reconciliation ManagedCarrierReconciliation,
) (ManagedCarrierInstallPlan, error) {
	if reconciliation.readiness == ManagedCarrierBlocked {
		return ManagedCarrierInstallPlan{
			reconciliation: cloneManagedCarrierReconciliation(reconciliation),
		}, nil
	}
	if reconciliation.readiness != ManagedCarrierReady {
		return ManagedCarrierInstallPlan{}, fmt.Errorf(
			"managed carrier reconciliation readiness is invalid",
		)
	}
	result, err := ApplyManagedCarrierReconciliation(
		reconciliation,
		reconciliation.observation.carrier,
	)
	if err != nil {
		return ManagedCarrierInstallPlan{}, err
	}
	return ManagedCarrierInstallPlan{
		reconciliation: cloneManagedCarrierReconciliation(reconciliation),
		result:         cloneManagedCarrierMutationResult(result),
		hasResult:      true,
	}, nil
}

func cloneManagedCarrierInstallPlan(
	plan ManagedCarrierInstallPlan,
) ManagedCarrierInstallPlan {
	return ManagedCarrierInstallPlan{
		reconciliation: cloneManagedCarrierReconciliation(plan.reconciliation),
		result:         cloneManagedCarrierMutationResult(plan.result),
		hasResult:      plan.hasResult,
	}
}

func cloneManagedCarrierInstallPlans(
	plans []ManagedCarrierInstallPlan,
) []ManagedCarrierInstallPlan {
	result := make([]ManagedCarrierInstallPlan, len(plans))
	for index, plan := range plans {
		result[index] = cloneManagedCarrierInstallPlan(plan)
	}
	return result
}

func (plan ManagedCarrierInstallPlan) Path() string {
	return plan.reconciliation.observation.carrier.path
}

func (plan ManagedCarrierInstallPlan) Components() ComponentSet {
	return plan.reconciliation.Components()
}

func (plan ManagedCarrierInstallPlan) Component() Component {
	return plan.reconciliation.Component()
}

func (plan ManagedCarrierInstallPlan) Readiness() ManagedCarrierReadiness {
	return plan.reconciliation.readiness
}

func (plan ManagedCarrierInstallPlan) Predecessor() ManagedCarrierInput {
	return cloneManagedCarrierInput(
		plan.reconciliation.observation.carrier,
	)
}

func (plan ManagedCarrierInstallPlan) MutationResult() (
	ManagedCarrierMutationResult,
	bool,
) {
	return cloneManagedCarrierMutationResult(plan.result), plan.hasResult
}

func (plan ManagedCarrierInstallPlan) Conflicts() []ManagedFragmentConflict {
	return plan.reconciliation.Conflicts()
}

func (plan ManagedCarrierInstallPlan) Effects() []ManagedFragmentEffect {
	return plan.reconciliation.Effects()
}

func (plan ManagedCarrierInstallPlan) ManifestBasis() OwnershipBasis {
	return plan.reconciliation.basis
}

func (result ManagedCarrierMutationResult) Kind() ManagedCarrierResultKind {
	return result.kind
}

func (result ManagedCarrierMutationResult) Path() string {
	return result.path
}

func (result ManagedCarrierMutationResult) Content() []byte {
	return slices.Clone(result.content)
}

func (result ManagedCarrierMutationResult) Digest() string {
	return result.digest
}

func (result ManagedCarrierMutationResult) Mode() fs.FileMode {
	return result.mode
}

func (result ManagedCarrierMutationResult) Changed() bool {
	return result.changed
}

func cloneManagedCarrierMutationResult(
	result ManagedCarrierMutationResult,
) ManagedCarrierMutationResult {
	return ManagedCarrierMutationResult{
		kind:    result.kind,
		path:    result.path,
		content: slices.Clone(result.content),
		digest:  result.digest,
		mode:    result.mode,
		changed: result.changed,
	}
}

func ApplyManagedCarrierReconciliation(
	plan ManagedCarrierReconciliation,
	input ManagedCarrierInput,
) (ManagedCarrierMutationResult, error) {
	if plan.readiness != ManagedCarrierReady {
		return ManagedCarrierMutationResult{}, fmt.Errorf(
			"blocked managed carrier reconciliation cannot be applied",
		)
	}
	if err := requireExactManagedCarrierPredecessor(
		plan.observation.carrier,
		input,
	); err != nil {
		return ManagedCarrierMutationResult{}, err
	}
	if !managedCarrierHasMutation(plan.effects) {
		return unchangedManagedCarrierResult(input), nil
	}
	var content []byte
	var err error
	if plan.syntax == managedCarrierJSON {
		content, err = applyManagedJSONEffects(
			plan.effects,
			input,
			plan.mergeEdition,
		)
	}
	if plan.syntax == managedCarrierTOML {
		content, err = applyManagedTOMLEffects(plan.effects, input)
	}
	if plan.syntax == managedCarrierYAML {
		content, err = applyManagedYAMLEffects(plan.effects, input)
	}
	if plan.syntax == managedCarrierText {
		content, err = applyManagedTextEffects(plan.effects, input)
	}
	if err != nil {
		return ManagedCarrierMutationResult{}, err
	}
	if content == nil {
		return ManagedCarrierMutationResult{}, fmt.Errorf(
			"managed carrier reconciliation syntax is unsupported",
		)
	}
	mode := input.mode
	if input.kind == ManagedCarrierMissing {
		mode = plan.createMode
	}
	if !validPermissionMode(mode) {
		return ManagedCarrierMutationResult{}, fmt.Errorf(
			"managed carrier output mode is unavailable",
		)
	}
	changed := input.kind == ManagedCarrierMissing ||
		!bytes.Equal(content, input.content)
	kind := ManagedCarrierUnchanged
	if changed {
		kind = ManagedCarrierWrite
	}
	return ManagedCarrierMutationResult{
		kind:    kind,
		path:    input.path,
		content: slices.Clone(content),
		digest:  managedFragmentDigest(content),
		mode:    mode,
		changed: changed,
	}, nil
}

func requireExactManagedCarrierPredecessor(
	expected ManagedCarrierInput,
	observed ManagedCarrierInput,
) error {
	if expected.path != observed.path ||
		expected.kind != observed.kind {
		return fmt.Errorf("managed carrier precondition changed")
	}
	if expected.kind == ManagedCarrierMissing {
		return nil
	}
	if expected.digest != observed.digest ||
		expected.mode != observed.mode ||
		!bytes.Equal(expected.content, observed.content) {
		return fmt.Errorf("managed carrier precondition changed")
	}
	return nil
}

func managedCarrierHasMutation(
	effects []ManagedFragmentEffect,
) bool {
	for _, effect := range effects {
		if effect.kind == ManagedFragmentCreate ||
			effect.kind == ManagedFragmentReplace ||
			effect.kind == ManagedFragmentRemove {
			return true
		}
	}
	return false
}

func unchangedManagedCarrierResult(
	input ManagedCarrierInput,
) ManagedCarrierMutationResult {
	if input.kind == ManagedCarrierMissing {
		return ManagedCarrierMutationResult{
			kind: ManagedCarrierAbsent,
			path: input.path,
		}
	}
	return ManagedCarrierMutationResult{
		kind:    ManagedCarrierUnchanged,
		path:    input.path,
		content: slices.Clone(input.content),
		digest:  input.digest,
		mode:    input.mode,
	}
}

func applyManagedJSONEffects(
	effects []ManagedFragmentEffect,
	input ManagedCarrierInput,
	mergeEdition string,
) ([]byte, error) {
	root := make(map[string]any)
	if input.kind == ManagedCarrierPresent {
		value, err := decodeManagedJSON(input.content, mergeEdition)
		if err != nil {
			return nil, fmt.Errorf("apply managed JSON carrier: %w", err)
		}
		typed, ok := value.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("managed JSON carrier root must be an object")
		}
		root = typed
	}
	for _, effect := range effects {
		if !managedFragmentEffectMutates(effect.kind) {
			continue
		}
		if effect.coordinate.kind == ManagedJSONObjectEntry {
			if err := applyJSONObjectEntryEffect(root, effect); err != nil {
				return nil, err
			}
			continue
		}
		if effect.coordinate.kind == ManagedJSONArrayMember {
			if err := applyJSONArrayMemberEffect(
				root,
				effect,
			); err != nil {
				return nil, err
			}
			continue
		}
		return nil, fmt.Errorf("managed JSON effect kind is invalid")
	}
	content, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode managed JSON carrier: %w", err)
	}
	return append(content, '\n'), nil
}

func managedFragmentEffectMutates(
	kind ManagedFragmentEffectKind,
) bool {
	return kind == ManagedFragmentCreate ||
		kind == ManagedFragmentReplace ||
		kind == ManagedFragmentRemove
}

func applyJSONObjectEntryEffect(
	root map[string]any,
	effect ManagedFragmentEffect,
) error {
	if effect.kind == ManagedFragmentRemove {
		return removeJSONObjectEntry(root, effect)
	}
	if !effect.hasDesired {
		return fmt.Errorf("managed JSON create/replace lacks desired fragment")
	}
	value, err := decodeUniqueJSON(effect.desired.content)
	if err != nil {
		return err
	}
	parent, key, found, err := resolveJSONObjectParent(
		root,
		effect.coordinate.jsonPath,
		effect.kind == ManagedFragmentCreate,
	)
	if err != nil {
		return err
	}
	current, exists := parent[key]
	if effect.kind == ManagedFragmentCreate && exists {
		return fmt.Errorf("managed JSON create target is no longer vacant")
	}
	if effect.kind == ManagedFragmentReplace {
		if !found || !exists {
			return fmt.Errorf("managed JSON replace target is missing")
		}
		if err := requireJSONValueDigest(current, effect.expectedDigest); err != nil {
			return err
		}
	}
	parent[key] = value
	return nil
}

func removeJSONObjectEntry(
	root map[string]any,
	effect ManagedFragmentEffect,
) error {
	parent, key, found, err := resolveJSONObjectParent(
		root,
		effect.coordinate.jsonPath,
		false,
	)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("managed JSON remove target is missing")
	}
	current, exists := parent[key]
	if !exists {
		return fmt.Errorf("managed JSON remove target is missing")
	}
	if err := requireJSONValueDigest(current, effect.expectedDigest); err != nil {
		return err
	}
	delete(parent, key)
	return nil
}

func resolveJSONObjectParent(
	root map[string]any,
	path []string,
	create bool,
) (map[string]any, string, bool, error) {
	if len(path) == 0 {
		return nil, "", false, fmt.Errorf("managed JSON selector is empty")
	}
	current := root
	for _, token := range path[:len(path)-1] {
		value, found := current[token]
		if !found && create {
			next := make(map[string]any)
			current[token] = next
			current = next
			continue
		}
		if !found {
			return current, path[len(path)-1], false, nil
		}
		next, ok := value.(map[string]any)
		if !ok {
			return nil, "", false, fmt.Errorf(
				"managed JSON selector crosses non-object token %q",
				token,
			)
		}
		current = next
	}
	return current, path[len(path)-1], true, nil
}

func requireJSONValueDigest(
	value any,
	expected string,
) error {
	canonical, err := marshalCanonicalJSONValue(value)
	if err != nil {
		return err
	}
	if managedFragmentDigest(canonical) != expected {
		return fmt.Errorf("managed JSON fragment precondition changed")
	}
	return nil
}

func applyJSONArrayMemberEffect(
	root map[string]any,
	effect ManagedFragmentEffect,
) error {
	parent, key, found, err := resolveJSONObjectParent(
		root,
		effect.coordinate.jsonPath,
		effect.kind == ManagedFragmentCreate,
	)
	if err != nil {
		return err
	}
	current, exists := parent[key]
	if !exists && effect.kind == ManagedFragmentCreate {
		current = []any{}
		parent[key] = current
		found = true
	}
	if !found || !exists && effect.kind != ManagedFragmentCreate {
		return fmt.Errorf("managed JSON array target is missing")
	}
	array, ok := current.([]any)
	if !ok {
		return fmt.Errorf(
			"managed JSON selector %s must name an array",
			effect.coordinate.selector,
		)
	}
	if effect.kind == ManagedFragmentCreate {
		value, err := decodeUniqueJSON(effect.desired.content)
		if err != nil {
			return err
		}
		parent[key] = append(array, value)
		return nil
	}
	if effect.coordinate.mergeEdition ==
		ManagedJSONArraySourceMergeEdition {
		return applyJSONArraySourceMemberEffect(
			parent,
			key,
			array,
			effect,
		)
	}
	index, err := exactJSONArrayMemberIndex(
		array,
		effect.expectedDigest,
	)
	if err != nil {
		return err
	}
	if effect.kind == ManagedFragmentRemove {
		parent[key] = append(
			slices.Clone(array[:index]),
			array[index+1:]...,
		)
		return nil
	}
	if effect.kind != ManagedFragmentReplace || !effect.hasDesired {
		return fmt.Errorf("managed JSON array effect is invalid")
	}
	value, err := decodeUniqueJSON(effect.desired.content)
	if err != nil {
		return err
	}
	next := slices.Clone(array)
	next[index] = value
	parent[key] = next
	return nil
}

func applyJSONArraySourceMemberEffect(
	parent map[string]any,
	key string,
	array []any,
	effect ManagedFragmentEffect,
) error {
	index, err := exactJSONArraySourceMemberIndex(
		array,
		effect.expectedDigest,
	)
	if err != nil {
		return err
	}
	if effect.kind == ManagedFragmentRemove {
		parent[key] = append(
			slices.Clone(array[:index]),
			array[index+1:]...,
		)
		return nil
	}
	if effect.kind != ManagedFragmentReplace || !effect.hasDesired {
		return fmt.Errorf("managed JSON array source effect is invalid")
	}
	value, err := decodeUniqueJSON(effect.desired.content)
	if err != nil {
		return err
	}
	desiredSource, ok := value.(string)
	if !ok {
		return fmt.Errorf(
			"managed JSON array source desired value must be a string",
		)
	}
	next := slices.Clone(array)
	if object, ok := next[index].(map[string]any); ok {
		updated := make(map[string]any, len(object))
		for field, fieldValue := range object {
			updated[field] = fieldValue
		}
		updated["source"] = desiredSource
		next[index] = updated
	} else {
		next[index] = desiredSource
	}
	parent[key] = next
	return nil
}

func exactJSONArraySourceMemberIndex(
	array []any,
	expectedDigest string,
) (int, error) {
	found := -1
	for index, value := range array {
		source, present := managedJSONArraySource(value)
		if !present {
			continue
		}
		canonical, err := marshalCanonicalJSONValue(source)
		if err != nil {
			return -1, err
		}
		if managedFragmentDigest(canonical) != expectedDigest {
			continue
		}
		if found >= 0 {
			return -1, fmt.Errorf(
				"managed JSON array source member is ambiguous",
			)
		}
		found = index
	}
	if found < 0 {
		return -1, fmt.Errorf(
			"managed JSON array source member precondition changed",
		)
	}
	return found, nil
}

func exactJSONArrayMemberIndex(
	array []any,
	expectedDigest string,
) (int, error) {
	found := -1
	for index, value := range array {
		canonical, err := marshalCanonicalJSONValue(value)
		if err != nil {
			return -1, err
		}
		if managedFragmentDigest(canonical) != expectedDigest {
			continue
		}
		if found >= 0 {
			return -1, fmt.Errorf("managed JSON array member is ambiguous")
		}
		found = index
	}
	if found < 0 {
		return -1, fmt.Errorf("managed JSON array member precondition changed")
	}
	return found, nil
}

func applyManagedTOMLEffects(
	effects []ManagedFragmentEffect,
	input ManagedCarrierInput,
) ([]byte, error) {
	content := slices.Clone(input.content)
	if _, err := scanTOMLSections(content); err != nil {
		return nil, fmt.Errorf("apply managed TOML carrier: %w", err)
	}
	for _, effect := range effects {
		if !managedFragmentEffectMutates(effect.kind) {
			continue
		}
		if effect.coordinate.kind != ManagedTOMLTableFamily &&
			effect.coordinate.kind != ManagedTOMLTableSet {
			return nil, fmt.Errorf("managed TOML effect kind is invalid")
		}
		sections, err := scanTOMLSections(content)
		if err != nil {
			return nil, err
		}
		var current []byte
		var found bool
		if effect.coordinate.kind == ManagedTOMLTableFamily {
			current, found = extractTOMLTableFamily(
				content,
				sections,
				effect.coordinate.tomlPrefix,
			)
		}
		if effect.coordinate.kind == ManagedTOMLTableSet {
			current, found, err = extractTOMLTableSet(
				content,
				sections,
				effect.coordinate.tomlTables,
			)
			if err != nil {
				return nil, err
			}
		}
		if effect.kind == ManagedFragmentCreate && found {
			return nil, fmt.Errorf("managed TOML create target is no longer vacant")
		}
		if effect.kind != ManagedFragmentCreate {
			if !found {
				return nil, fmt.Errorf("managed TOML target is missing")
			}
			if managedFragmentDigest(current) != effect.expectedDigest {
				return nil, fmt.Errorf("managed TOML fragment precondition changed")
			}
			if effect.coordinate.kind == ManagedTOMLTableFamily {
				content = removeTOMLTableFamily(
					content,
					sections,
					effect.coordinate.tomlPrefix,
				)
			}
		}
		if effect.kind == ManagedFragmentRemove {
			if effect.coordinate.kind == ManagedTOMLTableSet {
				content = replaceTOMLTableSet(
					content,
					sections,
					effect.coordinate.tomlTables,
					nil,
				)
			}
			continue
		}
		if !effect.hasDesired {
			return nil, fmt.Errorf("managed TOML create/replace lacks desired fragment")
		}
		if effect.coordinate.kind == ManagedTOMLTableSet {
			if effect.kind == ManagedFragmentCreate {
				content = insertTOMLTableSet(
					content,
					sections,
					effect.coordinate.tomlPrefix,
					effect.desired.content,
				)
				continue
			}
			content = replaceTOMLTableSet(
				content,
				sections,
				effect.coordinate.tomlTables,
				effect.desired.content,
			)
			continue
		}
		content = appendTOMLTableFamily(content, effect.desired.content)
	}
	return content, nil
}

func canonicalJSONValue(
	raw []byte,
) ([]byte, error) {
	value, err := decodeUniqueJSON(raw)
	if err != nil {
		return nil, err
	}
	if value == nil {
		return nil, fmt.Errorf("managed JSON fragment cannot be null")
	}
	return marshalCanonicalJSONValue(value)
}

func marshalCanonicalJSONValue(
	value any,
) ([]byte, error) {
	canonical, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode canonical JSON value: %w", err)
	}
	return canonical, nil
}

func decodeUniqueJSON(
	raw []byte,
) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	value, err := decodeUniqueJSONValue(decoder)
	if err != nil {
		return nil, err
	}
	var trailing any
	err = decoder.Decode(&trailing)
	if err != io.EOF {
		return nil, fmt.Errorf("JSON value has trailing content")
	}
	return value, nil
}

func decodeUniqueJSONValue(
	decoder *json.Decoder,
) (any, error) {
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	delimiter, compound := token.(json.Delim)
	if !compound {
		return token, nil
	}
	if delimiter == '{' {
		return decodeUniqueJSONObject(decoder)
	}
	if delimiter == '[' {
		return decodeUniqueJSONArray(decoder)
	}
	return nil, fmt.Errorf("unexpected JSON delimiter %q", delimiter)
}

func decodeUniqueJSONObject(
	decoder *json.Decoder,
) (map[string]any, error) {
	result := make(map[string]any)
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		key, ok := token.(string)
		if !ok {
			return nil, fmt.Errorf("JSON object key is not a string")
		}
		if _, duplicate := result[key]; duplicate {
			return nil, fmt.Errorf("duplicate JSON object key %q", key)
		}
		value, err := decodeUniqueJSONValue(decoder)
		if err != nil {
			return nil, err
		}
		result[key] = value
	}
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	if token != json.Delim('}') {
		return nil, fmt.Errorf("JSON object is not closed")
	}
	return result, nil
}

func decodeUniqueJSONArray(
	decoder *json.Decoder,
) ([]any, error) {
	result := make([]any, 0)
	for decoder.More() {
		value, err := decodeUniqueJSONValue(decoder)
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	if token != json.Delim(']') {
		return nil, fmt.Errorf("JSON array is not closed")
	}
	return result, nil
}

type tomlSectionSpan struct {
	name  string
	array bool
	start int
	end   int
}

func canonicalTOMLTableFamily(
	raw []byte,
	prefix string,
) ([]byte, error) {
	normalized := strings.TrimSpace(
		strings.ReplaceAll(string(raw), "\r\n", "\n"),
	)
	if normalized == "" {
		return nil, fmt.Errorf("managed TOML table-family content is empty")
	}
	content := []byte(normalized + "\n")
	sections, err := scanTOMLSections(content)
	if err != nil {
		return nil, fmt.Errorf("managed TOML table-family content: %w", err)
	}
	if len(sections) == 0 ||
		sections[0].start != 0 ||
		sections[0].name != prefix ||
		sections[0].array {
		return nil, fmt.Errorf(
			"managed TOML table-family must start with [%s]",
			prefix,
		)
	}
	for _, section := range sections {
		if section.array || !tomlTableBelongsToFamily(section.name, prefix) {
			return nil, fmt.Errorf(
				"managed TOML content escapes table family %s",
				prefix,
			)
		}
	}
	return content, nil
}

func canonicalTOMLTableNames(
	prefix string,
	raw []string,
) ([]string, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("managed TOML table set is empty")
	}
	tables := slices.Clone(raw)
	sort.Strings(tables)
	previous := ""
	for _, table := range tables {
		validated, err := validateTOMLTablePrefix(table)
		if err != nil {
			return nil, err
		}
		if !tomlTableBelongsToFamily(validated, prefix) {
			return nil, fmt.Errorf(
				"managed TOML table %s escapes prefix %s",
				validated,
				prefix,
			)
		}
		if validated == previous {
			return nil, fmt.Errorf(
				"managed TOML table set repeats %s",
				validated,
			)
		}
		previous = validated
	}
	return tables, nil
}

func canonicalTOMLTableSet(
	raw []byte,
	tables []string,
) ([]byte, error) {
	normalized := strings.TrimSpace(
		strings.ReplaceAll(string(raw), "\r\n", "\n"),
	)
	if normalized == "" {
		return nil, fmt.Errorf("managed TOML table-set content is empty")
	}
	content := []byte(normalized + "\n")
	sections, err := scanTOMLSections(content)
	if err != nil {
		return nil, fmt.Errorf("managed TOML table-set content: %w", err)
	}
	if len(sections) != len(tables) || sections[0].start != 0 {
		return nil, fmt.Errorf(
			"managed TOML table-set content does not match its exact table set",
		)
	}
	for index, section := range sections {
		if section.array || section.name != tables[index] {
			return nil, fmt.Errorf(
				"managed TOML table-set content must contain [%s] at position %d",
				tables[index],
				index+1,
			)
		}
	}
	return content, nil
}

func scanTOMLSections(
	raw []byte,
) ([]tomlSectionSpan, error) {
	sections := make([]tomlSectionSpan, 0)
	offset := 0
	for offset < len(raw) {
		lineEnd := bytes.IndexByte(raw[offset:], '\n')
		if lineEnd < 0 {
			lineEnd = len(raw)
		} else {
			lineEnd = offset + lineEnd + 1
		}
		if lineEnd < offset {
			lineEnd = len(raw)
		}
		line := string(raw[offset:lineEnd])
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") {
			match := tomlTableHeaderPattern.FindStringSubmatch(trimmed)
			if match == nil || !matchingTOMLHeaderBrackets(match[1], match[3]) {
				return nil, fmt.Errorf(
					"unsupported TOML table header %q",
					trimmed,
				)
			}
			if len(sections) > 0 {
				sections[len(sections)-1].end = offset
			}
			sections = append(sections, tomlSectionSpan{
				name:  match[2],
				array: match[1] == "[[",
				start: offset,
				end:   len(raw),
			})
		}
		offset = lineEnd
	}
	return sections, nil
}

func matchingTOMLHeaderBrackets(
	open string,
	close string,
) bool {
	return open == "[" && close == "]" ||
		open == "[[" && close == "]]"
}

func tomlTableBelongsToFamily(
	name string,
	prefix string,
) bool {
	return name == prefix || strings.HasPrefix(name, prefix+".")
}

func extractTOMLTableFamily(
	raw []byte,
	sections []tomlSectionSpan,
	prefix string,
) ([]byte, bool) {
	parts := make([]string, 0)
	for _, section := range sections {
		if section.array || !tomlTableBelongsToFamily(section.name, prefix) {
			continue
		}
		part := strings.TrimSpace(string(raw[section.start:section.end]))
		parts = append(parts, part)
	}
	if len(parts) == 0 {
		return nil, false
	}
	return []byte(strings.Join(parts, "\n\n") + "\n"), true
}

// ExtractTOMLTableFamily returns the exact normalized bytes used by managed
// fragment observation for one table family. Callers may use those bytes only
// as an observation; ownership still requires a manifest or a closed
// known-legacy registry.
func ExtractTOMLTableFamily(
	raw []byte,
	prefix string,
) ([]byte, bool, error) {
	validatedPrefix, err := validateTOMLTablePrefix(prefix)
	if err != nil {
		return nil, false, err
	}
	sections, err := scanTOMLSections(raw)
	if err != nil {
		return nil, false, err
	}
	content, found := extractTOMLTableFamily(
		raw,
		sections,
		validatedPrefix,
	)
	return slices.Clone(content), found, nil
}

// ExtractTOMLTableSet returns the exact normalized bytes used by managed
// fragment observation for a closed set of tables. Callers may validate those
// bytes as historical input, but the observation does not establish ownership.
func ExtractTOMLTableSet(
	raw []byte,
	prefix string,
	tables []string,
) ([]byte, bool, error) {
	validatedPrefix, err := validateTOMLTablePrefix(prefix)
	if err != nil {
		return nil, false, err
	}
	validatedTables, err := canonicalTOMLTableNames(
		validatedPrefix,
		tables,
	)
	if err != nil {
		return nil, false, err
	}
	sections, err := scanTOMLSections(raw)
	if err != nil {
		return nil, false, err
	}
	content, found, err := extractTOMLTableSet(
		raw,
		sections,
		validatedTables,
	)
	if err != nil {
		return nil, false, err
	}
	return slices.Clone(content), found, nil
}

func extractTOMLTableSet(
	raw []byte,
	sections []tomlSectionSpan,
	tables []string,
) ([]byte, bool, error) {
	parts := make([]string, 0, len(tables))
	found := false
	for _, table := range tables {
		matches := make([]tomlSectionSpan, 0, 1)
		for _, section := range sections {
			if !section.array && section.name == table {
				matches = append(matches, section)
			}
		}
		if len(matches) > 1 {
			return nil, false, fmt.Errorf(
				"managed TOML table [%s] is ambiguous",
				table,
			)
		}
		if len(matches) == 0 {
			continue
		}
		found = true
		part := strings.TrimSpace(
			string(raw[matches[0].start:matches[0].end]),
		)
		parts = append(parts, part)
	}
	if !found {
		return nil, false, nil
	}
	return []byte(strings.Join(parts, "\n\n") + "\n"), true, nil
}

func removeTOMLTableFamily(
	raw []byte,
	sections []tomlSectionSpan,
	prefix string,
) []byte {
	var output bytes.Buffer
	cursor := 0
	for _, section := range sections {
		if section.array || !tomlTableBelongsToFamily(section.name, prefix) {
			continue
		}
		output.Write(raw[cursor:section.start])
		cursor = section.end
	}
	output.Write(raw[cursor:])
	if cursor < len(raw) {
		// Bytes after the removed table family are outside the managed span.
		// Preserve that complete operator-owned suffix, including end-of-file
		// whitespace.
		return output.Bytes()
	}
	return []byte(strings.TrimRight(output.String(), " \t\r\n"))
}

func appendTOMLTableFamily(
	raw []byte,
	fragment []byte,
) []byte {
	prefix := strings.TrimRight(string(raw), " \t\r\n")
	if prefix == "" {
		return slices.Clone(fragment)
	}
	return []byte(prefix + "\n\n" + string(fragment))
}

func replaceTOMLTableSet(
	raw []byte,
	sections []tomlSectionSpan,
	tables []string,
	replacement []byte,
) []byte {
	selected := make(map[string]struct{}, len(tables))
	for _, table := range tables {
		selected[table] = struct{}{}
	}
	var output bytes.Buffer
	cursor := 0
	inserted := false
	for _, section := range sections {
		if section.array {
			continue
		}
		if _, ok := selected[section.name]; !ok {
			continue
		}
		output.Write(raw[cursor:section.start])
		if !inserted && len(replacement) > 0 {
			output.Write(replacement)
			output.WriteByte('\n')
			inserted = true
		}
		cursor = section.end
	}
	output.Write(raw[cursor:])
	if cursor < len(raw) {
		// Bytes after the replaced table set are outside the managed span.
		// Preserve that complete operator-owned suffix, including end-of-file
		// whitespace.
		return output.Bytes()
	}
	return []byte(strings.TrimRight(output.String(), " \t\r\n"))
}

func insertTOMLTableSet(
	raw []byte,
	sections []tomlSectionSpan,
	prefix string,
	fragment []byte,
) []byte {
	for _, section := range sections {
		if !section.array &&
			tomlTableBelongsToFamily(section.name, prefix) {
			var output bytes.Buffer
			output.Write(raw[:section.start])
			output.Write(fragment)
			output.WriteByte('\n')
			output.Write(raw[section.start:])
			return output.Bytes()
		}
	}
	return appendTOMLTableFamily(raw, fragment)
}
