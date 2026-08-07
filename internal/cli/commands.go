package cli

import (
	_ "embed"
	"os"
	"path/filepath"
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

//go:embed skill/h-note/SKILL.md
var embeddedHNoteSkill []byte

//go:embed skill/h-commission/SKILL.md
var embeddedHCommissionSkill []byte

// skillManifest declares a haft skill to be installed by `haft init`.
// AllowImplicit is the host routing policy. A routable skill may interpret a
// direct operator request; routability is never itself an authority receipt.
type skillManifest struct {
	Name          string
	Content       []byte
	AllowImplicit bool
}

// allSkills is the haft skill set installed by `haft init`. Order in
// this slice is the install order; first-failure semantics return the
// partial path so the operator sees which skill broke.
var allSkills = []skillManifest{
	// Source-first umbrella entry: names the minimum FPF distinctions and
	// teaches agents to query and hydrate source units before relying on them.
	// It may route to one independent capability when that concern is current;
	// it does not impose a project sequence.
	{Name: "h-reason", Content: embeddedHReasonSkill, AllowImplicit: true},

	// h-decide is a routable interpreter for direct operator choice requests.
	{Name: "h-decide", Content: embeddedHDecideSkill, AllowImplicit: true},

	// Independent reasoning capabilities; none implies the next.
	{Name: "h-frame", Content: embeddedHFrameSkill, AllowImplicit: true},
	{Name: "h-diagnose", Content: embeddedHDiagnoseSkill, AllowImplicit: true},
	{Name: "h-explore", Content: embeddedHExploreSkill, AllowImplicit: true},
	{Name: "h-compare", Content: embeddedHCompareSkill, AllowImplicit: true},

	// Independent memory, verification, spec, status, and bootstrap capabilities.
	{Name: "h-verify", Content: embeddedHVerifySkill, AllowImplicit: true},
	{Name: "h-spec", Content: embeddedHSpecSkill, AllowImplicit: true},
	{Name: "h-status", Content: embeddedHStatusSkill, AllowImplicit: true},
	{Name: "h-onboard", Content: embeddedHOnboardSkill, AllowImplicit: true},
	{Name: "h-note", Content: embeddedHNoteSkill, AllowImplicit: true},

	// Execution-authority grants remain manual-only.
	{Name: "h-commission", Content: embeddedHCommissionSkill, AllowImplicit: false},
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
	"h-spec-cover",
	"h-abduct",
	"h-boundary-unpack",
	"h-semio-review",
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
	case "agy":
		if local {
			return filepath.Join(projectRoot, ".gemini", "skills"), true
		}
		return filepath.Join(homeDir, ".gemini", "skills"), true
	case "opencode":
		if local {
			return filepath.Join(projectRoot, ".opencode", "skills"), true
		}
		return filepath.Join(homeDir, ".config", "opencode", "skills"), true
	case "grok":
		if local {
			return filepath.Join(projectRoot, ".grok", "skills"), true
		}
		return filepath.Join(homeDir, ".grok", "skills"), true
	}
	return "", false
}
