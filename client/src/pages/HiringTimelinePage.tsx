import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { Link, Navigate, useParams } from 'react-router'
import { ApiError } from '../api/client'
import { CandidateCard } from '../components/CandidateCard'
import { ErrorMessage } from '../components/form'
import { MatchReasons } from '../components/MatchReasons'
import { SwipeStack } from '../components/SwipeStack'
import { api } from '../lib/api'
import { useHiringCard } from '../lib/useHiringCards'
import { useMe } from '../lib/useMe'

// HiringTimelinePage: the candidates that fit one hiring card, best first,
// as a swipeable stack (swipes are saved in chunk 9).
export function HiringTimelinePage() {
  const { id = '' } = useParams()
  const me = useMe()
  const card = useHiringCard(id)
  const timeline = useQuery({ queryKey: ['hiring-timeline', id], queryFn: () => api.getHiringCardTimeline(id), retry: false })
  const [sent, setSent] = useState({ liked: 0, skipped: 0 })

  if (me.data?.role !== 'employer') {
    return <Navigate to="/" replace />
  }

  return (
    <main className="mx-auto flex min-h-screen max-w-xl flex-col gap-6 p-6">
      <div className="flex items-center justify-between gap-3">
        <div>
          <p className="text-sm text-gray-600">Candidates for</p>
          <h1 className="text-2xl font-semibold">{card.data?.title || 'Hiring card'}</h1>
        </div>
        <Link to="/" className="text-sm text-blue-700 underline">
          Your cards
        </Link>
      </div>

      {timeline.isPending && <p className="text-gray-600">Finding candidates who fit...</p>}
      {timeline.isError && (
        <div className="flex flex-col gap-2">
          <ErrorMessage message={timeline.error instanceof ApiError ? timeline.error.message : 'Could not load the timeline.'} />
          <Link to={`/hiring/${id}`} className="text-sm text-blue-700 underline">
            Go to the card
          </Link>
        </div>
      )}
      {timeline.isSuccess && (
        <SwipeStack
          items={timeline.data}
          getKey={(person) => person.candidate_id}
          render={(person) => (
            <div data-testid="timeline-candidate" className="flex flex-col gap-3 p-1">
              <CandidateCard card={person.card} />
              <MatchReasons score={person.score} reasons={person.reasons} />
            </div>
          )}
          onSend={(_, direction) =>
            setSent((n) => (direction === 'right' ? { ...n, liked: n.liked + 1 } : { ...n, skipped: n.skipped + 1 }))
          }
          emptyMessage="You have seen every candidate who fits this card. New candidates show up here as they join."
        />
      )}
      {(sent.liked > 0 || sent.skipped > 0) && (
        <p data-testid="sent-count" className="text-center text-xs text-gray-500">
          Sent this visit: {sent.liked} liked, {sent.skipped} skipped.
        </p>
      )}
    </main>
  )
}
