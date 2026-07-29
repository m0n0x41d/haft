package specmigrationv2

import (
	"fmt"
	"math"
	"strings"
	"unicode"
)

const SchemaVersionV2 uint32 = 2

type ByteLength struct {
	value uint64
}

func NewByteLength(value uint64) (ByteLength, error) {
	if value == 0 {
		return ByteLength{}, fmt.Errorf("byte length must be greater than zero")
	}
	return ByteLength{value: value}, nil
}

func (length ByteLength) Value() uint64 {
	return length.value
}

func (length ByteLength) valid() bool {
	return length.value > 0
}

type ExactByteSpan struct {
	start  uint64
	length ByteLength
	digest FragmentDigest
}

func NewExactByteSpan(start uint64, length ByteLength, digest FragmentDigest) (ExactByteSpan, error) {
	if !length.valid() {
		return ExactByteSpan{}, fmt.Errorf("exact byte span length is invalid")
	}
	if !digest.valid() {
		return ExactByteSpan{}, fmt.Errorf("exact byte span digest is invalid")
	}
	if start > math.MaxUint64-length.Value() {
		return ExactByteSpan{}, fmt.Errorf("exact byte span overflows uint64")
	}
	return ExactByteSpan{start: start, length: length, digest: digest}, nil
}

func (span ExactByteSpan) Start() uint64 {
	return span.start
}

func (span ExactByteSpan) Length() ByteLength {
	return span.length
}

func (span ExactByteSpan) End() uint64 {
	return span.start + span.length.Value()
}

func (span ExactByteSpan) Digest() FragmentDigest {
	return span.digest
}

func (span ExactByteSpan) valid() bool {
	if !span.length.valid() || !span.digest.valid() {
		return false
	}
	return span.start <= math.MaxUint64-span.length.Value()
}

type SourceSection struct {
	id   SourceSectionID
	span ExactByteSpan
}

func NewSourceSection(id SourceSectionID, span ExactByteSpan) (SourceSection, error) {
	if !id.valid() {
		return SourceSection{}, fmt.Errorf("source section ID is invalid")
	}
	if !span.valid() {
		return SourceSection{}, fmt.Errorf("source section span is invalid")
	}
	return SourceSection{id: id, span: span}, nil
}

func (section SourceSection) ID() SourceSectionID {
	return section.id
}

func (section SourceSection) Span() ExactByteSpan {
	return section.span
}

func (section SourceSection) valid() bool {
	return section.id.valid() && section.span.valid()
}

type ArchiveManifest struct {
	carrier      ArchiveCarrierID
	sourceDigest SourceDigest
}

func NewArchiveManifest(carrier ArchiveCarrierID, sourceDigest SourceDigest) (ArchiveManifest, error) {
	if !carrier.valid() {
		return ArchiveManifest{}, fmt.Errorf("archive carrier ID is invalid")
	}
	if !sourceDigest.valid() {
		return ArchiveManifest{}, fmt.Errorf("archive source digest is invalid")
	}
	return ArchiveManifest{carrier: carrier, sourceDigest: sourceDigest}, nil
}

func (manifest ArchiveManifest) Carrier() ArchiveCarrierID {
	return manifest.carrier
}

func (manifest ArchiveManifest) SourceDigest() SourceDigest {
	return manifest.sourceDigest
}

func (manifest ArchiveManifest) valid() bool {
	return manifest.carrier.valid() && manifest.sourceDigest.valid()
}

type SourceManifestInput struct {
	Carrier    SourceCarrierID
	Digest     SourceDigest
	ByteLength ByteLength
	Archive    ArchiveManifest
	Provenance DesignatedSourceProvenance
	Sections   []SourceSection
}

type SourceManifest struct {
	carrier    SourceCarrierID
	digest     SourceDigest
	byteLength ByteLength
	archive    ArchiveManifest
	provenance DesignatedSourceProvenance
	sections   []SourceSection
}

func NewSourceManifest(input SourceManifestInput) (SourceManifest, error) {
	if !input.Carrier.valid() {
		return SourceManifest{}, fmt.Errorf("source carrier ID is invalid")
	}
	if !input.Digest.valid() {
		return SourceManifest{}, fmt.Errorf("source digest is invalid")
	}
	if !input.ByteLength.valid() {
		return SourceManifest{}, fmt.Errorf("source byte length is invalid")
	}
	if !input.Archive.valid() {
		return SourceManifest{}, fmt.Errorf("source archive manifest is invalid")
	}
	if !input.Provenance.valid() {
		return SourceManifest{}, fmt.Errorf("designated-source provenance is invalid")
	}
	if input.Provenance.origin.Carrier().String() != input.Carrier.String() {
		return SourceManifest{}, fmt.Errorf("designated-source provenance carrier does not match source manifest")
	}
	if !input.Provenance.origin.DesignatedDigest().Equal(input.Digest) {
		return SourceManifest{}, fmt.Errorf("designated-source provenance digest does not match source manifest")
	}
	if len(input.Sections) == 0 {
		return SourceManifest{}, fmt.Errorf("source manifest requires at least one exact section")
	}
	sections := append([]SourceSection{}, input.Sections...)
	for index, section := range sections {
		if section.valid() {
			continue
		}
		return SourceManifest{}, fmt.Errorf("source section %d is invalid", index)
	}
	return SourceManifest{
		carrier:    input.Carrier,
		digest:     input.Digest,
		byteLength: input.ByteLength,
		archive:    input.Archive,
		provenance: input.Provenance,
		sections:   sections,
	}, nil
}

func (manifest SourceManifest) Carrier() SourceCarrierID {
	return manifest.carrier
}

func (manifest SourceManifest) Digest() SourceDigest {
	return manifest.digest
}

func (manifest SourceManifest) ByteLength() ByteLength {
	return manifest.byteLength
}

func (manifest SourceManifest) Archive() ArchiveManifest {
	return manifest.archive
}

func (manifest SourceManifest) Provenance() DesignatedSourceProvenance {
	return manifest.provenance
}

func (manifest SourceManifest) Sections() []SourceSection {
	return append([]SourceSection{}, manifest.sections...)
}

func (manifest SourceManifest) valid() bool {
	return manifest.carrier.valid() &&
		manifest.digest.valid() &&
		manifest.byteLength.valid() &&
		manifest.archive.valid() &&
		manifest.provenance.valid() &&
		manifest.provenance.origin.Carrier().String() == manifest.carrier.String() &&
		manifest.provenance.origin.DesignatedDigest().Equal(manifest.digest) &&
		len(manifest.sections) > 0
}

type TargetManifestInput struct {
	Carrier    TargetCarrierID
	Digest     TargetDigest
	ByteLength ByteLength
}

type TargetManifest struct {
	carrier    TargetCarrierID
	digest     TargetDigest
	byteLength ByteLength
}

func NewTargetManifest(input TargetManifestInput) (TargetManifest, error) {
	if !input.Carrier.valid() {
		return TargetManifest{}, fmt.Errorf("target carrier ID is invalid")
	}
	if !input.Digest.valid() {
		return TargetManifest{}, fmt.Errorf("target digest is invalid")
	}
	if !input.ByteLength.valid() {
		return TargetManifest{}, fmt.Errorf("target byte length is invalid")
	}
	return TargetManifest{
		carrier:    input.Carrier,
		digest:     input.Digest,
		byteLength: input.ByteLength,
	}, nil
}

func (manifest TargetManifest) Carrier() TargetCarrierID {
	return manifest.carrier
}

func (manifest TargetManifest) Digest() TargetDigest {
	return manifest.digest
}

func (manifest TargetManifest) ByteLength() ByteLength {
	return manifest.byteLength
}

func (manifest TargetManifest) valid() bool {
	return manifest.carrier.valid() && manifest.digest.valid() && manifest.byteLength.valid()
}

type TargetClaimSet struct {
	values  []TargetAtomicClaimID
	section TargetSectionID
}

func NewTargetClaimSet(values []TargetAtomicClaimID) (TargetClaimSet, error) {
	if len(values) == 0 {
		return TargetClaimSet{}, fmt.Errorf("MapOne requires at least one exact target atomic claim ID")
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]TargetAtomicClaimID, 0, len(values))
	section := values[0].Section()
	for index, value := range values {
		if !value.valid() {
			return TargetClaimSet{}, fmt.Errorf("target atomic claim ID %d is invalid", index)
		}
		key := value.String()
		if _, exists := seen[key]; exists {
			return TargetClaimSet{}, fmt.Errorf("duplicate target atomic claim ID %q", key)
		}
		if value.Section().String() != section.String() {
			return TargetClaimSet{}, fmt.Errorf(
				"MapOne target claims must belong to one exact target section; got %q and %q",
				section.String(),
				value.Section().String(),
			)
		}
		seen[key] = struct{}{}
		result = append(result, value)
	}
	return TargetClaimSet{values: result, section: section}, nil
}

func (set TargetClaimSet) Values() []TargetAtomicClaimID {
	return append([]TargetAtomicClaimID{}, set.values...)
}

func (set TargetClaimSet) TargetSection() TargetSectionID {
	return set.section
}

func (set TargetClaimSet) valid() bool {
	_, err := NewTargetClaimSet(set.values)
	return err == nil
}

type OutsideCarrierSet struct {
	values []OutsideCarrierID
}

func NewOutsideCarrierSet(values []OutsideCarrierID) (OutsideCarrierSet, error) {
	if len(values) == 0 {
		return OutsideCarrierSet{}, fmt.Errorf("OutsidePSS requires at least one exact carrier reference")
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]OutsideCarrierID, 0, len(values))
	for index, value := range values {
		if !value.valid() {
			return OutsideCarrierSet{}, fmt.Errorf("outside carrier ID %d is invalid", index)
		}
		key := value.String()
		if _, exists := seen[key]; exists {
			return OutsideCarrierSet{}, fmt.Errorf("duplicate outside carrier ID %q", key)
		}
		seen[key] = struct{}{}
		result = append(result, value)
	}
	return OutsideCarrierSet{values: result}, nil
}

func (set OutsideCarrierSet) Values() []OutsideCarrierID {
	return append([]OutsideCarrierID{}, set.values...)
}

func (set OutsideCarrierSet) valid() bool {
	_, err := NewOutsideCarrierSet(set.values)
	return err == nil
}

type Disposition interface {
	dispositionVariant()
}

type BranchDisposition interface {
	Disposition
	branchDispositionVariant()
}

type MapOne struct {
	targetClaims TargetClaimSet
}

func NewMapOne(targetClaims TargetClaimSet) (MapOne, error) {
	if !targetClaims.valid() {
		return MapOne{}, fmt.Errorf("MapOne target claim set is invalid")
	}
	return MapOne{targetClaims: targetClaims}, nil
}

func (MapOne) dispositionVariant() {}

func (MapOne) branchDispositionVariant() {}

func (mapping MapOne) TargetClaims() TargetClaimSet {
	return mapping.targetClaims
}

type RetireHistory struct {
	reason string
}

func NewRetireHistory(reason string) (RetireHistory, error) {
	value, err := requireNarrative("RetireHistory reason", reason)
	if err != nil {
		return RetireHistory{}, err
	}
	return RetireHistory{reason: value}, nil
}

func (RetireHistory) dispositionVariant() {}

func (RetireHistory) branchDispositionVariant() {}

func (retirement RetireHistory) Reason() string {
	return retirement.reason
}

type OutsidePSS struct {
	meaning  string
	carriers OutsideCarrierSet
}

func NewOutsidePSS(meaning string, carriers OutsideCarrierSet) (OutsidePSS, error) {
	value, err := requireNarrative("OutsidePSS meaning", meaning)
	if err != nil {
		return OutsidePSS{}, err
	}
	if !carriers.valid() {
		return OutsidePSS{}, fmt.Errorf("OutsidePSS carrier set is invalid")
	}
	return OutsidePSS{meaning: value, carriers: carriers}, nil
}

func (OutsidePSS) dispositionVariant() {}

func (OutsidePSS) branchDispositionVariant() {}

func (outside OutsidePSS) Meaning() string {
	return outside.meaning
}

func (outside OutsidePSS) Carriers() OutsideCarrierSet {
	return outside.carriers
}

type SplitBranch struct {
	fragment    ExactByteSpan
	disposition BranchDisposition
}

func NewSplitBranch(fragment ExactByteSpan, disposition BranchDisposition) (SplitBranch, error) {
	if !fragment.valid() {
		return SplitBranch{}, fmt.Errorf("split branch fragment is invalid")
	}
	if err := validateBranchDispositionShape(disposition); err != nil {
		return SplitBranch{}, err
	}
	return SplitBranch{fragment: fragment, disposition: disposition}, nil
}

func (branch SplitBranch) Fragment() ExactByteSpan {
	return branch.fragment
}

func (branch SplitBranch) Disposition() BranchDisposition {
	return branch.disposition
}

type SplitOneToMany struct {
	branches []SplitBranch
}

func NewSplitOneToMany(branches []SplitBranch) (SplitOneToMany, error) {
	if len(branches) < 2 {
		return SplitOneToMany{}, fmt.Errorf("SplitOneToMany requires at least two exact branches")
	}
	result := append([]SplitBranch{}, branches...)
	for index, branch := range result {
		if !branch.fragment.valid() {
			return SplitOneToMany{}, fmt.Errorf("split branch %d fragment is invalid", index)
		}
		if err := validateBranchDispositionShape(branch.disposition); err != nil {
			return SplitOneToMany{}, fmt.Errorf("split branch %d: %w", index, err)
		}
	}
	return SplitOneToMany{branches: result}, nil
}

func (SplitOneToMany) dispositionVariant() {}

func (split SplitOneToMany) Branches() []SplitBranch {
	return append([]SplitBranch{}, split.branches...)
}

type SourceDisposition struct {
	source      SourceSectionID
	disposition Disposition
}

func NewSourceDisposition(source SourceSectionID, disposition Disposition) (SourceDisposition, error) {
	if !source.valid() {
		return SourceDisposition{}, fmt.Errorf("source disposition section ID is invalid")
	}
	if err := validateDispositionShape(disposition); err != nil {
		return SourceDisposition{}, err
	}
	return SourceDisposition{source: source, disposition: disposition}, nil
}

func (mapping SourceDisposition) Source() SourceSectionID {
	return mapping.source
}

func (mapping SourceDisposition) Disposition() Disposition {
	return mapping.disposition
}

type OutsideCarrierRegistrationInput struct {
	ID      OutsideCarrierID
	Carrier SourceCarrierID
	Digest  OutsideCarrierDigest
}

type OutsideCarrierRegistration struct {
	id      OutsideCarrierID
	carrier SourceCarrierID
	digest  OutsideCarrierDigest
}

func NewOutsideCarrierRegistration(input OutsideCarrierRegistrationInput) (OutsideCarrierRegistration, error) {
	if !input.ID.valid() {
		return OutsideCarrierRegistration{}, fmt.Errorf("outside carrier registration ID is invalid")
	}
	if !input.Carrier.valid() {
		return OutsideCarrierRegistration{}, fmt.Errorf("outside carrier registration path is invalid")
	}
	if !input.Digest.valid() {
		return OutsideCarrierRegistration{}, fmt.Errorf("outside carrier registration digest is invalid")
	}
	return OutsideCarrierRegistration{id: input.ID, carrier: input.Carrier, digest: input.Digest}, nil
}

func (registration OutsideCarrierRegistration) ID() OutsideCarrierID {
	return registration.id
}

func (registration OutsideCarrierRegistration) Carrier() SourceCarrierID {
	return registration.carrier
}

func (registration OutsideCarrierRegistration) Digest() OutsideCarrierDigest {
	return registration.digest
}

type OutsideCarrierRegistry struct {
	values []OutsideCarrierRegistration
}

func NewOutsideCarrierRegistry(values []OutsideCarrierRegistration) (OutsideCarrierRegistry, error) {
	seenIDs := make(map[string]struct{}, len(values))
	seenCarriers := make(map[string]struct{}, len(values))
	result := make([]OutsideCarrierRegistration, 0, len(values))
	for index, value := range values {
		if !value.id.valid() || !value.carrier.valid() || !value.digest.valid() {
			return OutsideCarrierRegistry{}, fmt.Errorf("outside carrier registration %d is invalid", index)
		}
		key := value.id.String()
		if _, exists := seenIDs[key]; exists {
			return OutsideCarrierRegistry{}, fmt.Errorf("duplicate outside carrier registration %q", key)
		}
		carrierKey := value.carrier.String()
		if _, exists := seenCarriers[carrierKey]; exists {
			return OutsideCarrierRegistry{}, fmt.Errorf(
				"outside carrier path %q is registered under more than one identity",
				carrierKey,
			)
		}
		seenIDs[key] = struct{}{}
		seenCarriers[carrierKey] = struct{}{}
		result = append(result, value)
	}
	return OutsideCarrierRegistry{values: result}, nil
}

func (registry OutsideCarrierRegistry) Values() []OutsideCarrierRegistration {
	return append([]OutsideCarrierRegistration{}, registry.values...)
}

type PacketInput struct {
	ID                 MigrationPacketID
	SchemaVersion      uint32
	Source             SourceManifest
	Target             TargetManifest
	OutsideRegistry    OutsideCarrierRegistry
	SourceDispositions []SourceDisposition
}

type Packet struct {
	id                 MigrationPacketID
	schemaVersion      uint32
	source             SourceManifest
	target             TargetManifest
	outsideRegistry    OutsideCarrierRegistry
	sourceDispositions []SourceDisposition
	lineagePolicy      LineagePolicy
}

func NewPacket(input PacketInput) (Packet, error) {
	if !input.ID.valid() {
		return Packet{}, fmt.Errorf("migration packet ID is invalid")
	}
	if input.SchemaVersion != SchemaVersionV2 {
		return Packet{}, fmt.Errorf("migration packet schema version must be %d", SchemaVersionV2)
	}
	if !input.Source.valid() {
		return Packet{}, fmt.Errorf("migration packet source manifest is invalid")
	}
	if !input.Target.valid() {
		return Packet{}, fmt.Errorf("migration packet target manifest is invalid")
	}
	if len(input.SourceDispositions) == 0 {
		return Packet{}, fmt.Errorf("migration packet requires source dispositions")
	}
	dispositions := append([]SourceDisposition{}, input.SourceDispositions...)
	for index, disposition := range dispositions {
		if !disposition.source.valid() {
			return Packet{}, fmt.Errorf("source disposition %d has invalid source ID", index)
		}
		if err := validateDispositionShape(disposition.disposition); err != nil {
			return Packet{}, fmt.Errorf("source disposition %d: %w", index, err)
		}
	}
	lineagePolicy, err := compileLineagePolicy(
		input.Source,
		input.OutsideRegistry,
		dispositions,
	)
	if err != nil {
		return Packet{}, err
	}
	return Packet{
		id:                 input.ID,
		schemaVersion:      input.SchemaVersion,
		source:             input.Source,
		target:             input.Target,
		outsideRegistry:    input.OutsideRegistry,
		sourceDispositions: dispositions,
		lineagePolicy:      lineagePolicy,
	}, nil
}

func (packet Packet) ID() MigrationPacketID {
	return packet.id
}

func (packet Packet) SchemaVersion() uint32 {
	return packet.schemaVersion
}

func (packet Packet) Source() SourceManifest {
	return packet.source
}

func (packet Packet) Target() TargetManifest {
	return packet.target
}

func (packet Packet) OutsideRegistry() OutsideCarrierRegistry {
	return packet.outsideRegistry
}

func (packet Packet) SourceDispositions() []SourceDisposition {
	return append([]SourceDisposition{}, packet.sourceDispositions...)
}

func (packet Packet) LineagePolicy() LineagePolicy {
	entries := packet.lineagePolicy.Entries()
	return LineagePolicy{schemaVersion: packet.lineagePolicy.schemaVersion, entries: entries}
}

func validateDispositionShape(disposition Disposition) error {
	switch value := disposition.(type) {
	case MapOne:
		if !value.targetClaims.valid() {
			return fmt.Errorf("MapOne target claim set is invalid")
		}
		return nil
	case SplitOneToMany:
		if len(value.branches) < 2 {
			return fmt.Errorf("SplitOneToMany requires at least two branches")
		}
		return nil
	case RetireHistory:
		_, err := requireNarrative("RetireHistory reason", value.reason)
		return err
	case OutsidePSS:
		_, textErr := requireNarrative("OutsidePSS meaning", value.meaning)
		if textErr != nil {
			return textErr
		}
		if !value.carriers.valid() {
			return fmt.Errorf("OutsidePSS carrier set is invalid")
		}
		return nil
	default:
		return fmt.Errorf("unknown disposition variant")
	}
}

func validateBranchDispositionShape(disposition BranchDisposition) error {
	switch value := disposition.(type) {
	case MapOne:
		return validateDispositionShape(value)
	case RetireHistory:
		return validateDispositionShape(value)
	case OutsidePSS:
		return validateDispositionShape(value)
	default:
		return fmt.Errorf("split branch must be MapOne, RetireHistory, or OutsidePSS")
	}
}

func requireNarrative(name, raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" || strings.ContainsFunc(value, unicode.IsControl) {
		return "", fmt.Errorf("%s must be non-empty and contain no control characters", name)
	}
	return value, nil
}
