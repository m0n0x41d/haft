import { useCallback, useEffect, useState } from "react";
import {
  PanelLeftClose,
  PanelLeftOpen,
  Plus,
  Zap,
  Search,
  MoreHorizontal,
  X,
  Settings as SettingsIcon,
  ChevronRight,
  ChevronDown,
  Workflow,
  LayoutDashboard,
  ListTodo,
  FolderPlus,
  Scale,
  MessageSquare,
} from "lucide-react";
import { Dashboard } from "./pages/Dashboard";
import { Portfolios } from "./pages/Portfolios";
import { Settings } from "./pages/Settings";
import { Tasks } from "./pages/Tasks";
import { Flows as Jobs } from "./pages/Jobs";
import { Harness } from "./pages/Harness";
import { NotificationViewport, type DesktopNotification } from "./components/Notifications";
import { SearchOverlay } from "./components/SearchOverlay";
import { TerminalPanel } from "./components/TerminalPanel";
import { ToastViewport } from "./components/Toast";
import { RailBtn, SidebarTask } from "./components/shell";
import { listenForErrors, reportError, type AppErrorDetail } from "./lib/errors";
import { listProjects, removeProject, switchProject, listTasks, toggleMaximize, type ProjectInfo, type TaskState } from "./lib/api";
import { getPageTitle, resolveNavigation, type Page } from "./navigation";
import { subscribe } from "./lib/events";
import { projectIsRunnable, projectReadiness } from "./lib/projectReadiness";

const REASONING_NAV: { id: Page; label: string; icon: typeof LayoutDashboard }[] = [
  { id: "dashboard", label: "Core", icon: LayoutDashboard },
  { id: "portfolios", label: "Comparison", icon: Scale },
];

export default function App() {
  const [page, setPage] = useState<Page>("dashboard");
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [projects, setProjects] = useState<ProjectInfo[]>([]);
  const [tasks, setTasks] = useState<TaskState[]>([]);
  const [refreshKey, setRefreshKey] = useState(0);
  const [searchOpen, setSearchOpen] = useState(false);
  const [sidebarExpanded, setSidebarExpanded] = useState(true);
  const [expandedProjects, setExpandedProjects] = useState<Set<string>>(new Set());
  const [showNewTask, setShowNewTask] = useState(false);
  const [showPlusMenu, setShowPlusMenu] = useState(false);
  const [showNewProject, setShowNewProject] = useState(false);
  const [toasts, setToasts] = useState<AppErrorDetail[]>([]);
  const [notifications, setNotifications] = useState<DesktopNotification[]>([]);
  const [terminalOpen, setTerminalOpen] = useState(false);

  const refreshTasks = useCallback(() => {
    return listTasks()
      .then(setTasks)
      .catch((error) => {
        reportError(error, "tasks");
      });
  }, []);

  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === "k") {
        e.preventDefault();
        setSearchOpen((v) => !v);
      }

      if ((e.metaKey || e.ctrlKey) && e.key === "`") {
        e.preventDefault();
        setTerminalOpen((current) => !current);
      }
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, []);

  useEffect(() => {
    listProjects()
      .then((p) => {
        setProjects(p);

        // Expand all projects — like Zenflow, all repos visible in sidebar
        setExpandedProjects(new Set(p.map((proj) => proj.path)));
      })
      .catch((error) => {
        reportError(error, "projects");
      });
  }, [refreshKey]);

  useEffect(() => {
    void refreshTasks();

    // Single shared polling loop for both sidebar badges and Tasks page.
    const interval = setInterval(() => {
      void refreshTasks();
    }, 2000);

    return () => clearInterval(interval);
  }, [refreshKey, refreshTasks]);

  useEffect(() => {
    const stopListening = listenForErrors((detail) => {
      setToasts((current) => [...current, detail].slice(-4));
    });

    const stopBackendErrors = subscribe<{ scope?: string; message?: string }>("app.error", (payload) => {
      reportError(payload?.message ?? "Unexpected error", payload?.scope);
    });

    const stopNotifications = subscribe<DesktopNotification>("notification.push", (payload) => {
      setNotifications((current) => [...current, payload].slice(-4));
    });

    return () => {
      stopListening();
      stopBackendErrors?.();
      stopNotifications?.();
    };
  }, []);

  const navigate = (p: Page, id?: string) => {
    const nextNavigation = resolveNavigation(p, id);

    setPage(nextNavigation.page);
    setSelectedId(nextNavigation.selectedId);
  };

  const handleSwitchProject = async (path: string) => {
    try {
      await switchProject(path);
      setRefreshKey((k) => k + 1);
      setPage("dashboard");
      setSelectedId(null);
    } catch (error) {
      reportError(error, "switch project");
    }
  };

  const handleRemoveProject = async (path: string) => {
    try {
      await removeProject(path);
      setProjects((current) => current.filter((project) => project.path !== path));
      setExpandedProjects((current) => {
        const next = new Set(current);
        next.delete(path);
        return next;
      });
    } catch (error) {
      reportError(error, "remove project");
    }
  };

  const handleOpenTask = async (task: TaskState) => {
    if (task.project_path && task.project_path !== activeProject?.path) {
      await handleSwitchProject(task.project_path);
    }

    setPage("tasks");
    setSelectedId(task.id);
  };

  const toggleProject = (path: string) => {
    setExpandedProjects((prev) => {
      const next = new Set(prev);
      if (next.has(path)) next.delete(path);
      else next.add(path);
      return next;
    });
  };

  const activeProject = projects.find((p) => p.is_active && projectIsRunnable(p));
  const projectTasks = (projectPath: string) => {
    // Always filter by project_path. `listTasks()` invokes the backend
    // command with no project argument, so `tasks` is the union across
    // every registered project — returning the whole set for the active
    // project would duplicate entries and surface unrelated rows in the
    // active project's sidebar.
    return tasks.filter((task) => task.project_path === projectPath);
  };

  return (
    <div className="flex h-screen overflow-hidden">
      {/* Icon rail */}
      <div className="w-12 shrink-0 bg-surface-1 border-r border-border flex flex-col items-center py-2">
        <div data-tauri-drag-region className="h-12 w-full" />

        <RailBtn
          icon={sidebarExpanded ? PanelLeftClose : PanelLeftOpen}
          tip="Toggle sidebar"
          onClick={() => setSidebarExpanded(!sidebarExpanded)}
          active={sidebarExpanded}
        />
        <div className="relative">
          <RailBtn
            icon={Plus}
            tip="New task or project"
            onClick={() => setShowPlusMenu(!showPlusMenu)}
            accent
          />
          {showPlusMenu && (
            <>
              <div className="fixed inset-0 z-40" onClick={() => setShowPlusMenu(false)} />
              <div className="absolute left-12 top-0 z-50 w-40 rounded-lg border border-border bg-surface-1 py-1 shadow-xl">
                <button
                  onClick={() => {
                    setShowPlusMenu(false);
                    if (!activeProject) {
                      setPage("settings");
                      setSelectedId("projects");
                      return;
                    }

                    setPage("tasks");
                    setShowNewTask(true);
                  }}
                  className={`flex w-full items-center gap-2 px-3 py-2 text-xs transition-colors ${
                    activeProject
                      ? "text-text-secondary hover:bg-surface-2"
                      : "text-text-muted hover:bg-surface-2"
                  }`}
                >
                  <ListTodo size={14} />
                  {activeProject ? "New task" : "Select project first"}
                </button>
                <button
                  onClick={() => { setShowPlusMenu(false); setShowNewProject(true); }}
                  className="flex w-full items-center gap-2 px-3 py-2 text-xs text-text-secondary hover:bg-surface-2 transition-colors"
                >
                  <FolderPlus size={14} />
                  New project
                </button>
              </div>
            </>
          )}
        </div>
        <RailBtn
          icon={LayoutDashboard}
          tip="Core"
          label="core"
          onClick={() => setPage("dashboard")}
          active={page === "dashboard"}
        />
        <RailBtn
          icon={MessageSquare}
          tip="Conversations"
          label="chat"
          onClick={() => setPage("tasks")}
          active={page === "tasks"}
        />
        <RailBtn
          icon={Zap}
          tip="Jobs"
          label="jobs"
          onClick={() => setPage("jobs")}
          active={page === "jobs"}
        />
        <RailBtn
          icon={Workflow}
          tip="Runtime"
          label="run"
          onClick={() => setPage("harness")}
          active={page === "harness"}
        />
        <RailBtn
          icon={Search}
          tip="Search (Cmd+K)"
          label="find"
          onClick={() => setSearchOpen(true)}
        />
        <RailBtn icon={MoreHorizontal} tip="More" label="more" onClick={() => {}} />

        <div className="flex-1" />

        <RailBtn
          icon={SettingsIcon}
          tip="Settings"
          label="set"
          onClick={() => navigate("settings")}
          active={page === "settings"}
        />
        <span className="text-[9px] text-text-muted/50 mt-2 mb-1">0.1</span>
      </div>

      {/* Sidebar */}
      {sidebarExpanded && (
        <div className="w-56 shrink-0 bg-surface-1 border-r border-border flex flex-col overflow-hidden">
          <div
            data-tauri-drag-region
            className="h-10"
            onDoubleClick={() => { void toggleMaximize().catch(() => {}); }}
          />

          {/* Project tree */}
          <div className="flex-1 overflow-y-auto px-1">
            {projects.map((proj) => {
              const status = projectReadiness(proj);
              const runnable = projectIsRunnable(proj);
              const missing = status === "missing";
              const isExpanded = runnable && expandedProjects.has(proj.path);
              const pTasks = runnable ? projectTasks(proj.path) : [];

              return (
                <div key={proj.path} className="mb-1">
                  <div className="flex items-center group">
                    <button
                      onClick={() => {
                        if (!runnable) {
                          setPage("settings");
                          setSelectedId("projects");
                          return;
                        }

                        toggleProject(proj.path);
                        if (!proj.is_active) handleSwitchProject(proj.path);
                      }}
                      className={`flex-1 flex items-center gap-1.5 px-2 py-1.5 rounded text-sm transition-colors ${
                        proj.is_active
                          ? "text-text-primary"
                          : missing
                            ? "text-danger hover:text-danger"
                            : "text-text-secondary hover:text-text-primary"
                      }`}
                      title={runnable ? proj.path : `${proj.path} is not runnable. Open Settings to repair it.`}
                    >
                      {isExpanded ? (
                        <ChevronDown size={12} className="text-text-muted shrink-0" />
                      ) : (
                        <ChevronRight size={12} className="text-text-muted shrink-0" />
                      )}
                      <span className="truncate">{proj.name}</span>
                      {status !== "ready" && (
                        <span className={`ml-auto rounded px-1.5 py-0.5 text-[10px] ${
                          missing ? "bg-danger/10 text-danger" : "bg-warning/10 text-warning"
                        }`}>
                          {missing ? "missing" : "init"}
                        </span>
                      )}
                    </button>
                    <button
                      onClick={() => {
                        if (missing) {
                          setPage("settings");
                          setSelectedId("projects");
                          return;
                        }

                        if (!runnable) {
                          setPage("settings");
                          setSelectedId("projects");
                          return;
                        }

                        if (!proj.is_active) handleSwitchProject(proj.path);
                        setPage("tasks");
                        setShowNewTask(true);
                      }}
                      className={`p-0.5 transition-opacity ${
                        !runnable
                          ? "opacity-0 text-text-muted"
                          : "opacity-0 text-text-muted hover:text-accent group-hover:opacity-100"
                      }`}
                      disabled={!runnable}
                      title={runnable ? "New task" : "Project is not runnable"}
                    >
                      <Plus size={14} />
                    </button>
                    <button
                      onClick={() => {
                        if (missing) {
                          void handleRemoveProject(proj.path);
                        }
                      }}
                      className={`p-0.5 transition-opacity ${
                        missing
                          ? "text-danger opacity-100 hover:text-danger/80"
                          : "text-text-muted opacity-0 hover:text-text-primary group-hover:opacity-100"
                      }`}
                      title={missing ? "Remove missing project" : "Project options"}
                    >
                      {missing ? <X size={14} /> : <MoreHorizontal size={14} />}
                    </button>
                  </div>

                  {isExpanded && (
                    <div className="ml-2">
                      {pTasks.length === 0 && (
                        <p className="text-xs text-text-muted/50 px-2 py-1">No tasks</p>
                      )}
                      {pTasks.map((t) => (
                        <SidebarTask
                          key={t.id}
                          task={t}
                          selected={selectedId === t.id && page === "tasks"}
                          onSelect={() => { setPage("tasks"); setSelectedId(t.id); }}
                          onArchive={async () => {
                            try {
                              const { archiveTask: doArchive } = await import("./lib/api");
                              await doArchive(t.id);
                              setTasks((prev) => prev.filter((x) => x.id !== t.id));
                              if (selectedId === t.id) setSelectedId(null);
                            } catch (e) { console.error(e); }
                          }}
                        />
                      ))}
                    </div>
                  )}
                </div>
              );
            })}
          </div>

          {/* Reasoning nav */}
          <div className="border-t border-border py-2 px-1">
            <p className="text-[10px] text-text-muted/50 uppercase tracking-wider px-2 mb-1">Reasoning</p>
            {REASONING_NAV.map((item) => {
              const Icon = item.icon;
              return (
                <button
                  key={item.id}
                  onClick={() => navigate(item.id)}
                  className={`w-full flex items-center gap-2 px-3 py-1.5 rounded text-xs transition-colors ${
                    page === item.id
                      ? "bg-surface-2 text-text-primary"
                      : "text-text-secondary hover:bg-surface-2/50 hover:text-text-primary"
                  }`}
                >
                  <Icon size={13} />
                  {item.label}
                </button>
              );
            })}
          </div>
        </div>
      )}

      {/* Main */}
      <div className="flex flex-1 flex-col overflow-hidden bg-surface-0">
        <main className="flex-1 overflow-y-auto bg-surface-0">
          <div
            data-tauri-drag-region
            className="sticky top-0 z-10 flex h-10 items-center justify-between border-b border-border bg-surface-0/80 px-6 backdrop-blur-sm"
            onDoubleClick={() => { void toggleMaximize().catch(() => {}); }}
          >
            <h2 className="text-sm font-medium text-text-secondary">
              {activeProject?.name && <span className="text-text-muted">{activeProject.name} / </span>}
              {getPageTitle(page)}
            </h2>
            <div className="flex items-center gap-2">
              <button
                onClick={() => {
                  if (!activeProject) {
                    setPage("settings");
                    setSelectedId("projects");
                    return;
                  }

                  setTerminalOpen((current) => !current);
                }}
                className={`rounded border border-border bg-surface-1 px-2 py-1 text-xs transition-colors ${
                  activeProject ? "text-text-muted hover:text-text-secondary" : "text-text-muted/50"
                }`}
                title={activeProject ? "Open project terminal" : "Select a ready project first"}
              >
                Terminal <span className="ml-1 text-text-muted/50">Cmd+`</span>
              </button>
              <button
                onClick={() => setSearchOpen(true)}
                className="rounded border border-border bg-surface-1 px-2 py-1 text-xs text-text-muted transition-colors hover:text-text-secondary"
              >
                Search... <span className="ml-1 text-text-muted/50">Cmd+K</span>
              </button>
            </div>
          </div>

          <div className="p-6" key={refreshKey}>
            {page === "dashboard" && <Dashboard onNavigate={navigate} />}
            {page === "harness" && <Harness />}
            {page === "portfolios" && <Portfolios selectedId={selectedId} onNavigate={navigate} />}
            {page === "jobs" && <Jobs onOpenTask={handleOpenTask} />}
            {page === "tasks" && (
              <Tasks
                selectedTaskId={selectedId}
                showNewTask={showNewTask}
                onNewTaskClose={() => setShowNewTask(false)}
                tasks={tasks}
                onTasksChange={setTasks}
                onTasksRefresh={refreshTasks}
              />
            )}
            {page === "settings" && (
              <Settings
                initialTab={selectedId === "projects" ? "projects" : undefined}
                onProjectRegistryChange={() => {
                  setRefreshKey((key) => key + 1);
                }}
              />
            )}
          </div>
        </main>

        <TerminalPanel
          open={terminalOpen && Boolean(activeProject)}
          projectPath={activeProject?.path ?? ""}
          onClose={() => setTerminalOpen(false)}
        />
      </div>

      {/* New Project modal */}
      {showNewProject && (
        <NewProjectModal
          onClose={() => setShowNewProject(false)}
          onProjectAdded={() => { setRefreshKey((k) => k + 1); setShowNewProject(false); setPage("dashboard"); setSelectedId(null); }}
        />
      )}

      <SearchOverlay open={searchOpen} onClose={() => setSearchOpen(false)} onNavigate={(p, id) => navigate(p as Page, id)} />
      <NotificationViewport
        notifications={notifications}
        onDismiss={(id) => {
          setNotifications((current) => current.filter((notification) => notification.id !== id));
        }}
      />
      <ToastViewport
        toasts={toasts}
        onDismiss={(id) => {
          setToasts((current) => current.filter((toast) => toast.id !== id));
        }}
      />
    </div>
  );
}

function NewProjectModal({
  onClose,
  onProjectAdded,
}: {
  onClose: () => void;
  onProjectAdded: () => void;
}) {
  const [discovered, setDiscovered] = useState<ProjectInfo[]>([]);
  const [scanning, setScanning] = useState(false);
  const [selectedPath, setSelectedPath] = useState("");
  const [adding, setAdding] = useState(false);

  useEffect(() => {
    setScanning(true);
    import("./lib/api").then(({ scanForProjects, listProjects }) =>
      Promise.all([scanForProjects(), listProjects()]).then(([found, existing]) => {
        const existingPaths = new Set(existing.map((p: ProjectInfo) => p.path));
        setDiscovered(found.filter((f: ProjectInfo) => !existingPaths.has(f.path)));
        setScanning(false);
      })
    ).catch(() => setScanning(false));
  }, []);

  const handlePick = async () => {
    try {
      const { openDirectoryPicker } = await import("./lib/api");
      const path = await openDirectoryPicker();
      if (path) setSelectedPath(path);
    } catch { /* ignore */ }
  };

  // Smart add: checks for .haft/, inits if missing, registers, and switches.
  const handleSmartAdd = async (path: string) => {
    if (!path.trim() || adding) return;
    setAdding(true);
    try {
      const { addProjectSmart } = await import("./lib/api");
      await addProjectSmart(path);
      onProjectAdded();
    } catch (e) {
      console.error("Failed to add project:", e);
      const { reportError } = await import("./lib/errors");
      reportError(e, "add project");
    } finally {
      setAdding(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
      <div className="w-[480px] rounded-2xl border border-border bg-surface-1 overflow-hidden">
        <div className="flex items-center justify-between px-5 py-4 border-b border-border">
          <h3 className="text-lg font-semibold">Add project</h3>
          <button onClick={onClose} className="text-text-muted hover:text-text-primary">x</button>
        </div>

        <div className="p-5 space-y-4">
          <div className="flex gap-2">
            <input
              value={selectedPath}
              onChange={(e) => setSelectedPath(e.target.value)}
              placeholder="/path/to/project"
              className="flex-1 rounded-lg border border-border bg-surface-2 px-3 py-2 font-mono text-sm text-text-primary"
            />
            <button onClick={handlePick} className="rounded-lg border border-border bg-surface-2 px-3 py-2 text-sm text-text-secondary hover:bg-surface-3 transition-colors">
              Browse
            </button>
          </div>

          {selectedPath && (
            <button
              onClick={() => handleSmartAdd(selectedPath)}
              disabled={adding}
              className="rounded-full bg-accent px-4 py-2 text-sm text-surface-0 hover:bg-accent-hover transition-colors disabled:opacity-50"
            >
              {adding ? "Adding..." : "Add project"}
            </button>
          )}

          {scanning ? (
            <p className="text-xs text-text-muted text-center py-4">Scanning for projects...</p>
          ) : discovered.length > 0 ? (
            <div>
              <p className="text-xs text-text-muted uppercase tracking-wider mb-2">Discovered</p>
              <div className="space-y-1.5 max-h-64 overflow-y-auto">
                {discovered.map((p) => (
                  <button
                    key={p.path}
                    onClick={() => handleSmartAdd(p.path)}
                    className="w-full flex items-center gap-3 rounded-lg border border-border bg-surface-2/50 px-4 py-3 text-left hover:bg-surface-2 transition-colors"
                  >
                    <FolderPlus size={16} className="text-text-muted shrink-0" />
                    <div className="min-w-0">
                      <p className="text-sm font-medium truncate">{p.name}</p>
                      <p className="text-xs text-text-muted font-mono truncate">{p.path}</p>
                    </div>
                  </button>
                ))}
              </div>
            </div>
          ) : !selectedPath ? (
            <p className="text-xs text-text-muted text-center py-4">Browse or paste a project path</p>
          ) : null}
        </div>
      </div>
    </div>
  );
}
