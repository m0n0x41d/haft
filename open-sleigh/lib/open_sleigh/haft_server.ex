defmodule OpenSleigh.HaftServer do
  @moduledoc """
  L5 GenServer that **owns** the `haft serve` subprocess (Port) or a
  test `invoke_fun`. Implements the invoker contract expected by
  L4 `Haft.Client.call_tool/5`.

  Per v0.6.1 L4/L5 ownership seam:

  * L4 `Haft.Client` / `Haft.Protocol` are stateless.
  * L5 `HaftServer` holds the Port + WAL + retry logic.
  * The `invoke_fun` passed at startup is the only thing that
    differs between test (Mock) and production (real Port wrapping
    `haft serve` stdio).

  State:

      %{
        mode: {:invoke, fun} | {:port, port},
        wal_dir: Path.t() | nil,
        available: boolean(),
        health_misses: non_neg_integer()
      }
  """

  use GenServer

  alias OpenSleigh.{EffectError, Haft.Protocol, Haft.Wal}

  @max_line_bytes 10_000_000
  @initialize_id 1
  @default_command "haft serve"
  @default_read_timeout_ms 10_000
  @default_health_interval_ms 10_000
  @default_health_failure_threshold 3

  # ——— public API ———

  @typedoc "Startup options."
  @type opts :: [
          invoke_fun: (binary() -> {:ok, binary()} | {:error, EffectError.t()}),
          command: String.t(),
          project_root: Path.t(),
          read_timeout_ms: pos_integer(),
          health_interval_ms: pos_integer(),
          health_failure_threshold: pos_integer(),
          wal_dir: Path.t() | nil,
          now_ms_fun: (-> integer()),
          name: atom()
        ]

  @spec start_link(opts()) :: GenServer.on_start()
  def start_link(opts) do
    GenServer.start_link(__MODULE__, opts, name: Keyword.get(opts, :name, __MODULE__))
  end

  @doc """
  Dispatch a JSON-RPC request line to the underlying `haft serve` (or
  mock). The shape matches `Haft.Client`'s `invoke_fun` contract.
  """
  @spec call(binary()) :: {:ok, binary()} | {:error, EffectError.t()}
  def call(request_line) when is_binary(request_line) do
    call(__MODULE__, request_line)
  end

  @spec call(GenServer.server(), binary()) ::
          {:ok, binary()} | {:error, EffectError.t()}
  def call(server, request_line) when is_binary(request_line) do
    GenServer.call(server, {:dispatch, request_line})
  end

  @doc """
  Build an `invoke_fun` bound to this server — pass it directly to
  `OpenSleigh.Haft.Client.call_tool/5` or `write_artifact/3`.
  """
  @spec invoke_fun() :: (binary() -> {:ok, binary()} | {:error, EffectError.t()})
  def invoke_fun do
    invoke_fun(__MODULE__)
  end

  @spec invoke_fun(GenServer.server()) ::
          (binary() -> {:ok, binary()} | {:error, EffectError.t()})
  def invoke_fun(server) do
    fn request_line -> call(server, request_line) end
  end

  @doc "Return a small health/status snapshot."
  @spec status(GenServer.server()) :: map()
  def status(server) do
    GenServer.call(server, :status)
  end

  # ——— GenServer ———

  @impl true
  def init(opts) do
    opts
    |> build_state()
    |> init_result()
  end

  @impl true
  def handle_call(:status, _from, state) do
    snapshot = %{
      available: state.available,
      health_misses: state.health_misses,
      mode: mode_name(state.mode)
    }

    {:reply, snapshot, state}
  end

  @impl true
  def handle_call({:dispatch, request_line}, _from, %{available: true} = state) do
    {reply, new_state} =
      request_line
      |> dispatch_request(state)
      |> maybe_store_unavailable_request(request_line, state)

    {:reply, reply, new_state}
  end

  def handle_call({:dispatch, request_line}, _from, %{available: false} = state) do
    :ok = append_wal(state, request_line)
    {:reply, {:error, :haft_unavailable}, state}
  end

  @impl true
  def handle_info(:health_ping, state) do
    new_state =
      state
      |> run_health_ping()
      |> schedule_health_ping()

    {:noreply, new_state}
  end

  def handle_info({_port, {:exit_status, _status}}, state) do
    {:noreply, %{state | available: false}}
  end

  @impl true
  def terminate(_reason, state) do
    close_port(state)
    :ok
  end

  @spec build_state(opts()) :: {:ok, map()} | {:error, EffectError.t()}
  defp build_state(opts) do
    opts
    |> base_state()
    |> attach_mode(opts)
  end

  @spec init_result({:ok, map()} | {:error, EffectError.t()}) ::
          {:ok, map()} | {:stop, EffectError.t()}
  defp init_result({:ok, state}) do
    state =
      state
      |> initialize_mode()
      |> schedule_health_ping()

    {:ok, state}
  end

  defp init_result({:error, reason}), do: {:stop, reason}

  @spec base_state(opts()) :: map()
  defp base_state(opts) do
    %{
      wal_dir: Keyword.get(opts, :wal_dir) || default_wal_dir(),
      available: true,
      health_misses: 0,
      health_interval_ms: Keyword.get(opts, :health_interval_ms, @default_health_interval_ms),
      health_failure_threshold:
        Keyword.get(opts, :health_failure_threshold, @default_health_failure_threshold),
      read_timeout_ms: Keyword.get(opts, :read_timeout_ms, @default_read_timeout_ms),
      next_id: 2,
      now_ms_fun: Keyword.get(opts, :now_ms_fun, fn -> System.system_time(:millisecond) end)
    }
  end

  @spec default_wal_dir() :: Path.t()
  defp default_wal_dir do
    System.user_home!()
    |> Path.join(".open-sleigh/wal")
    |> Path.expand()
  end

  @spec attach_mode(map(), opts()) :: {:ok, map()} | {:error, EffectError.t()}
  defp attach_mode(state, opts) do
    case Keyword.fetch(opts, :invoke_fun) do
      {:ok, invoke_fun} when is_function(invoke_fun, 1) ->
        {:ok, Map.put(state, :mode, {:invoke, invoke_fun})}

      :error ->
        attach_port_mode(state, opts)
    end
  end

  @spec attach_port_mode(map(), opts()) :: {:ok, map()} | {:error, EffectError.t()}
  defp attach_port_mode(state, opts) do
    command = Keyword.get(opts, :command, @default_command)
    project_root = Keyword.get(opts, :project_root, File.cwd!())

    case open_port(command, project_root) do
      {:ok, port} -> {:ok, Map.put(state, :mode, {:port, port})}
      {:error, _reason} = error -> error
    end
  end

  @spec initialize_mode(map()) :: map()
  defp initialize_mode(%{mode: {:port, _port}} = state) do
    case initialize_port(state) do
      {:ok, new_state} -> new_state
      {:error, _reason} -> %{state | available: false}
    end
  end

  defp initialize_mode(state), do: state

  @spec open_port(String.t(), Path.t()) :: {:ok, port()} | {:error, EffectError.t()}
  defp open_port(command, project_root) when is_binary(command) and is_binary(project_root) do
    case System.find_executable("bash") do
      nil -> {:error, :agent_command_not_found}
      bash -> do_open_port(bash, command, project_root)
    end
  end

  @spec do_open_port(String.t(), String.t(), Path.t()) ::
          {:ok, port()} | {:error, EffectError.t()}
  defp do_open_port(bash, command, project_root) do
    port =
      Port.open(
        {:spawn_executable, String.to_charlist(bash)},
        [
          :binary,
          :exit_status,
          :stderr_to_stdout,
          args: [~c"-lc", String.to_charlist(command)],
          cd: String.to_charlist(project_root),
          line: @max_line_bytes
        ]
      )

    {:ok, port}
  rescue
    _ -> {:error, :agent_launch_failed}
  end

  @spec initialize_port(map()) :: {:ok, map()} | {:error, EffectError.t()}
  defp initialize_port(%{mode: {:port, port}} = state) do
    with :ok <- send_line(port, Protocol.encode_initialize(@initialize_id)),
         {:ok, _line} <- await_response(port, @initialize_id, state.read_timeout_ms, ""),
         :ok <- send_line(port, Protocol.encode_initialized()) do
      {:ok, state}
    end
  end

  @spec dispatch_request(binary(), map()) ::
          {{:ok, binary()} | {:error, EffectError.t()}, map()}
  defp dispatch_request(request_line, %{mode: {:invoke, invoke_fun}} = state) do
    {invoke_fun.(request_line), state}
  end

  defp dispatch_request(request_line, %{mode: {:port, port}} = state) do
    result =
      with {:ok, request_id} <- request_id(request_line),
           :ok <- send_line(port, request_line),
           {:ok, response_line} <- await_response(port, request_id, state.read_timeout_ms, "") do
        {:ok, response_line}
      end

    {result, state}
  end

  @spec maybe_store_unavailable_request(
          {{:ok, binary()} | {:error, EffectError.t()}, map()},
          binary(),
          map()
        ) :: {{:ok, binary()} | {:error, EffectError.t()}, map()}
  defp maybe_store_unavailable_request(
         {{:error, :haft_unavailable} = reply, new_state},
         line,
         state
       ) do
    :ok = append_wal(state, line)
    {reply, %{new_state | available: false}}
  end

  defp maybe_store_unavailable_request(result, _line, _state), do: result

  @spec run_health_ping(map()) :: map()
  defp run_health_ping(state) do
    state
    |> health_ping()
    |> apply_health_result(state)
  end

  @spec health_ping(map()) :: :ok | {:error, EffectError.t()}
  defp health_ping(%{mode: {:invoke, invoke_fun}, next_id: next_id}) do
    request_line = Protocol.encode_health_ping(-next_id)

    case invoke_fun.(request_line) do
      {:ok, _response_line} -> :ok
      {:error, reason} -> {:error, reason}
    end
  end

  defp health_ping(%{mode: {:port, port}, next_id: next_id} = state) do
    request_id = -next_id
    request_line = Protocol.encode_health_ping(request_id)

    with :ok <- send_line(port, request_line),
         {:ok, _response_line} <- await_response(port, request_id, state.read_timeout_ms, "") do
      :ok
    end
  end

  @spec apply_health_result(:ok | {:error, EffectError.t()}, map()) :: map()
  defp apply_health_result(:ok, state) do
    recovered_state =
      state
      |> replay_wal_after_reconnect()

    %{recovered_state | health_misses: 0, next_id: state.next_id + 1}
  end

  defp apply_health_result({:error, _reason}, state) do
    misses = state.health_misses + 1
    available = misses < state.health_failure_threshold
    %{state | available: available, health_misses: misses, next_id: state.next_id + 1}
  end

  @spec replay_wal_after_reconnect(map()) :: map()
  defp replay_wal_after_reconnect(%{available: false, wal_dir: wal_dir} = state)
       when is_binary(wal_dir) do
    wal_dir
    |> Wal.replay(replay_invoker(state))
    |> apply_wal_replay_result(state)
  end

  defp replay_wal_after_reconnect(state), do: %{state | available: true}

  @spec replay_invoker(map()) :: (binary() -> {:ok, binary()} | {:error, EffectError.t()})
  defp replay_invoker(state) do
    fn request_line ->
      request_line
      |> dispatch_request(%{state | available: true})
      |> elem(0)
    end
  end

  @spec apply_wal_replay_result(:ok | {:error, EffectError.t()}, map()) :: map()
  defp apply_wal_replay_result(:ok, state), do: %{state | available: true}
  defp apply_wal_replay_result({:error, _reason}, state), do: %{state | available: false}

  @spec append_wal(map(), binary()) :: :ok
  defp append_wal(%{wal_dir: wal_dir} = state, request_line) when is_binary(wal_dir) do
    _ =
      wal_dir
      |> Wal.append(request_line, state.now_ms_fun.())

    :ok
  end

  defp append_wal(_state, _request_line), do: :ok

  @spec schedule_health_ping(map()) :: map()
  defp schedule_health_ping(%{health_interval_ms: interval_ms} = state) when interval_ms > 0 do
    ref = Process.send_after(self(), :health_ping, interval_ms)
    Map.put(state, :health_timer_ref, ref)
  end

  defp schedule_health_ping(state), do: state

  @spec request_id(binary()) :: {:ok, integer()} | {:error, EffectError.t()}
  defp request_id(request_line) do
    case Jason.decode(request_line) do
      {:ok, %{"id" => id}} when is_integer(id) -> {:ok, id}
      _ -> {:error, :response_parse_error}
    end
  end

  @spec await_response(port(), integer(), pos_integer(), String.t()) ::
          {:ok, binary()} | {:error, EffectError.t()}
  defp await_response(port, request_id, timeout_ms, pending_line) do
    receive do
      {^port, {:data, {:eol, chunk}}} ->
        line = pending_line <> to_string(chunk)
        handle_response_line(line, port, request_id, timeout_ms)

      {^port, {:data, {:noeol, chunk}}} ->
        await_response(port, request_id, timeout_ms, pending_line <> to_string(chunk))

      {^port, {:exit_status, _status}} ->
        {:error, :port_exit_unexpected}
    after
      timeout_ms ->
        {:error, :haft_unavailable}
    end
  end

  @spec handle_response_line(String.t(), port(), integer(), pos_integer()) ::
          {:ok, binary()} | {:error, EffectError.t()}
  defp handle_response_line(line, port, request_id, timeout_ms) do
    line
    |> response_line_id()
    |> match_response_id(line, port, request_id, timeout_ms)
  end

  @spec response_line_id(String.t()) :: {:ok, integer()} | :ignore | {:error, EffectError.t()}
  defp response_line_id(line) do
    case Jason.decode(line) do
      {:ok, %{"id" => id}} when is_integer(id) -> {:ok, id}
      {:ok, _json} -> :ignore
      {:error, _reason} -> :ignore
    end
  end

  @spec match_response_id(
          {:ok, integer()} | :ignore | {:error, EffectError.t()},
          String.t(),
          port(),
          integer(),
          pos_integer()
        ) :: {:ok, binary()} | {:error, EffectError.t()}
  defp match_response_id({:ok, request_id}, line, _port, request_id, _timeout_ms) do
    {:ok, line <> "\n"}
  end

  defp match_response_id(_decoded, _line, port, request_id, timeout_ms) do
    await_response(port, request_id, timeout_ms, "")
  end

  @spec send_line(port(), binary()) :: :ok
  defp send_line(port, line) when is_port(port) and is_binary(line) do
    Port.command(port, line)
    :ok
  end

  @spec close_port(map()) :: :ok
  defp close_port(%{mode: {:port, port}}) when is_port(port) do
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

  @spec mode_name({:invoke, function()} | {:port, port()}) :: atom()
  defp mode_name({:invoke, _fun}), do: :invoke
  defp mode_name({:port, _port}), do: :port
end
