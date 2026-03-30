package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// testItem is a minimal ChatItem for testing.
type testItem struct {
	baseItem
	selectable bool
}

func newTestItem(text string, selectable bool) *testItem {
	return &testItem{
		baseItem:   newBaseItem(text),
		selectable: selectable,
	}
}

func (t *testItem) Selectable() bool { return t.selectable }

func makeTestList(itemTexts []string, width, height int) ChatList {
	cl := NewChatList(width, height)
	items := make([]ChatItem, len(itemTexts))
	for i, text := range itemTexts {
		items[i] = newTestItem(text, true)
	}
	cl.SetItems(items)
	return cl
}

func TestChatList_ItemAtLine(t *testing.T) {
	// 3 items: 1-line, 2-line, 1-line (with 1-line gaps between them)
	cl := NewChatList(80, 20)
	cl.SetItems([]ChatItem{
		newTestItem("item0", true),           // lines 0 (height=1)
		newTestItem("item1-a\nitem1-b", true), // lines 2-3 (height=2), gap at line 1
		newTestItem("item2", true),           // line 5 (height=1), gap at line 4
	})

	tests := []struct {
		line     int
		wantIdx  int
		wantLocal int
	}{
		{0, 0, 0},   // item 0, first line
		{1, -1, 0},  // gap between item 0 and 1
		{2, 1, 0},   // item 1, first line
		{3, 1, 1},   // item 1, second line
		{4, -1, 0},  // gap between item 1 and 2
		{5, 2, 0},   // item 2
		{6, -1, 0},  // out of bounds
		{-1, -1, 0}, // negative
	}
	for _, tt := range tests {
		idx, local := cl.itemAtLine(tt.line)
		if idx != tt.wantIdx || local != tt.wantLocal {
			t.Errorf("itemAtLine(%d) = (%d, %d), want (%d, %d)",
				tt.line, idx, local, tt.wantIdx, tt.wantLocal)
		}
	}
}

func TestChatList_ScrollBounds(t *testing.T) {
	cl := makeTestList([]string{"a", "b", "c", "d", "e"}, 80, 3)

	// Total: 5 items * 1 line + 4 gaps = 9 lines, viewport = 3
	if cl.totalHeight != 9 {
		t.Fatalf("totalHeight = %d, want 9", cl.totalHeight)
	}

	// Start at bottom (follow mode)
	if !cl.AtBottom() {
		t.Error("should start at bottom in follow mode")
	}

	// Scroll up
	cl.ScrollBy(-3)
	if cl.AtBottom() {
		t.Error("should not be at bottom after scrolling up")
	}

	// Scroll way past top — clamped to 0
	cl.ScrollBy(-100)
	if cl.yOffset != 0 {
		t.Errorf("yOffset should be 0 after scroll past top, got %d", cl.yOffset)
	}

	// Scroll way past bottom — clamped to max
	cl.ScrollBy(1000)
	if !cl.AtBottom() {
		t.Error("should be at bottom after scroll past end")
	}
}

func TestChatList_View_VisibleContent(t *testing.T) {
	cl := NewChatList(80, 3)
	cl.SetItems([]ChatItem{
		newTestItem("first", true),
		newTestItem("second", true),
		newTestItem("third", true),
	})

	// At bottom, viewport=3 lines, total=5 lines (3 items + 2 gaps)
	cl.ScrollToBottom()
	view := cl.View()
	if !strings.Contains(view, "third") {
		t.Errorf("bottom view should contain 'third', got:\n%s", view)
	}

	// Scroll to top
	cl.setYOffset(0)
	view = cl.View()
	if !strings.Contains(view, "first") {
		t.Errorf("top view should contain 'first', got:\n%s", view)
	}
}

func TestChatList_MouseWheel(t *testing.T) {
	cl := makeTestList([]string{"a", "b", "c", "d", "e", "f", "g", "h"}, 80, 5)

	cl.setYOffset(0) // start at top
	before := cl.yOffset
	cl.HandleMouseWheel(tea.MouseWheelDown)
	if cl.yOffset <= before {
		t.Error("mouse wheel down should increase yOffset")
	}

	before = cl.yOffset
	cl.HandleMouseWheel(tea.MouseWheelUp)
	if cl.yOffset >= before {
		t.Error("mouse wheel up should decrease yOffset")
	}
}

func TestChatList_MouseSelection(t *testing.T) {
	cl := NewChatList(80, 20)
	cl.SetItems([]ChatItem{
		newTestItem("hello world", true),
		newTestItem("goodbye world", true),
	})
	cl.setYOffset(0)

	// Mouse down on "hello world" at col 0
	cl.HandleMouseDown(0, 0)
	if cl.mouseState != mouseSelecting {
		t.Fatal("mouseState should be mouseSelecting after mouseDown")
	}

	// Drag to col 5 (selecting "hello")
	cl.HandleMouseDrag(5, 0)

	// Check highlight was applied
	sl, sc, el, ec := cl.items[0].Highlight()
	if sl < 0 || el < 0 {
		t.Fatal("first item should have highlight after drag")
	}
	t.Logf("highlight: (%d,%d)-(%d,%d)", sl, sc, el, ec)

	// Mouse up
	cmd := cl.HandleMouseUp(5, 0)
	if cl.mouseState != mouseSelected {
		t.Error("mouseState should be mouseSelected after mouseUp")
	}
	if cmd == nil {
		t.Error("mouseUp with selection should return a Cmd")
	}
}

func TestChatList_NonSelectableSkipped(t *testing.T) {
	cl := NewChatList(80, 20)
	cl.SetItems([]ChatItem{
		newTestItem("selectable", true),
		newTestItem("---", false), // divider
		newTestItem("also selectable", true),
	})
	cl.setYOffset(0)

	// Click on the non-selectable divider (line 2 = gap after item0 at line 0, divider at line 2)
	cl.HandleMouseDown(0, 2) // line 2 = divider item
	if cl.mouseState == mouseSelecting {
		t.Error("clicking non-selectable item should not start selection")
	}
}

func TestChatList_PageUpDown(t *testing.T) {
	items := make([]string, 20)
	for i := range items {
		items[i] = fmt.Sprintf("item %d", i)
	}
	cl := makeTestList(items, 80, 5)

	cl.setYOffset(0)
	cl.PageDown()
	if cl.yOffset != 5 {
		t.Errorf("PageDown from 0 with height=5: want yOffset=5, got %d", cl.yOffset)
	}

	before := cl.yOffset
	cl.PageUp()
	if cl.yOffset >= before {
		t.Error("PageUp should decrease yOffset")
	}
}
