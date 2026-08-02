package fpfrefresh

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/m0n0x41d/haft/internal/fpf"
	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
	"github.com/m0n0x41d/haft/internal/fpf/typeenvsql"
)

const (
	DefaultSourceRelativePath              = "data/FPF"
	DefaultDatabaseRelativePath            = "internal/cli/fpf.db"
	DefaultIntegrationLockRelativePath     = "data/haft/fpf-integration.lock.json"
	DefaultTokenGateFixtureRelativePath    = "internal/cli/testdata/fpf_query_token_gate_corpus.json" // #nosec G101 -- repository fixture path, not a credential.
	DefaultRefreshStateRelativeDirectory   = ".context/fpf-refresh"
	DefaultLocalPracticeCandidateRelative  = "data/haft/local-practice/typed-memory/candidates/1.5.0.yaml"
	DefaultCandidateRef                    = "origin/main"
	DefaultRefreshReportFilename           = "latest-report.json"
	DefaultRefreshReceiptFilename          = "apply-receipt.json"
	DefaultRefreshCompletedReceiptFilename = "last-complete-receipt.json"
)

var gitLinkLinePattern = regexp.MustCompile(
	`^160000\s+commit\s+([0-9a-f]{40}|[0-9a-f]{64})\t(.+)$`,
)

// RepositoryLayout is the closed path set used by refresh effects. All paths
// are absolute and canonical; apply/restore code does not discover extra
// mutation targets from directory scans, globs, or timestamps.
type RepositoryLayout struct {
	Root                         string
	SourceRepository             string
	Database                     string
	IntegrationLock              string
	TokenGateFixture             string
	StateDirectory               string
	Report                       string
	Receipt                      string
	CompletedReceipt             string
	LatestLocalPracticeCandidate string
}

// DatabaseSourceRevision reads the exact source revision owned by the current
// derived database. This is the predecessor basis for an uncommitted but
// coherent local refresh; the root commit's gitlink remains a separate
// publication coordinate.
func DatabaseSourceRevision(databasePath string) (string, error) {
	database, err := openIntegrationDatabaseReadOnly(databasePath)
	if err != nil {
		return "", err
	}
	defer func() { _ = database.Close() }()
	meta, err := readRequiredIntegrationMeta(database)
	if err != nil {
		return "", err
	}
	revision, err := normalizeCommitSHA(meta["fpf_commit"])
	if err != nil {
		return "", fmt.Errorf("FPF index fpf_commit: %w", err)
	}
	return revision, nil
}

// ResolveRepositoryLayout constructs the bounded default repository layout.
func ResolveRepositoryLayout(root string) (RepositoryLayout, error) {
	absoluteRoot, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return RepositoryLayout{}, fmt.Errorf("resolve repository root: %w", err)
	}
	info, err := os.Stat(absoluteRoot)
	if err != nil {
		return RepositoryLayout{}, fmt.Errorf("inspect repository root: %w", err)
	}
	if !info.IsDir() {
		return RepositoryLayout{}, fmt.Errorf("repository root %q is not a directory", absoluteRoot)
	}
	stateDirectory := filepath.Join(absoluteRoot, DefaultRefreshStateRelativeDirectory)
	return RepositoryLayout{
		Root:                         absoluteRoot,
		SourceRepository:             filepath.Join(absoluteRoot, DefaultSourceRelativePath),
		Database:                     filepath.Join(absoluteRoot, DefaultDatabaseRelativePath),
		IntegrationLock:              filepath.Join(absoluteRoot, DefaultIntegrationLockRelativePath),
		TokenGateFixture:             filepath.Join(absoluteRoot, DefaultTokenGateFixtureRelativePath),
		StateDirectory:               stateDirectory,
		Report:                       filepath.Join(stateDirectory, DefaultRefreshReportFilename),
		Receipt:                      filepath.Join(stateDirectory, DefaultRefreshReceiptFilename),
		CompletedReceipt:             filepath.Join(stateDirectory, DefaultRefreshCompletedReceiptFilename),
		LatestLocalPracticeCandidate: filepath.Join(absoluteRoot, DefaultLocalPracticeCandidateRelative),
	}, nil
}

// ValidateReportPath keeps the one caller-selected check output disjoint from
// repository publications, source, recovery controls, and durable artifacts.
// A report may live outside the repository or as an ordinary file directly in
// the bounded refresh state directory.
func ValidateReportPath(layout RepositoryLayout, path string) error {
	if path == "" || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return fmt.Errorf("refresh report path %q is not absolute and canonical", path)
	}
	if filepath.Dir(path) == path {
		return fmt.Errorf("refresh report path %q names a filesystem root", path)
	}
	if receiptPathWithin(path, layout.SourceRepository) {
		return fmt.Errorf("refresh report path %q is inside the FPF source publication", path)
	}
	if receiptPathWithin(path, layout.Root) &&
		!receiptPathWithin(path, layout.StateDirectory) {
		return fmt.Errorf(
			"refresh report path %q is inside the repository but outside the bounded refresh state directory",
			path,
		)
	}
	protectedFiles := []string{
		layout.Database,
		layout.IntegrationLock,
		layout.TokenGateFixture,
		layout.Receipt,
		layout.CompletedReceipt,
		layout.LatestLocalPracticeCandidate,
	}
	for _, protected := range protectedFiles {
		if path == protected {
			return fmt.Errorf("refresh report path collides with protected target %q", protected)
		}
	}
	for _, protectedDirectory := range []string{
		filepath.Join(layout.StateDirectory, "artifacts"),
		filepath.Join(layout.StateDirectory, "receipts"),
	} {
		if receiptPathWithin(path, protectedDirectory) {
			return fmt.Errorf(
				"refresh report path %q is inside protected recovery storage %q",
				path,
				protectedDirectory,
			)
		}
	}
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return fmt.Errorf("refresh report path %q is not a regular file", path)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect refresh report path %q: %w", path, err)
	}
	return nil
}

// TrackedSourceRevision returns the submodule gitlink recorded by the root
// commit. It does not consult the mutable submodule checkout.
func TrackedSourceRevision(
	ctx context.Context,
	layout RepositoryLayout,
) (string, error) {
	output, err := runRepositoryGit(
		ctx,
		layout.Root,
		"ls-tree",
		"HEAD",
		"--",
		DefaultSourceRelativePath,
	)
	if err != nil {
		return "", fmt.Errorf("resolve tracked FPF gitlink: %w", err)
	}
	line := strings.TrimSpace(string(output))
	match := gitLinkLinePattern.FindStringSubmatch(line)
	if len(match) != 3 || match[2] != DefaultSourceRelativePath {
		return "", fmt.Errorf(
			"resolve tracked FPF gitlink: unexpected ls-tree record %q",
			line,
		)
	}
	return match[1], nil
}

// CheckedOutSourceRevision returns the exact source repository HEAD without
// changing the checkout.
func CheckedOutSourceRevision(
	ctx context.Context,
	layout RepositoryLayout,
) (string, error) {
	output, err := runRepositoryGit(ctx, layout.SourceRepository, "rev-parse", "HEAD^{commit}")
	if err != nil {
		return "", fmt.Errorf("resolve checked-out FPF revision: %w", err)
	}
	revision := strings.TrimSpace(string(output))
	if !fullGitCommitSHAPattern.MatchString(revision) {
		return "", fmt.Errorf("checked-out FPF revision %q is not an exact commit SHA", revision)
	}
	return revision, nil
}

// VerifySourceCheckoutClean rejects unrelated tracked, staged, or untracked
// source-repository dirt before an apply/restore checkout.
func VerifySourceCheckoutClean(
	ctx context.Context,
	layout RepositoryLayout,
) error {
	output, err := runRepositoryGit(
		ctx,
		layout.SourceRepository,
		"status",
		"--porcelain=v1",
		"--untracked-files=all",
	)
	if err != nil {
		return fmt.Errorf("inspect FPF checkout dirt: %w", err)
	}
	if len(bytes.TrimSpace(output)) != 0 {
		return fmt.Errorf(
			"FPF source checkout has unrelated dirt; refresh refuses to checkout another revision:\n%s",
			strings.TrimSpace(string(output)),
		)
	}
	return nil
}

// CheckoutExactSource changes only the bounded FPF submodule checkout and only
// after its own worktree has been proven clean. An already-current revision is
// an idempotent no-op.
func CheckoutExactSource(
	ctx context.Context,
	layout RepositoryLayout,
	revision string,
) error {
	if !fullGitCommitSHAPattern.MatchString(revision) {
		return fmt.Errorf("checkout exact FPF source: revision is not a full lowercase commit SHA")
	}
	current, err := CheckedOutSourceRevision(ctx, layout)
	if err != nil {
		return err
	}
	if current == revision {
		return VerifySourceCheckoutClean(ctx, layout)
	}
	if err := VerifySourceCheckoutClean(ctx, layout); err != nil {
		return err
	}
	if _, err := runRepositoryGit(
		ctx,
		layout.SourceRepository,
		"checkout",
		"--detach",
		revision,
	); err != nil {
		return fmt.Errorf("checkout exact FPF source %s: %w", revision, err)
	}
	observed, err := CheckedOutSourceRevision(ctx, layout)
	if err != nil {
		return err
	}
	if observed != revision {
		return fmt.Errorf(
			"checkout exact FPF source: observed %s after requesting %s",
			observed,
			revision,
		)
	}
	return nil
}

// VerifyRepositoryIntegration proves that the checked-out publications,
// derived database, and generated lock bind the same source and Base TypeEnv.
// expectedGeneratedBy must be computed independently from the current refresh
// implementation; the generated lock cannot supply its own expectation.
// onVerified runs while the same operation lock used by apply/recovery is held,
// so callers can consume the verified result without racing a publication
// mutation. Verification may create only the private ignored refresh-state
// directory and operation-lock carrier; it does not rebuild, fetch, apply,
// mutate source/DB/integration-lock publications, or make a compatibility,
// semantic, lifecycle, installation, or release-authority claim.
func VerifyRepositoryIntegration(
	ctx context.Context,
	layout RepositoryLayout,
	expectedGeneratedBy string,
	tokenGate *TokenGateCoordinates,
	onVerified func(IntegrationLock) error,
) error {
	if expectedGeneratedBy == "" ||
		expectedGeneratedBy != strings.TrimSpace(expectedGeneratedBy) ||
		strings.ContainsAny(expectedGeneratedBy, "\x00\r\n") {
		return fmt.Errorf(
			"expected current generator identity must be exact, non-empty, and single-line",
		)
	}
	if onVerified == nil {
		return fmt.Errorf("verified repository integration consumer is required")
	}
	release, err := acquireRepositoryVerificationOperationLock(layout)
	if err != nil {
		return err
	}
	defer release()
	recovery, err := InspectRecovery(layout.Receipt)
	if err != nil {
		return err
	}
	if recovery.Required {
		return fmt.Errorf(
			"%w: receipt=%s state=%s; resume or restore that exact receipt before verifying repository integration",
			ErrRecoveryRequired,
			layout.Receipt,
			recovery.Receipt.State,
		)
	}
	revision, err := CheckedOutSourceRevision(ctx, layout)
	if err != nil {
		return err
	}
	payload, err := os.ReadFile(layout.IntegrationLock)
	if err != nil {
		return fmt.Errorf("read generated FPF integration lock: %w", err)
	}
	lock, err := ParseIntegrationLock(payload)
	if err != nil {
		return err
	}
	if lock.GeneratedBy != expectedGeneratedBy {
		return fmt.Errorf(
			"snapshot_pin_stale: integration lock generated_by %q differs from current generator identity %q",
			lock.GeneratedBy,
			expectedGeneratedBy,
		)
	}
	input := IntegrationCoordinateInput{
		SourceRevision: revision,
		ReadmePath:     filepath.Join(layout.SourceRepository, gitSourceReadmePath),
		SpecPath:       filepath.Join(layout.SourceRepository, gitSourceSpecPath),
		DatabasePath:   layout.Database,
		GeneratedBy:    expectedGeneratedBy,
		TokenGate:      tokenGate,
	}
	if err := VerifyIntegrationLock(lock, input); err != nil {
		return err
	}
	if err := verifyRepositoryDerivedProjection(
		layout,
		revision,
	); err != nil {
		return err
	}
	if err := VerifyLocalPracticeCandidateExact(
		layout.LatestLocalPracticeCandidate,
		layout.Database,
		lock.Coordinates.SourceRevision,
		lock.Coordinates.SpecDocumentDigest,
	); err != nil {
		return fmt.Errorf("verify current Local-Practice FPF basis: %w", err)
	}
	if _, err := VerifyCandidateQueryContract(layout.Database); err != nil {
		return err
	}
	if err := onVerified(lock); err != nil {
		return fmt.Errorf("consume verified repository integration: %w", err)
	}
	return nil
}

func acquireRepositoryVerificationOperationLock(
	layout RepositoryLayout,
) (func(), error) {
	if layout.Root == "" ||
		!filepath.IsAbs(layout.Root) ||
		filepath.Clean(layout.Root) != layout.Root {
		return nil, fmt.Errorf("repository root must be absolute and canonical")
	}
	if layout.StateDirectory == "" ||
		!filepath.IsAbs(layout.StateDirectory) ||
		filepath.Clean(layout.StateDirectory) != layout.StateDirectory ||
		layout.StateDirectory == layout.Root ||
		!receiptPathWithin(layout.StateDirectory, layout.Root) {
		return nil, fmt.Errorf(
			"refresh state directory must be an absolute canonical child of the repository root",
		)
	}
	if layout.Receipt == "" ||
		!filepath.IsAbs(layout.Receipt) ||
		filepath.Clean(layout.Receipt) != layout.Receipt ||
		filepath.Dir(layout.Receipt) != layout.StateDirectory {
		return nil, fmt.Errorf(
			"refresh operation receipt must be an absolute canonical file directly in the refresh state directory",
		)
	}
	created := false
	if _, err := os.Lstat(layout.StateDirectory); errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(layout.StateDirectory, 0o700); err != nil {
			return nil, fmt.Errorf("create private FPF refresh state directory: %w", err)
		}
		created = true
	} else if err != nil {
		return nil, fmt.Errorf("inspect FPF refresh state directory: %w", err)
	}
	info, err := os.Lstat(layout.StateDirectory)
	if err != nil {
		return nil, fmt.Errorf("inspect FPF refresh state directory: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, fmt.Errorf("FPF refresh state path is not a real directory")
	}
	if created && info.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("new FPF refresh state directory is not private")
	}
	return acquireOperationLock(layout.Receipt)
}

func verifyRepositoryDerivedProjection(
	layout RepositoryLayout,
	revision string,
) error {
	readme, err := os.ReadFile(
		filepath.Join(layout.SourceRepository, gitSourceReadmePath),
	)
	if err != nil {
		return fmt.Errorf("read checked-out FPF readme publication: %w", err)
	}
	specification, err := os.ReadFile(
		filepath.Join(layout.SourceRepository, gitSourceSpecPath),
	)
	if err != nil {
		return fmt.Errorf("read checked-out FPF specification publication: %w", err)
	}
	snapshot, err := buildLogicalPublicationSnapshot(
		readme,
		specification,
		revision,
	)
	if err != nil {
		return fmt.Errorf("build checked-out FPF publication snapshot: %w", err)
	}
	return verifyDerivedProjection(layout.Database, snapshot)
}

func verifyGitSourceDerivedProjection(
	databasePath string,
	source GitSourceSnapshot,
) error {
	snapshot, err := buildLogicalPublicationSnapshot(
		source.ReadmeBytes(),
		source.SpecificationBytes(),
		source.CommitSHA(),
	)
	if err != nil {
		return fmt.Errorf("build exact FPF publication snapshot: %w", err)
	}
	return verifyDerivedProjection(databasePath, snapshot)
}

func buildLogicalPublicationSnapshot(
	readme []byte,
	specification []byte,
	revision string,
) (fpf.PublicationSnapshot, error) {
	return fpf.BuildPublicationSnapshot(fpf.SourceBundle{
		Readme: fpf.SourceDocument{
			Path:           candidateLogicalReadmePath,
			SourceRevision: revision,
			Markdown:       readme,
		},
		Spec: fpf.SourceDocument{
			Path:           candidateLogicalSpecPath,
			SourceRevision: revision,
			Markdown:       specification,
		},
	})
}

func verifyDerivedProjection(
	databasePath string,
	snapshot fpf.PublicationSnapshot,
) error {
	database, err := openIntegrationDatabaseReadOnly(databasePath)
	if err != nil {
		return err
	}
	defer func() { _ = database.Close() }()
	if err := fpf.VerifyPublicationSnapshotReadOnlyDB(database, snapshot); err != nil {
		return fmt.Errorf("verify exact FPF publication projection: %w", err)
	}
	compilation, err := typeenv.CompileBaseTypeEnv(snapshot)
	if err != nil {
		return fmt.Errorf("compile checked-out FPF Base TypeEnv: %w", err)
	}
	expected, accepted := compilation.Artifact()
	if !accepted {
		return fmt.Errorf(
			"checked-out FPF Base TypeEnv is rejected: %v",
			compilation.Diagnostics(),
		)
	}
	stored, err := typeenvsql.LoadArtifactReadOnlyDB(context.Background(), database)
	if err != nil {
		return fmt.Errorf("load stored FPF Base TypeEnv: %w", err)
	}
	if stored.Digest() != expected.Digest() ||
		!bytes.Equal(stored.CanonicalBytes(), expected.CanonicalBytes()) {
		return fmt.Errorf(
			"stored FPF Base TypeEnv differs from checked-out source compilation",
		)
	}
	return nil
}

func runRepositoryGit(
	ctx context.Context,
	repositoryPath string,
	args ...string,
) ([]byte, error) {
	commandArgs := append([]string{"-C", repositoryPath}, args...)
	command := exec.CommandContext(ctx, "git", commandArgs...)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if err == nil {
		return output, nil
	}
	detail := strings.TrimSpace(stderr.String())
	if detail == "" {
		return nil, err
	}
	return nil, errors.Join(err, errors.New(detail))
}
