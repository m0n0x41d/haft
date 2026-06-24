package project

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type CarrierAuthorityClass string

const (
	CarrierAuthorityCurrent       CarrierAuthorityClass = "current_authority"
	CarrierAuthoritySupport       CarrierAuthorityClass = "support_material"
	CarrierAuthorityCompatibility CarrierAuthorityClass = "compatibility_carrier"
	CarrierAuthorityProvenance    CarrierAuthorityClass = "provenance"
	CarrierAuthorityArchive       CarrierAuthorityClass = "archive"
	CarrierAuthoritySidekick      CarrierAuthorityClass = "external_sidekick_out_of_scope"
)

type CarrierManifestEntry struct {
	ID                string                `json:"id"`
	PathPattern       string                `json:"path_pattern"`
	AuthorityClass    CarrierAuthorityClass `json:"authority_class"`
	Surface           string                `json:"surface"`
	Current           bool                  `json:"current"`
	Normativity       string                `json:"normativity"`
	DeadSurfacePolicy string                `json:"dead_surface_policy,omitempty"`
	Notes             string                `json:"notes,omitempty"`
}

type CarrierAuthorityManifest struct {
	SchemaVersion int                    `json:"schema_version"`
	GeneratedBy   string                 `json:"generated_by"`
	Entries       []CarrierManifestEntry `json:"entries"`
}

type CarrierSemioCheckResult struct {
	SchemaVersion            int                   `json:"schema_version"`
	CheckedFiles             []string              `json:"checked_files"`
	CheckedGeneratedSurfaces []string              `json:"checked_generated_surfaces,omitempty"`
	Findings                 []CarrierSemioFinding `json:"findings"`
}

type CarrierSemioVirtualText struct {
	Path    string
	Content string
}

type CarrierSemioFinding struct {
	Path       string `json:"path"`
	Line       int    `json:"line"`
	Term       string `json:"term"`
	Snippet    string `json:"snippet"`
	Diagnostic string `json:"diagnostic"`
}

func DefaultCarrierAuthorityManifest() CarrierAuthorityManifest {
	entries := []CarrierManifestEntry{
		{
			ID:             "project-spec-carriers",
			PathPattern:    ".haft/specs/*.md",
			AuthorityClass: CarrierAuthorityCurrent,
			Surface:        "spec",
			Current:        true,
			Normativity:    "current project specification carrier; SQL graph remains runtime source of truth",
			Notes:          "target/enabling/term-map carriers are reviewable projections imported through explicit sync/lifecycle commands",
		},
		{
			ID:             "agent-skill-carriers",
			PathPattern:    "internal/cli/skill/*/SKILL.md",
			AuthorityClass: CarrierAuthorityCurrent,
			Surface:        "skill",
			Current:        true,
			Normativity:    "method carrier only; kernel validates and persists authority",
			Notes:          "h-reason remains the current umbrella skill; h-decide and h-commission remain manual-only",
		},
		{
			ID:             "agent-template-carrier",
			PathPattern:    "internal/cli/claude_md_template.md",
			AuthorityClass: CarrierAuthorityCurrent,
			Surface:        "template",
			Current:        true,
			Normativity:    "installed project discipline template",
			Notes:          "mirrored with AGENTS.md haft section; host prompts are carriers, not enforcement",
		},
		{
			ID:             "host-discipline-mirror",
			PathPattern:    "CLAUDE.md",
			AuthorityClass: CarrierAuthorityCurrent,
			Surface:        "host_prompt",
			Current:        true,
			Normativity:    "host-facing discipline mirror; kernel validates authority and generated text remains non-binding",
			Notes:          "mirrored from the installed project discipline template; host prompts are carriers, not enforcement",
		},
		{
			ID:             "fpf-route-carriers",
			PathPattern:    "internal/fpf/patterns/*.md",
			AuthorityClass: CarrierAuthoritySupport,
			Surface:        "retrieval",
			Current:        true,
			Normativity:    "navigation/support carrier over FPF routes; non-normative unless linked to source section",
			Notes:          "retrieval provenance must expose source edition/hash and non-normativity",
		},
		{
			ID:             "target-system-support-docs",
			PathPattern:    "spec/target-system/*.md",
			AuthorityClass: CarrierAuthoritySupport,
			Surface:        "target_system_doc",
			Current:        true,
			Normativity:    "target-system support documentation; SQL/artifact graph remains runtime source of truth",
			Notes:          "product-value evidence docs must keep claims bounded to explicit evidence refs",
		},
		{
			ID:             "root-spec-support-docs",
			PathPattern:    "spec/*.md",
			AuthorityClass: CarrierAuthoritySupport,
			Surface:        "root_spec_doc",
			Current:        true,
			Normativity:    "root spec support documentation; SQL/artifact graph remains runtime source of truth",
			Notes:          "root spec docs must use current v8 surfaces and label removed surfaces as archived/provenance",
		},
		{
			ID:             "enabling-system-support-docs",
			PathPattern:    "spec/enabling-system/*.md",
			AuthorityClass: CarrierAuthoritySupport,
			Surface:        "enabling_system_doc",
			Current:        true,
			Normativity:    "enabling-system support documentation; SQL/artifact graph remains runtime source of truth",
			Notes:          "current enabling docs must name v8 surfaces; archived desktop docs must stay explicitly archival",
		},
		{
			ID:                "archived-desktop-layer-contract",
			PathPattern:       "spec/enabling-system/DESKTOP_LAYER_CONTRACT.md",
			AuthorityClass:    CarrierAuthorityArchive,
			Surface:           "desktop_archive_doc",
			Current:           false,
			Normativity:       "historical desktop layer contract retained for provenance; not current runtime scope",
			DeadSurfacePolicy: "desktop terms are allowed here only because the whole carrier is archived provenance",
		},
		{
			ID:             "pi-plugin-bundle",
			PathPattern:    "packages/haft-pi/**",
			AuthorityClass: CarrierAuthorityCompatibility,
			Surface:        "plugin_bundle",
			Current:        true,
			Normativity:    "compatibility packaging of kernel tools and skills; not an independent authority",
			Notes:          "Pi package mirrors kernel contracts and may lag only as a packaging defect",
		},
		{
			ID:                "readme-support-doc",
			PathPattern:       "README.md",
			AuthorityClass:    CarrierAuthoritySupport,
			Surface:           "doc",
			Current:           true,
			Normativity:       "operator-facing explanation; not a binding artifact",
			DeadSurfacePolicy: "standalone agent, TUI, and desktop wrappers may appear only as dropped/archive/provenance surfaces",
		},
		{
			ID:             "changelog-provenance",
			PathPattern:    "CHANGELOG.md",
			AuthorityClass: CarrierAuthorityProvenance,
			Surface:        "release_history",
			Current:        true,
			Normativity:    "historical release account; not current product scope by itself",
		},
		{
			ID:             "external-review-packets",
			PathPattern:    ".context/external-review/**",
			AuthorityClass: CarrierAuthorityProvenance,
			Surface:        "review_packet",
			Current:        false,
			Normativity:    "external review evidence/provenance; must be adopted through decisions before governing",
		},
		{
			ID:             "context-archive",
			PathPattern:    ".context/_archived/**",
			AuthorityClass: CarrierAuthorityArchive,
			Surface:        "archive",
			Current:        false,
			Normativity:    "historical archive; never current authority without explicit successor decision",
		},
		{
			ID:                "desktop-tui-standalone-code",
			PathPattern:       "desktop/**, internal/ui/**, internal/cli/run_tui.go, internal/cli/board.go",
			AuthorityClass:    CarrierAuthorityArchive,
			Surface:           "dead_runtime_surface",
			Current:           false,
			Normativity:       "dropped v8 surface; retained only as archive/provenance/support unless explicitly reopened",
			DeadSurfacePolicy: "do not present standalone agent, TUI, or desktop wrappers as current product surfaces",
		},
		{
			ID:             "open-sleigh-sidekick",
			PathPattern:    "open-sleigh/**",
			AuthorityClass: CarrierAuthoritySidekick,
			Surface:        "sidekick",
			Current:        false,
			Normativity:    "execution-adjacent sidekick; out of current Haft semantic authority model",
		},
	}

	return CarrierAuthorityManifest{
		SchemaVersion: 1,
		GeneratedBy:   "haft carrier manifest",
		Entries:       sortedCarrierEntries(entries),
	}
}

func ValidateCarrierAuthorityManifest(manifest CarrierAuthorityManifest) []string {
	var findings []string
	seen := map[string]struct{}{}
	for _, entry := range manifest.Entries {
		id := strings.TrimSpace(entry.ID)
		if id == "" {
			findings = append(findings, "carrier manifest entry has empty id")
			continue
		}
		if _, exists := seen[id]; exists {
			findings = append(findings, fmt.Sprintf("duplicate carrier manifest id %q", id))
		}
		seen[id] = struct{}{}
		if strings.TrimSpace(entry.PathPattern) == "" {
			findings = append(findings, fmt.Sprintf("%s has empty path_pattern", id))
		}
		if strings.TrimSpace(string(entry.AuthorityClass)) == "" {
			findings = append(findings, fmt.Sprintf("%s has empty authority_class", id))
		}
		if strings.TrimSpace(entry.Surface) == "" {
			findings = append(findings, fmt.Sprintf("%s has empty surface", id))
		}
		if entry.Current && entry.AuthorityClass == CarrierAuthorityArchive {
			findings = append(findings, fmt.Sprintf("%s marks archive as current", id))
		}
		if entry.Current && entry.AuthorityClass == CarrierAuthoritySidekick {
			findings = append(findings, fmt.Sprintf("%s marks sidekick as current", id))
		}
	}

	return findings
}

func CarrierAuthorityManifestJSON(manifest CarrierAuthorityManifest) ([]byte, error) {
	return json.MarshalIndent(manifest, "", "  ")
}

func CheckCarrierSemio(root string) (CarrierSemioCheckResult, error) {
	return CheckCarrierSemioWithVirtualTexts(root, nil)
}

func CheckCarrierSemioWithVirtualTexts(root string, virtualTexts []CarrierSemioVirtualText) (CarrierSemioCheckResult, error) {
	files, err := carrierSemioCheckFiles(root)
	if err != nil {
		return CarrierSemioCheckResult{}, err
	}

	result := CarrierSemioCheckResult{
		SchemaVersion: 1,
		CheckedFiles:  files,
		Findings:      []CarrierSemioFinding{},
	}
	for _, relPath := range files {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relPath)))
		if err != nil {
			return CarrierSemioCheckResult{}, fmt.Errorf("read semio carrier %s: %w", relPath, err)
		}
		result.Findings = append(result.Findings, checkCarrierSemioText(relPath, string(content))...)
	}
	for _, virtualText := range normalizeCarrierSemioVirtualTexts(virtualTexts) {
		result.CheckedGeneratedSurfaces = append(result.CheckedGeneratedSurfaces, virtualText.Path)
		result.Findings = append(result.Findings, checkCarrierSemioText(virtualText.Path, virtualText.Content)...)
	}

	return result, nil
}

func CarrierSemioCheckResultJSON(result CarrierSemioCheckResult) ([]byte, error) {
	return json.MarshalIndent(result, "", "  ")
}

func sortedCarrierEntries(entries []CarrierManifestEntry) []CarrierManifestEntry {
	result := append([]CarrierManifestEntry(nil), entries...)
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result
}

func carrierSemioCheckFiles(root string) ([]string, error) {
	var files []string
	addGlob := func(pattern string) error {
		matches, err := filepath.Glob(filepath.Join(root, filepath.FromSlash(pattern)))
		if err != nil {
			return err
		}
		for _, match := range matches {
			info, err := os.Stat(match)
			if err != nil || info.IsDir() {
				continue
			}
			rel, err := filepath.Rel(root, match)
			if err != nil {
				return err
			}
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	}
	addWalk := func(relRoot string, extensions map[string]struct{}) error {
		absRoot := filepath.Join(root, filepath.FromSlash(relRoot))
		info, err := os.Stat(absRoot)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if !info.IsDir() {
			return nil
		}
		return filepath.WalkDir(absRoot, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			if _, ok := extensions[strings.ToLower(filepath.Ext(path))]; !ok {
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			files = append(files, filepath.ToSlash(rel))
			return nil
		})
	}

	for _, pattern := range []string{
		"README.md",
		"AGENTS.md",
		"CLAUDE.md",
		".haft/specs/*.md",
		"spec/*.md",
		"spec/target-system/*.md",
		"spec/enabling-system/*.md",
		"internal/cli/claude_md_template.md",
		"internal/cli/skill/*/SKILL.md",
		"packages/haft-pi/package.json",
		"packages/haft-pi/*.md",
		"packages/haft-pi/prompts/*.md",
		"packages/haft-pi/skills/*/SKILL.md",
	} {
		if err := addGlob(pattern); err != nil {
			return nil, err
		}
	}
	if err := addWalk("packages/haft-pi/extensions", map[string]struct{}{
		".js":  {},
		".mjs": {},
		".ts":  {},
		".tsx": {},
	}); err != nil {
		return nil, err
	}

	return dedupeSortedStrings(files), nil
}

func normalizeCarrierSemioVirtualTexts(texts []CarrierSemioVirtualText) []CarrierSemioVirtualText {
	out := make([]CarrierSemioVirtualText, 0, len(texts))
	seen := map[string]struct{}{}
	for _, text := range texts {
		path := strings.TrimSpace(text.Path)
		if path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		out = append(out, CarrierSemioVirtualText{
			Path:    path,
			Content: text.Content,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Path < out[j].Path
	})
	return out
}

func checkCarrierSemioText(path string, content string) []CarrierSemioFinding {
	lines := strings.Split(content, "\n")
	var findings []CarrierSemioFinding
	for index, line := range lines {
		term, ok := deadSurfaceTerm(line)
		if ok {
			if archivedDeadSurfaceCarrierPath(path) || allowedDeadSurfaceContext(lines, index) {
				continue
			}
			findings = append(findings, CarrierSemioFinding{
				Path:       path,
				Line:       index + 1,
				Term:       term,
				Snippet:    strings.TrimSpace(line),
				Diagnostic: "dead runtime surface must be labeled dropped/archive/provenance/support/not-current in current carriers",
			})
		}

		term, ok = authorityBoundaryTerm(lines, index)
		if !ok {
			continue
		}
		findings = append(findings, CarrierSemioFinding{
			Path:       path,
			Line:       index + 1,
			Term:       term,
			Snippet:    strings.TrimSpace(line),
			Diagnostic: "carrier/generated-surface wording must not imply prompt text, model args, tool descriptions, or schema visibility are operator authorization",
		})
	}
	return findings
}

func archivedDeadSurfaceCarrierPath(path string) bool {
	cleaned := filepath.ToSlash(strings.TrimSpace(path))
	for _, archivePath := range []string{
		"spec/enabling-system/DESKTOP_LAYER_CONTRACT.md",
	} {
		if cleaned == archivePath {
			return true
		}
	}
	return false
}

func deadSurfaceTerm(line string) (string, bool) {
	normalized := strings.ToLower(line)
	if strings.Contains(normalized, "haft agent") {
		return "haft agent", true
	}
	for _, term := range []string{"desktop", "tui"} {
		if containsDelimitedTerm(normalized, term) {
			return term, true
		}
	}
	if containsDelimitedTerm(normalized, "standalone") {
		for _, qualifier := range []string{"agent", "interactive", "runtime", "surface", "tool"} {
			if containsDelimitedTerm(normalized, qualifier) {
				return "standalone", true
			}
		}
	}
	return "", false
}

func containsDelimitedTerm(line string, term string) bool {
	index := strings.Index(line, term)
	for index >= 0 {
		start := index
		end := index + len(term)
		if semioTermBoundary(line, start-1) && semioTermBoundary(line, end) {
			return true
		}
		nextStart := index + len(term)
		nextIndex := strings.Index(line[nextStart:], term)
		if nextIndex < 0 {
			return false
		}
		index = nextStart + nextIndex
	}
	return false
}

func semioTermBoundary(line string, index int) bool {
	if index < 0 || index >= len(line) {
		return true
	}
	char := line[index]
	return !(char >= 'a' && char <= 'z') &&
		!(char >= '0' && char <= '9') &&
		char != '_' &&
		char != '-'
}

func allowedDeadSurfaceContext(lines []string, index int) bool {
	window := strings.ToLower(semioContextWindow(lines, index))
	for _, marker := range []string{
		"dropped",
		"archive",
		"archived",
		"provenance",
		"support carrier",
		"support doc",
		"support material",
		"support unless",
		"historical",
		"dead",
		"not current",
		"non-current",
		"no longer",
		"do not recommend",
		"out of current",
		"sidekick",
		"compatibility",
		"legacy",
		"old",
		"removed",
		"retained",
		"unless explicitly reopened",
		"must not",
		"should not",
		"not active",
	} {
		if strings.Contains(window, marker) {
			return true
		}
	}
	return false
}

func authorityBoundaryTerm(lines []string, index int) (string, bool) {
	line := strings.ToLower(lines[index])
	window := strings.ToLower(semioContextWindow(lines, index))
	if !hasCarrierAuthoritySurfaceTerm(line) {
		return "", false
	}
	if !hasAuthorityGrantTerm(line) {
		return "", false
	}
	if allowedAuthorityBoundaryContext(window) {
		return "", false
	}
	return "operator_authorization_boundary", true
}

func hasCarrierAuthoritySurfaceTerm(window string) bool {
	for _, term := range []string{
		"prompt text",
		"model-supplied",
		"model supplied",
		"mcp argument",
		"mcp schema",
		"schema visibility",
		"tool description",
		"tool schema",
		"generated text",
		"generated schema",
		"host schema",
		"skill description",
		"plugin metadata",
		"pi metadata",
	} {
		if strings.Contains(window, term) {
			return true
		}
	}
	return false
}

func hasAuthorityGrantTerm(window string) bool {
	for _, term := range []string{
		"authorizes",
		"authorize",
		"authorization",
		"approves",
		"approve",
		"approval",
		"binds",
		"binding",
		"proof",
		"is evidence",
		"as evidence",
		"counts as evidence",
		"evidence for approval",
		"evidence for binding",
		"gate passage",
	} {
		if strings.Contains(window, term) {
			return true
		}
	}
	return false
}

func allowedAuthorityBoundaryContext(window string) bool {
	for _, marker := range []string{
		"not proof",
		"not operator authorization",
		"not authorization",
		"not binding authority",
		"not binding",
		"not evidence",
		"not gate passage",
		"never",
		"must not",
		"do not",
		"cannot",
		"fails closed",
		"fail closed",
		"fail-closed",
		"rejected",
		"requires explicit",
		"explicit operator",
		"operator approval required",
		"separate from binding",
		"separate from operator",
	} {
		if strings.Contains(window, marker) {
			return true
		}
	}
	return false
}

func semioContextWindow(lines []string, index int) string {
	start := index - 1
	if start < 0 {
		start = 0
	}
	end := index + 3
	if end > len(lines) {
		end = len(lines)
	}
	return strings.Join(lines[start:end], " ")
}

func dedupeSortedStrings(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	sort.Strings(result)
	return result
}
