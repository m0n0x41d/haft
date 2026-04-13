export type AppPhase = "input" | "streaming" | "permission" | "question"

export function isStreamOwnedPhase(phase: AppPhase): boolean {
  return phase === "streaming"
    || phase === "permission"
    || phase === "question"
}

export function shouldFinalizeStreaming(
  phase: AppPhase,
  hasPendingUpdate: boolean,
): boolean {
  return isStreamOwnedPhase(phase) || hasPendingUpdate
}
