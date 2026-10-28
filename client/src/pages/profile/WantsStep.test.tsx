import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { Profile } from '../../api/types'
import { WantsStep } from './WantsStep'

afterEach(cleanup)

const profile = {
  preferences: { role_types: [], locations: [], work_mode: 'any', min_pay: 0, pay_currency: 'USD', needs_visa: false, hidden_from: ['Fictional Payments Ltd'] },
  card_settings: { employer_display: 'name', employer_description: '', show_pay: true },
} as unknown as Profile

describe('WantsStep', () => {
  it('sends lists, the pay choice and the hidden-from companies', () => {
    const onNext = vi.fn()
    render(<WantsStep profile={profile} busy={false} onBack={() => {}} onNext={onNext} />)

    expect((screen.getByLabelText(/Hide my card from/) as HTMLInputElement).value).toBe('Fictional Payments Ltd')
    fireEvent.change(screen.getByLabelText('Locations (comma separated)'), { target: { value: 'Berlin, , Remote EU' } })
    fireEvent.change(screen.getByLabelText(/Hide my card from/), { target: { value: 'Fictional Payments Ltd, Rival Corp' } })
    fireEvent.click(screen.getByLabelText('Show my minimum pay on my card'))
    fireEvent.click(screen.getByRole('button', { name: 'Preview my card' }))

    const changes = onNext.mock.calls[0][0]
    expect(changes.preferences.locations).toEqual(['Berlin', 'Remote EU'])
    expect(changes.preferences.hidden_from).toEqual(['Fictional Payments Ltd', 'Rival Corp'])
    expect(changes.card_settings.show_pay).toBe(false)
  })
})
