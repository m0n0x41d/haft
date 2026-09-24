package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/core/carrier"
	"github.com/m0n0x41d/haft/internal/core/check"
)

// These regressions exercise public application preparation/observation with
// real uncached Go commands. The enclosing test environment owns all Go paths.
func runnerRegressionFixture(t *testing.T, files map[string]string) (Service, Request) {
	t.Helper()
	s := Service{Root: t.TempDir(), Now: func() time.Time { return time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC) }}
	files["go.mod"] = "module regression.example/answer\n\ngo 1.25.0\n"
	for p, raw := range files {
		runnerRegressionWrite(t, s.Root, p, raw)
	}
	record := carrier.Record{Kind: "spec", Title: "Answer", About: "domain:Review.Answer", Slug: "answer", ReceivingUse: "Review exact runner attribution", Claims: []carrier.Claim{{ID: "answer", Kind: "law", Text: "Answer returns one.", ImplementedBy: []carrier.Binding{{Ref: "sym:answer.go::Answer", Covers: "Answer implementation"}}, Checks: []carrier.Binding{{Ref: "test:answer_test.go::TestAnswer", Covers: "One example asserting Answer equals one"}}}}}
	run(t, s, Request{Operation: "remember", RequestID: "runner-regression-spec", Carrier: encode(t, record, nil)}, "written")
	return s, Request{Operation: "check", Action: "prepare", Ref: "spec:answer#answer", CheckRef: "test:answer_test.go::TestAnswer", Scope: "Answer equals one", FailureContract: "Answer example reports an unequal result through t.Fatal", FailurePattern: "answer must be one"}
}
func runnerRegressionWrite(t *testing.T, root, name, raw string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
}
func runnerRegressionPrepare(t *testing.T, s Service, q Request) map[string]any {
	t.Helper()
	return run(t, s, q, "prepared").Data.(map[string]any)
}
func runnerRegressionRun(t *testing.T, s Service, prepared map[string]any, wantExit int) check.ObservationInput {
	t.Helper()
	contract := prepared["expected"].(check.Contract)
	captured := prepared["basis_capture"].(CheckBasisCapture)
	verify := func() {
		t.Helper()
		if carrier.Digest(captured.DependencyPreimage) != contract.Basis.Dependencies || carrier.Digest(captured.ImplementationPreimage) != contract.Basis.Code {
			t.Fatal("capture preimage mismatch")
		}
		for _, files := range []map[string]string{captured.ImplementationFiles, captured.DependencyFiles} {
			for name, want := range files {
				raw, err := os.ReadFile(filepath.Join(s.Root, name))
				if err != nil || carrier.Digest(raw) != want {
					t.Fatalf("capture differs from actual file %s: %v", name, err)
				}
			}
		}
		raw, err := os.ReadFile(filepath.Join(s.Root, captured.Oracle.Path))
		if err != nil {
			t.Fatal(err)
		}
		if carrier.Digest(raw[captured.Oracle.StartByte:captured.Oracle.EndByte]) != contract.Basis.Check {
			t.Fatal("oracle differs from actual bytes")
		}
	}
	verify()
	command := append([]string(nil), prepared["command"].([]string)...)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	cmd.Dir = s.Root
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOTOOLCHAIN=local", "GOFLAGS=", "GOPROXY=off", "GOSUMDB=off")
	for key, value := range prepared["run_environment"].(map[string]string) {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	exit := 0
	if err != nil {
		if cmd.ProcessState == nil {
			t.Fatal(err)
		}
		exit = cmd.ProcessState.ExitCode()
	}
	if exit != wantExit {
		t.Fatalf("actual command %q exit %d want %d: %s %s", command, exit, wantExit, stdout.String(), stderr.String())
	}
	verify()
	t.Logf("actual command=%q exit=%d stdout=%s stderr=%s", command, exit, stdout.String(), stderr.String())
	return check.ObservationInput{Expected: contract, Observed: check.Run{Started: true, ExitCode: &exit, Command: command, Selector: contract.Selector, Basis: contract.Basis, Stdout: stdout.Bytes(), Stderr: stderr.Bytes()}}
}

const runnerAnswerTest = "package answer\nimport \"testing\"\nfunc TestAnswer(t *testing.T) { if Answer()!=1 {t.Fatal(\"answer must be one\")} }\n"

func TestRunnerBuildIncludesActualLocalTestInputs(t *testing.T) {
	if testing.Short() {
		t.Skip("actual Go test build")
	}
	cases := []struct {
		name           string
		files          map[string]string
		changed, after string
	}{
		{"external-init", map[string]string{"answer.go": "package answer\nvar Offset int\nfunc Answer() int { return 1+Offset }\n", "answer_test.go": runnerAnswerTest, "external_test.go": "package answer_test\nimport answer \"regression.example/answer\"\nfunc init(){answer.Offset=0}\n"}, "external_test.go", "package answer_test\nimport answer \"regression.example/answer\"\nfunc init(){answer.Offset=1}\n"},
		{"external-TestMain-local-import", map[string]string{"answer.go": "package answer\nvar Offset int\nfunc Answer() int { return 1+Offset }\n", "answer_test.go": runnerAnswerTest, "external_test.go": "package answer_test\nimport (\"os\";\"testing\";answer \"regression.example/answer\";\"regression.example/answer/setup\")\nfunc TestMain(m *testing.M){answer.Offset=setup.Offset();os.Exit(m.Run())}\n", "setup/setup.go": "package setup\nfunc Offset()int{return 0}\n"}, "setup/setup.go", "package setup\nfunc Offset()int{return 1}\n"},
		{"ordinary-Go-control", map[string]string{"answer.go": "package answer\nfunc Answer()int{return 1}\n", "answer_test.go": runnerAnswerTest}, "answer.go", "package answer\nfunc Answer()int{return 2}\n"},
	}
	// A real architecture-selected assembly function, with a separately mutable
	// local include. Unsupported execution architectures retain the pure tests.
	var asm string
	switch runtime.GOARCH {
	case "arm64":
		asm = "#include \"textflag.h\"\n#include \"answer.h\"\nTEXT ·Answer(SB),NOSPLIT,$0-8\n MOVD $ANSWER, R0\n MOVD R0, ret+0(FP)\n RET\n"
	case "amd64":
		asm = "#include \"textflag.h\"\n#include \"answer.h\"\nTEXT ·Answer(SB),NOSPLIT,$0-8\n MOVQ $ANSWER, AX\n MOVQ AX, ret+0(FP)\n RET\n"
	}
	if asm != "" {
		name := "answer_" + runtime.GOARCH + ".s"
		for _, mutation := range []struct{ name, path, raw string }{{"assembly", name, strings.Replace(asm, "$ANSWER", "$2", 1)}, {"local-header", "answer.h", "#define ANSWER 2\n"}} {
			cases = append(cases, struct {
				name           string
				files          map[string]string
				changed, after string
			}{mutation.name, map[string]string{"answer.go": "package answer\nfunc Answer()int\n", "answer_test.go": runnerAnswerTest, name: asm, "answer.h": "#define ANSWER 1\n"}, mutation.path, mutation.raw})
		}
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, q := runnerRegressionFixture(t, tc.files)
			before := runnerRegressionPrepare(t, s, q)
			old := runnerRegressionRun(t, s, before, 0)
			runnerRegressionWrite(t, s.Root, tc.changed, tc.after)
			after := runnerRegressionPrepare(t, s, q)
			runnerRegressionRun(t, s, after, 1)
			observed := run(t, s, Request{Operation: "check", Action: "observe", Observation: &old}, check.Passed)
			data := observed.Data.(map[string]any)
			if data["current_basis"] != "changed" {
				t.Errorf("old pass falsely current after %s mutation: %#v", tc.changed, data["current_basis"])
			}
			a := before["expected"].(check.Contract)
			b := after["expected"].(check.Contract)
			if a.Basis.Dependencies == b.Basis.Dependencies {
				t.Error("actual local test-build mutation left dependency basis unchanged")
			}
			if before["basis_capture"].(CheckBasisCapture).DependencyFiles[tc.changed] == "" {
				t.Errorf("actual test input absent from captured dependency files: %s", tc.changed)
			}
			if !bytes.Equal(data["observation"].(check.Observation).Input.Observed.Stdout, old.Observed.Stdout) {
				t.Error("historical observation bytes changed")
			}
		})
	}
}

func TestObserveCannotAttributeOtherPackageWithSameTestName(t *testing.T) {
	if testing.Short() {
		t.Skip("actual Go test build")
	}
	s, q := runnerRegressionFixture(t, map[string]string{"answer.go": "package answer\nfunc Answer()int{return 2}\n", "answer_test.go": runnerAnswerTest, "other/other_test.go": "package other\nimport \"testing\"\nfunc TestAnswer(t *testing.T){if 1+1!=2 {t.Fatal(\"arithmetic\")}}\n"})
	prepared := runnerRegressionPrepare(t, s, q)
	runnerRegressionRun(t, s, prepared, 1)
	altered := make(map[string]any)
	for k, v := range prepared {
		altered[k] = v
	}
	contract := prepared["expected"].(check.Contract)
	contract.Selector.Package += "/other"
	altered["expected"] = contract
	command := append([]string(nil), prepared["command"].([]string)...)
	command[len(command)-1] = contract.Selector.Package
	altered["command"] = command
	other := runnerRegressionRun(t, s, altered, 0)
	got := s.Execute(context.Background(), Request{Format: Format, Operation: "check", Action: "observe", Observation: &other})
	data := got.Data.(map[string]any)
	if got.Kind != check.Unattributable || data["current_basis"] == "same" || data["declared_check_binding"] != false {
		raw, _ := json.Marshal(got)
		t.Fatalf("different package falsely attributed to declared oracle: %s", raw)
	}
	if !bytes.Equal(data["observation"].(check.Observation).Input.Observed.Stdout, other.Observed.Stdout) {
		t.Fatal("rejected observation raw output lost")
	}
}

func TestPrepareRejectsUnsupportedLocalBuildComposition(t *testing.T) {
	cases := []struct {
		name  string
		extra map[string]string
	}{
		{"ignored-assembly", map[string]string{"answer.s": "// local assembly input\n", ".gitignore": "*.s\n"}},
		{"uncaptured-assembly-include", map[string]string{"answer.s": "#include \"../outside.h\"\n"}},
		{"embed", map[string]string{"asset.go": "package answer\nimport _ \"embed\"\n//go:embed payload.txt\nvar data string\n", "payload.txt": "payload"}},
		{"vendor", map[string]string{"vendor/modules.txt": "# local vendor mode needs a separate resolver\n"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			files := map[string]string{"answer.go": "package answer\nfunc Answer()int{return 1}\n", "answer_test.go": runnerAnswerTest}
			for p, v := range tc.extra {
				files[p] = v
			}
			s, q := runnerRegressionFixture(t, files)
			q.Format = Format
			got := s.Execute(context.Background(), q)
			if got.Kind == "prepared" {
				t.Fatal("unsupported composition silently received exact prepared basis")
			}
		})
	}
}

func TestRunnerBuildAgreesWithGoSelectedTestPackages(t *testing.T) {
	if testing.Short() {
		t.Skip("actual Go package selection")
	}
	s, q := runnerRegressionFixture(t, map[string]string{
		"answer.go":            "package answer\nfunc Answer()int{return 1}\n",
		"answer_test.go":       runnerAnswerTest,
		"external_test.go":     "package answer_test\nimport _ \"regression.example/answer/setup\"\n",
		"setup/setup.go":       "package setup\nimport _ \"regression.example/answer/helper\"\n",
		"setup/setup_test.go":  "package setup\nimport _ \"regression.example/answer/testonly\"\n",
		"helper/helper.go":     "package helper\n",
		"testonly/testonly.go": "package testonly\n",
	})
	prepared := runnerRegressionPrepare(t, s, q)
	cmd := exec.Command("go", "list", "-test", "-deps", "-json", "regression.example/answer")
	cmd.Dir = s.Root
	cmd.Env = append(os.Environ(), "GOPROXY=off", "GOSUMDB=off")
	for k, v := range prepared["run_environment"].(map[string]string) {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatal(err, stderr.String())
	}
	physical, err := filepath.EvalSymlinks(s.Root)
	if err != nil {
		t.Fatal(err)
	}
	linked := filepath.Join(t.TempDir(), "project-link")
	if err := os.Symlink(physical, linked); err != nil {
		t.Fatal(err)
	}
	var actual map[string]bool
	for _, root := range []string{s.Root, physical, linked} {
		selected := runnerSelectedInputs(t, root, stdout.Bytes())
		if len(selected) == 0 {
			t.Fatalf("go list selected no local inputs for %s", root)
		}
		if actual != nil && !sameJSON(selected, actual) {
			t.Fatalf("symlink/physical roots disagree: %v != %v", selected, actual)
		}
		actual = selected
	}
	if outside := runnerSelectedInputs(t, t.TempDir(), stdout.Bytes()); len(outside) != 0 {
		t.Fatalf("external packages accepted as local: %v", outside)
	}
	captured := prepared["basis_capture"].(CheckBasisCapture).DependencyFiles
	for p := range actual {
		if captured[p] == "" {
			t.Errorf("go list selected local file is absent: %s", p)
		}
	}
	for p := range captured {
		if strings.HasSuffix(p, ".go") && !actual[p] {
			t.Errorf("nonexecuted test package entered selected build: %s", p)
		}
	}
	names := []string{}
	for p := range actual {
		names = append(names, p)
	}
	sort.Strings(names)
	t.Logf("independent go list -test -deps local compilation inputs: %v", names)
}

func runnerSelectedInputs(t *testing.T, root string, raw []byte) map[string]bool {
	t.Helper()
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	actual := map[string]bool{}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	for {
		var pkg struct {
			Dir                                string
			GoFiles, SFiles, HFiles, SysoFiles []string
		}
		err := decoder.Decode(&pkg)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if pkg.Dir == "" { // Synthetic packages have no filesystem directory.
			continue
		}
		physical, err := filepath.EvalSymlinks(pkg.Dir)
		if err != nil {
			t.Fatal(err)
		}
		dir, err := filepath.Rel(root, physical)
		if err != nil {
			t.Fatal(err)
		}
		if dir == ".." || strings.HasPrefix(dir, ".."+string(filepath.Separator)) {
			continue
		}
		for _, names := range [][]string{pkg.GoFiles, pkg.SFiles, pkg.HFiles, pkg.SysoFiles} {
			for _, name := range names {
				if filepath.IsAbs(name) {
					continue
				} // generated test main in the Go build cache
				actual[filepath.ToSlash(filepath.Join(dir, name))] = true
			}
		}
	}
	return actual
}

func TestObserveRetainsHistoricalOracleBytesAndRejectsChangedConditions(t *testing.T) {
	if testing.Short() {
		t.Skip("actual Go test run")
	}
	s, q := runnerRegressionFixture(t, map[string]string{"answer.go": "package answer\nfunc Answer()int{return 1}\n", "answer_test.go": runnerAnswerTest})
	prepared := runnerRegressionPrepare(t, s, q)
	old := runnerRegressionRun(t, s, prepared, 0)
	runnerRegressionWrite(t, s.Root, "answer_test.go", strings.Replace(runnerAnswerTest, "if Answer()", "/* oracle presentation changed */ if Answer()", 1))
	historical := run(t, s, Request{Operation: "check", Action: "observe", Observation: &old}, check.Passed)
	if historical.Data.(map[string]any)["current_basis"] != "changed" {
		t.Fatal("old oracle observation promoted to current")
	}
	bad := old
	bad.Expected.Basis.Conditions = []string{"invented broader validation condition"}
	bad.Observed.Basis = bad.Expected.Basis
	got := run(t, s, Request{Operation: "check", Action: "observe", Observation: &bad}, check.Unattributable)
	if got.Data.(map[string]any)["declared_check_binding"] != false {
		t.Fatal("undeclared conditions attributed to exact old claim")
	}
}

func TestPreparedEnvironmentOverridesAmbientArchitectureFeatures(t *testing.T) {
	if testing.Short() {
		t.Skip("actual Go environment query")
	}
	key, value := "", ""
	switch runtime.GOARCH {
	case "amd64":
		key, value = "GOAMD64", "v4"
	case "arm64":
		key, value = "GOARM64", "v9.0"
	default:
		t.Skip("actual environment control covered on amd64/arm64")
	}
	s, q := runnerRegressionFixture(t, map[string]string{"answer.go": "package answer\nfunc Answer()int{return 1}\n", "answer_test.go": runnerAnswerTest})
	prepared := runnerRegressionPrepare(t, s, q)
	env := prepared["run_environment"].(map[string]string)
	contract := prepared["expected"].(check.Contract)
	if env[key] == "" || env[key] == value || env["GOFIPS140"] != "off" || contract.Basis.Environment[strings.ToLower(key)] != env[key] {
		t.Fatal("feature configuration not pinned", env, contract.Basis.Environment)
	}
	cmd := exec.Command("go", "env", "-json", key, "GOFIPS140", "GOEXPERIMENT", "CGO_ENABLED")
	cmd.Dir = s.Root
	cmd.Env = append(os.Environ(), key+"="+value, "GOFIPS140=latest")
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	raw, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	actual := map[string]string{}
	if err := json.Unmarshal(raw, &actual); err != nil {
		t.Fatal(err)
	}
	if actual[key] != env[key] || actual["GOFIPS140"] != "off" || actual["GOEXPERIMENT"] != "" || actual["CGO_ENABLED"] != "0" {
		t.Fatal("actual Go configuration disagrees with preparation", actual, env)
	}
	t.Logf("actual Go environment with conflicting inherited %s=%s: %s", key, value, raw)
}

func TestPrepareRejectsAmbiguousOrNonTestRegistration(t *testing.T) {
	cases := []struct{ name, oracle, external string }{
		{"duplicate-internal-external", runnerAnswerTest, "package answer_test\nimport \"testing\"\nfunc TestAnswer(t *testing.T){t.Fatal(\"different oracle\")}\n"},
		{"method-is-not-a-test", "package answer\nimport \"testing\"\ntype Suite struct{}\nfunc (Suite) TestAnswer(t *testing.T){}\n", "package answer_test\nimport \"testing\"\nfunc TestAnswer(t *testing.T){}\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, q := runnerRegressionFixture(t, map[string]string{"answer.go": "package answer\nfunc Answer()int{return 1}\n", "answer_test.go": tc.oracle, "external_test.go": tc.external})
			q.Format = Format
			got := s.Execute(context.Background(), q)
			if got.Kind == "prepared" {
				t.Fatal("declared source oracle does not identify one selectable Go test registration")
			}
		})
	}
}
