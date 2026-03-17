package artifact

import (
	"os"
	"strings"
	"testing"
)

func TestParseFile_Basic(t *testing.T) {
	content := `---
id: note-20260316-001
kind: Note
version: 1
status: active
context: auth
mode: note
created_at: 2026-03-16T12:00:00Z
updated_at: 2026-03-16T12:00:00Z
---

# RWMutex for Cache

Using RWMutex instead of channels. Contention <0.1%.
`

	a, err := ParseFile(content)
	if err != nil {
		t.Fatal(err)
	}

	if a.Meta.ID != "note-20260316-001" {
		t.Errorf("id = %q", a.Meta.ID)
	}
	if a.Meta.Kind != KindNote {
		t.Errorf("kind = %q", a.Meta.Kind)
	}
	if a.Meta.Version != 1 {
		t.Errorf("version = %d", a.Meta.Version)
	}
	if a.Meta.Status != StatusActive {
		t.Errorf("status = %q", a.Meta.Status)
	}
	if a.Meta.Context != "auth" {
		t.Errorf("context = %q", a.Meta.Context)
	}
	if a.Meta.Mode != ModeNote {
		t.Errorf("mode = %q", a.Meta.Mode)
	}
	if a.Meta.CreatedAt.IsZero() {
		t.Error("created_at is zero")
	}
	if a.Body == "" {
		t.Error("body is empty")
	}
	if a.Body[0] != '#' {
		t.Errorf("body should start with #, got %q", a.Body[:20])
	}
}

func TestParseFile_HorizontalRuleInBody(t *testing.T) {
	content := `---
id: dec-20260316-001
kind: DecisionRecord
---

# Decision

Some content here.

---

This part must NOT be lost.

More content after the horizontal rule.
`
	a, err := ParseFile(content)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(a.Body, "This part must NOT be lost") {
		t.Error("body was truncated at markdown horizontal rule --- bug")
	}
	if !strings.Contains(a.Body, "More content after") {
		t.Error("content after horizontal rule is missing")
	}
}

func TestParseFile_MultipleHorizontalRules(t *testing.T) {
	content := `---
id: note-001
kind: Note
---

# Title

---

Section 1

---

Section 2

---

Section 3
`
	a, err := ParseFile(content)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(a.Body, "Section 3") {
		t.Error("body missing Section 3 — multiple horizontal rules broke parsing")
	}
}

func TestParseFile_WithLinks(t *testing.T) {
	content := `---
id: sol-20260316-001
kind: SolutionPortfolio
version: 2
status: active
title: Event Options
valid_until: 2026-06-16T00:00:00Z
links:
  - ref: prob-20260316-001
    type: based_on
  - ref: evid-20260316-001
    type: informs
---

# Variants
`

	a, err := ParseFile(content)
	if err != nil {
		t.Fatal(err)
	}

	if len(a.Meta.Links) != 2 {
		t.Fatalf("expected 2 links, got %d", len(a.Meta.Links))
	}
	if a.Meta.Links[0].Ref != "prob-20260316-001" {
		t.Errorf("link[0].ref = %q", a.Meta.Links[0].Ref)
	}
	if a.Meta.Links[0].Type != "based_on" {
		t.Errorf("link[0].type = %q", a.Meta.Links[0].Type)
	}
	if a.Meta.Links[1].Ref != "evid-20260316-001" {
		t.Errorf("link[1].ref = %q", a.Meta.Links[1].Ref)
	}
	if a.Meta.ValidUntil != "2026-06-16T00:00:00Z" {
		t.Errorf("valid_until = %q", a.Meta.ValidUntil)
	}
}

func TestParseFile_NoFrontmatter(t *testing.T) {
	_, err := ParseFile("# Just a markdown file\n\nNo frontmatter here.")
	if err == nil {
		t.Error("expected error for missing frontmatter")
	}
}

func TestParseFile_MissingID(t *testing.T) {
	content := `---
kind: Note
---

Body
`
	_, err := ParseFile(content)
	if err == nil {
		t.Error("expected error for missing id")
	}
}

func TestParseFile_MissingKind(t *testing.T) {
	content := `---
id: test-001
---

Body
`
	_, err := ParseFile(content)
	if err == nil {
		t.Error("expected error for missing kind")
	}
}

func TestRoundTrip(t *testing.T) {
	original := &Artifact{
		Meta: Meta{
			ID:         "dec-20260316-001",
			Kind:       KindDecisionRecord,
			Version:    3,
			Status:     StatusActive,
			Context:    "payments",
			Mode:       ModeStandard,
			Title:      "NATS JetStream",
			ValidUntil: "2026-09-16T00:00:00Z",
			Links: []Link{
				{Ref: "prob-20260316-001", Type: "based_on"},
				{Ref: "sol-20260316-001", Type: "based_on"},
			},
		},
		Body: "# Decision: NATS JetStream\n\nSelected NATS over Kafka.\n",
	}

	dir := t.TempDir()
	path, err := WriteFile(dir, original)
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	parsed, err := ParseFile(string(data))
	if err != nil {
		t.Fatal(err)
	}

	if parsed.Meta.ID != original.Meta.ID {
		t.Errorf("id: %q != %q", parsed.Meta.ID, original.Meta.ID)
	}
	if parsed.Meta.Kind != original.Meta.Kind {
		t.Errorf("kind: %q != %q", parsed.Meta.Kind, original.Meta.Kind)
	}
	if parsed.Meta.Version != original.Meta.Version {
		t.Errorf("version: %d != %d", parsed.Meta.Version, original.Meta.Version)
	}
	if parsed.Meta.Context != original.Meta.Context {
		t.Errorf("context: %q != %q", parsed.Meta.Context, original.Meta.Context)
	}
	if parsed.Meta.Mode != original.Meta.Mode {
		t.Errorf("mode: %q != %q", parsed.Meta.Mode, original.Meta.Mode)
	}
	if parsed.Meta.ValidUntil != original.Meta.ValidUntil {
		t.Errorf("valid_until: %q != %q", parsed.Meta.ValidUntil, original.Meta.ValidUntil)
	}
	if len(parsed.Meta.Links) != len(original.Meta.Links) {
		t.Fatalf("links: %d != %d", len(parsed.Meta.Links), len(original.Meta.Links))
	}
	for i := range parsed.Meta.Links {
		if parsed.Meta.Links[i].Ref != original.Meta.Links[i].Ref {
			t.Errorf("link[%d].ref: %q != %q", i, parsed.Meta.Links[i].Ref, original.Meta.Links[i].Ref)
		}
		if parsed.Meta.Links[i].Type != original.Meta.Links[i].Type {
			t.Errorf("link[%d].type: %q != %q", i, parsed.Meta.Links[i].Type, original.Meta.Links[i].Type)
		}
	}
}
