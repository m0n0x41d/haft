import type { MsgUpdateParams } from "../protocol/types.js"

type TimerHandle = ReturnType<typeof setTimeout> | number

interface StreamUpdateBufferOptions {
  delayMs?: number
  onFlush: (update: MsgUpdateParams, reason: string) => void
  schedule?: (callback: () => void, delayMs: number) => TimerHandle
  cancel?: (handle: TimerHandle) => void
}

export interface StreamUpdateBuffer {
  push: (update: MsgUpdateParams) => void
  flush: (reason: string, overrides?: Partial<MsgUpdateParams>) => boolean
  replace: (update: MsgUpdateParams, reason: string) => void
  clear: () => void
  hasPending: () => boolean
}

const DEFAULT_DELAY_MS = 32

export function createStreamUpdateBuffer(
  options: StreamUpdateBufferOptions,
): StreamUpdateBuffer {
  const delayMs = options.delayMs ?? DEFAULT_DELAY_MS
  const schedule = options.schedule ?? setTimeout
  const cancel = options.cancel ?? clearTimeout

  let pending: MsgUpdateParams | null = null
  let timer: TimerHandle | null = null

  function clearTimer() {
    if (!timer) {
      return
    }

    cancel(timer)
    timer = null
  }

  function dispatch(update: MsgUpdateParams, reason: string) {
    options.onFlush(update, reason)
  }

  function flushPending(
    reason: string,
    overrides: Partial<MsgUpdateParams> = {},
  ): boolean {
    if (!pending) {
      clearTimer()
      return false
    }

    const update = { ...pending, ...overrides }

    pending = null
    clearTimer()
    dispatch(update, reason)
    return true
  }

  return {
    push(update) {
      pending = update

      if (timer) {
        return
      }

      timer = schedule(() => {
        timer = null
        flushPending("scheduled_flush")
      }, delayMs)
    },

    flush(reason, overrides) {
      return flushPending(reason, overrides)
    },

    replace(update, reason) {
      pending = null
      clearTimer()
      dispatch(update, reason)
    },

    clear() {
      pending = null
      clearTimer()
    },

    hasPending() {
      return pending !== null
    },
  }
}
