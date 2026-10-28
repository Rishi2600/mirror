import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ApiError } from '../api/client'
import { api } from '../lib/api'
import { TimelinePage } from './TimelinePage'

const view = {
  title: 'Backend engineer (Go)', company_name: 'Example Payments', employer_first_name: 'Sam', image_url: '',
  location: '', work_mode: 'remote', seniority: 'senior', skills: ['Go'], summary: 'Build APIs.',
  pay_min: 0, pay_max: 0, pay_currency: 'USD', visa_sponsorship: false,
}

vi.mock('../lib/api', () => ({
  api: {
    getMe: vi.fn().mockResolvedValue({ id: '1', email: 'c@example.com', role: 'candidate', first_name: '' }),
    getTimeline: vi.fn(),
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
})
