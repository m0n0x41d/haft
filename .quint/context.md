# Repository Context (A.2.6 Context Slice)

## Project Identity

**Name:** Crucible Code
**Purpose:** Structured reasoning framework for Claude Code — implement FPF methodology as slash commands
**Paradigm:** Meta-tooling (a tool for making decisions about other projects)

## Tech Stack (Inferred)

- **Language:** Markdown (commands), Bash (installer)
- **Runtime:** None — consumed by Claude Code at prompt time
- **Dependencies:** Claude Code CLI only
- **Artifacts:** Plain text files (`.md`, `.yaml`)

## Hard Constraints (User-Defined)

1. **FPF Fidelity:** Must not deviate significantly from core FPF principles (ADI cycle, WLNK, Transformer Mandate, assurance levels)
2. **Simplicity:** Must not become bloated or require significant onboarding overhead
3. **Git-Friendly:** All outputs must be diffable plain text
4. **Claude Code Compatibility:** Must work reliably across Claude Code versions

## Scale & Performance

N/A — no deployed runtime. "Performance" here means: command execution time in Claude Code, token efficiency, and cognitive load on the user.

## Usage Context

**Dogfooding:** This repo uses FPF to make decisions about FPF itself. Recursive meta-reasoning.

## Scope Notes

- Decisions made here affect the framework used by other projects
- Changes to command semantics propagate to all users
- Backward compatibility matters — don't break existing `.quint/` directories
