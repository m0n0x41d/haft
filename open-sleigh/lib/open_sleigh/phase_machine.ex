defmodule OpenSleigh.PhaseMachine do
  @moduledoc """
  Pure L3 transition function over `(WorkflowState, PhaseOutcome)`.

  Per `specs/enabling-system/FUNCTIONAL_ARCHITECTURE.md` L3 +
  `ILLEGAL_STATES.md` TR1–TR7:

  * **Total function** over legal inputs: for every `WorkflowState.t()`
    and every `PhaseOutcome.t()` constructed via `PhaseOutcome.new/2`,
    `next/2` returns a `NextDecision.t()` (no raise, no crash).
  * Pattern-match on `Phase.t()` is exhaustive because the sum is
    closed (Q-OS-2 v0.5).
  * TR1: transitions follow the workflow's `advance_map` — no
    skip-phase clauses.
  * TR2: terminal phases are absorbing — `next/2` on a terminal
    state returns `{:terminal, outcome.verdict}` immediately.
  * TR7 (MVP-1): regression is not representable — fail verdicts
    route to `:terminal` with `:fail`, not back to an earlier phase.
    (MVP-2 adds explicit regression paths; they live here when
    they arrive.)

  **`:await_human` is not one of our decisions.** By the time an
  outcome reaches `PhaseMachine.next/2`, any human gate declared
  by `phase_config` has already been satisfied (otherwise
  `PhaseOutcome.new/2` would have failed gate-config consistency
  per PR10). Awaiting-human is a PRE-outcome state owned by L5.
  """

  alias OpenSleigh.{GateResult, NextDecision, PhaseOutcome, Workflow, WorkflowState}

  @doc """
  Decide the next step for a session given its current workflow
  state and a freshly-constructed `PhaseOutcome`.

  Decision table:

  | Outcome phase | Gates combine | Decision |
  |---------------|---------------|----------|
  | terminal      | (any)         | `{:terminal, outcome.verdict \\|\\| :pass}` |
  | non-terminal  | `:advance`    | `{:advance, next_phase}` or `{:terminal, :pass}` if no successor |
  | non-terminal  | `:block`      | `{:block, reasons}` |
  """
  @spec next(WorkflowState.t(), PhaseOutcome.t()) :: NextDecision.t()
  def next(%WorkflowState{} = state, %PhaseOutcome{} = outcome) do
    cond do
      terminal_phase?(state, outcome) ->
        {:terminal, outcome.verdict || :pass}

      true ->
        decide_non_terminal(state, outcome)
    end
  end

  @doc """
  Advance the `WorkflowState` according to the decision. Convenience
  wrapper for L5 that bundles `next/2` + `WorkflowState.apply_outcome/2`.
  Returns `{:ok, new_state, decision}` on advance, or
  `{decision, state}` when no state-mutation is warranted (block,
  terminal).
  """
  @spec advance(WorkflowState.t(), PhaseOutcome.t()) ::
          {:ok, WorkflowState.t(), NextDecision.t()} | {NextDecision.t(), WorkflowState.t()}
  def advance(%WorkflowState{} = state, %PhaseOutcome{} = outcome) do
    decision = next(state, outcome)

    case decision do
      {:advance, _next_phase} ->
        case WorkflowState.apply_outcome(state, outcome) do
          {:ok, new_state} -> {:ok, new_state, decision}
          {:error, _reason} -> {decision, state}
        end

      _ ->
        {decision, state}
    end
  end

  # ——— internals ———

  @spec terminal_phase?(WorkflowState.t(), PhaseOutcome.t()) :: boolean()
  defp terminal_phase?(%WorkflowState{workflow: wf}, %PhaseOutcome{phase: phase}) do
    Workflow.terminal?(wf, phase)
  end

  @spec decide_non_terminal(WorkflowState.t(), PhaseOutcome.t()) :: NextDecision.t()
  defp decide_non_terminal(%WorkflowState{workflow: wf}, %PhaseOutcome{
         phase: phase,
         gate_results: results
       }) do
    case GateResult.combine(results) do
      {:advance, []} -> advance_decision(wf, phase)
      {:block, reasons} -> {:block, reasons}
    end
  end

  @spec advance_decision(Workflow.t(), OpenSleigh.Phase.t()) :: NextDecision.t()
  defp advance_decision(wf, phase) do
    case Workflow.advance_from(wf, phase) do
      nil -> {:terminal, :pass}
      next_phase -> advance_or_terminal(wf, next_phase)
    end
  end

  @spec advance_or_terminal(Workflow.t(), OpenSleigh.Phase.t()) :: NextDecision.t()
  defp advance_or_terminal(wf, next_phase) do
    if Workflow.terminal?(wf, next_phase) do
      {:terminal, :pass}
    else
      {:advance, next_phase}
    end
  end
end
