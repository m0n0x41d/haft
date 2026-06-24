package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/project/specflow"
)

var specUseCmd = &cobra.Command{
	Use:   "use SECTION_ID",
	Short: "Build a read-only SpecificationUseRecord for one active SpecSection",
	Long: `Build a deterministic SpecificationUseRecord for one SpecSection and one
declared use context.

The record separates source edition, baseline currentness, admission policy,
waiver expiry, and GateDecision status. It does not approve, rebaseline,
create evidence, create WorkCommissions, or mutate an OperationalGate.`,
	Args: cobra.ExactArgs(1),
	RunE: runSpecUse,
}

func runSpecUse(cmd *cobra.Command, args []string) error {
	projectRoot, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("not a haft project: %w", err)
	}

	authorityStore, closeAuthorityStore, authorityStoreErr := openArtifactStore(projectRoot)
	if closeAuthorityStore != nil {
		defer closeAuthorityStore()
	}

	gate, err := readSpecUseGateFile(specUseGateFile)
	if err != nil {
		return err
	}

	record, err := buildSpecUseRecord(
		context.Background(),
		projectRoot,
		args[0],
		specUseContext,
		specUsePolicy,
		specUseWaiverExpiresAt,
		gate,
		time.Now().UTC(),
		authorityStore,
		authorityStoreErr,
	)
	if err != nil {
		return err
	}

	output := cmd.OutOrStdout()
	if specUseJSON {
		return writeSpecUseJSON(output, record)
	}

	return writeSpecUseSummary(output, record)
}

func buildSpecUseRecord(
	ctx context.Context,
	projectRoot string,
	sectionID string,
	useContext string,
	policy string,
	waiverExpiresAt string,
	gate *specflow.OperationalGateProfile,
	now time.Time,
	authorityStore artifact.ArtifactStore,
	authorityStoreErr error,
) (specflow.SpecificationUseRecord, error) {
	specSet, err := loadProjectSpecificationSetSQLFirst(projectRoot)
	if err != nil {
		return specflow.SpecificationUseRecord{}, err
	}

	section, ok := specUseSectionByID(specSet, sectionID)
	if !ok {
		return specflow.SpecificationUseRecord{}, fmt.Errorf("spec section %q not found in ProjectSpecificationSet", strings.TrimSpace(sectionID))
	}

	baseline := specUseBaselineInput(projectRoot, section)
	input := specflow.SpecificationUseInput{
		SectionID:       section.ID,
		UseContext:      useContext,
		Policy:          policy,
		WaiverExpiresAt: waiverExpiresAt,
		OperationalGate: gate,
		CurrentAuthority: specUseCurrentAuthority(
			ctx,
			authorityStore,
			authorityStoreErr,
			section,
		),
		Now: now,
	}

	return specflow.BuildSpecificationUseRecord(section, baseline, input), nil
}

func specUseCurrentAuthority(
	ctx context.Context,
	store artifact.ArtifactStore,
	storeErr error,
	section project.SpecSection,
) *specflow.SpecificationUseCurrentAuthority {
	if storeErr != nil {
		return specUseUnknownCurrentAuthority(storeErr.Error())
	}
	if store == nil {
		return specUseUnknownCurrentAuthority("artifact_store_unavailable")
	}

	report, err := artifact.BuildCurrentGoverningSetReport(ctx, store)
	if err != nil {
		return specUseUnknownCurrentAuthority(err.Error())
	}

	sets := specUseCurrentAuthoritySets(report, section)
	return specUseCurrentAuthorityFromSets(report.Authority, sets)
}

func specUseUnknownCurrentAuthority(reason string) *specflow.SpecificationUseCurrentAuthority {
	return &specflow.SpecificationUseCurrentAuthority{
		Status:            specflow.SpecUseCurrentAuthorityUnknown,
		Reason:            "current_authority_frontier_unavailable",
		AuthorityBoundary: specflow.CurrentAuthorityBoundaryReadOnly,
		Error:             strings.TrimSpace(reason),
	}
}

func specUseCurrentAuthoritySets(
	report artifact.CurrentGoverningSetReport,
	section project.SpecSection,
) []artifact.CurrentGoverningSet {
	targetRefs := specUseCurrentAuthorityTargetRefs(section)
	sets := make([]artifact.CurrentGoverningSet, 0, len(report.Sets))
	for _, set := range report.Sets {
		if _, ok := targetRefs[set.TargetRef]; !ok {
			continue
		}

		sets = append(sets, set)
	}

	return sets
}

func specUseCurrentAuthorityTargetRefs(
	section project.SpecSection,
) map[string]struct{} {
	sectionID := strings.TrimSpace(section.ID)
	refs := map[string]struct{}{}
	for _, ref := range []string{
		sectionID,
		"spec_section:" + sectionID,
		"spec-section:" + sectionID,
	} {
		trimmed := strings.TrimSpace(ref)
		if trimmed == "" {
			continue
		}

		refs[trimmed] = struct{}{}
	}

	return refs
}

func specUseCurrentAuthorityFromSets(
	source string,
	sets []artifact.CurrentGoverningSet,
) *specflow.SpecificationUseCurrentAuthority {
	status := specflow.SpecUseCurrentAuthorityClear
	reason := "no_current_authority_conflict_for_spec_section"
	for _, set := range sets {
		if set.Posture == artifact.GoverningSetPostureConflict {
			status = specflow.SpecUseCurrentAuthorityConflict
			reason = "current_authority_conflict_requires_operator"
			break
		}
		if set.Posture == artifact.GoverningSetPostureOverlap {
			status = specflow.SpecUseCurrentAuthorityOverlap
			reason = "current_authority_overlap_requires_review"
		}
	}

	return &specflow.SpecificationUseCurrentAuthority{
		Status:            status,
		Reason:            reason,
		AuthorityBoundary: specflow.CurrentAuthorityBoundaryReadOnly,
		Source:            strings.TrimSpace(source),
		SetRefs:           specUseCurrentAuthoritySetRefs(sets),
		TargetRefs:        specUseCurrentAuthorityTargetRefList(sets),
		DecisionRefs:      specUseCurrentAuthorityDecisionRefs(sets),
	}
}

func specUseCurrentAuthoritySetRefs(
	sets []artifact.CurrentGoverningSet,
) []string {
	refs := make([]string, 0, len(sets))
	for _, set := range sets {
		refs = append(refs, set.SetID)
	}

	return compactSortedStrings(refs)
}

func specUseCurrentAuthorityTargetRefList(
	sets []artifact.CurrentGoverningSet,
) []string {
	refs := make([]string, 0, len(sets))
	for _, set := range sets {
		refs = append(refs, set.TargetRef)
	}

	return compactSortedStrings(refs)
}

func specUseCurrentAuthorityDecisionRefs(
	sets []artifact.CurrentGoverningSet,
) []string {
	refs := []string{}
	for _, set := range sets {
		refs = append(refs, set.CurrentDecisionRefs...)
	}

	return compactSortedStrings(refs)
}

func compactSortedStrings(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}

		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	sort.Strings(out)
	return out
}

func readSpecUseGateFile(path string) (*specflow.OperationalGateProfile, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return nil, nil
	}

	data, err := os.ReadFile(trimmed)
	if err != nil {
		return nil, fmt.Errorf("read operational gate file: %w", err)
	}

	var gate specflow.OperationalGateProfile
	if err := json.Unmarshal(data, &gate); err != nil {
		return nil, fmt.Errorf("parse operational gate file: %w", err)
	}

	return &gate, nil
}

func specUseSectionByID(
	specSet project.ProjectSpecificationSet,
	sectionID string,
) (project.SpecSection, bool) {
	needle := strings.TrimSpace(sectionID)
	for _, section := range specSet.Sections {
		if strings.TrimSpace(section.ID) != needle {
			continue
		}

		return section, true
	}

	return project.SpecSection{}, false
}

func specUseBaselineInput(projectRoot string, section project.SpecSection) specflow.SpecificationUseBaselineInput {
	store, projectID, closeFn, err := projectBaseline(projectRoot)
	defer closeFn()
	if err != nil {
		return specflow.SpecificationUseBaselineInput{
			Status: specflow.SpecUseBaselineError,
			Error:  err.Error(),
		}
	}
	if store == nil || strings.TrimSpace(projectID) == "" {
		return specflow.SpecificationUseBaselineInput{Status: specflow.SpecUseBaselineUnknown}
	}

	baseline, err := store.Get(projectID, section.ID)
	if errors.Is(err, specflow.ErrBaselineNotFound) {
		return specflow.SpecificationUseBaselineInput{
			ProjectID: projectID,
			Status:    specflow.SpecUseBaselineMissing,
		}
	}
	if err != nil {
		return specflow.SpecificationUseBaselineInput{
			ProjectID: projectID,
			Status:    specflow.SpecUseBaselineError,
			Error:     err.Error(),
		}
	}

	status := specflow.SpecUseBaselineCurrent
	if baseline.Hash != specflow.HashSection(section) {
		status = specflow.SpecUseBaselineDrifted
	}

	return specflow.SpecificationUseBaselineInput{
		ProjectID: projectID,
		Status:    status,
		Baseline:  baseline,
	}
}

func writeSpecUseJSON(w io.Writer, record specflow.SpecificationUseRecord) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")

	return encoder.Encode(record)
}

func writeSpecUseSummary(w io.Writer, record specflow.SpecificationUseRecord) error {
	builder := strings.Builder{}

	builder.WriteString(fmt.Sprintf(
		"haft spec use: %s %s section=%s\n",
		record.RecordKind,
		record.Authority,
		record.SectionID,
	))
	builder.WriteString(fmt.Sprintf(
		"admission: %s reason=%s stronger_use=%s\n",
		record.Admission.Disposition,
		record.Admission.Reason,
		record.Admission.StrongerUse,
	))
	builder.WriteString(fmt.Sprintf(
		"source_edition: hash=%s valid_until=%s\n",
		shortSpecUseHash(record.SourceEdition.Hash),
		record.SourceEdition.ValidUntil,
	))
	builder.WriteString(fmt.Sprintf(
		"baseline_currentness: %s current_hash=%s baseline_hash=%s\n",
		record.BaselineCurrentness.Status,
		shortSpecUseHash(record.BaselineCurrentness.CurrentHash),
		shortSpecUseHash(record.BaselineCurrentness.BaselineHash),
	))
	builder.WriteString(fmt.Sprintf(
		"current_authority: %s reason=%s boundary=%s\n",
		record.CurrentAuthority.Status,
		record.CurrentAuthority.Reason,
		record.CurrentAuthority.AuthorityBoundary,
	))
	builder.WriteString(fmt.Sprintf(
		"gate_decision: %s reason=%s gate=%s boundary=%s/%s/%s/%s/%s/%s/%s\n",
		record.GateDecision.Status,
		record.GateDecision.Reason,
		record.GateDecision.GateRef,
		record.GateDecision.AuthorityBoundary.Profile,
		record.GateDecision.AuthorityBoundary.Approval,
		record.GateDecision.AuthorityBoundary.Evidence,
		record.GateDecision.AuthorityBoundary.WorkCommission,
		record.GateDecision.AuthorityBoundary.ClaimTruth,
		record.GateDecision.AuthorityBoundary.GlobalTruth,
		record.GateDecision.AuthorityBoundary.Publication,
	))

	_, err := io.WriteString(w, builder.String())

	return err
}

func shortSpecUseHash(hash string) string {
	trimmed := strings.TrimSpace(hash)
	if len(trimmed) <= 12 {
		return trimmed
	}

	return trimmed[:12]
}
