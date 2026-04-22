defmodule OpenSleigh.Fixtures do
  @moduledoc """
  Test fixtures for constructing valid L1/L2/L3 structs with minimum
  boilerplate. Test-env only (see `elixirc_paths(:test)` in `mix.exs`).
  """

  alias OpenSleigh.{
    AdapterSession,
    ConfigHash,
    Evidence,
    GateContext,
    HumanGateApproval,
    PhaseConfig,
    PhaseOutcome,
    Session,
    SessionId,
    SessionScopedArtifactId,
    Ticket,
    Workflow,
    WorkflowState
  }

  @valid_ts ~U[2026-04-22 10:00:00Z]

  # ——— primitives ———

  @spec config_hash() :: ConfigHash.t()
  def config_hash, do: ConfigHash.from_iodata("test-config")

  @spec self_id() :: SessionScopedArtifactId.t()
  def self_id, do: SessionScopedArtifactId.generate()

  @spec session_id() :: SessionId.t()
  def session_id, do: SessionId.generate()

  @spec utc(keyword()) :: DateTime.t()
  def utc(_opts \\ []), do: @valid_ts

  # ——— structs ———

  @spec ticket(keyword() | map()) :: Ticket.t()
  def ticket(overrides \\ %{}) do
    defaults = %{
      id: "tracker-1",
      source: {:linear, "oct"},
      title: "Test ticket",
      body: "",
      state: :in_progress,
      problem_card_ref: "haft-pc-abc",
      target_branch: "feature/x",
      fetched_at: @valid_ts
    }

    {:ok, t} = defaults |> Map.merge(Map.new(overrides)) |> Ticket.new()
    t
  end

  @spec evidence(keyword() | map()) :: Evidence.t()
  def evidence(overrides \\ %{}) do
    defaults = %{
      kind: :pr_merge_sha,
      ref: "sha-abc",
      hash: nil,
      cl: 3,
      authoring_source: :git_host,
      captured_at: @valid_ts
    }

    merged = Map.merge(defaults, Map.new(overrides))

    {:ok, e} =
      Evidence.new(
        merged.kind,
        merged.ref,
        merged.hash,
        merged.cl,
        merged.authoring_source,
        merged.captured_at
      )

    e
  end

  @spec phase_config_execute(keyword() | map()) :: PhaseConfig.t()
  def phase_config_execute(overrides \\ %{}) do
    defaults = %{
      phase: :execute,
      agent_role: :executor,
      tools: [:read, :write, :bash],
      gates: %{
        structural: [:design_runtime_split_ok],
        semantic: [:lade_quadrants_split_ok],
        human: []
      },
      prompt_template_key: :execute,
      max_turns: 20,
      default_valid_until_days: 30
    }

    {:ok, pc} = defaults |> Map.merge(Map.new(overrides)) |> PhaseConfig.new()
    pc
  end

  @spec phase_config_frame(keyword() | map()) :: PhaseConfig.t()
  def phase_config_frame(overrides \\ %{}) do
    defaults = %{
      phase: :frame,
      agent_role: :frame_verifier,
      tools: [:haft_query, :read, :grep],
      gates: %{
        structural: [:problem_card_ref_present, :described_entity_field_present],
        semantic: [:object_of_talk_is_specific],
        human: []
      },
      prompt_template_key: :frame,
      max_turns: 1,
      default_valid_until_days: 7
    }

    {:ok, pc} = defaults |> Map.merge(Map.new(overrides)) |> PhaseConfig.new()
    pc
  end

  @spec phase_config_measure(keyword() | map()) :: PhaseConfig.t()
  def phase_config_measure(overrides \\ %{}) do
    defaults = %{
      phase: :measure,
      agent_role: :measurer,
      tools: [:haft_decision, :haft_refresh],
      gates: %{
        structural: [:evidence_ref_not_self, :valid_until_field_present],
        semantic: [:no_self_evidence_semantic],
        human: []
      },
      prompt_template_key: :measure,
      max_turns: 1,
      default_valid_until_days: 30
    }

    {:ok, pc} = defaults |> Map.merge(Map.new(overrides)) |> PhaseConfig.new()
    pc
  end

  @spec adapter_session(keyword() | map()) :: AdapterSession.t()
  def adapter_session(overrides \\ %{}) do
    defaults = %{
      session_id: session_id(),
      config_hash: config_hash(),
      scoped_tools: MapSet.new([:read, :write]),
      workspace_path: "/tmp/open-sleigh-workspaces/OCT-1",
      adapter_kind: :codex,
      adapter_version: "0.14.0",
      max_turns: 20,
      max_tokens_per_turn: 80_000,
      wall_clock_timeout_s: 600
    }

    {:ok, as} = defaults |> Map.merge(Map.new(overrides)) |> AdapterSession.new()
    as
  end

  @spec session(keyword() | map()) :: Session.t()
  def session(overrides \\ %{}) do
    defaults = %{
      id: session_id(),
      ticket: ticket(),
      phase: :execute,
      config_hash: config_hash(),
      scoped_tools: MapSet.new([:read, :write]),
      workspace_path: "/tmp/open-sleigh-workspaces/OCT-1",
      claimed_at: @valid_ts,
      adapter_session: adapter_session()
    }

    {:ok, s} = defaults |> Map.merge(Map.new(overrides)) |> Session.new()
    s
  end

  @spec gate_context(keyword() | map()) :: GateContext.t()
  def gate_context(overrides \\ %{}) do
    defaults = %{
      phase: :execute,
      phase_config: phase_config_execute(),
      ticket: ticket(),
      self_id: self_id(),
      config_hash: config_hash(),
      turn_result: %{text: "implementation complete"},
      evidence: [evidence()],
      upstream_problem_card: %{
        "describedEntity" => "lib/my_app/auth.ex",
        "groundingHolon" => "MyApp.Auth",
        "authoring_source" => "human"
      },
      proposed_valid_until: DateTime.add(@valid_ts, 30 * 86_400, :second)
    }

    {:ok, ctx} = defaults |> Map.merge(Map.new(overrides)) |> GateContext.new()
    ctx
  end

  @spec human_gate_approval(keyword() | map()) :: HumanGateApproval.t()
  def human_gate_approval(overrides \\ %{}) do
    defaults = %{
      approver: "ivan@weareocta.com",
      at: @valid_ts,
      config_hash: config_hash(),
      signal_source: :tracker_comment,
      signal_ref: "linear://comment/1",
      reason: nil
    }

    m = Map.merge(defaults, Map.new(overrides))

    {:ok, a} =
      HumanGateApproval.new(
        m.approver,
        m.at,
        m.config_hash,
        m.signal_source,
        m.signal_ref,
        m.reason
      )

    a
  end

  @spec phase_outcome(keyword() | map()) :: PhaseOutcome.t()
  def phase_outcome(overrides \\ %{}) do
    defaults = %{
      session_id: session_id(),
      phase: :execute,
      work_product: %{pr_url: "https://github.com/x/pull/1"},
      evidence: [evidence()],
      gate_results: [{:structural, :ok}],
      config_hash: config_hash(),
      valid_until: DateTime.add(@valid_ts, 30 * 86_400, :second),
      authoring_role: :executor,
      self_id: self_id(),
      produced_at: @valid_ts,
      phase_config: phase_config_execute()
    }

    {:ok, po} = defaults |> Map.merge(Map.new(overrides)) |> PhaseOutcome.new()
    po
  end

  @spec workflow_state_mvp1(keyword() | map()) :: WorkflowState.t()
  def workflow_state_mvp1(overrides \\ %{}) do
    base = WorkflowState.start(Workflow.mvp1())

    Enum.reduce(Map.new(overrides), base, fn {k, v}, acc ->
      Map.put(acc, k, v)
    end)
  end
end
