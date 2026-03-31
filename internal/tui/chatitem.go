package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"

	"github.com/m0n0x41d/haft/internal/agent"
)

// ---------------------------------------------------------------------------
// L2: Chat Items — domain rendering units with highlight support.
//
// Each item holds pre-rendered content (built by model from viewMessage data).
// Highlight is applied as a post-processing step via L1.ApplyHighlight.
//
// Depends on: L1 (selection geometry)
// ---------------------------------------------------------------------------

// ChatItem is one renderable, independently selectable unit in the chat list.
type ChatItem interface {
	// Render returns the rendered string. If a highlight is active,
	// it applies reverse-video via L1.ApplyHighlight.
	Render(width int) string

	// Height returns the number of display lines.
	Height() int

	// Selectable reports whether this item participates in text selection.
	Selectable() bool

	// SetHighlight sets the highlight range within this item.
	// Pass (-1,-1,-1,-1) to clear.
	SetHighlight(startLine, startCol, endLine, endCol int)

	// Highlight returns the current highlight range.
	Highlight() (startLine, startCol, endLine, endCol int)

	// PlainText returns ANSI-stripped text from the highlighted region
	// for clipboard extraction. Returns "" if no highlight.
	PlainText(width int) string
}

// ---------------------------------------------------------------------------
// Highlight state (embedded by concrete items)
// ---------------------------------------------------------------------------

type highlightState struct {
	hlStartLine, hlStartCol int
	hlEndLine, hlEndCol     int
}

func (h *highlightState) SetHighlight(sl, sc, el, ec int) {
	h.hlStartLine = sl
	h.hlStartCol = sc
	h.hlEndLine = el
	h.hlEndCol = ec
}

func (h *highlightState) Highlight() (int, int, int, int) {
	return h.hlStartLine, h.hlStartCol, h.hlEndLine, h.hlEndCol
}

func (h *highlightState) hasHighlight() bool {
	// -1 means "to end of item" — still a valid highlight
	if h.hlStartLine < 0 && h.hlEndLine < 0 {
		return false // both unset = no highlight
	}
	startLine := h.hlStartLine
	if startLine < 0 {
		startLine = 0
	}
	endLine := h.hlEndLine
	if endLine < 0 {
		return true // -1 end = "to end of item" = always has content
	}
	return startLine != endLine || h.hlStartCol != h.hlEndCol
}

// resolvedRange converts -1 sentinels to actual dimensions using item height.
func (h *highlightState) resolvedRange(height, width int) SelectionRange {
	sl, sc := h.hlStartLine, h.hlStartCol
	el, ec := h.hlEndLine, h.hlEndCol
	if sl < 0 {
		sl = 0
	}
	if sc < 0 {
		sc = 0
	}
	if el < 0 {
		el = height - 1
	}
	if ec < 0 {
		ec = width
	}
	return NormalizeRange(Coord{Line: sl, Col: sc}, Coord{Line: el, Col: ec})
}

func noHighlight() highlightState {
	return highlightState{-1, -1, -1, -1}
}

// ---------------------------------------------------------------------------
// Base item: pre-rendered content + highlight
// ---------------------------------------------------------------------------

type baseItem struct {
	rendered string
	height   int
	highlightState

	// Highlight rendering cache: parsed ScreenBuffer + last applied range.
	// Avoids re-parsing ANSI on every drag — only cell attr flips.
	cellBuf       *uv.ScreenBuffer
	cellBufWidth  int
	prevHighlight SelectionRange
}

func newBaseItem(rendered string) baseItem {
	h := 1
	if rendered != "" {
		h = strings.Count(rendered, "\n") + 1
	}
	return baseItem{
		rendered:       rendered,
		height:         h,
		highlightState: noHighlight(),
	}
}

func (b *baseItem) UpdateRendered(rendered string) {
	if b.rendered == rendered {
		return
	}
	b.rendered = rendered
	if rendered == "" {
		b.height = 1
	} else {
		b.height = strings.Count(rendered, "\n") + 1
	}
	b.cellBuf = nil
	b.cellBufWidth = 0
	b.prevHighlight = SelectionRange{}
}

func (b *baseItem) Render(width int) string {
	if !b.hasHighlight() {
		// Clear cached buffer highlight state when selection is removed
		if !b.prevHighlight.Empty() && b.cellBuf != nil {
			setReverse(b.cellBuf, b.height, b.prevHighlight, false)
			b.prevHighlight = SelectionRange{}
		}
		return b.rendered
	}

	cur := b.resolvedRange(b.height, width)

	// Lazy-parse buffer on first highlight (or if width changed)
	if b.cellBuf == nil || b.cellBufWidth != width {
		b.cellBuf = ParseContentBuffer(b.rendered, width)
		b.cellBufWidth = width
		b.prevHighlight = SelectionRange{}
	}

	result := RenderHighlighted(b.cellBuf, b.height, b.prevHighlight, cur)
	b.prevHighlight = cur
	return result
}

func (b *baseItem) Height() int { return b.height }

func (b *baseItem) PlainText(width int) string {
	if !b.hasHighlight() {
		return ""
	}
	return ExtractText(b.rendered, width, b.resolvedRange(b.height, width))
}

// ---------------------------------------------------------------------------
// Concrete item types
// ---------------------------------------------------------------------------

// userItem — user message with border decoration.
type userItem struct{ baseItem }

func (*userItem) Selectable() bool { return true }

// assistantTextItem — assistant text body (thinking + markdown/plain text).
type assistantTextItem struct{ baseItem }

func (*assistantTextItem) Selectable() bool { return true }

// toolItem — a single tool call (header + output).
type toolItem struct{ baseItem }

func (*toolItem) Selectable() bool { return true }

// errorItem — error message display.
type errorItem struct{ baseItem }

func (*errorItem) Selectable() bool { return true }

// dividerItem — visual separator between messages. Not selectable.
type dividerItem struct{ baseItem }

func (*dividerItem) Selectable() bool                { return false }
func (*dividerItem) SetHighlight(_, _, _, _ int)     {}
func (*dividerItem) Highlight() (int, int, int, int) { return -1, -1, -1, -1 }
func (*dividerItem) PlainText(_ int) string          { return "" }

// permissionItem — permission prompt. Not selectable (interactive element).
type permissionItem struct{ baseItem }

func (*permissionItem) Selectable() bool                { return false }
func (*permissionItem) SetHighlight(_, _, _, _ int)     {}
func (*permissionItem) Highlight() (int, int, int, int) { return -1, -1, -1, -1 }
func (*permissionItem) PlainText(_ int) string          { return "" }

func (m *Model) upsertUserItem(key, rendered string) ChatItem {
	if m.chatItemByID == nil {
		m.chatItemByID = make(map[string]ChatItem)
	}
	if existing, ok := m.chatItemByID[key]; ok {
		if item, ok := existing.(*userItem); ok {
			item.UpdateRendered(rendered)
			return item
		}
	}
	item := &userItem{baseItem: newBaseItem(rendered)}
	m.chatItemByID[key] = item
	return item
}

func (m *Model) upsertAssistantItem(key, rendered string) ChatItem {
	if m.chatItemByID == nil {
		m.chatItemByID = make(map[string]ChatItem)
	}
	if existing, ok := m.chatItemByID[key]; ok {
		if item, ok := existing.(*assistantTextItem); ok {
			item.UpdateRendered(rendered)
			return item
		}
	}
	item := &assistantTextItem{baseItem: newBaseItem(rendered)}
	m.chatItemByID[key] = item
	return item
}

func (m *Model) upsertToolItem(key, rendered string) ChatItem {
	if m.chatItemByID == nil {
		m.chatItemByID = make(map[string]ChatItem)
	}
	if existing, ok := m.chatItemByID[key]; ok {
		if item, ok := existing.(*toolItem); ok {
			item.UpdateRendered(rendered)
			return item
		}
	}
	item := &toolItem{baseItem: newBaseItem(rendered)}
	m.chatItemByID[key] = item
	return item
}

func (m *Model) upsertErrorItem(key, rendered string) ChatItem {
	if m.chatItemByID == nil {
		m.chatItemByID = make(map[string]ChatItem)
	}
	if existing, ok := m.chatItemByID[key]; ok {
		if item, ok := existing.(*errorItem); ok {
			item.UpdateRendered(rendered)
			return item
		}
	}
	item := &errorItem{baseItem: newBaseItem(rendered)}
	m.chatItemByID[key] = item
	return item
}

// ---------------------------------------------------------------------------
// Builder: viewMessage slice → ChatItem slice
// ---------------------------------------------------------------------------

// buildChatItems converts the model's viewMessages (+ transient state)
// into a flat list of ChatItems for the ChatList.
//
// Called from refreshChat() — rebuilds on every content change.
func (m *Model) buildChatItems() []ChatItem {
	fullWidth := max(20, m.width-2)
	bodyWidth := fullWidth // content width = viewport - divider margins only

	var items []ChatItem
	seen := make(map[string]bool)

	for _, msg := range m.messages {
		switch msg.Role {
		case agent.RoleUser:
			key := "user:" + msg.ID
			rendered := m.renderUserMessage(msg, fullWidth)
			item := m.upsertUserItem(key, rendered)
			items = append(items, item)
			seen[key] = true

		case agent.RoleAssistant:
			assistantItems, keys := m.buildAssistantItems(msg, bodyWidth)
			items = append(items, assistantItems...)
			for _, key := range keys {
				seen[key] = true
			}
		}
	}

	if m.errMsg != "" {
		key := "error"
		errContent := m.styles.ErrorText.Render("Error: " + m.errMsg)
		dismiss := m.styles.Dim.Render("\n\npress esc to dismiss")
		errBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("196")).
			Padding(0, 2).
			Width(min(bodyWidth, 70)).
			Render(errContent + dismiss)
		items = append(items, m.upsertErrorItem(key, errBox))
		seen[key] = true
	}

	for key := range m.chatItemByID {
		if !seen[key] {
			delete(m.chatItemByID, key)
		}
	}

	return items
}

// buildAssistantItems splits an assistant viewMessage into separate items:
// one for text (if present) and one per tool call.
func (m *Model) buildAssistantItems(msg viewMessage, w int) ([]ChatItem, []string) {
	var items []ChatItem
	var keys []string

	contentWidth := w - 2
	var textParts []string
	if msg.Thinking != "" {
		textParts = append(textParts, m.renderThinkingBox(msg.Thinking, contentWidth))
	}
	if msg.Text != "" {
		textParts = append(textParts, renderBodyText(msg.Text, contentWidth, m.styles.AssistantText))
	}

	textBody := strings.Join(textParts, "\n\n")
	if textBody != "" || len(msg.Tools) == 0 {
		key := "assistant:" + msg.ID
		rendered := m.renderAssistantBlock("", textBody)
		items = append(items, m.upsertAssistantItem(key, rendered))
		keys = append(keys, key)
	}

	for _, tool := range msg.Tools {
		key := "tool:" + tool.CallID
		rendered := m.renderTool(tool, w)
		items = append(items, m.upsertToolItem(key, rendered))
		keys = append(keys, key)
	}

	return items, keys
}
