package scopeauth

import (
	"path/filepath"
	"slices"
	"testing"
)

func TestAuthorizeWorkspaceDiffAllowsAllowedPath(t *testing.T) {
	facts := testPathFacts()
	changedPath := filepath.Join(facts.WorkspaceRoot, "src", "app.go")
	scope := CommissionScope{
		AllowedPaths:  []string{"src/**"},
		AffectedFiles: []string{"docs/plan.md"},
		Lockset:       []string{"docs/plan.md"},
	}

	summary := AuthorizeWorkspaceDiff(scope, []string{changedPath}, facts)

	if summary.Verdict != Allowed {
		t.Fatalf("verdict = %s, want %s", summary.Verdict, Allowed)
	}
	if !summary.CanApply() {
		t.Fatalf("CanApply = false, want true")
	}
	if !slices.Equal(summary.Allowed, []string{"src/app.go"}) {
		t.Fatalf("allowed = %#v, want src/app.go", summary.Allowed)
	}
}

func TestAuthorizeWorkspaceDiffForbiddenOverridesBroadAllowedPath(t *testing.T) {
	facts := testPathFacts()
	scope := CommissionScope{
		AllowedPaths:   []string{"**/*"},
		ForbiddenPaths: []string{"secrets/**"},
	}

	summary := AuthorizeWorkspaceDiff(scope, []string{"secrets/key.txt"}, facts)

	if summary.Verdict != Forbidden {
		t.Fatalf("verdict = %s, want %s", summary.Verdict, Forbidden)
	}
	if summary.CanApply() {
		t.Fatalf("CanApply = true, want false")
	}
	if !slices.Equal(summary.Forbidden, []string{"secrets/key.txt"}) {
		t.Fatalf("forbidden = %#v, want secrets/key.txt", summary.Forbidden)
	}
	if len(summary.Allowed) != 0 {
		t.Fatalf("allowed = %#v, want empty", summary.Allowed)
	}
	reason := summary.BlockingReason()
	if reason.Code != ReasonForbidden {
		t.Fatalf("blocking reason code = %s, want %s", reason.Code, ReasonForbidden)
	}
	if !slices.Equal(reason.Paths, []string{"secrets/key.txt"}) {
		t.Fatalf("blocking paths = %#v, want secrets/key.txt", reason.Paths)
	}
}

func TestAuthorizeWorkspaceDiffLocksetOnlyDoesNotAuthorizeMutation(t *testing.T) {
	facts := testPathFacts()
	scope := CommissionScope{
		Lockset: []string{"src/app.go"},
	}

	summary := AuthorizeWorkspaceDiff(scope, []string{"src/app.go"}, facts)

	if summary.Verdict != OutOfScope {
		t.Fatalf("verdict = %s, want %s", summary.Verdict, OutOfScope)
	}
	if summary.CanApply() {
		t.Fatalf("CanApply = true, want false")
	}
	if !slices.Equal(summary.OutOfScope, []string{"src/app.go"}) {
		t.Fatalf("out_of_scope = %#v, want src/app.go", summary.OutOfScope)
	}
}

func TestAuthorizeWorkspaceDiffAffectedFilesOnlyDoesNotAuthorizeMutation(t *testing.T) {
	facts := testPathFacts()
	scope := CommissionScope{
		AffectedFiles: []string{"src/app.go"},
	}

	summary := AuthorizeWorkspaceDiff(scope, []string{"src/app.go"}, facts)

	if summary.Verdict != OutOfScope {
		t.Fatalf("verdict = %s, want %s", summary.Verdict, OutOfScope)
	}
	if summary.CanApply() {
		t.Fatalf("CanApply = true, want false")
	}
	if !slices.Equal(summary.OutOfScope, []string{"src/app.go"}) {
		t.Fatalf("out_of_scope = %#v, want src/app.go", summary.OutOfScope)
	}
}

func TestAuthorizeWorkspaceDiffEmptyScopeRefusesApply(t *testing.T) {
	facts := testPathFacts()
	scope := CommissionScope{}

	summary := AuthorizeWorkspaceDiff(scope, []string{"src/app.go"}, facts)

	if summary.Verdict != UnknownScope {
		t.Fatalf("verdict = %s, want %s", summary.Verdict, UnknownScope)
	}
	if summary.CanApply() {
		t.Fatalf("CanApply = true, want false")
	}
	if !slices.Equal(summary.UnknownScope, []string{"src/app.go"}) {
		t.Fatalf("unknown_scope = %#v, want src/app.go", summary.UnknownScope)
	}
}

func testPathFacts() PathFacts {
	return PathFacts{
		WorkspaceRoot: filepath.Join("/", "tmp", "haft-workspace"),
		ProjectRoot:   filepath.Join("/", "tmp", "haft-project"),
	}
}
