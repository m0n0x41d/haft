import { useCallback, useEffect, useRef, useState } from "react";

import { EventsOn } from "../../wailsjs/runtime/runtime";
import {
  archiveTask,
  cancelTask,
  detectAgents,
  getConfig,
  getTaskOutput,
  handoffTask,
  listTasks,
  openPathInIDE,
  spawnTask,
  type DesktopConfig,
  type InstalledAgent,
  type TaskState,
} from "../lib/api";
import { reportError } from "../lib/errors";

interface TaskOutputEvent {
  id: string;
  chunk: string;
  output: string;
}

// PromptSection reserved for future collapsible brief in chat
// interface PromptSection { title: string; body: string; }

interface ProjectInfo {
  path: string;
  name: string;
  id: string;
  is_active: boolean;
}

export function Tasks({
  selectedTaskId: externalSelectedTask,
  showNewTask: externalShow,
  onNewTaskClose,
  projects,
  activeProjectPath,
}: {
  selectedTaskId?: string | null;
  showNewTask?: boolean;
  onNewTaskClose?: () => void;
  projects?: ProjectInfo[];
  activeProjectPath?: string;
} = {}) {
  const [tasks, setTasks] = useState<TaskState[]>([]);
  const [agents, setAgents] = useState<InstalledAgent[]>([]);
  const [config, setConfig] = useState<DesktopConfig | null>(null);
  const [internalShow, setInternalShow] = useState(false);
  const [selectedTask, setSelectedTask] = useState<string | null>(externalSelectedTask ?? null);
  const [showHandoff, setShowHandoff] = useState(false);
  const [handoffAgent, setHandoffAgent] = useState("codex");
  const outputRef = useRef<HTMLDivElement | null>(null);

  const showNewTask = externalShow || internalShow;

  const setShowNewTask = (visible: boolean) => {
    setInternalShow(visible);

    if (!visible && onNewTaskClose) {
      onNewTaskClose();
    }
  };

  const refresh = useCallback(async () => {
    try {
      const nextTasks = await listTasks();
      setTasks(nextTasks);
    } catch (error) {
      reportError(error, "tasks");
    }
  }, []);

  useEffect(() => {
    if (externalShow) {
      setInternalShow(true);
    }
  }, [externalShow]);

  useEffect(() => {
    if (!externalSelectedTask) {
      return;
    }

    setSelectedTask(externalSelectedTask);
  }, [externalSelectedTask]);

  useEffect(() => {
    detectAgents()
      .then(setAgents)
      .catch((error) => {
        reportError(error, "agents");
      });

    getConfig()
      .then(setConfig)
      .catch((error) => {
        reportError(error, "task config");
      });

    void refresh();

    const interval = window.setInterval(() => {
      void refresh();
    }, 2000);

    return () => {
      window.clearInterval(interval);
    };
  }, [refresh]);

  useEffect(() => {
    const stopOutput = EventsOn("task.output", (payload: TaskOutputEvent) => {
      setTasks((current) =>
        current.map((task) =>
          task.id === payload.id
            ? {
                ...task,
                output: payload.output,
              }
            : task,
        ),
      );
    });

    const stopStatus = EventsOn("task.status", (payload: TaskState) => {
      setTasks((current) => mergeTaskList(current, payload));
    });

    return () => {
      stopOutput?.();
      stopStatus?.();
    };
  }, []);

  useEffect(() => {
    if (tasks.length === 0) {
      setSelectedTask(null);
      return;
    }

    if (selectedTask && tasks.some((task) => task.id === selectedTask)) {
      return;
    }

    setSelectedTask(tasks[0]?.id ?? null);
  }, [selectedTask, tasks]);

  useEffect(() => {
    if (!selectedTask) {
      return;
    }

    getTaskOutput(selectedTask)
      .then((output) => {
        setTasks((current) =>
          current.map((task) =>
            task.id === selectedTask
              ? {
                  ...task,
                  output,
                }
              : task,
          ),
        );
      })
      .catch((error) => {
        reportError(error, "task output");
      });
  }, [selectedTask]);

  const detail = tasks.find((task) => task.id === selectedTask) ?? null;
  const workspacePath = detail ? detail.worktree_path || detail.project_path : "";

  useEffect(() => {
    if (!detail || !outputRef.current) {
      return;
    }

    outputRef.current.scrollTop = outputRef.current.scrollHeight;
  }, [detail]);

  const spawningRef = useRef(false);
  const handleSpawn = async (agent: string, prompt: string, worktree: boolean, branch: string) => {
    if (spawningRef.current) return;
    spawningRef.current = true;
    try {
      const task = await spawnTask(agent, prompt, worktree, branch);

      setTasks((current) => mergeTaskList(current, task));
      setSelectedTask(task.id);
      setShowNewTask(false);
      void refresh();
    } catch (error) {
      reportError(error, "spawn task");
    } finally {
      spawningRef.current = false;
    }
  };

  const handleCancel = async (id: string) => {
    try {
      await cancelTask(id);
      void refresh();
    } catch (error) {
      reportError(error, "cancel task");
    }
  };

  const handleArchive = async (id: string) => {
    try {
      await archiveTask(id);

      setTasks((current) => current.filter((task) => task.id !== id));
      if (selectedTask === id) {
        setSelectedTask(null);
      }
    } catch (error) {
      reportError(error, "archive task");
    }
  };

  const handleOpenWorkspace = async () => {
    if (!workspacePath) {
      return;
    }

    try {
      await openPathInIDE(workspacePath);
    } catch (error) {
      reportError(error, "open workspace");
    }
  };

  const handleHandoff = async () => {
    if (!detail) {
      return;
    }

    try {
      const task = await handoffTask(detail.id, handoffAgent);

      setTasks((current) => mergeTaskList(current, task));
      setSelectedTask(task.id);
      setShowHandoff(false);
      await refresh();
    } catch (error) {
      reportError(error, "handoff task");
    }
  };

  // handleCopy removed — not used in chat view

  return (
    <div className="space-y-6">
      {/* No header — tasks are selected from sidebar, new task from "+" menu */}

      {showNewTask && (
        <NewTaskModal
          agents={agents}
          config={config}
          projects={projects ?? []}
          activeProjectPath={activeProjectPath}
          onSpawn={handleSpawn}
          onClose={() => setShowNewTask(false)}
        />
      )}

      {showHandoff && detail && (
        <HandoffModal
          agents={agents}
          sourceTask={detail}
          targetAgent={handoffAgent}
          onChangeTarget={setHandoffAgent}
          onClose={() => setShowHandoff(false)}
          onConfirm={() => void handleHandoff()}
        />
      )}

      {/* Chat layout: no task selected = empty state, task selected = chat */}
      {!detail ? (
        <div className="flex h-[calc(100vh-12rem)] items-center justify-center">
          <div className="text-center">
            <p className="text-sm text-text-muted">
              {tasks.length === 0 ? "No tasks yet" : "Select a task from the sidebar"}
            </p>
            <p className="mt-1 text-xs text-text-muted">
              Click "+ New Task" or use the sidebar to start
            </p>
          </div>
        </div>
      ) : (
        <div className="flex h-[calc(100vh-12rem)] flex-col">
          {/* Compact header bar */}
          <div className="flex items-center justify-between border-b border-border px-4 py-2 shrink-0">
            <div className="flex items-center gap-3 min-w-0">
              <StatusBadge status={detail.status} />
              <span className="text-xs text-text-muted">{detail.agent}</span>
              {detail.branch && (
                <span className="text-xs text-text-muted font-mono truncate">{detail.branch}</span>
              )}
              {detail.status === "running" && (
                <span className="text-xs text-text-muted animate-pulse">
                  {detail.started_at ? elapsedSince(detail.started_at) : ""}
                </span>
              )}
            </div>
            <div className="flex items-center gap-1.5 shrink-0">
              {/* Auto-run toggle */}
              {detail.status === "running" && (
                <button
                  onClick={async () => {
                    try {
                      const { setTaskAutoRun } = await import("../lib/api");
                      await setTaskAutoRun(detail.id, !detail.auto_run);
                      setTasks((prev) =>
                        prev.map((t) =>
                          t.id === detail.id ? { ...t, auto_run: !t.auto_run } : t
                        )
                      );
                    } catch { /* ignore */ }
                  }}
                  className={`rounded-full border px-3 py-1 text-xs transition-colors ${
                    detail.auto_run
                      ? "border-accent/30 bg-accent/10 text-accent"
                      : "border-border bg-surface-2 text-text-muted"
                  }`}
                  title={detail.auto_run ? "Auto-run: agent proceeds without pausing" : "Checkpointed: agent pauses at breakpoints"}
                >
                  {detail.auto_run ? "Auto-run" : "Checkpointed"}
                </button>
              )}
              {detail.status === "running" && (
                <button
                  onClick={() => handleCancel(detail.id)}
                  className="rounded-lg border border-danger/20 bg-danger/10 px-2.5 py-1 text-xs text-danger transition-colors hover:bg-danger/20"
                >
                  Cancel
                </button>
              )}
              {workspacePath && (
                <button
                  onClick={handleOpenWorkspace}
                  className="rounded-lg border border-border bg-surface-2 px-2.5 py-1 text-xs text-text-secondary transition-colors hover:bg-surface-3"
                >
                  Open in IDE
                </button>
              )}
              <button
                onClick={() => {
                  const fallback = agents.find((a) => a.kind !== detail.agent)?.kind ?? "codex";
                  setHandoffAgent(fallback);
                  setShowHandoff(true);
                }}
                className="rounded-lg border border-border bg-surface-2 px-2.5 py-1 text-xs text-text-secondary transition-colors hover:bg-surface-3"
              >
                Hand off
              </button>
              {detail.status !== "running" && (
                <button
                  onClick={() => handleArchive(detail.id)}
                  className="rounded-lg border border-border bg-surface-2 px-2.5 py-1 text-xs text-text-secondary transition-colors hover:bg-surface-3"
                >
                  Archive
                </button>
              )}
            </div>
          </div>

          {/* Chat messages area */}
          <div
            ref={outputRef}
            className="flex-1 overflow-y-auto px-6 py-4 space-y-4"
          >
            {/* User message: the prompt */}
            <div className="flex justify-end">
              <div className="max-w-[70%] rounded-2xl rounded-tr-sm bg-accent/15 border border-accent/20 px-4 py-3">
                <p className="whitespace-pre-wrap text-sm text-text-primary">{detail.prompt}</p>
              </div>
            </div>

            {/* Error message if any */}
            {detail.error_message && (
              <div className="flex justify-start">
                <div className="max-w-[80%] rounded-2xl rounded-tl-sm border border-danger/20 bg-danger/5 px-4 py-3">
                  <p className="text-xs uppercase tracking-wider text-danger mb-1">Error</p>
                  <p className="text-sm text-danger/90">{detail.error_message}</p>
                </div>
              </div>
            )}

            {/* Agent response: streaming output */}
            {detail.output ? (
              <div className="flex justify-start">
                <div className="max-w-[85%] rounded-2xl rounded-tl-sm border border-border bg-surface-1 px-4 py-3">
                  {detail.status === "running" && (
                    <div className="flex items-center gap-2 mb-2">
                      <span className="h-1.5 w-1.5 rounded-full bg-accent animate-pulse" />
                      <span className="text-[11px] text-accent">Generating...</span>
                    </div>
                  )}
                  <pre className="whitespace-pre-wrap font-mono text-xs text-text-secondary leading-relaxed">
                    {detail.output}
                  </pre>
                </div>
              </div>
            ) : detail.status === "running" ? (
              <div className="flex justify-start">
                <div className="rounded-2xl rounded-tl-sm border border-border bg-surface-1 px-4 py-3">
                  <div className="flex items-center gap-2">
                    <span className="h-1.5 w-1.5 rounded-full bg-accent animate-pulse" />
                    <span className="text-xs text-text-muted">Agent is working...</span>
                  </div>
                </div>
              </div>
            ) : null}
          </div>

          {/* Input area at bottom */}
          <div className="shrink-0 border-t border-border bg-surface-1/50 px-4 py-3">
            <div className="flex items-end gap-3">
              <div className="flex-1 rounded-xl border border-border bg-surface-0 px-4 py-2.5">
                <p className="text-xs text-text-muted">
                  {detail.status === "running"
                    ? "Agent is working on this task..."
                    : detail.status === "completed"
                      ? "Task completed. Create a new task to continue."
                      : "Task ended."}
                </p>
              </div>
              <div className="flex items-center gap-2 shrink-0">
                <span className="text-xs text-text-muted px-2 py-1 rounded-lg border border-border bg-surface-2">
                  {detail.agent}
                </span>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

function NewTaskModal({
  agents,
  config,
  projects,
  activeProjectPath,
  onSpawn,
  onClose,
}: {
  agents: InstalledAgent[];
  config: DesktopConfig | null;
  projects: ProjectInfo[];
  activeProjectPath?: string;
  onSpawn: (agent: string, prompt: string, worktree: boolean, branch: string) => void;
  onClose: () => void;
}) {
  const [presetName, setPresetName] = useState("");
  const [agent, setAgent] = useState(config?.default_agent || "claude");
  const [prompt, setPrompt] = useState("");
  const [useWorktree, setUseWorktree] = useState(config?.default_worktree ?? true);
  const [branch, setBranch] = useState("");
  const [selectedProject, setSelectedProject] = useState(activeProjectPath || "");

  useEffect(() => {
    if (!config) {
      return;
    }

    setAgent(config.default_agent);
    setUseWorktree(config.default_worktree);
  }, [config]);

  const presets = config?.agent_presets ?? [];

  const handlePresetChange = (value: string) => {
    setPresetName(value);

    const preset = presets.find((item) => item.name === value);
    if (!preset) {
      return;
    }

    setAgent(preset.agent_kind);
  };

  const handleSubmit = () => {
    if (!prompt.trim()) {
      return;
    }

    const branchName = useWorktree ? branch.trim() || `haft-task-${Date.now()}` : "";
    onSpawn(agent, prompt.trim(), useWorktree, branchName);
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
      <div className="max-h-[80vh] w-[720px] overflow-y-auto rounded-2xl border border-border bg-surface-1">
        <div className="flex items-center justify-between border-b border-border px-6 py-4">
          <div>
            <h3 className="text-lg font-semibold">New Task</h3>
            <p className="mt-1 text-xs text-text-muted">
              Start from a preset or launch a one-off task with explicit workspace control.
            </p>
          </div>
          <button
            onClick={onClose}
            className="text-text-muted transition-colors hover:text-text-primary"
          >
            x
          </button>
        </div>

        <div className="space-y-4 px-6 py-5">
          {presets.length > 0 && (
            <div className="space-y-1">
              <label className="text-sm text-text-secondary">Preset</label>
              <select
                value={presetName}
                onChange={(event) => handlePresetChange(event.target.value)}
                className="w-full rounded-lg border border-border bg-surface-2 px-3 py-2 text-sm text-text-primary"
              >
                <option value="">Manual selection</option>
                {presets.map((preset) => (
                  <option key={`${preset.name}-${preset.role}`} value={preset.name}>
                    {preset.name} {preset.model ? `(${preset.model})` : ""}
                  </option>
                ))}
              </select>
            </div>
          )}

          {/* Project selector */}
          {projects.length > 1 && (
            <div className="space-y-1">
              <label className="text-sm text-text-secondary">Project</label>
              <select
                value={selectedProject}
                onChange={(event) => setSelectedProject(event.target.value)}
                className="w-full rounded-lg border border-border bg-surface-2 px-3 py-2 text-sm text-text-primary"
              >
                {projects.map((p) => (
                  <option key={p.path} value={p.path}>
                    {p.name}
                  </option>
                ))}
              </select>
            </div>
          )}

          <div className="rounded-xl border border-border bg-surface-2/40 px-4 py-3">
            <div className="flex flex-wrap items-center gap-3 text-sm">
              <span className="text-text-muted">Run in</span>

              <button
                onClick={() => setUseWorktree(true)}
                className={`rounded border px-3 py-1 text-xs transition-colors ${
                  useWorktree
                    ? "border-accent/30 bg-accent/10 text-accent"
                    : "border-border bg-surface-2 text-text-muted"
                }`}
              >
                Worktree
              </button>

              <button
                onClick={() => setUseWorktree(false)}
                className={`rounded border px-3 py-1 text-xs transition-colors ${
                  !useWorktree
                    ? "border-accent/30 bg-accent/10 text-accent"
                    : "border-border bg-surface-2 text-text-muted"
                }`}
              >
                Project folder
              </button>

              {useWorktree && (
                <>
                  <span className="text-text-muted">branch</span>
                  <input
                    value={branch}
                    onChange={(event) => setBranch(event.target.value)}
                    placeholder="auto-generated"
                    className="w-44 rounded border border-border bg-surface-2 px-2 py-1 font-mono text-xs text-text-primary"
                  />
                </>
              )}
            </div>
          </div>

          <div className="space-y-1">
            <label className="text-sm text-text-secondary">Prompt</label>
            <textarea
              value={prompt}
              onChange={(event) => setPrompt(event.target.value)}
              placeholder="Describe the task. Haft will keep runtime state durable and pass the exact brief through to the agent."
              rows={8}
              className="w-full resize-none rounded-xl border border-border bg-surface-0 px-4 py-3 text-sm text-text-primary focus:border-accent/50 focus:outline-none"
            />
          </div>

          <div className="space-y-1">
            <label className="text-sm text-text-secondary">Agent</label>
            <select
              value={agent}
              onChange={(event) => setAgent(event.target.value)}
              className="w-full rounded-lg border border-border bg-surface-2 px-3 py-2 text-sm text-text-primary"
            >
              {agents.map((item) => (
                <option key={item.kind} value={item.kind}>
                  {item.name} ({item.version})
                </option>
              ))}

              {agents.length === 0 && (
                <>
                  <option value="claude">Claude Code</option>
                  <option value="codex">Codex</option>
                  <option value="haft">Haft Agent</option>
                </>
              )}
            </select>
          </div>
        </div>

        <div className="flex items-center justify-end gap-3 border-t border-border px-6 py-4">
          <button
            onClick={onClose}
            className="px-4 py-2 text-sm text-text-secondary transition-colors hover:text-text-primary"
          >
            Cancel
          </button>

          <button
            onClick={handleSubmit}
            disabled={!prompt.trim()}
            className="rounded-full bg-accent px-4 py-2 text-sm text-surface-0 transition-colors hover:bg-accent-hover disabled:opacity-50"
          >
            Create & Run
          </button>
        </div>
      </div>
    </div>
  );
}

function elapsedSince(isoDate: string): string {
  try {
    const start = new Date(isoDate).getTime();
    const now = Date.now();
    const seconds = Math.floor((now - start) / 1000);
    if (seconds < 60) return `${seconds}s`;
    const minutes = Math.floor(seconds / 60);
    if (minutes < 60) return `${minutes}m ${seconds % 60}s`;
    const hours = Math.floor(minutes / 60);
    return `${hours}h ${minutes % 60}m`;
  } catch {
    return "";
  }
}

function StatusBadge({ status }: { status: string }) {
  const styles: Record<string, string> = {
    running: "border-blue-500/20 bg-blue-500/10 text-blue-400",
    completed: "border-success/20 bg-success/10 text-success",
    failed: "border-danger/20 bg-danger/10 text-danger",
    cancelled: "border-border bg-surface-2 text-text-muted",
    interrupted: "border-warning/20 bg-warning/10 text-warning",
  };

  return (
    <span
      className={`rounded-full border px-2 py-0.5 text-xs ${
        styles[status] ?? "border-border bg-surface-2 text-text-muted"
      }`}
    >
      {status}
    </span>
  );
}

function HandoffModal({
  agents,
  sourceTask,
  targetAgent,
  onChangeTarget,
  onClose,
  onConfirm,
}: {
  agents: InstalledAgent[];
  sourceTask: TaskState;
  targetAgent: string;
  onChangeTarget: (agent: string) => void;
  onClose: () => void;
  onConfirm: () => void;
}) {
  const availableAgents = agents.filter((agent) => agent.kind !== sourceTask.agent);

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 px-4">
      <div className="w-[520px] rounded-2xl border border-border bg-surface-1">
        <div className="flex items-center justify-between border-b border-border px-6 py-4">
          <div>
            <h3 className="text-lg font-semibold">Hand Off Task</h3>
            <p className="mt-1 text-xs text-text-muted">
              Preserve the original brief and bounded output tail, then continue with a different agent.
            </p>
          </div>
          <button
            onClick={onClose}
            className="text-text-muted transition-colors hover:text-text-primary"
          >
            x
          </button>
        </div>

        <div className="space-y-4 px-6 py-5">
          <div className="rounded-xl border border-border bg-surface-2/40 px-4 py-3">
            <p className="text-[11px] uppercase tracking-wider text-text-muted">Source task</p>
            <p className="mt-2 text-sm text-text-primary">{sourceTask.title}</p>
            <div className="mt-2 flex flex-wrap items-center gap-2 text-xs text-text-muted">
              <span>{sourceTask.agent}</span>
              <span>{sourceTask.status}</span>
              {sourceTask.branch && <span className="font-mono">{sourceTask.branch}</span>}
            </div>
          </div>

          <div className="space-y-1">
            <label className="text-sm text-text-secondary">Target agent</label>
            <select
              value={targetAgent}
              onChange={(event) => onChangeTarget(event.target.value)}
              className="w-full rounded-lg border border-border bg-surface-2 px-3 py-2 text-sm text-text-primary"
            >
              {availableAgents.map((agent) => (
                <option key={agent.kind} value={agent.kind}>
                  {agent.name} ({agent.version || agent.kind})
                </option>
              ))}

              {availableAgents.length === 0 && (
                <>
                  <option value="codex">Codex</option>
                  <option value="haft">Haft Agent</option>
                </>
              )}
            </select>
          </div>

          {sourceTask.status === "running" && (
            <div className="rounded-xl border border-warning/20 bg-warning/10 px-4 py-3 text-sm text-warning">
              The source task is still marked running. The handoff will preserve context, but you should
              reconcile workspace state before treating the previous output as final.
            </div>
          )}
        </div>

        <div className="flex items-center justify-end gap-3 border-t border-border px-6 py-4">
          <button
            onClick={onClose}
            className="px-4 py-2 text-sm text-text-secondary transition-colors hover:text-text-primary"
          >
            Cancel
          </button>

          <button
            onClick={onConfirm}
            className="rounded-full bg-accent px-4 py-2 text-sm text-surface-0 transition-colors hover:bg-accent-hover"
          >
            Create handoff
          </button>
        </div>
      </div>
    </div>
  );
}

function mergeTaskList(current: TaskState[], next: TaskState): TaskState[] {
  const existingIndex = current.findIndex((task) => task.id === next.id);

  if (existingIndex === -1) {
    return [next, ...current];
  }

  const merged = [...current];
  merged[existingIndex] = {
    ...merged[existingIndex],
    ...next,
  };

  return merged;
}

// parsePromptSections kept for future use if we add collapsible brief sections to chat
// function parsePromptSections(prompt: string): PromptSection[] { ... }
