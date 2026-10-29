import type { InfiniteData, QueryClient } from '@tanstack/react-query'
import type { ChatMessage, MessagesPage } from '../api/types'

// A chat is cached as pages: pages[0] holds the newest messages, later
// pages older ones. Each page is oldest first.
export type ChatPages = InfiniteData<MessagesPage>

export const messagesKey = (matchId: string) => ['messages', matchId]

// addMessage puts a new message at the end of an open chat, once.
export function addMessage(queryClient: QueryClient, msg: ChatMessage) {
  queryClient.setQueryData<ChatPages>(messagesKey(msg.match_id), (data) => {
    if (!data || data.pages.some((page) => page.messages.some((m) => m.id === msg.id))) {
      return data
    }
    const [newest, ...older] = data.pages
    return { ...data, pages: [{ ...newest, messages: [...newest.messages, msg] }, ...older] }
  })
}

export function markDeleted(queryClient: QueryClient, matchId: string, id: string) {
  queryClient.setQueryData<ChatPages>(messagesKey(matchId), (data) =>
    data && {
      ...data,
      pages: data.pages.map((page) => ({
        ...page,
        messages: page.messages.map((m) => (m.id === id ? { ...m, deleted: true, body: '', file: undefined } : m)),
      })),
    },
  )
}

export function setOtherReadAt(queryClient: QueryClient, matchId: string, at: string) {
  queryClient.setQueryData<ChatPages>(messagesKey(matchId), (data) =>
    data && { ...data, pages: data.pages.map((page, i) => (i === 0 ? { ...page, other_read_at: at } : page)) },
  )
}

// allMessages returns the cached chat oldest first.
export function allMessages(data: ChatPages | undefined): ChatMessage[] {
  return data ? [...data.pages].reverse().flatMap((page) => page.messages) : []
}
