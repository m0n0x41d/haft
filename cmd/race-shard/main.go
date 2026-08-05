package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/m0n0x41d/haft/internal/racequalification"
)

const (
	currentCLIPackage = "github.com/m0n0x41d/haft/internal/cli"
	runSchema         = "haft.race-qualification.run/v1"
	listParallelism   = 4

	modePlan      = "plan"
	modeShard     = "shard"
	modeAll       = "all"
	modeAggregate = "aggregate"
)

type configuration struct {
	mode           string
	shardCount     int
	shardIndex     int
	parallelism    int
	packageTimeout time.Duration
	outputPath     string
	inputPaths     stringListFlag
}

type stringListFlag []string

func (values *stringListFlag) String() string {
	if values == nil {
		return ""
	}
	return strings.Join(*values, ",")
}

func (values *stringListFlag) Set(value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("--input must be non-empty")
	}
	*values = append(*values, value)
	return nil
}

type commandExecutor interface {
	Output(context.Context, string, ...string) ([]byte, error)
	Run(
		context.Context,
		io.Writer,
		io.Writer,
		string,
		...string,
	) processResult
}

type processResult struct {
	exitCode int
	err      error
}

type localProcessExecutor struct {
	directory   string
	environment []string
}

func (executor localProcessExecutor) Output(
	ctx context.Context,
	name string,
	args ...string,
) ([]byte, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	result := executor.run(ctx, &stdout, &stderr, name, args...)
	if result.err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = strings.TrimSpace(stdout.String())
		}
		if detail == "" {
			return nil, fmt.Errorf(
				"%s: %w",
				strings.Join(append([]string{name}, args...), " "),
				result.err,
			)
		}
		return nil, fmt.Errorf(
			"%s: %w: %s",
			strings.Join(append([]string{name}, args...), " "),
			result.err,
			detail,
		)
	}
	return stdout.Bytes(), nil
}

func (executor localProcessExecutor) Run(
	ctx context.Context,
	stdout io.Writer,
	stderr io.Writer,
	name string,
	args ...string,
) processResult {
	return executor.run(ctx, stdout, stderr, name, args...)
}

func (executor localProcessExecutor) run(
	ctx context.Context,
	stdout io.Writer,
	stderr io.Writer,
	name string,
	args ...string,
) processResult {
	if err := ctx.Err(); err != nil {
		return processResult{exitCode: -1, err: err}
	}
	command := exec.Command(name, args...)
	command.Dir = executor.directory
	if len(executor.environment) > 0 {
		command.Env = environmentWithOverrides(
			os.Environ(),
			executor.environment,
		)
	}
	command.Stdout = stdout
	command.Stderr = stderr
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := command.Start(); err != nil {
		return processResultFromError(err)
	}

	done := make(chan error, 1)
	go func() {
		done <- command.Wait()
	}()
	select {
	case err := <-done:
		return processResultFromError(err)
	case <-ctx.Done():
		waitErr := terminateProcessGroup(command.Process.Pid, done)
		if waitErr == nil {
			return processResultFromError(ctx.Err())
		}
		return processResultFromError(fmt.Errorf(
			"%w: %v",
			ctx.Err(),
			waitErr,
		))
	}
}

func terminateProcessGroup(pid int, done <-chan error) error {
	if pid <= 0 {
		return nil
	}
	// A graceful signal can let a test descendant outlive the `go` group
	// leader and perform work after a timed-out/interrupted observation. The
	// runner owns this isolated process group, so terminate the complete group
	// atomically at the observation boundary.
	_ = syscall.Kill(-pid, syscall.SIGKILL)
	return <-done
}

func processResultFromError(err error) processResult {
	if err == nil {
		return processResult{exitCode: 0}
	}
	exitCode := -1
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		exitCode = exitError.ExitCode()
	}
	return processResult{exitCode: exitCode, err: err}
}

type commandSpec struct {
	packageID racequalification.PackageID
	workKind  racequalification.WorkKind
	argv      []string
}

type commandJob struct {
	shardIndex   int
	commandIndex int
	spec         commandSpec
}

type commandJobResult struct {
	shardIndex   int
	commandIndex int
	observation  commandObservation
}

type commandObservation struct {
	PackageID  racequalification.PackageID   `json:"package_id"`
	Argv       []string                      `json:"argv"`
	StartedAt  time.Time                     `json:"started_at"`
	FinishedAt time.Time                     `json:"finished_at"`
	ExitCode   int                           `json:"exit_code"`
	Status     racequalification.ShardStatus `json:"status"`
	Error      string                        `json:"error,omitempty"`
}

type shardRun struct {
	Observation racequalification.ShardObservation `json:"observation"`
	StartedAt   time.Time                          `json:"started_at"`
	FinishedAt  time.Time                          `json:"finished_at"`
	Commands    []commandObservation               `json:"commands"`
}

type runDocument struct {
	Schema     string                             `json:"schema"`
	Mode       string                             `json:"mode"`
	Status     string                             `json:"status"`
	StartedAt  time.Time                          `json:"started_at"`
	FinishedAt time.Time                          `json:"finished_at"`
	Plan       *racequalification.Plan            `json:"plan,omitempty"`
	Shards     []shardRun                         `json:"shards"`
	Aggregate  *racequalification.AggregateResult `json:"aggregate,omitempty"`
	Error      string                             `json:"error,omitempty"`
}

type application struct {
	executor commandExecutor
	now      func() time.Time
	stdout   io.Writer
	stderr   io.Writer
}

func main() {
	os.Exit(realMain())
}

func realMain() int {
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	root, err := findModuleRoot()
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		return 1
	}
	app := application{
		executor: localProcessExecutor{directory: root},
		now:      time.Now,
		stdout:   os.Stdout,
		stderr:   os.Stderr,
	}
	return runCLI(ctx, os.Args[1:], app, os.Stderr)
}

func runCLI(
	ctx context.Context,
	args []string,
	app application,
	errorOutput io.Writer,
) int {
	config, err := parseConfiguration(args, errorOutput)
	if err != nil {
		_, _ = fmt.Fprintln(errorOutput, err)
		return 2
	}
	if err := clearOutputPath(config.outputPath); err != nil {
		_, _ = fmt.Fprintf(
			errorOutput,
			"clear prior race qualification output: %v\n",
			err,
		)
		return 1
	}

	document, passed, runErr := app.run(ctx, config)
	if runErr == nil {
		if err := ctx.Err(); err != nil {
			runErr = fmt.Errorf("race qualification interrupted: %w", err)
			passed = false
		}
	}
	if runErr != nil {
		document.Status = string(racequalification.AggregateInvalid)
		document.Error = runErr.Error()
		_, _ = fmt.Fprintln(errorOutput, runErr)
	}
	if document.FinishedAt.IsZero() {
		document.FinishedAt = app.now().UTC()
	}
	if err := writeDocumentAtomically(config.outputPath, document); err != nil {
		_, _ = fmt.Fprintf(
			errorOutput,
			"write race qualification output: %v\n",
			err,
		)
		return 1
	}
	if runErr != nil || !passed {
		return 1
	}
	return 0
}

func parseConfiguration(
	args []string,
	errorOutput io.Writer,
) (configuration, error) {
	config := configuration{}
	flags := flag.NewFlagSet("race-shard", flag.ContinueOnError)
	flags.SetOutput(errorOutput)
	flags.StringVar(
		&config.mode,
		"mode",
		"",
		"plan, shard, all, or aggregate",
	)
	flags.IntVar(&config.shardCount, "shard-count", 12, "number of exact shards")
	flags.IntVar(&config.shardIndex, "shard-index", -1, "zero-based shard index")
	flags.IntVar(
		&config.parallelism,
		"parallelism",
		0,
		"maximum concurrent go test commands in all mode; 0 uses the bounded host-aware default",
	)
	flags.DurationVar(
		&config.packageTimeout,
		"package-timeout",
		90*time.Minute,
		"finite timeout for each go test invocation",
	)
	flags.StringVar(
		&config.outputPath,
		"output",
		"",
		"machine-readable JSON output path",
	)
	flags.Var(
		&config.inputPaths,
		"input",
		"shard run JSON input path; repeat in aggregate mode",
	)
	if err := flags.Parse(args); err != nil {
		return configuration{}, err
	}
	if flags.NArg() != 0 {
		return configuration{}, fmt.Errorf(
			"race-shard accepts no positional arguments",
		)
	}
	switch config.mode {
	case modePlan, modeShard, modeAll, modeAggregate:
	default:
		return configuration{}, fmt.Errorf(
			"--mode must be one of plan, shard, all, or aggregate",
		)
	}
	if config.shardCount <= 0 {
		return configuration{}, fmt.Errorf("--shard-count must be positive")
	}
	if config.parallelism < 0 {
		return configuration{}, fmt.Errorf("--parallelism must be non-negative")
	}
	if config.parallelism == 0 {
		config.parallelism = automaticCommandParallelism(runtime.GOMAXPROCS(0))
	}
	if config.packageTimeout <= 0 {
		return configuration{}, fmt.Errorf("--package-timeout must be positive")
	}
	if strings.TrimSpace(config.outputPath) == "" {
		return configuration{}, fmt.Errorf("--output is required")
	}
	if config.mode == modeShard {
		if config.shardIndex < 0 || config.shardIndex >= config.shardCount {
			return configuration{}, fmt.Errorf(
				"--shard-index must be inside [0,%d) in shard mode",
				config.shardCount,
			)
		}
	} else if config.shardIndex != -1 {
		return configuration{}, fmt.Errorf(
			"--shard-index is valid only in shard mode",
		)
	}
	if config.mode != modeAggregate && len(config.inputPaths) != 0 {
		return configuration{}, fmt.Errorf(
			"--input is valid only in aggregate mode",
		)
	}
	if config.mode == modeAggregate {
		outputPath, err := filepath.Abs(config.outputPath)
		if err != nil {
			return configuration{}, fmt.Errorf(
				"resolve aggregate output path: %w",
				err,
			)
		}
		outputPath = filepath.Clean(outputPath)
		for _, inputPath := range config.inputPaths {
			resolvedInput, err := filepath.Abs(inputPath)
			if err != nil {
				return configuration{}, fmt.Errorf(
					"resolve aggregate input path %q: %w",
					inputPath,
					err,
				)
			}
			if filepath.Clean(resolvedInput) == outputPath {
				return configuration{}, fmt.Errorf(
					"aggregate output must not replace input %q",
					inputPath,
				)
			}
		}
	}
	return config, nil
}

func automaticCommandParallelism(logicalCPUs int) int {
	perCommand := racequalification.CurrentCommandGOMAXPROCS
	if logicalCPUs >= 8 {
		return max(1, logicalCPUs*3/(4*perCommand))
	}
	return max(1, logicalCPUs*2/(3*perCommand))
}

func (app application) run(
	ctx context.Context,
	config configuration,
) (runDocument, bool, error) {
	if config.mode == modeAggregate {
		return app.aggregateDocuments(ctx, config)
	}
	document := runDocument{
		Schema:    runSchema,
		Mode:      config.mode,
		Status:    string(racequalification.AggregateInvalid),
		StartedAt: app.now().UTC(),
		Shards:    []shardRun{},
	}
	discovery, err := discover(ctx, app.executor)
	if err != nil {
		return document, false, err
	}
	built, err := racequalification.Build(discovery, config.shardCount)
	if err != nil {
		return document, false, fmt.Errorf(
			"build race qualification plan: %w",
			err,
		)
	}
	plan := built.Plan()
	document.Plan = &plan

	switch config.mode {
	case modePlan:
		document.Status = "planned"
		return document, true, nil
	case modeShard:
		shard, err := built.Shard(config.shardIndex)
		if err != nil {
			return document, false, err
		}
		run := app.executeShard(
			ctx,
			built.Digest(),
			shard,
			config.packageTimeout,
		)
		document.Shards = []shardRun{run}
		document.Status = string(run.Observation.Status)
		return document, run.Observation.Status == racequalification.ShardPassed, nil
	case modeAll:
		allModeApp, cleanup, err := app.withRaceSharedFixtureDirectory()
		if err != nil {
			return document, false, err
		}
		defer cleanup()
		runs := allModeApp.executeAll(
			ctx,
			built,
			config.parallelism,
			config.packageTimeout,
		)
		document.Shards = runs
		observations := make(
			[]racequalification.ShardObservation,
			len(runs),
		)
		for index, run := range runs {
			observations[index] = run.Observation
		}
		aggregate, err := racequalification.Aggregate(plan, observations)
		document.Aggregate = &aggregate
		document.Status = string(aggregate.Status)
		if err != nil {
			return document, false, fmt.Errorf(
				"aggregate race qualification result: %w",
				err,
			)
		}
		return document, aggregate.Status == racequalification.AggregatePassed, nil
	default:
		panic("configuration mode was not validated")
	}
}

func discover(
	ctx context.Context,
	executor commandExecutor,
) (racequalification.Discovery, error) {
	return discoverPackageClosure(
		ctx,
		executor,
		"./...",
		racequalification.CurrentSplitPackagePaths(),
	)
}

func discoverPackageClosure(
	ctx context.Context,
	executor commandExecutor,
	packagePattern string,
	splitPackagePaths []string,
) (racequalification.Discovery, error) {
	packageOutput, err := executor.Output(
		ctx,
		"go",
		"list",
		"-race",
		packagePattern,
	)
	if err != nil {
		return racequalification.Discovery{}, fmt.Errorf(
			"discover Go package closure: %w",
			err,
		)
	}
	rawPackages := nonEmptyLines(packageOutput)
	wholePackages := make(
		[]racequalification.PackageID,
		0,
		len(rawPackages),
	)
	requestedSplitPackages, err := canonicalSplitPackageSet(splitPackagePaths)
	if err != nil {
		return racequalification.Discovery{}, err
	}
	discoveredSplitPackages := make(map[string]struct{}, len(requestedSplitPackages))
	for _, rawPackage := range rawPackages {
		if isDesktopPackage(rawPackage) {
			continue
		}
		if _, split := requestedSplitPackages[rawPackage]; split {
			discoveredSplitPackages[rawPackage] = struct{}{}
			continue
		}
		packageID, err := racequalification.NewPackageID(rawPackage)
		if err != nil {
			return racequalification.Discovery{}, fmt.Errorf(
				"discover package %q: %w",
				rawPackage,
				err,
			)
		}
		wholePackages = append(wholePackages, packageID)
	}
	splitPackages := make(
		[]racequalification.SplitPackageDiscovery,
		0,
		len(requestedSplitPackages),
	)
	for _, splitPackagePath := range splitPackagePaths {
		if _, found := discoveredSplitPackages[splitPackagePath]; !found {
			return racequalification.Discovery{}, fmt.Errorf(
				"split package %q is absent from the Go package closure",
				splitPackagePath,
			)
		}
	}
	testLists, err := discoverSplitPackageTests(
		ctx,
		executor,
		splitPackagePaths,
	)
	if err != nil {
		return racequalification.Discovery{}, err
	}
	for index, splitPackagePath := range splitPackagePaths {
		splitPackage, err := racequalification.NewPackageID(splitPackagePath)
		if err != nil {
			return racequalification.Discovery{}, err
		}
		splitPackages = append(
			splitPackages,
			racequalification.SplitPackageDiscovery{
				Package:       splitPackage,
				TopLevelTests: testLists[index],
			},
		)
	}
	return racequalification.Discovery{
		WholePackages: wholePackages,
		SplitPackages: splitPackages,
	}, nil
}

type splitPackageTestResult struct {
	index  int
	output []byte
	err    error
}

func discoverSplitPackageTests(
	ctx context.Context,
	executor commandExecutor,
	packagePaths []string,
) ([][]racequalification.TopLevelTestID, error) {
	jobs := make(chan int)
	results := make(chan splitPackageTestResult, len(packagePaths))
	workerCount := min(listParallelism, len(packagePaths))
	var workers sync.WaitGroup
	for range workerCount {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for index := range jobs {
				output, err := executor.Output(
					ctx,
					"go",
					"test",
					"-race",
					"-list",
					"^(Test|Example|Fuzz)",
					packagePaths[index],
				)
				results <- splitPackageTestResult{
					index:  index,
					output: output,
					err:    err,
				}
			}
		}()
	}
	go func() {
		for index := range packagePaths {
			jobs <- index
		}
		close(jobs)
		workers.Wait()
		close(results)
	}()

	rawResults := make([]splitPackageTestResult, len(packagePaths))
	for result := range results {
		rawResults[result.index] = result
	}
	testLists := make(
		[][]racequalification.TopLevelTestID,
		len(packagePaths),
	)
	for index, result := range rawResults {
		packagePath := packagePaths[index]
		if result.err != nil {
			return nil, fmt.Errorf(
				"discover platform-visible tests for %q: %w",
				packagePath,
				result.err,
			)
		}
		testIDs, err := parseTopLevelTests(result.output, packagePath)
		if err != nil {
			return nil, err
		}
		testLists[index] = testIDs
	}
	return testLists, nil
}

func canonicalSplitPackageSet(
	packagePaths []string,
) (map[string]struct{}, error) {
	if len(packagePaths) == 0 {
		return nil, fmt.Errorf("at least one split package is required")
	}
	if !slices.IsSorted(packagePaths) {
		return nil, fmt.Errorf("split packages must be canonically sorted")
	}
	result := make(map[string]struct{}, len(packagePaths))
	for _, packagePath := range packagePaths {
		if _, exists := result[packagePath]; exists {
			return nil, fmt.Errorf("split package %q is repeated", packagePath)
		}
		result[packagePath] = struct{}{}
	}
	return result, nil
}

func nonEmptyLines(output []byte) []string {
	lines := strings.Split(strings.ReplaceAll(string(output), "\r\n", "\n"), "\n")
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}

func isDesktopPackage(packagePath string) bool {
	return strings.HasSuffix(packagePath, "/desktop") ||
		strings.Contains(packagePath, "/desktop/")
}

func parseTopLevelTests(
	output []byte,
	splitPackagePath string,
) ([]racequalification.TopLevelTestID, error) {
	lines := nonEmptyLines(output)
	tests := make([]racequalification.TopLevelTestID, 0, len(lines))
	for _, line := range lines {
		if !strings.HasPrefix(line, "Test") &&
			!strings.HasPrefix(line, "Example") &&
			!strings.HasPrefix(line, "Fuzz") {
			continue
		}
		testID, err := racequalification.NewTopLevelTestID(line)
		if err != nil {
			return nil, fmt.Errorf(
				"discover top-level test %q: %w",
				line,
				err,
			)
		}
		tests = append(tests, testID)
	}
	if len(tests) == 0 {
		return nil, fmt.Errorf(
			"split package %q exposed no top-level Test, Example, or Fuzz target",
			splitPackagePath,
		)
	}
	return tests, nil
}

func (app application) executeAll(
	ctx context.Context,
	plan racequalification.RaceQualificationPlan,
	parallelism int,
	packageTimeout time.Duration,
) []shardRun {
	projection := plan.Plan()
	runs := make([]shardRun, projection.ShardCount)
	commandSlots := make([][]commandObservation, projection.ShardCount)
	wholeJobs := make([]commandJob, 0)
	splitJobs := make([]commandJob, 0)
	for _, shard := range projection.Shards {
		run := shardRun{
			Observation: racequalification.ShardObservation{
				PlanDigest: plan.Digest(),
				ShardIndex: shard.Index,
				Status:     racequalification.ShardPassed,
			},
			Commands: []commandObservation{},
		}
		commands, err := commandsForShard(shard, packageTimeout)
		if err != nil {
			now := app.now().UTC()
			run.StartedAt = now
			run.FinishedAt = now
			run.Observation.Status = racequalification.ShardFailed
			run.Commands = []commandObservation{{
				PackageID:  "",
				Argv:       []string{},
				StartedAt:  now,
				FinishedAt: now,
				ExitCode:   -1,
				Status:     racequalification.ShardFailed,
				Error:      err.Error(),
			}}
			runs[shard.Index] = run
			continue
		}
		commandSlots[shard.Index] = make(
			[]commandObservation,
			len(commands),
		)
		for commandIndex, command := range commands {
			job := commandJob{
				shardIndex:   shard.Index,
				commandIndex: commandIndex,
				spec:         command,
			}
			if command.workKind == racequalification.SplitTopLevelTestWork {
				splitJobs = append(splitJobs, job)
				continue
			}
			wholeJobs = append(wholeJobs, job)
		}
		runs[shard.Index] = run
	}
	splitJobs = orderedSplitCommandJobs(splitJobs)
	jobCount := len(wholeJobs) + len(splitJobs)
	if jobCount == 0 {
		return runs
	}

	pools := commandWorkerPools(
		wholeJobs,
		splitJobs,
		parallelism,
	)
	results := make(chan commandJobResult, jobCount)
	var workers sync.WaitGroup
	startCommandWorkerPools(
		ctx,
		app,
		pools,
		packageTimeout,
		results,
		&workers,
	)
	go func() {
		workers.Wait()
		close(results)
	}()

	for result := range results {
		commandSlots[result.shardIndex][result.commandIndex] =
			result.observation
	}
	for shardIndex := range runs {
		if runs[shardIndex].Observation.Status !=
			racequalification.ShardPassed {
			continue
		}
		commands := commandSlots[shardIndex]
		runs[shardIndex].Commands = commands
		if len(commands) == 0 {
			now := app.now().UTC()
			runs[shardIndex].StartedAt = now
			runs[shardIndex].FinishedAt = now
			continue
		}
		runs[shardIndex].StartedAt = commands[0].StartedAt
		runs[shardIndex].FinishedAt = commands[0].FinishedAt
		for _, command := range commands {
			runs[shardIndex].StartedAt = earlierTime(
				runs[shardIndex].StartedAt,
				command.StartedAt,
			)
			runs[shardIndex].FinishedAt = laterTime(
				runs[shardIndex].FinishedAt,
				command.FinishedAt,
			)
			runs[shardIndex].Observation.Status = combinedShardStatus(
				runs[shardIndex].Observation.Status,
				command.Status,
			)
		}
	}
	return runs
}

type commandWorkerPool struct {
	jobs        []commandJob
	workerCount int
}

func orderedSplitCommandJobs(jobs []commandJob) []commandJob {
	ordered := append([]commandJob{}, jobs...)
	priority := racequalification.CurrentSplitPackageExecutionPriority()
	priorityByPackage := make(map[string]int, len(priority))
	for index, packagePath := range priority {
		priorityByPackage[packagePath] = index
	}
	slices.SortFunc(ordered, func(left, right commandJob) int {
		leftPackage := left.spec.packageID.String()
		rightPackage := right.spec.packageID.String()
		leftPriority, leftKnown := priorityByPackage[leftPackage]
		rightPriority, rightKnown := priorityByPackage[rightPackage]
		switch {
		case leftKnown && !rightKnown:
			return -1
		case !leftKnown && rightKnown:
			return 1
		case leftKnown && rightKnown && leftPriority != rightPriority:
			return leftPriority - rightPriority
		case !leftKnown && !rightKnown && leftPackage != rightPackage:
			return strings.Compare(leftPackage, rightPackage)
		case left.shardIndex != right.shardIndex:
			return left.shardIndex - right.shardIndex
		default:
			return left.commandIndex - right.commandIndex
		}
	})
	return ordered
}

func commandWorkerPools(
	wholeJobs []commandJob,
	splitJobs []commandJob,
	parallelism int,
) []commandWorkerPool {
	if len(wholeJobs) == 0 {
		return []commandWorkerPool{{
			jobs:        splitJobs,
			workerCount: min(parallelism, len(splitJobs)),
		}}
	}
	if len(splitJobs) == 0 {
		return []commandWorkerPool{{
			jobs:        wholeJobs,
			workerCount: min(parallelism, len(wholeJobs)),
		}}
	}
	if parallelism == 1 {
		jobs := append([]commandJob{}, splitJobs...)
		jobs = append(jobs, wholeJobs...)
		return []commandWorkerPool{{
			jobs:        jobs,
			workerCount: 1,
		}}
	}

	// Round the split-preferred share up. At the measured 12-core default the
	// per-command GOMAXPROCS budget yields three split-preferred and one
	// whole-preferred worker; the pools remain work-conserving once either
	// primary queue drains.
	splitWorkerCount := max(1, (parallelism*3+3)/4)
	splitWorkerCount = min(splitWorkerCount, len(splitJobs))
	wholeWorkerCount := parallelism - splitWorkerCount
	wholeWorkerCount = max(1, wholeWorkerCount)
	wholeWorkerCount = min(wholeWorkerCount, len(wholeJobs))
	remainingWorkers := parallelism -
		splitWorkerCount -
		wholeWorkerCount
	additionalWholeWorkers := min(
		remainingWorkers,
		len(wholeJobs)-wholeWorkerCount,
	)
	wholeWorkerCount += additionalWholeWorkers
	remainingWorkers -= additionalWholeWorkers
	additionalSplitWorkers := min(
		remainingWorkers,
		len(splitJobs)-splitWorkerCount,
	)
	splitWorkerCount += additionalSplitWorkers
	return []commandWorkerPool{
		{
			jobs:        splitJobs,
			workerCount: splitWorkerCount,
		},
		{
			jobs:        wholeJobs,
			workerCount: wholeWorkerCount,
		},
	}
}

func startCommandWorkerPools(
	ctx context.Context,
	app application,
	pools []commandWorkerPool,
	packageTimeout time.Duration,
	results chan<- commandJobResult,
	workers *sync.WaitGroup,
) {
	jobQueues := make([]chan commandJob, len(pools))
	for index, pool := range pools {
		jobQueue := make(chan commandJob, len(pool.jobs))
		for _, job := range pool.jobs {
			jobQueue <- job
		}
		close(jobQueue)
		jobQueues[index] = jobQueue
	}

	for index, pool := range pools {
		var fallback <-chan commandJob
		if len(pools) == 2 {
			fallback = jobQueues[1-index]
		}
		for range pool.workerCount {
			workers.Add(1)
			go func(
				primary <-chan commandJob,
				fallback <-chan commandJob,
			) {
				defer workers.Done()
				runCommandWorker(
					ctx,
					app,
					primary,
					fallback,
					packageTimeout,
					results,
				)
			}(jobQueues[index], fallback)
		}
	}
}

func runCommandWorker(
	ctx context.Context,
	app application,
	primary <-chan commandJob,
	fallback <-chan commandJob,
	packageTimeout time.Duration,
	results chan<- commandJobResult,
) {
	for job := range primary {
		executeCommandJob(ctx, app, job, packageTimeout, results)
	}
	if fallback == nil {
		return
	}
	for job := range fallback {
		executeCommandJob(ctx, app, job, packageTimeout, results)
	}
}

func executeCommandJob(
	ctx context.Context,
	app application,
	job commandJob,
	packageTimeout time.Duration,
	results chan<- commandJobResult,
) {
	observation := app.executeCommand(
		ctx,
		job.spec,
		packageTimeout,
	)
	results <- commandJobResult{
		shardIndex:   job.shardIndex,
		commandIndex: job.commandIndex,
		observation:  observation,
	}
}

func earlierTime(left time.Time, right time.Time) time.Time {
	if right.Before(left) {
		return right
	}
	return left
}

func laterTime(left time.Time, right time.Time) time.Time {
	if right.After(left) {
		return right
	}
	return left
}

func combinedShardStatus(
	current racequalification.ShardStatus,
	next racequalification.ShardStatus,
) racequalification.ShardStatus {
	priority := map[racequalification.ShardStatus]int{
		racequalification.ShardPassed:      0,
		racequalification.ShardFailed:      1,
		racequalification.ShardTimedOut:    2,
		racequalification.ShardInterrupted: 3,
	}
	if priority[next] > priority[current] {
		return next
	}
	return current
}

func (app application) executeShard(
	ctx context.Context,
	planDigest racequalification.PlanDigest,
	shard racequalification.Shard,
	packageTimeout time.Duration,
) shardRun {
	startedAt := app.now().UTC()
	run := shardRun{
		Observation: racequalification.ShardObservation{
			PlanDigest: planDigest,
			ShardIndex: shard.Index,
			Status:     racequalification.ShardPassed,
		},
		StartedAt: startedAt,
		Commands:  []commandObservation{},
	}
	commands, err := commandsForShard(shard, packageTimeout)
	if err != nil {
		run.Observation.Status = racequalification.ShardFailed
		run.Commands = append(run.Commands, commandObservation{
			PackageID:  "",
			Argv:       []string{},
			StartedAt:  startedAt,
			FinishedAt: app.now().UTC(),
			ExitCode:   -1,
			Status:     racequalification.ShardFailed,
			Error:      err.Error(),
		})
		run.FinishedAt = app.now().UTC()
		return run
	}

	for _, command := range commands {
		observation := app.executeCommand(ctx, command, packageTimeout)
		run.Commands = append(run.Commands, observation)
		run.Observation.Status = combinedShardStatus(
			run.Observation.Status,
			observation.Status,
		)
	}
	run.FinishedAt = app.now().UTC()
	return run
}

func (app application) executeCommand(
	ctx context.Context,
	command commandSpec,
	packageTimeout time.Duration,
) commandObservation {
	startedAt := app.now().UTC()
	if err := ctx.Err(); err != nil {
		return commandObservation{
			PackageID:  command.packageID,
			Argv:       append([]string{}, command.argv...),
			StartedAt:  startedAt,
			FinishedAt: app.now().UTC(),
			ExitCode:   -1,
			Status:     racequalification.ShardInterrupted,
			Error:      err.Error(),
		}
	}
	commandCtx, cancel := context.WithTimeout(ctx, packageTimeout)
	result := app.executor.Run(
		commandCtx,
		app.stdout,
		app.stderr,
		command.argv[0],
		command.argv[1:]...,
	)
	status := classifyProcessResult(ctx, commandCtx, result)
	cancel()
	observation := commandObservation{
		PackageID:  command.packageID,
		Argv:       append([]string{}, command.argv...),
		StartedAt:  startedAt,
		FinishedAt: app.now().UTC(),
		ExitCode:   result.exitCode,
		Status:     status,
	}
	if result.err != nil {
		observation.Error = result.err.Error()
	}
	return observation
}

func commandsForShard(
	shard racequalification.Shard,
	packageTimeout time.Duration,
) ([]commandSpec, error) {
	wholePackages := make([]racequalification.PackageID, 0, len(shard.Work))
	splitTests := make(
		map[racequalification.PackageID][]racequalification.TopLevelTestID,
	)
	for _, item := range shard.Work {
		switch item.Kind {
		case racequalification.WholePackageWork:
			wholePackages = append(wholePackages, item.Package)
		case racequalification.SplitTopLevelTestWork:
			splitTests[item.Package] = append(
				splitTests[item.Package],
				item.TopLevelTest,
			)
		default:
			return nil, fmt.Errorf(
				"shard %d contains unknown work kind %q",
				shard.Index,
				item.Kind,
			)
		}
	}
	slices.Sort(wholePackages)
	splitPackages := make(
		[]racequalification.PackageID,
		0,
		len(splitTests),
	)
	for packageID := range splitTests {
		splitPackages = append(splitPackages, packageID)
	}
	slices.Sort(splitPackages)

	commands := make(
		[]commandSpec,
		0,
		len(wholePackages)+len(splitPackages),
	)
	for _, packageID := range wholePackages {
		commands = append(commands, commandSpec{
			packageID: packageID,
			workKind:  racequalification.WholePackageWork,
			argv: goTestArgv(
				packageTimeout,
				"",
				packageID,
			),
		})
	}
	for _, packageID := range splitPackages {
		pattern, err := exactTopLevelPattern(splitTests[packageID])
		if err != nil {
			return nil, fmt.Errorf(
				"build split command for %q: %w",
				packageID,
				err,
			)
		}
		commands = append(commands, commandSpec{
			packageID: packageID,
			workKind:  racequalification.SplitTopLevelTestWork,
			argv: goTestArgv(
				packageTimeout,
				pattern,
				packageID,
			),
		})
	}
	return commands, nil
}

func goTestArgv(
	packageTimeout time.Duration,
	runPattern string,
	packageID racequalification.PackageID,
) []string {
	parallelismArgument := fmt.Sprintf(
		"-parallel=%d",
		racequalification.CurrentSubtestParallelism,
	)
	cpuArgument := fmt.Sprintf(
		"-cpu=%d",
		racequalification.CurrentCommandGOMAXPROCS,
	)
	argv := []string{
		"go",
		"test",
		"-race",
		"-count=1",
		cpuArgument,
		parallelismArgument,
		"-timeout=" + packageTimeout.String(),
		"-skip",
		racequalification.ConsolidatedP13SkipPattern,
	}
	if runPattern != "" {
		argv = append(argv, "-run", runPattern)
	}
	return append(argv, packageID.String())
}

func exactTopLevelPattern(
	tests []racequalification.TopLevelTestID,
) (string, error) {
	if len(tests) == 0 {
		return "", fmt.Errorf("at least one top-level test is required")
	}
	names := make([]string, len(tests))
	for index, testID := range tests {
		validated, err := racequalification.NewTopLevelTestID(testID.String())
		if err != nil {
			return "", err
		}
		names[index] = regexp.QuoteMeta(validated.String())
	}
	slices.Sort(names)
	return "^(?:" + strings.Join(names, "|") + ")$", nil
}

func classifyProcessResult(
	parentCtx context.Context,
	commandCtx context.Context,
	result processResult,
) racequalification.ShardStatus {
	if parentCtx.Err() != nil {
		return racequalification.ShardInterrupted
	}
	if errors.Is(commandCtx.Err(), context.DeadlineExceeded) {
		return racequalification.ShardTimedOut
	}
	if result.err != nil || result.exitCode != 0 {
		return racequalification.ShardFailed
	}
	return racequalification.ShardPassed
}

func writeDocumentAtomically(path string, document runDocument) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return fmt.Errorf("create output directory %q: %w", directory, err)
	}
	temporary, err := os.CreateTemp(
		directory,
		"."+filepath.Base(path)+".tmp-*",
	)
	if err != nil {
		return fmt.Errorf("create temporary output: %w", err)
	}
	temporaryPath := temporary.Name()
	removeTemporary := true
	defer func() {
		if removeTemporary {
			_ = os.Remove(temporaryPath)
		}
	}()

	encoder := json.NewEncoder(temporary)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(document); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("encode output: %w", err)
	}
	if err := temporary.Chmod(0o644); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("set output mode: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("sync output: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close output: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace output %q: %w", path, err)
	}
	removeTemporary = false
	return nil
}

func clearOutputPath(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect output %q: %w", path, err)
	}
	if info.IsDir() {
		return fmt.Errorf("output %q is a directory", path)
	}
	if !info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0 {
		return fmt.Errorf(
			"output %q is neither a regular file nor a symbolic link",
			path,
		)
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("remove prior output %q: %w", path, err)
	}
	return nil
}

func findModuleRoot() (string, error) {
	directory, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve current directory: %w", err)
	}
	for {
		info, err := os.Stat(filepath.Join(directory, "go.mod"))
		if err == nil && !info.IsDir() {
			return directory, nil
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("inspect module root candidate: %w", err)
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			return "", fmt.Errorf(
				"find module root from %q: go.mod is absent",
				directory,
			)
		}
		directory = parent
	}
}
