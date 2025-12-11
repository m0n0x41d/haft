---
description: "Gather external evidence from web and documentation (FPF Research phase)"
arguments:
  - name: hypothesis
    description: "Specific hypothesis ID to research (optional)"
    required: false
  - name: query
    description: "Specific research question (optional, derives from hypothesis if omitted)"
    required: false
---

# FPF Phase 3-Research: External Evidence Gathering

## Your Role

You are the **Researcher**. Gather evidence from external sources to verify or refute hypotheses.

This phase answers: "What does the outside world know about this?"

## Difference from /fpf:3-test

| /fpf:3-test | /fpf:3-research |
|-------------|-----------------|
| Run code, benchmarks | Search web, read docs |
| Internal evidence | External evidence |
| "Does it work HERE?" | "Does it work ELSEWHERE?" |
| Empirical (our data) | Empirical (others' data) |

**Both are Induction** — gathering real-world evidence. Different sources.

## Input

- `$ARGUMENTS.hypothesis` — Which hypothesis to research
- `$ARGUMENTS.query` — Specific question (or derive from hypothesis assumptions)

## Process

### 1. Load Context

```bash
# Read hypothesis to research
cat .fpf/knowledge/L1/[hypothesis].md

# Extract assumptions needing external validation
grep -A10 "Assumptions" .fpf/knowledge/L1/[hypothesis].md
```

### 2. Formulate Research Questions

For each assumption, create research questions:

```markdown
## Research Plan: [Hypothesis Name]

### Questions to Answer
| Assumption | Research Question | Sources to Check |
|------------|-------------------|------------------|
| [A1: Redis handles 10k RPS] | "Redis benchmark production workloads" | Web, Redis docs |
| [A2: Thread-safe by default] | "Redis thread safety model" | Official docs |
| [A3: Team can maintain] | "Redis operational complexity" | Blog posts, HN |
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

For each source, assess:

```markdown
## Source Evaluation

| Source | Type | Credibility | Recency | Relevance |
|--------|------|-------------|---------|-----------|
| [URL/Doc] | Official docs | High | [date] | Direct |
| [URL] | Blog post | Medium | [date] | Anecdotal |
| [URL] | HN discussion | Low-Med | [date] | Opinions |
```

**Credibility hierarchy:**
1. Official documentation
2. Peer-reviewed / reputable tech blogs
3. Stack Overflow (accepted + high votes)
4. Random blog posts
5. Forum discussions

### 5. Create Evidence Artifacts

For each significant finding, create `.fpf/evidence/[name].md`:

```markdown
---
id: [slug]
type: external-research
source: web | docs | paper
created: [timestamp]
hypothesis: [reference]
assumption_tested: [which one]
valid_until: [date — consider source freshness]
decay_action: refresh | deprecate | waive
congruence:
  level: high | medium | low
  source_context: "[where this evidence comes from]"
  our_context: "[our situation]"
  notes: "[why congruence is high/medium/low]"
  penalty_reason: "[if low, why it still matters]"
sources:
  - url: [URL]
    title: [title]
    accessed: [date]
    credibility: high | medium | low
scope:
  applies_to: "[when this evidence is relevant]"
  not_valid_for: "[when this doesn't apply to us]"
---

# Research: [Topic]

## Question
[What we were trying to find out]

## Congruence Assessment
**Source context:** [e.g., "Large enterprise, 1M users, AWS"]
**Our context:** [e.g., "Startup, 10k users, GCP"]
**Congruence:** [high/medium/low]
**Reasoning:** [Why evidence transfers or doesn't]

⚠️ If congruence is LOW:
- Evidence may not apply to our situation
- Consider as supporting info, not proof
- Seek internal testing to confirm

## Findings

### Source 1: [Name]
**URL:** [url]
**Credibility:** [high/medium/low]
**Key points:**
- [Point 1]
- [Point 2]

**Relevance to our context:**
[How well does this apply to us?]

**Relevant quote:**
> [Direct quote if important]

**Limitations:**
- [e.g., "Old post from 2019", "Different scale than ours"]

### Source 2: [Name]
...

## Synthesis
[What the combined evidence suggests]

## Verdict
- [ ] Assumption SUPPORTED by external evidence
- [ ] Assumption CONTRADICTED by external evidence
- [ ] MIXED evidence — need internal testing
- [ ] INSUFFICIENT evidence — need more research
- [ ] LOW CONGRUENCE — supports hypothesis but verify internally

## Gaps
[What we couldn't find / still uncertain about]
```

### 6. Update Hypothesis

Add research findings to hypothesis file:

```yaml
---
external_evidence:
  - ../evidence/[research-file].md
research_notes: |
  External sources support/contradict [assumption].
  Key finding: [summary]
---
```

### 7. Update Session

```markdown
## Status
Phase: RESEARCH_COMPLETE

## Research Summary
| Hypothesis | Sources Checked | Findings |
|------------|-----------------|----------|
| [H1] | 5 | Assumptions supported |
| [H2] | 3 | Mixed evidence |

## Next Step
- `/fpf:3-test` for internal empirical verification
- `/fpf:4-audit` if confident in combined evidence
```

## Output Format

```markdown
## Research Complete

### [Hypothesis Name]

**Questions Researched:** [N]
**Sources Consulted:** [N]

### Key Findings

**Assumption 1: [statement]**
- Status: ✓ Supported / ✗ Contradicted / ~ Mixed
- Sources: [list]
- Summary: [one line]

**Assumption 2: [statement]**
- Status: ✓ Supported / ✗ Contradicted / ~ Mixed
- Sources: [list]
- Summary: [one line]

### Evidence Created
- `.fpf/evidence/[file1].md`
- `.fpf/evidence/[file2].md`

### Credibility Assessment
- High-credibility sources: [N]
- Medium-credibility: [N]
- Low-credibility: [N]

### Gaps / Uncertainties
- [What we couldn't confirm externally]
- [Where internal testing is still needed]

### Recommendation
- [ ] Sufficient external evidence → proceed to `/fpf:4-audit`
- [ ] Need internal testing → run `/fpf:3-test`
- [ ] Need more research → specify what to search
```

## Research Quality Checklist

| Quality | Check |
|---------|-------|
| Multiple sources | Not relying on single source? |
| Source credibility | Prioritized official docs? |
| Recency | Information not stale? |
| Relevance | Actually answers our question? |
| Contradictions noted | Documented conflicting info? |
| **Congruence assessed** | Context match evaluated? |

## Congruence Levels

**High congruence** — Evidence directly applies:
- Same technology/version
- Similar scale (±1 order of magnitude)
- Similar use case
- Same constraints

**Medium congruence** — Evidence partially applies:
- Same technology, different version
- Different scale but same patterns expected
- Similar but not identical use case
- Some constraints differ

**Low congruence** — Evidence weakly applies:
- Different but related technology
- Very different scale
- Different use case, some overlap
- Many constraints differ

⚠️ **Low congruence evidence should be flagged in `/fpf:4-audit`**

## Anti-Patterns

| Anti-Pattern | Why It's Wrong |
|--------------|----------------|
| Single source | Could be wrong/biased |
| Only positive results | Confirmation bias |
| Ignoring dates | Tech changes fast |
| Blog > Official docs | Inverted credibility |
| No source links | Can't verify later |
