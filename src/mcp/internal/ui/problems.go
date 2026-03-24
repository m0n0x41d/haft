package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"

	"github.com/m0n0x41d/quint-code/internal/artifact"
)

// ProblemsView shows backlog and in-progress problems with drill-in.
type ProblemsView struct {
	data   *BoardData
	cursor int
	detail bool
	scroll int
}

func NewProblemsView(data *BoardData) *ProblemsView {
	return &ProblemsView{data: data}
}

func (v *ProblemsView) UpdateData(data *BoardData) { v.data = data }

func (v *ProblemsView) Title() string {
	count := len(v.data.BacklogProblems) + len(v.data.InProgressProblems)
	if count == 0 {
		return "Problems"
	}
	return fmt.Sprintf("Problems (%d)", count)
}

func (v *ProblemsView) HelpKeys() []HelpItem {
	if v.detail {
		return []HelpItem{{"j/k", "scroll"}, {"esc", "back"}}
	}
	if len(v.listItems()) == 0 {
		return nil
	}
	return []HelpItem{{"j/k", "navigate"}, {"enter", "detail"}}
}

func (v *ProblemsView) HandleKey(msg tea.KeyMsg) bool {
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

	items := v.listItems()
	switch msg.String() {
	case "j", "down":
		if v.cursor < len(items)-1 {
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
		if len(items) > 0 {
			v.cursor = len(items) - 1
		}
		return true
	case "enter":
		if len(items) > 0 {
			v.detail = true
		}
		return true
	}
	return false
}

type listEntry struct {
	art    *artifact.Artifact
	status string
}

func (v *ProblemsView) listItems() []listEntry {
	var items []listEntry
	for _, p := range v.data.BacklogProblems {
		items = append(items, listEntry{art: p, status: "backlog"})
	}
	for _, p := range v.data.InProgressProblems {
		items = append(items, listEntry{art: p, status: "exploring"})
	}
	return items
}

func (v *ProblemsView) Render(width, height int, s Styles) string {
	items := v.listItems()

	if len(items) == 0 {
		return "\n  " + s.DimText.Render("No active problems. Clean backlog.") + "\n"
	}

	if v.detail && v.cursor < len(items) {
		return renderArtifactDetail(items[v.cursor].art, width, height, v.scroll, s)
	}

	var b strings.Builder
	b.WriteString("\n")

	for i, item := range items {
		if i >= height-2 {
			b.WriteString(fmt.Sprintf("  %s\n", s.DimText.Render(fmt.Sprintf("... +%d more", len(items)-i))))
			break
		}

		statusStyle := s.Warning
		if item.status == "exploring" {
			statusStyle = s.Subtitle
		}

		line := fmt.Sprintf("  %s  %s  %s",
			statusStyle.Render(fmt.Sprintf("%-10s", item.status)),
			s.DimText.Render(item.art.Meta.ID),
			truncate(item.art.Meta.Title, width-35))

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

// renderArtifactDetail renders a single artifact with glamour markdown.
// Shared by problems and decisions views.
func renderArtifactDetail(a *artifact.Artifact, width, height, scroll int, s Styles) string {
	var b strings.Builder

	// Header with metadata
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  %s  %s\n",
		s.Title.Render(a.Meta.Title),
		s.DimText.Render(a.Meta.ID)))

	meta := []string{}
	if a.Meta.Mode != "" {
		meta = append(meta, fmt.Sprintf("mode: %s", a.Meta.Mode))
	}
	if a.Meta.Context != "" {
		meta = append(meta, fmt.Sprintf("context: %s", a.Meta.Context))
	}
	if a.Meta.ValidUntil != "" && len(a.Meta.ValidUntil) >= 10 {
		meta = append(meta, fmt.Sprintf("valid until: %s", a.Meta.ValidUntil[:10]))
	}
	if len(meta) > 0 {
		b.WriteString(fmt.Sprintf("  %s\n", s.DimText.Render(strings.Join(meta, " · "))))
	}
	b.WriteString("\n")

	// Render body with glamour
	renderWidth := width - 6
	if renderWidth < 40 {
		renderWidth = 40
	}

	renderer, err := glamour.NewTermRenderer(
		glamour.WithEnvironmentConfig(),
		glamour.WithWordWrap(renderWidth),
	)

	body := a.Body
	if err == nil {
		rendered, renderErr := renderer.Render(body)
		if renderErr == nil {
			body = rendered
		}
	}

	// Indent each line
	for _, line := range strings.Split(strings.TrimRight(body, "\n"), "\n") {
		b.WriteString("  " + line + "\n")
	}

	b.WriteString(fmt.Sprintf("\n  %s\n", s.DimText.Render("esc to go back · j/k to scroll")))

	// Apply scroll
	lines := strings.Split(b.String(), "\n")
	if scroll > 0 && scroll < len(lines) {
		lines = lines[scroll:]
	}
	if len(lines) > height {
		lines = lines[:height]
	}
	return strings.Join(lines, "\n")
}
