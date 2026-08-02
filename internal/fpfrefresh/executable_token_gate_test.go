package fpfrefresh

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const candidateTokenGateObservedEnvironment = "HAFT_TOKEN_GATE_TEST_OBSERVED"

func TestExecutableCandidateTokenGateRunsAgainstExactBoundCandidatePaths(t *testing.T) {
	fixture := newCandidateTokenGateFixture(t)
	observedPath := filepath.Join(t.TempDir(), "observed.txt")
	t.Setenv(candidateTokenGateObservedEnvironment, observedPath)
	t.Setenv(CandidateTokenGateDatabasePathEnvironment, "/stale/database")
	t.Setenv(CandidateTokenGateLockPathEnvironment, "/stale/lock")
	t.Setenv(CandidateTokenGateFixturePathEnvironment, "/stale/fixture")

	scriptPath := writeCandidateTokenGateScript(t, `#!/bin/sh
set -eu
printf '%s\n%s\n%s\n' \
  "$HAFT_QUERY_TOKEN_GATE_DATABASE_PATH" \
  "$HAFT_QUERY_TOKEN_GATE_LOCK_PATH" \
  "$HAFT_QUERY_TOKEN_GATE_FIXTURE_PATH" \
  > "$HAFT_TOKEN_GATE_TEST_OBSERVED"
`)
	gate := ExecutableCandidateTokenGate{
		ShellPath:  candidateTokenGateShellPath(t),
		ScriptPath: scriptPath,
	}
	var port CandidateTokenGate = gate
	if err := port.VerifyCandidate(context.Background(), fixture.input); err != nil {
		t.Fatalf("VerifyCandidate() error = %v", err)
	}
	observed, err := os.ReadFile(observedPath)
	if err != nil {
		t.Fatalf("read observed token-gate environment: %v", err)
	}
	want := strings.Join([]string{
		fixture.input.DatabasePath,
		fixture.input.IntegrationLockPath,
		fixture.input.FixturePath,
		"",
	}, "\n")
	if string(observed) != want {
		t.Fatalf("candidate token-gate paths = %q, want %q", observed, want)
	}
}

func TestExecutableCandidateTokenGateFailsClosedBeforeScriptOnCoordinateDrift(
	t *testing.T,
) {
	tests := map[string]func(*testing.T, candidateTokenGateFixture){
		"database bytes": func(t *testing.T, fixture candidateTokenGateFixture) {
			t.Helper()
			if err := os.WriteFile(
				fixture.input.DatabasePath,
				[]byte("changed database bytes"),
				0o600,
			); err != nil {
				t.Fatal(err)
			}
		},
		"fixture bytes": func(t *testing.T, fixture candidateTokenGateFixture) {
			t.Helper()
			payload, err := os.ReadFile(fixture.input.FixturePath)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(
				fixture.input.FixturePath,
				append(payload, ' '),
				0o600,
			); err != nil {
				t.Fatal(err)
			}
		},
		"missing fixture binding": func(t *testing.T, fixture candidateTokenGateFixture) {
			t.Helper()
			fixture.lock.TokenGate = nil
			writeCandidateTokenGateLock(t, fixture.input.IntegrationLockPath, fixture.lock)
		},
		"noncanonical lock bytes": func(t *testing.T, fixture candidateTokenGateFixture) {
			t.Helper()
			payload, err := os.ReadFile(fixture.input.IntegrationLockPath)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(
				fixture.input.IntegrationLockPath,
				append(payload, ' '),
				0o600,
			); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			fixture := newCandidateTokenGateFixture(t)
			observedPath := filepath.Join(t.TempDir(), "script-ran")
			t.Setenv(candidateTokenGateObservedEnvironment, observedPath)
			scriptPath := writeCandidateTokenGateScript(t, `#!/bin/sh
set -eu
: > "$HAFT_TOKEN_GATE_TEST_OBSERVED"
`)
			mutate(t, fixture)

			err := (ExecutableCandidateTokenGate{
				ShellPath:  candidateTokenGateShellPath(t),
				ScriptPath: scriptPath,
			}).VerifyCandidate(context.Background(), fixture.input)
			if err == nil {
				t.Fatal("VerifyCandidate() error = nil")
			}
			if _, statErr := os.Stat(observedPath); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("token-gate script ran despite coordinate drift: %v", statErr)
			}
		})
	}
}

func TestExecutableCandidateTokenGateRequiresCanonicalAbsoluteInputPaths(t *testing.T) {
	fixture := newCandidateTokenGateFixture(t)
	fixture.input.DatabasePath = filepath.Dir(fixture.input.DatabasePath) +
		string(filepath.Separator) + "." +
		string(filepath.Separator) + filepath.Base(fixture.input.DatabasePath)
	if filepath.Clean(fixture.input.DatabasePath) == fixture.input.DatabasePath {
		t.Fatal("test did not construct a noncanonical path")
	}
	gate := ExecutableCandidateTokenGate{
		ShellPath:  candidateTokenGateShellPath(t),
		ScriptPath: writeCandidateTokenGateScript(t, "#!/bin/sh\nexit 0\n"),
	}
	if err := gate.VerifyCandidate(context.Background(), fixture.input); err == nil {
		t.Fatal("VerifyCandidate(noncanonical path) error = nil")
	}
}

func TestExecutableCandidateTokenGateReturnsScriptFailure(t *testing.T) {
	fixture := newCandidateTokenGateFixture(t)
	gate := ExecutableCandidateTokenGate{
		ShellPath: candidateTokenGateShellPath(t),
		ScriptPath: writeCandidateTokenGateScript(t, `#!/bin/sh
echo "candidate gate failed" >&2
exit 23
`),
	}
	err := gate.VerifyCandidate(context.Background(), fixture.input)
	if err == nil || !strings.Contains(err.Error(), "candidate gate failed") {
		t.Fatalf("VerifyCandidate() error = %v, want bounded script detail", err)
	}
}

func TestCandidateTokenGateFuncRejectsNilAndAdaptsFunction(t *testing.T) {
	var nilFunction CandidateTokenGateFunc
	if err := nilFunction.VerifyCandidate(
		context.Background(),
		CandidateTokenGateInput{},
	); err == nil {
		t.Fatal("nil CandidateTokenGateFunc error = nil")
	}
	called := false
	function := CandidateTokenGateFunc(func(
		_ context.Context,
		input CandidateTokenGateInput,
	) error {
		called = input.DatabasePath == "database"
		return nil
	})
	if err := function.VerifyCandidate(
		context.Background(),
		CandidateTokenGateInput{DatabasePath: "database"},
	); err != nil {
		t.Fatalf("CandidateTokenGateFunc error = %v", err)
	}
	if !called {
		t.Fatal("CandidateTokenGateFunc did not receive input")
	}
}

func TestBoundedCandidateTokenGateOutputCapsRetainedDetail(t *testing.T) {
	var output boundedCandidateTokenGateOutput
	payload := strings.Repeat("x", maxCandidateTokenGateOutputBytes+1024)
	written, err := output.Write([]byte(payload))
	if err != nil {
		t.Fatal(err)
	}
	if written != len(payload) {
		t.Fatalf("Write() = %d, want caller-visible %d", written, len(payload))
	}
	detail := output.Detail()
	if !strings.HasSuffix(detail, "[output truncated]") ||
		len(detail) > maxCandidateTokenGateOutputBytes+len("\n[output truncated]") {
		t.Fatalf("bounded detail length/suffix = %d %q", len(detail), detail[len(detail)-32:])
	}
}

type candidateTokenGateFixture struct {
	input CandidateTokenGateInput
	lock  IntegrationLock
}

func newCandidateTokenGateFixture(t *testing.T) candidateTokenGateFixture {
	t.Helper()
	root := t.TempDir()
	databasePath := filepath.Join(root, "candidate.db")
	databasePayload := []byte("exact candidate database bytes")
	if err := os.WriteFile(databasePath, databasePayload, 0o600); err != nil {
		t.Fatal(err)
	}
	fixturePath := filepath.Join(root, "corpus.json")
	fixturePayload := []byte(`{
  "schema_version": "haft.fpf-query-token-gate-corpus/v1",
  "fixture_revision": "candidate-fixture-v1",
  "cases": [
    {"case_id": "candidate-case"}
  ]
}
`)
	if err := os.WriteFile(fixturePath, fixturePayload, 0o600); err != nil {
		t.Fatal(err)
	}
	tokenCoordinates, err := ReadTokenGateCoordinates(fixturePath)
	if err != nil {
		t.Fatal(err)
	}
	const digest = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	lock := IntegrationLock{
		SchemaVersion: IntegrationLockSchemaVersion,
		GeneratedBy:   "candidate-token-gate-test",
		Coordinates: IntegrationCoordinates{
			SourceRevision:         "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			ReadmeDocumentDigest:   digest,
			SpecDocumentDigest:     digest,
			DatabaseDigest:         digestBytesSHA256(databasePayload),
			SourceUnitCount:        1,
			IndexSchemaVersion:     "candidate-index-v1",
			BaseTypeEnvRef:         "typeenv:" + digest,
			BaseTypeEnvDigest:      digest,
			TypeEnvCompilerEdition: "candidate-compiler-v1",
		},
		TokenGate: &tokenCoordinates,
	}
	lockPath := filepath.Join(root, "fpf-integration.lock.json")
	writeCandidateTokenGateLock(t, lockPath, lock)
	return candidateTokenGateFixture{
		input: CandidateTokenGateInput{
			DatabasePath:        databasePath,
			IntegrationLockPath: lockPath,
			FixturePath:         fixturePath,
		},
		lock: lock,
	}
}

func writeCandidateTokenGateLock(t *testing.T, path string, lock IntegrationLock) {
	t.Helper()
	payload, err := MarshalIntegrationLock(lock)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeCandidateTokenGateScript(t *testing.T, payload string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "token-gate.sh")
	if err := os.WriteFile(path, []byte(payload), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func candidateTokenGateShellPath(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("POSIX shell is unavailable")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Clean(absolute)
}
