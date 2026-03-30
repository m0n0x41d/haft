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

func TestAnimationCellsAllTypes(t *testing.T) {
	anims := []struct {
		name Animation
		len  int
	}{
		{AnimGlider, 4},
		{AnimOrbit, 8},
		{AnimConverge, 6},
		{AnimPulse, 6},
	}

	for _, a := range anims {
		if AnimationLen(a.name) != a.len {
			t.Errorf("animation %d: expected %d frames, got %d", a.name, a.len, AnimationLen(a.name))
		}
		// Verify wraparound
		first := AnimationCells(a.name, 0)
		wrapped := AnimationCells(a.name, a.len)
		if first != wrapped {
			t.Errorf("animation %d: frame 0 != frame %d (wraparound broken)", a.name, a.len)
		}
	}
}

func TestAnimationStaticReturnsFirstGlider(t *testing.T) {
	got := AnimationCells(AnimStatic, 42)
	want := GliderCells(0)
	if got != want {
		t.Fatal("AnimStatic should always return glider phase 0")
	}
}

func TestOrbitVisitsAllPerimeterCells(t *testing.T) {
	// Each orbit frame should have exactly one cell lit
	for frame := range 8 {
		grid := AnimationCells(AnimOrbit, frame)
		count := 0
		for _, row := range grid {
			for _, cell := range row {
				if cell {
					count++
				}
			}
		}
		if count != 1 {
			t.Errorf("orbit frame %d: expected 1 lit cell, got %d", frame, count)
		}
	}
}

func TestPulseSymmetry(t *testing.T) {
	// Pulse should be vertically and horizontally symmetric in every frame
	for frame := range 6 {
		grid := AnimationCells(AnimPulse, frame)
		// horizontal symmetry: row[i] == row[2-i]
		if grid[0] != grid[2] {
			t.Errorf("pulse frame %d: not vertically symmetric", frame)
		}
		// each row symmetric: col[0] == col[2]
		for r := range 3 {
			if grid[r][0] != grid[r][2] {
				t.Errorf("pulse frame %d row %d: not horizontally symmetric", frame, r)
			}
		}
	}
}
