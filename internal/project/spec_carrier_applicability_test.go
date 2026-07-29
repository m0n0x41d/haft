package project

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/m0n0x41d/haft/internal/projectprofile"
)

func TestRequiredSpecCarriersFollowExactScopeCapabilityMatrix(t *testing.T) {
	tests := []struct {
		name          string
		applicability ProjectSpecificationSetApplicability
		wantPaths     []string
		wantMethod    projectprofile.CapabilityApplicabilityKind
	}{
		{
			name: "software",
			applicability: mustSpecificationApplicability(
				t,
				mustProjectProfileSoftwareScope(t, "software"),
				"software",
			),
			wantPaths: []string{
				filepath.Join("specs", "software-system.md"),
				filepath.Join("specs", "term-map.md"),
			},
			wantMethod: projectprofile.CapabilityRequired,
		},
		{
			name: "non-software",
			applicability: mustSpecificationApplicability(
				t,
				mustProjectProfileNonSoftwareScope(t, "documents"),
				"documents",
			),
			wantPaths: []string{
				filepath.Join("specs", "term-map.md"),
			},
			wantMethod: projectprofile.CapabilityNotApplicable,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			carriers, err := RequiredSpecCarriers(test.applicability)
			if err != nil {
				t.Fatalf("RequiredSpecCarriers: %v", err)
			}
			paths := specCarrierPaths(carriers)
			if !reflect.DeepEqual(paths, test.wantPaths) {
				t.Fatalf("carrier paths = %#v, want %#v", paths, test.wantPaths)
			}
			entry, err := test.applicability.ScopedCapabilityApplicability(
				projectprofile.SWEMethodPackCapability,
			)
			if err != nil || entry.Kind() != test.wantMethod {
				t.Fatalf(
					"SWE MethodPack entry = %#v, err=%v, want %q",
					entry,
					err,
					test.wantMethod,
				)
			}
		})
	}
}

func TestEnsureRequiredSpecCarriersPreservesExistingBytesAndNeverDeletes(
	t *testing.T,
) {
	haftDir := filepath.Join(t.TempDir(), ".haft")
	targetPath := filepath.Join(haftDir, "specs", "target-system.md")
	softwarePath := filepath.Join(haftDir, "specs", "software-system.md")
	targetBytes := []byte("# Existing target carrier\n")
	softwareBytes := []byte("# Existing software carrier\n")
	writeSpecCarrierApplicabilityFixture(t, targetPath, targetBytes)
	writeSpecCarrierApplicabilityFixture(t, softwarePath, softwareBytes)
	applicability := mustSpecificationApplicability(
		t,
		mustProjectProfileNonSoftwareScope(t, "documents"),
		"documents",
	)

	if err := EnsureRequiredSpecCarriers(haftDir, applicability); err != nil {
		t.Fatalf("EnsureRequiredSpecCarriers: %v", err)
	}
	assertSpecCarrierApplicabilityBytes(t, targetPath, targetBytes)
	assertSpecCarrierApplicabilityBytes(t, softwarePath, softwareBytes)
	termMapPath := filepath.Join(haftDir, "specs", "term-map.md")
	if _, err := os.Stat(termMapPath); err != nil {
		t.Fatalf("required term-map carrier was not created: %v", err)
	}
}

func specCarrierPaths(carriers []SpecCarrier) []string {
	if len(carriers) == 0 {
		return nil
	}
	paths := make([]string, len(carriers))
	fillSpecCarrierPaths(carriers, paths, 0)
	return paths
}

func fillSpecCarrierPaths(
	carriers []SpecCarrier,
	paths []string,
	index int,
) {
	if index == len(carriers) {
		return
	}
	paths[index] = carriers[index].RelativePath
	fillSpecCarrierPaths(carriers, paths, index+1)
}

func writeSpecCarrierApplicabilityFixture(
	t *testing.T,
	path string,
	data []byte,
) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertSpecCarrierApplicabilityBytes(
	t *testing.T,
	path string,
	want []byte,
) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s bytes changed:\ngot: %q\nwant: %q", path, got, want)
	}
}
