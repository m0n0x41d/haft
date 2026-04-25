package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/project"
)

var (
	specCheckJSON    bool
	specCoverageJSON bool
	specCheckExit    = os.Exit
)

var specCmd = &cobra.Command{
	Use:   "spec",
	Short: "Inspect project specification carriers",
}

var specCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Run L0/L1/L1.5 structural checks for spec carriers",
	Long: `Run deterministic L0/L1/L1.5 checks for project specification carriers.

The check parses fenced YAML spec-section blocks, validates required structural
fields and L1.5 carrier shapes, and verifies that the term-map carrier has
parseable term entries. It does not perform L2 semantic validation or L3
runtime/evidence validation.`,
	RunE: runSpecCheck,
}

var specCoverageCmd = &cobra.Command{
	Use:   "coverage",
	Short: "Show derived spec coverage by section",
	Long: `Show derived SpecCoverage for active spec sections.

Coverage is computed from spec-section refs, artifact links, WorkCommissions,
affected files, and attached evidence. It does not read or store manual
coverage status fields, and it does not report coverage percentages as the
primary truth.`,
	RunE: runSpecCoverage,
}

func init() {
	specCheckCmd.Flags().BoolVar(&specCheckJSON, "json", false, "print structured JSON output")
	specCoverageCmd.Flags().BoolVar(&specCoverageJSON, "json", false, "print structured JSON output")
	specCmd.AddCommand(specCheckCmd)
	specCmd.AddCommand(specCoverageCmd)
	rootCmd.AddCommand(specCmd)
}

func runSpecCheck(cmd *cobra.Command, _ []string) error {
	projectRoot, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("not a haft project: %w", err)
	}

	report, err := project.CheckSpecificationSet(projectRoot)
	if err != nil {
		return err
	}

	output := cmd.OutOrStdout()
	if specCheckJSON {
		err = writeSpecCheckJSON(output, report)
	} else {
		err = writeSpecCheckSummary(output, report)
	}
	if err != nil {
		return err
	}

	if report.HasFindings() {
		specCheckExit(1)
	}

	return nil
}

func runSpecCoverage(cmd *cobra.Command, _ []string) error {
	projectRoot, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("not a haft project: %w", err)
	}

	report, err := buildSpecCoverageReport(context.Background(), projectRoot)
	if err != nil {
		return err
	}

	output := cmd.OutOrStdout()
	if specCoverageJSON {
		return writeSpecCoverageJSON(output, report)
	}

	return writeSpecCoverageSummary(output, report)
}

func writeSpecCheckJSON(w io.Writer, report project.SpecCheckReport) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")

	return encoder.Encode(report)
}

func writeSpecCheckSummary(w io.Writer, report project.SpecCheckReport) error {
	builder := strings.Builder{}

	if report.HasFindings() {
		builder.WriteString(fmt.Sprintf("haft spec check: L0/L1/L1.5 findings found (%d finding(s))\n", report.Summary.TotalFindings))
	} else {
		builder.WriteString("haft spec check: clean (L0/L1/L1.5)\n")
	}

	builder.WriteString(fmt.Sprintf("spec_sections: %d\n", report.Summary.SpecSections))
	builder.WriteString(fmt.Sprintf("active_spec_sections: %d\n", report.Summary.ActiveSpecSections))
	builder.WriteString(fmt.Sprintf("term_map_entries: %d\n", report.Summary.TermMapEntries))

	if len(report.Findings) > 0 {
		builder.WriteString("\nFindings:\n")
	}

	for _, finding := range report.Findings {
		builder.WriteString(formatSpecCheckFinding(finding))
	}

	_, err := io.WriteString(w, builder.String())

	return err
}

func formatSpecCheckFinding(finding project.SpecCheckFinding) string {
	location := finding.Path
	if finding.Line > 0 {
		location = fmt.Sprintf("%s:%d", finding.Path, finding.Line)
	}

	section := ""
	if finding.SectionID != "" {
		section = " section=" + finding.SectionID
	}

	return fmt.Sprintf("- [%s] %s %s%s - %s\n",
		finding.Level,
		finding.Code,
		location,
		section,
		finding.Message,
	)
}

func buildSpecCoverageReport(
	ctx context.Context,
	projectRoot string,
) (project.SpecCoverageReport, error) {
	specCheck, err := project.CheckSpecificationSet(projectRoot)
	if err != nil {
		return project.SpecCoverageReport{}, err
	}
	if specCheck.HasFindings() {
		return project.SpecCoverageReport{}, fmt.Errorf(
			"spec coverage blocked: spec check has %d finding(s); run `haft spec check` first",
			specCheck.Summary.TotalFindings,
		)
	}

	sections, err := project.LoadSpecSections(projectRoot)
	if err != nil {
		return project.SpecCoverageReport{}, err
	}

	store, closeStore, err := openSpecCoverageStore(projectRoot)
	if err != nil {
		return project.SpecCoverageReport{}, err
	}
	defer closeStore()

	input := project.SpecCoverageInput{
		Sections: sections,
	}
	if store == nil {
		return project.DeriveSpecCoverage(input), nil
	}

	sectionIDs := specCoverageSectionIDSet(sections)
	input.Problems, err = specCoverageProblems(ctx, store, sectionIDs)
	if err != nil {
		return project.SpecCoverageReport{}, err
	}
	input.Decisions, err = specCoverageDecisions(ctx, store, projectRoot, sectionIDs)
	if err != nil {
		return project.SpecCoverageReport{}, err
	}
	input.Commissions, err = specCoverageCommissions(ctx, store, sectionIDs)
	if err != nil {
		return project.SpecCoverageReport{}, err
	}
	input.Evidence, err = specCoverageEvidence(ctx, store, input.Problems, input.Decisions, sectionIDs)
	if err != nil {
		return project.SpecCoverageReport{}, err
	}

	return project.DeriveSpecCoverage(input), nil
}

func openSpecCoverageStore(projectRoot string) (*artifact.Store, func(), error) {
	haftDir := filepath.Join(projectRoot, ".haft")
	cfg, err := project.Load(haftDir)
	if err != nil {
		return nil, func() {}, fmt.Errorf("load project config: %w", err)
	}
	if cfg == nil {
		return nil, func() {}, nil
	}

	dbPath, err := cfg.DBPath()
	if err != nil {
		return nil, func() {}, fmt.Errorf("get DB path: %w", err)
	}

	dsn := dbPath + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(3000)"
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, func() {}, fmt.Errorf("open DB: %w", err)
	}

	closeStore := func() {
		_ = sqlDB.Close()
	}

	return artifact.NewStore(sqlDB), closeStore, nil
}

func specCoverageProblems(
	ctx context.Context,
	store *artifact.Store,
	sectionIDs map[string]struct{},
) ([]project.SpecCoverageProblem, error) {
	items, err := loadSpecCoverageArtifacts(ctx, store, artifact.KindProblemCard)
	if err != nil {
		return nil, err
	}

	problems := make([]project.SpecCoverageProblem, 0, len(items))
	for _, item := range items {
		problems = append(problems, project.SpecCoverageProblem{
			ID:          item.Meta.ID,
			Title:       item.Meta.Title,
			Status:      string(item.Meta.Status),
			ValidUntil:  item.Meta.ValidUntil,
			SectionRefs: explicitSpecCoverageRefs(item, sectionIDs),
		})
	}

	return problems, nil
}

func specCoverageDecisions(
	ctx context.Context,
	store *artifact.Store,
	projectRoot string,
	sectionIDs map[string]struct{},
) ([]project.SpecCoverageDecision, error) {
	items, err := loadSpecCoverageArtifacts(ctx, store, artifact.KindDecisionRecord)
	if err != nil {
		return nil, err
	}

	drifted, err := specCoverageDriftedDecisionSet(ctx, store, projectRoot)
	if err != nil {
		return nil, err
	}

	decisions := make([]project.SpecCoverageDecision, 0, len(items))
	for _, item := range items {
		fields := item.UnmarshalDecisionFields()
		affectedFiles, err := store.GetAffectedFiles(ctx, item.Meta.ID)
		if err != nil {
			return nil, fmt.Errorf("load affected files for %s: %w", item.Meta.ID, err)
		}

		decisions = append(decisions, project.SpecCoverageDecision{
			ID:            item.Meta.ID,
			Title:         item.Meta.Title,
			Status:        string(item.Meta.Status),
			ValidUntil:    item.Meta.ValidUntil,
			ProblemRefs:   specCoverageProblemRefs(item, fields),
			SectionRefs:   explicitSpecCoverageRefs(item, sectionIDs),
			AffectedFiles: specCoverageAffectedFilePaths(affectedFiles),
			Drifted:       drifted[item.Meta.ID],
		})
	}

	return decisions, nil
}

func specCoverageCommissions(
	ctx context.Context,
	store *artifact.Store,
	sectionIDs map[string]struct{},
) ([]project.SpecCoverageCommission, error) {
	items, err := loadSpecCoverageArtifacts(ctx, store, artifact.KindWorkCommission)
	if err != nil {
		return nil, err
	}

	commissions := make([]project.SpecCoverageCommission, 0, len(items))
	for _, item := range items {
		payload, err := decodeWorkCommissionPayload(item.Meta.ID, item.StructuredData)
		if err != nil {
			return nil, err
		}

		validUntil := stringField(payload, "valid_until")
		if validUntil == "" {
			validUntil = item.Meta.ValidUntil
		}

		commissions = append(commissions, project.SpecCoverageCommission{
			ID:          item.Meta.ID,
			DecisionRef: stringField(payload, "decision_ref"),
			State:       stringField(payload, "state"),
			Status:      string(item.Meta.Status),
			ValidUntil:  validUntil,
			SectionRefs: specCoverageRefsFromMap(payload, sectionIDs),
		})
	}

	return commissions, nil
}

func specCoverageEvidence(
	ctx context.Context,
	store *artifact.Store,
	problems []project.SpecCoverageProblem,
	decisions []project.SpecCoverageDecision,
	sectionIDs map[string]struct{},
) ([]project.SpecCoverageEvidence, error) {
	artifactRefs := specCoverageEvidenceArtifactRefs(problems, decisions)
	evidence := make([]project.SpecCoverageEvidence, 0)

	for _, artifactRef := range artifactRefs {
		items, err := store.GetEvidenceItems(ctx, artifactRef)
		if err != nil {
			return nil, fmt.Errorf("load evidence for %s: %w", artifactRef, err)
		}

		for _, item := range items {
			evidence = append(evidence, project.SpecCoverageEvidence{
				ID:          item.ID,
				ArtifactRef: artifactRef,
				Type:        item.Type,
				Verdict:     item.Verdict,
				CarrierRef:  item.CarrierRef,
				ValidUntil:  item.ValidUntil,
				SectionRefs: specCoverageRefsFromEvidence(item, sectionIDs),
				CodeRefs:    specCoverageCodeRefsFromEvidence(item, sectionIDs),
				TestRefs:    specCoverageTestRefsFromEvidence(item, sectionIDs),
			})
		}
	}

	return evidence, nil
}

func loadSpecCoverageArtifacts(
	ctx context.Context,
	store *artifact.Store,
	kind artifact.Kind,
) ([]*artifact.Artifact, error) {
	items, err := store.ListByKind(ctx, kind, 0)
	if err != nil {
		return nil, fmt.Errorf("list %s artifacts: %w", kind, err)
	}

	loaded := make([]*artifact.Artifact, 0, len(items))
	for _, item := range items {
		fullItem, err := store.Get(ctx, item.Meta.ID)
		if err != nil {
			return nil, fmt.Errorf("load artifact %s: %w", item.Meta.ID, err)
		}

		loaded = append(loaded, fullItem)
	}

	return loaded, nil
}

func specCoverageDriftedDecisionSet(
	ctx context.Context,
	store *artifact.Store,
	projectRoot string,
) (map[string]bool, error) {
	reports, err := artifact.CheckDrift(ctx, store, projectRoot)
	if err != nil {
		return nil, fmt.Errorf("scan drift for spec coverage: %w", err)
	}

	drifted := map[string]bool{}
	for _, report := range reports {
		if !report.HasBaseline {
			continue
		}
		if len(report.Files) == 0 {
			continue
		}

		drifted[report.DecisionID] = true
	}

	return drifted, nil
}

func specCoverageProblemRefs(
	item *artifact.Artifact,
	fields artifact.DecisionFields,
) []string {
	refs := append([]string(nil), fields.ProblemRefs...)

	for _, link := range item.Meta.Links {
		if !strings.HasPrefix(link.Ref, artifact.KindProblemCard.IDPrefix()+"-") {
			continue
		}

		refs = append(refs, link.Ref)
	}

	return cleanStringSlice(refs)
}

func specCoverageAffectedFilePaths(files []artifact.AffectedFile) []string {
	paths := make([]string, 0, len(files))

	for _, file := range files {
		paths = append(paths, filepath.ToSlash(file.Path))
	}

	return cleanStringSlice(paths)
}

func specCoverageEvidenceArtifactRefs(
	problems []project.SpecCoverageProblem,
	decisions []project.SpecCoverageDecision,
) []string {
	refs := make([]string, 0, len(problems)+len(decisions))

	for _, problem := range problems {
		refs = append(refs, problem.ID)
	}
	for _, decision := range decisions {
		refs = append(refs, decision.ID)
	}

	return cleanStringSlice(refs)
}

func specCoverageSectionIDSet(sections []project.SpecSection) map[string]struct{} {
	ids := make(map[string]struct{}, len(sections))

	for _, section := range sections {
		ids[section.ID] = struct{}{}
	}

	return ids
}

func explicitSpecCoverageRefs(
	item *artifact.Artifact,
	sectionIDs map[string]struct{},
) []string {
	refs := make([]string, 0)

	for _, link := range item.Meta.Links {
		if _, ok := sectionIDs[link.Ref]; !ok {
			continue
		}

		refs = append(refs, link.Ref)
	}

	refs = append(refs, specCoverageRefsFromStructuredData(item.StructuredData, sectionIDs)...)

	return cleanStringSlice(refs)
}

func specCoverageRefsFromStructuredData(
	data string,
	sectionIDs map[string]struct{},
) []string {
	trimmed := strings.TrimSpace(data)
	if trimmed == "" {
		return nil
	}

	var payload any
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		return nil
	}

	return specCoverageRefsFromValue(payload, sectionIDs)
}

func specCoverageRefsFromMap(
	payload map[string]any,
	sectionIDs map[string]struct{},
) []string {
	return specCoverageRefsFromValue(payload, sectionIDs)
}

func specCoverageRefsFromValue(
	value any,
	sectionIDs map[string]struct{},
) []string {
	switch typed := value.(type) {
	case map[string]any:
		return specCoverageRefsFromObject(typed, sectionIDs)
	case []any:
		return specCoverageRefsFromNestedList(typed, sectionIDs)
	default:
		return nil
	}
}

func specCoverageRefsFromObject(
	payload map[string]any,
	sectionIDs map[string]struct{},
) []string {
	refs := make([]string, 0)

	for key, value := range payload {
		if specCoverageRefKey(key) {
			refs = append(refs, specCoverageRefsFromExplicitValue(value, sectionIDs)...)
			continue
		}

		refs = append(refs, specCoverageRefsFromValue(value, sectionIDs)...)
	}

	return cleanStringSlice(refs)
}

func specCoverageRefsFromNestedList(
	values []any,
	sectionIDs map[string]struct{},
) []string {
	refs := make([]string, 0)

	for _, value := range values {
		refs = append(refs, specCoverageRefsFromValue(value, sectionIDs)...)
	}

	return cleanStringSlice(refs)
}

func specCoverageRefsFromExplicitValue(
	value any,
	sectionIDs map[string]struct{},
) []string {
	switch typed := value.(type) {
	case string:
		return specCoverageRefsFromStrings([]string{typed}, sectionIDs)
	case []string:
		return specCoverageRefsFromStrings(typed, sectionIDs)
	case []any:
		return specCoverageRefsFromExplicitList(typed, sectionIDs)
	default:
		return nil
	}
}

func specCoverageRefsFromExplicitList(
	values []any,
	sectionIDs map[string]struct{},
) []string {
	refs := make([]string, 0, len(values))

	for _, value := range values {
		text, ok := value.(string)
		if !ok {
			continue
		}

		refs = append(refs, text)
	}

	return specCoverageRefsFromStrings(refs, sectionIDs)
}

func specCoverageRefsFromEvidence(
	item artifact.EvidenceItem,
	sectionIDs map[string]struct{},
) []string {
	refs := make([]string, 0)
	refs = append(refs, specCoverageRefsFromStrings(item.ClaimRefs, sectionIDs)...)
	refs = append(refs, specCoverageRefsFromStrings(item.ClaimScope, sectionIDs)...)
	refs = append(refs, specCoverageRefsFromStrings([]string{item.CarrierRef}, sectionIDs)...)

	return cleanStringSlice(refs)
}

func specCoverageRefsFromStrings(
	values []string,
	sectionIDs map[string]struct{},
) []string {
	refs := make([]string, 0, len(values))

	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if _, ok := sectionIDs[trimmed]; !ok {
			continue
		}

		refs = append(refs, trimmed)
	}

	return cleanStringSlice(refs)
}

func specCoverageCodeRefsFromEvidence(
	item artifact.EvidenceItem,
	sectionIDs map[string]struct{},
) []string {
	return specCoveragePathRefsFromEvidence(item, sectionIDs, specCoverageCodePath)
}

func specCoverageTestRefsFromEvidence(
	item artifact.EvidenceItem,
	sectionIDs map[string]struct{},
) []string {
	return specCoveragePathRefsFromEvidence(item, sectionIDs, specCoverageTestPath)
}

func specCoveragePathRefsFromEvidence(
	item artifact.EvidenceItem,
	sectionIDs map[string]struct{},
	predicate func(string) bool,
) []string {
	candidates := make([]string, 0, len(item.ClaimScope)+2)
	candidates = append(candidates, item.CarrierRef)
	candidates = append(candidates, item.ClaimScope...)

	refs := make([]string, 0)
	for _, candidate := range candidates {
		trimmed := strings.TrimSpace(candidate)
		if _, ok := sectionIDs[trimmed]; ok {
			continue
		}
		if !predicate(trimmed) {
			continue
		}

		refs = append(refs, filepath.ToSlash(trimmed))
	}

	return cleanStringSlice(refs)
}

func specCoverageCodePath(value string) bool {
	if value == "" {
		return false
	}
	if specCoverageTestPath(value) {
		return false
	}

	switch strings.ToLower(filepath.Ext(value)) {
	case ".go", ".rs", ".ts", ".tsx", ".js", ".jsx", ".py", ".java", ".kt", ".rb", ".php", ".c", ".cc", ".cpp", ".h", ".hpp":
		return true
	default:
		return false
	}
}

func specCoverageTestPath(value string) bool {
	normalized := strings.ToLower(filepath.ToSlash(value))
	if normalized == "" {
		return false
	}
	if strings.Contains(normalized, "/test/") || strings.Contains(normalized, "/tests/") {
		return true
	}
	if strings.Contains(normalized, "_test.") {
		return true
	}
	if strings.Contains(normalized, ".test.") || strings.Contains(normalized, ".spec.") {
		return true
	}

	return false
}

func specCoverageRefKey(key string) bool {
	switch strings.TrimSpace(key) {
	case "spec_ref", "spec_refs", "spec_section_ref", "spec_section_refs", "section_ref", "section_refs", "target_ref", "target_refs":
		return true
	default:
		return false
	}
}

func writeSpecCoverageJSON(w io.Writer, report project.SpecCoverageReport) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")

	return encoder.Encode(report)
}

func writeSpecCoverageSummary(w io.Writer, report project.SpecCoverageReport) error {
	builder := strings.Builder{}
	builder.WriteString(fmt.Sprintf("haft spec coverage: %d active section(s)\n", report.Summary.TotalSections))
	builder.WriteString("states:\n")

	for _, state := range specCoverageStateOrder() {
		count := report.Summary.StateCounts[string(state)]
		builder.WriteString(fmt.Sprintf("  %s: %d\n", state, count))
	}

	if len(report.Gaps) > 0 {
		builder.WriteString("\nGaps:\n")
		for _, gap := range report.Gaps {
			builder.WriteString(fmt.Sprintf("- %s: %s\n", gap.Kind, gap.Detail))
		}
	}

	if len(report.Sections) > 0 {
		builder.WriteString("\nSections:\n")
	}

	for _, section := range report.Sections {
		builder.WriteString(fmt.Sprintf("- %s [%s]\n", section.SectionID, section.State))
		builder.WriteString(fmt.Sprintf("  why: %s\n", strings.Join(section.Why, "; ")))
		builder.WriteString(fmt.Sprintf("  next_action: %s\n", section.NextAction))
		if len(section.Gaps) > 0 {
			builder.WriteString(fmt.Sprintf("  gaps: %s\n", formatSpecCoverageGapKinds(section.Gaps)))
		}
	}

	_, err := io.WriteString(w, builder.String())

	return err
}

func specCoverageStateOrder() []project.SpecCoverageState {
	return []project.SpecCoverageState{
		project.SpecCoverageUncovered,
		project.SpecCoverageReasoned,
		project.SpecCoverageCommissioned,
		project.SpecCoverageImplemented,
		project.SpecCoverageVerified,
		project.SpecCoverageStale,
	}
}

func formatSpecCoverageGapKinds(gaps []project.SpecCoverageGap) string {
	kinds := make([]string, 0, len(gaps))

	for _, gap := range gaps {
		kinds = append(kinds, gap.Kind)
	}

	return strings.Join(cleanStringSlice(kinds), ", ")
}
