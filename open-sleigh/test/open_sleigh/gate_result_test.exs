defmodule OpenSleigh.GateResultTest do
  use ExUnit.Case, async: true

  alias OpenSleigh.{ConfigHash, GateResult, HumanGateApproval}

  describe "valid?/1 — accepts the closed sum" do
    test "structural pass and fail" do
      assert GateResult.valid?({:structural, :ok})
      assert GateResult.valid?({:structural, {:error, :missing_field}})
    end

    test "semantic pass and fail" do
      assert GateResult.valid?({:semantic, %{verdict: :pass, cl: 3, rationale: "ok"}})
      assert GateResult.valid?({:semantic, %{verdict: :fail, cl: 2, rationale: "vague"}})
      assert GateResult.valid?({:semantic, {:error, :judge_unavailable}})
    end

    test "human approved / rejected / timeout" do
      {:ok, approval} = sample_approval()
      assert GateResult.valid?({:human, approval})
      assert GateResult.valid?({:human, :rejected})
      assert GateResult.valid?({:human, :timeout})
    end
  end

  describe "valid?/1 — rejects malformed values" do
    test "unknown kind tag" do
      refute GateResult.valid?({:runtime, :ok})
      refute GateResult.valid?({:human_judge, :ok})
    end

    test "semantic payload missing fields" do
      refute GateResult.valid?({:semantic, %{verdict: :pass}})
      refute GateResult.valid?({:semantic, %{verdict: :pass, cl: 99, rationale: "x"}})
    end

    test "non-tuple" do
      refute GateResult.valid?(:ok)
      refute GateResult.valid?(%{})
      refute GateResult.valid?(nil)
    end
  end

  describe "kind/1" do
    test "returns the kind tag" do
      assert :structural = GateResult.kind({:structural, :ok})
      assert :semantic = GateResult.kind({:semantic, %{verdict: :pass, cl: 3, rationale: "x"}})
      assert :human = GateResult.kind({:human, :rejected})
    end
  end

  describe "pass?/1 per-kind semantics" do
    test "structural" do
      assert GateResult.pass?({:structural, :ok})
      refute GateResult.pass?({:structural, {:error, :x}})
    end

    test "semantic pass-verdict vs fail" do
      assert GateResult.pass?({:semantic, %{verdict: :pass, cl: 3, rationale: "x"}})
      refute GateResult.pass?({:semantic, %{verdict: :fail, cl: 2, rationale: "x"}})
      refute GateResult.pass?({:semantic, %{verdict: :partial, cl: 1, rationale: "x"}})
      refute GateResult.pass?({:semantic, {:error, :x}})
    end

    test "human" do
      {:ok, approval} = sample_approval()
      assert GateResult.pass?({:human, approval})
      refute GateResult.pass?({:human, :rejected})
      refute GateResult.pass?({:human, :timeout})
    end
  end

  describe "combine/1 — GK6 returns sum, not boolean" do
    test "all-pass → :advance" do
      {:ok, approval} = sample_approval()

      assert {:advance, []} =
               GateResult.combine([
                 {:structural, :ok},
                 {:semantic, %{verdict: :pass, cl: 3, rationale: "x"}},
                 {:human, approval}
               ])
    end

    test "any structural fail → :block" do
      {:block, reasons} = GateResult.combine([{:structural, {:error, :missing_field}}])
      assert [{:structural, :missing_field}] == reasons
    end

    test "any semantic fail → :block with rationale" do
      {:block, reasons} =
        GateResult.combine([
          {:structural, :ok},
          {:semantic, %{verdict: :fail, cl: 2, rationale: "vague describedEntity"}}
        ])

      assert [{:semantic, :fail, "vague describedEntity"}] == reasons
    end

    test "empty list is :advance" do
      assert {:advance, []} = GateResult.combine([])
    end
  end

  # ——— helper ———

  defp sample_approval do
    HumanGateApproval.new(
      "ivan@weareocta.com",
      ~U[2026-04-22 10:00:00Z],
      ConfigHash.from_iodata("sample"),
      :tracker_comment,
      "linear://comment/1",
      nil
    )
  end
end
