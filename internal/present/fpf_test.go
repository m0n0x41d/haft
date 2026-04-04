package present_test

import (
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/present"
)

func TestFormatFPFSearch_NumberedWithHeader(t *testing.T) {
	results := []present.FPFSearchResult{
		{
			PatternID: "A.6",
			Heading:   "Signature Stack & Boundary Discipline",
			Tier:      "pattern",
			Reason:    "exact pattern id",
			Content:   "Boundary routing body",
		},
		{
			PatternID: "A.6.B",
			Heading:   "A.6.B — Boundary Norm Square",
			Tier:      "route",
			Reason:    "Boundary discipline and routing",
			Content:   "Norm square body",
		},
	}

	output := present.FormatFPFSearch(results, present.FPFSearchOptions{
		Header:       "## FPF Spec: boundary (2 results)",
		Enumerate:    true,
		ShowMetadata: true,
	})

	checks := []string{
		"## FPF Spec: boundary (2 results)",
		"### 1. A.6 — Signature Stack & Boundary Discipline",
		"tier: pattern · exact pattern id",
		"Boundary routing body",
		"### 2. A.6.B — Boundary Norm Square",
		"tier: route · Boundary discipline and routing",
	}

	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Fatalf("expected output to contain %q, got:\n%s", check, output)
		}
	}

	if strings.Count(output, "A.6.B — Boundary Norm Square") != 1 {
		t.Fatalf("expected pre-prefixed heading to stay deduplicated, got:\n%s", output)
	}
}

func TestFormatFPFSearch_EmptyMessage(t *testing.T) {
	output := present.FormatFPFSearch(nil, present.FPFSearchOptions{
		EmptyMessage: "No FPF spec matches for: A.6",
	})

	if output != "No FPF spec matches for: A.6\n" {
		t.Fatalf("unexpected empty output %q", output)
	}
}

func TestFormatFPFSearch_HidesMetadataByDefault(t *testing.T) {
	results := []present.FPFSearchResult{
		{
			PatternID: "A.6",
			Heading:   "Signature Stack & Boundary Discipline",
			Tier:      "pattern",
			Reason:    "exact pattern id",
			Content:   "Boundary routing body",
		},
	}

	output := present.FormatFPFSearch(results, present.FPFSearchOptions{
		Enumerate: true,
	})

	if strings.Contains(output, "tier:") {
		t.Fatalf("expected default formatting to hide metadata, got:\n%s", output)
	}
	if !strings.Contains(output, "### 1. A.6 — Signature Stack & Boundary Discipline") {
		t.Fatalf("expected heading to render, got:\n%s", output)
	}
}

func TestFormatFPFSection(t *testing.T) {
	output := present.FormatFPFSection("A.6", "Section body\n")
	want := "## A.6\n\nSection body\n"

	if output != want {
		t.Fatalf("unexpected section output:\nwant:\n%s\ngot:\n%s", want, output)
	}
}

func TestFormatFPFInfo(t *testing.T) {
	output := present.FormatFPFInfo(present.FPFInfo{
		Version:         "dev",
		Commit:          "abc1234",
		Source:          "https://github.com/ailev/FPF/commit/abc1234",
		IndexedSections: "321",
		BuildTime:       "2026-03-26T12:34:56Z",
		SpecPath:        "data/FPF/FPF-Spec.md",
		SchemaVersion:   "1",
	})

	checks := []string{
		"haft fpf version: dev",
		"FPF index schema version: 1",
		"FPF upstream commit: abc1234",
		"FPF source: https://github.com/ailev/FPF/commit/abc1234",
		"Indexed sections: 321",
		"Build time: 2026-03-26T12:34:56Z",
		"Spec path: data/FPF/FPF-Spec.md",
	}

	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Fatalf("expected output to contain %q, got:\n%s", check, output)
		}
	}
}
