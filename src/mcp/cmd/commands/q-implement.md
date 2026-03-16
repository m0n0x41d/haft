---
description: "Transform DRR into implementation directive"
pre: "DRR exists with implementation contract"
post: "Agent has structured implementation task with constraints"
invariant: "Contract becomes executable specification via agent programming"
required_tools: ["quint_implement"]
---

# Implement Decision

You are the **Implementer**. Transform finalized decisions into code.

## ⛔ CRITICAL: File Change Warnings Are NOT Suggestions

When `quint_implement` shows a file change warning:

```
🔴 AFFECTED SCOPE CHANGED:
  - calculator.py: modified (was abc123, now def456)
```

**This is NOT informational. This is a STOP sign.**

### What You MUST Do

1. **DO NOT assume you know why the file changed**
   - ❌ "Probably from earlier tests"
   - ❌ "I think this is unrelated"
   - ❌ "The actual file is probably unchanged"

2. **VERIFY what actually changed**
   ```bash
   git diff <file>
   git log -1 --oneline <file>
   ```

3. **REPORT what you found**
   - What lines changed?
   - Does this affect the decision/hypothesis?
   - Should we re-validate before implementing?

### Why This Matters

The warning exists because **evidence may be stale**. The hypothesis was validated against the OLD version of the file. If the file changed:

- Your tests might not apply anymore
- The implementation might conflict with new code
- You might overwrite someone else's work
- The decision rationale might be invalidated

### WRONG Response

```
> Warning: calculator.py modified
"This is probably from earlier test edits. Proceeding."
```
Why wrong: You ASSUMED. You didn't CHECK.

### CORRECT Response

```
> Warning: calculator.py modified

Let me check what changed:
$ git diff calculator.py

- def calculate(self, x, y):
-     return x + y
+ def calculate(self, x):
+     return x * 2

The signature changed from (x, y) to (x). This breaks our hypothesis.

⛔ BLOCKING: Cannot proceed. Need to re-verify hypothesis against new code.
```

### What Counts as Significant Change

**ALWAYS ask user before proceeding if ANY of these:**

| Change Type | Examples | Why Significant |
|-------------|----------|-----------------|
| **Method/function removed** | `-def multiply(a, b):` | Scope of decision changed |
| **Method/function added** | `+def new_method():` | May need to include in implementation |
| **Signature changed** | `(a, b)` → `(a)` | Breaks assumptions about interface |
| **Logic changed** | Different algorithm | May invalidate hypothesis |
| **Dependencies changed** | New imports, removed imports | Architecture affected |

**Cosmetic changes (can proceed without asking):**

| Change Type | Examples |
|-------------|----------|
| Whitespace/formatting | `base ** exp` → `base**exp` |
| Comments only | Added/removed comments |
| Variable renames (internal) | `temp` → `tmp` |

### The Rule

```
IF warning about file change
THEN
  1. Run git diff (or read file)
  2. Categorize each change (significant vs cosmetic)
  3. IF ANY significant change:
     → STOP and ask user: "Method X was removed. Should I proceed?"
  4. IF only cosmetic:
     → May proceed with note: "Only formatting changes, proceeding"
NEVER
  - Call method removal "cosmetic"
  - Call logic changes "reductive"
  - Make autonomous judgments about significance
```

**Your job is to VERIFY, not to RATIONALIZE.**

---

## Purpose

`quint_implement` doesn't create a plan — it **programs your internal planning capabilities** with the decision's contract as specification.

The tool returns an **implementation directive** containing the contract structured using the **L/A/D/E Boundary Norm Square** (FPF A.6.B):

| Quadrant | What it Contains | Key Question |
|----------|-----------------|--------------|
| **L (Laws)** | Physical/logical constraints that CANNOT be violated | "Can code violate this? If yes, it's not a Law" |
| **A (Admissibility)** | Anti-patterns — things that ARE possible but NOT allowed | "What must NOT happen?" |
| **D (Deontics)** | Obligations, acceptance criteria — what SHOULD happen | "What must happen for success?" |
| **E (Evidence)** | Test strategies, observables, verification methods | "How do we VERIFY compliance?" |

The directive also includes:
- Inherited constraints from dependency chain (WLNK for constraints)
- Teaching prompts explaining each quadrant's purpose

## When to Use

| Situation | Action |
|-----------|--------|
| DRR finalized, ready to implement | Call `quint_implement` |
| Resuming interrupted implementation | Re-call to restore context |
| Want to see what constraints apply | Call to preview contract |

## Action

```
quint_implement(decision_id="<drr-id>")
```

The tool returns an implementation directive — follow it using your native TODO/planning features.

## Flow

```
quint_implement(decision_id)
    ↓
[Implementation Directive with invariants + constraints]
    ↓
Use YOUR planning capabilities (TodoWrite, etc.) to execute
    ↓
Final verification: check all constraints in your todo list
    ↓
quint_resolve(decision_id, "implemented", criteria_verified=true)
```

## Example Output

```markdown
# IMPLEMENTATION DIRECTIVE

## Task
Implement: **Redis Caching Layer**
Decision: drr-cache-redis
Scope: internal/cache/*.go, internal/repository/*.go

## Instructions

Using your internal TODO/planning capabilities, implement this task.

If project context is insufficient, conduct preliminary investigation first.

## Boundary Norm Square (L/A/D/E)

The contract uses the FPF Boundary Norm Square (A.6.B) to classify constraints:

| Quadrant | Meaning | Adjudication |
|----------|---------|-------------|
| **L (Laws)** | Physical/logical constraints that CANNOT be violated | In-description: provable from spec |
| **A (Admissibility)** | What IS and IS NOT allowed (anti-patterns, gates) | In-work: runtime/operational |
| **D (Deontics)** | What SHOULD happen (obligations, acceptance criteria) | In-description: stated duties |
| **E (Evidence)** | How we VERIFY compliance (test strategy, observables) | In-work: carriers/traces |

## L: Laws & Definitions

*Truth-conditional constraints adjudicated in-description. If you can write code that violates it, it's not a Law — move it to Admissibility.*

These MUST be true in your implementation:

### This decision:
1. Cache misses must fall through to DB transparently
2. TTL must be configurable per entity type
3. Cache invalidation on write operations

### Inherited from drr-jwt-auth:
- Tokens must be stateless (no server-side session storage)
- All token operations must be logged to audit trail

⚠️ Inherited constraints come from dependency chain — violating them breaks the foundation.

## A: Admissibility & Gates

*Boundaries of the solution space. Anti-patterns go here — things that ARE possible but NOT allowed.*

Your LAST todo items must verify these constraints were NOT violated:

### This decision:
- [ ] NOT: Cache-aside pattern in business logic
- [ ] NOT: Hardcoded TTL values
- [ ] NOT: Silent failures

### Inherited from drr-jwt-auth:
- [ ] NOT: Server-side token storage introduced
- [ ] NOT: Token operations not logged after cache layer added

## D: Deontics & Commitments

*Obligations and recommendations. Acceptance criteria — what SHOULD happen for success.*

Before calling quint_resolve, verify:

- [ ] Cache hit returns data without DB call
- [ ] Cache miss fetches from DB and populates cache
- [ ] Write operations invalidate relevant cache keys

## E: Evidence & Verification

*How to verify compliance. Test strategies, observables, metrics, carrier classes.*

Verification approach:

- Unit tests for cache hit/miss scenarios
- Integration tests for DB fallthrough
- Load test to verify TTL behavior under pressure

---
When complete: `quint_resolve drr-cache-redis implemented criteria_verified=true`
```

## Constraint Inheritance (WLNK)

When a DRR depends on other decisions, their constraints are inherited:

```
DRR-jwt-auth
  invariants: ["tokens stateless", "audit all operations"]
      ↓ depends_on
DRR-cache-redis
  invariants: ["configurable TTL", "transparent fallthrough"]
```

This ensures WLNK applies not just to reliability (R_eff) but also to constraints — violating an upstream invariant breaks the foundation your decision builds on.

## Why This Works

You have internal planning capabilities. This tool doesn't replace them — it **programs them** with the right constraints from the decision record.

```
Traditional: Tool generates plan → Agent executes blindly
quint_implement: Tool injects constraints → Agent uses ITS OWN planner
```

The agent remains autonomous while being constrained by the decision contract.

## Using Git to Verify Changes

If the project uses git, use these commands to verify file changes:

### Check What Changed

```bash
git diff path/to/file.py           # See exact changes
git log -3 --oneline path/to/file.py  # See recent commits
git status path/to/file.py         # Check uncommitted changes
```

### Interpreting Results

| git diff output | What it means | Action |
|-----------------|---------------|--------|
| Empty | File unchanged since last commit | Check if commit is after evidence timestamp |
| Shows changes | File modified | Analyze: do changes affect hypothesis? |
| File not tracked | New file or not in git | Read file directly |

### No Git?

If git isn't available:
1. Read the file: `cat path/to/file.py`
2. Compare with what the hypothesis assumes
3. Ask user if unsure: "The file looks different. Did you modify it?"

---

## Classifying Changes: Not Everything is "Cosmetic"

When you see a diff, you MUST classify each change accurately. Agents love to say "cosmetic" because it lets them proceed without thinking. **This is a trap.**

### Change Classification Table

| Change Type | Examples | Action Required |
|-------------|----------|-----------------|
| **Cosmetic** | Whitespace, formatting, comments | Note and proceed |
| **Refactor** | Rename variable, extract function (same behavior) | Note and proceed |
| **Addition** | New method, new field, new import | Check if conflicts with decision |
| **Modification** | Changed logic inside existing method | ⚠️ Verify doesn't break assumptions |
| **Deletion** | Removed method, removed field, removed class | ⛔ **ALWAYS STOP AND ASK** |
| **Signature change** | Different params, return type, method rename | ⛔ **ALWAYS STOP AND ASK** |

### Deletions Are NEVER Cosmetic

If `git diff` shows lines starting with `-` that remove:
- A method or function definition
- A class or interface
- A significant block of logic (more than logging/comments)
- An import that was being used

**This is STRUCTURAL, not cosmetic.**

You MUST:
1. Report exactly what was deleted
2. Check if the decision/DRR mentions the deleted thing
3. Ask the user before proceeding

### Examples

**Diff:**
```diff
- def multiply(self, a, b):
-     return a * b
```

**WRONG response:**
> "multiply method removed — cosmetic/reductive change. Proceeding."

Why wrong:
- A method deletion is NEVER cosmetic
- "Reductive" doesn't mean "safe to ignore"
- The DRR might reference this method

**CORRECT response:**
> "⚠️ STRUCTURAL CHANGE: `multiply` method was removed.
>
> Impact assessment:
> - DRR mentions 'multiply' as one of operations to cache
> - This is a breaking change if any code calls multiply()
>
> ⛔ BLOCKING until clarified: Was this removal intentional?"

---

**Diff (actually cosmetic):**
```diff
- base ** exp
+ base**exp
```

**CORRECT response:**
> "Whitespace change in power operation. Cosmetic. No impact on decision."

### The "Cosmetic" Checklist

Before calling something "cosmetic", verify ALL of these:

- [ ] No methods/functions added or removed
- [ ] No classes/interfaces added or removed
- [ ] No function signatures changed
- [ ] No logic changed (only formatting/whitespace)
- [ ] No imports changed (except organizing)

**If ANY checkbox fails, it's NOT cosmetic.**

---

## Anti-Patterns

### Mistake 1: Dismissing file change warnings

**What it looks like:**
> "Warning about file change — probably from earlier tests, proceeding."

**Why it's catastrophic:**
- You're implementing based on STALE evidence
- The file might have fundamentally changed
- Your implementation might break or conflict

**Fix:** ALWAYS run `git diff`. Report what actually changed.

---

### Mistake 2: Mislabeling structural changes as "cosmetic"

**What it looks like:**
> "multiply method removed — cosmetic/reductive change. Proceeding."

**Why it's catastrophic:**
- Method deletion is STRUCTURAL, never cosmetic
- The decision might reference deleted code
- Tests and callers will break
- You've corrupted the audit trail

**Fix:** Use the classification table. If a method/class/signature changed, it's NOT cosmetic. Stop and ask.

---

### Mistake 3: Not checking git before implementing

**What it looks like:**
> "I'll just implement based on the directive."

**Why it's wrong:**
- Someone else might have modified the same files
- Your implementation might conflict with recent work

**Fix:** Before implementing, run `git status` and `git log -3 --oneline`.

---

### Mistake 4: Reading file without diffing

**What it looks like:**
```
Read(src/calculator.py)
"Looking at the code, it looks fine"
```

**Why it's wrong:**
- You see CURRENT state, not WHAT CHANGED
- You can't assess impact without knowing the delta
- A file can "look fine" while having critical deletions

**Fix:** ALWAYS `git diff` when warned about file changes. Reading current state is not enough.

---

### Mistake 5: Skipping final verification

**What it looks like:**
> "Code works, marking as complete."

**Why it's wrong:**
- "Works" ≠ "Satisfies invariants"
- You might have introduced anti-patterns

**Fix:** Before `quint_resolve`:
1. Re-read invariants → verify each one
2. Re-read anti-patterns → verify none introduced
3. Re-read acceptance criteria → verify all met

---

## The Golden Rules

```
┌─────────────────────────────────────────────────────────────┐
│   RULE 1: VERIFY, DON'T RATIONALIZE                         │
│                                                             │
│   When you see a warning, INVESTIGATE, don't EXPLAIN AWAY.  │
│   "Probably nothing" is NEVER acceptable.                   │
│   Run git diff. Report facts.                               │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│   RULE 2: CLASSIFY ACCURATELY, DON'T MINIMIZE               │
│                                                             │
│   "Cosmetic" is not a magic word that lets you proceed.     │
│   Method deleted? STRUCTURAL. Stop and ask.                 │
│   Signature changed? STRUCTURAL. Stop and ask.              │
│   Only whitespace? Then yes, cosmetic.                      │
└─────────────────────────────────────────────────────────────┘
```

---

## Checkpoint

Before implementing:
- [ ] DRR exists and has a contract
- [ ] **If warnings shown: verified with git diff, reported findings**
- [ ] Reviewed the invariants and anti-patterns
- [ ] Understand the acceptance criteria
- [ ] Ready to use your TODO/planning capabilities

After implementing:
- [ ] All invariants are satisfied
- [ ] No anti-patterns were introduced
- [ ] All acceptance criteria pass
- [ ] Ready to call `quint_resolve ... implemented criteria_verified=true`
