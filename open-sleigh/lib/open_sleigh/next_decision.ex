defmodule OpenSleigh.NextDecision do
  @moduledoc """
  Closed sum type — the return of `OpenSleigh.PhaseMachine.next/2`.

  Per `specs/target-system/TARGET_SYSTEM_MODEL.md §GateResult` + L3
  pattern.

  The three decisions:

  * `{:advance, Phase.t()}` — all gates passed; move to the next
    phase named by the workflow's `advance_map`.
  * `{:block, reasons}` — one or more gate failures; agent retries
    or session regresses. `reasons` is the output of
    `GateResult.combine/1` (a list of typed reasons per failed gate).
  * `{:terminal, Verdict.t()}` — phase was terminal in the workflow
    graph, or the outcome carries a verdict that concludes the
    session.

  `:await_human` is intentionally NOT one of the variants here — it
  is a pre-PhaseOutcome state handled by L5 `AgentWorker` before
  `PhaseOutcome.new/2` is even called (per gate-config consistency
  PR10: a PhaseOutcome with a declared human gate ALWAYS has the
  approval in `gate_results`, so by the time PhaseMachine sees it,
  there is no pending human gate).
  """

  alias OpenSleigh.{Phase, Verdict}

  @type t ::
          {:advance, Phase.t()}
          | {:block, [term()]}
          | {:terminal, Verdict.t()}

  @doc "Is `value` a valid `NextDecision`?"
  @spec valid?(term()) :: boolean()
  def valid?({:advance, phase}), do: Phase.valid?(phase)
  def valid?({:block, reasons}) when is_list(reasons), do: true
  def valid?({:terminal, verdict}), do: Verdict.valid?(verdict)
  def valid?(_), do: false
end
