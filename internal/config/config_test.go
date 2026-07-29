package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadReadsEmbeddingAndIgnoresHistoricalProviderFields(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, haftDir, configFile)
	err := os.MkdirAll(filepath.Dir(path), 0o755)
	if err != nil {
		t.Fatalf("create config directory: %v", err)
	}

	body := []byte(`
model: removed-agent-model
providers:
  openai:
    api_key: ignored-secret
embedding:
  provider: openai
  model: text-embedding-3-small
  dim: 512
`)
	err = os.WriteFile(path, body, 0o600)
	if err != nil {
		t.Fatalf("write config: %v", err)
	}

	t.Setenv("HOME", home)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Embedding.Provider != "openai" {
		t.Fatalf("embedding provider = %q", cfg.Embedding.Provider)
	}
	if cfg.Embedding.Model != "text-embedding-3-small" {
		t.Fatalf("embedding model = %q", cfg.Embedding.Model)
	}
	if cfg.Embedding.Dim != 512 {
		t.Fatalf("embedding dim = %d", cfg.Embedding.Dim)
	}
}

func TestLoadReturnsEmptyConfigWhenAbsent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load absent config: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected empty config")
	}
	if cfg.Embedding != (EmbeddingConfig{}) {
		t.Fatalf("embedding = %#v, want zero value", cfg.Embedding)
	}
}
