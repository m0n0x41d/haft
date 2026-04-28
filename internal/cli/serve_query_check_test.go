package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/spf13/cobra"
)

func TestHandleQuintQuery_CheckActionReturnsCheckReport(t *testing.T) {
	fixture := newCheckTestProject(t)
	seedGovernanceDebt(t, fixture)

	args := map[string]any{
		"action": "check",
	}

	result, err := handleQuintQuery(context.Background(), fixture.store, fixture.haftDir, args)
	if err != nil {
		t.Fatalf("handleQuintQuery(check) returned error: %v", err)
	}

	var report checkReport
	if err := json.Unmarshal([]byte(result), &report); err != nil {
		t.Fatalf("decode JSON: %v\nraw: %s", err, result)
	}

	if report.Summary.TotalFindings == 0 {
		t.Fatalf("expected at least one finding from seedGovernanceDebt; got 0\n%s", result)
	}
}

func TestHandleQuintQuery_CheckMatchesCLIJSON(t *testing.T) {
	fixture := newCheckTestProject(t)
	seedGovernanceDebt(t, fixture)

	mcpResult, err := handleQuintQuery(context.Background(), fixture.store, fixture.haftDir, map[string]any{"action": "check"})
	if err != nil {
		t.Fatalf("MCP check: %v", err)
	}

	restoreCwd := enterTestProjectRoot(t, fixture.root)
	defer restoreCwd()

	restoreJSON := stubCheckJSON(t, true)
	defer restoreJSON()
	_ = stubCheckExit(t)

	var cliBuf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&cliBuf)
	if err := runCheck(cmd, nil); err != nil {
		t.Fatalf("CLI check: %v", err)
	}

	var mcpReport, cliReport checkReport
	if err := json.Unmarshal([]byte(mcpResult), &mcpReport); err != nil {
		t.Fatalf("decode MCP JSON: %v", err)
	}
	if err := json.Unmarshal(cliBuf.Bytes(), &cliReport); err != nil {
		t.Fatalf("decode CLI JSON: %v", err)
	}

	if mcpReport.Summary.TotalFindings != cliReport.Summary.TotalFindings {
		t.Fatalf("total_findings parity broken: MCP=%d CLI=%d", mcpReport.Summary.TotalFindings, cliReport.Summary.TotalFindings)
	}
	if len(mcpReport.Stale) != len(cliReport.Stale) {
		t.Fatalf("stale parity: MCP=%d CLI=%d", len(mcpReport.Stale), len(cliReport.Stale))
	}
	if len(mcpReport.Drifted) != len(cliReport.Drifted) {
		t.Fatalf("drift parity: MCP=%d CLI=%d", len(mcpReport.Drifted), len(cliReport.Drifted))
	}
	if len(mcpReport.SpecHealth) != len(cliReport.SpecHealth) {
		t.Fatalf("spec_health parity: MCP=%d CLI=%d", len(mcpReport.SpecHealth), len(cliReport.SpecHealth))
	}
}

func TestHandleQuintQuery_CheckRejectsUnknownAction(t *testing.T) {
	fixture := newCheckTestProject(t)

	_, err := handleQuintQuery(context.Background(), fixture.store, fixture.haftDir, map[string]any{
		"action": "wat",
	})
	if err == nil {
		t.Fatalf("handleQuintQuery should reject unknown action")
	}
}
