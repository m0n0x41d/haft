package specmigrationv2

import (
	"fmt"
	"sort"

	profileadmissionsqlite "github.com/m0n0x41d/haft/internal/profileadmission/sqlite"
	"github.com/m0n0x41d/haft/internal/projectprofile"
)

type SourceSnapshotInput struct {
	Carrier SourceCarrierID
	Bytes   []byte
}

type SourceSnapshot struct {
	carrier SourceCarrierID
	bytes   []byte
}

func NewSourceSnapshot(input SourceSnapshotInput) (SourceSnapshot, error) {
	if !input.Carrier.valid() {
		return SourceSnapshot{}, fmt.Errorf("source snapshot carrier ID is invalid")
	}
	return SourceSnapshot{carrier: input.Carrier, bytes: append([]byte{}, input.Bytes...)}, nil
}

func (snapshot SourceSnapshot) Carrier() SourceCarrierID {
	return snapshot.carrier
}

func (snapshot SourceSnapshot) Bytes() []byte {
	return append([]byte{}, snapshot.bytes...)
}

type TargetSnapshotInput struct {
	Carrier TargetCarrierID
	Bytes   []byte
}

type TargetSnapshot struct {
	carrier TargetCarrierID
	bytes   []byte
}

func NewTargetSnapshot(input TargetSnapshotInput) (TargetSnapshot, error) {
	if !input.Carrier.valid() {
		return TargetSnapshot{}, fmt.Errorf("target snapshot carrier ID is invalid")
	}
	return TargetSnapshot{carrier: input.Carrier, bytes: append([]byte{}, input.Bytes...)}, nil
}

func (snapshot TargetSnapshot) Carrier() TargetCarrierID {
	return snapshot.carrier
}

func (snapshot TargetSnapshot) Bytes() []byte {
	return append([]byte{}, snapshot.bytes...)
}

type TargetClaimCatalogInput struct {
	Carrier TargetCarrierID
	Bytes   []byte
}

type TargetClaimCatalog struct {
	carrier TargetCarrierID
	digest  TargetDigest
	claims  []TargetAtomicClaimID
}

func NewTargetClaimCatalog(input TargetClaimCatalogInput) (TargetClaimCatalog, error) {
	if !input.Carrier.valid() {
		return TargetClaimCatalog{}, fmt.Errorf("target claim catalog carrier ID is invalid")
	}
	claims, err := parseTargetClaims(input.Bytes)
	if err != nil {
		return TargetClaimCatalog{}, err
	}
	digest := TargetDigestOf(input.Bytes)
	return TargetClaimCatalog{carrier: input.Carrier, digest: digest, claims: claims}, nil
}

func (catalog TargetClaimCatalog) Carrier() TargetCarrierID {
	return catalog.carrier
}

func (catalog TargetClaimCatalog) Digest() TargetDigest {
	return catalog.digest
}

func (catalog TargetClaimCatalog) Claims() []TargetAtomicClaimID {
	return append([]TargetAtomicClaimID{}, catalog.claims...)
}

type OutsideCarrierSnapshotInput struct {
	ID      OutsideCarrierID
	Carrier SourceCarrierID
	Bytes   []byte
}

type OutsideCarrierSnapshot struct {
	id      OutsideCarrierID
	carrier SourceCarrierID
	bytes   []byte
}

func NewOutsideCarrierSnapshot(input OutsideCarrierSnapshotInput) (OutsideCarrierSnapshot, error) {
	if !input.ID.valid() {
		return OutsideCarrierSnapshot{}, fmt.Errorf("outside carrier snapshot ID is invalid")
	}
	if !input.Carrier.valid() {
		return OutsideCarrierSnapshot{}, fmt.Errorf("outside carrier snapshot path is invalid")
	}
	return OutsideCarrierSnapshot{
		id:      input.ID,
		carrier: input.Carrier,
		bytes:   append([]byte{}, input.Bytes...),
	}, nil
}

func (snapshot OutsideCarrierSnapshot) ID() OutsideCarrierID {
	return snapshot.id
}

func (snapshot OutsideCarrierSnapshot) Carrier() SourceCarrierID {
	return snapshot.carrier
}

func (snapshot OutsideCarrierSnapshot) Bytes() []byte {
	return append([]byte{}, snapshot.bytes...)
}

type OutsideCarrierSnapshots struct {
	values []OutsideCarrierSnapshot
}

func NewOutsideCarrierSnapshots(values []OutsideCarrierSnapshot) (OutsideCarrierSnapshots, error) {
	seenIDs := make(map[string]struct{}, len(values))
	seenCarriers := make(map[string]struct{}, len(values))
	result := make([]OutsideCarrierSnapshot, 0, len(values))
	for index, value := range values {
		if !value.id.valid() || !value.carrier.valid() {
			return OutsideCarrierSnapshots{}, fmt.Errorf("outside carrier snapshot %d is invalid", index)
		}
		key := value.id.String()
		if _, exists := seenIDs[key]; exists {
			return OutsideCarrierSnapshots{}, fmt.Errorf("duplicate outside carrier snapshot %q", key)
		}
		carrierKey := value.carrier.String()
		if _, exists := seenCarriers[carrierKey]; exists {
			return OutsideCarrierSnapshots{}, fmt.Errorf(
				"outside carrier path %q has more than one observed snapshot identity",
				carrierKey,
			)
		}
		seenIDs[key] = struct{}{}
		seenCarriers[carrierKey] = struct{}{}
		result = append(result, value)
	}
	return OutsideCarrierSnapshots{values: result}, nil
}

func (snapshots OutsideCarrierSnapshots) Values() []OutsideCarrierSnapshot {
	return append([]OutsideCarrierSnapshot{}, snapshots.values...)
}

type StructuralRequestInput struct {
	Packet           Packet
	ProjectRoot      ProjectRootRef
	Source           SourceSnapshot
	Target           TargetSnapshot
	TargetClaims     TargetClaimCatalog
	OutsideSnapshots OutsideCarrierSnapshots
}

type StructuralRequest struct {
	packet           Packet
	projectRoot      ProjectRootRef
	source           SourceSnapshot
	target           TargetSnapshot
	targetClaims     TargetClaimCatalog
	outsideSnapshots OutsideCarrierSnapshots
}

func NewStructuralRequest(input StructuralRequestInput) (StructuralRequest, error) {
	if !input.Packet.id.valid() ||
		input.Packet.schemaVersion != SchemaVersionV2 ||
		!input.Packet.source.valid() ||
		!input.Packet.target.valid() ||
		!input.Packet.lineagePolicy.valid() ||
		len(input.Packet.sourceDispositions) == 0 {
		return StructuralRequest{}, fmt.Errorf("structural-analysis packet is invalid")
	}
	if !input.ProjectRoot.valid() {
		return StructuralRequest{}, fmt.Errorf("structural-analysis project root is invalid")
	}
	if !input.Source.carrier.valid() {
		return StructuralRequest{}, fmt.Errorf("structural-analysis source snapshot is invalid")
	}
	if !input.Target.carrier.valid() {
		return StructuralRequest{}, fmt.Errorf("structural-analysis target snapshot is invalid")
	}
	if !input.TargetClaims.carrier.valid() || !input.TargetClaims.digest.valid() {
		return StructuralRequest{}, fmt.Errorf("structural-analysis target claim catalog is invalid")
	}
	if _, err := NewOutsideCarrierSnapshots(input.OutsideSnapshots.values); err != nil {
		return StructuralRequest{}, fmt.Errorf("structural-analysis outside snapshots are invalid: %w", err)
	}
	return StructuralRequest{
		packet:           input.Packet,
		projectRoot:      input.ProjectRoot,
		source:           cloneSourceSnapshot(input.Source),
		target:           cloneTargetSnapshot(input.Target),
		targetClaims:     cloneTargetClaimCatalog(input.TargetClaims),
		outsideSnapshots: cloneOutsideCarrierSnapshots(input.OutsideSnapshots),
	}, nil
}

type DryRunRequestInput struct {
	Packet           Packet
	ProjectRoot      ProjectRootRef
	Profile          projectprofile.ConfiguredProjectProfile
	Review           MigrationReviewResolution
	Source           SourceSnapshot
	Target           TargetSnapshot
	TargetClaims     TargetClaimCatalog
	OutsideSnapshots OutsideCarrierSnapshots
}

type DryRunRequest struct {
	structural             StructuralRequest
	profile                projectprofile.ConfiguredProjectProfile
	canonicalApplicability profileadmissionsqlite.SoftwareSystemSpecMigrationApplicability
	review                 MigrationReviewResolution
	profileBasis           dryRunProfileBasis
}

type dryRunProfileBasis string

const (
	dryRunLegacyProfileBasis    dryRunProfileBasis = "legacy_profile"
	dryRunCanonicalProfileBasis dryRunProfileBasis = "canonical_profile"
)

func NewDryRunRequest(input DryRunRequestInput) (DryRunRequest, error) {
	if err := validateMigrationReviewResolution(input.Review); err != nil {
		return DryRunRequest{}, err
	}
	structural, err := NewStructuralRequest(StructuralRequestInput{
		Packet:           input.Packet,
		ProjectRoot:      input.ProjectRoot,
		Source:           input.Source,
		Target:           input.Target,
		TargetClaims:     input.TargetClaims,
		OutsideSnapshots: input.OutsideSnapshots,
	})
	if err != nil {
		return DryRunRequest{}, err
	}
	return DryRunRequest{
		structural:   structural,
		profile:      input.Profile,
		review:       input.Review,
		profileBasis: dryRunLegacyProfileBasis,
	}, nil
}

// CanonicalDryRunRequestInput supplies applicability resolved from the
// canonical SQLite ledger. This request remains write-free.
type CanonicalDryRunRequestInput struct {
	Packet               Packet
	ProjectRoot          ProjectRootRef
	ProfileApplicability profileadmissionsqlite.SoftwareSystemSpecMigrationApplicability
	Review               MigrationReviewResolution
	Source               SourceSnapshot
	Target               TargetSnapshot
	TargetClaims         TargetClaimCatalog
	OutsideSnapshots     OutsideCarrierSnapshots
}

func NewCanonicalDryRunRequest(
	input CanonicalDryRunRequestInput,
) (DryRunRequest, error) {
	if !input.ProfileApplicability.Valid() {
		return DryRunRequest{}, fmt.Errorf("canonical dry-run profile applicability is invalid")
	}
	structural, err := NewStructuralRequest(StructuralRequestInput{
		Packet:           input.Packet,
		ProjectRoot:      input.ProjectRoot,
		Source:           input.Source,
		Target:           input.Target,
		TargetClaims:     input.TargetClaims,
		OutsideSnapshots: input.OutsideSnapshots,
	})
	if err != nil {
		return DryRunRequest{}, err
	}
	applicabilityRoot := canonicalApplicabilityProjectRoot(input.ProfileApplicability)
	if applicabilityRoot != input.ProjectRoot.String() {
		return DryRunRequest{}, fmt.Errorf("canonical dry-run applicability root does not match structural project root")
	}
	if err := validateMigrationReviewResolution(input.Review); err != nil {
		return DryRunRequest{}, err
	}
	return DryRunRequest{
		structural:             structural,
		canonicalApplicability: input.ProfileApplicability,
		review:                 input.Review,
		profileBasis:           dryRunCanonicalProfileBasis,
	}, nil
}

func canonicalApplicabilityProjectRoot(
	applicability profileadmissionsqlite.SoftwareSystemSpecMigrationApplicability,
) string {
	if required, ok := applicability.Required(); ok {
		return required.ProjectRoot().String()
	}
	if notApplicable, ok := applicability.NotApplicable(); ok {
		return notApplicable.ProjectRoot().String()
	}
	if underdetermined, ok := applicability.Underdetermined(); ok {
		return underdetermined.ProjectRoot().String()
	}
	return ""
}

func validateMigrationReviewResolution(resolution MigrationReviewResolution) error {
	switch value := resolution.(type) {
	case pendingMigrationReview:
		missing := value.missing
		if !missing.valid() {
			return fmt.Errorf("dry-run pending migration review is invalid")
		}
		return nil
	case admittedMigrationReview:
		return validateAdmittedMigrationReview(value)
	default:
		return fmt.Errorf("dry-run migration review resolution is invalid")
	}
}

type DiagnosticCode string

const (
	DiagnosticSourceCarrierMismatch        DiagnosticCode = "source_carrier_mismatch"
	DiagnosticSourceLengthMismatch         DiagnosticCode = "source_length_mismatch"
	DiagnosticSourceDigestMismatch         DiagnosticCode = "source_digest_mismatch"
	DiagnosticSourceProvenanceRootMismatch DiagnosticCode = "source_provenance_root_mismatch"
	DiagnosticArchiveDigestMismatch        DiagnosticCode = "archive_digest_mismatch"
	DiagnosticCarrierCollision             DiagnosticCode = "carrier_collision"
	DiagnosticTargetCarrierMismatch        DiagnosticCode = "target_carrier_mismatch"
	DiagnosticTargetLengthMismatch         DiagnosticCode = "target_length_mismatch"
	DiagnosticTargetDigestMismatch         DiagnosticCode = "target_digest_mismatch"
	DiagnosticTargetCatalogCarrierMismatch DiagnosticCode = "target_catalog_carrier_mismatch"
	DiagnosticTargetCatalogDigestMismatch  DiagnosticCode = "target_catalog_digest_mismatch"
	DiagnosticDuplicateSourceSection       DiagnosticCode = "duplicate_source_section"
	DiagnosticSourceInventoryParseFailed   DiagnosticCode = "source_inventory_parse_failed"
	DiagnosticSourceInventoryMissing       DiagnosticCode = "source_inventory_missing"
	DiagnosticSourceInventoryUnexpected    DiagnosticCode = "source_inventory_unexpected"
	DiagnosticSourceInventorySpanMismatch  DiagnosticCode = "source_inventory_span_mismatch"
	DiagnosticSourceSectionOutOfBounds     DiagnosticCode = "source_section_out_of_bounds"
	DiagnosticSourceSectionDigestMismatch  DiagnosticCode = "source_section_digest_mismatch"
	DiagnosticSourceSectionOverlap         DiagnosticCode = "source_section_overlap"
	DiagnosticDuplicateSourceDisposition   DiagnosticCode = "duplicate_source_disposition"
	DiagnosticMissingSourceDisposition     DiagnosticCode = "missing_source_disposition"
	DiagnosticUnknownSourceDisposition     DiagnosticCode = "unknown_source_disposition"
	DiagnosticTargetClaimMissing           DiagnosticCode = "target_claim_missing"
	DiagnosticSplitFragmentOutOfBounds     DiagnosticCode = "split_fragment_out_of_bounds"
	DiagnosticSplitFragmentDigestMismatch  DiagnosticCode = "split_fragment_digest_mismatch"
	DiagnosticSplitFragmentGap             DiagnosticCode = "split_fragment_gap"
	DiagnosticSplitFragmentOverlap         DiagnosticCode = "split_fragment_overlap"
	DiagnosticOutsideCarrierUnregistered   DiagnosticCode = "outside_carrier_unregistered"
	DiagnosticOutsideCarrierUnresolved     DiagnosticCode = "outside_carrier_unresolved"
	DiagnosticOutsideCarrierPathMismatch   DiagnosticCode = "outside_carrier_path_mismatch"
	DiagnosticOutsideCarrierDigestMismatch DiagnosticCode = "outside_carrier_digest_mismatch"
	DiagnosticReviewPacketDigestMismatch   DiagnosticCode = "review_packet_digest_mismatch"
	DiagnosticReviewProjectRootMismatch    DiagnosticCode = "review_project_root_mismatch"
	DiagnosticReviewSourceCarrierMismatch  DiagnosticCode = "review_source_carrier_mismatch"
	DiagnosticReviewSourceDigestMismatch   DiagnosticCode = "review_source_digest_mismatch"
	DiagnosticReviewTargetCarrierMismatch  DiagnosticCode = "review_target_carrier_mismatch"
	DiagnosticReviewTargetDigestMismatch   DiagnosticCode = "review_target_digest_mismatch"
	DiagnosticInvalidCoreVariant           DiagnosticCode = "invalid_core_variant"
)

type Diagnostic struct {
	code    DiagnosticCode
	subject string
	detail  string
}

func newDiagnostic(code DiagnosticCode, subject, detail string) Diagnostic {
	return Diagnostic{code: code, subject: subject, detail: detail}
}

func (diagnostic Diagnostic) Code() DiagnosticCode {
	return diagnostic.code
}

func (diagnostic Diagnostic) Subject() string {
	return diagnostic.subject
}

func (diagnostic Diagnostic) Detail() string {
	return diagnostic.detail
}

type DiagnosticSet struct {
	values []Diagnostic
}

func newDiagnosticSet(values []Diagnostic) DiagnosticSet {
	result := append([]Diagnostic{}, values...)
	sort.Slice(result, func(left, right int) bool {
		leftKey := string(result[left].code) + "\x00" + result[left].subject + "\x00" + result[left].detail
		rightKey := string(result[right].code) + "\x00" + result[right].subject + "\x00" + result[right].detail
		return leftKey < rightKey
	})
	return DiagnosticSet{values: result}
}

func (set DiagnosticSet) Values() []Diagnostic {
	return append([]Diagnostic{}, set.values...)
}

type DryRunResult interface {
	dryRunResultVariant()
}

// StructuralAnalysisResult proves only deterministic packet/carrier
// mechanics. It carries no project-profile applicability, human review, or
// apply authority.
type StructuralAnalysisResult interface {
	structuralAnalysisResultVariant()
}

type ValidAnalysis interface {
	StructuralAnalysisResult
	Analysis() StructuralAnalysis
	validAnalysisVariant()
}

type validAnalysis struct {
	analysis StructuralAnalysis
}

func (validAnalysis) structuralAnalysisResultVariant() {}
func (validAnalysis) validAnalysisVariant()            {}

func (result validAnalysis) Analysis() StructuralAnalysis {
	return result.analysis
}

type InvalidDiagnostics interface {
	StructuralAnalysisResult
	Diagnostics() DiagnosticSet
	invalidDiagnosticsVariant()
}

type invalidDiagnostics struct {
	diagnostics DiagnosticSet
}

func (invalidDiagnostics) structuralAnalysisResultVariant() {}
func (invalidDiagnostics) invalidDiagnosticsVariant()       {}

func (result invalidDiagnostics) Diagnostics() DiagnosticSet {
	return result.diagnostics
}

// StructuralAnalysis proves only that the exact packet and observed carriers
// satisfy the pure migration structure. It cannot authorize an apply
// operation.
type StructuralAnalysis interface {
	structuralAnalysisVariant()
	PacketID() MigrationPacketID
	PacketDigest() PacketDigest
	SourceCarrier() SourceCarrierID
	SourceDigest() SourceDigest
	TargetCarrier() TargetCarrierID
	TargetDigest() TargetDigest
	ArchiveCarrier() ArchiveCarrierID
	SourceProvenance() DesignatedSourceProvenance
	LineagePolicy() LineagePolicy
	LineagePolicyDigest() LineagePolicyDigest
	DispositionCount() int
}

type structuralAnalysis struct {
	packetID         MigrationPacketID
	packetDigest     PacketDigest
	sourceCarrier    SourceCarrierID
	sourceDigest     SourceDigest
	targetCarrier    TargetCarrierID
	targetDigest     TargetDigest
	archiveCarrier   ArchiveCarrierID
	sourceProvenance DesignatedSourceProvenance
	lineagePolicy    LineagePolicy
	lineageDigest    LineagePolicyDigest
	dispositionCount int
}

func (structuralAnalysis) structuralAnalysisVariant() {}

func (analysis structuralAnalysis) PacketID() MigrationPacketID {
	return analysis.packetID
}

func (analysis structuralAnalysis) PacketDigest() PacketDigest {
	return analysis.packetDigest
}

func (analysis structuralAnalysis) SourceCarrier() SourceCarrierID {
	return analysis.sourceCarrier
}

func (analysis structuralAnalysis) SourceDigest() SourceDigest {
	return analysis.sourceDigest
}

func (analysis structuralAnalysis) TargetCarrier() TargetCarrierID {
	return analysis.targetCarrier
}

func (analysis structuralAnalysis) TargetDigest() TargetDigest {
	return analysis.targetDigest
}

func (analysis structuralAnalysis) ArchiveCarrier() ArchiveCarrierID {
	return analysis.archiveCarrier
}

func (analysis structuralAnalysis) SourceProvenance() DesignatedSourceProvenance {
	return analysis.sourceProvenance
}

func (analysis structuralAnalysis) LineagePolicy() LineagePolicy {
	entries := analysis.lineagePolicy.Entries()
	return LineagePolicy{schemaVersion: analysis.lineagePolicy.schemaVersion, entries: entries}
}

func (analysis structuralAnalysis) LineagePolicyDigest() LineagePolicyDigest {
	return analysis.lineageDigest
}

func (analysis structuralAnalysis) DispositionCount() int {
	return analysis.dispositionCount
}

type Underdetermined interface {
	DryRunResult
	Applicability() projectprofile.Underdetermined
	underdeterminedVariant()
}

type underdetermined struct {
	applicability projectprofile.Underdetermined
}

func (underdetermined) dryRunResultVariant()    {}
func (underdetermined) underdeterminedVariant() {}

func (result underdetermined) Applicability() projectprofile.Underdetermined {
	return result.applicability
}

type CanonicalUnderdetermined interface {
	DryRunResult
	Applicability() profileadmissionsqlite.SoftwareSystemSpecMigrationUnderdeterminedValue
	canonicalUnderdeterminedVariant()
}

type canonicalUnderdetermined struct {
	applicability profileadmissionsqlite.SoftwareSystemSpecMigrationUnderdeterminedValue
}

func (canonicalUnderdetermined) dryRunResultVariant()             {}
func (canonicalUnderdetermined) canonicalUnderdeterminedVariant() {}

func (result canonicalUnderdetermined) Applicability() profileadmissionsqlite.SoftwareSystemSpecMigrationUnderdeterminedValue {
	return result.applicability
}

type NotApplicable interface {
	DryRunResult
	Applicability() profileadmissionsqlite.SoftwareSystemSpecMigrationNotApplicableValue
	notApplicableVariant()
}

type notApplicable struct {
	applicability profileadmissionsqlite.SoftwareSystemSpecMigrationNotApplicableValue
}

func (notApplicable) dryRunResultVariant()  {}
func (notApplicable) notApplicableVariant() {}

func (result notApplicable) Applicability() profileadmissionsqlite.SoftwareSystemSpecMigrationNotApplicableValue {
	return result.applicability
}

type PendingReview interface {
	DryRunResult
	Applicability() profileadmissionsqlite.SoftwareSystemSpecMigrationRequired
	MissingBasis() ReviewMissingBasisSet
	pendingReviewVariant()
}

type pendingReview struct {
	applicability profileadmissionsqlite.SoftwareSystemSpecMigrationRequired
	missing       ReviewMissingBasisSet
}

func (pendingReview) dryRunResultVariant()  {}
func (pendingReview) pendingReviewVariant() {}

func (result pendingReview) Applicability() profileadmissionsqlite.SoftwareSystemSpecMigrationRequired {
	return result.applicability
}

func (result pendingReview) MissingBasis() ReviewMissingBasisSet {
	return ReviewMissingBasisSet{values: result.missing.Values()}
}

// Applicable is a write-free dry-run result. It proves the exact profile,
// structure, and semantic-review conjunction observed by DryRun, but it is not
// an ApplyRequest, WorkCommission, or durable effect receipt.
type Applicable interface {
	DryRunResult
	Analysis() StructuralAnalysis
	Applicability() profileadmissionsqlite.SoftwareSystemSpecMigrationRequired
	Review() AdmittedMigrationReview
	applicableVariant()
}

type applicable struct {
	analysis      structuralAnalysis
	applicability profileadmissionsqlite.SoftwareSystemSpecMigrationRequired
	review        admittedMigrationReview
}

func (applicable) dryRunResultVariant() {}
func (applicable) applicableVariant()   {}

func (result applicable) Analysis() StructuralAnalysis {
	return result.analysis
}

func (result applicable) Applicability() profileadmissionsqlite.SoftwareSystemSpecMigrationRequired {
	return result.applicability
}

func (result applicable) Review() AdmittedMigrationReview {
	return result.review
}

type Invalid interface {
	DryRunResult
	Diagnostics() DiagnosticSet
	invalidVariant()
}

type invalid struct {
	diagnostics DiagnosticSet
}

func (invalid) dryRunResultVariant() {}
func (invalid) invalidVariant()      {}

func (result invalid) Diagnostics() DiagnosticSet {
	return result.diagnostics
}

func cloneSourceSnapshot(snapshot SourceSnapshot) SourceSnapshot {
	return SourceSnapshot{carrier: snapshot.carrier, bytes: append([]byte{}, snapshot.bytes...)}
}

func cloneTargetSnapshot(snapshot TargetSnapshot) TargetSnapshot {
	return TargetSnapshot{carrier: snapshot.carrier, bytes: append([]byte{}, snapshot.bytes...)}
}

func cloneTargetClaimCatalog(catalog TargetClaimCatalog) TargetClaimCatalog {
	return TargetClaimCatalog{
		carrier: catalog.carrier,
		digest:  catalog.digest,
		claims:  append([]TargetAtomicClaimID{}, catalog.claims...),
	}
}

func cloneOutsideCarrierSnapshots(snapshots OutsideCarrierSnapshots) OutsideCarrierSnapshots {
	values := make([]OutsideCarrierSnapshot, 0, len(snapshots.values))
	for _, snapshot := range snapshots.values {
		values = append(values, OutsideCarrierSnapshot{
			id:      snapshot.id,
			carrier: snapshot.carrier,
			bytes:   append([]byte{}, snapshot.bytes...),
		})
	}
	return OutsideCarrierSnapshots{values: values}
}
