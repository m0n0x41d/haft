package profileonboarding

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	kerneldb "github.com/m0n0x41d/haft/db"
	profiledeclarationpreparationsqlite "github.com/m0n0x41d/haft/internal/profiledeclarationpreparation/sqlite"
	"github.com/m0n0x41d/haft/internal/profiledetector"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

func TestRunProfileDeclarationExplicitPolicyAdmitsAndReplaysAfterRestart(
	t *testing.T,
) {
	databasePath := filepath.Join(t.TempDir(), "generic-declaration.db")
	store, err := kerneldb.NewStore(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	database := store.GetRawDB()
	database.SetMaxOpenConns(1)
	root := canonicalWorkInputTestRoot(t)
	if err := os.MkdirAll(filepath.Join(root, ".haft"), 0o755); err != nil {
		t.Fatal(err)
	}
	insertProfileOnboardingTestLedgerBinding(
		t,
		database,
		root,
		time.Now().UTC().Round(0).Add(-time.Minute),
	)
	suggestion := workInputTestSuggestion(
		t,
		root,
		[]string{"go.mod", "internal/kernel.go"},
	)
	if suggestion.Classification() != profiledetector.SoftwareSignals {
		t.Fatalf("fixture classification = %q", suggestion.Classification())
	}
	proposal, err := ProposeProfileOnboardingWorkInput(suggestion)
	if err != nil {
		t.Fatal(err)
	}
	input, err := DecodeProfileOnboardingWorkInput(proposal, suggestion)
	if err != nil {
		t.Fatal(err)
	}
	policyCarrier := []byte("authority:\n  profile_declaration_mode: explicit_h_onboard\n")
	policy, err := NewProfileDeclarationPolicy(
		ProfileDeclarationModeExplicitHOnboard,
		".haft/config.yaml",
		policyCarrier,
	)
	if err != nil {
		t.Fatal(err)
	}
	revalidationCount := 0
	revalidate := profileDeclarationTestRevalidator(database, &revalidationCount)
	first, err := RunProfileDeclaration(
		context.Background(),
		database,
		root,
		input,
		policy,
		revalidate,
	)
	if err != nil {
		t.Fatal(err)
	}
	if first.Kind() != ResultSynchronized {
		failure, _ := first.Failure()
		rejections, _ := first.Rejections()
		t.Fatalf(
			"first declaration = %q, failure %s: %s, rejections %#v",
			first.Kind(),
			failure.Code(),
			failure.Detail(),
			rejections,
		)
	}
	firstAdmission, ok := first.Admission()
	if !ok {
		t.Fatal("first declaration omitted canonical admission")
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close first process database: %v", err)
	}
	restartedStore, err := kerneldb.NewStore(databasePath)
	if err != nil {
		t.Fatalf("reopen database after restart: %v", err)
	}
	t.Cleanup(func() { _ = restartedStore.Close() })
	database = restartedStore.GetRawDB()
	database.SetMaxOpenConns(1)
	revalidate = profileDeclarationTestRevalidator(database, &revalidationCount)
	second, err := RunProfileDeclaration(
		context.Background(),
		database,
		root,
		input,
		policy,
		revalidate,
	)
	if err != nil {
		t.Fatal(err)
	}
	if second.Kind() != ResultSynchronized {
		failure, _ := second.Failure()
		t.Fatalf(
			"replayed declaration = %q, failure %s: %s",
			second.Kind(),
			failure.Code(),
			failure.Detail(),
		)
	}
	secondAdmission, ok := second.Admission()
	if !ok {
		t.Fatal("replayed declaration omitted canonical admission")
	}
	if firstAdmission.AdmissionRecordDigest() != secondAdmission.AdmissionRecordDigest() {
		t.Fatal("replayed declaration resolved another admission")
	}
	if revalidationCount == 0 {
		t.Fatal("profile declaration never revalidated the checked ledger")
	}
}

func TestRunProfileDeclarationAdmitsManualFallbackWithExactProvenance(
	t *testing.T,
) {
	store, err := kerneldb.NewStore(
		filepath.Join(t.TempDir(), "manual-declaration.db"),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	database := store.GetRawDB()
	database.SetMaxOpenConns(1)
	root := canonicalWorkInputTestRoot(t)
	insertProfileOnboardingTestLedgerBinding(
		t,
		database,
		root,
		time.Now().UTC().Round(0).Add(-time.Minute),
	)
	suggestion := workInputTestSuggestion(
		t,
		root,
		[]string{"README.md"},
	)
	if suggestion.Classification() !=
		profiledetector.InsufficientDetectorBasis {
		t.Fatalf(
			"manual fixture classification = %q",
			suggestion.Classification(),
		)
	}
	proposal, err := ProposeManualProfileOnboardingWorkInput(
		suggestion,
		ManualProfileProposalInput{
			Basis: "The repository is a documentation product.",
			Scopes: []ManualProfileScopeInput{{
				ScopeID:         "documents",
				Label:           "Project handbook",
				RealizationKind: profiledetector.NonSoftwareRealization,
				EvidencePaths:   []string{"README.md"},
			}},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	input, err := DecodeProfileOnboardingWorkInput(
		proposal,
		suggestion,
	)
	if err != nil {
		t.Fatal(err)
	}
	policy, err := NewProfileDeclarationPolicy(
		ProfileDeclarationModeExplicitHOnboard,
		".haft/config.yaml",
		[]byte(
			"authority:\n  profile_declaration_mode: explicit_h_onboard\n",
		),
	)
	if err != nil {
		t.Fatal(err)
	}
	revalidationCount := 0
	result, err := RunProfileDeclaration(
		context.Background(),
		database,
		root,
		input,
		policy,
		profileDeclarationTestRevalidator(
			database,
			&revalidationCount,
		),
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Kind() != ResultSynchronized {
		failure, _ := result.Failure()
		rejections, _ := result.Rejections()
		t.Fatalf(
			"manual declaration = %q, failure %s: %s, rejections %#v",
			result.Kind(),
			failure.Code(),
			failure.Detail(),
			rejections,
		)
	}
	if _, ok := result.Admission(); !ok {
		t.Fatal("manual declaration omitted canonical admission")
	}
	classifier := ""
	classificationPolicy := ""
	workInputCanonical := ""
	err = database.QueryRow(
		`SELECT basis.classifier_version, basis.policy_version, input.canonical_json
		 FROM profile_declaration_authority_bases_v3 basis
		 JOIN profile_onboarding_work_inputs_v1 input
		   ON input.work_input_ref = basis.work_input_ref
		 WHERE basis.project_root = ?`,
		root,
	).Scan(
		&classifier,
		&classificationPolicy,
		&workInputCanonical,
	)
	if err != nil {
		t.Fatal(err)
	}
	if classifier != "haft-profile-manual-scope/v1" ||
		classificationPolicy !=
			"haft-profile-manual-scope-policy/v1" {
		t.Fatalf(
			"manual durable classifier=%q policy=%q",
			classifier,
			classificationPolicy,
		)
	}
	for _, expected := range []string{
		`"proposal_source":"manual_scope_proposal"`,
		`"observation_detector_version":"` +
			suggestion.DetectorVersion() + `"`,
		`"observation_policy_version":"` +
			profiledetector.PolicyVersion + `"`,
	} {
		if !strings.Contains(workInputCanonical, expected) {
			t.Fatalf(
				"manual durable input omits %s: %s",
				expected,
				workInputCanonical,
			)
		}
	}
	if revalidationCount == 0 {
		t.Fatal("manual declaration skipped ledger revalidation")
	}
}

func TestExplicitProfileAuthorityExactReuseAfterDatabaseRestart(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "authority-restart.db")
	store, err := kerneldb.NewStore(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	root := canonicalWorkInputTestRoot(t)
	database := store.GetRawDB()
	database.SetMaxOpenConns(1)
	insertProfileOnboardingTestLedgerBinding(
		t,
		database,
		root,
		time.Now().UTC().Round(0).Add(-time.Minute),
	)
	suggestion := workInputTestSuggestion(t, root, []string{"go.mod", "main.go"})
	proposal, err := ProposeProfileOnboardingWorkInput(suggestion)
	if err != nil {
		t.Fatal(err)
	}
	input, err := DecodeProfileOnboardingWorkInput(proposal, suggestion)
	if err != nil {
		t.Fatal(err)
	}
	policy, err := NewProfileDeclarationPolicy(
		ProfileDeclarationModeExplicitHOnboard,
		".haft/config.yaml",
		[]byte("profile_declaration_mode: explicit_h_onboard"),
	)
	if err != nil {
		t.Fatal(err)
	}
	firstTime := time.Now().UTC().Round(0)
	revalidationCount := 0
	firstOutcome, err := profiledeclarationpreparationsqlite.PrepareBeforeAdmission(
		context.Background(),
		database,
		root,
		input,
		policy,
		profileDeclarationTestSequenceClock(
			firstTime,
			firstTime.Add(time.Microsecond),
			firstTime.Add(2*time.Microsecond),
			firstTime.Add(3*time.Microsecond),
			firstTime.Add(4*time.Microsecond),
		),
		profileDeclarationTestRevalidator(database, &revalidationCount),
	)
	if err != nil {
		t.Fatal(err)
	}
	first, ok := firstOutcome.Prepared()
	if !ok {
		t.Fatalf("first preparation = %q", firstOutcome.Kind())
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	restarted, err := kerneldb.NewStore(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = restarted.Close() })
	database = restarted.GetRawDB()
	database.SetMaxOpenConns(1)
	secondOutcome, err := profiledeclarationpreparationsqlite.PrepareBeforeAdmission(
		context.Background(),
		database,
		root,
		input,
		policy,
		profileDeclarationTestSequenceClock(firstTime.Add(10*time.Minute)),
		profileDeclarationTestRevalidator(database, &revalidationCount),
	)
	if err != nil {
		t.Fatal(err)
	}
	second, ok := secondOutcome.Prepared()
	if !ok {
		t.Fatalf("second preparation = %q", secondOutcome.Kind())
	}
	firstBasisRef, firstBasisDigest, firstBasisOK := first.AuthorityBasis()
	secondBasisRef, secondBasisDigest, secondBasisOK := second.AuthorityBasis()
	firstResolutionRef, firstResolutionDigest, firstResolutionOK := first.AuthorityResolution()
	secondResolutionRef, secondResolutionDigest, secondResolutionOK := second.AuthorityResolution()
	if !firstBasisOK || !secondBasisOK || !firstResolutionOK || !secondResolutionOK {
		t.Fatal("restart preparation omitted authority provenance")
	}
	if firstBasisRef != secondBasisRef ||
		firstBasisDigest != secondBasisDigest ||
		firstResolutionRef != secondResolutionRef ||
		firstResolutionDigest != secondResolutionDigest {
		t.Fatal("restart produced another authority closure")
	}
	if firstOutcome.Kind() != profiledeclarationpreparationsqlite.OutcomePreparedNew ||
		secondOutcome.Kind() != profiledeclarationpreparationsqlite.OutcomeExactExisting {
		t.Fatalf(
			"restart preparation kinds = %q / %q",
			firstOutcome.Kind(),
			secondOutcome.Kind(),
		)
	}
	assertProfileAuthorityTableCounts(
		t,
		database,
		[]string{
			"profile_onboarding_work_inputs_v1",
			"profile_declaration_authority_bases_v3",
			"profile_declaration_authority_resolutions_v3",
		},
		1,
	)
}

func TestStrictProfileDeclarationFailsBeforeAnySealedAuthorityWrite(t *testing.T) {
	database := openProfileAuthoritySourceTestDatabase(t, "strict-fail-closed")
	database.SetMaxOpenConns(1)
	root := canonicalWorkInputTestRoot(t)
	insertProfileOnboardingTestLedgerBinding(
		t,
		database,
		root,
		time.Now().UTC().Round(0).Add(-time.Minute),
	)
	suggestion := workInputTestSuggestion(t, root, []string{"go.mod", "main.go"})
	proposal, err := ProposeProfileOnboardingWorkInput(suggestion)
	if err != nil {
		t.Fatal(err)
	}
	input, err := DecodeProfileOnboardingWorkInput(proposal, suggestion)
	if err != nil {
		t.Fatal(err)
	}
	policy, err := NewProfileDeclarationPolicy(
		ProfileDeclarationModeStrictSpeechAct,
		"",
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	revalidationCount := 0
	service, err := newService(
		database,
		profileDeclarationTestRevalidator(database, &revalidationCount),
	)
	if err != nil {
		t.Fatal(err)
	}
	result := service.DeclareProfile(
		context.Background(),
		root,
		input,
		policy,
	)
	if result.Kind() != ResultFailed {
		t.Fatalf("strict declaration result = %q", result.Kind())
	}
	failure, ok := result.Failure()
	if !ok || failure.Code() != "strict_profile_authority_not_available" {
		t.Fatalf("strict declaration failure = %#v", failure)
	}
	if revalidationCount != 0 {
		t.Fatalf("strict unavailable path crossed write orchestration %d times", revalidationCount)
	}
	assertProfileAuthoritySourceCounts(t, database, 0)
	assertProfileAuthorityClosureCounts(t, database, 0)
	assertProfileAuthorityTableCounts(
		t,
		database,
		[]string{
			"profile_onboarding_work_inputs_v1",
			"profile_declaration_authority_bases_v3",
			"profile_declaration_authority_resolutions_v3",
		},
		0,
	)
}

func profileDeclarationTestRevalidator(
	database *sql.DB,
	count *int,
) func(context.Context) error {
	return func(ctx context.Context) error {
		transaction, err := sqlitetransaction.BeginImmediate(ctx, database)
		if err != nil {
			return err
		}
		finish := transaction.Rollback(ctx)
		if !finish.Succeeded() {
			return finish.Err()
		}
		(*count)++
		return nil
	}
}

func profileDeclarationTestSequenceClock(values ...time.Time) func() time.Time {
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
