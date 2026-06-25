package overseer

import (
	"path/filepath"
	"sort"
	"strings"
)

const (
	reviewModeInvariant = "invariant_conformance"
	reviewModeSpec      = "spec_conformance"
	reviewModeTestGap   = "test_or_verification_gap"
	reviewModeSecurity  = "security_review"
)

type PathPolicy struct {
	Path string
}

func DefaultContextBudget() ContextBudget {
	return ContextBudget{
		MaxPacketBytes:        24000,
		MaxChangedFilesListed: 30,
		MaxInlineDiffBytes:    12000,
		MaxArtifactRefs:       12,
		FullSourcePolicy:      "fetch_on_demand",
		OmissionPolicy:        "summarize_and_handle",
	}
}

func DefaultProducer(version string) Producer {
	return Producer{
		Tool:    DefaultToolName,
		Version: strings.TrimSpace(version),
		PolicyVersions: map[string]string{
			"risk":  RiskPolicyVersion,
			"scope": ScopePolicyVersion,
		},
	}
}

func DefaultReviewAuthority() ReviewAuthority {
	return ReviewAuthority{
		Status: "advisory_only",
		Cannot: []string{
			"approve",
			"merge",
			"deploy",
			"decide",
			"commission",
			"rebaseline",
		},
	}
}

func AdvisoryFindingDefaults(finding ReviewFinding) ReviewFinding {
	finding.SupportPosture = "advisory_unverified"
	finding.CountsForREff = false
	return finding
}

func MatchPathPolicies(filePath string, policies []PathPolicy) []string {
	matches := make([]string, 0)
	normalizedPath := normalizePath(filePath)

	for _, policy := range policies {
		pattern := normalizePath(policy.Path)
		if pattern == "" {
			continue
		}
		if pathPolicyMatches(normalizedPath, pattern) {
			matches = append(matches, pattern)
		}
	}

	return stableUniqueStrings(matches)
}

func AssessRisk(changedFiles []ChangedFile) Risk {
	rules := make([]RiskRule, 0)

	for _, changedFile := range changedFiles {
		rules = append(rules, riskRulesForFile(changedFile)...)
	}

	rules = compactRiskRules(rules)
	score := riskScore(rules, changedFiles)

	level := "low"
	if score >= 8 {
		level = "high"
	} else if score >= 4 {
		level = "medium"
	}

	llmReview := "off"
	if score >= 4 {
		llmReview = "eligible"
	}

	return Risk{
		Level:          level,
		Score:          score,
		PolicyVersion:  RiskPolicyVersion,
		RulesTriggered: rules,
		LLMReview:      llmReview,
	}
}

func ReviewRequestForRisk(risk Risk) ReviewRequest {
	modes := make([]string, 0)
	for _, rule := range risk.RulesTriggered {
		modes = append(modes, rule.ReviewModesAdded...)
	}

	return ReviewRequest{
		Authority:     "advisory_only",
		Modes:         stableUniqueStrings(modes),
		MustNotReview: defaultMustNotReview(),
		HumanBound:    true,
	}
}

func riskRulesForFile(changedFile ChangedFile) []RiskRule {
	rules := make([]RiskRule, 0)
	path := normalizePath(changedFile.Path)

	if isGovernedInitSurface(path) {
		rules = append(rules, RiskRule{
			RuleID:           "governed_init_surface_changed",
			Source:           "path_policy",
			Basis:            path,
			ReviewModesAdded: []string{reviewModeInvariant},
		})
	}

	if len(changedFile.Governance.PathPolicies) > 0 {
		rules = append(rules, RiskRule{
			RuleID:           "workflow_path_policy_touched",
			Source:           "workflow",
			Basis:            path + " matches " + strings.Join(changedFile.Governance.PathPolicies, ", "),
			ReviewModesAdded: []string{reviewModeInvariant},
		})
	}

	if len(changedFile.Governance.AffectedDecisions) > 0 {
		rules = append(rules, RiskRule{
			RuleID:           "active_decision_linked_code_changed",
			Source:           "artifact_graph",
			Basis:            path,
			ReviewModesAdded: []string{reviewModeInvariant, reviewModeTestGap},
		})
	}

	if len(changedFile.Governance.AffectedSpecSections) > 0 {
		rules = append(rules, RiskRule{
			RuleID:           "spec_section_linked_code_changed",
			Source:           "spec_graph",
			Basis:            path,
			ReviewModesAdded: []string{reviewModeSpec, reviewModeTestGap},
		})
	}

	if isSpecCarrierPath(path) {
		rules = append(rules, RiskRule{
			RuleID:           "spec_carrier_changed",
			Source:           "path_heuristic",
			Basis:            path,
			ReviewModesAdded: []string{reviewModeSpec, reviewModeTestGap},
		})
	}

	if isSecuritySensitivePath(path) {
		rules = append(rules, RiskRule{
			RuleID:           "security_sensitive_surface_changed",
			Source:           "path_heuristic",
			Basis:            path,
			ReviewModesAdded: []string{reviewModeSecurity},
		})
	}

	if len(rules) == 0 && isDocsOrTestOnly(path) {
		rules = append(rules, RiskRule{
			RuleID:           "low_risk_docs_or_tests_only",
			Source:           "path_heuristic",
			Basis:            path,
			ReviewModesAdded: []string{},
		})
	}

	return rules
}

func riskScore(rules []RiskRule, changedFiles []ChangedFile) int {
	score := 1
	if len(changedFiles) > 10 {
		score += 1
	}

	for _, rule := range rules {
		switch rule.RuleID {
		case "governed_init_surface_changed":
			score += 6
		case "security_sensitive_surface_changed":
			score += 7
		case "active_decision_linked_code_changed":
			score += 4
		case "spec_carrier_changed":
			score += 3
		case "spec_section_linked_code_changed":
			score += 3
		case "workflow_path_policy_touched":
			score += 3
		}
	}

	if score > 10 {
		return 10
	}
	return score
}

func compactRiskRules(rules []RiskRule) []RiskRule {
	byKey := make(map[string]RiskRule)

	for _, rule := range rules {
		key := rule.RuleID + "\x00" + rule.Basis
		existing, ok := byKey[key]
		if !ok {
			rule.ReviewModesAdded = stableUniqueStrings(rule.ReviewModesAdded)
			byKey[key] = rule
			continue
		}
		existing.ReviewModesAdded = stableUniqueStrings(append(existing.ReviewModesAdded, rule.ReviewModesAdded...))
		byKey[key] = existing
	}

	out := make([]RiskRule, 0, len(byKey))
	for _, rule := range byKey {
		out = append(out, rule)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].RuleID == out[j].RuleID {
			return out[i].Basis < out[j].Basis
		}
		return out[i].RuleID < out[j].RuleID
	})

	return out
}

func defaultMustNotReview() []string {
	return []string{
		"style-only preferences",
		"unrelated files",
		"unaffected historical drift",
		"merge readiness",
		"deployment approval",
		"DecisionRecord creation",
		"WorkCommission creation",
		"rebaseline authority",
	}
}

func pathPolicyMatches(filePath string, pattern string) bool {
	if strings.HasSuffix(pattern, "/**") {
		prefix := strings.TrimSuffix(pattern, "/**")
		return filePath == prefix || strings.HasPrefix(filePath, prefix+"/")
	}

	if ok, _ := filepath.Match(pattern, filePath); ok {
		return true
	}

	return filePath == pattern
}

func isGovernedInitSurface(path string) bool {
	switch path {
	case "internal/cli/init.go":
		return true
	case "internal/cli/claude_md_template.md":
		return true
	case "AGENTS.md":
		return true
	default:
		return strings.Contains(path, "init") && strings.Contains(path, "template")
	}
}

func isSecuritySensitivePath(path string) bool {
	keywords := []string{
		"auth",
		"oauth",
		"token",
		"secret",
		"crypto",
		"permission",
		"webhook",
		"network",
	}
	for _, keyword := range keywords {
		if strings.Contains(path, keyword) {
			return true
		}
	}

	switch filepath.Base(path) {
	case "go.mod", "go.sum", "package.json", "package-lock.json":
		return true
	default:
		return false
	}
}

func isSpecCarrierPath(path string) bool {
	switch normalizePath(path) {
	case ".haft/specs/target-system.md",
		".haft/specs/enabling-system.md",
		".haft/specs/term-map.md":
		return true
	}
	return false
}

func isDocsOrTestOnly(path string) bool {
	if strings.HasSuffix(path, "_test.go") {
		return true
	}
	ext := filepath.Ext(path)
	switch ext {
	case ".md", ".txt", ".rst":
		return true
	default:
		return false
	}
}

func normalizePath(value string) string {
	return strings.Trim(strings.ReplaceAll(value, "\\", "/"), "/")
}

func stableUniqueStrings(values []string) []string {
	seen := make(map[string]bool)
	out := make([]string, 0, len(values))

	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		out = append(out, trimmed)
	}

	sort.Strings(out)
	return out
}
