import { useEffect } from "react";

export interface DesktopNotification {
  id: string;
  title: string;
  body: string;
  tone: string;
  source: string;
}

export function NotificationViewport({
  notifications,
  onDismiss,
}: {
  notifications: DesktopNotification[];
  onDismiss: (id: string) => void;
}) {
  useEffect(() => {
    const timers = notifications.map((notification) =>
      window.setTimeout(() => {
        onDismiss(notification.id);
      }, 6000),
    );

    return () => {
      timers.forEach((timer) => window.clearTimeout(timer));
    };
  }, [notifications, onDismiss]);

  if (notifications.length === 0) {
    return null;
  }

  return (
    <div className="fixed bottom-6 right-6 z-[90] flex w-96 flex-col gap-2">
      {notifications.map((notification) => (
        <div
          key={notification.id}
          className={`rounded-xl border px-4 py-3 shadow-2xl ${notificationClassName(notification.tone)}`}
        >
          <div className="flex items-start justify-between gap-3">
            <div>
              <p className="text-xs uppercase tracking-[0.2em] text-text-muted">
                {notification.source || "desktop"}
              </p>
              <p className="mt-1 text-sm font-medium text-text-primary">{notification.title}</p>
              <p className="mt-1 text-sm text-text-secondary">{notification.body}</p>
            </div>

            <button
              onClick={() => onDismiss(notification.id)}
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

function notificationClassName(tone: string): string {
  if (tone === "success") {
    return "border-success/20 bg-surface-1";
  }
  if (tone === "warning") {
    return "border-warning/20 bg-surface-1";
  }
  return "border-border bg-surface-1";
}
