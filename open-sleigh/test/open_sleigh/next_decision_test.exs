defmodule OpenSleigh.NextDecisionTest do
  use ExUnit.Case, async: true

  alias OpenSleigh.NextDecision

  test "valid? accepts :advance with a valid phase" do
    assert NextDecision.valid?({:advance, :execute})
    assert NextDecision.valid?({:advance, :terminal})
  end

  test "valid? rejects :advance with unknown phase" do
    refute NextDecision.valid?({:advance, :unknown_phase})
  end

  test "valid? accepts :block with a list of reasons" do
    assert NextDecision.valid?({:block, []})
    assert NextDecision.valid?({:block, [{:structural, :missing_field}]})
  end

  test "valid? rejects :block with non-list" do
    refute NextDecision.valid?({:block, :not_a_list})
  end

  test "valid? accepts :terminal with a verdict" do
    assert NextDecision.valid?({:terminal, :pass})
    assert NextDecision.valid?({:terminal, :fail})
    assert NextDecision.valid?({:terminal, :partial})
  end

  test "valid? rejects :terminal with unknown verdict" do
    refute NextDecision.valid?({:terminal, :unknown})
  end

  test "valid? rejects :await_human (not in MVP-1 decision set)" do
    # Await-human is a pre-outcome state owned by L5, not a NextDecision.
    refute NextDecision.valid?({:await_human, :commission_approved})
  end
end
