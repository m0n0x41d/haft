import * as vscode from 'vscode';
import { McpClient } from './mcpClient';
import { ProviderRegistry } from './providers/registry';
import { findCommand, getHelpText, SLASH_COMMANDS } from './commands';
import type { ChatMessage, ToolDefinition } from './providers/types';

const FPF_SYSTEM_PROMPT = `You are Quint Code, an FPF (First Principles Framework) engineering assistant.
You help engineers make structured, evidence-based decisions using the FPF methodology.

You have access to FPF tools for managing hypotheses, verification, validation, and decisions.
Use them when the user's request involves structured reasoning, decision-making, or knowledge management.

When using tools:
- quint_init: Initialize a new FPF project
- quint_propose: Create a new hypothesis (L0)
- quint_verify: Verify a hypothesis logically (L0 -> L1)
- quint_test: Validate with empirical evidence (L1 -> L2)
- quint_audit: Audit risks for a hypothesis
- quint_decide: Finalize a decision (DRR)
- quint_status: Check current FPF phase
- quint_audit_tree: Visualize assurance tree
- quint_calculate_r: Calculate reliability score
- quint_check_decay: Check evidence freshness

Be direct and technical. Skip validation theater.`;

export class ChatPanel implements vscode.WebviewViewProvider {
  private view?: vscode.WebviewView;
  private history: ChatMessage[] = [];
  private abortController?: AbortController;

  constructor(
    private readonly extensionUri: vscode.Uri,
    private readonly mcpClient: McpClient,
    private readonly providers: ProviderRegistry,
  ) {}

  resolveWebviewView(
    webviewView: vscode.WebviewView,
    _context: vscode.WebviewViewResolveContext,
    _token: vscode.CancellationToken,
  ): void {
    this.view = webviewView;

    webviewView.webview.options = {
      enableScripts: true,
      localResourceRoots: [vscode.Uri.joinPath(this.extensionUri, 'media')],
    };

    webviewView.webview.html = this.getHtml(webviewView.webview);

    webviewView.webview.onDidReceiveMessage(async (msg) => {
      switch (msg.type) {
        case 'send':
          await this.handleMessage(msg.text);
          break;
        case 'abort':
          this.abortController?.abort();
          break;
        case 'selectProvider':
          await this.selectProvider();
          break;
        case 'ready':
          this.postMessage({
            type: 'providerInfo',
            provider: this.providers.getActive().displayName,
            mcpConnected: this.mcpClient.isRunning(),
          });
          break;
      }
    });
  }

  private async handleMessage(text: string): Promise<void> {
    const parsed = findCommand(text);

    if (parsed) {
      if (parsed.command.name === '/help') {
        this.postMessage({ type: 'assistant', text: getHelpText() });
        return;
      }

      this.postMessage({ type: 'user', text });
      await this.handleSlashCommand(parsed.command.mcpTool, parsed.command.argsParser(parsed.args));
      return;
    }

    this.postMessage({ type: 'user', text });
    await this.handleChat(text);
  }

  private async handleSlashCommand(toolName: string, args: Record<string, unknown>): Promise<void> {
    if (!this.mcpClient.isRunning()) {
      this.postMessage({
        type: 'assistant',
        text: '**Error:** MCP server not running. Check that `quint-mcp` is installed and configured.',
      });
      return;
    }

    this.postMessage({ type: 'thinking', text: `Calling \`${toolName}\`...` });

    try {
      const result = await this.mcpClient.callTool(toolName, args);
      const text = result.content.map((c) => c.text).join('\n');

      if (result.isError) {
        this.postMessage({ type: 'assistant', text: `**Error:** ${text}` });
      } else {
        this.postMessage({ type: 'assistant', text: `\`\`\`\n${text}\n\`\`\`` });
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      this.postMessage({ type: 'assistant', text: `**Error:** ${message}` });
    }
  }

  private async handleChat(text: string): Promise<void> {
    const provider = this.providers.getActive();

    if (!provider.isConfigured()) {
      this.postMessage({
        type: 'assistant',
        text: `**Error:** Provider "${provider.displayName}" is not configured. Set the API key in settings or select a different provider.`,
      });
      return;
    }

    const config = vscode.workspace.getConfiguration('quintCode');
    const extraSystemPrompt = config.get<string>('systemPrompt') || '';
    const systemPrompt = extraSystemPrompt
      ? `${FPF_SYSTEM_PROMPT}\n\n${extraSystemPrompt}`
      : FPF_SYSTEM_PROMPT;

    this.history.push({ role: 'user', content: text });

    const tools: ToolDefinition[] = this.mcpClient.isRunning()
      ? this.mcpClient.getTools().map((t) => ({
          name: t.name,
          description: t.description,
          inputSchema: t.inputSchema,
        }))
      : [];

    const messages: ChatMessage[] = [
      { role: 'system', content: systemPrompt },
      ...this.history,
    ];

    this.abortController = new AbortController();
    this.postMessage({ type: 'thinking', text: 'Thinking...' });

    try {
      await this.runToolLoop(messages, tools, provider);
    } catch (err) {
      if ((err as Error).name === 'AbortError') {
        this.postMessage({ type: 'assistant', text: '*Cancelled*' });
      } else {
        const message = err instanceof Error ? err.message : String(err);
        this.postMessage({ type: 'assistant', text: `**Error:** ${message}` });
      }
    } finally {
      this.abortController = undefined;
      this.postMessage({ type: 'doneThinking' });
    }
  }

  private async runToolLoop(
    messages: ChatMessage[],
    tools: ToolDefinition[],
    provider: ReturnType<ProviderRegistry['getActive']>,
  ): Promise<void> {
    const maxIterations = 10;

    for (let i = 0; i < maxIterations; i++) {
      let fullText = '';

      const response = await provider.chat(
        messages,
        tools,
        (chunk) => {
          if (chunk.type === 'text' && chunk.text) {
            fullText += chunk.text;
            this.postMessage({ type: 'stream', text: chunk.text });
          } else if (chunk.type === 'tool_call' && chunk.toolCall) {
            this.postMessage({
              type: 'toolCall',
              name: chunk.toolCall.name,
              args: chunk.toolCall.arguments,
            });
          }
        },
        this.abortController?.signal,
      );

      this.history.push(response);
      messages.push(response);

      if (!response.toolCalls?.length) {
        this.postMessage({ type: 'streamEnd' });
        return;
      }

      for (const tc of response.toolCalls) {
        if (!this.mcpClient.isRunning()) {
          const toolMsg: ChatMessage = {
            role: 'tool',
            content: 'MCP server not running',
            toolCallId: tc.id,
          };
          this.history.push(toolMsg);
          messages.push(toolMsg);
          continue;
        }

        try {
          const result = await this.mcpClient.callTool(tc.name, tc.arguments);
          const resultText = result.content.map((c) => c.text).join('\n');

          this.postMessage({
            type: 'toolResult',
            name: tc.name,
            result: resultText,
            isError: result.isError,
          });

          const toolMsg: ChatMessage = {
            role: 'tool',
            content: resultText,
            toolCallId: tc.id,
          };
          this.history.push(toolMsg);
          messages.push(toolMsg);
        } catch (err) {
          const errMsg = err instanceof Error ? err.message : String(err);
          const toolMsg: ChatMessage = {
            role: 'tool',
            content: `Error: ${errMsg}`,
            toolCallId: tc.id,
          };
          this.history.push(toolMsg);
          messages.push(toolMsg);
        }
      }

      this.postMessage({ type: 'thinking', text: 'Processing tool results...' });
    }

    this.postMessage({
      type: 'assistant',
      text: '*Reached maximum tool call iterations. Please continue the conversation.*',
    });
  }

  async selectProvider(): Promise<void> {
    const providers = this.providers.listAll();
    const items = providers.map((p) => ({
      label: p.displayName,
      description: p.isConfigured() ? 'Configured' : 'Not configured',
      id: p.id,
    }));

    const selected = await vscode.window.showQuickPick(items, {
      placeHolder: 'Select LLM provider',
    });

    if (selected) {
      const config = vscode.workspace.getConfiguration('quintCode');
      await config.update('provider', selected.id, vscode.ConfigurationTarget.Global);
      this.postMessage({
        type: 'providerInfo',
        provider: selected.label,
        mcpConnected: this.mcpClient.isRunning(),
      });
    }
  }

  clearHistory(): void {
    this.history = [];
    this.postMessage({ type: 'clear' });
  }

  private postMessage(msg: unknown): void {
    this.view?.webview.postMessage(msg);
  }

  private getHtml(webview: vscode.Webview): string {
    const cssUri = webview.asWebviewUri(
      vscode.Uri.joinPath(this.extensionUri, 'media', 'chat.css'),
    );
    const jsUri = webview.asWebviewUri(
      vscode.Uri.joinPath(this.extensionUri, 'media', 'chat.js'),
    );

    const commandsJson = JSON.stringify(
      SLASH_COMMANDS.map((c) => ({ name: c.name, description: c.description })),
    );

    return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <meta http-equiv="Content-Security-Policy"
    content="default-src 'none'; style-src ${webview.cspSource} 'unsafe-inline'; script-src ${webview.cspSource};">
  <link rel="stylesheet" href="${cssUri}">
  <title>Quint Code</title>
</head>
<body>
  <div id="app">
    <div id="header">
      <span id="provider-name">Loading...</span>
      <span id="mcp-status" class="status-dot"></span>
      <button id="provider-btn" title="Change provider">&#9881;</button>
      <button id="clear-btn" title="Clear chat">&#10005;</button>
    </div>
    <div id="messages"></div>
    <div id="autocomplete" class="hidden"></div>
    <div id="input-area">
      <textarea id="input" placeholder="Type a message or /command..." rows="1"></textarea>
      <button id="send-btn" title="Send">&#9654;</button>
      <button id="abort-btn" title="Cancel" class="hidden">&#9632;</button>
    </div>
  </div>
  <script>const COMMANDS = ${commandsJson};</script>
  <script src="${jsUri}"></script>
</body>
</html>`;
  }
}
