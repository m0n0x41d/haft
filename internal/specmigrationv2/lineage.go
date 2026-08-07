package specmigrationv2

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const LineageSchemaVersionV1 uint32 = 1

const lineageDigestDomain = "haft.spec-migration-v2.lineage/v1"

var gitCommitOIDPattern = regexp.MustCompile(`^(?:sha1:[0-9a-f]{40}|sha256:[0-9a-f]{64})$`)

type ProjectRootRef struct {
	value string
}

func NewProjectRootRef(raw string) (ProjectRootRef, error) {
	if raw != strings.TrimSpace(raw) {
		return ProjectRootRef{}, fmt.Errorf("project-root ref must be canonical without surrounding whitespace")
	}
	value, err := requireNarrative("project-root ref", raw)
	if err != nil {
		return ProjectRootRef{}, err
	}
	return ProjectRootRef{value: value}, nil
}

func (ref ProjectRootRef) String() string {
	return ref.value
}

func (ref ProjectRootRef) valid() bool {
	return ref.value != "" && ref.value == strings.TrimSpace(ref.value)
}

type GitCommitOID struct {
	value string
}

func NewGitCommitOID(raw string) (GitCommitOID, error) {
	if raw != strings.TrimSpace(raw) || !gitCommitOIDPattern.MatchString(raw) {
		return GitCommitOID{}, fmt.Errorf("git commit OID must use sha1:<40 lowerhex> or sha256:<64 lowerhex>")
	}
	return GitCommitOID{value: raw}, nil
}

func (oid GitCommitOID) String() string {
	return oid.value
}

func (oid GitCommitOID) valid() bool {
	return gitCommitOIDPattern.MatchString(oid.value)
}

type WorktreeDeltaFormat string

const WorktreeDeltaGitBinaryV1 WorktreeDeltaFormat = "git_binary_diff_v1"

type WorktreeDeltaDigest struct {
	value SHA256
}

func NewWorktreeDeltaDigest(raw string) (WorktreeDeltaDigest, error) {
	value, err := NewSHA256(raw)
	if err != nil {
		return WorktreeDeltaDigest{}, err
	}
	return WorktreeDeltaDigest{value: value}, nil
}

func WorktreeDeltaDigestOf(value []byte) WorktreeDeltaDigest {
	return WorktreeDeltaDigest{value: DigestBytes(value)}
}

func (digest WorktreeDeltaDigest) String() string {
	return digest.value.String()
}

func (digest WorktreeDeltaDigest) valid() bool {
	return digest.value.valid()
}

type WorktreeDeltaBinding struct {
	format WorktreeDeltaFormat
	digest WorktreeDeltaDigest
}

func NewWorktreeDeltaBinding(
	format WorktreeDeltaFormat,
	digest WorktreeDeltaDigest,
) (WorktreeDeltaBinding, error) {
	if format != WorktreeDeltaGitBinaryV1 {
		return WorktreeDeltaBinding{}, fmt.Errorf("unsupported worktree delta format %q", format)
	}
	if !digest.valid() {
		return WorktreeDeltaBinding{}, fmt.Errorf("worktree delta digest is invalid")
	}
	return WorktreeDeltaBinding{format: format, digest: digest}, nil
}

func (binding WorktreeDeltaBinding) Format() WorktreeDeltaFormat {
	return binding.format
}

func (binding WorktreeDeltaBinding) Digest() WorktreeDeltaDigest {
	return binding.digest
}

func (binding WorktreeDeltaBinding) valid() bool {
	return binding.format == WorktreeDeltaGitBinaryV1 && binding.digest.valid()
}

type ProvenanceRecordRef struct {
	value string
}

func NewProvenanceRecordRef(raw string) (ProvenanceRecordRef, error) {
	if raw != strings.TrimSpace(raw) {
		return ProvenanceRecordRef{}, fmt.Errorf("provenance-record ref must be canonical without surrounding whitespace")
	}
	value, err := requireNarrative("provenance-record ref", raw)
	if err != nil {
		return ProvenanceRecordRef{}, err
	}
	return ProvenanceRecordRef{value: value}, nil
}

func (ref ProvenanceRecordRef) String() string {
	return ref.value
}

func (ref ProvenanceRecordRef) valid() bool {
	return ref.value != "" && ref.value == strings.TrimSpace(ref.value)
}

type ProvenanceRecordDigest struct {
	value SHA256
}

func NewProvenanceRecordDigest(raw string) (ProvenanceRecordDigest, error) {
	value, err := NewSHA256(raw)
	if err != nil {
		return ProvenanceRecordDigest{}, err
	}
	return ProvenanceRecordDigest{value: value}, nil
}

func ProvenanceRecordDigestOf(value []byte) ProvenanceRecordDigest {
	return ProvenanceRecordDigest{value: DigestBytes(value)}
}

func (digest ProvenanceRecordDigest) String() string {
	return digest.value.String()
}

func (digest ProvenanceRecordDigest) valid() bool {
	return digest.value.valid()
}

type ProvenanceRecordBinding struct {
	ref    ProvenanceRecordRef
	digest ProvenanceRecordDigest
}

func NewProvenanceRecordBinding(
	ref ProvenanceRecordRef,
	digest ProvenanceRecordDigest,
) (ProvenanceRecordBinding, error) {
	if !ref.valid() || !digest.valid() {
		return ProvenanceRecordBinding{}, fmt.Errorf("provenance-record binding is invalid")
	}
	return ProvenanceRecordBinding{ref: ref, digest: digest}, nil
}

func (binding ProvenanceRecordBinding) Ref() ProvenanceRecordRef {
	return binding.ref
}

func (binding ProvenanceRecordBinding) Digest() ProvenanceRecordDigest {
	return binding.digest
}

func (binding ProvenanceRecordBinding) valid() bool {
	return binding.ref.valid() && binding.digest.valid()
}

type SourceEditionOrigin interface {
	sourceEditionOriginVariant()
	ProjectRoot() ProjectRootRef
	Carrier() SourceCarrierID
	DesignatedDigest() SourceDigest
}

type RepositoryEdition struct {
	projectRoot ProjectRootRef
	commitOID   GitCommitOID
	carrier     SourceCarrierID
	bytesDigest SourceDigest
}

func NewRepositoryEdition(
	projectRoot ProjectRootRef,
	commitOID GitCommitOID,
	carrier SourceCarrierID,
	bytesDigest SourceDigest,
) (RepositoryEdition, error) {
	value := RepositoryEdition{
		projectRoot: projectRoot,
		commitOID:   commitOID,
		carrier:     carrier,
		bytesDigest: bytesDigest,
	}
	if !value.valid() {
		return RepositoryEdition{}, fmt.Errorf("repository source edition is invalid")
	}
	return value, nil
}

func (RepositoryEdition) sourceEditionOriginVariant() {}

func (edition RepositoryEdition) ProjectRoot() ProjectRootRef {
	return edition.projectRoot
}

func (edition RepositoryEdition) CommitOID() GitCommitOID {
	return edition.commitOID
}

func (edition RepositoryEdition) Carrier() SourceCarrierID {
	return edition.carrier
}

func (edition RepositoryEdition) DesignatedDigest() SourceDigest {
	return edition.bytesDigest
}

func (edition RepositoryEdition) valid() bool {
	return edition.projectRoot.valid() &&
		edition.commitOID.valid() &&
		edition.carrier.valid() &&
		edition.bytesDigest.valid()
}

type WorkingTreeEdition struct {
	parent           RepositoryEdition
	designatedDigest SourceDigest
	delta            WorktreeDeltaBinding
}

func NewWorkingTreeEdition(
	parent RepositoryEdition,
	designatedDigest SourceDigest,
	delta WorktreeDeltaBinding,
) (WorkingTreeEdition, error) {
	value := WorkingTreeEdition{
		parent:           parent,
		designatedDigest: designatedDigest,
		delta:            delta,
	}
	if !value.valid() {
		return WorkingTreeEdition{}, fmt.Errorf("working-tree source edition is invalid or canonicalizes to a repository edition")
	}
	return value, nil
}

func (WorkingTreeEdition) sourceEditionOriginVariant() {}

func (edition WorkingTreeEdition) ProjectRoot() ProjectRootRef {
	return edition.parent.projectRoot
}

func (edition WorkingTreeEdition) Parent() RepositoryEdition {
	return edition.parent
}

func (edition WorkingTreeEdition) Carrier() SourceCarrierID {
	return edition.parent.carrier
}

func (edition WorkingTreeEdition) DesignatedDigest() SourceDigest {
	return edition.designatedDigest
}

func (edition WorkingTreeEdition) Delta() WorktreeDeltaBinding {
	return edition.delta
}

func (edition WorkingTreeEdition) valid() bool {
	return edition.parent.valid() &&
		edition.designatedDigest.valid() &&
		edition.delta.valid() &&
		!edition.designatedDigest.Equal(edition.parent.bytesDigest)
}

type DesignatedSourceProvenance struct {
	origin           SourceEditionOrigin
	resolutionRecord ProvenanceRecordBinding
}

func NewDesignatedSourceProvenance(
	origin SourceEditionOrigin,
	resolutionRecord ProvenanceRecordBinding,
) (DesignatedSourceProvenance, error) {
	value := DesignatedSourceProvenance{
		origin:           origin,
		resolutionRecord: resolutionRecord,
	}
	if !value.valid() {
		return DesignatedSourceProvenance{}, fmt.Errorf("designated-source provenance is invalid")
	}
	return value, nil
}

func (provenance DesignatedSourceProvenance) Origin() SourceEditionOrigin {
	return provenance.origin
}

func (provenance DesignatedSourceProvenance) ResolutionRecord() ProvenanceRecordBinding {
	return provenance.resolutionRecord
}

func (provenance DesignatedSourceProvenance) valid() bool {
	if !provenance.resolutionRecord.valid() {
		return false
	}
	switch origin := provenance.origin.(type) {
	case RepositoryEdition:
		return origin.valid()
	case WorkingTreeEdition:
		return origin.valid()
	default:
		return false
	}
}

type LineageSubject interface {
	lineageSubjectVariant()
	Source() SourceSectionID
	Span() ExactByteSpan
}

type wholeSourceSection struct {
	source SourceSectionID
	span   ExactByteSpan
}

func (wholeSourceSection) lineageSubjectVariant() {}

func (subject wholeSourceSection) Source() SourceSectionID {
	return subject.source
}

func (subject wholeSourceSection) Span() ExactByteSpan {
	return subject.span
}

type sourceFragment struct {
	source   SourceSectionID
	fragment ExactByteSpan
}

func (sourceFragment) lineageSubjectVariant() {}

func (subject sourceFragment) Source() SourceSectionID {
	return subject.source
}

func (subject sourceFragment) Span() ExactByteSpan {
	return subject.fragment
}

type LineageOutcome interface {
	lineageOutcomeVariant()
}

type MeaningMappedToTargetClaims interface {
	LineageOutcome
	TargetClaims() TargetClaimSet
	meaningMappedToTargetClaimsVariant()
}

type meaningMappedToTargetClaims struct {
	targetClaims TargetClaimSet
}

func (meaningMappedToTargetClaims) lineageOutcomeVariant()              {}
func (meaningMappedToTargetClaims) meaningMappedToTargetClaimsVariant() {}

func (outcome meaningMappedToTargetClaims) TargetClaims() TargetClaimSet {
	return outcome.targetClaims
}

type RetainedAsHistoryOnly interface {
	LineageOutcome
	ArchiveCarrier() ArchiveCarrierID
	SourceEditionDigest() SourceDigest
	Reason() string
	retainedAsHistoryOnlyVariant()
}

type retainedAsHistoryOnly struct {
	archive       ArchiveCarrierID
	sourceEdition SourceDigest
	reason        string
}

func (retainedAsHistoryOnly) lineageOutcomeVariant()        {}
func (retainedAsHistoryOnly) retainedAsHistoryOnlyVariant() {}
func (outcome retainedAsHistoryOnly) ArchiveCarrier() ArchiveCarrierID {
	return outcome.archive
}

func (outcome retainedAsHistoryOnly) SourceEditionDigest() SourceDigest {
	return outcome.sourceEdition
}

func (outcome retainedAsHistoryOnly) Reason() string {
	return outcome.reason
}

type ResolvedOutsideCarrierBinding struct {
	id      OutsideCarrierID
	carrier SourceCarrierID
	digest  OutsideCarrierDigest
}

func (binding ResolvedOutsideCarrierBinding) ID() OutsideCarrierID {
	return binding.id
}

func (binding ResolvedOutsideCarrierBinding) Carrier() SourceCarrierID {
	return binding.carrier
}

func (binding ResolvedOutsideCarrierBinding) Digest() OutsideCarrierDigest {
	return binding.digest
}

type ContinuesOutsidePSS interface {
	LineageOutcome
	Meaning() string
	Carriers() OutsideCarrierSet
	ResolvedCarriers() []ResolvedOutsideCarrierBinding
	continuesOutsidePSSVariant()
}

type continuesOutsidePSS struct {
	meaning  string
	carriers OutsideCarrierSet
	resolved []ResolvedOutsideCarrierBinding
}

func (continuesOutsidePSS) lineageOutcomeVariant()      {}
func (continuesOutsidePSS) continuesOutsidePSSVariant() {}
func (outcome continuesOutsidePSS) Carriers() OutsideCarrierSet {
	return outcome.carriers
}

func (outcome continuesOutsidePSS) Meaning() string {
	return outcome.meaning
}

func (outcome continuesOutsidePSS) ResolvedCarriers() []ResolvedOutsideCarrierBinding {
	return append([]ResolvedOutsideCarrierBinding{}, outcome.resolved...)
}

type LineageEntry struct {
	subject LineageSubject
	outcome LineageOutcome
}

func (entry LineageEntry) Subject() LineageSubject {
	return entry.subject
}

func (entry LineageEntry) Outcome() LineageOutcome {
	return entry.outcome
}

type LineagePolicy struct {
	schemaVersion uint32
	entries       []LineageEntry
}

func (policy LineagePolicy) SchemaVersion() uint32 {
	return policy.schemaVersion
}

func (policy LineagePolicy) Entries() []LineageEntry {
	return append([]LineageEntry{}, policy.entries...)
}

func (policy LineagePolicy) valid() bool {
	return policy.schemaVersion == LineageSchemaVersionV1 && len(policy.entries) > 0
}

type LineagePolicyDigest struct {
	value SHA256
}

func (digest LineagePolicyDigest) String() string {
	return digest.value.String()
}

func (digest LineagePolicyDigest) valid() bool {
	return digest.value.valid()
}

func compileLineagePolicy(
	source SourceManifest,
	registry OutsideCarrierRegistry,
	dispositions []SourceDisposition,
) (LineagePolicy, error) {
	sections := make(map[string]SourceSection, len(source.sections))
	for _, section := range source.sections {
		sections[section.id.String()] = section
	}
	entries := []LineageEntry{}
	for _, sourceDisposition := range dispositions {
		section, exists := sections[sourceDisposition.source.String()]
		if !exists {
			return LineagePolicy{}, fmt.Errorf(
				"cannot compile lineage for unknown source section %q",
				sourceDisposition.source.String(),
			)
		}
		compiled, err := compileDispositionLineage(
			source,
			registry,
			section,
			sourceDisposition.disposition,
		)
		if err != nil {
			return LineagePolicy{}, err
		}
		entries = append(entries, compiled...)
	}
	policy := LineagePolicy{schemaVersion: LineageSchemaVersionV1, entries: entries}
	if !policy.valid() {
		return LineagePolicy{}, fmt.Errorf("compiled lineage policy is empty")
	}
	return policy, nil
}

func compileDispositionLineage(
	source SourceManifest,
	registry OutsideCarrierRegistry,
	section SourceSection,
	disposition Disposition,
) ([]LineageEntry, error) {
	whole := wholeSourceSection{source: section.id, span: section.span}
	switch value := disposition.(type) {
	case MapOne:
		outcome := meaningMappedToTargetClaims(value)
		return []LineageEntry{{subject: whole, outcome: outcome}}, nil
	case RetireHistory:
		outcome := retainedAsHistoryOnly{
			archive:       source.archive.carrier,
			sourceEdition: source.digest,
			reason:        value.reason,
		}
		return []LineageEntry{{subject: whole, outcome: outcome}}, nil
	case OutsidePSS:
		outcome := compileOutsideLineage(value, registry)
		return []LineageEntry{{subject: whole, outcome: outcome}}, nil
	case SplitOneToMany:
		return compileSplitLineage(source, registry, section.id, value)
	default:
		return nil, fmt.Errorf("cannot compile lineage for unknown disposition variant")
	}
}

func compileSplitLineage(
	source SourceManifest,
	registry OutsideCarrierRegistry,
	sectionID SourceSectionID,
	split SplitOneToMany,
) ([]LineageEntry, error) {
	entries := make([]LineageEntry, 0, len(split.branches))
	for _, branch := range split.branches {
		subject := sourceFragment{source: sectionID, fragment: branch.fragment}
		outcome, err := compileBranchOutcome(source, registry, branch.disposition)
		if err != nil {
			return nil, err
		}
		entries = append(entries, LineageEntry{subject: subject, outcome: outcome})
	}
	return entries, nil
}

func compileBranchOutcome(
	source SourceManifest,
	registry OutsideCarrierRegistry,
	disposition BranchDisposition,
) (LineageOutcome, error) {
	switch value := disposition.(type) {
	case MapOne:
		return meaningMappedToTargetClaims(value), nil
	case RetireHistory:
		return retainedAsHistoryOnly{
			archive:       source.archive.carrier,
			sourceEdition: source.digest,
			reason:        value.reason,
		}, nil
	case OutsidePSS:
		return compileOutsideLineage(value, registry), nil
	default:
		return nil, fmt.Errorf("cannot compile lineage for unknown branch disposition variant")
	}
}

func compileOutsideLineage(
	outside OutsidePSS,
	registry OutsideCarrierRegistry,
) continuesOutsidePSS {
	registrations := make(map[string]OutsideCarrierRegistration, len(registry.values))
	for _, registration := range registry.values {
		registrations[registration.id.String()] = registration
	}
	resolved := make([]ResolvedOutsideCarrierBinding, 0, len(outside.carriers.values))
	for _, id := range outside.carriers.values {
		registration, found := registrations[id.String()]
		if !found {
			continue
		}
		resolved = append(
			resolved,
			ResolvedOutsideCarrierBinding(registration),
		)
	}
	sort.Slice(resolved, func(left, right int) bool {
		return resolvedOutsideCarrierSortKey(resolved[left]) < resolvedOutsideCarrierSortKey(resolved[right])
	})
	return continuesOutsidePSS{
		meaning:  outside.meaning,
		carriers: outside.carriers,
		resolved: resolved,
	}
}

type lineageDigestWriter struct {
	hash hash.Hash
}

func newLineageDigestWriter() lineageDigestWriter {
	writer := lineageDigestWriter{hash: sha256.New()}
	writer.add(lineageDigestDomain)
	return writer
}

func (writer lineageDigestWriter) add(value string) {
	var size [8]byte
	binary.BigEndian.PutUint64(size[:], uint64(len(value)))
	_, _ = writer.hash.Write(size[:])
	_, _ = writer.hash.Write([]byte(value))
}

func (writer lineageDigestWriter) digest() LineagePolicyDigest {
	value := "sha256:" + hex.EncodeToString(writer.hash.Sum(nil))
	return LineagePolicyDigest{value: SHA256{value: value}}
}

func LineagePolicyDigestOf(policy LineagePolicy) (LineagePolicyDigest, error) {
	if !policy.valid() {
		return LineagePolicyDigest{}, fmt.Errorf("cannot digest an invalid lineage policy")
	}
	writer := newLineageDigestWriter()
	writer.add(strconv.FormatUint(uint64(policy.schemaVersion), 10))
	entries := append([]LineageEntry{}, policy.entries...)
	sort.Slice(entries, func(left, right int) bool {
		return lineageEntrySortKey(entries[left]) < lineageEntrySortKey(entries[right])
	})
	writer.add(strconv.Itoa(len(entries)))
	for _, entry := range entries {
		if err := addLineageEntryDigest(writer, entry); err != nil {
			return LineagePolicyDigest{}, err
		}
	}
	return writer.digest(), nil
}

func lineageEntrySortKey(entry LineageEntry) string {
	span := entry.subject.Span()
	return entry.subject.Source().String() + "\x00" +
		fmt.Sprintf("%020d\x00%020d\x00", span.Start(), span.Length().Value()) +
		lineageOutcomeSortKey(entry.outcome)
}

func lineageOutcomeSortKey(outcome LineageOutcome) string {
	switch value := outcome.(type) {
	case meaningMappedToTargetClaims:
		claims := value.targetClaims.Values()
		parts := make([]string, 0, len(claims))
		for _, claim := range claims {
			parts = append(parts, claim.String())
		}
		sort.Strings(parts)
		return "mapped\x00" + strings.Join(parts, "\x00")
	case retainedAsHistoryOnly:
		return "history\x00" +
			value.archive.String() + "\x00" +
			value.sourceEdition.String() + "\x00" +
			value.reason
	case continuesOutsidePSS:
		carriers := value.carriers.Values()
		parts := make([]string, 0, len(carriers))
		for _, carrier := range carriers {
			parts = append(parts, carrier.String())
		}
		sort.Strings(parts)
		resolved := append([]ResolvedOutsideCarrierBinding{}, value.resolved...)
		sort.Slice(resolved, func(left, right int) bool {
			return resolvedOutsideCarrierSortKey(resolved[left]) < resolvedOutsideCarrierSortKey(resolved[right])
		})
		resolvedParts := make([]string, 0, len(resolved))
		for _, binding := range resolved {
			resolvedParts = append(resolvedParts, resolvedOutsideCarrierSortKey(binding))
		}
		return "outside\x00" +
			value.meaning + "\x00" +
			strings.Join(parts, "\x00") + "\x00" +
			strings.Join(resolvedParts, "\x00")
	default:
		return "unknown"
	}
}

func addLineageEntryDigest(writer lineageDigestWriter, entry LineageEntry) error {
	switch entry.subject.(type) {
	case wholeSourceSection:
		writer.add("whole_source_section")
	case sourceFragment:
		writer.add("source_fragment")
	default:
		return fmt.Errorf("cannot digest unknown lineage subject variant")
	}
	writer.add(entry.subject.Source().String())
	span := entry.subject.Span()
	writer.add(strconv.FormatUint(span.Start(), 10))
	writer.add(strconv.FormatUint(span.Length().Value(), 10))
	writer.add(span.Digest().String())
	switch outcome := entry.outcome.(type) {
	case meaningMappedToTargetClaims:
		writer.add("meaning_mapped_to_target_claims")
		claims := outcome.targetClaims.Values()
		sort.Slice(claims, func(left, right int) bool {
			return claims[left].String() < claims[right].String()
		})
		writer.add(strconv.Itoa(len(claims)))
		for _, claim := range claims {
			writer.add(claim.String())
		}
		return nil
	case retainedAsHistoryOnly:
		writer.add("retained_as_history_only")
		writer.add(outcome.archive.String())
		writer.add(outcome.sourceEdition.String())
		writer.add(outcome.reason)
		return nil
	case continuesOutsidePSS:
		writer.add("continues_outside_pss")
		writer.add(outcome.meaning)
		carriers := outcome.carriers.Values()
		sort.Slice(carriers, func(left, right int) bool {
			return carriers[left].String() < carriers[right].String()
		})
		writer.add(strconv.Itoa(len(carriers)))
		for _, carrier := range carriers {
			writer.add(carrier.String())
		}
		resolved := append([]ResolvedOutsideCarrierBinding{}, outcome.resolved...)
		sort.Slice(resolved, func(left, right int) bool {
			return resolvedOutsideCarrierSortKey(resolved[left]) < resolvedOutsideCarrierSortKey(resolved[right])
		})
		writer.add(strconv.Itoa(len(resolved)))
		for _, binding := range resolved {
			writer.add(binding.id.String())
			writer.add(binding.carrier.String())
			writer.add(binding.digest.String())
		}
		return nil
	default:
		return fmt.Errorf("cannot digest unknown lineage outcome variant")
	}
}

func resolvedOutsideCarrierSortKey(binding ResolvedOutsideCarrierBinding) string {
	return binding.id.String() + "\x00" +
		binding.carrier.String() + "\x00" +
		binding.digest.String()
}
