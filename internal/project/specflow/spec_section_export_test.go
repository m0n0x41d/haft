package specflow

import (
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/project"
)

func TestRenderSpecSectionEditionMarkdownRoundTripsThroughEmptyStore(t *testing.T) {
	sourceStore := NewSQLiteSpecSectionEditionStore(newTestBaselineDB(t).GetRawDB())
	emptyStore := NewSQLiteSpecSectionEditionStore(newTestBaselineDB(t).GetRawDB())
	sourceSection := specSectionEditionRoundTripTestSection()
	sourceEdition := NewSpecSectionEdition("proj-1", sourceSection, SpecSectionSourceSQL, time.Now().UTC())

	if err := sourceStore.PutCurrent(sourceEdition); err != nil {
		t.Fatalf("PutCurrent source: %v", err)
	}

	storedSource, err := sourceStore.GetCurrent("proj-1", sourceSection.ID)
	if err != nil {
		t.Fatalf("GetCurrent source: %v", err)
	}

	publication, err := RenderSpecSectionEditionMarkdown(storedSource)
	if err != nil {
		t.Fatalf("RenderSpecSectionEditionMarkdown: %v", err)
	}
	if publication.SourceEditionHash != storedSource.SemanticHash {
		t.Fatalf("source edition hash = %q, want %q", publication.SourceEditionHash, storedSource.SemanticHash)
	}
	if publication.AuthorityBoundary != SpecSectionPublicationProjectionAuthorityBoundary {
		t.Fatalf("authority boundary = %q", publication.AuthorityBoundary)
	}
	for _, want := range []string{"claim_truth", "global_truth"} {
		if !strings.Contains(publication.AuthorityBoundary, want) {
			t.Fatalf("authority boundary missing %q: %q", want, publication.AuthorityBoundary)
		}
	}
	if !strings.Contains(publication.Markdown, "```yaml spec-section") {
		t.Fatalf("publication markdown did not contain spec-section fence:\n%s", publication.Markdown)
	}

	document := project.SpecDocumentInput{
		Path:    publication.CarrierPath,
		Kind:    sourceSection.DocumentKind,
		Content: publication.Markdown,
	}
	report := project.CheckSpecDocuments([]project.SpecDocumentInput{document})
	if report.Summary.TotalFindings != 0 {
		t.Fatalf("publication should parse cleanly, findings = %#v", report.Findings)
	}
	sections := project.SpecSectionsFromDocuments([]project.SpecDocumentInput{document})
	if len(sections) != 1 {
		t.Fatalf("parsed sections = %d, want 1", len(sections))
	}

	importedEdition := NewSpecSectionEdition("proj-1", sections[0], SpecSectionSourceCarrierImport, time.Now().UTC())
	if err := emptyStore.PutCurrent(importedEdition); err != nil {
		t.Fatalf("PutCurrent imported: %v", err)
	}

	storedImport, err := emptyStore.GetCurrent("proj-1", sourceSection.ID)
	if err != nil {
		t.Fatalf("GetCurrent imported: %v", err)
	}
	if storedImport.SemanticHash != storedSource.SemanticHash {
		t.Fatalf("imported semantic hash = %q, want %q", storedImport.SemanticHash, storedSource.SemanticHash)
	}
	if len(storedImport.Section.Claims) != 1 {
		t.Fatalf("imported claims = %#v, want one claim", storedImport.Section.Claims)
	}
	if storedImport.Section.Claims[0].EvidenceRefs[0] != "ev-spec-round-trip" {
		t.Fatalf("claim evidence refs = %#v", storedImport.Section.Claims[0].EvidenceRefs)
	}
}

func TestRenderSpecSectionEditionMarkdownFailsClosedOnLossyProjection(t *testing.T) {
	section := specSectionEditionRoundTripTestSection()
	section.Terms = []string{"sync-back", "carrier"}
	edition := NewSpecSectionEdition("proj-1", section, SpecSectionSourceSQL, time.Now().UTC())

	_, err := RenderSpecSectionEditionMarkdown(edition)
	if err == nil {
		t.Fatal("expected semantic identity loss error")
	}
	if !strings.Contains(err.Error(), "loses semantic identity") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRenderSpecSectionEditionMarkdownSeparatesCarrierPathFromSemanticEdition(t *testing.T) {
	section := specSectionEditionRoundTripTestSection()
	section.Path = ".haft/specs/target-system.md"
	source := NewSpecSectionEdition("proj-1", section, SpecSectionSourceSQL, time.Now().UTC())

	carrierMoved := section
	carrierMoved.Path = ".haft/specs/target-system-renamed.md"
	moved := NewSpecSectionEdition("proj-1", carrierMoved, SpecSectionSourceSQL, time.Now().UTC())

	semanticChanged := section
	semanticChanged.DependsOn = append([]string{}, section.DependsOn...)
	semanticChanged.DependsOn = append(semanticChanged.DependsOn, "TS.new-semantic-parent.001")
	changed := NewSpecSectionEdition("proj-1", semanticChanged, SpecSectionSourceSQL, time.Now().UTC())

	sourcePublication, err := RenderSpecSectionEditionMarkdown(source)
	if err != nil {
		t.Fatalf("Render source publication: %v", err)
	}
	movedPublication, err := RenderSpecSectionEditionMarkdown(moved)
	if err != nil {
		t.Fatalf("Render moved publication: %v", err)
	}
	changedPublication, err := RenderSpecSectionEditionMarkdown(changed)
	if err != nil {
		t.Fatalf("Render changed publication: %v", err)
	}

	if moved.SemanticHash != source.SemanticHash {
		t.Fatalf("carrier path move changed semantic hash: got %s want %s", moved.SemanticHash, source.SemanticHash)
	}
	if movedPublication.SourceEditionHash != sourcePublication.SourceEditionHash {
		t.Fatalf("carrier path move changed source edition hash: got %s want %s", movedPublication.SourceEditionHash, sourcePublication.SourceEditionHash)
	}
	if movedPublication.PublicationHash != sourcePublication.PublicationHash {
		t.Fatalf("carrier path move changed publication hash: got %s want %s", movedPublication.PublicationHash, sourcePublication.PublicationHash)
	}
	if movedPublication.CarrierPath == sourcePublication.CarrierPath {
		t.Fatalf("carrier path was not preserved as separate metadata: %q", movedPublication.CarrierPath)
	}
	if changedPublication.SourceEditionHash == sourcePublication.SourceEditionHash {
		t.Fatalf("semantic edit did not change source edition hash")
	}
	if changedPublication.PublicationHash == sourcePublication.PublicationHash {
		t.Fatalf("semantic edit did not change publication hash")
	}
}

func TestRenderSpecSectionEditionMarkdownPreservesStringEvidenceRequirements(t *testing.T) {
	sourceStore := NewSQLiteSpecSectionEditionStore(newTestBaselineDB(t).GetRawDB())
	emptyStore := NewSQLiteSpecSectionEditionStore(newTestBaselineDB(t).GetRawDB())
	sourceSection := specSectionEditionRoundTripTestSection()
	sourceSection.EvidenceRequired = []project.SpecEvidenceRequirement{{
		Description: "Runtime evidence links to this section.",
	}}
	sourceEdition := NewSpecSectionEdition("proj-1", sourceSection, SpecSectionSourceSQL, time.Now().UTC())

	if err := sourceStore.PutCurrent(sourceEdition); err != nil {
		t.Fatalf("PutCurrent source: %v", err)
	}

	storedSource, err := sourceStore.GetCurrent("proj-1", sourceSection.ID)
	if err != nil {
		t.Fatalf("GetCurrent source: %v", err)
	}
	publication, err := RenderSpecSectionEditionMarkdown(storedSource)
	if err != nil {
		t.Fatalf("RenderSpecSectionEditionMarkdown: %v", err)
	}
	if !strings.Contains(publication.Markdown, "- Runtime evidence links to this section.") {
		t.Fatalf("publication did not preserve string evidence requirement:\n%s", publication.Markdown)
	}

	document := project.SpecDocumentInput{
		Path:    publication.CarrierPath,
		Kind:    sourceSection.DocumentKind,
		Content: publication.Markdown,
	}
	sections := project.SpecSectionsFromDocuments([]project.SpecDocumentInput{document})
	if len(sections) != 1 {
		t.Fatalf("parsed sections = %d, want 1", len(sections))
	}
	importedEdition := NewSpecSectionEdition("proj-1", sections[0], SpecSectionSourceCarrierImport, time.Now().UTC())
	if err := emptyStore.PutCurrent(importedEdition); err != nil {
		t.Fatalf("PutCurrent imported: %v", err)
	}
	storedImport, err := emptyStore.GetCurrent("proj-1", sourceSection.ID)
	if err != nil {
		t.Fatalf("GetCurrent imported: %v", err)
	}
	if storedImport.SemanticHash != storedSource.SemanticHash {
		t.Fatalf("imported semantic hash = %q, want %q", storedImport.SemanticHash, storedSource.SemanticHash)
	}
}

func specSectionEditionRoundTripTestSection() project.SpecSection {
	section := specSectionEditionTestSection("TS.roundtrip.001")
	section.Title = "Round trip identity"
	section.Terms = []string{"carrier", "sync-back"}
	section.DependsOn = []string{"TS.boundary.001"}
	section.TargetRefs = []string{"api:haft-spec-sync"}
	section.EvidenceRequired = []project.SpecEvidenceRequirement{{
		Kind:        "E2E",
		Description: "DB to markdown to empty DB preserves semantic identity.",
	}}
	section.Claims = []project.SpecClaim{{
		ID:           "claim-roundtrip",
		Class:        "L",
		Statement:    "SpecSection publication projection preserves typed claim fields.",
		Scope:        []string{"TS.roundtrip.001"},
		SupportRefs:  []string{"dec-20260618-semantic-spine-v3-first-slice-1b53439e"},
		EvidenceRefs: []string{"ev-spec-round-trip"},
		ValidUntil:   "2026-08-01",
	}}
	return section
}
