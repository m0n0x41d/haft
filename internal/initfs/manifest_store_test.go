package initfs

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/m0n0x41d/haft/internal/initplanning"
)

func TestManifestStorePersistsCanonicalManifestAndIsIdempotent(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".haft", "host-installations", "codex.project.json")
	manifest := mustManifestForContent(t, root, []byte("canonical skill"))
	store := mustManifestStore(t, root, path)

	before, err := store.Read()
	if err != nil {
		t.Fatalf("read missing manifest: %v", err)
	}
	if before.Kind() != ManifestReadMissing || before.Path() != path {
		t.Fatalf("missing read = %#v", before)
	}
	first, err := store.Persist(manifest, ExpectManifestMissing())
	if err != nil {
		t.Fatalf("persist first manifest: %v", err)
	}
	if first.Kind() != ManifestPersisted ||
		first.DesiredDigest() != manifest.Digest() ||
		first.ObservedDigest() != "" {
		t.Fatalf("first persist = %#v", first)
	}
	assertStoredManifest(t, store, manifest)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat manifest: %v", err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Fatalf("manifest mode = %o, want 644", info.Mode().Perm())
	}

	retry, err := store.Persist(manifest, ExpectManifestMissing())
	if err != nil {
		t.Fatalf("idempotent missing-precondition retry: %v", err)
	}
	if retry.Kind() != ManifestAlreadyCurrent ||
		retry.ObservedDigest() != manifest.Digest() {
		t.Fatalf("idempotent retry = %#v", retry)
	}
	expected, err := ExpectManifestWithDigest(manifest.Digest())
	if err != nil {
		t.Fatalf("build exact precondition: %v", err)
	}
	exactRetry, err := store.Persist(manifest, expected)
	if err != nil {
		t.Fatalf("idempotent exact retry: %v", err)
	}
	if exactRetry.Kind() != ManifestAlreadyCurrent {
		t.Fatalf("exact retry kind = %s", exactRetry.Kind())
	}
	assertNoManifestStagesAndSafeLock(t, path)
}

func TestManifestStoreCompareAndSwapPreservesUnexpectedManifest(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".haft", "host-installations", "codex.project.json")
	firstManifest := mustManifestForContent(t, root, []byte("first"))
	secondManifest := mustManifestForContent(t, root, []byte("second"))
	store := mustManifestStore(t, root, path)
	if _, err := store.Persist(firstManifest, ExpectManifestMissing()); err != nil {
		t.Fatalf("persist first manifest: %v", err)
	}

	wrong, err := ExpectManifestWithDigest("sha256:" + strings.Repeat("f", 64))
	if err != nil {
		t.Fatalf("build wrong precondition: %v", err)
	}
	rejected, err := store.Persist(secondManifest, wrong)
	if err != nil {
		t.Fatalf("reject stale precondition: %v", err)
	}
	if rejected.Kind() != ManifestPreconditionFailed ||
		rejected.ObservedDigest() != firstManifest.Digest() {
		t.Fatalf("stale outcome = %#v", rejected)
	}
	assertStoredManifest(t, store, firstManifest)

	expected, err := ExpectManifestWithDigest(firstManifest.Digest())
	if err != nil {
		t.Fatalf("build first precondition: %v", err)
	}
	applied, err := store.Persist(secondManifest, expected)
	if err != nil {
		t.Fatalf("replace manifest: %v", err)
	}
	if applied.Kind() != ManifestPersisted ||
		applied.ObservedDigest() != firstManifest.Digest() {
		t.Fatalf("replace outcome = %#v", applied)
	}
	assertStoredManifest(t, store, secondManifest)

	staleButCurrent, err := store.Persist(secondManifest, expected)
	if err != nil {
		t.Fatalf("idempotent stale-precondition retry: %v", err)
	}
	if staleButCurrent.Kind() != ManifestAlreadyCurrent {
		t.Fatalf("stale-but-current kind = %s", staleButCurrent.Kind())
	}
	assertNoManifestStagesAndSafeLock(t, path)
}

func TestManifestStoreConcurrentFirstPersistPublishesExactlyOneManifest(
	t *testing.T,
) {
	root := t.TempDir()
	path := filepath.Join(root, ".haft", "host-installations", "codex.project.json")
	manifest := mustManifestForContent(t, root, []byte("concurrent"))
	store := mustManifestStore(t, root, path)

	const workers = 24
	start := make(chan struct{})
	results := make(chan ManifestPersistOutcome, workers)
	failures := make(chan error, workers)
	var group sync.WaitGroup
	group.Add(workers)
	for range workers {
		go func() {
			defer group.Done()
			<-start
			outcome, err := store.Persist(manifest, ExpectManifestMissing())
			if err != nil {
				failures <- err
				return
			}
			results <- outcome
		}()
	}
	close(start)
	group.Wait()
	close(results)
	close(failures)
	for err := range failures {
		t.Fatalf("concurrent persist: %v", err)
	}
	persisted := 0
	for outcome := range results {
		switch outcome.Kind() {
		case ManifestPersisted:
			persisted++
		case ManifestAlreadyCurrent, ManifestLockHeld:
		default:
			t.Fatalf("concurrent outcome = %#v", outcome)
		}
	}
	if persisted != 1 {
		t.Fatalf("persisted outcomes = %d, want exactly 1", persisted)
	}
	assertStoredManifest(t, store, manifest)
	assertNoManifestStagesAndSafeLock(t, path)
}

func TestManifestStoreLeaseSerializesAWholePublicationWindow(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".haft", "host-installations", "codex.project.json")
	manifest := mustManifestForContent(t, root, []byte("leased"))
	store := mustManifestStore(t, root, path)

	lease, acquired, err := store.TryAcquire()
	if err != nil {
		t.Fatalf("acquire manifest lease: %v", err)
	}
	if !acquired || lease == nil {
		t.Fatal("first manifest lease was not acquired")
	}
	second, acquired, err := store.TryAcquire()
	if err != nil {
		t.Fatalf("contended manifest lease: %v", err)
	}
	if acquired || second != nil {
		t.Fatal("contended manifest lease was acquired")
	}
	outcome, err := lease.Persist(manifest, ExpectManifestMissing())
	if err != nil {
		t.Fatalf("persist through lease: %v", err)
	}
	if outcome.Kind() != ManifestPersisted {
		t.Fatalf("leased persist kind = %s", outcome.Kind())
	}
	if err := lease.Release(); err != nil {
		t.Fatalf("release manifest lease: %v", err)
	}
	if err := lease.Release(); err != nil {
		t.Fatalf("idempotent lease release: %v", err)
	}
	if _, err := lease.Read(); err == nil {
		t.Fatal("released lease remained readable")
	}
	next, acquired, err := store.TryAcquire()
	if err != nil {
		t.Fatalf("reacquire manifest lease: %v", err)
	}
	if !acquired || next == nil {
		t.Fatal("manifest lease was not reacquired after release")
	}
	if err := next.Release(); err != nil {
		t.Fatalf("release reacquired lease: %v", err)
	}
	assertStoredManifest(t, store, manifest)
	assertNoManifestStagesAndSafeLock(t, path)
}

func TestManifestStoreLeaseReconcilesOnlyBoundedRegularStageDebt(t *testing.T) {
	t.Run("regular debt", func(t *testing.T) {
		root := t.TempDir()
		path := filepath.Join(root, "manifest.json")
		store := mustManifestStore(t, root, path)
		for _, base := range []string{
			filepath.Base(path),
			filepath.Base(path + ".pending"),
		} {
			name := canonicalCarrierStagePrefix(base) + strings.Repeat("a", 32)
			stage := filepath.Join(root, name)
			if err := os.WriteFile(stage, []byte("stale"), 0o600); err != nil {
				t.Fatalf("write stale stage: %v", err)
			}
		}
		lease, acquired, err := store.TryAcquire()
		if err != nil {
			t.Fatalf("acquire after stage debt: %v", err)
		}
		if !acquired {
			t.Fatal("lease was not acquired after regular stage debt")
		}
		if err := lease.Release(); err != nil {
			t.Fatalf("release lease: %v", err)
		}
		assertNoManifestStagesAndSafeLock(t, path)
	})

	t.Run("non-regular debt", func(t *testing.T) {
		root := t.TempDir()
		path := filepath.Join(root, "manifest.json")
		store := mustManifestStore(t, root, path)
		name := canonicalCarrierStagePrefix(filepath.Base(path)) + strings.Repeat("b", 32)
		stage := filepath.Join(root, name)
		if err := os.Mkdir(stage, 0o700); err != nil {
			t.Fatalf("create non-regular stage: %v", err)
		}
		if _, acquired, err := store.TryAcquire(); err == nil || acquired {
			t.Fatal("non-regular stage debt was accepted")
		}
		info, err := os.Stat(stage)
		if err != nil || !info.IsDir() {
			t.Fatalf("non-regular stage was removed or changed: %v", err)
		}
	})
}

func TestManifestStoreRejectsSymlinkedParentAndMalformedCarrier(t *testing.T) {
	t.Run("symlinked parent", func(t *testing.T) {
		root := t.TempDir()
		outside := t.TempDir()
		linked := filepath.Join(root, "linked")
		if err := os.Symlink(outside, linked); err != nil {
			t.Fatalf("create parent symlink: %v", err)
		}
		path := filepath.Join(linked, "manifest.json")
		manifest := mustManifestForContent(t, root, []byte("safe"))
		store := mustManifestStore(t, root, path)
		_, err := store.Persist(manifest, ExpectManifestMissing())
		assertObservationFailure(t, err, ObservationUnsafePath, linked)
		if _, err := os.Stat(filepath.Join(outside, "manifest.json")); !os.IsNotExist(err) {
			t.Fatalf("outside manifest exists=%v err=%v", err == nil, err)
		}
	})

	t.Run("malformed carrier", func(t *testing.T) {
		root := t.TempDir()
		path := filepath.Join(root, "manifest.json")
		if err := os.WriteFile(path, []byte("{\"schema\":\"foreign\"}"), 0o644); err != nil {
			t.Fatalf("write malformed manifest: %v", err)
		}
		store := mustManifestStore(t, root, path)
		if _, err := store.Read(); err == nil {
			t.Fatal("malformed manifest was accepted")
		}
	})

	t.Run("symlinked lock", func(t *testing.T) {
		root := t.TempDir()
		path := filepath.Join(root, "manifest.json")
		outside := filepath.Join(t.TempDir(), "outside-lock")
		if err := os.WriteFile(outside, []byte("outside"), 0o600); err != nil {
			t.Fatalf("write outside lock: %v", err)
		}
		if err := os.Symlink(outside, path+".lock"); err != nil {
			t.Fatalf("create lock symlink: %v", err)
		}
		store := mustManifestStore(t, root, path)
		if _, _, err := store.TryAcquire(); err == nil {
			t.Fatal("symlinked manifest lock was accepted")
		}
		assertFileBytes(t, outside, []byte("outside"))
	})
}

func TestManifestStoreRejectsInvalidConstructionAndPrecondition(t *testing.T) {
	root := t.TempDir()
	if _, err := NewManifestStore(root, root, 1024); err == nil {
		t.Fatal("root itself was accepted as a manifest path")
	}
	if _, err := NewManifestStore(root, filepath.Join(t.TempDir(), "manifest"), 1024); err == nil {
		t.Fatal("path outside root was accepted")
	}
	if _, err := NewManifestStore(root, filepath.Join(root, "manifest"), 0); err == nil {
		t.Fatal("zero byte limit was accepted")
	}
	if _, err := ExpectManifestWithDigest("not-a-digest"); err == nil {
		t.Fatal("invalid digest precondition was accepted")
	}
}

func mustManifestStore(
	t *testing.T,
	root string,
	path string,
) ManifestStore {
	t.Helper()
	store, err := NewManifestStore(root, path, 1<<20)
	if err != nil {
		t.Fatalf("new manifest store: %v", err)
	}
	return store
}

func mustManifestForContent(
	t *testing.T,
	root string,
	content []byte,
) initplanning.InstallationManifest {
	t.Helper()
	path := filepath.Join(root, "skills", "h-reason", "SKILL.md")
	output := mustObservationOutput(t, path, content)
	projection := mustObservationProjection(
		t,
		root,
		[]initplanning.RenderedOutput{output},
	)
	observation, err := initplanning.ObserveMissingPath(
		path,
		initplanning.ComponentSkills,
	)
	if err != nil {
		t.Fatalf("build missing observation: %v", err)
	}
	currentness, err := initplanning.ClassifyFirstInstallationCurrentness(
		projection,
		[]initplanning.PathObservation{observation},
		initplanning.WithoutKnownLegacyRegistry(),
	)
	if err != nil {
		t.Fatalf("classify first installation: %v", err)
	}
	plan, err := initplanning.CompileHostAdapterReconciliation(currentness)
	if err != nil {
		t.Fatalf("compile first installation: %v", err)
	}
	manifest, err := initplanning.BuildInstallationManifest(plan)
	if err != nil {
		t.Fatalf("build installation manifest: %v", err)
	}
	return manifest
}

func assertStoredManifest(
	t *testing.T,
	store ManifestStore,
	want initplanning.InstallationManifest,
) {
	t.Helper()
	observed, err := store.Read()
	if err != nil {
		t.Fatalf("read stored manifest: %v", err)
	}
	if observed.Kind() != ManifestReadPresent ||
		observed.Manifest().Digest() != want.Digest() ||
		string(observed.Manifest().CanonicalBytes()) != string(want.CanonicalBytes()) {
		t.Fatalf(
			"stored manifest = kind %s digest %s, want %s",
			observed.Kind(),
			observed.Manifest().Digest(),
			want.Digest(),
		)
	}
}

func assertNoManifestStagesAndSafeLock(t *testing.T, manifestPath string) {
	t.Helper()
	stages, err := filepath.Glob(
		filepath.Join(
			filepath.Dir(manifestPath),
			canonicalCarrierStagePrefix(filepath.Base(manifestPath))+"*",
		),
	)
	if err != nil {
		t.Fatalf("glob manifest stages: %v", err)
	}
	if len(stages) != 0 {
		t.Fatalf("manifest stages remain: %v", stages)
	}
	journalStages, err := filepath.Glob(
		filepath.Join(
			filepath.Dir(manifestPath),
			canonicalCarrierStagePrefix(filepath.Base(manifestPath+".pending"))+"*",
		),
	)
	if err != nil {
		t.Fatalf("glob publication journal stages: %v", err)
	}
	if len(journalStages) != 0 {
		t.Fatalf("publication journal stages remain: %v", journalStages)
	}
	info, err := os.Lstat(manifestPath + ".lock")
	if errors.Is(err, os.ErrNotExist) {
		return
	}
	if err != nil {
		t.Fatalf("inspect manifest lock: %v", err)
	}
	if !info.Mode().IsRegular() ||
		info.Mode().Perm() != 0o600 ||
		info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("manifest lock is not a safe regular 0600 carrier: %v", info.Mode())
	}
}
