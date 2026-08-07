package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/racequalification"
)

func TestAggregateModeAcceptsCompleteCanonicalEvidenceWithoutDiscovery(
	t *testing.T,
) {
	t.Parallel()

	plan := aggregateFixturePlan(t, "example.test/package")
	inputs := aggregateFixtureInputs(t, plan, -1)
	slices.Reverse(inputs)
	executor := &fakeExecutor{
		outputErr: errors.New("aggregate mode must not discover packages"),
	}
	firstOutput := filepath.Join(t.TempDir(), "aggregate.json")
	var firstErrors strings.Builder
	firstExit := runCLI(
		context.Background(),
		aggregateArgs(inputs, firstOutput),
		testApplication(executor),
		&firstErrors,
	)
	if firstExit != 0 {
		t.Fatalf("first exit = %d, stderr = %s", firstExit, firstErrors.String())
	}
	if got := executor.runCount(); got != 0 {
		t.Fatalf("aggregate executed %d commands, want 0", got)
	}

	first := readRunDocument(t, firstOutput)
	if first.Mode != modeAggregate ||
		first.Status != string(racequalification.AggregatePassed) {
		t.Fatalf("aggregate document = %#v", first)
	}
	if first.Aggregate == nil ||
		first.Aggregate.Status != racequalification.AggregatePassed {
		t.Fatalf("aggregate result = %#v", first.Aggregate)
	}
	if len(first.Shards) != plan.ShardCount {
		t.Fatalf("shard runs = %d, want %d", len(first.Shards), plan.ShardCount)
	}
	for index, run := range first.Shards {
		if run.Observation.ShardIndex != index {
			t.Fatalf(
				"canonical shard position %d has index %d",
				index,
				run.Observation.ShardIndex,
			)
		}
	}

	secondOutput := filepath.Join(t.TempDir(), "aggregate.json")
	slices.Reverse(inputs)
	var secondErrors strings.Builder
	secondExit := runCLI(
		context.Background(),
		aggregateArgs(inputs, secondOutput),
		testApplication(executor),
		&secondErrors,
	)
	if secondExit != 0 {
		t.Fatalf(
			"second exit = %d, stderr = %s",
			secondExit,
			secondErrors.String(),
		)
	}
	firstBytes, err := os.ReadFile(firstOutput)
	if err != nil {
		t.Fatal(err)
	}
	secondBytes, err := os.ReadFile(secondOutput)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstBytes, secondBytes) {
		t.Fatal("canonical aggregate bytes depend on input order or output path")
	}
}

func TestAggregateModeWritesValidFailedResultForCompleteFailedShard(
	t *testing.T,
) {
	t.Parallel()

	plan := aggregateFixturePlan(t, "example.test/failed")
	inputs := aggregateFixtureInputs(t, plan, 4)
	output := filepath.Join(t.TempDir(), "aggregate.json")
	var errorOutput strings.Builder
	exitCode := runCLI(
		context.Background(),
		aggregateArgs(inputs, output),
		testApplication(&fakeExecutor{}),
		&errorOutput,
	)
	if exitCode != 1 {
		t.Fatalf("exit = %d, want 1", exitCode)
	}
	document := readRunDocument(t, output)
	if document.Status != string(racequalification.AggregateFailed) ||
		document.Error != "" {
		t.Fatalf("failed aggregate document = %#v", document)
	}
	if document.Aggregate == nil ||
		document.Aggregate.Status != racequalification.AggregateFailed ||
		!slices.Equal(document.Aggregate.FailedShards, []int{4}) {
		t.Fatalf("failed aggregate = %#v", document.Aggregate)
	}
}

func TestAggregateModeRejectsIncompleteDuplicateAndMismatchedEvidence(
	t *testing.T,
) {
	t.Parallel()

	basePlan := aggregateFixturePlan(t, "example.test/base")
	baseInputs := aggregateFixtureInputs(t, basePlan, -1)
	otherPlan := aggregateFixturePlan(t, "example.test/other")
	otherInputs := aggregateFixtureInputs(t, otherPlan, -1)

	testCases := []struct {
		name   string
		inputs []string
		want   string
	}{
		{
			name:   "missing",
			inputs: append([]string{}, baseInputs[:len(baseInputs)-1]...),
			want:   "want 12",
		},
		{
			name: "duplicate",
			inputs: append(
				append([]string{}, baseInputs[:len(baseInputs)-1]...),
				baseInputs[0],
			),
			want: "duplicate observations",
		},
		{
			name: "mismatched plan",
			inputs: append(
				append([]string{}, baseInputs[:len(baseInputs)-1]...),
				otherInputs[len(otherInputs)-1],
			),
			want: "different validated plan",
		},
	}
	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			output := filepath.Join(t.TempDir(), "aggregate.json")
			var errorOutput strings.Builder
			exitCode := runCLI(
				context.Background(),
				aggregateArgs(testCase.inputs, output),
				testApplication(&fakeExecutor{}),
				&errorOutput,
			)
			if exitCode != 1 {
				t.Fatalf("exit = %d, want 1", exitCode)
			}
			document := readRunDocument(t, output)
			if document.Status != string(racequalification.AggregateInvalid) {
				t.Fatalf("status = %q, want invalid", document.Status)
			}
			if !strings.Contains(document.Error, testCase.want) {
				t.Fatalf(
					"error = %q, want %q",
					document.Error,
					testCase.want,
				)
			}
		})
	}
}

func TestAggregateModeRejectsPrefixOnlyAndUnknownJSONEvidence(t *testing.T) {
	t.Parallel()

	plan := aggregateFixturePlan(t, "example.test/malformed")
	inputs := aggregateFixtureInputs(t, plan, -1)

	prefix := readRunDocument(t, inputs[0])
	prefix.Shards[0].Commands = prefix.Shards[0].Commands[:0]
	if err := writeDocumentAtomically(inputs[0], prefix); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "prefix.json")
	var prefixErrors strings.Builder
	if exitCode := runCLI(
		context.Background(),
		aggregateArgs(inputs, output),
		testApplication(&fakeExecutor{}),
		&prefixErrors,
	); exitCode != 1 {
		t.Fatalf("prefix-only exit = %d, want 1", exitCode)
	}
	if document := readRunDocument(t, output); !strings.Contains(
		document.Error,
		"command observations",
	) {
		t.Fatalf("prefix-only error = %q", document.Error)
	}

	inputs = aggregateFixtureInputs(t, plan, -1)
	encoded, err := os.ReadFile(inputs[0])
	if err != nil {
		t.Fatal(err)
	}
	encoded = bytes.Replace(
		encoded,
		[]byte(`"schema":`),
		[]byte(`"unknown_field":true,"schema":`),
		1,
	)
	if err := os.WriteFile(inputs[0], encoded, 0o644); err != nil {
		t.Fatal(err)
	}
	output = filepath.Join(t.TempDir(), "unknown.json")
	var unknownErrors strings.Builder
	if exitCode := runCLI(
		context.Background(),
		aggregateArgs(inputs, output),
		testApplication(&fakeExecutor{}),
		&unknownErrors,
	); exitCode != 1 {
		t.Fatalf("unknown-field exit = %d, want 1", exitCode)
	}
	if document := readRunDocument(t, output); !strings.Contains(
		document.Error,
		"unknown field",
	) {
		t.Fatalf("unknown-field error = %q", document.Error)
	}
}

func TestAggregateModeWithoutInputsWritesInvalidEvidence(t *testing.T) {
	t.Parallel()

	output := filepath.Join(t.TempDir(), "aggregate.json")
	var errorOutput strings.Builder
	exitCode := runCLI(
		context.Background(),
		aggregateArgs(nil, output),
		testApplication(&fakeExecutor{}),
		&errorOutput,
	)
	if exitCode != 1 {
		t.Fatalf("exit = %d, want 1", exitCode)
	}
	document := readRunDocument(t, output)
	if document.Status != string(racequalification.AggregateInvalid) ||
		!strings.Contains(document.Error, "at least one --input") {
		t.Fatalf("zero-input document = %#v", document)
	}
}

func TestAggregateModeRejectsNonRegularInputWithoutBlocking(t *testing.T) {
	t.Parallel()

	input := filepath.Join(t.TempDir(), "shard.fifo")
	if err := syscall.Mkfifo(input, 0o600); err != nil {
		t.Fatalf("create aggregate input FIFO: %v", err)
	}
	output := filepath.Join(t.TempDir(), "aggregate.json")
	result := make(chan struct {
		exitCode int
		stderr   string
	}, 1)
	go func() {
		var errorOutput strings.Builder
		exitCode := runCLI(
			context.Background(),
			aggregateArgs([]string{input}, output),
			testApplication(&fakeExecutor{}),
			&errorOutput,
		)
		result <- struct {
			exitCode int
			stderr   string
		}{
			exitCode: exitCode,
			stderr:   errorOutput.String(),
		}
	}()

	select {
	case got := <-result:
		if got.exitCode != 1 {
			t.Fatalf("exit = %d, want 1; stderr = %s", got.exitCode, got.stderr)
		}
		document := readRunDocument(t, output)
		if document.Status != string(racequalification.AggregateInvalid) ||
			!strings.Contains(document.Error, "regular file") {
			t.Fatalf("non-regular input document = %#v", document)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("aggregate mode blocked while opening a non-regular input")
	}
}

func TestAggregateModeHonorsCancellationBeforeReadingInput(t *testing.T) {
	t.Parallel()

	input := filepath.Join(t.TempDir(), "shard.json")
	if err := os.WriteFile(input, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "aggregate.json")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var errorOutput strings.Builder
	exitCode := runCLI(
		ctx,
		aggregateArgs([]string{input}, output),
		testApplication(&fakeExecutor{}),
		&errorOutput,
	)
	if exitCode != 1 {
		t.Fatalf("exit = %d, want 1", exitCode)
	}
	document := readRunDocument(t, output)
	if document.Status != string(racequalification.AggregateInvalid) ||
		!strings.Contains(document.Error, context.Canceled.Error()) {
		t.Fatalf("cancelled aggregate document = %#v", document)
	}
}

func TestExecuteShardRecordsEveryCommandAfterFailureAndCancellation(
	t *testing.T,
) {
	t.Parallel()

	first := mustPackageID(t, "example.test/first")
	second := mustPackageID(t, "example.test/second")
	shard := racequalification.Shard{
		Index: 0,
		Work: []racequalification.WorkItem{
			{Kind: racequalification.WholePackageWork, Package: first},
			{Kind: racequalification.WholePackageWork, Package: second},
		},
	}
	var calls atomic.Int64
	failingExecutor := &fakeExecutor{
		runFunc: func(context.Context) processResult {
			if calls.Add(1) == 1 {
				return processResult{
					exitCode: 1,
					err:      errors.New("first command failed"),
				}
			}
			return processResult{exitCode: 0}
		},
	}
	failed := testApplication(failingExecutor).executeShard(
		context.Background(),
		"sha256:complete-failure",
		shard,
		time.Minute,
	)
	if failed.Observation.Status != racequalification.ShardFailed ||
		len(failed.Commands) != 2 ||
		failingExecutor.runCount() != 2 {
		t.Fatalf("complete failed shard = %#v", failed)
	}
	if failed.Commands[0].Status != racequalification.ShardFailed ||
		failed.Commands[1].Status != racequalification.ShardPassed {
		t.Fatalf("command statuses = %#v", failed.Commands)
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	canceledExecutor := &fakeExecutor{}
	interrupted := testApplication(canceledExecutor).executeShard(
		canceled,
		"sha256:complete-interruption",
		shard,
		time.Minute,
	)
	if interrupted.Observation.Status != racequalification.ShardInterrupted ||
		len(interrupted.Commands) != 2 ||
		canceledExecutor.runCount() != 0 {
		t.Fatalf("complete interrupted shard = %#v", interrupted)
	}
	for _, command := range interrupted.Commands {
		if command.Status != racequalification.ShardInterrupted {
			t.Fatalf("interrupted command = %#v", command)
		}
	}
}

func TestParseConfigurationBoundsAggregateInputs(t *testing.T) {
	t.Parallel()

	var errorOutput strings.Builder
	config, err := parseConfiguration(
		[]string{
			"--mode", modeAggregate,
			"--input", "one.json",
			"--input", "two.json",
			"--output", "aggregate.json",
		},
		&errorOutput,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(
		[]string(config.inputPaths),
		[]string{"one.json", "two.json"},
	) {
		t.Fatalf("input paths = %#v", config.inputPaths)
	}

	for _, args := range [][]string{
		{
			"--mode", modePlan,
			"--input", "one.json",
			"--output", "plan.json",
		},
		{
			"--mode", modeAggregate,
			"--input", "same.json",
			"--output", "same.json",
		},
		{
			"--mode", modeAggregate,
			"--input", "",
			"--output", "aggregate.json",
		},
	} {
		if _, err := parseConfiguration(args, &errorOutput); err == nil {
			t.Fatalf("parseConfiguration(%#v) error = nil", args)
		}
	}
}

func TestRunCLIRemovesStaleOutputBeforeWork(t *testing.T) {
	t.Parallel()

	output := filepath.Join(t.TempDir(), "result.json")
	if err := os.WriteFile(output, []byte(`{"status":"passed"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	executor := &fakeExecutor{outputErr: errors.New("discovery unavailable")}
	var errorOutput strings.Builder
	exitCode := runCLI(
		context.Background(),
		[]string{"--mode", modePlan, "--output", output},
		testApplication(executor),
		&errorOutput,
	)
	if exitCode != 1 {
		t.Fatalf("exit = %d, want 1", exitCode)
	}
	document := readRunDocument(t, output)
	if document.Status != string(racequalification.AggregateInvalid) ||
		!strings.Contains(document.Error, "discovery unavailable") {
		t.Fatalf("replacement document = %#v", document)
	}

	directoryOutput := t.TempDir()
	if err := clearOutputPath(directoryOutput); err == nil {
		t.Fatal("clearOutputPath accepted a directory")
	}
}

func aggregateFixturePlan(
	t *testing.T,
	prefix string,
) racequalification.Plan {
	t.Helper()
	packages := make([]racequalification.PackageID, 12)
	for index := range packages {
		packages[index] = mustPackageID(
			t,
			prefix+"-"+string(rune('a'+index)),
		)
	}
	built, err := racequalification.Build(
		racequalification.Discovery{
			WholePackages: packages,
			SplitPackages: []racequalification.SplitPackageDiscovery{},
		},
		12,
	)
	if err != nil {
		t.Fatal(err)
	}
	return built.Plan()
}

func aggregateFixtureInputs(
	t *testing.T,
	plan racequalification.Plan,
	failedShard int,
) []string {
	t.Helper()
	directory := t.TempDir()
	inputs := make([]string, plan.ShardCount)
	for shardIndex := range plan.ShardCount {
		document := aggregateFixtureShardDocument(
			t,
			plan,
			shardIndex,
			shardIndex == failedShard,
		)
		path := filepath.Join(
			directory,
			"shard-"+string(rune('a'+shardIndex))+".json",
		)
		if err := writeDocumentAtomically(path, document); err != nil {
			t.Fatal(err)
		}
		inputs[shardIndex] = path
	}
	return inputs
}

func aggregateFixtureShardDocument(
	t *testing.T,
	plan racequalification.Plan,
	shardIndex int,
	failed bool,
) runDocument {
	t.Helper()
	commands, err := commandsForShard(
		plan.Shards[shardIndex],
		90*time.Minute,
	)
	if err != nil {
		t.Fatal(err)
	}
	documentStart := time.Date(
		2026,
		time.July,
		30,
		10,
		shardIndex,
		0,
		0,
		time.UTC,
	)
	runStart := documentStart.Add(time.Second)
	observations := make([]commandObservation, len(commands))
	status := racequalification.ShardPassed
	for index, command := range commands {
		observations[index] = commandObservation{
			PackageID:  command.packageID,
			Argv:       append([]string{}, command.argv...),
			StartedAt:  runStart.Add(time.Duration(index+1) * time.Second),
			FinishedAt: runStart.Add(time.Duration(index+2) * time.Second),
			ExitCode:   0,
			Status:     racequalification.ShardPassed,
		}
	}
	if failed {
		observations[0].ExitCode = 1
		observations[0].Status = racequalification.ShardFailed
		observations[0].Error = "exit status 1"
		status = racequalification.ShardFailed
	}
	runFinish := runStart.Add(time.Duration(len(commands)+3) * time.Second)
	planCopy := plan
	return runDocument{
		Schema:     runSchema,
		Mode:       modeShard,
		Status:     string(status),
		StartedAt:  documentStart,
		FinishedAt: runFinish.Add(time.Second),
		Plan:       &planCopy,
		Shards: []shardRun{{
			Observation: racequalification.ShardObservation{
				PlanDigest: plan.PlanDigest,
				ShardIndex: shardIndex,
				Status:     status,
			},
			StartedAt:  runStart,
			FinishedAt: runFinish,
			Commands:   observations,
		}},
	}
}

func aggregateArgs(inputs []string, output string) []string {
	args := []string{
		"--mode", modeAggregate,
		"--shard-count", "12",
		"--package-timeout", "90m",
	}
	for _, input := range inputs {
		args = append(args, "--input", input)
	}
	return append(args, "--output", output)
}
