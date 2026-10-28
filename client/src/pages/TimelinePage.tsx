import { useQuery } from '@tanstack/react-query'
import { Link, Navigate } from 'react-router'
import { ApiError } from '../api/client'
import { ErrorMessage } from '../components/form'
import { JobCard } from '../components/JobCard'
import { MatchReasons } from '../components/MatchReasons'
import { api } from '../lib/api'
import { useMe } from '../lib/useMe'

// TimelinePage: the hiring cards that fit the candidate, best first.
// Swiping comes next; for now the cards are listed.
export function TimelinePage() {
  const me = useMe()
  const timeline = useQuery({ queryKey: ['timeline'], queryFn: api.getTimeline, retry: false })

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

      {timeline.isPending && <p className="text-gray-600">Finding jobs that fit you...</p>}
      {timeline.isError && (
        <div className="flex flex-col gap-2">
          <ErrorMessage message={timeline.error instanceof ApiError ? timeline.error.message : 'Could not load your timeline.'} />
          <Link to="/profile" className="text-sm text-blue-700 underline">
            Go to your card
          </Link>
        </div>
      )}
      {timeline.isSuccess && timeline.data.length === 0 && (
        <p className="text-gray-600">No jobs fit your card yet. New jobs show up here as employers post them.</p>
      )}
      {timeline.isSuccess && (
        <ol className="flex flex-col gap-6">
          {timeline.data.map((job) => (
            <li key={job.card_id} data-testid="timeline-job" className="flex flex-col gap-2">
              <JobCard view={job.view} />
              <MatchReasons score={job.score} reasons={job.reasons} />
            </li>
          ))}
        </ol>
      )}
    </main>
  )
}
