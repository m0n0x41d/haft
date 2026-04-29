defmodule OpenSleigh.Agent.Protocol do
  @moduledoc """
  Pure JSON-RPC 2.0 codec for the agent app-server handshake + turn
  protocol (per `specs/target-system/AGENT_PROTOCOL.md §1–§4`).

  Stateless. This module:

  * **Encodes** outgoing requests (initialize / initialized /
    thread_start / turn_start / tool_result) into JSON-RPC messages.
  * **Decodes** inbound lines into typed `AgentEvent` shapes.
  * Does NOT call `:gen_tcp`, `Port.open`, or any I/O. The L5
    `AgentWorker` owns transport; this module is the codec.

  Testable without subprocesses. All functions are pure and
  property-friendly.
  """

  alias OpenSleigh.AdapterSession

  @typedoc "A JSON-RPC request envelope."
  @type rpc_request :: %{
          required(String.t()) => String.t() | integer() | map()
        }

  # ——— encoders ———

  @doc """
  Encode the `initialize` request (handshake step 1).
  """
  @spec encode_initialize(id :: integer()) :: binary()
  def encode_initialize(id) when is_integer(id) do
    encode_rpc(%{
      "jsonrpc" => "2.0",
      "id" => id,
      "method" => "initialize",
      "params" => %{
        "clientInfo" => %{"name" => "open-sleigh", "version" => "0.1"},
        "capabilities" => %{"experimentalApi" => true}
      }
    })
  end

  @doc "Encode the `initialized` notification (handshake step 2)."
  @spec encode_initialized() :: binary()
  def encode_initialized do
    encode_rpc(%{"jsonrpc" => "2.0", "method" => "initialized", "params" => %{}})
  end

  @doc """
  Encode the `thread/start` request (handshake step 3). `tools` is
  the phase-scoped tool atom list from `AdapterSession.scoped_tools`.
  """
  @spec encode_thread_start(integer(), AdapterSession.t(), %{
          optional(:approval_policy) => String.t(),
          optional(:sandbox) => String.t()
        }) :: binary()
  def encode_thread_start(id, %AdapterSession{} = session, opts \\ %{}) do
    tools = session.scoped_tools |> MapSet.to_list() |> Enum.map(&Atom.to_string/1)

    encode_rpc(%{
      "jsonrpc" => "2.0",
      "id" => id,
      "method" => "thread/start",
      "params" => %{
        "approvalPolicy" => Map.get(opts, :approval_policy, "never"),
        "sandbox" => Map.get(opts, :sandbox, "workspace-write"),
        "cwd" => session.workspace_path,
        "tools" => tools
      }
    })
  end

  @doc """
  Encode a `turn/start` request. `prompt` is the full rendered
  prompt on turn 1 or continuation guidance on subsequent turns.
  """
  @spec encode_turn_start(integer(), AdapterSession.t(), thread_id :: String.t(), String.t(), %{
          optional(:title) => String.t()
        }) :: binary()
  def encode_turn_start(id, %AdapterSession{} = session, thread_id, prompt, opts \\ %{}) do
    encode_rpc(%{
      "jsonrpc" => "2.0",
      "id" => id,
      "method" => "turn/start",
      "params" => %{
        "threadId" => thread_id,
        "input" => [%{"type" => "text", "text" => prompt_with_hash(prompt, session)}],
        "cwd" => session.workspace_path,
        "title" => Map.get(opts, :title, "open-sleigh turn"),
        "approvalPolicy" => Map.get(opts, :approval_policy, "never"),
        "sandboxPolicy" => Map.get(opts, :sandbox_policy, %{"type" => "workspaceWrite"})
      }
    })
  end

  @doc """
  Append the session's `config_hash` as a trailer comment to the
  rendered prompt (per SPEC §8.1 provenance).
  """
  @spec prompt_with_hash(String.t(), AdapterSession.t()) :: String.t()
  def prompt_with_hash(prompt, %AdapterSession{config_hash: hash}) do
    prompt <> "\n\n<!-- config_hash: " <> hash <> " -->\n"
  end

  # ——— decoders ———

  @typedoc """
  Normalised agent event. The `event` atom is one of the categories
  in AGENT_PROTOCOL.md §4. Unknown events decode to
  `:unsupported_event_category`.
  """
  @type event :: %{
          required(:event) => atom(),
          required(:timestamp) => DateTime.t(),
          optional(any()) => any()
        }

  @doc """
  Decode a single line of agent stdout into either a response tied
  to a request id, or a streaming event.

  Returns `{:ok, {:response, id, result}}` for request/response
  pairs, `{:ok, {:event, event}}` for notifications, or
  `{:error, :response_parse_error}` on malformed JSON.
  """
  @spec decode_line(binary(), now :: (-> DateTime.t())) ::
          {:ok, {:response, integer(), map()}}
          | {:ok, {:event, event()}}
          | {:error, :response_parse_error}
  def decode_line(line, now_fun \\ &default_now/0) when is_binary(line) do
    case parse_json(line) do
      {:ok, %{"id" => id, "result" => result}} when is_integer(id) ->
        {:ok, {:response, id, result}}

      {:ok, %{"method" => method} = msg} when is_binary(method) ->
        {:ok, {:event, normalise_event(msg, now_fun.())}}

      {:ok, _} ->
        {:error, :response_parse_error}

      :error ->
        {:error, :response_parse_error}
    end
  end

  @spec normalise_event(map(), DateTime.t()) :: event()
  defp normalise_event(%{"method" => method} = msg, now) do
    category = method_to_event_category(method)
    payload = Map.get(msg, "params", %{})

    %{
      event: category,
      timestamp: now,
      payload: payload
    }
  end

  @spec method_to_event_category(String.t()) :: atom()
  defp method_to_event_category("session/started"), do: :session_started
  defp method_to_event_category("turn/started"), do: :turn_started
  defp method_to_event_category("turn/completed"), do: :turn_completed
  defp method_to_event_category("turn/failed"), do: :turn_failed
  defp method_to_event_category("turn/cancelled"), do: :turn_cancelled
  defp method_to_event_category("turn/inputRequired"), do: :turn_input_required
  defp method_to_event_category("notification"), do: :notification
  defp method_to_event_category("item/tool/call"), do: :tool_call
  defp method_to_event_category("item/tool/result"), do: :tool_result
  defp method_to_event_category("thread/tokenUsage/updated"), do: :usage
  defp method_to_event_category("rateLimit/updated"), do: :rate_limit
  defp method_to_event_category(_), do: :unsupported_event_category

  # ——— helpers ———

  @spec encode_rpc(map()) :: binary()
  defp encode_rpc(map), do: Jason.encode!(map) <> "\n"

  @spec parse_json(binary()) :: {:ok, map()} | :error
  defp parse_json(line) do
    case Jason.decode(line) do
      {:ok, decoded} when is_map(decoded) -> {:ok, decoded}
      _ -> :error
    end
  end

  @spec default_now() :: DateTime.t()
  defp default_now, do: DateTime.utc_now()
end
