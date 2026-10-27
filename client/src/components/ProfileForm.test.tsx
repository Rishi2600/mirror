import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { Profile } from '../api/types'
import { api } from '../lib/api'
import { ProfileForm } from './ProfileForm'

vi.mock('../lib/api', () => ({
  api: { updateProfile: vi.fn() },
}))

afterEach(cleanup)

const profile: Profile = {
  parse_status: 'done',
  parse_error: '',
  updated_at: '2026-10-27T12:00:00Z',
  cv: {
    full_name: 'Jordan Example',
    email: 'jordan.example@example.com',
    phone: '',
    headline: 'Senior backend engineer',
    summary: '',
    current_title: 'Senior Backend Engineer',
    current_employer: 'Fictional Payments Ltd',
    location: 'Example City',
    years_experience: 7,
    seniority: 'senior',
    skills: ['Go', 'PostgreSQL'],
    experience: [],
    education: [],
  },
  preferences: { role_types: [], locations: [], work_mode: 'any', min_pay: 0, pay_currency: 'USD', needs_visa: false },
}

describe('ProfileForm', () => {
  it('saves edits, turning comma-separated text into lists', async () => {
    vi.mocked(api.updateProfile).mockResolvedValue(profile)
    render(
      <QueryClientProvider client={new QueryClient()}>
        <ProfileForm profile={profile} />
      </QueryClientProvider>,
    )

    expect((screen.getByLabelText('Skills (comma separated)') as HTMLInputElement).value).toBe('Go, PostgreSQL')
    fireEvent.change(screen.getByLabelText('Skills (comma separated)'), { target: { value: 'Go, Rust, , Kafka' } })
    fireEvent.change(screen.getByLabelText('Locations (comma separated)'), { target: { value: 'Berlin, Remote EU' } })
    fireEvent.change(screen.getByLabelText('Work mode'), { target: { value: 'remote' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save profile' }))

    expect(await screen.findByText('Saved')).toBeTruthy()
    const [cv, prefs] = vi.mocked(api.updateProfile).mock.calls[0]
    expect(cv.skills).toEqual(['Go', 'Rust', 'Kafka'])
    expect(prefs.locations).toEqual(['Berlin', 'Remote EU'])
    expect(prefs.work_mode).toBe('remote')
  })
})
