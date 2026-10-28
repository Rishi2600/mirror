import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router'
import type { Match } from '../api/types'
import { ErrorMessage } from '../components/form'
import { api } from '../lib/api'

// MatchesPage lists the user's matches, newest first.
export function MatchesPage() {
  const matches = useQuery({ queryKey: ['matches'], queryFn: api.listMatches })

  return (
    <main className="mx-auto flex min-h-screen max-w-xl flex-col gap-6 p-6">
      <div className="flex items-center justify-between">
        <h1 className="text-3xl font-semibold">Your matches</h1>
        <Link to="/" className="text-sm text-blue-700 underline">
          Home
        </Link>
      </div>

      {matches.isPending && <p className="text-gray-600">Loading your matches...</p>}
      {matches.isError && <ErrorMessage message="Could not load your matches." />}
      {matches.isSuccess && matches.data.length === 0 && (
        <p className="text-gray-600">No matches yet. When someone you liked likes you back, they show up here.</p>
      )}
      {matches.isSuccess && matches.data.length > 0 && (
        <ul className="flex flex-col gap-3">
          {matches.data.map((m) => (
            <li key={m.id}>
              <Link to={`/matches/${m.id}`} className="flex items-center gap-4 rounded-lg border border-gray-200 p-3 hover:bg-gray-50">
                <MatchSummary match={m} />
              </Link>
            </li>
          ))}
        </ul>
      )}
    </main>
  )
}

// MatchSummary is one row: the job for a candidate, the person for an employer.
function MatchSummary({ match }: { match: Match }) {
  const image = match.job?.image_url || match.candidate?.photo_url || ''
  const title = match.job ? match.job.title : (match.candidate?.name ?? '')
  const detail = match.job ? match.job.company_name : `For ${match.hiring_card_title}`
  return (
    <>
      {image ? <img src={image} alt="" className="h-12 w-12 rounded-full object-cover" /> : <div className="h-12 w-12 rounded-full bg-gray-200" />}
      <div>
        <p className="font-medium">{title}</p>
        <p className="text-sm text-gray-600">{detail}</p>
      </div>
    </>
  )
}
