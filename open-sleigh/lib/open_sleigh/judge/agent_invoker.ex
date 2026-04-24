defmodule OpenSleigh.Judge.AgentInvoker do
  @moduledoc """
  Narrow L4 bridge from semantic gates to an `OpenSleigh.Agent.Adapter`.

  `JudgeClient` owns the gate contract: calibration, prompt rendering, and
  gate-specific response parsing. This module owns only the provider call:
  create a judge-scoped adapter session, ask for a JSON object, parse the
  adapter text reply, and close the session.
  """

  alias OpenSleigh.{AdapterSession, ConfigHash, EffectError, SessionId}

  @type config :: %{
          required(:adapter) => module(),
          required(:workspace_path) => Path.t(),
          required(:config_hash) => ConfigHash.t(),
          required(:adapter_version) => String.t(),
          required(:max_tokens_per_turn) => pos_integer(),
          required(:wall_clock_timeout_s) => pos_integer()
        }

  @type invoke_result :: {:ok, map()} | {:error, EffectError.t()}

  @doc "Return a `JudgeClient.invoke_fun/0` compatible closure."
  @spec invoke_fun(config()) :: OpenSleigh.JudgeClient.invoke_fun()
  def invoke_fun(%{} = config) do
    fn prompt -> invoke(prompt, config) end
  end

  @doc "Invoke the configured agent adapter once and parse a JSON object reply."
  @spec invoke(String.t(), config()) :: invoke_result()
  def invoke(prompt, %{adapter: adapter} = config) when is_binary(prompt) and is_atom(adapter) do
    with :ok <- ensure_workspace(config),
         {:ok, session} <- adapter_session(config),
         {:ok, handle} <- adapter.start_session(session) do
      handle
      |> send_judge_turn(adapter, prompt, session)
      |> close_after(adapter, handle)
    end
  end

  def invoke(_prompt, _config), do: {:error, :judge_unavailable}

  @spec ensure_workspace(config()) :: :ok | {:error, EffectError.t()}
  defp ensure_workspace(%{workspace_path: workspace_path}) when is_binary(workspace_path) do
    workspace_path
    |> File.mkdir_p()
    |> ensure_workspace_result()
  end

  defp ensure_workspace(_config), do: {:error, :judge_unavailable}

  @spec ensure_workspace_result(:ok | {:error, term()}) :: :ok | {:error, EffectError.t()}
  defp ensure_workspace_result(:ok), do: :ok
  defp ensure_workspace_result({:error, _reason}), do: {:error, :judge_unavailable}

  @spec config(module(), Path.t(), ConfigHash.t(), map()) :: config()
  def config(adapter, workspace_path, config_hash, attrs)
      when is_atom(adapter) and is_binary(workspace_path) and is_map(attrs) do
    %{
      adapter: adapter,
      workspace_path: workspace_path,
      config_hash: config_hash,
      adapter_version:
        config_string(Map.get(attrs, :adapter_version, Map.get(attrs, "adapter_version", "mvp1"))),
      max_tokens_per_turn:
        positive_integer(
          Map.get(attrs, :max_tokens_per_turn, Map.get(attrs, "max_tokens_per_turn", 4_000))
        ),
      wall_clock_timeout_s:
        positive_integer(
          Map.get(attrs, :wall_clock_timeout_s, Map.get(attrs, "wall_clock_timeout_s", 120))
        )
    }
  end

  @spec adapter_session(config()) :: {:ok, AdapterSession.t()} | {:error, EffectError.t()}
  defp adapter_session(%{
         adapter: adapter,
         workspace_path: workspace_path,
         config_hash: config_hash,
         adapter_version: adapter_version,
         max_tokens_per_turn: max_tokens_per_turn,
         wall_clock_timeout_s: wall_clock_timeout_s
       }) do
    %{
      session_id: SessionId.generate(),
      config_hash: config_hash,
      scoped_tools: MapSet.new(),
      workspace_path: workspace_path,
      adapter_kind: adapter.adapter_kind(),
      adapter_version: adapter_version,
      max_turns: 1,
      max_tokens_per_turn: max_tokens_per_turn,
      wall_clock_timeout_s: wall_clock_timeout_s
    }
    |> AdapterSession.new()
    |> adapter_session_result()
  end

  @spec adapter_session_result({:ok, AdapterSession.t()} | {:error, atom()}) ::
          {:ok, AdapterSession.t()} | {:error, EffectError.t()}
  defp adapter_session_result({:ok, _session} = result), do: result
  defp adapter_session_result({:error, _reason}), do: {:error, :judge_unavailable}

  @spec send_judge_turn(map(), module(), String.t(), AdapterSession.t()) :: invoke_result()
  defp send_judge_turn(handle, adapter, prompt, session) do
    handle
    |> adapter.send_turn(judge_prompt(prompt), session)
    |> parse_reply()
  end

  @spec close_after(invoke_result(), module(), map()) :: invoke_result()
  defp close_after(result, adapter, handle) do
    :ok = adapter.close_session(handle)
    result
  end

  @spec parse_reply({:ok, map()} | {:error, EffectError.t()}) :: invoke_result()
  defp parse_reply({:ok, %{status: :completed, text: text}}) when is_binary(text) do
    text
    |> extract_json_object()
    |> decode_json_object()
  end

  defp parse_reply({:ok, %{status: :timeout}}), do: {:error, :turn_timeout}
  defp parse_reply({:ok, %{status: :failed}}), do: {:error, :judge_unavailable}
  defp parse_reply({:ok, %{status: :cancelled}}), do: {:error, :judge_unavailable}
  defp parse_reply({:ok, _reply}), do: {:error, :judge_response_malformed}
  defp parse_reply({:error, _reason} = error), do: error

  @spec extract_json_object(String.t()) :: String.t()
  defp extract_json_object(text) do
    case Regex.run(~r/\{.*\}/s, text) do
      [json] -> json
      _ -> text
    end
  end

  @spec decode_json_object(String.t()) :: invoke_result()
  defp decode_json_object(text) do
    case Jason.decode(text) do
      {:ok, decoded} when is_map(decoded) -> {:ok, decoded}
      _ -> {:error, :judge_response_malformed}
    end
  end

  @spec judge_prompt(String.t()) :: String.t()
  defp judge_prompt(prompt) do
    """
    You are Open-Sleigh's semantic gate judge.

    Return only one JSON object with exactly these keys:
    {"verdict":"pass|fail","cl":0-3,"rationale":"one sentence"}

    #{prompt}
    """
    |> String.trim()
  end

  @spec positive_integer(term()) :: pos_integer()
  defp positive_integer(value) when is_integer(value) and value >= 1, do: value

  defp positive_integer(value) when is_binary(value) do
    value
    |> Integer.parse()
    |> positive_integer_parse_result()
  end

  defp positive_integer(_value), do: 1

  @spec positive_integer_parse_result({integer(), String.t()} | :error) :: pos_integer()
  defp positive_integer_parse_result({value, ""}) when value >= 1, do: value
  defp positive_integer_parse_result(_result), do: 1

  @spec config_string(term()) :: String.t()
  defp config_string(value) when is_binary(value), do: value
  defp config_string(value) when is_atom(value), do: Atom.to_string(value)
  defp config_string(value), do: to_string(value)
end
