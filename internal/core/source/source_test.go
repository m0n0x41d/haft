package source

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/m0n0x41d/haft/internal/core/carrier"
)

func fixtureFiles(t *testing.T, dir string) []File {
	t.Helper()
	root := filepath.Join("testdata", dir)
	var files []File
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		files = append(files, File{filepath.ToSlash(rel), raw})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}
func fixtureReader(t *testing.T, dir string) *Reader {
	t.Helper()
	r, err := NewReader("synthetic-source-fixture", fixtureFiles(t, dir), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func copyFixture(t *testing.T, dir string) string {
	t.Helper()
	root := t.TempDir()
	for _, f := range fixtureFiles(t, dir) {
		p := filepath.Join(root, filepath.FromSlash(f.Path))
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, f.Raw, 0644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}
func requireFound(t *testing.T, r *Reader, ref string) *Unit {
	t.Helper()
	i := r.Inspect(ref)
	if i.Kind != "found" || i.Unit == nil {
		t.Fatalf("%s: %+v", ref, i)
	}
	return i.Unit
}
func hasDiagnostic(ds []carrier.Diagnostic, code string) bool {
	for _, d := range ds {
		if d.Code == code {
			return true
		}
	}
	return false
}

func TestWholePatternExpectedFixtureManifest(t *testing.T) {
	var manifest struct {
		Expected []struct {
			Pin         string `json:"pin"`
			Ref         string `json:"ref"`
			Publication string `json:"publication_path"`
			Lines       []int  `json:"lines"`
			Bytes       int    `json:"bytes"`
			Digest      string `json:"body_digest"`
		} `json:"expected_inspect"`
	}
	b, err := os.ReadFile("testdata/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(b, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Expected) != 6 {
		t.Fatalf("expected six independent authored spans; got %d", len(manifest.Expected))
	}
	readers := map[string]*Reader{"pin-a": fixtureReader(t, "pin-a"), "pin-b": fixtureReader(t, "pin-b")}
	for _, want := range manifest.Expected {
		t.Run(want.Pin+"/"+want.Ref, func(t *testing.T) {
			u := requireFound(t, readers[want.Pin], want.Ref)
			if u.Kind != "pattern" || u.Source.Ref != want.Ref || u.Source.PublicationPath != want.Publication || !reflect.DeepEqual(u.Source.Lines, want.Lines) || len(u.Raw) != want.Bytes || u.Source.BodyDigest != want.Digest {
				t.Fatalf("wrong whole body: %+v bytes=%d", u.Source, len(u.Raw))
			}
			original, err := os.ReadFile(filepath.Join("testdata", want.Pin, filepath.FromSlash(want.Publication)))
			if err != nil {
				t.Fatal(err)
			}
			lines := bytes.SplitAfter(original, []byte{'\n'})
			body := bytes.Join(lines[want.Lines[0]-1:want.Lines[1]], nil)
			if !bytes.Equal(u.Raw, body) {
				t.Fatal("raw source span changed")
			}
			if want.Pin == "pin-b" && want.Ref == "SYSE.31" && bytes.Count(u.Raw, []byte("\r\n")) != 18 {
				t.Fatal("CRLF lost")
			}
			s, err := carrier.ReadSourceSnapshot(u.Snapshot, u.Source.SnapshotRef)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(s.Raw, u.Raw) || s.Provenance.BodyDigest != u.Source.BodyDigest {
				t.Fatal("snapshot does not preserve read body")
			}
		})
	}
	for _, r := range readers {
		if r.Status().Kind != "available" || r.Status().Patterns != 3 || r.Status().Publications != 6 {
			t.Fatalf("unexpected fixture status: %+v", r.Status())
		}
	}
}

func TestExactIdentityNavigationAndFencedBoundaries(t *testing.T) {
	r := fixtureReader(t, "pin-a")
	for _, ref := range []string{"A.99", "A.100", "A", "A.1:4", "A.1 ", "../FPF-Spec.md"} {
		if r.Inspect(ref).Kind != "unavailable" {
			t.Fatalf("exact inspect broadened %q", ref)
		}
	}
	for alias, p := range map[string]string{"fpf-usage-guide": "USING-FPF.md", "fpf-ecosystem": "Readme.md", "engineering-suite": "Engineering DPF Suite/README.md", "engineering-suite-reference": "Engineering DPF Suite/ENGINEERING-DPF-SUITE-REFERENCE.md"} {
		u := requireFound(t, r, alias)
		exact := requireFound(t, r, p)
		if u.Kind != "document" || u.Source.Ref != p || !bytes.Equal(u.Raw, exact.Raw) {
			t.Fatalf("document alias invented identity: %+v", u.Source)
		}
	}
	u := requireFound(t, r, "A.1")
	if !bytes.Contains(u.Raw, []byte("Conformance checklist")) || !bytes.Contains(u.Raw, []byte("## A.99")) || bytes.Contains(u.Raw, []byte("Inter-pattern prose")) {
		t.Fatal("fence or end boundary wrong")
	}
	// Tilde fences and a longer closing fence obey the same rule.
	raw := []byte("## C.2.2a - Mixed case\n   ~~~md\n## A.90 - Fake\n### C.2.2a:End\n   ~~~~\n### C.2.2a:End")
	x, err := NewReader("authored", []File{{"FPF-Spec.md", raw}}, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	got := requireFound(t, x, "C.2.2a")
	if !bytes.Equal(got.Raw, raw) || got.Source.Lines[1] != 6 || x.Inspect("A.90").Kind != "unavailable" {
		t.Fatal("mixed case or tilde boundary failed")
	}
}

func TestMalformedBoundariesNeverBecomeWholePatterns(t *testing.T) {
	for _, tc := range []struct{ dir, kind, code string }{
		{"duplicate-within", "ambiguous", "ambiguous_pattern"},
		{"duplicate-across", "ambiguous", "ambiguous_pattern"},
		{"missing-end", "invalid", "missing_pattern_end"},
		{"mismatched-end", "invalid", "mismatched_pattern_end"},
		{"duplicate-end", "invalid", "duplicate_pattern_end"},
	} {
		t.Run(tc.dir, func(t *testing.T) {
			r := fixtureReader(t, "malformed/"+tc.dir)
			got := r.Inspect("A.1")
			if got.Kind != tc.kind || got.Unit != nil || !hasDiagnostic(got.Diagnostics, tc.code) {
				t.Fatalf("malformed body trusted: %+v", got)
			}
			if r.Status().Kind != "degraded" {
				t.Fatal("status hid malformed input")
			}
			for _, candidate := range r.Search("fixture", 50).Candidates {
				if candidate.Ref == "A.1" {
					t.Fatal("malformed body entered search")
				}
			}
		})
	}
	u := requireFound(t, fixtureReader(t, "malformed/missing-end"), "A.10")
	if bytes.Contains(u.Raw, []byte("## A.1 -")) {
		t.Fatal("later body absorbed malformed predecessor")
	}
}

func TestImmutableReadersPinsAndConcurrentCopies(t *testing.T) {
	files := fixtureFiles(t, "pin-a")
	r, err := NewReader("fixture", files, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	before := requireFound(t, r, "A.1")
	other := fixtureReader(t, "pin-b")
	b := requireFound(t, other, "A.1")
	if before.Source.SourceRevision.TreeDigest == b.Source.SourceRevision.TreeDigest || before.Source.BodyDigest == b.Source.BodyDigest {
		t.Fatal("different pins collapsed")
	}
	for i := range files {
		for j := range files[i].Raw {
			files[i].Raw[j] = 'x'
		}
	}
	u := requireFound(t, r, "A.1")
	u.Raw[0] = 'x'
	u.Snapshot[0] = 'x'
	u.Source.Lines[0] = 999
	s := r.Status()
	s.Manifest[0].Digest = "tampered"
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				a := r.Inspect("A.1")
				z := other.Inspect("A.1")
				if a.Unit == nil || z.Unit == nil || !bytes.Equal(a.Unit.Raw, before.Raw) || !bytes.Equal(z.Unit.Raw, b.Raw) {
					t.Error("readers mixed or caller mutated internal bytes")
				}
				r.Search("subject", 2)
				r.Status()
			}
		}()
	}
	wg.Wait()
	if !bytes.Equal(requireFound(t, r, "A.1").Raw, before.Raw) || r.Status().Manifest[0].Digest == "tampered" {
		t.Fatal("public data aliases reader memory")
	}
}

func TestManifestUsesEveryByteAndStableOrder(t *testing.T) {
	files := fixtureFiles(t, "pin-a")
	a, err := NewReader("first", files, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	for i, j := 0, len(files)-1; i < j; i, j = i+1, j-1 {
		files[i], files[j] = files[j], files[i]
	}
	b, err := NewReader("first", files, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a.Status().Revision, b.Status().Revision) {
		t.Fatal("input ordering changed content identity")
	}
	files[0].Raw = append(files[0].Raw, ' ')
	c, err := NewReader("first", files, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if c.Status().Revision.TreeDigest == a.Status().Revision.TreeDigest {
		t.Fatal("changed document bytes did not change tree identity")
	}
}

func TestLexicalSearchBoundedAdvisoryAndExactInspect(t *testing.T) {
	r := fixtureReader(t, "pin-a")
	result := r.Search("subject observation feedback", 1)
	if result.Kind != "results" || !result.Advisory || len(result.Candidates) != 1 || !result.Truncated || result.TotalMatches <= 1 {
		t.Fatalf("search bounds: %+v", result)
	}
	for _, candidate := range result.Candidates {
		u := requireFound(t, r, candidate.Ref)
		if !reflect.DeepEqual(u.Source, candidate.Source) {
			t.Fatal("search provenance differs from inspected source")
		}
	}
	if r.Search("несуществующийлексическийключ", 10).Kind != "insufficient_basis" || r.Search("", 10).Kind != "insufficient_basis" {
		t.Fatal("no lexical matches became applicability")
	}
	if r.Search(strings.Repeat("q", 4097), 1).Kind != "invalid" || r.Search("test", -1).Kind != "invalid" {
		t.Fatal("query limits ignored")
	}
	terms := make([]string, 65)
	for i := range terms {
		terms[i] = fmt.Sprintf("term%d", i)
	}
	if r.Search(strings.Join(terms, " "), 1).Kind != "invalid" {
		t.Fatal("term budget ignored")
	}
	all := r.Search("fixture", 5000)
	if len(all.Candidates) > 50 {
		t.Fatal("result budget exceeded")
	}
}

func TestAmbiguousNavigationAliasRetainsExactDocuments(t *testing.T) {
	r, err := NewReader("fixture", []File{{"Readme.md", []byte("# First\n")}, {"README.md", []byte("# Second\n")}}, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if r.Status().Kind != "degraded" || r.Inspect("fpf-ecosystem").Kind != "ambiguous" {
		t.Fatal("colliding document alias silently chose one")
	}
	if string(requireFound(t, r, "Readme.md").Raw) == string(requireFound(t, r, "README.md").Raw) {
		t.Fatal("exact documents merged")
	}
}

func TestPortableReplayNoOriginalAndTamper(t *testing.T) {
	root := copyFixture(t, "pin-a")
	r, err := Capture(root, "authored-fixture", Limits{})
	if err != nil {
		t.Fatal(err)
	}
	u := requireFound(t, r, "A.1")
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	if _, err := Capture(root, "authored-fixture", Limits{}); err == nil {
		t.Fatal("missing source silently fell back")
	}
	replay := InspectSnapshot(u.Snapshot, u.Source.SnapshotRef)
	if replay.Kind != "found" || replay.Unit.Kind != "captured_body" || !bytes.Equal(replay.Unit.Raw, u.Raw) || !reflect.DeepEqual(replay.Unit.Source, u.Source) {
		t.Fatalf("offline replay failed: %+v", replay)
	}
	if InspectSnapshot(nil, u.Source.SnapshotRef).Kind != "unavailable" {
		t.Fatal("absent portable bytes repaired")
	}
	tampered := bytes.Clone(u.Snapshot)
	tampered[20] ^= 1
	if InspectSnapshot(tampered, u.Source.SnapshotRef).Kind != "invalid" {
		t.Fatal("tampered provenance or bytes accepted")
	}
	var none *Reader
	if none.Status().Kind != "unavailable" || none.Inspect("A.1").Kind != "unavailable" || none.Search("subject", 1).Kind != "unavailable" {
		t.Fatal("missing reader did not stay unavailable")
	}
	unknown := carrier.Source{Ref: "unrecovered-locator", BodyDigest: carrier.Digest([]byte("excerpt")), SourceRevision: carrier.SourceRevision{Kind: "unknown", Reason: "Original revision was not retained"}}
	_, blob, digest, err := carrier.NewSourceSnapshot([]byte("excerpt"), unknown)
	if err != nil {
		t.Fatal(err)
	}
	got := InspectSnapshot(blob, digest)
	if got.Kind != "found" || got.Unit.Source.SourceRevision.Kind != "unknown" || got.Unit.Source.SourceRevision.Commit != "" {
		t.Fatal("unknown replay acquired a revision")
	}
}

func TestInputLimitsAndPublicationIdentity(t *testing.T) {
	valid := File{"FPF-Spec.md", []byte("# Source\n")}
	for _, tc := range []struct {
		files  []File
		limits Limits
	}{
		{nil, Limits{}}, {[]File{valid, valid}, Limits{}}, {[]File{{"../FPF-Spec.md", valid.Raw}}, Limits{}}, {[]File{{"Engineering DPF Suite/../escape.md", valid.Raw}}, Limits{}}, {[]File{{"/FPF-Spec.md", valid.Raw}}, Limits{}}, {[]File{{"FPF-Spec.md", []byte{0xff}}}, Limits{}}, {[]File{valid}, Limits{MaxFileBytes: 1}}, {[]File{valid}, Limits{MaxBytes: 1}}, {[]File{valid}, Limits{MaxFiles: -1}},
	} {
		if _, err := NewReader("fixture", tc.files, tc.limits); err == nil {
			t.Fatalf("invalid capture accepted: %+v", tc)
		}
	}
	if _, err := NewReader("", []File{valid}, Limits{}); err == nil {
		t.Fatal("invented repository identity")
	}
}
