import { useCallback, useEffect, useRef, useState } from 'react'

export type Direction = 'left' | 'right'

export type Swipe<T> = { item: T; direction: Direction }

// How long a swipe waits before it is sent, so it can still be undone.
export const UNDO_MS = 5000

// useSwipeQueue holds the last swipe for UNDO_MS before calling onSend. Only
// that swipe can be undone; a new swipe sends the waiting one at once, and
// leaving the page sends it too. Once sent, a swipe is final (it may have
// made a match).
export function useSwipeQueue<T>(onSend: (swipe: Swipe<T>) => void, undoMs = UNDO_MS) {
  const [waiting, setWaiting] = useState<Swipe<T> | null>(null)
  const waitingRef = useRef<Swipe<T> | null>(null)
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)
  const onSendRef = useRef(onSend)
  useEffect(() => {
    onSendRef.current = onSend
  })

  const send = useCallback(() => {
    clearTimeout(timer.current)
    const swipe = waitingRef.current
    waitingRef.current = null
    setWaiting(null)
    if (swipe) {
      onSendRef.current(swipe)
    }
  }, [])

  const swipe = useCallback(
    (item: T, direction: Direction) => {
      send()
      waitingRef.current = { item, direction }
      setWaiting(waitingRef.current)
      timer.current = setTimeout(send, undoMs)
    },
    [send, undoMs],
  )

  // undo takes back the waiting swipe and returns it (null if there is none).
  const undo = useCallback(() => {
    clearTimeout(timer.current)
    const swipe = waitingRef.current
    waitingRef.current = null
    setWaiting(null)
    return swipe
  }, [])

  useEffect(() => send, [send])

  return { waiting, swipe, undo }
}
