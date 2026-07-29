package fpf

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestSourceProbePhrases_RemoveOnlyQueryScaffoldAndPreserveOrder(t *testing.T) {
	phrases := derivedSourcePhrases(CandidateProbe{
		Text: "What is the target system here?",
	})
	if len(phrases) != 2 ||
		phrases[0].Kind != SourcePhraseKindExactProbeSpan ||
		phrases[1].ProbeField != "text" ||
		phrases[1].Value != "target system" ||
		phrases[1].Kind != SourcePhraseKindScaffoldCompressed {
		t.Fatalf("derived source phrases = %#v, want exact span plus ordered scaffold-compressed target system", phrases)
	}

	exact := sourceUnitExactGroundingValues(SourceUnit{
		UnitID:   "readme:practical_use_card:system-in-context",
		SourceID: "SYSTEM-IN-CONTEXT",
		Role:     SourceUnitRolePracticalUseCard,
		Title:    "Make the current system question explicit",
	})
	if _, splitSourceID := exact["system"]; splitSourceID {
		t.Fatal("single SourceID component system must not become exact identity evidence")
	}
}

func TestExactSourceAnchorsPreserveProbeFieldAndStructuredTokens(t *testing.T) {
	anchors := exactSourceAnchors(CandidateProbe{
		Text:         "Как составить план?",
		KnownContext: []string{"U.WorkPlan/A.15.2", "A.22.CGUS"},
	})

	want := map[string]bool{
		"known_context\x00U.WorkPlan": false,
		"known_context\x00A.15.2":     false,
		"known_context\x00A.22.CGUS":  false,
	}
	for _, anchor := range anchors {
		key := anchor.ProbeField + "\x00" + anchor.Value
		if _, expected := want[key]; expected {
			want[key] = true
		}
	}
	for key, found := range want {
		if !found {
			t.Fatalf("exact source anchors omit %q: %#v", key, anchors)
		}
	}
}

func TestSQLiteQueryIndex_DerivedPhraseDoesNotCrossSourceFieldBoundary(t *testing.T) {
	units := minimalValidSourceUnits()
	for index := range units {
		if units[index].Role != SourceUnitRolePatternBody {
			continue
		}
		units[index].Title = "target"
		units[index].Body = "system"
		units[index].Provenance.ContentHash = sourceContentHash(units[index].Body)
	}

	dbPath := filepath.Join(t.TempDir(), "fpf.db")
	if err := StoreSourceUnits(dbPath, units); err != nil {
		t.Fatalf("store source units: %v", err)
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open source query db: %v", err)
	}
	defer func() { _ = db.Close() }()

	batch, err := NewSQLiteQueryIndex(db).SearchSourceProbePhrases(
		[]SourceProbePhrase{{ProbeField: "text", Value: "target system", Kind: SourcePhraseKindExactProbeSpan}},
		[]SourceUnitRole{SourceUnitRolePatternBody},
	)
	if err != nil {
		t.Fatalf("search derived source phrase: %v", err)
	}
	if len(batch.Candidates) != 0 {
		t.Fatalf("phrase crossed title/body boundary: %#v", batch.Candidates)
	}
}
