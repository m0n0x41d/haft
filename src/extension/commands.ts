export interface SlashCommand {
  name: string;
  description: string;
  mcpTool: string;
  argsParser: (input: string) => Record<string, unknown>;
}

function parseJsonArg(input: string): Record<string, unknown> {
  const trimmed = input.trim();
  if (!trimmed) {
    return {};
  }
  if (trimmed.startsWith('{')) {
    try {
      return JSON.parse(trimmed);
    } catch {
      return { content: trimmed };
    }
  }
  return { content: trimmed };
}

export const SLASH_COMMANDS: SlashCommand[] = [
  {
    name: '/q0-init',
    description: 'Initialize FPF project structure',
    mcpTool: 'quint_init',
    argsParser: () => ({}),
  },
  {
    name: '/q-status',
    description: 'Show current FPF phase and context',
    mcpTool: 'quint_status',
    argsParser: () => ({}),
  },
  {
    name: '/q1-hypothesize',
    description: 'Propose a new hypothesis (L0)',
    mcpTool: 'quint_propose',
    argsParser: (input: string) => {
      const args = parseJsonArg(input);
      if (args.content && !args.title) {
        return {
          title: String(args.content).slice(0, 80),
          content: args.content,
          scope: args.scope || 'project',
          kind: args.kind || 'system',
          rationale: args.rationale || JSON.stringify({
            anomaly: String(args.content),
            approach: 'To be determined',
            alternatives_rejected: 'None yet',
          }),
        };
      }
      return args;
    },
  },
  {
    name: '/q1-add',
    description: 'Add a user hypothesis (shortcut for /q1-hypothesize)',
    mcpTool: 'quint_propose',
    argsParser: (input: string) => {
      const content = input.trim();
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
  {
    name: '/q2-verify',
    description: 'Verify a hypothesis (L0 -> L1)',
    mcpTool: 'quint_verify',
    argsParser: parseJsonArg,
  },
  {
    name: '/q3-validate',
    description: 'Validate a hypothesis with tests (L1 -> L2)',
    mcpTool: 'quint_test',
    argsParser: parseJsonArg,
  },
  {
    name: '/q4-audit',
    description: 'Audit a hypothesis for risks',
    mcpTool: 'quint_audit',
    argsParser: parseJsonArg,
  },
  {
    name: '/q5-decide',
    description: 'Finalize a decision (DRR)',
    mcpTool: 'quint_decide',
    argsParser: parseJsonArg,
  },
  {
    name: '/q-query',
    description: 'Query the FPF knowledge base',
    mcpTool: 'quint_audit_tree',
    argsParser: (input: string) => ({ holon_id: input.trim() }),
  },
  {
    name: '/q-decay',
    description: 'Check evidence freshness',
    mcpTool: 'quint_check_decay',
    argsParser: parseJsonArg,
  },
  {
    name: '/q-context',
    description: 'Record bounded context (vocabulary + invariants)',
    mcpTool: 'quint_record_context',
    argsParser: parseJsonArg,
  },
  {
    name: '/q-tree',
    description: 'Visualize assurance tree for a holon',
    mcpTool: 'quint_audit_tree',
    argsParser: (input: string) => ({ holon_id: input.trim() }),
  },
  {
    name: '/q-reff',
    description: 'Calculate effective reliability (R_eff) for a holon',
    mcpTool: 'quint_calculate_r',
    argsParser: (input: string) => ({ holon_id: input.trim() }),
  },
  {
    name: '/help',
    description: 'Show available commands',
    mcpTool: '',
    argsParser: () => ({}),
  },
];

export function findCommand(input: string): { command: SlashCommand; args: string } | undefined {
  const trimmed = input.trim();
  if (!trimmed.startsWith('/')) {
    return undefined;
  }

  const spaceIndex = trimmed.indexOf(' ');
  const commandName = spaceIndex >= 0 ? trimmed.slice(0, spaceIndex) : trimmed;
  const args = spaceIndex >= 0 ? trimmed.slice(spaceIndex + 1) : '';

  const command = SLASH_COMMANDS.find((c) => c.name === commandName);
  if (!command) {
    return undefined;
  }

  return { command, args };
}

export function getHelpText(): string {
  const lines = ['**Quint Code - Available Commands**\n'];
  for (const cmd of SLASH_COMMANDS) {
    lines.push(`\`${cmd.name}\` - ${cmd.description}`);
  }
  lines.push('');
  lines.push('Type any message without a `/` prefix to chat with the LLM.');
  lines.push('The LLM has access to all FPF tools and will use them automatically.');
  return lines.join('\n');
}
