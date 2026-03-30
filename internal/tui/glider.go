package tui

// ---------------------------------------------------------------------------
// 3x3 cell animations for the status bar indicator.
//
// Each animation is a sequence of [3][3]bool frames.
// The status bar picks the animation based on current state
// and advances frames on gliderTick.
// ---------------------------------------------------------------------------

// Animation selects which pattern plays in the status indicator.
type Animation int

const (
	AnimGlider   Animation = iota // Conway's glider — streaming/thinking
	AnimOrbit                     // single dot circling perimeter — tool execution
	AnimConverge                  // corners→center — compaction
	AnimPulse                     // center radiates outward — waiting for user
	AnimStatic                    // frozen first glider frame — idle
)

// AnimationCells returns the 3x3 grid for the given animation at the given frame.
func AnimationCells(anim Animation, frame int) [3][3]bool {
	switch anim {
	case AnimGlider:
		return gliderPhases[frame%len(gliderPhases)]
	case AnimOrbit:
		return orbitPhases[frame%len(orbitPhases)]
	case AnimConverge:
		return convergePhases[frame%len(convergePhases)]
	case AnimPulse:
		return pulsePhases[frame%len(pulsePhases)]
	default:
		return gliderPhases[0]
	}
}

// AnimationLen returns the number of frames in the animation.
func AnimationLen(anim Animation) int {
	switch anim {
	case AnimGlider:
		return len(gliderPhases)
	case AnimOrbit:
		return len(orbitPhases)
	case AnimConverge:
		return len(convergePhases)
	case AnimPulse:
		return len(pulsePhases)
	default:
		return 1
	}
}

// GliderCells returns the 3x3 glider state at the given animation frame.
// Kept for backward compatibility — prefer AnimationCells.
func GliderCells(frame int) [3][3]bool {
	return AnimationCells(AnimGlider, frame)
}

// ---------------------------------------------------------------------------
// Glider — Conway's Game of Life canonical pattern (4 phases)
//
//	Phase 0:  ○●○    Phase 1:  ○○●    Phase 2:  ○●○    Phase 3:  ●○○
//	          ○○●              ●○●              ●●○              ○●●
//	          ●●●              ○●●              ○○●              ○●○
// ---------------------------------------------------------------------------

var gliderPhases = [4][3][3]bool{
	{
		{false, true, false},
		{false, false, true},
		{true, true, true},
	},
	{
		{false, false, true},
		{true, false, true},
		{false, true, true},
	},
	{
		{false, true, false},
		{true, true, false},
		{false, false, true},
	},
	{
		{true, false, false},
		{false, true, true},
		{false, true, false},
	},
}

// ---------------------------------------------------------------------------
// Orbit — single dot tracing the perimeter clockwise (8 phases)
//
//	0: ●○○  1: ○●○  2: ○○●  3: ○○○  4: ○○○  5: ○○○  6: ○○○  7: ○○○
//	   ○○○     ○○○     ○○○     ○○●     ○○○     ○○○     ○○○     ●○○
//	   ○○○     ○○○     ○○○     ○○○     ○○●     ○●○     ●○○     ○○○
// ---------------------------------------------------------------------------

var orbitPhases = [8][3][3]bool{
	{{true, false, false}, {false, false, false}, {false, false, false}}, // top-left
	{{false, true, false}, {false, false, false}, {false, false, false}}, // top-center
	{{false, false, true}, {false, false, false}, {false, false, false}}, // top-right
	{{false, false, false}, {false, false, true}, {false, false, false}}, // mid-right
	{{false, false, false}, {false, false, false}, {false, false, true}}, // bottom-right
	{{false, false, false}, {false, false, false}, {false, true, false}}, // bottom-center
	{{false, false, false}, {false, false, false}, {true, false, false}}, // bottom-left
	{{false, false, false}, {true, false, false}, {false, false, false}}, // mid-left
}

// ---------------------------------------------------------------------------
// Converge — corners collapse to center, then expand back (6 phases)
//
//	0: ●○●  1: ○○○  2: ○○○  3: ○○○  4: ○○○  5: ●○●
//	   ○○○     ○○○     ○●○     ○●○     ○○○     ○○○
//	   ●○●     ○○○     ○○○     ○○○     ○○○     ●○●
//
// Phase 0,5: corners lit. Phase 2,3: center lit. Phases 1,4: all dark (transition).
// ---------------------------------------------------------------------------

var convergePhases = [6][3][3]bool{
	// corners
	{{true, false, true}, {false, false, false}, {true, false, true}},
	// fade out
	{{false, false, false}, {false, false, false}, {false, false, false}},
	// center appears
	{{false, false, false}, {false, true, false}, {false, false, false}},
	// center holds
	{{false, false, false}, {false, true, false}, {false, false, false}},
	// fade out
	{{false, false, false}, {false, false, false}, {false, false, false}},
	// corners return
	{{true, false, true}, {false, false, false}, {true, false, true}},
}

// ---------------------------------------------------------------------------
// Pulse — center radiates outward, then contracts (6 phases)
//
//	0: ○○○  1: ○○○  2: ○●○  3: ●●●  4: ○●○  5: ○○○
//	   ○●○     ○●○     ●●●     ●○●     ●●●     ○●○
//	   ○○○     ○○○     ○●○     ●●●     ○●○     ○○○
//
// Center → cross → ring → cross → center → dot
// ---------------------------------------------------------------------------

var pulsePhases = [6][3][3]bool{
	// dot
	{{false, false, false}, {false, true, false}, {false, false, false}},
	// dot holds
	{{false, false, false}, {false, true, false}, {false, false, false}},
	// cross
	{{false, true, false}, {true, true, true}, {false, true, false}},
	// ring (full border, empty center)
	{{true, true, true}, {true, false, true}, {true, true, true}},
	// cross
	{{false, true, false}, {true, true, true}, {false, true, false}},
	// back to dot
	{{false, false, false}, {false, true, false}, {false, false, false}},
}
