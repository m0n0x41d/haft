import type { TaskState } from "../lib/api";

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
  const currentLabel = deriveExecutionLabel(rawStatus, requiresVerification);

  if (!currentLabel) {
    return null;
  }

  const steps = EXECUTION_LADDER_LABELS.map((label) => ({
    label,
    state: executionStepState(label, currentLabel, rawStatus, requiresVerification),
  }));

  return {
    currentLabel,
    rawStatus,
    summary: executionSummary(currentLabel, rawStatus),
    steps,
  };
}

function isImplementationTask(task: TaskState): boolean {
  const title = task.title.trim();
  const prompt = task.prompt.trim();

  return (
    title.startsWith("Implement:") ||
    prompt.includes("## Implement Decision:") ||
    task.status === "Ready for PR" ||
    task.status === "Needs attention"
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
  rawStatus: string,
  requiresVerification: boolean,
): ExecutionLadderLabel | null {
  if (rawStatus === "" || rawStatus === "pending") {
    return "Planned";
  }

  if (rawStatus === "running") {
    return "Running";
  }

  if (rawStatus === "completed") {
    return requiresVerification ? "Verifying" : null;
  }

  if (rawStatus === "Ready for PR") {
    return "Ready for PR";
  }

  if (rawStatus === "Needs attention") {
    return "Needs attention";
  }

  if (
    rawStatus === "failed" ||
    rawStatus === "cancelled" ||
    rawStatus === "interrupted"
  ) {
    return "Needs attention";
  }

  return null;
}

function executionStepState(
  label: ExecutionLadderLabel,
  currentLabel: ExecutionLadderLabel,
  rawStatus: string,
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

  if (rawStatus === "Needs attention") {
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
  rawStatus: string,
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

  if (rawStatus === "Needs attention") {
    return "Post-execution verification found an issue. Review the task output before continuing.";
  }

  if (rawStatus === "cancelled") {
    return "Execution was cancelled before the task reached a verified handoff.";
  }

  if (rawStatus === "interrupted") {
    return "Execution was interrupted before the task reached a verified handoff.";
  }

  if (rawStatus === "failed") {
    return "Execution failed before the task reached a verified handoff.";
  }

  return "Execution needs attention before the task can move to PR handoff.";
}
