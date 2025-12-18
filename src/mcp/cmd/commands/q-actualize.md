---
description: "Reconcile the project's FPF state with recent repository changes."
---

# Actualize Knowledge Base

This command is a core part of maintaining a living assurance case. It helps you keep your FPF knowledge base (`.quint/`) in sync with the evolving reality of your project's codebase.

The command performs a three-part audit against recent git changes to surface potential context drift, stale evidence, and outdated decisions. This aligns with the **Observe** phase of the FPF Canonical Evolution Loop (B.4) and helps manage **Epistemic Debt** (B.3.4).

The LLM persona for this command is the **Actualizer**.

## Instruction

1.  **Identify Baseline for Changes:**
    -   The Actualizer will first determine the set of recent changes to analyze. It will look for a baseline commit hash in `.quint/state/actualize.log`.
    -   If the file doesn't exist, it will ask you to choose a baseline: the latest git tag, a specific commit hash, or a time window (e.g., "last 7 days").
    -   It will then generate a list of all files that have changed between the baseline and the current `HEAD`.

2.  **Analyze Context Drift:**
    -   The Actualizer will check if any core project configuration files (e.g., `package.json`, `go.mod`, `Dockerfile`, `pom.xml`) are in the list of changed files.
    -   If they are, it will re-run the context analysis logic from `/q0-init` to generate a "current context" summary.
    -   It will then present a diff between the detected current context and the contents of `.quint/context.md`.
    -   It will ask you if you want to update the `context.md` file.

3.  **Analyze Evidence Staleness (Epistemic Debt):**
    -   The Actualizer will scan all evidence files in `.quint/evidence/`.
    -   For each piece of evidence that has a `carrier_ref` pointing to a file, it will check if that file has been modified in the recent git changes.
    -   If a referenced file has changed, the evidence will be flagged as **stale**.
    -   All stale evidence will be compiled into a "Stale Evidence Report," noting which hypotheses or decisions are affected by the potentially decayed evidence.

4.  **Analyze Decision Relevance:**
    -   The Actualizer will examine all decision records (`DRR*`) in `.quint/decisions/`.
    -   For each decision, it will trace its justification back through its supporting evidence to the original source files (`carrier_ref`).
    -   If any of these foundational source files have changed, the decision record will be flagged as **"Potentially Outdated"**.
    -   All such decisions will be compiled into a "Decisions to Review" report.

5.  **Present the Actualization Report:**
    -   The Actualizer will summarize all findings for you in a clear, actionable report with three sections:
        -   **Context Drift:** (if any) Shows a diff of `context.md` and prompts for action.
        -   **Stale Evidence:** Lists all evidence files that need re-validation, suggesting the use of `/q3-validate`.
        -   **Decisions to Review:** Lists all decisions that may no longer be valid, suggesting a new reasoning cycle (`/q1-hypothesize`) to re-evaluate them.

6.  **Update Baseline:**
    -   After you have reviewed the report, the Actualizer will ask for confirmation to update the baseline.
    -   Upon confirmation, it will execute a tool call to write the current `HEAD` commit hash into `.quint/state/actualize.log`, setting the baseline for the next run.
