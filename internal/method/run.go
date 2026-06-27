package method

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/artifact"
)

func CreateRun(ctx context.Context, store artifact.ArtifactStore, haftDir string, input PullInput) (*artifact.Artifact, string, MethodRun, error) {
	run, err := Pull(input)
	if err != nil {
		return nil, "", MethodRun{}, err
	}
	now := time.Now().UTC()
	run.ID = artifact.GenerateIDWithTaskContext(artifact.KindMethodRun, 0, input.Task)
	run.OpenedAt = nowRFC3339(now)

	encoded, err := json.Marshal(run)
	if err != nil {
		return nil, "", MethodRun{}, fmt.Errorf("encode method run: %w", err)
	}

	a := &artifact.Artifact{
		Meta: artifact.Meta{
			ID:        run.ID,
			Kind:      artifact.KindMethodRun,
			Title:     methodRunTitle(run),
			Status:    artifact.StatusActive,
			Context:   input.Context,
			Mode:      artifact.ModeTactical,
			CreatedAt: now,
			UpdatedAt: now,
			Links:     methodRunLinks(input),
		},
		Body:           RenderRunBody(run),
		StructuredData: string(encoded),
		SearchKeywords: methodRunSearchKeywords(run),
	}

	if err := store.Create(ctx, a); err != nil {
		return nil, "", MethodRun{}, fmt.Errorf("store method run: %w", err)
	}
	filePath, err := artifact.WriteFile(haftDir, a)
	if err != nil {
		return a, "", run, fmt.Errorf("file write (DB saved OK): %w", err)
	}
	return a, filePath, run, nil
}

func CloseRun(ctx context.Context, store artifact.ArtifactStore, haftDir string, input CloseInput) (*artifact.Artifact, string, MethodRun, error) {
	if strings.TrimSpace(input.PullID) == "" {
		return nil, "", MethodRun{}, fmt.Errorf("pull_id is required")
	}

	a, err := store.Get(ctx, input.PullID)
	if err != nil {
		return nil, "", MethodRun{}, fmt.Errorf("method run %s not found: %w", input.PullID, err)
	}
	if a.Meta.Kind != artifact.KindMethodRun {
		return nil, "", MethodRun{}, fmt.Errorf("%s is %s, not MethodRun", input.PullID, a.Meta.Kind)
	}

	run, err := DecodeRun(a)
	if err != nil {
		return nil, "", MethodRun{}, err
	}
	if run.Status == "closed" {
		return nil, "", MethodRun{}, fmt.Errorf("method run %s is already closed", input.PullID)
	}
	if err := ValidateClose(run, input); err != nil {
		return nil, "", MethodRun{}, err
	}
	if err := ValidateProblemClosureHygiene(ctx, store, a, run, input); err != nil {
		return nil, "", MethodRun{}, err
	}

	now := time.Now().UTC()
	closeout := Closeout{
		ChangedFiles: input.ChangedFiles,
		GateResults:  input.GateResults,
		Verification: input.Verification,
		Waivers:      input.Waivers,
		ClosedAt:     nowRFC3339(now),
	}
	closeout.CarryThrough = closeoutCarryThroughItems(input.CarryThrough, closeout.ClosedAt)
	run.Status = "closed"
	run.ClosedAt = closeout.ClosedAt
	run.Closeout = &closeout

	encoded, err := encodeRunForUpdate(a.StructuredData, run)
	if err != nil {
		return nil, "", MethodRun{}, fmt.Errorf("encode method run: %w", err)
	}
	a.Meta.Status = artifact.StatusAddressed
	a.Body = RenderRunBody(run)
	a.StructuredData = string(encoded)
	a.SearchKeywords = methodRunSearchKeywords(run)

	if err := store.Update(ctx, a); err != nil {
		return nil, "", MethodRun{}, fmt.Errorf("update method run: %w", err)
	}
	filePath, err := artifact.WriteFile(haftDir, a)
	if err != nil {
		return a, "", run, fmt.Errorf("file write (DB saved OK): %w", err)
	}
	return a, filePath, run, nil
}

func DecodeRun(a *artifact.Artifact) (MethodRun, error) {
	if a == nil {
		return MethodRun{}, fmt.Errorf("method run artifact is nil")
	}
	var run MethodRun
	if strings.TrimSpace(a.StructuredData) == "" {
		return MethodRun{}, fmt.Errorf("method run %s has no structured data", a.Meta.ID)
	}
	if err := json.Unmarshal([]byte(a.StructuredData), &run); err != nil {
		return MethodRun{}, fmt.Errorf("decode method run %s: %w", a.Meta.ID, err)
	}
	return run, nil
}

var methodRunStructuredKeys = []string{
	"id",
	"catalog_id",
	"catalog_version",
	"status",
	"task_signature",
	"deterministic_context",
	"methods",
	"carry_through",
	"checkpoints",
	"opened_at",
	"closed_at",
	"closeout",
}

func encodeRunForUpdate(previous string, run MethodRun) (string, error) {
	encoded, err := json.Marshal(run)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(previous) == "" {
		return string(encoded), nil
	}

	previousFields := map[string]json.RawMessage{}
	if err := json.Unmarshal([]byte(previous), &previousFields); err != nil {
		return string(encoded), nil
	}

	updatedFields := map[string]json.RawMessage{}
	if err := json.Unmarshal(encoded, &updatedFields); err != nil {
		return "", err
	}
	for _, key := range methodRunStructuredKeys {
		delete(previousFields, key)
	}
	for key, value := range updatedFields {
		previousFields[key] = value
	}

	merged, err := json.Marshal(previousFields)
	if err != nil {
		return "", err
	}
	return string(merged), nil
}

func ValidateClose(run MethodRun, input CloseInput) error {
	results := map[string]GateResult{}
	waivers := map[string]string{}
	var invalid []string
	for index, result := range input.GateResults {
		result.GateID = strings.TrimSpace(result.GateID)
		if result.GateID == "" {
			invalid = append(invalid, fmt.Sprintf("gate_results[%d] missing gate_id", index))
			continue
		}
		results[result.GateID] = result
		status := normalizeCloseStatus(result.Status)
		reason := strings.TrimSpace(result.WaiverReason)
		if status == "waived" || reason != "" {
			waivers[result.GateID] = reason
		}
	}
	for index, waiver := range input.Waivers {
		waiver.GateID = strings.TrimSpace(waiver.GateID)
		if waiver.GateID == "" {
			invalid = append(invalid, fmt.Sprintf("waivers[%d] missing gate_id", index))
			continue
		}
		waivers[waiver.GateID] = strings.TrimSpace(waiver.Reason)
	}

	missing := append([]string{}, invalid...)
	for _, card := range run.Methods {
		for _, gate := range card.HardGates {
			result, hasResult := results[gate.ID]
			waiverReason, hasWaiver := waivers[gate.ID]
			if hasWaiver {
				if !gate.Waiver.Allowed {
					missing = append(missing, gate.ID+" cannot be waived")
					continue
				}
				if gate.Waiver.RequiresReason && waiverReason == "" {
					missing = append(missing, gate.ID+" waiver needs reason")
					continue
				}
				continue
			}
			if !hasResult {
				missing = append(missing, gate.ID+" missing gate result")
				continue
			}
			if normalizeCloseStatus(result.Status) != "satisfied" {
				missing = append(missing, gate.ID+" status must be satisfied or waived (use status=\"satisfied\")")
				continue
			}
			if len(gate.RequiredEvidence) > 0 && len(result.EvidenceRefs) == 0 {
				missing = append(missing, gate.ID+" needs evidence_refs")
			}
		}
	}
	missing = append(missing, validateCloseCarryThrough(run, input)...)
	if len(missing) > 0 {
		return fmt.Errorf("method close incomplete: %s; %s", strings.Join(missing, "; "), CloseInputShapeHint())
	}
	return nil
}

const problemClosureHygieneGateID = "problem_graph_closure_hygiene_recorded"

func ValidateProblemClosureHygiene(ctx context.Context, store artifact.ArtifactStore, runArtifact *artifact.Artifact, run MethodRun, input CloseInput) error {
	if !runHasGate(run, problemClosureHygieneGateID) {
		return nil
	}
	if closeInputHasWaiver(input, problemClosureHygieneGateID) {
		return nil
	}

	problemRefs := linkedProblemRefs(runArtifact)
	if len(problemRefs) == 0 {
		return nil
	}

	var missing []string
	for _, ref := range problemRefs {
		if problemHasClosurePath(ctx, store, ref) {
			continue
		}
		missing = append(missing, ref)
	}
	if len(missing) == 0 {
		return nil
	}

	return fmt.Errorf(
		"method close incomplete: linked ProblemCard(s) lack graph closure path: %s; link each problem to a SolutionPortfolio or DecisionRecord with based_on, attach supporting evidence, deprecate/supersede it, or waive gate %q with an operator reason",
		strings.Join(missing, ", "),
		problemClosureHygieneGateID,
	)
}

func runHasGate(run MethodRun, gateID string) bool {
	for _, card := range run.Methods {
		for _, gate := range card.HardGates {
			if gate.ID == gateID {
				return true
			}
		}
	}
	return false
}

func closeInputHasWaiver(input CloseInput, gateID string) bool {
	for _, result := range input.GateResults {
		if strings.TrimSpace(result.GateID) != gateID {
			continue
		}
		if normalizeCloseStatus(result.Status) == "waived" || strings.TrimSpace(result.WaiverReason) != "" {
			return true
		}
	}
	for _, waiver := range input.Waivers {
		if strings.TrimSpace(waiver.GateID) == gateID && strings.TrimSpace(waiver.Reason) != "" {
			return true
		}
	}
	return false
}

func linkedProblemRefs(a *artifact.Artifact) []string {
	if a == nil {
		return nil
	}
	var refs []string
	for _, link := range a.Meta.Links {
		if link.Type != "relates_to" && link.Type != "based_on" {
			continue
		}
		if !strings.HasPrefix(link.Ref, artifact.KindProblemCard.IDPrefix()+"-") {
			continue
		}
		refs = append(refs, link.Ref)
	}
	return dedupeStrings(refs)
}

func problemHasClosurePath(ctx context.Context, store artifact.ArtifactStore, problemRef string) bool {
	problem, err := store.Get(ctx, problemRef)
	if err != nil {
		return false
	}
	if problem.Meta.Kind != artifact.KindProblemCard {
		return false
	}
	if problem.Meta.Status != artifact.StatusActive {
		return true
	}

	backlinks, _ := store.GetBacklinks(ctx, problemRef)
	for _, backlink := range backlinks {
		if backlink.Type != "based_on" {
			continue
		}
		linked, err := store.Get(ctx, backlink.Ref)
		if err != nil {
			continue
		}
		if linked.Meta.Kind == artifact.KindSolutionPortfolio || linked.Meta.Kind == artifact.KindDecisionRecord {
			return true
		}
	}

	evidence, _ := store.GetEvidenceItems(ctx, problemRef)
	for _, item := range evidence {
		if evidenceSupportsClosure(item.Verdict) {
			return true
		}
	}
	return false
}

func evidenceSupportsClosure(verdict string) bool {
	switch strings.TrimSpace(strings.ToLower(verdict)) {
	case "supports", "accepted":
		return true
	default:
		return false
	}
}

func OpenRuns(ctx context.Context, store artifact.ArtifactStore, limit int) ([]MethodRun, error) {
	artifacts, err := store.ListActiveByKind(ctx, artifact.KindMethodRun, limit)
	if err != nil {
		return nil, err
	}
	runs := make([]MethodRun, 0, len(artifacts))
	for _, a := range artifacts {
		full, err := store.Get(ctx, a.Meta.ID)
		if err != nil {
			continue
		}
		run, err := DecodeRun(full)
		if err != nil {
			continue
		}
		if run.Status == "open" {
			runs = append(runs, run)
		}
	}
	return runs, nil
}

func normalizeCloseStatus(status string) string {
	status = strings.TrimSpace(strings.ToLower(status))
	status = strings.ReplaceAll(status, "-", "_")
	status = strings.ReplaceAll(status, " ", "_")
	return status
}

func methodRunTitle(run MethodRun) string {
	task := strings.TrimSpace(run.TaskSignature.Task)
	if task == "" {
		return "Method pull"
	}
	if len(task) > 80 {
		task = task[:80]
	}
	return "Method pull: " + task
}

func methodRunLinks(input PullInput) []artifact.Link {
	var links []artifact.Link
	if input.ArtifactRefs.ProblemRef != "" {
		links = append(links, artifact.Link{Ref: input.ArtifactRefs.ProblemRef, Type: "relates_to"})
	}
	if input.ArtifactRefs.DecisionRef != "" {
		links = append(links, artifact.Link{Ref: input.ArtifactRefs.DecisionRef, Type: "relates_to"})
	}
	if input.ArtifactRefs.CommissionRef != "" {
		links = append(links, artifact.Link{Ref: input.ArtifactRefs.CommissionRef, Type: "relates_to"})
	}
	return links
}

func methodRunSearchKeywords(run MethodRun) string {
	parts := []string{"method", "methodpack", run.TaskSignature.NormalizedTaskKind, run.TaskSignature.ChangeIntent}
	for _, card := range run.Methods {
		parts = append(parts, card.ID)
	}
	for _, signal := range run.TaskSignature.RiskSignals {
		parts = append(parts, signal.ID)
	}
	for _, item := range run.CarryThrough {
		parts = append(parts, item.SourceRef, item.SourceItemRef, item.AcceptanceRef)
	}
	for _, checkpoint := range run.Checkpoints {
		parts = append(parts,
			checkpoint.RecordKind,
			checkpoint.CheckpointID,
			checkpoint.TargetRef,
			checkpoint.CheckRef,
			checkpoint.NextTargetRef,
		)
	}
	return strings.Join(dedupeStrings(parts), " ")
}
