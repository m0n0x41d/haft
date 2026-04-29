defmodule OpenSleigh.VerdictTest do
  use ExUnit.Case, async: true

  alias OpenSleigh.Verdict

  test "all/0 is the closed three-atom alphabet" do
    assert Verdict.all() == [:pass, :fail, :partial]
  end

  test "valid?/1 accepts only the three atoms" do
    for atom <- [:pass, :fail, :partial], do: assert(Verdict.valid?(atom))
    refute Verdict.valid?(:success)
    refute Verdict.valid?(:error)
    refute Verdict.valid?("pass")
    refute Verdict.valid?(nil)
  end

  test "pass?/1 distinguishes :pass" do
    assert Verdict.pass?(:pass)
    refute Verdict.pass?(:fail)
    refute Verdict.pass?(:partial)
  end
end
