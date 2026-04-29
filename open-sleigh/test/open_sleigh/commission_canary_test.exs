defmodule OpenSleigh.CommissionCanaryTest do
  @moduledoc """
  Local commission-first canaries.

  These tests use only fixture-backed WorkCommissions. They do not need
  Linear, Jira, GitHub, or Haft server credentials, and they do not start
  ProjectionWriterAgent.
  """

  use ExUnit.Case, async: false

  alias OpenSleigh.Adapter.PathGuard
  alias OpenSleigh.Agent.Adapter
  alias OpenSleigh.CommissionRevisionSnapshot
  alias OpenSleigh.CommissionSource.Local

  alias OpenSleigh.Gates.Structural.{
    CommissionRunnable,
    DecisionFresh,
    ScopeSnapshotFresh
  }

  alias OpenSleigh.{
    AdapterSession,
    ConfigHash,
    GateContext,
    PhaseConfig,
    SessionId,
    SessionScopedArtifactId,
    Ticket,
    WorkCommission
  }

  @checked_at ~U[2026-04-22 10:00:00Z]

  setup do
    suffix =
      :erlang.unique_integer([:positive, :monotonic])

    tmp =
      System.tmp_dir!()
      |> Path.join("open_sleigh_commission_canary_#{suffix}")

    workspace =
      tmp
      |> Path.join("workspace")

    File.mkdir_p!(workspace)

    on_exit(fn -> File.rm_rf!(tmp) end)

    %{
      workspace: workspace,
      guard_config: %{forbidden_paths: [], forbidden_remote_substrings: ["open-sleigh"]}
    }
  end

  test "green local-only canary claims a fixture commission and passes preflight", ctx do
    fixture =
      "green_local_only.json"
      |> fixture_path()

    handle =
      fixture
      |> local_handle!()

    assert {:ok, runnable} = Local.list_runnable(handle)

    runnable_ids =
      runnable
      |> Enum.map(& &1.id)

    assert runnable_ids == ["wc-canary-green-local"]

    commission =
      handle
      |> claim!("wc-canary-green-local")

    commission_snapshot =
      commission
      |> snapshot!("problem-r1", "lease-green")

    current_snapshot = commission_snapshot

    preflight_ctx =
      commission
      |> preflight_context!(
        commission_snapshot,
        current_snapshot,
        current_decision("decision-r1")
      )

    session =
      ctx.workspace
      |> adapter_session!(commission)

    target_path =
      ctx.workspace
      |> Path.join("src/local_green.ex")

    assert commission.projection_policy == :local_only
    assert commission.state == :preflighting
    assert Adapter.commission_id(session) == "wc-canary-green-local"
    assert :pass = preflight_verdict(preflight_ctx)
    assert :ok = Adapter.ensure_in_scope(session, :write, %{path: target_path})
    assert :ok = Adapter.validate_terminal_diff(session, ["src/local_green.ex"])
  end

  test "stale-block canary stops a revision-drifted commission before Execute" do
    commission =
      "stale_block.json"
      |> fixture_path()
      |> local_handle!()
      |> claim!("wc-canary-stale-block")

    commission_snapshot =
      commission
      |> snapshot!("problem-r1", "lease-stale")

    current_snapshot =
      commission_snapshot
      |> snapshot_with!(%{decision_revision_hash: "decision-r2"})

    ctx =
      commission
      |> preflight_context!(
        commission_snapshot,
        current_snapshot,
        current_decision("decision-r2")
      )

    assert {:blocked_stale, :decision_revision_changed} = preflight_verdict(ctx)
  end

  test "scope-block canary rejects in-workspace mutation outside commission scope", ctx do
    commission =
      "scope_block.json"
      |> fixture_path()
      |> local_handle!()
      |> claim!("wc-canary-scope-block")

    session =
      ctx.workspace
      |> adapter_session!(commission)

    allowed_path =
      ctx.workspace
      |> Path.join("src/allowed.ex")

    outside_path =
      ctx.workspace
      |> Path.join("src/outside.ex")

    assert {:ok, "src/outside.ex"} =
             PathGuard.relative_to_workspace(ctx.workspace, outside_path, ctx.guard_config)

    assert :ok = Adapter.ensure_in_scope(session, :write, %{path: allowed_path})

    assert {:terminal_failure, :mutation_outside_commission_scope} =
             mutation_verdict(session, outside_path)

    assert {:error, :mutation_outside_commission_scope} =
             Adapter.validate_terminal_diff(session, ["src/allowed.ex", "src/outside.ex"])
  end

  defp fixture_path(name) do
    __DIR__
    |> Path.join("../fixtures/commissions")
    |> Path.join(name)
    |> Path.expand()
  end

  defp local_handle!(fixture_path) do
    config = %{commission_source: %{fixture_path: fixture_path}}

    {:ok, handle} =
      config
      |> Local.new()

    handle
  end

  defp claim!(handle, commission_id) do
    {:ok, commission} =
      handle
      |> Local.claim_for_preflight(commission_id)

    commission
  end

  defp snapshot!(commission, problem_revision_hash, lease_id) do
    attrs = %{
      problem_revision_hash: problem_revision_hash,
      lease_id: lease_id,
      lease_state: :claimed_for_preflight
    }

    {:ok, snapshot} =
      commission
      |> WorkCommission.revision_snapshot(attrs)

    snapshot
  end

  defp snapshot_with!(snapshot, overrides) do
    attrs =
      snapshot
      |> Map.from_struct()
      |> Map.merge(overrides)

    {:ok, updated_snapshot} =
      attrs
      |> CommissionRevisionSnapshot.new()

    updated_snapshot
  end

  defp current_decision(decision_revision_hash) do
    %{
      decision_ref: "dec-20260422-001",
      decision_revision_hash: decision_revision_hash,
      status: :active,
      refresh_due: false,
      freshness: :healthy
    }
  end

  defp preflight_context!(
         commission,
         commission_snapshot,
         current_snapshot,
         current_decision
       ) do
    turn_result =
      %{}
      |> Map.put(:commission, commission)
      |> Map.put(:commission_snapshot, commission_snapshot)
      |> Map.put(:current_snapshot, current_snapshot)
      |> Map.put(:current_decision, current_decision)
      |> Map.put(:checked_at, @checked_at)

    attrs = %{
      phase: :execute,
      phase_config: preflight_phase_config!(),
      ticket: ticket!(commission),
      self_id: SessionScopedArtifactId.generate(),
      config_hash: ConfigHash.from_iodata("commission-canary"),
      turn_result: turn_result,
      evidence: []
    }

    {:ok, ctx} =
      attrs
      |> GateContext.new()

    ctx
  end

  defp preflight_phase_config! do
    attrs = %{
      phase: :execute,
      agent_role: :executor,
      tools: [:read, :write, :bash],
      gates: %{
        structural: [:commission_runnable, :decision_fresh, :scope_snapshot_fresh],
        semantic: [],
        human: []
      },
      prompt_template_key: :execute,
      max_turns: 1,
      default_valid_until_days: 1
    }

    {:ok, config} =
      attrs
      |> PhaseConfig.new()

    config
  end

  defp ticket!(commission) do
    attrs = %{
      id: commission.id,
      source: {:github, "local/commission-fixture"},
      title: "Local commission canary",
      body: "",
      state: :ready,
      problem_card_ref: commission.problem_card_ref,
      target_branch: commission.scope.target_branch,
      fetched_at: @checked_at
    }

    {:ok, ticket} =
      attrs
      |> Ticket.new()

    ticket
  end

  defp preflight_verdict(ctx) do
    ctx
    |> preflight_gate_results()
    |> preflight_verdict_from_results()
  end

  defp preflight_gate_results(ctx) do
    runnable =
      ctx
      |> CommissionRunnable.apply()

    decision =
      ctx
      |> DecisionFresh.apply()

    scope =
      ctx
      |> ScopeSnapshotFresh.apply()

    [runnable, decision, scope]
  end

  defp preflight_verdict_from_results(results) do
    results
    |> Enum.find(&match?({:error, _reason}, &1))
    |> preflight_error_or_pass()
  end

  defp preflight_error_or_pass(nil), do: :pass
  defp preflight_error_or_pass({:error, reason}), do: blocked_preflight_reason(reason)

  defp blocked_preflight_reason(:decision_revision_changed),
    do: {:blocked_stale, :decision_revision_changed}

  defp blocked_preflight_reason(:decision_ref_changed),
    do: {:blocked_stale, :decision_ref_changed}

  defp blocked_preflight_reason(:decision_superseded),
    do: {:blocked_stale, :decision_superseded}

  defp blocked_preflight_reason(:decision_deprecated),
    do: {:blocked_stale, :decision_deprecated}

  defp blocked_preflight_reason(:decision_refresh_due),
    do: {:blocked_stale, :decision_refresh_due}

  defp blocked_preflight_reason(:decision_stale),
    do: {:blocked_stale, :decision_stale}

  defp blocked_preflight_reason(reason), do: {:blocked_policy, reason}

  defp adapter_session!(workspace, commission) do
    attrs = %{
      session_id: SessionId.generate(),
      config_hash: ConfigHash.from_iodata("commission-canary"),
      scoped_tools: MapSet.new([:read, :write, :bash]),
      workspace_path: workspace,
      adapter_kind: :codex,
      adapter_version: "0.14.0",
      max_turns: 20,
      max_tokens_per_turn: 80_000,
      wall_clock_timeout_s: 600
    }

    {:ok, session} =
      attrs
      |> AdapterSession.new()

    session
    |> Adapter.attach_commission_context(commission)
  end

  defp mutation_verdict(session, path) do
    session
    |> Adapter.ensure_in_scope(:write, %{path: path})
    |> mutation_result()
  end

  defp mutation_result(:ok), do: :pass

  defp mutation_result({:error, :mutation_outside_commission_scope}),
    do: {:terminal_failure, :mutation_outside_commission_scope}

  defp mutation_result({:error, reason}), do: {:blocked_policy, reason}
end
