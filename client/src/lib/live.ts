import type { QueryClient } from '@tanstack/react-query'
import { createContext, useContext } from 'react'
import type { LiveEvent } from '../api/types'
import { addMessage, markDeleted, messagesKey, setOtherReadAt } from './chatCache'

export type Listener = (e: LiveEvent) => void

export type Live = {
  // subscribe calls fn for every live event; it returns the unsubscribe.
  subscribe: (fn: Listener) => () => void
  sendTyping: (matchId: string) => void
}

export const LiveContext = createContext<Live>({ subscribe: () => () => {}, sendTyping: () => {} })

export function useLive() {
  return useContext(LiveContext)
}

// socketUrl turns the API base (empty means this page's origin) into the
// WebSocket address.
export function socketUrl(apiBase: string, pageOrigin: string) {
  return (apiBase || pageOrigin).replace(/^http/, 'ws') + '/v1/ws'
}

// applyEvent updates the cached data for one live event.
export function applyEvent(queryClient: QueryClient, e: LiveEvent) {
  switch (e.type) {
    case 'message':
      if (e.message) {
        addMessage(queryClient, e.message)
      }
      break
    case 'message_deleted':
      if (e.message_id) {
        markDeleted(queryClient, e.match_id, e.message_id)
      }
      break
    case 'read':
      if (e.read_at) {
        setOtherReadAt(queryClient, e.match_id, e.read_at)
      }
      return
    case 'unmatched':
      queryClient.invalidateQueries({ queryKey: ['match', e.match_id] })
      queryClient.invalidateQueries({ queryKey: messagesKey(e.match_id) })
      break
    case 'typing':
      return
  }
  queryClient.invalidateQueries({ queryKey: ['matches'] })
  queryClient.invalidateQueries({ queryKey: ['unread'] })
}
