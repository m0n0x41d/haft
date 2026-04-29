defmodule OpenSleigh.CommissionSource.IntakeTest do
  use ExUnit.Case, async: true

  alias OpenSleigh.CommissionSource.Intake
  alias OpenSleigh.Tracker.Mock, as: TrackerMock
  alias OpenSleigh.{Scope, WorkCommission}

  defmodule SourceAdapter do
    def list_runnable(%{commissions: commissions}), do: {:ok, commissions}

    def record_skip(%{owner: owner}, commission_id, reason) do
      send(owner, {:skip_recorded, commission_id, reason})

      :ok
    end

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

  test "replenish skips commissions older than the default lease age cap", ctx do
    stale_fetched_at =
      DateTime.utc_now()
      |> DateTime.add(-25 * 60 * 60, :second)

    source =
      Intake.source_ref(
        SourceAdapter,
        %{
          owner: self(),
          claims: %{},
          commissions: [
            commission_fixture!("wc-intake-stale", %{
              state: :preflighting,
              fetched_at: stale_fetched_at
            }),
            commission_fixture!("wc-intake-fresh")
          ]
        },
        1,
        true
      )

    assert {:ok, 1} = Intake.replenish(source, TrackerMock, ctx.tracker)
    assert_receive {:skip_recorded, "wc-intake-stale", {:lease_too_old, details}}
    assert details.commission_id == "wc-intake-stale"
    assert details.state == :preflighting
    assert details.age_seconds > 24 * 60 * 60
    assert details.lease_age_cap_seconds == 24 * 60 * 60
    assert_receive {:claim_requested, "wc-intake-fresh"}
    refute_received {:claim_requested, "wc-intake-stale"}

    assert {:ok, [%{id: "wc-intake-fresh"}]} = TrackerMock.list_active(ctx.tracker)
  end

  test "replenish does not age-cap queued commissions", ctx do
    old_fetched_at =
      DateTime.utc_now()
      |> DateTime.add(-25 * 60 * 60, :second)

    source =
      Intake.source_ref(
        SourceAdapter,
        %{
          owner: self(),
          claims: %{},
          commissions: [
            commission_fixture!("wc-intake-old-queued", %{fetched_at: old_fetched_at})
          ]
        },
        1,
        true
      )

    assert {:ok, 1} = Intake.replenish(source, TrackerMock, ctx.tracker)
    assert_receive {:claim_requested, "wc-intake-old-queued"}
    refute_received {:skip_recorded, "wc-intake-old-queued", _reason}

    assert {:ok, [%{id: "wc-intake-old-queued"}]} = TrackerMock.list_active(ctx.tracker)
  end

  test "replenish honors an explicit lease age cap", ctx do
    fetched_at =
      DateTime.utc_now()
      |> DateTime.add(-25 * 60 * 60, :second)

    source =
      Intake.source_ref(
        SourceAdapter,
        %{
          owner: self(),
          claims: %{},
          commissions: [
            commission_fixture!("wc-intake-within-custom-cap", %{
              state: :preflighting,
              fetched_at: fetched_at
            })
          ]
        },
        1,
        true,
        48 * 60 * 60
      )

    assert {:ok, 1} = Intake.replenish(source, TrackerMock, ctx.tracker)
    assert_receive {:claim_requested, "wc-intake-within-custom-cap"}
    refute_received {:skip_recorded, "wc-intake-within-custom-cap", _reason}

    assert {:ok, [%{id: "wc-intake-within-custom-cap"}]} = TrackerMock.list_active(ctx.tracker)
  end

  defp commission_fixture!(id) do
    commission_fixture!(id, %{})
  end

  defp commission_fixture!(id, attrs) do
    fetched_at =
      DateTime.utc_now()
      |> DateTime.add(-60, :second)

    scope = scope_fixture!()

    base_attrs = %{
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
      fetched_at: fetched_at
    }

    attrs =
      base_attrs
      |> Map.merge(attrs)

    attrs
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
