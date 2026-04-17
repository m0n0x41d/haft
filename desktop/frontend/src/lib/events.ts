import { listen, type Event, type UnlistenFn } from "@tauri-apps/api/event";

/**
 * Subscribe to a Tauri event, extracting the payload for the handler.
 * Returns a synchronous cleanup function (handles async unlisten internally)
 * so callers can use the same pattern as the old Wails EventsOn.
 */
export function subscribe<T>(event: string, handler: (payload: T) => void): () => void {
  const promise: Promise<UnlistenFn> = listen<T>(event, (e: Event<T>) => {
    handler(e.payload);
  });

  return () => {
    void promise.then((unlisten) => unlisten());
  };
}
