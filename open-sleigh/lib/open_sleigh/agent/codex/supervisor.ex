defmodule OpenSleigh.Agent.Codex.Supervisor do
  @moduledoc """
  Per-session supervisor for a Codex app-server Port owner.

  The supervised child is `OpenSleigh.Agent.Codex.Server`, which owns
  the subprocess Port. The supervisor is intentionally per-session so
  a dead app-server is isolated to one `(Ticket × Phase)` session.
  """

  use Supervisor

  alias OpenSleigh.EffectError
  alias OpenSleigh.Agent.Codex.Server

  @type handle :: %{required(:server) => pid(), required(:supervisor) => pid()}

  @spec start_session(keyword()) :: {:ok, handle()} | {:error, EffectError.t()}
  def start_session(opts) when is_list(opts) do
    opts
    |> start_link()
    |> server_handle()
  end

  @spec stop_session(handle()) :: :ok
  def stop_session(%{server: server, supervisor: supervisor}) do
    _ = Server.close(server)
    _ = Supervisor.stop(supervisor)
    :ok
  end

  @spec start_link(keyword()) :: Supervisor.on_start()
  def start_link(opts) when is_list(opts) do
    Supervisor.start_link(__MODULE__, opts)
  end

  @impl true
  def init(opts) do
    children = [
      %{
        id: Server,
        start: {Server, :start_link, [opts]},
        restart: :temporary,
        shutdown: 5_000,
        type: :worker
      }
    ]

    Supervisor.init(children, strategy: :one_for_one)
  end

  @spec server_handle(Supervisor.on_start()) :: {:ok, handle()} | {:error, EffectError.t()}
  defp server_handle({:ok, supervisor}) do
    case Supervisor.which_children(supervisor) do
      [{Server, server, :worker, [Server]}] when is_pid(server) ->
        {:ok, %{supervisor: supervisor, server: server}}

      _ ->
        _ = Supervisor.stop(supervisor)
        {:error, :agent_launch_failed}
    end
  end

  defp server_handle({:error, _reason}), do: {:error, :agent_launch_failed}
end
