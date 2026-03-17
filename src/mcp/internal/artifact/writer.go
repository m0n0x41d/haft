package artifact

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WriteFile writes an artifact as a markdown file with YAML frontmatter
// to the appropriate .quint/ subdirectory. Creates the directory if needed.
func WriteFile(quintDir string, a *Artifact) (string, error) {
	dir := filepath.Join(quintDir, a.Meta.Kind.Dir())
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create dir %s: %w", dir, err)
	}

	filename := a.Meta.ID + ".md"
	path := filepath.Join(dir, filename)

	content := renderArtifact(a)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("write %s: %w", path, err)
	}

	return path, nil
}

// writeFileQuiet writes the artifact file, logging but not propagating errors.
// Use when the DB write already succeeded and the file is a secondary projection.
func writeFileQuiet(quintDir string, a *Artifact) {
	if _, err := WriteFile(quintDir, a); err != nil {
		// Log to stderr since this is a non-fatal projection failure
		fmt.Fprintf(os.Stderr, "warning: file write failed for %s: %v\n", a.Meta.ID, err)
	}
}

func renderArtifact(a *Artifact) string {
	var sb strings.Builder

	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("id: %s\n", a.Meta.ID))
	sb.WriteString(fmt.Sprintf("kind: %s\n", a.Meta.Kind))
	sb.WriteString(fmt.Sprintf("version: %d\n", a.Meta.Version))
	sb.WriteString(fmt.Sprintf("status: %s\n", a.Meta.Status))
	if a.Meta.Context != "" {
		sb.WriteString(fmt.Sprintf("context: %s\n", a.Meta.Context))
	}
	if a.Meta.Mode != "" {
		sb.WriteString(fmt.Sprintf("mode: %s\n", a.Meta.Mode))
	}
	if a.Meta.ValidUntil != "" {
		sb.WriteString(fmt.Sprintf("valid_until: %s\n", a.Meta.ValidUntil))
	}
	sb.WriteString(fmt.Sprintf("created_at: %s\n", a.Meta.CreatedAt.Format("2006-01-02T15:04:05Z")))
	sb.WriteString(fmt.Sprintf("updated_at: %s\n", a.Meta.UpdatedAt.Format("2006-01-02T15:04:05Z")))
	if len(a.Meta.Links) > 0 {
		sb.WriteString("links:\n")
		for _, l := range a.Meta.Links {
			sb.WriteString(fmt.Sprintf("  - ref: %s\n", l.Ref))
			sb.WriteString(fmt.Sprintf("    type: %s\n", l.Type))
		}
	}
	sb.WriteString("---\n\n")
	sb.WriteString(a.Body)
	if !strings.HasSuffix(a.Body, "\n") {
		sb.WriteString("\n")
	}

	return sb.String()
}
