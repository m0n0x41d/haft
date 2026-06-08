package cli

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

//go:embed skill/h-reason/SKILL.md
var embeddedHReasonSkill []byte

//go:embed skill/h-decide/SKILL.md
var embeddedHDecideSkill []byte

//go:embed skill/h-frame/SKILL.md
var embeddedHFrameSkill []byte

//go:embed skill/h-diagnose/SKILL.md
var embeddedHDiagnoseSkill []byte

//go:embed skill/h-explore/SKILL.md
var embeddedHExploreSkill []byte

//go:embed skill/h-compare/SKILL.md
var embeddedHCompareSkill []byte

//go:embed skill/h-verify/SKILL.md
var embeddedHVerifySkill []byte

//go:embed skill/h-status/SKILL.md
var embeddedHStatusSkill []byte

//go:embed skill/h-spec/SKILL.md
var embeddedHSpecSkill []byte

//go:embed skill/h-onboard/SKILL.md
var embeddedHOnboardSkill []byte

//go:embed skill/h-spec-cover/SKILL.md
var embeddedHSpecCoverSkill []byte

//go:embed skill/h-note/SKILL.md
var embeddedHNoteSkill []byte

//go:embed skill/h-commission/SKILL.md
var embeddedHCommissionSkill []byte

//go:embed skill/h-abduct/SKILL.md
var embeddedHAbductSkill []byte

//go:embed skill/h-boundary-unpack/SKILL.md
var embeddedHBoundaryUnpackSkill []byte

//go:embed skill/h-semio-review/SKILL.md
var embeddedHSemioReviewSkill []byte

// skillManifest declares a haft skill to be installed by `haft init`.
// AllowImplicit is the codex policy gate — false means the skill is
// explicit-only (e.g., h-decide manual-only per Transformer Mandate).
type skillManifest struct {
	Name          string
	Content       []byte
	AllowImplicit bool
}

// allSkills is the haft skill set installed by `haft init`. Order in
// this slice is the install order; first-failure semantics return the
// partial path so the operator sees which skill broke.
var allSkills = []skillManifest{
	// Umbrella reasoning entry — carries full FPF reasoning palette
	// (frame, explore, compare, verify, note, slideument patterns).
	// Operators can type /h-reason explicitly; the description is broad
	// enough to also fire as fallback on ambiguous "let's think about X"
	// signals where no specialized skill clearly matches.
	{Name: "h-reason", Content: embeddedHReasonSkill, AllowImplicit: true},

	// Manual-only Transformer Mandate skills (cannot auto-fire)
	{Name: "h-decide", Content: embeddedHDecideSkill, AllowImplicit: false},

	// Auto-triggering workflow skills (framing + exploration)
	{Name: "h-frame", Content: embeddedHFrameSkill, AllowImplicit: true},
	{Name: "h-diagnose", Content: embeddedHDiagnoseSkill, AllowImplicit: true},
	{Name: "h-explore", Content: embeddedHExploreSkill, AllowImplicit: true},
	{Name: "h-compare", Content: embeddedHCompareSkill, AllowImplicit: true},

	// Auto-triggering workflow skills (verify + operate)
	{Name: "h-verify", Content: embeddedHVerifySkill, AllowImplicit: true},
	{Name: "h-spec-cover", Content: embeddedHSpecCoverSkill, AllowImplicit: true},
	{Name: "h-spec", Content: embeddedHSpecSkill, AllowImplicit: true},
	{Name: "h-status", Content: embeddedHStatusSkill, AllowImplicit: true},
	{Name: "h-onboard", Content: embeddedHOnboardSkill, AllowImplicit: true},
	{Name: "h-note", Content: embeddedHNoteSkill, AllowImplicit: true},

	// Manual-only sacred skill (execution authority — Transformer Mandate)
	{Name: "h-commission", Content: embeddedHCommissionSkill, AllowImplicit: false},

	// Subroutine skills (explicit-only — typically called by other skills
	// or by the operator when working a specific FPF sub-discipline)
	{Name: "h-abduct", Content: embeddedHAbductSkill, AllowImplicit: false},
	{Name: "h-boundary-unpack", Content: embeddedHBoundaryUnpackSkill, AllowImplicit: false},
	{Name: "h-semio-review", Content: embeddedHSemioReviewSkill, AllowImplicit: false},
}

// deprecatedSkillDirs lists skill directory names that prior haft
// versions installed but the current skill set has replaced.
// `installSkill` removes them on every install so operators who re-run
// `haft init` get a clean state without manual cleanup. Add older
// deprecated skill names here as the skill set evolves; do NOT remove
// entries (operators who skip versions need the cumulative cleanup
// list).
//
// TODO(future release): once enough time has passed that no live install
// could still carry these directories, prune the list. Track via
// `dec-20260525-v8-architecture-pivot` predictions — when its
// rollback window closes cleanly, the migration entries here can drop.
var deprecatedSkillDirs = []string{
	"q-reason",
	"h-fpf",
}

// cleanupLegacySlashCommands removes any legacy haft slash-command
// files left behind by prior installs. Skills are the primary surface
// in this haft version; slash commands of the same names would
// duplicate the skill bodies. Returns the display path of the
// commands directory + number of files removed.
//
// TODO(future release): once enough time has passed that no live
// install could still carry these files, this function and the call
// sites in init.go can be deleted. Track the supersedence window of
// dec-20260525-v8-architecture-pivot.
func cleanupLegacySlashCommands(projectRoot, platform string, local bool) (string, int) {
	homeDir, _ := os.UserHomeDir()

	var destDir, ext string
	switch platform {
	case "claude":
		if local {
			destDir = filepath.Join(projectRoot, ".claude", "commands")
		} else {
			destDir = filepath.Join(homeDir, ".claude", "commands")
		}
		ext = ".md"
	case "cursor":
		if local {
			destDir = filepath.Join(projectRoot, ".cursor", "commands")
		} else {
			destDir = filepath.Join(homeDir, ".cursor", "commands")
		}
		ext = ".md"
	case "gemini":
		if local {
			destDir = filepath.Join(projectRoot, ".gemini", "commands")
		} else {
			destDir = filepath.Join(homeDir, ".gemini", "commands")
		}
		ext = ".toml"
	case "codex":
		destDir = filepath.Join(homeDir, ".codex", "prompts")
		ext = ".md"
	case "opencode":
		if local {
			destDir = filepath.Join(projectRoot, ".opencode", "commands")
		} else {
			destDir = filepath.Join(homeDir, ".config", "opencode", "commands")
		}
		ext = ".md"
	default:
		return "", 0
	}

	if _, err := os.Stat(destDir); err != nil {
		return displayHomePath(destDir, homeDir), 0
	}

	removed := 0
	for _, cmd := range deprecatedCommands {
		path := filepath.Join(destDir, cmd+ext)
		if err := os.Remove(path); err == nil {
			removed++
		}
	}
	return displayHomePath(destDir, homeDir), removed
}

func installCodexSkills(projectRoot string, local bool) (string, int, error) {
	homeDir, _ := os.UserHomeDir()
	skillsRoot := codexSkillsRoot(homeDir, projectRoot, local)

	if err := os.MkdirAll(skillsRoot, 0755); err != nil {
		return "", 0, err
	}

	cleanupOldCodexSkills(skillsRoot)
	// Remove deprecated skill dirs (q-reason, h-reason) on every install.
	for _, name := range deprecatedSkillDirs {
		_ = os.RemoveAll(filepath.Join(skillsRoot, name))
	}

	// Install the haft skills with per-skill codex policy gates.
	count := 0
	for _, sk := range allSkills {
		body := transformCodexSkillReferences(string(sk.Content))
		if err := writeCodexSkill(skillsRoot, sk.Name, body, sk.AllowImplicit); err != nil {
			return "", 0, err
		}
		count++
	}

	return displayHomePath(skillsRoot, homeDir), count, nil
}

func codexSkillsRoot(homeDir, projectRoot string, local bool) string {
	if local {
		return filepath.Join(projectRoot, ".agents", "skills")
	}
	return filepath.Join(homeDir, ".agents", "skills")
}

func cleanupOldCodexSkills(skillsRoot string) {
	for _, cmd := range deprecatedCommands {
		_ = os.RemoveAll(filepath.Join(skillsRoot, cmd))
	}
}

func writeCodexSkill(skillsRoot, name, content string, allowImplicit bool) error {
	skillDir := filepath.Join(skillsRoot, name)
	if err := os.MkdirAll(filepath.Join(skillDir, "agents"), 0755); err != nil {
		return err
	}

	skillPath := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillPath, []byte(content), 0644); err != nil {
		return err
	}

	return writeCodexSkillPolicy(skillDir, allowImplicit)
}

func transformCodexSkillReferences(content string) string {
	replacer := strings.NewReplacer(
		"/h-", "$h-",
		"Slash commands", "Explicit skill invocations",
		"slash commands", "explicit skill invocations",
		"Slash command", "Explicit skill",
		"slash command", "explicit skill",
		"Quint", "Haft",
		"quint", "haft",
	)
	return replacer.Replace(content)
}

// deprecatedCommands lists slash-command names that prior haft versions
// installed but the current command set no longer ships. Cleanup runs on every
// install so re-running `haft init` leaves the host's command directory clean.
// Keep entries cumulative — operators who skip versions need the full list to
// migrate forward.
//
// TODO(future release): prune the oldest entries (q0-init through the
// q-prefix block) once enough time has passed that no live install
// could still carry them. Track via the supersedence window of
// `dec-20260525-v8-architecture-pivot`.
var deprecatedCommands = []string{
	// Pre-rename q-prefix commands
	"q0-init", "q-decay", "q-actualize", "q1-add", "q-implement",
	"q-internalize", "q-query", "q-reset", "q-resolve",
	"q1-hypothesize", "q2-verify", "q3-validate", "q4-audit", "q5-decide",
	// q-prefix renamed to h-prefix
	"q-apply", "q-char", "q-compare", "q-decide", "q-explore",
	"q-frame", "q-note", "q-onboard", "q-problems", "q-refresh",
	"q-reason", "q-search", "q-status",
	// h-refresh replaced by h-verify
	"h-refresh",
	// Folded into the current skill set — functionality moved to
	// h-frame, h-status, mcp__haft__haft_query, or projection MCP tools.
	"h-char",     // characterization folded into h-frame
	"h-problems", // covered by h-status
	"h-search",   // covered by haft_query(action="search")
	"h-view",     // covered by mcp projection tools
	"h-reason",   // legacy slash-command file cleanup — h-reason now ships as a skill, not a commands/ file
	// Slash-command files dropped — skills are the primary surface; in
	// Claude Code typing /skill-name fires the skill directly, so a
	// parallel slash-command file would either duplicate the skill body
	// or shadow it. Operators upgrading from a prior install need these
	// names wiped from their commands directory.
	"h-frame", "h-explore", "h-compare", "h-decide", "h-verify",
	"h-status", "h-spec", "h-onboard", "h-note", "h-commission",
	"h-diagnose", "h-fpf", "h-spec-cover", "h-abduct",
	"h-boundary-unpack", "h-semio-review",
}

func cleanupCodexPromptCommands() (string, int, error) {
	homeDir, _ := os.UserHomeDir()
	destDir := filepath.Join(homeDir, ".codex", "prompts")

	if _, err := os.Stat(destDir); err != nil {
		return displayHomePath(destDir, homeDir), 0, nil
	}

	removed := 0
	for _, name := range deprecatedCommands {
		path := filepath.Join(destDir, name+".md")
		if _, err := os.Stat(path); err != nil {
			continue
		}
		if err := os.Remove(path); err != nil {
			return "", removed, err
		}
		removed++
	}

	return displayHomePath(destDir, homeDir), removed, nil
}

// skillsRoot returns the per-platform parent directory under which
// individual skill folders live (one folder per skill name).
func skillsRoot(platform string, local bool, projectRoot string) (string, bool) {
	homeDir, _ := os.UserHomeDir()
	switch platform {
	case "claude":
		if local {
			return filepath.Join(projectRoot, ".claude", "skills"), true
		}
		return filepath.Join(homeDir, ".claude", "skills"), true
	case "cursor":
		if local {
			return filepath.Join(projectRoot, ".cursor", "skills"), true
		}
		return filepath.Join(homeDir, ".cursor", "skills"), true
	case "air":
		return filepath.Join(projectRoot, "skills"), true
	case "codex":
		if local {
			return filepath.Join(projectRoot, ".agents", "skills"), true
		}
		return filepath.Join(homeDir, ".agents", "skills"), true
	case "opencode":
		if local {
			return filepath.Join(projectRoot, ".opencode", "skills"), true
		}
		return filepath.Join(homeDir, ".config", "opencode", "skills"), true
	}
	return "", false
}

// installSkill installs all skills in `allSkills` under the
// platform-appropriate skills directory and removes deprecated skill
// folders that prior haft versions left behind. Returns the display
// path of the skills root + count of skills installed.
//
// Per skill: SKILL.md is the markdown body; codex writes an additional
// `agents/openai.yaml` policy controlling implicit invocation. Operator
// invocation behavior is governed by frontmatter (`disable-model-invocation`
// on Claude Code; `policy.allow_implicit_invocation` on Codex).
func installSkill(platform string, local bool, projectRoot string) (string, int, error) {
	root, ok := skillsRoot(platform, local, projectRoot)
	if !ok {
		return "", 0, nil
	}

	if err := os.MkdirAll(root, 0755); err != nil {
		return "", 0, fmt.Errorf("failed to create skills root %q: %w", root, err)
	}

	// Remove deprecated skill directories so operators who re-run
	// `haft init` land on the current set without manual cleanup.
	for _, name := range deprecatedSkillDirs {
		_ = os.RemoveAll(filepath.Join(root, name))
	}

	installed := 0
	for _, sk := range allSkills {
		skillDir := filepath.Join(root, sk.Name)
		if err := os.MkdirAll(skillDir, 0755); err != nil {
			return "", 0, fmt.Errorf("create skill dir %q: %w", skillDir, err)
		}

		content := sk.Content
		if platform == "codex" {
			content = []byte(transformCodexSkillReferences(string(sk.Content)))
		}

		destPath := filepath.Join(skillDir, "SKILL.md")
		if err := os.WriteFile(destPath, content, 0644); err != nil {
			return "", 0, fmt.Errorf("write skill %q: %w", sk.Name, err)
		}

		if platform == "codex" {
			if err := writeCodexSkillPolicy(skillDir, sk.AllowImplicit); err != nil {
				return "", 0, fmt.Errorf("write codex policy %q: %w", sk.Name, err)
			}
		}
		installed++
	}

	homeDir, _ := os.UserHomeDir()
	return displayHomePath(root, homeDir), installed, nil
}

func writeCodexSkillPolicy(skillDir string, allowImplicit bool) error {
	agentsDir := filepath.Join(skillDir, "agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		return err
	}

	policy := fmt.Sprintf("policy:\n  allow_implicit_invocation: %t\n", allowImplicit)
	return os.WriteFile(filepath.Join(agentsDir, "openai.yaml"), []byte(policy), 0644)
}

func displayHomePath(path, homeDir string) string {
	displayPath := path
	if homeDir == "" {
		return displayPath
	}

	homePrefix := homeDir + string(os.PathSeparator)
	if path == homeDir || strings.HasPrefix(path, homePrefix) {
		displayPath = "~" + strings.TrimPrefix(path, homeDir)
	}
	return displayPath
}
