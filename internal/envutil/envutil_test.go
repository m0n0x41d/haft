package envutil

import (
	"reflect"
	"testing"
)

func TestStripSingleKey(t *testing.T) {
	env := []string{"PATH=/usr/bin", "ANTHROPIC_API_KEY=sk-x", "HOME=/root"}
	got := Strip(env, "ANTHROPIC_API_KEY")
	want := []string{"PATH=/usr/bin", "HOME=/root"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Strip = %v, want %v", got, want)
	}
}

func TestStripMultipleKeys(t *testing.T) {
	env := []string{"DEV=true", "FORCE_COLOR=1", "TERM=xterm", "HOME=/u"}
	got := Strip(env, "DEV", "FORCE_COLOR", "TERM")
	want := []string{"HOME=/u"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Strip = %v, want %v", got, want)
	}
}

func TestStripDoesNotMatchSharedPrefix(t *testing.T) {
	env := []string{"PATH=/bin", "PATHEXT=.EXE"}
	got := Strip(env, "PATH")
	want := []string{"PATHEXT=.EXE"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Strip matched prefix-related key: got %v", got)
	}
}

func TestStripNoKeysReturnsInput(t *testing.T) {
	env := []string{"A=1", "B=2"}
	got := Strip(env)
	// With no keys, same slice back (identity optimization is fine here).
	if !reflect.DeepEqual(got, env) {
		t.Fatalf("Strip() with no keys = %v, want input %v", got, env)
	}
}

func TestStripKeepsOrder(t *testing.T) {
	env := []string{"A=1", "B=2", "C=3", "D=4"}
	got := Strip(env, "B", "D")
	want := []string{"A=1", "C=3"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("order lost: got %v want %v", got, want)
	}
}
