package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

	f, err := os.Open(specPath)
	if err != nil {
		return fmt.Errorf("opening spec: %w", err)
	}
	defer func() { _ = f.Close() }()

	chunks, err := fpf.ChunkMarkdown(f)
	if err != nil {
		return fmt.Errorf("chunking: %w", err)
	}

	var filtered []fpf.SpecChunk
	for _, c := range chunks {
		if len(strings.TrimSpace(c.Body)) > 20 {
			filtered = append(filtered, c)
		}
	}

	if err := fpf.BuildSpecIndex(dbPath, filtered); err != nil {
		return fmt.Errorf("building index: %w", err)
	}

	if commitSHA != "" {
		if err := fpf.SetSpecMeta(dbPath, "fpf_commit", commitSHA); err != nil {
			return fmt.Errorf("setting meta: %w", err)
		}
	}

	fmt.Printf("Indexed %d chunks (from %d total) into %s\n", len(filtered), len(chunks), dbPath)
	return nil
}
