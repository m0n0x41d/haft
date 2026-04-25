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

export type TaskRunState =
  | {
      kind: "running";
    }
  | {
      kind: "checkpointed";
    }
  | {
      kind: "completed";
    }
  | {
      kind: "blocked";
    }
  | {
      kind: "unavailable";
    };

export function taskRunState(status: string): TaskRunState {
  const normalizedStatus = status.trim().toLowerCase();

  if (normalizedStatus === "") {
    return { kind: "unavailable" };
  }

  if (normalizedStatus === "running") {
    return { kind: "running" };
  }

  if (normalizedStatus === "idle") {
    return { kind: "running" };
  }

  if (normalizedStatus === "checkpointed" || normalizedStatus === "waiting") {
    return { kind: "checkpointed" };
  }

  if (normalizedStatus === "completed" || normalizedStatus === "cancelled") {
    return { kind: "completed" };
  }

  if (
    normalizedStatus === "failed" ||
    normalizedStatus === "blocked" ||
    normalizedStatus === "needs attention" ||
    normalizedStatus === "interrupted"
  ) {
    return { kind: "blocked" };
  }

  return { kind: "completed" };
}

export function taskInputCapability(status: string): TaskInputCapability {
  const runState = taskRunState(status);

  if (runState.kind === "unavailable") {
    return {
      kind: "unavailable",
      placeholder: "Select a task",
    };
  }

  if (runState.kind === "running") {
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

export function taskAcceptsInput(status: string): boolean {
  const capability = taskInputCapability(status);

  return capability.kind === "live_input";
}

export function taskFollowUpAction(status: string): TaskFollowUpAction {
  const capability = taskInputCapability(status);

  if (capability.kind === "live_input") {
    return { kind: "write_live_input" };
  }

  if (capability.kind === "continuation") {
    return { kind: "continue_task" };
  }

  return { kind: "none" };
}

export function taskCanSubmitFollowUp(status: string): boolean {
  const action = taskFollowUpAction(status);

  return action.kind !== "none";
}
