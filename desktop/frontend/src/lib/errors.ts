export interface AppErrorDetail {
  id: string;
  message: string;
  scope?: string;
}

const APP_ERROR_EVENT = "desktop:error";

export function reportError(error: unknown, scope?: string): void {
  const message = formatError(error);

  window.dispatchEvent(
    new CustomEvent<AppErrorDetail>(APP_ERROR_EVENT, {
      detail: {
        id: `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
        message,
        scope,
      },
    }),
  );
}

export function listenForErrors(handler: (detail: AppErrorDetail) => void): () => void {
  const listener = (event: Event) => {
    const customEvent = event as CustomEvent<AppErrorDetail>;

    if (!customEvent.detail) {
      return;
    }

    handler(customEvent.detail);
  };

  window.addEventListener(APP_ERROR_EVENT, listener as EventListener);

  return () => {
    window.removeEventListener(APP_ERROR_EVENT, listener as EventListener);
  };
}

function formatError(error: unknown): string {
  if (error instanceof Error) {
    return error.message;
  }

  if (typeof error === "string") {
    return error;
  }

  return "Unexpected error";
}
