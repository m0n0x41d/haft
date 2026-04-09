import { useEffect, useState } from "react";

import {
  type DecisionCreateInput,
  type DecisionPredictionInput,
  type DecisionRejectionInput,
  type PortfolioDetail,
  type VariantView,
} from "../lib/api";

interface DecisionFormProps {
  portfolio: PortfolioDetail;
  onSubmit: (value: DecisionCreateInput) => Promise<void> | void;
  onCancel?: () => void;
  submitting: boolean;
}

export function DecisionForm({
  portfolio,
  onSubmit,
  onCancel,
  submitting,
}: DecisionFormProps) {
  const [value, setValue] = useState<DecisionCreateInput>(() => buildDecisionInput(portfolio));

  useEffect(() => {
    setValue(buildDecisionInput(portfolio));
  }, [portfolio]);

  const selectedVariant = portfolio.variants.find(
    (variant) => variant.id === value.selected_ref || variant.title === value.selected_title,
  );
  const mode = value.mode || "standard";

  const submit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    await onSubmit({
      ...value,
      selected_title: selectedVariant?.title ?? value.selected_title.trim(),
      weakest_link: value.weakest_link.trim() || selectedVariant?.weakest_link || "",
    });
  };

  return (
    <form onSubmit={submit} className="space-y-5 rounded-2xl border border-border bg-surface-1 p-5">
      <div className="flex items-start justify-between gap-4">
        <div>
          <p className="text-xs uppercase tracking-[0.22em] text-text-muted">Decide</p>
          <h3 className="mt-1 text-lg font-semibold text-text-primary">Record the selected variant and the contract around it</h3>
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

      <div className="grid gap-4 md:grid-cols-2">
        <Field label="Mode">
          <select
            value={value.mode}
            onChange={(event) => setValue({ ...value, mode: event.target.value })}
            className={inputClassName}
          >
            <option value="tactical">tactical</option>
            <option value="standard">standard</option>
            <option value="deep">deep</option>
          </select>
        </Field>

        <Field label="Selected Variant">
          <select
            value={value.selected_ref}
            onChange={(event) => {
              const nextRef = event.target.value;
              const variant = portfolio.variants.find((item) => item.id === nextRef);
              setValue({
                ...value,
                selected_ref: nextRef,
                selected_title: variant?.title ?? "",
                weakest_link: variant?.weakest_link ?? value.weakest_link,
              });
            }}
            className={inputClassName}
          >
            {portfolio.variants.map((variant) => (
              <option key={variant.id} value={variant.id}>
                {variant.title}
              </option>
            ))}
          </select>
        </Field>
      </div>

      <Field label="Selection Policy" required>
        <textarea
          value={value.selection_policy}
          onChange={(event) => setValue({ ...value, selection_policy: event.target.value })}
          className={`${inputClassName} min-h-20`}
          placeholder="State the rule used to choose the winner."
        />
      </Field>

      <div className="grid gap-4 md:grid-cols-2">
        <Field label="Why Selected" required>
          <textarea
            value={value.why_selected}
            onChange={(event) => setValue({ ...value, why_selected: event.target.value })}
            className={`${inputClassName} min-h-28`}
            placeholder="Why did this variant win?"
          />
        </Field>

        <Field label="Counterargument" required>
          <textarea
            value={value.counterargument}
            onChange={(event) => setValue({ ...value, counterargument: event.target.value })}
            className={`${inputClassName} min-h-28`}
            placeholder="What is the strongest argument against it?"
          />
        </Field>
      </div>

      <Field label="Weakest Link" required>
        <input
          value={value.weakest_link}
          onChange={(event) => setValue({ ...value, weakest_link: event.target.value })}
          className={inputClassName}
          placeholder="What most plausibly breaks this choice?"
        />
      </Field>

      <EditableRejections
        values={value.why_not_others}
        onChange={(why_not_others) => setValue({ ...value, why_not_others })}
        variants={portfolio.variants}
      />

      <div className="grid gap-4 md:grid-cols-2">
        <StringListEditor
          label="Rollback Triggers"
          values={value.rollback?.triggers ?? [""]}
          onChange={(triggers) =>
            setValue({
              ...value,
              rollback: {
                triggers,
                steps: value.rollback?.steps ?? [""],
                blast_radius: value.rollback?.blast_radius ?? "",
              },
            })
          }
        />

        <StringListEditor
          label="Rollback Steps"
          values={value.rollback?.steps ?? [""]}
          onChange={(steps) =>
            setValue({
              ...value,
              rollback: {
                triggers: value.rollback?.triggers ?? [""],
                steps,
                blast_radius: value.rollback?.blast_radius ?? "",
              },
            })
          }
        />
      </div>

      <Field label="Rollback Blast Radius">
        <input
          value={value.rollback?.blast_radius ?? ""}
          onChange={(event) =>
            setValue({
              ...value,
              rollback: {
                triggers: value.rollback?.triggers ?? [""],
                steps: value.rollback?.steps ?? [""],
                blast_radius: event.target.value,
              },
            })
          }
          className={inputClassName}
          placeholder="What does reversal touch?"
        />
      </Field>

      <div className="rounded-xl border border-border bg-surface-2/60 p-4">
        <p className="text-xs uppercase tracking-[0.2em] text-text-muted">Depth-aware fields</p>
        <p className="mt-1 text-xs text-text-muted">
          Tactical mode keeps the contract minimal. Standard and deep modes surface more implementation and refresh detail.
        </p>
      </div>

      {mode !== "tactical" && (
        <div className="grid gap-4 md:grid-cols-2">
          <StringListEditor
            label="Invariants"
            values={value.invariants}
            onChange={(invariants) => setValue({ ...value, invariants })}
          />

          <StringListEditor
            label="Admissibility"
            values={value.admissibility}
            onChange={(admissibility) => setValue({ ...value, admissibility })}
          />

          <StringListEditor
            label="Pre-conditions"
            values={value.pre_conditions}
            onChange={(pre_conditions) => setValue({ ...value, pre_conditions })}
          />

          <StringListEditor
            label="Post-conditions"
            values={value.post_conditions}
            onChange={(post_conditions) => setValue({ ...value, post_conditions })}
          />
        </div>
      )}

      {mode === "deep" && (
        <div className="space-y-4">
          <StringListEditor
            label="Evidence Requirements"
            values={value.evidence_requirements}
            onChange={(evidence_requirements) => setValue({ ...value, evidence_requirements })}
          />

          <StringListEditor
            label="Refresh Triggers"
            values={value.refresh_triggers}
            onChange={(refresh_triggers) => setValue({ ...value, refresh_triggers })}
          />

          <PredictionEditor
            values={value.predictions}
            onChange={(predictions) => setValue({ ...value, predictions })}
          />

          <Field label="Valid Until">
            <input
              value={value.valid_until}
              onChange={(event) => setValue({ ...value, valid_until: event.target.value })}
              className={inputClassName}
              placeholder="2026-07-01"
            />
          </Field>
        </div>
      )}

      <div className="flex justify-end">
        <button
          type="submit"
          disabled={submitting}
          className="rounded-lg bg-accent px-4 py-2 text-sm text-white transition-colors hover:bg-accent-hover disabled:opacity-50"
        >
          {submitting ? "Saving..." : "Create Decision"}
        </button>
      </div>
    </form>
  );
}

function buildDecisionInput(portfolio: PortfolioDetail): DecisionCreateInput {
  const selectedVariant =
    portfolio.variants.find((variant) => variant.id === portfolio.comparison?.selected_ref) ??
    portfolio.variants[0];

  return {
    problem_ref: portfolio.problem_ref,
    problem_refs: portfolio.problem_ref ? [portfolio.problem_ref] : [],
    portfolio_ref: portfolio.id,
    selected_ref: selectedVariant?.id ?? "",
    selected_title: selectedVariant?.title ?? "",
    why_selected: "",
    selection_policy: portfolio.comparison?.policy_applied ?? "",
    counterargument: "",
    why_not_others: buildDefaultRejections(portfolio.variants, selectedVariant),
    invariants: [""],
    pre_conditions: [""],
    post_conditions: [""],
    admissibility: [""],
    evidence_requirements: [""],
    rollback: {
      triggers: [""],
      steps: [""],
      blast_radius: "",
    },
    refresh_triggers: [""],
    weakest_link: selectedVariant?.weakest_link ?? "",
    valid_until: "",
    context: "",
    mode: portfolio.comparison ? "standard" : "tactical",
    affected_files: [""],
    predictions: [],
    search_keywords: "",
    first_module_coverage: false,
  };
}

function buildDefaultRejections(
  variants: VariantView[],
  selectedVariant?: VariantView,
): DecisionRejectionInput[] {
  const selectedTitle = selectedVariant?.title ?? "";

  const rejections = variants
    .filter((variant) => variant.title !== selectedTitle)
    .map((variant) => ({
      variant: variant.title,
      reason: `Did not beat ${selectedTitle || "the selected option"} under the current selection policy.`,
    }));

  return rejections.length > 0 ? rejections : [{ variant: "", reason: "" }];
}

function EditableRejections({
  values,
  onChange,
  variants,
}: {
  values: DecisionRejectionInput[];
  onChange: (value: DecisionRejectionInput[]) => void;
  variants: VariantView[];
}) {
  const entries = values.length > 0 ? values : [{ variant: "", reason: "" }];

  return (
    <div className="space-y-2">
      <div>
        <p className="text-xs uppercase tracking-[0.2em] text-text-muted">Rejected Alternatives</p>
        <p className="mt-1 text-xs text-text-muted">Each rejected variant needs a concrete losing reason.</p>
      </div>

      {entries.map((entry, index) => (
        <div key={`rejection-${index}`} className="grid gap-3 rounded-xl border border-border bg-surface-2/60 p-4 md:grid-cols-[220px_1fr_auto]">
          <select
            value={entry.variant}
            onChange={(event) =>
              onChange(
                entries.map((current, currentIndex) =>
                  currentIndex === index ? { ...current, variant: event.target.value } : current,
                ),
              )
            }
            className={inputClassName}
          >
            <option value="">Select variant</option>
            {variants.map((variant) => (
              <option key={variant.id} value={variant.title}>
                {variant.title}
              </option>
            ))}
          </select>

          <input
            value={entry.reason}
            onChange={(event) =>
              onChange(
                entries.map((current, currentIndex) =>
                  currentIndex === index ? { ...current, reason: event.target.value } : current,
                ),
              )
            }
            className={inputClassName}
            placeholder="Why this option lost"
          />

          <button
            type="button"
            onClick={() => onChange(entries.filter((_, currentIndex) => currentIndex !== index))}
            className="rounded-lg border border-border px-3 py-2 text-xs text-text-secondary transition-colors hover:bg-surface-2"
          >
            Remove
          </button>
        </div>
      ))}

      <button
        type="button"
        onClick={() => onChange([...entries, { variant: "", reason: "" }])}
        className="rounded-lg border border-dashed border-border px-3 py-2 text-xs text-text-secondary transition-colors hover:border-accent/50 hover:text-text-primary"
      >
        Add rejection
      </button>
    </div>
  );
}

function PredictionEditor({
  values,
  onChange,
}: {
  values: DecisionPredictionInput[];
  onChange: (value: DecisionPredictionInput[]) => void;
}) {
  return (
    <div className="space-y-2">
      <div>
        <p className="text-xs uppercase tracking-[0.2em] text-text-muted">Predictions</p>
        <p className="mt-1 text-xs text-text-muted">Deep decisions should state what will be measured later.</p>
      </div>

      {values.map((prediction, index) => (
        <div key={`prediction-${index}`} className="grid gap-3 rounded-xl border border-border bg-surface-2/60 p-4 md:grid-cols-2">
          <input
            value={prediction.claim}
            onChange={(event) => onChange(updatePrediction(values, index, "claim", event.target.value))}
            className={inputClassName}
            placeholder="Claim"
          />
          <input
            value={prediction.observable}
            onChange={(event) => onChange(updatePrediction(values, index, "observable", event.target.value))}
            className={inputClassName}
            placeholder="Observable"
          />
          <input
            value={prediction.threshold}
            onChange={(event) => onChange(updatePrediction(values, index, "threshold", event.target.value))}
            className={inputClassName}
            placeholder="Threshold"
          />
          <div className="flex items-center gap-2">
            <input
              value={prediction.verify_after}
              onChange={(event) => onChange(updatePrediction(values, index, "verify_after", event.target.value))}
              className={inputClassName}
              placeholder="Verify after"
            />
            <button
              type="button"
              onClick={() => onChange(values.filter((_, currentIndex) => currentIndex !== index))}
              className="rounded-lg border border-border px-3 py-2 text-xs text-text-secondary transition-colors hover:bg-surface-2"
            >
              Remove
            </button>
          </div>
        </div>
      ))}

      <button
        type="button"
        onClick={() =>
          onChange([
            ...values,
            { claim: "", observable: "", threshold: "", verify_after: "" },
          ])
        }
        className="rounded-lg border border-dashed border-border px-3 py-2 text-xs text-text-secondary transition-colors hover:border-accent/50 hover:text-text-primary"
      >
        Add prediction
      </button>
    </div>
  );
}

function updatePrediction(
  values: DecisionPredictionInput[],
  index: number,
  key: keyof DecisionPredictionInput,
  nextValue: string,
): DecisionPredictionInput[] {
  return values.map((value, currentIndex) =>
    currentIndex === index
      ? {
          ...value,
          [key]: nextValue,
        }
      : value,
  );
}

function StringListEditor({
  label,
  values,
  onChange,
}: {
  label: string;
  values: string[];
  onChange: (value: string[]) => void;
}) {
  const entries = values.length > 0 ? values : [""];

  return (
    <div className="space-y-2">
      <p className="text-xs uppercase tracking-[0.2em] text-text-muted">{label}</p>
      {entries.map((entry, index) => (
        <div key={`${label}-${index}`} className="flex items-center gap-2">
          <input
            value={entry}
            onChange={(event) =>
              onChange(
                entries.map((current, currentIndex) =>
                  currentIndex === index ? event.target.value : current,
                ),
              )
            }
            className={inputClassName}
            placeholder={`${label} ${index + 1}`}
          />
          <button
            type="button"
            onClick={() => onChange(entries.filter((_, currentIndex) => currentIndex !== index))}
            className="rounded-lg border border-border px-3 py-2 text-xs text-text-secondary transition-colors hover:bg-surface-2"
          >
            Remove
          </button>
        </div>
      ))}
      <button
        type="button"
        onClick={() => onChange([...entries, ""])}
        className="rounded-lg border border-dashed border-border px-3 py-2 text-xs text-text-secondary transition-colors hover:border-accent/50 hover:text-text-primary"
      >
        Add {label.toLowerCase().slice(0, -1)}
      </button>
    </div>
  );
}

function Field({
  label,
  children,
  required,
}: {
  label: string;
  children: React.ReactNode;
  required?: boolean;
}) {
  return (
    <label className="block space-y-1.5">
      <span className="text-xs uppercase tracking-[0.2em] text-text-muted">
        {label}
        {required ? " *" : ""}
      </span>
      {children}
    </label>
  );
}

const inputClassName =
  "w-full rounded-xl border border-border bg-surface-2 px-3 py-2 text-sm text-text-primary outline-none transition-colors focus:border-accent/60";
