import { useEffect, useState } from "react";

import { EventsOn } from "../../wailsjs/runtime/runtime";
import { DecisionForm } from "../components/DecisionForm";
import {
  adoptProblemCandidate,
  createDecision,
  dismissProblemCandidate,
  getConfig,
  getDecision,
  getGovernanceOverview,
  getPortfolio,
  implementDecision,
  listDecisions,
  listPortfolios,
  refreshGovernance,
  verifyDecision,
  type DecisionDetail,
  type DecisionSummary,
  type DesktopConfig,
  type GovernanceOverview,
  type PortfolioDetail,
  type PortfolioSummary,
} from "../lib/api";
import { reportError } from "../lib/errors";

type NavigateFn = (
  page: "dashboard" | "problems" | "portfolios" | "decisions" | "tasks" | "settings",
  id?: string,
) => void;

export function Decisions({
  selectedId,
  onNavigate,
}: {
  selectedId: string | null;
  onNavigate: NavigateFn;
}) {
  const [decisions, setDecisions] = useState<DecisionSummary[]>([]);
  const [portfolios, setPortfolios] = useState<PortfolioSummary[]>([]);
  const [detail, setDetail] = useState<DecisionDetail | null>(null);
  const [activeId, setActiveId] = useState<string | null>(selectedId);
  const [config, setConfig] = useState<DesktopConfig | null>(null);
  const [governance, setGovernance] = useState<GovernanceOverview | null>(null);
  const [showCreate, setShowCreate] = useState(false);
  const [draftPortfolioID, setDraftPortfolioID] = useState("");
  const [draftPortfolio, setDraftPortfolio] = useState<PortfolioDetail | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const refreshDecisions = async () => {
    try {
      const nextDecisions = await listDecisions();
      setDecisions(nextDecisions);
    } catch (error) {
      reportError(error, "decisions");
    }
  };

  const refreshGovernanceOverview = async () => {
    try {
      const nextOverview = await getGovernanceOverview();
      setGovernance(nextOverview);
    } catch (error) {
      reportError(error, "governance");
    }
  };

  useEffect(() => {
    void refreshDecisions();
    listPortfolios()
      .then((items) => setPortfolios(items.filter((portfolio) => portfolio.has_comparison)))
      .catch((error) => reportError(error, "portfolios"));
    getConfig().then(setConfig).catch((error) => reportError(error, "decision config"));
    void refreshGovernanceOverview();
  }, []);

  useEffect(() => {
    let stopStale: (() => void) | undefined;
    let stopDrift: (() => void) | undefined;

    try {
      stopStale = EventsOn("scan.stale", () => {
        void refreshGovernanceOverview();
      });
      stopDrift = EventsOn("scan.drift", () => {
        void refreshGovernanceOverview();
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

  useEffect(() => {
    setActiveId(selectedId);
  }, [selectedId]);

  useEffect(() => {
    if (!activeId) {
      setDetail(null);
      return;
    }

    getDecision(activeId)
      .then(setDetail)
      .catch((error) => {
        reportError(error, "decision detail");
      });
  }, [activeId]);

  useEffect(() => {
    if (!draftPortfolioID) {
      setDraftPortfolio(null);
      return;
    }

    getPortfolio(draftPortfolioID)
      .then(setDraftPortfolio)
      .catch((error) => {
        reportError(error, "decision portfolio");
      });
  }, [draftPortfolioID]);

  useEffect(() => {
    if (showCreate && !draftPortfolioID && portfolios.length > 0) {
      setDraftPortfolioID(portfolios[0].id);
    }
  }, [draftPortfolioID, portfolios, showCreate]);

  const handleCreate = async (value: Parameters<typeof createDecision>[0]) => {
    setSubmitting(true);

    try {
      const created = await createDecision(value);
      setDetail(created);
      setActiveId(created.id);
      setShowCreate(false);
      await refreshDecisions();
    } catch (error) {
      reportError(error, "create decision");
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div>
      <div className="mb-4">
        <p className="font-mono text-xs uppercase tracking-[1.2px] text-text-muted">EXECUTE</p>
        <p className="text-xs text-text-muted mt-0.5">Implement with contract, verify with evidence</p>
      </div>
      <div className="flex gap-6 h-[calc(100vh-7rem)]">
      <div className="w-80 shrink-0 overflow-y-auto space-y-3">
        <button
          onClick={() => setShowCreate(true)}
          className="w-full rounded-xl border border-dashed border-accent/40 bg-accent/5 px-4 py-3 text-left text-sm text-text-primary transition-colors hover:border-accent/60 hover:bg-accent/10"
        >
          <span className="font-medium">+ Record decision</span>
          <p className="mt-1 text-xs text-text-muted">Turn a compared portfolio into a decision record.</p>
        </button>

        <div className="space-y-1">
          {decisions.map((decision) => (
            <button
              key={decision.id}
              onClick={() => {
                setActiveId(decision.id);
                setShowCreate(false);
              }}
              className={`w-full rounded-lg border px-4 py-3 text-left transition-colors ${
                activeId === decision.id
                  ? "border-accent/30 bg-surface-2"
                  : "border-transparent bg-surface-1 hover:border-border hover:bg-surface-2"
              }`}
            >
              <span className="block truncate text-sm font-medium">{decision.selected_title}</span>
              <div className="mt-1 flex items-center gap-2">
                <span className="font-mono text-xs text-text-muted">{decision.id}</span>
                {decision.valid_until && (
                  <span className="text-xs text-text-muted">until {decision.valid_until}</span>
                )}
              </div>
              {decision.weakest_link && (
                <p className="mt-1 line-clamp-1 text-xs text-warning/70">WLNK: {decision.weakest_link}</p>
              )}
            </button>
          ))}

          {decisions.length === 0 && (
            <p className="py-8 text-center text-sm text-text-muted">No decisions</p>
          )}
        </div>
      </div>

      <div className="flex-1 overflow-y-auto space-y-6 pb-8">
        {showCreate && (
          <div className="space-y-5 rounded-2xl border border-border bg-surface-1 p-5">
            <div className="flex items-start justify-between gap-4">
              <div>
                <p className="text-xs uppercase tracking-[0.22em] text-text-muted">Decide</p>
                <h3 className="mt-1 text-lg font-semibold text-text-primary">Select from a compared portfolio</h3>
              </div>
              <button
                onClick={() => setShowCreate(false)}
                className="rounded-lg border border-border px-3 py-1.5 text-xs text-text-secondary transition-colors hover:bg-surface-2"
              >
                Cancel
              </button>
            </div>

            <label className="block space-y-1.5">
              <span className="text-xs uppercase tracking-[0.2em] text-text-muted">Compared Portfolio</span>
              <select
                value={draftPortfolioID}
                onChange={(event) => setDraftPortfolioID(event.target.value)}
                className={inputClassName}
              >
                {portfolios.length === 0 && <option value="">No compared portfolios available</option>}
                {portfolios.map((portfolio) => (
                  <option key={portfolio.id} value={portfolio.id}>
                    {portfolio.title}
                  </option>
                ))}
              </select>
            </label>

            {draftPortfolio ? (
              <DecisionForm
                portfolio={draftPortfolio}
                onSubmit={handleCreate}
                onCancel={() => setShowCreate(false)}
                submitting={submitting}
              />
            ) : (
              <div className="rounded-xl border border-dashed border-border bg-surface-2/60 p-5 text-sm text-text-secondary">
                Compare a portfolio first. Decisions require portfolio-backed selection data.
              </div>
            )}
          </div>
        )}

        {detail ? (
          <DecisionDetailPanel
            detail={detail}
            config={config}
            governance={governance}
            onNavigate={onNavigate}
            onGovernanceChange={refreshGovernanceOverview}
          />
        ) : activeId ? (
          <p className="py-8 text-center text-sm text-text-muted">Loading...</p>
        ) : (
          <div className="rounded-2xl border border-border bg-surface-1 p-6 text-sm text-text-muted">
            Select a decision to inspect it, or record a new one from a compared portfolio.
          </div>
        )}
      </div>
    </div>
    </div>
  );
}

function DecisionDetailPanel({
  detail,
  config,
  governance,
  onNavigate,
  onGovernanceChange,
}: {
  detail: DecisionDetail;
  config: DesktopConfig | null;
  governance: GovernanceOverview | null;
  onNavigate: NavigateFn;
  onGovernanceChange: () => Promise<void>;
}) {
  const [implementing, setImplementing] = useState(false);
  const [verifying, setVerifying] = useState(false);
  const [refreshingScan, setRefreshingScan] = useState(false);

  const relevantFindings = (governance?.findings ?? []).filter(
    (finding) => finding.artifact_ref === detail.id,
  );
  const relevantCandidates = (governance?.problem_candidates ?? []).filter(
    (candidate) => candidate.source_artifact_ref === detail.id,
  );

  const handleImplement = async () => {
    setImplementing(true);
    try {
      const task = await implementDecision(
        detail.id,
        config?.default_agent ?? "claude",
        config?.default_worktree ?? true,
        "",
      );
      onNavigate("tasks", task.id);
    } catch (error) {
      reportError(error, "implement decision");
    } finally {
      setImplementing(false);
    }
  };

  const handleVerify = async () => {
    setVerifying(true);
    try {
      const task = await verifyDecision(
        detail.id,
        config?.verify_agent ?? config?.default_agent ?? "claude",
      );
      onNavigate("tasks", task.id);
    } catch (error) {
      reportError(error, "verify decision");
    } finally {
      setVerifying(false);
    }
  };

  const handleRefreshGovernance = async () => {
    setRefreshingScan(true);
    try {
      await refreshGovernance();
      await onGovernanceChange();
    } catch (error) {
      reportError(error, "refresh governance");
    } finally {
      setRefreshingScan(false);
    }
  };

  const handleAdoptCandidate = async (candidateID: string) => {
    try {
      const problem = await adoptProblemCandidate(candidateID);
      await onGovernanceChange();
      onNavigate("problems", problem.id);
    } catch (error) {
      reportError(error, "adopt problem candidate");
    }
  };

  const handleDismissCandidate = async (candidateID: string) => {
    try {
      await dismissProblemCandidate(candidateID);
      await onGovernanceChange();
    } catch (error) {
      reportError(error, "dismiss problem candidate");
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex items-start justify-between gap-4">
        <div>
          <h2 className="text-xl font-semibold">{detail.selected_title}</h2>
          <p className="mt-1 font-mono text-xs text-text-muted">{detail.id}</p>
          {detail.valid_until && (
            <p className="mt-1 text-xs text-text-secondary">Valid until {detail.valid_until}</p>
          )}
          <p className="mt-2 text-xs text-text-muted">
            Implement uses <span className="font-mono text-text-secondary">{config?.default_agent ?? "claude"}</span>
            {config?.default_worktree === false ? " in the active project folder." : " in a fresh worktree by default."}
          </p>
          <p className="mt-1 text-xs text-text-muted">
            Verify uses <span className="font-mono text-text-secondary">{config?.verify_agent ?? config?.default_agent ?? "claude"}</span>.
          </p>
        </div>
        <div className="flex shrink-0 items-center gap-2">
          <button
            onClick={handleRefreshGovernance}
            disabled={refreshingScan}
            className="rounded-lg border border-border bg-surface-2 px-3 py-1.5 text-xs text-text-secondary transition-colors hover:bg-surface-3 disabled:opacity-50"
          >
            {refreshingScan ? "Scanning..." : "Refresh Scan"}
          </button>
          <button
            onClick={handleVerify}
            disabled={verifying}
            className="rounded-lg border border-border bg-surface-2 px-3 py-1.5 text-xs text-text-secondary transition-colors hover:bg-surface-3 disabled:opacity-50"
          >
            {verifying ? "Verifying..." : "Verify Claims"}
          </button>
          <button
            onClick={handleImplement}
            disabled={implementing}
            className="rounded-full bg-accent px-3 py-1.5 text-xs text-surface-0 transition-colors hover:bg-accent-hover disabled:opacity-50"
          >
            {implementing ? "Spawning..." : "Implement"}
          </button>
        </div>
      </div>

      <Field label="Why Selected" value={detail.why_selected} />
      {detail.selection_policy && <Field label="Selection Policy" value={detail.selection_policy} />}
      {detail.weakest_link && <Field label="Weakest Link" value={detail.weakest_link} variant="warning" />}
      {detail.counterargument && <Field label="Counterargument" value={detail.counterargument} />}

      {detail.invariants.length > 0 && (
        <ListField label="Invariants (must hold)" items={detail.invariants} />
      )}
      {detail.admissibility.length > 0 && (
        <ListField label="Not Acceptable" items={detail.admissibility} variant="danger" />
      )}

      {detail.claims.length > 0 && (
        <div>
          <h4 className="mb-2 text-xs uppercase tracking-wider text-text-muted">Claims & Predictions</h4>
          <div className="overflow-hidden rounded-lg border border-border">
            <table className="w-full text-sm">
              <thead>
                <tr className="bg-surface-2">
                  <th className="px-4 py-2 text-left text-xs font-medium text-text-muted">Claim</th>
                  <th className="px-4 py-2 text-left text-xs font-medium text-text-muted">Observable</th>
                  <th className="px-4 py-2 text-left text-xs font-medium text-text-muted">Threshold</th>
                  <th className="px-4 py-2 text-left text-xs font-medium text-text-muted">Verify After</th>
                  <th className="px-4 py-2 text-left text-xs font-medium text-text-muted">Status</th>
                </tr>
              </thead>
              <tbody>
                {detail.claims.map((claim) => (
                  <tr key={claim.id} className="border-t border-border">
                    <td className="px-4 py-2">{claim.claim}</td>
                    <td className="px-4 py-2 text-text-secondary">{claim.observable}</td>
                    <td className="px-4 py-2 font-mono text-xs">{claim.threshold}</td>
                    <td className="px-4 py-2 font-mono text-xs text-text-muted">{claim.verify_after || "now"}</td>
                    <td className="px-4 py-2"><ClaimStatusBadge status={claim.status} /></td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* Evidence F/G/R Decomposition */}
      {detail.evidence && detail.evidence.items?.length > 0 && (
        <EvidenceSection evidence={detail.evidence} />
      )}

      {/* Coverage gaps */}
      {detail.evidence && detail.evidence.coverage_gaps?.length > 0 && (
        <div>
          <h4 className="mb-2 text-xs uppercase tracking-wider text-text-muted">
            Evidence Gaps ({detail.evidence.coverage_gaps.length}/{detail.evidence.total_claims} claims uncovered)
          </h4>
          <div className="space-y-1">
            {detail.evidence.coverage_gaps.map((gap: string, i: number) => (
              <div key={i} className="rounded-lg border border-danger/20 bg-danger/5 px-4 py-2 text-sm text-danger/80">
                {gap}
              </div>
            ))}
          </div>
        </div>
      )}

      {detail.affected_files.length > 0 && (
        <div>
          <h4 className="mb-2 text-xs uppercase tracking-wider text-text-muted">Affected Files</h4>
          <div className="rounded-lg border border-border bg-surface-1 px-4 py-3">
            <div className="space-y-1">
              {detail.affected_files.map((filePath) => (
                <p key={filePath} className="font-mono text-xs text-text-secondary">{filePath}</p>
              ))}
            </div>
          </div>
        </div>
      )}

      {detail.coverage_modules.length > 0 && (
        <div>
          <h4 className="mb-2 text-xs uppercase tracking-wider text-text-muted">Impacted Modules</h4>
          <div className="space-y-2">
            {detail.coverage_modules.map((module) => (
              <div key={`${module.id}-${module.path}`} className="rounded-lg border border-border bg-surface-1 px-4 py-3">
                <div className="flex items-start justify-between gap-3">
                  <div>
                    <p className="text-sm font-medium text-text-primary">{module.path || "(root)"}</p>
                    <p className="mt-1 text-xs text-text-muted">
                      {module.lang} · {module.decision_count} decision(s)
                    </p>
                  </div>

                  <span className={`rounded-full border px-2 py-0.5 text-[11px] ${coverageStatusClassName(module.status)}`}>
                    {module.status}
                  </span>
                </div>

                {module.files.length > 0 && (
                  <div className="mt-3 space-y-1">
                    {module.files.map((filePath) => (
                      <p key={filePath} className="font-mono text-[11px] text-text-muted">{filePath}</p>
                    ))}
                  </div>
                )}
              </div>
            ))}
          </div>
        </div>
      )}

      {detail.coverage_warnings.length > 0 && (
        <ListField label="Coverage Warnings" items={detail.coverage_warnings} variant="warning" />
      )}

      {detail.first_module_coverage && (
        <Field
          label="Governance Signal"
          value="This decision established the first explicit governance coverage for at least one affected module."
          variant="warning"
        />
      )}

      {relevantFindings.length > 0 && (
        <div>
          <h4 className="mb-2 text-xs uppercase tracking-wider text-text-muted">Live Findings</h4>
          <div className="space-y-2">
            {relevantFindings.map((finding) => (
              <div key={finding.id} className="rounded-lg border border-warning/20 bg-warning/5 px-4 py-3">
                <div className="flex items-start justify-between gap-3">
                  <span className="text-sm font-medium text-text-primary">{finding.category.replaceAll("_", " ")}</span>
                  <span className="text-[11px] text-text-muted">{finding.valid_until || "active"}</span>
                </div>
                <p className="mt-2 text-sm text-text-secondary">{finding.reason}</p>
              </div>
            ))}
          </div>
        </div>
      )}

      {relevantCandidates.length > 0 && (
        <div>
          <h4 className="mb-2 text-xs uppercase tracking-wider text-text-muted">Follow-up Candidates</h4>
          <div className="space-y-3">
            {relevantCandidates.map((candidate) => (
              <div key={candidate.id} className="rounded-lg border border-warning/20 bg-surface-1 px-4 py-4">
                <p className="text-sm font-medium text-text-primary">{candidate.title}</p>
                <p className="mt-2 text-sm text-text-secondary">{candidate.signal}</p>
                <p className="mt-2 text-xs text-text-muted">Acceptance: {candidate.acceptance}</p>
                <div className="mt-4 flex items-center gap-2">
                  <button
                    onClick={() => void handleDismissCandidate(candidate.id)}
                    className="rounded-lg border border-border bg-surface-2 px-3 py-1.5 text-xs text-text-secondary transition-colors hover:bg-surface-3"
                  >
                    Dismiss
                  </button>
                  <button
                    onClick={() => void handleAdoptCandidate(candidate.id)}
                    className="rounded-full bg-accent px-3 py-1.5 text-xs text-surface-0 transition-colors hover:bg-accent-hover"
                  >
                    Adopt problem
                  </button>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {detail.why_not_others.length > 0 && (
        <div>
          <h4 className="mb-2 text-xs uppercase tracking-wider text-text-muted">Rejected Alternatives</h4>
          <div className="space-y-2">
            {detail.why_not_others.map((rejection, index) => (
              <div key={`${rejection.variant}-${index}`} className="rounded-lg border border-border bg-surface-1 px-4 py-3">
                <span className="text-sm font-medium text-text-secondary">{rejection.variant}</span>
                <p className="mt-1 text-xs text-text-muted">{rejection.reason}</p>
              </div>
            ))}
          </div>
        </div>
      )}

      {detail.pre_conditions.length > 0 && <ListField label="Pre-conditions" items={detail.pre_conditions} />}
      {detail.post_conditions.length > 0 && <ListField label="Post-conditions" items={detail.post_conditions} />}
      {detail.evidence_requirements.length > 0 && (
        <ListField label="Evidence Requirements" items={detail.evidence_requirements} />
      )}

      {detail.rollback_triggers.length > 0 && (
        <div className="rounded-lg border border-danger/20 bg-danger/5 p-4">
          <h4 className="mb-2 text-xs uppercase tracking-wider text-danger">Rollback Plan</h4>
          <div className="space-y-2">
            <div>
              <span className="text-xs text-text-muted">Triggers:</span>
              <ul className="mt-1 space-y-1">
                {detail.rollback_triggers.map((trigger) => (
                  <li key={trigger} className="text-sm text-text-secondary">{trigger}</li>
                ))}
              </ul>
            </div>

            {detail.rollback_steps.length > 0 && (
              <div>
                <span className="text-xs text-text-muted">Steps:</span>
                <ol className="mt-1 list-inside list-decimal space-y-1">
                  {detail.rollback_steps.map((step) => (
                    <li key={step} className="text-sm text-text-secondary">{step}</li>
                  ))}
                </ol>
              </div>
            )}

            {detail.rollback_blast_radius && (
              <p className="mt-2 text-xs text-text-muted">Blast radius: {detail.rollback_blast_radius}</p>
            )}
          </div>
        </div>
      )}

      {detail.refresh_triggers.length > 0 && <ListField label="Refresh Triggers" items={detail.refresh_triggers} />}
    </div>
  );
}

function Field({
  label,
  value,
  variant,
}: {
  label: string;
  value: string;
  variant?: "warning" | "danger";
}) {
  const borderColor =
    variant === "warning"
      ? "border-warning/20"
      : variant === "danger"
        ? "border-danger/20"
        : "border-border";

  return (
    <div>
      <h4 className="mb-1 text-xs uppercase tracking-wider text-text-muted">{label}</h4>
      <p className={`rounded-lg border bg-surface-1 px-4 py-3 text-sm text-text-primary ${borderColor}`}>
        {value}
      </p>
    </div>
  );
}

function ListField({
  label,
  items,
  variant,
}: {
  label: string;
  items: string[];
  variant?: "warning" | "danger";
}) {
  const borderColor =
    variant === "warning"
      ? "border-warning/20"
      : variant === "danger"
        ? "border-danger/20"
        : "border-border";

  return (
    <div>
      <h4 className="mb-2 text-xs uppercase tracking-wider text-text-muted">{label}</h4>
      <ul className="space-y-2">
        {items.map((item) => (
          <li
            key={`${label}-${item}`}
            className={`rounded-lg border bg-surface-1 px-4 py-2 text-sm text-text-primary ${borderColor}`}
          >
            {item}
          </li>
        ))}
      </ul>
    </div>
  );
}

function ClaimStatusBadge({ status }: { status: string }) {
  const colors: Record<string, string> = {
    unverified: "border-border bg-surface-2 text-text-muted",
    supported: "border-success/20 bg-success/10 text-success",
    weakened: "border-warning/20 bg-warning/10 text-warning",
    refuted: "border-danger/20 bg-danger/10 text-danger",
    inconclusive: "border-border bg-surface-2 text-text-muted",
  };

  return (
    <span className={`rounded-full border px-2 py-0.5 text-xs ${colors[status] ?? colors.unverified}`}>
      {status}
    </span>
  );
}

function coverageStatusClassName(status: string): string {
  if (status === "covered") {
    return "border-success/20 bg-success/10 text-success";
  }
  if (status === "partial") {
    return "border-warning/20 bg-warning/10 text-warning";
  }
  return "border-danger/20 bg-danger/10 text-danger";
}

interface EvidenceItem {
  id: string;
  type: string;
  content: string;
  verdict: string;
  formality_level: number;
  congruence_level: number;
  claim_refs: string[];
  valid_until: string;
  is_expired: boolean;
}

interface EvidenceSummary {
  items: EvidenceItem[];
  total_claims: number;
  covered_claims: number;
  coverage_gaps: string[];
}

function EvidenceSection({ evidence }: { evidence: EvidenceSummary }) {
  const formalityLabels: Record<number, { label: string; color: string }> = {
    0: { label: "F0 informal", color: "text-text-muted" },
    1: { label: "F1 test", color: "text-blue-400" },
    2: { label: "F2 formal", color: "text-success" },
    3: { label: "F3 proof", color: "text-yellow-400" },
  };

  const verdictColors: Record<string, string> = {
    supports: "border-success/20 bg-success/10 text-success",
    weakens: "border-warning/20 bg-warning/10 text-warning",
    refutes: "border-danger/20 bg-danger/10 text-danger",
  };

  return (
    <div>
      <div className="mb-2 flex items-center justify-between">
        <h4 className="text-xs uppercase tracking-wider text-text-muted">Evidence</h4>
        <div className="flex items-center gap-3 text-xs text-text-muted">
          <span>Coverage: {evidence.covered_claims}/{evidence.total_claims} claims</span>
          <span>{evidence.items.length} item{evidence.items.length !== 1 ? "s" : ""}</span>
        </div>
      </div>
      <div className="space-y-2">
        {evidence.items.map((item) => {
          const f = formalityLabels[item.formality_level] ?? formalityLabels[0];
          const verdictCls = verdictColors[item.verdict] ?? "border-border bg-surface-2 text-text-muted";
          const freshnessColor = item.is_expired
            ? "text-danger"
            : item.valid_until
              ? "text-success"
              : "text-text-muted";

          return (
            <div key={item.id} className="rounded-lg border border-border bg-surface-1 px-4 py-3">
              <div className="flex items-center justify-between gap-3">
                <div className="flex items-center gap-2">
                  <span className={`font-mono text-xs ${f.color}`}>{f.label}</span>
                  <span className={`rounded-full border px-2 py-0.5 text-xs ${verdictCls}`}>
                    {item.verdict || "pending"}
                  </span>
                  <span className="text-xs text-text-muted">{item.type}</span>
                </div>
                <div className="flex items-center gap-2 text-xs">
                  <span className="text-text-muted">CL{item.congruence_level}</span>
                  {item.valid_until && (
                    <span className={freshnessColor}>
                      {item.is_expired ? "expired" : `until ${item.valid_until}`}
                    </span>
                  )}
                </div>
              </div>
              <p className="mt-2 text-sm text-text-secondary">{item.content}</p>
              {item.claim_refs?.length > 0 && (
                <p className="mt-1 text-xs text-text-muted">
                  Covers: {item.claim_refs.join(", ")}
                </p>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}

const inputClassName =
  "w-full rounded-xl border border-border bg-surface-2 px-3 py-2 text-sm text-text-primary outline-none transition-colors focus:border-accent/60";
