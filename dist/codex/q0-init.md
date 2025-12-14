---
description: FPF Initialization
argument-hint: (optional)
---

---
description: "Initialize FPF (First Principles Framework) for structured reasoning"
---

# FPF Initialization

## What This Does

Creates the `.fpf/` directory structure for systematic hypothesis-driven reasoning.

## Process

### 1. Check Existing State

```bash
if [ -d ".fpf" ]; then
    echo "FPF already initialized. Showing current state..."
    # Run /q-status logic instead
fi
```

If `.fpf/` exists, show current state and exit (do not reinitialize).

### 2. Create Directory Structure

```bash
mkdir -p .fpf/knowledge/L0
mkdir -p .fpf/knowledge/L1
mkdir -p .fpf/knowledge/L2
mkdir -p .fpf/knowledge/invalid
mkdir -p .fpf/evidence
mkdir -p .fpf/decisions
mkdir -p .fpf/sessions
```

### 3. Create Context File (Agentic)

**Do not just create a blank file.**

1.  **Investigate:** Scan the repository for technical signals.
    - Check `package.json`, `go.mod`, `Cargo.toml`, `requirements.txt`, `pom.xml`, `Gemfile`.
    - Check `Dockerfile`, `docker-compose.yml`, `k8s/`, `.github/workflows`.
    - Check `README.md` for architecture notes.

2.  **Draft & Interview:**
    - Present what you found: "I detected Python 3.11 and Django..."
    - Ask **specific** questions for what you can't see (Scale, Budget, Constraints).
    - *Example:* "I see this is a web app. What is the target user scale? (<1k, >1M?)"

3.  **Write `.fpf/context.md`:**
    - Combine your findings and the user's answers.

```markdown
# Repository Context (A.2.6 Context Slice)

## Tech Stack (Inferred)
- **Language:** [e.g. Python 3.11]
- **Frameworks:** [e.g. Django 4.2]
- **Infra:** [e.g. Docker, AWS]

## Scale & Performance (User-Defined)
- **Users:** [Value]
- **Traffic:** [Value]
- **Latency Target:** [Value]

## Hard Constraints (User-Defined)
- [Constraint 1]
- [Constraint 2]
```

### 4. Create Session File

Create `.fpf/session.md`:

```markdown
# FPF Session

## Status
Phase: INITIALIZED
Started: [timestamp]
Problem: (none yet)

## Active Hypotheses
(none)

## Phase Transitions Log
| Timestamp | From | To | Trigger |
|-----------|------|-----|---------|
| [now] | — | INITIALIZED | /q0-init |

## Next Step
Run `/q1-hypothesize <problem>` to begin reasoning cycle.

---

## Valid Phase Transitions

```
INITIALIZED ─────────────────► ABDUCTION_COMPLETE
     │                              │
     │ /q1-hypothesize           │ /q2-check
     │                              ▼
     │                        DEDUCTION_COMPLETE
     │                              │
     │               ┌──────────────┴──────────────┐
     │               │ /q3-test                 │ /q3-research
     │               │ /q3-research             │ /q3-test
     │               ▼                             ▼
     │         INDUCTION_COMPLETE ◄────────────────┘
     │               │
     │               │ /q4-audit (recommended)
     │               │ /q5-decide (allowed with warning)
     │               ▼
     │         AUDIT_COMPLETE
     │               │
     │               │ /q5-decide
     │               ▼
     └─────────► DECIDED ──► (new cycle or end)
```

## Command Reference
| # | Command | Valid From Phase | Result |
|---|---------|------------------|--------|
| 0 | `/q0-init` | (none) | INITIALIZED |
| 1 | `/q1-hypothesize` | INITIALIZED | ABDUCTION_COMPLETE |
| 2 | `/q2-check` | ABDUCTION_COMPLETE | DEDUCTION_COMPLETE |
| 3a | `/q3-test` | DEDUCTION_COMPLETE | INDUCTION_COMPLETE |
| 3b | `/q3-research` | DEDUCTION_COMPLETE | INDUCTION_COMPLETE |
| 4 | `/q4-audit` | INDUCTION_COMPLETE | AUDIT_COMPLETE |
| 5 | `/q5-decide` | INDUCTION_COMPLETE*, AUDIT_COMPLETE | DECIDED |

*With warning if audit skipped
```

### 4. Create Optional Config File

Create `.fpf/config.yaml` (optional, for project-level settings):

```yaml
# FPF Project Configuration
# All values are optional — defaults shown

# Evidence validity defaults (days)
validity_defaults:
  internal_benchmark: 90
  internal_test: 180
  external_docs: 180
  external_blog: 365
  external_paper: 730

# Congruence penalty values (Φ function)
congruence_penalties:
  high: 0.00    # Direct applicability
  medium: 0.15  # Partial context match
  low: 0.35     # Weak applicability

# Epistemic debt thresholds
epistemic_debt:
  warning_days: 30   # Warn when evidence expires within N days
  
# Hypothesis generation
hypothesize:
  min_hypotheses: 3
  require_diversity: true  # At least one conservative, innovative, minimal

# Audit settings  
audit:
  required_before_decide: false  # If true, blocks /q5-decide without audit
```

### 5. Create .gitignore Entry (if needed)

Check if `.fpf/` should be tracked. Typically **YES** — this is valuable project knowledge.

Suggest adding to project `.gitignore` only if user explicitly wants ephemeral FPF:
```
# Uncomment to exclude FPF from version control
# .fpf/
```

## Output

Confirm initialization:

```
✓ FPF initialized.

Structure created:
  .fpf/
  ├── knowledge/
  │   ├── L0/        (observations, hypotheses)
  │   ├── L1/        (logically verified)
  │   ├── L2/        (empirically tested)
  │   └── invalid/   (disproved — kept for learning)
  ├── evidence/      (test results, research findings)
  ├── decisions/     (DRRs — Design Rationale Records)
  ├── sessions/      (archived reasoning cycles)
  ├── session.md     (current cycle state)
  └── config.yaml    (project settings — optional)

Assurance Levels:
  L0 → L1: Pass logical consistency check (/q2-check)
  L1 → L2: Pass empirical verification (/q3-test or /q3-research)
  
Key Principle:
  Assurance = min(evidence) — weakest link caps everything.

Next: /q1-hypothesize <problem statement>
```

## If Already Initialized

Show current state instead of reinitializing:

```
FPF already initialized.

Current state:
  Phase: [current phase]
  Problem: [current problem or "none"]
  Hypotheses: L0=[N] L1=[N] L2=[N] Invalid=[N]
  Evidence: [N] files
  Decisions: [N] DRRs

Use /q-status for detailed view.
To start fresh: delete .fpf/ directory and run /q0-init again.
```
