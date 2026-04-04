package cli

import "testing"

func TestTUIProcessEnvForcesSafeRuntimeDefaults(t *testing.T) {
	base := []string{
		"DEV=true",
		"FORCE_COLOR=0",
		"PATH=/usr/bin",
		"TERM=screen",
	}

	env := tuiProcessEnv(base, "screen-256color")

	if hasEnvEntry(env, "DEV=true") {
		t.Fatalf("DEV=true should be removed from TUI environment")
	}
	if !hasEnvEntry(env, "DEV=false") {
		t.Fatalf("DEV=false missing from TUI environment")
	}
	if !hasEnvEntry(env, "FORCE_COLOR=1") {
		t.Fatalf("FORCE_COLOR=1 missing from TUI environment")
	}
	if !hasEnvEntry(env, "TERM=screen-256color") {
		t.Fatalf("TERM override missing from TUI environment")
	}
}

func hasEnvEntry(env []string, want string) bool {
	for _, entry := range env {
		if entry == want {
			return true
		}
	}

	return false
}
