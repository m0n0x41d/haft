defmodule OpenSleigh.Agent.Adapter do
  @moduledoc """
  Behaviour contract for agent-adapter implementations.

  Per `specs/target-system/AGENT_PROTOCOL.md` + v0.6.1 L4/L5 ownership
  seam:

  * L4 `Agent.Adapter` modules are **stateless** typed APIs.
  * Any `Port` / GenServer that owns the subprocess lives at L5
    (the L5 `AgentWorker` spawns + owns the `Port`, and passes a
    `handle` to L4 functions).
  * Callbacks take the `AdapterSession` context + handle; they
    return `{:ok, _} | {:error, EffectError.t()}`. No raises for
    expected failures.

  Conformance per AGENT_PROTOCOL.md §10:

  * JSON-RPC handshake in order: `initialize → initialized →
    thread/start → turn/start`.
  * Support continuation turns on the same thread (MVP-1 Execute).
  * Emit all required event categories.
  * Map every failure to `EffectError.t()`.
  * Enforce phase-scoped tool dispatch (compile-time adapter tool
    registry + runtime per-phase `MapSet`).
  * Enforce stall detection.
  * Respect `PathGuard` for filesystem-touching tools.
  * Expose NO direct tracker-mutation tool.

  The parity plan (`specs/target-system/ADAPTER_PARITY.md`) ensures
  Codex and Claude impls satisfy this behaviour identically before
  MVP-1.5 ships.
  """

  alias OpenSleigh.{AdapterSession, EffectError, Scope, WorkCommission}

  @typedoc "Per-impl handle (e.g. a Port, a server pid, a mock state id)."
  @type handle :: term()

  @typedoc "Normalised agent event streamed to the orchestrator."
  @type event :: %{
          required(:event) => atom(),
          required(:timestamp) => DateTime.t(),
          optional(any()) => any()
        }

  @typedoc "Agent reply to a turn request."
  @type agent_reply :: %{
          required(:turn_id) => String.t(),
          required(:status) => :completed | :failed | :cancelled | :timeout,
          optional(:events) => [event()],
          optional(:usage) => map(),
          optional(:text) => String.t()
        }

  @typedoc "Tool-call result returned by `dispatch_tool/4`."
  @type tool_result :: %{
          required(:call_id) => String.t(),
          required(:result) => term()
        }

  @doc """
  Start a session — perform the JSON-RPC handshake and open a thread.
  Returns a handle that identifies the live thread for subsequent
  `send_turn/3` calls.
  """
  @callback start_session(AdapterSession.t()) ::
              {:ok, handle()} | {:error, EffectError.t()}

  @doc """
  Send a turn request to the live thread. Used for both the first
  turn (full prompt) and continuation turns (guidance text).
  """
  @callback send_turn(handle(), prompt :: String.t(), AdapterSession.t()) ::
              {:ok, agent_reply()} | {:error, EffectError.t()}

  @doc """
  Dispatch a tool call the agent requested. Enforces phase-scope
  via `AdapterSession.scoped_tools`.
  """
  @callback dispatch_tool(handle(), tool :: atom(), args :: map(), AdapterSession.t()) ::
              {:ok, tool_result()} | {:error, EffectError.t()}

  @doc "Close the session — clean up the adapter's handle."
  @callback close_session(handle()) :: :ok

  @doc "Name of this adapter (used by parity-plan + telemetry)."
  @callback adapter_kind() :: atom()

  @doc """
  Compile-time closed atom set of tools this adapter supports.
  Unknown atoms fail at function-clause match per CL1.
  """
  @callback tool_registry() :: [atom()]

  # ——— helpers (available via `use Agent.Adapter`) ———

  @commission_scope_error :mutation_outside_commission_scope

  @type commission_scope_error :: :mutation_outside_commission_scope
  @type scope_check_error :: EffectError.t() | commission_scope_error()

  @doc """
  Runtime per-phase-scope check. Called by `dispatch_tool/4` impls
  before dispatching to the underlying transport. Catches CL2 /
  CL3 — phase-scope violations. When L5 has attached commission
  context, this also checks broad WorkCommission action authority.

  Mutating path tools (`:write`, `:edit`) need `ensure_in_scope/3`.
  Calling this arity for those tools fails closed under a commission
  because no target path is available to authorize.
  """
  @spec ensure_in_scope(AdapterSession.t(), atom()) ::
          :ok | {:error, scope_check_error()}
  def ensure_in_scope(%AdapterSession{} = session, tool)
      when is_atom(tool) do
    session.scoped_tools
    |> phase_tool_result(tool)
    |> ensure_commission_action_scope(session, tool)
    |> require_mutation_target_args(session, tool)
  end

  @doc """
  Runtime per-phase and per-commission check for tool calls that expose
  arguments. Adapter implementations can call this arity before
  executing mutating tools so path arguments are checked against the
  commission Scope as well as the phase tool set.
  """
  @spec ensure_in_scope(AdapterSession.t(), atom(), map()) ::
          :ok | {:error, scope_check_error()}
  def ensure_in_scope(%AdapterSession{} = session, tool, args)
      when is_atom(tool) and is_map(args) do
    session.scoped_tools
    |> phase_tool_result(tool)
    |> ensure_commission_action_scope(session, tool)
    |> ensure_commission_path_scope(session, tool, args)
  end

  @doc """
  Validate the terminal diff for a completed run.

  Every changed path must remain inside the WorkCommission Scope. This
  check is independent from `PathGuard`: a path can be safely inside
  `workspace_path` and still be outside the commission authority.
  """
  @spec validate_terminal_diff(AdapterSession.t(), [Path.t()]) ::
          :ok | {:error, commission_scope_error()}
  def validate_terminal_diff(%AdapterSession{} = session, changed_paths)
      when is_list(changed_paths) do
    session
    |> scope()
    |> terminal_diff_result(session, changed_paths)
  end

  @doc """
  Return changed paths that would violate the terminal WorkCommission Scope.

  Used by operator-facing observability after `validate_terminal_diff/2`
  blocks a run. The authority decision stays boolean; this helper only
  explains which paths caused the block.
  """
  @spec terminal_diff_out_of_scope_paths(AdapterSession.t(), [Path.t()]) :: [Path.t()]
  def terminal_diff_out_of_scope_paths(%AdapterSession{} = session, changed_paths)
      when is_list(changed_paths) do
    session
    |> scope()
    |> terminal_diff_out_of_scope_paths_result(session, changed_paths)
  end

  @doc "Attach WorkCommission context to the AdapterSession carrier."
  @spec attach_commission_context(AdapterSession.t(), WorkCommission.t()) :: AdapterSession.t()
  def attach_commission_context(
        %AdapterSession{} = session,
        %WorkCommission{} = commission
      ) do
    session
    |> Map.put(:commission, commission)
    |> Map.put(:commission_id, commission.id)
    |> Map.put(:scope, commission.scope)
  end

  @doc "Return the WorkCommission snapshot carried by an adapter session, when present."
  @spec commission(AdapterSession.t()) :: WorkCommission.t() | nil
  def commission(%AdapterSession{} = session) do
    Map.get(session, :commission)
  end

  @doc "Return the WorkCommission id carried by an adapter session, when present."
  @spec commission_id(AdapterSession.t()) :: String.t() | nil
  def commission_id(%AdapterSession{} = session) do
    Map.get(session, :commission_id)
  end

  @doc "Return the Scope carried by an adapter session, when present."
  @spec scope(AdapterSession.t()) :: Scope.t() | nil
  def scope(%AdapterSession{} = session) do
    Map.get(session, :scope)
  end

  @spec phase_tool_result(MapSet.t(atom()), atom()) ::
          :ok | {:error, :tool_forbidden_by_phase_scope}
  defp phase_tool_result(scoped, tool) do
    if MapSet.member?(scoped, tool),
      do: :ok,
      else: {:error, :tool_forbidden_by_phase_scope}
  end

  @spec ensure_commission_action_scope(
          :ok | {:error, scope_check_error()},
          AdapterSession.t(),
          atom()
        ) ::
          :ok | {:error, scope_check_error()}
  defp ensure_commission_action_scope({:error, _reason} = error, _session, _tool), do: error

  defp ensure_commission_action_scope(:ok, %AdapterSession{} = session, tool) do
    session
    |> scope()
    |> commission_action_result(tool)
  end

  @spec ensure_commission_path_scope(
          :ok | {:error, scope_check_error()},
          AdapterSession.t(),
          atom(),
          map()
        ) :: :ok | {:error, scope_check_error()}
  defp ensure_commission_path_scope({:error, _reason} = error, _session, _tool, _args), do: error

  defp ensure_commission_path_scope(:ok, %AdapterSession{} = session, tool, args) do
    session
    |> scope()
    |> commission_path_result(session, tool, args)
  end

  @spec require_mutation_target_args(
          :ok | {:error, scope_check_error()},
          AdapterSession.t(),
          atom()
        ) :: :ok | {:error, scope_check_error()}
  defp require_mutation_target_args({:error, _reason} = error, _session, _tool), do: error

  defp require_mutation_target_args(:ok, %AdapterSession{} = session, tool) do
    session
    |> scope()
    |> mutation_target_args_result(tool)
  end

  @spec mutation_target_args_result(Scope.t() | nil, atom()) ::
          :ok | {:error, commission_scope_error()}
  defp mutation_target_args_result(nil, _tool), do: :ok

  defp mutation_target_args_result(%Scope{}, tool) do
    tool
    |> tool_action()
    |> mutation_target_args_result()
  end

  @spec mutation_target_args_result(atom() | nil) ::
          :ok | {:error, commission_scope_error()}
  defp mutation_target_args_result(:edit_files), do: {:error, @commission_scope_error}
  defp mutation_target_args_result(_action), do: :ok

  @spec commission_action_result(Scope.t() | nil, atom()) ::
          :ok | {:error, commission_scope_error()}
  defp commission_action_result(nil, _tool), do: :ok

  defp commission_action_result(%Scope{} = scope, tool) do
    tool
    |> tool_action()
    |> action_allowed_result(scope)
  end

  @spec action_allowed_result(atom() | nil, Scope.t()) ::
          :ok | {:error, commission_scope_error()}
  defp action_allowed_result(nil, %Scope{}), do: :ok

  defp action_allowed_result(action, %Scope{allowed_actions: actions}) do
    if MapSet.member?(actions, action),
      do: :ok,
      else: {:error, @commission_scope_error}
  end

  @spec commission_path_result(Scope.t() | nil, AdapterSession.t(), atom(), map()) ::
          :ok | {:error, commission_scope_error()}
  defp commission_path_result(nil, %AdapterSession{}, _tool, _args), do: :ok

  defp commission_path_result(%Scope{} = scope, %AdapterSession{} = session, tool, args) do
    tool
    |> tool_action()
    |> path_scope_result(scope, session, args)
  end

  @spec path_scope_result(atom() | nil, Scope.t(), AdapterSession.t(), map()) ::
          :ok | {:error, commission_scope_error()}
  defp path_scope_result(:edit_files, %Scope{} = scope, %AdapterSession{} = session, args) do
    args
    |> target_paths()
    |> paths_allowed_result(scope, session)
  end

  defp path_scope_result(_action, %Scope{}, %AdapterSession{}, _args), do: :ok

  @spec paths_allowed_result([String.t()], Scope.t(), AdapterSession.t()) ::
          :ok | {:error, commission_scope_error()}
  defp paths_allowed_result([], %Scope{}, %AdapterSession{}),
    do: {:error, @commission_scope_error}

  defp paths_allowed_result(paths, %Scope{} = scope, %AdapterSession{} = session) do
    paths
    |> Enum.all?(&path_allowed?(&1, scope, session))
    |> path_allowed_result()
  end

  @spec path_allowed_result(boolean()) :: :ok | {:error, commission_scope_error()}
  defp path_allowed_result(true), do: :ok
  defp path_allowed_result(false), do: {:error, @commission_scope_error}

  @spec terminal_diff_result(Scope.t() | nil, AdapterSession.t(), [Path.t()]) ::
          :ok | {:error, commission_scope_error()}
  defp terminal_diff_result(nil, %AdapterSession{}, _changed_paths), do: :ok

  defp terminal_diff_result(%Scope{} = scope, %AdapterSession{} = session, changed_paths) do
    changed_paths
    |> Enum.all?(&terminal_path_allowed?(&1, scope, session))
    |> path_allowed_result()
  end

  @spec terminal_diff_out_of_scope_paths_result(Scope.t() | nil, AdapterSession.t(), [Path.t()]) ::
          [Path.t()]
  defp terminal_diff_out_of_scope_paths_result(nil, %AdapterSession{}, _changed_paths), do: []

  defp terminal_diff_out_of_scope_paths_result(
         %Scope{} = scope,
         %AdapterSession{} = session,
         changed_paths
       ) do
    changed_paths
    |> Enum.reject(&terminal_path_allowed?(&1, scope, session))
    |> Enum.uniq()
  end

  @spec terminal_path_allowed?(term(), Scope.t(), AdapterSession.t()) :: boolean()
  defp terminal_path_allowed?(path, %Scope{} = scope, %AdapterSession{} = session)
       when is_binary(path) do
    normalized = normalize_scope_path(path, session)
    runtime_owned_terminal_path?(normalized) or scoped_relative_path_allowed?(normalized, scope)
  end

  defp terminal_path_allowed?(_path, %Scope{}, %AdapterSession{}), do: false

  @spec runtime_owned_terminal_path?(String.t()) :: boolean()
  defp runtime_owned_terminal_path?(".tmp"), do: true

  defp runtime_owned_terminal_path?(path) when is_binary(path),
    do: String.starts_with?(path, ".tmp/")

  @spec path_allowed?(String.t(), Scope.t(), AdapterSession.t()) :: boolean()
  defp path_allowed?(path, %Scope{} = scope, %AdapterSession{} = session) do
    path
    |> normalize_scope_path(session)
    |> scoped_relative_path_allowed?(scope)
  end

  @spec scoped_relative_path_allowed?(String.t(), Scope.t()) :: boolean()
  defp scoped_relative_path_allowed?(path, %Scope{} = scope) do
    path
    |> inside_workspace_relative_path?()
    |> relative_path_scope_result(path, scope)
  end

  @spec inside_workspace_relative_path?(String.t()) :: boolean()
  defp inside_workspace_relative_path?(path) do
    Path.type(path) == :relative and not String.starts_with?(path, "..")
  end

  @spec relative_path_scope_result(boolean(), String.t(), Scope.t()) :: boolean()
  defp relative_path_scope_result(true, path, %Scope{} = scope) do
    scope_path_allowed?(path, scope)
  end

  defp relative_path_scope_result(false, _path, %Scope{}), do: false

  @spec scope_path_allowed?(String.t(), Scope.t()) :: boolean()
  defp scope_path_allowed?(path, %Scope{} = scope) do
    allowed =
      scope.allowed_paths
      |> Enum.any?(&path_matches?(&1, path))

    forbidden =
      scope.forbidden_paths
      |> Enum.any?(&path_matches?(&1, path))

    allowed and not forbidden
  end

  @spec normalize_scope_path(String.t(), AdapterSession.t()) :: String.t()
  defp normalize_scope_path(path, %AdapterSession{workspace_path: workspace}) do
    path
    |> Path.expand(workspace)
    |> Path.relative_to(Path.expand(workspace))
  end

  @spec path_matches?(String.t(), String.t()) :: boolean()
  defp path_matches?("**/*", path) when is_binary(path), do: not String.starts_with?(path, "..")

  defp path_matches?(pattern, path) when is_binary(pattern) and is_binary(path) do
    cond do
      String.ends_with?(pattern, "/**") ->
        prefix = String.trim_trailing(pattern, "/**")
        path == prefix or String.starts_with?(path, prefix <> "/")

      String.contains?(pattern, "*") ->
        pattern
        |> glob_regex()
        |> Regex.match?(path)

      true ->
        pattern == path
    end
  end

  @spec glob_regex(String.t()) :: Regex.t()
  defp glob_regex(pattern) do
    pattern
    |> Regex.escape()
    |> String.replace("\\*\\*", ".*")
    |> String.replace("\\*", "[^/]*")
    |> then(&Regex.compile!("^" <> &1 <> "$"))
  end

  @spec target_paths(map()) :: [String.t()]
  defp target_paths(args) do
    [
      :path,
      "path",
      :paths,
      "paths",
      :file,
      "file",
      :files,
      "files",
      :filepath,
      "filepath",
      :target,
      "target",
      :target_path,
      "target_path",
      :filename,
      "filename"
    ]
    |> Enum.flat_map(&target_path_values(args, &1))
    |> Enum.filter(&is_binary/1)
  end

  @spec target_path_values(map(), atom() | String.t()) :: [term()]
  defp target_path_values(args, key) do
    args
    |> Map.get(key)
    |> target_path_values()
  end

  @spec target_path_values(term()) :: [term()]
  defp target_path_values(paths) when is_list(paths), do: paths
  defp target_path_values(path), do: [path]

  @spec tool_action(atom()) :: atom() | nil
  defp tool_action(tool) when tool in [:write, :edit], do: :edit_files
  defp tool_action(:bash), do: :run_tests
  defp tool_action(_tool), do: nil
end
