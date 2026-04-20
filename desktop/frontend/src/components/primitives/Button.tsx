import type { ButtonHTMLAttributes, ReactNode } from "react";

export type ButtonVariant =
  | "primary"
  | "secondary"
  | "ghost"
  | "warning"
  | "danger"
  | "accent-chip";

export interface ButtonProps
  extends Omit<ButtonHTMLAttributes<HTMLButtonElement>, "className"> {
  variant?: ButtonVariant;
  icon?: ReactNode;
  className?: string;
  children?: ReactNode;
}

const VARIANT_CLASSES: Record<ButtonVariant, string> = {
  // Primary: pill, accent fill, white-on-green CTA. The signature CTA shape.
  primary:
    "rounded-full bg-accent text-surface-0 font-medium border border-transparent " +
    "hover:bg-accent-hover " +
    "disabled:bg-surface-2 disabled:text-text-muted disabled:border-border disabled:cursor-not-allowed",
  // Secondary: rounded rect, surface fill, neutral border.
  secondary:
    "rounded-lg bg-surface-2 text-text-primary border border-border " +
    "hover:bg-surface-3 hover:border-border-bright",
  // Ghost: transparent until hover.
  ghost:
    "rounded-lg bg-transparent text-text-secondary border border-border " +
    "hover:bg-surface-2 hover:text-text-primary",
  // Warning: pill, warning wash.
  warning:
    "rounded-full bg-warning/10 text-warning border border-warning/30 " +
    "hover:bg-warning/20",
  // Danger: rect, transparent danger outline. Reserved for destructive actions.
  danger:
    "rounded-lg bg-transparent text-danger border border-danger/30 " +
    "hover:bg-danger/10",
  // Accent chip: rect, accent wash. For non-primary affordances that still
  // signal accent-coloured semantics (selected filters, active toggles).
  "accent-chip":
    "rounded-lg bg-accent-wash text-accent border border-accent-border " +
    "hover:bg-accent/15",
};

/**
 * Standard Haft button. Six variants encode the full CTA hierarchy used
 * across the cockpit. Stays consistent with the codebase's Tailwind theme
 * tokens — no separate kit.css required.
 */
export function Button({
  variant = "secondary",
  icon,
  className = "",
  children,
  type = "button",
  ...rest
}: ButtonProps) {
  const variantClasses = VARIANT_CLASSES[variant];
  return (
    <button
      type={type}
      className={`inline-flex items-center gap-1.5 px-3.5 py-1.5 text-xs cursor-pointer transition-colors ${variantClasses} ${className}`}
      {...rest}
    >
      {icon}
      {children}
    </button>
  );
}
