package cli

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/artifact"
	methodpkg "github.com/m0n0x41d/haft/internal/method"
	"github.com/m0n0x41d/haft/internal/testsupport/profileadmissionfixture"
)

func TestHandleHaftMethodForProjectOmitsSWEForNonSoftware(
	t *testing.T,
) {
	root := t.TempDir()
	harness := profileadmissionfixture.New(t, root)
	harness.AdmitNonSoftwareRevision(t, "method-nonsoftware")
	store := artifact.NewStore(harness.Database())
	result, ref, err := handleHaftMethodForProject(
		context.Background(),
		store,
		filepath.Join(root, ".haft"),
		map[string]any{
			"action":             "pull",
			"task":               "Update a document model",
			"declared_task_kind": "documentation",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if ref != "" {
		t.Fatalf("non-software pull created ref %q", ref)
	}
	response := methodProfileApplicabilityResponse{}
	if err := json.Unmarshal([]byte(result), &response); err != nil {
		t.Fatal(err)
	}
	if response.Applicability != "not_applicable" ||
		response.ArtifactCreated {
		t.Fatalf("non-software response = %#v", response)
	}
	runs, err := store.ListByKind(
		context.Background(),
		artifact.KindMethodRun,
		0,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 0 {
		t.Fatalf("non-software pull created MethodRuns: %#v", runs)
	}
}

func TestHandleHaftMethodForProjectCreatesSWEForSoftware(
	t *testing.T,
) {
	root := t.TempDir()
	harness := profileadmissionfixture.New(t, root)
	harness.AdmitSoftwareRevision(t, "method-software")
	store := artifact.NewStore(harness.Database())
	result, ref, err := handleHaftMethodForProject(
		context.Background(),
		store,
		filepath.Join(root, ".haft"),
		map[string]any{
			"action":             "pull",
			"task":               "Fix a failing parser",
			"declared_task_kind": "bugfix",
			"change_intent":      "fix_bug",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if ref == "" {
		t.Fatalf("software pull omitted MethodRun ref:\n%s", result)
	}
	run, err := store.Get(context.Background(), ref)
	if err != nil {
		t.Fatal(err)
	}
	if run.Meta.Kind != artifact.KindMethodRun {
		t.Fatalf("created kind = %q", run.Meta.Kind)
	}
}

func TestHandleHaftMethodForProjectIgnoresNonScopeSelectorForSingleton(
	t *testing.T,
) {
	root := t.TempDir()
	harness := profileadmissionfixture.New(t, root)
	harness.AdmitSoftwareRevision(t, "method-software-singleton")
	store := artifact.NewStore(harness.Database())
	result, ref, err := handleHaftMethodForProject(
		context.Background(),
		store,
		filepath.Join(root, ".haft"),
		map[string]any{
			"action":             "pull",
			"task":               "Fix a failing parser",
			"declared_task_kind": "bugfix",
			"change_intent":      "fix_bug",
			"scope_id":           "task-019fb9c0-8c3e-7282-be6f-615698e685e6",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if ref == "" {
		t.Fatalf("singleton selector prevented MethodRun:\n%s", result)
	}
	if !strings.Contains(result, "ignored_unnecessary_selector") ||
		!strings.Contains(result, "selected=\"software-method-software-singleton\"") {
		t.Fatalf("singleton selector diagnostic is absent:\n%s", result)
	}
}

func TestHandleHaftMethodForProjectCatalogIsTypedWhenProfileUnderdetermined(
	t *testing.T,
) {
	root := t.TempDir()
	harness := profileadmissionfixture.New(t, root)
	store := artifact.NewStore(harness.Database())
	result, ref, err := handleHaftMethodForProject(
		context.Background(),
		store,
		filepath.Join(root, ".haft"),
		map[string]any{
			"action":        "catalog",
			"method_status": string(methodpkg.LifecycleCurrent),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if ref != "" {
		t.Fatalf("underdetermined catalog returned ref %q", ref)
	}
	response := methodProfileApplicabilityResponse{}
	if err := json.Unmarshal([]byte(result), &response); err != nil {
		t.Fatal(err)
	}
	if response.Applicability !=
		string(projectSpecificationProfileUnderdetermined) {
		t.Fatalf("underdetermined response = %#v", response)
	}
	if response.SchemaVersion != 2 {
		t.Fatalf("schema_version = %d, want 2", response.SchemaVersion)
	}
	if response.BlocksCurrentWork ||
		response.RequiresMethodRun ||
		response.RequiresHumanGate {
		t.Fatalf(
			"underdetermined profile became a work gate: %#v",
			response,
		)
	}
	if response.Continuation !=
		"continue_already_authorized_work_without_method_run" {
		t.Fatalf("continuation = %q", response.Continuation)
	}
	for _, required := range []string{
		"do_not_request_profile_admission_only_to_obtain_method_run",
		"do_not_create_or_broaden_work_commission_to_compensate",
	} {
		if !stringSliceContains(response.ForbiddenCompensations, required) {
			t.Fatalf(
				"forbidden_compensations = %#v, missing %q",
				response.ForbiddenCompensations,
				required,
			)
		}
	}
	if response.ProfileApplicability.Cue == nil {
		t.Fatalf("underdetermined response omitted cue: %#v", response)
	}
}
