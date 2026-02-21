import * as vscode from 'vscode';
import type { ChatMessage, ChatProvider, StreamChunk, ToolCall, ToolDefinition } from './types';

export class OpenAIProvider implements ChatProvider {
  readonly id = 'openai';
  readonly displayName = 'OpenAI';

  isConfigured(): boolean {
    return !!this.getApiKey();
  }

  private getApiKey(): string {
    const config = vscode.workspace.getConfiguration('quintCode');
    return config.get<string>('openai.apiKey') || process.env.OPENAI_API_KEY || '';
  }

  private getModel(): string {
    const config = vscode.workspace.getConfiguration('quintCode');
    return config.get<string>('openai.model') || 'gpt-4o';
  }

  private getBaseUrl(): string | undefined {
    const config = vscode.workspace.getConfiguration('quintCode');
    const url = config.get<string>('openai.baseUrl');
    return url || undefined;
  }

  async chat(
    messages: ChatMessage[],
    tools: ToolDefinition[],
    onChunk: (chunk: StreamChunk) => void,
    _signal?: AbortSignal,
  ): Promise<ChatMessage> {
    const OpenAI = (await import('openai')).default;
    const client = new OpenAI({
      apiKey: this.getApiKey(),
      baseURL: this.getBaseUrl(),
    });

    const openaiMessages = this.convertMessages(messages);
    const openaiTools = tools.length > 0 ? this.convertTools(tools) : undefined;

    const params: Record<string, unknown> = {
      model: this.getModel(),
      messages: openaiMessages,
    };

    if (openaiTools && openaiTools.length > 0) {
      params.tools = openaiTools;
    }

    const response = await client.chat.completions.create(
      params as Parameters<typeof client.chat.completions.create>[0],
    );

    const choice = (response as { choices: Array<{ message: { content?: string; tool_calls?: Array<{ id: string; function: { name: string; arguments: string } }> } }> }).choices[0];
    const msg = choice.message;

    const text = msg.content || '';
    if (text) {
      onChunk({ type: 'text', text });
    }

    const toolCalls: ToolCall[] = [];
    if (msg.tool_calls) {
      for (const tc of msg.tool_calls) {
        const toolCall: ToolCall = {
          id: tc.id,
          name: tc.function.name,
          arguments: JSON.parse(tc.function.arguments),
        };
        toolCalls.push(toolCall);
        onChunk({ type: 'tool_call', toolCall });
      }
    }

    onChunk({ type: 'done' });

    const result: ChatMessage = { role: 'assistant', content: text };
    if (toolCalls.length > 0) {
      result.toolCalls = toolCalls;
    }
    return result;
  }

  private convertMessages(messages: ChatMessage[]): unknown[] {
    return messages.map((msg) => {
      if (msg.role === 'tool') {
        return {
          role: 'tool',
          tool_call_id: msg.toolCallId,
          content: msg.content,
        };
      }

      if (msg.role === 'assistant' && msg.toolCalls?.length) {
        return {
          role: 'assistant',
          content: msg.content || null,
          tool_calls: msg.toolCalls.map((tc) => ({
            id: tc.id,
            type: 'function',
            function: {
              name: tc.name,
              arguments: JSON.stringify(tc.arguments),
            },
          })),
        };
      }

      return { role: msg.role, content: msg.content };
    });
  }

  private convertTools(tools: ToolDefinition[]): unknown[] {
    return tools.map((t) => ({
      type: 'function',
      function: {
        name: t.name,
        description: t.description,
        parameters: t.inputSchema,
      },
    }));
  }
}
