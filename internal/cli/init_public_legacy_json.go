package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/m0n0x41d/haft/internal/initplanning"
)

const legacyQuintProjectRootEnv = "QUINT_PROJECT_ROOT"

type publicLegacyJSONMCPShape string

const (
	publicLegacyJSONMCPClaude publicLegacyJSONMCPShape = "claude"
	publicLegacyJSONMCPCursor publicLegacyJSONMCPShape = "cursor"
	publicLegacyJSONMCPGemini publicLegacyJSONMCPShape = "gemini"
)

func currentPublicLegacyJSONMCPFragments(
	projection initplanning.HostAdapterProjection,
) ([]initplanning.ManagedFragment, error) {
	shape := publicLegacyJSONMCPShape("")
	switch {
	case projection.Host() == initplanning.HostClaude &&
		projection.Scope() == initplanning.ScopeProject:
		shape = publicLegacyJSONMCPClaude
	case projection.Host() == initplanning.HostCursor &&
		projection.Scope() == initplanning.ScopeProject:
		shape = publicLegacyJSONMCPCursor
	case projection.Host() == initplanning.HostGemini &&
		projection.Scope() == initplanning.ScopeUser:
		shape = publicLegacyJSONMCPGemini
	default:
		return nil, nil
	}
	desired, found, err := currentPublicJSONMCPFragment(projection)
	if err != nil || !found {
		return nil, err
	}
	expected := publicLegacyJSONMCPValue(
		shape,
		"quint-code",
		projection.ProjectRoot(),
	)
	content, err := json.Marshal(expected)
	if err != nil {
		return nil, fmt.Errorf(
			"encode known legacy JSON MCP fragment: %w",
			err,
		)
	}
	path := desired.Coordinate().CarrierPath()
	raw, readErr := os.ReadFile(path)
	if readErr == nil {
		observed, present, extractErr :=
			initplanning.ExtractJSONObjectEntry(
				raw,
				[]string{"mcpServers", "quint-code"},
				desired.Coordinate().MergeEdition(),
			)
		if extractErr != nil {
			return nil, fmt.Errorf(
				"inspect legacy JSON MCP configuration: %w",
				extractErr,
			)
		}
		if present && isPublicLegacyJSONMCPValue(
			observed,
			shape,
			projection.ProjectRoot(),
		) {
			content = observed
		}
	} else if !os.IsNotExist(readErr) {
		return nil, fmt.Errorf(
			"read legacy JSON MCP configuration: %w",
			readErr,
		)
	}
	fragment, err := initplanning.NewJSONObjectEntryFragment(
		path,
		initplanning.ComponentMCP,
		[]string{"mcpServers", "quint-code"},
		content,
		desired.CreateMode(),
		desired.Coordinate().MergeEdition(),
	)
	if err != nil {
		return nil, err
	}
	return []initplanning.ManagedFragment{fragment}, nil
}

func currentPublicJSONMCPFragment(
	projection initplanning.HostAdapterProjection,
) (initplanning.ManagedFragment, bool, error) {
	var result initplanning.ManagedFragment
	found := false
	for _, fragment := range projection.ManagedFragments() {
		coordinate := fragment.Coordinate()
		if coordinate.Kind() != initplanning.ManagedJSONObjectEntry ||
			coordinate.Selector() != "/mcpServers/haft" {
			continue
		}
		if found {
			return initplanning.ManagedFragment{}, false,
				fmt.Errorf(
					"host projection repeats its JSON MCP fragment",
				)
		}
		result = fragment
		found = true
	}
	return result, found, nil
}

func publicLegacyJSONMCPValue(
	shape publicLegacyJSONMCPShape,
	command string,
	projectRoot string,
) map[string]any {
	value := map[string]any{
		"command": command,
		"args":    []string{"serve"},
		"env": map[string]string{
			legacyQuintProjectRootEnv: projectRoot,
		},
	}
	if shape == publicLegacyJSONMCPGemini {
		value["cwd"] = projectRoot
		value["timeout"] = 30000
	}
	return value
}

func isPublicLegacyJSONMCPValue(
	content []byte,
	shape publicLegacyJSONMCPShape,
	projectRoot string,
) bool {
	var value map[string]any
	if err := json.Unmarshal(content, &value); err != nil {
		return false
	}
	command, ok := value["command"].(string)
	if !ok || !isLegacyQuintCommand(command) {
		return false
	}
	args, ok := value["args"].([]any)
	if !ok || len(args) != 1 || args[0] != "serve" {
		return false
	}
	for field := range value {
		if field != "command" &&
			field != "args" &&
			field != "cwd" &&
			field != "env" &&
			field != "timeout" {
			return false
		}
	}
	cwd, hasCWD := value["cwd"]
	if hasCWD {
		text, ok := cwd.(string)
		if !ok || !samePublicLegacyProjectRoot(
			text,
			projectRoot,
		) {
			return false
		}
	}
	envValue, hasEnv := value["env"]
	if hasEnv {
		env, ok := envValue.(map[string]any)
		root, rootOK := env[legacyQuintProjectRootEnv].(string)
		if !ok ||
			len(env) != 1 ||
			!rootOK ||
			!samePublicLegacyProjectRoot(root, projectRoot) {
			return false
		}
	}
	timeoutValue, hasTimeout := value["timeout"]
	timeout, timeoutOK := timeoutValue.(float64)
	if hasTimeout && !timeoutOK {
		return false
	}
	if shape == publicLegacyJSONMCPClaude {
		return !hasTimeout && (hasCWD || hasEnv)
	}
	if shape == publicLegacyJSONMCPCursor {
		return !hasTimeout
	}
	return shape == publicLegacyJSONMCPGemini &&
		hasCWD &&
		hasTimeout &&
		timeoutOK &&
		timeout == 30000
}

func isLegacyQuintCommand(command string) bool {
	if command == "quint-code" {
		return true
	}
	return filepath.IsAbs(command) &&
		filepath.Base(command) == "quint-code"
}

func samePublicLegacyProjectRoot(
	observed string,
	current string,
) bool {
	if filepath.Clean(observed) == filepath.Clean(current) {
		return true
	}
	observedResolved, observedErr := filepath.EvalSymlinks(observed)
	currentResolved, currentErr := filepath.EvalSymlinks(current)
	return observedErr == nil &&
		currentErr == nil &&
		filepath.Clean(observedResolved) ==
			filepath.Clean(currentResolved)
}
