defmodule OpenSleigh.StatusHTTP do
  @moduledoc """
  Pure request handler for the local read-only status API and dashboard.

  The handler reads only the runtime status snapshot. It redacts secret-like
  keys and artifact-body carriers before returning JSON or HTML.
  """

  @secret_keys MapSet.new([
                 "api_key",
                 "apikey",
                 "authorization",
                 "token",
                 "access_token",
                 "secret",
                 "password",
                 "artifact_body",
                 "artifactBody",
                 "haft_artifact_body"
               ])

  @type response :: %{
          required(:status) => non_neg_integer(),
          required(:content_type) => String.t(),
          required(:body) => String.t()
        }

  @doc "Handle a read-only HTTP request."
  @spec handle(String.t(), String.t(), Path.t()) :: response()
  def handle("GET", "/api/v1/state", status_path) do
    status_path
    |> read_snapshot()
    |> state_response()
  end

  def handle("GET", "/", status_path) do
    status_path
    |> read_snapshot()
    |> dashboard_response()
  end

  def handle("GET", "/dashboard", status_path) do
    status_path
    |> read_snapshot()
    |> dashboard_response()
  end

  def handle("GET", "/healthz", _status_path) do
    %{
      status: 200,
      content_type: "application/json",
      body: Jason.encode!(%{"ok" => true})
    }
  end

  def handle("GET", "/favicon.ico", _status_path) do
    %{
      status: 200,
      content_type: "image/svg+xml",
      body:
        ~s(<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16"><rect width="16" height="16" fill="#202124"/></svg>)
    }
  end

  def handle(_method, _path, _status_path) do
    %{
      status: 404,
      content_type: "application/json",
      body: Jason.encode!(%{"error" => "not_found"})
    }
  end

  @doc "Recursively remove secret-like and Haft artifact body keys."
  @spec redact(term()) :: term()
  def redact(%{} = map) do
    map
    |> Enum.reject(&redacted_key?/1)
    |> Enum.map(&redact_pair/1)
    |> Map.new()
  end

  def redact(list) when is_list(list), do: Enum.map(list, &redact/1)
  def redact(value), do: value

  @spec read_snapshot(Path.t()) :: {:ok, map()} | {:error, atom()}
  defp read_snapshot(path) do
    path
    |> File.read()
    |> decode_snapshot()
  end

  @spec decode_snapshot({:ok, binary()} | {:error, term()}) :: {:ok, map()} | {:error, atom()}
  defp decode_snapshot({:ok, encoded}) do
    case Jason.decode(encoded) do
      {:ok, decoded} when is_map(decoded) -> {:ok, redact(decoded)}
      _ -> {:error, :status_malformed}
    end
  end

  defp decode_snapshot({:error, _reason}), do: {:error, :status_missing}

  @spec state_response({:ok, map()} | {:error, atom()}) :: response()
  defp state_response({:ok, snapshot}) do
    %{
      status: 200,
      content_type: "application/json",
      body: Jason.encode!(snapshot)
    }
  end

  defp state_response({:error, reason}) do
    %{
      status: 503,
      content_type: "application/json",
      body: Jason.encode!(%{"error" => Atom.to_string(reason)})
    }
  end

  @spec dashboard_response({:ok, map()} | {:error, atom()}) :: response()
  defp dashboard_response({:ok, snapshot}) do
    %{
      status: 200,
      content_type: "text/html; charset=utf-8",
      body: dashboard_html(snapshot)
    }
  end

  defp dashboard_response({:error, reason}) do
    %{
      status: 503,
      content_type: "text/html; charset=utf-8",
      body: dashboard_html(%{"error" => Atom.to_string(reason)})
    }
  end

  @spec dashboard_html(map()) :: String.t()
  defp dashboard_html(snapshot) do
    encoded = Jason.encode!(snapshot, pretty: true)
    summary = dashboard_summary(snapshot)

    """
    <!doctype html>
    <html lang="en">
    <head>
      <meta charset="utf-8">
      <meta name="viewport" content="width=device-width, initial-scale=1">
      <title>Open-Sleigh Status</title>
      <style>
        :root { color-scheme: light; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; }
        body { margin: 0; background: #f7f7f4; color: #202124; }
        main { max-width: 1120px; margin: 0 auto; padding: 28px; }
        header { display: flex; justify-content: space-between; gap: 16px; align-items: end; border-bottom: 1px solid #d7d7cf; padding-bottom: 18px; }
        h1 { margin: 0; font-size: 28px; font-weight: 700; }
        .updated { color: #5f6368; font-size: 14px; }
        .grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 12px; margin: 20px 0; }
        .card { background: #ffffff; border: 1px solid #deded6; border-radius: 8px; padding: 14px; }
        .label { color: #5f6368; font-size: 12px; text-transform: uppercase; letter-spacing: 0; }
        .value { font-size: 24px; font-weight: 700; margin-top: 6px; }
        pre { overflow: auto; background: #202124; color: #f1f3f4; border-radius: 8px; padding: 16px; line-height: 1.45; }
        @media (max-width: 760px) { main { padding: 18px; } header { display: block; } .grid { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
      </style>
    </head>
    <body>
      <main>
        <header>
          <h1>Open-Sleigh Status</h1>
          <div class="updated">#{html_escape(summary.updated_at)}</div>
        </header>
        <section class="grid">
          <div class="card"><div class="label">Claimed</div><div class="value">#{summary.claimed}</div></div>
          <div class="card"><div class="label">Running</div><div class="value">#{summary.running}</div></div>
          <div class="card"><div class="label">Human Gates</div><div class="value">#{summary.human_gates}</div></div>
          <div class="card"><div class="label">Failures</div><div class="value">#{summary.failures}</div></div>
        </section>
        <pre>#{html_escape(encoded)}</pre>
      </main>
    </body>
    </html>
    """
    |> String.trim()
  end

  @spec dashboard_summary(map()) :: map()
  defp dashboard_summary(snapshot) do
    orchestrator = Map.get(snapshot, "orchestrator", %{})

    %{
      updated_at: Map.get(snapshot, "updated_at", "not written yet"),
      claimed: list_count(orchestrator, "claimed"),
      running: list_count(orchestrator, "running"),
      human_gates: list_count(snapshot, "human_gates"),
      failures: list_count(snapshot, "failures")
    }
  end

  @spec list_count(map(), String.t()) :: non_neg_integer()
  defp list_count(map, key) do
    map
    |> Map.get(key, [])
    |> count_list()
  end

  @spec count_list(term()) :: non_neg_integer()
  defp count_list(list) when is_list(list), do: length(list)
  defp count_list(_value), do: 0

  @spec redacted_key?({term(), term()}) :: boolean()
  defp redacted_key?({key, _value}) do
    key
    |> key_string()
    |> sensitive_key?()
  end

  @spec redact_pair({term(), term()}) :: {term(), term()}
  defp redact_pair({key, value}), do: {key, redact(value)}

  @spec sensitive_key?(String.t()) :: boolean()
  defp sensitive_key?(key) do
    key
    |> String.downcase()
    |> then(&MapSet.member?(@secret_keys, &1))
  end

  @spec key_string(term()) :: String.t()
  defp key_string(key) when is_binary(key), do: key
  defp key_string(key) when is_atom(key), do: Atom.to_string(key)
  defp key_string(key), do: to_string(key)

  @spec html_escape(String.t()) :: String.t()
  defp html_escape(value) do
    value
    |> String.replace("&", "&amp;")
    |> String.replace("<", "&lt;")
    |> String.replace(">", "&gt;")
    |> String.replace("\"", "&quot;")
  end
end
