defmodule OpenSleigh.Tracker.MockTest do
  use ExUnit.Case, async: true

  alias OpenSleigh.Tracker.Mock

  setup do
    {:ok, handle} = Mock.start()

    :ok =
      Mock.seed(handle, [
        %{
          id: "OCT-1",
          source: {:linear, "oct"},
          title: "First",
          body: "",
          state: :in_progress,
          problem_card_ref: "haft-pc-1",
          fetched_at: ~U[2026-04-22 10:00:00Z]
        },
        %{
          id: "OCT-2",
          source: {:linear, "oct"},
          title: "Second",
          body: "",
          state: :done,
          problem_card_ref: "haft-pc-2",
          fetched_at: ~U[2026-04-22 10:00:00Z]
        }
      ])

    %{handle: handle}
  end

  test "list_active/1 returns only in-progress + todo tickets", ctx do
    {:ok, tickets} = Mock.list_active(ctx.handle)
    ids = Enum.map(tickets, & &1.id)
    assert "OCT-1" in ids
    refute "OCT-2" in ids
  end

  test "get/2 fetches a specific ticket by id", ctx do
    assert {:ok, t} = Mock.get(ctx.handle, "OCT-1")
    assert t.id == "OCT-1"
    assert t.state == :in_progress
  end

  test "get/2 unknown id returns error", ctx do
    assert {:error, :tracker_response_malformed} = Mock.get(ctx.handle, "OCT-999")
  end

  test "transition/3 mutates the stored ticket", ctx do
    :ok = Mock.transition(ctx.handle, "OCT-1", :done)
    assert {:ok, t} = Mock.get(ctx.handle, "OCT-1")
    assert t.state == :done

    # And list_active now excludes it.
    {:ok, active} = Mock.list_active(ctx.handle)
    refute Enum.any?(active, &(&1.id == "OCT-1"))
  end

  test "post_comment/3 + comments/2 round-trip", ctx do
    :ok = Mock.post_comment(ctx.handle, "OCT-1", "hello")
    :ok = Mock.post_comment(ctx.handle, "OCT-1", "world")
    assert Mock.comments(ctx.handle, "OCT-1") == ["hello", "world"]
  end

  test "list_comments/2 returns normalized comments oldest-first", ctx do
    :ok = Mock.add_comment(ctx.handle, "OCT-1", "ivan@example.com", "/approve")

    assert {:ok, [comment]} = Mock.list_comments(ctx.handle, "OCT-1")
    assert comment.body == "/approve"
    assert comment.author == "ivan@example.com"
    assert is_binary(comment.id)
  end

  test "adapter_kind/0" do
    assert Mock.adapter_kind() == :mock
  end
end
