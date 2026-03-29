package tui

import (
	"strings"
	"unicode"

	"github.com/charmbracelet/x/ansi"

	uv "github.com/charmbracelet/ultraviolet"
)

// ---------------------------------------------------------------------------
// L1: Selection Geometry — pure coordinate math and cell-level operations.
//
// No side effects, no domain knowledge (messages, tools, etc.).
// Depends on: ultraviolet (L0) for cell-level highlight + text extraction.
// ---------------------------------------------------------------------------

// Coord is a position in rendered content (line-relative).
type Coord struct {
	Line int
	Col  int
}

// SelectionRange is a normalized selection where Start <= End.
// Zero value means no selection.
type SelectionRange struct {
	Start Coord
	End   Coord
}

// NormalizeRange returns a range where Start <= End.
// Handles backward selection (user dragged upward/left).
func NormalizeRange(a, b Coord) SelectionRange {
	if a.Line > b.Line || (a.Line == b.Line && a.Col > b.Col) {
		a, b = b, a
	}
	return SelectionRange{Start: a, End: b}
}

// Empty reports whether the range selects zero characters.
func (r SelectionRange) Empty() bool {
	return r.Start == r.End
}

// RangeForLine returns the column span [startCol, endCol) that the selection
// covers on the given line. Returns (-1, -1) if the line is outside the range.
func (r SelectionRange) RangeForLine(line, lineWidth int) (startCol, endCol int) {
	if line < r.Start.Line || line > r.End.Line {
		return -1, -1
	}
	startCol = 0
	endCol = lineWidth
	if line == r.Start.Line {
		startCol = r.Start.Col
	}
	if line == r.End.Line {
		endCol = r.End.Col
	}
	return startCol, endCol
}

// ---------------------------------------------------------------------------
// Word and line boundaries (pure text operations)
// ---------------------------------------------------------------------------

// WordAt returns the column range [start, end) of the word at col
// in a plain-text (ANSI-stripped) line. Returns (col, col) if col
// is on whitespace or out of bounds.
func WordAt(line string, col int) (startCol, endCol int) {
	runes := []rune(line)
	if col < 0 || col >= len(runes) {
		return col, col
	}
	if unicode.IsSpace(runes[col]) {
		return col, col
	}

	isWord := func(r rune) bool {
		return !unicode.IsSpace(r) && r != '(' && r != ')' &&
			r != '{' && r != '}' && r != '[' && r != ']' &&
			r != '"' && r != '\'' && r != '`' &&
			r != ',' && r != ';' && r != ':'
	}

	startCol = col
	for startCol > 0 && isWord(runes[startCol-1]) {
		startCol--
	}
	endCol = col + 1
	for endCol < len(runes) && isWord(runes[endCol]) {
		endCol++
	}
	return startCol, endCol
}

// LineExtent returns (0, displayWidth) for a line — selects the full line.
func LineExtent(line string) (startCol, endCol int) {
	return 0, ansi.StringWidth(line)
}

// ---------------------------------------------------------------------------
// Highlight application: content string → highlighted ANSI string
// ---------------------------------------------------------------------------

// ParseContentBuffer parses ANSI content into a reusable ScreenBuffer.
// Expensive (ANSI parsing) — call once, reuse across drags.
func ParseContentBuffer(content string, width int) *uv.ScreenBuffer {
	lines := strings.Split(content, "\n")
	height := len(lines)
	if height == 0 {
		return nil
	}

	bufWidth := width
	for _, l := range lines {
		if w := ansi.StringWidth(l); w > bufWidth {
			bufWidth = w
		}
	}

	buf := uv.NewScreenBuffer(bufWidth, height)
	styled := uv.NewStyledString(content)
	styled.Draw(&buf, uv.Rect(0, 0, bufWidth, height))
	return &buf
}

// setReverse applies or clears AttrReverse on cells in the given range.
func setReverse(buf *uv.ScreenBuffer, height int, sel SelectionRange, on bool) {
	if sel.Empty() || buf == nil {
		return
	}
	for y := sel.Start.Line; y <= sel.End.Line && y < height; y++ {
		line := buf.Line(y)
		if line == nil {
			continue
		}

		colStart, colEnd := sel.RangeForLine(y, len(line))
		if colStart < 0 {
			continue
		}

		lastContentX := findLastContent(line, colStart, colEnd)
		highlightEnd := colStart
		if lastContentX >= 0 {
			highlightEnd = lastContentX + 1
		}

		for x := colStart; x < highlightEnd && x < len(line); x++ {
			cell := line.At(x)
			if cell == nil {
				continue
			}
			if on {
				cell.Style.Attrs |= uv.AttrReverse
			} else {
				cell.Style.Attrs &^= uv.AttrReverse
			}
		}
	}
}

// RenderHighlighted applies reverse to a cached buffer and renders to string.
// Cheap: no ANSI re-parsing, just cell attr flip + render.
func RenderHighlighted(buf *uv.ScreenBuffer, height int, prev, cur SelectionRange) string {
	setReverse(buf, height, prev, false) // clear old
	setReverse(buf, height, cur, true)   // apply new
	return renderBuffer(buf, height)
}

// ApplyHighlight renders content into a cell buffer, applies reverse-video
// to cells within sel, and returns the resulting ANSI string.
// Non-cached path — used for one-shot rendering (tests, non-interactive).
func ApplyHighlight(content string, width int, sel SelectionRange) string {
	if sel.Empty() {
		return content
	}
	lines := strings.Split(content, "\n")
	height := len(lines)
	if height == 0 {
		return content
	}
	buf := ParseContentBuffer(content, width)
	if buf == nil {
		return content
	}
	setReverse(buf, height, sel, true)
	return renderBuffer(buf, height)
}

// ExtractText renders content into a cell buffer and collects plain text
// from cells within sel. Returns clipboard-ready text.
func ExtractText(content string, width int, sel SelectionRange) string {
	if sel.Empty() {
		return ""
	}

	lines := strings.Split(content, "\n")
	height := len(lines)
	if height == 0 {
		return ""
	}

	// Buffer width must fit the widest content line
	bufWidth := width
	for _, l := range lines {
		if w := ansi.StringWidth(l); w > bufWidth {
			bufWidth = w
		}
	}

	buf := uv.NewScreenBuffer(bufWidth, height)
	styled := uv.NewStyledString(content)
	styled.Draw(&buf, uv.Rect(0, 0, bufWidth, height))

	var sb strings.Builder
	prevY := -1

	for y := sel.Start.Line; y <= sel.End.Line && y < height; y++ {
		line := buf.Line(y)
		if line == nil {
			continue
		}

		colStart, colEnd := sel.RangeForLine(y, len(line))
		if colStart < 0 {
			continue
		}

		if prevY >= 0 {
			sb.WriteString("\n")
		}
		prevY = y

		for x := colStart; x < colEnd && x < len(line); x++ {
			cell := line.At(x)
			if cell != nil && !cell.IsZero() {
				sb.WriteString(cell.Content)
			}
		}
	}

	return strings.TrimRight(sb.String(), " \t")
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// findLastContent returns the x position of the last non-blank cell in
// [colStart, colEnd). Returns -1 if all cells are blank.
func findLastContent(line uv.Line, colStart, colEnd int) int {
	last := -1
	for x := colStart; x < colEnd && x < len(line); x++ {
		cell := line.At(x)
		if cell != nil && !cell.IsZero() && !cell.Equal(&uv.EmptyCell) {
			last = x
		}
	}
	return last
}

// renderBuffer converts a ScreenBuffer back to an ANSI string with newlines.
func renderBuffer(buf *uv.ScreenBuffer, height int) string {
	var sb strings.Builder
	for y := range height {
		if y > 0 {
			sb.WriteString("\n")
		}
		line := buf.Line(y)
		if line != nil {
			sb.WriteString(line.Render())
		}
	}
	return sb.String()
}
