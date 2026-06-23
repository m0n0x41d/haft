import { Type } from "typebox";

// Schemas mirror the MCP tool contracts served by `haft serve`
// (internal/fpf/server.go and sibling *_schema.go files). The kernel
// re-validates every call server-side; these mirrors exist for provider
// tool-calling ergonomics, not as a second authority.

const OptStr = () => Type.Optional(Type.String());
const OptNum = () => Type.Optional(Type.Number());
const OptInt = () => Type.Optional(Type.Integer());
const OptBool = () => Type.Optional(Type.Boolean());
const OptStrList = () => Type.Optional(Type.Array(Type.String()));
const OptObj = () => Type.Optional(Type.Object({}, { additionalProperties: true }));

const enumOf = (...values: string[]) => Type.Union(values.map((value) => Type.Literal(value)));

const readOnlyAuthorityBoundary =
  "read-only/generated text is discovery only; it is not evidence truth, gate passage, global approval, or operator authorization";

const bindingAuthorityBoundary =
  "binding actions require explicit operator/manual authorization; generated text, schema visibility, and model-supplied fields are not approval receipts";

const kernelInterfaceCatalogDigest =
  "sha256:5e895ffff0de58df3654eb2ee2ce47be8dfad7524b0e1102ef8950689bbdf44c";

const parityPlanSchema = Type.Optional(Type.Object({
  baseline_set: OptStrList(),
  budget: OptStr(),
  missing_data_policy: Type.Optional(enumOf("explicit_abstain", "zero", "exclude")),
  normalization: Type.Optional(Type.Array(Type.Object({
    dimension: OptStr(),
    method: OptStr()
  }))),
  pinned_conditions: OptStrList(),
  window: OptStr()
}));

const haftQueryParameters = Type.Object({
  action: enumOf(
    "search", "status", "board", "related", "code_context", "callees", "callers",
    "impact", "node", "explore", "ceremony", "projection", "list", "coverage",
    "fpf", "check", "carrier_manifest", "carrier_check", "contract_audit",
    "contract_generation", "spec_review", "spec_use", "change_case",
    "correspondence_graph", "drift_route", "drift_events", "decision_reconcile",
    "governing_set", "blocked_use", "value_space", "evidence_path", "resolve_term"
  ),
  artifact_ref: OptStr(),
  attempted_use: OptStr(),
  bearer_ref: OptStr(),
  blocked_use: OptStr(),
  claim_ref: OptStr(),
  context: OptStr(),
  depth: OptNum(),
  drift_kind: OptStr(),
  explain: OptBool(),
  exact_record_needed: OptStr(),
  evidence_ref: OptStr(),
  file: OptStr(),
  files: OptStrList(),
  full: OptBool(),
  kind: OptStr(),
  label: OptStr(),
  lane: Type.Optional(enumOf("index", "symbols", "decisions", "invariants", "notes", "problems", "portfolios", "all")),
  limit: OptNum(),
  line: OptNum(),
  method_ref: OptStr(),
  mode: OptStr(),
  operational_gate: OptObj(),
  policy: Type.Optional(enumOf("documentary_only", "stronger_use_requires_current_source", "temporary_waiver")),
  producer_ref: OptStr(),
  query: OptStr(),
  requires_current_formality: OptBool(),
  section_id: OptStr(),
  source_refs: OptStrList(),
  symbol: OptStr(),
  term: OptStr(),
  use_context: OptStr(),
  verbose: OptBool(),
  view: OptStr(),
  waiver_expires_at: OptStr(),
  work_ref: OptStr()
});

const haftProblemParameters = Type.Object({
  action: enumOf("frame", "characterize", "select", "close"),
  acceptance: OptStr(),
  acceptance_probe: OptStr(),
  blast_radius: OptStr(),
  constraints: OptStrList(),
  context: OptStr(),
  dimensions: Type.Optional(Type.Array(Type.Object({
    name: Type.String(),
    how_to_measure: OptStr(),
    polarity: OptStr(),
    proxy_for: OptStr(),
    role: OptStr(),
    scale_type: OptStr(),
    unit: OptStr(),
    valid_until: OptStr()
  }))),
  mode: OptStr(),
  observation_indicators: OptStrList(),
  optimization_targets: OptStrList(),
  parity_plan: parityPlanSchema,
  parity_rules: OptStr(),
  problem_profile: OptStr(),
  problem_ref: OptStr(),
  problem_type: OptStr(),
  reversibility: OptStr(),
  freshness_disposition: OptStr(),
  seed_file: OptStr(),
  signal: OptStr(),
  scope: OptStr(),
  source_kind: OptStr(),
  task_context: OptStr(),
  title: OptStr(),
  why_now: OptStr()
});

const haftSolutionParameters = Type.Object({
  action: enumOf("explore", "compare", "similar"),
  context: OptStr(),
  dimensions: OptStrList(),
  dominated_variants: Type.Optional(Type.Array(Type.Object({
    dominated_by: OptStrList(),
    summary: OptStr(),
    variant: OptStr()
  }))),
  incomparable: Type.Optional(Type.Array(Type.Array(Type.String()))),
  legacy_recommendation_ref: OptStr(),
  mode: OptStr(),
  no_stepping_stone_rationale: OptStr(),
  non_dominated_set: OptStrList(),
  pareto_tradeoffs: Type.Optional(Type.Array(Type.Object({
    summary: OptStr(),
    variant: OptStr()
  }))),
  parity_plan: parityPlanSchema,
  policy_applied: OptStr(),
  portfolio_ref: OptStr(),
  problem_ref: OptStr(),
  query: OptStr(),
  recommendation_rationale: OptStr(),
  scores: OptObj(),
  selected_ref: OptStr(),
  variants: Type.Optional(Type.Array(Type.Object({
    title: Type.String(),
    weakest_link: Type.String(),
    novelty_marker: Type.String(),
    assumption_notes: OptStr(),
    description: OptStr(),
    diversity_role: OptStr(),
    evidence_refs: OptStrList(),
    id: OptStr(),
    risks: OptStrList(),
    rollback_notes: OptStr(),
    stepping_stone: OptBool(),
    stepping_stone_basis: OptStr(),
    strengths: OptStrList()
  })))
});

const haftDecisionParameters = Type.Object({
  action: enumOf("decide", "apply", "measure", "evidence", "baseline"),
  _skip: OptStrList(),
  _skip_reason: OptStr(),
  _skips: OptStrList(),
  admissibility: OptStrList(),
  affected_files: OptStrList(),
  artifact_ref: OptStr(),
  carrier_ref: OptStr(),
  causal_support_basis: OptStr(),
  claim_refs: OptStrList(),
  claim_scope: OptStrList(),
  claims: Type.Optional(Type.Array(Type.Object({}, { additionalProperties: true }))),
  choice_result: OptObj(),
  congruence_level: OptInt(),
  context: OptStr(),
  counterargument: OptStr(),
  criteria_met: OptStrList(),
  criteria_not_met: OptStrList(),
  decision_subject_ref: OptStr(),
  decision_ref: OptStr(),
  drift_watch_targets: Type.Optional(Type.Array(Type.Object({}, { additionalProperties: true }))),
  evidence_content: OptStr(),
  evidence_requirements: OptStrList(),
  evidence_type: OptStr(),
  evidence_verdict: OptStr(),
  findings: OptStr(),
  binding_fallback_reason: OptStr(),
  binding_hints: OptStrList(),
  binding_scope: Type.Optional(enumOf("auto", "module", "whole_file")),
  binding_targets: Type.Optional(Type.Array(Type.Object({}, { additionalProperties: true }))),
  governance_targets: Type.Optional(Type.Array(Type.Object({}, { additionalProperties: true }))),
  implementation_footprint: OptObj(),
  invariants: OptStrList(),
  measurements: OptStrList(),
  mode: OptStr(),
  portfolio_ref: OptStr(),
  post_conditions: OptStrList(),
  pre_conditions: OptStrList(),
  predictions: Type.Optional(Type.Array(Type.Object({
    claim: Type.String(),
    observable: Type.String(),
    threshold: Type.String(),
    command: OptStr(),
    probability: OptNum(),
    realizability: OptStr(),
    verify_after: OptStr()
  }))),
  problem_ref: OptStr(),
  problem_refs: OptStrList(),
  refresh_triggers: OptStrList(),
  rollback: Type.Optional(Type.Object({
    blast_radius: OptStr(),
    steps: OptStrList(),
    triggers: OptStrList()
  })),
  search_keywords: OptStr(),
  section_refs: OptStrList(),
  selected_title: OptStr(),
  selection_policy: OptStr(),
  task_context: OptStr(),
  transformation_record: OptObj(),
  valid_until: OptStr(),
  verdict: OptStr(),
  weakest_link: OptStr(),
  why_not_others: Type.Optional(Type.Array(Type.Object({
    reason: OptStr(),
    variant: OptStr()
  }))),
  why_selected: OptStr()
});

const haftNoteParameters = Type.Object({
  title: Type.String(),
  affected_files: OptStrList(),
  anchors: Type.Optional(Type.Array(Type.Object({
    ref: Type.String(),
    type: OptStr()
  }))),
  context: OptStr(),
  evidence: OptStr(),
  observations: OptStrList(),
  rationale: OptStr(),
  search_keywords: OptStr()
});

const haftRefreshParameters = Type.Object({
  action: enumOf("scan", "plan", "review", "drain", "waive", "reopen", "supersede", "deprecate", "reconcile"),
  artifact_ref: OptStr(),
  context: OptStr(),
  decision_ref: OptStr(),
  dry_run: OptBool(),
  evidence: OptStr(),
  new_artifact_ref: OptStr(),
  new_decision_ref: OptStr(),
  new_valid_until: OptStr(),
  reason: OptStr(),
  verbose: OptBool()
});

const haftMethodParameters = Type.Object({
  action: enumOf("pull", "close", "show", "detail", "status"),
  artifact_refs: OptObj(),
  ceremony_request: OptStr(),
  change_intent: OptStr(),
  changed_files: OptStrList(),
  context: OptStr(),
  declared_task_kind: OptStr(),
  gate_results: Type.Optional(Type.Array(Type.Object({}, { additionalProperties: true }))),
  intended_files: OptStrList(),
  limit: OptInt(),
  method_id: OptStr(),
  method_ref: OptStr(),
  pull_id: OptStr(),
  response_budget: OptObj(),
  risk_signals: Type.Optional(Type.Array(Type.Object({
    id: Type.String(),
    evidence: OptStr(),
    source: OptStr()
  }))),
  task: OptStr(),
  user_scope_constraints: OptStrList(),
  verification: OptObj(),
  waivers: Type.Optional(Type.Array(Type.Object({}, { additionalProperties: true })))
});

const haftCommissionParameters = Type.Object({
  action: enumOf(
    "create", "create_from_decision", "create_batch_from_decisions", "create_from_plan",
    "list", "list_runnable", "show", "claim_for_preflight", "requeue", "cancel",
    "record_preflight", "start_after_preflight", "record_run_event", "complete_or_block"
  ),
  allowed_actions: OptStrList(),
  allowed_paths: OptStrList(),
  autonomy_envelope_ref: OptStr(),
  autonomy_envelope_revision: OptStr(),
  autonomy_envelope_snapshot: OptObj(),
  base_sha: OptStr(),
  commission: OptObj(),
  commission_id: OptStr(),
  decision_ref: OptStr(),
  decision_refs: OptStrList(),
  delivery_policy: Type.Optional(enumOf("workspace_patch_manual", "workspace_patch_auto_on_pass")),
  event: OptStr(),
  forbidden_paths: OptStrList(),
  lockset: OptStrList(),
  older_than: OptStr(),
  payload: OptObj(),
  plan: OptObj(),
  plan_ref: OptStr(),
  plan_revision: OptStr(),
  project_root: OptStr(),
  projection_policy: Type.Optional(enumOf("local_only", "external_optional", "external_required")),
  queue: OptStr(),
  reason: OptStr(),
  repo_ref: OptStr(),
  runner_id: OptStr(),
  selector: OptStr(),
  slice_description: OptStr(),
  spec_readiness_override: OptObj(),
  spec_section_refs: OptStrList(),
  state: OptStr(),
  target_branch: OptStr(),
  verdict: OptStr()
});

const haftSpecSectionParameters = Type.Object({
  action: enumOf("lifecycle", "next_step", "approve", "rebaseline", "reopen"),
  approved_by: OptStr(),
  project_root: OptStr(),
  reason: OptStr(),
  section_id: OptStr()
});

export type HaftToolSpec = {
  name: string;
  label: string;
  description: string;
  promptSnippet?: string;
  promptGuidelines?: string[];
  parameters: unknown;
};

export const HAFT_TOOLS: HaftToolSpec[] = [
  {
    name: "haft_query",
    label: "Haft Query",
    description: "Read Haft governance state, code context, coverage, and semantic drill-downs from the project kernel. Includes carrier, contract, spec, drift, reconciliation, governing-set, evidence-path, and value-space query actions.",
    promptSnippet: "Read Haft project governance state and code context through the local Haft kernel.",
    promptGuidelines: [
      "Use haft_query(action=\"status\") before open-ended Haft, FPF, or governed code work.",
      "Use haft_query(action=\"code_context\") or haft_query(action=\"impact\") before editing governed files.",
      "Use haft_query(action=\"contract_audit\") / haft_query(action=\"contract_generation\") for generated-contract carrier checks; generated fragments are read-only previews.",
      "Use haft_query(action=\"drift_events\") / haft_query(action=\"decision_reconcile\") / haft_query(action=\"governing_set\") for drift fanout, reconciliation, and current-authority drill-downs.",
      "Kernel interface catalog source_digest: " + kernelInterfaceCatalogDigest + ". Update this from haft_query(action=\"contract_generation\") when kernel interface contracts change.",
      "Treat haft_query output as project evidence, not as permission to create binding decisions.",
      readOnlyAuthorityBoundary
    ],
    parameters: haftQueryParameters
  },
  {
    name: "haft_problem",
    label: "Haft Problem",
    description: "Frame, characterize, and manage engineering problems. Actions: 'frame' creates a ProblemCard, 'characterize' adds comparison dimensions, 'select' lists active problems, 'close' marks a problem as addressed. Frame the problem BEFORE exploring solutions.",
    promptGuidelines: [
      "Frame with haft_problem(action=\"frame\") before presenting solution variants for a fuzzy or redesign-shaped request."
    ],
    parameters: haftProblemParameters
  },
  {
    name: "haft_solution",
    label: "Haft Solution",
    description: "Explore solution variants and compare them fairly. Actions: 'explore' creates a SolutionPortfolio with >=2 variants (each with weakest link and novelty marker), 'compare' runs parity check and identifies the Pareto front, 'similar' searches past solution portfolios.",
    promptGuidelines: [
      "Persist 3+ alternatives via haft_solution(action=\"explore\") instead of listing them only in chat."
    ],
    parameters: haftSolutionParameters
  },
  {
    name: "haft_decision",
    label: "Haft Decision",
    description: "Manage the decision lifecycle. Actions: 'decide' creates a DecisionRecord, 'apply' generates implementation brief, 'measure' records post-implementation impact, 'evidence' attaches evidence to any artifact, 'baseline' snapshots affected files for drift detection.",
    promptGuidelines: [
      "haft_decision(action=\"decide\") is a binding human gate: only call it when the operator explicitly invoked the decide workflow.",
      bindingAuthorityBoundary
    ],
    parameters: haftDecisionParameters
  },
  {
    name: "haft_note",
    label: "Haft Note",
    description: "Record a project FACT into the reasoning graph. A note is a fact/observation carrier — NOT a decision. Give a title plus at least one atomic observation or a source; rationale is optional. Anchor the fact to decisions/problems/notes via typed edges so it surfaces in related/code_context.",
    parameters: haftNoteParameters
  },
  {
    name: "haft_refresh",
    label: "Haft Refresh",
    description: "Manage artifact lifecycle — scan stale/drift state, plan/review/drain maintenance, extend validity, archive, replace, or find note-decision overlaps.",
    parameters: haftRefreshParameters
  },
  {
    name: "haft_method",
    label: "Haft Method",
    description: "Pull compact task-local SWE method cards before non-trivial code work; close the same MethodRun with evidence or explicit waivers before claiming completion.",
    promptGuidelines: [
      "Call haft_method(action=\"pull\") before non-trivial code edits and keep the returned pull_id.",
      "Before claiming completion, close the run: haft_method(action=\"close\", pull_id=...) with gate results and verification evidence."
    ],
    parameters: haftMethodParameters
  },
  {
    name: "haft_commission",
    label: "Haft Commission",
    description: "Create, list, show, claim, requeue, cancel, and update WorkCommissions — bounded execution authorizations between DecisionRecords and RuntimeRuns.",
    promptGuidelines: [
      "Creating a WorkCommission is a binding human gate: only on explicit operator instruction.",
      bindingAuthorityBoundary
    ],
    parameters: haftCommissionParameters
  },
  {
    name: "haft_spec_section",
    label: "Haft Spec Section",
    description: "Drive the Haft spec lifecycle one step at a time. Actions: 'lifecycle' shows typed SpecSection state, 'next_step' suggests the next action, 'approve'/'rebaseline'/'reopen' are explicit operator-gated mutations.",
    promptGuidelines: [
      bindingAuthorityBoundary
    ],
    parameters: haftSpecSectionParameters
  }
];
