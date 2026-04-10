import { useEffect, useState } from "react";

import { EventsOn } from "../../wailsjs/runtime/runtime";
import {
  adoptProblemCandidate,
  dismissProblemCandidate,
  getDashboard,
  getGovernanceOverview,
  type CoverageModule,
  type DashboardData,
  type DecisionSummary,
  type GovernanceOverview,
  type ProblemCandidate,
  type ProblemSummary,
} from "../lib/api";
import { reportError } from "../lib/errors";

type NavigateFn = (page: "dashboard" | "problems" | "decisions", id?: string) => void;

export function Dashboard({ onNavigate }: { onNavigate: NavigateFn }) {
  const [data, setData] = useState<DashboardData | null>(null);
  const [overview, setOverview] = useState<GovernanceOverview | null>(null);
  const [loading, setLoading] = useState(true);

  const refresh = async () => {
    setLoading(true);

    try {
      const [nextDashboard, nextOverview] = await Promise.all([
        getDashboard(),
        getGovernanceOverview(),
      ]);

      setData(nextDashboard);
      setOverview(nextOverview);
    } catch (error) {
      reportError(error, "dashboard");
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
      stopStale = EventsOn("scan.stale", () => {
        void refresh();
      });
      stopDrift = EventsOn("scan.drift", () => {
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

  const handleAdoptCandidate = async (candidateID: string) => {
    try {
      const problem = await adoptProblemCandidate(candidateID);
      await refresh();
      onNavigate("problems", problem.id);
    } catch (error) {
      reportError(error, "adopt problem candidate");
    }
  };

  const handleDismissCandidate = async (candidateID: string) => {
    try {
      await dismissProblemCandidate(candidateID);
      await refresh();
    } catch (error) {
      reportError(error, "dismiss problem candidate");
    }
  };

  if (loading && (!data || !overview)) {
    return (
      <div className="p-8 text-center">
        <p className="text-sm text-text-muted">Loading...</p>
      </div>
    );
  }

  if (!data || !overview) {
    return (
      <div className="p-8 text-center">
        <p className="text-sm text-text-muted">Governance data is unavailable.</p>
      </div>
    );
  }

  return (
    <div className="space-y-8 pb-8">
      <div className="mb-2">
        <p className="font-mono text-xs uppercase tracking-[1.2px] text-text-muted">VERIFY</p>
        <p className="text-xs text-text-muted mt-0.5">Decision governance, evidence health, and follow-up work</p>
      </div>
      <div className="flex items-end justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">{data.project_name}</h1>
          <p className="mt-1 text-sm text-text-muted">
            Decision execution, verification pressure, and follow-up governance work in one view.
          </p>
        </div>

        <div className="rounded-xl border border-border bg-surface-1 px-4 py-3 text-right">
          <p className="text-[11px] uppercase tracking-[0.22em] text-text-muted">Last Scan</p>
          <p className="mt-1 font-mono text-xs text-text-secondary">
            {overview.last_scan_at || "Not scanned yet"}
          </p>
        </div>
      </div>

      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-6">
        <StatCard label="Problems" count={data.problem_count} onClick={() => onNavigate("problems")} />
        <StatCard label="Decisions" count={data.decision_count} onClick={() => onNavigate("decisions")} />
        <StatCard label="Portfolios" count={data.portfolio_count} />
        <StatCard label="Coverage" count={overview.coverage.governed_percent} suffix="%" variant={overview.coverage.blind_count > 0 ? "warning" : "default"} />
        <StatCard label="Findings" count={overview.findings.length} variant={overview.findings.length > 0 ? "warning" : "default"} />
        <StatCard label="Candidates" count={overview.problem_candidates.length} variant={overview.problem_candidates.length > 0 ? "accent" : "default"} />
      </div>

      <div className="grid gap-6 xl:grid-cols-[minmax(0,1.15fr)_minmax(0,0.85fr)]">
        <div className="space-y-6">
          <Section title="Governance Findings">
            {overview.findings.length === 0 ? (
              <EmptyState text="No stale or drift findings." />
            ) : (
              <div className="space-y-2">
                {overview.findings.map((finding) => (
                  <button
                    key={finding.id}
                    onClick={() => {
                      if (finding.kind === "DecisionRecord" && finding.artifact_ref) {
                        onNavigate("decisions", finding.artifact_ref);
                      }
                    }}
                    className="w-full rounded-xl border border-border bg-surface-1 px-4 py-3 text-left transition-colors hover:border-border-bright hover:bg-surface-2"
                  >
                    <div className="flex items-start justify-between gap-3">
                      <div>
                        <p className="text-sm font-medium text-text-primary">{finding.title}</p>
                        <p className="mt-1 text-xs uppercase tracking-[0.2em] text-warning/80">
                          {finding.category.replaceAll("_", " ")}
                        </p>
                      </div>

                      <div className="text-right text-[11px] text-text-muted">
                        {finding.drift_count > 0 && <p>{finding.drift_count} drift file(s)</p>}
                        {finding.days_stale > 0 && <p>{finding.days_stale} day(s) stale</p>}
                      </div>
                    </div>

                    <p className="mt-2 text-sm text-text-secondary">{finding.reason}</p>
                  </button>
                ))}
              </div>
            )}
          </Section>

          <Section title="Follow-up Candidates">
            {overview.problem_candidates.length === 0 ? (
              <EmptyState text="No follow-up problems surfaced by the latest governance scan." />
            ) : (
              <div className="space-y-3">
                {overview.problem_candidates.map((candidate) => (
                  <CandidateCard
                    key={candidate.id}
                    candidate={candidate}
                    onAdopt={() => void handleAdoptCandidate(candidate.id)}
                    onDismiss={() => void handleDismissCandidate(candidate.id)}
                    onOpenSource={() => {
                      if (candidate.source_artifact_ref) {
                        onNavigate("decisions", candidate.source_artifact_ref);
                      }
                    }}
                  />
                ))}
              </div>
            )}
          </Section>
        </div>

        <div className="space-y-6">
          <Section title="Module Coverage">
            <CoverageSummary overview={overview} />
          </Section>

          <Section title="Recent Decisions">
            {data.recent_decisions.length === 0 ? (
              <EmptyState text="No active decisions" />
            ) : (
              <div className="space-y-2">
                {data.recent_decisions.map((decision: DecisionSummary) => (
                  <button
                    key={decision.id}
                    onClick={() => onNavigate("decisions", decision.id)}
                    className="w-full rounded-xl border border-border bg-surface-1 px-4 py-3 text-left transition-colors hover:border-border-bright hover:bg-surface-2"
                  >
                    <div className="flex items-center justify-between gap-3">
                      <span className="text-sm font-medium text-text-primary">{decision.selected_title}</span>
                      <span className="font-mono text-[11px] text-text-muted">{decision.id}</span>
                    </div>
                    <div className="mt-2 flex flex-wrap items-center gap-3 text-xs text-text-muted">
                      <span>WLNK: {decision.weakest_link}</span>
                      {decision.valid_until && <span>Valid until {decision.valid_until}</span>}
                    </div>
                  </button>
                ))}
              </div>
            )}
          </Section>

          <Section title="Recent Problems">
            {data.recent_problems.length === 0 ? (
              <EmptyState text="No active problems" />
            ) : (
              <div className="space-y-2">
                {data.recent_problems.map((problem: ProblemSummary) => (
                  <button
                    key={problem.id}
                    onClick={() => onNavigate("problems", problem.id)}
                    className="w-full rounded-xl border border-border bg-surface-1 px-4 py-3 text-left transition-colors hover:border-border-bright hover:bg-surface-2"
                  >
                    <div className="flex items-center justify-between gap-3">
                      <span className="text-sm font-medium text-text-primary">{problem.title}</span>
                      <span className="font-mono text-[11px] text-text-muted">{problem.id}</span>
                    </div>
                    <p className="mt-2 text-sm text-text-secondary">{problem.signal}</p>
                  </button>
                ))}
              </div>
            )}
          </Section>
        </div>
      </div>
    </div>
  );
}

function CoverageSummary({ overview }: { overview: GovernanceOverview }) {
  const impactedModules = overview.coverage.modules.filter((module) => module.impacted);
  const displayedModules = impactedModules.length > 0
    ? impactedModules
    : overview.coverage.modules.slice(0, 6);

  return (
    <div className="space-y-4">
      <div className="rounded-xl border border-border bg-surface-1 px-4 py-4">
        <div className="flex items-center justify-between gap-4">
          <div>
            <p className="text-[11px] uppercase tracking-[0.22em] text-text-muted">Governed Surface</p>
            <p className="mt-1 text-3xl font-semibold text-text-primary">
              {overview.coverage.governed_percent}%
            </p>
          </div>

          <div className="text-right text-xs text-text-secondary">
            <p>{overview.coverage.covered_count} covered</p>
            <p>{overview.coverage.partial_count} partial</p>
            <p>{overview.coverage.blind_count} blind</p>
          </div>
        </div>
      </div>

      <div className="space-y-2">
        {displayedModules.map((module) => (
          <CoverageModuleCard key={`${module.id}-${module.path}`} module={module} />
        ))}

        {displayedModules.length === 0 && (
          <EmptyState text="Run a scan to populate module coverage." />
        )}
      </div>
    </div>
  );
}

function CoverageModuleCard({ module }: { module: CoverageModule }) {
  const statusClassName =
    module.status === "covered"
      ? "border-success/20 bg-success/10 text-success"
      : module.status === "partial"
        ? "border-warning/20 bg-warning/10 text-warning"
        : "border-danger/20 bg-danger/10 text-danger";

  return (
    <div className="rounded-xl border border-border bg-surface-1 px-4 py-3">
      <div className="flex items-start justify-between gap-3">
        <div>
          <p className="text-sm font-medium text-text-primary">{module.path || "(root)"}</p>
          <p className="mt-1 text-xs text-text-muted">
            {module.lang} · {module.decision_count} decision(s)
          </p>
        </div>

        <div className="flex items-center gap-2">
          {module.impacted && (
            <span className="rounded-full border border-accent/20 bg-accent/10 px-2 py-0.5 text-[11px] text-accent">
              impacted
            </span>
          )}
          <span className={`rounded-full border px-2 py-0.5 text-[11px] ${statusClassName}`}>
            {module.status}
          </span>
        </div>
      </div>

      {module.files && module.files.length > 0 && (
        <div className="mt-3 space-y-1">
          {module.files.map((filePath) => (
            <p key={filePath} className="font-mono text-[11px] text-text-muted">
              {filePath}
            </p>
          ))}
        </div>
      )}
    </div>
  );
}

function CandidateCard({
  candidate,
  onAdopt,
  onDismiss,
  onOpenSource,
}: {
  candidate: ProblemCandidate;
  onAdopt: () => void;
  onDismiss: () => void;
  onOpenSource: () => void;
}) {
  return (
    <div className="rounded-xl border border-warning/20 bg-surface-1 px-4 py-4">
      <div className="flex items-start justify-between gap-4">
        <div>
          <p className="text-sm font-medium text-text-primary">{candidate.title}</p>
          <p className="mt-1 text-xs uppercase tracking-[0.2em] text-warning/80">
            {candidate.category.replaceAll("_", " ")}
          </p>
        </div>

        <div className="flex shrink-0 items-center gap-2">
          {candidate.source_artifact_ref && (
            <button
              onClick={onOpenSource}
              className="rounded-lg border border-border bg-surface-2 px-3 py-1.5 text-xs text-text-secondary transition-colors hover:bg-surface-3"
            >
              Open source
            </button>
          )}
          <button
            onClick={onDismiss}
            className="rounded-lg border border-border bg-surface-2 px-3 py-1.5 text-xs text-text-secondary transition-colors hover:bg-surface-3"
          >
            Dismiss
          </button>
          <button
            onClick={onAdopt}
            className="rounded-full bg-accent px-3 py-1.5 text-xs text-surface-0 transition-colors hover:bg-accent-hover"
          >
            Adopt problem
          </button>
        </div>
      </div>

      <p className="mt-3 text-sm text-text-secondary">{candidate.signal}</p>
      <p className="mt-2 text-xs text-text-muted">Acceptance: {candidate.acceptance}</p>
    </div>
  );
}

function StatCard({
  label,
  count,
  suffix = "",
  variant = "default",
  onClick,
}: {
  label: string;
  count: number;
  suffix?: string;
  variant?: "default" | "warning" | "accent";
  onClick?: () => void;
}) {
  const componentClassName = onClick
    ? "cursor-pointer hover:bg-surface-2 hover:border-border-bright"
    : "";
  const toneClassName =
    variant === "warning"
      ? "text-warning"
      : variant === "accent"
        ? "text-accent"
        : "text-text-primary";
  const Component = onClick ? "button" : "div";

  return (
    <Component
      onClick={onClick}
      className={`rounded-xl border border-border bg-surface-1 px-4 py-4 text-left transition-colors ${componentClassName}`}
    >
      <p className="text-[11px] uppercase tracking-[0.22em] text-text-muted">{label}</p>
      <p className={`mt-2 text-2xl font-semibold ${toneClassName}`}>
        {count}
        {suffix}
      </p>
    </Component>
  );
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div>
      <h3 className="mb-3 text-xs uppercase tracking-[0.24em] text-text-muted">{title}</h3>
      {children}
    </div>
  );
}

function EmptyState({ text }: { text: string }) {
  return (
    <div className="rounded-xl border border-dashed border-border bg-surface-1/60 px-4 py-8 text-center">
      <p className="text-sm text-text-muted">{text}</p>
    </div>
  );
}
