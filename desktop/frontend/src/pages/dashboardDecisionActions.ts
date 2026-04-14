import type { DecisionImplementGuard } from "../lib/api";

export interface DecisionImplementActionState {
  disabled: boolean;
  reason: string;
  confirmationMessages: string[];
  warningMessages: string[];
}

export function getDecisionImplementActionState(
  status: string,
  guard?: DecisionImplementGuard,
): DecisionImplementActionState {
  const normalizedStatus = status.trim().toLowerCase();
  const confirmationMessages = guard?.confirmation_messages ?? [];
  const warningMessages = guard?.warning_messages ?? [];

  if (normalizedStatus === "active") {
    if (guard?.blocked_reason) {
      return {
        disabled: true,
        reason: guard.blocked_reason,
        confirmationMessages,
        warningMessages,
      };
    }

    return {
      disabled: false,
      reason: "",
      confirmationMessages,
      warningMessages,
    };
  }

  if (normalizedStatus === "superseded" || normalizedStatus === "deprecated") {
    return {
      disabled: true,
      reason: `Implement is unavailable for ${normalizedStatus} DecisionRecords.`,
      confirmationMessages,
      warningMessages,
    };
  }

  return {
    disabled: true,
    reason: "Implement is available only for active DecisionRecords.",
    confirmationMessages,
    warningMessages,
  };
}
