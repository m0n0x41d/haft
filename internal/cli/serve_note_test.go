package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/artifact"
)

func TestHandleQuintNoteReturnsArtifactID(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	_, ref, err := handleQuintNote(ctx, store, haftDir, map[string]any{
		"title": "Hybrid recall invalidation",
		"observations": []any{
			"Created notes should invalidate semantic recall in the current MCP session.",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if ref == "" {
		t.Fatal("note handler returned empty createdRef; expected canonical note ID")
	}
	if !strings.HasPrefix(ref, "note-") {
		t.Fatalf("createdRef = %q, want note-*", ref)
	}

	note, err := store.Get(ctx, ref)
	if err != nil {
		t.Fatalf("createdRef %q does not resolve to a stored artifact: %v", ref, err)
	}
	if note.Meta.Kind != artifact.KindNote {
		t.Fatalf("createdRef %q resolved to %s, want Note", ref, note.Meta.Kind)
	}
}

func TestHandleQuintNotePreservesTaskContextAndExplicitValidity(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()
	validUntil := "2026-11-30T12:00:00Z"

	_, ref, err := handleQuintNote(ctx, store, haftDir, map[string]any{
		"title":        "Task-local release observation",
		"task_context": "release-qualification",
		"valid_until":  validUntil,
		"observations": []any{
			"This fact is valid only for the named release qualification window.",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ref, "release-qualification") {
		t.Fatalf("createdRef = %q, want task-context slug", ref)
	}

	note, err := store.Get(ctx, ref)
	if err != nil {
		t.Fatalf("load note %q: %v", ref, err)
	}
	if note.Meta.ValidUntil != validUntil {
		t.Fatalf(
			"note valid_until = %q, want %q",
			note.Meta.ValidUntil,
			validUntil,
		)
	}
}
