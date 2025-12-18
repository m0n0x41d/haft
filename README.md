<img src="assets/banner.svg" alt="Quint Code" width="600">

**Structured reasoning for AI coding tools** — make better decisions, remember why you made them.

**Supports:** Claude Code, Cursor, Gemini CLI, Codex CLI

> **Works exceptionally well with Claude Code!**

## The Problem This Solves

You're deep in a codebase. You need to handle cross-service transactions in a system that's grown beyond the point where distributed transactions are viable. Event choreography? Saga pattern? Outbox with CDC? Each has non-obvious failure modes.

Your AI tool gives you *an* answer. It's coherent. But:

- **Why** this approach? (You won't remember the reasoning in 3 months)
- **What alternatives** were considered? (Were there alternatives, or did you anchor on the first idea?)
- **What evidence** supported it? (Documentation? Benchmarks? Or just pattern-matching from training data?)
- **When does this decision expire?** (The tradeoffs shift as the system evolves)

FPF gives you a structured way to think through these decisions. You generate hypotheses, verify them, test them, and document *why* you chose what you chose.

## Quick Start

### Install (per-project)

Quint Code is installed **per-project** by design. Each project maintains its own knowledge base, evidence, and decision history in `.quint/`. This ensures context isolation — decisions and reasoning are bound to the codebase they belong to.

```bash
cd /path/to/your/project
curl -fsSL https://raw.githubusercontent.com/m0n0x41d/quint-code/main/install.sh | bash
```

The installer will:
1. Create `.quint/` directory structure
2. Install the MCP server binary
3. Configure `.mcp.json` for your AI tool
4. Copy slash commands to your selected tools (Claude Code, Cursor, Gemini CLI)

### Initialize

```bash
# In your AI coding tool:
/q0-init  # Scans context and initializes knowledge base

# Start reasoning
/q1-hypothesize "How should we handle state synchronization across browser tabs?"
```

## How It Works: The FPF Engine

Quint Code implements the **First Principles Framework (FPF)** as a set of **Commands** (Methods) that drive **Tools** (Work).

### Concepts: Agents vs. Personas

In FPF terms, an **Agent** is simply a system playing a specific **Role**. Quint Code operationalizes this as **Personas**:

- **No Invisible Threads:** Unlike "autonomous agents" that run in the background, Quint Code Personas (e.g., *Abductor*, *Auditor*) run entirely within your visible chat thread.
- **You are the Transformer:** You execute the command. The AI adopts the Persona to help you reason constraints-first.
- **Strict Distinction:** We call them **Personas** in the CLI to avoid confusion, but they are architecturally **Agential Roles** (A.13) defined in `.quint/agents/`.

### The ADI Cycle

The workflow follows the Canonical Reasoning Cycle (Pattern B.5):

1.  **Abduction (`/q1-hypothesize`)**: Generate plausible, competing hypotheses (L0).
2.  **Deduction (`/q2-verify`)**: Logically verify the hypotheses. Check constraints and typing. Promotes L0 → L1.
3.  **Induction (`/q3-validate`)**: Gather empirical evidence via tests or research. Promotes L1 → L2.
4.  **Audit (`/q4-audit`)**: Compute trust scores (R_eff) based on Weakest Link (WLNK) and Congruence (CL).
5.  **Decision (`/q5-decide`)**: Select the winner and generate the Design Rationale Record (DRR).

### Commands Reference

| Command | Phase | What It Does |
|---------|-------|--------------|
| `/q0-init` | Setup | Initialize `.quint/` and record Bounded Context |
| `/q1-hypothesize` | Abduction | Generate 3-5 L0 hypotheses |
| `/q2-verify` | Deduction | Verify logic/types. Promote L0 → L1 |
| `/q3-validate` | Induction | Run tests or research. Promote L1 → L2 |
| `/q4-audit` | Audit | WLNK analysis and bias check |
| `/q5-decide` | Decision | Finalize and create DRR (E.9) |
| `/q-status` | Utility | Show current phase and state |
| `/q-decay` | Maintenance | Check for expired evidence |

## Architecture: Surface vs. Grounding

This project strictly separates the **User Experience** from the **Assurance Layer** (Pattern E.14).

*   **Surface (What you see):** Clean, concise summaries in the chat.
*   **Grounding (What is stored):** Detailed JSON structures, proofs, and evidence logs stored in `.quint/knowledge/` and the SQLite database.

This ensures you have a rigorous audit trail without cluttering your thinking process.

## License

MIT License. FPF methodology by Anatoly Levenchuk.
