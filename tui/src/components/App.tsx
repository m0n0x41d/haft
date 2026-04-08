// L5: App Shell — wires protocol, state, scroll, and components.

import React, { useReducer, useEffect, useCallback, useRef, useState, useMemo } from "react"
import { Box, Text, useApp, useStdout } from "ink"
import { useInput } from "../hooks/useInput.js"
import { trace } from "../debug.js"
import type { EventEmitter } from "node:events"
import type { JsonRpcClient } from "../protocol/client.js"
import type { SessionListResponse, ModelListResponse, FileListResponse, MsgUpdateParams } from "../protocol/types.js"
import { initialState, reducer } from "../state/store.js"
import { buildTranscript } from "../state/transcript.js"
import { useScroll } from "../scroll/useScroll.js"
import { useMeasuredTranscript } from "../scroll/useMeasuredTranscript.js"
// highlight.ts no longer has a subscriber pattern — highlighting applies
// lazily on natural re-renders (scroll, new message, etc.)
import type { InputEvent } from "../terminal/input.js"
import { INITIAL_SELECTION, reduceSelection, hasSelection, normalizedRange, type SelectionState } from "../selection/state.js"
import { copyToClipboard } from "../selection/clipboard.js"
import { extractSelection, type ViewportLayout } from "../selection/extract.js"
import { TranscriptViewport } from "./TranscriptViewport.js"
import { StatusBar } from "./StatusBar.js"
import { InputArea, type InputAreaHandle } from "./InputArea.js"
import { PermissionDialog } from "./PermissionDialog.js"
import { QuestionDialog } from "./QuestionDialog.js"
import { Picker, type PickerItem } from "./Picker.js"
import { Attachments } from "./Attachments.js"
import type { AttachmentItem } from "./attachmentLayout.js"
import { computeBottomRows, computeChatHeight, estimateInputRows } from "./appLayout.js"
import { shouldFinalizeStreaming } from "./streamLifecycle.js"
import { createStreamUpdateBuffer } from "./streamUpdateBuffer.js"
import {
  createPromptHistoryEntry,
  createPromptSubmission,
  drainPromptSubmissions,
  expandPromptSubmissionText,
  leadingSlashCommand,
  restoreQueuedSubmission,
  shouldResumeQueuedReplayAfterCommandResolution,
  shouldResumeQueuedReplayAfterPickerCancel,
  submissionTexts,
  type PromptSubmission,
} from "./promptSubmission.js"
import {
  collapsePastedText,
  filterReferencedCollapsedPastes,
  type CollapsedPaste,
} from "./pastedText.js"

type PickerMode = null | "sessions" | "models" | "files" | "commands"

const SLASH_COMMANDS: PickerItem[] = [
  { id: "/compact", label: "/compact", desc: "Compress context window" },
  { id: "/model", label: "/model", desc: "Switch model" },
  { id: "/resume", label: "/resume", desc: "Resume previous session" },
  { id: "/help", label: "/help", desc: "Show commands" },
  { id: "/frame", label: "/frame", desc: "Frame an engineering problem" },
  { id: "/explore", label: "/explore", desc: "Explore solution variants" },
  { id: "/decide", label: "/decide", desc: "Finalize a decision" },
  { id: "/status", label: "/status", desc: "Decision dashboard" },
  { id: "/problems", label: "/problems", desc: "List active problems" },
  { id: "/refresh", label: "/refresh", desc: "Check for stale decisions" },
  { id: "/note", label: "/note", desc: "Record a micro-decision" },
  { id: "/search", label: "/search", desc: "Search past decisions" },
]

interface AppProps {
  client: JsonRpcClient
  inputEvents: EventEmitter  // L1 mouse/scroll events from terminal router
}

export function App({ client, inputEvents }: AppProps) {
  const [state, dispatch] = useReducer(reducer, initialState())
  const { exit } = useApp()
  const { stdout } = useStdout()
  const width = stdout?.columns ?? 80
  const height = stdout?.rows ?? 24

  const [pickerMode, setPickerMode] = useState<PickerMode>(null)
  const [pickerItems, setPickerItems] = useState<PickerItem[]>([])
  const [toolHistoryExpanded, setToolHistoryExpanded] = useState(false)
  const respondRef = useRef<((result: unknown) => void) | null>(null)
  const inputRef = useRef<InputAreaHandle>(null)
  const [queuedMessages, setQueuedMessages] = useState<PromptSubmission[]>([])
  const [attachments, setAttachments] = useState<AttachmentItem[]>([])
  const [draftPastes, setDraftPastes] = useState<CollapsedPaste[]>([])
  const [attachmentSelection, setAttachmentSelection] = useState(false)
  const [inputRows, setInputRows] = useState(() => estimateInputRows(""))
  const nextAttachmentId = useRef(1)
  const nextPasteId = useRef(1)
  const phaseRef = useRef(state.phase)
  phaseRef.current = state.phase
  const queuedMessageTexts = useMemo(
    () => submissionTexts(queuedMessages),
    [queuedMessages],
  )

  // Stable ref to replay the drained queued prefix after streaming finishes.
  const replayQueueRef = useRef<(submissions: readonly PromptSubmission[]) => void>(() => {})
  const resumeQueuedOnPickerCancelRef = useRef(false)

  // Syntax highlighting applies lazily — no forced re-render on load.
  // Components pick up highlighting on their next natural render cycle.

  // Force rerender for Ctrl+L screen redraw
  const [, setRedrawTick] = useState(0)

  // --- Selection state (mouse drag → copy to clipboard) ---
  const selRef = useRef<SelectionState>(INITIAL_SELECTION)
  const layoutRef = useRef<ViewportLayout>({
    chatHeight: 0, atBottom: true,
    visibleWindow: {
      start: 0,
      end: 0,
      viewTop: 0,
      viewBottom: 0,
      cropTop: 0,
      topSpacer: 0,
      bottomSpacer: 0,
      totalLines: 0,
    },
    entryHeights: [],
    entryOffsets: [0],
    transcript: [],
  })

  // --- L1: Paste events (bypass Ink's input system, one render per paste) ---
  useEffect(() => {
    const handler = (text: string) => {
      if (!text) return
      const collapsed = collapsePastedText(text, nextPasteId.current)

      if (collapsed.pastes.length > 0) {
        nextPasteId.current += collapsed.pastes.length
        setDraftPastes((current) => [...current, ...collapsed.pastes])
      }

      if (inputRef.current) {
        inputRef.current.insert(collapsed.displayText)
      }
    }
    inputEvents.on("paste", handler)
    return () => { inputEvents.off("paste", handler) }
  }, [inputEvents])

  const dispatchStreamUpdate = useCallback((params: MsgUpdateParams, reason: string) => {
    trace("stream_update_flushed", {
      reason,
      streaming: params.streaming,
      textLength: params.text.length,
      thinkingLength: params.thinking?.length ?? 0,
      toolCount: params.tools?.length ?? 0,
    })
    dispatch({ type: "msg.update", params })
  }, [])
  const streamUpdateBufferRef = useRef(createStreamUpdateBuffer({
    onFlush: dispatchStreamUpdate,
  }))

  // --- L3: Transcript ---
  const transcript = useMemo(() => buildTranscript({
    messages: state.messages,
    streaming: state.phase === "streaming",
    streamingMsgId: state.streamingMsgId,
    thinkExpanded: state.thinkExpanded,
    error: state.error,
    model: state.session.model,
  }), [state.messages, state.phase, state.streamingMsgId, state.thinkExpanded, state.error, state.session.model])

  // --- L2: Scroll (measured line-based) ---
  const showInput = !pickerMode && (state.phase === "input" || state.phase === "streaming")
  const bottomRows = useMemo(() => computeBottomRows({
    width,
    queuedMessages: queuedMessageTexts,
    attachments,
    attachmentSelection,
    inputRows,
    showInput,
  }), [width, queuedMessageTexts, attachments, attachmentSelection, inputRows, showInput])
  const chatHeight = computeChatHeight(height, bottomRows)
  const { entryHeights, measureRef } = useMeasuredTranscript(
    transcript,
    width,
    toolHistoryExpanded,
  )
  const { state: scrollState, scroll, unreadBelow, entryOffsets, visibleWindow: vw, isAtBottom: atBottom } = useScroll(
    inputEvents,
    entryHeights,
    chatHeight,
  )

  // Visible entries based on scroll
  const visibleEntries = useMemo(() => {
    return transcript.slice(vw.start, vw.end)
  }, [transcript, vw.start, vw.end])

  // --- Selection: keep layout ref current for mouse event handler ---
  layoutRef.current = { chatHeight, atBottom, visibleWindow: vw, entryHeights, entryOffsets, transcript }

  useEffect(() => {
    const handler = (ev: InputEvent) => {
      if (ev.type === "mouseClick" && ev.button === 0) {
        selRef.current = reduceSelection(selRef.current, { type: "mouseDown", col: ev.col, row: ev.row })
      } else if (ev.type === "mouseDrag" && ev.button === 0) {
        selRef.current = reduceSelection(selRef.current, { type: "mouseDrag", col: ev.col, row: ev.row })
      } else if (ev.type === "mouseRelease") {
        const sel = selRef.current
        if (hasSelection(sel)) {
          const range = normalizedRange(sel)!
          const text = extractSelection(range.start.row, range.end.row, layoutRef.current)
          if (text.trim()) {
            copyToClipboard(text, process.stderr)
            dispatch({ type: "set.notification", text: "Copied to clipboard" })
          }
        }
        selRef.current = INITIAL_SELECTION
      }
    }
    inputEvents.on("input", handler)
    return () => { inputEvents.off("input", handler) }
  }, [inputEvents])

  const resumeQueuedMessages = useCallback(() => {
    setQueuedMessages((submissions) => {
      const drained = drainPromptSubmissions(submissions)

      if (drained.replay.length > 0) {
        trace("queue_replay_start", {
          count: drained.replay.length,
        })
        const replay = drained.replay

        setTimeout(() => replayQueueRef.current(replay), 100)
      }

      return drained.remaining
    })
  }, [])

  const flushBufferedStream = useCallback((
    reason: string,
    overrides?: Partial<MsgUpdateParams>,
  ) => {
    return streamUpdateBufferRef.current.flush(reason, overrides)
  }, [])
  const replaceBufferedStream = useCallback((
    params: MsgUpdateParams,
    reason: string,
  ) => {
    streamUpdateBufferRef.current.replace(params, reason)
  }, [])
  const finishStreaming = useCallback((
    reason: string,
    options: { resumeQueue: boolean },
  ) => {
    const flushedPending = flushBufferedStream(reason, { streaming: false })
    const shouldFinalize = shouldFinalizeStreaming(
      phaseRef.current,
      flushedPending,
    )

    if (!shouldFinalize) {
      return
    }

    trace("stream_finished", {
      reason,
      flushedPending,
      resumeQueue: options.resumeQueue,
    })
    dispatch({ type: "coord.done" })

    if (options.resumeQueue) {
      resumeQueuedMessages()
    }
  }, [flushBufferedStream, resumeQueuedMessages])

  useEffect(() => {
    return () => {
      streamUpdateBufferRef.current.clear()
    }
  }, [])

  // --- Protocol ---
  useEffect(() => {
    client.setNotificationHandler((method, params) => {
      trace(`notification: ${method}`)
      const p = params as any
      switch (method) {
        case "init":
          dispatch({ type: "init", session: p.session, projectRoot: p.projectRoot, messages: p.messages }); break
        case "msg.update":
          trace("stream_update_received", {
            streaming: p.streaming,
            textLength: p.text.length,
            thinkingLength: p.thinking?.length ?? 0,
            toolCount: p.tools?.length ?? 0,
          })
          if (p.streaming) {
            streamUpdateBufferRef.current.push(p)
            break
          }
          replaceBufferedStream(p, "final_msg_update")
          break
        case "tool.start": dispatch({ type: "tool.start", params: p }); break
        case "tool.progress": dispatch({ type: "tool.progress", params: p }); break
        case "tool.done": dispatch({ type: "tool.done", params: p }); break
        case "token.update": dispatch({ type: "token.update", params: p }); break
        case "session.title": dispatch({ type: "session.title", title: p.title }); break
        case "cycle.update": dispatch({ type: "cycle.update", params: p }); break
        case "subagent.start": dispatch({ type: "subagent.start", params: p }); break
        case "subagent.done": dispatch({ type: "subagent.done", params: p }); break
        case "overseer.alert":
          trace(`overseer.alert alerts=${JSON.stringify(p.alerts).slice(0, 200)}`)
          dispatch({ type: "overseer.alert", params: p })
          trace("overseer.alert dispatch done")
          break
        case "drift.update": dispatch({ type: "drift.update", params: p }); break
        case "lsp.update": dispatch({ type: "lsp.update", params: p }); break
        case "error":
          finishStreaming("stream_error", { resumeQueue: false })
          dispatch({ type: "error", message: p.message })
          break
        case "coord.done":
          finishStreaming("coord_done", { resumeQueue: true })
          break
      }
    })

    client.setRequestHandler((method, params, respond) => {
      const p = params as any
      if (method === "permission.ask") {
        dispatch({ type: "permission.ask", id: 0, toolName: p.toolName, args: p.args, description: p.description, diff: p.diff, adds: p.adds, dels: p.dels })
        respondRef.current = respond
      } else if (method === "question.ask") {
        dispatch({ type: "question.ask", id: 0, question: p.question, options: p.options })
        respondRef.current = respond
      }
    })

  }, [client, finishStreaming, replaceBufferedStream]) // eslint-disable-line react-hooks/exhaustive-deps — handlers registered once

  // Forward terminal resize to backend (also sends initial size on mount)
  useEffect(() => {
    client.send("resize", { width, height })
  }, [client, width, height])

  // --- Notification auto-clear ---
  useEffect(() => {
    if (!state.notification) return
    const timer = setTimeout(() => dispatch({ type: "clear.notification" }), 5000)
    return () => clearTimeout(timer)
  }, [state.notification])

  // --- Handlers ---
  const openFilePicker = useCallback(async () => {
    try {
      const resp = await client.request<FileListResponse>("file.list", { limit: 200 })
      setPickerItems(resp.files.map((f) => ({ id: f.path, label: f.path, desc: formatSize(f.size) })))
      setPickerMode("files")
    } catch (e: any) { dispatch({ type: "error", message: `file list: ${e.message}` }) }
  }, [client])

  const openModelPicker = useCallback(async () => {
    try {
      const resp = await client.request<ModelListResponse>("model.list", {})
      setPickerItems(resp.models.map((m) => ({ id: m.id, label: m.name || m.id, desc: `${m.provider} \u00B7 ${Math.round(m.contextWindow / 1000)}k` })))
      setPickerMode("models")
    } catch (e: any) {
      const shouldResume = resumeQueuedOnPickerCancelRef.current

      resumeQueuedOnPickerCancelRef.current = false
      dispatch({ type: "error", message: `model list: ${e.message}` })

      if (shouldResume) {
        resumeQueuedMessages()
      }
    }
  }, [client, resumeQueuedMessages])

  const openSessionPicker = useCallback(async () => {
    try {
      const resp = await client.request<SessionListResponse>("session.list", { limit: 20 })
      setPickerItems(resp.sessions.map((s) => ({ id: s.id, label: s.title || s.id.slice(0, 8) + "\u2026", desc: s.model })))
      setPickerMode("sessions")
    } catch (e: any) {
      const shouldResume = resumeQueuedOnPickerCancelRef.current

      resumeQueuedOnPickerCancelRef.current = false
      dispatch({ type: "error", message: `session list: ${e.message}` })

      if (shouldResume) {
        resumeQueuedMessages()
      }
    }
  }, [client, resumeQueuedMessages])

  const sendSubmission = useCallback((submission: PromptSubmission) => {
    const expandedText = expandPromptSubmissionText(submission)

    dispatch({ type: "submitted" })
    dispatch({
      type: "msg.update",
      params: {
        id: `user-${Date.now()}`,
        text: expandedText,
        attachments: toMessageAttachments(submission.attachments),
        streaming: false,
      },
    })

    const submitAttachments = submission.attachments.map((attachment) => ({
      name: attachment.name,
      path: attachment.path,
      isImage: attachment.isImage,
      mimeType: attachment.isImage ? "image/*" : undefined,
    }))

    client.send("submit", {
      text: expandedText,
      displayText: submission.text,
      attachments: submitAttachments.length > 0 ? submitAttachments : undefined,
    })
  }, [client])

  const handleSlashCommand = useCallback((
    text: string,
    fromQueuedReplay: boolean,
  ): "unhandled" | "pause" => {
    const cmd = leadingSlashCommand(text)
    const shouldResumeOnCancel =
      fromQueuedReplay &&
      shouldResumeQueuedReplayAfterPickerCancel(text)
    const shouldResumeOnResolution =
      fromQueuedReplay &&
      shouldResumeQueuedReplayAfterCommandResolution(text)

    resumeQueuedOnPickerCancelRef.current = shouldResumeOnCancel

    if (!cmd) {
      return "unhandled"
    }

    switch (cmd) {
      case "/model":
        openModelPicker()
        return "pause"
      case "/resume":
        openSessionPicker()
        return "pause"
      case "/compact":
        Promise.resolve()
          .then(() => client.request("compact", {}))
          .then((r: any) => {
          dispatch({ type: "set.notification", text: `Compacted ${r.before} \u2192 ${r.after} messages` })
          if (shouldResumeOnResolution) {
            resumeQueuedMessages()
          }
        }).catch((e: Error) => {
          dispatch({ type: "error", message: e.message })

          if (shouldResumeOnResolution) {
            resumeQueuedMessages()
          }
        })
        return "pause"
      case "/help":
        setPickerMode("commands")
        setPickerItems(SLASH_COMMANDS)
        return "pause"
      default:
        return "unhandled"
    }
  }, [client, openModelPicker, openSessionPicker, resumeQueuedMessages])

  const replaySubmission = useCallback((submission: PromptSubmission) => {
    const commandResult = handleSlashCommand(submission.text, true)

    if (commandResult === "pause") {
      return true
    }
    sendSubmission(submission)
    return true
  }, [handleSlashCommand, sendSubmission])

  const replayQueuedSubmissions = useCallback((submissions: readonly PromptSubmission[]) => {
    submissions.some((submission) => replaySubmission(submission))
    trace("queue_replay_finish", {
      requested: submissions.length,
      processed: submissions.length > 0 ? 1 : 0,
    })
  }, [replaySubmission])

  const handleSubmit = useCallback((text: string) => {
    trace(`handleSubmit phase=${phaseRef.current} text=${text.slice(0, 40)}`)
    const submission = createPromptSubmission(text, attachments, draftPastes)
    const historyEntry = createPromptHistoryEntry(submission)

    if (phaseRef.current === "streaming") {
      setQueuedMessages((current) => [...current, submission])
      setAttachments([])
      setDraftPastes([])
      setAttachmentSelection(false)
      return historyEntry
    }

    const commandResult = handleSlashCommand(text, false)

    if (commandResult !== "unhandled") {
      return historyEntry
    }

    sendSubmission(submission)
    setAttachments([])
    setDraftPastes([])
    setAttachmentSelection(false)
    return historyEntry
  }, [attachments, draftPastes, handleSlashCommand, sendSubmission])

  const handleRemoveAttachment = useCallback((id: number) => {
    setAttachments((current) => {
      const next = current.filter((item) => item.id !== id)

      if (next.length === 0) {
        setAttachmentSelection(false)
      }

      return next
    })
  }, [])
  const handleInputTextChange = useCallback((text: string) => {
    setDraftPastes((current) => {
      const next = filterReferencedCollapsedPastes(text, current)

      if (sameCollapsedPastes(current, next)) {
        return current
      }

      return next
    })
  }, [])
  replayQueueRef.current = replayQueuedSubmissions

  const handlePermission = useCallback((action: "allow" | "allow_session" | "deny") => {
    const yolo = action === "allow_session"
    respondRef.current?.({ action, yolo }); respondRef.current = null
    dispatch({ type: "permission.replied" })
    if (yolo && !state.yolo) {
      dispatch({ type: "toggle.yolo" })
      client.send("yolo.toggle", { yolo: true })
      dispatch({ type: "set.notification", text: "yolo enabled" })
    }
  }, [client, state.yolo])

  const handleQuestion = useCallback((answer: string) => {
    respondRef.current?.({ answer }); respondRef.current = null
    dispatch({ type: "question.replied" })
  }, [])

  const handlePickerCancel = useCallback(() => {
    const shouldResume = resumeQueuedOnPickerCancelRef.current

    resumeQueuedOnPickerCancelRef.current = false
    setPickerMode(null)

    if (shouldResume) {
      resumeQueuedMessages()
    }
  }, [resumeQueuedMessages])

  const handlePickerSelect = useCallback((item: PickerItem) => {
    const mode = pickerMode
    const shouldResume = resumeQueuedOnPickerCancelRef.current

    resumeQueuedOnPickerCancelRef.current = false

    setPickerMode(null)
    switch (mode) {
      case "models":
        client.request("model.switch", { model: item.id })
          .then(() => {
            if (shouldResume) {
              resumeQueuedMessages()
            }
          })
          .catch((e: any) => {
            dispatch({ type: "error", message: e.message })

            if (shouldResume) {
              resumeQueuedMessages()
            }
          })
        break
      case "sessions":
        client.request("session.resume", { sessionId: item.id })
          .then((r: any) => {
            dispatch({ type: "init", session: r.session, projectRoot: state.projectRoot, messages: r.messages })
            if (shouldResume) {
              resumeQueuedMessages()
            }
          })
          .catch((e: any) => {
            dispatch({ type: "error", message: e.message })

            if (shouldResume) {
              resumeQueuedMessages()
            }
          })
        break
      case "files": {
        const isImg = /\.(png|jpg|jpeg|gif|webp|svg|bmp)$/i.test(item.id)
        const id = nextAttachmentId.current++
        setAttachments((a) => [...a, { id, name: item.id.split("/").pop() || item.id, path: item.id, isImage: isImg }])
        break
      }
      case "commands":
        if (shouldResume) {
          replaySubmission(createPromptSubmission(item.id, []))
          break
        }
        handleSubmit(item.id)
        break
    }
  }, [pickerMode, client, state.projectRoot, handleSubmit, replaySubmission, resumeQueuedMessages])

  // --- Keyboard scroll + global shortcuts ---
  // Our useInput uses useEventCallback internally — handler closures are
  // always fresh (reads latest state/pickerMode), but the listener is
  // registered ONCE and never re-appended. No refs needed.
  useInput((input, key) => {
    if (pickerMode) return

    // Ctrl+C ALWAYS works — never blocked
    if (key.ctrl && input === "c") {
      trace(`ctrl-c phase=${state.phase}`)
      if (state.phase === "streaming") { client.send("cancel", {}); finishStreaming("ctrl_c_cancel", { resumeQueue: false }) }
      else exit()
      return
    }
    // Ctrl+D — exit (same as Ctrl+C when not streaming)
    if (key.ctrl && input === "d") {
      if (state.phase === "streaming") { client.send("cancel", {}); finishStreaming("ctrl_d_cancel", { resumeQueue: false }) }
      else exit()
      return
    }
    // Ctrl+L — redraw screen (clear + force Ink repaint via state change)
    if (key.ctrl && input === "l") {
      stdout?.write("\x1b[2J\x1b[3J\x1b[H")
      setRedrawTick((t) => t + 1)
      return
    }
    if (key.ctrl && input === "q") {
      const newMode = state.mode === "checkpointed" ? "autonomous" : "checkpointed"
      dispatch({ type: "toggle.autonomy" })
      client.send("autonomy.toggle", { mode: newMode })
      dispatch({
        type: "set.notification",
        text: newMode === "autonomous" ? "autonomous mode enabled" : "checkpointed mode enabled",
      })
      return
    }
    if (key.ctrl && input === "y") {
      const nextYolo = !state.yolo
      dispatch({ type: "toggle.yolo" })
      client.send("yolo.toggle", { yolo: nextYolo })
      dispatch({ type: "set.notification", text: nextYolo ? "yolo enabled" : "yolo disabled" })
      return
    }
    if (key.ctrl && input === "m") { openModelPicker(); return }
    if (key.ctrl && input === "o") {
      setToolHistoryExpanded((expanded) => {
        const nextExpanded = !expanded
        dispatch({
          type: "set.notification",
          text: nextExpanded ? "Expanded tool history" : "Collapsed tool history",
        })
        return nextExpanded
      })
      return
    }
    // Escape: cancel streaming, clear error, or clear scroll
    if (key.escape) {
      if (state.error) { dispatch({ type: "clear.error" }); return }
      if (state.phase === "streaming") { client.send("cancel", {}); finishStreaming("escape_cancel", { resumeQueue: false }); return }
      return
    }

    // Keyboard scroll
    if (key.upArrow && key.shift) { scroll({ type: "wheelUp", amount: 3 }); return }
    if (key.downArrow && key.shift) { scroll({ type: "wheelDown", amount: 3 }); return }
    if (key.pageUp) { scroll({ type: "pageUp" }); return }
    if (key.pageDown) { scroll({ type: "pageDown" }); return }
    if (key.home && key.ctrl) { scroll({ type: "home" }); return }
    if (key.end && key.ctrl) { scroll({ type: "end" }); return }

    if (state.phase !== "input" || input === "") {
      if (input === "t") { dispatch({ type: "toggle.think" }); return }
    }
  })

  const showPermission = state.phase === "permission" && state.permissionRequest
  const showQuestion = state.phase === "question" && state.questionRequest

  return (
    <Box flexDirection="column" width={width} height={height}>
      {/* Chat: fixed-height viewport over the cropped mounted transcript slice. */}
      <Box flexDirection="column" height={chatHeight} overflowY="hidden" width={width}>
        {atBottom && <Box flexGrow={1} />}
        <TranscriptViewport
          entries={visibleEntries}
          measureRef={measureRef}
          viewport={vw}
          toolHistoryExpanded={toolHistoryExpanded}
          width={width}
        />
      </Box>

      {/* Scroll indicator — always 1 row to prevent layout shift / input blink */}
      <Text dimColor>
        {scrollState.offset > 0 || unreadBelow > 0
          ? <>
              {"  "}
              {scrollState.offset > 0 && <>{`\u2191 ${scrollState.offset} lines above`}</>}
              {scrollState.offset > 0 && unreadBelow > 0 && <>{" \u2219 "}</>}
              {unreadBelow > 0 && <>{`\u2193 ${unreadBelow} new below`}</>}
              {" (Ctrl+End live)"}
            </>
          : " "
        }
      </Text>

      {/* Overlays */}
      {showPermission && <PermissionDialog request={state.permissionRequest!} onRespond={handlePermission} width={width} />}
      {showQuestion && <QuestionDialog question={state.questionRequest!.question} options={state.questionRequest!.options} onRespond={handleQuestion} width={width} />}
      {pickerMode && <Picker title={pickerTitle(pickerMode)} items={pickerItems} onSelect={handlePickerSelect} onCancel={handlePickerCancel} width={width} />}

      {/* Separator */}
      <Text dimColor>{"\u2500".repeat(width)}</Text>

      {/* Queued messages */}
      {queuedMessages.length > 0 && (
        <Box flexDirection="column" paddingX={1} width={width}>
          {queuedMessageTexts.map((message, index) => (
            <Box key={index} width={width}><Text backgroundColor="blackBright" dimColor>{" \u276F "}{message}{" "}</Text></Box>
          ))}
        </Box>
      )}

      {/* Attachments */}
      {attachments.length > 0 && (
        <Attachments
          items={attachments}
          onRemove={handleRemoveAttachment}
          selectionMode={attachmentSelection}
          onExitSelection={() => setAttachmentSelection(false)}
          width={width}
        />
      )}

      {/* Input */}
      <InputArea
        ref={inputRef}
        phase={pickerMode ? "picker" : state.phase}
        onSubmit={handleSubmit}
        onAtMention={openFilePicker}
        onSlashCommand={() => { setPickerMode("commands"); setPickerItems(SLASH_COMMANDS) }}
        onPopQueue={() => {
          if (queuedMessages.length === 0) {
            return null
          }

          const restored = restoreQueuedSubmission(queuedMessages)
          const draft = restored.draft

          if (!draft) {
            return null
          }

          setQueuedMessages(restored.remaining)
          setAttachments(draft.attachments)
          setDraftPastes(draft.pastes)
          setAttachmentSelection(restored.attachmentSelection)

          return draft
        }}
        onEnterAttachmentSelection={() => setAttachmentSelection(true)}
        onPasteImage={(path) => { const id = nextAttachmentId.current++; setAttachments((a) => [...a, { id, name: `Image #${id}`, path, isImage: true }]) }}
        hasAttachments={attachments.length > 0}
        width={width}
        hasQueuedMessages={queuedMessages.length > 0}
        onRowsChange={setInputRows}
        onTextChange={handleInputTextChange}
        draftPastes={draftPastes}
        onHistoryPastesRestore={setDraftPastes}
      />

      {/* Bottom separator */}
      <Text dimColor>{"\u2500".repeat(width)}</Text>

      {/* Status */}
      <StatusBar
        model={state.session.model} tokensUsed={state.tokensUsed} tokensLimit={state.tokensLimit}
        mode={state.mode} yolo={state.yolo} streaming={state.phase === "streaming"} subagents={state.activeSubagents}
        cycle={state.cycle} drift={state.drift} notification={state.notification} width={width}
      />
    </Box>
  )
}

function toMessageAttachments(items: readonly AttachmentItem[]) {
  if (items.length === 0) {
    return undefined
  }

  return items.map((item) => ({
    name: item.name,
    isImage: item.isImage,
  }))
}

function pickerTitle(mode: PickerMode): string {
  switch (mode) {
    case "models": return "Select model"
    case "sessions": return "Resume session"
    case "files": return "Select file"
    case "commands": return "Commands"
    default: return "Select"
  }
}

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes}B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)}K`
  return `${(bytes / (1024 * 1024)).toFixed(1)}M`
}

function sameCollapsedPastes(
  left: readonly CollapsedPaste[],
  right: readonly CollapsedPaste[],
): boolean {
  if (left.length !== right.length) {
    return false
  }

  return left.every((paste, index) => {
    const other = right[index]

    return other !== undefined
      && paste.id === other.id
      && paste.rowCount === other.rowCount
      && paste.text === other.text
  })
}
