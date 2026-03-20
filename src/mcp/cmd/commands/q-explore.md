---
description: "Explore solution variants for a framed problem"
---

# Explore Solutions

Generate genuinely distinct solution variants. Each must have a weakest link identified.

Use `quint_solution` tool with `action="explore"` and:
- `problem_ref`: ProblemCard ID (auto-detected if only one active)
- `variants`: array of options, each with:
  - `title`: variant name
  - `description`: what this option does
  - `strengths`: array of advantages
  - `weakest_link`: what bounds this option's quality (REQUIRED)
  - `risks`: array of risk notes
  - `stepping_stone`: true if opens future possibilities
  - `rollback_notes`: how to reverse

At least 2 variants required. Prefer 3+ that differ in kind, not degree.

## After the tool call — what to show the user

The user needs enough detail to evaluate and challenge each variant. Do NOT compress into a summary table alone.

For each variant, present:
1. **Title** and 2-3 sentence core idea (what makes this approach distinct)
2. **Strengths** — not just listed, briefly explain WHY each is an advantage
3. **Weakest link** — the single thing that bounds quality. Explain the mechanism, not just name it
4. **Key risks** — 2-3 most important, with enough context to assess severity
5. **Stepping stone** — if yes, what future paths it opens

End with a quick-reference summary table (variant / weakest link / stepping stone) for scanning, but the table supplements the detail — it doesn't replace it.

If there are diversity warnings, surface them — the user should know if variants are too similar.

$ARGUMENTS
