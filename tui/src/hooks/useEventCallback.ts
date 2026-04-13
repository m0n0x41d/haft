// Stable callback wrapper — returns a function with fixed identity
// that always calls the latest version of the wrapped function.
// Prevents listener re-registration when callback closures change.

import { useRef, useCallback, useLayoutEffect } from "react"

export function useEventCallback<T extends (...args: any[]) => any>(fn: T): T {
  const ref = useRef(fn)
  useLayoutEffect(() => { ref.current = fn })
  // eslint-disable-next-line react-hooks/exhaustive-deps
  return useCallback((...args: any[]) => ref.current(...args), []) as T
}
