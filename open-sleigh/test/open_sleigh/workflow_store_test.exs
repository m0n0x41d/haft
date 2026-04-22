defmodule OpenSleigh.WorkflowStoreTest do
  use ExUnit.Case, async: true

  alias OpenSleigh.{Fixtures, WorkflowStore}

  setup do
    frame_cfg = Fixtures.phase_config_frame()
    execute_cfg = Fixtures.phase_config_execute()
    measure_cfg = Fixtures.phase_config_measure()

    name = String.to_atom("WorkflowStore_#{:erlang.unique_integer([:positive])}")

    {:ok, _pid} =
      WorkflowStore.start_link(
        phase_configs: %{frame: frame_cfg, execute: execute_cfg, measure: measure_cfg},
        prompts: %{
          frame: "Frame prompt",
          execute: "Execute prompt",
          measure: "Measure prompt"
        },
        config_hashes: %{
          frame: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
          execute: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
        },
        external_publication: %{branch_regex: "^(main|master)$"},
        name: name
      )

    %{store: name}
  end

  test "phase_config/2 resolves known phases", ctx do
    assert {:ok, %{phase: :frame}} = WorkflowStore.phase_config(ctx.store, :frame)
    assert {:ok, %{phase: :execute}} = WorkflowStore.phase_config(ctx.store, :execute)
  end

  test "phase_config/2 unknown phase returns :unknown_phase", ctx do
    assert {:error, :unknown_phase} = WorkflowStore.phase_config(ctx.store, :problematize)
  end

  test "prompt_for/2", ctx do
    assert {:ok, "Frame prompt"} = WorkflowStore.prompt_for(ctx.store, :frame)
    assert {:error, :unknown_phase} = WorkflowStore.prompt_for(ctx.store, :unknown)
  end

  test "config_hash_for/2", ctx do
    assert {:ok, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"} =
             WorkflowStore.config_hash_for(ctx.store, :frame)

    assert {:error, :unknown_phase} = WorkflowStore.config_hash_for(ctx.store, :measure)
  end

  test "external_publication/1", ctx do
    assert %{branch_regex: "^(main|master)$"} =
             WorkflowStore.external_publication(ctx.store)
  end

  test "put_compiled/2 hot-reloads atomically", ctx do
    new_prompts = %{frame: "Frame v2", execute: "Exec v2", measure: "Measure v2"}
    new_configs = %{frame: Fixtures.phase_config_frame()}

    assert :ok =
             WorkflowStore.put_compiled(ctx.store, %{
               phase_configs: new_configs,
               prompts: new_prompts,
               config_hashes: %{
                 frame: "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
               }
             })

    assert {:ok, "Frame v2"} = WorkflowStore.prompt_for(ctx.store, :frame)

    assert {:ok, "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"} =
             WorkflowStore.config_hash_for(ctx.store, :frame)

    # Old execute phase_config is gone after hot-reload.
    assert {:error, :unknown_phase} = WorkflowStore.phase_config(ctx.store, :execute)
  end
end
