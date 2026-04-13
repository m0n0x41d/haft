import { useEffect, useState } from "react";

import { DimensionEditor } from "../components/DimensionEditor";
import { ProblemForm } from "../components/ProblemForm";
import {
  characterizeProblem,
  createProblem,
  getProblem,
  listProblems,
  type CharacterizationView,
  type ProblemDetail,
  type ProblemSummary,
} from "../lib/api";
import { reportError } from "../lib/errors";

type NavigateFn = (
  page: "dashboard" | "problems" | "portfolios" | "decisions",
  id?: string,
) => void;

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
  const [showCreate, setShowCreate] = useState(false);
  const [showCharacterize, setShowCharacterize] = useState(false);
  const [submitting, setSubmitting] = useState(false);

  const refreshProblems = async () => {
    try {
      const nextProblems = await listProblems();
      setProblems(nextProblems);
    } catch (error) {
      reportError(error, "problems");
    }
  };

  useEffect(() => {
    void refreshProblems();
  }, []);

  useEffect(() => {
    setActiveId(selectedId);
  }, [selectedId]);

  useEffect(() => {
    if (!activeId) {
      setDetail(null);
      return;
    }

    getProblem(activeId)
      .then(setDetail)
      .catch((error) => {
        reportError(error, "problem detail");
      });
  }, [activeId]);

  const handleCreate = async (value: Parameters<typeof createProblem>[0]) => {
    setSubmitting(true);

    try {
      const created = await createProblem(value);
      setDetail(created);
      setActiveId(created.id);
      setShowCreate(false);
      await refreshProblems();
    } catch (error) {
      reportError(error, "create problem");
    } finally {
      setSubmitting(false);
    }
  };

  const handleCharacterize = async (value: {
    dimensions: Parameters<typeof characterizeProblem>[0]["dimensions"];
    parity_plan: Parameters<typeof characterizeProblem>[0]["parity_plan"];
    parity_rules: string;
  }) => {
    if (!detail) {
      return;
    }

    setSubmitting(true);

    try {
      const updated = await characterizeProblem({
        problem_ref: detail.id,
        dimensions: value.dimensions,
        parity_plan: value.parity_plan,
        parity_rules: value.parity_rules,
      });
      setDetail(updated);
      setShowCharacterize(false);
    } catch (error) {
      reportError(error, "characterize problem");
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div>
      <div className="mb-4">
        <p className="font-mono text-xs uppercase tracking-[1.2px] text-text-muted">UNDERSTAND</p>
        <p className="text-xs text-text-muted mt-0.5">Frame the problem as a signal, not a solution</p>
      </div>
      <div className="flex gap-6 h-[calc(100vh-7rem)]">
      <div className="w-80 shrink-0 overflow-y-auto space-y-3">
        <button
          onClick={() => {
            setShowCreate(true);
            setShowCharacterize(false);
          }}
          className="w-full rounded-xl border border-dashed border-accent/40 bg-accent/5 px-4 py-3 text-left text-sm text-text-primary transition-colors hover:border-accent/60 hover:bg-accent/10"
        >
          <span className="font-medium">+ Frame problem</span>
          <p className="mt-1 text-xs text-text-muted">Start the left side of the reasoning loop.</p>
        </button>

        <div className="space-y-1">
          {problems.map((problem) => (
            <button
              key={problem.id}
              onClick={() => {
                setActiveId(problem.id);
                setShowCreate(false);
                setShowCharacterize(false);
              }}
              className={`w-full rounded-lg border px-4 py-3 text-left transition-colors ${
                activeId === problem.id
                  ? "border-accent/30 bg-surface-2"
                  : "border-transparent bg-surface-1 hover:border-border hover:bg-surface-2"
              }`}
            >
              <div className="flex items-center justify-between gap-3">
                <span className="truncate text-sm font-medium">{problem.title}</span>
                <ModeBadge mode={problem.mode} />
              </div>
              <p className="mt-1 line-clamp-2 text-xs text-text-secondary">{problem.signal}</p>
              <div className="mt-2 flex items-center gap-2">
                <span className="font-mono text-xs text-text-muted">{problem.id}</span>
                {problem.reversibility && <ReversibilityBadge value={problem.reversibility} />}
              </div>
            </button>
          ))}

          {problems.length === 0 && (
            <p className="py-8 text-center text-sm text-text-muted">No active problems</p>
          )}
        </div>
      </div>

      <div className="flex-1 overflow-y-auto space-y-6 pb-8">
        {showCreate && (
          <ProblemForm
            onSubmit={handleCreate}
            onCancel={() => setShowCreate(false)}
            submitting={submitting}
          />
        )}

        {showCharacterize && detail && (
          <DimensionEditor
            initialDimensions={detail.latest_characterization?.dimensions ?? []}
            initialParityPlan={detail.latest_characterization?.parity_plan ?? null}
            initialParityRules=""
            onSubmit={handleCharacterize}
            onCancel={() => setShowCharacterize(false)}
            submitting={submitting}
          />
        )}

        {detail ? (
          <ProblemDetailPanel
            detail={detail}
            onNavigate={onNavigate}
            onCharacterize={() => {
              setShowCharacterize(true);
              setShowCreate(false);
            }}
          />
        ) : activeId ? (
          <p className="py-8 text-center text-sm text-text-muted">Loading...</p>
        ) : (
          <div className="rounded-2xl border border-border bg-surface-1 p-6 text-sm text-text-muted">
            Select a problem to inspect it, or frame a new one to start the loop.
          </div>
        )}
      </div>
    </div>
    </div>
  );
}

function ProblemDetailPanel({
  detail,
  onNavigate,
  onCharacterize,
}: {
  detail: ProblemDetail;
  onNavigate: NavigateFn;
  onCharacterize: () => void;
}) {
  const latestCharacterization = detail.latest_characterization;

  return (
    <div className="space-y-6">
      <div className="flex items-start justify-between gap-4">
        <div>
          <div className="mb-2 flex items-center gap-3">
            <ModeBadge mode={detail.mode} />
            <StatusBadge status={detail.status} />
          </div>
          <h2 className="text-xl font-semibold">{detail.title}</h2>
          <p className="mt-1 font-mono text-xs text-text-muted">{detail.id}</p>
        </div>

        <div className="flex items-center gap-2">
          {detail.linked_portfolios.length > 0 && (
            <button
              onClick={() => onNavigate("portfolios", detail.linked_portfolios[0]?.id)}
              className="rounded-lg border border-border px-3 py-1.5 text-xs text-text-secondary transition-colors hover:bg-surface-2"
            >
              Compare
            </button>
          )}
          <button
            onClick={onCharacterize}
            className="rounded-full bg-accent px-3 py-1.5 text-xs text-surface-0 transition-colors hover:bg-accent-hover"
          >
            {latestCharacterization ? "Revise characterization" : "Characterize"}
          </button>
        </div>
      </div>

      <Field label="Signal" value={detail.signal} />

      {detail.constraints.length > 0 && <ListField label="Constraints" items={detail.constraints} />}
      {detail.optimization_targets.length > 0 && (
        <ListField label="Optimization Targets" items={detail.optimization_targets} />
      )}
      {detail.observation_indicators.length > 0 && (
        <ListField
          label="Observation Indicators (Anti-Goodhart)"
          items={detail.observation_indicators}
        />
      )}
      {detail.acceptance && <Field label="Acceptance" value={detail.acceptance} />}

      <div className="grid gap-4 md:grid-cols-2">
        {detail.blast_radius && <Field label="Blast Radius" value={detail.blast_radius} />}
        {detail.reversibility && <Field label="Reversibility" value={detail.reversibility} />}
      </div>

      <CharacterizationCard characterization={latestCharacterization} />

      {detail.linked_portfolios.length > 0 && (
        <LinkedList
          label="Solution Portfolios"
          items={detail.linked_portfolios}
          onOpen={(id) => onNavigate("portfolios", id)}
        />
      )}

      {detail.linked_decisions.length > 0 && (
        <LinkedList
          label="Decisions"
          items={detail.linked_decisions}
          onOpen={(id) => onNavigate("decisions", id)}
        />
      )}
    </div>
  );
}

function CharacterizationCard({
  characterization,
}: {
  characterization: CharacterizationView | null;
}) {
  if (!characterization) {
    return (
      <div className="rounded-2xl border border-dashed border-border bg-surface-1 p-5">
        <p className="text-xs uppercase tracking-[0.2em] text-text-muted">Characterization</p>
        <p className="mt-2 text-sm text-text-secondary">
          No comparison dimensions yet. Add them before you compare variants so the Pareto front means something.
        </p>
      </div>
    );
  }

  return (
    <div className="space-y-4 rounded-2xl border border-border bg-surface-1 p-5">
      <div>
        <p className="text-xs uppercase tracking-[0.2em] text-text-muted">Characterization v{characterization.version}</p>
        <p className="mt-2 text-sm text-text-secondary">
          Dimensions define what the compare step will evaluate and how each score should be interpreted.
        </p>
      </div>

      <div className="overflow-hidden rounded-xl border border-border">
        <table className="w-full text-sm">
          <thead>
            <tr className="bg-surface-2 text-left text-xs text-text-muted">
              <th className="px-4 py-2.5">Dimension</th>
              <th className="px-4 py-2.5">Role</th>
              <th className="px-4 py-2.5">Polarity</th>
              <th className="px-4 py-2.5">Measure</th>
            </tr>
          </thead>
          <tbody>
            {characterization.dimensions.map((dimension) => (
              <tr key={dimension.name} className="border-t border-border">
                <td className="px-4 py-3 text-text-primary">{dimension.name}</td>
                <td className="px-4 py-3 text-text-secondary">{dimension.role || "target"}</td>
                <td className="px-4 py-3 text-text-secondary">{dimension.polarity || "n/a"}</td>
                <td className="px-4 py-3 text-text-secondary">{dimension.how_to_measure || "Not specified"}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {characterization.parity_plan && (
        <div className="rounded-xl border border-border bg-surface-2/60 p-4 text-sm text-text-secondary">
          <p className="text-xs uppercase tracking-[0.2em] text-text-muted">Parity Plan</p>
          <div className="mt-3 grid gap-2 md:grid-cols-2">
            <span>Baseline: {characterization.parity_plan.baseline_set?.join(", ") || "Not set"}</span>
            <span>Window: {characterization.parity_plan.window || "Not set"}</span>
            <span>Budget: {characterization.parity_plan.budget || "Not set"}</span>
            <span>Missing data: {characterization.parity_plan.missing_data_policy || "Not set"}</span>
          </div>
          {characterization.parity_plan.pinned_conditions?.length > 0 && (
            <p className="mt-3">
              Pinned conditions: {characterization.parity_plan.pinned_conditions.join(", ")}
            </p>
          )}
        </div>
      )}
    </div>
  );
}

function Field({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <h4 className="mb-1 text-xs uppercase tracking-wider text-text-muted">{label}</h4>
      <p className="rounded-lg border border-border bg-surface-1 px-4 py-3 text-sm text-text-primary">
        {value}
      </p>
    </div>
  );
}

function ListField({ label, items }: { label: string; items: string[] }) {
  return (
    <div>
      <h4 className="mb-2 text-xs uppercase tracking-wider text-text-muted">{label}</h4>
      <ul className="space-y-2">
        {items.map((item) => (
          <li
            key={`${label}-${item}`}
            className="rounded-lg border border-border bg-surface-1 px-4 py-2 text-sm text-text-primary"
          >
            {item}
          </li>
        ))}
      </ul>
    </div>
  );
}

function LinkedList({
  label,
  items,
  onOpen,
}: {
  label: string;
  items: ProblemDetail["linked_portfolios"];
  onOpen: (id: string) => void;
}) {
  return (
    <div>
      <h4 className="mb-2 text-xs uppercase tracking-wider text-text-muted">{label}</h4>
      <div className="space-y-2">
        {items.map((item) => (
          <button
            key={item.id}
            onClick={() => onOpen(item.id)}
            className="block w-full rounded-lg border border-border bg-surface-1 px-4 py-3 text-left transition-colors hover:border-accent/30 hover:bg-surface-2"
          >
            <span className="text-sm text-accent">{item.title}</span>
            <span className="ml-2 font-mono text-xs text-text-muted">{item.id}</span>
          </button>
        ))}
      </div>
    </div>
  );
}

function ModeBadge({ mode }: { mode: string }) {
  const colors: Record<string, string> = {
    tactical: "border-blue-500/20 bg-blue-500/10 text-blue-400",
    standard: "border-accent/20 bg-accent/10 text-accent",
    deep: "border-purple-500/20 bg-purple-500/10 text-purple-400",
    note: "border-border bg-surface-2 text-text-muted",
  };

  return (
    <span className={`rounded-full border px-2 py-0.5 text-xs ${colors[mode] ?? colors.note}`}>
      {mode}
    </span>
  );
}

function StatusBadge({ status }: { status: string }) {
  const colors: Record<string, string> = {
    active: "border-success/20 bg-success/10 text-success",
    refresh_due: "border-warning/20 bg-warning/10 text-warning",
    superseded: "border-border bg-surface-2 text-text-muted",
  };

  return (
    <span className={`rounded-full border px-2 py-0.5 text-xs ${colors[status] ?? colors.superseded}`}>
      {status}
    </span>
  );
}

function ReversibilityBadge({ value }: { value: string }) {
  const colors: Record<string, string> = {
    low: "text-danger",
    medium: "text-warning",
    high: "text-success",
  };

  return <span className={`text-xs ${colors[value] ?? "text-text-muted"}`}>{value} reversibility</span>;
}
