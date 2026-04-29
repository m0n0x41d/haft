import type { ComponentType } from "react";

export interface RailBtnProps {
  icon: ComponentType<{ size?: number | string; className?: string }>;
  tip: string;
  onClick: () => void;
  active?: boolean;
  accent?: boolean;
  label?: string;
  className?: string;
}

/**
 * Icon rail button — the 48px-wide far-left navigation column. Three
 * visual states: muted (default), active (surface-2 fill), accent (used
 * for the Plus button; green foreground, no fill until hover).
 *
 * The icon component must accept a `size` prop in the Lucide convention.
 */
export function RailBtn({
  icon: Icon,
  tip,
  onClick,
  active = false,
  accent = false,
  label,
  className = "",
}: RailBtnProps) {
  const toneClasses = accent
    ? "text-accent hover:bg-accent/10"
    : active
      ? "bg-surface-2 text-text-primary"
      : "text-text-muted hover:bg-surface-2/50 hover:text-text-primary";

  return (
    <button
      onClick={onClick}
      title={tip}
      className={`mb-1 flex ${label ? "h-11" : "h-9"} w-9 flex-col items-center justify-center gap-0.5 rounded-lg transition-colors ${toneClasses} ${className}`}
    >
      <Icon size={label ? 16 : 18} />
      {label ? (
        <span className="font-mono text-[8px] uppercase leading-none tracking-tight">
          {label}
        </span>
      ) : null}
    </button>
  );
}
