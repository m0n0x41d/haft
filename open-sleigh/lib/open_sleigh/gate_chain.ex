defmodule OpenSleigh.GateChain do
  @moduledoc """
  Evaluate the full gate chain for a phase — structural gates +
  semantic gates — and assemble a list of `GateResult.t()` ready for
  `PhaseOutcome.new/2`.

  Per `specs/enabling-system/REFERENCE_ALGORITHMS.md §4` worker loop:
  gates are evaluated BEFORE
  `PhaseOutcome.new/2` (the canonical single-constructor shape per
  Q-OS-3 v0.5). Human gates are NOT evaluated here — they are
  triggered, awaited externally by L5 `HumanGateListener`, and
  merged into `gate_results` by L5 when the signal arrives.

  L2 semantic gates define a contract (prompt + response parser); the
  actual LLM call is an L4 `JudgeClient` effect. For that reason,
  `evaluate_structural/2` is pure, while `evaluate_semantic/3` takes
  an injected judge function so the evaluator itself stays testable
  without the L4 runtime.
  """

  alias OpenSleigh.{GateContext, GateResult, PhaseConfig}
  alias OpenSleigh.Gates.{Registry, Semantic}

  @typedoc """
  A judge-invoker function — L4 supplies this. Takes the semantic
  gate module and the `GateContext`, returns the parsed
  `SemanticGateResult` or a typed error.
  """
  @type judge_fun ::
          (module(), GateContext.t() ->
             {:ok, Semantic.result()} | {:error, atom()})

  @doc """
  Run only the structural gates declared in `phase_config`. Returns
  `{:ok, [GateResult.t()]}` on success (including structural-fail
  results, since a `{:structural, {:error, _}}` is still a valid
  gate result) or `{:error, {:unknown_gate, name}}` if any declared
  gate name isn't in the Registry.
  """
  @spec evaluate_structural(PhaseConfig.t(), GateContext.t()) ::
          {:ok, [GateResult.t()]} | {:error, {:unknown_gate, atom()}}
  def evaluate_structural(%PhaseConfig{gates: %{structural: names}}, %GateContext{} = ctx) do
    Enum.reduce_while(names, {:ok, []}, fn name, {:ok, acc} ->
      case Registry.structural_module(name) do
        {:ok, module} ->
          result = apply_structural(module, ctx)
          {:cont, {:ok, acc ++ [result]}}

        {:error, :unknown_gate} ->
          {:halt, {:error, {:unknown_gate, name}}}
      end
    end)
  end

  @doc """
  Run the semantic gates declared in `phase_config`, using the
  injected `judge_fun` to invoke the L4 LLM-judge.
  """
  @spec evaluate_semantic(PhaseConfig.t(), GateContext.t(), judge_fun()) ::
          {:ok, [GateResult.t()]} | {:error, {:unknown_gate, atom()}}
  def evaluate_semantic(%PhaseConfig{gates: %{semantic: names}}, %GateContext{} = ctx, judge_fun) do
    Enum.reduce_while(names, {:ok, []}, fn name, {:ok, acc} ->
      case Registry.semantic_module(name) do
        {:ok, module} ->
          result =
            judge_fun
            |> apply_semantic(module, ctx)
            |> Semantic.to_gate_result()

          {:cont, {:ok, acc ++ [result]}}

        {:error, :unknown_gate} ->
          {:halt, {:error, {:unknown_gate, name}}}
      end
    end)
  end

  @doc """
  Evaluate structural + semantic chains and concat results. Human
  gate results are NOT produced here; L5 merges them separately when
  external signals arrive.
  """
  @spec evaluate(PhaseConfig.t(), GateContext.t(), judge_fun()) ::
          {:ok, [GateResult.t()]} | {:error, {:unknown_gate, atom()}}
  def evaluate(%PhaseConfig{} = pc, %GateContext{} = ctx, judge_fun) do
    with {:ok, structural} <- evaluate_structural(pc, ctx),
         {:ok, semantic} <- evaluate_semantic(pc, ctx, judge_fun) do
      {:ok, structural ++ semantic}
    end
  end

  # ——— helpers ———

  @spec apply_structural(module(), GateContext.t()) :: GateResult.t()
  defp apply_structural(module, ctx) do
    case module.apply(ctx) do
      :ok -> {:structural, :ok}
      {:error, reason} when is_atom(reason) -> {:structural, {:error, reason}}
    end
  end

  @spec apply_semantic(judge_fun(), module(), GateContext.t()) ::
          {:ok, Semantic.result()} | {:error, atom()}
  defp apply_semantic(judge_fun, module, ctx), do: judge_fun.(module, ctx)
end
