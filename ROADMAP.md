# Quint Code Roadmap

This document outlines the future development focus for `quint-code`, prioritizing improvements that deepen its adherence to the First Principles Framework (FPF) and enhance its robustness and utility.

## Future Focus: Deepening FPF Integration

The following items are considered high-priority for moving `quint-code` towards a more complete and rigorous implementation of the FPF calculus.

### 1. Complete the F-G-R Assurance Calculus

**Status:** Currently, the assurance calculator focuses primarily on **Reliability (R)**.
**Next Step:** Implement **Formality (F)** and **ClaimScope (G)**.

-   **Formality (F) - The "How Strictly"**:
    -   **Why:** To distinguish between an informal idea and a formally specified, verifiable claim (Pattern C.2.3). This allows the system to reason about the rigor of a hypothesis.
    -   **Implementation:**
        -   Add a `formality: F[0-9]` field to hypothesis frontmatter.
        -   Update the assurance calculator to propagate `F` via the **weakest-link principle** (`min(F)`).
        -   Display `F` in the `/q-audit` visualization.

-   **ClaimScope (G) - The "Where"**:
    -   **Why:** To define the context and boundaries where a claim is valid (Pattern A.2.6). This prevents over-generalization and ensures decisions are applied only where they are supported by evidence.
    -   **Implementation:**
        -   Enhance the `scope:` field in hypotheses to support structured `U.ContextSlice` definitions.
        -   Update the assurance calculator to propagate `G` via **intersection** for serial dependencies and **SpanUnion** for parallel ones.
        -   Display `G` in the `/q-audit` visualization.

### 2. Implement Normative Congruence Penalty (Φ(CL))

**Status:** The `CL` penalty is currently a linear `1.0 - cl`.
**Next Step:** Implement the non-linear penalty function from the FPF specification.

-   **Why:** To more accurately model the trust decay from integrating poorly-aligned evidence (Pattern B.3). A low-congruence link should be more punishing than a medium-congruence one.
-   **Implementation:**
    -   Update the `calculateCLPenalty` function in `calculator.go` to use the table-based penalty function `Φ(CL)` from FPF pattern B.1.3 (e.g., CL2=0.5, CL1=1.0).

### 3. Enhance DRR Generation

**Status:** The `/q5-decide` command creates a basic DRR.
**Next Step:** Make the DRR a comprehensive, self-contained audit artifact.

-   **Why:** To fully realize the goal of a durable, auditable decision record (Pattern E.9).
-   **Implementation:**
    -   Embed the complete audit tree from `/q-audit` (with the full `⟨F, G, R⟩` tuples) directly into the DRR markdown file.
    -   Include a summary of the final assurance scores and any waivers or accepted risks.

### 4. Enhance q-actualize Command

**Status:** The command has been redesigned conceptually but requires implementation in the `mcp` binary.
**Next Step:** Implement the new functionality for `/q-actualize`.

-   **Why:** To actively reconcile the FPF knowledge base with the evolving codebase, identifying context drift, stale evidence, and outdated decisions (Pattern B.4 Observe Phase, B.3.4 Epistemic Debt). This transforms `/q-actualize` into a crucial tool for maintaining a living assurance case.
-   **Implementation:**
    -   Implement `git` integration for detecting changed files.
    -   Develop logic for analyzing context drift by comparing current project config files with `.quint/context.md`.
    -   Create functions to parse evidence (`carrier_ref`) and decision files to track dependencies.
    -   Implement recursive traversal (leveraging `assurance/calculator.go`'s logic) to identify stale evidence and outdated decisions.
    -   Develop a structured output for the actualization report.
## Housekeeping & Future Hardening

- **Logging:** Refine the `stderr` warnings into a more structured logging system (e.g., with levels like `INFO`, `WARN`, `ERROR`).
- **Dependency Management:** Continue to audit and verify third-party dependencies.
- **Broader FPF Concepts:** Longer-term, the engine could be expanded to model more advanced FPF concepts like:
    - **`Γ` (Universal Algebra of Aggregation):** Modeling how systems and epistemes are composed.
    - **Meta-Holon Transition (MHT):** Modeling the emergence of new, coherent wholes.

### Minor: Improve Installation Process

- **Self-Bootstrapping Binary:** Replace shell-based installer with a Go binary that embeds all assets and can bootstrap/update itself. See `tmp/bootstrap.md` for details.
    - Embed commands via `//go:embed`
    - `quint-mcp bootstrap` command for installation
    - `quint-mcp update` for self-updates with checksum verification
    - Eliminates shell script quirks and external dependencies (curl, tar, python3)
