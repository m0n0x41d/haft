defmodule OpenSleigh.Scope do
  @moduledoc """
  Closed authorization object for one WorkCommission.

  Scope is not prompt context. It is the domain object that says which
  repo, branch, paths, actions, modules, and lockset a runner may use.
  The constructor verifies the caller-supplied `hash` against the
  canonical serialized form of every authorization field.
  """

  alias OpenSleigh.ConfigHash

  @hash_length 64

  @enforce_keys [
    :repo_ref,
    :base_sha,
    :target_branch,
    :allowed_paths,
    :forbidden_paths,
    :allowed_actions,
    :affected_files,
    :allowed_modules,
    :lockset,
    :hash
  ]
  defstruct [
    :repo_ref,
    :base_sha,
    :target_branch,
    :allowed_paths,
    :forbidden_paths,
    :allowed_actions,
    :affected_files,
    :allowed_modules,
    :lockset,
    :hash
  ]

  @type action :: :edit_files | :run_tests | :commit | atom()

  @type t :: %__MODULE__{
          repo_ref: String.t(),
          base_sha: String.t(),
          target_branch: String.t(),
          allowed_paths: [String.t()],
          forbidden_paths: [String.t()],
          allowed_actions: MapSet.t(action()),
          affected_files: [String.t()],
          allowed_modules: [String.t()],
          lockset: [String.t()],
          hash: String.t()
        }

  @type new_error ::
          :invalid_repo_ref
          | :invalid_base_sha
          | :invalid_target_branch
          | :invalid_allowed_paths
          | :invalid_forbidden_paths
          | :invalid_allowed_actions
          | :invalid_affected_files
          | :invalid_allowed_modules
          | :invalid_lockset
          | :invalid_hash
          | :scope_hash_mismatch

  @doc """
  Construct a Scope from a map or keyword list.

  Required collection fields must be present. `forbidden_paths` may be
  an empty list; `allowed_modules` is optional and canonicalizes to an
  empty list when absent.
  """
  @spec new(keyword() | map()) :: {:ok, t()} | {:error, new_error()}
  def new(attrs) when is_list(attrs), do: new(Map.new(attrs))

  def new(%{} = attrs) do
    with {:ok, canonical} <- canonical_fields(attrs),
         :ok <- validate_hash(attrs[:hash]),
         :ok <- validate_hash_match(attrs[:hash], canonical) do
      {:ok,
       %__MODULE__{
         repo_ref: canonical.repo_ref,
         base_sha: canonical.base_sha,
         target_branch: canonical.target_branch,
         allowed_paths: canonical.allowed_paths,
         forbidden_paths: canonical.forbidden_paths,
         allowed_actions: canonical.allowed_actions,
         affected_files: canonical.affected_files,
         allowed_modules: canonical.allowed_modules,
         lockset: canonical.lockset,
         hash: attrs.hash
       }}
    end
  end

  @doc """
  Compute the canonical sha256 hash for a Scope-shaped map, keyword
  list, or `%Scope{}`. The `:hash` field itself is never included.
  """
  @spec canonical_hash(keyword() | map() | t()) :: {:ok, String.t()} | {:error, new_error()}
  def canonical_hash(attrs) when is_list(attrs), do: attrs |> Map.new() |> canonical_hash()

  def canonical_hash(%__MODULE__{} = scope) do
    scope
    |> Map.from_struct()
    |> canonical_hash()
  end

  def canonical_hash(%{} = attrs) do
    with {:ok, canonical} <- canonical_fields(attrs) do
      canonical
      |> canonical_payload()
      |> ConfigHash.from_iodata()
      |> then(&{:ok, &1})
    end
  end

  @doc "Runtime shape check for scope hashes."
  @spec valid_hash?(term()) :: boolean()
  def valid_hash?(hash) when is_binary(hash) and byte_size(hash) == @hash_length do
    String.match?(hash, ~r/^[0-9a-f]{64}$/)
  end

  def valid_hash?(_), do: false

  @spec canonical_fields(map()) :: {:ok, map()} | {:error, new_error()}
  defp canonical_fields(attrs) do
    with {:ok, repo_ref} <- required_string(attrs, :repo_ref, :invalid_repo_ref),
         {:ok, base_sha} <- required_string(attrs, :base_sha, :invalid_base_sha),
         {:ok, target_branch} <- required_string(attrs, :target_branch, :invalid_target_branch),
         {:ok, allowed_paths} <-
           required_string_list(attrs, :allowed_paths, :invalid_allowed_paths, :non_empty),
         {:ok, forbidden_paths} <-
           required_string_list(attrs, :forbidden_paths, :invalid_forbidden_paths, :allow_empty),
         {:ok, allowed_actions} <- required_action_set(attrs, :allowed_actions),
         {:ok, affected_files} <-
           required_string_list(attrs, :affected_files, :invalid_affected_files, :non_empty),
         {:ok, allowed_modules} <-
           optional_string_list(attrs, :allowed_modules, :invalid_allowed_modules),
         {:ok, lockset} <- required_string_list(attrs, :lockset, :invalid_lockset, :non_empty) do
      {:ok,
       %{
         repo_ref: repo_ref,
         base_sha: base_sha,
         target_branch: target_branch,
         allowed_paths: allowed_paths,
         forbidden_paths: forbidden_paths,
         allowed_actions: allowed_actions,
         affected_files: affected_files,
         allowed_modules: allowed_modules,
         lockset: lockset
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

  @spec required_string_list(map(), atom(), new_error(), :allow_empty | :non_empty) ::
          {:ok, [String.t()]} | {:error, new_error()}
  defp required_string_list(attrs, field, error, emptiness) do
    attrs
    |> Map.get(field)
    |> validate_string_list(error, emptiness)
  end

  @spec optional_string_list(map(), atom(), new_error()) ::
          {:ok, [String.t()]} | {:error, new_error()}
  defp optional_string_list(attrs, field, error) do
    attrs
    |> Map.get(field, [])
    |> validate_string_list(error, :allow_empty)
  end

  @spec validate_string_list(term(), new_error(), :allow_empty | :non_empty) ::
          {:ok, [String.t()]} | {:error, new_error()}
  defp validate_string_list(values, error, emptiness) when is_list(values) do
    values
    |> valid_string_list?(emptiness)
    |> valid_list_result(values, error)
  end

  defp validate_string_list(_values, error, _emptiness), do: {:error, error}

  @spec valid_string_list?([term()], :allow_empty | :non_empty) :: boolean()
  defp valid_string_list?(values, :allow_empty) do
    Enum.all?(values, &valid_string?/1)
  end

  defp valid_string_list?(values, :non_empty) do
    values != [] and Enum.all?(values, &valid_string?/1)
  end

  @spec valid_string?(term()) :: boolean()
  defp valid_string?(value) when is_binary(value), do: String.trim(value) != ""
  defp valid_string?(_value), do: false

  @spec valid_list_result(boolean(), [String.t()], new_error()) ::
          {:ok, [String.t()]} | {:error, new_error()}
  defp valid_list_result(true, values, _error) do
    values
    |> Enum.uniq()
    |> Enum.sort()
    |> then(&{:ok, &1})
  end

  defp valid_list_result(false, _values, error), do: {:error, error}

  @spec required_action_set(map(), atom()) ::
          {:ok, MapSet.t(action())} | {:error, :invalid_allowed_actions}
  defp required_action_set(attrs, field) do
    attrs
    |> Map.get(field)
    |> validate_action_set()
  end

  @spec validate_action_set(term()) ::
          {:ok, MapSet.t(action())} | {:error, :invalid_allowed_actions}
  defp validate_action_set(%MapSet{} = actions) do
    actions
    |> valid_action_set?()
    |> valid_action_result(actions)
  end

  defp validate_action_set(_actions), do: {:error, :invalid_allowed_actions}

  @spec valid_action_set?(MapSet.t(term())) :: boolean()
  defp valid_action_set?(actions) do
    actions != MapSet.new() and Enum.all?(actions, &(is_atom(&1) and not is_nil(&1)))
  end

  @spec valid_action_result(boolean(), MapSet.t(action())) ::
          {:ok, MapSet.t(action())} | {:error, :invalid_allowed_actions}
  defp valid_action_result(true, actions), do: {:ok, actions}
  defp valid_action_result(false, _actions), do: {:error, :invalid_allowed_actions}

  @spec validate_hash(term()) :: :ok | {:error, :invalid_hash}
  defp validate_hash(hash) do
    if valid_hash?(hash) do
      :ok
    else
      {:error, :invalid_hash}
    end
  end

  @spec validate_hash_match(String.t(), map()) :: :ok | {:error, :scope_hash_mismatch}
  defp validate_hash_match(hash, canonical) do
    expected_hash =
      canonical
      |> canonical_payload()
      |> ConfigHash.from_iodata()

    if hash == expected_hash do
      :ok
    else
      {:error, :scope_hash_mismatch}
    end
  end

  @spec canonical_payload(map()) :: iodata()
  defp canonical_payload(canonical) do
    [
      "{",
      ~s("affected_files":),
      Jason.encode!(canonical.affected_files),
      ~s(,"allowed_actions":),
      Jason.encode!(canonical_action_names(canonical.allowed_actions)),
      ~s(,"allowed_modules":),
      Jason.encode!(canonical.allowed_modules),
      ~s(,"allowed_paths":),
      Jason.encode!(canonical.allowed_paths),
      ~s(,"base_sha":),
      Jason.encode!(canonical.base_sha),
      ~s(,"forbidden_paths":),
      Jason.encode!(canonical.forbidden_paths),
      ~s(,"lockset":),
      Jason.encode!(canonical.lockset),
      ~s(,"repo_ref":),
      Jason.encode!(canonical.repo_ref),
      ~s(,"target_branch":),
      Jason.encode!(canonical.target_branch),
      "}"
    ]
  end

  @spec canonical_action_names(MapSet.t(action())) :: [String.t()]
  defp canonical_action_names(actions) do
    actions
    |> Enum.map(&Atom.to_string/1)
    |> Enum.sort()
  end
end
