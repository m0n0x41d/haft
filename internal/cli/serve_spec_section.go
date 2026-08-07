package cli

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/project/specflow"
)

func handleHaftSpecSectionWithProjectionRef(
	ctx context.Context,
	haftDir string,
	args map[string]any,
) (string, string, error) {
	action := strings.TrimSpace(stringArg(args, "action"))
	if action == "" {
		return "", "", fmt.Errorf("action is required")
	}
	if err := validateSpecSectionActionArguments(action, args); err != nil {
		return "", "", err
	}
	if ctx == nil {
		ctx = context.Background()
	}

	projectRoot := strings.TrimSpace(stringArg(args, "project_root"))
	if projectRoot == "" {
		projectRoot = filepath.Dir(haftDir)
	}

	switch action {
	case "lifecycle":
		result, err := handleSpecSectionLifecycle(ctx, projectRoot, args)
		return result, "", err
	case "next_step":
		result, err := handleSpecSectionNextStep(ctx, projectRoot, args)
		return result, "", err
	case "draft_contract":
		result, err := encodeDraftContractJSON()
		return result, "", err
	case "project":
		return handleSpecSectionProjectionSource(
			ctx,
			projectRoot,
			args,
		)
	case "approve":
		result, err := handleSpecSectionApprove(projectRoot, args)
		return result, "", err
	case "rebaseline":
		result, err := handleSpecSectionRebaseline(projectRoot, args)
		return result, "", err
	case "reopen":
		result, err := handleSpecSectionReopen(projectRoot, args)
		return result, "", err
	default:
		return "", "", fmt.Errorf("unknown action: %s", action)
	}
}

func validateSpecSectionActionArguments(
	action string,
	args map[string]any,
) error {
	if action == "draft_contract" {
		return validateProfileIndependentDraftArguments(action, args)
	}
	if action != "lifecycle" && action != "next_step" {
		return nil
	}
	sectionID := strings.TrimSpace(stringArg(args, "section_id"))
	if sectionID == "" {
		return nil
	}
	return fmt.Errorf(
		"section_id_not_applicable: haft_spec_section action=%q is a "+
			"project/scope-level ProjectSpecificationSet workflow projection "+
			"and cannot inspect exact section %q; use "+
			"haft_query(action=\"spec_trace\", section_id=%q) for its current "+
			"edition, lifecycle, and baseline, or "+
			"haft_query(action=\"spec_use\", section_id=%q, "+
			"use_context=\"<named use>\") for stronger-use admission",
		action,
		sectionID,
		sectionID,
		sectionID,
	)
}

func validateProfileIndependentDraftArguments(
	action string,
	args map[string]any,
) error {
	if strings.TrimSpace(stringArg(args, "scope_id")) != "" {
		return fmt.Errorf(
			"scope_id_not_applicable: haft_spec_section action=%q is profile-independent and does not resolve or select project-profile applicability",
			action,
		)
	}
	if strings.TrimSpace(stringArg(args, "section_id")) != "" {
		return fmt.Errorf(
			"section_id_not_applicable: haft_spec_section action=%q publishes the canonical project-independent drafting contract; use haft_query action=spec_validate to check current draft carriers",
			action,
		)
	}
	return nil
}

const specSectionProjectionAuthorityBoundary = "typed_spec_section_at_concern_candidate_not_approval_rebaseline_reopen_evidence_truth_or_authority"
const specSectionProjectionEditionRefPrefix = "spec-section-edition:"
const specSectionProjectionEditionRefSeparator = "@"

type specSectionProjectionSourceResult struct {
	Action            string `json:"action"`
	SectionID         string `json:"section_id"`
	EditionRef        string `json:"edition_ref"`
	SemanticHash      string `json:"semantic_hash"`
	SourceKind        string `json:"source_kind"`
	CarrierPath       string `json:"carrier_path,omitempty"`
	AuthorityBoundary string `json:"authority_boundary"`
}

func newSpecSectionProjectionEditionRef(
	sectionID string,
	semanticHash string,
) (string, error) {
	sectionID = strings.TrimSpace(sectionID)
	semanticHash = strings.TrimSpace(semanticHash)
	if sectionID == "" {
		return "", fmt.Errorf(
			"SpecSection projection requires an exact section ID",
		)
	}
	decoded, err := hex.DecodeString(semanticHash)
	if err != nil || len(decoded) != 32 {
		return "", fmt.Errorf(
			"SpecSection projection requires an exact SHA-256 semantic hash",
		)
	}
	return specSectionProjectionEditionRefPrefix +
		sectionID +
		specSectionProjectionEditionRefSeparator +
		semanticHash, nil
}

func parseSpecSectionProjectionEditionRef(
	raw string,
) (string, string, error) {
	value := strings.TrimSpace(raw)
	if !strings.HasPrefix(
		value,
		specSectionProjectionEditionRefPrefix,
	) {
		return "", "", fmt.Errorf(
			"invalid SpecSection edition reference %q",
			raw,
		)
	}
	coordinate := strings.TrimPrefix(
		value,
		specSectionProjectionEditionRefPrefix,
	)
	separator := strings.LastIndex(
		coordinate,
		specSectionProjectionEditionRefSeparator,
	)
	if separator <= 0 || separator == len(coordinate)-1 {
		return "", "", fmt.Errorf(
			"invalid SpecSection edition reference %q",
			raw,
		)
	}
	sectionID := coordinate[:separator]
	semanticHash := coordinate[separator+1:]
	exact, err := newSpecSectionProjectionEditionRef(
		sectionID,
		semanticHash,
	)
	if err != nil {
		return "", "", err
	}
	if exact != value {
		return "", "", fmt.Errorf(
			"SpecSection edition reference is noncanonical",
		)
	}
	return sectionID, semanticHash, nil
}

func handleSpecSectionProjectionSource(
	ctx context.Context,
	projectRoot string,
	args map[string]any,
) (string, string, error) {
	if err := requireSectionID(args); err != nil {
		return "", "", err
	}
	cfg, err := project.Load(
		filepath.Join(projectRoot, ".haft"),
	)
	if err != nil {
		return "", "", err
	}
	projectID, store, closeStore, err :=
		openSpecSectionEditionReadStore(
			ctx,
			projectRoot,
			cfg,
		)
	if err != nil {
		return "", "", err
	}
	defer closeStore()

	sectionID := strings.TrimSpace(stringArg(args, "section_id"))
	edition, err := store.GetCurrent(
		projectID,
		sectionID,
	)
	if err != nil {
		return "", "", fmt.Errorf(
			"load current SpecSection edition %q: %w",
			sectionID,
			err,
		)
	}
	editionRef, err := newSpecSectionProjectionEditionRef(
		edition.SectionID,
		edition.SemanticHash,
	)
	if err != nil {
		return "", "", err
	}
	result := specSectionProjectionSourceResult{
		Action:            "project",
		SectionID:         edition.SectionID,
		EditionRef:        editionRef,
		SemanticHash:      edition.SemanticHash,
		SourceKind:        string(edition.SourceKind),
		CarrierPath:       edition.CarrierPath,
		AuthorityBoundary: specSectionProjectionAuthorityBoundary,
	}
	payload, err := json.Marshal(result)
	if err != nil {
		return "", "", fmt.Errorf(
			"marshal SpecSection projection source: %w",
			err,
		)
	}
	return string(payload), editionRef, nil
}

func handleSpecSectionLifecycle(
	ctx context.Context,
	projectRoot string,
	args map[string]any,
) (string, error) {
	request, err := projectSpecificationScopeRequestFromSpecSectionArgs(args)
	if err != nil {
		return "", err
	}
	result, err := buildPublicSpecLifecycle(ctx, projectRoot, request)
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("marshal lifecycle projection: %w", err)
	}

	return string(payload), nil
}

type publicSpecNextStepResult struct {
	*specflow.WorkflowIntent
	ProfileApplicability publicProjectSpecificationApplicability `json:"profile_applicability"`
}

func handleSpecSectionNextStep(
	ctx context.Context,
	projectRoot string,
	args map[string]any,
) (string, error) {
	request, err := projectSpecificationScopeRequestFromSpecSectionArgs(args)
	if err != nil {
		return "", err
	}
	lifecycle, err := buildPublicSpecLifecycle(ctx, projectRoot, request)
	if err != nil {
		return "", err
	}
	result := publicSpecNextStepFromLifecycle(lifecycle)
	payload, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("marshal intent: %w", err)
	}

	return string(payload), nil
}

func publicSpecNextStepFromLifecycle(
	lifecycle publicSpecLifecycleResult,
) publicSpecNextStepResult {
	result := publicSpecNextStepResult{
		ProfileApplicability: lifecycle.ProfileApplicability,
	}
	if lifecycle.SpecLifecycleProjection == nil {
		return result
	}
	result.WorkflowIntent = &lifecycle.WorkflowIntent
	return result
}

func projectSpecificationScopeRequestFromSpecSectionArgs(
	args map[string]any,
) (projectSpecificationScopeRequest, error) {
	raw, found := args["scope_id"]
	if !found {
		return automaticProjectSpecificationScopeRequest(), nil
	}
	scopeID, ok := raw.(string)
	if !ok {
		return projectSpecificationScopeRequest{}, fmt.Errorf(
			"scope_id must be a string",
		)
	}
	request, err := projectSpecificationScopeRequestFromFlag(scopeID)
	if err != nil {
		return projectSpecificationScopeRequest{}, fmt.Errorf(
			"invalid scope_id: %w",
			err,
		)
	}
	return request, nil
}

// SpecSectionBaselineResult is the response shape for approve / rebaseline
// / reopen actions. Surfaces serialize this as JSON; the same shape is
// reused by the CLI subcommand for parity.
type SpecSectionBaselineResult struct {
	Action          string                        `json:"action"`
	BaselineKind    specflow.BaselineKind         `json:"baseline_kind,omitempty"`
	BaselineProfile *specflow.BaselineKindProfile `json:"baseline_profile,omitempty"`
	SectionID       string                        `json:"section_id"`
	ProjectID       string                        `json:"project_id"`
	Hash            string                        `json:"hash,omitempty"`
	CapturedAt      string                        `json:"captured_at,omitempty"`
	ApprovedBy      string                        `json:"approved_by,omitempty"`
	Reason          string                        `json:"reason,omitempty"`
	Message         string                        `json:"message"`
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
		require:     requireRebaseline,
		apply:       applyRebaseline,
	})
}

func handleSpecSectionReopen(projectRoot string, args map[string]any) (string, error) {
	return runBaselineMutation(projectRoot, args, baselineMutation{
		actionLabel: "reopen",
		require:     requireReopen,
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

	specSet, err := loadProjectSpecificationSetSQLFirst(projectRoot)
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

func requireRebaseline(args map[string]any) error {
	return requireSectionReason(args, "rebaseline", "the baseline change")
}

func requireReopen(args map[string]any) error {
	return requireSectionReason(args, "reopen", "why the baseline is reopened")
}

func requireSectionReason(args map[string]any, action string, auditSubject string) error {
	if err := requireSectionID(args); err != nil {
		return err
	}
	if strings.TrimSpace(stringArg(args, "reason")) == "" {
		return fmt.Errorf("reason is required for %s so the audit trail explains %s", action, auditSubject)
	}
	return nil
}

func applyApprove(ctx baselineContext) (SpecSectionBaselineResult, error) {
	sectionID := strings.TrimSpace(stringArg(ctx.args, "section_id"))
	approvedBy := approvedByArg(ctx.args)

	section, ok := findActiveSection(ctx.specSet, sectionID)
	if !ok {
		return SpecSectionBaselineResult{}, fmt.Errorf(
			"approve requires current SpecSection %q to exist with status: active before recording a baseline",
			sectionID,
		)
	}

	if err := requireCleanSpecCheckForApprove(ctx.specSet, section); err != nil {
		return SpecSectionBaselineResult{}, err
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
				Action:          ctx.actionLabel,
				BaselineKind:    existing.Kind,
				BaselineProfile: baselineProfile(existing.Kind),
				SectionID:       sectionID,
				ProjectID:       ctx.projectID,
				Hash:            existing.Hash,
				CapturedAt:      existing.CapturedAt.UTC().Format(time.RFC3339),
				ApprovedBy:      existing.ApprovedBy,
				Message:         "baseline already current",
			}, nil
		}
		return SpecSectionBaselineResult{}, fmt.Errorf(
			"section %q already has a baseline that does not match current carrier; use rebaseline with a reason if the carrier change is intentional, or reopen to drop the baseline",
			sectionID,
		)
	}

	captured := time.Now().UTC()
	baseline := specflow.NewSpecSectionApprovalBaseline(ctx.projectID, section, approvedBy, captured)
	if err := ctx.store.PutSpecSectionApproval(baseline); err != nil {
		return SpecSectionBaselineResult{}, err
	}

	return SpecSectionBaselineResult{
		Action:          ctx.actionLabel,
		BaselineKind:    specflow.BaselineKindSpecSectionApproval,
		BaselineProfile: baselineProfile(specflow.BaselineKindSpecSectionApproval),
		SectionID:       sectionID,
		ProjectID:       ctx.projectID,
		Hash:            baseline.Hash,
		CapturedAt:      captured.Format(time.RFC3339),
		ApprovedBy:      approvedBy,
		Message:         "baseline recorded",
	}, nil
}

func requireCleanSpecCheckForApprove(specSet project.ProjectSpecificationSet, section project.SpecSection) error {
	report := project.SpecCheckReportFromSpecificationSet(specSet)
	findings := approveBlockingFindings(report, section)
	if len(findings) == 0 {
		return nil
	}

	lines := make([]string, 0, len(findings))
	for _, finding := range findings {
		line := formatSpecCheckFinding(finding)
		line = strings.TrimSpace(line)
		lines = append(lines, line)
	}

	message := strings.Join(lines, "\n")
	return fmt.Errorf(
		"approve blocked by spec check findings; run `haft spec check --json` and fix before recording a baseline:\n%s",
		message,
	)
}

func approveBlockingFindings(report project.SpecCheckReport, section project.SpecSection) []project.SpecCheckFinding {
	sectionPath := strings.TrimSpace(section.Path)
	sectionPath = filepath.ToSlash(sectionPath)

	termMapPath := filepath.Join(".haft", "specs", "term-map.md")
	termMapPath = filepath.ToSlash(termMapPath)

	findings := make([]project.SpecCheckFinding, 0, len(report.Findings))
	for _, finding := range report.Findings {
		findingPath := strings.TrimSpace(finding.Path)
		findingPath = filepath.ToSlash(findingPath)

		switch {
		case finding.SectionID == section.ID:
			findings = append(findings, finding)
		case findingPath == sectionPath:
			findings = append(findings, finding)
		case findingPath == termMapPath:
			findings = append(findings, finding)
		}
	}

	return findings
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

	captured := time.Now().UTC()

	baseline := specflow.NewSpecSectionApprovalBaseline(ctx.projectID, section, approvedBy, captured)
	if err := ctx.store.PutSpecSectionApproval(baseline); err != nil {
		return SpecSectionBaselineResult{}, err
	}

	return SpecSectionBaselineResult{
		Action:          ctx.actionLabel,
		BaselineKind:    specflow.BaselineKindSpecSectionApproval,
		BaselineProfile: baselineProfile(specflow.BaselineKindSpecSectionApproval),
		SectionID:       sectionID,
		ProjectID:       ctx.projectID,
		Hash:            baseline.Hash,
		CapturedAt:      captured.Format(time.RFC3339),
		ApprovedBy:      approvedBy,
		Reason:          reason,
		Message:         "baseline overwritten with current carrier hash",
	}, nil
}

func applyReopen(ctx baselineContext) (SpecSectionBaselineResult, error) {
	sectionID := strings.TrimSpace(stringArg(ctx.args, "section_id"))
	reason := strings.TrimSpace(stringArg(ctx.args, "reason"))

	if err := ctx.store.Delete(ctx.projectID, sectionID); err != nil {
		return SpecSectionBaselineResult{}, err
	}

	return SpecSectionBaselineResult{
		Action:          ctx.actionLabel,
		BaselineKind:    specflow.BaselineKindSpecSectionApproval,
		BaselineProfile: baselineProfile(specflow.BaselineKindSpecSectionApproval),
		SectionID:       sectionID,
		ProjectID:       ctx.projectID,
		Reason:          reason,
		Message:         "baseline removed; section re-enters the onboarding loop on next NextStep call",
	}, nil
}

func baselineProfile(kind specflow.BaselineKind) *specflow.BaselineKindProfile {
	profile := specflow.DescribeBaselineKind(kind)
	return &profile
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
