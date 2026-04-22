defmodule OpenSleigh.GateContextTest do
  use ExUnit.Case, async: true

  alias OpenSleigh.{Fixtures, GateContext}

  test "new/1 happy path" do
    attrs = %{
      phase: :execute,
      phase_config: Fixtures.phase_config_execute(),
      ticket: Fixtures.ticket(),
      self_id: Fixtures.self_id(),
      config_hash: Fixtures.config_hash(),
      turn_result: %{text: "x"},
      evidence: []
    }

    assert {:ok, %GateContext{upstream_problem_card: nil}} = GateContext.new(attrs)
  end

  test "rejects invalid phase" do
    attrs = %{
      phase: :unknown,
      phase_config: Fixtures.phase_config_execute(),
      ticket: Fixtures.ticket(),
      self_id: Fixtures.self_id(),
      config_hash: Fixtures.config_hash(),
      turn_result: %{},
      evidence: []
    }

    assert {:error, :invalid_phase} = GateContext.new(attrs)
  end

  test "rejects non-list evidence" do
    attrs = %{
      phase: :execute,
      phase_config: Fixtures.phase_config_execute(),
      ticket: Fixtures.ticket(),
      self_id: Fixtures.self_id(),
      config_hash: Fixtures.config_hash(),
      turn_result: %{},
      evidence: %{not: :a_list}
    }

    assert {:error, :invalid_evidence} = GateContext.new(attrs)
  end
end
