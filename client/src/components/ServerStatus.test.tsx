import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import { ServerStatus } from './ServerStatus'

afterEach(cleanup)

describe('ServerStatus', () => {
  it('shows the server status and version', () => {
    render(
      <ServerStatus
        health={{ status: 'ok', version: 'dev', time: '2026-10-27T12:00:00Z' }}
        isLoading={false}
        isError={false}
      />,
    )
    expect(screen.getByTestId('server-status').textContent).toBe('ok')
    expect(screen.getByTestId('server-version').textContent).toBe('dev')
    expect(screen.queryByRole('alert')).toBeNull()
  })

  it('says so when the API is unreachable', () => {
    render(<ServerStatus isLoading={false} isError={true} />)
    expect(screen.getByRole('alert').textContent).toContain('The API is unreachable')
    expect(screen.queryByTestId('server-status')).toBeNull()
  })
})
