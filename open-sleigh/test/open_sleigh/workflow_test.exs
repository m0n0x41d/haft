defmodule OpenSleigh.WorkflowTest do
  use ExUnit.Case, async: true

  alias OpenSleigh.Workflow

  describe "MVP-1 workflow" do
    test "id and entry are frame" do
      wf = Workflow.mvp1()
      assert wf.id == :mvp1
      assert wf.entry_phase == :frame
    end

    test "linear advance: frame → execute → measure → terminal" do
      wf = Workflow.mvp1()
      assert :execute = Workflow.advance_from(wf, :frame)
      assert :measure = Workflow.advance_from(wf, :execute)
      assert :terminal = Workflow.advance_from(wf, :measure)
      assert nil == Workflow.advance_from(wf, :terminal)
    end

    test "MVP-2 phases are not in MVP-1 alphabet" do
      wf = Workflow.mvp1()
      refute Workflow.contains_phase?(wf, :problematize)
      refute Workflow.contains_phase?(wf, :parity_run)
    end

    test "advance_from raises on phase outside MVP-1 alphabet" do
      wf = Workflow.mvp1()

      assert_raise ArgumentError, fn ->
        Workflow.advance_from(wf, :problematize)
      end
    end

    test "terminal?/2" do
      wf = Workflow.mvp1()
      assert Workflow.terminal?(wf, :terminal)
      refute Workflow.terminal?(wf, :frame)
    end
  end

  describe "MVP-1R workflow" do
    test "id and entry are commission preflight" do
      wf = Workflow.mvp1r()
      assert wf.id == :mvp1r
      assert wf.entry_phase == :preflight
    end

    test "linear advance: preflight → frame → execute → measure → terminal" do
      wf = Workflow.mvp1r()
      assert :frame = Workflow.advance_from(wf, :preflight)
      assert :execute = Workflow.advance_from(wf, :frame)
      assert :measure = Workflow.advance_from(wf, :execute)
      assert :terminal = Workflow.advance_from(wf, :measure)
      assert nil == Workflow.advance_from(wf, :terminal)
    end
  end

  describe "MVP-2 workflow" do
    test "id and entry are characterize_situation" do
      wf = Workflow.mvp2()
      assert wf.id == :mvp2
      assert wf.entry_phase == :characterize_situation
    end

    test "full lemniscate advance chain (Slide 12)" do
      wf = Workflow.mvp2()

      assert :measure_situation = Workflow.advance_from(wf, :characterize_situation)
      assert :problematize = Workflow.advance_from(wf, :measure_situation)
      assert :select_spec = Workflow.advance_from(wf, :problematize)
      assert :accept_spec = Workflow.advance_from(wf, :select_spec)
      assert :generate = Workflow.advance_from(wf, :accept_spec)
      assert :parity_run = Workflow.advance_from(wf, :generate)
      assert :select = Workflow.advance_from(wf, :parity_run)
      assert :commission = Workflow.advance_from(wf, :select)
      assert :measure_impact = Workflow.advance_from(wf, :commission)
      assert :terminal = Workflow.advance_from(wf, :measure_impact)
    end

    test "MVP-1 phases are NOT in MVP-2 alphabet (MVP-2 skips Frame/Execute/Measure)" do
      # MVP-2 is a different lifecycle graph — Frame/Execute/Measure
      # were MVP-1 compressions that map to expanded MVP-2 phases.
      wf = Workflow.mvp2()
      refute Workflow.contains_phase?(wf, :frame)
      refute Workflow.contains_phase?(wf, :execute)
      refute Workflow.contains_phase?(wf, :measure)
    end
  end
end
