package cmd

import "testing"

func TestBubbleTeaEnvKeepsNormalEnvironment(t *testing.T) {
	in := []string{"TERM=xterm-256color", "TERM_PROGRAM=WezTerm", "HOME=/tmp"}
	if inTmuxLikeEnv(in) {
		t.Fatal("expected non-tmux environment")
	}
}

func TestInTmuxLikeEnvDetectsTmuxMarkers(t *testing.T) {
	cases := [][]string{
		{"TMUX=/tmp/tmux-1000/default,123,0", "TERM=xterm-256color"},
		{"TERM=screen-256color"},
		{"TERM=tmux-256color"},
	}
	for _, env := range cases {
		if !inTmuxLikeEnv(env) {
			t.Fatalf("expected tmux-like environment for %v", env)
		}
	}
}
