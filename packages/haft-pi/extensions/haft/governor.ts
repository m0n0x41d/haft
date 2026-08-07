import { bridgeFor } from "./bridge.ts";

export function composeGovernedSystemPrompt(
  currentSystemPrompt: string,
  status: string
): string {
  const guidance = renderPromptGovernorBlock(status);
  const parts = [
    currentSystemPrompt,
    guidance
  ];
  return parts
    .map((part) => part.trim())
    .filter((part) => part.length > 0)
    .join("\n\n");
}

export async function fetchGovernorStatus(
  projectRoot: string,
  signal: AbortSignal | undefined
): Promise<string> {
  try {
    const bridge = await bridgeFor(projectRoot, signal);
    // No `context` arg here: haft_query treats it as an artifact-context
    // FILTER, not an annotation — passing prose would filter the dashboard
    // down to nothing.
    const args = {
      action: "status",
      view: "governor",
      full: false
    };
    return await bridge.callTool("haft_query", args, signal);
  } catch (error) {
    return formatStartupError(error);
  }
}

export function renderPromptGovernorBlock(status: string): string {
  // A governor-aware kernel returns its own budgeted "## Haft Project State
  // (governor)" block; an older kernel ignores `view` and returns the full
  // dashboard, so the clip below stays as the fallback budget.
  const clippedStatus = clipText(withGovernorHeader(status), 2600);
  return [
    clippedStatus,
    "",
    "## Haft Carrier Rules",
    "- Pi is the host carrier; Haft kernel and `.haft/` artifacts remain the governance authority.",
    "- Description is not work: call the native Haft tools for status, context, framing, comparison, and verification when the task is open-ended or governed.",
    "- Before non-trivial code edits call haft_method(action=\"pull\", ...); before claiming completion close the same run with haft_method(action=\"close\", pull_id=...) carrying evidence or explicit waivers.",
    "- Binding decisions and commissions require explicit human action. Do not silently create them from this prompt guidance.",
    "- Use the smallest reversible step, then gather evidence before claiming completion."
  ].join("\n");
}

export function formatStartupError(error: unknown): string {
  const message = error instanceof Error ? error.message : String(error);
  return [
    "Haft status could not be loaded for this turn.",
    `Reason: ${message}`,
    "If this is a Haft project, check that the `haft` binary is on PATH or set HAFT_BIN."
  ].join("\n");
}

export function withGovernorHeader(status: string): string {
  if (status.startsWith("## Haft")) {
    return status;
  }

  return `## Haft Project State\n${status}`;
}

export function clipText(text: string, maxChars: number): string {
  if (text.length <= maxChars) {
    return text;
  }

  const clipped = text
    .slice(0, maxChars)
    .trimEnd();
  return `${clipped}\n[truncated by @haft/pi prompt governor]`;
}
