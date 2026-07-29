package agenthostrestart

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

// OSEvidence derives restart identities from Git, the project ledger, exact
// carrier bytes, and the observed post-restart process. It has no application
// quit, launchd submission, or heartbeat mutation capability.
type OSEvidence struct{}

func NewOSEvidence() OSEvidence { return OSEvidence{} }

func (OSEvidence) CapturePreparation(
	ctx context.Context,
	request PreparationRequest,
) (PreparationEvidence, error) {
	taskRuntime, err := captureCurrentCodexTaskRuntime(ctx)
	if err != nil {
		return PreparationEvidence{}, fmt.Errorf("current Codex task runtime: %w", err)
	}
	projectBasis, err := captureProjectBasis(ctx, request.ProjectRoot)
	if err != nil {
		return PreparationEvidence{}, err
	}
	binaryDigest, err := digestRegularFile(request.CandidateHaftBinary)
	if err != nil {
		return PreparationEvidence{}, fmt.Errorf("candidate Haft binary: %w", err)
	}
	carriers, err := captureCarrierObservation(
		request.SkillCarriersRoot,
		request.InstructionCarrier,
		request.MCPConfigCarrier,
	)
	if err != nil {
		return PreparationEvidence{}, err
	}
	return PreparationEvidence{
		RepositoryHead:              projectBasis.RepositoryHead,
		DirtyStateDigest:            projectBasis.DirtyStateDigest,
		DesiredHaftBinaryDigest:     binaryDigest,
		ExpectedFPFRevision:         projectBasis.FPFRevision,
		ExpectedTypeEnvDigest:       projectBasis.TypeEnvDigest,
		ExpectedTypeEnvHeadRevision: projectBasis.TypeEnvHeadRevision,
		ExpectedGraphRevision:       projectBasis.GraphRevision,
		ExpectedSkillCarriersDigest: carriers.SkillDigest,
		ExpectedInstructionDigest:   carriers.InstructionDigest,
		ExpectedMCPConfigDigest:     carriers.MCPConfigDigest,
		TaskRuntime:                 taskRuntime,
	}, nil
}

func (OSEvidence) CaptureRuntime(
	ctx context.Context,
	request VerificationRequest,
	output io.Writer,
) (RuntimeVerification, error) {
	checkpoint := request.Checkpoint
	cliPath, err := canonicalExistingFile(checkpoint.expectedHaftBinaryPath)
	if err != nil {
		return RuntimeVerification{}, fmt.Errorf("installed Haft CLI: %w", err)
	}
	cliDigest, err := digestRegularFile(cliPath)
	if err != nil {
		return RuntimeVerification{}, fmt.Errorf("installed Haft CLI digest: %w", err)
	}
	if err := validateLiveMCPReceipt(checkpoint, request.LiveMCPReceipt); err != nil {
		return RuntimeVerification{}, err
	}
	observedMCP, err := observeMCPProcess(
		ctx,
		request.LiveMCPReceipt.PID,
		checkpoint.repositoryRoot,
	)
	if err != nil {
		return RuntimeVerification{}, fmt.Errorf("live MCP process: %w", err)
	}
	if observedMCP.ExecutablePath != request.LiveMCPReceipt.ExecutablePath ||
		observedMCP.ExecutableDigest != request.LiveMCPReceipt.ExecutableDigest ||
		observedMCP.ProjectRoot != request.LiveMCPReceipt.ProjectRoot ||
		!observedMCP.StartedAt.Equal(request.LiveMCPReceipt.ProcessStartedAt) {
		return RuntimeVerification{}, fmt.Errorf("live MCP process changed after its status receipt")
	}
	projectBasis, err := captureProjectBasis(ctx, checkpoint.repositoryRoot)
	if err != nil {
		return RuntimeVerification{}, err
	}
	carriers, err := captureCarrierObservation(
		checkpoint.expectedSkillCarriersRoot,
		checkpoint.expectedInstructionPath,
		checkpoint.expectedMCPConfigPath,
	)
	if err != nil {
		return RuntimeVerification{}, err
	}
	if err := runSmokeCommand(
		ctx,
		checkpoint.repositoryRoot,
		"contract",
		cliPath,
		request.ContractSmokeArguments,
		output,
	); err != nil {
		return RuntimeVerification{}, err
	}
	return RuntimeVerification{
		CLIExecutablePath:          cliPath,
		CLIExecutableDigest:        cliDigest,
		ProjectBasis:               projectBasis,
		Carriers:                   carriers,
		LiveMCPReceipt:             request.LiveMCPReceipt,
		FallbackReceipt:            request.FallbackReceipt,
		supervisorRemoval:          request.supervisorRemoval,
		ChangedContractSmokePassed: true,
	}, nil
}

func captureProjectBasis(
	ctx context.Context,
	root string,
) (ProjectBasisObservation, error) {
	repositoryHead, err := gitText(ctx, root, "rev-parse", "HEAD")
	if err != nil {
		return ProjectBasisObservation{}, fmt.Errorf("repository HEAD: %w", err)
	}
	dirtyDigest, err := digestGitDirtyState(ctx, root)
	if err != nil {
		return ProjectBasisObservation{}, fmt.Errorf("dirty repository bytes: %w", err)
	}
	fpfRevision, err := gitText(ctx, filepath.Join(root, "data", "FPF"), "rev-parse", "HEAD")
	if err != nil {
		return ProjectBasisObservation{}, fmt.Errorf("FPF revision: %w", err)
	}
	typeEnvDigest, headRevision, graphRevision, err := currentProjectMemoryBasis(ctx, root)
	if err != nil {
		return ProjectBasisObservation{}, fmt.Errorf("selected project memory basis: %w", err)
	}
	return ProjectBasisObservation{
		RepositoryHead:      repositoryHead,
		DirtyStateDigest:    dirtyDigest,
		FPFRevision:         fpfRevision,
		TypeEnvDigest:       typeEnvDigest,
		TypeEnvHeadRevision: headRevision,
		GraphRevision:       graphRevision,
	}, nil
}

func currentProjectMemoryBasis(
	ctx context.Context,
	root string,
) (string, uint64, uint64, error) {
	ledger, err := projectledger.OpenExisting(ctx, root, projectledger.ReadOnly)
	if err != nil {
		return "", 0, 0, err
	}
	defer ledger.Close()
	transaction, err := ledger.Database().BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return "", 0, 0, err
	}
	defer transaction.Rollback()
	row := transaction.QueryRowContext(
		ctx,
		`SELECT head_revision, selected_composite_ref
		 FROM project_typeenv_heads
		 WHERE project_id = ?`,
		ledger.ProjectID().String(),
	)
	var headRevision int64
	var selectedRef string
	if err := row.Scan(&headRevision, &selectedRef); err != nil {
		return "", 0, 0, fmt.Errorf("read current project TypeEnv head: %w", err)
	}
	graphRow := transaction.QueryRowContext(
		ctx,
		`SELECT graph_revision, active_type_env_ref
		 FROM typed_memory_graph_heads
		 WHERE project_id = ?`,
		ledger.ProjectID().String(),
	)
	var graphRevision int64
	var activeRef string
	if err := graphRow.Scan(&graphRevision, &activeRef); err != nil {
		return "", 0, 0, fmt.Errorf("read current project graph head: %w", err)
	}
	if headRevision <= 0 || graphRevision < 0 {
		return "", 0, 0, fmt.Errorf("project head revisions are outside their valid ranges")
	}
	if selectedRef != activeRef {
		return "", 0, 0, fmt.Errorf("selected TypeEnv and active graph TypeEnv differ")
	}
	reference, err := typedmemory.ParseTypeEnvRef(selectedRef)
	if err != nil {
		return "", 0, 0, fmt.Errorf("parse current selected TypeEnv ref: %w", err)
	}
	if err := transaction.Commit(); err != nil {
		return "", 0, 0, err
	}
	return reference.Digest().String(), uint64(headRevision), uint64(graphRevision), nil
}

func digestGitDirtyState(ctx context.Context, root string) (string, error) {
	status, err := gitBytes(
		ctx,
		root,
		"status",
		"--porcelain=v2",
		"-z",
		"--untracked-files=all",
		"--ignore-submodules=none",
	)
	if err != nil {
		return "", err
	}
	working, err := gitBytes(ctx, root, "diff", "--binary", "--no-ext-diff", "HEAD", "--")
	if err != nil {
		return "", err
	}
	staged, err := gitBytes(ctx, root, "diff", "--cached", "--binary", "--no-ext-diff", "HEAD", "--")
	if err != nil {
		return "", err
	}
	submodules, err := gitBytes(ctx, root, "submodule", "status", "--recursive")
	if err != nil {
		return "", err
	}
	untrackedList, err := gitBytes(ctx, root, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return "", err
	}
	untracked, err := digestUntrackedFiles(root, untrackedList)
	if err != nil {
		return "", err
	}
	hash := sha256.New()
	writeDigestFrame(hash, "git-status-v2", status)
	writeDigestFrame(hash, "working-tree-diff", working)
	writeDigestFrame(hash, "index-diff", staged)
	writeDigestFrame(hash, "submodule-state", submodules)
	writeDigestFrame(hash, "untracked-bytes", untracked)
	return "sha256:" + hex.EncodeToString(hash.Sum(nil)), nil
}

func digestUntrackedFiles(root string, list []byte) ([]byte, error) {
	rawPaths := bytes.Split(list, []byte{0})
	paths := make([]string, 0, len(rawPaths))
	for _, raw := range rawPaths {
		if len(raw) == 0 {
			continue
		}
		paths = append(paths, string(raw))
	}
	sort.Strings(paths)
	buffer := bytes.NewBuffer(nil)
	for _, relative := range paths {
		path, err := safeRepositoryPath(root, relative)
		if err != nil {
			return nil, err
		}
		info, err := os.Lstat(path)
		if err != nil {
			return nil, fmt.Errorf("inspect untracked path %s: %w", relative, err)
		}
		var content []byte
		switch {
		case info.Mode().IsRegular():
			content, err = os.ReadFile(path)
		case info.Mode()&os.ModeSymlink != 0:
			var target string
			target, err = os.Readlink(path)
			content = []byte(target)
		default:
			err = fmt.Errorf("untracked path %s has unsupported mode %s", relative, info.Mode())
		}
		if err != nil {
			return nil, err
		}
		writeDigestFrame(buffer, relative, append([]byte(info.Mode().String()+"\x00"), content...))
	}
	return buffer.Bytes(), nil
}

func safeRepositoryPath(root string, relative string) (string, error) {
	if filepath.IsAbs(relative) || filepath.Clean(relative) != relative || relative == "." || strings.HasPrefix(relative, "..") {
		return "", fmt.Errorf("git returned unsafe repository path %q", relative)
	}
	path := filepath.Join(root, relative)
	relativeCheck, err := filepath.Rel(root, path)
	if err != nil || relativeCheck != relative {
		return "", fmt.Errorf("git repository path %q escaped its root", relative)
	}
	return path, nil
}

func digestHaftSkillCarriers(root string) (string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", err
	}
	paths := make([]string, 0)
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "h-") {
			continue
		}
		base := filepath.Join(root, entry.Name())
		err := filepath.WalkDir(base, func(path string, item os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if item.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("skill carrier %s is a symlink", path)
			}
			if item.Type().IsRegular() {
				paths = append(paths, path)
			}
			return nil
		})
		if err != nil {
			return "", err
		}
	}
	if len(paths) == 0 {
		return "", fmt.Errorf("no h-* skill carriers found in %s", root)
	}
	sort.Strings(paths)
	hash := sha256.New()
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return "", err
		}
		writeDigestFrame(hash, filepath.ToSlash(relative), content)
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil)), nil
}

func captureCarrierObservation(
	skillRoot string,
	instructionPath string,
	mcpConfigPath string,
) (CarrierObservation, error) {
	physicalSkillRoot, err := canonicalExistingDirectory(skillRoot)
	if err != nil {
		return CarrierObservation{}, fmt.Errorf("codex skill carrier root: %w", err)
	}
	if physicalSkillRoot != filepath.Clean(skillRoot) {
		return CarrierObservation{}, fmt.Errorf("codex skill carrier root changed physical location")
	}
	physicalInstruction, err := canonicalExistingFile(instructionPath)
	if err != nil {
		return CarrierObservation{}, fmt.Errorf("instruction carrier: %w", err)
	}
	if physicalInstruction != filepath.Clean(instructionPath) {
		return CarrierObservation{}, fmt.Errorf("instruction carrier changed physical location")
	}
	physicalMCPConfig, err := canonicalExistingFile(mcpConfigPath)
	if err != nil {
		return CarrierObservation{}, fmt.Errorf("MCP config carrier: %w", err)
	}
	if physicalMCPConfig != filepath.Clean(mcpConfigPath) {
		return CarrierObservation{}, fmt.Errorf("MCP config carrier changed physical location")
	}
	skillDigest, err := digestHaftSkillCarriers(physicalSkillRoot)
	if err != nil {
		return CarrierObservation{}, fmt.Errorf("codex skill carriers: %w", err)
	}
	instructionDigest, err := digestRegularFile(physicalInstruction)
	if err != nil {
		return CarrierObservation{}, fmt.Errorf("instruction carrier: %w", err)
	}
	mcpConfigDigest, err := digestRegularFile(physicalMCPConfig)
	if err != nil {
		return CarrierObservation{}, fmt.Errorf("MCP config carrier: %w", err)
	}
	return CarrierObservation{
		SkillCarriersRoot: physicalSkillRoot,
		SkillDigest:       skillDigest,
		InstructionPath:   physicalInstruction,
		InstructionDigest: instructionDigest,
		MCPConfigPath:     physicalMCPConfig,
		MCPConfigDigest:   mcpConfigDigest,
	}, nil
}

func digestRegularFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("%s is not a regular file", path)
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil)), nil
}

func writeDigestFrame(writer io.Writer, label string, content []byte) {
	_ = binary.Write(writer, binary.BigEndian, uint64(len(label)))
	_, _ = io.WriteString(writer, label)
	_ = binary.Write(writer, binary.BigEndian, uint64(len(content)))
	_, _ = writer.Write(content)
}

func gitText(ctx context.Context, root string, args ...string) (string, error) {
	output, err := gitBytes(ctx, root, args...)
	if err != nil {
		return "", err
	}
	value := strings.TrimSpace(string(output))
	if value == "" {
		return "", fmt.Errorf("git returned an empty value")
	}
	return value, nil
}

func gitBytes(ctx context.Context, root string, args ...string) ([]byte, error) {
	git, err := exec.LookPath("git")
	if err != nil {
		return nil, err
	}
	commandArgs := append([]string{"-C", root}, args...)
	command := exec.CommandContext(ctx, git, commandArgs...)
	output, err := command.Output()
	if err == nil {
		return output, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return nil, fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(string(exit.Stderr)))
	}
	return nil, err
}

func processExecutableIdentity(ctx context.Context, pid int) (string, time.Time, error) {
	path, err := processExecutablePath(ctx, pid)
	if err != nil {
		return "", time.Time{}, err
	}
	path, err = canonicalExistingFile(path)
	if err != nil {
		return "", time.Time{}, err
	}
	startedAt, err := processStartedAt(ctx, pid)
	if err != nil {
		return "", time.Time{}, err
	}
	return path, startedAt, nil
}

func observeMCPProcess(
	ctx context.Context,
	pid int,
	root string,
) (mcpProcessObservation, error) {
	path, startedAt, err := processExecutableIdentity(ctx, pid)
	if err != nil {
		return mcpProcessObservation{}, err
	}
	arguments, err := processArguments(ctx, pid)
	if err != nil {
		return mcpProcessObservation{}, err
	}
	if !isHaftServeArguments(arguments) {
		return mcpProcessObservation{}, fmt.Errorf("process %d is not an exact Haft serve process", pid)
	}
	cwd, err := processWorkingDirectory(ctx, pid)
	if err != nil {
		return mcpProcessObservation{}, err
	}
	physicalCWD, err := canonicalExistingDirectory(cwd)
	if err != nil {
		return mcpProcessObservation{}, fmt.Errorf("canonicalize process working directory: %w", err)
	}
	physicalRoot, err := canonicalExistingDirectory(root)
	if err != nil {
		return mcpProcessObservation{}, fmt.Errorf("canonicalize expected project root: %w", err)
	}
	if physicalCWD != physicalRoot {
		return mcpProcessObservation{}, fmt.Errorf("process %d serves another project root", pid)
	}
	digest, err := digestRegularFile(path)
	if err != nil {
		return mcpProcessObservation{}, err
	}
	confirmedStart, err := processStartedAt(ctx, pid)
	if err != nil {
		return mcpProcessObservation{}, err
	}
	if !confirmedStart.Equal(startedAt) {
		return mcpProcessObservation{}, fmt.Errorf("process %d changed generation during observation", pid)
	}
	return mcpProcessObservation{
		PID:              pid,
		ExecutablePath:   path,
		ExecutableDigest: digest,
		ProjectRoot:      physicalRoot,
		StartedAt:        startedAt,
	}, nil
}

func processArguments(ctx context.Context, pid int) ([]string, error) {
	if runtime.GOOS == "linux" {
		content, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "cmdline"))
		if err != nil {
			return nil, err
		}
		parts := bytes.Split(bytes.TrimSuffix(content, []byte{0}), []byte{0})
		arguments := make([]string, 0, len(parts))
		for _, part := range parts {
			arguments = append(arguments, string(part))
		}
		return arguments, nil
	}
	ps, err := exec.LookPath("ps")
	if err != nil {
		return nil, err
	}
	command := exec.CommandContext(ctx, ps, "-p", strconv.Itoa(pid), "-o", "command=")
	output, err := command.Output()
	if err != nil {
		return nil, err
	}
	return strings.Fields(strings.TrimSpace(string(output))), nil
}

func isHaftServeArguments(arguments []string) bool {
	if len(arguments) != 2 {
		return false
	}
	return filepath.Base(arguments[0]) == "haft" && arguments[1] == "serve"
}

func processWorkingDirectory(ctx context.Context, pid int) (string, error) {
	if runtime.GOOS == "linux" {
		return os.Readlink(filepath.Join("/proc", strconv.Itoa(pid), "cwd"))
	}
	lsof, err := exec.LookPath("lsof")
	if err != nil {
		return "", err
	}
	command := exec.CommandContext(
		ctx,
		lsof,
		"-a",
		"-p",
		strconv.Itoa(pid),
		"-d",
		"cwd",
		"-Fn",
	)
	output, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("inspect process working directory with lsof: %w", err)
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

func processExecutablePath(ctx context.Context, pid int) (string, error) {
	if runtime.GOOS == "linux" {
		return os.Readlink(filepath.Join("/proc", strconv.Itoa(pid), "exe"))
	}
	lsof, err := exec.LookPath("lsof")
	if err != nil {
		return "", err
	}
	command := exec.CommandContext(
		ctx,
		lsof,
		"-a",
		"-p",
		strconv.Itoa(pid),
		"-d",
		"txt",
		"-Fn",
	)
	output, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("inspect process executable with lsof: %w", err)
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
	return "", fmt.Errorf("process %d has no executable text path", pid)
}

func processStartedAt(ctx context.Context, pid int) (time.Time, error) {
	ps, err := exec.LookPath("ps")
	if err != nil {
		return time.Time{}, err
	}
	command := exec.CommandContext(ctx, ps, "-p", strconv.Itoa(pid), "-o", "lstart=")
	output, err := command.Output()
	if err != nil {
		return time.Time{}, fmt.Errorf("inspect process start time: %w", err)
	}
	raw := strings.TrimSpace(string(output))
	startedAt, err := time.ParseInLocation("Mon Jan _2 15:04:05 2006", raw, time.Local)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse process start time %q: %w", raw, err)
	}
	return startedAt.UTC(), nil
}

func runSmokeCommand(
	ctx context.Context,
	root string,
	label string,
	executable string,
	arguments []string,
	output io.Writer,
) error {
	command := exec.CommandContext(ctx, executable, arguments...)
	command.Dir = root
	combined, err := command.CombinedOutput()
	_, _ = fmt.Fprintf(output, "%s smoke output:\n%s", label, combined)
	if len(combined) == 0 || combined[len(combined)-1] != '\n' {
		_, _ = fmt.Fprintln(output)
	}
	if err != nil {
		return fmt.Errorf("%s smoke failed: %w", label, err)
	}
	return nil
}
