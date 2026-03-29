package tui

import (
	"strings"
	"testing"
)

func TestNormalizeRange_Forward(t *testing.T) {
	a := Coord{Line: 1, Col: 5}
	b := Coord{Line: 3, Col: 10}
	r := NormalizeRange(a, b)
	if r.Start != a || r.End != b {
		t.Fatalf("forward range should stay as-is: got %+v", r)
	}
}

func TestNormalizeRange_Backward(t *testing.T) {
	a := Coord{Line: 3, Col: 10}
	b := Coord{Line: 1, Col: 5}
	r := NormalizeRange(a, b)
	if r.Start != b || r.End != a {
		t.Fatalf("backward range should swap: got %+v", r)
	}
}

func TestNormalizeRange_SameLineBackward(t *testing.T) {
	a := Coord{Line: 2, Col: 15}
	b := Coord{Line: 2, Col: 3}
	r := NormalizeRange(a, b)
	if r.Start.Col != 3 || r.End.Col != 15 {
		t.Fatalf("same-line backward should swap cols: got %+v", r)
	}
}

func TestSelectionRange_Empty(t *testing.T) {
	r := SelectionRange{Start: Coord{1, 5}, End: Coord{1, 5}}
	if !r.Empty() {
		t.Fatal("identical start/end should be empty")
	}
	r2 := SelectionRange{Start: Coord{1, 5}, End: Coord{1, 6}}
	if r2.Empty() {
		t.Fatal("different start/end should not be empty")
	}
}

func TestRangeForLine(t *testing.T) {
	r := SelectionRange{
		Start: Coord{Line: 2, Col: 5},
		End:   Coord{Line: 4, Col: 10},
	}

	// Before range
	s, e := r.RangeForLine(1, 80)
	if s != -1 || e != -1 {
		t.Fatalf("line before range: want (-1,-1), got (%d,%d)", s, e)
	}

	// Start line
	s, e = r.RangeForLine(2, 80)
	if s != 5 || e != 80 {
		t.Fatalf("start line: want (5,80), got (%d,%d)", s, e)
	}

	// Middle line
	s, e = r.RangeForLine(3, 80)
	if s != 0 || e != 80 {
		t.Fatalf("middle line: want (0,80), got (%d,%d)", s, e)
	}

	// End line
	s, e = r.RangeForLine(4, 80)
	if s != 0 || e != 10 {
		t.Fatalf("end line: want (0,10), got (%d,%d)", s, e)
	}

	// After range
	s, e = r.RangeForLine(5, 80)
	if s != -1 || e != -1 {
		t.Fatalf("line after range: want (-1,-1), got (%d,%d)", s, e)
	}
}

func TestWordAt(t *testing.T) {
	tests := []struct {
		line      string
		col       int
		wantStart int
		wantEnd   int
	}{
		{"hello world", 2, 0, 5},         // middle of "hello"
		{"hello world", 0, 0, 5},         // start of "hello"
		{"hello world", 4, 0, 5},         // end of "hello"
		{"hello world", 6, 6, 11},        // start of "world"
		{"hello world", 5, 5, 5},         // on space
		{"foo(bar)", 4, 4, 7},            // "bar" inside parens
		{"", 0, 0, 0},                    // empty
		{"hello", 10, 10, 10},            // out of bounds
		{"  hello  ", 0, 0, 0},           // on leading space
		{"one-two-three", 5, 0, 13},      // hyphens are word chars
	}
	for _, tt := range tests {
		s, e := WordAt(tt.line, tt.col)
		if s != tt.wantStart || e != tt.wantEnd {
			t.Errorf("WordAt(%q, %d) = (%d,%d), want (%d,%d)",
				tt.line, tt.col, s, e, tt.wantStart, tt.wantEnd)
		}
	}
}

func TestExtractText_PlainContent(t *testing.T) {
	content := "line zero\nline one\nline two\nline three"
	sel := SelectionRange{
		Start: Coord{Line: 1, Col: 5},
		End:   Coord{Line: 2, Col: 4},
	}
	got := ExtractText(content, 40, sel)

	// Should get "one" from line 1 (cols 5-8) and "line" from line 2 (cols 0-3)
	if !strings.Contains(got, "one") {
		t.Errorf("expected 'one' in extracted text, got %q", got)
	}
	if !strings.Contains(got, "line") {
		t.Errorf("expected 'line' in extracted text, got %q", got)
	}
}

func TestExtractText_EmptySelection(t *testing.T) {
	content := "hello world"
	sel := SelectionRange{
		Start: Coord{Line: 0, Col: 3},
		End:   Coord{Line: 0, Col: 3},
	}
	got := ExtractText(content, 40, sel)
	if got != "" {
		t.Errorf("empty selection should return empty string, got %q", got)
	}
}

func TestApplyHighlight_ReturnsNonEmpty(t *testing.T) {
	content := "hello world"
	sel := SelectionRange{
		Start: Coord{Line: 0, Col: 0},
		End:   Coord{Line: 0, Col: 5},
	}
	got := ApplyHighlight(content, 40, sel)
	if got == "" {
		t.Fatal("ApplyHighlight should return non-empty string")
	}
	// The output should contain ANSI reverse sequences
	if !strings.Contains(got, "\x1b[") {
		t.Error("highlighted output should contain ANSI escape sequences")
	}
}

func TestApplyHighlight_EmptySelection_Passthrough(t *testing.T) {
	content := "hello world"
	sel := SelectionRange{}
	got := ApplyHighlight(content, 40, sel)
	if got != content {
		t.Errorf("empty selection should return content unchanged, got %q", got)
	}
}
