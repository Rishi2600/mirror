import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router'
import type { Match } from '../api/types'
import { ErrorMessage } from '../components/form'
import { api } from '../lib/api'
import { useMe } from '../lib/useMe'

// MatchesPage is the inbox: the user's matches, the latest activity first.
export function MatchesPage() {
  const matches = useQuery({ queryKey: ['matches'], queryFn: api.listMatches })
  const me = useMe()

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
                <MatchSummary match={m} myId={me.data?.id ?? ''} />
              </Link>
            </li>
          ))}
        </ul>
      )}
    </main>
  )
}

// MatchSummary is one row: the job for a candidate, the person for an
// employer, with the last message and what is unread.
function MatchSummary({ match, myId }: { match: Match; myId: string }) {
  const image = match.job?.image_url || match.candidate?.photo_url || ''
  const title = match.job ? match.job.title : (match.candidate?.name ?? '')
  const detail = match.job ? match.job.company_name : `For ${match.hiring_card_title}`
  return (
    <>
      {image ? <img src={image} alt="" className="h-12 w-12 rounded-full object-cover" /> : <div className="h-12 w-12 shrink-0 rounded-full bg-gray-200" />}
      <div className="min-w-0 flex-1">
        <p className="flex items-center gap-2 font-medium">
          {title}
          {match.new && <span className="rounded-full bg-green-100 px-2 text-xs text-green-800">New</span>}
        </p>
        <p className="text-sm text-gray-600">{detail}</p>
        <p className={`truncate text-sm ${match.unread > 0 ? 'font-medium text-gray-900' : 'text-gray-500'}`}>
          {preview(match, myId)}
        </p>
      </div>
      {match.unread > 0 && (
        <span aria-label={`${match.unread} unread`} className="rounded-full bg-blue-600 px-2 py-0.5 text-xs font-semibold text-white">
          {match.unread}
        </span>
      )}
    </>
  )
}

function preview(match: Match, myId: string) {
  const last = match.last_message
  if (!last) {
    return 'Say hello'
  }
  const text = last.deleted ? 'Message deleted' : last.body || (last.file ? `Sent ${last.file.name}` : '')
  return last.sender_id === myId ? `You: ${text}` : text
}
