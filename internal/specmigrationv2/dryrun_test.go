package specmigrationv2_test

import (
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/projectprofile"
	"github.com/m0n0x41d/haft/internal/specmigrationv2"
)

func TestAnalyzeStructureZeroRequestReturnsInvalidDiagnostics(t *testing.T) {
	result := specmigrationv2.AnalyzeStructure(specmigrationv2.StructuralRequest{})
	invalid, ok := result.(specmigrationv2.InvalidDiagnostics)
	if !ok {
		t.Fatalf("AnalyzeStructure result = %T, want InvalidDiagnostics", result)
	}
	diagnostics := invalid.Diagnostics().Values()
	if len(diagnostics) != 1 {
		t.Fatalf("diagnostics = %d, want 1", len(diagnostics))
	}
	if diagnostics[0].Code() != specmigrationv2.DiagnosticInvalidCoreVariant {
		t.Fatalf("diagnostic code = %q, want %q", diagnostics[0].Code(), specmigrationv2.DiagnosticInvalidCoreVariant)
	}
}

func TestAnalyzeStructureReturnsValidForExactStructurallyValidPacket(t *testing.T) {
	fixture := newFixture(t)
	result := fixture.analyze(t, fixture.packet, fixture.sourceBytes, fixture.targetBytes, fixture.snapshots, fixture.catalog)
	valid, ok := result.(specmigrationv2.ValidAnalysis)
	if !ok {
		t.Fatalf("AnalyzeStructure result = %T, want ValidAnalysis", result)
	}
	analysis := valid.Analysis()
	if analysis.PacketID().String() != "migration-fixture-v2" {
		t.Fatalf("packet ID = %q", analysis.PacketID().String())
	}
	if analysis.DispositionCount() != 4 {
		t.Fatalf("disposition count = %d, want 4", analysis.DispositionCount())
	}
	if len(analysis.LineagePolicy().Entries()) != 5 {
		t.Fatalf("lineage entries = %d, want 5", len(analysis.LineagePolicy().Entries()))
	}
}

func TestDryRunRejectsExactSourceDrift(t *testing.T) {
	fixture := newFixture(t)
	drifted := append([]byte{}, fixture.sourceBytes...)
	drifted[0] = 'X'
	result := fixture.analyze(t, fixture.packet, drifted, fixture.targetBytes, fixture.snapshots, fixture.catalog)
	assertDiagnostic(t, result, specmigrationv2.DiagnosticSourceDigestMismatch)
}

func TestDryRunRejectsDuplicateSourceDisposition(t *testing.T) {
	fixture := newFixture(t)
	dispositions := append([]specmigrationv2.SourceDisposition{}, fixture.dispositions...)
	dispositions = append(dispositions, fixture.dispositions[0])
	packet := fixture.packetWith(t, dispositions, fixture.registry)
	result := fixture.analyze(t, packet, fixture.sourceBytes, fixture.targetBytes, fixture.snapshots, fixture.catalog)
	assertDiagnostic(t, result, specmigrationv2.DiagnosticDuplicateSourceDisposition)
}

func TestDryRunRejectsMissingSourceDisposition(t *testing.T) {
	fixture := newFixture(t)
	dispositions := append([]specmigrationv2.SourceDisposition{}, fixture.dispositions[:3]...)
	packet := fixture.packetWith(t, dispositions, fixture.registry)
	result := fixture.analyze(t, packet, fixture.sourceBytes, fixture.targetBytes, fixture.snapshots, fixture.catalog)
	assertDiagnostic(t, result, specmigrationv2.DiagnosticMissingSourceDisposition)
}

func TestDryRunRejectsSourceSectionOmittedFromBothInventoryAndDispositions(t *testing.T) {
	fixture := newFixture(t)
	sections := append([]specmigrationv2.SourceSection{}, fixture.sections[:3]...)
	source := mustSourceManifest(
		t,
		fixture.sourceCarrier,
		fixture.archiveCarrier,
		fixture.sourceBytes,
		sections,
	)
	dispositions := append([]specmigrationv2.SourceDisposition{}, fixture.dispositions[:3]...)
	packet := mustPacket(t, source, fixture.target, fixture.registry, dispositions)
	result := fixture.analyze(t, packet, fixture.sourceBytes, fixture.targetBytes, fixture.snapshots, fixture.catalog)
	assertDiagnostic(t, result, specmigrationv2.DiagnosticSourceInventoryMissing)
}

func TestSplitOneToManyRejectsOneBranch(t *testing.T) {
	fixture := newFixture(t)
	branch := fixture.splitBranches[0]

	_, err := specmigrationv2.NewSplitOneToMany([]specmigrationv2.SplitBranch{branch})
	if err == nil {
		t.Fatal("NewSplitOneToMany accepted one branch")
	}
}

func TestDryRunRejectsOverlappingSplitPartition(t *testing.T) {
	fixture := newFixture(t)
	splitSection := fixture.sections[1]
	span := splitSection.Span()
	firstBytes := fixture.sourceBytes[span.Start() : span.Start()+4]
	secondBytes := fixture.sourceBytes[span.Start()+3 : span.End()]
	firstSpan := mustSpan(t, span.Start(), firstBytes)
	secondSpan := mustSpan(t, span.Start()+3, secondBytes)
	mapBranch := mustSplitBranch(t, firstSpan, fixture.mapA)
	retireBranch := mustSplitBranch(t, secondSpan, fixture.retire)
	split := mustSplit(t, []specmigrationv2.SplitBranch{mapBranch, retireBranch})
	disposition := mustSourceDisposition(t, splitSection.ID(), split)
	dispositions := append([]specmigrationv2.SourceDisposition{}, fixture.dispositions...)
	dispositions[1] = disposition
	packet := fixture.packetWith(t, dispositions, fixture.registry)
	result := fixture.analyze(t, packet, fixture.sourceBytes, fixture.targetBytes, fixture.snapshots, fixture.catalog)
	assertDiagnostic(t, result, specmigrationv2.DiagnosticSplitFragmentOverlap)
}

func TestDryRunRejectsUnresolvedOutsideCarrier(t *testing.T) {
	fixture := newFixture(t)
	emptyRegistry, err := specmigrationv2.NewOutsideCarrierRegistry([]specmigrationv2.OutsideCarrierRegistration{})
	if err != nil {
		t.Fatalf("NewOutsideCarrierRegistry: %v", err)
	}
	packet := fixture.packetWith(t, fixture.dispositions, emptyRegistry)
	result := fixture.analyze(t, packet, fixture.sourceBytes, fixture.targetBytes, fixture.snapshots, fixture.catalog)
	assertDiagnostic(t, result, specmigrationv2.DiagnosticOutsideCarrierUnregistered)
}

func TestDryRunRejectsRegisteredOutsideCarrierWithoutObservedSnapshot(t *testing.T) {
	fixture := newFixture(t)
	emptySnapshots, err := specmigrationv2.NewOutsideCarrierSnapshots([]specmigrationv2.OutsideCarrierSnapshot{})
	if err != nil {
		t.Fatalf("NewOutsideCarrierSnapshots: %v", err)
	}
	result := fixture.analyze(t, fixture.packet, fixture.sourceBytes, fixture.targetBytes, emptySnapshots, fixture.catalog)
	assertDiagnostic(t, result, specmigrationv2.DiagnosticOutsideCarrierUnresolved)
}

func TestDryRunRejectsTargetDigestMismatch(t *testing.T) {
	fixture := newFixture(t)
	drifted := append([]byte{}, fixture.targetBytes...)
	drifted[0] = 'X'
	result := fixture.analyze(t, fixture.packet, fixture.sourceBytes, drifted, fixture.snapshots, fixture.catalog)
	assertDiagnostic(t, result, specmigrationv2.DiagnosticTargetDigestMismatch)
}

func TestDryRunRejectsTargetClaimAbsentFromDigestPinnedCatalog(t *testing.T) {
	fixture := newFixture(t)
	missingClaim := mustTargetClaimID(t, "SS.alpha.001.D9")
	missingMap := mustMapOne(t, []specmigrationv2.TargetAtomicClaimID{missingClaim})
	dispositions := append([]specmigrationv2.SourceDisposition{}, fixture.dispositions...)
	dispositions[0] = mustSourceDisposition(t, fixture.sections[0].ID(), missingMap)
	packet := fixture.packetWith(t, dispositions, fixture.registry)
	result := fixture.analyze(t, packet, fixture.sourceBytes, fixture.targetBytes, fixture.snapshots, fixture.catalog)
	assertDiagnostic(t, result, specmigrationv2.DiagnosticTargetClaimMissing)
}

func TestTargetClaimCatalogRejectsClaimNamesMentionedOnlyInProse(t *testing.T) {
	carrier := mustTargetCarrierID(t, ".haft/specs/software-system.md")
	bytes := []byte("# prose mentions SS.alpha.001.L1 but defines no spec-section")

	_, err := specmigrationv2.NewTargetClaimCatalog(specmigrationv2.TargetClaimCatalogInput{
		Carrier: carrier,
		Bytes:   bytes,
	})
	if err == nil {
		t.Fatal("NewTargetClaimCatalog accepted claim names mentioned only in prose")
	}
}

func TestTargetClaimCatalogRejectsMalformedClaimClass(t *testing.T) {
	carrier := mustTargetCarrierID(t, ".haft/specs/software-system.md")
	bytes := []byte("## SS.alpha.001 Alpha\n\n```yaml spec-section\nid: SS.alpha.001\nspec: software-system\nkind: software.functional_behavior\nstatus: draft\nclaims:\n  - id: SS.alpha.001.D1\n    class: L\n    statement: Wrong class.\n    scope: [fixture]\n```\n")

	_, err := specmigrationv2.NewTargetClaimCatalog(specmigrationv2.TargetClaimCatalogInput{
		Carrier: carrier,
		Bytes:   bytes,
	})
	if err == nil {
		t.Fatal("NewTargetClaimCatalog accepted claim class inconsistent with its exact ID")
	}
}

func TestTargetClaimCatalogRejectsMalformedYAML(t *testing.T) {
	carrier := mustTargetCarrierID(t, ".haft/specs/software-system.md")
	bytes := []byte("## SS.alpha.001 Alpha\n\n```yaml spec-section\nid: [unterminated\n```\n")

	_, err := specmigrationv2.NewTargetClaimCatalog(specmigrationv2.TargetClaimCatalogInput{
		Carrier: carrier,
		Bytes:   bytes,
	})
	if err == nil {
		t.Fatal("NewTargetClaimCatalog accepted malformed YAML")
	}
}

func TestTargetClaimCatalogRejectsMultipleYAMLDocumentsInOneSection(t *testing.T) {
	carrier := mustTargetCarrierID(t, ".haft/specs/software-system.md")
	bytes := []byte("## SS.alpha.001 Alpha\n\n```yaml spec-section\nid: SS.alpha.001\nspec: software-system\nkind: software.functional_behavior\nstatus: draft\nclaims:\n  - id: SS.alpha.001.L1\n    class: L\n    statement: First document.\n    scope: [fixture]\n---\nid: SS.beta.001\nspec: software-system\nkind: software.functional_behavior\nstatus: draft\nclaims:\n  - id: SS.beta.001.L1\n    class: L\n    statement: Hidden second document.\n    scope: [fixture]\n```\n")

	_, err := specmigrationv2.NewTargetClaimCatalog(specmigrationv2.TargetClaimCatalogInput{
		Carrier: carrier,
		Bytes:   bytes,
	})
	if err == nil {
		t.Fatal("NewTargetClaimCatalog accepted multiple YAML documents in one section")
	}
}

func TestTargetClaimCatalogIgnoresCommentedAndExampleSpecFences(t *testing.T) {
	carrier := mustTargetCarrierID(t, ".haft/specs/software-system.md")
	bytes := []byte("<!--\n## SS.hidden-comment.001 Hidden\n```yaml spec-section\nid: SS.hidden-comment.001\nspec: software-system\nkind: software.functional_behavior\nstatus: draft\nclaims:\n  - id: SS.hidden-comment.001.L1\n    class: L\n    statement: Hidden comment.\n    scope: [fixture]\n```\n-->\n\n~~~~markdown\n## SS.hidden-example.001 Hidden\n```yaml spec-section\nid: SS.hidden-example.001\nspec: software-system\nkind: software.functional_behavior\nstatus: draft\nclaims:\n  - id: SS.hidden-example.001.L1\n    class: L\n    statement: Hidden example.\n    scope: [fixture]\n```\n~~~~\n\n## SS.visible.001 Visible\n\n```yaml spec-section\nid: SS.visible.001\nspec: software-system\nkind: software.functional_behavior\nstatus: draft\nclaims:\n  - id: SS.visible.001.L1\n    class: L\n    statement: Visible source unit.\n    scope: [fixture]\n```\n")

	catalog, err := specmigrationv2.NewTargetClaimCatalog(specmigrationv2.TargetClaimCatalogInput{
		Carrier: carrier,
		Bytes:   bytes,
	})
	if err != nil {
		t.Fatalf("NewTargetClaimCatalog: %v", err)
	}
	claims := catalog.Claims()
	if len(claims) != 1 || claims[0].String() != "SS.visible.001.L1" {
		t.Fatalf("claims = %#v, want only visible source claim", claims)
	}
}

func TestDryRunRejectsUnclassifiedSourcePrologueBytes(t *testing.T) {
	fixture := newFixture(t)
	prefix := []byte("unclassified source meaning\n")
	sourceBytes := append(prefix, fixture.sourceBytes...)
	result := fixture.analyze(t, fixture.packet, sourceBytes, fixture.targetBytes, fixture.snapshots, fixture.catalog)
	assertDiagnostic(t, result, specmigrationv2.DiagnosticSourceInventoryParseFailed)
}

func TestDryRunRejectsNonESSiblingH2InsteadOfSwallowingItIntoPriorSection(t *testing.T) {
	fixture := newFixture(t)
	sourceBytes := strings.Replace(
		string(fixture.sourceBytes),
		"## ES.retire.001 Retire",
		"## Unclassified policy\n\nUnclassified meaning.\n\n## ES.retire.001 Retire",
		1,
	)
	result := fixture.analyze(t, fixture.packet, []byte(sourceBytes), fixture.targetBytes, fixture.snapshots, fixture.catalog)
	assertDiagnostic(t, result, specmigrationv2.DiagnosticSourceInventoryParseFailed)
}

func TestDryRunRejectsUnexpectedTopLevelH1InsideSourceInventory(t *testing.T) {
	fixture := newFixture(t)
	sourceBytes := strings.Replace(
		string(fixture.sourceBytes),
		"## ES.retire.001 Retire",
		"# Unexpected replacement title\n\nUnclassified meaning.\n\n## ES.retire.001 Retire",
		1,
	)
	result := fixture.analyze(t, fixture.packet, []byte(sourceBytes), fixture.targetBytes, fixture.snapshots, fixture.catalog)
	assertDiagnostic(t, result, specmigrationv2.DiagnosticSourceInventoryParseFailed)
}

func TestDryRunRejectsSourceSectionWithWrongSpecKind(t *testing.T) {
	fixture := newFixture(t)
	sourceBytes := strings.Replace(
		string(fixture.sourceBytes),
		"spec: enabling-system",
		"spec: software-system",
		1,
	)
	result := fixture.analyze(t, fixture.packet, []byte(sourceBytes), fixture.targetBytes, fixture.snapshots, fixture.catalog)
	assertDiagnostic(t, result, specmigrationv2.DiagnosticSourceInventoryParseFailed)
}

func TestTargetClaimCatalogRejectsUnknownSoftwareKindAndNonDraftLifecycle(t *testing.T) {
	carrier := mustTargetCarrierID(t, ".haft/specs/software-system.md")
	unknownKind := []byte("## SS.alpha.001 Alpha\n\n```yaml spec-section\nid: SS.alpha.001\nspec: software-system\nkind: software.not_a_real_kind\nstatus: draft\nclaims:\n  - id: SS.alpha.001.L1\n    class: L\n    statement: Claim.\n    scope: [fixture]\n```\n")
	active := []byte("## SS.alpha.001 Alpha\n\n```yaml spec-section\nid: SS.alpha.001\nspec: software-system\nkind: software.functional_behavior\nstatus: active\nclaims:\n  - id: SS.alpha.001.L1\n    class: L\n    statement: Claim.\n    scope: [fixture]\n```\n")

	if _, err := specmigrationv2.NewTargetClaimCatalog(specmigrationv2.TargetClaimCatalogInput{
		Carrier: carrier,
		Bytes:   unknownKind,
	}); err == nil {
		t.Fatal("NewTargetClaimCatalog accepted an unknown software kind")
	}
	if _, err := specmigrationv2.NewTargetClaimCatalog(specmigrationv2.TargetClaimCatalogInput{
		Carrier: carrier,
		Bytes:   active,
	}); err == nil {
		t.Fatal("NewTargetClaimCatalog accepted a non-draft review target")
	}
}

func TestTargetClaimCatalogRejectsHeadingAndYAMLMismatch(t *testing.T) {
	carrier := mustTargetCarrierID(t, ".haft/specs/software-system.md")
	bytes := []byte("## SS.alpha.001 Alpha\n\n```yaml spec-section\nid: SS.beta.001\nspec: software-system\nkind: software.functional_behavior\nstatus: draft\nclaims:\n  - id: SS.beta.001.L1\n    class: L\n    statement: Claim.\n    scope: [fixture]\n```\n")

	_, err := specmigrationv2.NewTargetClaimCatalog(specmigrationv2.TargetClaimCatalogInput{
		Carrier: carrier,
		Bytes:   bytes,
	})
	if err == nil {
		t.Fatal("NewTargetClaimCatalog accepted a heading/YAML section mismatch")
	}
}

func TestDryRunDeclaredNonSoftwareFailsClosedWithoutSealedAdmission(t *testing.T) {
	fixture := newFixture(t)
	profile := declaredNonSoftwareProfile(t)
	invalidSource := []byte("not the pinned source")
	invalidTarget := []byte("not the pinned target")
	request := fixture.request(t, fixture.packet, profile, invalidSource, invalidTarget, fixture.snapshots, fixture.catalog)

	result := specmigrationv2.DryRun(request)
	assertUnderdetermined(
		t,
		result,
		projectprofile.MissingCanonicalDurableProfileAdmission,
	)
}

func TestDryRunReturnsUnderdeterminedBeforePacketValidationForAutoProfile(t *testing.T) {
	fixture := newFixture(t)
	profile := projectprofile.Auto{}
	invalidSource := []byte("not the pinned source")
	invalidTarget := []byte("not the pinned target")
	request := fixture.request(t, fixture.packet, profile, invalidSource, invalidTarget, fixture.snapshots, fixture.catalog)

	result := specmigrationv2.DryRun(request)
	assertUnderdetermined(t, result, projectprofile.MissingAuthoritativeProfile)
}

func TestDryRunTreatsLegacyDeclarationAsMissingCanonicalDurableAdmission(t *testing.T) {
	fixture := newFixture(t)
	profile := declaredSoftwareProfile(t)
	request := fixture.request(
		t,
		fixture.packet,
		profile,
		fixture.sourceBytes,
		fixture.targetBytes,
		fixture.snapshots,
		fixture.catalog,
	)

	result := specmigrationv2.DryRun(request)
	assertUnderdetermined(
		t,
		result,
		projectprofile.MissingCanonicalDurableProfileAdmission,
	)
}

func TestDryRunCannotReachEffectUsableMigrationFromLegacyProfile(t *testing.T) {
	fixture := newFixture(t)
	profile := declaredSoftwareProfile(t)
	request := fixture.request(
		t,
		fixture.packet,
		profile,
		fixture.sourceBytes,
		fixture.targetBytes,
		fixture.snapshots,
		fixture.catalog,
	)

	result := specmigrationv2.DryRun(request)
	assertUnderdetermined(
		t,
		result,
		projectprofile.MissingCanonicalDurableProfileAdmission,
	)
}

func TestDispositionConstructorsRejectUnresolvedShapes(t *testing.T) {
	_, mapErr := specmigrationv2.NewTargetClaimSet([]specmigrationv2.TargetAtomicClaimID{})
	if mapErr == nil {
		t.Error("NewTargetClaimSet accepted an empty target set")
	}
	_, outsideErr := specmigrationv2.NewOutsideCarrierSet([]specmigrationv2.OutsideCarrierID{})
	if outsideErr == nil {
		t.Error("NewOutsideCarrierSet accepted an empty outside carrier set")
	}
	_, retireErr := specmigrationv2.NewRetireHistory("   ")
	if retireErr == nil {
		t.Error("NewRetireHistory accepted an empty reason")
	}
}

func TestMapOneRejectsClaimsAcrossTargetSections(t *testing.T) {
	first := mustTargetClaimID(t, "SS.alpha.001.L1")
	second := mustTargetClaimID(t, "SS.beta.001.A1")

	_, err := specmigrationv2.NewTargetClaimSet([]specmigrationv2.TargetAtomicClaimID{first, second})
	if err == nil {
		t.Fatal("NewTargetClaimSet accepted claims from two target sections")
	}
}

func TestDryRunRejectsOutsideCarrierCollisionWithMigrationCarrier(t *testing.T) {
	fixture := newFixture(t)
	registration := mustOutsideRegistration(
		t,
		fixture.outsideID,
		fixture.sourceCarrier,
		fixture.outsideBytes,
	)
	registry, err := specmigrationv2.NewOutsideCarrierRegistry(
		[]specmigrationv2.OutsideCarrierRegistration{registration},
	)
	if err != nil {
		t.Fatalf("NewOutsideCarrierRegistry: %v", err)
	}
	snapshot := mustOutsideSnapshot(
		t,
		fixture.outsideID,
		fixture.sourceCarrier,
		fixture.outsideBytes,
	)
	snapshots, err := specmigrationv2.NewOutsideCarrierSnapshots(
		[]specmigrationv2.OutsideCarrierSnapshot{snapshot},
	)
	if err != nil {
		t.Fatalf("NewOutsideCarrierSnapshots: %v", err)
	}
	packet := fixture.packetWith(t, fixture.dispositions, registry)
	result := fixture.analyze(t, packet, fixture.sourceBytes, fixture.targetBytes, snapshots, fixture.catalog)
	assertDiagnostic(t, result, specmigrationv2.DiagnosticCarrierCollision)
}

func TestOutsideCarrierCollectionsRejectTwoIdentitiesForOnePath(t *testing.T) {
	firstID := mustOutsideCarrierID(t, "outside-a")
	secondID := mustOutsideCarrierID(t, "outside-b")
	carrier := mustSourceCarrierID(t, "AGENTS.md")
	firstBytes := []byte("first")
	secondBytes := []byte("second")
	firstRegistration := mustOutsideRegistration(t, firstID, carrier, firstBytes)
	secondRegistration := mustOutsideRegistration(t, secondID, carrier, secondBytes)
	firstSnapshot := mustOutsideSnapshot(t, firstID, carrier, firstBytes)
	secondSnapshot := mustOutsideSnapshot(t, secondID, carrier, secondBytes)

	if _, err := specmigrationv2.NewOutsideCarrierRegistry(
		[]specmigrationv2.OutsideCarrierRegistration{firstRegistration, secondRegistration},
	); err == nil {
		t.Fatal("NewOutsideCarrierRegistry accepted two identities for one path")
	}
	if _, err := specmigrationv2.NewOutsideCarrierSnapshots(
		[]specmigrationv2.OutsideCarrierSnapshot{firstSnapshot, secondSnapshot},
	); err == nil {
		t.Fatal("NewOutsideCarrierSnapshots accepted two identities for one path")
	}
}

func TestAnalyzeStructureRejectsDesignatedSourceProvenanceFromAnotherProjectRoot(t *testing.T) {
	fixture := newFixture(t)
	provenance := mustRepositoryProvenance(
		t,
		"project-root:other",
		fixture.sourceCarrier,
		specmigrationv2.SourceDigestOf(fixture.sourceBytes),
		"review:other-source-designation",
	)
	source := mustSourceManifestWithProvenance(
		t,
		fixture.sourceCarrier,
		fixture.archiveCarrier,
		fixture.sourceBytes,
		fixture.sections,
		provenance,
	)
	packet := mustPacket(t, source, fixture.target, fixture.registry, fixture.dispositions)
	projectRoot := mustProjectRootRef(t, "project-root:haft")
	request := fixture.structuralRequest(
		t,
		packet,
		projectRoot,
		fixture.sourceBytes,
		fixture.targetBytes,
		fixture.snapshots,
		fixture.catalog,
	)
	result := specmigrationv2.AnalyzeStructure(request)
	assertDiagnostic(t, result, specmigrationv2.DiagnosticSourceProvenanceRootMismatch)
}

func TestPacketCompilesOneLineageEntryPerLeafDisposition(t *testing.T) {
	fixture := newFixture(t)
	policy := fixture.packet.LineagePolicy()
	entries := policy.Entries()
	if len(entries) != 5 {
		t.Fatalf("lineage entries = %d, want 5", len(entries))
	}
	mapped := 0
	history := 0
	outside := 0
	for _, entry := range entries {
		switch entry.Outcome().(type) {
		case specmigrationv2.MeaningMappedToTargetClaims:
			mapped++
		case specmigrationv2.RetainedAsHistoryOnly:
			history++
		case specmigrationv2.ContinuesOutsidePSS:
			outside++
		default:
			t.Fatalf("unknown lineage outcome %T", entry.Outcome())
		}
	}
	if mapped != 2 || history != 2 || outside != 1 {
		t.Fatalf("lineage outcome counts = mapped:%d history:%d outside:%d", mapped, history, outside)
	}
}

func TestCompiledLineageRetainsDispositionMeaningAndResolvedOutsideBinding(t *testing.T) {
	fixture := newFixture(t)
	entries := fixture.packet.LineagePolicy().Entries()
	history := 0
	outside := 0
	for _, entry := range entries {
		switch outcome := entry.Outcome().(type) {
		case specmigrationv2.RetainedAsHistoryOnly:
			history++
			if outcome.Reason() != "legacy meaning is retained only in history" {
				t.Fatalf("history reason = %q", outcome.Reason())
			}
		case specmigrationv2.ContinuesOutsidePSS:
			outside++
			if outcome.Meaning() != "operator discipline remains outside PSS" {
				t.Fatalf("outside meaning = %q", outcome.Meaning())
			}
			resolved := outcome.ResolvedCarriers()
			if len(resolved) != 1 {
				t.Fatalf("resolved outside bindings = %d, want 1", len(resolved))
			}
			binding := resolved[0]
			if binding.ID().String() != fixture.outsideID.String() {
				t.Fatalf("resolved outside ID = %q", binding.ID().String())
			}
			if binding.Carrier().String() != "AGENTS.md" {
				t.Fatalf("resolved outside carrier = %q", binding.Carrier().String())
			}
			wantDigest := specmigrationv2.OutsideCarrierDigestOf(fixture.outsideBytes)
			if binding.Digest().String() != wantDigest.String() {
				t.Fatalf("resolved outside digest = %q, want %q", binding.Digest().String(), wantDigest.String())
			}
		}
	}
	if history != 2 || outside != 1 {
		t.Fatalf("lossless lineage outcomes = history:%d outside:%d", history, outside)
	}
}

func TestInvalidDiagnosticsAreDeterministic(t *testing.T) {
	fixture := newFixture(t)
	dispositions := []specmigrationv2.SourceDisposition{fixture.dispositions[0]}
	packet := fixture.packetWith(t, dispositions, fixture.registry)
	request := fixture.defaultStructuralRequest(
		t,
		packet,
		fixture.sourceBytes,
		fixture.targetBytes,
		fixture.snapshots,
		fixture.catalog,
	)

	first := diagnosticSignature(t, specmigrationv2.AnalyzeStructure(request))
	for attempt := 0; attempt < 20; attempt++ {
		observed := diagnosticSignature(t, specmigrationv2.AnalyzeStructure(request))
		if observed != first {
			t.Fatalf("diagnostic order changed: first=%q observed=%q", first, observed)
		}
	}
}

type fixture struct {
	sourceBytes    []byte
	targetBytes    []byte
	outsideBytes   []byte
	sourceCarrier  specmigrationv2.SourceCarrierID
	targetCarrier  specmigrationv2.TargetCarrierID
	archiveCarrier specmigrationv2.ArchiveCarrierID
	outsideID      specmigrationv2.OutsideCarrierID
	sections       []specmigrationv2.SourceSection
	claimL         specmigrationv2.TargetAtomicClaimID
	claimA         specmigrationv2.TargetAtomicClaimID
	mapA           specmigrationv2.MapOne
	retire         specmigrationv2.RetireHistory
	splitBranches  []specmigrationv2.SplitBranch
	dispositions   []specmigrationv2.SourceDisposition
	registry       specmigrationv2.OutsideCarrierRegistry
	snapshots      specmigrationv2.OutsideCarrierSnapshots
	catalog        specmigrationv2.TargetClaimCatalog
	source         specmigrationv2.SourceManifest
	target         specmigrationv2.TargetManifest
	packet         specmigrationv2.Packet
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	parts := [][]byte{
		[]byte("## ES.map.001 Map\n\n```yaml spec-section\nid: ES.map.001\nspec: enabling-system\nkind: enabling.map\nstatus: draft\n```\n\n"),
		[]byte("## ES.split.001 Split\n\n```yaml spec-section\nid: ES.split.001\nspec: enabling-system\nkind: enabling.split\nstatus: draft\n```\n\n"),
		[]byte("## ES.retire.001 Retire\n\n```yaml spec-section\nid: ES.retire.001\nspec: enabling-system\nkind: enabling.retire\nstatus: draft\n```\n\n"),
		[]byte("## ES.outside.001 Outside\n\n```yaml spec-section\nid: ES.outside.001\nspec: enabling-system\nkind: enabling.outside\nstatus: draft\n```\n"),
	}
	sourceBytes := []byte{}
	sections := make([]specmigrationv2.SourceSection, 0, len(parts))
	sectionNames := []string{"ES.map.001", "ES.split.001", "ES.retire.001", "ES.outside.001"}
	for index, part := range parts {
		start := uint64(len(sourceBytes))
		sourceBytes = append(sourceBytes, part...)
		span := mustSpan(t, start, part)
		sectionID := mustSourceSectionID(t, sectionNames[index])
		section := mustSourceSection(t, sectionID, span)
		sections = append(sections, section)
	}
	targetBytes := []byte("# Exact target\n\n## SS.alpha.001 Alpha\n\n```yaml spec-section\nid: SS.alpha.001\nspec: software-system\nkind: software.functional_behavior\nstatus: draft\nclaims:\n  - id: SS.alpha.001.L1\n    class: L\n    statement: Fixture law claim.\n    scope:\n      - fixture\n  - id: SS.alpha.001.A1\n    class: A\n    statement: Fixture admissibility claim.\n    scope:\n      - fixture\n```\n")
	outsideBytes := []byte("# exact outside carrier")
	sourceCarrier := mustSourceCarrierID(t, ".haft/specs/enabling-system.md")
	targetCarrier := mustTargetCarrierID(t, ".haft/specs/software-system.md")
	archiveCarrier := mustArchiveCarrierID(t, ".haft/migration-archive/enabling-system.md")
	outsideID := mustOutsideCarrierID(t, "host_discipline")
	outsideCarrier := mustSourceCarrierID(t, "AGENTS.md")
	claimL := mustTargetClaimID(t, "SS.alpha.001.L1")
	claimA := mustTargetClaimID(t, "SS.alpha.001.A1")
	mapL := mustMapOne(t, []specmigrationv2.TargetAtomicClaimID{claimL})
	mapA := mustMapOne(t, []specmigrationv2.TargetAtomicClaimID{claimA})
	retire := mustRetire(t, "legacy meaning is retained only in history")
	outside := mustOutside(t, "operator discipline remains outside PSS", []specmigrationv2.OutsideCarrierID{outsideID})

	splitSpan := sections[1].Span()
	firstSplitBytes := sourceBytes[splitSpan.Start() : splitSpan.Start()+3]
	secondSplitBytes := sourceBytes[splitSpan.Start()+3 : splitSpan.End()]
	firstSplitSpan := mustSpan(t, splitSpan.Start(), firstSplitBytes)
	secondSplitSpan := mustSpan(t, splitSpan.Start()+3, secondSplitBytes)
	firstBranch := mustSplitBranch(t, firstSplitSpan, mapA)
	secondBranch := mustSplitBranch(t, secondSplitSpan, retire)
	splitBranches := []specmigrationv2.SplitBranch{firstBranch, secondBranch}
	split := mustSplit(t, splitBranches)

	dispositions := []specmigrationv2.SourceDisposition{
		mustSourceDisposition(t, sections[0].ID(), mapL),
		mustSourceDisposition(t, sections[1].ID(), split),
		mustSourceDisposition(t, sections[2].ID(), retire),
		mustSourceDisposition(t, sections[3].ID(), outside),
	}
	registration := mustOutsideRegistration(t, outsideID, outsideCarrier, outsideBytes)
	registry, err := specmigrationv2.NewOutsideCarrierRegistry([]specmigrationv2.OutsideCarrierRegistration{registration})
	if err != nil {
		t.Fatalf("NewOutsideCarrierRegistry: %v", err)
	}
	snapshot := mustOutsideSnapshot(t, outsideID, outsideCarrier, outsideBytes)
	snapshots, err := specmigrationv2.NewOutsideCarrierSnapshots([]specmigrationv2.OutsideCarrierSnapshot{snapshot})
	if err != nil {
		t.Fatalf("NewOutsideCarrierSnapshots: %v", err)
	}
	catalog := mustTargetCatalog(t, targetCarrier, targetBytes)
	source := mustSourceManifest(t, sourceCarrier, archiveCarrier, sourceBytes, sections)
	target := mustTargetManifest(t, targetCarrier, targetBytes)
	packet := mustPacket(t, source, target, registry, dispositions)
	return fixture{
		sourceBytes:    sourceBytes,
		targetBytes:    targetBytes,
		outsideBytes:   outsideBytes,
		sourceCarrier:  sourceCarrier,
		targetCarrier:  targetCarrier,
		archiveCarrier: archiveCarrier,
		outsideID:      outsideID,
		sections:       sections,
		claimL:         claimL,
		claimA:         claimA,
		mapA:           mapA,
		retire:         retire,
		splitBranches:  splitBranches,
		dispositions:   dispositions,
		registry:       registry,
		snapshots:      snapshots,
		catalog:        catalog,
		source:         source,
		target:         target,
		packet:         packet,
	}
}

func (fixture fixture) packetWith(
	t *testing.T,
	dispositions []specmigrationv2.SourceDisposition,
	registry specmigrationv2.OutsideCarrierRegistry,
) specmigrationv2.Packet {
	t.Helper()
	return mustPacket(t, fixture.source, fixture.target, registry, dispositions)
}

func (fixture fixture) analyze(
	t *testing.T,
	packet specmigrationv2.Packet,
	sourceBytes []byte,
	targetBytes []byte,
	snapshots specmigrationv2.OutsideCarrierSnapshots,
	catalog specmigrationv2.TargetClaimCatalog,
) specmigrationv2.StructuralAnalysisResult {
	t.Helper()
	request := fixture.defaultStructuralRequest(
		t,
		packet,
		sourceBytes,
		targetBytes,
		snapshots,
		catalog,
	)
	return specmigrationv2.AnalyzeStructure(request)
}

func (fixture fixture) defaultStructuralRequest(
	t *testing.T,
	packet specmigrationv2.Packet,
	sourceBytes []byte,
	targetBytes []byte,
	snapshots specmigrationv2.OutsideCarrierSnapshots,
	catalog specmigrationv2.TargetClaimCatalog,
) specmigrationv2.StructuralRequest {
	t.Helper()
	projectRoot := mustProjectRootRef(t, "project-root:haft")
	return fixture.structuralRequest(
		t,
		packet,
		projectRoot,
		sourceBytes,
		targetBytes,
		snapshots,
		catalog,
	)
}

func (fixture fixture) structuralRequest(
	t *testing.T,
	packet specmigrationv2.Packet,
	projectRoot specmigrationv2.ProjectRootRef,
	sourceBytes []byte,
	targetBytes []byte,
	snapshots specmigrationv2.OutsideCarrierSnapshots,
	catalog specmigrationv2.TargetClaimCatalog,
) specmigrationv2.StructuralRequest {
	t.Helper()
	source := mustSourceSnapshot(t, fixture.sourceCarrier, sourceBytes)
	target := mustTargetSnapshot(t, fixture.targetCarrier, targetBytes)
	request, err := specmigrationv2.NewStructuralRequest(specmigrationv2.StructuralRequestInput{
		Packet:           packet,
		ProjectRoot:      projectRoot,
		Source:           source,
		Target:           target,
		TargetClaims:     catalog,
		OutsideSnapshots: snapshots,
	})
	if err != nil {
		t.Fatalf("NewStructuralRequest: %v", err)
	}
	return request
}

func (fixture fixture) request(
	t *testing.T,
	packet specmigrationv2.Packet,
	profile projectprofile.ConfiguredProjectProfile,
	sourceBytes []byte,
	targetBytes []byte,
	snapshots specmigrationv2.OutsideCarrierSnapshots,
	catalog specmigrationv2.TargetClaimCatalog,
) specmigrationv2.DryRunRequest {
	t.Helper()
	source, err := specmigrationv2.NewSourceSnapshot(specmigrationv2.SourceSnapshotInput{
		Carrier: fixture.sourceCarrier,
		Bytes:   sourceBytes,
	})
	if err != nil {
		t.Fatalf("NewSourceSnapshot: %v", err)
	}
	target, err := specmigrationv2.NewTargetSnapshot(specmigrationv2.TargetSnapshotInput{
		Carrier: fixture.targetCarrier,
		Bytes:   targetBytes,
	})
	if err != nil {
		t.Fatalf("NewTargetSnapshot: %v", err)
	}
	request, err := specmigrationv2.NewDryRunRequest(specmigrationv2.DryRunRequestInput{
		Packet:           packet,
		ProjectRoot:      mustProjectRootRef(t, "project-root:haft"),
		Profile:          profile,
		Review:           mustPendingReview(t),
		Source:           source,
		Target:           target,
		TargetClaims:     catalog,
		OutsideSnapshots: snapshots,
	})
	if err != nil {
		t.Fatalf("NewDryRunRequest: %v", err)
	}
	return request
}

func mustPendingReview(t *testing.T) specmigrationv2.PendingMigrationReview {
	t.Helper()
	missing, err := specmigrationv2.NewReviewMissingBasisSet([]specmigrationv2.ReviewMissingBasis{
		specmigrationv2.MissingHumanSemanticZeroReview,
		specmigrationv2.MissingExactReviewBinding,
		specmigrationv2.MissingLifecycleResolution,
	})
	if err != nil {
		t.Fatalf("NewReviewMissingBasisSet: %v", err)
	}
	review, err := specmigrationv2.NewPendingMigrationReview(missing)
	if err != nil {
		t.Fatalf("NewPendingMigrationReview: %v", err)
	}
	return review
}

func declaredSoftwareProfile(
	t *testing.T,
) projectprofile.ConfiguredProjectProfile {
	t.Helper()
	scopeID := mustScopeID(t, "haft-software")
	scope, err := projectprofile.NewSoftwareRealization(scopeID, projectprofile.NoEntityReference{})
	if err != nil {
		t.Fatalf("NewSoftwareRealization: %v", err)
	}
	return mustDeclaredProfile(t, []projectprofile.RealizationScope{scope})
}

func declaredNonSoftwareProfile(
	t *testing.T,
) projectprofile.ConfiguredProjectProfile {
	t.Helper()
	scopeID := mustScopeID(t, "haft-documents")
	scope, err := projectprofile.NewNonSoftwareRealization(
		scopeID,
		projectprofile.NoEntityReference{},
		projectprofile.UnspecifiedKindOrientation{},
		[]projectprofile.SourceUnitRef{},
		[]projectprofile.SpecSectionRef{},
	)
	if err != nil {
		t.Fatalf("NewNonSoftwareRealization: %v", err)
	}
	return mustDeclaredProfile(t, []projectprofile.RealizationScope{scope})
}

func mustDeclaredProfile(
	t *testing.T,
	scopes []projectprofile.RealizationScope,
) projectprofile.Declared {
	t.Helper()
	scopeSet, err := projectprofile.NewScopeSet(scopes)
	if err != nil {
		t.Fatalf("NewScopeSet: %v", err)
	}
	observedBasis, err := projectprofile.NewObservedBasis("fixture", "explicit test profile")
	if err != nil {
		t.Fatalf("NewObservedBasis: %v", err)
	}
	observedBasisDigest, err := projectprofile.DigestObservedBasis(
		[]projectprofile.ObservedBasis{observedBasis},
	)
	if err != nil {
		t.Fatalf("DigestObservedBasis: %v", err)
	}
	scopePayloadDigest, err := projectprofile.DigestScopePayload(scopeSet)
	if err != nil {
		t.Fatalf("DigestScopePayload: %v", err)
	}
	windowStart := time.Date(2026, 7, 14, 1, 0, 0, 0, time.UTC)
	windowEnd := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	window, err := projectprofile.NewObservationWindow(
		"host-session:test",
		windowStart,
		windowEnd,
	)
	if err != nil {
		t.Fatalf("NewObservationWindow: %v", err)
	}
	revision, err := projectprofile.NewCarrierRevision(1)
	if err != nil {
		t.Fatalf("NewCarrierRevision: %v", err)
	}
	recordBuilder := projectprofile.NewOperatorDeclaredRecordBuilder(
		"authority-basis:legacy-fixture:1",
		"work:fixture-profile-declaration",
	)
	recordBuilder = recordBuilder.ForProject("project-root:haft")
	recordBuilder = recordBuilder.ForScopePayload(scopePayloadDigest)
	recordBuilder = recordBuilder.ForObservedBasis(observedBasisDigest)
	recordBuilder = recordBuilder.ObservedWithin(window)
	recordBuilder = recordBuilder.AtCarrierRevision(revision)
	record, err := recordBuilder.Build()
	if err != nil {
		t.Fatalf("OperatorDeclaredRecordBuilder.Build: %v", err)
	}
	profile, err := projectprofile.NewDeclared(scopeSet, record)
	if err != nil {
		t.Fatalf("NewDeclared: %v", err)
	}
	return profile
}

func mustPacket(
	t *testing.T,
	source specmigrationv2.SourceManifest,
	target specmigrationv2.TargetManifest,
	registry specmigrationv2.OutsideCarrierRegistry,
	dispositions []specmigrationv2.SourceDisposition,
) specmigrationv2.Packet {
	t.Helper()
	id, err := specmigrationv2.NewMigrationPacketID("migration-fixture-v2")
	if err != nil {
		t.Fatalf("NewMigrationPacketID: %v", err)
	}
	packet, err := specmigrationv2.NewPacket(specmigrationv2.PacketInput{
		ID:                 id,
		SchemaVersion:      specmigrationv2.SchemaVersionV2,
		Source:             source,
		Target:             target,
		OutsideRegistry:    registry,
		SourceDispositions: dispositions,
	})
	if err != nil {
		t.Fatalf("NewPacket: %v", err)
	}
	return packet
}

func mustSourceManifest(
	t *testing.T,
	carrier specmigrationv2.SourceCarrierID,
	archiveCarrier specmigrationv2.ArchiveCarrierID,
	bytes []byte,
	sections []specmigrationv2.SourceSection,
) specmigrationv2.SourceManifest {
	t.Helper()
	digest := specmigrationv2.SourceDigestOf(bytes)
	provenance := mustRepositoryProvenance(
		t,
		"project-root:haft",
		carrier,
		digest,
		"review:fixture-source-designation",
	)
	return mustSourceManifestWithProvenance(
		t,
		carrier,
		archiveCarrier,
		bytes,
		sections,
		provenance,
	)
}

func mustSourceManifestWithProvenance(
	t *testing.T,
	carrier specmigrationv2.SourceCarrierID,
	archiveCarrier specmigrationv2.ArchiveCarrierID,
	bytes []byte,
	sections []specmigrationv2.SourceSection,
	provenance specmigrationv2.DesignatedSourceProvenance,
) specmigrationv2.SourceManifest {
	t.Helper()
	length, err := specmigrationv2.NewByteLength(uint64(len(bytes)))
	if err != nil {
		t.Fatalf("NewByteLength: %v", err)
	}
	digest := specmigrationv2.SourceDigestOf(bytes)
	archive, err := specmigrationv2.NewArchiveManifest(archiveCarrier, digest)
	if err != nil {
		t.Fatalf("NewArchiveManifest: %v", err)
	}
	manifest, err := specmigrationv2.NewSourceManifest(specmigrationv2.SourceManifestInput{
		Carrier:    carrier,
		Digest:     digest,
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

func mustRepositoryProvenance(
	t *testing.T,
	projectRootRaw string,
	carrier specmigrationv2.SourceCarrierID,
	digest specmigrationv2.SourceDigest,
	recordRefRaw string,
) specmigrationv2.DesignatedSourceProvenance {
	t.Helper()
	projectRoot, err := specmigrationv2.NewProjectRootRef(projectRootRaw)
	if err != nil {
		t.Fatalf("NewProjectRootRef: %v", err)
	}
	commitOID, err := specmigrationv2.NewGitCommitOID("sha1:" + strings.Repeat("a", 40))
	if err != nil {
		t.Fatalf("NewGitCommitOID: %v", err)
	}
	edition, err := specmigrationv2.NewRepositoryEdition(
		projectRoot,
		commitOID,
		carrier,
		digest,
	)
	if err != nil {
		t.Fatalf("NewRepositoryEdition: %v", err)
	}
	recordRef, err := specmigrationv2.NewProvenanceRecordRef(recordRefRaw)
	if err != nil {
		t.Fatalf("NewProvenanceRecordRef: %v", err)
	}
	recordDigest := specmigrationv2.ProvenanceRecordDigestOf(
		[]byte("fixture source designation record:" + recordRefRaw),
	)
	recordBinding, err := specmigrationv2.NewProvenanceRecordBinding(recordRef, recordDigest)
	if err != nil {
		t.Fatalf("NewProvenanceRecordBinding: %v", err)
	}
	provenance, err := specmigrationv2.NewDesignatedSourceProvenance(edition, recordBinding)
	if err != nil {
		t.Fatalf("NewDesignatedSourceProvenance: %v", err)
	}
	return provenance
}

func mustTargetManifest(
	t *testing.T,
	carrier specmigrationv2.TargetCarrierID,
	bytes []byte,
) specmigrationv2.TargetManifest {
	t.Helper()
	length, err := specmigrationv2.NewByteLength(uint64(len(bytes)))
	if err != nil {
		t.Fatalf("NewByteLength: %v", err)
	}
	manifest, err := specmigrationv2.NewTargetManifest(specmigrationv2.TargetManifestInput{
		Carrier:    carrier,
		Digest:     specmigrationv2.TargetDigestOf(bytes),
		ByteLength: length,
	})
	if err != nil {
		t.Fatalf("NewTargetManifest: %v", err)
	}
	return manifest
}

func mustTargetCatalog(
	t *testing.T,
	carrier specmigrationv2.TargetCarrierID,
	bytes []byte,
) specmigrationv2.TargetClaimCatalog {
	t.Helper()
	catalog, err := specmigrationv2.NewTargetClaimCatalog(specmigrationv2.TargetClaimCatalogInput{
		Carrier: carrier,
		Bytes:   bytes,
	})
	if err != nil {
		t.Fatalf("NewTargetClaimCatalog: %v", err)
	}
	return catalog
}

func mustSourceSnapshot(
	t *testing.T,
	carrier specmigrationv2.SourceCarrierID,
	bytes []byte,
) specmigrationv2.SourceSnapshot {
	t.Helper()
	snapshot, err := specmigrationv2.NewSourceSnapshot(specmigrationv2.SourceSnapshotInput{
		Carrier: carrier,
		Bytes:   bytes,
	})
	if err != nil {
		t.Fatalf("NewSourceSnapshot: %v", err)
	}
	return snapshot
}

func mustTargetSnapshot(
	t *testing.T,
	carrier specmigrationv2.TargetCarrierID,
	bytes []byte,
) specmigrationv2.TargetSnapshot {
	t.Helper()
	snapshot, err := specmigrationv2.NewTargetSnapshot(specmigrationv2.TargetSnapshotInput{
		Carrier: carrier,
		Bytes:   bytes,
	})
	if err != nil {
		t.Fatalf("NewTargetSnapshot: %v", err)
	}
	return snapshot
}

func mustOutsideRegistration(
	t *testing.T,
	id specmigrationv2.OutsideCarrierID,
	carrier specmigrationv2.SourceCarrierID,
	bytes []byte,
) specmigrationv2.OutsideCarrierRegistration {
	t.Helper()
	registration, err := specmigrationv2.NewOutsideCarrierRegistration(specmigrationv2.OutsideCarrierRegistrationInput{
		ID:      id,
		Carrier: carrier,
		Digest:  specmigrationv2.OutsideCarrierDigestOf(bytes),
	})
	if err != nil {
		t.Fatalf("NewOutsideCarrierRegistration: %v", err)
	}
	return registration
}

func mustOutsideSnapshot(
	t *testing.T,
	id specmigrationv2.OutsideCarrierID,
	carrier specmigrationv2.SourceCarrierID,
	bytes []byte,
) specmigrationv2.OutsideCarrierSnapshot {
	t.Helper()
	snapshot, err := specmigrationv2.NewOutsideCarrierSnapshot(specmigrationv2.OutsideCarrierSnapshotInput{
		ID:      id,
		Carrier: carrier,
		Bytes:   bytes,
	})
	if err != nil {
		t.Fatalf("NewOutsideCarrierSnapshot: %v", err)
	}
	return snapshot
}

func mustSourceDisposition(
	t *testing.T,
	section specmigrationv2.SourceSectionID,
	disposition specmigrationv2.Disposition,
) specmigrationv2.SourceDisposition {
	t.Helper()
	result, err := specmigrationv2.NewSourceDisposition(section, disposition)
	if err != nil {
		t.Fatalf("NewSourceDisposition: %v", err)
	}
	return result
}

func mustSplitBranch(
	t *testing.T,
	span specmigrationv2.ExactByteSpan,
	disposition specmigrationv2.BranchDisposition,
) specmigrationv2.SplitBranch {
	t.Helper()
	branch, err := specmigrationv2.NewSplitBranch(span, disposition)
	if err != nil {
		t.Fatalf("NewSplitBranch: %v", err)
	}
	return branch
}

func mustSplit(t *testing.T, branches []specmigrationv2.SplitBranch) specmigrationv2.SplitOneToMany {
	t.Helper()
	split, err := specmigrationv2.NewSplitOneToMany(branches)
	if err != nil {
		t.Fatalf("NewSplitOneToMany: %v", err)
	}
	return split
}

func mustMapOne(t *testing.T, claims []specmigrationv2.TargetAtomicClaimID) specmigrationv2.MapOne {
	t.Helper()
	set, err := specmigrationv2.NewTargetClaimSet(claims)
	if err != nil {
		t.Fatalf("NewTargetClaimSet: %v", err)
	}
	mapping, err := specmigrationv2.NewMapOne(set)
	if err != nil {
		t.Fatalf("NewMapOne: %v", err)
	}
	return mapping
}

func mustOutside(
	t *testing.T,
	meaning string,
	carrierIDs []specmigrationv2.OutsideCarrierID,
) specmigrationv2.OutsidePSS {
	t.Helper()
	set, err := specmigrationv2.NewOutsideCarrierSet(carrierIDs)
	if err != nil {
		t.Fatalf("NewOutsideCarrierSet: %v", err)
	}
	outside, err := specmigrationv2.NewOutsidePSS(meaning, set)
	if err != nil {
		t.Fatalf("NewOutsidePSS: %v", err)
	}
	return outside
}

func mustRetire(t *testing.T, reason string) specmigrationv2.RetireHistory {
	t.Helper()
	retire, err := specmigrationv2.NewRetireHistory(reason)
	if err != nil {
		t.Fatalf("NewRetireHistory: %v", err)
	}
	return retire
}

func mustSpan(t *testing.T, start uint64, bytes []byte) specmigrationv2.ExactByteSpan {
	t.Helper()
	length, err := specmigrationv2.NewByteLength(uint64(len(bytes)))
	if err != nil {
		t.Fatalf("NewByteLength: %v", err)
	}
	span, err := specmigrationv2.NewExactByteSpan(start, length, specmigrationv2.FragmentDigestOf(bytes))
	if err != nil {
		t.Fatalf("NewExactByteSpan: %v", err)
	}
	return span
}

func mustSourceSection(
	t *testing.T,
	id specmigrationv2.SourceSectionID,
	span specmigrationv2.ExactByteSpan,
) specmigrationv2.SourceSection {
	t.Helper()
	section, err := specmigrationv2.NewSourceSection(id, span)
	if err != nil {
		t.Fatalf("NewSourceSection: %v", err)
	}
	return section
}

func mustSourceCarrierID(t *testing.T, raw string) specmigrationv2.SourceCarrierID {
	t.Helper()
	id, err := specmigrationv2.NewSourceCarrierID(raw)
	if err != nil {
		t.Fatalf("NewSourceCarrierID: %v", err)
	}
	return id
}

func mustTargetCarrierID(t *testing.T, raw string) specmigrationv2.TargetCarrierID {
	t.Helper()
	id, err := specmigrationv2.NewTargetCarrierID(raw)
	if err != nil {
		t.Fatalf("NewTargetCarrierID: %v", err)
	}
	return id
}

func mustArchiveCarrierID(t *testing.T, raw string) specmigrationv2.ArchiveCarrierID {
	t.Helper()
	id, err := specmigrationv2.NewArchiveCarrierID(raw)
	if err != nil {
		t.Fatalf("NewArchiveCarrierID: %v", err)
	}
	return id
}

func mustSourceSectionID(t *testing.T, raw string) specmigrationv2.SourceSectionID {
	t.Helper()
	id, err := specmigrationv2.NewSourceSectionID(raw)
	if err != nil {
		t.Fatalf("NewSourceSectionID: %v", err)
	}
	return id
}

func mustTargetClaimID(t *testing.T, raw string) specmigrationv2.TargetAtomicClaimID {
	t.Helper()
	id, err := specmigrationv2.NewTargetAtomicClaimID(raw)
	if err != nil {
		t.Fatalf("NewTargetAtomicClaimID: %v", err)
	}
	return id
}

func mustOutsideCarrierID(t *testing.T, raw string) specmigrationv2.OutsideCarrierID {
	t.Helper()
	id, err := specmigrationv2.NewOutsideCarrierID(raw)
	if err != nil {
		t.Fatalf("NewOutsideCarrierID: %v", err)
	}
	return id
}

func mustScopeID(t *testing.T, raw string) projectprofile.ScopeID {
	t.Helper()
	id, err := projectprofile.NewScopeID(raw)
	if err != nil {
		t.Fatalf("NewScopeID: %v", err)
	}
	return id
}

func mustProjectRootRef(t *testing.T, raw string) specmigrationv2.ProjectRootRef {
	t.Helper()
	ref, err := specmigrationv2.NewProjectRootRef(raw)
	if err != nil {
		t.Fatalf("NewProjectRootRef: %v", err)
	}
	return ref
}

func assertDiagnostic(t *testing.T, result any, code specmigrationv2.DiagnosticCode) {
	t.Helper()
	var diagnostics specmigrationv2.DiagnosticSet
	switch value := result.(type) {
	case specmigrationv2.Invalid:
		diagnostics = value.Diagnostics()
	case specmigrationv2.InvalidDiagnostics:
		diagnostics = value.Diagnostics()
	default:
		t.Fatalf("result = %T, want Invalid or InvalidDiagnostics", result)
	}
	for _, diagnostic := range diagnostics.Values() {
		if diagnostic.Code() == code {
			return
		}
	}
	values := diagnostics.Values()
	codes := make([]string, 0, len(values))
	for _, diagnostic := range values {
		codes = append(codes, string(diagnostic.Code()))
	}
	t.Fatalf("diagnostics = %s, want %s", strings.Join(codes, ", "), code)
}

func assertUnderdetermined(
	t *testing.T,
	result specmigrationv2.DryRunResult,
	want projectprofile.MissingBasis,
) {
	t.Helper()
	underdetermined, ok := result.(specmigrationv2.Underdetermined)
	if !ok {
		t.Fatalf("DryRun result = %T, want Underdetermined", result)
	}
	missing := underdetermined.Applicability().MissingBasis().Values()
	if len(missing) != 1 || missing[0] != want {
		t.Fatalf("missing basis = %#v, want %q", missing, want)
	}
}

func diagnosticSignature(t *testing.T, result any) string {
	t.Helper()
	var diagnostics specmigrationv2.DiagnosticSet
	switch value := result.(type) {
	case specmigrationv2.Invalid:
		diagnostics = value.Diagnostics()
	case specmigrationv2.InvalidDiagnostics:
		diagnostics = value.Diagnostics()
	default:
		t.Fatalf("result = %T, want Invalid or InvalidDiagnostics", result)
	}
	values := diagnostics.Values()
	parts := make([]string, 0, len(values))
	for _, diagnostic := range values {
		part := string(diagnostic.Code()) + ":" + diagnostic.Subject() + ":" + diagnostic.Detail()
		parts = append(parts, part)
	}
	return strings.Join(parts, "|")
}
