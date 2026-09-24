package code

import (
	"reflect"
	"strings"
	"testing"
)

func testBuildIndex(t *testing.T, source map[string]string, config Config) Index {
	t.Helper()
	files := map[string][]byte{}
	for p, raw := range source {
		files[p] = []byte(raw)
	}
	index, err := IndexFromFiles(files, config)
	if err != nil {
		t.Fatal(err)
	}
	return index
}

func TestSelectedTestBuildVariantsAndArchitecture(t *testing.T) {
	files := map[string]string{
		"go.mod":                 "module example.test/answer\n\ngo 1.25\n",
		"answer.go":              "package answer\nfunc Answer()int\n",
		"answer_test.go":         "package answer\nimport \"testing\"\nfunc TestAnswer(t *testing.T){}\n",
		"external_test.go":       "package answer_test\nimport _ \"example.test/answer/setup\"\n",
		"setup/setup.go":         "package setup\nimport _ \"example.test/answer/helper\"\n",
		"setup/setup_test.go":    "package setup\nimport _ \"example.test/answer/testonly\"\n",
		"setup/external_test.go": "package setup_test\nimport _ \"example.test/answer/testonly\"\n",
		"helper/helper.go":       "package helper\n",
		"testonly/testonly.go":   "package testonly\n",
		"answer_amd64.s":         "#include \"textflag.h\"\n/* legal comment */ # include \"answer.h\"\n",
		"answer_arm64.s":         "#include \"unavailable-on-other-architecture.h\"\n",
		"answer.h":               "#include \"constants.inc\"\n",
		"constants.inc":          "#define ANSWER 1\n",
		"answer_amd64.syso":      "captured object bytes",
		"unused.txt":             "ordinary runtime data is not a local compile input",
	}
	index := testBuildIndex(t, files, Config{GOOS: "linux", GOARCH: "amd64", Toolchain: "go1.25.8", IncludeTests: true})
	got, err := index.GoTestBuild("answer_test.go")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"answer.go", "answer.h", "answer_amd64.s", "answer_amd64.syso", "answer_test.go", "constants.inc", "external_test.go", "helper/helper.go", "setup/setup.go"}
	if !reflect.DeepEqual(got.Files, want) || got.Package != "example.test/answer" {
		t.Fatalf("selected build %#v want %#v", got, want)
	}
	// Choosing the external declaration still builds internal tests and ordinary
	// package code. Dependency tests and other-architecture assembly remain absent.
	ext, err := index.GoTestBuild("external_test.go")
	if err != nil || !reflect.DeepEqual(ext, got) {
		t.Fatalf("external test variant: %#v %v", ext, err)
	}
}

func TestSelectedTestBuildRejectsUnsupportedComposition(t *testing.T) {
	base := map[string]string{"go.mod": "module example.test/answer\n\ngo 1.25\n", "answer.go": "package answer\nfunc Answer()int{return 1}\n", "answer_test.go": "package answer\nimport \"testing\"\nfunc TestAnswer(t *testing.T){}\n"}
	cases := []struct {
		name  string
		files map[string]string
	}{
		{"local-replace", map[string]string{"go.mod": "module example.test/answer\n\nreplace other.test/m => ../other\n"}},
		{"vendor", map[string]string{"vendor/modules.txt": "# vendor manifest"}},
		{"missing-local-dependency", map[string]string{"external_test.go": "package answer_test\nimport _ \"example.test/answer/missing\"\n"}},
		{"macro-include", map[string]string{"answer.s": "#define HEADER \"outside.h\"\n#include HEADER\n"}},
		{"parent-include", map[string]string{"answer.s": "/* comment */ #include \"../outside.h\"\n"}},
		{"ignored-local-toolchain-shadow", map[string]string{"answer.s": "#include \"textflag.h\"\n", "textflag.h": "#define NOSPLIT 4\n", ".gitignore": "textflag.h\n"}},
		{"compiler-feature-tag", map[string]string{"tagged.go": "//go:build amd64.v2\n\npackage answer\n"}},
		{"c-source", map[string]string{"answer.c": "int answer(void){return 1;}\n"}},
		{"workspace", map[string]string{"go.work": "go 1.25\nuse .\n"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			files := map[string]string{}
			for p, v := range base {
				files[p] = v
			}
			for p, v := range tc.files {
				files[p] = v
			}
			index := testBuildIndex(t, files, Config{GOOS: "linux", GOARCH: "amd64", Toolchain: "go1.25.8", IncludeTests: true})
			if _, err := index.GoTestBuild("answer_test.go"); err == nil {
				t.Fatal("unsupported local composition received a complete build closure")
			}
		})
	}
}

func TestAssemblyIncludeCommentsDoNotHideInputs(t *testing.T) {
	names, err := assemblyIncludes([]byte("/*header*/ #include /*name*/ \"table.inc\" // comment\n// #include \"absent.h\"\n"))
	if err != nil || !reflect.DeepEqual(names, []string{"table.inc"}) {
		t.Fatal(names, err)
	}
	_, err = assemblyIncludes([]byte("#include \"dir/table.inc\"\n"))
	if err == nil || !strings.Contains(err.Error(), "captured package") {
		t.Fatal(err)
	}
}
