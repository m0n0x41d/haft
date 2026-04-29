defmodule OpenSleigh.PhaseOutcomeTest do
  use ExUnit.Case, async: true
  use ExUnitProperties

  alias OpenSleigh.{
    ConfigHash,
    Evidence,
    HumanGateApproval,
    PhaseConfig,
    PhaseOutcome,
    SessionId,
    SessionScopedArtifactId
  }

  # ——— shared fixtures ———

  defp base_attrs(overrides \\ %{}) do
    {:ok, evidence} =
      Evidence.new(:pr_merge_sha, "sha-abc", nil, 3, :git_host, ~U[2026-04-22 10:05:00Z])

    {:ok, phase_config} =
      PhaseConfig.new(%{
        phase: :execute,
        agent_role: :executor,
        tools: [:read, :write],
        gates: %{structural: [:design_runtime_split_ok], semantic: [], human: []},
        prompt_template_key: :execute,
        max_turns: 20,
        default_valid_until_days: 30
      })

    defaults = %{
      session_id: SessionId.generate(),
      phase: :execute,
      work_product: %{pr_url: "https://github.com/x/pull/1"},
      evidence: [evidence],
      gate_results: [{:structural, :ok}],
      config_hash: ConfigHash.from_iodata("test-config"),
      valid_until: ~U[2026-05-22 10:00:00Z],
      authoring_role: :executor,
      self_id: SessionScopedArtifactId.generate(),
      produced_at: ~U[2026-04-22 10:10:00Z],
      phase_config: phase_config
    }

    Map.merge(defaults, overrides)
  end

  defp sample_approval(config_hash) do
    {:ok, a} =
      HumanGateApproval.new(
        "ivan@weareocta.com",
        ~U[2026-04-22 10:08:00Z],
        config_hash,
        :tracker_comment,
        "linear://comment/1",
        nil
      )

    a
  end

  # ——— happy path ———

  describe "new/1 happy path" do
    test "constructs a valid PhaseOutcome with required fields" do
      assert {:ok, %PhaseOutcome{phase: :execute}} = PhaseOutcome.new(base_attrs())
    end

    test "accepts keyword list form" do
      assert {:ok, %PhaseOutcome{}} = PhaseOutcome.new(Enum.to_list(base_attrs()))
    end

    test "verdict on :terminal phase is accepted" do
      {:ok, pc} =
        PhaseConfig.new(%{
          phase: :execute,
          agent_role: :executor,
          tools: [],
          gates: %{structural: [], semantic: [], human: []},
          prompt_template_key: :execute,
          max_turns: 20,
          default_valid_until_days: 30
        })

      attrs = base_attrs(%{phase: :terminal, verdict: :pass, phase_config: pc})
      assert {:ok, %PhaseOutcome{verdict: :pass}} = PhaseOutcome.new(attrs)
    end
  end

  # ——— PR1–PR3 provenance required ———

  describe "PR1–PR3 — required provenance fields" do
    test "missing config_hash fails" do
      attrs = base_attrs() |> Map.delete(:config_hash)
      assert {:error, :invalid_config_hash} = PhaseOutcome.new(attrs)
    end

    test "missing valid_until fails" do
      attrs = base_attrs() |> Map.delete(:valid_until)
      assert {:error, :invalid_valid_until} = PhaseOutcome.new(attrs)
    end

    test "missing authoring_role fails" do
      attrs = base_attrs() |> Map.delete(:authoring_role)
      assert {:error, :invalid_authoring_role} = PhaseOutcome.new(attrs)
    end

    test "missing self_id fails" do
      attrs = base_attrs() |> Map.delete(:self_id)
      assert {:error, :invalid_self_id} = PhaseOutcome.new(attrs)
    end
  end

  # ——— PR4 rationale bound ———

  describe "PR4 — rationale ≤ 1000 chars" do
    test "accepts nil rationale" do
      assert {:ok, %PhaseOutcome{rationale: nil}} =
               PhaseOutcome.new(base_attrs(%{rationale: nil}))
    end

    test "accepts 1000-char rationale" do
      assert {:ok, _} =
               PhaseOutcome.new(base_attrs(%{rationale: String.duplicate("x", 1000)}))
    end

    test "rejects 1001-char rationale" do
      assert {:error, :rationale_too_long} =
               PhaseOutcome.new(base_attrs(%{rationale: String.duplicate("x", 1001)}))
    end
  end

  # ——— PR5 — the CRITICAL-2 v0.5 fix ———

  describe "PR5 — evidence self-reference (v0.5 CRITICAL-2 fix)" do
    test "rejects PhaseOutcome where any evidence.ref == self_id" do
      self_id = SessionScopedArtifactId.generate()
      self_id_str = SessionScopedArtifactId.to_string(self_id)

      {:ok, self_refing_evidence} =
        Evidence.new(:test, self_id_str, nil, 2, :ci, ~U[2026-04-22 10:05:00Z])

      attrs = base_attrs(%{self_id: self_id, evidence: [self_refing_evidence]})
      assert {:error, :evidence_self_reference} = PhaseOutcome.new(attrs)
    end

    test "accepts evidence when refs are distinct from self_id" do
      self_id = SessionScopedArtifactId.generate()

      {:ok, external_ev} =
        Evidence.new(:pr_merge_sha, "git-sha-12345", nil, 3, :git_host, ~U[2026-04-22 10:05:00Z])

      attrs = base_attrs(%{self_id: self_id, evidence: [external_ev]})
      assert {:ok, _} = PhaseOutcome.new(attrs)
    end

    property "fuzz: no self-refing evidence ever slips through" do
      check all(payload_bytes <- StreamData.binary(min_length: 1, max_length: 64)) do
        self_id = SessionScopedArtifactId.generate()
        self_id_str = SessionScopedArtifactId.to_string(self_id)

        {:ok, self_refing} =
          Evidence.new(:test, self_id_str, nil, 1, :ci, ~U[2026-04-22 10:05:00Z])

        {:ok, other} =
          Evidence.new(:test, Base.encode16(payload_bytes), nil, 1, :ci, ~U[2026-04-22 10:05:00Z])

        # Any evidence list containing the self-refing item must fail.
        attrs = base_attrs(%{self_id: self_id, evidence: [other, self_refing]})
        assert {:error, :evidence_self_reference} = PhaseOutcome.new(attrs)
      end
    end
  end

  # ——— PR10 — gate-config consistency (v0.5 Q-OS-3) ———

  describe "PR10 — gate-config consistency" do
    test "phase_config declares human gate AND gate_results has approved :human → accepted" do
      ch = ConfigHash.from_iodata("gc-test")

      {:ok, pc} =
        PhaseConfig.new(%{
          phase: :execute,
          agent_role: :executor,
          tools: [],
          gates: %{structural: [], semantic: [], human: [:commission_approved]},
          prompt_template_key: :execute,
          max_turns: 5,
          default_valid_until_days: 30
        })

      approval = sample_approval(ch)

      attrs =
        base_attrs(%{
          config_hash: ch,
          phase_config: pc,
          gate_results: [{:structural, :ok}, {:human, approval}]
        })

      assert {:ok, _} = PhaseOutcome.new(attrs)
    end

    test "phase_config declares human gate BUT gate_results has no :human → fail" do
      {:ok, pc} =
        PhaseConfig.new(%{
          phase: :execute,
          agent_role: :executor,
          tools: [],
          gates: %{structural: [], semantic: [], human: [:commission_approved]},
          prompt_template_key: :execute,
          max_turns: 5,
          default_valid_until_days: 30
        })

      attrs = base_attrs(%{phase_config: pc, gate_results: [{:structural, :ok}]})

      assert {:error, :human_gate_required_by_phase_config_but_missing} =
               PhaseOutcome.new(attrs)
    end

    test "human in gate_results but :rejected does NOT satisfy the requirement" do
      {:ok, pc} =
        PhaseConfig.new(%{
          phase: :execute,
          agent_role: :executor,
          tools: [],
          gates: %{structural: [], semantic: [], human: [:commission_approved]},
          prompt_template_key: :execute,
          max_turns: 5,
          default_valid_until_days: 30
        })

      attrs = base_attrs(%{phase_config: pc, gate_results: [{:human, :rejected}]})

      assert {:error, :human_gate_required_by_phase_config_but_missing} =
               PhaseOutcome.new(attrs)
    end

    test "no human gate declared → no approval needed" do
      # Base attrs already use a phase_config with empty human list.
      assert {:ok, _} = PhaseOutcome.new(base_attrs())
    end
  end

  # ——— other invariants ———

  describe "verdict-only-on-terminal" do
    test "verdict on non-terminal phase → fail" do
      attrs = base_attrs(%{verdict: :pass})
      assert {:error, :verdict_only_on_terminal} = PhaseOutcome.new(attrs)
    end
  end

  describe "Measure requires non-empty evidence" do
    test ":measure with no evidence → fail" do
      {:ok, pc} =
        PhaseConfig.new(%{
          phase: :measure,
          agent_role: :measurer,
          tools: [],
          gates: %{structural: [], semantic: [], human: []},
          prompt_template_key: :measure,
          max_turns: 1,
          default_valid_until_days: 30
        })

      attrs =
        base_attrs(%{
          phase: :measure,
          authoring_role: :measurer,
          phase_config: pc,
          evidence: []
        })

      assert {:error, :evidence_required_on_measure} = PhaseOutcome.new(attrs)
    end
  end

  describe "authoring_role matches phase_config.agent_role" do
    test ":executor authoring an :execute-phase outcome → ok" do
      assert {:ok, _} = PhaseOutcome.new(base_attrs(%{authoring_role: :executor}))
    end

    test "role mismatch → fail" do
      attrs = base_attrs(%{authoring_role: :measurer})
      assert {:error, :authoring_role_does_not_match_phase_config} = PhaseOutcome.new(attrs)
    end

    test ":judge and :human are always accepted (gate-authored outcomes)" do
      assert {:ok, _} = PhaseOutcome.new(base_attrs(%{authoring_role: :judge}))
      assert {:ok, _} = PhaseOutcome.new(base_attrs(%{authoring_role: :human}))
    end
  end
end
