package projectpath

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseCanonicalProjectPath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{name: "plain", raw: "internal/cli/query.go", want: "internal/cli/query.go"},
		{name: "windows separators", raw: `internal\cli\query.go`, want: "internal/cli/query.go"},
		{name: "clean", raw: " internal//cli/./query.go ", want: "internal/cli/query.go"},
		{name: "literal wildcard", raw: "pkg/a_%/file.go", want: "pkg/a_%/file.go"},
		{name: "blank", raw: "", wantErr: true},
		{name: "root dot", raw: ".", wantErr: true},
		{name: "absolute", raw: "/tmp/file.go", wantErr: true},
		{name: "drive absolute", raw: `C:\tmp\file.go`, wantErr: true},
		{name: "drive relative", raw: `C:relative\file.go`, wantErr: true},
		{name: "unc path", raw: `\\server\share\file.go`, wantErr: true},
		{name: "escape", raw: "../file.go", wantErr: true},
		{name: "cleaned escape", raw: "a/../../file.go", wantErr: true},
		{name: "control", raw: "internal/\x00file.go", wantErr: true},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := Parse(test.raw)
			if test.wantErr {
				if err == nil {
					t.Fatalf("Parse(%q) succeeded", test.raw)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse(%q): %v", test.raw, err)
			}
			if got.String() != test.want {
				t.Fatalf("Parse(%q) = %q, want %q", test.raw, got.String(), test.want)
			}
		})
	}
}

func TestModuleContainsIsSegmentSafe(t *testing.T) {
	t.Parallel()
	module, err := ParseModule("internal/cli")
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{"internal/cli", "internal/cli/query.go"} {
		candidate, parseErr := Parse(raw)
		if parseErr != nil {
			t.Fatal(parseErr)
		}
		if !module.Contains(candidate) {
			t.Fatalf("%q should be in module", raw)
		}
	}
	for _, raw := range []string{"internal/client/query.go", "internal/artifact/query.go"} {
		candidate, parseErr := Parse(raw)
		if parseErr != nil {
			t.Fatal(parseErr)
		}
		if module.Contains(candidate) {
			t.Fatalf("%q should not be in module", raw)
		}
	}
	root, err := ParseModule("")
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := Parse("main.go")
	if err != nil {
		t.Fatal(err)
	}
	if !root.Contains(candidate) {
		t.Fatal("root module should contain main.go")
	}
}

func TestResolveMostSpecificModule(t *testing.T) {
	t.Parallel()
	root, err := NewModuleRef("root", "")
	if err != nil {
		t.Fatal(err)
	}
	internal, err := NewModuleRef("internal", "internal")
	if err != nil {
		t.Fatal(err)
	}
	cli, err := NewModuleRef("cli", `internal\cli`)
	if err != nil {
		t.Fatal(err)
	}
	target, err := Parse("internal/cli/main.go")
	if err != nil {
		t.Fatal(err)
	}

	resolved, ok, err := ResolveMostSpecificModule(
		[]ModuleRef{root, internal, cli},
		target,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || resolved.ID() != "cli" ||
		resolved.Path().String() != "internal/cli" {
		t.Fatalf("resolved = %#v, %v", resolved, ok)
	}
}

func TestResolveMostSpecificModuleRejectsAmbiguousCanonicalPath(t *testing.T) {
	t.Parallel()
	left, err := NewModuleRef("left", `internal\cli`)
	if err != nil {
		t.Fatal(err)
	}
	right, err := NewModuleRef("right", "internal/cli")
	if err != nil {
		t.Fatal(err)
	}
	target, err := Parse("internal/cli/main.go")
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = ResolveMostSpecificModule(
		[]ModuleRef{left, right},
		target,
	)
	if err == nil {
		t.Fatal("duplicate canonical module paths must fail closed")
	}
}

func TestResolveExistingRejectsEscapingSymlink(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	candidate, err := Parse("escape/secret.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveExisting(root, candidate); err == nil {
		t.Fatal("escaping symlink was accepted")
	}
}

func TestResolvePotentialRejectsMissingChildBelowEscapingSymlink(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	candidate, err := Parse("escape/new/file.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ResolvePotential(root, candidate); err == nil {
		t.Fatal("missing child below escaping symlink was accepted")
	}
}
