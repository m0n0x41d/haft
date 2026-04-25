import type { ProjectInfo } from "./api";

export type ProjectReadiness = "ready" | "needs_init" | "needs_onboard" | "missing";

type ProjectReadinessInput = Pick<ProjectInfo, "status" | "exists" | "has_haft" | "has_specs">;

const PROJECT_READINESS_BADGE_LABELS: Record<ProjectReadiness, string> = {
  ready: "ready",
  needs_init: "init",
  needs_onboard: "onboard",
  missing: "missing",
};

const PROJECT_ACTIVATION_LABELS: Record<ProjectReadiness, string> = {
  ready: "Switch",
  needs_init: "Init & switch",
  needs_onboard: "Onboard",
  missing: "Missing",
};

const PROJECT_TASK_BLOCKED_TITLES: Record<ProjectReadiness, string> = {
  ready: "New task",
  needs_init: "Project needs initialization before generic tasks",
  needs_onboard: "Project needs onboarding before generic tasks",
  missing: "Project is missing",
};

const PROJECT_READINESS_RANKS: Record<ProjectReadiness, number> = {
  ready: 0,
  needs_onboard: 1,
  needs_init: 2,
  missing: 3,
};

export function projectReadiness(project: ProjectReadinessInput): ProjectReadiness {
  const candidates = [
    projectFactReadiness(project),
    project.status,
  ].filter(isProjectReadiness);

  return candidates.reduce(moreRestrictiveProjectReadiness, "ready");
}

function projectFactReadiness(project: ProjectReadinessInput): ProjectReadiness {
  if (project.exists === false) {
    return "missing";
  }

  if (project.has_haft === false) {
    return "needs_init";
  }

  if (project.has_specs === false) {
    return "needs_onboard";
  }

  return "ready";
}

function isProjectReadiness(value: ProjectReadiness | undefined): value is ProjectReadiness {
  return value !== undefined;
}

function moreRestrictiveProjectReadiness(
  left: ProjectReadiness,
  right: ProjectReadiness,
): ProjectReadiness {
  if (PROJECT_READINESS_RANKS[right] > PROJECT_READINESS_RANKS[left]) {
    return right;
  }

  return left;
}

export function projectIsRunnable(project: ProjectReadinessInput): boolean {
  return projectReadiness(project) === "ready";
}

export function projectIsMissing(project: ProjectReadinessInput): boolean {
  return projectReadiness(project) === "missing";
}

export function projectNeedsInitialization(project: ProjectReadinessInput): boolean {
  return projectReadiness(project) === "needs_init";
}

export function projectNeedsOnboarding(project: ProjectReadinessInput): boolean {
  return projectReadiness(project) === "needs_onboard";
}

export function projectReadinessBadgeLabel(project: ProjectReadinessInput): string {
  return PROJECT_READINESS_BADGE_LABELS[projectReadiness(project)];
}

export function projectActivationLabel(project: ProjectReadinessInput): string {
  return PROJECT_ACTIVATION_LABELS[projectReadiness(project)];
}

export function projectTaskBlockedTitle(project: ProjectReadinessInput): string {
  return PROJECT_TASK_BLOCKED_TITLES[projectReadiness(project)];
}
