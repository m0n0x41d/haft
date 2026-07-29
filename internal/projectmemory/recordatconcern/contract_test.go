package recordatconcern

import (
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/projectmemory/recordcarrier"
	"github.com/m0n0x41d/haft/internal/recordmapping"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestClosedContractsAcceptOnlyTheirExactMappingCoordinates(t *testing.T) {
	digest := contractTestDigest(t)
	noteManifest := contractTestManifest(
		t,
		"haft.note-at-concern",
		"2.0.0",
		digest,
	)
	noteAdapter := contractTestAdapter(t, "haft-note-adapter/2.0.0")
	note, err := NewNoteContract(noteManifest, noteAdapter)
	if err != nil {
		t.Fatalf("NewNoteContract() error = %v", err)
	}
	if !note.valid() || note.SignatureID() != "Haft.NoteAtConcern" {
		t.Fatal("exact Note contract did not seal its source-owned signature")
	}
	legacyNoteManifest := contractTestManifest(
		t,
		"haft.note-at-concern",
		"1.0.0",
		digest,
	)
	if _, err := NewNoteContract(legacyNoteManifest, noteAdapter); err == nil {
		t.Fatal("Note contract silently treated the legacy unqualified mapping as version 2")
	}
	legacyNoteAdapter := contractTestAdapter(t, "haft-note-adapter/1.0.0")
	if _, err := NewNoteContract(noteManifest, legacyNoteAdapter); err == nil {
		t.Fatal("Note contract silently treated the legacy unqualified adapter as version 2")
	}

	problemManifest := contractTestManifest(
		t,
		"haft.problem-card-at-concern",
		"2.0.0",
		digest,
	)
	problemAdapter := contractTestAdapter(
		t,
		"haft-problem-card-adapter/2.0.0",
	)
	problem, err := NewProblemCardContract(problemManifest, problemAdapter)
	if err != nil {
		t.Fatalf("NewProblemCardContract() error = %v", err)
	}
	if !problem.valid() ||
		problem.SignatureID() != "Haft.ProblemCardAtConcern" {
		t.Fatal("exact ProblemCard contract did not seal its source-owned signature")
	}

	if _, err := NewProblemCardContract(noteManifest, problemAdapter); err == nil {
		t.Fatal("ProblemCard contract accepted the Note mapping manifest")
	}
	if _, err := NewProblemCardContract(problemManifest, noteAdapter); err == nil {
		t.Fatal("ProblemCard contract accepted the Note adapter version")
	}

	portfolioManifest := contractTestManifest(
		t,
		"haft.solution-portfolio-at-concern",
		"2.0.0",
		digest,
	)
	portfolioAdapter := contractTestAdapter(
		t,
		"haft-solution-portfolio-adapter/2.0.0",
	)
	portfolio, err := NewSolutionPortfolioContract(
		portfolioManifest,
		portfolioAdapter,
	)
	if err != nil {
		t.Fatalf("NewSolutionPortfolioContract() error = %v", err)
	}
	if !portfolio.valid() ||
		portfolio.SignatureID() != "Haft.SolutionPortfolioAtConcern" {
		t.Fatal("exact SolutionPortfolio contract did not seal its source-owned signature")
	}

	comparisonManifest := contractTestManifest(
		t,
		"haft.portfolio-comparison",
		"2.0.0",
		digest,
	)
	comparisonAdapter := contractTestAdapter(
		t,
		"haft-portfolio-comparison-adapter/2.0.0",
	)
	comparison, err := NewPortfolioComparisonContract(
		comparisonManifest,
		comparisonAdapter,
	)
	if err != nil {
		t.Fatalf("NewPortfolioComparisonContract() error = %v", err)
	}
	if !comparison.valid() ||
		comparison.SignatureID() != "Haft.PortfolioComparison" {
		t.Fatal("exact PortfolioComparison contract did not seal its source-owned signature")
	}
	if _, err := NewPortfolioComparisonContract(
		portfolioManifest,
		comparisonAdapter,
	); err == nil {
		t.Fatal("PortfolioComparison contract accepted the SolutionPortfolio manifest")
	}

	decisionManifest := contractTestManifest(
		t,
		"haft.decision-choice-at-concern",
		"2.0.0",
		digest,
	)
	decisionAdapter := contractTestAdapter(
		t,
		"haft-decision-record-adapter/2.0.0",
	)
	decision, err := NewDecisionRecordContract(
		decisionManifest,
		decisionAdapter,
	)
	if err != nil {
		t.Fatalf("NewDecisionRecordContract() error = %v", err)
	}
	if !decision.valid() ||
		decision.SignatureID() != "Haft.DecisionChoiceAtConcern" ||
		decision.definition.recordKindID != decisionRecordKindID ||
		decision.definition.recordRefID != decisionRecordRefID ||
		decision.definition.recordVariant !=
			(recordcarrier.DecisionRecordVariantV1{}).Token() {
		t.Fatal("exact DecisionRecord contract did not seal its specialized record mapping")
	}
	if _, err := NewDecisionRecordContract(
		comparisonManifest,
		decisionAdapter,
	); err == nil {
		t.Fatal("DecisionRecord contract accepted the PortfolioComparison manifest")
	}

	specManifest := contractTestManifest(
		t,
		"haft.spec-section-at-concern",
		"2.0.0",
		digest,
	)
	specAdapter := contractTestAdapter(
		t,
		"haft-spec-section-adapter/2.0.0",
	)
	spec, err := NewSpecSectionContract(specManifest, specAdapter)
	if err != nil {
		t.Fatalf("NewSpecSectionContract() error = %v", err)
	}
	if !spec.valid() ||
		spec.SignatureID() != "Haft.SpecSectionAtConcern" ||
		spec.definition.recordKindID != specSectionRecordKindID ||
		spec.definition.recordRefID != specSectionRecordRefID ||
		spec.definition.recordVariant !=
			(recordcarrier.SpecSectionRecordVariantV1{}).Token() {
		t.Fatal("exact SpecSection contract did not seal its specialized record mapping")
	}
	if _, err := NewSpecSectionContract(problemManifest, specAdapter); err == nil {
		t.Fatal("SpecSection contract accepted the ProblemCard mapping manifest")
	}
}

func TestZeroContractIsNotAUsableMappingShape(t *testing.T) {
	var zero Contract
	if zero.valid() {
		t.Fatal("zero record-at-concern contract became usable")
	}
	if zero.SignatureID() != "" {
		t.Fatal("zero record-at-concern contract exposed a signature")
	}
}

func contractTestManifest(
	t *testing.T,
	id string,
	version string,
	digest typedmemory.SHA256Digest,
) recordmapping.MappingManifestRef {
	t.Helper()
	value, err := recordmapping.NewMappingManifestRef(id, version, digest)
	if err != nil {
		t.Fatalf("NewMappingManifestRef() error = %v", err)
	}
	return value
}

func contractTestAdapter(
	t *testing.T,
	raw string,
) recordmapping.AdapterVersion {
	t.Helper()
	value, err := recordmapping.NewAdapterVersion(raw)
	if err != nil {
		t.Fatalf("NewAdapterVersion() error = %v", err)
	}
	return value
}

func contractTestDigest(t *testing.T) typedmemory.SHA256Digest {
	t.Helper()
	value, err := typedmemory.NewSHA256Digest(
		"sha256:" + strings.Repeat("a", 64),
	)
	if err != nil {
		t.Fatalf("NewSHA256Digest() error = %v", err)
	}
	return value
}
