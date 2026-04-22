defmodule OpenSleigh.Agent.Codex do
  @moduledoc """
  `OpenSleigh.Agent.Adapter` implementation for Codex app-server.

  This module is the L4 stateless adapter surface. The subprocess
  Port is owned by the L5 `OpenSleigh.Agent.Codex.Server`; this
  module starts a per-session supervisor and delegates all live
  protocol operations through `GenServer.call/3`.
  """

  @behaviour OpenSleigh.Agent.Adapter

  alias OpenSleigh.{AdapterSession, EffectError}
  alias OpenSleigh.Agent.Adapter, as: AgentAdapter
  alias OpenSleigh.Agent.Codex.{Server, Supervisor}

  @tool_registry [
    :read,
    :write,
    :edit,
    :bash,
    :grep,
    :haft_query,
    :haft_note,
    :haft_problem,
    :haft_decision,
    :haft_refresh,
    :haft_solution
  ]

  @impl true
  @spec adapter_kind() :: atom()
  def adapter_kind, do: :codex

  @impl true
  @spec tool_registry() :: [atom()]
  def tool_registry, do: @tool_registry

  @impl true
  @spec start_session(AdapterSession.t()) :: {:ok, map()} | {:error, EffectError.t()}
  def start_session(%AdapterSession{} = session) do
    session
    |> start_supervised_server()
    |> complete_start_session(session)
  end

  @impl true
  @spec send_turn(map(), String.t(), AdapterSession.t()) ::
          {:ok, map()} | {:error, EffectError.t()}
  def send_turn(%{server: server}, prompt, %AdapterSession{} = session)
      when is_pid(server) and is_binary(prompt) do
    GenServer.call(server, {:send_turn, prompt, session}, :infinity)
  end

  @impl true
  @spec dispatch_tool(map(), atom(), map(), AdapterSession.t()) ::
          {:ok, map()} | {:error, EffectError.t()}
  def dispatch_tool(_handle, tool, _args, %AdapterSession{} = session)
      when tool in @tool_registry do
    session
    |> AgentAdapter.ensure_in_scope(tool)
    |> dispatch_scoped_tool(tool)
  end

  def dispatch_tool(_handle, _tool, _args, %AdapterSession{}),
    do: {:error, :tool_unknown_to_adapter}

  @impl true
  @spec close_session(map()) :: :ok
  def close_session(%{server: server, supervisor: supervisor})
      when is_pid(server) and is_pid(supervisor) do
    Supervisor.stop_session(%{server: server, supervisor: supervisor})
  end

  @spec start_supervised_server(AdapterSession.t()) ::
          {:ok, map()} | {:error, EffectError.t()}
  defp start_supervised_server(%AdapterSession{} = _session) do
    Supervisor.start_session(server_opts())
  end

  @spec complete_start_session({:ok, map()} | {:error, EffectError.t()}, AdapterSession.t()) ::
          {:ok, map()} | {:error, EffectError.t()}
  defp complete_start_session({:ok, handle}, session) do
    case Server.start_session(handle.server, session) do
      {:ok, metadata} ->
        {:ok, Map.merge(handle, metadata)}

      {:error, reason} ->
        :ok = Supervisor.stop_session(handle)
        {:error, reason}
    end
  end

  defp complete_start_session({:error, _reason} = error, _session), do: error

  @spec dispatch_scoped_tool(:ok | {:error, EffectError.t()}, atom()) ::
          {:ok, map()} | {:error, EffectError.t()}
  defp dispatch_scoped_tool(:ok, _tool), do: {:error, :tool_execution_failed}
  defp dispatch_scoped_tool({:error, _reason} = error, _tool), do: error

  @spec server_opts() :: keyword()
  defp server_opts do
    Application.get_env(:open_sleigh, __MODULE__, [])
  end
end
