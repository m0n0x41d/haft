package embedding

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveOpenAIAPIKeyPrefersEnvironment(t *testing.T) {
	home := writeLegacyHaftAuthFile(t, `{"api_key":"file-key"}`)
	t.Setenv("HOME", home)
	t.Setenv("OPENAI_API_KEY", "environment-key")

	key, err := resolveOpenAIAPIKey()
	if err != nil {
		t.Fatalf("resolve OpenAI API key: %v", err)
	}
	if key != "environment-key" {
		t.Fatalf("key = %q, want environment-key", key)
	}
}

func TestResolveOpenAIAPIKeyReadsLegacyHaftAuthFile(t *testing.T) {
	home := writeLegacyHaftAuthFile(t, `{"api_key":"file-key"}`)
	t.Setenv("HOME", home)
	t.Setenv("OPENAI_API_KEY", "")

	key, err := resolveOpenAIAPIKey()
	if err != nil {
		t.Fatalf("resolve OpenAI API key: %v", err)
	}
	if key != "file-key" {
		t.Fatalf("key = %q, want file-key", key)
	}
}

func TestResolveOpenAIAPIKeyRejectsOAuthOnlyLegacyAuth(t *testing.T) {
	home := writeLegacyHaftAuthFile(
		t,
		`{"codex_access_token":"oauth-token","codex_refresh_token":"refresh-token"}`,
	)
	t.Setenv("HOME", home)
	t.Setenv("OPENAI_API_KEY", "")

	_, err := resolveOpenAIAPIKey()
	if err == nil {
		t.Fatal("expected OAuth-only auth to be rejected for embeddings")
	}
}

func TestResolveOpenAIAPIKeyDoesNotReadCodexOAuth(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".codex")
	err := os.MkdirAll(dir, 0o755)
	if err != nil {
		t.Fatalf("create Codex auth directory: %v", err)
	}

	path := filepath.Join(dir, "auth.json")
	body := `{"tokens":{"access_token":"oauth-token"}}`
	err = os.WriteFile(path, []byte(body), 0o600)
	if err != nil {
		t.Fatalf("write Codex auth file: %v", err)
	}

	t.Setenv("HOME", home)
	t.Setenv("OPENAI_API_KEY", "")

	_, err = resolveOpenAIAPIKey()
	if err == nil {
		t.Fatal("expected Codex OAuth to be rejected for embeddings")
	}
}

func TestResolveOpenAIAPIKeyRejectsMalformedLegacyAuth(t *testing.T) {
	home := writeLegacyHaftAuthFile(t, `{"api_key":`)
	t.Setenv("HOME", home)
	t.Setenv("OPENAI_API_KEY", "")

	_, err := resolveOpenAIAPIKey()
	if err == nil {
		t.Fatal("expected malformed legacy auth to be rejected")
	}
}

func TestResolveOpenAIAPIKeyFailsWithoutDirectKey(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("OPENAI_API_KEY", "")

	_, err := resolveOpenAIAPIKey()
	if err == nil {
		t.Fatal("expected missing API key error")
	}
}

func writeLegacyHaftAuthFile(t *testing.T, body string) string {
	t.Helper()

	home := t.TempDir()
	dir := filepath.Join(home, ".config", "haft")
	err := os.MkdirAll(dir, 0o755)
	if err != nil {
		t.Fatalf("create legacy auth directory: %v", err)
	}

	path := filepath.Join(dir, "auth.json")
	err = os.WriteFile(path, []byte(body), 0o600)
	if err != nil {
		t.Fatalf("write legacy auth file: %v", err)
	}
	return home
}
