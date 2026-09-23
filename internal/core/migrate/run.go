package migrate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/core/carrier"
	"github.com/m0n0x41d/haft/internal/core/store"
)

func run(ctx context.Context, r Request) (Result, error) {
	if !r.DryRun {
		return Result{Kind: "unsupported"}, fmt.Errorf("B1 supports dry-run with explicit staging only; live migration is not implemented")
	}
	if _, err := time.Parse(time.RFC3339, r.CreatedAt); err != nil {
		return Result{Kind: "invalid"}, fmt.Errorf("created_at must be an explicit RFC3339 migration timestamp")
	}
	if err := validateRoots(r); err != nil {
		return Result{Kind: "invalid"}, err
	}
	snapshot, err := Capture(ctx, r)
	if err != nil {
		return Result{Kind: "unavailable"}, err
	}
	if r.RequestID == "" {
		r.RequestID = "migration-" + strings.TrimPrefix(snapshot.Digest, "sha256:")
	}
	payload := requestPayload(r, snapshot.Digest)
	payloadDigest := carrier.Digest(payload)
	if err := os.MkdirAll(r.OutputRoot, 0755); err != nil {
		return Result{Kind: "unavailable"}, err
	}
	s := store.Store{Root: r.OutputRoot}
	if replay, found, err := s.Replay(ctx, r.RequestID, payloadDigest); found || err != nil {
		result := Result{Kind: replay.Kind, Publication: replay, ReportPath: reportPath(snapshot.Digest)}
		if err == nil && replay.Kind == "replayed" {
			view, readErr := s.Read(ctx)
			if readErr != nil {
				return result, readErr
			}
			if err := json.Unmarshal(view.Files[result.ReportPath], &result.Report); err != nil {
				return result, fmt.Errorf("replayed report unavailable: %w", err)
			}
		}
		return result, err
	}
	view, err := s.Read(ctx)
	if err != nil {
		return Result{Kind: "unavailable"}, err
	}
	plan := Build(snapshot, r)
	// Pin each converted edition with the same explicit interpretation basis the
	// reader will use. Historical evidence still carries no invented target use.
	basisView := view
	basisView.Files = make(map[string][]byte, len(view.Files)+len(plan.Outputs))
	for p, raw := range view.Files {
		basisView.Files[p] = raw
	}
	for _, output := range plan.Outputs {
		basisView.Files[output.Path] = output.Bytes
	}
	var editions []store.Output
	for _, output := range plan.Outputs {
		if strings.HasPrefix(output.Path, "migration/") || output.Path == "specs/terms.md" {
			continue
		}
		d := carrier.Parse(output.Bytes)
		if !d.Valid() {
			return Result{Kind: "invalid"}, fmt.Errorf("converted carrier is invalid: %s", output.Path)
		}
		basis, err := assistanceBasis(d.Record, basisView)
		if err != nil {
			return Result{Kind: "invalid"}, err
		}
		_, raw, digest, err := carrier.NewSnapshot(output.Bytes, basis)
		if err != nil {
			return Result{Kind: "invalid"}, err
		}
		editions = append(editions, store.Output{Path: "editions/sha256/" + strings.TrimPrefix(digest, "sha256:") + ".json", Bytes: raw})
	}
	plan.Outputs = append(plan.Outputs, editions...)
	reportRaw, err := json.MarshalIndent(plan.Report, "", "  ")
	if err != nil {
		return Result{Kind: "invalid"}, err
	}
	reportRaw = append(reportRaw, '\n')
	rp := reportPath(snapshot.Digest)
	plan.Outputs = append(plan.Outputs, store.Output{Path: rp, Bytes: reportRaw})
	// Unchanged source sidecars can be reused across a newly captured source
	// generation; they are addressed by their exact bytes. Changed existing data
	// is left to Store's no-clobber conflict, never replaced.
	outputs := []store.Output{}
	for _, output := range plan.Outputs {
		if old, exists := view.Files[output.Path]; exists && carrier.Digest(old) == carrier.Digest(output.Bytes) {
			continue
		}
		outputs = append(outputs, output)
	}
	if len(outputs) == 0 {
		return Result{Kind: "already_staged", Report: plan.Report, ReportPath: rp}, nil
	}
	publication, err := s.Publish(ctx, store.Request{RequestID: r.RequestID, PayloadDigest: payloadDigest, RequestPayload: payload, ExpectedGeneration: view.Generation, Outputs: outputs})
	result := Result{Kind: publication.Kind, Report: plan.Report, ReportPath: rp, Publication: publication}
	if publication.Kind == "written" {
		result.Kind = "staged"
	}
	return result, err
}
func requestPayload(r Request, sourceDigest string) []byte {
	raw, _ := json.Marshal(struct {
		Format       string  `json:"format"`
		SourceDigest string  `json:"source_digest"`
		Request      Request `json:"request"`
	}{"haft.migration-request/1", sourceDigest, r})
	return raw
}
func absoluteResolved(p string) (string, error) {
	absolute, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err == nil {
		return resolved, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	parent, err := absoluteResolved(filepath.Dir(absolute))
	if err != nil {
		return "", err
	}
	return filepath.Join(parent, filepath.Base(absolute)), nil
}
func contains(root, p string) bool {
	rel, err := filepath.Rel(root, p)
	return err == nil && (rel == "." || rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}
func validateRoots(r Request) error {
	if strings.TrimSpace(r.OutputRoot) == "" {
		return fmt.Errorf("explicit staging output_root required")
	}
	out, err := absoluteResolved(r.OutputRoot)
	if err != nil {
		return err
	}
	if info, e := os.Lstat(r.OutputRoot); e == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("staging root cannot be a symlink")
	}
	if filepath.Base(out) == ".haft" {
		return fmt.Errorf("output_root must be a separate staging project, not a .haft directory")
	}
	if r.DatabasePath != "" {
		db, err := absoluteResolved(r.DatabasePath)
		if err != nil {
			return err
		}
		if contains(out, db) {
			return fmt.Errorf("staging output contains original database")
		}
	}
	if r.CarrierRoot != "" {
		source, err := absoluteResolved(r.CarrierRoot)
		if err != nil {
			return err
		}
		if contains(source, out) || contains(out, source) {
			return fmt.Errorf("staging output overlaps original carrier scope")
		}
	}
	return nil
}

func queue(ctx context.Context, root string) ([]QueueItem, error) {
	view, err := (store.Store{Root: root}).Read(ctx)
	if err != nil {
		return nil, err
	}
	items := map[string]QueueItem{}
	for p, raw := range view.Files {
		if strings.HasPrefix(p, "migration/reports/") && strings.HasSuffix(p, ".json") {
			var report Report
			if err := json.Unmarshal(raw, &report); err != nil {
				return nil, fmt.Errorf("invalid migration report %s: %w", p, err)
			}
			if report.Format != "haft.migration-report/1" || !carrier.ValidDigest(report.SourceDigest) || p != reportPath(report.SourceDigest) {
				return nil, fmt.Errorf("invalid migration report identity: %s", p)
			}
			for _, q := range report.Queue {
				if !sourceAvailable(view, q.Original) {
					q.Status = "source_unavailable"
				}
				if previous, exists := items[q.ID]; exists && previous != q {
					return nil, fmt.Errorf("conflicting queue item %s across migration reports", q.ID)
				}
				items[q.ID] = q
			}
		}
	}
	for p, raw := range view.Files {
		if strings.HasPrefix(p, "migration/resolutions/") && strings.HasSuffix(p, ".json") {
			var resolution queueResolution
			if err := json.Unmarshal(raw, &resolution); err != nil {
				return nil, fmt.Errorf("invalid queue resolution %s", p)
			}
			item, ok := items[resolution.QueueID]
			if !ok {
				continue
			}
			if resolution.Format != "haft.migration-queue-resolution/1" || resolution.Source != item.Source || p != "migration/resolutions/"+item.ID+".json" {
				return nil, fmt.Errorf("queue resolution identity mismatch: %s", p)
			}
			res := view.Resolve(resolution.ProposalRef)
			if sourceAvailable(view, item.Original) && res.Kind == "found" && res.Document != nil && res.Document.Record.Origin == "agent_proposal" && res.Document.Record.Status == "proposed" && !res.Document.Record.OperatorConfirmed && res.Document.Record.Legacy["migration_queue_id"] == item.ID && res.Document.Record.Legacy["source_locator"] == item.Source && res.Document.Record.Legacy["source_bytes"] == item.Original {
				item.Status = "proposed_recovery"
				item.ProposalRef = resolution.ProposalRef
				items[item.ID] = item
			}
		}
	}
	out := []QueueItem{}
	for _, item := range items {
		out = append(out, item)
	}
	sortQueue(out)
	return out, nil
}

type queueResolution struct {
	Format      string `json:"format"`
	QueueID     string `json:"queue_id"`
	Source      string `json:"source"`
	ProposalRef string `json:"proposal_ref"`
	Reason      string `json:"reason"`
}

func assist(ctx context.Context, r AssistanceRequest) (store.Result, error) {
	if r.RequestID == "" || strings.TrimSpace(r.Reason) == "" {
		return store.Result{Kind: "invalid"}, fmt.Errorf("assistance requires request_id and an explanation of the repair")
	}
	s := store.Store{Root: r.OutputRoot}
	payload, _ := json.Marshal(r)
	digest := carrier.Digest(payload)
	if result, found, err := s.Replay(ctx, r.RequestID, digest); found || err != nil {
		return result, err
	}
	items, err := Queue(ctx, r.OutputRoot)
	if err != nil {
		return store.Result{Kind: "unavailable"}, err
	}
	var selected *QueueItem
	for _, q := range items {
		if q.ID == r.QueueID {
			copy := q
			selected = &copy
		}
	}
	if selected == nil {
		return store.Result{Kind: "invalid"}, fmt.Errorf("queue item not found")
	}
	if selected.Status == "source_unavailable" {
		return store.Result{Kind: "source_unavailable"}, fmt.Errorf("queued original sidecar is missing or differs from its captured bytes: %s", selected.Original)
	}
	if selected.Status != "pending" {
		return store.Result{Kind: "queue_conflict"}, nil
	}
	d := carrier.Parse(r.Proposal)
	if !d.Valid() || d.Record.Status != "proposed" || d.Record.Origin != "agent_proposal" || d.Record.OperatorConfirmed || selected.Kind != "unknown" && d.Record.Kind != selected.Kind {
		return store.Result{Kind: "invalid", Diagnostics: d.Diagnostics}, fmt.Errorf("repair must be a valid agent proposal of the queued kind, without confirmation")
	}
	view, err := s.Read(ctx)
	if err != nil {
		return store.Result{Kind: "unavailable"}, err
	}
	if view.Generation != r.ExpectedGeneration {
		return store.Result{Kind: "concurrent_write", Generation: view.Generation}, nil
	}
	if !sourceAvailable(view, selected.Original) {
		return store.Result{Kind: "source_unavailable"}, fmt.Errorf("queued original sidecar is missing or differs from its captured bytes: %s", selected.Original)
	}
	if len(d.Record.Supersedes) > 0 {
		return store.Result{Kind: "invalid"}, fmt.Errorf("bounded assistance creates a new proposal; acceptance/supersession needs its own current effect")
	}
	if ds := carrier.ValidateSuccessor(d.Record, view.Projection, view.Snapshots, nil); carrier.HasErrors(ds) {
		return store.Result{Kind: "invalid", Diagnostics: ds}, nil
	}
	for _, source := range d.Record.Sources {
		if source.SnapshotRef == "" {
			continue
		}
		blob, ok := view.Snapshots[source.SnapshotRef]
		if !ok {
			return store.Result{Kind: "invalid"}, fmt.Errorf("declared source snapshot is not persisted")
		}
		captured, err := carrier.ReadSourceSnapshot(blob, source.SnapshotRef)
		if err != nil {
			return store.Result{Kind: "invalid"}, err
		}
		declared, actual := source, captured.Provenance
		declared.SnapshotRef, actual.SnapshotRef = "", ""
		a, _ := json.Marshal(declared)
		b, _ := json.Marshal(actual)
		if string(a) != string(b) {
			return store.Result{Kind: "invalid"}, fmt.Errorf("source declaration differs from its exact snapshot provenance")
		}
	}
	if d.Record.Legacy == nil {
		d.Record.Legacy = carrier.Extra{}
	}
	d.Record.Legacy["migration_queue_id"] = selected.ID
	d.Record.Legacy["source_locator"] = selected.Source
	d.Record.Legacy["source_bytes"] = selected.Original
	d.Record.Legacy["repair_reason"] = r.Reason
	d.Record.WriteReceipt = &carrier.WriteReceipt{RequestID: r.RequestID, PayloadDigest: digest}
	raw, err := carrier.Encode(d.Record, d.Body)
	if err != nil {
		return store.Result{Kind: "invalid"}, err
	}
	basis, err := assistanceBasis(d.Record, view)
	if err != nil {
		return store.Result{Kind: "invalid"}, err
	}
	_, snapshot, edition, err := carrier.NewSnapshot(raw, basis)
	if err != nil {
		return store.Result{Kind: "invalid"}, err
	}
	ref := d.Record.ID + "@" + edition
	resolution, _ := json.Marshal(queueResolution{Format: "haft.migration-queue-resolution/1", QueueID: selected.ID, Source: selected.Source, ProposalRef: ref, Reason: r.Reason})
	resolution = append(resolution, '\n')
	outputs := []store.Output{{Path: kindDirs[d.Record.Kind] + "/" + d.Record.ID + ".md", Bytes: raw}, {Path: "editions/sha256/" + strings.TrimPrefix(edition, "sha256:") + ".json", Bytes: snapshot}, {Path: "migration/resolutions/" + selected.ID + ".json", Bytes: resolution}}
	return s.Publish(ctx, store.Request{RequestID: r.RequestID, PayloadDigest: digest, RequestPayload: payload, ExpectedGeneration: r.ExpectedGeneration, Outputs: outputs})
}

// LookupLegacy returns every preserved source position for an old ID. Multiple
// positions are explicit ambiguity, not a choice of the first matching record.
func LookupLegacy(ctx context.Context, root, id string) ([]Alias, error) {
	view, err := (store.Store{Root: root}).Read(ctx)
	if err != nil {
		return nil, err
	}
	return LookupLegacyInView(view, id)
}

// LookupLegacyInView resolves aliases from one caller-captured generation. It
// performs no filesystem reads and never chooses among multiple source positions.
func LookupLegacyInView(view store.View, id string) ([]Alias, error) {
	var aliases []Alias
	seen := map[string]bool{}
	for p, raw := range view.Files {
		if !strings.HasPrefix(p, "migration/reports/") {
			continue
		}
		var report Report
		if err := json.Unmarshal(raw, &report); err != nil {
			return nil, err
		}
		if report.Format != "haft.migration-report/1" || !carrier.ValidDigest(report.SourceDigest) || p != reportPath(report.SourceDigest) {
			return nil, fmt.Errorf("invalid migration report identity: %s", p)
		}
		for _, a := range report.Aliases {
			key := a.Source + "\x00" + a.NativeID + "\x00" + a.Original
			if a.LegacyID == id && !seen[key] {
				seen[key] = true
				aliases = append(aliases, a)
			}
		}
	}
	sort.Slice(aliases, func(i, j int) bool {
		if aliases[i].Source != aliases[j].Source {
			return aliases[i].Source < aliases[j].Source
		}
		if aliases[i].NativeID != aliases[j].NativeID {
			return aliases[i].NativeID < aliases[j].NativeID
		}
		return aliases[i].Original < aliases[j].Original
	})
	return aliases, nil
}

func sourceAvailable(view store.View, sourcePath string) bool {
	raw, exists := view.Files[sourcePath]
	base := filepath.Base(sourcePath)
	digest := strings.TrimSuffix(base, filepath.Ext(base))
	return exists && sourcePath == "migration/source/"+base && carrier.Digest(raw) == "sha256:"+digest
}

func assistanceBasis(record carrier.Record, view store.View) (carrier.InterpretationBasis, error) {
	basis := carrier.InterpretationBasis{}
	if raw, ok := view.Files["project.yaml"]; ok {
		node, ds := carrier.ParseYAML(raw)
		if carrier.HasErrors(ds) {
			return basis, fmt.Errorf("invalid staging project identity")
		}
		var config struct {
			Format       string `yaml:"format"`
			RepositoryID string `yaml:"repository_id"`
		}
		if err := node.Decode(&config); err != nil || config.Format != "haft.project/1" || config.RepositoryID == "" {
			return basis, fmt.Errorf("invalid staging project identity")
		}
		basis.Namespace = config.RepositoryID
	}
	needsTerms := len(record.Terms) > 0
	for _, claim := range record.Claims {
		needsTerms = needsTerms || len(claim.Terms) > 0
	}
	if needsTerms {
		raw, ok := view.Files["specs/terms.md"]
		if !ok {
			return basis, fmt.Errorf("declared term basis is unavailable")
		}
		parsed := carrier.ParseTerms(raw)
		ds := append(parsed.Diagnostics, carrier.ValidateTermRefs(record, parsed.Terms)...)
		if carrier.HasErrors(ds) {
			return basis, fmt.Errorf("invalid declared term basis: %v", ds)
		}
		basis.Terms = &carrier.BasisFile{Name: "specs/terms.md", Bytes: raw}
	}
	return basis, nil
}
