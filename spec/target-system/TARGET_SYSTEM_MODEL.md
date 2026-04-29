# Target System Model — Reading Order

> This is the index. Read documents in order.
>
> **Start here if you're an agent:** read [AGENT_CONTRACT.md](../AGENT_CONTRACT.md) first — it tells you what you may edit and what rules apply.

## Layer 1: Context (10 min)

| # | Document | What you learn |
|---|----------|---------------|
| 1 | [SYSTEM_CONTEXT.md](SYSTEM_CONTEXT.md) | Why Haft exists, who it's for, what it's not, supersystem diagram |
| 2 | [TERM_MAP.md](TERM_MAP.md) | Canonical vocabulary — one meaning per term |
| 3 | [SCOPE_FREEZE.md](SCOPE_FREEZE.md) | What's in v6 / v7 / v8 / later / never |

## Layer 2: Domain Ontologies (30 min)

| # | Document | What you learn |
|---|----------|---------------|
| 4 | [SPECIFICATION_ONTOLOGY.md](SPECIFICATION_ONTOLOGY.md) | ProjectSpecificationSet, TargetSystemSpec, EnablingSystemSpec, SpecCoverage, semantic architecture |
| 5 | [MODE_ONTOLOGY.md](MODE_ONTOLOGY.md) | The 5 engineering modes, depth calibration, interaction modes |
| 6 | [ARTIFACT_ONTOLOGY.md](ARTIFACT_ONTOLOGY.md) | Artifact kinds, lifecycle states, relationships |
| 7 | [EVIDENCE_ONTOLOGY.md](EVIDENCE_ONTOLOGY.md) | R_eff, CL, decay, claims, verdicts, measurement |

## Layer 3: Constraints (15 min)

| # | Document | What you learn |
|---|----------|---------------|
| 8 | [PROJECT_ONBOARDING_CONTRACT.md](PROJECT_ONBOARDING_CONTRACT.md) | Add/init/onboard/spec-check/spec-plan/commission/runtime E2E contract |
| 9 | [ILLEGAL_STATES.md](ILLEGAL_STATES.md) | What must be unrepresentable in the artifact graph |
| 10 | [EXECUTION_CONTRACT.md](EXECUTION_CONTRACT.md) | WorkCommission, RuntimeRun, preflight, YOLO, and external projection authority boundaries |

## Layer 4: Open Issues

| # | Document | What you learn |
|---|----------|---------------|
| 11 | [OPEN_QUESTIONS.md](OPEN_QUESTIONS.md) | Unresolved design questions |

## Enabling System

| Document | What you learn |
|----------|---------------|
| [../enabling-system/ARCHITECTURE.md](../enabling-system/ARCHITECTURE.md) | Module map: Core / Flow / Governor / Surfaces |
| [../enabling-system/STACK_DECISION.md](../enabling-system/STACK_DECISION.md) | Technology choices and rationale |

## Research Archive

Full system engineering analysis with GPT-5.4 Pro review results lives in `.context/model/`.
11 core documents + 8 research results. This `spec/` directory is the **structured distillation** of that research.
