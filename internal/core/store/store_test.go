package store

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/core/carrier"
)

func note(id, text string) []byte {
	r := carrier.Record{Format: "haft/1", ID: id, Kind: "note", Title: text, Status: "active", Origin: "agent_proposal", About: "system:fixture", CreatedAt: "2026-09-23T10:00:00Z"}
	raw, err := carrier.Encode(r, []byte(text+"\n"))
	if err != nil {
		panic(err)
	}
	return raw
}
func read(t *testing.T, s Store) View {
	t.Helper()
	v, err := s.Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return v
}
func request(v View, key string, outputs ...Output) Request {
	payload, _ := json.Marshal(struct {
		Key     string   `json:"key"`
		Outputs []Output `json:"outputs"`
	}{key, outputs})
	return Request{RequestID: key, PayloadDigest: carrier.Digest(payload), RequestPayload: payload, ExpectedGeneration: v.Generation, Outputs: outputs}
}
func publish(t *testing.T, s Store, r Request) Result {
	t.Helper()
	result, err := s.Publish(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	if result.Kind != "written" {
		t.Fatalf("publish: %+v", result)
	}
	return result
}
func direct(t *testing.T, root, p string, b []byte) {
	t.Helper()
	p = filepath.Join(root, ".haft", filepath.FromSlash(p))
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, b, 0644); err != nil {
		t.Fatal(err)
	}
}
func hasDiagnostic(ds []carrier.Diagnostic, code string) bool {
	for _, d := range ds {
		if d.Code == code {
			return true
		}
	}
	return false
}

func TestFreshReadPublicationReplayAndIndependentInstance(t *testing.T) {
	root := t.TempDir()
	s := Store{Root: root}
	first := read(t, s)
	if first.Coverage != "complete" || len(first.Files) != 0 {
		t.Fatal(first)
	}
	if _, err := os.Stat(filepath.Join(root, ".haft")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("empty read created canonical root")
	}
	r := request(first, "one", Output{Path: "notes/note-20260923-00000001.md", Bytes: note("note-20260923-00000001", "first")})
	written := publish(t, s, r)
	if written.Generation == first.Generation {
		t.Fatal("write did not move generation")
	}
	second := read(t, Store{Root: root})
	if len(second.Documents) != 1 || second.Coverage != "complete" || second.Generation != written.Generation {
		t.Fatalf("second instance: %+v", second)
	}
	res, err := s.Publish(context.Background(), r)
	if err != nil || res.Kind != "replayed" || res.TransactionID != written.TransactionID {
		t.Fatal(res, err)
	}
	res, found, err := s.Replay(context.Background(), r.RequestID, r.PayloadDigest)
	if err != nil || !found || res.Kind != "replayed" {
		t.Fatal(res, found, err)
	}
	r.RequestPayload = []byte("different")
	r.PayloadDigest = carrier.Digest(r.RequestPayload)
	res, err = s.Publish(context.Background(), r)
	if err != nil || res.Kind != "request_conflict" {
		t.Fatal(res, err)
	}
	r.PayloadDigest = carrier.Digest([]byte("not the request"))
	res, err = s.Publish(context.Background(), r)
	if err != nil || res.Kind != "invalid" {
		t.Fatal("payload mismatch was not rejected", res, err)
	}
	if len(read(t, s).Documents) != 1 {
		t.Fatal("replay duplicated carrier")
	}
}

func TestNoClobberGenerationCASAndManualByteEdits(t *testing.T) {
	s := Store{Root: t.TempDir()}
	v := read(t, s)
	raw := note("note-20260923-00000001", "first")
	p := "notes/note-20260923-00000001.md"
	publish(t, s, request(v, "one", Output{p, raw}))
	v = read(t, s)
	full := filepath.Join(s.Root, ".haft", filepath.FromSlash(p))
	info, err := os.Stat(full)
	if err != nil {
		t.Fatal(err)
	}
	changed := bytes.ReplaceAll(raw, []byte("first"), []byte("other"))
	if len(changed) != len(raw) {
		t.Fatal("test size changed")
	}
	if err := os.WriteFile(full, changed, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(full, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	after := read(t, s)
	if after.Generation == v.Generation || !bytes.Equal(after.Files[p], changed) {
		t.Fatal("same mtime/size mutation was missed")
	}
	r := request(v, "two", Output{"notes/note-20260923-00000002.md", note("note-20260923-00000002", "second")})
	res, err := s.Publish(context.Background(), r)
	if err != nil || res.Kind != "concurrent_write" {
		t.Fatal(res, err)
	}
	r = request(after, "overwrite", Output{p, raw})
	res, err = s.Publish(context.Background(), r)
	if err != nil || res.Kind != "path_conflict" {
		t.Fatal("existing ID overwritten", res, err)
	}
	if !bytes.Equal(read(t, s).Files[p], changed) {
		t.Fatal("conflict changed external bytes")
	}
	res, found, err := s.Replay(context.Background(), "one", request(v, "one", Output{p, raw}).PayloadDigest)
	if err != nil || !found || res.Kind != "replay_conflict" {
		t.Fatal("edited receipt regained trust", res, found, err)
	}
}

func TestBatchInterruptionWithholdsPartialOutputsAndRecovers(t *testing.T) {
	for _, stage := range []string{"staged", "published", "before_commit", "committed"} {
		t.Run(stage, func(t *testing.T) {
			root := t.TempDir()
			s := Store{Root: root}
			base := read(t, s)
			a := note("note-20260923-00000001", "one")
			b := note("note-20260923-00000002", "two")
			_, snapshot, digest, err := carrier.NewSnapshot(a, carrier.InterpretationBasis{})
			if err != nil {
				t.Fatal(err)
			}
			r := request(base, "batch", Output{"notes/note-20260923-00000001.md", a}, Output{"notes/note-20260923-00000002.md", b}, Output{"editions/sha256/" + strings.TrimPrefix(digest, "sha256:") + ".json", snapshot})
			s.Fault = func(p Point) error {
				if p.Stage == stage {
					return fmt.Errorf("injected interruption")
				}
				return nil
			}
			res, err := s.Publish(context.Background(), r)
			if err == nil {
				t.Fatal("fault did not interrupt")
			}
			s.Fault = nil
			view := read(t, s)
			if stage == "committed" {
				if res.Kind != "written" || len(view.Documents) != 2 {
					t.Fatal("committed result not visible", res)
				}
			} else {
				if res.Kind != "interrupted" || len(view.Documents) != 0 || len(view.Snapshots) != 0 || !hasDiagnostic(view.Diagnostics, "publication_pending") {
					t.Fatal("partial batch projected", res, view)
				}
				if view.Generation != base.Generation {
					t.Fatal("uncommitted outputs moved visible generation")
				}
			}
			res, found, err := s.Replay(context.Background(), r.RequestID, r.PayloadDigest)
			if err != nil || !found || res.Kind != "replayed" {
				t.Fatal("retry failed to recover", res, err)
			}
			view = read(t, s)
			if len(view.Documents) != 2 || len(view.Snapshots) != 1 || view.Coverage != "complete" {
				t.Fatal("recovery incomplete", view)
			}
			for _, o := range r.Outputs {
				if !bytes.Equal(view.Files[o.Path], o.Bytes) {
					t.Fatal("recovery changed staged bytes")
				}
			}
		})
	}
}

func TestRecoveryPreservesForeignChangesAndPreparedBytes(t *testing.T) {
	for _, mutation := range []string{"input", "output", "added-input", "staged"} {
		t.Run(mutation, func(t *testing.T) {
			s := Store{Root: t.TempDir()}
			publish(t, s, request(read(t, s), "base", Output{"notes/note-20260923-00000001.md", note("note-20260923-00000001", "base")}))
			v := read(t, s)
			output := Output{"notes/note-20260923-00000002.md", note("note-20260923-00000002", "next")}
			r := request(v, "next", output)
			s.Fault = func(p Point) error {
				if p.Stage == "published" {
					return errors.New("stop")
				}
				return nil
			}
			res, err := s.Publish(context.Background(), r)
			if err == nil {
				t.Fatal("no interruption")
			}
			s.Fault = nil
			switch mutation {
			case "input":
				direct(t, s.Root, "notes/note-20260923-00000001.md", []byte("foreign input"))
			case "output":
				direct(t, s.Root, output.Path, []byte("foreign output"))
			case "added-input":
				direct(t, s.Root, "notes/note-20260923-00000003.md", []byte("foreign addition"))
			case "staged":
				direct(t, s.Root, "transactions/"+res.TransactionID+"/staged/000000", []byte("corrupt stage"))
			}
			got, err := s.Recover(context.Background())
			if err != nil || got.Kind != "recovery_conflict" {
				t.Fatal("foreign modification was overwritten", got, err)
			}
			if mutation == "output" {
				original, err := os.ReadFile(filepath.Join(s.Root, ".haft", "transactions", res.TransactionID, "staged", "000000"))
				if err != nil || !bytes.Equal(original, output.Bytes) {
					t.Fatal("live in-place edit destroyed staged recovery bytes")
				}
				if string(read(t, s).Files[output.Path]) != "" {
					t.Fatal("uncommitted foreign output should stay withheld")
				}
			}
		})
	}
}

func TestSnapshotDurabilityTermsAndCorruption(t *testing.T) {
	s := Store{Root: t.TempDir()}
	raw, err := os.ReadFile("../carrier/testdata/valid/spec.md")
	if err != nil {
		t.Fatal(err)
	}
	terms, err := os.ReadFile("../carrier/testdata/terms.md")
	if err != nil {
		t.Fatal(err)
	}
	direct(t, s.Root, "specs/spec-20260923-00000001.md", raw)
	direct(t, s.Root, "specs/terms.md", terms)
	v := read(t, s)
	if v.Coverage != "complete" || len(v.CurrentSnapshots) != 1 || len(v.Snapshots) != 0 {
		t.Fatal("read implied persistence", v.Diagnostics)
	}
	digest := v.Documents[0].Edition
	target := v.Documents[0].Record.ID + "@" + digest + "#L1"
	if v.Resolve(target).Kind != "unresolved" || v.Resolve("spec:order-cancel#L1").Kind != "found" {
		t.Fatal("computed current snapshot became pinned history")
	}
	p := "editions/sha256/" + strings.TrimPrefix(digest, "sha256:") + ".json"
	publish(t, s, request(v, "pin", Output{p, v.CurrentSnapshots[digest]}))
	v = read(t, s)
	if v.Resolve(target).Kind != "found" {
		t.Fatal("persisted history unresolved")
	}
	direct(t, s.Root, "specs/terms.md", bytes.ReplaceAll(terms, []byte("billing domain"), []byte("changed domain")))
	changed := read(t, s)
	if changed.Documents[0].Edition == digest {
		t.Fatal("term basis mutation missed")
	}
	old, err := carrier.ReadSnapshot(changed.Snapshots[digest], digest)
	if err != nil || !bytes.Equal(old.Interpretation.Terms.Bytes, terms) {
		t.Fatal("historical terms reinterpreted")
	}
	direct(t, s.Root, p, []byte("corrupt"))
	broken := read(t, s)
	if broken.Coverage != "degraded" || !hasDiagnostic(broken.Diagnostics, "corrupt_snapshot") || broken.Resolve(target).Kind != "unresolved" {
		t.Fatal("corrupt snapshot fell back", broken.Diagnostics)
	}
}

func TestNotesIgnoreUnrelatedTermsAndCacheIsDisposable(t *testing.T) {
	s := Store{Root: t.TempDir()}
	direct(t, s.Root, "notes/note-20260923-00000001.md", note("note-20260923-00000001", "text"))
	v := read(t, s)
	edition := v.Documents[0].Edition
	direct(t, s.Root, "specs/terms.md", []byte("a term map not used by this note"))
	after := read(t, s)
	if after.Documents[0].Edition != edition {
		t.Fatal("unrelated terms reinterpreted note")
	}
	direct(t, s.Root, ".cache/stale-index.json", []byte("garbage"))
	cached := read(t, s)
	if cached.Generation != after.Generation {
		t.Fatal("cache changed canonical generation")
	}
	if err := os.RemoveAll(filepath.Join(s.Root, ".haft", ".cache")); err != nil {
		t.Fatal(err)
	}
	fresh := read(t, Store{Root: s.Root})
	if fresh.Generation != after.Generation || fresh.Resolve("note-20260923-00000001").Kind != "found" {
		t.Fatal("cache deletion lost memory")
	}
}

func TestUnsafePathsAndSymlinks(t *testing.T) {
	for _, p := range []string{"../outside.md", "/tmp/outside.md", "notes/../../outside.md", "notes\\evil.md", ".runtime/writer.lock", "transactions/fake/manifest.json"} {
		t.Run(p, func(t *testing.T) {
			s := Store{Root: t.TempDir()}
			res, err := s.Publish(context.Background(), request(read(t, s), "unsafe", Output{p, []byte("bad")}))
			if err != nil || res.Kind != "invalid" {
				t.Fatal(res, err)
			}
		})
	}
	root, outside := t.TempDir(), t.TempDir()
	direct(t, root, "notes/placeholder", []byte("x"))
	if err := os.RemoveAll(filepath.Join(root, ".haft", "notes")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, ".haft", "notes")); err != nil {
		t.Fatal(err)
	}
	s := Store{Root: root}
	v := read(t, s)
	if v.Coverage != "degraded" || !hasDiagnostic(v.Diagnostics, "symlink_not_read") {
		t.Fatal("symlink coverage concealed")
	}
	res, err := s.Publish(context.Background(), request(v, "escape", Output{"notes/out.md", []byte("bad")}))
	if err != nil || res.Kind == "written" {
		t.Fatal(res, err)
	}
	if _, err := os.Stat(filepath.Join(outside, "out.md")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("escaped root")
	}
	root = t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, ".haft")); err != nil {
		t.Fatal(err)
	}
	_, err = Store{Root: root}.Read(context.Background())
	if err == nil {
		t.Fatal("symlink store root accepted")
	}
}

func TestSearchTypedLinksAndUnknownReadability(t *testing.T) {
	s := Store{Root: t.TempDir()}
	raw, err := os.ReadFile("../carrier/testdata/valid/spec.md")
	if err != nil {
		t.Fatal(err)
	}
	terms, err := os.ReadFile("../carrier/testdata/terms.md")
	if err != nil {
		t.Fatal(err)
	}
	direct(t, s.Root, "specs/spec-20260923-00000001.md", raw)
	direct(t, s.Root, "specs/terms.md", terms)
	direct(t, s.Root, "notes/broken.md", []byte("---\nformat: future/99\n---\nCancel remains searchable\n"))
	v := read(t, s)
	search := v.Search("Cancel", 1)
	if !search.Truncated || len(search.Hits) != 1 || search.Generation != v.Generation {
		t.Fatal("search truncation/coverage missing")
	}
	search = v.Search("remains searchable", 10)
	if len(search.Hits) != 1 || search.Hits[0].State != carrier.Invalid {
		t.Fatal("invalid content omitted")
	}
	edges := v.Backlinks("sym:internal/order/cancel.go::Order.Cancel")
	if len(edges) != 1 || edges[0].Kind != "implementation" || edges[0].Scope == "" {
		t.Fatal("implementation relation lost", edges)
	}
	constraints := v.Backlinks("file:internal/order/cancel.go")
	if len(constraints) != 1 || constraints[0].Kind != "constraint" {
		t.Fatal("implementation and constraint collapsed")
	}
}

func TestTwoInstancesSerializeWithOneConflict(t *testing.T) {
	root := t.TempDir()
	s := Store{Root: root}
	v := read(t, s)
	start := make(chan struct{})
	results := make(chan Result, 2)
	errorsCh := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 1; i <= 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			id := fmt.Sprintf("note-20260923-%08x", i)
			r := request(v, fmt.Sprintf("r%d", i), Output{"notes/" + id + ".md", note(id, "concurrent")})
			result, err := (Store{Root: root}).Publish(context.Background(), r)
			results <- result
			errorsCh <- err
		}(i)
	}
	close(start)
	wg.Wait()
	close(results)
	close(errorsCh)
	counts := map[string]int{}
	for r := range results {
		counts[r.Kind]++
	}
	for err := range errorsCh {
		if err != nil {
			t.Fatal(err)
		}
	}
	if counts["written"] != 1 || counts["concurrent_write"] != 1 {
		t.Fatal(counts)
	}
}

func TestStoreProcessHelper(t *testing.T) {
	if os.Getenv("HAFT_STORE_HELPER") != "1" {
		return
	}
	args := os.Args
	split := 0
	for i, a := range args {
		if a == "--" {
			split = i
			break
		}
	}
	root, mode := args[split+1], args[split+2]
	s := Store{Root: root, LockTimeout: time.Second}
	if mode == "lock" {
		h, err := s.open(context.Background(), true)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(3)
		}
		defer h.close()
		fmt.Println("locked")
		for {
			time.Sleep(time.Hour)
		}
	}
	v, err := s.Read(context.Background())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(3)
	}
	if mode == "crash" {
		s.Fault = func(p Point) error {
			if p.Stage == "published" {
				os.Exit(41)
			}
			return nil
		}
	}
	id := "note-20260923-00000001"
	key := "process"
	if mode == "publish" {
		id = args[split+3]
		key = id
		v.Generation = args[split+4]
	}
	r := request(v, key, Output{"notes/" + id + ".md", note(id, "process publication")})
	result, err := s.Publish(context.Background(), r)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(3)
	}
	_ = json.NewEncoder(os.Stdout).Encode(result)
	os.Exit(0)
}
func helper(root, mode string, args ...string) *exec.Cmd {
	command := append([]string{"-test.run=^TestStoreProcessHelper$", "--", root, mode}, args...)
	cmd := exec.Command(os.Args[0], command...)
	cmd.Env = append(os.Environ(), "HAFT_STORE_HELPER=1")
	return cmd
}

func TestKilledProcessReleasesOSLock(t *testing.T) {
	root := t.TempDir()
	cmd := helper(root, "lock")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	ready := make(chan bool, 1)
	go func() { scan := bufio.NewScanner(stdout); ready <- scan.Scan() && scan.Text() == "locked" }()
	select {
	case ok := <-ready:
		if !ok {
			t.Fatal("helper did not lock")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("helper timed out")
	}
	s := Store{Root: root, LockTimeout: 50 * time.Millisecond}
	_, err = s.Read(context.Background())
	if err == nil || !strings.Contains(err.Error(), "lock_timeout") {
		t.Fatal("second process ignored lock", err)
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	s.LockTimeout = time.Second
	v := read(t, s)
	publish(t, s, request(v, "after-kill", Output{"notes/note-20260923-00000002.md", note("note-20260923-00000002", "after kill")}))
}

func TestActualProcessCrashLeavesRecoverableBatch(t *testing.T) {
	root := t.TempDir()
	cmd := helper(root, "crash")
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("helper did not crash")
	}
	if ee, ok := err.(*exec.ExitError); !ok || ee.ExitCode() != 41 {
		t.Fatalf("unexpected crash: %s %v", output, err)
	}
	s := Store{Root: root}
	v := read(t, s)
	if len(v.Documents) != 0 || !hasDiagnostic(v.Diagnostics, "publication_pending") {
		t.Fatal("crashed process published partial batch")
	}
	res, err := s.Recover(context.Background())
	if err != nil || res.Kind != "recovered" || len(read(t, s).Documents) != 1 {
		t.Fatal(res, err)
	}
}

func TestTwoActualProcessesShareOneWriterBoundary(t *testing.T) {
	root := t.TempDir()
	v := read(t, Store{Root: root})
	commands := []*exec.Cmd{helper(root, "publish", "note-20260923-00000001", v.Generation), helper(root, "publish", "note-20260923-00000002", v.Generation)}
	var outputs [2]bytes.Buffer
	for i, cmd := range commands {
		cmd.Stdout = &outputs[i]
		cmd.Stderr = &outputs[i]
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
	}
	counts := map[string]int{}
	for i, cmd := range commands {
		if err := cmd.Wait(); err != nil {
			t.Fatal(err, outputs[i].String())
		}
		var result Result
		if err := json.Unmarshal(outputs[i].Bytes(), &result); err != nil {
			t.Fatal(err, outputs[i].String())
		}
		counts[result.Kind]++
	}
	if counts["written"] != 1 || counts["concurrent_write"] != 1 {
		t.Fatal(counts)
	}
}

func TestChangeBytesAndDotfileBasisArePreserved(t *testing.T) {
	s := Store{Root: t.TempDir()}
	direct(t, s.Root, ".gitignore", []byte(".runtime/\n.cache/\n"))
	change := []byte("---\nformat: haft.change/1\nid: chg-20260923-00000001\n---\nIntent and tasks\n")
	path := "changes/chg-20260923-00000001.md"
	publish(t, s, request(read(t, s), "change", Output{path, change}))
	v := read(t, s)
	if !bytes.Equal(v.Files[path], change) || len(v.Documents) != 0 || v.Editions[path] == "" {
		t.Fatal("generic change publication entered six-kind projection or lost bytes")
	}
	if _, ok := v.CurrentSnapshots[v.Editions[path]]; !ok {
		t.Fatal("change snapshot missing")
	}
}

func TestExternalEditBeforeCommitAndCacheDeletionDuringWrite(t *testing.T) {
	s := Store{Root: t.TempDir()}
	direct(t, s.Root, "notes/note-20260923-00000001.md", note("note-20260923-00000001", "before"))
	direct(t, s.Root, ".cache/old", []byte("cache"))
	v := read(t, s)
	s.Fault = func(p Point) error {
		if p.Stage == "published" {
			return os.RemoveAll(filepath.Join(s.Root, ".haft", ".cache"))
		}
		return nil
	}
	publish(t, s, request(v, "cache-delete", Output{"notes/note-20260923-00000002.md", note("note-20260923-00000002", "cache independent")}))
	s.Fault = nil
	v = read(t, s)
	s.Fault = func(p Point) error {
		if p.Stage == "before_commit" {
			direct(t, s.Root, "notes/note-20260923-00000001.md", []byte("external change"))
		}
		return nil
	}
	res, err := s.Publish(context.Background(), request(v, "external", Output{"notes/note-20260923-00000003.md", note("note-20260923-00000003", "pending")}))
	if err != nil || res.Kind != "recovery_conflict" {
		t.Fatal("external edit was missed", res, err)
	}
	s.Fault = nil
	v = read(t, s)
	if _, exists := v.Files["notes/note-20260923-00000003.md"]; exists {
		t.Fatal("partial output leaked after external edit")
	}
}

func TestPortableSnapshotReadInAnotherRootWithoutCache(t *testing.T) {
	s := Store{Root: t.TempDir()}
	raw := note("note-20260923-00000001", "portable history")
	_, snapshot, digest, err := carrier.NewSnapshot(raw, carrier.InterpretationBasis{})
	if err != nil {
		t.Fatal(err)
	}
	p := "editions/sha256/" + strings.TrimPrefix(digest, "sha256:") + ".json"
	publish(t, s, request(read(t, s), "portable", Output{"notes/note-20260923-00000001.md", raw}, Output{p, snapshot}))
	clone := t.TempDir()
	source := filepath.Join(s.Root, ".haft")
	err = filepath.WalkDir(source, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, e := filepath.Rel(source, p)
		if e != nil {
			return e
		}
		if rel == ".runtime" || rel == ".cache" {
			return fs.SkipDir
		}
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(clone, ".haft", rel), 0755)
		}
		b, e := os.ReadFile(p)
		if e != nil {
			return e
		}
		return os.WriteFile(filepath.Join(clone, ".haft", rel), b, 0644)
	})
	if err != nil {
		t.Fatal(err)
	}
	v := read(t, Store{Root: clone})
	if v.Coverage != "complete" || v.Resolve("note-20260923-00000001@"+digest).Kind != "found" {
		t.Fatal("portable snapshot required prior cache/process", v.Diagnostics)
	}
}

func TestCorruptJournalNeverProjectsUnknownPartialBatch(t *testing.T) {
	s := Store{Root: t.TempDir()}
	result := publish(t, s, request(read(t, s), "journal", Output{"notes/note-20260923-00000001.md", note("note-20260923-00000001", "journal")}))
	direct(t, s.Root, "transactions/"+result.TransactionID+"/commit.json", []byte("incomplete marker"))
	v := read(t, s)
	if v.Coverage != "unavailable" || len(v.Documents) != 0 || !hasDiagnostic(v.Diagnostics, "invalid_journal") {
		t.Fatal("corrupt commit marker hid uncertainty", v)
	}
	res, err := s.Recover(context.Background())
	if err != nil || res.Kind != "recovery_conflict" {
		t.Fatal(res, err)
	}
}

func TestMovingTreeHasBoundedDegradedObservation(t *testing.T) {
	s := Store{Root: t.TempDir(), ScanAttempts: 2}
	p := "notes/note-20260923-00000001.md"
	raw := note("note-20260923-00000001", "first")
	direct(t, s.Root, p, raw)
	observations := 0
	s.Fault = func(point Point) error {
		if point.Stage == "between_scans" {
			observations++
			text := "other"
			if observations%2 == 0 {
				text = "first"
			}
			direct(t, s.Root, p, bytes.ReplaceAll(raw, []byte("first"), []byte(text)))
		}
		return nil
	}
	v := read(t, s)
	if observations != 2 || v.Coverage != "degraded" || !hasDiagnostic(v.Diagnostics, "unstable_scan") {
		t.Fatal("moving tree claimed stable/atomic capture", observations, v.Diagnostics)
	}
}

func TestReadAndPublicationBudgetsNeverImplyEmptyCompleteScope(t *testing.T) {
	root := t.TempDir()
	direct(t, root, "notes/large.md", bytes.Repeat([]byte("x"), 128))
	v := read(t, Store{Root: root, MaxFileBytes: 64})
	if v.Coverage != "degraded" || !hasDiagnostic(v.Diagnostics, "unreadable_path") || len(v.Files) != 0 {
		t.Fatal("oversized unread content appeared complete", v.Diagnostics)
	}
	s := Store{Root: t.TempDir(), MaxTotalBytes: 64}
	r := request(read(t, s), "over-budget", Output{"notes/large.md", bytes.Repeat([]byte("x"), 128)})
	res, err := s.Publish(context.Background(), r)
	if err != nil || res.Kind != "invalid" || !hasDiagnostic(res.Diagnostics, "publication_budget_exceeded") {
		t.Fatal("unreadable-size publication accepted", res, err)
	}
	if len(read(t, s).Files) != 0 {
		t.Fatal("budget failure published bytes")
	}
}
