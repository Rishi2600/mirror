import { QueryClient } from '@tanstack/react-query'
import { describe, expect, it } from 'vitest'
import type { ChatMessage } from '../api/types'
import { allMessages, messagesKey, type ChatPages } from './chatCache'
import { applyEvent, socketUrl } from './live'

const msg = (id: string): ChatMessage => ({
  id, match_id: 'm1', sender_id: 'u', client_id: id, body: `text ${id}`, created_at: '', deleted: false,
})

function clientWithChat() {
  const qc = new QueryClient()
  const pages: ChatPages = {
    pages: [{ messages: [msg('c')], has_more: true, other_read_at: null }, { messages: [msg('a'), msg('b')], has_more: false, other_read_at: null }],
    pageParams: ['', 'c'],
  }
  qc.setQueryData(messagesKey('m1'), pages)
  return qc
}

describe('applyEvent', () => {
  it('adds a new message to the end of the open chat, once', () => {
    const qc = clientWithChat()
    applyEvent(qc, { type: 'message', match_id: 'm1', message: msg('d') })
    applyEvent(qc, { type: 'message', match_id: 'm1', message: msg('d') })
    expect(allMessages(qc.getQueryData(messagesKey('m1'))).map((m) => m.id)).toEqual(['a', 'b', 'c', 'd'])
  })

  it('hides a deleted message and records the read time', () => {
    const qc = clientWithChat()
    applyEvent(qc, { type: 'message_deleted', match_id: 'm1', message_id: 'b' })
    applyEvent(qc, { type: 'read', match_id: 'm1', read_at: '2026-10-29T10:00:00Z' })
    const data = qc.getQueryData<ChatPages>(messagesKey('m1'))
    expect(allMessages(data).find((m) => m.id === 'b')).toMatchObject({ deleted: true, body: '' })
    expect(data?.pages[0].other_read_at).toBe('2026-10-29T10:00:00Z')
  })

  it('leaves chats that are not open alone', () => {
    const qc = new QueryClient()
    applyEvent(qc, { type: 'message', match_id: 'other', message: { ...msg('x'), match_id: 'other' } })
    expect(qc.getQueryData(messagesKey('other'))).toBeUndefined()
  })
})

describe('socketUrl', () => {
  it('uses the page origin when no API URL is set', () => {
    expect(socketUrl('', 'https://mirror.example')).toBe('wss://mirror.example/v1/ws')
    expect(socketUrl('http://localhost:8080', 'http://localhost:5173')).toBe('ws://localhost:8080/v1/ws')
  })
})
