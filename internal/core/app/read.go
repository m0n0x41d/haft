package app

import (
	"context"
	"fmt"
	"runtime"
	"strings"

	"github.com/m0n0x41d/haft/internal/core/carrier"
	"github.com/m0n0x41d/haft/internal/core/code"
	"github.com/m0n0x41d/haft/internal/core/migrate"
	"github.com/m0n0x41d/haft/internal/core/source"
	"github.com/m0n0x41d/haft/internal/core/store"
)

func recall(q Request, r Result, v store.View) Result {
	if q.Action == "legacy" {
		matches, err := migrate.LookupLegacyInView(v, q.Ref)
		if err != nil {
			return failure(r, "legacy_report_invalid", err.Error())
		}
		r.Kind = "legacy_matches"
		if len(matches) == 0 {
			r.Kind = "absent"
		}
		r.Data = map[string]any{"matches": matches, "ambiguous": len(matches) > 1}
		r.Limits = append(r.Limits, "Legacy lookup preserves all exact source positions; a converted native record is not the historical evidence target edition")
		return r
	}
	if q.Ref == "" {
		r.Kind = "results"
		r.Data = v.Search(q.Query, q.Limit)
		return r
	}
	found := v.Resolve(q.Ref)
	r.Kind = found.Kind
	r.Diagnostics = append(r.Diagnostics, found.Diagnostics...)
	data := map[string]any{"resolution": found, "links": v.Links(q.Ref), "backlinks": v.Backlinks(q.Ref)}
	if found.Document != nil {
		ref, err := carrier.ParseRef(q.Ref)
		if err == nil {
			if !ref.Pinned() {
				ref.RecordID = found.Document.Record.ID
				ref.Alias = ""
				ref.Digest = found.Document.Edition
			}
			data["exact_ref"] = ref.String()
			data["evidence_uses"] = v.Projection.EvidenceUsesFor(ref.String())
			_, persisted := v.Snapshots[ref.Digest]
			data["snapshot_persisted"] = persisted
			if persisted {
				data["snapshot_bytes_base64"] = v.Snapshots[ref.Digest]
			}
		}
	}
	r.Data = data
	return r
}

func (s Service) codeIndex(q Request) (code.Index, code.CaptureResult, error) {
	cfg := code.Config{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, Toolchain: runtime.Version(), IncludeTests: true}
	if q.CodeConfig != nil {
		cfg = *q.CodeConfig
	}
	first, err := code.Capture(s.Root, code.CaptureConfig{IgnorePatterns: cfg.IgnorePatterns})
	if err != nil {
		return code.Index{}, first, err
	}
	i, err := first.Index(cfg)
	if err != nil {
		return i, first, err
	}
	// Capture twice and compare raw complete index bases, including dependencies.
	second, err := code.Capture(s.Root, code.CaptureConfig{IgnorePatterns: cfg.IgnorePatterns})
	if err != nil {
		return i, second, err
	}
	next, err := second.Index(cfg)
	if err != nil {
		return i, second, err
	}
	if i.Basis != next.Basis {
		return i, second, fmt.Errorf("code_changed_during_capture")
	}
	next.Complete = next.Complete && first.Complete && second.Complete
	return next, second, nil
}

func (s Service) context(q Request, r Result, v store.View) Result {
	i, capture, err := s.codeIndex(q)
	if err != nil {
		return unavailable(r, err)
	}
	r.Basis["code_basis"] = i.Basis
	r.Diagnostics = append(r.Diagnostics, capture.Diagnostics...)
	r.Diagnostics = append(r.Diagnostics, i.Diagnostics...)
	if !i.Complete {
		r.Coverage = "degraded"
	}
	data := map[string]any{"code_complete": i.Complete, "exclusions": capture.Exclusions}
	if q.CaptureCode {
		data["code_capture"] = CodeCapture{Format: "haft.code-capture/1", Files: capture.Files, Config: i.Config, Basis: i.Basis, Complete: i.Complete}
	}
	if q.PriorCode != nil {
		prior, err := code.IndexFromFiles(q.PriorCode.Files, q.PriorCode.Config)
		if err != nil {
			return failure(r, "invalid_code_capture", err.Error())
		}
		if q.PriorCode.Format != "haft.code-capture/1" || prior.Basis != q.PriorCode.Basis {
			return failure(r, "code_capture_digest_mismatch", "Prior capture does not reconstruct its declared raw basis")
		}
		prior.Complete = prior.Complete && q.PriorCode.Complete
		data["code_change"] = code.Compare(prior, i, q.Ref)
	}
	if q.Ref == "" {
		data["symbols"] = contextPage(i.Symbols, q)
		r.Kind = "results"
	} else if strings.HasPrefix(q.Ref, "sym:") || strings.HasPrefix(q.Ref, "file:") || strings.HasPrefix(q.Ref, "dir:") {
		related := i.Related(q.Ref)
		data["code_related"] = map[string]any{"resolution": related.Resolution, "basis": related.Basis, "complete": related.Complete, "dependency_files": contextPage(related.DependencyFiles, q), "affected_files": contextPage(related.AffectedFiles, q), "external_dependencies": contextPage(related.ExternalDependencies, q), "limits": related.Limits}
		if !related.Complete {
			r.Coverage = "degraded"
		}
		r.Kind = related.Resolution.Kind
	} else {
		memory := recall(q, r, v)
		data["memory"] = memory.Data
		r.Kind = memory.Kind
		r.Diagnostics = memory.Diagnostics
	}
	enrichContext(q, &r, data, v, i)
	r.Data = data
	r.Limits = append(r.Limits, "Syntactic Go declaration and package closure; an implementation edge does not prove claim satisfaction")
	return r
}

func (s Service) source(_ context.Context, q Request, r Result) Result {
	r.Limits = []string{"Source retrieval is advisory; inspect full governing bodies to assess applicability", "Offline captured source; upstream currentness unknown"}
	if q.Action == "inspect" && carrier.ValidDigest(q.Ref) {
		inspection := source.InspectSnapshot(q.Snapshots[q.Ref], q.Ref)
		r.Kind = inspection.Kind
		r.Data = inspection
		r.Diagnostics = inspection.Diagnostics
		if inspection.Kind == "found" {
			r.Coverage = "complete"
			r.Basis["source_snapshot"] = q.Ref
		}
		return r
	}
	reader, err := source.Capture(s.SourceRoot, s.SourceRepository, s.SourceLimits)
	if err != nil {
		return unavailable(r, err)
	}
	status := reader.Status()
	r.Coverage = "complete"
	if status.Kind != "available" {
		r.Coverage = "degraded"
	}
	r.Basis["source_tree"] = status.Revision.TreeDigest
	r.Diagnostics = append(r.Diagnostics, status.Diagnostics...)
	switch q.Action {
	case "status", "":
		r.Kind = status.Kind
		r.Data = status
	case "search":
		found := reader.Search(q.Query, q.Limit)
		r.Kind = found.Kind
		r.Data = found
	case "inspect":
		found := reader.Inspect(q.Ref)
		r.Kind = found.Kind
		data := map[string]any{"inspection": found}
		if found.Unit != nil {
			data["snapshot_bytes_base64"] = found.Unit.Snapshot
			r.Basis["source_snapshot"] = found.Unit.Source.SnapshotRef
		}
		r.Data = data
		r.Diagnostics = append(r.Diagnostics, found.Diagnostics...)
	default:
		return failure(r, "unsupported_action", "Source action is status, search or inspect")
	}
	return r
}
