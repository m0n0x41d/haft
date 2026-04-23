defmodule OpenSleigh.Adapter.PathGuardTest do
  use ExUnit.Case, async: false

  alias OpenSleigh.Adapter.PathGuard

  setup do
    # Use a unique tmp subdirectory per test so parallel test runs
    # don't interfere. `async: false` above is belt-and-braces.
    tmp =
      Path.join(
        System.tmp_dir!(),
        "open_sleigh_path_guard_#{:erlang.unique_integer([:positive, :monotonic])}"
      )

    File.mkdir_p!(tmp)
    forbidden = Path.join(tmp, "forbidden")
    File.mkdir_p!(forbidden)

    workspace = Path.join(tmp, "workspace")
    File.mkdir_p!(workspace)

    on_exit(fn -> File.rm_rf!(tmp) end)

    config = %{
      forbidden_paths: [forbidden],
      forbidden_remote_substrings: ["open-sleigh"]
    }

    %{tmp: tmp, forbidden: forbidden, workspace: workspace, config: config}
  end

  describe "canonical/2 — CL5 traversal" do
    test "absolute path inside workspace is accepted", ctx do
      target = Path.join(ctx.workspace, "file.txt")
      assert {:ok, resolved} = PathGuard.canonical(target, ctx.config)
      assert resolved == target
    end

    test "..-traversal that resolves into forbidden tree is rejected", ctx do
      traversal = Path.join(ctx.workspace, "../forbidden/evil.txt")

      assert {:error, :path_outside_workspace} =
               PathGuard.canonical(traversal, ctx.config)
    end
  end

  describe "canonical/2 — CL6 symlink escape" do
    test "symlink pointing to a forbidden directory is rejected", ctx do
      link = Path.join(ctx.workspace, "link_to_forbidden")
      :ok = File.ln_s(ctx.forbidden, link)

      assert {:error, :path_outside_workspace} = PathGuard.canonical(link, ctx.config)
    end

    test "chain of symlinks that eventually resolves OK is accepted", ctx do
      real = Path.join(ctx.workspace, "real.txt")
      File.write!(real, "x")

      link1 = Path.join(ctx.workspace, "link1")
      link2 = Path.join(ctx.workspace, "link2")
      :ok = File.ln_s(real, link1)
      :ok = File.ln_s(link1, link2)

      assert {:ok, _resolved} = PathGuard.canonical(link2, ctx.config)
    end
  end

  describe "canonical/2 — CL8 symlink loop" do
    test "symlink loop is rejected at depth 8", ctx do
      a = Path.join(ctx.workspace, "a")
      b = Path.join(ctx.workspace, "b")
      :ok = File.ln_s(b, a)
      :ok = File.ln_s(a, b)

      result = PathGuard.canonical(a, ctx.config)
      # Accept either symlink_loop (our max-depth) or symlink_escape
      # depending on how the OS reports the read_link on a cycle.
      assert {:error, reason} = result
      assert reason in [:path_symlink_loop, :path_symlink_escape]
    end
  end

  describe "canonical/2 — CL9 workspace is a clone of self" do
    test "workspace containing .git/config with forbidden-remote substring is rejected",
         ctx do
      git_dir = Path.join(ctx.workspace, ".git")
      File.mkdir_p!(git_dir)

      File.write!(Path.join(git_dir, "config"), """
      [remote "origin"]
        url = https://github.com/m0n0x41d/open-sleigh.git
      """)

      assert {:error, :workspace_is_self} =
               PathGuard.canonical(ctx.workspace, ctx.config)
    end

    test "workspace with unrelated remote is accepted", ctx do
      git_dir = Path.join(ctx.workspace, ".git")
      File.mkdir_p!(git_dir)

      File.write!(Path.join(git_dir, "config"), """
      [remote "origin"]
        url = https://github.com/some/other-project.git
      """)

      assert {:ok, _} = PathGuard.canonical(ctx.workspace, ctx.config)
    end

    test "workspace without .git is accepted", ctx do
      assert {:ok, _} = PathGuard.canonical(ctx.workspace, ctx.config)
    end
  end

  describe "canonical/2 — non-existent paths" do
    test "non-existent path that would resolve inside workspace is accepted (future write target)",
         ctx do
      future = Path.join(ctx.workspace, "does/not/exist/yet.txt")
      assert {:ok, _} = PathGuard.canonical(future, ctx.config)
    end
  end

  describe "relative_to_workspace/3" do
    test "returns a workspace-relative path for targets inside workspace", ctx do
      target = Path.join(ctx.workspace, "lib/b.ex")

      assert {:ok, "lib/b.ex"} =
               PathGuard.relative_to_workspace(ctx.workspace, target, ctx.config)
    end

    test "rejects targets outside workspace without consulting commission scope", ctx do
      target = Path.join(ctx.tmp, "outside.ex")

      assert {:error, :path_outside_workspace} =
               PathGuard.relative_to_workspace(ctx.workspace, target, ctx.config)
    end
  end

  describe "canonical/2 — invalid input" do
    test "non-binary path → :invalid_workspace_cwd" do
      config = %{forbidden_paths: [], forbidden_remote_substrings: []}
      assert {:error, :invalid_workspace_cwd} = PathGuard.canonical(nil, config)
      assert {:error, :invalid_workspace_cwd} = PathGuard.canonical(42, config)
    end
  end
end
