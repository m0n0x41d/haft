defmodule OpenSleigh.GateKind do
  @moduledoc """
  Closed sum of gate kinds: `:structural | :semantic | :human`.

  Per `specs/target-system/PHASE_ONTOLOGY.md` axis 2 and
  `specs/target-system/ILLEGAL_STATES.md` GK1–GK6: the three kinds are
  **compile-time distinct sum variants**; never confused, never merged.

  * `:structural` — pure L2 function, deterministic, field/shape checks.
  * `:semantic`   — effectful L2 contract (LLM-judge via L4 JudgeClient).
    Non-deterministic; drift tracked via golden-set calibration (§6b.1).
  * `:human`      — triggered, not computed; blocks transition pending an
    external `/approve` signal.

  Pattern-matching on the kind tag is the canonical way to dispatch —
  `GateResult.combine/1` rejects untyped merges.
  """

  @typedoc "A gate-kind atom."
  @type t :: :structural | :semantic | :human

  @all [:structural, :semantic, :human]

  @doc "All admissible gate-kind atoms."
  @spec all() :: [t(), ...]
  def all, do: @all

  @doc "Is `value` a valid gate kind?"
  @spec valid?(term()) :: boolean()
  def valid?(value) when value in @all, do: true
  def valid?(_), do: false
end
