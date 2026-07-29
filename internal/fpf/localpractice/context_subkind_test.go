package localpractice

import (
	"strings"
	"testing"
)

func TestParseBoundedContextAndSubkindDeclarations(t *testing.T) {
	source := contextSubkindCarrier(t)
	parsed, err := Parse(source)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	declarations := parsed.Carrier().Signature().Vocabulary().Declarations()
	if len(declarations) != 12 {
		t.Fatalf("declarations = %d, want 12", len(declarations))
	}
	boundedContext, ok := declarations[0].(BoundedContextDeclaration)
	if !ok {
		t.Fatalf("bounded-context declaration type = %T", declarations[0])
	}
	if boundedContext.Kind() != DeclarationBoundedContext ||
		boundedContext.Symbol().Value() != "haft-project" {
		t.Fatalf("bounded-context declaration = %#v", boundedContext)
	}
	subkind, ok := declarations[1].(SubkindDeclaration)
	if !ok {
		t.Fatalf("subkind declaration type = %T", declarations[1])
	}
	if subkind.Kind() != DeclarationSubkind ||
		subkind.Symbol().Value() != "Haft.Subkind.ProjectConcernEntity" ||
		subkind.ChildKind().Value() != "Haft.ProjectConcern" ||
		subkind.SuperKind().Value() != "U.Entity" {
		t.Fatalf("subkind declaration = %#v", subkind)
	}
}

func TestParseBoundedContextAndSubkindDeclarationsAreClosed(t *testing.T) {
	baseline := string(contextSubkindCarrier(t))
	tests := []struct {
		name        string
		old         string
		replacement string
		want        string
	}{
		{
			name:        "bounded context extra field",
			old:         "        symbol: haft-project\n      - kind: subkind",
			replacement: "        symbol: haft-project\n        child_kind: Haft.ProjectConcern\n      - kind: subkind",
			want:        "unknown field \"child_kind\"",
		},
		{
			name:        "subkind missing super",
			old:         "        super_kind: U.Entity\n      - kind: value_kind",
			replacement: "      - kind: value_kind",
			want:        "missing required field \"super_kind\"",
		},
		{
			name:        "subkind equal endpoints",
			old:         "        super_kind: U.Entity",
			replacement: "        super_kind: Haft.ProjectConcern",
			want:        "child_kind and super_kind must be distinct",
		},
		{
			name:        "subkind non-qualified child",
			old:         "        child_kind: Haft.ProjectConcern",
			replacement: "        child_kind: Haft/ProjectConcern",
			want:        "must not contain whitespace, slash, or backslash",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source := strings.Replace(baseline, test.old, test.replacement, 1)
			if source == baseline {
				t.Fatal("test mutation did not change source")
			}
			_, err := Parse([]byte(source))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Parse() error = %v, want %q", err, test.want)
			}
		})
	}
}

func contextSubkindCarrier(t *testing.T) []byte {
	t.Helper()
	source := string(readGoldenCarrier(t))
	source = strings.Replace(
		source,
		"    - Haft.ProjectConcern\n",
		"    - haft-project\n    - Haft.ProjectConcern\n",
		1,
	)
	source = strings.Replace(
		source,
		"      - kind: value_kind\n",
		"      - kind: bounded_context\n"+
			"        symbol: haft-project\n"+
			"      - kind: subkind\n"+
			"        symbol: Haft.Subkind.ProjectConcernEntity\n"+
			"        child_kind: Haft.ProjectConcern\n"+
			"        super_kind: U.Entity\n"+
			"      - kind: value_kind\n",
		1,
	)
	return []byte(source)
}
