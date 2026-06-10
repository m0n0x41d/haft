Record a project fact into the Haft reasoning graph.

Use for: a resolved non-trivial bug's cause + lesson, a ruled-out variant
and why, an operator correction, a surprising codebase discovery, a
constraint found mid-work. Lighter than a decision; the kernel rejects
content-free notes.

Call the native `haft_note` tool:

```json
{
  "title": "<what + why in one line>",
  "observations": ["<atomic fact>", "<another fact>"],
  "rationale": "<why this matters later>",
  "anchors": [{ "type": "problem", "ref": "<prob-...>" }],
  "affected_files": ["<implementation files this fact lives in>"]
}
```

Anchor to related decisions/problems so the fact surfaces in
related/code_context lookups. Prefer implementation files over shared
manifests in affected_files (manifests churn and cause false drift).

A note is a fact carrier, NOT a decision — a choice among alternatives goes
through manual /h-decide.
