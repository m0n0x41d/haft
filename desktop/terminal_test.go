package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTerminalSessionLifecycle(t *testing.T) {
	projectRoot := t.TempDir()

	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	app := NewApp()
	app.projectRoot = projectRoot
	app.terminals = newTerminalManager(app)

	session, err := app.CreateTerminalSession(projectRoot)
	if err != nil {
		t.Fatalf("CreateTerminalSession: %v", err)
	}

	sessions, err := app.ListTerminalSessions()
	if err != nil {
		t.Fatalf("ListTerminalSessions: %v", err)
	}

	if len(sessions) != 1 {
		t.Fatalf("expected 1 terminal session, got %d", len(sessions))
	}

	if err := app.WriteTerminalInput(session.ID, "echo desktop-terminal-test\r"); err != nil {
		t.Fatalf("WriteTerminalInput: %v", err)
	}

	if err := app.ResizeTerminalSession(session.ID, 100, 30); err != nil {
		t.Fatalf("ResizeTerminalSession: %v", err)
	}

	if err := app.CloseTerminalSession(session.ID); err != nil {
		t.Fatalf("CloseTerminalSession: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		sessions, err = app.ListTerminalSessions()
		if err != nil {
			t.Fatalf("ListTerminalSessions after close: %v", err)
		}

		if len(sessions) == 0 {
			return
		}

		if time.Now().After(deadline) {
			t.Fatalf("expected terminal session to close, got %#v", sessions)
		}

		time.Sleep(25 * time.Millisecond)
	}
}

func TestDefaultShellPathHonorsShellEnv(t *testing.T) {
	shellPath := filepath.Join(t.TempDir(), "custom-shell")

	if err := os.WriteFile(shellPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	t.Setenv("SHELL", shellPath)

	got, err := defaultShellPath()
	if err != nil {
		t.Fatalf("defaultShellPath: %v", err)
	}

	if got != shellPath {
		t.Fatalf("expected shell path %q, got %q", shellPath, got)
	}
}
