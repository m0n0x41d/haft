---
description: "Reconcile the project's FPF state with recent repository changes."
---

# Actualize Knowledge Base

This command is a core part of maintaining a living assurance case. It helps you keep your FPF knowledge base (`.quint/`) in sync with the evolving reality of your project's codebase.

The command performs a three-part audit against recent git changes to surface potential context drift, stale evidence, and outdated decisions. This aligns with the **Observe** phase of the FPF Canonical Evolution Loop (B.4) and helps manage **Epistemic Debt** (B.3.4).

The LLM persona for this command is the **Actualizer**.

## Instruction

1.  **Execute Actualization:**
    -   The Actualizer **MUST** first execute the `quint_actualize` tool.
    -   This tool will automatically:
        -   Identify the baseline commit from the FPF state.
        -   Perform any necessary legacy migrations.
        -   Generate a report of all file changes since the last actualization.
        -   Update the FPF state baseline to the current `HEAD`.

2.  **Analyze Report for Context Drift:**
    -   Review the `quint_actualize` report for changes to core project configuration files (e.g., `package.json`, `go.mod`, `Dockerfile`, `pom.xml`).
    -   If these files have changed, re-run the context analysis logic from `/q0-init` to generate a "current context" summary.
    -   Present a diff between the detected current context and the contents of `.quint/context.md`.
    -   Ask the user if they want to update the `context.md` file.

3.  **Analyze Report for Evidence Staleness (Epistemic Debt):**
    -   Cross-reference the list of changed files from the report against the `carrier_ref` of all evidence files in `.quint/evidence/`.
    -   If a referenced file is in the changed list, flag the evidence as **stale**.
    -   Compile all stale evidence into a "Stale Evidence Report," noting which hypotheses or decisions are affected.

4.  **Analyze Report for Decision Relevance:**
    -   Trace the justification of all decision records (`DRR*` in `.quint/decisions/`) back to their source evidence and carrier files.
    -   If any foundational source files appear in the change report, flag the decision record as **"Potentially Outdated"**.
    -   Compile these into a "Decisions to Review" report.

5.  **Present Findings:**
    -   Summarize the analysis in a clear, actionable report:
        -   **Context Drift:** (if any) Diff and prompt for update.
        -   **Stale Evidence:** List of evidence needing re-validation via `/q3-validate`.
        -   **Decisions to Review:** List of decisions needing re-evaluation via `/q1-hypothesize`.
