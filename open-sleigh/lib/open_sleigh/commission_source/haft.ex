defmodule OpenSleigh.CommissionSource.Haft do
  @moduledoc """
  Haft-backed `CommissionSource` adapter.

  This is the MVP-1R intake boundary: Open-Sleigh asks Haft for runnable
  WorkCommissions and atomically claims one for deterministic Preflight.
  It does not create, approve, refresh, complete, or otherwise author
  commissions.
  """

  @behaviour OpenSleigh.CommissionSource

  alias OpenSleigh.{
    AdapterSession,
    CommissionSource,
    ConfigHash,
    Haft.Client,
    Scope,
    SessionId,
    WorkCommission
  }

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

  @enforce_keys [:invoke_fun, :session, :selector, :runner_id]
  defstruct [:invoke_fun, :session, :selector, :runner_id, :plan_ref, :queue]

  @type t :: %__MODULE__{
          invoke_fun: Client.invoke_fun(),
          session: AdapterSession.t(),
          selector: String.t(),
          runner_id: String.t(),
          plan_ref: String.t() | nil,
          queue: String.t() | nil
        }

  @type config :: map()
  @type new_error :: :commission_source_invalid | AdapterSession.new_error()

  @doc "Build a Haft commission-source handle from `sleigh.md` config and a Haft invoker."
  @spec new(config(), Client.invoke_fun()) :: {:ok, t()} | {:error, new_error()}
  def new(config, invoke_fun) when is_map(config) and is_function(invoke_fun, 1) do
    source = commission_source_config(config)

    with {:ok, session} <- adapter_session(config) do
      {:ok,
       %__MODULE__{
         invoke_fun: invoke_fun,
         session: session,
         selector: config_string(value_at(source, :selector, "runnable")),
         runner_id: config_string(value_at(source, :runner_id, default_runner_id())),
         plan_ref: optional_string(value_at(source, :plan_ref)),
         queue: optional_string(value_at(source, :queue))
       }}
    end
  end

  def new(_config, _invoke_fun), do: {:error, :commission_source_invalid}

  @impl true
  @spec adapter_kind() :: :haft
  def adapter_kind, do: :haft

  @impl true
  @spec list_runnable(t()) ::
          {:ok, [WorkCommission.t()]} | {:error, CommissionSource.source_error()}
  def list_runnable(%__MODULE__{} = handle) do
    handle
    |> call_commission(:list_runnable, request_params(handle))
    |> decode_result()
    |> extract_commission_entries()
    |> parse_commissions()
    |> filter_runnable_commissions()
  end

  @impl true
  @spec claim_for_preflight(t()) ::
          {:ok, WorkCommission.t()} | {:error, CommissionSource.source_error()}
  def claim_for_preflight(%__MODULE__{} = handle) do
    handle
    |> call_commission(:claim_for_preflight, request_params(handle))
    |> decode_result()
    |> extract_claimed_commission()
    |> parse_commission()
  end

  @impl true
  @spec claim_for_preflight(t(), String.t()) ::
          {:ok, WorkCommission.t()} | {:error, CommissionSource.source_error()}
  def claim_for_preflight(%__MODULE__{} = handle, commission_id)
      when is_binary(commission_id) do
    params =
      handle
      |> request_params()
      |> Map.put("commission_id", commission_id)

    handle
    |> call_commission(:claim_for_preflight, params)
    |> decode_result()
    |> extract_claimed_commission()
    |> parse_commission()
  end

  @spec call_commission(t(), atom(), map()) ::
          {:ok, binary()} | {:error, CommissionSource.source_error()}
  defp call_commission(%__MODULE__{} = handle, action, params) do
    Client.call_tool(handle.session, :haft_commission, action, params, handle.invoke_fun)
  end

  @spec request_params(t()) :: map()
  defp request_params(%__MODULE__{} = handle) do
    %{
      "selector" => handle.selector,
      "runner_id" => handle.runner_id
    }
    |> maybe_put("plan_ref", handle.plan_ref)
    |> maybe_put("queue", handle.queue)
  end

  @spec decode_result({:ok, binary()} | {:error, CommissionSource.source_error()}) ::
          {:ok, map()} | {:error, CommissionSource.source_error()}
  defp decode_result({:ok, encoded}) do
    case Jason.decode(encoded) do
      {:ok, decoded} when is_map(decoded) -> {:ok, decoded}
      _other -> {:error, :response_parse_error}
    end
  end

  defp decode_result({:error, _reason} = error), do: error

  @spec extract_commission_entries({:ok, map()} | {:error, CommissionSource.source_error()}) ::
          {:ok, [map()]} | {:error, CommissionSource.source_error()}
  defp extract_commission_entries({:ok, %{"commissions" => entries}}) when is_list(entries),
    do: {:ok, entries}

  defp extract_commission_entries({:ok, %{"work_commissions" => entries}}) when is_list(entries),
    do: {:ok, entries}

  defp extract_commission_entries({:ok, %{"content" => content}}) when is_list(content) do
    content
    |> decode_content_payload()
    |> extract_commission_entries()
  end

  defp extract_commission_entries({:ok, _result}), do: {:error, :commission_response_malformed}
  defp extract_commission_entries({:error, _reason} = error), do: error

  @spec extract_claimed_commission({:ok, map()} | {:error, CommissionSource.source_error()}) ::
          {:ok, map()} | {:error, CommissionSource.source_error()}
  defp extract_claimed_commission({:ok, %{"commission" => commission}})
       when is_map(commission),
       do: {:ok, commission}

  defp extract_claimed_commission({:ok, %{"work_commission" => commission}})
       when is_map(commission),
       do: {:ok, commission}

  defp extract_claimed_commission({:ok, %{"content" => content}}) when is_list(content) do
    content
    |> decode_content_payload()
    |> extract_claimed_commission()
  end

  defp extract_claimed_commission({:ok, _result}), do: {:error, :commission_response_malformed}
  defp extract_claimed_commission({:error, _reason} = error), do: error

  @spec decode_content_payload([map()]) :: {:ok, map()} | {:error, :commission_response_malformed}
  defp decode_content_payload(content) do
    content
    |> Enum.find_value(&decode_content_item/1)
    |> content_payload_result()
  end

  @spec decode_content_item(map() | term()) :: map() | nil
  defp decode_content_item(%{"text" => text}) when is_binary(text) do
    case Jason.decode(text) do
      {:ok, decoded} when is_map(decoded) -> decoded
      _other -> nil
    end
  end

  defp decode_content_item(_content), do: nil

  @spec content_payload_result(map() | nil) ::
          {:ok, map()} | {:error, :commission_response_malformed}
  defp content_payload_result(%{} = payload), do: {:ok, payload}
  defp content_payload_result(nil), do: {:error, :commission_response_malformed}

  @spec parse_commissions({:ok, [map()]} | {:error, CommissionSource.source_error()}) ::
          {:ok, [WorkCommission.t()]} | {:error, CommissionSource.source_error()}
  defp parse_commissions({:ok, entries}) do
    entries
    |> Enum.reduce_while({:ok, []}, &accumulate_commission/2)
    |> parsed_commissions()
  end

  defp parse_commissions({:error, _reason} = error), do: error

  @spec parse_commission({:ok, map()} | {:error, CommissionSource.source_error()}) ::
          {:ok, WorkCommission.t()} | {:error, CommissionSource.source_error()}
  defp parse_commission({:ok, entry}), do: commission_from_entry(entry)
  defp parse_commission({:error, _reason} = error), do: error

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

  @spec filter_runnable_commissions(
          {:ok, [WorkCommission.t()]}
          | {:error, CommissionSource.source_error()}
        ) ::
          {:ok, [WorkCommission.t()]} | {:error, CommissionSource.source_error()}
  defp filter_runnable_commissions({:ok, commissions}) do
    commissions
    |> Enum.filter(&runnable?/1)
    |> then(&{:ok, &1})
  end

  defp filter_runnable_commissions({:error, _reason} = error), do: error

  @spec commission_from_entry(map()) ::
          {:ok, WorkCommission.t()} | {:error, CommissionSource.source_error()}
  defp commission_from_entry(%{} = entry) do
    with {:ok, scope} <- scope_from_entry(value_at(entry, :scope)),
         {:ok, attrs} <- commission_attrs(entry, scope) do
      WorkCommission.new(attrs)
    end
  end

  defp commission_from_entry(_entry), do: {:error, :commission_response_malformed}

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
         autonomy_envelope_ref: value_at(entry, :autonomy_envelope_ref),
         autonomy_envelope_revision: value_at(entry, :autonomy_envelope_revision),
         state: state,
         valid_until: valid_until,
         fetched_at: fetched_at
       }}
    end
  end

  @spec adapter_session(config()) ::
          {:ok, AdapterSession.t()} | {:error, AdapterSession.new_error()}
  defp adapter_session(config) do
    AdapterSession.new(%{
      session_id: SessionId.generate(),
      config_hash: ConfigHash.from_iodata(:erlang.term_to_binary(config)),
      scoped_tools: MapSet.new([:haft_commission]),
      workspace_path: File.cwd!(),
      adapter_kind: :haft_commission_source,
      adapter_version: "mvp1r",
      max_turns: 1,
      max_tokens_per_turn: 1,
      wall_clock_timeout_s: 30
    })
  end

  @spec commission_source_config(config()) :: map()
  defp commission_source_config(config) do
    case value_at(config, :commission_source) do
      %{} = source -> source
      _other -> config
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

  @spec maybe_put(map(), String.t(), String.t() | nil) :: map()
  defp maybe_put(map, _key, nil), do: map
  defp maybe_put(map, key, value), do: Map.put(map, key, value)

  @spec optional_string(term()) :: String.t() | nil
  defp optional_string(nil), do: nil
  defp optional_string(value) when is_binary(value), do: blank_to_nil(String.trim(value))
  defp optional_string(value), do: value |> config_string() |> optional_string()

  @spec blank_to_nil(String.t()) :: String.t() | nil
  defp blank_to_nil(""), do: nil
  defp blank_to_nil(value), do: value

  @spec config_string(term()) :: String.t()
  defp config_string(value) when is_binary(value), do: value
  defp config_string(value) when is_atom(value), do: Atom.to_string(value)
  defp config_string(value), do: to_string(value)

  @spec default_runner_id() :: String.t()
  defp default_runner_id, do: "open-sleigh:" <> Atom.to_string(Node.self())

  @spec value_at(term(), atom()) :: term()
  defp value_at(value, key), do: value_at(value, key, nil)

  @spec value_at(term(), atom(), term()) :: term()
  defp value_at(%{} = map, key, fallback) do
    Map.get(map, Atom.to_string(key), Map.get(map, key, fallback))
  end

  defp value_at(_value, _key, fallback), do: fallback
end
