import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ApiError } from '../api/client'
import type { JobView } from '../api/types'
import { api } from '../lib/api'
import { TimelinePage } from './TimelinePage'

const view: JobView = {
  title: 'Backend engineer (Go)', company_name: 'Example Payments', employer_first_name: 'Sam', image_url: '',
  location: '', work_mode: 'remote', seniority: 'senior', skills: ['Go'], summary: 'Build APIs.',
  pay_min: 0, pay_max: 0, pay_currency: 'USD', visa_sponsorship: false,
}

vi.mock('../lib/api', () => ({
  api: {
    getMe: vi.fn().mockResolvedValue({ id: '1', email: 'c@example.com', role: 'candidate', first_name: '' }),
    getTimeline: vi.fn(),
    getSelectivity: vi.fn().mockResolvedValue({ ghosting: false }),
    swipe: vi.fn(),
  },
}))

afterEach(cleanup)

function renderPage() {
  render(
    <QueryClientProvider client={new QueryClient()}>
      <MemoryRouter>
        <TimelinePage />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('TimelinePage', () => {
  it('lists fitting jobs with their fit and reasons', async () => {
    vi.mocked(api.getTimeline).mockResolvedValue([
      { card_id: 'a', view, score: 0.92, reasons: ['Skills match: Go', 'Same seniority'] },
    ])
    renderPage()

    expect(await screen.findByText('92% fit')).toBeTruthy()
    expect(screen.getByRole('heading', { name: 'Backend engineer (Go)' })).toBeTruthy()
    expect(screen.getByRole('list', { name: 'Why this card' }).textContent).toBe('Skills match: GoSame seniority')
  })

  it("shows the server's message before the card is finished", async () => {
    vi.mocked(api.getTimeline).mockRejectedValue(new ApiError(409, 'finish your card first; then fitting jobs show up here'))
    renderPage()

    expect((await screen.findByRole('alert')).textContent).toContain('finish your card first')
    expect(screen.getByRole('link', { name: 'Go to your card' })).toBeTruthy()
  })

  it('shows the likes left and stops likes when none remain', async () => {
    vi.mocked(api.getTimeline).mockResolvedValue([{ card_id: 'a', view, score: 0.92, reasons: [] }])
    vi.mocked(api.getSelectivity).mockResolvedValueOnce({
      ghosting: false,
      likes: { daily_budget: 15, left: 0, resets_at: '2026-10-31T18:30:00Z', paused_until: null, low_score: false },
    })
    renderPage()

    expect((await screen.findByRole('status')).textContent).toContain("You have used today's 15 likes.")
    await waitFor(() => expect((screen.getByRole('button', { name: 'Like' }) as HTMLButtonElement).disabled).toBe(true))
  })

  it('sends the time on the card, and puts a refused like back', async () => {
    const second = { ...view, title: 'Data engineer' }
    vi.mocked(api.getTimeline).mockResolvedValue([
      { card_id: 'a', view, score: 0.92, reasons: [] },
      { card_id: 'b', view: second, score: 0.9, reasons: [] },
    ])
    vi.mocked(api.swipe).mockRejectedValue(new ApiError(429, 'likes are paused for now; you can still skip'))
    renderPage()

    await screen.findByText('92% fit')
    fireEvent.click(screen.getByRole('button', { name: 'Like' }))
    await waitFor(() => expect(screen.getByTestId('swipe-card').textContent).toContain('Data engineer'))
    fireEvent.click(screen.getByRole('button', { name: 'Skip' }))

    await waitFor(() => expect(api.swipe).toHaveBeenCalled())
    expect(vi.mocked(api.swipe).mock.calls[0][0]).toMatchObject({ hiring_card_id: 'a', direction: 'right', dwell_ms: expect.any(Number) })
    await waitFor(() => expect(screen.getByTestId('swipe-card').textContent).toContain('Backend engineer'))
  })
})
