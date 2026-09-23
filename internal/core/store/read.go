package store

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/m0n0x41d/haft/internal/core/carrier"
)

type capture struct {
	files       map[string][]byte
	journals    []journal
	diagnostics []carrier.Diagnostic
	unavailable bool
	fingerprint string
}

func generation(files map[string][]byte) string {
	manifest := map[string]string{}
	for p, b := range files {
		manifest[p] = carrier.Digest(b)
	}
	b, _ := json.Marshal(struct {
		Format         string            `json:"format"`
		Interpretation string            `json:"interpretation"`
		Files          map[string]string `json:"files"`
	}{"haft.capture/1", carrier.InterpretationVersion, manifest})
	return carrier.Digest(b)
}
func (s Store) read(ctx context.Context) (View, error) {
	h, err := s.open(ctx, false)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			_, absentErr := os.Lstat(filepath.Join(s.Root, ".haft"))
			if info, e := os.Stat(s.Root); e == nil && info.IsDir() && errors.Is(absentErr, fs.ErrNotExist) {
				return buildView(capture{files: map[string][]byte{}}, true), nil
			}
		}
		return View{Coverage: "unavailable", Diagnostics: []carrier.Diagnostic{diag("store_unavailable", "", err.Error())}}, err
	}
	defer h.close()
	c, stable, err := s.stableCapture(ctx, h)
	if err != nil {
		return View{Coverage: "unavailable", Diagnostics: []carrier.Diagnostic{diag("store_unavailable", "", err.Error())}}, err
	}
	if err := h.checkIdentity(); err != nil {
		return View{Coverage: "unavailable", Diagnostics: []carrier.Diagnostic{diag("store_path_changed", "", err.Error())}}, err
	}
	return buildView(c, stable), nil
}
func (s Store) stableCapture(ctx context.Context, h *handle) (capture, bool, error) {
	attempts := s.ScanAttempts
	if attempts <= 0 {
		attempts = 3
	}
	var last capture
	for i := 0; i < attempts; i++ {
		if err := ctx.Err(); err != nil {
			return last, false, err
		}
		first, err := s.captureOnce(h)
		if err != nil {
			return last, false, err
		}
		if err := s.inject(Point{Stage: "between_scans", Index: i}); err != nil {
			return last, false, err
		}
		second, err := s.captureOnce(h)
		if err != nil {
			return last, false, err
		}
		last = second
		if first.fingerprint == second.fingerprint {
			return second, true, nil
		}
	}
	last.diagnostics = append(last.diagnostics, diag("unstable_scan", "", "Content moved during bounded repeated scans; this is not an atomic filesystem snapshot"))
	return last, false, nil
}
func (s Store) captureOnce(h *handle) (capture, error) {
	c := capture{files: map[string][]byte{}}
	js, ds, err := s.readJournals(h.root)
	c.journals = js
	c.diagnostics = append(c.diagnostics, ds...)
	if err != nil {
		c.unavailable = true
		c.diagnostics = append(c.diagnostics, diag("invalid_journal", "transactions", err.Error()))
		c.fingerprint = carrier.Digest([]byte(err.Error()))
		return c, nil
	}
	hidden := map[string]bool{}
	for _, j := range js {
		if !j.Committed {
			c.diagnostics = append(c.diagnostics, diag("publication_pending", "transactions/"+j.Manifest.TransactionID, "Unfinished transaction outputs are withheld until recovery commits the exact batch"))
			for _, o := range j.Manifest.Outputs {
				if !o.Preexisting {
					hidden[o.Path] = true
				}
			}
		}
	}
	var total int64
	err = fs.WalkDir(h.root.FS(), ".", func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			c.diagnostics = append(c.diagnostics, diag("unreadable_path", p, walkErr.Error()))
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if p == ".runtime" || p == ".cache" || p == "transactions" {
			return fs.SkipDir
		}
		if p == "." || d.IsDir() {
			return nil
		}
		if hidden[p] {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			c.diagnostics = append(c.diagnostics, diag("symlink_not_read", p, "Canonical scan never follows symlinks"))
			return nil
		}
		b, e := regularBytes(h.root, p, s.maxFile())
		if e != nil {
			c.diagnostics = append(c.diagnostics, diag("unreadable_path", p, e.Error()))
			return nil
		}
		total += int64(len(b))
		if total > s.maxTotal() {
			c.diagnostics = append(c.diagnostics, diag("scan_budget_exceeded", p, "Total capture byte budget exhausted"))
			return fs.SkipAll
		}
		c.files[p] = b
		return nil
	})
	if err != nil {
		return c, err
	}
	fingerprint := map[string]string{"@files": generation(c.files)}
	for _, j := range js {
		state := "pending"
		if j.Committed {
			state = "committed"
		}
		fingerprint[j.Manifest.TransactionID] = state + ":" + carrier.Digest(j.Raw)
	}
	for i, d := range c.diagnostics {
		fingerprint[fmt.Sprintf("@diagnostic-%d", i)] = d.Code + ":" + d.Path + ":" + d.Message
	}
	b, _ := json.Marshal(fingerprint)
	c.fingerprint = carrier.Digest(b)
	return c, nil
}
func buildView(c capture, stable bool) View {
	v := View{Files: c.files, Snapshots: map[string][]byte{}, CurrentSnapshots: map[string][]byte{}, Editions: map[string]string{}, Generation: generation(c.files), Coverage: "complete", Diagnostics: append([]carrier.Diagnostic{}, c.diagnostics...)}
	if !stable || len(c.diagnostics) > 0 {
		v.Coverage = "degraded"
	}
	if c.unavailable {
		v.Coverage = "unavailable"
		return v
	}
	keys := make([]string, 0, len(c.files))
	for p := range c.files {
		keys = append(keys, p)
	}
	sort.Strings(keys)
	for _, p := range keys {
		if !strings.HasPrefix(p, "editions/sha256/") || !strings.HasSuffix(p, ".json") {
			continue
		}
		digest := "sha256:" + strings.TrimSuffix(strings.TrimPrefix(p, "editions/sha256/"), ".json")
		b := c.files[p]
		if !carrier.ValidDigest(digest) || carrier.Digest(b) != digest {
			v.Coverage = "degraded"
			v.Diagnostics = append(v.Diagnostics, diag("corrupt_snapshot", p, "Snapshot file name does not match its complete bytes"))
			continue
		}
		var format struct {
			Format string `json:"format"`
		}
		_ = json.Unmarshal(b, &format)
		var err error
		switch format.Format {
		case "haft.snapshot/1":
			_, err = carrier.ReadSnapshot(b, digest)
		case "haft.source-snapshot/1":
			_, err = carrier.ReadSourceSnapshot(b, digest)
		default:
			err = fmt.Errorf("unsupported snapshot format %q", format.Format)
		}
		if err != nil {
			v.Coverage = "degraded"
			v.Diagnostics = append(v.Diagnostics, diag("invalid_snapshot", p, err.Error()))
			continue
		}
		v.Snapshots[digest] = bytes.Clone(b)
	}
	namespace := ""
	if raw, ok := c.files["project.yaml"]; ok {
		node, ds := carrier.ParseYAML(raw)
		if carrier.HasErrors(ds) {
			v.Coverage = "degraded"
			v.Diagnostics = append(v.Diagnostics, ds...)
		} else {
			var project struct {
				Format       string `yaml:"format"`
				RepositoryID string `yaml:"repository_id"`
			}
			if err := node.Decode(&project); err != nil || project.Format != "haft.project/1" || strings.TrimSpace(project.RepositoryID) == "" {
				v.Coverage = "degraded"
				v.Diagnostics = append(v.Diagnostics, diag("invalid_project_identity", "project.yaml", "Unrecognized project identity; local root scope is retained"))
			} else {
				namespace = project.RepositoryID
			}
		}
	}
	for _, p := range keys {
		dir := path.Dir(p)
		if dir == "changes" && strings.HasSuffix(p, ".md") {
			_, raw, digest, err := carrier.NewSnapshot(c.files[p], carrier.InterpretationBasis{Namespace: namespace})
			if err == nil {
				v.CurrentSnapshots[digest] = raw
				v.Editions[p] = digest
			}
			continue
		}
		if !strings.HasSuffix(p, ".md") || p == "specs/terms.md" || !(dir == "decisions" || dir == "specs" || dir == "notes" || dir == "problems" || dir == "evidence" || dir == "options") {
			continue
		}
		d := carrier.Parse(c.files[p])
		for _, source := range d.Record.Sources {
			if source.SourceRevision.Kind == "unknown" {
				continue
			}
			snapshot, exists := v.Snapshots[source.SnapshotRef]
			if !exists {
				v.Coverage = "degraded"
				v.Diagnostics = append(v.Diagnostics, diag("source_snapshot_unresolved", p, "Referenced source bytes are unavailable: "+source.SnapshotRef))
				continue
			}
			captured, err := carrier.ReadSourceSnapshot(snapshot, source.SnapshotRef)
			a, b := captured.Provenance.SourceRevision, source.SourceRevision
			if err != nil || captured.Provenance.BodyDigest != source.BodyDigest || captured.Provenance.Ref != source.Ref || captured.Provenance.PublicationPath != source.PublicationPath || a.Kind != b.Kind || a.Repository != b.Repository || a.Commit != b.Commit || a.TreeDigest != b.TreeDigest || a.Reason != b.Reason || !reflect.DeepEqual(captured.Provenance.Lines, source.Lines) {
				v.Coverage = "degraded"
				v.Diagnostics = append(v.Diagnostics, diag("source_snapshot_mismatch", p, "Referenced source snapshot differs from the declared source basis"))
			}
		}
		basis := carrier.InterpretationBasis{Namespace: namespace}
		needsTerms := len(d.Record.Terms) > 0
		for _, claim := range d.Record.Claims {
			needsTerms = needsTerms || len(claim.Terms) > 0
		}
		if needsTerms {
			if raw, ok := c.files["specs/terms.md"]; ok {
				basis.Terms = &carrier.BasisFile{Name: "specs/terms.md", Bytes: raw}
				terms := carrier.ParseTerms(raw)
				if carrier.HasErrors(terms.Diagnostics) {
					v.Coverage = "degraded"
					v.Diagnostics = append(v.Diagnostics, terms.Diagnostics...)
				} else {
					ds := carrier.ValidateTermRefs(d.Record, terms.Terms)
					if len(ds) > 0 {
						v.Coverage = "degraded"
						v.Diagnostics = append(v.Diagnostics, ds...)
					}
				}
			} else {
				v.Coverage = "degraded"
				v.Diagnostics = append(v.Diagnostics, diag("term_basis_unavailable", p, "Declared term dependencies lack their current interpretation carrier"))
			}
		}
		_, raw, digest, err := carrier.NewSnapshot(d.Raw, basis)
		if err != nil {
			v.Coverage = "degraded"
			v.Diagnostics = append(v.Diagnostics, diag("snapshot_unavailable", p, err.Error()))
		} else {
			d.Edition = digest
			v.CurrentSnapshots[digest] = raw
			v.Editions[p] = digest
		}
		v.Documents = append(v.Documents, d)
		v.DocumentPaths = append(v.DocumentPaths, p)
	}
	v.Projection = carrier.ProjectCaptured(v.Documents, v.CurrentSnapshots, v.Snapshots)
	for i, e := range v.Projection.Entries {
		if e.State == carrier.Invalid || len(e.Diagnostics) > 0 {
			for _, d := range e.Diagnostics {
				d.Path = v.DocumentPaths[i] + ":" + d.Path
				v.Diagnostics = append(v.Diagnostics, d)
			}
			v.Coverage = "degraded"
		}
	}
	return v
}
