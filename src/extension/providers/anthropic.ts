import * as vscode from 'vscode';
import type { ChatMessage, ChatProvider, StreamChunk, ToolCall, ToolDefinition } from './types';

export class AnthropicProvider implements ChatProvider {
  readonly id = 'anthropic';
  readonly displayName = 'Anthropic Claude';

  isConfigured(): boolean {
    const key = this.getApiKey();
    return !!key;
  }

  private getApiKey(): string {
    const config = vscode.workspace.getConfiguration('quintCode');
    return config.get<string>('anthropic.apiKey') || process.env.ANTHROPIC_API_KEY || '';
  }

  private getModel(): string {
    const config = vscode.workspace.getConfiguration('quintCode');
    return config.get<string>('anthropic.model') || 'claude-sonnet-4-20250514';
  }

  async chat(
    messages: ChatMessage[],
    tools: ToolDefinition[],
    onChunk: (chunk: StreamChunk) => void,
    signal?: AbortSignal,
  ): Promise<ChatMessage> {
    const Anthropic = (await import('@anthropic-ai/sdk')).default;
    const client = new Anthropic({ apiKey: this.getApiKey() });

    const systemMessages = messages.filter((m) => m.role === 'system');
    const nonSystemMessages = messages.filter((m) => m.role !== 'system');

    const systemPrompt = systemMessages.map((m) => m.content).join('\n\n');

    const anthropicMessages = this.convertMessages(nonSystemMessages);
    const anthropicTools = this.convertTools(tools);

    const params: Record<string, unknown> = {
      model: this.getModel(),
      max_tokens: 8192,
      messages: anthropicMessages,
    };

    if (systemPrompt) {
      params.system = systemPrompt;
    }

    if (anthropicTools.length > 0) {
      params.tools = anthropicTools;
    }

    const response = await client.messages.create(params as Parameters<typeof client.messages.create>[0]);

    let text = '';
    const toolCalls: ToolCall[] = [];

    for (const block of (response as { content: Array<{ type: string; text?: string; id?: string; name?: string; input?: Record<string, unknown> }> }).content) {
      if (block.type === 'text' && block.text) {
        text += block.text;
        onChunk({ type: 'text', text: block.text });
      } else if (block.type === 'tool_use') {
        const tc: ToolCall = {
          id: block.id!,
          name: block.name!,
          arguments: block.input as Record<string, unknown>,
        };
        toolCalls.push(tc);
        onChunk({ type: 'tool_call', toolCall: tc });
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
    const result: unknown[] = [];

    for (const msg of messages) {
      if (msg.role === 'tool') {
        result.push({
          role: 'user',
          content: [
            {
              type: 'tool_result',
              tool_use_id: msg.toolCallId,
              content: msg.content,
            },
          ],
        });
      } else if (msg.role === 'assistant' && msg.toolCalls?.length) {
        const content: unknown[] = [];
        if (msg.content) {
          content.push({ type: 'text', text: msg.content });
        }
        for (const tc of msg.toolCalls) {
          content.push({
            type: 'tool_use',
            id: tc.id,
            name: tc.name,
            input: tc.arguments,
          });
        }
        result.push({ role: 'assistant', content });
      } else {
        result.push({ role: msg.role, content: msg.content });
      }
    }

    return result;
  }

  private convertTools(tools: ToolDefinition[]): unknown[] {
    return tools.map((t) => ({
      name: t.name,
      description: t.description,
      input_schema: t.inputSchema,
    }));
  }
}
