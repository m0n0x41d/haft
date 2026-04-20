import type { ReactNode } from "react";

export interface EyebrowProps {
  children: ReactNode;
  className?: string;
}

/**
 * Uppercase mono eyebrow label — the signature typographic move of Haft's
 * design system. Used above sections, table headers, and grouped labels.
 *
 * Visual contract: 11px, mono, 0.22em letter-spacing, text-muted.
 */
export function Eyebrow({ children, className = "" }: EyebrowProps) {
  return (
    <div
      className={`font-mono text-[11px] uppercase tracking-[0.22em] text-text-muted ${className}`}
    >
      {children}
    </div>
  );
}
