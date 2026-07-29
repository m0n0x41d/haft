package governance

import (
	"testing"

	"github.com/m0n0x41d/haft/internal/projectpath"
)

func TestDecisionPathPolicyAuthorityMatrix(t *testing.T) {
	tests := []struct {
		name        string
		data        string
		affected    string
		modulePath  string
		fileBinding bool
		module      bool
		typedRoot   bool
		shouldError bool
	}{
		{
			name:        "legacy defaults to module",
			data:        `{}`,
			affected:    "a.go",
			modulePath:  "",
			fileBinding: false,
			module:      true,
		},
		{
			name:        "exact does not widen",
			data:        `{"governance_mode":"exact"}`,
			affected:    "a.go",
			modulePath:  "",
			fileBinding: false,
			module:      false,
		},
		{
			name:        "implementation footprint is not authority",
			data:        `{"implementation_footprint":{"files":["a.go"]}}`,
			affected:    "a.go",
			modulePath:  "",
			fileBinding: false,
			module:      false,
		},
		{
			name:        "matching explicit file binding restores only exact context",
			data:        `{"implementation_footprint":{"files":["a.go"]},"binding_targets":[{"kind":"symbol","file_path":"a.go"}]}`,
			affected:    "a.go",
			modulePath:  "",
			fileBinding: true,
			module:      false,
		},
		{
			name:        "typed decision without footprint may use exact module root path",
			data:        `{"binding_targets":[{"kind":"symbol","file_path":"a.go"}]}`,
			affected:    "a.go",
			modulePath:  "",
			fileBinding: true,
			module:      false,
			typedRoot:   true,
		},
		{
			name:        "exact mode disables typed module root path",
			data:        `{"governance_mode":"exact","binding_targets":[{"kind":"symbol","file_path":"a.go"}]}`,
			affected:    "a.go",
			modulePath:  "",
			fileBinding: true,
			module:      false,
			typedRoot:   false,
		},
		{
			name:        "binding on another file does not upgrade footprint",
			data:        `{"implementation_footprint":{"files":["a.go","b.go"]},"binding_targets":[{"kind":"symbol","file_path":"a.go"}]}`,
			affected:    "b.go",
			modulePath:  "",
			fileBinding: false,
			module:      false,
		},
		{
			name:        "explicit module binding grants only that module",
			data:        `{"implementation_footprint":{"files":["internal/cli/a.go"]},"binding_targets":[{"kind":"module","module_path":"internal/cli"}]}`,
			affected:    "internal/cli/a.go",
			modulePath:  "internal/cli",
			fileBinding: false,
			module:      true,
		},
		{
			name:        "invalid mode fails closed",
			data:        `{"governance_mode":"recursive-ish"}`,
			shouldError: true,
		},
		{
			name:        "malformed json fails closed",
			data:        `{`,
			shouldError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			policy, err := ParseDecisionPathPolicy(test.data)
			if test.shouldError {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			affected, err := projectpath.Parse(test.affected)
			if err != nil {
				t.Fatal(err)
			}
			module, err := projectpath.ParseModule(test.modulePath)
			if err != nil {
				t.Fatal(err)
			}
			if policy.HasBindingInFile(affected) != test.fileBinding {
				t.Fatalf(
					"file binding = %t, want %t",
					policy.HasBindingInFile(affected),
					test.fileBinding,
				)
			}
			if policy.AllowsModuleContext(module) != test.module {
				t.Fatalf(
					"module context = %t, want %t",
					policy.AllowsModuleContext(module),
					test.module,
				)
			}
			if policy.UsesTypedModuleRootPathScope() != test.typedRoot {
				t.Fatalf(
					"typed module-root path scope = %t, want %t",
					policy.UsesTypedModuleRootPathScope(),
					test.typedRoot,
				)
			}
		})
	}
}

func TestDecisionPathPolicyUsesEffectiveTargetPrecedence(t *testing.T) {
	policy, err := ParseDecisionPathPolicy(`{
		"binding_targets":[
			{"kind":"module","module_path":"internal/legacy"}
		],
		"governance_targets":[{
			"kind":"code",
			"ref":"internal/middle",
			"binding_target":{
				"kind":"module",
				"module_path":"internal/middle"
			}
		}],
		"drift_watch_targets":[{
			"target_ref":"current",
			"trigger":"source_changed",
			"binding_target":{
				"kind":"whole_file_fallback",
				"file_path":"internal/current.go"
			}
		}]
	}`)
	if err != nil {
		t.Fatal(err)
	}
	current, err := projectpath.Parse("internal/current.go")
	if err != nil {
		t.Fatal(err)
	}
	if !policy.HasBindingInFile(current) {
		t.Fatal("effective drift target was not selected")
	}
	for _, raw := range []string{"internal/legacy", "internal/middle"} {
		module, err := projectpath.ParseModule(raw)
		if err != nil {
			t.Fatal(err)
		}
		if policy.AllowsModuleContext(module) {
			t.Fatalf("superseded target %q remained authoritative", raw)
		}
	}
}

func TestDecisionPathPolicyAffectedPathModuleContextMatrix(
	t *testing.T,
) {
	tests := []struct {
		name        string
		data        string
		rawPath     string
		modulePath  string
		indexedFile bool
		want        bool
	}{
		{
			name:        "legacy canonical indexed file",
			data:        `{}`,
			rawPath:     "internal/cli/main.go",
			modulePath:  "internal/cli",
			indexedFile: true,
			want:        true,
		},
		{
			name:        "legacy canonical unindexed file",
			data:        `{}`,
			rawPath:     "internal/cli/missing.go",
			modulePath:  "internal/cli",
			indexedFile: false,
			want:        false,
		},
		{
			name:       "legacy exact module root",
			data:       `{}`,
			rawPath:    "internal/cli",
			modulePath: "internal/cli",
			want:       true,
		},
		{
			name:        "legacy backslash row is read-only",
			data:        `{}`,
			rawPath:     `internal\cli\main.go`,
			modulePath:  "internal/cli",
			indexedFile: true,
			want:        false,
		},
		{
			name:        "legacy dot-cleaned row is read-only",
			data:        `{}`,
			rawPath:     "internal/cli/./main.go",
			modulePath:  "internal/cli",
			indexedFile: true,
			want:        false,
		},
		{
			name:       "typed exact module root",
			data:       `{"binding_targets":[{"kind":"symbol","file_path":"db/migrations.go"}]}`,
			rawPath:    "internal/cli",
			modulePath: "internal/cli",
			want:       true,
		},
		{
			name:        "typed sibling file never widens",
			data:        `{"binding_targets":[{"kind":"symbol","file_path":"db/migrations.go"}]}`,
			rawPath:     "internal/cli/main.go",
			modulePath:  "internal/cli",
			indexedFile: true,
			want:        false,
		},
		{
			name:       "typed footprint root is provenance only",
			data:       `{"implementation_footprint":{"files":["internal/cli"]},"binding_targets":[{"kind":"symbol","file_path":"db/migrations.go"}]}`,
			rawPath:    "internal/cli",
			modulePath: "internal/cli",
			want:       false,
		},
		{
			name:       "exact mode root does not widen",
			data:       `{"governance_mode":"exact","binding_targets":[{"kind":"symbol","file_path":"db/migrations.go"}]}`,
			rawPath:    "internal/cli",
			modulePath: "internal/cli",
			want:       false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			policy, err := ParseDecisionPathPolicy(test.data)
			if err != nil {
				t.Fatal(err)
			}
			affectedPath, err := projectpath.Parse(test.rawPath)
			if err != nil {
				t.Fatal(err)
			}
			modulePath, err := projectpath.ParseModule(test.modulePath)
			if err != nil {
				t.Fatal(err)
			}
			actual := policy.AllowsAffectedPathModuleContext(
				test.rawPath,
				affectedPath,
				modulePath,
				test.indexedFile,
			)
			if actual != test.want {
				t.Fatalf(
					"AllowsAffectedPathModuleContext() = %t, want %t",
					actual,
					test.want,
				)
			}
		})
	}
}
