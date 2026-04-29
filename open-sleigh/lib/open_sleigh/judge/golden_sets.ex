defmodule OpenSleigh.Judge.GoldenSets do
  @moduledoc """
  Hand-labelled semantic-gate examples used for calibration and drift checks.

  The examples are small by design for MVP-1: they prove that every
  configured semantic gate has a stable contract fixture before the runtime
  sends live work through a judge provider.
  """

  alias OpenSleigh.{
    ConfigHash,
    Evidence,
    GateContext,
    JudgeClient,
    PhaseConfig,
    SessionScopedArtifactId,
    Ticket
  }

  alias OpenSleigh.Gates.Registry

  @valid_ts ~U[2026-04-22 00:00:00Z]

  @examples [
    %{
      id: "object-specific-file-path",
      gate: :object_of_talk_is_specific,
      expected: :pass,
      input: %{
        upstream_problem_card: %{
          "id" => "pc-object-pass",
          "describedEntity" => "lib/open_sleigh/orchestrator.ex",
          "groundingHolon" => "OpenSleigh.Orchestrator",
          "authoring_source" => "human"
        }
      }
    },
    %{
      id: "object-vague-system",
      gate: :object_of_talk_is_specific,
      expected: :fail,
      input: %{
        upstream_problem_card: %{
          "id" => "pc-object-fail",
          "describedEntity" => "the system",
          "groundingHolon" => "unbounded",
          "authoring_source" => "human"
        }
      }
    },
    %{
      id: "lade-explicit-quadrants",
      gate: :lade_quadrants_split_ok,
      expected: :pass,
      input: %{
        turn_result: %{
          text:
            "Law defines ticket scope. Admissibility checks tests. Deontics says implementer must update code. Work-effect / Evidence records CI."
        }
      }
    },
    %{
      id: "lade-process-narration",
      gate: :lade_quadrants_split_ok,
      expected: :pass,
      input: %{
        turn_result: %{
          text:
            "I’m locating the decision record and commission first, then I’ll extract the bounded implementation scope and verify what code changes and evidence commands are required before making changes."
        }
      }
    },
    %{
      id: "lade-mixed-obligation",
      gate: :lade_quadrants_split_ok,
      expected: :fail,
      input: %{
        turn_result: %{
          text:
            "The system MUST guarantee the feature, and evidence of any run is accepted as proof."
        }
      }
    },
    %{
      id: "evidence-external-ci",
      gate: :no_self_evidence_semantic,
      expected: :pass,
      input: %{
        turn_result: %{
          claim: "CI run verified the branch behavior; the sha remains only a carrier."
        },
        evidence: [%{kind: :ci_run_id, ref: "ci-123", cl: 3, authoring_source: :ci}]
      }
    },
    %{
      id: "evidence-self-authored",
      gate: :no_self_evidence_semantic,
      expected: :fail,
      input: %{
        turn_result: %{claim: "The PR sha proves the runtime effect."},
        evidence: [%{kind: :pr_merge_sha, ref: "sha-self", cl: 2, authoring_source: :agent}]
      }
    }
  ]

  @type example :: %{
          required(:id) => String.t(),
          required(:gate) => atom(),
          required(:expected) => :pass | :fail,
          required(:input) => map()
        }

  @type row :: %{
          required(:id) => String.t(),
          required(:gate) => atom(),
          required(:expected) => :pass | :fail,
          required(:actual) => :pass | :fail | :error,
          required(:status) => :pass | :fail,
          optional(:cl) => 0..3,
          optional(:rationale) => String.t(),
          optional(:reason) => atom()
        }

  @doc "All golden-set examples."
  @spec examples() :: [example()]
  def examples, do: @examples

  @doc "Golden-set examples for one semantic gate."
  @spec examples_for(atom()) :: [example()]
  def examples_for(gate) when is_atom(gate) do
    @examples
    |> Enum.filter(&(&1.gate == gate))
  end

  @doc "Calibration map accepted by `JudgeClient`."
  @spec calibration() :: JudgeClient.calibration()
  def calibration do
    @examples
    |> Enum.map(& &1.gate)
    |> MapSet.new()
    |> Map.new(&{&1, true})
  end

  @doc "Evaluate every golden-set example through the supplied judge invoker."
  @spec evaluate(JudgeClient.invoke_fun()) :: [row()]
  def evaluate(invoke_fun) when is_function(invoke_fun, 1) do
    @examples
    |> Enum.map(&evaluate_example(&1, invoke_fun))
  end

  @doc "Summarise report rows."
  @spec summary([row()]) :: map()
  def summary(rows) when is_list(rows) do
    %{
      total: length(rows),
      passed: Enum.count(rows, &(&1.status == :pass)),
      failed: Enum.count(rows, &(&1.status == :fail))
    }
  end

  @spec evaluate_example(example(), JudgeClient.invoke_fun()) :: row()
  defp evaluate_example(example, invoke_fun) do
    with {:ok, module} <- Registry.semantic_module(example.gate),
         {:ok, ctx} <- context(example),
         {:ok, result} <- JudgeClient.evaluate(module, ctx, invoke_fun, calibration()) do
      result_row(example, result)
    else
      {:error, reason} -> error_row(example, reason)
    end
  end

  @spec result_row(example(), OpenSleigh.Gates.Semantic.result()) :: row()
  defp result_row(example, result) do
    %{
      id: example.id,
      gate: example.gate,
      expected: example.expected,
      actual: result.verdict,
      status: row_status(example.expected, result.verdict),
      cl: result.cl,
      rationale: result.rationale
    }
  end

  @spec error_row(example(), atom()) :: row()
  defp error_row(example, reason) do
    %{
      id: example.id,
      gate: example.gate,
      expected: example.expected,
      actual: :error,
      status: :fail,
      reason: reason
    }
  end

  @spec row_status(:pass | :fail, :pass | :fail) :: :pass | :fail
  defp row_status(expected, expected), do: :pass
  defp row_status(_expected, _actual), do: :fail

  @spec context(example()) :: {:ok, GateContext.t()} | {:error, atom()}
  defp context(%{gate: gate, input: input}) do
    with {:ok, phase_config} <- phase_config(gate),
         {:ok, ticket} <- ticket(),
         {:ok, evidence} <- evidence_list(Map.get(input, :evidence, [])) do
      GateContext.new(%{
        phase: phase_for_gate(gate),
        phase_config: phase_config,
        ticket: ticket,
        self_id: SessionScopedArtifactId.generate(),
        config_hash: ConfigHash.from_iodata("golden-set"),
        turn_result: Map.get(input, :turn_result, %{}),
        evidence: evidence,
        upstream_problem_card: Map.get(input, :upstream_problem_card),
        proposed_valid_until: DateTime.add(@valid_ts, 30 * 86_400, :second)
      })
    end
  end

  @spec phase_config(atom()) :: {:ok, PhaseConfig.t()} | {:error, PhaseConfig.new_error()}
  defp phase_config(:object_of_talk_is_specific) do
    PhaseConfig.new(%{
      phase: :frame,
      agent_role: :frame_verifier,
      tools: [:read],
      gates: %{structural: [], semantic: [:object_of_talk_is_specific], human: []},
      prompt_template_key: :frame,
      max_turns: 1,
      default_valid_until_days: 7
    })
  end

  defp phase_config(:lade_quadrants_split_ok) do
    PhaseConfig.new(%{
      phase: :execute,
      agent_role: :executor,
      tools: [:read, :write],
      gates: %{structural: [], semantic: [:lade_quadrants_split_ok], human: []},
      prompt_template_key: :execute,
      max_turns: 20,
      default_valid_until_days: 30
    })
  end

  defp phase_config(:no_self_evidence_semantic) do
    PhaseConfig.new(%{
      phase: :measure,
      agent_role: :measurer,
      tools: [:haft_decision],
      gates: %{structural: [], semantic: [:no_self_evidence_semantic], human: []},
      prompt_template_key: :measure,
      max_turns: 1,
      default_valid_until_days: 30
    })
  end

  @spec phase_for_gate(atom()) :: atom()
  defp phase_for_gate(:object_of_talk_is_specific), do: :frame
  defp phase_for_gate(:lade_quadrants_split_ok), do: :execute
  defp phase_for_gate(:no_self_evidence_semantic), do: :measure

  @spec ticket() :: {:ok, Ticket.t()} | {:error, Ticket.new_error()}
  defp ticket do
    Ticket.new(%{
      id: "GOLDEN-1",
      source: {:linear, "OCT"},
      title: "Golden set semantic gate fixture",
      body: "",
      state: :in_progress,
      problem_card_ref: "pc-golden",
      target_branch: "feature/golden-set",
      fetched_at: @valid_ts
    })
  end

  @spec evidence_list([map()]) :: {:ok, [Evidence.t()]} | {:error, Evidence.new_error()}
  defp evidence_list(attrs) when is_list(attrs) do
    attrs
    |> Enum.map(&evidence/1)
    |> collect_evidence()
  end

  @spec evidence(map()) :: {:ok, Evidence.t()} | {:error, Evidence.new_error()}
  defp evidence(attrs) do
    Evidence.new(
      Map.fetch!(attrs, :kind),
      Map.fetch!(attrs, :ref),
      Map.get(attrs, :hash),
      Map.fetch!(attrs, :cl),
      Map.fetch!(attrs, :authoring_source),
      @valid_ts
    )
  end

  @spec collect_evidence([{:ok, Evidence.t()} | {:error, Evidence.new_error()}]) ::
          {:ok, [Evidence.t()]} | {:error, Evidence.new_error()}
  defp collect_evidence(results) do
    results
    |> Enum.reduce_while({:ok, []}, &collect_evidence_result/2)
    |> reverse_evidence()
  end

  @spec collect_evidence_result(
          {:ok, Evidence.t()} | {:error, Evidence.new_error()},
          {:ok, [Evidence.t()]}
        ) :: {:cont, {:ok, [Evidence.t()]}} | {:halt, {:error, Evidence.new_error()}}
  defp collect_evidence_result({:ok, evidence}, {:ok, acc}), do: {:cont, {:ok, [evidence | acc]}}
  defp collect_evidence_result({:error, reason}, _acc), do: {:halt, {:error, reason}}

  @spec reverse_evidence({:ok, [Evidence.t()]} | {:error, Evidence.new_error()}) ::
          {:ok, [Evidence.t()]} | {:error, Evidence.new_error()}
  defp reverse_evidence({:ok, evidence}), do: {:ok, Enum.reverse(evidence)}
  defp reverse_evidence({:error, _reason} = error), do: error
end
