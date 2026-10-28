import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { HiringCardList } from './HiringCardList'

vi.mock('../lib/api', () => ({
  api: {
    listHiringCards: vi.fn().mockResolvedValue([
      { id: 'a', title: 'Backend engineer', company_name: 'Fictional Co', status: 'live', image_url: '' },
      { id: 'b', title: 'Product designer', company_name: '', status: 'draft', image_url: '' },
      { id: 'c', title: '', company_name: '', status: 'closed', image_url: '' },
    ]),
  },
}))

afterEach(cleanup)

describe('HiringCardList', () => {
  it("lists the employer's cards with their status, linking to each", async () => {
    render(
      <QueryClientProvider client={new QueryClient()}>
        <MemoryRouter>
          <HiringCardList />
        </MemoryRouter>
      </QueryClientProvider>,
    )

    const items = await screen.findAllByTestId('hiring-card-item')
    expect(items.map((item) => item.textContent)).toEqual([
      'Backend engineerFictional CoLive',
      'Product designerDraft',
      'Untitled roleClosed',
    ])
    expect(items[0].getAttribute('href')).toBe('/hiring/a')
    expect(screen.getByRole('link', { name: 'Create a new card' }).getAttribute('href')).toBe('/hiring/new')
  })
})
