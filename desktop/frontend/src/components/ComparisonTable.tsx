import type { ComparisonView, PortfolioDetail } from "../lib/api";
import { Badge, Eyebrow, Pill } from "./primitives";

export interface ComparisonTableProps {
  comparison: NonNullable<PortfolioDetail["comparison"]>;
  className?: string;
}

/**
 * Comparison view — characteristic × variant grid with Pareto-front
 * highlighting and a recommendation banner. Replaces the legacy inline
 * table that lived in Portfolios.tsx.
 *
 * Visual contract follows haft-design-system/project/ui_kits/haft-tauri/
 * Comparison.jsx:
 * - Border-first grid (CSS grid, not <table>) so cell borders align with
 *   the kit's surface-2 header row.
 * - Variants in `non_dominated_set` are coloured by accent in their column
 *   header. The selected variant gets the strongest accent treatment.
 * - Per-cell accent-wash background marks "this variant is on the
 *   non-dominated set" for that row's dimension — making it scannable
 *   which variant survives the parity comparison.
 * - Recommendation banner uses the standard Badge primitive with eyebrow
 *   case ("Recommendation") and renders the policy + commentary below.
 */
export function ComparisonTable({
  comparison,
  className = "",
}: ComparisonTableProps) {
  const variants = Object.keys(comparison.scores);
  const nonDominated = new Set(comparison.non_dominated_set);
  const dimensions = comparison.dimensions;
  const gridColumns = `minmax(160px, 1.4fr) ${variants.map(() => "1fr").join(" ")}`;

  const variantTone = (variant: string): "selected" | "front" | "muted" => {
    if (comparison.selected_ref === variant) return "selected";
    if (nonDominated.has(variant)) return "front";
    return "muted";
  };

  return (
    <div className={`space-y-3 ${className}`}>
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-medium text-text-secondary">
          Compare / Pareto
        </h3>
        {comparison.policy_applied ? (
          <Pill>{comparison.policy_applied}</Pill>
        ) : null}
      </div>

      <div
        className="grid overflow-hidden rounded-xl border border-border bg-surface-1"
        style={{ gridTemplateColumns: gridColumns }}
      >
        <HeaderCell isFirst>
          <Eyebrow>Dimension</Eyebrow>
        </HeaderCell>
        {variants.map((variant) => (
          <HeaderCell key={`hdr-${variant}`}>
            <Eyebrow>Variant</Eyebrow>
            <div
              className={`mt-0.5 text-[13px] font-medium ${
                variantTone(variant) === "selected"
                  ? "text-accent"
                  : variantTone(variant) === "front"
                    ? "text-success"
                    : "text-text-primary"
              }`}
            >
              {variant}
            </div>
          </HeaderCell>
        ))}
        {dimensions.map((dimension) => (
          <DimensionRow
            key={dimension}
            dimension={dimension}
            variants={variants}
            scores={comparison.scores}
            nonDominated={nonDominated}
            selectedRef={comparison.selected_ref}
          />
        ))}
      </div>

      {comparison.non_dominated_set.length > 0 ? (
        <div className="rounded-xl border border-success/20 bg-success/10 px-4 py-3 text-sm text-success">
          Computed Pareto front: {comparison.non_dominated_set.join(", ")}
        </div>
      ) : (
        <div className="rounded-xl border border-border bg-surface-1 px-4 py-3 text-sm text-text-muted">
          No Pareto front computed yet.
        </div>
      )}

      {comparison.recommendation ? (
        <div className="rounded-xl border border-accent-border bg-accent-wash px-4 py-3.5">
          <div className="mb-1 flex items-center gap-2.5">
            <Badge tone="accent">Recommendation</Badge>
            {comparison.selected_ref ? (
              <span className="text-[11px] text-text-muted">
                selected: {comparison.selected_ref}
              </span>
            ) : null}
          </div>
          <p className="text-[13px] text-text-primary">{comparison.recommendation}</p>
        </div>
      ) : null}
    </div>
  );
}

interface HeaderCellProps {
  isFirst?: boolean;
  children: React.ReactNode;
}

function HeaderCell({ isFirst = false, children }: HeaderCellProps) {
  return (
    <div
      className={`bg-surface-2 px-4 py-3.5 ${
        isFirst ? "" : "border-l border-border"
      } border-b border-border`}
    >
      {children}
    </div>
  );
}

interface DimensionRowProps {
  dimension: string;
  variants: string[];
  scores: ComparisonView["scores"];
  nonDominated: Set<string>;
  selectedRef: string;
}

function DimensionRow({
  dimension,
  variants,
  scores,
  nonDominated,
  selectedRef,
}: DimensionRowProps) {
  return (
    <>
      <div className="border-t border-border px-4 py-3.5 text-[13px] text-text-secondary">
        {dimension}
      </div>
      {variants.map((variant) => {
        const value = scores[variant]?.[dimension] ?? "—";
        const isSelected = selectedRef === variant;
        const isFront = nonDominated.has(variant);
        const cellTone = isSelected
          ? "bg-accent-wash text-accent"
          : isFront
            ? "bg-surface-2/40 text-text-primary"
            : "text-text-primary";
        return (
          <div
            key={`${variant}-${dimension}`}
            className={`border-l border-t border-border px-4 py-3.5 text-[13px] ${cellTone}`}
          >
            {value}
          </div>
        );
      })}
    </>
  );
}
