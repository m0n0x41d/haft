package cli

import (
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/m0n0x41d/haft/internal/initplanning"
	haftpi "github.com/m0n0x41d/haft/packages/haft-pi"
)

func TestCurrentPiCondensedParityDeclaresExactSourceAndLossBoundary(
	t *testing.T,
) {
	bundle, err := currentSkillSourceBundle()
	if err != nil {
		t.Fatalf("currentSkillSourceBundle: %v", err)
	}
	packageRoot := filepath.Join(t.TempDir(), "haft-pi")
	outputs, err := currentPiPackageOutputs(packageRoot)
	if err != nil {
		t.Fatalf("currentPiPackageOutputs: %v", err)
	}
	report, err := currentPiCondensedParity(
		bundle,
		packageRoot,
		outputs,
	)
	if err != nil {
		t.Fatalf("currentPiCondensedParity: %v", err)
	}
	t.Logf(
		"bundle=%s kernel=%s package=%s declaration=%s skills=%d tools=%d",
		report.BundleDigest,
		report.KernelCatalogDigest,
		report.PackageDigest,
		report.Declaration.Digest(),
		len(report.Skills),
		len(report.Tools),
	)
	wantSkills := make([]string, 0, len(bundle.Skills()))
	for _, skill := range bundle.Skills() {
		wantSkills = append(wantSkills, skill.Name())
	}
	if !slices.Equal(report.Skills, wantSkills) {
		t.Fatalf("Pi skills = %v, want %v", report.Skills, wantSkills)
	}
	if !slices.Equal(
		report.ManualSkills,
		[]string{"h-commission"},
	) {
		t.Fatalf("Pi manual skills = %v", report.ManualSkills)
	}
	if !slices.Equal(report.Tools, currentKernelMCPToolNames()) {
		t.Fatalf(
			"Pi tools = %v, kernel tools = %v",
			report.Tools,
			currentKernelMCPToolNames(),
		)
	}
	declaration := report.Declaration
	if !declaration.Valid() ||
		declaration.SourceBearingSideRef() != bundle.Ref() ||
		declaration.SourceBearingDigest() != bundle.Digest() ||
		declaration.CoarsenedRenderingDigest() !=
			report.PackageDigest ||
		declaration.NarrowerAdmissibleUse() !=
			currentPiAdmissibleUse {
		t.Fatalf(
			"Pi controlled-coarsening declaration = %s",
			declaration.CanonicalBytes(),
		)
	}
	wantLosses := []initplanning.SourceLossMode{
		initplanning.SourceLossOmittedDetail,
		initplanning.SourceLossRecoverability,
		initplanning.SourceLossRepresentationFactor,
	}
	if !slices.Equal(declaration.SourceLossModes(), wantLosses) {
		t.Fatalf(
			"Pi source-loss modes = %v, want %v",
			declaration.SourceLossModes(),
			wantLosses,
		)
	}
	for _, required := range []string{
		"binding_authorization",
		"canonical_skill_source",
		"semantic_authority",
		"standalone_release_or_parity_evidence",
	} {
		if !slices.Contains(declaration.NonAdmissibleUses(), required) {
			t.Fatalf(
				"Pi non-admissible uses omit %s: %v",
				required,
				declaration.NonAdmissibleUses(),
			)
		}
	}
}

func TestPiCondensedParityRejectsCriticalCarrierDrift(t *testing.T) {
	bundle, err := currentSkillSourceBundle()
	if err != nil {
		t.Fatalf("currentSkillSourceBundle: %v", err)
	}
	tests := map[string]func(fstest.MapFS){
		"missing prompt": func(assets fstest.MapFS) {
			delete(assets, "prompts/h-spec.md")
		},
		"source-first marker": func(assets fstest.MapFS) {
			path := "skills/h-reason/SKILL.md"
			content := string(assets[path].Data)
			content = strings.ReplaceAll(content, "Source-first", "Source first")
			content = strings.ReplaceAll(content, "source-first", "source first")
			assets[path].Data = []byte(content)
		},
		"operator request boundary": func(assets fstest.MapFS) {
			path := "prompts/h-decide.md"
			content := string(assets[path].Data)
			content = strings.Replace(
				content,
				"host_routed_operator_request",
				"generated metadata is convenient",
				1,
			)
			assets[path].Data = []byte(content)
		},
		"native tool set": func(assets fstest.MapFS) {
			path := "extensions/haft/tools.ts"
			content := string(assets[path].Data)
			content = strings.Replace(
				content,
				`name: "haft_memory"`,
				`name: "haft_memory_missing"`,
				1,
			)
			assets[path].Data = []byte(content)
		},
		"kernel source pin": func(assets fstest.MapFS) {
			path := "skills/h-status/SKILL.md"
			content := string(assets[path].Data)
			content = strings.Replace(
				content,
				bundle.KernelCatalogDigest(),
				"sha256:"+strings.Repeat("0", 64),
				1,
			)
			assets[path].Data = []byte(content)
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			assets := clonePiAssets(t)
			mutate(assets)
			packageRoot := filepath.Join(t.TempDir(), "haft-pi")
			outputs, err := piPackageOutputsFromFS(
				assets,
				packageRoot,
			)
			if err != nil {
				t.Fatalf("piPackageOutputsFromFS: %v", err)
			}
			_, err = verifyPiCondensedParity(
				bundle,
				assets,
				packageRoot,
				outputs,
				currentKernelMCPToolNames(),
			)
			if err == nil {
				t.Fatal("drifted Pi package passed condensed parity")
			}
		})
	}
}

func TestPiPackageDigestIsRootIndependentAndContentSensitive(t *testing.T) {
	leftRoot := filepath.Join(t.TempDir(), "left")
	rightRoot := filepath.Join(t.TempDir(), "right")
	left, err := currentPiPackageOutputs(leftRoot)
	if err != nil {
		t.Fatalf("currentPiPackageOutputs left: %v", err)
	}
	right, err := currentPiPackageOutputs(rightRoot)
	if err != nil {
		t.Fatalf("currentPiPackageOutputs right: %v", err)
	}
	leftDigest, err := currentPiPackageDigest(leftRoot, left)
	if err != nil {
		t.Fatalf("currentPiPackageDigest left: %v", err)
	}
	rightDigest, err := currentPiPackageDigest(rightRoot, right)
	if err != nil {
		t.Fatalf("currentPiPackageDigest right: %v", err)
	}
	if leftDigest != rightDigest {
		t.Fatalf(
			"Pi package digest depends on install root: %s != %s",
			leftDigest,
			rightDigest,
		)
	}
	assets := clonePiAssets(t)
	path := "README.md"
	assets[path].Data = append(assets[path].Data, []byte("\nchanged\n")...)
	changed, err := piPackageOutputsFromFS(assets, rightRoot)
	if err != nil {
		t.Fatalf("piPackageOutputsFromFS changed: %v", err)
	}
	changedDigest, err := currentPiPackageDigest(rightRoot, changed)
	if err != nil {
		t.Fatalf("currentPiPackageDigest changed: %v", err)
	}
	if changedDigest == leftDigest {
		t.Fatal("Pi package content change preserved rendering digest")
	}
}

func clonePiAssets(t *testing.T) fstest.MapFS {
	t.Helper()
	result := make(fstest.MapFS)
	err := fs.WalkDir(
		haftpi.Assets,
		".",
		func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			content, err := fs.ReadFile(haftpi.Assets, path)
			if err != nil {
				return err
			}
			result[path] = &fstest.MapFile{
				Data: append([]byte(nil), content...),
				Mode: 0o644,
			}
			return nil
		},
	)
	if err != nil {
		t.Fatalf("clone embedded Pi assets: %v", err)
	}
	return result
}
