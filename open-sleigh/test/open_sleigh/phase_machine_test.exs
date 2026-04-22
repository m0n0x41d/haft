defmodule OpenSleigh.PhaseMachineTest do
  use ExUnit.Case, async: true

  alias OpenSleigh.{Fixtures, PhaseMachine, Workflow, WorkflowState}

  describe "next/2 — MVP-1 happy path" do
    test "Frame outcome with passing gates → {:advance, :execute}" do
      state = WorkflowState.start(Workflow.mvp1())

      outcome =
        Fixtures.phase_outcome(%{
          phase: :frame,
          phase_config: Fixtures.phase_config_frame(),
          authoring_role: :frame_verifier,
          gate_results: [{:structural, :ok}]
        })

      assert {:advance, :execute} = PhaseMachine.next(state, outcome)
    end

    test "Execute outcome with passing gates → {:advance, :measure}" do
      {:ok, state} = WorkflowState.new(Workflow.mvp1(), :execute, [])

      outcome =
        Fixtures.phase_outcome(%{
          phase: :execute,
          gate_results: [{:structural, :ok}]
        })

      assert {:advance, :measure} = PhaseMachine.next(state, outcome)
    end

    test "Measure outcome with passing gates → {:terminal, :pass} (next phase is terminal)" do
      {:ok, state} = WorkflowState.new(Workflow.mvp1(), :measure, [])

      outcome =
        Fixtures.phase_outcome(%{
          phase: :measure,
          phase_config: Fixtures.phase_config_measure(),
          authoring_role: :measurer,
          gate_results: [{:structural, :ok}]
        })

      # When the next phase is :terminal, PhaseMachine collapses
      # :advance → :terminal into a direct {:terminal, :pass} decision
      # (TR2 absorbing state + Orchestrator simplicity).
      assert {:terminal, :pass} = PhaseMachine.next(state, outcome)
    end
  end

  describe "next/2 — block path" do
    test "structural gate failure → :block" do
      state = WorkflowState.start(Workflow.mvp1())

      outcome =
        Fixtures.phase_outcome(%{
          phase: :frame,
          phase_config: Fixtures.phase_config_frame(),
          authoring_role: :frame_verifier,
          gate_results: [{:structural, {:error, :no_upstream_frame}}]
        })

      assert {:block, [{:structural, :no_upstream_frame}]} = PhaseMachine.next(state, outcome)
    end

    test "semantic fail verdict → :block" do
      state = WorkflowState.start(Workflow.mvp1())

      outcome =
        Fixtures.phase_outcome(%{
          phase: :frame,
          phase_config: Fixtures.phase_config_frame(),
          authoring_role: :frame_verifier,
          gate_results: [{:semantic, %{verdict: :fail, cl: 2, rationale: "vague"}}]
        })

      assert {:block, [{:semantic, :fail, "vague"}]} = PhaseMachine.next(state, outcome)
    end
  end

  describe "next/2 — terminal absorbing (TR2)" do
    test "terminal-phase outcome → {:terminal, verdict}" do
      {:ok, state} = WorkflowState.new(Workflow.mvp1(), :terminal, [])

      outcome =
        Fixtures.phase_outcome(%{
          phase: :terminal,
          verdict: :pass,
          phase_config: Fixtures.phase_config_execute(),
          gate_results: [{:structural, :ok}]
        })

      assert {:terminal, :pass} = PhaseMachine.next(state, outcome)
    end

    test "terminal defaults to :pass if outcome verdict is nil (phase-less terminal entry)" do
      {:ok, state} = WorkflowState.new(Workflow.mvp1(), :terminal, [])

      outcome =
        Fixtures.phase_outcome(%{
          phase: :terminal,
          verdict: :pass,
          phase_config: Fixtures.phase_config_execute(),
          gate_results: [{:structural, :ok}]
        })

      assert {:terminal, :pass} = PhaseMachine.next(state, outcome)
    end
  end

  describe "advance/2 convenience" do
    test "on :advance, mutates state to the next phase" do
      state = WorkflowState.start(Workflow.mvp1())

      outcome =
        Fixtures.phase_outcome(%{
          phase: :frame,
          phase_config: Fixtures.phase_config_frame(),
          authoring_role: :frame_verifier,
          gate_results: [{:structural, :ok}]
        })

      assert {:ok, %WorkflowState{current: :execute}, {:advance, :execute}} =
               PhaseMachine.advance(state, outcome)
    end

    test "on :block, returns original state + decision" do
      state = WorkflowState.start(Workflow.mvp1())

      outcome =
        Fixtures.phase_outcome(%{
          phase: :frame,
          phase_config: Fixtures.phase_config_frame(),
          authoring_role: :frame_verifier,
          gate_results: [{:structural, {:error, :x}}]
        })

      assert {{:block, _reasons}, ^state} = PhaseMachine.advance(state, outcome)
    end
  end
end
