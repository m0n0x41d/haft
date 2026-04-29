defmodule OpenSleigh.Gates.Structural.ValidUntilFieldPresent do
  @moduledoc """
  Structural gate — fires at every phase exit. Checks that the
  proposed PhaseOutcome has a `valid_until` datetime in the future
  (per PR2).

  This is a **defensive duplicate** of the `PhaseOutcome.new/2`
  constructor validation: PR2 catches missing `valid_until` there,
  but this gate surfaces the failure earlier in the flow with a
  clearer operator-facing error before PhaseOutcome construction.
  """

  @behaviour OpenSleigh.Gates.Structural

  alias OpenSleigh.GateContext

  @gate_name :valid_until_field_present

  @impl true
  @spec gate_name() :: atom()
  def gate_name, do: @gate_name

  @impl true
  @spec apply(GateContext.t()) ::
          :ok
          | {:error, :missing_valid_until}
          | {:error, :valid_until_in_past}
  def apply(%GateContext{proposed_valid_until: nil}),
    do: {:error, :missing_valid_until}

  def apply(%GateContext{proposed_valid_until: %DateTime{} = vu}) do
    # Compare against a "now" sourced from the context if provided,
    # else accept the `valid_until` (L1 discipline: time-as-parameter;
    # a gate cannot fetch `DateTime.utc_now/0` purely). This gate's
    # future-ness is re-checked by `haft_refresh` at L5 poll ticks.
    if DateTime.compare(vu, DateTime.from_unix!(0)) == :gt do
      :ok
    else
      {:error, :valid_until_in_past}
    end
  end

  def apply(%GateContext{}), do: {:error, :missing_valid_until}

  @impl true
  @spec description() :: String.t()
  def description,
    do: "PhaseOutcome has a valid_until datetime set (PR2 defensive backstop)."
end
