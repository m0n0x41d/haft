defmodule OpenSleigh.PhaseConfigTest do
  use ExUnit.Case, async: true

  alias OpenSleigh.PhaseConfig

  @valid_frame_attrs %{
    phase: :frame,
    agent_role: :frame_verifier,
    tools: [:haft_query, :read, :grep],
    gates: %{
      structural: [:problem_card_ref_present, :valid_until_field_present],
      semantic: [:object_of_talk_is_specific],
      human: []
    },
    prompt_template_key: :frame,
    max_turns: 1,
    default_valid_until_days: 7
  }

  @valid_execute_attrs %{
    phase: :execute,
    agent_role: :executor,
    tools: [:read, :write, :edit, :bash, :haft_note],
    gates: %{
      structural: [:design_runtime_split_ok],
      semantic: [:lade_quadrants_split_ok],
      human: [:commission_approved]
    },
    prompt_template_key: :execute,
    max_turns: 20,
    default_valid_until_days: 30
  }

  test "new/1 happy path for Frame" do
    assert {:ok, %PhaseConfig{phase: :frame}} = PhaseConfig.new(@valid_frame_attrs)
  end

  test "new/1 happy path for Execute (multi-turn)" do
    assert {:ok, %PhaseConfig{max_turns: 20}} = PhaseConfig.new(@valid_execute_attrs)
  end

  test "CT3 — rejects max_turns < 1" do
    attrs = Map.put(@valid_frame_attrs, :max_turns, 0)
    assert {:error, :invalid_max_turns} = PhaseConfig.new(attrs)
  end

  test "CT4 — Frame must be max_turns == 1" do
    attrs = Map.put(@valid_frame_attrs, :max_turns, 2)

    assert {:error, :single_turn_phase_max_turns_must_be_one} =
             PhaseConfig.new(attrs)
  end

  test "CT4 — Measure must be max_turns == 1" do
    attrs = Map.merge(@valid_frame_attrs, %{phase: :measure, max_turns: 5})

    assert {:error, :single_turn_phase_max_turns_must_be_one} =
             PhaseConfig.new(attrs)
  end

  test "agent_role must be agent-phase role (not :judge or :human)" do
    attrs = Map.put(@valid_frame_attrs, :agent_role, :judge)
    assert {:error, :invalid_agent_role} = PhaseConfig.new(attrs)

    attrs = Map.put(@valid_frame_attrs, :agent_role, :human)
    assert {:error, :invalid_agent_role} = PhaseConfig.new(attrs)
  end

  test "gates map requires three kind keys" do
    attrs = Map.put(@valid_frame_attrs, :gates, %{structural: []})
    assert {:error, :invalid_gates} = PhaseConfig.new(attrs)
  end

  test "declares_human_gate?/1" do
    {:ok, frame} = PhaseConfig.new(@valid_frame_attrs)
    refute PhaseConfig.declares_human_gate?(frame)

    {:ok, execute} = PhaseConfig.new(@valid_execute_attrs)
    assert PhaseConfig.declares_human_gate?(execute)
  end

  test "default_valid_until/2 adds days from reference time" do
    {:ok, pc} = PhaseConfig.new(@valid_frame_attrs)
    ref = ~U[2026-04-22 10:00:00Z]
    vu = PhaseConfig.default_valid_until(pc, ref)
    assert DateTime.diff(vu, ref, :second) == 7 * 86_400
  end
end
