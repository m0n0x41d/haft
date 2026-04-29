export type WorkCommissionLifecycleState =
  | "draft"
  | "queued"
  | "ready"
  | "preflighting"
  | "running"
  | "blocked_stale"
  | "blocked_policy"
  | "blocked_conflict"
  | "needs_human_review"
  | "completed"
  | "completed_with_projection_debt"
  | "failed"
  | "cancelled"
  | "expired";

const RUNNABLE_STATES = new Set<string>(["queued", "ready"]);
const EXECUTING_STATES = new Set<string>(["preflighting", "running"]);
const COMPLETION_STATES = new Set<string>([
  "completed",
  "completed_with_projection_debt",
]);
const TERMINAL_STATES = new Set<string>([
  "completed",
  "completed_with_projection_debt",
  "cancelled",
  "expired",
]);
const RECOVERABLE_STATES = new Set<string>([
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
const OPERATOR_DECISION_STATES = new Set<string>([
  "blocked_stale",
  "blocked_policy",
  "blocked_conflict",
  "needs_human_review",
  "failed",
]);

export function normalizeWorkCommissionState(state: string | undefined): string {
  return (state ?? "").trim().toLowerCase();
}

export function isRunnableWorkCommissionState(state: string | undefined): boolean {
  const normalized = normalizeWorkCommissionState(state);
  return RUNNABLE_STATES.has(normalized);
}

export function isExecutingWorkCommissionState(state: string | undefined): boolean {
  const normalized = normalizeWorkCommissionState(state);
  return EXECUTING_STATES.has(normalized);
}

export function isCompletionWorkCommissionState(state: string | undefined): boolean {
  const normalized = normalizeWorkCommissionState(state);
  return COMPLETION_STATES.has(normalized);
}

export function isTerminalWorkCommissionState(state: string | undefined): boolean {
  const normalized = normalizeWorkCommissionState(state);
  return TERMINAL_STATES.has(normalized);
}

export function isRecoverableWorkCommissionState(state: string | undefined): boolean {
  const normalized = normalizeWorkCommissionState(state);
  return RECOVERABLE_STATES.has(normalized);
}

export function requiresOperatorDecisionWorkCommissionState(
  state: string | undefined,
): boolean {
  const normalized = normalizeWorkCommissionState(state);
  return OPERATOR_DECISION_STATES.has(normalized);
}

export function satisfiesDependencyWorkCommissionState(
  state: string | undefined,
): boolean {
  return isCompletionWorkCommissionState(state);
}
