import { useEffect, useState } from "react";

import { EventsOn } from "../../wailsjs/runtime/runtime";
import {
  adoptProblemCandidate,
  dismissProblemCandidate,
  getConfig,
  getDashboard,
  getGovernanceOverview,
  implementDecision,
  reopenDecision,
  waiveDecision,
  type CoverageModule,
  type DashboardData,
  type DesktopConfig,
  type DecisionSummary,
  type GovernanceFinding,
  type GovernanceOverview,
  type ProblemCandidate,
} from "../lib/api";
import { reportError } from "../lib/errors";
import { buildRecentActivity, type DashboardActivityItem } from "./dashboardActivity";
import { getDecisionImplementActionState } from "./dashboardDecisionActions";
import { getGovernanceFindingActionState } from "./dashboardGovernanceActions";

type NavigateFn = (
  page: "dashboard" | "problems" | "decisions" | "tasks",
  id?: string,
) => void;

export function Dashboard({ onNavigate }: { onNavigate: NavigateFn }) {
  const [data, setData] = useState<DashboardData | null>(null);
  const [overview, setOverview] = useState<GovernanceOverview | null>(null);
  const [config, setConfig] = useState<DesktopConfig | null>(null);
  const [implementingDecisionIDs, setImplementingDecisionIDs] = useState<string[]>([]);
  const [findingActionKeys, setFindingActionKeys] = useState<string[]>([]);
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
    getConfig()
      .then(setConfig)
      .catch((error) => {
        reportError(error, "dashboard config");
      });
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

  const handleImplementDecision = async (decisionID: string) => {
    setImplementingDecisionIDs((currentDecisionIDs) => {
      if (currentDecisionIDs.includes(decisionID)) {
        return currentDecisionIDs;
      }

      return [...currentDecisionIDs, decisionID];
    });

    try {
      const task = await implementDecision(
        decisionID,
        config?.default_agent ?? "claude",
        config?.default_worktree ?? true,
        "",
      );

      onNavigate("tasks", task.id);
    } catch (error) {
      reportError(error, "implement decision");
    } finally {
      setImplementingDecisionIDs((currentDecisionIDs) =>
        currentDecisionIDs.filter((currentDecisionID) => currentDecisionID !== decisionID),
      );
    }
  };

  const startFindingAction = (actionKey: string) => {
    setFindingActionKeys((currentActionKeys) => {
      if (currentActionKeys.includes(actionKey)) {
        return currentActionKeys;
      }

      return [...currentActionKeys, actionKey];
    });
  };

  const finishFindingAction = (actionKey: string) => {
    setFindingActionKeys((currentActionKeys) =>
      currentActionKeys.filter((currentActionKey) => currentActionKey !== actionKey),
    );
  };

  const handleAdoptFinding = async (findingID: string, candidateID: string) => {
    const actionKey = buildFindingActionKey(findingID, "adopt");
    startFindingAction(actionKey);

    try {
      const problem = await adoptProblemCandidate(candidateID);
      await refresh();
      onNavigate("problems", problem.id);
    } catch (error) {
      reportError(error, "adopt governance finding");
    } finally {
      finishFindingAction(actionKey);
    }
  };

  const handleWaiveFinding = async (finding: GovernanceFinding) => {
    const decisionID = finding.artifact_ref;
    const reason = promptForGovernanceDecisionReason("waive", finding);

    if (!decisionID || reason === "") {
      return;
    }

    const actionKey = buildFindingActionKey(finding.id, "waive");
    startFindingAction(actionKey);

    try {
      await waiveDecision(decisionID, reason);
      await refresh();
      onNavigate("decisions", decisionID);
    } catch (error) {
      reportError(error, "waive governance finding");
    } finally {
      finishFindingAction(actionKey);
    }
  };

  const handleReopenFinding = async (finding: GovernanceFinding) => {
    const decisionID = finding.artifact_ref;
    const reason = promptForGovernanceDecisionReason("reopen", finding);

    if (!decisionID || reason === "") {
      return;
    }

    const actionKey = buildFindingActionKey(finding.id, "reopen");
    startFindingAction(actionKey);

    try {
      const problem = await reopenDecision(decisionID, reason);
      await refresh();
      onNavigate("problems", problem.id);
    } catch (error) {
      reportError(error, "reopen governance finding");
    } finally {
      finishFindingAction(actionKey);
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

  const recentActivity = buildRecentActivity(data.recent_problems, data.recent_decisions);

  return (
    <div className="space-y-8 pb-8">
      <div className="mb-2">
        <p className="font-mono text-xs uppercase tracking-[1.2px] text-text-muted">DASHBOARD</p>
        <p className="mt-0.5 text-xs text-text-muted">
          Unified operator view for active decisions, governance findings, and recent activity.
        </p>
      </div>
      <div className="flex items-end justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">{data.project_name}</h1>
          <p className="mt-1 text-sm text-text-muted">
            Decision execution, governance pressure, and artifact activity in one surface.
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
          <Section title="Active Decisions">
            {data.healthy_decisions.length === 0 &&
            data.pending_decisions.length === 0 &&
            data.unassessed_decisions.length === 0 ? (
              <EmptyState text="No active decisions." />
            ) : (
              <div className="space-y-4">
                <DecisionBucket
                  title="Shipped / Healthy"
                  decisions={data.healthy_decisions}
                  onOpenDecision={(decisionID) => onNavigate("decisions", decisionID)}
                  onImplementDecision={handleImplementDecision}
                  implementingDecisionIDs={implementingDecisionIDs}
                />
                <DecisionBucket
                  title="Pending"
                  decisions={data.pending_decisions}
                  onOpenDecision={(decisionID) => onNavigate("decisions", decisionID)}
                  onImplementDecision={handleImplementDecision}
                  implementingDecisionIDs={implementingDecisionIDs}
                />
                <DecisionBucket
                  title="Unassessed"
                  decisions={data.unassessed_decisions}
                  onOpenDecision={(decisionID) => onNavigate("decisions", decisionID)}
                  onImplementDecision={handleImplementDecision}
                  implementingDecisionIDs={implementingDecisionIDs}
                />
              </div>
            )}
          </Section>

          <Section title="Governance Findings">
            {overview.findings.length === 0 ? (
              <EmptyState text="No stale or drift findings." />
            ) : (
              <div className="space-y-3">
                {overview.findings.map((finding) => (
                  <GovernanceFindingCard
                    key={finding.id}
                    finding={finding}
                    candidates={overview.problem_candidates}
                    isAdopting={findingActionKeys.includes(buildFindingActionKey(finding.id, "adopt"))}
                    isWaiving={findingActionKeys.includes(buildFindingActionKey(finding.id, "waive"))}
                    isReopening={findingActionKeys.includes(buildFindingActionKey(finding.id, "reopen"))}
                    onOpenDecision={() => {
                      if (finding.kind === "DecisionRecord" && finding.artifact_ref) {
                        onNavigate("decisions", finding.artifact_ref);
                      }
                    }}
                    onAdopt={(candidateID) => void handleAdoptFinding(finding.id, candidateID)}
                    onWaive={() => void handleWaiveFinding(finding)}
                    onReopen={() => void handleReopenFinding(finding)}
                  />
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
          <Section title="Recent Activity">
            {recentActivity.length === 0 ? (
              <EmptyState text="No recent problem or decision activity." />
            ) : (
              <div className="space-y-2">
                {recentActivity.map((item) => (
                  <RecentActivityCard
                    key={`${item.kind}-${item.id}`}
                    item={item}
                    onOpen={() => onNavigate(item.page, item.id)}
                  />
                ))}
              </div>
            )}
          </Section>

          <Section title="Module Coverage">
            <CoverageSummary overview={overview} />
          </Section>
        </div>
      </div>
    </div>
  );
}

function DecisionBucket({
  title,
  decisions,
  onOpenDecision,
  onImplementDecision,
  implementingDecisionIDs,
}: {
  title: string;
  decisions: DecisionSummary[];
  onOpenDecision: (decisionID: string) => void;
  onImplementDecision: (decisionID: string) => Promise<void>;
  implementingDecisionIDs: string[];
}) {
  if (decisions.length === 0) {
    return null;
  }

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between gap-3">
        <p className="text-[11px] uppercase tracking-[0.22em] text-text-muted">{title}</p>
        <span className="rounded-full border border-border bg-surface-2 px-2 py-0.5 text-[11px] text-text-secondary">
          {decisions.length}
        </span>
      </div>

      {decisions.map((decision) => (
        <DecisionCard
          key={decision.id}
          decision={decision}
          isImplementing={implementingDecisionIDs.includes(decision.id)}
          onOpenDecision={onOpenDecision}
          onImplementDecision={onImplementDecision}
        />
      ))}
    </div>
  );
}

function GovernanceFindingCard({
  finding,
  candidates,
  isAdopting,
  isWaiving,
  isReopening,
  onOpenDecision,
  onAdopt,
  onWaive,
  onReopen,
}: {
  finding: GovernanceFinding;
  candidates: ProblemCandidate[];
  isAdopting: boolean;
  isWaiving: boolean;
  isReopening: boolean;
  onOpenDecision: () => void;
  onAdopt: (candidateID: string) => void;
  onWaive: () => void;
  onReopen: () => void;
}) {
  const actionState = getGovernanceFindingActionState(finding, candidates);
  const canOpenDecision = finding.kind === "DecisionRecord" && finding.artifact_ref !== "";

  return (
    <div className="rounded-xl border border-border bg-surface-1 px-4 py-3 transition-colors hover:border-border-bright hover:bg-surface-2">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0 flex-1">
          {canOpenDecision ? (
            <button onClick={onOpenDecision} className="min-w-0 text-left">
              <p className="truncate text-sm font-medium text-text-primary">{finding.title}</p>
            </button>
          ) : (
            <p className="text-sm font-medium text-text-primary">{finding.title}</p>
          )}
          <p className="mt-1 text-xs uppercase tracking-[0.2em] text-warning/80">
            {finding.category.replaceAll("_", " ")}
          </p>
        </div>

        <div className="shrink-0 text-right text-[11px] text-text-muted">
          {finding.drift_count > 0 && <p>{finding.drift_count} drift file(s)</p>}
          {finding.days_stale > 0 && <p>{finding.days_stale} day(s) stale</p>}
        </div>
      </div>

      <p className="mt-2 text-sm text-text-secondary">{finding.reason}</p>

      {actionState.showActions && (
        <div className="mt-3 flex flex-wrap items-center gap-2">
          <button
            onClick={() => onAdopt(actionState.adoptCandidateID)}
            disabled={actionState.adoptDisabled || isAdopting}
            title={actionState.adoptReason}
            className="rounded-full bg-accent px-3 py-1.5 text-xs text-surface-0 transition-colors hover:bg-accent-hover disabled:cursor-not-allowed disabled:border disabled:border-border disabled:bg-surface-2 disabled:text-text-muted"
          >
            {isAdopting ? "Adopting..." : "Adopt"}
          </button>
          <button
            onClick={onWaive}
            disabled={isWaiving}
            className="rounded-full border border-border bg-surface-2 px-3 py-1.5 text-xs text-text-primary transition-colors hover:border-border-bright hover:bg-surface-3 disabled:cursor-not-allowed disabled:text-text-muted"
          >
            {isWaiving ? "Waiving..." : "Waive"}
          </button>
          <button
            onClick={onReopen}
            disabled={isReopening}
            className="rounded-full border border-warning/30 bg-warning/10 px-3 py-1.5 text-xs text-warning transition-colors hover:border-warning/50 hover:bg-warning/15 disabled:cursor-not-allowed disabled:border-border disabled:bg-surface-2 disabled:text-text-muted"
          >
            {isReopening ? "Reopening..." : "Reopen"}
          </button>
        </div>
      )}
    </div>
  );
}

function DecisionCard({
  decision,
  isImplementing,
  onOpenDecision,
  onImplementDecision,
}: {
  decision: DecisionSummary;
  isImplementing: boolean;
  onOpenDecision: (decisionID: string) => void;
  onImplementDecision: (decisionID: string) => Promise<void>;
}) {
  const implementAction = getDecisionImplementActionState(decision.status);
  const isImplementDisabled = implementAction.disabled || isImplementing;

  return (
    <div className="rounded-xl border border-border bg-surface-1 px-4 py-3 transition-colors hover:border-border-bright hover:bg-surface-2">
      <div className="flex items-start justify-between gap-3">
        <button
          onClick={() => onOpenDecision(decision.id)}
          className="min-w-0 flex-1 text-left"
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

        <button
          onClick={() => void onImplementDecision(decision.id)}
          disabled={isImplementDisabled}
          title={implementAction.reason}
          className="shrink-0 rounded-full bg-accent px-3 py-1.5 text-xs text-surface-0 transition-colors hover:bg-accent-hover disabled:cursor-not-allowed disabled:border disabled:border-border disabled:bg-surface-2 disabled:text-text-muted"
        >
          {isImplementing ? "Spawning..." : "Implement"}
        </button>
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

function buildFindingActionKey(
  findingID: string,
  action: "adopt" | "waive" | "reopen",
): string {
  return `${findingID}:${action}`;
}

function promptForGovernanceDecisionReason(
  action: "waive" | "reopen",
  finding: GovernanceFinding,
): string {
  if (typeof window === "undefined") {
    return "";
  }

  const promptMessage =
    action === "waive"
      ? `Waive will extend this DecisionRecord by 90 days.\n\nEnter justification for ${finding.title}:`
      : `Reopen will mark this DecisionRecord as refresh due and create a new ProblemCard.\n\nEnter reason for ${finding.title}:`;
  const response = window.prompt(promptMessage, finding.reason);

  return response ? response.trim() : "";
}

function RecentActivityCard({
  item,
  onOpen,
}: {
  item: DashboardActivityItem;
  onOpen: () => void;
}) {
  const badgeClassName =
    item.kind === "DecisionRecord"
      ? "border-success/20 bg-success/10 text-success"
      : "border-warning/20 bg-warning/10 text-warning";
  const badgeLabel = item.kind === "DecisionRecord" ? "Decision" : "Problem";

  return (
    <button
      onClick={onOpen}
      className="w-full rounded-xl border border-border bg-surface-1 px-4 py-3 text-left transition-colors hover:border-border-bright hover:bg-surface-2"
    >
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <span className={`rounded-full border px-2 py-0.5 text-[11px] ${badgeClassName}`}>
              {badgeLabel}
            </span>
            <p className="truncate text-sm font-medium text-text-primary">{item.title}</p>
          </div>
          <p className="mt-2 line-clamp-2 text-sm text-text-secondary">{item.summary}</p>
        </div>

        <div className="shrink-0 text-right">
          <p className="font-mono text-[11px] text-text-muted">{item.id}</p>
          <p className="mt-1 text-[11px] text-text-muted">{item.created_at}</p>
        </div>
      </div>
    </button>
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
