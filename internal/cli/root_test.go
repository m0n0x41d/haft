package cli

import "testing"

func TestEffectiveBuildInfoUsesVCSFallbackForDevBuilds(t *testing.T) {
	oldCommit := Commit
	oldBuildDate := BuildDate
	t.Cleanup(func() {
		Commit = oldCommit
		BuildDate = oldBuildDate
	})

	Commit = "none"
	BuildDate = "unknown"

	info := effectiveBuildInfo(map[string]string{
		"vcs.revision": "abcdef1234567890",
		"vcs.time":     "2026-06-23T07:00:00Z",
	})

	if info.Commit != "abcdef1234567890" {
		t.Fatalf("commit = %q, want VCS fallback", info.Commit)
	}
	if info.BuildDate != "unknown" {
		t.Fatalf("build date = %q, want ldflag value preserved", info.BuildDate)
	}
	if info.SourceTime != "2026-06-23T07:00:00Z" {
		t.Fatalf("source time = %q", info.SourceTime)
	}
}

func TestEffectiveBuildInfoKeepsLdflagsAheadOfVCSFallback(t *testing.T) {
	oldCommit := Commit
	oldBuildDate := BuildDate
	t.Cleanup(func() {
		Commit = oldCommit
		BuildDate = oldBuildDate
	})

	Commit = "release-sha"
	BuildDate = "2026-06-23T08:00:00Z"

	info := effectiveBuildInfo(map[string]string{
		"vcs.revision": "dev-sha",
		"vcs.modified": "true",
	})

	if info.Commit != "release-sha" {
		t.Fatalf("commit = %q, want ldflag commit", info.Commit)
	}
	if info.BuildDate != "2026-06-23T08:00:00Z" {
		t.Fatalf("build date = %q, want ldflag build date", info.BuildDate)
	}
	if !info.Modified {
		t.Fatalf("modified = false, want true")
	}
}
