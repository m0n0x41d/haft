import type { PiUI } from "./pi-api.ts";

// D4 cockpit (sol-20260610-92b4e846): keep governance state visible at the
// point of work via Pi's string-based UI surfaces. Visibility only — no
// authority, no workflow control.

export const COCKPIT_KEY = "haft";

const widgetLineBudget = 100;

export function governorWidgetLines(governorText: string): string[] {
  const lines = governorText.split("\n");
  const counts = lines
    .filter((line) => line.startsWith("Overseer: ") || line.startsWith("Decisions: "));
  const methodRuns = sectionItems(lines, "Open method runs:")
    .slice(0, 1)
    .map((item) => `Open method run: ${item}`);

  return [...counts, ...methodRuns]
    .map((line) => clipLine(line, widgetLineBudget));
}

export function updateCockpitWidget(ui: PiUI | undefined, governorText: string): void {
  const lines = governorWidgetLines(governorText);
  if (lines.length === 0) {
    return;
  }

  ui?.setWidget?.(COCKPIT_KEY, lines);
}

export function reportToolStatus(
  ui: PiUI | undefined,
  toolName: string,
  action: string | undefined,
  ok: boolean
): void {
  const call = action === undefined ? toolName : `${toolName}(${action})`;
  const mark = ok ? "✓" : "✗";
  ui?.setStatus?.(COCKPIT_KEY, `haft ${call} ${mark}`);
}

function sectionItems(lines: string[], header: string): string[] {
  const start = lines.indexOf(header);
  if (start === -1) {
    return [];
  }

  const items: string[] = [];
  for (const line of lines.slice(start + 1)) {
    if (!line.startsWith("- ")) {
      break;
    }

    items.push(line.slice(2));
  }
  return items;
}

function clipLine(line: string, maxChars: number): string {
  if (line.length <= maxChars) {
    return line;
  }

  return `${line.slice(0, maxChars - 1)}…`;
}
