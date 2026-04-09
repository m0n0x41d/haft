package main

import (
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/artifact"
)

func TestBuildImplementationPrompt_IncludesGovernanceContext(t *testing.T) {
	decision := &artifact.Artifact{
		Meta: artifact.Meta{
			ID:    "dec-123",
			Title: "Desktop governance loop",
		},
	}

	detail := DecisionDetailView{
		ID:              "dec-123",
		Title:           "Desktop governance loop",
		SelectedTitle:   "Desktop governance loop",
		SelectionPolicy: "Prefer backend-authoritative governance.",
		WhySelected:     "It keeps coverage and stale rules in one place.",
		Invariants:      []string{"Use shared artifact logic."},
		Admissibility:   []string{"Do not fork rules into the frontend."},
		AffectedFiles:   []string{"desktop/app.go"},
		CoverageModules: []CoverageModuleView{
			{
				Path:          "desktop",
				Lang:          "go",
				Status:        "covered",
				DecisionCount: 2,
				Files:         []string{"desktop/app.go"},
			},
		},
		Claims: []ClaimView{
			{
				Claim:       "Verification remains possible from the desktop shell.",
				Observable:  "verification task",
				Threshold:   "measurement recorded",
				Status:      "unverified",
				VerifyAfter: "2026-04-16",
			},
		},
	}

	problem := &artifact.Artifact{
		Meta:           artifact.Meta{Title: "Desktop governance gap"},
		StructuredData: `{"signal":"The operator cannot see governance scope.","acceptance":"The operator sees coverage and stale findings.","constraints":["Reuse shared Go logic."]}`,
	}

	prompt := buildImplementationPrompt(decision, detail, []*artifact.Artifact{problem})

	expectedSnippets := []string{
		"## Implement Decision: Desktop governance loop",
		"## Problem Context",
		"desktop/app.go",
		"status=covered",
		"Verify after: 2026-04-16",
	}

	for _, snippet := range expectedSnippets {
		if !strings.Contains(prompt, snippet) {
			t.Fatalf("implementation prompt missing %q:\n%s", snippet, prompt)
		}
	}
}

func TestBuildVerificationPrompt_IncludesClaimWindows(t *testing.T) {
	decision := &artifact.Artifact{
		Meta: artifact.Meta{
			ID:    "dec-verify",
			Title: "Verify desktop governance",
		},
	}

	detail := DecisionDetailView{
		ID:            "dec-verify",
		SelectedTitle: "Verify desktop governance",
		AffectedFiles: []string{"internal/auth/auth.go"},
		Claims: []ClaimView{
			{
				Claim:       "Due claim",
				Observable:  "desktop verification task",
				Threshold:   "measurement recorded",
				Status:      "unverified",
				VerifyAfter: "2026-04-16",
			},
		},
	}

	prompt := buildVerificationPrompt(decision, detail)

	if !strings.Contains(prompt, "Verify after: 2026-04-16") {
		t.Fatalf("verification prompt should include verify_after:\n%s", prompt)
	}

	if !strings.Contains(prompt, "haft_decision(action=\"measure\")") {
		t.Fatalf("verification prompt should include measurement instruction:\n%s", prompt)
	}
}
