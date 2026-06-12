package overseer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

type ReviewerRunResult struct {
	Input      ReviewResultInput
	PromptPath string
	PacketPath string
	ResultPath string
	Stdout     string
	Abstained  bool
}

func RunConfiguredReviewer(
	ctx context.Context,
	projectRoot string,
	config Config,
	stored StoredRun,
) (ReviewerRunResult, error) {
	config = normalizeConfig(config)
	command := strings.TrimSpace(config.ReviewerCommand)
	if command == "" {
		return ReviewerRunResult{}, fmt.Errorf("reviewer_command is required for reviewer_agent=%s", config.ReviewerAgent)
	}

	timeout := time.Duration(config.ReviewTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = time.Duration(DefaultConfig().ReviewTimeoutSeconds) * time.Second
	}

	runDir := filepath.Join(OverseerDir(projectRoot), "reviewer", stored.Run.ReviewRunID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return ReviewerRunResult{}, fmt.Errorf("create reviewer run dir: %w", err)
	}

	packetPath := filepath.Join(runDir, "packet.json")
	promptPath := filepath.Join(runDir, "prompt.md")
	resultPath := filepath.Join(runDir, "review-result.json")
	schemaPath := filepath.Join(runDir, "review-result.schema.json")

	if err := writeJSONFile(packetPath, stored.Packet); err != nil {
		return ReviewerRunResult{}, fmt.Errorf("write reviewer packet: %w", err)
	}
	if err := os.WriteFile(promptPath, []byte(BuildReviewerPrompt(stored, packetPath)), 0o644); err != nil {
		return ReviewerRunResult{}, fmt.Errorf("write reviewer prompt: %w", err)
	}
	if err := os.WriteFile(schemaPath, []byte(ReviewResultJSONSchema()), 0o644); err != nil {
		return ReviewerRunResult{}, fmt.Errorf("write reviewer schema: %w", err)
	}

	env := append(os.Environ(),
		"HAFT_PROJECT_ROOT="+projectRoot,
		"HAFT_OVERSEER_PACKET_FILE="+packetPath,
		"HAFT_OVERSEER_PROMPT_FILE="+promptPath,
		"HAFT_OVERSEER_RESULT_FILE="+resultPath,
		"HAFT_OVERSEER_SCHEMA_FILE="+schemaPath,
		"HAFT_OVERSEER_RUN_ID="+stored.Run.ReviewRunID,
		"HAFT_OVERSEER_PACKET_ID="+stored.Packet.PacketID,
	)

	stdout, err := runReviewerShellCommand(ctx, command, projectRoot, env, timeout)
	if errors.Is(err, context.DeadlineExceeded) {
		return ReviewerRunResult{}, fmt.Errorf("reviewer command timed out after %s: %s", timeout, stdout)
	}
	if err != nil {
		return ReviewerRunResult{}, fmt.Errorf("reviewer command failed: %w: %s", err, stdout)
	}

	input, err := readReviewerResult(resultPath, stdout)
	if err != nil {
		return ReviewerRunResult{}, err
	}
	input.Reviewer = reviewerForConfig(config, input.Reviewer)

	return ReviewerRunResult{
		Input:      input,
		PromptPath: promptPath,
		PacketPath: packetPath,
		ResultPath: resultPath,
		Stdout:     stdout,
	}, nil
}

func runReviewerShellCommand(
	ctx context.Context,
	command string,
	projectRoot string,
	env []string,
	timeout time.Duration,
) (string, error) {
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.Command("/bin/sh", "-c", command)
	cmd.Dir = projectRoot
	cmd.Env = env
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output

	if err := cmd.Start(); err != nil {
		return "", err
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		return strings.TrimSpace(output.String()), err
	case <-execCtx.Done():
		_ = terminateProcessGroup(cmd.Process.Pid, done)
		return strings.TrimSpace(output.String()), execCtx.Err()
	}
}

func terminateProcessGroup(pid int, done <-chan error) error {
	if pid <= 0 {
		return nil
	}

	_ = syscall.Kill(-pid, syscall.SIGTERM)
	select {
	case err := <-done:
		return err
	case <-time.After(2 * time.Second):
	}

	_ = syscall.Kill(-pid, syscall.SIGKILL)
	return <-done
}

func ReviewResultJSONSchema() string {
	return `{
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "reviewer": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "agent": {"type": "string"}
      },
      "required": ["agent"]
    },
    "verdict": {"type": "string"},
    "findings": {
      "type": "array",
      "items": {
        "type": "object",
        "additionalProperties": false,
        "properties": {
          "id": {"type": "string"},
          "severity": {"type": "string", "enum": ["high", "medium", "low", "critical"]},
          "confidence": {"type": "string"},
          "category": {"type": "string"},
          "claim": {"type": "string"},
          "concrete_harm": {"type": "string"},
          "minimal_fix": {"type": "string"},
          "locations": {
            "type": "array",
            "items": {
              "type": "object",
              "additionalProperties": false,
              "properties": {
                "path": {"type": "string"},
                "line_start": {"type": "integer"},
                "line_end": {"type": "integer"},
                "evidence_ref": {"type": "string"}
              },
              "required": ["path"]
            }
          }
        },
        "required": ["id", "severity", "confidence", "category", "claim", "concrete_harm", "minimal_fix", "locations"]
      }
    },
    "non_findings_under_scope": {
      "type": "array",
      "items": {
        "type": "object",
        "additionalProperties": false,
        "properties": {
          "claim": {"type": "string"},
          "basis": {"type": "string"},
          "scope": {"type": "string"}
        },
        "required": ["claim", "basis", "scope"]
      }
    }
  },
  "required": ["reviewer", "verdict", "findings", "non_findings_under_scope"]
}`
}

func BuildReviewerPrompt(stored StoredRun, packetPath string) string {
	modes := strings.Join(stored.Packet.ReviewRequest.Modes, ", ")
	if modes == "" {
		modes = "general_review"
	}
	return strings.Join([]string{
		"# Haft Overseer Review",
		"",
		"You are an advisory reviewer. Return JSON matching `overseer.review_result.v1`.",
		"Do not approve, merge, deploy, decide, commission, or rebaseline.",
		"Only report concrete findings grounded in the packet/code. Leave style preferences out.",
		"Read the packet JSON first; do not inspect unrelated files unless the packet explicitly requires fetch-on-demand.",
		"Return exactly one final JSON object. Do not emit progress JSON as a final answer.",
		"",
		"Required output shape:",
		"",
		"```json",
		`{"reviewer":{"agent":"..."},"verdict":"findings_recorded","findings":[{"id":"ofind-...","severity":"high|medium|low","confidence":"high|medium|low","category":"invariant_conformance","claim":"...","concrete_harm":"...","locations":[{"path":"...","line_start":1}],"minimal_fix":"..."}],"non_findings_under_scope":[]}`,
		"```",
		"",
		"Packet ID: " + stored.Packet.PacketID,
		"Packet JSON path: " + packetPath,
		"Run ID: " + stored.Run.ReviewRunID,
		"Review modes: " + modes,
	}, "\n")
}

func ReviewAbstention(config Config, stored StoredRun, reason string) ReviewResultInput {
	return ReviewResultInput{
		Mode:    "review_abstention",
		Verdict: "review_abstained",
		Reviewer: reviewerForConfig(config, Reviewer{
			SessionRelationToAuthor: "independent_review_session",
			InputSources:            []string{stored.Packet.PacketID},
		}),
		ScopeCoverage: ScopeCoverage{
			ModesReviewed: []string{},
			FilesReviewed: []string{},
			FetchesUsed:   []string{},
			Abstentions:   []string{strings.TrimSpace(reason)},
		},
		Findings: []ReviewFinding{},
		NonFindings: []NonFinding{{
			Claim: "Reviewer did not run.",
			Basis: strings.TrimSpace(reason),
			Scope: stored.Run.ReviewRunID,
		}},
	}
}

func readReviewerResult(path string, stdout string) (ReviewResultInput, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) && strings.TrimSpace(stdout) != "" {
		data = []byte(stdout)
		err = nil
	}
	if err != nil {
		return ReviewResultInput{}, fmt.Errorf("read reviewer result: %w", err)
	}

	var input ReviewResultInput
	if err := json.Unmarshal(data, &input); err != nil {
		return ReviewResultInput{}, fmt.Errorf("decode reviewer result JSON: %w", err)
	}
	return input, nil
}

func reviewerForConfig(config Config, reviewer Reviewer) Reviewer {
	agent := strings.TrimSpace(reviewer.Agent)
	if agent == "" {
		agent = strings.TrimSpace(config.ReviewerAgent)
	}
	if agent == "" {
		agent = "command"
	}
	reviewer.Agent = agent
	reviewer.ModelOrRuntime = firstNonEmpty(reviewer.ModelOrRuntime, "configured_command")
	reviewer.SessionRelationToAuthor = firstNonEmpty(reviewer.SessionRelationToAuthor, "independent_review_session")
	return reviewer
}
