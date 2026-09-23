package app

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/core/carrier"
	"github.com/m0n0x41d/haft/internal/core/change"
	"github.com/m0n0x41d/haft/internal/core/check"
)

func TestSecondProjectSpecChangeCheckRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("actual isolated Go runner")
	}
	root := t.TempDir()
	for _, name := range []string{"go.mod", "reservation.go", "reservation_test.go"} {
		raw, err := os.ReadFile("../testdata/reservation/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	s := Service{Root: root}
	raw, err := os.ReadFile("../testdata/reservation/spec.md")
	if err != nil {
		t.Fatal(err)
	}
	run(t, s, Request{Operation: "remember", RequestID: "stock-spec", Carrier: string(raw)}, "written")
	v := readView(t, s)
	doc := v.Documents[0]
	base := doc.Record.ID + "@" + doc.Edition
	claim := doc.Record.Claims[0]
	claim.Text += " The sum of the two counters is conserved."
	c := change.Change{Format: change.Format, ID: "chg-20260924-00000001", ChangeKey: "chg-20260924-00000001", Title: "Clarify stock sum", Intent: "Make counter conservation explicit", State: "open", CreatedAt: s.now(), Patches: []change.SectionPatch{{Base: base, Operations: []change.Operation{{Op: "MODIFIED", ClaimID: claim.ID, Claim: &claim, Reason: "Clarify existing conservation"}}}}}
	changeRaw, err := change.Encode(c, nil)
	if err != nil {
		t.Fatal(err)
	}
	run(t, s, Request{Operation: "change", Action: "create", Carrier: string(changeRaw), RequestID: "stock-change"}, "written")
	preview := run(t, s, Request{Operation: "change", Action: "preview", Ref: c.ID}, "ready")
	run(t, s, Request{Operation: "change", Action: "sync", Ref: preview.Basis["change_ref"], RequestID: "stock-sync", PreviewDigest: preview.Basis["preview_digest"], ExpectedGeneration: preview.Basis["memory_generation"]}, "written")
	run(t, s, Request{Operation: "context", Ref: "sym:reservation.go::Stock.Reserve"}, "exact")
	prepared := run(t, s, Request{Operation: "check", Action: "prepare", Ref: "spec:stock-reservation#conservation", CheckRef: "pbt:reservation_test.go::TestReservationConservesStock", Scope: "1000 generated uint32 stock triples, seed 24"}, "prepared").Data.(map[string]any)
	contract := prepared["expected"].(check.Contract)
	command := prepared["command"].([]string)
	sandbox := t.TempDir()
	for _, dir := range []string{"home", "cache", "mod", "tmp"} {
		if err := os.Mkdir(filepath.Join(sandbox, dir), 0700); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	cmd.Dir = root
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + filepath.Join(sandbox, "home"), "GOCACHE=" + filepath.Join(sandbox, "cache"), "GOMODCACHE=" + filepath.Join(sandbox, "mod"), "GOPATH=" + filepath.Join(sandbox, "mod"), "TMPDIR=" + filepath.Join(sandbox, "tmp")}
	for key, value := range prepared["run_environment"].(map[string]string) {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("second project runner: %v\n%s\n%s", err, stdout.Bytes(), stderr.Bytes())
	}
	exit := 0
	input := check.ObservationInput{Expected: contract, Observed: check.Run{Started: true, ExitCode: &exit, Command: command, Selector: contract.Selector, Basis: contract.Basis, Stdout: stdout.Bytes(), Stderr: stderr.Bytes()}}
	observed := run(t, s, Request{Operation: "check", Action: "observe", Observation: &input}, check.Passed)
	observation := observed.Data.(map[string]any)["observation"].(check.Observation)
	proof, err := json.MarshalIndent(observation, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	evidence := carrier.Record{Kind: "evidence", Title: "Stock property observation", Status: "active", About: "domain:Inventory.Reservation", Claim: "Recorded finite stock conservation run passed", ObservedAt: s.now(), Method: "actual Go testing/quick run", Source: "observation embedded in this carrier body", Basis: &carrier.EvidenceBasis{Kind: "code", Ref: observation.ID}, Uses: []carrier.EvidenceUse{{ID: "stock", Target: contract.Basis.Claim, Check: contract.Ref, Polarity: "supports", Scope: contract.Scope}}}
	run(t, s, Request{Operation: "remember", Carrier: encode(t, evidence, proof), RequestID: "stock-result"}, "written")
	fresh := Service{Root: root}
	run(t, fresh, Request{Operation: "recall", Ref: contract.Basis.Claim}, "found")
	v = readView(t, fresh)
	if len(v.Projection.UsesFor(contract.Basis.Claim)) != 1 {
		t.Fatal("second project evidence was not recovered")
	}
	if len(v.Projection.UsesFor(base+"#conservation")) != 0 {
		t.Fatal("new run attached to old spec wording")
	}
}
