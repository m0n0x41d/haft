defmodule OpenSleigh.Haft.Mock do
  @moduledoc """
  In-memory Haft for tests. Implements the `invoke_fun` signature
  used by `OpenSleigh.Haft.Client`, echoing well-formed responses
  without spawning `haft serve`.

  Uses an Agent for artifact storage so tests can inspect what was
  "written" to Haft after a call.
  """

  alias OpenSleigh.EffectError

  @doc "Start a fresh mock Haft."
  @spec start() :: {:ok, pid()}
  def start do
    Agent.start_link(fn -> %{artifacts: [], next_id: 1} end)
  end

  @doc """
  Build an `invoke_fun` compatible with `Haft.Client.call_tool/5`
  and `write_artifact/3` — echoes back a success response with the
  same id.
  """
  @spec invoke_fun(pid()) ::
          (binary() -> {:ok, binary()} | {:error, EffectError.t()})
  def invoke_fun(handle) do
    fn request_line -> handle_request(handle, request_line) end
  end

  @spec handle_request(pid(), binary()) ::
          {:ok, binary()} | {:error, EffectError.t()}
  defp handle_request(handle, request_line) do
    with {:ok, %{"id" => id, "params" => params}} <- Jason.decode(request_line) do
      :ok = record_artifact(handle, params)
      {:ok, ok_response(id)}
    else
      _ -> {:error, :response_parse_error}
    end
  end

  @spec record_artifact(pid(), map()) :: :ok
  defp record_artifact(handle, params) do
    Agent.update(handle, fn state ->
      %{state | artifacts: [params | state.artifacts]}
    end)
  end

  @spec ok_response(integer()) :: binary()
  defp ok_response(id) do
    body =
      Jason.encode!(%{
        "jsonrpc" => "2.0",
        "id" => id,
        "result" => %{"artifact_id" => "mock-haft-" <> Integer.to_string(id)}
      })

    body <> "\n"
  end

  @doc "Read back all artifacts the mock received, oldest-first."
  @spec artifacts(pid()) :: [map()]
  def artifacts(handle), do: handle |> Agent.get(& &1.artifacts) |> Enum.reverse()
end
