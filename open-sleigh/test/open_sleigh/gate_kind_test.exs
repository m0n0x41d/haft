defmodule OpenSleigh.GateKindTest do
  use ExUnit.Case, async: true

  alias OpenSleigh.GateKind

  test "all/0 returns the three compile-time distinct kinds" do
    assert GateKind.all() == [:structural, :semantic, :human]
  end

  test "valid?/1 accepts only the three kinds" do
    for atom <- GateKind.all(), do: assert(GateKind.valid?(atom))
    refute GateKind.valid?(:runtime)
    refute GateKind.valid?(:llm)
    refute GateKind.valid?("human")
  end
end
