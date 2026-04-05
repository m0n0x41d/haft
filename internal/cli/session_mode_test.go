package cli

import (
	"testing"

	"github.com/m0n0x41d/haft/internal/agent"
	"github.com/m0n0x41d/haft/internal/protocol"
)

func TestApplyModeUpdateUsesCanonicalExecutionMode(t *testing.T) {
	t.Helper()

	sess := &agent.Session{}
	sess.SetExecutionMode(agent.ExecutionModeSymbiotic)

	updated := applyModeUpdate(sess, protocol.ModeUpdate{Mode: "autonomous"})
	if !updated {
		t.Fatal("applyModeUpdate returned false, want true")
	}
	if sess.ExecutionMode() != agent.ExecutionModeAutonomous {
		t.Fatalf("ExecutionMode = %q, want autonomous", sess.ExecutionMode())
	}
}

func TestApplyModeUpdateIgnoresUnknownMode(t *testing.T) {
	t.Helper()

	sess := &agent.Session{}
	sess.SetExecutionMode(agent.ExecutionModeAutonomous)

	updated := applyModeUpdate(sess, protocol.ModeUpdate{Mode: "unknown"})
	if updated {
		t.Fatal("applyModeUpdate returned true for unknown mode")
	}
	if sess.ExecutionMode() != agent.ExecutionModeAutonomous {
		t.Fatalf("ExecutionMode = %q, want autonomous", sess.ExecutionMode())
	}
}

func TestSessionInfoEmitsCanonicalAndLegacyModeFields(t *testing.T) {
	t.Helper()

	sess := &agent.Session{
		ID:    "sess-1",
		Title: "title",
		Model: "model",
		Yolo:  true,
	}
	sess.SetExecutionMode(agent.ExecutionModeAutonomous)

	info := sessionInfo(sess)
	if info.Mode != "autonomous" {
		t.Fatalf("Mode = %q, want autonomous", info.Mode)
	}
	if info.Interaction != info.Mode {
		t.Fatalf("Interaction = %q, want %q", info.Interaction, info.Mode)
	}
	if !info.Yolo {
		t.Fatal("Yolo = false, want true")
	}
}
