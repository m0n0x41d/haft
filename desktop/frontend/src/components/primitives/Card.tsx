import type { HTMLAttributes, ReactNode } from "react";

export type CardState = "default" | "hover" | "selected";

export interface CardProps
  extends Omit<HTMLAttributes<HTMLDivElement>, "className"> {
  state?: CardState;
  dashed?: boolean;
  className?: string;
  children: ReactNode;
}

/**
 * Standard Haft surface card. Border-first depth — never a drop shadow.
 *
 * - Default: surface-1 + border + 12px radius.
 * - Hover: brightens border + bumps surface to -2.
 * - Selected: surface-2 + accent border at 30% alpha.
 * - Dashed: transparent fill with dashed border, used for empty states /
 *   "add new" affordances.
 */
export function Card({
  state = "default",
  dashed = false,
  className = "",
  children,
  ...rest
}: CardProps) {
  const baseClasses = "rounded-xl border px-[18px] py-4 transition-colors";

  const stateClasses = (() => {
    if (dashed) {
      return "bg-transparent border-dashed border-border text-text-secondary";
    }
    switch (state) {
      case "selected":
        return "bg-surface-2 border-accent/30";
      case "hover":
        return "bg-surface-2 border-border-bright";
      default:
        return "bg-surface-1 border-border hover:bg-surface-2 hover:border-border-bright";
    }
  })();

  return (
    <div className={`${baseClasses} ${stateClasses} ${className}`} {...rest}>
      {children}
    </div>
  );
}
