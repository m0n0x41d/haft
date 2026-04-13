package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/project"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync .haft/ markdown files into local SQLite database",
	Long: `Reads all .haft/*.md files (problems, decisions, portfolios, notes, etc.)
and upserts them into the local SQLite database.

Use this after git pull when working in a team — each team member has
their own SQLite database, and .haft/ markdown files in git are the
shared source of truth.

Workflow:
  1. Engineer A creates a decision → .haft/decisions/dec-001.md appears
  2. git commit && git push
  3. Engineer B does git pull → sees new .md file
  4. haft sync → their local SQLite is updated
  5. Both engineers see the same decisions in haft status / haft board`,
	RunE: runSync,
}

func init() {
	rootCmd.AddCommand(syncCmd)
}

// frontmatter represents the YAML frontmatter of a .haft/*.md file.
type frontmatter struct {
	ID        string    `yaml:"id"`
	Kind      string    `yaml:"kind"`
	Version   int       `yaml:"version"`
	Status    string    `yaml:"status"`
	Title     string    `yaml:"title"`
	Mode      string    `yaml:"mode"`
	Context   string    `yaml:"context"`
	ValidUntil string  `yaml:"valid_until"`
	CreatedAt time.Time `yaml:"created_at"`
	UpdatedAt time.Time `yaml:"updated_at"`
	Links     []struct {
		Ref  string `yaml:"ref"`
		Type string `yaml:"type"`
	} `yaml:"links"`
}

func runSync(cmd *cobra.Command, args []string) error {
	projectRoot, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("not a haft project (no .haft/ directory found): %w", err)
	}

	haftDir := filepath.Join(projectRoot, ".haft")

	projCfg, err := project.Load(haftDir)
	if err != nil {
		return fmt.Errorf("load project config: %w", err)
	}
	if projCfg == nil {
		return fmt.Errorf("project not initialized — run 'haft init' first")
	}

	dbPath, err := projCfg.DBPath()
	if err != nil {
		return fmt.Errorf("get DB path: %w", err)
	}

	database, err := db.NewStore(dbPath)
	if err != nil {
		return fmt.Errorf("open DB: %w", err)
	}
	defer database.Close()

	store := artifact.NewStore(database.GetRawDB())
	ctx := context.Background()

	// Scan all .haft/ subdirectories for .md files
	dirs := []string{"problems", "decisions", "solutions", "notes", "evidence", "refresh"}
	var synced, skipped, failed int

	for _, dir := range dirs {
		dirPath := filepath.Join(haftDir, dir)
		entries, err := os.ReadDir(dirPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("read %s: %w", dirPath, err)
		}

		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
				continue
			}

			filePath := filepath.Join(dirPath, entry.Name())
			result, err := syncOneFile(ctx, store, filePath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "  FAIL %s: %v\n", entry.Name(), err)
				failed++
				continue
			}

			switch result {
			case "created":
				fmt.Printf("  + %s\n", entry.Name())
				synced++
			case "updated":
				fmt.Printf("  ~ %s\n", entry.Name())
				synced++
			case "unchanged":
				skipped++
			}
		}
	}

	fmt.Printf("\nSync complete: %d synced, %d unchanged, %d failed\n", synced, skipped, failed)
	return nil
}

func syncOneFile(ctx context.Context, store *artifact.Store, filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	fm, body, err := parseFrontmatter(string(data))
	if err != nil {
		return "", fmt.Errorf("parse frontmatter: %w", err)
	}

	if fm.ID == "" || fm.Kind == "" {
		return "", fmt.Errorf("missing id or kind in frontmatter")
	}

	// Check if artifact already exists
	existing, err := store.Get(ctx, fm.ID)
	if err == nil && existing != nil {
		// Exists — check if markdown is newer
		if !fm.UpdatedAt.After(existing.Meta.UpdatedAt) {
			return "unchanged", nil
		}
	}

	// Build artifact from frontmatter + body
	art := &artifact.Artifact{
		Meta: artifact.Meta{
			ID:         fm.ID,
			Kind:       artifact.Kind(fm.Kind),
			Version:    fm.Version,
			Status:     artifact.Status(fm.Status),
			Context:    fm.Context,
			Mode:       artifact.Mode(fm.Mode),
			Title:      fm.Title,
			ValidUntil: fm.ValidUntil,
			CreatedAt:  fm.CreatedAt,
			UpdatedAt:  fm.UpdatedAt,
		},
		Body: body,
	}

	for _, link := range fm.Links {
		art.Meta.Links = append(art.Meta.Links, artifact.Link{
			Ref:  link.Ref,
			Type: link.Type,
		})
	}

	// Upsert: create or update
	if existing == nil {
		err = store.Create(ctx, art)
		if err != nil {
			// If create fails (already exists from race), try update
			if strings.Contains(err.Error(), "already exists") {
				err = store.Update(ctx, art)
				if err != nil {
					return "", err
				}
				return "updated", nil
			}
			return "", err
		}
		return "created", nil
	}

	err = store.Update(ctx, art)
	if err != nil {
		return "", err
	}
	return "updated", nil
}

// parseFrontmatter extracts YAML frontmatter and body from a markdown file.
// Expects: ---\n<yaml>\n---\n<body>
func parseFrontmatter(content string) (frontmatter, string, error) {
	var fm frontmatter

	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, "---") {
		return fm, content, fmt.Errorf("no frontmatter found (must start with ---)")
	}

	// Find closing ---
	rest := content[3:]
	if idx := strings.Index(rest, "\n---"); idx >= 0 {
		yamlBlock := rest[:idx]
		body := strings.TrimSpace(rest[idx+4:])

		// Fix unquoted YAML values containing colons — common in haft-generated titles.
		// The title field often contains "Solutions for: ..." which breaks YAML parsing.
		yamlBlock = fixUnquotedYAMLValues(yamlBlock)

		if err := yaml.Unmarshal([]byte(yamlBlock), &fm); err != nil {
			return fm, "", fmt.Errorf("invalid YAML: %w", err)
		}

		return fm, body, nil
	}

	return fm, "", fmt.Errorf("no closing --- found for frontmatter")
}

// fixUnquotedYAMLValues quotes values on known problematic fields (title, context)
// that may contain colons, which break YAML parsing when unquoted.
func fixUnquotedYAMLValues(yamlBlock string) string {
	lines := strings.Split(yamlBlock, "\n")
	quotableFields := map[string]bool{
		"title":      true,
		"context":    true,
		"valid_until": false, // timestamps with colons are fine in YAML
	}

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "-") {
			continue
		}

		colonIdx := strings.Index(trimmed, ":")
		if colonIdx <= 0 {
			continue
		}

		key := strings.TrimSpace(trimmed[:colonIdx])
		if !quotableFields[key] {
			continue
		}

		value := strings.TrimSpace(trimmed[colonIdx+1:])
		if value == "" || strings.HasPrefix(value, "'") || strings.HasPrefix(value, "\"") {
			continue
		}

		// Check if value contains a colon (the problematic case)
		if strings.Contains(value, ":") {
			// Quote the value
			escaped := strings.ReplaceAll(value, "\"", "\\\"")
			lines[i] = key + ": \"" + escaped + "\""
		}
	}

	return strings.Join(lines, "\n")
}
