defmodule OpenSleigh.RuntimeStatusWriterTest do
  use ExUnit.Case, async: false

  alias OpenSleigh.{ObservationsBus, RuntimeStatusWriter}

  defmodule StatusServer do
    use GenServer

    def start_link(state), do: GenServer.start_link(__MODULE__, state)

    @impl true
    def init(state), do: {:ok, state}

    @impl true
    def handle_call(:status, _from, state) do
      {:reply, state.status, state}
    end

    def handle_call(:pending_human_gates, _from, state) do
      {:reply, state.human_gates, state}
    end
  end

  setup do
    ObservationsBus.reset()

    root =
      System.tmp_dir!()
      |> Path.join("open-sleigh-runtime-status-" <> unique_suffix())

    File.rm_rf!(root)

    on_exit(fn ->
      ObservationsBus.reset()
      File.rm_rf!(root)
    end)

    %{root: root}
  end

  test "commission-first metadata scopes path and snapshot by commission_id", ctx do
    status_root = Path.join(ctx.root, "status")

    {:ok, orchestrator} =
      StatusServer.start_link(%{
        status: %{claimed: [], running: [], pending_human: ["session-1"], retries: %{}},
        human_gates: [
          %{
            ticket_id: "OCT-HG",
            commission_id: "wc/local-7",
            session_id: "session-1",
            gate_name: "one_way_door_approved"
          }
        ]
      })

    :ok =
      ObservationsBus.emit(:dispatch_failed, :no_upstream_frame, %{
        commission_id: "wc/local-7",
        ticket: "OCT-HG",
        session_id: "session-1"
      })

    {:ok, _writer} =
      RuntimeStatusWriter.start_link(
        orchestrator: orchestrator,
        path: status_root,
        metadata: %{commission_id: "wc/local-7", ticket_id: "OCT-HG"},
        interval_ms: 0
      )

    status_path = Path.join(status_root, "wc_local-7.json")
    status = read_status(status_path)

    assert status["commission_id"] == "wc/local-7"
    assert status["legacy_ticket_id"] == "OCT-HG"
    assert get_in(status, ["human_gates", Access.at(0), "commission_id"]) == "wc/local-7"
    assert get_in(status, ["human_gates", Access.at(0), "legacy_ticket_id"]) == "OCT-HG"
    assert get_in(status, ["failures", Access.at(0), "commission_id"]) == "wc/local-7"
    assert get_in(status, ["failures", Access.at(0), "legacy_ticket_id"]) == "OCT-HG"
    assert get_in(status, ["failures", Access.at(0), "ticket"]) == "OCT-HG"
  end

  test "legacy metadata keeps ticket_id scoped path and snapshot", ctx do
    status_root = Path.join(ctx.root, "status")

    {:ok, orchestrator} =
      StatusServer.start_link(%{
        status: %{claimed: ["OCT-2"], running: [], pending_human: [], retries: %{}},
        human_gates: []
      })

    {:ok, _writer} =
      RuntimeStatusWriter.start_link(
        orchestrator: orchestrator,
        path: status_root,
        metadata: %{ticket_id: "OCT-2"},
        interval_ms: 0
      )

    status_path = Path.join(status_root, "OCT-2.json")
    status = read_status(status_path)

    assert status["ticket_id"] == "OCT-2"
    refute Map.has_key?(status, "commission_id")
  end

  defp read_status(path) do
    path
    |> File.read!()
    |> Jason.decode!()
  end

  defp unique_suffix do
    [:positive, :monotonic]
    |> System.unique_integer()
    |> Integer.to_string()
  end
end
