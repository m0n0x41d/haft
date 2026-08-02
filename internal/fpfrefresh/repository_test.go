package fpfrefresh

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/m0n0x41d/haft/internal/fpf"
	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
	"github.com/m0n0x41d/haft/internal/fpf/typeenvsql"
	"github.com/m0n0x41d/haft/internal/racequalification"
	"golang.org/x/sys/unix"
)

const (
	effectsQueryDatabaseCachePrefix   = "haft-fpfrefresh-query-fixture-"
	effectsQueryDatabaseCacheContract = "haft-fpfrefresh-query-fixture/v2"
)

var effectsQueryDatabaseCache struct {
	sync.Mutex
	root  string
	owned bool
}

func TestMain(testingMain *testing.M) {
	code := testingMain.Run()
	effectsQueryDatabaseCache.Lock()
	root := effectsQueryDatabaseCache.root
	owned := effectsQueryDatabaseCache.owned
	effectsQueryDatabaseCache.root = ""
	effectsQueryDatabaseCache.owned = false
	effectsQueryDatabaseCache.Unlock()
	if owned &&
		root != "" &&
		strings.HasPrefix(filepath.Base(root), effectsQueryDatabaseCachePrefix) {
		_ = os.RemoveAll(root)
	}
	os.Exit(code)
}

func TestRepositoryCoordinatesRemainDistinctAndIntegrationFailsClosed(t *testing.T) {
	fixture := newRefreshEffectsFixture(t)
	ctx := context.Background()

	layout, err := ResolveRepositoryLayout(fixture.root)
	if err != nil {
		t.Fatalf("ResolveRepositoryLayout() error = %v", err)
	}
	assertRepositoryLayout(t, fixture, layout)
	fixture.checkoutSource(t, fixture.candidateSHA)

	tracked, err := TrackedSourceRevision(ctx, layout)
	if err != nil {
		t.Fatalf("TrackedSourceRevision() error = %v", err)
	}
	checkedOut, err := CheckedOutSourceRevision(ctx, layout)
	if err != nil {
		t.Fatalf("CheckedOutSourceRevision() error = %v", err)
	}
	databaseOnlyPath := filepath.Join(fixture.stateDirectory, "coordinate-only.db")
	writeIntegrationMetaDatabase(t, databaseOnlyPath, fixture.databaseOnlySHA)
	databaseRevision, err := DatabaseSourceRevision(databaseOnlyPath)
	if err != nil {
		t.Fatalf("DatabaseSourceRevision() error = %v", err)
	}
	if tracked != fixture.predecessorSHA ||
		checkedOut != fixture.candidateSHA ||
		databaseRevision != fixture.databaseOnlySHA {
		t.Fatalf(
			"coordinate distinction lost: tracked=%s checkout=%s database=%s",
			tracked,
			checkedOut,
			databaseRevision,
		)
	}
	if tracked == checkedOut ||
		tracked == databaseRevision ||
		checkedOut == databaseRevision {
		t.Fatal("fixture did not establish three distinct repository coordinates")
	}

	fixture.installCandidatePair(t)
	lock, err := verifyRepositoryIntegrationForTest(ctx, layout, "fpf-refresh-test", nil)
	if err != nil {
		t.Fatalf("VerifyRepositoryIntegration(coherent candidate) error = %v", err)
	}
	if lock.Coordinates.SourceRevision != fixture.candidateSHA ||
		lock.Coordinates.DatabaseDigest != fixture.candidateDatabaseDigest {
		t.Fatalf("verified candidate coordinates = %#v", lock.Coordinates)
	}

	t.Run("stale source checkout", func(t *testing.T) {
		fixture.installCandidatePair(t)
		fixture.checkoutSource(t, fixture.predecessorSHA)
		_, err := verifyRepositoryIntegrationForTest(ctx, layout, "fpf-refresh-test", nil)
		if err == nil || !strings.Contains(err.Error(), "snapshot_pin_stale") {
			t.Fatalf("stale source error = %v, want snapshot_pin_stale", err)
		}
		assertFileDigest(t, fixture.targetDatabase, fixture.candidateDatabaseDigest)
		assertFileDigest(t, fixture.targetLock, fixture.candidateLockDigest)
	})

	t.Run("stale database", func(t *testing.T) {
		fixture.installCandidatePair(t)
		copyEffectsFile(t, fixture.predecessorDatabaseBackup, fixture.targetDatabase)
		_, err := verifyRepositoryIntegrationForTest(ctx, layout, "fpf-refresh-test", nil)
		if err == nil || !strings.Contains(err.Error(), "snapshot_pin_stale") {
			t.Fatalf("stale database error = %v, want snapshot_pin_stale", err)
		}
		assertFileDigest(t, fixture.targetDatabase, fixture.predecessorDatabaseDigest)
		assertFileDigest(t, fixture.targetLock, fixture.candidateLockDigest)
	})

	t.Run("stale generated lock", func(t *testing.T) {
		fixture.installCandidatePair(t)
		copyEffectsFile(t, fixture.predecessorLockBackup, fixture.targetLock)
		_, err := verifyRepositoryIntegrationForTest(ctx, layout, "fpf-refresh-test", nil)
		if err == nil || !strings.Contains(err.Error(), "snapshot_pin_stale") {
			t.Fatalf("stale lock error = %v, want snapshot_pin_stale", err)
		}
		assertFileDigest(t, fixture.targetDatabase, fixture.candidateDatabaseDigest)
		assertFileDigest(t, fixture.targetLock, fixture.predecessorLockDigest)
	})

	t.Run("canonical generated_by drift", func(t *testing.T) {
		fixture.installCandidatePair(t)
		payload, err := os.ReadFile(fixture.targetLock)
		if err != nil {
			t.Fatal(err)
		}
		drifted, err := ParseIntegrationLock(payload)
		if err != nil {
			t.Fatalf("ParseIntegrationLock(candidate) error = %v", err)
		}
		drifted.GeneratedBy = "fpf-refresh-drifted"
		if err := WriteIntegrationLock(fixture.targetLock, drifted); err != nil {
			t.Fatal(err)
		}
		_, err = verifyRepositoryIntegrationForTest(
			ctx,
			layout,
			"fpf-refresh-test",
			nil,
		)
		if err == nil ||
			!strings.Contains(err.Error(), "integration lock generated_by") ||
			!strings.Contains(err.Error(), "current generator identity") {
			t.Fatalf("canonical generated_by drift error = %v", err)
		}
	})

	t.Run("holds operation lock through verified consumer", func(t *testing.T) {
		fixture.installCandidatePair(t)
		consumed := false
		err := VerifyRepositoryIntegration(
			ctx,
			layout,
			"fpf-refresh-test",
			nil,
			func(IntegrationLock) error {
				consumed = true
				secondRelease, acquireErr := acquireOperationLock(layout.Receipt)
				if secondRelease != nil {
					secondRelease()
					return fmt.Errorf("concurrent operation lock unexpectedly succeeded")
				}
				if !errors.Is(acquireErr, ErrReceiptBusy) {
					return fmt.Errorf(
						"concurrent operation lock error = %v, want ErrReceiptBusy",
						acquireErr,
					)
				}
				return nil
			},
		)
		if err != nil {
			t.Fatalf("VerifyRepositoryIntegration() error = %v", err)
		}
		if !consumed {
			t.Fatal("verified integration consumer did not run")
		}
		release, err := acquireOperationLock(layout.Receipt)
		if err != nil {
			t.Fatalf("operation lock remained held after verification: %v", err)
		}
		release()
	})

	t.Run("busy operation rejects verification", func(t *testing.T) {
		fixture.installCandidatePair(t)
		release, err := acquireOperationLock(layout.Receipt)
		if err != nil {
			t.Fatal(err)
		}
		consumed := false
		verifyErr := VerifyRepositoryIntegration(
			ctx,
			layout,
			"fpf-refresh-test",
			nil,
			func(IntegrationLock) error {
				consumed = true
				return nil
			},
		)
		release()
		if !errors.Is(verifyErr, ErrReceiptBusy) {
			t.Fatalf("busy verification error = %v, want ErrReceiptBusy", verifyErr)
		}
		if consumed {
			t.Fatal("busy verification ran the verified consumer")
		}
		assertFileDigest(t, fixture.targetDatabase, fixture.candidateDatabaseDigest)
		assertFileDigest(t, fixture.targetLock, fixture.candidateLockDigest)
	})

	t.Run("active recovery rejects verification", func(t *testing.T) {
		basis := fixture.receiptBasis(ReceiptLockPresent, fixture.predecessorSHA)
		prepareEffectsReceipt(
			t,
			fixture,
			basis,
			ReceiptStatePrepared,
			false,
		)
		defer func() {
			if err := os.Remove(layout.Receipt); err != nil && !errors.Is(err, os.ErrNotExist) {
				t.Errorf("remove active receipt: %v", err)
			}
		}()
		consumed := false
		err := VerifyRepositoryIntegration(
			ctx,
			layout,
			"fpf-refresh-test",
			nil,
			func(IntegrationLock) error {
				consumed = true
				return nil
			},
		)
		if !errors.Is(err, ErrRecoveryRequired) {
			t.Fatalf("active recovery verification error = %v, want ErrRecoveryRequired", err)
		}
		if consumed {
			t.Fatal("active recovery ran the verified consumer")
		}
	})

	t.Run("terminal receipt remains nonblocking", func(t *testing.T) {
		basis := fixture.receiptBasis(ReceiptLockPresent, fixture.predecessorSHA)
		prepareEffectsReceipt(
			t,
			fixture,
			basis,
			ReceiptStateComplete,
			false,
		)
		defer func() {
			if err := os.Remove(layout.Receipt); err != nil && !errors.Is(err, os.ErrNotExist) {
				t.Errorf("remove terminal receipt: %v", err)
			}
		}()
		if _, err := verifyRepositoryIntegrationForTest(
			ctx,
			layout,
			"fpf-refresh-test",
			nil,
		); err != nil {
			t.Fatalf("terminal-receipt verification error = %v", err)
		}
	})

	t.Run("fresh verification materializes only private operation carrier", func(t *testing.T) {
		fixture.installCandidatePair(t)
		freshLayout := layout
		freshLayout.StateDirectory = filepath.Join(
			fixture.root,
			".context",
			"fresh-verification",
		)
		freshLayout.Receipt = filepath.Join(
			freshLayout.StateDirectory,
			DefaultRefreshReceiptFilename,
		)
		if err := os.RemoveAll(freshLayout.StateDirectory); err != nil {
			t.Fatal(err)
		}
		if _, err := verifyRepositoryIntegrationForTest(
			ctx,
			freshLayout,
			"fpf-refresh-test",
			nil,
		); err != nil {
			t.Fatalf("fresh VerifyRepositoryIntegration() error = %v", err)
		}
		info, err := os.Lstat(freshLayout.StateDirectory)
		if err != nil {
			t.Fatal(err)
		}
		if !info.IsDir() || info.Mode().Perm()&0o077 != 0 {
			t.Fatalf("fresh verification state mode = %v, want private directory", info.Mode())
		}
		entries, err := os.ReadDir(freshLayout.StateDirectory)
		if err != nil {
			t.Fatal(err)
		}
		wantCarrier := filepath.Base(freshLayout.Receipt) + ".operation.lock"
		if len(entries) != 1 || entries[0].Name() != wantCarrier {
			t.Fatalf("fresh verification state entries = %v, want only %s", entries, wantCarrier)
		}
		assertFileDigest(t, fixture.targetDatabase, fixture.candidateDatabaseDigest)
		assertFileDigest(t, fixture.targetLock, fixture.candidateLockDigest)
	})

	t.Run("dirty source publication", func(t *testing.T) {
		fixture.installCandidatePair(t)
		specPath := filepath.Join(fixture.sourceRepository, gitSourceSpecPath)
		original, err := os.ReadFile(specPath)
		if err != nil {
			t.Fatal(err)
		}
		dirty := append(append([]byte(nil), original...), []byte("\nlocal unrelated edit\n")...)
		if err := os.WriteFile(specPath, dirty, 0o644); err != nil {
			t.Fatal(err)
		}
		_, verifyErr := verifyRepositoryIntegrationForTest(
			ctx,
			layout,
			"fpf-refresh-test",
			nil,
		)
		if verifyErr == nil || !strings.Contains(verifyErr.Error(), "snapshot_pin_stale") {
			t.Fatalf("dirty publication error = %v, want snapshot_pin_stale", verifyErr)
		}
		got, err := os.ReadFile(specPath)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(dirty) {
			t.Fatal("verification changed the dirty source publication")
		}
		if err := os.WriteFile(specPath, original, 0o644); err != nil {
			t.Fatal(err)
		}
	})

	fixture.installCandidatePair(t)
	if got := readEffectsFile(t, fixture.unrelatedSentinel); got != fixture.unrelatedSentinelBytes {
		t.Fatalf("repository verification changed unrelated file: %q", got)
	}
	trackedAfter, err := TrackedSourceRevision(ctx, layout)
	if err != nil {
		t.Fatal(err)
	}
	if trackedAfter != fixture.predecessorSHA {
		t.Fatalf("tracked gitlink changed to %s, want %s", trackedAfter, fixture.predecessorSHA)
	}
}

func verifyRepositoryIntegrationForTest(
	ctx context.Context,
	layout RepositoryLayout,
	expectedGeneratedBy string,
	tokenGate *TokenGateCoordinates,
) (IntegrationLock, error) {
	var verified IntegrationLock
	err := VerifyRepositoryIntegration(
		ctx,
		layout,
		expectedGeneratedBy,
		tokenGate,
		func(lock IntegrationLock) error {
			verified = lock
			return nil
		},
	)
	return verified, err
}

func TestValidateReportPathRejectsRepositoryAndRecoveryTargets(t *testing.T) {
	fixture := newRefreshEffectsFixture(t)
	layout, err := ResolveRepositoryLayout(fixture.root)
	if err != nil {
		t.Fatal(err)
	}
	allowedExternal := filepath.Join(t.TempDir(), "report.json")
	allowedState := filepath.Join(layout.StateDirectory, "review-report.json")
	for _, path := range []string{allowedExternal, allowedState} {
		if err := ValidateReportPath(layout, path); err != nil {
			t.Fatalf("ValidateReportPath(%s) error = %v", path, err)
		}
	}
	for _, path := range []string{
		layout.Database,
		layout.IntegrationLock,
		layout.TokenGateFixture,
		layout.Receipt,
		layout.CompletedReceipt,
		filepath.Join(layout.SourceRepository, gitSourceReadmePath),
		filepath.Join(layout.Root, "internal", "fpfrefresh", "check.go"),
		filepath.Join(layout.StateDirectory, "artifacts", "candidate.db"),
		filepath.Join(layout.StateDirectory, "receipts", "complete.json"),
	} {
		if err := ValidateReportPath(layout, path); err == nil {
			t.Fatalf("ValidateReportPath(%s) error = nil, want protected-path rejection", path)
		}
	}
}

func TestCheckoutExactSourceRejectsAndPreservesUnrelatedDirt(t *testing.T) {
	fixture := newRefreshEffectsFixture(t)
	ctx := context.Background()
	layout, err := ResolveRepositoryLayout(fixture.root)
	if err != nil {
		t.Fatal(err)
	}
	fixture.checkoutSource(t, fixture.candidateSHA)

	dirtPath := filepath.Join(fixture.sourceRepository, "operator-untracked.txt")
	const dirt = "operator-owned bytes\n"
	if err := os.WriteFile(dirtPath, []byte(dirt), 0o644); err != nil {
		t.Fatal(err)
	}
	err = CheckoutExactSource(ctx, layout, fixture.predecessorSHA)
	if err == nil || !strings.Contains(err.Error(), "unrelated dirt") {
		t.Fatalf("CheckoutExactSource(dirty) error = %v, want unrelated-dirt rejection", err)
	}
	current, currentErr := CheckedOutSourceRevision(ctx, layout)
	if currentErr != nil {
		t.Fatal(currentErr)
	}
	if current != fixture.candidateSHA {
		t.Fatalf("dirty checkout moved to %s, want %s", current, fixture.candidateSHA)
	}
	if got := readEffectsFile(t, dirtPath); got != dirt {
		t.Fatalf("dirty file changed to %q", got)
	}

	if err := os.Remove(dirtPath); err != nil {
		t.Fatal(err)
	}
	if err := CheckoutExactSource(ctx, layout, fixture.predecessorSHA); err != nil {
		t.Fatalf("CheckoutExactSource(clean) error = %v", err)
	}
	current, err = CheckedOutSourceRevision(ctx, layout)
	if err != nil {
		t.Fatal(err)
	}
	if current != fixture.predecessorSHA {
		t.Fatalf("clean checkout revision = %s, want %s", current, fixture.predecessorSHA)
	}
	tracked, err := TrackedSourceRevision(ctx, layout)
	if err != nil {
		t.Fatal(err)
	}
	if tracked != fixture.predecessorSHA {
		t.Fatalf("bounded source checkout changed root gitlink to %s", tracked)
	}
	if got := readEffectsFile(t, fixture.unrelatedSentinel); got != fixture.unrelatedSentinelBytes {
		t.Fatalf("bounded checkout changed unrelated file to %q", got)
	}
}

type refreshEffectsFixture struct {
	root                      string
	sourceRepository          string
	stateDirectory            string
	targetDatabase            string
	targetLock                string
	receiptPath               string
	candidateDatabase         string
	candidateLock             string
	predecessorDatabaseBackup string
	predecessorLockBackup     string
	unrelatedSentinel         string
	unrelatedSentinelBytes    string
	predecessorSHA            string
	candidateSHA              string
	databaseOnlySHA           string
	predecessorDatabaseDigest string
	candidateDatabaseDigest   string
	predecessorLockDigest     string
	candidateLockDigest       string
}

func newRefreshEffectsFixture(t *testing.T) *refreshEffectsFixture {
	t.Helper()
	fixture := &refreshEffectsFixture{
		root:                   t.TempDir(),
		unrelatedSentinelBytes: "operator-owned root bytes\n",
	}
	fixture.sourceRepository = filepath.Join(fixture.root, DefaultSourceRelativePath)
	fixture.stateDirectory = filepath.Join(fixture.root, DefaultRefreshStateRelativeDirectory)
	fixture.targetDatabase = filepath.Join(fixture.root, DefaultDatabaseRelativePath)
	fixture.targetLock = filepath.Join(fixture.root, DefaultIntegrationLockRelativePath)
	fixture.receiptPath = filepath.Join(fixture.stateDirectory, DefaultRefreshReceiptFilename)
	artifactDirectory := filepath.Join(fixture.stateDirectory, "prepared")
	fixture.candidateDatabase = filepath.Join(artifactDirectory, "candidate-fpf.db")
	fixture.candidateLock = filepath.Join(artifactDirectory, "candidate-lock.json")
	fixture.predecessorDatabaseBackup = filepath.Join(
		artifactDirectory,
		"predecessor-fpf.db",
	)
	fixture.predecessorLockBackup = filepath.Join(
		artifactDirectory,
		"predecessor-lock.json",
	)
	fixture.unrelatedSentinel = filepath.Join(fixture.root, "operator-unrelated.txt")
	for _, directory := range []string{
		fixture.sourceRepository,
		fixture.stateDirectory,
		artifactDirectory,
		filepath.Dir(fixture.targetDatabase),
		filepath.Dir(fixture.targetLock),
	} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	for _, name := range []string{gitSourceReadmePath, gitSourceSpecPath} {
		sourcePath := filepath.Join("..", "..", "data", "FPF", name)
		content, err := os.ReadFile(sourcePath)
		if err != nil {
			t.Fatalf("read production source fixture %s: %v", sourcePath, err)
		}
		targetPath := filepath.Join(fixture.sourceRepository, name)
		if err := os.WriteFile(targetPath, content, 0o644); err != nil {
			t.Fatalf("write source fixture %s: %v", targetPath, err)
		}
	}
	runEffectsGit(t, fixture.sourceRepository, "init", "--quiet")
	runEffectsGit(t, fixture.sourceRepository, "config", "user.name", "FPF Refresh Test")
	runEffectsGit(t, fixture.sourceRepository, "config", "user.email", "fpf-refresh@example.invalid")
	runEffectsGit(t, fixture.sourceRepository, "add", gitSourceReadmePath, gitSourceSpecPath)
	runEffectsGit(t, fixture.sourceRepository, "commit", "--quiet", "-m", "predecessor")
	fixture.predecessorSHA = runEffectsGit(
		t,
		fixture.sourceRepository,
		"rev-parse",
		"HEAD^{commit}",
	)

	versionMarker := filepath.Join(fixture.sourceRepository, "fixture-version.txt")
	if err := os.WriteFile(versionMarker, []byte("candidate\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runEffectsGit(t, fixture.sourceRepository, "add", "fixture-version.txt")
	runEffectsGit(t, fixture.sourceRepository, "commit", "--quiet", "-m", "candidate")
	fixture.candidateSHA = runEffectsGit(
		t,
		fixture.sourceRepository,
		"rev-parse",
		"HEAD^{commit}",
	)
	if err := os.WriteFile(versionMarker, []byte("database-only\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runEffectsGit(t, fixture.sourceRepository, "add", "fixture-version.txt")
	runEffectsGit(t, fixture.sourceRepository, "commit", "--quiet", "-m", "database-only")
	fixture.databaseOnlySHA = runEffectsGit(
		t,
		fixture.sourceRepository,
		"rev-parse",
		"HEAD^{commit}",
	)
	fixture.checkoutSource(t, fixture.candidateSHA)

	rootMarker := filepath.Join(fixture.root, "root-marker.txt")
	if err := os.WriteFile(rootMarker, []byte("root\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runEffectsGit(t, fixture.root, "init", "--quiet")
	runEffectsGit(t, fixture.root, "config", "user.name", "FPF Refresh Test")
	runEffectsGit(t, fixture.root, "config", "user.email", "fpf-refresh@example.invalid")
	runEffectsGit(t, fixture.root, "add", "root-marker.txt")
	runEffectsGit(
		t,
		fixture.root,
		"update-index",
		"--add",
		"--cacheinfo",
		"160000",
		fixture.predecessorSHA,
		DefaultSourceRelativePath,
	)
	runEffectsGit(t, fixture.root, "commit", "--quiet", "-m", "pin predecessor")
	if err := os.WriteFile(
		fixture.unrelatedSentinel,
		[]byte(fixture.unrelatedSentinelBytes),
		0o644,
	); err != nil {
		t.Fatal(err)
	}

	readmePath := filepath.Join(fixture.sourceRepository, gitSourceReadmePath)
	specPath := filepath.Join(fixture.sourceRepository, gitSourceSpecPath)
	for _, database := range []struct {
		path     string
		revision string
	}{
		{path: fixture.predecessorDatabaseBackup, revision: fixture.predecessorSHA},
		{path: fixture.candidateDatabase, revision: fixture.candidateSHA},
	} {
		writeEffectsQueryDatabase(
			t,
			database.path,
			readmePath,
			specPath,
			database.revision,
		)
	}
	installEffectsLocalPracticeCandidate(
		t,
		fixture.root,
		fixture.candidateSHA,
		fixture.candidateDatabase,
	)
	fixture.predecessorDatabaseDigest = effectsDigest(t, fixture.predecessorDatabaseBackup)
	fixture.candidateDatabaseDigest = effectsDigest(t, fixture.candidateDatabase)

	predecessorLock, err := BuildIntegrationLock(IntegrationCoordinateInput{
		SourceRevision: fixture.predecessorSHA,
		ReadmePath:     readmePath,
		SpecPath:       specPath,
		DatabasePath:   fixture.predecessorDatabaseBackup,
		GeneratedBy:    "fpf-refresh-test",
	})
	if err != nil {
		t.Fatalf("BuildIntegrationLock(predecessor) error = %v", err)
	}
	if err := WriteIntegrationLock(fixture.predecessorLockBackup, predecessorLock); err != nil {
		t.Fatal(err)
	}
	candidateLock, err := BuildIntegrationLock(IntegrationCoordinateInput{
		SourceRevision: fixture.candidateSHA,
		ReadmePath:     readmePath,
		SpecPath:       specPath,
		DatabasePath:   fixture.candidateDatabase,
		GeneratedBy:    "fpf-refresh-test",
	})
	if err != nil {
		t.Fatalf("BuildIntegrationLock(candidate) error = %v", err)
	}
	if err := WriteIntegrationLock(fixture.candidateLock, candidateLock); err != nil {
		t.Fatal(err)
	}
	fixture.predecessorLockDigest = effectsDigest(t, fixture.predecessorLockBackup)
	fixture.candidateLockDigest = effectsDigest(t, fixture.candidateLock)
	fixture.installPredecessorPair(t, ReceiptLockPresent)
	return fixture
}

func installEffectsLocalPracticeCandidate(
	t *testing.T,
	root string,
	sourceRevision string,
	candidateDatabasePath string,
) {
	t.Helper()
	sourcePath := filepath.Join("..", "..", DefaultLocalPracticeCandidateRelative)
	content, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("read production Local-Practice fixture: %v", err)
	}
	productionDatabase, err := openIntegrationDatabaseReadOnly(
		filepath.Join("..", "cli", "fpf.db"),
	)
	if err != nil {
		t.Fatalf("open production FPF database: %v", err)
	}
	defer func() { _ = productionDatabase.Close() }()
	productionMeta, err := readRequiredIntegrationMeta(productionDatabase)
	if err != nil {
		t.Fatalf("read production FPF coordinates: %v", err)
	}
	productionRevision := productionMeta["fpf_commit"]
	if count := bytes.Count(content, []byte(productionRevision)); count == 0 {
		t.Fatalf("production Local-Practice fixture has no source revision %s", productionRevision)
	}
	candidate := bytes.ReplaceAll(
		content,
		[]byte(productionRevision),
		[]byte(sourceRevision),
	)
	candidateDatabase, err := openIntegrationDatabaseReadOnly(candidateDatabasePath)
	if err != nil {
		t.Fatalf("open candidate FPF database: %v", err)
	}
	defer func() { _ = candidateDatabase.Close() }()
	candidateMeta, err := readRequiredIntegrationMeta(candidateDatabase)
	if err != nil {
		t.Fatalf("read candidate FPF coordinates: %v", err)
	}
	productionBase := productionMeta["typeenv_ref"]
	if count := bytes.Count(candidate, []byte(productionBase)); count != 1 {
		t.Fatalf("production Local-Practice Base ref count = %d, want 1", count)
	}
	candidate = bytes.Replace(
		candidate,
		[]byte(productionBase),
		[]byte(candidateMeta["typeenv_ref"]),
		1,
	)
	targetPath := filepath.Join(root, DefaultLocalPracticeCandidateRelative)
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		t.Fatalf("create Local-Practice fixture directory: %v", err)
	}
	if err := os.WriteFile(targetPath, candidate, 0o600); err != nil {
		t.Fatalf("write Local-Practice fixture: %v", err)
	}
}

func (fixture *refreshEffectsFixture) receiptBasis(
	presence ReceiptLockPresence,
	initialSourceSHA string,
) ReceiptBasis {
	predecessorLock := ReceiptPredecessorLock{Presence: presence}
	if presence == ReceiptLockPresent {
		predecessorLock.BackupPath = fixture.predecessorLockBackup
		predecessorLock.Digest = fixture.predecessorLockDigest
	}
	return ReceiptBasis{
		Predecessor: ReceiptCoordinates{
			SourceSHA:      fixture.predecessorSHA,
			DatabaseDigest: fixture.predecessorDatabaseDigest,
		},
		Candidate: ReceiptCoordinates{
			SourceSHA:      fixture.candidateSHA,
			DatabaseDigest: fixture.candidateDatabaseDigest,
		},
		InitialSourceSHA: initialSourceSHA,
		Targets: ReceiptTargets{
			SourcePath:   fixture.sourceRepository,
			DatabasePath: fixture.targetDatabase,
			LockPath:     fixture.targetLock,
		},
		Artifacts: ReceiptArtifacts{
			CandidateDatabasePath:         fixture.candidateDatabase,
			CandidateLockPath:             fixture.candidateLock,
			CandidateLockDigest:           fixture.candidateLockDigest,
			PredecessorDatabaseBackupPath: fixture.predecessorDatabaseBackup,
			PredecessorLock:               predecessorLock,
		},
	}
}

func (fixture *refreshEffectsFixture) installPredecessorPair(
	t *testing.T,
	presence ReceiptLockPresence,
) {
	t.Helper()
	fixture.checkoutSource(t, fixture.predecessorSHA)
	copyEffectsFile(t, fixture.predecessorDatabaseBackup, fixture.targetDatabase)
	switch presence {
	case ReceiptLockMissing:
		if err := os.Remove(fixture.targetLock); err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
	case ReceiptLockPresent:
		copyEffectsFile(t, fixture.predecessorLockBackup, fixture.targetLock)
	default:
		t.Fatalf("unsupported predecessor lock presence %q", presence)
	}
}

func (fixture *refreshEffectsFixture) installCandidatePair(t *testing.T) {
	t.Helper()
	fixture.checkoutSource(t, fixture.candidateSHA)
	copyEffectsFile(t, fixture.candidateDatabase, fixture.targetDatabase)
	copyEffectsFile(t, fixture.candidateLock, fixture.targetLock)
}

func (fixture *refreshEffectsFixture) checkoutSource(t *testing.T, revision string) {
	t.Helper()
	runEffectsGit(t, fixture.sourceRepository, "checkout", "--quiet", "--detach", revision)
}

func assertRepositoryLayout(
	t *testing.T,
	fixture *refreshEffectsFixture,
	layout RepositoryLayout,
) {
	t.Helper()
	want := map[string]string{
		"root":                   fixture.root,
		"source":                 fixture.sourceRepository,
		"database":               fixture.targetDatabase,
		"lock":                   fixture.targetLock,
		"token-gate fixture":     filepath.Join(fixture.root, DefaultTokenGateFixtureRelativePath),
		"state":                  fixture.stateDirectory,
		"receipt":                fixture.receiptPath,
		"report":                 filepath.Join(fixture.stateDirectory, DefaultRefreshReportFilename),
		"completed receipt":      filepath.Join(fixture.stateDirectory, DefaultRefreshCompletedReceiptFilename),
		"Local-Practice carrier": filepath.Join(fixture.root, DefaultLocalPracticeCandidateRelative),
	}
	got := map[string]string{
		"root":                   layout.Root,
		"source":                 layout.SourceRepository,
		"database":               layout.Database,
		"lock":                   layout.IntegrationLock,
		"token-gate fixture":     layout.TokenGateFixture,
		"state":                  layout.StateDirectory,
		"receipt":                layout.Receipt,
		"report":                 layout.Report,
		"completed receipt":      layout.CompletedReceipt,
		"Local-Practice carrier": layout.LatestLocalPracticeCandidate,
	}
	for name, expected := range want {
		if got[name] != expected {
			t.Fatalf("%s path = %q, want %q", name, got[name], expected)
		}
		if !filepath.IsAbs(got[name]) || filepath.Clean(got[name]) != got[name] {
			t.Fatalf("%s path is not absolute and clean: %q", name, got[name])
		}
	}
}

func writeIntegrationMetaDatabase(t *testing.T, path string, revision string) {
	t.Helper()
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	database, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	values := map[string]string{
		"fpf_commit":                      revision,
		"indexed_source_units":            "1",
		"readme_document_digest":          effectsRepeatedDigest("1"),
		"schema_version":                  "11",
		"spec_document_digest":            effectsRepeatedDigest("2"),
		"typeenv_artifact_digest":         effectsRepeatedDigest("3"),
		"typeenv_compiler_schema_version": "fpf-base-typeenv.cov2.v4",
		"typeenv_ref":                     "typeenv:" + effectsRepeatedDigest("3"),
		"typeenv_source_revision":         revision,
	}
	if _, err := database.Exec(`CREATE TABLE meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)`); err != nil {
		_ = database.Close()
		t.Fatal(err)
	}
	for key, value := range values {
		if _, err := database.Exec(`INSERT INTO meta(key, value) VALUES (?, ?)`, key, value); err != nil {
			_ = database.Close()
			t.Fatal(err)
		}
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
}

func writeEffectsQueryDatabase(
	t *testing.T,
	path string,
	readmePath string,
	specPath string,
	revision string,
) {
	t.Helper()
	cachePath := cachedEffectsQueryDatabase(
		t,
		readmePath,
		specPath,
		revision,
	)
	cacheDigest := effectsDigest(t, cachePath)
	copyEffectsFile(t, cachePath, path)
	assertFileDigest(t, path, cacheDigest)
}

func cachedEffectsQueryDatabase(
	t *testing.T,
	readmePath string,
	specPath string,
	revision string,
) string {
	t.Helper()
	readme, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatal(err)
	}
	specification, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatal(err)
	}

	effectsQueryDatabaseCache.Lock()
	defer effectsQueryDatabaseCache.Unlock()

	cacheRoot := effectsQueryDatabaseCacheRoot(t)
	cacheBasis := strings.Join(
		[]string{
			effectsQueryDatabaseCacheContract,
			fpf.SpecIndexSchemaVersion,
			typeenv.BaseTypeEnvCompilerSchemaV5,
			revision,
			digestBytesSHA256(readme),
			digestBytesSHA256(specification),
		},
		"\x00",
	)
	cacheKey := strings.TrimPrefix(
		digestBytesSHA256([]byte(cacheBasis)),
		"sha256:",
	)
	path := filepath.Join(cacheRoot, cacheKey+".db")
	unlock := lockEffectsQueryDatabaseCache(t, path+".lock")
	defer unlock()

	if reusable, err := reusableEffectsQueryDatabase(path); err != nil {
		t.Fatal(err)
	} else if reusable {
		return path
	}

	temporary, err := os.CreateTemp(cacheRoot, "."+cacheKey+"-*.db")
	if err != nil {
		t.Fatal(err)
	}
	temporaryPath := temporary.Name()
	if err := temporary.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(temporaryPath); err != nil {
		t.Fatal(err)
	}
	published := false
	defer func() {
		if published {
			return
		}
		for _, suffix := range []string{"", "-shm", "-wal"} {
			_ = os.Remove(temporaryPath + suffix)
		}
	}()

	snapshot, err := buildLogicalPublicationSnapshot(
		readme,
		specification,
		revision,
	)
	if err != nil {
		t.Fatalf("buildLogicalPublicationSnapshot(%s) error = %v", revision, err)
	}
	units := snapshot.SourceUnits()
	if err := fpf.StoreSourceUnits(temporaryPath, units); err != nil {
		t.Fatalf("StoreSourceUnits(%s) error = %v", temporaryPath, err)
	}
	database, err := sql.Open("sqlite", temporaryPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(
		`CREATE TABLE meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)`,
	); err != nil {
		_ = database.Close()
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	compilation, err := typeenv.CompileBaseTypeEnv(snapshot)
	if err != nil {
		t.Fatalf("CompileBaseTypeEnv(%s) error = %v", revision, err)
	}
	artifact, accepted := compilation.Artifact()
	if !accepted {
		t.Fatalf(
			"CompileBaseTypeEnv(%s) rejected fixture: %v",
			revision,
			compilation.Diagnostics(),
		)
	}
	envelope, err := typeenv.NewCompilationEnvelope(
		artifact,
		typeenv.NewInitialCompatibilityAssessment(),
	)
	if err != nil {
		t.Fatalf("NewCompilationEnvelope(%s) error = %v", revision, err)
	}
	database, err = sql.Open("sqlite", temporaryPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := typeenvsql.ReplaceEnvelopeDB(
		context.Background(),
		database,
		envelope,
	); err != nil {
		_ = database.Close()
		t.Fatalf("ReplaceEnvelopeDB(%s) error = %v", revision, err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	readmeDigest, err := digestFile(readmePath)
	if err != nil {
		t.Fatal(err)
	}
	specDigest, err := digestFile(specPath)
	if err != nil {
		t.Fatal(err)
	}
	typeEnvDigest := artifact.Digest().String()
	typeEnvRef, exists := artifact.TypeEnvRef()
	if !exists {
		t.Fatalf("compiled fixture TypeEnv %s has no reference", revision)
	}
	if err := fpf.SetSpecMetaEntries(temporaryPath, map[string]string{
		"fpf_commit":                      revision,
		"indexed_source_units":            fmt.Sprintf("%d", len(units)),
		"readme_document_digest":          readmeDigest,
		"schema_version":                  fpf.SpecIndexSchemaVersion,
		"spec_document_digest":            specDigest,
		"typeenv_artifact_digest":         typeEnvDigest,
		"typeenv_compiler_schema_version": artifact.CompilerSchemaVersion().String(),
		"typeenv_ref":                     typeEnvRef.String(),
		"typeenv_source_revision":         revision,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyCandidateQueryContract(temporaryPath); err != nil {
		t.Fatalf("verify reusable query fixture before publication: %v", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		t.Fatalf("publish reusable query fixture: %v", err)
	}
	published = true
	return path
}

func effectsQueryDatabaseCacheRoot(t *testing.T) string {
	t.Helper()
	if effectsQueryDatabaseCache.root != "" {
		return effectsQueryDatabaseCache.root
	}

	externalRoot := os.Getenv(
		racequalification.SharedFixtureDirectoryEnvironment,
	)
	if externalRoot == "" {
		root, err := os.MkdirTemp("", effectsQueryDatabaseCachePrefix)
		if err != nil {
			t.Fatal(err)
		}
		effectsQueryDatabaseCache.root = root
		effectsQueryDatabaseCache.owned = true
		return root
	}
	if externalRoot != strings.TrimSpace(externalRoot) ||
		!filepath.IsAbs(externalRoot) ||
		filepath.Clean(externalRoot) != externalRoot {
		t.Fatalf(
			"%s must name an absolute clean path",
			racequalification.SharedFixtureDirectoryEnvironment,
		)
	}
	assertPrivateEffectsCacheDirectory(t, externalRoot)

	root := filepath.Join(externalRoot, "internal-fpfrefresh-query-databases")
	if err := os.Mkdir(root, 0o700); err != nil &&
		!errors.Is(err, os.ErrExist) {
		t.Fatal(err)
	}
	assertPrivateEffectsCacheDirectory(t, root)
	effectsQueryDatabaseCache.root = root
	effectsQueryDatabaseCache.owned = false
	return root
}

func assertPrivateEffectsCacheDirectory(t *testing.T, path string) {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		t.Fatalf("shared query fixture root %s is not a real directory", path)
	}
	if info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("shared query fixture root %s is not private", path)
	}
}

func lockEffectsQueryDatabaseCache(t *testing.T, path string) func() {
	t.Helper()
	lock, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := unix.Flock(
		int(lock.Fd()), // #nosec G115 -- Fd is the descriptor of this open lock file.
		unix.LOCK_EX,
	); err != nil {
		_ = lock.Close()
		t.Fatal(err)
	}
	return func() {
		if err := unix.Flock(
			int(lock.Fd()), // #nosec G115 -- Fd is the descriptor of this open lock file.
			unix.LOCK_UN,
		); err != nil {
			t.Errorf("unlock shared query fixture: %v", err)
		}
		if err := lock.Close(); err != nil {
			t.Errorf("close shared query fixture lock: %v", err)
		}
	}
}

func reusableEffectsQueryDatabase(path string) (bool, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !info.Mode().IsRegular() {
		return false, fmt.Errorf(
			"cached query fixture %s is not a regular file",
			path,
		)
	}
	// A cache entry becomes visible only after the builder closes and verifies
	// its private temporary database, then atomically renames it while holding
	// this key's interprocess lock. Every caller proves that its private copy is
	// byte-identical to that validated artifact. Re-running the complete query
	// suite here would serialize the same assertion under the cache lock.
	return true, nil
}

func runEffectsGit(t *testing.T, repositoryPath string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", repositoryPath}, args...)...)
	command.Env = append(
		os.Environ(),
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_DATE=2026-01-01T00:00:00Z",
		"GIT_COMMITTER_DATE=2026-01-01T00:00:00Z",
	)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}

func copyEffectsFile(t *testing.T, sourcePath string, targetPath string) {
	t.Helper()
	content, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("read %s: %v", sourcePath, err)
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(targetPath, content, 0o644); err != nil {
		t.Fatalf("write %s: %v", targetPath, err)
	}
}

func readEffectsFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}

func effectsDigest(t *testing.T, path string) string {
	t.Helper()
	digest, err := digestFile(path)
	if err != nil {
		t.Fatalf("digest %s: %v", path, err)
	}
	return digest
}

func assertFileDigest(t *testing.T, path string, want string) {
	t.Helper()
	if got := effectsDigest(t, path); got != want {
		t.Fatalf("%s digest = %s, want %s", path, got, want)
	}
}

func effectsRepeatedDigest(character string) string {
	return fmt.Sprintf("sha256:%s", strings.Repeat(character, 64))
}
