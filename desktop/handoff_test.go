package main

import (
	"strings"
	"testing"
)

func TestBuildHandoffPromptIncludesOriginalBriefAndTail(t *testing.T) {
	source := TaskState{
		ID:           "task-123",
		Title:        "Implement operator tooling",
		Agent:        "claude",
		Project:      "haft",
		Status:       "running",
		Prompt:       "Implement flows, handoff, and terminal tooling.",
		Branch:       "feat/operator-tooling",
		WorktreePath: "/tmp/haft/.haft/worktrees/feat/operator-tooling",
		Output: strings.Join([]string{
			"line one",
			"line two",
			"line three",
		}, "\n"),
	}

	prompt := buildHandoffPrompt(source, "codex")

	expectedSnippets := []string{
		"## Task Handoff: Implement operator tooling",
		"Previous agent: claude",
		"Target agent: codex",
		"## Original Brief",
		"Implement flows, handoff, and terminal tooling.",
		"## Recent Output Tail",
		"line three",
		"still marked running",
	}

	for _, snippet := range expectedSnippets {
		if !strings.Contains(prompt, snippet) {
			t.Fatalf("handoff prompt missing %q:\n%s", snippet, prompt)
		}
	}
}
