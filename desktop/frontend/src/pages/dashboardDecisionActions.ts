export interface DecisionImplementActionState {
  disabled: boolean;
  reason: string;
}

export function getDecisionImplementActionState(
  status: string,
): DecisionImplementActionState {
  const normalizedStatus = status.trim().toLowerCase();

  if (normalizedStatus === "active") {
    return {
      disabled: false,
      reason: "",
    };
  }

  if (normalizedStatus === "superseded" || normalizedStatus === "deprecated") {
    return {
      disabled: true,
      reason: `Implement is unavailable for ${normalizedStatus} DecisionRecords.`,
    };
  }

  return {
    disabled: true,
    reason: "Implement is available only for active DecisionRecords.",
  };
}
