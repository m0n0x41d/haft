package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type projectPathRepairStrategy string

const (
	projectPathRepairLiteral        projectPathRepairStrategy = "literal"
	projectPathRepairJSONString     projectPathRepairStrategy = "json_string"
	projectPathRepairCodexProject   projectPathRepairStrategy = "codex_project"
	projectPathRepairManualRequired projectPathRepairStrategy = "manual_required"
)

type projectPathCarrier struct {
	Label    string
	Path     string
	Strategy projectPathRepairStrategy
}

type projectPathCarrierResult struct {
	Label       string
	Path        string
	Strategy    projectPathRepairStrategy
	Occurrences int
	Repairable  bool
	Changed     bool
	Message     string
}

func projectPathCarrierCandidates(homeDir string, projectRoot string) []projectPathCarrier {
	candidates := []projectPathCarrier{
		{
			Label:    "Codex user project trust",
			Path:     filepath.Join(homeDir, ".codex", "config.toml"),
			Strategy: projectPathRepairCodexProject,
		},
		{
			Label:    "Claude user project state",
			Path:     filepath.Join(homeDir, ".claude.json"),
			Strategy: projectPathRepairJSONString,
		},
	}

	projectLocal := []projectPathCarrier{
		{
			Label:    "Claude project MCP",
			Path:     filepath.Join(projectRoot, ".mcp.json"),
			Strategy: projectPathRepairLiteral,
		},
		{
			Label:    "Cursor project MCP",
			Path:     filepath.Join(projectRoot, ".cursor", "mcp.json"),
			Strategy: projectPathRepairLiteral,
		},
		{
			Label:    "Codex project MCP",
			Path:     filepath.Join(projectRoot, ".codex", "config.toml"),
			Strategy: projectPathRepairLiteral,
		},
		{
			Label:    "OpenCode project config",
			Path:     filepath.Join(projectRoot, "opencode.json"),
			Strategy: projectPathRepairLiteral,
		},
	}

	return append(candidates, projectLocal...)
}

func auditProjectPathCarriers(carriers []projectPathCarrier, oldRoot string) ([]projectPathCarrierResult, error) {
	results := make([]projectPathCarrierResult, 0, len(carriers))
	for _, carrier := range carriers {
		data, err := os.ReadFile(carrier.Path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}

		content := string(data)
		result := projectPathCarrierResult{
			Label:       carrier.Label,
			Path:        carrier.Path,
			Strategy:    carrier.Strategy,
			Occurrences: countProjectPathOccurrences(content, carrier.Strategy, oldRoot),
			Repairable:  carrier.Strategy != projectPathRepairManualRequired,
		}
		results = append(results, result)
	}

	return results, nil
}

func repairProjectPathCarriers(results []projectPathCarrierResult, oldRoot string, newRoot string) ([]projectPathCarrierResult, error) {
	repaired := make([]projectPathCarrierResult, 0, len(results))
	for _, result := range results {
		if result.Occurrences == 0 {
			repaired = append(repaired, result)
			continue
		}
		if !result.Repairable {
			repaired = append(repaired, result)
			continue
		}

		changed, message, err := repairProjectPathCarrier(result.Path, result.Strategy, oldRoot, newRoot)
		if err != nil {
			return nil, err
		}

		result.Changed = changed
		result.Message = message
		repaired = append(repaired, result)
	}

	return repaired, nil
}

func countProjectPathOccurrences(content string, strategy projectPathRepairStrategy, oldRoot string) int {
	switch strategy {
	case projectPathRepairCodexProject:
		return strings.Count(content, codexProjectTableHeader(oldRoot))
	case projectPathRepairJSONString:
		return strings.Count(content, jsonStringLiteral(oldRoot))
	default:
		return strings.Count(content, oldRoot)
	}
}

func repairProjectPathCarrier(path string, strategy projectPathRepairStrategy, oldRoot string, newRoot string) (bool, string, error) {
	switch strategy {
	case projectPathRepairCodexProject:
		return repairCodexProjectRoot(path, oldRoot, newRoot)
	case projectPathRepairJSONString:
		return repairJSONStringProjectRoot(path, oldRoot, newRoot)
	case projectPathRepairLiteral:
		return repairLiteralProjectRoot(path, oldRoot, newRoot)
	default:
		return false, "manual repair required", nil
	}
}

func repairCodexProjectRoot(path string, oldRoot string, newRoot string) (bool, string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return false, "", err
	}

	text := string(content)
	oldHeader := codexProjectTableHeader(oldRoot)
	newHeader := codexProjectTableHeader(newRoot)
	if !strings.Contains(text, oldHeader) {
		return false, "old project table not found", nil
	}

	next := strings.Replace(text, oldHeader, newHeader, 1)
	message := "renamed project table"
	if strings.Contains(text, newHeader) {
		next = removeExactTomlSection(text, oldHeader)
		message = "removed stale project table because current root already exists"
	}
	if next == text {
		return false, message, nil
	}

	return true, message, os.WriteFile(path, []byte(next), 0o644)
}

func repairJSONStringProjectRoot(path string, oldRoot string, newRoot string) (bool, string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return false, "", err
	}

	oldLiteral := jsonStringLiteral(oldRoot)
	newLiteral := jsonStringLiteral(newRoot)
	text := string(content)
	next := strings.ReplaceAll(text, oldLiteral, newLiteral)
	if next == text {
		return false, "exact JSON string not found", nil
	}

	if !json.Valid([]byte(next)) {
		return false, "", fmt.Errorf("repair would make %s invalid JSON", path)
	}

	return true, "rewrote exact JSON string literal(s)", os.WriteFile(path, []byte(next), 0o644)
}

func repairLiteralProjectRoot(path string, oldRoot string, newRoot string) (bool, string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return false, "", err
	}

	text := string(content)
	next := strings.ReplaceAll(text, oldRoot, newRoot)
	if next == text {
		return false, "old project root not found", nil
	}

	return true, "rewrote exact text occurrence(s)", os.WriteFile(path, []byte(next), 0o644)
}

func codexProjectTableHeader(projectRoot string) string {
	return "[projects." + strconv.Quote(projectRoot) + "]"
}

func jsonStringLiteral(value string) string {
	return strconv.Quote(value)
}

func removeExactTomlSection(content string, header string) string {
	lines := strings.Split(content, "\n")
	next := make([]string, 0, len(lines))
	skipping := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.Contains(trimmed, "]") {
			skipping = trimmed == header
			if skipping {
				continue
			}
		}
		if !skipping {
			next = append(next, line)
		}
	}

	return strings.TrimLeft(strings.Join(next, "\n"), "\n")
}
