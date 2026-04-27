defmodule OpenSleigh.AutonomyEnvelope do
  @moduledoc """
  Human-approved bounds for batch/YOLO continuation.

  This object is a limiter only. It cannot grant permission outside the
  WorkCommission Scope and cannot skip freshness, scope, evidence, lease,
  lockset, or one-way-door gates.
  """

  @states [:draft, :approved, :active, :exhausted, :revoked, :expired]
  @active_states [:approved, :active]
  @failure_strategies [:block_plan, :block_node, :continue_independent]
  @required_gates [:freshness, :scope, :evidence]

  @enforce_keys [
    :ref,
    :revision,
    :state,
    :allowed_repos,
    :allowed_paths,
    :forbidden_paths,
    :allowed_actions,
    :allowed_modules,
    :forbidden_actions,
    :forbidden_one_way_door_actions,
    :max_concurrency,
    :commission_budget,
    :active_concurrency,
    :consumed_commissions,
    :on_failure,
    :on_stale,
    :valid_until,
    :revoked_at,
    :required_gates
  ]
  defstruct [
    :ref,
    :revision,
    :state,
    :allowed_repos,
    :allowed_paths,
    :forbidden_paths,
    :allowed_actions,
    :allowed_modules,
    :forbidden_actions,
    :forbidden_one_way_door_actions,
    :max_concurrency,
    :commission_budget,
    :active_concurrency,
    :consumed_commissions,
    :on_failure,
    :on_stale,
    :valid_until,
    :revoked_at,
    :required_gates
  ]

  @type t :: %__MODULE__{
          ref: String.t(),
          revision: String.t(),
          state: atom(),
          allowed_repos: [String.t()],
          allowed_paths: [String.t()],
          forbidden_paths: [String.t()],
          allowed_actions: MapSet.t(atom()),
          allowed_modules: [String.t()],
          forbidden_actions: MapSet.t(atom()),
          forbidden_one_way_door_actions: MapSet.t(atom()),
          max_concurrency: pos_integer(),
          commission_budget: pos_integer(),
          active_concurrency: non_neg_integer(),
          consumed_commissions: non_neg_integer(),
          on_failure: atom(),
          on_stale: atom() | nil,
          valid_until: DateTime.t(),
          revoked_at: DateTime.t() | nil,
          required_gates: [atom()]
        }

  @type new_error ::
          :invalid_autonomy_envelope_ref
          | :invalid_autonomy_envelope_revision
          | :invalid_autonomy_envelope_state
          | :invalid_allowed_repos
          | :invalid_allowed_paths
          | :invalid_forbidden_paths
          | :invalid_allowed_actions
          | :invalid_allowed_modules
          | :invalid_forbidden_actions
          | :invalid_forbidden_one_way_door_actions
          | :invalid_max_concurrency
          | :invalid_commission_budget
          | :invalid_active_concurrency
          | :invalid_consumed_commissions
          | :invalid_on_failure
          | :invalid_on_stale
          | :invalid_valid_until
          | :invalid_revoked_at
          | :gate_skip_forbidden
          | :required_gate_missing

  @type allow_error ::
          :autonomy_envelope_missing
          | :autonomy_envelope_not_active
          | :autonomy_envelope_expired
          | :autonomy_envelope_revoked
          | :autonomy_envelope_exhausted
          | :repo_outside_autonomy_envelope
          | :path_outside_autonomy_envelope
          | :action_outside_autonomy_envelope
          | :action_forbidden_by_autonomy_envelope
          | :one_way_door_action_forbidden
          | :module_outside_autonomy_envelope
          | :autonomy_envelope_concurrency_exhausted
          | :autonomy_envelope_commission_budget_exhausted

  @doc "Construct an envelope snapshot from a Haft payload."
  @spec new(keyword() | map()) :: {:ok, t()} | {:error, new_error()}
  def new(attrs) when is_list(attrs), do: attrs |> Map.new() |> new()

  def new(%{} = attrs) do
    with :ok <- reject_gate_skips(attrs),
         {:ok, ref} <- required_string(attrs, :ref, :invalid_autonomy_envelope_ref),
         {:ok, revision} <-
           required_string(attrs, :revision, :invalid_autonomy_envelope_revision),
         {:ok, state} <-
           enum_atom(value_at(attrs, :state), @states, :invalid_autonomy_envelope_state),
         {:ok, allowed_repos} <-
           required_string_list(attrs, :allowed_repos, :invalid_allowed_repos),
         {:ok, allowed_paths} <-
           required_string_list(attrs, :allowed_paths, :invalid_allowed_paths),
         {:ok, forbidden_paths} <-
           optional_string_list(attrs, :forbidden_paths, :invalid_forbidden_paths),
         {:ok, allowed_actions} <-
           required_action_set(value_at(attrs, :allowed_actions), :invalid_allowed_actions),
         {:ok, allowed_modules} <-
           optional_string_list(attrs, :allowed_modules, :invalid_allowed_modules),
         {:ok, forbidden_actions} <-
           action_set(value_at(attrs, :forbidden_actions, []), :invalid_forbidden_actions),
         {:ok, forbidden_one_way_door_actions} <-
           action_set(
             value_at(attrs, :forbidden_one_way_door_actions, []),
             :invalid_forbidden_one_way_door_actions
           ),
         {:ok, max_concurrency} <-
           positive_integer(attrs, :max_concurrency, :invalid_max_concurrency),
         {:ok, commission_budget} <- commission_budget(attrs),
         {:ok, active_concurrency} <-
           non_negative_integer(attrs, :active_concurrency, :invalid_active_concurrency),
         {:ok, consumed_commissions} <-
           non_negative_integer(attrs, :consumed_commissions, :invalid_consumed_commissions),
         {:ok, on_failure} <-
           enum_atom(value_at(attrs, :on_failure), @failure_strategies, :invalid_on_failure),
         {:ok, on_stale} <- optional_failure_strategy(attrs),
         {:ok, valid_until} <- datetime(value_at(attrs, :valid_until), :invalid_valid_until),
         {:ok, revoked_at} <-
           optional_datetime(value_at(attrs, :revoked_at), :invalid_revoked_at),
         {:ok, required_gates} <- required_gates(attrs) do
      {:ok,
       %__MODULE__{
         ref: ref,
         revision: revision,
         state: state,
         allowed_repos: allowed_repos,
         allowed_paths: allowed_paths,
         forbidden_paths: forbidden_paths,
         allowed_actions: allowed_actions,
         allowed_modules: allowed_modules,
         forbidden_actions: forbidden_actions,
         forbidden_one_way_door_actions: forbidden_one_way_door_actions,
         max_concurrency: max_concurrency,
         commission_budget: commission_budget,
         active_concurrency: active_concurrency,
         consumed_commissions: consumed_commissions,
         on_failure: on_failure,
         on_stale: on_stale,
         valid_until: valid_until,
         revoked_at: revoked_at,
         required_gates: required_gates
       }}
    end
  end

  @doc "Check whether this envelope admits a WorkCommission at preflight time."
  @spec allow_commission(t() | nil, OpenSleigh.WorkCommission.t(), DateTime.t(), boolean()) ::
          :ok | {:error, allow_error()}
  def allow_commission(nil, _commission, _checked_at, true),
    do: {:error, :autonomy_envelope_missing}

  def allow_commission(nil, _commission, _checked_at, false), do: :ok

  def allow_commission(%__MODULE__{} = envelope, commission, %DateTime{} = checked_at, _required) do
    with :ok <- active_state(envelope),
         :ok <- not_expired(envelope, checked_at),
         :ok <- not_revoked(envelope, checked_at),
         :ok <- repo_allowed(envelope, commission),
         :ok <- paths_allowed(envelope, commission),
         :ok <- actions_allowed(envelope, commission),
         :ok <- modules_allowed(envelope, commission),
         :ok <- budget_available(envelope) do
      :ok
    end
  end

  @spec reject_gate_skips(map()) :: :ok | {:error, :gate_skip_forbidden}
  defp reject_gate_skips(attrs) do
    attrs
    |> value_at(:skip_gates, [])
    |> reject_gate_skips_result()
  end

  @spec reject_gate_skips_result(term()) :: :ok | {:error, :gate_skip_forbidden}
  defp reject_gate_skips_result([]), do: :ok
  defp reject_gate_skips_result(nil), do: :ok
  defp reject_gate_skips_result(_value), do: {:error, :gate_skip_forbidden}

  @spec required_string(map(), atom(), new_error()) :: {:ok, String.t()} | {:error, new_error()}
  defp required_string(attrs, field, error) do
    attrs
    |> value_at(field)
    |> validate_string(error)
  end

  @spec validate_string(term(), new_error()) :: {:ok, String.t()} | {:error, new_error()}
  defp validate_string(value, error) when is_binary(value) do
    value
    |> String.trim()
    |> validate_clean_string(error)
  end

  defp validate_string(_value, error), do: {:error, error}

  @spec validate_clean_string(String.t(), new_error()) ::
          {:ok, String.t()} | {:error, new_error()}
  defp validate_clean_string("", error), do: {:error, error}
  defp validate_clean_string(value, _error), do: {:ok, value}

  @spec required_string_list(map(), atom(), new_error()) ::
          {:ok, [String.t()]} | {:error, new_error()}
  defp required_string_list(attrs, field, error) do
    attrs
    |> value_at(field)
    |> string_list(error, :non_empty)
  end

  @spec optional_string_list(map(), atom(), new_error()) ::
          {:ok, [String.t()]} | {:error, new_error()}
  defp optional_string_list(attrs, field, error) do
    attrs
    |> value_at(field, [])
    |> string_list(error, :allow_empty)
  end

  @spec string_list(term(), new_error(), :allow_empty | :non_empty) ::
          {:ok, [String.t()]} | {:error, new_error()}
  defp string_list(values, error, emptiness) when is_list(values) do
    values
    |> Enum.reduce_while({:ok, []}, &accumulate_string(error, &1, &2))
    |> string_list_result(error, emptiness)
  end

  defp string_list(_values, error, _emptiness), do: {:error, error}

  @spec accumulate_string(new_error(), term(), {:ok, [String.t()]}) ::
          {:cont, {:ok, [String.t()]}} | {:halt, {:error, new_error()}}
  defp accumulate_string(error, value, {:ok, values}) when is_binary(value) do
    value
    |> String.trim()
    |> accumulate_clean_string(error, values)
  end

  defp accumulate_string(error, _value, _values), do: {:halt, {:error, error}}

  @spec accumulate_clean_string(String.t(), new_error(), [String.t()]) ::
          {:cont, {:ok, [String.t()]}} | {:halt, {:error, new_error()}}
  defp accumulate_clean_string("", error, _values), do: {:halt, {:error, error}}
  defp accumulate_clean_string(value, _error, values), do: {:cont, {:ok, [value | values]}}

  @spec string_list_result({:ok, [String.t()]} | {:error, new_error()}, new_error(), atom()) ::
          {:ok, [String.t()]} | {:error, new_error()}
  defp string_list_result({:error, _reason} = error, _list_error, _emptiness), do: error
  defp string_list_result({:ok, []}, error, :non_empty), do: {:error, error}

  defp string_list_result({:ok, values}, _error, _emptiness) do
    values
    |> Enum.uniq()
    |> Enum.sort()
    |> then(&{:ok, &1})
  end

  @spec action_set(term(), new_error()) :: {:ok, MapSet.t(atom())} | {:error, new_error()}
  defp action_set(actions, error) when is_list(actions) do
    actions
    |> Enum.reduce_while({:ok, []}, &accumulate_action(error, &1, &2))
    |> action_set_result(error)
  end

  defp action_set(_actions, error), do: {:error, error}

  @spec required_action_set(term(), new_error()) ::
          {:ok, MapSet.t(atom())} | {:error, new_error()}
  defp required_action_set(actions, error) do
    actions
    |> action_set(error)
    |> required_action_set_result(error)
  end

  @spec required_action_set_result({:ok, MapSet.t(atom())} | {:error, new_error()}, new_error()) ::
          {:ok, MapSet.t(atom())} | {:error, new_error()}
  defp required_action_set_result({:ok, actions}, error) do
    actions
    |> MapSet.size()
    |> required_action_set_size_result(actions, error)
  end

  defp required_action_set_result({:error, _reason} = error, _field_error), do: error

  @spec required_action_set_size_result(non_neg_integer(), MapSet.t(atom()), new_error()) ::
          {:ok, MapSet.t(atom())} | {:error, new_error()}
  defp required_action_set_size_result(0, _actions, error), do: {:error, error}
  defp required_action_set_size_result(_size, actions, _error), do: {:ok, actions}

  @spec accumulate_action(new_error(), term(), {:ok, [atom()]}) ::
          {:cont, {:ok, [atom()]}} | {:halt, {:error, new_error()}}
  defp accumulate_action(error, action, {:ok, actions}) do
    case action_atom(action) do
      {:ok, atom} -> {:cont, {:ok, [atom | actions]}}
      :error -> {:halt, {:error, error}}
    end
  end

  @spec action_atom(term()) :: {:ok, atom()} | :error
  defp action_atom(action) when is_atom(action) and not is_nil(action), do: {:ok, action}

  defp action_atom(action) when is_binary(action) do
    action
    |> String.trim()
    |> action_atom_from_string()
  end

  defp action_atom(_action), do: :error

  @spec action_atom_from_string(String.t()) :: {:ok, atom()} | :error
  defp action_atom_from_string(""), do: :error
  defp action_atom_from_string(action), do: {:ok, String.to_atom(action)}

  @spec action_set_result({:ok, [atom()]} | {:error, new_error()}, new_error()) ::
          {:ok, MapSet.t(atom())} | {:error, new_error()}
  defp action_set_result({:ok, actions}, _error), do: {:ok, MapSet.new(actions)}
  defp action_set_result({:error, _reason}, error), do: {:error, error}

  @spec enum_atom(term(), [atom()], new_error()) :: {:ok, atom()} | {:error, new_error()}
  defp enum_atom(value, allowed, error) when is_atom(value) do
    allowed
    |> Enum.member?(value)
    |> enum_atom_result(value, error)
  end

  defp enum_atom(value, allowed, error) when is_binary(value) do
    value
    |> String.trim()
    |> String.downcase()
    |> enum_atom_from_string(allowed, error)
  end

  defp enum_atom(_value, _allowed, error), do: {:error, error}

  @spec enum_atom_from_string(String.t(), [atom()], new_error()) ::
          {:ok, atom()} | {:error, new_error()}
  defp enum_atom_from_string(value, allowed, error) do
    allowed
    |> Enum.find(&(Atom.to_string(&1) == value))
    |> enum_atom_lookup_result(error)
  end

  @spec enum_atom_result(boolean(), atom(), new_error()) :: {:ok, atom()} | {:error, new_error()}
  defp enum_atom_result(true, value, _error), do: {:ok, value}
  defp enum_atom_result(false, _value, error), do: {:error, error}

  @spec enum_atom_lookup_result(atom() | nil, new_error()) ::
          {:ok, atom()} | {:error, new_error()}
  defp enum_atom_lookup_result(nil, error), do: {:error, error}
  defp enum_atom_lookup_result(value, _error), do: {:ok, value}

  @spec positive_integer(map(), atom(), new_error()) ::
          {:ok, pos_integer()} | {:error, new_error()}
  defp positive_integer(attrs, field, error) do
    attrs
    |> value_at(field)
    |> positive_integer_result(error)
  end

  @spec commission_budget(map()) :: {:ok, pos_integer()} | {:error, :invalid_commission_budget}
  defp commission_budget(attrs) do
    attrs
    |> value_at(:commission_budget, value_at(attrs, :max_commissions))
    |> positive_integer_result(:invalid_commission_budget)
  end

  @spec positive_integer_result(term(), new_error()) ::
          {:ok, pos_integer()} | {:error, new_error()}
  defp positive_integer_result(value, _error) when is_integer(value) and value > 0,
    do: {:ok, value}

  defp positive_integer_result(_value, error), do: {:error, error}

  @spec non_negative_integer(map(), atom(), new_error()) ::
          {:ok, non_neg_integer()} | {:error, new_error()}
  defp non_negative_integer(attrs, field, error) do
    attrs
    |> value_at(field, 0)
    |> non_negative_integer_result(error)
  end

  @spec non_negative_integer_result(term(), new_error()) ::
          {:ok, non_neg_integer()} | {:error, new_error()}
  defp non_negative_integer_result(value, _error) when is_integer(value) and value >= 0,
    do: {:ok, value}

  defp non_negative_integer_result(_value, error), do: {:error, error}

  @spec optional_failure_strategy(map()) :: {:ok, atom() | nil} | {:error, :invalid_on_stale}
  defp optional_failure_strategy(attrs) do
    attrs
    |> value_at(:on_stale)
    |> optional_enum_atom(@failure_strategies, :invalid_on_stale)
  end

  @spec optional_enum_atom(term(), [atom()], new_error()) ::
          {:ok, atom() | nil} | {:error, new_error()}
  defp optional_enum_atom(nil, _allowed, _error), do: {:ok, nil}
  defp optional_enum_atom(value, allowed, error), do: enum_atom(value, allowed, error)

  @spec datetime(term(), new_error()) :: {:ok, DateTime.t()} | {:error, new_error()}
  defp datetime(%DateTime{} = datetime, _error), do: {:ok, datetime}

  defp datetime(value, error) when is_binary(value) do
    value
    |> DateTime.from_iso8601()
    |> datetime_result(error)
  end

  defp datetime(_value, error), do: {:error, error}

  @spec optional_datetime(term(), new_error()) ::
          {:ok, DateTime.t() | nil} | {:error, new_error()}
  defp optional_datetime(nil, _error), do: {:ok, nil}
  defp optional_datetime(value, error), do: datetime(value, error)

  @spec datetime_result({:ok, DateTime.t(), integer()} | {:error, term()}, new_error()) ::
          {:ok, DateTime.t()} | {:error, new_error()}
  defp datetime_result({:ok, datetime, _offset}, _error), do: {:ok, datetime}
  defp datetime_result({:error, _reason}, error), do: {:error, error}

  @spec required_gates(map()) :: {:ok, [atom()]} | {:error, :required_gate_missing}
  defp required_gates(attrs) do
    attrs
    |> value_at(:required_gates, [])
    |> gate_list()
    |> required_gate_result()
  end

  @spec gate_list(term()) :: [atom()]
  defp gate_list(values) when is_list(values) do
    values
    |> Enum.flat_map(&gate_atom/1)
    |> Enum.uniq()
    |> Enum.sort()
  end

  defp gate_list(_values), do: []

  @spec gate_atom(term()) :: [atom()]
  defp gate_atom(value) when is_atom(value), do: [value]

  defp gate_atom(value) when is_binary(value) do
    value
    |> String.trim()
    |> gate_atom_from_string()
  end

  defp gate_atom(_value), do: []

  @spec gate_atom_from_string(String.t()) :: [atom()]
  defp gate_atom_from_string(""), do: []
  defp gate_atom_from_string(value), do: [String.to_atom(value)]

  @spec required_gate_result([atom()]) :: {:ok, [atom()]} | {:error, :required_gate_missing}
  defp required_gate_result([]), do: {:ok, []}

  defp required_gate_result(gates) do
    @required_gates
    |> Enum.all?(&Enum.member?(gates, &1))
    |> required_gate_present_result(gates)
  end

  @spec required_gate_present_result(boolean(), [atom()]) ::
          {:ok, [atom()]} | {:error, :required_gate_missing}
  defp required_gate_present_result(true, gates), do: {:ok, gates}
  defp required_gate_present_result(false, _gates), do: {:error, :required_gate_missing}

  @spec active_state(t()) :: :ok | {:error, allow_error()}
  defp active_state(%__MODULE__{state: state}) when state in @active_states, do: :ok
  defp active_state(%__MODULE__{state: :revoked}), do: {:error, :autonomy_envelope_revoked}
  defp active_state(%__MODULE__{state: :expired}), do: {:error, :autonomy_envelope_expired}
  defp active_state(%__MODULE__{state: :exhausted}), do: {:error, :autonomy_envelope_exhausted}
  defp active_state(%__MODULE__{}), do: {:error, :autonomy_envelope_not_active}

  @spec not_expired(t(), DateTime.t()) :: :ok | {:error, :autonomy_envelope_expired}
  defp not_expired(%__MODULE__{valid_until: valid_until}, checked_at) do
    valid_until
    |> DateTime.compare(checked_at)
    |> not_expired_result()
  end

  @spec not_expired_result(:lt | :eq | :gt) :: :ok | {:error, :autonomy_envelope_expired}
  defp not_expired_result(:gt), do: :ok
  defp not_expired_result(_order), do: {:error, :autonomy_envelope_expired}

  @spec not_revoked(t(), DateTime.t()) :: :ok | {:error, :autonomy_envelope_revoked}
  defp not_revoked(%__MODULE__{revoked_at: nil}, _checked_at), do: :ok

  defp not_revoked(%__MODULE__{revoked_at: revoked_at}, checked_at) do
    revoked_at
    |> DateTime.compare(checked_at)
    |> not_revoked_result()
  end

  @spec not_revoked_result(:lt | :eq | :gt) :: :ok | {:error, :autonomy_envelope_revoked}
  defp not_revoked_result(:gt), do: :ok
  defp not_revoked_result(_order), do: {:error, :autonomy_envelope_revoked}

  @spec repo_allowed(t(), OpenSleigh.WorkCommission.t()) ::
          :ok | {:error, :repo_outside_autonomy_envelope}
  defp repo_allowed(%__MODULE__{allowed_repos: repos}, commission) do
    repos
    |> Enum.member?(commission.scope.repo_ref)
    |> repo_allowed_result()
  end

  @spec repo_allowed_result(boolean()) :: :ok | {:error, :repo_outside_autonomy_envelope}
  defp repo_allowed_result(true), do: :ok
  defp repo_allowed_result(false), do: {:error, :repo_outside_autonomy_envelope}

  @spec paths_allowed(t(), OpenSleigh.WorkCommission.t()) ::
          :ok | {:error, :path_outside_autonomy_envelope}
  defp paths_allowed(%__MODULE__{} = envelope, commission) do
    commission.scope.allowed_paths
    |> Enum.all?(&path_allowed?(envelope, &1))
    |> paths_allowed_result()
  end

  @spec path_allowed?(t(), String.t()) :: boolean()
  defp path_allowed?(%__MODULE__{} = envelope, path) do
    Enum.any?(envelope.allowed_paths, &path_matches?(&1, path)) and
      not Enum.any?(envelope.forbidden_paths, &path_matches?(&1, path))
  end

  @spec path_matches?(String.t(), String.t()) :: boolean()
  defp path_matches?("**/*", _path), do: true

  defp path_matches?(pattern, path) do
    pattern
    |> String.trim_trailing("/**")
    |> path_match_result(pattern, path)
  end

  @spec path_match_result(String.t(), String.t(), String.t()) :: boolean()
  defp path_match_result(prefix, pattern, path) when pattern != prefix do
    path == prefix or String.starts_with?(path, prefix <> "/")
  end

  defp path_match_result(_prefix, pattern, path), do: pattern == path

  @spec paths_allowed_result(boolean()) :: :ok | {:error, :path_outside_autonomy_envelope}
  defp paths_allowed_result(true), do: :ok
  defp paths_allowed_result(false), do: {:error, :path_outside_autonomy_envelope}

  @spec actions_allowed(t(), OpenSleigh.WorkCommission.t()) :: :ok | {:error, allow_error()}
  defp actions_allowed(%__MODULE__{} = envelope, commission) do
    commission.scope.allowed_actions
    |> Enum.reduce_while(:ok, &action_allowed(envelope, &1, &2))
  end

  @spec action_allowed(t(), atom(), :ok) ::
          {:cont, :ok} | {:halt, {:error, allow_error()}}
  defp action_allowed(envelope, action, :ok) do
    envelope
    |> action_allowed_result(action)
    |> action_allowed_reduction()
  end

  @spec action_allowed_result(t(), atom()) :: :ok | {:error, allow_error()}
  defp action_allowed_result(envelope, action) do
    one_way_forbidden = MapSet.member?(envelope.forbidden_one_way_door_actions, action)
    forbidden = MapSet.member?(envelope.forbidden_actions, action)
    allowed = MapSet.member?(envelope.allowed_actions, action)

    cond do
      one_way_forbidden -> {:error, :one_way_door_action_forbidden}
      forbidden -> {:error, :action_forbidden_by_autonomy_envelope}
      allowed -> :ok
      true -> {:error, :action_outside_autonomy_envelope}
    end
  end

  @spec action_allowed_reduction(:ok | {:error, allow_error()}) ::
          {:cont, :ok} | {:halt, {:error, allow_error()}}
  defp action_allowed_reduction(:ok), do: {:cont, :ok}
  defp action_allowed_reduction({:error, _reason} = error), do: {:halt, error}

  @spec modules_allowed(t(), OpenSleigh.WorkCommission.t()) ::
          :ok | {:error, :module_outside_autonomy_envelope}
  defp modules_allowed(%__MODULE__{allowed_modules: []}, _commission), do: :ok

  defp modules_allowed(%__MODULE__{} = envelope, commission) do
    commission.scope.allowed_modules
    |> Enum.all?(&Enum.member?(envelope.allowed_modules, &1))
    |> modules_allowed_result()
  end

  @spec modules_allowed_result(boolean()) :: :ok | {:error, :module_outside_autonomy_envelope}
  defp modules_allowed_result(true), do: :ok
  defp modules_allowed_result(false), do: {:error, :module_outside_autonomy_envelope}

  @spec budget_available(t()) :: :ok | {:error, allow_error()}
  defp budget_available(%__MODULE__{} = envelope) do
    cond do
      envelope.active_concurrency >= envelope.max_concurrency ->
        {:error, :autonomy_envelope_concurrency_exhausted}

      envelope.consumed_commissions >= envelope.commission_budget ->
        {:error, :autonomy_envelope_commission_budget_exhausted}

      true ->
        :ok
    end
  end

  @spec value_at(term(), atom()) :: term()
  defp value_at(value, key), do: value_at(value, key, nil)

  @spec value_at(term(), atom(), term()) :: term()
  defp value_at(%{} = map, key, fallback) do
    Map.get(map, key, Map.get(map, Atom.to_string(key), fallback))
  end

  defp value_at(_value, _key, fallback), do: fallback
end
