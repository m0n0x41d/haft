defmodule OpenSleigh.HumanGateListenerTest do
  use ExUnit.Case, async: false

  alias OpenSleigh.{ConfigHash, HumanGateApproval, HumanGateListener}
  alias OpenSleigh.Tracker.Mock, as: TrackerMock

  defmodule ProbeOrchestrator do
    use GenServer

    @spec start_link(pid(), [map()]) :: GenServer.on_start()
    def start_link(parent, pending) do
      GenServer.start_link(__MODULE__, %{parent: parent, pending: pending})
    end

    @impl true
    def init(state), do: {:ok, state}

    @impl true
    def handle_call(:pending_human_gates, _from, state) do
      {:reply, state.pending, state}
    end

    @impl true
    def handle_cast(message, state) do
      send(state.parent, message)
      {:noreply, state}
    end
  end

  setup do
    {:ok, tracker} = TrackerMock.start()
    now = ~U[2026-04-22 10:00:00Z]
    config_hash = ConfigHash.from_iodata("human-gate-test")

    pending = [
      %{
        ticket_id: "OCT-1",
        session_id: "session-1",
        gate_name: :commission_approved,
        config_hash: config_hash,
        requested_at: now
      }
    ]

    {:ok, orchestrator} = ProbeOrchestrator.start_link(self(), pending)

    %{
      tracker: tracker,
      orchestrator: orchestrator,
      now: now,
      config_hash: config_hash
    }
  end

  test "authorized /approve builds HumanGateApproval and sends it to orchestrator", ctx do
    :ok = TrackerMock.add_comment(ctx.tracker, "OCT-1", "ivan@example.com", "/approve LGTM")
    {:ok, listener} = start_listener(ctx, approvers: ["ivan@example.com"])

    HumanGateListener.poke(listener)

    assert_receive {:human_approval, "OCT-1", %HumanGateApproval{} = approval}
    assert approval.approver == "ivan@example.com"
    assert approval.config_hash == ctx.config_hash
    assert approval.reason == "LGTM"
  end

  test "unauthorized approval is rejected with a tracker comment", ctx do
    :ok = TrackerMock.add_comment(ctx.tracker, "OCT-1", "mallory@example.com", "/approve")
    {:ok, listener} = start_listener(ctx, approvers: ["ivan@example.com"])

    HumanGateListener.poke(listener)
    Process.sleep(20)

    comments = TrackerMock.comments(ctx.tracker, "OCT-1")
    assert Enum.any?(comments, &String.contains?(&1, "unauthorised approver"))
    refute_received {:human_approval, "OCT-1", _approval}
  end

  test "authorized /reject sends rejection reason to orchestrator", ctx do
    :ok = TrackerMock.add_comment(ctx.tracker, "OCT-1", "ivan@example.com", "/reject not ready")
    {:ok, listener} = start_listener(ctx, approvers: ["ivan@example.com"])

    HumanGateListener.poke(listener)

    assert_receive {:human_rejection, "OCT-1", "not ready"}
  end

  test "timeout escalation posts a reminder once", ctx do
    {:ok, listener} =
      start_listener(ctx,
        approvers: ["ivan@example.com"],
        escalate_after_ms: 1,
        cancel_after_ms: 10_000
      )

    HumanGateListener.poke(listener)
    HumanGateListener.poke(listener)
    Process.sleep(20)

    comments = TrackerMock.comments(ctx.tracker, "OCT-1")
    reminders = Enum.filter(comments, &String.contains?(&1, "still waiting"))
    assert length(reminders) == 1
  end

  test "cancel timeout notifies orchestrator", ctx do
    {:ok, listener} =
      start_listener(ctx,
        approvers: ["ivan@example.com"],
        escalate_after_ms: 1,
        cancel_after_ms: 1
      )

    HumanGateListener.poke(listener)

    assert_receive {:human_timeout, "OCT-1"}
  end

  defp start_listener(ctx, opts) do
    opts =
      [
        tracker_handle: ctx.tracker,
        tracker_adapter: TrackerMock,
        orchestrator: ctx.orchestrator,
        poll_interval_ms: 0,
        now_fun: fn -> DateTime.add(ctx.now, 2, :second) end
      ]
      |> Keyword.merge(opts)

    HumanGateListener.start_link(opts)
  end
end
