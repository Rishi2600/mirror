import { useCallback, useEffect, useRef } from 'react'

// useVisibleTime measures how long the page has shown the current key,
// leaving out time on another tab. It returns a function that reads the
// time in milliseconds, and starts again when the key changes.
export function useVisibleTime(key: string | undefined) {
  const spent = useRef(0)
  const since = useRef<number | null>(null)

  useEffect(() => {
    spent.current = 0
    since.current = document.visibilityState === 'visible' ? performance.now() : null

    function onChange() {
      if (document.visibilityState === 'visible') {
        since.current = performance.now()
      } else if (since.current !== null) {
        spent.current += performance.now() - since.current
        since.current = null
      }
    }
    document.addEventListener('visibilitychange', onChange)
    return () => document.removeEventListener('visibilitychange', onChange)
  }, [key])

  return useCallback(() => {
    const running = since.current === null ? 0 : performance.now() - since.current
    return Math.round(spent.current + running)
  }, [])
}
