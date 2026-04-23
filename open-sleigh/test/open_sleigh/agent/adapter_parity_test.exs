defmodule OpenSleigh.Agent.AdapterParityTest do
  use ExUnit.Case, async: true

  alias OpenSleigh.Agent.{Adapter, Claude, Codex}
  alias OpenSleigh.{Fixtures, Ticket}

  @adapters [Codex, Claude]

  test "provider adapters expose the same tool registry" do
    registries =
      @adapters
      |> Map.new(&{&1.adapter_kind(), MapSet.new(&1.tool_registry())})

    assert registries.claude == registries.codex
    assert :read in registries.codex
    assert :haft_decision in registries.codex
  end

  test "provider adapters reject unknown tools with the same error" do
    session = Fixtures.adapter_session(%{scoped_tools: MapSet.new([:not_registered])})

    @adapters
    |> Enum.each(fn adapter ->
      assert {:error, :tool_unknown_to_adapter} =
               adapter.dispatch_tool(%{}, :not_registered, %{}, session)
    end)
  end

  test "provider adapters enforce phase-scoped tools before execution" do
    session = Fixtures.adapter_session(%{scoped_tools: MapSet.new([:read])})

    @adapters
    |> Enum.each(fn adapter ->
      assert {:error, :tool_forbidden_by_phase_scope} =
               adapter.dispatch_tool(%{}, :bash, %{}, session)
    end)
  end

  test "adapter session carries commission scope context" do
    commission = synthetic_commission!()

    session =
      %{scoped_tools: MapSet.new([:write])}
      |> Fixtures.adapter_session()
      |> Adapter.attach_commission_context(commission)

    assert Adapter.commission_id(session) == commission.id
    assert Adapter.commission(session) == commission
    assert Adapter.scope(session) == commission.scope
  end

  test "shared adapter scope helper rejects path outside commission scope" do
    commission = synthetic_commission!()

    session =
      %{scoped_tools: MapSet.new([:write])}
      |> Fixtures.adapter_session()
      |> Adapter.attach_commission_context(commission)

    assert :ok =
             Adapter.ensure_in_scope(session, :write, %{
               path: "open-sleigh/lib/open_sleigh/orchestrator.ex"
             })

    assert {:error, :mutation_outside_commission_scope} =
             Adapter.ensure_in_scope(session, :write, %{
               path: "open-sleigh/lib/open_sleigh/ticket.ex"
             })
  end

  test "provider adapters pass tool args into commission scope checks" do
    commission = synthetic_commission!()

    session =
      %{scoped_tools: MapSet.new([:write])}
      |> Fixtures.adapter_session()
      |> Adapter.attach_commission_context(commission)

    @adapters
    |> Enum.each(fn adapter ->
      assert {:error, :tool_execution_failed} =
               adapter.dispatch_tool(
                 %{},
                 :write,
                 %{path: "open-sleigh/lib/open_sleigh/orchestrator.ex"},
                 session
               )

      assert {:error, :mutation_outside_commission_scope} =
               adapter.dispatch_tool(
                 %{},
                 :write,
                 %{path: "open-sleigh/lib/open_sleigh/ticket.ex"},
                 session
               )
    end)
  end

  test "Claude skeleton fails live session startup explicitly" do
    session = Fixtures.adapter_session(%{adapter_kind: :claude})

    assert {:error, :agent_command_not_found} = Claude.start_session(session)
  end

  defp synthetic_commission! do
    %{
      id: "OCT-SCOPE",
      source: {:linear, "oct"},
      title: "Scope context",
      body: "",
      state: :in_progress,
      problem_card_ref: "haft-pc-abc",
      target_branch: "feature/scope-context",
      fetched_at: ~U[2026-04-22 10:00:00Z],
      metadata: %{
        allowed_paths: ["open-sleigh/lib/open_sleigh/orchestrator.ex"],
        affected_files: ["open-sleigh/lib/open_sleigh/orchestrator.ex"],
        lockset: ["open-sleigh/lib/open_sleigh/orchestrator.ex"]
      }
    }
    |> Ticket.new()
    |> unwrap!()
    |> Ticket.to_work_commission()
    |> unwrap!()
  end

  defp unwrap!({:ok, value}), do: value
end
