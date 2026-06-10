import { spawn, type ChildProcessWithoutNullStreams } from "node:child_process";
import { existsSync } from "node:fs";
import { join } from "node:path";

type JsonRpcRequest = {
  jsonrpc: "2.0";
  id?: number;
  method: string;
  params?: unknown;
};

export type JsonRpcResponse = {
  jsonrpc: "2.0";
  id?: number;
  result?: unknown;
  error?: {
    code: number;
    message: string;
  };
};

type PendingCall = {
  resolve: (value: unknown) => void;
  reject: (error: Error) => void;
};

type ToolContent = {
  type: string;
  text?: string;
};

type ToolCallResult = {
  content?: ToolContent[];
  isError?: boolean;
};

const bridgeRegistry = new Map<string, HaftBridge>();

export async function bridgeFor(
  projectRoot: string,
  signal: AbortSignal | undefined
): Promise<HaftBridge> {
  const existing = bridgeRegistry.get(projectRoot);
  if (existing !== undefined) {
    await existing.start(signal);
    return existing;
  }

  const bridge = new HaftBridge(projectRoot);
  bridgeRegistry.set(projectRoot, bridge);
  await bridge.start(signal);
  return bridge;
}

export function stopAllBridges(): void {
  bridgeRegistry.forEach((bridge) => bridge.stop());
  bridgeRegistry.clear();
}

export class HaftBridge {
  private process: ChildProcessWithoutNullStreams | undefined;
  private startPromise: Promise<void> | undefined;
  private buffer = "";
  private nextID = 0;
  private pending = new Map<number, PendingCall>();
  private stderrTail: string[] = [];
  private advertisedTools = new Set<string>();
  private readonly projectRoot: string;

  constructor(projectRoot: string) {
    this.projectRoot = projectRoot;
  }

  // Concurrent callers share one start: the spawn+initialize handshake runs
  // once, everyone awaits the same promise, and a failed start resets state
  // so the next call can retry with a fresh process.
  start(signal: AbortSignal | undefined): Promise<void> {
    if (this.startPromise === undefined) {
      this.startPromise = this.spawnAndInitialize(signal).catch((error: unknown) => {
        this.startPromise = undefined;
        this.stop();
        throw error;
      });
    }

    return this.startPromise;
  }

  private async spawnAndInitialize(signal: AbortSignal | undefined): Promise<void> {
    const command = resolveHaftCommand(this.projectRoot);
    const child = spawn(command, ["serve"], {
      cwd: this.projectRoot,
      env: {
        ...process.env,
        HAFT_PROJECT_ROOT: this.projectRoot
      },
      stdio: "pipe"
    });

    this.process = child;
    child.stdout.setEncoding("utf8");
    child.stderr.setEncoding("utf8");
    child.stdout.on("data", (chunk: string) => this.acceptStdout(chunk));
    child.stderr.on("data", (chunk: string) => this.acceptStderr(chunk));
    child.on("error", (error) => this.handleSpawnError(error));
    child.on("exit", (code, cause) => this.handleExit(code, cause));

    await this.initialize(signal);
  }

  async callTool(
    name: string,
    args: Record<string, unknown>,
    signal: AbortSignal | undefined
  ): Promise<string> {
    this.ensureToolAdvertised(name);
    const result = await this.call(
      "tools/call",
      {
        name,
        arguments: args
      },
      30000,
      signal
    );

    return toolText(result);
  }

  stop(): void {
    const child = this.process;
    if (child === undefined) {
      return;
    }

    this.process = undefined;
    this.startPromise = undefined;
    child.stdin.end();
    child.kill();
    this.failAll(new Error("Haft MCP bridge stopped."));
  }

  // Spawn errors (e.g. ENOENT for a missing haft binary) arrive without a
  // matching `exit` event; reset state here or the bridge stays "running"
  // with a dead child forever.
  private handleSpawnError(error: Error): void {
    this.process = undefined;
    this.startPromise = undefined;
    this.failAll(error);
  }

  private ensureToolAdvertised(name: string): void {
    if (this.advertisedTools.has(name)) {
      return;
    }

    const available = [...this.advertisedTools].sort().join(", ");
    throw new Error(
      `Tool ${name} is not advertised by this haft binary. ` +
      `Upgrade haft or set HAFT_BIN to a newer build. Available tools: ${available}`
    );
  }

  private async initialize(signal: AbortSignal | undefined): Promise<void> {
    await this.call(
      "initialize",
      {
        protocolVersion: "2024-11-05",
        capabilities: {},
        clientInfo: {
          name: "@haft/pi",
          version: "0.0.0-dev"
        }
      },
      10000,
      signal
    );

    this.notify("notifications/initialized", {});

    const listed = await this.call("tools/list", {}, 10000, signal);
    this.advertisedTools = new Set(toolNames(listed));
    if (!this.advertisedTools.has("haft_query")) {
      const available = [...this.advertisedTools].sort().join(", ");
      throw new Error(`haft_query tool was not advertised by haft serve. Available tools: ${available}`);
    }
  }

  private call(
    method: string,
    params: unknown,
    timeoutMs: number,
    signal: AbortSignal | undefined
  ): Promise<unknown> {
    const child = this.requireProcess();
    const id = this.nextRequestID();
    const request: JsonRpcRequest = {
      jsonrpc: "2.0",
      id,
      method,
      params
    };
    const body = `${JSON.stringify(request)}\n`;

    return new Promise((resolveCall, rejectCall) => {
      const cleanup = this.registerPending(id, resolveCall, rejectCall, timeoutMs, signal);
      child.stdin.write(body, "utf8", (error) => {
        if (error == null) {
          return;
        }

        cleanup();
        rejectCall(error);
      });
    });
  }

  private notify(method: string, params: unknown): void {
    const child = this.requireProcess();
    const request: JsonRpcRequest = {
      jsonrpc: "2.0",
      method,
      params
    };
    const body = `${JSON.stringify(request)}\n`;
    child.stdin.write(body);
  }

  private registerPending(
    id: number,
    resolveCall: (value: unknown) => void,
    rejectCall: (error: Error) => void,
    timeoutMs: number,
    signal: AbortSignal | undefined
  ): () => void {
    let settled = false;
    const timer = setTimeout(() => {
      finish(() => {
        const tail = this.stderrTail.join("\n");
        const suffix = tail.length > 0 ? ` Last stderr:\n${tail}` : "";
        rejectCall(new Error(`Timeout after ${timeoutMs}ms waiting for Haft MCP response.${suffix}`));
      });
    }, timeoutMs);

    const onAbort = () => {
      finish(() => rejectCall(new Error("Haft MCP request aborted.")));
    };

    const finish = (handler: () => void) => {
      if (settled) {
        return;
      }

      settled = true;
      clearTimeout(timer);
      this.pending.delete(id);
      signal?.removeEventListener("abort", onAbort);
      handler();
    };

    if (signal?.aborted === true) {
      finish(() => rejectCall(new Error("Haft MCP request aborted.")));
      return () => undefined;
    }

    signal?.addEventListener("abort", onAbort, { once: true });

    this.pending.set(id, {
      resolve: (value) => finish(() => resolveCall(value)),
      reject: (error) => finish(() => rejectCall(error))
    });

    return () => finish(() => undefined);
  }

  private acceptStdout(chunk: string): void {
    this.buffer += chunk;
    const lines = this.buffer.split("\n");
    this.buffer = lines.pop() ?? "";
    lines
      .map((line) => line.trim())
      .filter((line) => line.length > 0)
      .forEach((line) => this.dispatchLine(line));
  }

  private acceptStderr(chunk: string): void {
    chunk
      .split("\n")
      .map((line) => line.trim())
      .filter((line) => line.length > 0)
      .forEach((line) => this.pushStderr(line));
  }

  private pushStderr(line: string): void {
    this.stderrTail.push(line);
    this.stderrTail = this.stderrTail.slice(-8);
  }

  private dispatchLine(line: string): void {
    const response = parseResponse(line);
    if (response === undefined) {
      return;
    }

    const id = response.id;
    if (id === undefined) {
      return;
    }

    const pending = this.pending.get(id);
    if (pending === undefined) {
      return;
    }

    if (response.error !== undefined) {
      const message = `${response.error.code}: ${response.error.message}`;
      pending.reject(new Error(message));
      return;
    }

    pending.resolve(response.result);
  }

  private handleExit(code: number | null, cause: NodeJS.Signals | null): void {
    const codeText = code === null ? "unknown" : String(code);
    const causeText = cause === null ? "none" : cause;
    this.process = undefined;
    this.startPromise = undefined;
    this.failAll(new Error(`Haft MCP bridge exited. code=${codeText} signal=${causeText}`));
  }

  private failAll(error: Error): void {
    const calls = [...this.pending.values()];
    this.pending.clear();
    calls.forEach((call) => call.reject(error));
  }

  private requireProcess(): ChildProcessWithoutNullStreams {
    const child = this.process;
    if (child !== undefined) {
      return child;
    }

    throw new Error("Haft MCP bridge is not running.");
  }

  private nextRequestID(): number {
    this.nextID += 1;
    return this.nextID;
  }
}

export function resolveHaftCommand(projectRoot: string): string {
  const configured = process.env.HAFT_BIN;
  if (configured !== undefined && configured.trim().length > 0) {
    return configured.trim();
  }

  const localBinary = join(projectRoot, "haft");
  if (existsSync(localBinary)) {
    return localBinary;
  }

  return "haft";
}

export function parseResponse(line: string): JsonRpcResponse | undefined {
  try {
    return JSON.parse(line) as JsonRpcResponse;
  } catch {
    return undefined;
  }
}

export function toolNames(result: unknown): string[] {
  const envelope = result as { tools?: Array<{ name?: string }> };
  const tools = envelope.tools ?? [];
  return tools
    .map((tool) => tool.name)
    .filter((name): name is string => typeof name === "string");
}

export function toolText(result: unknown): string {
  const envelope = result as ToolCallResult;
  const content = envelope.content ?? [];
  const text = content
    .filter((item) => item.type === "text")
    .map((item) => item.text ?? "")
    .filter((item) => item.length > 0)
    .join("\n");

  if (envelope.isError === true) {
    throw new Error(text.length > 0 ? text : "Haft tool returned an error.");
  }

  return text;
}
