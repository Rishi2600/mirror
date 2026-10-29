import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router'
import { api } from '../lib/api'

// MatchesLink opens the inbox, with a badge for new matches and unread
// messages. Live events refresh the count.
export function MatchesLink() {
  const unread = useQuery({ queryKey: ['unread'], queryFn: api.getUnread })
  const count = (unread.data?.new_matches ?? 0) + (unread.data?.messages ?? 0)
  return (
    <Link to="/matches" className="flex items-center justify-center gap-2 rounded border border-gray-300 px-3 py-2 font-medium">
      Your matches
      {count > 0 && (
        <span data-testid="unread-badge" className="rounded-full bg-blue-600 px-2 py-0.5 text-xs font-semibold text-white">
          {count}
        </span>
      )}
    </Link>
  )
}
