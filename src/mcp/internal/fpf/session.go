package fpf

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/m0n0x41d/quint-code/logger"
)

func (t *Tools) Internalize(ctx context.Context, input InternalizeInput) (string, error) {
	defer t.RecordWork("Internalize", time.Now())

	hasWriteOp := input.Remember != nil || input.Forget != "" || input.Overwrite != nil
	if hasWriteOp {
		return t.handleContextWrite(ctx, input)
	}

	logger.Info().Str("root_dir", t.RootDir).Msg("Internalize called")

	result := InternalizeResult{
		Phase:          string(StageEmpty),
		SuggestedPhase: "No hypotheses yet",
		Role:           string(RoleObserver),
		LayerCounts:    make(map[string]int),
		NextAction:     "→ /q1-hypothesize to start reasoning",
	}

	if !t.IsInitialized() {
		if err := t.InitProject(); err != nil {
			return "", fmt.Errorf("initialization failed: %w", err)
		}
		result.Status = "INITIALIZED"
		result.ContextChanges = []string{"Created .quint/ structure"}

		pCtx, err := t.AnalyzeProject()
		if err != nil {
			result.ContextChanges = append(result.ContextChanges, fmt.Sprintf("Warning: auto-analysis failed: %v", err))
		} else {
			if _, err := t.RecordContextFromProject(pCtx); err != nil {
				result.ContextChanges = append(result.ContextChanges, fmt.Sprintf("Warning: failed to record context: %v", err))
			} else {
				result.ContextChanges = append(result.ContextChanges, "Auto-generated context from project analysis")
			}
		}

		result.Phase = string(StageEmpty)
		result.SuggestedPhase = "No hypotheses yet"
		result.Role = string(RoleObserver)
	} else {
		stale, signals := t.IsContextStale()
		if stale {
			pCtx, err := t.AnalyzeProject()
			if err != nil {
				result.ContextChanges = append(result.ContextChanges, fmt.Sprintf("Warning: re-analysis failed: %v", err))
			} else {
				if _, err := t.RecordContextFromProject(pCtx); err != nil {
					result.ContextChanges = append(result.ContextChanges, fmt.Sprintf("Warning: failed to update context: %v", err))
				}
			}
			result.Status = "UPDATED"
			result.ContextChanges = signals
		} else {
			result.Status = "READY"
		}
	}

	result.ContextID = "default"
	result.ArchivedCounts = make(map[string]int)

	result.ContextMaturity, result.MissingSections = t.CalculateContextMaturity()
	_, result.StalenessWarnings = t.IsContextStale()

	if t.DB != nil {
		activeCounts, err := t.DB.CountActiveHolonsByLayer(ctx)
		if err == nil {
			for _, c := range activeCounts {
				result.LayerCounts[c.Layer] = int(c.Count)
			}
		} else {
			result.LayerCounts["L0"] = t.countHolons(ctx, "L0")
			result.LayerCounts["L1"] = t.countHolons(ctx, "L1")
			result.LayerCounts["L2"] = t.countHolons(ctx, "L2")
		}

		archivedCounts, err := t.DB.CountArchivedHolonsByLayer(ctx)
		if err == nil {
			for _, c := range archivedCounts {
				result.ArchivedCounts[c.Layer] = int(c.Count)
			}
		}
	} else {
		result.LayerCounts["L0"] = t.countHolons(ctx, "L0")
		result.LayerCounts["L1"] = t.countHolons(ctx, "L1")
		result.LayerCounts["L2"] = t.countHolons(ctx, "L2")
	}
	result.LayerCounts["DRR"] = t.countDRRs()

	if t.DB != nil {
		holons, err := t.DB.GetActiveRecentHolons(ctx, 10)
		if err == nil {
			for _, h := range holons {
				summary := HolonSummary{
					ID:    h.ID,
					Title: h.Title,
					Layer: h.Layer,
				}
				if h.Kind.Valid {
					summary.Kind = h.Kind.String
				}
				if h.CachedRScore.Valid {
					summary.RScore = h.CachedRScore.Float64
				}
				if h.UpdatedAt.Valid {
					summary.UpdatedAt = h.UpdatedAt.Time
				}
				result.RecentHolons = append(result.RecentHolons, summary)
			}
		}

		evidence, err := t.DB.GetDecayingEvidence(ctx, 7)
		if err == nil {
			for _, e := range evidence {
				warning := DecayWarning{
					EvidenceID: e.ID,
					HolonID:    e.HolonID,
				}
				if e.ValidUntil.Valid {
					warning.ExpiresAt = e.ValidUntil.Time
					warning.DaysLeft = int(time.Until(e.ValidUntil.Time).Hours() / 24)
				}
				if title, err := t.DB.GetHolonTitle(ctx, e.HolonID); err == nil {
					warning.HolonTitle = title
				}
				result.DecayWarnings = append(result.DecayWarnings, warning)
			}
		}

		openDecisions, err := t.GetOpenDecisions(ctx)
		if err == nil {
			result.OpenDecisions = openDecisions
			for _, d := range openDecisions {
				warnings := t.checkDecisionAffectedScope(d.ID, d.Title)
				result.AffectedScopeWarnings = append(result.AffectedScopeWarnings, warnings...)
			}
		}
		resolvedDecisions, err := t.GetRecentResolvedDecisions(ctx, 5)
		if err == nil {
			result.ResolvedDecisions = resolvedDecisions
		}

		activeContexts, err := t.GetActiveDecisionContexts(ctx)
		if err == nil {
			result.ActiveContexts = activeContexts
			if len(activeContexts) > 0 {
				mostAdvancedStage := t.getMostAdvancedStage(activeContexts)
				result.Phase = string(mostAdvancedStage)
				result.SuggestedPhase, result.NextAction = GetContextStageDescription(mostAdvancedStage)
			}
		}
	}

	if len(result.ActiveContexts) == 0 {
		result.NextAction = t.getNextAction(StageEmpty, result.LayerCounts["L0"], result.LayerCounts["L1"], result.LayerCounts["L2"])
	}

	logger.Info().
		Str("status", result.Status).
		Int("active_contexts", len(result.ActiveContexts)).
		Int("decay_warnings", len(result.DecayWarnings)).
		Int("scope_warnings", len(result.AffectedScopeWarnings)).
		Msg("Internalize: completed")

	return t.formatInternalizeOutput(result), nil
}

func (t *Tools) checkDecisionAffectedScope(drrID, drrTitle string) []AffectedScopeWarning {
	var warnings []AffectedScopeWarning

	contract, err := t.getDRRContract(drrID)
	if err != nil || contract == nil {
		return warnings
	}

	if len(contract.AffectedHashes) == 0 {
		return warnings
	}

	for file, oldHash := range contract.AffectedHashes {
		if oldHash == "_missing_" {
			continue
		}
		fullPath := filepath.Join(t.RootDir, file)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			warnings = append(warnings, AffectedScopeWarning{
				DecisionID:    drrID,
				DecisionTitle: drrTitle,
				FilePath:      file,
				ChangeType:    "removed",
				OldHash:       oldHash,
			})
			continue
		}
		hash := sha256.Sum256(content)
		currentHash := hex.EncodeToString(hash[:8])
		if currentHash != oldHash {
			warnings = append(warnings, AffectedScopeWarning{
				DecisionID:    drrID,
				DecisionTitle: drrTitle,
				FilePath:      file,
				ChangeType:    "modified",
				OldHash:       oldHash,
				NewHash:       currentHash,
			})
		}
	}
	return warnings
}

func (t *Tools) formatInternalizeOutput(r InternalizeResult) string {
	var sb strings.Builder

	sb.WriteString("=== QUINT INTERNALIZE ===\n\n")
	sb.WriteString(fmt.Sprintf("Status: %s\n", r.Status))
	sb.WriteString(fmt.Sprintf("Context Maturity: %s\n", r.ContextMaturity))
	sb.WriteString(fmt.Sprintf("Session Phase: %s\n", r.Phase))
	if r.SuggestedPhase != "" && r.SuggestedPhase != r.Phase {
		sb.WriteString(fmt.Sprintf("Suggested Phase: %s (based on knowledge state)\n", r.SuggestedPhase))
	}
	sb.WriteString(fmt.Sprintf("Role: %s\n", r.Role))
	sb.WriteString(fmt.Sprintf("Context: %s\n\n", r.ContextID))

	if len(r.StalenessWarnings) > 0 {
		sb.WriteString("📋 Context Staleness:\n")
		for _, w := range r.StalenessWarnings {
			sb.WriteString(fmt.Sprintf("  - %s\n", w))
		}
		sb.WriteString("\n")
	}

	if len(r.MissingSections) > 0 && r.ContextMaturity != "L3" {
		sb.WriteString("📝 Missing Context Sections:\n")
		for _, s := range r.MissingSections {
			sb.WriteString(fmt.Sprintf("  - %s\n", s))
		}
		sb.WriteString("  → Agent should ask user about these to enrich context.md\n\n")
	}

	if len(r.ContextChanges) > 0 {
		sb.WriteString("Context Changes:\n")
		for _, c := range r.ContextChanges {
			sb.WriteString(fmt.Sprintf("  - %s\n", c))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("Knowledge State (Active):\n")
	sb.WriteString(fmt.Sprintf("  L0 (Conjecture): %d\n", r.LayerCounts["L0"]))
	sb.WriteString(fmt.Sprintf("  L1 (Substantiated): %d\n", r.LayerCounts["L1"]))
	sb.WriteString(fmt.Sprintf("  L2 (Corroborated): %d\n", r.LayerCounts["L2"]))
	if r.LayerCounts["DRR"] > 0 {
		sb.WriteString(fmt.Sprintf("  DRRs: %d\n", r.LayerCounts["DRR"]))
	}

	totalArchived := r.ArchivedCounts["L0"] + r.ArchivedCounts["L1"] + r.ArchivedCounts["L2"]
	if totalArchived > 0 {
		sb.WriteString(fmt.Sprintf("  (Archived: %d holons in resolved decisions)\n", totalArchived))
	}
	sb.WriteString("\n")

	if len(r.ActiveContexts) > 0 {
		sb.WriteString(fmt.Sprintf("Active Decision Contexts (%d/3):\n", len(r.ActiveContexts)))
		for _, dc := range r.ActiveContexts {
			desc, _ := GetContextStageDescription(dc.Stage)
			sb.WriteString(fmt.Sprintf("  - %s: %s (%d hypotheses) [%s]\n",
				dc.ID, dc.Title, dc.HypothesisCount, desc))
			if dc.DiversityWarning != "" {
				sb.WriteString(fmt.Sprintf("    ⚠️ %s\n", dc.DiversityWarning))
			}
		}
		sb.WriteString("\n")
	} else {
		sb.WriteString("No active decision contexts. Use /q1-hypothesize to start.\n\n")
	}

	if len(r.RecentHolons) > 0 {
		sb.WriteString("Recent Active Holons:\n")
		for _, h := range r.RecentHolons {
			age := formatAge(h.UpdatedAt)
			sb.WriteString(fmt.Sprintf("  - %s [%s] R=%.2f - %s\n", h.ID, h.Layer, h.RScore, age))
		}
		sb.WriteString("\n")
	}

	if len(r.DecayWarnings) > 0 {
		sb.WriteString("⚠ Attention Required:\n")
		for _, w := range r.DecayWarnings {
			sb.WriteString(fmt.Sprintf("  - Evidence \"%s\" for \"%s\" expires in %d days\n",
				w.EvidenceID, w.HolonTitle, w.DaysLeft))
		}
		sb.WriteString("\n")
	}

	if len(r.AffectedScopeWarnings) > 0 {
		sb.WriteString("🔴 AFFECTED SCOPE CHANGED:\n")
		grouped := make(map[string][]AffectedScopeWarning)
		for _, w := range r.AffectedScopeWarnings {
			grouped[w.DecisionID] = append(grouped[w.DecisionID], w)
		}
		for drrID, warnings := range grouped {
			title := warnings[0].DecisionTitle
			sb.WriteString(fmt.Sprintf("  %s (%s):\n", drrID, title))
			for _, w := range warnings {
				if w.ChangeType == "removed" {
					sb.WriteString(fmt.Sprintf("    - %s: file removed\n", w.FilePath))
				} else {
					sb.WriteString(fmt.Sprintf("    - %s: modified (was %s, now %s)\n", w.FilePath, w.OldHash, w.NewHash))
				}
			}
		}
		sb.WriteString("  → Check changes with 'git diff', then either:\n")
		sb.WriteString("    • /q-implement — if changes don't invalidate decision, proceed with implementation\n")
		sb.WriteString("    • /q-resolve abandoned — if changes make decision obsolete\n")
		sb.WriteString("    • /q1-hypothesize — start fresh if requirements changed\n\n")
	}

	if len(r.OpenDecisions) > 0 {
		sb.WriteString("⚠ Unresolved Decisions (status unknown — check acceptance criteria before resolving):\n")
		for _, d := range r.OpenDecisions {
			age := formatAge(d.CreatedAt)
			sb.WriteString(fmt.Sprintf("  - %s: %s (%s)\n", d.ID, d.Title, age))
		}
		sb.WriteString("\n")
	}

	if len(r.ResolvedDecisions) > 0 {
		sb.WriteString("Recent Resolutions:\n")
		for _, d := range r.ResolvedDecisions {
			age := formatAge(d.ResolvedAt)
			sb.WriteString(fmt.Sprintf("  - %s: %s [%s] %s\n", d.ID, d.Title, d.Resolution, age))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(fmt.Sprintf("Next Action: %s", r.NextAction))

	return sb.String()
}

func (t *Tools) IsInitialized() bool {
	_, err := os.Stat(t.GetFPFDir())
	return err == nil
}

func (t *Tools) handleContextWrite(ctx context.Context, input InternalizeInput) (string, error) {
	if t.DB == nil {
		return "", fmt.Errorf("database not initialized")
	}

	var sb strings.Builder
	sb.WriteString("=== QUINT INTERNALIZE (Write) ===\n\n")

	if input.Remember != nil {
		if input.Remember.Category == "" || input.Remember.Content == "" {
			return "", fmt.Errorf("remember requires both category and content")
		}
		if err := t.DB.AppendContextFact(ctx, input.Remember.Category, input.Remember.Content); err != nil {
			return "", fmt.Errorf("failed to remember: %w", err)
		}
		sb.WriteString(fmt.Sprintf("✓ Remembered fact in category '%s'\n", input.Remember.Category))
	}

	if input.Forget != "" {
		if err := t.DB.DeleteContextFact(ctx, input.Forget); err != nil {
			return "", fmt.Errorf("failed to forget: %w", err)
		}
		sb.WriteString(fmt.Sprintf("✓ Forgot category '%s'\n", input.Forget))
	}

	if input.Overwrite != nil {
		if input.Overwrite.Category == "" || input.Overwrite.Content == "" {
			return "", fmt.Errorf("overwrite requires both category and content")
		}
		if err := t.DB.UpsertContextFact(ctx, input.Overwrite.Category, input.Overwrite.Content); err != nil {
			return "", fmt.Errorf("failed to overwrite: %w", err)
		}
		sb.WriteString(fmt.Sprintf("✓ Overwrote category '%s'\n", input.Overwrite.Category))
	}

	if err := t.regenerateContextMD(ctx); err != nil {
		sb.WriteString(fmt.Sprintf("\n⚠ Warning: failed to regenerate context.md: %v\n", err))
	} else {
		sb.WriteString("\n✓ context.md regenerated from DB\n")
	}

	facts, err := t.DB.GetAllContextFacts(ctx)
	if err == nil && len(facts) > 0 {
		sb.WriteString("\nCurrent categories:\n")
		for _, f := range facts {
			preview := f.Content
			if len(preview) > 50 {
				preview = preview[:50] + "..."
			}
			sb.WriteString(fmt.Sprintf("  - %s: %s\n", f.Category, preview))
		}
	}

	return sb.String(), nil
}

func (t *Tools) regenerateContextMD(ctx context.Context) error {
	facts, err := t.DB.GetAllContextFacts(ctx)
	if err != nil {
		return fmt.Errorf("failed to get context facts: %w", err)
	}

	pCtx, _ := t.AnalyzeProject()

	var sb strings.Builder
	sb.WriteString("<!-- ATTENTION: This file is auto-generated by quint_internalize.\n")
	sb.WriteString("     DO NOT EDIT DIRECTLY - changes will be overwritten.\n")
	sb.WriteString("     Use quint_internalize(remember/forget/overwrite) to modify. -->\n\n")
	sb.WriteString("# Bounded Context\n\n")

	for _, f := range facts {
		sb.WriteString(fmt.Sprintf("## %s\n\n", f.Category))
		sb.WriteString(f.Content)
		sb.WriteString("\n\n")
	}

	if len(pCtx.DRRInvariants) > 0 {
		sb.WriteString("## Invariants (from DRRs)\n\n")
		for _, inv := range pCtx.DRRInvariants {
			sb.WriteString(fmt.Sprintf("- %s\n", inv))
		}
		sb.WriteString("\n")
	}

	if len(pCtx.DRRAntiPatterns) > 0 {
		sb.WriteString("## Anti-Patterns (from DRRs)\n\n")
		for _, ap := range pCtx.DRRAntiPatterns {
			sb.WriteString(fmt.Sprintf("- %s\n", ap))
		}
		sb.WriteString("\n")
	}

	hasCustomNotes := false
	for _, f := range facts {
		if f.Category == "Custom Notes" {
			hasCustomNotes = true
			break
		}
	}
	if !hasCustomNotes {
		sb.WriteString("## Custom Notes\n\n")
		sb.WriteString("*Add project-specific notes here. This section is preserved across regenerations.*\n\n")
	}

	path := filepath.Join(t.GetFPFDir(), "context.md")
	return os.WriteFile(path, []byte(sb.String()), 0644)
}

func (t *Tools) AnalyzeProject() (ProjectContext, error) {
	pCtx := ProjectContext{}

	pCtx.TechStack = t.detectTechStack()
	pCtx.DRRInvariants, pCtx.DRRAntiPatterns = t.aggregateDRRContracts()

	return pCtx, nil
}


func (t *Tools) detectTechStack() []string {
	var stack []string

	if _, err := os.Stat(filepath.Join(t.RootDir, "go.mod")); err == nil {
		stack = append(stack, "Go")
	}
	if _, err := os.Stat(filepath.Join(t.RootDir, "package.json")); err == nil {
		stack = append(stack, "Node.js")
	}
	pythonMarkers := []string{"requirements.txt", "pyproject.toml", "setup.py", "Pipfile"}
	for _, marker := range pythonMarkers {
		if _, err := os.Stat(filepath.Join(t.RootDir, marker)); err == nil {
			stack = append(stack, "Python")
			break
		}
	}
	if _, err := os.Stat(filepath.Join(t.RootDir, "Cargo.toml")); err == nil {
		stack = append(stack, "Rust")
	}
	if _, err := os.Stat(filepath.Join(t.RootDir, "pom.xml")); err == nil {
		stack = append(stack, "Java (Maven)")
	}
	if _, err := os.Stat(filepath.Join(t.RootDir, "build.gradle")); err == nil {
		stack = append(stack, "Java/Kotlin (Gradle)")
	}
	if _, err := os.Stat(filepath.Join(t.RootDir, "build.gradle.kts")); err == nil {
		stack = append(stack, "Kotlin (Gradle KTS)")
	}
	if _, err := os.Stat(filepath.Join(t.RootDir, "Gemfile")); err == nil {
		stack = append(stack, "Ruby")
	}
	if _, err := os.Stat(filepath.Join(t.RootDir, "Makefile")); err == nil {
		stack = append(stack, "Make")
	}

	return stack
}


func (t *Tools) aggregateDRRContracts() ([]string, []string) {
	var invariants, antiPatterns []string

	decisionsDir := filepath.Join(t.GetFPFDir(), "decisions")
	files, err := os.ReadDir(decisionsDir)
	if err != nil {
		return invariants, antiPatterns
	}

	resolvedDRRs := make(map[string]bool)
	if t.DB != nil {
		ctx := context.Background()
		resolved, err := t.DB.GetResolvedDecisions(ctx, "implementation", 100)
		if err == nil {
			for _, d := range resolved {
				resolvedDRRs[d.ID] = true
			}
		}
	}

	for _, f := range files {
		if f.IsDir() || !strings.HasPrefix(f.Name(), "DRR-") || !strings.HasSuffix(f.Name(), ".md") {
			continue
		}

		drrID := strings.TrimSuffix(f.Name(), ".md")
		if len(resolvedDRRs) > 0 && !resolvedDRRs[drrID] {
			continue
		}

		contract, err := t.getDRRContract(drrID)
		if err != nil || contract == nil {
			continue
		}

		for _, inv := range contract.GetLaws() {
			invariants = append(invariants, inv)
		}
		for _, ap := range contract.GetAdmissibility() {
			antiPatterns = append(antiPatterns, ap)
		}
	}

	return invariants, antiPatterns
}

func (t *Tools) IsContextStale() (bool, []string) {
	var signals []string

	contextPath := filepath.Join(t.GetFPFDir(), "context.md")
	contextInfo, err := os.Stat(contextPath)
	if err != nil {
		return true, []string{"No context.md found, creating initial context"}
	}
	contextMod := contextInfo.ModTime()

	goModPath := filepath.Join(t.RootDir, "go.mod")
	if info, err := os.Stat(goModPath); err == nil {
		if info.ModTime().After(contextMod) {
			signals = append(signals, "go.mod modified since last context update")
		}
	}

	pkgPath := filepath.Join(t.RootDir, "package.json")
	if info, err := os.Stat(pkgPath); err == nil {
		if info.ModTime().After(contextMod) {
			signals = append(signals, "package.json modified since last context update")
		}
	}

	readmePath := filepath.Join(t.RootDir, "README.md")
	if info, err := os.Stat(readmePath); err == nil {
		if info.ModTime().After(contextMod) {
			signals = append(signals, "README.md modified since last context update")
		}
	}

	decisionsDir := filepath.Join(t.GetFPFDir(), "decisions")
	if entries, err := os.ReadDir(decisionsDir); err == nil {
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), "DRR-") && strings.HasSuffix(entry.Name(), ".md") {
				info, err := entry.Info()
				if err == nil && info.ModTime().After(contextMod) {
					signals = append(signals, "New DRRs created since last context update")
					break
				}
			}
		}
	}

	if time.Since(contextMod) > 7*24*time.Hour {
		signals = append(signals, fmt.Sprintf("Context is %d days old", int(time.Since(contextMod).Hours()/24)))
	}

	return len(signals) > 0, signals
}

func (t *Tools) CalculateContextMaturity() (string, []string) {
	contextPath := filepath.Join(t.GetFPFDir(), "context.md")
	content, err := os.ReadFile(contextPath)
	if err != nil {
		return "L0", []string{"Overview", "Tech Stack", "Structure", "Invariants (from DRRs)", "Custom Notes"}
	}

	contentStr := string(content)

	sections := map[string]bool{
		"Overview":             strings.Contains(contentStr, "## Overview"),
		"Tech Stack":           strings.Contains(contentStr, "## Tech Stack"),
		"Structure":            strings.Contains(contentStr, "## Structure"),
		"Invariants (from DRRs)": strings.Contains(contentStr, "## Invariants (from DRRs)"),
		"Custom Notes":         t.hasCustomNotesContent(contentStr),
	}

	var missing []string
	present := 0
	for name, exists := range sections {
		if exists {
			present++
		} else {
			missing = append(missing, name)
		}
	}

	var maturity string
	switch {
	case present >= 5:
		maturity = "L3"
	case present >= 4:
		maturity = "L2"
	case present >= 2:
		maturity = "L1"
	default:
		maturity = "L0"
	}

	return maturity, missing
}

func (t *Tools) hasCustomNotesContent(content string) bool {
	lines := strings.Split(content, "\n")
	inCustomNotes := false
	for _, line := range lines {
		if strings.HasPrefix(line, "## Custom Notes") {
			inCustomNotes = true
			continue
		}
		if inCustomNotes {
			if strings.HasPrefix(line, "## ") {
				break
			}
			trimmed := strings.TrimSpace(line)
			if trimmed != "" && !strings.HasPrefix(trimmed, "*Add project-specific notes") {
				return true
			}
		}
	}
	return false
}

func (t *Tools) GetStatus(ctx context.Context) (string, error) {
	stage := StageEmpty
	if t.DB != nil {
		if activeContexts, err := t.GetActiveDecisionContexts(ctx); err == nil && len(activeContexts) > 0 {
			stage = t.getMostAdvancedStage(activeContexts)
		}
	}

	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("STAGE: %s\n", stage))
	sb.WriteString(fmt.Sprintf("ROLE: %s\n\n", RoleObserver))

	l0 := t.countHolons(ctx, "L0")
	l1 := t.countHolons(ctx, "L1")
	l2 := t.countHolons(ctx, "L2")
	drr := t.countDRRs()

	sb.WriteString("## Knowledge\n")
	sb.WriteString(fmt.Sprintf("- L0 (Conjecture): %d\n", l0))
	sb.WriteString(fmt.Sprintf("- L1 (Substantiated): %d\n", l1))
	sb.WriteString(fmt.Sprintf("- L2 (Corroborated): %d\n", l2))
	if drr > 0 {
		sb.WriteString(fmt.Sprintf("- DRR (Decisions): %d\n", drr))
	}
	sb.WriteString("\n")

	sb.WriteString("## Next\n")
	sb.WriteString(t.getNextAction(stage, l0, l1, l2))

	return sb.String(), nil
}

func (t *Tools) countHolons(ctx context.Context, layer string) int {
	if t.DB == nil {
		return 0
	}
	return int(t.DB.CountHypothesesByLayer(ctx, layer))
}

func (t *Tools) countDRRs() int {
	dir := filepath.Join(t.GetFPFDir(), "decisions")
	files, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	count := 0
	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".md") && strings.HasPrefix(f.Name(), "DRR-") {
			count++
		}
	}
	return count
}

func (t *Tools) getMostAdvancedStage(contexts []DecisionContextSummary) ContextStage {
	stagePriority := map[ContextStage]int{
		StageEmpty:           0,
		StageNeedsVerify:     1,
		StageNeedsValidation: 2,
		StageNeedsAudit:      3,
		StageReadyToDecide:   4,
	}

	maxStage := StageEmpty
	for _, c := range contexts {
		if stagePriority[c.Stage] > stagePriority[maxStage] {
			maxStage = c.Stage
		}
	}
	return maxStage
}

func (t *Tools) getNextAction(stage ContextStage, l0, l1, l2 int) string {
	switch stage {
	case StageEmpty:
		return "→ /q1-hypothesize to start reasoning\n"
	case StageNeedsVerify:
		if l0 > 0 {
			return fmt.Sprintf("→ %d L0 ready for /q2-verify\n", l0)
		}
		return "→ /q1-hypothesize to generate hypotheses\n"
	case StageNeedsValidation:
		if l1 > 0 {
			return fmt.Sprintf("→ %d L1 ready for /q3-validate\n", l1)
		}
		return "→ /q2-verify to check logic\n"
	case StageNeedsAudit:
		if l2 > 0 {
			return fmt.Sprintf("→ %d L2 ready for /q4-audit\n", l2)
		}
		return "→ /q3-validate to gather evidence\n"
	case StageReadyToDecide:
		return "→ /q5-decide to finalize\n"
	default:
		return ""
	}
}
