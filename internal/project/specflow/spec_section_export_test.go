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
	if publication.AuthorityBoundary != "publication_projection_only_not_approval_rebaseline_evidence_or_gate" {
		t.Fatalf("authority boundary = %q", publication.AuthorityBoundary)
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
