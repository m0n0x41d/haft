package embedding

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestNewNormalizesSupportedProviders(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("OPENAI_API_KEY", "test-api-key")
	t.Setenv(sidecarBinaryEnv, filepath.Join(t.TempDir(), "missing-haft-embed"))

	tests := []struct {
		name         string
		provider     string
		wantErr      error
		wantProvider string
	}{
		{
			name:     "empty defaults to local",
			provider: "",
			wantErr:  ErrSidecarUnavailable,
		},
		{
			name:     "local ignores case and whitespace",
			provider: " LoCaL ",
			wantErr:  ErrSidecarUnavailable,
		},
		{
			name:         "openai ignores case and whitespace",
			provider:     " OpEnAI ",
			wantProvider: ProviderOpenAI,
		},
		{
			name:     "none ignores case and whitespace",
			provider: " NoNe ",
			wantErr:  ErrDisabled,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			embedder, err := New(Config{Provider: tt.provider})
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("New() error = %v, want %v", err, tt.wantErr)
				}
				if embedder != nil {
					t.Fatalf("New() embedder = %#v, want nil", embedder)
				}
				return
			}
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			t.Cleanup(func() {
				if closeErr := embedder.Close(); closeErr != nil {
					t.Errorf("close embedder: %v", closeErr)
				}
			})
			if got := embedder.Descriptor().Provider; got != tt.wantProvider {
				t.Fatalf("provider = %q, want %q", got, tt.wantProvider)
			}
		})
	}
}

func TestNewRejectsUnknownProviderBeforeAdapterConstruction(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv(sidecarBinaryEnv, filepath.Join(t.TempDir(), "missing-haft-embed"))

	embedder, err := New(Config{Provider: "  typo  "})
	if embedder != nil {
		t.Fatalf("New() embedder = %#v, want nil", embedder)
	}
	if err == nil || err.Error() != `unknown embedding provider "  typo  "` {
		t.Fatalf("New() error = %v, want unknown-provider failure", err)
	}
	if errors.Is(err, ErrSidecarUnavailable) || errors.Is(err, ErrDisabled) {
		t.Fatalf("New() error = %v, want validation before adapter selection", err)
	}
}
