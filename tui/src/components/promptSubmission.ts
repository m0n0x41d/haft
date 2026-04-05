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
