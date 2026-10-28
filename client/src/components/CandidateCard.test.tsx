import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import type { Card } from '../api/types'
import { CandidateCard } from './CandidateCard'

afterEach(cleanup)

const card: Card = {
  name: 'Jordan Example',
  photo_url: '/v1/profile/photo?v=1',
  headline: 'Senior backend engineer',
  location: 'Example City',
  current_title: 'Senior Backend Engineer',
  current_employer: 'a payments company',
  years_experience: 7,
  seniority: 'senior',
  skills: ['Go', 'PostgreSQL'],
  summary: 'Builds APIs.',
  experience: [{ title: 'Senior Backend Engineer', company: 'a payments company', start: '2022', end: 'present', summary: '' }],
  education: [{ degree: 'BSc Computer Science', institution: 'Example State University', year: '2018' }],
  work_mode: 'remote',
  role_types: ['Backend'],
  min_pay: 90000,
  pay_currency: 'EUR',
}

describe('CandidateCard', () => {
  it('shows the name, photo and professional details', () => {
    render(<CandidateCard card={card} />)
    expect(screen.getByRole('heading', { name: 'Jordan Example' })).toBeTruthy()
    expect(screen.getByRole('img').getAttribute('src')).toBe('/v1/profile/photo?v=1')
    expect(screen.getByText(/7 years · senior · remote · Now at a payments company/)).toBeTruthy()
    expect(screen.getByText(/from 90,000 EUR a year/)).toBeTruthy()
  })

  it('leaves out pay when it is hidden (0)', () => {
    render(<CandidateCard card={{ ...card, min_pay: 0 }} />)
    expect(screen.queryByText(/a year/)).toBeNull()
    expect(screen.getByText(/Looking for:/).closest('p')?.textContent).toBe('Looking for: Backend')
  })
})
