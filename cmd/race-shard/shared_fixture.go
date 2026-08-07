package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/m0n0x41d/haft/internal/racequalification"
)

const raceSharedFixtureDirectoryPrefix = "haft-race-shared-fixtures-"

type environmentOverrideExecutor interface {
	commandExecutor
	withEnvironmentOverride(string, string) commandExecutor
}

func (executor localProcessExecutor) withEnvironmentOverride(
	name string,
	value string,
) commandExecutor {
	executor.environment = environmentWithOverrides(
		executor.environment,
		[]string{name + "=" + value},
	)
	return executor
}

func (app application) withRaceSharedFixtureDirectory() (
	application,
	func(),
	error,
) {
	executor, supported := app.executor.(environmentOverrideExecutor)
	if !supported {
		return app, func() {}, nil
	}
	root, err := os.MkdirTemp("", raceSharedFixtureDirectoryPrefix)
	if err != nil {
		return application{}, nil, fmt.Errorf(
			"create race shared fixture directory: %w",
			err,
		)
	}
	app.executor = executor.withEnvironmentOverride(
		racequalification.SharedFixtureDirectoryEnvironment,
		root,
	)
	cleanup := func() {
		if strings.HasPrefix(
			filepath.Base(root),
			raceSharedFixtureDirectoryPrefix,
		) {
			_ = os.RemoveAll(root)
		}
	}
	return app, cleanup, nil
}

func environmentWithOverrides(
	base []string,
	overrides []string,
) []string {
	overrideNames := make(map[string]struct{}, len(overrides))
	for _, override := range overrides {
		name, _, found := strings.Cut(override, "=")
		if found && name != "" {
			overrideNames[name] = struct{}{}
		}
	}
	result := make([]string, 0, len(base)+len(overrides))
	for _, entry := range base {
		name, _, found := strings.Cut(entry, "=")
		if found {
			if _, overridden := overrideNames[name]; overridden {
				continue
			}
		}
		result = append(result, entry)
	}
	return append(result, overrides...)
}
