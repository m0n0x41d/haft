export type MonoIdTone = "neutral" | "accent";

export interface MonoIdProps {
  id: string;
  tone?: MonoIdTone;
  className?: string;
}

const TONE_CLASSES: Record<MonoIdTone, string> = {
  neutral: "bg-surface-2 border-border text-text-secondary",
  accent: "bg-accent-wash border-accent-border text-accent",
};

/**
 * Pill-shaped monospaced artifact ID badge — used to render canonical haft
 * IDs (e.g. dec-20260418-a3f7c1, prob-20260418-84fa77) inline. Always
 * monospaced; tone defaults to neutral, accent reserved for selected /
 * recommended artifact references.
 */
export function MonoId({ id, tone = "neutral", className = "" }: MonoIdProps) {
  return (
    <span
      className={`inline-flex items-center rounded-full border px-2 py-0.5 font-mono text-[11px] ${TONE_CLASSES[tone]} ${className}`}
    >
      {id}
    </span>
  );
}
