package fpfrefresh

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	maxCandidateTokenGateLockBytes   = 1 << 20
	maxCandidateTokenGateOutputBytes = 64 << 10
)

// ExecutableCandidateTokenGate adapts the repository token-gate script to the
// CandidateTokenGate port. ShellPath and ScriptPath are explicit executable
// coordinates; neither is discovered from PATH or the working directory.
type ExecutableCandidateTokenGate struct {
	ShellPath  string
	ScriptPath string
}

func (gate ExecutableCandidateTokenGate) VerifyCandidate(
	ctx context.Context,
	input CandidateTokenGateInput,
) error {
	if ctx == nil {
		return fmt.Errorf("candidate token-gate context is required")
	}
	normalized, err := normalizeCandidateTokenGateInput(input)
	if err != nil {
		return err
	}
	if err := verifyCandidateTokenGateFile(gate.ShellPath, "shell", true, 0); err != nil {
		return err
	}
	if err := verifyCandidateTokenGateFile(gate.ScriptPath, "script", false, 0); err != nil {
		return err
	}
	if err := verifyCandidateTokenGateFile(
		normalized.DatabasePath,
		"database",
		false,
		0,
	); err != nil {
		return err
	}
	if err := verifyCandidateTokenGateFile(
		normalized.IntegrationLockPath,
		"integration lock",
		false,
		maxCandidateTokenGateLockBytes,
	); err != nil {
		return err
	}
	if err := verifyCandidateTokenGateFile(
		normalized.FixturePath,
		"fixture",
		false,
		maxTokenGateFixtureBytes,
	); err != nil {
		return err
	}
	if err := verifyCandidateTokenGateCoordinates(normalized); err != nil {
		return err
	}

	command := exec.CommandContext(ctx, gate.ShellPath, gate.ScriptPath)
	command.Dir = filepath.Dir(gate.ScriptPath)
	command.Env = candidateTokenGateEnvironment(os.Environ(), normalized)
	var output boundedCandidateTokenGateOutput
	command.Stdout = &output
	command.Stderr = &output
	if err := command.Run(); err != nil {
		detail := output.Detail()
		if detail == "" {
			return fmt.Errorf("run candidate query-token gate: %w", err)
		}
		return fmt.Errorf("run candidate query-token gate: %w: %s", err, detail)
	}
	if err := verifyCandidateTokenGateCoordinates(normalized); err != nil {
		return fmt.Errorf(
			"candidate query-token gate inputs changed during execution: %w",
			err,
		)
	}
	return nil
}

type boundedCandidateTokenGateOutput struct {
	buffer    bytes.Buffer
	truncated bool
}

func (output *boundedCandidateTokenGateOutput) Write(payload []byte) (int, error) {
	originalLength := len(payload)
	remaining := maxCandidateTokenGateOutputBytes - output.buffer.Len()
	if remaining <= 0 {
		output.truncated = output.truncated || originalLength > 0
		return originalLength, nil
	}
	if len(payload) > remaining {
		payload = payload[:remaining]
		output.truncated = true
	}
	_, err := output.buffer.Write(payload)
	return originalLength, err
}

func (output *boundedCandidateTokenGateOutput) Detail() string {
	detail := strings.TrimSpace(output.buffer.String())
	if output.truncated {
		if detail == "" {
			return "[output truncated]"
		}
		return detail + "\n[output truncated]"
	}
	return detail
}

func verifyCandidateTokenGateFile(
	path string,
	label string,
	requireExecutable bool,
	maximumBytes int64,
) error {
	if strings.TrimSpace(path) == "" ||
		path != strings.TrimSpace(path) ||
		!filepath.IsAbs(path) ||
		filepath.Clean(path) != path {
		return fmt.Errorf(
			"candidate token-gate %s path must be a canonical absolute path",
			label,
		)
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("inspect candidate token-gate %s: %w", label, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("candidate token-gate %s is not a regular file", label)
	}
	if requireExecutable && info.Mode().Perm()&0o111 == 0 {
		return fmt.Errorf("candidate token-gate %s is not executable", label)
	}
	if info.Size() <= 0 {
		return fmt.Errorf("candidate token-gate %s is empty", label)
	}
	if maximumBytes > 0 && info.Size() > maximumBytes {
		return fmt.Errorf(
			"candidate token-gate %s size %d exceeds %d bytes",
			label,
			info.Size(),
			maximumBytes,
		)
	}
	return nil
}

func verifyCandidateTokenGateCoordinates(input CandidateTokenGateInput) error {
	lockPayload, err := os.ReadFile(input.IntegrationLockPath)
	if err != nil {
		return fmt.Errorf("read candidate token-gate integration lock: %w", err)
	}
	lock, err := ParseIntegrationLock(lockPayload)
	if err != nil {
		return fmt.Errorf("verify candidate token-gate integration lock: %w", err)
	}
	if lock.TokenGate == nil {
		return fmt.Errorf(
			"verify candidate token-gate integration lock: token_gate coordinates are required",
		)
	}
	databaseDigest, err := digestFile(input.DatabasePath)
	if err != nil {
		return fmt.Errorf("digest candidate token-gate database: %w", err)
	}
	if databaseDigest != lock.Coordinates.DatabaseDigest {
		return fmt.Errorf(
			"snapshot_pin_stale: candidate token-gate database digest %q differs from integration lock %q",
			databaseDigest,
			lock.Coordinates.DatabaseDigest,
		)
	}
	fixtureCoordinates, err := ReadTokenGateCoordinates(input.FixturePath)
	if err != nil {
		return fmt.Errorf("verify candidate token-gate fixture: %w", err)
	}
	if fixtureCoordinates != *lock.TokenGate {
		return fmt.Errorf(
			"snapshot_pin_stale: candidate token-gate fixture coordinates %#v differ from integration lock %#v",
			fixtureCoordinates,
			*lock.TokenGate,
		)
	}
	return nil
}

func candidateTokenGateEnvironment(
	environment []string,
	input CandidateTokenGateInput,
) []string {
	result := replaceProcessEnvironment(
		environment,
		CandidateTokenGateDatabasePathEnvironment,
		input.DatabasePath,
	)
	result = replaceProcessEnvironment(
		result,
		CandidateTokenGateLockPathEnvironment,
		input.IntegrationLockPath,
	)
	return replaceProcessEnvironment(
		result,
		CandidateTokenGateFixturePathEnvironment,
		input.FixturePath,
	)
}
