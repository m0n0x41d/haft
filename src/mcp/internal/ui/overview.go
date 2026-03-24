package ui

import (
	"fmt"
	"image/color"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// OverviewView renders the health dashboard.
type OverviewView struct {
	data *BoardData
}

func NewOverviewView(data *BoardData) *OverviewView {
	return &OverviewView{data: data}
}

func (v *OverviewView) Title() string { return "Overview" }

func (v *OverviewView) UpdateData(data *BoardData) { v.data = data }

func (v *OverviewView) HandleKey(_ tea.KeyMsg) bool { return false }

func (v *OverviewView) HelpKeys() []HelpItem { return nil }

func (v *OverviewView) Render(width, height int, s Styles) string {
	halfW := width/2 - 2

	// Left column
	left := v.renderHealth(s) + "\n" +
		v.renderExpiring(s) + "\n" +
		v.renderContexts(s, halfW) + "\n" +
		v.renderEvidence(s)

	// Right column
	right := v.renderActivity(s) + "\n" +
		v.renderDepth(s, halfW) + "\n" +
		v.renderCoverage(s, halfW)

	leftBlock := lipgloss.NewStyle().Width(halfW).Render(left)
	rightBlock := lipgloss.NewStyle().Width(halfW).Render(right)

	return lipgloss.JoinHorizontal(lipgloss.Top, "  "+leftBlock, "  "+rightBlock)
}

func (v *OverviewView) renderHealth(s Styles) string {
	d := v.data
	var b strings.Builder

	b.WriteString(s.Section.Render("HEALTH"))
	b.WriteString("\n")

	if d.ShippedCount > 0 {
		b.WriteString(fmt.Sprintf("  %s %d shipped\n", s.OK.Render("✓"), d.ShippedCount))
	}
	if d.PendingCount > 0 {
		b.WriteString(fmt.Sprintf("  %s %d pending implementation\n", s.Warning.Render("⏳"), d.PendingCount))
	}
	if len(d.StaleItems) > 0 {
		b.WriteString(fmt.Sprintf("  %s %d stale\n", s.Error.Render("✗"), len(d.StaleItems)))
	}
	if d.ShippedCount > 0 && d.PendingCount == 0 && len(d.StaleItems) == 0 {
		b.WriteString(fmt.Sprintf("  %s\n", s.OK.Render("No issues")))
	}

	// Problems pipeline
	b.WriteString(fmt.Sprintf("\n  %s %d backlog",
		s.Label.Render("Problems:"), len(d.BacklogProblems)))
	if len(d.InProgressProblems) > 0 {
		b.WriteString(fmt.Sprintf(", %d exploring", len(d.InProgressProblems)))
	}
	b.WriteString(fmt.Sprintf(", %d addressed\n", d.AddressedCount))

	return b.String()
}

func (v *OverviewView) renderExpiring(s Styles) string {
	d := v.data
	if len(d.ExpiringSoon) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString(s.Section.Render("EXPIRING SOON"))
	b.WriteString("\n")
	for _, e := range d.ExpiringSoon {
		style := s.Warning
		if e.ExpiresIn <= 7 {
			style = s.Error
		}
		b.WriteString(fmt.Sprintf("  %s  %s\n",
			style.Render(fmt.Sprintf("%dd", e.ExpiresIn)),
			s.DimText.Render(truncate(e.Title, 40))))
	}
	return b.String()
}

func (v *OverviewView) renderContexts(s Styles, width int) string {
	d := v.data
	if len(d.ContextGroups) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString(s.Section.Render("BY CONTEXT"))
	b.WriteString("\n")

	// Sort by count descending
	type kv struct {
		K string
		V int
	}
	var sorted []kv
	for k, v := range d.ContextGroups {
		sorted = append(sorted, kv{k, v})
	}
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].V != sorted[j].V {
			return sorted[i].V > sorted[j].V
		}
		return sorted[i].K < sorted[j].K
	})

	for _, kv := range sorted {
		b.WriteString(fmt.Sprintf("  %s %s\n",
			s.Value.Render(fmt.Sprintf("%2d", kv.V)),
			s.Label.Render(kv.K)))
	}
	return b.String()
}

func (v *OverviewView) renderEvidence(s Styles) string {
	d := v.data
	if d.EvidenceTotal == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString(s.Section.Render("EVIDENCE"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  %s items, avg age %s, oldest %s\n",
		s.Value.Render(fmt.Sprintf("%d", d.EvidenceTotal)),
		s.Label.Render(fmt.Sprintf("%dd", d.EvidenceAvgAge)),
		s.Label.Render(fmt.Sprintf("%dd", d.EvidenceOldest))))
	if d.EvidenceExpired > 0 {
		b.WriteString(fmt.Sprintf("  %s\n", s.Error.Render(fmt.Sprintf("%d expired", d.EvidenceExpired))))
	}
	return b.String()
}

func (v *OverviewView) renderActivity(s Styles) string {
	d := v.data

	var b strings.Builder
	b.WriteString(s.Section.Render("ACTIVITY (7 days)"))
	b.WriteString("\n")

	if len(d.RecentActivity) == 0 {
		b.WriteString(fmt.Sprintf("  %s\n", s.DimText.Render("No recent activity")))
		return b.String()
	}

	// Count by kind
	counts := make(map[string]int)
	for _, a := range d.RecentActivity {
		counts[a.Kind]++
	}
	for kind, count := range counts {
		b.WriteString(fmt.Sprintf("  %s %s\n",
			s.Value.Render(fmt.Sprintf("%d", count)),
			s.Label.Render(kind)))
	}
	return b.String()
}

func (v *OverviewView) renderDepth(s Styles, width int) string {
	d := v.data

	var b strings.Builder
	b.WriteString(s.Section.Render("DEPTH"))
	b.WriteString("\n")

	total := d.TacticalCount + d.StandardCount + d.DeepCount
	if total == 0 {
		b.WriteString(fmt.Sprintf("  %s\n", s.DimText.Render("No decisions")))
		return b.String()
	}

	barW := width - 20
	if barW < 10 {
		barW = 10
	}

	b.WriteString(fmt.Sprintf("  tactical  %3d %s\n", d.TacticalCount, renderBar(d.TacticalCount, total, barW, s.OK, s.Theme.Dim)))
	b.WriteString(fmt.Sprintf("  standard  %3d %s\n", d.StandardCount, renderBar(d.StandardCount, total, barW, s.Subtitle, s.Theme.Dim)))
	b.WriteString(fmt.Sprintf("  deep      %3d %s\n", d.DeepCount, renderBar(d.DeepCount, total, barW, s.Warning, s.Theme.Dim)))
	return b.String()
}

func (v *OverviewView) renderCoverage(s Styles, width int) string {
	d := v.data
	if d.CoverageReport == nil {
		return ""
	}

	cr := d.CoverageReport
	var b strings.Builder
	b.WriteString(s.Section.Render("MODULE COVERAGE"))
	b.WriteString("\n")

	pct := 0
	if cr.TotalModules > 0 {
		pct = (cr.CoveredCount + cr.PartialCount) * 100 / cr.TotalModules
	}

	barW := width - 15
	if barW < 10 {
		barW = 10
	}
	filled := barW * pct / 100
	bar := s.OK.Render(strings.Repeat("█", filled)) +
		s.DimText.Render(strings.Repeat("░", barW-filled))
	b.WriteString(fmt.Sprintf("  %s %s %d%%\n", bar,
		s.Label.Render(fmt.Sprintf("%d/%d", cr.CoveredCount+cr.PartialCount, cr.TotalModules)),
		pct))

	return b.String()
}

func renderBar(value, total, width int, style lipgloss.Style, dimColor color.Color) string {
	if total == 0 {
		return ""
	}
	filled := width * value / total
	if filled == 0 && value > 0 {
		filled = 1
	}
	return style.Render(strings.Repeat("█", filled)) +
		lipgloss.NewStyle().Foreground(dimColor).Render(strings.Repeat("░", width-filled))
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}
