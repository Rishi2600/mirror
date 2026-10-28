import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, cleanup, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { Profile } from '../api/types'
import { profileQueryKey } from '../lib/useProfile'
import { ProfilePage } from './ProfilePage'

const pending = { parse_status: 'pending', onboarded: false } as Profile

vi.mock('../lib/api', () => ({
  api: {
    getMe: vi.fn().mockResolvedValue({ id: '1', email: 'c@example.com', role: 'candidate' }),
    getProfile: vi.fn().mockResolvedValue({ parse_status: 'pending', onboarded: false }),
  },
}))

afterEach(cleanup)

describe('ProfilePage', () => {
  // A finished CV parse must not move the candidate to the next step: they
  // could be about to click Next, which would then skip a step.
  it('stays on step 1 when the CV finishes reading', async () => {
    const queryClient = new QueryClient()
    render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter>
          <ProfilePage />
        </MemoryRouter>
      </QueryClientProvider>,
    )
    expect((await screen.findByTestId('step-title')).textContent).toBe('Step 1 of 6: Your CV')

    act(() => queryClient.setQueryData(profileQueryKey, { ...pending, parse_status: 'done' }))

    // Next turns on once the page has the read CV; the step must not change.
    await waitFor(() => expect((screen.getByRole('button', { name: 'Next' }) as HTMLButtonElement).disabled).toBe(false))
    expect(screen.getByTestId('step-title').textContent).toBe('Step 1 of 6: Your CV')
  })
})
