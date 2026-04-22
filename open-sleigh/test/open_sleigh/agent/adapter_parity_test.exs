defmodule OpenSleigh.Agent.AdapterParityTest do
  use ExUnit.Case, async: true

  alias OpenSleigh.Agent.{Claude, Codex}
  alias OpenSleigh.Fixtures

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

  test "Claude skeleton fails live session startup explicitly" do
    session = Fixtures.adapter_session(%{adapter_kind: :claude})

    assert {:error, :agent_command_not_found} = Claude.start_session(session)
  end
end
