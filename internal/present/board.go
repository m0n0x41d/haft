// Board rendering — pure functions that format BoardData as rich ANSI terminal output.
// Used by both `haft board` CLI and `/board` agent command.
// No side effects, no store access.
package present

import (
	"fmt"
	"sort"
	"strings"

	"github.com/m0n0x41d/haft/internal/codebase"
	"github.com/m0n0x41d/haft/internal/ui"
)

// ANSI helpers
const (
	reset  = "\033[0m"
	bold   = "\033[1m"
	dim    = "\033[2m"
	red    = "\033[31m"
	green  = "\033[32m"
	yellow = "\033[33m"
	cyan   = "\033[36m"
	white  = "\033[37m"
)

func c(color, text string) string    { return color + text + reset }
func bc(color, text string) string   { return bold + color + text + reset }
func reffC(r float64) string {
	if r >= 0.7 { return green }
	if r >= 0.3 { return yellow }
	return red
}
func bar(filled, total, width int) string {
	if total == 0 { return strings.Repeat("░", width) }
	w := filled * width / total
	if w > width { w = width }
	return c(green, strings.Repeat("█", w)) + c(dim, strings.Repeat("░", width-w))
}
func pad(s string, w int) string {
	if len(s) >= w { return s[:w] }
	return s + strings.Repeat(" ", w-len(s))
}
func trunc(s string, w int) string {
	if len(s) <= w { return s }
	if w <= 3 { return s[:w] }
	return s[:w-3] + "..."
}
func sep(w int) string { return c(dim, strings.Repeat("─", w)) + "\n" }

// ──────────────────────────────────────────────────────────────
// View: Overview
// ──────────────────────────────────────────────────────────────

func BoardOverview(d *ui.BoardData) string { return BoardOverviewW(d, 0) }

func BoardOverviewW(d *ui.BoardData, width int) string {
	if width <= 0 { width = 100 }
	var sb strings.Builder

	// Header
	header := "HAFT HEALTH: " + d.ProjectName
	sb.WriteString(bc(cyan, header))
	if d.CoverageReport != nil && d.CoverageReport.TotalModules > 0 {
		cr := d.CoverageReport
		pct := (cr.CoveredCount + cr.PartialCount) * 100 / cr.TotalModules
		sb.WriteString(fmt.Sprintf("   %s %d%% governed", bar(cr.CoveredCount+cr.PartialCount, cr.TotalModules, 16), pct))
	}
	sb.WriteString("\n" + sep(width) + "\n")

	// 4-column summary
	g, a, r, bare := trustCounts(d)
	totalDec := len(d.Decisions)
	totalProb := len(d.BacklogProblems) + len(d.InProgressProblems) + d.AddressedCount

	col1 := width / 4
	col2 := width / 4
	col3 := width / 4

	sb.WriteString(fmt.Sprintf(" %-*s %-*s %-*s %s\n",
		col1, bc(white, "TRUST"),
		col2, bc(white, "PIPELINE"),
		col3, bc(white, "DEPTH"),
		bc(white, "EVIDENCE")))

	sb.WriteString(fmt.Sprintf(" %s %2d %-*s backlog:    %-*d %-6s %d %-*s %d total\n",
		c(green, "●"), g, col1-6, "green",
		col2-14, len(d.BacklogProblems),
		depthBar(d.TacticalCount, totalDec, 6), d.TacticalCount, col3-10, "tactical",
		d.EvidenceTotal))

	sb.WriteString(fmt.Sprintf(" %s %2d %-*s exploring:  %-*d %-6s %d %-*s %s\n",
		c(yellow, "●"), a, col1-6, "amber",
		col2-14, len(d.InProgressProblems),
		depthBar(d.StandardCount, totalDec, 6), d.StandardCount, col3-10, "standard",
		expiredLabel(d.EvidenceExpired)))

	sb.WriteString(fmt.Sprintf(" %s %2d %-*s addressed:  %-*d %-6s %d %-*s avg: %dd\n",
		c(red, "●"), r, col1-6, "red",
		col2-14, d.AddressedCount,
		depthBar(d.DeepCount, totalDec, 6), d.DeepCount, col3-10, "deep",
		d.EvidenceAvgAge))

	if bare > 0 {
		sb.WriteString(fmt.Sprintf(" %s %2d bare\n", c(dim, "○"), bare))
	}

	sb.WriteString(fmt.Sprintf("\n %s: %d decisions, %d problems, %d notes\n",
		c(dim, "Totals"), totalDec, totalProb, len(d.RecentNotes)))
	sb.WriteString("\n")

	// Expiring soon
	if len(d.ExpiringSoon) > 0 {
		sb.WriteString(bc(white, " EXPIRING SOON") + "\n")
		for _, item := range d.ExpiringSoon {
			icon := c(yellow, "⚠")
			if item.ExpiresIn <= 7 { icon = c(red, "⚠") }
			titleW := width - 40
			if titleW < 20 { titleW = 20 }
			sb.WriteString(fmt.Sprintf(" %s  %-24s %-*s  %s\n",
				icon, item.ID, titleW, trunc(item.Title, titleW),
				c(dim, fmt.Sprintf("in %dd", item.ExpiresIn))))
		}
		sb.WriteString("\n")
	}

	// Stale items
	if len(d.StaleItems) > 0 {
		sb.WriteString(bc(white, fmt.Sprintf(" STALE ITEMS (%d)", len(d.StaleItems))) + "\n")
		shown := d.StaleItems
		if len(shown) > 5 { shown = shown[:5] }
		for _, item := range shown {
			icon := c(yellow, "⚠")
			if item.DaysStale > 30 { icon = c(red, "⚠") }
			reasonW := width - 46
			if reasonW < 10 { reasonW = 10 }
			sb.WriteString(fmt.Sprintf(" %s  %-24s  %s  %s\n",
				icon, item.ID,
				c(dim, fmt.Sprintf("%3dd", item.DaysStale)),
				c(dim, trunc(item.Reason, reasonW))))
		}
		if len(d.StaleItems) > 5 {
			sb.WriteString(c(dim, fmt.Sprintf("    ... and %d more\n", len(d.StaleItems)-5)))
		}
		sb.WriteString("\n")
	}

	// Context groups + Activity side by side
	if len(d.ContextGroups) > 0 || len(d.RecentActivity) > 0 {
		half := width / 2

		// Build left column (contexts)
		var leftLines []string
		if len(d.ContextGroups) > 0 {
			leftLines = append(leftLines, bc(white, "BY CONTEXT"))
			groups := sortedContextGroups(d.ContextGroups)
			maxC := 0
			for _, g := range groups { if g.count > maxC { maxC = g.count } }
			for _, g := range groups {
				barW := g.count * 12 / max(maxC, 1)
				if barW < 1 { barW = 1 }
				leftLines = append(leftLines, fmt.Sprintf("  %-16s %s %d", trunc(g.name, 16), c(cyan, strings.Repeat("█", barW)), g.count))
			}
		}

		// Build right column (activity)
		var rightLines []string
		if len(d.RecentActivity) > 0 {
			rightLines = append(rightLines, bc(white, "ACTIVITY (7d)"))
			kindCounts := map[string]int{}
			for _, a := range d.RecentActivity { kindCounts[a.Kind]++ }
			for _, kind := range []string{"DecisionRecord", "ProblemCard", "SolutionPortfolio", "Note"} {
				if cnt, ok := kindCounts[kind]; ok {
					label := strings.TrimSuffix(strings.TrimSuffix(kind, "Record"), "Card")
					rightLines = append(rightLines, fmt.Sprintf("  %3d %s", cnt, strings.ToLower(label)))
				}
			}
		}

		// Merge columns
		maxLines := len(leftLines)
		if len(rightLines) > maxLines { maxLines = len(rightLines) }
		for i := 0; i < maxLines; i++ {
			left := ""
			if i < len(leftLines) { left = leftLines[i] }
			right := ""
			if i < len(rightLines) { right = rightLines[i] }
			sb.WriteString(fmt.Sprintf(" %-*s  %s\n", half-2, left, right))
		}
		sb.WriteString("\n")
	}

	// Footer
	if d.CriticalCount > 0 {
		sb.WriteString(bc(red, fmt.Sprintf(" CRITICAL: %d issue(s) require attention\n", d.CriticalCount)))
	} else {
		sb.WriteString(c(green, " OK: no critical issues") + "\n")
	}

	return sb.String()
}

// ──────────────────────────────────────────────────────────────
// View: Decisions
// ──────────────────────────────────────────────────────────────

func BoardDecisions(d *ui.BoardData) string { return BoardDecisionsW(d, 0) }

func BoardDecisionsW(d *ui.BoardData, width int) string {
	if width <= 0 { width = 100 }
	var sb strings.Builder

	sb.WriteString(bc(cyan, "DECISIONS"))
	sb.WriteString(fmt.Sprintf("  %d shipped, %d pending\n", d.ShippedCount, d.PendingCount))
	sb.WriteString(sep(width))

	if len(d.Decisions) == 0 {
		sb.WriteString(c(dim, "  No decisions yet. Use /h-frame to start.\n"))
		return sb.String()
	}

	// Column widths: St(2) + R_eff(6) + Drift(7) + Context(ctxW) + Title(fill) + Valid(12)
	// Fixed columns take: 2 + 1 + 6 + 2 + 7 + 2 + ctxW + 2 + 12 = 34 + ctxW
	validW := 12
	ctxW := 16
	fixedW := 34 + ctxW + validW
	titleW := width - fixedW
	if titleW < 20 { titleW = 20 }

	sb.WriteString(fmt.Sprintf(" %s %s  %s  %s  %s %*s\n",
		c(dim, "St"), c(dim, pad("R_eff", 6)), c(dim, pad("Drift", 7)),
		c(dim, pad("Context", ctxW)), c(dim, pad("Title", titleW)),
		validW, c(dim, "Valid")))
	sb.WriteString(sep(width))

	for _, dec := range d.Decisions {
		shipped := d.DecisionShipped[dec.Meta.ID]
		rEff := d.DecisionREff[dec.Meta.ID]
		drift := d.DecisionDrift[dec.Meta.ID]

		stIcon := c(yellow, "⏳")
		if shipped { stIcon = c(green, "✓ ") }

		rEffP := "  —  "
		rEffCol := dim
		if rEff > 0 {
			rEffP = fmt.Sprintf(" %.2f", rEff)
			rEffCol = reffC(rEff)
		}

		driftP := "no bl"
		driftCol := dim
		switch drift {
		case "clean": driftP = "clean"; driftCol = green
		case "drift": driftP = "DRIFT"; driftCol = red
		}

		ctx := dec.Meta.Context
		if ctx == "" { ctx = "—" }

		validPlain := "—"
		if dec.Meta.ValidUntil != "" {
			validPlain = trunc(dec.Meta.ValidUntil, validW)
		}

		sb.WriteString(fmt.Sprintf(" %s %s  %s  %s  %s %s\n",
			stIcon,
			c(rEffCol, pad(rEffP, 6)),
			c(driftCol, pad(driftP, 7)),
			pad(trunc(ctx, ctxW), ctxW),
			pad(trunc(dec.Meta.Title, titleW), titleW),
			c(dim, fmt.Sprintf("%*s", validW, validPlain))))
	}

	return sb.String()
}

// ──────────────────────────────────────────────────────────────
// View: Problems
// ──────────────────────────────────────────────────────────────

func BoardProblems(d *ui.BoardData) string { return BoardProblemsW(d, 0) }

func BoardProblemsW(d *ui.BoardData, width int) string {
	if width <= 0 { width = 100 }
	var sb strings.Builder

	total := len(d.BacklogProblems) + len(d.InProgressProblems) + d.AddressedCount
	sb.WriteString(bc(cyan, "PROBLEMS"))
	sb.WriteString(fmt.Sprintf("  %d total  ", total))
	sb.WriteString(c(cyan, fmt.Sprintf("◐ %d exploring", len(d.InProgressProblems))))
	sb.WriteString("  ")
	sb.WriteString(c(yellow, fmt.Sprintf("● %d backlog", len(d.BacklogProblems))))
	sb.WriteString("  ")
	sb.WriteString(c(green, fmt.Sprintf("✓ %d addressed", d.AddressedCount)))
	sb.WriteString("\n" + sep(width))

	if len(d.BacklogProblems) == 0 && len(d.InProgressProblems) == 0 {
		sb.WriteString(c(dim, "  No active problems. Use /h-frame to frame one.\n"))
		return sb.String()
	}

	// Columns: status(9) + gap(2) + ID(22) + gap(2) + title(fill) + gap(2) + mode(8)
	modeW := 8
	fixedW := 9 + 2 + 22 + 2 + 2 + modeW
	titleW := width - fixedW
	if titleW < 20 { titleW = 20 }

	sb.WriteString(fmt.Sprintf(" %s  %s  %s  %s\n",
		c(dim, pad("Status", 9)), c(dim, pad("ID", 22)),
		c(dim, pad("Title", titleW)), c(dim, pad("Mode", modeW))))
	sb.WriteString(sep(width))

	for _, p := range d.InProgressProblems {
		mode := trunc(string(p.Meta.Mode), modeW)
		sb.WriteString(fmt.Sprintf(" %s  %s  %s  %s\n",
			c(cyan, pad("exploring", 9)),
			pad(p.Meta.ID, 22),
			pad(trunc(p.Meta.Title, titleW), titleW),
			c(dim, pad(mode, modeW))))
	}
	for _, p := range d.BacklogProblems {
		mode := trunc(string(p.Meta.Mode), modeW)
		sb.WriteString(fmt.Sprintf(" %s  %s  %s  %s\n",
			c(yellow, pad("backlog", 9)),
			pad(p.Meta.ID, 22),
			pad(trunc(p.Meta.Title, titleW), titleW),
			c(dim, pad(mode, modeW))))
	}
	if d.AddressedCount > 0 {
		sb.WriteString(fmt.Sprintf("\n %s\n", c(green, fmt.Sprintf(" %d problems addressed (have linked decisions)", d.AddressedCount))))
	}

	return sb.String()
}

// ──────────────────────────────────────────────────────────────
// View: Coverage
// ──────────────────────────────────────────────────────────────

func BoardCoverage(d *ui.BoardData) string { return BoardCoverageW(d, 0) }

func BoardCoverageW(d *ui.BoardData, width int) string {
	if width <= 0 { width = 100 }
	var sb strings.Builder

	sb.WriteString(bc(cyan, "MODULE COVERAGE"))

	if d.CoverageReport == nil || d.CoverageReport.TotalModules == 0 {
		sb.WriteString("\n" + c(dim, "  No module scan yet. Run 'haft scan' first.\n"))
		return sb.String()
	}

	cr := d.CoverageReport
	governed := cr.CoveredCount + cr.PartialCount
	pct := governed * 100 / cr.TotalModules
	barW := width - 30
	if barW < 20 { barW = 20 }

	sb.WriteString(fmt.Sprintf("  %d%% (%d/%d)  %s covered  %s partial  %s blind\n",
		pct, governed, cr.TotalModules,
		c(green, fmt.Sprintf("%d", cr.CoveredCount)),
		c(yellow, fmt.Sprintf("%d", cr.PartialCount)),
		c(red, fmt.Sprintf("%d", cr.BlindCount))))
	sb.WriteString(" " + bar(governed, cr.TotalModules, barW) + "\n")
	sb.WriteString(sep(width))

	pathW := width - 30
	if pathW < 20 { pathW = 20 }

	sb.WriteString(fmt.Sprintf(" %s  %s  %s  %s\n",
		c(dim, "  "),
		c(dim, pad("Module", pathW)),
		c(dim, pad("Lang", 6)),
		c(dim, "Decisions")))
	sb.WriteString(sep(width))

	for _, m := range cr.Modules {
		icon := c(green, "✓")
		if m.DecisionCount == 0 {
			icon = c(red, "✗")
		} else if m.Status == codebase.CoveragePartial {
			icon = c(yellow, "◐")
		}
		cnt := fmt.Sprintf("%d", m.DecisionCount)
		if m.DecisionCount == 0 {
			cnt = c(red, "BLIND")
		}
		sb.WriteString(fmt.Sprintf(" %s  %s  %s  %s\n",
			icon,
			pad(trunc(m.Module.Path, pathW), pathW),
			pad(m.Module.Lang, 6),
			cnt))
	}

	return sb.String()
}

// ──────────────────────────────────────────────────────────────
// View: Evidence
// ──────────────────────────────────────────────────────────────

func BoardEvidence(d *ui.BoardData) string { return BoardEvidenceW(d, 0) }

func BoardEvidenceW(d *ui.BoardData, width int) string {
	if width <= 0 { width = 100 }
	var sb strings.Builder

	sb.WriteString(bc(cyan, "EVIDENCE & REFRESH") + "\n" + sep(width) + "\n")

	// Evidence + Trust side by side
	half := width / 2

	g, a, r, bare := trustCounts(d)
	total := g + a + r + bare

	var leftLines, rightLines []string

	leftLines = append(leftLines, bc(white, "EVIDENCE HEALTH"))
	leftLines = append(leftLines, fmt.Sprintf(" %d total  |  %d active  |  %s",
		d.EvidenceTotal, d.EvidenceTotal-d.EvidenceExpired, expiredLabel(d.EvidenceExpired)))
	leftLines = append(leftLines, fmt.Sprintf(" avg age: %dd  |  oldest: %dd", d.EvidenceAvgAge, d.EvidenceOldest))

	rightLines = append(rightLines, bc(white, "TRUST DISTRIBUTION"))
	rightLines = append(rightLines, fmt.Sprintf(" %s %d green  %s %d amber  %s %d red  %s %d bare",
		c(green, "●"), g, c(yellow, "●"), a, c(red, "●"), r, c(dim, "○"), bare))
	if total > 0 {
		bw := half - 4
		rightLines = append(rightLines, " "+
			c(green, strings.Repeat("█", g*bw/total))+
			c(yellow, strings.Repeat("█", a*bw/total))+
			c(red, strings.Repeat("█", r*bw/total))+
			c(dim, strings.Repeat("░", bare*bw/total)))
	}

	maxLines := len(leftLines)
	if len(rightLines) > maxLines { maxLines = len(rightLines) }
	for i := 0; i < maxLines; i++ {
		l := ""; if i < len(leftLines) { l = leftLines[i] }
		ri := ""; if i < len(rightLines) { ri = rightLines[i] }
		sb.WriteString(fmt.Sprintf(" %-*s  %s\n", half-2, l, ri))
	}
	sb.WriteString("\n")

	// Refresh queue
	if len(d.StaleItems) > 0 {
		sb.WriteString(bc(white, fmt.Sprintf(" REFRESH QUEUE (%d items)", len(d.StaleItems))) + "\n")
		sb.WriteString(sep(width))

		reasonW := width - 40
		if reasonW < 20 { reasonW = 20 }

		for _, item := range d.StaleItems {
			icon := c(yellow, "⚠")
			if item.DaysStale > 30 { icon = c(red, "⚠") }
			sb.WriteString(fmt.Sprintf(" %s  %-24s  %s  %s\n",
				icon, item.ID,
				c(dim, fmt.Sprintf("%3dd", item.DaysStale)),
				c(dim, trunc(item.Reason, reasonW))))
		}
	} else {
		sb.WriteString(c(green, " No stale items — all evidence fresh.") + "\n")
	}

	return sb.String()
}

// ──────────────────────────────────────────────────────────────
// Combined views
// ──────────────────────────────────────────────────────────────

func BoardFull(d *ui.BoardData) string { return BoardFullW(d, 0) }

func BoardFullW(d *ui.BoardData, width int) string {
	return BoardOverviewW(d, width) + "\n" +
		BoardDecisionsW(d, width) + "\n" +
		BoardProblemsW(d, width) + "\n" +
		BoardCoverageW(d, width) + "\n" +
		BoardEvidenceW(d, width)
}

func BoardCheck(d *ui.BoardData) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Haft Health: %s\n", d.ProjectName))
	sb.WriteString(fmt.Sprintf("  Decisions: %d shipped, %d pending\n", d.ShippedCount, d.PendingCount))
	sb.WriteString(fmt.Sprintf("  Problems:  %d backlog, %d addressed\n", len(d.BacklogProblems), d.AddressedCount))
	sb.WriteString(fmt.Sprintf("  Stale:     %d items\n", len(d.StaleItems)))
	if d.CoverageReport != nil && d.CoverageReport.TotalModules > 0 {
		cr := d.CoverageReport
		pct := (cr.CoveredCount + cr.PartialCount) * 100 / cr.TotalModules
		sb.WriteString(fmt.Sprintf("  Coverage:  %d%% (%d/%d modules)\n", pct, cr.CoveredCount+cr.PartialCount, cr.TotalModules))
	}
	g, a, r, _ := trustCounts(d)
	if g+a+r > 0 {
		sb.WriteString(fmt.Sprintf("  Trust:     %d green, %d amber, %d red\n", g, a, r))
	}
	if d.CriticalCount > 0 {
		sb.WriteString(fmt.Sprintf("\n  CRITICAL: %d issue(s) require attention\n", d.CriticalCount))
	} else {
		sb.WriteString("\n  OK: no critical issues\n")
	}
	return sb.String()
}

// ──────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────

func trustCounts(d *ui.BoardData) (green, amber, red, bare int) {
	for _, dec := range d.Decisions {
		r := d.DecisionREff[dec.Meta.ID]
		switch {
		case r == 0: bare++
		case r >= 0.7: green++
		case r >= 0.3: amber++
		default: red++
		}
	}
	return
}

func depthBar(count, total, width int) string {
	if total == 0 { return strings.Repeat("░", width) }
	w := count * width / total
	if w < 1 && count > 0 { w = 1 }
	return c(cyan, strings.Repeat("█", w)) + strings.Repeat(" ", width-w)
}

func expiredLabel(n int) string {
	if n == 0 { return c(green, "0 expired") }
	return c(red, fmt.Sprintf("%d expired ⚠", n))
}

type contextGroup struct { name string; count int }

func sortedContextGroups(groups map[string]int) []contextGroup {
	result := make([]contextGroup, 0, len(groups))
	for name, count := range groups { result = append(result, contextGroup{name, count}) }
	sort.Slice(result, func(i, j int) bool { return result[i].count > result[j].count })
	return result
}
