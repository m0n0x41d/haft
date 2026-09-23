package check

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/core/carrier"
	"github.com/m0n0x41d/haft/internal/core/code"
)

type probeFixture struct {
	Case     string            `json:"case"`
	Selector string            `json:"selector"`
	ExitCode int               `json:"exit_code"`
	Command  []string          `json:"command"`
	Sources  map[string][]byte `json:"sources"`
	Stdout   []byte            `json:"stdout"`
	Stderr   []byte            `json:"stderr"`
	Scope    string            `json:"scope"`
}

func loadProbe(t *testing.T, name string) ObservationInput {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "probes", name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture probeFixture
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	idx, err := code.IndexFromFiles(fixture.Sources, code.Config{GOOS: "darwin", GOARCH: "arm64", Toolchain: "go1.25.8", IncludeTests: true})
	if err != nil {
		t.Fatal(err)
	}
	seed := int64(23)
	selector := Selector{Package: "example.test/orders", Test: fixture.Selector}
	basis := Basis{Claim: "spec-20260923-12345678@" + carrier.Digest([]byte("adapter-test claim: cancellation preserves total")) + "#preserves-total", Code: idx.Basis, Check: carrier.Digest(fixture.Sources["order_test.go"]), Dependencies: carrier.Digest(append(bytes.Clone(fixture.Sources["policy.go"]), fixture.Sources["go.mod"]...)), Conditions: []string{"direct domain cancellation; finite quick.Check sample; no transport/persistence coverage"}, Environment: map[string]string{"toolchain": "go1.25.8", "goos": "darwin", "goarch": "arm64"}, Seed: &seed}
	contract := Contract{Ref: "sym:order_test.go::" + fixture.Selector, Selector: selector, Basis: basis, Scope: fixture.Scope, FailureContract: "fixture order oracle: testing/quick Check error reported by t.Fatal, reviewed against order_test.go", FailurePattern: `(?m)^\s+order_test\.go:\d+: #\d+: failed on input `}
	return ObservationInput{Expected: contract, Observed: Run{Started: true, ExitCode: &fixture.ExitCode, Command: fixture.Command, Selector: selector, Basis: basis, Stdout: fixture.Stdout, Stderr: fixture.Stderr}}
}
func TestActualRunnerProbeClassification(t *testing.T) {
	cases := []struct{ name, status, reason string }{{"property_pass", Passed, "selected_test_passed"}, {"scenario_pass", Passed, "selected_test_passed"}, {"assertion_failure", AssertionFailure, "declared_oracle_failure_observed"}, {"dependency_failure", AssertionFailure, "declared_oracle_failure_observed"}, {"irrelevant_pass", Unattributable, "selector_mismatch"}, {"zero_tests", NotRun, "zero_tests"}, {"skipped", Skipped, "selected_test_skipped"}, {"environment_failure", EnvironmentFailure, "build_failure"}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := loadProbe(t, tc.name)
			if tc.name == "irrelevant_pass" {
				input.Expected.Selector.Test = "TestCancelPreservesTotal"
				input.Expected.Ref = "sym:order_test.go::TestCancelPreservesTotal"
			}
			result := GoTestObservation(input)
			if result.Status != tc.status || result.ReasonCode != tc.reason {
				t.Fatalf("got %s/%s want %s/%s: %#v", result.Status, result.ReasonCode, tc.status, tc.reason, result.Diagnostics)
			}
			if !bytes.Equal(result.Input.Observed.Stdout, input.Observed.Stdout) || len(result.RawEvents) == 0 {
				t.Fatal("raw observation was not retained")
			}
			if (tc.name == "assertion_failure" || tc.name == "dependency_failure") && !bytes.Contains(result.Input.Observed.Stdout, []byte("failed on input")) {
				t.Fatal("counterexample absent")
			}
		})
	}
}

func TestAuthoredCheckAdapterSelectorIsPreserved(t *testing.T) {
	for _, kind := range []string{"test", "pbt"} {
		input := loadProbe(t, "property_pass")
		input.Expected.Ref = kind + ":order_test.go::TestCancelPreservesTotal"
		result := GoTestObservation(input)
		if result.Status != Passed || result.Input.Expected.Ref != input.Expected.Ref {
			t.Fatalf("adapter selector: %s/%s", result.Status, result.ReasonCode)
		}
	}
	input := loadProbe(t, "property_pass")
	input.Expected.Ref = "pbt:order_test.go::TestCurrency"
	if GoTestObservation(input).Status != Unattributable {
		t.Fatal("different oracle symbol accepted")
	}
}
func TestPassCannotCrossExactBasesOrSelector(t *testing.T) {
	mutations := []struct {
		name   string
		mutate func(*ObservationInput)
	}{
		{"claim", func(i *ObservationInput) {
			i.Expected.Basis.Claim = strings.Replace(i.Expected.Basis.Claim, "#preserves-total", "#new-claim", 1)
		}},
		{"oracle", func(i *ObservationInput) { i.Expected.Basis.Check = carrier.Digest([]byte("new oracle")) }},
		{"code", func(i *ObservationInput) { i.Expected.Basis.Code = carrier.Digest([]byte("new raw code bytes")) }},
		{"dependencies", func(i *ObservationInput) { i.Expected.Basis.Dependencies = carrier.Digest([]byte("changed sibling")) }},
		{"conditions", func(i *ObservationInput) { i.Expected.Basis.Conditions = []string{"changed assumptions"} }},
		{"seed", func(i *ObservationInput) { seed := int64(42); i.Expected.Basis.Seed = &seed }},
		{"environment", func(i *ObservationInput) { i.Expected.Basis.Environment = map[string]string{"toolchain": "other"} }},
	}
	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			input := loadProbe(t, "property_pass")
			mutation.mutate(&input)
			result := GoTestObservation(input)
			if result.Status != Unattributable || result.ReasonCode != "basis_mismatch" {
				t.Fatalf("old pass accepted: %#v", result)
			}
		})
	}
	input := loadProbe(t, "property_pass")
	input.Expected.Basis.Claim = "spec:orders#preserves-total"
	if r := GoTestObservation(input); r.Status != Unattributable || r.ReasonCode != "basis_or_contract_absent" {
		t.Fatal("floating claim target accepted")
	}
}
func TestNoGuessedAssertionAndFailureRemainsAfterLaterPass(t *testing.T) {
	input := loadProbe(t, "assertion_failure")
	input.Expected.FailurePattern = ""
	input.Expected.FailureContract = ""
	unknown := GoTestObservation(input)
	if unknown.Status != Unattributable || unknown.ReasonCode != "unknown_cause" {
		t.Fatal("generic fail guessed to be assertion")
	}
	failed := GoTestObservation(loadProbe(t, "assertion_failure"))
	passed := GoTestObservation(loadProbe(t, "property_pass"))
	if failed.ID == passed.ID || failed.Status != AssertionFailure || passed.Status != Passed {
		t.Fatal("distinct observations collapsed")
	}
	input = loadProbe(t, "assertion_failure")
	input.Expected.FailurePattern = "arbitrary"
	input.Expected.FailureContract = ""
	if GoTestObservation(input).ReasonCode != "basis_or_contract_absent" {
		t.Fatal("uncontracted arbitrary marker accepted")
	}
}
func TestIncompleteAndContradictoryRunnerOutput(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*ObservationInput)
		reason string
	}{
		{"not_started", func(i *ObservationInput) {
			i.Observed.Started = false
			i.Observed.ExitCode = nil
			i.Observed.Stdout = nil
		}, "runner_not_started"},
		{"contradictory_started", func(i *ObservationInput) { i.Observed.Started = false }, "contradictory_run_state"},
		{"no_exit", func(i *ObservationInput) { i.Observed.ExitCode = nil }, "exit_status_absent"},
		{"no_json", func(i *ObservationInput) { i.Observed.Stdout = []byte("PASS\n") }, "malformed_output"},
		{"empty", func(i *ObservationInput) { i.Observed.Stdout = nil }, "unknown_cause"},
		{"duplicate", func(i *ObservationInput) {
			i.Observed.Stdout = []byte(`{"Action":"run","Action":"pass","Package":"example.test/orders","Test":"TestCancelPreservesTotal"}`)
		}, "malformed_output"},
		{"partial", func(i *ObservationInput) {
			lines := bytes.Split(i.Observed.Stdout, []byte{'\n'})
			i.Observed.Stdout = bytes.Join(lines[:len(lines)-2], []byte{'\n'})
		}, "incomplete_output"},
		{"exit", func(i *ObservationInput) { exit := 1; i.Observed.ExitCode = &exit }, "contradictory_exit"},
		{"command", func(i *ObservationInput) { i.Observed.Command = []string{"go", "test", "-json", "./..."} }, "runner_command_mismatch"},
		{"cached", func(i *ObservationInput) {
			i.Observed.Command = []string{"go", "test", "-json", "-run", ExactRunPattern(i.Observed.Selector.Test), "./..."}
		}, "runner_command_mismatch"},
		{"last_count_wins", func(i *ObservationInput) { i.Observed.Command = append(i.Observed.Command, "-count=0") }, "runner_command_mismatch"},
		{"test_args", func(i *ObservationInput) { i.Observed.Command = append(i.Observed.Command, "-args", "-test.run=Other") }, "runner_command_mismatch"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := loadProbe(t, "property_pass")
			tc.mutate(&input)
			result := GoTestObservation(input)
			if result.ReasonCode != tc.reason || result.Status == Passed {
				t.Fatalf("got %s/%s expected %s", result.Status, result.ReasonCode, tc.reason)
			}
			if _, err := json.Marshal(result); err != nil {
				t.Fatalf("malformed raw input broke output serialization: %v", err)
			}
		})
	}
}
func TestStableSubtestAndObservationCopies(t *testing.T) {
	input := loadProbe(t, "scenario_pass")
	input.Expected.Selector.Test = "TestCancelStates/new"
	input.Expected.Ref = "sym:order_test.go::TestCancelStates"
	input.Observed.Selector = input.Expected.Selector
	input.Observed.Command = []string{"go", "test", "-count=1", "-json", "-run", ExactRunPattern(input.Observed.Selector.Test), "./..."}
	result := GoTestObservation(input)
	if result.Status != Passed {
		t.Fatalf("exact subtest: %s/%s", result.Status, result.ReasonCode)
	}
	for _, event := range result.SelectedEvents {
		if event.Test != "TestCancelStates/new" {
			t.Fatal("other scenario selected")
		}
	}
	saved := bytes.Clone(result.Input.Observed.Stdout)
	input.Observed.Stdout[0] = 'x'
	input.Expected.Basis.Environment["toolchain"] = "other"
	if !bytes.Equal(saved, result.Input.Observed.Stdout) || result.Input.Expected.Basis.Environment["toolchain"] == "other" {
		t.Fatal("input aliases escaped pure adapter")
	}
}
