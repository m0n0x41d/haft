defmodule OpenSleigh.Agent.Claude do
  @moduledoc """
  Claude Code adapter skeleton.

  This module intentionally implements the same `Agent.Adapter` boundary as
  the incumbent adapter without starting a live provider process yet. It lets
  config compilation and parity tests cover the adapter surface while the
  live rollout remains blocked until the Codex canary evidence exists.
  """

  @behaviour OpenSleigh.Agent.Adapter

  alias OpenSleigh.{AdapterSession, EffectError}
  alias OpenSleigh.Agent.Adapter, as: AgentAdapter

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
  def adapter_kind, do: :claude

  @impl true
  @spec tool_registry() :: [atom()]
  def tool_registry, do: @tool_registry

  @impl true
  @spec start_session(AdapterSession.t()) :: {:ok, map()} | {:error, EffectError.t()}
  def start_session(%AdapterSession{}), do: {:error, :agent_command_not_found}

  @impl true
  @spec send_turn(map(), String.t(), AdapterSession.t()) ::
          {:ok, map()} | {:error, EffectError.t()}
  def send_turn(_handle, _prompt, %AdapterSession{}), do: {:error, :agent_command_not_found}

  @impl true
  @spec dispatch_tool(map(), atom(), map(), AdapterSession.t()) ::
          {:ok, map()} | {:error, EffectError.t()}
  def dispatch_tool(_handle, tool, _args, %AdapterSession{} = session)
      when tool in @tool_registry do
    session
    |> AgentAdapter.ensure_in_scope(tool)
    |> dispatch_scoped_tool()
  end

  def dispatch_tool(_handle, _tool, _args, %AdapterSession{}),
    do: {:error, :tool_unknown_to_adapter}

  @impl true
  @spec close_session(map()) :: :ok
  def close_session(_handle), do: :ok

  @spec dispatch_scoped_tool(:ok | {:error, EffectError.t()}) ::
          {:ok, map()} | {:error, EffectError.t()}
  defp dispatch_scoped_tool(:ok), do: {:error, :tool_execution_failed}
  defp dispatch_scoped_tool({:error, _reason} = error), do: error
end
