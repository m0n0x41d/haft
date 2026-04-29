defmodule OpenSleigh.Gates.HumanTest do
  use ExUnit.Case, async: true

  alias OpenSleigh.Fixtures
  alias OpenSleigh.Gates.Human.CommissionApproved

  @config %{branch_regex: "^(main|master|release/.*)$"}

  describe "CommissionApproved.fires?/3" do
    test "fires on :execute when branch matches main" do
      ticket = Fixtures.ticket(%{target_branch: "main"})
      assert CommissionApproved.fires?(:execute, ticket, @config)
    end

    test "fires on :execute when branch matches release/*" do
      ticket = Fixtures.ticket(%{target_branch: "release/2026.04"})
      assert CommissionApproved.fires?(:execute, ticket, @config)
    end

    test "does NOT fire on non-matching branch" do
      ticket = Fixtures.ticket(%{target_branch: "feature/xyz"})
      refute CommissionApproved.fires?(:execute, ticket, @config)
    end

    test "does NOT fire on :frame or :measure (only :execute is gated pre-Measure)" do
      ticket = Fixtures.ticket(%{target_branch: "main"})
      refute CommissionApproved.fires?(:frame, ticket, @config)
      refute CommissionApproved.fires?(:measure, ticket, @config)
    end

    test "does NOT fire when target_branch is nil" do
      ticket = Fixtures.ticket(%{target_branch: nil})
      refute CommissionApproved.fires?(:execute, ticket, @config)
    end
  end

  test "render_request produces a tracker comment" do
    ticket = Fixtures.ticket(%{id: "tracker-99", target_branch: "main"})
    out = CommissionApproved.render_request(:execute, ticket)
    assert out =~ "HumanGate"
    assert out =~ "tracker-99"
    assert out =~ "main"
    assert out =~ "/approve"
  end

  test "gate_name" do
    assert CommissionApproved.gate_name() == :commission_approved
  end
end
