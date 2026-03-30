package cli

import (
	"os"
	"strings"
)

// bubbleTeaEnv returns environment variables for Bubble Tea capability probing.
// Under tmux-like sessions we suppress terminal capability heuristics that can
// trigger Unicode Core / synchronized output negotiation and corrupt rendering.
func bubbleTeaEnv() []string {
	env := os.Environ()
	if !inTmuxLikeEnv(env) {
		return env
	}

	out := make([]string, 0, len(env))
	for _, kv := range env {
		switch {
		case strings.HasPrefix(kv, "TERM_PROGRAM="):
			continue
		case strings.HasPrefix(kv, "WT_SESSION="):
			continue
		case strings.HasPrefix(kv, "TERM="):
			out = append(out, "TERM=screen")
		default:
			out = append(out, kv)
		}
	}
	return out
}

func inTmuxLikeEnv(env []string) bool {
	for _, kv := range env {
		if strings.HasPrefix(kv, "TMUX=") || strings.HasPrefix(kv, "TERM=screen") || strings.HasPrefix(kv, "TERM=tmux") {
			return true
		}
	}
	return false
}
