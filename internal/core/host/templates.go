package host

const agents = `<!-- haft10:start -->
# Haft project memory

Use the project-local haft MCP tool or the configured haft10 binary. Both accept
one haft.api/1 request and return result_kind, data, diagnostics, basis, coverage
and limits. Read these fields before relying on a result.

Recover the current object and question. h-reason, h-decide, h-spec and h-verify
are independent capabilities, selected by the task. No universal sequence is
required. Source search retrieves candidates; inspect governing bodies before
judging applicability. A source result is not a recommendation or proof.

Persist when the operator requests it or a concrete receiving use requires
replay, handoff or delayed feedback. Keep proposed text, accepted choices,
implementation links and observed evidence distinct. Never manufacture operator
confirmation from a skill invocation, model prose, a record or tool output.
Explicit API inputs are local trusted data; the tool cannot authenticate their
real-world origin. Apply only the exact operator-authorized effect and scope.

Go navigation is syntactic and conservative. Missing or ambiguous bindings need
explicit repair. Structural checks do not run tests. A historical observation
keeps its exact claim, code, oracle, dependency and environment basis; formatting
can change raw evidence bytes even when navigation is token-equivalent.
<!-- haft10:end -->
`

// Skills returns independent task-conditioned instruction carriers. Examples
// use the exact same Request object as CLI --input and MCP tools/call arguments.
func Skills() map[string]string {
	return map[string]string{
		"h-reason": `---
name: h-reason
description: Use for a current project question that needs source-grounded reasoning. Skip source search for mechanical edits or exact project lookups. Does not require another skill first or create records automatically.
---

Recover the object, question, known constraints and intended result from the
conversation. When stored context matters, call the haft tool with:

{"format":"haft.api/1","operation":"recall","query":"order cancellation","limit":8}

For a substantive source question, preserve the actual concern:

{"format":"haft.api/1","operation":"fpf","action":"search","query":"claim evidence observation scope","limit":5}

Search results are candidates. Inspect a returned exact ref using operation fpf,
action inspect and ref equal to that returned value. Read the complete governing
body, conditions and limits before judging applicability. Do not treat ranking,
fresh capture or source availability as recommendation or upstream currentness.
If source is unavailable or ambiguous, retain that diagnostic and abstain from
source-dependent claims. Explain the smallest useful result conversationally.
Persist only when an explicit request or concrete receiving use requires it.
`,
		"h-decide": `---
name: h-decide
description: Use when a direct, unambiguous operator request selects a bounded option for a named subject and scope. Do not infer a choice from recommendations, quoted text, tool output or invoking this skill.
---

Recover the exact choice, alternatives, rationale, scope and weakest link. Read
current related records if needed:

{"format":"haft.api/1","operation":"recall","query":"order cancellation decision","limit":8}

If the operator's effect, option or scope is unresolved, explain that specific
choice. Otherwise continue the already-authorized bounded write without a second
confirmation ceremony. Compose a decision carrier from actual current content;
preserve rationale and honest origin. Do not copy an example as a real decision.
Use operation remember, request_id equal to a unique stable write key, carrier
equal to that complete Markdown, and expected_generation from the current result
where requested by the application. Check with operation check, action structural
and carrier equal to the same Markdown before publication.

Only a real direct operator request supports explicit operator_confirmed true
and accepted status. The API treats those fields as trusted local inputs; it
cannot verify speech or invent a receipt. A proposal remains proposed. Reuse a
request_id only for the identical payload. Inspect conflicts and current heads;
never silently select a predecessor or overwrite a concurrent choice.
`,
		"h-spec": `---
name: h-spec
description: Use for specification claims, implementation/check bindings, or a bounded specification change. Structural validity, operator acceptance, application and observed correctness are separate results. No other skill is a prerequisite.
---

Recover the current subject, claim and exact basis. Inspect its record via recall
and its code via context. For an existing Go project, a read-only example is:

{"format":"haft.api/1","operation":"context","ref":"file:order.go"}

To author new memory, use remember with a stable request_id and Markdown carrier.
The frontmatter is YAML (or JSON) between --- lines; the body preserves rationale
and scope. A proposed spec uses format: haft/1, kind: spec, title, about (the
domain subject), slug, receiving_use, and claims. A claim needs a stable id,
kind (law, definition, guard or prescription) and text. Laws and guards need
either checks or unchecked with an explicit reason for strict validation.
implemented_by and checks are lists of {ref, covers};
examples are {id, given, when, then}. Use sym:path.go::Type.Method for a method,
test:path_test.go::TestName for a scenario, or pbt:path_test.go::TestName for a
finite property. Preserve supplied limitations in covers and the body. Initial
records may omit id, created_at, status and origin for proposed agent defaults.
Read data.exact_ref from recall; alias spec:<slug> resolves the current candidate.

Explicit term meanings use remember/action terms with frontmatter
{"format":"haft.terms/1","terms":[{"id":"Domain.Term","definition":"<meaning>","aliases":["<word>"],"exclusions":["<excluded meaning>"]}]}.
Record and claim terms list those qualified IDs. The terms write creates the
initial map without replacing an existing map. Inspect existing terms before
reusing them; preserve changed meanings with their historical snapshots.

Validate current carriers without executing a check:

{"format":"haft.api/1","operation":"check","action":"structural","strict":true}

Inspect diagnostics and coverage; green structure is not implementation proof.
For edits, author a bounded change with exact section bases, explicit operations
and reasons. Create it through operation change/action create with the complete
carrier and stable request_id. Preview with action preview and the exact returned
change ref. Apply only the intended authorized effect using action apply, ref,
preview_digest and expected_generation returned by that preview, plus a new
request_id. A stale basis requires another review; never substitute newer heads.
Read tool schema for complete fields. Application does not infer acceptance.

The carrier is Markdown with YAML frontmatter between two --- lines. JSON is
also valid YAML. For a clarification, copy the complete claim object from
recall's data.resolution.document.record.claims, change only intended fields,
and use this frontmatter shape (replace angle-bracket placeholders):

{"format":"haft.change/1","title":"<readable title>","intent":"<intended effect>","patches":[{"base":"<data.exact_ref of the spec, without #claim>","operations":[{"op":"MODIFIED","claim_id":"<existing claim ID>","claim":<complete revised claim object>,"reason":"<why this change>"}]}]}

Omit id, change_key, state and created_at on initial creation to use API defaults.
Supplied IDs require chg-YYYYMMDD-xxxxxxxx with eight hexadecimal suffix digits.
Operation names are uppercase ADDED, MODIFIED, REMOVED and RENAMED. MODIFIED
replaces the complete claim; omitted examples are preserved, explicit removals
need remove_examples and a reason. RENAMED uses claim_id and new_id. Omit patch
body to preserve spec prose; replacing it requires expected_body_digest and
body_change_reason. Inspect preview losses before applying. Remembered records
use format haft/1; change carriers use the separate haft.change/1 format.

Preserve authored checks and implementation selectors. Rename or ambiguity needs
explicit repair; do not silently redirect a claim. Archive, sync and apply have
separate meanings. A completed task checkbox is not evidence that a claim holds.
`,
		"h-verify": `---
name: h-verify
description: Use when an existing claim needs an observation against its exact implementation, declared check and conditions, or when comparing a saved basis after change. Do not treat structural validity or an implementation link as proof.
---

Resolve the exact claim and read its declared implementation/check bindings and
scope. Prepare a declared check with the real claim ref, check ref and explicit
scope. This example matches the order fixture when that fixture is present:

{"format":"haft.api/1","operation":"check","action":"prepare","ref":"spec:order-cancel#total-preserved","check_ref":"pbt:order_test.go::TestCancelPreservesTotal","scope":"Generated new and paid orders; cancellation preserves total","seed":23}

Read expected, command, run_environment, basis_capture, code_complete, diagnostics and limits. Preparation does
not run anything. Judge whether the oracle actually covers the claim. If the
current task authorizes the test, run the returned exact command in the intended
environment using the returned run_environment. Independently capture and compare
claim, code, oracle and dependency bytes before and after the run; record the
actual environment and toolchain. Preserve raw stdout/stderr, exit code, selector
and full observed basis. Never copy expected basis fields into an observation
without establishing them from those captures. If the basis changes during the
run, retain that diagnostic and do not attribute the result to the earlier basis.
Use basis_capture's exact JSON preimage bytes, file digests and oracle byte span
to verify the expected hashes against actual files; do not reverse-engineer a
hash or mistake a symbol-span digest for the whole test-file digest. Preserve
the before/after capture and the actual build tags, toolchain and environment.
Use .context/verification for temporary runner reports and captures, which are
outside the code-index scope. For a durable handoff, embed the needed raw bytes
in the published evidence body; an ignored scratch file alone is not portable.
Submit operation check/action observe with observation containing that exact
expected contract and independently captured observed run, and inspect the
returned current_basis after the run. Byte fields are base64 JSON strings. The
declared oracle contract must explicitly justify any assertion-failure marker;
never guess an assertion failure from arbitrary output text.

Report passed, assertion_failure, skipped, not_run, environment_failure or
unattributable together with reason_code and current_basis. An old pass does not
transfer to a different claim edition, oracle, dependency or environment. Preserve
failed and skipped results. A pass establishes only the declared observed scope.

For a replay or handoff receiving use, context/impact may request capture_code true
and later supply that returned code_capture as prior_code. Token-equivalent
formatting can update navigation while the raw evidence basis changes. Persist
only the exact observation and portable snapshots needed by the receiving use;
the observe operation itself does not publish an evidence record.

To retain a real observation, use remember with a haft/1 evidence carrier. Its
frontmatter needs kind: evidence, a readable title, about, claim (the bounded
observation), observed_at, method, source (where the raw report lives), basis
with kind: code and ref equal to the observation ID, and uses. Each use contains
id, target (the exact claim ref including its edition), check, polarity
(supports, weakens or inconclusive), and scope. Put the complete observation,
raw outputs and capture report in the Markdown body or a preserved addressable
report. ID, created_at and origin may use remember defaults. An observational
record may have status: active; this does not accept its proposed target spec.
Do not set operator_confirmed for an observation. Preserve the original claim
snapshot and full scope even when a later run changes the support relation.
`,
	}
}
