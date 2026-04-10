import { useEffect, useMemo, useState } from "react";

import { VariantForm } from "../components/VariantForm";
import {
  comparePortfolio,
  createPortfolio,
  getConfig,
  getPortfolio,
  getProblem,
  listPortfolios,
  listProblems,
  spawnTask,
  type DesktopConfig,
  type PortfolioCompareInput,
  type PortfolioCreateInput,
  type PortfolioDetail,
  type PortfolioSummary,
  type ProblemDetail,
  type ProblemSummary,
} from "../lib/api";
import { reportError } from "../lib/errors";

type NavigateFn = (
  page: "dashboard" | "problems" | "portfolios" | "decisions" | "tasks",
  id?: string,
) => void;

interface VariantReasoningDraft {
  variant: string;
  summary: string;
  dominated_by: string;
}

export function Portfolios({
  selectedId,
  onNavigate,
}: {
  selectedId: string | null;
  onNavigate: NavigateFn;
}) {
  const [portfolios, setPortfolios] = useState<PortfolioSummary[]>([]);
  const [problems, setProblems] = useState<ProblemSummary[]>([]);
  const [detail, setDetail] = useState<PortfolioDetail | null>(null);
  const [problemDetail, setProblemDetail] = useState<ProblemDetail | null>(null);
  const [activeId, setActiveId] = useState<string | null>(selectedId);
  const [showCreate, setShowCreate] = useState(false);
  const [showCompare, setShowCompare] = useState(false);
  const [config, setConfig] = useState<DesktopConfig | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [createValue, setCreateValue] = useState<PortfolioCreateInput>(emptyPortfolioInput());
  const [compareValue, setCompareValue] = useState<PortfolioCompareInput>(emptyComparisonInput());
  const [variantReasoning, setVariantReasoning] = useState<VariantReasoningDraft[]>([]);

  const refreshPortfolios = async () => {
    try {
      const nextPortfolios = await listPortfolios();
      setPortfolios(nextPortfolios);
    } catch (error) {
      reportError(error, "portfolios");
    }
  };

  useEffect(() => {
    void refreshPortfolios();
    listProblems().then(setProblems).catch((error) => reportError(error, "problems"));
    getConfig().then(setConfig).catch((error) => reportError(error, "portfolio config"));
  }, []);

  useEffect(() => {
    setActiveId(selectedId);
  }, [selectedId]);

  useEffect(() => {
    if (!activeId) {
      setDetail(null);
      return;
    }

    getPortfolio(activeId)
      .then((portfolio) => {
        setDetail(portfolio);
      })
      .catch((error) => {
        reportError(error, "portfolio detail");
      });
  }, [activeId]);

  useEffect(() => {
    if (!detail?.problem_ref) {
      setProblemDetail(null);
      return;
    }

    getProblem(detail.problem_ref)
      .then(setProblemDetail)
      .catch((error) => {
        reportError(error, "portfolio problem detail");
      });
  }, [detail?.problem_ref]);

  useEffect(() => {
    if (!detail) {
      setCompareValue(emptyComparisonInput());
      setVariantReasoning([]);
      return;
    }

    const variants = detail.variants.map((variant) => ({
      variant: variant.id,
      summary:
        detail.comparison?.pareto_tradeoffs.find((note) => note.variant === variant.id || note.variant === variant.title)?.summary ??
        detail.comparison?.dominated_notes.find((note) => note.variant === variant.id || note.variant === variant.title)?.summary ??
        "",
      dominated_by:
        detail.comparison?.dominated_notes
          .find((note) => note.variant === variant.id || note.variant === variant.title)
          ?.dominated_by.join(", ") ?? "",
    }));

    setVariantReasoning(variants);
    setCompareValue({
      portfolio_ref: detail.id,
      dimensions: problemDetail?.latest_characterization?.dimensions.map((dimension) => dimension.name) ?? detail.comparison?.dimensions ?? [],
      scores: buildScoreDraft(detail, problemDetail),
      non_dominated_set: [],
      incomparable: [],
      dominated_notes: [],
      pareto_tradeoffs: [],
      policy_applied: detail.comparison?.policy_applied ?? "",
      selected_ref: detail.comparison?.selected_ref ?? detail.variants[0]?.id ?? "",
      recommendation: detail.comparison?.recommendation ?? "",
      parity_plan: problemDetail?.latest_characterization?.parity_plan ?? null,
    });
  }, [detail, problemDetail]);

  const selectedProblem = useMemo(
    () => problems.find((problem) => problem.id === createValue.problem_ref) ?? null,
    [createValue.problem_ref, problems],
  );

  const dimensions = problemDetail?.latest_characterization?.dimensions ?? [];

  const handleCreate = async () => {
    setSubmitting(true);

    try {
      const created = await createPortfolio(createValue);
      setDetail(created);
      setActiveId(created.id);
      setShowCreate(false);
      await refreshPortfolios();
    } catch (error) {
      reportError(error, "create portfolio");
    } finally {
      setSubmitting(false);
    }
  };

  const handleCompare = async () => {
    if (!detail) {
      return;
    }

    setSubmitting(true);

    try {
      const updated = await comparePortfolio({
        ...compareValue,
        portfolio_ref: detail.id,
        dimensions: dimensions.map((dimension) => dimension.name),
        dominated_notes: variantReasoning.map((reasoning) => ({
          variant: reasoning.variant,
          dominated_by: splitCommaList(reasoning.dominated_by),
          summary: reasoning.summary.trim() || "Reasoning not yet captured.",
        })),
        pareto_tradeoffs: variantReasoning.map((reasoning) => ({
          variant: reasoning.variant,
          summary: reasoning.summary.trim() || "Reasoning not yet captured.",
        })),
      });
      setDetail(updated);
      setShowCompare(false);
      await refreshPortfolios();
    } catch (error) {
      reportError(error, "compare portfolio");
    } finally {
      setSubmitting(false);
    }
  };

  const handleAssist = async () => {
    if (!selectedProblem) {
      reportError("Select a problem first.", "variant assist");
      return;
    }

    setSubmitting(true);

    try {
      const sourceProblem = await getProblem(selectedProblem.id);
      const prompt = buildExplorationPrompt(sourceProblem);
      const task = await spawnTask(
        config?.default_agent ?? "claude",
        prompt,
        config?.default_worktree ?? true,
        `explore-${selectedProblem.id}`,
      );
      onNavigate("tasks", task.id);
    } catch (error) {
      reportError(error, "variant assist");
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div>
      <div className="mb-4">
        <p className="font-mono text-xs uppercase tracking-[1.2px] text-text-muted">EXPLORE</p>
        <p className="text-xs text-text-muted mt-0.5">Generate genuinely distinct options</p>
      </div>
      <div className="flex gap-6 h-[calc(100vh-7rem)]">
      <div className="w-80 shrink-0 overflow-y-auto space-y-3">
        <button
          onClick={() => {
            setShowCreate(true);
            setShowCompare(false);
          }}
          className="w-full rounded-xl border border-dashed border-accent/40 bg-accent/5 px-4 py-3 text-left text-sm text-text-primary transition-colors hover:border-accent/60 hover:bg-accent/10"
        >
          <span className="font-medium">+ Explore portfolio</span>
          <p className="mt-1 text-xs text-text-muted">Turn a framed problem into distinct variants.</p>
        </button>

        <div className="space-y-1">
          {portfolios.map((portfolio) => (
            <button
              key={portfolio.id}
              onClick={() => {
                setActiveId(portfolio.id);
                setShowCreate(false);
                setShowCompare(false);
              }}
              className={`w-full rounded-lg border px-4 py-3 text-left transition-colors ${
                activeId === portfolio.id
                  ? "border-accent/30 bg-surface-2"
                  : "border-transparent bg-surface-1 hover:border-border hover:bg-surface-2"
              }`}
            >
              <div className="flex items-center justify-between gap-3">
                <span className="truncate text-sm font-medium">{portfolio.title}</span>
                <span
                  className={`rounded-full border px-2 py-0.5 text-[11px] ${
                    portfolio.has_comparison
                      ? "border-success/20 bg-success/10 text-success"
                      : "border-border bg-surface-2 text-text-muted"
                  }`}
                >
                  {portfolio.has_comparison ? "compared" : "exploring"}
                </span>
              </div>
              <div className="mt-2 flex items-center gap-2">
                <span className="font-mono text-xs text-text-muted">{portfolio.id}</span>
                {portfolio.problem_ref && (
                  <span className="text-xs text-text-secondary">problem {portfolio.problem_ref}</span>
                )}
              </div>
            </button>
          ))}

          {portfolios.length === 0 && (
            <p className="py-8 text-center text-sm text-text-muted">No portfolios</p>
          )}
        </div>
      </div>

      <div className="flex-1 overflow-y-auto space-y-6 pb-8">
        {showCreate && (
          <div className="space-y-5 rounded-2xl border border-border bg-surface-1 p-5">
            <div className="flex items-start justify-between gap-4">
              <div>
                <p className="text-xs uppercase tracking-[0.22em] text-text-muted">Explore</p>
                <h3 className="mt-1 text-lg font-semibold text-text-primary">Author a solution portfolio</h3>
              </div>
              <button
                onClick={() => setShowCreate(false)}
                className="rounded-lg border border-border px-3 py-1.5 text-xs text-text-secondary transition-colors hover:bg-surface-2"
              >
                Cancel
              </button>
            </div>

            <div className="grid gap-4 md:grid-cols-2">
              <label className="block space-y-1.5">
                <span className="text-xs uppercase tracking-[0.2em] text-text-muted">Problem</span>
                <select
                  value={createValue.problem_ref}
                  onChange={(event) =>
                    setCreateValue({ ...createValue, problem_ref: event.target.value })
                  }
                  className={inputClassName}
                >
                  <option value="">Select a problem</option>
                  {problems.map((problem) => (
                    <option key={problem.id} value={problem.id}>
                      {problem.title}
                    </option>
                  ))}
                </select>
              </label>

              <label className="block space-y-1.5">
                <span className="text-xs uppercase tracking-[0.2em] text-text-muted">Mode</span>
                <select
                  value={createValue.mode}
                  onChange={(event) => setCreateValue({ ...createValue, mode: event.target.value })}
                  className={inputClassName}
                >
                  <option value="tactical">tactical</option>
                  <option value="standard">standard</option>
                  <option value="deep">deep</option>
                </select>
              </label>
            </div>

            <div className="rounded-xl border border-border bg-surface-2/60 p-4">
              <div className="flex items-start justify-between gap-4">
                <div>
                  <p className="text-sm font-medium text-text-primary">Agent-assisted exploration</p>
                  <p className="mt-1 text-xs text-text-muted">
                    Spawn a task that drafts variant ideas from the framed problem before you persist them.
                  </p>
                </div>
                <button
                  onClick={handleAssist}
                  disabled={submitting}
                  className="rounded-lg border border-border px-3 py-1.5 text-xs text-text-secondary transition-colors hover:bg-surface-2 disabled:opacity-50"
                >
                  {submitting ? "Working..." : "Ask agent"}
                </button>
              </div>
              {selectedProblem && (
                <p className="mt-3 text-xs text-text-secondary">
                  Using problem <span className="font-mono">{selectedProblem.id}</span>: {selectedProblem.signal}
                </p>
              )}
            </div>

            <VariantForm
              value={createValue.variants}
              onChange={(variants) => setCreateValue({ ...createValue, variants })}
            />

            <label className="block space-y-1.5">
              <span className="text-xs uppercase tracking-[0.2em] text-text-muted">
                No Stepping Stone Rationale
              </span>
              <textarea
                value={createValue.no_stepping_stone_rationale}
                onChange={(event) =>
                  setCreateValue({
                    ...createValue,
                    no_stepping_stone_rationale: event.target.value,
                  })
                }
                className={`${inputClassName} min-h-24`}
                placeholder="Explain why none of the variants is a stepping stone if that is true."
              />
            </label>

            <div className="flex justify-end">
              <button
                onClick={handleCreate}
                disabled={submitting}
                className="rounded-full bg-accent px-4 py-2 text-sm text-surface-0 transition-colors hover:bg-accent-hover disabled:opacity-50"
              >
                {submitting ? "Saving..." : "Create portfolio"}
              </button>
            </div>
          </div>
        )}

        {detail ? (
          <PortfolioDetailView
            detail={detail}
            dimensions={dimensions}
            compareValue={compareValue}
            setCompareValue={setCompareValue}
            variantReasoning={variantReasoning}
            setVariantReasoning={setVariantReasoning}
            showCompare={showCompare}
            setShowCompare={setShowCompare}
            submitting={submitting}
            onCompare={handleCompare}
            onNavigate={onNavigate}
          />
        ) : activeId ? (
          <p className="py-8 text-center text-sm text-text-muted">Loading...</p>
        ) : (
          <div className="rounded-2xl border border-border bg-surface-1 p-6 text-sm text-text-muted">
            Select a portfolio to compare variants, or explore a new one from a framed problem.
          </div>
        )}
      </div>
    </div>
    </div>
  );
}

function PortfolioDetailView({
  detail,
  dimensions,
  compareValue,
  setCompareValue,
  variantReasoning,
  setVariantReasoning,
  showCompare,
  setShowCompare,
  submitting,
  onCompare,
  onNavigate,
}: {
  detail: PortfolioDetail;
  dimensions: ProblemDetail["characterizations"][number]["dimensions"];
  compareValue: PortfolioCompareInput;
  setCompareValue: (value: PortfolioCompareInput) => void;
  variantReasoning: VariantReasoningDraft[];
  setVariantReasoning: (value: VariantReasoningDraft[]) => void;
  showCompare: boolean;
  setShowCompare: (value: boolean) => void;
  submitting: boolean;
  onCompare: () => Promise<void>;
  onNavigate: NavigateFn;
}) {
  const nonDominated = new Set(detail.comparison?.non_dominated_set ?? []);
  const displayedTradeoffs =
    detail.comparison?.pareto_tradeoffs.filter((note) => nonDominated.has(note.variant)) ?? [];
  const displayedDominated =
    detail.comparison?.dominated_notes.filter((note) => !nonDominated.has(note.variant)) ?? [];

  return (
    <div className="space-y-6">
      <div className="flex items-start justify-between gap-4">
        <div>
          <h2 className="text-xl font-semibold">{detail.title}</h2>
          <p className="mt-1 font-mono text-xs text-text-muted">{detail.id}</p>
          {detail.problem_ref && (
            <button
              onClick={() => onNavigate("problems", detail.problem_ref)}
              className="mt-2 text-xs text-accent transition-colors hover:text-accent-hover"
            >
              Problem: {detail.problem_ref}
            </button>
          )}
        </div>

        <button
          onClick={() => setShowCompare(!showCompare)}
          className="rounded-full bg-accent px-3 py-1.5 text-xs text-surface-0 transition-colors hover:bg-accent-hover"
        >
          {detail.comparison ? "Revise compare" : "Compare variants"}
        </button>
      </div>

      <div className="space-y-3">
        {detail.variants.map((variant) => {
          const variantID = variant.id;
          const isOnFront = nonDominated.has(variantID) || nonDominated.has(variant.title);
          const isSelected =
            detail.comparison?.selected_ref === variantID ||
            detail.comparison?.selected_ref === variant.title;

          return (
            <div
              key={variant.id}
              className={`rounded-2xl border px-4 py-4 ${
                isSelected
                  ? "border-accent/30 bg-accent/5"
                  : isOnFront
                    ? "border-success/20 bg-surface-1"
                    : "border-border bg-surface-1"
              }`}
            >
              <div className="flex items-center gap-2">
                <span className="text-sm font-medium">{variant.title}</span>
                {isSelected && (
                  <span className="rounded-full border border-accent/20 bg-accent/10 px-2 py-0.5 text-xs text-accent">
                    selected
                  </span>
                )}
                {isOnFront && !isSelected && (
                  <span className="rounded-full border border-success/20 bg-success/10 px-2 py-0.5 text-xs text-success">
                    Pareto front
                  </span>
                )}
              </div>
              {variant.description && (
                <p className="mt-2 text-sm text-text-secondary">{variant.description}</p>
              )}
              <div className="mt-3 flex flex-wrap gap-3 text-xs text-text-muted">
                <span>WLNK: {variant.weakest_link}</span>
                <span>Novelty: {variant.novelty_marker}</span>
                {variant.stepping_stone && <span>stepping stone</span>}
              </div>
            </div>
          );
        })}
      </div>

      {showCompare && (
        <ComparisonEditor
          detail={detail}
          dimensions={dimensions}
          value={compareValue}
          onChange={setCompareValue}
          variantReasoning={variantReasoning}
          onReasoningChange={setVariantReasoning}
          submitting={submitting}
          onSubmit={onCompare}
        />
      )}

      {detail.comparison && <ComparisonTable comparison={detail.comparison} />}

      {displayedTradeoffs.length > 0 && (
        <NoteSection label="Pareto Trade-offs" notes={displayedTradeoffs.map((note) => ({
          variant: note.variant,
          summary: note.summary,
        }))} />
      )}

      {displayedDominated.length > 0 && (
        <NoteSection
          label="Eliminated"
          notes={displayedDominated.map((note) => ({
            variant: note.variant,
            summary: note.dominated_by.length > 0
              ? `${note.summary} Dominated by ${note.dominated_by.join(", ")}.`
              : note.summary,
          }))}
        />
      )}
    </div>
  );
}

function ComparisonEditor({
  detail,
  dimensions,
  value,
  onChange,
  variantReasoning,
  onReasoningChange,
  submitting,
  onSubmit,
}: {
  detail: PortfolioDetail;
  dimensions: ProblemDetail["characterizations"][number]["dimensions"];
  value: PortfolioCompareInput;
  onChange: (value: PortfolioCompareInput) => void;
  variantReasoning: VariantReasoningDraft[];
  onReasoningChange: (value: VariantReasoningDraft[]) => void;
  submitting: boolean;
  onSubmit: () => Promise<void>;
}) {
  if (dimensions.length === 0) {
    return (
      <div className="rounded-2xl border border-dashed border-border bg-surface-1 p-5 text-sm text-text-secondary">
        This problem has no characterization yet. Add dimensions on the problem page before you compare variants.
      </div>
    );
  }

  return (
    <div className="space-y-5 rounded-2xl border border-border bg-surface-1 p-5">
      <div className="mb-2">
        <p className="font-mono text-xs uppercase tracking-[1.2px] text-text-muted">CHOOSE</p>
        <p className="text-xs text-text-muted mt-0.5">Which is better -- and should we even decide now?</p>
      </div>
      <div>
        <p className="text-xs uppercase tracking-[0.22em] text-text-muted">Compare</p>
        <h3 className="mt-1 text-lg font-semibold text-text-primary">Score variants and let the backend compute the Pareto front</h3>
      </div>

      <div className="overflow-x-auto rounded-xl border border-border">
        <table className="w-full text-sm">
          <thead>
            <tr className="bg-surface-2 text-left text-xs text-text-muted">
              <th className="px-4 py-2.5">Dimension</th>
              {detail.variants.map((variant) => (
                <th key={variant.id} className="px-4 py-2.5">{variant.title}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {dimensions.map((dimension) => (
              <tr key={dimension.name} className="border-t border-border">
                <td className="px-4 py-3 align-top text-text-secondary">
                  <div className="font-medium text-text-primary">{dimension.name}</div>
                  <div className="mt-1 text-xs">{dimension.role} · {dimension.polarity}</div>
                </td>
                {detail.variants.map((variant) => (
                  <td key={`${variant.id}-${dimension.name}`} className="px-4 py-3">
                    <input
                      value={value.scores[variant.id]?.[dimension.name] ?? ""}
                      onChange={(event) =>
                        onChange({
                          ...value,
                          scores: {
                            ...value.scores,
                            [variant.id]: {
                              ...value.scores[variant.id],
                              [dimension.name]: event.target.value,
                            },
                          },
                        })
                      }
                      className={inputClassName}
                      placeholder="Low, 42ms, $100"
                    />
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="grid gap-4 md:grid-cols-2">
        <label className="block space-y-1.5">
          <span className="text-xs uppercase tracking-[0.2em] text-text-muted">Advisory Selection</span>
          <select
            value={value.selected_ref}
            onChange={(event) => onChange({ ...value, selected_ref: event.target.value })}
            className={inputClassName}
          >
            {detail.variants.map((variant) => (
              <option key={variant.id} value={variant.id}>
                {variant.title}
              </option>
            ))}
          </select>
        </label>

        <label className="block space-y-1.5">
          <span className="text-xs uppercase tracking-[0.2em] text-text-muted">Selection Policy</span>
          <textarea
            value={value.policy_applied}
            onChange={(event) => onChange({ ...value, policy_applied: event.target.value })}
            className={`${inputClassName} min-h-24`}
            placeholder="State the rule that turns the compare table into a recommendation."
          />
        </label>
      </div>

      <label className="block space-y-1.5">
        <span className="text-xs uppercase tracking-[0.2em] text-text-muted">Recommendation Rationale</span>
        <textarea
          value={value.recommendation}
          onChange={(event) => onChange({ ...value, recommendation: event.target.value })}
          className={`${inputClassName} min-h-24`}
          placeholder="Explain why the advisory selection is preferred."
        />
      </label>

      <div className="space-y-3">
        <div>
          <p className="text-xs uppercase tracking-[0.2em] text-text-muted">Variant reasoning</p>
          <p className="mt-1 text-xs text-text-muted">
            Capture the elimination or trade-off narrative for each variant. The backend still computes the Pareto front.
          </p>
        </div>

        {variantReasoning.map((reasoning, index) => {
          const variant = detail.variants.find((item) => item.id === reasoning.variant);
          return (
            <div key={reasoning.variant} className="grid gap-3 rounded-xl border border-border bg-surface-2/60 p-4 md:grid-cols-[220px_1fr_240px]">
              <div className="text-sm font-medium text-text-primary">{variant?.title ?? reasoning.variant}</div>
              <textarea
                value={reasoning.summary}
                onChange={(event) =>
                  onReasoningChange(
                    variantReasoning.map((current, currentIndex) =>
                      currentIndex === index ? { ...current, summary: event.target.value } : current,
                    ),
                  )
                }
                className={`${inputClassName} min-h-24`}
                placeholder="What does this variant win or lose on?"
              />
              <input
                value={reasoning.dominated_by}
                onChange={(event) =>
                  onReasoningChange(
                    variantReasoning.map((current, currentIndex) =>
                      currentIndex === index ? { ...current, dominated_by: event.target.value } : current,
                    ),
                  )
                }
                className={inputClassName}
                placeholder="Dominated by (optional)"
              />
            </div>
          );
        })}
      </div>

      <div className="flex justify-end">
        <button
          onClick={onSubmit}
          disabled={submitting}
          className="rounded-full bg-accent px-4 py-2 text-sm text-surface-0 transition-colors hover:bg-accent-hover disabled:opacity-50"
        >
          {submitting ? "Saving..." : detail.comparison ? "Recompute compare" : "Compute Pareto front"}
        </button>
      </div>
    </div>
  );
}

function ComparisonTable({
  comparison,
}: {
  comparison: NonNullable<PortfolioDetail["comparison"]>;
}) {
  const variants = Object.keys(comparison.scores);
  const nonDominated = new Set(comparison.non_dominated_set);

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-medium uppercase tracking-wider text-text-secondary">Compare / Pareto</h3>
        {comparison.policy_applied && (
          <span className="rounded-full border border-border bg-surface-2 px-2.5 py-1 text-xs text-text-muted">
            {comparison.policy_applied}
          </span>
        )}
      </div>

      <div className="overflow-x-auto rounded-xl border border-border">
        <table className="w-full text-sm">
          <thead>
            <tr className="bg-surface-2 text-left text-xs text-text-muted">
              <th className="px-4 py-2.5">Dimension</th>
              {variants.map((variant) => (
                <th
                  key={variant}
                  className={`px-4 py-2.5 ${
                    comparison.selected_ref === variant
                      ? "text-accent"
                      : nonDominated.has(variant)
                        ? "text-success"
                        : "text-text-muted"
                  }`}
                >
                  {variant}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {comparison.dimensions.map((dimension) => (
              <tr key={dimension} className="border-t border-border">
                <td className="px-4 py-3 text-text-secondary">{dimension}</td>
                {variants.map((variant) => (
                  <td key={`${variant}-${dimension}`} className="px-4 py-3 text-text-primary">
                    {comparison.scores[variant]?.[dimension] ?? "—"}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="rounded-xl border border-success/20 bg-success/10 px-4 py-3 text-sm text-success">
        Computed Pareto front: {comparison.non_dominated_set.join(", ") || "none"}
      </div>

      {comparison.recommendation && (
        <div className="rounded-xl border border-accent/20 bg-accent/5 px-4 py-3 text-sm text-text-primary">
          <span className="text-xs uppercase tracking-[0.2em] text-accent">Recommendation</span>
          <p className="mt-2">{comparison.recommendation}</p>
        </div>
      )}
    </div>
  );
}

function NoteSection({
  label,
  notes,
}: {
  label: string;
  notes: { variant: string; summary: string }[];
}) {
  return (
    <div className="space-y-2">
      <h4 className="text-xs uppercase tracking-wider text-text-muted">{label}</h4>
      {notes.map((note) => (
        <div key={`${label}-${note.variant}`} className="rounded-xl border border-border bg-surface-1 px-4 py-3">
          <span className="text-sm font-medium text-text-primary">{note.variant}</span>
          <p className="mt-1 text-sm text-text-secondary">{note.summary}</p>
        </div>
      ))}
    </div>
  );
}

function emptyPortfolioInput(): PortfolioCreateInput {
  return {
    problem_ref: "",
    context: "",
    mode: "standard",
    no_stepping_stone_rationale: "",
    variants: [
      {
        id: "var-1",
        title: "",
        description: "",
        strengths: [""],
        weakest_link: "",
        novelty_marker: "",
        risks: [""],
        stepping_stone: true,
        stepping_stone_basis: "",
        diversity_role: "",
        assumption_notes: "",
        rollback_notes: "",
        evidence_refs: [],
      },
      {
        id: "var-2",
        title: "",
        description: "",
        strengths: [""],
        weakest_link: "",
        novelty_marker: "",
        risks: [""],
        stepping_stone: false,
        stepping_stone_basis: "",
        diversity_role: "",
        assumption_notes: "",
        rollback_notes: "",
        evidence_refs: [],
      },
    ],
  };
}

function emptyComparisonInput(): PortfolioCompareInput {
  return {
    portfolio_ref: "",
    dimensions: [],
    scores: {},
    non_dominated_set: [],
    incomparable: [],
    dominated_notes: [],
    pareto_tradeoffs: [],
    policy_applied: "",
    selected_ref: "",
    recommendation: "",
    parity_plan: null,
  };
}

function buildScoreDraft(detail: PortfolioDetail, problem: ProblemDetail | null): Record<string, Record<string, string>> {
  const dimensionNames = problem?.latest_characterization?.dimensions.map((dimension) => dimension.name)
    ?? detail.comparison?.dimensions
    ?? [];
  const scores: Record<string, Record<string, string>> = {};

  detail.variants.forEach((variant) => {
    scores[variant.id] = {};
    dimensionNames.forEach((dimension) => {
      scores[variant.id][dimension] = detail.comparison?.scores[variant.id]?.[dimension] ?? "";
    });
  });

  return scores;
}

function buildExplorationPrompt(problem: ProblemDetail): string {
  const lines = [
    `Explore solution variants for problem: ${problem.title}`,
    "",
    "Signal:",
    problem.signal,
    "",
    "Constraints:",
    ...problem.constraints.map((constraint) => `- ${constraint}`),
  ];

  if (problem.latest_characterization?.dimensions.length) {
    lines.push("", "Characterized dimensions:");
    lines.push(
      ...problem.latest_characterization.dimensions.map(
        (dimension) => `- ${dimension.name} (${dimension.role}, ${dimension.polarity})`,
      ),
    );
  }

  lines.push(
    "",
    "Return at least 3 genuinely distinct variants.",
    "For each variant include: title, description, novelty marker, weakest link, strengths, risks, and whether it is a stepping stone.",
  );

  return lines.join("\n");
}

function splitCommaList(value: string): string[] {
  return value
    .split(",")
    .map((entry) => entry.trim())
    .filter(Boolean);
}

const inputClassName =
  "w-full rounded-xl border border-border bg-surface-2 px-3 py-2 text-sm text-text-primary outline-none transition-colors focus:border-accent/60";
