defmodule OpenSleigh.Gates.Structural.PreflightTest do
  use ExUnit.Case, async: true

  alias OpenSleigh.{
    CommissionRevisionSnapshot,
    GateContext,
    PhaseConfig,
    Scope,
    SessionScopedArtifactId,
    Ticket,
    WorkCommission
  }

  alias OpenSleigh.Gates.Structural.{
    CommissionRunnable,
    DecisionFresh,
    ScopeSnapshotFresh
  }

  @checked_at ~U[2026-04-22 10:00:00Z]
  @valid_until ~U[2026-05-22 10:00:00Z]

  describe "preflight structural gates" do
    test "pass when commission and deterministic snapshots match" do
      commission = commission()
      commission_snapshot = snapshot(commission)
      current_snapshot = snapshot(commission)

      ctx =
        context(%{
          commission: commission,
          commission_snapshot: commission_snapshot,
          current_snapshot: current_snapshot,
          current_decision: current_decision()
        })

      assert :ok = CommissionRunnable.apply(ctx)
      assert :ok = DecisionFresh.apply(ctx)
      assert :ok = ScopeSnapshotFresh.apply(ctx)
    end

    test "commission_runnable blocks expired commissions before Execute" do
      commission = commission_with(%{valid_until: ~U[2026-04-21 10:00:00Z]})
      commission_snapshot = snapshot(commission)

      ctx =
        context(%{
          commission: commission,
          commission_snapshot: commission_snapshot,
          current_snapshot: commission_snapshot,
          current_decision: current_decision()
        })

      assert {:error, :commission_expired} = CommissionRunnable.apply(ctx)
    end

    test "commission_runnable blocks commissions without claimed preflight lease" do
      commission = commission()
      commission_snapshot = snapshot_with(snapshot(commission), %{lease_id: nil})

      ctx =
        context(%{
          commission: commission,
          commission_snapshot: commission_snapshot,
          current_snapshot: commission_snapshot,
          current_decision: current_decision()
        })

      assert {:error, :preflight_lease_missing} = CommissionRunnable.apply(ctx)
    end

    test "decision_fresh blocks superseded decisions before Execute" do
      commission = commission()
      commission_snapshot = snapshot(commission)

      ctx =
        context(%{
          commission: commission,
          commission_snapshot: commission_snapshot,
          current_snapshot: commission_snapshot,
          current_decision: current_decision(%{status: :superseded})
        })

      assert {:error, :decision_superseded} = DecisionFresh.apply(ctx)
    end

    test "decision_fresh blocks stale decision freshness before Execute" do
      commission = commission()
      commission_snapshot = snapshot(commission)

      ctx =
        context(%{
          commission: commission,
          commission_snapshot: commission_snapshot,
          current_snapshot: commission_snapshot,
          current_decision: current_decision(%{freshness: :stale})
        })

      assert {:error, :decision_stale} = DecisionFresh.apply(ctx)
    end

    test "decision_fresh blocks decision revision drift before Execute" do
      commission = commission()
      commission_snapshot = snapshot(commission)

      current_snapshot =
        snapshot_with(commission_snapshot, %{decision_revision_hash: "decision-r2"})

      ctx =
        context(%{
          commission: commission,
          commission_snapshot: commission_snapshot,
          current_snapshot: current_snapshot,
          current_decision: current_decision(%{decision_revision_hash: "decision-r2"})
        })

      assert {:error, :decision_revision_changed} = DecisionFresh.apply(ctx)
    end

    test "scope_snapshot_fresh blocks scope hash drift before Execute" do
      commission = commission()
      commission_snapshot = snapshot(commission)

      current_snapshot =
        snapshot_with(commission_snapshot, %{scope_hash: String.duplicate("a", 64)})

      ctx =
        context(%{
          commission: commission,
          commission_snapshot: commission_snapshot,
          current_snapshot: current_snapshot,
          current_decision: current_decision()
        })

      assert {:error, :scope_hash_changed} = ScopeSnapshotFresh.apply(ctx)
    end

    test "scope_snapshot_fresh blocks base SHA drift before Execute" do
      commission = commission()
      commission_snapshot = snapshot(commission)
      current_snapshot = snapshot_with(commission_snapshot, %{base_sha: "base-r2"})

      ctx =
        context(%{
          commission: commission,
          commission_snapshot: commission_snapshot,
          current_snapshot: current_snapshot,
          current_decision: current_decision()
        })

      assert {:error, :base_sha_changed} = ScopeSnapshotFresh.apply(ctx)
    end

    test "scope_snapshot_fresh blocks implementation plan drift before Execute" do
      commission = commission()
      commission_snapshot = snapshot(commission)

      current_snapshot =
        snapshot_with(commission_snapshot, %{implementation_plan_revision: "plan-r2"})

      ctx =
        context(%{
          commission: commission,
          commission_snapshot: commission_snapshot,
          current_snapshot: current_snapshot,
          current_decision: current_decision()
        })

      assert {:error, :implementation_plan_revision_changed} =
               ScopeSnapshotFresh.apply(ctx)
    end

    test "scope_snapshot_fresh blocks autonomy envelope drift before Execute" do
      commission = commission()
      commission_snapshot = snapshot(commission)

      current_snapshot =
        snapshot_with(commission_snapshot, %{autonomy_envelope_revision: "envelope-r2"})

      ctx =
        context(%{
          commission: commission,
          commission_snapshot: commission_snapshot,
          current_snapshot: current_snapshot,
          current_decision: current_decision()
        })

      assert {:error, :autonomy_envelope_revision_changed} =
               ScopeSnapshotFresh.apply(ctx)
    end

    test "scope_snapshot_fresh treats unknown current snapshot as non-pass" do
      commission = commission()
      commission_snapshot = snapshot(commission)

      ctx =
        context(%{
          commission: commission,
          commission_snapshot: commission_snapshot,
          current_snapshot: :uncertain,
          current_decision: current_decision()
        })

      assert {:error, :missing_current_snapshot} = ScopeSnapshotFresh.apply(ctx)
    end
  end

  defp context(turn_result) do
    {:ok, ctx} =
      %{
        phase: :execute,
        phase_config: phase_config(),
        ticket: ticket(),
        self_id: SessionScopedArtifactId.generate(),
        config_hash: OpenSleigh.ConfigHash.from_iodata("preflight-test"),
        turn_result: Map.put(turn_result, :checked_at, @checked_at),
        evidence: []
      }
      |> GateContext.new()

    ctx
  end

  defp phase_config do
    {:ok, phase_config} =
      %{
        phase: :execute,
        agent_role: :executor,
        tools: [],
        gates: %{
          structural: [:commission_runnable, :decision_fresh, :scope_snapshot_fresh],
          semantic: [],
          human: []
        },
        prompt_template_key: :execute,
        max_turns: 1,
        default_valid_until_days: 1
      }
      |> PhaseConfig.new()

    phase_config
  end

  defp ticket do
    {:ok, ticket} =
      %{
        id: "ticket-1",
        source: {:github, "m0n0x41d/haft"},
        title: "Preflight structural gates",
        body: "",
        state: :ready,
        problem_card_ref: "pc-20260422-001",
        target_branch: "feature/preflight",
        fetched_at: @checked_at
      }
      |> Ticket.new()

    ticket
  end

  defp current_decision do
    current_decision(%{})
  end

  defp current_decision(overrides) do
    %{
      decision_ref: "dec-20260422-001",
      decision_revision_hash: "decision-r1",
      status: :active,
      refresh_due: false,
      freshness: :healthy
    }
    |> Map.merge(overrides)
  end

  defp commission do
    commission_with(%{})
  end

  defp commission_with(overrides) do
    {:ok, commission} =
      scope()
      |> commission_attrs()
      |> Map.merge(overrides)
      |> WorkCommission.new()

    commission
  end

  defp commission_attrs(scope) do
    %{
      id: "wc-20260422-001",
      decision_ref: "dec-20260422-001",
      decision_revision_hash: "decision-r1",
      problem_card_ref: "pc-20260422-001",
      implementation_plan_ref: "plan-20260422-001",
      implementation_plan_revision: "plan-r1",
      scope: scope,
      scope_hash: scope.hash,
      base_sha: scope.base_sha,
      lockset: scope.lockset,
      evidence_requirements: [],
      projection_policy: :local_only,
      autonomy_envelope_ref: "envelope-20260422-001",
      autonomy_envelope_revision: "envelope-r1",
      state: :preflighting,
      valid_until: @valid_until,
      fetched_at: @checked_at
    }
  end

  defp scope do
    attrs = scope_attrs()
    {:ok, hash} = Scope.canonical_hash(attrs)

    {:ok, scope} =
      attrs
      |> Map.put(:hash, hash)
      |> Scope.new()

    scope
  end

  defp scope_attrs do
    %{
      repo_ref: "github:m0n0x41d/haft",
      base_sha: "base-r1",
      target_branch: "feature/preflight",
      allowed_paths: [
        "open-sleigh/lib/open_sleigh/gates/structural/commission_runnable.ex",
        "open-sleigh/lib/open_sleigh/gates/structural/decision_fresh.ex",
        "open-sleigh/lib/open_sleigh/gates/structural/scope_snapshot_fresh.ex"
      ],
      forbidden_paths: [],
      allowed_actions: MapSet.new([:edit_files, :run_tests]),
      affected_files: [
        "open-sleigh/lib/open_sleigh/gates/structural/commission_runnable.ex",
        "open-sleigh/lib/open_sleigh/gates/structural/decision_fresh.ex",
        "open-sleigh/lib/open_sleigh/gates/structural/scope_snapshot_fresh.ex"
      ],
      allowed_modules: [],
      lockset: [
        "open-sleigh/lib/open_sleigh/gates/structural/commission_runnable.ex",
        "open-sleigh/lib/open_sleigh/gates/structural/decision_fresh.ex",
        "open-sleigh/lib/open_sleigh/gates/structural/scope_snapshot_fresh.ex"
      ]
    }
  end

  defp snapshot(%WorkCommission{} = commission) do
    {:ok, snapshot} =
      commission
      |> WorkCommission.revision_snapshot(%{
        problem_revision_hash: "problem-r1",
        lease_id: "lease-1",
        lease_state: :claimed_for_preflight
      })

    snapshot
  end

  defp snapshot_with(%CommissionRevisionSnapshot{} = snapshot, overrides) do
    {:ok, updated_snapshot} =
      snapshot
      |> Map.from_struct()
      |> Map.merge(overrides)
      |> CommissionRevisionSnapshot.new()

    updated_snapshot
  end
end
