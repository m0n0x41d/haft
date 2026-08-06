package cli

import "testing"

// isArtifactRef distinguishes an artifact-ID anchor from a code-symbol anchor.
func TestIsArtifactRef(t *testing.T) {
	t.Parallel()

	artifacts := []string{
		"dec-20260604-ef966a11",
		"prob-20260603-5d066704",
		"note-20260604-1d776694",
		"sol-20260604-abc",
	}
	symbols := []string{
		"SearchSymbols",
		"getUserName",
		"ResolveFileEdges@internal/codebase/resolver.go",
		"foo-bar", // lowercase prefix but not a date digit after '-'
		"X",
		"",
	}
	for _, r := range artifacts {
		if !isArtifactRef(r) {
			t.Errorf("isArtifactRef(%q) = false, want true (artifact ID)", r)
		}
	}
	for _, r := range symbols {
		if isArtifactRef(r) {
			t.Errorf("isArtifactRef(%q) = true, want false (symbol ref)", r)
		}
	}
}
