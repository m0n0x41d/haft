package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/overseer"
	"github.com/m0n0x41d/haft/internal/project"
)

func TestBuildOverseerPacket_IncludesChangedGovernedFile(t *testing.T) {
	fixture := newCheckTestProject(t)
	setupOverseerGitRepo(t, fixture.root)

	writeFile(t, filepath.Join(fixture.root, "governed.go"), "package main\n\nfunc governed() string { return \"base\" }\n")
	git(t, fixture.root, "add", "governed.go")
	git(t, fixture.root, "commit", "-m", "base")

	decision := mustCreateDecision(t, fixture, artifact.DecideInput{
		SelectedTitle:   "Govern governed.go",
		WhySelected:     "The overseer packet test needs one affected decision.",
		SelectionPolicy: "Prefer a single governed file for stable packet output.",
		CounterArgument: "A synthetic decision may miss real graph complexity.",
		WeakestLink:     "The packet must join changed files back to affected decisions.",
		Invariants:      []string{"governed() behavior remains explicit."},
		AffectedFiles:   []string{"governed.go"},
		WhyNotOthers: []artifact.RejectionReason{{
			Variant: "Ungoverned file",
			Reason:  "Would not exercise governance joins.",
		}},
		Rollback: &artifact.RollbackSpec{
			Triggers: []string{"The packet no longer includes governed.go."},
		},
	})

	before := countArtifacts(t, fixture)

	writeFile(t, filepath.Join(fixture.root, "governed.go"), "package main\n\nfunc governed() string { return \"changed\" }\n")
	git(t, fixture.root, "add", "governed.go")
	git(t, fixture.root, "commit", "-m", "change governed file")

	packet, err := buildOverseerPacket(context.Background(), fixture.store, fixture.root, "HEAD", "test")
	if err != nil {
		t.Fatalf("buildOverseerPacket returned error: %v", err)
	}

	after := countArtifacts(t, fixture)
	if after != before {
		t.Fatalf("artifact count changed: before=%d after=%d", before, after)
	}

	if got := len(packet.ChangedFiles); got != 1 {
		t.Fatalf("changed files = %d, want 1", got)
	}
	if packet.ChangedFiles[0].Path != "governed.go" {
		t.Fatalf("changed path = %q, want governed.go", packet.ChangedFiles[0].Path)
	}
	if packet.ChangedFiles[0].Governance.ModuleState != "covered" {
		t.Fatalf("module state = %q, want covered", packet.ChangedFiles[0].Governance.ModuleState)
	}
	if !packetHasDecision(packet, decision.Meta.ID) {
		t.Fatalf("packet missing affected decision %s: %+v", decision.Meta.ID, packet.ChangedFiles[0].Governance.AffectedDecisions)
	}
	if !containsString(packet.ReviewRequest.Modes, "invariant_conformance") {
		t.Fatalf("review modes = %v, want invariant_conformance", packet.ReviewRequest.Modes)
	}
	if packet.ReviewRequest.Authority != "advisory_only" {
		t.Fatalf("authority = %q, want advisory_only", packet.ReviewRequest.Authority)
	}
}

func TestBuildOverseerPacket_IncludesRefreshDueGovernedDecision(t *testing.T) {
	fixture := newCheckTestProject(t)
	setupOverseerGitRepo(t, fixture.root)

	writeFile(t, filepath.Join(fixture.root, "governed.go"), "package main\n\nfunc governed() string { return \"base\" }\n")
	git(t, fixture.root, "add", "governed.go")
	git(t, fixture.root, "commit", "-m", "base")

	decision := mustCreateDecision(t, fixture, artifact.DecideInput{
		SelectedTitle:   "Refresh due governed file",
		WhySelected:     "The overseer packet must still scope refresh_due decisions.",
		SelectionPolicy: "Prefer the decision governing the changed file.",
		CounterArgument: "Refresh-due decisions still need review visibility.",
		WeakestLink:     "The packet must not treat refresh_due as unrelated debt.",
		Invariants:      []string{"governed() behavior remains explicit."},
		AffectedFiles:   []string{"governed.go"},
		WhyNotOthers: []artifact.RejectionReason{{
			Variant: "Ignore refresh_due",
			Reason:  "Would suppress the debt that most needs review.",
		}},
		Rollback: &artifact.RollbackSpec{
			Triggers: []string{"The packet no longer includes refresh_due governed decisions."},
		},
	})
	if _, _, err := artifact.ReopenDecision(
		context.Background(),
		fixture.store,
		fixture.haftDir,
		decision.Meta.ID,
		"Refresh needed before accepting changed governed code.",
	); err != nil {
		t.Fatalf("reopen decision: %v", err)
	}

	writeFile(t, filepath.Join(fixture.root, "governed.go"), "package main\n\nfunc governed() string { return \"changed\" }\n")
	git(t, fixture.root, "add", "governed.go")
	git(t, fixture.root, "commit", "-m", "change refresh due governed file")

	packet, err := buildOverseerPacket(context.Background(), fixture.store, fixture.root, "HEAD", "test")
	if err != nil {
		t.Fatalf("buildOverseerPacket returned error: %v", err)
	}

	if !packetHasDecision(packet, decision.Meta.ID) {
		t.Fatalf("packet missing refresh_due affected decision %s: %+v", decision.Meta.ID, packet.ChangedFiles[0].Governance.AffectedDecisions)
	}
	if len(packet.DeterministicFindings.Stale) == 0 {
		t.Fatalf("packet suppressed scoped stale finding for refresh_due decision: %+v", packet.DeterministicFindings)
	}
}

func TestMapOverseerGovernanceScopesSpecHealthToAffectedSections(t *testing.T) {
	report := checkReport{
		SpecHealth: []project.SpecCheckFinding{
			{
				Code:      "spec_section_stale",
				Path:      ".haft/specs/target-system.md",
				SectionID: "TS.scope.001",
				Message:   "spec section is stale",
			},
			{
				Code:      "spec_section_stale",
				Path:      ".haft/specs/enabling-system.md",
				SectionID: "ES.unrelated.001",
				Message:   "unrelated spec section is stale",
			},
		},
	}
	changedFiles := []overseer.ChangedFile{{
		Path: "governed.go",
		Governance: overseer.ChangedFileGovernance{
			AffectedSpecSections: []string{"TS.scope.001"},
		},
	}}

	governance := mapOverseerGovernance(report, changedFiles, map[string]bool{})

	if len(governance.SpecHealth) != 1 {
		t.Fatalf("spec health findings = %+v, want one scoped finding", governance.SpecHealth)
	}
	if governance.SpecHealth[0].ID != "TS.scope.001" {
		t.Fatalf("scoped spec health ID = %q, want TS.scope.001", governance.SpecHealth[0].ID)
	}
	if governance.Suppressed.UnrelatedSpecHealth != 1 {
		t.Fatalf("unrelated spec health count = %d, want 1", governance.Suppressed.UnrelatedSpecHealth)
	}
}

func TestRunOverseerPacket_JSON(t *testing.T) {
	fixture := newCheckTestProject(t)
	setupOverseerGitRepo(t, fixture.root)

	writeFile(t, filepath.Join(fixture.root, "README.md"), "# test\n")
	git(t, fixture.root, "add", "README.md")
	git(t, fixture.root, "commit", "-m", "docs")

	restore := enterTestProjectRoot(t, fixture.root)
	defer restore()

	restoreFlags := stubOverseerPacketFlags(t, "HEAD", true)
	defer restoreFlags()

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)

	if err := runOverseerPacket(cmd, nil); err != nil {
		t.Fatalf("runOverseerPacket returned error: %v", err)
	}

	var packet overseer.Packet
	if err := json.Unmarshal(output.Bytes(), &packet); err != nil {
		t.Fatalf("decode packet JSON: %v\nraw: %s", err, output.String())
	}

	if packet.SchemaVersion != overseer.ReviewPacketSchemaVersion {
		t.Fatalf("schema version = %q, want %q", packet.SchemaVersion, overseer.ReviewPacketSchemaVersion)
	}
	if packet.PacketID == "" {
		t.Fatalf("packet ID is empty")
	}
	if packet.Risk.LLMReview != "off" {
		t.Fatalf("docs-only llm_review = %q, want off", packet.Risk.LLMReview)
	}
}

func TestOverseerInitFlagExistsAndIsOptIn(t *testing.T) {
	if initCmd.Flags().Lookup("overseer") == nil {
		t.Fatalf("expected opt-in --overseer flag")
	}
	if initCmd.Flags().Lookup("no-file-instructions") == nil {
		t.Fatalf("expected generic --no-file-instructions flag")
	}
	if _, _, err := overseerCmd.Find([]string{"init"}); err != nil {
		t.Fatalf("expected overseer init command to be registered")
	}
	haftDir := filepath.Join(t.TempDir(), ".haft")
	if err := createDirectoryStructure(haftDir); err != nil {
		t.Fatalf("createDirectoryStructure returned error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(haftDir, "overseer")); !os.IsNotExist(err) {
		t.Fatalf("overseer directory should not exist without opt-in; err=%v", err)
	}
}

func TestRunOverseerInitAutoDetectsCodexProject(t *testing.T) {
	fixture := newCheckTestProject(t)
	setupOverseerGitRepo(t, fixture.root)

	codexConfigPath := filepath.Join(fixture.root, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(codexConfigPath), 0o755); err != nil {
		t.Fatalf("create codex config dir: %v", err)
	}
	if err := os.WriteFile(codexConfigPath, []byte("[mcp_servers.haft]\n"), 0o644); err != nil {
		t.Fatalf("write codex config: %v", err)
	}

	restore := enterTestProjectRoot(t, fixture.root)
	defer restore()

	restoreFlags := stubOverseerInitFlags(t, overseer.ReviewerAuto, "", false, 45, false)
	defer restoreFlags()

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)
	if err := runOverseerInit(cmd, nil); err != nil {
		t.Fatalf("runOverseerInit returned error: %v", err)
	}
	if !strings.Contains(output.String(), "reviewer=codex") {
		t.Fatalf("init summary missing codex reviewer:\n%s", output.String())
	}

	config, err := overseer.LoadConfig(fixture.root)
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}
	if config.ReviewerAgent != overseer.ReviewerCodex {
		t.Fatalf("reviewer_agent = %q, want codex", config.ReviewerAgent)
	}
	if config.LLMReview != "on" || !config.ReviewOnHook {
		t.Fatalf("codex overseer init should enable hook review: %+v", config)
	}
	hook, err := os.ReadFile(filepath.Join(fixture.root, ".git", "hooks", "post-commit"))
	if err != nil {
		t.Fatalf("read post-commit hook: %v", err)
	}
	if !strings.Contains(string(hook), "haft overseer hook --commit HEAD --async || true") {
		t.Fatalf("hook missing overseer command:\n%s", string(hook))
	}
	if _, err := os.Stat(filepath.Join(fixture.root, "CLAUDE.md")); !os.IsNotExist(err) {
		t.Fatalf("overseer init should not create instruction files; err=%v", err)
	}
}

func TestRunOverseerRunStoresLatestAndReminder(t *testing.T) {
	fixture := newCheckTestProject(t)
	setupOverseerGitRepo(t, fixture.root)

	initPath := filepath.Join(fixture.root, "internal", "cli", "init.go")
	if err := os.MkdirAll(filepath.Dir(initPath), 0o755); err != nil {
		t.Fatalf("create init dir: %v", err)
	}
	writeFile(t, initPath, "package cli\n\nfunc initSurface() {}\n")
	git(t, fixture.root, "add", "internal/cli/init.go")
	git(t, fixture.root, "commit", "-m", "touch init surface")

	restore := enterTestProjectRoot(t, fixture.root)
	defer restore()

	restoreFlags := stubOverseerRunFlags(t, "HEAD", false, false)
	defer restoreFlags()

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)

	if err := runOverseerRun(cmd, nil); err != nil {
		t.Fatalf("runOverseerRun returned error: %v", err)
	}
	if !strings.Contains(output.String(), "haft overseer: stored") {
		t.Fatalf("run summary missing stored message:\n%s", output.String())
	}

	stored, err := overseer.LoadLatestRun(fixture.root)
	if err != nil {
		t.Fatalf("LoadLatestRun returned error: %v", err)
	}
	if stored.Run.Verdict != "packet_generated" {
		t.Fatalf("verdict = %q, want packet_generated", stored.Run.Verdict)
	}
	if stored.Packet.Risk.Level != "medium" && stored.Packet.Risk.Level != "high" {
		t.Fatalf("risk level = %q, want medium/high", stored.Packet.Risk.Level)
	}

	restoreRemind := stubOverseerRemindFlags(t, false)
	defer restoreRemind()

	var reminder bytes.Buffer
	remindCmd := &cobra.Command{}
	remindCmd.SetOut(&reminder)
	if err := runOverseerRemind(remindCmd, nil); err != nil {
		t.Fatalf("runOverseerRemind returned error: %v", err)
	}
	if !strings.Contains(reminder.String(), "haft overseer show") {
		t.Fatalf("reminder output missing show command:\n%s", reminder.String())
	}
}

func TestHandleQuintQueryStatusPrependsOverseerSignals(t *testing.T) {
	fixture := newCheckTestProject(t)
	packet, err := overseer.BuildPacket(overseer.BuildInput{
		Producer: overseer.DefaultProducer("test"),
		Subject: overseer.Subject{
			Kind:     "commit",
			Ref:      "HEAD",
			SHA:      "abc123",
			DiffHash: "sha256:diff",
		},
		RepoState: overseer.RepoState{GitRoot: fixture.root, Branch: "main"},
		ChangedFiles: []overseer.ChangedFile{{
			Path:   "internal/cli/init.go",
			Status: "modified",
		}},
		Budget: overseer.DefaultContextBudget(),
	})
	if err != nil {
		t.Fatalf("BuildPacket returned error: %v", err)
	}
	run := overseer.NewDeterministicReviewRun(packet, "2026-06-09T00:00:00Z")
	if err := overseer.StoreRun(fixture.root, packet, run); err != nil {
		t.Fatalf("StoreRun returned error: %v", err)
	}

	result, err := handleQuintQuery(context.Background(), fixture.store, nil, fixture.haftDir, map[string]any{
		"action": "status",
	})
	if err != nil {
		t.Fatalf("handleQuintQuery(status) returned error: %v", err)
	}
	if !strings.HasPrefix(result, "## Overseer Signals") {
		t.Fatalf("status did not start with overseer signals:\n%s", result)
	}
	if !strings.Contains(result, "## Haft Status") {
		t.Fatalf("status missing normal Haft dashboard:\n%s", result)
	}
}

func TestRunOverseerMaintainStoresMaintenanceRun(t *testing.T) {
	fixture := newCheckTestProject(t)
	seedGovernanceDebt(t, fixture)

	restore := enterTestProjectRoot(t, fixture.root)
	defer restore()

	restoreFlags := stubOverseerMaintainFlags(t, false)
	defer restoreFlags()

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)

	if err := runOverseerMaintain(cmd, nil); err != nil {
		t.Fatalf("runOverseerMaintain returned error: %v", err)
	}
	if !strings.Contains(output.String(), "maintenance stored") {
		t.Fatalf("maintain summary missing stored message:\n%s", output.String())
	}

	run, err := overseer.LoadLatestMaintenanceRun(fixture.root)
	if err != nil {
		t.Fatalf("LoadLatestMaintenanceRun returned error: %v", err)
	}
	if run.MaintenanceID == "" {
		t.Fatalf("maintenance id is empty")
	}
	if run.Authority.Status != "advisory_only" {
		t.Fatalf("authority = %q, want advisory_only", run.Authority.Status)
	}
	if run.Summary.SignalCount == 0 {
		t.Fatalf("expected maintenance signals from seeded governance debt, got %+v", run.Summary)
	}
}

func TestRunOverseerHookRunsConfiguredReviewer(t *testing.T) {
	fixture := newCheckTestProject(t)
	setupOverseerGitRepo(t, fixture.root)

	initPath := filepath.Join(fixture.root, "internal", "cli", "init.go")
	if err := os.MkdirAll(filepath.Dir(initPath), 0o755); err != nil {
		t.Fatalf("create init dir: %v", err)
	}
	writeFile(t, initPath, "package cli\n\nfunc initSurface() {}\n")
	git(t, fixture.root, "add", "internal/cli/init.go")
	git(t, fixture.root, "commit", "-m", "touch init surface")

	config := overseer.DefaultConfig()
	config.LLMReview = "on"
	config.ReviewOnHook = true
	config.ReviewerAgent = "command"
	config.ReviewerCommand = `printf '%s' '{"findings":[{"id":"ofind-hook","severity":"high","confidence":"high","claim":"Hook reviewer found a governance violation.","concrete_harm":"The authoring agent could miss the failed review."}]}' > "$HAFT_OVERSEER_RESULT_FILE"`
	if err := overseer.SaveConfig(fixture.root, config); err != nil {
		t.Fatalf("SaveConfig returned error: %v", err)
	}

	restore := enterTestProjectRoot(t, fixture.root)
	defer restore()

	restoreFlags := stubOverseerHookFlags(t, "HEAD", false, false, false)
	defer restoreFlags()

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)
	if err := runOverseerHook(cmd, nil); err != nil {
		t.Fatalf("runOverseerHook returned error: %v", err)
	}
	if !strings.Contains(output.String(), "review: reviewed") {
		t.Fatalf("hook summary missing reviewed state:\n%s", output.String())
	}

	stored, err := overseer.LoadLatestRun(fixture.root)
	if err != nil {
		t.Fatalf("LoadLatestRun returned error: %v", err)
	}
	if stored.Run.Verdict != "findings_recorded" {
		t.Fatalf("verdict = %q, want findings_recorded", stored.Run.Verdict)
	}
	if len(overseer.UnresolvedFindings(stored.Run)) != 1 {
		t.Fatalf("unresolved findings = %d, want 1", len(overseer.UnresolvedFindings(stored.Run)))
	}
	if _, err := overseer.LoadLatestMaintenanceRun(fixture.root); err != nil {
		t.Fatalf("LoadLatestMaintenanceRun returned error: %v", err)
	}
}

func TestRunOverseerHookAsyncSchedulesConfiguredReviewer(t *testing.T) {
	fixture := newCheckTestProject(t)
	setupOverseerGitRepo(t, fixture.root)

	initPath := filepath.Join(fixture.root, "internal", "cli", "init.go")
	if err := os.MkdirAll(filepath.Dir(initPath), 0o755); err != nil {
		t.Fatalf("create init dir: %v", err)
	}
	writeFile(t, initPath, "package cli\n\nfunc initSurface() {}\n")
	git(t, fixture.root, "add", "internal/cli/init.go")
	git(t, fixture.root, "commit", "-m", "touch init surface")

	config := overseer.DefaultConfig()
	config.LLMReview = "on"
	config.ReviewOnHook = true
	config.ReviewerAgent = "command"
	config.ReviewerCommand = `printf '%s' '{"findings":[{"id":"ofind-hook","severity":"high","confidence":"high","claim":"Hook reviewer found a governance violation.","concrete_harm":"The authoring agent could miss the failed review."}]}' > "$HAFT_OVERSEER_RESULT_FILE"`
	if err := overseer.SaveConfig(fixture.root, config); err != nil {
		t.Fatalf("SaveConfig returned error: %v", err)
	}

	restore := enterTestProjectRoot(t, fixture.root)
	defer restore()

	restoreFlags := stubOverseerHookFlags(t, "HEAD", false, false, true)
	defer restoreFlags()

	previousStarter := startOverseerDaemon
	startOverseerDaemon = func(projectRoot string) (overseerDaemonStartResult, error) {
		if projectRoot != fixture.root {
			t.Fatalf("projectRoot = %q, want %q", projectRoot, fixture.root)
		}
		return overseerDaemonStartResult{
			Started: true,
			Running: true,
			PID:     12345,
			LogPath: filepath.Join(fixture.root, ".haft", "overseer", "logs", "daemon.log"),
		}, nil
	}
	defer func() { startOverseerDaemon = previousStarter }()

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)
	if err := runOverseerHook(cmd, nil); err != nil {
		t.Fatalf("runOverseerHook returned error: %v", err)
	}
	if !strings.Contains(output.String(), "review: queued job=ojob-") ||
		!strings.Contains(output.String(), "daemon_pid=12345") {
		t.Fatalf("hook summary missing daemon queue schedule:\n%s", output.String())
	}

	stored, err := overseer.LoadLatestRun(fixture.root)
	if err != nil {
		t.Fatalf("LoadLatestRun returned error: %v", err)
	}
	if stored.Run.Verdict != "packet_generated" {
		t.Fatalf("async hook should not run reviewer inline; verdict = %q", stored.Run.Verdict)
	}
	jobID := "ojob-" + strings.TrimPrefix(stored.Run.ReviewRunID, "rrun-")
	job, err := overseer.LoadReviewJob(fixture.root, jobID)
	if err != nil {
		t.Fatalf("LoadReviewJob returned error: %v", err)
	}
	if job.ReviewRunID != stored.Run.ReviewRunID || job.Status != overseer.JobStatusPending {
		t.Fatalf("queued job = %+v, want pending job for %s", job, stored.Run.ReviewRunID)
	}
	reminder := overseer.BuildReminder(stored)
	if !reminder.HasReminder {
		t.Fatalf("pending packet should still remind the agent: %+v", reminder)
	}
}

func TestRunOverseerDaemonLoopProcessesQueuedReviewJob(t *testing.T) {
	fixture := newCheckTestProject(t)
	setupOverseerGitRepo(t, fixture.root)

	initPath := filepath.Join(fixture.root, "internal", "cli", "init.go")
	if err := os.MkdirAll(filepath.Dir(initPath), 0o755); err != nil {
		t.Fatalf("create init dir: %v", err)
	}
	writeFile(t, initPath, "package cli\n\nfunc initSurface() {}\n")
	git(t, fixture.root, "add", "internal/cli/init.go")
	git(t, fixture.root, "commit", "-m", "touch init surface")

	config := overseer.DefaultConfig()
	config.LLMReview = "on"
	config.ReviewOnHook = true
	config.ReviewerAgent = "command"
	config.ReviewerCommand = `printf '%s' '{"findings":[{"id":"ofind-daemon","severity":"high","confidence":"high","claim":"Daemon reviewer found a governance violation.","concrete_harm":"The authoring agent could miss the queued review."}]}' > "$HAFT_OVERSEER_RESULT_FILE"`
	if err := overseer.SaveConfig(fixture.root, config); err != nil {
		t.Fatalf("SaveConfig returned error: %v", err)
	}

	stored, _, err := prepareOverseerHookRun(context.Background(), fixture.root, fixture.store, "HEAD")
	if err != nil {
		t.Fatalf("prepareOverseerHookRun returned error: %v", err)
	}
	job, err := overseer.EnqueueReviewJob(fixture.root, stored, "2026-06-10T00:00:00Z")
	if err != nil {
		t.Fatalf("EnqueueReviewJob returned error: %v", err)
	}

	var output bytes.Buffer
	summary, err := runOverseerDaemonLoop(
		context.Background(),
		fixture.root,
		config,
		10*time.Millisecond,
		10*time.Millisecond,
		&output,
	)
	if err != nil {
		t.Fatalf("runOverseerDaemonLoop returned error: %v", err)
	}
	if summary.Done != 1 || summary.Pending != 0 {
		t.Fatalf("queue summary = %+v, want one done job", summary)
	}

	stored, err = overseer.LoadLatestRun(fixture.root)
	if err != nil {
		t.Fatalf("LoadLatestRun returned error: %v", err)
	}
	if stored.Run.Verdict != "findings_recorded" {
		t.Fatalf("verdict = %q, want findings_recorded", stored.Run.Verdict)
	}
	if len(overseer.UnresolvedFindings(stored.Run)) != 1 {
		t.Fatalf("unresolved findings = %d, want 1", len(overseer.UnresolvedFindings(stored.Run)))
	}

	job, err = overseer.LoadReviewJob(fixture.root, job.JobID)
	if err != nil {
		t.Fatalf("LoadReviewJob returned error: %v", err)
	}
	if job.Status != overseer.JobStatusDone {
		t.Fatalf("job status = %q, want done", job.Status)
	}
}

func TestProcessOverseerReviewJobPreservesCanceledJobForNextDaemon(t *testing.T) {
	fixture := newCheckTestProject(t)
	setupOverseerGitRepo(t, fixture.root)

	initPath := filepath.Join(fixture.root, "internal", "cli", "init.go")
	if err := os.MkdirAll(filepath.Dir(initPath), 0o755); err != nil {
		t.Fatalf("create init dir: %v", err)
	}
	writeFile(t, initPath, "package cli\n\nfunc initSurface() {}\n")
	git(t, fixture.root, "add", "internal/cli/init.go")
	git(t, fixture.root, "commit", "-m", "touch init surface")

	config := overseer.DefaultConfig()
	config.LLMReview = "on"
	config.ReviewOnHook = true
	config.ReviewerAgent = "command"
	config.ReviewerCommand = "sleep 5"
	config.ReviewTimeoutSeconds = 30
	if err := overseer.SaveConfig(fixture.root, config); err != nil {
		t.Fatalf("SaveConfig returned error: %v", err)
	}

	stored, _, err := prepareOverseerHookRun(context.Background(), fixture.root, fixture.store, "HEAD")
	if err != nil {
		t.Fatalf("prepareOverseerHookRun returned error: %v", err)
	}
	job, err := overseer.EnqueueReviewJob(fixture.root, stored, "2026-06-10T00:00:00Z")
	if err != nil {
		t.Fatalf("EnqueueReviewJob returned error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := processOverseerReviewJob(ctx, fixture.root, config, job, nil); err != nil {
		t.Fatalf("processOverseerReviewJob returned error: %v", err)
	}

	job, err = overseer.LoadReviewJob(fixture.root, job.JobID)
	if err != nil {
		t.Fatalf("LoadReviewJob returned error: %v", err)
	}
	if job.Status != overseer.JobStatusRunning {
		t.Fatalf("job status = %q, want running for restart requeue", job.Status)
	}
	if job.Attempts != 1 {
		t.Fatalf("attempts = %d, want 1", job.Attempts)
	}
	if !strings.Contains(job.LastError, "daemon cancellation") {
		t.Fatalf("last_error = %q, want cancellation preservation", job.LastError)
	}
}

func TestRunOverseerIngestAndDispositionDriveStatusSignals(t *testing.T) {
	fixture := newCheckTestProject(t)
	packet, err := overseer.BuildPacket(overseer.BuildInput{
		Producer: overseer.DefaultProducer("test"),
		Subject: overseer.Subject{
			Kind:     "commit",
			Ref:      "HEAD",
			SHA:      "abc123",
			DiffHash: "sha256:diff",
		},
		RepoState: overseer.RepoState{GitRoot: fixture.root, Branch: "main"},
		ChangedFiles: []overseer.ChangedFile{{
			Path:   "internal/cli/init.go",
			Status: "modified",
		}},
		Budget: overseer.DefaultContextBudget(),
	})
	if err != nil {
		t.Fatalf("BuildPacket returned error: %v", err)
	}
	run := overseer.NewDeterministicReviewRun(packet, "2026-06-09T00:00:00Z")
	if err := overseer.StoreRun(fixture.root, packet, run); err != nil {
		t.Fatalf("StoreRun returned error: %v", err)
	}

	inputPath := filepath.Join(fixture.root, "review-result.json")
	if err := os.WriteFile(inputPath, []byte(`{
		"reviewer": {"agent": "codex-reviewer"},
		"findings": [{
			"id": "ofind-cli",
			"severity": "high",
			"confidence": "high",
			"category": "invariant_conformance",
			"claim": "The status surface can miss unresolved reviewer findings.",
			"concrete_harm": "Agents may claim clean work without closing a finding.",
			"counts_for_r_eff": true
		}]
	}`), 0o644); err != nil {
		t.Fatalf("write review input: %v", err)
	}

	restore := enterTestProjectRoot(t, fixture.root)
	defer restore()

	restoreIngest := stubOverseerIngestFlags(t, "latest", inputPath, false)
	defer restoreIngest()

	var ingestOutput bytes.Buffer
	ingestCmd := &cobra.Command{}
	ingestCmd.SetOut(&ingestOutput)
	if err := runOverseerIngest(ingestCmd, nil); err != nil {
		t.Fatalf("runOverseerIngest returned error: %v", err)
	}
	if !strings.Contains(ingestOutput.String(), "ingested review") {
		t.Fatalf("ingest summary missing marker:\n%s", ingestOutput.String())
	}

	statusWithFinding, err := handleQuintQuery(context.Background(), fixture.store, nil, fixture.haftDir, map[string]any{
		"action": "status",
	})
	if err != nil {
		t.Fatalf("handleQuintQuery(status) after ingest: %v", err)
	}
	if !strings.Contains(statusWithFinding, "The status surface can miss unresolved reviewer findings") {
		t.Fatalf("status missing ingested finding:\n%s", statusWithFinding)
	}

	restoreDisposition := stubOverseerDispositionFlags(t, "latest", "fixed_by_commit", "agent", "fixed in test", "abc123", false)
	defer restoreDisposition()

	var dispositionOutput bytes.Buffer
	dispositionCmd := &cobra.Command{}
	dispositionCmd.SetOut(&dispositionOutput)
	if err := runOverseerDisposition(dispositionCmd, []string{"ofind-cli"}); err != nil {
		t.Fatalf("runOverseerDisposition returned error: %v", err)
	}
	if !strings.Contains(dispositionOutput.String(), "unresolved: 0") {
		t.Fatalf("disposition summary missing unresolved zero:\n%s", dispositionOutput.String())
	}

	statusAfterDisposition, err := handleQuintQuery(context.Background(), fixture.store, nil, fixture.haftDir, map[string]any{
		"action": "status",
	})
	if err != nil {
		t.Fatalf("handleQuintQuery(status) after disposition: %v", err)
	}
	if strings.Contains(statusAfterDisposition, "The status surface can miss unresolved reviewer findings") {
		t.Fatalf("closed finding still present in status:\n%s", statusAfterDisposition)
	}
}

func setupOverseerGitRepo(t *testing.T, root string) {
	t.Helper()

	git(t, root, "init")
	git(t, root, "config", "user.email", "test@example.com")
	git(t, root, "config", "user.name", "Test User")
}

func git(t *testing.T, root string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %s: %v", strings.Join(args, " "), strings.TrimSpace(string(output)), err)
	}
	return strings.TrimSpace(string(output))
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func countArtifacts(t *testing.T, fixture checkTestProject) int {
	t.Helper()

	artifacts, err := fixture.store.ListByKind(context.Background(), "", 0)
	if err != nil {
		t.Fatalf("list artifacts: %v", err)
	}
	return len(artifacts)
}

func packetHasDecision(packet overseer.Packet, decisionID string) bool {
	for _, file := range packet.ChangedFiles {
		for _, decision := range file.Governance.AffectedDecisions {
			if decision.ID == decisionID {
				return true
			}
		}
	}
	return false
}

func containsString(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func stubOverseerPacketFlags(t *testing.T, commit string, jsonFlag bool) func() {
	t.Helper()

	previousCommit := overseerPacketCommit
	previousJSON := overseerPacketJSON
	overseerPacketCommit = commit
	overseerPacketJSON = jsonFlag

	return func() {
		overseerPacketCommit = previousCommit
		overseerPacketJSON = previousJSON
	}
}

func stubOverseerRunFlags(t *testing.T, commit string, jsonFlag bool, quiet bool) func() {
	t.Helper()

	previousCommit := overseerRunCommit
	previousJSON := overseerRunJSON
	previousQuiet := overseerRunQuiet
	overseerRunCommit = commit
	overseerRunJSON = jsonFlag
	overseerRunQuiet = quiet

	return func() {
		overseerRunCommit = previousCommit
		overseerRunJSON = previousJSON
		overseerRunQuiet = previousQuiet
	}
}

func stubOverseerInitFlags(
	t *testing.T,
	agent string,
	command string,
	reviewOnHook bool,
	timeout int,
	jsonFlag bool,
) func() {
	t.Helper()

	previousAgent := overseerInitAgent
	previousCommand := overseerInitCommand
	previousReviewOnHook := overseerInitReviewOnHook
	previousTimeout := overseerInitTimeout
	previousJSON := overseerInitJSON
	overseerInitAgent = agent
	overseerInitCommand = command
	overseerInitReviewOnHook = reviewOnHook
	overseerInitTimeout = timeout
	overseerInitJSON = jsonFlag

	return func() {
		overseerInitAgent = previousAgent
		overseerInitCommand = previousCommand
		overseerInitReviewOnHook = previousReviewOnHook
		overseerInitTimeout = previousTimeout
		overseerInitJSON = previousJSON
	}
}

func stubOverseerRemindFlags(t *testing.T, jsonFlag bool) func() {
	t.Helper()

	previousJSON := overseerRemindJSON
	overseerRemindJSON = jsonFlag

	return func() {
		overseerRemindJSON = previousJSON
	}
}

func stubOverseerMaintainFlags(t *testing.T, jsonFlag bool) func() {
	t.Helper()

	previousJSON := overseerMaintainJSON
	overseerMaintainJSON = jsonFlag

	return func() {
		overseerMaintainJSON = previousJSON
	}
}

func stubOverseerHookFlags(t *testing.T, commit string, jsonFlag bool, quiet bool, async bool) func() {
	t.Helper()

	previousCommit := overseerHookCommit
	previousJSON := overseerHookJSON
	previousQuiet := overseerHookQuiet
	previousAsync := overseerHookAsync
	overseerHookCommit = commit
	overseerHookJSON = jsonFlag
	overseerHookQuiet = quiet
	overseerHookAsync = async

	return func() {
		overseerHookCommit = previousCommit
		overseerHookJSON = previousJSON
		overseerHookQuiet = previousQuiet
		overseerHookAsync = previousAsync
	}
}

func stubOverseerIngestFlags(t *testing.T, run string, inputFile string, jsonFlag bool) func() {
	t.Helper()

	previousRun := overseerIngestRun
	previousFile := overseerIngestFile
	previousJSON := overseerIngestJSON
	overseerIngestRun = run
	overseerIngestFile = inputFile
	overseerIngestJSON = jsonFlag

	return func() {
		overseerIngestRun = previousRun
		overseerIngestFile = previousFile
		overseerIngestJSON = previousJSON
	}
}

func stubOverseerDispositionFlags(
	t *testing.T,
	run string,
	status string,
	actor string,
	reason string,
	commit string,
	jsonFlag bool,
) func() {
	t.Helper()

	previousRun := overseerDispositionRun
	previousStatus := overseerDispositionStatus
	previousActor := overseerDispositionActor
	previousReason := overseerDispositionReason
	previousCommit := overseerDispositionCommit
	previousJSON := overseerDispositionJSON
	overseerDispositionRun = run
	overseerDispositionStatus = status
	overseerDispositionActor = actor
	overseerDispositionReason = reason
	overseerDispositionCommit = commit
	overseerDispositionJSON = jsonFlag

	return func() {
		overseerDispositionRun = previousRun
		overseerDispositionStatus = previousStatus
		overseerDispositionActor = previousActor
		overseerDispositionReason = previousReason
		overseerDispositionCommit = previousCommit
		overseerDispositionJSON = previousJSON
	}
}
