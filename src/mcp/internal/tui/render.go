package tui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	glamour "charm.land/glamour/v2"
	glamansi "charm.land/glamour/v2/ansi"
	glamstyles "charm.land/glamour/v2/styles"
	"charm.land/lipgloss/v2"

	"github.com/m0n0x41d/quint-code/internal/agent"
)

// ---------------------------------------------------------------------------
// View-model types
// ---------------------------------------------------------------------------

type viewMessage struct {
	Role     agent.Role
	Text     string
	Thinking string // reasoning/thinking summary (shown in dim box)
	Tools    []viewTool
	Phase    agent.Phase // which lemniscate phase produced this message
}

type viewTool struct {
	CallID             string
	Name, Args, Output string
	IsError, Running   bool
	SubagentID         string     // for spawn_agent: tracks which subagent
	Children           []viewTool // nested tool calls from subagent
	Expanded           bool       // ctrl+o toggles child visibility
}

func (vm *viewMessage) hasCompletedTools() bool {
	for _, t := range vm.Tools {
		if !t.Running {
			return true
		}
	}
	return false
}

// renderUserMessage renders user text as a full-width accent block.
func (m Model) renderUserMessage(msg viewMessage, width int) string {
	border := m.styles.InputBorder.Render(strings.Repeat("━", width))
	body := renderPlainText(msg.Text, max(20, width-4), m.styles.UserText)
	body = indentBlock(body, "  ")
	return border + "\n" + body + "\n" + border
}

// renderAssistantMessage renders assistant text with a Claude-like marker + body.
// renderThinkingBox renders reasoning text with a subtle left border.
func (m Model) renderThinkingBox(thinking string, w int) string {
	const maxLines = 10
	lines := strings.Split(thinking, "\n")
	hidden := 0
	if len(lines) > maxLines {
		hidden = len(lines) - maxLines
		lines = lines[len(lines)-maxLines:]
	}

	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	var rendered []string
	if hidden > 0 {
		rendered = append(rendered, dimStyle.Render(fmt.Sprintf("… (%d lines hidden)", hidden)))
	}
	for _, line := range lines {
		if len(line) > w-4 {
			line = line[:w-4] + "…"
		}
		rendered = append(rendered, dimStyle.Render(line))
	}

	// Subtle left border — dim vertical line
	content := strings.Join(rendered, "\n")
	return lipgloss.NewStyle().
		BorderLeft(true).
		BorderStyle(lipgloss.ThickBorder()).
		BorderForeground(lipgloss.Color("236")).
		PaddingLeft(1).
		Render(content)
}

func (m Model) renderAssistantBlock(label string, body string) string {
	mark := m.styles.AssistantMark.Render("⏣")
	if label != "" {
		mark += " " + m.styles.AssistantLabel.Render(label)
	}
	if body == "" {
		return mark
	}
	// Short single-line text goes inline with the mark
	if !strings.Contains(body, "\n") && lipgloss.Width(body) < 80 {
		return mark + " " + body
	}
	// Use lipgloss PaddingLeft for ANSI-aware indentation (not string prefix)
	indented := lipgloss.NewStyle().PaddingLeft(2).Render(body)
	return mark + "\n" + indented
}

// toolDisplayName maps raw tool names to human-friendly display names.
func toolDisplayName(name string) string {
	switch name {
	case "bash":
		return "Bash"
	case "read":
		return "Read"
	case "write":
		return "Write"
	case "edit":
		return "Update"
	case "glob":
		return "Search"
	case "grep":
		return "Grep"
	case "spawn_agent":
		return "Explore"
	case "quint_problem":
		return "Frame"
	case "quint_solution":
		return "Explore variants"
	case "quint_decision":
		return "Decide"
	case "quint_query":
		return "Query"
	case "quint_refresh":
		return "Refresh"
	case "quint_note":
		return "Note"
	default:
		return name
	}
}

// renderTool renders a tool call block with icon + name + param + output.
func (m Model) renderTool(t viewTool, w int) string {
	var lines []string

	// Header: icon + DisplayName(param)
	icon := toolIcon(t, m)
	displayName := toolDisplayName(t.Name)

	param := extractToolParam(t.Name, t.Args)
	var header string
	if param != "" {
		header = fmt.Sprintf("%s %s(%s)", icon, m.styles.ToolName.Render(displayName), m.styles.ToolParam.Render(truncate(param, w-20)))
	} else {
		header = fmt.Sprintf("%s %s", icon, m.styles.ToolName.Render(displayName))
	}
	lines = append(lines, header)

	// Subagent tools: peek (last 3) by default, ctrl+o shows all
	if t.SubagentID != "" && len(t.Children) > 0 {
		if t.Expanded {
			// Full list — all children
			for i, child := range t.Children {
				lines = append(lines, m.renderChildTool(child, w, i == len(t.Children)-1))
			}
		} else {
			// Peek: last ~3 while running, summary line when done
			lines = append(lines, m.renderSubagentSummary(t, w))
		}
		return strings.Join(lines, "\n")
	}

	// Running state (non-subagent tools) — hexagon pulses, no spinner needed
	if t.Running {
		return strings.Join(lines, "\n")
	}

	// Output body — special rendering for edit diffs
	if t.Output != "" {
		if t.Name == "edit" && strings.Contains(t.Output, "--- old") {
			lines = append(lines, m.renderEditDiff(t.Output, w))
		} else {
			lines = append(lines, m.renderToolOutput(t.Output, t.IsError, w))
		}
	}

	return strings.Join(lines, "\n")
}

// renderEditDiff renders edit tool output with colored diff lines.
func (m Model) renderEditDiff(output string, w int) string {
	allLines := strings.Split(output, "\n")
	var result []string

	// First line is the summary (e.g., "Edited path (-3 +5 lines)")
	if len(allLines) > 0 {
		result = append(result, m.styles.Dim.Render("  "+allLines[0]))
	}

	bw := w - 4
	for _, line := range allLines[1:] {
		if len(line) > bw {
			line = line[:bw] + "…"
		}
		switch {
		case strings.HasPrefix(line, "--- old") || strings.HasPrefix(line, "+++ new"):
			// Skip diff headers
			continue
		case strings.HasPrefix(line, "-"):
			// Removed line — red background
			result = append(result, lipgloss.NewStyle().
				Foreground(lipgloss.Color("210")).Render("  "+line))
		case strings.HasPrefix(line, "+"):
			// Added line — green
			result = append(result, lipgloss.NewStyle().
				Foreground(lipgloss.Color("114")).Render("  "+line))
		default:
			result = append(result, m.styles.ToolBody.Render("  "+line))
		}
	}

	return strings.Join(result, "\n")
}

// renderToolOutput renders standard tool output with truncation.
// Uses ⎿ connector on first line (Claude Code pattern).
func (m Model) renderToolOutput(output string, isError bool, w int) string {
	const maxLines = 15
	outLines := strings.Split(output, "\n")
	truncated := len(outLines) > maxLines
	if truncated {
		outLines = outLines[:maxLines]
	}

	var lines []string
	bw := w - 4
	for i, line := range outLines {
		if len(line) > bw {
			line = line[:bw] + "…"
		}
		prefix := "  "
		if i == 0 {
			prefix = m.styles.Dim.Render("⎿") + " "
		}
		if isError {
			lines = append(lines, prefix+m.styles.ToolError.Render(line))
		} else {
			lines = append(lines, prefix+m.styles.ToolBody.Render(line))
		}
	}
	if truncated {
		total := len(strings.Split(output, "\n"))
		lines = append(lines, m.styles.Dim.Render(
			fmt.Sprintf("  … (%d more lines)", total-maxLines)))
	}

	return strings.Join(lines, "\n")
}

// renderPermission renders the permission prompt as a bordered box.
func (m Model) renderPermission(w int) string {
	param := extractToolParam(m.permToolName, m.permArgs)
	if param == "" {
		param = truncate(m.permArgs, w-20)
	}

	title := m.styles.PermTitle.Render(fmt.Sprintf("Allow %s?", m.permToolName))

	var body strings.Builder
	body.WriteString(title)
	if param != "" {
		body.WriteString("\n")
		body.WriteString(m.styles.ToolParam.Render(truncate(param, w-10)))
	}
	body.WriteString("\n\n")
	body.WriteString(
		m.styles.PermKey.Render(" y ") + " allow   " +
			m.styles.PermDeny.Render(" n ") + " deny")

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("214")). // yellow
		Padding(0, 2).
		Width(min(w-4, 60)).
		Render(body.String())

	return box
}

func (m Model) renderSectionDivider(width int) string {
	return m.styles.BlockDivider.Render(strings.Repeat("─", width))
}

// ---------------------------------------------------------------------------
// Markdown
// ---------------------------------------------------------------------------

// Cached markdown renderer — re-created only when width changes.
var (
	cachedRenderer      *glamour.TermRenderer
	cachedRendererWidth int
)

func renderBodyText(text string, width int, style lipgloss.Style) string {
	if looksLikeMarkdown(text) {
		return renderMarkdown(text, width)
	}
	return renderPlainText(text, width, style)
}

func renderPlainText(text string, width int, style lipgloss.Style) string {
	if text == "" {
		return ""
	}

	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	rendered := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			rendered = append(rendered, "")
			continue
		}
		wrapped := strings.Split(wrapPlainLine(line, width), "\n")
		for _, visualLine := range wrapped {
			rendered = append(rendered, style.Render(visualLine))
		}
	}

	return strings.Join(rendered, "\n")
}

func looksLikeMarkdown(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return false
	}
	if strings.Contains(trimmed, "```") || strings.Contains(trimmed, "`") {
		return true
	}
	for _, line := range strings.Split(trimmed, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "# "):
			return true
		case strings.HasPrefix(line, "## "):
			return true
		case strings.HasPrefix(line, "### "):
			return true
		case strings.HasPrefix(line, "- "):
			return true
		case strings.HasPrefix(line, "* "):
			return true
		case strings.HasPrefix(line, "> "):
			return true
		case strings.HasPrefix(line, "|"):
			return true
		case strings.HasPrefix(line, "1. "):
			return true
		case strings.HasPrefix(line, "2. "):
			return true
		case strings.HasPrefix(line, "3. "):
			return true
		}
	}
	return false
}

// renderMarkdown renders text as terminal markdown using glamour.
func renderMarkdown(text string, width int) string {
	if text == "" {
		return ""
	}
	if cachedRenderer == nil || cachedRendererWidth != width {
		style := glamstyles.DarkStyleConfig
		style.Document = glamansi.StyleBlock{
			StylePrimitive: glamansi.StylePrimitive{
				Color: strPtr("252"),
			},
			Margin: uintPtr(0),
		}
		style.BlockQuote.Indent = uintPtr(0)
		style.BlockQuote.IndentToken = strPtr("")
		r, err := glamour.NewTermRenderer(
			glamour.WithStyles(style),
			glamour.WithWordWrap(width),
		)
		if err != nil {
			return text
		}
		cachedRenderer = r
		cachedRendererWidth = width
	}
	rendered, err := cachedRenderer.Render(text)
	if err != nil {
		return text
	}
	return strings.TrimRight(rendered, "\n")
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// renderSubagentSummary renders a collapsed summary of subagent activity.
// Shows: peek of recent tools + count + expand hint
func (m Model) renderSubagentSummary(t viewTool, w int) string {
	total := len(t.Children)

	var lines []string

	if t.Running {
		// Show last 3 tools as a peek (like Claude Code's collapsed view)
		peekStart := len(t.Children) - 3
		if peekStart < 0 {
			peekStart = 0
		}
		for _, child := range t.Children[peekStart:] {
			displayName := toolDisplayName(child.Name)
			param := extractToolParam(child.Name, child.Args)
			peek := displayName
			if param != "" {
				peek += "(" + truncate(param, w-30) + ")"
			}
			icon := toolIcon(child, m)
			lines = append(lines, "  "+icon+" "+m.styles.Dim.Render(peek))
		}
		if total > 3 {
			lines = append(lines, m.styles.Dim.Render(fmt.Sprintf("  +%d more tool uses (ctrl+o to expand)", total-3)))
		}
	} else {
		expandHint := "ctrl+o to expand"
		if t.Expanded {
			expandHint = "ctrl+o to collapse"
		}
		// Completed: show count + peek of what was done
		peek := ""
		if total > 0 {
			last := t.Children[total-1]
			peek = toolDisplayName(last.Name)
			if p := extractToolParam(last.Name, last.Args); p != "" {
				peek += "(" + truncate(p, 40) + ")"
			}
			peek = ", last: " + peek
		}
		lines = append(lines, m.styles.Dim.Render(fmt.Sprintf("  %d tool uses%s (%s)", total, peek, expandHint)))
	}

	return strings.Join(lines, "\n")
}

// renderChildTool renders a subagent's tool call with tree connector lines.
func (m Model) renderChildTool(t viewTool, w int, isLast bool) string {
	connector := "├─"
	if isLast && !t.Running {
		connector = "└─"
	}

	icon := toolIcon(t, m)

	param := extractToolParam(t.Name, t.Args)
	paramStr := ""
	if param != "" {
		paramStr = " " + m.styles.ToolParam.Render(truncate(param, w-25))
	}

	return fmt.Sprintf("  %s %s %s%s",
		m.styles.Dim.Render(connector), icon, m.styles.ToolName.Render(toolDisplayName(t.Name)), paramStr)
}

// toolIcon returns the colored ⏣ icon for a tool call state.
// Running state pulses through brightness levels on the spinner tick.
func toolIcon(t viewTool, m Model) string {
	switch {
	case t.Running:
		// Pulsate: cycle through 4 brightness levels (~320ms per full cycle)
		shades := []string{"72", "78", "84", "78"} // green brightness pulse
		shade := shades[m.spinnerTick%len(shades)]
		return lipgloss.NewStyle().Foreground(lipgloss.Color(shade)).Render("⏣")
	case t.IsError:
		return m.styles.ErrorText.Render("⏣")
	default:
		return m.styles.ToolDone.Render("⏣")
	}
}

func (m Model) findPhaseName(phase agent.Phase) string {
	names := map[agent.Phase]string{
		agent.PhaseFramer:   "haft-framer",
		agent.PhaseExplorer: "haft-explorer",
		agent.PhaseDecider:  "haft-decider",
		agent.PhaseWorker:   "haft-worker",
		agent.PhaseMeasure:  "haft-measure",
	}
	return names[phase]
}

func indentBlock(content string, prefix string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}

func wrapPlainLine(line string, width int) string {
	if width <= 0 {
		return line
	}

	words := strings.Fields(line)
	if len(words) == 0 {
		return ""
	}

	var lines []string
	current := words[0]

	for _, word := range words[1:] {
		candidate := current + " " + word
		if utf8.RuneCountInString(candidate) <= width {
			current = candidate
			continue
		}
		lines = append(lines, current)
		current = word
	}

	lines = append(lines, current)
	return strings.Join(lines, "\n")
}

func uintPtr(v uint) *uint {
	return &v
}

func strPtr(v string) *string {
	return &v
}

func extractToolParam(name, argsJSON string) string {
	var key string
	switch name {
	case "bash":
		key = "command"
	case "read", "write", "edit":
		key = "path"
	case "glob", "grep":
		key = "pattern"
	case "spawn_agent":
		key = "task"
	case "quint_problem", "quint_solution", "quint_decision":
		key = "action"
	case "quint_query":
		key = "action"
	case "quint_note":
		key = "title"
	default:
		return ""
	}
	return extractJSONString(argsJSON, key)
}

func extractJSONString(json, key string) string {
	for _, needle := range []string{`"` + key + `":"`, `"` + key + `": "`} {
		idx := strings.Index(json, needle)
		if idx < 0 {
			continue
		}
		start := idx + len(needle)
		end := strings.Index(json[start:], `"`)
		if end < 0 {
			continue
		}
		s := json[start : start+end]
		s = strings.ReplaceAll(s, `\n`, " ")
		s = strings.ReplaceAll(s, `\t`, " ")
		return s
	}
	return ""
}

func truncate(s string, max int) string {
	if max < 4 {
		max = 4
	}
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}
