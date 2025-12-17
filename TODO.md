# Architectural Debt & Future Improvements (FPF Alignment)

## 1. C.16 MM-CHR: Characteristic Space for Decisions
**Current State:** The "Decision" phase relies on free-text rationale.
**Improvement:** Introduce a **Characteristic Space** (e.g., Latency, Cost, Reliability). 
- `quint_decide` should accept a `justification_matrix` comparing L2 options against these metrics.
- This creates auditable trade-offs instead of just narrative.

## 2. E.9 Design Rationale Record (DRR): Structure Enforcement
**Current State:** `quint_decide` accepts a blob of `content`.
**Improvement:** Enforce the 4-part DRR structure strictly within the tool schema:
- **Context:** Why are we deciding this?
- **Decision:** What is the selected option?
- **Rationale:** Why this option over others? (Link to Characteristic Space)
- **Consequences:** What happens next? (Risks, Tech Debt)

## 3. C.3 Kind-CAL: Typed Reasoning for Hypotheses
**Current State:** All artifacts are generic Markdown.
**Improvement:** Differentiate between `U.System` hypotheses (architectural changes) and `U.Episteme` hypotheses (documentation/theory changes).
- Allows the **Deductor** to apply different logic rules (e.g., checking compiler compliance vs. checking logical consistency).
