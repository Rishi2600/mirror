import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { SwipeStack } from './SwipeStack'

afterEach(cleanup)

const cards = ['Backend engineer', 'Product designer', 'Data analyst']

function renderStack(onSend = vi.fn()) {
  render(
    <SwipeStack
      items={cards}
      getKey={(c) => c}
      render={(c) => <h2>{c}</h2>}
      onSend={onSend}
      emptyMessage="All caught up"
    />,
  )
  return onSend
}

// The top card is the one inside the draggable card.
const topCard = () => screen.getByTestId('swipe-card').textContent

describe('SwipeStack', () => {
  it('like moves to the next card; undo brings the card back', async () => {
    const onSend = renderStack()
    expect(topCard()).toContain('Backend engineer')

    fireEvent.click(screen.getByRole('button', { name: 'Like' }))
    await waitFor(() => expect(topCard()).toContain('Product designer'))
    expect(screen.getByText(/Liked. You can undo/)).toBeTruthy()

    fireEvent.click(screen.getByRole('button', { name: 'Undo' }))
    await waitFor(() => expect(topCard()).toContain('Backend engineer'))
    expect(onSend).not.toHaveBeenCalled()
  })

  it('a second swipe sends the first; the arrow keys swipe too', async () => {
    const onSend = renderStack()

    fireEvent.click(screen.getByRole('button', { name: 'Skip' }))
    await waitFor(() => expect(topCard()).toContain('Product designer'))
    fireEvent.keyDown(window, { key: 'ArrowRight' })
    await waitFor(() => expect(topCard()).toContain('Data analyst'))

    expect(onSend).toHaveBeenCalledTimes(1)
    expect(onSend).toHaveBeenCalledWith('Backend engineer', 'left')
  })

  it('shows the empty message after the last card', async () => {
    renderStack()
    for (const next of ['Product designer', 'Data analyst']) {
      fireEvent.click(screen.getByRole('button', { name: 'Skip' }))
      await waitFor(() => expect(topCard()).toContain(next))
    }
    fireEvent.click(screen.getByRole('button', { name: 'Skip' }))
    expect(await screen.findByText('All caught up')).toBeTruthy()
    expect((screen.getByRole('button', { name: 'Like' }) as HTMLButtonElement).disabled).toBe(true)
  })
})
