package app

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/core/check"
)

// This test is an actual uncached runner, separate from the pure JSON adapter
// fixtures. Its subprocess paths are local to the test sandbox.
func TestActualGoPropertyObservationAndDependencyMutation(t *testing.T) {
	if testing.Short() {
		t.Skip("actual isolated Go compilation")
	}
	s := service(t)
	d, _ := seed(t, s)
	sandbox := t.TempDir()
	for _, name := range []string{"home", "cache", "modules", "tmp", "gopath"} {
		if err := os.Mkdir(filepath.Join(sandbox, name), 0700); err != nil {
			t.Fatal(err)
		}
	}
	seed := int64(23)
	q := Request{Operation: "check", Action: "prepare", Ref: d.Record.ID + "#total-preserved", CheckRef: "pbt:order_test.go::TestCancelPreservesTotal",
		Scope: "1000 generated new/paid cases, signed integer totals; no HTTP/storage coverage", Seed: &seed,
		FailureContract: "Reviewed testing/quick false-property failure from TestCancelPreservesTotal",
		FailurePattern:  `#[0-9]+: failed on input`}
	observe := func(want string) check.ObservationInput {
		t.Helper()
		prepared := run(t, s, q, "prepared").Data.(map[string]any)
		contract := prepared["expected"].(check.Contract)
		command := prepared["command"].([]string)
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, command[0], command[1:]...)
		cmd.Dir = s.Root
		cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + filepath.Join(sandbox, "home"), "GOCACHE=" + filepath.Join(sandbox, "cache"), "GOMODCACHE=" + filepath.Join(sandbox, "modules"), "GOPATH=" + filepath.Join(sandbox, "gopath"), "TMPDIR=" + filepath.Join(sandbox, "tmp"), "GOWORK=off", "GOTOOLCHAIN=local", "GOOS=" + contract.Basis.Environment["goos"], "GOARCH=" + contract.Basis.Environment["goarch"], "CGO_ENABLED=0"}
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err := cmd.Run()
		exit := 0
		environment := ""
		if err != nil {
			if cmd.ProcessState != nil {
				exit = cmd.ProcessState.ExitCode()
			} else {
				environment = "start_failed"
				exit = -1
			}
		}
		if ctx.Err() != nil {
			environment = "timeout"
		}
		input := check.ObservationInput{Expected: contract, Observed: check.Run{Started: cmd.Process != nil, ExitCode: &exit, Command: command, Selector: contract.Selector, Basis: contract.Basis, Stdout: stdout.Bytes(), Stderr: stderr.Bytes(), EnvironmentFailure: environment}}
		got := run(t, s, Request{Operation: "check", Action: "observe", Observation: &input}, want)
		if got.Data.(map[string]any)["current_basis"] != "same" {
			t.Fatal("captured code basis mismatch for actual observation")
		}
		return input
	}
	passed := observe(check.Passed)
	helper := filepath.Join(s.Root, "policy.go")
	raw, err := os.ReadFile(helper)
	if err != nil {
		t.Fatal(err)
	}
	changed := strings.Replace(string(raw), `status == "new" || status == "paid"`, `status == "new"`, 1)
	if changed == string(raw) {
		t.Fatal("mutation oracle failed to edit the intended helper")
	}
	if err := os.WriteFile(helper, []byte(changed), 0600); err != nil {
		t.Fatal(err)
	}
	old := run(t, s, Request{Operation: "check", Action: "observe", Observation: &passed}, check.Passed)
	if old.Data.(map[string]any)["current_basis"] != "changed" {
		t.Fatal("old pass presented as current after dependency edit")
	}
	failed := observe(check.AssertionFailure)
	if failed.Expected.Basis.Code != passed.Expected.Basis.Code {
		t.Fatal("method file changed instead of its helper")
	}
	if failed.Expected.Basis.Dependencies == passed.Expected.Basis.Dependencies {
		t.Fatal("dependency change absent from exact runner basis")
	}
}
