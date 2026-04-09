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

interface PromptSection {
  title: string;
  body: string;
}

export function Tasks({
  selectedTaskId: externalSelectedTask,
  showNewTask: externalShow,
  onNewTaskClose,
}: {
  selectedTaskId?: string | null;
  showNewTask?: boolean;
  onNewTaskClose?: () => void;
} = {}) {
  const [tasks, setTasks] = useState<TaskState[]>([]);
  const [agents, setAgents] = useState<InstalledAgent[]>([]);
  const [config, setConfig] = useState<DesktopConfig | null>(null);
  const [internalShow, setInternalShow] = useState(false);
  const [selectedTask, setSelectedTask] = useState<string | null>(externalSelectedTask ?? null);
  const [showHandoff, setShowHandoff] = useState(false);
  const [handoffAgent, setHandoffAgent] = useState("codex");
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
  const promptSections = detail ? parsePromptSections(detail.prompt) : [];
  const workspacePath = detail ? detail.worktree_path || detail.project_path : "";

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

  const handleCopy = async (value: string, label: string) => {
    if (!value) {
      return;
    }

    if (!navigator.clipboard?.writeText) {
      reportError(`Clipboard is not available for ${label}.`, "clipboard");
      return;
    }

    try {
      await navigator.clipboard.writeText(value);
    } catch (error) {
      reportError(error, `copy ${label}`);
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Tasks</h1>
          <p className="mt-1 text-sm text-text-muted">
            Run agents with durable task state, worktree-aware actions, and a visible reasoning brief.
          </p>
        </div>

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

      <div className="grid gap-6 xl:grid-cols-[22rem_minmax(0,1fr)]">
        <div className="space-y-2">
          {tasks.length === 0 && (
            <div className="rounded-2xl border border-dashed border-border bg-surface-1/60 px-6 py-12 text-center">
              <p className="text-sm text-text-muted">No tasks yet</p>
              <p className="mt-1 text-xs text-text-muted">
                Start a task to see live output, workspace actions, and the injected brief.
              </p>
            </div>
          )}

          {tasks.map((task) => (
            <button
              key={task.id}
              onClick={() => setSelectedTask(task.id)}
              className={`w-full rounded-2xl border px-4 py-3 text-left transition-colors ${
                selectedTask === task.id
                  ? "border-accent/30 bg-surface-2"
                  : "border-transparent bg-surface-1 hover:bg-surface-2"
              }`}
            >
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                  <span className="block truncate text-sm font-medium">{task.title}</span>
                  <div className="mt-1 flex flex-wrap items-center gap-2 text-xs text-text-muted">
                    <span>{task.agent}</span>
                    {task.branch && <span className="font-mono">{task.branch}</span>}
                  </div>
                </div>
                <StatusDot status={task.status} />
              </div>

              <p className="mt-2 line-clamp-2 text-xs text-text-muted">
                {firstNonEmptyLine(task.output) || task.prompt}
              </p>

              {task.error_message && (
                <p className="mt-2 line-clamp-2 text-xs text-danger/80">{task.error_message}</p>
              )}
            </button>
          ))}
        </div>

        {detail && (
          <div className="space-y-4">
            <div className="rounded-2xl border border-border bg-surface-1 px-5 py-5">
              <div className="flex flex-col gap-4 xl:flex-row xl:items-start xl:justify-between">
                <div className="space-y-2">
                  <div className="flex flex-wrap items-center gap-2 text-xs text-text-muted">
                    <StatusBadge status={detail.status} />
                    <span>{detail.agent}</span>
                    <span>{detail.project}</span>
                    {detail.branch && <span className="font-mono">{detail.branch}</span>}
                  </div>
                  <h3 className="text-xl font-semibold text-text-primary">{detail.title}</h3>
                  <p className="max-w-3xl text-sm text-text-muted">
                    {describeTask(detail)}
                  </p>
                </div>

                <div className="flex flex-wrap items-center gap-2">
                  {detail.status === "running" && (
                    <button
                      onClick={() => handleCancel(detail.id)}
                      className="rounded-lg border border-danger/20 bg-danger/10 px-3 py-2 text-xs text-danger transition-colors hover:bg-danger/20"
                    >
                      Cancel
                    </button>
                  )}

                  {detail.status !== "running" && (
                    <button
                      onClick={() => handleArchive(detail.id)}
                      className="rounded-lg border border-border bg-surface-2 px-3 py-2 text-xs text-text-secondary transition-colors hover:bg-surface-3"
                    >
                      Archive
                    </button>
                  )}

                  {workspacePath && (
                    <button
                      onClick={handleOpenWorkspace}
                      className="rounded-lg border border-border bg-surface-2 px-3 py-2 text-xs text-text-secondary transition-colors hover:bg-surface-3"
                    >
                      Open workspace
                    </button>
                  )}

                  <button
                    onClick={() => void handleCopy(detail.prompt, "task brief")}
                    className="rounded-lg border border-border bg-surface-2 px-3 py-2 text-xs text-text-secondary transition-colors hover:bg-surface-3"
                  >
                    Copy brief
                  </button>

                  <button
                    onClick={() => {
                      const fallback = agents.find((agent) => agent.kind !== detail.agent)?.kind ?? "codex";
                      setHandoffAgent(fallback);
                      setShowHandoff(true);
                    }}
                    className="rounded-lg border border-border bg-surface-2 px-3 py-2 text-xs text-text-secondary transition-colors hover:bg-surface-3"
                  >
                    Hand off
                  </button>

                  <button
                    onClick={() => void handleCopy(detail.output, "task output")}
                    className="rounded-lg border border-border bg-surface-2 px-3 py-2 text-xs text-text-secondary transition-colors hover:bg-surface-3"
                  >
                    Copy output
                  </button>
                </div>
              </div>

              <div className="mt-5 grid gap-3 md:grid-cols-2 xl:grid-cols-4">
                <TaskFact label="Started" value={detail.started_at} />
                <TaskFact label="Completed" value={detail.completed_at || "Still running"} />
                <TaskFact
                  label="Workspace"
                  value={
                    detail.worktree
                      ? detail.reused_worktree
                        ? "Reused worktree"
                        : "Fresh worktree"
                      : "Project folder"
                  }
                />
                <TaskFact label="Path" value={workspacePath || "Not available"} mono />
              </div>
            </div>

            {detail.error_message && (
              <div className="rounded-2xl border border-danger/20 bg-danger/5 px-5 py-4">
                <h4 className="mb-1 text-xs uppercase tracking-wider text-danger">Task error</h4>
                <p className="text-sm text-danger/90">{detail.error_message}</p>
              </div>
            )}

            <div className="grid gap-4 xl:grid-cols-[minmax(0,0.92fr)_minmax(0,1.08fr)]">
              <div className="rounded-2xl border border-border bg-surface-1 px-5 py-4">
                <div className="mb-4 flex items-center justify-between gap-3">
                  <div>
                    <h4 className="text-sm font-medium text-text-primary">Injected task brief</h4>
                    <p className="mt-1 text-xs text-text-muted">
                      The exact reasoning context and operator instructions passed to the agent.
                    </p>
                  </div>
                  <span className="rounded-full border border-border bg-surface-2 px-2 py-1 text-[11px] text-text-muted">
                    {promptSections.length} section{promptSections.length === 1 ? "" : "s"}
                  </span>
                </div>

                <div className="space-y-3">
                  {promptSections.map((section) => (
                    <div
                      key={`${detail.id}-${section.title}`}
                      className="rounded-xl border border-border bg-surface-2/50 px-4 py-3"
                    >
                      <h5 className="text-xs uppercase tracking-wider text-text-muted">
                        {section.title}
                      </h5>
                      <p className="mt-2 whitespace-pre-wrap text-sm text-text-secondary">
                        {section.body}
                      </p>
                    </div>
                  ))}
                </div>
              </div>

              <div className="rounded-2xl border border-border bg-surface-1 px-5 py-4">
                <div className="mb-4 flex items-center justify-between gap-3">
                  <div>
                    <h4 className="text-sm font-medium text-text-primary">Streaming output</h4>
                    <p className="mt-1 text-xs text-text-muted">
                      Bounded live tail with persisted recovery on reload.
                    </p>
                  </div>
                  <span
                    className={`rounded-full border px-2 py-1 text-[11px] ${
                      detail.status === "running"
                        ? "border-accent/20 bg-accent/10 text-accent"
                        : "border-border bg-surface-2 text-text-muted"
                    }`}
                  >
                    {detail.status === "running" ? "Live" : "Snapshot"}
                  </span>
                </div>

                <pre
                  ref={outputRef}
                  className="max-h-[38rem] overflow-x-auto overflow-y-auto whitespace-pre-wrap rounded-xl border border-border bg-surface-0 px-4 py-3 font-mono text-xs text-text-secondary"
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
            className="rounded-lg bg-accent px-4 py-2 text-sm text-white transition-colors hover:bg-accent-hover disabled:opacity-50"
          >
            Create & Run
          </button>
        </div>
      </div>
    </div>
  );
}

function TaskFact({
  label,
  value,
  mono,
}: {
  label: string;
  value: string;
  mono?: boolean;
}) {
  return (
    <div className="rounded-xl border border-border bg-surface-2/40 px-4 py-3">
      <p className="text-[11px] uppercase tracking-wider text-text-muted">{label}</p>
      <p className={`mt-2 text-sm text-text-primary ${mono ? "font-mono break-all" : ""}`}>
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

  return <span className={`mt-1 h-2 w-2 rounded-full ${colors[status] ?? "bg-text-muted"}`} />;
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
            className="rounded-lg bg-accent px-4 py-2 text-sm text-white transition-colors hover:bg-accent-hover"
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

function firstNonEmptyLine(value: string): string {
  return value
    .split("\n")
    .map((line) => line.trim())
    .find((line) => line.length > 0) || "";
}

function describeTask(task: TaskState): string {
  if (task.worktree && task.worktree_path) {
    return `Running against ${task.reused_worktree ? "a reused" : "a dedicated"} worktree at ${task.worktree_path}.`;
  }

  if (task.project_path) {
    return `Running directly in the active project at ${task.project_path}.`;
  }

  return "Task workspace has not been assigned yet.";
}

function parsePromptSections(prompt: string): PromptSection[] {
  const normalizedPrompt = prompt.trim();
  if (!normalizedPrompt) {
    return [{ title: "Prompt", body: "No prompt captured." }];
  }

  if (!normalizedPrompt.startsWith("## ")) {
    return [{ title: "Prompt", body: normalizedPrompt }];
  }

  return normalizedPrompt
    .split(/\n(?=## )/)
    .map((chunk) => chunk.trim())
    .filter((chunk) => chunk.length > 0)
    .map((chunk) => {
      const lines = chunk.split("\n");
      const title = lines[0].replace(/^##\s+/, "").trim() || "Prompt";
      const body = lines.slice(1).join("\n").trim() || "No additional details.";

      return { title, body };
    });
}
