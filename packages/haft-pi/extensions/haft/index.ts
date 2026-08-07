import { bridgeFor, stopAllBridges } from "./bridge.ts";
import { reportToolStatus, updateCockpitWidget } from "./cockpit.ts";
import { composeGovernedSystemPrompt, fetchGovernorStatus } from "./governor.ts";
import type { PiExtensionAPI, PiToolResult, PiContext } from "./pi-api.ts";
import { contextCwd, findHaftProjectRoot, requireHaftProjectRoot } from "./project.ts";
import { HAFT_TOOLS, type HaftToolSpec } from "./tools.ts";

export default function haftPi(pi: PiExtensionAPI) {
  HAFT_TOOLS.forEach((spec) => pi.registerTool({
    ...spec,
    execute: (_toolCallId, params, signal, _onUpdate, ctx) => executeHaftTool(spec, params, signal, ctx)
  }));

  pi.on("before_agent_start", async (event, ctx) => {
    const projectRoot = findHaftProjectRoot(contextCwd(ctx));
    if (projectRoot === undefined) {
      return;
    }

    const status = await fetchGovernorStatus(projectRoot, ctx.signal);
    updateCockpitWidget(ctx.ui, status);
    return {
      systemPrompt: composeGovernedSystemPrompt(event.systemPrompt, status)
    };
  });

  pi.on("session_shutdown", () => {
    stopAllBridges();
  });
}

async function executeHaftTool(
  spec: HaftToolSpec,
  params: unknown,
  signal: AbortSignal | undefined,
  ctx: PiContext
): Promise<PiToolResult> {
  const args = cleanArgs(params as Record<string, unknown>);
  const action = typeof args.action === "string" ? args.action : undefined;
  try {
    const projectRoot = requireHaftProjectRoot(ctx);
    const bridge = await bridgeFor(projectRoot, signal);
    const text = await bridge.callTool(spec.name, args, signal);
    reportToolStatus(ctx.ui, spec.name, action, true);
    return {
      content: [
        {
          type: "text",
          text
        }
      ],
      details: {
        action: args.action,
        projectRoot
      }
    };
  } catch (error) {
    reportToolStatus(ctx.ui, spec.name, action, false);
    throw error;
  }
}

function cleanArgs(args: Record<string, unknown>): Record<string, unknown> {
  const entries = Object.entries(args)
    .filter((entry) => entry[1] !== undefined);

  return Object.fromEntries(entries);
}
