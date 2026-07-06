package fpf

import (
	"strings"
	"testing"
)

func TestPatternRecallCompactStripsSourceBody(t *testing.T) {
	record := PatternRecallFromRetrievedCandidates(
		PatternRecallRequest{
			Query: "Use Boundary Norm Square",
			Mode:  PatternRecallCompactMode,
		},
		[]PatternUseRetrievedCandidate{patternRecallFixtureCandidate()},
	)

	if err := record.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if record.SupportLevel != PatternRecallSupportSourceCardRetrieved {
		t.Fatalf("support = %q", record.SupportLevel)
	}
	if len(record.CandidateSourceCards) != 1 {
		t.Fatalf("candidate count = %d", len(record.CandidateSourceCards))
	}
	if record.CandidateSourceCards[0].PatternID != "A.6.B" {
		t.Fatalf("pattern = %q", record.CandidateSourceCards[0].PatternID)
	}
	if record.CandidateSourceCards[0].SourceCard != nil {
		t.Fatalf("compact recall must not carry source_card: %#v", record.CandidateSourceCards[0].SourceCard)
	}
	if !strings.Contains(record.FullRecallCommand, `mode="full"`) {
		t.Fatalf("full command = %q", record.FullRecallCommand)
	}
}

func TestPatternRecallFullIncludesSourceBodyAndProvenance(t *testing.T) {
	record := PatternRecallFromRetrievedCandidates(
		PatternRecallRequest{
			Query: "Use Boundary Norm Square",
			Mode:  PatternRecallFullMode,
		},
		[]PatternUseRetrievedCandidate{patternRecallFixtureCandidate()},
	)

	if err := record.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	card := record.CandidateSourceCards[0].SourceCard
	if card == nil {
		t.Fatal("source_card missing")
	}
	if !strings.Contains(card.Body, "Full boundary norm square card body") {
		t.Fatalf("body = %q", card.Body)
	}
	if card.SourcePath != "FPF.md" {
		t.Fatalf("source path = %q", card.SourcePath)
	}
	if card.SourceCommit != "abc123" {
		t.Fatalf("source commit = %q", card.SourceCommit)
	}
	if card.LineStart != 10 || card.LineEnd != 20 {
		t.Fatalf("range = %d..%d", card.LineStart, card.LineEnd)
	}
	if card.BodyHash == "" {
		t.Fatal("body hash missing")
	}
}

func TestPatternRecallMissingForMechanicalAndRouterMetaQueries(t *testing.T) {
	cases := []string{
		"what time is it",
		"what is the term in this equation",
		"Надо ли компилировать все 250 FPF карточек в route cards?",
	}

	for _, query := range cases {
		t.Run(query, func(t *testing.T) {
			record := PatternRecallFromRetrievedCandidates(
				PatternRecallRequest{Query: query, Mode: PatternRecallCompactMode},
				[]PatternUseRetrievedCandidate{patternRecallFixtureCandidate()},
			)
			if err := record.Validate(); err != nil {
				t.Fatalf("Validate returned error: %v", err)
			}
			if record.SupportLevel != PatternRecallSupportMissing {
				t.Fatalf("support = %q", record.SupportLevel)
			}
			if len(record.CandidateSourceCards) != 0 {
				t.Fatalf("candidates = %#v", record.CandidateSourceCards)
			}
		})
	}
}

func TestPatternRecallValidationRejectsAuthorityOverclaim(t *testing.T) {
	record := PatternRecallResult{
		SchemaVersion:     PatternRecallSchemaVersion,
		RecordKind:        PatternRecallRecordKind,
		Authority:         PatternRecallAuthority,
		Mode:              PatternRecallCompactMode,
		SupportLevel:      string(PatternUseSupportImplementedSubstrate),
		AuthorityBoundary: append([]string(nil), patternRecallAuthorityBoundary...),
	}

	if err := record.Validate(); err == nil {
		t.Fatal("expected implemented_substrate overclaim to fail validation")
	}
}

func TestPatternRecallValidationRejectsInvalidSourceTier(t *testing.T) {
	record := PatternRecallFromRetrievedCandidates(
		PatternRecallRequest{
			Query: "Use Boundary Norm Square",
			Mode:  PatternRecallCompactMode,
		},
		[]PatternUseRetrievedCandidate{patternRecallFixtureCandidate()},
	)
	record.CandidateSourceCards[0].SourceTier = "semantic"

	if err := record.Validate(); err == nil {
		t.Fatal("expected invalid source_tier to fail validation")
	}
}

func patternRecallFixtureCandidate() PatternUseRetrievedCandidate {
	return PatternUseRetrievedCandidate{
		PatternRef:    "A.6.B",
		Title:         "A.6.B Boundary Norm Square",
		Summary:       "Use the boundary norm square for source/claim/use/authority alignment.",
		Snippet:       "Boundary Norm Square snippet",
		SourceTier:    SpecSearchTierFTS,
		SourceReason:  "hybrid semantic match",
		SourceRef:     "FPF.md",
		SourceKind:    "fpf_pattern_card",
		Normativity:   "normative_fpf_source",
		RetrievalMode: SpecRetrievalModeSemantic,
		SourceCard: &PatternUseSourceCard{
			BodyKind:    PatternAtlasBodyKindFullCardRange,
			SourceRef:   "FPF.md",
			FPFCommit:   "abc123",
			StartLine:   10,
			EndLine:     20,
			RootNodeID:  "node-a6b",
			ContentHash: "sha256-fixture",
			NodeCount:   3,
			Body:        "Full boundary norm square card body.",
		},
	}
}
