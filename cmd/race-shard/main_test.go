package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/racequalification"
)

func TestPlanModeDiscoversAndWritesPlanWithoutRunningTests(t *testing.T) {
	t.Parallel()

	executor := newDiscoveryExecutor()
	outputPath := filepath.Join(t.TempDir(), "plan.json")
	var errorOutput strings.Builder
	exitCode := runCLI(
		context.Background(),
		[]string{
			"--mode", modePlan,
			"--shard-count", "12",
			"--package-timeout", "90m",
			"--output", outputPath,
		},
		testApplication(executor),
		&errorOutput,
	)
	if exitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %s", exitCode, errorOutput.String())
	}
	if got := executor.runCount(); got != 0 {
		t.Fatalf("go test executions = %d, want 0", got)
	}

	document := readRunDocument(t, outputPath)
	if document.Status != "planned" {
		t.Fatalf("status = %q, want planned", document.Status)
	}
	if document.Plan == nil {
		t.Fatal("plan output is missing")
	}
	if err := racequalification.Validate(*document.Plan); err != nil {
		t.Fatalf("output plan validation: %v", err)
	}
	if len(document.Shards) != 0 {
		t.Fatalf("plan mode emitted %d shard runs, want 0", len(document.Shards))
	}
}

func TestShardModeWritesFailedObservationBeforeReturningFailure(t *testing.T) {
	t.Parallel()

	executor := newDiscoveryExecutor()
	executor.runResult = processResult{
		exitCode: 1,
		err:      errors.New("deliberate test failure"),
	}
	outputPath := filepath.Join(t.TempDir(), "shard.json")
	var errorOutput strings.Builder
	exitCode := runCLI(
		context.Background(),
		[]string{
			"--mode", modeShard,
			"--shard-count", "1",
			"--shard-index", "0",
			"--package-timeout", "1m",
			"--output", outputPath,
		},
		testApplication(executor),
		&errorOutput,
	)
	if exitCode != 1 {
		t.Fatalf("exit code = %d, want 1", exitCode)
	}

	document := readRunDocument(t, outputPath)
	if document.Status != string(racequalification.ShardFailed) {
		t.Fatalf("status = %q, want failed", document.Status)
	}
	if len(document.Shards) != 1 {
		t.Fatalf("shard runs = %d, want 1", len(document.Shards))
	}
	observation := document.Shards[0].Observation
	if observation.Status != racequalification.ShardFailed {
		t.Fatalf("observation status = %q, want failed", observation.Status)
	}
	expectedCommands, err := commandsForShard(
		document.Plan.Shards[0],
		time.Minute,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(document.Shards[0].Commands) != len(expectedCommands) {
		t.Fatalf(
			"command observations = %d, want %d",
			len(document.Shards[0].Commands),
			len(expectedCommands),
		)
	}
	for index, command := range document.Shards[0].Commands {
		if command.Status != racequalification.ShardFailed {
			t.Fatalf("command %d status = %q, want failed", index, command.Status)
		}
	}
}

func TestRunCLIWritesInvalidDocumentWhenDiscoveryFails(t *testing.T) {
	t.Parallel()

	executor := &fakeExecutor{outputErr: errors.New("go list unavailable")}
	outputPath := filepath.Join(t.TempDir(), "invalid.json")
	var errorOutput strings.Builder
	exitCode := runCLI(
		context.Background(),
		[]string{
			"--mode", modePlan,
			"--output", outputPath,
		},
		testApplication(executor),
		&errorOutput,
	)
	if exitCode != 1 {
		t.Fatalf("exit code = %d, want 1", exitCode)
	}
	document := readRunDocument(t, outputPath)
	if document.Status != string(racequalification.AggregateInvalid) {
		t.Fatalf("status = %q, want invalid", document.Status)
	}
	if !strings.Contains(document.Error, "go list unavailable") {
		t.Fatalf("error = %q, want discovery cause", document.Error)
	}
}

func TestAllModeWritesFailClosedAggregate(t *testing.T) {
	t.Parallel()

	executor := newDiscoveryExecutor()
	executor.runResult = processResult{
		exitCode: 1,
		err:      errors.New("deliberate shard failure"),
	}
	outputPath := filepath.Join(t.TempDir(), "all.json")
	var errorOutput strings.Builder
	exitCode := runCLI(
		context.Background(),
		[]string{
			"--mode", modeAll,
			"--shard-count", "2",
			"--parallelism", "1",
			"--output", outputPath,
		},
		testApplication(executor),
		&errorOutput,
	)
	if exitCode != 1 {
		t.Fatalf("exit code = %d, want 1", exitCode)
	}
	document := readRunDocument(t, outputPath)
	if document.Aggregate == nil {
		t.Fatal("aggregate result is missing")
	}
	if document.Aggregate.Status != racequalification.AggregateFailed {
		t.Fatalf(
			"aggregate status = %q, want failed",
			document.Aggregate.Status,
		)
	}
	if len(document.Aggregate.Observations) != 2 {
		t.Fatalf(
			"aggregate observations = %d, want 2",
			len(document.Aggregate.Observations),
		)
	}
}

func TestExactTopLevelPatternEscapesEveryNameAsRegexLiteral(t *testing.T) {
	t.Parallel()

	first, err := racequalification.NewTopLevelTestID("TestA+B")
	if err != nil {
		t.Fatal(err)
	}
	second, err := racequalification.NewTopLevelTestID("ExampleDot.Name")
	if err != nil {
		t.Fatal(err)
	}
	third, err := racequalification.NewTopLevelTestID("FuzzRoundTrip")
	if err != nil {
		t.Fatal(err)
	}
	pattern, err := exactTopLevelPattern(
		[]racequalification.TopLevelTestID{first, second, third},
	)
	if err != nil {
		t.Fatal(err)
	}
	if pattern != `^(?:ExampleDot\.Name|FuzzRoundTrip|TestA\+B)$` {
		t.Fatalf("pattern = %q", pattern)
	}
}

func TestParseTopLevelTestsRejectsSubtestIdentity(t *testing.T) {
	t.Parallel()

	_, err := parseTopLevelTests(
		[]byte("TestParent/child\n"),
		currentCLIPackage,
	)
	if err == nil || !strings.Contains(err.Error(), "subtests remain") {
		t.Fatalf("error = %v, want subtest rejection", err)
	}
}

func TestParseTopLevelTestsIncludesFuzzTargets(t *testing.T) {
	t.Parallel()

	tests, err := parseTopLevelTests(
		[]byte("TestParent\nExample_partition\nFuzzRoundTrip\n"),
		currentCLIPackage,
	)
	if err != nil {
		t.Fatal(err)
	}
	if got := tests[len(tests)-1].String(); got != "FuzzRoundTrip" {
		t.Fatalf("last discovered target = %q, want FuzzRoundTrip", got)
	}
}

func TestDesktopExclusionUsesAnExactPathSegment(t *testing.T) {
	t.Parallel()

	for _, packagePath := range []string{
		"github.com/m0n0x41d/haft/desktop",
		"github.com/m0n0x41d/haft/desktop/backend",
	} {
		if !isDesktopPackage(packagePath) {
			t.Fatalf("%q was not excluded", packagePath)
		}
	}
	for _, packagePath := range []string{
		"github.com/m0n0x41d/haft/desktopish",
		"github.com/m0n0x41d/haft/internal/desktop_adapter",
	} {
		if isDesktopPackage(packagePath) {
			t.Fatalf("%q was excluded without a desktop path segment", packagePath)
		}
	}
}

func TestCommandsForShardPreserveRaceCountTimeoutAndSoleSkip(t *testing.T) {
	t.Parallel()

	whole := mustPackageID(t, "github.com/m0n0x41d/haft/internal/whole")
	split := mustPackageID(t, currentCLIPackage)
	first := mustTopLevelTestID(t, "TestOne")
	second := mustTopLevelTestID(t, "ExampleTwo")
	commands, err := commandsForShard(
		racequalification.Shard{
			Index: 0,
			Work: []racequalification.WorkItem{
				{
					Kind:    racequalification.WholePackageWork,
					Package: whole,
				},
				{
					Kind:         racequalification.SplitTopLevelTestWork,
					Package:      split,
					TopLevelTest: first,
				},
				{
					Kind:         racequalification.SplitTopLevelTestWork,
					Package:      split,
					TopLevelTest: second,
				},
			},
		},
		90*time.Minute,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(commands) != 2 {
		t.Fatalf("commands = %d, want 2", len(commands))
	}
	for _, command := range commands {
		requireArgument(t, command.argv, "-race")
		requireArgument(t, command.argv, "-count=1")
		requireArgument(t, command.argv, "-cpu=2")
		requireArgument(t, command.argv, "-parallel=2")
		requireArgument(t, command.argv, "-timeout=1h30m0s")
		if slices.Contains(command.argv, "-v") {
			t.Fatalf("ordinary passing race command is verbose: %#v", command.argv)
		}
		if got := argumentValues(command.argv, "-skip"); !slices.Equal(
			got,
			[]string{racequalification.ConsolidatedP13SkipPattern},
		) {
			t.Fatalf("-skip values = %#v", got)
		}
	}
	splitRunValues := argumentValues(commands[1].argv, "-run")
	if !slices.Equal(
		splitRunValues,
		[]string{"^(?:ExampleTwo|TestOne)$"},
	) {
		t.Fatalf("split -run values = %#v", splitRunValues)
	}
}

func TestExecuteShardRunsPassingAndFailingFixturePackages(t *testing.T) {
	root, err := findModuleRoot()
	if err != nil {
		t.Fatal(err)
	}
	app := application{
		executor: localProcessExecutor{directory: root},
		now:      time.Now,
		stdout:   io.Discard,
		stderr:   io.Discard,
	}
	for _, testCase := range []struct {
		name       string
		packageID  string
		wantStatus racequalification.ShardStatus
	}{
		{
			name:       "pass",
			packageID:  "github.com/m0n0x41d/haft/cmd/race-shard/testdata/pass",
			wantStatus: racequalification.ShardPassed,
		},
		{
			name:       "failure",
			packageID:  "github.com/m0n0x41d/haft/cmd/race-shard/testdata/fail",
			wantStatus: racequalification.ShardFailed,
		},
	} {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			packageID := mustPackageID(t, testCase.packageID)
			run := app.executeShard(
				context.Background(),
				"sha256:fixture",
				racequalification.Shard{
					Index: 0,
					Work: []racequalification.WorkItem{
						{
							Kind:    racequalification.WholePackageWork,
							Package: packageID,
						},
					},
				},
				2*time.Minute,
			)
			if run.Observation.Status != testCase.wantStatus {
				t.Fatalf(
					"status = %q, want %q",
					run.Observation.Status,
					testCase.wantStatus,
				)
			}
			if len(run.Commands) != 1 {
				t.Fatalf("commands = %d, want 1", len(run.Commands))
			}
			requireArgument(t, run.Commands[0].Argv, "-race")
			requireArgument(t, run.Commands[0].Argv, "-count=1")
		})
	}
}

func TestEnvironmentWithOverridesReplacesExactNames(t *testing.T) {
	t.Parallel()

	got := environmentWithOverrides(
		[]string{"KEEP=one", "REPLACE=old", "PREFIX_REPLACE=kept"},
		[]string{"REPLACE=new", "ADDED=two"},
	)
	want := []string{
		"KEEP=one",
		"PREFIX_REPLACE=kept",
		"REPLACE=new",
		"ADDED=two",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("merged environment = %#v, want %#v", got, want)
	}
}

func TestRaceSharedFixtureDirectoryIsPrivateAndRunnerOwned(t *testing.T) {
	t.Parallel()

	configured, cleanup, err := (application{
		executor: localProcessExecutor{directory: t.TempDir()},
	}).withRaceSharedFixtureDirectory()
	if err != nil {
		t.Fatal(err)
	}
	executor, ok := configured.executor.(localProcessExecutor)
	if !ok {
		t.Fatalf("configured executor type = %T", configured.executor)
	}
	var root string
	prefix := racequalification.SharedFixtureDirectoryEnvironment + "="
	for _, entry := range executor.environment {
		if strings.HasPrefix(entry, prefix) {
			root = strings.TrimPrefix(entry, prefix)
		}
	}
	if root == "" {
		t.Fatalf(
			"configured environment omits %s",
			racequalification.SharedFixtureDirectoryEnvironment,
		)
	}
	info, err := os.Lstat(root)
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("shared fixture root mode = %s, want real directory", info.Mode())
	}
	if info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("shared fixture root permissions = %o, want private", info.Mode().Perm())
	}

	cleanup()
	if _, err := os.Lstat(root); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("runner-owned shared fixture root survived cleanup: %v", err)
	}
	cleanup()
}

func TestSplitFixturePipelineDiscoversBuildsValidatesAndExecutesTopLevelWork(
	t *testing.T,
) {
	root, err := findModuleRoot()
	if err != nil {
		t.Fatal(err)
	}
	executor := localProcessExecutor{directory: root}
	for _, testCase := range []struct {
		name            string
		packagePath     string
		shardCount      int
		wantAggregate   racequalification.AggregateStatus
		wantCommands    int
		wantOutputParts []string
	}{
		{
			name:          "pass with parent subtests and example",
			packagePath:   "github.com/m0n0x41d/haft/cmd/race-shard/testdata/splitpass",
			shardCount:    2,
			wantAggregate: racequalification.AggregatePassed,
			wantCommands:  2,
		},
		{
			name:          "deliberate subtest failure",
			packagePath:   "github.com/m0n0x41d/haft/cmd/race-shard/testdata/splitfail",
			shardCount:    1,
			wantAggregate: racequalification.AggregateFailed,
			wantCommands:  1,
			wantOutputParts: []string{
				"TestParent/deliberate-failure",
			},
		},
	} {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			discovery, err := discoverPackageClosure(
				context.Background(),
				executor,
				testCase.packagePath,
				[]string{testCase.packagePath},
			)
			if err != nil {
				t.Fatal(err)
			}
			if len(discovery.WholePackages) != 0 ||
				len(discovery.SplitPackages) != 1 {
				t.Fatalf("fixture discovery = %#v", discovery)
			}
			for _, testID := range discovery.SplitPackages[0].TopLevelTests {
				if strings.Contains(testID.String(), "/") {
					t.Fatalf("discovery split subtest %q into independent work", testID)
				}
			}
			built, err := racequalification.Build(
				discovery,
				testCase.shardCount,
			)
			if err != nil {
				t.Fatal(err)
			}
			if err := racequalification.Validate(built.Plan()); err != nil {
				t.Fatal(err)
			}

			var output strings.Builder
			app := application{
				executor: executor,
				now:      time.Now,
				stdout:   &output,
				stderr:   &output,
			}
			runs := app.executeAll(
				context.Background(),
				built,
				1,
				2*time.Minute,
			)
			observations := make(
				[]racequalification.ShardObservation,
				len(runs),
			)
			commandCount := 0
			for index, run := range runs {
				observations[index] = run.Observation
				commandCount += len(run.Commands)
			}
			if commandCount != testCase.wantCommands {
				t.Fatalf(
					"executed commands = %d, want %d",
					commandCount,
					testCase.wantCommands,
				)
			}
			aggregate, err := racequalification.Aggregate(
				built.Plan(),
				observations,
			)
			if err != nil {
				t.Fatal(err)
			}
			if aggregate.Status != testCase.wantAggregate {
				t.Fatalf(
					"aggregate status = %q, want %q\n%s",
					aggregate.Status,
					testCase.wantAggregate,
					output.String(),
				)
			}
			for _, want := range testCase.wantOutputParts {
				if !strings.Contains(output.String(), want) {
					t.Fatalf(
						"fixture output missing %q:\n%s",
						want,
						output.String(),
					)
				}
			}
		})
	}
}

func TestExecuteShardClassifiesTimeoutAndInterruption(t *testing.T) {
	t.Parallel()

	packageID := mustPackageID(t, "github.com/m0n0x41d/haft/internal/slow")
	shard := racequalification.Shard{
		Index: 0,
		Work: []racequalification.WorkItem{
			{
				Kind:    racequalification.WholePackageWork,
				Package: packageID,
			},
		},
	}
	timeoutExecutor := &fakeExecutor{
		runFunc: func(ctx context.Context) processResult {
			<-ctx.Done()
			return processResult{exitCode: -1, err: ctx.Err()}
		},
	}
	timeoutRun := testApplication(timeoutExecutor).executeShard(
		context.Background(),
		"sha256:timeout",
		shard,
		5*time.Millisecond,
	)
	if timeoutRun.Observation.Status != racequalification.ShardTimedOut {
		t.Fatalf(
			"timeout status = %q, want %q",
			timeoutRun.Observation.Status,
			racequalification.ShardTimedOut,
		)
	}

	interrupted, cancel := context.WithCancel(context.Background())
	cancel()
	interruptedRun := testApplication(&fakeExecutor{}).executeShard(
		interrupted,
		"sha256:interrupted",
		shard,
		time.Minute,
	)
	if interruptedRun.Observation.Status !=
		racequalification.ShardInterrupted {
		t.Fatalf(
			"interrupted status = %q, want %q",
			interruptedRun.Observation.Status,
			racequalification.ShardInterrupted,
		)
	}
}

func TestExecuteShardRecordsExplicitEmptyShardAsPassedNoWork(t *testing.T) {
	t.Parallel()

	executor := &fakeExecutor{}
	run := testApplication(executor).executeShard(
		context.Background(),
		"sha256:empty",
		racequalification.Shard{
			Index: 3,
			Work:  []racequalification.WorkItem{},
		},
		time.Minute,
	)
	if run.Observation.Status != racequalification.ShardPassed {
		t.Fatalf("status = %q, want passed", run.Observation.Status)
	}
	if run.Commands == nil || len(run.Commands) != 0 {
		t.Fatalf("commands = %#v, want explicit empty array", run.Commands)
	}
	if got := executor.runCount(); got != 0 {
		t.Fatalf("executed commands = %d, want 0", got)
	}
}

func TestLocalProcessCancellationTerminatesSignalIgnoringDescendant(
	t *testing.T,
) {
	root, err := findModuleRoot()
	if err != nil {
		t.Fatal(err)
	}
	readyPath := filepath.Join(t.TempDir(), "ready")
	survivedPath := filepath.Join(t.TempDir(), "survived")
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan processResult, 1)
	go func() {
		result <- (localProcessExecutor{directory: root}).Run(
			ctx,
			io.Discard,
			io.Discard,
			"/bin/sh",
			"-c",
			`(trap '' TERM; printf ready > "$1"; sleep 0.5; printf survived > "$2") & wait`,
			"race-shard-process-tree",
			readyPath,
			survivedPath,
		)
	}()

	readyDeadline := time.Now().Add(2 * time.Second)
	for {
		if _, err := os.Stat(readyPath); err == nil {
			break
		} else if !os.IsNotExist(err) {
			t.Fatal(err)
		}
		if time.Now().After(readyDeadline) {
			t.Fatal("signal-ignoring descendant did not become ready")
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	select {
	case observed := <-result:
		if !errors.Is(observed.err, context.Canceled) {
			t.Fatalf("canceled process error = %v", observed.err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("canceled process group did not terminate")
	}
	time.Sleep(700 * time.Millisecond)
	if _, err := os.Stat(survivedPath); !os.IsNotExist(err) {
		t.Fatalf(
			"signal-ignoring descendant survived cancellation: %v",
			err,
		)
	}
}

func TestExecuteAllUsesBoundedWorkerPoolAndReturnsEveryShard(t *testing.T) {
	t.Parallel()

	discovery := racequalification.Discovery{
		WholePackages: []racequalification.PackageID{
			mustPackageID(t, "example.test/one"),
			mustPackageID(t, "example.test/two"),
			mustPackageID(t, "example.test/three"),
			mustPackageID(t, "example.test/four"),
			mustPackageID(t, "example.test/five"),
			mustPackageID(t, "example.test/six"),
		},
		SplitPackages: []racequalification.SplitPackageDiscovery{},
	}
	plan, err := racequalification.Build(discovery, 4)
	if err != nil {
		t.Fatal(err)
	}
	var active atomic.Int64
	var maximum atomic.Int64
	executor := &fakeExecutor{
		runFunc: func(context.Context) processResult {
			current := active.Add(1)
			for {
				prior := maximum.Load()
				if current <= prior || maximum.CompareAndSwap(prior, current) {
					break
				}
			}
			time.Sleep(20 * time.Millisecond)
			active.Add(-1)
			return processResult{exitCode: 0}
		},
	}
	runs := testApplication(executor).executeAll(
		context.Background(),
		plan,
		2,
		time.Minute,
	)
	if len(runs) != 4 {
		t.Fatalf("shard runs = %d, want 4", len(runs))
	}
	if got := maximum.Load(); got < 2 || got > 2 {
		t.Fatalf("maximum concurrent commands = %d, want 2", got)
	}
	for index, run := range runs {
		if run.Observation.ShardIndex != index {
			t.Fatalf(
				"run %d has shard index %d",
				index,
				run.Observation.ShardIndex,
			)
		}
		if run.Observation.Status != racequalification.ShardPassed {
			t.Fatalf("shard %d status = %q", index, run.Observation.Status)
		}
	}
}

func TestExecuteAllSharesTheCommandPoolAcrossOneShard(t *testing.T) {
	t.Parallel()

	discovery := racequalification.Discovery{
		WholePackages: []racequalification.PackageID{
			mustPackageID(t, "example.test/one"),
			mustPackageID(t, "example.test/two"),
			mustPackageID(t, "example.test/three"),
			mustPackageID(t, "example.test/four"),
		},
		SplitPackages: []racequalification.SplitPackageDiscovery{},
	}
	plan, err := racequalification.Build(discovery, 1)
	if err != nil {
		t.Fatal(err)
	}
	var active atomic.Int64
	var maximum atomic.Int64
	executor := &fakeExecutor{
		runFunc: func(context.Context) processResult {
			current := active.Add(1)
			for {
				prior := maximum.Load()
				if current <= prior || maximum.CompareAndSwap(prior, current) {
					break
				}
			}
			time.Sleep(20 * time.Millisecond)
			active.Add(-1)
			return processResult{exitCode: 0}
		},
	}
	runs := testApplication(executor).executeAll(
		context.Background(),
		plan,
		2,
		time.Minute,
	)
	if len(runs) != 1 {
		t.Fatalf("shard runs = %d, want 1", len(runs))
	}
	if got := maximum.Load(); got != 2 {
		t.Fatalf("maximum concurrent commands = %d, want 2", got)
	}
	if got := len(runs[0].Commands); got != 4 {
		t.Fatalf("command observations = %d, want 4", got)
	}
	if runs[0].Observation.Status != racequalification.ShardPassed {
		t.Fatalf("shard status = %q, want passed", runs[0].Observation.Status)
	}
}

func TestCommandWorkerPoolsReserveCapacityForWholeAndSplitWork(t *testing.T) {
	t.Parallel()

	wholeJobs := make([]commandJob, 140)
	splitJobs := make([]commandJob, 84)
	pools := commandWorkerPools(wholeJobs, splitJobs, 8)
	if len(pools) != 2 {
		t.Fatalf("worker pools = %d, want 2", len(pools))
	}
	if got := pools[0].workerCount; got != 6 {
		t.Fatalf("split workers = %d, want 6", got)
	}
	if got := pools[1].workerCount; got != 2 {
		t.Fatalf("whole-package workers = %d, want 2", got)
	}
	if got := len(pools[0].jobs); got != len(splitJobs) {
		t.Fatalf("split jobs = %d, want %d", got, len(splitJobs))
	}
	if got := len(pools[1].jobs); got != len(wholeJobs) {
		t.Fatalf("whole-package jobs = %d, want %d", got, len(wholeJobs))
	}

	pools = commandWorkerPools(wholeJobs, splitJobs, 9)
	if got := pools[0].workerCount; got != 7 {
		t.Fatalf("nine-worker split preference = %d, want 7", got)
	}
	if got := pools[1].workerCount; got != 2 {
		t.Fatalf("nine-worker whole preference = %d, want 2", got)
	}
}

func TestCommandWorkerPoolsReuseCapacityWhenOneClassIsScarce(t *testing.T) {
	t.Parallel()

	wholeJobs := make([]commandJob, 10)
	splitJobs := make([]commandJob, 1)
	pools := commandWorkerPools(wholeJobs, splitJobs, 8)
	if len(pools) != 2 {
		t.Fatalf("worker pools = %d, want 2", len(pools))
	}
	if got := pools[0].workerCount; got != 1 {
		t.Fatalf("split workers = %d, want 1", got)
	}
	if got := pools[1].workerCount; got != 7 {
		t.Fatalf("whole-package workers = %d, want 7", got)
	}
}

func TestOrderedSplitCommandJobsUsePriorityThenShardThenCommand(t *testing.T) {
	t.Parallel()

	newJob := func(packagePath string, shardIndex int, commandIndex int) commandJob {
		packageID := mustPackageID(t, packagePath)
		return commandJob{
			shardIndex:   shardIndex,
			commandIndex: commandIndex,
			spec: commandSpec{
				packageID: packageID,
				workKind:  racequalification.SplitTopLevelTestWork,
				argv:      []string{"go", "test", "-run", "TestSplit", packagePath},
			},
		}
	}
	key := func(job commandJob) string {
		return job.spec.packageID.String() + ":" +
			string(rune('0'+job.shardIndex)) + ":" +
			string(rune('0'+job.commandIndex))
	}

	input := []commandJob{
		newJob("github.com/m0n0x41d/haft/db", 0, 4),
		newJob("github.com/m0n0x41d/haft/internal/fpfrefresh", 2, 8),
		newJob("github.com/m0n0x41d/haft/internal/cli", 0, 5),
		newJob("github.com/m0n0x41d/haft/internal/fpfrefresh", 0, 9),
		newJob("github.com/m0n0x41d/haft/internal/fpfrefresh", 0, 3),
		newJob(
			"github.com/m0n0x41d/haft/internal/projecttypeenvselectioneffect/sqlite",
			1,
			6,
		),
		newJob("example.test/zeta", 0, 1),
		newJob("example.test/alpha", 0, 2),
	}
	original := append([]commandJob{}, input...)
	ordered := orderedSplitCommandJobs(input)

	want := []commandJob{
		input[4],
		input[3],
		input[1],
		input[2],
		input[5],
		input[0],
		input[7],
		input[6],
	}
	if len(ordered) != len(want) {
		t.Fatalf("ordered jobs = %d, want %d", len(ordered), len(want))
	}
	for index := range want {
		if got, expected := key(ordered[index]), key(want[index]); got != expected {
			t.Fatalf("ordered job %d = %q, want %q", index, got, expected)
		}
		if got, expected := key(input[index]), key(original[index]); got != expected {
			t.Fatalf("ordering mutated input at %d: got %q, want %q", index, got, expected)
		}
	}
}

func TestExecuteAllPreservesCommandSlotsAfterPriorityDispatch(t *testing.T) {
	t.Parallel()

	cli := mustPackageID(t, "github.com/m0n0x41d/haft/internal/cli")
	refresh := mustPackageID(t, "github.com/m0n0x41d/haft/internal/fpfrefresh")
	discovery := racequalification.Discovery{
		WholePackages: []racequalification.PackageID{},
		SplitPackages: []racequalification.SplitPackageDiscovery{
			{
				Package: cli,
				TopLevelTests: []racequalification.TopLevelTestID{
					mustTopLevelTestID(t, "TestCLIAlpha"),
					mustTopLevelTestID(t, "TestCLIBeta"),
				},
			},
			{
				Package: refresh,
				TopLevelTests: []racequalification.TopLevelTestID{
					mustTopLevelTestID(t, "TestRefreshAlpha"),
					mustTopLevelTestID(t, "TestRefreshBeta"),
				},
			},
		},
	}
	plan, err := racequalification.Build(discovery, 2)
	if err != nil {
		t.Fatal(err)
	}
	executor := &fakeExecutor{}
	runs := testApplication(executor).executeAll(
		context.Background(),
		plan,
		1,
		time.Minute,
	)

	executor.mu.Lock()
	executed := append([][]string{}, executor.runs...)
	executor.mu.Unlock()
	if len(executed) != 4 {
		t.Fatalf("executed commands = %d, want 4", len(executed))
	}
	for index := 0; index < 2; index++ {
		if got := executed[index][len(executed[index])-1]; got != refresh.String() {
			t.Fatalf("priority command %d package = %q, want %q", index, got, refresh)
		}
	}
	for shardIndex, run := range runs {
		expected, err := commandsForShard(plan.Plan().Shards[shardIndex], time.Minute)
		if err != nil {
			t.Fatal(err)
		}
		if len(run.Commands) != len(expected) {
			t.Fatalf(
				"shard %d commands = %d, want %d",
				shardIndex,
				len(run.Commands),
				len(expected),
			)
		}
		for commandIndex := range expected {
			if run.Commands[commandIndex].PackageID != expected[commandIndex].packageID ||
				!slices.Equal(
					run.Commands[commandIndex].Argv,
					expected[commandIndex].argv,
				) {
				t.Fatalf(
					"shard %d command %d escaped its canonical result slot",
					shardIndex,
					commandIndex,
				)
			}
		}
	}
}

func TestCommandWorkersStealAfterPrimaryQueueDrains(t *testing.T) {
	t.Parallel()

	const parallelism = 8
	releaseSplit := make(chan struct{})
	splitReachedEight := make(chan struct{})
	var reachedOnce sync.Once
	var active atomic.Int64
	var maximum atomic.Int64
	var splitStarted atomic.Int64
	executor := &fakeExecutor{
		runCommandFunc: func(
			ctx context.Context,
			command []string,
		) processResult {
			current := active.Add(1)
			for {
				prior := maximum.Load()
				if current <= prior || maximum.CompareAndSwap(prior, current) {
					break
				}
			}
			if slices.Contains(command, "-run") {
				if splitStarted.Add(1) == parallelism {
					reachedOnce.Do(func() {
						close(splitReachedEight)
					})
				}
				select {
				case <-releaseSplit:
				case <-ctx.Done():
					active.Add(-1)
					return processResult{exitCode: -1, err: ctx.Err()}
				}
			}
			active.Add(-1)
			return processResult{exitCode: 0}
		},
	}
	newJob := func(
		packagePath string,
		workKind racequalification.WorkKind,
		index int,
	) commandJob {
		argv := []string{"go", "test"}
		if workKind == racequalification.SplitTopLevelTestWork {
			argv = append(argv, "-run", "TestSplit")
		}
		argv = append(argv, packagePath)
		return commandJob{
			shardIndex:   index,
			commandIndex: 0,
			spec: commandSpec{
				packageID: mustPackageID(t, packagePath),
				workKind:  workKind,
				argv:      argv,
			},
		}
	}
	splitJobs := make([]commandJob, 12)
	for index := range splitJobs {
		splitJobs[index] = newJob(
			"github.com/m0n0x41d/haft/internal/fpfrefresh",
			racequalification.SplitTopLevelTestWork,
			index,
		)
	}
	wholeJobs := []commandJob{
		newJob("example.test/whole-one", racequalification.WholePackageWork, 20),
		newJob("example.test/whole-two", racequalification.WholePackageWork, 21),
	}
	pools := commandWorkerPools(wholeJobs, splitJobs, parallelism)
	results := make(chan commandJobResult, len(splitJobs)+len(wholeJobs))
	var workers sync.WaitGroup
	startCommandWorkerPools(
		context.Background(),
		testApplication(executor),
		pools,
		time.Minute,
		results,
		&workers,
	)

	reached := false
	select {
	case <-splitReachedEight:
		reached = true
	case <-time.After(2 * time.Second):
	}
	close(releaseSplit)
	workers.Wait()
	close(results)
	if !reached {
		t.Fatal("whole-preferred workers did not steal split jobs")
	}
	if got := maximum.Load(); got != parallelism {
		t.Fatalf("maximum concurrent commands = %d, want %d", got, parallelism)
	}
	if got := len(results); got != len(splitJobs)+len(wholeJobs) {
		t.Fatalf(
			"command results = %d, want %d",
			got,
			len(splitJobs)+len(wholeJobs),
		)
	}
}

func TestCommandWorkersDrainCanceledQueuesExactlyOnce(t *testing.T) {
	t.Parallel()

	newJob := func(workKind racequalification.WorkKind, index int) commandJob {
		packagePath := "example.test/canceled"
		argv := []string{"go", "test"}
		if workKind == racequalification.SplitTopLevelTestWork {
			argv = append(argv, "-run", "TestCanceled")
		}
		argv = append(argv, packagePath)
		return commandJob{
			shardIndex:   index,
			commandIndex: index,
			spec: commandSpec{
				packageID: mustPackageID(t, packagePath),
				workKind:  workKind,
				argv:      argv,
			},
		}
	}
	splitJobs := make([]commandJob, 10)
	for index := range splitJobs {
		splitJobs[index] = newJob(racequalification.SplitTopLevelTestWork, index)
	}
	wholeJobs := make([]commandJob, 4)
	for index := range wholeJobs {
		wholeJobs[index] = newJob(racequalification.WholePackageWork, index+10)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	executor := &fakeExecutor{}
	results := make(chan commandJobResult, len(splitJobs)+len(wholeJobs))
	var workers sync.WaitGroup
	startCommandWorkerPools(
		ctx,
		testApplication(executor),
		commandWorkerPools(wholeJobs, splitJobs, 8),
		time.Minute,
		results,
		&workers,
	)
	done := make(chan struct{})
	go func() {
		workers.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("canceled command queues did not drain")
	}
	close(results)

	seen := map[[2]int]struct{}{}
	for result := range results {
		key := [2]int{result.shardIndex, result.commandIndex}
		if _, exists := seen[key]; exists {
			t.Fatalf("job result %v was emitted more than once", key)
		}
		seen[key] = struct{}{}
		if result.observation.Status != racequalification.ShardInterrupted {
			t.Fatalf(
				"job result %v status = %q, want interrupted",
				key,
				result.observation.Status,
			)
		}
	}
	if got, want := len(seen), len(splitJobs)+len(wholeJobs); got != want {
		t.Fatalf("terminal job results = %d, want %d", got, want)
	}
	if got := executor.runCount(); got != 0 {
		t.Fatalf("executor runs after pre-cancellation = %d, want 0", got)
	}
}

func TestRealCurrentPlanIsStableAndCoversDiscoveredNonDesktopClosure(t *testing.T) {
	root, err := findModuleRoot()
	if err != nil {
		t.Fatal(err)
	}
	executor := localProcessExecutor{directory: root}
	firstDiscovery, err := discover(context.Background(), executor)
	if err != nil {
		t.Fatal(err)
	}
	secondDiscovery, err := discover(context.Background(), executor)
	if err != nil {
		t.Fatal(err)
	}
	first, err := racequalification.Build(firstDiscovery, 12)
	if err != nil {
		t.Fatal(err)
	}
	second, err := racequalification.Build(secondDiscovery, 12)
	if err != nil {
		t.Fatal(err)
	}
	if first.Digest() != second.Digest() {
		t.Fatalf(
			"identical current discovery digests differ: %q != %q",
			first.Digest(),
			second.Digest(),
		)
	}
	if err := racequalification.Validate(first.Plan()); err != nil {
		t.Fatalf("current plan validation: %v", err)
	}

	rawList, err := executor.Output(
		context.Background(),
		"go",
		"list",
		"-race",
		"./...",
	)
	if err != nil {
		t.Fatal(err)
	}
	wantPackages := 0
	for _, packagePath := range nonEmptyLines(rawList) {
		if !isDesktopPackage(packagePath) {
			wantPackages++
		}
	}
	gotPackages := len(firstDiscovery.WholePackages) +
		len(firstDiscovery.SplitPackages)
	if gotPackages != wantPackages {
		t.Fatalf(
			"discovered packages = %d, want %d current non-desktop packages",
			gotPackages,
			wantPackages,
		)
	}

	wholePackagePaths := make(
		map[string]struct{},
		len(firstDiscovery.WholePackages),
	)
	for _, packageID := range firstDiscovery.WholePackages {
		wholePackagePaths[packageID.String()] = struct{}{}
	}
	splitPackageTests := make(
		map[string][]racequalification.TopLevelTestID,
		len(firstDiscovery.SplitPackages),
	)
	for _, split := range firstDiscovery.SplitPackages {
		splitPackageTests[split.Package.String()] = split.TopLevelTests
	}
	for _, packagePath := range racequalification.CurrentSplitPackagePaths() {
		tests, found := splitPackageTests[packagePath]
		if !found {
			t.Errorf("configured split package %q was not discovered as split", packagePath)
		} else if len(tests) == 0 {
			t.Errorf("configured split package %q exposed no top-level tests", packagePath)
		}
		if _, found := wholePackagePaths[packagePath]; found {
			t.Errorf("configured split package %q was also discovered whole", packagePath)
		}
	}

	assigned := 0
	for _, shard := range first.Plan().Shards {
		assigned += len(shard.Work)
	}
	wantWork := len(firstDiscovery.WholePackages)
	for _, split := range firstDiscovery.SplitPackages {
		wantWork += len(split.TopLevelTests)
	}
	if assigned != wantWork {
		t.Fatalf("assigned work = %d, want %d", assigned, wantWork)
	}
}

func TestParseConfigurationRejectsInvalidModeIndexAndTimeout(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name string
		args []string
	}{
		{
			name: "mode",
			args: []string{"--mode", "unknown", "--output", "out.json"},
		},
		{
			name: "missing shard index",
			args: []string{"--mode", modeShard, "--output", "out.json"},
		},
		{
			name: "index outside range",
			args: []string{
				"--mode", modeShard,
				"--shard-count", "2",
				"--shard-index", "2",
				"--output", "out.json",
			},
		},
		{
			name: "index in plan mode",
			args: []string{
				"--mode", modePlan,
				"--shard-index", "0",
				"--output", "out.json",
			},
		},
		{
			name: "timeout",
			args: []string{
				"--mode", modePlan,
				"--package-timeout", "0s",
				"--output", "out.json",
			},
		},
		{
			name: "negative parallelism",
			args: []string{
				"--mode", modePlan,
				"--parallelism", "-1",
				"--output", "out.json",
			},
		},
	} {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			var output strings.Builder
			if _, err := parseConfiguration(testCase.args, &output); err == nil {
				t.Fatal("parseConfiguration() error = nil")
			}
		})
	}
}

func TestParseConfigurationResolvesAutomaticParallelism(t *testing.T) {
	t.Parallel()

	var output strings.Builder
	config, err := parseConfiguration(
		[]string{
			"--mode", modePlan,
			"--parallelism", "0",
			"--output", "out.json",
		},
		&output,
	)
	if err != nil {
		t.Fatal(err)
	}
	if config.parallelism <= 0 {
		t.Fatalf("resolved parallelism = %d, want positive GOMAXPROCS", config.parallelism)
	}
}

func TestAutomaticCommandParallelismReservesCapacityForTheHost(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		logicalCPUs int
		want        int
	}{
		{logicalCPUs: 1, want: 1},
		{logicalCPUs: 2, want: 1},
		{logicalCPUs: 3, want: 1},
		{logicalCPUs: 4, want: 1},
		{logicalCPUs: 8, want: 3},
		{logicalCPUs: 12, want: 4},
	} {
		got := automaticCommandParallelism(testCase.logicalCPUs)
		if got != testCase.want {
			t.Fatalf(
				"automaticCommandParallelism(%d) = %d, want %d",
				testCase.logicalCPUs,
				got,
				testCase.want,
			)
		}
	}
}

func TestAutomaticCommandParallelismBoundsAggregateTestCPUCapacity(t *testing.T) {
	t.Parallel()

	for _, logicalCPUs := range []int{1, 2, 3, 4, 8, 12, 16, 32} {
		workers := automaticCommandParallelism(logicalCPUs)
		if logicalCPUs > 1 &&
			workers*racequalification.CurrentCommandGOMAXPROCS > logicalCPUs {
			t.Fatalf(
				"automatic workers consume %d test CPUs on a %d-CPU host",
				workers*racequalification.CurrentCommandGOMAXPROCS,
				logicalCPUs,
			)
		}
	}
}

func TestCanonicalSplitPackageSetRejectsMissingOrderAndDuplicates(t *testing.T) {
	t.Parallel()

	for _, packagePaths := range [][]string{
		nil,
		{"example.test/zeta", "example.test/alpha"},
		{"example.test/same", "example.test/same"},
	} {
		if _, err := canonicalSplitPackageSet(packagePaths); err == nil {
			t.Fatalf("canonicalSplitPackageSet(%#v) error = nil", packagePaths)
		}
	}
}

func testApplication(executor commandExecutor) application {
	return application{
		executor: executor,
		now: func() time.Time {
			return time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
		},
		stdout: io.Discard,
		stderr: io.Discard,
	}
}

type fakeExecutor struct {
	mu             sync.Mutex
	outputErr      error
	runResult      processResult
	runFunc        func(context.Context) processResult
	runCommandFunc func(context.Context, []string) processResult
	runs           [][]string
}

func newDiscoveryExecutor() *fakeExecutor {
	return &fakeExecutor{}
}

func (executor *fakeExecutor) Output(
	_ context.Context,
	name string,
	args ...string,
) ([]byte, error) {
	if executor.outputErr != nil {
		return nil, executor.outputErr
	}
	command := append([]string{name}, args...)
	switch {
	case slices.Equal(command, []string{"go", "list", "-race", "./..."}):
		packages := racequalification.CurrentSplitPackagePaths()
		packages = append(
			packages,
			"github.com/m0n0x41d/haft/internal/other",
			"github.com/m0n0x41d/haft/desktop",
		)
		return []byte(strings.Join(packages, "\n") + "\n"), nil
	case isSplitPackageListCommand(command):
		packagePath := command[len(command)-1]
		return []byte(
			"TestAlpha\nExampleBeta\nFuzzGamma\nok  " + packagePath + " 0.1s\n",
		), nil
	default:
		return nil, errors.New("unexpected output command")
	}
}

func isSplitPackageListCommand(command []string) bool {
	if len(command) != 6 {
		return false
	}
	prefix := []string{
		"go",
		"test",
		"-race",
		"-list",
		"^(Test|Example|Fuzz)",
	}
	if !slices.Equal(command[:5], prefix) {
		return false
	}
	packagePath := command[5]
	return slices.Contains(
		racequalification.CurrentSplitPackagePaths(),
		packagePath,
	)
}

func (executor *fakeExecutor) Run(
	ctx context.Context,
	_ io.Writer,
	_ io.Writer,
	name string,
	args ...string,
) processResult {
	command := append([]string{name}, args...)
	executor.mu.Lock()
	executor.runs = append(
		executor.runs,
		command,
	)
	runFunc := executor.runFunc
	runCommandFunc := executor.runCommandFunc
	result := executor.runResult
	executor.mu.Unlock()
	if runCommandFunc != nil {
		return runCommandFunc(ctx, command)
	}
	if runFunc != nil {
		return runFunc(ctx)
	}
	return result
}

func (executor *fakeExecutor) runCount() int {
	executor.mu.Lock()
	defer executor.mu.Unlock()
	return len(executor.runs)
}

func readRunDocument(t *testing.T, path string) runDocument {
	t.Helper()
	encoded, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var document runDocument
	if err := json.Unmarshal(encoded, &document); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return document
}

func mustPackageID(
	t *testing.T,
	raw string,
) racequalification.PackageID {
	t.Helper()
	id, err := racequalification.NewPackageID(raw)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func mustTopLevelTestID(
	t *testing.T,
	raw string,
) racequalification.TopLevelTestID {
	t.Helper()
	id, err := racequalification.NewTopLevelTestID(raw)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func requireArgument(t *testing.T, argv []string, argument string) {
	t.Helper()
	if !slices.Contains(argv, argument) {
		t.Fatalf("argv %#v does not contain %q", argv, argument)
	}
}

func argumentValues(argv []string, name string) []string {
	values := []string{}
	for index := 0; index+1 < len(argv); index++ {
		if argv[index] == name {
			values = append(values, argv[index+1])
		}
	}
	return values
}
