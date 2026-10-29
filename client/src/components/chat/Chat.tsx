import { useInfiniteQuery, useQueryClient } from '@tanstack/react-query'
import { Fragment, useEffect, useLayoutEffect, useRef, useState } from 'react'
import { ApiError } from '../../api/client'
import { api } from '../../lib/api'
import { addMessage, allMessages, markDeleted, messagesKey } from '../../lib/chatCache'
import { useLive } from '../../lib/live'
import { ErrorMessage } from '../form'
import { Composer } from './Composer'
import { MessageBubble } from './MessageBubble'

const typingShowsMs = 3000
const nearEdgePx = 80

// A message on its way to the server, or one that could not be sent.
type Pending = { clientId: string; body: string; file?: File; error: string }

type Props = { matchId: string; myId: string; otherName: string }

// Chat is the conversation of one match. New messages arrive live; older
// ones load when you scroll to the top.
export function Chat({ matchId, myId, otherName }: Props) {
  const queryClient = useQueryClient()
  const live = useLive()
  const chat = useInfiniteQuery({
    queryKey: messagesKey(matchId),
    queryFn: ({ pageParam }) => api.listMessages(matchId, pageParam),
    initialPageParam: '',
    getNextPageParam: (last) => (last.has_more ? last.messages[0]?.id : undefined),
  })
  const messages = allMessages(chat.data)
  const otherReadAt = chat.data?.pages[0]?.other_read_at
  const lastId = messages.at(-1)?.id
  const [pending, setPending] = useState<Pending[]>([])
  const [typing, setTyping] = useState(false)

  // Scrolling: stay at the bottom as messages come in (if already there),
  // and keep the view still when older messages are added above.
  const list = useRef<HTMLDivElement>(null)
  const atBottom = useRef(true)
  const heightBeforeOlder = useRef(0)
  function keepAtBottom() {
    const el = list.current
    if (el && atBottom.current) {
      el.scrollTop = el.scrollHeight
    }
  }
  useLayoutEffect(keepAtBottom, [lastId, pending.length, typing])
  useLayoutEffect(() => {
    const el = list.current
    if (el && heightBeforeOlder.current) {
      el.scrollTop += el.scrollHeight - heightBeforeOlder.current
      heightBeforeOlder.current = 0
    }
  }, [chat.data?.pages.length])

  function loadOlder() {
    if (list.current && chat.hasNextPage && !chat.isFetchingNextPage) {
      heightBeforeOlder.current = list.current.scrollHeight
      chat.fetchNextPage()
    }
  }

  function onScroll() {
    const el = list.current
    if (!el) {
      return
    }
    atBottom.current = el.scrollHeight - el.scrollTop - el.clientHeight < nearEdgePx
    if (el.scrollTop < nearEdgePx) {
      loadOlder()
    }
  }

  // Reading: mark the chat read when it opens, when a message arrives while
  // the page is visible, and when the page becomes visible again.
  useEffect(() => {
    if (!chat.isSuccess) {
      return
    }
    function read() {
      if (document.visibilityState !== 'visible') {
        return
      }
      api.markRead(matchId).then(() => {
        queryClient.invalidateQueries({ queryKey: ['unread'] })
        queryClient.invalidateQueries({ queryKey: ['matches'] })
      }, () => {})
    }
    read()
    document.addEventListener('visibilitychange', read)
    return () => document.removeEventListener('visibilitychange', read)
  }, [matchId, lastId, chat.isSuccess, queryClient])

  // Typing: show it for a few seconds after each signal, or until their
  // message arrives.
  useEffect(() => {
    let timer: ReturnType<typeof setTimeout> | undefined
    const stop = live.subscribe((e) => {
      if (e.match_id !== matchId) {
        return
      }
      if (e.type === 'typing') {
        setTyping(true)
        clearTimeout(timer)
        timer = setTimeout(() => setTyping(false), typingShowsMs)
      }
      if (e.type === 'message' && e.message?.sender_id !== myId) {
        setTyping(false)
      }
    })
    return () => {
      stop()
      clearTimeout(timer)
    }
  }, [live, matchId, myId])

  async function send(p: Pending) {
    setPending((all) => [...all.filter((x) => x.clientId !== p.clientId), { ...p, error: '' }])
    atBottom.current = true
    try {
      const msg = await api.sendMessage(matchId, p.body, p.clientId, p.file, p.file?.name)
      addMessage(queryClient, msg)
      setPending((all) => all.filter((x) => x.clientId !== p.clientId))
      queryClient.invalidateQueries({ queryKey: ['matches'] })
    } catch (err) {
      // A server answer (such as a refused file) will not change on retry.
      const error = err instanceof ApiError ? err.message : 'Not sent. Check your connection.'
      setPending((all) => all.map((x) => (x.clientId === p.clientId ? { ...x, error } : x)))
    }
  }

  async function remove(id: string) {
    await api.deleteMessage(id).catch(() => {})
    markDeleted(queryClient, matchId, id)
    queryClient.invalidateQueries({ queryKey: ['matches'] })
  }

  const lastMine = messages.findLast((m) => m.sender_id === myId && !m.deleted)
  const seen = lastMine && otherReadAt && new Date(otherReadAt) >= new Date(lastMine.created_at)
  const seenId = seen ? lastMine.id : ''

  return (
    <section className="flex flex-col gap-2 rounded-lg border border-gray-200 p-3">
      <div ref={list} onScroll={onScroll} data-testid="chat-messages" className="flex h-[60vh] flex-col gap-3 overflow-y-auto p-1">
        {chat.hasNextPage && (
          <button type="button" onClick={loadOlder} className="self-center text-sm text-blue-700 underline">
            {chat.isFetchingNextPage ? 'Loading...' : 'Load older messages'}
          </button>
        )}
        {chat.isPending && <p className="text-gray-600">Loading the chat...</p>}
        {chat.isError && <ErrorMessage message="Could not load the chat." />}
        {chat.isSuccess && messages.length === 0 && pending.length === 0 && (
          <p className="m-auto text-center text-gray-600">Say hello to {otherName}. You both said yes.</p>
        )}
        {messages.map((m, i) => (
          <Fragment key={m.id}>
            {(i === 0 || dayOf(messages[i - 1].created_at) !== dayOf(m.created_at)) && (
              <p className="self-center text-xs text-gray-500">{dayOf(m.created_at)}</p>
            )}
            <MessageBubble message={m} mine={m.sender_id === myId} seen={m.id === seenId} onDelete={() => remove(m.id)} onImageLoad={keepAtBottom} />
          </Fragment>
        ))}
        {pending.map((p) => (
          <div key={p.clientId} className="flex flex-col items-end gap-1">
            <p className="max-w-[80%] rounded-2xl bg-blue-300 px-3 py-2 break-words whitespace-pre-wrap text-white">
              {p.body || p.file?.name}
            </p>
            {p.error ? (
              <p className="flex gap-2 text-xs text-red-700">
                <span>{p.error}</span>
                <button type="button" onClick={() => send(p)} className="underline">
                  Retry
                </button>
                <button
                  type="button"
                  onClick={() => setPending((all) => all.filter((x) => x.clientId !== p.clientId))}
                  className="underline"
                >
                  Remove
                </button>
              </p>
            ) : (
              <p className="text-xs text-gray-500">Sending...</p>
            )}
          </div>
        ))}
        {typing && <p className="text-sm text-gray-500 italic">{otherName} is typing...</p>}
      </div>
      <Composer onSend={(body, file) => send({ clientId: crypto.randomUUID(), body, file, error: '' })} onTyping={() => live.sendTyping(matchId)} />
    </section>
  )
}

function dayOf(iso: string) {
  return new Date(iso).toLocaleDateString([], { weekday: 'short', day: 'numeric', month: 'short' })
}
