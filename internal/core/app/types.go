// Package app is the shared v10 application boundary for CLI and MCP clients.
// Each request captures fresh state; client lifetime does not pin mutable files.
package app

import (
	"context"
	"encoding/json"
	"time"

	"github.com/m0n0x41d/haft/internal/core/carrier"
	"github.com/m0n0x41d/haft/internal/core/change"
	"github.com/m0n0x41d/haft/internal/core/check"
	"github.com/m0n0x41d/haft/internal/core/code"
	"github.com/m0n0x41d/haft/internal/core/source"
	"github.com/m0n0x41d/haft/internal/core/store"
)

const Format = "haft.api/1"

// Request is one transport-independent contract. Carrier is Markdown text;
// snapshot and runner bytes use JSON base64 through Go's []byte encoding.
type Request struct {
	Format             string                     `json:"format"`
	Operation          string                     `json:"operation"`
	Action             string                     `json:"action,omitempty"`
	RequestID          string                     `json:"request_id,omitempty"`
	Carrier            string                     `json:"carrier,omitempty"`
	Ref                string                     `json:"ref,omitempty"`
	Query              string                     `json:"query,omitempty"`
	Limit              int                        `json:"limit,omitempty"`
	Offset             int                        `json:"offset,omitempty"`
	ExpectedGeneration string                     `json:"expected_generation,omitempty"`
	ExpectedHeads      []string                   `json:"expected_heads,omitempty"`
	ExpectedTargets    map[string]string          `json:"expected_target_digests,omitempty"`
	Snapshots          map[string][]byte          `json:"snapshots,omitempty"`
	PreviewDigest      string                     `json:"preview_digest,omitempty"`
	Metadata           map[string]change.Metadata `json:"metadata,omitempty"`
	Revision           *change.Revision           `json:"revision,omitempty"`
	Strict             bool                       `json:"strict,omitempty"`
	CodeConfig         *code.Config               `json:"code_config,omitempty"`
	CheckRef           string                     `json:"check_ref,omitempty"`
	Scope              string                     `json:"scope,omitempty"`
	FailureContract    string                     `json:"failure_contract,omitempty"`
	FailurePattern     string                     `json:"failure_pattern,omitempty"`
	Seed               *int64                     `json:"seed,omitempty"`
	Observation        *check.ObservationInput    `json:"observation,omitempty"`
	CaptureCode        bool                       `json:"capture_code,omitempty"`
	PriorCode          *CodeCapture               `json:"prior_code,omitempty"`
	View               string                     `json:"view,omitempty"`
	Part               string                     `json:"part,omitempty"`
	Cursor             string                     `json:"cursor,omitempty"`
	ExpectedDigest     string                     `json:"expected_digest,omitempty"`
	Retain             []Retention                `json:"retain,omitempty"`
	forDelivery        bool
	payload            []byte
}

type Retention struct {
	Ref  string `json:"ref"`
	Part string `json:"part"`
}

// CodeCapture is an optional portable input for advisory code comparisons.
// Its raw basis is reconstructed and verified before using cached syntax fields.
type CodeCapture struct {
	Format   string            `json:"format"`
	Files    map[string][]byte `json:"files"`
	Config   code.Config       `json:"config"`
	Basis    string            `json:"basis"`
	Complete bool              `json:"complete"`
}

type Result struct {
	Format      string               `json:"format"`
	Operation   string               `json:"operation"`
	Kind        string               `json:"result_kind"`
	Data        any                  `json:"data"`
	Diagnostics []carrier.Diagnostic `json:"diagnostics"`
	Basis       map[string]string    `json:"basis"`
	Coverage    string               `json:"coverage"`
	Limits      []string             `json:"limits"`
}

type Service struct {
	Root             string
	SourceRoot       string
	SourceRepository string
	SourceLimits     source.Limits
	Store            *store.Store // optional fault/budget instrumentation
	Now              func() time.Time
	NewID            func(kind string) (string, error)
}

func (s Service) memory() store.Store {
	if s.Store != nil {
		return *s.Store
	}
	return store.Store{Root: s.Root}
}

func (s Service) Execute(ctx context.Context, q Request) Result {
	r := Result{Format: Format, Operation: q.Operation, Kind: "invalid", Data: nil,
		Diagnostics: []carrier.Diagnostic{}, Basis: map[string]string{}, Coverage: "unavailable",
		Limits: []string{"Local project scope; structural validity and retrieval do not establish truth, applicability or operator authority"}}
	if q.Format != Format {
		return failure(r, "unsupported_format", "Request requires haft.api/1")
	}
	if q.Limit < 0 || q.Limit > 500 {
		return failure(r, "invalid_limit", "Limit must be between zero and 500")
	}
	if q.Offset < 0 || q.Offset > 100000 {
		return failure(r, "invalid_offset", "Offset must be between zero and 100000")
	}
	if q.Operation == "source" || q.Operation == "fpf" {
		return s.source(ctx, q, r)
	}
	switch q.Operation {
	case "remember", "recall", "context", "impact", "check", "change", "recover":
	default:
		return failure(r, "unsupported_operation", "Unknown application operation")
	}
	payload, err := json.Marshal(q)
	if err != nil {
		return failure(r, "invalid_request", err.Error())
	}
	if q.payload != nil {
		payload = q.payload
	}
	if q.Operation == "remember" || q.Operation == "change" && q.Action != "preview" && q.Action != "list" && q.Action != "show" {
		prior, found, err := s.memory().Replay(ctx, q.RequestID, carrier.Digest(payload))
		if err != nil {
			return unavailable(r, err)
		}
		if found {
			return published(r, prior)
		}
	}
	if q.Operation == "recover" {
		v, err := s.memory().Recover(ctx)
		if err != nil {
			return unavailable(r, err)
		}
		return published(r, v)
	}
	v, err := s.memory().Read(ctx)
	if err != nil {
		return unavailable(r, err)
	}
	r.Basis["memory_generation"] = v.Generation
	r.Coverage = v.Coverage
	r.Diagnostics = append(r.Diagnostics, v.Diagnostics...)
	if v.Coverage == "unavailable" {
		r.Kind = "unavailable"
		return r
	}
	switch q.Operation {
	case "remember":
		return s.remember(ctx, q, r, v, payload)
	case "recall":
		return recall(q, r, v)
	case "context", "impact":
		return s.context(q, r, v)
	case "check":
		return s.check(q, r, v)
	case "change":
		return s.change(ctx, q, r, v, payload)
	}
	return r
}

func failure(r Result, code, message string) Result {
	r.Kind = "invalid"
	r.Diagnostics = append(r.Diagnostics, carrier.Diagnostic{Code: code, Message: message, Severity: "error"})
	return r
}
func conflict(r Result, code, message string) Result {
	r = failure(r, code, message)
	r.Kind = "conflict"
	return r
}
func unavailable(r Result, err error) Result {
	r = failure(r, "unavailable", err.Error())
	r.Kind = "unavailable"
	r.Coverage = "unavailable"
	return r
}
func published(r Result, v store.Result) Result {
	r.Kind = v.Kind
	r.Data = v
	r.Diagnostics = append(r.Diagnostics, v.Diagnostics...)
	if v.Generation != "" {
		r.Basis["memory_generation"] = v.Generation
	}
	if (v.Kind == "written" || v.Kind == "replayed" || v.Kind == "recovered") && r.Coverage == "unavailable" {
		r.Coverage = "publication_only"
	}
	return r
}

// Failed is shared transport signalling. Structured data remains available even
// when a write conflicts or a runner observation cannot establish a pass.
func (r Result) Failed() bool {
	for _, d := range r.Diagnostics {
		if d.Severity == "error" {
			return true
		}
	}
	switch r.Kind {
	case "invalid", "unavailable", "conflict", "request_conflict", "replay_conflict", "concurrent_write", "path_conflict", "recovery_conflict", "queue_conflict", "interrupted", "publication_pending", "commit_outcome_unknown", "unsupported", "assertion_failure", "environment_failure", "unattributable", "not_run", "skipped":
		return true
	}
	return false
}
