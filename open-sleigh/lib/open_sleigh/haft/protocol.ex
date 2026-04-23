defmodule OpenSleigh.Haft.Protocol do
  @moduledoc """
  Pure JSON-RPC 2.0 codec for the Haft MCP contract.

  Per `specs/target-system/HAFT_CONTRACT.md`: 7 MCP tools (`haft_problem`,
  `haft_solution`, `haft_decision`, `haft_commission`, `haft_refresh`,
  `haft_note`, `haft_query`). Transport is line-delimited JSON on stdio, just
  like the agent app-server — but semantics are different (request/
  response only, no streaming events).

  Stateless. No I/O. The L5 `HaftServer` GenServer owns the
  subprocess and uses this module to encode/decode messages.
  """

  alias OpenSleigh.{AdapterSession, EffectError}

  @type tool ::
          :haft_problem
          | :haft_solution
          | :haft_decision
          | :haft_refresh
          | :haft_note
          | :haft_query
          | :haft_commission

  @valid_tools [
    :haft_problem,
    :haft_solution,
    :haft_decision,
    :haft_refresh,
    :haft_note,
    :haft_query,
    :haft_commission
  ]

  @doc "Encode the MCP `initialize` request."
  @spec encode_initialize(integer()) :: binary()
  def encode_initialize(id) when is_integer(id) do
    Jason.encode!(%{
      "jsonrpc" => "2.0",
      "id" => id,
      "method" => "initialize",
      "params" => %{
        "protocolVersion" => "2024-11-05",
        "capabilities" => %{},
        "clientInfo" => %{"name" => "open-sleigh", "version" => "0.1"}
      }
    }) <> "\n"
  end

  @doc "Encode the MCP initialized notification."
  @spec encode_initialized() :: binary()
  def encode_initialized do
    Jason.encode!(%{
      "jsonrpc" => "2.0",
      "method" => "notifications/initialized",
      "params" => %{}
    }) <> "\n"
  end

  @doc """
  Encode a tool call. Attaches the session's `config_hash` so every
  write is traceable to the exact effective `sleigh.md` slice
  (SPEC §8.1 provenance, SE7).
  """
  @spec encode_call(integer(), tool(), atom(), map(), AdapterSession.t()) ::
          {:ok, binary()} | {:error, EffectError.t()}
  def encode_call(id, tool, action, params, %AdapterSession{config_hash: hash})
      when is_integer(id) and tool in @valid_tools and is_atom(action) and is_map(params) do
    body =
      Jason.encode!(%{
        "jsonrpc" => "2.0",
        "id" => id,
        "method" => "tools/call",
        "params" => %{
          "name" => Atom.to_string(tool),
          "arguments" =>
            params
            |> Map.put("action", Atom.to_string(action))
            |> Map.put("config_hash", hash)
        }
      })

    {:ok, body <> "\n"}
  end

  def encode_call(_id, tool, _action, _params, _session) when tool not in @valid_tools do
    {:error, :tool_unknown_to_adapter}
  end

  def encode_call(_id, _tool, _action, _params, _session) do
    {:error, :tool_arg_invalid}
  end

  @doc "Encode a raw Haft health ping. Used by the L5 process owner."
  @spec encode_health_ping(integer()) :: binary()
  def encode_health_ping(id) when is_integer(id) do
    Jason.encode!(%{
      "jsonrpc" => "2.0",
      "id" => id,
      "method" => "tools/call",
      "params" => %{
        "name" => "haft_query",
        "arguments" => %{"action" => "status", "config_hash" => "open-sleigh-health"}
      }
    }) <> "\n"
  end

  @doc """
  Decode a response line. Returns `{:ok, {id, result}}` on a valid
  response, `{:error, EffectError.t()}` otherwise.
  """
  @spec decode_response(binary()) ::
          {:ok, {integer(), map()}} | {:error, EffectError.t()}
  def decode_response(line) when is_binary(line) do
    case Jason.decode(line) do
      {:ok, %{"id" => id, "result" => result}} when is_integer(id) and is_map(result) ->
        {:ok, {id, result}}

      {:ok, %{"id" => id, "error" => %{"message" => msg}}} when is_integer(id) ->
        _ = msg
        {:error, :haft_unavailable}

      _ ->
        {:error, :response_parse_error}
    end
  end

  @doc "Which atoms are admitted as tool names."
  @spec valid_tools() :: [tool(), ...]
  def valid_tools, do: @valid_tools
end
