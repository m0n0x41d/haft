defmodule OpenSleigh.CommissionSource.Local do
  @moduledoc """
  Local fixture-backed `CommissionSource` adapter.

  This adapter is the local-only bootstrap path for commission-first Open-
  Sleigh. It reads JSON or YAML fixture files from `commission_source.fixture_path`
  and normalises each entry into the existing `WorkCommission` and `Scope`
  domain types. No tracker credentials or Haft server are required.
  """

  @behaviour OpenSleigh.CommissionSource

  alias OpenSleigh.{CommissionSource, Scope, WorkCommission}

  @commission_states [
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

  @projection_policies [:local_only, :external_optional, :external_required]
  @runnable_states [:queued, :ready]

  @enforce_keys [:fixture_path]
  defstruct [:fixture_path]

  @type t :: %__MODULE__{fixture_path: Path.t()}
  @type config :: map()
  @type new_error :: :fixture_path_missing | :fixture_path_invalid

  @doc "Build a local-source handle from `sleigh.md`-shaped config."
  @spec new(config()) :: {:ok, t()} | {:error, new_error()}
  def new(config) when is_map(config) do
    config
    |> fixture_path()
    |> build_handle()
  end

  @impl true
  @spec adapter_kind() :: :local
  def adapter_kind, do: :local

  @impl true
  @spec list_runnable(t()) ::
          {:ok, [WorkCommission.t()]} | {:error, CommissionSource.source_error()}
  def list_runnable(%__MODULE__{} = handle) do
    with {:ok, commissions} <- load_commissions(handle) do
      commissions
      |> Enum.filter(&runnable?/1)
      |> then(&{:ok, &1})
    end
  end

  @impl true
  @spec claim_for_preflight(t()) ::
          {:ok, WorkCommission.t()} | {:error, CommissionSource.source_error()}
  def claim_for_preflight(%__MODULE__{} = handle) do
    case list_runnable(handle) do
      {:ok, [commission | _rest]} -> claim_commission(commission)
      {:ok, []} -> {:error, :commission_not_found}
      {:error, _reason} = error -> error
    end
  end

  @impl true
  @spec claim_for_preflight(t(), String.t()) ::
          {:ok, WorkCommission.t()} | {:error, CommissionSource.source_error()}
  def claim_for_preflight(%__MODULE__{} = handle, commission_id) when is_binary(commission_id) do
    with {:ok, commissions} <- load_commissions(handle),
         {:ok, commission} <- find_commission(commissions, commission_id),
         :ok <- ensure_runnable(commission) do
      claim_commission(commission)
    end
  end

  @spec build_handle(term()) :: {:ok, t()} | {:error, new_error()}
  defp build_handle(path) when is_binary(path) do
    path
    |> String.trim()
    |> expanded_path()
  end

  defp build_handle(nil), do: {:error, :fixture_path_missing}
  defp build_handle(_path), do: {:error, :fixture_path_invalid}

  @spec expanded_path(String.t()) :: {:ok, t()} | {:error, new_error()}
  defp expanded_path(""), do: {:error, :fixture_path_missing}

  defp expanded_path(path) do
    path
    |> Path.expand()
    |> then(&{:ok, %__MODULE__{fixture_path: &1}})
  end

  @spec fixture_path(config()) :: term()
  defp fixture_path(config) do
    config
    |> commission_source_config()
    |> value_at(:fixture_path)
  end

  @spec commission_source_config(config()) :: map()
  defp commission_source_config(config) do
    case value_at(config, :commission_source) do
      %{} = source -> source
      _other -> config
    end
  end

  @spec load_commissions(t()) ::
          {:ok, [WorkCommission.t()]} | {:error, CommissionSource.source_error()}
  defp load_commissions(%__MODULE__{fixture_path: path}) do
    with {:ok, body} <- read_fixture(path),
         {:ok, payload} <- decode_fixture(path, body),
         {:ok, entries} <- commission_entries(payload) do
      parse_commissions(entries)
    end
  end

  @spec read_fixture(Path.t()) :: {:ok, String.t()} | {:error, :fixture_read_failed}
  defp read_fixture(path) do
    path
    |> File.read()
    |> read_result()
  end

  @spec read_result({:ok, String.t()} | {:error, term()}) ::
          {:ok, String.t()} | {:error, :fixture_read_failed}
  defp read_result({:ok, body}), do: {:ok, body}
  defp read_result({:error, _reason}), do: {:error, :fixture_read_failed}

  @spec decode_fixture(Path.t(), String.t()) ::
          {:ok, term()} | {:error, :fixture_parse_failed}
  defp decode_fixture(path, body) do
    path
    |> Path.extname()
    |> String.downcase()
    |> decode_by_extension(body)
  end

  @spec decode_by_extension(String.t(), String.t()) ::
          {:ok, term()} | {:error, :fixture_parse_failed}
  defp decode_by_extension(".json", body), do: body |> Jason.decode() |> decode_result()

  defp decode_by_extension(".yaml", body),
    do: body |> YamlElixir.read_from_string() |> decode_result()

  defp decode_by_extension(".yml", body),
    do: body |> YamlElixir.read_from_string() |> decode_result()

  defp decode_by_extension(_extension, body), do: body |> Jason.decode() |> decode_result()

  @spec decode_result({:ok, term()} | {:error, term()}) ::
          {:ok, term()} | {:error, :fixture_parse_failed}
  defp decode_result({:ok, payload}), do: {:ok, payload}
  defp decode_result({:error, _reason}), do: {:error, :fixture_parse_failed}

  @spec commission_entries(term()) :: {:ok, [map()]} | {:error, :fixture_payload_invalid}
  defp commission_entries(entries) when is_list(entries), do: {:ok, entries}

  defp commission_entries(%{} = payload) do
    payload
    |> value_at(:commissions)
    |> commission_entries()
  end

  defp commission_entries(_payload), do: {:error, :fixture_payload_invalid}

  @spec parse_commissions([map()]) ::
          {:ok, [WorkCommission.t()]} | {:error, CommissionSource.source_error()}
  defp parse_commissions(entries) do
    entries
    |> Enum.reduce_while({:ok, []}, &accumulate_commission/2)
    |> parsed_commissions()
  end

  @spec accumulate_commission(map(), {:ok, [WorkCommission.t()]}) ::
          {:cont, {:ok, [WorkCommission.t()]}}
          | {:halt, {:error, CommissionSource.source_error()}}
  defp accumulate_commission(entry, {:ok, commissions}) do
    case commission_from_entry(entry) do
      {:ok, commission} -> {:cont, {:ok, [commission | commissions]}}
      {:error, reason} -> {:halt, {:error, reason}}
    end
  end

  @spec parsed_commissions(
          {:ok, [WorkCommission.t()]}
          | {:error, CommissionSource.source_error()}
        ) ::
          {:ok, [WorkCommission.t()]} | {:error, CommissionSource.source_error()}
  defp parsed_commissions({:ok, commissions}) do
    commissions
    |> Enum.reverse()
    |> then(&{:ok, &1})
  end

  defp parsed_commissions({:error, _reason} = error), do: error

  @spec commission_from_entry(map()) ::
          {:ok, WorkCommission.t()} | {:error, CommissionSource.source_error()}
  defp commission_from_entry(%{} = entry) do
    with {:ok, scope} <- scope_from_entry(value_at(entry, :scope)),
         {:ok, attrs} <- commission_attrs(entry, scope) do
      WorkCommission.new(attrs)
    end
  end

  defp commission_from_entry(_entry), do: {:error, :fixture_payload_invalid}

  @spec scope_from_entry(term()) ::
          {:ok, Scope.t()} | {:error, :invalid_scope | Scope.new_error()}
  defp scope_from_entry(%{} = payload) do
    with {:ok, attrs} <- scope_attrs(payload),
         {:ok, hash} <- scope_hash(payload, attrs) do
      attrs
      |> Map.put(:hash, hash)
      |> Scope.new()
    end
  end

  defp scope_from_entry(_payload), do: {:error, :invalid_scope}

  @spec scope_attrs(map()) :: {:ok, map()} | {:error, :invalid_allowed_actions}
  defp scope_attrs(payload) do
    with {:ok, allowed_actions} <- action_set(value_at(payload, :allowed_actions)) do
      {:ok,
       %{
         repo_ref: value_at(payload, :repo_ref),
         base_sha: value_at(payload, :base_sha),
         target_branch: value_at(payload, :target_branch),
         allowed_paths: value_at(payload, :allowed_paths),
         forbidden_paths: value_at(payload, :forbidden_paths, []),
         allowed_actions: allowed_actions,
         affected_files: value_at(payload, :affected_files),
         allowed_modules: value_at(payload, :allowed_modules, []),
         lockset: value_at(payload, :lockset)
       }}
    end
  end

  @spec scope_hash(map(), map()) :: {:ok, String.t()} | {:error, Scope.new_error()}
  defp scope_hash(payload, attrs) do
    payload
    |> value_at(:hash)
    |> scope_hash_value(attrs)
  end

  @spec scope_hash_value(term(), map()) :: {:ok, String.t()} | {:error, Scope.new_error()}
  defp scope_hash_value(nil, attrs), do: Scope.canonical_hash(attrs)
  defp scope_hash_value(hash, _attrs), do: {:ok, hash}

  @spec commission_attrs(map(), Scope.t()) ::
          {:ok, map()}
          | {:error,
             :invalid_state
             | :invalid_projection_policy
             | :invalid_valid_until
             | :invalid_fetched_at}
  defp commission_attrs(entry, %Scope{} = scope) do
    with {:ok, state} <- enum_atom(value_at(entry, :state), @commission_states, :invalid_state),
         {:ok, projection_policy} <-
           enum_atom(
             value_at(entry, :projection_policy),
             @projection_policies,
             :invalid_projection_policy
           ),
         {:ok, valid_until} <- datetime(value_at(entry, :valid_until), :invalid_valid_until),
         {:ok, fetched_at} <- datetime(value_at(entry, :fetched_at), :invalid_fetched_at) do
      {:ok,
       %{
         id: value_at(entry, :id),
         decision_ref: value_at(entry, :decision_ref),
         decision_revision_hash: value_at(entry, :decision_revision_hash),
         problem_card_ref: value_at(entry, :problem_card_ref),
         implementation_plan_ref: value_at(entry, :implementation_plan_ref),
         implementation_plan_revision: value_at(entry, :implementation_plan_revision),
         scope: scope,
         scope_hash: value_at(entry, :scope_hash, scope.hash),
         base_sha: value_at(entry, :base_sha, scope.base_sha),
         lockset: value_at(entry, :lockset, scope.lockset),
         evidence_requirements: value_at(entry, :evidence_requirements),
         projection_policy: projection_policy,
         delivery_policy: value_at(entry, :delivery_policy),
         autonomy_envelope_ref: value_at(entry, :autonomy_envelope_ref),
         autonomy_envelope_revision: value_at(entry, :autonomy_envelope_revision),
         state: state,
         valid_until: valid_until,
         fetched_at: fetched_at
       }}
    end
  end

  @spec action_set(term()) :: {:ok, MapSet.t(atom())} | {:error, :invalid_allowed_actions}
  defp action_set(actions) when is_list(actions) do
    actions
    |> Enum.reduce_while({:ok, []}, &accumulate_action/2)
    |> action_set_result()
  end

  defp action_set(_actions), do: {:error, :invalid_allowed_actions}

  @spec accumulate_action(term(), {:ok, [atom()]}) ::
          {:cont, {:ok, [atom()]}} | {:halt, {:error, :invalid_allowed_actions}}
  defp accumulate_action(action, {:ok, actions}) do
    case action_atom(action) do
      {:ok, atom} -> {:cont, {:ok, [atom | actions]}}
      {:error, reason} -> {:halt, {:error, reason}}
    end
  end

  @spec action_atom(term()) :: {:ok, atom()} | {:error, :invalid_allowed_actions}
  defp action_atom(action) when is_atom(action) and not is_nil(action), do: {:ok, action}

  defp action_atom(action) when is_binary(action) do
    action
    |> String.trim()
    |> action_atom_from_string()
  end

  defp action_atom(_action), do: {:error, :invalid_allowed_actions}

  @spec action_atom_from_string(String.t()) :: {:ok, atom()} | {:error, :invalid_allowed_actions}
  defp action_atom_from_string(""), do: {:error, :invalid_allowed_actions}
  defp action_atom_from_string(action), do: {:ok, String.to_atom(action)}

  @spec action_set_result({:ok, [atom()]} | {:error, :invalid_allowed_actions}) ::
          {:ok, MapSet.t(atom())} | {:error, :invalid_allowed_actions}
  defp action_set_result({:ok, actions}) do
    actions
    |> MapSet.new()
    |> then(&{:ok, &1})
  end

  defp action_set_result({:error, _reason} = error), do: error

  @spec enum_atom(term(), [atom()], atom()) :: {:ok, atom()} | {:error, atom()}
  defp enum_atom(value, allowed, error) when is_atom(value) do
    allowed
    |> Enum.member?(value)
    |> enum_atom_member_result(value, error)
  end

  defp enum_atom(value, allowed, error) when is_binary(value) do
    value
    |> String.trim()
    |> String.downcase()
    |> enum_atom_from_string(allowed, error)
  end

  defp enum_atom(_value, _allowed, error), do: {:error, error}

  @spec enum_atom_from_string(String.t(), [atom()], atom()) :: {:ok, atom()} | {:error, atom()}
  defp enum_atom_from_string(value, allowed, error) do
    allowed
    |> Enum.find(&(Atom.to_string(&1) == value))
    |> enum_atom_result(error)
  end

  @spec enum_atom_result(atom() | nil, atom()) :: {:ok, atom()} | {:error, atom()}
  defp enum_atom_result(nil, error), do: {:error, error}
  defp enum_atom_result(value, _error), do: {:ok, value}

  @spec enum_atom_member_result(boolean(), atom(), atom()) :: {:ok, atom()} | {:error, atom()}
  defp enum_atom_member_result(true, value, _error), do: {:ok, value}
  defp enum_atom_member_result(false, _value, error), do: {:error, error}

  @spec datetime(term(), atom()) :: {:ok, DateTime.t()} | {:error, atom()}
  defp datetime(%DateTime{} = datetime, _error), do: {:ok, datetime}

  defp datetime(value, error) when is_binary(value) do
    value
    |> DateTime.from_iso8601()
    |> datetime_result(error)
  end

  defp datetime(_value, error), do: {:error, error}

  @spec datetime_result({:ok, DateTime.t(), integer()} | {:error, term()}, atom()) ::
          {:ok, DateTime.t()} | {:error, atom()}
  defp datetime_result({:ok, datetime, _offset}, _error), do: {:ok, datetime}
  defp datetime_result({:error, _reason}, error), do: {:error, error}

  @spec runnable?(WorkCommission.t()) :: boolean()
  defp runnable?(%WorkCommission{} = commission) do
    commission.state in @runnable_states and
      DateTime.compare(commission.valid_until, DateTime.utc_now()) == :gt
  end

  @spec find_commission([WorkCommission.t()], String.t()) ::
          {:ok, WorkCommission.t()} | {:error, :commission_not_found}
  defp find_commission(commissions, commission_id) do
    commissions
    |> Enum.find(&(&1.id == commission_id))
    |> find_commission_result()
  end

  @spec find_commission_result(WorkCommission.t() | nil) ::
          {:ok, WorkCommission.t()} | {:error, :commission_not_found}
  defp find_commission_result(nil), do: {:error, :commission_not_found}
  defp find_commission_result(%WorkCommission{} = commission), do: {:ok, commission}

  @spec ensure_runnable(WorkCommission.t()) :: :ok | {:error, :commission_not_runnable}
  defp ensure_runnable(%WorkCommission{} = commission) do
    commission
    |> runnable?()
    |> runnable_result()
  end

  @spec runnable_result(boolean()) :: :ok | {:error, :commission_not_runnable}
  defp runnable_result(true), do: :ok
  defp runnable_result(false), do: {:error, :commission_not_runnable}

  @spec claim_commission(WorkCommission.t()) ::
          {:ok, WorkCommission.t()} | {:error, WorkCommission.new_error()}
  defp claim_commission(%WorkCommission{} = commission) do
    commission
    |> Map.from_struct()
    |> Map.put(:state, :preflighting)
    |> WorkCommission.new()
  end

  @spec value_at(term(), atom()) :: term()
  defp value_at(value, key), do: value_at(value, key, nil)

  @spec value_at(term(), atom(), term()) :: term()
  defp value_at(%{} = map, key, fallback) do
    Map.get(map, Atom.to_string(key), Map.get(map, key, fallback))
  end

  defp value_at(_value, _key, fallback), do: fallback
end
