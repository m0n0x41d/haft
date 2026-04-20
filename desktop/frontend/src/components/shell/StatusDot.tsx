export type TaskStatus =
  | "running"
  | "idle"
  | "completed"
  | "failed"
  | "cancelled"
  | "pending"
  | (string & {}); // accepts unknown strings at the type boundary; unknown → muted

export interface StatusDotProps {
  status: TaskStatus;
  className?: string;
}

const STATUS_CLASSES: Record<string, string> = {
  running: "bg-accent animate-pulse",
  idle: "bg-accent",
  completed: "bg-success",
  failed: "bg-danger",
  cancelled: "bg-text-muted",
  pending: "bg-warning",
};

/**
 * Status indicator dot used in the sidebar task list and other inline
 * agent-state displays. Pulse animation for `running` mirrors the
 * streaming indicator throughout the cockpit.
 */
export function StatusDot({ status, className = "" }: StatusDotProps) {
  const tone = STATUS_CLASSES[status] ?? "bg-text-muted";
  return (
    <span className={`h-2 w-2 shrink-0 rounded-full ${tone} ${className}`} />
  );
}
