export type CommissionSelector = "stale" | "open" | "terminal" | "runnable" | "all";

export function listCommissionsIpcArgs(selector: CommissionSelector): {
  selector: CommissionSelector;
  state: string;
  olderThan: string;
} {
  return {
    selector,
    state: "",
    olderThan: "",
  };
}

export function commissionIpcArgs(commissionID: string): {
  commissionId: string;
} {
  return {
    commissionId: commissionID,
  };
}

export function commissionTailIpcArgs(
  commissionID: string,
  lineCount: number,
): {
  commissionId: string;
  lineCount: number;
} {
  return {
    commissionId: commissionID,
    lineCount,
  };
}
