package p13acceptance

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"unicode"
)

type restrictedSourceLabel struct {
	runeWidth int
	digest    [sha256.Size]byte
}

func TestProductCarriersUseSourceNeutralNames(t *testing.T) {
	root, err := repositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	labels := restrictedSourceLabels(t)
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if entry.IsDir() && sourceNamingExcludedDirectory(relative, entry.Name()) {
			return filepath.SkipDir
		}
		if entry.IsDir() || !sourceNamingTextCarrier(path) {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		assertSourceNeutralText(t, relative, string(content), labels)
		return nil
	})
	if err != nil {
		t.Fatalf("scan product carriers: %v", err)
	}
}

func TestTrackedCarriersUseSourceNeutralNames(t *testing.T) {
	root, err := repositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	_, err = exec.LookPath("git")
	if err != nil {
		t.Skip("git is unavailable; tracked-carrier scan requires a repository checkout")
	}
	gitMetadata := filepath.Join(root, ".git")
	_, err = os.Stat(gitMetadata)
	if os.IsNotExist(err) {
		t.Skip("git metadata is unavailable; tracked-carrier scan requires a repository checkout")
	}
	if err != nil {
		t.Fatalf("stat git metadata: %v", err)
	}
	command := exec.Command("git", "-C", root, "ls-files", "-z")
	output, err := command.Output()
	if err != nil {
		t.Fatalf("list tracked carriers: %v", err)
	}
	paths := strings.Split(string(output), "\x00")
	labels := restrictedSourceLabels(t)
	for _, relative := range paths {
		if relative == "" {
			continue
		}
		if sourceNamingExcludedTrackedPath(relative) {
			continue
		}
		assertSourceNeutralText(t, relative, relative, labels)
		path := filepath.Join(root, relative)
		info, statErr := os.Stat(path)
		if os.IsNotExist(statErr) {
			continue
		}
		if statErr != nil {
			t.Fatalf("stat tracked carrier %s: %v", relative, statErr)
		}
		if !info.Mode().IsRegular() || !sourceNamingTextCarrier(path) {
			continue
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read tracked carrier %s: %v", relative, readErr)
		}
		assertSourceNeutralText(t, relative, string(content), labels)
	}
}

func TestSourceNamingExcludedTrackedPathsMatchDirectoryPolicy(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		excluded bool
	}{
		{
			name:     "historical decision carrier",
			path:     ".haft/decisions/dec-example.md",
			excluded: true,
		},
		{
			name:     "method run carrier",
			path:     ".haft/method-runs/mpull-example.md",
			excluded: true,
		},
		{
			name:     "product spec carrier",
			path:     ".haft/specs/software-system.md",
			excluded: false,
		},
		{
			name:     "product source",
			path:     "internal/p13acceptance/source_naming_purity_test.go",
			excluded: false,
		},
		{
			name:     "bundled FPF source",
			path:     "data/FPF/FPF-Spec.md",
			excluded: true,
		},
		{
			name:     "nested dependency",
			path:     "web/node_modules/dependency/index.js",
			excluded: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := sourceNamingExcludedTrackedPath(test.path)
			if actual != test.excluded {
				t.Fatalf(
					"sourceNamingExcludedTrackedPath(%q) = %t, want %t",
					test.path,
					actual,
					test.excluded,
				)
			}
		})
	}
}

func assertSourceNeutralText(t *testing.T, carrier string, content string, labels []restrictedSourceLabel) {
	t.Helper()
	tokens := sourceNamingTokens(content)
	for index, label := range labels {
		if !containsRestrictedSourceLabel(tokens, label) {
			continue
		}
		t.Errorf(
			"product carrier %s contains private-source marker #%d; use source-neutral naming",
			carrier,
			index+1,
		)
	}
}

func restrictedSourceLabels(t *testing.T) []restrictedSourceLabel {
	t.Helper()
	digests := []struct {
		runeWidth int
		hexDigest string
	}{
		{runeWidth: 10, hexDigest: "29d196c50d4396dfa24d6c20147bc460d3064f82e80d8693f04f76c7deb6643b"},
		{runeWidth: 7, hexDigest: "b246cf89fe828ae4e9e8cc69e938034b3cd13d41808318a9cf47d3c6e57d6807"},
		{runeWidth: 10, hexDigest: "e9d69e6fd235382567342050169d63589b5c52c5aa29d70bc5c9067d091645f8"},
		{runeWidth: 7, hexDigest: "5844e09d9379ca622164cff9c44a0a045a9aaf2fc02ae259ca3514c9003b1fe0"},
		{runeWidth: 29, hexDigest: "835208109e5fd5238ba1bd83b4142c11c842f0a619ae5252bea92f4b0abad781"},
	}
	labels := make([]restrictedSourceLabel, 0, len(digests))
	for _, value := range digests {
		decoded, err := hex.DecodeString(value.hexDigest)
		if err != nil {
			t.Fatalf("decode restricted source-label digest: %v", err)
		}
		var digest [sha256.Size]byte
		copy(digest[:], decoded)
		labels = append(labels, restrictedSourceLabel{
			runeWidth: value.runeWidth,
			digest:    digest,
		})
	}
	return labels
}

func sourceNamingTokens(content string) [][]rune {
	lowered := strings.ToLower(content)
	parts := strings.FieldsFunc(lowered, func(value rune) bool {
		return !unicode.IsLetter(value) && !unicode.IsDigit(value) && value != '_'
	})
	tokens := make([][]rune, 0, len(parts))
	for _, part := range parts {
		tokens = append(tokens, []rune(part))
	}
	return tokens
}

func containsRestrictedSourceLabel(tokens [][]rune, label restrictedSourceLabel) bool {
	for _, token := range tokens {
		if len(token) < label.runeWidth {
			continue
		}
		limit := len(token) - label.runeWidth
		for start := 0; start <= limit; start++ {
			end := start + label.runeWidth
			candidate := string(token[start:end])
			digest := sha256.Sum256([]byte(candidate))
			if digest == label.digest {
				return true
			}
		}
	}
	return false
}

func sourceNamingExcludedDirectory(relative string, name string) bool {
	if relative == "data/FPF" {
		return true
	}
	if strings.HasPrefix(relative, ".haft"+string(filepath.Separator)) {
		parts := strings.Split(relative, string(filepath.Separator))
		if len(parts) == 2 && parts[1] != "specs" {
			return true
		}
	}
	if name == ".context" || strings.HasPrefix(name, ".context-") {
		return true
	}
	excluded := map[string]struct{}{
		".cache":               {},
		".git":                 {},
		".playwright-mcp":      {},
		".pnpm-store":          {},
		".task":                {},
		".zenflow":             {},
		".zenflow-attachments": {},
		"_build":               {},
		"deps":                 {},
		"node_modules":         {},
		"tmp":                  {},
	}
	_, found := excluded[name]
	return found
}

func sourceNamingExcludedTrackedPath(relative string) bool {
	directory := filepath.Dir(filepath.Clean(relative))
	for directory != "." {
		name := filepath.Base(directory)
		if sourceNamingExcludedDirectory(directory, name) {
			return true
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			return false
		}
		directory = parent
	}
	return false
}

func sourceNamingTextCarrier(path string) bool {
	extensions := map[string]struct{}{
		".astro": {},
		".css":   {},
		".ex":    {},
		".exs":   {},
		".go":    {},
		".html":  {},
		".js":    {},
		".json":  {},
		".md":    {},
		".mjs":   {},
		".sh":    {},
		".sql":   {},
		".toml":  {},
		".ts":    {},
		".tsx":   {},
		".txt":   {},
		".yaml":  {},
		".yml":   {},
	}
	_, found := extensions[strings.ToLower(filepath.Ext(path))]
	return found
}
