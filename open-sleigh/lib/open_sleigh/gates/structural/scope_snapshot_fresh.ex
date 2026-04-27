defmodule OpenSleigh.Gates.Structural.ScopeSnapshotFresh do
  @moduledoc """
  Structural gate - checks deterministic CommissionRevisionSnapshot drift.

  The equality set is closed for preflight: problem revision, scope hash,
  base SHA, implementation plan revision, autonomy envelope revision,
  projection policy, and lease fields must still match before Execute.
  Unknown or uncertain snapshot state is not treated as pass.
  """

  @behaviour OpenSleigh.Gates.Structural

  alias OpenSleigh.{CommissionRevisionSnapshot, GateContext}

  @gate_name :scope_snapshot_fresh

  @field_checks [
    {:commission_id, :commission_id_changed},
    {:problem_card_ref, :problem_card_ref_changed},
    {:problem_revision_hash, :problem_revision_changed},
    {:spec_section_refs, :spec_section_refs_changed},
    {:spec_revision_hashes, :spec_revision_hashes_changed},
    {:scope_hash, :scope_hash_changed},
    {:base_sha, :base_sha_changed},
    {:implementation_plan_revision, :implementation_plan_revision_changed},
    {:autonomy_envelope_revision, :autonomy_envelope_revision_changed},
    {:projection_policy, :projection_policy_changed},
    {:lease_id, :lease_changed},
    {:lease_state, :lease_state_changed}
  ]

  @impl true
  @spec gate_name() :: atom()
  def gate_name, do: @gate_name

  @impl true
  @spec apply(GateContext.t()) ::
          :ok
          | {:error, :missing_commission_snapshot}
          | {:error, :missing_current_snapshot}
          | {:error, :commission_id_changed}
          | {:error, :problem_card_ref_changed}
          | {:error, :problem_revision_changed}
          | {:error, :spec_section_refs_changed}
          | {:error, :spec_revision_hashes_changed}
          | {:error, :scope_hash_changed}
          | {:error, :base_sha_changed}
          | {:error, :implementation_plan_revision_changed}
          | {:error, :autonomy_envelope_revision_changed}
          | {:error, :projection_policy_changed}
          | {:error, :lease_changed}
          | {:error, :lease_state_changed}
  def apply(%GateContext{} = ctx) do
    with {:ok, commission_snapshot} <- commission_snapshot(ctx),
         {:ok, current_snapshot} <- current_snapshot(ctx),
         :ok <- snapshot_fields_match(commission_snapshot, current_snapshot) do
      :ok
    end
  end

  @impl true
  @spec description() :: String.t()
  def description do
    "CommissionRevisionSnapshot deterministic equality set still matches before Execute."
  end

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

  @spec snapshot_fields_match(CommissionRevisionSnapshot.t(), CommissionRevisionSnapshot.t()) ::
          :ok | {:error, atom()}
  defp snapshot_fields_match(left, right) do
    @field_checks
    |> Enum.reduce_while(:ok, fn {field, error}, :ok ->
      left
      |> Map.fetch!(field)
      |> field_match_result(Map.fetch!(right, field), error)
    end)
  end

  @spec field_match_result(term(), term(), atom()) ::
          {:cont, :ok} | {:halt, {:error, atom()}}
  defp field_match_result(value, value, _error), do: {:cont, :ok}
  defp field_match_result(_left, _right, error), do: {:halt, {:error, error}}

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
