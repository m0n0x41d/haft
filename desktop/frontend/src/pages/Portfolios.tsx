import { useEffect, useState } from "react";
import {
  listProblems,
  getPortfolio,
  type ProblemSummary,
  type PortfolioDetail,
} from "../lib/api";

type NavigateFn = (page: "dashboard" | "problems" | "portfolios" | "decisions", id?: string) => void;

// We need to get portfolios linked to problems. For now, use a binding that lists them.
// The portfolio IDs come from problem backlinks or direct listing.
let ListPortfolios: (() => Promise<{ id: string; title: string; status: string; problem_ref: string }[]>) | null = null;
try {
  const bindingPath = "../../wailsjs/go/main/App";
  const mod = await import(/* @vite-ignore */ bindingPath);
  ListPortfolios = mod.ListPortfolios;
} catch {
  // not available
}

interface PortfolioSummary {
  id: string;
  title: string;
  status: string;
  problem_ref: string;
}

export function Portfolios({
  selectedId,
  onNavigate,
}: {
  selectedId: string | null;
  onNavigate: NavigateFn;
}) {
  const [portfolios, setPortfolios] = useState<PortfolioSummary[]>([]);
  const [detail, setDetail] = useState<PortfolioDetail | null>(null);
  const [activeId, setActiveId] = useState<string | null>(selectedId);

  useEffect(() => {
    // Try to load portfolios from problems' linked artifacts
    if (ListPortfolios) {
      ListPortfolios().then(setPortfolios).catch(console.error);
    } else {
      // Mock
      listProblems().then((problems: ProblemSummary[]) => {
        setPortfolios(
          problems.map((p) => ({
            id: `sol-mock-${p.id}`,
            title: `Solutions for: ${p.title}`,
            status: "active",
            problem_ref: p.id,
          }))
        );
      });
    }
  }, []);

  useEffect(() => {
    if (!activeId) {
      setDetail(null);
      return;
    }
    getPortfolio(activeId).then(setDetail).catch(console.error);
  }, [activeId]);

  return (
    <div className="flex gap-6 h-[calc(100vh-7rem)]">
      {/* Portfolio list */}
      <div className="w-80 shrink-0 overflow-y-auto space-y-1">
        {portfolios.map((p) => (
          <button
            key={p.id}
            onClick={() => setActiveId(p.id)}
            className={`w-full text-left px-4 py-3 rounded-lg transition-colors border ${
              activeId === p.id
                ? "bg-surface-2 border-accent/30"
                : "bg-surface-1 border-transparent hover:bg-surface-2 hover:border-border"
            }`}
          >
            <span className="text-sm font-medium block truncate">{p.title}</span>
            <span className="text-xs text-text-muted font-mono mt-1 block">{p.id}</span>
          </button>
        ))}
        {portfolios.length === 0 && (
          <p className="text-sm text-text-muted text-center py-8">No portfolios</p>
        )}
      </div>

      {/* Detail */}
      <div className="flex-1 overflow-y-auto">
        {detail ? (
          <PortfolioDetailView detail={detail} onNavigate={onNavigate} />
        ) : activeId ? (
          <p className="text-sm text-text-muted py-8 text-center">Loading...</p>
        ) : (
          <p className="text-sm text-text-muted py-8 text-center">
            Select a portfolio to view comparison
          </p>
        )}
      </div>
    </div>
  );
}

function PortfolioDetailView({
  detail,
  onNavigate,
}: {
  detail: PortfolioDetail;
  onNavigate: NavigateFn;
}) {
  return (
    <div className="space-y-8">
      {/* Header */}
      <div>
        <h2 className="text-xl font-semibold">{detail.title}</h2>
        <p className="text-xs text-text-muted font-mono mt-1">{detail.id}</p>
        {detail.problem_ref && (
          <button
            onClick={() => onNavigate("problems", detail.problem_ref)}
            className="text-xs text-accent hover:text-accent-hover mt-1"
          >
            Problem: {detail.problem_ref}
          </button>
        )}
      </div>

      {/* Variants */}
      {detail.variants.length > 0 && (
        <div>
          <h3 className="text-sm font-medium text-text-secondary uppercase tracking-wider mb-3">
            Variants ({detail.variants.length})
          </h3>
          <div className="space-y-3">
            {detail.variants.map((v) => {
              const isDominated = detail.comparison?.dominated_notes?.some(
                (d) => d.variant === v.id || d.variant === v.title
              );
              const isOnFront = detail.comparison?.non_dominated_set?.some(
                (nds) => nds === v.id || nds === v.title
              );
              const isSelected =
                detail.comparison?.selected_ref === v.id ||
                detail.comparison?.selected_ref === v.title;

              return (
                <div
                  key={v.id}
                  className={`rounded-lg px-4 py-3 border transition-colors ${
                    isSelected
                      ? "bg-accent/5 border-accent/30"
                      : isDominated
                        ? "bg-surface-1/50 border-border/50 opacity-60"
                        : isOnFront
                          ? "bg-surface-1 border-success/20"
                          : "bg-surface-1 border-border"
                  }`}
                >
                  <div className="flex items-center gap-2 mb-1">
                    <span className="text-sm font-medium">{v.title}</span>
                    {isSelected && (
                      <span className="text-xs px-1.5 py-0.5 rounded bg-accent/10 text-accent border border-accent/20">
                        selected
                      </span>
                    )}
                    {isOnFront && !isSelected && (
                      <span className="text-xs px-1.5 py-0.5 rounded bg-success/10 text-success border border-success/20">
                        Pareto front
                      </span>
                    )}
                    {isDominated && (
                      <span className="text-xs px-1.5 py-0.5 rounded bg-surface-2 text-text-muted border border-border">
                        dominated
                      </span>
                    )}
                    {v.stepping_stone && (
                      <span className="text-xs px-1.5 py-0.5 rounded bg-purple-500/10 text-purple-400 border border-purple-500/20">
                        stepping stone
                      </span>
                    )}
                  </div>
                  {v.description && (
                    <p className="text-xs text-text-secondary mb-2 line-clamp-2">
                      {v.description}
                    </p>
                  )}
                  <div className="flex items-center gap-4 text-xs">
                    {v.weakest_link && (
                      <span className="text-warning/70">WLNK: {v.weakest_link}</span>
                    )}
                    {v.novelty_marker && (
                      <span className="text-text-muted">Novelty: {v.novelty_marker}</span>
                    )}
                  </div>
                </div>
              );
            })}
          </div>
        </div>
      )}

      {/* Comparison Table — THE DIFFERENTIATOR */}
      {detail.comparison && <ComparisonTable comparison={detail.comparison} />}

      {/* Recommendation */}
      {detail.comparison?.recommendation && (
        <div className="bg-accent/5 rounded-lg p-4 border border-accent/20">
          <h4 className="text-xs text-accent uppercase tracking-wider mb-1">
            Recommendation
          </h4>
          <p className="text-sm text-text-primary">{detail.comparison.recommendation}</p>
        </div>
      )}
    </div>
  );
}

function ComparisonTable({
  comparison,
}: {
  comparison: NonNullable<PortfolioDetail["comparison"]>;
}) {
  if (!comparison.scores || !comparison.dimensions) return null;

  const variants = Object.keys(comparison.scores);
  const dimensions = comparison.dimensions;
  const nonDominated = new Set(comparison.non_dominated_set || []);
  const dominatedSet = new Set(
    (comparison.dominated_notes || []).map((d) => d.variant)
  );

  return (
    <div>
      <h3 className="text-sm font-medium text-text-secondary uppercase tracking-wider mb-3">
        Comparison Table
      </h3>

      {/* Parity banner */}
      {comparison.policy_applied && (
        <div className="mb-3 px-3 py-2 bg-surface-1 rounded-lg border border-border text-xs text-text-muted">
          Selection policy: {comparison.policy_applied}
        </div>
      )}

      <div className="border border-border rounded-lg overflow-hidden overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="bg-surface-2">
              <th className="text-left px-4 py-2.5 text-xs text-text-muted font-medium border-r border-border min-w-[140px]">
                Dimension
              </th>
              {variants.map((v) => {
                const isOnFront = nonDominated.has(v);
                const isDominated = dominatedSet.has(v);
                const isSelected = comparison.selected_ref === v;
                return (
                  <th
                    key={v}
                    className={`text-left px-4 py-2.5 text-xs font-medium min-w-[160px] ${
                      isSelected
                        ? "text-accent bg-accent/5"
                        : isDominated
                          ? "text-text-muted/50"
                          : isOnFront
                            ? "text-success"
                            : "text-text-muted"
                    }`}
                  >
                    <div className="flex items-center gap-1.5">
                      <span className="truncate">{v}</span>
                      {isSelected && <span className="text-accent">*</span>}
                      {isDominated && <span className="text-text-muted">(dom)</span>}
                    </div>
                  </th>
                );
              })}
            </tr>
          </thead>
          <tbody>
            {dimensions.map((dim, i) => (
              <tr
                key={dim}
                className={`border-t border-border ${i % 2 === 0 ? "" : "bg-surface-1/30"}`}
              >
                <td className="px-4 py-2 text-xs text-text-secondary border-r border-border font-medium">
                  {dim}
                </td>
                {variants.map((v) => {
                  const score = comparison.scores[v]?.[dim] || "—";
                  const isDominated = dominatedSet.has(v);
                  return (
                    <td
                      key={`${v}-${dim}`}
                      className={`px-4 py-2 text-xs ${
                        isDominated ? "text-text-muted/40" : "text-text-primary"
                      }`}
                    >
                      {score}
                    </td>
                  );
                })}
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* Trade-off notes */}
      {comparison.pareto_tradeoffs && comparison.pareto_tradeoffs.length > 0 && (
        <div className="mt-4 space-y-2">
          <h4 className="text-xs text-text-muted uppercase tracking-wider">
            Pareto Trade-offs
          </h4>
          {comparison.pareto_tradeoffs.map((t, i) => (
            <div
              key={i}
              className="bg-surface-1 rounded-lg px-4 py-2 border border-border"
            >
              <span className="text-xs font-medium text-text-secondary">
                {t.variant}:
              </span>{" "}
              <span className="text-xs text-text-muted">{t.summary}</span>
            </div>
          ))}
        </div>
      )}

      {/* Dominated explanations */}
      {comparison.dominated_notes && comparison.dominated_notes.length > 0 && (
        <div className="mt-4 space-y-2">
          <h4 className="text-xs text-text-muted uppercase tracking-wider">
            Eliminated
          </h4>
          {comparison.dominated_notes.map((d, i) => (
            <div
              key={i}
              className="bg-surface-1/50 rounded-lg px-4 py-2 border border-border/50"
            >
              <span className="text-xs font-medium text-text-muted">
                {d.variant}
              </span>
              {d.dominated_by && d.dominated_by.length > 0 && (
                <span className="text-xs text-text-muted">
                  {" "}
                  (dominated by {d.dominated_by.join(", ")})
                </span>
              )}
              <p className="text-xs text-text-muted/70 mt-0.5">{d.summary}</p>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
