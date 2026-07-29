package specmigrationv2

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"
)

func TestAuditPacketCandidateVerifiesExactEightSectionPartition(t *testing.T) {
	fixture := newPacketPartitionAuditFixture(t, TargetDigest{})

	audit, err := AuditPacketCandidate(fixture.candidate, fixture.request)
	if err != nil {
		t.Fatalf("AuditPacketCandidate: %v", err)
	}
	if audit.Status() != PacketPartitionAuditVerified {
		t.Fatalf("audit status = %q, want %q", audit.Status(), PacketPartitionAuditVerified)
	}
	if len(audit.Diagnostics()) != 0 {
		t.Fatalf("audit diagnostics = %v, want none", audit.Diagnostics())
	}
	counts := audit.Counts()
	assertPacketPartitionAuditCount(t, "source sections", counts.SourceSections(), 8)
	assertPacketPartitionAuditCount(t, "top-level dispositions", counts.TopLevelDispositions(), 8)
	assertPacketPartitionAuditCount(t, "split sections", counts.SplitSections(), 7)
	assertPacketPartitionAuditCount(t, "split leaves", counts.SplitLeaves(), 69)
	assertPacketPartitionAuditCount(t, "whole-section outcomes", counts.WholeSectionOutcomes(), 1)
	assertPacketPartitionAuditCount(t, "lineage entries", counts.LineageEntries(), 70)
	if !audit.PacketDigest().Equal(fixture.candidate.PacketDigest()) {
		t.Fatal("audit does not bind the candidate packet digest")
	}
	if !audit.PacketCarrierDigest().Equal(fixture.candidate.CarrierDigest()) {
		t.Fatal("audit does not bind the candidate carrier digest")
	}
	if audit.Digest().String() != DigestBytes(audit.CanonicalBytes()).String() {
		t.Fatal("audit digest does not cover the exact canonical bytes")
	}
	assertPacketPartitionAuditProjection(t, audit.CanonicalBytes(), fixture)
}

func TestAuditPacketCandidateIsDeterministic(t *testing.T) {
	fixture := newPacketPartitionAuditFixture(t, TargetDigest{})
	first, err := AuditPacketCandidate(fixture.candidate, fixture.request)
	if err != nil {
		t.Fatalf("first AuditPacketCandidate: %v", err)
	}
	for attempt := 0; attempt < 20; attempt++ {
		observed, observedErr := AuditPacketCandidate(fixture.candidate, fixture.request)
		if observedErr != nil {
			t.Fatalf("AuditPacketCandidate attempt %d: %v", attempt, observedErr)
		}
		if !bytes.Equal(observed.CanonicalBytes(), first.CanonicalBytes()) {
			t.Fatalf("canonical audit bytes changed on attempt %d", attempt)
		}
		if !observed.Digest().Equal(first.Digest()) {
			t.Fatalf("audit digest changed on attempt %d", attempt)
		}
	}
}

func TestAuditPacketCandidateRejectsObservedTargetMaterialDrift(t *testing.T) {
	fixture := newPacketPartitionAuditFixture(t, TargetDigest{})
	driftedBytes := append([]byte{}, fixture.targetBytes...)
	driftedBytes = append(driftedBytes, []byte("\n<!-- drift -->\n")...)
	driftedTarget, err := NewTargetSnapshot(TargetSnapshotInput{
		Carrier: fixture.packet.Target().Carrier(),
		Bytes:   driftedBytes,
	})
	if err != nil {
		t.Fatalf("NewTargetSnapshot: %v", err)
	}
	request, err := NewStructuralRequest(StructuralRequestInput{
		Packet:           fixture.packet,
		ProjectRoot:      fixture.projectRoot,
		Source:           fixture.request.source,
		Target:           driftedTarget,
		TargetClaims:     fixture.request.targetClaims,
		OutsideSnapshots: fixture.request.outsideSnapshots,
	})
	if err != nil {
		t.Fatalf("NewStructuralRequest: %v", err)
	}

	audit, err := AuditPacketCandidate(fixture.candidate, request)
	if err != nil {
		t.Fatalf("AuditPacketCandidate: %v", err)
	}
	if audit.Status() != PacketPartitionAuditRejected {
		t.Fatalf("audit status = %q, want %q", audit.Status(), PacketPartitionAuditRejected)
	}
	assertPacketPartitionAuditDiagnostic(t, audit.Diagnostics(), DiagnosticReviewTargetDigestMismatch)
	assertPacketPartitionAuditDiagnostic(t, audit.Diagnostics(), DiagnosticTargetDigestMismatch)
	assertPacketPartitionAuditDiagnostic(t, audit.Diagnostics(), DiagnosticTargetLengthMismatch)
	assertPacketPartitionAuditDiagnosticOrder(t, audit.Diagnostics())
}

func TestAuditPacketCandidateRejectsReviewBasisTargetDigestMismatch(t *testing.T) {
	wrongDigest := TargetDigestOf([]byte("different reviewed SoftwareSystemSpec"))
	fixture := newPacketPartitionAuditFixture(t, wrongDigest)

	audit, err := AuditPacketCandidate(fixture.candidate, fixture.request)
	if err != nil {
		t.Fatalf("AuditPacketCandidate: %v", err)
	}
	if audit.Status() != PacketPartitionAuditRejected {
		t.Fatalf("audit status = %q, want %q", audit.Status(), PacketPartitionAuditRejected)
	}
	diagnostics := audit.Diagnostics()
	if len(diagnostics) != 1 {
		t.Fatalf("diagnostic count = %d, want 1", len(diagnostics))
	}
	assertPacketPartitionAuditDiagnostic(t, diagnostics, DiagnosticReviewTargetDigestMismatch)
}

func TestAuditPacketCandidateRejectsLiveSourceAndOutsideSnapshotDrift(t *testing.T) {
	fixture := newPacketPartitionAuditFixture(t, TargetDigest{})
	driftedSourceBytes := append([]byte{}, fixture.sourceBytes...)
	driftedSourceBytes = append(driftedSourceBytes, []byte("<!-- source drift -->\n")...)
	driftedSource, err := NewSourceSnapshot(SourceSnapshotInput{
		Carrier: fixture.request.source.Carrier(),
		Bytes:   driftedSourceBytes,
	})
	if err != nil {
		t.Fatalf("NewSourceSnapshot: %v", err)
	}
	outsideValues := fixture.request.outsideSnapshots.Values()
	driftedOutsideBytes := append(outsideValues[0].Bytes(), []byte("# drift\n")...)
	driftedOutside, err := NewOutsideCarrierSnapshot(OutsideCarrierSnapshotInput{
		ID:      outsideValues[0].ID(),
		Carrier: outsideValues[0].Carrier(),
		Bytes:   driftedOutsideBytes,
	})
	if err != nil {
		t.Fatalf("NewOutsideCarrierSnapshot: %v", err)
	}
	outsideValues[0] = driftedOutside
	driftedOutsideSet, err := NewOutsideCarrierSnapshots(outsideValues)
	if err != nil {
		t.Fatalf("NewOutsideCarrierSnapshots: %v", err)
	}
	request, err := NewStructuralRequest(StructuralRequestInput{
		Packet:           fixture.packet,
		ProjectRoot:      fixture.projectRoot,
		Source:           driftedSource,
		Target:           fixture.request.target,
		TargetClaims:     fixture.request.targetClaims,
		OutsideSnapshots: driftedOutsideSet,
	})
	if err != nil {
		t.Fatalf("NewStructuralRequest: %v", err)
	}

	audit, err := AuditPacketCandidate(fixture.candidate, request)
	if err != nil {
		t.Fatalf("AuditPacketCandidate: %v", err)
	}
	if audit.Status() != PacketPartitionAuditRejected {
		t.Fatalf("audit status = %q, want %q", audit.Status(), PacketPartitionAuditRejected)
	}
	assertPacketPartitionAuditDiagnostic(t, audit.Diagnostics(), DiagnosticSourceDigestMismatch)
	assertPacketPartitionAuditDiagnostic(t, audit.Diagnostics(), DiagnosticOutsideCarrierDigestMismatch)
	assertPacketPartitionAuditDiagnosticOrder(t, audit.Diagnostics())
}

func TestAuditPacketCandidateRejectsDifferentStructuralPacket(t *testing.T) {
	fixture := newPacketPartitionAuditFixture(t, TargetDigest{})
	differentID, err := NewMigrationPacketID("different-packet")
	if err != nil {
		t.Fatalf("NewMigrationPacketID: %v", err)
	}
	differentPacket, err := NewPacket(PacketInput{
		ID:                 differentID,
		SchemaVersion:      fixture.packet.SchemaVersion(),
		Source:             fixture.packet.Source(),
		Target:             fixture.packet.Target(),
		OutsideRegistry:    fixture.packet.OutsideRegistry(),
		SourceDispositions: fixture.packet.SourceDispositions(),
	})
	if err != nil {
		t.Fatalf("NewPacket: %v", err)
	}
	request, err := NewStructuralRequest(StructuralRequestInput{
		Packet:           differentPacket,
		ProjectRoot:      fixture.projectRoot,
		Source:           fixture.request.source,
		Target:           fixture.request.target,
		TargetClaims:     fixture.request.targetClaims,
		OutsideSnapshots: fixture.request.outsideSnapshots,
	})
	if err != nil {
		t.Fatalf("NewStructuralRequest: %v", err)
	}

	_, err = AuditPacketCandidate(fixture.candidate, request)
	if err == nil {
		t.Fatal("AuditPacketCandidate accepted a structural request for another packet")
	}
}

type packetPartitionAuditFixture struct {
	packet        Packet
	candidate     FinalCandidatePacketCarrier
	request       StructuralRequest
	projectRoot   ProjectRootRef
	sourceBytes   []byte
	targetBytes   []byte
	reviewCarrier TargetCarrierID
}

func newPacketPartitionAuditFixture(
	t *testing.T,
	softwareReviewDigest TargetDigest,
) packetPartitionAuditFixture {
	t.Helper()
	sourceBytes := packetPartitionAuditSourceBytes()
	sections, err := deriveSourceSections(sourceBytes)
	if err != nil {
		t.Fatalf("deriveSourceSections: %v", err)
	}
	targetBytes := []byte("# Audit target\n\n## SS.audit.001 Audit\n\n```yaml spec-section\nid: SS.audit.001\nspec: software-system\nkind: software.functional_behavior\nstatus: draft\nclaims:\n  - id: SS.audit.001.L1\n    class: L\n    statement: The exact audit fixture behavior is preserved.\n    scope:\n      - audit-fixture\n```\n")
	projectRoot := auditProjectRoot(t)
	sourceCarrier := auditSourceCarrier(t, ".haft/specs/enabling-system.md")
	targetCarrier := auditTargetCarrier(t, ".haft/specs/software-system.md")
	archiveCarrier := auditArchiveCarrier(t, ".haft/migration-archive/enabling-system.md")
	source := auditSourceManifest(
		t,
		projectRoot,
		sourceCarrier,
		archiveCarrier,
		sourceBytes,
		sections,
	)
	target := auditTargetManifest(t, targetCarrier, targetBytes)
	claim := auditTargetClaim(t, "SS.audit.001.L1")
	mapping := auditMapOne(t, claim)
	outsideDisposition, registry, outsideSnapshots := auditOutsideMaterials(t)
	retirement, err := NewRetireHistory("placeholder history remains explicit")
	if err != nil {
		t.Fatalf("NewRetireHistory: %v", err)
	}
	dispositions := make([]SourceDisposition, 0, len(sections))
	direct, err := NewSourceDisposition(sections[0].ID(), retirement)
	if err != nil {
		t.Fatalf("NewSourceDisposition direct: %v", err)
	}
	dispositions = append(dispositions, direct)
	leafCounts := []int{10, 10, 10, 10, 10, 10, 9}
	for index, section := range sections[1:] {
		branches := auditSplitBranches(t, sourceBytes, section.Span(), leafCounts[index], mapping)
		if index == 0 {
			outsideBranch, outsideErr := NewSplitBranch(branches[0].Fragment(), outsideDisposition)
			if outsideErr != nil {
				t.Fatalf("NewSplitBranch outside: %v", outsideErr)
			}
			branches[0] = outsideBranch
		}
		split, splitErr := NewSplitOneToMany(branches)
		if splitErr != nil {
			t.Fatalf("NewSplitOneToMany section %d: %v", index, splitErr)
		}
		disposition, dispositionErr := NewSourceDisposition(section.ID(), split)
		if dispositionErr != nil {
			t.Fatalf("NewSourceDisposition split %d: %v", index, dispositionErr)
		}
		dispositions = append(dispositions, disposition)
	}
	packetID, err := NewMigrationPacketID("packet-partition-audit-fixture")
	if err != nil {
		t.Fatalf("NewMigrationPacketID: %v", err)
	}
	packet, err := NewPacket(PacketInput{
		ID:                 packetID,
		SchemaVersion:      SchemaVersionV2,
		Source:             source,
		Target:             target,
		OutsideRegistry:    registry,
		SourceDispositions: dispositions,
	})
	if err != nil {
		t.Fatalf("NewPacket: %v", err)
	}
	if !softwareReviewDigest.valid() {
		softwareReviewDigest = TargetDigestOf(targetBytes)
	}
	reviewCarrier := auditTargetCarrier(t, ".context/review/software-system.md")
	basis := auditReviewBasis(t, reviewCarrier, softwareReviewDigest)
	candidate, err := FinalizePacketCarrier(packet, basis)
	if err != nil {
		t.Fatalf("FinalizePacketCarrier: %v", err)
	}
	sourceSnapshot, err := NewSourceSnapshot(SourceSnapshotInput{
		Carrier: sourceCarrier,
		Bytes:   sourceBytes,
	})
	if err != nil {
		t.Fatalf("NewSourceSnapshot: %v", err)
	}
	targetSnapshot, err := NewTargetSnapshot(TargetSnapshotInput{
		Carrier: targetCarrier,
		Bytes:   targetBytes,
	})
	if err != nil {
		t.Fatalf("NewTargetSnapshot: %v", err)
	}
	targetCatalog, err := NewTargetClaimCatalog(TargetClaimCatalogInput{
		Carrier: targetCarrier,
		Bytes:   targetBytes,
	})
	if err != nil {
		t.Fatalf("NewTargetClaimCatalog: %v", err)
	}
	request, err := NewStructuralRequest(StructuralRequestInput{
		Packet:           packet,
		ProjectRoot:      projectRoot,
		Source:           sourceSnapshot,
		Target:           targetSnapshot,
		TargetClaims:     targetCatalog,
		OutsideSnapshots: outsideSnapshots,
	})
	if err != nil {
		t.Fatalf("NewStructuralRequest: %v", err)
	}
	return packetPartitionAuditFixture{
		packet:        packet,
		candidate:     candidate,
		request:       request,
		projectRoot:   projectRoot,
		sourceBytes:   sourceBytes,
		targetBytes:   targetBytes,
		reviewCarrier: reviewCarrier,
	}
}

func auditOutsideMaterials(
	t *testing.T,
) (OutsidePSS, OutsideCarrierRegistry, OutsideCarrierSnapshots) {
	t.Helper()
	type material struct {
		id      string
		carrier string
		bytes   []byte
	}
	materials := []material{
		{id: "workflow_policy", carrier: ".github/workflows/ci.yml", bytes: []byte("name: audit fixture\n")},
		{id: "agent_discipline", carrier: "AGENTS.md", bytes: []byte("# Audit fixture discipline\n")},
	}
	ids := make([]OutsideCarrierID, 0, len(materials))
	registrations := make([]OutsideCarrierRegistration, 0, len(materials))
	snapshots := make([]OutsideCarrierSnapshot, 0, len(materials))
	for _, candidate := range materials {
		id, err := NewOutsideCarrierID(candidate.id)
		if err != nil {
			t.Fatalf("NewOutsideCarrierID: %v", err)
		}
		carrier := auditSourceCarrier(t, candidate.carrier)
		registration, err := NewOutsideCarrierRegistration(OutsideCarrierRegistrationInput{
			ID:      id,
			Carrier: carrier,
			Digest:  OutsideCarrierDigestOf(candidate.bytes),
		})
		if err != nil {
			t.Fatalf("NewOutsideCarrierRegistration: %v", err)
		}
		snapshot, err := NewOutsideCarrierSnapshot(OutsideCarrierSnapshotInput{
			ID:      id,
			Carrier: carrier,
			Bytes:   candidate.bytes,
		})
		if err != nil {
			t.Fatalf("NewOutsideCarrierSnapshot: %v", err)
		}
		ids = append(ids, id)
		registrations = append(registrations, registration)
		snapshots = append(snapshots, snapshot)
	}
	set, err := NewOutsideCarrierSet(ids)
	if err != nil {
		t.Fatalf("NewOutsideCarrierSet: %v", err)
	}
	disposition, err := NewOutsidePSS("agent and workflow policy remains outside the PSS", set)
	if err != nil {
		t.Fatalf("NewOutsidePSS: %v", err)
	}
	registry, err := NewOutsideCarrierRegistry(registrations)
	if err != nil {
		t.Fatalf("NewOutsideCarrierRegistry: %v", err)
	}
	observed, err := NewOutsideCarrierSnapshots(snapshots)
	if err != nil {
		t.Fatalf("NewOutsideCarrierSnapshots: %v", err)
	}
	return disposition, registry, observed
}

func packetPartitionAuditSourceBytes() []byte {
	result := []byte("# Audit source\n\n")
	for index := 0; index < 8; index++ {
		id := fmt.Sprintf("ES.audit%d.001", index)
		section := fmt.Sprintf(
			"## %s Audit %d\n\n```yaml spec-section\nid: %s\nspec: enabling-system\nkind: enabling.audit%d\nstatus: draft\n```\n\n",
			id,
			index,
			id,
			index,
		)
		result = append(result, []byte(section)...)
	}
	return result
}

func auditSplitBranches(
	t *testing.T,
	source []byte,
	section ExactByteSpan,
	count int,
	mapping MapOne,
) []SplitBranch {
	t.Helper()
	total := section.Length().Value()
	if total < uint64(count) {
		t.Fatalf("section length %d is smaller than leaf count %d", total, count)
	}
	base := total / uint64(count)
	remainder := total % uint64(count)
	start := section.Start()
	result := make([]SplitBranch, 0, count)
	for index := 0; index < count; index++ {
		lengthValue := base
		if uint64(index) < remainder {
			lengthValue++
		}
		length, err := NewByteLength(lengthValue)
		if err != nil {
			t.Fatalf("NewByteLength: %v", err)
		}
		end := start + lengthValue
		span, err := NewExactByteSpan(start, length, FragmentDigestOf(source[start:end]))
		if err != nil {
			t.Fatalf("NewExactByteSpan: %v", err)
		}
		branch, err := NewSplitBranch(span, mapping)
		if err != nil {
			t.Fatalf("NewSplitBranch: %v", err)
		}
		result = append(result, branch)
		start = end
	}
	return result
}

func auditProjectRoot(t *testing.T) ProjectRootRef {
	t.Helper()
	value, err := NewProjectRootRef("/tmp/haft-packet-partition-audit")
	if err != nil {
		t.Fatalf("NewProjectRootRef: %v", err)
	}
	return value
}

func auditSourceCarrier(t *testing.T, raw string) SourceCarrierID {
	t.Helper()
	value, err := NewSourceCarrierID(raw)
	if err != nil {
		t.Fatalf("NewSourceCarrierID: %v", err)
	}
	return value
}

func auditTargetCarrier(t *testing.T, raw string) TargetCarrierID {
	t.Helper()
	value, err := NewTargetCarrierID(raw)
	if err != nil {
		t.Fatalf("NewTargetCarrierID: %v", err)
	}
	return value
}

func auditArchiveCarrier(t *testing.T, raw string) ArchiveCarrierID {
	t.Helper()
	value, err := NewArchiveCarrierID(raw)
	if err != nil {
		t.Fatalf("NewArchiveCarrierID: %v", err)
	}
	return value
}

func auditSourceManifest(
	t *testing.T,
	root ProjectRootRef,
	carrier SourceCarrierID,
	archiveCarrier ArchiveCarrierID,
	bytes []byte,
	sections []SourceSection,
) SourceManifest {
	t.Helper()
	commit, err := NewGitCommitOID("sha1:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if err != nil {
		t.Fatalf("NewGitCommitOID: %v", err)
	}
	edition, err := NewRepositoryEdition(root, commit, carrier, SourceDigestOf(bytes))
	if err != nil {
		t.Fatalf("NewRepositoryEdition: %v", err)
	}
	recordRef, err := NewProvenanceRecordRef(".context/source-designation.md")
	if err != nil {
		t.Fatalf("NewProvenanceRecordRef: %v", err)
	}
	record, err := NewProvenanceRecordBinding(
		recordRef,
		ProvenanceRecordDigestOf([]byte("audit source designation")),
	)
	if err != nil {
		t.Fatalf("NewProvenanceRecordBinding: %v", err)
	}
	provenance, err := NewDesignatedSourceProvenance(edition, record)
	if err != nil {
		t.Fatalf("NewDesignatedSourceProvenance: %v", err)
	}
	archive, err := NewArchiveManifest(archiveCarrier, SourceDigestOf(bytes))
	if err != nil {
		t.Fatalf("NewArchiveManifest: %v", err)
	}
	length, err := NewByteLength(uint64(len(bytes)))
	if err != nil {
		t.Fatalf("NewByteLength: %v", err)
	}
	manifest, err := NewSourceManifest(SourceManifestInput{
		Carrier:    carrier,
		Digest:     SourceDigestOf(bytes),
		ByteLength: length,
		Archive:    archive,
		Provenance: provenance,
		Sections:   sections,
	})
	if err != nil {
		t.Fatalf("NewSourceManifest: %v", err)
	}
	return manifest
}

func auditTargetManifest(t *testing.T, carrier TargetCarrierID, bytes []byte) TargetManifest {
	t.Helper()
	length, err := NewByteLength(uint64(len(bytes)))
	if err != nil {
		t.Fatalf("NewByteLength: %v", err)
	}
	manifest, err := NewTargetManifest(TargetManifestInput{
		Carrier:    carrier,
		Digest:     TargetDigestOf(bytes),
		ByteLength: length,
	})
	if err != nil {
		t.Fatalf("NewTargetManifest: %v", err)
	}
	return manifest
}

func auditTargetClaim(t *testing.T, raw string) TargetAtomicClaimID {
	t.Helper()
	value, err := NewTargetAtomicClaimID(raw)
	if err != nil {
		t.Fatalf("NewTargetAtomicClaimID: %v", err)
	}
	return value
}

func auditMapOne(t *testing.T, claim TargetAtomicClaimID) MapOne {
	t.Helper()
	set, err := NewTargetClaimSet([]TargetAtomicClaimID{claim})
	if err != nil {
		t.Fatalf("NewTargetClaimSet: %v", err)
	}
	value, err := NewMapOne(set)
	if err != nil {
		t.Fatalf("NewMapOne: %v", err)
	}
	return value
}

func auditReviewBasis(
	t *testing.T,
	softwareCarrier TargetCarrierID,
	softwareDigest TargetDigest,
) FinalCandidateReviewBasis {
	t.Helper()
	targetSystemCarrier := auditTargetCarrier(t, ".context/review/target-system.md")
	termMapCarrier := auditTargetCarrier(t, ".context/review/term-map.md")
	zeroPassCarrier := auditTargetCarrier(t, ".context/review/semantic-zero-pass.md")
	basis, err := NewFinalCandidateReviewBasis(FinalCandidateReviewBasisInput{
		CarrierDigests: []ReviewCarrierDigestInput{
			{
				Role:    ReviewTargetSystemCarrier,
				Carrier: targetSystemCarrier,
				Digest:  DigestBytes([]byte("target-system review bytes")),
			},
			{
				Role:    ReviewSoftwareSystemCarrier,
				Carrier: softwareCarrier,
				Digest:  softwareDigest.value,
			},
			{
				Role:    ReviewTermMapCarrier,
				Carrier: termMapCarrier,
				Digest:  DigestBytes([]byte("term-map review bytes")),
			},
		},
		FPFRevision: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		SemanticZeroPass: SemanticZeroPassInput{
			Carrier: zeroPassCarrier,
			Digest:  DigestBytes([]byte("semantic zero-pass bytes")),
		},
		LifecycleIntent: []LifecycleIntentInput{
			{SectionRef: "SS.audit.001", Operation: LifecycleActivate},
		},
	})
	if err != nil {
		t.Fatalf("NewFinalCandidateReviewBasis: %v", err)
	}
	return basis
}

func assertPacketPartitionAuditCount(t *testing.T, name string, observed, expected int) {
	t.Helper()
	if observed != expected {
		t.Fatalf("%s = %d, want %d", name, observed, expected)
	}
}

func assertPacketPartitionAuditDiagnostic(
	t *testing.T,
	diagnostics []PacketPartitionAuditDiagnostic,
	want DiagnosticCode,
) {
	t.Helper()
	for _, diagnostic := range diagnostics {
		if diagnostic.Code() == want {
			return
		}
	}
	t.Fatalf("diagnostics do not contain %q: %#v", want, diagnostics)
}

func assertPacketPartitionAuditDiagnosticOrder(
	t *testing.T,
	diagnostics []PacketPartitionAuditDiagnostic,
) {
	t.Helper()
	for index := 1; index < len(diagnostics); index++ {
		previous := string(diagnostics[index-1].Code()) + "\x00" + diagnostics[index-1].Subject() + "\x00" + diagnostics[index-1].Detail()
		current := string(diagnostics[index].Code()) + "\x00" + diagnostics[index].Subject() + "\x00" + diagnostics[index].Detail()
		if previous > current {
			t.Fatalf("diagnostics are not sorted at %d: %q > %q", index, previous, current)
		}
	}
}

func assertPacketPartitionAuditProjection(
	t *testing.T,
	canonical []byte,
	fixture packetPartitionAuditFixture,
) {
	t.Helper()
	var projection packetPartitionAuditJSON
	if err := json.Unmarshal(canonical, &projection); err != nil {
		t.Fatalf("decode canonical audit: %v", err)
	}
	if projection.Kind != packetPartitionAuditKind {
		t.Fatalf("audit kind = %q", projection.Kind)
	}
	if projection.BindingPosture.Status != packetPartitionAuditPosture {
		t.Fatalf("binding posture = %q", projection.BindingPosture.Status)
	}
	if projection.ObservedInputs.TargetMaterial.ReviewCarrier != fixture.reviewCarrier.String() {
		t.Fatalf("review target carrier = %q", projection.ObservedInputs.TargetMaterial.ReviewCarrier)
	}
	if projection.ObservedInputs.TargetMaterial.ReviewBasisSHA256 != TargetDigestOf(fixture.targetBytes).String() {
		t.Fatalf("review-basis target digest = %q", projection.ObservedInputs.TargetMaterial.ReviewBasisSHA256)
	}
	if projection.ObservedInputs.TargetMaterial.LogicalTargetCarrier != fixture.packet.Target().Carrier().String() {
		t.Fatalf("logical target carrier = %q", projection.ObservedInputs.TargetMaterial.LogicalTargetCarrier)
	}
	if projection.ObservedInputs.TargetMaterial.SHA256 != TargetDigestOf(fixture.targetBytes).String() {
		t.Fatalf("observed target digest = %q", projection.ObservedInputs.TargetMaterial.SHA256)
	}
	if projection.ObservedInputs.Source.SHA256 != SourceDigestOf(fixture.sourceBytes).String() {
		t.Fatalf("observed source digest = %q", projection.ObservedInputs.Source.SHA256)
	}
	if len(projection.ObservedInputs.Outside) != 2 {
		t.Fatalf("outside observations = %#v, want two exact snapshots", projection.ObservedInputs.Outside)
	}
	if projection.ObservedInputs.Outside[0].ID != "agent_discipline" ||
		projection.ObservedInputs.Outside[1].ID != "workflow_policy" {
		t.Fatalf("outside observations are not in canonical ID order: %#v", projection.ObservedInputs.Outside)
	}
	if projection.Diagnostics == nil || len(projection.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v, want canonical empty array", projection.Diagnostics)
	}
}
