package cli

import (
	"strings"
	"testing"
	"time"
)

func TestDoctorServeProcessStatusReportsNoCurrentProjectServe(t *testing.T) {
	status, ok := doctorServeProcessStatus("/repo/haft", doctorServeProcessSnapshot{
		Processes: []doctorServeProcess{
			{PID: 7, CWD: "/repo/other", Executable: "/bin/haft"},
		},
	})

	if !ok {
		t.Fatalf("ok = false, status = %q", status)
	}
	if !strings.Contains(status, "none for current project") {
		t.Fatalf("status = %q, want current-project absence", status)
	}
}

func TestDoctorServeProcessStatusWarnsOnDuplicateCurrentProjectServe(t *testing.T) {
	now := time.Date(2026, 6, 24, 1, 2, 3, 0, time.UTC)
	status, ok := doctorServeProcessStatus("/repo/haft", doctorServeProcessSnapshot{
		PathHaft:      "/repo/bin/haft",
		PathHaftMTime: now,
		Processes: []doctorServeProcess{
			{PID: 10, CWD: "/repo/haft", Executable: "/repo/bin/haft", ExecutableMTime: now},
			{PID: 11, CWD: "/repo/haft", Executable: "/repo/bin/haft", ExecutableMTime: now},
		},
	})

	if ok {
		t.Fatalf("ok = true, status = %q", status)
	}
	if !strings.Contains(status, "multiple current-project serve processes") {
		t.Fatalf("status = %q, want duplicate warning", status)
	}
}

func TestDoctorServeProcessStatusWarnsOnStaleServeExecutable(t *testing.T) {
	pathMTime := time.Date(2026, 6, 24, 5, 17, 0, 0, time.UTC)
	serveMTime := time.Date(2026, 6, 24, 4, 31, 0, 0, time.UTC)
	startedAt := time.Date(2026, 6, 24, 4, 45, 0, 0, time.UTC)
	status, ok := doctorServeProcessStatus("/repo/haft", doctorServeProcessSnapshot{
		PathHaft:      "/Users/me/go/bin/haft",
		PathHaftMTime: pathMTime,
		Processes: []doctorServeProcess{
			{
				PID:             348,
				StartTime:       startedAt,
				CWD:             "/repo/haft",
				Executable:      "/Users/me/.local/bin/haft",
				ExecutableMTime: serveMTime,
			},
		},
	})

	if ok {
		t.Fatalf("ok = true, status = %q", status)
	}
	for _, want := range []string{
		"serve executable differs from PATH haft",
		"serve executable older than PATH haft",
		"pid=348",
	} {
		if !strings.Contains(status, want) {
			t.Fatalf("status = %q, want %q", status, want)
		}
	}
}

func TestDoctorServeProcessStatusWarnsWhenServeStartedBeforeExecutableRebuild(t *testing.T) {
	rebuildTime := time.Date(2026, 6, 24, 5, 17, 0, 0, time.UTC)
	startedAt := time.Date(2026, 6, 24, 5, 1, 0, 0, time.UTC)
	status, ok := doctorServeProcessStatus("/repo/haft", doctorServeProcessSnapshot{
		PathHaft:      "/repo/bin/haft",
		PathHaftMTime: rebuildTime,
		Processes: []doctorServeProcess{
			{
				PID:             42,
				StartTime:       startedAt,
				CWD:             "/repo/haft",
				Executable:      "/repo/bin/haft",
				ExecutableMTime: rebuildTime,
			},
		},
	})

	if ok {
		t.Fatalf("ok = true, status = %q", status)
	}
	if !strings.Contains(status, "serve process predates executable rebuild") {
		t.Fatalf("status = %q, want start-time stale warning", status)
	}
}

func TestDoctorServeProcessStatusAcceptsSingleCurrentProjectServe(t *testing.T) {
	now := time.Date(2026, 6, 24, 5, 17, 0, 0, time.UTC)
	status, ok := doctorServeProcessStatus("/repo/haft", doctorServeProcessSnapshot{
		PathHaft:      "/repo/bin/haft",
		PathHaftMTime: now,
		Processes: []doctorServeProcess{
			{
				PID:             42,
				StartTime:       now.Add(time.Minute),
				CWD:             "/repo/haft",
				Executable:      "/repo/bin/haft",
				ExecutableMTime: now,
			},
		},
	})

	if !ok {
		t.Fatalf("ok = false, status = %q", status)
	}
	if !strings.Contains(status, "1 current-project serve process") {
		t.Fatalf("status = %q, want current serve summary", status)
	}
}

func TestParseDoctorServeProcessLineAcceptsPathHaftServe(t *testing.T) {
	process, ok := parseDoctorServeProcessLine(" 348 /Users/me/.local/bin/haft serve")
	if !ok {
		t.Fatal("line was not accepted")
	}
	if process.PID != 348 || process.Command != "/Users/me/.local/bin/haft serve" {
		t.Fatalf("process = %#v", process)
	}
}

func TestParseDoctorServeProcessLineRejectsNonServeHaft(t *testing.T) {
	_, ok := parseDoctorServeProcessLine(" 349 /Users/me/.local/bin/haft doctor")
	if ok {
		t.Fatal("doctor command accepted as serve")
	}
}

func TestDoctorProcessCWDAndExecutablePrefersHaftText(t *testing.T) {
	cwd, executable := doctorProcessCWDAndExecutable(strings.Join([]string{
		"p348",
		"fcwd",
		"n/repo/haft",
		"ftxt",
		"n/Users/me/.local/bin/haft",
		"ftxt",
		"n/usr/lib/dyld",
	}, "\n"))

	if cwd != "/repo/haft" {
		t.Fatalf("cwd = %q", cwd)
	}
	if executable != "/Users/me/.local/bin/haft" {
		t.Fatalf("executable = %q", executable)
	}
}
