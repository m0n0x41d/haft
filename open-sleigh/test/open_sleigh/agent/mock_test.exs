defmodule OpenSleigh.Agent.MockTest do
  use ExUnit.Case, async: true

  alias OpenSleigh.{Agent.Mock, Fixtures}

  setup do
    Mock.reset!()
  end

  describe "handshake" do
    test "start_session → returns a handle with thread_id" do
      session = Fixtures.adapter_session()
      assert {:ok, %{thread_id: "mock-thread-" <> _}} = Mock.start_session(session)
    end

    test "send_turn → :completed with usage" do
      session = Fixtures.adapter_session()
      {:ok, handle} = Mock.start_session(session)

      assert {:ok, reply} = Mock.send_turn(handle, "first prompt", session)
      assert reply.status == :completed
      assert reply.usage.total_tokens == 150
    end

    test "send_turn can return scripted non-completed status" do
      session = Fixtures.adapter_session()
      {:ok, handle} = Mock.start_session(session)

      :ok = Mock.put_turn_replies([%{status: :failed, text: "blocked"}])

      assert {:ok, reply} = Mock.send_turn(handle, "first prompt", session)
      assert reply.status == :failed
      assert reply.text == "blocked"
    end
  end

  describe "dispatch_tool — CL1 + CL2" do
    test "allows a tool in-registry AND in-phase-scope" do
      session =
        Fixtures.adapter_session(%{scoped_tools: MapSet.new([:read, :haft_query])})

      {:ok, handle} = Mock.start_session(session)

      assert {:ok, %{call_id: _, result: %{success: true, tool: :read}}} =
               Mock.dispatch_tool(handle, :read, %{path: "/tmp/x"}, session)
    end

    test "CL2 — tool in registry but NOT in phase scope" do
      # :bash is in @tool_registry but NOT in scoped_tools.
      session = Fixtures.adapter_session(%{scoped_tools: MapSet.new([:read])})
      {:ok, handle} = Mock.start_session(session)

      assert {:error, :tool_forbidden_by_phase_scope} =
               Mock.dispatch_tool(handle, :bash, %{cmd: "ls"}, session)
    end

    test "CL1 — tool unknown to adapter → :tool_unknown_to_adapter" do
      session = Fixtures.adapter_session(%{scoped_tools: MapSet.new([:hogwash])})
      {:ok, handle} = Mock.start_session(session)

      assert {:error, :tool_unknown_to_adapter} =
               Mock.dispatch_tool(handle, :hogwash, %{}, session)
    end

    test "CL3 — Frame session with haft_problem tool not in scope → forbidden" do
      # Frame's canonical scoped_tools excludes :haft_problem.
      session = Fixtures.adapter_session(%{scoped_tools: MapSet.new([:haft_query, :read])})
      {:ok, handle} = Mock.start_session(session)

      assert {:error, :tool_forbidden_by_phase_scope} =
               Mock.dispatch_tool(handle, :haft_problem, %{}, session)
    end
  end

  describe "close_session" do
    test "returns :ok" do
      session = Fixtures.adapter_session()
      {:ok, handle} = Mock.start_session(session)
      assert :ok = Mock.close_session(handle)
    end
  end

  test "adapter_kind and tool_registry" do
    assert Mock.adapter_kind() == :mock
    assert :read in Mock.tool_registry()
    assert :haft_query in Mock.tool_registry()
    # haft_problem IS in the adapter registry (CL1 allows it); scope
    # discipline at CL2 is what excludes it from Frame.
    assert :haft_problem in Mock.tool_registry()
  end
end
