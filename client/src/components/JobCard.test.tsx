import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import type { JobView } from '../api/types'
import { JobCard } from './JobCard'

afterEach(cleanup)

const view: JobView = {
  title: 'Senior Backend Engineer',
  company_name: 'Fictional Payments Ltd',
  employer_first_name: 'Ana',
  image_url: '',
  location: 'Berlin',
  work_mode: 'remote',
  seniority: 'senior',
  skills: ['Go', 'PostgreSQL'],
  summary: 'Build payment APIs.',
  pay_min: 90000,
  pay_max: 120000,
  pay_currency: 'EUR',
  visa_sponsorship: true,
}

describe('JobCard', () => {
  it('shows the role, who posted it, pay and visa', () => {
    render(<JobCard view={view} />)
    expect(screen.getByRole('heading', { name: 'Senior Backend Engineer' })).toBeTruthy()
    expect(screen.getByText('Posted by Ana')).toBeTruthy()
    expect(screen.getByText('Berlin · remote · senior')).toBeTruthy()
    expect(screen.getByText('90,000 - 120,000 EUR a year')).toBeTruthy()
    expect(screen.getByText('Visa sponsorship available')).toBeTruthy()
  })

  it('leaves out pay that is not given', () => {
    render(<JobCard view={{ ...view, pay_min: 0, pay_max: 0, visa_sponsorship: false }} />)
    expect(screen.queryByText(/a year/)).toBeNull()
    expect(screen.queryByText('Visa sponsorship available')).toBeNull()
  })
})
