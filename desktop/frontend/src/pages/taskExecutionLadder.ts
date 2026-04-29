import type { TaskState } from "../lib/api";
import {
  taskHasTerminalOutcome,
  taskIsLive,
  taskRunState,
  type TaskRunState,
} from "../lib/taskInput.ts";

export const EXECUTION_LADDER_LABELS = [
  "Planned",
  "Running",
  "Verifying",
  "Ready for PR",
  "Needs attention",
] as const;

export type ExecutionLadderLabel = (typeof EXECUTION_LADDER_LABELS)[number];
export type ExecutionLadderStepState = "complete" | "current" | "upcoming";

export interface ExecutionLadderStep {
  label: ExecutionLadderLabel;
  state: ExecutionLadderStepState;
}

export interface TaskExecutionLadder {
  currentLabel: ExecutionLadderLabel;
  rawStatus: string;
  summary: string;
  steps: ExecutionLadderStep[];
}

export function getTaskExecutionLadder(task: TaskState): TaskExecutionLadder | null {
  if (!isImplementationTask(task)) {
    return null;
  }

  const rawStatus = task.status.trim();
  const requiresVerification = taskRequiresVerification(task);
  const runState = taskRunState(task.status);
  const currentLabel = deriveExecutionLabel(runState, requiresVerification);

  if (!currentLabel) {
    return null;
  }

  const steps = EXECUTION_LADDER_LABELS.map((label) => ({
    label,
    state: executionStepState(label, currentLabel, runState, requiresVerification),
  }));

  return {
    currentLabel,
    rawStatus,
    summary: executionSummary(currentLabel, runState),
    steps,
  };
}

function isImplementationTask(task: TaskState): boolean {
  const title = task.title.trim();
  const prompt = task.prompt.trim();
  const runState = taskRunState(task.status);

  return (
    title.startsWith("Implement:") ||
    prompt.includes("## Implement Decision:") ||
    taskHasTerminalOutcome(runState, "ready_for_pr") ||
    taskHasTerminalOutcome(runState, "needs_attention")
  );
}

function taskRequiresVerification(task: TaskState): boolean {
  const prompt = task.prompt.trim();

  return (
    prompt.includes("## Invariants (must hold)") &&
    prompt.includes("## Affected Files")
  );
}

function deriveExecutionLabel(
  runState: TaskRunState,
  requiresVerification: boolean,
): ExecutionLadderLabel | null {
  if (runState.kind === "unavailable" || runState.kind === "pending") {
    return "Planned";
  }

  if (taskIsLive(runState)) {
    return "Running";
  }

  if (taskHasTerminalOutcome(runState, "completed")) {
    return requiresVerification ? "Verifying" : null;
  }

  if (taskHasTerminalOutcome(runState, "ready_for_pr")) {
    return "Ready for PR";
  }

  if (taskHasTerminalOutcome(runState, "needs_attention")) {
    return "Needs attention";
  }

  if (taskHasTerminalOutcome(runState, "failed")) {
    return "Needs attention";
  }

  if (taskHasTerminalOutcome(runState, "cancelled")) {
    return "Needs attention";
  }

  if (taskHasTerminalOutcome(runState, "interrupted")) {
    return "Needs attention";
  }

  return null;
}

function executionStepState(
  label: ExecutionLadderLabel,
  currentLabel: ExecutionLadderLabel,
  runState: TaskRunState,
  requiresVerification: boolean,
): ExecutionLadderStepState {
  if (currentLabel === "Planned") {
    return label === "Planned" ? "current" : "upcoming";
  }

  if (currentLabel === "Running") {
    if (label === "Planned") {
      return "complete";
    }

    return label === "Running" ? "current" : "upcoming";
  }

  if (currentLabel === "Verifying") {
    if (label === "Planned" || label === "Running") {
      return "complete";
    }

    return label === "Verifying" ? "current" : "upcoming";
  }

  if (currentLabel === "Ready for PR") {
    if (label === "Needs attention") {
      return "upcoming";
    }

    return label === "Ready for PR" ? "current" : "complete";
  }

  if (label === "Needs attention") {
    return "current";
  }

  if (taskHasTerminalOutcome(runState, "needs_attention")) {
    if (label === "Ready for PR") {
      return "upcoming";
    }

    return "complete";
  }

  if (label === "Ready for PR") {
    return "upcoming";
  }

  if (label === "Verifying" && !requiresVerification) {
    return "upcoming";
  }

  if (label === "Planned" || label === "Running") {
    return "complete";
  }

  return "upcoming";
}

function executionSummary(
  currentLabel: ExecutionLadderLabel,
  runState: TaskRunState,
): string {
  if (currentLabel === "Planned") {
    return "Task is queued. Execution has not started yet.";
  }

  if (currentLabel === "Running") {
    return "Implementation is running and task output is updating live.";
  }

  if (currentLabel === "Verifying") {
    return "Implementation finished. Post-execution verification is the active gate before PR handoff.";
  }

  if (currentLabel === "Ready for PR") {
    return "Implementation and post-execution verification passed. The worktree is ready for PR handoff.";
  }

  if (taskHasTerminalOutcome(runState, "needs_attention")) {
    return "Post-execution verification found an issue. Review the task output before continuing.";
  }

  if (taskHasTerminalOutcome(runState, "cancelled")) {
    return "Execution was cancelled before the task reached a verified handoff.";
  }

  if (taskHasTerminalOutcome(runState, "interrupted")) {
    return "Execution was interrupted before the task reached a verified handoff.";
  }

  if (taskHasTerminalOutcome(runState, "failed")) {
    return "Execution failed before the task reached a verified handoff.";
  }

  return "Execution needs attention before the task can move to PR handoff.";
}
