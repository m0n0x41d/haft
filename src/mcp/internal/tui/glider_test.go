package tui

import "testing"

func TestGliderCellsUsesExpectedPhase(t *testing.T) {
	got := GliderCells(0)

	want := [3][3]bool{
		{false, true, false},
		{false, false, true},
		{true, true, true},
	}

	if got != want {
		t.Fatalf("unexpected phase 0: %#v", got)
	}
}

func TestGliderCellsWraps(t *testing.T) {
	if GliderCells(4) != GliderCells(0) {
		t.Fatalf("expected frame sequence to wrap after 4 phases")
	}
}
