import { useCallback, useEffect, useRef, useState } from "react";

import { EventsOn } from "../../wailsjs/runtime/runtime";
import {
  archiveTask,
  cancelTask,
  detectAgents,
  getConfig,
  getTaskOutput,
  listTasks,
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

export function Tasks({
  showNewTask: externalShow,
  onNewTaskClose,
}: {
  showNewTask?: boolean;
  onNewTaskClose?: () => void;
} = {}) {
  const [tasks, setTasks] = useState<TaskState[]>([]);
  const [agents, setAgents] = useState<InstalledAgent[]>([]);
  const [config, setConfig] = useState<DesktopConfig | null>(null);
  const [internalShow, setInternalShow] = useState(false);
  const [selectedTask, setSelectedTask] = useState<string | null>(null);
  const outputRef = useRef<HTMLPreElement | null>(null);

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

  useEffect(() => {
    if (!detail || !outputRef.current) {
      return;
    }

    outputRef.current.scrollTop = outputRef.current.scrollHeight;
  }, [detail]);

  const handleSpawn = async (agent: string, prompt: string, worktree: boolean, branch: string) => {
    try {
      const task = await spawnTask(agent, prompt, worktree, branch);

      setTasks((current) => mergeTaskList(current, task));
      setSelectedTask(task.id);
      setShowNewTask(false);
      void refresh();
    } catch (error) {
      reportError(error, "spawn task");
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

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold tracking-tight">Tasks</h1>
        <button
          onClick={() => setShowNewTask(true)}
          className="rounded-lg bg-accent px-4 py-2 text-sm text-white transition-colors hover:bg-accent-hover"
        >
          + New Task
        </button>
      </div>

      {showNewTask && (
        <NewTaskModal
          agents={agents}
          config={config}
          onSpawn={handleSpawn}
          onClose={() => setShowNewTask(false)}
        />
      )}

      <div className="flex gap-6">
        <div className="w-96 shrink-0 space-y-2">
          {tasks.length === 0 && (
            <div className="py-12 text-center">
              <p className="text-sm text-text-muted">No tasks yet</p>
              <p className="mt-1 text-xs text-text-muted">
                Click "+ New Task" to spawn an agent
              </p>
            </div>
          )}

          {tasks.map((task) => (
            <button
              key={task.id}
              onClick={() => setSelectedTask(task.id)}
              className={`w-full rounded-lg border px-4 py-3 text-left transition-colors ${
                selectedTask === task.id
                  ? "border-accent/30 bg-surface-2"
                  : "border-transparent bg-surface-1 hover:bg-surface-2"
              }`}
            >
              <div className="flex items-center justify-between">
                <span className="truncate text-sm font-medium">{task.title}</span>
                <StatusDot status={task.status} />
              </div>

              <div className="mt-1 flex items-center gap-2 text-xs text-text-muted">
                <span>{task.agent}</span>
                {task.branch && <span className="font-mono">{task.branch}</span>}
              </div>

              {task.error_message && (
                <p className="mt-2 line-clamp-2 text-xs text-danger/80">{task.error_message}</p>
              )}
            </button>
          ))}
        </div>

        {detail && (
          <div className="flex-1 overflow-y-auto">
            <div className="space-y-4">
              <div className="flex items-start justify-between gap-4">
                <div>
                  <h3 className="text-lg font-semibold">{detail.title}</h3>

                  <div className="mt-1 flex flex-wrap items-center gap-2 text-xs text-text-muted">
                    <StatusBadge status={detail.status} />
                    <span>{detail.agent}</span>
                    <span>{detail.started_at}</span>
                    {detail.completed_at && <span>completed {detail.completed_at}</span>}
                  </div>
                </div>

                <div className="flex items-center gap-2">
                  {detail.status === "running" && (
                    <button
                      onClick={() => handleCancel(detail.id)}
                      className="rounded border border-danger/20 bg-danger/10 px-3 py-1.5 text-xs text-danger transition-colors hover:bg-danger/20"
                    >
                      Cancel
                    </button>
                  )}

                  {detail.status !== "running" && (
                    <button
                      onClick={() => handleArchive(detail.id)}
                      className="rounded border border-border bg-surface-2 px-3 py-1.5 text-xs text-text-secondary transition-colors hover:bg-surface-3"
                    >
                      Archive
                    </button>
                  )}
                </div>
              </div>

              <MetaRow label="Prompt" value={detail.prompt} multiline />
              <MetaRow label="Branch" value={detail.branch || "Not set"} />

              {detail.worktree && (
                <MetaRow
                  label="Worktree"
                  value={
                    detail.worktree_path
                      ? `${detail.worktree_path}${detail.reused_worktree ? " (reused)" : ""}`
                      : "Enabled"
                  }
                />
              )}

              {detail.error_message && (
                <div className="rounded-lg border border-danger/20 bg-danger/5 px-4 py-3">
                  <h4 className="mb-1 text-xs uppercase tracking-wider text-danger">Error</h4>
                  <p className="text-sm text-danger/90">{detail.error_message}</p>
                </div>
              )}

              <div>
                <h4 className="mb-1 text-xs uppercase tracking-wider text-text-muted">Output</h4>
                <pre
                  ref={outputRef}
                  className="max-h-[32rem] overflow-x-auto overflow-y-auto whitespace-pre-wrap rounded-lg border border-border bg-surface-1 px-4 py-3 font-mono text-xs text-text-secondary"
                >
                  {detail.output || "Waiting for task output..."}
                </pre>
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

function NewTaskModal({
  agents,
  config,
  onSpawn,
  onClose,
}: {
  agents: InstalledAgent[];
  config: DesktopConfig | null;
  onSpawn: (agent: string, prompt: string, worktree: boolean, branch: string) => void;
  onClose: () => void;
}) {
  const [presetName, setPresetName] = useState("");
  const [agent, setAgent] = useState(config?.default_agent || "claude");
  const [prompt, setPrompt] = useState("");
  const [useWorktree, setUseWorktree] = useState(config?.default_worktree ?? true);
  const [branch, setBranch] = useState("");

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

    const branchName = branch.trim() || `haft-task-${Date.now()}`;
    onSpawn(agent, prompt.trim(), useWorktree, branchName);
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
      <div className="max-h-[80vh] w-[680px] overflow-y-auto rounded-xl border border-border bg-surface-1">
        <div className="flex items-center justify-between border-b border-border px-6 py-4">
          <h3 className="text-lg font-semibold">New Task</h3>
          <button onClick={onClose} className="text-text-muted transition-colors hover:text-text-primary">
            x
          </button>
        </div>

        <div className="space-y-4 px-6 py-4">
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

          <div className="flex items-center gap-3 text-sm">
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

          <div className="space-y-1">
            <label className="text-sm text-text-secondary">Prompt</label>
            <textarea
              value={prompt}
              onChange={(event) => setPrompt(event.target.value)}
              placeholder="Describe the task. Haft will keep the runtime state durable and inject reasoning tooling when configured."
              rows={7}
              className="w-full resize-none rounded-lg border border-border bg-surface-0 px-4 py-3 text-sm text-text-primary focus:border-accent/50 focus:outline-none"
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
            className="rounded-lg bg-accent px-4 py-2 text-sm text-white transition-colors hover:bg-accent-hover disabled:opacity-50"
          >
            Create & Run
          </button>
        </div>
      </div>
    </div>
  );
}

function MetaRow({
  label,
  value,
  multiline,
}: {
  label: string;
  value: string;
  multiline?: boolean;
}) {
  return (
    <div>
      <h4 className="mb-1 text-xs uppercase tracking-wider text-text-muted">{label}</h4>
      <p
        className={`rounded-lg border border-border bg-surface-1 px-4 py-3 text-sm text-text-secondary ${
          multiline ? "whitespace-pre-wrap" : "break-all"
        }`}
      >
        {value}
      </p>
    </div>
  );
}

function StatusDot({ status }: { status: string }) {
  const colors: Record<string, string> = {
    running: "bg-blue-400 animate-pulse",
    completed: "bg-success",
    failed: "bg-danger",
    cancelled: "bg-text-muted",
    interrupted: "bg-warning",
    pending: "bg-warning",
  };

  return <span className={`h-2 w-2 rounded-full ${colors[status] ?? "bg-text-muted"}`} />;
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
    <span className={`rounded-full border px-2 py-0.5 text-xs ${styles[status] ?? "border-border bg-surface-2 text-text-muted"}`}>
      {status}
    </span>
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
