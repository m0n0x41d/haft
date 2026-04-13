import { useEffect } from "react";

import type { AppErrorDetail } from "../lib/errors";

export function ToastViewport({
  toasts,
  onDismiss,
}: {
  toasts: AppErrorDetail[];
  onDismiss: (id: string) => void;
}) {
  useEffect(() => {
    const timers = toasts.map((toast) =>
      window.setTimeout(() => {
        onDismiss(toast.id);
      }, 5000),
    );

    return () => {
      timers.forEach((timer) => window.clearTimeout(timer));
    };
  }, [onDismiss, toasts]);

  if (toasts.length === 0) {
    return null;
  }

  return (
    <div className="fixed top-14 right-6 z-[100] flex w-96 flex-col gap-2">
      {toasts.map((toast) => (
        <div
          key={toast.id}
          className="rounded-xl border border-danger/20 bg-surface-1 px-4 py-3 shadow-2xl"
        >
          <div className="flex items-start justify-between gap-3">
            <div>
              <p className="text-xs uppercase tracking-wider text-danger">
                {toast.scope || "Desktop error"}
              </p>
              <p className="mt-1 text-sm text-text-primary">{toast.message}</p>
            </div>

            <button
              onClick={() => onDismiss(toast.id)}
              className="text-xs text-text-muted transition-colors hover:text-text-primary"
            >
              x
            </button>
          </div>
        </div>
      ))}
    </div>
  );
}
