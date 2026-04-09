import { useState } from "react";

import {
  type DimensionInput,
  type ParityPlanInput,
} from "../lib/api";

interface DimensionEditorProps {
  initialDimensions: DimensionInput[];
  initialParityPlan: ParityPlanInput | null;
  initialParityRules: string;
  onSubmit: (value: {
    dimensions: DimensionInput[];
    parity_plan: ParityPlanInput | null;
    parity_rules: string;
  }) => Promise<void> | void;
  onCancel?: () => void;
  submitting: boolean;
}

export function DimensionEditor({
  initialDimensions,
  initialParityPlan,
  initialParityRules,
  onSubmit,
  onCancel,
  submitting,
}: DimensionEditorProps) {
  const [dimensions, setDimensions] = useState<DimensionInput[]>(
    initialDimensions.length > 0 ? initialDimensions : [emptyDimension()],
  );
  const [parityRules, setParityRules] = useState(initialParityRules);
  const [parityPlan, setParityPlan] = useState<ParityPlanInput | null>(initialParityPlan);

  const submit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    await onSubmit({
      dimensions,
      parity_plan: parityPlan,
      parity_rules: parityRules.trim(),
    });
  };

  return (
    <form onSubmit={submit} className="space-y-5 rounded-2xl border border-border bg-surface-1 p-5">
      <div className="flex items-start justify-between gap-4">
        <div>
          <p className="text-xs uppercase tracking-[0.22em] text-text-muted">Characterize</p>
          <h3 className="mt-1 text-lg font-semibold text-text-primary">Define how variants will be compared</h3>
        </div>
        {onCancel && (
          <button
            type="button"
            onClick={onCancel}
            className="rounded-lg border border-border px-3 py-1.5 text-xs text-text-secondary transition-colors hover:bg-surface-2"
          >
            Cancel
          </button>
        )}
      </div>

      <div className="space-y-3">
        {dimensions.map((dimension, index) => (
          <div key={`dimension-${index}`} className="rounded-xl border border-border bg-surface-2/60 p-4">
            <div className="grid gap-3 md:grid-cols-2">
              <LabeledField label="Name">
                <input
                  value={dimension.name}
                  onChange={(event) =>
                    setDimensions(updateDimensions(dimensions, index, "name", event.target.value))
                  }
                  className={inputClassName}
                  placeholder="operator load"
                />
              </LabeledField>

              <LabeledField label="Role">
                <select
                  value={dimension.role}
                  onChange={(event) =>
                    setDimensions(updateDimensions(dimensions, index, "role", event.target.value))
                  }
                  className={inputClassName}
                >
                  <option value="target">target</option>
                  <option value="constraint">constraint</option>
                  <option value="observation">observation</option>
                </select>
              </LabeledField>

              <LabeledField label="Scale">
                <select
                  value={dimension.scale_type}
                  onChange={(event) =>
                    setDimensions(updateDimensions(dimensions, index, "scale_type", event.target.value))
                  }
                  className={inputClassName}
                >
                  <option value="ordinal">ordinal</option>
                  <option value="ratio">ratio</option>
                  <option value="nominal">nominal</option>
                </select>
              </LabeledField>

              <LabeledField label="Polarity">
                <select
                  value={dimension.polarity}
                  onChange={(event) =>
                    setDimensions(updateDimensions(dimensions, index, "polarity", event.target.value))
                  }
                  className={inputClassName}
                >
                  <option value="higher_better">higher_better</option>
                  <option value="lower_better">lower_better</option>
                </select>
              </LabeledField>

              <LabeledField label="Unit">
                <input
                  value={dimension.unit}
                  onChange={(event) =>
                    setDimensions(updateDimensions(dimensions, index, "unit", event.target.value))
                  }
                  className={inputClassName}
                  placeholder="ms, $, score"
                />
              </LabeledField>

              <LabeledField label="Valid Until">
                <input
                  value={dimension.valid_until}
                  onChange={(event) =>
                    setDimensions(updateDimensions(dimensions, index, "valid_until", event.target.value))
                  }
                  className={inputClassName}
                  placeholder="2026-07-01"
                />
              </LabeledField>
            </div>

            <LabeledField label="How To Measure">
              <textarea
                value={dimension.how_to_measure}
                onChange={(event) =>
                  setDimensions(updateDimensions(dimensions, index, "how_to_measure", event.target.value))
                }
                className={`${inputClassName} mt-1 min-h-20`}
                placeholder="Explain how scores should be collected."
              />
            </LabeledField>

            <div className="mt-3 flex justify-end">
              <button
                type="button"
                onClick={() => setDimensions(dimensions.filter((_, currentIndex) => currentIndex !== index))}
                className="rounded-lg border border-border px-3 py-1.5 text-xs text-text-secondary transition-colors hover:bg-surface-2"
              >
                Remove dimension
              </button>
            </div>
          </div>
        ))}
      </div>

      <button
        type="button"
        onClick={() => setDimensions([...dimensions, emptyDimension()])}
        className="rounded-lg border border-dashed border-border px-3 py-2 text-xs text-text-secondary transition-colors hover:border-accent/50 hover:text-text-primary"
      >
        Add dimension
      </button>

      <LabeledField label="Parity Rules">
        <textarea
          value={parityRules}
          onChange={(event) => setParityRules(event.target.value)}
          className={`${inputClassName} min-h-20`}
          placeholder="What must be equal across variants for the comparison to be fair?"
        />
      </LabeledField>

      <div className="rounded-xl border border-border bg-surface-2/60 p-4">
        <div className="flex items-center justify-between gap-3">
          <div>
            <p className="text-sm font-medium text-text-primary">Structured parity plan</p>
            <p className="mt-1 text-xs text-text-muted">
              Optional for tactical work, recommended for standard, required in deep comparisons.
            </p>
          </div>
          <button
            type="button"
            onClick={() => setParityPlan(parityPlan ? null : emptyParityPlan())}
            className="rounded-lg border border-border px-3 py-1.5 text-xs text-text-secondary transition-colors hover:bg-surface-2"
          >
            {parityPlan ? "Remove" : "Add plan"}
          </button>
        </div>

        {parityPlan && (
          <div className="mt-4 grid gap-3 md:grid-cols-2">
            <LabeledField label="Baseline Set">
              <input
                value={parityPlan.baseline_set.join(", ")}
                onChange={(event) =>
                  setParityPlan({
                    ...parityPlan,
                    baseline_set: splitCommaList(event.target.value),
                  })
                }
                className={inputClassName}
                placeholder="var-1, var-2"
              />
            </LabeledField>

            <LabeledField label="Missing Data Policy">
              <select
                value={parityPlan.missing_data_policy}
                onChange={(event) =>
                  setParityPlan({
                    ...parityPlan,
                    missing_data_policy: event.target.value,
                  })
                }
                className={inputClassName}
              >
                <option value="explicit_abstain">explicit_abstain</option>
                <option value="zero">zero</option>
                <option value="exclude">exclude</option>
              </select>
            </LabeledField>

            <LabeledField label="Window">
              <input
                value={parityPlan.window}
                onChange={(event) => setParityPlan({ ...parityPlan, window: event.target.value })}
                className={inputClassName}
                placeholder="single release step"
              />
            </LabeledField>

            <LabeledField label="Budget">
              <input
                value={parityPlan.budget}
                onChange={(event) => setParityPlan({ ...parityPlan, budget: event.target.value })}
                className={inputClassName}
                placeholder="one iteration"
              />
            </LabeledField>

            <LabeledField label="Pinned Conditions">
              <input
                value={parityPlan.pinned_conditions.join(", ")}
                onChange={(event) =>
                  setParityPlan({
                    ...parityPlan,
                    pinned_conditions: splitCommaList(event.target.value),
                  })
                }
                className={inputClassName}
                placeholder="same project, same input set"
              />
            </LabeledField>
          </div>
        )}
      </div>

      <div className="flex justify-end">
        <button
          type="submit"
          disabled={submitting}
          className="rounded-lg bg-accent px-4 py-2 text-sm text-white transition-colors hover:bg-accent-hover disabled:opacity-50"
        >
          {submitting ? "Saving..." : "Save characterization"}
        </button>
      </div>
    </form>
  );
}

function updateDimensions(
  dimensions: DimensionInput[],
  index: number,
  key: keyof DimensionInput,
  value: string,
): DimensionInput[] {
  return dimensions.map((dimension, currentIndex) =>
    currentIndex === index
      ? {
          ...dimension,
          [key]: value,
        }
      : dimension,
  );
}

function emptyDimension(): DimensionInput {
  return {
    name: "",
    scale_type: "ordinal",
    unit: "",
    polarity: "higher_better",
    role: "target",
    how_to_measure: "",
    valid_until: "",
  };
}

function emptyParityPlan(): ParityPlanInput {
  return {
    baseline_set: [],
    window: "",
    budget: "",
    normalization: [],
    missing_data_policy: "explicit_abstain",
    pinned_conditions: [],
  };
}

function splitCommaList(value: string): string[] {
  return value
    .split(",")
    .map((entry) => entry.trim())
    .filter(Boolean);
}

function LabeledField({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}) {
  return (
    <label className="block space-y-1.5">
      <span className="text-xs uppercase tracking-[0.2em] text-text-muted">{label}</span>
      {children}
    </label>
  );
}

const inputClassName =
  "w-full rounded-xl border border-border bg-surface-2 px-3 py-2 text-sm text-text-primary outline-none transition-colors focus:border-accent/60";
