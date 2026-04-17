import { useCallback, useEffect, useRef, useState } from "react";

import { FitAddon } from "@xterm/addon-fit";
import { Terminal } from "@xterm/xterm";
import "@xterm/xterm/css/xterm.css";
import { subscribe } from "../lib/events";
import {
  closeTerminalSession,
  createTerminalSession,
  listTerminalSessions,
  resizeTerminalSession,
  writeTerminalInput,
  type TerminalSession,
} from "../lib/api";
import { reportError } from "../lib/errors";

interface TerminalOutputEvent {
  id: string;
  data: string;
}

export function TerminalPanel({
  open,
  projectPath,
  onClose,
}: {
  open: boolean;
  projectPath: string;
  onClose: () => void;
}) {
  const [sessions, setSessions] = useState<TerminalSession[]>([]);
  const [activeId, setActiveId] = useState<string | null>(null);
  const [height, setHeight] = useState(320);
  const creatingRef = useRef(false);

  const createSession = useCallback(async () => {
    if (creatingRef.current) {
      return;
    }

    creatingRef.current = true;

    try {
      const session = await createTerminalSession(projectPath);
      setSessions((current) => [...current, session]);
      setActiveId(session.id);
    } catch (error) {
      reportError(error, "terminal");
    } finally {
      creatingRef.current = false;
    }
  }, [projectPath]);

  useEffect(() => {
    if (!open) {
      return;
    }

    listTerminalSessions()
      .then((currentSessions) => {
        setSessions(currentSessions);
        setActiveId((current) => current ?? currentSessions[0]?.id ?? null);

        if (currentSessions.length === 0) {
          void createSession();
        }
      })
      .catch((error) => {
        reportError(error, "terminal sessions");
      });
  }, [open, createSession]);

  useEffect(() => {
    if (!open) {
      return;
    }

    const stopStatus = subscribe<TerminalSession>("terminal.status", (payload) => {
      if (payload.status === "running") {
        setSessions((current) => upsertSession(current, payload));
        setActiveId((current) => current ?? payload.id);
        return;
      }

      setSessions((current) => {
        const remaining = current.filter((session) => session.id !== payload.id);
        setActiveId((active) => (active === payload.id ? remaining[0]?.id ?? null : active));
        return remaining;
      });
    });

    return () => {
      stopStatus?.();
    };
  }, [open]);

  useEffect(() => {
    if (!open || sessions.length !== 0) {
      return;
    }

    void createSession();
  }, [open, sessions.length, createSession]);

  const handleClose = useCallback(() => {
    onClose();
  }, [onClose]);

  const handleCloseTab = useCallback((id: string) => {
    closeTerminalSession(id)
      .then(() => {
        setSessions((current) => {
          const remaining = current.filter((session) => session.id !== id);
          setActiveId((active) => (active === id ? remaining[0]?.id ?? null : active));
          return remaining;
        });
      })
      .catch((error) => {
        reportError(error, "close terminal");
      });
  }, []);

  const startResize = useCallback(() => {
    const handleMouseMove = (event: MouseEvent) => {
      setHeight((current) => {
        const nextHeight = window.innerHeight - event.clientY;
        if (!Number.isFinite(nextHeight)) {
          return current;
        }

        return Math.max(220, Math.min(nextHeight, 640));
      });
    };

    const handleMouseUp = () => {
      window.removeEventListener("mousemove", handleMouseMove);
      window.removeEventListener("mouseup", handleMouseUp);
    };

    window.addEventListener("mousemove", handleMouseMove);
    window.addEventListener("mouseup", handleMouseUp);
  }, []);

  if (!open) {
    return null;
  }

  return (
    <div
      className="border-t border-border bg-surface-1/95 backdrop-blur-sm"
      style={{ height }}
    >
      <div
        onMouseDown={startResize}
        className="flex h-3 w-full cursor-row-resize items-center justify-center text-text-muted"
      >
        <span className="h-1 w-16 rounded-full bg-border" />
      </div>

      <div className="flex h-[calc(100%-0.75rem)] flex-col overflow-hidden">
        <div className="flex shrink-0 items-center justify-between border-b border-border px-4 py-2">
          <div className="flex items-center gap-2 overflow-x-auto">
            {sessions.map((session) => (
              <button
                key={session.id}
                type="button"
                onClick={() => setActiveId(session.id)}
                className={`rounded-lg px-3 py-1 text-xs transition-colors ${
                  activeId === session.id
                    ? "bg-surface-0 text-text-primary"
                    : "bg-surface-2 text-text-muted hover:text-text-primary"
                }`}
              >
                {session.title}
              </button>
            ))}

            <button
              type="button"
              onClick={() => void createSession()}
              className="rounded-lg border border-border bg-surface-2 px-3 py-1 text-xs text-text-secondary transition-colors hover:bg-surface-3"
            >
              + Tab
            </button>
          </div>

          <div className="flex items-center gap-2">
            {activeId && (
              <button
                type="button"
                onClick={() => handleCloseTab(activeId)}
                className="rounded-lg border border-border bg-surface-2 px-3 py-1 text-xs text-text-secondary transition-colors hover:bg-surface-3"
              >
                Close tab
              </button>
            )}
            <button
              type="button"
              onClick={handleClose}
              className="rounded-lg border border-border bg-surface-2 px-3 py-1 text-xs text-text-secondary transition-colors hover:bg-surface-3"
            >
              Hide
            </button>
          </div>
        </div>

        <div className="min-h-0 flex-1 overflow-hidden bg-[#080809]">
          {sessions.map((session) => (
            <TerminalViewport
              key={session.id}
              session={session}
              active={session.id === activeId}
            />
          ))}

          {sessions.length === 0 && (
            <div className="flex h-full items-center justify-center text-sm text-text-muted">
              Preparing terminal session...
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

function TerminalViewport({
  session,
  active,
}: {
  session: TerminalSession;
  active: boolean;
}) {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const terminalRef = useRef<Terminal | null>(null);
  const fitAddonRef = useRef<FitAddon | null>(null);
  const fitFrameRef = useRef<number>(0);

  useEffect(() => {
    if (!containerRef.current) {
      return;
    }

    const terminal = new Terminal({
      allowProposedApi: true,
      cursorBlink: true,
      fontFamily: "JetBrains Mono, ui-monospace, monospace",
      fontSize: 12,
      lineHeight: 1.25,
      theme: {
        background: "#080809",
        foreground: "#e4e4e7",
        cursor: "#a1a1aa",
      },
    });
    const fitAddon = new FitAddon();

    terminal.loadAddon(fitAddon);
    terminal.open(containerRef.current);

    terminalRef.current = terminal;
    fitAddonRef.current = fitAddon;

    const stopOutput = subscribe<TerminalOutputEvent>("terminal.output", (payload) => {
      if (payload.id !== session.id) {
        return;
      }

      terminal.write(payload.data);
    });

    const dataListener = terminal.onData((data) => {
      void writeTerminalInput(session.id, data).catch((error) => {
        reportError(error, "terminal input");
      });
    });

    const safeFit = () => {
      const container = containerRef.current;
      if (!container || !fitAddonRef.current || !terminalRef.current) {
        return;
      }

      if (container.offsetWidth === 0 || container.offsetHeight === 0) {
        return;
      }

      try {
        fitAddonRef.current.fit();
        void resizeTerminalSession(session.id, terminalRef.current.cols, terminalRef.current.rows).catch(() => {});
      } catch {
        // FitAddon.fit() throws when dimensions are invalid — ignore
      }
    };

    const debouncedFit = () => {
      cancelAnimationFrame(fitFrameRef.current);
      fitFrameRef.current = requestAnimationFrame(safeFit);
    };

    const observer = new ResizeObserver(debouncedFit);
    observer.observe(containerRef.current);

    // Delay initial fit to let layout settle
    const initTimer = setTimeout(safeFit, 50);

    return () => {
      clearTimeout(initTimer);
      cancelAnimationFrame(fitFrameRef.current);
      observer.disconnect();
      dataListener.dispose();
      stopOutput?.();
      terminal.dispose();
      terminalRef.current = null;
      fitAddonRef.current = null;
    };
  }, [session.id]);

  useEffect(() => {
    if (!active || !fitAddonRef.current || !terminalRef.current) {
      return;
    }

    const container = containerRef.current;
    if (!container || container.offsetWidth === 0 || container.offsetHeight === 0) {
      return;
    }

    try {
      fitAddonRef.current.fit();
      void resizeTerminalSession(session.id, terminalRef.current.cols, terminalRef.current.rows).catch(() => {});
    } catch {
      // ignore
    }
  }, [active, session.id]);

  return (
    <div
      ref={containerRef}
      className={`h-full w-full overflow-hidden ${active ? "block" : "hidden"}`}
    />
  );
}

function upsertSession(current: TerminalSession[], nextSession: TerminalSession): TerminalSession[] {
  const withoutCurrent = current.filter((session) => session.id !== nextSession.id);
  return [...withoutCurrent, nextSession];
}
