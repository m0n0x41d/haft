package main

import (
	"fmt"
	"strings"

	"github.com/m0n0x41d/haft/internal/artifact"
)

type implementationPromptContext struct {
	PortfolioRationale string
	WorkflowMarkdown   string
}

func buildImplementationPrompt(
	decision *artifact.Artifact,
	detail DecisionDetailView,
	problems []*artifact.Artifact,
	context implementationPromptContext,
) string {
	var brief strings.Builder
	decisionInvariants, governingInvariants := splitImplementationInvariants(detail.Invariants)

	writeSectionTitle(&brief, "Implement Decision", firstNonEmpty(detail.SelectedTitle, decision.Meta.Title))
	writeMetaLine(&brief, "Decision ID", detail.ID)
	writeMetaLine(&brief, "Selected", firstNonEmpty(detail.SelectedTitle, detail.Title))
	writeMetaLine(&brief, "Selection policy", detail.SelectionPolicy)
	writeBlankLine(&brief)

	writeProblemContexts(&brief, problems)
	writeParagraphSection(&brief, "Solution Portfolio Rationale", context.PortfolioRationale)
	writeParagraphSection(&brief, "Why Selected", detail.WhySelected)
	writeParagraphSection(&brief, "Counterargument", detail.CounterArgument)
	writeStringListSection(&brief, "Invariants (must hold)", decisionInvariants, "- ")
	writeStringListSection(&brief, "Governing Invariants (knowledge graph)", governingInvariants, "- ")
	writeStringListSection(&brief, "Not Acceptable", detail.Admissibility, "- ")
	writeStringListSection(&brief, "Affected Files", detail.AffectedFiles, "- ")
	writeCoverageSection(&brief, detail.CoverageModules)
	writeStringListSection(&brief, "Coverage Warnings", detail.CoverageWarnings, "- ")
	writeStringListSection(&brief, "Post-conditions", detail.PostConditions, "- [ ] ")
	writeClaimsSection(&brief, detail.Claims)
	writeLiteralSection(&brief, "Workflow Policy (.haft/workflow.md)", context.WorkflowMarkdown)
	writeInstructionSection(
		&brief,
		[]string{
			"Inspect the current code path before editing and keep the change scoped to the selected decision.",
			"Preserve every invariant and every admissibility boundary while implementing.",
			"Treat the affected files list as the primary implementation scope. If scope must expand, make that explicit in your rationale and evidence.",
			"After implementation, verify each post-condition and note any claim that still needs asynchronous evidence.",
			"Use h-reason discipline while coding: frame the actual local problem before choosing a non-trivial variant.",
		},
	)

	return brief.String()
}

func buildVerificationPrompt(
	decision *artifact.Artifact,
	detail DecisionDetailView,
) string {
	var prompt strings.Builder

	writeSectionTitle(&prompt, "Verify Decision", firstNonEmpty(detail.SelectedTitle, decision.Meta.Title))
	writeMetaLine(&prompt, "Decision ID", detail.ID)
	writeMetaLine(&prompt, "Selected", firstNonEmpty(detail.SelectedTitle, detail.Title))
	writeBlankLine(&prompt)

	writeStringListSection(&prompt, "Affected Files", detail.AffectedFiles, "- ")
	writeCoverageSection(&prompt, detail.CoverageModules)
	writeStringListSection(&prompt, "Coverage Warnings", detail.CoverageWarnings, "- ")
	writeClaimsSection(&prompt, detail.Claims)
	writeInstructionSection(
		&prompt,
		[]string{
			"Check each claim with concrete evidence. Use commands, tests, file inspection, or dashboards as needed.",
			"Prioritize claims whose verify_after window has passed, but do not skip the rest.",
			"Assess each claim as supported, weakened, refuted, or inconclusive with an explicit reason.",
			"Call haft_decision(action=\"measure\") with the measured findings once evidence is gathered.",
			"Do not fabricate evidence or mark a claim supported without a concrete observable.",
		},
	)

	return prompt.String()
}

func decisionTaskTitle(prefix string, detail DecisionDetailView) string {
	label := firstNonEmpty(detail.SelectedTitle, detail.Title, detail.ID)
	return fmt.Sprintf("%s: %s", prefix, label)
}

func writeProblemContexts(builder *strings.Builder, problems []*artifact.Artifact) {
	if len(problems) == 0 {
		return
	}

	builder.WriteString("## Problem Context\n")
	for _, problem := range problems {
		fields := problem.UnmarshalProblemFields()

		builder.WriteString(fmt.Sprintf("- %s\n", problem.Meta.Title))
		if fields.Signal != "" {
			builder.WriteString(fmt.Sprintf("  Signal: %s\n", fields.Signal))
		}
		if fields.Acceptance != "" {
			builder.WriteString(fmt.Sprintf("  Acceptance: %s\n", fields.Acceptance))
		}
		for _, constraint := range fields.Constraints {
			builder.WriteString(fmt.Sprintf("  Constraint: %s\n", constraint))
		}
	}
	writeBlankLine(builder)
}

func writeClaimsSection(builder *strings.Builder, claims []ClaimView) {
	if len(claims) == 0 {
		return
	}

	builder.WriteString("## Claims\n")
	for _, claim := range claims {
		builder.WriteString(fmt.Sprintf("- %s\n", claim.Claim))
		builder.WriteString(fmt.Sprintf("  Observable: %s\n", claim.Observable))
		builder.WriteString(fmt.Sprintf("  Threshold: %s\n", claim.Threshold))
		builder.WriteString(fmt.Sprintf("  Current status: %s\n", claim.Status))
		if claim.VerifyAfter != "" {
			builder.WriteString(fmt.Sprintf("  Verify after: %s\n", claim.VerifyAfter))
		}
	}
	writeBlankLine(builder)
}

func writeCoverageSection(builder *strings.Builder, modules []CoverageModuleView) {
	if len(modules) == 0 {
		return
	}

	builder.WriteString("## Impacted Modules\n")
	for _, module := range modules {
		path := firstNonEmpty(module.Path, "(root)")
		builder.WriteString(fmt.Sprintf("- %s [%s] status=%s decisions=%d\n", path, module.Lang, module.Status, module.DecisionCount))
		for _, filePath := range module.Files {
			builder.WriteString(fmt.Sprintf("  File: %s\n", filePath))
		}
	}
	writeBlankLine(builder)
}

func writeStringListSection(builder *strings.Builder, title string, values []string, prefix string) {
	if len(values) == 0 {
		return
	}

	builder.WriteString(fmt.Sprintf("## %s\n", title))
	for _, value := range values {
		builder.WriteString(prefix)
		builder.WriteString(value)
		builder.WriteString("\n")
	}
	writeBlankLine(builder)
}

func writeParagraphSection(builder *strings.Builder, title string, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}

	builder.WriteString(fmt.Sprintf("## %s\n", title))
	builder.WriteString(value)
	builder.WriteString("\n\n")
}

func writeLiteralSection(builder *strings.Builder, title string, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}

	builder.WriteString(fmt.Sprintf("## %s\n", title))
	builder.WriteString(strings.TrimSpace(value))
	builder.WriteString("\n\n")
}

func writeInstructionSection(builder *strings.Builder, instructions []string) {
	if len(instructions) == 0 {
		return
	}

	builder.WriteString("## Instructions\n")
	for index, instruction := range instructions {
		builder.WriteString(fmt.Sprintf("%d. %s\n", index+1, instruction))
	}
	writeBlankLine(builder)
}

func writeSectionTitle(builder *strings.Builder, title string, value string) {
	builder.WriteString(fmt.Sprintf("## %s: %s\n\n", title, value))
}

func writeMetaLine(builder *strings.Builder, label string, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}

	builder.WriteString(fmt.Sprintf("%s: %s\n", label, value))
}

func writeBlankLine(builder *strings.Builder) {
	builder.WriteString("\n")
}

func splitImplementationInvariants(values []string) ([]string, []string) {
	decisionInvariants := make([]string, 0, len(values))
	governingInvariants := make([]string, 0, len(values))

	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if isKnowledgeGraphInvariant(trimmed) {
			governingInvariants = append(governingInvariants, trimmed)
			continue
		}
		decisionInvariants = append(decisionInvariants, trimmed)
	}

	return decisionInvariants, governingInvariants
}

func isKnowledgeGraphInvariant(value string) bool {
	if !strings.HasPrefix(value, "[dec-") {
		return false
	}

	return strings.Contains(value, "] ")
}
