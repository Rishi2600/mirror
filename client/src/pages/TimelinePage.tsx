import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { Link, Navigate } from 'react-router'
import { ApiError } from '../api/client'
import type { TimelineJob } from '../api/types'
import { ErrorMessage } from '../components/form'
import { JobCard } from '../components/JobCard'
import { MatchBanner } from '../components/MatchBanner'
import { MatchReasons } from '../components/MatchReasons'
import { SwipeStack } from '../components/SwipeStack'
import { api } from '../lib/api'
import { sendSwipe } from '../lib/sendSwipe'
import { useMe } from '../lib/useMe'

// TimelinePage: the hiring cards that fit the candidate, best first, as a
// swipeable stack. A like the employer already returned is a match.
export function TimelinePage() {
  const me = useMe()
  const timeline = useQuery({ queryKey: ['timeline'], queryFn: api.getTimeline, retry: false })
  const [matchId, setMatchId] = useState('')
  const [sendError, setSendError] = useState('')

  // Called once a swipe leaves the undo window.
  function send(job: TimelineJob, direction: 'left' | 'right') {
    sendSwipe({ hiring_card_id: job.card_id, direction })
      .then((res) => res?.matched && setMatchId(res.match_id))
      .catch(() => setSendError('A swipe could not be saved. Check your connection and reload the page.'))
  }

  if (me.data?.role !== 'candidate') {
    return <Navigate to="/" replace />
  }

  return (
    <main className="mx-auto flex min-h-screen max-w-xl flex-col gap-6 p-6">
      <div className="flex items-center justify-between">
        <h1 className="text-3xl font-semibold">Your timeline</h1>
        <Link to="/" className="text-sm text-blue-700 underline">
          Home
        </Link>
      </div>

      {matchId && <MatchBanner matchId={matchId} onClose={() => setMatchId('')} />}
      {timeline.isPending && <p className="text-gray-600">Finding jobs that fit you...</p>}
      {timeline.isError && (
        <div className="flex flex-col gap-2">
          <ErrorMessage message={timeline.error instanceof ApiError ? timeline.error.message : 'Could not load your timeline.'} />
          <Link to="/profile" className="text-sm text-blue-700 underline">
            Go to your card
          </Link>
        </div>
      )}
      {timeline.isSuccess && (
        <SwipeStack
          items={timeline.data}
          getKey={(job) => job.card_id}
          render={(job) => (
            <div data-testid="timeline-job" className="flex flex-col gap-3 p-1">
              <JobCard view={job.view} />
              <MatchReasons score={job.score} reasons={job.reasons} />
            </div>
          )}
          onSend={send}
          emptyMessage="You have seen every job that fits your card. New jobs show up here as employers post them."
        />
      )}
      <ErrorMessage message={sendError} />
    </main>
  )
}
