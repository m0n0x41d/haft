package tui

// Canonical Conway's Game of Life glider — 4 phases of its natural evolution.
//
//	Phase 0:  ○●○    Phase 1:  ○○●    Phase 2:  ○●○    Phase 3:  ●○○
//	          ○○●              ●○●              ●●○              ○●●
//	          ●●●              ○●●              ○○●              ○●○
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

// GliderCells returns the 3x3 glider state at the given animation frame.
func GliderCells(frame int) [3][3]bool {
	return gliderPhases[frame%len(gliderPhases)]
}
