defmodule OpenSleigh.RuntimeLogWriter do
  @moduledoc """
  Appends structured runtime events to a JSONL file.

  The log is an operator-facing carrier for lifecycle and troubleshooting
  events. It intentionally stores metadata and event summaries only, not Haft
  artifact bodies or secret-bearing adapter payloads.
  """

  use GenServer

  @type opts :: [
          path: Path.t(),
          metadata: map(),
          name: atom()
        ]

  @spec start_link(opts()) :: GenServer.on_start()
  def start_link(opts) do
    case Keyword.fetch(opts, :name) do
      {:ok, name} -> GenServer.start_link(__MODULE__, opts, name: name)
      :error -> GenServer.start_link(__MODULE__, opts)
    end
  end

  @spec event(GenServer.server(), atom() | String.t(), map()) :: :ok
  def event(server, event, data) when is_map(data) do
    GenServer.call(server, {:event, event, data})
  end

  @spec stop(GenServer.server()) :: :ok
  def stop(server) do
    GenServer.call(server, :stop)
  end

  @impl true
  def init(opts) do
    state = %{
      path: Keyword.fetch!(opts, :path),
      metadata: Keyword.get(opts, :metadata, %{})
    }

    :ok = append_event(state, :runtime_started, %{})
    {:ok, state}
  end

  @impl true
  def handle_call({:event, event, data}, _from, state) do
    :ok = append_event(state, event, data)
    {:reply, :ok, state}
  end

  def handle_call(:stop, _from, state) do
    :ok = append_event(state, :runtime_stopping, %{})
    {:stop, :normal, :ok, state}
  end

  @spec append_event(map(), atom() | String.t(), map()) :: :ok
  defp append_event(state, event, data) do
    state
    |> log_entry(event, data)
    |> Jason.encode!()
    |> append_line(state.path)
  end

  @spec log_entry(map(), atom() | String.t(), map()) :: map()
  defp log_entry(state, event, data) do
    %{
      event_id: event_id(),
      event: event,
      at: DateTime.utc_now(),
      metadata: state.metadata,
      data: data
    }
    |> serialise()
  end

  @spec event_id() :: String.t()
  defp event_id do
    suffix =
      [:positive, :monotonic]
      |> System.unique_integer()
      |> Integer.to_string()

    "evt_" <> suffix
  end

  @spec append_line(binary(), Path.t()) :: :ok
  defp append_line(encoded, path) do
    path
    |> Path.dirname()
    |> File.mkdir_p!()

    File.write!(path, encoded <> "\n", [:append])
  end

  @spec serialise(term()) :: term()
  defp serialise(nil), do: nil
  defp serialise(boolean) when is_boolean(boolean), do: boolean
  defp serialise(%DateTime{} = datetime), do: DateTime.to_iso8601(datetime)
  defp serialise(%MapSet{} = set), do: set |> MapSet.to_list() |> serialise()

  defp serialise(%{} = map) do
    map
    |> Enum.map(fn {key, value} -> {serialise_key(key), serialise(value)} end)
    |> Map.new()
  end

  defp serialise(list) when is_list(list), do: Enum.map(list, &serialise/1)
  defp serialise(tuple) when is_tuple(tuple), do: inspect(tuple)
  defp serialise(pid) when is_pid(pid), do: inspect(pid)
  defp serialise(ref) when is_reference(ref), do: inspect(ref)
  defp serialise(atom) when is_atom(atom), do: Atom.to_string(atom)
  defp serialise(value), do: value

  @spec serialise_key(term()) :: String.t()
  defp serialise_key(key) when is_atom(key), do: Atom.to_string(key)
  defp serialise_key(key) when is_binary(key), do: key
  defp serialise_key(key), do: to_string(key)
end
