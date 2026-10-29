import { useQueryClient } from '@tanstack/react-query'
import { useEffect, useRef, useState, type ReactNode } from 'react'
import type { LiveEvent } from '../api/types'
import { applyEvent, LiveContext, socketUrl, type Listener, type Live } from '../lib/live'

const maxRetryMs = 30_000

// LiveProvider keeps one WebSocket open while the user is signed in, and
// reconnects when it drops. After a reconnect it reloads what may have
// changed in between.
export function LiveProvider({ children }: { children: ReactNode }) {
  const queryClient = useQueryClient()
  const listeners = useRef(new Set<Listener>())
  const socket = useRef<WebSocket | null>(null)

  useEffect(() => {
    let stopped = false
    let failures = 0
    let retry: ReturnType<typeof setTimeout> | undefined

    function connect() {
      const ws = new WebSocket(socketUrl(import.meta.env.VITE_API_URL ?? '', window.location.origin))
      socket.current = ws
      ws.onopen = () => {
        if (failures > 0) {
          for (const key of ['matches', 'unread', 'match', 'messages']) {
            queryClient.invalidateQueries({ queryKey: [key] })
          }
        }
        failures = 0
      }
      ws.onmessage = (msg) => {
        const e = JSON.parse(msg.data) as LiveEvent
        applyEvent(queryClient, e)
        listeners.current.forEach((fn) => fn(e))
      }
      ws.onclose = () => {
        socket.current = null
        if (stopped) {
          return
        }
        failures++
        retry = setTimeout(connect, Math.min(1000 * 2 ** (failures - 1), maxRetryMs))
      }
    }

    connect()
    return () => {
      stopped = true
      clearTimeout(retry)
      socket.current?.close()
    }
  }, [queryClient])

  const [live] = useState<Live>(() => ({
    subscribe: (fn) => {
      listeners.current.add(fn)
      return () => {
        listeners.current.delete(fn)
      }
    },
    sendTyping: (matchId) => {
      if (socket.current?.readyState === WebSocket.OPEN) {
        socket.current.send(JSON.stringify({ type: 'typing', match_id: matchId }))
      }
    },
  }))

  return <LiveContext.Provider value={live}>{children}</LiveContext.Provider>
}
