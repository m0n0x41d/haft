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

  alias OpenSleigh.{AdapterSession, EffectError}

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

  @doc """
  Runtime per-phase-scope check. Called by `dispatch_tool/4` impls
  before dispatching to the underlying transport. Catches CL2 /
  CL3 — phase-scope violations.
  """
  @spec ensure_in_scope(AdapterSession.t(), atom()) ::
          :ok | {:error, :tool_forbidden_by_phase_scope}
  def ensure_in_scope(%AdapterSession{scoped_tools: scoped}, tool)
      when is_atom(tool) do
    if MapSet.member?(scoped, tool),
      do: :ok,
      else: {:error, :tool_forbidden_by_phase_scope}
  end
end
