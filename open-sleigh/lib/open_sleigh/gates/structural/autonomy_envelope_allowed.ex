defmodule OpenSleigh.Gates.Structural.AutonomyEnvelopeAllowed do
  @moduledoc """
  Structural gate - checks optional AutonomyEnvelope limits.

  A missing envelope is admissible only for checkpointed/manual execution.
  When `turn_result.autonomy_required` is true, absence blocks. When a
  snapshot is present, it can only further restrict execution.
  """

  @behaviour OpenSleigh.Gates.Structural

  alias OpenSleigh.{AutonomyEnvelope, GateContext, WorkCommission}

  @gate_name :autonomy_envelope_allowed

  @impl true
  @spec gate_name() :: atom()
  def gate_name, do: @gate_name

  @impl true
  @spec apply(GateContext.t()) ::
          :ok
          | {:error, :missing_commission}
          | {:error, :missing_checked_at}
          | {:error, AutonomyEnvelope.allow_error()}
  def apply(%GateContext{} = ctx) do
    with {:ok, commission} <- commission(ctx),
         {:ok, checked_at} <- checked_at(ctx) do
      AutonomyEnvelope.allow_commission(
        commission.autonomy_envelope_snapshot,
        commission,
        checked_at,
        autonomy_required?(ctx)
      )
    end
  end

  @impl true
  @spec description() :: String.t()
  def description do
    "AutonomyEnvelope, when present or required, does not expand commission authority."
  end

  @spec commission(GateContext.t()) ::
          {:ok, WorkCommission.t()} | {:error, :missing_commission}
  defp commission(%GateContext{} = ctx) do
    ctx
    |> context_value(:commission)
    |> commission_result()
  end

  @spec commission_result(term()) ::
          {:ok, WorkCommission.t()} | {:error, :missing_commission}
  defp commission_result(%WorkCommission{} = commission), do: {:ok, commission}
  defp commission_result(_value), do: {:error, :missing_commission}

  @spec checked_at(GateContext.t()) ::
          {:ok, DateTime.t()} | {:error, :missing_checked_at}
  defp checked_at(%GateContext{} = ctx) do
    ctx
    |> context_value(:checked_at)
    |> checked_at_result()
  end

  @spec checked_at_result(term()) ::
          {:ok, DateTime.t()} | {:error, :missing_checked_at}
  defp checked_at_result(%DateTime{} = checked_at), do: {:ok, checked_at}
  defp checked_at_result(_value), do: {:error, :missing_checked_at}

  @spec autonomy_required?(GateContext.t()) :: boolean()
  defp autonomy_required?(%GateContext{} = ctx) do
    ctx
    |> context_value(:autonomy_required)
    |> autonomy_required_result()
  end

  @spec autonomy_required_result(term()) :: boolean()
  defp autonomy_required_result(true), do: true
  defp autonomy_required_result(_value), do: false

  @spec context_value(GateContext.t(), atom()) :: term()
  defp context_value(%GateContext{turn_result: turn_result}, key) do
    turn_result
    |> value_at(key)
  end

  @spec value_at(term(), atom()) :: term()
  defp value_at(%{} = map, key) do
    Map.get(map, key, Map.get(map, Atom.to_string(key)))
  end

  defp value_at(_value, _key), do: nil
end
