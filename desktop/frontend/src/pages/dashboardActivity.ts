import type { DecisionSummary, ProblemSummary } from "../lib/api";

export interface DashboardActivityItem {
  id: string;
  kind: "ProblemCard" | "DecisionRecord";
  page: "problems" | "decisions";
  title: string;
  summary: string;
  created_at: string;
}

const DEFAULT_ACTIVITY_LIMIT = 8;

export function buildRecentActivity(
  recentProblems: ProblemSummary[],
  recentDecisions: DecisionSummary[],
  limit = DEFAULT_ACTIVITY_LIMIT,
): DashboardActivityItem[] {
  const problemItems = recentProblems.map(problemToActivityItem);
  const decisionItems = recentDecisions.map(decisionToActivityItem);
  const mergedItems = [...problemItems, ...decisionItems];
  const sortedItems = mergedItems.sort(compareActivityItemsByCreatedAt);
  const limitedItems = sortedItems.slice(0, limit);

  return limitedItems;
}

function problemToActivityItem(problem: ProblemSummary): DashboardActivityItem {
  return {
    id: problem.id,
    kind: "ProblemCard",
    page: "problems",
    title: problem.title,
    summary: problem.signal,
    created_at: problem.created_at,
  };
}

function decisionToActivityItem(decision: DecisionSummary): DashboardActivityItem {
  const summary = decision.weakest_link
    ? `WLNK: ${decision.weakest_link}`
    : decision.status;

  return {
    id: decision.id,
    kind: "DecisionRecord",
    page: "decisions",
    title: decision.selected_title,
    summary,
    created_at: decision.created_at,
  };
}

function compareActivityItemsByCreatedAt(
  left: DashboardActivityItem,
  right: DashboardActivityItem,
): number {
  const leftTimestamp = timestampForSort(left.created_at);
  const rightTimestamp = timestampForSort(right.created_at);

  return rightTimestamp - leftTimestamp;
}

function timestampForSort(value: string): number {
  const parsedValue = Date.parse(value);

  if (Number.isNaN(parsedValue)) {
    return 0;
  }

  return parsedValue;
}
