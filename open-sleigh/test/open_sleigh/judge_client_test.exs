defmodule OpenSleigh.JudgeClientTest do
  use ExUnit.Case, async: true

  alias OpenSleigh.{Fixtures, Gates, JudgeClient}

  @gate_module Gates.Semantic.ObjectOfTalkIsSpecific

  defp pass_invoker,
    do: fn _prompt ->
      {:ok, %{"verdict" => "pass", "cl" => 3, "rationale" => "concrete file path"}}
    end

  defp fail_invoker,
    do: fn _prompt ->
      {:ok, %{"verdict" => "fail", "cl" => 1, "rationale" => "vague"}}
    end

  defp malformed_invoker,
    do: fn _prompt -> {:ok, %{"what" => "garbage"}} end

  defp calibrated, do: %{object_of_talk_is_specific: true}

  test "evaluate/4 passes through judge output with calibration" do
    ctx = Fixtures.gate_context()

    assert {:ok, %{verdict: :pass, cl: 3}} =
             JudgeClient.evaluate(@gate_module, ctx, pass_invoker(), calibrated())
  end

  test "evaluate/4 preserves fail verdicts (still a pass-shape response)" do
    ctx = Fixtures.gate_context()

    assert {:ok, %{verdict: :fail, cl: 1}} =
             JudgeClient.evaluate(@gate_module, ctx, fail_invoker(), calibrated())
  end

  test "evaluate/4 returns :judge_response_malformed on bad judge output" do
    ctx = Fixtures.gate_context()

    assert {:error, :judge_response_malformed} =
             JudgeClient.evaluate(@gate_module, ctx, malformed_invoker(), calibrated())
  end

  test "GK5 — :uncalibrated when gate missing from calibration map" do
    ctx = Fixtures.gate_context()

    assert {:error, :uncalibrated} =
             JudgeClient.evaluate(@gate_module, ctx, pass_invoker(), %{})
  end

  test "judge_fun/2 returns a function compatible with GateChain" do
    jf = JudgeClient.judge_fun(pass_invoker(), calibrated())
    ctx = Fixtures.gate_context()

    assert {:ok, %{verdict: :pass}} = jf.(@gate_module, ctx)
  end
end
