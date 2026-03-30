package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// ---------------------------------------------------------------------------
// Command palette: overlay with slash command picker + fuzzy filter.
// Pure data — no side effects. Model routes keys, View renders overlay.
// ---------------------------------------------------------------------------

// SlashCmd describes one available slash command.
type SlashCmd struct {
	Name   string // without leading "/"
	Desc   string
	Hotkey string // optional hotkey hint (e.g., "ctrl+m")
}

// slashCommands is the command registry. Add new commands here.
var slashCommands = []SlashCmd{
	{Name: "model", Desc: "Switch model/provider", Hotkey: "ctrl+m"},
	{Name: "resume", Desc: "Resume a previous session"},
	{Name: "compact", Desc: "Compact context window"},
	{Name: "help", Desc: "Show available commands"},
	{Name: "frame", Desc: "Frame an engineering problem"},
	{Name: "explore", Desc: "Explore solution variants"},
	{Name: "decide", Desc: "Finalize a decision"},
	{Name: "measure", Desc: "Measure implementation results"},
	{Name: "status", Desc: "Dashboard of decisions and problems"},
	{Name: "reason", Desc: "Think before building"},
	{Name: "note", Desc: "Record a micro-decision"},
	{Name: "search", Desc: "Search past decisions and notes"},
	{Name: "compare", Desc: "Compare solution variants"},
	{Name: "problems", Desc: "List active engineering problems"},
	{Name: "refresh", Desc: "Manage artifact lifecycle"},
	{Name: "char", Desc: "Define comparison dimensions"},
	{Name: "setup", Desc: "Full provider setup"},
}

// CommandPalette holds the palette state.
type CommandPalette struct {
	filtered []SlashCmd
	selected int
	filter   string // current filter text (without "/")
}

// Update recomputes filtered list from the current input text.
// Pass the full input value (e.g., "/res").
func (p *CommandPalette) Update(inputValue string) {
	if !strings.HasPrefix(inputValue, "/") {
		p.filtered = nil
		p.selected = 0
		p.filter = ""
		return
	}

	p.filter = strings.ToLower(inputValue[1:])
	p.filtered = p.filtered[:0]

	for _, cmd := range slashCommands {
		if p.filter == "" || strings.Contains(cmd.Name, p.filter) {
			p.filtered = append(p.filtered, cmd)
		}
	}

	// Clamp selection
	if p.selected >= len(p.filtered) {
		p.selected = max(0, len(p.filtered)-1)
	}
}

// Visible reports whether the palette should be shown.
func (p *CommandPalette) Visible() bool {
	return len(p.filtered) > 0
}

// MoveUp moves selection up (or wraps to bottom).
func (p *CommandPalette) MoveUp() {
	if len(p.filtered) == 0 {
		return
	}
	p.selected--
	if p.selected < 0 {
		p.selected = len(p.filtered) - 1
	}
}

// MoveDown moves selection down (or wraps to top).
func (p *CommandPalette) MoveDown() {
	if len(p.filtered) == 0 {
		return
	}
	p.selected++
	if p.selected >= len(p.filtered) {
		p.selected = 0
	}
}

// Selected returns the currently selected command name (with "/").
// Returns "" if nothing is selected.
func (p *CommandPalette) Selected() string {
	if p.selected < 0 || p.selected >= len(p.filtered) {
		return ""
	}
	return "/" + p.filtered[p.selected].Name
}

// Render draws the palette as a bordered box.
func (p *CommandPalette) Render(width int, styles Styles) string {
	if !p.Visible() {
		return ""
	}

	const maxVisible = 10
	paletteWidth := min(width-4, 60)

	nameWidth := 0
	for _, cmd := range p.filtered {
		if len(cmd.Name)+1 > nameWidth {
			nameWidth = len(cmd.Name) + 1
		}
	}

	descWidth := paletteWidth - nameWidth - 6 // padding + border
	if descWidth < 10 {
		descWidth = 10
	}

	var lines []string
	visible := p.filtered
	if len(visible) > maxVisible {
		visible = visible[:maxVisible]
	}

	for i, cmd := range visible {
		name := "/" + cmd.Name
		desc := cmd.Desc
		if r := []rune(desc); len(r) > descWidth {
			desc = string(r[:descWidth-1]) + "…"
		}

		hotkeyHint := ""
		if cmd.Hotkey != "" {
			hotkeyHint = "  " + styles.Dim.Render(cmd.Hotkey)
		}

		if i == p.selected {
			selectedDesc := desc
			if cmd.Hotkey != "" {
				selectedDesc += "  " + cmd.Hotkey
			}
			line := lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("0")).
				Background(lipgloss.Color("39")).
				Width(paletteWidth - 4).
				Render(padRight("/"+cmd.Name, nameWidth+1) + selectedDesc)
			lines = append(lines, line)
		} else {
			line := padRight(name, nameWidth+1) + styles.Dim.Render(desc) + hotkeyHint
			lines = append(lines, line)
		}
	}

	if len(p.filtered) > maxVisible {
		more := styles.Dim.Render(
			padRight("", nameWidth+1) + "… and more")
		lines = append(lines, more)
	}

	content := strings.Join(lines, "\n")

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("39")).
		Padding(0, 1).
		Width(paletteWidth).
		Render(content)

	return box
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}
