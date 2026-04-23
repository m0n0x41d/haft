defmodule OpenSleigh.MixTasksTest do
  use ExUnit.Case, async: false

  setup do
    Mix.shell(Mix.Shell.Process)
    OpenSleigh.ObservationsBus.reset()

    on_exit(fn ->
      Mix.shell(Mix.Shell.IO)
      Mix.Task.reenable("open_sleigh.doctor")
      Mix.Task.reenable("open_sleigh.status")
      Mix.Task.reenable("open_sleigh.start")
      Mix.Task.reenable("open_sleigh.canary")
      Mix.Task.reenable("open_sleigh.gate_report")
    end)

    :ok
  end

  test "open_sleigh.start prints help" do
    Mix.Task.run("open_sleigh.start", ["--help"])

    assert_receive {:mix_shell, :info, [help]}, 500
    assert String.contains?(help, "mix open_sleigh.start")
    assert String.contains?(help, "--path")
    assert String.contains?(help, "--mock-haft")
  end

  test "open_sleigh.canary prints help" do
    Mix.Task.run("open_sleigh.canary", ["--help"])

    assert_receive {:mix_shell, :info, [help]}, 500
    assert String.contains?(help, "mix open_sleigh.canary")
    assert String.contains?(help, "--duration")
  end

  test "open_sleigh.doctor prints help" do
    Mix.Task.run("open_sleigh.doctor", ["--help"])

    assert_receive {:mix_shell, :info, [help]}, 500
    assert String.contains?(help, "mix open_sleigh.doctor")
    assert String.contains?(help, "--json")
    assert String.contains?(help, "--mock-haft")
  end

  test "open_sleigh.status prints help" do
    Mix.Task.run("open_sleigh.status", ["--help"])

    assert_receive {:mix_shell, :info, [help]}, 500
    assert String.contains?(help, "mix open_sleigh.status")
    assert String.contains?(help, "--json")
  end

  test "open_sleigh.gate_report prints help" do
    Mix.Task.run("open_sleigh.gate_report", ["--help"])

    assert_receive {:mix_shell, :info, [help]}, 500
    assert String.contains?(help, "mix open_sleigh.gate_report")
    assert String.contains?(help, "--live")
  end

  test "open_sleigh.gate_report emits a passing JSON report" do
    Mix.Task.run("open_sleigh.gate_report", ["--json"])

    assert_receive {:mix_shell, :info, [encoded]}, 500
    assert {:ok, report} = Jason.decode(encoded)
    assert report["total"] == 6
    assert report["failed"] == 0
    assert Enum.all?(report["rows"], &(&1["status"] == "pass"))
  end

  test "open_sleigh.status text shows recent failures" do
    path = tmp_sleigh_path()
    status_path = Path.join(Path.dirname(path), "status.json")

    status = %{
      "updated_at" => "2000-01-01T00:00:00Z",
      "metadata" => %{"config_path" => "test/sleigh.md"},
      "orchestrator" => %{
        "claimed" => [],
        "running" => [],
        "pending_human" => ["OCT-HG"],
        "retries" => %{}
      },
      "human_gates" => [
        %{
          "ticket_id" => "OCT-HG",
          "session_id" => "session-1",
          "gate_name" => "commission_approved",
          "requested_at" => "2026-04-22T10:00:00Z"
        }
      ],
      "failures" => [
        %{
          "metric" => "dispatch_failed",
          "reason" => "no_upstream_frame",
          "ticket" => "OCT-1"
        }
      ],
      "observations" => []
    }

    File.write!(status_path, Jason.encode!(status))
    on_exit(fn -> File.rm_rf!(Path.dirname(path)) end)

    Mix.Task.run("open_sleigh.status", ["--path", status_path])

    assert_receive {:mix_shell, :info, ["stale: true"]}, 500
    assert_receive {:mix_shell, :info, ["pending_human: 1"]}, 500
    assert_receive {:mix_shell, :info, ["pending_human_details:"]}, 500

    assert_receive {:mix_shell, :info,
                    [
                      "- ticket=OCT-HG session_id=session-1 gate=commission_approved requested_at=2026-04-22T10:00:00Z"
                    ]},
                   500

    assert_receive {:mix_shell, :info, ["failures: 1"]}, 500
    assert_receive {:mix_shell, :info, ["recent_failures:"]}, 500

    assert_receive {:mix_shell, :info,
                    ["- dispatch_failed reason=no_upstream_frame ticket=OCT-1"]},
                   500
  end

  test "open_sleigh.doctor fails fast when real runtime env is missing" do
    old_key = System.get_env("LINEAR_API_KEY")
    old_repo = System.get_env("REPO_URL")
    System.delete_env("LINEAR_API_KEY")
    System.delete_env("REPO_URL")

    path = tmp_sleigh_path()

    File.write!(path, sleigh_example_source(path))

    on_exit(fn ->
      restore_env("LINEAR_API_KEY", old_key)
      restore_env("REPO_URL", old_repo)
      File.rm_rf!(Path.dirname(path))
    end)

    assert_raise Mix.Error, ~r/Open-Sleigh doctor failed:/, fn ->
      Mix.Task.run("open_sleigh.doctor", ["--path", path])
    end

    assert_receive {:mix_shell, :info, ["error linear.api_key: LINEAR_API_KEY is missing"]}, 500
    assert_receive {:mix_shell, :info, ["error hooks.repo_url: REPO_URL is missing"]}, 500
  end

  test "open_sleigh.doctor passes with required env and executables" do
    old_key = System.get_env("LINEAR_API_KEY")
    old_repo = System.get_env("REPO_URL")
    old_path = System.get_env("PATH")

    path = tmp_sleigh_path()
    File.write!(path, sleigh_example_source(path))
    bin_dir = fake_bin_dir(["codex", "haft", "git"])

    System.put_env("LINEAR_API_KEY", "lin-test")
    System.put_env("REPO_URL", "git@example.com:oct/repo.git")
    System.put_env("PATH", bin_dir <> ":" <> old_path)

    on_exit(fn ->
      restore_env("LINEAR_API_KEY", old_key)
      restore_env("REPO_URL", old_repo)
      restore_env("PATH", old_path)
      File.rm_rf!(Path.dirname(path))
      File.rm_rf!(bin_dir)
    end)

    Mix.Task.run("open_sleigh.doctor", ["--path", path, "--json"])

    assert_receive {:mix_shell, :info, [encoded]}, 500
    assert {:ok, report} = Jason.decode(encoded)
    assert report["ready"] == true
    assert report["errors"] == 0
    assert has_check?(report, "workspace.root", "ok")
    assert has_check?(report, "workspace.cleanup_policy", "ok")
    assert has_check?(report, "hooks.repo_url_format", "ok")
    assert has_check?(report, "hooks.failure_policy", "ok")
    assert has_check?(report, "external_publication.branch_regex", "ok")
    assert has_check?(report, "gates.semantic", "ok")
  end

  test "open_sleigh.doctor passes local commission source without tracker or Haft env" do
    old_key = System.get_env("LINEAR_API_KEY")
    old_repo = System.get_env("REPO_URL")
    old_path = System.get_env("PATH")

    path = tmp_sleigh_path()
    bin_dir = fake_bin_dir(["codex"])

    source =
      "sleigh.commission.md.example"
      |> File.read!()
      |> String.replace(
        "fixture_path: fixtures/commissions/local_bootstrap.yaml",
        "fixture_path: #{Path.expand("fixtures/commissions/local_bootstrap.yaml")}"
      )
      |> String.replace("root: ~/.open-sleigh/workspaces", "root: #{test_workspace_root(path)}")
      |> String.replace(
        "  after_create: |\n    git clone --depth 1 $REPO_URL .\n    mix deps.get || true",
        "  after_create: null"
      )
      |> String.replace(
        "  before_run: |\n    git pull --ff-only origin main || true",
        "  before_run: null"
      )

    File.write!(path, source)

    System.delete_env("LINEAR_API_KEY")
    System.delete_env("REPO_URL")
    System.put_env("PATH", bin_dir <> ":" <> old_path)

    on_exit(fn ->
      restore_env("LINEAR_API_KEY", old_key)
      restore_env("REPO_URL", old_repo)
      restore_env("PATH", old_path)
      File.rm_rf!(Path.dirname(path))
      File.rm_rf!(bin_dir)
    end)

    Mix.Task.run("open_sleigh.doctor", ["--path", path, "--mock-haft", "--json"])

    assert_receive {:mix_shell, :info, [encoded]}, 500
    assert {:ok, report} = Jason.decode(encoded)
    assert report["ready"] == true
    assert has_check?(report, "commission_source.fixture_path", "ok")
    assert has_check?(report, "haft.command", "ok")
  end

  test "open_sleigh.doctor passes Haft commission source without tracker env" do
    old_key = System.get_env("LINEAR_API_KEY")
    old_repo = System.get_env("REPO_URL")
    old_path = System.get_env("PATH")

    path = tmp_sleigh_path()
    bin_dir = fake_bin_dir(["codex"])

    source =
      "sleigh.commission.md.example"
      |> File.read!()
      |> String.replace("kind: local", "kind: haft")
      |> String.replace("root: ~/.open-sleigh/workspaces", "root: #{test_workspace_root(path)}")
      |> String.replace(
        "  after_create: |\n    git clone --depth 1 $REPO_URL .\n    mix deps.get || true",
        "  after_create: null"
      )
      |> String.replace(
        "  before_run: |\n    git pull --ff-only origin main || true",
        "  before_run: null"
      )

    File.write!(path, source)

    System.delete_env("LINEAR_API_KEY")
    System.delete_env("REPO_URL")
    System.put_env("PATH", bin_dir <> ":" <> old_path)

    on_exit(fn ->
      restore_env("LINEAR_API_KEY", old_key)
      restore_env("REPO_URL", old_repo)
      restore_env("PATH", old_path)
      File.rm_rf!(Path.dirname(path))
      File.rm_rf!(bin_dir)
    end)

    Mix.Task.run("open_sleigh.doctor", ["--path", path, "--mock-haft", "--json"])

    assert_receive {:mix_shell, :info, [encoded]}, 500
    assert {:ok, report} = Jason.decode(encoded)
    assert report["ready"] == true
    assert has_check?(report, "commission_source.selector", "ok")
    assert has_check?(report, "haft.command", "ok")
  end

  test "open_sleigh.doctor JSON reports schema field path and fix hint" do
    path = tmp_sleigh_path()

    source =
      path
      |> sleigh_example_source()
      |> String.replace("object_of_talk_is_specific", "object_gate_that_does_not_exist")

    File.write!(path, source)

    on_exit(fn -> File.rm_rf!(Path.dirname(path)) end)

    assert_raise Mix.Error, ~r/Open-Sleigh doctor failed:/, fn ->
      Mix.Task.run("open_sleigh.doctor", ["--path", path, "--json"])
    end

    assert_receive {:mix_shell, :info, [encoded]}, 500
    assert {:ok, report} = Jason.decode(encoded)

    schema_check =
      report["checks"]
      |> Enum.find(&(&1["name"] == "config.schema"))

    assert schema_check["field_path"] == "phases.*.gates"
    assert schema_check["expected"] =~ "registered"
    assert schema_check["actual"] == "object_gate_that_does_not_exist"
    assert schema_check["hint"] =~ "Registry"
  end

  test "open_sleigh.doctor rejects invalid hook failure policy" do
    old_key = System.get_env("LINEAR_API_KEY")
    old_repo = System.get_env("REPO_URL")
    old_path = System.get_env("PATH")

    path = tmp_sleigh_path()

    source =
      path
      |> sleigh_example_source()
      |> String.replace("before_run: blocking", "before_run: explode")

    File.write!(path, source)
    bin_dir = fake_bin_dir(["codex", "haft", "git"])

    System.put_env("LINEAR_API_KEY", "lin-test")
    System.put_env("REPO_URL", "git@example.com:oct/repo.git")
    System.put_env("PATH", bin_dir <> ":" <> old_path)

    on_exit(fn ->
      restore_env("LINEAR_API_KEY", old_key)
      restore_env("REPO_URL", old_repo)
      restore_env("PATH", old_path)
      File.rm_rf!(Path.dirname(path))
      File.rm_rf!(bin_dir)
    end)

    assert_raise Mix.Error, ~r/Open-Sleigh doctor failed:/, fn ->
      Mix.Task.run("open_sleigh.doctor", ["--path", path])
    end

    assert_receive {:mix_shell, :info,
                    ["error hooks.failure_policy: Invalid hook failure policy: " <> _message]},
                   500
  end

  test "open_sleigh.doctor rejects malformed repository URL" do
    old_key = System.get_env("LINEAR_API_KEY")
    old_repo = System.get_env("REPO_URL")
    old_path = System.get_env("PATH")

    path = tmp_sleigh_path()
    File.write!(path, sleigh_example_source(path))
    bin_dir = fake_bin_dir(["codex", "haft", "git"])

    System.put_env("LINEAR_API_KEY", "lin-test")
    System.put_env("REPO_URL", "not a git remote")
    System.put_env("PATH", bin_dir <> ":" <> old_path)

    on_exit(fn ->
      restore_env("LINEAR_API_KEY", old_key)
      restore_env("REPO_URL", old_repo)
      restore_env("PATH", old_path)
      File.rm_rf!(Path.dirname(path))
      File.rm_rf!(bin_dir)
    end)

    assert_raise Mix.Error, ~r/Open-Sleigh doctor failed:/, fn ->
      Mix.Task.run("open_sleigh.doctor", ["--path", path])
    end

    assert_receive {:mix_shell, :info,
                    [
                      "error hooks.repo_url_format: REPO_URL must be an SSH/HTTPS/file git remote or local repository path"
                    ]},
                   500
  end

  test "open_sleigh.doctor rejects an unwritable workspace root" do
    old_key = System.get_env("LINEAR_API_KEY")
    old_repo = System.get_env("REPO_URL")
    old_path = System.get_env("PATH")

    path = tmp_sleigh_path()
    workspace_file = Path.join(Path.dirname(path), "workspace-file")

    source =
      path
      |> sleigh_example_source()
      |> String.replace("root: #{test_workspace_root(path)}", "root: #{workspace_file}")

    File.write!(path, source)
    File.write!(workspace_file, "not a directory")
    bin_dir = fake_bin_dir(["codex", "haft", "git"])

    System.put_env("LINEAR_API_KEY", "lin-test")
    System.put_env("REPO_URL", "git@example.com:oct/repo.git")
    System.put_env("PATH", bin_dir <> ":" <> old_path)

    on_exit(fn ->
      restore_env("LINEAR_API_KEY", old_key)
      restore_env("REPO_URL", old_repo)
      restore_env("PATH", old_path)
      File.rm_rf!(Path.dirname(path))
      File.rm_rf!(bin_dir)
    end)

    assert_raise Mix.Error, ~r/Open-Sleigh doctor failed:/, fn ->
      Mix.Task.run("open_sleigh.doctor", ["--path", path])
    end

    assert_receive {:mix_shell, :info, ["error workspace.root: " <> _message]}, 500
  end

  test "open_sleigh.doctor rejects destructive workspace cleanup policy" do
    old_key = System.get_env("LINEAR_API_KEY")
    old_repo = System.get_env("REPO_URL")
    old_path = System.get_env("PATH")

    path = tmp_sleigh_path()

    source =
      path
      |> sleigh_example_source()
      |> String.replace("cleanup_policy: keep", "cleanup_policy: delete_terminal")

    File.write!(path, source)
    bin_dir = fake_bin_dir(["codex", "haft", "git"])

    System.put_env("LINEAR_API_KEY", "lin-test")
    System.put_env("REPO_URL", "git@example.com:oct/repo.git")
    System.put_env("PATH", bin_dir <> ":" <> old_path)

    on_exit(fn ->
      restore_env("LINEAR_API_KEY", old_key)
      restore_env("REPO_URL", old_repo)
      restore_env("PATH", old_path)
      File.rm_rf!(Path.dirname(path))
      File.rm_rf!(bin_dir)
    end)

    assert_raise Mix.Error, ~r/Open-Sleigh doctor failed:/, fn ->
      Mix.Task.run("open_sleigh.doctor", ["--path", path])
    end

    assert_receive {:mix_shell, :info,
                    [
                      "error workspace.cleanup_policy: Only non-destructive workspace cleanup policy is supported now: keep, got \"delete_terminal\""
                    ]},
                   500
  end

  test "open_sleigh.start boots a minimal sleigh.md in mock once mode" do
    old_status_path = System.get_env("OPEN_SLEIGH_STATUS_PATH")
    old_log_path = System.get_env("OPEN_SLEIGH_LOG_PATH")
    path = tmp_sleigh_path()
    status_path = Path.join(Path.dirname(path), "status.json")
    log_path = Path.join(Path.dirname(path), "runtime.jsonl")

    System.put_env("OPEN_SLEIGH_STATUS_PATH", status_path)
    System.put_env("OPEN_SLEIGH_LOG_PATH", log_path)

    source =
      "sleigh.md.example"
      |> File.read!()
      |> String.replace("root: ~/.open-sleigh/workspaces", "root: #{test_workspace_root(path)}")
      |> String.replace("status_path: ~/.open-sleigh/status.json", "status_path: #{status_path}")
      |> String.replace("log_path: ~/.open-sleigh/runtime.jsonl", "log_path: #{log_path}")

    File.write!(path, source)

    on_exit(fn ->
      restore_env("OPEN_SLEIGH_STATUS_PATH", old_status_path)
      restore_env("OPEN_SLEIGH_LOG_PATH", old_log_path)
      File.rm_rf!(Path.dirname(path))
    end)

    Mix.Task.run("open_sleigh.start", ["--path", path, "--mock", "--once"])

    assert_receive {:mix_shell, :info, ["Open-Sleigh engine started"]}, 1_000
    assert_receive {:mix_shell, :info, ["Open-Sleigh status: " <> encoded]}, 1_000

    assert {:ok, status} = Jason.decode(encoded)
    assert status["claimed"] == []

    assert {:ok, stored_status} =
             status_path
             |> File.read!()
             |> Jason.decode()

    assert get_in(stored_status, ["metadata", "config_path"]) == path
    assert get_in(stored_status, ["metadata", "agent_kind"]) == "mock"
    assert get_in(stored_status, ["metadata", "tracker_kind"]) == "mock"
    assert get_in(stored_status, ["metadata", "workspace_cleanup_policy"]) == "keep"
    assert get_in(stored_status, ["orchestrator", "claimed"]) == []

    Mix.Task.run("open_sleigh.status", ["--path", status_path, "--json"])

    assert_receive {:mix_shell, :info, [stored_encoded]}, 500
    assert {:ok, %{"updated_at" => _updated_at}} = Jason.decode(stored_encoded)

    assert log_events =
             log_path
             |> File.read!()
             |> String.split("\n", trim: true)
             |> Enum.map(&Jason.decode!/1)

    assert Enum.any?(log_events, &(&1["event"] == "runtime_started"))
    assert Enum.any?(log_events, &(&1["event"] == "runtime_stopping"))
    assert Enum.all?(log_events, &is_binary(&1["event_id"]))
    assert Enum.all?(log_events, &(get_in(&1, ["metadata", "config_path"]) == path))
  end

  test "open_sleigh.start boots local commission source in mock once mode" do
    old_status_path = System.get_env("OPEN_SLEIGH_STATUS_PATH")
    old_log_path = System.get_env("OPEN_SLEIGH_LOG_PATH")
    path = tmp_sleigh_path()
    status_path = Path.join(Path.dirname(path), "commission-status.json")
    log_path = Path.join(Path.dirname(path), "commission-runtime.jsonl")

    System.put_env("OPEN_SLEIGH_STATUS_PATH", status_path)
    System.put_env("OPEN_SLEIGH_LOG_PATH", log_path)

    source =
      "sleigh.commission.md.example"
      |> File.read!()
      |> String.replace(
        "fixture_path: fixtures/commissions/local_bootstrap.yaml",
        "fixture_path: #{Path.expand("fixtures/commissions/local_bootstrap.yaml")}"
      )
      |> String.replace("root: ~/.open-sleigh/workspaces", "root: #{test_workspace_root(path)}")
      |> String.replace("status_path: ~/.open-sleigh/status.json", "status_path: #{status_path}")
      |> String.replace("log_path: ~/.open-sleigh/runtime.jsonl", "log_path: #{log_path}")
      |> String.replace(
        "  after_create: |\n    git clone --depth 1 $REPO_URL .\n    mix deps.get || true",
        "  after_create: null"
      )
      |> String.replace(
        "  before_run: |\n    git pull --ff-only origin main || true",
        "  before_run: null"
      )

    File.write!(path, source)

    on_exit(fn ->
      restore_env("OPEN_SLEIGH_STATUS_PATH", old_status_path)
      restore_env("OPEN_SLEIGH_LOG_PATH", old_log_path)
      File.rm_rf!(Path.dirname(path))
    end)

    Mix.Task.run("open_sleigh.start", ["--path", path, "--mock", "--once"])

    assert_receive {:mix_shell, :info, ["Open-Sleigh engine started"]}, 1_000
    assert_receive {:mix_shell, :info, ["Open-Sleigh status: " <> encoded]}, 1_000

    assert {:ok, status} = Jason.decode(encoded)
    assert is_list(status["claimed"])
    assert is_list(status["running"])

    assert {:ok, stored_status} =
             status_path
             |> File.read!()
             |> Jason.decode()

    assert get_in(stored_status, ["metadata", "tracker_kind"]) == "commission_source:local"
    assert get_in(stored_status, ["metadata", "agent_kind"]) == "mock"

    assert log_events =
             log_path
             |> File.read!()
             |> String.split("\n", trim: true)
             |> Enum.map(&Jason.decode!/1)

    assert Enum.any?(log_events, &(&1["event"] == "runtime_started"))
    assert Enum.any?(log_events, &(&1["event"] == "once_poll_completed"))
  end

  test "open_sleigh.start boots local source with independently mocked effects" do
    old_status_path = System.get_env("OPEN_SLEIGH_STATUS_PATH")
    old_log_path = System.get_env("OPEN_SLEIGH_LOG_PATH")
    path = tmp_sleigh_path()
    status_path = Path.join(Path.dirname(path), "split-status.json")
    log_path = Path.join(Path.dirname(path), "split-runtime.jsonl")

    System.put_env("OPEN_SLEIGH_STATUS_PATH", status_path)
    System.put_env("OPEN_SLEIGH_LOG_PATH", log_path)

    source =
      "sleigh.commission.md.example"
      |> File.read!()
      |> String.replace(
        "fixture_path: fixtures/commissions/local_bootstrap.yaml",
        "fixture_path: #{Path.expand("fixtures/commissions/local_bootstrap.yaml")}"
      )
      |> String.replace("root: ~/.open-sleigh/workspaces", "root: #{test_workspace_root(path)}")
      |> String.replace("status_path: ~/.open-sleigh/status.json", "status_path: #{status_path}")
      |> String.replace("log_path: ~/.open-sleigh/runtime.jsonl", "log_path: #{log_path}")
      |> String.replace(
        "  after_create: |\n    git clone --depth 1 $REPO_URL .\n    mix deps.get || true",
        "  after_create: null"
      )
      |> String.replace(
        "  before_run: |\n    git pull --ff-only origin main || true",
        "  before_run: null"
      )

    File.write!(path, source)

    on_exit(fn ->
      restore_env("OPEN_SLEIGH_STATUS_PATH", old_status_path)
      restore_env("OPEN_SLEIGH_LOG_PATH", old_log_path)
      File.rm_rf!(Path.dirname(path))
    end)

    Mix.Task.run(
      "open_sleigh.start",
      ["--path", path, "--mock-agent", "--mock-haft", "--mock-judge", "--once"]
    )

    assert_receive {:mix_shell, :info, ["Open-Sleigh engine started"]}, 1_000
    assert_receive {:mix_shell, :info, ["Open-Sleigh status: " <> _encoded]}, 1_000

    assert {:ok, stored_status} =
             status_path
             |> File.read!()
             |> Jason.decode()

    assert get_in(stored_status, ["metadata", "tracker_kind"]) == "commission_source:local"
    assert get_in(stored_status, ["metadata", "agent_kind"]) == "mock"
  end

  test "open_sleigh.start boots Haft commission source with independently mocked effects" do
    old_status_path = System.get_env("OPEN_SLEIGH_STATUS_PATH")
    old_log_path = System.get_env("OPEN_SLEIGH_LOG_PATH")
    path = tmp_sleigh_path()
    status_path = Path.join(Path.dirname(path), "haft-status.json")
    log_path = Path.join(Path.dirname(path), "haft-runtime.jsonl")

    System.put_env("OPEN_SLEIGH_STATUS_PATH", status_path)
    System.put_env("OPEN_SLEIGH_LOG_PATH", log_path)

    source =
      "sleigh.commission.md.example"
      |> File.read!()
      |> String.replace("kind: local", "kind: haft")
      |> String.replace("root: ~/.open-sleigh/workspaces", "root: #{test_workspace_root(path)}")
      |> String.replace("status_path: ~/.open-sleigh/status.json", "status_path: #{status_path}")
      |> String.replace("log_path: ~/.open-sleigh/runtime.jsonl", "log_path: #{log_path}")
      |> String.replace(
        "  after_create: |\n    git clone --depth 1 $REPO_URL .\n    mix deps.get || true",
        "  after_create: null"
      )
      |> String.replace(
        "  before_run: |\n    git pull --ff-only origin main || true",
        "  before_run: null"
      )

    File.write!(path, source)

    on_exit(fn ->
      restore_env("OPEN_SLEIGH_STATUS_PATH", old_status_path)
      restore_env("OPEN_SLEIGH_LOG_PATH", old_log_path)
      File.rm_rf!(Path.dirname(path))
    end)

    Mix.Task.run(
      "open_sleigh.start",
      ["--path", path, "--mock-agent", "--mock-haft", "--mock-judge", "--once"]
    )

    assert_receive {:mix_shell, :info, ["Open-Sleigh engine started"]}, 1_000
    assert_receive {:mix_shell, :info, ["Open-Sleigh status: " <> _encoded]}, 1_000

    assert {:ok, stored_status} =
             status_path
             |> File.read!()
             |> Jason.decode()

    assert get_in(stored_status, ["metadata", "tracker_kind"]) == "commission_source:haft"
    assert get_in(stored_status, ["metadata", "agent_kind"]) == "mock"
  end

  test "open_sleigh.start real mode fails fast when Linear API key is missing" do
    old_key = System.get_env("LINEAR_API_KEY")
    System.delete_env("LINEAR_API_KEY")

    path = tmp_sleigh_path()
    File.write!(path, sleigh_example_source(path))

    on_exit(fn ->
      restore_env("LINEAR_API_KEY", old_key)
      File.rm_rf!(Path.dirname(path))
    end)

    assert_raise Mix.Error, ~r/:missing_linear_api_key/, fn ->
      Mix.Task.run("open_sleigh.start", ["--path", path, "--once"])
    end
  end

  test "open_sleigh.canary runs the mock T3 HumanGate path" do
    Mix.Task.run("open_sleigh.canary", ["--duration", "0s"])

    assert_receive {:mix_shell, :info, ["Open-Sleigh canary passed: " <> encoded]}, 2_000

    assert {:ok, summary} = Jason.decode(encoded)
    assert summary["ticket_id"] == "CANARY-T3"
    assert summary["haft_artifacts"] == 3
    assert summary["coverage"] == ["t3_human_gate"]
  end

  defp tmp_sleigh_path do
    workspace =
      System.tmp_dir!()
      |> Path.join("open_sleigh_mix_task_#{System.unique_integer([:positive])}")
      |> tap(&File.mkdir_p!/1)

    Path.join(workspace, "sleigh.md")
  end

  defp fake_bin_dir(names) do
    bin_dir =
      System.tmp_dir!()
      |> Path.join("open_sleigh_fake_bin_#{System.unique_integer([:positive])}")
      |> tap(&File.mkdir_p!/1)

    Enum.each(names, &write_fake_executable(bin_dir, &1))

    bin_dir
  end

  defp write_fake_executable(bin_dir, name) do
    path = Path.join(bin_dir, name)
    File.write!(path, "#!/bin/sh\nexit 0\n")
    File.chmod!(path, 0o755)
  end

  defp restore_env(name, nil), do: System.delete_env(name)
  defp restore_env(name, value), do: System.put_env(name, value)

  defp sleigh_example_source(path) do
    "sleigh.md.example"
    |> File.read!()
    |> String.replace("root: ~/.open-sleigh/workspaces", "root: #{test_workspace_root(path)}")
  end

  defp test_workspace_root(path) do
    path
    |> Path.dirname()
    |> Path.join("workspaces")
  end

  defp has_check?(report, name, status) do
    report
    |> Map.fetch!("checks")
    |> Enum.any?(&(&1["name"] == name and &1["status"] == status))
  end
end
