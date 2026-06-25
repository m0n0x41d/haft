//go:build !windows

package embedding

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildSidecarStatusCombinesSocketArtifactsAndProcesses(t *testing.T) {
	socketDir := t.TempDir()
	nativeSocket := filepath.Join(socketDir, "embed-native.sock")
	nativeLock := filepath.Join(socketDir, "embed-native.lock")
	dimSocket := filepath.Join(socketDir, "embed-dim.sock")

	touchFile(t, nativeSocket)
	touchFile(t, nativeLock)
	touchFile(t, dimSocket)

	processes := []sidecarProcessStatus{
		{
			PID:             101,
			PPID:            10,
			RSSKB:           2048,
			VSZKB:           4096,
			Command:         "haft-embed --model fake --cache-dir /tmp/models --serve-socket " + nativeSocket + " --idle-timeout-secs 1200",
			ServeSocket:     nativeSocket,
			Model:           "fake",
			CacheDir:        "/tmp/models",
			DimLabel:        "native",
			IdleTimeoutSecs: 1200,
		},
		{
			PID:      202,
			PPID:     20,
			RSSKB:    1024,
			VSZKB:    2048,
			Command:  "haft-embed --model fake --cache-dir /tmp/models",
			Model:    "fake",
			CacheDir: "/tmp/models",
			DimLabel: "native",
		},
	}

	report := buildSidecarStatus(socketDir, processes)

	if len(report.Entries) != 3 {
		t.Fatalf("entries = %d, want 3: %#v", len(report.Entries), report.Entries)
	}
	active := report.Entries[0]
	if active.Key != "embed-native" || active.State != sidecarStateActive {
		t.Fatalf("first entry = %#v, want active embed-native", active)
	}
	if active.PID != 101 || !active.SocketExists || !active.LockExists {
		t.Fatalf("active process/socket fields = %#v", active)
	}
	stdio := report.Entries[1]
	if stdio.State != sidecarStateStdioProcess || stdio.PID != 202 {
		t.Fatalf("second entry = %#v, want stdio process", stdio)
	}
	stale := report.Entries[2]
	if stale.Key != "embed-dim" || stale.State != sidecarStateStaleSocket {
		t.Fatalf("third entry = %#v, want stale socket", stale)
	}
}

func TestParseSidecarProcessesExtractsSharedArgs(t *testing.T) {
	output := ` 3821     1 497008 490360416 /Users/me/.haft/runtimes/haft-embed/current/bin/haft-embed --model embeddinggemma-300m-q --cache-dir /Users/me/.haft/models --dim 256 --serve-socket /tmp/haft-501/embed/embed-f3.sock --idle-timeout-secs 1200
 91549 29505 313328 493630720 /Users/me/.haft/runtimes/haft-embed/current/bin/haft-embed --model embeddinggemma-300m-q --cache-dir /Users/me/.haft/models --serve-socket /tmp/haft-501/embed/embed-994.sock --idle-timeout-secs 1200
`

	processes := parseSidecarProcesses(output)

	if len(processes) != 2 {
		t.Fatalf("processes = %d, want 2: %#v", len(processes), processes)
	}
	first := processes[0]
	if first.PID != 3821 || first.RequestedDim != 256 || first.DimLabel != "256" {
		t.Fatalf("first parsed process = %#v", first)
	}
	second := processes[1]
	if second.PID != 91549 || second.RequestedDim != 0 || second.DimLabel != "native" {
		t.Fatalf("second parsed process = %#v", second)
	}
}

func TestParseVMMapPhysicalFootprintMB(t *testing.T) {
	output := "Physical footprint:         3.7G\n"

	got, ok := parseVMMapPhysicalFootprintMB(output)

	if !ok {
		t.Fatalf("physical footprint not parsed")
	}
	want := 3788.8
	if got != want {
		t.Fatalf("physical footprint MB = %v, want %v", got, want)
	}
}

func touchFile(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte{}, 0o600); err != nil {
		t.Fatalf("touch %s: %v", path, err)
	}
}
