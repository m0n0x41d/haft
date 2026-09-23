package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/m0n0x41d/haft/internal/core/carrier"
	"github.com/m0n0x41d/haft/internal/core/store"
)

func (s Service) now() string {
	if s.Now != nil {
		return s.Now().UTC().Format(time.RFC3339)
	}
	return time.Now().UTC().Format(time.RFC3339)
}
func (s Service) newID(kind string) (string, error) {
	if s.NewID != nil {
		return s.NewID(kind)
	}
	prefix := map[string]string{"decision": "dec", "spec": "spec", "note": "note", "problem": "prob", "evidence": "ev", "options": "opt", "change": "chg"}[kind]
	if prefix == "" {
		return "", fmt.Errorf("unknown kind %q", kind)
	}
	var nonce [4]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return "", err
	}
	day := strings.ReplaceAll(s.now()[:10], "-", "")
	return prefix + "-" + day + "-" + hex.EncodeToString(nonce[:]), nil
}
func recordPath(r carrier.Record) string {
	dir := map[string]string{"decision": "decisions", "spec": "specs", "note": "notes", "problem": "problems", "evidence": "evidence", "options": "options"}[r.Kind]
	return dir + "/" + r.ID + ".md"
}
func snapshotPath(hash string) string {
	return "editions/sha256/" + strings.TrimPrefix(hash, "sha256:") + ".json"
}
func receipt(q Request, payload []byte) *carrier.WriteReceipt {
	if q.RequestID == "" {
		return nil
	}
	return &carrier.WriteReceipt{RequestID: q.RequestID, PayloadDigest: carrier.Digest(payload)}
}
func decodeCarrier(raw []byte) (carrier.Record, []byte, error) {
	if !utf8.Valid(raw) {
		return carrier.Record{}, nil, fmt.Errorf("carrier is not valid UTF-8")
	}
	front, body, err := carrier.SplitFrontmatter(raw)
	if err != nil {
		return carrier.Record{}, nil, err
	}
	node, ds := carrier.ParseYAML(front)
	if carrier.HasErrors(ds) {
		return carrier.Record{}, nil, fmt.Errorf("invalid YAML: %v", ds)
	}
	var r carrier.Record
	if err := node.Decode(&r); err != nil {
		return r, nil, err
	}
	return r, body, nil
}

func (s Service) remember(ctx context.Context, q Request, r Result, v store.View, payload []byte) Result {
	if q.Action == "terms" {
		d := carrier.ParseTerms([]byte(q.Carrier))
		r.Diagnostics = append(r.Diagnostics, d.Diagnostics...)
		if carrier.HasErrors(d.Diagnostics) {
			r.Kind = "invalid"
			return r
		}
		return s.publish(ctx, q, r, v, payload, []store.Output{{Path: "specs/terms.md", Bytes: []byte(q.Carrier)}}, nil)
	}
	d, body, err := decodeCarrier([]byte(q.Carrier))
	if err != nil {
		return failure(r, "invalid_carrier", err.Error())
	}
	if d.Format == "" {
		d.Format = "haft/1"
	}
	if d.ID == "" {
		d.ID, err = s.newID(d.Kind)
		if err != nil {
			return failure(r, "invalid_kind", err.Error())
		}
	}
	if d.CreatedAt == "" {
		d.CreatedAt = s.now()
	}
	if d.Status == "" {
		d.Status = "proposed"
	}
	if d.Origin == "" {
		d.Origin = "agent_proposal"
	}
	d.WriteReceipt = receipt(q, payload)
	outputs := []store.Output{}
	snaps, err := snapshotInputs(v, q.Snapshots, &outputs)
	if err != nil {
		return failure(r, "invalid_snapshot_input", err.Error())
	}
	refs := []string{}
	pin := func(address string, requireExpected bool) (string, error) {
		ref, err := carrier.ParseRef(address)
		if err != nil {
			return "", err
		}
		if !ref.Pinned() {
			found := v.Projection.Resolve(address)
			if found.Kind != "found" || found.Document == nil {
				return "", fmt.Errorf("target %s: %s", address, found.Kind)
			}
			if requireExpected && q.ExpectedTargets[address] != found.Document.Edition {
				return "", fmt.Errorf("expected_target_digest differs or is missing for %s", address)
			}
			ref.RecordID = found.Document.Record.ID
			ref.Alias = ""
			ref.Digest = found.Document.Edition
		}
		snapshot, ok := snaps[ref.Digest]
		if !ok {
			return "", fmt.Errorf("exact snapshot unavailable: %s", address)
		}
		captured, err := carrier.ReadSnapshot(snapshot, ref.Digest)
		if err != nil {
			return "", err
		}
		doc := captured.Document()
		if !doc.Valid() || doc.Record.ID != ref.RecordID {
			return "", fmt.Errorf("snapshot identity mismatch: %s", address)
		}
		refs = append(refs, ref.String())
		outputs = appendSnapshot(outputs, ref.Digest, snapshot)
		return ref.String(), nil
	}
	for i, ref := range d.Supersedes {
		d.Supersedes[i], err = pin(ref, false)
		if err != nil {
			return conflict(r, "predecessor_conflict", err.Error())
		}
	}
	if d.OptionsRef != "" {
		d.OptionsRef, err = pin(d.OptionsRef, true)
		if err != nil {
			return conflict(r, "target_conflict", err.Error())
		}
	}
	for i := range d.Uses {
		d.Uses[i].Target, err = pin(d.Uses[i].Target, true)
		if err != nil {
			return conflict(r, "target_conflict", err.Error())
		}
	}
	for _, link := range d.Links {
		ref, e := carrier.ParseRef(link.Target)
		if e == nil && ref.Pinned() {
			if _, err = pin(link.Target, false); err != nil {
				return conflict(r, "link_conflict", err.Error())
			}
		}
	}
	for _, claim := range d.Claims {
		for _, input := range claim.EvidenceInputs {
			if _, err = pin(input.Ref, false); err != nil {
				return conflict(r, "evidence_input_conflict", err.Error())
			}
		}
		for _, ref := range claim.Refs {
			if strings.Contains(ref, "@") {
				if _, err = pin(ref, false); err != nil {
					return conflict(r, "claim_ref_conflict", err.Error())
				}
			}
		}
	}
	for _, src := range d.Sources {
		if src.SnapshotRef == "" {
			continue
		}
		blob, ok := snaps[src.SnapshotRef]
		if !ok {
			return conflict(r, "source_snapshot_unavailable", src.SnapshotRef)
		}
		captured, err := carrier.ReadSourceSnapshot(blob, src.SnapshotRef)
		if err != nil {
			return failure(r, "invalid_source_snapshot", err.Error())
		}
		declared, capturedSource := src, captured.Provenance
		declared.SnapshotRef, capturedSource.SnapshotRef = "", ""
		if !sameJSON(declared, capturedSource) {
			return conflict(r, "source_basis_mismatch", "Source declaration differs from its exact portable snapshot")
		}
		outputs = appendSnapshot(outputs, src.SnapshotRef, blob)
	}
	publicationBasis := publicationSnapshots(v, outputs)
	p := carrier.ProjectCaptured(v.Documents, v.CurrentSnapshots, publicationBasis)
	ds := carrier.ValidateSuccessor(d, p, publicationBasis, q.ExpectedHeads)
	r.Diagnostics = append(r.Diagnostics, ds...)
	if carrier.HasErrors(ds) {
		r.Kind = "invalid"
		return r
	}
	raw, err := carrier.Encode(d, body)
	if err != nil {
		return failure(r, "encode", err.Error())
	}
	if decoded := carrier.Parse(raw); !decoded.Valid() {
		r.Kind = "invalid"
		r.Diagnostics = append(r.Diagnostics, decoded.Diagnostics...)
		return r
	}
	outputs, _, err = addRecord(outputs, raw, d, v)
	if err != nil {
		return failure(r, "interpretation_basis", err.Error())
	}
	return s.publish(ctx, q, r, v, payload, outputs, refs)
}

func snapshotInputs(v store.View, provided map[string][]byte, outputs *[]store.Output) (map[string][]byte, error) {
	snaps := v.AllSnapshots()
	for hash, raw := range provided {
		var head struct {
			Format string `json:"format"`
		}
		if err := json.Unmarshal(raw, &head); err != nil {
			return nil, err
		}
		var err error
		switch head.Format {
		case "haft.snapshot/1":
			_, err = carrier.ReadSnapshot(raw, hash)
		case "haft.source-snapshot/1":
			_, err = carrier.ReadSourceSnapshot(raw, hash)
		default:
			err = fmt.Errorf("unsupported snapshot format")
		}
		if err != nil {
			return nil, err
		}
		snaps[hash] = raw
		*outputs = appendSnapshot(*outputs, hash, raw)
	}
	return snaps, nil
}
func appendSnapshot(outputs []store.Output, hash string, raw []byte) []store.Output {
	p := snapshotPath(hash)
	for _, o := range outputs {
		if o.Path == p {
			return outputs
		}
	}
	return append(outputs, store.Output{Path: p, Bytes: raw})
}

func publicationSnapshots(v store.View, outputs []store.Output) map[string][]byte {
	snapshots := map[string][]byte{}
	for hash, raw := range v.Snapshots {
		snapshots[hash] = raw
	}
	for _, output := range outputs {
		if strings.HasPrefix(output.Path, "editions/sha256/") && strings.HasSuffix(output.Path, ".json") {
			hash := "sha256:" + strings.TrimSuffix(strings.TrimPrefix(output.Path, "editions/sha256/"), ".json")
			if carrier.ValidDigest(hash) && carrier.Digest(output.Bytes) == hash {
				snapshots[hash] = output.Bytes
			}
		}
	}
	return snapshots
}
func interpretation(d carrier.Record, v store.View) (carrier.InterpretationBasis, error) {
	basis := carrier.InterpretationBasis{}
	if project, ok := v.Files["project.yaml"]; ok {
		node, ds := carrier.ParseYAML(project)
		if carrier.HasErrors(ds) {
			return basis, fmt.Errorf("invalid project identity")
		}
		var config struct {
			Format       string `yaml:"format"`
			RepositoryID string `yaml:"repository_id"`
		}
		if err := node.Decode(&config); err != nil || config.Format != "haft.project/1" || config.RepositoryID == "" {
			return basis, fmt.Errorf("invalid project identity")
		}
		basis.Namespace = config.RepositoryID
	}
	needs := len(d.Terms) > 0
	for _, c := range d.Claims {
		needs = needs || len(c.Terms) > 0
	}
	if needs {
		terms, ok := v.Files["specs/terms.md"]
		if !ok {
			return basis, fmt.Errorf("declared term basis is unavailable")
		}
		parsed := carrier.ParseTerms(terms)
		ds := append(parsed.Diagnostics, carrier.ValidateTermRefs(d, parsed.Terms)...)
		if carrier.HasErrors(ds) {
			return basis, fmt.Errorf("invalid term basis: %v", ds)
		}
		basis.Terms = &carrier.BasisFile{Name: "specs/terms.md", Bytes: terms}
	}
	return basis, nil
}
func addRecord(outputs []store.Output, raw []byte, d carrier.Record, v store.View) ([]store.Output, string, error) {
	basis, err := interpretation(d, v)
	if err != nil {
		return nil, "", err
	}
	_, snap, hash, err := carrier.NewSnapshot(raw, basis)
	if err != nil {
		return nil, "", err
	}
	outputs = appendSnapshot(outputs, hash, snap)
	outputs = append(outputs, store.Output{Path: recordPath(d), Bytes: raw})
	return outputs, d.ID + "@" + hash, nil
}
func sameJSON(a, b any) bool {
	x, e := json.Marshal(a)
	y, f := json.Marshal(b)
	return e == nil && f == nil && string(x) == string(y)
}
func (s Service) publish(ctx context.Context, q Request, r Result, v store.View, payload []byte, outputs []store.Output, refs []string) Result {
	if q.ExpectedGeneration != "" && q.ExpectedGeneration != v.Generation {
		return conflict(r, "concurrent_write", "Memory generation differs from the request's read basis")
	}
	sort.Strings(refs)
	result, err := s.memory().Publish(ctx, store.Request{RequestID: q.RequestID, PayloadDigest: carrier.Digest(payload), RequestPayload: payload, ExpectedGeneration: v.Generation, Outputs: outputs, InputRefs: refs})
	r = published(r, result)
	if err != nil {
		r.Diagnostics = append(r.Diagnostics, carrier.Diagnostic{Code: "publication_error", Message: err.Error(), Severity: "error"})
	}
	return r
}
