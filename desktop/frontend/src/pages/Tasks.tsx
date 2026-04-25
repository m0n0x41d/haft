import { useCallback, useEffect, useRef, useState, type Dispatch, type SetStateAction } from "react";

import { subscribe } from "../lib/events";
import {
  archiveTask,
  cancelTask,
  continueTask,
  createPullRequest,
  detectAgents,
  getConfig,
  getTaskTranscriptState,
  handoffTask,
  listTasks,
  openPathInIDE,
  resolveAdoptBaseline,
  resolveAdoptDeprecate,
  resolveAdoptMeasure,
  resolveAdoptReopen,
  resolveAdoptWaive,
  spawnTask,
  setTaskAutoRun,
  writeTaskInput,
  type ChatTranscriptState,
  type DesktopConfig,
  type InstalledAgent,
  type PullRequestResult,
  type TaskOutputEvent,
  type TaskState,
} from "../lib/api";
import { mergeTaskStatusEvent, type TaskStatusEvent } from "../lib/taskState";
import { ChatInput } from "../components/ChatInput";
import { ChatView } from "../components/ChatView";
import { IrreversibleActionDialog } from "../components/IrreversibleActionDialog";
import {
  buildIrreversibleActionDialogModel,
  type IrreversibleActionDialogModel,
} from "../components/irreversibleActionDialogModel";
import { reportError } from "../lib/errors";
import {
  getTaskExecutionLadder,
  type TaskExecutionLadder,
  type ExecutionLadderStep,
} from "./taskExecutionLadder";
import { taskFollowUpAction, taskInputCapability } from "../lib/taskInput";
import { visibleInitialPrompt } from "../lib/taskPrompt";

type AdoptResolutionMode = "drift" | "stale";
type AdoptResolutionContext = {
  findingID: string;
  decisionID: string;
  mode: AdoptResolutionMode;
};

type TaskPendingConfirmation = {
  model: IrreversibleActionDialogModel;
  reason: string;
  isSubmitting: boolean;
  run: (reason: string) => Promise<void>;
  errorScope: string;
};

// PromptSection reserved for future collapsible brief in chat
// interface PromptSection { title: string; body: string; }

export function Tasks({
  selectedTaskId: externalSelectedTask,
  showNewTask: externalShow,
  onNewTaskClose,
  tasks: controlledTasks,
  onTasksChange,
  onTasksRefresh,
}: {
  selectedTaskId?: string | null;
  showNewTask?: boolean;
  onNewTaskClose?: () => void;
  tasks?: TaskState[];
  onTasksChange?: Dispatch<SetStateAction<TaskState[]>>;
  onTasksRefresh?: () => Promise<void> | void;
} = {}) {
  const [internalTasks, setInternalTasks] = useState<TaskState[]>([]);
  const [agents, setAgents] = useState<InstalledAgent[]>([]);
  const [config, setConfig] = useState<DesktopConfig | null>(null);
  const [internalShow, setInternalShow] = useState(false);
  const [selectedTask, setSelectedTask] = useState<string | null>(externalSelectedTask ?? null);
  const [showHandoff, setShowHandoff] = useState(false);
  const [handoffAgent, setHandoffAgent] = useState("codex");
  const [resolutionAction, setResolutionAction] = useState("");
  const [taskActionMessage, setTaskActionMessage] = useState("");
  const [followUpInput, setFollowUpInput] = useState("");
  const [isSubmittingFollowUp, setIsSubmittingFollowUp] = useState(false);
  const [pendingConfirmation, setPendingConfirmation] =
    useState<TaskPendingConfirmation | null>(null);
  const outputRef = useRef<HTMLDivElement | null>(null);
  const selectedTaskRef = useRef<string | null>(externalSelectedTask ?? null);
  const transcriptRefreshTimerRef = useRef<number | null>(null);
  const tasks = controlledTasks ?? internalTasks;
  const setTasks = onTasksChange ?? setInternalTasks;

  const showNewTask = externalShow || internalShow;

  const setShowNewTask = (visible: boolean) => {
    setInternalShow(visible);

    if (!visible && onNewTaskClose) {
      onNewTaskClose();
    }
  };

  const refresh = useCallback(async () => {
    if (onTasksRefresh) {
      await onTasksRefresh();
      return;
    }

    try {
      const nextTasks = await listTasks();
      setTasks(nextTasks);
    } catch (error) {
      reportError(error, "tasks");
    }
  }, [onTasksRefresh, setTasks]);

  const syncTaskTranscript = useCallback(
    async (taskID: string) => {
      try {
        const transcript = await getTaskTranscriptState(taskID);

        setTasks((current) =>
          current.map((task) =>
            task.id === taskID
              ? mergeTaskTranscript(task, transcript)
              : task,
          ),
        );
      } catch (error) {
        reportError(error, "task transcript");
      }
    },
    [setTasks],
  );

  const scheduleTaskTranscriptSync = useCallback(
    (taskID: string) => {
      if (transcriptRefreshTimerRef.current !== null) {
        window.clearTimeout(transcriptRefreshTimerRef.current);
      }

      transcriptRefreshTimerRef.current = window.setTimeout(() => {
        transcriptRefreshTimerRef.current = null;
        void syncTaskTranscript(taskID);
      }, 150);
    },
    [syncTaskTranscript],
  );

  useEffect(() => {
    if (externalShow) {
      setInternalShow(true);
    }
  }, [externalShow]);

  useEffect(() => {
    if (!externalSelectedTask) {
      return;
    }

    setSelectedTask(externalSelectedTask);
  }, [externalSelectedTask]);

  useEffect(() => {
    selectedTaskRef.current = selectedTask;
  }, [selectedTask]);

  useEffect(() => {
    return () => {
      if (transcriptRefreshTimerRef.current !== null) {
        window.clearTimeout(transcriptRefreshTimerRef.current);
      }
    };
  }, []);

  useEffect(() => {
    detectAgents()
      .then(setAgents)
      .catch((error) => {
        reportError(error, "agents");
      });

    getConfig()
      .then(setConfig)
      .catch((error) => {
        reportError(error, "task config");
      });

    if (!onTasksRefresh) {
      void refresh();

      const interval = window.setInterval(() => {
        void refresh();
      }, 2000);

      return () => {
        window.clearInterval(interval);
      };
    }

    return undefined;
  }, [onTasksRefresh, refresh]);

  useEffect(() => {
    const stopOutput = subscribe<TaskOutputEvent>("task.output", (payload) => {
      setTasks((current) =>
        current.map((task) =>
          task.id === payload.id
            ? {
                ...task,
                output: payload.output,
                raw_output: payload.output,
              }
            : task,
        ),
      );

      if (selectedTaskRef.current === payload.id) {
        scheduleTaskTranscriptSync(payload.id);
      }
    });

    const stopStatus = subscribe<TaskStatusEvent>("task.status", (payload) => {
      setTasks((current) => mergeTaskStatusEvent(current, payload));
    });

    return () => {
      stopOutput?.();
      stopStatus?.();
    };
  }, [scheduleTaskTranscriptSync, setTasks]);

  useEffect(() => {
    if (tasks.length === 0) {
      setSelectedTask(null);
      return;
    }

    if (selectedTask && tasks.some((task) => task.id === selectedTask)) {
      return;
    }

    setSelectedTask(tasks[0]?.id ?? null);
  }, [selectedTask, tasks]);

  useEffect(() => {
    if (!selectedTask) {
      return;
    }

    getTaskTranscriptState(selectedTask)
      .then((transcript) => {
        setTasks((current) =>
          current.map((task) =>
            task.id === selectedTask
              ? mergeTaskTranscript(task, transcript)
              : task,
          ),
        );
      })
      .catch((error) => {
        reportError(error, "task transcript");
      });
  }, [selectedTask, setTasks]);

  const detail = tasks.find((task) => task.id === selectedTask) ?? null;
  const workspacePath = detail ? detail.worktree_path || detail.project_path : "";
  const adoptResolution = getAdoptResolutionContext(detail);
  const executionLadder = detail ? getTaskExecutionLadder(detail) : null;
  const displayStatus = executionLadder?.currentLabel ?? detail?.status ?? "";
  const initialBrief = detail
    ? visibleInitialPrompt(detail.prompt, detail.chat_blocks)
    : "";

  useEffect(() => {
    setResolutionAction("");
    setTaskActionMessage("");
    setFollowUpInput("");
    setIsSubmittingFollowUp(false);
    setPendingConfirmation(null);
  }, [detail?.id]);

  useEffect(() => {
    if (!detail || !outputRef.current) {
      return;
    }

    outputRef.current.scrollTop = outputRef.current.scrollHeight;
  }, [detail]);

  const spawningRef = useRef(false);
  const handleSpawn = async (agent: string, prompt: string, worktree: boolean, branch: string) => {
    if (spawningRef.current) return;
    spawningRef.current = true;
    try {
      const task = await spawnTask(agent, prompt, worktree, branch);

      setTasks((current) => mergeTaskList(current, task));
      setSelectedTask(task.id);
      setShowNewTask(false);
      void refresh();
    } catch (error) {
      reportError(error, "spawn task");
    } finally {
      spawningRef.current = false;
    }
  };

  const handleCancel = async (id: string) => {
    try {
      await cancelTask(id);
      void refresh();
    } catch (error) {
      reportError(error, "cancel task");
    }
  };

  const handleArchive = async (id: string) => {
    try {
      await archiveTask(id);

      setTasks((current) => current.filter((task) => task.id !== id));
      if (selectedTask === id) {
        setSelectedTask(null);
      }
    } catch (error) {
      reportError(error, "archive task");
    }
  };

  const handleOpenWorkspace = async () => {
    if (!workspacePath) {
      return;
    }

    try {
      await openPathInIDE(workspacePath);
    } catch (error) {
      reportError(error, "open workspace");
    }
  };

  const handleHandoff = async () => {
    if (!detail) {
      return;
    }

    try {
      const task = await handoffTask(detail.id, handoffAgent);

      setTasks((current) => mergeTaskList(current, task));
      setSelectedTask(task.id);
      setShowHandoff(false);
      await refresh();
    } catch (error) {
      reportError(error, "handoff task");
    }
  };

  const runResolutionAction = async (
    actionKey: string,
    run: () => Promise<void>,
    errorScope: string,
  ) => {
    setResolutionAction(actionKey);
    setTaskActionMessage("");

    try {
      await run();
      await refresh();
    } catch (error) {
      reportError(error, errorScope);
    } finally {
      setResolutionAction("");
    }
  };

  const handleRebaseline = async (resolution: AdoptResolutionContext) => {
    if (typeof window !== "undefined") {
      const confirmed = window.confirm(
        "Re-baseline will replace the stored SHA-256 snapshot for this DecisionRecord using the current project state.\n\nContinue?",
      );

      if (!confirmed) {
        return;
      }
    }

    await runResolutionAction(
      "baseline",
      async () => {
        await resolveAdoptBaseline(resolution.findingID, resolution.decisionID);
        setTaskActionMessage(`Re-baselined ${resolution.decisionID}.`);
      },
      "baseline decision",
    );
  };

  const handleWaive = async (resolution: AdoptResolutionContext, task: TaskState) => {
    const reason = promptForAdoptReason(task);
    if (reason === "") {
      return;
    }

    await runResolutionAction(
      "waive",
      async () => {
        await resolveAdoptWaive(resolution.findingID, resolution.decisionID, reason);
        setTaskActionMessage(`Waived ${resolution.decisionID}.`);
      },
      "waive decision",
    );
  };

  const handleReopen = async (resolution: AdoptResolutionContext, task: TaskState) => {
    setPendingConfirmation({
      model: buildIrreversibleActionDialogModel({
        action: "reopen",
        currentArtifact: {
          kind: "DecisionRecord",
          ref: resolution.decisionID,
          title: task.title,
        },
      }),
      reason: taskPromptMetaValue(task.prompt, "Reason"),
      isSubmitting: false,
      run: async (reason) => {
        setResolutionAction("reopen");
        setTaskActionMessage("");

        try {
          const problem = await resolveAdoptReopen(
            resolution.findingID,
            resolution.decisionID,
            reason,
          );
          setTaskActionMessage(`Reopened ${resolution.decisionID} as ${problem.id}.`);
          await refresh();
        } finally {
          setResolutionAction("");
        }
      },
      errorScope: "reopen decision",
    });
  };

  const handleDeprecate = async (resolution: AdoptResolutionContext, task: TaskState) => {
    setPendingConfirmation({
      model: buildIrreversibleActionDialogModel({
        action: "deprecate",
        currentArtifact: {
          kind: "DecisionRecord",
          ref: resolution.decisionID,
          title: task.title,
        },
      }),
      reason: taskPromptMetaValue(task.prompt, "Reason"),
      isSubmitting: false,
      run: async (reason) => {
        setResolutionAction("deprecate");
        setTaskActionMessage("");

        try {
          await resolveAdoptDeprecate(resolution.findingID, resolution.decisionID, reason);
          setTaskActionMessage(`Deprecated ${resolution.decisionID}.`);
          await refresh();
        } finally {
          setResolutionAction("");
        }
      },
      errorScope: "deprecate decision",
    });
  };

  const handleMeasure = async (resolution: AdoptResolutionContext, task: TaskState) => {
    const findings = promptForMeasureFindings(task);
    if (findings === "") {
      return;
    }

    const verdict = promptForMeasureVerdict();
    if (verdict === "") {
      return;
    }

    await runResolutionAction(
      "measure",
      async () => {
        await resolveAdoptMeasure(
          resolution.findingID,
          resolution.decisionID,
          findings,
          verdict,
        );
        setTaskActionMessage(`Measured ${resolution.decisionID} with verdict ${verdict}.`);
      },
      "measure decision",
    );
  };

  const handleCreatePullRequest = async (task: TaskState) => {
    const decisionID = taskPromptMetaValue(task.prompt, "Decision ID");

    setPendingConfirmation({
      model: buildIrreversibleActionDialogModel({
        action: "create_pr",
        branch: task.branch,
        currentArtifact: {
          kind: decisionID ? "DecisionRecord" : "Task",
          ref: decisionID || task.id,
          title: task.title,
        },
      }),
      reason: "",
      isSubmitting: false,
      run: async () => {
        setResolutionAction("create_pr");
        setTaskActionMessage("");

        try {
          const result = await createPullRequest(task.id, decisionID, task.branch);
          setTaskActionMessage(buildPullRequestMessage(result));
        } finally {
          setResolutionAction("");
        }
      },
      errorScope: "create pull request",
    });
  };

  const handleCancelConfirmation = () => {
    setPendingConfirmation((currentPendingConfirmation) => {
      if (currentPendingConfirmation?.isSubmitting) {
        return currentPendingConfirmation;
      }

      return null;
    });
  };

  const handleConfirmAction = async () => {
    const currentPendingConfirmation = pendingConfirmation;

    if (!currentPendingConfirmation) {
      return;
    }

    if (
      currentPendingConfirmation.model.requiresReason &&
      currentPendingConfirmation.reason.trim() === ""
    ) {
      return;
    }

    setPendingConfirmation((previousPendingConfirmation) =>
      previousPendingConfirmation
        ? {
            ...previousPendingConfirmation,
            isSubmitting: true,
          }
        : previousPendingConfirmation,
    );

    try {
      await currentPendingConfirmation.run(currentPendingConfirmation.reason.trim());
      setPendingConfirmation(null);
    } catch (error) {
      reportError(error, currentPendingConfirmation.errorScope);
      setPendingConfirmation((previousPendingConfirmation) =>
        previousPendingConfirmation
          ? {
              ...previousPendingConfirmation,
              isSubmitting: false,
            }
          : previousPendingConfirmation,
      );
    }
  };

  const inputCapability = detail
    ? taskInputCapability(detail.status)
    : taskInputCapability("");
  const followUpAction = detail
    ? taskFollowUpAction(detail.status)
    : taskFollowUpAction("");

  const handleFollowUpSubmit = async (value: string) => {
    if (!detail) {
      return;
    }

    if (followUpAction.kind === "none") {
      return;
    }

    setIsSubmittingFollowUp(true);

    try {
      if (followUpAction.kind === "write_live_input") {
        await writeTaskInput(detail.id, value);
        scheduleTaskTranscriptSync(detail.id);
      } else {
        const task = await continueTask(detail.id, value);

        setTasks((current) => mergeTaskList(
          current.filter((item) => item.id !== detail.id),
          task,
        ));
        setSelectedTask(task.id);
        await refresh();
      }
      setFollowUpInput("");
    } catch (error) {
      reportError(error, "task follow-up");
    } finally {
      setIsSubmittingFollowUp(false);
    }
  };

  // handleCopy removed — not used in chat view

  return (
    <>
      <div className="space-y-6">
        {/* No header — tasks are selected from sidebar, new task from "+" menu */}

        {showNewTask && (
          <NewTaskModal
            agents={agents}
            config={config}
            onSpawn={handleSpawn}
            onClose={() => setShowNewTask(false)}
          />
        )}

        {showHandoff && detail && (
          <HandoffModal
            agents={agents}
            sourceTask={detail}
            targetAgent={handoffAgent}
            onChangeTarget={setHandoffAgent}
            onClose={() => setShowHandoff(false)}
            onConfirm={() => void handleHandoff()}
          />
        )}

        {/* Chat layout: no task selected = empty state, task selected = chat */}
        {!detail ? (
          <div className="flex h-[calc(100vh-12rem)] items-center justify-center">
            <div className="text-center">
              <p className="text-sm text-text-muted">
                {tasks.length === 0 ? "No tasks yet" : "Select a task from the sidebar"}
              </p>
              <p className="mt-1 text-xs text-text-muted">
                Click "+ New Task" or use the sidebar to start
              </p>
            </div>
          </div>
        ) : (
          <div className="flex flex-col -mx-6 -mb-6" style={{ height: "calc(100vh - 7rem)" }}>
            {/* Compact header bar */}
            <div className="flex items-center justify-between border-b border-border px-4 py-2 shrink-0">
              <div className="flex items-center gap-3 min-w-0">
                <StatusBadge status={displayStatus} />
                <span className="truncate text-sm text-text-primary font-medium max-w-[200px]" title={detail.title}>
                  {detail.title}
                </span>
                <span className="text-xs text-text-muted">{detail.agent}</span>
                {detail.status === "running" && detail.started_at && (
                  <ElapsedTimer startedAt={detail.started_at} />
                )}
              </div>
              <div className="flex items-center gap-1.5 shrink-0">
                {/* Auto-run toggle */}
                {detail.status === "running" && (
                  <button
                    onClick={async () => {
                      try {
                        await setTaskAutoRun(detail.id, !detail.auto_run);
                        setTasks((prev) =>
                          prev.map((t) =>
                            t.id === detail.id ? { ...t, auto_run: !t.auto_run } : t
                          )
                        );
                      } catch { /* ignore */ }
                    }}
                    className={`rounded-full border px-3 py-1 text-xs transition-colors ${
                      detail.auto_run
                        ? "border-accent/30 bg-accent/10 text-accent"
                        : "border-border bg-surface-2 text-text-muted"
                    }`}
                    title={detail.auto_run ? "Auto-run: agent proceeds without pausing" : "Checkpointed: agent pauses at breakpoints"}
                  >
                    {detail.auto_run ? "Auto-run" : "Checkpointed"}
                  </button>
                )}
                {detail.status === "running" && (
                  <button
                    onClick={() => handleCancel(detail.id)}
                    className="rounded-lg border border-danger/20 bg-danger/10 px-2.5 py-1 text-xs text-danger transition-colors hover:bg-danger/20"
                  >
                    Cancel
                  </button>
                )}
                {detail.status === "Ready for PR" && (
                  <button
                    onClick={() => void handleCreatePullRequest(detail)}
                    disabled={resolutionAction !== ""}
                    className="rounded-lg bg-accent px-2.5 py-1 text-xs text-surface-0 transition-colors hover:bg-accent-hover disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    {resolutionAction === "create_pr" ? "Publishing..." : "Create PR"}
                  </button>
                )}
                {workspacePath && (
                  <button
                    onClick={handleOpenWorkspace}
                    className="rounded-lg border border-border bg-surface-2 px-2.5 py-1 text-xs text-text-secondary transition-colors hover:bg-surface-3"
                  >
                    Open in IDE
                  </button>
                )}
                <button
                  onClick={() => {
                    const fallback = agents.find((a) => a.kind !== detail.agent)?.kind ?? "codex";
                    setHandoffAgent(fallback);
                    setShowHandoff(true);
                  }}
                  className="rounded-lg border border-border bg-surface-2 px-2.5 py-1 text-xs text-text-secondary transition-colors hover:bg-surface-3"
                >
                  Hand off
                </button>
                {detail.status !== "running" && (
                  <button
                    onClick={() => handleArchive(detail.id)}
                    className="rounded-lg border border-border bg-surface-2 px-2.5 py-1 text-xs text-text-secondary transition-colors hover:bg-surface-3"
                  >
                    Archive
                  </button>
                )}
              </div>
            </div>

          {executionLadder && <ExecutionStatusLadder ladder={executionLadder} />}

          {/* Chat messages area */}
          <div
            ref={outputRef}
            className="flex flex-1 flex-col justify-end overflow-y-auto px-6 py-4"
          >
            <div className="space-y-3">
              {initialBrief !== "" && (
                <div className="flex justify-end">
                  <div className="max-w-[70%] rounded-2xl rounded-tr-sm bg-accent/10 px-4 py-3">
                    <p className="whitespace-pre-wrap text-sm text-text-primary">{initialBrief}</p>
                  </div>
                </div>
              )}

              <ChatView
                task={detail}
                emptyMessage="No transcript yet."
              />
            </div>
          </div>

          {/* Input area at bottom */}
          <div className="shrink-0">
            {adoptResolution && detail.status !== "running" && (
              <div className="border-t border-border bg-surface-1/50 px-4 pt-3">
                <div className="mb-3 rounded-xl border border-border bg-surface-0 px-4 py-3">
                  <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
                    <div>
                      <p className="text-[11px] uppercase tracking-wider text-text-muted">
                        Adopt Resolution
                      </p>
                      <p className="mt-1 text-xs text-text-muted">
                        {adoptResolution.mode === "drift"
                          ? `Resolve drift for ${adoptResolution.decisionID} with re-baseline, reopen, or waive.`
                          : `Resolve staleness for ${adoptResolution.decisionID} with measure, waive, deprecate, or reopen.`}
                      </p>
                    </div>

                    <div className="flex flex-wrap gap-2">
                      {adoptResolution.mode === "drift" && (
                        <button
                          onClick={() => void handleRebaseline(adoptResolution)}
                          disabled={resolutionAction !== ""}
                          className="rounded-lg border border-border bg-surface-2 px-2.5 py-1 text-xs text-text-secondary transition-colors hover:bg-surface-3 disabled:opacity-50"
                        >
                          {resolutionAction === "baseline" ? "Re-baselining..." : "Re-baseline"}
                        </button>
                      )}
                      {adoptResolution.mode === "stale" && (
                        <button
                          onClick={() => void handleMeasure(adoptResolution, detail)}
                          disabled={resolutionAction !== ""}
                          className="rounded-lg border border-border bg-surface-2 px-2.5 py-1 text-xs text-text-secondary transition-colors hover:bg-surface-3 disabled:opacity-50"
                        >
                          {resolutionAction === "measure" ? "Measuring..." : "Measure"}
                        </button>
                      )}
                      <button
                        onClick={() => void handleWaive(adoptResolution, detail)}
                        disabled={resolutionAction !== ""}
                        className="rounded-lg border border-border bg-surface-2 px-2.5 py-1 text-xs text-text-secondary transition-colors hover:bg-surface-3 disabled:opacity-50"
                      >
                        {resolutionAction === "waive" ? "Waiving..." : "Waive"}
                      </button>
                      {adoptResolution.mode === "stale" && (
                        <button
                          onClick={() => void handleDeprecate(adoptResolution, detail)}
                          disabled={resolutionAction !== ""}
                          className="rounded-lg border border-danger/20 bg-danger/10 px-2.5 py-1 text-xs text-danger transition-colors hover:bg-danger/20 disabled:opacity-50"
                        >
                          {resolutionAction === "deprecate" ? "Deprecating..." : "Deprecate"}
                        </button>
                      )}
                      <button
                        onClick={() => void handleReopen(adoptResolution, detail)}
                        disabled={resolutionAction !== ""}
                        className="rounded-lg border border-warning/20 bg-warning/10 px-2.5 py-1 text-xs text-warning transition-colors hover:bg-warning/20 disabled:opacity-50"
                      >
                        {resolutionAction === "reopen" ? "Reopening..." : "Reopen"}
                      </button>
                    </div>
                  </div>
                  {taskActionMessage.trim() !== "" && (
                    <p className="mt-2 text-xs text-success">{taskActionMessage}</p>
                  )}
                </div>
              </div>
            )}

            <ChatInput
              agentLabel={detail.agent}
              disabled={inputCapability.kind === "unavailable"}
              isSubmitting={isSubmittingFollowUp}
              placeholder={inputCapability.placeholder}
              value={followUpInput}
              onChange={setFollowUpInput}
              onSubmit={handleFollowUpSubmit}
            />
          </div>
        </div>
        )}
      </div>

      {pendingConfirmation && (
        <IrreversibleActionDialog
          model={pendingConfirmation.model}
          reason={pendingConfirmation.reason}
          isSubmitting={pendingConfirmation.isSubmitting}
          onReasonChange={(value) =>
            setPendingConfirmation((currentPendingConfirmation) =>
              currentPendingConfirmation
                ? {
                    ...currentPendingConfirmation,
                    reason: value,
                  }
                : currentPendingConfirmation,
            )
          }
          onCancel={handleCancelConfirmation}
          onConfirm={() => void handleConfirmAction()}
        />
      )}
    </>
  );
}

function NewTaskModal({
  agents,
  config,
  onSpawn,
  onClose,
}: {
  agents: InstalledAgent[];
  config: DesktopConfig | null;
  onSpawn: (agent: string, prompt: string, worktree: boolean, branch: string) => void;
  onClose: () => void;
}) {
  const [presetName, setPresetName] = useState("");
  const [agent, setAgent] = useState(config?.default_agent || "claude");
  const [prompt, setPrompt] = useState("");
  const [useWorktree, setUseWorktree] = useState(config?.default_worktree ?? true);
  const [branch, setBranch] = useState("");

  useEffect(() => {
    if (!config) {
      return;
    }

    setAgent(config.default_agent);
    setUseWorktree(config.default_worktree);
  }, [config]);

  const presets = config?.agent_presets ?? [];

  const handlePresetChange = (value: string) => {
    setPresetName(value);

    const preset = presets.find((item) => item.name === value);
    if (!preset) {
      return;
    }

    setAgent(preset.agent_kind);
  };

  const handleSubmit = () => {
    if (!prompt.trim()) {
      return;
    }

    const branchName = useWorktree ? branch.trim() || `haft-task-${Date.now()}` : "";
    onSpawn(agent, prompt.trim(), useWorktree, branchName);
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
      <div className="max-h-[80vh] w-[720px] overflow-y-auto rounded-2xl border border-border bg-surface-1">
        <div className="flex items-center justify-between border-b border-border px-6 py-4">
          <div>
            <h3 className="text-lg font-semibold">New Task</h3>
            <p className="mt-1 text-xs text-text-muted">
              Start from a preset or launch a one-off task with explicit workspace control.
            </p>
          </div>
          <button
            onClick={onClose}
            className="text-text-muted transition-colors hover:text-text-primary"
          >
            x
          </button>
        </div>

        <div className="space-y-4 px-6 py-5">
          {presets.length > 0 && (
            <div className="space-y-1">
              <label className="text-sm text-text-secondary">Preset</label>
              <select
                value={presetName}
                onChange={(event) => handlePresetChange(event.target.value)}
                className="w-full rounded-lg border border-border bg-surface-2 px-3 py-2 text-sm text-text-primary"
              >
                <option value="">Manual selection</option>
                {presets.map((preset) => (
                  <option key={`${preset.name}-${preset.role}`} value={preset.name}>
                    {preset.name} {preset.model ? `(${preset.model})` : ""}
                  </option>
                ))}
              </select>
            </div>
          )}

          {/* Project: tasks run in the active project. Switch projects from the sidebar. */}

          <div className="rounded-xl border border-border bg-surface-2/40 px-4 py-3">
            <div className="flex flex-wrap items-center gap-3 text-sm">
              <span className="text-text-muted">Run in</span>

              <button
                onClick={() => setUseWorktree(true)}
                className={`rounded border px-3 py-1 text-xs transition-colors ${
                  useWorktree
                    ? "border-accent/30 bg-accent/10 text-accent"
                    : "border-border bg-surface-2 text-text-muted"
                }`}
              >
                Worktree
              </button>

              <button
                onClick={() => setUseWorktree(false)}
                className={`rounded border px-3 py-1 text-xs transition-colors ${
                  !useWorktree
                    ? "border-accent/30 bg-accent/10 text-accent"
                    : "border-border bg-surface-2 text-text-muted"
                }`}
              >
                Project folder
              </button>

              {useWorktree && (
                <>
                  <span className="text-text-muted">branch</span>
                  <input
                    value={branch}
                    onChange={(event) => setBranch(event.target.value)}
                    placeholder="auto-generated"
                    className="w-44 rounded border border-border bg-surface-2 px-2 py-1 font-mono text-xs text-text-primary"
                  />
                </>
              )}
            </div>
          </div>

          <div className="space-y-1">
            <label className="text-sm text-text-secondary">Prompt</label>
            <textarea
              value={prompt}
              onChange={(event) => setPrompt(event.target.value)}
              placeholder="Describe the task. Haft will keep runtime state durable and pass the exact brief through to the agent."
              rows={8}
              className="w-full resize-none rounded-xl border border-border bg-surface-0 px-4 py-3 text-sm text-text-primary focus:border-accent/50 focus:outline-none"
            />
          </div>

          <div className="space-y-1">
            <label className="text-sm text-text-secondary">Agent</label>
            <select
              value={agent}
              onChange={(event) => setAgent(event.target.value)}
              className="w-full rounded-lg border border-border bg-surface-2 px-3 py-2 text-sm text-text-primary"
            >
              {agents.map((item) => (
                <option key={item.kind} value={item.kind}>
                  {item.name} ({item.version})
                </option>
              ))}

              {agents.length === 0 && (
                <>
                  <option value="claude">Claude Code</option>
                  <option value="codex">Codex</option>
                </>
              )}
            </select>
          </div>
        </div>

        <div className="flex items-center justify-end gap-3 border-t border-border px-6 py-4">
          <button
            onClick={onClose}
            className="px-4 py-2 text-sm text-text-secondary transition-colors hover:text-text-primary"
          >
            Cancel
          </button>

          <button
            onClick={handleSubmit}
            disabled={!prompt.trim()}
            className="rounded-full bg-accent px-4 py-2 text-sm text-surface-0 transition-colors hover:bg-accent-hover disabled:opacity-50"
          >
            Create & Run
          </button>
        </div>
      </div>
    </div>
  );
}

function ElapsedTimer({ startedAt }: { startedAt: string }) {
  const [, setTick] = useState(0);

  useEffect(() => {
    const interval = window.setInterval(() => setTick((t) => t + 1), 1000);
    return () => window.clearInterval(interval);
  }, []);

  return (
    <span className="text-xs text-text-muted">
      {elapsedSince(startedAt)}
    </span>
  );
}

function elapsedSince(isoDate: string): string {
  try {
    const start = new Date(isoDate).getTime();
    const now = Date.now();
    const seconds = Math.floor((now - start) / 1000);
    if (seconds < 60) return `${seconds}s`;
    const minutes = Math.floor(seconds / 60);
    if (minutes < 60) return `${minutes}m ${seconds % 60}s`;
    const hours = Math.floor(minutes / 60);
    return `${hours}h ${minutes % 60}m`;
  } catch {
    return "";
  }
}

function StatusBadge({ status }: { status: string }) {
  const styles: Record<string, string> = {
    Planned: "border-border bg-surface-2 text-text-muted",
    running: "border-blue-500/20 bg-blue-500/10 text-blue-400",
    Running: "border-blue-500/20 bg-blue-500/10 text-blue-400",
    checkpointed: "border-warning/20 bg-warning/10 text-warning",
    idle: "border-accent/30 bg-accent/10 text-accent",
    Verifying: "border-warning/20 bg-warning/10 text-warning",
    completed: "border-success/20 bg-success/10 text-success",
    failed: "border-danger/20 bg-danger/10 text-danger",
    blocked: "border-danger/20 bg-danger/10 text-danger",
    cancelled: "border-border bg-surface-2 text-text-muted",
    interrupted: "border-warning/20 bg-warning/10 text-warning",
    "Ready for PR": "border-accent/30 bg-accent/10 text-accent",
    "Needs attention": "border-warning/20 bg-warning/10 text-warning",
  };

  return (
    <span
      className={`rounded-full border px-2 py-0.5 text-xs ${
        styles[status] ?? "border-border bg-surface-2 text-text-muted"
      }`}
    >
      {status}
    </span>
  );
}

function ExecutionStatusLadder({ ladder }: { ladder: TaskExecutionLadder }) {
  return (
    <div className="border-b border-border bg-surface-0/60 px-4 py-3 shrink-0">
      <div className="flex flex-col gap-2">
        <div className="flex flex-wrap items-center gap-2">
          {ladder.steps.map((step, index) => (
            <div key={step.label} className="flex items-center gap-2">
              <span
                className={`rounded-full border px-2.5 py-1 text-[11px] uppercase tracking-wide ${executionStepClassName(step)}`}
              >
                {step.label}
              </span>
              {index < ladder.steps.length - 1 && (
                <span className="h-px w-4 bg-border/80" aria-hidden="true" />
              )}
            </div>
          ))}
        </div>
        <p className="text-xs text-text-muted">{ladder.summary}</p>
        {ladder.rawStatus !== ladder.currentLabel && (
          <p className="text-[11px] text-text-muted">
            Raw task status: {ladder.rawStatus}
          </p>
        )}
      </div>
    </div>
  );
}

function executionStepClassName(step: ExecutionLadderStep): string {
  if (step.state === "upcoming") {
    return "border-border bg-surface-2 text-text-muted";
  }

  if (step.label === "Ready for PR") {
    return step.state === "current"
      ? "border-accent/30 bg-accent/10 text-accent"
      : "border-accent/20 bg-accent/5 text-accent";
  }

  if (step.label === "Needs attention") {
    return step.state === "current"
      ? "border-warning/20 bg-warning/10 text-warning"
      : "border-warning/20 bg-warning/5 text-warning";
  }

  if (step.label === "Running") {
    return step.state === "current"
      ? "border-blue-500/20 bg-blue-500/10 text-blue-400"
      : "border-blue-500/20 bg-blue-500/5 text-blue-300";
  }

  if (step.label === "Verifying") {
    return step.state === "current"
      ? "border-warning/20 bg-warning/10 text-warning"
      : "border-warning/20 bg-warning/5 text-warning";
  }

  return step.state === "current"
    ? "border-accent/20 bg-accent/10 text-accent"
    : "border-accent/20 bg-accent/5 text-accent";
}

function HandoffModal({
  agents,
  sourceTask,
  targetAgent,
  onChangeTarget,
  onClose,
  onConfirm,
}: {
  agents: InstalledAgent[];
  sourceTask: TaskState;
  targetAgent: string;
  onChangeTarget: (agent: string) => void;
  onClose: () => void;
  onConfirm: () => void;
}) {
  const availableAgents = agents.filter((agent) => agent.kind !== sourceTask.agent);

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 px-4">
      <div className="w-[520px] rounded-2xl border border-border bg-surface-1">
        <div className="flex items-center justify-between border-b border-border px-6 py-4">
          <div>
            <h3 className="text-lg font-semibold">Hand Off Task</h3>
            <p className="mt-1 text-xs text-text-muted">
              Preserve the original brief and bounded output tail, then continue with a different agent.
            </p>
          </div>
          <button
            onClick={onClose}
            className="text-text-muted transition-colors hover:text-text-primary"
          >
            x
          </button>
        </div>

        <div className="space-y-4 px-6 py-5">
          <div className="rounded-xl border border-border bg-surface-2/40 px-4 py-3">
            <p className="text-[11px] uppercase tracking-wider text-text-muted">Source task</p>
            <p className="mt-2 text-sm text-text-primary">{sourceTask.title}</p>
            <div className="mt-2 flex flex-wrap items-center gap-2 text-xs text-text-muted">
              <span>{sourceTask.agent}</span>
              <span>{sourceTask.status}</span>
              {sourceTask.branch && <span className="font-mono">{sourceTask.branch}</span>}
            </div>
          </div>

          <div className="space-y-1">
            <label className="text-sm text-text-secondary">Target agent</label>
            <select
              value={targetAgent}
              onChange={(event) => onChangeTarget(event.target.value)}
              className="w-full rounded-lg border border-border bg-surface-2 px-3 py-2 text-sm text-text-primary"
            >
              {availableAgents.map((agent) => (
                <option key={agent.kind} value={agent.kind}>
                  {agent.name} ({agent.version || agent.kind})
                </option>
              ))}

              {availableAgents.length === 0 && (
                <>
                  <option value="claude">Claude Code</option>
                  <option value="codex">Codex</option>
                </>
              )}
            </select>
          </div>

          {sourceTask.status === "running" && (
            <div className="rounded-xl border border-warning/20 bg-warning/10 px-4 py-3 text-sm text-warning">
              The source task is still marked running. The handoff will preserve context, but you should
              reconcile workspace state before treating the previous output as final.
            </div>
          )}
        </div>

        <div className="flex items-center justify-end gap-3 border-t border-border px-6 py-4">
          <button
            onClick={onClose}
            className="px-4 py-2 text-sm text-text-secondary transition-colors hover:text-text-primary"
          >
            Cancel
          </button>

          <button
            onClick={onConfirm}
            className="rounded-full bg-accent px-4 py-2 text-sm text-surface-0 transition-colors hover:bg-accent-hover"
          >
            Create handoff
          </button>
        </div>
      </div>
    </div>
  );
}

function mergeTaskList(current: TaskState[], next: TaskState): TaskState[] {
  const existingIndex = current.findIndex((task) => task.id === next.id);

  if (existingIndex === -1) {
    return [next, ...current];
  }

  const merged = [...current];
  merged[existingIndex] = {
    ...merged[existingIndex],
    ...next,
  };

  return merged;
}

function mergeTaskTranscript(
  task: TaskState,
  transcript: ChatTranscriptState,
): TaskState {
  const nextOutput = transcript.output || task.output;
  const nextRawOutput = transcript.raw_output || nextOutput || task.raw_output;
  const nextChatBlocks = transcript.chat_blocks.length > 0 || task.chat_blocks.length === 0
    ? transcript.chat_blocks
    : task.chat_blocks;
  const nextStatus = transcript.status || task.status;
  const shouldReplaceError = transcript.status !== "" || transcript.error_message !== "";

  return {
    ...task,
    output: nextOutput,
    raw_output: nextRawOutput,
    chat_blocks: nextChatBlocks,
    status: nextStatus,
    error_message: shouldReplaceError
      ? transcript.error_message
      : task.error_message,
  };
}

function getAdoptResolutionContext(
  task: TaskState | null,
): AdoptResolutionContext | null {
  if (!task) {
    return null;
  }

  const prompt = task.prompt ?? "";
  const findingID = taskPromptMetaValue(prompt, "Finding ID");
  const decisionID = taskPromptMetaValue(prompt, "Decision ID");
  const mode = prompt.includes("## Adopt Drift Finding")
    ? "drift"
    : prompt.includes("## Adopt Stale Finding")
      ? "stale"
      : null;

  if (!findingID || !decisionID || !mode) {
    return null;
  }

  return { findingID, decisionID, mode };
}

function taskPromptMetaValue(prompt: string, label: string): string {
  const prefix = `${label.trim()}:`;
  const lines = prompt.split("\n");

  for (const line of lines) {
    const trimmed = line.trim();

    if (!trimmed.startsWith(prefix)) {
      continue;
    }

    return trimmed.slice(prefix.length).trim();
  }

  return "";
}

function promptForAdoptReason(task: TaskState): string {
  if (typeof window === "undefined") {
    return "";
  }

  const defaultReason = taskPromptMetaValue(task.prompt, "Reason");
  const promptMessage = "Waive will extend this DecisionRecord by 90 days.\n\nEnter justification:";
  const response = window.prompt(promptMessage, defaultReason);

  return response ? response.trim() : "";
}

function buildPullRequestMessage(result: PullRequestResult): string {
  const lines = compactNonEmptyStrings([
    result.draft_created && result.url ? `Draft PR created: ${result.url}` : "",
    result.copied_to_clipboard ? "PR body copied to the clipboard for manual creation." : "",
    result.warnings.join("\n"),
  ]);

  return lines.join("\n");
}

function promptForMeasureFindings(task: TaskState): string {
  if (typeof window === "undefined") {
    return "";
  }

  const defaultFindings = taskPromptMetaValue(task.prompt, "Reason");
  const response = window.prompt(
    "Measure will record new verification evidence on this DecisionRecord.\n\nDescribe what was verified and what actually happened:",
    defaultFindings,
  );

  return response ? response.trim() : "";
}

function promptForMeasureVerdict(): string {
  if (typeof window === "undefined") {
    return "";
  }

  const response = window.prompt(
    "Enter measurement verdict: accepted, partial, or failed.",
    "accepted",
  );
  const verdict = response ? response.trim().toLowerCase() : "";
  const validVerdicts = new Set(["accepted", "partial", "failed"]);

  if (verdict === "") {
    return "";
  }

  if (validVerdicts.has(verdict)) {
    return verdict;
  }

  window.alert("Verdict must be one of: accepted, partial, failed.");
  return "";
}

function compactNonEmptyStrings(values: string[]): string[] {
  return values.map((value) => value.trim()).filter(Boolean);
}

// parsePromptSections kept for future use if we add collapsible brief sections to chat
// function parsePromptSections(prompt: string): PromptSection[] { ... }
