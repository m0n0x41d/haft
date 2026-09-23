// Package migrate implements bounded read-only v9 capture and explicit staging.
// It never activates a project or imports the legacy semantic runtime.
package migrate

import (
	"context"

	"github.com/m0n0x41d/haft/internal/core/carrier"
	"github.com/m0n0x41d/haft/internal/core/store"
)

type Request struct {
	DatabasePath string `json:"database_path,omitempty"`
	CarrierRoot  string `json:"carrier_root,omitempty"` // explicit legacy .haft directory
	OutputRoot   string `json:"output_root"`            // staging project root, outside sources
	RequestID    string `json:"request_id,omitempty"`
	CreatedAt    string `json:"created_at"`
	DryRun       bool   `json:"dry_run"` // staging/report only; false is rejected in B1
	MaxRows      int    `json:"max_rows,omitempty"`
	MaxBytes     int64  `json:"max_bytes,omitempty"`
}
type Result struct {
	Kind        string       `json:"kind"`
	Report      Report       `json:"report"`
	ReportPath  string       `json:"report_path,omitempty"`
	Publication store.Result `json:"publication"`
}
type Value struct {
	StorageClass string `json:"storage_class"`
	Bytes        []byte `json:"bytes_base64"`
}
type Row struct {
	Table   string           `json:"table"`
	Key     string           `json:"key"`
	Columns map[string]Value `json:"columns"`
}
type Snapshot struct {
	DatabasePath  string               `json:"database_path,omitempty"`
	DatabaseFiles map[string]string    `json:"database_files,omitempty"`
	CarrierRoot   string               `json:"carrier_root,omitempty"`
	Carriers      map[string][]byte    `json:"carriers"`
	Rows          []Row                `json:"rows"`
	Scopes        []Scope              `json:"scopes"`
	Digest        string               `json:"digest"`
	Diagnostics   []carrier.Diagnostic `json:"diagnostics,omitempty"`
}
type Scope struct {
	Source      string `json:"source"`
	Disposition string `json:"disposition"`
	Count       *int   `json:"count,omitempty"`
	Reason      string `json:"reason,omitempty"`
}
type Item struct {
	Source      string   `json:"source"`
	LegacyID    string   `json:"legacy_id,omitempty"`
	Kind        string   `json:"kind,omitempty"`
	Disposition string   `json:"disposition"`
	Destination string   `json:"destination,omitempty"`
	NativeID    string   `json:"native_id,omitempty"`
	Original    string   `json:"original,omitempty"` // preserved raw sidecar, otherwise source locator
	Reasons     []string `json:"reasons,omitempty"`
	Losses      []Loss   `json:"losses,omitempty"`
}
type Loss struct {
	Kind        string `json:"kind"` // weakened, field_loss, record_loss, existing_unresolved
	Field       string `json:"field,omitempty"`
	Original    any    `json:"original,omitempty"`
	Reason      string `json:"reason"`
	AffectedUse string `json:"affected_use"`
}
type Alias struct {
	LegacyID    string `json:"legacy_id"`
	Source      string `json:"source"`
	NativeID    string `json:"native_id,omitempty"`
	Original    string `json:"original"`
	Disposition string `json:"disposition"`
}
type QueueItem struct {
	ID          string `json:"id"`
	Source      string `json:"source"`
	Original    string `json:"original"`
	Kind        string `json:"kind"`
	Reason      string `json:"reason"`
	Priority    string `json:"priority"`
	Status      string `json:"status"`
	ProposalRef string `json:"proposal_ref,omitempty"`
}
type Report struct {
	Format        string               `json:"format"`
	SourceDigest  string               `json:"source_digest"`
	DatabasePath  string               `json:"database_path,omitempty"`
	DatabaseFiles map[string]string    `json:"database_files,omitempty"`
	CarrierRoot   string               `json:"carrier_root,omitempty"`
	OutputRoot    string               `json:"output_root"`
	CreatedAt     string               `json:"created_at"`
	Scopes        []Scope              `json:"scopes"`
	Items         []Item               `json:"items"`
	Aliases       []Alias              `json:"aliases"`
	Queue         []QueueItem          `json:"queue"`
	Excluded      map[string][]string  `json:"excluded"`
	Counts        map[string]int       `json:"counts"`
	Diagnostics   []carrier.Diagnostic `json:"diagnostics,omitempty"`
	Limits        []string             `json:"limits"`
}
type Plan struct {
	Report  Report         `json:"report"`
	Outputs []store.Output `json:"outputs"`
}

func Run(ctx context.Context, r Request) (Result, error)       { return run(ctx, r) }
func Capture(ctx context.Context, r Request) (Snapshot, error) { return capture(ctx, r) }
func Build(snapshot Snapshot, r Request) Plan                  { return build(snapshot, r) }

type AssistanceRequest struct {
	OutputRoot         string `json:"output_root"`
	QueueID            string `json:"queue_id"`
	Proposal           []byte `json:"proposal"`
	Reason             string `json:"reason"`
	RequestID          string `json:"request_id"`
	ExpectedGeneration string `json:"expected_generation"`
}

// Assist records an agent-authored proposed repair with source provenance and an
// immutable queue resolution. It cannot accept a decision/spec or alter sources.
func Assist(ctx context.Context, r AssistanceRequest) (store.Result, error) { return assist(ctx, r) }
func Queue(ctx context.Context, outputRoot string) ([]QueueItem, error) {
	return queue(ctx, outputRoot)
}
