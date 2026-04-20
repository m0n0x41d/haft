import type { ReactNode } from "react";

export type BadgeTone = "neutral" | "accent" | "success" | "warning" | "danger";

export interface BadgeProps {
  tone?: BadgeTone;
  mono?: boolean;
  className?: string;
  children: ReactNode;
}

const TONE_CLASSES: Record<BadgeTone, string> = {
  neutral: "bg-surface-2 border-border text-text-muted",
  accent: "bg-accent-wash border-accent-border text-accent",
  success: "bg-success/10 border-success/30 text-success",
  warning: "bg-warning/10 border-warning/30 text-warning",
  danger: "bg-danger/10 border-danger/30 text-danger",
};

/**
 * Pill-shaped status badge. Tones mirror the semantic palette + a neutral
 * default. Set `mono` for IDs / numeric badges.
 */
export function Badge({
  tone = "neutral",
  mono = false,
  className = "",
  children,
}: BadgeProps) {
  return (
    <span
      className={`inline-flex items-center gap-1.5 rounded-full border px-2.5 py-0.5 text-[11px] ${
        mono ? "font-mono" : ""
      } ${TONE_CLASSES[tone]} ${className}`}
    >
      {children}
    </span>
  );
}
