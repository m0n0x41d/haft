import type {
  HarnessApplyDisabledReason,
  HarnessEvidenceRequirement,
  HarnessPhaseOutcome,
  HarnessRunResult,
  HarnessTailResult,
  HarnessWorkspaceAuthorization,
  WorkCommission,
} from "./api.ts";
import {
  isRecoverableWorkCommissionState,
  isTerminalWorkCommissionState,
  normalizeWorkCommissionState,
} from "./workCommissionLifecycle.ts";

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

export type HarnessApplyDisabledKind =
  | "forbidden"
  | "out_of_scope"
  | "unknown_scope"
  | "not_ready";

export type HarnessApplyReadinessView =
  | {
      kind: "ready";
      canApply: true;
      reason: string;
      paths: string[];
    }
  | {
      kind: "disabled";
      canApply: false;
      disabledKind: HarnessApplyDisabledKind;
      reason: string;
      paths: string[];
    };

export interface HarnessWorkspaceView {
  path: string;
  diffState: string;
  changedFiles: string[];
  gitStatus: string[];
  diffStat: string[];
  error: string;
  canApply: boolean;
  applyReadiness: HarnessApplyReadinessView;
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
};

const STATE_RULES: Record<string, StateRule> = {
  draft: {
    kind: "blocked",
    label: "Draft",
    tone: "neutral",
  },
  queued: {
    kind: "runnable",
    label: "Runnable",
    tone: "neutral",
  },
  ready: {
    kind: "runnable",
    label: "Runnable",
    tone: "accent",
  },
  preflighting: {
    kind: "running",
    label: "Preflighting",
    tone: "accent",
  },
  running: {
    kind: "running",
    label: "Running",
    tone: "accent",
  },
  blocked_stale: {
    kind: "blocked",
    label: "Blocked stale",
    tone: "warning",
  },
  blocked_policy: {
    kind: "blocked",
    label: "Blocked policy",
    tone: "warning",
  },
  blocked_conflict: {
    kind: "blocked",
    label: "Blocked conflict",
    tone: "warning",
  },
  needs_human_review: {
    kind: "blocked",
    label: "Needs review",
    tone: "warning",
  },
  completed: {
    kind: "completed",
    label: "Completed",
    tone: "success",
  },
  completed_with_projection_debt: {
    kind: "completed",
    label: "Projection debt",
    tone: "warning",
  },
  failed: {
    kind: "failed",
    label: "Failed",
    tone: "danger",
  },
  cancelled: {
    kind: "blocked",
    label: "Cancelled",
    tone: "danger",
  },
  expired: {
    kind: "blocked",
    label: "Expired",
    tone: "danger",
  },
};

const UNKNOWN_STATE_RULE: StateRule = {
  kind: "blocked",
  label: "Unknown",
  tone: "neutral",
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

const APPLY_READY_REASON = "completed workspace has unapplied changes";
const APPLY_NOT_READY_REASON = "no completed workspace diff is ready to apply";

const APPLY_DISABLED_KIND_BY_VERDICT: Record<string, HarnessApplyDisabledKind> = {
  forbidden: "forbidden",
  out_of_scope: "out_of_scope",
  unknown_scope: "unknown_scope",
};

const APPLY_DISABLED_KIND_BY_CODE: Record<string, HarnessApplyDisabledKind> = {
  forbidden_paths: "forbidden",
  out_of_scope_paths: "out_of_scope",
  unknown_scope_paths: "unknown_scope",
};

const APPLY_DISABLED_REASON_RULES: Record<
  HarnessApplyDisabledKind,
  {
    prefix: string;
    fallback: string;
  }
> = {
  forbidden: {
    prefix: "workspace diff contains paths forbidden by commission scope",
    fallback: "workspace diff contains forbidden paths",
  },
  out_of_scope: {
    prefix: "workspace diff contains paths outside commission scope",
    fallback: "workspace diff contains paths outside commission scope",
  },
  unknown_scope: {
    prefix: "workspace diff cannot be applied because commission scope is unknown for paths",
    fallback: "workspace diff cannot be applied because commission scope is unknown",
  },
  not_ready: {
    prefix: "",
    fallback: APPLY_NOT_READY_REASON,
  },
};

const AUTHORIZATION_PATHS_BY_KIND: Record<
  HarnessApplyDisabledKind,
  (authorization: HarnessWorkspaceAuthorization) => string[]
> = {
  forbidden: (authorization) => authorization.forbidden_paths,
  out_of_scope: (authorization) => authorization.out_of_scope_paths,
  unknown_scope: (authorization) => authorization.unknown_scope_paths,
  not_ready: () => [],
};

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
  const raw = normalizeWorkCommissionState(commission.state);
  const rule = STATE_RULES[raw] ?? UNKNOWN_STATE_RULE;
  const terminal = isTerminalWorkCommissionState(raw);

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
  const applyReadiness = normalizeApplyReadiness(result);
  const context = {
    commission,
    result,
    state,
    applyReadiness,
  };

  return ACTION_ORDER.map((kind) => actionView(kind, context));
}

function actionView(
  kind: HarnessActionKind,
  context: {
    commission: WorkCommission;
    result: HarnessRunResult | null;
    state: HarnessCommissionStateView;
    applyReadiness: HarnessApplyReadinessView;
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
    applyReadiness: HarnessApplyReadinessView;
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
  apply: ({ applyReadiness }) => ({
    enabled: applyReadiness.canApply,
    reason: applyReadiness.reason,
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
    const enabled = isRecoverableWorkCommissionState(raw) && !expired && !state.terminal;

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
  const applyReadiness = normalizeApplyReadiness(result);

  return {
    path: facts?.path || result?.workspace || "",
    diffState: facts?.diff_state || "unknown",
    changedFiles: facts?.changed_files ?? result?.changed_files ?? [],
    gitStatus: facts?.git_status ?? [],
    diffStat: facts?.diff_stat ?? [],
    error: facts?.error ?? "",
    canApply: applyReadiness.canApply,
    applyReadiness,
  };
}

export function normalizeApplyReadiness(
  result: HarnessRunResult | null,
): HarnessApplyReadinessView {
  if (result?.can_apply) {
    return {
      kind: "ready",
      canApply: true,
      reason: APPLY_READY_REASON,
      paths: [],
    };
  }

  const authorizationReadiness = applyReadinessFromAuthorization(
    result?.workspace_facts?.authorization,
  );
  if (authorizationReadiness) {
    return authorizationReadiness;
  }

  const operatorReasonReadiness = applyReadinessFromOperatorReason(
    result?.operator_next?.apply_disabled_reason,
  );
  if (operatorReasonReadiness) {
    return operatorReasonReadiness;
  }

  return disabledApplyReadiness("not_ready", [], "");
}

function applyReadinessFromAuthorization(
  authorization: HarnessWorkspaceAuthorization | null | undefined,
): HarnessApplyReadinessView | null {
  if (!authorization) {
    return null;
  }

  const disabledKind = applyDisabledKindFromVerdict(authorization.verdict);
  if (!disabledKind) {
    return null;
  }

  const paths = AUTHORIZATION_PATHS_BY_KIND[disabledKind](authorization);
  const fallbackReason = authorization.operator_reason?.message ?? "";
  return disabledApplyReadiness(disabledKind, paths, fallbackReason);
}

function applyReadinessFromOperatorReason(
  reason: HarnessApplyDisabledReason | null | undefined,
): HarnessApplyReadinessView | null {
  if (!reason) {
    return null;
  }

  const disabledKind = applyDisabledKindFromReason(reason);
  if (!disabledKind) {
    return null;
  }

  return disabledApplyReadiness(disabledKind, reason.paths, reason.message);
}

function applyDisabledKindFromReason(
  reason: HarnessApplyDisabledReason,
): HarnessApplyDisabledKind | null {
  const codeKind = applyDisabledKindFromCode(reason.code);
  if (codeKind) {
    return codeKind;
  }

  return applyDisabledKindFromVerdict(reason.verdict);
}

function applyDisabledKindFromCode(code: string): HarnessApplyDisabledKind | null {
  const normalizedCode = code.trim();
  return APPLY_DISABLED_KIND_BY_CODE[normalizedCode] ?? null;
}

function applyDisabledKindFromVerdict(
  verdict: string | undefined,
): HarnessApplyDisabledKind | null {
  const normalizedVerdict = verdict?.trim() ?? "";
  return APPLY_DISABLED_KIND_BY_VERDICT[normalizedVerdict] ?? null;
}

function disabledApplyReadiness(
  disabledKind: HarnessApplyDisabledKind,
  paths: string[],
  fallbackReason: string,
): HarnessApplyReadinessView {
  const cleanPaths = paths
    .map((path) => path.trim())
    .filter((path) => path !== "");
  const reason = formatApplyDisabledReason(disabledKind, cleanPaths, fallbackReason);

  return {
    kind: "disabled",
    canApply: false,
    disabledKind,
    reason,
    paths: cleanPaths,
  };
}

function formatApplyDisabledReason(
  disabledKind: HarnessApplyDisabledKind,
  paths: string[],
  fallbackReason: string,
): string {
  const rule = APPLY_DISABLED_REASON_RULES[disabledKind];
  const pathList = paths.join(", ");
  if (pathList !== "" && rule.prefix !== "") {
    return `${rule.prefix}: ${pathList}`;
  }

  const cleanFallbackReason = fallbackReason.trim();
  if (cleanFallbackReason !== "") {
    return cleanFallbackReason;
  }

  return rule.fallback;
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
