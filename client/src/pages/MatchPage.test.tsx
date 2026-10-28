import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { Card, CV, JobView } from '../api/types'
import { api } from '../lib/api'
import { MatchPage } from './MatchPage'

vi.mock('../lib/api', () => ({
  api: { getMatch: vi.fn(), unmatch: vi.fn() },
}))

afterEach(cleanup)

const job: JobView = {
  title: 'Backend engineer (Go)', company_name: 'Example Payments', employer_first_name: 'Sam', image_url: '',
  location: '', work_mode: 'remote', seniority: 'senior', skills: ['Go'], summary: 'Build APIs.',
  pay_min: 0, pay_max: 0, pay_currency: 'USD', visa_sponsorship: false,
}

const card: Card = {
  name: 'Jordan', photo_url: '', headline: 'Go developer', location: '', current_title: 'Engineer',
  current_employer: '', years_experience: 5, seniority: 'senior', skills: ['Go'], summary: '',
  experience: [], education: [], work_mode: 'remote', role_types: [], min_pay: 0, pay_currency: 'USD',
}

const cv = { full_name: 'Jordan Example' } as CV

function renderPage() {
  render(
    <QueryClientProvider client={new QueryClient()}>
      <MemoryRouter initialEntries={['/matches/m1']}>
        <Routes>
          <Route path="/matches/:id" element={<MatchPage />} />
          <Route path="/matches" element={<p>All matches page</p>} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('MatchPage', () => {
  it("shows the employer the candidate's contact details and CV link", async () => {
    vi.mocked(api.getMatch).mockResolvedValue({
      id: 'm1', matched_at: '', hiring_card_title: 'Backend engineer', candidate: card,
      contact: { full_name: 'Jordan Example', email: 'jordan@example.com', phone: '+1 555 0100', cv, cv_url: '/v1/matches/m1/cv?exp=1&sig=x' },
    })
    renderPage()

    expect(await screen.findByRole('heading', { name: 'Jordan Example' })).toBeTruthy()
    expect(screen.getByRole('link', { name: 'jordan@example.com' })).toBeTruthy()
    expect(screen.getByRole('link', { name: 'Download CV' }).getAttribute('href')).toBe('/v1/matches/m1/cv?exp=1&sig=x')
    expect(screen.queryByRole('button', { name: 'Unmatch' })).toBeNull()
  })

  it('lets the candidate unmatch after confirming', async () => {
    vi.mocked(api.getMatch).mockResolvedValue({ id: 'm1', matched_at: '', job })
    vi.mocked(api.unmatch).mockResolvedValue(undefined)
    renderPage()

    expect(await screen.findByRole('heading', { name: 'Backend engineer (Go)' })).toBeTruthy()
    expect(screen.queryByTestId('contact')).toBeNull()

    fireEvent.click(screen.getByRole('button', { name: 'Unmatch' }))
    fireEvent.click(screen.getByRole('button', { name: 'Keep the match' }))
    expect(api.unmatch).not.toHaveBeenCalled()

    fireEvent.click(screen.getByRole('button', { name: 'Unmatch' }))
    fireEvent.click(screen.getByRole('button', { name: 'Yes, unmatch' }))
    await waitFor(() => expect(api.unmatch).toHaveBeenCalledWith('m1'))
    expect(await screen.findByText('All matches page')).toBeTruthy()
  })
})
