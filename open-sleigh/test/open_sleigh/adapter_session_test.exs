defmodule OpenSleigh.AdapterSessionTest do
  use ExUnit.Case, async: true

  alias OpenSleigh.{AdapterSession, ConfigHash, SessionId}

  setup do
    %{
      session_id: SessionId.generate(),
      config_hash: ConfigHash.from_iodata("x"),
      scoped_tools: MapSet.new([:read, :write]),
      workspace_path: "/tmp/open-sleigh-workspaces/OCT-1",
      adapter_kind: :codex,
      adapter_version: "0.14.0",
      max_turns: 20,
      max_tokens_per_turn: 80_000,
      wall_clock_timeout_s: 600
    }
  end

  test "new/1 happy path", ctx do
    assert {:ok, %AdapterSession{}} = AdapterSession.new(ctx)
  end

  test "scoped_tools must be a MapSet (not list)", ctx do
    attrs = Map.put(ctx, :scoped_tools, [:read, :write])
    assert {:error, :invalid_scoped_tools} = AdapterSession.new(attrs)
  end

  test "workspace_path must be non-empty binary", ctx do
    attrs = Map.put(ctx, :workspace_path, "")
    assert {:error, :invalid_workspace_path} = AdapterSession.new(attrs)

    attrs = Map.put(ctx, :workspace_path, nil)
    assert {:error, :invalid_workspace_path} = AdapterSession.new(attrs)
  end

  test "positive-integer bounds on budgets", ctx do
    for field <- [:max_turns, :max_tokens_per_turn, :wall_clock_timeout_s] do
      attrs = Map.put(ctx, field, 0)
      assert {:error, _} = AdapterSession.new(attrs)
    end
  end

  test "invalid config_hash rejected", ctx do
    attrs = Map.put(ctx, :config_hash, "not-a-hash")
    assert {:error, :invalid_config_hash} = AdapterSession.new(attrs)
  end
end
