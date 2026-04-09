import { useEffect, useState } from "react";
import {
  listProblems,
  getProblem,
  type ProblemSummary,
  type ProblemDetail,
} from "../lib/api";

type NavigateFn = (page: "dashboard" | "problems" | "decisions", id?: string) => void;

export function Problems({
  selectedId,
  onNavigate,
}: {
  selectedId: string | null;
  onNavigate: NavigateFn;
}) {
  const [problems, setProblems] = useState<ProblemSummary[]>([]);
  const [detail, setDetail] = useState<ProblemDetail | null>(null);
  const [activeId, setActiveId] = useState<string | null>(selectedId);

  useEffect(() => {
    listProblems().then(setProblems).catch(console.error);
  }, []);

  useEffect(() => {
    if (!activeId) {
      setDetail(null);
      return;
    }
    getProblem(activeId).then(setDetail).catch(console.error);
  }, [activeId]);

  return (
    <div className="flex gap-6 h-[calc(100vh-7rem)]">
      <div className="w-80 shrink-0 overflow-y-auto space-y-1">
        {problems.map((p) => (
          <button
            key={p.id}
            onClick={() => setActiveId(p.id)}
            className={`w-full text-left px-4 py-3 rounded-lg transition-colors border ${
              activeId === p.id
                ? "bg-surface-2 border-accent/30"
                : "bg-surface-1 border-transparent hover:bg-surface-2 hover:border-border"
            }`}
          >
            <div className="flex items-center justify-between">
              <span className="text-sm font-medium truncate">{p.title}</span>
              <ModeBadge mode={p.mode} />
            </div>
            <p className="text-xs text-text-secondary mt-1 line-clamp-2">{p.signal}</p>
            <div className="flex items-center gap-2 mt-2">
              <span className="text-xs text-text-muted font-mono">{p.id}</span>
              {p.reversibility && <ReversibilityBadge value={p.reversibility} />}
            </div>
          </button>
        ))}
        {problems.length === 0 && (
          <p className="text-sm text-text-muted text-center py-8">No active problems</p>
        )}
      </div>

      <div className="flex-1 overflow-y-auto">
        {detail ? (
          <ProblemDetailPanel detail={detail} onNavigate={onNavigate} />
        ) : activeId ? (
          <p className="text-sm text-text-muted py-8 text-center">Loading...</p>
        ) : (
          <p className="text-sm text-text-muted py-8 text-center">
            Select a problem to view details
          </p>
        )}
      </div>
    </div>
  );
}

function ProblemDetailPanel({ detail, onNavigate }: { detail: ProblemDetail; onNavigate: NavigateFn }) {
  return (
    <div className="space-y-6">
      <div>
        <div className="flex items-center gap-3 mb-2">
          <ModeBadge mode={detail.mode} />
          <StatusBadge status={detail.status} />
        </div>
        <h2 className="text-xl font-semibold">{detail.title}</h2>
        <p className="text-xs text-text-muted font-mono mt-1">{detail.id}</p>
      </div>

      <Field label="Signal" value={detail.signal} />

      {detail.constraints?.length > 0 && <ListField label="Constraints" items={detail.constraints} />}
      {detail.optimization_targets?.length > 0 && <ListField label="Optimization Targets" items={detail.optimization_targets} />}
      {detail.observation_indicators?.length > 0 && <ListField label="Observation Indicators (Anti-Goodhart)" items={detail.observation_indicators} />}
      {detail.acceptance && <Field label="Acceptance" value={detail.acceptance} />}

      <div className="grid grid-cols-2 gap-4">
        {detail.blast_radius && <Field label="Blast Radius" value={detail.blast_radius} />}
        {detail.reversibility && <Field label="Reversibility" value={detail.reversibility} />}
      </div>

      {detail.linked_portfolios?.length > 0 && (
        <div>
          <h4 className="text-xs text-text-muted uppercase tracking-wider mb-2">Solution Portfolios</h4>
          {detail.linked_portfolios.map((p) => (
            <div key={p.id} className="text-sm text-accent px-3 py-1.5">
              {p.title} <span className="text-text-muted font-mono ml-2">{p.id}</span>
            </div>
          ))}
        </div>
      )}

      {detail.linked_decisions?.length > 0 && (
        <div>
          <h4 className="text-xs text-text-muted uppercase tracking-wider mb-2">Decisions</h4>
          {detail.linked_decisions.map((d) => (
            <button key={d.id} onClick={() => onNavigate("decisions", d.id)} className="block text-left text-sm text-accent hover:text-accent-hover px-3 py-1.5">
              {d.title} <span className="text-text-muted font-mono ml-2">{d.id}</span>
            </button>
          ))}
        </div>
      )}
    </div>
  );
}

function Field({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <h4 className="text-xs text-text-muted uppercase tracking-wider mb-1">{label}</h4>
      <p className="text-sm text-text-primary bg-surface-1 rounded-lg px-4 py-3 border border-border">{value}</p>
    </div>
  );
}

function ListField({ label, items }: { label: string; items: string[] }) {
  return (
    <div>
      <h4 className="text-xs text-text-muted uppercase tracking-wider mb-1">{label}</h4>
      <ul className="space-y-1">
        {items.map((item, i) => (
          <li key={i} className="text-sm text-text-primary bg-surface-1 rounded-lg px-4 py-2 border border-border">{item}</li>
        ))}
      </ul>
    </div>
  );
}

function ModeBadge({ mode }: { mode: string }) {
  const colors: Record<string, string> = {
    tactical: "bg-blue-500/10 text-blue-400 border-blue-500/20",
    standard: "bg-accent/10 text-accent border-accent/20",
    deep: "bg-purple-500/10 text-purple-400 border-purple-500/20",
    note: "bg-surface-2 text-text-muted border-border",
  };
  return <span className={`text-xs px-2 py-0.5 rounded-full border ${colors[mode] ?? colors.note}`}>{mode}</span>;
}

function StatusBadge({ status }: { status: string }) {
  const colors: Record<string, string> = {
    active: "bg-success/10 text-success border-success/20",
    refresh_due: "bg-warning/10 text-warning border-warning/20",
    superseded: "bg-surface-2 text-text-muted border-border",
  };
  return <span className={`text-xs px-2 py-0.5 rounded-full border ${colors[status] ?? "bg-surface-2 text-text-muted border-border"}`}>{status}</span>;
}

function ReversibilityBadge({ value }: { value: string }) {
  const colors: Record<string, string> = { low: "text-danger", medium: "text-warning", high: "text-success" };
  return <span className={`text-xs ${colors[value] ?? "text-text-muted"}`}>{value} reversibility</span>;
}
