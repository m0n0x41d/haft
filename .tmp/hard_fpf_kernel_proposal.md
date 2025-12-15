# Proposal: Quint CLI - The Hard FPF Kernel

## Executive Summary

This document proposes building a dedicated Command Line Interface (CLI), tentatively named `quint`, to act as a **Hard FPF Kernel**. This CLI will serve as the authoritative supervisor for the First Principles Framework (FPF) reasoning cycle, enforcing FPF's strict invariants and phase transitions. This shifts control from the AI (which is prone to "premature convergence") to deterministic code, transforming FPF from a suggested methodology into an enforced "physics" of decision-making.

## 1. The Core Problem: AI as Supervisor vs. AI as Subsystem

The current "Quint Code" implementation, while effective at prompting, suffers from a fundamental limitation: the AI (LLM) acts as the supervisor of the FPF cycle. This leads to what we've termed the **"Premature Convergence"** problem, where the AI, being probabilistic and eager to provide a solution, may:
*   Violate the ADI cycle's sequential order.
*   Advance phases incorrectly (e.g., marking INDUCTION_COMPLETE after testing only one hypothesis out of four).
*   Perform arithmetic or logical checks unreliably.

This is because the AI owns the state (by writing to `session.md`), and thus owns the truth of the FPF cycle's progression.

## 2. The Solution: Quint CLI - The FPF Native Client

The proposed `quint` CLI will be a standalone application designed to:
*   **Be the Supervisor:** It will strictly manage the FPF state machine.
*   **Enforce Invariants:** All FPF rules (WLNK, MONO, ADI cycle) will be implemented in code.
*   **Orchestrate AI Interaction:** It will act as a wrapper around LLM APIs, controlling the context, available tools, and allowed actions for the AI at each phase.

## 3. Architectural Blueprint: "The Constraint Engine"

### Core Components:

*   **FPF Kernel (`quint` CLI):** A Python/Rust/Go application that
    *   Manages the FPF state (in `local_state.json`).
    *   Implements FPF invariants and phase transition logic.
    *   Provides atomic FPF operations (e.g., `quint record hypothesis`, `quint add-evidence`).
    *   Acts as a wrapper for LLM API calls.
*   **The Brain (LLM API):** Anthropic (Claude) or OpenAI (GPT). This performs the creative generation, research, and analysis tasks within the constraints set by the `quint` CLI.
*   **The State (Local Database):** A structured file (e.g., `.fpf/state.json` or SQLite) that `quint` owns and manages. The AI **cannot edit this file directly**.

### Workflow: "AI as Worker, Code as Boss"

1.  **Initialization:** `quint init` sets up the project (`.fpf/state.json`, `.fpf/config.json`).
2.  **Phase Transition Logic (Hardcoded):**
    *   The `quint` CLI **computes** the current FPF phase based on the content of `.fpf/state.json`.
    *   It exposes only the tools (API calls or internal commands) that are valid for the *current computed phase*.
3.  **Dynamic System Prompt Generation:**
    *   `quint` reads the FPF state.
    *   It constructs a tailored System Prompt for the LLM that clearly defines the AI's role for the current phase, available tools, and specific constraints.
    *   It injects relevant FPF data (e.g., specific hypothesis text, current WLNK score) directly into the prompt, avoiding the need for the AI to "remember" long contexts.
4.  **Tool Interception & Validation:**
    *   When the LLM suggests a tool call (e.g., "call `quint record-evidence`"), `quint` intercepts it.
    *   It **validates** the proposed tool call against the hardcoded FPF rules for the current phase.
        *   *Example:* If the AI tries to `quint decide` in the `INDUCTION` phase, but not all hypotheses are resolved (verified or falsified), `quint` rejects the call with a hard error message: "Error: Premature Convergence. All hypotheses must be verified or falsified before deciding."
5.  **State Updates:**
    *   Valid tool calls are executed by `quint`.
    *   `quint` updates its internal state (`.fpf/state.json`) deterministically.

### 4. Solving the "Premature Convergence" Problem

This architecture **prevents the AI from violating the ADI cycle** by:

*   **CLI as State Owner:** The AI is removed from direct control over the FPF cycle's state. `quint` is the sole manager of `.fpf/state.json`.
*   **Hard-Coded Gates:** Phase transitions are *calculated* by `quint` based on the status of hypotheses and evidence, not *declared* by the AI.
*   **Dynamic Tool Exposure:** The AI is only ever given access to tools (via the LLM API's tool-use mechanism) that are valid for the current FPF phase.

### 5. Authentication & LLM Backend

*   **Primary Backend:** Support for Anthropic's Claude API (as requested by the user's context in `quint-code`'s `README.md`).
*   **API Key Authorization:** The `quint` CLI will configure calls using standard API keys, allowing users to leverage their Pro/Max subscriptions for robust, high-volume interaction. This is the most reliable and stateless method.
*   **OpenCode/Cursor Style (Session Token):** While possible (reverse-engineering web sessions), it's generally brittle and not recommended for a robust FPF Kernel.
*   **BYO-Provider:** The CLI can be designed with a pluggable LLM backend, allowing users to configure OpenAI, local LLMs, or other providers.

### 6. Why This Is a "Banger" Idea

*   **Enforced Rigor:** FPF becomes a "physics" that cannot be ignored. The system literally prevents premature decisions or logic violations.
*   **True Memory Management:** `quint` manages the memory of the FPF cycle. The AI doesn't need to "remember" old context; `quint` injects the *relevant* context slice for the current phase, making interaction cost-effective and reliable.
*   **Scalable Trust:** FPF decision artifacts (hypotheses, evidence, DRRs) are stored in a structured, machine-readable format, enabling automatic audit and long-term knowledge management.
*   **Cost Efficiency:** By managing the context window effectively, `quint` can reduce token usage, leading to more economical LLM interactions.
*   **Clear Roles:** The user remains the ultimate decision-maker, `quint` enforces the FPF methodology, and the LLM acts as the powerful "worker" within those constraints.

### 7. Next Steps

1.  **Define the State Schema:** Design the JSON schema for `.fpf/state.json` to rigorously define `Hypothesis` objects (ID, status, evidence links), `Evidence` (source, CL, valid_until), and `Decision` objects.
2.  **Outline Core Commands:** Define the CLI commands (`quint init`, `quint hypothesize`, `quint add-evidence`, `quint check-logic`, `quint run-test`, `quint audit`, `quint decide`) and their phase-specific behaviors.
3.  **Develop Phase Transition Logic:** Map out the exact rules for how `quint` computes the current phase based on the state.
4.  **Integrate LLM API:** Set up the connection to the chosen LLM provider.

This dedicated CLI will be the true embodiment of a "Hard FPF Kernel," providing an unprecedented level of rigor and trustworthiness to AI-assisted decision-making.
