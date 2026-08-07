package cli

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

const (
	processValueSliceKind      = "haft_process_value_slice"
	processValueSliceAuthority = "read_only_value_slice_report_not_product_value_proof"
)

var (
	processValueSliceJSON  bool
	processValueSliceInput string
)

var processValueSliceCmd = &cobra.Command{
	Use:   "value-slice",
	Short: "Report equal-budget AI SWE product-value observations",
	Long: `Read a small, explicit value-slice observation file and return a bounded
report for comparing Haft MethodPack/carry-through/process checks against a
baseline agent condition.

The report is evidence input only. It is not product-value proof, not a scalar
TPF maturity score, not evidence truth, not approval, and not gate passage.`,
	RunE: runProcessValueSlice,
}

type processValueSliceInputEnvelope struct {
	Cases []processValueSliceCase `json:"cases,omitempty"`
	Tasks []processValueSliceCase `json:"tasks,omitempty"`
}

type processValueSliceCase struct {
	TaskID                          string                         `json:"task_id,omitempty"`
	Title                           string                         `json:"title,omitempty"`
	TaskText                        string                         `json:"task_text,omitempty"`
	TaskTextDigest                  string                         `json:"task_text_digest,omitempty"`
	ComparisonGroupRef              string                         `json:"comparison_group_ref,omitempty"`
	Model                           string                         `json:"model,omitempty"`
	Host                            string                         `json:"host,omitempty"`
	ToolBudget                      string                         `json:"tool_budget,omitempty"`
	TimeWindow                      string                         `json:"time_window,omitempty"`
	Condition                       string                         `json:"condition,omitempty"`
	AcceptedFindingsAtStart         int                            `json:"accepted_findings_at_start,omitempty"`
	MissedAcceptanceCriteriaCount   int                            `json:"missed_acceptance_criteria_count,omitempty"`
	CloseState                      string                         `json:"close_state,omitempty"`
	UnresolvedAcceptedFindings      int                            `json:"unresolved_accepted_findings_at_close,omitempty"`
	ReviewFindingsAfterDone         int                            `json:"review_findings_after_done,omitempty"`
	ReworkCycles                    int                            `json:"rework_cycles,omitempty"`
	OperatorCorrections             int                            `json:"operator_corrections,omitempty"`
	CloseRejectionsBeforeValidClose int                            `json:"close_rejections_before_valid_close,omitempty"`
	DefaultStatusActionLines        int                            `json:"default_status_action_lines,omitempty"`
	EscapedGovernedDrift            int                            `json:"escaped_governed_drift,omitempty"`
	TokensCallsWallclock            processValueSliceRuntimeVector `json:"tokens_calls_wallclock,omitempty"`
	CheckpointHeavy                 bool                           `json:"checkpoint_heavy,omitempty"`
	ObservedFields                  []string                       `json:"observed_fields,omitempty"`
	PolicyObservedFields            []string                       `json:"-"`
}

type processValueSliceRuntimeVector struct {
	Tokens    int     `json:"tokens,omitempty"`
	Calls     int     `json:"calls,omitempty"`
	Wallclock float64 `json:"wallclock,omitempty"`
}

type processValueSliceReport struct {
	Kind             string                              `json:"kind"`
	SchemaVersion    int                                 `json:"schema_version"`
	Authority        string                              `json:"authority"`
	MutationBoundary []string                            `json:"mutation_boundary"`
	InputRef         string                              `json:"input_ref"`
	Summary          processValueSliceSummary            `json:"summary"`
	BudgetParity     []processValueSliceBudgetGroup      `json:"budget_parity"`
	PolicyInput      processValueSlicePolicyInput        `json:"policy_input"`
	Missingness      processValueSliceMissingnessSummary `json:"missingness"`
	Conditions       []processValueSliceCondition        `json:"conditions"`
	Policy           processValueSlicePolicy             `json:"policy"`
	Cases            []processValueSliceCaseObservation  `json:"cases,omitempty"`
	Notes            []string                            `json:"notes,omitempty"`
}

type processValueSliceSummary struct {
	Cases             int `json:"cases"`
	HaftMethodPack    int `json:"haft_methodpack"`
	BaselineAgent     int `json:"baseline_agent"`
	OtherConditions   int `json:"other_conditions"`
	EqualBudgetGroups int `json:"equal_budget_groups"`
}

type processValueSliceBudgetGroup struct {
	BudgetKey          string         `json:"budget_key"`
	ComparisonKey      string         `json:"comparison_key,omitempty"`
	ComparisonGroupRef string         `json:"comparison_group_ref,omitempty"`
	TaskID             string         `json:"task_id,omitempty"`
	TaskTextDigest     string         `json:"task_text_digest,omitempty"`
	Model              string         `json:"model,omitempty"`
	Host               string         `json:"host,omitempty"`
	ToolBudget         string         `json:"tool_budget,omitempty"`
	TimeWindow         string         `json:"time_window,omitempty"`
	BudgetComplete     bool           `json:"budget_complete"`
	ComparisonComplete bool           `json:"comparison_complete"`
	ByCondition        map[string]int `json:"by_condition"`
	EqualBudgetPair    bool           `json:"equal_budget_pair"`
}

type processValueSlicePolicyInput struct {
	PairedCases                int `json:"paired_cases"`
	UnpairedCases              int `json:"unpaired_cases"`
	EqualBudgetGroups          int `json:"equal_budget_groups"`
	PairedHaftCases            int `json:"paired_haft_methodpack_cases"`
	PairedBaselineCases        int `json:"paired_baseline_agent_cases"`
	IncompleteBudgetGroups     int `json:"incomplete_budget_groups"`
	IncompleteComparisonGroups int `json:"incomplete_comparison_groups"`
}

type processValueSliceCondition struct {
	Condition      string                  `json:"condition"`
	Cases          int                     `json:"cases"`
	ObservedVector processValueSliceVector `json:"observed_vector"`
}

type processValueSliceVector struct {
	AcceptedFindingsAtStart           int                            `json:"accepted_findings_at_start"`
	MissedAcceptanceCriteriaCount     int                            `json:"missed_acceptance_criteria_count"`
	UnresolvedAcceptedFindingsAtClose int                            `json:"unresolved_accepted_findings_at_close"`
	ReviewFindingsAfterDone           int                            `json:"review_findings_after_done"`
	ReworkCycles                      int                            `json:"rework_cycles"`
	OperatorCorrections               int                            `json:"operator_corrections"`
	CloseRejectionsBeforeValidClose   int                            `json:"close_rejections_before_valid_close"`
	DefaultStatusActionLines          int                            `json:"default_status_action_lines"`
	EscapedGovernedDrift              int                            `json:"escaped_governed_drift"`
	TokensCallsWallclock              processValueSliceRuntimeVector `json:"tokens_calls_wallclock"`
	CheckpointHeavyCases              int                            `json:"checkpoint_heavy_cases"`
}

type processValueSlicePolicy struct {
	Label     string   `json:"label"`
	Rationale []string `json:"rationale"`
}

type processValueSliceCaseObservation struct {
	TaskID             string                  `json:"task_id,omitempty"`
	Title              string                  `json:"title,omitempty"`
	ComparisonGroupRef string                  `json:"comparison_group_ref,omitempty"`
	Condition          string                  `json:"condition"`
	BudgetKey          string                  `json:"budget_key"`
	ComparisonKey      string                  `json:"comparison_key,omitempty"`
	ObservedFields     []string                `json:"observed_fields,omitempty"`
	MissingFields      []string                `json:"missing_fields,omitempty"`
	ObservedVector     processValueSliceVector `json:"observed_vector"`
}

type processValueSliceMissingnessSummary struct {
	RequiredFieldsForPolicy        []string       `json:"required_fields_for_policy"`
	PairedCasesChecked             int            `json:"paired_cases_checked"`
	CasesWithMissingRequiredFields int            `json:"cases_with_missing_required_fields"`
	MissingByField                 map[string]int `json:"missing_by_field,omitempty"`
}

var (
	processValueSliceObservableFields = map[string]struct{}{
		"accepted_findings_at_start":            {},
		"missed_acceptance_criteria_count":      {},
		"unresolved_accepted_findings_at_close": {},
		"review_findings_after_done":            {},
		"rework_cycles":                         {},
		"operator_corrections":                  {},
		"close_rejections_before_valid_close":   {},
		"default_status_action_lines":           {},
		"escaped_governed_drift":                {},
		"tokens_calls_wallclock":                {},
		"checkpoint_heavy":                      {},
	}
	processValueSliceRequiredPolicyFields = []string{
		"accepted_findings_at_start",
		"missed_acceptance_criteria_count",
		"unresolved_accepted_findings_at_close",
		"review_findings_after_done",
		"rework_cycles",
		"operator_corrections",
		"default_status_action_lines",
	}
)

func (item *processValueSliceCase) UnmarshalJSON(data []byte) error {
	type caseAlias processValueSliceCase
	decoded := caseAlias{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	raw := map[string]json.RawMessage{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	observed := map[string]struct{}{}
	policyObserved := map[string]struct{}{}
	for key := range raw {
		field := processValueSliceNormalizeFieldName(key)
		if _, ok := processValueSliceObservableFields[field]; ok {
			if processValueSliceRawFieldObserved(field, raw[key]) {
				observed[field] = struct{}{}
			}
		}
		if _, ok := processValueSliceRequiredPolicyFieldSet()[field]; ok {
			if processValueSliceRawNumericObserved(raw[key]) {
				policyObserved[field] = struct{}{}
			}
		}
	}
	for _, field := range decoded.ObservedFields {
		normalized := processValueSliceNormalizeFieldName(field)
		if _, ok := processValueSliceObservableFields[normalized]; ok {
			observed[normalized] = struct{}{}
		}
	}
	decoded.ObservedFields = processValueSliceSortedKeys(observed)
	decoded.PolicyObservedFields = processValueSliceSortedKeys(policyObserved)
	*item = processValueSliceCase(decoded)
	return nil
}

func init() {
	processValueSliceCmd.Flags().BoolVar(&processValueSliceJSON, "json", false, "print structured JSON output")
	processValueSliceCmd.Flags().StringVar(&processValueSliceInput, "input", "", "value-slice observation JSON file")
	processCmd.AddCommand(processValueSliceCmd)
}

func runProcessValueSlice(cmd *cobra.Command, _ []string) error {
	inputPath := strings.TrimSpace(processValueSliceInput)
	if inputPath == "" {
		return fmt.Errorf("--input is required")
	}
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read value-slice input: %w", err)
	}
	report, err := buildProcessValueSliceReport(inputPath, data)
	if err != nil {
		return err
	}
	if processValueSliceJSON {
		return writeJSON(cmd.OutOrStdout(), report)
	}
	return writeProcessValueSliceText(cmd.OutOrStdout(), report)
}

func buildProcessValueSliceReport(inputRef string, data []byte) (processValueSliceReport, error) {
	cases, err := decodeProcessValueSliceCases(data)
	if err != nil {
		return processValueSliceReport{}, err
	}
	budgetGroups := processValueSliceBudgetGroups(cases)
	pairedCases := processValueSlicePairedCases(cases, budgetGroups)
	report := processValueSliceReport{
		Kind:          processValueSliceKind,
		SchemaVersion: 1,
		Authority:     processValueSliceAuthority,
		MutationBoundary: []string{
			"read_only_value_observation",
			"does_not_mutate_method_runs_decisions_evidence_or_carriers",
			"does_not_create_product_value_proof_or_scalar_maturity_score",
			"does_not_create_evidence_truth_approval_gate_passage_global_truth_or_publication",
		},
		InputRef:     valueSliceInputRef(inputRef),
		BudgetParity: budgetGroups,
		PolicyInput:  processValueSlicePolicyInputFrom(pairedCases, cases, budgetGroups),
		Missingness:  processValueSliceMissingnessFrom(pairedCases),
		Conditions:   processValueSliceConditions(cases),
		Cases:        processValueSliceCaseObservations(cases),
		Notes: []string{
			"Value-slice reports are evidence input only, not product-value proof.",
			"Policy labels are computed only from comparison_group_ref/task_id/task_text_digest/task_text-paired and budget-complete haft_methodpack/baseline_agent groups.",
			"Required policy metrics must be present as numeric non-null values; observed_fields cannot invent missing numbers.",
			"No natural-language classifier or scalar TPF maturity score is used.",
		},
	}
	report.Summary = processValueSliceSummaryFrom(report, cases)
	report.Policy = processValueSlicePolicyFrom(processValueSliceConditions(pairedCases), report.Missingness)
	return report, nil
}

func decodeProcessValueSliceCases(data []byte) ([]processValueSliceCase, error) {
	var envelope processValueSliceInputEnvelope
	if err := json.Unmarshal(data, &envelope); err == nil {
		cases := append([]processValueSliceCase{}, envelope.Cases...)
		cases = append(cases, envelope.Tasks...)
		if len(cases) > 0 {
			return normalizeProcessValueSliceCases(cases), nil
		}
	}
	var cases []processValueSliceCase
	if err := json.Unmarshal(data, &cases); err == nil && len(cases) > 0 {
		return normalizeProcessValueSliceCases(cases), nil
	}
	var single processValueSliceCase
	if err := json.Unmarshal(data, &single); err != nil {
		return nil, fmt.Errorf("decode value-slice input: %w", err)
	}
	if strings.TrimSpace(single.TaskID) == "" && strings.TrimSpace(single.Title) == "" {
		return nil, fmt.Errorf("value-slice input needs at least one case with task_id or title")
	}
	return normalizeProcessValueSliceCases([]processValueSliceCase{single}), nil
}

func normalizeProcessValueSliceCases(cases []processValueSliceCase) []processValueSliceCase {
	normalized := make([]processValueSliceCase, 0, len(cases))
	for _, item := range cases {
		item.Condition = processNormalizeStatus(item.Condition)
		item.CloseState = processNormalizeStatus(item.CloseState)
		item.Model = strings.TrimSpace(item.Model)
		item.Host = strings.TrimSpace(item.Host)
		item.ToolBudget = strings.TrimSpace(item.ToolBudget)
		item.TimeWindow = strings.TrimSpace(item.TimeWindow)
		item.TaskID = strings.TrimSpace(item.TaskID)
		item.Title = strings.TrimSpace(item.Title)
		item.TaskText = strings.TrimSpace(item.TaskText)
		item.TaskTextDigest = strings.TrimSpace(item.TaskTextDigest)
		item.ComparisonGroupRef = strings.TrimSpace(item.ComparisonGroupRef)
		item.ObservedFields = processValueSliceNormalizeObservedFields(item.ObservedFields)
		item.PolicyObservedFields = processValueSliceNormalizeObservedFields(item.PolicyObservedFields)
		normalized = append(normalized, item)
	}
	return normalized
}

func processValueSliceBudgetGroups(cases []processValueSliceCase) []processValueSliceBudgetGroup {
	byKey := map[string]processValueSliceBudgetGroup{}
	for _, item := range cases {
		key := processValueSliceBudgetKey(item)
		group := byKey[key]
		if group.BudgetKey == "" {
			group = processValueSliceBudgetGroup{
				BudgetKey:          key,
				ComparisonKey:      processValueSliceComparisonKey(item),
				ComparisonGroupRef: item.ComparisonGroupRef,
				TaskID:             item.TaskID,
				TaskTextDigest:     processValueSliceTaskTextDigest(item),
				Model:              item.Model,
				Host:               item.Host,
				ToolBudget:         item.ToolBudget,
				TimeWindow:         item.TimeWindow,
				BudgetComplete:     processValueSliceBudgetComplete(item),
				ComparisonComplete: processValueSliceComparisonComplete(item),
				ByCondition:        map[string]int{},
			}
		}
		group.ByCondition[item.Condition]++
		group.EqualBudgetPair = group.BudgetComplete &&
			group.ComparisonComplete &&
			group.ByCondition["haft_methodpack"] > 0 &&
			group.ByCondition["baseline_agent"] > 0
		byKey[key] = group
	}
	keys := make([]string, 0, len(byKey))
	for key := range byKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	groups := make([]processValueSliceBudgetGroup, 0, len(keys))
	for _, key := range keys {
		groups = append(groups, byKey[key])
	}
	return groups
}

func processValueSlicePairedCases(cases []processValueSliceCase, groups []processValueSliceBudgetGroup) []processValueSliceCase {
	pairedKeys := map[string]struct{}{}
	for _, group := range groups {
		if group.EqualBudgetPair {
			pairedKeys[group.BudgetKey] = struct{}{}
		}
	}
	paired := make([]processValueSliceCase, 0, len(cases))
	for _, item := range cases {
		if _, ok := pairedKeys[processValueSliceBudgetKey(item)]; !ok {
			continue
		}
		if item.Condition != "haft_methodpack" && item.Condition != "baseline_agent" {
			continue
		}
		paired = append(paired, item)
	}
	return paired
}

func processValueSliceConditions(cases []processValueSliceCase) []processValueSliceCondition {
	byCondition := map[string]processValueSliceCondition{}
	for _, item := range cases {
		condition := item.Condition
		if condition == "" {
			condition = "unknown"
		}
		current := byCondition[condition]
		current.Condition = condition
		current.Cases++
		current.ObservedVector = processValueSliceVectorAdd(current.ObservedVector, processValueSliceVectorFromCase(item))
		byCondition[condition] = current
	}
	keys := make([]string, 0, len(byCondition))
	for key := range byCondition {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	conditions := make([]processValueSliceCondition, 0, len(keys))
	for _, key := range keys {
		conditions = append(conditions, byCondition[key])
	}
	return conditions
}

func processValueSliceCaseObservations(cases []processValueSliceCase) []processValueSliceCaseObservation {
	observations := make([]processValueSliceCaseObservation, 0, len(cases))
	for _, item := range cases {
		condition := item.Condition
		if condition == "" {
			condition = "unknown"
		}
		observations = append(observations, processValueSliceCaseObservation{
			TaskID:             item.TaskID,
			Title:              item.Title,
			ComparisonGroupRef: item.ComparisonGroupRef,
			Condition:          condition,
			BudgetKey:          processValueSliceBudgetKey(item),
			ComparisonKey:      processValueSliceComparisonKey(item),
			ObservedFields:     item.ObservedFields,
			MissingFields:      processValueSliceMissingRequiredFields(item),
			ObservedVector:     processValueSliceVectorFromCase(item),
		})
	}
	return observations
}

func processValueSliceVectorFromCase(item processValueSliceCase) processValueSliceVector {
	vector := processValueSliceVector{
		AcceptedFindingsAtStart:           item.AcceptedFindingsAtStart,
		MissedAcceptanceCriteriaCount:     item.MissedAcceptanceCriteriaCount,
		UnresolvedAcceptedFindingsAtClose: item.UnresolvedAcceptedFindings,
		ReviewFindingsAfterDone:           item.ReviewFindingsAfterDone,
		ReworkCycles:                      item.ReworkCycles,
		OperatorCorrections:               item.OperatorCorrections,
		CloseRejectionsBeforeValidClose:   item.CloseRejectionsBeforeValidClose,
		DefaultStatusActionLines:          item.DefaultStatusActionLines,
		EscapedGovernedDrift:              item.EscapedGovernedDrift,
		TokensCallsWallclock:              item.TokensCallsWallclock,
	}
	if item.CheckpointHeavy {
		vector.CheckpointHeavyCases = 1
	}
	return vector
}

func processValueSliceVectorAdd(left processValueSliceVector, right processValueSliceVector) processValueSliceVector {
	left.AcceptedFindingsAtStart += right.AcceptedFindingsAtStart
	left.MissedAcceptanceCriteriaCount += right.MissedAcceptanceCriteriaCount
	left.UnresolvedAcceptedFindingsAtClose += right.UnresolvedAcceptedFindingsAtClose
	left.ReviewFindingsAfterDone += right.ReviewFindingsAfterDone
	left.ReworkCycles += right.ReworkCycles
	left.OperatorCorrections += right.OperatorCorrections
	left.CloseRejectionsBeforeValidClose += right.CloseRejectionsBeforeValidClose
	if right.DefaultStatusActionLines > left.DefaultStatusActionLines {
		left.DefaultStatusActionLines = right.DefaultStatusActionLines
	}
	left.EscapedGovernedDrift += right.EscapedGovernedDrift
	left.TokensCallsWallclock.Tokens += right.TokensCallsWallclock.Tokens
	left.TokensCallsWallclock.Calls += right.TokensCallsWallclock.Calls
	left.TokensCallsWallclock.Wallclock += right.TokensCallsWallclock.Wallclock
	left.CheckpointHeavyCases += right.CheckpointHeavyCases
	return left
}

func processValueSlicePolicyInputFrom(pairedCases []processValueSliceCase, allCases []processValueSliceCase, groups []processValueSliceBudgetGroup) processValueSlicePolicyInput {
	input := processValueSlicePolicyInput{
		PairedCases:   len(pairedCases),
		UnpairedCases: len(allCases) - len(pairedCases),
	}
	for _, group := range groups {
		if group.EqualBudgetPair {
			input.EqualBudgetGroups++
		}
		if !group.BudgetComplete {
			input.IncompleteBudgetGroups++
		}
		if !group.ComparisonComplete {
			input.IncompleteComparisonGroups++
		}
	}
	for _, item := range pairedCases {
		switch item.Condition {
		case "haft_methodpack":
			input.PairedHaftCases++
		case "baseline_agent":
			input.PairedBaselineCases++
		}
	}
	return input
}

func processValueSliceMissingnessFrom(cases []processValueSliceCase) processValueSliceMissingnessSummary {
	missing := processValueSliceMissingnessSummary{
		RequiredFieldsForPolicy: append([]string{}, processValueSliceRequiredPolicyFields...),
		PairedCasesChecked:      len(cases),
		MissingByField:          map[string]int{},
	}
	for _, item := range cases {
		fields := processValueSliceMissingRequiredFields(item)
		if len(fields) == 0 {
			continue
		}
		missing.CasesWithMissingRequiredFields++
		for _, field := range fields {
			missing.MissingByField[field]++
		}
	}
	if len(missing.MissingByField) == 0 {
		missing.MissingByField = nil
	}
	return missing
}

func processValueSliceMissingRequiredFields(item processValueSliceCase) []string {
	observed := map[string]struct{}{}
	for _, field := range item.PolicyObservedFields {
		observed[field] = struct{}{}
	}
	missing := make([]string, 0, len(processValueSliceRequiredPolicyFields))
	for _, field := range processValueSliceRequiredPolicyFields {
		if _, ok := observed[field]; !ok {
			missing = append(missing, field)
		}
	}
	return missing
}

func processValueSliceSummaryFrom(report processValueSliceReport, cases []processValueSliceCase) processValueSliceSummary {
	summary := processValueSliceSummary{Cases: len(cases)}
	for _, item := range cases {
		switch item.Condition {
		case "haft_methodpack":
			summary.HaftMethodPack++
		case "baseline_agent":
			summary.BaselineAgent++
		default:
			summary.OtherConditions++
		}
	}
	for _, group := range report.BudgetParity {
		if group.EqualBudgetPair {
			summary.EqualBudgetGroups++
		}
	}
	return summary
}

func processValueSlicePolicyFrom(conditions []processValueSliceCondition, missingness processValueSliceMissingnessSummary) processValueSlicePolicy {
	haft, hasHaft := processValueSliceConditionByName(conditions, "haft_methodpack")
	baseline, hasBaseline := processValueSliceConditionByName(conditions, "baseline_agent")
	if !hasHaft || !hasBaseline {
		return processValueSlicePolicy{
			Label:     "insufficient_data",
			Rationale: []string{"need both haft_methodpack and baseline_agent conditions under equal-budget groups before making a product-value policy call"},
		}
	}
	if missingness.CasesWithMissingRequiredFields > 0 {
		return processValueSlicePolicy{
			Label: "insufficient_data",
			Rationale: []string{
				fmt.Sprintf("%d paired case(s) are missing required observed fields", missingness.CasesWithMissingRequiredFields),
				"absent value-slice metrics are not treated as observed zero",
			},
		}
	}
	hv := haft.ObservedVector
	bv := baseline.ObservedVector
	if hv.CheckpointHeavyCases > 0 &&
		hv.UnresolvedAcceptedFindingsAtClose >= bv.UnresolvedAcceptedFindingsAtClose &&
		processValueSliceRuntimeGreater(hv.TokensCallsWallclock, bv.TokensCallsWallclock) {
		return processValueSlicePolicy{
			Label: "pause_checkpoint",
			Rationale: []string{
				"checkpoint-heavy Haft cases did not improve unresolved accepted findings",
				"runtime burden is higher than baseline",
			},
		}
	}
	if hv.UnresolvedAcceptedFindingsAtClose < bv.UnresolvedAcceptedFindingsAtClose &&
		hv.ReviewFindingsAfterDone <= bv.ReviewFindingsAfterDone &&
		hv.ReworkCycles <= bv.ReworkCycles &&
		hv.OperatorCorrections <= bv.OperatorCorrections &&
		hv.DefaultStatusActionLines <= targetDefaultStatusActionLines {
		return processValueSlicePolicy{
			Label: "continue",
			Rationale: []string{
				"unresolved accepted findings decreased",
				"review findings, rework cycles, and operator corrections did not increase",
				fmt.Sprintf("default status action lines stayed within target <= %d", targetDefaultStatusActionLines),
			},
		}
	}
	if hv.UnresolvedAcceptedFindingsAtClose >= bv.UnresolvedAcceptedFindingsAtClose ||
		hv.ReviewFindingsAfterDone > bv.ReviewFindingsAfterDone ||
		hv.ReworkCycles > bv.ReworkCycles ||
		hv.OperatorCorrections > bv.OperatorCorrections {
		return processValueSlicePolicy{
			Label: "simplify",
			Rationale: []string{
				"Haft condition did not show better closure outcome under the observed vector",
				"process machinery should be simplified or narrowed before adding new primitives",
			},
		}
	}
	return processValueSlicePolicy{
		Label:     "observe_more",
		Rationale: []string{"observed vector is mixed; gather more equal-budget cases before changing product direction"},
	}
}

func processValueSliceNormalizeObservedFields(fields []string) []string {
	observed := map[string]struct{}{}
	for _, field := range fields {
		normalized := processValueSliceNormalizeFieldName(field)
		if _, ok := processValueSliceObservableFields[normalized]; ok {
			observed[normalized] = struct{}{}
		}
	}
	return processValueSliceSortedKeys(observed)
}

func processValueSliceRequiredPolicyFieldSet() map[string]struct{} {
	required := map[string]struct{}{}
	for _, field := range processValueSliceRequiredPolicyFields {
		required[field] = struct{}{}
	}
	return required
}

func processValueSliceRawFieldObserved(field string, raw json.RawMessage) bool {
	if _, ok := processValueSliceRequiredPolicyFieldSet()[field]; ok {
		return processValueSliceRawNumericObserved(raw)
	}
	trimmed := strings.TrimSpace(string(raw))
	return trimmed != "" && trimmed != "null"
}

func processValueSliceRawNumericObserved(raw json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return false
	}
	var number float64
	return json.Unmarshal(raw, &number) == nil
}

func processValueSliceNormalizeFieldName(field string) string {
	return processNormalizeStatus(strings.TrimSpace(field))
}

func processValueSliceSortedKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func processValueSliceConditionByName(conditions []processValueSliceCondition, name string) (processValueSliceCondition, bool) {
	for _, condition := range conditions {
		if condition.Condition == name {
			return condition, true
		}
	}
	return processValueSliceCondition{}, false
}

func processValueSliceRuntimeGreater(left processValueSliceRuntimeVector, right processValueSliceRuntimeVector) bool {
	return left.Tokens > right.Tokens || left.Calls > right.Calls || left.Wallclock > right.Wallclock
}

func processValueSliceBudgetKey(item processValueSliceCase) string {
	parts := []string{
		"comparison=" + processValueSliceComparisonKey(item),
		"model=" + item.Model,
		"host=" + item.Host,
		"tool_budget=" + item.ToolBudget,
		"time_window=" + item.TimeWindow,
	}
	return strings.Join(parts, "|")
}

func processValueSliceComparisonKey(item processValueSliceCase) string {
	if item.ComparisonGroupRef != "" {
		return "comparison_group_ref=" + item.ComparisonGroupRef
	}
	if item.TaskID != "" {
		return "task_id=" + item.TaskID
	}
	if item.TaskTextDigest != "" {
		return "task_text_digest=" + item.TaskTextDigest
	}
	digest := processValueSliceTaskTextDigest(item)
	if digest != "" {
		return "task_text_digest=" + digest
	}
	return "comparison=missing"
}

func processValueSliceTaskTextDigest(item processValueSliceCase) string {
	if item.TaskTextDigest != "" {
		return item.TaskTextDigest
	}
	text := strings.TrimSpace(item.TaskText)
	if text == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(text))
	return fmt.Sprintf("sha256:%x", sum[:8])
}

func processValueSliceBudgetComplete(item processValueSliceCase) bool {
	return item.Model != "" &&
		item.Host != "" &&
		item.ToolBudget != "" &&
		item.TimeWindow != ""
}

func processValueSliceComparisonComplete(item processValueSliceCase) bool {
	return item.ComparisonGroupRef != "" ||
		item.TaskID != "" ||
		item.TaskTextDigest != "" ||
		strings.TrimSpace(item.TaskText) != ""
}

func valueSliceInputRef(inputRef string) string {
	inputRef = strings.TrimSpace(inputRef)
	if inputRef == "" {
		return processValueSliceDefaultInputName
	}
	return inputRef
}

func writeProcessValueSliceText(output io.Writer, report processValueSliceReport) error {
	printf := func(format string, args ...any) error {
		_, err := fmt.Fprintf(output, format, args...)
		return err
	}
	if err := printf("Haft process value slice\n"); err != nil {
		return err
	}
	if err := printf("authority: %s\n", report.Authority); err != nil {
		return err
	}
	if err := printf(
		"summary: cases=%d haft_methodpack=%d baseline_agent=%d equal_budget_groups=%d paired_cases=%d unpaired_cases=%d policy=%s\n",
		report.Summary.Cases,
		report.Summary.HaftMethodPack,
		report.Summary.BaselineAgent,
		report.Summary.EqualBudgetGroups,
		report.PolicyInput.PairedCases,
		report.PolicyInput.UnpairedCases,
		report.Policy.Label,
	); err != nil {
		return err
	}
	for _, rationale := range report.Policy.Rationale {
		if err := printf("- %s\n", rationale); err != nil {
			return err
		}
	}
	return nil
}
