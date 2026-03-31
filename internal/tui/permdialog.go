package tui

import (
	"encoding/json"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// ---------------------------------------------------------------------------
// L2: Permission dialog — modal with diff view, scrollable viewport, 3 buttons.
// Replaces the old inline permission box.
// ---------------------------------------------------------------------------

// PermAction is the user's response to a permission prompt.
type PermAction int

const (
	PermNone         PermAction = iota
	PermAllow                          // y: allow this one
	PermAllowSession                   // a: allow all for session
	PermDeny                           // n: deny
)

// PermDialog is a modal overlay for tool permission prompts.
// Shows tool name, description, file diff (for edit/write), and three action buttons.
type PermDialog struct {
	toolName   string
	toolDesc   string
	args       string
	filePath   string
	diffView   string // pre-rendered colored diff
	diffAdds   int
	diffDels   int

	viewport   viewport.Model
	selected   int // 0=allow, 1=allow session, 2=deny
	fullscreen bool
	ready      bool // viewport initialized

	width  int
	height int
}

// NewPermDialog creates a permission dialog for a tool call.
func NewPermDialog(toolName, args string, width, height int) *PermDialog {
	d := &PermDialog{
		toolName: toolName,
		toolDesc: toolDescription(toolName),
		args:     args,
		selected: 0,
		width:    width,
		height:   height,
	}

	// Extract file path and diff for edit/write/multiedit tools
	d.filePath, d.diffView, d.diffAdds, d.diffDels = extractDiffFromArgs(toolName, args, width-8)

	return d
}

// Update handles a key press. Returns the action if user made a choice.
func (d *PermDialog) Update(msg tea.Msg) (PermAction, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		key := msg.Key()
		switch {
		// Direct action keys (y/a/n + 1/2/3)
		case key.Text == "y" || key.Text == "Y" || key.Text == "1":
			return PermAllow, nil
		case key.Text == "a" || key.Text == "A" || key.Text == "2":
			return PermAllowSession, nil
		case key.Text == "n" || key.Text == "N" || key.Text == "3", key.Code == tea.KeyEscape:
			return PermDeny, nil

		// Button navigation
		case key.Code == tea.KeyTab, key.Code == tea.KeyRight:
			d.selected = (d.selected + 1) % 3
		case key.Code == tea.KeyLeft:
			d.selected = (d.selected + 2) % 3
		case key.Code == tea.KeyEnter:
			switch d.selected {
			case 0:
				return PermAllow, nil
			case 1:
				return PermAllowSession, nil
			case 2:
				return PermDeny, nil
			}

		// Fullscreen toggle
		case key.Text == "f" || key.Text == "F":
			d.fullscreen = !d.fullscreen
			d.ready = false // force viewport resize

		// Scroll
		default:
			if d.ready {
				var cmd tea.Cmd
				d.viewport, cmd = d.viewport.Update(msg)
				return PermNone, cmd
			}
		}
	}
	return PermNone, nil
}

// Render draws the modal.
func (d *PermDialog) Render() string {
	// Calculate dialog dimensions
	dialogWidth := min(d.width-4, 90)
	dialogHeight := d.height - 4
	if d.fullscreen || d.diffView != "" {
		dialogWidth = min(d.width-2, 180)
		dialogHeight = d.height - 2
	}
	contentWidth := dialogWidth - 6 // padding + border

	// Header: tool name + description
	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("214")).
		Render("Permission Required")

	toolLine := lipgloss.NewStyle().
		Bold(true).
		Render("Tool: " + d.toolName)
	if d.toolDesc != "" {
		toolLine += "\n" + lipgloss.NewStyle().
			Foreground(lipgloss.Color("247")).
			Render(d.toolDesc)
	}

	// File path + diff summary
	fileLine := ""
	if d.filePath != "" {
		summary := d.filePath
		if d.diffAdds > 0 || d.diffDels > 0 {
			summary = RenderDiffCompact(d.filePath, d.diffAdds, d.diffDels)
		}
		fileLine = "File: " + summary
	}

	// Content: args preview or diff
	var content string
	if d.diffView != "" {
		content = d.diffView
	} else {
		// Show args as plain text (truncated)
		content = d.args
		if len(content) > 500 {
			content = content[:500] + "..."
		}
	}

	// Setup viewport for scrollable content
	headerLines := 4 // header + tool + file + blank
	if fileLine != "" {
		headerLines++
	}
	buttonLines := 3 // blank + buttons + help
	availHeight := dialogHeight - headerLines - buttonLines - 4 // borders + padding
	if availHeight < 3 {
		availHeight = 3
	}

	if !d.ready {
		d.viewport = viewport.New()
		d.viewport.SetWidth(contentWidth)
		d.viewport.SetHeight(availHeight)
		d.viewport.SetContent(content)
		d.ready = true
	}

	viewContent := d.viewport.View()

	// Scroll indicator
	scrollInfo := ""
	if d.viewport.TotalLineCount() > availHeight {
		pct := int(d.viewport.ScrollPercent() * 100)
		scrollInfo = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Render(fmt.Sprintf(" %d%% ", pct))
	}

	// Buttons
	buttons := d.renderButtons()

	// Help line
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	help := dim.Render("1/y allow · 2/a allow all · 3/n deny · ↑/↓ scroll · f fullscreen")

	// Assemble
	var parts []string
	parts = append(parts, header)
	parts = append(parts, toolLine)
	if fileLine != "" {
		parts = append(parts, fileLine)
	}
	parts = append(parts, "")
	parts = append(parts, viewContent+scrollInfo)
	parts = append(parts, "")
	parts = append(parts, buttons)
	parts = append(parts, help)

	body := strings.Join(parts, "\n")

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("214")).
		Padding(1, 2).
		Width(dialogWidth).
		Render(body)
}

func (d *PermDialog) renderButtons() string {
	labels := []string{"Allow", "Allow Session", "Deny"}
	colors := []string{"42", "39", "196"} // green, blue, red

	var buttons []string
	for i, label := range labels {
		style := lipgloss.NewStyle().
			Padding(0, 2).
			Bold(i == d.selected)

		if i == d.selected {
			style = style.
				Foreground(lipgloss.Color("0")).
				Background(lipgloss.Color(colors[i]))
		} else {
			style = style.
				Foreground(lipgloss.Color(colors[i])).
				Border(lipgloss.NormalBorder()).
				BorderForeground(lipgloss.Color("238"))
		}
		buttons = append(buttons, style.Render(label))
	}

	return lipgloss.JoinHorizontal(lipgloss.Center, buttons...)
}

// ---------------------------------------------------------------------------
// Diff extraction from tool args
// ---------------------------------------------------------------------------

// extractDiffFromArgs generates a colored diff from edit/write tool arguments.
// For edit: computes diff between old_string and new_string.
// For write: shows the new content as all-adds.
func extractDiffFromArgs(toolName, argsJSON string, width int) (string, string, int, int) {
	var args map[string]any
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", "", 0, 0
	}

	filePath, _ := args["path"].(string)
	if filePath == "" {
		filePath, _ = args["file_path"].(string)
	}

	switch toolName {
	case "edit":
		oldStr, _ := args["old_string"].(string)
		newStr, _ := args["new_string"].(string)
		if oldStr == "" && newStr == "" {
			return filePath, "", 0, 0
		}
		diff, adds, dels := RenderDiff(filePath, oldStr, newStr, width)
		return filePath, diff, adds, dels

	case "multiedit":
		editsRaw, ok := args["edits"].([]any)
		if !ok || len(editsRaw) == 0 {
			return filePath, "", 0, 0
		}
		var allOld, allNew strings.Builder
		for _, e := range editsRaw {
			if edit, ok := e.(map[string]any); ok {
				old, _ := edit["old_string"].(string)
				new_, _ := edit["new_string"].(string)
				allOld.WriteString(old + "\n")
				allNew.WriteString(new_ + "\n")
			}
		}
		diff, adds, dels := RenderDiff(filePath, allOld.String(), allNew.String(), width)
		return filePath, diff, adds, dels

	case "write":
		content, _ := args["content"].(string)
		if content == "" {
			return filePath, "", 0, 0
		}
		diff, adds, dels := RenderDiff(filePath, "", content, width)
		return filePath, diff, adds, dels

	default:
		return filePath, "", 0, 0
	}
}
