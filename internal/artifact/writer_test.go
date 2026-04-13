package artifact

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWriteFile(t *testing.T) {
	haftDir := t.TempDir()

	a := &Artifact{
		Meta: Meta{
			ID:        "note-20260316-001",
			Kind:      KindNote,
			Version:   1,
			Status:    StatusActive,
			Context:   "auth",
			Mode:      ModeNote,
			Title:     "RWMutex for cache",
			CreatedAt: time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC),
			UpdatedAt: time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC),
		},
		Body: "Using RWMutex instead of channels.\nContention <0.1%.\n",
	}

	path, err := WriteFile(haftDir, a)
	if err != nil {
		t.Fatal(err)
	}

	expectedDir := filepath.Join(haftDir, "notes")
	if _, err := os.Stat(expectedDir); os.IsNotExist(err) {
		t.Error("notes/ directory not created")
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	s := string(content)

	if !strings.Contains(s, "---\n") {
		t.Error("missing frontmatter delimiters")
	}
	if !strings.Contains(s, "id: note-20260316-001") {
		t.Error("missing id in frontmatter")
	}
	if !strings.Contains(s, "kind: Note") {
		t.Error("missing kind in frontmatter")
	}
	if !strings.Contains(s, "context: auth") {
		t.Error("missing context in frontmatter")
	}
	if !strings.Contains(s, "mode: note") {
		t.Error("missing mode in frontmatter")
	}
	if !strings.Contains(s, "Using RWMutex instead of channels.") {
		t.Error("missing body content")
	}
}

func TestWriteFileWithLinks(t *testing.T) {
	haftDir := t.TempDir()

	a := &Artifact{
		Meta: Meta{
			ID:        "sol-20260316-001",
			Kind:      KindSolutionPortfolio,
			Version:   1,
			Status:    StatusActive,
			Title:     "Event Infrastructure Options",
			CreatedAt: time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC),
			UpdatedAt: time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC),
			Links: []Link{
				{Ref: "prob-20260316-001", Type: "based_on"},
			},
		},
		Body: "# Variants\n\n## NATS JetStream\n...\n",
	}

	path, err := WriteFile(haftDir, a)
	if err != nil {
		t.Fatal(err)
	}

	content, _ := os.ReadFile(path)
	s := string(content)

	if !strings.Contains(s, "links:") {
		t.Error("missing links section")
	}
	if !strings.Contains(s, "ref: prob-20260316-001") {
		t.Error("missing link ref")
	}
	if !strings.Contains(s, "type: based_on") {
		t.Error("missing link type")
	}
}

func TestWriteFileCreatesSubdirectory(t *testing.T) {
	haftDir := t.TempDir()

	kinds := []Kind{KindNote, KindProblemCard, KindSolutionPortfolio, KindDecisionRecord, KindEvidencePack, KindRefreshReport}

	for _, kind := range kinds {
		a := &Artifact{
			Meta: Meta{
				ID:        GenerateID(kind, 1),
				Kind:      kind,
				Title:     "Test",
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			Body: "test body",
		}

		_, err := WriteFile(haftDir, a)
		if err != nil {
			t.Errorf("WriteFile for %s: %v", kind, err)
		}

		dir := filepath.Join(haftDir, kind.Dir())
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			t.Errorf("directory %s not created for kind %s", kind.Dir(), kind)
		}
	}
}
