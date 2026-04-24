import type { ProjectInfo } from "./api";

export type ProjectReadiness = "ready" | "needs_init" | "missing";

type ProjectReadinessInput = Pick<ProjectInfo, "status" | "exists" | "has_haft">;

export function projectReadiness(project: ProjectReadinessInput): ProjectReadiness {
  if (project.exists === false) {
    return "missing";
  }

  if (project.status) {
    return project.status;
  }

  if (project.has_haft === false) {
    return "needs_init";
  }

  return "ready";
}

export function projectIsRunnable(project: ProjectReadinessInput): boolean {
  return projectReadiness(project) === "ready";
}
