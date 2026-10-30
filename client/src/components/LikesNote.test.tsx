import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import type { Likes } from '../api/types'
import { LikesNote } from './LikesNote'

afterEach(cleanup)

const likes: Likes = { daily_budget: 15, left: 12, resets_at: '2026-10-31T18:30:00Z', paused_until: null, low_score: false }

describe('LikesNote', () => {
  it('shows the likes left, and warns when few remain', () => {
    render(<LikesNote likes={likes} />)
    expect(screen.getByText('12 of 15 likes left today')).toBeTruthy()
    cleanup()
    render(<LikesNote likes={{ ...likes, left: 1 }} />)
    expect(screen.getByText('1 like left today. They reset at midnight.')).toBeTruthy()
  })

  it('explains a used budget and a pause, and says skips still work', () => {
    render(<LikesNote likes={{ ...likes, left: 0 }} />)
    expect(screen.getByRole('status').textContent).toContain("You have used today's 15 likes.")
    cleanup()
    render(<LikesNote likes={{ ...likes, paused_until: '2026-10-31T09:02:00Z' }} />)
    const note = screen.getByRole('status').textContent
    expect(note).toContain('Likes are paused until')
    expect(note).toContain('You can still skip.')
  })

  it('gives the low score nudge, never the score itself', () => {
    render(<LikesNote likes={{ ...likes, low_score: true }} />)
    expect(screen.getByText(/Your likes count for more when you choose/)).toBeTruthy()
  })
})
