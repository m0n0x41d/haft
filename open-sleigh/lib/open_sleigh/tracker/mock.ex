defmodule OpenSleigh.Tracker.Mock do
  @moduledoc """
  In-memory `Tracker.Adapter` for tests. Uses an Agent process (note:
  `Agent` is a standard-library L5 primitive; this module lives at
  L4 because it implements the L4 `Tracker.Adapter` behaviour, but
  its storage detail is an Agent to keep tests simple).

  Test setup:

      {:ok, handle} = OpenSleigh.Tracker.Mock.start()
      :ok = OpenSleigh.Tracker.Mock.seed(handle, [
        %{id: "OCT-1", ...}
      ])

  The mock returns `Ticket.t()` values identical to what a real
  adapter would — so upstream layers see no difference.
  """

  @behaviour OpenSleigh.Tracker.Adapter

  alias OpenSleigh.{EffectError, Ticket}

  @impl true
  @spec adapter_kind() :: atom()
  def adapter_kind, do: :mock

  @doc "Start a fresh in-memory store. Returns a handle (pid)."
  @spec start() :: {:ok, pid()}
  def start do
    Agent.start_link(fn -> %{tickets: %{}, comments: %{}} end)
  end

  @doc "Seed the store with a list of ticket attr-maps."
  @spec seed(pid(), [map()]) :: :ok | {:error, atom()}
  def seed(handle, attrs_list) when is_list(attrs_list) do
    Enum.reduce_while(attrs_list, :ok, &seed_one(handle, &1, &2))
  end

  @spec seed_one(pid(), map(), :ok) ::
          {:cont, :ok} | {:halt, {:error, atom()}}
  defp seed_one(handle, attrs, _acc) do
    with {:ok, ticket} <- Ticket.new(attrs) do
      :ok = Agent.update(handle, &put_in(&1, [:tickets, ticket.id], ticket))
      {:cont, :ok}
    else
      {:error, _reason} = err -> {:halt, err}
    end
  end

  @impl true
  @spec list_active(pid()) :: {:ok, [Ticket.t()]} | {:error, EffectError.t()}
  def list_active(handle) when is_pid(handle) do
    tickets =
      handle
      |> Agent.get(& &1)
      |> Map.fetch!(:tickets)
      |> Map.values()
      |> Enum.filter(&(&1.state == :in_progress or &1.state == :todo))

    {:ok, tickets}
  end

  @impl true
  @spec get(pid(), String.t()) :: {:ok, Ticket.t()} | {:error, EffectError.t()}
  def get(handle, ticket_id) when is_pid(handle) and is_binary(ticket_id) do
    case Agent.get(handle, &get_in(&1, [:tickets, ticket_id])) do
      %Ticket{} = t -> {:ok, t}
      nil -> {:error, :tracker_response_malformed}
    end
  end

  @impl true
  @spec transition(pid(), String.t(), atom()) :: :ok | {:error, EffectError.t()}
  def transition(handle, ticket_id, new_state)
      when is_pid(handle) and is_binary(ticket_id) and is_atom(new_state) do
    Agent.get_and_update(handle, &do_transition(&1, ticket_id, new_state))
  end

  @spec do_transition(map(), String.t(), atom()) ::
          {:ok, map()} | {{:error, EffectError.t()}, map()}
  defp do_transition(state, ticket_id, new_state) do
    case get_in(state, [:tickets, ticket_id]) do
      %Ticket{} = t ->
        updated = %{t | state: new_state}
        {:ok, put_in(state, [:tickets, ticket_id], updated)}

      nil ->
        {{:error, :tracker_response_malformed}, state}
    end
  end

  @impl true
  @spec post_comment(pid(), String.t(), String.t()) :: :ok | {:error, EffectError.t()}
  def post_comment(handle, ticket_id, body)
      when is_pid(handle) and is_binary(ticket_id) and is_binary(body) do
    comment = %{
      id: "mock-comment-#{System.unique_integer([:positive, :monotonic])}",
      body: body,
      author: "open-sleigh",
      created_at: DateTime.utc_now(),
      url: "mock://comment/#{ticket_id}"
    }

    put_comment(handle, ticket_id, comment)
  end

  @impl true
  @spec list_comments(pid(), String.t()) ::
          {:ok, [OpenSleigh.Tracker.Adapter.normalised_comment()]} | {:error, EffectError.t()}
  def list_comments(handle, ticket_id) when is_pid(handle) and is_binary(ticket_id) do
    comments =
      handle
      |> Agent.get(&Map.get(&1.comments, ticket_id, []))
      |> Enum.reverse()

    {:ok, comments}
  end

  @doc "Test helper — seed a tracker-origin comment with author metadata."
  @spec add_comment(pid(), String.t(), String.t(), String.t()) :: :ok | {:error, EffectError.t()}
  def add_comment(handle, ticket_id, author, body)
      when is_pid(handle) and is_binary(ticket_id) and is_binary(author) and is_binary(body) do
    comment = %{
      id: "mock-comment-#{System.unique_integer([:positive, :monotonic])}",
      body: body,
      author: author,
      created_at: DateTime.utc_now(),
      url: "mock://comment/#{ticket_id}"
    }

    put_comment(handle, ticket_id, comment)
  end

  @spec put_comment(pid(), String.t(), OpenSleigh.Tracker.Adapter.normalised_comment()) ::
          :ok | {:error, EffectError.t()}
  defp put_comment(handle, ticket_id, comment) do
    Agent.update(handle, fn state ->
      comments = Map.get(state.comments, ticket_id, [])
      put_in(state, [:comments, ticket_id], [comment | comments])
    end)
  end

  @doc "Test helper — read back all posted comments for a ticket."
  @spec comments(pid(), String.t()) :: [String.t()]
  def comments(handle, ticket_id) when is_pid(handle) do
    Agent.get(handle, &Map.get(&1.comments, ticket_id, []))
    |> Enum.map(& &1.body)
    |> Enum.reverse()
  end
end
