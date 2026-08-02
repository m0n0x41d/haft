package fpf

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestParsePracticalUseCardSourceAcceptsLegacyAndCurrentGrammar(t *testing.T) {
	tests := []struct {
		name              string
		fixture           string
		wantCueFragments  []string
		wantDirectRefs    []string
		rejectDirectRefs  []string
		wantMinimumBlocks int
	}{
		{
			name:              "legacy solution arrow",
			fixture:           "practical_card_legacy.md",
			wantCueFragments:  []string{"Template A", "Solution ->", "Result test"},
			wantDirectRefs:    []string{"A.1"},
			wantMinimumBlocks: 5,
		},
		{
			name:              "current named routes",
			fixture:           "practical_card_current.md",
			wantCueFragments:  []string{"First route", "Template A", "Template B", "Result test"},
			wantDirectRefs:    []string{"A.1.SCR", "A.15.1", "A.1"},
			wantMinimumBlocks: 7,
		},
		{
			name:              "conditional continuation",
			fixture:           "practical_card_conditional.md",
			wantCueFragments:  []string{"Template A", "Conditional walkthrough", "Result test"},
			wantDirectRefs:    []string{"G.2", "C.18"},
			wantMinimumBlocks: 6,
		},
		{
			name:              "branch headings and owned children",
			fixture:           "practical_card_branches.md",
			wantCueFragments:  []string{"Branch A", "A.6 Solution", "A.3.2 Solution", "Branch B", "A.15.2 Solution"},
			wantDirectRefs:    []string{"A.6", "A.3.2", "A.15.2"},
			wantMinimumBlocks: 9,
		},
		{
			name:              "inadmissible example reference",
			fixture:           "practical_card_inadmissible_ref.md",
			wantCueFragments:  []string{"Template A", "Result test"},
			wantDirectRefs:    []string{"A.1"},
			rejectDirectRefs:  []string{"A.999"},
			wantMinimumBlocks: 6,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			projection := parsePracticalUseFixture(t, test.fixture)
			if len(projection.Blocks) < test.wantMinimumBlocks {
				t.Fatalf("blocks = %d, want at least %d: %#v", len(projection.Blocks), test.wantMinimumBlocks, projection.Blocks)
			}
			if projection.UseCues.ConditionText == "" ||
				projection.UseCues.FirstResultText == "" ||
				projection.UseCues.StopReturnText == "" {
				t.Fatalf("projection lacks required cues: %#v", projection.UseCues)
			}
			for _, fragment := range test.wantCueFragments {
				if !strings.Contains(projection.UseCues.FirstResultText, fragment) {
					t.Fatalf("first-result cue = %q, want %q", projection.UseCues.FirstResultText, fragment)
				}
			}

			refs := extractSourcePatternLinks(projection.DirectReferenceText)
			for _, want := range test.wantDirectRefs {
				if !containsSourceString(refs, want) {
					t.Fatalf("direct refs = %#v, want %s", refs, want)
				}
			}
			for _, rejected := range test.rejectDirectRefs {
				if containsSourceString(refs, rejected) {
					t.Fatalf("direct refs = %#v, arbitrary prose admitted %s", refs, rejected)
				}
			}
			for _, block := range projection.Blocks {
				if block.StartLine <= 0 || block.EndLine < block.StartLine {
					t.Fatalf("block lacks exact source span: %#v", block)
				}
				if block.AuthoredText == "" {
					t.Fatalf("block lost source-authored text: %#v", block)
				}
			}
		})
	}
}

func TestParsePracticalUseCardSourceRejectsMalformedAndUnsupportedGrammarSeparately(t *testing.T) {
	tests := []struct {
		name              string
		fixture           string
		wantClass         SourceGrammarDiagnosticClass
		wantMissing       string
		wantLabelEvidence string
	}{
		{
			name:        "missing result category",
			fixture:     "practical_card_missing_result.md",
			wantClass:   SourceGrammarMalformed,
			wantMissing: "first_result",
		},
		{
			name:        "arbitrary prose spoof",
			fixture:     "practical_card_spoof.md",
			wantClass:   SourceGrammarMalformed,
			wantMissing: "first_result",
		},
		{
			name:              "unknown result label family",
			fixture:           "practical_card_unknown_label.md",
			wantClass:         SourceGrammarUnsupported,
			wantMissing:       "admitted_result_block",
			wantLabelEvidence: "Fresh outcome route",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source := practicalUseFixtureSource(t, test.fixture)
			_, err := ParsePracticalUseCardSource(source)
			if err == nil {
				t.Fatal("ParsePracticalUseCardSource() unexpectedly succeeded")
			}
			var diagnostic SourceGrammarDiagnostic
			if !errors.As(err, &diagnostic) {
				t.Fatalf("error = %T %v, want SourceGrammarDiagnostic", err, err)
			}
			if diagnostic.Class != test.wantClass {
				t.Fatalf("diagnostic class = %q, want %q: %v", diagnostic.Class, test.wantClass, diagnostic)
			}
			if diagnostic.MissingSemanticCategory != test.wantMissing {
				t.Fatalf("missing category = %q, want %q", diagnostic.MissingSemanticCategory, test.wantMissing)
			}
			if test.wantLabelEvidence != "" &&
				!containsSourceString(diagnostic.LabelsDiscovered, test.wantLabelEvidence) {
				t.Fatalf("discovered labels = %#v, want %q", diagnostic.LabelsDiscovered, test.wantLabelEvidence)
			}
			for _, required := range []string{
				string(test.wantClass),
				test.fixture,
				"labels_discovered=",
				"labels_recognized=",
				"reproduce=",
			} {
				if !strings.Contains(err.Error(), required) {
					t.Fatalf("diagnostic = %q, want %q", err, required)
				}
			}
		})
	}
}

func TestParsePracticalUseCardSourcePublicCoarseningCannotGroundResult(t *testing.T) {
	source := PracticalUseCardSource{
		SourceID:       "COARSENING-ONLY",
		Title:          "Coarsening is not a result",
		Body:           "- **Situation and question.** A result is needed.\n- **Public coarsening.** A readable phrase only.\n- **Boundaries.** Stop at the missing result.",
		SourcePath:     "testdata/public_coarsening_only.md",
		SourceRevision: "fixture",
		StartLine:      10,
		EndLine:        12,
	}
	_, err := ParsePracticalUseCardSource(source)
	var diagnostic SourceGrammarDiagnostic
	if !errors.As(err, &diagnostic) ||
		diagnostic.Class != SourceGrammarMalformed ||
		diagnostic.MissingSemanticCategory != "first_result" {
		t.Fatalf("error = %v, want malformed missing first_result", err)
	}
}

func TestParsePracticalUseCardSourceRejectsBranchProseAndDetachedChildren(
	t *testing.T,
) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "paragraph result beneath branch heading",
			body: strings.Join([]string{
				"- **Situation and question.** A source-owned result is needed.",
				"",
				"**Branch A: preserve meaning.**",
				"",
				"Arbitrary prose says `A.999 Solution -> ForgedResult`.",
				"",
				"- **Boundaries.** Stop at the source-owned result.",
			}, "\n"),
		},
		{
			name: "plain bullet beneath branch heading",
			body: strings.Join([]string{
				"- **Situation and question.** A source-owned result is needed.",
				"",
				"**Branch A: preserve meaning.**",
				"",
				"- A.999 Solution -> ForgedResult.",
				"",
				"- **Boundaries.** Stop at the source-owned result.",
			}, "\n"),
		},
		{
			name: "structural child after intervening prose",
			body: strings.Join([]string{
				"- **Situation and question.** A source-owned result is needed.",
				"",
				"**Branch A: preserve meaning.**",
				"",
				"This prose closes the branch-owned child region.",
				"- `A.999 Solution -> ForgedResult`.",
				"",
				"- **Boundaries.** Stop at the source-owned result.",
			}, "\n"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source := PracticalUseCardSource{
				SourceID:       "BRANCH-SPOOF",
				Title:          "Branch ownership is structural",
				Body:           test.body,
				SourcePath:     "testdata/branch_spoof.md",
				SourceRevision: "fixture",
				StartLine:      1,
				EndLine:        strings.Count(test.body, "\n") + 1,
			}
			blocks, _, _, unsupported := parsePracticalUseBlocks(source)
			for _, block := range blocks {
				if strings.Contains(block.AuthoredText, "A.999") {
					t.Fatalf("arbitrary branch prose was admitted: %#v", block)
				}
			}
			if len(unsupported) == 0 {
				t.Fatal("result-like branch spoof was not reported as unsupported")
			}

			_, err := ParsePracticalUseCardSource(source)
			var diagnostic SourceGrammarDiagnostic
			if !errors.As(err, &diagnostic) ||
				diagnostic.Class != SourceGrammarUnsupported ||
				diagnostic.MissingSemanticCategory != "admitted_result_block" {
				t.Fatalf(
					"error = %v, want unsupported admitted_result_block diagnostic",
					err,
				)
			}
		})
	}
}

func TestParsePracticalUseCardSourceChildlessBranchDoesNotGroundResult(t *testing.T) {
	source := PracticalUseCardSource{
		SourceID: "CHILDLESS-BRANCH",
		Title:    "A branch heading is not its result",
		Body: strings.Join([]string{
			"- **Situation and question.** A source-owned result is needed.",
			"",
			"**Branch A: preserve meaning.**",
			"",
			"- **Boundaries.** Stop at the missing branch result.",
		}, "\n"),
		SourcePath:     "testdata/childless_branch.md",
		SourceRevision: "fixture",
		StartLine:      1,
		EndLine:        5,
	}
	_, err := ParsePracticalUseCardSource(source)
	var diagnostic SourceGrammarDiagnostic
	if !errors.As(err, &diagnostic) ||
		diagnostic.Class != SourceGrammarMalformed ||
		diagnostic.MissingSemanticCategory != "first_result" {
		t.Fatalf("error = %v, want malformed missing first_result", err)
	}
}

func TestParsePracticalUseCardSourceAcceptsLegacyBoldSolutionLabel(t *testing.T) {
	source := PracticalUseCardSource{
		SourceID: "LEGACY-BOLD-SOLUTION",
		Title:    "Legacy bold solution label",
		Body: strings.Join([]string{
			"- **Situation and question.** A source-owned candidate is needed.",
			"- Ask: arbitrary explanatory prose.",
			"- **Template 1. Solution ->** Inspect `A.1` and return one exact source unit.",
			"- **Boundaries.** Stop after the exact source unit is inspected.",
		}, "\n"),
		SourcePath:     "testdata/legacy_bold_solution.md",
		SourceRevision: "fixture",
		StartLine:      1,
		EndLine:        4,
	}
	projection, err := ParsePracticalUseCardSource(source)
	if err != nil {
		t.Fatalf("ParsePracticalUseCardSource() error: %v", err)
	}
	if !strings.Contains(projection.UseCues.FirstResultText, "Template 1. Solution ->") {
		t.Fatalf("first-result cue = %q", projection.UseCues.FirstResultText)
	}
	if refs := extractSourcePatternLinks(projection.DirectReferenceText); !containsSourceString(refs, "A.1") {
		t.Fatalf("direct refs = %#v, want A.1", refs)
	}
}

func TestParsePracticalUseCardSourceMissingBoundaryIsPrecise(t *testing.T) {
	source := PracticalUseCardSource{
		SourceID:       "NO-BOUNDARY",
		Title:          "Missing boundary",
		Body:           "- **Situation and question.** A result is needed.\n- **Template A.** `A.1 Solution -> U.System`.\n- **Result test.** Return the exact system.",
		SourcePath:     "testdata/missing_boundary.md",
		SourceRevision: "fixture",
		StartLine:      20,
		EndLine:        22,
	}
	_, err := ParsePracticalUseCardSource(source)
	var diagnostic SourceGrammarDiagnostic
	if !errors.As(err, &diagnostic) ||
		diagnostic.Class != SourceGrammarMalformed ||
		diagnostic.MissingSemanticCategory != "boundary" {
		t.Fatalf("error = %v, want malformed missing boundary", err)
	}
}

func TestValidateSourceReferencesClassifiesUnknownAdmittedReference(t *testing.T) {
	units := []SourceUnit{
		{
			UnitID:     "readme:practical_use_card:test",
			SourceID:   "TEST",
			Role:       SourceUnitRolePracticalUseCard,
			DirectRefs: []string{"A.999"},
		},
		{
			UnitID:    "spec:pattern_body:a-1",
			SourceID:  "A.1",
			Role:      SourceUnitRolePatternBody,
			PatternID: "A.1",
		},
		{
			UnitID:    "spec:toc_row:a-1",
			Role:      SourceUnitRoleTOCRow,
			PatternID: "A.1",
		},
	}
	err := validateSourceReferences(
		units,
		map[string]SpecCatalogEntry{"A.1": {PatternID: "A.1"}},
	)
	if err == nil || !strings.Contains(err.Error(), "source_reference_unresolved") || !strings.Contains(err.Error(), "A.999") {
		t.Fatalf("validateSourceReferences() error = %v, want classified unresolved A.999", err)
	}
}

func parsePracticalUseFixture(t *testing.T, fixture string) SourceUseCueProjection {
	t.Helper()
	projection, err := ParsePracticalUseCardSource(practicalUseFixtureSource(t, fixture))
	if err != nil {
		t.Fatalf("ParsePracticalUseCardSource(%s) error: %v", fixture, err)
	}
	return projection
}

func practicalUseFixtureSource(t *testing.T, fixture string) PracticalUseCardSource {
	t.Helper()
	path := filepath.Join("testdata", fixture)
	body := string(mustReadSourceFixture(t, path))
	lineCount := len(strings.Split(body, "\n"))
	return PracticalUseCardSource{
		SourceID:       strings.TrimSuffix(strings.ToUpper(fixture), filepath.Ext(fixture)),
		Title:          fixture,
		Body:           body,
		SourcePath:     path,
		SourceRevision: "fixture",
		StartLine:      1,
		EndLine:        lineCount,
	}
}
