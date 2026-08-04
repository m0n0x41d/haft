package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/initplanning"
	"github.com/m0n0x41d/haft/internal/method"
	profileadmissionsqlite "github.com/m0n0x41d/haft/internal/profileadmission/sqlite"
	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/testsupport/profileadmissionfixture"
)

func TestTypedPublicCorePlanAndEffectInstallOnlyRequiredProfileCapabilities(
	t *testing.T,
) {
	tests := []struct {
		name  string
		admit func(
			*testing.T,
			*profileadmissionfixture.Harness,
		) profileadmissionsqlite.CanonicalProfileAdmission
		wantSpecPaths    []string
		wantMethodExists bool
	}{
		{
			name: "software",
			admit: func(
				t *testing.T,
				harness *profileadmissionfixture.Harness,
			) profileadmissionsqlite.CanonicalProfileAdmission {
				return harness.AdmitSoftwareRevision(
					t,
					"typed-init-software",
				)
			},
			wantSpecPaths: []string{
				filepath.Join("specs", "target-system.md"),
				filepath.Join("specs", "software-system.md"),
				filepath.Join("specs", "term-map.md"),
			},
			wantMethodExists: true,
		},
		{
			name: "software with admitted target",
			admit: func(
				t *testing.T,
				harness *profileadmissionfixture.Harness,
			) profileadmissionsqlite.CanonicalProfileAdmission {
				return harness.AdmitSoftwareRevisionWithTargetEntity(
					t,
					"typed-init-software-target",
					"entity:typed-init-target",
				)
			},
			wantSpecPaths: []string{
				filepath.Join("specs", "target-system.md"),
				filepath.Join("specs", "software-system.md"),
				filepath.Join("specs", "term-map.md"),
			},
			wantMethodExists: true,
		},
		{
			name: "non-software",
			admit: func(
				t *testing.T,
				harness *profileadmissionfixture.Harness,
			) profileadmissionsqlite.CanonicalProfileAdmission {
				return harness.AdmitNonSoftwareRevision(
					t,
					"typed-init-documents",
				)
			},
			wantSpecPaths: []string{
				filepath.Join("specs", "target-system.md"),
				filepath.Join("specs", "term-map.md"),
			},
			wantMethodExists: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			projectRoot := physicalInitTestTempDir(t)
			harness := profileadmissionfixture.New(
				t,
				projectRoot,
			)
			admitted := test.admit(t, harness)
			request, err := compilePublicInitRequest(
				weakPublicInitRequest{
					invocation:  initplanning.InvocationExplicit,
					projectRoot: projectRoot,
					projectID:   harness.ProjectID(),
					coreOnly:    true,
					overseer:    publicOverseerWeakDisabled(),
				},
			)
			if err != nil {
				t.Fatalf("compilePublicInitRequest: %v", err)
			}
			homeRoot, err := os.UserHomeDir()
			if err != nil {
				t.Fatalf("resolve fixture home: %v", err)
			}
			homeRoot, err = filepath.EvalSymlinks(homeRoot)
			if err != nil {
				t.Fatalf("resolve physical fixture home: %v", err)
			}
			core, err := compilePublicCorePlan(
				context.Background(),
				request,
				homeRoot,
			)
			if err != nil {
				t.Fatalf("compilePublicCorePlan: %v", err)
			}
			wantPaths := expectedInitProfileDependentPaths(
				test.wantSpecPaths,
				test.wantMethodExists,
			)
			haftDir := filepath.Join(projectRoot, ".haft")
			gotPlanned := plannedInitProfileDependentPaths(
				t,
				haftDir,
				core,
			)
			if !reflect.DeepEqual(gotPlanned, wantPaths) {
				t.Fatalf(
					"planned profile-dependent paths = %#v, want %#v",
					gotPlanned,
					wantPaths,
				)
			}
			if gotExisting := existingInitProfileDependentPaths(
				t,
				haftDir,
			); len(gotExisting) != 0 {
				t.Fatalf(
					"planning wrote profile-dependent paths: %#v",
					gotExisting,
				)
			}
			effect := newPublicProjectCoreEffect(
				request,
				io.Discard,
			)
			if _, err := effect.ApplyCore(
				context.Background(),
				core,
			); err != nil {
				t.Fatalf("ApplyCore: %v", err)
			}
			service, err := profileadmissionsqlite.NewService(
				harness.Database(),
			)
			if err != nil {
				t.Fatalf("create profile admission resolver: %v", err)
			}
			resolved := service.ResolveCurrent(
				context.Background(),
				harness.Root(),
			)
			current, present := resolved.Admission()
			if !present {
				t.Fatalf(
					"init lost canonical profile admission: kind=%q",
					resolved.Kind(),
				)
			}
			if current.AdmissionRecordRef() != admitted.AdmissionRecordRef() ||
				current.AdmissionRecordDigest() !=
					admitted.AdmissionRecordDigest() ||
				current.PayloadDigest() != admitted.PayloadDigest() ||
				current.LedgerRevision() != admitted.LedgerRevision() {
				t.Fatalf(
					"init changed canonical profile admission:\nbefore=%q/%q/%q/%d\nafter=%q/%q/%q/%d",
					admitted.AdmissionRecordRef(),
					admitted.AdmissionRecordDigest(),
					admitted.PayloadDigest(),
					admitted.LedgerRevision().Value(),
					current.AdmissionRecordRef(),
					current.AdmissionRecordDigest(),
					current.PayloadDigest(),
					current.LedgerRevision().Value(),
				)
			}
			gotPaths := existingInitProfileDependentPaths(
				t,
				haftDir,
			)
			if !reflect.DeepEqual(gotPaths, wantPaths) {
				t.Fatalf(
					"installed profile-dependent paths = %#v, want %#v",
					gotPaths,
					wantPaths,
				)
			}
			methodPath := filepath.Join(
				haftDir,
				"methods",
				method.CatalogID,
				"manifest.yaml",
			)
			_, methodErr := os.Stat(methodPath)
			if test.wantMethodExists && methodErr != nil {
				t.Fatalf("required SWE MethodPack missing: %v", methodErr)
			}
			if !test.wantMethodExists && !os.IsNotExist(methodErr) {
				t.Fatalf(
					"non-applicable SWE MethodPack was installed: %v",
					methodErr,
				)
			}
		})
	}
}

func expectedInitProfileDependentPaths(
	specPaths []string,
	includeMethods bool,
) []string {
	result := append([]string{}, specPaths...)
	if includeMethods {
		catalog := method.BuiltinCatalog()
		methodRoot := filepath.Join("methods", catalog.ID)
		result = append(
			result,
			filepath.Join(methodRoot, "manifest.yaml"),
		)
		for _, definition := range catalog.Methods {
			result = append(
				result,
				filepath.Join(methodRoot, definition.ID+".yaml"),
			)
		}
	}
	sort.Strings(result)
	return result
}

func plannedInitProfileDependentPaths(
	t *testing.T,
	haftDir string,
	core initplanning.CoreProjectPlan,
) []string {
	t.Helper()
	result := []string{}
	for _, effect := range core.FileEffects() {
		relative, err := filepath.Rel(haftDir, effect.Path())
		if err != nil {
			t.Fatalf("relativize planned path %s: %v", effect.Path(), err)
		}
		if !initProfileDependentRelativePath(relative) {
			continue
		}
		result = append(result, relative)
	}
	sort.Strings(result)
	return result
}

func existingInitProfileDependentPaths(
	t *testing.T,
	haftDir string,
) []string {
	t.Helper()
	result := []string{}
	err := filepath.WalkDir(
		haftDir,
		func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			relative, err := filepath.Rel(haftDir, path)
			if err != nil {
				return err
			}
			if initProfileDependentRelativePath(relative) {
				result = append(result, relative)
			}
			return nil
		},
	)
	if err != nil {
		t.Fatalf("inspect profile-dependent paths: %v", err)
	}
	sort.Strings(result)
	return result
}

func initProfileDependentRelativePath(relative string) bool {
	clean := filepath.Clean(relative)
	for _, root := range []string{"specs", "methods"} {
		if clean == root ||
			strings.HasPrefix(clean, root+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func TestTypedPublicInitWithoutAdmissionCreatesNoProfileDependentCarriers(
	t *testing.T,
) {
	projectRoot := physicalInitTestTempDir(t)
	home := physicalInitTestTempDir(t)
	t.Setenv("HOME", home)
	restoreDirectory := changeInitTestDirectory(t, projectRoot)
	defer restoreDirectory()
	restoreFlags := captureInitHostFlagState()
	defer restoreFlags.apply()
	clearInitHostFlags()
	initCodex = true
	initLocal = true
	initNoFileInstructions = true
	output := bytes.Buffer{}
	command := newPublicInitTestCommand()
	command.SetOut(&output)
	command.SetErr(&output)
	var codex bool
	var local bool
	var omitInstructions bool
	command.Flags().BoolVar(&codex, "codex", false, "")
	command.Flags().BoolVar(&local, "local", false, "")
	command.Flags().BoolVar(
		&omitInstructions,
		"no-file-instructions",
		false,
		"",
	)
	for _, flag := range []string{
		"codex",
		"local",
		"no-file-instructions",
	} {
		if err := command.Flags().Set(flag, "true"); err != nil {
			t.Fatalf("set %s flag: %v", flag, err)
		}
	}

	if err := runPublicInit(command, nil); err != nil {
		t.Fatalf("runPublicInit: %v", err)
	}
	if !strings.Contains(
		output.String(),
		"Haft initialization complete",
	) {
		t.Fatalf("typed init output = %q", output.String())
	}
	for _, want := range []string{
		"Next setup surface: h-onboard status",
		"Profile admission is separate from initialization and may still be required.",
	} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("fresh init output missing %q: %q", want, output.String())
		}
	}

	currentOutput := bytes.Buffer{}
	currentCommand := newPublicInitTestCommand()
	currentCommand.SetOut(&currentOutput)
	currentCommand.SetErr(&currentOutput)
	var currentCodex bool
	var currentLocal bool
	var currentOmitInstructions bool
	currentCommand.Flags().BoolVar(&currentCodex, "codex", false, "")
	currentCommand.Flags().BoolVar(&currentLocal, "local", false, "")
	currentCommand.Flags().BoolVar(
		&currentOmitInstructions,
		"no-file-instructions",
		false,
		"",
	)
	for _, flag := range []string{
		"codex",
		"local",
		"no-file-instructions",
	} {
		if err := currentCommand.Flags().Set(flag, "true"); err != nil {
			t.Fatalf("set current %s flag: %v", flag, err)
		}
	}
	if err := runPublicInit(currentCommand, nil); err != nil {
		t.Fatalf("rerun current public init: %v", err)
	}
	for _, want := range []string{
		"Haft is already initialized",
		"Next setup surface: h-onboard status",
		"Profile admission is separate from initialization and may still be required.",
	} {
		if !strings.Contains(currentOutput.String(), want) {
			t.Fatalf(
				"already-current init output missing %q: %q",
				want,
				currentOutput.String(),
			)
		}
	}
	for _, path := range []string{
		filepath.Join(projectRoot, ".haft", "specs"),
		filepath.Join(
			projectRoot,
			".haft",
			"methods",
			method.CatalogID,
		),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("Auto init created profile-dependent path %s: %v", path, err)
		}
	}
	if _, err := project.Load(filepath.Join(projectRoot, ".haft")); err != nil {
		t.Fatalf("Auto init omitted project identity: %v", err)
	}
}

func TestInitScopeFlagIsReadOnlySelectionOnly(t *testing.T) {
	if initCmd.Flags().Lookup("scope-id") == nil {
		t.Fatal("haft init omitted exact --scope-id selector")
	}
	for _, forbidden := range []string{
		"declare-profile",
		"write-profile",
		"approve-profile",
		"force-software",
	} {
		if initCmd.Flags().Lookup(forbidden) != nil {
			t.Fatalf("haft init exposed forbidden effect flag --%s", forbidden)
		}
	}
}

func TestInitProfileSelectionErrorSupportsErrorsAs(t *testing.T) {
	failure := initProfileSelectionError{
		kind:              projectSpecificationScopeChoiceRequired,
		availableScopeIDs: []string{"documents", "software"},
	}
	var typed initProfileSelectionError
	if !errors.As(failure, &typed) {
		t.Fatal("init profile selection error lost its typed variant")
	}
}
