// Package check interprets captured runner observations. It performs no IO,
// starts no runner, and does not decide whether an oracle expresses a claim.
package check

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"

	"github.com/m0n0x41d/haft/internal/core/carrier"
)

const AdapterVersion = "haft.go-test-observation/1"
const (
	Passed             = "passed"
	AssertionFailure   = "assertion_failure"
	Skipped            = "skipped"
	NotRun             = "not_run"
	EnvironmentFailure = "environment_failure"
	Unattributable     = "unattributable"
)

type Selector struct {
	Package string `json:"package"`
	Test    string `json:"test"`
}
type Basis struct {
	Claim        string            `json:"claim"`
	Code         string            `json:"code"`
	Check        string            `json:"check"`
	Dependencies string            `json:"dependencies"`
	Conditions   []string          `json:"conditions"`
	Environment  map[string]string `json:"environment"`
	Seed         *int64            `json:"seed,omitempty"`
}
type Contract struct {
	Ref      string   `json:"ref"`
	Selector Selector `json:"selector"`
	Basis    Basis    `json:"basis"`
	Scope    string   `json:"scope"`
	// FailurePattern is meaningful only under this explicit reviewed adapter/
	// oracle contract. A generic Go fail event cannot establish assertion cause.
	FailureContract string `json:"failure_contract,omitempty"`
	FailurePattern  string `json:"failure_pattern,omitempty"`
}
type Run struct {
	Started  bool     `json:"started"`
	ExitCode *int     `json:"exit_code"`
	Command  []string `json:"command"`
	Selector Selector `json:"selector"`
	Basis    Basis    `json:"basis"`
	Stdout   []byte   `json:"stdout_base64"`
	Stderr   []byte   `json:"stderr_base64"`
	// These are observations supplied by the process adapter, not guesses from
	// arbitrary stderr: start_failed, timeout, or signal.
	EnvironmentFailure string `json:"environment_failure,omitempty"`
}
type ObservationInput struct {
	Expected Contract `json:"expected"`
	Observed Run      `json:"observed"`
}
type Event struct {
	Action      string `json:"Action"`
	Package     string `json:"Package,omitempty"`
	ImportPath  string `json:"ImportPath,omitempty"`
	Test        string `json:"Test,omitempty"`
	Output      string `json:"Output,omitempty"`
	FailedBuild string `json:"FailedBuild,omitempty"`
}
type Observation struct {
	Adapter        string           `json:"adapter"`
	ID             string           `json:"id"`
	Status         string           `json:"status"`
	ReasonCode     string           `json:"reason_code"`
	Input          ObservationInput `json:"input"`
	RawEvents      [][]byte         `json:"raw_events_base64"`
	SelectedEvents []Event          `json:"selected_events"`
	Diagnostics    []string         `json:"diagnostics,omitempty"`
	Limits         []string         `json:"limits"`
}

// ExactRunPattern preserves a stable subtest path by anchoring each Go -run
// component. Root-only selectors include their descendant cases in the scope.
func ExactRunPattern(test string) string {
	parts := strings.Split(test, "/")
	for i, p := range parts {
		parts[i] = "^" + regexp.QuoteMeta(p) + "$"
	}
	return strings.Join(parts, "/")
}

func GoTestObservation(input ObservationInput) Observation {
	// A round trip copies all raw bytes, selectors, maps, seed and exit pointers.
	encoded, _ := json.Marshal(input)
	var copied ObservationInput
	_ = json.Unmarshal(encoded, &copied)
	r := Observation{Adapter: AdapterVersion, ID: carrier.Digest(append([]byte(AdapterVersion+"\n"), encoded...)), Input: copied, Status: Unattributable, ReasonCode: "unknown_cause", RawEvents: [][]byte{}, SelectedEvents: []Event{}, Limits: []string{"Runner observation applies only to the recorded selector, exact bases and scope; oracle fitness requires content review", "A finite property run is not a proof for all inputs"}}
	expected, run := copied.Expected, copied.Observed
	finish := func(status, reason string) Observation { r.Status = status; r.ReasonCode = reason; return r }
	// Preserve every raw event even when attribution will later be rejected.
	events := []Event{}
	malformed := false
	for _, line := range bytes.Split(run.Stdout, []byte{'\n'}) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		r.RawEvents = append(r.RawEvents, bytes.Clone(line))
		event, err := decodeEvent(line)
		if err != nil {
			malformed = true
			r.Diagnostics = append(r.Diagnostics, err.Error())
			continue
		}
		events = append(events, event)
		if event.Package == expected.Selector.Package && (event.Test == expected.Selector.Test || strings.HasPrefix(event.Test, expected.Selector.Test+"/")) {
			r.SelectedEvents = append(r.SelectedEvents, event)
		}
	}
	if run.EnvironmentFailure != "" {
		switch run.EnvironmentFailure {
		case "start_failed", "timeout", "signal":
			return finish(EnvironmentFailure, run.EnvironmentFailure)
		default:
			return finish(Unattributable, "unknown_environment_failure")
		}
	}
	if !run.Started {
		if run.ExitCode != nil || len(bytes.TrimSpace(run.Stdout)) != 0 {
			return finish(Unattributable, "contradictory_run_state")
		}
		return finish(NotRun, "runner_not_started")
	}
	if run.ExitCode == nil {
		return finish(Unattributable, "exit_status_absent")
	}
	if err := validateContract(expected); err != nil {
		r.Diagnostics = append(r.Diagnostics, err.Error())
		return finish(Unattributable, "basis_or_contract_absent")
	}
	if run.Selector != expected.Selector {
		return finish(Unattributable, "selector_mismatch")
	}
	if !reflect.DeepEqual(run.Basis, expected.Basis) {
		return finish(Unattributable, "basis_mismatch")
	}
	if !exactCommand(run.Command, expected) {
		return finish(Unattributable, "runner_command_mismatch")
	}
	if malformed {
		return finish(Unattributable, "malformed_output")
	}
	selectedRun := false
	selectedEnd := ""
	packageEnd := ""
	buildFailed := false
	anyTest := false
	selectedSkipped := false
	selectedFailed := false
	selectedOutput := ""
	running := map[string]bool{}
	ended := map[string]bool{}
	for _, event := range events {
		switch event.Action {
		case "start", "run", "pause", "cont", "pass", "bench", "fail", "output", "skip", "build-output", "build-fail":
		default:
			return finish(Unattributable, "unsupported_runner_event")
		}
		if event.Action == "build-fail" && (event.ImportPath == run.Selector.Package || strings.HasPrefix(event.ImportPath, run.Selector.Package+" [")) {
			buildFailed = true
		}
		if event.Package != run.Selector.Package {
			continue
		}
		if event.FailedBuild != "" {
			buildFailed = true
		}
		if event.Test == "" {
			if event.Action == "pass" || event.Action == "fail" || event.Action == "skip" {
				if packageEnd != "" {
					return finish(Unattributable, "contradictory_events")
				}
				packageEnd = event.Action
			}
			continue
		}
		if packageEnd != "" {
			return finish(Unattributable, "contradictory_events")
		}
		if event.Action == "run" {
			anyTest = true
		}
		selected := event.Test == run.Selector.Test || strings.HasPrefix(event.Test, run.Selector.Test+"/")
		if !selected {
			continue
		}
		if event.Action == "run" {
			if running[event.Test] || ended[event.Test] {
				return finish(Unattributable, "repeated_test_execution")
			}
			running[event.Test] = true
			if event.Test == run.Selector.Test {
				selectedRun = true
			}
		}
		if event.Action == "output" {
			selectedOutput += event.Output
		}
		if event.Action == "pass" || event.Action == "fail" || event.Action == "skip" {
			if !running[event.Test] || ended[event.Test] {
				return finish(Unattributable, "incomplete_test_events")
			}
			ended[event.Test] = true
			if event.Action == "skip" {
				selectedSkipped = true
			}
			if event.Action == "fail" {
				selectedFailed = true
			}
			if event.Test == run.Selector.Test {
				selectedEnd = event.Action
			}
		}
	}
	if buildFailed {
		if *run.ExitCode == 0 {
			return finish(Unattributable, "contradictory_exit")
		}
		return finish(EnvironmentFailure, "build_failure")
	}
	if !selectedRun {
		if *run.ExitCode == 0 && (packageEnd == "pass" || packageEnd == "skip") && !anyTest {
			return finish(NotRun, "zero_tests")
		}
		if anyTest {
			return finish(Unattributable, "selected_test_not_observed")
		}
		return finish(Unattributable, "unknown_cause")
	}
	for test := range running {
		if !ended[test] {
			return finish(Unattributable, "incomplete_test_events")
		}
	}
	if selectedEnd == "" || packageEnd == "" {
		return finish(Unattributable, "incomplete_output")
	}
	switch selectedEnd {
	case "pass":
		if selectedFailed {
			return finish(Unattributable, "contradictory_events")
		}
		if *run.ExitCode != 0 || packageEnd != "pass" {
			return finish(Unattributable, "contradictory_exit")
		}
		if selectedSkipped {
			return finish(Skipped, "selected_scope_partially_skipped")
		}
		return finish(Passed, "selected_test_passed")
	case "skip":
		if *run.ExitCode != 0 || packageEnd != "pass" {
			return finish(Unattributable, "contradictory_exit")
		}
		return finish(Skipped, "selected_test_skipped")
	case "fail":
		if *run.ExitCode == 0 || packageEnd != "fail" {
			return finish(Unattributable, "contradictory_exit")
		}
		if expected.FailureContract != "" && expected.FailurePattern != "" {
			pattern, err := regexp.Compile(expected.FailurePattern)
			if err == nil && pattern.MatchString(selectedOutput) {
				return finish(AssertionFailure, "declared_oracle_failure_observed")
			}
		}
	}
	return finish(Unattributable, "unknown_cause")
}

func validateContract(c Contract) error {
	ref, err := carrier.ParseRef(c.Basis.Claim)
	if err != nil || !ref.Pinned() || ref.ClaimID == "" {
		return fmt.Errorf("claim must identify an exact pinned claim edition")
	}
	if !carrier.ValidDigest(c.Basis.Code) || !carrier.ValidDigest(c.Basis.Check) || !carrier.ValidDigest(c.Basis.Dependencies) {
		return fmt.Errorf("exact code/check/dependency digests required")
	}
	if len(c.Basis.Conditions) == 0 || len(c.Basis.Environment) == 0 {
		return fmt.Errorf("declared conditions and environment basis required")
	}
	if tags, exists := c.Basis.Environment["build_tags"]; !exists || !validBuildTagsValue(tags) {
		return fmt.Errorf("explicit canonical build_tags environment basis required")
	}
	if c.Scope == "" || c.Selector.Package == "" || c.Selector.Test == "" || strings.ContainsAny(c.Selector.Test, "\x00\r\n") {
		return fmt.Errorf("explicit scope and exact package/test selector required")
	}
	oracleRef := c.Ref
	if kind, value, _ := strings.Cut(c.Ref, ":"); kind == "test" || kind == "pbt" {
		oracleRef = "sym:" + value
	}
	if err := carrier.ValidateSelector(oracleRef); err != nil {
		return err
	}
	if strings.HasPrefix(oracleRef, "sym:") {
		_, name, _ := strings.Cut(oracleRef, "::")
		name, _, _ = strings.Cut(name, "/")
		base, _, _ := strings.Cut(c.Selector.Test, "/")
		if name != base {
			return fmt.Errorf("oracle symbol and exact test selector disagree")
		}
	}
	if c.FailurePattern != "" {
		if c.FailureContract == "" {
			return fmt.Errorf("failure marker needs an explicit adapter/oracle contract")
		}
		if _, err := regexp.Compile(c.FailurePattern); err != nil {
			return err
		}
	}
	return nil
}

// GoTestCommand constructs the narrow uncached command supported by this
// adapter. Extra runner/build flags require a separately captured adapter basis.
func GoTestCommand(selector Selector, tags []string) ([]string, error) {
	value := strings.Join(tags, ",")
	if !validBuildTagsValue(value) {
		return nil, fmt.Errorf("unsupported build tag names")
	}
	for _, tag := range tags {
		if tag == "" || strings.Contains(tag, ",") {
			return nil, fmt.Errorf("each build tag must be one nonempty name")
		}
	}
	if selector.Package == "" || strings.HasPrefix(selector.Package, "-") || strings.ContainsAny(selector.Package, " \t\r\n") || strings.Contains(selector.Package, "...") {
		return nil, fmt.Errorf("one exact package identity is required")
	}
	command := []string{"go", "test", "-json", "-count=1", "-run", ExactRunPattern(selector.Test)}
	if value != "" {
		command = append(command, "-tags="+value)
	}
	return append(command, selector.Package), nil
}

func validBuildTagsValue(value string) bool {
	if value == "" {
		return true
	}
	previous := ""
	for _, tag := range strings.Split(value, ",") {
		if tag == "" || tag <= previous {
			return false
		}
		for _, r := range tag {
			if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '.') {
				return false
			}
		}
		previous = tag
	}
	return true
}

func exactCommand(command []string, expected Contract) bool {
	if len(command) < 3 || filepath.Base(command[0]) != "go" || command[1] != "test" {
		return false
	}
	seen := map[string]bool{}
	values := map[string]string{}
	packageSeen := false
	for n := 2; n < len(command); n++ {
		arg := command[n]
		if !strings.HasPrefix(arg, "-") {
			if packageSeen || n != len(command)-1 || arg != expected.Selector.Package {
				return false
			}
			packageSeen = true
			continue
		}
		if packageSeen {
			return false
		}
		flag, value, assigned := strings.Cut(arg, "=")
		if seen[flag] {
			return false
		}
		seen[flag] = true
		switch flag {
		case "-json":
			if !assigned {
				value = "true"
			}
			if value != "true" {
				return false
			}
		case "-count", "-run", "-tags":
			if !assigned {
				if n+1 >= len(command) {
					return false
				}
				n++
				value = command[n]
			}
		default:
			return false
		}
		values[flag] = value
	}
	tags, declared := expected.Basis.Environment["build_tags"]
	return declared && validBuildTagsValue(tags) && packageSeen && seen["-json"] &&
		values["-count"] == "1" && values["-run"] == ExactRunPattern(expected.Selector.Test) &&
		values["-tags"] == tags && (tags == "" || seen["-tags"])
}
func decodeEvent(raw []byte) (Event, error) {
	var e Event
	decoder := json.NewDecoder(bytes.NewReader(raw))
	start, err := decoder.Token()
	if err != nil || start != json.Delim('{') {
		return e, fmt.Errorf("runner event is not a JSON object")
	}
	seen := map[string]bool{}
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return e, err
		}
		name, ok := key.(string)
		if !ok || seen[name] {
			return e, fmt.Errorf("duplicate or invalid runner event key")
		}
		seen[name] = true
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return e, err
		}
	}
	if _, err := decoder.Token(); err != nil {
		return e, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return e, fmt.Errorf("trailing runner event content")
	}
	if err := json.Unmarshal(raw, &e); err != nil {
		return e, err
	}
	if e.Action == "" {
		return e, fmt.Errorf("runner event action absent")
	}
	return e, nil
}
