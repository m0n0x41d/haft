export type TaskInputCapability =
  | {
      kind: "live_input";
      placeholder: string;
    }
  | {
      kind: "continuation";
      placeholder: string;
    }
  | {
      kind: "unavailable";
      placeholder: string;
    };

export type TaskFollowUpAction =
  | {
      kind: "write_live_input";
    }
  | {
      kind: "continue_task";
    }
  | {
      kind: "none";
    };

export type TaskFollowUpSubmission =
  | {
      kind: "write_live_input";
      id: string;
      data: string;
    }
  | {
      kind: "continue_task";
      id: string;
      message: string;
    }
  | {
      kind: "none";
      reason: "empty_input" | "unavailable";
    };

export type TaskTerminalOutcome =
  | "completed"
  | "failed"
  | "cancelled"
  | "interrupted"
  | "ready_for_pr"
  | "needs_attention"
  | "unknown";

export type TaskRunState =
  | {
      kind: "unavailable";
      rawStatus: string;
      normalizedStatus: string;
    }
  | {
      kind: "pending";
      rawStatus: string;
      normalizedStatus: string;
    }
  | {
      kind: "live";
      rawStatus: string;
      normalizedStatus: string;
      mode: "running" | "idle";
    }
  | {
      kind: "needs_operator";
      rawStatus: string;
      normalizedStatus: string;
      reason: "checkpointed" | "waiting" | "blocked";
    }
  | {
      kind: "terminal";
      rawStatus: string;
      normalizedStatus: string;
      outcome: TaskTerminalOutcome;
    };

type TaskRunStateCore =
  | {
      kind: "pending";
    }
  | {
      kind: "live";
      mode: "running" | "idle";
    }
  | {
      kind: "needs_operator";
      reason: "checkpointed" | "waiting" | "blocked";
    }
  | {
      kind: "terminal";
      outcome: TaskTerminalOutcome;
    };

const TASK_RUN_STATE_BY_STATUS: Partial<Record<string, TaskRunStateCore>> = {
  pending: { kind: "pending" },
  running: { kind: "live", mode: "running" },
  idle: { kind: "live", mode: "idle" },
  checkpointed: { kind: "needs_operator", reason: "checkpointed" },
  waiting: { kind: "needs_operator", reason: "waiting" },
  blocked: { kind: "needs_operator", reason: "blocked" },
  completed: { kind: "terminal", outcome: "completed" },
  failed: { kind: "terminal", outcome: "failed" },
  cancelled: { kind: "terminal", outcome: "cancelled" },
  canceled: { kind: "terminal", outcome: "cancelled" },
  interrupted: { kind: "terminal", outcome: "interrupted" },
  "ready for pr": { kind: "terminal", outcome: "ready_for_pr" },
  "needs attention": { kind: "terminal", outcome: "needs_attention" },
};

export function taskRunState(rawStatus: string): TaskRunState {
  const normalizedStatus = normalizeTaskStatus(rawStatus);

  if (normalizedStatus === "") {
    return {
      kind: "unavailable",
      rawStatus,
      normalizedStatus,
    };
  }

  const core = TASK_RUN_STATE_BY_STATUS[normalizedStatus] ?? unknownTaskRunStateCore();

  return {
    ...core,
    rawStatus,
    normalizedStatus,
  };
}

export function taskIsLive(
  state: TaskRunState,
): state is Extract<TaskRunState, { kind: "live" }> {
  return state.kind === "live";
}

export function taskCanCancel(state: TaskRunState): boolean {
  return taskIsLive(state);
}

export function taskCanArchive(state: TaskRunState): boolean {
  return !taskIsLive(state);
}

export function taskCountsAsActive(state: TaskRunState): boolean {
  return state.kind === "live" || state.kind === "needs_operator";
}

export function taskAcceptsInput(state: TaskRunState): boolean {
  return taskIsLive(state);
}

export function taskHasTerminalOutcome(
  state: TaskRunState,
  outcome: TaskTerminalOutcome,
): boolean {
  return state.kind === "terminal" && state.outcome === outcome;
}

export function taskNeedsOperatorAction(state: TaskRunState): boolean {
  if (state.kind === "live" && state.mode === "idle") {
    return true;
  }

  if (state.kind === "needs_operator") {
    return true;
  }

  if (state.kind !== "terminal") {
    return false;
  }

  return state.outcome === "failed"
    || state.outcome === "interrupted"
    || state.outcome === "needs_attention";
}

export function taskInputCapability(state: TaskRunState): TaskInputCapability {
  if (state.kind === "unavailable") {
    return {
      kind: "unavailable",
      placeholder: "Select a task",
    };
  }

  if (taskIsLive(state)) {
    return {
      kind: "live_input",
      placeholder: "Message...",
    };
  }

  return {
    kind: "continuation",
    placeholder: "Continue this conversation...",
  };
}

export function taskFollowUpAction(state: TaskRunState): TaskFollowUpAction {
  const capability = taskInputCapability(state);

  if (capability.kind === "live_input") {
    return { kind: "write_live_input" };
  }

  if (capability.kind === "continuation") {
    return { kind: "continue_task" };
  }

  return { kind: "none" };
}

export function taskCanSubmitFollowUp(state: TaskRunState): boolean {
  const action = taskFollowUpAction(state);

  return action.kind !== "none";
}

export function taskFollowUpSubmission(
  id: string,
  state: TaskRunState,
  value: string,
): TaskFollowUpSubmission {
  const trimmed = value.trim();

  if (trimmed === "") {
    return {
      kind: "none",
      reason: "empty_input",
    };
  }

  const action = taskFollowUpAction(state);

  if (action.kind === "write_live_input") {
    return {
      kind: "write_live_input",
      id,
      data: trimmed,
    };
  }

  if (action.kind === "continue_task") {
    return {
      kind: "continue_task",
      id,
      message: trimmed,
    };
  }

  return {
    kind: "none",
    reason: "unavailable",
  };
}

function normalizeTaskStatus(status: string): string {
  const normalizedStatus = status.trim().toLowerCase();

  return normalizedStatus.replaceAll("_", " ");
}

function unknownTaskRunStateCore(): TaskRunStateCore {
  return {
    kind: "terminal",
    outcome: "unknown",
  };
}
