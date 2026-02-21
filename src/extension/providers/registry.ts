import * as vscode from 'vscode';
import type { ChatProvider } from './types';
import { AnthropicProvider } from './anthropic';
import { OpenAIProvider } from './openai';
import { OllamaProvider } from './ollama';
import { VSCodeLMProvider } from './vscodeProvider';

export class ProviderRegistry {
  private providers = new Map<string, ChatProvider>();

  constructor() {
    this.register(new AnthropicProvider());
    this.register(new OpenAIProvider());
    this.register(new OllamaProvider());
    this.register(new VSCodeLMProvider());
  }

  private register(provider: ChatProvider): void {
    this.providers.set(provider.id, provider);
  }

  get(id: string): ChatProvider | undefined {
    return this.providers.get(id);
  }

  getActive(): ChatProvider {
    const config = vscode.workspace.getConfiguration('quintCode');
    const providerId = config.get<string>('provider') || 'anthropic';
    const provider = this.providers.get(providerId);
    if (!provider) {
      throw new Error(`Unknown provider: ${providerId}`);
    }
    return provider;
  }

  listAll(): ChatProvider[] {
    return Array.from(this.providers.values());
  }

  listConfigured(): ChatProvider[] {
    return this.listAll().filter((p) => p.isConfigured());
  }
}
