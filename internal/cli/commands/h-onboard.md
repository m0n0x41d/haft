---
description: "Discover existing project knowledge and build a legacy map"
---

# Project Onboarding

Haft was just initialized on this project. Discover and capture existing engineering knowledge.

## Phase 1: Surface scan (always run)

### 1. Project overview
- Read `README.md` — extract purpose, tech stack, key components, architecture
- Create a `haft_note` summarizing the project identity

### 2. Existing decision records
Scan for common decision artifact locations:
- `docs/adr/`, `adr/`, `doc/adr/` — Architecture Decision Records
- `docs/architecture.md`, `ARCHITECTURE.md` — architecture documentation
- `.context/`, `docs/decisions/` — decision documentation
- `DECISIONS.md`, `DESIGN.md` — design rationale

For each decision found: create a `haft_note` with the decision's title, rationale (summarized), and affected files.

### 3. Existing specifications and plans
Scan for:
- `docs/`, `spec/`, `specs/` — project documentation
- `openspec/` — OpenSpec specifications
- `TODO.md`, `ROADMAP.md`, `PLAN.md` — planning docs
- `CONTRIBUTING.md` — development process docs

For significant specs: create `haft_note` entries linking to the source files.

### 4. Test documentation
Scan for:
- `docs/testing.md`, `test-plan.md` — test strategy
- Benchmark results, performance baselines

### 5. Tech stack and constraints
From `go.mod`, `package.json`, `pyproject.toml`, `Cargo.toml`, `Dockerfile`, `.github/workflows/`:
- Identify key dependencies and their versions
- Note infrastructure constraints (CI pipeline, deployment target)
- Create a `haft_note` capturing tech stack decisions

### 6. Recent history
Check `git log --oneline -20` for:
- Major refactors or migrations
- Architecture changes
- Patterns in commit messages that suggest undocumented decisions

## Phase 2: Module coverage deep scan (always run)

### 7. Module coverage analysis
Run `haft_query(action="coverage")` to get the full module map:
- List ALL modules with their governance status (governed / partial / blind)
- Count total modules, governed percentage, blind count
- **Prioritize blind modules** by: number of dependents (from dependency graph), lines of code, recent git activity

### 8. Deep scan of blind modules

**This is the critical step.** For each blind module, starting with highest priority:

- **Read the module's code** — main files, exported interfaces, key types
- **Determine the module's responsibility** — what does it do, what invariants does it maintain
- **Identify dependencies** — what does it import, what depends on it
- **Look for implicit decisions** — coding patterns, error handling conventions, data flow assumptions
- **Record findings** as `haft_note` with:
  - Module name and responsibility
  - Key invariants (what must hold)
  - Implicit decisions worth making explicit
  - Risk assessment (what breaks if this module changes without governance)

**If you have subagent/parallel execution capability:** spawn one subagent per blind module for parallel analysis. Each subagent gets:
- Module path and file list
- Project context (README, tech stack from Phase 1)
- Instruction: "Read all code in this module. Report: responsibility, invariants, implicit decisions, dependencies, risks. Record as haft_note."

Merge subagent results and continue.

### 9. Unresolved problems
If you find architectural tensions, TODO comments about design issues, or open questions:
- Use `haft_problem(action="frame")` to capture them as ProblemCards
- Cross-reference with blind modules — blind modules with design tensions are highest priority

## Output

After scanning, report:

### Coverage summary
```
Modules: X total, Y governed (Z%), W blind
Blind modules scanned: N (with deep analysis)
```

### Findings
- Notes created: N (X from docs, Y from module analysis)
- Problems discovered: M
- Key decisions now searchable
- Gaps: decisions mentioned but not documented, missing ADRs
- **Top 3 highest-risk blind spots** with reasoning

### Next steps
- `/h-status` — see the knowledge map and module coverage
- `/h-frame` — frame the most pressing unresolved problem
- For very large projects: suggest running onboard again focused on specific subsystems

$ARGUMENTS
