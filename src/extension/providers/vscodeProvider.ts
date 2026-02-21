import * as vscode from 'vscode';
import type { ChatMessage, ChatProvider, StreamChunk, ToolDefinition } from './types';

export class VSCodeLMProvider implements ChatProvider {
  readonly id = 'vscode-lm';
  readonly displayName = 'VS Code Language Model';

  isConfigured(): boolean {
    return typeof vscode.lm !== 'undefined';
  }

  async chat(
    messages: ChatMessage[],
    _tools: ToolDefinition[],
    onChunk: (chunk: StreamChunk) => void,
    signal?: AbortSignal,
  ): Promise<ChatMessage> {
    if (typeof vscode.lm === 'undefined') {
      throw new Error('VS Code Language Model API not available. Install GitHub Copilot or a compatible extension.');
    }

    const models = await vscode.lm.selectChatModels();
    if (models.length === 0) {
      throw new Error('No language models available. Install GitHub Copilot or a compatible extension.');
    }

    const model = models[0];

    const vscodeMessages: vscode.LanguageModelChatMessage[] = [];
    for (const msg of messages) {
      if (msg.role === 'user' || msg.role === 'system') {
        vscodeMessages.push(vscode.LanguageModelChatMessage.User(msg.content));
      } else if (msg.role === 'assistant') {
        vscodeMessages.push(vscode.LanguageModelChatMessage.Assistant(msg.content));
      }
    }

    const token = signal
      ? new vscode.CancellationTokenSource().token
      : new vscode.CancellationTokenSource().token;

    const response = await model.sendRequest(vscodeMessages, {}, token);

    let text = '';
    for await (const chunk of response.text) {
      text += chunk;
      onChunk({ type: 'text', text: chunk });
    }

    onChunk({ type: 'done' });

    return { role: 'assistant', content: text };
  }
}
