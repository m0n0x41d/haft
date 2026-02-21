import * as vscode from 'vscode';
import { McpClient, findMcpBinary } from './mcpClient';
import { registerChatParticipant } from './chatParticipant';

let mcpClient: McpClient | undefined;
let outputChannel: vscode.OutputChannel;

export async function activate(context: vscode.ExtensionContext): Promise<void> {
  outputChannel = vscode.window.createOutputChannel('Quint Code');
  context.subscriptions.push(outputChannel);

  const workDir = vscode.workspace.workspaceFolders?.[0]?.uri.fsPath || process.cwd();

  mcpClient = await startMcpServer(workDir);

  const mcpForParticipant = mcpClient ?? createStubClient();

  registerChatParticipant(context, mcpForParticipant);

  context.subscriptions.push(
    vscode.commands.registerCommand('quint-code.init', async () => {
      if (!mcpForParticipant.isRunning()) {
        vscode.window.showErrorMessage('Quint Code: MCP server not running. Install quint-mcp first.');
        return;
      }
      try {
        const result = await mcpForParticipant.callTool('quint_init', {});
        const text = result.content.map((c) => c.text).join('\n');
        vscode.window.showInformationMessage(`Quint Code: ${text}`);
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        vscode.window.showErrorMessage(`Quint Code: ${msg}`);
      }
    }),
  );

  context.subscriptions.push(
    vscode.commands.registerCommand('quint-code.status', async () => {
      if (!mcpForParticipant.isRunning()) {
        vscode.window.showInformationMessage('Quint Code: MCP server not connected');
        return;
      }
      try {
        const result = await mcpForParticipant.callTool('quint_status', {});
        const text = result.content.map((c) => c.text).join('\n');
        vscode.window.showInformationMessage(`Quint Code phase: ${text}`);
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        vscode.window.showErrorMessage(`Quint Code: ${msg}`);
      }
    }),
  );

  vscode.workspace.onDidChangeConfiguration((e) => {
    if (e.affectsConfiguration('quintCode.mcpServerPath')) {
      restartMcpServer(workDir);
    }
  }, null, context.subscriptions);

  outputChannel.appendLine('Quint Code extension activated');
  if (mcpClient?.isRunning()) {
    outputChannel.appendLine(`MCP server running (${mcpClient.getTools().length} tools)`);
  } else {
    outputChannel.appendLine('MCP server not available. Install quint-mcp to enable FPF tools.');
  }
}

export function deactivate(): void {
  mcpClient?.stop();
}

async function startMcpServer(workDir: string): Promise<McpClient | undefined> {
  const binaryPath = findMcpBinary();
  if (!binaryPath) {
    outputChannel.appendLine('quint-mcp binary not found. FPF tools disabled.');
    outputChannel.appendLine('Install with: go install github.com/m0n0x41d/quint-code/src/mcp@latest');
    return undefined;
  }

  outputChannel.appendLine(`Starting MCP server: ${binaryPath}`);

  const client = new McpClient(workDir, binaryPath);

  client.on('log', (msg: string) => {
    outputChannel.appendLine(`[MCP] ${msg}`);
  });

  client.on('exit', (code: number) => {
    outputChannel.appendLine(`MCP server exited with code ${code}`);
  });

  client.on('error', (err: Error) => {
    outputChannel.appendLine(`MCP server error: ${err.message}`);
  });

  try {
    await client.start();
    return client;
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    outputChannel.appendLine(`Failed to start MCP server: ${msg}`);
    return undefined;
  }
}

async function restartMcpServer(workDir: string): Promise<void> {
  mcpClient?.stop();
  mcpClient = await startMcpServer(workDir);
}

function createStubClient(): McpClient {
  return {
    isRunning: () => false,
    getTools: () => [],
    callTool: () => Promise.reject(new Error('MCP server not running')),
    on: () => {},
    stop: () => {},
    start: () => Promise.resolve(),
  } as unknown as McpClient;
}
