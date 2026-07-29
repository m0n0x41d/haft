package p14acceptance

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/agenthostrestart"
)

const (
	p14ClaudeHostProofRequestEnvironmentKey = "HAFT_P14_CAPTURE_CLAUDE_HOST_PROOF"
	p14ClaudeHostProofRequestSchema         = "haft.p14.claude-host-proof-request/v2"
	p14ClaudeHostProofRequestPrefix         = "p14-claude-host-proof-request"
	p14ClaudeHostProofCarrierSchema         = "haft.p14.claude-host-proof/v2"
	p14ClaudeHostProofCarrierPrefix         = "p14-claude-host-proof"
	p14ClaudeHostProofStatus                = "observed_not_final"
	p14ClaudeProcessObservationTimeout      = 60 * time.Second
)

type p14ClaudeHostProofRequest struct {
	Schema              string `json:"schema"`
	PreparedCarrierPath string `json:"prepared_carrier_path"`
	ClaudePID           int    `json:"claude_pid"`
	MCPPID              int    `json:"mcp_pid"`
}

type p14ClaudeHostProcessObservation struct {
	PID              int    `json:"pid"`
	ParentPID        int    `json:"parent_pid"`
	ExecutablePath   string `json:"executable_path"`
	ExecutableDigest string `json:"executable_digest"`
	StartedAt        string `json:"started_at"`
	ArgumentsDigest  string `json:"arguments_digest"`
}

type p14ClaudeMCPProcessObservation struct {
	PID              int    `json:"pid"`
	ParentPID        int    `json:"parent_pid"`
	ExecutablePath   string `json:"executable_path"`
	ExecutableDigest string `json:"executable_digest"`
	ProjectRoot      string `json:"project_root"`
	StartedAt        string `json:"started_at"`
	ArgumentsDigest  string `json:"arguments_digest"`
	AncestorPIDs     []int  `json:"ancestor_pids"`
}

type p14ClaudeHostProofCarrier struct {
	Schema                  string                          `json:"schema"`
	Status                  string                          `json:"status"`
	CarrierPath             string                          `json:"carrier_path"`
	EvidenceDigest          string                          `json:"evidence_digest"`
	ReleaseClaim            bool                            `json:"release_claim"`
	PreparedCarrier         p14PreparedObservationBinding   `json:"prepared_carrier"`
	P13Evidence             p13EvidenceBinding              `json:"p13_evidence"`
	CandidateGitHead        string                          `json:"candidate_git_head"`
	CandidateDigest         string                          `json:"candidate_digest"`
	ProjectRoot             string                          `json:"project_root"`
	RestartCheckpointDigest string                          `json:"restart_checkpoint_digest"`
	RestartCheckpointAt     string                          `json:"restart_checkpoint_at"`
	LiveMCPReceiptDigest    string                          `json:"live_mcp_receipt_digest"`
	ProtocolDiscovery       p14MCPProtocolDiscovery         `json:"separate_protocol_discovery"`
	SessionHistory          p14ClaudeSessionHistoryEvidence `json:"claude_session_history"`
	Claude                  p14ClaudeHostProcessObservation `json:"claude"`
	MCP                     p14ClaudeMCPProcessObservation  `json:"mcp"`
	ProcessObservedBeforeAt string                          `json:"process_observed_before_at"`
	ProcessObservedAfterAt  string                          `json:"process_observed_after_at"`
	ObservedAt              string                          `json:"observed_at"`
}

type p14ClaudeHostProofBinding struct {
	CarrierPath    string `json:"carrier_path"`
	CarrierDigest  string `json:"carrier_digest"`
	EvidenceDigest string `json:"evidence_digest"`
}

type p14ObservedProcess struct {
	PID              int
	ParentPID        int
	ExecutablePath   string
	ExecutableDigest string
	WorkingDirectory string
	StartedAt        time.Time
	Arguments        []string
	ArgumentsDigest  string
}

func TestP14CaptureActualClaudeHostProof(t *testing.T) {
	requestPath := os.Getenv(p14ClaudeHostProofRequestEnvironmentKey)
	if requestPath == "" {
		t.Skip(
			"set HAFT_P14_CAPTURE_CLAUDE_HOST_PROOF after Claude restart",
		)
	}
	repositoryRoot, err := p14RepositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	contract, _, err := loadRequestOracleContract(repositoryRoot)
	if err != nil {
		t.Fatal(err)
	}
	request, err := loadP14ClaudeHostProofRequest(
		repositoryRoot,
		requestPath,
	)
	if err != nil {
		t.Fatal(err)
	}
	prepared, preparedDigest, err := loadP14PreparedCarrierForExecution(
		repositoryRoot,
		contract,
		request.PreparedCarrierPath,
	)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := agenthostrestart.LoadVerifiedRuntimeSnapshot(
		repositoryRoot,
	)
	if err != nil {
		t.Fatal(err)
	}
	runtimeBinding, err := p14RuntimeBindingFromVerifiedSnapshot(
		prepared.Preparation,
		snapshot,
	)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(
		context.Background(),
		p14ClaudeProcessObservationTimeout,
	)
	defer cancel()
	carrier, err := captureP14ClaudeHostProof(
		ctx,
		prepared,
		preparedDigest,
		runtimeBinding,
		request,
	)
	if err != nil {
		t.Fatal(err)
	}
	path, digest, err := persistP14ClaudeHostProof(
		repositoryRoot,
		carrier,
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf(
		"P14_CLAUDE_HOST_PROOF path=%s digest=%s evidence=%s",
		path,
		digest,
		carrier.EvidenceDigest,
	)
}

func loadP14ClaudeHostProofRequest(
	repositoryRoot string,
	path string,
) (p14ClaudeHostProofRequest, error) {
	canonical, err := resolveP14ExecutionCarrierPath(
		repositoryRoot,
		path,
		p14ClaudeHostProofRequestPrefix,
	)
	if err != nil {
		return p14ClaudeHostProofRequest{}, err
	}
	raw, err := os.ReadFile(canonical)
	if err != nil {
		return p14ClaudeHostProofRequest{}, err
	}
	request := p14ClaudeHostProofRequest{}
	if err := decodeP14CanonicalCarrier(
		raw,
		&request,
		"Claude host proof request",
	); err != nil {
		return p14ClaudeHostProofRequest{}, err
	}
	if request.Schema != p14ClaudeHostProofRequestSchema ||
		request.PreparedCarrierPath == "" ||
		request.ClaudePID <= 1 ||
		request.MCPPID <= 1 ||
		request.ClaudePID == request.MCPPID {
		return p14ClaudeHostProofRequest{}, fmt.Errorf(
			"P14 Claude host proof request differs",
		)
	}
	return request, nil
}

func captureP14ClaudeHostProof(
	ctx context.Context,
	prepared preparedRequestOracleCarrier,
	preparedDigest string,
	runtimeBinding p14RuntimeObservationBinding,
	request p14ClaudeHostProofRequest,
) (p14ClaudeHostProofCarrier, error) {
	processObservedBeforeAt := time.Now().UTC()
	claude, err := observeP14Process(ctx, request.ClaudePID)
	if err != nil {
		return p14ClaudeHostProofCarrier{}, fmt.Errorf(
			"observe actual Claude host process: %w",
			err,
		)
	}
	mcp, err := observeP14Process(ctx, request.MCPPID)
	if err != nil {
		return p14ClaudeHostProofCarrier{}, fmt.Errorf(
			"observe Claude MCP process: %w",
			err,
		)
	}
	ancestors, err := p14ProcessAncestors(ctx, mcp.ParentPID, 16)
	if err != nil {
		return p14ClaudeHostProofCarrier{}, err
	}
	protocolDiscovery, err := captureP14MCPProtocolDiscovery(
		ctx,
		runtimeBinding,
	)
	if err != nil {
		return p14ClaudeHostProofCarrier{}, fmt.Errorf(
			"capture separate Claude-basis MCP protocol proof: %w",
			err,
		)
	}
	observedAt := time.Now().UTC()
	sessionHistory, err := captureP14ClaudeSessionHistory(
		prepared.Preparation,
		runtimeBinding,
		protocolDiscovery,
		observedAt,
	)
	if err != nil {
		return p14ClaudeHostProofCarrier{}, err
	}
	if err := p14ClaudeSessionMatchesProcessArguments(
		claude.Arguments,
		sessionHistory.SessionID,
	); err != nil {
		return p14ClaudeHostProofCarrier{}, err
	}
	claudeAfter, err := observeP14Process(ctx, request.ClaudePID)
	if err != nil {
		return p14ClaudeHostProofCarrier{}, fmt.Errorf(
			"reobserve actual Claude host process after transcript: %w",
			err,
		)
	}
	mcpAfter, err := observeP14Process(ctx, request.MCPPID)
	if err != nil {
		return p14ClaudeHostProofCarrier{}, fmt.Errorf(
			"reobserve Claude MCP process after transcript: %w",
			err,
		)
	}
	ancestorsAfter, err := p14ProcessAncestors(
		ctx,
		mcpAfter.ParentPID,
		16,
	)
	if err != nil {
		return p14ClaudeHostProofCarrier{}, err
	}
	if !equalP14ObservedProcess(claude, claudeAfter) ||
		!equalP14ObservedProcess(mcp, mcpAfter) ||
		!slices.Equal(ancestors, ancestorsAfter) {
		return p14ClaudeHostProofCarrier{}, fmt.Errorf(
			"P14 Claude or MCP process changed across transcript observation",
		)
	}
	processObservedAfterAt := time.Now().UTC()
	carrier := p14ClaudeHostProofCarrier{
		Schema:       p14ClaudeHostProofCarrierSchema,
		Status:       p14ClaudeHostProofStatus,
		ReleaseClaim: false,
		PreparedCarrier: p14PreparedObservationBinding{
			CarrierPath:       prepared.CarrierPath,
			CarrierDigest:     preparedDigest,
			PreparationDigest: prepared.PreparationDigest,
		},
		P13Evidence:             prepared.Preparation.P13Evidence,
		CandidateGitHead:        prepared.Preparation.FrozenBasis.Candidate.GitHead,
		CandidateDigest:         prepared.Preparation.FrozenBasis.Candidate.ExecutableDigest,
		ProjectRoot:             prepared.Preparation.FrozenBasis.SelectedProject.ProjectRoot,
		RestartCheckpointDigest: runtimeBinding.RestartCheckpointDigest,
		RestartCheckpointAt:     runtimeBinding.RestartCheckpointCreatedAt,
		LiveMCPReceiptDigest:    runtimeBinding.LiveMCPReceiptDigest,
		ProtocolDiscovery:       protocolDiscovery,
		SessionHistory:          sessionHistory,
		Claude: p14ClaudeHostProcessObservation{
			PID:              claude.PID,
			ParentPID:        claude.ParentPID,
			ExecutablePath:   claude.ExecutablePath,
			ExecutableDigest: claude.ExecutableDigest,
			StartedAt:        claude.StartedAt.Format(time.RFC3339Nano),
			ArgumentsDigest:  claude.ArgumentsDigest,
		},
		MCP: p14ClaudeMCPProcessObservation{
			PID:              mcp.PID,
			ParentPID:        mcp.ParentPID,
			ExecutablePath:   mcp.ExecutablePath,
			ExecutableDigest: mcp.ExecutableDigest,
			ProjectRoot:      mcp.WorkingDirectory,
			StartedAt:        mcp.StartedAt.Format(time.RFC3339Nano),
			ArgumentsDigest:  mcp.ArgumentsDigest,
			AncestorPIDs:     ancestors,
		},
		ProcessObservedBeforeAt: processObservedBeforeAt.Format(
			time.RFC3339Nano,
		),
		ProcessObservedAfterAt: processObservedAfterAt.Format(
			time.RFC3339Nano,
		),
		ObservedAt: observedAt.Format(time.RFC3339Nano),
	}
	if err := validateP14ClaudeHostProof(
		ctx,
		prepared,
		runtimeBinding,
		carrier,
		true,
	); err != nil {
		return p14ClaudeHostProofCarrier{}, err
	}
	digest, err := p14ClaudeHostProofEvidenceDigest(carrier)
	if err != nil {
		return p14ClaudeHostProofCarrier{}, err
	}
	carrier.EvidenceDigest = digest
	body := strings.TrimPrefix(digest, "sha256:")
	carrier.CarrierPath = filepath.ToSlash(filepath.Join(
		".context",
		"p14",
		p14ClaudeHostProofCarrierPrefix+"-"+body[:16]+".json",
	))
	if err := validateP14ClaudeHostProof(
		ctx,
		prepared,
		runtimeBinding,
		carrier,
		true,
	); err != nil {
		return p14ClaudeHostProofCarrier{}, err
	}
	return carrier, nil
}

func validateP14ClaudeHostProof(
	ctx context.Context,
	prepared preparedRequestOracleCarrier,
	runtimeBinding p14RuntimeObservationBinding,
	carrier p14ClaudeHostProofCarrier,
	reobserve bool,
) error {
	candidate := prepared.Preparation.FrozenBasis.Candidate
	project := prepared.Preparation.FrozenBasis.SelectedProject
	if carrier.Schema != p14ClaudeHostProofCarrierSchema ||
		carrier.Status != p14ClaudeHostProofStatus ||
		carrier.ReleaseClaim ||
		carrier.PreparedCarrier.CarrierPath != prepared.CarrierPath ||
		!validP14Digest(carrier.PreparedCarrier.CarrierDigest) ||
		carrier.PreparedCarrier.PreparationDigest !=
			prepared.PreparationDigest ||
		carrier.P13Evidence != prepared.Preparation.P13Evidence ||
		carrier.CandidateGitHead != candidate.GitHead ||
		carrier.CandidateDigest != candidate.ExecutableDigest ||
		carrier.ProjectRoot != project.ProjectRoot ||
		carrier.RestartCheckpointDigest !=
			runtimeBinding.RestartCheckpointDigest ||
		carrier.RestartCheckpointAt !=
			runtimeBinding.RestartCheckpointCreatedAt ||
		carrier.LiveMCPReceiptDigest !=
			runtimeBinding.LiveMCPReceiptDigest ||
		carrier.MCP.PID != runtimeBinding.LiveMCPPID ||
		carrier.MCP.ExecutablePath !=
			runtimeBinding.LiveMCPExecutablePath ||
		carrier.MCP.ExecutableDigest != candidate.ExecutableDigest ||
		carrier.MCP.ProjectRoot != project.ProjectRoot ||
		carrier.Claude.PID <= 1 ||
		carrier.MCP.PID <= 1 ||
		carrier.Claude.PID == carrier.MCP.PID {
		return fmt.Errorf("P14 Claude host proof basis differs")
	}
	if err := validateP14MCPProtocolDiscovery(
		runtimeBinding,
		carrier.ProtocolDiscovery,
	); err != nil {
		return err
	}
	if carrier.ProtocolDiscovery.ProcessPID == carrier.MCP.PID {
		return fmt.Errorf(
			"P14 separate protocol proof claims the live Claude MCP PID",
		)
	}
	if err := validateP14ClaudeSessionHistoryEvidence(
		prepared.Preparation,
		runtimeBinding,
		carrier.ProtocolDiscovery,
		carrier.SessionHistory,
	); err != nil {
		return err
	}
	checkpointAt, err := time.Parse(
		time.RFC3339Nano,
		carrier.RestartCheckpointAt,
	)
	if err != nil {
		return fmt.Errorf("P14 Claude proof checkpoint time is invalid: %w", err)
	}
	claudeStartedAt, err := time.Parse(
		time.RFC3339Nano,
		carrier.Claude.StartedAt,
	)
	if err != nil || !claudeStartedAt.After(checkpointAt) {
		return fmt.Errorf("P14 Claude host process is not a new generation")
	}
	mcpStartedAt, err := time.Parse(time.RFC3339Nano, carrier.MCP.StartedAt)
	if err != nil ||
		!mcpStartedAt.After(checkpointAt) ||
		carrier.MCP.StartedAt != runtimeBinding.LiveMCPStartedAt ||
		mcpStartedAt.Before(claudeStartedAt) {
		return fmt.Errorf("P14 Claude MCP process is not a new host generation")
	}
	observedAt, err := time.Parse(time.RFC3339Nano, carrier.ObservedAt)
	if err != nil || observedAt.Before(mcpStartedAt) {
		return fmt.Errorf("P14 Claude proof observation chronology differs")
	}
	processObservedBeforeAt, beforeErr := time.Parse(
		time.RFC3339Nano,
		carrier.ProcessObservedBeforeAt,
	)
	processObservedAfterAt, afterErr := time.Parse(
		time.RFC3339Nano,
		carrier.ProcessObservedAfterAt,
	)
	if beforeErr != nil ||
		afterErr != nil ||
		processObservedBeforeAt.After(observedAt) ||
		processObservedAfterAt.Before(observedAt) ||
		processObservedAfterAt.Before(processObservedBeforeAt) {
		return fmt.Errorf(
			"P14 Claude process observation bracket differs",
		)
	}
	if !slices.Contains(carrier.MCP.AncestorPIDs, carrier.Claude.PID) {
		return fmt.Errorf("P14 Claude MCP process is outside the Claude host tree")
	}
	if !validP14Digest(carrier.Claude.ExecutableDigest) ||
		!validP14Digest(carrier.Claude.ArgumentsDigest) ||
		!validP14Digest(carrier.MCP.ArgumentsDigest) ||
		!validP14Digest(carrier.LiveMCPReceiptDigest) {
		return fmt.Errorf("P14 Claude host proof digest is invalid")
	}
	if carrier.EvidenceDigest != "" {
		digest, err := p14ClaudeHostProofEvidenceDigest(carrier)
		if err != nil {
			return err
		}
		if carrier.EvidenceDigest != digest {
			return fmt.Errorf("P14 Claude host proof evidence digest differs")
		}
		body := strings.TrimPrefix(digest, "sha256:")
		expectedPath := filepath.ToSlash(filepath.Join(
			".context",
			"p14",
			p14ClaudeHostProofCarrierPrefix+"-"+body[:16]+".json",
		))
		if carrier.CarrierPath != expectedPath {
			return fmt.Errorf("P14 Claude host proof path differs")
		}
	}
	if !reobserve {
		return nil
	}
	return reobserveP14ClaudeHostProof(ctx, carrier)
}

func reobserveP14ClaudeHostProof(
	ctx context.Context,
	carrier p14ClaudeHostProofCarrier,
) error {
	claude, err := observeP14Process(ctx, carrier.Claude.PID)
	if err != nil {
		return fmt.Errorf("reobserve Claude host process: %w", err)
	}
	mcp, err := observeP14Process(ctx, carrier.MCP.PID)
	if err != nil {
		return fmt.Errorf("reobserve Claude MCP process: %w", err)
	}
	ancestors, err := p14ProcessAncestors(ctx, mcp.ParentPID, 16)
	if err != nil {
		return err
	}
	expectedClaude := p14ClaudeHostProcessObservation{
		PID:              claude.PID,
		ParentPID:        claude.ParentPID,
		ExecutablePath:   claude.ExecutablePath,
		ExecutableDigest: claude.ExecutableDigest,
		StartedAt:        claude.StartedAt.Format(time.RFC3339Nano),
		ArgumentsDigest:  claude.ArgumentsDigest,
	}
	expectedMCP := p14ClaudeMCPProcessObservation{
		PID:              mcp.PID,
		ParentPID:        mcp.ParentPID,
		ExecutablePath:   mcp.ExecutablePath,
		ExecutableDigest: mcp.ExecutableDigest,
		ProjectRoot:      mcp.WorkingDirectory,
		StartedAt:        mcp.StartedAt.Format(time.RFC3339Nano),
		ArgumentsDigest:  mcp.ArgumentsDigest,
		AncestorPIDs:     ancestors,
	}
	if carrier.Claude != expectedClaude ||
		!equalP14ClaudeMCPObservation(carrier.MCP, expectedMCP) {
		return fmt.Errorf(
			"P14 Claude host proof is stale or belongs to another generation",
		)
	}
	if !p14LooksLikeClaudeHost(claude.Arguments) {
		return fmt.Errorf("P14 selected host process is not Claude")
	}
	if err := p14ClaudeSessionMatchesProcessArguments(
		claude.Arguments,
		carrier.SessionHistory.SessionID,
	); err != nil {
		return err
	}
	if !p14IsExactHaftServe(mcp.Arguments, carrier.MCP.ExecutablePath) {
		return fmt.Errorf("P14 selected Claude MCP process is not exact Haft serve")
	}
	return nil
}

func equalP14ClaudeMCPObservation(
	left p14ClaudeMCPProcessObservation,
	right p14ClaudeMCPProcessObservation,
) bool {
	return left.PID == right.PID &&
		left.ParentPID == right.ParentPID &&
		left.ExecutablePath == right.ExecutablePath &&
		left.ExecutableDigest == right.ExecutableDigest &&
		left.ProjectRoot == right.ProjectRoot &&
		left.StartedAt == right.StartedAt &&
		left.ArgumentsDigest == right.ArgumentsDigest &&
		slices.Equal(left.AncestorPIDs, right.AncestorPIDs)
}

func equalP14ObservedProcess(
	left p14ObservedProcess,
	right p14ObservedProcess,
) bool {
	return left.PID == right.PID &&
		left.ParentPID == right.ParentPID &&
		left.ExecutablePath == right.ExecutablePath &&
		left.ExecutableDigest == right.ExecutableDigest &&
		left.WorkingDirectory == right.WorkingDirectory &&
		left.StartedAt.Equal(right.StartedAt) &&
		left.ArgumentsDigest == right.ArgumentsDigest &&
		slices.Equal(left.Arguments, right.Arguments)
}

func p14ClaudeSessionMatchesProcessArguments(
	arguments []string,
	sessionID string,
) error {
	selectors := make([]string, 0, 2)
	for index, argument := range arguments {
		if argument != "--session-id" && argument != "--resume" {
			continue
		}
		if index+1 >= len(arguments) ||
			strings.HasPrefix(arguments[index+1], "-") {
			if argument == "--resume" {
				continue
			}
			return fmt.Errorf(
				"P14 Claude process has an incomplete session selector",
			)
		}
		selectors = append(selectors, arguments[index+1])
	}
	for _, selector := range selectors {
		if selector != sessionID {
			return fmt.Errorf(
				"P14 Claude transcript differs from the process session selector",
			)
		}
	}
	return nil
}

func p14ClaudeHostProofEvidenceDigest(
	carrier p14ClaudeHostProofCarrier,
) (string, error) {
	basis := carrier
	basis.CarrierPath = ""
	basis.EvidenceDigest = ""
	raw, err := marshalP14CanonicalJSON(basis)
	if err != nil {
		return "", err
	}
	return p14Digest(raw), nil
}

func persistP14ClaudeHostProof(
	repositoryRoot string,
	carrier p14ClaudeHostProofCarrier,
) (string, string, error) {
	canonical, err := json.MarshalIndent(carrier, "", "  ")
	if err != nil {
		return "", "", err
	}
	canonical = append(canonical, '\n')
	if err := publishP14NoClobber(
		repositoryRoot,
		filepath.FromSlash(carrier.CarrierPath),
		canonical,
	); err != nil {
		return "", "", err
	}
	return carrier.CarrierPath, p14Digest(canonical), nil
}

func loadP14ClaudeHostProofForFinalization(
	ctx context.Context,
	repositoryRoot string,
	path string,
	prepared preparedRequestOracleCarrier,
	runtimeBinding p14RuntimeObservationBinding,
) (p14ClaudeHostProofCarrier, string, error) {
	canonical, err := resolveP14ExecutionCarrierPath(
		repositoryRoot,
		path,
		p14ClaudeHostProofCarrierPrefix,
	)
	if err != nil {
		return p14ClaudeHostProofCarrier{}, "", err
	}
	raw, err := os.ReadFile(canonical)
	if err != nil {
		return p14ClaudeHostProofCarrier{}, "", err
	}
	carrier := p14ClaudeHostProofCarrier{}
	if err := decodeP14CanonicalCarrier(
		raw,
		&carrier,
		"Claude host proof carrier",
	); err != nil {
		return p14ClaudeHostProofCarrier{}, "", err
	}
	if err := validateP14ClaudeHostProof(
		ctx,
		prepared,
		runtimeBinding,
		carrier,
		true,
	); err != nil {
		return p14ClaudeHostProofCarrier{}, "", err
	}
	preparedPath := filepath.Join(
		repositoryRoot,
		filepath.FromSlash(prepared.CarrierPath),
	)
	preparedRaw, err := os.ReadFile(preparedPath)
	if err != nil {
		return p14ClaudeHostProofCarrier{}, "", err
	}
	if carrier.PreparedCarrier.CarrierDigest != p14Digest(preparedRaw) {
		return p14ClaudeHostProofCarrier{}, "", fmt.Errorf(
			"P14 Claude proof prepared carrier digest differs",
		)
	}
	if err := verifyP14ClaudeSessionHistorySource(
		prepared.Preparation,
		runtimeBinding,
		carrier.ProtocolDiscovery,
		carrier.SessionHistory,
	); err != nil {
		return p14ClaudeHostProofCarrier{}, "", err
	}
	return carrier, p14Digest(raw), nil
}

func observeP14Process(
	ctx context.Context,
	pid int,
) (p14ObservedProcess, error) {
	if pid <= 1 {
		return p14ObservedProcess{}, fmt.Errorf("invalid process ID %d", pid)
	}
	path, err := p14ProcessExecutablePath(ctx, pid)
	if err != nil {
		return p14ObservedProcess{}, err
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return p14ObservedProcess{}, err
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return p14ObservedProcess{}, fmt.Errorf(
			"process %d executable is not a regular file",
			pid,
		)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return p14ObservedProcess{}, err
	}
	startedAt, err := p14ProcessStartedAt(ctx, pid)
	if err != nil {
		return p14ObservedProcess{}, err
	}
	parentPID, err := p14ProcessParentPID(ctx, pid)
	if err != nil {
		return p14ObservedProcess{}, err
	}
	arguments, err := p14ProcessArguments(ctx, pid)
	if err != nil {
		return p14ObservedProcess{}, err
	}
	argumentBytes, err := marshalP14CanonicalJSON(arguments)
	if err != nil {
		return p14ObservedProcess{}, err
	}
	workingDirectory, err := p14ProcessWorkingDirectory(ctx, pid)
	if err != nil {
		return p14ObservedProcess{}, err
	}
	workingDirectory, err = filepath.EvalSymlinks(workingDirectory)
	if err != nil {
		return p14ObservedProcess{}, err
	}
	confirmedStart, err := p14ProcessStartedAt(ctx, pid)
	if err != nil || !confirmedStart.Equal(startedAt) {
		return p14ObservedProcess{}, fmt.Errorf(
			"process %d changed generation during observation",
			pid,
		)
	}
	return p14ObservedProcess{
		PID:              pid,
		ParentPID:        parentPID,
		ExecutablePath:   path,
		ExecutableDigest: p14Digest(raw),
		WorkingDirectory: workingDirectory,
		StartedAt:        startedAt.UTC(),
		Arguments:        arguments,
		ArgumentsDigest:  p14Digest(argumentBytes),
	}, nil
}

func p14ProcessExecutablePath(ctx context.Context, pid int) (string, error) {
	if runtime.GOOS == "linux" {
		return os.Readlink(filepath.Join("/proc", strconv.Itoa(pid), "exe"))
	}
	output, err := p14RunProcessCommand(
		ctx,
		"lsof",
		"-a",
		"-p",
		strconv.Itoa(pid),
		"-d",
		"txt",
		"-Fn",
	)
	if err != nil {
		return "", err
	}
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "n/") {
			return strings.TrimPrefix(line, "n"), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("process %d has no executable path", pid)
}

func p14ProcessWorkingDirectory(
	ctx context.Context,
	pid int,
) (string, error) {
	if runtime.GOOS == "linux" {
		return os.Readlink(filepath.Join("/proc", strconv.Itoa(pid), "cwd"))
	}
	output, err := p14RunProcessCommand(
		ctx,
		"lsof",
		"-a",
		"-p",
		strconv.Itoa(pid),
		"-d",
		"cwd",
		"-Fn",
	)
	if err != nil {
		return "", err
	}
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "n/") {
			return strings.TrimPrefix(line, "n"), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("process %d has no working directory", pid)
}

func p14ProcessArguments(
	ctx context.Context,
	pid int,
) ([]string, error) {
	if runtime.GOOS == "linux" {
		raw, err := os.ReadFile(
			filepath.Join("/proc", strconv.Itoa(pid), "cmdline"),
		)
		if err != nil {
			return nil, err
		}
		parts := bytes.Split(bytes.TrimSuffix(raw, []byte{0}), []byte{0})
		result := make([]string, 0, len(parts))
		for _, part := range parts {
			result = append(result, string(part))
		}
		return result, nil
	}
	output, err := p14RunProcessCommand(
		ctx,
		"ps",
		"-ww",
		"-p",
		strconv.Itoa(pid),
		"-o",
		"command=",
	)
	if err != nil {
		return nil, err
	}
	return strings.Fields(strings.TrimSpace(string(output))), nil
}

func p14ProcessParentPID(ctx context.Context, pid int) (int, error) {
	output, err := p14RunProcessCommand(
		ctx,
		"ps",
		"-p",
		strconv.Itoa(pid),
		"-o",
		"ppid=",
	)
	if err != nil {
		return 0, err
	}
	value := strings.TrimSpace(string(output))
	parent, err := strconv.Atoi(value)
	if err != nil || parent < 0 {
		return 0, fmt.Errorf("process %d parent PID is invalid", pid)
	}
	return parent, nil
}

func p14ProcessStartedAt(
	ctx context.Context,
	pid int,
) (time.Time, error) {
	output, err := p14RunProcessCommand(
		ctx,
		"ps",
		"-p",
		strconv.Itoa(pid),
		"-o",
		"lstart=",
	)
	if err != nil {
		return time.Time{}, err
	}
	value := strings.Join(
		strings.Fields(strings.TrimSpace(string(output))),
		" ",
	)
	startedAt, err := time.ParseInLocation(
		"Mon Jan 2 15:04:05 2006",
		value,
		time.Local,
	)
	if err != nil {
		return time.Time{}, fmt.Errorf(
			"parse process %d start time %q: %w",
			pid,
			value,
			err,
		)
	}
	return startedAt.UTC(), nil
}

func p14ProcessAncestors(
	ctx context.Context,
	parentPID int,
	limit int,
) ([]int, error) {
	result := make([]int, 0, limit)
	current := parentPID
	for current > 1 && len(result) < limit {
		result = append(result, current)
		next, err := p14ProcessParentPID(ctx, current)
		if err != nil {
			return nil, err
		}
		if next == current {
			return nil, fmt.Errorf("process %d is its own parent", current)
		}
		current = next
	}
	if current > 1 {
		return nil, fmt.Errorf("P14 Claude process ancestry exceeds %d", limit)
	}
	return result, nil
}

func p14RunProcessCommand(
	ctx context.Context,
	name string,
	args ...string,
) ([]byte, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		return nil, err
	}
	command := exec.CommandContext(ctx, path, args...)
	command.Env = append(slices.Clone(os.Environ()), "LC_ALL=C")
	output, err := command.Output()
	if err == nil {
		return output, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return nil, fmt.Errorf(
			"%s %s: %s",
			name,
			strings.Join(args, " "),
			strings.TrimSpace(string(exit.Stderr)),
		)
	}
	return nil, err
}

func p14LooksLikeClaudeHost(arguments []string) bool {
	for _, argument := range arguments {
		lower := strings.ToLower(argument)
		base := strings.ToLower(filepath.Base(argument))
		if base == "claude" ||
			base == "claude-code" ||
			strings.Contains(lower, "@anthropic-ai/claude-code") {
			return true
		}
	}
	return false
}

func p14IsExactHaftServe(arguments []string, executable string) bool {
	if len(arguments) != 2 || arguments[1] != "serve" {
		return false
	}
	resolved, err := filepath.EvalSymlinks(arguments[0])
	if err != nil {
		return false
	}
	return resolved == executable
}

func TestP14ClaudeHostProofRejectsMissingAndOldGeneration(t *testing.T) {
	preparation, runtimeBinding, protocol :=
		syntheticP14ClaudeSessionBasis(t)
	sessionObservedAt := time.Date(
		2026,
		7,
		28,
		10,
		2,
		0,
		0,
		time.UTC,
	)
	sessionRaw := syntheticP14ClaudeSessionJSONL(
		t,
		preparation,
		runtimeBinding,
		protocol,
		"normal",
	)
	sessionHistory, err := deriveP14ClaudeSessionHistoryEvidence(
		syntheticP14ClaudeSessionPath(t),
		sessionRaw,
		preparation,
		runtimeBinding,
		protocol,
		sessionObservedAt,
	)
	if err != nil {
		t.Fatal(err)
	}
	prepared := preparedRequestOracleCarrier{
		CarrierPath:       ".context/p14/p14-prepared-synthetic.json",
		PreparationDigest: p14TestDigest("claude-preparation"),
		Preparation:       preparation,
	}
	runtimeBinding.RestartCheckpointDigest =
		p14TestDigest("claude-restart")
	carrier := p14ClaudeHostProofCarrier{
		Schema: p14ClaudeHostProofCarrierSchema,
		Status: p14ClaudeHostProofStatus,
		PreparedCarrier: p14PreparedObservationBinding{
			CarrierPath:       prepared.CarrierPath,
			CarrierDigest:     p14TestDigest("claude-prepared-carrier"),
			PreparationDigest: prepared.PreparationDigest,
		},
		P13Evidence:             preparation.P13Evidence,
		CandidateGitHead:        prepared.Preparation.FrozenBasis.Candidate.GitHead,
		CandidateDigest:         preparation.FrozenBasis.Candidate.ExecutableDigest,
		ProjectRoot:             preparation.FrozenBasis.SelectedProject.ProjectRoot,
		RestartCheckpointDigest: runtimeBinding.RestartCheckpointDigest,
		RestartCheckpointAt:     runtimeBinding.RestartCheckpointCreatedAt,
		LiveMCPReceiptDigest:    runtimeBinding.LiveMCPReceiptDigest,
		ProtocolDiscovery:       protocol,
		SessionHistory:          sessionHistory,
		Claude: p14ClaudeHostProcessObservation{
			PID:              80,
			ParentPID:        2,
			ExecutablePath:   "/synthetic/claude",
			ExecutableDigest: p14TestDigest("claude-executable"),
			StartedAt:        "2026-07-28T09:59:00Z",
			ArgumentsDigest:  p14TestDigest("claude-arguments"),
		},
		MCP: p14ClaudeMCPProcessObservation{
			PID:              runtimeBinding.LiveMCPPID,
			ParentPID:        80,
			ExecutablePath:   runtimeBinding.LiveMCPExecutablePath,
			ExecutableDigest: runtimeBinding.LiveMCPExecutableDigest,
			ProjectRoot:      runtimeBinding.LiveMCPProjectRoot,
			StartedAt:        runtimeBinding.LiveMCPStartedAt,
			ArgumentsDigest:  p14TestDigest("claude-mcp-arguments"),
			AncestorPIDs:     []int{80, 2},
		},
		ProcessObservedBeforeAt: "2026-07-28T10:01:59Z",
		ProcessObservedAfterAt:  "2026-07-28T10:02:01Z",
		ObservedAt:              sessionObservedAt.Format(time.RFC3339Nano),
	}
	if err := validateP14ClaudeHostProof(
		context.Background(),
		prepared,
		runtimeBinding,
		carrier,
		false,
	); err == nil {
		t.Fatal("P14 Claude proof accepted a host process predating restart")
	}
	carrier.Claude.StartedAt = "2026-07-28T10:00:01Z"
	if err := validateP14ClaudeHostProof(
		context.Background(),
		prepared,
		runtimeBinding,
		carrier,
		false,
	); err != nil {
		t.Fatal(err)
	}
	wrongPID := carrier
	wrongPID.MCP.PID++
	if err := validateP14ClaudeHostProof(
		context.Background(),
		prepared,
		runtimeBinding,
		wrongPID,
		false,
	); err == nil {
		t.Fatal(
			"P14 Claude proof accepted an MCP PID outside the live challenge receipt",
		)
	}
	wrongReceipt := carrier
	wrongReceipt.LiveMCPReceiptDigest =
		p14TestDigest("another-live-MCP-receipt")
	if err := validateP14ClaudeHostProof(
		context.Background(),
		prepared,
		runtimeBinding,
		wrongReceipt,
		false,
	); err == nil {
		t.Fatal("P14 Claude proof accepted another live MCP receipt")
	}
	if err := validateP14ClaudeHostProofBinding(
		p14ClaudeHostProofBinding{},
	); err == nil {
		t.Fatal("P14 final carrier accepted a missing Claude proof binding")
	}
}
