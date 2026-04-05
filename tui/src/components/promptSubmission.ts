import type { AttachmentItem } from "./attachmentLayout.js"

export interface PromptSubmission {
  text: string
  attachments: AttachmentItem[]
}

interface ShiftPromptSubmissionsResult {
  current: PromptSubmission | null
  remaining: PromptSubmission[]
}

export interface RestoredQueuedSubmission {
  draft: PromptSubmission | null
  remaining: PromptSubmission[]
  attachmentSelection: boolean
}

export interface DrainedPromptSubmissions {
  replay: PromptSubmission[]
  remaining: PromptSubmission[]
}

export type QueuedPromptReplayDisposition =
  | "continue"
  | "pause"
  | "submit"

const PAUSING_LOCAL_SLASH_COMMANDS = new Set([
  "/compact",
  "/help",
  "/model",
  "/resume",
])

export function hasSubmittableText(text: string): boolean {
  return text.trim().length > 0
}

export function createPromptSubmission(
  text: string,
  attachments: readonly AttachmentItem[],
): PromptSubmission {
  return {
    text,
    attachments: cloneAttachmentItems(attachments),
  }
}

export function submissionTexts(
  submissions: readonly PromptSubmission[],
): string[] {
  return submissions.map((submission) => submission.text)
}

export function leadingSlashCommand(text: string): string | null {
  if (!text.startsWith("/")) {
    return null
  }

  const [command] = text.split(" ")

  return command ?? null
}

export function queuedPromptReplayDisposition(
  text: string,
): QueuedPromptReplayDisposition {
  const command = leadingSlashCommand(text)

  if (command === null) {
    return "submit"
  }
  if (PAUSING_LOCAL_SLASH_COMMANDS.has(command)) {
    return "pause"
  }

  return "submit"
}

export function shouldResumeQueuedReplayAfterPickerCancel(
  text: string,
): boolean {
  const command = leadingSlashCommand(text)

  return command === "/help" || command === "/model" || command === "/resume"
}

export function shiftPromptSubmissions(
  submissions: readonly PromptSubmission[],
): ShiftPromptSubmissionsResult {
  const [current, ...remaining] = submissions

  return {
    current: current ? clonePromptSubmission(current) : null,
    remaining: [...remaining],
  }
}

export function restoreQueuedSubmission(
  submissions: readonly PromptSubmission[],
): RestoredQueuedSubmission {
  const shifted = shiftPromptSubmissions(submissions)

  return {
    draft: shifted.current,
    remaining: shifted.remaining,
    attachmentSelection: false,
  }
}

export function drainPromptSubmissions(
  submissions: readonly PromptSubmission[],
): DrainedPromptSubmissions {
  const stopIndex = submissions.findIndex((submission) => {
    return queuedPromptReplayDisposition(submission.text) !== "continue"
  })
  const replayCount = stopIndex === -1 ? submissions.length : stopIndex + 1
  const replay = submissions
    .slice(0, replayCount)
    .map(clonePromptSubmission)
  const remaining = submissions.slice(replayCount)

  return {
    replay,
    remaining: [...remaining],
  }
}

function cloneAttachmentItems(
  attachments: readonly AttachmentItem[],
): AttachmentItem[] {
  return attachments.map((attachment) => ({
    ...attachment,
  }))
}

function clonePromptSubmission(
  submission: PromptSubmission,
): PromptSubmission {
  return createPromptSubmission(
    submission.text,
    submission.attachments,
  )
}
