defmodule OpenSleigh.Gates.Structural do
  @moduledoc """
  Behaviour contract for structural gates (L2).

  Per `specs/target-system/PHASE_ONTOLOGY.md §Structural` +
  `ILLEGAL_STATES.md` GK category:

  * Pure L2 function; deterministic; no LLM, no I/O beyond inspecting
    the `GateContext` it receives.
  * Returns `:ok` on pass or `{:error, reason_atom}` on fail.
  * Failure is **type-level** for the error taxonomy — the reason
    atom is an enumerated member of each gate module's own `@reason`
    sum (per ILLEGAL_STATES four-label taxonomy).

  An implementing module should:

  * Be registered in `OpenSleigh.Gates.Registry` under its canonical
    atom name (`CF3` / `CF4` catch unknown names at L6 compile).
  * Provide a `gate_name/0` convenience to return the registry atom.
  * Keep the apply/1 function side-effect-free.
  """

  alias OpenSleigh.GateContext

  @doc "The canonical atom name of this gate (as used in `sleigh.md`)."
  @callback gate_name() :: atom()

  @doc """
  Evaluate the gate against a `GateContext`. Pure — same input, same
  output.
  """
  @callback apply(GateContext.t()) :: :ok | {:error, atom()}

  @doc """
  Short human-readable description of what this gate checks. Used in
  operator-facing error messages and in golden-set labelling docs.
  """
  @callback description() :: String.t()
end
