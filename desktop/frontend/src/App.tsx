import { useEffect, useState } from "react";
import {
  PanelLeftClose,
  PanelLeftOpen,
  Plus,
  Zap,
  Search,
  MoreHorizontal,
  Settings as SettingsIcon,
  ChevronRight,
  ChevronDown,
  LayoutDashboard,
  AlertTriangle,
  Scale,
  CheckCircle2,
} from "lucide-react";
import { Dashboard } from "./pages/Dashboard";
import { Problems } from "./pages/Problems";
import { Decisions } from "./pages/Decisions";
import { Portfolios } from "./pages/Portfolios";
import { Settings } from "./pages/Settings";
import { Tasks } from "./pages/Tasks";
import { Flows } from "./pages/Flows";
import { NotificationViewport, type DesktopNotification } from "./components/Notifications";
import { SearchOverlay } from "./components/SearchOverlay";
import { TerminalPanel } from "./components/TerminalPanel";
import { ToastViewport } from "./components/Toast";
import { listenForErrors, reportError, type AppErrorDetail } from "./lib/errors";
import { listProjects, switchProject, listTasks, type ProjectInfo, type TaskState } from "./lib/api";
import { EventsOn } from "../wailsjs/runtime/runtime";

type Page = "dashboard" | "problems" | "portfolios" | "decisions" | "flows" | "tasks" | "settings";

const REASONING_NAV: { id: Page; label: string; icon: typeof LayoutDashboard }[] = [
  { id: "dashboard", label: "Overview", icon: LayoutDashboard },
  { id: "problems", label: "Problems", icon: AlertTriangle },
  { id: "portfolios", label: "Comparison", icon: Scale },
  { id: "decisions", label: "Decisions", icon: CheckCircle2 },
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
  const [toasts, setToasts] = useState<AppErrorDetail[]>([]);
  const [notifications, setNotifications] = useState<DesktopNotification[]>([]);
  const [terminalOpen, setTerminalOpen] = useState(false);

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

        const active = p.find((proj) => proj.is_active);
        if (active) setExpandedProjects(new Set([active.path]));
      })
      .catch((error) => {
        reportError(error, "projects");
      });
  }, [refreshKey]);

  useEffect(() => {
    const load = () =>
      listTasks()
        .then(setTasks)
        .catch((error) => {
          reportError(error, "tasks");
        });

    load();
    const interval = setInterval(load, 3000);
    return () => clearInterval(interval);
  }, [refreshKey]);

  useEffect(() => {
    const stopListening = listenForErrors((detail) => {
      setToasts((current) => [...current, detail].slice(-4));
    });

    let stopBackendErrors: (() => void) | undefined;

    try {
      stopBackendErrors = EventsOn("app.error", (payload: { scope?: string; message?: string }) => {
        reportError(payload?.message ?? "Unexpected error", payload?.scope);
      });
    } catch {
      stopBackendErrors = undefined;
    }

    let stopNotifications: (() => void) | undefined;

    try {
      stopNotifications = EventsOn("notification.push", (payload: DesktopNotification) => {
        setNotifications((current) => [...current, payload].slice(-4));
      });
    } catch {
      stopNotifications = undefined;
    }

    return () => {
      stopListening();
      stopBackendErrors?.();
      stopNotifications?.();
    };
  }, []);

  const navigate = (p: Page, id?: string) => {
    setPage(p);
    setSelectedId(id ?? null);
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

  const activeProject = projects.find((p) => p.is_active);
  const projectTasks = (projectPath: string) =>
    tasks.filter((task) => task.project_path === projectPath);

  return (
    <div className="flex h-screen overflow-hidden">
      {/* Icon rail */}
      <div className="w-12 shrink-0 bg-surface-1 border-r border-border flex flex-col items-center py-2">
        <div className="wails-drag h-8 w-full" />

        <RailBtn
          icon={sidebarExpanded ? PanelLeftClose : PanelLeftOpen}
          tip="Toggle sidebar"
          onClick={() => setSidebarExpanded(!sidebarExpanded)}
          active={sidebarExpanded}
        />
        <RailBtn
          icon={Plus}
          tip="New task"
          onClick={() => { setPage("tasks"); setShowNewTask(true); }}
          accent
        />
        <RailBtn
          icon={Zap}
          tip="Flows"
          onClick={() => setPage("flows")}
          active={page === "flows"}
        />
        <RailBtn icon={Search} tip="Search (Cmd+K)" onClick={() => setSearchOpen(true)} />
        <RailBtn icon={MoreHorizontal} tip="More" onClick={() => {}} />

        <div className="flex-1" />

        <RailBtn
          icon={SettingsIcon}
          tip="Settings"
          onClick={() => navigate("settings")}
          active={page === "settings"}
        />
        <span className="text-[9px] text-text-muted/50 mt-2 mb-1">0.1</span>
      </div>

      {/* Sidebar */}
      {sidebarExpanded && (
        <div className="w-56 shrink-0 bg-surface-1 border-r border-border flex flex-col overflow-hidden">
          <div className="wails-drag h-10" />

          {/* Project tree */}
          <div className="flex-1 overflow-y-auto px-1">
            {projects.map((proj) => {
              const isExpanded = expandedProjects.has(proj.path);
              const pTasks = projectTasks(proj.path);
              return (
                <div key={proj.path} className="mb-1">
                  <div className="flex items-center group">
                    <button
                      onClick={() => {
                        toggleProject(proj.path);
                        if (!proj.is_active) handleSwitchProject(proj.path);
                      }}
                      className={`flex-1 flex items-center gap-1.5 px-2 py-1.5 rounded text-sm transition-colors ${
                        proj.is_active ? "text-text-primary" : "text-text-secondary hover:text-text-primary"
                      }`}
                    >
                      {isExpanded ? (
                        <ChevronDown size={12} className="text-text-muted shrink-0" />
                      ) : (
                        <ChevronRight size={12} className="text-text-muted shrink-0" />
                      )}
                      <span className="truncate">{proj.name}</span>
                    </button>
                    <button
                      onClick={() => {
                        if (!proj.is_active) handleSwitchProject(proj.path);
                        setPage("tasks");
                        setShowNewTask(true);
                      }}
                      className="opacity-0 group-hover:opacity-100 text-text-muted hover:text-accent p-0.5 transition-opacity"
                      title="New task"
                    >
                      <Plus size={14} />
                    </button>
                    <button className="opacity-0 group-hover:opacity-100 text-text-muted hover:text-text-primary p-0.5 transition-opacity">
                      <MoreHorizontal size={14} />
                    </button>
                  </div>

                  {isExpanded && (
                    <div className="ml-2">
                      {pTasks.length === 0 && (
                        <p className="text-xs text-text-muted/50 px-2 py-1">No tasks</p>
                      )}
                      {pTasks.map((t) => (
                        <button
                          key={t.id}
                          onClick={() => { setPage("tasks"); setSelectedId(t.id); }}
                          className="w-full flex items-center gap-1.5 px-2 py-1 rounded text-xs text-text-secondary hover:bg-surface-2 transition-colors"
                        >
                          <StatusDot status={t.status} />
                          <span className="truncate">{t.title}</span>
                        </button>
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
          <div className="wails-drag sticky top-0 z-10 flex h-10 items-center justify-between border-b border-border bg-surface-0/80 px-6 backdrop-blur-sm">
            <h2 className="text-sm font-medium text-text-secondary">
              {activeProject?.name && <span className="text-text-muted">{activeProject.name} / </span>}
              {pageTitle(page)}
            </h2>
            <div className="flex items-center gap-2">
              <button
                onClick={() => setTerminalOpen((current) => !current)}
                className="wails-no-drag rounded border border-border bg-surface-1 px-2 py-1 text-xs text-text-muted transition-colors hover:text-text-secondary"
              >
                Terminal <span className="ml-1 text-text-muted/50">Cmd+`</span>
              </button>
              <button
                onClick={() => setSearchOpen(true)}
                className="wails-no-drag rounded border border-border bg-surface-1 px-2 py-1 text-xs text-text-muted transition-colors hover:text-text-secondary"
              >
                Search... <span className="ml-1 text-text-muted/50">Cmd+K</span>
              </button>
            </div>
          </div>

          <div className="p-6" key={refreshKey}>
            {page === "dashboard" && <Dashboard onNavigate={navigate} />}
            {page === "problems" && <Problems selectedId={selectedId} onNavigate={navigate} />}
            {page === "portfolios" && <Portfolios selectedId={selectedId} onNavigate={navigate} />}
            {page === "decisions" && <Decisions selectedId={selectedId} onNavigate={navigate} />}
            {page === "flows" && <Flows onOpenTask={handleOpenTask} />}
            {page === "tasks" && (
              <Tasks
                selectedTaskId={selectedId}
                showNewTask={showNewTask}
                onNewTaskClose={() => setShowNewTask(false)}
              />
            )}
            {page === "settings" && (
              <Settings
                onProjectRegistryChange={() => {
                  setRefreshKey((key) => key + 1);
                }}
              />
            )}
          </div>
        </main>

        <TerminalPanel
          open={terminalOpen}
          projectPath={activeProject?.path ?? ""}
          onClose={() => setTerminalOpen(false)}
        />
      </div>

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

function pageTitle(page: Page): string {
  if (page === "tasks") {
    return "Tasks";
  }

  if (page === "flows") {
    return "Automation";
  }

  if (page === "settings") {
    return "Settings";
  }

  return REASONING_NAV.find((item) => item.id === page)?.label ?? "Workspace";
}

function RailBtn({ icon: Icon, tip, onClick, active, accent }: {
  icon: typeof Plus;
  tip: string;
  onClick: () => void;
  active?: boolean;
  accent?: boolean;
}) {
  return (
    <button
      onClick={onClick}
      title={tip}
      className={`w-9 h-9 flex items-center justify-center rounded-lg mb-1 transition-colors ${
        accent
          ? "text-accent hover:bg-accent/10"
          : active
            ? "bg-surface-2 text-text-primary"
            : "text-text-muted hover:bg-surface-2/50 hover:text-text-primary"
      }`}
    >
      <Icon size={18} />
    </button>
  );
}

function StatusDot({ status }: { status: string }) {
  const colors: Record<string, string> = {
    running: "bg-blue-400 animate-pulse",
    completed: "bg-success",
    failed: "bg-danger",
    cancelled: "bg-text-muted",
    pending: "bg-warning",
  };
  return <span className={`w-2 h-2 rounded-full shrink-0 ${colors[status] ?? "bg-text-muted"}`} />;
}
