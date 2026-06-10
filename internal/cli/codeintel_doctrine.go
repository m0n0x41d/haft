package cli

import (
	"strings"

	"github.com/m0n0x41d/haft/internal/project"
)

// codeIntelDoctrine is the always-on playbook for the haft code-graph tools,
// emitted in the MCP `initialize` response (so every MCP client surfaces it in
// the agent's system prompt, regardless of whether per-tool schemas are deferred
// or how many other servers the user runs). Kept tight — it is read every
// session. The moat vs a plain code-graph: results are FUSED with the decision
// graph, so the doctrine pushes "consult before editing" not just to understand
// structure but to avoid violating a recorded decision.
const codeIntelDoctrine = `# haft code graph — code intelligence FUSED with the decision graph

` + "`haft_query`" + ` is a graph of this repo's symbols + call/dispatch edges where
every result is fused with the reasoning graph: the decisions, invariants, and
problems recorded about that code. The index self-refreshes (rebuilt on change),
so results are current. Consult it BEFORE writing or editing code, not during.

## Why before editing (the point)
Other code graphs tell you HOW code is wired. This one also tells you what was
DECIDED about it. Calling explore / code_context before a change surfaces the
decisions and invariants governing the symbol — so you avoid breaking a recorded
decision, not just misreading structure. A blind Read/Grep cannot see this.

## Tool selection by intent — haft_query(action=…)
- "how does X work / the flow / understand this area" -> explore (PRIMARY: ONE
  call returns the connected call chain, each symbol's governing decisions +
  invariants, the blast radius, and verbatim source — usually the only call needed)
- "before I touch this file/symbol, what's decided here?" -> code_context
- "what breaks if I change X / who depends on it?" -> impact (callers + covering
  decisions) — call BEFORE editing a symbol
- "what calls X / what does X call?" -> callers / callees
- "read this symbol's source (and all overloads) with its governance" -> node
  (instead of Read: byte-exact, freshness-revalidated, per-overload fusion)
- "find a decision / problem / note / fact by MEANING, not a symbol" -> search
  (FTS over the reasoning graph; splits compound terms, and a kind: qualifier —
  kind:DecisionRecord / ProblemCard / Note / … — narrows to that artifact kind)

## Seeds are forgiving — you don't need the exact name
- Don't know the exact name? Pass a FRAGMENT — the actions match by substring AND
  tolerate typos (a misspelled fragment still resolves by edit distance), then
  either use the single hit (labeled "fuzzy") or list candidates. So you never
  Grep just to find a symbol's name.
- NARROW a seed with field qualifiers when a fragment is ambiguous:
  kind:/lang:/path:/name: (e.g. "kind:func Handler", "path:internal/cli Frame").
- explore takes SEVERAL names at once (space- or comma-separated, e.g.
  "FrameProblem Create") and shows the call path connecting them — or says
  honestly there is no static path. Use this for "how does X reach Y".

## Honest coverage — heed the markers
Unresolved dynamic dispatch is dropped and marked, never guessed. An empty
"callers" says some call forms (reflection, callbacks) may be unresolved — do
NOT read it as "safe to change". A dispatch hop is flagged heuristic. Trust
resolved results (AST + decision join); don't re-verify them with grep.

## Anti-patterns
- Don't blind Read/Grep to understand an area explore answers in one fused call.
- Don't edit a governed symbol without code_context/impact first — the decision
  you'd violate is exactly what the fusion exists to surface.`

// methodPackDoctrine is emitted in MCP initialize instructions so hosts learn
// the pull/close protocol before any tool call. Keep it short: the full method
// cards are task-local and retrieved through haft_method(action="pull").
const methodPackDoctrine = `# Haft MethodPack — task-local engineering gates

This is the default code-work loop. Treat it as a habit trigger, not advice:
before non-trivial code action, pull the task-local method; before claiming
done, close the same run with evidence.

1. Before non-trivial code work, call ` + "`haft_method(action=\"pull\")`" + `. Pass the
   operator task, declared_task_kind, change_intent, intended_files, and known
   risk_signals. Non-trivial means feature, bugfix/debug, refactor, external
   integration, governed files, cross-module edits, behavior changes, or
   failing tests.
2. ` + "`pull`" + ` creates an open MethodRun (` + "`mpull-*`" + `) and returns compact cards:
   max 3 methods, max 3 hard gates per method. It is not an internal LLM classifier;
   the agent declares the task shape and the kernel matches methods deterministically.
3. Keep the ` + "`pull_id`" + `. If context compacts, recover open runs with
   ` + "`haft_method(action=\"status\")`" + ` or
   ` + "`haft_method(action=\"show\", pull_id=\"mpull-...\")`" + `.
4. Before claiming completion, call
   ` + "`haft_method(action=\"close\", pull_id=\"mpull-...\")`" + ` with changed_files,
   gate_results, verification evidence, and explicit waivers. This is a
   checklist-style closeout; hard gates require evidence or a waiver reason.
   Soft gates are advisory.

` + "`haft init`" + ` installs the built-in method catalog by default under
` + "`.haft/methods/swe-core`" + `; no extra setup command is needed.
Mechanical edits should request low or no ceremony and should not get architecture gates.`

// sessionStartMandate is the single loudest rule, placed FIRST in the server
// instructions: a governed project's working memory lives in the haft graph, so
// the agent must load that state before touching anything. Maximum weight on
// purpose — it is the one action that prevents reasoning on a stale graph,
// re-deciding settled questions, or editing code a recorded decision governs.
const sessionStartMandate = `# ⚠ MANDATORY FIRST ACTION — run the haft status check before anything else

CRITICAL, NON-NEGOTIABLE: the FIRST thing you do in EVERY fresh session on this
project — before reading, searching, planning, or editing ANY code — is call
` + "`haft_query(action=\"status\")`" + `. It returns the project's live governance
state in one shot.

This is a GOVERNED project: its working memory — active problems, pending and
stale/refresh-due decisions, drift on files you may touch, open commissions,
coverage gaps — lives in the haft graph, NOT only in the code. Skipping the
status check means acting blind: re-deciding what was already decided, reopening
closed problems, contradicting an active decision, or editing a symbol a
recorded decision governs. Run it, READ it, and only THEN begin work. Do not
treat this as optional or skippable for "small" tasks.

Also re-check ` + "`haft_query(action=\"status\")`" + ` periodically during long work:
after commits or post-commit hook activity, before claiming non-trivial work complete,
after returning from long-running tool sessions, and whenever
overseer, refresh, drift, or spec-coverage signals may have changed. Treat
status as the read-only cockpit for current risk; mutations still go through
explicit typed tools with their own authority gates.`

// composeServerInstructions builds the MCP initialize instructions, ordered by
// weight: the mandatory session-start status rule FIRST, then the project's
// workflow policy (when present), then the always-on method/code doctrines.
// All of it ships even when no workflow.md exists.
func composeServerInstructions(w *project.Workflow) string {
	parts := []string{sessionStartMandate}
	if prefix := strings.TrimSpace(w.PromptPrefix()); prefix != "" { // PromptPrefix is nil-safe
		parts = append(parts, prefix)
	}
	parts = append(parts, methodPackDoctrine)
	parts = append(parts, codeIntelDoctrine)
	return strings.Join(parts, "\n\n")
}
