import type {
  GovernanceOverview,
  ProblemCandidate,
  TaskState,
  WorkCommission,
} from "../lib/api";
import {
  taskHasTerminalOutcome,
  taskNeedsOperatorAction,
  taskRunState,
  type TaskRunState,
} from "../lib/taskInput.ts";
import {
  isCompletionWorkCommissionState,
  isExecutingWorkCommissionState,
  isTerminalWorkCommissionState,
  normalizeWorkCommissionState,
  requiresOperatorDecisionWorkCommissionState,
} from "../lib/workCommissionLifecycle.ts";

export type CoreTone = "neutral" | "accent" | "success" | "warning" | "danger";
export type CoreAttentionKind =
  | "runtime"
  | "conversation"
  | "governance"
  | "candidate";
export type CoreRuntimePhase =
  | "queued"
  | "preflight"
  | "frame"
  | "execute"
  | "measure"
  | "done"
  | "blocked";

export interface CoreAttentionItem {
  id: string;
  kind: CoreAttentionKind;
  tone: CoreTone;
  title: string;
  detail: string;
  meta: string;
  action: "open_runtime" | "open_task" | "open_decision" | "open_problem";
  actionRef: string;
}

export interface CoreRuntimeItem {
  id: string;
  state: string;
  phase: CoreRuntimePhase;
  decisionRef: string;
  problemRef: string;
  title: string;
  meta: string;
  tone: CoreTone;
  attentionReason: string;
}

export interface CoreAttentionInput {
  overview: GovernanceOverview;
  tasks: TaskState[];
  commissions: WorkCommission[];
}

export function buildCoreAttention(input: CoreAttentionInput): CoreAttentionItem[] {
  return [
    ...runtimeAttention(input.commissions),
    ...conversationAttention(input.tasks),
    ...governanceAttention(input.overview),
    ...candidateAttention(input.overview.problem_candidates ?? []),
  ];
}

export function buildCoreRuntimeItems(commissions: WorkCommission[]): CoreRuntimeItem[] {
  return commissions
    .filter((commission) => !commissionIsTerminal(commission))
    .map((commission) => {
      const state = normalizeWorkCommissionState(commission.state);
      const phase = commissionPhase(commission);
      const attentionReason = commission.operator?.attention_reason?.trim() ?? "";

      return {
        id: commission.id,
        state,
        phase,
        decisionRef: commission.decision_ref || "decision unknown",
        problemRef: commission.problem_card_ref || "problem unknown",
        title: commission.decision_ref || commission.id,
        meta: commission.scope?.target_branch
          ? `branch ${commission.scope.target_branch}`
          : commission.projection_policy || "local projection",
        tone: runtimeTone(commission),
        attentionReason,
      };
    });
}

export function commissionPhase(commission: WorkCommission): CoreRuntimePhase {
  const state = normalizeWorkCommissionState(commission.state);

  if (requiresOperatorDecisionWorkCommissionState(state)) return "blocked";
  if (state.includes("preflight")) return "preflight";
  if (state.includes("frame")) return "frame";
  if (state.includes("measure")) return "measure";
  if (isCompletionWorkCommissionState(state)) return "done";
  if (isExecutingWorkCommissionState(state) || state.includes("claim") || state.includes("execute")) {
    return "execute";
  }

  return "queued";
}

export function commissionIsTerminal(commission: WorkCommission): boolean {
  return isTerminalWorkCommissionState(commission.state);
}

function runtimeAttention(commissions: WorkCommission[]): CoreAttentionItem[] {
  return commissions
    .filter((commission) => commission.operator?.attention || commissionNeedsAction(commission))
    .map((commission) => ({
      id: `runtime:${commission.id}`,
      kind: "runtime",
      tone: runtimeTone(commission),
      title: runtimeTitle(commission),
      detail: commission.operator?.attention_reason?.trim()
        || `WorkCommission ${commission.id} is ${commission.state}.`,
      meta: [commission.id, commission.decision_ref].filter(Boolean).join(" · "),
      action: "open_runtime",
      actionRef: commission.id,
    }));
}

function conversationAttention(tasks: TaskState[]): CoreAttentionItem[] {
  return tasks
    .map((task) => ({
      task,
      state: taskRunState(task.status),
    }))
    .filter((item) => taskNeedsOperatorAction(item.state))
    .map((item) => {
      const tone = taskAttentionTone(item.state);
      const title = taskAttentionTitle(item.state);

      return {
        id: `task:${item.task.id}`,
        kind: "conversation",
        tone,
        title,
        detail: item.task.error_message || item.task.title || "Agent turn needs operator attention.",
        meta: [item.task.title, item.task.agent].filter(Boolean).join(" · "),
        action: "open_task",
        actionRef: item.task.id,
      };
    });
}

function governanceAttention(overview: GovernanceOverview): CoreAttentionItem[] {
  return (overview.findings ?? []).map((finding) => ({
    id: `finding:${finding.id}`,
    kind: "governance",
    tone: finding.drift_count > 0 || finding.days_stale > 0 ? "warning" : "accent",
    title: finding.category.replaceAll("_", " "),
    detail: finding.reason,
    meta: [finding.artifact_ref, finding.kind].filter(Boolean).join(" · "),
    action: "open_decision",
    actionRef: finding.artifact_ref,
  }));
}

function candidateAttention(candidates: ProblemCandidate[]): CoreAttentionItem[] {
  return candidates.map((candidate) => ({
    id: `candidate:${candidate.id}`,
    kind: "candidate",
    tone: "accent",
    title: "Follow-up problem candidate",
    detail: candidate.title,
    meta: candidate.category.replaceAll("_", " "),
    action: "open_problem",
    actionRef: candidate.problem_ref || candidate.id,
  }));
}

function commissionNeedsAction(commission: WorkCommission): boolean {
  const state = normalizeWorkCommissionState(commission.state);

  return requiresOperatorDecisionWorkCommissionState(state)
    || state.includes("stale")
    || state.includes("expired");
}

function runtimeTitle(commission: WorkCommission): string {
  const state = normalizeWorkCommissionState(commission.state);

  if (state.includes("block")) return "Runtime blocked";
  if (state.includes("fail")) return "Runtime failed";
  if (state.includes("stale") || state.includes("expired")) return "Runtime stale";

  return "Runtime needs attention";
}

function runtimeTone(commission: WorkCommission): CoreTone {
  const state = normalizeWorkCommissionState(commission.state);

  if (requiresOperatorDecisionWorkCommissionState(state)) return "danger";
  if (commission.operator?.attention || state.includes("stale") || state.includes("expired")) {
    return "warning";
  }
  if (isExecutingWorkCommissionState(state) || state.includes("claim") || state.includes("execute")) {
    return "accent";
  }

  return "neutral";
}

function taskAttentionTone(state: TaskRunState): CoreTone {
  if (taskHasTerminalOutcome(state, "failed") || taskHasTerminalOutcome(state, "interrupted")) {
    return "danger";
  }

  return "warning";
}

function taskAttentionTitle(state: TaskRunState): string {
  if (taskHasTerminalOutcome(state, "failed")) {
    return "Conversation failed";
  }

  if (taskHasTerminalOutcome(state, "interrupted")) {
    return "Conversation interrupted";
  }

  return "Conversation needs input";
}
