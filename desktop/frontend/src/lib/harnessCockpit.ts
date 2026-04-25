import type {
  HarnessEvidenceRequirement,
  HarnessPhaseOutcome,
  HarnessRunResult,
  HarnessTailResult,
  WorkCommission,
} from "./api.ts";

export type HarnessCommissionKind =
  | "runnable"
  | "running"
  | "blocked"
  | "completed"
  | "failed";

export type HarnessTone = "neutral" | "accent" | "success" | "warning" | "danger";

export type HarnessActionKind =
  | "result"
  | "tail"
  | "apply"
  | "cancel"
  | "requeue";

export interface HarnessCommissionStateView {
  kind: HarnessCommissionKind;
  raw: string;
  label: string;
  tone: HarnessTone;
  terminal: boolean;
}

export interface HarnessActionView {
  kind: HarnessActionKind;
  label: string;
  enabled: boolean;
  reason: string;
}

export interface HarnessWorkspaceView {
  path: string;
  diffState: string;
  changedFiles: string[];
  gitStatus: string[];
  diffStat: string[];
  error: string;
  canApply: boolean;
}

export interface HarnessRuntimeView {
  active: boolean;
  phase: string;
  subState: string;
  sessionID: string;
  taskPID: string;
  workspacePath: string;
  statusUpdatedAt: string;
  lastEvent: string;
  lastEventAt: string;
  lastTurnStatus: string;
  lastTurnID: string;
  preview: string;
}

export interface HarnessEvidenceView {
  requiredCount: number;
  requirements: string[];
  latestMeasure: string;
  terminal: string;
}

export interface HarnessTailView {
  lines: string[];
  hasEvents: boolean;
  followCommand: string;
}

export interface HarnessCockpitDetail {
  id: string;
  decisionRef: string;
  problemRef: string;
  state: HarnessCommissionStateView;
  actions: HarnessActionView[];
  workspace: HarnessWorkspaceView;
  runtime: HarnessRuntimeView;
  evidence: HarnessEvidenceView;
  tail: HarnessTailView;
}

type StateRule = {
  kind: HarnessCommissionKind;
  label: string;
  tone: HarnessTone;
  terminal: boolean;
};

const STATE_RULES: Record<string, StateRule> = {
  draft: {
    kind: "blocked",
    label: "Draft",
    tone: "neutral",
    terminal: false,
  },
  queued: {
    kind: "runnable",
    label: "Runnable",
    tone: "neutral",
    terminal: false,
  },
  ready: {
    kind: "runnable",
    label: "Runnable",
    tone: "accent",
    terminal: false,
  },
  preflighting: {
    kind: "running",
    label: "Preflighting",
    tone: "accent",
    terminal: false,
  },
  running: {
    kind: "running",
    label: "Running",
    tone: "accent",
    terminal: false,
  },
  blocked_stale: {
    kind: "blocked",
    label: "Blocked stale",
    tone: "warning",
    terminal: false,
  },
  blocked_policy: {
    kind: "blocked",
    label: "Blocked policy",
    tone: "warning",
    terminal: false,
  },
  blocked_conflict: {
    kind: "blocked",
    label: "Blocked conflict",
    tone: "warning",
    terminal: false,
  },
  needs_human_review: {
    kind: "blocked",
    label: "Needs review",
    tone: "warning",
    terminal: false,
  },
  completed: {
    kind: "completed",
    label: "Completed",
    tone: "success",
    terminal: true,
  },
  completed_with_projection_debt: {
    kind: "completed",
    label: "Projection debt",
    tone: "warning",
    terminal: true,
  },
  failed: {
    kind: "failed",
    label: "Failed",
    tone: "danger",
    terminal: false,
  },
  cancelled: {
    kind: "blocked",
    label: "Cancelled",
    tone: "danger",
    terminal: true,
  },
  expired: {
    kind: "blocked",
    label: "Expired",
    tone: "danger",
    terminal: true,
  },
};

const UNKNOWN_STATE_RULE: StateRule = {
  kind: "blocked",
  label: "Unknown",
  tone: "neutral",
  terminal: false,
};

const ACTION_ORDER: HarnessActionKind[] = [
  "result",
  "tail",
  "apply",
  "cancel",
  "requeue",
];

const ACTION_LABELS: Record<HarnessActionKind, string> = {
  result: "Result",
  tail: "Tail",
  apply: "Apply",
  cancel: "Cancel",
  requeue: "Requeue",
};

const REQUEUEABLE_STATES = new Set([
  "queued",
  "ready",
  "preflighting",
  "running",
  "blocked_stale",
  "blocked_policy",
  "blocked_conflict",
  "needs_human_review",
  "failed",
]);

export function buildHarnessCockpitDetail(
  commission: WorkCommission,
  result: HarnessRunResult | null,
  tail: HarnessTailResult | null,
): HarnessCockpitDetail {
  const state = normalizeHarnessCommissionState(commission);
  const actions = harnessCommissionActions(commission, result);
  const workspace = normalizeWorkspaceFacts(result);
  const runtime = normalizeRuntimeFacts(result);
  const evidence = normalizeEvidenceFacts(result);
  const tailView = normalizeTailFacts(commission.id, tail);

  return {
    id: commission.id,
    decisionRef: commission.decision_ref || "decision unknown",
    problemRef: commission.problem_card_ref || "problem unknown",
    state,
    actions,
    workspace,
    runtime,
    evidence,
    tail: tailView,
  };
}

export function normalizeHarnessCommissionState(
  commission: WorkCommission,
): HarnessCommissionStateView {
  const raw = normalizeState(commission.state);
  const rule = STATE_RULES[raw] ?? UNKNOWN_STATE_RULE;
  const terminal = Boolean(commission.operator?.terminal) || rule.terminal;

  return {
    ...rule,
    raw,
    terminal,
  };
}

export function harnessCommissionActions(
  commission: WorkCommission,
  result: HarnessRunResult | null,
): HarnessActionView[] {
  const state = normalizeHarnessCommissionState(commission);
  const context = {
    commission,
    result,
    state,
  };

  return ACTION_ORDER.map((kind) => actionView(kind, context));
}

function actionView(
  kind: HarnessActionKind,
  context: {
    commission: WorkCommission;
    result: HarnessRunResult | null;
    state: HarnessCommissionStateView;
  },
): HarnessActionView {
  const availability = ACTION_AVAILABILITY[kind](context);

  return {
    kind,
    label: ACTION_LABELS[kind],
    enabled: availability.enabled,
    reason: availability.reason,
  };
}

const ACTION_AVAILABILITY: Record<
  HarnessActionKind,
  (context: {
    commission: WorkCommission;
    result: HarnessRunResult | null;
    state: HarnessCommissionStateView;
  }) => { enabled: boolean; reason: string }
> = {
  result: ({ commission }) => ({
    enabled: commission.id.trim() !== "",
    reason: "inspect structured harness result",
  }),
  tail: ({ commission }) => ({
    enabled: commission.id.trim() !== "",
    reason: "inspect humanized runtime events",
  }),
  apply: ({ result }) => ({
    enabled: Boolean(result?.can_apply),
    reason: result?.can_apply
      ? "completed workspace has unapplied changes"
      : "no completed workspace diff is ready to apply",
  }),
  cancel: ({ state }) => ({
    enabled: !state.terminal,
    reason: state.terminal
      ? "terminal commissions cannot be cancelled"
      : "cancel unfinished commission without deleting audit history",
  }),
  requeue: ({ commission, state }) => {
    const raw = state.raw;
    const expired = Boolean(commission.operator?.expired);
    const enabled = REQUEUEABLE_STATES.has(raw) && !expired && !state.terminal;

    return {
      enabled,
      reason: enabled
        ? "return recoverable commission to the runnable queue"
        : "commission is not requeueable in this state",
    };
  },
};

function normalizeWorkspaceFacts(result: HarnessRunResult | null): HarnessWorkspaceView {
  const facts = result?.workspace_facts;

  return {
    path: facts?.path || result?.workspace || "",
    diffState: facts?.diff_state || "unknown",
    changedFiles: facts?.changed_files ?? result?.changed_files ?? [],
    gitStatus: facts?.git_status ?? [],
    diffStat: facts?.diff_stat ?? [],
    error: facts?.error ?? "",
    canApply: Boolean(result?.can_apply),
  };
}

function normalizeRuntimeFacts(result: HarnessRunResult | null): HarnessRuntimeView {
  const facts = result?.runtime_facts;

  return {
    active: Boolean(facts?.active),
    phase: facts?.phase ?? "",
    subState: facts?.sub_state ?? "",
    sessionID: facts?.session_id ?? "",
    taskPID: facts?.task_pid ?? "",
    workspacePath: facts?.workspace_path ?? "",
    statusUpdatedAt: facts?.status_updated_at ?? "",
    lastEvent: facts?.last_event ?? "",
    lastEventAt: facts?.last_event_at ?? "",
    lastTurnStatus: facts?.last_turn_status ?? "",
    lastTurnID: facts?.last_turn_id ?? "",
    preview: facts?.preview ?? "",
  };
}

function normalizeEvidenceFacts(result: HarnessRunResult | null): HarnessEvidenceView {
  const facts = result?.evidence_facts;
  const requirements = facts?.requirements ?? [];

  return {
    requiredCount: facts?.required_count ?? requirements.length,
    requirements: requirements.map(formatEvidenceRequirement),
    latestMeasure: formatPhaseOutcome(facts?.latest_measure),
    terminal: formatPhaseOutcome(facts?.terminal),
  };
}

function normalizeTailFacts(
  commissionID: string,
  tail: HarnessTailResult | null,
): HarnessTailView {
  return {
    lines: tail?.lines ?? [],
    hasEvents: Boolean(tail?.has_events),
    followCommand: tail?.follow_command || `haft harness tail ${commissionID} --follow`,
  };
}

function formatEvidenceRequirement(requirement: HarnessEvidenceRequirement): string {
  const fields = [
    keyValue("kind", requirement.kind),
    keyValue("command", requirement.command),
    keyValue("description", requirement.description),
    keyValue("claim", requirement.claim_ref),
    keyValue("section", requirement.section_ref),
  ];

  return fields
    .filter((field) => field !== "")
    .join(" ");
}

function formatPhaseOutcome(outcome: HarnessPhaseOutcome | null | undefined): string {
  if (!outcome) {
    return "";
  }

  const fields = [
    keyValue("phase", outcome.phase),
    keyValue("verdict", outcome.verdict),
    keyValue("next", outcome.next),
    keyValue("at", outcome.at),
  ];

  return fields
    .filter((field) => field !== "")
    .join(" ");
}

function keyValue(key: string, value: string | undefined): string {
  const clean = value?.trim() ?? "";
  if (clean === "") {
    return "";
  }

  return `${key}=${clean}`;
}

function normalizeState(state: string | undefined): string {
  return (state ?? "").trim().toLowerCase();
}
