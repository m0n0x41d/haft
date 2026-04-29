defmodule OpenSleigh.Notifications.LocalLog do
  @moduledoc """
  Local JSONL notification adapter.

  This adapter is the MVP-1 concrete notification sink: it is durable enough
  for local operation and exercises the same notification port future team
  channel adapters must implement.
  """

  @behaviour OpenSleigh.Notifications.Adapter

  alias OpenSleigh.Notifications.Adapter

  @type handle :: %{required(:path) => Path.t()}

  @doc "Build a local-log notification handle."
  @spec handle(Path.t()) :: handle()
  def handle(path) when is_binary(path), do: %{path: path}

  @impl true
  @spec notify(handle(), Adapter.notification()) :: :ok | {:error, atom()}
  def notify(%{path: path}, notification) do
    if Adapter.valid?(notification) do
      notification
      |> encode_line()
      |> append_line(path)
    else
      {:error, :invalid_notification}
    end
  end

  @spec encode_line(Adapter.notification()) :: binary()
  defp encode_line(notification) do
    %{
      "kind" => Atom.to_string(notification.kind),
      "message" => notification.message,
      "metadata" => notification.metadata,
      "at" => DateTime.utc_now() |> DateTime.to_iso8601()
    }
    |> Jason.encode!()
    |> Kernel.<>("\n")
  end

  @spec append_line(binary(), Path.t()) :: :ok | {:error, atom()}
  defp append_line(line, path) do
    path
    |> Path.dirname()
    |> File.mkdir_p()
    |> append_line_after_mkdir(line, path)
  end

  @spec append_line_after_mkdir(:ok | {:error, term()}, binary(), Path.t()) ::
          :ok | {:error, atom()}
  defp append_line_after_mkdir(:ok, line, path) do
    path
    |> File.write(line, [:append])
    |> write_result()
  end

  defp append_line_after_mkdir({:error, _reason}, _line, _path),
    do: {:error, :notification_failed}

  @spec write_result(:ok | {:error, term()}) :: :ok | {:error, atom()}
  defp write_result(:ok), do: :ok
  defp write_result({:error, _reason}), do: {:error, :notification_failed}
end
