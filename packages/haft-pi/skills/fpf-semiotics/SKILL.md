---
name: fpf-semiotics
description: Semiotic discipline for engineering artifacts — keep meaning, distinctions, context, authority, and evidence intact across documents, specs, prompts, handoffs, and rewrites. Use when reading or writing project artifacts, renaming or restructuring docs, merging duplicated descriptions (fanout), unpacking vague umbrella words, or judging what a document actually authorizes.
---

# Semiotics for project work (Levenchuk 2026 seminar distillate)

Semiotic failures are invisible until they surface as rework. What breaks is
rarely the data — it is distinctions, context, authority, and evidence.

## The strict distinctions (hold them always)

- **Object ≠ description ≠ representation ≠ carrier.** A `.haft/*.md` file is
  a carrier; the SpecSection is a description; the running system is the
  object. Editing the carrier changes nothing in the world.
- **Role ≠ capability ≠ method ≠ work.** Assigned ≠ able; method stated ≠
  done; one lucky run ≠ proven capability.
- **Design-time ≠ run-time. Promise ≠ delivery. Plan ≠ reality.**
- Documents do not act. Always ask: which system, in which role, acts on
  this description?

## Reading checklist (run on every note, spec, dashboard, prompt)

1. What is the object of talk here — or is it about everything = nothing?
2. In which context does this make sense — or is it a context mash?
3. Is this a description, requirement, promise, explanation, gate, or
   evidence — or porridge?
4. Which words are ad-hoc local convenience, and which are terms that must
   stay exact?
5. Where does faithful re-expression end and reinterpretation (with possible
   invention) begin?
6. What outcome does this serve — or none anymore?

## Umbrella-word repair

When overloaded words start carrying architectural load — "service",
"quality", "done", "reliable", "based on", "same", "process" — unpack before
designing: identify the trigger word, ground the situation's objects and
relations, restore the local ontology, then choose precise replacement terms.
Refuse to frame problems or write acceptance criteria on umbrella words.
In haft: `haft_query(action="resolve_term")` and
`haft_query(action="fpf", query="A.6.P")` for the repair protocol.

## Boundary square (L/A/D/E)

Mixed normative sentences hide four different claims. Split them:

- **L** — law/definition: what the term means;
- **A** — admissibility/gate: what makes a thing valid;
- **D** — deontics: who MUST do what;
- **E** — work-effect/evidence: what record proves it happened.

"Service guarantees 99.9%" is broken until you name the accountable
promiser (D), the measurement claim with window and carrier (E), and the
gate (A). "Aligned enough to ship" is a policy-bearing shorthand — unpack it
before anyone deploys on it.

## Typical semiosis failures (catch these)

- Same name — different thing; different names — same thing (fanout).
- One document carrying rule + promise + report + evidence at once.
- Handoff transduction loss: the unimportant gets retold, the important
  silently dropped.
- Conservative rewrite that quietly changes claim strength or object of talk
  ("I never asserted that" from the original author = the symptom).
- Representation shift (text → table/dashboard) that adds categories,
  thresholds, or rankings the source never had — then people discuss the
  artifact instead of the meaning.
- Explanation becoming a second semantic track: decisions made on the
  convenient retelling, not the source.
- Authored-unit drift: a doc declared about one topic answering a different
  question by midpoint.
- Strong conclusions from text whose scope, context, and authority were
  never restored.

## Early material corridor

Meaning rarely arrives as a finished requirement. Weak signals and partially
said material must neither be left in private intuition nor published as
settled conclusions. Keep observation, interpretation, and proposed action
separated; preserve source, applicability bounds, and confidence on every
handoff; name the next owner. In haft: persist early material as
`haft_note` with anchors — a note is a fact carrier, not a decision.

## Authority question (normative layer)

For every artifact pair that overlaps (plan vs backlog, spec vs README):
which one is authoritative for which decision, who may split/merge/cancel,
where does status change? Without this, perfect descriptions still produce
desync. In haft the kernel and `.haft/` graph are the authority; everything
else — including this skill — only routes and reminds.
