import { useCallback, useEffect, useMemo, useState } from "react";
import {
  Ban,
  CheckCircle2,
  Download,
  FileText,
  FolderOpen,
  GitBranch,
  RefreshCw,
  RotateCcw,
  Terminal,
  Timer,
} from "lucide-react";

import {
  applyHarnessResult,
  cancelCommission,
  getHarnessResult,
  getHarnessTail,
  listCommissions,
  openPathInIDE,
  requeueCommission,
  type HarnessTailResult,
  type HarnessRunResult,
  type WorkCommission,
} from "../lib/api";
import {
  buildHarnessCockpitDetail,
  normalizeHarnessCommissionState,
  type HarnessActionKind,
  type HarnessActionView,
  type HarnessCockpitDetail,
  type HarnessCommissionStateView,
} from "../lib/harnessCockpit";
import { Badge, MonoId, Pill } from "../components/primitives";
import { reportError } from "../lib/errors";

const SELECTORS = [
  { id: "stale", label: "Attention" },
  { id: "open", label: "Open" },
  { id: "runnable", label: "Runnable" },
  { id: "terminal", label: "Terminal" },
  { id: "all", label: "All" },
] as const;

type CommissionSelector = typeof SELECTORS[number]["id"];

export function Harness() {
  const [selector, setSelector] = useState<CommissionSelector>("open");
  const [commissions, setCommissions] = useState<WorkCommission[]>([]);
  const [selectedID, setSelectedID] = useState("");
  const [result, setResult] = useState<HarnessRunResult | null>(null);
  const [tail, setTail] = useState<HarnessTailResult | null>(null);
  const [loading, setLoading] = useState(true);
  const [busyAction, setBusyAction] = useState("");
  const [notice, setNotice] = useState("");

  const selectedCommission = useMemo(
    () => commissions.find((commission) => commission.id === selectedID) ?? null,
    [commissions, selectedID],
  );

  const selectedDetail = useMemo(
    () => selectedCommission ? buildHarnessCockpitDetail(selectedCommission, result, tail) : null,
    [selectedCommission, result, tail],
  );

  const refresh = useCallback(async () => {
    try {
      const nextCommissions = await listCommissions(selector);
      setCommissions(nextCommissions);
      setSelectedID((current) => {
        if (current && nextCommissions.some((commission) => commission.id === current)) {
          return current;
        }
        return nextCommissions[0]?.id ?? "";
      });
    } catch (error) {
      reportError(error, "harness commissions");
    } finally {
      setLoading(false);
    }
  }, [selector]);

  const refreshResult = useCallback(async (commissionID: string) => {
    if (!commissionID) {
      setResult(null);
      return;
    }

    try {
      const nextResult = await getHarnessResult(commissionID);
      setResult(nextResult);
    } catch (error) {
      reportError(error, "harness result");
      setResult(null);
    }
  }, []);

  useEffect(() => {
    setLoading(true);
    void refresh();

    const interval = window.setInterval(() => {
      void refresh();
    }, 4000);

    return () => {
      window.clearInterval(interval);
    };
  }, [refresh]);

  useEffect(() => {
    setTail(null);
    setResult(null);
    void refreshResult(selectedID);
  }, [refreshResult, selectedID]);

  const runAction = async (actionID: string, action: () => Promise<void>) => {
    if (busyAction) {
      return;
    }

    setBusyAction(actionID);
    setNotice("");
    try {
      await action();
      await refresh();
      if (selectedID) {
        await refreshResult(selectedID);
      }
    } catch (error) {
      reportError(error, actionID);
    } finally {
      setBusyAction("");
    }
  };

  const handleCancel = () =>
    runAction("cancel commission", async () => {
      if (!selectedID) return;
      await cancelCommission(selectedID, "cancelled from desktop harness operator");
      setNotice(`Cancelled ${selectedID}`);
    });

  const handleRequeue = () =>
    runAction("requeue commission", async () => {
      if (!selectedID) return;
      await requeueCommission(selectedID, "requeued from desktop harness operator");
      setNotice(`Requeued ${selectedID}`);
    });

  const handleApply = () =>
    runAction("apply harness diff", async () => {
      if (!selectedID) return;
      const applyResult = await applyHarnessResult(selectedID);
      setNotice(`Applied ${applyResult.files.length} file(s) from ${selectedID}`);
    });

  const handleResult = () =>
    runAction("refresh harness result", async () => {
      if (!selectedID) return;
      await refreshResult(selectedID);
      setNotice(`Refreshed result for ${selectedID}`);
    });

  const handleTail = () =>
    runAction("tail harness log", async () => {
      if (!selectedID) return;
      const nextTail = await getHarnessTail(selectedID, 20);
      setTail(nextTail);
      setNotice(`Loaded ${nextTail.lines.length} tail line(s) for ${selectedID}`);
    });

  const actionHandlers: Record<HarnessActionKind, () => void> = {
    result: handleResult,
    tail: handleTail,
    apply: handleApply,
    cancel: handleCancel,
    requeue: handleRequeue,
  };

  return (
    <div className="space-y-6 pb-8">
      <div className="flex flex-col gap-4 xl:flex-row xl:items-end xl:justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Runtime</h1>
          <p className="mt-1 max-w-3xl text-sm text-text-muted">
            Harness engine, WorkCommissions, workspace delivery, and operator actions.
          </p>
        </div>

        <button
          onClick={() => void refresh()}
          className="inline-flex items-center gap-2 rounded-lg border border-border bg-surface-1 px-3 py-2 text-sm text-text-secondary transition-colors hover:bg-surface-2 hover:text-text-primary"
        >
          <RefreshCw size={15} />
          Refresh
        </button>
      </div>

      <div className="flex flex-wrap gap-2">
        {SELECTORS.map((item) => (
          <button
            key={item.id}
            onClick={() => {
              setSelector(item.id);
              setSelectedID("");
              setResult(null);
              setTail(null);
            }}
            className={`rounded-full border px-3 py-1.5 text-xs transition-colors ${
              selector === item.id
                ? "border-accent-border bg-accent-wash text-accent"
                : "border-border bg-surface-1 text-text-secondary hover:bg-surface-2"
            }`}
          >
            {item.label}
          </button>
        ))}
      </div>

      {notice && (
        <div className="rounded-lg border border-success/30 bg-success/10 px-4 py-3 text-sm text-success">
          {notice}
        </div>
      )}

      <div className="grid gap-6 xl:grid-cols-[minmax(22rem,0.85fr)_minmax(0,1.15fr)]">
        <section className="rounded-xl border border-border bg-surface-1/80">
          <div className="flex items-center justify-between border-b border-border px-4 py-3">
            <div>
              <h2 className="text-sm font-medium text-text-primary">Queue</h2>
              <p className="mt-0.5 text-xs text-text-muted">{commissions.length} WorkCommission(s)</p>
            </div>
            {loading ? <Badge>Loading</Badge> : null}
          </div>

          <div className="max-h-[calc(100vh-18rem)] overflow-y-auto p-2">
            {commissions.length === 0 && (
              <div className="rounded-lg border border-dashed border-border px-4 py-10 text-center text-sm text-text-muted">
                Nothing in this lane.
              </div>
            )}

            {commissions.map((commission) => (
              <CommissionRow
                key={commission.id}
                commission={commission}
                selected={commission.id === selectedID}
                onSelect={() => setSelectedID(commission.id)}
              />
            ))}
          </div>
        </section>

        <section className="rounded-xl border border-border bg-surface-1/80">
          <div className="flex flex-col gap-3 border-b border-border px-4 py-4 xl:flex-row xl:items-start xl:justify-between">
            <div className="min-w-0">
              <div className="flex flex-wrap items-center gap-2">
                <h2 className="text-sm font-medium text-text-primary">Run Detail</h2>
                {selectedDetail ? <StateBadge state={selectedDetail.state} /> : null}
              </div>
              {selectedCommission ? (
                <div className="mt-2 flex flex-wrap items-center gap-2">
                  <MonoId id={selectedCommission.id} tone="accent" />
                  <MonoId id={selectedDetail?.decisionRef || "decision unknown"} />
                </div>
              ) : (
                <p className="mt-2 text-sm text-text-muted">Select a WorkCommission.</p>
              )}
            </div>

            <div className="flex flex-wrap items-center gap-2">
              {selectedDetail?.actions.map((action) => (
                <button
                  key={action.kind}
                  onClick={actionHandlers[action.kind]}
                  disabled={!action.enabled || Boolean(busyAction)}
                  title={action.reason}
                  className={actionButtonClass(action)}
                >
                  <ActionIcon kind={action.kind} />
                  {action.label}
                </button>
              ))}
              <button
                onClick={() => selectedDetail?.workspace.path && void openPathInIDE(selectedDetail.workspace.path)}
                disabled={!selectedDetail?.workspace.path}
                className="inline-flex items-center gap-2 rounded-lg border border-border bg-surface-2 px-3 py-2 text-xs text-text-secondary transition-colors hover:bg-surface-3 disabled:opacity-40"
                title="Open isolated harness workspace"
              >
                <FolderOpen size={14} />
                Workspace
              </button>
            </div>
          </div>

          {selectedDetail ? (
            <div className="space-y-4 p-4">
              <div className="grid gap-3 md:grid-cols-3">
                <Fact icon={GitBranch} label="Branch" value={selectedCommission?.scope?.target_branch || "unknown"} />
                <Fact icon={Timer} label="Valid Until" value={formatDateTime(selectedCommission?.valid_until || "")} />
                <Fact icon={CheckCircle2} label="Delivery" value={selectedCommission?.delivery_policy || "unknown"} />
              </div>

              {selectedCommission?.operator?.attention_reason && (
                <div className="rounded-lg border border-warning/30 bg-warning/10 px-4 py-3 text-sm text-warning">
                  {selectedCommission.operator?.attention_reason}
                </div>
              )}

              <section className="rounded-lg border border-border bg-surface-0/70 p-4">
                <div className="flex flex-wrap items-start justify-between gap-3">
                  <div>
                    <h3 className="text-xs font-medium uppercase text-text-muted">Workspace</h3>
                    <p className="mt-1 font-mono text-xs text-text-secondary">
                      {selectedDetail.workspace.path || "workspace unknown"}
                    </p>
                  </div>
                  <Badge tone={selectedDetail.workspace.canApply ? "accent" : "neutral"}>
                    {selectedDetail.workspace.diffState}
                  </Badge>
                </div>

                {selectedDetail.workspace.error ? (
                  <p className="mt-3 rounded-md border border-warning/30 bg-warning/10 px-3 py-2 text-xs text-warning">
                    {selectedDetail.workspace.error}
                  </p>
                ) : null}

                <FactList
                  title="Changed files"
                  empty="No changed files detected."
                  items={selectedDetail.workspace.changedFiles}
                  mono
                />
                <FactList
                  title="Git status"
                  empty="Clean."
                  items={selectedDetail.workspace.gitStatus}
                  mono
                />
                <FactList
                  title="Diff stat"
                  empty="Empty."
                  items={selectedDetail.workspace.diffStat}
                  mono
                />
              </section>

              <section className="rounded-lg border border-border bg-surface-0/70 p-4">
                <div className="flex flex-wrap items-center justify-between gap-3">
                  <h3 className="text-xs font-medium uppercase text-text-muted">Runtime</h3>
                  <Badge tone={selectedDetail.runtime.active ? "accent" : "neutral"}>
                    {selectedDetail.runtime.active ? "Active" : "No active run"}
                  </Badge>
                </div>
                {hasRuntimeFacts(selectedDetail) ? (
                  <div className="mt-3 grid gap-2 md:grid-cols-2">
                    <RuntimeFact label="Phase" value={selectedDetail.runtime.phase} />
                    <RuntimeFact label="Sub-state" value={selectedDetail.runtime.subState} />
                    <RuntimeFact label="Session" value={selectedDetail.runtime.sessionID} mono />
                    <RuntimeFact label="Turn" value={selectedDetail.runtime.lastTurnID} mono />
                    <RuntimeFact label="Last event" value={selectedDetail.runtime.lastEvent} />
                    <RuntimeFact label="Updated" value={formatDateTime(selectedDetail.runtime.statusUpdatedAt)} />
                  </div>
                ) : (
                  <p className="mt-3 text-sm text-text-muted">No active runtime detail for this commission.</p>
                )}
                {selectedDetail.runtime.preview ? (
                  <p className="mt-3 rounded-md border border-border bg-surface-2/50 px-3 py-2 text-xs text-text-secondary">
                    {selectedDetail.runtime.preview}
                  </p>
                ) : null}
              </section>

              <section className="rounded-lg border border-border bg-surface-0/70 p-4">
                <div className="flex flex-wrap items-center justify-between gap-3">
                  <h3 className="text-xs font-medium uppercase text-text-muted">Evidence</h3>
                  <Badge>{selectedDetail.evidence.requiredCount} required</Badge>
                </div>
                <FactList
                  title="Requirements"
                  empty="No evidence requirements declared."
                  items={selectedDetail.evidence.requirements}
                />
                <FactList
                  title="Latest measure"
                  empty="No measure outcome recorded."
                  items={selectedDetail.evidence.latestMeasure ? [selectedDetail.evidence.latestMeasure] : []}
                />
                <FactList
                  title="Terminal outcome"
                  empty="No terminal outcome recorded."
                  items={selectedDetail.evidence.terminal ? [selectedDetail.evidence.terminal] : []}
                />
              </section>

              <section className="rounded-lg border border-border bg-surface-0/70 p-4">
                <div className="flex flex-wrap items-center justify-between gap-3">
                  <h3 className="text-xs font-medium uppercase text-text-muted">Tail</h3>
                  <span className="font-mono text-[11px] text-text-muted">
                    {selectedDetail.tail.followCommand}
                  </span>
                </div>
                <FactList
                  title="Recent runtime events"
                  empty="No tail lines loaded."
                  items={selectedDetail.tail.lines}
                  mono
                />
              </section>
            </div>
          ) : (
            <div className="p-12 text-center text-sm text-text-muted">
              No WorkCommission selected.
            </div>
          )}
        </section>
      </div>
    </div>
  );
}

function CommissionRow({
  commission,
  selected,
  onSelect,
}: {
  commission: WorkCommission;
  selected: boolean;
  onSelect: () => void;
}) {
  const state = normalizeHarnessCommissionState(commission);

  return (
    <button
      onClick={onSelect}
      className={`mb-2 w-full rounded-lg border px-3 py-3 text-left transition-colors ${
        selected
          ? "border-accent-border bg-accent-wash"
          : "border-border bg-surface-0/60 hover:bg-surface-2"
      }`}
    >
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <MonoId id={commission.id} tone={selected ? "accent" : "neutral"} />
            <StateBadge state={state} />
          </div>
          <p className="mt-2 truncate font-mono text-xs text-text-secondary">
            {commission.decision_ref || "decision:unknown"}
          </p>
        </div>
        {commission.operator?.attention ? <Badge tone="warning">Attention</Badge> : null}
      </div>

      <div className="mt-3 flex flex-wrap gap-2">
        {(commission.lockset ?? []).slice(0, 3).map((path) => (
          <Pill key={path}>{path}</Pill>
        ))}
      </div>
    </button>
  );
}

function Fact({
  icon: Icon,
  label,
  value,
}: {
  icon: typeof GitBranch;
  label: string;
  value: string;
}) {
  return (
    <div className="rounded-lg border border-border bg-surface-2/50 px-3 py-3">
      <div className="flex items-center gap-2 text-[11px] uppercase text-text-muted">
        <Icon size={13} />
        {label}
      </div>
      <p className="mt-2 truncate text-sm text-text-primary">{value}</p>
    </div>
  );
}

function StateBadge({ state }: { state: HarnessCommissionStateView }) {
  return <Badge tone={state.tone}>{state.label}</Badge>;
}

function ActionIcon({ kind }: { kind: HarnessActionKind }) {
  const Icon = {
    result: FileText,
    tail: Terminal,
    apply: Download,
    cancel: Ban,
    requeue: RotateCcw,
  }[kind];

  return <Icon size={14} />;
}

function actionButtonClass(action: HarnessActionView): string {
  const base = "inline-flex items-center gap-2 rounded-lg border px-3 py-2 text-xs transition-colors disabled:opacity-40";
  const variants: Record<HarnessActionKind, string> = {
    result: "border-border bg-surface-2 text-text-secondary hover:bg-surface-3",
    tail: "border-border bg-surface-2 text-text-secondary hover:bg-surface-3",
    apply: "border-accent-border bg-accent-wash text-accent hover:bg-accent/15",
    cancel: "border-danger/30 bg-danger/10 text-danger hover:bg-danger/20",
    requeue: "border-border bg-surface-2 text-text-secondary hover:bg-surface-3",
  };

  return `${base} ${variants[action.kind]}`;
}

function FactList({
  title,
  empty,
  items,
  mono,
}: {
  title: string;
  empty: string;
  items: string[];
  mono?: boolean;
}) {
  return (
    <div className="mt-4">
      <h4 className="mb-2 text-[11px] font-medium uppercase text-text-muted">{title}</h4>
      {items.length === 0 ? (
        <p className="text-xs text-text-muted">{empty}</p>
      ) : (
        <div className="flex flex-wrap gap-2">
          {items.map((item) => (
            <span
              key={item}
              className={`rounded-md border border-border bg-surface-2 px-2 py-1 text-[11px] text-text-secondary ${
                mono ? "font-mono" : ""
              }`}
            >
              {item}
            </span>
          ))}
        </div>
      )}
    </div>
  );
}

function RuntimeFact({
  label,
  value,
  mono,
}: {
  label: string;
  value: string;
  mono?: boolean;
}) {
  return (
    <div className="rounded-md border border-border bg-surface-2/50 px-3 py-2">
      <div className="text-[11px] uppercase text-text-muted">{label}</div>
      <div className={`mt-1 truncate text-xs text-text-secondary ${mono ? "font-mono" : ""}`}>
        {value || "unknown"}
      </div>
    </div>
  );
}

function hasRuntimeFacts(detail: HarnessCockpitDetail): boolean {
  const fields = [
    detail.runtime.phase,
    detail.runtime.subState,
    detail.runtime.sessionID,
    detail.runtime.lastEvent,
    detail.runtime.lastTurnID,
  ];

  return fields.some((field) => field.trim() !== "");
}

function formatDateTime(value: string): string {
  if (!value) {
    return "unknown";
  }

  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }

  return date.toLocaleString();
}
