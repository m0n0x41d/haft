export type BeforeAgentStartEvent = {
  systemPrompt: string;
  prompt?: string;
};

export type BeforeAgentStartResult = {
  systemPrompt?: string;
};

export type PiUI = {
  setStatus?(key: string, text: string): void;
  setWidget?(key: string, lines: string[]): void;
  notify?(message: string, level: "info" | "warning" | "error"): void;
};

export type PiContext = {
  cwd: string;
  signal?: AbortSignal;
  ui?: PiUI;
};

export type ToolContent = {
  type: string;
  text?: string;
};

export type PiToolResult = {
  content: ToolContent[];
  details?: Record<string, unknown>;
};

export type PiToolUpdate = {
  content?: ToolContent[];
  details?: Record<string, unknown>;
};

export type PiToolDefinition = {
  name: string;
  label: string;
  description: string;
  promptSnippet?: string;
  promptGuidelines?: string[];
  parameters: unknown;
  execute: (
    toolCallId: string,
    params: unknown,
    signal: AbortSignal | undefined,
    onUpdate: ((update: PiToolUpdate) => void) | undefined,
    ctx: PiContext
  ) => Promise<PiToolResult>;
};

export type PiExtensionAPI = {
  registerTool(tool: PiToolDefinition): void;
  on(
    event: "before_agent_start",
    handler: (
      event: BeforeAgentStartEvent,
      ctx: PiContext
    ) => Promise<BeforeAgentStartResult | void> | BeforeAgentStartResult | void
  ): void;
  on(event: "session_shutdown", handler: () => Promise<void> | void): void;
};
