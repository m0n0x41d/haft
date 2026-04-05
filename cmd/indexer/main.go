package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/fpf"
	_ "modernc.org/sqlite"
)

const routeArtifactPath = ".context/fpf-routes.json"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 {
		return fmt.Errorf("usage: indexer <FPF-Spec.md> [output.db] [fpf-commit-sha]")
	}

	specPath := os.Args[1]
	dbPath := filepath.Join("internal", "cli", "fpf.db")
	if len(os.Args) >= 3 {
		dbPath = os.Args[2]
	}
	commitSHA := ""
	if len(os.Args) >= 4 {
		commitSHA = os.Args[3]
	}

	return buildIndex(specPath, dbPath, commitSHA, routeArtifactPath)
}

func buildIndex(specPath, dbPath, commitSHA, routePath string) error {
	corpus, err := fpf.LoadSpecIndexCorpus(specPath)
	if err != nil {
		return fmt.Errorf("load production spec corpus: %w", err)
	}

	routes, err := fpf.LoadRoutes(routePath)
	if err != nil {
		return fmt.Errorf("loading routes: %w", err)
	}

	if err := fpf.BuildSpecIndex(dbPath, corpus.Indexed, routes); err != nil {
		return fmt.Errorf("building index: %w", err)
	}

	metadata := buildSpecIndexMetadata(specPath, len(corpus.Indexed), commitSHA, time.Now().UTC())
	if err := fpf.SetSpecMetaEntries(dbPath, metadata); err != nil {
		return fmt.Errorf("setting meta: %w", err)
	}

	fmt.Printf("Indexed %d chunks (from %d total) into %s\n", len(corpus.Indexed), len(corpus.Chunks), dbPath)
	return nil
}

func buildSpecIndexMetadata(specPath string, indexedSections int, explicitCommit string, buildTime time.Time) map[string]string {
	return map[string]string{
		"fpf_commit":       resolveSpecCommit(explicitCommit, specPath),
		"indexed_sections": fmt.Sprintf("%d", indexedSections),
		"build_time":       buildTime.UTC().Format(time.RFC3339),
		"spec_path":        filepath.Clean(specPath),
		"schema_version":   fpf.SpecIndexSchemaVersion,
	}
}

func resolveSpecCommit(explicitCommit, specPath string) string {
	commit := strings.TrimSpace(explicitCommit)
	if commit != "" {
		return commit
	}

	return detectSpecCommit(specPath)
}

func detectSpecCommit(specPath string) string {
	gitDir, err := specGitLookupDir(specPath)
	if err != nil {
		return ""
	}

	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = gitDir

	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(output))
}

func specGitLookupDir(specPath string) (string, error) {
	absPath, err := filepath.Abs(specPath)
	if err != nil {
		return "", err
	}

	return filepath.Dir(absPath), nil
}
