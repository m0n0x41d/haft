import { useEffect, useMemo, useState } from "react";
import {
  Activity,
  ArrowRight,
  Bot,
  CheckCircle2,
  CircleAlert,
  MessageSquare,
  RefreshCw,
  Workflow,
} from "lucide-react";

import { subscribe } from "../lib/events";
import {
  getDashboard,
  getGovernanceOverview,
  listCommissions,
  listTasks,
  type DashboardData,
  type GovernanceOverview,
  type TaskState,
  type WorkCommission,
} from "../lib/api";
import { Badge, Button, Card, Eyebrow, MonoId, Pill } from "../components/primitives";
import { reportError } from "../lib/errors";
import {
  buildCoreAttention,
  buildCoreRuntimeItems,
  type CoreAttentionItem,
  type CoreRuntimeItem,
  type CoreRuntimePhase,
  type CoreTone,
} from "./coreModel";
import { taskCountsAsActive, taskRunState } from "../lib/taskInput.ts";
import type { Page } from "../navigation";

type NavigateFn = (page: Page, id?: string) => void;

export function Dashboard({ onNavigate }: { onNavigate: NavigateFn }) {
  const [data, setData] = useState<DashboardData | null>(null);
  const [overview, setOverview] = useState<GovernanceOverview | null>(null);
  const [tasks, setTasks] = useState<TaskState[]>([]);
  const [commissions, setCommissions] = useState<WorkCommission[]>([]);
  const [loading, setLoading] = useState(true);
  const [proMode, setProMode] = useState(false);

  const refresh = async () => {
    try {
      const [
        nextDashboard,
        nextOverview,
        nextTasks,
        nextCommissions,
      ] = await Promise.all([
        getDashboard(),
        getGovernanceOverview(),
        listTasks(),
        listCommissions("open"),
      ]);

      setData(nextDashboard);
      setOverview(nextOverview);
      setTasks(nextTasks);
      setCommissions(nextCommissions);
    } catch (error) {
      reportError(error, "core");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void refresh();
  }, []);

  useEffect(() => {
    let stopStale: (() => void) | undefined;
    let stopDrift: (() => void) | undefined;

    try {
      stopStale = subscribe("scan.stale", () => {
        void refresh();
      });
      stopDrift = subscribe("scan.drift", () => {
        void refresh();
      });
    } catch {
      stopStale = undefined;
      stopDrift = undefined;
    }

    return () => {
      stopStale?.();
      stopDrift?.();
    };
  }, []);

  const attentionItems = useMemo(() => {
    if (!overview) return [];

    return buildCoreAttention({
      overview,
      tasks,
      commissions,
    });
  }, [overview, tasks, commissions]);

  const runtimeItems = useMemo(
    () => buildCoreRuntimeItems(commissions),
    [commissions],
  );

  if (loading && (!data || !overview)) {
    return <CoreSkeleton />;
  }

  if (!data || !overview) {
    return (
      <div className="mx-auto max-w-[920px] px-6 py-12">
        <Card dashed>
          <p className="text-sm text-text-muted">Core data is unavailable.</p>
        </Card>
      </div>
    );
  }

  const activeTasks = tasks.filter((task) => {
    const state = taskRunState(task.status);

    return taskCountsAsActive(state);
  });
  const displayedAttention = attentionItems.slice(0, proMode ? 8 : 4);
  const displayedRuntime = runtimeItems.slice(0, proMode ? 8 : 3);

  return (
    <div className="mx-auto max-w-[980px] px-6 pb-12 pt-8">
      <CoreHeader
        projectName={data.project_name || "haft"}
        attentionCount={attentionItems.length}
        activeRunCount={runtimeItems.length}
        activeTaskCount={activeTasks.length}
        proMode={proMode}
        onTogglePro={() => setProMode((current) => !current)}
        onRefresh={() => void refresh()}
      />

      <section className="mt-8">
        <SectionHeader
          title={`Needs You · ${attentionItems.length}`}
          description="Only things that require operator judgment or recovery."
        />
        {displayedAttention.length === 0 ? (
          <Card className="mt-3 px-5 py-6">
            <div className="flex items-center gap-3">
              <CheckCircle2 size={18} className="text-success" />
              <div>
                <p className="text-sm font-medium text-text-primary">No operator work right now.</p>
                <p className="mt-0.5 text-xs text-text-muted">
                  Conversations, governance, and runtime queue are calm.
                </p>
              </div>
            </div>
          </Card>
        ) : (
          <div className="mt-3 space-y-2">
            {displayedAttention.map((item) => (
              <AttentionCard
                key={item.id}
                item={item}
                onOpen={() => openAttentionItem(item, onNavigate)}
              />
            ))}
          </div>
        )}
      </section>

      <section className="mt-8">
        <SectionHeader
          title={`Active Runtime · ${runtimeItems.length}`}
          description="Harness work flowing through commission phases."
          action={
            <Button
              variant="ghost"
              icon={<Workflow size={14} />}
              onClick={() => onNavigate("harness")}
            >
              Open Runtime
            </Button>
          }
        />
        {displayedRuntime.length === 0 ? (
          <Card dashed className="mt-3 px-5 py-8 text-center">
            <p className="text-sm text-text-muted">No active WorkCommissions.</p>
          </Card>
        ) : (
          <div className="mt-3 space-y-2">
            {displayedRuntime.map((item) => (
              <RuntimeCard
                key={item.id}
                item={item}
                onOpen={() => onNavigate("harness")}
              />
            ))}
          </div>
        )}
      </section>

      {proMode ? (
        <section className="mt-8">
          <SectionHeader
            title="System Facts"
            description="Reference data kept out of the default cockpit path."
          />
          <div className="mt-3 grid gap-3 md:grid-cols-2 xl:grid-cols-4">
            <FactCard label="Problems" value={data.problem_count} />
            <FactCard label="Decisions" value={data.decision_count} />
            <FactCard label="Findings" value={overview.findings.length} tone="warning" />
            <FactCard
              label="Coverage"
              value={`${overview.coverage.governed_percent}%`}
              tone={overview.coverage.blind_count > 0 ? "warning" : "success"}
            />
          </div>
        </section>
      ) : null}

      <footer className="mt-8 flex flex-wrap items-center gap-2 border-t border-border pt-4 text-xs text-text-muted">
        <QuietJump
          label={`${data.decision_count} decisions`}
          onClick={() => onNavigate("dashboard")}
        />
        <QuietJump
          label={`${data.problem_count} problems`}
          onClick={() => onNavigate("dashboard")}
        />
        <QuietJump
          label={`${commissions.length} commissions`}
          onClick={() => onNavigate("harness")}
        />
        <QuietJump
          label={`${overview.coverage.total_modules} modules`}
          onClick={() => setProMode(true)}
        />
      </footer>
    </div>
  );
}

function CoreHeader({
  projectName,
  attentionCount,
  activeRunCount,
  activeTaskCount,
  proMode,
  onTogglePro,
  onRefresh,
}: {
  projectName: string;
  attentionCount: number;
  activeRunCount: number;
  activeTaskCount: number;
  proMode: boolean;
  onTogglePro: () => void;
  onRefresh: () => void;
}) {
  return (
    <header className="flex flex-col gap-5 md:flex-row md:items-start md:justify-between">
      <div>
        <Eyebrow>Core</Eyebrow>
        <div className="mt-2 flex flex-wrap items-baseline gap-3">
          <h1 className="text-3xl font-semibold tracking-tight text-text-primary">
            {projectName}
          </h1>
          <Pill tone={attentionCount > 0 ? "warning" : "accent"}>
            {attentionCount > 0 ? `${attentionCount} need attention` : "all clear"}
          </Pill>
        </div>
        <p className="mt-2 max-w-2xl text-sm text-text-muted">
          Project cockpit for conversations, FPF governance, WorkCommissions, and harness runtime.
        </p>
      </div>

      <div className="flex flex-wrap items-center gap-2">
        <StatusChip icon={<Bot size={14} />} value={`${activeTaskCount} conversations`} />
        <StatusChip icon={<Workflow size={14} />} value={`${activeRunCount} runtime`} />
        <Button
          variant={proMode ? "accent-chip" : "ghost"}
          onClick={onTogglePro}
        >
          Pro {proMode ? "on" : "off"}
        </Button>
        <Button
          variant="ghost"
          icon={<RefreshCw size={14} />}
          onClick={onRefresh}
        >
          Refresh
        </Button>
      </div>
    </header>
  );
}

function SectionHeader({
  title,
  description,
  action,
}: {
  title: string;
  description: string;
  action?: React.ReactNode;
}) {
  return (
    <div className="flex flex-col gap-2 sm:flex-row sm:items-end sm:justify-between">
      <div>
        <Eyebrow>{title}</Eyebrow>
        <p className="mt-1 text-xs text-text-muted">{description}</p>
      </div>
      {action}
    </div>
  );
}

function AttentionCard({
  item,
  onOpen,
}: {
  item: CoreAttentionItem;
  onOpen: () => void;
}) {
  return (
    <button
      onClick={onOpen}
      className={`w-full rounded-xl border bg-surface-1 px-4 py-3 text-left transition-colors hover:bg-surface-2 ${borderForTone(item.tone)}`}
    >
      <div className="flex items-start gap-3">
        <AttentionIcon tone={item.tone} />
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <Badge tone={item.tone}>{labelForKind(item.kind)}</Badge>
            <p className="truncate text-sm font-medium text-text-primary">{item.title}</p>
          </div>
          <p className="mt-2 line-clamp-2 text-sm text-text-secondary">{item.detail}</p>
          {item.meta ? (
            <p className="mt-2 font-mono text-[11px] text-text-muted">{item.meta}</p>
          ) : null}
        </div>
        <ArrowRight size={15} className="mt-1 shrink-0 text-text-muted" />
      </div>
    </button>
  );
}

function RuntimeCard({
  item,
  onOpen,
}: {
  item: CoreRuntimeItem;
  onOpen: () => void;
}) {
  return (
    <Card className="px-4 py-3">
      <button onClick={onOpen} className="w-full text-left">
        <div className="flex flex-wrap items-center gap-2">
          <MonoId id={item.id} tone={item.tone === "accent" ? "accent" : "neutral"} />
          <Badge tone={item.tone}>{item.state || "queued"}</Badge>
          <span className="min-w-0 flex-1 truncate text-sm font-medium text-text-primary">
            {item.title}
          </span>
          <span className="font-mono text-[11px] text-text-muted">{item.meta}</span>
        </div>

        <div className="mt-3">
          <PhaseCells phase={item.phase} />
        </div>

        {item.attentionReason ? (
          <p className="mt-2 font-mono text-[11px] text-warning">
            {item.attentionReason}
          </p>
        ) : (
          <p className="mt-2 font-mono text-[11px] text-text-muted">
            {item.decisionRef} · {item.problemRef}
          </p>
        )}
      </button>
    </Card>
  );
}

function PhaseCells({ phase }: { phase: CoreRuntimePhase }) {
  const phases: CoreRuntimePhase[] = ["preflight", "frame", "execute", "measure", "done"];
  const index = phase === "queued" ? -1 : phases.indexOf(phase);
  const blocked = phase === "blocked";

  return (
    <div className="grid grid-cols-5 gap-1.5">
      {phases.map((item, itemIndex) => {
        const state = blocked && itemIndex === 0
          ? "blocked"
          : itemIndex < index
            ? "done"
            : itemIndex === index
              ? "active"
              : "pending";

        return (
          <div
            key={item}
            className={`min-w-0 rounded-md border px-2 py-1 ${phaseCellClass(state)}`}
          >
            <div className="flex items-center gap-1.5">
              <span className="font-mono text-[9px] opacity-70">{itemIndex + 1}</span>
              <span className="truncate font-mono text-[10px] uppercase tracking-wide">
                {item}
              </span>
            </div>
          </div>
        );
      })}
    </div>
  );
}

function FactCard({
  label,
  value,
  tone = "neutral",
}: {
  label: string;
  value: number | string;
  tone?: CoreTone;
}) {
  return (
    <Card className="px-4 py-3">
      <p className={`text-2xl font-semibold tracking-tight ${textForTone(tone)}`}>{value}</p>
      <p className="mt-1 text-xs text-text-muted">{label}</p>
    </Card>
  );
}

function QuietJump({
  label,
  onClick,
}: {
  label: string;
  onClick: () => void;
}) {
  return (
    <button
      onClick={onClick}
      className="inline-flex items-center gap-1 rounded-full border border-border bg-surface-1 px-3 py-1 text-xs text-text-muted transition-colors hover:bg-surface-2 hover:text-text-primary"
    >
      {label}
      <ArrowRight size={12} />
    </button>
  );
}

function StatusChip({
  icon,
  value,
}: {
  icon: React.ReactNode;
  value: string;
}) {
  return (
    <span className="inline-flex items-center gap-1.5 rounded-full border border-border bg-surface-1 px-3 py-1 text-xs text-text-secondary">
      {icon}
      {value}
    </span>
  );
}

function AttentionIcon({ tone }: { tone: CoreTone }) {
  if (tone === "success") return <CheckCircle2 size={18} className="mt-0.5 text-success" />;
  if (tone === "danger") return <CircleAlert size={18} className="mt-0.5 text-danger" />;
  if (tone === "warning") return <CircleAlert size={18} className="mt-0.5 text-warning" />;
  if (tone === "accent") return <Activity size={18} className="mt-0.5 text-accent" />;

  return <MessageSquare size={18} className="mt-0.5 text-text-muted" />;
}

function CoreSkeleton() {
  return (
    <div className="mx-auto max-w-[980px] px-6 py-10">
      <div className="h-7 w-32 rounded bg-surface-2" />
      <div className="mt-4 h-4 w-96 rounded bg-surface-2" />
      <div className="mt-8 space-y-3">
        <div className="h-24 rounded-xl border border-border bg-surface-1" />
        <div className="h-24 rounded-xl border border-border bg-surface-1" />
        <div className="h-24 rounded-xl border border-border bg-surface-1" />
      </div>
    </div>
  );
}

function openAttentionItem(item: CoreAttentionItem, onNavigate: NavigateFn) {
  switch (item.action) {
    case "open_runtime":
      onNavigate("harness");
      return;
    case "open_task":
      onNavigate("tasks", item.actionRef);
      return;
    case "open_decision":
      onNavigate("dashboard");
      return;
    case "open_problem":
      onNavigate("dashboard");
      return;
  }
}

function labelForKind(kind: CoreAttentionItem["kind"]): string {
  if (kind === "runtime") return "Runtime";
  if (kind === "conversation") return "Conversation";
  if (kind === "governance") return "Governance";
  return "Problem";
}

function borderForTone(tone: CoreTone): string {
  if (tone === "danger") return "border-danger/30";
  if (tone === "warning") return "border-warning/30";
  if (tone === "success") return "border-success/30";
  if (tone === "accent") return "border-accent-border";
  return "border-border";
}

function textForTone(tone: CoreTone): string {
  if (tone === "danger") return "text-danger";
  if (tone === "warning") return "text-warning";
  if (tone === "success") return "text-success";
  if (tone === "accent") return "text-accent";
  return "text-text-primary";
}

function phaseCellClass(state: "pending" | "active" | "done" | "blocked"): string {
  if (state === "done") return "border-success/30 bg-success/10 text-success";
  if (state === "active") return "border-warning/40 bg-warning/10 text-warning";
  if (state === "blocked") return "border-danger/40 bg-danger/10 text-danger";
  return "border-border bg-surface-2 text-text-muted";
}
