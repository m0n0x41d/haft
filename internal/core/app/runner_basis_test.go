package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/core/check"
)

func TestActualWrongBuildTagsCannotSupportCapturedBasis(t *testing.T) {
	s := service(t)
	d, _ := seed(t, s)
	policy := filepath.Join(s.Root, "policy.go")
	raw, err := os.ReadFile(policy)
	if err != nil {
		t.Fatal(err)
	}
	broken := strings.Replace(string(raw), `status == "new" || status == "paid"`, `status == "new"`, 1)
	if err := os.WriteFile(policy, []byte("//go:build !alternate\n\n"+broken), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.Root, "policy_alternate.go"), append([]byte("//go:build alternate\n\n"), raw...), 0600); err != nil {
		t.Fatal(err)
	}
	prepared := run(t, s, Request{Operation: "check", Action: "prepare", Ref: d.Record.ID + "#total-preserved", CheckRef: "pbt:order_test.go::TestCancelPreservesTotal", Scope: "finite property fixture"}, "prepared").Data.(map[string]any)
	contract := prepared["expected"].(check.Contract)
	command := append([]string(nil), prepared["command"].([]string)...)
	actual := append(append([]string(nil), command[:len(command)-1]...), "-tags=alternate", command[len(command)-1])
	cmd := exec.Command(actual[0], actual[1:]...)
	cmd.Dir = s.Root
	cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOWORK=off", "GOFLAGS=", "CGO_ENABLED=0", "GOOS="+contract.Basis.Environment["goos"], "GOARCH="+contract.Basis.Environment["goarch"])
	control := exec.Command(command[0], command[1:]...)
	control.Dir, control.Env = s.Root, cmd.Env
	controlOutput, controlErr := control.CombinedOutput()
	if controlErr == nil || !bytes.Contains(controlOutput, []byte("failed on input")) {
		t.Fatalf("captured default implementation did not fail the oracle: %v %s", controlErr, controlOutput)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatal(err, stdout.String(), stderr.String())
	}
	zero := 0
	input := check.ObservationInput{Expected: contract, Observed: check.Run{Started: true, ExitCode: &zero, Command: actual, Selector: contract.Selector, Basis: contract.Basis, Stdout: stdout.Bytes(), Stderr: stderr.Bytes()}}
	got := s.Execute(context.Background(), Request{Format: Format, Operation: "check", Action: "observe", Observation: &input})
	t.Logf("prepared=%q actual=%q result=%s current_basis=%v", command, actual, got.Kind, got.Data.(map[string]any)["current_basis"])
	if got.Kind != check.Unattributable {
		t.Fatal("wrong actual build tags passed as exact current capture")
	}
}

func TestPrepareExposesIndependentlyHashableActualBasis(t *testing.T) {
	s := service(t)
	d, _ := seed(t, s)
	result := run(t, s, Request{Operation: "check", Action: "prepare", Ref: d.Record.ID + "#total-preserved", CheckRef: "pbt:order_test.go::TestCancelPreservesTotal", Scope: "fixture basis inspection"}, "prepared")
	// Use serialized public JSON, as an external client must, rather than helper
	// internals. Hash actual source bytes independently with the standard library.
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var public struct {
		Data struct {
			Expected check.Contract `json:"expected"`
			Capture  struct {
				Format                 string             `json:"format"`
				Algorithm              string             `json:"hash_algorithm"`
				Implementations        map[string]string  `json:"implementation_files"`
				ImplementationPreimage string             `json:"implementation_preimage_base64"`
				Oracle                 CheckOracleCapture `json:"oracle"`
				Dependencies           map[string]string  `json:"dependency_files"`
				DependencyPreimage     string             `json:"dependency_preimage_base64"`
			} `json:"basis_capture"`
		} `json:"data"`
	}
	if err := json.Unmarshal(encoded, &public); err != nil {
		t.Fatal(err)
	}
	capture := public.Data.Capture
	contract := public.Data.Expected
	digest := func(raw []byte) string { sum := sha256.Sum256(raw); return "sha256:" + hex.EncodeToString(sum[:]) }
	impl, err := base64.StdEncoding.DecodeString(capture.ImplementationPreimage)
	if err != nil {
		t.Fatal(err)
	}
	dependencies, err := base64.StdEncoding.DecodeString(capture.DependencyPreimage)
	if err != nil {
		t.Fatal(err)
	}
	if capture.Format != "haft.check-basis-capture/1" || capture.Algorithm == "" || digest(impl) != contract.Basis.Code || digest(dependencies) != contract.Basis.Dependencies {
		t.Fatal("public preimages do not reproduce declared digests")
	}
	for _, files := range []map[string]string{capture.Implementations, capture.Dependencies} {
		for name, want := range files {
			raw, err := os.ReadFile(filepath.Join(s.Root, name))
			if err != nil || digest(raw) != want {
				t.Fatal("actual source digest mismatch", name, err)
			}
		}
	}
	oracle, err := os.ReadFile(filepath.Join(s.Root, capture.Oracle.Path))
	if err != nil {
		t.Fatal(err)
	}
	if capture.Oracle.StartByte < 0 || capture.Oracle.EndByte > len(oracle) || capture.Oracle.StartByte >= capture.Oracle.EndByte {
		t.Fatal("unusable oracle offsets")
	}
	if digest(oracle[capture.Oracle.StartByte:capture.Oracle.EndByte]) != contract.Basis.Check || contract.Basis.Check != capture.Oracle.RawDigest {
		t.Fatal("oracle slice does not reproduce check basis")
	}
	var implFiles map[string]string
	if err := json.Unmarshal(impl, &implFiles); err != nil || len(implFiles) != len(capture.Implementations) {
		t.Fatal("implementation map differs from preimage", err)
	}
	for name, value := range implFiles {
		if capture.Implementations[name] != value {
			t.Fatal("preimage map mismatch")
		}
	}
}
