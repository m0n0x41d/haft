import * as vscode from 'vscode';
import type { ChatMessage, ChatProvider, StreamChunk, ToolCall, ToolDefinition } from './types';

export class OllamaProvider implements ChatProvider {
  readonly id = 'ollama';
  readonly displayName = 'Ollama (Local)';

  isConfigured(): boolean {
    return true;
  }

  private getBaseUrl(): string {
    const config = vscode.workspace.getConfiguration('quintCode');
    return config.get<string>('ollama.baseUrl') || 'http://localhost:11434';
  }

  private getModel(): string {
    const config = vscode.workspace.getConfiguration('quintCode');
    return config.get<string>('ollama.model') || 'llama3.1';
  }

  async chat(
    messages: ChatMessage[],
    tools: ToolDefinition[],
    onChunk: (chunk: StreamChunk) => void,
    _signal?: AbortSignal,
  ): Promise<ChatMessage> {
    const OpenAI = (await import('openai')).default;
    const client = new OpenAI({
      apiKey: 'ollama',
      baseURL: `${this.getBaseUrl()}/v1`,
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

    try {
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
          let args: Record<string, unknown>;
          try {
            args = JSON.parse(tc.function.arguments);
          } catch {
            args = {};
          }
          const toolCall: ToolCall = {
            id: tc.id,
            name: tc.function.name,
            arguments: args,
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
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      onChunk({ type: 'error', error: `Ollama error: ${message}. Is Ollama running at ${this.getBaseUrl()}?` });
      throw err;
    }
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
