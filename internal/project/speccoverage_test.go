package project

import (
	"strings"
	"testing"
	"time"
)

func TestDeriveSpecCoverage_DerivesStateFromEdgesNotManualWords(t *testing.T) {
	now := time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)
	section := coverageTestSection("TS.checkout.001")

	uncovered := DeriveSpecCoverage(SpecCoverageInput{
		Sections: []SpecSection{section},
		Evidence: []SpecCoverageEvidence{{
			ID:      "evid-manual-word",
			Verdict: "supports",
		}},
		Now: now,
	})
	assertCoverageState(t, uncovered, SpecCoverageUncovered)

	reasoned := DeriveSpecCoverage(SpecCoverageInput{
		Sections: []SpecSection{section},
		Decisions: []SpecCoverageDecision{{
			ID:          "dec-linked",
			Status:      "active",
			Title:       "Manual title says verified",
			SectionRefs: []string{section.ID},
		}},
		Now: now,
	})
	assertCoverageState(t, reasoned, SpecCoverageReasoned)

	commissioned := DeriveSpecCoverage(SpecCoverageInput{
		Sections: []SpecSection{section},
		Decisions: []SpecCoverageDecision{{
			ID:          "dec-linked",
			Status:      "active",
			SectionRefs: []string{section.ID},
		}},
		Commissions: []SpecCoverageCommission{{
			ID:          "wc-linked",
			DecisionRef: "dec-linked",
			State:       "queued",
			Status:      "active",
		}},
		Now: now,
	})
	assertCoverageState(t, commissioned, SpecCoverageCommissioned)

	implemented := DeriveSpecCoverage(SpecCoverageInput{
		Sections: []SpecSection{section},
		Decisions: []SpecCoverageDecision{{
			ID:            "dec-linked",
			Status:        "active",
			SectionRefs:   []string{section.ID},
			AffectedFiles: []string{"internal/checkout/flow.go"},
		}},
		Evidence: []SpecCoverageEvidence{{
			ID:          "evid-weak",
			ArtifactRef: "dec-linked",
			Type:        "measurement",
			Verdict:     "weakens",
		}},
		Now: now,
	})
	assertCoverageState(t, implemented, SpecCoverageImplemented)

	verified := DeriveSpecCoverage(SpecCoverageInput{
		Sections: []SpecSection{section},
		Decisions: []SpecCoverageDecision{{
			ID:            "dec-linked",
			Status:        "active",
			SectionRefs:   []string{section.ID},
			AffectedFiles: []string{"internal/checkout/flow.go"},
		}},
		Evidence: []SpecCoverageEvidence{{
			ID:          "evid-supports",
			ArtifactRef: "dec-linked",
			Type:        "measurement",
			Verdict:     "supports",
		}},
		Now: now,
	})
	assertCoverageState(t, verified, SpecCoverageVerified)
}

func TestDeriveSpecCoverage_InheritsDecisionCoverageThroughProblemRefs(t *testing.T) {
	now := time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)
	section := coverageTestSection("TS.onboarding.001")

	report := DeriveSpecCoverage(SpecCoverageInput{
		Sections: []SpecSection{section},
		Problems: []SpecCoverageProblem{{
			ID:          "prob-section",
			Status:      "active",
			SectionRefs: []string{section.ID},
		}},
		Decisions: []SpecCoverageDecision{{
			ID:          "dec-through-problem",
			Status:      "active",
			ProblemRefs: []string{"prob-section"},
		}},
		Now: now,
	})

	assertCoverageState(t, report, SpecCoverageReasoned)
	if !coverageTestHasEdgeTarget(report.Sections[0].Edges, "prob-section") {
		t.Fatalf("edges = %#v, want prob-section edge", report.Sections[0].Edges)
	}
}

func TestDeriveSpecCoverage_StaleOverridesVerifiedEvidence(t *testing.T) {
	now := time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)
	section := coverageTestSection("TS.freshness.001")
	section.ValidUntil = "2026-01-01"

	report := DeriveSpecCoverage(SpecCoverageInput{
		Sections: []SpecSection{section},
		Decisions: []SpecCoverageDecision{{
			ID:          "dec-linked",
			Status:      "active",
			SectionRefs: []string{section.ID},
		}},
		Evidence: []SpecCoverageEvidence{{
			ID:          "evid-supports",
			ArtifactRef: "dec-linked",
			Verdict:     "supports",
		}},
		Now: now,
	})

	assertCoverageState(t, report, SpecCoverageStale)
	if !strings.Contains(strings.Join(report.Sections[0].Why, ","), "spec section valid_until has expired") {
		t.Fatalf("why = %#v, want section expiry", report.Sections[0].Why)
	}
}

func TestDeriveSpecCoverage_ReportsUnsupportedRuntimeRunEdgeAsGap(t *testing.T) {
	report := DeriveSpecCoverage(SpecCoverageInput{
		Sections: []SpecSection{coverageTestSection("TS.runtime.001")},
		Now:      time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC),
	})

	if len(report.Gaps) != 1 {
		t.Fatalf("global gaps = %#v, want one unsupported edge gap", report.Gaps)
	}
	if got := report.Gaps[0].Kind; got != "unsupported_edge" {
		t.Fatalf("gap kind = %q, want unsupported_edge", got)
	}
}

func coverageTestSection(id string) SpecSection {
	return SpecSection{
		ID:            id,
		Kind:          "environment-change",
		Title:         "Coverage fixture",
		StatementType: "definition",
		ClaimLayer:    "object",
		Status:        "active",
		DocumentKind:  "target-system",
		Path:          ".haft/specs/target-system.md",
	}
}

func assertCoverageState(t *testing.T, report SpecCoverageReport, want SpecCoverageState) {
	t.Helper()

	if len(report.Sections) != 1 {
		t.Fatalf("sections = %#v, want one section", report.Sections)
	}
	if got := report.Sections[0].State; got != want {
		t.Fatalf("state = %q, want %q; section = %#v", got, want, report.Sections[0])
	}
}

func coverageTestHasEdgeTarget(edges []SpecCoverageEdge, target string) bool {
	for _, edge := range edges {
		if edge.Target == target {
			return true
		}
	}

	return false
}
