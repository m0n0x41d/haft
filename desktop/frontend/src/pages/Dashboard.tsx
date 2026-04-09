import { useEffect, useState } from "react";
import {
  getDashboard,
  type DashboardData,
  type ProblemSummary,
  type DecisionSummary,
} from "../lib/api";

type NavigateFn = (page: "dashboard" | "problems" | "decisions", id?: string) => void;

export function Dashboard({ onNavigate }: { onNavigate: NavigateFn }) {
  const [data, setData] = useState<DashboardData | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    getDashboard()
      .then(setData)
      .catch((e: Error) => setError(e.message || String(e)));
  }, []);

  if (error) {
    return (
      <div className="p-8 text-center">
        <p className="text-danger text-sm">{error}</p>
      </div>
    );
  }

  if (!data) {
    return (
      <div className="p-8 text-center">
        <p className="text-text-muted text-sm">Loading...</p>
      </div>
    );
  }

  return (
    <div className="space-y-8">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">{data.project_name}</h1>
        <p className="text-sm text-text-muted mt-1">Engineering reasoning workspace</p>
      </div>

      <div className="grid grid-cols-5 gap-3">
        <StatCard label="Problems" count={data.problem_count} onClick={() => onNavigate("problems")} />
        <StatCard label="Decisions" count={data.decision_count} onClick={() => onNavigate("decisions")} />
        <StatCard label="Portfolios" count={data.portfolio_count} />
        <StatCard label="Notes" count={data.note_count} />
        <StatCard label="Stale" count={data.stale_count} variant={data.stale_count > 0 ? "warning" : "default"} />
      </div>

      <Section title="Recent Problems">
        {data.recent_problems.length === 0 ? (
          <EmptyState text="No active problems" />
        ) : (
          <div className="space-y-1">
            {data.recent_problems.map((p: ProblemSummary) => (
              <button
                key={p.id}
                onClick={() => onNavigate("problems", p.id)}
                className="w-full text-left px-4 py-3 rounded-lg bg-surface-1 hover:bg-surface-2 transition-colors border border-transparent hover:border-border"
              >
                <div className="flex items-center justify-between">
                  <span className="text-sm font-medium">{p.title}</span>
                  <span className="text-xs text-text-muted font-mono">{p.id}</span>
                </div>
                <p className="text-xs text-text-secondary mt-1 line-clamp-1">{p.signal}</p>
              </button>
            ))}
          </div>
        )}
      </Section>

      <Section title="Recent Decisions">
        {data.recent_decisions.length === 0 ? (
          <EmptyState text="No active decisions" />
        ) : (
          <div className="space-y-1">
            {data.recent_decisions.map((d: DecisionSummary) => (
              <button
                key={d.id}
                onClick={() => onNavigate("decisions", d.id)}
                className="w-full text-left px-4 py-3 rounded-lg bg-surface-1 hover:bg-surface-2 transition-colors border border-transparent hover:border-border"
              >
                <div className="flex items-center justify-between">
                  <span className="text-sm font-medium">{d.selected_title}</span>
                  <span className="text-xs text-text-muted font-mono">{d.id}</span>
                </div>
                <div className="flex items-center gap-3 mt-1">
                  <span className="text-xs text-text-secondary">WLNK: {d.weakest_link}</span>
                  {d.valid_until && <span className="text-xs text-text-muted">Valid until {d.valid_until}</span>}
                </div>
              </button>
            ))}
          </div>
        )}
      </Section>

      {data.stale_count > 0 && (
        <Section title="Stale Items">
          <div className="space-y-1">
            {data.stale_items.map((s) => (
              <div key={s.id} className="px-4 py-2 rounded-lg bg-surface-1 border border-warning/20">
                <div className="flex items-center justify-between">
                  <span className="text-sm text-warning">{s.title}</span>
                  <span className="text-xs text-text-muted font-mono">{s.kind}</span>
                </div>
              </div>
            ))}
          </div>
        </Section>
      )}
    </div>
  );
}

function StatCard({
  label,
  count,
  variant = "default",
  onClick,
}: {
  label: string;
  count: number;
  variant?: "default" | "warning";
  onClick?: () => void;
}) {
  const Component = onClick ? "button" : "div";
  return (
    <Component
      onClick={onClick}
      className={`rounded-lg p-4 bg-surface-1 border border-border text-left transition-colors ${
        onClick ? "hover:bg-surface-2 hover:border-border-bright cursor-pointer" : ""
      }`}
    >
      <p className="text-xs text-text-muted uppercase tracking-wider">{label}</p>
      <p className={`text-2xl font-semibold mt-1 font-mono ${variant === "warning" && count > 0 ? "text-warning" : "text-text-primary"}`}>
        {count}
      </p>
    </Component>
  );
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div>
      <h3 className="text-sm font-medium text-text-secondary mb-3 uppercase tracking-wider">{title}</h3>
      {children}
    </div>
  );
}

function EmptyState({ text }: { text: string }) {
  return (
    <div className="px-4 py-8 text-center">
      <p className="text-sm text-text-muted">{text}</p>
    </div>
  );
}
