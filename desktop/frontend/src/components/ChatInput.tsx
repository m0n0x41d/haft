import type { FormEvent, KeyboardEvent } from "react";

interface ChatInputProps {
  agentLabel: string;
  disabled?: boolean;
  isSubmitting?: boolean;
  placeholder?: string;
  value: string;
  onChange: (value: string) => void;
  onSubmit: (value: string) => void | Promise<void>;
}

export function ChatInput({
  agentLabel,
  disabled = false,
  isSubmitting = false,
  placeholder = "Message...",
  value,
  onChange,
  onSubmit,
}: ChatInputProps) {
  const canSubmit = !disabled && !isSubmitting && value.trim() !== "";

  const handleSubmit = (event?: FormEvent<HTMLFormElement>) => {
    event?.preventDefault();

    if (!canSubmit) {
      return;
    }

    void onSubmit(value.trim());
  };

  const handleKeyDown = (event: KeyboardEvent<HTMLTextAreaElement>) => {
    if (event.key !== "Enter" || event.shiftKey) {
      return;
    }

    event.preventDefault();
    handleSubmit();
  };

  return (
    <form onSubmit={handleSubmit} className="shrink-0 px-6 pb-2 pt-2">
      <div className="rounded-xl border border-border bg-surface-0 px-4 py-2.5">
        <textarea
          value={value}
          disabled={disabled || isSubmitting}
          onChange={(event) => onChange(event.target.value)}
          onKeyDown={handleKeyDown}
          placeholder={placeholder}
          rows={1}
          className="w-full resize-none bg-transparent text-sm text-text-primary outline-none placeholder:text-text-muted"
        />
        <div className="flex items-center justify-between">
          <span className="text-[11px] text-text-muted/50">{agentLabel}</span>
          <button
            type="submit"
            disabled={!canSubmit}
            className="text-text-muted transition-colors hover:text-text-primary disabled:opacity-20"
            title="Send (Enter)"
          >
            <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
              <path d="M3 8h7M7 5l3 3-3 3" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
              <path d="M12 3v10" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
            </svg>
          </button>
        </div>
      </div>
    </form>
  );
}
