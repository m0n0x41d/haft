defmodule OpenSleigh.EffectErrorTest do
  use ExUnit.Case, async: true

  alias OpenSleigh.EffectError

  test "all/0 contains the declared MVP-1 failure alphabet" do
    assert length(EffectError.all()) >= 20
    assert :agent_launch_failed in EffectError.all()
    assert :haft_unavailable in EffectError.all()
    assert :tool_forbidden_by_phase_scope in EffectError.all()
    assert :mutation_outside_commission_scope in EffectError.all()
    assert :path_outside_workspace in EffectError.all()
    assert :workspace_is_self in EffectError.all()
    assert :uncalibrated in EffectError.all()
  end

  test "valid?/1 accepts every declared atom" do
    for atom <- EffectError.all(), do: assert(EffectError.valid?(atom))
  end

  test "valid?/1 rejects unknown atoms (AD1 closed-sum)" do
    refute EffectError.valid?(:random_error)
    refute EffectError.valid?(:any_string_error)
    refute EffectError.valid?("agent_launch_failed")
    refute EffectError.valid?(nil)
  end
end
