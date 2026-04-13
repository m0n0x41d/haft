---
description: "Discover existing project knowledge and build a legacy map"
---

# Project Onboarding

Quint was just initialized on this project. Help the user discover and capture existing engineering knowledge.

## Discovery protocol

Scan the project systematically. For each finding, use the appropriate Quint tool to record it.

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

### 7. Module coverage analysis
Run `haft_query(action="coverage")` to get the module map and decision coverage:
- Identify **blind modules** — parts of the codebase with no engineering decisions
- Prioritize blind modules by criticality: modules with many dependents and no decisions are the highest-risk blind spots
- For the most critical blind modules: suggest framing problems for key invariants
- Report the coverage percentage and highlight the top 3-5 blind spots

### 8. Unresolved problems
If you find architectural tensions, TODO comments about design issues, or open questions:
- Use `haft_problem(action="frame")` to capture them as ProblemCards
- These become the starting point for the user's next reasoning cycle
- Cross-reference with blind modules from step 7 — blind modules with design tensions are highest priority

## Output
After scanning, report:
- How many notes were created
- How many problems were discovered
- What key decisions are now searchable in Quint
- What gaps exist (decisions mentioned but not documented, missing ADRs)
- **Module coverage**: X modules detected, Y% governed, Z blind spots identified

Suggest next steps: `/h-status` to see the knowledge map and module coverage, `/h-frame` for the most pressing unresolved problem or highest-risk blind module.

$ARGUMENTS
