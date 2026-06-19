package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

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

	gate, err := readSpecUseGateFile(specUseGateFile)
	if err != nil {
		return err
	}

	record, err := buildSpecUseRecord(
		projectRoot,
		args[0],
		specUseContext,
		specUsePolicy,
		specUseWaiverExpiresAt,
		gate,
		time.Now().UTC(),
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
	projectRoot string,
	sectionID string,
	useContext string,
	policy string,
	waiverExpiresAt string,
	gate *specflow.OperationalGateProfile,
	now time.Time,
) (specflow.SpecificationUseRecord, error) {
	specSet, err := project.LoadProjectSpecificationSet(projectRoot)
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
		Now:             now,
	}

	return specflow.BuildSpecificationUseRecord(section, baseline, input), nil
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
		"gate_decision: %s reason=%s gate=%s\n",
		record.GateDecision.Status,
		record.GateDecision.Reason,
		record.GateDecision.GateRef,
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
