package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDoctorEmbeddingConfigStatusRejectsMalformedActiveConfig(t *testing.T) {
	home := writeDoctorGlobalConfig(t, "embedding: [")
	t.Setenv("HOME", home)

	_, err := doctorEmbeddingConfigStatus()
	if err == nil || !strings.Contains(err.Error(), "parse config") {
		t.Fatalf("error = %v, want parse config failure", err)
	}
}

func TestDoctorEmbeddingConfigStatusReportsCurrentEmbeddingConfig(t *testing.T) {
	home := writeDoctorGlobalConfig(
		t,
		"embedding:\n  provider: openai\n  model: text-embedding-3-small\n  dim: 512\n",
	)
	t.Setenv("HOME", home)
	t.Setenv("OPENAI_API_KEY", "")

	status, err := doctorEmbeddingConfigStatus()
	if err != nil {
		t.Fatalf("doctor embedding config status: %v", err)
	}
	if status != "provider=openai model=text-embedding-3-small dim=512" {
		t.Fatalf("status = %q", status)
	}
}

func TestDoctorEmbeddingConfigStatusDoesNotRequireLocalSidecar(t *testing.T) {
	home := writeDoctorGlobalConfig(t, "embedding:\n  provider: local\n")
	t.Setenv("HOME", home)
	t.Setenv("HAFT_EMBED_BIN", filepath.Join(home, "missing-haft-embed"))

	status, err := doctorEmbeddingConfigStatus()
	if err != nil {
		t.Fatalf("doctor embedding config status: %v", err)
	}
	if status != "provider=local model=default dim=0" {
		t.Fatalf("status = %q", status)
	}
}

func TestDoctorEmbeddingConfigStatusRejectsUnknownProvider(t *testing.T) {
	home := writeDoctorGlobalConfig(t, "embedding:\n  provider: typo\n")
	t.Setenv("HOME", home)

	_, err := doctorEmbeddingConfigStatus()
	if err == nil || !strings.Contains(err.Error(), `unknown embedding provider "typo"`) {
		t.Fatalf("error = %v, want unknown provider failure", err)
	}
}

func writeDoctorGlobalConfig(t *testing.T, body string) string {
	t.Helper()

	home := t.TempDir()
	dir := filepath.Join(home, ".haft")
	err := os.MkdirAll(dir, 0o755)
	if err != nil {
		t.Fatalf("create global config directory: %v", err)
	}

	path := filepath.Join(dir, "config.yaml")
	err = os.WriteFile(path, []byte(body), 0o600)
	if err != nil {
		t.Fatalf("write global config: %v", err)
	}
	return home
}
