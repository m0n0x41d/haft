package store

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/m0n0x41d/haft/internal/core/carrier"
)

func (s Store) readJournals(root *os.Root) ([]journal, []carrier.Diagnostic, error) {
	if err := rejectSymlinks(root, "transactions", true); err != nil {
		return nil, nil, err
	}
	dir, err := root.Open("transactions")
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	entries, err := dir.ReadDir(-1)
	_ = dir.Close()
	if err != nil {
		return nil, nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	var out []journal
	var ds []carrier.Diagnostic
	seenRequests := map[string]string{}
	for _, entry := range entries {
		if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			return nil, ds, fmt.Errorf("unexpected journal entry: %s", entry.Name())
		}
		txn := entry.Name()
		if !validTransactionID(txn) {
			return nil, ds, fmt.Errorf("invalid transaction directory: %s", txn)
		}
		base := "transactions/" + txn
		raw, err := regularBytes(root, base+"/manifest.json", s.maxFile())
		if errors.Is(err, fs.ErrNotExist) {
			ds = append(ds, diag("incomplete_staging", base, "No manifest exists; no outputs could have been published by this staging directory"))
			continue
		}
		if err != nil {
			return nil, ds, err
		}
		var m manifest
		if err := decodeJSON(raw, &m); err != nil {
			return nil, ds, err
		}
		if err := validateManifest(m, txn); err != nil {
			return nil, ds, err
		}
		if m.RequestID != "" {
			if old, ok := seenRequests[m.RequestID]; ok {
				return nil, ds, fmt.Errorf("duplicate request journal: %s and %s", old, txn)
			}
			seenRequests[m.RequestID] = txn
		}
		j := journal{Manifest: m, Raw: raw}
		marker, err := regularBytes(root, base+"/commit.json", s.maxFile())
		if err == nil {
			var commit commitMarker
			if err := decodeJSON(marker, &commit); err != nil {
				return nil, ds, err
			}
			if commit.Format != "haft.transaction-commit/1" || commit.ManifestDigest != carrier.Digest(raw) {
				return nil, ds, fmt.Errorf("invalid commit marker: %s", txn)
			}
			j.Committed = true
		} else if !errors.Is(err, fs.ErrNotExist) {
			return nil, ds, err
		}
		out = append(out, j)
	}
	return out, ds, nil
}
func validTransactionID(s string) bool {
	if !strings.HasPrefix(s, "txn-") || len(s) != 36 {
		return false
	}
	_, err := hex.DecodeString(s[4:])
	return err == nil && strings.ToLower(s) == s
}
func validateManifest(m manifest, id string) error {
	if m.Format != "haft.transaction/1" || m.TransactionID != id || !carrier.ValidDigest(m.PayloadDigest) || !carrier.ValidDigest(m.ExpectedGeneration) || len(m.Outputs) == 0 {
		return fmt.Errorf("invalid transaction manifest: %s", id)
	}
	for p, d := range m.Inputs {
		if safePath(p) != nil || !carrier.ValidDigest(d) {
			return fmt.Errorf("invalid input basis: %s", p)
		}
	}
	seen := map[string]bool{}
	for i, o := range m.Outputs {
		if safePath(o.Path) != nil || !carrier.ValidDigest(o.Digest) || seen[o.Path] || o.StagePath != fmt.Sprintf("transactions/%s/staged/%06d", id, i) {
			return fmt.Errorf("invalid output manifest: %s", o.Path)
		}
		seen[o.Path] = true
		if o.Preexisting && !isSnapshotPath(o.Path) {
			return fmt.Errorf("only immutable snapshots can preexist")
		}
	}
	return nil
}
func decodeJSON(raw []byte, v any) error {
	_, ds := carrier.ParseYAML(raw)
	if carrier.HasErrors(ds) {
		return fmt.Errorf("ambiguous journal JSON: %v", ds)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return fmt.Errorf("trailing journal JSON")
	}
	return nil
}
func isSnapshotPath(p string) bool {
	return strings.HasPrefix(p, "editions/sha256/") && strings.HasSuffix(p, ".json")
}
func (s Store) publish(ctx context.Context, r Request) (Result, error) {
	if !carrier.ValidDigest(r.PayloadDigest) || carrier.Digest(r.RequestPayload) != r.PayloadDigest {
		return Result{Kind: "invalid", Diagnostics: []carrier.Diagnostic{diag("payload_digest_mismatch", "payload_digest", "Digest must cover the exact canonical original request bytes")}}, nil
	}
	if !carrier.ValidDigest(r.ExpectedGeneration) {
		return Result{Kind: "invalid", Diagnostics: []carrier.Diagnostic{diag("expected_generation_required", "expected_generation", "Publish requires the exact captured generation")}}, nil
	}
	if len(r.Outputs) == 0 {
		return Result{Kind: "invalid", Diagnostics: []carrier.Diagnostic{diag("outputs_required", "outputs", "Publication cannot be empty")}}, nil
	}
	outputs := append([]Output{}, r.Outputs...)
	sort.Slice(outputs, func(i, j int) bool {
		a, b := isSnapshotPath(outputs[i].Path), isSnapshotPath(outputs[j].Path)
		if a != b {
			return a
		}
		return outputs[i].Path < outputs[j].Path
	})
	seen := map[string]bool{}
	for _, o := range outputs {
		if err := safePath(o.Path); err != nil {
			return Result{Kind: "invalid", Diagnostics: []carrier.Diagnostic{diag("unsafe_path", o.Path, err.Error())}}, nil
		}
		if seen[o.Path] || int64(len(o.Bytes)) > s.maxFile() {
			return Result{Kind: "invalid", Diagnostics: []carrier.Diagnostic{diag("invalid_output", o.Path, "Duplicate output or byte budget exceeded")}}, nil
		}
		seen[o.Path] = true
		if isSnapshotPath(o.Path) {
			digest := "sha256:" + strings.TrimSuffix(strings.TrimPrefix(o.Path, "editions/sha256/"), ".json")
			if !carrier.ValidDigest(digest) || carrier.Digest(o.Bytes) != digest {
				return Result{Kind: "invalid", Diagnostics: []carrier.Diagnostic{diag("snapshot_digest_mismatch", o.Path, "Snapshot output path must match exact bytes")}}, nil
			}
		}
	}
	h, err := s.open(ctx, true)
	if err != nil {
		return Result{Kind: "unavailable", Diagnostics: []carrier.Diagnostic{diag("writer_unavailable", "", err.Error())}}, err
	}
	defer h.close()
	js, _, err := s.readJournals(h.root)
	if err != nil {
		return Result{Kind: "recovery_conflict", Diagnostics: []carrier.Diagnostic{diag("invalid_journal", "transactions", err.Error())}}, nil
	}
	// Replay is checked before the old expected generation: a committed request
	// necessarily changed that generation itself.
	if r.RequestID != "" {
		for _, j := range js {
			if j.Manifest.RequestID == r.RequestID {
				if j.Manifest.PayloadDigest != r.PayloadDigest {
					return Result{Kind: "request_conflict", TransactionID: j.Manifest.TransactionID}, nil
				}
				if j.Committed {
					return s.replay(h, j)
				}
				result, err := s.complete(ctx, h, j, true)
				if err != nil || result.Kind != "recovered" {
					return result, err
				}
				return s.replay(h, j)
			}
		}
	}
	for _, j := range js {
		if !j.Committed {
			res, err := s.complete(ctx, h, j, true)
			if err != nil || res.Kind != "recovered" {
				return res, err
			}
		}
	}
	c, stable, err := s.stableCapture(ctx, h)
	if err != nil {
		return Result{Kind: "unavailable"}, err
	}
	if !stable || c.unavailable || blockingDiagnostics(c.diagnostics) {
		return Result{Kind: "unavailable", Diagnostics: c.diagnostics}, nil
	}
	gen := generation(c.files)
	if gen != r.ExpectedGeneration {
		return Result{Kind: "concurrent_write", Generation: gen}, nil
	}
	var total int64
	for _, b := range c.files {
		total += int64(len(b))
	}
	for _, o := range outputs {
		if _, ok := c.files[o.Path]; !ok {
			total += int64(len(o.Bytes))
		}
	}
	if total > s.maxTotal() {
		return Result{Kind: "invalid", Diagnostics: []carrier.Diagnostic{diag("publication_budget_exceeded", "outputs", "Publication would exceed the declared total capture budget")}}, nil
	}
	for _, o := range outputs {
		if old, ok := c.files[o.Path]; ok && (!isSnapshotPath(o.Path) || !bytes.Equal(old, o.Bytes)) {
			return Result{Kind: "path_conflict", Generation: gen, Diagnostics: []carrier.Diagnostic{diag("no_clobber", o.Path, "Existing canonical file will not be overwritten")}}, nil
		}
		if err := rejectSymlinks(h.root, o.Path, true); err != nil {
			return Result{Kind: "path_conflict", Diagnostics: []carrier.Diagnostic{diag("unsafe_path", o.Path, err.Error())}}, nil
		}
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return Result{Kind: "unavailable"}, err
	}
	id := "txn-" + hex.EncodeToString(nonce[:])
	base := "transactions/" + id
	if err := rejectSymlinks(h.root, "transactions", true); err != nil {
		return Result{Kind: "unavailable"}, err
	}
	if err := ensureDirs(h.root, "transactions", 0755); err != nil {
		return Result{Kind: "unavailable"}, err
	}
	if err := h.root.Mkdir(base, 0755); err != nil {
		return Result{Kind: "unavailable"}, err
	}
	if err := h.root.Mkdir(base+"/staged", 0755); err != nil {
		return Result{Kind: "interrupted", TransactionID: id}, err
	}
	m := manifest{Format: "haft.transaction/1", TransactionID: id, RequestID: r.RequestID, PayloadDigest: r.PayloadDigest, ExpectedGeneration: gen, InputRefs: append([]string{}, r.InputRefs...), Inputs: map[string]string{}}
	for p, b := range c.files {
		m.Inputs[p] = carrier.Digest(b)
	}
	for i, o := range outputs {
		stage := fmt.Sprintf("%s/staged/%06d", base, i)
		if err := exclusiveFile(h.root, stage, o.Bytes); err != nil {
			return Result{Kind: "interrupted", TransactionID: id}, err
		}
		_, exists := c.files[o.Path]
		m.Outputs = append(m.Outputs, manifestOutput{Path: o.Path, Digest: carrier.Digest(o.Bytes), StagePath: stage, Preexisting: exists})
	}
	raw, err := json.Marshal(m)
	if err != nil {
		return Result{Kind: "interrupted", TransactionID: id}, err
	}
	raw = append(raw, '\n')
	if err := publishBytes(h.root, base+"/manifest.json", raw); err != nil {
		return Result{Kind: "interrupted", TransactionID: id}, err
	}
	if err := syncDir(h.root, "transactions"); err != nil {
		return Result{Kind: "interrupted", TransactionID: id}, err
	}
	j := journal{Manifest: m, Raw: raw}
	if err := s.inject(Point{Stage: "staged"}); err != nil {
		return Result{Kind: "interrupted", TransactionID: id}, err
	}
	return s.complete(ctx, h, j, false)
}
func blockingDiagnostics(ds []carrier.Diagnostic) bool {
	for _, d := range ds {
		if d.Code != "incomplete_staging" && d.Code != "publication_pending" {
			return true
		}
	}
	return false
}
func (s Store) inject(p Point) error {
	if s.Fault != nil {
		return s.Fault(p)
	}
	return nil
}
func (s Store) replay(h *handle, j journal) (Result, error) {
	paths := []string{}
	for _, o := range j.Manifest.Outputs {
		raw, err := regularBytes(h.root, o.Path, s.maxFile())
		if err != nil || carrier.Digest(raw) != o.Digest {
			return Result{Kind: "replay_conflict", TransactionID: j.Manifest.TransactionID, Diagnostics: []carrier.Diagnostic{diag("published_output_changed", o.Path, "Recorded output no longer matches; replay cannot renew trust or overwrite it")}}, nil
		}
		paths = append(paths, o.Path)
	}
	c, stable, err := s.stableCapture(context.Background(), h)
	if err != nil {
		return Result{Kind: "unavailable"}, err
	}
	if !stable || c.unavailable || blockingDiagnostics(c.diagnostics) {
		return Result{Kind: "unavailable", Diagnostics: c.diagnostics}, nil
	}
	if err := h.checkIdentity(); err != nil {
		return Result{Kind: "unavailable"}, err
	}
	return Result{Kind: "replayed", Generation: generation(c.files), Paths: paths, TransactionID: j.Manifest.TransactionID}, nil
}
func (s Store) complete(ctx context.Context, h *handle, j journal, recovering bool) (Result, error) {
	conflict := func(p, msg string) (Result, error) {
		return Result{Kind: "recovery_conflict", TransactionID: j.Manifest.TransactionID, Diagnostics: []carrier.Diagnostic{diag("recovery_conflict", p, msg)}}, nil
	}
	c, stable, err := s.stableCapture(ctx, h)
	if err != nil {
		return Result{Kind: "unavailable"}, err
	}
	if !stable || c.unavailable || blockingDiagnostics(c.diagnostics) {
		return conflict("", "Cannot verify a complete stable input basis")
	}
	if len(c.files) != len(j.Manifest.Inputs) {
		return conflict("", "Canonical input set changed after staging")
	}
	for p, d := range j.Manifest.Inputs {
		if carrier.Digest(c.files[p]) != d {
			return conflict(p, "Canonical input bytes changed after staging")
		}
	}
	for _, o := range j.Manifest.Outputs {
		b, err := regularBytes(h.root, o.StagePath, s.maxFile())
		if err != nil || carrier.Digest(b) != o.Digest {
			return conflict(o.StagePath, "Staged bytes are missing or changed")
		}
		if err := rejectSymlinks(h.root, o.Path, true); err != nil {
			return conflict(o.Path, err.Error())
		}
		old, err := regularBytes(h.root, o.Path, s.maxFile())
		if err == nil && !recovering && !o.Preexisting {
			return conflict(o.Path, "A destination appeared after admission; initial publication cannot adopt it")
		}
		if err == nil && carrier.Digest(old) != o.Digest {
			return conflict(o.Path, "A foreign output occupies the intended destination")
		}
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return conflict(o.Path, err.Error())
		}
		if o.Preexisting && err != nil {
			return conflict(o.Path, "Preexisting snapshot disappeared")
		}
	}
	paths := []string{}
	for i, o := range j.Manifest.Outputs {
		if err := ctx.Err(); err != nil {
			return Result{Kind: "interrupted", TransactionID: j.Manifest.TransactionID}, err
		}
		if err := h.checkIdentity(); err != nil {
			return Result{Kind: "interrupted", TransactionID: j.Manifest.TransactionID}, err
		}
		if err := rejectSymlinks(h.root, o.Path, true); err != nil {
			return conflict(o.Path, err.Error())
		}
		if err := ensureDirs(h.root, path.Dir(o.Path), 0755); err != nil {
			return Result{Kind: "interrupted", TransactionID: j.Manifest.TransactionID}, err
		}
		staged, err := regularBytes(h.root, o.StagePath, s.maxFile())
		if err != nil || carrier.Digest(staged) != o.Digest {
			return conflict(o.StagePath, "Staged bytes changed before publication")
		}
		if err := publishBytes(h.root, o.Path, staged); err != nil {
			if !errors.Is(err, fs.ErrExist) {
				return Result{Kind: "interrupted", TransactionID: j.Manifest.TransactionID}, err
			}
			if !recovering && !o.Preexisting {
				return conflict(o.Path, "A foreign destination won initial no-clobber publication")
			}
			b, e := regularBytes(h.root, o.Path, s.maxFile())
			if e != nil || carrier.Digest(b) != o.Digest {
				return conflict(o.Path, "Concurrent foreign bytes won the no-clobber publication")
			}
		}
		if err := syncDir(h.root, path.Dir(o.Path)); err != nil {
			return Result{Kind: "interrupted", TransactionID: j.Manifest.TransactionID}, err
		}
		paths = append(paths, o.Path)
		if err := s.inject(Point{Stage: "published", Path: o.Path, Index: i}); err != nil {
			return Result{Kind: "interrupted", TransactionID: j.Manifest.TransactionID, Paths: paths}, err
		}
	}
	if err := s.inject(Point{Stage: "before_commit"}); err != nil {
		return Result{Kind: "interrupted", TransactionID: j.Manifest.TransactionID, Paths: paths}, err
	}
	// Recheck untouched inputs and all outputs immediately before the cooperative
	// visibility point. External editors do not obey the lock; no atomic CAS is
	// claimed for an adversarial filesystem mutation after this observation.
	c, stable, err = s.stableCapture(ctx, h)
	if err != nil || !stable || c.unavailable || blockingDiagnostics(c.diagnostics) {
		return conflict("", "Input verification failed before commit")
	}
	if len(c.files) != len(j.Manifest.Inputs) {
		return conflict("", "Input set changed before commit")
	}
	for p, d := range j.Manifest.Inputs {
		if carrier.Digest(c.files[p]) != d {
			return conflict(p, "Input changed before commit")
		}
	}
	for _, o := range j.Manifest.Outputs {
		b, e := regularBytes(h.root, o.Path, s.maxFile())
		if e != nil || carrier.Digest(b) != o.Digest {
			return conflict(o.Path, "Output changed before commit")
		}
	}
	if err := h.checkIdentity(); err != nil {
		return Result{Kind: "interrupted", TransactionID: j.Manifest.TransactionID}, err
	}
	marker, _ := json.Marshal(commitMarker{Format: "haft.transaction-commit/1", ManifestDigest: carrier.Digest(j.Raw)})
	marker = append(marker, '\n')
	if err := publishBytes(h.root, "transactions/"+j.Manifest.TransactionID+"/commit.json", marker); err != nil {
		return Result{Kind: "interrupted", TransactionID: j.Manifest.TransactionID}, err
	}
	if err := s.inject(Point{Stage: "committed"}); err != nil {
		return Result{Kind: "written", TransactionID: j.Manifest.TransactionID, Paths: paths, Diagnostics: []carrier.Diagnostic{diag("reply_interrupted", "", "Publication committed; retry the same request ID to recover the result")}}, err
	}
	for _, o := range j.Manifest.Outputs {
		c.files[o.Path], _ = regularBytes(h.root, o.Path, s.maxFile())
	}
	kind := "written"
	if recovering {
		kind = "recovered"
	}
	return Result{Kind: kind, Generation: generation(c.files), Paths: paths, TransactionID: j.Manifest.TransactionID}, nil
}
func (s Store) recover(ctx context.Context) (Result, error) {
	h, err := s.open(ctx, true)
	if err != nil {
		return Result{Kind: "unavailable"}, err
	}
	defer h.close()
	js, ds, err := s.readJournals(h.root)
	if err != nil {
		return Result{Kind: "recovery_conflict", Diagnostics: []carrier.Diagnostic{diag("invalid_journal", "transactions", err.Error())}}, nil
	}
	result := Result{Kind: "nothing_to_recover", Diagnostics: ds}
	for _, j := range js {
		if j.Committed {
			continue
		}
		result, err = s.complete(ctx, h, j, true)
		if err != nil || result.Kind != "recovered" {
			return result, err
		}
	}
	return result, nil
}
