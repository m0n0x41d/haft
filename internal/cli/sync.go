package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

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

	art, err := artifact.ParseFile(string(data))
	if err != nil {
		return "", fmt.Errorf("parse frontmatter: %w", err)
	}

	if art.Meta.ID == "" || art.Meta.Kind == "" {
		return "", fmt.Errorf("missing id or kind in frontmatter")
	}

	// Check if artifact already exists
	existing, err := store.Get(ctx, art.Meta.ID)
	if err == nil && existing != nil {
		// Exists — check if markdown is newer
		if !art.Meta.UpdatedAt.After(existing.Meta.UpdatedAt) {
			return "unchanged", nil
		}
	}

	if err := artifact.NormalizeProblemStructuredDataForImport(art); err != nil {
		return "", err
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
