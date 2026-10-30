import { useQuery } from '@tanstack/react-query'
import { api } from '../lib/api'
import { selectivityKey } from '../lib/useLikes'

// GhostingNote tells an employer when their cards rank lower for not
// replying to matches. A rule that affects them is never silent.
export function GhostingNote() {
  const status = useQuery({ queryKey: selectivityKey, queryFn: api.getSelectivity })
  if (!status.data?.ghosting) {
    return null
  }
  return (
    <p role="status" className="rounded border border-amber-300 bg-amber-50 p-3 text-sm text-amber-900">
      You have replied to few of your recent matches, so your cards now show lower to candidates. Reply to your matches
      to recover.
    </p>
  )
}
