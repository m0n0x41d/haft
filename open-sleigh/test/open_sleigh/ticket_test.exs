defmodule OpenSleigh.TicketTest do
  use ExUnit.Case, async: true

  alias OpenSleigh.Ticket

  @valid_attrs %{
    id: "tracker-123",
    source: {:linear, "oct"},
    title: "Add rate limiter",
    body: "body text",
    state: :in_progress,
    problem_card_ref: "haft-pc-abc",
    target_branch: "feature/rate-limiter",
    fetched_at: ~U[2026-04-22 10:00:00Z]
  }

  test "new/1 happy path" do
    assert {:ok, %Ticket{}} = Ticket.new(@valid_attrs)
  end

  test "UP1 — missing problem_card_ref hard-fails" do
    attrs = Map.delete(@valid_attrs, :problem_card_ref)
    assert {:error, :missing_problem_card_ref} = Ticket.new(attrs)
  end

  test "UP1 — nil problem_card_ref hard-fails" do
    attrs = Map.put(@valid_attrs, :problem_card_ref, nil)
    assert {:error, :missing_problem_card_ref} = Ticket.new(attrs)
  end

  test "UP1 — empty string problem_card_ref hard-fails" do
    attrs = Map.put(@valid_attrs, :problem_card_ref, "")
    assert {:error, :missing_problem_card_ref} = Ticket.new(attrs)
  end

  test "rejects invalid source discriminator" do
    attrs = Map.put(@valid_attrs, :source, :linear)
    assert {:error, :invalid_source} = Ticket.new(attrs)
  end

  test "accepts keyword list form" do
    assert {:ok, %Ticket{}} = Ticket.new(Enum.to_list(@valid_attrs))
  end

  test "body may be empty" do
    attrs = Map.put(@valid_attrs, :body, "")
    assert {:ok, _} = Ticket.new(attrs)
  end

  test "target_branch may be nil" do
    attrs = Map.put(@valid_attrs, :target_branch, nil)
    assert {:ok, _} = Ticket.new(attrs)
  end

  test "metadata defaults to empty map" do
    attrs = Map.delete(@valid_attrs, :metadata)
    assert {:ok, %Ticket{metadata: %{}}} = Ticket.new(attrs)
  end
end
