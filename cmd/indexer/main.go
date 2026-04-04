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

	catalogFile, err := os.Open(specPath)
	if err != nil {
		return fmt.Errorf("opening spec for catalog parse: %w", err)
	}
	catalog, err := fpf.ParseSpecCatalog(catalogFile)
	_ = catalogFile.Close()
	if err != nil {
		return fmt.Errorf("parse catalog: %w", err)
	}

	f, err := os.Open(specPath)
	if err != nil {
		return fmt.Errorf("opening spec: %w", err)
	}
	defer func() { _ = f.Close() }()

	chunks, err := fpf.ChunkMarkdown(f)
	if err != nil {
		return fmt.Errorf("chunking: %w", err)
	}
	chunks = fpf.EnrichChunks(chunks, catalog)

	var filtered []fpf.SpecChunk
	for _, c := range chunks {
		if len(strings.TrimSpace(c.Body)) > 20 {
			filtered = append(filtered, c)
		}
	}

	if err := fpf.BuildSpecIndex(dbPath, filtered); err != nil {
		return fmt.Errorf("building index: %w", err)
	}

	metadata := buildSpecIndexMetadata(specPath, len(filtered), commitSHA, time.Now().UTC())
	if err := fpf.SetSpecMetaEntries(dbPath, metadata); err != nil {
		return fmt.Errorf("setting meta: %w", err)
	}

	fmt.Printf("Indexed %d chunks (from %d total) into %s\n", len(filtered), len(chunks), dbPath)
	return nil
}

func buildSpecIndexMetadata(specPath string, indexedSections int, explicitCommit string, buildTime time.Time) map[string]string {
	return map[string]string{
		"fpf_commit":       resolveSpecCommit(specPath, explicitCommit),
		"indexed_sections": fmt.Sprintf("%d", indexedSections),
		"build_time":       buildTime.UTC().Format(time.RFC3339),
		"spec_path":        filepath.Clean(specPath),
		"schema_version":   fpf.SpecIndexSchemaVersion,
	}
}

func resolveSpecCommit(specPath, explicitCommit string) string {
	if commit := strings.TrimSpace(explicitCommit); commit != "" {
		return commit
	}

	output, err := exec.Command("git", "-C", filepath.Dir(specPath), "rev-parse", "HEAD").Output()
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(output))
}
