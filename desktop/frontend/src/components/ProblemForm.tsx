import { useState } from "react";

import { type ProblemCreateInput } from "../lib/api";

interface ProblemFormProps {
  initialValue?: ProblemCreateInput;
  onSubmit: (value: ProblemCreateInput) => Promise<void> | void;
  onCancel?: () => void;
  submitting: boolean;
}

export function ProblemForm({
  initialValue,
  onSubmit,
  onCancel,
  submitting,
}: ProblemFormProps) {
  const [value, setValue] = useState<ProblemCreateInput>(() => initialValue ?? emptyProblemInput());

  const submit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    await onSubmit({
      ...value,
      title: value.title.trim(),
      signal: value.signal.trim(),
      acceptance: value.acceptance.trim(),
      blast_radius: value.blast_radius.trim(),
      reversibility: value.reversibility.trim(),
      context: value.context.trim(),
      mode: value.mode.trim(),
    });
  };

  return (
    <form onSubmit={submit} className="space-y-5 rounded-2xl border border-border bg-surface-1 p-5">
      <div className="flex items-start justify-between gap-4">
        <div>
          <p className="text-xs uppercase tracking-[0.22em] text-text-muted">Frame Problem</p>
          <h3 className="mt-1 text-lg font-semibold text-text-primary">Capture the signal before solving it</h3>
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
        <Field label="Title" required>
          <input
            value={value.title}
            onChange={(event) => setValue({ ...value, title: event.target.value })}
            className={inputClassName}
            placeholder="Reasoning authoring gap"
          />
        </Field>

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
      </div>

      <Field label="Signal" required>
        <textarea
          value={value.signal}
          onChange={(event) => setValue({ ...value, signal: event.target.value })}
          className={`${inputClassName} min-h-24`}
          placeholder="What observation does not fit?"
        />
      </Field>

      <div className="grid gap-4 md:grid-cols-2">
        <Field label="Acceptance">
          <textarea
            value={value.acceptance}
            onChange={(event) => setValue({ ...value, acceptance: event.target.value })}
            className={`${inputClassName} min-h-24`}
            placeholder="How will we know this is solved?"
          />
        </Field>

        <Field label="Blast Radius">
          <textarea
            value={value.blast_radius}
            onChange={(event) => setValue({ ...value, blast_radius: event.target.value })}
            className={`${inputClassName} min-h-24`}
            placeholder="Who or what is affected?"
          />
        </Field>
      </div>

      <div className="grid gap-4 md:grid-cols-2">
        <Field label="Reversibility">
          <input
            value={value.reversibility}
            onChange={(event) => setValue({ ...value, reversibility: event.target.value })}
            className={inputClassName}
            placeholder="low | medium | high"
          />
        </Field>

        <Field label="Context">
          <input
            value={value.context}
            onChange={(event) => setValue({ ...value, context: event.target.value })}
            className={inputClassName}
            placeholder="desktop-authoring"
          />
        </Field>
      </div>

      <StringListEditor
        label="Constraints"
        description="Hard limits that variants must respect."
        values={value.constraints}
        onChange={(constraints) => setValue({ ...value, constraints })}
      />

      <StringListEditor
        label="Optimization Targets"
        description="One to three outcomes worth improving."
        values={value.optimization_targets}
        onChange={(optimization_targets) => setValue({ ...value, optimization_targets })}
      />

      <StringListEditor
        label="Observation Indicators"
        description="Signals to watch without optimizing for them."
        values={value.observation_indicators}
        onChange={(observation_indicators) => setValue({ ...value, observation_indicators })}
      />

      <div className="flex items-center justify-between gap-3 rounded-xl border border-border bg-surface-2/60 px-4 py-3">
        <p className="text-xs text-text-muted">
          Characterization lives in the next step. Frame the problem first, then attach dimensions and parity rules.
        </p>
        <button
          type="submit"
          disabled={submitting}
          className="rounded-lg bg-accent px-4 py-2 text-sm text-white transition-colors hover:bg-accent-hover disabled:opacity-50"
        >
          {submitting ? "Saving..." : "Create Problem"}
        </button>
      </div>
    </form>
  );
}

function emptyProblemInput(): ProblemCreateInput {
  return {
    title: "",
    signal: "",
    acceptance: "",
    blast_radius: "",
    reversibility: "medium",
    context: "",
    mode: "standard",
    constraints: [""],
    optimization_targets: [""],
    observation_indicators: [""],
  };
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

function StringListEditor({
  label,
  description,
  values,
  onChange,
}: {
  label: string;
  description: string;
  values: string[];
  onChange: (values: string[]) => void;
}) {
  const nextValues = values.length > 0 ? values : [""];

  return (
    <div className="space-y-2">
      <div>
        <p className="text-xs uppercase tracking-[0.2em] text-text-muted">{label}</p>
        <p className="mt-1 text-xs text-text-muted">{description}</p>
      </div>

      <div className="space-y-2">
        {nextValues.map((entry, index) => (
          <div key={`${label}-${index}`} className="flex items-center gap-2">
            <input
              value={entry}
              onChange={(event) => {
                const updated = [...nextValues];
                updated[index] = event.target.value;
                onChange(updated);
              }}
              className={inputClassName}
              placeholder={`${label} ${index + 1}`}
            />
            <button
              type="button"
              onClick={() => onChange(nextValues.filter((_, currentIndex) => currentIndex !== index))}
              className="rounded-lg border border-border px-3 py-2 text-xs text-text-secondary transition-colors hover:bg-surface-2"
            >
              Remove
            </button>
          </div>
        ))}
      </div>

      <button
        type="button"
        onClick={() => onChange([...nextValues, ""])}
        className="rounded-lg border border-dashed border-border px-3 py-2 text-xs text-text-secondary transition-colors hover:border-accent/50 hover:text-text-primary"
      >
        Add {label.toLowerCase().slice(0, -1)}
      </button>
    </div>
  );
}

const inputClassName =
  "w-full rounded-xl border border-border bg-surface-2 px-3 py-2 text-sm text-text-primary outline-none transition-colors focus:border-accent/60";
