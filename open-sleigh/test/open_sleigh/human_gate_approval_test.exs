defmodule OpenSleigh.HumanGateApprovalTest do
  use ExUnit.Case, async: true

  alias OpenSleigh.{ConfigHash, HumanGateApproval}

  setup do
    %{
      approver: "ivan@weareocta.com",
      at: ~U[2026-04-22 10:00:00Z],
      config_hash: ConfigHash.from_iodata("sample"),
      signal_source: :tracker_comment,
      signal_ref: "linear://comment/123",
      reason: "LGTM"
    }
  end

  test "new/6 happy path", ctx do
    assert {:ok, %HumanGateApproval{} = a} =
             HumanGateApproval.new(
               ctx.approver,
               ctx.at,
               ctx.config_hash,
               ctx.signal_source,
               ctx.signal_ref,
               ctx.reason
             )

    assert a.approver == ctx.approver
    assert a.signal_source == :tracker_comment
  end

  test "accepts nil reason", ctx do
    assert {:ok, _} =
             HumanGateApproval.new(
               ctx.approver,
               ctx.at,
               ctx.config_hash,
               ctx.signal_source,
               ctx.signal_ref,
               nil
             )
  end

  test "rejects empty approver", ctx do
    assert {:error, :invalid_approver} =
             HumanGateApproval.new(
               "",
               ctx.at,
               ctx.config_hash,
               ctx.signal_source,
               ctx.signal_ref,
               nil
             )
  end

  test "rejects unsupported signal_source", ctx do
    assert {:error, :invalid_signal_source} =
             HumanGateApproval.new(
               ctx.approver,
               ctx.at,
               ctx.config_hash,
               :email,
               ctx.signal_ref,
               nil
             )
  end

  test "rejects reason > 500 chars", ctx do
    too_long = String.duplicate("x", 501)

    assert {:error, :reason_too_long} =
             HumanGateApproval.new(
               ctx.approver,
               ctx.at,
               ctx.config_hash,
               ctx.signal_source,
               ctx.signal_ref,
               too_long
             )
  end

  test "rejects invalid config_hash", ctx do
    assert {:error, :invalid_config_hash} =
             HumanGateApproval.new(
               ctx.approver,
               ctx.at,
               "not-a-hash",
               ctx.signal_source,
               ctx.signal_ref,
               nil
             )
  end
end
