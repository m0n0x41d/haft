package method

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/artifact"
)

const (
	CheckpointRecordOpen  = "checkpoint_open"
	CheckpointRecordClose = "checkpoint_close"

	CheckpointStatusOpen    = "open"
	CheckpointStatusClosed  = "closed"
	CheckpointStatusExpired = "expired"

	CheckpointAuthority = "method_checkpoint_attention_telemetry_not_evidence_truth_gate_passage_or_correctness_proof"
)

const DefaultCheckpointTTL = 2 * time.Hour

type CheckpointOpenInput struct {
	RunID        string
	TargetRef    string
	CheckRef     string
	TargetDigest string
	TTL          time.Duration
}

type CheckpointCloseInput struct {
	CloseToken      string
	Outcome         string
	ObservationRefs []string
	ResultingDigest string
	NextTargetRef   string
}

type CheckpointResult struct {
	Kind              string           `json:"kind"`
	SchemaVersion     int              `json:"schema_version"`
	Authority         string           `json:"authority"`
	AuthorityBoundary string           `json:"authority_boundary"`
	RunID             string           `json:"run_id"`
	CheckpointID      string           `json:"checkpoint_id"`
	CloseToken        string           `json:"close_token,omitempty"`
	Record            CheckpointRecord `json:"record"`
	Message           string           `json:"message"`
}

type CheckpointTraceReport struct {
	Kind              string                 `json:"kind"`
	SchemaVersion     int                    `json:"schema_version"`
	Authority         string                 `json:"authority"`
	AuthorityBoundary string                 `json:"authority_boundary"`
	RunID             string                 `json:"run_id"`
	Summary           CheckpointTraceSummary `json:"summary"`
	States            []CheckpointState      `json:"states,omitempty"`
	Records           []CheckpointRecord     `json:"records,omitempty"`
	Notes             []string               `json:"notes,omitempty"`
}

type CheckpointTraceSummary struct {
	Records int `json:"records"`
	Open    int `json:"open"`
	Closed  int `json:"closed"`
	Expired int `json:"expired"`
}

type CheckpointState struct {
	CheckpointID      string   `json:"checkpoint_id"`
	Status            string   `json:"status"`
	RunRef            string   `json:"run_ref"`
	TargetRef         string   `json:"target_ref,omitempty"`
	CheckRef          string   `json:"check_ref,omitempty"`
	TargetDigest      string   `json:"target_digest,omitempty"`
	Sequence          int      `json:"sequence,omitempty"`
	CloseToken        string   `json:"close_token,omitempty"`
	CloseTokenHash    string   `json:"close_token_hash,omitempty"`
	OpenedAt          string   `json:"opened_at,omitempty"`
	ExpiresAt         string   `json:"expires_at,omitempty"`
	Outcome           string   `json:"outcome,omitempty"`
	ObservationRefs   []string `json:"observation_refs,omitempty"`
	ResultingDigest   string   `json:"resulting_digest,omitempty"`
	NextTargetRef     string   `json:"next_target_ref,omitempty"`
	ClosedAt          string   `json:"closed_at,omitempty"`
	AuthorityBoundary string   `json:"authority_boundary,omitempty"`
}

func OpenCheckpoint(ctx context.Context, store artifact.ArtifactStore, haftDir string, input CheckpointOpenInput) (CheckpointResult, error) {
	input.RunID = strings.TrimSpace(input.RunID)
	input.TargetRef = strings.TrimSpace(input.TargetRef)
	input.CheckRef = strings.TrimSpace(input.CheckRef)
	input.TargetDigest = strings.TrimSpace(input.TargetDigest)
	if input.RunID == "" {
		return CheckpointResult{}, fmt.Errorf("run_id is required")
	}
	if input.TargetRef == "" {
		return CheckpointResult{}, fmt.Errorf("target_ref is required")
	}
	if input.CheckRef == "" {
		return CheckpointResult{}, fmt.Errorf("check_ref is required")
	}
	if input.TargetDigest == "" {
		return CheckpointResult{}, fmt.Errorf("target_digest is required")
	}
	ttl := input.TTL
	if ttl == 0 {
		ttl = DefaultCheckpointTTL
	}
	if ttl < time.Minute {
		return CheckpointResult{}, fmt.Errorf("checkpoint ttl must be at least 1 minute")
	}

	runArtifact, run, err := loadRunArtifact(ctx, store, input.RunID)
	if err != nil {
		return CheckpointResult{}, err
	}
	if run.Status != "open" {
		return CheckpointResult{}, fmt.Errorf("method run %s is %s; checkpoints can only open on open runs", input.RunID, run.Status)
	}

	now := time.Now().UTC()
	token, err := newCheckpointToken()
	if err != nil {
		return CheckpointResult{}, err
	}
	sequence := nextCheckpointSequence(run.Checkpoints)
	record := CheckpointRecord{
		RecordKind:        CheckpointRecordOpen,
		CheckpointID:      fmt.Sprintf("chk-%03d", sequence),
		RunRef:            run.ID,
		TargetRef:         input.TargetRef,
		CheckRef:          input.CheckRef,
		TargetDigest:      input.TargetDigest,
		Sequence:          sequence,
		CloseTokenHash:    checkpointTokenHash(token),
		OpenedAt:          nowRFC3339(now),
		ExpiresAt:         nowRFC3339(now.Add(ttl)),
		AuthorityBoundary: CheckpointAuthority,
	}
	run.Checkpoints = append(run.Checkpoints, record)
	if err := persistRun(ctx, store, haftDir, runArtifact, run); err != nil {
		return CheckpointResult{}, err
	}

	return CheckpointResult{
		Kind:              "method_checkpoint_open",
		SchemaVersion:     1,
		Authority:         CheckpointAuthority,
		AuthorityBoundary: CheckpointAuthority,
		RunID:             run.ID,
		CheckpointID:      record.CheckpointID,
		CloseToken:        token,
		Record:            record,
		Message:           "checkpoint opened; close with token before expiry",
	}, nil
}

func CloseCheckpoint(ctx context.Context, store artifact.ArtifactStore, haftDir string, input CheckpointCloseInput) (CheckpointResult, error) {
	input.CloseToken = strings.TrimSpace(input.CloseToken)
	input.Outcome = strings.TrimSpace(input.Outcome)
	input.ResultingDigest = strings.TrimSpace(input.ResultingDigest)
	input.NextTargetRef = strings.TrimSpace(input.NextTargetRef)
	input.ObservationRefs = compactCheckpointStrings(input.ObservationRefs)
	if input.CloseToken == "" {
		return CheckpointResult{}, fmt.Errorf("close_token is required")
	}
	if input.Outcome == "" {
		return CheckpointResult{}, fmt.Errorf("outcome is required")
	}

	runArtifact, run, opened, err := findOpenCheckpointByToken(ctx, store, input.CloseToken)
	if err != nil {
		return CheckpointResult{}, err
	}
	if run.Status != "open" {
		return CheckpointResult{}, fmt.Errorf("method run %s is %s; checkpoints can only close while the run is open", run.ID, run.Status)
	}
	now := time.Now().UTC()
	expiresAt, err := time.Parse(time.RFC3339, opened.ExpiresAt)
	if err != nil {
		return CheckpointResult{}, fmt.Errorf("checkpoint %s has invalid expires_at %q", opened.CheckpointID, opened.ExpiresAt)
	}
	if now.After(expiresAt) {
		return CheckpointResult{}, fmt.Errorf("checkpoint %s close token expired at %s", opened.CheckpointID, opened.ExpiresAt)
	}

	record := CheckpointRecord{
		RecordKind:        CheckpointRecordClose,
		CheckpointID:      opened.CheckpointID,
		RunRef:            run.ID,
		CloseTokenHash:    checkpointTokenHash(input.CloseToken),
		Outcome:           input.Outcome,
		ObservationRefs:   input.ObservationRefs,
		ResultingDigest:   input.ResultingDigest,
		NextTargetRef:     input.NextTargetRef,
		ClosedAt:          nowRFC3339(now),
		AuthorityBoundary: CheckpointAuthority,
	}
	run.Checkpoints = append(run.Checkpoints, record)
	if err := persistRun(ctx, store, haftDir, runArtifact, run); err != nil {
		return CheckpointResult{}, err
	}

	return CheckpointResult{
		Kind:              "method_checkpoint_close",
		SchemaVersion:     1,
		Authority:         CheckpointAuthority,
		AuthorityBoundary: CheckpointAuthority,
		RunID:             run.ID,
		CheckpointID:      record.CheckpointID,
		Record:            record,
		Message:           "checkpoint closed; record remains attention telemetry, not correctness evidence",
	}, nil
}

func TraceCheckpoints(ctx context.Context, store artifact.ArtifactStore, runID string) (CheckpointTraceReport, error) {
	_, run, err := loadRunArtifact(ctx, store, strings.TrimSpace(runID))
	if err != nil {
		return CheckpointTraceReport{}, err
	}
	states := checkpointStates(run.Checkpoints, time.Now().UTC())
	return CheckpointTraceReport{
		Kind:              "method_checkpoint_trace",
		SchemaVersion:     1,
		Authority:         CheckpointAuthority,
		AuthorityBoundary: CheckpointAuthority,
		RunID:             run.ID,
		Summary:           summarizeCheckpointTrace(run.Checkpoints, states),
		States:            states,
		Records:           checkpointTraceRecords(run.Checkpoints),
		Notes: []string{
			"Checkpoint records are attention telemetry, not evidence truth, gate passage, or correctness proof.",
			"Open and close records are append-only within the MethodRun structured data.",
		},
	}, nil
}

func loadRunArtifact(ctx context.Context, store artifact.ArtifactStore, runID string) (*artifact.Artifact, MethodRun, error) {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil, MethodRun{}, fmt.Errorf("run_id is required")
	}
	runArtifact, err := store.Get(ctx, runID)
	if err != nil {
		return nil, MethodRun{}, fmt.Errorf("method run %s not found: %w", runID, err)
	}
	if runArtifact.Meta.Kind != artifact.KindMethodRun {
		return nil, MethodRun{}, fmt.Errorf("%s is %s, not MethodRun", runID, runArtifact.Meta.Kind)
	}
	run, err := DecodeRun(runArtifact)
	if err != nil {
		return nil, MethodRun{}, err
	}
	return runArtifact, run, nil
}

func persistRun(ctx context.Context, store artifact.ArtifactStore, haftDir string, runArtifact *artifact.Artifact, run MethodRun) error {
	encoded, err := encodeRunForUpdate(runArtifact.StructuredData, run)
	if err != nil {
		return fmt.Errorf("encode method run: %w", err)
	}
	runArtifact.Body = RenderRunBody(run)
	runArtifact.StructuredData = string(encoded)
	runArtifact.SearchKeywords = methodRunSearchKeywords(run)
	if err := store.Update(ctx, runArtifact); err != nil {
		return fmt.Errorf("update method run: %w", err)
	}
	if _, err := artifact.WriteFile(haftDir, runArtifact); err != nil {
		return fmt.Errorf("file write (DB saved OK): %w", err)
	}
	return nil
}

func findOpenCheckpointByToken(ctx context.Context, store artifact.ArtifactStore, token string) (*artifact.Artifact, MethodRun, CheckpointRecord, error) {
	artifacts, err := store.ListByKind(ctx, artifact.KindMethodRun, 0)
	if err != nil {
		return nil, MethodRun{}, CheckpointRecord{}, fmt.Errorf("list method runs: %w", err)
	}
	for _, item := range artifacts {
		runArtifact, run, err := loadRunArtifact(ctx, store, item.Meta.ID)
		if err != nil {
			continue
		}
		opened, ok := openCheckpointByToken(run.Checkpoints, token)
		if !ok {
			continue
		}
		if checkpointHasClose(run.Checkpoints, opened.CheckpointID) {
			return nil, MethodRun{}, CheckpointRecord{}, fmt.Errorf("checkpoint %s is already closed", opened.CheckpointID)
		}
		return runArtifact, run, opened, nil
	}
	return nil, MethodRun{}, CheckpointRecord{}, fmt.Errorf("checkpoint close token not found")
}

func openCheckpointByToken(records []CheckpointRecord, token string) (CheckpointRecord, bool) {
	tokenHash := checkpointTokenHash(token)
	for _, record := range records {
		if record.RecordKind != CheckpointRecordOpen {
			continue
		}
		if checkpointTokenMatches(record, token, tokenHash) {
			return record, true
		}
	}
	return CheckpointRecord{}, false
}

func checkpointTokenMatches(record CheckpointRecord, token string, tokenHash string) bool {
	if strings.TrimSpace(record.CloseTokenHash) != "" {
		return strings.TrimSpace(record.CloseTokenHash) == tokenHash
	}
	return strings.TrimSpace(record.CloseToken) == token
}

func checkpointHasClose(records []CheckpointRecord, checkpointID string) bool {
	for _, record := range records {
		if record.RecordKind == CheckpointRecordClose && record.CheckpointID == checkpointID {
			return true
		}
	}
	return false
}

func checkpointStates(records []CheckpointRecord, now time.Time) []CheckpointState {
	closes := map[string]CheckpointRecord{}
	for _, record := range records {
		if record.RecordKind == CheckpointRecordClose {
			closes[record.CheckpointID] = record
		}
	}
	var states []CheckpointState
	for _, record := range records {
		if record.RecordKind != CheckpointRecordOpen {
			continue
		}
		state := CheckpointState{
			CheckpointID:      record.CheckpointID,
			Status:            CheckpointStatusOpen,
			RunRef:            record.RunRef,
			TargetRef:         record.TargetRef,
			CheckRef:          record.CheckRef,
			TargetDigest:      record.TargetDigest,
			Sequence:          record.Sequence,
			CloseTokenHash:    record.CloseTokenHash,
			OpenedAt:          record.OpenedAt,
			ExpiresAt:         record.ExpiresAt,
			AuthorityBoundary: record.AuthorityBoundary,
		}
		if closeRecord, ok := closes[record.CheckpointID]; ok {
			state.Status = CheckpointStatusClosed
			state.Outcome = closeRecord.Outcome
			state.ObservationRefs = append([]string(nil), closeRecord.ObservationRefs...)
			state.ResultingDigest = closeRecord.ResultingDigest
			state.NextTargetRef = closeRecord.NextTargetRef
			state.ClosedAt = closeRecord.ClosedAt
			state.AuthorityBoundary = closeRecord.AuthorityBoundary
		} else if expiresAt, err := time.Parse(time.RFC3339, record.ExpiresAt); err == nil && now.After(expiresAt) {
			state.Status = CheckpointStatusExpired
		}
		states = append(states, state)
	}
	return states
}

func summarizeCheckpointTrace(records []CheckpointRecord, states []CheckpointState) CheckpointTraceSummary {
	summary := CheckpointTraceSummary{Records: len(records)}
	for _, state := range states {
		switch state.Status {
		case CheckpointStatusOpen:
			summary.Open++
		case CheckpointStatusClosed:
			summary.Closed++
		case CheckpointStatusExpired:
			summary.Expired++
		}
	}
	return summary
}

func checkpointTraceRecords(records []CheckpointRecord) []CheckpointRecord {
	traceRecords := make([]CheckpointRecord, 0, len(records))
	for _, record := range records {
		record.CloseToken = ""
		traceRecords = append(traceRecords, record)
	}
	return traceRecords
}

func nextCheckpointSequence(records []CheckpointRecord) int {
	maxSequence := 0
	for _, record := range records {
		if record.RecordKind != CheckpointRecordOpen {
			continue
		}
		if record.Sequence > maxSequence {
			maxSequence = record.Sequence
		}
	}
	return maxSequence + 1
}

func newCheckpointToken() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate checkpoint close token: %w", err)
	}
	return "mchk-" + hex.EncodeToString(raw[:]), nil
}

func checkpointTokenHash(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func compactCheckpointStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		out = append(out, value)
	}
	return out
}
