defmodule OpenSleigh.PhaseTest do
  use ExUnit.Case, async: true

  alias OpenSleigh.Phase

  describe "alphabet closure (TR5)" do
    test "all/0 contains MVP-1 and MVP-2 atoms" do
      assert Phase.all() |> length() == 14
      assert :frame in Phase.all()
      assert :execute in Phase.all()
      assert :measure in Phase.all()
      assert :terminal in Phase.all()
      assert :characterize_situation in Phase.all()
      assert :problematize in Phase.all()
      assert :measure_impact in Phase.all()
    end

    test "mvp1/0 is a strict subset of all/0" do
      assert Enum.all?(Phase.mvp1(), &(&1 in Phase.all()))
      assert length(Phase.mvp1()) == 4
      assert length(Phase.all()) > length(Phase.mvp1())
    end

    test "valid?/1 accepts every declared atom" do
      for atom <- Phase.all() do
        assert Phase.valid?(atom), "expected #{inspect(atom)} to be valid"
      end
    end

    test "valid?/1 rejects unknown atoms (TR5 backstop)" do
      refute Phase.valid?(:unknown_phase)
      refute Phase.valid?(:frame_verifier)
      refute Phase.valid?(:Frame)
      refute Phase.valid?(nil)
    end

    test "valid?/1 rejects non-atoms" do
      refute Phase.valid?("frame")
      refute Phase.valid?(%{phase: :frame})
      refute Phase.valid?({:frame})
      refute Phase.valid?(42)
      refute Phase.valid?([:frame])
    end
  end

  describe "mvp1?/1" do
    test "true for MVP-1 phases" do
      for phase <- [:frame, :execute, :measure, :terminal] do
        assert Phase.mvp1?(phase)
      end
    end

    test "false for MVP-2-only phases" do
      for phase <- [:problematize, :generate, :parity_run, :commission] do
        refute Phase.mvp1?(phase)
      end
    end
  end

  describe "terminal?/1 (TR2 absorbing state)" do
    test ":terminal is absorbing" do
      assert Phase.terminal?(:terminal)
    end

    test "no other phase is terminal" do
      for phase <- Phase.all(), phase != :terminal do
        refute Phase.terminal?(phase), "#{inspect(phase)} should not be terminal"
      end
    end
  end

  describe "single_turn?/1 (CT4 — Frame/Measure are single-turn)" do
    test "Frame and Measure are single-turn" do
      assert Phase.single_turn?(:frame)
      assert Phase.single_turn?(:measure)
    end

    test "Execute admits continuation turns" do
      refute Phase.single_turn?(:execute)
    end

    test ":terminal is single-turn trivially" do
      assert Phase.single_turn?(:terminal)
    end

    test "every declared phase returns a boolean" do
      for phase <- Phase.all() do
        assert is_boolean(Phase.single_turn?(phase)),
               "expected single_turn? to return a boolean for #{inspect(phase)}; got non-boolean"
      end
    end
  end
end
