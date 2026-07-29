package initfs

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/initplanning"
)

func TestFileObserverReadsExactManagedCarrierWithoutFollowingLinks(
	t *testing.T,
) {
	root := t.TempDir()
	path := filepath.Join(root, "shared", "config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create shared carrier parent: %v", err)
	}
	content := []byte(`{"theme":"dark"}`)
	if err := os.WriteFile(path, content, 0o640); err != nil {
		t.Fatalf("write shared carrier: %v", err)
	}
	fragment, err := initplanning.NewJSONObjectEntryFragment(
		path,
		initplanning.ComponentMCP,
		[]string{"mcpServers", "haft"},
		[]byte(`{"command":"haft"}`),
		0o600,
		"json-merge-v1",
	)
	if err != nil {
		t.Fatalf("NewJSONObjectEntryFragment: %v", err)
	}
	plan, err := initplanning.BuildManagedFragmentObservationPlan(
		[]initplanning.ManagedFragment{fragment},
		initplanning.NoPriorManagedFragmentBaseline(),
		initplanning.NoManagedFragmentLegacyRegistry(),
	)
	if err != nil {
		t.Fatalf("BuildManagedFragmentObservationPlan: %v", err)
	}
	observer, err := NewFileObserver(1024)
	if err != nil {
		t.Fatalf("NewFileObserver: %v", err)
	}
	input, err := observer.ObserveManagedCarrier(
		plan,
		[]string{root},
	)
	if err != nil {
		t.Fatalf("ObserveManagedCarrier: %v", err)
	}
	if input.Kind() != initplanning.ManagedCarrierPresent ||
		!bytes.Equal(input.Content(), content) ||
		input.Mode() != 0o640 {
		t.Fatalf("managed carrier input = %+v", input)
	}

	if err := os.Remove(path); err != nil {
		t.Fatalf("remove shared carrier: %v", err)
	}
	missing, err := observer.ObserveManagedCarrier(
		plan,
		[]string{root},
	)
	if err != nil {
		t.Fatalf("ObserveManagedCarrier(missing): %v", err)
	}
	if missing.Kind() != initplanning.ManagedCarrierMissing {
		t.Fatalf("missing managed carrier = %+v", missing)
	}

	if err := os.Symlink(filepath.Join(root, "operator.json"), path); err != nil {
		t.Fatalf("link shared carrier: %v", err)
	}
	if _, err := observer.ObserveManagedCarrier(
		plan,
		[]string{root},
	); err == nil {
		t.Fatal("managed carrier observer followed a symbolic link")
	}
}

func TestFileObserverProducesExactRequiredObservationsAndOmitsAbsentLegacy(
	t *testing.T,
) {
	root := t.TempDir()
	skillsRoot := filepath.Join(root, "skills")
	if err := os.MkdirAll(skillsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	existingPath := filepath.Join(skillsRoot, "existing.md")
	missingPath := filepath.Join(skillsRoot, "missing.md")
	optionalLegacyPath := filepath.Join(skillsRoot, "retired.md")
	existingBytes := []byte("desired bytes without an ownership receipt")
	if err := os.WriteFile(existingPath, existingBytes, 0o640); err != nil {
		t.Fatal(err)
	}
	projection := mustObservationProjection(t, root, []initplanning.RenderedOutput{
		mustObservationOutput(t, existingPath, existingBytes),
		mustObservationOutput(t, missingPath, []byte("new output")),
	})
	registry, err := initplanning.BuildKnownLegacyDigestRegistry(
		initplanning.KnownLegacyDigestRegistryInput{
			Edition:     "legacy.v1",
			ProjectRoot: root,
			ProjectID:   "qnt_e3149c17",
			Host:        initplanning.HostCodex,
			Scope:       initplanning.ScopeProject,
			TargetRoots: []string{root},
			Paths: []initplanning.KnownLegacyPath{{
				Path:      optionalLegacyPath,
				Component: initplanning.ComponentSkills,
				Digest:    digestForObserver([]byte("retired")),
			}},
		},
	)
	if err != nil {
		t.Fatalf("build legacy registry: %v", err)
	}
	legacySelection, err := initplanning.WithKnownLegacyRegistry(registry)
	if err != nil {
		t.Fatalf("select legacy registry: %v", err)
	}
	plan, err := initplanning.BuildFirstInstallationObservationPlan(
		projection,
		legacySelection,
	)
	if err != nil {
		t.Fatalf("build observation plan: %v", err)
	}
	observer, err := NewFileObserver(1024)
	if err != nil {
		t.Fatalf("new file observer: %v", err)
	}
	observations, err := observer.Observe(plan)
	if err != nil {
		t.Fatalf("observe files: %v", err)
	}
	if len(observations) != 2 {
		t.Fatalf("observations = %#v", observations)
	}
	if !slices.IsSortedFunc(observations, func(left, right initplanning.PathObservation) int {
		return strings.Compare(left.Path(), right.Path())
	}) {
		t.Fatalf("observations are not sorted: %#v", observations)
	}
	byPath := make(map[string]initplanning.PathObservation, len(observations))
	for _, observation := range observations {
		byPath[observation.Path()] = observation
	}
	existing := byPath[existingPath]
	if existing.Kind() != initplanning.PathObservedPresent ||
		existing.Digest() != digestForObserver(existingBytes) ||
		existing.Mode().Perm() != 0o640 {
		t.Fatalf("existing observation = %#v", existing)
	}
	if byPath[missingPath].Kind() != initplanning.PathObservedMissing {
		t.Fatalf("missing observation = %#v", byPath[missingPath])
	}
	if _, exists := byPath[optionalLegacyPath]; exists {
		t.Fatal("absent optional legacy path emitted an unrelated missing observation")
	}
	currentness, err := initplanning.ClassifyFirstInstallationCurrentness(
		projection,
		observations,
		legacySelection,
	)
	if err != nil {
		t.Fatalf("classify observed first install: %v", err)
	}
	if firstObservedState(t, currentness, existingPath) != initplanning.PathForeign {
		t.Fatal("desired-byte equality became ownership after filesystem observation")
	}
}

func TestFileObserverRejectsSymbolicLinksInsideManagedRoot(t *testing.T) {
	root := t.TempDir()
	skillsRoot := filepath.Join(root, "skills")
	if err := os.MkdirAll(skillsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	realPath := filepath.Join(root, "real.md")
	if err := os.WriteFile(realPath, []byte("real"), 0o644); err != nil {
		t.Fatal(err)
	}
	targetPath := filepath.Join(skillsRoot, "linked.md")
	if err := os.Symlink(realPath, targetPath); err != nil {
		t.Fatal(err)
	}
	projection := mustObservationProjection(t, root, []initplanning.RenderedOutput{
		mustObservationOutput(t, targetPath, []byte("desired")),
	})
	plan, err := initplanning.BuildFirstInstallationObservationPlan(
		projection,
		initplanning.WithoutKnownLegacyRegistry(),
	)
	if err != nil {
		t.Fatalf("build observation plan: %v", err)
	}
	observer, err := NewFileObserver(1024)
	if err != nil {
		t.Fatalf("new file observer: %v", err)
	}
	_, err = observer.Observe(plan)
	assertObservationFailure(t, err, ObservationUnsafePath, targetPath)
}

func TestFileObserverRejectsSymbolicManagedRoot(t *testing.T) {
	parent := t.TempDir()
	realRoot := filepath.Join(parent, "real")
	linkedRoot := filepath.Join(parent, "linked")
	if err := os.MkdirAll(realRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(realRoot, linkedRoot); err != nil {
		t.Fatal(err)
	}
	targetPath := filepath.Join(linkedRoot, "skill.md")
	projection := mustObservationProjection(t, linkedRoot, []initplanning.RenderedOutput{
		mustObservationOutput(t, targetPath, []byte("desired")),
	})
	plan, err := initplanning.BuildFirstInstallationObservationPlan(
		projection,
		initplanning.WithoutKnownLegacyRegistry(),
	)
	if err != nil {
		t.Fatalf("build observation plan: %v", err)
	}
	observer, err := NewFileObserver(1024)
	if err != nil {
		t.Fatalf("new file observer: %v", err)
	}
	_, err = observer.Observe(plan)
	assertObservationFailure(t, err, ObservationUnsafePath, linkedRoot)
}

func TestFileObserverRejectsOversizedAndInvalidRequests(t *testing.T) {
	if _, err := NewFileObserver(0); err == nil {
		t.Fatal("zero byte limit was accepted")
	}
	observer, err := NewFileObserver(4)
	if err != nil {
		t.Fatalf("new file observer: %v", err)
	}
	if _, err := observer.Observe(initplanning.InstallationObservationPlan{}); err == nil {
		t.Fatal("zero observation plan was accepted")
	}
	root := t.TempDir()
	path := filepath.Join(root, "large.md")
	if err := os.WriteFile(path, []byte("12345"), 0o644); err != nil {
		t.Fatal(err)
	}
	projection := mustObservationProjection(t, root, []initplanning.RenderedOutput{
		mustObservationOutput(t, path, []byte("desired")),
	})
	plan, err := initplanning.BuildFirstInstallationObservationPlan(
		projection,
		initplanning.WithoutKnownLegacyRegistry(),
	)
	if err != nil {
		t.Fatalf("build observation plan: %v", err)
	}
	_, err = observer.Observe(plan)
	assertObservationFailure(t, err, ObservationFileTooLarge, path)
}

func mustObservationProjection(
	t *testing.T,
	root string,
	outputs []initplanning.RenderedOutput,
) initplanning.HostAdapterProjection {
	t.Helper()
	components, err := initplanning.ParseComponentSet([]string{string(initplanning.ComponentSkills)})
	if err != nil {
		t.Fatalf("parse components: %v", err)
	}
	publication, err := initplanning.NewPublicationIdentity(
		initplanning.PublicationIdentityInput{
			HaftVersion:         "v9-test",
			ExecutablePath:      filepath.Join(root, "bin", "haft"),
			ExecutableDigest:    "sha256:" + strings.Repeat("a", 64),
			SkillBundleDigest:   "sha256:" + strings.Repeat("b", 64),
			KernelCatalogDigest: "sha256:" + strings.Repeat("c", 64),
		},
	)
	if err != nil {
		t.Fatalf("build publication identity: %v", err)
	}
	recovery, err := initplanning.NewRecoveryOperation([]string{"haft", "init", "--check"})
	if err != nil {
		t.Fatalf("build recovery operation: %v", err)
	}
	builder := initplanning.NewHostAdapterProjectionBuilder(initplanning.HostCodex)
	builder = builder.AtEdition("codex.skills.v1")
	builder = builder.PublishedFrom(publication)
	builder = builder.ForProject(root, "qnt_e3149c17")
	builder = builder.WithSelection(initplanning.ScopeProject, components)
	builder = builder.AddTargetRoot(root)
	for _, output := range outputs {
		builder = builder.AddOutput(output)
	}
	builder = builder.RecoverWith(recovery)
	projection, err := builder.Build()
	if err != nil {
		t.Fatalf("build host projection: %v", err)
	}
	return projection
}

func mustObservationOutput(
	t *testing.T,
	path string,
	content []byte,
) initplanning.RenderedOutput {
	t.Helper()
	output, err := initplanning.NewRenderedOutput(
		path,
		initplanning.ComponentSkills,
		content,
		0o644,
	)
	if err != nil {
		t.Fatalf("build rendered output: %v", err)
	}
	return output
}

func digestForObserver(content []byte) string {
	digest := sha256.Sum256(content)
	return fmt.Sprintf("sha256:%x", digest)
}

func firstObservedState(
	t *testing.T,
	currentness initplanning.InstallationCurrentness,
	path string,
) initplanning.PathCurrentnessKind {
	t.Helper()
	for _, current := range currentness.Paths() {
		if current.Path() == path {
			return current.Kind()
		}
	}
	t.Fatalf("path %s is absent from currentness", path)
	return ""
}

func assertObservationFailure(
	t *testing.T,
	err error,
	kind ObservationFailureKind,
	path string,
) {
	t.Helper()
	var failure ObservationFailure
	if !errors.As(err, &failure) {
		t.Fatalf("error = %v, want ObservationFailure", err)
	}
	if failure.Kind() != kind || failure.Path() != path {
		t.Fatalf("failure = %#v, want kind=%s path=%s", failure, kind, path)
	}
}
