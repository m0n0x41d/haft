defmodule OpenSleigh.CommissionRevisionSnapshot do
  @moduledoc """
  Deterministic equality set frozen around a WorkCommission lease.

  This type carries only the fields that deterministic preflight must
  compare before Execute. It is not runtime evidence and does not infer
  semantic freshness from prose.
  """

  alias OpenSleigh.{ConfigHash, ProblemCardRef, Scope}

  @projection_policies [:local_only, :external_optional, :external_required]
  @hash_length 64

  @enforce_keys [
    :commission_id,
    :decision_ref,
    :decision_revision_hash,
    :problem_card_ref,
    :problem_revision_hash,
    :spec_section_refs,
    :spec_revision_hashes,
    :scope_hash,
    :base_sha,
    :implementation_plan_revision,
    :autonomy_envelope_revision,
    :projection_policy,
    :lease_id,
    :lease_state,
    :hash
  ]
  defstruct [
    :commission_id,
    :decision_ref,
    :decision_revision_hash,
    :problem_card_ref,
    :problem_revision_hash,
    :spec_section_refs,
    :spec_revision_hashes,
    :scope_hash,
    :base_sha,
    :implementation_plan_revision,
    :autonomy_envelope_revision,
    :projection_policy,
    :lease_id,
    :lease_state,
    :hash
  ]

  @type t :: %__MODULE__{
          commission_id: String.t(),
          decision_ref: String.t(),
          decision_revision_hash: String.t(),
          problem_card_ref: ProblemCardRef.t(),
          problem_revision_hash: String.t(),
          spec_section_refs: [String.t()],
          spec_revision_hashes: %{optional(String.t()) => String.t()},
          scope_hash: String.t(),
          base_sha: String.t(),
          implementation_plan_revision: String.t() | nil,
          autonomy_envelope_revision: String.t() | nil,
          projection_policy: :local_only | :external_optional | :external_required,
          lease_id: String.t() | nil,
          lease_state: atom(),
          hash: String.t()
        }

  @type new_error ::
          :invalid_commission_id
          | :invalid_decision_ref
          | :invalid_decision_revision_hash
          | :invalid_problem_card_ref
          | :invalid_problem_revision_hash
          | :invalid_spec_section_refs
          | :invalid_spec_revision_hashes
          | :invalid_scope_hash
          | :invalid_base_sha
          | :missing_implementation_plan_revision
          | :invalid_implementation_plan_revision
          | :missing_autonomy_envelope_revision
          | :invalid_autonomy_envelope_revision
          | :invalid_projection_policy
          | :missing_lease_id
          | :invalid_lease_id
          | :invalid_lease_state

  @doc """
  Construct a CommissionRevisionSnapshot.

  `implementation_plan_revision`, `autonomy_envelope_revision`, and
  `lease_id` are deterministic equality inputs, so their keys must be
  present even when the value is `nil`.
  """
  @spec new(keyword() | map()) :: {:ok, t()} | {:error, new_error()}
  def new(attrs) when is_list(attrs), do: new(Map.new(attrs))

  def new(%{} = attrs) do
    with {:ok, canonical} <- canonical_fields(attrs) do
      {:ok,
       %__MODULE__{
         commission_id: canonical.commission_id,
         decision_ref: canonical.decision_ref,
         decision_revision_hash: canonical.decision_revision_hash,
         problem_card_ref: canonical.problem_card_ref,
         problem_revision_hash: canonical.problem_revision_hash,
         spec_section_refs: canonical.spec_section_refs,
         spec_revision_hashes: canonical.spec_revision_hashes,
         scope_hash: canonical.scope_hash,
         base_sha: canonical.base_sha,
         implementation_plan_revision: canonical.implementation_plan_revision,
         autonomy_envelope_revision: canonical.autonomy_envelope_revision,
         projection_policy: canonical.projection_policy,
         lease_id: canonical.lease_id,
         lease_state: canonical.lease_state,
         hash: snapshot_hash(canonical)
       }}
    end
  end

  @doc "Compare two snapshots by their canonical snapshot hash."
  @spec equal?(t(), t()) :: boolean()
  def equal?(%__MODULE__{} = left, %__MODULE__{} = right), do: left.hash == right.hash

  @doc "Runtime shape check for snapshot hashes."
  @spec valid_hash?(term()) :: boolean()
  def valid_hash?(hash) when is_binary(hash) and byte_size(hash) == @hash_length do
    String.match?(hash, ~r/^[0-9a-f]{64}$/)
  end

  def valid_hash?(_hash), do: false

  @spec canonical_fields(map()) :: {:ok, map()} | {:error, new_error()}
  defp canonical_fields(attrs) do
    with {:ok, commission_id} <- required_string(attrs, :commission_id, :invalid_commission_id),
         {:ok, decision_ref} <- required_string(attrs, :decision_ref, :invalid_decision_ref),
         {:ok, decision_revision_hash} <-
           required_string(attrs, :decision_revision_hash, :invalid_decision_revision_hash),
         {:ok, problem_card_ref} <- required_problem_ref(attrs),
         {:ok, problem_revision_hash} <-
           required_string(attrs, :problem_revision_hash, :invalid_problem_revision_hash),
         {:ok, spec_section_refs} <- optional_string_list(attrs, :spec_section_refs),
         {:ok, spec_revision_hashes} <- optional_string_map(attrs, :spec_revision_hashes),
         {:ok, scope_hash} <- required_scope_hash(attrs),
         {:ok, base_sha} <- required_string(attrs, :base_sha, :invalid_base_sha),
         {:ok, implementation_plan_revision} <-
           optional_string_input(
             attrs,
             :implementation_plan_revision,
             :missing_implementation_plan_revision,
             :invalid_implementation_plan_revision
           ),
         {:ok, autonomy_envelope_revision} <-
           optional_string_input(
             attrs,
             :autonomy_envelope_revision,
             :missing_autonomy_envelope_revision,
             :invalid_autonomy_envelope_revision
           ),
         {:ok, projection_policy} <- required_projection_policy(attrs),
         {:ok, lease_id} <-
           optional_string_input(attrs, :lease_id, :missing_lease_id, :invalid_lease_id),
         {:ok, lease_state} <- required_lease_state(attrs) do
      {:ok,
       %{
         commission_id: commission_id,
         decision_ref: decision_ref,
         decision_revision_hash: decision_revision_hash,
         problem_card_ref: problem_card_ref,
         problem_revision_hash: problem_revision_hash,
         spec_section_refs: spec_section_refs,
         spec_revision_hashes: spec_revision_hashes,
         scope_hash: scope_hash,
         base_sha: base_sha,
         implementation_plan_revision: implementation_plan_revision,
         autonomy_envelope_revision: autonomy_envelope_revision,
         projection_policy: projection_policy,
         lease_id: lease_id,
         lease_state: lease_state
       }}
    end
  end

  @spec required_string(map(), atom(), new_error()) :: {:ok, String.t()} | {:error, new_error()}
  defp required_string(attrs, field, error) do
    attrs
    |> Map.get(field)
    |> validate_string(error)
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

  @spec required_scope_hash(map()) :: {:ok, String.t()} | {:error, :invalid_scope_hash}
  defp required_scope_hash(attrs) do
    hash = Map.get(attrs, :scope_hash)

    if Scope.valid_hash?(hash) do
      {:ok, hash}
    else
      {:error, :invalid_scope_hash}
    end
  end

  @spec optional_string_input(map(), atom(), new_error(), new_error()) ::
          {:ok, String.t() | nil} | {:error, new_error()}
  defp optional_string_input(attrs, field, missing_error, invalid_error) do
    attrs
    |> Map.has_key?(field)
    |> optional_string_value(attrs, field, missing_error, invalid_error)
  end

  @spec optional_string_value(boolean(), map(), atom(), new_error(), new_error()) ::
          {:ok, String.t() | nil} | {:error, new_error()}
  defp optional_string_value(false, _attrs, _field, missing_error, _invalid_error) do
    {:error, missing_error}
  end

  defp optional_string_value(true, attrs, field, _missing_error, invalid_error) do
    attrs
    |> Map.get(field)
    |> validate_optional_string(invalid_error)
  end

  @spec validate_optional_string(term(), new_error()) ::
          {:ok, String.t() | nil} | {:error, new_error()}
  defp validate_optional_string(nil, _error), do: {:ok, nil}
  defp validate_optional_string(value, error), do: validate_string(value, error)

  @spec required_projection_policy(map()) ::
          {:ok, :local_only | :external_optional | :external_required}
          | {:error, :invalid_projection_policy}
  defp required_projection_policy(attrs) do
    policy = Map.get(attrs, :projection_policy)

    if policy in @projection_policies do
      {:ok, policy}
    else
      {:error, :invalid_projection_policy}
    end
  end

  @spec required_lease_state(map()) :: {:ok, atom()} | {:error, :invalid_lease_state}
  defp required_lease_state(attrs) do
    lease_state = Map.get(attrs, :lease_state)

    if is_atom(lease_state) and not is_nil(lease_state) do
      {:ok, lease_state}
    else
      {:error, :invalid_lease_state}
    end
  end

  @spec snapshot_hash(map()) :: String.t()
  defp snapshot_hash(canonical) do
    canonical
    |> snapshot_payload()
    |> ConfigHash.from_iodata()
  end

  @spec snapshot_payload(map()) :: iodata()
  defp snapshot_payload(canonical) do
    [
      "{",
      ~s("autonomy_envelope_revision":),
      Jason.encode!(canonical.autonomy_envelope_revision),
      ~s(,"base_sha":),
      Jason.encode!(canonical.base_sha),
      ~s(,"commission_id":),
      Jason.encode!(canonical.commission_id),
      ~s(,"decision_ref":),
      Jason.encode!(canonical.decision_ref),
      ~s(,"decision_revision_hash":),
      Jason.encode!(canonical.decision_revision_hash),
      ~s(,"implementation_plan_revision":),
      Jason.encode!(canonical.implementation_plan_revision),
      ~s(,"lease_id":),
      Jason.encode!(canonical.lease_id),
      ~s(,"lease_state":),
      Jason.encode!(Atom.to_string(canonical.lease_state)),
      ~s(,"problem_card_ref":),
      Jason.encode!(canonical.problem_card_ref),
      ~s(,"problem_revision_hash":),
      Jason.encode!(canonical.problem_revision_hash),
      ~s(,"projection_policy":),
      Jason.encode!(Atom.to_string(canonical.projection_policy)),
      ~s(,"scope_hash":),
      Jason.encode!(canonical.scope_hash),
      ~s(,"spec_revision_hashes":),
      Jason.encode!(sorted_string_pairs(canonical.spec_revision_hashes)),
      ~s(,"spec_section_refs":),
      Jason.encode!(canonical.spec_section_refs),
      "}"
    ]
  end

  @spec sorted_string_pairs(map()) :: [[String.t()]]
  defp sorted_string_pairs(values) do
    values
    |> Enum.sort()
    |> Enum.map(fn {key, value} -> [key, value] end)
  end

  @spec optional_string_list(map(), atom()) ::
          {:ok, [String.t()]} | {:error, :invalid_spec_section_refs}
  defp optional_string_list(attrs, field) do
    attrs
    |> Map.get(field, [])
    |> validate_string_list()
  end

  @spec validate_string_list(term()) :: {:ok, [String.t()]} | {:error, :invalid_spec_section_refs}
  defp validate_string_list(values) when is_list(values) do
    values
    |> Enum.reduce_while({:ok, []}, &accumulate_string/2)
    |> string_list_result()
  end

  defp validate_string_list(_values), do: {:error, :invalid_spec_section_refs}

  @spec accumulate_string(term(), {:ok, [String.t()]}) ::
          {:cont, {:ok, [String.t()]}} | {:halt, {:error, :invalid_spec_section_refs}}
  defp accumulate_string(value, {:ok, values}) when is_binary(value) do
    value
    |> String.trim()
    |> accumulate_clean_string(values)
  end

  defp accumulate_string(_value, _values), do: {:halt, {:error, :invalid_spec_section_refs}}

  @spec accumulate_clean_string(String.t(), [String.t()]) ::
          {:cont, {:ok, [String.t()]}} | {:halt, {:error, :invalid_spec_section_refs}}
  defp accumulate_clean_string("", _values), do: {:halt, {:error, :invalid_spec_section_refs}}
  defp accumulate_clean_string(value, values), do: {:cont, {:ok, [value | values]}}

  @spec string_list_result({:ok, [String.t()]} | {:error, :invalid_spec_section_refs}) ::
          {:ok, [String.t()]} | {:error, :invalid_spec_section_refs}
  defp string_list_result({:ok, values}) do
    values
    |> Enum.uniq()
    |> Enum.sort()
    |> then(&{:ok, &1})
  end

  defp string_list_result({:error, _reason} = error), do: error

  @spec optional_string_map(map(), atom()) ::
          {:ok, %{optional(String.t()) => String.t()}} | {:error, :invalid_spec_revision_hashes}
  defp optional_string_map(attrs, field) do
    attrs
    |> Map.get(field, %{})
    |> validate_string_map()
  end

  @spec validate_string_map(term()) ::
          {:ok, %{optional(String.t()) => String.t()}} | {:error, :invalid_spec_revision_hashes}
  defp validate_string_map(values) when is_map(values) do
    values
    |> Enum.reduce_while({:ok, %{}}, &accumulate_string_pair/2)
  end

  defp validate_string_map(_values), do: {:error, :invalid_spec_revision_hashes}

  @spec accumulate_string_pair({term(), term()}, {:ok, map()}) ::
          {:cont, {:ok, map()}} | {:halt, {:error, :invalid_spec_revision_hashes}}
  defp accumulate_string_pair({key, value}, {:ok, values})
       when is_binary(key) and is_binary(value) do
    clean_key = String.trim(key)
    clean_value = String.trim(value)

    if clean_key == "" or clean_value == "" do
      {:halt, {:error, :invalid_spec_revision_hashes}}
    else
      {:cont, {:ok, Map.put(values, clean_key, clean_value)}}
    end
  end

  defp accumulate_string_pair(_pair, _values),
    do: {:halt, {:error, :invalid_spec_revision_hashes}}
end
