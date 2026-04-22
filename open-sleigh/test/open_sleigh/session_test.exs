defmodule OpenSleigh.SessionTest do
  use ExUnit.Case, async: true

  alias OpenSleigh.{AdapterSession, ConfigHash, Session, SessionId, Ticket}

  setup do
    sid = SessionId.generate()
    ch = ConfigHash.from_iodata("x")

    {:ok, ticket} =
      Ticket.new(%{
        id: "tracker-1",
        source: {:linear, "oct"},
        title: "T",
        body: "",
        state: :in_progress,
        problem_card_ref: "haft-pc-xyz",
        fetched_at: ~U[2026-04-22 10:00:00Z]
      })

    {:ok, adapter_session} =
      AdapterSession.new(%{
        session_id: sid,
        config_hash: ch,
        scoped_tools: MapSet.new([:read, :write]),
        workspace_path: "/tmp/ws/OCT-1",
        adapter_kind: :codex,
        adapter_version: "0.14.0",
        max_turns: 20,
        max_tokens_per_turn: 80_000,
        wall_clock_timeout_s: 600
      })

    %{
      session_id: sid,
      config_hash: ch,
      ticket: ticket,
      adapter_session: adapter_session
    }
  end

  test "new/1 happy path starts in :preparing_workspace", ctx do
    assert {:ok, %Session{sub_state: :preparing_workspace} = s} =
             Session.new(%{
               id: ctx.session_id,
               ticket: ctx.ticket,
               phase: :frame,
               config_hash: ctx.config_hash,
               scoped_tools: MapSet.new([:haft_query]),
               workspace_path: "/tmp/ws/OCT-1",
               claimed_at: ~U[2026-04-22 10:00:00Z],
               adapter_session: ctx.adapter_session
             })

    assert s.turn_count == 0
    assert s.thread_id == nil
    assert s.codex_total_tokens == 0
  end

  test "transition/2 updates sub_state", ctx do
    {:ok, s} = build_session(ctx)

    s2 = Session.transition(s, :streaming_turn)
    assert s2.sub_state == :streaming_turn
    assert s.sub_state == :preparing_workspace
  end

  test "record_turn_completed/2 increments turn_count", ctx do
    {:ok, s} = build_session(ctx)
    s2 = Session.record_turn_completed(s, ~U[2026-04-22 10:05:00Z])
    assert s2.turn_count == 1
    assert s2.last_event_at == ~U[2026-04-22 10:05:00Z]
  end

  test "ingest_token_totals/4 computes deltas correctly (TA2)", ctx do
    {:ok, s} = build_session(ctx)
    s = Session.ingest_token_totals(s, 100, 200, 300)
    assert s.codex_total_tokens == 300
    assert s.last_reported_total_tokens == 300

    # Another update — absolute totals grow, we add only the delta.
    s = Session.ingest_token_totals(s, 150, 250, 400)
    assert s.codex_total_tokens == 400
    assert s.codex_input_tokens == 150
    assert s.codex_output_tokens == 250
  end

  test "set_thread_id/2 is one-shot (phase boundary closes thread)", ctx do
    {:ok, s} = build_session(ctx)
    s = Session.set_thread_id(s, "thread-abc")
    assert s.thread_id == "thread-abc"

    # Second call is a no-op — same session can't swap threads.
    s2 = Session.set_thread_id(s, "thread-xyz")
    assert s2.thread_id == "thread-abc"
  end

  # ——— helper ———

  defp build_session(ctx) do
    Session.new(%{
      id: ctx.session_id,
      ticket: ctx.ticket,
      phase: :execute,
      config_hash: ctx.config_hash,
      scoped_tools: MapSet.new([:read, :write, :bash]),
      workspace_path: "/tmp/ws/OCT-1",
      claimed_at: ~U[2026-04-22 10:00:00Z],
      adapter_session: ctx.adapter_session
    })
  end
end
