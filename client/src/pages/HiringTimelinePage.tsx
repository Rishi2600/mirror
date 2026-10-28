import { useQuery } from '@tanstack/react-query'
import { Link, Navigate, useParams } from 'react-router'
import { ApiError } from '../api/client'
import { CandidateCard } from '../components/CandidateCard'
import { ErrorMessage } from '../components/form'
import { MatchReasons } from '../components/MatchReasons'
import { api } from '../lib/api'
import { useHiringCard } from '../lib/useHiringCards'
import { useMe } from '../lib/useMe'

// HiringTimelinePage: the candidates that fit one hiring card, best first.
export function HiringTimelinePage() {
  const { id = '' } = useParams()
  const me = useMe()
  const card = useHiringCard(id)
  const timeline = useQuery({ queryKey: ['hiring-timeline', id], queryFn: () => api.getHiringCardTimeline(id), retry: false })

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
      {timeline.isSuccess && timeline.data.length === 0 && (
        <p className="text-gray-600">No candidates fit this card yet. New candidates show up here as they join.</p>
      )}
      {timeline.isSuccess && (
        <ol className="flex flex-col gap-6">
          {timeline.data.map((person) => (
            <li key={person.candidate_id} data-testid="timeline-candidate" className="flex flex-col gap-2">
              <CandidateCard card={person.card} />
              <MatchReasons score={person.score} reasons={person.reasons} />
            </li>
          ))}
        </ol>
      )}
    </main>
  )
}
