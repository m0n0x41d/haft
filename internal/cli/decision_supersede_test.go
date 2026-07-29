package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/spf13/cobra"
)

func TestRunDecisionSupersedeRecordsTerminalStatusAndLineage(t *testing.T) {
	fixture := newCheckTestProject(t)
	restoreRoot := enterTestProjectRoot(t, fixture.root)
	defer restoreRoot()
	seedDecisionSupersedeArtifact(t, fixture, "dec-old", artifact.StatusActive)
	seedDecisionSupersedeArtifact(t, fixture, "dec-new", artifact.StatusActive)
	restoreFlags := stubDecisionSupersedeFlags("dec-new", "The source-derived FPF Query replaces compiled routing.", true)
	defer restoreFlags()

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)
	if err := runDecisionSupersede(cmd, []string{"dec-old"}); err != nil {
		t.Fatalf("runDecisionSupersede returned error: %v", err)
	}

	var receipt decisionSupersedeReceipt
	if err := json.Unmarshal(output.Bytes(), &receipt); err != nil {
		t.Fatalf("decode receipt: %v\n%s", err, output.String())
	}
	if receipt.Authority != "manual_cli_operator_supersession" {
		t.Fatalf("authority = %q", receipt.Authority)
	}
	if receipt.SupersededRef != "dec-old" || receipt.SuccessorRef != "dec-new" {
		t.Fatalf("receipt refs = %#v", receipt)
	}
	if receipt.Status != string(artifact.StatusSuperseded) {
		t.Fatalf("receipt status = %q", receipt.Status)
	}

	updated, err := fixture.store.Get(context.Background(), "dec-old")
	if err != nil {
		t.Fatalf("load superseded decision: %v", err)
	}
	if updated.Meta.Status != artifact.StatusSuperseded {
		t.Fatalf("stored status = %q", updated.Meta.Status)
	}
	if !strings.Contains(updated.Body, "The source-derived FPF Query replaces compiled routing.") {
		t.Fatalf("supersession reason missing from body:\n%s", updated.Body)
	}
	links, err := fixture.store.GetLinks(context.Background(), "dec-new")
	if err != nil {
		t.Fatalf("load successor links: %v", err)
	}
	if !decisionSupersedeHasLink(links, "dec-old", "supersedes") {
		t.Fatalf("successor links = %#v", links)
	}
}

func TestRunDecisionSupersedeFailsClosedWithoutReason(t *testing.T) {
	fixture := newCheckTestProject(t)
	restoreRoot := enterTestProjectRoot(t, fixture.root)
	defer restoreRoot()
	seedDecisionSupersedeArtifact(t, fixture, "dec-old", artifact.StatusActive)
	seedDecisionSupersedeArtifact(t, fixture, "dec-new", artifact.StatusActive)
	restoreFlags := stubDecisionSupersedeFlags("dec-new", "", false)
	defer restoreFlags()

	err := runDecisionSupersede(&cobra.Command{}, []string{"dec-old"})
	if err == nil || !strings.Contains(err.Error(), "--reason is required") {
		t.Fatalf("error = %v", err)
	}
	oldDecision, getErr := fixture.store.Get(context.Background(), "dec-old")
	if getErr != nil {
		t.Fatal(getErr)
	}
	if oldDecision.Meta.Status != artifact.StatusActive {
		t.Fatalf("status changed after rejected request: %s", oldDecision.Meta.Status)
	}
}

func TestRunDecisionSupersedeRejectsTerminalSuccessor(t *testing.T) {
	fixture := newCheckTestProject(t)
	restoreRoot := enterTestProjectRoot(t, fixture.root)
	defer restoreRoot()
	seedDecisionSupersedeArtifact(t, fixture, "dec-old", artifact.StatusActive)
	seedDecisionSupersedeArtifact(t, fixture, "dec-new", artifact.StatusSuperseded)
	restoreFlags := stubDecisionSupersedeFlags("dec-new", "Replace the old decision.", false)
	defer restoreFlags()

	err := runDecisionSupersede(&cobra.Command{}, []string{"dec-old"})
	if err == nil || !strings.Contains(err.Error(), "successor dec-new status is superseded") {
		t.Fatalf("error = %v", err)
	}
	oldDecision, getErr := fixture.store.Get(context.Background(), "dec-old")
	if getErr != nil {
		t.Fatal(getErr)
	}
	if oldDecision.Meta.Status != artifact.StatusActive {
		t.Fatalf("status changed after rejected successor: %s", oldDecision.Meta.Status)
	}
}

func TestDecisionSupersedeCommandHasNoMCPAlias(t *testing.T) {
	if decisionSupersedeCmd.Use != "supersede OLD_DECISION --new NEW_DECISION --reason REASON" {
		t.Fatalf("unexpected use: %s", decisionSupersedeCmd.Use)
	}
	if !strings.Contains(decisionSupersedeCmd.Long, "There is no MCP alias") {
		t.Fatalf("manual-only boundary missing:\n%s", decisionSupersedeCmd.Long)
	}
}

func seedDecisionSupersedeArtifact(
	t *testing.T,
	fixture checkTestProject,
	ref string,
	status artifact.Status,
) {
	t.Helper()
	err := fixture.store.Create(context.Background(), &artifact.Artifact{
		Meta: artifact.Meta{
			ID:     ref,
			Kind:   artifact.KindDecisionRecord,
			Title:  ref,
			Status: status,
		},
		Body: ref,
	})
	if err != nil {
		t.Fatalf("seed %s: %v", ref, err)
	}
}

func stubDecisionSupersedeFlags(newRef string, reason string, jsonOutput bool) func() {
	oldNewRef := decisionSupersedeNewRef
	oldReason := decisionSupersedeReason
	oldJSON := decisionSupersedeJSON
	decisionSupersedeNewRef = newRef
	decisionSupersedeReason = reason
	decisionSupersedeJSON = jsonOutput
	return func() {
		decisionSupersedeNewRef = oldNewRef
		decisionSupersedeReason = oldReason
		decisionSupersedeJSON = oldJSON
	}
}

func decisionSupersedeHasLink(links []artifact.Link, ref string, relation string) bool {
	for _, link := range links {
		if link.Ref == ref && link.Type == relation {
			return true
		}
	}
	return false
}
