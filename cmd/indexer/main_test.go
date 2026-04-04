package main

import (
	"testing"
	"time"
)

func TestResolveSpecCommit_UsesOnlyExplicitCommit(t *testing.T) {
	tests := []struct {
		name           string
		explicitCommit string
		want           string
	}{
		{
			name:           "empty",
			explicitCommit: "",
			want:           "",
		},
		{
			name:           "trimmed",
			explicitCommit: "  abc123  ",
			want:           "abc123",
		},
	}

	for _, tt := range tests {
		got := resolveSpecCommit(tt.explicitCommit)
		if got != tt.want {
			t.Fatalf("%s: resolveSpecCommit(%q) = %q, want %q", tt.name, tt.explicitCommit, got, tt.want)
		}
	}
}

func TestBuildSpecIndexMetadata_DoesNotInventCommitProvenance(t *testing.T) {
	buildTime := time.Date(2026, time.March, 26, 12, 34, 56, 0, time.UTC)
	metadata := buildSpecIndexMetadata("data/FPF/FPF-Spec.md", 42, "", buildTime)

	if metadata["fpf_commit"] != "" {
		t.Fatalf("expected empty fpf_commit without explicit upstream revision, got %q", metadata["fpf_commit"])
	}
	if metadata["indexed_sections"] != "42" {
		t.Fatalf("unexpected indexed_sections %q", metadata["indexed_sections"])
	}
}
