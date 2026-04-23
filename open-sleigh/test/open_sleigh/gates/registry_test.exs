defmodule OpenSleigh.Gates.RegistryTest do
  use ExUnit.Case, async: true

  alias OpenSleigh.Gates.Registry

  describe "structural_module/1" do
    test "resolves every MVP-1 structural gate atom" do
      for name <- Registry.structural_gates() do
        assert {:ok, module} = Registry.structural_module(name)
        assert is_atom(module)
        assert module.gate_name() == name
      end
    end

    test "rejects unknown atom" do
      assert {:error, :unknown_gate} = Registry.structural_module(:not_a_gate)
    end
  end

  describe "semantic_module/1" do
    test "resolves every MVP-1 semantic gate atom" do
      for name <- Registry.semantic_gates() do
        assert {:ok, module} = Registry.semantic_module(name)
        assert module.gate_name() == name
      end
    end

    test "rejects unknown atom" do
      assert {:error, :unknown_gate} = Registry.semantic_module(:not_a_gate)
    end
  end

  describe "human_module/1" do
    test "resolves commission_approved" do
      assert {:ok, module} = Registry.human_module(:commission_approved)
      assert module.gate_name() == :commission_approved
    end

    test "rejects unknown atom" do
      assert {:error, :unknown_gate} = Registry.human_module(:not_a_gate)
    end
  end

  describe "known?/1" do
    test "true for structural / semantic / human gates" do
      assert Registry.known?(:problem_card_ref_present)
      assert Registry.known?(:object_of_talk_is_specific)
      assert Registry.known?(:commission_approved)
    end

    test "false for unknown atoms" do
      refute Registry.known?(:unknown_gate)
      refute Registry.known?(nil)
    end
  end

  test "gate counts match MVP-1 spec" do
    # Per `specs/target-system/GATES.md §1/§2/§4`: 8 currently implemented
    # structural gates, 3 semantic gates, 1 human gate.
    assert length(Registry.structural_gates()) == 8
    assert length(Registry.semantic_gates()) == 3
    assert length(Registry.human_gates()) == 1
  end
end
