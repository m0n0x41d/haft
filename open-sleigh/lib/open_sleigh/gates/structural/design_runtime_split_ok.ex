defmodule OpenSleigh.Gates.Structural.DesignRuntimeSplitOk do
  @moduledoc """
  Structural gate — Execute entry. Per `specs/target-system/GATES.md §1`:
  "No `MethodDescription`
  nodes embedded in `Work` traces (structural check on Haft graph
  shape)."

  **MVP-1 status: stub.** Full implementation requires L4 Haft graph
  access to walk the upstream ProblemCard's related artifacts and
  check that design-time Method nodes aren't embedded in run-time
  Work traces. L2 here defines the contract; the real traversal is
  an L4 effect wired into the gate's apply clause as the Haft graph
  slice is populated into `ctx.turn_result` or a sibling field.

  For MVP-1 canary, this gate returns `:ok` unconditionally. The
  implementation lands with the first Problem-Factory phase (MVP-2);
  its absence is tracked.
  """

  @behaviour OpenSleigh.Gates.Structural

  alias OpenSleigh.GateContext

  @gate_name :design_runtime_split_ok

  @impl true
  @spec gate_name() :: atom()
  def gate_name, do: @gate_name

  @impl true
  @spec apply(GateContext.t()) :: :ok
  def apply(%GateContext{}), do: :ok

  @impl true
  @spec description() :: String.t()
  def description,
    do:
      "No MethodDescription embedded in Work traces (MVP-1 stub; real check lands with MVP-2 Problem Factory)."
end
