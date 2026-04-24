defmodule OpenSleigh.Judge.GoldenSetsTest do
  use ExUnit.Case, async: true

  alias OpenSleigh.Judge.{GoldenSets, RuleBased}

  test "calibration covers every registered semantic gate used in the golden set" do
    calibration = GoldenSets.calibration()

    GoldenSets.examples()
    |> Enum.map(& &1.gate)
    |> Enum.uniq()
    |> Enum.each(fn gate ->
      assert Map.get(calibration, gate) == true
    end)
  end

  test "rule-based baseline passes the full golden set" do
    rows = GoldenSets.evaluate(&RuleBased.invoke/1)
    summary = GoldenSets.summary(rows)

    assert summary.total == 7
    assert summary.failed == 0
    assert Enum.all?(rows, &(&1.status == :pass))
  end

  test "each semantic gate has at least one pass and one fail fixture" do
    GoldenSets.examples()
    |> Enum.group_by(& &1.gate)
    |> Enum.each(fn {_gate, examples} ->
      assert Enum.any?(examples, &(&1.expected == :pass))
      assert Enum.any?(examples, &(&1.expected == :fail))
    end)
  end
end
