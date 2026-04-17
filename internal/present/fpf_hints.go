package present

import "github.com/m0n0x41d/haft/internal/fpf"

// FPFPhaseHint returns a compact nudge for the given reasoning phase,
// reminding the agent which FPF patterns are relevant and how to retrieve them.
// The hint content is derived from embedded pattern files at first call; there
// is no hardcoded map. Renaming a pattern heading in the .md file propagates
// automatically. Returns empty string for non-reasoning or unknown phases.
func FPFPhaseHint(phase string) string {
	return fpf.PhaseHint(phase)
}
