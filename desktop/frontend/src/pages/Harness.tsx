import { useCallback, useEffect, useMemo, useState } from "react";
import {
  Ban,
  CheckCircle2,
  Download,
  FolderOpen,
  GitBranch,
  RefreshCw,
  RotateCcw,
  Timer,
} from "lucide-react";

import {
  applyHarnessResult,
  cancelCommission,
  getHarnessResult,
  listCommissions,
  openPathInIDE,
  requeueCommission,
  type HarnessRunResult,
  type WorkCommission,
} from "../lib/api";
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
  const [loading, setLoading] = useState(true);
  const [busyAction, setBusyAction] = useState("");
  const [notice, setNotice] = useState("");

  const selectedCommission = useMemo(
    () => commissions.find((commission) => commission.id === selectedID) ?? null,
    [commissions, selectedID],
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
                {selectedCommission ? <StateBadge state={selectedCommission.state} /> : null}
              </div>
              {selectedCommission ? (
                <div className="mt-2 flex flex-wrap items-center gap-2">
                  <MonoId id={selectedCommission.id} tone="accent" />
                  <MonoId id={selectedCommission.decision_ref || "decision:unknown"} />
                </div>
              ) : (
                <p className="mt-2 text-sm text-text-muted">Select a WorkCommission.</p>
              )}
            </div>

            <div className="flex flex-wrap items-center gap-2">
              <button
                onClick={() => result?.workspace && void openPathInIDE(result.workspace)}
                disabled={!result?.workspace}
                className="inline-flex items-center gap-2 rounded-lg border border-border bg-surface-2 px-3 py-2 text-xs text-text-secondary transition-colors hover:bg-surface-3 disabled:opacity-40"
              >
                <FolderOpen size={14} />
                Workspace
              </button>
              <button
                onClick={handleRequeue}
                disabled={!selectedCommission || !canRequeue(selectedCommission) || Boolean(busyAction)}
                className="inline-flex items-center gap-2 rounded-lg border border-border bg-surface-2 px-3 py-2 text-xs text-text-secondary transition-colors hover:bg-surface-3 disabled:opacity-40"
              >
                <RotateCcw size={14} />
                Requeue
              </button>
              <button
                onClick={handleCancel}
                disabled={!selectedCommission || isTerminal(selectedCommission.state) || Boolean(busyAction)}
                className="inline-flex items-center gap-2 rounded-lg border border-danger/30 bg-danger/10 px-3 py-2 text-xs text-danger transition-colors hover:bg-danger/20 disabled:opacity-40"
              >
                <Ban size={14} />
                Cancel
              </button>
              <button
                onClick={handleApply}
                disabled={!result?.can_apply || Boolean(busyAction)}
                className="inline-flex items-center gap-2 rounded-lg border border-accent-border bg-accent-wash px-3 py-2 text-xs text-accent transition-colors hover:bg-accent/15 disabled:opacity-40"
              >
                <Download size={14} />
                Apply
              </button>
            </div>
          </div>

          {selectedCommission ? (
            <div className="space-y-4 p-4">
              <div className="grid gap-3 md:grid-cols-3">
                <Fact icon={GitBranch} label="Branch" value={selectedCommission.scope?.target_branch || "unknown"} />
                <Fact icon={Timer} label="Valid Until" value={formatDateTime(selectedCommission.valid_until || "")} />
                <Fact icon={CheckCircle2} label="Delivery" value={selectedCommission.delivery_policy || "unknown"} />
              </div>

              {selectedCommission.operator?.attention_reason && (
                <div className="rounded-lg border border-warning/30 bg-warning/10 px-4 py-3 text-sm text-warning">
                  {selectedCommission.operator.attention_reason}
                </div>
              )}

              {result?.changed_files?.length ? (
                <div>
                  <h3 className="mb-2 text-xs font-medium uppercase text-text-muted">Changed files</h3>
                  <div className="flex flex-wrap gap-2">
                    {result.changed_files.map((file) => (
                      <span
                        key={file}
                        className="rounded-full border border-border bg-surface-2 px-2 py-1 font-mono text-[11px] text-text-secondary"
                      >
                        {file}
                      </span>
                    ))}
                  </div>
                </div>
              ) : null}

              <pre className="max-h-[32rem] overflow-auto rounded-lg border border-border bg-surface-0 p-4 font-mono text-xs leading-5 text-text-secondary">
                {result?.raw || "No run result yet."}
              </pre>
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
            <StateBadge state={commission.state} />
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

function StateBadge({ state }: { state: string }) {
  if (state === "completed" || state === "completed_with_projection_debt") {
    return <Badge tone="success">{state}</Badge>;
  }
  if (state === "failed" || state === "cancelled" || state === "expired") {
    return <Badge tone="danger">{state}</Badge>;
  }
  if (state.startsWith("blocked") || state === "needs_human_review") {
    return <Badge tone="warning">{state}</Badge>;
  }
  if (state === "running" || state === "preflighting") {
    return <Badge tone="accent">{state}</Badge>;
  }
  return <Badge>{state || "unknown"}</Badge>;
}

function canRequeue(commission: WorkCommission): boolean {
  if (isTerminal(commission.state)) {
    return false;
  }

  return commission.state !== "draft";
}

function isTerminal(state: string): boolean {
  return ["completed", "completed_with_projection_debt", "cancelled", "expired"].includes(state);
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
