package initfs

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/m0n0x41d/haft/internal/initplanning"
)

func TestHostPublisherMergesManagedFragmentWithoutOwningSharedCarrier(
	t *testing.T,
) {
	root := t.TempDir()
	projection, carrierPath := mustCoherentPublicationProjection(t, root)
	if err := os.MkdirAll(filepath.Dir(carrierPath), 0o755); err != nil {
		t.Fatalf("create shared carrier parent: %v", err)
	}
	if err := os.WriteFile(
		carrierPath,
		[]byte(`{"theme":"dark"}`),
		0o640,
	); err != nil {
		t.Fatalf("write user-owned shared carrier: %v", err)
	}
	store := mustManifestStore(
		t,
		root,
		filepath.Join(
			root,
			".haft",
			"host-installations",
			"codex.project.json",
		),
	)
	firstBatch := mustFirstCoherentBatch(t, projection)
	first, err := mustHostPublisher(t).Publish(firstBatch, store)
	if err != nil {
		t.Fatalf("publish coherent host: %v", err)
	}
	if first.Kind() != HostPublicationApplied {
		t.Fatalf("coherent publication outcome = %#v", first)
	}
	assertSharedCarrierFields(
		t,
		carrierPath,
		"dark",
	)
	stored, err := store.Read()
	if err != nil {
		t.Fatalf("read coherent manifest: %v", err)
	}
	if stored.Kind() != ManifestReadPresent ||
		stored.Manifest().Schema() != "haft.host-installation-manifest/v2" ||
		len(stored.Manifest().ManagedFragments()) != 1 {
		t.Fatalf("stored coherent manifest = %#v", stored)
	}
	for _, path := range stored.Manifest().RenderedPaths() {
		if path.Path == carrierPath {
			t.Fatalf(
				"manifest claimed shared carrier as a whole path: %+v",
				path,
			)
		}
	}

	operatorBytes := []byte(
		`{"theme":"light","mcpServers":{"haft":{"args":["serve"],"command":"/usr/local/bin/haft"}}}`,
	)
	if err := os.WriteFile(carrierPath, operatorBytes, 0o640); err != nil {
		t.Fatalf("change user-owned field: %v", err)
	}
	currentBatch := mustInstalledCoherentBatch(
		t,
		store,
		projection,
	)
	carrierStep := stepsByPath(currentBatch.Steps())[carrierPath]
	if carrierStep.Kind() != initplanning.PublicationPreserve ||
		!carrierStep.IsManagedCarrier() {
		t.Fatalf("current managed carrier step = %+v", carrierStep)
	}
	repeated, err := mustHostPublisher(t).Publish(
		currentBatch,
		store,
	)
	if err != nil {
		t.Fatalf("publish current coherent host: %v", err)
	}
	if repeated.Kind() != HostPublicationAlreadyCurrent {
		t.Fatalf("current coherent outcome = %#v", repeated)
	}
	assertSharedCarrierFields(
		t,
		carrierPath,
		"light",
	)
}

func mustCoherentPublicationProjection(
	t *testing.T,
	root string,
) (initplanning.HostAdapterProjection, string) {
	t.Helper()
	carrierPath := filepath.Join(root, ".codex", "config.json")
	skillPath := filepath.Join(
		root,
		".codex",
		"skills",
		"h-reason",
		"SKILL.md",
	)
	skill := mustObservationOutput(
		t,
		skillPath,
		[]byte("canonical skill"),
	)
	fragment, err := initplanning.NewJSONObjectEntryFragment(
		carrierPath,
		initplanning.ComponentMCP,
		[]string{"mcpServers", "haft"},
		[]byte(
			`{"command":"/usr/local/bin/haft","args":["serve"]}`,
		),
		0o600,
		"json-merge-v1",
	)
	if err != nil {
		t.Fatalf("NewJSONObjectEntryFragment: %v", err)
	}
	publication, err := initplanning.NewPublicationIdentity(
		initplanning.PublicationIdentityInput{
			HaftVersion:         "v9.dev",
			ExecutablePath:      filepath.Join(root, "bin", "haft"),
			ExecutableDigest:    "sha256:" + strings.Repeat("a", 64),
			SkillBundleDigest:   "sha256:" + strings.Repeat("b", 64),
			KernelCatalogDigest: "sha256:" + strings.Repeat("c", 64),
		},
	)
	if err != nil {
		t.Fatalf("NewPublicationIdentity: %v", err)
	}
	components, err := initplanning.ParseComponentSet(
		[]string{
			string(initplanning.ComponentMCP),
			string(initplanning.ComponentSkills),
		},
	)
	if err != nil {
		t.Fatalf("ParseComponentSet: %v", err)
	}
	recovery, err := initplanning.NewRecoveryOperation(
		[]string{"haft", "init", "--codex", "--local"},
	)
	if err != nil {
		t.Fatalf("NewRecoveryOperation: %v", err)
	}
	builder := initplanning.NewHostAdapterProjectionBuilder(
		initplanning.HostCodex,
	)
	builder = builder.AtEdition("codex.coherent.v1")
	builder = builder.PublishedFrom(publication)
	builder = builder.ForProject(root, "qnt_e3149c17")
	builder = builder.WithSelection(
		initplanning.ScopeProject,
		components,
	)
	builder = builder.AddTargetRoot(root)
	builder = builder.AddOutput(skill)
	builder = builder.AddManagedFragment(fragment)
	builder = builder.RecoverWith(recovery)
	projection, err := builder.Build()
	if err != nil {
		t.Fatalf("build coherent projection: %v", err)
	}
	return projection, carrierPath
}

func mustFirstCoherentBatch(
	t *testing.T,
	projection initplanning.HostAdapterProjection,
) initplanning.HostPublicationBatch {
	t.Helper()
	observer := mustHostObserver(t)
	wholePlan, err := initplanning.BuildFirstInstallationObservationPlan(
		projection,
		initplanning.WithoutKnownLegacyRegistry(),
	)
	if err != nil {
		t.Fatalf("BuildFirstInstallationObservationPlan: %v", err)
	}
	whole, err := observer.Observe(wholePlan)
	if err != nil {
		t.Fatalf("observe whole coherent paths: %v", err)
	}
	fragmentPlan, err := initplanning.BuildManagedFragmentObservationPlan(
		projection.ManagedFragments(),
		initplanning.NoPriorManagedFragmentBaseline(),
		initplanning.NoManagedFragmentLegacyRegistry(),
	)
	if err != nil {
		t.Fatalf("BuildManagedFragmentObservationPlan: %v", err)
	}
	carrier, err := observer.ObserveManagedCarrier(
		fragmentPlan,
		projection.TargetRoots(),
	)
	if err != nil {
		t.Fatalf("observe coherent managed carrier: %v", err)
	}
	currentness, err :=
		initplanning.ClassifyFirstCoherentInstallationCurrentness(
			projection,
			whole,
			[]initplanning.ManagedCarrierInput{carrier},
			initplanning.WithoutKnownLegacyRegistry(),
			initplanning.NoManagedFragmentLegacyRegistry(),
		)
	if err != nil {
		t.Fatalf("classify first coherent installation: %v", err)
	}
	plan, err := initplanning.CompileCoherentHostAdapterReconciliation(
		currentness,
	)
	if err != nil {
		t.Fatalf("compile first coherent installation: %v", err)
	}
	batch, err := initplanning.BuildHostPublicationBatch(plan)
	if err != nil {
		t.Fatalf("build first coherent batch: %v", err)
	}
	return batch
}

func mustInstalledCoherentBatch(
	t *testing.T,
	store ManifestStore,
	projection initplanning.HostAdapterProjection,
) initplanning.HostPublicationBatch {
	t.Helper()
	read, err := store.Read()
	if err != nil || read.Kind() != ManifestReadPresent {
		t.Fatalf("read installed coherent manifest: %#v / %v", read, err)
	}
	manifest := read.Manifest()
	observer := mustHostObserver(t)
	wholePlan, err := initplanning.BuildInstalledObservationPlan(
		manifest,
		projection,
		initplanning.WithoutKnownLegacyRegistry(),
	)
	if err != nil {
		t.Fatalf("BuildInstalledObservationPlan: %v", err)
	}
	whole, err := observer.Observe(wholePlan)
	if err != nil {
		t.Fatalf("observe installed coherent paths: %v", err)
	}
	baseline, err := manifest.ManagedFragmentBaseline()
	if err != nil {
		t.Fatalf("ManagedFragmentBaseline: %v", err)
	}
	fragmentPlan, err := initplanning.BuildManagedFragmentObservationPlan(
		projection.ManagedFragments(),
		baseline,
		initplanning.NoManagedFragmentLegacyRegistry(),
	)
	if err != nil {
		t.Fatalf("BuildManagedFragmentObservationPlan(installed): %v", err)
	}
	carrier, err := observer.ObserveManagedCarrier(
		fragmentPlan,
		projection.TargetRoots(),
	)
	if err != nil {
		t.Fatalf("observe installed coherent carrier: %v", err)
	}
	currentness, err :=
		initplanning.ClassifyCoherentInstallationCurrentness(
			manifest,
			projection,
			whole,
			[]initplanning.ManagedCarrierInput{carrier},
			initplanning.WithoutKnownLegacyRegistry(),
			initplanning.NoManagedFragmentLegacyRegistry(),
		)
	if err != nil {
		t.Fatalf("classify installed coherent installation: %v", err)
	}
	plan, err := initplanning.CompileCoherentHostAdapterReconciliation(
		currentness,
	)
	if err != nil {
		t.Fatalf("compile installed coherent installation: %v", err)
	}
	batch, err := initplanning.BuildHostPublicationBatch(plan)
	if err != nil {
		t.Fatalf("build installed coherent batch: %v", err)
	}
	return batch
}

func mustHostObserver(t *testing.T) FileObserver {
	t.Helper()
	observer, err := NewFileObserver(1 << 20)
	if err != nil {
		t.Fatalf("NewFileObserver: %v", err)
	}
	return observer
}

func assertSharedCarrierFields(
	t *testing.T,
	path string,
	theme string,
) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read shared carrier: %v", err)
	}
	var value map[string]any
	if err := json.Unmarshal(content, &value); err != nil {
		t.Fatalf("decode shared carrier: %v", err)
	}
	if value["theme"] != theme {
		t.Fatalf("shared carrier theme = %#v, want %q", value["theme"], theme)
	}
	servers, ok := value["mcpServers"].(map[string]any)
	if !ok {
		t.Fatalf("shared carrier mcpServers = %#v", value["mcpServers"])
	}
	if _, ok := servers["haft"].(map[string]any); !ok {
		t.Fatalf("shared carrier Haft entry = %#v", servers["haft"])
	}
}

func TestHostPublisherFreshInstallAndRepeatedInitAreExactAndIdempotent(
	t *testing.T,
) {
	root := t.TempDir()
	content := map[string][]byte{
		"skills/h-reason/SKILL.md": []byte("reason"),
		"skills/h-status/SKILL.md": []byte("status"),
	}
	projection := mustPublicationProjection(t, root, content)
	batch := mustFreshBatchForProjection(t, projection)
	store := mustManifestStore(
		t,
		root,
		filepath.Join(root, ".haft", "host-installations", "codex.project.json"),
	)
	publisher := mustHostPublisher(t)

	first, err := publisher.Publish(batch, store)
	if err != nil {
		t.Fatalf("publish fresh host: %v", err)
	}
	if first.Kind() != HostPublicationApplied ||
		first.PartialEffectBoundary() != "" ||
		len(first.Receipts()) != len(content) ||
		len(first.PendingPaths()) != 0 {
		t.Fatalf("fresh publication outcome = %#v", first)
	}
	assertPublishedContent(t, root, content)
	assertStoredManifest(t, store, batch.Manifest())
	assertPublicationJournalMissing(t, store)
	assertNoHostPublicationStages(t, root)

	currentBatch := mustInstalledBatchForProjection(t, store, projection)
	second, err := publisher.Publish(currentBatch, store)
	if err != nil {
		t.Fatalf("publish current host: %v", err)
	}
	if second.Kind() != HostPublicationAlreadyCurrent ||
		len(second.Receipts()) != len(content) ||
		len(second.PendingPaths()) != 0 {
		t.Fatalf("idempotent publication outcome = %#v", second)
	}
	assertPublishedContent(t, root, content)
	assertStoredManifest(t, store, currentBatch.Manifest())
	assertPublicationJournalMissing(t, store)
	assertNoHostPublicationStages(t, root)
}

func TestHostPublisherPreservesDesiredBytesWithoutOwnershipWitness(
	t *testing.T,
) {
	root := t.TempDir()
	content := map[string][]byte{
		"skills/h-reason/SKILL.md": []byte("desired but unowned"),
	}
	projection := mustPublicationProjection(t, root, content)
	batch := mustFreshBatchForProjection(t, projection)
	path := filepath.Join(root, "skills", "h-reason", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create foreign parent: %v", err)
	}
	if err := os.WriteFile(path, content["skills/h-reason/SKILL.md"], 0o644); err != nil {
		t.Fatalf("write foreign desired bytes: %v", err)
	}
	store := mustManifestStore(
		t,
		root,
		filepath.Join(root, ".haft", "host-installations", "codex.project.json"),
	)
	outcome, err := mustHostPublisher(t).Publish(batch, store)
	if err != nil {
		t.Fatalf("publish over desired foreign bytes: %v", err)
	}
	if outcome.Kind() != HostPublicationPreconditionChanged ||
		len(outcome.PreconditionChanges()) != 1 {
		t.Fatalf("foreign desired-byte outcome = %#v", outcome)
	}
	assertFileBytes(t, path, content["skills/h-reason/SKILL.md"])
	assertManifestMissing(t, store)
	assertPublicationJournalMissing(t, store)
}

func TestHostPublisherRejectsSymlinkedHostParentWithoutOutsideEffect(
	t *testing.T,
) {
	root := t.TempDir()
	content := map[string][]byte{
		"skills/h-reason/SKILL.md": []byte("reason"),
	}
	projection := mustPublicationProjection(t, root, content)
	batch := mustFreshBatchForProjection(t, projection)
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "skills")); err != nil {
		t.Fatalf("create host-parent symlink: %v", err)
	}
	store := mustManifestStore(
		t,
		root,
		filepath.Join(root, ".haft", "host-installations", "codex.project.json"),
	)
	outcome, err := mustHostPublisher(t).Publish(batch, store)
	if err != nil {
		t.Fatalf("inspect symlinked host parent: %v", err)
	}
	failure, exists := outcome.Failure()
	if outcome.Kind() != HostPublicationIncomplete ||
		!exists ||
		failure.Kind() != HostPublicationPreflightFailure {
		t.Fatalf("symlinked-parent outcome = %#v", outcome)
	}
	outsideTarget := filepath.Join(outside, "h-reason", "SKILL.md")
	if _, err := os.Lstat(outsideTarget); !os.IsNotExist(err) {
		t.Fatalf("outside target exists after rejected publication: %v", err)
	}
	assertManifestMissing(t, store)
	assertPublicationJournalMissing(t, store)
}

func TestHostPublisherCrashRecoveryNeverReclassifiesPartialBytesAsForeign(
	t *testing.T,
) {
	root := t.TempDir()
	content := map[string][]byte{
		"skills/h-reason/SKILL.md": []byte("reason"),
		"skills/h-status/SKILL.md": []byte("status"),
	}
	batch := mustFreshPublicationBatch(t, root, content)
	store := mustManifestStore(
		t,
		root,
		filepath.Join(root, ".haft", "host-installations", "codex.project.json"),
	)
	publisher := mustHostPublisher(t)
	pathCalls := 0
	publisher.hooks.beforePath = func(initplanning.HostPublicationStep) error {
		pathCalls++
		if pathCalls == 2 {
			return errors.New("injected process interruption")
		}
		return nil
	}
	first, err := publisher.Publish(batch, store)
	if err != nil {
		t.Fatalf("publish interrupted host: %v", err)
	}
	if first.Kind() != HostPublicationIncomplete ||
		first.PartialEffectBoundary() != "core_applied_host_incomplete" ||
		len(first.Receipts()) != 1 ||
		len(first.PendingPaths()) != 1 ||
		first.JournalDigest() == "" {
		t.Fatalf("partial publication outcome = %#v", first)
	}
	mutations := publicationMutationPaths(batch)
	assertFileBytes(t, mutations[0], batchOutputForPath(t, batch, mutations[0]).Content())
	if _, err := os.Lstat(mutations[1]); !os.IsNotExist(err) {
		t.Fatalf("second path exists after interruption: %v", err)
	}
	assertManifestMissing(t, store)
	journal := readPublicationJournal(t, store)
	if journal.Phase() != PublicationJournalApplying ||
		journal.ActivePath() != mutations[1] ||
		len(journal.CompletedPaths()) != 1 {
		t.Fatalf("partial journal = %#v", journal)
	}

	recovered, err := mustHostPublisher(t).Publish(batch, store)
	if err != nil {
		t.Fatalf("recover partial publication: %v", err)
	}
	if recovered.Kind() != HostPublicationApplied ||
		len(recovered.Receipts()) != 2 ||
		len(recovered.PendingPaths()) != 0 {
		t.Fatalf("recovered publication outcome = %#v", recovered)
	}
	receipts := receiptsByPath(recovered.Receipts())
	if !receipts[mutations[0]].Recovered() {
		t.Fatal("previously completed path was not reported as recovered")
	}
	assertPublishedContent(t, root, content)
	assertStoredManifest(t, store, batch.Manifest())
	assertPublicationJournalMissing(t, store)
	assertNoHostPublicationStages(t, root)
}

func TestHostPublisherRecoversManifestWindowWithoutRepublishingPaths(
	t *testing.T,
) {
	root := t.TempDir()
	content := map[string][]byte{
		"skills/h-reason/SKILL.md": []byte("reason"),
		"skills/h-status/SKILL.md": []byte("status"),
	}
	batch := mustFreshPublicationBatch(t, root, content)
	store := mustManifestStore(
		t,
		root,
		filepath.Join(root, ".haft", "host-installations", "codex.project.json"),
	)
	publisher := mustHostPublisher(t)
	publisher.hooks.beforeManifest = func() error {
		return errors.New("injected interruption before manifest")
	}
	first, err := publisher.Publish(batch, store)
	if err != nil {
		t.Fatalf("interrupt manifest publication: %v", err)
	}
	if first.Kind() != HostPublicationIncomplete ||
		len(first.Receipts()) != len(content) {
		t.Fatalf("manifest-window outcome = %#v", first)
	}
	assertPublishedContent(t, root, content)
	assertManifestMissing(t, store)
	journal := readPublicationJournal(t, store)
	if journal.Phase() != PublicationJournalManifest {
		t.Fatalf("journal phase = %s, want manifest", journal.Phase())
	}

	recovered, err := mustHostPublisher(t).Publish(batch, store)
	if err != nil {
		t.Fatalf("recover manifest window: %v", err)
	}
	if recovered.Kind() != HostPublicationApplied {
		t.Fatalf("manifest recovery outcome = %#v", recovered)
	}
	for _, receipt := range recovered.Receipts() {
		if !receipt.Recovered() {
			t.Fatalf("manifest recovery republished path: %#v", receipt)
		}
	}
	assertStoredManifest(t, store, batch.Manifest())
	assertPublicationJournalMissing(t, store)
	assertNoHostPublicationStages(t, root)
}

func TestHostPublisherRecoversPathLinearizationBeforeJournalAdvance(
	t *testing.T,
) {
	root := t.TempDir()
	content := map[string][]byte{
		"skills/h-reason/SKILL.md": []byte("reason"),
	}
	batch := mustFreshPublicationBatch(t, root, content)
	store := mustManifestStore(
		t,
		root,
		filepath.Join(root, ".haft", "host-installations", "codex.project.json"),
	)
	publisher := mustHostPublisher(t)
	publisher.hooks.afterPath = func(initplanning.HostPublicationStep) error {
		return errors.New("injected crash after path linearization")
	}
	first, err := publisher.Publish(batch, store)
	if err != nil {
		t.Fatalf("interrupt after path linearization: %v", err)
	}
	if first.Kind() != HostPublicationIncomplete ||
		len(first.Receipts()) != 0 {
		t.Fatalf("path-linearization outcome = %#v", first)
	}
	path := publicationMutationPaths(batch)[0]
	assertFileBytes(t, path, batchOutputForPath(t, batch, path).Content())
	journal := readPublicationJournal(t, store)
	if journal.Phase() != PublicationJournalApplying ||
		journal.ActivePath() != path ||
		len(journal.CompletedPaths()) != 0 {
		t.Fatalf("path-linearization journal = %#v", journal)
	}

	recovered, err := mustHostPublisher(t).Publish(batch, store)
	if err != nil {
		t.Fatalf("recover path linearization: %v", err)
	}
	if recovered.Kind() != HostPublicationApplied ||
		len(recovered.Receipts()) != 1 ||
		!recovered.Receipts()[0].Recovered() {
		t.Fatalf("path-linearization recovery = %#v", recovered)
	}
	assertStoredManifest(t, store, batch.Manifest())
	assertPublicationJournalMissing(t, store)
	assertNoHostPublicationStages(t, root)
}

func TestHostPublisherRecoversManifestCommitBeforeJournalCleanup(
	t *testing.T,
) {
	root := t.TempDir()
	content := map[string][]byte{
		"skills/h-reason/SKILL.md": []byte("reason"),
	}
	batch := mustFreshPublicationBatch(t, root, content)
	store := mustManifestStore(
		t,
		root,
		filepath.Join(root, ".haft", "host-installations", "codex.project.json"),
	)
	publisher := mustHostPublisher(t)
	publisher.hooks.afterManifest = func() error {
		return errors.New("injected crash after manifest commit")
	}
	first, err := publisher.Publish(batch, store)
	if err != nil {
		t.Fatalf("interrupt after manifest commit: %v", err)
	}
	if first.Kind() != HostPublicationIncomplete {
		t.Fatalf("manifest-commit outcome = %#v", first)
	}
	assertStoredManifest(t, store, batch.Manifest())
	if readPublicationJournal(t, store).Phase() != PublicationJournalManifest {
		t.Fatal("manifest-commit journal is not in manifest phase")
	}

	recovered, err := mustHostPublisher(t).Publish(batch, store)
	if err != nil {
		t.Fatalf("recover manifest commit: %v", err)
	}
	if recovered.Kind() != HostPublicationApplied ||
		len(recovered.Receipts()) != 1 ||
		!recovered.Receipts()[0].Recovered() {
		t.Fatalf("manifest-commit recovery = %#v", recovered)
	}
	assertStoredManifest(t, store, batch.Manifest())
	assertPublicationJournalMissing(t, store)
	assertNoHostPublicationStages(t, root)
}

func TestHostPublisherRecoveryPreservesPostPartialLocalModification(
	t *testing.T,
) {
	root := t.TempDir()
	content := map[string][]byte{
		"skills/h-reason/SKILL.md": []byte("reason"),
		"skills/h-status/SKILL.md": []byte("status"),
	}
	batch := mustFreshPublicationBatch(t, root, content)
	store := mustManifestStore(
		t,
		root,
		filepath.Join(root, ".haft", "host-installations", "codex.project.json"),
	)
	publisher := mustHostPublisher(t)
	pathCalls := 0
	publisher.hooks.beforePath = func(initplanning.HostPublicationStep) error {
		pathCalls++
		if pathCalls == 2 {
			return errors.New("interrupt after one path")
		}
		return nil
	}
	first, err := publisher.Publish(batch, store)
	if err != nil {
		t.Fatalf("create partial publication: %v", err)
	}
	if first.Kind() != HostPublicationIncomplete {
		t.Fatalf("partial publication kind = %s", first.Kind())
	}
	completed := first.Receipts()[0].Path()
	if err := os.WriteFile(completed, []byte("local edit"), 0o644); err != nil {
		t.Fatalf("modify completed path: %v", err)
	}

	recovery, err := mustHostPublisher(t).Publish(batch, store)
	if err != nil {
		t.Fatalf("inspect conflicted recovery: %v", err)
	}
	failure, exists := recovery.Failure()
	if recovery.Kind() != HostPublicationIncomplete ||
		!exists ||
		failure.Kind() != HostPublicationRecoveryConflict {
		t.Fatalf("conflicted recovery outcome = %#v", recovery)
	}
	assertFileBytes(t, completed, []byte("local edit"))
	assertManifestMissing(t, store)
	if readPublicationJournal(t, store).Digest() == "" {
		t.Fatal("recovery journal disappeared after a local modification")
	}
}

func TestHostPublisherUpdatesOwnedCarrierAndRemovesExactOrphan(t *testing.T) {
	root := t.TempDir()
	firstContent := map[string][]byte{
		"skills/h-reason/SKILL.md": []byte("old reason"),
		"skills/h-status/SKILL.md": []byte("old status"),
	}
	firstProjection := mustPublicationProjection(t, root, firstContent)
	firstBatch := mustFreshBatchForProjection(t, firstProjection)
	store := mustManifestStore(
		t,
		root,
		filepath.Join(root, ".haft", "host-installations", "codex.project.json"),
	)
	if outcome, err := mustHostPublisher(t).Publish(firstBatch, store); err != nil ||
		outcome.Kind() != HostPublicationApplied {
		t.Fatalf("publish predecessor = %#v err=%v", outcome, err)
	}

	nextContent := map[string][]byte{
		"skills/h-reason/SKILL.md": []byte("new reason"),
	}
	nextProjection := mustPublicationProjection(t, root, nextContent)
	nextBatch := mustInstalledBatchForProjection(t, store, nextProjection)
	steps := stepsByPath(nextBatch.Steps())
	reasonPath := filepath.Join(root, "skills", "h-reason", "SKILL.md")
	statusPath := filepath.Join(root, "skills", "h-status", "SKILL.md")
	if steps[reasonPath].Kind() != initplanning.PublicationReplace ||
		steps[statusPath].Kind() != initplanning.PublicationRemove {
		t.Fatalf("update/removal steps = %#v", steps)
	}
	outcome, err := mustHostPublisher(t).Publish(nextBatch, store)
	if err != nil {
		t.Fatalf("publish update and orphan removal: %v", err)
	}
	if outcome.Kind() != HostPublicationApplied {
		t.Fatalf("update/removal outcome = %#v", outcome)
	}
	assertFileBytes(t, reasonPath, []byte("new reason"))
	if _, err := os.Lstat(statusPath); !os.IsNotExist(err) {
		t.Fatalf("owned orphan remains: %v", err)
	}
	assertStoredManifest(t, store, nextBatch.Manifest())
	assertPublicationJournalMissing(t, store)
}

func TestHostPublisherConcurrentSameBindingReturnsBusyWithoutInterleaving(
	t *testing.T,
) {
	root := t.TempDir()
	batch := mustFreshPublicationBatch(t, root, map[string][]byte{
		"skills/h-reason/SKILL.md": []byte("reason"),
	})
	store := mustManifestStore(
		t,
		root,
		filepath.Join(root, ".haft", "host-installations", "codex.project.json"),
	)
	firstPublisher := mustHostPublisher(t)
	entered := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	firstPublisher.hooks.beforePath = func(initplanning.HostPublicationStep) error {
		once.Do(func() {
			close(entered)
		})
		<-release
		return nil
	}
	type publishResult struct {
		outcome HostPublicationOutcome
		err     error
	}
	done := make(chan publishResult, 1)
	go func() {
		outcome, err := firstPublisher.Publish(batch, store)
		done <- publishResult{outcome: outcome, err: err}
	}()
	<-entered
	second, err := mustHostPublisher(t).Publish(batch, store)
	if err != nil {
		t.Fatalf("contended publication: %v", err)
	}
	if second.Kind() != HostPublicationBusy ||
		len(second.PendingPaths()) != 1 {
		t.Fatalf("contended outcome = %#v", second)
	}
	close(release)
	first := <-done
	if first.err != nil || first.outcome.Kind() != HostPublicationApplied {
		t.Fatalf("first publication = %#v err=%v", first.outcome, first.err)
	}
	assertStoredManifest(t, store, batch.Manifest())
	assertPublicationJournalMissing(t, store)
}

func TestHostPublisherRejectsAnotherBatchWhileRecoveryJournalIsLive(
	t *testing.T,
) {
	root := t.TempDir()
	firstBatch := mustFreshPublicationBatch(t, root, map[string][]byte{
		"skills/h-reason/SKILL.md": []byte("first"),
	})
	store := mustManifestStore(
		t,
		root,
		filepath.Join(root, ".haft", "host-installations", "codex.project.json"),
	)
	publisher := mustHostPublisher(t)
	publisher.hooks.beforePath = func(initplanning.HostPublicationStep) error {
		return errors.New("leave prepared recovery")
	}
	first, err := publisher.Publish(firstBatch, store)
	if err != nil || first.Kind() != HostPublicationIncomplete {
		t.Fatalf("create recovery journal = %#v err=%v", first, err)
	}
	secondBatch := mustFreshPublicationBatch(t, root, map[string][]byte{
		"skills/h-reason/SKILL.md": []byte("second"),
	})
	second, err := mustHostPublisher(t).Publish(secondBatch, store)
	if err != nil {
		t.Fatalf("publish another batch: %v", err)
	}
	failure, exists := second.Failure()
	if second.Kind() != HostPublicationIncomplete ||
		!exists ||
		failure.Kind() != HostPublicationRecoveryConflict {
		t.Fatalf("another-batch outcome = %#v", second)
	}
	assertManifestMissing(t, store)
	if readPublicationJournal(t, store).BatchDigest() != firstBatch.Digest() {
		t.Fatal("another batch replaced the live recovery journal")
	}
}

func mustHostPublisher(t *testing.T) HostPublisher {
	t.Helper()
	publisher, err := NewHostPublisher(1 << 20)
	if err != nil {
		t.Fatalf("new host publisher: %v", err)
	}
	return publisher
}

func mustPublicationProjection(
	t *testing.T,
	root string,
	relativeContent map[string][]byte,
) initplanning.HostAdapterProjection {
	t.Helper()
	relativePaths := make([]string, 0, len(relativeContent))
	for relative := range relativeContent {
		relativePaths = append(relativePaths, relative)
	}
	sort.Strings(relativePaths)
	outputs := make([]initplanning.RenderedOutput, 0, len(relativePaths))
	for _, relative := range relativePaths {
		path := filepath.Join(root, filepath.FromSlash(relative))
		output := mustObservationOutput(t, path, relativeContent[relative])
		outputs = append(outputs, output)
	}
	return mustObservationProjection(t, root, outputs)
}

func mustFreshBatchForProjection(
	t *testing.T,
	projection initplanning.HostAdapterProjection,
) initplanning.HostPublicationBatch {
	t.Helper()
	observationPlan, err := initplanning.BuildFirstInstallationObservationPlan(
		projection,
		initplanning.WithoutKnownLegacyRegistry(),
	)
	if err != nil {
		t.Fatalf("build first observation plan: %v", err)
	}
	observer, err := NewFileObserver(1 << 20)
	if err != nil {
		t.Fatalf("new observer: %v", err)
	}
	observations, err := observer.Observe(observationPlan)
	if err != nil {
		t.Fatalf("observe first installation: %v", err)
	}
	currentness, err := initplanning.ClassifyFirstInstallationCurrentness(
		projection,
		observations,
		initplanning.WithoutKnownLegacyRegistry(),
	)
	if err != nil {
		t.Fatalf("classify first installation: %v", err)
	}
	plan, err := initplanning.CompileHostAdapterReconciliation(currentness)
	if err != nil {
		t.Fatalf("compile first reconciliation: %v", err)
	}
	batch, err := initplanning.BuildHostPublicationBatch(plan)
	if err != nil {
		t.Fatalf("build first publication batch: %v", err)
	}
	return batch
}

func mustInstalledBatchForProjection(
	t *testing.T,
	store ManifestStore,
	projection initplanning.HostAdapterProjection,
) initplanning.HostPublicationBatch {
	t.Helper()
	read, err := store.Read()
	if err != nil {
		t.Fatalf("read installed manifest: %v", err)
	}
	if read.Kind() != ManifestReadPresent {
		t.Fatal("installed manifest is missing")
	}
	manifest := read.Manifest()
	observationPlan, err := initplanning.BuildInstalledObservationPlan(
		manifest,
		projection,
		initplanning.WithoutKnownLegacyRegistry(),
	)
	if err != nil {
		t.Fatalf("build installed observation plan: %v", err)
	}
	observer, err := NewFileObserver(1 << 20)
	if err != nil {
		t.Fatalf("new installed observer: %v", err)
	}
	observations, err := observer.Observe(observationPlan)
	if err != nil {
		t.Fatalf("observe installed publication: %v", err)
	}
	currentness, err := initplanning.ClassifyInstallationCurrentness(
		manifest,
		projection,
		observations,
		initplanning.WithoutKnownLegacyRegistry(),
	)
	if err != nil {
		t.Fatalf("classify installed publication: %v", err)
	}
	plan, err := initplanning.CompileHostAdapterReconciliation(currentness)
	if err != nil {
		t.Fatalf("compile installed reconciliation: %v", err)
	}
	batch, err := initplanning.BuildHostPublicationBatch(plan)
	if err != nil {
		t.Fatalf("build installed publication batch: %v", err)
	}
	return batch
}

func assertPublishedContent(
	t *testing.T,
	root string,
	content map[string][]byte,
) {
	t.Helper()
	for relative, want := range content {
		path := filepath.Join(root, filepath.FromSlash(relative))
		assertFileBytes(t, path, want)
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat published path %s: %v", path, err)
		}
		if info.Mode().Perm() != 0o644 {
			t.Fatalf("published path %s mode = %o", path, info.Mode().Perm())
		}
	}
}

func assertFileBytes(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(got) != string(want) {
		t.Fatalf("path %s bytes = %q, want %q", path, got, want)
	}
}

func assertManifestMissing(t *testing.T, store ManifestStore) {
	t.Helper()
	read, err := store.Read()
	if err != nil {
		t.Fatalf("read missing manifest: %v", err)
	}
	if read.Kind() != ManifestReadMissing {
		t.Fatalf("manifest kind = %s, want missing", read.Kind())
	}
}

func assertPublicationJournalMissing(t *testing.T, store ManifestStore) {
	t.Helper()
	journalStore, err := newPublicationJournalStore(store)
	if err != nil {
		t.Fatalf("new publication journal store: %v", err)
	}
	read, err := journalStore.read()
	if err != nil {
		t.Fatalf("read publication journal: %v", err)
	}
	if read.kind != publicationJournalMissing {
		t.Fatalf("publication journal kind = %s, want missing", read.kind)
	}
}

func readPublicationJournal(
	t *testing.T,
	store ManifestStore,
) PublicationJournal {
	t.Helper()
	journalStore, err := newPublicationJournalStore(store)
	if err != nil {
		t.Fatalf("new publication journal store: %v", err)
	}
	read, err := journalStore.read()
	if err != nil {
		t.Fatalf("read publication journal: %v", err)
	}
	if read.kind != publicationJournalPresent {
		t.Fatalf("publication journal kind = %s, want present", read.kind)
	}
	return read.journal
}

func assertNoHostPublicationStages(t *testing.T, root string) {
	t.Helper()
	var stages []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if strings.HasPrefix(entry.Name(), ".haft-host-stage-") {
			stages = append(stages, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk publication stages: %v", err)
	}
	if len(stages) != 0 {
		t.Fatalf("publication stages remain: %v", stages)
	}
}

func batchOutputForPath(
	t *testing.T,
	batch initplanning.HostPublicationBatch,
	path string,
) initplanning.RenderedOutput {
	t.Helper()
	step, exists := stepsByPath(batch.Steps())[path]
	if !exists {
		t.Fatalf("publication step %s is absent", path)
	}
	output, exists := step.Output()
	if !exists {
		t.Fatalf("publication output %s is absent", path)
	}
	return output
}

func receiptsByPath(
	receipts []HostPathReceipt,
) map[string]HostPathReceipt {
	result := make(map[string]HostPathReceipt, len(receipts))
	for _, receipt := range receipts {
		result[receipt.Path()] = receipt
	}
	return result
}

func stepsByPath(
	steps []initplanning.HostPublicationStep,
) map[string]initplanning.HostPublicationStep {
	result := make(map[string]initplanning.HostPublicationStep, len(steps))
	for _, step := range steps {
		result[step.Path()] = step
	}
	return result
}
