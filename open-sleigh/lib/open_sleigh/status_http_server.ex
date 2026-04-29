defmodule OpenSleigh.StatusHTTPServer do
  @moduledoc """
  Minimal local HTTP server for `OpenSleigh.StatusHTTP`.

  This server is intentionally read-only and dependency-free. It binds to a
  local interface, accepts simple HTTP/1.1 GET requests, delegates response
  construction to the pure handler, and never receives tracker credentials.
  """

  use GenServer

  alias OpenSleigh.StatusHTTP

  @type opts :: [
          status_path: Path.t(),
          host: :inet.ip_address(),
          port: :inet.port_number(),
          name: GenServer.name()
        ]

  @doc "Start the local status HTTP server."
  @spec start_link(opts()) :: GenServer.on_start()
  def start_link(opts) do
    GenServer.start_link(__MODULE__, opts, name: Keyword.get(opts, :name))
  end

  @doc "Return the bound TCP port."
  @spec port(GenServer.server()) :: non_neg_integer()
  def port(server), do: GenServer.call(server, :port)

  @impl true
  def init(opts) do
    host = Keyword.get(opts, :host, {127, 0, 0, 1})
    port = Keyword.get(opts, :port, 0)
    status_path = Keyword.fetch!(opts, :status_path)

    with {:ok, listener} <- listen(host, port),
         {:ok, actual_port} <- :inet.port(listener) do
      task = Task.async(fn -> accept_loop(listener, status_path) end)

      {:ok,
       %{
         listener: listener,
         accept_task: task,
         status_path: status_path,
         port: actual_port
       }}
    end
  end

  @impl true
  def handle_call(:port, _from, state) do
    {:reply, state.port, state}
  end

  @impl true
  def terminate(_reason, state) do
    :gen_tcp.close(state.listener)
    Task.shutdown(state.accept_task, :brutal_kill)
    :ok
  end

  @spec listen(:inet.ip_address(), :inet.port_number()) ::
          {:ok, :gen_tcp.socket()} | {:error, term()}
  defp listen(host, port) do
    :gen_tcp.listen(port, [
      :binary,
      active: false,
      ip: host,
      packet: :raw,
      reuseaddr: true
    ])
  end

  @spec accept_loop(:gen_tcp.socket(), Path.t()) :: :ok
  defp accept_loop(listener, status_path) do
    case :gen_tcp.accept(listener) do
      {:ok, socket} ->
        Task.start(fn -> handle_socket(socket, status_path) end)
        accept_loop(listener, status_path)

      {:error, :closed} ->
        :ok

      {:error, _reason} ->
        accept_loop(listener, status_path)
    end
  end

  @spec handle_socket(:gen_tcp.socket(), Path.t()) :: :ok
  defp handle_socket(socket, status_path) do
    socket
    |> :gen_tcp.recv(0, 2_000)
    |> request_response(status_path)
    |> send_response(socket)
  end

  @spec request_response({:ok, binary()} | {:error, term()}, Path.t()) :: StatusHTTP.response()
  defp request_response({:ok, request}, status_path) do
    request
    |> request_line()
    |> route(status_path)
  end

  defp request_response({:error, _reason}, _status_path) do
    %{
      status: 400,
      content_type: "application/json",
      body: Jason.encode!(%{"error" => "bad_request"})
    }
  end

  @spec request_line(binary()) :: {String.t(), String.t()}
  defp request_line(request) do
    request
    |> String.split("\r\n", parts: 2)
    |> List.first()
    |> request_parts()
  end

  @spec request_parts(String.t() | nil) :: {String.t(), String.t()}
  defp request_parts(nil), do: {"", ""}

  defp request_parts(line) do
    line
    |> String.split(" ", parts: 3)
    |> method_path()
  end

  @spec method_path([String.t()]) :: {String.t(), String.t()}
  defp method_path([method, path, _version]), do: {method, path}
  defp method_path(_parts), do: {"", ""}

  @spec route({String.t(), String.t()}, Path.t()) :: StatusHTTP.response()
  defp route({method, path}, status_path), do: StatusHTTP.handle(method, path, status_path)

  @spec send_response(StatusHTTP.response(), :gen_tcp.socket()) :: :ok
  defp send_response(response, socket) do
    :ok =
      :gen_tcp.send(socket, [
        "HTTP/1.1 ",
        Integer.to_string(response.status),
        " ",
        reason_phrase(response.status),
        "\r\ncontent-type: ",
        response.content_type,
        "\r\ncontent-length: ",
        response.body |> byte_size() |> Integer.to_string(),
        "\r\nconnection: close\r\n\r\n",
        response.body
      ])

    :gen_tcp.close(socket)
  end

  @spec reason_phrase(non_neg_integer()) :: String.t()
  defp reason_phrase(200), do: "OK"
  defp reason_phrase(400), do: "Bad Request"
  defp reason_phrase(404), do: "Not Found"
  defp reason_phrase(503), do: "Service Unavailable"
  defp reason_phrase(_status), do: "OK"
end
