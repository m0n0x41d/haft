package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/m0n0x41d/quint-code/internal/fpf"
	_ "modernc.org/sqlite"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: indexer <FPF-Spec.md> [output.db] [fpf-commit-sha]\n")
		os.Exit(1)
	}

	specPath := os.Args[1]
	dbPath := filepath.Join("src", "mcp", "cmd", "fpf.db")
	if len(os.Args) >= 3 {
		dbPath = os.Args[2]
	}
	commitSHA := ""
	if len(os.Args) >= 4 {
		commitSHA = os.Args[3]
	}

	f, err := os.Open(specPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening spec: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	chunks, err := fpf.ChunkMarkdown(f)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error chunking: %v\n", err)
		os.Exit(1)
	}

	var filtered []fpf.SpecChunk
	for _, c := range chunks {
		if len(strings.TrimSpace(c.Body)) > 20 {
			filtered = append(filtered, c)
		}
	}

	if err := fpf.BuildSpecIndex(dbPath, filtered); err != nil {
		fmt.Fprintf(os.Stderr, "Error building index: %v\n", err)
		os.Exit(1)
	}

	if commitSHA != "" {
		if err := fpf.SetSpecMeta(dbPath, "fpf_commit", commitSHA); err != nil {
			fmt.Fprintf(os.Stderr, "Error setting meta: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Printf("Indexed %d chunks (from %d total) into %s\n", len(filtered), len(chunks), dbPath)
}
