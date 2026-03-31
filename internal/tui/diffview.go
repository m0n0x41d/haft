package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/aymanbagabas/go-udiff"
	"github.com/charmbracelet/x/ansi"
)

// ---------------------------------------------------------------------------
// L1: Diff renderer — pure function, takes old/new content → colored string.
// No I/O, no state. Used by both permission modal and inline tool output.
// ---------------------------------------------------------------------------

// DiffStyle controls colors for diff rendering.
type DiffStyle struct {
	AddLine    lipgloss.Style // green background for added lines
	DelLine    lipgloss.Style // red background for removed lines
	AddSymbol  lipgloss.Style // + symbol
	DelSymbol  lipgloss.Style // - symbol
	HunkHeader lipgloss.Style // @@ line
	Context    lipgloss.Style // unchanged lines
	LineNum    lipgloss.Style // line numbers
}

// DefaultDiffStyle returns the standard dark-theme diff colors.
func DefaultDiffStyle() DiffStyle {
	return DiffStyle{
		AddLine:    lipgloss.NewStyle().Foreground(lipgloss.Color("114")).Background(lipgloss.Color("22")),
		DelLine:    lipgloss.NewStyle().Foreground(lipgloss.Color("210")).Background(lipgloss.Color("52")),
		AddSymbol:  lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Background(lipgloss.Color("22")).Bold(true),
		DelSymbol:  lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Background(lipgloss.Color("52")).Bold(true),
		HunkHeader: lipgloss.NewStyle().Foreground(lipgloss.Color("75")),
		Context:    lipgloss.NewStyle().Foreground(lipgloss.Color("250")),
		LineNum:    lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
	}
}

// RenderDiff generates a unified diff between old and new content,
// then colorizes it for terminal display.
// Returns the rendered string and counts of additions/removals.
// Pure function — no side effects.
func RenderDiff(fileName, oldContent, newContent string, width int) (string, int, int) {
	if oldContent == newContent {
		return "", 0, 0
	}

	// Generate unified diff
	unified := udiff.Unified("a/"+fileName, "b/"+fileName, oldContent, newContent)
	if unified == "" {
		return "", 0, 0
	}

	style := DefaultDiffStyle()
	lines := strings.Split(strings.TrimRight(unified, "\n"), "\n")

	var rendered []string
	adds, dels := 0, 0

	for _, line := range lines {
		if width > 0 && ansi.StringWidth(line) > width {
			line = string([]rune(line)[:width])
		}

		switch {
		case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---"):
			// File headers — dim
			rendered = append(rendered, style.HunkHeader.Render(line))
		case strings.HasPrefix(line, "@@"):
			// Hunk header
			rendered = append(rendered, style.HunkHeader.Render(line))
		case strings.HasPrefix(line, "+"):
			adds++
			symbol := style.AddSymbol.Render("+")
			content := style.AddLine.Render(line[1:])
			rendered = append(rendered, symbol+content)
		case strings.HasPrefix(line, "-"):
			dels++
			symbol := style.DelSymbol.Render("-")
			content := style.DelLine.Render(line[1:])
			rendered = append(rendered, symbol+content)
		default:
			// Context line
			rendered = append(rendered, style.Context.Render(line))
		}
	}

	return strings.Join(rendered, "\n"), adds, dels
}

// RenderDiffCompact generates a short summary line for inline display.
// Example: "file.go +9 -3"
func RenderDiffCompact(fileName string, adds, dels int) string {
	style := DefaultDiffStyle()
	parts := []string{
		lipgloss.NewStyle().Foreground(lipgloss.Color("247")).Render(fileName),
	}
	if adds > 0 {
		parts = append(parts, style.AddSymbol.Render(fmt.Sprintf("+%d", adds)))
	}
	if dels > 0 {
		parts = append(parts, style.DelSymbol.Render(fmt.Sprintf("-%d", dels)))
	}
	return strings.Join(parts, " ")
}
