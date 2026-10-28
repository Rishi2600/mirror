import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { Link, Navigate, useParams } from 'react-router'
import { ApiError } from '../api/client'
import type { TimelineCandidate } from '../api/types'
import { CandidateCard } from '../components/CandidateCard'
import { ErrorMessage } from '../components/form'
import { MatchBanner } from '../components/MatchBanner'
import { MatchReasons } from '../components/MatchReasons'
import { SwipeStack } from '../components/SwipeStack'
import { api } from '../lib/api'
import { sendSwipe } from '../lib/sendSwipe'
import { useHiringCard } from '../lib/useHiringCards'
import { useMe } from '../lib/useMe'

// HiringTimelinePage: the candidates that fit one hiring card, as a
// swipeable stack. Candidates who already liked the card come first.
export function HiringTimelinePage() {
  const { id = '' } = useParams()
  const me = useMe()
  const card = useHiringCard(id)
  const timeline = useQuery({ queryKey: ['hiring-timeline', id], queryFn: () => api.getHiringCardTimeline(id), retry: false })
  const [matchId, setMatchId] = useState('')
  const [sendError, setSendError] = useState('')

  // Called once a swipe leaves the undo window.
  function send(person: TimelineCandidate, direction: 'left' | 'right') {
    sendSwipe({ hiring_card_id: id, candidate_id: person.candidate_id, direction })
      .then((res) => res?.matched && setMatchId(res.match_id))
      .catch(() => setSendError('A swipe could not be saved. Check your connection and reload the page.'))
  }

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

      {matchId && <MatchBanner matchId={matchId} onClose={() => setMatchId('')} />}
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
              {person.liked && (
                <p className="self-start rounded-full bg-green-100 px-3 py-1 text-sm font-medium text-green-800">
                  Liked your card: like back to match
                </p>
              )}
              <CandidateCard card={person.card} />
              <MatchReasons score={person.score} reasons={person.reasons} />
            </div>
          )}
          onSend={send}
          emptyMessage="You have seen every candidate who fits this card. New candidates show up here as they join."
        />
      )}
      <ErrorMessage message={sendError} />
    </main>
  )
}
