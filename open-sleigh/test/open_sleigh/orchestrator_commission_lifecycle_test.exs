defmodule OpenSleigh.OrchestratorCommissionLifecycleTest do
  use ExUnit.Case, async: false

  defmodule BlockingAgent do
    @behaviour OpenSleigh.Agent.Adapter

    def adapter_kind, do: :blocking_mock

    def tool_registry do
      [:haft_query, :read, :write, :edit, :bash, :haft_note, :haft_decision, :haft_refresh]
    end

    def start_session(%OpenSleigh.AdapterSession{session_id: session_id}) do
      owner = :persistent_term.get({__MODULE__, :owner})
      send(owner, {:blocking_agent_started, session_id})
      {:ok, %{owner: owner, session_id: session_id}}
    end

    def send_turn(%{owner: owner, session_id: session_id}, _prompt, _session) do
      send(owner, {:blocking_agent_turn, session_id, self()})

      receive do
        {:release_blocking_agent, ^session_id} ->
          {:ok,
           %{
             turn_id: "blocking-turn-" <> session_id,
             status: :completed,
             events: [],
             usage: %{input_tokens: 1, output_tokens: 1, total_tokens: 2},
             text: "done"
           }}
      after
        5_000 ->
          {:error, :turn_timeout}
      end
    end

    def dispatch_tool(_handle, _tool, _args, _session), do: {:error, :tool_unknown_to_adapter}
    def close_session(_handle), do: :ok
  end

  defmodule LifecycleAgent do
    @behaviour OpenSleigh.Agent.Adapter

    alias OpenSleigh.Agent.Adapter, as: AgentAdapter

    @tool_registry [
      :haft_query,
      :read,
      :write,
      :edit,
      :bash,
      :haft_note,
      :haft_decision,
      :haft_refresh
    ]

    def adapter_kind, do: :lifecycle_mock
    def tool_registry, do: @tool_registry

    def start_session(%OpenSleigh.AdapterSession{session_id: session_id}) do
      {:ok, %{session_id: session_id}}
    end

    def send_turn(%{session_id: session_id}, prompt, %OpenSleigh.AdapterSession{} = session) do
      {:ok, reply(phase_from(prompt, session), session_id)}
    end

    def dispatch_tool(_handle, tool, args, %OpenSleigh.AdapterSession{} = session)
        when tool in @tool_registry do
      case AgentAdapter.ensure_in_scope(session, tool, args) do
        :ok ->
          {:ok, %{call_id: "lifecycle-call-" <> Atom.to_string(tool), result: %{success: true}}}

        {:error, _reason} = error ->
          error
      end
    end

    def dispatch_tool(_handle, _tool, _args, %OpenSleigh.AdapterSession{}),
      do: {:error, :tool_unknown_to_adapter}

    def close_session(_handle), do: :ok

    defp reply(:measure, session_id) do
      session_id
      |> reply_base()
      |> Map.put(:events, [measure_evidence_event()])
    end

    defp reply(_phase, session_id), do: reply_base(session_id)

    defp phase_from("Measure " <> _rest, _session), do: :measure
    defp phase_from(_prompt, _session), do: :other

    defp reply_base(session_id) do
      %{
        turn_id: "lifecycle-turn-" <> session_id,
        status: :completed,
        events: [],
        usage: %{input_tokens: 1, output_tokens: 1, total_tokens: 2},
        text: "mock agent output"
      }
    end

    defp measure_evidence_event do
      %{
        payload: %{
          "item" => %{
            "type" => "commandExecution",
            "status" => "completed",
            "command" => "mix test",
            "exitCode" => 0,
            "aggregatedOutput" => "ok\n"
          }
        }
      }
    end
  end

  alias OpenSleigh.Agent.Mock, as: AgentMock
  alias OpenSleigh.Haft.Mock, as: HaftMock
  alias OpenSleigh.Tracker.Mock, as: TrackerMock

  alias OpenSleigh.{
    JudgeClient,
    ObservationsBus,
    Orchestrator,
    PhaseConfig,
    Scope,
    WorkCommission,
    Workflow,
    WorkflowStore
  }

  setup do
    ObservationsBus.reset()
    AgentMock.reset!()

    workspace_root =
      System.tmp_dir!()
      |> Path.join("orchestrator_commission_lifecycle_#{System.unique_integer([:positive])}")

    File.mkdir_p!(workspace_root)
    on_exit(fn -> File.rm_rf!(workspace_root) end)

    {:ok, haft} = HaftMock.start()
    {:ok, tracker} = TrackerMock.start()

    commission = commission_fixture!()
    ticket_attrs = ticket_attrs(commission)

    :ok = TrackerMock.seed(tracker, [ticket_attrs])

    suffix = :erlang.unique_integer([:positive])
    store_name = String.to_atom("commission_lifecycle_store_#{suffix}")
    orchestrator_name = String.to_atom("commission_lifecycle_orch_#{suffix}")

    {:ok, _store} =
      WorkflowStore.start_link(
        phase_configs: phase_configs(),
        prompts: prompts(),
        external_publication: %{branch_regex: "^(main|master)$"},
        name: store_name
      )

    {:ok, _orchestrator} =
      Orchestrator.start_link(
        workflow: Workflow.mvp1r(),
        tracker_handle: tracker,
        tracker_adapter: TrackerMock,
        agent_adapter: LifecycleAgent,
        external_publication: %{tracker_transition_to: []},
        judge_fun: JudgeClient.judge_fun(fn _prompt -> {:ok, %{}} end, %{}),
        haft_invoker: HaftMock.invoke_fun(haft),
        workspace_root: workspace_root,
        guard_config: %{forbidden_paths: [], forbidden_remote_substrings: ["open-sleigh"]},
        task_supervisor: OpenSleigh.AgentSupervisor,
        workflow_store: store_name,
        hooks: execute_seed_hooks(),
        name: orchestrator_name
      )

    %{
      haft: haft,
      orchestrator: orchestrator_name,
      tracker: tracker,
      workspace_root: workspace_root
    }
  end

  test "MVP-1R records WorkCommission lifecycle while phases run", ctx do
    {:ok, tickets} = TrackerMock.list_active(ctx.tracker)
    Orchestrator.submit_candidates(ctx.orchestrator, tickets)

    wait_result = wait_for_terminal(ctx.orchestrator, 10_000)
    status = Orchestrator.status(ctx.orchestrator)
    artifacts = HaftMock.artifacts(ctx.haft)

    assert wait_result == :ok,
           inspect(%{wait_result: wait_result, status: status, artifacts: artifacts}, pretty: true)

    actions =
      ctx.haft
      |> HaftMock.artifacts()
      |> Enum.filter(&(Map.get(&1, "name") == "haft_commission"))
      |> Enum.map(&get_in(&1, ["arguments", "action"]))

    assert actions == [
             "record_run_event",
             "record_preflight",
             "start_after_preflight",
             "record_run_event",
             "record_run_event",
             "record_run_event",
             "complete_or_block"
           ]

    terminal =
      ctx.haft
      |> HaftMock.artifacts()
      |> Enum.find(&(get_in(&1, ["arguments", "action"]) == "complete_or_block"))

    assert get_in(terminal, ["arguments", "commission_id"]) == "wc-orchestrator-lifecycle"
    assert get_in(terminal, ["arguments", "verdict"]) == "pass"

    assert {:ok, ticket} = TrackerMock.get(ctx.tracker, "wc-orchestrator-lifecycle")
    assert ticket.state == :done
    assert {:ok, []} = TrackerMock.list_active(ctx.tracker)
  end

  test "external_required terminal pass leaves external carrier open", ctx do
    {:ok, tracker} = TrackerMock.start()
    commission = commission_fixture!("wc-external-required-carrier-open", %{projection_policy: :external_required})
    :ok = TrackerMock.seed(tracker, [ticket_attrs(commission)])

    orchestrator_name = :"commission_lifecycle_projection_debt_orch_#{System.unique_integer([:positive])}"
    store_name = :"commission_lifecycle_projection_debt_store_#{System.unique_integer([:positive])}"

    {:ok, _store} =
      WorkflowStore.start_link(
        phase_configs: phase_configs(),
        prompts: prompts(),
        external_publication: %{branch_regex: "^(main|master)$"},
        name: store_name
      )

    {:ok, _orchestrator} =
      Orchestrator.start_link(
        workflow: Workflow.mvp1r(),
        tracker_handle: tracker,
        tracker_adapter: TrackerMock,
        agent_adapter: LifecycleAgent,
        external_publication: %{tracker_transition_to: []},
        judge_fun: JudgeClient.judge_fun(fn _prompt -> {:ok, %{}} end, %{}),
        haft_invoker: HaftMock.invoke_fun(ctx.haft),
        workspace_root: ctx.workspace_root,
        guard_config: %{forbidden_paths: [], forbidden_remote_substrings: ["open-sleigh"]},
        task_supervisor: OpenSleigh.AgentSupervisor,
        workflow_store: store_name,
        hooks: execute_seed_hooks(),
        name: orchestrator_name
      )

    {:ok, tickets} = TrackerMock.list_active(tracker)
    Orchestrator.submit_candidates(orchestrator_name, tickets)

    wait_result = wait_for_terminal(orchestrator_name, 10_000)
    status = Orchestrator.status(orchestrator_name)
    artifacts = HaftMock.artifacts(ctx.haft)

    assert wait_result == :ok,
           inspect(%{wait_result: wait_result, status: status, artifacts: artifacts}, pretty: true)

    terminal =
      ctx.haft
      |> HaftMock.artifacts()
      |> Enum.find(&(get_in(&1, ["arguments", "action"]) == "complete_or_block"))

    assert get_in(terminal, ["arguments", "commission_id"]) == "wc-external-required-carrier-open"
    assert get_in(terminal, ["arguments", "verdict"]) == "pass"
    assert get_in(terminal, ["arguments", "payload", "projection_policy"]) == "external_required"

    assert {:ok, ticket} = TrackerMock.get(tracker, "wc-external-required-carrier-open")
    assert ticket.state == :in_progress

    assert {:ok, active} = TrackerMock.list_active(tracker)
    assert Enum.map(active, & &1.id) == ["wc-external-required-carrier-open"]
  end

  test "blocked non-terminal phases also close WorkCommission lifecycle", ctx do
    {:ok, tracker} = TrackerMock.start()
    commission = commission_fixture!("wc-orchestrator-blocked")
    :ok = TrackerMock.seed(tracker, [ticket_attrs(commission)])

    orchestrator_name = :"commission_lifecycle_block_orch_#{System.unique_integer([:positive])}"
    store_name = :"commission_lifecycle_block_store_#{System.unique_integer([:positive])}"

    {:ok, _store} =
      WorkflowStore.start_link(
        phase_configs: phase_configs_with_frame_semantic_gate(),
        prompts: prompts(),
        external_publication: %{branch_regex: "^(main|master)$"},
        name: store_name
      )

    {:ok, _orchestrator} =
      Orchestrator.start_link(
        workflow: Workflow.mvp1r(),
        tracker_handle: tracker,
        tracker_adapter: TrackerMock,
        agent_adapter: AgentMock,
        external_publication: %{tracker_transition_to: []},
        judge_fun: fail_judge_fun(),
        haft_invoker: HaftMock.invoke_fun(ctx.haft),
        workspace_root: ctx.workspace_root,
        guard_config: %{forbidden_paths: [], forbidden_remote_substrings: ["open-sleigh"]},
        task_supervisor: OpenSleigh.AgentSupervisor,
        workflow_store: store_name,
        name: orchestrator_name
      )

    {:ok, tickets} = TrackerMock.list_active(tracker)
    Orchestrator.submit_candidates(orchestrator_name, tickets)

    wait_result = wait_for_terminal(orchestrator_name, 10_000)
    status = Orchestrator.status(orchestrator_name)
    artifacts = HaftMock.artifacts(ctx.haft)

    assert wait_result == :ok,
           inspect(%{wait_result: wait_result, status: status, artifacts: artifacts}, pretty: true)

    actions =
      ctx.haft
      |> HaftMock.artifacts()
      |> Enum.filter(&(Map.get(&1, "name") == "haft_commission"))
      |> Enum.map(&get_in(&1, ["arguments", "action"]))

    assert actions == [
             "record_run_event",
             "record_preflight",
             "start_after_preflight",
             "record_run_event",
             "complete_or_block"
           ]

    blocked =
      ctx.haft
      |> HaftMock.artifacts()
      |> Enum.find(&(get_in(&1, ["arguments", "action"]) == "complete_or_block"))

    assert get_in(blocked, ["arguments", "commission_id"]) == "wc-orchestrator-blocked"
    assert get_in(blocked, ["arguments", "verdict"]) == "blocked"
    assert get_in(blocked, ["arguments", "event"]) == "phase_blocked"

    assert {:ok, ticket} = TrackerMock.get(tracker, "wc-orchestrator-blocked")
    assert ticket.state == :blocked
    assert {:ok, []} = TrackerMock.list_active(tracker)
  end

  test "terminal diff validation failure also closes WorkCommission lifecycle", ctx do
    {:ok, tracker} = TrackerMock.start()
    commission =
      "wc-orchestrator-scope-block"
      |> commission_fixture!(%{
        scope:
          scope_fixture!(%{
            allowed_paths: ["allowed.md"],
            affected_files: ["allowed.md"],
            lockset: ["allowed.md"]
          })
      })

    :ok = TrackerMock.seed(tracker, [ticket_attrs(commission)])

    orchestrator_name = :"commission_lifecycle_scope_block_orch_#{System.unique_integer([:positive])}"
    store_name = :"commission_lifecycle_scope_block_store_#{System.unique_integer([:positive])}"

    {:ok, _store} =
      WorkflowStore.start_link(
        phase_configs: phase_configs_without_execute_semantic_gates(),
        prompts: prompts(),
        external_publication: %{branch_regex: "^(main|master)$"},
        name: store_name
      )

    {:ok, _orchestrator} =
      Orchestrator.start_link(
        workflow: Workflow.mvp1r(),
        tracker_handle: tracker,
        tracker_adapter: TrackerMock,
        agent_adapter: AgentMock,
        external_publication: %{tracker_transition_to: []},
        judge_fun: JudgeClient.judge_fun(fn _prompt -> {:ok, %{}} end, %{}),
        haft_invoker: HaftMock.invoke_fun(ctx.haft),
        workspace_root: ctx.workspace_root,
        guard_config: %{forbidden_paths: [], forbidden_remote_substrings: ["open-sleigh"]},
        task_supervisor: OpenSleigh.AgentSupervisor,
        workflow_store: store_name,
        hooks: %{
          after_create: """
          git init -q
          git config user.email test@example.com
          git config user.name "Open Sleigh Test"
          printf 'allowed\\n' > allowed.md
          printf 'outside\\n' > outside.md
          git add allowed.md outside.md
          git commit -qm 'baseline'
          printf 'scope violation\\n' >> outside.md
          """
        },
        name: orchestrator_name
      )

    {:ok, tickets} = TrackerMock.list_active(tracker)
    Orchestrator.submit_candidates(orchestrator_name, tickets)

    wait_result = wait_for_terminal(orchestrator_name, 10_000)
    status = Orchestrator.status(orchestrator_name)
    artifacts = HaftMock.artifacts(ctx.haft)

    assert wait_result == :ok,
           inspect(%{wait_result: wait_result, status: status, artifacts: artifacts}, pretty: true)

    blocked =
      ctx.haft
      |> HaftMock.artifacts()
      |> Enum.find(&(get_in(&1, ["arguments", "commission_id"]) == "wc-orchestrator-scope-block"))
      |> then(fn _ ->
        ctx.haft
        |> HaftMock.artifacts()
        |> Enum.filter(&(get_in(&1, ["arguments", "commission_id"]) == "wc-orchestrator-scope-block"))
        |> Enum.find(&(get_in(&1, ["arguments", "action"]) == "complete_or_block"))
      end)

    assert get_in(blocked, ["arguments", "commission_id"]) == "wc-orchestrator-scope-block"
    assert get_in(blocked, ["arguments", "verdict"]) == "blocked"
    assert get_in(blocked, ["arguments", "event"]) == "phase_blocked"
    assert get_in(blocked, ["arguments", "reason"]) == "mutation_outside_commission_scope"
    assert get_in(blocked, ["arguments", "payload", "out_of_scope_paths"]) == ["outside.md"]

    assert {:ok, ticket} = TrackerMock.get(tracker, "wc-orchestrator-scope-block")
    assert ticket.state == :blocked
    assert {:ok, []} = TrackerMock.list_active(tracker)
  end

  test "max_concurrency keeps extra commission tickets queued" do
    :persistent_term.put({BlockingAgent, :owner}, self())
    on_exit(fn -> :persistent_term.erase({BlockingAgent, :owner}) end)

    workspace_root =
      System.tmp_dir!()
      |> Path.join("orchestrator_commission_concurrency_#{System.unique_integer([:positive])}")

    File.mkdir_p!(workspace_root)
    on_exit(fn -> File.rm_rf!(workspace_root) end)

    {:ok, haft} = HaftMock.start()
    {:ok, tracker} = TrackerMock.start()

    commission_a = commission_fixture!("wc-concurrency-a")
    commission_b = commission_fixture!("wc-concurrency-b")

    :ok = TrackerMock.seed(tracker, [ticket_attrs(commission_a), ticket_attrs(commission_b)])

    suffix = :erlang.unique_integer([:positive])
    store_name = String.to_atom("commission_concurrency_store_#{suffix}")
    orchestrator_name = String.to_atom("commission_concurrency_orch_#{suffix}")

    {:ok, _store} =
      WorkflowStore.start_link(
        phase_configs: Map.take(phase_configs(), [:preflight]),
        prompts: Map.take(prompts(), [:preflight]),
        external_publication: %{},
        name: store_name
      )

    {:ok, _orchestrator} =
      Orchestrator.start_link(
        workflow: single_phase_preflight_workflow(),
        tracker_handle: tracker,
        tracker_adapter: TrackerMock,
        agent_adapter: BlockingAgent,
        external_publication: %{tracker_transition_to: []},
        judge_fun: JudgeClient.judge_fun(fn _prompt -> {:ok, %{}} end, %{}),
        haft_invoker: HaftMock.invoke_fun(haft),
        workspace_root: workspace_root,
        guard_config: %{forbidden_paths: [], forbidden_remote_substrings: ["open-sleigh"]},
        task_supervisor: OpenSleigh.AgentSupervisor,
        workflow_store: store_name,
        max_concurrency: 1,
        name: orchestrator_name
      )

    {:ok, tickets} = TrackerMock.list_active(tracker)
    Orchestrator.submit_candidates(orchestrator_name, tickets)

    assert_receive {:blocking_agent_turn, session_id, worker}, 1_000
    refute_receive {:blocking_agent_turn, _other_session_id, _other_worker}, 100

    status = Orchestrator.status(orchestrator_name)
    assert length(status.running) == 1

    send(worker, {:release_blocking_agent, session_id})
    assert :ok = wait_for_terminal(orchestrator_name, 10_000)
  end

  defp phase_configs do
    %{
      preflight:
        phase_config(:preflight, :preflight_checker, [:haft_query, :read], :preflight, 1),
      frame: phase_config(:frame, :frame_verifier, [:haft_query, :read], :frame, 1),
      execute: phase_config(:execute, :executor, [:read, :write, :bash, :haft_note], :execute, 2),
      measure: phase_config(:measure, :measurer, [:haft_decision, :haft_refresh], :measure, 1)
    }
  end

  defp phase_configs_with_frame_semantic_gate do
    phase_configs()
    |> Map.update!(:frame, fn config ->
      %{config | gates: %{structural: [], semantic: [:lade_quadrants_split_ok], human: []}}
    end)
  end

  defp phase_configs_without_execute_semantic_gates do
    phase_configs()
    |> Map.update!(:execute, fn config ->
      %{config | gates: %{structural: [], semantic: [], human: []}, max_turns: 1}
    end)
  end

  defp phase_config(phase, role, tools, prompt_key, max_turns) do
    %{
      phase: phase,
      agent_role: role,
      tools: tools,
      gates: %{structural: [], semantic: [], human: []},
      prompt_template_key: prompt_key,
      max_turns: max_turns,
      default_valid_until_days: 7
    }
    |> PhaseConfig.new()
    |> unwrap!()
  end

  defp prompts do
    %{
      preflight: "Check WorkCommission {{commission.id}}",
      frame: "Verify frame for {{commission.id}}",
      execute: "Execute {{commission.id}}",
      measure: "Measure {{commission.id}}"
    }
  end

  defp fail_judge_fun do
    fn _gate_module, _gate_context ->
      {:ok, %{verdict: :fail, cl: 3, rationale: "phase exit gates still fail"}}
    end
  end

  defp ticket_attrs(%WorkCommission{} = commission) do
    %{
      id: commission.id,
      source: {:github, commission.scope.repo_ref},
      title: "WorkCommission " <> commission.id,
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
    }
  end

  defp single_phase_preflight_workflow do
    Workflow.mvp1r()
    |> Map.put(:phases, [:preflight, :terminal])
    |> Map.put(:advance_map, %{preflight: :terminal})
  end

  defp execute_seed_hooks do
    %{
      after_create: """
      git init -q
      git config user.email test@example.com
      git config user.name "Open Sleigh Test"
      mkdir -p lib/open_sleigh
      printf 'baseline\\n' > lib/open_sleigh/orchestrator.ex
      git add lib/open_sleigh/orchestrator.ex
      git commit -qm 'baseline'
      printf 'dirty\\n' >> lib/open_sleigh/orchestrator.ex
      """
    }
  end

  defp commission_fixture!(id \\ "wc-orchestrator-lifecycle", overrides \\ %{}) do
    scope =
      overrides
      |> Map.get(:scope, scope_fixture!())

    %{
      id: id,
      decision_ref: "dec-orchestrator-lifecycle",
      decision_revision_hash: "decision-r1",
      problem_card_ref: "pc-orchestrator-lifecycle",
      implementation_plan_ref: "plan-orchestrator-lifecycle",
      implementation_plan_revision: "plan-r1",
      scope: scope,
      scope_hash: scope.hash,
      base_sha: scope.base_sha,
      lockset: scope.lockset,
      evidence_requirements: [],
      projection_policy: :local_only,
      state: :preflighting,
      valid_until: ~U[2099-01-01 00:00:00Z],
      fetched_at: ~U[2026-04-22 10:00:00Z]
    }
    |> Map.merge(Map.delete(overrides, :scope))
    |> WorkCommission.new()
    |> unwrap!()
  end

  defp scope_fixture!(overrides \\ %{}) do
    attrs = %{
      repo_ref: "local:haft",
      base_sha: "base-r1",
      target_branch: "feature/open-sleigh-commission-lifecycle",
      allowed_paths: ["**/*"],
      forbidden_paths: [],
      allowed_actions: MapSet.new([:edit_files, :run_tests]),
      affected_files: ["**/*"],
      allowed_modules: [],
      lockset: ["**/*"]
    }
    |> Map.merge(overrides)

    {:ok, hash} = Scope.canonical_hash(attrs)

    attrs
    |> Map.put(:hash, hash)
    |> Scope.new()
    |> unwrap!()
  end

  defp wait_for_terminal(orchestrator, timeout_ms) do
    deadline = System.monotonic_time(:millisecond) + timeout_ms
    wait_until_terminal(orchestrator, deadline)
  end

  defp wait_until_terminal(orchestrator, deadline) do
    status = Orchestrator.status(orchestrator)

    cond do
      status.claimed == [] and status.running == [] ->
        :ok

      System.monotonic_time(:millisecond) > deadline ->
        {:error, :timeout}

      true ->
        Process.sleep(10)
        wait_until_terminal(orchestrator, deadline)
    end
  end

  defp unwrap!({:ok, value}), do: value
end
