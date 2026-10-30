import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { createMemoryRouter, RouterProvider } from 'react-router'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ApiError } from '../api/client'
import { api } from '../lib/api'
import { SignInPage } from './SignInPage'

vi.mock('../lib/api', () => ({
  api: {
    getMe: vi.fn().mockResolvedValue(null),
    requestCode: vi.fn().mockResolvedValue(undefined),
    verifyCode: vi.fn().mockResolvedValue({ id: '1', email: 'ada@example.com', role: '' }),
  },
}))

afterEach(cleanup)

function renderSignIn() {
  const router = createMemoryRouter(
    [
      { path: '/sign-in', element: <SignInPage /> },
      { path: '/choose-role', element: <p>role picker</p> },
    ],
    { initialEntries: ['/sign-in'] },
  )
  render(
    <QueryClientProvider client={new QueryClient()}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  )
}

describe('SignInPage', () => {
  it('asks for the code after the email, then signs in', async () => {
    renderSignIn()

    fireEvent.change(screen.getByLabelText('Email'), { target: { value: 'ada@example.com' } })
    fireEvent.click(screen.getByRole('button', { name: 'Send me a code' }))
    expect(await screen.findByText(/We sent a 6-digit code/)).toBeTruthy()
    expect(api.requestCode).toHaveBeenCalledWith('ada@example.com')

    fireEvent.change(screen.getByLabelText('Code'), { target: { value: '123456' } })
    fireEvent.click(screen.getByRole('button', { name: 'Sign in' }))
    expect(await screen.findByText('role picker')).toBeTruthy()
    expect(api.verifyCode).toHaveBeenCalledWith('ada@example.com', '123456', Intl.DateTimeFormat().resolvedOptions().timeZone)
  })

  it("shows the server's message when the code is wrong", async () => {
    vi.mocked(api.verifyCode).mockRejectedValueOnce(new ApiError(401, 'that code is not right'))
    renderSignIn()

    fireEvent.change(screen.getByLabelText('Email'), { target: { value: 'ada@example.com' } })
    fireEvent.click(screen.getByRole('button', { name: 'Send me a code' }))
    fireEvent.change(await screen.findByLabelText('Code'), { target: { value: '000000' } })
    fireEvent.click(screen.getByRole('button', { name: 'Sign in' }))

    expect((await screen.findByRole('alert')).textContent).toBe('that code is not right')
  })
})
