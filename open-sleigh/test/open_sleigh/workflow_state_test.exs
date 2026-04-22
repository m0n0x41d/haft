defmodule OpenSleigh.WorkflowStateTest do
  use ExUnit.Case, async: true

  alias OpenSleigh.{Fixtures, Workflow, WorkflowState}

  test "start/1 begins at the workflow's entry phase" do
    s = WorkflowState.start(Workflow.mvp1())
    assert s.current == :frame
    assert s.completed_outcomes == []
  end

  test "new/3 accepts a valid current phase" do
    assert {:ok, %WorkflowState{current: :execute}} =
             WorkflowState.new(Workflow.mvp1(), :execute, [])
  end

  test "new/3 rejects phase outside workflow alphabet" do
    assert {:error, :current_phase_not_in_workflow} =
             WorkflowState.new(Workflow.mvp1(), :problematize, [])
  end

  test "new/3 rejects invalid atom" do
    assert {:error, :invalid_current_phase} =
             WorkflowState.new(Workflow.mvp1(), :garbage, [])
  end

  test "apply_outcome/2 advances :frame → :execute" do
    state = WorkflowState.start(Workflow.mvp1())

    outcome =
      Fixtures.phase_outcome(%{
        phase: :frame,
        phase_config: Fixtures.phase_config_frame(),
        authoring_role: :frame_verifier
      })

    assert {:ok, %WorkflowState{current: :execute}} =
             WorkflowState.apply_outcome(state, outcome)
  end

  test "apply_outcome/2 rejects phase mismatch (TR1 — no skipping)" do
    state = WorkflowState.start(Workflow.mvp1())
    # state.current = :frame but outcome.phase = :execute.
    outcome = Fixtures.phase_outcome(%{phase: :execute})

    assert {:error, :phase_mismatch} = WorkflowState.apply_outcome(state, outcome)
  end

  test "apply_outcome/2 refuses transitions from terminal state (TR2)" do
    {:ok, terminal_state} = WorkflowState.new(Workflow.mvp1(), :terminal, [])

    outcome =
      Fixtures.phase_outcome(%{
        phase: :terminal,
        verdict: :pass
      })

    assert {:error, :terminal_state} =
             WorkflowState.apply_outcome(terminal_state, outcome)
  end

  test "terminal?/1" do
    refute WorkflowState.terminal?(WorkflowState.start(Workflow.mvp1()))
    {:ok, t} = WorkflowState.new(Workflow.mvp1(), :terminal, [])
    assert WorkflowState.terminal?(t)
  end
end
