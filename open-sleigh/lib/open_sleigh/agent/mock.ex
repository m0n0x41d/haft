defmodule OpenSleigh.Agent.Mock do
  @moduledoc """
  In-memory `Agent.Adapter` implementation for tests.

  Does not spawn a real subprocess. `start_session/1` returns a
  pseudo-handle; `send_turn/3` returns a canned `agent_reply`;
  `dispatch_tool/4` enforces phase-scope via
  `OpenSleigh.Agent.Adapter.ensure_in_scope/2` then returns a canned
  result. Unknown tools (not in `@tool_registry`) fail at function-
  clause — demonstrating CL1 compile-time enforcement at adapter
  scope.

  Used by L2 / L3 / L5 tests that need an `Agent.Adapter` impl but
  don't want to spawn `codex app-server`.
  """

  @behaviour OpenSleigh.Agent.Adapter

  alias OpenSleigh.{AdapterSession, EffectError}
  alias OpenSleigh.Agent.Adapter, as: AgentAdapter

  # Per the MVP-1 scoped toolsets (SPEC §8 sleigh.md example), the
  # union of tools an MVP-1 agent can invoke across all phases.
  # Unknown atoms fail `dispatch_tool/4` via function-clause match.
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

  @turn_count_key {__MODULE__, :turn_count}
  @turn_prompts_key {__MODULE__, :turn_prompts}
  @turn_scopes_key {__MODULE__, :turn_scopes}
  @turn_replies_key {__MODULE__, :turn_replies}
  @after_turn_key {__MODULE__, :after_turn}
  @start_count_key {__MODULE__, :start_count}

  @doc "Reset the process-local mock script and call history."
  @spec reset!() :: :ok
  def reset! do
    [
      @turn_count_key,
      @turn_prompts_key,
      @turn_scopes_key,
      @turn_replies_key,
      @after_turn_key,
      @start_count_key
    ]
    |> Enum.each(&Process.delete/1)

    :ok
  end

  @doc "Configure per-turn reply overrides for the current process."
  @spec put_turn_replies([map()]) :: :ok
  def put_turn_replies(replies) when is_list(replies) do
    Process.put(@turn_replies_key, replies)
    :ok
  end

  @doc "Configure a callback invoked after each mock turn."
  @spec put_after_turn((pos_integer(), map() -> term())) :: :ok
  def put_after_turn(fun) when is_function(fun, 2) do
    Process.put(@after_turn_key, fun)
    :ok
  end

  @doc "Return prompts sent to this process-local mock, oldest-first."
  @spec turn_prompts() :: [String.t()]
  def turn_prompts do
    @turn_prompts_key
    |> Process.get([])
    |> Enum.reverse()
  end

  @doc "Return scoped tool sets observed by `send_turn/3`, oldest-first."
  @spec turn_scopes() :: [MapSet.t(atom())]
  def turn_scopes do
    @turn_scopes_key
    |> Process.get([])
    |> Enum.reverse()
  end

  @doc "Return how many times `start_session/1` ran in this process."
  @spec start_count() :: non_neg_integer()
  def start_count do
    Process.get(@start_count_key, 0)
  end

  @impl true
  @spec adapter_kind() :: atom()
  def adapter_kind, do: :mock

  @impl true
  @spec tool_registry() :: [atom()]
  def tool_registry, do: @tool_registry

  @impl true
  @spec start_session(AdapterSession.t()) :: {:ok, map()} | {:error, EffectError.t()}
  def start_session(%AdapterSession{session_id: sid}) do
    increment_start_count()

    {:ok,
     %{
       thread_id: "mock-thread-" <> sid,
       turn_number: 0
     }}
  end

  @impl true
  @spec send_turn(map(), String.t(), AdapterSession.t()) ::
          {:ok, map()} | {:error, EffectError.t()}
  def send_turn(%{thread_id: _thread_id} = _handle, prompt, %AdapterSession{} = session)
      when is_binary(prompt) do
    turn_number = next_turn_number()

    turn_number
    |> default_reply(session, prompt)
    |> Map.merge(scripted_reply(turn_number))
    |> tap(fn _reply -> record_prompt(prompt) end)
    |> tap(fn _reply -> record_scope(session.scoped_tools) end)
    |> tap(&run_after_turn(turn_number, &1))
    |> then(&{:ok, &1})
  end

  @impl true
  @spec dispatch_tool(map(), atom(), map(), AdapterSession.t()) ::
          {:ok, map()} | {:error, EffectError.t()}
  def dispatch_tool(_handle, tool, args, %AdapterSession{} = session)
      when tool in @tool_registry do
    case AgentAdapter.ensure_in_scope(session, tool, args) do
      :ok ->
        {:ok,
         %{
           call_id: "mock-call-" <> Atom.to_string(tool),
           result: %{success: true, tool: tool}
         }}

      {:error, _} = err ->
        err
    end
  end

  # Function-clause fallback — CL1 "tool unknown to adapter" is
  # function-clause-enforced, not runtime-guarded.
  def dispatch_tool(_handle, _tool, _args, %AdapterSession{}),
    do: {:error, :tool_unknown_to_adapter}

  @impl true
  @spec close_session(map()) :: :ok
  def close_session(%{thread_id: _}), do: :ok

  @spec increment_start_count() :: non_neg_integer()
  defp increment_start_count do
    @start_count_key
    |> Process.get(0)
    |> Kernel.+(1)
    |> tap(&Process.put(@start_count_key, &1))
  end

  @spec next_turn_number() :: pos_integer()
  defp next_turn_number do
    @turn_count_key
    |> Process.get(0)
    |> Kernel.+(1)
    |> tap(&Process.put(@turn_count_key, &1))
  end

  @spec default_reply(pos_integer(), AdapterSession.t(), String.t()) :: map()
  defp default_reply(turn_number, session, prompt) do
    %{
      turn_id: "mock-turn-" <> Integer.to_string(turn_number),
      status: :completed,
      events: default_events(session, prompt),
      usage: %{input_tokens: 100, output_tokens: 50, total_tokens: 150},
      text: "mock agent output"
    }
  end

  @spec default_events(AdapterSession.t(), String.t()) :: [map()]
  defp default_events(%AdapterSession{} = session, prompt) do
    session
    |> measure_session?(prompt)
    |> measure_events()
  end

  @spec measure_session?(AdapterSession.t(), String.t()) :: boolean()
  defp measure_session?(%AdapterSession{scoped_tools: scoped_tools}, prompt) do
    scoped_tools
    |> measure_scoped_tools?()
    |> Kernel.or(measure_prompt?(prompt))
  end

  @spec measure_scoped_tools?(MapSet.t(atom())) :: boolean()
  defp measure_scoped_tools?(scoped_tools) do
    scoped_tools
    |> MapSet.member?(:haft_refresh)
    |> Kernel.and(MapSet.member?(scoped_tools, :haft_decision))
  end

  @spec measure_prompt?(String.t()) :: boolean()
  defp measure_prompt?(prompt) do
    prompt
    |> String.downcase()
    |> measure_prompt_text?()
  end

  @spec measure_prompt_text?(String.t()) :: boolean()
  defp measure_prompt_text?(prompt) do
    prompt
    |> String.contains?("measure")
    |> Kernel.or(String.contains?(prompt, "evidence"))
  end

  @spec measure_events(boolean()) :: [map()]
  defp measure_events(true), do: [measure_evidence_event()]
  defp measure_events(false), do: []

  @spec measure_evidence_event() :: map()
  defp measure_evidence_event do
    %{
      payload: %{
        "item" => %{
          "type" => "commandExecution",
          "status" => "completed",
          "command" => "mock measure",
          "exitCode" => 0,
          "aggregatedOutput" => "mock evidence\n"
        }
      }
    }
  end

  @spec scripted_reply(pos_integer()) :: map()
  defp scripted_reply(turn_number) do
    @turn_replies_key
    |> Process.get([])
    |> Enum.at(turn_number - 1, %{})
  end

  @spec record_prompt(String.t()) :: :ok
  defp record_prompt(prompt) do
    prompts = Process.get(@turn_prompts_key, [])
    Process.put(@turn_prompts_key, [prompt | prompts])
    :ok
  end

  @spec record_scope(MapSet.t(atom())) :: :ok
  defp record_scope(scoped_tools) do
    scopes = Process.get(@turn_scopes_key, [])
    Process.put(@turn_scopes_key, [scoped_tools | scopes])
    :ok
  end

  @spec run_after_turn(pos_integer(), map()) :: :ok
  defp run_after_turn(turn_number, reply) do
    case Process.get(@after_turn_key) do
      fun when is_function(fun, 2) -> fun.(turn_number, reply)
      _ -> :ok
    end

    :ok
  end
end
