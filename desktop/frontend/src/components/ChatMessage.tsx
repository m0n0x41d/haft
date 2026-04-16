import type { ReactNode } from "react";

import type { ChatBlock, ChatEntry } from "../lib/api";

type MarkdownBlock =
  | { kind: "heading"; key: string; level: number; content: string }
  | { kind: "paragraph"; key: string; content: string }
  | { kind: "blockquote"; key: string; content: string }
  | { kind: "unordered_list"; key: string; items: string[] }
  | { kind: "ordered_list"; key: string; items: string[]; start: number }
  | { kind: "code"; key: string; language: string; content: string };

type InlineMatch = {
  captures: string[];
  fullText: string;
  index: number;
  kind: "code" | "emphasis" | "link" | "strong";
};

const INLINE_PATTERNS = [
  {
    kind: "link" as const,
    regex: /\[([^\]]+)\]\(((?:https?:\/\/|mailto:)[^)]+)\)/,
  },
  {
    kind: "code" as const,
    regex: /`([^`\n]+)`/,
  },
  {
    kind: "strong" as const,
    regex: /\*\*([^*]+)\*\*/,
  },
  {
    kind: "emphasis" as const,
    regex: /\*([^*]+)\*/,
  },
];

interface ChatMessageProps {
  block: ChatBlock;
  toolResults?: ChatBlock[];
  groupedTools?: ChatEntry[];
  toolCount?: number;
  thinkingCount?: number;
}

export function ChatMessage({
  block,
  toolResults = [],
  groupedTools,
  toolCount,
  thinkingCount,
}: ChatMessageProps) {
  if (block.type === "tool_group" && groupedTools) {
    return (
      <ToolGroupChip
        groupedTools={groupedTools}
        toolCount={toolCount ?? 0}
        thinkingCount={thinkingCount ?? 0}
      />
    );
  }

  if (block.type === "tool_use") {
    return <ToolUseCard block={block} toolResults={toolResults} />;
  }

  if (block.type === "tool_result") {
    return null;
  }

  if (block.type === "thinking") {
    return <ThinkingMessage block={block} />;
  }

  return <TextMessage block={block} />;
}

function TextMessage({ block }: { block: ChatBlock }) {
  const markdown = firstNonEmpty(
    block.text,
    block.output,
    block.input,
    block.name,
  );
  const isUser = block.role === "user";
  const isSystem = block.role === "system";
  const isError = block.is_error === true;
  const bubbleClassName = isUser
    ? "bg-accent/10"
    : isError
      ? "border border-danger/20 bg-danger/5"
    : isSystem
      ? "border border-warning/20 bg-warning/10"
      : "";

  if (markdown.trim() === "") {
    return null;
  }

  return (
    <div className={`flex ${isUser ? "justify-end" : "justify-start"}`}>
      <article
        className={`max-w-[85%] rounded-2xl px-4 py-3 ${bubbleClassName} ${
          isUser ? "rounded-tr-sm" : "rounded-tl-sm"
        }`}
      >
        <RoleLabel role={block.role} />
        <MarkdownText markdown={markdown} />
      </article>
    </div>
  );
}

function ThinkingMessage({ block }: { block: ChatBlock }) {
  const markdown = block.text ?? "";

  if (markdown.trim() === "") {
    return null;
  }

  return (
    <details className="group">
      <summary className="flex cursor-pointer list-none items-center gap-1.5 text-xs text-text-muted hover:text-text-secondary">
        <ChevronIcon />
        <span>1 thought</span>
      </summary>
      <div className="ml-5 mt-1.5 max-w-[85%] rounded-lg border border-border/40 bg-surface-1/40 px-3 py-2">
        <MarkdownText markdown={markdown} />
      </div>
    </details>
  );
}

function ToolUseCard({
  block,
  toolResults,
}: {
  block: ChatBlock;
  toolResults: ChatBlock[];
}) {
  const title = firstNonEmpty(block.name, "Tool call");
  const input = block.input ?? "";
  const inputLabel = input === ""
    ? ""
    : looksLikeStructuredData(input)
      ? "Arguments"
      : "Input";

  return (
    <div className="flex justify-start">
      <article className="max-w-[85%] rounded-2xl rounded-tl-sm border border-border bg-surface-1 px-4 py-3">
        <header className="flex flex-wrap items-center gap-2">
          <span className="text-xs uppercase tracking-[0.18em] text-text-muted">
            Tool
          </span>
          <span className="rounded-full border border-border bg-surface-0 px-2 py-0.5 font-mono text-xs text-text-primary">
            {title}
          </span>
          {block.call_id && (
            <span className="font-mono text-[11px] text-text-muted">
              {block.call_id}
            </span>
          )}
        </header>

        {input !== "" && (
          <ValuePanel
            className="mt-3"
            label={inputLabel}
            value={input}
          />
        )}

        {toolResults.length > 0 && (
          <div className="mt-3 space-y-3 border-t border-border pt-3">
            {toolResults.map((toolResult) => (
              <ToolResultCard
                key={toolResult.id || `${block.id}-${toolResult.call_id || "result"}`}
                block={toolResult}
              />
            ))}
          </div>
        )}
      </article>
    </div>
  );
}

function ToolGroupChip({
  groupedTools,
  toolCount,
  thinkingCount,
}: {
  groupedTools: ChatEntry[];
  toolCount: number;
  thinkingCount: number;
}) {
  const parts: string[] = [];

  if (toolCount > 0) {
    parts.push(`${toolCount} tool${toolCount > 1 ? "s" : ""}`);
  }

  if (thinkingCount > 0) {
    parts.push(`${thinkingCount} thought${thinkingCount > 1 ? "s" : ""}`);
  }

  const label = parts.length > 0
    ? `Show more: ${parts.join(", ")}`
    : `Show more: ${groupedTools.length} items`;

  const toolEntries = groupedTools.filter((e) => e.block.type === "tool_use");

  return (
    <details className="group">
      <summary className="flex cursor-pointer list-none items-center gap-1.5 text-xs text-text-muted hover:text-text-secondary">
        <ChevronIcon />
        <span>{label}</span>
      </summary>
      <div className="ml-5 mt-1.5 space-y-1">
        {toolEntries.map((entry) => (
          <div
            key={entry.block.id || `group-item-${entry.block.call_id}`}
            className="flex items-center gap-2 text-xs text-text-muted"
          >
            <TerminalIcon />
            <span className="font-mono">
              {formatToolSummary(entry.block.name, entry.block.input)}
            </span>
          </div>
        ))}
      </div>
    </details>
  );
}

function ChevronIcon() {
  return (
    <svg
      width="14"
      height="14"
      viewBox="0 0 14 14"
      fill="none"
      className="shrink-0 transition-transform group-open:rotate-90"
    >
      <path d="M5 3.5L8.5 7 5 10.5" stroke="currentColor" strokeWidth="1.3" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

function TerminalIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 14 14" fill="none" className="shrink-0">
      <rect x="1.5" y="2" width="11" height="10" rx="1.5" stroke="currentColor" strokeWidth="1.1" />
      <path d="M4 5.5L5.5 7 4 8.5" stroke="currentColor" strokeWidth="1.1" strokeLinecap="round" strokeLinejoin="round" />
      <path d="M7 8.5h3" stroke="currentColor" strokeWidth="1.1" strokeLinecap="round" />
    </svg>
  );
}

function formatToolSummary(name?: string, input?: string): string {
  const toolName = name ?? "tool";

  if (!input) {
    return toolName;
  }

  const flat = input.trim().replace(/\n/g, " ");
  const maxLength = 80;

  if (flat.length <= maxLength) {
    return `${toolName} ${flat}`;
  }

  return `${toolName} ${flat.slice(0, maxLength)}...`;
}

function ToolResultCard({ block }: { block: ChatBlock }) {
  const label = block.is_error ? "Tool error" : "Tool result";
  const toneClassName = block.is_error
    ? "border-danger/20 bg-danger/5"
    : "border-border bg-surface-0";
  const value = firstNonEmpty(block.output, block.text, "No output");

  return (
    <section className={`rounded-xl border px-3 py-3 ${toneClassName}`}>
      <div className="flex flex-wrap items-center gap-2">
        <span
          className={`text-xs uppercase tracking-[0.18em] ${
            block.is_error ? "text-danger" : "text-text-muted"
          }`}
        >
          {label}
        </span>
        {block.name && (
          <span className="font-mono text-[11px] text-text-muted">
            {block.name}
          </span>
        )}
      </div>

      <ValuePanel className="mt-2" label="" value={value} />
    </section>
  );
}

function ValuePanel({
  className,
  label,
  value,
}: {
  className?: string;
  label: string;
  value: string;
}) {
  const formattedValue = formatStructuredValue(value);

  return (
    <div className={className}>
      {label !== "" && (
        <p className="mb-2 text-[11px] uppercase tracking-[0.18em] text-text-muted">
          {label}
        </p>
      )}
      <pre className="overflow-x-auto whitespace-pre-wrap rounded-xl border border-border/80 bg-surface-0/80 px-3 py-3 font-mono text-xs leading-6 text-text-secondary">
        <code>{formattedValue}</code>
      </pre>
    </div>
  );
}

function RoleLabel({ role }: { role?: string }) {
  if (role !== "system") {
    return null;
  }

  return (
    <p className="mb-2 text-[11px] uppercase tracking-[0.18em] text-text-muted">
      System
    </p>
  );
}

function MarkdownText({ markdown }: { markdown: string }) {
  const blocks = parseMarkdownBlocks(markdown);

  return (
    <div className="space-y-3 text-sm text-text-primary">
      {blocks.map((block) => renderMarkdownBlock(block))}
    </div>
  );
}

function renderMarkdownBlock(block: MarkdownBlock): ReactNode {
  if (block.kind === "heading") {
    return (
      <Heading
        key={block.key}
        level={block.level}
      >
        {renderInline(block.content, `${block.key}-heading`)}
      </Heading>
    );
  }

  if (block.kind === "code") {
    return (
      <pre
        key={block.key}
        className="overflow-x-auto whitespace-pre-wrap rounded-xl border border-border bg-surface-0 px-4 py-3 font-mono text-xs leading-6 text-text-secondary"
      >
        {block.language && (
          <span className="mb-2 block text-[11px] uppercase tracking-[0.18em] text-text-muted">
            {block.language}
          </span>
        )}
        <code>{block.content}</code>
      </pre>
    );
  }

  if (block.kind === "unordered_list") {
    return (
      <ul key={block.key} className="list-disc space-y-1 pl-5">
        {block.items.map((item, index) => (
          <li key={`${block.key}-item-${index}`}>
            {renderInline(item, `${block.key}-item-${index}`)}
          </li>
        ))}
      </ul>
    );
  }

  if (block.kind === "ordered_list") {
    return (
      <ol
        key={block.key}
        start={block.start}
        className="list-decimal space-y-1 pl-5"
      >
        {block.items.map((item, index) => (
          <li key={`${block.key}-item-${index}`}>
            {renderInline(item, `${block.key}-item-${index}`)}
          </li>
        ))}
      </ol>
    );
  }

  if (block.kind === "blockquote") {
    return (
      <blockquote
        key={block.key}
        className="border-l-2 border-border pl-4 text-text-secondary"
      >
        <MarkdownText markdown={block.content} />
      </blockquote>
    );
  }

  return (
    <p key={block.key} className="leading-6">
      {renderInline(block.content, `${block.key}-paragraph`)}
    </p>
  );
}

function Heading({
  children,
  level,
}: {
  children: ReactNode;
  level: number;
}) {
  const className = headingClassName(level);

  if (level === 1) {
    return <h1 className={className}>{children}</h1>;
  }

  if (level === 2) {
    return <h2 className={className}>{children}</h2>;
  }

  if (level === 3) {
    return <h3 className={className}>{children}</h3>;
  }

  if (level === 4) {
    return <h4 className={className}>{children}</h4>;
  }

  if (level === 5) {
    return <h5 className={className}>{children}</h5>;
  }

  return <h6 className={className}>{children}</h6>;
}

function headingClassName(level: number): string {
  if (level === 1) {
    return "text-xl font-semibold leading-8";
  }

  if (level === 2) {
    return "text-lg font-semibold leading-7";
  }

  if (level === 3) {
    return "text-base font-semibold leading-7";
  }

  return "text-sm font-semibold leading-6";
}

function parseMarkdownBlocks(markdown: string): MarkdownBlock[] {
  const lines = markdown.replaceAll("\r\n", "\n").split("\n");
  const blocks: MarkdownBlock[] = [];
  let index = 0;

  while (index < lines.length) {
    const line = lines[index];
    const trimmed = line.trim();

    if (trimmed === "") {
      index += 1;
      continue;
    }

    if (trimmed.startsWith("```")) {
      const language = trimmed.slice(3).trim();
      const codeLines: string[] = [];
      index += 1;

      while (index < lines.length && !lines[index].trim().startsWith("```")) {
        codeLines.push(lines[index]);
        index += 1;
      }

      if (index < lines.length) {
        index += 1;
      }

      blocks.push({
        kind: "code",
        key: `code-${blocks.length}`,
        language,
        content: codeLines.join("\n"),
      });
      continue;
    }

    const headingMatch = /^#{1,6}\s+/.exec(trimmed);
    if (headingMatch) {
      const level = trimmed.match(/^#{1,6}/)?.[0].length ?? 1;
      blocks.push({
        kind: "heading",
        key: `heading-${blocks.length}`,
        level,
        content: trimmed.slice(level).trim(),
      });
      index += 1;
      continue;
    }

    if (/^>\s?/.test(trimmed)) {
      const quoteLines: string[] = [];

      while (index < lines.length && /^>\s?/.test(lines[index].trim())) {
        quoteLines.push(lines[index].trim().replace(/^>\s?/, ""));
        index += 1;
      }

      blocks.push({
        kind: "blockquote",
        key: `blockquote-${blocks.length}`,
        content: quoteLines.join("\n"),
      });
      continue;
    }

    const unorderedMatch = /^[-*+]\s+/.exec(trimmed);
    if (unorderedMatch) {
      const items: string[] = [];

      while (index < lines.length && /^[-*+]\s+/.test(lines[index].trim())) {
        items.push(lines[index].trim().replace(/^[-*+]\s+/, ""));
        index += 1;
      }

      blocks.push({
        kind: "unordered_list",
        key: `ul-${blocks.length}`,
        items,
      });
      continue;
    }

    const orderedMatch = /^(\d+)\.\s+/.exec(trimmed);
    if (orderedMatch) {
      const items: string[] = [];
      const start = Number.parseInt(orderedMatch[1], 10);

      while (index < lines.length && /^\d+\.\s+/.test(lines[index].trim())) {
        items.push(lines[index].trim().replace(/^\d+\.\s+/, ""));
        index += 1;
      }

      blocks.push({
        kind: "ordered_list",
        key: `ol-${blocks.length}`,
        items,
        start,
      });
      continue;
    }

    const paragraphLines: string[] = [];

    while (index < lines.length && canContinueParagraph(lines[index])) {
      paragraphLines.push(lines[index]);
      index += 1;
    }

    blocks.push({
      kind: "paragraph",
      key: `paragraph-${blocks.length}`,
      content: paragraphLines.join("\n").trim(),
    });
  }

  return blocks;
}

function canContinueParagraph(line: string): boolean {
  const trimmed = line.trim();

  if (trimmed === "") {
    return false;
  }

  if (trimmed.startsWith("```")) {
    return false;
  }

  if (/^#{1,6}\s+/.test(trimmed)) {
    return false;
  }

  if (/^>\s?/.test(trimmed)) {
    return false;
  }

  if (/^[-*+]\s+/.test(trimmed)) {
    return false;
  }

  if (/^\d+\.\s+/.test(trimmed)) {
    return false;
  }

  return true;
}

function renderInline(markdown: string, keyPrefix: string): ReactNode[] {
  const lines = markdown.split("\n");

  return lines.flatMap((line, index) => {
    const lineNodes = renderInlineSegment(line, `${keyPrefix}-line-${index}`);

    if (index === lines.length - 1) {
      return lineNodes;
    }

    return [
      ...lineNodes,
      <br key={`${keyPrefix}-break-${index}`} />,
    ];
  });
}

function renderInlineSegment(text: string, keyPrefix: string): ReactNode[] {
  const match = firstInlineMatch(text);

  if (!match) {
    return [text];
  }

  const before = text.slice(0, match.index);
  const after = text.slice(match.index + match.fullText.length);
  const nodes: ReactNode[] = [];

  if (before !== "") {
    nodes.push(...renderInlineSegment(before, `${keyPrefix}-before`));
  }

  nodes.push(renderInlineMatch(match, `${keyPrefix}-match`));

  if (after !== "") {
    nodes.push(...renderInlineSegment(after, `${keyPrefix}-after`));
  }

  return nodes;
}

function firstInlineMatch(text: string): InlineMatch | null {
  let earliestMatch: InlineMatch | null = null;

  INLINE_PATTERNS.forEach((pattern) => {
    const result = pattern.regex.exec(text);

    if (!result) {
      return;
    }

    const match: InlineMatch = {
      kind: pattern.kind,
      index: result.index,
      fullText: result[0],
      captures: result.slice(1),
    };

    if (!earliestMatch || match.index < earliestMatch.index) {
      earliestMatch = match;
    }
  });

  return earliestMatch;
}

function renderInlineMatch(match: InlineMatch, key: string): ReactNode {
  if (match.kind === "link") {
    const [label, href] = match.captures;

    return (
      <a
        key={key}
        href={href}
        target="_blank"
        rel="noreferrer"
        className="text-accent underline decoration-accent/40 underline-offset-4 transition-colors hover:text-accent-hover"
      >
        {renderInlineSegment(label, `${key}-label`)}
      </a>
    );
  }

  if (match.kind === "code") {
    return (
      <code
        key={key}
        className="rounded bg-surface-0 px-1.5 py-0.5 font-mono text-[0.92em] text-text-primary"
      >
        {match.captures[0]}
      </code>
    );
  }

  if (match.kind === "strong") {
    return (
      <strong key={key} className="font-semibold text-text-primary">
        {renderInlineSegment(match.captures[0], `${key}-strong`)}
      </strong>
    );
  }

  return (
    <em key={key} className="italic">
      {renderInlineSegment(match.captures[0], `${key}-emphasis`)}
    </em>
  );
}

function firstNonEmpty(...values: Array<string | undefined>): string {
  for (const value of values) {
    if (!value) {
      continue;
    }

    if (value.trim() !== "") {
      return value;
    }
  }

  return "";
}

function formatStructuredValue(value: string): string {
  const trimmed = value.trim();

  if (!looksLikeStructuredData(trimmed)) {
    return value;
  }

  try {
    return JSON.stringify(JSON.parse(trimmed), null, 2);
  } catch {
    return value;
  }
}

function looksLikeStructuredData(value: string): boolean {
  const trimmed = value.trim();

  if (trimmed === "") {
    return false;
  }

  return (
    (trimmed.startsWith("{") && trimmed.endsWith("}")) ||
    (trimmed.startsWith("[") && trimmed.endsWith("]"))
  );
}
