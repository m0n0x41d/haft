package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// ModulesView shows module coverage tree.
type ModulesView struct {
	data   *BoardData
	cursor int
}

func NewModulesView(data *BoardData) *ModulesView {
	return &ModulesView{data: data}
}

func (v *ModulesView) UpdateData(data *BoardData) { v.data = data }

func (v *ModulesView) Title() string {
	if v.data.CoverageReport == nil {
		return "Modules"
	}
	return fmt.Sprintf("Modules (%d)", v.data.CoverageReport.TotalModules)
}

func (v *ModulesView) HelpKeys() []HelpItem {
	if v.data.CoverageReport == nil || len(v.data.CoverageReport.Modules) == 0 {
		return nil
	}
	return []HelpItem{{"j/k", "navigate"}}
}

func (v *ModulesView) HandleKey(msg tea.KeyMsg) bool {
	if v.data.CoverageReport == nil {
		return false
	}
	count := len(v.data.CoverageReport.Modules)

	switch msg.String() {
	case "j", "down":
		if v.cursor < count-1 {
			v.cursor++
		}
		return true
	case "k", "up":
		if v.cursor > 0 {
			v.cursor--
		}
		return true
	case "g":
		v.cursor = 0
		return true
	case "G":
		if count > 0 {
			v.cursor = count - 1
		}
		return true
	}
	return false
}

func (v *ModulesView) Render(width, height int, s Styles) string {
	cr := v.data.CoverageReport
	if cr == nil {
		return "\n  " + s.DimText.Render("No modules scanned. Run /q-refresh or /q-status first.") + "\n"
	}

	// Column widths
	const (
		colIcon = 3
		colLang = 5
		colDec  = 10
	)
	colPath := width - colIcon - colLang - colDec - 10
	if colPath < 20 {
		colPath = 20
	}

	var b strings.Builder
	b.WriteString("\n")

	// Coverage summary bar
	pct := 0
	if cr.TotalModules > 0 {
		pct = (cr.CoveredCount + cr.PartialCount) * 100 / cr.TotalModules
	}
	barW := width - 25
	if barW < 10 {
		barW = 10
	}
	filled := barW * pct / 100
	bar := s.OK.Render(strings.Repeat("█", filled)) +
		s.DimText.Render(strings.Repeat("░", barW-filled))
	b.WriteString(fmt.Sprintf("  %s  %s  %d%%\n\n",
		bar,
		s.Label.Render(fmt.Sprintf("%d/%d governed", cr.CoveredCount+cr.PartialCount, cr.TotalModules)),
		pct))

	// Header
	hdr := lipgloss.NewStyle().Foreground(s.Theme.Dim).Bold(true)
	b.WriteString(fmt.Sprintf("  %s %s %s %s\n",
		padRight("", colIcon),
		padRight(hdr.Render("Module"), colPath),
		padRight(hdr.Render("Lang"), colLang),
		hdr.Render("Decisions")))

	sep := lipgloss.NewStyle().Foreground(s.Theme.Border)
	b.WriteString(fmt.Sprintf("  %s\n", sep.Render(strings.Repeat("─", width-4))))

	for i, mod := range cr.Modules {
		if i >= height-6 {
			b.WriteString(fmt.Sprintf("  %s\n",
				s.DimText.Render(fmt.Sprintf("... +%d more", len(cr.Modules)-i))))
			break
		}

		icon := s.OK.Render("✓")
		decStr := s.Value.Render(fmt.Sprintf("%d", mod.DecisionCount))
		if mod.DecisionCount == 0 {
			icon = s.Error.Render("✗")
			decStr = s.Error.Render("blind")
		}

		path := mod.Module.Path
		if path == "" {
			path = "(root)"
		}

		line := fmt.Sprintf("  %s %s %s %s",
			padRight(icon, colIcon),
			padRight(truncate(path, colPath), colPath),
			padRight(s.DimText.Render(mod.Module.Lang), colLang),
			decStr)

		if i == v.cursor {
			b.WriteString(s.SelectedItem.Width(width - 2).Render(line))
		} else if i%2 == 0 {
			b.WriteString(s.DimRow.Render(line))
		} else {
			b.WriteString(line)
		}
		b.WriteString("\n")
	}

	return b.String()
}
