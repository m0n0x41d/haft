defmodule OpenSleigh.Gates.Structural.CommissionRunnable do
  @moduledoc """
  Structural gate - fires at Preflight entry before Execute can start.

  Checks only deterministic commission fields: a WorkCommission snapshot
  exists, is in a runnable/preflight-owned state, is not expired at the
  caller-supplied `checked_at`, and carries a claimed preflight lease in
  the CommissionRevisionSnapshot.
  """

  @behaviour OpenSleigh.Gates.Structural

  alias OpenSleigh.{CommissionRevisionSnapshot, GateContext, WorkCommission}

  @gate_name :commission_runnable
  @runnable_states [:queued, :ready, :preflighting]
  @claimed_lease_states [:claimed_for_preflight]

  @impl true
  @spec gate_name() :: atom()
  def gate_name, do: @gate_name

  @impl true
  @spec apply(GateContext.t()) ::
          :ok
          | {:error, :missing_commission}
          | {:error, :commission_not_runnable}
          | {:error, :missing_checked_at}
          | {:error, :commission_expired}
          | {:error, :missing_commission_snapshot}
          | {:error, :commission_snapshot_mismatch}
          | {:error, :preflight_lease_missing}
          | {:error, :preflight_lease_not_claimed}
  def apply(%GateContext{} = ctx) do
    with {:ok, commission} <- commission(ctx),
         :ok <- runnable_state(commission),
         {:ok, checked_at} <- checked_at(ctx),
         :ok <- not_expired(commission, checked_at),
         {:ok, snapshot} <- commission_snapshot(ctx),
         :ok <- snapshot_commission_matches(commission, snapshot),
         :ok <- lease_present(snapshot),
         :ok <- lease_claimed(snapshot) do
      :ok
    end
  end

  @impl true
  @spec description() :: String.t()
  def description do
    "WorkCommission is runnable, unexpired, and leased for deterministic Preflight."
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

  @spec runnable_state(WorkCommission.t()) ::
          :ok | {:error, :commission_not_runnable}
  defp runnable_state(%WorkCommission{state: state}) when state in @runnable_states, do: :ok
  defp runnable_state(%WorkCommission{}), do: {:error, :commission_not_runnable}

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

  @spec not_expired(WorkCommission.t(), DateTime.t()) ::
          :ok | {:error, :commission_expired}
  defp not_expired(%WorkCommission{valid_until: valid_until}, %DateTime{} = checked_at) do
    valid_until
    |> DateTime.compare(checked_at)
    |> not_expired_result()
  end

  @spec not_expired_result(:lt | :eq | :gt) :: :ok | {:error, :commission_expired}
  defp not_expired_result(:gt), do: :ok
  defp not_expired_result(_order), do: {:error, :commission_expired}

  @spec commission_snapshot(GateContext.t()) ::
          {:ok, CommissionRevisionSnapshot.t()} | {:error, :missing_commission_snapshot}
  defp commission_snapshot(%GateContext{} = ctx) do
    ctx
    |> context_value(:commission_snapshot)
    |> commission_snapshot_result()
  end

  @spec commission_snapshot_result(term()) ::
          {:ok, CommissionRevisionSnapshot.t()} | {:error, :missing_commission_snapshot}
  defp commission_snapshot_result(%CommissionRevisionSnapshot{} = snapshot), do: {:ok, snapshot}
  defp commission_snapshot_result(_value), do: {:error, :missing_commission_snapshot}

  @spec snapshot_commission_matches(WorkCommission.t(), CommissionRevisionSnapshot.t()) ::
          :ok | {:error, :commission_snapshot_mismatch}
  defp snapshot_commission_matches(
         %WorkCommission{id: commission_id},
         %CommissionRevisionSnapshot{commission_id: commission_id}
       ),
       do: :ok

  defp snapshot_commission_matches(%WorkCommission{}, %CommissionRevisionSnapshot{}),
    do: {:error, :commission_snapshot_mismatch}

  @spec lease_present(CommissionRevisionSnapshot.t()) ::
          :ok | {:error, :preflight_lease_missing}
  defp lease_present(%CommissionRevisionSnapshot{lease_id: lease_id}) when is_binary(lease_id) do
    lease_id
    |> String.trim()
    |> lease_present_result()
  end

  defp lease_present(%CommissionRevisionSnapshot{}), do: {:error, :preflight_lease_missing}

  @spec lease_present_result(String.t()) :: :ok | {:error, :preflight_lease_missing}
  defp lease_present_result(""), do: {:error, :preflight_lease_missing}
  defp lease_present_result(_lease_id), do: :ok

  @spec lease_claimed(CommissionRevisionSnapshot.t()) ::
          :ok | {:error, :preflight_lease_not_claimed}
  defp lease_claimed(%CommissionRevisionSnapshot{lease_state: lease_state})
       when lease_state in @claimed_lease_states,
       do: :ok

  defp lease_claimed(%CommissionRevisionSnapshot{}), do: {:error, :preflight_lease_not_claimed}

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
