---
description: "Gather external evidence from web and documentation (FPF Induction phase)"
arguments:
  - name: hypothesis
    description: "Specific hypothesis ID to research (optional)"
    required: false
  - name: query
    description: "Specific research question (optional, derives from hypothesis if omitted)"
    required: false
---

# FPF Phase 3: Induction — External Research

## Phase Gate (MANDATORY)

**STOP. Verify phase before proceeding:**

1. Read `.fpf/session.md`, extract `Phase:` value
2. Check validity:

| Current Phase | Can Run? | Action |
|---------------|----------|--------|
| DEDUCTION_COMPLETE | ✅ YES | Proceed |
| INDUCTION_COMPLETE | ✅ YES | Add more evidence |
| Any other | ❌ NO | See below |

**If INITIALIZED or ABDUCTION_COMPLETE:**
```
⛔ BLOCKED: Hypotheses not yet verified for logical consistency.
Run /fpf-2-check first.
```

**If AUDIT_COMPLETE or DECIDED:**
```
⛔ BLOCKED: Cycle already past induction phase.
Current phase: [PHASE]
Start new cycle with /fpf-1-hypothesize if needed.
```

---

## Your Role

You are the **Researcher**. Gather evidence from external sources to verify or refute hypotheses.

This phase answers: **"What does the outside world know about this?"**

## Difference from /fpf-3-test

| /fpf-3-test | This Command (/fpf-3-research) |
|-------------|-------------------------------|
| Run code, benchmarks | Search web, read docs |
| **Internal** evidence | **External** evidence |
| "Does it work HERE?" | "Does it work ELSEWHERE?" |
| Direct applicability | **Requires congruence assessment** |

**Both are Induction** — gathering real-world evidence. Different sources.

**Critical:** External evidence carries a **congruence penalty**. Evidence from a different context (scale, tech version, use case) is less reliable for our situation.

## Input

- `$ARGUMENTS.hypothesis` — Which hypothesis to research
- `$ARGUMENTS.query` — Specific question (or derive from hypothesis assumptions)

## Process

### 1. Load Context

```bash
# Read hypothesis to research
cat .fpf/knowledge/L1/[hypothesis].md

# Read project context for congruence assessment
cat .fpf/context.md

# Extract assumptions needing external validation
grep -A10 "Assumptions" .fpf/knowledge/L1/[hypothesis].md

# Check existing evidence to avoid duplication
ls .fpf/evidence/
```

### 2. Formulate Research Questions

For each assumption that can be validated externally:

```markdown
## Research Plan: [Hypothesis Name]

### Questions to Answer

| Assumption | Research Question | Sources to Check | Priority |
|------------|-------------------|------------------|----------|
| [A1: Redis handles 10k RPS] | "Redis benchmark production workloads" | Redis docs, benchmarks | High |
| [A2: Thread-safe by default] | "Redis thread safety model" | Official docs | High |
| [A3: Team can maintain] | "Redis operational complexity" | Blog posts, HN | Med |
```

### 3. Execute Research

**Use available tools:**

1. **Context7 MCP** (preferred for libraries/frameworks):
   ```
   mcp__context7__resolve-library-id → find library
   mcp__context7__get-library-docs → fetch docs
   ```

2. **Web Search** (for broader questions):
   ```
   WebSearch → find relevant sources
   WebFetch → read full content
   ```

3. **Specific URLs** (if user provided or known):
   ```
   WebFetch → get content directly
   ```

### 4. Evaluate Sources

For each source, assess credibility:

```markdown
## Source Evaluation

| Source | Type | Credibility | Date | Relevance |
|--------|------|-------------|------|-----------|
| [URL/Doc] | Official docs | High | [date] | Direct |
| [URL] | Tech blog (reputable) | Medium | [date] | Related |
| [URL] | Forum/HN | Low-Med | [date] | Anecdotal |
```

**Credibility Hierarchy:**
1. **Official documentation** — Highest
2. **Peer-reviewed / reputable tech blogs** (major companies, known experts)
3. **Stack Overflow** (accepted + high votes)
4. **Random blog posts** — Lower
5. **Forum discussions** — Lowest (anecdotal)

### 5. Assess Congruence (CRITICAL)

**For EVERY external source, assess how well it matches our context.**

```markdown
## Congruence Assessment: [Source]

| Dimension | Source Context | Our Context | Match |
|-----------|---------------|-------------|-------|
| **Technology** | [version/variant] | [our version] | ✓/⚠/✗ |
| **Scale** | [their scale] | [our scale] | ✓/⚠/✗ |
| **Use Case** | [their use case] | [our use case] | ✓/⚠/✗ |
| **Environment** | [their env] | [our env] | ✓/⚠/✗ |
| **Constraints** | [their constraints] | [our constraints] | ✓/⚠/✗ |

**Overall Congruence Level:** High / Medium / Low

**Justification:** [Why this level]
```

**Congruence Levels and Penalties:**

| Level | Description | Φ(CL) Penalty | Effective R |
|-------|-------------|---------------|-------------|
| **High** | Direct match: same tech, similar scale, similar use case | 0.00 | R × 1.00 |
| **Medium** | Partial match: same tech different version, or different scale (±1 order of magnitude) | 0.15 | R × 0.85 |
| **Low** | Weak match: related but different tech, very different scale, different use case | 0.35 | R × 0.65 |

**Formula:** `R_eff = max(0, R_base - Φ(CL))`

⚠️ **Low-congruence evidence should be flagged in /fpf-4-audit and verified internally if possible.**

### 6. Create Evidence Artifact

For each significant finding, create `.fpf/evidence/[date]-[name].md`:

```markdown
---
id: [slug]
type: external-research
source: web | docs | paper
created: [timestamp]
hypothesis: [path to hypothesis file]
assumption_tested: "[which assumption]"
valid_until: [date — consider source freshness]
decay_action: refresh | deprecate
congruence:
  level: high | medium | low
  penalty: 0.00 | 0.15 | 0.35
  source_context: "[their context summary]"
  our_context: "[our context summary]"
  justification: "[why this congruence level]"
sources:
  - url: [URL]
    title: [title]
    type: [official-docs | tech-blog | forum | paper]
    accessed: [date]
    credibility: high | medium | low
scope:
  applies_to: "[when this evidence is relevant]"
  not_valid_for: "[when this doesn't apply]"
---

# Research: [Topic]

## Purpose
[What we were trying to find out]

## Hypothesis Reference
- **File:** `.fpf/knowledge/L1/[hypothesis].md`
- **Assumption tested:** [specific assumption]

## Congruence Assessment

**Source context:** [e.g., "Large enterprise, 1M users, AWS, Redis 7.0"]
**Our context:** [e.g., "Startup, 10k users, GCP, Redis 6.2"]

| Dimension | Match | Notes |
|-----------|-------|-------|
| Technology | ⚠ | Different Redis version |
| Scale | ⚠ | 100x difference |
| Use Case | ✓ | Similar caching pattern |
| Environment | ⚠ | Different cloud |

**Congruence Level:** Medium
**Penalty:** 0.15
**R_eff:** 0.85 (if base R = 1.0)

⚠️ **Note:** Scale difference means performance claims may not transfer directly. Consider internal benchmarking to verify.

## Findings

### Source 1: [Name]

**URL:** [url]
**Type:** [official-docs / tech-blog / etc]
**Credibility:** [High/Medium/Low]
**Accessed:** [date]

**Key points:**
- [Point 1]
- [Point 2]

**Relevant quote (if important):**
> [Direct quote — keep short]

**Limitations:**
- [e.g., "Old post from 2019"]
- [e.g., "Different scale than ours"]

### Source 2: [Name]
[Same structure...]

## Synthesis

[What the combined evidence suggests, accounting for congruence]

## Verdict

- [ ] Assumption **SUPPORTED** by external evidence (with congruence: [level])
- [ ] Assumption **CONTRADICTED** by external evidence
- [ ] **MIXED** evidence — need internal testing to resolve
- [ ] **INSUFFICIENT** evidence — need more research
- [ ] **LOW CONGRUENCE** — supports hypothesis but verify internally

## Gaps

[What we couldn't find / still uncertain about]

## Recommendations

[Next steps based on findings]
```

### 7. Update Hypothesis

Add research findings to hypothesis file:

```yaml
---
external_evidence:
  - file: ../evidence/[research-file].md
    congruence: medium
    finding: [brief summary]
research_notes: |
  External sources [support/contradict/mixed on] [assumption].
  Key finding: [summary]
  Congruence concern: [if any]
---
```

### 8. Update Session

```markdown
## Status
Phase: INDUCTION_COMPLETE

## Research Summary
| Hypothesis | Sources Checked | Findings | Congruence |
|------------|-----------------|----------|------------|
| [H1] | 5 | Assumptions supported | Mixed (2 high, 3 med) |
| [H2] | 3 | Mixed evidence | Low (needs internal test) |

## Phase Transitions Log
| Timestamp | From | To | Trigger |
|-----------|------|-----|---------|
| [...] | ... | ... | ... |
| [now] | DEDUCTION_COMPLETE | INDUCTION_COMPLETE | /fpf-3-research |

## Next Step
- `/fpf-3-test` for internal empirical verification (especially for low-congruence findings)
- `/fpf-4-audit` if confident in combined evidence
- `/fpf-5-decide` to finalize (audit recommended first)
```

## Output Format

```markdown
## External Research Complete

### [Hypothesis Name]

**Questions Researched:** [N]
**Sources Consulted:** [N]

### Findings Summary

| Assumption | Verdict | Sources | Congruence | Confidence |
|------------|---------|---------|------------|------------|
| [A1] | ✓ Supported | 3 | High | High |
| [A2] | ✗ Contradicted | 2 | High | High |
| [A3] | ~ Mixed | 4 | Medium | Medium |

### Congruence Analysis

| Source | Credibility | Congruence | Φ Penalty | Notes |
|--------|-------------|------------|-----------|-------|
| Redis Official Docs | High | High | 0.00 | Direct match |
| TechCorp Blog | Medium | Medium | 0.15 | Different scale |
| HN Discussion | Low | Low | 0.35 | Anecdotal |

**Lowest congruence:** [source] at [level] — this caps R_eff via WLNK

### Evidence Created
- `.fpf/evidence/[file1].md` (congruence: high)
- `.fpf/evidence/[file2].md` (congruence: medium)

### Gaps / Uncertainties
- [What we couldn't confirm externally]
- [Where internal testing would help]

### Recommendations
- [ ] Sufficient external evidence → proceed to `/fpf-4-audit`
- [ ] Need internal testing → run `/fpf-3-test` for low-congruence items
- [ ] Need more research → specify what to search

---

**Next Step:**
- `/fpf-3-test` — Internal testing (especially for low-congruence findings)
- `/fpf-4-audit` — Critical review (will flag congruence issues)
```

## Research Quality Checklist

| Quality | Check | ✓/✗ |
|---------|-------|-----|
| **Multiple sources** | Not relying on single source? | |
| **Source credibility** | Prioritized official docs? | |
| **Recency** | Information not stale? | |
| **Relevance** | Actually answers our question? | |
| **Contradictions noted** | Documented conflicting info? | |
| **Congruence assessed** | Context match evaluated for ALL sources? | |
| **Penalties applied** | Low congruence flagged? | |

## Common Mistakes to Avoid

| Mistake | Why It's Wrong | Do This Instead |
|---------|----------------|-----------------|
| Single source | Could be wrong/biased | Find multiple sources |
| Only positive results | Confirmation bias | Look for contradicting evidence too |
| Ignoring dates | Tech changes fast | Check recency, set validity window |
| Blog > Official docs | Inverted credibility | Prioritize official sources |
| No source links | Can't verify later | Always include URLs and access dates |
| **Ignoring congruence** | Evidence may not apply to our context | Always assess congruence and apply penalties |
| Treating all external evidence equally | Different contexts = different reliability | Use Φ(CL) penalty in calculations |
