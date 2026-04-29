defmodule OpenSleigh.WorkspaceManagerTest do
  use ExUnit.Case, async: false

  alias OpenSleigh.WorkspaceManager

  setup do
    tmp =
      Path.join(
        System.tmp_dir!(),
        "open_sleigh_wm_#{:erlang.unique_integer([:positive, :monotonic])}"
      )

    File.mkdir_p!(tmp)
    on_exit(fn -> File.rm_rf!(tmp) end)

    %{root: tmp, guard: %{forbidden_paths: [], forbidden_remote_substrings: ["open-sleigh"]}}
  end

  describe "sanitize/1" do
    test "passes through safe identifiers" do
      assert "OCT-123" == WorkspaceManager.sanitize("OCT-123")
      assert "abc_DEF.v1" == WorkspaceManager.sanitize("abc_DEF.v1")
    end

    test "replaces unsafe chars with _" do
      assert "OCT_123_with_space" == WorkspaceManager.sanitize("OCT/123 with space")
      assert "a_b_c" == WorkspaceManager.sanitize("a*b*c")
    end
  end

  describe "create_for_ticket/3" do
    test "creates a new workspace directory", ctx do
      assert {:ok, path, :new} =
               WorkspaceManager.create_for_ticket(ctx.root, "OCT-1", ctx.guard)

      assert File.dir?(path)
      assert Path.basename(path) == "OCT-1"
    end

    test "reuses an existing workspace", ctx do
      {:ok, path, :new} = WorkspaceManager.create_for_ticket(ctx.root, "OCT-2", ctx.guard)
      File.write!(Path.join(path, "marker"), "x")

      assert {:ok, ^path, :reused} =
               WorkspaceManager.create_for_ticket(ctx.root, "OCT-2", ctx.guard)

      assert File.exists?(Path.join(path, "marker"))
    end

    test "sanitises identifier when building path", ctx do
      assert {:ok, path, :new} =
               WorkspaceManager.create_for_ticket(ctx.root, "OCT/slash", ctx.guard)

      assert Path.basename(path) == "OCT_slash"
    end
  end

  describe "run_hook/3" do
    test "returns :ok on zero-exit script", ctx do
      {:ok, ws, _} = WorkspaceManager.create_for_ticket(ctx.root, "OCT-3", ctx.guard)
      assert :ok = WorkspaceManager.run_hook(ws, "exit 0", 5_000)
    end

    test "returns :hook_failed on non-zero exit", ctx do
      {:ok, ws, _} = WorkspaceManager.create_for_ticket(ctx.root, "OCT-4", ctx.guard)

      assert {:error, :hook_failed} =
               WorkspaceManager.run_hook(ws, "exit 1", 5_000)
    end

    test "returns :hook_timeout on slow script", ctx do
      {:ok, ws, _} = WorkspaceManager.create_for_ticket(ctx.root, "OCT-5", ctx.guard)

      assert {:error, :hook_timeout} =
               WorkspaceManager.run_hook(ws, "sleep 2", 50)
    end
  end

  describe "cleanup/1" do
    test "removes workspace dir", ctx do
      {:ok, ws, _} = WorkspaceManager.create_for_ticket(ctx.root, "OCT-6", ctx.guard)
      assert File.dir?(ws)
      assert :ok = WorkspaceManager.cleanup(ws)
      refute File.exists?(ws)
    end

    test "no-op on non-existent path", ctx do
      assert :ok = WorkspaceManager.cleanup(Path.join(ctx.root, "does-not-exist"))
    end
  end
end
