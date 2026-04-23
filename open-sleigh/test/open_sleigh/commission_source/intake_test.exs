defmodule OpenSleigh.CommissionSource.IntakeTest do
  use ExUnit.Case, async: true

  alias OpenSleigh.CommissionSource.Intake
  alias OpenSleigh.Tracker.Mock, as: TrackerMock
  alias OpenSleigh.{Scope, WorkCommission}

  defmodule SourceAdapter do
    def list_runnable(%{commissions: commissions}), do: {:ok, commissions}

    def claim_for_preflight(
          %{owner: owner, claims: claims, commissions: commissions},
          commission_id
        ) do
      send(owner, {:claim_requested, commission_id})

      claims
      |> Map.get(commission_id, :ok)
      |> claim_result(commissions, commission_id)
    end

    defp claim_result(:ok, commissions, commission_id) do
      commissions
      |> Enum.find(&(&1.id == commission_id))
      |> claimed_result()
    end

    defp claim_result(reason, _commissions, _commission_id), do: {:error, reason}

    defp claimed_result(nil), do: {:error, :commission_not_found}

    defp claimed_result(commission) do
      commission
      |> Map.put(:state, :preflighting)
      |> then(&{:ok, &1})
    end
  end

  setup do
    {:ok, tracker} = TrackerMock.start()
    %{tracker: tracker}
  end

  test "replenish claims runnable commissions and seeds tracker tickets", ctx do
    source =
      Intake.source_ref(
        SourceAdapter,
        %{
          owner: self(),
          claims: %{},
          commissions: [commission_fixture!("wc-intake-001")]
        },
        1,
        true
      )

    assert {:ok, 1} = Intake.replenish(source, TrackerMock, ctx.tracker)
    assert_receive {:claim_requested, "wc-intake-001"}

    assert {:ok, [%{id: "wc-intake-001"}]} = TrackerMock.list_active(ctx.tracker)
  end

  test "replenish skips transient claim conflicts and continues", ctx do
    source =
      Intake.source_ref(
        SourceAdapter,
        %{
          owner: self(),
          claims: %{"wc-intake-locked" => :commission_lock_conflict},
          commissions: [
            commission_fixture!("wc-intake-locked"),
            commission_fixture!("wc-intake-free")
          ]
        },
        1,
        true
      )

    assert {:ok, 1} = Intake.replenish(source, TrackerMock, ctx.tracker)
    assert_receive {:claim_requested, "wc-intake-locked"}
    assert_receive {:claim_requested, "wc-intake-free"}

    assert {:ok, [%{id: "wc-intake-free"}]} = TrackerMock.list_active(ctx.tracker)
  end

  defp commission_fixture!(id) do
    scope = scope_fixture!()

    %{
      id: id,
      decision_ref: "dec-intake",
      decision_revision_hash: "decision-r1",
      problem_card_ref: "pc-intake",
      implementation_plan_ref: "plan-intake",
      implementation_plan_revision: "plan-r1",
      scope: scope,
      scope_hash: scope.hash,
      base_sha: scope.base_sha,
      lockset: scope.lockset,
      evidence_requirements: [],
      projection_policy: :local_only,
      state: :queued,
      valid_until: ~U[2099-01-01 00:00:00Z],
      fetched_at: ~U[2026-04-22 10:00:00Z]
    }
    |> WorkCommission.new()
    |> unwrap!()
  end

  defp scope_fixture! do
    attrs = %{
      repo_ref: "local:haft",
      base_sha: "base-r1",
      target_branch: "feature/intake",
      allowed_paths: ["**/*"],
      forbidden_paths: [],
      allowed_actions: MapSet.new([:edit_files, :run_tests]),
      affected_files: ["**/*"],
      allowed_modules: [],
      lockset: ["**/*"]
    }

    {:ok, hash} = Scope.canonical_hash(attrs)

    attrs
    |> Map.put(:hash, hash)
    |> Scope.new()
    |> unwrap!()
  end

  defp unwrap!({:ok, value}), do: value
end
