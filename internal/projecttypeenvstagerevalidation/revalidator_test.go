package projecttypeenvstagerevalidation_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv"
	"github.com/m0n0x41d/haft/internal/projectgraphobservation"
	"github.com/m0n0x41d/haft/internal/projectprofile"
	"github.com/m0n0x41d/haft/internal/projecttypeenvassertionrevalidation"
	"github.com/m0n0x41d/haft/internal/projecttypeenvprofilebasis"
	"github.com/m0n0x41d/haft/internal/projecttypeenvprofilefit"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/projecttypeenvstagerevalidation"
	"github.com/m0n0x41d/haft/internal/projecttypeenvtransitioncompatibility"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestCurrentSelectionStageZeroIsInvalidAndNonSerializable(t *testing.T) {
	var current projecttypeenvstagerevalidation.CurrentSelectionStage
	if current.Valid() {
		t.Fatal("zero CurrentSelectionStage is valid")
	}
	if _, exists := current.Stage(); exists {
		t.Fatal("zero CurrentSelectionStage exposes a Stage")
	}
	if _, err := json.Marshal(current); !errors.Is(
		err,
		projecttypeenvstagerevalidation.ErrCurrentSelectionStageNotSerializable,
	) {
		t.Fatalf("json.Marshal error = %v", err)
	}
	if err := json.Unmarshal([]byte(`{}`), &current); !errors.Is(
		err,
		projecttypeenvstagerevalidation.ErrCurrentSelectionStageNotSerializable,
	) {
		t.Fatalf("json.Unmarshal error = %v", err)
	}
}

func TestExactGenesisWithoutCanonicalProfileIsSelectable(t *testing.T) {
	fixtures := targetFixtures(t)
	project := testProject(t, "qnt_11111111")
	draft := genesisStage(t, project, fixtures.alpha)
	currentProfile := noCanonicalProfileForStage(t, draft)
	stage := stageWithCurrentProfile(
		t,
		draft,
		fixtures.alpha,
		currentProfile,
	)
	absent, err := projecttypeenvstagerevalidation.NewObservedNoProjectTypeEnvHead(
		project,
	)
	if err != nil {
		t.Fatalf("NewObservedNoProjectTypeEnvHead(): %v", err)
	}
	result := projecttypeenvstagerevalidation.Revalidate(
		projecttypeenvstagerevalidation.ProjectTypeEnvStageRevalidationInput{
			Stage:                 stage,
			FinalVerification:     fixtures.alpha.verification,
			ExecutableTarget:      fixtures.alpha.snapshot,
			TargetRuntimeRegistry: exactRuntimeRegistryForTarget(t, fixtures.alpha),
			CurrentGraph:          graphObservationForStage(t, stage, fixtures.alpha),
			CurrentProfile:        currentProfile,
			CurrentHead:           absent,
		},
	)
	current, ok := result.(projecttypeenvstagerevalidation.CurrentSelectionStage)
	if !ok {
		t.Fatalf(
			"result = %T issues=%#v, want CurrentSelectionStage",
			result,
			resultIssueCodes(result),
		)
	}
	if !current.Valid() {
		t.Fatal("profile-less Genesis did not mint a valid current Stage")
	}
	if _, present := current.Stage(); !present {
		t.Fatal("profile-less Genesis current Stage is absent")
	}
}

func TestExactCompatibleGenesisMintsNonSerializableCurrentSelectionStage(
	t *testing.T,
) {
	fixtures := targetFixtures(t)
	project := testProject(t, "qnt_19191919")
	draft := genesisStage(t, project, fixtures.alpha)
	currentProfile := declaredSoftwareProfileForStage(t, draft, "a")
	stage := stageWithCurrentProfile(
		t,
		draft,
		fixtures.alpha,
		currentProfile,
	)
	absent, err := projecttypeenvstagerevalidation.NewObservedNoProjectTypeEnvHead(
		project,
	)
	if err != nil {
		t.Fatalf("NewObservedNoProjectTypeEnvHead(): %v", err)
	}

	result := revalidateFixtureWithProfile(
		t,
		stage,
		fixtures.alpha,
		absent,
		currentProfile,
	)
	current, ok := result.(projecttypeenvstagerevalidation.CurrentSelectionStage)
	if !ok {
		t.Fatalf(
			"result = %T issues=%#v, want CurrentSelectionStage",
			result,
			resultIssueCodes(result),
		)
	}
	if !current.Valid() {
		t.Fatal("minted CurrentSelectionStage is invalid")
	}
	profileBasis, exists := current.ProfileBasis()
	if !exists {
		t.Fatal("CurrentSelectionStage did not expose its exact current profile basis")
	}
	if profileBasis.ProfileBasisRef() != currentProfile.ProfileBasisRef() ||
		profileBasis.Digest() != currentProfile.Digest() ||
		profileBasis.LedgerRevision() != stage.ProfileLedgerRevision() ||
		profileBasis.ProfileLedgerDigest() != stage.ProfileLedgerDigest() {
		t.Fatal("CurrentSelectionStage did not cross-bind Stage to the exact profile basis")
	}
	currentStage, exists := current.Stage()
	if !exists || currentStage.Ref() != stage.Ref() {
		t.Fatal("CurrentSelectionStage did not retain the exact immutable Stage")
	}
	assertions, exists := current.AssertionRevalidation()
	if !exists ||
		assertions.Digest() != stage.ExistingAssertionRevalidation().Digest() ||
		!bytes.Equal(
			assertions.CanonicalBytes(),
			stage.ExistingAssertionRevalidation().CanonicalBytes(),
		) {
		t.Fatal("CurrentSelectionStage did not retain the exact current assertion report")
	}
	profile, exists := current.ProfileAssessment()
	if !exists {
		t.Fatal("CurrentSelectionStage did not expose its exact profile assessment")
	}
	if _, compatible := profile.(projecttypeenvprofilefit.Compatible); !compatible {
		t.Fatalf("profile assessment = %T, want Compatible", profile)
	}
	if profile.BasisRef() != profileBasis.ProfileBasisRef() ||
		profile.BasisDigest() != profileBasis.Digest() {
		t.Fatal("profile assessment is not bound to the retained profile basis")
	}
	if profile.Digest() != stage.ProfileCompatibility().Digest() ||
		!bytes.Equal(
			profile.CanonicalBytes(),
			stage.ProfileCompatibility().CanonicalBytes(),
		) {
		t.Fatal("CurrentSelectionStage did not retain the exact current profile assessment")
	}
	if _, err := json.Marshal(current); !errors.Is(
		err,
		projecttypeenvstagerevalidation.ErrCurrentSelectionStageNotSerializable,
	) {
		t.Fatalf("json.Marshal(minted capability) error = %v", err)
	}
	var decoded projecttypeenvstagerevalidation.CurrentSelectionStage
	if err := json.Unmarshal([]byte(`{}`), &decoded); !errors.Is(
		err,
		projecttypeenvstagerevalidation.ErrCurrentSelectionStageNotSerializable,
	) {
		t.Fatalf("json.Unmarshal(minted capability target) error = %v", err)
	}
	if decoded.Valid() {
		t.Fatal("JSON created a CurrentSelectionStage capability")
	}
}

func TestCurrentProfileLedgerCoordinatesAreExactDriftGates(t *testing.T) {
	fixtures := targetFixtures(t)
	project := testProject(t, "qnt_18181818")
	draft := genesisStage(t, project, fixtures.alpha)
	currentProfile := declaredSoftwareProfileForStage(t, draft, "1")
	exact := stageWithCurrentProfile(
		t,
		draft,
		fixtures.alpha,
		currentProfile,
	)
	absent, err := projecttypeenvstagerevalidation.NewObservedNoProjectTypeEnvHead(
		project,
	)
	if err != nil {
		t.Fatalf("NewObservedNoProjectTypeEnvHead(): %v", err)
	}

	t.Run("revision", func(t *testing.T) {
		changedRevision := projectprofile.NewLedgerRevision(
			currentProfile.LedgerRevision().Value() + 1,
		)
		changed := stageWithProfileLedgerCoordinates(
			t,
			exact,
			fixtures.alpha,
			changedRevision,
			currentProfile.ProfileLedgerDigest(),
		)
		result := revalidateFixtureWithProfile(
			t,
			changed,
			fixtures.alpha,
			absent,
			currentProfile,
		)
		want := []projecttypeenvstagerevalidation.StageRevalidationIssueCode{
			projecttypeenvstagerevalidation.IssueProfileLedgerRevisionMismatch,
		}
		assertDriftCodes(t, result, want)
	})

	t.Run("digest", func(t *testing.T) {
		changedDigest := testDigest(t, "e")
		if changedDigest == currentProfile.ProfileLedgerDigest() {
			t.Fatal("digest fixture did not change the profile-ledger coordinate")
		}
		changed := stageWithProfileLedgerCoordinates(
			t,
			exact,
			fixtures.alpha,
			currentProfile.LedgerRevision(),
			changedDigest,
		)
		result := revalidateFixtureWithProfile(
			t,
			changed,
			fixtures.alpha,
			absent,
			currentProfile,
		)
		want := []projecttypeenvstagerevalidation.StageRevalidationIssueCode{
			projecttypeenvstagerevalidation.IssueProfileLedgerDigestMismatch,
		}
		assertDriftCodes(t, result, want)
	})
}

func TestCurrentProfileAssessmentIdentityMismatchIsDrift(t *testing.T) {
	fixtures := targetFixtures(t)
	project := testProject(t, "qnt_28282828")
	draft := genesisStage(t, project, fixtures.alpha)
	stagedProfile := declaredSoftwareProfileForStage(t, draft, "2")
	currentProfile := declaredSoftwareProfileForStage(t, draft, "3")
	exactForStagedProfile := stageWithCurrentProfile(
		t,
		draft,
		fixtures.alpha,
		stagedProfile,
	)
	stageWithCurrentLedger := stageWithProfileLedgerCoordinates(
		t,
		exactForStagedProfile,
		fixtures.alpha,
		currentProfile.LedgerRevision(),
		currentProfile.ProfileLedgerDigest(),
	)
	absent, err := projecttypeenvstagerevalidation.NewObservedNoProjectTypeEnvHead(
		project,
	)
	if err != nil {
		t.Fatalf("NewObservedNoProjectTypeEnvHead(): %v", err)
	}
	result := revalidateFixtureWithProfile(
		t,
		stageWithCurrentLedger,
		fixtures.alpha,
		absent,
		currentProfile,
	)
	want := []projecttypeenvstagerevalidation.StageRevalidationIssueCode{
		projecttypeenvstagerevalidation.IssueProfileFitMismatch,
	}
	assertDriftCodes(t, result, want)
}

func TestExactIncompatibleProfileIsRejectedNotDrifted(t *testing.T) {
	fixtures := targetFixtures(t)
	project := testProject(t, "qnt_38383838")
	draft := genesisStage(t, project, fixtures.alpha)
	currentProfile := declaredIncompatibleProfileForStage(t, draft, "4")
	stage := stageWithCurrentProfile(
		t,
		draft,
		fixtures.alpha,
		currentProfile,
	)
	absent, err := projecttypeenvstagerevalidation.NewObservedNoProjectTypeEnvHead(
		project,
	)
	if err != nil {
		t.Fatalf("NewObservedNoProjectTypeEnvHead(): %v", err)
	}
	result := revalidateFixtureWithProfile(
		t,
		stage,
		fixtures.alpha,
		absent,
		currentProfile,
	)
	rejected, ok := result.(projecttypeenvstagerevalidation.RejectedSelectionStage)
	if !ok {
		t.Fatalf(
			"result = %T issues=%#v, want RejectedSelectionStage",
			result,
			resultIssueCodes(result),
		)
	}
	want := []projecttypeenvstagerevalidation.StageRevalidationIssueCode{
		projecttypeenvstagerevalidation.IssueProfileIncompatible,
	}
	if got := issueCodes(rejected.Issues()); !reflect.DeepEqual(got, want) {
		t.Fatalf("issue codes = %#v, want %#v", got, want)
	}
}

func TestMissingExactTargetRuntimeRemainsExplicitlyUnavailable(t *testing.T) {
	fixtures := targetFixtures(t)
	project := testProject(t, "qnt_10101010")
	stage := genesisStage(t, project, fixtures.alpha)
	absent, err := projecttypeenvstagerevalidation.NewObservedNoProjectTypeEnvHead(
		project,
	)
	if err != nil {
		t.Fatalf("NewObservedNoProjectTypeEnvHead(): %v", err)
	}
	result := projecttypeenvstagerevalidation.Revalidate(
		projecttypeenvstagerevalidation.ProjectTypeEnvStageRevalidationInput{
			Stage:             stage,
			FinalVerification: fixtures.alpha.verification,
			ExecutableTarget:  fixtures.alpha.snapshot,
			CurrentGraph:      graphObservationForStage(t, stage, fixtures.alpha),
			CurrentProfile:    noCanonicalProfileForStage(t, stage),
			CurrentHead:       absent,
		},
	)
	unavailable, ok := result.(projecttypeenvstagerevalidation.UnavailableSelectionStage)
	if !ok {
		t.Fatalf("result = %T, want UnavailableSelectionStage", result)
	}
	requirements := unavailable.Requirements()
	if !containsRequirement(
		requirements,
		projecttypeenvstagerevalidation.RequirementTargetRuntimeRegistry,
	) {
		t.Fatalf("requirements = %#v, missing target runtime registry", requirements)
	}
	if containsRequirement(
		requirements,
		projecttypeenvstagerevalidation.RequirementTrustedStageEditions,
	) {
		t.Fatalf("requirements = %#v, retained package-owned trusted-edition derivation", requirements)
	}
}

func TestTypedNilCurrentProfileIsInvalidWithoutPanic(t *testing.T) {
	fixtures := targetFixtures(t)
	project := testProject(t, "qnt_17171717")
	stage := genesisStage(t, project, fixtures.alpha)
	absent, err := projecttypeenvstagerevalidation.NewObservedNoProjectTypeEnvHead(
		project,
	)
	if err != nil {
		t.Fatalf("NewObservedNoProjectTypeEnvHead(): %v", err)
	}
	var typedNil *projecttypeenvprofilebasis.NoCanonicalProjectProfile
	result := projecttypeenvstagerevalidation.Revalidate(
		projecttypeenvstagerevalidation.ProjectTypeEnvStageRevalidationInput{
			Stage:                 stage,
			FinalVerification:     fixtures.alpha.verification,
			ExecutableTarget:      fixtures.alpha.snapshot,
			TargetRuntimeRegistry: exactRuntimeRegistryForTarget(t, fixtures.alpha),
			CurrentGraph:          graphObservationForStage(t, stage, fixtures.alpha),
			CurrentProfile:        typedNil,
			CurrentHead:           absent,
		},
	)
	invalid, ok := result.(projecttypeenvstagerevalidation.InvalidSelectionStage)
	if !ok {
		t.Fatalf("result = %T, want InvalidSelectionStage", result)
	}
	want := []projecttypeenvstagerevalidation.StageRevalidationIssueCode{
		projecttypeenvstagerevalidation.IssueProjectProfileBasisInvalid,
	}
	if got := issueCodes(invalid.Issues()); !reflect.DeepEqual(got, want) {
		t.Fatalf("issue codes = %#v, want %#v", got, want)
	}
}

func TestExactRuntimeRegistryForAnotherTargetCIsDrifted(t *testing.T) {
	fixtures := targetFixtures(t)
	project := testProject(t, "qnt_20202020")
	stage := genesisStage(t, project, fixtures.alpha)
	absent, err := projecttypeenvstagerevalidation.NewObservedNoProjectTypeEnvHead(
		project,
	)
	if err != nil {
		t.Fatalf("NewObservedNoProjectTypeEnvHead(): %v", err)
	}
	result := projecttypeenvstagerevalidation.Revalidate(
		projecttypeenvstagerevalidation.ProjectTypeEnvStageRevalidationInput{
			Stage:                 stage,
			FinalVerification:     fixtures.alpha.verification,
			ExecutableTarget:      fixtures.alpha.snapshot,
			TargetRuntimeRegistry: exactRuntimeRegistryForTarget(t, fixtures.beta),
			CurrentGraph:          graphObservationForStage(t, stage, fixtures.alpha),
			CurrentProfile:        noCanonicalProfileForStage(t, stage),
			CurrentHead:           absent,
		},
	)
	assertDriftCode(
		t,
		result,
		projecttypeenvstagerevalidation.IssueTargetRuntimeBasisMismatch,
	)
}

func TestUnsupportedTrustedStageEditionsAreDriftedWithExactCodes(t *testing.T) {
	fixtures := targetFixtures(t)
	project := testProject(t, "qnt_30303030")
	stage := genesisStage(t, project, fixtures.alpha)
	stage = stageWithUnsupportedEditions(t, stage)
	absent, err := projecttypeenvstagerevalidation.NewObservedNoProjectTypeEnvHead(
		project,
	)
	if err != nil {
		t.Fatalf("NewObservedNoProjectTypeEnvHead(): %v", err)
	}
	result := projecttypeenvstagerevalidation.Revalidate(
		projecttypeenvstagerevalidation.ProjectTypeEnvStageRevalidationInput{
			Stage:                 stage,
			FinalVerification:     fixtures.alpha.verification,
			ExecutableTarget:      fixtures.alpha.snapshot,
			TargetRuntimeRegistry: exactRuntimeRegistryForTarget(t, fixtures.alpha),
			CurrentGraph:          graphObservationForStage(t, stage, fixtures.alpha),
			CurrentProfile:        noCanonicalProfileForStage(t, stage),
			CurrentHead:           absent,
		},
	)
	drifted, ok := result.(projecttypeenvstagerevalidation.DriftedSelectionStage)
	if !ok {
		t.Fatalf("result = %T, want DriftedSelectionStage", result)
	}
	want := []projecttypeenvstagerevalidation.StageRevalidationIssueCode{
		projecttypeenvstagerevalidation.IssueStageCompilerUnsupported,
		projecttypeenvstagerevalidation.IssueStageProducerUnsupported,
		projecttypeenvstagerevalidation.IssueStageRevalidatorUnsupported,
	}
	if got := issueCodes(drifted.Issues()); !reflect.DeepEqual(got, want) {
		t.Fatalf("issue codes = %#v, want %#v", got, want)
	}
}

func TestTargetClosureCrossBindingIsRejected(t *testing.T) {
	fixtures := targetFixtures(t)
	project := testProject(t, "qnt_22222222")
	stage := genesisStage(t, project, fixtures.alpha)
	absent, err := projecttypeenvstagerevalidation.NewObservedNoProjectTypeEnvHead(
		project,
	)
	if err != nil {
		t.Fatalf("NewObservedNoProjectTypeEnvHead(): %v", err)
	}
	result := projecttypeenvstagerevalidation.Revalidate(
		projecttypeenvstagerevalidation.ProjectTypeEnvStageRevalidationInput{
			Stage:                 stage,
			FinalVerification:     fixtures.beta.verification,
			ExecutableTarget:      fixtures.beta.snapshot,
			TargetRuntimeRegistry: exactRuntimeRegistryForTarget(t, fixtures.beta),
			CurrentGraph:          graphObservationForStage(t, stage, fixtures.alpha),
			CurrentProfile:        noCanonicalProfileForStage(t, stage),
			CurrentHead:           absent,
		},
	)
	drifted, ok := result.(projecttypeenvstagerevalidation.DriftedSelectionStage)
	if !ok {
		t.Fatalf("result = %T, want DriftedSelectionStage", result)
	}
	codes := issueCodes(drifted.Issues())
	required := []projecttypeenvstagerevalidation.StageRevalidationIssueCode{
		projecttypeenvstagerevalidation.IssueRuntimeBasisMismatch,
		projecttypeenvstagerevalidation.IssueCompositeMismatch,
		projecttypeenvstagerevalidation.IssueFinalVerificationMismatch,
		projecttypeenvstagerevalidation.IssueExecutableSnapshotMismatch,
	}
	for _, code := range required {
		if !containsIssueCode(codes, code) {
			t.Fatalf("issue codes = %#v, missing %q", codes, code)
		}
	}
}

func TestCurrentGraphDriftIsSeparateFromTargetAndHead(t *testing.T) {
	fixtures := targetFixtures(t)
	project := testProject(t, "qnt_12121212")
	otherProject := testProject(t, "qnt_34343434")
	stage := genesisStage(t, project, fixtures.alpha)
	absent, err := projecttypeenvstagerevalidation.NewObservedNoProjectTypeEnvHead(
		project,
	)
	if err != nil {
		t.Fatalf("NewObservedNoProjectTypeEnvHead: %v", err)
	}
	otherBasis, err := projecttypeenvselection.SealProjectGraphSnapshotBasis(
		projecttypeenvselection.ProjectGraphSnapshotBasisInput{
			Project:       otherProject,
			GraphRevision: typedmemory.NewGraphRevision(0),
			Closure:       projecttypeenvselection.EmptyProjectGraphClosure{},
		},
	)
	if err != nil {
		t.Fatalf("SealProjectGraphSnapshotBasis(other): %v", err)
	}
	otherActive, err := projectgraphobservation.NewCurrentActiveAssertionSet(
		otherProject,
		typedmemory.NewGraphRevision(0),
		nil,
	)
	if err != nil {
		t.Fatalf("NewCurrentActiveAssertionSet(other): %v", err)
	}
	otherGraph, err := projectgraphobservation.NewCurrentProjectGraphObservation(
		otherBasis,
		fixtures.alpha.snapshot.TypeEnvRef(),
		otherActive,
	)
	if err != nil {
		t.Fatalf("NewCurrentProjectGraphObservation(other): %v", err)
	}
	result := projecttypeenvstagerevalidation.Revalidate(
		projecttypeenvstagerevalidation.ProjectTypeEnvStageRevalidationInput{
			Stage:                 stage,
			FinalVerification:     fixtures.alpha.verification,
			ExecutableTarget:      fixtures.alpha.snapshot,
			TargetRuntimeRegistry: exactRuntimeRegistryForTarget(t, fixtures.alpha),
			CurrentGraph:          otherGraph,
			CurrentProfile:        noCanonicalProfileForStage(t, stage),
			CurrentHead:           absent,
		},
	)
	drifted, ok := result.(projecttypeenvstagerevalidation.DriftedSelectionStage)
	if !ok {
		t.Fatalf("result = %T; want DriftedSelectionStage", result)
	}
	codes := issueCodes(drifted.Issues())
	if !containsIssueCode(
		codes,
		projecttypeenvstagerevalidation.IssueGraphProjectMismatch,
	) || !containsIssueCode(
		codes,
		projecttypeenvstagerevalidation.IssueGraphSnapshotMismatch,
	) {
		t.Fatalf("current graph drift issue codes = %#v", codes)
	}
}

func TestCommittedGraphClosureCoordinatesArePartOfSnapshotIdentity(t *testing.T) {
	fixtures := targetFixtures(t)
	project := testProject(t, "qnt_23232323")
	draft := genesisStage(t, project, fixtures.alpha)
	currentProfile := declaredSoftwareProfileForStage(t, draft, "5")
	stagedBasis := committedGraphBasis(t, draft, "a", "b", "c")
	stagedGraph := graphObservationForBasis(
		t,
		stagedBasis,
		fixtures.alpha.snapshot.TypeEnvRef(),
	)
	stage := stageWithCurrentGraphAndProfile(
		t,
		draft,
		fixtures.alpha,
		stagedGraph,
		currentProfile,
	)
	absent, err := projecttypeenvstagerevalidation.NewObservedNoProjectTypeEnvHead(
		project,
	)
	if err != nil {
		t.Fatalf("NewObservedNoProjectTypeEnvHead(): %v", err)
	}
	baseline := revalidateFixtureWithGraphAndProfile(
		t,
		stage,
		fixtures.alpha,
		absent,
		stagedGraph,
		currentProfile,
	)
	if current, ok :=
		baseline.(projecttypeenvstagerevalidation.CurrentSelectionStage); !ok ||
		!current.Valid() {
		t.Fatalf(
			"baseline = %T issues=%#v, want valid CurrentSelectionStage",
			baseline,
			resultIssueCodes(baseline),
		)
	}

	cases := []struct {
		name                string
		eventSeed           string
		commitSeed          string
		materializationSeed string
	}{
		{name: "event", eventSeed: "d", commitSeed: "b", materializationSeed: "c"},
		{name: "commit", eventSeed: "a", commitSeed: "e", materializationSeed: "c"},
		{name: "materialization", eventSeed: "a", commitSeed: "b", materializationSeed: "f"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			changedBasis := committedGraphBasis(
				t,
				draft,
				testCase.eventSeed,
				testCase.commitSeed,
				testCase.materializationSeed,
			)
			if changedBasis.Ref() == stagedBasis.Ref() {
				t.Fatal("committed closure mutation did not change aggregate basis identity")
			}
			changedGraph := graphObservationForBasis(
				t,
				changedBasis,
				fixtures.alpha.snapshot.TypeEnvRef(),
			)
			result := revalidateFixtureWithGraphAndProfile(
				t,
				stage,
				fixtures.alpha,
				absent,
				changedGraph,
				currentProfile,
			)
			want := []projecttypeenvstagerevalidation.StageRevalidationIssueCode{
				projecttypeenvstagerevalidation.IssueGraphSnapshotMismatch,
			}
			assertDriftCodes(t, result, want)
		})
	}
}

func TestPredecessorVariants(t *testing.T) {
	fixtures := targetFixtures(t)
	project := testProject(t, "qnt_33333333")
	otherProject := testProject(t, "qnt_44444444")
	genesis := genesisStage(t, project, fixtures.alpha)
	absent, err := projecttypeenvstagerevalidation.NewObservedNoProjectTypeEnvHead(
		project,
	)
	if err != nil {
		t.Fatalf("NewObservedNoProjectTypeEnvHead(): %v", err)
	}
	prior := headState(t, project, fixtures.alpha.snapshot.TypeEnvRef(), 3)
	present, err := projecttypeenvstagerevalidation.NewObservedProjectTypeEnvHead(prior)
	if err != nil {
		t.Fatalf("NewObservedProjectTypeEnvHead(): %v", err)
	}

	t.Run("Genesis rejects present head", func(t *testing.T) {
		result := revalidateFixture(t, genesis, fixtures.alpha, present)
		assertDriftCode(
			t,
			result,
			projecttypeenvstagerevalidation.IssueHeadPresenceMismatch,
		)
	})

	t.Run("Transition remains unavailable without prior executable C", func(t *testing.T) {
		draft := transitionStageFromExecutable(
			t,
			project,
			fixtures.beta,
			prior,
			fixtures.alpha.snapshot,
		)
		currentProfile := noCanonicalProfileForStage(t, draft)
		transition := stageWithCurrentProfile(
			t,
			draft,
			fixtures.beta,
			currentProfile,
		)
		result := revalidateFixtureWithProfile(
			t,
			transition,
			fixtures.beta,
			present,
			currentProfile,
		)
		unavailable, ok := result.(projecttypeenvstagerevalidation.UnavailableSelectionStage)
		if !ok {
			t.Fatalf(
				"result = %T issues=%#v, want UnavailableSelectionStage",
				result,
				resultIssueCodes(result),
			)
		}
		if !containsRequirement(
			unavailable.Requirements(),
			projecttypeenvstagerevalidation.RequirementTypeEnvCompatibility,
		) {
			t.Fatalf(
				"Transition unavailable requirements = %#v, missing compatibility",
				unavailable.Requirements(),
			)
		}
	})

	t.Run("Transition keeps unchanged installed profile roles compatible", func(t *testing.T) {
		draft := transitionStageFromExecutable(
			t,
			project,
			fixtures.beta,
			prior,
			fixtures.alpha.snapshot,
		)
		currentProfile := declaredSoftwareProfileForStage(t, draft, "c")
		transition := stageWithCurrentProfile(
			t,
			draft,
			fixtures.beta,
			currentProfile,
		)
		result := revalidateFixtureWithPrior(
			t,
			transition,
			fixtures.beta,
			present,
			currentProfile,
			fixtures.alpha.snapshot,
		)
		if _, ok := result.(projecttypeenvstagerevalidation.CurrentSelectionStage); !ok {
			t.Fatalf(
				"result = %T issues=%#v, want CurrentSelectionStage",
				result,
				resultIssueCodes(result),
			)
		}
	})

	t.Run("Transition rejects a different prior executable C", func(t *testing.T) {
		draft := transitionStage(t, project, fixtures.beta, prior)
		currentProfile := noCanonicalProfileForStage(t, draft)
		transition := stageWithCurrentProfile(
			t,
			draft,
			fixtures.beta,
			currentProfile,
		)
		result := revalidateFixtureWithPrior(
			t,
			transition,
			fixtures.beta,
			present,
			currentProfile,
			fixtures.beta.snapshot,
		)
		assertDriftCode(
			t,
			result,
			projecttypeenvstagerevalidation.IssuePriorExecutableSnapshotMismatch,
		)
	})

	t.Run("Transition detects installed profile compatibility drift", func(t *testing.T) {
		draft := transitionStageFromExecutable(
			t,
			project,
			fixtures.beta,
			prior,
			fixtures.alpha.snapshot,
		)
		currentProfile := noCanonicalProfileForStage(t, draft)
		currentGraph := graphObservationForStage(t, draft, fixtures.beta)
		original, exists := draft.TransitionProjectionProfileCompatibility()
		if !exists {
			t.Fatal("Transition fixture has no projection-profile compatibility")
		}
		changedCarrier := append(
			original.ProjectionProfilesCanonicalBytes(),
			0x01,
		)
		driftedArtifact, err := projecttypeenvtransitioncompatibility.New(
			original.SuccessorDiff(),
			changedCarrier,
		)
		if err != nil {
			t.Fatalf("projecttypeenvtransitioncompatibility.New(): %v", err)
		}
		transition := stageWithCurrentGraphProfileAndTransitionProfiles(
			t,
			draft,
			fixtures.beta,
			currentGraph,
			currentProfile,
			driftedArtifact,
		)
		result := revalidateFixtureWithPrior(
			t,
			transition,
			fixtures.beta,
			present,
			currentProfile,
			fixtures.alpha.snapshot,
		)
		assertDriftCode(
			t,
			result,
			projecttypeenvstagerevalidation.IssueProjectionProfileCompatibilityMismatch,
		)
	})

	t.Run("Transition rejects absent head", func(t *testing.T) {
		transition := transitionStage(t, project, fixtures.alpha, prior)
		result := revalidateFixture(t, transition, fixtures.alpha, absent)
		assertDriftCode(
			t,
			result,
			projecttypeenvstagerevalidation.IssueHeadPresenceMismatch,
		)
	})

	t.Run("Transition rejects changed exact head", func(t *testing.T) {
		transition := transitionStage(t, project, fixtures.alpha, prior)
		changed := headState(t, project, syntheticTypeEnvRef(t, "9"), 4)
		changedObservation, observationErr :=
			projecttypeenvstagerevalidation.NewObservedProjectTypeEnvHead(changed)
		if observationErr != nil {
			t.Fatalf("NewObservedProjectTypeEnvHead(changed): %v", observationErr)
		}
		result := revalidateFixture(t, transition, fixtures.alpha, changedObservation)
		drifted, ok := result.(projecttypeenvstagerevalidation.DriftedSelectionStage)
		if !ok {
			t.Fatalf("result = %T, want DriftedSelectionStage", result)
		}
		codes := issueCodes(drifted.Issues())
		if !containsIssueCode(
			codes,
			projecttypeenvstagerevalidation.IssuePriorHeadRevisionMismatch,
		) || !containsIssueCode(
			codes,
			projecttypeenvstagerevalidation.IssuePriorSelectedCompositeDrift,
		) {
			t.Fatalf("changed-head issue codes = %#v", codes)
		}
	})

	t.Run("Cross-project observation is rejected", func(t *testing.T) {
		otherAbsent, observationErr :=
			projecttypeenvstagerevalidation.NewObservedNoProjectTypeEnvHead(otherProject)
		if observationErr != nil {
			t.Fatalf("NewObservedNoProjectTypeEnvHead(other): %v", observationErr)
		}
		result := revalidateFixture(t, genesis, fixtures.alpha, otherAbsent)
		assertDriftCode(
			t,
			result,
			projecttypeenvstagerevalidation.IssueHeadProjectMismatch,
		)
	})
}

func TestIssueOrderIsDeterministic(t *testing.T) {
	fixtures := targetFixtures(t)
	project := testProject(t, "qnt_55555555")
	otherProject := testProject(t, "qnt_66666666")
	stage := genesisStage(t, project, fixtures.alpha)
	otherHead := headState(t, otherProject, syntheticTypeEnvRef(t, "a"), 7)
	observation, err :=
		projecttypeenvstagerevalidation.NewObservedProjectTypeEnvHead(otherHead)
	if err != nil {
		t.Fatalf("NewObservedProjectTypeEnvHead(): %v", err)
	}
	first := revalidateFixture(t, stage, fixtures.beta, observation)
	second := revalidateFixture(t, stage, fixtures.beta, observation)
	firstDrift, firstOK := first.(projecttypeenvstagerevalidation.DriftedSelectionStage)
	secondDrift, secondOK := second.(projecttypeenvstagerevalidation.DriftedSelectionStage)
	if !firstOK || !secondOK {
		t.Fatalf("results = %T/%T, want DriftedSelectionStage", first, second)
	}
	firstIssues := firstDrift.Issues()
	secondIssues := secondDrift.Issues()
	if !reflect.DeepEqual(firstIssues, secondIssues) {
		t.Fatalf("issue order changed across identical evaluation")
	}
	wantPrefix := []projecttypeenvstagerevalidation.StageRevalidationIssueCode{
		projecttypeenvstagerevalidation.IssueRuntimeBasisMismatch,
		projecttypeenvstagerevalidation.IssueCompositeMismatch,
		projecttypeenvstagerevalidation.IssueFinalVerificationMismatch,
	}
	got := issueCodes(firstIssues)
	if len(got) < len(wantPrefix) ||
		!reflect.DeepEqual(got[:len(wantPrefix)], wantPrefix) {
		t.Fatalf("issue order prefix = %#v, want %#v", got, wantPrefix)
	}
	if !containsIssueCode(
		got,
		projecttypeenvstagerevalidation.IssueHeadProjectMismatch,
	) || !containsIssueCode(
		got,
		projecttypeenvstagerevalidation.IssueHeadPresenceMismatch,
	) {
		t.Fatalf("issue codes = %#v, missing predecessor issues", got)
	}
}

func TestResultSlicesAreMutationIsolated(t *testing.T) {
	fixtures := targetFixtures(t)
	project := testProject(t, "qnt_77777777")
	stage := genesisStage(t, project, fixtures.alpha)
	absent, err := projecttypeenvstagerevalidation.NewObservedNoProjectTypeEnvHead(
		project,
	)
	if err != nil {
		t.Fatalf("NewObservedNoProjectTypeEnvHead(): %v", err)
	}

	unavailableResult := projecttypeenvstagerevalidation.Revalidate(
		projecttypeenvstagerevalidation.ProjectTypeEnvStageRevalidationInput{
			Stage:             stage,
			FinalVerification: fixtures.alpha.verification,
			ExecutableTarget:  fixtures.alpha.snapshot,
			CurrentGraph:      graphObservationForStage(t, stage, fixtures.alpha),
			CurrentProfile:    noCanonicalProfileForStage(t, stage),
			CurrentHead:       absent,
		},
	)
	unavailable, ok :=
		unavailableResult.(projecttypeenvstagerevalidation.UnavailableSelectionStage)
	if !ok {
		t.Fatalf(
			"result = %T issues=%#v, want UnavailableSelectionStage",
			unavailableResult,
			resultIssueCodes(unavailableResult),
		)
	}
	requirements := unavailable.Requirements()
	requirements[0] = projecttypeenvstagerevalidation.RequirementTypeEnvCompatibility
	if unavailable.Requirements()[0] !=
		projecttypeenvstagerevalidation.RequirementTargetRuntimeRegistry {
		t.Fatal("UnavailableSelectionStage exposed mutable requirement storage")
	}

	driftedResult := revalidateFixture(t, stage, fixtures.beta, absent)
	drifted := driftedResult.(projecttypeenvstagerevalidation.DriftedSelectionStage)
	issues := drifted.Issues()
	issues[0] = projecttypeenvstagerevalidation.StageRevalidationIssue{}
	if drifted.Issues()[0].Code() == "" {
		t.Fatal("DriftedSelectionStage exposed mutable issue storage")
	}

	exactProfile := declaredIncompatibleProfileForStage(t, stage, "f")
	exactStage := stageWithCurrentProfile(
		t,
		stage,
		fixtures.alpha,
		exactProfile,
	)
	rejectedResult := revalidateFixtureWithProfile(
		t,
		exactStage,
		fixtures.alpha,
		absent,
		exactProfile,
	)
	rejected, ok :=
		rejectedResult.(projecttypeenvstagerevalidation.RejectedSelectionStage)
	if !ok {
		t.Fatalf(
			"result = %T issues=%#v, want RejectedSelectionStage",
			rejectedResult,
			resultIssueCodes(rejectedResult),
		)
	}
	rejectedIssues := rejected.Issues()
	rejectedIssues[0] = projecttypeenvstagerevalidation.StageRevalidationIssue{}
	if rejected.Issues()[0].Code() == "" {
		t.Fatal("RejectedSelectionStage exposed mutable issue storage")
	}
}

func revalidateFixture(
	t *testing.T,
	stage projecttypeenvselection.ProjectTypeEnvStage,
	target targetClosureFixture,
	head projecttypeenvstagerevalidation.CurrentProjectTypeEnvHeadObservation,
) projecttypeenvstagerevalidation.StageRevalidationResult {
	t.Helper()
	return revalidateFixtureWithProfile(
		t,
		stage,
		target,
		head,
		noCanonicalProfileForStage(t, stage),
	)
}

func revalidateFixtureWithProfile(
	t *testing.T,
	stage projecttypeenvselection.ProjectTypeEnvStage,
	target targetClosureFixture,
	head projecttypeenvstagerevalidation.CurrentProjectTypeEnvHeadObservation,
	profile projecttypeenvprofilebasis.CurrentProjectProfileBasis,
) projecttypeenvstagerevalidation.StageRevalidationResult {
	t.Helper()
	graph := graphObservationForStage(t, stage, target)
	return revalidateFixtureWithGraphAndProfile(
		t,
		stage,
		target,
		head,
		graph,
		profile,
	)
}

func revalidateFixtureWithGraphAndProfile(
	t *testing.T,
	stage projecttypeenvselection.ProjectTypeEnvStage,
	target targetClosureFixture,
	head projecttypeenvstagerevalidation.CurrentProjectTypeEnvHeadObservation,
	graph projectgraphobservation.CurrentProjectGraphObservation,
	profile projecttypeenvprofilebasis.CurrentProjectProfileBasis,
) projecttypeenvstagerevalidation.StageRevalidationResult {
	t.Helper()
	return projecttypeenvstagerevalidation.Revalidate(
		projecttypeenvstagerevalidation.ProjectTypeEnvStageRevalidationInput{
			Stage:                 stage,
			FinalVerification:     target.verification,
			ExecutableTarget:      target.snapshot,
			TargetRuntimeRegistry: exactRuntimeRegistryForTarget(t, target),
			CurrentGraph:          graph,
			CurrentProfile:        profile,
			CurrentHead:           head,
		},
	)
}

func revalidateFixtureWithPrior(
	t *testing.T,
	stage projecttypeenvselection.ProjectTypeEnvStage,
	target targetClosureFixture,
	head projecttypeenvstagerevalidation.CurrentProjectTypeEnvHeadObservation,
	profile projecttypeenvprofilebasis.CurrentProjectProfileBasis,
	prior projecttypeenv.ProjectTypeEnvExecutableSnapshot,
) projecttypeenvstagerevalidation.StageRevalidationResult {
	t.Helper()
	return projecttypeenvstagerevalidation.Revalidate(
		projecttypeenvstagerevalidation.ProjectTypeEnvStageRevalidationInput{
			Stage:                 stage,
			FinalVerification:     target.verification,
			ExecutableTarget:      target.snapshot,
			TargetRuntimeRegistry: exactRuntimeRegistryForTarget(t, target),
			CurrentGraph:          graphObservationForStage(t, stage, target),
			CurrentProfile:        profile,
			CurrentHead:           head,
			PriorExecutable:       prior,
		},
	)
}

func noCanonicalProfileForStage(
	t *testing.T,
	stage projecttypeenvselection.ProjectTypeEnvStage,
) projecttypeenvprofilebasis.NoCanonicalProjectProfile {
	t.Helper()
	root, err := projectprofile.NewProjectRootV1(
		"/tmp/haft-stage-revalidation-" + stage.Project().String(),
	)
	if err != nil {
		t.Fatalf("NewProjectRootV1(): %v", err)
	}
	profile, err := projecttypeenvprofilebasis.NewNoCanonicalProjectProfile(root)
	if err != nil {
		t.Fatalf("NewNoCanonicalProjectProfile(): %v", err)
	}
	return profile
}

func declaredSoftwareProfileForStage(
	t *testing.T,
	stage projecttypeenvselection.ProjectTypeEnvStage,
	seed string,
) projecttypeenvprofilebasis.DeclaredCanonicalProjectProfile {
	t.Helper()
	scopeID, err := projectprofile.NewScopeID("software")
	if err != nil {
		t.Fatalf("NewScopeID(): %v", err)
	}
	scope, err := projectprofile.NewSoftwareRealization(
		scopeID,
		projectprofile.NoEntityReference{},
	)
	if err != nil {
		t.Fatalf("NewSoftwareRealization(): %v", err)
	}
	scopes, err := projectprofile.NewScopeSet(
		[]projectprofile.RealizationScope{scope},
	)
	if err != nil {
		t.Fatalf("NewScopeSet(): %v", err)
	}
	payload, err := projectprofile.NewProfileDeclarationPayload(scopes)
	if err != nil {
		t.Fatalf("NewProfileDeclarationPayload(): %v", err)
	}
	return declaredProfileForStage(t, stage, seed, payload)
}

func declaredIncompatibleProfileForStage(
	t *testing.T,
	stage projecttypeenvselection.ProjectTypeEnvStage,
	seed string,
) projecttypeenvprofilebasis.DeclaredCanonicalProjectProfile {
	t.Helper()
	scopeID, err := projectprofile.NewScopeID("non-software")
	if err != nil {
		t.Fatalf("NewScopeID(): %v", err)
	}
	kindRef, err := projectprofile.NewKindRef("invalid kind coordinate")
	if err != nil {
		t.Fatalf("NewKindRef(): %v", err)
	}
	scope, err := projectprofile.NewNonSoftwareRealization(
		scopeID,
		projectprofile.NoEntityReference{},
		projectprofile.NewReferencedKindOrientation(kindRef),
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("NewNonSoftwareRealization(): %v", err)
	}
	scopes, err := projectprofile.NewScopeSet(
		[]projectprofile.RealizationScope{scope},
	)
	if err != nil {
		t.Fatalf("NewScopeSet(): %v", err)
	}
	payload, err := projectprofile.NewProfileDeclarationPayload(scopes)
	if err != nil {
		t.Fatalf("NewProfileDeclarationPayload(): %v", err)
	}
	return declaredProfileForStage(t, stage, seed, payload)
}

func declaredProfileForStage(
	t *testing.T,
	stage projecttypeenvselection.ProjectTypeEnvStage,
	seed string,
	payload projectprofile.ProfileDeclarationPayload,
) projecttypeenvprofilebasis.DeclaredCanonicalProjectProfile {
	t.Helper()
	if len(seed) != 1 || !strings.Contains("0123456789abcdef", seed) {
		t.Fatalf("profile seed %q must be one lowercase hexadecimal digit", seed)
	}
	root, err := projectprofile.NewProjectRootV1(
		"/tmp/haft-stage-revalidation-" + stage.Project().String(),
	)
	if err != nil {
		t.Fatalf("NewProjectRootV1(): %v", err)
	}
	admissionRef, err :=
		projectprofile.NewProfileDeclarationAdmissionRecordRef(
			"profile-admission:stage-" + seed,
		)
	if err != nil {
		t.Fatalf("NewProfileDeclarationAdmissionRecordRef(): %v", err)
	}
	workRef, err := projectprofile.NewProfileOnboardingWorkRecordRef(
		"work:stage-profile-" + seed,
	)
	if err != nil {
		t.Fatalf("NewProfileOnboardingWorkRecordRef(): %v", err)
	}
	authorityBasisRef, err :=
		projectprofile.NewProfileDeclarationAuthorityBasisRef(
			"authority-basis:stage-profile-" + seed,
		)
	if err != nil {
		t.Fatalf("NewProfileDeclarationAuthorityBasisRef(): %v", err)
	}
	authorityResolutionRef, err :=
		projectprofile.NewAuthorityResolutionRecordRef(
			"authority-resolution:stage-profile-" + seed,
		)
	if err != nil {
		t.Fatalf("NewAuthorityResolutionRecordRef(): %v", err)
	}
	roleAssignmentRef, err := projectprofile.NewRoleAssignmentRef(
		"role-assignment:stage-profile-" + seed,
	)
	if err != nil {
		t.Fatalf("NewRoleAssignmentRef(): %v", err)
	}
	observedBasisRef, err := projectprofile.NewObservedProjectBasisRefV1(
		"observed-basis:stage-profile-" + seed,
	)
	if err != nil {
		t.Fatalf("NewObservedProjectBasisRefV1(): %v", err)
	}
	outcomeRef, err :=
		projectprofile.NewProfileOnboardingOutcomeAssessmentRefV1(
			"outcome:stage-profile-" + seed,
		)
	if err != nil {
		t.Fatalf("NewProfileOnboardingOutcomeAssessmentRefV1(): %v", err)
	}
	digest, err := projectprofile.NewContentDigest(
		"sha256:" + strings.Repeat(seed, 64),
	)
	if err != nil {
		t.Fatalf("NewContentDigest(): %v", err)
	}
	profile, err :=
		projecttypeenvprofilebasis.NewDeclaredCanonicalProjectProfile(
			projecttypeenvprofilebasis.DeclaredProjectProfileBasisInput{
				ProjectRoot:                       root,
				LedgerRevision:                    projectprofile.NewLedgerRevision(1),
				Payload:                           payload,
				AdmissionRecordRef:                admissionRef,
				AdmissionRecordDigest:             digest,
				AdmissionRecordCanonicalJSON:      []byte(`{"schema":"test-admission/v1"}`),
				ReceiptDigest:                     digest,
				ReceiptCanonicalJSON:              []byte(`{"schema":"test-receipt/v1"}`),
				CandidateProvenanceDigest:         digest,
				WorkRecordRef:                     workRef,
				WorkRecordDigest:                  digest,
				AuthorityBasisRef:                 authorityBasisRef,
				AuthorityBasisDigest:              digest,
				AuthorityResolutionRef:            authorityResolutionRef,
				AuthorityResolutionDigest:         digest,
				ProfileAuthorRoleAssignmentRef:    roleAssignmentRef,
				ProfileAuthorRoleAssignmentDigest: digest,
				ObservedProjectBasisRef:           observedBasisRef,
				ObservedProjectBasisDigest:        digest,
				OutcomeAssessmentRef:              outcomeRef,
				OutcomeAssessmentDigest:           digest,
			},
		)
	if err != nil {
		t.Fatalf("NewDeclaredCanonicalProjectProfile(): %v", err)
	}
	return profile
}

func stageWithCurrentProfile(
	t *testing.T,
	stage projecttypeenvselection.ProjectTypeEnvStage,
	target targetClosureFixture,
	profile projecttypeenvprofilebasis.CurrentProjectProfileBasis,
) projecttypeenvselection.ProjectTypeEnvStage {
	t.Helper()
	currentGraph := graphObservationForStage(t, stage, target)
	return stageWithCurrentGraphAndProfile(
		t,
		stage,
		target,
		currentGraph,
		profile,
	)
}

func stageWithCurrentGraphAndProfile(
	t *testing.T,
	stage projecttypeenvselection.ProjectTypeEnvStage,
	target targetClosureFixture,
	currentGraph projectgraphobservation.CurrentProjectGraphObservation,
	profile projecttypeenvprofilebasis.CurrentProjectProfileBasis,
) projecttypeenvselection.ProjectTypeEnvStage {
	t.Helper()
	transitionProjectionProfiles, _ := stage.TransitionProjectionProfileCompatibility()
	return stageWithCurrentGraphProfileAndTransitionProfiles(
		t,
		stage,
		target,
		currentGraph,
		profile,
		transitionProjectionProfiles,
	)
}

func stageWithCurrentGraphProfileAndTransitionProfiles(
	t *testing.T,
	stage projecttypeenvselection.ProjectTypeEnvStage,
	target targetClosureFixture,
	currentGraph projectgraphobservation.CurrentProjectGraphObservation,
	profile projecttypeenvprofilebasis.CurrentProjectProfileBasis,
	transitionProjectionProfiles projecttypeenvtransitioncompatibility.Set,
) projecttypeenvselection.ProjectTypeEnvStage {
	t.Helper()
	if err := currentGraph.Verify(); err != nil {
		t.Fatalf("CurrentProjectGraphObservation.Verify(): %v", err)
	}
	graphBasis := currentGraph.GraphSnapshotBasis()
	if graphBasis.Project() != stage.Project() {
		t.Fatal("current graph and Stage projects differ")
	}
	assessment, err := projecttypeenvprofilefit.AssessProjectTypeEnvProfileFit(
		profile,
		target.snapshot,
	)
	if err != nil {
		t.Fatalf("AssessProjectTypeEnvProfileFit(): %v", err)
	}
	targetRuntime := exactRuntimeRegistryForTarget(t, target)
	assertions, err := projecttypeenvassertionrevalidation.Revalidate(
		projecttypeenvassertionrevalidation.Input{
			CurrentGraph:  currentGraph,
			TargetTypeEnv: target.snapshot.Environment(),
			TargetRuntime: targetRuntime,
		},
	)
	if err != nil {
		t.Fatalf("projecttypeenvassertionrevalidation.Revalidate(): %v", err)
	}
	updated, err := projecttypeenvselection.SealProjectTypeEnvStage(
		projecttypeenvselection.ProjectTypeEnvStageInput{
			Project:                                  stage.Project(),
			Predecessor:                              stage.Predecessor(),
			Base:                                     stage.Base(),
			OrderedExtensions:                        stage.OrderedExtensions(),
			RuntimeBasis:                             stage.RuntimeBasis(),
			VerifiedComposite:                        target.verification,
			Composite:                                stage.VerifiedComposite(),
			GraphSnapshotBasis:                       graphBasis,
			GraphSnapshotBasisRef:                    graphBasis.Ref(),
			GraphSnapshotBasisDigest:                 graphBasis.Ref().Digest(),
			GraphRevision:                            graphBasis.GraphRevision(),
			ProfileLedgerRevision:                    profile.LedgerRevision(),
			ProfileLedgerDigest:                      profile.ProfileLedgerDigest(),
			Compatibility:                            stage.Compatibility(),
			ExistingAssertionRevalidation:            assertions,
			ProfileCompatibility:                     assessment,
			TransitionProjectionProfileCompatibility: transitionProjectionProfiles,
		},
	)
	if err != nil {
		t.Fatalf("SealProjectTypeEnvStage(profile): %v", err)
	}
	return updated
}

func stageWithProfileLedgerCoordinates(
	t *testing.T,
	stage projecttypeenvselection.ProjectTypeEnvStage,
	target targetClosureFixture,
	revision projectprofile.LedgerRevision,
	digest typedmemory.SHA256Digest,
) projecttypeenvselection.ProjectTypeEnvStage {
	t.Helper()
	graphBasis, err := projecttypeenvselection.SealProjectGraphSnapshotBasis(
		projecttypeenvselection.ProjectGraphSnapshotBasisInput{
			Project:       stage.Project(),
			GraphRevision: stage.GraphRevision(),
			Closure:       projecttypeenvselection.EmptyProjectGraphClosure{},
		},
	)
	if err != nil {
		t.Fatalf("SealProjectGraphSnapshotBasis(): %v", err)
	}
	if graphBasis.Ref() != stage.GraphSnapshotBasis() {
		t.Fatal("reconstructed graph basis differs from Stage")
	}
	transitionProjectionProfiles, _ := stage.TransitionProjectionProfileCompatibility()
	updated, err := projecttypeenvselection.SealProjectTypeEnvStage(
		projecttypeenvselection.ProjectTypeEnvStageInput{
			Project:                                  stage.Project(),
			Predecessor:                              stage.Predecessor(),
			Base:                                     stage.Base(),
			OrderedExtensions:                        stage.OrderedExtensions(),
			RuntimeBasis:                             stage.RuntimeBasis(),
			VerifiedComposite:                        target.verification,
			Composite:                                stage.VerifiedComposite(),
			GraphSnapshotBasis:                       graphBasis,
			GraphSnapshotBasisRef:                    graphBasis.Ref(),
			GraphSnapshotBasisDigest:                 graphBasis.Ref().Digest(),
			GraphRevision:                            graphBasis.GraphRevision(),
			ProfileLedgerRevision:                    revision,
			ProfileLedgerDigest:                      digest,
			Compatibility:                            stage.Compatibility(),
			ExistingAssertionRevalidation:            stage.ExistingAssertionRevalidation(),
			ProfileCompatibility:                     stage.ProfileCompatibility(),
			TransitionProjectionProfileCompatibility: transitionProjectionProfiles,
		},
	)
	if err != nil {
		t.Fatalf("SealProjectTypeEnvStage(profile ledger coordinates): %v", err)
	}
	return updated
}

func committedGraphBasis(
	t *testing.T,
	stage projecttypeenvselection.ProjectTypeEnvStage,
	eventSeed string,
	commitSeed string,
	materializationSeed string,
) projecttypeenvselection.ProjectGraphSnapshotBasis {
	t.Helper()
	event, err := projecttypeenvselection.ParseGraphEventRef(
		"typed-memory-event:" + strings.Repeat(eventSeed, 64),
	)
	if err != nil {
		t.Fatalf("ParseGraphEventRef(): %v", err)
	}
	commit, err := projecttypeenvselection.ParseGraphCommitRef(
		"typed-memory-commit:" + strings.Repeat(commitSeed, 64),
	)
	if err != nil {
		t.Fatalf("ParseGraphCommitRef(): %v", err)
	}
	closure, err := projecttypeenvselection.NewCommittedProjectGraphClosure(
		projecttypeenvselection.CommittedProjectGraphClosureInput{
			Event:                 event,
			Commit:                commit,
			MaterializationDigest: testDigest(t, materializationSeed),
		},
	)
	if err != nil {
		t.Fatalf("NewCommittedProjectGraphClosure(): %v", err)
	}
	basis, err := projecttypeenvselection.SealProjectGraphSnapshotBasis(
		projecttypeenvselection.ProjectGraphSnapshotBasisInput{
			Project:       stage.Project(),
			GraphRevision: typedmemory.NewGraphRevision(1),
			Closure:       closure,
		},
	)
	if err != nil {
		t.Fatalf("SealProjectGraphSnapshotBasis(committed): %v", err)
	}
	return basis
}

func graphObservationForBasis(
	t *testing.T,
	basis projecttypeenvselection.ProjectGraphSnapshotBasis,
	activeTypeEnv typedmemory.TypeEnvRef,
) projectgraphobservation.CurrentProjectGraphObservation {
	t.Helper()
	active, err := projectgraphobservation.NewCurrentActiveAssertionSet(
		basis.Project(),
		basis.GraphRevision(),
		nil,
	)
	if err != nil {
		t.Fatalf("NewCurrentActiveAssertionSet(committed): %v", err)
	}
	observation, err := projectgraphobservation.NewCurrentProjectGraphObservation(
		basis,
		activeTypeEnv,
		active,
	)
	if err != nil {
		t.Fatalf("NewCurrentProjectGraphObservation(committed): %v", err)
	}
	return observation
}

func graphObservationForStage(
	t *testing.T,
	stage projecttypeenvselection.ProjectTypeEnvStage,
	target targetClosureFixture,
) projectgraphobservation.CurrentProjectGraphObservation {
	t.Helper()
	basis, err := projecttypeenvselection.SealProjectGraphSnapshotBasis(
		projecttypeenvselection.ProjectGraphSnapshotBasisInput{
			Project:       stage.Project(),
			GraphRevision: typedmemory.NewGraphRevision(0),
			Closure:       projecttypeenvselection.EmptyProjectGraphClosure{},
		},
	)
	if err != nil {
		t.Fatalf("SealProjectGraphSnapshotBasis(graph observation): %v", err)
	}
	if basis.Ref() != stage.GraphSnapshotBasis() ||
		basis.Ref().Digest() != stage.GraphSnapshotBasisDigest() {
		t.Fatal("fixture graph observation does not reconstruct the Stage basis")
	}
	active, err := projectgraphobservation.NewCurrentActiveAssertionSet(
		stage.Project(),
		basis.GraphRevision(),
		nil,
	)
	if err != nil {
		t.Fatalf("NewCurrentActiveAssertionSet: %v", err)
	}
	observation, err := projectgraphobservation.NewCurrentProjectGraphObservation(
		basis,
		target.snapshot.TypeEnvRef(),
		active,
	)
	if err != nil {
		t.Fatalf("NewCurrentProjectGraphObservation: %v", err)
	}
	return observation
}

func issueCodes(
	issues []projecttypeenvstagerevalidation.StageRevalidationIssue,
) []projecttypeenvstagerevalidation.StageRevalidationIssueCode {
	result := make([]projecttypeenvstagerevalidation.StageRevalidationIssueCode, 0, len(issues))
	for _, issue := range issues {
		result = append(result, issue.Code())
	}
	return result
}

func resultIssueCodes(
	result projecttypeenvstagerevalidation.StageRevalidationResult,
) []projecttypeenvstagerevalidation.StageRevalidationIssueCode {
	switch value := result.(type) {
	case projecttypeenvstagerevalidation.InvalidSelectionStage:
		return issueCodes(value.Issues())
	case projecttypeenvstagerevalidation.DriftedSelectionStage:
		return issueCodes(value.Issues())
	case projecttypeenvstagerevalidation.RejectedSelectionStage:
		return issueCodes(value.Issues())
	default:
		return nil
	}
}

func assertRejectedCode(
	t *testing.T,
	result projecttypeenvstagerevalidation.StageRevalidationResult,
	expected projecttypeenvstagerevalidation.StageRevalidationIssueCode,
) {
	t.Helper()
	rejected, ok := result.(projecttypeenvstagerevalidation.RejectedSelectionStage)
	if !ok {
		t.Fatalf(
			"result = %T issues=%#v, want RejectedSelectionStage",
			result,
			resultIssueCodes(result),
		)
	}
	codes := issueCodes(rejected.Issues())
	if !containsIssueCode(codes, expected) {
		t.Fatalf("rejected issue codes = %#v, missing %q", codes, expected)
	}
}

func containsIssueCode(
	values []projecttypeenvstagerevalidation.StageRevalidationIssueCode,
	expected projecttypeenvstagerevalidation.StageRevalidationIssueCode,
) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func containsRequirement(
	values []projecttypeenvstagerevalidation.DerivedInputRequirement,
	expected projecttypeenvstagerevalidation.DerivedInputRequirement,
) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func assertDriftCode(
	t *testing.T,
	result projecttypeenvstagerevalidation.StageRevalidationResult,
	expected projecttypeenvstagerevalidation.StageRevalidationIssueCode,
) {
	t.Helper()
	drifted, ok := result.(projecttypeenvstagerevalidation.DriftedSelectionStage)
	if !ok {
		t.Fatalf(
			"result = %T issues=%#v, want DriftedSelectionStage",
			result,
			resultIssueCodes(result),
		)
	}
	if !containsIssueCode(issueCodes(drifted.Issues()), expected) {
		t.Fatalf("issues = %#v, missing %q", drifted.Issues(), expected)
	}
}

func assertDriftCodes(
	t *testing.T,
	result projecttypeenvstagerevalidation.StageRevalidationResult,
	expected []projecttypeenvstagerevalidation.StageRevalidationIssueCode,
) {
	t.Helper()
	drifted, ok := result.(projecttypeenvstagerevalidation.DriftedSelectionStage)
	if !ok {
		t.Fatalf(
			"result = %T issues=%#v, want DriftedSelectionStage",
			result,
			resultIssueCodes(result),
		)
	}
	if got := issueCodes(drifted.Issues()); !reflect.DeepEqual(got, expected) {
		t.Fatalf("issue codes = %#v, want %#v", got, expected)
	}
}
