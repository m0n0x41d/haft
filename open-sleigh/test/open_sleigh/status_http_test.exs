defmodule OpenSleigh.StatusHTTPTest do
  use ExUnit.Case, async: true

  alias OpenSleigh.{StatusHTTP, StatusHTTPServer}

  test "state endpoint mirrors status JSON and redacts secrets/artifact bodies" do
    path = tmp_status_path()

    File.write!(
      path,
      Jason.encode!(%{
        "updated_at" => "2026-04-22T00:00:00Z",
        "api_key" => "secret",
        "orchestrator" => %{"claimed" => ["OCT-1"], "running" => []},
        "haft" => %{"artifact_body" => "large private body"},
        "failures" => []
      })
    )

    response = StatusHTTP.handle("GET", "/api/v1/state", path)

    assert response.status == 200
    assert response.content_type == "application/json"
    assert {:ok, body} = Jason.decode(response.body)
    refute Map.has_key?(body, "api_key")
    refute Map.has_key?(body["haft"], "artifact_body")
    assert get_in(body, ["orchestrator", "claimed"]) == ["OCT-1"]
  end

  test "dashboard endpoint renders redacted HTML summary" do
    path = tmp_status_path()

    File.write!(
      path,
      Jason.encode!(%{
        "updated_at" => "2026-04-22T00:00:00Z",
        "token" => "secret",
        "orchestrator" => %{"claimed" => ["OCT-1"], "running" => ["OCT-2"]},
        "human_gates" => [%{"ticket_id" => "OCT-3"}],
        "failures" => [%{"reason" => "blocked"}]
      })
    )

    response = StatusHTTP.handle("GET", "/dashboard", path)

    assert response.status == 200
    assert response.content_type =~ "text/html"
    assert response.body =~ "Open-Sleigh Status"
    assert response.body =~ "<div class=\"value\">1</div>"
    refute response.body =~ "secret"
  end

  test "server responds over local HTTP" do
    path = tmp_status_path()
    File.write!(path, Jason.encode!(%{"updated_at" => "2026-04-22T00:00:00Z"}))

    {:ok, server} = StatusHTTPServer.start_link(status_path: path, port: 0)
    port = StatusHTTPServer.port(server)

    {:ok, socket} = :gen_tcp.connect({127, 0, 0, 1}, port, [:binary, active: false])
    :ok = :gen_tcp.send(socket, "GET /healthz HTTP/1.1\r\nhost: localhost\r\n\r\n")
    {:ok, response} = :gen_tcp.recv(socket, 0, 2_000)

    assert response =~ "HTTP/1.1 200 OK"
    assert response =~ ~s("ok":true)

    GenServer.stop(server)
  end

  defp tmp_status_path do
    dir =
      System.tmp_dir!()
      |> Path.join("open_sleigh_status_http_#{System.unique_integer([:positive])}")
      |> tap(&File.mkdir_p!/1)

    on_exit(fn -> File.rm_rf!(dir) end)
    Path.join(dir, "status.json")
  end
end
