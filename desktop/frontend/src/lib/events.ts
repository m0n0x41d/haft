import { listen, type Event, type UnlistenFn } from "@tauri-apps/api/event";

/**
 * Subscribe to a Tauri event, extracting the payload for the handler.
 * Returns a synchronous cleanup function (handles async unlisten internally)
 * so callers can use the same pattern as the old Wails EventsOn.
 *
 * When running outside Tauri (e.g. `npm run dev` in a plain browser), we
 * skip the listen call and return a no-op unsubscribe. Otherwise
 * `@tauri-apps/api/event` rejects at import/load time and every effect
 * that subscribes on mount (dashboard, tasks, terminal) would break the
 * mock-transport flow in `api.ts`.
 */
function isTauri(): boolean {
  return typeof window !== "undefined" && "__TAURI_INTERNALS__" in window;
}

export function subscribe<T>(event: string, handler: (payload: T) => void): () => void {
  if (!isTauri()) {
    return () => {};
  }

  const promise: Promise<UnlistenFn> = listen<T>(event, (e: Event<T>) => {
    handler(e.payload);
  });

  return () => {
    void promise.then((unlisten) => unlisten());
  };
}
