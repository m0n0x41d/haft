import { ChildProcess, spawn } from 'child_process';
import { EventEmitter } from 'events';
import * as vscode from 'vscode';

export interface McpTool {
  name: string;
  description: string;
  inputSchema: Record<string, unknown>;
}

export interface McpToolResult {
  content: Array<{ type: string; text: string }>;
  isError?: boolean;
}

interface JsonRpcRequest {
  jsonrpc: '2.0';
  method: string;
  params?: unknown;
  id: number;
}

interface JsonRpcResponse {
  jsonrpc: '2.0';
  result?: unknown;
  error?: { code: number; message: string };
  id: number;
}

export class McpClient extends EventEmitter {
  private process: ChildProcess | null = null;
  private requestId = 0;
  private pending = new Map<number, {
    resolve: (value: unknown) => void;
    reject: (reason: Error) => void;
  }>();
  private buffer = '';
  private tools: McpTool[] = [];
  private workDir: string;
  private binaryPath: string;

  constructor(workDir: string, binaryPath: string) {
    super();
    this.workDir = workDir;
    this.binaryPath = binaryPath;
  }

  async start(): Promise<void> {
    if (this.process) {
      return;
    }

    this.process = spawn(this.binaryPath, ['serve'], {
      cwd: this.workDir,
      stdio: ['pipe', 'pipe', 'pipe'],
      env: { ...process.env },
    });

    this.process.stdout!.on('data', (data: Buffer) => {
      this.buffer += data.toString();
      this.processBuffer();
    });

    this.process.stderr!.on('data', (data: Buffer) => {
      const msg = data.toString().trim();
      if (msg) {
        this.emit('log', msg);
      }
    });

    this.process.on('exit', (code) => {
      this.emit('exit', code);
      this.process = null;
      for (const [, handler] of this.pending) {
        handler.reject(new Error(`MCP server exited with code ${code}`));
      }
      this.pending.clear();
    });

    this.process.on('error', (err) => {
      this.emit('error', err);
    });

    await this.initialize();
    await this.loadTools();
  }

  stop(): void {
    if (this.process) {
      this.process.kill();
      this.process = null;
    }
  }

  isRunning(): boolean {
    return this.process !== null;
  }

  getTools(): McpTool[] {
    return this.tools;
  }

  async callTool(name: string, args: Record<string, unknown>): Promise<McpToolResult> {
    const result = await this.send('tools/call', { name, arguments: args });
    return result as McpToolResult;
  }

  private async initialize(): Promise<void> {
    await this.send('initialize', {
      protocolVersion: '2024-11-05',
      capabilities: {},
      clientInfo: { name: 'quint-code-vscode', version: '0.1.0' },
    });

    this.sendNotification('notifications/initialized', {});
  }

  private async loadTools(): Promise<void> {
    const result = await this.send('tools/list', {}) as { tools: McpTool[] };
    this.tools = result.tools;
  }

  private send(method: string, params?: unknown): Promise<unknown> {
    return new Promise((resolve, reject) => {
      if (!this.process?.stdin?.writable) {
        reject(new Error('MCP server not running'));
        return;
      }

      const id = ++this.requestId;
      const request: JsonRpcRequest = {
        jsonrpc: '2.0',
        method,
        params,
        id,
      };

      this.pending.set(id, { resolve, reject });

      const data = JSON.stringify(request) + '\n';
      this.process.stdin.write(data, (err) => {
        if (err) {
          this.pending.delete(id);
          reject(err);
        }
      });

      setTimeout(() => {
        if (this.pending.has(id)) {
          this.pending.delete(id);
          reject(new Error(`Request ${method} timed out`));
        }
      }, 30_000);
    });
  }

  private sendNotification(method: string, params?: unknown): void {
    if (!this.process?.stdin?.writable) {
      return;
    }

    const data = JSON.stringify({ jsonrpc: '2.0', method, params }) + '\n';
    this.process.stdin.write(data);
  }

  private processBuffer(): void {
    const lines = this.buffer.split('\n');
    this.buffer = lines.pop() ?? '';

    for (const line of lines) {
      if (!line.trim()) {
        continue;
      }

      try {
        const response: JsonRpcResponse = JSON.parse(line);
        if (response.id !== undefined && this.pending.has(response.id)) {
          const handler = this.pending.get(response.id)!;
          this.pending.delete(response.id);

          if (response.error) {
            handler.reject(new Error(response.error.message));
          } else {
            handler.resolve(response.result);
          }
        }
      } catch {
        this.emit('log', `Failed to parse MCP response: ${line}`);
      }
    }
  }
}

export function findMcpBinary(): string | undefined {
  const config = vscode.workspace.getConfiguration('quintCode');
  const configured = config.get<string>('mcpServerPath');
  if (configured) {
    return configured;
  }

  const { execSync } = require('child_process');
  try {
    const result = execSync('which quint-mcp', { encoding: 'utf-8' }).trim();
    if (result) {
      return result;
    }
  } catch {
    // not on PATH
  }

  return undefined;
}
