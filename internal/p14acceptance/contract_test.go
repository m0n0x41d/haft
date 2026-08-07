package p14acceptance

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const (
	p14ContractRelativePath  = "internal/p14acceptance/contract.json"
	p14ContractSchema        = "haft.p14.request-oracle-contract/v3"
	p14ContractStatus        = "prepared_not_executed"
	p14ModulePath            = "github.com/m0n0x41d/haft"
	p14PlanRelativePath      = ".context/haft-v9-deterministic-closeout.plan.md"
	p14PlanHeading           = "## P14 — installed candidate and live host acceptance"
	p14PlanSectionRef        = "heading:" + p14PlanHeading
	p14ChecklistPath         = ".context/haft-v9-p14-live-rehearsal-checklist.md"
	p14ChecklistHeading      = "# Haft v9 P14 installed-runtime rehearsal"
	p14TopLevelScenarioCount = 26
)

type requestOracleContract struct {
	Schema          string             `json:"schema"`
	Status          string             `json:"status"`
	PlanRef         string             `json:"plan_ref"`
	PlanSpan        string             `json:"plan_span"`
	ChecklistRef    string             `json:"checklist_ref"`
	ReleaseClaim    bool               `json:"release_claim"`
	ResultSemantics string             `json:"result_semantics"`
	Materialization string             `json:"exact_request_materialization"`
	BindingGroups   []string           `json:"binding_groups"`
	Scenarios       []scenarioContract `json:"scenarios"`
}

type scenarioContract struct {
	ID               string   `json:"id"`
	Family           string   `json:"family"`
	RequestBuilder   string   `json:"request_builder"`
	Surfaces         []string `json:"surfaces"`
	OracleKind       string   `json:"oracle_kind"`
	ExpectedEffect   string   `json:"expected_effect"`
	RequiredBindings []string `json:"required_bindings"`
	LocalOracleTests []string `json:"local_oracle_tests"`
}

type expectedScenarioContract struct {
	Surfaces       []string
	OracleKind     string
	ExpectedEffect string
}

// p14PrivateCarriersPresent reports whether the untracked `.context` carriers
// this contract validates against exist. They are produced during live P14
// preparation and git never carries them, so a fresh checkout has none.
func p14PrivateCarriersPresent(repositoryRoot string) bool {
	for _, relative := range []string{p14ChecklistPath, p14PlanRelativePath} {
		if _, err := os.Stat(filepath.Join(repositoryRoot, relative)); err != nil {
			return false
		}
	}
	return true
}

func TestP14RequestOracleContract(t *testing.T) {
	repositoryRoot, err := p14RepositoryRoot()
	if err == nil && !p14PrivateCarriersPresent(repositoryRoot) {
		t.Skipf("P14 preparation carriers under .context are absent — skipping")
	}
	if err != nil {
		t.Fatal(err)
	}
	contract, raw, err := loadRequestOracleContract(repositoryRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateRequestOracleContract(repositoryRoot, contract); err != nil {
		t.Fatal(err)
	}
	canonical, err := encodeRequestOracleContract(contract)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) == 0 || !json.Valid(canonical) {
		t.Fatal("P14 request/oracle contract has no stable JSON encoding")
	}
	tampered := contract
	tampered.Scenarios = slices.Clone(contract.Scenarios)
	tampered.Scenarios[0].Surfaces = []string{"installed_cli"}
	if err := validateRequestOracleContract(repositoryRoot, tampered); err == nil {
		t.Fatal("P14 contract accepted a removed live runtime surface")
	}
	if len(contract.Scenarios) != p14TopLevelScenarioCount ||
		len(p14InitMatrixPolicies()) != 6 {
		t.Fatalf(
			"P14 matrix = top-level:%d init-subcases:%d",
			len(contract.Scenarios),
			len(p14InitMatrixPolicies()),
		)
	}
}

func loadRequestOracleContract(
	repositoryRoot string,
) (requestOracleContract, []byte, error) {
	path := filepath.Join(repositoryRoot, p14ContractRelativePath)
	raw, err := os.ReadFile(path)
	if err != nil {
		return requestOracleContract{}, nil, fmt.Errorf(
			"read P14 request/oracle contract: %w",
			err,
		)
	}
	reader := bytes.NewReader(raw)
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var contract requestOracleContract
	if err := decoder.Decode(&contract); err != nil {
		return requestOracleContract{}, nil, fmt.Errorf(
			"decode P14 request/oracle contract: %w",
			err,
		)
	}
	var trailing any
	err = decoder.Decode(&trailing)
	if err != io.EOF {
		return requestOracleContract{}, nil, fmt.Errorf(
			"P14 request/oracle contract contains trailing JSON content",
		)
	}
	return contract, raw, nil
}

func encodeRequestOracleContract(
	contract requestOracleContract,
) ([]byte, error) {
	raw, err := json.MarshalIndent(contract, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode P14 request/oracle contract: %w", err)
	}
	raw = append(raw, '\n')
	return raw, nil
}

func validateRequestOracleContract(
	repositoryRoot string,
	contract requestOracleContract,
) error {
	if contract.Schema != p14ContractSchema || contract.Status != p14ContractStatus {
		return fmt.Errorf("P14 request/oracle schema or status is invalid")
	}
	if contract.PlanRef != p14PlanRelativePath ||
		contract.PlanSpan != p14PlanSectionRef ||
		contract.ChecklistRef != p14ChecklistPath {
		return fmt.Errorf("P14 request/oracle plan coordinates are invalid")
	}
	if err := validateP14PlanSpan(
		repositoryRoot,
		contract.PlanRef,
		contract.PlanSpan,
	); err != nil {
		return err
	}
	if err := validateP14CarrierAnchor(
		repositoryRoot,
		contract.ChecklistRef,
		p14ChecklistHeading,
		"checklist",
	); err != nil {
		return err
	}
	if err := validateP14ChecklistScenarioMatrix(
		repositoryRoot,
		contract.ChecklistRef,
	); err != nil {
		return err
	}
	if contract.ReleaseClaim {
		return fmt.Errorf("P14 prepared contract makes a release claim")
	}
	if contract.Materialization != "post_p13_pending" {
		return fmt.Errorf("P14 contract overstates exact request materialization")
	}
	requiredSemantics := []string{
		"design-time execution input",
		"not performed Work",
		"release evidence",
		"authority",
	}
	for _, marker := range requiredSemantics {
		if !strings.Contains(contract.ResultSemantics, marker) {
			return fmt.Errorf("P14 result semantics omit %q", marker)
		}
	}
	expectedBindings := expectedP14BindingGroups()
	if !slices.Equal(contract.BindingGroups, expectedBindings) {
		return fmt.Errorf("P14 binding groups differ from the closed contract")
	}
	expectedOrder := expectedP14ScenarioOrder()
	if len(contract.Scenarios) != p14TopLevelScenarioCount ||
		len(expectedOrder) != p14TopLevelScenarioCount {
		return fmt.Errorf("P14 scenario count = %d", len(contract.Scenarios))
	}
	expected := expectedP14Scenarios()
	seenBuilders := make(map[string]struct{}, len(contract.Scenarios))
	for index, scenario := range contract.Scenarios {
		if scenario.ID != expectedOrder[index] || scenario.Family != scenario.ID {
			return fmt.Errorf("P14 scenario %d identity or order is invalid", index)
		}
		policy, present := expected[scenario.ID]
		if !present {
			return fmt.Errorf("P14 scenario %q is not closed", scenario.ID)
		}
		if !slices.Equal(scenario.Surfaces, policy.Surfaces) ||
			scenario.OracleKind != policy.OracleKind ||
			scenario.ExpectedEffect != policy.ExpectedEffect {
			return fmt.Errorf("P14 scenario %q policy differs", scenario.ID)
		}
		if scenario.RequestBuilder == "" {
			return fmt.Errorf("P14 scenario %q has no request builder", scenario.ID)
		}
		if _, duplicate := seenBuilders[scenario.RequestBuilder]; duplicate {
			return fmt.Errorf("P14 request builder %q is duplicated", scenario.RequestBuilder)
		}
		seenBuilders[scenario.RequestBuilder] = struct{}{}
		if err := validateScenarioBindings(scenario, expectedBindings); err != nil {
			return err
		}
		if err := validateLocalOracleTests(repositoryRoot, scenario); err != nil {
			return err
		}
	}
	return nil
}

func validateP14PlanSpan(
	repositoryRoot string,
	relativePath string,
	span string,
) error {
	if span != p14PlanSectionRef {
		return fmt.Errorf("P14 plan span is not the canonical heading coordinate")
	}
	path := filepath.Join(repositoryRoot, filepath.FromSlash(relativePath))
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read P14 plan carrier: %w", err)
	}
	lines := strings.Split(string(raw), "\n")
	start := -1
	end := len(lines)
	for index, line := range lines {
		if line == p14PlanHeading {
			if start >= 0 {
				return fmt.Errorf("P14 plan heading occurs more than once")
			}
			start = index
			continue
		}
		if start >= 0 &&
			index > start &&
			strings.HasPrefix(line, "## ") {
			end = index
			break
		}
	}
	if start < 0 || end <= start+1 {
		return fmt.Errorf("P14 plan span does not resolve")
	}
	section := strings.Join(lines[start:end], "\n")
	for _, heading := range []string{
		"### D3 — prepare the exact P14 candidate",
		"### D4 — prepare one live restart attempt",
		"### D5 — Human Gate: real host transition",
		"### D6 — execute and verify live P14",
	} {
		if strings.Count(section, heading) != 1 {
			return fmt.Errorf(
				"P14 plan span omits exact subsection %q",
				heading,
			)
		}
	}
	return nil
}

func validateP14ChecklistScenarioMatrix(
	repositoryRoot string,
	relativePath string,
) error {
	path := filepath.Join(repositoryRoot, filepath.FromSlash(relativePath))
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read P14 checklist matrix: %w", err)
	}
	text := string(raw)
	start := strings.Index(text, "## 1. Complete installed scenario matrix")
	end := strings.Index(text, "## 2. Code-only matrix")
	if start < 0 || end <= start {
		return fmt.Errorf("P14 checklist scenario matrix boundaries are invalid")
	}
	section := text[start:end]
	rows := make([]string, 0, p14TopLevelScenarioCount)
	for _, line := range strings.Split(section, "\n") {
		if !strings.HasPrefix(line, "| ") ||
			strings.HasPrefix(line, "| Family |") ||
			strings.HasPrefix(line, "|---") {
			continue
		}
		rows = append(rows, line)
	}
	if len(rows) != p14TopLevelScenarioCount {
		return fmt.Errorf(
			"P14 checklist scenario row count = %d",
			len(rows),
		)
	}
	initRow := ""
	agentMemoryRow := ""
	for _, row := range rows {
		if strings.HasPrefix(row, "| Fresh initialization |") {
			initRow = row
		}
		if strings.HasPrefix(
			row,
			"| Agent typed-memory orientation |",
		) {
			agentMemoryRow = row
		}
	}
	requiredInitSubcases := []string{
		"`haft init --core-only`",
		"`haft init --claude`",
		"`haft init --codex`",
		"`haft init --codex --mcp-only`",
		"`haft init --agents`",
		"`haft init --all`",
		"not a seventh subcase",
	}
	for _, required := range requiredInitSubcases {
		if !strings.Contains(initRow, required) {
			return fmt.Errorf(
				"P14 checklist init row omits %q",
				required,
			)
		}
	}
	for _, required := range []string{
		"nine profile fixtures",
		"TypeScript",
		"Python",
		"Rust",
		"Zig/Elixir/Dart manual fallback",
		"docs-only",
		"mixed software/model",
		"empty manual fallback",
		"source evidence, not an installed observation",
	} {
		if !strings.Contains(initRow, required) {
			return fmt.Errorf(
				"P14 checklist init row omits supplemental profile marker %q",
				required,
			)
		}
	}
	for _, required := range []string{
		"known_absent",
		"explicit-save prompt",
		"`haft_entity.establish`",
		"unchanged request and idempotency key",
		"canonical `U.EntityRef`",
		"exactly one graph-revision advance",
		"byte-composable `next_read`",
		"non-authorizing interpretations",
		"no raw TypeEnv/basis/ref-kind/authority/change-set",
	} {
		if !strings.Contains(agentMemoryRow, required) {
			return fmt.Errorf(
				"P14 checklist agent-memory row omits %q",
				required,
			)
		}
	}
	if !strings.Contains(section, "Codex-specific") ||
		!strings.Contains(section, "separate actual-host Claude proof") {
		return fmt.Errorf(
			"P14 checklist overstates actual-host coverage",
		)
	}
	requiredExecutableProof := []string{
		"raw JSON-RPC `initialize` / `tools/list` capture",
		"append-only Codex JSONL session",
		"selected line digests",
		"haft.p14.claude-host-proof-request/v2",
		"`mcp_instructions_delta` / `deferred_tools_delta`",
		"status → bounded onboarding status → status",
		"first status pair must bracket the private live-MCP receipt",
		"server-emitted runtime line with",
		"exact receipt PID, start time, and physical executable path",
		"separate protocol-byte proof",
		"never imported Codex evidence",
		"unique canonical main-session JSONL",
		"TestP14CaptureActualClaudeHostProof",
		"haft.p14.installed-observation-finalization-request/v2",
		"claude_host_proof_path",
		"TestP14FinalizeInstalledObservationCarrier",
	}
	for _, marker := range requiredExecutableProof {
		if !strings.Contains(text, marker) {
			return fmt.Errorf(
				"P14 checklist omits executable host-proof marker %q",
				marker,
			)
		}
	}
	if strings.Contains(
		text,
		"--plan-path .context/haft-v9-typed-memory-e2e-master-plan.md",
	) || strings.Count(
		text,
		"--plan-path "+p14PlanRelativePath,
	) != 2 {
		return fmt.Errorf(
			"P14 checkpoint commands do not bind the canonical closeout carrier",
		)
	}
	return nil
}

func validateP14CarrierAnchor(
	repositoryRoot string,
	relativePath string,
	heading string,
	label string,
) error {
	path := filepath.Join(repositoryRoot, filepath.FromSlash(relativePath))
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read P14 %s carrier: %w", label, err)
	}
	occurrences := bytes.Count(raw, []byte(heading))
	if occurrences != 1 {
		return fmt.Errorf(
			"P14 %s carrier heading occurs %d times",
			label,
			occurrences,
		)
	}
	return nil
}

func validateScenarioBindings(
	scenario scenarioContract,
	allowed []string,
) error {
	if len(scenario.RequiredBindings) == 0 ||
		!slices.Contains(scenario.RequiredBindings, "candidate_basis") ||
		!slices.Contains(scenario.RequiredBindings, "p13_evidence") {
		return fmt.Errorf("P14 scenario %q omits its frozen candidate or P13 basis", scenario.ID)
	}
	seen := make(map[string]struct{}, len(scenario.RequiredBindings))
	for _, binding := range scenario.RequiredBindings {
		if !slices.Contains(allowed, binding) {
			return fmt.Errorf("P14 scenario %q has unknown binding %q", scenario.ID, binding)
		}
		if _, duplicate := seen[binding]; duplicate {
			return fmt.Errorf("P14 scenario %q repeats binding %q", scenario.ID, binding)
		}
		seen[binding] = struct{}{}
	}
	return nil
}

func validateLocalOracleTests(
	repositoryRoot string,
	scenario scenarioContract,
) error {
	if len(scenario.LocalOracleTests) == 0 {
		return fmt.Errorf("P14 scenario %q has no local oracle test", scenario.ID)
	}
	for _, anchor := range scenario.LocalOracleTests {
		parts := strings.Split(anchor, "::")
		if len(parts) != 2 || !strings.HasPrefix(parts[0], p14ModulePath+"/") {
			return fmt.Errorf("P14 scenario %q has invalid oracle anchor %q", scenario.ID, anchor)
		}
		relativePackage := strings.TrimPrefix(parts[0], p14ModulePath+"/")
		packageDirectory := filepath.Join(repositoryRoot, filepath.FromSlash(relativePackage))
		present, err := packageDefinesTest(packageDirectory, parts[1])
		if err != nil {
			return err
		}
		if !present {
			return fmt.Errorf("P14 oracle test %q does not exist", anchor)
		}
	}
	return nil
}

func packageDefinesTest(directory string, testName string) (bool, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return false, fmt.Errorf("read P14 oracle package %s: %w", directory, err)
	}
	fileSet := token.NewFileSet()
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(directory, entry.Name())
		parsed, parseErr := parser.ParseFile(fileSet, path, nil, 0)
		if parseErr != nil {
			return false, fmt.Errorf("parse P14 oracle file %s: %w", path, parseErr)
		}
		for _, declaration := range parsed.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if ok && function.Name.Name == testName {
				return true, nil
			}
		}
	}
	return false, nil
}

func expectedP14BindingGroups() []string {
	return []string{
		"candidate_basis",
		"p13_evidence",
		"selected_project_basis",
		"golden_memory_fixture",
		"init_matrix",
		"identifier_fixture",
		"restart_checkpoint",
		"live_mcp_process",
	}
}

func expectedP14ScenarioOrder() []string {
	return []string{
		"runtime_identity",
		"fpf_query_projection",
		"fresh_initialization",
		"positive_typed_write",
		"invalid",
		"underdetermined",
		"authority_rejection",
		"concurrency_idempotency",
		"existing_record_backfill",
		"legacy_read",
		"exact_profile_neighborhood",
		"unknown_eoc",
		"known_eoc_recall",
		"facet_coverage",
		"retry_required",
		"read_affordance",
		"identifier_namespace",
		"spec_section_read_protocol",
		"host_resume",
		"loop_cleanup",
		"code_graph_exact_explore",
		"code_graph_concern_explore",
		"code_graph_ambiguous_concern",
		"code_graph_coverage_diagnostic",
		"agent_code_graph_orientation",
		"agent_typed_memory_orientation",
	}
}

func expectedP14Scenarios() map[string]expectedScenarioContract {
	cliAndMCP := []string{"installed_cli", "live_mcp"}
	return map[string]expectedScenarioContract{
		"runtime_identity": {
			Surfaces:       []string{"installed_cli", "live_mcp", "host_process"},
			OracleKind:     "live_predicate",
			ExpectedEffect: "none",
		},
		"fpf_query_projection": {
			Surfaces:       slices.Clone(cliAndMCP),
			OracleKind:     "normalized_digest",
			ExpectedEffect: "none",
		},
		"fresh_initialization": {
			Surfaces:       []string{"installed_cli"},
			OracleKind:     "normalized_digest",
			ExpectedEffect: "fixture_write",
		},
		"positive_typed_write": {
			Surfaces:       slices.Clone(cliAndMCP),
			OracleKind:     "normalized_digest",
			ExpectedEffect: "fixture_semantic_write",
		},
		"invalid": {
			Surfaces:       slices.Clone(cliAndMCP),
			OracleKind:     "normalized_digest",
			ExpectedEffect: "none",
		},
		"underdetermined": {
			Surfaces:       slices.Clone(cliAndMCP),
			OracleKind:     "normalized_digest",
			ExpectedEffect: "none",
		},
		"authority_rejection": {
			Surfaces:       slices.Clone(cliAndMCP),
			OracleKind:     "normalized_digest",
			ExpectedEffect: "none",
		},
		"concurrency_idempotency": {
			Surfaces:       slices.Clone(cliAndMCP),
			OracleKind:     "normalized_digest",
			ExpectedEffect: "fixture_semantic_write",
		},
		"existing_record_backfill": {
			Surfaces:       []string{"installed_cli"},
			OracleKind:     "normalized_digest",
			ExpectedEffect: "fixture_semantic_write",
		},
		"legacy_read": {
			Surfaces:       slices.Clone(cliAndMCP),
			OracleKind:     "normalized_digest",
			ExpectedEffect: "none",
		},
		"exact_profile_neighborhood": {
			Surfaces:       slices.Clone(cliAndMCP),
			OracleKind:     "normalized_digest",
			ExpectedEffect: "none",
		},
		"unknown_eoc": {
			Surfaces:       slices.Clone(cliAndMCP),
			OracleKind:     "normalized_digest",
			ExpectedEffect: "none",
		},
		"known_eoc_recall": {
			Surfaces:       slices.Clone(cliAndMCP),
			OracleKind:     "normalized_digest",
			ExpectedEffect: "none",
		},
		"facet_coverage": {
			Surfaces:       slices.Clone(cliAndMCP),
			OracleKind:     "normalized_digest",
			ExpectedEffect: "none",
		},
		"retry_required": {
			Surfaces:       slices.Clone(cliAndMCP),
			OracleKind:     "normalized_digest",
			ExpectedEffect: "none",
		},
		"read_affordance": {
			Surfaces:       slices.Clone(cliAndMCP),
			OracleKind:     "normalized_digest",
			ExpectedEffect: "none",
		},
		"identifier_namespace": {
			Surfaces:       []string{"live_mcp"},
			OracleKind:     "normalized_digest",
			ExpectedEffect: "none",
		},
		"spec_section_read_protocol": {
			Surfaces:       []string{"live_mcp"},
			OracleKind:     "normalized_digest",
			ExpectedEffect: "none",
		},
		"host_resume": {
			Surfaces:       []string{"host_process"},
			OracleKind:     "live_predicate",
			ExpectedEffect: "host_process_transition",
		},
		"loop_cleanup": {
			Surfaces:       []string{"host_process"},
			OracleKind:     "live_predicate",
			ExpectedEffect: "host_process_transition",
		},
		"code_graph_exact_explore": {
			Surfaces:       slices.Clone(cliAndMCP),
			OracleKind:     "normalized_digest",
			ExpectedEffect: "none",
		},
		"code_graph_concern_explore": {
			Surfaces:       slices.Clone(cliAndMCP),
			OracleKind:     "normalized_digest",
			ExpectedEffect: "none",
		},
		"code_graph_ambiguous_concern": {
			Surfaces:       slices.Clone(cliAndMCP),
			OracleKind:     "normalized_digest",
			ExpectedEffect: "none",
		},
		"code_graph_coverage_diagnostic": {
			Surfaces:       slices.Clone(cliAndMCP),
			OracleKind:     "normalized_digest",
			ExpectedEffect: "none",
		},
		"agent_code_graph_orientation": {
			Surfaces: []string{
				"host_process",
				"installed_cli",
				"live_mcp",
			},
			OracleKind:     "live_predicate",
			ExpectedEffect: "host_process_observation",
		},
		"agent_typed_memory_orientation": {
			Surfaces: []string{
				"host_process",
				"installed_cli",
				"live_mcp",
			},
			OracleKind:     "live_predicate",
			ExpectedEffect: "host_process_observation_and_non_binding_entity_establishment",
		},
	}
}

func p14RepositoryRoot() (string, error) {
	workingDirectory, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("read P14 working directory: %w", err)
	}
	current := workingDirectory
	for {
		moduleFile := filepath.Join(current, "go.mod")
		info, statErr := os.Stat(moduleFile)
		if statErr == nil && info.Mode().IsRegular() {
			canonical, evalErr := filepath.EvalSymlinks(current)
			if evalErr != nil {
				return "", fmt.Errorf("canonicalize P14 repository root: %w", evalErr)
			}
			return canonical, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("P14 repository root with go.mod was not found")
		}
		current = parent
	}
}
