import {
  type PortfolioVariantInput,
} from "../lib/api";

interface VariantFormProps {
  value: PortfolioVariantInput[];
  onChange: (value: PortfolioVariantInput[]) => void;
}

export function VariantForm({ value, onChange }: VariantFormProps) {
  const variants = value.length > 0 ? value : [emptyVariant(0), emptyVariant(1)];

  return (
    <div className="space-y-4">
      {variants.map((variant, index) => (
        <div key={`${variant.id || "variant"}-${index}`} className="rounded-2xl border border-border bg-surface-1 p-4">
          <div className="flex items-start justify-between gap-3">
            <div>
              <p className="text-xs uppercase tracking-[0.22em] text-text-muted">Variant {index + 1}</p>
              <h4 className="mt-1 text-sm font-semibold text-text-primary">
                {variant.title || "Untitled variant"}
              </h4>
            </div>
            <button
              type="button"
              onClick={() => onChange(variants.filter((_, currentIndex) => currentIndex !== index))}
              className="rounded-lg border border-border px-3 py-1.5 text-xs text-text-secondary transition-colors hover:bg-surface-2"
            >
              Remove
            </button>
          </div>

          <div className="mt-4 grid gap-3 md:grid-cols-2">
            <LabeledField label="Title">
              <input
                value={variant.title}
                onChange={(event) => onChange(updateVariant(variants, index, "title", event.target.value))}
                className={inputClassName}
                placeholder="Inline modal authoring"
              />
            </LabeledField>

            <LabeledField label="Weakest Link">
              <input
                value={variant.weakest_link}
                onChange={(event) => onChange(updateVariant(variants, index, "weakest_link", event.target.value))}
                className={inputClassName}
                placeholder="Dense forms can become heavy"
              />
            </LabeledField>

            <LabeledField label="Novelty Marker">
              <input
                value={variant.novelty_marker}
                onChange={(event) => onChange(updateVariant(variants, index, "novelty_marker", event.target.value))}
                className={inputClassName}
                placeholder="Preserves the current shell"
              />
            </LabeledField>

            <LabeledField label="Diversity Role">
              <input
                value={variant.diversity_role}
                onChange={(event) => onChange(updateVariant(variants, index, "diversity_role", event.target.value))}
                className={inputClassName}
                placeholder="low-blast-radius baseline"
              />
            </LabeledField>
          </div>

          <LabeledField label="Description">
            <textarea
              value={variant.description}
              onChange={(event) => onChange(updateVariant(variants, index, "description", event.target.value))}
              className={`${inputClassName} mt-1 min-h-20`}
              placeholder="What this option does"
            />
          </LabeledField>

          <div className="mt-4 grid gap-4 md:grid-cols-2">
            <StringListEditor
              label="Strengths"
              values={variant.strengths}
              onChange={(strengths) => onChange(updateVariantList(variants, index, "strengths", strengths))}
            />

            <StringListEditor
              label="Risks"
              values={variant.risks}
              onChange={(risks) => onChange(updateVariantList(variants, index, "risks", risks))}
            />
          </div>

          <div className="mt-4 rounded-xl border border-border bg-surface-2/60 p-4">
            <label className="flex items-center gap-3 text-sm text-text-primary">
              <input
                type="checkbox"
                checked={variant.stepping_stone}
                onChange={(event) => onChange(updateVariantBoolean(variants, index, "stepping_stone", event.target.checked))}
                className="h-4 w-4 rounded border-border bg-surface-2"
              />
              Stepping stone
            </label>

            {variant.stepping_stone && (
              <LabeledField label="Stepping Stone Basis">
                <textarea
                  value={variant.stepping_stone_basis}
                  onChange={(event) =>
                    onChange(updateVariant(variants, index, "stepping_stone_basis", event.target.value))
                  }
                  className={`${inputClassName} mt-2 min-h-20`}
                  placeholder="Why this option opens future moves even if it is not the end state."
                />
              </LabeledField>
            )}
          </div>
        </div>
      ))}

      <button
        type="button"
        onClick={() => onChange([...variants, emptyVariant(variants.length)])}
        className="rounded-lg border border-dashed border-border px-3 py-2 text-xs text-text-secondary transition-colors hover:border-accent/50 hover:text-text-primary"
      >
        Add variant
      </button>
    </div>
  );
}

function updateVariant(
  variants: PortfolioVariantInput[],
  index: number,
  key: keyof PortfolioVariantInput,
  value: string,
): PortfolioVariantInput[] {
  return variants.map((variant, currentIndex) =>
    currentIndex === index
      ? {
          ...variant,
          [key]: value,
        }
      : variant,
  );
}

function updateVariantList(
  variants: PortfolioVariantInput[],
  index: number,
  key: "strengths" | "risks",
  value: string[],
): PortfolioVariantInput[] {
  return variants.map((variant, currentIndex) =>
    currentIndex === index
      ? {
          ...variant,
          [key]: value,
        }
      : variant,
  );
}

function updateVariantBoolean(
  variants: PortfolioVariantInput[],
  index: number,
  key: "stepping_stone",
  value: boolean,
): PortfolioVariantInput[] {
  return variants.map((variant, currentIndex) =>
    currentIndex === index
      ? {
          ...variant,
          [key]: value,
        }
      : variant,
  );
}

function emptyVariant(index: number): PortfolioVariantInput {
  return {
    id: `var-${index + 1}`,
    title: "",
    description: "",
    strengths: [""],
    weakest_link: "",
    novelty_marker: "",
    risks: [""],
    stepping_stone: index === 0,
    stepping_stone_basis: "",
    diversity_role: "",
    assumption_notes: "",
    rollback_notes: "",
    evidence_refs: [],
  };
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

function StringListEditor({
  label,
  values,
  onChange,
}: {
  label: string;
  values: string[];
  onChange: (values: string[]) => void;
}) {
  const entries = values.length > 0 ? values : [""];

  return (
    <div className="space-y-2">
      <p className="text-xs uppercase tracking-[0.2em] text-text-muted">{label}</p>
      {entries.map((entry, index) => (
        <div key={`${label}-${index}`} className="flex items-center gap-2">
          <input
            value={entry}
            onChange={(event) => {
              const next = [...entries];
              next[index] = event.target.value;
              onChange(next);
            }}
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

const inputClassName =
  "w-full rounded-xl border border-border bg-surface-2 px-3 py-2 text-sm text-text-primary outline-none transition-colors focus:border-accent/60";
