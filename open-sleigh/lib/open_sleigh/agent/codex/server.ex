defmodule OpenSleigh.Agent.Codex.Server do
  @moduledoc """
  L5 GenServer that owns one `codex app-server` subprocess Port.

  The server speaks the app-server JSON-RPC protocol over stdio using
  `OpenSleigh.Agent.Protocol` for request encoding and event
  normalisation. It is intentionally session-scoped: one server owns
  one live app-server thread for one `(Ticket × Phase)` session.
  """

  use GenServer

  alias OpenSleigh.{AdapterSession, EffectError}
  alias OpenSleigh.Agent.Protocol
  alias OpenSleigh.Agent.ToolRuntime

  @max_line_bytes 10_000_000
  @initialize_id 1
  @thread_start_id 2
  @non_interactive_tool_input_answer "This is a non-interactive Open-Sleigh session. Operator input is unavailable."
  @crash_dump_dir_env "OPEN_SLEIGH_CRASH_DUMP_DIR"

  @type state :: %{
          required(:command) => String.t(),
          required(:read_timeout_ms) => pos_integer(),
          required(:turn_timeout_ms) => pos_integer(),
          required(:stall_timeout_ms) => integer(),
          required(:approval_policy) => String.t(),
          required(:auto_approve_requests) => boolean(),
          required(:thread_sandbox) => String.t(),
          required(:turn_sandbox_policy) => map(),
          required(:next_id) => pos_integer(),
          optional(:dynamic_tool_timeout_ms) => pos_integer(),
          optional(:haft_invoker) => (binary() -> {:ok, binary()} | {:error, EffectError.t()}),
          optional(:port) => port(),
          optional(:thread_id) => String.t(),
          optional(:app_server_pid) => String.t(),
          optional(:session) => AdapterSession.t()
        }

  @spec start_link(keyword()) :: GenServer.on_start()
  def start_link(opts) when is_list(opts) do
    GenServer.start_link(__MODULE__, opts)
  end

  @spec start_session(pid(), AdapterSession.t()) ::
          {:ok, map()} | {:error, EffectError.t()}
  def start_session(server, %AdapterSession{} = session) when is_pid(server) do
    GenServer.call(server, {:start_session, session}, :infinity)
  end

  @spec dispatch_tool(pid(), atom(), map(), AdapterSession.t()) ::
          {:ok, map()} | {:error, EffectError.t()}
  def dispatch_tool(server, tool, args, %AdapterSession{} = session)
      when is_pid(server) and is_atom(tool) and is_map(args) do
    GenServer.call(server, {:dispatch_tool, tool, args, session}, :infinity)
  end

  @spec close(pid()) :: :ok
  def close(server) when is_pid(server) do
    GenServer.call(server, :close, 5_000)
  catch
    :exit, _reason -> :ok
  end

  @impl true
  def init(opts) do
    state =
      opts
      |> Map.new()
      |> initial_state()

    {:ok, state}
  end

  @impl true
  def handle_call({:start_session, %AdapterSession{} = session}, _from, state) do
    result =
      session
      |> open_port(state)
      |> initialize_thread(session, state)

    reply_start_result(result, state)
  end

  def handle_call({:send_turn, prompt, %AdapterSession{} = session}, _from, state)
      when is_binary(prompt) do
    result =
      state
      |> start_turn(session, prompt)
      |> await_turn_result(state)

    reply_turn_result(result, state)
  end

  def handle_call({:dispatch_tool, tool, args, %AdapterSession{} = session}, _from, state) do
    result = tool_dispatch_result(tool, args, session, state)
    {:reply, result, state}
  end

  def handle_call(:close, _from, state) do
    close_port(state)
    {:stop, :normal, :ok, state}
  end

  @impl true
  def terminate(_reason, state) do
    close_port(state)
    :ok
  end

  @impl true
  def handle_info({port, {:data, {_mode, _chunk}}}, %{port: port} = state) do
    {:noreply, state}
  end

  def handle_info({port, {:exit_status, _status}}, %{port: port} = state) do
    {:noreply, Map.delete(state, :port)}
  end

  def handle_info(_message, state) do
    {:noreply, state}
  end

  @spec initial_state(map()) :: state()
  defp initial_state(opts) do
    %{
      command: Map.get(opts, :command, "codex app-server"),
      read_timeout_ms: Map.get(opts, :read_timeout_ms, 5_000),
      turn_timeout_ms: Map.get(opts, :turn_timeout_ms, 3_600_000),
      stall_timeout_ms: Map.get(opts, :stall_timeout_ms, 300_000),
      approval_policy: Map.get(opts, :approval_policy, "never"),
      auto_approve_requests:
        opts
        |> Map.get(:approval_policy, "never")
        |> auto_approve_requests?(),
      thread_sandbox: Map.get(opts, :thread_sandbox, "workspace-write"),
      turn_sandbox_policy: Map.get(opts, :turn_sandbox_policy, %{"type" => "workspaceWrite"}),
      dynamic_tool_timeout_ms: Map.get(opts, :dynamic_tool_timeout_ms, 60_000),
      haft_invoker: Map.get(opts, :haft_invoker),
      next_id: 3
    }
  end

  @spec open_port(AdapterSession.t(), state()) :: {:ok, port(), map()} | {:error, EffectError.t()}
  defp open_port(%AdapterSession{workspace_path: cwd} = session, %{command: command}) do
    case System.find_executable("bash") do
      nil -> {:error, :agent_command_not_found}
      bash -> do_open_port(bash, command, cwd, session)
    end
  end

  @spec do_open_port(String.t(), String.t(), Path.t(), AdapterSession.t()) ::
          {:ok, port(), map()} | {:error, EffectError.t()}
  defp do_open_port(bash, command, cwd, %AdapterSession{} = session) do
    crash_dump_path = crash_dump_path(session)

    with :ok <- ensure_crash_dump_dir(crash_dump_path) do
      port =
        Port.open(
          {:spawn_executable, String.to_charlist(bash)},
          [
            :binary,
            :exit_status,
            :stderr_to_stdout,
            args: [~c"-lc", String.to_charlist(command)],
            cd: String.to_charlist(cwd),
            env: [{~c"ERL_CRASH_DUMP", String.to_charlist(crash_dump_path)}],
            line: @max_line_bytes
          ]
        )

      {:ok, port, port_metadata(port)}
    else
      {:error, _reason} -> {:error, :agent_launch_failed}
    end
  rescue
    _ -> {:error, :agent_launch_failed}
  end

  @spec ensure_crash_dump_dir(Path.t()) :: :ok | {:error, File.posix()}
  defp ensure_crash_dump_dir(path) do
    path
    |> Path.dirname()
    |> File.mkdir_p()
  end

  @spec crash_dump_path(AdapterSession.t()) :: Path.t()
  defp crash_dump_path(%AdapterSession{session_id: session_id}) do
    crash_dump_dir()
    |> Path.join("agent-#{filename_fragment(session_id)}.dump")
  end

  @spec crash_dump_dir() :: Path.t()
  defp crash_dump_dir do
    @crash_dump_dir_env
    |> System.get_env()
    |> crash_dump_dir_result()
  end

  @spec crash_dump_dir_result(String.t() | nil) :: Path.t()
  defp crash_dump_dir_result(value) when is_binary(value) do
    value
    |> String.trim()
    |> crash_dump_dir_value()
  end

  defp crash_dump_dir_result(_value), do: default_crash_dump_dir()

  @spec crash_dump_dir_value(String.t()) :: Path.t()
  defp crash_dump_dir_value(""), do: default_crash_dump_dir()

  defp crash_dump_dir_value(path) do
    path
    |> Path.expand()
  end

  @spec default_crash_dump_dir() :: Path.t()
  defp default_crash_dump_dir do
    case System.user_home() do
      home when is_binary(home) and byte_size(home) > 0 ->
        Path.join([home, ".open-sleigh", "crash_dumps"])

      _other ->
        Path.join(System.tmp_dir!(), "open-sleigh-crash-dumps")
    end
  end

  @spec filename_fragment(String.t()) :: String.t()
  defp filename_fragment(value) do
    value
    |> String.replace(~r/[^A-Za-z0-9_.-]+/, "-")
    |> String.trim("-")
    |> filename_fragment_result()
  end

  @spec filename_fragment_result(String.t()) :: String.t()
  defp filename_fragment_result(""), do: "unknown-session"
  defp filename_fragment_result(value), do: value

  @spec initialize_thread(
          {:ok, port(), map()} | {:error, EffectError.t()},
          AdapterSession.t(),
          state()
        ) :: {:ok, map(), state()} | {:error, EffectError.t()}
  defp initialize_thread({:ok, port, metadata}, session, state) do
    state =
      Map.merge(state, %{port: port, app_server_pid: Map.get(metadata, :codex_app_server_pid)})

    with :ok <- initialize_app_server(port, state),
         {:ok, thread_id} <- start_thread(port, session, state) do
      metadata =
        metadata
        |> Map.put(:thread_id, thread_id)
        |> Map.put(:server, self())

      new_state =
        state
        |> Map.put(:thread_id, thread_id)
        |> Map.put(:session, session)

      {:ok, metadata, new_state}
    else
      {:error, reason} ->
        close_port(state)
        {:error, reason}
    end
  end

  defp initialize_thread({:error, _reason} = error, _session, _state), do: error

  @spec initialize_app_server(port(), state()) :: :ok | {:error, EffectError.t()}
  defp initialize_app_server(port, state) do
    port
    |> send_line(Protocol.encode_initialize(@initialize_id))
    |> await_initialize_response(port, state)
  end

  @spec await_initialize_response(:ok, port(), state()) :: :ok | {:error, EffectError.t()}
  defp await_initialize_response(:ok, port, state) do
    case await_response(port, @initialize_id, state.read_timeout_ms, :handshake_timeout, "") do
      {:ok, _result} ->
        send_line(port, Protocol.encode_initialized())

      {:error, _reason} = error ->
        error
    end
  end

  @spec start_thread(port(), AdapterSession.t(), state()) ::
          {:ok, String.t()} | {:error, EffectError.t()}
  defp start_thread(port, session, state) do
    opts = %{approval_policy: state.approval_policy, sandbox: state.thread_sandbox}
    line = Protocol.encode_thread_start(@thread_start_id, session, opts)

    port
    |> send_line(line)
    |> await_thread_response(port, state)
  end

  @spec await_thread_response(:ok, port(), state()) ::
          {:ok, String.t()} | {:error, EffectError.t()}
  defp await_thread_response(:ok, port, state) do
    case await_response(port, @thread_start_id, state.read_timeout_ms, :thread_start_failed, "") do
      {:ok, %{"thread" => %{"id" => thread_id}}} when is_binary(thread_id) ->
        {:ok, thread_id}

      {:ok, _result} ->
        {:error, :thread_start_failed}

      {:error, _reason} = error ->
        error
    end
  end

  @spec start_turn(state(), AdapterSession.t(), String.t()) ::
          {:ok, String.t(), state()} | {:error, EffectError.t()}
  defp start_turn(%{port: port, thread_id: thread_id} = state, session, prompt) do
    id = state.next_id

    opts = %{
      title: session.session_id,
      approval_policy: state.approval_policy,
      sandbox_policy: state.turn_sandbox_policy
    }

    line = Protocol.encode_turn_start(id, session, thread_id, prompt, opts)

    port
    |> send_line(line)
    |> await_turn_start_response(port, id, state)
  end

  defp start_turn(_state, _session, _prompt), do: {:error, :initialize_failed}

  @spec await_turn_start_response(:ok, port(), pos_integer(), state()) ::
          {:ok, String.t(), state()} | {:error, EffectError.t()}
  defp await_turn_start_response(:ok, port, id, state) do
    case await_response(port, id, state.read_timeout_ms, :turn_start_failed, "") do
      {:ok, %{"turn" => %{"id" => turn_id}}} when is_binary(turn_id) ->
        {:ok, turn_id, %{state | next_id: id + 1}}

      {:ok, _result} ->
        {:error, :turn_start_failed}

      {:error, _reason} = error ->
        error
    end
  end

  @spec await_turn_result({:ok, String.t(), state()} | {:error, EffectError.t()}, state()) ::
          {:ok, map(), state()} | {:error, EffectError.t()}
  defp await_turn_result({:ok, turn_id, state}, _old_state) do
    deadlines = turn_deadlines(state)
    await_turn_event(state.port, turn_id, state, deadlines, [], "")
  end

  defp await_turn_result({:error, _reason} = error, _state), do: error

  @spec await_response(port(), integer(), pos_integer(), EffectError.t(), String.t()) ::
          {:ok, map()} | {:error, EffectError.t()}
  defp await_response(port, request_id, timeout_ms, timeout_error, pending_line) do
    receive do
      {^port, {:data, {:eol, chunk}}} ->
        pending_line
        |> Kernel.<>(to_string(chunk))
        |> handle_response_line(port, request_id, timeout_ms, timeout_error)

      {^port, {:data, {:noeol, chunk}}} ->
        await_response(
          port,
          request_id,
          timeout_ms,
          timeout_error,
          pending_line <> to_string(chunk)
        )

      {^port, {:exit_status, _status}} ->
        {:error, :port_exit_unexpected}
    after
      timeout_ms ->
        {:error, timeout_error}
    end
  end

  @spec handle_response_line(String.t(), port(), integer(), pos_integer(), EffectError.t()) ::
          {:ok, map()} | {:error, EffectError.t()}
  defp handle_response_line(line, port, request_id, timeout_ms, timeout_error) do
    line
    |> decode_rpc_line()
    |> handle_response_decode(port, request_id, timeout_ms, timeout_error)
  end

  @spec handle_response_decode(term(), port(), integer(), pos_integer(), EffectError.t()) ::
          {:ok, map()} | {:error, EffectError.t()}
  defp handle_response_decode(
         {:response, request_id, result},
         _port,
         request_id,
         _timeout_ms,
         _timeout_error
       ) do
    {:ok, result}
  end

  defp handle_response_decode(
         {:rpc_error, request_id, _error},
         _port,
         request_id,
         _timeout_ms,
         timeout_error
       ) do
    {:error, timeout_error}
  end

  defp handle_response_decode(:malformed, _port, _request_id, _timeout_ms, _timeout_error) do
    {:error, :response_parse_error}
  end

  defp handle_response_decode(_decoded, port, request_id, timeout_ms, timeout_error) do
    await_response(port, request_id, timeout_ms, timeout_error, "")
  end

  @typep deadlines :: %{
           required(:turn_deadline) => integer(),
           required(:stall_deadline) => integer() | :disabled
         }

  @typep turn_wait :: %{
           required(:port) => port(),
           required(:turn_id) => String.t(),
           required(:state) => state(),
           required(:deadlines) => deadlines(),
           required(:events) => [map()],
           required(:pending_line) => String.t()
         }

  @spec turn_deadlines(state()) :: deadlines()
  defp turn_deadlines(state) do
    now = monotonic_ms()

    %{
      turn_deadline: now + state.turn_timeout_ms,
      stall_deadline: stall_deadline(now, state.stall_timeout_ms)
    }
  end

  @spec stall_deadline(integer(), integer()) :: integer() | :disabled
  defp stall_deadline(_now, stall_timeout_ms) when stall_timeout_ms <= 0, do: :disabled
  defp stall_deadline(now, stall_timeout_ms), do: now + stall_timeout_ms

  @spec await_turn_event(port(), String.t(), state(), deadlines(), [map()], String.t()) ::
          {:ok, map(), state()} | {:error, EffectError.t()}
  defp await_turn_event(port, turn_id, state, deadlines, events, pending_line) do
    await_turn_event(%{
      port: port,
      turn_id: turn_id,
      state: state,
      deadlines: deadlines,
      events: events,
      pending_line: pending_line
    })
  end

  @spec await_turn_event(turn_wait()) :: {:ok, map(), state()} | {:error, EffectError.t()}
  defp await_turn_event(wait) do
    case next_receive_timeout(wait.deadlines) do
      {:expired, reason} ->
        {:error, reason}

      {:wait, timeout_ms} ->
        receive_turn_event(wait, timeout_ms)
    end
  end

  @spec receive_turn_event(turn_wait(), non_neg_integer()) ::
          {:ok, map(), state()} | {:error, EffectError.t()}
  defp receive_turn_event(%{port: port} = wait, timeout_ms) do
    receive do
      {^port, {:data, {:eol, chunk}}} ->
        wait.pending_line
        |> Kernel.<>(to_string(chunk))
        |> handle_turn_line(%{wait | pending_line: ""})

      {^port, {:data, {:noeol, chunk}}} ->
        await_turn_event(%{wait | pending_line: wait.pending_line <> to_string(chunk)})

      {^port, {:exit_status, _status}} ->
        {:error, :port_exit_unexpected}
    after
      timeout_ms ->
        timeout_after_wait(wait.deadlines)
    end
  end

  @spec handle_turn_line(String.t(), turn_wait()) ::
          {:ok, map(), state()} | {:error, EffectError.t()}
  defp handle_turn_line(line, wait) do
    line
    |> decode_rpc_line()
    |> handle_turn_decode(line, wait)
  end

  @spec handle_turn_decode(term(), String.t(), turn_wait()) ::
          {:ok, map(), state()} | {:error, EffectError.t()}
  defp handle_turn_decode({:client_request, id, method, params}, _line, wait) do
    case respond_to_client_request(wait.port, id, method, params, wait.state) do
      :ok ->
        deadlines = reset_stall_deadline(wait.deadlines, wait.state)
        await_turn_event(%{wait | deadlines: deadlines})

      {:error, _reason} = error ->
        error
    end
  end

  defp handle_turn_decode({:event, event}, _line, wait) do
    deadlines = reset_stall_deadline(wait.deadlines, wait.state)
    handle_turn_event(event, %{wait | deadlines: deadlines})
  end

  defp handle_turn_decode({:rpc_error, _id, _error}, _line, _wait) do
    {:error, :response_parse_error}
  end

  defp handle_turn_decode(:malformed, line, wait) do
    await_turn_event(%{wait | events: [malformed_event(line) | wait.events]})
  end

  defp handle_turn_decode(_decoded, _line, wait) do
    await_turn_event(wait)
  end

  @spec handle_turn_event(map(), turn_wait()) ::
          {:ok, map(), state()} | {:error, EffectError.t()}
  defp handle_turn_event(%{event: :turn_completed} = event, wait) do
    {:ok, completed_reply(wait.turn_id, event, wait.events), wait.state}
  end

  defp handle_turn_event(%{event: :turn_failed} = event, wait) do
    {:ok, terminal_reply(:failed, wait.turn_id, event, wait.events), wait.state}
  end

  defp handle_turn_event(%{event: :turn_cancelled} = event, wait) do
    {:ok, terminal_reply(:cancelled, wait.turn_id, event, wait.events), wait.state}
  end

  defp handle_turn_event(%{event: :turn_input_required}, _wait) do
    {:error, :turn_input_required}
  end

  defp handle_turn_event(event, wait) do
    await_turn_event(%{wait | events: [event | wait.events]})
  end

  @spec completed_reply(String.t(), map(), [map()]) :: map()
  defp completed_reply(turn_id, event, events) do
    all_events = Enum.reverse([event | events])

    %{
      turn_id: event_turn_id(event, turn_id),
      status: :completed,
      events: all_events,
      usage: event_usage(event),
      text: reply_text(all_events)
    }
  end

  @spec terminal_reply(:failed | :cancelled, String.t(), map(), [map()]) :: map()
  defp terminal_reply(status, turn_id, event, events) do
    all_events = Enum.reverse([event | events])

    %{
      turn_id: event_turn_id(event, turn_id),
      status: status,
      events: all_events,
      usage: event_usage(event),
      text: reply_text(all_events)
    }
  end

  @spec event_turn_id(map(), String.t()) :: String.t()
  defp event_turn_id(%{payload: %{"turn" => %{"id" => turn_id}}}, _fallback)
       when is_binary(turn_id),
       do: turn_id

  defp event_turn_id(%{payload: %{"turn_id" => turn_id}}, _fallback) when is_binary(turn_id),
    do: turn_id

  defp event_turn_id(_event, fallback), do: fallback

  @spec event_usage(map()) :: map()
  defp event_usage(%{payload: %{"usage" => usage}}) when is_map(usage), do: usage
  defp event_usage(%{payload: %{"turn" => %{"usage" => usage}}}) when is_map(usage), do: usage
  defp event_usage(_event), do: %{}

  @spec reply_text([map()]) :: String.t()
  defp reply_text(events) do
    events
    |> preferred_reply_text()
    |> fallback_reply_text()
  end

  @spec preferred_reply_text([map()]) :: String.t() | nil
  defp preferred_reply_text(events) do
    [
      &turn_completed_text/1,
      &final_answer_text/1,
      &last_agent_message_text/1,
      &delta_text/1
    ]
    |> Enum.find_value(fn text_fun ->
      events
      |> text_fun.()
      |> blank_to_nil()
    end)
  end

  @spec turn_completed_text([map()]) :: String.t() | nil
  defp turn_completed_text(events) do
    events
    |> Enum.reverse()
    |> Enum.find_value(&turn_completed_event_text/1)
  end

  @spec turn_completed_event_text(map()) :: String.t() | nil
  defp turn_completed_event_text(%{event: :turn_completed, payload: %{"text" => text}})
       when is_binary(text),
       do: text

  defp turn_completed_event_text(%{event: :turn_failed, payload: %{"message" => text}})
       when is_binary(text),
       do: text

  defp turn_completed_event_text(_event), do: nil

  @spec final_answer_text([map()]) :: String.t() | nil
  defp final_answer_text(events) do
    events
    |> Enum.reverse()
    |> Enum.find_value(&final_answer_event_text/1)
  end

  @spec final_answer_event_text(map()) :: String.t() | nil
  defp final_answer_event_text(%{
         payload: %{
           "item" => %{"type" => "agentMessage", "phase" => "final_answer", "text" => text}
         }
       })
       when is_binary(text),
       do: text

  defp final_answer_event_text(_event), do: nil

  @spec last_agent_message_text([map()]) :: String.t() | nil
  defp last_agent_message_text(events) do
    events
    |> Enum.reverse()
    |> Enum.find_value(&agent_message_text/1)
  end

  @spec agent_message_text(map()) :: String.t() | nil
  defp agent_message_text(%{payload: %{"item" => %{"type" => "agentMessage", "text" => text}}})
       when is_binary(text),
       do: text

  defp agent_message_text(_event), do: nil

  @spec delta_text([map()]) :: String.t()
  defp delta_text(events) do
    events
    |> Enum.map(&delta_fragment/1)
    |> Enum.reject(&is_nil/1)
    |> Enum.join()
  end

  @spec delta_fragment(map()) :: String.t() | nil
  defp delta_fragment(%{payload: %{"delta" => delta}}) when is_binary(delta), do: delta
  defp delta_fragment(_event), do: nil

  @spec blank_to_nil(String.t() | nil) :: String.t() | nil
  defp blank_to_nil(nil), do: nil

  defp blank_to_nil(text) do
    if String.trim(text) == "", do: nil, else: text
  end

  @spec fallback_reply_text(String.t() | nil) :: String.t()
  defp fallback_reply_text(text) when is_binary(text), do: text
  defp fallback_reply_text(nil), do: "codex turn completed"

  @spec malformed_event(String.t()) :: map()
  defp malformed_event(line) do
    %{
      event: :malformed,
      timestamp: DateTime.utc_now(),
      payload: %{raw: line}
    }
  end

  @spec reset_stall_deadline(deadlines(), state()) :: deadlines()
  defp reset_stall_deadline(%{stall_deadline: :disabled} = deadlines, _state), do: deadlines

  defp reset_stall_deadline(deadlines, state) do
    %{deadlines | stall_deadline: monotonic_ms() + state.stall_timeout_ms}
  end

  @spec next_receive_timeout(deadlines()) ::
          {:wait, non_neg_integer()} | {:expired, EffectError.t()}
  defp next_receive_timeout(%{turn_deadline: turn_deadline, stall_deadline: stall_deadline}) do
    now = monotonic_ms()

    [
      {:turn_timeout, turn_deadline - now},
      stall_timeout_remaining(stall_deadline, now)
    ]
    |> Enum.reject(&is_nil/1)
    |> Enum.min_by(fn {_reason, remaining} -> remaining end)
    |> wait_or_expire()
  end

  @spec stall_timeout_remaining(integer() | :disabled, integer()) ::
          {:stall_timeout, integer()} | nil
  defp stall_timeout_remaining(:disabled, _now), do: nil
  defp stall_timeout_remaining(stall_deadline, now), do: {:stall_timeout, stall_deadline - now}

  @spec wait_or_expire({EffectError.t(), integer()}) ::
          {:wait, non_neg_integer()} | {:expired, EffectError.t()}
  defp wait_or_expire({reason, remaining}) when remaining <= 0, do: {:expired, reason}
  defp wait_or_expire({_reason, remaining}), do: {:wait, remaining}

  @spec timeout_after_wait(deadlines()) :: {:error, EffectError.t()}
  defp timeout_after_wait(deadlines) do
    case next_receive_timeout(deadlines) do
      {:expired, reason} -> {:error, reason}
      {:wait, _timeout_ms} -> {:error, :turn_timeout}
    end
  end

  @spec decode_rpc_line(String.t()) ::
          {:response, integer(), map()}
          | {:rpc_error, integer(), term()}
          | {:client_request, integer(), String.t(), map()}
          | {:event, map()}
          | :diagnostic
          | :malformed
  defp decode_rpc_line(line) do
    line
    |> Jason.decode()
    |> decode_json_line(line)
  end

  @spec decode_json_line({:ok, term()} | {:error, term()}, String.t()) ::
          {:response, integer(), map()}
          | {:rpc_error, integer(), term()}
          | {:client_request, integer(), String.t(), map()}
          | {:event, map()}
          | :diagnostic
          | :malformed
  defp decode_json_line({:ok, %{"id" => id, "result" => result}}, _line)
       when is_integer(id) and is_map(result) do
    {:response, id, result}
  end

  defp decode_json_line({:ok, %{"id" => id, "error" => error}}, _line)
       when is_integer(id) do
    {:rpc_error, id, error}
  end

  defp decode_json_line({:ok, %{"id" => id, "method" => method} = message}, _line)
       when is_integer(id) and is_binary(method) do
    params = Map.get(message, "params", %{})
    {:client_request, id, method, params}
  end

  defp decode_json_line({:ok, %{"method" => _method}}, line), do: decode_event(line)
  defp decode_json_line({:ok, _other}, _line), do: :diagnostic
  defp decode_json_line({:error, _reason}, line), do: malformed_or_diagnostic(line)

  @spec decode_event(String.t()) :: {:event, map()} | :malformed
  defp decode_event(line) do
    case Protocol.decode_line(line) do
      {:ok, {:event, event}} -> {:event, event}
      {:ok, {:response, _id, _result}} -> :malformed
      {:error, :response_parse_error} -> :malformed
    end
  end

  @spec malformed_or_diagnostic(String.t()) :: :diagnostic | :malformed
  defp malformed_or_diagnostic(line) do
    line
    |> String.trim_leading()
    |> String.starts_with?("{")
    |> malformed_or_diagnostic_from_jsonish()
  end

  @spec malformed_or_diagnostic_from_jsonish(boolean()) :: :diagnostic | :malformed
  defp malformed_or_diagnostic_from_jsonish(true), do: :malformed
  defp malformed_or_diagnostic_from_jsonish(false), do: :diagnostic

  @spec respond_to_client_request(port(), integer(), String.t(), map(), state()) ::
          :ok | {:error, EffectError.t()}
  defp respond_to_client_request(port, id, method, params, state) do
    method
    |> client_request_response(params, state)
    |> send_client_response(port, id)
  end

  @spec client_request_response(String.t(), map(), state()) ::
          {:ok, map()} | {:rpc_error, map()} | {:error, EffectError.t()}
  defp client_request_response("item/commandExecution/requestApproval", _params, state) do
    approval_response(state, "acceptForSession")
  end

  defp client_request_response("item/fileChange/requestApproval", _params, state) do
    approval_response(state, "acceptForSession")
  end

  defp client_request_response("execCommandApproval", _params, state) do
    approval_response(state, "approved_for_session")
  end

  defp client_request_response("applyPatchApproval", _params, state) do
    approval_response(state, "approved_for_session")
  end

  defp client_request_response("item/tool/requestUserInput", params, _state) do
    {:ok, %{"answers" => tool_input_answers(params)}}
  end

  defp client_request_response("item/tool/call", params, state) do
    params
    |> tool_call_input()
    |> tool_call_response(state)
  end

  defp client_request_response(method, _params, _state) do
    {:rpc_error, %{"code" => -32601, "message" => "Unsupported client request: #{method}"}}
  end

  @spec approval_response(state(), String.t()) :: {:ok, map()} | {:error, EffectError.t()}
  defp approval_response(%{auto_approve_requests: true}, decision) do
    {:ok, %{"decision" => decision}}
  end

  defp approval_response(%{auto_approve_requests: false}, _decision) do
    {:error, :turn_input_required}
  end

  @spec tool_input_answers(map()) :: map()
  defp tool_input_answers(%{"questions" => questions}) when is_list(questions) do
    questions
    |> Enum.map(&tool_input_answer/1)
    |> Enum.reject(&is_nil/1)
    |> Map.new()
  end

  defp tool_input_answers(_params), do: %{}

  @spec tool_input_answer(map() | term()) :: {String.t(), map()} | nil
  defp tool_input_answer(%{"id" => id}) when is_binary(id) do
    {id, %{"answers" => [@non_interactive_tool_input_answer]}}
  end

  defp tool_input_answer(_question), do: nil

  @spec tool_dispatch_result(atom(), map(), AdapterSession.t(), state()) ::
          {:ok, map()} | {:error, EffectError.t()}
  defp tool_dispatch_result(tool, args, %AdapterSession{} = session, state) do
    tool
    |> ToolRuntime.execute(args, session, dynamic_tool_opts(state))
    |> dispatch_tool_result(tool)
  end

  @spec dispatch_tool_result({:ok, map()} | {:error, EffectError.t()}, atom()) ::
          {:ok, map()} | {:error, EffectError.t()}
  defp dispatch_tool_result({:ok, execution}, tool) do
    {:ok,
     %{
       call_id: "codex-call-" <> Atom.to_string(tool),
       result: execution
     }}
  end

  defp dispatch_tool_result({:error, _reason} = error, _tool), do: error

  @spec tool_call_input(map()) :: {String.t() | atom(), term()}
  defp tool_call_input(params) do
    tool = Map.get(params, "tool")
    arguments = Map.get(params, "arguments", %{})
    {tool, arguments}
  end

  @spec tool_call_response({String.t() | atom(), term()}, state()) ::
          {:ok, map()} | {:error, EffectError.t()}
  defp tool_call_response({tool, arguments}, %{session: %AdapterSession{} = session} = state) do
    ToolRuntime.dynamic_response(tool, arguments, session, dynamic_tool_opts(state))
  end

  defp tool_call_response({_tool, _arguments}, _state) do
    ToolRuntime.dynamic_response("unknown", %{}, placeholder_session())
  end

  @spec dynamic_tool_opts(state()) :: keyword()
  defp dynamic_tool_opts(state) do
    [
      haft_invoker: Map.get(state, :haft_invoker),
      bash_timeout_ms: Map.get(state, :dynamic_tool_timeout_ms, 60_000)
    ]
  end

  @spec placeholder_session() :: AdapterSession.t()
  defp placeholder_session do
    %AdapterSession{
      session_id: "sess-placeholder",
      config_hash: "0000000000000000000000000000000000000000000000000000000000000000",
      scoped_tools: MapSet.new(),
      workspace_path: File.cwd!(),
      adapter_kind: :codex,
      adapter_version: "placeholder",
      max_turns: 1,
      max_tokens_per_turn: 1,
      wall_clock_timeout_s: 1
    }
  end

  @spec send_client_response(
          {:ok, map()} | {:rpc_error, map()} | {:error, EffectError.t()},
          port(),
          integer()
        ) ::
          :ok | {:error, EffectError.t()}
  defp send_client_response({:ok, result}, port, id) do
    line =
      %{"jsonrpc" => "2.0", "id" => id, "result" => result}
      |> Jason.encode!()
      |> Kernel.<>("\n")

    send_line(port, line)
  end

  defp send_client_response({:rpc_error, error}, port, id) do
    line =
      %{"jsonrpc" => "2.0", "id" => id, "error" => error}
      |> Jason.encode!()
      |> Kernel.<>("\n")

    send_line(port, line)
  end

  defp send_client_response({:error, _reason} = error, _port, _id), do: error

  @spec auto_approve_requests?(term()) :: boolean()
  defp auto_approve_requests?("never"), do: true
  defp auto_approve_requests?("auto_approve_in_session"), do: true
  defp auto_approve_requests?(_approval_policy), do: false

  @spec reply_start_result({:ok, map(), state()} | {:error, EffectError.t()}, state()) ::
          {:reply, {:ok, map()} | {:error, EffectError.t()}, state()}
  defp reply_start_result({:ok, metadata, new_state}, _old_state),
    do: {:reply, {:ok, metadata}, new_state}

  defp reply_start_result({:error, _reason} = error, state), do: {:reply, error, state}

  @spec reply_turn_result({:ok, map(), state()} | {:error, EffectError.t()}, state()) ::
          {:reply, {:ok, map()} | {:error, EffectError.t()}, state()}
  defp reply_turn_result({:ok, reply, new_state}, _old_state),
    do: {:reply, {:ok, reply}, new_state}

  defp reply_turn_result({:error, _reason} = error, state), do: {:reply, error, state}

  @spec send_line(port(), binary()) :: :ok
  defp send_line(port, line) when is_port(port) and is_binary(line) do
    Port.command(port, line)
    :ok
  end

  @spec close_port(state()) :: :ok
  defp close_port(%{port: port}) when is_port(port) do
    case :erlang.port_info(port) do
      :undefined ->
        :ok

      _ ->
        Port.close(port)
        :ok
    end
  rescue
    ArgumentError -> :ok
  end

  defp close_port(_state), do: :ok

  @spec port_metadata(port()) :: map()
  defp port_metadata(port) do
    case :erlang.port_info(port, :os_pid) do
      {:os_pid, os_pid} -> %{codex_app_server_pid: Integer.to_string(os_pid)}
      _ -> %{}
    end
  end

  @spec monotonic_ms() :: integer()
  defp monotonic_ms, do: System.monotonic_time(:millisecond)
end
