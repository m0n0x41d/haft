defmodule OpenSleigh.Haft.Wal do
  @moduledoc """
  Per-ticket JSONL write-ahead log for Haft requests.

  WAL entries store the exact JSON-RPC request line that failed to
  reach Haft. Replay sends those same lines back through the supplied
  invoker and removes a file only after every entry in that file
  succeeds.
  """

  @type invoker :: (binary() -> {:ok, binary()} | {:error, atom()})

  @doc "Append a failed Haft request to its per-ticket WAL file."
  @spec append(Path.t(), binary(), integer()) :: :ok | {:error, atom()}
  def append(wal_dir, request_line, now_ms)
      when is_binary(wal_dir) and is_binary(request_line) and is_integer(now_ms) do
    with {:ok, ticket_id} <- ticket_id(request_line),
         :ok <- File.mkdir_p(wal_dir),
         path <- wal_path(wal_dir, ticket_id),
         entry <- encode_entry(ticket_id, request_line, now_ms),
         :ok <- File.write(path, entry, [:append]) do
      :ok
    else
      {:error, _reason} -> {:error, :haft_unavailable}
    end
  end

  @doc "Replay all WAL files in ticket-file arrival order."
  @spec replay(Path.t(), invoker()) :: :ok | {:error, atom()}
  def replay(wal_dir, invoker) when is_binary(wal_dir) and is_function(invoker, 1) do
    wal_dir
    |> wal_files()
    |> Enum.reduce_while(:ok, &replay_file(&1, &2, invoker))
  end

  @spec wal_files(Path.t()) :: [Path.t()]
  defp wal_files(wal_dir) do
    wal_dir
    |> Path.join("*.jsonl")
    |> Path.wildcard()
    |> Enum.sort_by(&mtime_sort_key/1)
  end

  @spec mtime_sort_key(Path.t()) :: integer()
  defp mtime_sort_key(path) do
    case File.stat(path, time: :posix) do
      {:ok, stat} -> stat.mtime
      {:error, _reason} -> 0
    end
  end

  @spec replay_file(Path.t(), :ok, invoker()) :: {:cont, :ok} | {:halt, {:error, atom()}}
  defp replay_file(path, :ok, invoker) do
    case do_replay_file(path, invoker) do
      :ok ->
        :ok = File.rm(path)
        {:cont, :ok}

      {:error, reason} ->
        {:halt, {:error, reason}}
    end
  end

  @spec do_replay_file(Path.t(), invoker()) :: :ok | {:error, atom()}
  defp do_replay_file(path, invoker) do
    path
    |> File.stream!()
    |> Enum.reduce_while(:ok, &replay_line(&1, &2, invoker))
  rescue
    _ -> {:error, :haft_unavailable}
  end

  @spec replay_line(binary(), :ok, invoker()) :: {:cont, :ok} | {:halt, {:error, atom()}}
  defp replay_line(line, :ok, invoker) do
    with {:ok, request_line} <- decode_request(line),
         {:ok, _response_line} <- invoker.(request_line) do
      {:cont, :ok}
    else
      {:error, reason} -> {:halt, {:error, reason}}
      _ -> {:halt, {:error, :haft_unavailable}}
    end
  end

  @spec decode_request(binary()) :: {:ok, binary()} | {:error, atom()}
  defp decode_request(line) do
    case Jason.decode(line) do
      {:ok, %{"request" => request}} when is_binary(request) -> {:ok, request}
      _ -> {:error, :haft_unavailable}
    end
  end

  @spec encode_entry(String.t(), binary(), integer()) :: binary()
  defp encode_entry(ticket_id, request_line, now_ms) do
    %{
      "ticket_id" => ticket_id,
      "request" => request_line,
      "appended_at_unix_ms" => now_ms
    }
    |> Jason.encode!()
    |> Kernel.<>("\n")
  end

  @spec ticket_id(binary()) :: {:ok, String.t()} | {:error, atom()}
  defp ticket_id(request_line) do
    case Jason.decode(request_line) do
      {:ok, %{"params" => %{"arguments" => %{"ticket_id" => ticket_id}}}}
      when is_binary(ticket_id) and ticket_id != "" ->
        {:ok, ticket_id}

      _ ->
        {:error, :missing_ticket_id}
    end
  end

  @spec wal_path(Path.t(), String.t()) :: Path.t()
  defp wal_path(wal_dir, ticket_id) do
    safe_ticket_id = String.replace(ticket_id, ~r/[^A-Za-z0-9._-]/, "_")
    Path.join(wal_dir, safe_ticket_id <> ".jsonl")
  end
end
