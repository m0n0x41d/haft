Record an explicitly requested project fact into the Haft graph.

Use when the operator says to remember or save a non-binding observation, or
when a named receiving use needs an addressable fact. Do not auto-persist
ordinary reasoning merely because it might be useful later.

Call the native `haft_note` tool:

```json
{
  "title": "<what + why in one line>",
  "observations": ["<atomic fact>", "<another fact>"],
  "rationale": "<why this matters later>",
  "anchors": [{ "type": "problem", "ref": "<prob-...>" }],
  "affected_files": ["<implementation files this fact lives in>"],
  "entity_ref": {
    "ref_kind_id": "U.EntityRef",
    "reference_id": "<exact current EntityOfConcern>"
  },
  "bounded_context_ref": "<exact current bounded context>"
}
```

Omit the concern fields rather than guessing when exact identity is unknown.
When the typed projection commits, preserve its exact
`Haft.ProjectRecordRef`; do not derive `record:<note-id>`. An
underdetermined projection does not invalidate the saved note.

Anchor to related decisions/problems so the fact surfaces in
related/code_context lookups. Prefer implementation files over shared
manifests in affected_files (manifests churn and cause false drift).

A note is a fact carrier, not a choice, ProblemCard, evidence verdict,
approval, or WorkPlan. A binding choice requires manual `/h-decide`.
