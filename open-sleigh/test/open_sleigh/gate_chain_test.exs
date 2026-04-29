defmodule OpenSleigh.GateChainTest do
  use ExUnit.Case, async: true

  alias OpenSleigh.{Fixtures, GateChain}

  # A fake judge function that always returns :pass with cl 3.
  defp pass_judge(_module, _ctx),
    do: {:ok, %{verdict: :pass, cl: 3, rationale: "fake pass"}}

  defp fail_judge(_module, _ctx),
    do: {:ok, %{verdict: :fail, cl: 2, rationale: "fake fail"}}

  defp err_judge(_module, _ctx), do: {:error, :judge_unavailable}

  describe "evaluate_structural/2" do
    test "runs all declared structural gates and returns results" do
      pc =
        Fixtures.phase_config_execute(%{
          gates: %{
            structural: [:design_runtime_split_ok, :valid_until_field_present],
            semantic: [],
            human: []
          }
        })

      ctx = Fixtures.gate_context(%{phase_config: pc})

      assert {:ok, results} = GateChain.evaluate_structural(pc, ctx)
      assert length(results) == 2
      assert Enum.all?(results, fn {kind, _} -> kind == :structural end)
    end

    test "fails on unknown gate name (CF3 backstop)" do
      pc =
        Fixtures.phase_config_execute(%{
          gates: %{structural: [:nonexistent_gate], semantic: [], human: []}
        })

      ctx = Fixtures.gate_context(%{phase_config: pc})

      assert {:error, {:unknown_gate, :nonexistent_gate}} =
               GateChain.evaluate_structural(pc, ctx)
    end

    test "empty structural list returns empty results" do
      pc =
        Fixtures.phase_config_execute(%{
          gates: %{structural: [], semantic: [], human: []}
        })

      ctx = Fixtures.gate_context(%{phase_config: pc})
      assert {:ok, []} = GateChain.evaluate_structural(pc, ctx)
    end
  end

  describe "evaluate_semantic/3" do
    test "runs declared semantic gates via the injected judge_fun" do
      pc =
        Fixtures.phase_config_execute(%{
          gates: %{structural: [], semantic: [:lade_quadrants_split_ok], human: []}
        })

      ctx = Fixtures.gate_context(%{phase_config: pc})

      assert {:ok, [{:semantic, %{verdict: :pass, cl: 3}}]} =
               GateChain.evaluate_semantic(pc, ctx, &pass_judge/2)
    end

    test "propagates judge errors as {:semantic, {:error, reason}}" do
      pc =
        Fixtures.phase_config_execute(%{
          gates: %{structural: [], semantic: [:lade_quadrants_split_ok], human: []}
        })

      ctx = Fixtures.gate_context(%{phase_config: pc})

      assert {:ok, [{:semantic, {:error, :judge_unavailable}}]} =
               GateChain.evaluate_semantic(pc, ctx, &err_judge/2)
    end

    test "fails on unknown semantic gate" do
      pc =
        Fixtures.phase_config_execute(%{
          gates: %{structural: [], semantic: [:unknown_semantic], human: []}
        })

      ctx = Fixtures.gate_context(%{phase_config: pc})

      assert {:error, {:unknown_gate, :unknown_semantic}} =
               GateChain.evaluate_semantic(pc, ctx, &pass_judge/2)
    end
  end

  describe "evaluate/3 — full chain" do
    test "returns structural ++ semantic in order" do
      pc =
        Fixtures.phase_config_execute(%{
          gates: %{
            structural: [:design_runtime_split_ok],
            semantic: [:lade_quadrants_split_ok],
            human: [:commission_approved]
          }
        })

      ctx = Fixtures.gate_context(%{phase_config: pc})

      assert {:ok, [{:structural, :ok}, {:semantic, %{verdict: :pass}}]} =
               GateChain.evaluate(pc, ctx, &pass_judge/2)
    end

    test "human gates are NOT evaluated here (L5 merges them)" do
      pc =
        Fixtures.phase_config_execute(%{
          gates: %{structural: [], semantic: [], human: [:commission_approved]}
        })

      ctx = Fixtures.gate_context(%{phase_config: pc})

      # Only empty results — no :human tuple because chain skips them.
      assert {:ok, []} = GateChain.evaluate(pc, ctx, &pass_judge/2)
    end

    test "semantic fail verdict is still a valid :advance-eligible result at combine step" do
      # GateChain itself just returns results; combining happens at
      # GateResult.combine/1. Here we verify the shape is well-formed.
      pc =
        Fixtures.phase_config_execute(%{
          gates: %{
            structural: [:design_runtime_split_ok],
            semantic: [:lade_quadrants_split_ok],
            human: []
          }
        })

      ctx = Fixtures.gate_context(%{phase_config: pc})

      assert {:ok, results} = GateChain.evaluate(pc, ctx, &fail_judge/2)
      assert [{:structural, :ok}, {:semantic, %{verdict: :fail}}] = results
    end
  end
end
