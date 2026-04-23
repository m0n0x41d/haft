defmodule OpenSleigh.Gates.Structural.DecisionFresh do
  @moduledoc """
  Structural gate - checks the linked DecisionRecord before Execute.

  This gate does not infer semantic context freshness from prose. It
  only verifies deterministic snapshot equality plus the current
  DecisionRecord lifecycle fields supplied by the preflight caller.
  """

  @behaviour OpenSleigh.Gates.Structural

  alias OpenSleigh.{CommissionRevisionSnapshot, GateContext}

  @gate_name :decision_fresh

  @impl true
  @spec gate_name() :: atom()
  def gate_name, do: @gate_name

  @impl true
  @spec apply(GateContext.t()) ::
          :ok
          | {:error, :decision_missing}
          | {:error, :decision_ref_changed}
          | {:error, :decision_revision_changed}
          | {:error, :decision_superseded}
          | {:error, :decision_deprecated}
          | {:error, :decision_refresh_due}
          | {:error, :decision_stale}
          | {:error, :decision_not_active}
          | {:error, :missing_commission_snapshot}
          | {:error, :missing_current_snapshot}
  def apply(%GateContext{} = ctx) do
    with {:ok, decision} <- current_decision(ctx),
         :ok <- active_status(decision),
         :ok <- no_refresh_due(decision),
         :ok <- freshness_not_stale(decision),
         {:ok, commission_snapshot} <- commission_snapshot(ctx),
         {:ok, current_snapshot} <- current_snapshot(ctx),
         :ok <- snapshot_field_matches(commission_snapshot, current_snapshot, :decision_ref),
         :ok <-
           snapshot_field_matches(
             commission_snapshot,
             current_snapshot,
             :decision_revision_hash
           ),
         :ok <- optional_decision_field_matches(decision, :decision_ref, current_snapshot),
         :ok <-
           optional_decision_field_matches(decision, :decision_revision_hash, current_snapshot) do
      :ok
    end
  end

  @impl true
  @spec description() :: String.t()
  def description do
    "Linked DecisionRecord exists, is active/fresh, and matches the commission snapshot."
  end

  @spec current_decision(GateContext.t()) :: {:ok, map()} | {:error, :decision_missing}
  defp current_decision(%GateContext{} = ctx) do
    ctx
    |> context_value(:current_decision)
    |> current_decision_result()
  end

  @spec current_decision_result(term()) :: {:ok, map()} | {:error, :decision_missing}
  defp current_decision_result(%{} = decision), do: {:ok, decision}
  defp current_decision_result(_value), do: {:error, :decision_missing}

  @spec active_status(map()) ::
          :ok
          | {:error, :decision_superseded}
          | {:error, :decision_deprecated}
          | {:error, :decision_refresh_due}
          | {:error, :decision_stale}
          | {:error, :decision_not_active}
  defp active_status(decision) do
    decision
    |> value_at(:status)
    |> normalized_marker()
    |> active_status_result()
  end

  @spec active_status_result(atom() | nil | term()) ::
          :ok
          | {:error, :decision_superseded}
          | {:error, :decision_deprecated}
          | {:error, :decision_refresh_due}
          | {:error, :decision_stale}
          | {:error, :decision_not_active}
  defp active_status_result(:active), do: :ok
  defp active_status_result(:superseded), do: {:error, :decision_superseded}
  defp active_status_result(:deprecated), do: {:error, :decision_deprecated}
  defp active_status_result(:refresh_due), do: {:error, :decision_refresh_due}
  defp active_status_result(:stale), do: {:error, :decision_stale}
  defp active_status_result(_status), do: {:error, :decision_not_active}

  @spec no_refresh_due(map()) :: :ok | {:error, :decision_refresh_due}
  defp no_refresh_due(decision) do
    decision
    |> refresh_due_value()
    |> refresh_due_result()
  end

  @spec refresh_due_value(map()) :: term()
  defp refresh_due_value(decision) do
    decision
    |> value_at(:refresh_due)
    |> refresh_due_value_result(decision)
  end

  @spec refresh_due_value_result(term(), map()) :: term()
  defp refresh_due_value_result(nil, decision), do: value_at(decision, :refresh_due?)
  defp refresh_due_value_result(value, _decision), do: value

  @spec refresh_due_result(term()) :: :ok | {:error, :decision_refresh_due}
  defp refresh_due_result(true), do: {:error, :decision_refresh_due}
  defp refresh_due_result(_value), do: :ok

  @spec freshness_not_stale(map()) :: :ok | {:error, :decision_stale}
  defp freshness_not_stale(decision) do
    decision
    |> value_at(:freshness)
    |> normalized_marker()
    |> freshness_result()
  end

  @spec freshness_result(atom() | nil | term()) :: :ok | {:error, :decision_stale}
  defp freshness_result(:stale), do: {:error, :decision_stale}
  defp freshness_result(:at_risk), do: {:error, :decision_stale}
  defp freshness_result(_freshness), do: :ok

  @spec commission_snapshot(GateContext.t()) ::
          {:ok, CommissionRevisionSnapshot.t()} | {:error, :missing_commission_snapshot}
  defp commission_snapshot(%GateContext{} = ctx) do
    ctx
    |> context_value(:commission_snapshot)
    |> commission_snapshot_result()
  end

  @spec current_snapshot(GateContext.t()) ::
          {:ok, CommissionRevisionSnapshot.t()} | {:error, :missing_current_snapshot}
  defp current_snapshot(%GateContext{} = ctx) do
    ctx
    |> context_value(:current_snapshot)
    |> current_snapshot_result()
  end

  @spec commission_snapshot_result(term()) ::
          {:ok, CommissionRevisionSnapshot.t()} | {:error, :missing_commission_snapshot}
  defp commission_snapshot_result(%CommissionRevisionSnapshot{} = snapshot), do: {:ok, snapshot}
  defp commission_snapshot_result(_value), do: {:error, :missing_commission_snapshot}

  @spec current_snapshot_result(term()) ::
          {:ok, CommissionRevisionSnapshot.t()} | {:error, :missing_current_snapshot}
  defp current_snapshot_result(%CommissionRevisionSnapshot{} = snapshot), do: {:ok, snapshot}
  defp current_snapshot_result(_value), do: {:error, :missing_current_snapshot}

  @spec snapshot_field_matches(
          CommissionRevisionSnapshot.t(),
          CommissionRevisionSnapshot.t(),
          atom()
        ) ::
          :ok | {:error, :decision_ref_changed} | {:error, :decision_revision_changed}
  defp snapshot_field_matches(left, right, :decision_ref) do
    left
    |> Map.fetch!(:decision_ref)
    |> values_match(Map.fetch!(right, :decision_ref), :decision_ref_changed)
  end

  defp snapshot_field_matches(left, right, :decision_revision_hash) do
    left
    |> Map.fetch!(:decision_revision_hash)
    |> values_match(
      Map.fetch!(right, :decision_revision_hash),
      :decision_revision_changed
    )
  end

  @spec optional_decision_field_matches(map(), atom(), CommissionRevisionSnapshot.t()) ::
          :ok | {:error, :decision_ref_changed} | {:error, :decision_revision_changed}
  defp optional_decision_field_matches(decision, :decision_ref, current_snapshot) do
    decision
    |> value_at(:decision_ref)
    |> optional_value_match(current_snapshot.decision_ref, :decision_ref_changed)
  end

  defp optional_decision_field_matches(decision, :decision_revision_hash, current_snapshot) do
    decision
    |> value_at(:decision_revision_hash)
    |> optional_value_match(
      current_snapshot.decision_revision_hash,
      :decision_revision_changed
    )
  end

  @spec values_match(term(), term(), atom()) :: :ok | {:error, atom()}
  defp values_match(value, value, _error), do: :ok
  defp values_match(_left, _right, error), do: {:error, error}

  @spec optional_value_match(term(), term(), atom()) :: :ok | {:error, atom()}
  defp optional_value_match(nil, _expected, _error), do: :ok
  defp optional_value_match(value, value, _error), do: :ok
  defp optional_value_match(_value, _expected, error), do: {:error, error}

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

  @spec normalized_marker(term()) :: atom() | nil | term()
  defp normalized_marker(value) when is_binary(value) do
    value
    |> String.trim()
    |> String.downcase()
    |> String.replace(" ", "_")
    |> marker_from_string()
  end

  defp normalized_marker(value), do: value

  @spec marker_from_string(String.t()) :: atom() | nil | String.t()
  defp marker_from_string(""), do: nil
  defp marker_from_string("active"), do: :active
  defp marker_from_string("superseded"), do: :superseded
  defp marker_from_string("deprecated"), do: :deprecated
  defp marker_from_string("refresh_due"), do: :refresh_due
  defp marker_from_string("stale"), do: :stale
  defp marker_from_string("at_risk"), do: :at_risk
  defp marker_from_string("healthy"), do: :healthy
  defp marker_from_string(value), do: value
end
