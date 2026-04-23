defmodule OpenSleigh.CommissionSource.Intake do
  @moduledoc """
  Shared intake boundary from `CommissionSource` snapshots into the local
  tracker queue used by the orchestrator.

  A source may be one-shot (local fixtures) or dynamic (Haft). Dynamic
  sources are replenished on every tracker poll so long-running harnesses
  can pick up new WorkCommissions without restarting.
  """

  alias OpenSleigh.WorkCommission

  @skip_claim_errors [:commission_lock_conflict, :commission_not_runnable, :commission_not_found]

  @type source_ref :: %{
          required(:adapter) => module(),
          required(:handle) => term(),
          required(:max_claims) => pos_integer(),
          optional(:dynamic?) => boolean()
        }

  @doc "Build a source reference carried by the tracker runtime map."
  @spec source_ref(module(), term(), pos_integer(), boolean()) :: source_ref()
  def source_ref(adapter, handle, max_claims, dynamic?)
      when is_atom(adapter) and is_integer(max_claims) and max_claims > 0 do
    %{
      adapter: adapter,
      handle: handle,
      max_claims: max_claims,
      dynamic?: dynamic?
    }
  end

  @doc "Is this source safe to replenish on every poll?"
  @spec dynamic?(source_ref() | nil | term()) :: boolean()
  def dynamic?(%{dynamic?: true}), do: true
  def dynamic?(_source), do: false

  @doc "Claim runnable commissions from the source and seed the tracker queue."
  @spec replenish(source_ref(), module(), term()) :: {:ok, non_neg_integer()} | {:error, term()}
  def replenish(
        %{adapter: adapter, handle: handle, max_claims: max_claims},
        tracker_adapter,
        tracker_handle
      )
      when is_atom(adapter) and is_atom(tracker_adapter) do
    with {:ok, commissions} <- adapter.list_runnable(handle),
         {:ok, claimed} <- claim_commissions(adapter, handle, commissions, max_claims),
         :ok <- seed_claimed(tracker_adapter, tracker_handle, claimed) do
      {:ok, length(claimed)}
    end
  end

  @spec claim_commissions(module(), term(), [WorkCommission.t()], pos_integer()) ::
          {:ok, [WorkCommission.t()]} | {:error, term()}
  defp claim_commissions(adapter, handle, commissions, max_claims) do
    commissions
    |> Enum.reduce_while({:ok, [], max_claims}, &claim_commission(adapter, handle, &1, &2))
    |> claimed_commissions()
  end

  @spec claim_commission(
          module(),
          term(),
          WorkCommission.t(),
          {:ok, [WorkCommission.t()], non_neg_integer()}
        ) ::
          {:cont, {:ok, [WorkCommission.t()], non_neg_integer()}}
          | {:halt, {:ok, [WorkCommission.t()], non_neg_integer()}}
          | {:halt, {:error, term()}}
  defp claim_commission(_adapter, _handle, _commission, {:ok, claimed, 0}) do
    {:halt, {:ok, claimed, 0}}
  end

  defp claim_commission(adapter, handle, commission, {:ok, claimed, remaining}) do
    case adapter.claim_for_preflight(handle, commission.id) do
      {:ok, next_commission} ->
        {:cont, {:ok, [next_commission | claimed], remaining - 1}}

      {:error, reason} when reason in @skip_claim_errors ->
        {:cont, {:ok, claimed, remaining}}

      {:error, reason} ->
        {:halt, {:error, reason}}
    end
  end

  @spec claimed_commissions({:ok, [WorkCommission.t()], non_neg_integer()} | {:error, term()}) ::
          {:ok, [WorkCommission.t()]} | {:error, term()}
  defp claimed_commissions({:ok, commissions, _remaining}) do
    commissions
    |> Enum.reverse()
    |> then(&{:ok, &1})
  end

  defp claimed_commissions({:error, _reason} = error), do: error

  @spec seed_claimed(module(), term(), [WorkCommission.t()]) :: :ok | {:error, term()}
  defp seed_claimed(_tracker_adapter, _tracker_handle, []), do: :ok

  defp seed_claimed(tracker_adapter, tracker_handle, commissions) do
    tickets =
      commissions
      |> Enum.map(&ticket_attrs/1)

    tracker_adapter.seed(tracker_handle, tickets)
  end

  @spec ticket_attrs(WorkCommission.t()) :: map()
  defp ticket_attrs(commission) do
    %{
      id: commission.id,
      source: {:github, commission.scope.repo_ref},
      title: "WorkCommission " <> commission.id,
      body: "",
      state: :todo,
      problem_card_ref: commission.problem_card_ref,
      target_branch: commission.scope.target_branch,
      fetched_at: commission.fetched_at,
      metadata: %{
        commission: commission,
        commission_id: commission.id,
        source_mode: :commission_first,
        problem_revision_hash: commission.decision_revision_hash,
        lease_id: "local-preflight:" <> commission.id,
        lease_state: :claimed_for_preflight,
        current_decision: current_decision(commission)
      }
    }
  end

  @spec current_decision(WorkCommission.t()) :: map()
  defp current_decision(commission) do
    %{
      decision_ref: commission.decision_ref,
      decision_revision_hash: commission.decision_revision_hash,
      status: :active,
      refresh_due: false,
      freshness: :healthy
    }
  end
end
