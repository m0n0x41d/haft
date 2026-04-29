defmodule OpenSleigh.Gates.Structural.EvidenceRefNotSelf do
  @moduledoc """
  Structural gate — Measure exit. Narrowed from v0.2.

  Per `specs/target-system/GATES.md §1` + `ILLEGAL_STATES.md` PR5:

  * Every evidence item has non-empty `ref`/`hash`
  * `evidence.ref != ctx.self_id`

  This is **duplicate** of the `PhaseOutcome.new/2` PR5 constructor
  check (v0.5 CRITICAL-2 fix). Defensive: surfaces self-reference
  earlier with a gate-level error before PhaseOutcome construction.
  """

  @behaviour OpenSleigh.Gates.Structural

  alias OpenSleigh.{GateContext, SessionScopedArtifactId}

  @gate_name :evidence_ref_not_self

  @impl true
  @spec gate_name() :: atom()
  def gate_name, do: @gate_name

  @impl true
  @spec apply(GateContext.t()) ::
          :ok
          | {:error, :evidence_empty_ref}
          | {:error, :evidence_self_reference}
  def apply(%GateContext{evidence: evidence, self_id: self_id}) do
    self_id_str = SessionScopedArtifactId.to_string(self_id)

    cond do
      Enum.any?(evidence, &(&1.ref == "")) -> {:error, :evidence_empty_ref}
      Enum.any?(evidence, &(&1.ref == self_id_str)) -> {:error, :evidence_self_reference}
      true -> :ok
    end
  end

  @impl true
  @spec description() :: String.t()
  def description,
    do: "Every evidence.ref is non-empty AND != self_id (PR5 defensive gate)."
end
