import type { InputHTMLAttributes } from "react";
import { Eyebrow } from "./Eyebrow";

export interface InputProps
  extends Omit<InputHTMLAttributes<HTMLInputElement>, "className"> {
  label?: string;
  mono?: boolean;
  className?: string;
}

/**
 * Standard Haft form input with optional uppercase mono eyebrow label.
 * Focus state is a single accent-tinted border shift — no separate outline.
 *
 * Set `mono` for monospaced fields (IDs, code, numeric data).
 */
export function Input({
  label,
  mono = false,
  className = "",
  ...rest
}: InputProps) {
  return (
    <div className="flex flex-col gap-1">
      {label ? <Eyebrow>{label}</Eyebrow> : null}
      <input
        className={`w-full rounded-xl border border-border bg-surface-2 px-3 py-2 text-[13px] text-text-primary outline-none transition-colors placeholder:text-text-faint focus:border-accent/60 ${
          mono ? "font-mono" : ""
        } ${className}`}
        {...rest}
      />
    </div>
  );
}
