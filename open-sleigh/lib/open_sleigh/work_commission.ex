defmodule OpenSleigh.WorkCommission do
  @moduledoc """
  Haft-authored authorization boundary between a DecisionRecord and a
  RuntimeRun.

  Open-Sleigh treats this as immutable snapshot data. The constructor
  validates required fields and rejects drift between the WorkCommission
  fields and the embedded Scope before any runtime session can depend on
  them.
  """

  alias OpenSleigh.{CommissionRevisionSnapshot, ProblemCardRef, Scope}

  @projection_policies [:local_only, :external_optional, :external_required]

  @states [
    :draft,
    :queued,
    :ready,
    :preflighting,
    :running,
    :blocked_stale,
    :blocked_policy,
    :blocked_conflict,
    :needs_human_review,
    :completed,
    :completed_with_projection_debt,
    :failed,
    :cancelled,
    :expired
  ]

  @enforce_keys [
    :id,
    :decision_ref,
    :decision_revision_hash,
    :problem_card_ref,
    :scope,
    :scope_hash,
    :base_sha,
    :lockset,
    :evidence_requirements,
    :projection_policy,
    :state,
    :valid_until,
    :fetched_at
  ]
  defstruct [
    :id,
    :decision_ref,
    :decision_revision_hash,
    :problem_card_ref,
    :implementation_plan_ref,
    :implementation_plan_revision,
    :scope,
    :scope_hash,
    :base_sha,
    :lockset,
    :evidence_requirements,
    :projection_policy,
    :autonomy_envelope_ref,
    :autonomy_envelope_revision,
    :state,
    :valid_until,
    :fetched_at
  ]

  @type state ::
          :draft
          | :queued
          | :ready
          | :preflighting
          | :running
          | :blocked_stale
          | :blocked_policy
          | :blocked_conflict
          | :needs_human_review
          | :completed
          | :completed_with_projection_debt
          | :failed
          | :cancelled
          | :expired

  @type projection_policy :: :local_only | :external_optional | :external_required

  @type t :: %__MODULE__{
          id: String.t(),
          decision_ref: String.t(),
          decision_revision_hash: String.t(),
          problem_card_ref: ProblemCardRef.t(),
          implementation_plan_ref: String.t() | nil,
          implementation_plan_revision: String.t() | nil,
          scope: Scope.t(),
          scope_hash: String.t(),
          base_sha: String.t(),
          lockset: [String.t()],
          evidence_requirements: [term()],
          projection_policy: projection_policy(),
          autonomy_envelope_ref: String.t() | nil,
          autonomy_envelope_revision: String.t() | nil,
          state: state(),
          valid_until: DateTime.t(),
          fetched_at: DateTime.t()
        }

  @type new_error ::
          :invalid_id
          | :invalid_decision_ref
          | :invalid_decision_revision_hash
          | :invalid_problem_card_ref
          | :invalid_implementation_plan_ref
          | :invalid_implementation_plan_revision
          | :invalid_scope
          | :invalid_scope_hash
          | :scope_hash_mismatch
          | :invalid_base_sha
          | :base_sha_scope_mismatch
          | :invalid_lockset
          | :lockset_scope_mismatch
          | :invalid_evidence_requirements
          | :invalid_projection_policy
          | :invalid_autonomy_envelope_ref
          | :invalid_autonomy_envelope_revision
          | :invalid_state
          | :invalid_valid_until
          | :invalid_fetched_at

  @doc "Construct a WorkCommission snapshot returned by Haft."
  @spec new(keyword() | map()) :: {:ok, t()} | {:error, new_error()}
  def new(attrs) when is_list(attrs), do: new(Map.new(attrs))

  def new(%{} = attrs) do
    with {:ok, id} <- required_string(attrs, :id, :invalid_id),
         {:ok, decision_ref} <- required_string(attrs, :decision_ref, :invalid_decision_ref),
         {:ok, decision_revision_hash} <-
           required_string(attrs, :decision_revision_hash, :invalid_decision_revision_hash),
         {:ok, problem_card_ref} <- required_problem_ref(attrs),
         {:ok, implementation_plan_ref} <-
           optional_string(attrs, :implementation_plan_ref, :invalid_implementation_plan_ref),
         {:ok, implementation_plan_revision} <-
           optional_string(
             attrs,
             :implementation_plan_revision,
             :invalid_implementation_plan_revision
           ),
         {:ok, scope} <- required_scope(attrs),
         {:ok, scope_hash} <- required_scope_hash(attrs),
         :ok <- validate_scope_hash_match(scope_hash, scope),
         {:ok, base_sha} <- required_string(attrs, :base_sha, :invalid_base_sha),
         :ok <- validate_base_sha_match(base_sha, scope),
         {:ok, lockset} <- required_lockset(attrs),
         :ok <- validate_lockset_match(lockset, scope),
         {:ok, evidence_requirements} <- required_evidence_requirements(attrs),
         {:ok, projection_policy} <- required_projection_policy(attrs),
         {:ok, autonomy_envelope_ref} <-
           optional_string(attrs, :autonomy_envelope_ref, :invalid_autonomy_envelope_ref),
         {:ok, autonomy_envelope_revision} <-
           optional_string(
             attrs,
             :autonomy_envelope_revision,
             :invalid_autonomy_envelope_revision
           ),
         {:ok, state} <- required_state(attrs),
         {:ok, valid_until} <- required_datetime(attrs, :valid_until, :invalid_valid_until),
         {:ok, fetched_at} <- required_datetime(attrs, :fetched_at, :invalid_fetched_at) do
      {:ok,
       %__MODULE__{
         id: id,
         decision_ref: decision_ref,
         decision_revision_hash: decision_revision_hash,
         problem_card_ref: problem_card_ref,
         implementation_plan_ref: implementation_plan_ref,
         implementation_plan_revision: implementation_plan_revision,
         scope: scope,
         scope_hash: scope_hash,
         base_sha: base_sha,
         lockset: lockset,
         evidence_requirements: evidence_requirements,
         projection_policy: projection_policy,
         autonomy_envelope_ref: autonomy_envelope_ref,
         autonomy_envelope_revision: autonomy_envelope_revision,
         state: state,
         valid_until: valid_until,
         fetched_at: fetched_at
       }}
    end
  end

  @doc """
  Build a deterministic snapshot from this commission plus the
  externally-fetched equality inputs that are not stored on the
  WorkCommission struct itself.
  """
  @spec revision_snapshot(t(), keyword() | map()) ::
          {:ok, CommissionRevisionSnapshot.t()} | {:error, CommissionRevisionSnapshot.new_error()}
  def revision_snapshot(%__MODULE__{} = commission, attrs) when is_list(attrs) do
    commission
    |> revision_snapshot(Map.new(attrs))
  end

  def revision_snapshot(%__MODULE__{} = commission, %{} = attrs) do
    attrs
    |> Map.merge(%{
      commission_id: commission.id,
      decision_ref: commission.decision_ref,
      decision_revision_hash: commission.decision_revision_hash,
      problem_card_ref: commission.problem_card_ref,
      scope_hash: commission.scope_hash,
      base_sha: commission.base_sha,
      implementation_plan_revision: commission.implementation_plan_revision,
      autonomy_envelope_revision: commission.autonomy_envelope_revision,
      projection_policy: commission.projection_policy
    })
    |> CommissionRevisionSnapshot.new()
  end

  @spec required_string(map(), atom(), new_error()) :: {:ok, String.t()} | {:error, new_error()}
  defp required_string(attrs, field, error) do
    attrs
    |> Map.get(field)
    |> validate_string(error)
  end

  @spec optional_string(map(), atom(), new_error()) ::
          {:ok, String.t() | nil} | {:error, new_error()}
  defp optional_string(attrs, field, error) do
    attrs
    |> Map.get(field)
    |> validate_optional_string(error)
  end

  @spec validate_string(term(), new_error()) :: {:ok, String.t()} | {:error, new_error()}
  defp validate_string(value, error) when is_binary(value) do
    if String.trim(value) == "" do
      {:error, error}
    else
      {:ok, value}
    end
  end

  defp validate_string(_value, error), do: {:error, error}

  @spec validate_optional_string(term(), new_error()) ::
          {:ok, String.t() | nil} | {:error, new_error()}
  defp validate_optional_string(nil, _error), do: {:ok, nil}
  defp validate_optional_string(value, error), do: validate_string(value, error)

  @spec required_problem_ref(map()) ::
          {:ok, ProblemCardRef.t()} | {:error, :invalid_problem_card_ref}
  defp required_problem_ref(attrs) do
    ref = Map.get(attrs, :problem_card_ref)

    if ProblemCardRef.valid?(ref) do
      {:ok, ref}
    else
      {:error, :invalid_problem_card_ref}
    end
  end

  @spec required_scope(map()) :: {:ok, Scope.t()} | {:error, :invalid_scope}
  defp required_scope(attrs) do
    case Map.get(attrs, :scope) do
      %Scope{} = scope -> {:ok, scope}
      _other -> {:error, :invalid_scope}
    end
  end

  @spec required_scope_hash(map()) :: {:ok, String.t()} | {:error, :invalid_scope_hash}
  defp required_scope_hash(attrs) do
    hash = Map.get(attrs, :scope_hash)

    if Scope.valid_hash?(hash) do
      {:ok, hash}
    else
      {:error, :invalid_scope_hash}
    end
  end

  @spec validate_scope_hash_match(String.t(), Scope.t()) ::
          :ok | {:error, :scope_hash_mismatch}
  defp validate_scope_hash_match(hash, %Scope{hash: hash} = scope) do
    case Scope.canonical_hash(scope) do
      {:ok, ^hash} -> :ok
      {:ok, _other_hash} -> {:error, :scope_hash_mismatch}
    end
  end

  defp validate_scope_hash_match(_hash, %Scope{}), do: {:error, :scope_hash_mismatch}

  @spec validate_base_sha_match(String.t(), Scope.t()) ::
          :ok | {:error, :base_sha_scope_mismatch}
  defp validate_base_sha_match(base_sha, %Scope{base_sha: base_sha}), do: :ok
  defp validate_base_sha_match(_base_sha, %Scope{}), do: {:error, :base_sha_scope_mismatch}

  @spec required_lockset(map()) :: {:ok, [String.t()]} | {:error, :invalid_lockset}
  defp required_lockset(attrs) do
    attrs
    |> Map.get(:lockset)
    |> validate_lockset()
  end

  @spec validate_lockset(term()) :: {:ok, [String.t()]} | {:error, :invalid_lockset}
  defp validate_lockset(values) when is_list(values) do
    values
    |> valid_lockset?()
    |> valid_lockset_result(values)
  end

  defp validate_lockset(_values), do: {:error, :invalid_lockset}

  @spec valid_lockset?([term()]) :: boolean()
  defp valid_lockset?(values) do
    values != [] and Enum.all?(values, &valid_string?/1)
  end

  @spec valid_string?(term()) :: boolean()
  defp valid_string?(value) when is_binary(value), do: String.trim(value) != ""
  defp valid_string?(_value), do: false

  @spec valid_lockset_result(boolean(), [String.t()]) ::
          {:ok, [String.t()]} | {:error, :invalid_lockset}
  defp valid_lockset_result(true, values) do
    values
    |> Enum.uniq()
    |> Enum.sort()
    |> then(&{:ok, &1})
  end

  defp valid_lockset_result(false, _values), do: {:error, :invalid_lockset}

  @spec validate_lockset_match([String.t()], Scope.t()) ::
          :ok | {:error, :lockset_scope_mismatch}
  defp validate_lockset_match(lockset, %Scope{lockset: lockset}), do: :ok
  defp validate_lockset_match(_lockset, %Scope{}), do: {:error, :lockset_scope_mismatch}

  @spec required_evidence_requirements(map()) ::
          {:ok, [term()]} | {:error, :invalid_evidence_requirements}
  defp required_evidence_requirements(attrs) do
    case Map.get(attrs, :evidence_requirements) do
      requirements when is_list(requirements) -> {:ok, requirements}
      _other -> {:error, :invalid_evidence_requirements}
    end
  end

  @spec required_projection_policy(map()) ::
          {:ok, projection_policy()} | {:error, :invalid_projection_policy}
  defp required_projection_policy(attrs) do
    policy = Map.get(attrs, :projection_policy)

    if policy in @projection_policies do
      {:ok, policy}
    else
      {:error, :invalid_projection_policy}
    end
  end

  @spec required_state(map()) :: {:ok, state()} | {:error, :invalid_state}
  defp required_state(attrs) do
    state = Map.get(attrs, :state)

    if state in @states do
      {:ok, state}
    else
      {:error, :invalid_state}
    end
  end

  @spec required_datetime(map(), atom(), new_error()) ::
          {:ok, DateTime.t()} | {:error, new_error()}
  defp required_datetime(attrs, field, error) do
    attrs
    |> Map.get(field)
    |> validate_datetime(error)
  end

  @spec validate_datetime(term(), new_error()) :: {:ok, DateTime.t()} | {:error, new_error()}
  defp validate_datetime(%DateTime{} = datetime, _error), do: {:ok, datetime}
  defp validate_datetime(_value, error), do: {:error, error}
end
