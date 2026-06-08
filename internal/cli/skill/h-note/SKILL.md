---
name: h-note
description: |
  Records a fact, observation, or small non-binding rationale into the haft artifact graph — lighter than a full DecisionRecord but persisted so future sessions and conflict detection can surface it. Make sure to use this skill whenever the user says "remember that", "FYI for later", "note that", "side note", "for the record", "worth noting", "TIL", "important caveat", "save this thought" — or whenever a project fact belongs in memory but does not justify the full DRR ceremony. The kernel rejects content-free notes. For binding choices use h-decide (manual-only). For framing problems use h-frame.
when_to_use: |
  Fact, observation, or small non-binding rationale worth persisting across sessions, but lighter than a binding DecisionRecord.
argument-hint: "[note text — what + why]"
allowed-tools: Bash mcp__haft__haft_note mcp__haft__haft_problem mcp__haft__haft_query
---

# h-note — Record a note

You are recording a Note via `mcp__haft__haft_note(...)`. Notes are lightweight
fact/observation carriers in the graph — they do not have the DRR ceremony,
and they are not binding choices. The kernel rejects content-free notes.

## Compact interface discovery

If you need the exact compact contract, run:

```bash
haft interface note.record --json
```

Use that as discovery; do not paste long MCP schemas or CLI help into the
session. For large payloads prefer the input-file path:

```bash
haft artifact create note.record --input-file <input.json> --json
```

`mcp__haft__haft_note(...)` remains the compatible fallback when the host
cannot write a local input file.

## Step 1 — Confirm intent

The operator said something note-worthy. Before persisting:
- Verify the note has substantive content: a fact OR an observation OR non-binding rationale
- Reject: "FYI" alone, "we should remember this" with no payload
- Accept: "Observation: tests run 30s slower on M1 baseline since dependency update"
- Reroute binding choices ("we choose X over Y") to h-decide; do not store them as notes

## Step 2 — Capture rationale explicitly

Every Note must answer at least one of:
- WHAT was observed (atomic facts in `observations`)
- WHERE the evidence came from (`evidence`)
- WHY it matters (`rationale`, optional for bare facts)

## Step 3 — Persist

Use the dedicated note-write tool:

```json
{
  "title": "<short title>",
  "observations": ["<atomic fact>"],
  "rationale": "<optional reason this fact matters>",
  "anchors": [{"type": "relates_to", "ref": "<dec-/prob-/sol-/note-ref>"}]
}
```

Then either run `haft artifact create note.record --input-file <input.json> --json`
or call `mcp__haft__haft_note(...)` with that payload. Do not emulate notes by
creating tactical ProblemCards.

## Step 4 — Confirm to operator

Surface:
- The note ID (the recorded artifact)
- A reminder that future `/h-status` or related-query lookups will surface this note when relevant context arises

## What NOT to do

- DO NOT persist notes that lack rationale. Force the operator to articulate WHY, or refuse and ask for the rationale.
- DO NOT use h-note for binding choices — those go through `/h-decide` (manual-only).
- DO NOT use h-note for full problem framing — that's `/h-frame`.
- DO NOT silently expand a note into a DecisionRecord. If the operator's intent is bigger than a note, recommend `/h-decide` and let them invoke it explicitly.
- DO NOT capture meta-notes about agent behavior ("agent helped me with X") — those are session telemetry, not project knowledge.
- DO NOT emulate notes with `haft_problem(action="frame")`; notes have a dedicated carrier.

## FPF spec references

- DEC-01 — Decision record structure (notes are the lightweight cousin — same problem-frame + decision + rationale + consequences minimum, just compressed)
- E.9 — Design-Rationale Record (full DRR; notes are sub-DRR but still rationale-bearing)

Look up via `mcp__haft__haft_query(action="fpf", query="DEC-01")`.
