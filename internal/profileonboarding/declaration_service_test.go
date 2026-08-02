package profileonboarding

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	kerneldb "github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/operatorrequest"
	profiledeclarationpreparationsqlite "github.com/m0n0x41d/haft/internal/profiledeclarationpreparation/sqlite"
	"github.com/m0n0x41d/haft/internal/profiledetector"
	"github.com/m0n0x41d/haft/internal/projectprofile"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/testsupport/kerneldbfixture"
)

func TestConcurrentAutomaticProfileBootstrapConvergesOnOneAdmission(
	t *testing.T,
) {
	store, err := kerneldbfixture.OpenCurrentStore(
		filepath.Join(t.TempDir(), "automatic-profile-race.db"),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	database := store.GetRawDB()
	database.SetMaxOpenConns(4)
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
		[]string{"go.mod", "internal/kernel.go"},
	)
	var revalidationCount atomic.Int64
	revalidate := func(ctx context.Context) error {
		transaction, err := sqlitetransaction.BeginImmediate(ctx, database)
		if err != nil {
			return err
		}
		finish := transaction.Rollback(ctx)
		if !finish.Succeeded() {
			return finish.Err()
		}
		revalidationCount.Add(1)
		return nil
	}
	type raceResult struct {
		result Result
		err    error
	}
	results := make(chan raceResult, 2)
	start := make(chan struct{})
	var workers sync.WaitGroup
	workers.Add(2)
	for range 2 {
		go func() {
			defer workers.Done()
			<-start
			result, err := RunAutomaticInitialProfileBootstrap(
				context.Background(),
				database,
				root,
				suggestion,
				revalidate,
			)
			results <- raceResult{result: result, err: err}
		}()
	}
	close(start)
	workers.Wait()
	close(results)
	digests := []string{}
	for outcome := range results {
		if outcome.err != nil {
			t.Fatalf("automatic bootstrap race error: %v", outcome.err)
		}
		admission, ok := outcome.result.Admission()
		if outcome.result.Kind() != ResultSynchronized || !ok {
			failure, _ := outcome.result.Failure()
			t.Fatalf(
				"automatic bootstrap race result=%q failure=%s",
				outcome.result.Kind(),
				failure.Detail(),
			)
		}
		digests = append(digests, admission.AdmissionRecordDigest().String())
	}
	if len(digests) != 2 || digests[0] != digests[1] {
		t.Fatalf("race admission digests = %#v", digests)
	}
	var admissionCount int
	if err := database.QueryRow(
		"SELECT COUNT(*) FROM project_profile_admissions_v4",
	).Scan(&admissionCount); err != nil {
		t.Fatal(err)
	}
	if admissionCount != 1 || revalidationCount.Load() == 0 {
		t.Fatalf(
			"race admissions=%d revalidations=%d",
			admissionCount,
			revalidationCount.Load(),
		)
	}
}

func TestAutomaticProfileBootstrapIsReplaySafeAndExplicitOnboardMayOverride(
	t *testing.T,
) {
	store, err := kerneldbfixture.OpenCurrentStore(
		filepath.Join(t.TempDir(), "automatic-profile-bootstrap.db"),
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
		[]string{"go.mod", "internal/kernel.go"},
	)
	revalidationCount := 0
	revalidate := profileDeclarationTestRevalidator(
		database,
		&revalidationCount,
	)
	first, err := RunAutomaticInitialProfileBootstrap(
		context.Background(),
		database,
		root,
		suggestion,
		revalidate,
	)
	if err != nil {
		t.Fatal(err)
	}
	firstAdmission, ok := first.Admission()
	if first.Kind() != ResultSynchronized || !ok ||
		firstAdmission.Origin() !=
			projectprofile.ProfileAdmissionOriginDetectorDefault ||
		firstAdmission.LedgerRevision().Value() != 1 {
		t.Fatalf("automatic admission = %q %#v", first.Kind(), firstAdmission)
	}
	replayed, err := RunAutomaticInitialProfileBootstrap(
		context.Background(),
		database,
		root,
		suggestion,
		revalidate,
	)
	if err != nil {
		t.Fatal(err)
	}
	replayedAdmission, ok := replayed.Admission()
	if replayed.Kind() != ResultSynchronized || !ok ||
		replayedAdmission.AdmissionRecordDigest() !=
			firstAdmission.AdmissionRecordDigest() {
		t.Fatalf("automatic replay = %q %#v", replayed.Kind(), replayedAdmission)
	}
	manualJSON, err := ProposeProfileOnboardingWorkInput(suggestion)
	if err != nil {
		t.Fatal(err)
	}
	manualInput, err := DecodeProfileOnboardingWorkInput(
		manualJSON,
		suggestion,
	)
	if err != nil {
		t.Fatal(err)
	}
	explicitPolicy, err := newHostRoutedProfileDeclarationTestPolicy(manualInput)
	if err != nil {
		t.Fatal(err)
	}
	overridden, err := RunProfileDeclaration(
		context.Background(),
		database,
		root,
		manualInput,
		explicitPolicy,
		revalidate,
	)
	if err != nil {
		t.Fatal(err)
	}
	overriddenAdmission, ok := overridden.Admission()
	if overridden.Kind() != ResultSynchronized || !ok ||
		overriddenAdmission.Origin() !=
			projectprofile.ProfileAdmissionOriginHostRoutedOperatorRequest ||
		overriddenAdmission.LedgerRevision().Value() != 2 {
		failure, _ := overridden.Failure()
		rejections, _ := overridden.Rejections()
		t.Fatalf(
			"explicit override = %q %#v failure=%s rejections=%#v",
			overridden.Kind(),
			overriddenAdmission,
			failure.Detail(),
			rejections,
		)
	}
	differentSuggestion := workInputTestSuggestion(
		t,
		root,
		[]string{"notes/one.md", "notes/two.mdx", "notes/three.rst"},
	)
	automaticAfterExplicit, err := RunAutomaticInitialProfileBootstrap(
		context.Background(),
		database,
		root,
		differentSuggestion,
		revalidate,
	)
	if err != nil {
		t.Fatal(err)
	}
	if automaticAfterExplicit.Kind() != ResultNotAdmitted {
		t.Fatalf("automatic override of explicit profile = %q", automaticAfterExplicit.Kind())
	}
}

func TestRunProfileDeclarationExplicitPolicyAdmitsAndReplaysAfterRestart(
	t *testing.T,
) {
	databasePath := filepath.Join(t.TempDir(), "generic-declaration.db")
	store, err := kerneldbfixture.OpenCurrentStore(databasePath)
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
	policy, err := newHostRoutedProfileDeclarationTestPolicy(input)
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
	store, err := kerneldbfixture.OpenCurrentStore(
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
	policy, err := newHostRoutedProfileDeclarationTestPolicy(input)
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
		 FROM profile_declaration_authority_bases_v5 basis
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

func TestHostRoutedProfileAuthorityExactReuseAfterDatabaseRestart(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "authority-restart.db")
	store, err := kerneldbfixture.OpenCurrentStore(databasePath)
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
	policy, err := newHostRoutedProfileDeclarationTestPolicy(input)
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
			"profile_declaration_authority_bases_v5",
			"profile_declaration_authority_resolutions_v5",
		},
		1,
	)
}

func TestMismatchedOperatorRequestFailsBeforeAnySealedAuthorityWrite(t *testing.T) {
	database := openProfileAuthoritySourceTestDatabase(t, "request-mismatch")
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
	request, err := operatorrequest.New(
		operatorrequest.ProfileDeclaration,
		input.Ref().String(),
		[]byte("another reviewed WorkInput"),
	)
	if err != nil {
		t.Fatal(err)
	}
	policy, err := NewProfileDeclarationPolicy(request)
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
		t.Fatalf("mismatched declaration result = %q", result.Kind())
	}
	failure, ok := result.Failure()
	if !ok || !strings.Contains(failure.Detail(), "does not bind the exact reviewed WorkInput") {
		t.Fatalf("mismatched declaration failure = %#v", failure)
	}
	assertProfileAuthoritySourceCounts(t, database, 0)
	assertProfileAuthorityClosureCounts(t, database, 0)
	assertProfileAuthorityTableCounts(
		t,
		database,
		[]string{
			"profile_onboarding_work_inputs_v1",
			"profile_declaration_authority_bases_v5",
			"profile_declaration_authority_resolutions_v5",
		},
		0,
	)
}

func newHostRoutedProfileDeclarationTestPolicy(
	input ProfileOnboardingWorkInput,
) (ProfileDeclarationPolicy, error) {
	request, err := operatorrequest.New(
		operatorrequest.ProfileDeclaration,
		input.Ref().String(),
		input.CanonicalJSON(),
	)
	if err != nil {
		return ProfileDeclarationPolicy{}, err
	}
	return NewProfileDeclarationPolicy(request)
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
