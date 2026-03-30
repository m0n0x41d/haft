package tui

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// ---------------------------------------------------------------------------
// L3: ChatList — scrollable list of ChatItems with mouse text selection.
//
// Replaces charm.land/bubbles/v2/viewport for the chat area.
// Handles: scrolling, mouse hit-testing, selection state machine, clipboard.
//
// Depends on: L2 (ChatItem for rendering + highlight)
// ---------------------------------------------------------------------------

const (
	itemGap             = 1 // blank lines between items
	scrollLines         = 3 // lines per mouse wheel tick
	doubleClickTimeout  = 400 * time.Millisecond
	clickPositionJitter = 2 // pixels of tolerance for multi-click detection
)

// CopySelectionMsg is sent when the user releases the mouse after selecting text.
// The shell (model.go) handles clipboard I/O.
type CopySelectionMsg struct {
	Text string
}

// ChatList is a scrollable list of ChatItems with mouse text selection.
type ChatList struct {
	items       []ChatItem
	width       int
	height      int // visible viewport height in lines
	totalHeight int // total content height in lines

	// Viewport state
	yOffset int  // line offset into total content
	follow  bool // auto-scroll to bottom on new items

	// Item layout: itemOffsets[i] = first content line of items[i].
	// len(itemOffsets) == len(items).
	itemOffsets []int

	// Mouse state machine
	mouseState mouseSelectState
	downCoord  Coord // content-space coords where mouse went down
	dragCoord  Coord // current drag position in content-space
	downItem   int   // item index at mouse-down
	dragItem   int   // current item index during drag

	// Click detection (double/triple)
	lastClickTime time.Time
	lastClickX    int
	lastClickY    int
	clickCount    int

	// Drag debounce
	lastDragTime time.Time
}

type mouseSelectState int

const (
	mouseIdle mouseSelectState = iota
	mouseSelecting
	mouseSelected // highlight persists until next mouseDown
)

// NewChatList creates an empty chat list.
func NewChatList(width, height int) ChatList {
	return ChatList{
		width:  width,
		height: height,
		follow: true,
	}
}

// ---------------------------------------------------------------------------
// Content management
// ---------------------------------------------------------------------------

// SetItems replaces all items, recomputes layout, and preserves scroll.
func (cl *ChatList) SetItems(items []ChatItem) {
	wasAtBottom := cl.AtBottom()
	cl.items = items
	cl.recomputeOffsets()
	if wasAtBottom || cl.follow {
		cl.ScrollToBottom()
	}
}

// recomputeOffsets recalculates itemOffsets and totalHeight.
func (cl *ChatList) recomputeOffsets() {
	cl.itemOffsets = make([]int, len(cl.items))
	offset := 0
	for i, item := range cl.items {
		cl.itemOffsets[i] = offset
		offset += item.Height()
		if i < len(cl.items)-1 {
			offset += itemGap
		}
	}
	cl.totalHeight = offset
}

// SetSize updates the viewport dimensions.
func (cl *ChatList) SetSize(width, height int) {
	cl.width = width
	cl.height = height
}

// ---------------------------------------------------------------------------
// Viewport / scrolling
// ---------------------------------------------------------------------------

// ScrollBy scrolls by n lines (positive = down, negative = up).
func (cl *ChatList) ScrollBy(n int) {
	cl.setYOffset(cl.yOffset + n)
	if n < 0 {
		cl.follow = false
	}
}

// PageUp scrolls up by viewport height.
func (cl *ChatList) PageUp() {
	cl.ScrollBy(-cl.height)
}

// PageDown scrolls down by viewport height.
func (cl *ChatList) PageDown() {
	cl.ScrollBy(cl.height)
}

// ScrollToBottom scrolls to the very bottom.
func (cl *ChatList) ScrollToBottom() {
	cl.setYOffset(cl.totalHeight - cl.height)
	cl.follow = true
}

// AtBottom reports whether the viewport is at the bottom.
func (cl *ChatList) AtBottom() bool {
	maxOffset := cl.totalHeight - cl.height
	if maxOffset <= 0 {
		return true
	}
	return cl.yOffset >= maxOffset
}

// ScrollPercent returns 0.0–1.0 indicating scroll position.
func (cl *ChatList) ScrollPercent() float64 {
	maxOffset := cl.totalHeight - cl.height
	if maxOffset <= 0 {
		return 1.0
	}
	return float64(cl.yOffset) / float64(maxOffset)
}

// YOffset returns the current vertical scroll offset.
func (cl *ChatList) YOffset() int {
	return cl.yOffset
}

func (cl *ChatList) setYOffset(offset int) {
	maxOffset := cl.totalHeight - cl.height
	if maxOffset < 0 {
		maxOffset = 0
	}
	if offset < 0 {
		offset = 0
	}
	if offset > maxOffset {
		offset = maxOffset
	}
	cl.yOffset = offset
}

// ---------------------------------------------------------------------------
// Mouse interaction
// ---------------------------------------------------------------------------

// HandleMouseWheel processes mouse wheel events.
func (cl *ChatList) HandleMouseWheel(btn tea.MouseButton) {
	switch btn {
	case tea.MouseWheelUp:
		cl.ScrollBy(-scrollLines)
	case tea.MouseWheelDown:
		cl.ScrollBy(scrollLines)
	}
}

// HandleMouseDown processes a mouse-down event at viewport-relative (x, y).
// Returns a tea.Cmd if a delayed action is needed (double-click detection).
func (cl *ChatList) HandleMouseDown(x, y int) tea.Cmd {
	if len(cl.items) == 0 {
		return nil
	}

	contentLine := cl.yOffset + y
	itemIdx, localLine := cl.itemAtLine(contentLine)
	if itemIdx < 0 || !cl.items[itemIdx].Selectable() {
		return nil
	}

	// Clear previous highlight
	cl.clearHighlight()

	// Detect multi-click
	now := time.Now()
	if now.Sub(cl.lastClickTime) <= doubleClickTimeout &&
		abs(x-cl.lastClickX) <= clickPositionJitter &&
		abs(y-cl.lastClickY) <= clickPositionJitter {
		cl.clickCount++
	} else {
		cl.clickCount = 1
	}
	cl.lastClickTime = now
	cl.lastClickX = x
	cl.lastClickY = y

	cl.mouseState = mouseSelecting
	cl.downItem = itemIdx
	cl.downCoord = Coord{Line: localLine, Col: x}
	cl.dragItem = itemIdx
	cl.dragCoord = Coord{Line: localLine, Col: x}

	switch cl.clickCount {
	case 2:
		cl.selectWord(itemIdx, x, localLine)
	case 3:
		cl.selectLine(itemIdx, localLine)
		cl.clickCount = 0
	}

	return nil
}

// HandleMouseDrag processes mouse motion during selection.
// Debounced to ~60fps to avoid expensive re-renders on every pixel move.
func (cl *ChatList) HandleMouseDrag(x, y int) {
	if cl.mouseState != mouseSelecting {
		return
	}
	if len(cl.items) == 0 {
		return
	}

	// Debounce: skip if < 16ms since last drag (≈60fps cap)
	now := time.Now()
	if now.Sub(cl.lastDragTime) < 16*time.Millisecond {
		return
	}
	cl.lastDragTime = now

	// Auto-scroll at viewport edges
	if y <= 0 {
		cl.ScrollBy(-1)
	} else if y >= cl.height-1 {
		cl.ScrollBy(1)
	}

	contentLine := cl.yOffset + y
	itemIdx, localLine := cl.itemAtLine(contentLine)
	if itemIdx < 0 {
		return
	}

	cl.dragItem = itemIdx
	cl.dragCoord = Coord{Line: localLine, Col: x}

	cl.applyHighlight()
}

// HandleMouseUp processes mouse-up. Returns a CopySelectionMsg if text was selected.
// Text is extracted immediately — delayed extraction breaks because
// refreshChat() rebuilds items and clears highlights.
func (cl *ChatList) HandleMouseUp(x, y int) tea.Cmd {
	if cl.mouseState != mouseSelecting {
		return nil
	}

	cl.mouseState = mouseSelected

	if !cl.HasSelection() {
		cl.mouseState = mouseIdle
		return nil
	}

	// Extract text now, before refreshChat() clears highlights.
	text := cl.SelectedText()
	if text == "" {
		cl.mouseState = mouseIdle
		return nil
	}

	return func() tea.Msg { return CopySelectionMsg{Text: text} }
}

// ClearSelection removes all highlights and resets mouse state.
func (cl *ChatList) ClearSelection() {
	cl.clearHighlight()
	cl.mouseState = mouseIdle
}

// HasSelection reports whether there is highlighted text.
func (cl *ChatList) HasSelection() bool {
	for _, item := range cl.items {
		if !item.Selectable() {
			continue
		}
		sl, sc, el, ec := item.Highlight()
		if sl >= 0 && el >= 0 && (sl != el || sc != ec) {
			return true
		}
	}
	return false
}

// SelectedText returns the concatenated plain text from all highlighted items.
func (cl *ChatList) SelectedText() string {
	var parts []string
	for _, item := range cl.items {
		text := item.PlainText(cl.width)
		if text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n")
}

// ---------------------------------------------------------------------------
// Hit-testing
// ---------------------------------------------------------------------------

// itemAtLine returns the item index and local line offset for a content line.
// Returns (-1, 0) if the line is in a gap or out of bounds.
func (cl *ChatList) itemAtLine(contentLine int) (itemIdx, localLine int) {
	if len(cl.items) == 0 || contentLine < 0 {
		return -1, 0
	}

	// Binary search for the item containing contentLine
	lo, hi := 0, len(cl.items)-1
	for lo <= hi {
		mid := (lo + hi) / 2
		itemStart := cl.itemOffsets[mid]
		itemEnd := itemStart + cl.items[mid].Height() - 1

		if contentLine < itemStart {
			hi = mid - 1
		} else if contentLine > itemEnd {
			lo = mid + 1
		} else {
			return mid, contentLine - itemStart
		}
	}

	return -1, 0 // in a gap between items
}

// ---------------------------------------------------------------------------
// Selection helpers
// ---------------------------------------------------------------------------

func (cl *ChatList) clearHighlight() {
	for _, item := range cl.items {
		item.SetHighlight(-1, -1, -1, -1)
	}
}

// applyHighlight sets highlight on items between downItem/downCoord and dragItem/dragCoord.
func (cl *ChatList) applyHighlight() {
	cl.clearHighlight()

	startItem, startCoord := cl.downItem, cl.downCoord
	endItem, endCoord := cl.dragItem, cl.dragCoord

	// Normalize: ensure startItem <= endItem
	if startItem > endItem || (startItem == endItem &&
		(startCoord.Line > endCoord.Line ||
			(startCoord.Line == endCoord.Line && startCoord.Col > endCoord.Col))) {
		startItem, endItem = endItem, startItem
		startCoord, endCoord = endCoord, startCoord
	}

	for i := startItem; i <= endItem && i < len(cl.items); i++ {
		item := cl.items[i]
		if !item.Selectable() {
			continue
		}

		switch {
		case startItem == endItem:
			// Single-item selection
			item.SetHighlight(startCoord.Line, startCoord.Col, endCoord.Line, endCoord.Col)
		case i == startItem:
			// First item: from start coord to end of item
			item.SetHighlight(startCoord.Line, startCoord.Col, -1, -1)
		case i == endItem:
			// Last item: from beginning to end coord
			item.SetHighlight(0, 0, endCoord.Line, endCoord.Col)
		default:
			// Middle item: fully highlighted
			item.SetHighlight(0, 0, -1, -1)
		}
	}
}

// selectWord selects the word at (col, localLine) in the given item.
func (cl *ChatList) selectWord(itemIdx, col, localLine int) {
	item := cl.items[itemIdx]
	rendered := item.Render(cl.width) // un-highlighted render
	lines := strings.Split(rendered, "\n")
	if localLine < 0 || localLine >= len(lines) {
		return
	}

	plain := ansi.Strip(lines[localLine])
	startCol, endCol := WordAt(plain, col)
	if startCol == endCol {
		return
	}

	cl.downItem = itemIdx
	cl.downCoord = Coord{Line: localLine, Col: startCol}
	cl.dragItem = itemIdx
	cl.dragCoord = Coord{Line: localLine, Col: endCol}
	cl.applyHighlight()
}

// selectLine selects the full line at localLine in the given item.
func (cl *ChatList) selectLine(itemIdx, localLine int) {
	item := cl.items[itemIdx]
	rendered := item.Render(cl.width)
	lines := strings.Split(rendered, "\n")
	if localLine < 0 || localLine >= len(lines) {
		return
	}

	plain := ansi.Strip(lines[localLine])
	startCol, endCol := LineExtent(plain)

	cl.downItem = itemIdx
	cl.downCoord = Coord{Line: localLine, Col: startCol}
	cl.dragItem = itemIdx
	cl.dragCoord = Coord{Line: localLine, Col: endCol}
	cl.applyHighlight()
}

// ---------------------------------------------------------------------------
// Rendering
// ---------------------------------------------------------------------------

// View renders the visible portion of the chat list.
// Only items overlapping the viewport are rendered — off-screen items are skipped.
func (cl *ChatList) View() string {
	if cl.width == 0 || cl.height == 0 {
		return ""
	}

	// Bottom-align: when content is shorter than viewport, render all with top padding
	if cl.totalHeight < cl.height {
		var allLines []string
		for i, item := range cl.items {
			if i > 0 {
				for range itemGap {
					allLines = append(allLines, "")
				}
			}
			allLines = append(allLines, strings.Split(item.Render(cl.width), "\n")...)
		}
		padded := make([]string, cl.height)
		copy(padded[cl.height-len(allLines):], allLines)
		return strings.Join(padded, "\n")
	}

	// Normal case: only render items that overlap [yOffset, yOffset+height)
	visibleStart := cl.yOffset
	visibleEnd := cl.yOffset + cl.height

	var result []string
	linePos := 0

	for i, item := range cl.items {
		// Gap before item (except first)
		if i > 0 {
			for range itemGap {
				if linePos >= visibleStart && linePos < visibleEnd {
					result = append(result, "")
				}
				linePos++
			}
		}

		itemHeight := item.Height()
		itemEndLine := linePos + itemHeight

		// Entirely off-screen — skip render
		if itemEndLine <= visibleStart || linePos >= visibleEnd {
			linePos = itemEndLine
			continue
		}

		// At least partially visible — render
		rendered := item.Render(cl.width)
		for _, line := range strings.Split(rendered, "\n") {
			if linePos >= visibleStart && linePos < visibleEnd {
				result = append(result, line)
			}
			linePos++
		}
	}

	// Pad if we didn't fill the viewport (can happen at very bottom)
	for len(result) < cl.height {
		result = append(result, "")
	}

	return strings.Join(result, "\n")
}

// ---------------------------------------------------------------------------
// Utilities
// ---------------------------------------------------------------------------

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
