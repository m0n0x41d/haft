package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/m0n0x41d/haft/internal/racequalification"
)

const maxAggregateInputBytes int64 = 64 << 20

func (app application) aggregateDocuments(
	ctx context.Context,
	config configuration,
) (runDocument, bool, error) {
	document := runDocument{
		Schema:    runSchema,
		Mode:      modeAggregate,
		Status:    string(racequalification.AggregateInvalid),
		StartedAt: app.now().UTC(),
		Shards:    []shardRun{},
	}
	if len(config.inputPaths) == 0 {
		return document, false, fmt.Errorf(
			"aggregate requires at least one --input",
		)
	}

	inputPaths := append([]string{}, config.inputPaths...)
	slices.Sort(inputPaths)
	var expectedPlan *racequalification.Plan
	for _, inputPath := range inputPaths {
		if err := ctx.Err(); err != nil {
			return document, false, fmt.Errorf(
				"aggregate interrupted: %w",
				err,
			)
		}
		input, err := readRunDocumentStrict(ctx, inputPath)
		if err != nil {
			return document, false, fmt.Errorf(
				"read aggregate input %q: %w",
				inputPath,
				err,
			)
		}
		if err := validateShardInputDocument(
			input,
			config.shardCount,
			config.packageTimeout,
		); err != nil {
			return document, false, fmt.Errorf(
				"validate aggregate input %q: %w",
				inputPath,
				err,
			)
		}
		if expectedPlan == nil {
			plan := *input.Plan
			expectedPlan = &plan
			document.Plan = expectedPlan
			document.StartedAt = input.StartedAt
			document.FinishedAt = input.FinishedAt
		} else {
			equal, err := plansEqual(*expectedPlan, *input.Plan)
			if err != nil {
				return document, false, fmt.Errorf(
					"compare aggregate input plan %q: %w",
					inputPath,
					err,
				)
			}
			if !equal {
				return document, false, fmt.Errorf(
					"aggregate input %q carries a different validated plan",
					inputPath,
				)
			}
			document.StartedAt = earlierTime(
				document.StartedAt,
				input.StartedAt,
			)
			document.FinishedAt = laterTime(
				document.FinishedAt,
				input.FinishedAt,
			)
		}
		document.Shards = append(document.Shards, input.Shards[0])
	}
	if err := ctx.Err(); err != nil {
		return document, false, fmt.Errorf(
			"aggregate interrupted: %w",
			err,
		)
	}

	slices.SortFunc(document.Shards, func(left, right shardRun) int {
		return left.Observation.ShardIndex - right.Observation.ShardIndex
	})
	observations := make(
		[]racequalification.ShardObservation,
		len(document.Shards),
	)
	for index, run := range document.Shards {
		observations[index] = run.Observation
	}
	aggregate, err := racequalification.Aggregate(
		*expectedPlan,
		observations,
	)
	document.Aggregate = &aggregate
	document.Status = string(aggregate.Status)
	if err != nil {
		return document, false, fmt.Errorf(
			"aggregate distributed race qualification result: %w",
			err,
		)
	}
	return document,
		aggregate.Status == racequalification.AggregatePassed,
		nil
}

func readRunDocumentStrict(
	ctx context.Context,
	path string,
) (runDocument, error) {
	if err := ctx.Err(); err != nil {
		return runDocument{}, err
	}
	pathInfo, err := os.Lstat(path)
	if err != nil {
		return runDocument{}, err
	}
	if !pathInfo.Mode().IsRegular() &&
		pathInfo.Mode()&os.ModeSymlink == 0 {
		return runDocument{}, fmt.Errorf(
			"input must be a regular file or a link to one",
		)
	}
	file, err := os.OpenFile(
		path,
		os.O_RDONLY|syscall.O_NONBLOCK,
		0,
	)
	if err != nil {
		return runDocument{}, err
	}
	defer func() { _ = file.Close() }()

	fileInfo, err := file.Stat()
	if err != nil {
		return runDocument{}, fmt.Errorf("inspect opened input: %w", err)
	}
	if !fileInfo.Mode().IsRegular() {
		return runDocument{}, fmt.Errorf(
			"opened input must be a regular file",
		)
	}
	if fileInfo.Size() > maxAggregateInputBytes {
		return runDocument{}, fmt.Errorf(
			"input is %d bytes, maximum is %d",
			fileInfo.Size(),
			maxAggregateInputBytes,
		)
	}
	encoded, err := io.ReadAll(io.LimitReader(
		contextBoundReader{ctx: ctx, reader: file},
		maxAggregateInputBytes+1,
	))
	if err != nil {
		return runDocument{}, fmt.Errorf("read JSON: %w", err)
	}
	if int64(len(encoded)) > maxAggregateInputBytes {
		return runDocument{}, fmt.Errorf(
			"input exceeds %d bytes",
			maxAggregateInputBytes,
		)
	}
	if err := ctx.Err(); err != nil {
		return runDocument{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	var document runDocument
	if err := decoder.Decode(&document); err != nil {
		return runDocument{}, fmt.Errorf("decode JSON: %w", err)
	}
	var trailing json.RawMessage
	err = decoder.Decode(&trailing)
	if err == nil {
		return runDocument{}, fmt.Errorf("decode JSON: trailing value")
	}
	if !errors.Is(err, io.EOF) {
		return runDocument{}, fmt.Errorf("decode JSON trailer: %w", err)
	}
	return document, nil
}

type contextBoundReader struct {
	ctx    context.Context
	reader io.Reader
}

func (reader contextBoundReader) Read(buffer []byte) (int, error) {
	if err := reader.ctx.Err(); err != nil {
		return 0, err
	}
	return reader.reader.Read(buffer)
}

func validateShardInputDocument(
	document runDocument,
	shardCount int,
	packageTimeout time.Duration,
) error {
	if document.Schema != runSchema {
		return fmt.Errorf("schema must be %q", runSchema)
	}
	if document.Mode != modeShard {
		return fmt.Errorf("mode must be %q", modeShard)
	}
	if document.Plan == nil {
		return fmt.Errorf("validated plan is required")
	}
	if err := racequalification.Validate(*document.Plan); err != nil {
		return fmt.Errorf("validate plan: %w", err)
	}
	if document.Plan.ShardCount != shardCount {
		return fmt.Errorf(
			"plan shard count is %d, want %d",
			document.Plan.ShardCount,
			shardCount,
		)
	}
	if document.Shards == nil || len(document.Shards) != 1 {
		return fmt.Errorf(
			"shard document must contain exactly one explicit shard run",
		)
	}
	if document.Aggregate != nil {
		return fmt.Errorf("shard document must not contain an aggregate")
	}
	if document.Error != "" {
		return fmt.Errorf("shard document contains top-level error %q", document.Error)
	}
	if err := validateTimeWindow(
		"shard document",
		document.StartedAt,
		document.FinishedAt,
	); err != nil {
		return err
	}

	run := document.Shards[0]
	if err := validateShardRun(
		*document.Plan,
		run,
		packageTimeout,
	); err != nil {
		return err
	}
	if document.Status != string(run.Observation.Status) {
		return fmt.Errorf(
			"document status %q does not match shard status %q",
			document.Status,
			run.Observation.Status,
		)
	}
	if run.StartedAt.Before(document.StartedAt) ||
		run.FinishedAt.After(document.FinishedAt) {
		return fmt.Errorf("shard run timestamps escape document window")
	}
	return nil
}

func validateShardRun(
	plan racequalification.Plan,
	run shardRun,
	packageTimeout time.Duration,
) error {
	if run.Observation.PlanDigest != plan.PlanDigest {
		return fmt.Errorf(
			"shard %d plan digest is %q, want %q",
			run.Observation.ShardIndex,
			run.Observation.PlanDigest,
			plan.PlanDigest,
		)
	}
	shardIndex := run.Observation.ShardIndex
	if shardIndex < 0 || shardIndex >= plan.ShardCount {
		return fmt.Errorf(
			"shard index %d is outside [0,%d)",
			shardIndex,
			plan.ShardCount,
		)
	}
	if !validEvidenceStatus(run.Observation.Status) {
		return fmt.Errorf(
			"shard %d has unknown status %q",
			shardIndex,
			run.Observation.Status,
		)
	}
	if err := validateTimeWindow(
		fmt.Sprintf("shard %d", shardIndex),
		run.StartedAt,
		run.FinishedAt,
	); err != nil {
		return err
	}
	if run.Commands == nil {
		return fmt.Errorf("shard %d commands must be an explicit array", shardIndex)
	}

	expected, err := commandsForShard(
		plan.Shards[shardIndex],
		packageTimeout,
	)
	if err != nil {
		return fmt.Errorf("build shard %d command policy: %w", shardIndex, err)
	}
	if len(run.Commands) != len(expected) {
		return fmt.Errorf(
			"shard %d has %d command observations, want %d",
			shardIndex,
			len(run.Commands),
			len(expected),
		)
	}

	combinedStatus := racequalification.ShardPassed
	for index, command := range run.Commands {
		want := expected[index]
		if command.PackageID != want.packageID {
			return fmt.Errorf(
				"shard %d command %d package is %q, want %q",
				shardIndex,
				index,
				command.PackageID,
				want.packageID,
			)
		}
		if !slices.Equal(command.Argv, want.argv) {
			return fmt.Errorf(
				"shard %d command %d argv does not match qualification policy",
				shardIndex,
				index,
			)
		}
		if err := validateCommandObservation(
			shardIndex,
			index,
			command,
		); err != nil {
			return err
		}
		if command.StartedAt.Before(run.StartedAt) ||
			command.FinishedAt.After(run.FinishedAt) {
			return fmt.Errorf(
				"shard %d command %d timestamps escape shard window",
				shardIndex,
				index,
			)
		}
		combinedStatus = combinedShardStatus(
			combinedStatus,
			command.Status,
		)
	}
	if run.Observation.Status != combinedStatus {
		return fmt.Errorf(
			"shard %d status is %q, want combined command status %q",
			shardIndex,
			run.Observation.Status,
			combinedStatus,
		)
	}
	return nil
}

func validateCommandObservation(
	shardIndex int,
	commandIndex int,
	command commandObservation,
) error {
	if !validEvidenceStatus(command.Status) {
		return fmt.Errorf(
			"shard %d command %d has unknown status %q",
			shardIndex,
			commandIndex,
			command.Status,
		)
	}
	if err := validateTimeWindow(
		fmt.Sprintf("shard %d command %d", shardIndex, commandIndex),
		command.StartedAt,
		command.FinishedAt,
	); err != nil {
		return err
	}
	if command.Status == racequalification.ShardPassed {
		if command.ExitCode != 0 || command.Error != "" {
			return fmt.Errorf(
				"shard %d command %d passed with exit code %d or error %q",
				shardIndex,
				commandIndex,
				command.ExitCode,
				command.Error,
			)
		}
		return nil
	}
	if command.ExitCode == 0 {
		return fmt.Errorf(
			"shard %d command %d has non-passing status with zero exit code",
			shardIndex,
			commandIndex,
		)
	}
	if strings.TrimSpace(command.Error) == "" {
		return fmt.Errorf(
			"shard %d command %d has non-passing status without an error",
			shardIndex,
			commandIndex,
		)
	}
	return nil
}

func validEvidenceStatus(status racequalification.ShardStatus) bool {
	switch status {
	case racequalification.ShardPassed,
		racequalification.ShardFailed,
		racequalification.ShardTimedOut,
		racequalification.ShardInterrupted:
		return true
	default:
		return false
	}
}

func validateTimeWindow(
	label string,
	startedAt time.Time,
	finishedAt time.Time,
) error {
	if startedAt.IsZero() || finishedAt.IsZero() {
		return fmt.Errorf("%s timestamps must be non-zero", label)
	}
	if finishedAt.Before(startedAt) {
		return fmt.Errorf("%s finishes before it starts", label)
	}
	return nil
}

func plansEqual(
	left racequalification.Plan,
	right racequalification.Plan,
) (bool, error) {
	leftJSON, err := json.Marshal(left)
	if err != nil {
		return false, err
	}
	rightJSON, err := json.Marshal(right)
	if err != nil {
		return false, err
	}
	return bytes.Equal(leftJSON, rightJSON), nil
}
