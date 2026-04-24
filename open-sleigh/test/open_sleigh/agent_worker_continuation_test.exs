defmodule OpenSleigh.AgentWorkerContinuationTest do
  use ExUnit.Case, async: false

  alias OpenSleigh.Agent.Mock, as: AgentMock
  alias OpenSleigh.Agent.Adapter, as: AgentAdapter
  alias OpenSleigh.Haft.Mock, as: HaftMock
  alias OpenSleigh.Tracker.Mock, as: TrackerMock

  alias OpenSleigh.{
    AdapterSession,
    AgentWorker,
    ConfigHash,
    Fixtures,
    ObservationsBus,
    PhaseConfig,
    Scope,
    Session,
    SessionId,
    WorkCommission
  }

  defmodule ForwardingOrchestrator do
    @moduledoc false

    use GenServer

    @spec start_link(pid()) :: GenServer.on_start()
    def start_link(owner), do: GenServer.start_link(__MODULE__, owner)

    @impl true
    def init(owner), do: {:ok, owner}

    @impl true
    def handle_cast(message, owner) do
      send(owner, {:orchestrator_cast, message})
      {:noreply, owner}
    end
  end

  setup do
    ObservationsBus.reset()
    AgentMock.reset!()

    workspace_root =
      System.tmp_dir!()
      |> Path.join("agent_worker_continuation_#{System.unique_integer([:positive])}")

    File.mkdir_p!(workspace_root)
    on_exit(fn -> File.rm_rf!(workspace_root) end)

    {:ok, haft} = HaftMock.start()
    {:ok, tracker} = TrackerMock.start()
    {:ok, orchestrator} = ForwardingOrchestrator.start_link(self())

    ticket_attrs = %{
      id: "OCT-CONT",
      source: {:linear, "oct"},
      title: "Continuation ticket",
      body: "Exercise continuation turns",
      state: :in_progress,
      problem_card_ref: "haft-pc-continuation",
      target_branch: "feature/continuation",
      fetched_at: ~U[2026-04-22 10:00:00Z]
    }

    ticket = Fixtures.ticket(ticket_attrs)

    :ok = TrackerMock.seed(tracker, [ticket_attrs])

    %{
      haft: haft,
      orchestrator: orchestrator,
      ticket: ticket,
      tracker: tracker,
      workspace_root: workspace_root
    }
  end

  test "Frame sessions run exactly one turn even when gates fail", ctx do
    phase_config =
      Fixtures.phase_config_frame(%{
        gates: %{structural: [], semantic: [:object_of_talk_is_specific], human: []}
      })

    message =
      ctx
      |> worker_ctx(phase_config, fail_judge_fun())
      |> run_worker()

    assert {:outcome, _session_id, outcome} = message
    assert outcome.phase == :frame
    assert length(AgentMock.turn_prompts()) == 1
    assert AgentMock.start_count() == 1
  end

  test "Measure sessions run exactly one turn even when gates fail", ctx do
    phase_config =
      Fixtures.phase_config_measure(%{
        gates: %{structural: [], semantic: [:no_self_evidence_semantic], human: []}
      })

    AgentMock.put_turn_replies([
      %{
        events: [
          %{
            payload: %{
              "item" => %{
                "type" => "commandExecution",
                "status" => "completed",
                "command" => "go test ./internal/cli -run TestInit",
                "exitCode" => 0,
                "aggregatedOutput" => "ok  github.com/m0n0x41d/haft/internal/cli 0.123s\n"
              }
            }
          },
          %{
            payload: %{
              "item" => %{
                "id" => "msg-final",
                "type" => "agentMessage",
                "phase" => "final_answer",
                "text" => "The focused init test passed."
              }
            }
          }
        ]
      }
    ])

    message =
      ctx
      |> worker_ctx(phase_config, fail_judge_fun())
      |> run_worker()

    assert {:outcome, _session_id, outcome} = message
    assert outcome.phase == :measure
    assert length(AgentMock.turn_prompts()) == 1
    assert AgentMock.start_count() == 1
  end

  test "measure passes final claim text and command evidence into semantic gates", ctx do
    phase_config =
      Fixtures.phase_config_measure(%{
        gates: %{structural: [], semantic: [:no_self_evidence_semantic], human: []}
      })

    AgentMock.put_turn_replies([
      %{
        events: [
          %{
            payload: %{
              "item" => %{
                "id" => "msg-commentary",
                "type" => "agentMessage",
                "phase" => "commentary",
                "text" => "commentary text"
              }
            }
          },
          %{
            payload: %{
              "item" => %{
                "type" => "commandExecution",
                "status" => "completed",
                "command" => "go test ./internal/cli -run TestInit",
                "exitCode" => 0,
                "aggregatedOutput" => "ok  github.com/m0n0x41d/haft/internal/cli 0.123s\n"
              }
            }
          },
          %{
            payload: %{
              "item" => %{
                "id" => "msg-final",
                "type" => "agentMessage",
                "phase" => "final_answer",
                "text" =>
                  "The portability change is verified: the focused init test command passed and observed output was `ok`."
              }
            }
          }
        ]
      }
    ])

    judge_fun = capture_gate_context(self())

    message =
      ctx
      |> worker_ctx(phase_config, judge_fun)
      |> run_worker()

    assert {:outcome, _session_id, outcome} = message
    assert outcome.phase == :measure

    assert_receive {:gate_context, gate_context}, 1_000
    assert gate_context.turn_result.claim =~ "The portability change is verified"

    assert [
             %OpenSleigh.Evidence{
               kind: :external_measurement,
               authoring_source: :external,
               cl: 3,
               ref: ref
             }
           ] = gate_context.evidence

    assert ref =~ "command: go test ./internal/cli -run TestInit"
    assert ref =~ "exit_code: 0"
    assert ref =~ "ok  github.com/m0n0x41d/haft/internal/cli 0.123s"
  end

  test "Execute sessions run up to max_turns while gates do not pass", ctx do
    phase_config =
      Fixtures.phase_config_execute(%{
        gates: %{structural: [], semantic: [:lade_quadrants_split_ok], human: []},
        max_turns: 3
      })

    first_prompt = "Implement continuation behavior"

    message =
      ctx
      |> worker_ctx(phase_config, fail_judge_fun(), first_prompt)
      |> Map.put(:hooks, execute_seed_hooks())
      |> run_worker()

    prompts = AgentMock.turn_prompts()

    assert {:outcome, _session_id, outcome} = message
    assert outcome.phase == :execute
    assert length(prompts) == 3
    assert AgentMock.start_count() == 1
    assert Enum.at(prompts, 0) == first_prompt
    assert Enum.at(prompts, 1) =~ "Continuation guidance — Open-Sleigh Phase: execute"
    refute Enum.at(prompts, 1) =~ first_prompt
    assert Enum.at(prompts, 1) =~ "continuation turn #2 of 3"
    assert Enum.uniq(AgentMock.turn_scopes()) == [MapSet.new(phase_config.tools)]
  end

  test "Execute sessions stop when gates pass before max_turns", ctx do
    phase_config =
      Fixtures.phase_config_execute(%{
        gates: %{structural: [], semantic: [:lade_quadrants_split_ok], human: []},
        max_turns: 5
      })

    message =
      ctx
      |> worker_ctx(phase_config, pass_on_second_turn_judge_fun())
      |> Map.put(:hooks, execute_seed_hooks())
      |> run_worker()

    assert {:outcome, _session_id, outcome} = message
    assert outcome.phase == :execute
    assert length(AgentMock.turn_prompts()) == 2
    assert AgentMock.start_count() == 1
  end

  test "Execute sessions stop early when tracker state leaves active set", ctx do
    phase_config =
      Fixtures.phase_config_execute(%{
        gates: %{structural: [], semantic: [:lade_quadrants_split_ok], human: []},
        max_turns: 5
      })

    AgentMock.put_after_turn(fn
      1, _reply -> TrackerMock.transition(ctx.tracker, ctx.ticket.id, :done)
      _turn_number, _reply -> :ok
    end)

    message =
      ctx
      |> worker_ctx(phase_config, fail_judge_fun())
      |> Map.put(:hooks, execute_seed_hooks())
      |> run_worker()

    assert {:outcome, _session_id, outcome} = message
    assert outcome.phase == :execute
    assert length(AgentMock.turn_prompts()) == 1
    assert AgentMock.start_count() == 1
  end

  test "after_create hook failure blocks the session before agent start", ctx do
    phase_config =
      Fixtures.phase_config_frame(%{
        gates: %{structural: [], semantic: [], human: []}
      })

    message =
      ctx
      |> worker_ctx(phase_config, fail_judge_fun())
      |> Map.put(:hooks, %{after_create: "exit 1"})
      |> run_worker()

    assert {:error, _session_id, :hook_failed} = message
    assert AgentMock.start_count() == 0
  end

  test "warning hook policy records failure and continues", ctx do
    phase_config =
      Fixtures.phase_config_frame(%{
        gates: %{structural: [], semantic: [], human: []}
      })

    message =
      ctx
      |> worker_ctx(phase_config, fail_judge_fun())
      |> Map.put(:hooks, %{before_run: "exit 1"})
      |> Map.put(:hook_failure_policy, %{before_run: :warning})
      |> run_worker()

    assert {:outcome, _session_id, outcome} = message
    assert outcome.phase == :frame
    assert AgentMock.start_count() == 1

    assert Enum.any?(
             ObservationsBus.snapshot(),
             &(&1.metric == :hook_failed and &1.tags.hook == :before_run and
                 &1.tags.policy == :warning)
           )
  end

  test "Preflight gates use runtime commission facts instead of agent-authored facts", ctx do
    commission = preflight_commission!()
    ticket = commission_ticket!(commission)

    phase_config =
      preflight_phase_config()

    AgentMock.put_turn_replies([
      %{
        commission: nil,
        commission_snapshot: nil,
        current_snapshot: nil,
        current_decision: %{status: :superseded},
        checked_at: nil
      }
    ])

    message =
      ctx
      |> Map.put(:ticket, ticket)
      |> worker_ctx(phase_config, fail_judge_fun())
      |> run_worker()

    assert {:outcome, _session_id, outcome} = message
    assert outcome.phase == :preflight
    assert Enum.all?(outcome.gate_results, &match?({:structural, :ok}, &1))
  end

  test "terminal diff validation rejects out-of-scope git mutations", ctx do
    commission = preflight_commission!()
    ticket = commission_ticket!(commission)

    phase_config =
      Fixtures.phase_config_execute(%{
        gates: %{structural: [], semantic: [], human: []},
        max_turns: 1
      })

    message =
      ctx
      |> Map.put(:ticket, ticket)
      |> worker_ctx(phase_config, fail_judge_fun())
      |> Map.put(:hooks, %{after_create: "git init -q\nmkdir -p lib\ntouch lib/outside.ex"})
      |> run_worker()

    assert {:error, _session_id, :mutation_outside_commission_scope} = message
  end

  test "execute blocks when only runtime scratch paths changed", ctx do
    commission = preflight_commission!()
    ticket = commission_ticket!(commission)

    phase_config =
      Fixtures.phase_config_execute(%{
        gates: %{structural: [], semantic: [], human: []},
        max_turns: 1
      })

    message =
      ctx
      |> Map.put(:ticket, ticket)
      |> worker_ctx(phase_config, fail_judge_fun())
      |> Map.put(
        :hooks,
        %{
          after_create:
            "git init -q\n" <>
              "git config user.email test@example.com\n" <>
              "git config user.name 'Open Sleigh Test'\n" <>
              "mkdir -p lib/open_sleigh .tmp/gocache\n" <>
              "printf 'baseline\\n' > lib/open_sleigh/agent_worker.ex\n" <>
              "git add lib/open_sleigh/agent_worker.ex\n" <>
              "git commit -qm 'baseline'\n" <>
              "printf 'cache\\n' > .tmp/gocache/state"
        }
      )
      |> run_worker()

    assert {:error, _session_id, :no_commission_mutation} = message
  end

  test "reused preflight workspace resets stale git diff before validation", ctx do
    commission = preflight_commission!()
    ticket = commission_ticket!(commission)
    workspace_path = Path.join(ctx.workspace_root, ticket.id)

    :ok = seed_reused_git_workspace(workspace_path)

    message =
      ctx
      |> Map.put(:ticket, ticket)
      |> worker_ctx(preflight_phase_config(), fail_judge_fun())
      |> run_worker()

    assert {:outcome, _session_id, outcome} = message
    assert outcome.phase == :preflight
    assert workspace_git_status_lines(workspace_path) == []
  end

  test "reused non-preflight workspace keeps stale git diff for validation", ctx do
    commission = preflight_commission!()
    ticket = commission_ticket!(commission)
    workspace_path = Path.join(ctx.workspace_root, ticket.id)

    :ok = seed_reused_git_workspace(workspace_path)

    phase_config =
      Fixtures.phase_config_execute(%{
        gates: %{structural: [], semantic: [], human: []},
        max_turns: 1
      })

    message =
      ctx
      |> Map.put(:ticket, ticket)
      |> worker_ctx(phase_config, fail_judge_fun())
      |> run_worker()

    assert {:error, _session_id, :mutation_outside_commission_scope} = message
    assert workspace_git_status_lines(workspace_path) == [" M lib/outside.ex"]
  end

  @spec worker_ctx(map(), PhaseConfig.t(), OpenSleigh.GateChain.judge_fun()) :: AgentWorker.ctx()
  defp worker_ctx(ctx, phase_config, judge_fun) do
    worker_ctx(ctx, phase_config, judge_fun, "first-turn prompt")
  end

  @spec worker_ctx(map(), PhaseConfig.t(), OpenSleigh.GateChain.judge_fun(), String.t()) ::
          AgentWorker.ctx()
  defp worker_ctx(ctx, phase_config, judge_fun, prompt) do
    session_id = SessionId.generate()
    config_hash = ConfigHash.from_iodata("continuation-test")
    workspace_path = Path.join(ctx.workspace_root, ctx.ticket.id)
    scoped_tools = MapSet.new(phase_config.tools)

    {:ok, adapter_session} =
      AdapterSession.new(%{
        session_id: session_id,
        config_hash: config_hash,
        scoped_tools: scoped_tools,
        workspace_path: workspace_path,
        adapter_kind: :mock,
        adapter_version: "mvp1-test",
        max_turns: phase_config.max_turns,
        max_tokens_per_turn: 80_000,
        wall_clock_timeout_s: 600
      })

    adapter_session =
      ctx.ticket
      |> ticket_commission()
      |> maybe_attach_adapter_commission(adapter_session)

    {:ok, session} =
      Session.new(%{
        id: session_id,
        ticket: ctx.ticket,
        phase: phase_config.phase,
        config_hash: config_hash,
        scoped_tools: scoped_tools,
        workspace_path: workspace_path,
        claimed_at: ~U[2026-04-22 10:00:00Z],
        adapter_session: adapter_session
      })

    session =
      ctx.ticket
      |> ticket_commission()
      |> maybe_attach_session_commission(session)

    %{
      session: session,
      phase_config: phase_config,
      prompt: prompt,
      upstream_problem_card: nil,
      agent_adapter: AgentMock,
      judge_fun: judge_fun,
      haft_invoker: HaftMock.invoke_fun(ctx.haft),
      orchestrator: ctx.orchestrator,
      workspace_root: ctx.workspace_root,
      guard_config: %{forbidden_paths: [], forbidden_remote_substrings: ["open-sleigh"]},
      hooks: %{},
      hook_timeout_ms: 60_000,
      now_fun: fn -> ~U[2026-04-22 10:00:00Z] end,
      tracker_handle: ctx.tracker,
      tracker_adapter: TrackerMock,
      active_states: [:todo, :in_progress]
    }
  end

  @spec run_worker(AgentWorker.ctx()) :: term()
  defp run_worker(worker_context) do
    assert :ok = AgentWorker.run(worker_context)
    assert_receive {:orchestrator_cast, message}, 1_000
    refute_receive {:orchestrator_cast, _message}, 50
    message
  end

  @spec fail_judge_fun() :: OpenSleigh.GateChain.judge_fun()
  defp fail_judge_fun do
    fn _gate_module, _gate_context ->
      {:ok, %{verdict: :fail, cl: 3, rationale: "phase exit gates still fail"}}
    end
  end

  @spec pass_on_second_turn_judge_fun() :: OpenSleigh.GateChain.judge_fun()
  defp pass_on_second_turn_judge_fun do
    fn _gate_module, gate_context ->
      gate_context.turn_result
      |> Map.fetch!(:turn_id)
      |> pass_after_first_turn()
    end
  end

  @spec pass_after_first_turn(String.t()) :: {:ok, map()}
  defp pass_after_first_turn("mock-turn-1") do
    {:ok, %{verdict: :fail, cl: 3, rationale: "first turn incomplete"}}
  end

  defp pass_after_first_turn(_turn_id) do
    {:ok, %{verdict: :pass, cl: 3, rationale: "second turn passes"}}
  end

  @spec capture_gate_context(pid()) :: OpenSleigh.GateChain.judge_fun()
  defp capture_gate_context(owner) do
    fn _gate_module, gate_context ->
      send(owner, {:gate_context, gate_context})
      {:ok, %{verdict: :pass, cl: 3, rationale: "captured"}}
    end
  end

  @spec ticket_commission(OpenSleigh.Ticket.t()) :: WorkCommission.t() | nil
  defp ticket_commission(ticket) do
    ticket.metadata
    |> Map.get(:commission)
    |> ticket_commission_result()
  end

  @spec ticket_commission_result(term()) :: WorkCommission.t() | nil
  defp ticket_commission_result(%WorkCommission{} = commission), do: commission
  defp ticket_commission_result(_value), do: nil

  @spec maybe_attach_adapter_commission(WorkCommission.t() | nil, AdapterSession.t()) ::
          AdapterSession.t()
  defp maybe_attach_adapter_commission(nil, adapter_session), do: adapter_session

  defp maybe_attach_adapter_commission(%WorkCommission{} = commission, adapter_session) do
    adapter_session
    |> AgentAdapter.attach_commission_context(commission)
  end

  @spec maybe_attach_session_commission(WorkCommission.t() | nil, Session.t()) :: Session.t()
  defp maybe_attach_session_commission(nil, session), do: session

  defp maybe_attach_session_commission(%WorkCommission{} = commission, session) do
    session
    |> Map.put(:commission, commission)
    |> Map.put(:commission_id, commission.id)
    |> Map.put(:scope, commission.scope)
  end

  @spec preflight_phase_config() :: PhaseConfig.t()
  defp preflight_phase_config do
    Fixtures.phase_config_execute(%{
      phase: :preflight,
      agent_role: :preflight_checker,
      tools: [:read, :grep],
      gates: %{
        structural: [:commission_runnable, :decision_fresh, :scope_snapshot_fresh],
        semantic: [],
        human: []
      },
      prompt_template_key: :preflight,
      max_turns: 1,
      default_valid_until_days: 1
    })
  end

  @spec commission_ticket!(WorkCommission.t()) :: OpenSleigh.Ticket.t()
  defp commission_ticket!(commission) do
    Fixtures.ticket(%{
      id: commission.id,
      source: {:github, commission.scope.repo_ref},
      title: "Preflight commission",
      body: "",
      state: :in_progress,
      problem_card_ref: commission.problem_card_ref,
      target_branch: commission.scope.target_branch,
      fetched_at: commission.fetched_at,
      metadata: %{
        commission: commission,
        commission_id: commission.id,
        source_mode: :commission_first
      }
    })
  end

  @spec preflight_commission!() :: WorkCommission.t()
  defp preflight_commission! do
    scope = preflight_scope!()

    attrs = %{
      id: "wc-agent-worker-preflight",
      decision_ref: "dec-20260422-001",
      decision_revision_hash: "decision-r1",
      problem_card_ref: "pc-agent-worker-preflight",
      implementation_plan_ref: "plan-agent-worker-preflight",
      implementation_plan_revision: "plan-r1",
      scope: scope,
      scope_hash: scope.hash,
      base_sha: scope.base_sha,
      lockset: scope.lockset,
      evidence_requirements: [],
      projection_policy: :local_only,
      state: :preflighting,
      valid_until: ~U[2026-05-22 10:00:00Z],
      fetched_at: ~U[2026-04-22 10:00:00Z]
    }

    {:ok, commission} =
      attrs
      |> WorkCommission.new()

    commission
  end

  @spec preflight_scope!() :: Scope.t()
  defp preflight_scope! do
    attrs = %{
      repo_ref: "local:open-sleigh-preflight-test",
      base_sha: "base-r1",
      target_branch: "feature/preflight-runtime-facts",
      allowed_paths: ["lib/open_sleigh/agent_worker.ex"],
      forbidden_paths: [],
      allowed_actions: MapSet.new([:edit_files, :run_tests]),
      affected_files: ["lib/open_sleigh/agent_worker.ex"],
      allowed_modules: ["OpenSleigh.AgentWorker"],
      lockset: ["lib/open_sleigh/agent_worker.ex"]
    }

    {:ok, hash} =
      attrs
      |> Scope.canonical_hash()

    {:ok, scope} =
      attrs
      |> Map.put(:hash, hash)
      |> Scope.new()

    scope
  end

  @spec seed_reused_git_workspace(Path.t()) :: :ok
  defp seed_reused_git_workspace(workspace_path) do
    File.mkdir_p!(Path.join(workspace_path, "lib/open_sleigh"))
    File.mkdir_p!(Path.join(workspace_path, "lib"))

    script =
      """
      git init -q
      git config user.email test@example.com
      git config user.name "Open Sleigh Test"
      printf 'allowed\\n' > lib/open_sleigh/agent_worker.ex
      printf 'outside\\n' > lib/outside.ex
      git add lib/open_sleigh/agent_worker.ex lib/outside.ex
      git commit -qm 'baseline'
      printf 'dirty\\n' >> lib/outside.ex
      """
      |> String.trim()

    {"", 0} =
      System.cmd("bash", ["-lc", script],
        cd: workspace_path,
        stderr_to_stdout: true
      )

    :ok
  end

  @spec workspace_git_status_lines(Path.t()) :: [String.t()]
  defp workspace_git_status_lines(workspace_path) do
    {_output, 0} =
      System.cmd("git", ["-C", workspace_path, "rev-parse", "--is-inside-work-tree"],
        stderr_to_stdout: true
      )

    {output, 0} =
      System.cmd("git", ["-C", workspace_path, "status", "--porcelain", "--untracked-files=all"],
        stderr_to_stdout: true
      )

    output
    |> String.split("\n", trim: true)
  end

  @spec execute_seed_hooks() :: map()
  defp execute_seed_hooks do
    %{
      after_create: """
      git init -q
      git config user.email test@example.com
      git config user.name "Open Sleigh Test"
      mkdir -p lib/open_sleigh
      printf 'baseline\\n' > lib/open_sleigh/agent_worker.ex
      git add lib/open_sleigh/agent_worker.ex
      git commit -qm 'baseline'
      printf 'dirty\\n' >> lib/open_sleigh/agent_worker.ex
      """
    }
  end
end
