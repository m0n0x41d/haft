defmodule OpenSleigh.Sleigh.Watcher do
  @moduledoc """
  L5 watcher that hot-loads `sleigh.md` into `WorkflowStore`.

  The watcher owns the file polling side effect. Compilation remains in
  `OpenSleigh.Sleigh.Compiler`, and the atomic carrier swap remains in
  `OpenSleigh.WorkflowStore`.
  """

  use GenServer

  alias OpenSleigh.{WorkflowStore, Sleigh.Compiler}

  @default_poll_interval_ms 1_000

  @typedoc "Startup options."
  @type opts :: [
          path: Path.t(),
          workflow_store: GenServer.server(),
          poll_interval_ms: non_neg_integer(),
          compiler: module(),
          name: atom()
        ]

  @doc "Start a watcher process."
  @spec start_link(opts()) :: GenServer.on_start()
  def start_link(opts) do
    GenServer.start_link(__MODULE__, opts, name: Keyword.get(opts, :name))
  end

  @doc "Return a small watcher status snapshot."
  @spec status(GenServer.server()) :: map()
  def status(server) do
    GenServer.call(server, :status)
  end

  @impl true
  def init(opts) do
    state = %{
      path: Keyword.fetch!(opts, :path),
      workflow_store: Keyword.fetch!(opts, :workflow_store),
      poll_interval_ms: Keyword.get(opts, :poll_interval_ms, @default_poll_interval_ms),
      compiler: Keyword.get(opts, :compiler, Compiler),
      source_hash: nil,
      last_error: nil,
      loaded_count: 0
    }

    send(self(), :poll)

    {:ok, state}
  end

  @impl true
  def handle_call(:status, _from, state) do
    snapshot = %{
      path: state.path,
      last_error: state.last_error,
      loaded_count: state.loaded_count
    }

    {:reply, snapshot, state}
  end

  @impl true
  def handle_info(:poll, state) do
    new_state =
      state
      |> maybe_reload()
      |> schedule_poll()

    {:noreply, new_state}
  end

  @spec maybe_reload(map()) :: map()
  defp maybe_reload(state) do
    state.path
    |> File.read()
    |> reload_source(state)
  end

  @spec reload_source({:ok, binary()} | {:error, term()}, map()) :: map()
  defp reload_source({:ok, source}, state) do
    source
    |> source_hash()
    |> reload_changed_source(source, state)
  end

  defp reload_source({:error, reason}, state) do
    %{state | last_error: {:file_read_failed, reason}}
  end

  @spec reload_changed_source(integer(), binary(), map()) :: map()
  defp reload_changed_source(source_hash, _source, %{source_hash: source_hash} = state), do: state

  defp reload_changed_source(source_hash, source, state) do
    compiler = state.compiler

    source
    |> compiler.compile()
    |> apply_compile_result(source_hash, state)
  end

  @spec apply_compile_result(
          {:ok, WorkflowStore.bundle()} | {:error, [term()]},
          integer(),
          map()
        ) :: map()
  defp apply_compile_result({:ok, bundle}, source_hash, state) do
    :ok = WorkflowStore.put_compiled(state.workflow_store, bundle)

    %{
      state
      | source_hash: source_hash,
        last_error: nil,
        loaded_count: state.loaded_count + 1
    }
  end

  defp apply_compile_result({:error, errors}, source_hash, state) do
    %{state | source_hash: source_hash, last_error: errors}
  end

  @spec schedule_poll(map()) :: map()
  defp schedule_poll(%{poll_interval_ms: interval_ms} = state) when interval_ms > 0 do
    Process.send_after(self(), :poll, interval_ms)
    state
  end

  defp schedule_poll(state), do: state

  @spec source_hash(binary()) :: integer()
  defp source_hash(source), do: :erlang.phash2(source)
end
