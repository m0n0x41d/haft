package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// DecisionsView shows all decisions with R_eff, status, and drill-in.
type DecisionsView struct {
	data   *BoardData
	cursor int
	detail bool
	scroll int
}

func NewDecisionsView(data *BoardData) *DecisionsView {
	return &DecisionsView{data: data}
}

func (v *DecisionsView) UpdateData(data *BoardData) { v.data = data }

func (v *DecisionsView) Title() string {
	return fmt.Sprintf("Decisions (%d)", len(v.data.Decisions))
}

func (v *DecisionsView) HelpKeys() []HelpItem {
	if v.detail {
		return []HelpItem{{"j/k", "scroll"}, {"esc", "back"}}
	}
	if len(v.data.Decisions) == 0 {
		return nil
	}
	return []HelpItem{{"j/k", "navigate"}, {"enter", "detail"}}
}

func (v *DecisionsView) HandleKey(msg tea.KeyMsg) bool {
	if v.detail {
		switch msg.String() {
		case "esc", "backspace":
			v.detail = false
			v.scroll = 0
			return true
		case "j", "down":
			v.scroll++
			return true
		case "k", "up":
			if v.scroll > 0 {
				v.scroll--
			}
			return true
		}
		return false
	}

	switch msg.String() {
	case "j", "down":
		if v.cursor < len(v.data.Decisions)-1 {
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
		if len(v.data.Decisions) > 0 {
			v.cursor = len(v.data.Decisions) - 1
		}
		return true
	case "enter":
		if len(v.data.Decisions) > 0 {
			v.detail = true
		}
		return true
	}
	return false
}

// padRight pads a styled string to a fixed visible width using spaces.
// Unlike fmt %-*s, this correctly handles ANSI escape codes.
func padRight(s string, width int) string {
	visible := ansi.StringWidth(s)
	if visible >= width {
		return s
	}
	return s + strings.Repeat(" ", width-visible)
}

func (v *DecisionsView) Render(width, height int, s Styles) string {
	if len(v.data.Decisions) == 0 {
		return "\n  " + s.DimText.Render("No decisions yet. Use /q-decide to create one.") + "\n"
	}

	if v.detail && v.cursor < len(v.data.Decisions) {
		return renderArtifactDetail(v.data.Decisions[v.cursor], width, height, v.scroll, s)
	}

	// Fixed column widths
	const (
		colStatus = 3
		colREff   = 6
		colDrift  = 8
	)
	colTitle := width - colStatus - colREff - colDrift - 8
	if colTitle < 20 {
		colTitle = 20
	}

	var b strings.Builder
	b.WriteString("\n")

	// Header
	hdr := lipgloss.NewStyle().Foreground(s.Theme.Dim).Bold(true)
	b.WriteString(fmt.Sprintf("  %s%s%s%s\n",
		padRight("", colStatus+1),
		padRight(hdr.Render("R_eff"), colREff+1),
		padRight(hdr.Render("Drift"), colDrift+1),
		hdr.Render("Title")))

	sep := lipgloss.NewStyle().Foreground(s.Theme.Border)
	b.WriteString(fmt.Sprintf("  %s\n", sep.Render(strings.Repeat("─", width-4))))

	for i, d := range v.data.Decisions {
		if i >= height-4 {
			b.WriteString(fmt.Sprintf("  %s\n",
				s.DimText.Render(fmt.Sprintf("... +%d more", len(v.data.Decisions)-i))))
			break
		}

		// Build each column as a styled string, then pad to fixed width
		reff := v.data.DecisionREff[d.Meta.ID]

		statusStr := s.Warning.Render("⏳")
		if reff > 0 {
			statusStr = s.OK.Render("✓")
		}

		reffStr := s.DimText.Render("—")
		if reff > 0 {
			reffStr = s.REffStyle(reff).Render(fmt.Sprintf("%.2f", reff))
		}

		drift := v.data.DecisionDrift[d.Meta.ID]
		driftStr := s.DimText.Render("—")
		switch drift {
		case "clean":
			driftStr = s.OK.Render("clean")
		case "drift":
			driftStr = s.Error.Render("DRIFT")
		case "no baseline":
			driftStr = s.Warning.Render("no bl")
		}

		title := truncate(d.Meta.Title, colTitle)

		line := fmt.Sprintf("  %s %s %s %s",
			padRight(statusStr, colStatus),
			padRight(reffStr, colREff),
			padRight(driftStr, colDrift),
			title)

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
