package code

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/m0n0x41d/haft/internal/core/carrier"
)

func testConfig() Config {
	return Config{GOOS: "linux", GOARCH: "amd64", Toolchain: "go1.25.8", IncludeTests: true}
}
func fixtureFiles(t *testing.T) map[string][]byte {
	t.Helper()
	files := map[string][]byte{}
	for _, name := range []string{"go.mod", "order.go", "order_test.go", "policy.go"} {
		raw, err := os.ReadFile(filepath.Join("..", "testdata", "order", name))
		if err != nil {
			t.Fatal(err)
		}
		files[name] = raw
	}
	return files
}
func mustIndex(t *testing.T, files map[string][]byte) Index {
	t.Helper()
	idx, err := IndexFromFiles(files, testConfig())
	if err != nil {
		t.Fatal(err)
	}
	return idx
}

func TestOrderNavigationAndPureCapturedBytes(t *testing.T) {
	files := fixtureFiles(t)
	idx := mustIndex(t, files)
	if !idx.Complete || len(idx.Symbols) < 8 {
		t.Fatalf("index incomplete: %#v", idx.Diagnostics)
	}
	resolved := idx.Resolve("sym:order.go::Order.Cancel")
	if resolved.Kind != "exact" || resolved.Candidates[0].Kind != "method" {
		t.Fatalf("resolve: %#v", resolved)
	}
	if r := idx.Resolve("sym:order.go::Cancel"); r.Kind != "exact" {
		t.Fatalf("unqualified unique: %#v", r)
	}
	for _, selector := range []string{"file:order.go", "dir:.", "sym:policy.go::cancelable", "sym:order.go::ErrNotCancelable", "sym:order_test.go::TestCancelPreservesTotal"} {
		if r := idx.Resolve(selector); r.Kind != "exact" {
			t.Fatalf("%s: %#v", selector, r)
		}
	}
	related := idx.Related("sym:order.go::Order.Cancel")
	if !reflect.DeepEqual(related.DependencyFiles, []string{"order.go", "order_test.go", "policy.go"}) {
		t.Fatalf("siblings absent: %#v", related)
	}
	before, _ := json.Marshal(idx)
	files["order.go"][0] = 'x'
	delete(files, "policy.go")
	after, _ := json.Marshal(idx)
	if !bytes.Equal(before, after) || idx.Files[1].Raw[0] == 'x' {
		t.Fatal("index retained caller buffer")
	}
}
func TestFormattingBasisAndDependencyChanges(t *testing.T) {
	files := fixtureFiles(t)
	before := mustIndex(t, files)
	selector := "sym:order.go::Order.Cancel"
	files["order.go"] = append([]byte("// ordinary comment\n\n"), files["order.go"]...)
	after := mustIndex(t, files)
	change := Compare(before, after, selector)
	if change.Kind != "navigation_only" || !change.EvidenceBasisChanged || change.Before.Candidates[0].Anchor != change.After.Candidates[0].Anchor {
		t.Fatalf("formatting: %#v", change)
	}
	if change.After.Candidates[0].Line == change.Before.Candidates[0].Line {
		t.Fatal("line coordinates did not update")
	}
	files["policy.go"] = bytes.ReplaceAll(files["policy.go"], []byte(`status == "paid"`), []byte(`status == "shipped"`))
	dependency := mustIndex(t, files)
	change = Compare(after, dependency, selector)
	if change.Kind != "dependency_or_configuration_changed" || change.Before.Candidates[0].RawDigest != change.After.Candidates[0].RawDigest {
		t.Fatalf("dependency: %#v", change)
	}
	files["order.go"] = bytes.ReplaceAll(files["order.go"], []byte("Cancel()"), []byte("Withdraw()"))
	renamed := mustIndex(t, files)
	if Compare(dependency, renamed, selector).Kind != "unresolved" {
		t.Fatal("rename silently rebound old locator")
	}
}
func TestTokensDoNotEraseLiteralsOrDirectives(t *testing.T) {
	if syntaxDigest([]byte("func f(){ println(\"a b\") }"), false) == syntaxDigest([]byte("func f(){ println(\"ab\") }"), false) {
		t.Fatal("literal content erased")
	}
	if syntaxDigest([]byte("//go:build linux\npackage p"), false) == syntaxDigest([]byte("//go:build darwin\npackage p"), false) {
		t.Fatal("build directive erased")
	}
	if syntaxDigest([]byte("//line other.go:3\npackage p"), false) == syntaxDigest([]byte("package p"), false) {
		t.Fatal("line directive erased")
	}
	if syntaxDigest([]byte("package p\nfunc f(){return\n1}"), false) == syntaxDigest([]byte("package p\nfunc f(){return 1}"), false) {
		t.Fatal("implicit semicolon erased")
	}
}
func TestDeclarationsAmbiguityAndUnsupported(t *testing.T) {
	files := map[string][]byte{"go.mod": []byte("module example.test/m\ngo 1.25.8\n"), "a.go": []byte("package p\ntype A[T any] struct{}\ntype B struct{}\ntype Alias = B\nconst X,Y = 1,2\nvar V,W int\nfunc (a *A[T]) Run(){}\nfunc (b B) Run(){}\n"), "x.py": []byte("def Run(): pass")}
	idx := mustIndex(t, files)
	if !idx.Complete {
		t.Fatalf("parse: %#v", idx.Diagnostics)
	}
	for _, name := range []string{"A", "B", "Alias", "X", "Y", "V", "W", "A.Run", "B.Run"} {
		if r := idx.Resolve("sym:a.go::" + name); r.Kind != "exact" {
			t.Fatalf("%s: %#v", name, r)
		}
	}
	if r := idx.Resolve("sym:a.go::Run"); r.Kind != "ambiguous" || len(r.Candidates) != 2 {
		t.Fatalf("ambiguity: %#v", r)
	}
	for _, selector := range []string{"sym:x.py::Run", "sym:../a.go::Run"} {
		if idx.Resolve(selector).Kind != "unsupported" {
			t.Fatalf("not unsupported: %s", selector)
		}
	}
	files["broken.go"] = []byte("package p\nfunc Broken( {")
	broken := mustIndex(t, files)
	if broken.Complete || broken.Resolve("sym:broken.go::Broken").Kind != "unresolved" {
		t.Fatal("invalid source asserted complete")
	}
}
func TestBuildSelectionIgnoresAndReverseClosure(t *testing.T) {
	files := map[string][]byte{"go.mod": []byte("module example.test/m\ngo 1.25.8\n"), "base/base.go": []byte("package base\nfunc F(){}"), "consumer/main.go": []byte("package consumer\nimport \"example.test/m/base\"\nfunc G(){base.F()}"), "base/only_darwin.go": []byte("package base\nfunc Mac(){}"), "base/tagged.go": []byte("//go:build enterprise\n\npackage base\nfunc Tagged(){}"), "base/.gitignore": []byte("ignored.go\n!kept.go\n"), "base/ignored.go": []byte("package base\nfunc Ignored(){}"), ".haftignore": []byte("secret/\n"), "secret/no.go": []byte("package secret\nfunc Secret(){}")}
	idx := mustIndex(t, files)
	for _, s := range []string{"sym:base/only_darwin.go::Mac", "sym:base/tagged.go::Tagged", "sym:base/ignored.go::Ignored", "sym:secret/no.go::Secret"} {
		if idx.Resolve(s).Kind != "unresolved" {
			t.Fatalf("excluded path visible: %s", s)
		}
	}
	r := idx.Related("sym:base/base.go::F")
	if !reflect.DeepEqual(r.AffectedFiles, []string{"base/base.go", "consumer/main.go"}) {
		t.Fatalf("reverse closure: %#v", r)
	}
	if metadata := idx.Related("file:go.mod"); !reflect.DeepEqual(metadata.AffectedFiles, r.AffectedFiles) {
		t.Fatalf("module basis did not affect all packages: %#v", metadata)
	}
	config := testConfig()
	config.BuildTags = []string{"enterprise"}
	other, err := IndexFromFiles(files, config)
	if err != nil || other.Resolve("sym:base/tagged.go::Tagged").Kind != "exact" {
		t.Fatalf("build tags: %v %#v", err, other.Diagnostics)
	}
	files["base/ignored.go"] = []byte("not valid Go but still ignored")
	rebuilt := mustIndex(t, files)
	if !rebuilt.Complete {
		t.Fatal("ignored bytes were parsed")
	}
	delete(files, "base/.gitignore")
	if mustIndex(t, files).Complete {
		t.Fatal("ignore change did not invalidate parse selection")
	}
}
func TestConcurrentPureIndexAndBindings(t *testing.T) {
	files := fixtureFiles(t)
	idx := mustIndex(t, files)
	raw := []byte("fixture exact claim carrier bytes")
	doc := carrier.Document{Raw: raw, Record: carrier.Record{ID: "spec-20260923-12345678", Claims: []carrier.Claim{{ID: "cancel", ImplementedBy: []carrier.Binding{{Ref: "sym:order.go::Order.Cancel", Covers: "eligible state"}}, Checks: []carrier.Binding{{Ref: "pbt:order_test.go::TestCancelPreservesTotal", Covers: "total preserved"}}}}}}
	bindings := idx.Bindings([]carrier.Document{doc})
	if len(bindings) != 2 || !strings.Contains(bindings[0].Claim, "@sha256:") {
		t.Fatalf("bindings: %#v", bindings)
	}
	if bindings[0].Kind != "checks" || bindings[0].Resolution.Kind != "exact" || bindings[0].Ref != "pbt:order_test.go::TestCancelPreservesTotal" || bindings[0].Resolution.CodeSelector != "sym:order_test.go::TestCancelPreservesTotal" {
		t.Fatalf("check adapter mapping lost authored selector: %#v", bindings)
	}
	if testBinding := idx.BindingsFor("sym:order_test.go::TestCancelPreservesTotal", []carrier.Document{doc}); len(testBinding) == 0 || testBinding[0].MatchKind != "authored_exact_selector" {
		t.Fatalf("check reverse lookup: %#v", testBinding)
	}
	for _, unsupported := range []string{"manual:review", "lint:golangci"} {
		if r := idx.ResolveCheck(unsupported); r.Kind != "unsupported" || r.Diagnostics[0].Code != "unsupported_check_adapter" {
			t.Fatalf("unsupported check: %#v", r)
		}
	}
	if r := idx.ResolveCheck("test:order_test.go::TestCancelStates/new"); r.Kind != "exact" || r.Complete {
		t.Fatalf("subtest pretended static case resolution: %#v", r)
	}
	reverse := idx.BindingsFor("sym:policy.go::cancelable", []carrier.Document{doc})
	if len(reverse) != 2 {
		t.Fatalf("reverse bindings: %#v", reverse)
	}
	var wg sync.WaitGroup
	for n := 0; n < 8; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			other, err := IndexFromFiles(files, testConfig())
			if err != nil || other.Basis != idx.Basis {
				t.Errorf("independent parse differs: %v", err)
			}
			_ = idx.Resolve("sym:order.go::Order.Cancel")
		}()
	}
	wg.Wait()
}
func TestCaptureLimitsSymlinksAndFreshRebuild(t *testing.T) {
	root := t.TempDir()
	write := func(name, stringData string) {
		t.Helper()
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(stringData), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.test/m\ngo 1.25.8\n")
	write("a.go", "package p\nfunc A(){}\n")
	write(".gitignore", "*.tmp\nsub/ignored.go\n!keep.tmp\n")
	write("secret.tmp", "secret")
	write("keep.tmp", "kept")
	write("sub/ignored.go", "broken go")
	write("sub/.haftignore", "hidden.go\n")
	write("sub/hidden.go", "broken go")
	write(".haft/data", "private")
	capture, err := Capture(root, CaptureConfig{})
	if err != nil || !capture.Complete {
		t.Fatalf("capture: %v %#v", err, capture.Diagnostics)
	}
	for _, p := range []string{"secret.tmp", "sub/ignored.go", "sub/hidden.go", ".haft/data"} {
		if _, ok := capture.Files[p]; ok {
			t.Fatalf("excluded bytes captured: %s", p)
		}
	}
	if string(capture.Files["keep.tmp"]) != "kept" {
		t.Fatal("negation ignored")
	}
	first := mustIndex(t, capture.Files)
	stat, _ := os.Stat(filepath.Join(root, "a.go"))
	write("a.go", "package p\nfunc B(){}\n")
	_ = os.Chtimes(filepath.Join(root, "a.go"), stat.ModTime(), stat.ModTime())
	next, err := Capture(root, CaptureConfig{})
	if err != nil {
		t.Fatal(err)
	}
	second := mustIndex(t, next.Files)
	if first.Basis == second.Basis || second.Resolve("sym:a.go::B").Kind != "exact" {
		t.Fatal("equal mtime edit missed")
	}
	clean := mustIndex(t, next.Files)
	if !reflect.DeepEqual(clean, second) {
		t.Fatal("fresh index differs")
	}
	out := filepath.Join(t.TempDir(), "outside.go")
	_ = os.WriteFile(out, []byte("package outside"), 0644)
	if err := os.Symlink(out, filepath.Join(root, "link.go")); err != nil {
		t.Fatal(err)
	}
	limited, err := Capture(root, CaptureConfig{MaxFiles: 1})
	if err != nil || limited.Complete {
		t.Fatal("bounded capture asserted complete")
	}
	if _, ok := limited.Files["link.go"]; ok {
		t.Fatal("symlink followed")
	}
}

func TestIncrementalRefreshMatchesFreshWithoutReparsingSiblings(t *testing.T) {
	files := fixtureFiles(t)
	before := mustIndex(t, files)
	files["policy.go"] = bytes.ReplaceAll(files["policy.go"], []byte(`"paid"`), []byte(`"shipped"`))
	updated, stats, err := RefreshFromFiles(before, files, testConfig())
	if err != nil {
		t.Fatal(err)
	}
	fresh := mustIndex(t, files)
	if !reflect.DeepEqual(updated, fresh) || stats.ParsedFiles != 1 || stats.ReusedFiles != 2 {
		t.Fatalf("incremental differs or reparsed siblings: %#v", stats)
	}
	files[".haftignore"] = []byte("policy.go\n")
	filtered, stats, err := RefreshFromFiles(updated, files, testConfig())
	if err != nil || !reflect.DeepEqual(filtered, mustIndex(t, files)) || stats.ParsedFiles != 0 || stats.ReusedFiles != 2 {
		t.Fatalf("ignore refresh: %v %#v", err, stats)
	}
	if filtered.Related("sym:order.go::Order.Cancel").Complete {
		t.Fatal("ignored package sibling disappeared from closure limitations")
	}
	delete(files, "order_test.go")
	deleted, _, err := RefreshFromFiles(filtered, files, testConfig())
	if err != nil || !reflect.DeepEqual(deleted, mustIndex(t, files)) {
		t.Fatalf("delete refresh differs: %v", err)
	}
}

func TestCaptureIndexCarriesReadFailuresAndConflictingPackages(t *testing.T) {
	capture := CaptureResult{Files: fixtureFiles(t), Complete: false, Diagnostics: []Diagnostic{diagnostic("read_failure", "missing.go", "unreadable sibling")}}
	index, err := capture.Index(testConfig())
	if err != nil || index.Complete {
		t.Fatal("capture failure was hidden by successful parsing")
	}
	files := fixtureFiles(t)
	files["other.go"] = []byte("package other\nfunc F(){}\n")
	if mustIndex(t, files).Complete {
		t.Fatal("conflicting package names asserted a complete Go package")
	}
}

func TestCgoNestedModuleAndEmbedLimitsRemainVisible(t *testing.T) {
	files := map[string][]byte{"go.mod": []byte("module example.test/m\ngo 1.25.8\n"), "c.go": []byte("package p\n/* int value = 1; */\nimport \"C\"\nfunc F(){}\n"), "nested/go.mod": []byte("module example.test/n\n"), "nested/n.go": []byte("package n\nfunc N(){}\n"), "embedded.go": []byte("package p\nimport _ \"embed\"\n//go:embed value.txt\nvar Text string\n"), "value.txt": []byte("one")}
	index := mustIndex(t, files)
	if index.Resolve("sym:c.go::F").Kind != "unresolved" || index.Resolve("sym:nested/n.go::N").Kind != "unsupported" {
		t.Fatal("cgo/build/module selection not explicit")
	}
	config := testConfig()
	config.CGOEnabled = true
	cgo, err := IndexFromFiles(files, config)
	if err != nil || cgo.Resolve("sym:c.go::F").Kind != "exact" || cgo.Related("sym:c.go::F").Complete {
		t.Fatalf("cgo closure: %v %#v", err, cgo.Diagnostics)
	}
	if relation := cgo.Related("sym:embedded.go::Text"); relation.Complete || len(relation.Limits) < 2 {
		t.Fatalf("embed closure silently complete: %#v", relation)
	}
	capture := CaptureResult{Files: fixtureFiles(t), Complete: true, Config: CaptureConfig{IgnorePatterns: []string{"other/"}}}
	if _, err := capture.Index(testConfig()); err == nil {
		t.Fatal("capture policy disappeared from index basis")
	}
}
