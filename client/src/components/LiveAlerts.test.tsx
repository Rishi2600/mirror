import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, cleanup, render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { LiveEvent } from '../api/types'
import { LiveContext, type Listener } from '../lib/live'
import { meQueryKey } from '../lib/useMe'
import { LiveAlerts } from './LiveAlerts'

vi.mock('../lib/api', () => ({ api: { getMe: vi.fn() } }))

afterEach(cleanup)

function renderAt(path: string) {
  let listener: Listener = () => {}
  const live = { subscribe: (fn: Listener) => ((listener = fn), () => {}), sendTyping: () => {} }
  const queryClient = new QueryClient()
  queryClient.setQueryData(meQueryKey, { id: 'me', email: 'me@example.com', role: 'candidate', first_name: '' })
  render(
    <QueryClientProvider client={queryClient}>
      <LiveContext.Provider value={live}>
        <MemoryRouter initialEntries={[path]}>
          <LiveAlerts />
        </MemoryRouter>
      </LiveContext.Provider>
    </QueryClientProvider>,
  )
  return (e: LiveEvent) => act(() => listener(e))
}

const message = (sender: string): LiveEvent => ({
  type: 'message', match_id: 'm1',
  message: { id: 'x', match_id: 'm1', sender_id: sender, client_id: 'x', body: 'Are you free Monday?', created_at: '', deleted: false },
})

describe('LiveAlerts', () => {
  it('shows a new match and a new message with a link to the chat', () => {
    const push = renderAt('/')
    push({ type: 'match', match_id: 'm1' })
    expect(screen.getByRole('status').textContent).toContain("It's a match!")
    push(message('other'))
    expect(screen.getByRole('status').textContent).toContain('Are you free Monday?')
    expect(screen.getByRole('link').getAttribute('href')).toBe('/matches/m1')
  })

  it('stays quiet for my own messages and for the chat on screen', () => {
    const push = renderAt('/')
    push(message('me'))
    expect(screen.queryByRole('status')).toBeNull()
    cleanup()

    const pushOnChat = renderAt('/matches/m1')
    pushOnChat(message('other'))
    expect(screen.queryByRole('status')).toBeNull()
  })
})
