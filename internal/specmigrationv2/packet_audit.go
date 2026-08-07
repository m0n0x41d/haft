package specmigrationv2

import (
	"encoding/json"
	"fmt"
	"sort"
)

const PacketPartitionAuditSchemaVersionV1 = "haft.spec-migration-v2.packet-partition-audit/v1"

const packetPartitionAuditKind = "NonBindingReadOnlyDerivedEvidence"

const packetPartitionAuditPosture = "non_binding_read_only_derived_evidence"

type PacketPartitionAuditStatus string

const (
	PacketPartitionAuditVerified PacketPartitionAuditStatus = "verified"
	PacketPartitionAuditRejected PacketPartitionAuditStatus = "rejected"
)

type PacketPartitionAuditDigest struct {
	value SHA256
}

func (digest PacketPartitionAuditDigest) String() string {
	return digest.value.String()
}

func (digest PacketPartitionAuditDigest) Equal(other PacketPartitionAuditDigest) bool {
	return digest.value.Equal(other.value)
}

// PacketPartitionAuditBinding is the exact non-authority audit evidence shown
// at the human review boundary. Binding it into the SpeechAct and admission
// record prevents a verified audit displayed for one observation from being
// silently replaced by another observation of the same packet carrier.
type PacketPartitionAuditBinding struct {
	schema string
	status PacketPartitionAuditStatus
	digest PacketPartitionAuditDigest
}

func (binding PacketPartitionAuditBinding) Schema() string {
	return binding.schema
}

func (binding PacketPartitionAuditBinding) Status() PacketPartitionAuditStatus {
	return binding.status
}

func (binding PacketPartitionAuditBinding) Digest() PacketPartitionAuditDigest {
	return binding.digest
}

func (binding PacketPartitionAuditBinding) valid() bool {
	return binding.schema == PacketPartitionAuditSchemaVersionV1 &&
		binding.status == PacketPartitionAuditVerified &&
		binding.digest.value.valid()
}

type PacketPartitionAuditCounts struct {
	sourceSections       int
	topLevelDispositions int
	splitSections        int
	splitLeaves          int
	wholeSectionOutcomes int
	lineageEntries       int
}

func (counts PacketPartitionAuditCounts) SourceSections() int {
	return counts.sourceSections
}

func (counts PacketPartitionAuditCounts) TopLevelDispositions() int {
	return counts.topLevelDispositions
}

func (counts PacketPartitionAuditCounts) SplitSections() int {
	return counts.splitSections
}

func (counts PacketPartitionAuditCounts) SplitLeaves() int {
	return counts.splitLeaves
}

func (counts PacketPartitionAuditCounts) WholeSectionOutcomes() int {
	return counts.wholeSectionOutcomes
}

func (counts PacketPartitionAuditCounts) LineageEntries() int {
	return counts.lineageEntries
}

type PacketPartitionAuditDiagnostic struct {
	code    DiagnosticCode
	subject string
	detail  string
}

func (diagnostic PacketPartitionAuditDiagnostic) Code() DiagnosticCode {
	return diagnostic.code
}

func (diagnostic PacketPartitionAuditDiagnostic) Subject() string {
	return diagnostic.subject
}

func (diagnostic PacketPartitionAuditDiagnostic) Detail() string {
	return diagnostic.detail
}

type packetPartitionAuditSnapshot struct {
	id         string
	carrier    string
	digest     string
	byteLength uint64
}

type PacketPartitionAudit struct {
	status              PacketPartitionAuditStatus
	packetID            MigrationPacketID
	packetDigest        PacketDigest
	packetCarrierDigest PacketCarrierDigest
	source              packetPartitionAuditSnapshot
	targetMaterial      packetPartitionAuditSnapshot
	targetReviewDigest  SHA256
	logicalTarget       TargetCarrierID
	outside             []packetPartitionAuditSnapshot
	counts              PacketPartitionAuditCounts
	diagnostics         []PacketPartitionAuditDiagnostic
	canonicalBytes      []byte
}

func (audit PacketPartitionAudit) Status() PacketPartitionAuditStatus {
	return audit.status
}

func (audit PacketPartitionAudit) PacketID() MigrationPacketID {
	return audit.packetID
}

func (audit PacketPartitionAudit) PacketDigest() PacketDigest {
	return audit.packetDigest
}

func (audit PacketPartitionAudit) PacketCarrierDigest() PacketCarrierDigest {
	return audit.packetCarrierDigest
}

func (audit PacketPartitionAudit) Counts() PacketPartitionAuditCounts {
	return audit.counts
}

func (audit PacketPartitionAudit) Diagnostics() []PacketPartitionAuditDiagnostic {
	return append([]PacketPartitionAuditDiagnostic{}, audit.diagnostics...)
}

func (audit PacketPartitionAudit) CanonicalBytes() []byte {
	return append([]byte{}, audit.canonicalBytes...)
}

func (audit PacketPartitionAudit) Digest() PacketPartitionAuditDigest {
	return PacketPartitionAuditDigest{value: DigestBytes(audit.canonicalBytes)}
}

func (audit PacketPartitionAudit) Binding() PacketPartitionAuditBinding {
	return PacketPartitionAuditBinding{
		schema: PacketPartitionAuditSchemaVersionV1,
		status: audit.status,
		digest: audit.Digest(),
	}
}

// AuditPacketCandidate produces one-way read-only evidence over an exact
// canonical packet carrier and exact observed materials. The result is not a
// packet input, semantic-review admission, authority receipt, or apply token.
func AuditPacketCandidate(
	candidate FinalCandidatePacketCarrier,
	request StructuralRequest,
) (PacketPartitionAudit, error) {
	packet := candidate.Packet()
	candidateDigest, err := PacketDigestOf(packet)
	if err != nil {
		return PacketPartitionAudit{}, fmt.Errorf("audit packet candidate: %w", err)
	}
	requestDigest, err := PacketDigestOf(request.packet)
	if err != nil {
		return PacketPartitionAudit{}, fmt.Errorf("audit structural request packet: %w", err)
	}
	if !candidateDigest.Equal(candidate.PacketDigest()) {
		return PacketPartitionAudit{}, fmt.Errorf("audit packet candidate digest does not match its packet")
	}
	if !candidateDigest.Equal(requestDigest) {
		return PacketPartitionAudit{}, fmt.Errorf("audit structural request packet does not match the exact candidate")
	}
	carrierDigest := PacketCarrierDigest{value: DigestBytes(candidate.CanonicalBytes())}
	if !carrierDigest.Equal(candidate.CarrierDigest()) {
		return PacketPartitionAudit{}, fmt.Errorf("audit packet candidate carrier digest does not match its canonical bytes")
	}
	targetMaterialBinding, err := softwareReviewBindingFromBasis(candidate.ReviewBasis())
	if err != nil {
		return PacketPartitionAudit{}, err
	}
	audit := PacketPartitionAudit{
		packetID:            packet.ID(),
		packetDigest:        candidateDigest,
		packetCarrierDigest: candidate.CarrierDigest(),
		source:              sourceAuditSnapshot(request.source),
		targetMaterial:      targetAuditSnapshot(targetMaterialBinding.Carrier(), request.target),
		targetReviewDigest:  targetMaterialBinding.Digest(),
		logicalTarget:       packet.Target().Carrier(),
		outside:             outsideAuditSnapshots(request.outsideSnapshots),
		counts:              packetPartitionCounts(packet),
	}
	audit.diagnostics = targetMaterialDiagnostics(targetMaterialBinding, request.target)
	result := AnalyzeStructure(request)
	switch value := result.(type) {
	case ValidAnalysis:
		if len(audit.diagnostics) == 0 {
			audit.status = PacketPartitionAuditVerified
			break
		}
		audit.status = PacketPartitionAuditRejected
	case InvalidDiagnostics:
		audit.status = PacketPartitionAuditRejected
		audit.diagnostics = append(audit.diagnostics, auditDiagnostics(value.Diagnostics())...)
	default:
		return PacketPartitionAudit{}, fmt.Errorf("structural analysis returned an unknown result variant")
	}
	sortAuditDiagnostics(audit.diagnostics)
	canonical, err := encodePacketPartitionAudit(audit)
	if err != nil {
		return PacketPartitionAudit{}, err
	}
	audit.canonicalBytes = canonical
	return audit, nil
}

func softwareReviewBindingFromBasis(
	basis FinalCandidateReviewBasis,
) (ReviewCarrierDigest, error) {
	for _, binding := range basis.CarrierDigests().Values() {
		if binding.Role() == ReviewSoftwareSystemCarrier {
			return binding, nil
		}
	}
	return ReviewCarrierDigest{}, fmt.Errorf("final packet candidate has no SoftwareSystemSpec review carrier")
}

func targetMaterialDiagnostics(
	binding ReviewCarrierDigest,
	snapshot TargetSnapshot,
) []PacketPartitionAuditDiagnostic {
	observed := DigestBytes(snapshot.Bytes())
	if binding.Digest().Equal(observed) {
		return []PacketPartitionAuditDiagnostic{}
	}
	return []PacketPartitionAuditDiagnostic{{
		code:    DiagnosticReviewTargetDigestMismatch,
		subject: binding.Carrier().String(),
		detail:  "observed SoftwareSystemSpec review material does not match the exact final-candidate review basis",
	}}
}

func sourceAuditSnapshot(snapshot SourceSnapshot) packetPartitionAuditSnapshot {
	bytes := snapshot.Bytes()
	return packetPartitionAuditSnapshot{
		carrier:    snapshot.Carrier().String(),
		digest:     SourceDigestOf(bytes).String(),
		byteLength: uint64(len(bytes)),
	}
}

func targetAuditSnapshot(
	reviewCarrier TargetCarrierID,
	snapshot TargetSnapshot,
) packetPartitionAuditSnapshot {
	bytes := snapshot.Bytes()
	return packetPartitionAuditSnapshot{
		carrier:    reviewCarrier.String(),
		digest:     TargetDigestOf(bytes).String(),
		byteLength: uint64(len(bytes)),
	}
}

func outsideAuditSnapshots(snapshots OutsideCarrierSnapshots) []packetPartitionAuditSnapshot {
	result := make([]packetPartitionAuditSnapshot, 0, len(snapshots.values))
	for _, snapshot := range snapshots.Values() {
		bytes := snapshot.Bytes()
		result = append(result, packetPartitionAuditSnapshot{
			id:         snapshot.ID().String(),
			carrier:    snapshot.Carrier().String(),
			digest:     OutsideCarrierDigestOf(bytes).String(),
			byteLength: uint64(len(bytes)),
		})
	}
	sort.Slice(result, func(left, right int) bool {
		leftKey := result[left].id + "\x00" + result[left].carrier
		rightKey := result[right].id + "\x00" + result[right].carrier
		return leftKey < rightKey
	})
	return result
}

func packetPartitionCounts(packet Packet) PacketPartitionAuditCounts {
	dispositions := packet.SourceDispositions()
	splitSections := 0
	splitLeaves := 0
	wholeSectionOutcomes := 0
	for _, sourceDisposition := range dispositions {
		split, ok := sourceDisposition.Disposition().(SplitOneToMany)
		if ok {
			splitSections++
			splitLeaves += len(split.Branches())
			continue
		}
		wholeSectionOutcomes++
	}
	return PacketPartitionAuditCounts{
		sourceSections:       len(packet.Source().Sections()),
		topLevelDispositions: len(dispositions),
		splitSections:        splitSections,
		splitLeaves:          splitLeaves,
		wholeSectionOutcomes: wholeSectionOutcomes,
		lineageEntries:       len(packet.LineagePolicy().Entries()),
	}
}

func auditDiagnostics(set DiagnosticSet) []PacketPartitionAuditDiagnostic {
	values := set.Values()
	result := make([]PacketPartitionAuditDiagnostic, 0, len(values))
	for _, diagnostic := range values {
		result = append(result, PacketPartitionAuditDiagnostic{
			code:    diagnostic.Code(),
			subject: diagnostic.Subject(),
			detail:  diagnostic.Detail(),
		})
	}
	return result
}

func sortAuditDiagnostics(values []PacketPartitionAuditDiagnostic) {
	sort.Slice(values, func(left, right int) bool {
		leftKey := string(values[left].code) + "\x00" + values[left].subject + "\x00" + values[left].detail
		rightKey := string(values[right].code) + "\x00" + values[right].subject + "\x00" + values[right].detail
		return leftKey < rightKey
	})
}

type packetPartitionAuditJSON struct {
	Kind           string                               `json:"kind"`
	SchemaVersion  string                               `json:"schema_version"`
	BindingPosture packetPartitionAuditPostureJSON      `json:"binding_posture"`
	Status         PacketPartitionAuditStatus           `json:"status"`
	ObservedInputs packetPartitionAuditObservedJSON     `json:"observed_inputs"`
	Counts         packetPartitionAuditCountsJSON       `json:"counts"`
	Diagnostics    []packetPartitionAuditDiagnosticJSON `json:"diagnostics"`
}

type packetPartitionAuditPostureJSON struct {
	Status    string `json:"status"`
	Statement string `json:"statement"`
}

type packetPartitionAuditObservedJSON struct {
	Packet         packetPartitionAuditPacketJSON     `json:"packet"`
	Source         packetPartitionAuditSnapshotJSON   `json:"source"`
	TargetMaterial packetPartitionAuditTargetJSON     `json:"target_material"`
	Outside        []packetPartitionAuditSnapshotJSON `json:"outside"`
}

type packetPartitionAuditPacketJSON struct {
	ID            string `json:"id"`
	PacketDigest  string `json:"packet_digest"`
	CarrierDigest string `json:"carrier_digest"`
}

type packetPartitionAuditSnapshotJSON struct {
	ID         string `json:"id,omitempty"`
	Carrier    string `json:"carrier"`
	SHA256     string `json:"sha256"`
	ByteLength uint64 `json:"byte_length"`
}

type packetPartitionAuditTargetJSON struct {
	ReviewCarrier        string `json:"review_carrier"`
	ReviewBasisSHA256    string `json:"review_basis_sha256"`
	LogicalTargetCarrier string `json:"logical_target_carrier"`
	SHA256               string `json:"sha256"`
	ByteLength           uint64 `json:"byte_length"`
}

type packetPartitionAuditCountsJSON struct {
	SourceSections       int `json:"source_sections"`
	TopLevelDispositions int `json:"top_level_dispositions"`
	SplitSections        int `json:"split_sections"`
	SplitLeaves          int `json:"split_leaves"`
	WholeSectionOutcomes int `json:"whole_section_outcomes"`
	LineageEntries       int `json:"lineage_entries"`
}

type packetPartitionAuditDiagnosticJSON struct {
	Code    DiagnosticCode `json:"code"`
	Subject string         `json:"subject"`
	Detail  string         `json:"detail"`
}

func encodePacketPartitionAudit(audit PacketPartitionAudit) ([]byte, error) {
	diagnostics := make([]packetPartitionAuditDiagnosticJSON, 0, len(audit.diagnostics))
	for _, diagnostic := range audit.diagnostics {
		diagnostics = append(diagnostics, packetPartitionAuditDiagnosticJSON{
			Code:    diagnostic.code,
			Subject: diagnostic.subject,
			Detail:  diagnostic.detail,
		})
	}
	outside := make([]packetPartitionAuditSnapshotJSON, 0, len(audit.outside))
	for _, snapshot := range audit.outside {
		outside = append(outside, packetPartitionAuditSnapshotJSON{
			ID:         snapshot.id,
			Carrier:    snapshot.carrier,
			SHA256:     snapshot.digest,
			ByteLength: snapshot.byteLength,
		})
	}
	projection := packetPartitionAuditJSON{
		Kind:          packetPartitionAuditKind,
		SchemaVersion: PacketPartitionAuditSchemaVersionV1,
		BindingPosture: packetPartitionAuditPostureJSON{
			Status: packetPartitionAuditPosture,
			Statement: "This audit is one-way read-only derived evidence. It is not a migration packet, " +
				"semantic-review admission, authority receipt, apply authorization, or migration effect.",
		},
		Status: audit.status,
		ObservedInputs: packetPartitionAuditObservedJSON{
			Packet: packetPartitionAuditPacketJSON{
				ID:            audit.packetID.String(),
				PacketDigest:  audit.packetDigest.String(),
				CarrierDigest: audit.packetCarrierDigest.String(),
			},
			Source: packetPartitionAuditSnapshotJSON{
				Carrier:    audit.source.carrier,
				SHA256:     audit.source.digest,
				ByteLength: audit.source.byteLength,
			},
			TargetMaterial: packetPartitionAuditTargetJSON{
				ReviewCarrier:        audit.targetMaterial.carrier,
				ReviewBasisSHA256:    audit.targetReviewDigest.String(),
				LogicalTargetCarrier: audit.logicalTarget.String(),
				SHA256:               audit.targetMaterial.digest,
				ByteLength:           audit.targetMaterial.byteLength,
			},
			Outside: outside,
		},
		Counts: packetPartitionAuditCountsJSON{
			SourceSections:       audit.counts.sourceSections,
			TopLevelDispositions: audit.counts.topLevelDispositions,
			SplitSections:        audit.counts.splitSections,
			SplitLeaves:          audit.counts.splitLeaves,
			WholeSectionOutcomes: audit.counts.wholeSectionOutcomes,
			LineageEntries:       audit.counts.lineageEntries,
		},
		Diagnostics: diagnostics,
	}
	encoded, err := json.MarshalIndent(projection, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode packet partition audit: %w", err)
	}
	return append(encoded, '\n'), nil
}
