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

  @type runtime_identity ::
          {:commission, String.t(), legacy_ticket_id :: String.t() | nil}
          | {:ticket, String.t()}
          | :none

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
    metadata = Keyword.get(opts, :metadata, %{})

    state = %{
      path:
        opts
        |> Keyword.fetch!(:path)
        |> runtime_path(metadata, ".jsonl"),
      metadata: metadata
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
    identity =
      state.metadata
      |> Map.merge(data)
      |> runtime_identity()

    %{
      event_id: event_id(),
      event: event,
      at: DateTime.utc_now(),
      metadata: state.metadata,
      data: data
    }
    |> put_identity_fields(identity)
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

  @spec runtime_path(Path.t(), map(), String.t()) :: Path.t()
  defp runtime_path(path, metadata, extension) do
    metadata
    |> runtime_identity()
    |> identity_path(path, extension)
  end

  @spec identity_path(runtime_identity(), Path.t(), String.t()) :: Path.t()
  defp identity_path({:commission, commission_id, _legacy_ticket_id}, path, extension) do
    scoped_path(path, commission_id, extension)
  end

  defp identity_path({:ticket, ticket_id}, path, extension) do
    scoped_path(path, ticket_id, extension)
  end

  defp identity_path(:none, path, _extension), do: path

  @spec scoped_path(Path.t(), String.t(), String.t()) :: Path.t()
  defp scoped_path(path, identity, extension) do
    if Path.extname(path) == "" do
      path
      |> Path.join(safe_identity(identity) <> extension)
    else
      path
    end
  end

  @spec safe_identity(String.t()) :: String.t()
  defp safe_identity(identity) do
    String.replace(identity, ~r/[^A-Za-z0-9._-]/, "_")
  end

  @spec runtime_identity(map()) :: runtime_identity()
  defp runtime_identity(%{} = attrs) do
    attrs
    |> identity_pair()
    |> runtime_identity_result()
  end

  @spec identity_pair(map()) :: {String.t() | nil, String.t() | nil}
  defp identity_pair(attrs) do
    {
      string_value(attrs, :commission_id),
      string_value(attrs, :ticket_id) || string_value(attrs, :ticket)
    }
  end

  @spec runtime_identity_result({String.t() | nil, String.t() | nil}) :: runtime_identity()
  defp runtime_identity_result({commission_id, ticket_id})
       when is_binary(commission_id) and commission_id != "" do
    {:commission, commission_id, ticket_id}
  end

  defp runtime_identity_result({_commission_id, ticket_id})
       when is_binary(ticket_id) and ticket_id != "" do
    {:ticket, ticket_id}
  end

  defp runtime_identity_result(_pair), do: :none

  @spec string_value(map(), atom()) :: String.t() | nil
  defp string_value(attrs, key) do
    attrs
    |> Map.get(key, Map.get(attrs, Atom.to_string(key)))
    |> non_empty_string()
  end

  @spec non_empty_string(term()) :: String.t() | nil
  defp non_empty_string(value) when is_binary(value) and value != "", do: value
  defp non_empty_string(_value), do: nil

  @spec put_identity_fields(map(), runtime_identity()) :: map()
  defp put_identity_fields(entry, {:commission, commission_id, legacy_ticket_id}) do
    entry
    |> Map.put(:commission_id, commission_id)
    |> maybe_put_legacy_ticket_id(legacy_ticket_id, commission_id)
  end

  defp put_identity_fields(entry, {:ticket, ticket_id}) do
    Map.put(entry, :ticket_id, ticket_id)
  end

  defp put_identity_fields(entry, :none), do: entry

  @spec maybe_put_legacy_ticket_id(map(), String.t() | nil, String.t()) :: map()
  defp maybe_put_legacy_ticket_id(entry, legacy_ticket_id, commission_id)
       when is_binary(legacy_ticket_id) and legacy_ticket_id != commission_id do
    Map.put(entry, :legacy_ticket_id, legacy_ticket_id)
  end

  defp maybe_put_legacy_ticket_id(entry, _legacy_ticket_id, _commission_id), do: entry

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
