package fpf

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type FPFEngineLint struct {
	Path    string `json:"path"`
	Line    int    `json:"line"`
	RuleID  string `json:"rule_id"`
	Message string `json:"message"`
}

func LintFPFEngineText(path string, content []byte) []FPFEngineLint {
	normalizedPath := filepath.ToSlash(path)
	if fpfEngineLintPathExcluded(normalizedPath) {
		return nil
	}

	lines := strings.Split(string(content), "\n")
	var lints []FPFEngineLint
	for index, line := range lines {
		lineNumber := index + 1
		lower := strings.ToLower(line)
		lints = append(lints, lintPatternPullLine(normalizedPath, lineNumber, line, lower)...)
		lints = append(lints, lintWeakAffordanceLine(normalizedPath, lineNumber, lower)...)
		lints = append(lints, lintPatternAtlasAuthorityLine(normalizedPath, lineNumber, lower)...)
		lints = append(lints, lintPatternUseMethodPackLine(normalizedPath, lineNumber, lower)...)
		lints = append(lints, lintMethodPackSourceAuthorityLine(normalizedPath, lineNumber, lower)...)
	}
	return lints
}

func LintFPFEngineFiles(root string, relPaths []string) ([]FPFEngineLint, error) {
	var lints []FPFEngineLint
	for _, relPath := range relPaths {
		normalized := filepath.ToSlash(relPath)
		if fpfEngineLintPathExcluded(normalized) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.Clean(relPath)))
		if err != nil {
			return nil, fmt.Errorf("read lint target %s: %w", relPath, err)
		}
		lints = append(lints, LintFPFEngineText(normalized, data)...)
	}
	return lints, nil
}

func fpfEngineLintPathExcluded(path string) bool {
	excludedFragments := []string{
		".context/external-review/",
		".context/_archived/",
		".context/ailev-blog-discussion-2026-07-02/",
		"repomix",
	}
	for _, fragment := range excludedFragments {
		if strings.Contains(path, fragment) {
			return true
		}
	}
	return false
}

func lintPatternPullLine(path string, lineNumber int, line, lower string) []FPFEngineLint {
	if !strings.Contains(line, "PatternPull") {
		return nil
	}
	if patternPullUseAllowed(lower) {
		return nil
	}
	return []FPFEngineLint{{
		Path:    path,
		Line:    lineNumber,
		RuleID:  "deprecated_patternpull_formal_term",
		Message: "PatternPull is deprecated as a formal term; use PatternUseGateway, PatternRecall, source_hydration, or PatternRouteSelector.",
	}}
}

func patternPullUseAllowed(lower string) bool {
	allowedFragments := []string{
		"deprecated",
		"deprecation",
		"historical",
		"informal alias",
		"migration note",
		"must not",
		"do not introduce",
		"forbidden",
		"legacy",
	}
	for _, fragment := range allowedFragments {
		if strings.Contains(lower, fragment) {
			return true
		}
	}
	return false
}

func lintWeakAffordanceLine(path string, lineNumber int, lower string) []FPFEngineLint {
	if !strings.Contains(lower, "weak_affordance") {
		return nil
	}
	return []FPFEngineLint{{
		Path:    path,
		Line:    lineNumber,
		RuleID:  "weak_affordance_deprecated",
		Message: "Use routing_affordance_candidate for internal-only matrix metadata.",
	}}
}

func lintPatternAtlasAuthorityLine(path string, lineNumber int, lower string) []FPFEngineLint {
	if !strings.Contains(lower, "patternatlas") {
		return nil
	}
	for _, phrase := range []string{
		"patternatlas supports route",
		"patternatlas supports routes",
		"patternatlas is evidence",
		"patternatlas approves",
		"patternatlas gate",
		"patternatlas library of principles",
	} {
		if strings.Contains(lower, phrase) && !negativeBoundarySentence(lower) {
			return []FPFEngineLint{{
				Path:    path,
				Line:    lineNumber,
				RuleID:  "patternatlas_authority_overclaim",
				Message: "PatternAtlas is a deterministic source-card substrate only, not route/evidence/approval/gate authority.",
			}}
		}
	}
	return nil
}

func lintPatternUseMethodPackLine(path string, lineNumber int, lower string) []FPFEngineLint {
	if !strings.Contains(lower, "patternuse") {
		return nil
	}
	for _, phrase := range []string{
		"patternuse opens methodrun",
		"patternuse closes methodrun",
		"patternuse satisfies methodpack gate",
		"patternuse satisfies methodpack gates",
	} {
		if strings.Contains(lower, phrase) && !negativeBoundarySentence(lower) {
			return []FPFEngineLint{{
				Path:    path,
				Line:    lineNumber,
				RuleID:  "patternuse_methodpack_authority_overclaim",
				Message: "PatternUse is advisory and cannot open/close MethodRuns or satisfy MethodPack gates.",
			}}
		}
	}
	return nil
}

func lintMethodPackSourceAuthorityLine(path string, lineNumber int, lower string) []FPFEngineLint {
	if !strings.Contains(lower, "methodpack") {
		return nil
	}
	for _, phrase := range []string{
		"methodpack is fpf source",
		"methodpack is dpf source",
		"methodpack proves",
	} {
		if strings.Contains(lower, phrase) && !negativeBoundarySentence(lower) {
			return []FPFEngineLint{{
				Path:    path,
				Line:    lineNumber,
				RuleID:  "methodpack_source_authority_overclaim",
				Message: "MethodPack is a task-local work/evidence harness, not FPF/DPF source authority or proof.",
			}}
		}
	}
	return nil
}

func negativeBoundarySentence(lower string) bool {
	for _, fragment := range []string{
		"not ",
		"never",
		"cannot",
		"must not",
		"do not",
		"does not",
		"can't",
		"не ",
	} {
		if strings.Contains(lower, fragment) {
			return true
		}
	}
	return false
}
