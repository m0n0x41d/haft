defmodule OpenSleigh.CommissionSource.Intake do
  @moduledoc """
  Shared intake boundary from `CommissionSource` snapshots into the local
  tracker queue used by the orchestrator.

  A source may be one-shot (local fixtures) or dynamic (Haft). Dynamic
  sources are replenished on every tracker poll so long-running harnesses
  can pick up new WorkCommissions without restarting.
  """

  alias OpenSleigh.WorkCommission

  @default_lease_age_cap_seconds 24 * 60 * 60
  @lease_age_capped_states [:preflighting]
  @skip_claim_error_atoms [
    :commission_lock_conflict,
    :commission_not_runnable,
    :commission_not_found,
    :lease_too_old
  ]

  @type source_ref :: %{
          required(:adapter) => module(),
          required(:handle) => term(),
          required(:max_claims) => pos_integer(),
          optional(:dynamic?) => boolean(),
          optional(:lease_age_cap_seconds) => pos_integer()
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

  @doc "Build a source reference with an explicit stale-lease age cap."
  @spec source_ref(module(), term(), pos_integer(), boolean(), pos_integer()) :: source_ref()
  def source_ref(adapter, handle, max_claims, dynamic?, lease_age_cap_seconds)
      when is_atom(adapter) and is_integer(max_claims) and max_claims > 0 and
             is_integer(lease_age_cap_seconds) and lease_age_cap_seconds > 0 do
    %{
      adapter: adapter,
      handle: handle,
      max_claims: max_claims,
      dynamic?: dynamic?,
      lease_age_cap_seconds: lease_age_cap_seconds
    }
  end

  @doc "Is this source safe to replenish on every poll?"
  @spec dynamic?(source_ref() | nil | term()) :: boolean()
  def dynamic?(%{dynamic?: true}), do: true
  def dynamic?(_source), do: false

  @doc "Claim runnable commissions from the source and seed the tracker queue."
  @spec replenish(source_ref(), module(), term()) :: {:ok, non_neg_integer()} | {:error, term()}
  def replenish(
        %{adapter: adapter, handle: handle, max_claims: max_claims} = source,
        tracker_adapter,
        tracker_handle
      )
      when is_atom(adapter) and is_atom(tracker_adapter) do
    lease_age_cap_seconds =
      source
      |> lease_age_cap_seconds()

    intake_time = DateTime.utc_now()

    with {:ok, commissions} <- adapter.list_runnable(handle),
         {:ok, claimed} <-
           claim_commissions(
             adapter,
             handle,
             commissions,
             max_claims,
             lease_age_cap_seconds,
             intake_time
           ),
         :ok <- seed_claimed(tracker_adapter, tracker_handle, claimed) do
      {:ok, length(claimed)}
    end
  end

  @spec lease_age_cap_seconds(source_ref()) :: pos_integer()
  defp lease_age_cap_seconds(%{lease_age_cap_seconds: lease_age_cap_seconds}),
    do: lease_age_cap_seconds

  defp lease_age_cap_seconds(_source), do: @default_lease_age_cap_seconds

  @spec claim_commissions(
          module(),
          term(),
          [WorkCommission.t()],
          pos_integer(),
          pos_integer(),
          DateTime.t()
        ) ::
          {:ok, [WorkCommission.t()]} | {:error, term()}
  defp claim_commissions(
         adapter,
         handle,
         commissions,
         max_claims,
         lease_age_cap_seconds,
         intake_time
       ) do
    claim_context = %{
      lease_age_cap_seconds: lease_age_cap_seconds,
      intake_time: intake_time
    }

    commissions
    |> Enum.reduce_while(
      {:ok, [], max_claims, claim_context},
      &claim_commission(adapter, handle, &1, &2)
    )
    |> claimed_commissions()
  end

  @spec claim_commission(
          module(),
          term(),
          WorkCommission.t(),
          {:ok, [WorkCommission.t()], non_neg_integer(), map()}
        ) ::
          {:cont, {:ok, [WorkCommission.t()], non_neg_integer(), map()}}
          | {:halt, {:ok, [WorkCommission.t()], non_neg_integer(), map()}}
          | {:halt, {:error, term()}}
  defp claim_commission(_adapter, _handle, _commission, {:ok, claimed, 0, claim_context}) do
    {:halt, {:ok, claimed, 0, claim_context}}
  end

  defp claim_commission(adapter, handle, commission, {:ok, claimed, remaining, claim_context}) do
    case claim_gate(commission, claim_context) do
      :claim ->
        claim_available_commission(adapter, handle, commission, claimed, remaining, claim_context)

      {:skip, reason} ->
        record_skip(adapter, handle, commission, reason)
        |> skipped_commission(claimed, remaining, claim_context)
    end
  end

  @spec claim_available_commission(
          module(),
          term(),
          WorkCommission.t(),
          [WorkCommission.t()],
          pos_integer(),
          map()
        ) ::
          {:cont, {:ok, [WorkCommission.t()], non_neg_integer(), map()}}
          | {:halt, {:error, term()}}
  defp claim_available_commission(adapter, handle, commission, claimed, remaining, claim_context) do
    case adapter.claim_for_preflight(handle, commission.id) do
      {:ok, next_commission} ->
        {:cont, {:ok, [next_commission | claimed], remaining - 1, claim_context}}

      {:error, reason} ->
        claim_error_result(reason, claimed, remaining, claim_context)
    end
  end

  @spec claim_error_result(term(), [WorkCommission.t()], pos_integer(), map()) ::
          {:cont, {:ok, [WorkCommission.t()], non_neg_integer(), map()}}
          | {:halt, {:error, term()}}
  defp claim_error_result(reason, claimed, remaining, claim_context) do
    case skip_claim_error?(reason) do
      true ->
        {:cont, {:ok, claimed, remaining, claim_context}}

      false ->
        {:halt, {:error, reason}}
    end
  end

  @spec skip_claim_error?(term()) :: boolean()
  defp skip_claim_error?(reason) when reason in @skip_claim_error_atoms, do: true
  defp skip_claim_error?({:lease_too_old, _details}), do: true
  defp skip_claim_error?(_reason), do: false

  @spec claim_gate(WorkCommission.t(), map()) :: :claim | {:skip, term()}
  defp claim_gate(%{state: state} = commission, claim_context)
       when state in @lease_age_capped_states do
    commission
    |> lease_age_gate(claim_context)
  end

  defp claim_gate(_commission, _claim_context), do: :claim

  @spec lease_age_gate(WorkCommission.t(), map()) :: :claim | {:skip, term()}
  defp lease_age_gate(commission, %{
         intake_time: intake_time,
         lease_age_cap_seconds: lease_age_cap_seconds
       }) do
    age_seconds =
      intake_time
      |> DateTime.diff(commission.fetched_at, :second)

    commission
    |> stale_lease_result(age_seconds, lease_age_cap_seconds)
  end

  @spec stale_lease_result(WorkCommission.t(), integer(), pos_integer()) ::
          :claim | {:skip, term()}
  defp stale_lease_result(commission, age_seconds, lease_age_cap_seconds)
       when age_seconds > lease_age_cap_seconds do
    reason = {
      :lease_too_old,
      %{
        commission_id: commission.id,
        state: commission.state,
        fetched_at: commission.fetched_at,
        age_seconds: age_seconds,
        lease_age_cap_seconds: lease_age_cap_seconds
      }
    }

    {:skip, reason}
  end

  defp stale_lease_result(_commission, _age_seconds, _lease_age_cap_seconds), do: :claim

  @spec record_skip(module(), term(), WorkCommission.t(), term()) :: :ok | {:error, term()}
  defp record_skip(adapter, handle, commission, reason) do
    case function_exported?(adapter, :record_skip, 3) do
      true ->
        adapter.record_skip(handle, commission.id, reason)

      false ->
        :ok
    end
  end

  @spec skipped_commission(:ok | {:error, term()}, [WorkCommission.t()], non_neg_integer(), map()) ::
          {:cont, {:ok, [WorkCommission.t()], non_neg_integer(), map()}}
          | {:halt, {:error, term()}}
  defp skipped_commission(:ok, claimed, remaining, claim_context) do
    {:cont, {:ok, claimed, remaining, claim_context}}
  end

  defp skipped_commission({:error, reason}, _claimed, _remaining, _claim_context) do
    {:halt, {:error, reason}}
  end

  @spec claimed_commissions(
          {:ok, [WorkCommission.t()], non_neg_integer(), map()}
          | {:error, term()}
        ) ::
          {:ok, [WorkCommission.t()]} | {:error, term()}
  defp claimed_commissions({:ok, commissions, _remaining, _claim_context}) do
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
