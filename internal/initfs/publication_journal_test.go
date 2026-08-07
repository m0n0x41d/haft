package initfs

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/m0n0x41d/haft/internal/initplanning"
)

func TestPublicationJournalTypestateIsCanonicalAndBatchBound(t *testing.T) {
	root := t.TempDir()
	batch := mustFreshPublicationBatch(t, root, map[string][]byte{
		"skills/h-reason/SKILL.md": []byte("reason"),
		"skills/h-status/SKILL.md": []byte("status"),
	})
	manifestPath := filepath.Join(root, ".haft", "host-installations", "codex.project.json")
	journal, err := NewPublicationJournal(batch, manifestPath)
	if err != nil {
		t.Fatalf("new publication journal: %v", err)
	}
	if journal.Phase() != PublicationJournalPrepared ||
		journal.ActivePath() != "" ||
		len(journal.CompletedPaths()) != 0 ||
		journal.BatchDigest() != batch.Digest() ||
		journal.DesiredManifestDigest() != batch.Manifest().Digest() {
		t.Fatalf("initial journal = %#v", journal)
	}
	parsed, err := ParsePublicationJournal(journal.CanonicalBytes())
	if err != nil {
		t.Fatalf("parse canonical journal: %v", err)
	}
	if parsed.Digest() != journal.Digest() {
		t.Fatalf("parsed digest = %s, want %s", parsed.Digest(), journal.Digest())
	}

	mutations := publicationMutationPaths(batch)
	for _, path := range mutations {
		applying, err := BeginPublicationStep(journal, batch, path)
		if err != nil {
			t.Fatalf("begin %s: %v", path, err)
		}
		if applying.Phase() != PublicationJournalApplying ||
			applying.ActivePath() != path {
			t.Fatalf("applying journal = %#v", applying)
		}
		if _, err := BeginPublicationStep(applying, batch, path); err == nil {
			t.Fatal("second active publication step was accepted")
		}
		journal, err = CompletePublicationStep(applying, batch, path)
		if err != nil {
			t.Fatalf("complete %s: %v", path, err)
		}
	}
	manifestPhase, err := BeginManifestPublication(journal, batch)
	if err != nil {
		t.Fatalf("begin manifest publication: %v", err)
	}
	if manifestPhase.Phase() != PublicationJournalManifest ||
		len(manifestPhase.CompletedPaths()) != len(mutations) {
		t.Fatalf("manifest journal = %#v", manifestPhase)
	}

	other := mustFreshPublicationBatch(t, root, map[string][]byte{
		"skills/h-reason/SKILL.md": []byte("changed"),
	})
	if err := journal.ValidateAgainst(other, manifestPath); err == nil {
		t.Fatal("journal accepted another exact publication batch")
	}
}

func TestPublicationJournalParserRejectsNonCanonicalOrOpenInput(t *testing.T) {
	root := t.TempDir()
	batch := mustFreshPublicationBatch(t, root, map[string][]byte{
		"skills/h-reason/SKILL.md": []byte("reason"),
	})
	path := filepath.Join(root, "manifest.json")
	journal, err := NewPublicationJournal(batch, path)
	if err != nil {
		t.Fatalf("new publication journal: %v", err)
	}
	for name, raw := range map[string][]byte{
		"whitespace": append([]byte(" "), journal.CanonicalBytes()...),
		"trailing":   append(journal.CanonicalBytes(), []byte("{}")...),
		"unknown": bytes.Replace(
			journal.CanonicalBytes(),
			[]byte(`"schema":`),
			[]byte(`"unknown":true,"schema":`),
			1,
		),
		"null completed paths": bytes.Replace(
			journal.CanonicalBytes(),
			[]byte(`"completed_paths":[]`),
			[]byte(`"completed_paths":null`),
			1,
		),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ParsePublicationJournal(raw); err == nil {
				t.Fatal("invalid publication journal was accepted")
			}
		})
	}
}

func TestPublicationJournalStoreUsesExactCreateReplaceRemoveCAS(t *testing.T) {
	root := t.TempDir()
	batch := mustFreshPublicationBatch(t, root, map[string][]byte{
		"skills/h-reason/SKILL.md": []byte("reason"),
	})
	manifestPath := filepath.Join(root, ".haft", "host-installations", "codex.project.json")
	manifestStore := mustManifestStore(t, root, manifestPath)
	lease, acquired, err := manifestStore.TryAcquire()
	if err != nil {
		t.Fatalf("acquire manifest lease: %v", err)
	}
	if !acquired {
		t.Fatal("manifest lease was not acquired")
	}
	defer lease.Release()
	store, err := newPublicationJournalStore(manifestStore)
	if err != nil {
		t.Fatalf("new publication journal store: %v", err)
	}
	initial, err := NewPublicationJournal(batch, manifestPath)
	if err != nil {
		t.Fatalf("new initial journal: %v", err)
	}
	if err := store.create(initial); err != nil {
		t.Fatalf("create publication journal: %v", err)
	}
	if err := store.create(initial); err != nil {
		t.Fatalf("idempotent journal create: %v", err)
	}
	path := publicationMutationPaths(batch)[0]
	applying, err := BeginPublicationStep(initial, batch, path)
	if err != nil {
		t.Fatalf("begin publication step: %v", err)
	}
	if err := store.replace(applying, "sha256:"+string(bytes.Repeat([]byte("f"), 64))); err == nil {
		t.Fatal("journal replacement accepted a stale digest")
	}
	current, err := store.read()
	if err != nil {
		t.Fatalf("read preserved journal: %v", err)
	}
	if current.journal.Digest() != initial.Digest() {
		t.Fatal("stale replacement changed the journal")
	}
	if err := store.replace(applying, initial.Digest()); err != nil {
		t.Fatalf("replace publication journal: %v", err)
	}
	if err := store.remove(initial.Digest()); err == nil {
		t.Fatal("journal removal accepted a stale digest")
	}
	if err := store.remove(applying.Digest()); err != nil {
		t.Fatalf("remove publication journal: %v", err)
	}
	removed, err := store.read()
	if err != nil {
		t.Fatalf("read removed journal: %v", err)
	}
	if removed.kind != publicationJournalMissing {
		t.Fatalf("journal read kind = %s, want missing", removed.kind)
	}
	stages, err := filepath.Glob(
		filepath.Join(
			filepath.Dir(store.path),
			"."+filepath.Base(store.path)+".stage-*",
		),
	)
	if err != nil {
		t.Fatalf("glob journal stages: %v", err)
	}
	if len(stages) != 0 {
		t.Fatalf("journal stages remain: %v", stages)
	}
	if _, err := os.Lstat(store.path); !os.IsNotExist(err) {
		t.Fatalf("journal carrier remains: %v", err)
	}
}

func mustFreshPublicationBatch(
	t *testing.T,
	root string,
	relativeContent map[string][]byte,
) initplanning.HostPublicationBatch {
	t.Helper()
	relativePaths := make([]string, 0, len(relativeContent))
	for relative := range relativeContent {
		relativePaths = append(relativePaths, relative)
	}
	sort.Strings(relativePaths)
	outputs := make([]initplanning.RenderedOutput, 0, len(relativePaths))
	observations := make([]initplanning.PathObservation, 0, len(relativePaths))
	for _, relative := range relativePaths {
		path := filepath.Join(root, filepath.FromSlash(relative))
		output := mustObservationOutput(t, path, relativeContent[relative])
		outputs = append(outputs, output)
		observation, err := initplanning.ObserveMissingPath(
			path,
			initplanning.ComponentSkills,
		)
		if err != nil {
			t.Fatalf("build missing observation: %v", err)
		}
		observations = append(observations, observation)
	}
	projection := mustObservationProjection(t, root, outputs)
	currentness, err := initplanning.ClassifyFirstInstallationCurrentness(
		projection,
		observations,
		initplanning.WithoutKnownLegacyRegistry(),
	)
	if err != nil {
		t.Fatalf("classify fresh currentness: %v", err)
	}
	plan, err := initplanning.CompileHostAdapterReconciliation(currentness)
	if err != nil {
		t.Fatalf("compile fresh reconciliation: %v", err)
	}
	batch, err := initplanning.BuildHostPublicationBatch(plan)
	if err != nil {
		t.Fatalf("build fresh publication batch: %v", err)
	}
	return batch
}

func publicationMutationPaths(
	batch initplanning.HostPublicationBatch,
) []string {
	paths := make([]string, 0)
	for _, step := range batch.Steps() {
		switch step.Kind() {
		case initplanning.PublicationCreate,
			initplanning.PublicationReplace,
			initplanning.PublicationRemove:
			paths = append(paths, step.Path())
		}
	}
	sort.Strings(paths)
	return paths
}
