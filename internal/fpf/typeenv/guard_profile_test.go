package typeenv

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/fpf"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestGuardSourceProfilesPreserveDistinctOutcomeModels(t *testing.T) {
	cases := []struct {
		fixture     string
		coordinates []string
	}{
		{
			fixture: "c3_guard_59c4553.md",
			coordinates: []string{
				"declaration_compatibility", "candidate_classification", "scope_coverage",
				"evidence_freshness", "bridge_applicability", "action_disposition", "true_false_unknown",
			},
		},
		{
			fixture: "c3_guard_admissibility_21296c8.md",
			coordinates: []string{
				"declaration_compatibility", "candidate_admissibility",
				"candidate_and_slice_admissibility_before_judgement",
				"not_applicable_no_judgement", "admissible_true_false_unknown",
				"inadmissible_refusal_without_judgement", "classification_value_preserved_on_refusal",
				"candidate_classification", "scope_coverage", "evidence_freshness",
				"bridge_applicability", "action_disposition", "r_only_bridge_consequences",
			},
		},
	}
	for _, test := range cases {
		t.Run(test.fixture, func(t *testing.T) {
			unit := guardProfileFixture(t, test.fixture)
			outcome := ParseStructuralUnit(unit)
			parsed, ok := outcome.(GrammarParsed)
			if !ok {
				t.Fatalf("guard source parse = %#v", outcome)
			}
			declaration := findC3Contract(t, parsed.Declarations())
			if declaration.Kind() != C3KindGuardSeparationContract || declaration.Designator() != "GuardDisposition" {
				t.Fatalf("guard declaration = %#v", declaration)
			}
			if !slices.Equal(declaration.Coordinates(), test.coordinates) {
				t.Fatalf("guard coordinates = %v, want %v", declaration.Coordinates(), test.coordinates)
			}
			linked, coverage, err := linkC3SourceContract(declaration)
			if err != nil {
				t.Fatal(err)
			}
			if linked.Symbol().String() != "constraint:FPF.C3.GuardSeparation" || coverage.Posture() != typedmemory.CoverageSourceOnly {
				t.Fatalf("guard contract lost its source-only symbol: %#v / %#v", linked, coverage)
			}
			location, err := sourceLocation(unit)
			if err != nil {
				t.Fatal(err)
			}
			locations := linked.Basis().SourceLocations()
			if len(locations) != 1 || locations[0] != location || coverage.Source() != location {
				t.Fatalf("guard contract lost exact source location: %v", locations)
			}
			wantCoordinates := slices.Clone(test.coordinates)
			slices.Sort(wantCoordinates)
			gotCoordinates := []string{}
			for _, field := range linked.Body().Fields() {
				if field.Name() != "coordinates" {
					continue
				}
				set := field.Value().(SetValue)
				for _, value := range set.Values() {
					text := value.(TextValue)
					gotCoordinates = append(gotCoordinates, text.Value())
				}
			}
			slices.Sort(gotCoordinates)
			if !slices.Equal(gotCoordinates, wantCoordinates) {
				t.Fatalf("linked coordinates = %v, want %v", gotCoordinates, wantCoordinates)
			}
		})
	}
}

func TestGuardAdmissibilityProfileRejectsMissingAndContradictoryClauses(t *testing.T) {
	unit := guardProfileFixture(t, "c3_guard_admissibility_21296c8.md")
	cases := []struct {
		name string
		from string
		to   string
	}{
		{"missing admissibility order", "Check candidate and slice admissibility under the pinned declarations before any C.3.2 or C.3.4 judgment below.", ""},
		{"missing no judgement", "no judgment is formed.", ""},
		{"inadmissible is unknown", "An inadmissible request gives `not-applicable`; no judgment is formed.", "An inadmissible request gives `unknown`."},
		{"inadmissible is false", "An inadmissible request gives `not-applicable`; no judgment is formed.", "An inadmissible request gives `false`."},
		{"judgement formed", "no judgment is formed.", "a judgment is formed."},
		{"unconditional classification", "For admissible inputs,", "For all inputs,"},
		{"missing separate refusal", "An inadmissible request causes refusal without a classification judgment.", ""},
		{"refusal rewrites judgement", "the guard MUST preserve which classification value it consumed.", "the guard MUST replace the classification value with `false`."},
		{"appended contradictory value", "All guards obey these invariants.", "All guards obey these invariants. Inadmissible requests are classified as `false`."},
		{"old cue cannot rescue current body", "Admissibility, then three classification values.", "Three classification values."},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if !strings.Contains(unit.Body, test.from) {
				t.Fatalf("mutation target absent: %q", test.from)
			}
			mutated := unit
			mutated.Body = strings.Replace(unit.Body, test.from, test.to, 1)
			outcome := ParseStructuralUnit(mutated)
			malformed, ok := outcome.(GrammarMalformed)
			if !ok || malformed.Diagnostics()[0].Code() != "current_c3_contract_malformed" {
				t.Fatalf("contradictory guard parsed: %#v", outcome)
			}
		})
	}
}

func TestOlderGuardProfileCannotAbsorbAdmissibilityClauses(t *testing.T) {
	unit := guardProfileFixture(t, "c3_guard_59c4553.md")
	unit.Body += "\nAn inadmissible request gives `not-applicable`; no judgment is formed."
	outcome := ParseStructuralUnit(unit)
	if _, ok := outcome.(GrammarMalformed); !ok {
		t.Fatalf("mixed old/new guard parsed: %#v", outcome)
	}
}

func guardProfileFixture(t *testing.T, name string) fpf.SourceUnit {
	t.Helper()
	path := filepath.Join("testdata", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	body := string(data)
	body = strings.TrimSpace(body)
	digest := sha256.Sum256([]byte(body))
	return fpf.SourceUnit{
		UnitID:          "fixture:guard:" + name,
		SourceID:        "C.3.A:3",
		Role:            fpf.SourceUnitRolePatternSection,
		PatternID:       "C.3.A:3",
		ParentPatternID: "C.3.A",
		Body:            body,
		Provenance: fpf.SourceProvenance{
			SourcePath:     path,
			SourceRevision: "fixture-excerpt:" + name,
			ContentHash:    fmt.Sprintf("%x", digest),
			StartLine:      1,
			EndLine:        strings.Count(body, "\n") + 1,
		},
	}
}
