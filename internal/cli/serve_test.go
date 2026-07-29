package cli

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/projectledger"
)

func TestServeExpectedProjectIDForRunPrefersFlag(t *testing.T) {
	oldServeExpectedProjectID := serveExpectedProjectID
	t.Cleanup(func() {
		serveExpectedProjectID = oldServeExpectedProjectID
	})
	t.Setenv(envExpectedProjectID, "env-project")

	serveExpectedProjectID = "flag-project"

	if got := serveExpectedProjectIDForRun(); got != "flag-project" {
		t.Fatalf("serveExpectedProjectIDForRun = %q, want flag-project", got)
	}
}

func TestServeExpectedProjectIDForRunFallsBackToEnv(t *testing.T) {
	oldServeExpectedProjectID := serveExpectedProjectID
	t.Cleanup(func() {
		serveExpectedProjectID = oldServeExpectedProjectID
	})
	t.Setenv(envExpectedProjectID, "env-project")

	serveExpectedProjectID = ""

	if got := serveExpectedProjectIDForRun(); got != "env-project" {
		t.Fatalf("serveExpectedProjectIDForRun = %q, want env-project", got)
	}
}

func TestRevalidatedServeV5HandlerFailsCleanlyBeforeSQLiteReadWhenSidecarGenerationChanged(
	t *testing.T,
) {
	revalidator := &serveProjectLedgerRevalidationSequence{
		results: []error{
			projectledger.ErrSQLiteSidecarGenerationChanged,
		},
	}
	called := false
	next := func(
		context.Context,
		string,
		json.RawMessage,
	) (string, error) {
		called = true
		return "untrusted", nil
	}
	handler := makeRevalidatedServeV5Handler(
		ProjectBinding{
			ProjectRoot: "/checked/project",
			ProjectID:   "qnt_a7f3b2c1",
		},
		revalidator,
		next,
	)

	result, err := handler(
		context.Background(),
		"haft_query",
		json.RawMessage(`{}`),
	)
	if result != "" || err == nil {
		t.Fatalf("stale handler result = %q, error = %v", result, err)
	}
	if called {
		t.Fatal("stale handler invoked the unchecked MCP operation")
	}
	for _, want := range []string{
		"SQLite sidecar generation changed",
		"restart the long-lived Haft MCP process",
		"do not run `haft project migrate`",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("stale handler error missing %q:\n%s", want, err)
		}
	}
	if strings.Contains(strings.ToLower(err.Error()), "malformed") {
		t.Fatalf("stale handler leaked a corruption diagnosis: %v", err)
	}
}

func TestRevalidatedServeV5HandlerQualifiesPostOperationGenerationChange(
	t *testing.T,
) {
	revalidator := &serveProjectLedgerRevalidationSequence{
		results: []error{
			nil,
			projectledger.ErrSQLiteSidecarGenerationChanged,
		},
	}
	next := func(
		context.Context,
		string,
		json.RawMessage,
	) (string, error) {
		return "operation-result", nil
	}
	handler := makeRevalidatedServeV5Handler(
		ProjectBinding{
			ProjectRoot: "/checked/project",
			ProjectID:   "qnt_a7f3b2c1",
		},
		revalidator,
		next,
	)

	result, err := handler(
		context.Background(),
		"haft_note",
		json.RawMessage(`{}`),
	)
	if result != "operation-result" || err == nil {
		t.Fatalf("post-change result = %q, error = %v", result, err)
	}
	for _, want := range []string{
		"mutation outcome as unknown until idempotent replay",
		"restart the long-lived Haft MCP process",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("post-change error missing %q:\n%s", want, err)
		}
	}
}

type serveProjectLedgerRevalidationSequence struct {
	results []error
	calls   int
}

func (sequence *serveProjectLedgerRevalidationSequence) Revalidate(
	context.Context,
) error {
	if sequence.calls >= len(sequence.results) {
		return errors.New("unexpected project-ledger revalidation")
	}
	result := sequence.results[sequence.calls]
	sequence.calls++
	return result
}
