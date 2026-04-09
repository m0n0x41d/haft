import { useEffect, useState, type ReactNode } from "react";

import {
  addProject,
  getConfig,
  listProjects,
  saveConfig,
  scanForProjects,
  type AgentPreset,
  type DesktopConfig,
  type ProjectInfo,
} from "../lib/api";
import { reportError } from "../lib/errors";

type SettingsTab = "general" | "projects" | "agents" | "mcp";

export function Settings() {
  const [tab, setTab] = useState<SettingsTab>("general");
  const [config, setConfig] = useState<DesktopConfig | null>(null);
  const [saving, setSaving] = useState(false);
  const [dirty, setDirty] = useState(false);

  useEffect(() => {
    getConfig()
      .then(setConfig)
      .catch((error) => {
        reportError(error, "settings");
      });
  }, []);

  const updateConfig = (next: DesktopConfig) => {
    setConfig(next);
    setDirty(true);
  };

  const persistConfig = async () => {
    if (!config) {
      return;
    }

    setSaving(true);

    try {
      const saved = await saveConfig(config);
      setConfig(saved);
      setDirty(false);
    } catch (error) {
      reportError(error, "settings");
    } finally {
      setSaving(false);
    }
  };

  const showSaveBar = (tab === "general" || tab === "agents") && config;

  return (
    <div className="flex gap-6 h-[calc(100vh-7rem)]">
      <div className="w-44 shrink-0 space-y-0.5">
        {(
          [
            { id: "general", label: "General" },
            { id: "projects", label: "Projects" },
            { id: "agents", label: "Agents" },
            { id: "mcp", label: "MCP Servers" },
          ] as { id: SettingsTab; label: string }[]
        ).map((item) => (
          <button
            key={item.id}
            onClick={() => setTab(item.id)}
            className={`w-full text-left px-3 py-2 rounded-lg text-sm transition-colors ${
              tab === item.id
                ? "bg-surface-2 text-text-primary"
                : "text-text-secondary hover:bg-surface-2/50"
            }`}
          >
            {item.label}
          </button>
        ))}
      </div>

      <div className="flex-1 overflow-y-auto space-y-6">
        {showSaveBar && (
          <div className="flex items-center justify-between rounded-xl border border-border bg-surface-1 px-4 py-3">
            <div>
              <p className="text-sm text-text-primary">Desktop config</p>
              <p className="text-xs text-text-muted">
                {dirty ? "Unsaved changes" : "Saved to ~/.haft/desktop-config.json"}
              </p>
            </div>

            <button
              onClick={persistConfig}
              disabled={!dirty || saving}
              className="rounded-lg bg-accent px-4 py-2 text-sm text-white transition-colors hover:bg-accent-hover disabled:opacity-50"
            >
              {saving ? "Saving..." : "Save"}
            </button>
          </div>
        )}

        {tab === "general" && config && (
          <GeneralSettings config={config} onChange={updateConfig} />
        )}
        {tab === "projects" && <ProjectSettings />}
        {tab === "agents" && config && (
          <AgentSettings config={config} onChange={updateConfig} />
        )}
        {tab === "mcp" && config && <MCPSettings config={config} />}
      </div>
    </div>
  );
}

function GeneralSettings({
  config,
  onChange,
}: {
  config: DesktopConfig;
  onChange: (next: DesktopConfig) => void;
}) {
  return (
    <div className="space-y-6 max-w-3xl">
      <h3 className="text-lg font-semibold">General</h3>

      <SettingsCard title="Tasks" description="Runtime defaults for spawned desktop tasks">
        <div className="grid gap-4 md:grid-cols-2">
          <Field label="Task timeout (minutes)">
            <input
              type="number"
              min={1}
              value={config.task_timeout_minutes}
              onChange={(event) =>
                onChange({
                  ...config,
                  task_timeout_minutes: Number(event.target.value) || config.task_timeout_minutes,
                })
              }
              className="w-full rounded-lg border border-border bg-surface-2 px-3 py-2 text-sm text-text-primary"
            />
          </Field>

          <Field label="Default IDE">
            <select
              value={config.default_ide}
              onChange={(event) =>
                onChange({
                  ...config,
                  default_ide: event.target.value,
                })
              }
              className="w-full rounded-lg border border-border bg-surface-2 px-3 py-2 text-sm text-text-primary"
            >
              <option value="code">VS Code</option>
              <option value="zed">Zed</option>
              <option value="idea">IntelliJ IDEA</option>
            </select>
          </Field>
        </div>

        <Toggle
          label="Default to worktrees"
          description="New tasks start in .haft/worktrees/{branch} unless overridden."
          checked={config.default_worktree}
          onChange={(checked) =>
            onChange({
              ...config,
              default_worktree: checked,
            })
          }
        />
      </SettingsCard>

      <SettingsCard title="Notifications" description="Operator feedback when tasks finish or fail">
        <Toggle
          label="Sound on task completion"
          checked={config.sound_enabled}
          onChange={(checked) =>
            onChange({
              ...config,
              sound_enabled: checked,
            })
          }
        />

        <Toggle
          label="Desktop notifications"
          checked={config.notify_enabled}
          onChange={(checked) =>
            onChange({
              ...config,
              notify_enabled: checked,
            })
          }
        />
      </SettingsCard>
    </div>
  );
}

function ProjectSettings() {
  const [projects, setProjects] = useState<ProjectInfo[]>([]);
  const [scanning, setScanning] = useState(false);
  const [discovered, setDiscovered] = useState<ProjectInfo[]>([]);

  useEffect(() => {
    listProjects()
      .then(setProjects)
      .catch((error) => {
        reportError(error, "projects");
      });
  }, []);

  const handleScan = async () => {
    setScanning(true);

    try {
      const found = await scanForProjects();
      const existingPaths = new Set(projects.map((project) => project.path));

      setDiscovered(found.filter((project) => !existingPaths.has(project.path)));
    } catch (error) {
      reportError(error, "scan projects");
    } finally {
      setScanning(false);
    }
  };

  const handleAdd = async (path: string) => {
    try {
      const added = await addProject(path);

      setProjects((current) => [...current, added]);
      setDiscovered((current) => current.filter((project) => project.path !== path));
    } catch (error) {
      reportError(error, "add project");
    }
  };

  return (
    <div className="space-y-6 max-w-3xl">
      <div className="flex items-center justify-between">
        <h3 className="text-lg font-semibold">Projects</h3>
        <button
          onClick={handleScan}
          disabled={scanning}
          className="rounded-lg bg-accent px-3 py-1.5 text-xs text-white transition-colors hover:bg-accent-hover disabled:opacity-50"
        >
          {scanning ? "Scanning..." : "Scan for projects"}
        </button>
      </div>

      <div className="space-y-2">
        {projects.map((project) => (
          <div
            key={project.path}
            className="flex items-center justify-between rounded-lg border border-border bg-surface-1 px-4 py-3"
          >
            <div>
              <span className="text-sm font-medium">{project.name}</span>
              <p className="mt-0.5 font-mono text-xs text-text-muted">{project.path}</p>
            </div>

            <div className="flex items-center gap-3 text-xs text-text-muted">
              <span>{project.problem_count} problems</span>
              <span>{project.decision_count} decisions</span>
              {project.stale_count > 0 && (
                <span className="text-warning">{project.stale_count} stale</span>
              )}
            </div>
          </div>
        ))}
      </div>

      {discovered.length > 0 && (
        <div>
          <h4 className="mb-2 text-xs uppercase tracking-wider text-text-muted">
            Discovered ({discovered.length})
          </h4>

          <div className="space-y-1">
            {discovered.map((project) => (
              <div
                key={project.path}
                className="flex items-center justify-between rounded-lg border border-border/50 bg-surface-1/50 px-4 py-2"
              >
                <div>
                  <span className="text-sm text-text-secondary">{project.name}</span>
                  <p className="mt-0.5 font-mono text-xs text-text-muted">{project.path}</p>
                </div>

                <button
                  onClick={() => handleAdd(project.path)}
                  className="rounded border border-accent/20 bg-accent/10 px-2 py-1 text-xs text-accent transition-colors hover:bg-accent/20"
                >
                  Add
                </button>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

function AgentSettings({
  config,
  onChange,
}: {
  config: DesktopConfig;
  onChange: (next: DesktopConfig) => void;
}) {
  const updatePreset = (index: number, nextPreset: AgentPreset) => {
    const presets = config.agent_presets.map((preset, presetIndex) =>
      presetIndex === index ? nextPreset : preset,
    );

    onChange({
      ...config,
      agent_presets: presets,
    });
  };

  const addPreset = () => {
    onChange({
      ...config,
      agent_presets: [
        ...config.agent_presets,
        { name: "New preset", agent_kind: config.default_agent, model: "", role: "implementation" },
      ],
    });
  };

  const removePreset = (index: number) => {
    onChange({
      ...config,
      agent_presets: config.agent_presets.filter((_, presetIndex) => presetIndex !== index),
    });
  };

  return (
    <div className="space-y-6 max-w-3xl">
      <h3 className="text-lg font-semibold">Agents</h3>

      <SettingsCard title="Default roles" description="Pick the default agents used by the desktop shell">
        <div className="grid gap-4 md:grid-cols-3">
          <Field label="Implementation">
            <AgentKindSelect
              value={config.default_agent}
              onChange={(value) =>
                onChange({
                  ...config,
                  default_agent: value,
                })
              }
            />
          </Field>

          <Field label="Review">
            <AgentKindSelect
              value={config.review_agent}
              onChange={(value) =>
                onChange({
                  ...config,
                  review_agent: value,
                })
              }
            />
          </Field>

          <Field label="Verify">
            <AgentKindSelect
              value={config.verify_agent}
              onChange={(value) =>
                onChange({
                  ...config,
                  verify_agent: value,
                })
              }
            />
          </Field>
        </div>
      </SettingsCard>

      <SettingsCard title="Runtime behavior" description="Desktop-specific agent wiring and preset management">
        <Toggle
          label="Auto-wire Haft MCP"
          description="Inject Haft into agent configs when supported so spawned tasks inherit the reasoning toolset."
          checked={config.auto_wire_mcp}
          onChange={(checked) =>
            onChange({
              ...config,
              auto_wire_mcp: checked,
            })
          }
        />

        <div className="space-y-3">
          <div className="flex items-center justify-between">
            <h4 className="text-sm font-medium text-text-primary">Agent presets</h4>
            <button
              onClick={addPreset}
              className="rounded border border-accent/20 bg-accent/10 px-2 py-1 text-xs text-accent transition-colors hover:bg-accent/20"
            >
              Add preset
            </button>
          </div>

          {config.agent_presets.map((preset, index) => (
            <div
              key={`${preset.name}-${index}`}
              className="grid gap-3 rounded-lg border border-border bg-surface-2/50 p-4 md:grid-cols-[1.2fr,1fr,1fr,auto]"
            >
              <input
                value={preset.name}
                onChange={(event) =>
                  updatePreset(index, {
                    ...preset,
                    name: event.target.value,
                  })
                }
                placeholder="Preset name"
                className="rounded-lg border border-border bg-surface-2 px-3 py-2 text-sm text-text-primary"
              />

              <AgentKindSelect
                value={preset.agent_kind}
                onChange={(value) =>
                  updatePreset(index, {
                    ...preset,
                    agent_kind: value,
                  })
                }
              />

              <select
                value={preset.role}
                onChange={(event) =>
                  updatePreset(index, {
                    ...preset,
                    role: event.target.value,
                  })
                }
                className="rounded-lg border border-border bg-surface-2 px-3 py-2 text-sm text-text-primary"
              >
                <option value="implementation">Implementation</option>
                <option value="review">Review</option>
                <option value="verify">Verify</option>
              </select>

              <button
                onClick={() => removePreset(index)}
                className="rounded border border-danger/20 bg-danger/10 px-3 py-2 text-xs text-danger transition-colors hover:bg-danger/20"
              >
                Remove
              </button>

              <input
                value={preset.model}
                onChange={(event) =>
                  updatePreset(index, {
                    ...preset,
                    model: event.target.value,
                  })
                }
                placeholder="Optional model"
                className="rounded-lg border border-border bg-surface-2 px-3 py-2 text-sm text-text-primary md:col-span-3"
              />
            </div>
          ))}
        </div>
      </SettingsCard>
    </div>
  );
}

function MCPSettings({ config }: { config: DesktopConfig }) {
  return (
    <div className="space-y-6 max-w-2xl">
      <h3 className="text-lg font-semibold">MCP Servers</h3>

      <SettingsCard
        title="Haft MCP Server"
        description="Built-in reasoning tools server used by desktop-spawned tasks"
      >
        <div className="space-y-2">
          <div className="flex items-center justify-between">
            <span className="text-sm text-text-secondary">Status</span>
            <span className="rounded-full border border-success/20 bg-success/10 px-2 py-0.5 text-xs text-success">
              configured
            </span>
          </div>

          <div className="flex items-center justify-between">
            <span className="text-sm text-text-secondary">Transport</span>
            <span className="font-mono text-xs text-text-muted">stdio (haft serve)</span>
          </div>

          <div className="flex items-center justify-between">
            <span className="text-sm text-text-secondary">Auto-wire on spawn</span>
            <span className={`text-xs ${config.auto_wire_mcp ? "text-success" : "text-text-muted"}`}>
              {config.auto_wire_mcp ? "enabled" : "disabled"}
            </span>
          </div>
        </div>
      </SettingsCard>
    </div>
  );
}

function AgentKindSelect({
  value,
  onChange,
}: {
  value: string;
  onChange: (value: string) => void;
}) {
  return (
    <select
      value={value}
      onChange={(event) => onChange(event.target.value)}
      className="w-full rounded-lg border border-border bg-surface-2 px-3 py-2 text-sm text-text-primary"
    >
      <option value="claude">Claude Code</option>
      <option value="codex">Codex</option>
      <option value="haft">Haft Agent</option>
    </select>
  );
}

function Toggle({
  label,
  description,
  checked,
  onChange,
}: {
  label: string;
  description?: string;
  checked: boolean;
  onChange: (value: boolean) => void;
}) {
  return (
    <label className="flex items-start justify-between gap-4">
      <div>
        <span className="text-sm text-text-secondary">{label}</span>
        {description && <p className="mt-0.5 text-xs text-text-muted">{description}</p>}
      </div>

      <input
        type="checkbox"
        checked={checked}
        onChange={(event) => onChange(event.target.checked)}
        className="mt-1 h-4 w-4 rounded border border-border bg-surface-2"
      />
    </label>
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

function SettingsCard({
  title,
  description,
  children,
}: {
  title: string;
  description?: string;
  children: ReactNode;
}) {
  return (
    <div className="rounded-lg border border-border bg-surface-1 px-5 py-4">
      <div className="mb-4">
        <h4 className="text-sm font-medium">{title}</h4>
        {description && <p className="mt-1 text-xs text-text-muted">{description}</p>}
      </div>

      <div className="space-y-4">{children}</div>
    </div>
  );
}
