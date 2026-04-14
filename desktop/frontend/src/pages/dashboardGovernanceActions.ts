import type { GovernanceFinding, ProblemCandidate } from "../lib/api";

const ACTIONABLE_DECISION_FINDING_CATEGORIES = new Set([
  "decision_stale",
  "evidence_expired",
  "reff_degraded",
]);

export interface GovernanceFindingActionState {
  showActions: boolean;
  adoptCandidateID: string;
  adoptDisabled: boolean;
  adoptReason: string;
}

export function getGovernanceFindingActionState(
  finding: GovernanceFinding,
  candidates: ProblemCandidate[],
): GovernanceFindingActionState {
  const normalizedKind = normalizeValue(finding.kind);
  const normalizedCategory = normalizeValue(finding.category);
  const isActionable =
    normalizedKind === "decisionrecord" &&
    ACTIONABLE_DECISION_FINDING_CATEGORIES.has(normalizedCategory);

  if (!isActionable) {
    return {
      showActions: false,
      adoptCandidateID: "",
      adoptDisabled: true,
      adoptReason: "",
    };
  }

  const matchingCandidate = candidates.find((candidate) => {
    const sameArtifact = candidate.source_artifact_ref === finding.artifact_ref;
    const sameCategory = normalizeValue(candidate.category) === normalizedCategory;
    const isActive = normalizeValue(candidate.status) === "active";

    return sameArtifact && sameCategory && isActive;
  });

  if (matchingCandidate) {
    return {
      showActions: true,
      adoptCandidateID: matchingCandidate.id,
      adoptDisabled: false,
      adoptReason: "",
    };
  }

  return {
    showActions: true,
    adoptCandidateID: "",
    adoptDisabled: true,
    adoptReason: "Adopt is unavailable because no active follow-up candidate matches this finding.",
  };
}

function normalizeValue(value: string): string {
  return value.trim().toLowerCase();
}
