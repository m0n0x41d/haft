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

export function taskInputCapability(status: string): TaskInputCapability {
  const normalizedStatus = status.trim().toLowerCase();

  if (normalizedStatus === "") {
    return {
      kind: "unavailable",
      placeholder: "Select a task",
    };
  }

  if (normalizedStatus === "running") {
    return {
      kind: "live_input",
      placeholder: "Message...",
    };
  }

  if (normalizedStatus === "idle") {
    return {
      kind: "live_input",
      placeholder: "Continue this conversation...",
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

export function taskCanSubmitFollowUp(status: string): boolean {
  const capability = taskInputCapability(status);

  return capability.kind !== "unavailable";
}
