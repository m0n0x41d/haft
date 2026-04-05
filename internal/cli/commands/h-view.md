---
description: "Advanced: render a canonical brief, rationale, audit, or compare view from live Haft state"
---

# View

Render one deterministic projection of the current Haft artifact graph. This is an advanced publishing/handoff command, not a replacement for `/h-frame`, `/h-explore`, `/h-compare`, or `/h-decide`.

Use `haft_query` tool with `action="projection"` and:
- `view`: one of `engineer`, `manager`, `audit`, `compare`, `delegated-agent`, `change-rationale`
- `context`: optional context filter

Prefer these aliases from user intent:
- `brief`, `handoff`, `delegated` -> `delegated-agent`
- `rationale`, `pr`, `change` -> `change-rationale`
- `status` -> `manager`
- `evidence` -> `audit`
- `pareto` -> `compare`

When to use it:
- after `/h-decide`, when you need a clean implementation handoff -> `delegated-agent`
- after implementation/measurement, when you need PR or change summary text -> `change-rationale`
- during review or refresh, when you need evidence posture -> `audit`
- when the user asks for the current trade-off surface -> `compare`

Do NOT use projections to replace missing reasoning. If the underlying artifacts do not exist yet, go back and create them with the normal FPF commands first.

$ARGUMENTS
