package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/m0n0x41d/haft/internal/agent"
)

// ---------------------------------------------------------------------------
// View-model types
// ---------------------------------------------------------------------------

type viewMessage struct {
	Role        agent.Role
	Text        string
	Thinking    string // reasoning/thinking summary (shown in dim box)
	Tools       []viewTool
	Streaming   bool
	Attachments []viewAttachment
}

type viewAttachment struct {
	Name    string
	IsImage bool
}

type viewTool struct {
	CallID             string
	Name, Args, Output string
	IsError, Running   bool
	SubagentID         string     // for spawn_agent: tracks which subagent
	Children           []viewTool // nested tool calls from subagent
	Expanded           bool       // ctrl+o toggles child visibility
	OutputExpanded     bool       // toggle full output (default: collapsed at 15 lines)
}

func (vm *viewMessage) hasCompletedTools() bool {
	for _, t := range vm.Tools {
		if !t.Running {
			return true
		}
	}
	return false
}

// renderUserMessage renders user text as a highlighted block — background color, no borders.
// Matches Claude Code style: full-width background highlight on user messages.
func (m Model) renderUserMessage(msg viewMessage, width int) string {
	var parts []string
	if len(msg.Attachments) > 0 {
		chipStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("0")).
			Background(lipgloss.Color("48")).
			Bold(true)
		chips := make([]string, 0, len(msg.Attachments))
		for _, att := range msg.Attachments {
			label := " 📎 " + att.Name + " "
			if att.IsImage {
				label = " 🖼 " + att.Name + " "
			}
			chips = append(chips, chipStyle.Render(label))
		}
		parts = append(parts, strings.Join(chips, " "))
	}
	if msg.Text != "" {
		parts = append(parts, wrapPlainLine(msg.Text, max(20, width-4)))
	}
	if len(parts) == 0 {
		return ""
	}
	content := strings.Join(parts, "\n")
	return lipgloss.NewStyle().
		Background(lipgloss.Color("236")). // subtle dark highlight
		Foreground(lipgloss.Color("255")). // bright white text
		Width(width).
		Padding(0, 1).
		Render(content)
}

// renderAssistantMessage renders assistant text with a Claude-like marker + body.
// renderThinkingBox renders reasoning text with a subtle left border.
// When thinkingExpanded is false, caps at 10 lines with expand hint.
func (m Model) renderThinkingBox(thinking string, w int) string {
	const maxLines = 10
	lines := strings.Split(thinking, "\n")
	total := len(lines)
	hidden := 0

	if !m.thinkingExpanded && total > maxLines {
		hidden = total - maxLines
		lines = lines[total-maxLines:]
	}

	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	var rendered []string
	if hidden > 0 {
		rendered = append(rendered, dimStyle.Render(fmt.Sprintf("… (%d lines hidden — press t to expand)", hidden)))
	}
	for _, line := range lines {
		if ansi.StringWidth(line) > w-4 {
			line = ansi.Truncate(line, w-4, "…")
		}
		rendered = append(rendered, dimStyle.Render(line))
	}
	if m.thinkingExpanded && total > maxLines {
		rendered = append(rendered, dimStyle.Render("(press t to collapse)"))
	}

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
	body = strings.TrimLeft(body, "\n\r") // strip leading blank lines from LLM output
	if body == "" {
		return mark
	}
	// Short single-line text goes inline with the mark
	if !strings.Contains(body, "\n") && lipgloss.Width(body) < 80 {
		return mark + " " + body
	}
	// Indent by prepending spaces per-line. Safe with ANSI because
	// spaces before escape codes don't affect terminal interpretation.
	return mark + "\n" + indentBlock(body, "  ")
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
	case "haft_problem":
		return "Frame"
	case "haft_solution":
		return "Explore variants"
	case "haft_decision":
		return "Decide"
	case "haft_query":
		return "Query"
	case "haft_refresh":
		return "Refresh"
	case "haft_note":
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
			lines = append(lines, m.renderToolOutput(t.Output, t.IsError, t.OutputExpanded, w))
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
		if ansi.StringWidth(line) > bw {
			line = ansi.Truncate(line, bw, "…")
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
// When expanded=false, caps at 15 lines with a hint to expand.
func (m Model) renderToolOutput(output string, isError bool, expanded bool, w int) string {
	const maxLines = 15
	outLines := strings.Split(output, "\n")
	total := len(outLines)
	truncated := !expanded && total > maxLines
	if truncated {
		outLines = outLines[:maxLines]
	}

	var lines []string
	bw := w - 4
	for i, line := range outLines {
		if ansi.StringWidth(line) > bw {
			line = ansi.Truncate(line, bw, "…")
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
		lines = append(lines, m.styles.Dim.Render(
			fmt.Sprintf("  … (%d more lines — press e to expand)", total-maxLines)))
	} else if total > maxLines {
		lines = append(lines, m.styles.Dim.Render("  (press e to collapse)"))
	}

	return strings.Join(lines, "\n")
}

// renderPermission renders the permission prompt as a bordered box with 3 choices.
func (m Model) renderPermission(w int) string {
	param := extractToolParam(m.permToolName, m.permArgs)
	if param == "" {
		param = truncate(m.permArgs, w-20)
	}

	title := m.styles.PermTitle.Render(fmt.Sprintf("Allow %s?", m.permToolName))

	// Tool description for context
	desc := toolDescription(m.permToolName)

	var body strings.Builder
	body.WriteString(title)
	if desc != "" {
		body.WriteString("\n")
		body.WriteString(m.styles.Dim.Render(desc))
	}
	if param != "" {
		body.WriteString("\n")
		body.WriteString(m.styles.ToolParam.Render(truncate(param, w-10)))
	}
	body.WriteString("\n\n")
	body.WriteString(
		m.styles.PermKey.Render(" y ") + " allow   " +
			m.styles.PermKey.Render(" a ") + " allow all   " +
			m.styles.PermDeny.Render(" n ") + " deny")

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("214")).
		Padding(0, 2).
		Width(min(w-4, 70)).
		Render(body.String())

	return box
}

// toolDescription returns a brief description of what a tool does.
func toolDescription(name string) string {
	descs := map[string]string{
		"bash":           "Run a shell command",
		"read":           "Read a file from disk",
		"write":          "Create or overwrite a file",
		"edit":           "Modify an existing file",
		"multiedit":      "Apply multiple edits to a file",
		"glob":           "Search for files by pattern",
		"grep":           "Search file contents",
		"fetch":          "HTTP request to a URL",
		"spawn_agent":    "Launch a subagent",
		"lsp_diagnostics": "Get language server diagnostics",
		"lsp_references": "Find symbol references via LSP",
		"lsp_restart":    "Restart language server",
	}
	return descs[name]
}

func (m Model) renderSectionDivider(width int) string {
	return m.styles.BlockDivider.Render(strings.Repeat("─", width))
}

// ---------------------------------------------------------------------------
// Markdown
// ---------------------------------------------------------------------------

// maxTextWidth caps word-wrap width for readability on wide terminals.
const maxTextWidth = 120

func renderBodyText(text string, width int, style lipgloss.Style) string {
	if text == "" {
		return ""
	}
	renderWidth := min(width, maxTextWidth)
	return renderSimpleMarkdown(text, renderWidth, style)
}

func renderStreamingBodyText(text string, width int, style lipgloss.Style) string {
	if text == "" {
		return ""
	}
	renderWidth := min(width, maxTextWidth)
	return style.Render(wrapPlainLinePreserveWhitespace(text, renderWidth))
}

// renderSimpleMarkdown renders markdown without glamour.
// Handles: inline code, code blocks, bold, lists. No word-level corruption.
func renderSimpleMarkdown(text string, width int, style lipgloss.Style) string {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	var result []string
	inCodeBlock := false
	codeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Background(lipgloss.Color("236"))
	boldStyle := lipgloss.NewStyle().Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))

	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inCodeBlock = !inCodeBlock
			result = append(result, dimStyle.Render(line))
			continue
		}
		if inCodeBlock {
			result = append(result, codeStyle.Render(line))
			continue
		}

		if strings.TrimSpace(line) == "" {
			result = append(result, "")
			continue
		}

		// First wrap plain text, THEN apply inline formatting.
		// Never format before wrapping — ANSI codes break width measurement.
		wrapped := strings.Split(wrapPlainLine(line, width), "\n")
		for _, seg := range wrapped {
			result = append(result, processInlineFormatting(seg, codeStyle, boldStyle))
		}
	}
	return strings.Join(result, "\n")
}

// processInlineFormatting handles `code` and **bold** in a line.
// Splits into segments first, then styles each — never searches styled output.
func processInlineFormatting(line string, codeStyle, boldStyle lipgloss.Style) string {
	var result strings.Builder
	i := 0
	runes := []rune(line)
	n := len(runes)

	for i < n {
		// Check for inline code: `text`
		if runes[i] == '`' {
			end := -1
			for j := i + 1; j < n; j++ {
				if runes[j] == '`' {
					end = j
					break
				}
			}
			if end > i {
				code := string(runes[i+1 : end])
				result.WriteString(codeStyle.Render(" " + code + " "))
				i = end + 1
				continue
			}
		}
		// Check for bold: **text**
		if i+1 < n && runes[i] == '*' && runes[i+1] == '*' {
			end := -1
			for j := i + 2; j+1 < n; j++ {
				if runes[j] == '*' && runes[j+1] == '*' {
					end = j
					break
				}
			}
			if end > i {
				bold := string(runes[i+2 : end])
				result.WriteString(boldStyle.Render(bold))
				i = end + 2
				continue
			}
		}
		result.WriteRune(runes[i])
		i++
	}
	return result.String()
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

// findPhaseName is unused in v2 (no phase machine). Kept for potential future use.
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
	return wrapPlainLinePreserveWhitespace(line, width)
}

func wrapPlainLinePreserveWhitespace(line string, width int) string {
	if width <= 0 || line == "" {
		return line
	}

	var lines []string
	var current strings.Builder
	currentWidth := 0

	flush := func(trimTrailing bool) {
		value := current.String()
		if trimTrailing {
			value = strings.TrimRight(value, " ")
		}
		lines = append(lines, value)
		current.Reset()
		currentWidth = 0
	}

	for _, token := range splitWrapTokens(line) {
		tokenWidth := ansi.StringWidth(token)
		if currentWidth == 0 {
			if tokenWidth <= width {
				current.WriteString(token)
				currentWidth = tokenWidth
				continue
			}
			lines = append(lines, splitTokenToWidth(token, width)...)
			continue
		}

		if currentWidth+tokenWidth <= width {
			current.WriteString(token)
			currentWidth += tokenWidth
			continue
		}

		flush(true)
		if tokenWidth <= width {
			current.WriteString(token)
			currentWidth = tokenWidth
			continue
		}
		lines = append(lines, splitTokenToWidth(token, width)...)
	}

	if current.Len() > 0 {
		flush(false)
	}
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n")
}

func splitWrapTokens(line string) []string {
	var tokens []string
	var current strings.Builder
	inWhitespace := false
	for i, r := range line {
		isWhitespace := r == ' ' || r == '\t'
		if i == 0 {
			inWhitespace = isWhitespace
		}
		if isWhitespace != inWhitespace {
			tokens = append(tokens, current.String())
			current.Reset()
			inWhitespace = isWhitespace
		}
		current.WriteRune(r)
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}
	return tokens
}

func splitTokenToWidth(token string, width int) []string {
	if width <= 0 || token == "" {
		return []string{token}
	}

	var chunks []string
	var current strings.Builder
	currentWidth := 0
	for _, r := range token {
		rw := ansi.StringWidth(string(r))
		if currentWidth > 0 && currentWidth+rw > width {
			chunks = append(chunks, current.String())
			current.Reset()
			currentWidth = 0
		}
		current.WriteRune(r)
		currentWidth += rw
	}
	if current.Len() > 0 {
		chunks = append(chunks, current.String())
	}
	return chunks
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
	case "haft_problem", "haft_solution", "haft_decision":
		key = "action"
	case "haft_query":
		key = "action"
	case "haft_note":
		key = "title"
	default:
		return ""
	}
	return extractJSONString(argsJSON, key)
}

func extractJSONString(json, key string) string {
	for _, needle := range []string{`"` + key + `":"`, `"` + key + `": `} {
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
	if ansi.StringWidth(s) <= max {
		return s
	}
	return ansi.Truncate(s, max, "…")
}
