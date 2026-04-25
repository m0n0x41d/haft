import { taskHasTerminalOutcome, taskIsLive, taskRunState } from "../../lib/taskInput.ts";

export type TaskStatus =
  | "running"
  | "checkpointed"
  | "idle"
  | "completed"
  | "failed"
  | "blocked"
  | "cancelled"
  | "pending"
  | (string & {}); // accepts unknown strings at the type boundary; unknown → muted

export interface StatusDotProps {
  status: TaskStatus;
  className?: string;
}

const STATUS_CLASSES: Record<string, string> = {
  blocked: "bg-danger",
  cancelled: "bg-text-muted",
  checkpointed: "bg-warning",
  completed: "bg-success",
  failed: "bg-danger",
  idle: "bg-accent",
  pending: "bg-warning",
  running: "bg-accent animate-pulse",
  unavailable: "bg-text-muted",
};

/**
 * Status indicator dot used in the sidebar task list and other inline
 * agent-state displays. Pulse animation for `running` mirrors the
 * streaming indicator throughout the cockpit.
 */
export function StatusDot({ status, className = "" }: StatusDotProps) {
  const tone = statusClassName(status);

  return (
    <span className={`h-2 w-2 shrink-0 rounded-full ${tone} ${className}`} />
  );
}

function statusClassName(status: string): string {
  const state = taskRunState(status);

  if (taskIsLive(state)) {
    return state.mode === "running"
      ? STATUS_CLASSES.running
      : STATUS_CLASSES.idle;
  }

  if (state.kind === "needs_operator") {
    return state.reason === "blocked"
      ? STATUS_CLASSES.blocked
      : STATUS_CLASSES[state.reason] ?? STATUS_CLASSES.unavailable;
  }

  if (taskHasTerminalOutcome(state, "completed")) {
    return STATUS_CLASSES.completed;
  }

  if (taskHasTerminalOutcome(state, "failed")) {
    return STATUS_CLASSES.failed;
  }

  if (taskHasTerminalOutcome(state, "cancelled")) {
    return STATUS_CLASSES.cancelled;
  }

  if (state.kind === "pending") {
    return STATUS_CLASSES.pending;
  }

  return STATUS_CLASSES.unavailable;
}
