package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/embedding"
)

func TestWriteEmbedStatusSummaryShowsProcessContractAndMemoryNote(t *testing.T) {
	report := embedding.SidecarStatusReport{
		SharedEnabled: true,
		SocketDir:     "/tmp/haft-501/embed",
		BinaryFound:   true,
		Binary:        "/Users/me/.haft/runtimes/haft-embed/current/bin/haft-embed",
		Entries: []embedding.SidecarStatusEntry{
			{
				Key:             "embed-f3cdb939ab94baef",
				State:           "active",
				SocketPath:      "/tmp/haft-501/embed/embed-f3cdb939ab94baef.sock",
				LockPath:        "/tmp/haft-501/embed/embed-f3cdb939ab94baef.lock",
				SocketExists:    true,
				LockExists:      true,
				PID:             3821,
				PPID:            1,
				Model:           "embeddinggemma-300m-q",
				CacheDir:        "/Users/me/.haft/models",
				RequestedDim:    256,
				DimLabel:        "256",
				IdleTimeoutSecs: 1200,
				RSSKB:           497008,
				VSZKB:           490360416,
			},
		},
	}

	var out bytes.Buffer
	if err := writeEmbedStatusSummary(&out, report); err != nil {
		t.Fatal(err)
	}

	got := out.String()
	mustContain(t, got, "haft embed status: shared=enabled sidecars=1 socket_dir=/tmp/haft-501/embed")
	mustContain(t, got, "- embed-f3cdb939ab94baef state=active pid=3821 ppid=1 model=embeddinggemma-300m-q dim=256")
	mustContain(t, got, "socket: /tmp/haft-501/embed/embed-f3cdb939ab94baef.sock exists=true")
	mustContain(t, got, "memory_note: rss/vsz come from ps; use --footprint on macOS for physical footprint.")
}

func mustContain(t *testing.T, got string, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Fatalf("summary missing %q:\n%s", want, got)
	}
}
