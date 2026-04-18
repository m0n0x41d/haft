import type { ReactNode } from "react";

export type PillTone = "neutral" | "accent" | "warning";

export interface PillProps {
  tone?: PillTone;
  className?: string;
  children: ReactNode;
}

const TONE_CLASSES: Record<PillTone, string> = {
  neutral: "bg-surface-2 border-border text-text-secondary",
  accent: "bg-accent-wash border-accent-border text-accent",
  warning: "bg-warning/10 border-warning/30 text-warning",
};

/**
 * Generic pill chip. Sibling of Badge with a softer default; used for
 * non-status decorations like keyboard hints, small counts, decoration tags.
 */
export function Pill({ tone = "neutral", className = "", children }: PillProps) {
  return (
    <span
      className={`inline-flex items-center rounded-full border px-2.5 py-0.5 text-[11px] ${TONE_CLASSES[tone]} ${className}`}
    >
      {children}
    </span>
  );
}
