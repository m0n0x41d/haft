package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/operatorrequest"
	"github.com/m0n0x41d/haft/internal/profiledeclarationpreparation"
	"github.com/m0n0x41d/haft/internal/profiledetector"
	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/testsupport/kerneldbfixture"
)

func TestPrepareBeforeAdmissionCommitsV3AuthorityThenV2WorkAndReplays(t *testing.T) {
	database, root := newPreparationDatabase(t)
	input := newPreparationInput(t, root, []string{"go.mod", "internal/kernel.go"})
	policy := newPreparationPolicy(t, input, "base")
	checkedAt := time.Date(2026, 7, 19, 9, 0, 0, 0, time.UTC)
	revalidations := 0
	revalidate := func(ctx context.Context) error {
		revalidations++
		transaction, err := sqlitetransaction.BeginImmediate(ctx, database)
		if err != nil {
			return fmt.Errorf("authority transaction remained open: %w", err)
		}
		finish := transaction.Rollback(ctx)
		return finish.Err()
	}

	first, err := PrepareBeforeAdmission(
		context.Background(),
		database,
		root,
		input,
		policy,
		sequenceClock(
			checkedAt,
			checkedAt.Add(time.Microsecond),
			checkedAt.Add(2*time.Microsecond),
			checkedAt.Add(3*time.Microsecond),
			checkedAt.Add(4*time.Microsecond),
		),
		revalidate,
	)
	if err != nil {
		t.Fatal(err)
	}
	if first.Kind() != OutcomePreparedNew {
		t.Fatalf("first outcome = %q", first.Kind())
	}
	prepared := requirePrepared(t, first)
	values, ok := prepared.Values()
	if !ok {
		t.Fatal("prepared outcome omitted v2 values")
	}
	typedInput, ok := values.WorkRecord().ProfileOnboardingWorkInputRefV2()
	if !ok || typedInput != input.Ref() {
		t.Fatalf("v2 Work input = %q, present = %t", typedInput.String(), ok)
	}
	assertPreparationCounts(t, database, []int{1, 1, 1, 1, 0, 0, 0})

	second, err := PrepareBeforeAdmission(
		context.Background(),
		database,
		root,
		input,
		policy,
		sequenceClock(checkedAt.Add(time.Minute)),
		revalidate,
	)
	if err != nil {
		t.Fatal(err)
	}
	if second.Kind() != OutcomeExactExisting {
		t.Fatalf("second outcome = %q", second.Kind())
	}
	replayed := requirePrepared(t, second)
	_, firstDigest, _ := prepared.AuthorityResolution()
	_, replayDigest, _ := replayed.AuthorityResolution()
	if replayDigest != firstDigest {
		t.Fatal("exact replay changed the authority resolution digest")
	}
	if revalidations != 2 {
		t.Fatalf("revalidations = %d", revalidations)
	}
	assertPreparationCounts(t, database, []int{1, 1, 1, 1, 0, 0, 0})
}

func TestPrepareBeforeAdmissionRecoversAfterPostAuthorityRevalidationFailure(t *testing.T) {
	database, root := newPreparationDatabase(t)
	input := newPreparationInput(
		t,
		root,
		[]string{"go.mod", "internal/kernel.go", "models/current.onnx"},
	)
	policy := newPreparationPolicy(t, input, "recovery")
	checkedAt := time.Date(2026, 7, 19, 10, 0, 0, 0, time.UTC)
	_, err := PrepareBeforeAdmission(
		context.Background(),
		database,
		root,
		input,
		policy,
		sequenceClock(checkedAt),
		func(context.Context) error { return fmt.Errorf("ledger drifted") },
	)
	if err == nil {
		t.Fatal("revalidation failure was accepted")
	}
	assertPreparationCounts(t, database, []int{1, 1, 1, 0, 0, 0, 0})

	recovered, err := PrepareBeforeAdmission(
		context.Background(),
		database,
		root,
		input,
		policy,
		sequenceClock(
			checkedAt.Add(time.Minute),
			checkedAt.Add(time.Minute+time.Microsecond),
			checkedAt.Add(time.Minute+2*time.Microsecond),
			checkedAt.Add(time.Minute+3*time.Microsecond),
			checkedAt.Add(time.Minute+4*time.Microsecond),
		),
		func(context.Context) error { return nil },
	)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.Kind() != OutcomePreparedNew {
		t.Fatalf("recovery outcome = %q", recovered.Kind())
	}
	assertPreparationCounts(t, database, []int{1, 1, 1, 1, 0, 0, 0})
}

func TestPrepareBeforeAdmissionReturnsConflictWithoutCandidate(t *testing.T) {
	database, root := newPreparationDatabase(t)
	input := newPreparationInput(t, root, []string{"go.mod", "main.go"})
	checkedAt := time.Date(2026, 7, 19, 11, 0, 0, 0, time.UTC)
	first, err := PrepareBeforeAdmission(
		context.Background(),
		database,
		root,
		input,
		newPreparationPolicy(t, input, "first"),
		sequenceClock(
			checkedAt,
			checkedAt.Add(time.Microsecond),
			checkedAt.Add(2*time.Microsecond),
			checkedAt.Add(3*time.Microsecond),
			checkedAt.Add(4*time.Microsecond),
		),
		func(context.Context) error { return nil },
	)
	if err != nil || first.Kind() != OutcomePreparedNew {
		t.Fatalf("initial preparation = %v, %v", first, err)
	}
	conflict, err := PrepareBeforeAdmission(
		context.Background(),
		database,
		root,
		input,
		newPreparationPolicy(t, input, "different-carrier"),
		sequenceClock(checkedAt.Add(time.Minute)),
		func(context.Context) error { return nil },
	)
	if err != nil {
		t.Fatal(err)
	}
	if conflict.Kind() != OutcomeConflict {
		t.Fatalf("conflict outcome = %q", conflict.Kind())
	}
	if _, ok := conflict.Prepared(); ok {
		t.Fatal("conflict exposed an admission candidate")
	}
	if detail, ok := conflict.ConflictDetail(); !ok || detail == "" {
		t.Fatal("conflict omitted an actionable detail")
	}
	assertPreparationCounts(t, database, []int{1, 1, 1, 1, 0, 0, 0})
}

func newPreparationDatabase(t testing.TB) (*sql.DB, string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	projectID := "qnt_f17e0001"
	if err := os.MkdirAll(filepath.Join(root, ".haft"), 0o755); err != nil {
		t.Fatal(err)
	}
	config := []byte("id: " + projectID + "\nname: preparation-fixture\n")
	if err := os.WriteFile(filepath.Join(root, ".haft", "project.yaml"), config, 0o644); err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	directory := filepath.Join(home, ".haft", "projects", projectID)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	store, err := kerneldbfixture.OpenCurrentStore(
		filepath.Join(directory, "haft.db"),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := projectledger.BindInitialized(
		context.Background(),
		root,
		time.Now().UTC(),
	); err != nil {
		t.Fatal(err)
	}
	return store.GetRawDB(), root
}

func newPreparationInput(
	t testing.TB,
	root string,
	files []string,
) profiledeclarationpreparation.ProfileOnboardingWorkInput {
	t.Helper()
	snapshot, err := profiledetector.NewSnapshot(root, files, len(files), false)
	if err != nil {
		t.Fatal(err)
	}
	suggestion := profiledetector.Detect(snapshot)
	proposal, err := profiledeclarationpreparation.ProposeProfileOnboardingWorkInput(suggestion)
	if err != nil {
		t.Fatal(err)
	}
	input, err := profiledeclarationpreparation.DecodeProfileOnboardingWorkInput(
		proposal,
		suggestion,
	)
	if err != nil {
		t.Fatal(err)
	}
	return input
}

func newPreparationPolicy(
	t testing.TB,
	input profiledeclarationpreparation.ProfileOnboardingWorkInput,
	suffix string,
) profiledeclarationpreparation.Policy {
	t.Helper()
	request, err := operatorrequest.New(
		operatorrequest.ProfileDeclaration,
		"profile-review:"+suffix,
		input.CanonicalJSON(),
	)
	if err != nil {
		t.Fatal(err)
	}
	policy, err := profiledeclarationpreparation.NewHostRoutedOperatorRequestPolicy(request)
	if err != nil {
		t.Fatal(err)
	}
	return policy
}

func requirePrepared(t testing.TB, outcome Outcome) Prepared {
	t.Helper()
	prepared, ok := outcome.Prepared()
	if !ok {
		t.Fatalf("outcome %q omitted Prepared", outcome.Kind())
	}
	if _, ok := prepared.Candidate(); !ok {
		t.Fatal("Prepared omitted a valid candidate")
	}
	return prepared
}

func sequenceClock(values ...time.Time) func() time.Time {
	index := 0
	return func() time.Time {
		if index >= len(values) {
			return values[len(values)-1]
		}
		value := values[index]
		index++
		return value
	}
}

func assertPreparationCounts(
	t testing.TB,
	database *sql.DB,
	want []int,
) {
	t.Helper()
	tables := []string{
		"profile_onboarding_work_inputs_v1",
		"profile_declaration_authority_bases_v5",
		"profile_declaration_authority_resolutions_v5",
		"profile_onboarding_work_records",
		"project_profile_admissions_v5",
		"profile_declaration_authority_uses_v5",
		"project_profile_revisions_v5",
	}
	for index, table := range tables {
		count := 0
		query := "SELECT COUNT(*) FROM " + table
		if err := database.QueryRow(query).Scan(&count); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if count != want[index] {
			t.Fatalf("%s count = %d, want %d", table, count, want[index])
		}
	}
}
