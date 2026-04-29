package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/project/specflow"
)

// handleHaftSpecSection dispatches haft_spec_section MCP tool calls. The
// server-bound project root (parent of haftDir) is the default; callers
// may override via "project_root" arg.
func handleHaftSpecSection(_ context.Context, _ *artifact.Store, haftDir string, args map[string]any) (string, error) {
	action := strings.TrimSpace(stringArg(args, "action"))
	if action == "" {
		return "", fmt.Errorf("action is required")
	}

	projectRoot := strings.TrimSpace(stringArg(args, "project_root"))
	if projectRoot == "" {
		projectRoot = filepath.Dir(haftDir)
	}

	switch action {
	case "next_step":
		return handleSpecSectionNextStep(projectRoot)
	case "approve":
		return handleSpecSectionApprove(projectRoot, args)
	case "rebaseline":
		return handleSpecSectionRebaseline(projectRoot, args)
	case "reopen":
		return handleSpecSectionReopen(projectRoot, args)
	default:
		return "", fmt.Errorf("unknown action: %s", action)
	}
}

func handleSpecSectionNextStep(projectRoot string) (string, error) {
	specSet, err := project.LoadProjectSpecificationSet(projectRoot)
	if err != nil {
		return "", err
	}

	store, projectID, closeFn, _ := projectBaseline(projectRoot)
	defer closeFn()

	intent := specflow.NextStep(specflow.DeriveStateWithBaselines(specSet, store, projectID))

	payload, err := json.Marshal(intent)
	if err != nil {
		return "", fmt.Errorf("marshal intent: %w", err)
	}

	return string(payload), nil
}

// SpecSectionBaselineResult is the response shape for approve / rebaseline
// / reopen actions. Surfaces serialize this as JSON; the same shape is
// reused by the CLI subcommand for parity.
type SpecSectionBaselineResult struct {
	Action     string `json:"action"`
	SectionID  string `json:"section_id"`
	ProjectID  string `json:"project_id"`
	Hash       string `json:"hash,omitempty"`
	CapturedAt string `json:"captured_at,omitempty"`
	ApprovedBy string `json:"approved_by,omitempty"`
	Reason     string `json:"reason,omitempty"`
	Message    string `json:"message"`
}

func handleSpecSectionApprove(projectRoot string, args map[string]any) (string, error) {
	return runBaselineMutation(projectRoot, args, baselineMutation{
		actionLabel: "approve",
		require:     requireApprove,
		apply:       applyApprove,
	})
}

func handleSpecSectionRebaseline(projectRoot string, args map[string]any) (string, error) {
	return runBaselineMutation(projectRoot, args, baselineMutation{
		actionLabel: "rebaseline",
		require:     requireSectionAndReason,
		apply:       applyRebaseline,
	})
}

func handleSpecSectionReopen(projectRoot string, args map[string]any) (string, error) {
	return runBaselineMutation(projectRoot, args, baselineMutation{
		actionLabel: "reopen",
		require:     requireSectionID,
		apply:       applyReopen,
	})
}

type baselineMutation struct {
	actionLabel string
	require     func(args map[string]any) error
	apply       func(ctx baselineContext) (SpecSectionBaselineResult, error)
}

type baselineContext struct {
	actionLabel string
	projectRoot string
	projectID   string
	specSet     project.ProjectSpecificationSet
	store       specflow.BaselineStore
	args        map[string]any
}

func runBaselineMutation(projectRoot string, args map[string]any, mutation baselineMutation) (string, error) {
	if err := mutation.require(args); err != nil {
		return "", err
	}

	store, projectID, closeFn, err := projectBaseline(projectRoot)
	defer closeFn()
	if err != nil {
		return "", err
	}
	if store == nil || projectID == "" {
		return "", fmt.Errorf("project has no .haft/project.yaml or DB; run `haft init` first")
	}

	specSet, err := project.LoadProjectSpecificationSet(projectRoot)
	if err != nil {
		return "", err
	}

	result, err := mutation.apply(baselineContext{
		actionLabel: mutation.actionLabel,
		projectRoot: projectRoot,
		projectID:   projectID,
		specSet:     specSet,
		store:       store,
		args:        args,
	})
	if err != nil {
		return "", err
	}

	payload, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("marshal baseline result: %w", err)
	}
	return string(payload), nil
}

func requireSectionID(args map[string]any) error {
	if strings.TrimSpace(stringArg(args, "section_id")) == "" {
		return fmt.Errorf("section_id is required")
	}
	return nil
}

func requireApprove(args map[string]any) error {
	return requireSectionID(args)
}

func requireSectionAndReason(args map[string]any) error {
	if err := requireSectionID(args); err != nil {
		return err
	}
	if strings.TrimSpace(stringArg(args, "reason")) == "" {
		return fmt.Errorf("reason is required for rebaseline so the audit trail explains the baseline change")
	}
	return nil
}

func applyApprove(ctx baselineContext) (SpecSectionBaselineResult, error) {
	sectionID := strings.TrimSpace(stringArg(ctx.args, "section_id"))
	approvedBy := approvedByArg(ctx.args)

	section, ok := findActiveSection(ctx.specSet, sectionID)
	if !ok {
		return SpecSectionBaselineResult{}, fmt.Errorf(
			"approve requires section %q to exist with status: active in .haft/specs/* before recording a baseline",
			sectionID,
		)
	}

	currentHash := specflow.HashSection(section)

	existing, err := ctx.store.Get(ctx.projectID, sectionID)
	switch {
	case errors.Is(err, specflow.ErrBaselineNotFound):
		// fresh baseline below.
	case err != nil:
		return SpecSectionBaselineResult{}, err
	default:
		if existing.Hash == currentHash {
			return SpecSectionBaselineResult{
				Action:     ctx.actionLabel,
				SectionID:  sectionID,
				ProjectID:  ctx.projectID,
				Hash:       existing.Hash,
				CapturedAt: existing.CapturedAt.UTC().Format(time.RFC3339),
				ApprovedBy: existing.ApprovedBy,
				Message:    "baseline already current",
			}, nil
		}
		return SpecSectionBaselineResult{}, fmt.Errorf(
			"section %q already has a baseline that does not match current carrier; use rebaseline with a reason if the carrier change is intentional, or reopen to drop the baseline",
			sectionID,
		)
	}

	captured := time.Now().UTC()
	baseline := specflow.SectionBaseline{
		ProjectID:  ctx.projectID,
		SectionID:  sectionID,
		Hash:       currentHash,
		CapturedAt: captured,
		ApprovedBy: approvedBy,
	}
	if err := ctx.store.Put(baseline); err != nil {
		return SpecSectionBaselineResult{}, err
	}

	return SpecSectionBaselineResult{
		Action:     ctx.actionLabel,
		SectionID:  sectionID,
		ProjectID:  ctx.projectID,
		Hash:       currentHash,
		CapturedAt: captured.Format(time.RFC3339),
		ApprovedBy: approvedBy,
		Message:    "baseline recorded",
	}, nil
}

func applyRebaseline(ctx baselineContext) (SpecSectionBaselineResult, error) {
	sectionID := strings.TrimSpace(stringArg(ctx.args, "section_id"))
	approvedBy := approvedByArg(ctx.args)
	reason := strings.TrimSpace(stringArg(ctx.args, "reason"))

	section, ok := findActiveSection(ctx.specSet, sectionID)
	if !ok {
		return SpecSectionBaselineResult{}, fmt.Errorf(
			"rebaseline requires section %q to exist with status: active in .haft/specs/*",
			sectionID,
		)
	}

	currentHash := specflow.HashSection(section)
	captured := time.Now().UTC()

	baseline := specflow.SectionBaseline{
		ProjectID:  ctx.projectID,
		SectionID:  sectionID,
		Hash:       currentHash,
		CapturedAt: captured,
		ApprovedBy: approvedBy,
	}
	if err := ctx.store.Put(baseline); err != nil {
		return SpecSectionBaselineResult{}, err
	}

	return SpecSectionBaselineResult{
		Action:     ctx.actionLabel,
		SectionID:  sectionID,
		ProjectID:  ctx.projectID,
		Hash:       currentHash,
		CapturedAt: captured.Format(time.RFC3339),
		ApprovedBy: approvedBy,
		Reason:     reason,
		Message:    "baseline overwritten with current carrier hash",
	}, nil
}

func applyReopen(ctx baselineContext) (SpecSectionBaselineResult, error) {
	sectionID := strings.TrimSpace(stringArg(ctx.args, "section_id"))
	reason := strings.TrimSpace(stringArg(ctx.args, "reason"))

	if err := ctx.store.Delete(ctx.projectID, sectionID); err != nil {
		return SpecSectionBaselineResult{}, err
	}

	return SpecSectionBaselineResult{
		Action:    ctx.actionLabel,
		SectionID: sectionID,
		ProjectID: ctx.projectID,
		Reason:    reason,
		Message:   "baseline removed; section re-enters the onboarding loop on next NextStep call",
	}, nil
}

func approvedByArg(args map[string]any) string {
	approvedBy := strings.TrimSpace(stringArg(args, "approved_by"))
	if approvedBy == "" {
		approvedBy = "human"
	}
	return approvedBy
}

func findActiveSection(set project.ProjectSpecificationSet, sectionID string) (project.SpecSection, bool) {
	for _, section := range set.Sections {
		if section.ID != sectionID {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(section.Status), string(project.SpecSectionStateActive)) {
			continue
		}
		return section, true
	}
	return project.SpecSection{}, false
}
