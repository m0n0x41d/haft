defmodule OpenSleigh.HaftSupervisor do
  @moduledoc """
  Supervises the `HaftServer` + WAL replayer.

  For MVP-1 skeleton, this supervisor hosts a single `HaftServer`
  with an injected `invoke_fun`. The real `haft serve` Port-owning
  impl lands later.
  """

  use Supervisor

  alias OpenSleigh.HaftServer

  @spec start_link(keyword()) :: Supervisor.on_start()
  def start_link(opts) do
    Supervisor.start_link(__MODULE__, opts, name: Keyword.get(opts, :name, __MODULE__))
  end

  @impl true
  def init(opts) do
    children = [
      {HaftServer, server_opts(opts)}
    ]

    Supervisor.init(children, strategy: :one_for_one)
  end

  @spec server_opts(keyword()) :: keyword()
  defp server_opts(opts) do
    opts
    |> mode_opts()
    |> Keyword.merge(
      wal_dir: Keyword.get(opts, :wal_dir),
      name: Keyword.get(opts, :server_name, HaftServer)
    )
  end

  @spec mode_opts(keyword()) :: keyword()
  defp mode_opts(opts) do
    if Keyword.has_key?(opts, :invoke_fun) do
      [invoke_fun: Keyword.fetch!(opts, :invoke_fun)]
    else
      [
        command: Keyword.get(opts, :command, "haft serve"),
        project_root: Keyword.get(opts, :project_root, File.cwd!())
      ]
    end
  end
end
