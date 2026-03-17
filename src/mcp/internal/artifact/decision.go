package artifact

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// DecideInput is the input for creating a DecisionRecord.
type DecideInput struct {
	ProblemRef    string              `json:"problem_ref,omitempty"`
	PortfolioRef  string              `json:"portfolio_ref,omitempty"`
	SelectedTitle string              `json:"selected_title"`
	WhySelected   string              `json:"why_selected"`
	WhyNotOthers  []RejectionReason   `json:"why_not_others,omitempty"`
	Invariants    []string            `json:"invariants,omitempty"`
	PreConditions []string            `json:"pre_conditions,omitempty"`
	PostConditions []string           `json:"post_conditions,omitempty"`
	Admissibility []string            `json:"admissibility,omitempty"`
	EvidenceReqs  []string            `json:"evidence_requirements,omitempty"`
	Rollback      *RollbackSpec       `json:"rollback,omitempty"`
	RefreshTriggers []string          `json:"refresh_triggers,omitempty"`
	WeakestLink   string              `json:"weakest_link,omitempty"`
	ValidUntil    string              `json:"valid_until,omitempty"`
	Context       string              `json:"context,omitempty"`
	Mode          string              `json:"mode,omitempty"`
	AffectedFiles []string            `json:"affected_files,omitempty"`
}

// RejectionReason explains why a variant was not selected.
type RejectionReason struct {
	Variant string `json:"variant"`
	Reason  string `json:"reason"`
}

// RollbackSpec defines when and how to reverse a decision.
type RollbackSpec struct {
	Triggers   []string `json:"triggers,omitempty"`
	Steps      []string `json:"steps,omitempty"`
	BlastRadius string  `json:"blast_radius,omitempty"`
}

// ApplyInput is the input for generating an implementation brief.
type ApplyInput struct {
	DecisionRef string `json:"decision_ref"`
}

// Decide creates a DecisionRecord artifact — the crown jewel.
func Decide(ctx context.Context, store *Store, quintDir string, input DecideInput) (*Artifact, string, error) {
	if input.SelectedTitle == "" {
		return nil, "", fmt.Errorf("selected_title is required — what variant was chosen?")
	}
	if input.WhySelected == "" {
		return nil, "", fmt.Errorf("why_selected is required — rationale for the choice")
	}

	seq, err := store.NextSequence(ctx, KindDecisionRecord)
	if err != nil {
		return nil, "", fmt.Errorf("generate ID: %w", err)
	}

	id := GenerateID(KindDecisionRecord, seq)
	now := time.Now().UTC()

	mode := Mode(input.Mode)
	if mode == "" {
		mode = ModeStandard
	}

	var links []Link
	if input.ProblemRef != "" {
		links = append(links, Link{Ref: input.ProblemRef, Type: "based_on"})
	}
	if input.PortfolioRef != "" {
		links = append(links, Link{Ref: input.PortfolioRef, Type: "based_on"})
	}

	// Inherit context from linked artifacts
	if input.Context == "" {
		if input.PortfolioRef != "" {
			if p, err := store.Get(ctx, input.PortfolioRef); err == nil {
				input.Context = p.Meta.Context
			}
		} else if input.ProblemRef != "" {
			if p, err := store.Get(ctx, input.ProblemRef); err == nil {
				input.Context = p.Meta.Context
			}
		}
	}

	title := fmt.Sprintf("Decision: %s", input.SelectedTitle)

	// Build the DRR markdown
	var body strings.Builder
	body.WriteString(fmt.Sprintf("# %s\n\n", title))

	body.WriteString(fmt.Sprintf("## Selected Variant\n\n%s\n\n", input.SelectedTitle))
	body.WriteString(fmt.Sprintf("## Why Selected\n\n%s\n\n", input.WhySelected))

	if len(input.WhyNotOthers) > 0 {
		body.WriteString("## Why Not Others\n\n")
		body.WriteString("| Variant | Verdict | Reason |\n")
		body.WriteString("|---------|---------|--------|\n")
		body.WriteString(fmt.Sprintf("| %s | Selected | %s |\n", input.SelectedTitle, truncate(input.WhySelected, 60)))
		for _, r := range input.WhyNotOthers {
			body.WriteString(fmt.Sprintf("| %s | Rejected | %s |\n", r.Variant, r.Reason))
		}
		body.WriteString("\n")
	}

	if len(input.Invariants) > 0 {
		body.WriteString("## Invariants\n\n")
		for _, inv := range input.Invariants {
			body.WriteString(fmt.Sprintf("- %s\n", inv))
		}
		body.WriteString("\n")
	}

	if len(input.PreConditions) > 0 {
		body.WriteString("## Pre-conditions\n\n")
		for _, pc := range input.PreConditions {
			body.WriteString(fmt.Sprintf("- [ ] %s\n", pc))
		}
		body.WriteString("\n")
	}

	if len(input.PostConditions) > 0 {
		body.WriteString("## Post-conditions\n\n")
		for _, pc := range input.PostConditions {
			body.WriteString(fmt.Sprintf("- [ ] %s\n", pc))
		}
		body.WriteString("\n")
	}

	if len(input.Admissibility) > 0 {
		body.WriteString("## Admissibility\n\n")
		for _, a := range input.Admissibility {
			body.WriteString(fmt.Sprintf("- NOT: %s\n", a))
		}
		body.WriteString("\n")
	}

	if len(input.EvidenceReqs) > 0 {
		body.WriteString("## Evidence Requirements\n\n")
		for _, e := range input.EvidenceReqs {
			body.WriteString(fmt.Sprintf("- %s\n", e))
		}
		body.WriteString("\n")
	}

	if input.Rollback != nil {
		body.WriteString("## Rollback Plan\n\n")
		if len(input.Rollback.Triggers) > 0 {
			body.WriteString("**Triggers:**\n")
			for _, t := range input.Rollback.Triggers {
				body.WriteString(fmt.Sprintf("- %s\n", t))
			}
		}
		if len(input.Rollback.Steps) > 0 {
			body.WriteString("\n**Steps:**\n")
			for i, s := range input.Rollback.Steps {
				body.WriteString(fmt.Sprintf("%d. %s\n", i+1, s))
			}
		}
		if input.Rollback.BlastRadius != "" {
			body.WriteString(fmt.Sprintf("\n**Blast radius:** %s\n", input.Rollback.BlastRadius))
		}
		body.WriteString("\n")
	}

	if len(input.RefreshTriggers) > 0 {
		body.WriteString("## Refresh Triggers\n\n")
		for _, rt := range input.RefreshTriggers {
			body.WriteString(fmt.Sprintf("- %s\n", rt))
		}
		body.WriteString("\n")
	}

	if input.WeakestLink != "" {
		body.WriteString(fmt.Sprintf("## Weakest Link\n\n%s\n\n", input.WeakestLink))
	}

	a := &Artifact{
		Meta: Meta{
			ID:         id,
			Kind:       KindDecisionRecord,
			Version:    1,
			Status:     StatusActive,
			Context:    input.Context,
			Mode:       mode,
			Title:      title,
			ValidUntil: input.ValidUntil,
			CreatedAt:  now,
			UpdatedAt:  now,
			Links:      links,
		},
		Body: body.String(),
	}

	if err := store.Create(ctx, a); err != nil {
		return nil, "", fmt.Errorf("store decision: %w", err)
	}

	var warnings []string

	if len(input.AffectedFiles) > 0 {
		var files []AffectedFile
		for _, f := range input.AffectedFiles {
			files = append(files, AffectedFile{Path: f})
		}
		if err := store.SetAffectedFiles(ctx, id, files); err != nil {
			warnings = append(warnings, fmt.Sprintf("failed to track affected files: %v", err))
		}
	}

	filePath, err := WriteFile(quintDir, a)
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("file write failed (DB saved OK): %v", err))
	}

	if len(warnings) > 0 {
		return a, filePath, &WriteWarning{Warnings: warnings}
	}

	return a, filePath, nil
}

// Apply generates an Implementation Brief from an existing DecisionRecord.
func Apply(ctx context.Context, store *Store, decisionRef string) (string, error) {
	a, err := store.Get(ctx, decisionRef)
	if err != nil {
		return "", fmt.Errorf("decision %s not found: %w", decisionRef, err)
	}
	if a.Meta.Kind != KindDecisionRecord {
		return "", fmt.Errorf("%s is %s, not DecisionRecord", decisionRef, a.Meta.Kind)
	}

	var brief strings.Builder
	brief.WriteString(fmt.Sprintf("# Implementation Brief: %s\n\n", a.Meta.Title))
	brief.WriteString(fmt.Sprintf("Decision: %s\n\n", a.Meta.ID))

	// Extract key sections from the DRR body and reformat as brief
	sections := map[string]string{
		"Selected Variant":      "",
		"Invariants":            "",
		"Pre-conditions":        "",
		"Post-conditions":       "",
		"Admissibility":         "",
		"Evidence Requirements": "",
		"Rollback Plan":         "",
	}

	currentSection := ""
	for _, line := range strings.Split(a.Body, "\n") {
		if strings.HasPrefix(line, "## ") {
			heading := strings.TrimPrefix(line, "## ")
			if _, ok := sections[heading]; ok {
				currentSection = heading
			} else {
				currentSection = ""
			}
			continue
		}
		if currentSection != "" {
			sections[currentSection] += line + "\n"
		}
	}

	if s := strings.TrimSpace(sections["Selected Variant"]); s != "" {
		brief.WriteString(fmt.Sprintf("## What to Implement\n\n%s\n\n", s))
	}

	if s := strings.TrimSpace(sections["Invariants"]); s != "" {
		brief.WriteString(fmt.Sprintf("## Invariants (MUST hold)\n\n%s\n\n", s))
	}

	if s := strings.TrimSpace(sections["Admissibility"]); s != "" {
		brief.WriteString(fmt.Sprintf("## NOT Acceptable\n\n%s\n\n", s))
	}

	if s := strings.TrimSpace(sections["Pre-conditions"]); s != "" {
		brief.WriteString(fmt.Sprintf("## Before Starting\n\n%s\n\n", s))
	}

	if s := strings.TrimSpace(sections["Post-conditions"]); s != "" {
		brief.WriteString(fmt.Sprintf("## Definition of Done\n\n%s\n\n", s))
	}

	if s := strings.TrimSpace(sections["Evidence Requirements"]); s != "" {
		brief.WriteString(fmt.Sprintf("## Evidence to Collect\n\n%s\n\n", s))
	}

	if s := strings.TrimSpace(sections["Rollback Plan"]); s != "" {
		brief.WriteString(fmt.Sprintf("## If Things Go Wrong\n\n%s\n\n", s))
	}

	brief.WriteString("---\n\n")
	brief.WriteString(fmt.Sprintf("When complete: use quint_decision(action=\"resolve\", decision_ref=\"%s\")\n", a.Meta.ID))

	return brief.String(), nil
}

// FormatDecisionResponse builds the MCP tool response.
func FormatDecisionResponse(action string, a *Artifact, filePath string, extra string, navStrip string) string {
	var sb strings.Builder

	switch action {
	case "decide":
		sb.WriteString(fmt.Sprintf("Decision recorded: %s\n", a.Meta.Title))
		sb.WriteString(fmt.Sprintf("ID: %s\n", a.Meta.ID))
		if a.Meta.ValidUntil != "" {
			sb.WriteString(fmt.Sprintf("Valid until: %s\n", a.Meta.ValidUntil))
		}
		if filePath != "" {
			sb.WriteString(fmt.Sprintf("File: %s\n", filePath))
		}
	case "apply":
		sb.WriteString(extra)
	}

	sb.WriteString(navStrip)
	return sb.String()
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
