import type { ButtonHTMLAttributes, ReactNode } from "react";

export type StatCardVariant = "default" | "accent" | "warning";

export interface StatCardProps
  extends Omit<ButtonHTMLAttributes<HTMLButtonElement>, "className"> {
  label: ReactNode;
  count: ReactNode;
  /** Optional trailing unit (e.g. "%") rendered inline at the same weight as the count. */
  suffix?: ReactNode;
  variant?: StatCardVariant;
  className?: string;
}

const NUMBER_TONE: Record<StatCardVariant, string> = {
  default: "text-text-primary",
  accent: "text-accent",
  warning: "text-warning",
};

const BORDER_TONE: Record<StatCardVariant, string> = {
  default: "border-border hover:border-border-bright",
  accent: "border-accent/30 hover:border-accent/40",
  warning: "border-warning/30 hover:border-warning/40",
};

/**
 * Stat card for dashboard counts. Number above, label below. Variants
 * tint the number + border accordingly. Always interactive — clicking
 * conventionally drills into the underlying list.
 */
export function StatCard({
  label,
  count,
  suffix,
  variant = "default",
  className = "",
  type = "button",
  ...rest
}: StatCardProps) {
  return (
    <button
      type={type}
      className={`min-w-[130px] rounded-xl border bg-surface-1 px-4 py-3.5 text-left transition-colors hover:bg-surface-2 ${BORDER_TONE[variant]} ${className}`}
      {...rest}
    >
      <div
        className={`text-[24px] font-semibold leading-none tracking-[-0.01em] ${NUMBER_TONE[variant]}`}
      >
        {count}
        {suffix ? (
          <span className="ml-0.5 text-[20px] font-medium text-text-muted">
            {suffix}
          </span>
        ) : null}
      </div>
      <div className="mt-0.5 text-[11px] text-text-muted">{label}</div>
    </button>
  );
}
