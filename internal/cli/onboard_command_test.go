package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/onboarding"
	"github.com/m0n0x41d/haft/internal/onboardingfs"
	"github.com/m0n0x41d/haft/internal/projecttypeenvreviewcarrier"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func TestOnboardPublicCommandTreeHidesLowLevelMemorySetup(
	t *testing.T,
) {
	t.Parallel()

	fixtures := []struct {
		path []string
		want *cobra.Command
	}{
		{
			path: []string{"onboard", "profile", "apply"},
			want: onboardProfileApplyCmd,
		},
	}
	for _, obsolete := range [][]string{
		{"onboard", "memory", "enable"},
		{"onboard", "memory", "defer"},
	} {
		command, _, err := rootCmd.Find(obsolete)
		if err == nil && (command == onboardMemoryEnableCmd || command == onboardMemoryDeferCmd) {
			t.Fatalf("obsolete public memory command remains registered: %v", obsolete)
		}
	}
	for _, fixture := range fixtures {
		command, remaining, err := rootCmd.Find(fixture.path)
		if err != nil {
			t.Fatal(err)
		}
		if command != fixture.want || len(remaining) != 0 {
			t.Fatalf(
				"find %v = %v/%v",
				fixture.path,
				command,
				remaining,
			)
		}
		flagNames := []string{}
		command.LocalNonPersistentFlags().VisitAll(
			func(flag *pflag.Flag) {
				flagNames = append(flagNames, flag.Name)
			},
		)
		if !reflect.DeepEqual(flagNames, []string{"json"}) {
			t.Fatalf(
				"%s public flags = %#v, want json only",
				command.CommandPath(),
				flagNames,
			)
		}
	}
	hidden, _, err := rootCmd.Find(
		[]string{"memory", "typeenv", "select"},
	)
	if err != nil || hidden != memoryTypeEnvSelectCmd {
		t.Fatalf(
			"hidden diagnostic command is not callable: %v/%v",
			hidden,
			err,
		)
	}
	output := &bytes.Buffer{}
	previous := memoryCmd.OutOrStdout()
	memoryCmd.SetOut(output)
	defer memoryCmd.SetOut(previous)
	if err := memoryCmd.Help(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(
		strings.ToLower(output.String()),
		"typeenv",
	) {
		t.Fatalf(
			"default memory help exposed low-level setup:\n%s",
			output,
		)
	}
}

func TestOnboardProfileMatrixRunsReviewThroughPublicApplyCommand(
	t *testing.T,
) {
	fixtures := []struct {
		name          string
		files         []string
		manual        bool
		manualScopes  []onboardScopeWire
		wantScopeKind []string
	}{
		{
			name:  "typescript",
			files: []string{"package.json", "src/index.ts"},
			wantScopeKind: []string{
				"software",
			},
		},
		{
			name:  "python",
			files: []string{"pyproject.toml", "src/app.py"},
			wantScopeKind: []string{
				"software",
			},
		},
		{
			name:  "rust",
			files: []string{"Cargo.toml", "src/main.rs"},
			wantScopeKind: []string{
				"software",
			},
		},
		{
			name:   "zig manual fallback",
			files:  []string{"build.zig", "src/main.zig"},
			manual: true,
			manualScopes: []onboardScopeWire{{
				ScopeID:         "zig-app",
				Label:           "Zig application",
				RealizationKind: "software",
				EvidencePaths:   []string{"build.zig"},
			}},
			wantScopeKind: []string{
				"software",
			},
		},
		{
			name:   "elixir manual fallback",
			files:  []string{"mix.exs", "lib/app.ex"},
			manual: true,
			manualScopes: []onboardScopeWire{{
				ScopeID:         "elixir-app",
				Label:           "Elixir application",
				RealizationKind: "software",
				EvidencePaths:   []string{"mix.exs"},
			}},
			wantScopeKind: []string{
				"software",
			},
		},
		{
			name:   "dart manual fallback",
			files:  []string{"pubspec.yaml", "lib/main.dart"},
			manual: true,
			manualScopes: []onboardScopeWire{{
				ScopeID:         "dart-app",
				Label:           "Dart application",
				RealizationKind: "software",
				EvidencePaths:   []string{"pubspec.yaml"},
			}},
			wantScopeKind: []string{
				"software",
			},
		},
		{
			name: "docs only",
			files: []string{
				"mkdocs.yml",
				"docs/intro.md",
				"docs/usage.md",
				"docs/design.rst",
			},
			wantScopeKind: []string{
				"non_software",
			},
		},
		{
			name: "mixed software and model",
			files: []string{
				"package.json",
				"src/index.ts",
				"models/current.onnx",
			},
			wantScopeKind: []string{
				"non_software",
				"software",
			},
		},
		{
			name:   "empty manual fallback",
			manual: true,
			manualScopes: []onboardScopeWire{{
				ScopeID:         "future-app",
				Label:           "Future application",
				RealizationKind: "software",
				EvidencePaths:   []string{},
			}},
			wantScopeKind: []string{
				"software",
			},
		},
	}
	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			project := newCLIProfileOnboardLedgerFixture(t)
			for _, relative := range fixture.files {
				writeProfileInspectionFixture(
					t,
					project.root,
					relative,
				)
			}
			binding := mustOnboardProjectBinding(
				t,
				project.root,
			)
			t.Setenv(envProjectRoot, binding.ProjectRoot)
			t.Setenv(
				envExpectedProjectID,
				binding.ProjectID,
			)
			surface, err := openSealedProjectOnboardSurface(
				context.Background(),
				binding,
			)
			if err != nil {
				t.Fatal(err)
			}
			defer surface.Close()
			handler := surface.Handler()
			preparedOutput := callOnboardHandler(
				t,
				handler,
				`{"action":"profile_prepare"}`,
			)
			prepared := decodeOnboardResponse(
				t,
				preparedOutput,
			)
			if fixture.manual {
				if prepared.Result !=
					"needs_scope_review" {
					t.Fatalf(
						"automatic fallback = %#v",
						prepared,
					)
				}
				preparedOutput = prepareManualProfileReview(
					t,
					handler,
					fixture.manualScopes,
				)
				prepared = decodeOnboardResponse(
					t,
					preparedOutput,
				)
			}
			if prepared.Result !=
				"profile_review_prepared" ||
				prepared.Status !=
					"profile_review_ready" {
				t.Fatalf(
					"prepared review = %#v",
					prepared,
				)
			}
			kinds := onboardScopeKinds(prepared.Scopes)
			if !reflect.DeepEqual(
				kinds,
				fixture.wantScopeKind,
			) {
				t.Fatalf(
					"prepared kinds = %#v, want %#v",
					kinds,
					fixture.wantScopeKind,
				)
			}
			assertOnboardOutputHasNoInternalJargon(
				t,
				preparedOutput,
			)
			firstOutput, firstErr :=
				runOnboardProfileApplyJSONForTest(t)
			if firstErr != nil {
				t.Fatalf(
					"public profile apply: %v\n%s",
					firstErr,
					firstOutput,
				)
			}
			first := decodeOnboardTaskEffect(
				t,
				firstOutput,
			)
			if first.Result != "profile_applied" ||
				first.Status != "needs_init" ||
				!first.Effects.CanonicalProfileChanged {
				t.Fatalf(
					"fresh public apply = %#v",
					first,
				)
			}
			assertOnboardOutputHasNoInternalJargon(
				t,
				firstOutput,
			)
			replayOutput, replayErr :=
				runOnboardProfileApplyJSONForTest(t)
			if replayErr != nil {
				t.Fatalf(
					"public profile replay: %v\n%s",
					replayErr,
					replayOutput,
				)
			}
			replay := decodeOnboardTaskEffect(
				t,
				replayOutput,
			)
			if replay.Result != "profile_applied" ||
				replay.Status != "needs_init" ||
				replay.Delivery != "reused" ||
				replay.Effects.CanonicalProfileChanged {
				t.Fatalf(
					"public apply replay = %#v",
					replay,
				)
			}
		})
	}
}

func TestOnboardFreshProjectMemoryEnableRestartAndDeferralLifecycle(
	t *testing.T,
) {
	t.Run("enable and reconnect", func(t *testing.T) {
		fixture := prepareAppliedTypeScriptProject(t)
		defer fixture.surface.Close()
		preparedOutput := callOnboardHandler(
			t,
			fixture.surface.Handler(),
			`{"action":"memory_prepare"}`,
		)
		prepared := decodeOnboardResponse(t, preparedOutput)
		if prepared.Result != "memory_review_prepared" {
			t.Fatalf("memory prepare = %#v", prepared)
		}
		assertMemoryReviewReadyForTaskCommand(
			t,
			fixture.binding,
		)
		enableOutput, enableErr :=
			runOnboardMemoryEnableJSONForTest(t)
		if enableErr != nil {
			t.Fatalf(
				"public memory enable: %v\n%s",
				enableErr,
				enableOutput,
			)
		}
		enabled := decodeOnboardTaskEffect(
			t,
			enableOutput,
		)
		if enabled.Result != "restart_required" ||
			enabled.Status != "restart_required" ||
			enabled.Delivery != "applied" ||
			!enabled.Effects.StructuredMemoryEnabled {
			t.Fatalf("memory enable = %#v", enabled)
		}
		assertOnboardOutputHasNoInternalJargon(
			t,
			enableOutput,
		)
		staleOutput := callOnboardHandler(
			t,
			fixture.surface.Handler(),
			`{"action":"status"}`,
		)
		stale := decodeOnboardResponse(t, staleOutput)
		if stale.Result != "restart_required" {
			t.Fatalf("old MCP status = %#v", stale)
		}
		fresh, err := openSealedProjectOnboardSurface(
			context.Background(),
			fixture.binding,
		)
		if err != nil {
			t.Fatal(err)
		}
		defer fresh.Close()
		freshOutput := callOnboardHandler(
			t,
			fresh.Handler(),
			`{"action":"status"}`,
		)
		freshStatus := decodeOnboardResponse(
			t,
			freshOutput,
		)
		if freshStatus.Result != "ready" ||
			freshStatus.Status != "ready" {
			t.Fatalf(
				"fresh MCP status = %#v",
				freshStatus,
			)
		}
		profileReplayOutput, profileReplayErr :=
			runOnboardProfileApplyJSONForTest(t)
		if profileReplayErr != nil {
			t.Fatalf(
				"profile replay after memory enable: %v\n%s",
				profileReplayErr,
				profileReplayOutput,
			)
		}
		profileReplay := decodeOnboardTaskEffect(
			t,
			profileReplayOutput,
		)
		if profileReplay.Result != "profile_applied" ||
			profileReplay.Status != "ready" ||
			profileReplay.Delivery != "reused" ||
			profileReplay.Effects.CanonicalProfileChanged {
			t.Fatalf(
				"profile replay after memory enable = %#v",
				profileReplay,
			)
		}
		replayOutput, replayErr :=
			runOnboardMemoryEnableJSONForTest(t)
		if replayErr != nil {
			t.Fatalf(
				"public memory replay: %v\n%s",
				replayErr,
				replayOutput,
			)
		}
		replay := decodeOnboardTaskEffect(
			t,
			replayOutput,
		)
		if replay.Delivery != "reused" ||
			replay.Effects.StructuredMemoryEnabled {
			t.Fatalf(
				"memory replay = %#v",
				replay,
			)
		}
	})

	t.Run("defer replay reopen and enable", func(t *testing.T) {
		fixture := prepareAppliedTypeScriptProject(t)
		defer fixture.surface.Close()
		callOnboardHandler(
			t,
			fixture.surface.Handler(),
			`{"action":"memory_prepare"}`,
		)
		assertMemoryReviewReadyForTaskCommand(
			t,
			fixture.binding,
		)
		deferOutput, deferErr :=
			runOnboardMemoryDeferJSONForTest(t)
		if deferErr != nil {
			t.Fatalf(
				"public memory defer: %v\n%s",
				deferErr,
				deferOutput,
			)
		}
		deferred := decodeOnboardTaskEffect(
			t,
			deferOutput,
		)
		if deferred.Result != "memory_deferred" ||
			deferred.Status != "memory_deferred" ||
			!deferred.Effects.MemoryDeferred {
			t.Fatalf("memory defer = %#v", deferred)
		}
		assertOnboardOutputHasNoInternalJargon(
			t,
			deferOutput,
		)
		statusOutput := callOnboardHandler(
			t,
			fixture.surface.Handler(),
			`{"action":"status"}`,
		)
		status := decodeOnboardResponse(t, statusOutput)
		if status.Result != "onboarding_required" ||
			status.Status != "needs_init" {
			t.Fatalf("deferred status = %#v", status)
		}
		profileReplayOutput, profileReplayErr :=
			runOnboardProfileApplyJSONForTest(t)
		if profileReplayErr != nil {
			t.Fatalf(
				"profile replay after memory defer: %v\n%s",
				profileReplayErr,
				profileReplayOutput,
			)
		}
		profileReplay := decodeOnboardTaskEffect(
			t,
			profileReplayOutput,
		)
		if profileReplay.Result != "profile_applied" ||
			profileReplay.Status != "needs_init" ||
			profileReplay.Delivery != "reused" ||
			profileReplay.Effects.CanonicalProfileChanged {
			t.Fatalf(
				"profile replay after memory defer = %#v",
				profileReplay,
			)
		}
		replayOutput, replayErr :=
			runOnboardMemoryDeferJSONForTest(t)
		if replayErr != nil {
			t.Fatalf(
				"public defer replay: %v\n%s",
				replayErr,
				replayOutput,
			)
		}
		replay := decodeOnboardTaskEffect(
			t,
			replayOutput,
		)
		if replay.Delivery != "reused" ||
			replay.Effects.MemoryDeferred {
			t.Fatalf("defer replay = %#v", replay)
		}
		reopenedOutput := callOnboardHandler(
			t,
			fixture.surface.Handler(),
			`{"action":"memory_prepare"}`,
		)
		reopened := decodeOnboardResponse(
			t,
			reopenedOutput,
		)
		if reopened.Result != "memory_review_reused" ||
			reopened.Status != "memory_review_ready" {
			t.Fatalf(
				"reopened review = %#v",
				reopened,
			)
		}
		readyOutput := callOnboardHandler(
			t,
			fixture.surface.Handler(),
			`{"action":"status"}`,
		)
		ready := decodeOnboardResponse(t, readyOutput)
		if ready.Result != "onboarding_required" ||
			ready.Status != "needs_init" {
			t.Fatalf(
				"reopened status = %#v",
				ready,
			)
		}
		enableOutput, enableErr :=
			runOnboardMemoryEnableJSONForTest(t)
		if enableErr != nil {
			t.Fatalf(
				"enable after reopen: %v\n%s",
				enableErr,
				enableOutput,
			)
		}
		enabled := decodeOnboardTaskEffect(
			t,
			enableOutput,
		)
		if enabled.Result != "restart_required" {
			t.Fatalf(
				"enable after reopen = %#v",
				enabled,
			)
		}
	})
}

func TestOnboardStatusDoesNotTrustUnreadableOrForeignMemoryDeferral(
	t *testing.T,
) {
	t.Run("unreadable", func(t *testing.T) {
		fixture := prepareAppliedTypeScriptProject(t)
		defer fixture.surface.Close()
		callOnboardHandler(
			t,
			fixture.surface.Handler(),
			`{"action":"memory_prepare"}`,
		)
		carrierPath := filepath.Join(
			fixture.binding.ProjectRoot,
			onboardingfs.DirectoryName,
			onboardingfs.FileName,
		)
		if err := os.WriteFile(
			carrierPath,
			[]byte(`{"tampered":true}`),
			0o600,
		); err != nil {
			t.Fatal(err)
		}
		output := callOnboardHandler(
			t,
			fixture.surface.Handler(),
			`{"action":"status"}`,
		)
		response := decodeOnboardResponse(t, output)
		if response.Result != "onboarding_required" ||
			response.Status != "needs_init" ||
			!strings.Contains(response.Detail, "unreadable") {
			t.Fatalf(
				"unreadable deferral status = %#v",
				response,
			)
		}
	})

	t.Run("foreign project", func(t *testing.T) {
		fixture := prepareAppliedTypeScriptProject(t)
		defer fixture.surface.Close()
		callOnboardHandler(
			t,
			fixture.surface.Handler(),
			`{"action":"memory_prepare"}`,
		)
		current := currentMemoryDeferralForTest(
			t,
			fixture.binding,
		)
		foreign, err := onboarding.NewMemoryDeferral(
			onboarding.MemoryDeferralInput{
				ProjectID:    "foreign-project",
				ReviewRef:    current.ReviewRef(),
				ReviewDigest: current.ReviewDigest(),
				Choice:       current.Choice(),
				RecordedAt:   current.RecordedAt(),
			},
		)
		if err != nil {
			t.Fatal(err)
		}
		installed, err := onboardingfs.Install(
			fixture.binding.ProjectRoot,
			foreign,
		)
		if err != nil {
			t.Fatal(err)
		}
		if _, created := installed.(onboardingfs.Created); !created {
			t.Fatalf(
				"foreign deferral install = %T, want Created",
				installed,
			)
		}
		output := callOnboardHandler(
			t,
			fixture.surface.Handler(),
			`{"action":"status"}`,
		)
		response := decodeOnboardResponse(t, output)
		if response.Result != "onboarding_required" ||
			response.Status != "needs_init" ||
			!strings.Contains(
				response.Detail,
				"different onboarding basis",
			) {
			t.Fatalf(
				"foreign deferral status = %#v",
				response,
			)
		}
	})
}

func TestOnboardMemoryEnableProjectsEveryClosedNoCommitOutcome(
	t *testing.T,
) {
	observation, err := onboarding.NewObservation(
		onboarding.ObservationInput{
			Initialized:       true,
			ProfileDeclared:   true,
			MemoryReviewReady: true,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	fixtures := []struct {
		name    string
		outcome projectTypeEnvGenesisSelectionOutcome
		result  string
		status  string
	}{
		{
			name: "not selected",
			outcome: projectTypeEnvGenesisNotSelected{
				Kind:   "not_selected",
				Reason: "review_expired",
			},
			result: "not_enabled",
			status: "needs_init",
		},
		{
			name: "replay conflict",
			outcome: projectTypeEnvGenesisReplayConflict{
				Kind: "replay_conflict",
			},
			result: "enablement_conflict",
			status: "needs_init",
		},
		{
			name: "commit outcome unknown",
			outcome: projectTypeEnvGenesisCommitOutcomeUnknown{
				Kind: "commit_outcome_unknown",
			},
			result: "commit_outcome_unknown",
			status: "outcome_unknown",
		},
	}
	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			output := &bytes.Buffer{}
			command := onboardingTaskTestCommand(output)
			previous := onboardMemoryEnableJSON
			onboardMemoryEnableJSON = true
			defer func() {
				onboardMemoryEnableJSON = previous
			}()
			if err := writeOnboardMemoryNoCommit(
				command,
				observation,
				fixture.outcome,
			); err == nil {
				t.Fatal("no-commit task result returned success")
			}
			response := decodeOnboardTaskEffect(
				t,
				output.String(),
			)
			if response.Result != fixture.result ||
				response.Status != fixture.status {
				t.Fatalf(
					"task projection = %#v",
					response,
				)
			}
			assertOnboardOutputHasNoInternalJargon(
				t,
				output.String(),
			)
		})
	}
}

func TestOnboardConcurrentEnableAndDeferKeepEnabledMemoryAuthoritative(
	t *testing.T,
) {
	t.Run("enable commits after defer appears", func(t *testing.T) {
		fixture := prepareAppliedTypeScriptProject(t)
		defer fixture.surface.Close()
		callOnboardHandler(
			t,
			fixture.surface.Handler(),
			`{"action":"memory_prepare"}`,
		)
		output := &bytes.Buffer{}
		command := onboardingTaskTestCommand(output)
		previous := onboardMemoryEnableJSON
		onboardMemoryEnableJSON = true
		defer func() {
			onboardMemoryEnableJSON = previous
		}()
		effect := func(
			ctx context.Context,
		) (projectTypeEnvGenesisSelectionResponse, error) {
			deferral := currentMemoryDeferralForTest(
				t,
				fixture.binding,
			)
			installed, err := onboardingfs.Install(
				fixture.binding.ProjectRoot,
				deferral,
			)
			if err != nil {
				t.Fatal(err)
			}
			if _, created := installed.(onboardingfs.Created); !created {
				t.Fatalf(
					"concurrent deferral = %T, want Created",
					installed,
				)
			}
			return executeReviewedMemorySelection(ctx)
		}
		if err := runOnboardMemoryEnableWithEffect(
			command,
			effect,
		); err != nil {
			t.Fatalf(
				"enable after concurrent defer: %v\n%s",
				err,
				output,
			)
		}
		response := decodeOnboardTaskEffect(
			t,
			output.String(),
		)
		if response.Result != "restart_required" ||
			!response.Effects.StructuredMemoryEnabled {
			t.Fatalf(
				"concurrent enable result = %#v",
				response,
			)
		}
		if _, present, err := readOnboardMemoryDeferral(
			fixture.binding.ProjectRoot,
		); err != nil || present {
			t.Fatalf(
				"superseded deferral remained: %v/%v",
				present,
				err,
			)
		}
	})

	t.Run("defer observes concurrently enabled memory", func(t *testing.T) {
		fixture := prepareAppliedTypeScriptProject(t)
		defer fixture.surface.Close()
		callOnboardHandler(
			t,
			fixture.surface.Handler(),
			`{"action":"memory_prepare"}`,
		)
		output := &bytes.Buffer{}
		command := onboardingTaskTestCommand(output)
		previous := onboardMemoryDeferJSON
		onboardMemoryDeferJSON = true
		defer func() {
			onboardMemoryDeferJSON = previous
		}()
		effect := func(
			projectRoot string,
			deferral onboarding.MemoryDeferral,
		) (onboardingfs.InstallResult, error) {
			installed, err := onboardingfs.Install(
				projectRoot,
				deferral,
			)
			if err != nil {
				return installed, err
			}
			selection, selectionErr :=
				executeReviewedMemorySelection(
					context.Background(),
				)
			if selectionErr != nil {
				return installed, selectionErr
			}
			_, committed :=
				selection.Outcome.(projectTypeEnvGenesisFreshlyCommitted)
			if !committed {
				t.Fatalf(
					"concurrent selection = %T",
					selection.Outcome,
				)
			}
			return installed, nil
		}
		err := runOnboardMemoryDeferWithEffect(
			command,
			effect,
		)
		if err == nil {
			t.Fatal(
				"superseded defer returned success",
			)
		}
		response := decodeOnboardTaskEffect(
			t,
			output.String(),
		)
		if response.Result !=
			"defer_superseded_by_enabled_memory" ||
			response.Status != "restart_required" {
			t.Fatalf(
				"superseded defer = %#v",
				response,
			)
		}
	})
}

type appliedOnboardingFixture struct {
	binding ProjectBinding
	surface *sealedProjectOnboardSurface
}

func currentMemoryDeferralForTest(
	t *testing.T,
	binding ProjectBinding,
) onboarding.MemoryDeferral {
	t.Helper()
	review, err := projecttypeenvreviewcarrier.Read(
		binding.ProjectRoot,
	)
	if err != nil {
		t.Fatal(err)
	}
	deferral, err := onboarding.NewMemoryDeferral(
		onboarding.MemoryDeferralInput{
			ProjectID:    binding.ProjectID,
			ReviewRef:    onboarding.MemoryReviewRef,
			ReviewDigest: review.Digest().String(),
			Choice: onboarding.
				DeferStructuredMemoryChoice,
			RecordedAt: time.Now().UTC(),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	return deferral
}

func prepareAppliedTypeScriptProject(
	t *testing.T,
) appliedOnboardingFixture {
	t.Helper()
	project := newCLIProfileOnboardLedgerFixture(t)
	writeProfileInspectionFixture(
		t,
		project.root,
		"package.json",
	)
	writeProfileInspectionFixture(
		t,
		project.root,
		"src/index.ts",
	)
	binding := mustOnboardProjectBinding(t, project.root)
	t.Setenv(envProjectRoot, binding.ProjectRoot)
	t.Setenv(envExpectedProjectID, binding.ProjectID)
	surface, err := openSealedProjectOnboardSurface(
		context.Background(),
		binding,
	)
	if err != nil {
		t.Fatal(err)
	}
	prepared := callOnboardHandler(
		t,
		surface.Handler(),
		`{"action":"profile_prepare"}`,
	)
	if decodeOnboardResponse(t, prepared).Result !=
		"profile_review_prepared" {
		t.Fatalf("profile prepare = %s", prepared)
	}
	applied, err := runOnboardProfileApplyJSONForTest(t)
	if err != nil {
		t.Fatalf("profile apply: %v\n%s", err, applied)
	}
	return appliedOnboardingFixture{
		binding: binding,
		surface: surface,
	}
}

func TestOnboardProfileChangePrepareAndApplyRepairsSpecApplicability(
	t *testing.T,
) {
	fixture := prepareAppliedTypeScriptProject(t)
	defer fixture.surface.Close()
	statusOutput := callOnboardHandler(
		t,
		fixture.surface.Handler(),
		`{"action":"status"}`,
	)
	status := decodeOnboardResponse(t, statusOutput)
	if len(status.Scopes) != 1 {
		t.Fatalf("profile scopes = %#v", status.Scopes)
	}
	request := onboardRequestWire{
		Action:    "profile_change_prepare",
		ScopeID:   stringPointer(status.Scopes[0].ScopeID),
		EntityRef: stringPointer("entity:typescript-target-system"),
	}
	requestBytes, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	preparedOutput := callOnboardHandler(
		t,
		fixture.surface.Handler(),
		string(requestBytes),
	)
	prepared := decodeOnboardResponse(t, preparedOutput)
	if prepared.Result != "profile_change_review_prepared" ||
		prepared.Status != "profile_change_review_ready" ||
		prepared.ReviewRef != "review:onboard-profile-change" ||
		!prepared.Effects.ReviewCarrierCreated ||
		prepared.Effects.CanonicalProfileChanged {
		t.Fatalf("profile change prepare = %#v", prepared)
	}
	if _, err := os.Stat(profileChangeReviewPath(fixture.binding.ProjectRoot)); err != nil {
		t.Fatalf("profile change review: %v", err)
	}
	reviewStatusOutput := callOnboardHandler(
		t,
		fixture.surface.Handler(),
		`{"action":"status"}`,
	)
	reviewStatus := decodeOnboardResponse(t, reviewStatusOutput)
	if reviewStatus.Result != "profile_change_review_ready" ||
		reviewStatus.Status != "profile_change_review_ready" ||
		!reviewStatus.ProfileChangeEligible ||
		reviewStatus.ReviewRef != "review:onboard-profile-change" {
		t.Fatalf("profile change review status = %#v", reviewStatus)
	}
	output, err := runOnboardProfileChangeApplyJSONForTest(t)
	if err != nil {
		t.Fatalf("profile change apply: %v\n%s", err, output)
	}
	response := onboardTaskEffectResponse{}
	if err := json.Unmarshal([]byte(output), &response); err != nil {
		t.Fatal(err)
	}
	if response.Result != "profile_change_applied" ||
		response.Action != "profile_change_apply" ||
		!response.Effects.CanonicalProfileChanged {
		t.Fatalf("profile change apply response = %#v", response)
	}
	if _, err := os.Stat(profileChangeReviewPath(fixture.binding.ProjectRoot)); !os.IsNotExist(err) {
		t.Fatalf("applied profile change review was not consumed: %v", err)
	}
	inspection, err := executeProfileInspection(
		context.Background(),
		fixture.binding.ProjectRoot,
		false,
	)
	if err != nil {
		t.Fatal(err)
	}
	if inspection.CanonicalProfile.LedgerRevision != 2 {
		t.Fatalf(
			"canonical profile revision = %d, want 2",
			inspection.CanonicalProfile.LedgerRevision,
		)
	}
	lifecycle, err := buildPublicSpecLifecycle(
		context.Background(),
		fixture.binding.ProjectRoot,
		automaticProjectSpecificationScopeRequest(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if lifecycle.ProfileApplicability.Kind != "resolved" ||
		len(lifecycle.ProfileApplicability.UnderdeterminedKinds) != 0 {
		t.Fatalf(
			"spec applicability after profile change = %#v",
			lifecycle.ProfileApplicability,
		)
	}
}

func prepareManualProfileReview(
	t *testing.T,
	handler func(
		context.Context,
		json.RawMessage,
	) (string, error),
	scopes []onboardScopeWire,
) string {
	t.Helper()
	request := onboardRequestWire{
		Action: "profile_prepare",
		Basis: stringPointer(
			"Explicit readable scope for a repository shape outside automatic detection.",
		),
		Scopes: &scopes,
	}
	content, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	return callOnboardHandler(
		t,
		handler,
		string(content),
	)
}

func assertMemoryReviewReadyForTaskCommand(
	t *testing.T,
	want ProjectBinding,
) {
	t.Helper()
	binding, err := resolveOnboardTaskBinding(
		context.Background(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if binding.ProjectRoot != want.ProjectRoot ||
		binding.ProjectID != want.ProjectID {
		t.Fatalf(
			"task binding = %#v, want %#v",
			binding,
			want,
		)
	}
	observation, err := newProjectOnboardingRuntime(
		binding,
	).Observe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !observation.MemoryReviewReady() {
		t.Fatalf(
			"task runtime observation = ready:%v deferred:%v detail:%q",
			observation.MemoryReady(),
			observation.MemoryDeferred(),
			observation.Detail(),
		)
	}
}

func onboardScopeKinds(
	scopes []onboardScopeResponseWire,
) []string {
	result := make([]string, len(scopes))
	for index, scope := range scopes {
		result[index] = scope.RealizationKind
	}
	return result
}

func runOnboardProfileApplyJSONForTest(
	t *testing.T,
) (string, error) {
	t.Helper()
	output := &bytes.Buffer{}
	command := onboardingTaskTestCommand(output)
	previous := onboardProfileApplyJSON
	onboardProfileApplyJSON = true
	defer func() {
		onboardProfileApplyJSON = previous
	}()
	err := runOnboardProfileApply(command, nil)
	return output.String(), err
}

func runOnboardProfileChangeApplyJSONForTest(
	t *testing.T,
) (string, error) {
	t.Helper()
	output := &bytes.Buffer{}
	command := onboardingTaskTestCommand(output)
	previous := onboardProfileChangeApplyJSON
	onboardProfileChangeApplyJSON = true
	defer func() {
		onboardProfileChangeApplyJSON = previous
	}()
	err := runOnboardProfileChangeApply(command, nil)
	return output.String(), err
}

func runOnboardMemoryEnableJSONForTest(
	t *testing.T,
) (string, error) {
	t.Helper()
	output := &bytes.Buffer{}
	command := onboardingTaskTestCommand(output)
	previous := onboardMemoryEnableJSON
	onboardMemoryEnableJSON = true
	defer func() {
		onboardMemoryEnableJSON = previous
	}()
	err := runOnboardMemoryEnable(command, nil)
	return output.String(), err
}

func runOnboardMemoryDeferJSONForTest(
	t *testing.T,
) (string, error) {
	t.Helper()
	output := &bytes.Buffer{}
	command := onboardingTaskTestCommand(output)
	previous := onboardMemoryDeferJSON
	onboardMemoryDeferJSON = true
	defer func() {
		onboardMemoryDeferJSON = previous
	}()
	err := runOnboardMemoryDefer(command, nil)
	return output.String(), err
}

func onboardingTaskTestCommand(
	output *bytes.Buffer,
) *cobra.Command {
	command := &cobra.Command{}
	command.SetContext(context.Background())
	command.SetOut(output)
	command.SetErr(output)
	command.SetIn(strings.NewReader(""))
	return command
}

func decodeOnboardTaskEffect(
	t *testing.T,
	output string,
) onboardTaskEffectResponse {
	t.Helper()
	response := onboardTaskEffectResponse{}
	if err := json.Unmarshal(
		[]byte(output),
		&response,
	); err != nil {
		t.Fatalf(
			"decode task effect: %v\n%s",
			err,
			output,
		)
	}
	return response
}
