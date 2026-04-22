defmodule OpenSleigh.HumanGateListener do
  @moduledoc """
  L5 listener for tracker-based HumanGate signals.

  The listener polls comments on tickets parked by `OpenSleigh.Orchestrator`
  and turns authorised `/approve` or `/reject <reason>` comments into
  typed messages back to the orchestrator.
  """

  use GenServer

  alias OpenSleigh.{HumanGateApproval, Orchestrator}

  @default_poll_interval_ms 30_000
  @default_escalate_after_ms 86_400_000
  @default_cancel_after_ms 259_200_000

  @type opts :: [
          tracker_handle: term(),
          tracker_adapter: module(),
          orchestrator: GenServer.server(),
          approvers: [String.t()],
          poll_interval_ms: non_neg_integer(),
          escalate_after_ms: non_neg_integer(),
          cancel_after_ms: non_neg_integer(),
          now_fun: (-> DateTime.t()),
          name: atom()
        ]

  @type signal ::
          {:approve, String.t() | nil}
          | {:reject, String.t()}
          | :none

  @spec start_link(opts()) :: GenServer.on_start()
  def start_link(opts) do
    GenServer.start_link(__MODULE__, opts, name: Keyword.get(opts, :name, __MODULE__))
  end

  @doc "Trigger an immediate comment scan."
  @spec poke(GenServer.server()) :: :ok
  def poke(server) do
    GenServer.cast(server, :poke)
  end

  @impl true
  def init(opts) do
    state = %{
      tracker_handle: Keyword.fetch!(opts, :tracker_handle),
      tracker_adapter: Keyword.fetch!(opts, :tracker_adapter),
      orchestrator: Keyword.fetch!(opts, :orchestrator),
      approvers: opts |> Keyword.get(:approvers, []) |> MapSet.new(),
      poll_interval_ms: Keyword.get(opts, :poll_interval_ms, @default_poll_interval_ms),
      escalate_after_ms: Keyword.get(opts, :escalate_after_ms, @default_escalate_after_ms),
      cancel_after_ms: Keyword.get(opts, :cancel_after_ms, @default_cancel_after_ms),
      now_fun: Keyword.get(opts, :now_fun, &DateTime.utc_now/0),
      seen_comments: MapSet.new(),
      escalated_tickets: MapSet.new()
    }

    state = schedule_poll(state)
    {:ok, state}
  end

  @impl true
  def handle_cast(:poke, state) do
    {:noreply, scan_pending(state)}
  end

  @impl true
  def handle_info(:poll, state) do
    state =
      state
      |> scan_pending()
      |> schedule_poll()

    {:noreply, state}
  end

  @spec scan_pending(map()) :: map()
  defp scan_pending(state) do
    state.orchestrator
    |> Orchestrator.pending_human_gates()
    |> Enum.reduce(state, &scan_pending_gate/2)
  end

  @spec scan_pending_gate(map(), map()) :: map()
  defp scan_pending_gate(pending, state) do
    state
    |> maybe_handle_timeout(pending)
    |> scan_comments(pending)
  end

  @spec scan_comments(map(), map()) :: map()
  defp scan_comments(state, pending) do
    case state.tracker_adapter.list_comments(state.tracker_handle, pending.ticket_id) do
      {:ok, comments} ->
        Enum.reduce(comments, state, &handle_comment(&1, &2, pending))

      {:error, _reason} ->
        state
    end
  end

  @spec handle_comment(OpenSleigh.Tracker.Adapter.normalised_comment(), map(), map()) :: map()
  defp handle_comment(comment, state, pending) do
    cond do
      MapSet.member?(state.seen_comments, comment.id) ->
        state

      true ->
        comment
        |> comment_signal()
        |> handle_signal(comment, mark_seen(state, comment), pending)
    end
  end

  @spec handle_signal(signal(), OpenSleigh.Tracker.Adapter.normalised_comment(), map(), map()) ::
          map()
  defp handle_signal(:none, _comment, state, _pending), do: state

  defp handle_signal({:approve, reason}, comment, state, pending) do
    if authorised?(state, comment.author) do
      approval =
        comment
        |> build_approval(pending, state.now_fun.(), reason)
        |> unwrap_approval()

      GenServer.cast(state.orchestrator, {:human_approval, pending.ticket_id, approval})
      state
    else
      post_unauthorised(state, pending, comment.author)
    end
  end

  defp handle_signal({:reject, reason}, comment, state, pending) do
    if authorised?(state, comment.author) do
      GenServer.cast(state.orchestrator, {:human_rejection, pending.ticket_id, reason})
      state
    else
      post_unauthorised(state, pending, comment.author)
    end
  end

  @spec build_approval(
          OpenSleigh.Tracker.Adapter.normalised_comment(),
          map(),
          DateTime.t(),
          String.t() | nil
        ) :: {:ok, HumanGateApproval.t()} | {:error, HumanGateApproval.new_error()}
  defp build_approval(comment, pending, now, reason) do
    HumanGateApproval.new(
      comment.author,
      now,
      pending.config_hash,
      :tracker_comment,
      signal_ref(comment),
      reason
    )
  end

  @spec unwrap_approval({:ok, HumanGateApproval.t()} | {:error, term()}) :: HumanGateApproval.t()
  defp unwrap_approval({:ok, approval}), do: approval

  @spec comment_signal(OpenSleigh.Tracker.Adapter.normalised_comment()) :: signal()
  defp comment_signal(%{body: body}) when is_binary(body) do
    body
    |> String.trim()
    |> parse_signal()
  end

  @spec parse_signal(String.t()) :: signal()
  defp parse_signal("/approve"), do: {:approve, nil}

  defp parse_signal("/approve " <> reason) do
    {:approve, String.trim(reason)}
  end

  defp parse_signal("/reject"), do: {:reject, "rejected"}

  defp parse_signal("/reject " <> reason) do
    {:reject, String.trim(reason)}
  end

  defp parse_signal(_body), do: :none

  @spec maybe_handle_timeout(map(), map()) :: map()
  defp maybe_handle_timeout(state, pending) do
    elapsed_ms = DateTime.diff(state.now_fun.(), pending.requested_at, :millisecond)

    cond do
      elapsed_ms >= state.cancel_after_ms ->
        cancel_pending(state, pending)

      elapsed_ms >= state.escalate_after_ms ->
        escalate_pending(state, pending)

      true ->
        state
    end
  end

  @spec cancel_pending(map(), map()) :: map()
  defp cancel_pending(state, pending) do
    _ =
      state.tracker_adapter.post_comment(
        state.tracker_handle,
        pending.ticket_id,
        "Open-Sleigh HumanGate timed out; cancelling the parked transition."
      )

    GenServer.cast(state.orchestrator, {:human_timeout, pending.ticket_id})
    state
  end

  @spec escalate_pending(map(), map()) :: map()
  defp escalate_pending(state, pending) do
    if MapSet.member?(state.escalated_tickets, pending.ticket_id) do
      state
    else
      _ =
        state.tracker_adapter.post_comment(
          state.tracker_handle,
          pending.ticket_id,
          "Open-Sleigh HumanGate is still waiting for `/approve` or `/reject <reason>`."
        )

      %{state | escalated_tickets: MapSet.put(state.escalated_tickets, pending.ticket_id)}
    end
  end

  @spec post_unauthorised(map(), map(), String.t()) :: map()
  defp post_unauthorised(state, pending, author) do
    _ =
      state.tracker_adapter.post_comment(
        state.tracker_handle,
        pending.ticket_id,
        "Ignoring HumanGate signal from unauthorised approver `#{author}`."
      )

    state
  end

  @spec authorised?(map(), String.t()) :: boolean()
  defp authorised?(%{approvers: approvers}, author), do: MapSet.member?(approvers, author)

  @spec mark_seen(map(), OpenSleigh.Tracker.Adapter.normalised_comment()) :: map()
  defp mark_seen(state, comment) do
    %{state | seen_comments: MapSet.put(state.seen_comments, comment.id)}
  end

  @spec signal_ref(OpenSleigh.Tracker.Adapter.normalised_comment()) :: String.t()
  defp signal_ref(%{url: url}) when is_binary(url) and url != "", do: url
  defp signal_ref(%{id: id}), do: "tracker://comment/#{id}"

  @spec schedule_poll(map()) :: map()
  defp schedule_poll(%{poll_interval_ms: interval_ms} = state) when interval_ms > 0 do
    ref = Process.send_after(self(), :poll, interval_ms)
    Map.put(state, :poll_ref, ref)
  end

  defp schedule_poll(state), do: state
end
