// Package store owns captured filesystem reads and recoverable publication. It
// does not choose claims, accept decisions, or interpret operator authority.
package store

import (
	"context"
	"time"

	"github.com/m0n0x41d/haft/internal/core/carrier"
)

type Store struct {
	Root          string
	LockTimeout   time.Duration
	ScanAttempts  int
	MaxFileBytes  int64
	MaxTotalBytes int64
	// Fault is optional instrumentation for controlled interruption tests. It is
	// invoked while a shared or exclusive project lock is held; ordinary callers
	// leave it nil. between_scans is a read-observation hook, not a write effect.
	Fault func(Point) error
}
type Point struct {
	Stage string `json:"stage"` // between_scans, staged, published, before_commit, committed
	Path  string `json:"path,omitempty"`
	Index int    `json:"index,omitempty"`
}
type Output struct {
	Path  string `json:"path"` // relative to .haft, never a project-external path
	Bytes []byte `json:"bytes"`
}
type Request struct {
	RequestID          string   `json:"request_id,omitempty"`
	PayloadDigest      string   `json:"payload_digest"`
	RequestPayload     []byte   `json:"request_payload"`
	ExpectedGeneration string   `json:"expected_generation"`
	InputRefs          []string `json:"input_refs,omitempty"`
	Outputs            []Output `json:"outputs"`
}
type Result struct {
	Kind          string               `json:"kind"`
	Generation    string               `json:"generation,omitempty"`
	Paths         []string             `json:"paths,omitempty"`
	TransactionID string               `json:"transaction_id,omitempty"`
	Diagnostics   []carrier.Diagnostic `json:"diagnostics,omitempty"`
}
type View struct {
	Files            map[string][]byte    `json:"files"`
	Documents        []carrier.Document   `json:"documents"`
	DocumentPaths    []string             `json:"document_paths"`
	Snapshots        map[string][]byte    `json:"snapshots"`         // persisted and digest verified
	CurrentSnapshots map[string][]byte    `json:"current_snapshots"` // computed, not persisted
	Editions         map[string]string    `json:"editions"`          // carrier/change path to computed digest
	Generation       string               `json:"generation"`
	Coverage         string               `json:"coverage"` // complete, degraded, unavailable
	Diagnostics      []carrier.Diagnostic `json:"diagnostics"`
	Projection       carrier.Projection   `json:"projection"`
}

func (v View) AllSnapshots() map[string][]byte {
	m := map[string][]byte{}
	for k, b := range v.CurrentSnapshots {
		m[k] = b
	}
	for k, b := range v.Snapshots {
		m[k] = b
	}
	return m
}
func (s Store) Read(ctx context.Context) (View, error)                 { return s.read(ctx) }
func (s Store) Publish(ctx context.Context, r Request) (Result, error) { return s.publish(ctx, r) }
func (s Store) Recover(ctx context.Context) (Result, error)            { return s.recover(ctx) }

// Replay checks a request before the application repeats semantic preconditions
// that its successful first publication may already have changed. It verifies
// journal outputs and may recover an exact interrupted request under the lock.
func (s Store) Replay(ctx context.Context, requestID, payloadDigest string) (Result, bool, error) {
	if requestID == "" {
		return Result{}, false, nil
	}
	if !carrier.ValidDigest(payloadDigest) {
		return Result{Kind: "invalid", Diagnostics: []carrier.Diagnostic{diag("invalid_payload_digest", "payload_digest", "Replay needs a full original payload digest")}}, true, nil
	}
	h, err := s.open(ctx, true)
	if err != nil {
		return Result{Kind: "unavailable"}, true, err
	}
	defer h.close()
	js, _, err := s.readJournals(h.root)
	if err != nil {
		return Result{Kind: "recovery_conflict", Diagnostics: []carrier.Diagnostic{diag("invalid_journal", "transactions", err.Error())}}, true, nil
	}
	for _, j := range js {
		if j.Manifest.RequestID != requestID {
			continue
		}
		if j.Manifest.PayloadDigest != payloadDigest {
			return Result{Kind: "request_conflict", TransactionID: j.Manifest.TransactionID}, true, nil
		}
		if !j.Committed {
			result, err := s.complete(ctx, h, j, true)
			if err != nil || result.Kind != "recovered" {
				return result, true, err
			}
		}
		result, err := s.replay(h, j)
		return result, true, err
	}
	return Result{}, false, nil
}

type manifest struct {
	Format             string            `json:"format"`
	TransactionID      string            `json:"transaction_id"`
	RequestID          string            `json:"request_id,omitempty"`
	PayloadDigest      string            `json:"payload_digest"`
	ExpectedGeneration string            `json:"expected_generation"`
	InputRefs          []string          `json:"input_refs,omitempty"`
	Inputs             map[string]string `json:"inputs"`
	Outputs            []manifestOutput  `json:"outputs"`
}
type manifestOutput struct {
	Path        string `json:"path"`
	Digest      string `json:"digest"`
	StagePath   string `json:"stage_path"`
	Preexisting bool   `json:"preexisting,omitempty"`
}
type commitMarker struct {
	Format         string `json:"format"`
	ManifestDigest string `json:"manifest_digest"`
}
type journal struct {
	Manifest  manifest
	Raw       []byte
	Committed bool
}

func diag(code, path, message string) carrier.Diagnostic {
	return carrier.Diagnostic{Code: code, Path: path, Message: message, Severity: "warning"}
}
