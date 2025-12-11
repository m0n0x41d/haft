---
description: "Search FPF knowledge base"
arguments:
  - name: topic
    description: "Topic, keyword, or domain to search for"
    required: true
  - name: level
    description: "Filter by level: L0, L1, L2, invalid, all (default: all)"
    required: false
---

# FPF Query

## Purpose

Search project knowledge base for relevant epistemes and decisions.

## Input

- `$ARGUMENTS.topic` — Search term(s)
- `$ARGUMENTS.level` — Optional filter (L0, L1, L2, invalid, all)

## Process

### 1. Search Knowledge Files

```bash
# Search across all knowledge levels
grep -r -l -i "$TOPIC" .fpf/knowledge/

# Or filter by level
grep -r -l -i "$TOPIC" .fpf/knowledge/L2/  # Only verified
```

### 2. Search Decisions

```bash
grep -r -l -i "$TOPIC" .fpf/decisions/
```

### 3. Search Evidence

```bash
grep -r -l -i "$TOPIC" .fpf/evidence/
```

### 4. Compile Results

For each match, extract:
- File path
- Title (from frontmatter or first heading)
- Status/Level
- Relevance snippet

## Output Format

```markdown
## Knowledge Query: "[topic]"

### Verified Knowledge (L2)
[Most reliable — empirically tested]

**[episteme-name]** `.fpf/knowledge/L2/[file].md`
> [Relevant snippet or summary]
> Evidence: [linked evidence files]
> Decided in: [DRR reference if any]

---

### Reasoned Knowledge (L1)  
[Logically verified, not empirically tested]

**[episteme-name]** `.fpf/knowledge/L1/[file].md`
> [Relevant snippet]
> Needs: Empirical verification

---

### Observations (L0)
[Unverified — treat with caution]

**[episteme-name]** `.fpf/knowledge/L0/[file].md`
> [Relevant snippet]
> Status: Hypothesis / Observation

---

### Related Decisions

**DRR-[NNN]: [title]** `.fpf/decisions/DRR-[NNN].md`
> [How it relates to query]

---

### Related Evidence

**[evidence-name]** `.fpf/evidence/[file].md`
> [What it tested/showed]

---

### Summary

| Level | Matches |
|-------|---------|
| L2 (Verified) | [N] |
| L1 (Reasoned) | [N] |
| L0 (Observations) | [N] |
| Decisions | [N] |
| Evidence | [N] |

**Confidence for "[topic]":** 
[High if L2 matches / Medium if L1 / Low if only L0 / None if no matches]
```

## No Results

```markdown
## Knowledge Query: "[topic]"

No matches found in knowledge base.

### Suggestions
- Check spelling / try synonyms
- This topic may not have been investigated yet
- Start investigation: `/fpf:1-hypothesize "[topic]-related question"`

### Related (fuzzy)
[If partial matches exist, suggest them]
```

## Usage Examples

```bash
# Find everything about caching
/fpf:query caching

# Only verified knowledge about auth
/fpf:query auth --level L2

# What do we know about performance?
/fpf:query performance

# Check specific technology
/fpf:query redis
```

## Integration with Decision Making

When making decisions, query first:

```markdown
Before: /fpf:1-hypothesize "should we use Redis?"
Do:     /fpf:query redis
        /fpf:query caching
        
[Check if we already have verified knowledge on this topic]
```

If L2 knowledge exists → may not need full ADI cycle, just reference existing DRR.
