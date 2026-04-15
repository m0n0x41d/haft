import { useState } from "react";

import { ChatMessage } from "./ChatMessage";
import {
  buildChatEntries,
  hasStructuredChatBlocks,
  taskTranscriptText,
  type ChatTranscriptState,
} from "../lib/api";

interface ChatViewProps {
  task: ChatTranscriptState;
  className?: string;
  emptyMessage?: string;
}

const RAW_OUTPUT_MAX_LINES = 500;
const RAW_OUTPUT_VISIBLE_LINES = 200;

export function ChatView({
  task,
  className,
  emptyMessage = "No transcript yet.",
}: ChatViewProps) {
  const entries = buildChatEntries(task.chat_blocks);
  const transcript = taskTranscriptText(task);
  const hasStructuredBlocks = hasStructuredChatBlocks(task);
  const hasFallbackTranscript = transcript.trim() !== "";
  const hasErrorMessage = task.error_message.trim() !== "";
  const isRunning = task.status === "running";

  // Agents that produce structured JSONL always use structured view,
  // even when blocks are empty (still streaming). Raw transcript only
  // for haft agent or unknown types.
  const agentKind = task.agent ?? "";
  const structuredAgent = agentKind === "claude" || agentKind === "codex";
  const preferStructured = hasStructuredBlocks || structuredAgent;

  const shouldShowEmptyState =
    !preferStructured &&
    !hasFallbackTranscript &&
    !hasErrorMessage &&
    !isRunning;

  if (shouldShowEmptyState) {
    return (
      <div className={joinClassNames("flex justify-center", className)}>
        <div className="rounded-2xl border border-dashed border-border bg-surface-1/50 px-6 py-5 text-sm text-text-muted">
          {emptyMessage}
        </div>
      </div>
    );
  }

  return (
    <div className={joinClassNames("space-y-4", className)}>
      {hasErrorMessage && (
        <div className="flex justify-start">
          <div className="max-w-[85%] rounded-2xl rounded-tl-sm border border-danger/20 bg-danger/5 px-4 py-3">
            <p className="mb-1 text-xs uppercase tracking-[0.18em] text-danger">
              Error
            </p>
            <p className="whitespace-pre-wrap text-sm text-danger/90">
              {task.error_message}
            </p>
          </div>
        </div>
      )}

      {preferStructured ? (
        <>
          {entries.map((entry) => (
            <ChatMessage
              key={entry.block.id || `${entry.block.type}-${entry.block.call_id || "block"}`}
              block={entry.block}
              toolResults={entry.toolResults}
            />
          ))}
          {entries.length === 0 && isRunning && (
            <div className="flex justify-start">
              <div className="rounded-2xl rounded-tl-sm border border-border bg-surface-1 px-4 py-3">
                <div className="flex items-center gap-2">
                  <span className="h-1.5 w-1.5 animate-pulse rounded-full bg-accent" />
                  <span className="text-xs text-text-muted">Starting...</span>
                </div>
              </div>
            </div>
          )}
        </>
      ) : (
        <RawTranscriptBubble output={transcript} running={isRunning} />
      )}

      {isRunning && hasStructuredBlocks && (
        <div className="flex justify-start">
          <div className="rounded-2xl rounded-tl-sm border border-border bg-surface-1 px-4 py-3">
            <div className="flex items-center gap-2">
              <span className="h-1.5 w-1.5 animate-pulse rounded-full bg-accent" />
              <span className="text-xs text-text-muted">Agent is working...</span>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

function RawTranscriptBubble({
  output,
  running,
}: {
  output: string;
  running: boolean;
}) {
  const [showFull, setShowFull] = useState(false);
  const lines = output.split("\n");
  const isTruncated = !showFull && lines.length > RAW_OUTPUT_MAX_LINES;
  const visibleOutput = isTruncated
    ? lines.slice(-RAW_OUTPUT_VISIBLE_LINES).join("\n")
    : output;

  return (
    <div className="flex justify-start">
      <div className="max-w-[85%] rounded-2xl rounded-tl-sm border border-border bg-surface-1 px-4 py-3">
        <p className="mb-2 text-xs uppercase tracking-[0.18em] text-text-muted">
          Raw transcript
        </p>

        {running && (
          <div className="mb-2 flex items-center gap-2">
            <span className="h-1.5 w-1.5 animate-pulse rounded-full bg-accent" />
            <span className="text-[11px] text-accent">Streaming</span>
          </div>
        )}

        {isTruncated && (
          <button
            type="button"
            onClick={() => setShowFull(true)}
            className="mb-2 text-xs text-accent transition-colors hover:text-accent-hover"
          >
            Showing last {RAW_OUTPUT_VISIBLE_LINES} of {lines.length} lines. Show full output.
          </button>
        )}

        <pre className="overflow-x-auto whitespace-pre-wrap font-mono text-xs leading-6 text-text-secondary">
          {visibleOutput}
        </pre>
      </div>
    </div>
  );
}

function joinClassNames(...values: Array<string | undefined>): string {
  return values.filter(Boolean).join(" ");
}
