defmodule OpenSleigh.AuthoringRoleTest do
  use ExUnit.Case, async: true

  alias OpenSleigh.AuthoringRole

  test "all/0 contains the six roles with :frame_verifier (not :framer)" do
    assert AuthoringRole.all() ==
             [:preflight_checker, :frame_verifier, :executor, :measurer, :judge, :human]
  end

  test "valid?/1 rejects legacy :framer and reserved :open_sleigh_self" do
    # :framer was renamed in v0.5; should no longer pass validation.
    refute AuthoringRole.valid?(:framer)
    # :open_sleigh_self is blacklisted per OB4 / UP3.
    refute AuthoringRole.valid?(:open_sleigh_self)
  end

  test "valid?/1 accepts the canonical roles" do
    for role <- AuthoringRole.all(), do: assert(AuthoringRole.valid?(role))
  end

  test "agent_phase_role?/1 separates agent-phase roles from judge/human" do
    assert AuthoringRole.agent_phase_role?(:preflight_checker)
    assert AuthoringRole.agent_phase_role?(:frame_verifier)
    assert AuthoringRole.agent_phase_role?(:executor)
    assert AuthoringRole.agent_phase_role?(:measurer)
    refute AuthoringRole.agent_phase_role?(:judge)
    refute AuthoringRole.agent_phase_role?(:human)
  end
end
