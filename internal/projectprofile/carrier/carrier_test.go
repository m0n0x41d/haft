package carrier_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/projectprofile"
	"github.com/m0n0x41d/haft/internal/projectprofile/carrier"
)

func TestLoadMissingLegacyProfileAsAuto(t *testing.T) {
	profile, err := carrier.Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, ok := profile.(projectprofile.Auto); !ok {
		t.Fatalf("profile = %T, want projectprofile.Auto", profile)
	}
}

func TestLoadRejectsSymlinkAndNonRegularCarrier(t *testing.T) {
	t.Run("symlink", func(t *testing.T) {
		root := t.TempDir()
		haftDir := filepath.Join(root, ".haft")
		if err := os.MkdirAll(haftDir, 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		target := filepath.Join(root, "elsewhere.yaml")
		if err := os.WriteFile(target, []byte("schema_version: haft.project-profile/v1\nprofile:\n  kind: auto\n"), 0o600); err != nil {
			t.Fatalf("WriteFile target: %v", err)
		}
		path := filepath.Join(haftDir, "project-profile.yaml")
		if err := os.Symlink(target, path); err != nil {
			t.Fatalf("Symlink: %v", err)
		}
		if _, err := carrier.Load(root); err == nil || !strings.Contains(err.Error(), "non-symlink") {
			t.Fatalf("Load symlink error = %v", err)
		}
	})

	t.Run("directory", func(t *testing.T) {
		root := t.TempDir()
		path := filepath.Join(root, ".haft", "project-profile.yaml")
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatalf("MkdirAll carrier path: %v", err)
		}
		if _, err := carrier.Load(root); err == nil || !strings.Contains(err.Error(), "regular") {
			t.Fatalf("Load non-regular error = %v", err)
		}
	})
}

func TestLegacyFixturesRemainReadableCompatibilityCorpus(t *testing.T) {
	fixtures := []string{
		"declared-software.yaml",
		"declared-non-software.yaml",
		"declared-mixed.yaml",
		"declared-imported.yaml",
	}
	for _, fixture := range fixtures {
		t.Run(fixture, func(t *testing.T) {
			source := readFixture(t, fixture)
			original := bytes.Clone(source)
			profile, err := carrier.Decode(source)
			if err != nil {
				t.Fatalf("Decode: %v", err)
			}
			if _, ok := profile.(projectprofile.Declared); !ok {
				t.Fatalf("profile = %T, want legacy projectprofile.Declared", profile)
			}
			if !bytes.Equal(source, original) {
				t.Fatal("legacy carrier bytes changed during decode")
			}
		})
	}
}

func TestLoadLegacyCarrierDoesNotModifyCompatibilityInput(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, filepath.FromSlash(carrier.RelativePath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	want := readFixture(t, "declared-software.yaml")
	if err := os.WriteFile(path, want, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	profile, err := carrier.Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, ok := profile.(projectprofile.Declared); !ok {
		t.Fatalf("profile = %T, want legacy projectprofile.Declared", profile)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("legacy carrier changed during Load\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestDecodedLegacyDeclaredCannotBecomeFinalV1OrBindingApplicability(t *testing.T) {
	profile, err := carrier.Decode(readFixture(t, "declared-software.yaml"))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if _, ok := profile.(projectprofile.ConfiguredProjectProfileV1); ok {
		t.Fatal("legacy YAML profile unexpectedly implements ConfiguredProjectProfileV1")
	}
	result := projectprofile.ResolveSoftwareSystemSpecMigration(profile)
	underdetermined, ok := result.(projectprofile.Underdetermined)
	if !ok {
		t.Fatalf("applicability = %T, want projectprofile.Underdetermined", result)
	}
	missing := underdetermined.MissingBasis().Values()
	if len(missing) != 1 || missing[0] != projectprofile.MissingCanonicalDurableProfileAdmission {
		t.Fatalf("missing basis = %#v, want canonical durable admission", missing)
	}
}

func TestMixedFixturePreservesStableScopeIDsAndTypedReferences(t *testing.T) {
	profile, err := carrier.Decode(readFixture(t, "declared-mixed.yaml"))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	declared, ok := profile.(projectprofile.Declared)
	if !ok {
		t.Fatalf("profile = %T, want projectprofile.Declared", profile)
	}
	scopes := declared.Scopes().Values()
	if len(scopes) != 2 {
		t.Fatalf("scopes = %d, want 2", len(scopes))
	}
	nonSoftware, ok := scopes[0].(projectprofile.NonSoftwareRealization)
	if !ok {
		t.Fatalf("first scope = %T", scopes[0])
	}
	if nonSoftware.ScopeID().String() != "knowledge-model" {
		t.Fatalf("scope id = %q", nonSoftware.ScopeID().String())
	}
	orientation, ok := nonSoftware.KindOrientation().(projectprofile.ReferencedKindOrientation)
	if !ok {
		t.Fatalf("kind orientation = %T, want ReferencedKindOrientation", nonSoftware.KindOrientation())
	}
	if orientation.Ref().String() != "U.Episteme" {
		t.Fatalf("kind orientation ref = %q, want U.Episteme", orientation.Ref().String())
	}
	patterns := nonSoftware.GoverningPatternRefs()
	if len(patterns) != 2 || patterns[0].String() != "A.7" || patterns[1].String() != "A.6.B" {
		t.Fatalf("governing patterns = %#v", patterns)
	}
	software, ok := scopes[1].(projectprofile.SoftwareRealization)
	if !ok {
		t.Fatalf("second scope = %T", scopes[1])
	}
	if software.ScopeID().String() != "model-tooling" {
		t.Fatalf("scope id = %q", software.ScopeID().String())
	}
}

func TestDecodeFailsClosedForMalformedUnknownOrAmbiguousCarrier(t *testing.T) {
	cases := map[string][]byte{
		"malformed yaml":        readFixture(t, "malformed.yaml"),
		"unknown field":         replaceFixture(t, "declared-software.yaml", "  kind: declared\n", "  kind: declared\n  surprise: true\n"),
		"unknown schema":        replaceFixture(t, "declared-software.yaml", carrier.SchemaVersion, "haft.project-profile/v2"),
		"multiple documents":    append(readFixture(t, "declared-software.yaml"), []byte("---\n{}\n")...),
		"explicit null":         replaceFixture(t, "declared-software.yaml", "      entity_ref: entity:haft\n", "      entity_ref: null\n"),
		"duplicate mapping key": replaceFixture(t, "declared-software.yaml", "  kind: declared\n", "  kind: declared\n  kind: declared\n"),
		"anchored value":        replaceFixture(t, "declared-software.yaml", "      scope_id: haft-software\n", "      scope_id: &scope haft-software\n"),
		"aliased value":         append([]byte("scope: &scope haft-software\n"), replaceFixture(t, "declared-software.yaml", "      scope_id: haft-software\n", "      scope_id: *scope\n")...),
		"whitespace identity":   replaceFixture(t, "declared-software.yaml", "      scope_id: haft-software\n", "      scope_id: \" haft-software\"\n"),
		"noncanonical time":     replaceFixture(t, "declared-software.yaml", "11:50:00+04:00", "11:50:00.000+04:00"),
		"relative project root": replaceFixture(
			t,
			"declared-software.yaml",
			"/workspace/haft",
			"relative/project",
		),
		"candidate provenance": replaceFixture(
			t,
			"declared-software.yaml",
			"    declaration_authority_basis_ref: authority-basis:onboard:1\n",
			"    authorization_speech_act_ref: speech-act:operator-profile-approval\n",
		),
	}

	for name, source := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := carrier.Decode(source)
			if err == nil {
				t.Fatal("Decode succeeded")
			}
		})
	}
}

func TestDecodeRejectsScopePayloadDigestThatDoesNotMatchScopes(t *testing.T) {
	tampered := replaceFixture(
		t,
		"declared-software.yaml",
		"scope_id: haft-software",
		"scope_id: renamed-haft-software",
	)

	_, err := carrier.Decode(tampered)

	if err == nil || !strings.Contains(err.Error(), "scope-payload digest does not match") {
		t.Fatalf("error = %v, want canonical scope-payload digest mismatch", err)
	}
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return content
}

func replaceFixture(t *testing.T, name, old, replacement string) []byte {
	t.Helper()
	source := string(readFixture(t, name))
	if !strings.Contains(source, old) {
		t.Fatalf("fixture %s does not contain %q", name, old)
	}
	return []byte(strings.Replace(source, old, replacement, 1))
}
