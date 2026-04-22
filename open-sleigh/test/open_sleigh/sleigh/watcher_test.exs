defmodule OpenSleigh.Sleigh.WatcherTest do
  use ExUnit.Case, async: false

  alias OpenSleigh.{WorkflowStore, Sleigh.Watcher}

  setup do
    workspace = tmp_workspace()
    path = Path.join(workspace, "sleigh.md")
    on_exit(fn -> File.rm_rf!(workspace) end)

    %{path: path}
  end

  test "loads sleigh.md into WorkflowStore and hot-reloads changed prompts", ctx do
    File.write!(ctx.path, source_with_execute_prompt("Execute v1 {{ticket.title}}"))

    store = server_name("WorkflowStoreWatcher")
    watcher_name = server_name("SleighWatcher")

    {:ok, _store} = WorkflowStore.start_link(name: store)

    {:ok, watcher} =
      Watcher.start_link(
        path: ctx.path,
        workflow_store: store,
        poll_interval_ms: 0,
        name: watcher_name
      )

    assert :ok = wait_until(fn -> loaded?(store, "Execute v1") end, 500)

    {:ok, first_hash} = WorkflowStore.config_hash_for(store, :execute)

    File.write!(ctx.path, source_with_execute_prompt("Execute v2 {{ticket.title}}"))
    send(watcher, :poll)

    assert :ok = wait_until(fn -> loaded?(store, "Execute v2") end, 500)

    {:ok, second_hash} = WorkflowStore.config_hash_for(store, :execute)

    assert first_hash != second_hash
    assert %{loaded_count: 2, last_error: nil} = Watcher.status(watcher_name)
  end

  defp loaded?(store, expected) do
    case WorkflowStore.prompt_for(store, :execute) do
      {:ok, prompt} -> String.contains?(prompt, expected)
      {:error, :unknown_phase} -> false
    end
  end

  defp source_with_execute_prompt(prompt) do
    source = File.read!("sleigh.md.example")

    Regex.replace(
      ~r/## Execute\n.*?\n\n## Measure/s,
      source,
      "## Execute\n#{prompt}\n\n## Measure"
    )
  end

  defp wait_until(check_fun, timeout_ms) do
    deadline = System.monotonic_time(:millisecond) + timeout_ms
    do_wait_until(check_fun, deadline)
  end

  defp do_wait_until(check_fun, deadline) do
    cond do
      check_fun.() ->
        :ok

      System.monotonic_time(:millisecond) > deadline ->
        {:error, :timeout}

      true ->
        Process.sleep(10)
        do_wait_until(check_fun, deadline)
    end
  end

  defp tmp_workspace do
    System.tmp_dir!()
    |> Path.join("open_sleigh_sleigh_watcher_#{System.unique_integer([:positive])}")
    |> tap(&File.mkdir_p!/1)
  end

  defp server_name(prefix) do
    String.to_atom("#{prefix}_#{:erlang.unique_integer([:positive])}")
  end
end
