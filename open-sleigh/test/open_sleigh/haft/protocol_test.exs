defmodule OpenSleigh.Haft.ProtocolTest do
  use ExUnit.Case, async: true

  alias OpenSleigh.{Fixtures, Haft.Protocol}

  test "encode_initialize/1 builds MCP handshake request" do
    line = Protocol.encode_initialize(1)

    assert {:ok, decoded} = Jason.decode(line)
    assert decoded["id"] == 1
    assert decoded["method"] == "initialize"
    assert decoded["params"]["clientInfo"]["name"] == "open-sleigh"
  end

  test "encode_initialized/0 builds MCP initialized notification" do
    line = Protocol.encode_initialized()

    assert {:ok, decoded} = Jason.decode(line)
    assert decoded["method"] == "notifications/initialized"
    refute Map.has_key?(decoded, "id")
  end

  test "encode_call/5 attaches config_hash as arg" do
    session = Fixtures.adapter_session()

    assert {:ok, line} =
             Protocol.encode_call(1, :haft_query, :status, %{"project" => "oct"}, session)

    assert {:ok, decoded} = Jason.decode(line)
    assert decoded["method"] == "tools/call"
    assert decoded["params"]["name"] == "haft_query"
    assert decoded["params"]["arguments"]["config_hash"] == session.config_hash
    assert decoded["params"]["arguments"]["action"] == "status"
    assert decoded["params"]["arguments"]["project"] == "oct"
  end

  test "encode_call/5 rejects unknown tool atom" do
    session = Fixtures.adapter_session()

    assert {:error, :tool_unknown_to_adapter} =
             Protocol.encode_call(1, :haft_made_up, :x, %{}, session)
  end

  test "decode_response/1 parses success result" do
    line = Jason.encode!(%{"id" => 5, "result" => %{"x" => 1}})
    assert {:ok, {5, %{"x" => 1}}} = Protocol.decode_response(line)
  end

  test "encode_health_ping/1 builds haft_query status call" do
    line = Protocol.encode_health_ping(-2)

    assert {:ok, decoded} = Jason.decode(line)
    assert decoded["id"] == -2
    assert decoded["method"] == "tools/call"
    assert decoded["params"]["name"] == "haft_query"
    assert decoded["params"]["arguments"]["action"] == "status"
  end

  test "decode_response/1 returns :haft_unavailable on error envelope" do
    line = Jason.encode!(%{"id" => 5, "error" => %{"message" => "down"}})
    assert {:error, :haft_unavailable} = Protocol.decode_response(line)
  end

  test "decode_response/1 returns :response_parse_error on garbage" do
    assert {:error, :response_parse_error} = Protocol.decode_response("{junk")
    assert {:error, :response_parse_error} = Protocol.decode_response(Jason.encode!(%{}))
  end

  test "valid_tools/0 lists the 6 canonical MCP tools" do
    assert length(Protocol.valid_tools()) == 6
    assert :haft_problem in Protocol.valid_tools()
    assert :haft_decision in Protocol.valid_tools()
  end
end
