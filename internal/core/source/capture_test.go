package source

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/core/carrier"
)

func TestCaptureIndependentOfProjectAndLaterEdits(t *testing.T) {
	root := copyFixture(t, "pin-a")
	r, err := Capture(root, "fixture-tree", Limits{})
	if err != nil {
		t.Fatal(err)
	}
	a := requireFound(t, r, "A.1")
	if err := os.MkdirAll(filepath.Join(root, ".haft"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".haft", "haft.db"), []byte("incompatible ancient database"), 0644); err != nil {
		t.Fatal(err)
	}
	same, err := Capture(root, "fixture-tree", Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(same.Status().Manifest, r.Status().Manifest) {
		t.Fatal("project DB changed source capture")
	}
	p := filepath.Join(root, "FPF-Spec.md")
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	changed := bytes.ReplaceAll(raw, []byte("serial alpha"), []byte("serial bravo"))
	if len(changed) != len(raw) {
		t.Fatal("test requires equal sized edit")
	}
	if err := os.WriteFile(p, changed, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(p, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	later, err := Capture(root, "fixture-tree", Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(requireFound(t, r, "A.1").Raw, a.Raw) {
		t.Fatal("existing reader followed mutable source")
	}
	b := requireFound(t, later, "A.1")
	if bytes.Equal(a.Raw, b.Raw) || a.Source.SourceRevision.TreeDigest == b.Source.SourceRevision.TreeDigest {
		t.Fatal("preserved metadata concealed changed bytes")
	}
	// This is the exact inspect-to-remember material: old bytes stay old after a new capture.
	replay := InspectSnapshot(a.Snapshot, a.Source.SnapshotRef)
	if replay.Kind != "found" || !bytes.Equal(replay.Unit.Raw, a.Raw) || replay.Unit.Source.SourceRevision.TreeDigest != a.Source.SourceRevision.TreeDigest {
		t.Fatal("replay substituted later source")
	}
	if r.Status().Revision.Kind != "local_tree" || r.Status().Revision.Commit != "" || !r.Status().Offline || r.Status().UpstreamCurrentness != "unknown" {
		t.Fatal("local capture acquired official/current-upstream provenance")
	}
}

func TestCaptureDetectsSameMetadataChangeWithBoundedRetry(t *testing.T) {
	before, err := os.ReadFile("testdata/capture/before/FPF-Spec.md")
	if err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile("testdata/capture/after/FPF-Spec.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != len(after) || bytes.Equal(before, after) {
		t.Fatal("race fixture lost distinguishing equal-size bytes")
	}
	root := t.TempDir()
	p := filepath.Join(root, "FPF-Spec.md")
	if err := os.WriteFile(p, before, 0644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	write := func(raw []byte) {
		if err := os.WriteFile(p, raw, 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, info.ModTime(), info.ModTime()); err != nil {
			t.Fatal(err)
		}
	}
	calls := 0
	r, err := capture(root, "race-fixture", Limits{}, func(attempt int) {
		calls++
		if attempt == 0 {
			write(after)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 3 {
		t.Fatalf("expected three scans after first interference, got %d", calls)
	}
	if !bytes.Equal(requireFound(t, r, "FPF-Spec.md").Raw, after) {
		t.Fatal("unstable first capture escaped")
	}
	write(before)
	calls = 0
	r, err = capture(root, "race-fixture", Limits{}, func(attempt int) {
		calls++
		if attempt%2 == 0 {
			write(after)
		} else {
			write(before)
		}
	})
	if err == nil || r != nil || !strings.Contains(err.Error(), "source_changed_during_capture") || calls != 3 {
		t.Fatalf("unstable capture did not fail boundedly: calls=%d err=%v", calls, err)
	}
}

func TestCaptureRejectsSymlinkEscapesAndBounds(t *testing.T) {
	outside := t.TempDir()
	target := filepath.Join(outside, "outside.md")
	if err := os.WriteFile(target, []byte("## A.1 - Outside\n### A.1:End\n"), 0644); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.Symlink(target, filepath.Join(root, "FPF-Spec.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := Capture(root, "fixture", Limits{}); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("followed source symlink: %v", err)
	}
	root = t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "Engineering DPF Suite")); err != nil {
		t.Fatal(err)
	}
	if _, err := Capture(root, "fixture", Limits{}); err == nil {
		t.Fatal("followed suite directory symlink")
	}
	parent := t.TempDir()
	alias := filepath.Join(parent, "aliased-source")
	if err := os.Symlink(outside, alias); err != nil {
		t.Fatal(err)
	}
	if _, err := Capture(alias, "fixture", Limits{}); err == nil {
		t.Fatal("followed root symlink")
	}
	root = copyFixture(t, "pin-a")
	if _, err := Capture(root, "fixture", Limits{MaxFiles: 1}); err == nil {
		t.Fatal("file budget ignored")
	}
	if _, err := Capture(root, "fixture", Limits{MaxFileBytes: 10}); err == nil {
		t.Fatal("per-file budget ignored")
	}
	if _, err := Capture(root, "fixture", Limits{MaxBytes: 100}); err == nil {
		t.Fatal("total byte budget ignored")
	}
	root = t.TempDir()
	for i := 0; i < 17; i++ {
		if err := os.WriteFile(filepath.Join(root, string(rune('a'+i))+".txt"), nil, 0644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Capture(root, "fixture", Limits{MaxFiles: 1}); err == nil || !strings.Contains(err.Error(), "source_entry_limit") {
		t.Fatalf("unbounded directory scan: %v", err)
	}
}

// The full corpus stays outside the repository. Opt-in verification uses the
// exact reviewed body digests, not a developer-specific pathname in product code.
func TestPinnedCorpusOptIn(t *testing.T) {
	root := os.Getenv("HAFT_SOURCE_CORPUS_TEST_ROOT")
	if root == "" {
		t.Skip("set HAFT_SOURCE_CORPUS_TEST_ROOT to the reviewed 2026-09-22 raw corpus")
	}
	r, err := Capture(root, "reviewed-2026-09-22-raw-corpus", Limits{})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"A.6.B":                  "sha256:f82965daae3a52f54ee2fe4d3ca65cb1176d9f46e31136721fd9c597defd44e5",
		"C.2.1":                  "sha256:a55ce03018862250f2ed2ef542e213e5286f547c8b1a0e9a46d376a25c38db6e",
		"A.10":                   "sha256:c1c633ff7bba308fe420e3c5689a35518a48bb7e155dc7d53604013f6a418978",
		"E.11.PUA":               "sha256:c0b56e6661385812fc1d430ebec91cbf8cbcc7de61b309787eddb2677402b572",
		"E.11.PUR":               "sha256:288797a1363edd0cdca95fe604a3ea231d9d62dede4ccf006131934104d017e4",
		"SYSE.31":                "sha256:ba4a840718a2b7b9e46b7db5376185bb30070dfc2eada2a8274485db7f85abf2",
		"A.19.SelectorMechanism": "sha256:b7e2f6ec8f08d2e908bd198eed18cf77f2d04639c72c0fd84fd56188e4c1c6ed",
		"C.2.2a":                 "sha256:529490410d8bbc6bf76455b7d4b1959d9b07eb092576ecacf6543352d6c311db",
		"G.Core":                 "sha256:a372817f30b6af861665ec8ac047694dc6bf4fe602c37b1cfeb59ce205684002",
	}
	for ref, digest := range want {
		u := requireFound(t, r, ref)
		if carrier.Digest(u.Raw) != digest {
			t.Fatalf("reviewed exact body changed: %s", ref)
		}
	}
	status := r.Status()
	if status.Kind != "available" || status.Patterns != 690 || status.Publications != 27 {
		t.Fatalf("unexpected corpus inventory: %+v", status)
	}
	t.Logf("source revision=%s publications=%d patterns=%d", status.Revision.TreeDigest, status.Publications, status.Patterns)
	for _, entry := range status.Manifest {
		t.Logf("manifest %s %d %s", entry.Path, entry.Bytes, entry.Digest)
	}
}
