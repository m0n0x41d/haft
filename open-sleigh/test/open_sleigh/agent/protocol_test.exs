defmodule OpenSleigh.Agent.ProtocolTest do
  use ExUnit.Case, async: true

  alias OpenSleigh.{Agent.Protocol, Fixtures}

  describe "encode_initialize/1" do
    test "produces line-terminated JSON-RPC request with id" do
      out = Protocol.encode_initialize(1)
      assert String.ends_with?(out, "\n")
      {:ok, decoded} = Jason.decode(out)
      assert decoded["method"] == "initialize"
      assert decoded["id"] == 1
      assert decoded["params"]["clientInfo"]["name"] == "open-sleigh"
      assert decoded["params"]["capabilities"]["experimentalApi"] == true
    end
  end

  describe "encode_initialized/0" do
    test "produces notification (no id)" do
      {:ok, decoded} = Protocol.encode_initialized() |> Jason.decode()
      assert decoded["method"] == "initialized"
      refute Map.has_key?(decoded, "id")
    end
  end

  describe "encode_thread_start/3" do
    test "attaches scoped_tools as list of strings" do
      session = Fixtures.adapter_session(%{scoped_tools: MapSet.new([:read, :write])})

      {:ok, decoded} = Protocol.encode_thread_start(2, session) |> Jason.decode()
      tools = decoded["params"]["tools"]
      assert is_list(tools)
      assert Enum.sort(tools) == ["read", "write"]
      assert decoded["params"]["cwd"] == session.workspace_path
    end

    test "uses supplied approval_policy" do
      session = Fixtures.adapter_session()
      opts = %{approval_policy: "never", sandbox: "read-only"}
      {:ok, decoded} = Protocol.encode_thread_start(2, session, opts) |> Jason.decode()
      assert decoded["params"]["approvalPolicy"] == "never"
      assert decoded["params"]["sandbox"] == "read-only"
    end
  end

  describe "encode_turn_start/5" do
    test "attaches config_hash as trailer in prompt" do
      session = Fixtures.adapter_session()

      {:ok, decoded} =
        Protocol.encode_turn_start(3, session, "thread-x", "Do the work") |> Jason.decode()

      prompt = get_in(decoded, ["params", "input", Access.at(0), "text"])
      assert prompt =~ "Do the work"
      assert prompt =~ "<!-- config_hash: " <> session.config_hash <> " -->"
    end

    test "uses supplied approval and sandbox policy" do
      session = Fixtures.adapter_session()

      opts = %{
        approval_policy: "never",
        sandbox_policy: %{"type" => "workspaceWrite"}
      }

      {:ok, decoded} =
        Protocol.encode_turn_start(3, session, "thread-x", "Do the work", opts)
        |> Jason.decode()

      assert decoded["params"]["approvalPolicy"] == "never"
      assert decoded["params"]["sandboxPolicy"] == %{"type" => "workspaceWrite"}
    end
  end

  describe "decode_line/1" do
    test "decodes a response line into {:response, id, result}" do
      line = Jason.encode!(%{"id" => 2, "result" => %{"thread" => %{"id" => "t1"}}})
      {:ok, {:response, id, result}} = Protocol.decode_line(line)
      assert id == 2
      assert result == %{"thread" => %{"id" => "t1"}}
    end

    test "decodes a method notification into {:event, event}" do
      line =
        Jason.encode!(%{
          "jsonrpc" => "2.0",
          "method" => "turn/completed",
          "params" => %{"turn_id" => "abc"}
        })

      {:ok, {:event, event}} = Protocol.decode_line(line)
      assert event.event == :turn_completed
      assert %DateTime{} = event.timestamp
      assert event.payload == %{"turn_id" => "abc"}
    end

    test "unknown method → :unsupported_event_category" do
      line = Jason.encode!(%{"method" => "something/weird", "params" => %{}})
      {:ok, {:event, event}} = Protocol.decode_line(line)
      assert event.event == :unsupported_event_category
    end

    test "invalid JSON → :response_parse_error" do
      assert {:error, :response_parse_error} = Protocol.decode_line("{not json")
    end
  end
end
