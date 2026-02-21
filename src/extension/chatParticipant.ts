import * as vscode from 'vscode';
import { McpClient } from './mcpClient';

const PARTICIPANT_ID = 'quint-code.chat';

const FPF_SYSTEM_PROMPT = `You are Quint Code, an FPF (First Principles Framework) engineering assistant.
You help engineers make structured, evidence-based decisions using the FPF methodology.

When using tools, follow the FPF workflow:
1. quint_init - Initialize project
2. quint_propose - Create hypotheses (L0)
3. quint_verify - Logical verification (L0->L1)
4. quint_test - Empirical validation (L1->L2)
5. quint_audit / quint_audit_tree / quint_calculate_r - Risk analysis
6. quint_decide - Finalize decision (DRR)
7. quint_status - Check phase, quint_check_decay - Check evidence freshness

Be direct and technical. Use the FPF tools when the user's request involves structured reasoning or decision-making.`;

interface CommandMapping {
  mcpTool: string;
  buildArgs: (prompt: string) => Record<string, unknown>;
}

const COMMAND_MAP: Record<string, CommandMapping> = {
  'q0-init': {
    mcpTool: 'quint_init',
    buildArgs: () => ({}),
  },
  'q-status': {
    mcpTool: 'quint_status',
    buildArgs: () => ({}),
  },
  'q1-hypothesize': {
    mcpTool: 'quint_propose',
    buildArgs: (prompt) => {
      const trimmed = prompt.trim();
      if (!trimmed) {
        return {};
      }
      try {
        if (trimmed.startsWith('{')) {
          return JSON.parse(trimmed);
        }
      } catch { /* not JSON, use as content */ }
      return {
        title: trimmed.slice(0, 80),
        content: trimmed,
        scope: 'project',
        kind: 'system',
        rationale: JSON.stringify({
          anomaly: trimmed,
          approach: 'To be determined',
          alternatives_rejected: 'None yet',
        }),
      };
    },
  },
  'q1-add': {
    mcpTool: 'quint_propose',
    buildArgs: (prompt) => {
      const content = prompt.trim();
      return {
        title: content.slice(0, 80),
        content,
        scope: 'project',
        kind: 'system',
        rationale: JSON.stringify({
          anomaly: content,
          approach: 'User hypothesis',
          alternatives_rejected: 'None',
        }),
      };
    },
  },
  'q2-verify': {
    mcpTool: 'quint_verify',
    buildArgs: (prompt) => parseJsonOrWrap(prompt),
  },
  'q3-validate': {
    mcpTool: 'quint_test',
    buildArgs: (prompt) => parseJsonOrWrap(prompt),
  },
  'q4-audit': {
    mcpTool: 'quint_audit',
    buildArgs: (prompt) => parseJsonOrWrap(prompt),
  },
  'q5-decide': {
    mcpTool: 'quint_decide',
    buildArgs: (prompt) => parseJsonOrWrap(prompt),
  },
  'q-tree': {
    mcpTool: 'quint_audit_tree',
    buildArgs: (prompt) => ({ holon_id: prompt.trim() }),
  },
  'q-reff': {
    mcpTool: 'quint_calculate_r',
    buildArgs: (prompt) => ({ holon_id: prompt.trim() }),
  },
  'q-decay': {
    mcpTool: 'quint_check_decay',
    buildArgs: (prompt) => parseJsonOrWrap(prompt),
  },
  'q-context': {
    mcpTool: 'quint_record_context',
    buildArgs: (prompt) => parseJsonOrWrap(prompt),
  },
  'q-query': {
    mcpTool: 'quint_audit_tree',
    buildArgs: (prompt) => ({ holon_id: prompt.trim() }),
  },
};

function parseJsonOrWrap(input: string): Record<string, unknown> {
  const trimmed = input.trim();
  if (!trimmed) {
    return {};
  }
  try {
    if (trimmed.startsWith('{')) {
      return JSON.parse(trimmed);
    }
  } catch { /* not JSON */ }
  return { content: trimmed };
}

export function registerChatParticipant(
  context: vscode.ExtensionContext,
  mcpClient: McpClient,
): void {
  const handler: vscode.ChatRequestHandler = async (
    request: vscode.ChatRequest,
    chatContext: vscode.ChatContext,
    response: vscode.ChatResponseStream,
    token: vscode.CancellationToken,
  ): Promise<vscode.ChatResult> => {
    if (request.command === 'help') {
      return handleHelp(response);
    }

    if (request.command && COMMAND_MAP[request.command]) {
      return handleSlashCommand(request, response, mcpClient, token);
    }

    return handleFreeChat(request, chatContext, response, mcpClient, token);
  };

  const participant = vscode.chat.createChatParticipant(PARTICIPANT_ID, handler);
  participant.iconPath = vscode.Uri.joinPath(context.extensionUri, 'media', 'icon.svg');

  context.subscriptions.push(participant);
}

function handleHelp(response: vscode.ChatResponseStream): vscode.ChatResult {
  const lines = [
    '### Quint Code - FPF Commands\n',
    '| # | Command | Phase | Description |',
    '|---|---------|-------|-------------|',
    '| 0 | `/q0-init` | Setup | Initialize FPF project structure |',
    '| 1 | `/q1-hypothesize` | Abduction | Propose a new hypothesis (L0) |',
    '| 1b | `/q1-add` | Abduction | Inject a user hypothesis (L0) |',
    '| 2 | `/q2-verify` | Deduction | Verify hypothesis (L0 -> L1) |',
    '| 3 | `/q3-validate` | Induction | Validate with tests (L1 -> L2) |',
    '| 4 | `/q4-audit` | Bias-Audit | WLNK analysis, congruence check |',
    '| 5 | `/q5-decide` | Decision | Finalize decision (DRR) |',
    '| S | `/q-status` | — | Show current phase and next steps |',
    '| Q | `/q-query` | — | Search the knowledge base |',
    '| T | `/q-tree` | — | Visualize assurance tree |',
    '| R | `/q-reff` | — | Calculate R_eff |',
    '| D | `/q-decay` | — | Check evidence freshness |',
    '| C | `/q-context` | — | Record bounded context |',
    '| ? | `/help` | — | Show this help |',
    '',
    'You can also chat freely — the LLM has access to all FPF tools.',
  ];
  response.markdown(lines.join('\n'));
  return {};
}

async function handleSlashCommand(
  request: vscode.ChatRequest,
  response: vscode.ChatResponseStream,
  mcpClient: McpClient,
  _token: vscode.CancellationToken,
): Promise<vscode.ChatResult> {
  const mapping = COMMAND_MAP[request.command!];

  if (!mcpClient.isRunning()) {
    response.markdown('**Error:** MCP server not running. Install `quint-mcp` and configure the path in settings.');
    return {};
  }

  const args = mapping.buildArgs(request.prompt);

  response.progress(`Calling \`${mapping.mcpTool}\`...`);

  try {
    const result = await mcpClient.callTool(mapping.mcpTool, args);
    const text = result.content.map((c) => c.text).join('\n');

    if (result.isError) {
      response.markdown(`**Error:** ${text}`);
    } else {
      response.markdown(`\`\`\`\n${text}\n\`\`\``);
    }
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    response.markdown(`**Error:** ${msg}`);
  }

  return {};
}

async function handleFreeChat(
  request: vscode.ChatRequest,
  chatContext: vscode.ChatContext,
  response: vscode.ChatResponseStream,
  mcpClient: McpClient,
  token: vscode.CancellationToken,
): Promise<vscode.ChatResult> {
  const model = request.model;

  const mcpTools = mcpClient.isRunning() ? mcpClient.getTools() : [];
  const lmTools: vscode.LanguageModelChatTool[] = mcpTools.map((t) => ({
    name: t.name,
    description: t.description,
    inputSchema: t.inputSchema,
  }));

  const messages: vscode.LanguageModelChatMessage[] = [];

  messages.push(vscode.LanguageModelChatMessage.User(FPF_SYSTEM_PROMPT));

  for (const turn of chatContext.history) {
    if (turn instanceof vscode.ChatRequestTurn) {
      messages.push(vscode.LanguageModelChatMessage.User(turn.prompt));
    } else if (turn instanceof vscode.ChatResponseTurn) {
      const parts: string[] = [];
      for (const part of turn.response) {
        if (part instanceof vscode.ChatResponseMarkdownPart) {
          parts.push(part.value.value);
        }
      }
      if (parts.length > 0) {
        messages.push(vscode.LanguageModelChatMessage.Assistant(parts.join('\n')));
      }
    }
  }

  messages.push(vscode.LanguageModelChatMessage.User(request.prompt));

  const options: vscode.LanguageModelChatRequestOptions = {
    justification: 'Quint Code FPF assistant needs to process your request',
  };

  if (lmTools.length > 0) {
    options.tools = lmTools;
  }

  const maxIterations = 10;

  for (let i = 0; i < maxIterations; i++) {
    if (token.isCancellationRequested) {
      return {};
    }

    const chatResponse = await model.sendRequest(messages, options, token);

    const toolCalls: vscode.LanguageModelToolCallPart[] = [];
    let responseText = '';

    for await (const part of chatResponse.stream) {
      if (part instanceof vscode.LanguageModelTextPart) {
        responseText += part.value;
        response.markdown(part.value);
      } else if (part instanceof vscode.LanguageModelToolCallPart) {
        toolCalls.push(part);
      }
    }

    if (toolCalls.length === 0) {
      return {};
    }

    const assistantContent: Array<vscode.LanguageModelTextPart | vscode.LanguageModelToolCallPart> = [];
    if (responseText) {
      assistantContent.push(new vscode.LanguageModelTextPart(responseText));
    }
    for (const tc of toolCalls) {
      assistantContent.push(new vscode.LanguageModelToolCallPart(tc.callId, tc.name, tc.input));
    }
    messages.push(vscode.LanguageModelChatMessage.Assistant(assistantContent));

    const toolResults: Array<vscode.LanguageModelToolResultPart> = [];

    for (const tc of toolCalls) {
      response.progress(`Running \`${tc.name}\`...`);

      let resultText: string;
      try {
        const result = await mcpClient.callTool(tc.name, tc.input as Record<string, unknown>);
        resultText = result.content.map((c) => c.text).join('\n');
      } catch (err) {
        resultText = `Error: ${err instanceof Error ? err.message : String(err)}`;
      }

      toolResults.push(
        new vscode.LanguageModelToolResultPart(tc.callId, [
          new vscode.LanguageModelTextPart(resultText),
        ]),
      );
    }

    messages.push(vscode.LanguageModelChatMessage.User(toolResults));
  }

  response.markdown('\n\n*Reached maximum tool iterations.*');
  return {};
}
