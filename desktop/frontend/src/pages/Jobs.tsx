import { useCallback, useEffect, useMemo, useState, type ReactNode } from "react";

import {
  createFlow,
  deleteFlow,
  detectAgents,
  listAllTasks,
  listFlows,
  listFlowTemplates,
  runFlowNow,
  toggleFlow,
  updateFlow,
  type DesktopFlow,
  type FlowInput,
  type FlowTemplate,
  type InstalledAgent,
  type TaskState,
} from "../lib/api";
import { Badge, MonoId, Pill } from "../components/primitives";
import { reportError } from "../lib/errors";
import { taskHasTerminalOutcome, taskIsLive, taskRunState } from "../lib/taskInput.ts";

const BOARD_COLUMNS = ["Running", "Needs Input", "Completed", "Failed"] as const;

type BoardColumn = typeof BOARD_COLUMNS[number];

const EMPTY_FLOW: FlowInput = {
  id: "",
  title: "",
  description: "",
  template_id: "",
  agent: "claude",
  prompt: "",
  schedule: "0 9 * * 1",
  branch: "",
  use_worktree: true,
  enabled: true,
};

export function Flows({
  onOpenTask,
}: {
  onOpenTask: (task: TaskState) => void | Promise<void>;
}) {
  const [flows, setFlows] = useState<DesktopFlow[]>([]);
  const [templates, setTemplates] = useState<FlowTemplate[]>([]);
  const [tasks, setTasks] = useState<TaskState[]>([]);
  const [agents, setAgents] = useState<InstalledAgent[]>([]);
  const [editingFlow, setEditingFlow] = useState<FlowInput | null>(null);

  const refresh = useCallback(async () => {
    try {
      const [nextFlows, nextTemplates, nextTasks, nextAgents] = await Promise.all([
        listFlows(),
        listFlowTemplates(),
        listAllTasks(),
        detectAgents(),
      ]);

      setFlows(nextFlows);
      setTemplates(nextTemplates);
      setTasks(nextTasks);
      setAgents(nextAgents);
    } catch (error) {
      reportError(error, "automation");
    }
  }, []);

  useEffect(() => {
    void refresh();

    const interval = window.setInterval(() => {
      void refresh();
    }, 4000);

    return () => {
      window.clearInterval(interval);
    };
  }, [refresh]);

  const groupedTasks = useMemo(() => groupTasksByBoardColumn(tasks), [tasks]);

  const handleTemplateCreate = (template: FlowTemplate) => {
    setEditingFlow(flowInputFromTemplate(template));
  };

  const handleToggle = async (flow: DesktopFlow) => {
    try {
      const nextFlow = await toggleFlow(flow.id, !flow.enabled);
      setFlows((current) => current.map((item) => (item.id === nextFlow.id ? nextFlow : item)));
    } catch (error) {
      reportError(error, "toggle flow");
    }
  };

  const handleDelete = async (flowID: string) => {
    try {
      await deleteFlow(flowID);
      setFlows((current) => current.filter((item) => item.id !== flowID));
    } catch (error) {
      reportError(error, "delete flow");
    }
  };

  const handleRun = async (flowID: string) => {
    try {
      await runFlowNow(flowID);
      await refresh();
    } catch (error) {
      reportError(error, "run flow");
    }
  };

  const handleSave = async (input: FlowInput) => {
    try {
      const savedFlow = input.id
        ? await updateFlow(input)
        : await createFlow(input);

      setFlows((current) => upsertFlow(current, savedFlow));
      setEditingFlow(null);
      await refresh();
    } catch (error) {
      reportError(error, "save flow");
    }
  };

  return (
    <div className="space-y-8 pb-8">
      <div className="flex flex-col gap-4 xl:flex-row xl:items-end xl:justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Automation</h1>
          <p className="mt-1 max-w-3xl text-sm text-text-muted">
            Reusable job schedules, a cross-project task board, and operator shortcuts anchored in the
            same desktop shell.
          </p>
        </div>

        <button
          onClick={() => setEditingFlow({ ...EMPTY_FLOW })}
          className="rounded-full bg-accent px-4 py-2 text-sm text-surface-0 transition-colors hover:bg-accent-hover"
        >
          + New Job
        </button>
      </div>

      {editingFlow && (
        <FlowModal
          agents={agents}
          templates={templates}
          initialValue={editingFlow}
          onClose={() => setEditingFlow(null)}
          onSave={handleSave}
        />
      )}

      <div className="grid gap-6 xl:grid-cols-[minmax(0,0.85fr)_minmax(0,1.15fr)]">
        <section className="space-y-6">
          <Panel
            title="Templates"
            subtitle="Reusable reasoning and governance automations. Start here, then customize."
          >
            <div className="grid gap-3">
              {templates.map((template) => (
                <button
                  key={template.id}
                  onClick={() => handleTemplateCreate(template)}
                  className="rounded-2xl border border-border bg-surface-1 px-4 py-4 text-left transition-colors hover:border-border-bright hover:bg-surface-2"
                >
                  <div className="flex items-start justify-between gap-4">
                    <div>
                      <h3 className="text-sm font-medium text-text-primary">{template.name}</h3>
                      <p className="mt-1 text-sm text-text-secondary">{template.description}</p>
                    </div>
                    <Pill>{template.agent}</Pill>
                  </div>

                  <div className="mt-3 flex flex-wrap items-center gap-2 text-[11px] text-text-muted">
                    <span className="rounded-full border border-border bg-surface-0 px-2 py-1 font-mono">
                      {template.schedule}
                    </span>
                    {template.use_worktree ? <Pill>worktree</Pill> : null}
                  </div>
                </button>
              ))}
            </div>
          </Panel>

          <Panel
            title="Project Flows"
            subtitle="Editable schedules for the active project. Cron syntax is standard 5-field format."
          >
            <div className="space-y-3">
              {flows.length === 0 && (
                <EmptyState text="No flows yet. Start from a template or create one manually." />
              )}

              {flows.map((flow) => (
                <div
                  key={flow.id}
                  className="rounded-2xl border border-border bg-surface-1 px-4 py-4"
                >
                  <div className="flex flex-col gap-3 xl:flex-row xl:items-start xl:justify-between">
                    <div className="space-y-2">
                      <div className="flex flex-wrap items-center gap-2">
                        <h3 className="text-sm font-medium text-text-primary">{flow.title}</h3>
                        <StatusChip active={flow.enabled} />
                        <Pill>{flow.agent}</Pill>
                      </div>
                      {flow.description && (
                        <p className="text-sm text-text-secondary">{flow.description}</p>
                      )}
                      <div className="flex flex-wrap items-center gap-2 text-[11px] text-text-muted">
                        <span className="rounded-full border border-border bg-surface-0 px-2 py-1 font-mono">
                          {flow.schedule}
                        </span>
                        {flow.branch ? <MonoId id={flow.branch} /> : null}
                        {flow.use_worktree ? <Pill>worktree</Pill> : null}
                      </div>
                    </div>

                    <div className="flex flex-wrap items-center gap-2">
                      <button
                        onClick={() => void handleRun(flow.id)}
                        className="rounded-lg border border-accent/20 bg-accent/10 px-3 py-2 text-xs text-accent transition-colors hover:bg-accent/20"
                      >
                        Run now
                      </button>
                      <button
                        onClick={() => setEditingFlow(flowInputFromFlow(flow))}
                        className="rounded-lg border border-border bg-surface-2 px-3 py-2 text-xs text-text-secondary transition-colors hover:bg-surface-3"
                      >
                        Edit
                      </button>
                      <button
                        onClick={() => void handleToggle(flow)}
                        className="rounded-lg border border-border bg-surface-2 px-3 py-2 text-xs text-text-secondary transition-colors hover:bg-surface-3"
                      >
                        {flow.enabled ? "Pause" : "Enable"}
                      </button>
                      <button
                        onClick={() => void handleDelete(flow.id)}
                        className="rounded-lg border border-danger/20 bg-danger/10 px-3 py-2 text-xs text-danger transition-colors hover:bg-danger/20"
                      >
                        Delete
                      </button>
                    </div>
                  </div>

                  <div className="mt-4 grid gap-3 md:grid-cols-3">
                    <Fact label="Last run" value={formatDateTime(flow.last_run_at)} />
                    <Fact label="Next run" value={formatDateTime(flow.next_run_at)} />
                    <Fact label="Last task" value={flow.last_task_id || "Not run yet"} mono />
                  </div>

                  {flow.last_error && (
                    <div className="mt-3 rounded-xl border border-danger/20 bg-danger/5 px-3 py-3 text-sm text-danger/90">
                      {flow.last_error}
                    </div>
                  )}
                </div>
              ))}
            </div>
          </Panel>
        </section>

        <Panel
          title="Cross-Project Board"
          subtitle="Persisted tasks from every registered project, grouped by operator-facing status."
        >
          <div className="grid gap-4 xl:grid-cols-4">
            {BOARD_COLUMNS.map((column) => (
              <div
                key={column}
                className="rounded-2xl border border-border bg-surface-1/70 px-3 py-3"
              >
                <div className="mb-3 flex items-center justify-between gap-2">
                  <h3 className="text-xs font-semibold uppercase tracking-[0.22em] text-text-muted">
                    {column}
                  </h3>
                  <span className="rounded-full border border-border bg-surface-2 px-2 py-1 text-[11px] text-text-muted">
                    {groupedTasks[column].length}
                  </span>
                </div>

                <div className="space-y-3">
                  {groupedTasks[column].length === 0 && (
                    <EmptyState text="No tasks in this column." compact />
                  )}

                  {groupedTasks[column].map((task) => (
                    <button
                      key={task.id}
                      onClick={() => void onOpenTask(task)}
                      className="w-full rounded-2xl border border-border bg-surface-0 px-3 py-3 text-left transition-colors hover:border-border-bright hover:bg-surface-2"
                    >
                      <div className="flex items-start justify-between gap-3">
                        <div className="min-w-0">
                          <p className="truncate text-sm font-medium text-text-primary">{task.title}</p>
                          <p className="mt-1 text-xs text-text-muted">{task.project}</p>
                        </div>
                        <span className="rounded-full border border-border bg-surface-1 px-2 py-1 text-[11px] text-text-muted">
                          {task.agent}
                        </span>
                      </div>

                      <p className="mt-2 line-clamp-3 text-sm text-text-secondary">
                        {task.error_message || firstBoardLine(task.output) || task.prompt}
                      </p>

                      <div className="mt-3 flex flex-wrap items-center gap-2 text-[11px] text-text-muted">
                        {task.branch && <span className="font-mono">{task.branch}</span>}
                        <span>{formatDateTime(task.started_at)}</span>
                      </div>
                    </button>
                  ))}
                </div>
              </div>
            ))}
          </div>
        </Panel>
      </div>
    </div>
  );
}

function FlowModal({
  agents,
  templates,
  initialValue,
  onClose,
  onSave,
}: {
  agents: InstalledAgent[];
  templates: FlowTemplate[];
  initialValue: FlowInput;
  onClose: () => void;
  onSave: (input: FlowInput) => void | Promise<void>;
}) {
  const [form, setForm] = useState<FlowInput>(initialValue);

  useEffect(() => {
    setForm(initialValue);
  }, [initialValue]);

  const handleTemplateChange = (templateID: string) => {
    const nextTemplate = templates.find((template) => template.id === templateID);
    if (!nextTemplate) {
      setForm((current) => ({ ...current, template_id: "" }));
      return;
    }

    setForm((current) => ({
      ...current,
      title: current.id ? current.title : nextTemplate.name,
      description: current.id ? current.description : nextTemplate.description,
      template_id: nextTemplate.id,
      agent: nextTemplate.agent,
      prompt: nextTemplate.prompt,
      schedule: nextTemplate.schedule,
      branch: nextTemplate.branch,
      use_worktree: nextTemplate.use_worktree,
    }));
  };

  const handleSubmit = () => {
    if (!form.title.trim() || !form.prompt.trim() || !form.schedule.trim()) {
      return;
    }

    void onSave({
      ...form,
      title: form.title.trim(),
      description: form.description.trim(),
      prompt: form.prompt.trim(),
      schedule: form.schedule.trim(),
      branch: form.branch.trim(),
    });
  };

  const availableAgents = agents.length > 0
    ? agents
    : [
        { kind: "claude", name: "Claude Code", path: "", version: "" },
        { kind: "codex", name: "Codex", path: "", version: "" },
      ];

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 px-4">
      <div className="max-h-[88vh] w-[840px] overflow-y-auto rounded-2xl border border-border bg-surface-1">
        <div className="flex items-center justify-between border-b border-border px-6 py-4">
          <div>
            <h3 className="text-lg font-semibold">{form.id ? "Edit Job" : "New Job"}</h3>
            <p className="mt-1 text-xs text-text-muted">
              Standard cron syntax: minute hour day-of-month month day-of-week.
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
          <div className="grid gap-4 md:grid-cols-2">
            <Field label="Template">
              <select
                value={form.template_id}
                onChange={(event) => handleTemplateChange(event.target.value)}
                className="w-full rounded-lg border border-border bg-surface-2 px-3 py-2 text-sm text-text-primary"
              >
                <option value="">Custom</option>
                {templates.map((template) => (
                  <option key={template.id} value={template.id}>
                    {template.name}
                  </option>
                ))}
              </select>
            </Field>

            <Field label="Agent">
              <select
                value={form.agent}
                onChange={(event) => setForm((current) => ({ ...current, agent: event.target.value }))}
                className="w-full rounded-lg border border-border bg-surface-2 px-3 py-2 text-sm text-text-primary"
              >
                {availableAgents.map((agent) => (
                  <option key={agent.kind} value={agent.kind}>
                    {agent.name}
                  </option>
                ))}
              </select>
            </Field>
          </div>

          <div className="grid gap-4 md:grid-cols-[minmax(0,1fr)_14rem]">
            <Field label="Title">
              <input
                value={form.title}
                onChange={(event) => setForm((current) => ({ ...current, title: event.target.value }))}
                className="w-full rounded-lg border border-border bg-surface-0 px-3 py-2 text-sm text-text-primary"
              />
            </Field>

            <Field label="Cron schedule">
              <input
                value={form.schedule}
                onChange={(event) => setForm((current) => ({ ...current, schedule: event.target.value }))}
                className="w-full rounded-lg border border-border bg-surface-0 px-3 py-2 font-mono text-sm text-text-primary"
              />
            </Field>
          </div>

          <Field label="Description">
            <textarea
              value={form.description}
              onChange={(event) => setForm((current) => ({ ...current, description: event.target.value }))}
              rows={3}
              className="w-full resize-none rounded-xl border border-border bg-surface-0 px-4 py-3 text-sm text-text-primary"
            />
          </Field>

          <div className="grid gap-4 md:grid-cols-[minmax(0,1fr)_12rem_auto]">
            <Field label="Branch">
              <input
                value={form.branch}
                onChange={(event) => setForm((current) => ({ ...current, branch: event.target.value }))}
                className="w-full rounded-lg border border-border bg-surface-0 px-3 py-2 font-mono text-sm text-text-primary"
              />
            </Field>

            <Field label="Workspace">
              <button
                onClick={() => setForm((current) => ({ ...current, use_worktree: !current.use_worktree }))}
                className={`w-full rounded-lg border px-3 py-2 text-sm transition-colors ${
                  form.use_worktree
                    ? "border-accent/30 bg-accent/10 text-accent"
                    : "border-border bg-surface-2 text-text-muted"
                }`}
              >
                {form.use_worktree ? "Worktree" : "Project folder"}
              </button>
            </Field>

            <Field label="Enabled">
              <button
                onClick={() => setForm((current) => ({ ...current, enabled: !current.enabled }))}
                className={`w-full rounded-lg border px-3 py-2 text-sm transition-colors ${
                  form.enabled
                    ? "border-success/20 bg-success/10 text-success"
                    : "border-border bg-surface-2 text-text-muted"
                }`}
              >
                {form.enabled ? "Enabled" : "Paused"}
              </button>
            </Field>
          </div>

          <Field label="Prompt">
            <textarea
              value={form.prompt}
              onChange={(event) => setForm((current) => ({ ...current, prompt: event.target.value }))}
              rows={10}
              className="w-full resize-none rounded-xl border border-border bg-surface-0 px-4 py-3 text-sm text-text-primary"
            />
          </Field>
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
            disabled={!form.title.trim() || !form.prompt.trim() || !form.schedule.trim()}
            className="rounded-full bg-accent px-4 py-2 text-sm text-surface-0 transition-colors hover:bg-accent-hover disabled:opacity-50"
          >
            Save Flow
          </button>
        </div>
      </div>
    </div>
  );
}

function Panel({
  title,
  subtitle,
  children,
}: {
  title: string;
  subtitle: string;
  children: ReactNode;
}) {
  return (
    <section className="rounded-[1.75rem] border border-border bg-surface-1/80 px-5 py-5">
      <div className="mb-4">
        <h2 className="text-sm font-medium text-text-primary">{title}</h2>
        <p className="mt-1 text-xs text-text-muted">{subtitle}</p>
      </div>

      {children}
    </section>
  );
}

function Field({
  label,
  children,
}: {
  label: string;
  children: ReactNode;
}) {
  return (
    <label className="space-y-1">
      <span className="text-sm text-text-secondary">{label}</span>
      {children}
    </label>
  );
}

function Fact({
  label,
  value,
  mono,
}: {
  label: string;
  value: string;
  mono?: boolean;
}) {
  return (
    <div className="rounded-xl border border-border bg-surface-2/40 px-3 py-3">
      <p className="text-[11px] uppercase tracking-wider text-text-muted">{label}</p>
      <p className={`mt-2 text-sm text-text-primary ${mono ? "font-mono break-all" : ""}`}>
        {value}
      </p>
    </div>
  );
}

function StatusChip({ active }: { active: boolean }) {
  return (
    <Badge tone={active ? "success" : "neutral"}>
      {active ? "Live" : "Paused"}
    </Badge>
  );
}

function EmptyState({
  text,
  compact,
}: {
  text: string;
  compact?: boolean;
}) {
  return (
    <div className={`rounded-2xl border border-dashed border-border bg-surface-0/60 px-4 text-center ${compact ? "py-6" : "py-10"}`}>
      <p className="text-sm text-text-muted">{text}</p>
    </div>
  );
}

function flowInputFromTemplate(template: FlowTemplate): FlowInput {
  return {
    ...EMPTY_FLOW,
    title: template.name,
    description: template.description,
    template_id: template.id,
    agent: template.agent,
    prompt: template.prompt,
    schedule: template.schedule,
    branch: template.branch,
    use_worktree: template.use_worktree,
  };
}

function flowInputFromFlow(flow: DesktopFlow): FlowInput {
  return {
    id: flow.id,
    title: flow.title,
    description: flow.description,
    template_id: flow.template_id,
    agent: flow.agent,
    prompt: flow.prompt,
    schedule: flow.schedule,
    branch: flow.branch,
    use_worktree: flow.use_worktree,
    enabled: flow.enabled,
  };
}

function upsertFlow(current: DesktopFlow[], nextFlow: DesktopFlow): DesktopFlow[] {
  const withoutCurrent = current.filter((flow) => flow.id !== nextFlow.id);
  return [nextFlow, ...withoutCurrent];
}

function groupTasksByBoardColumn(tasks: TaskState[]): Record<BoardColumn, TaskState[]> {
  return BOARD_COLUMNS.reduce<Record<BoardColumn, TaskState[]>>((result, column) => {
    result[column] = tasks.filter((task) => taskBoardColumn(task) === column);
    return result;
  }, {
    Running: [],
    "Needs Input": [],
    Completed: [],
    Failed: [],
  });
}

function taskBoardColumn(task: TaskState): BoardColumn {
  const state = taskRunState(task.status);

  if (taskIsLive(state)) {
    return "Running";
  }

  if (taskHasTerminalOutcome(state, "completed")) {
    return "Completed";
  }

  if (taskHasTerminalOutcome(state, "failed")) {
    return "Failed";
  }

  return "Needs Input";
}

function formatDateTime(value: string): string {
  if (!value) {
    return "Not yet";
  }

  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }

  return date.toLocaleString();
}

function firstBoardLine(value: string): string {
  return value.split("\n").map((line) => line.trim()).find(Boolean) ?? "";
}
