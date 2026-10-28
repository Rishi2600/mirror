import { Link } from 'react-router'

// MatchBanner celebrates a new match and links to it.
export function MatchBanner({ matchId, onClose }: { matchId: string; onClose: () => void }) {
  return (
    <div role="status" data-testid="match-banner" className="flex items-center justify-between gap-3 rounded-lg bg-green-700 p-4 text-white">
      <div>
        <p className="text-lg font-semibold">It's a match!</p>
        <p className="text-sm text-green-100">You both liked each other.</p>
      </div>
      <div className="flex items-center gap-3">
        <Link to={`/matches/${matchId}`} className="rounded bg-white px-3 py-2 text-sm font-medium text-green-800">
          Open the match
        </Link>
        <button type="button" onClick={onClose} aria-label="Close" className="text-sm text-green-100 underline">
          Later
        </button>
      </div>
    </div>
  )
}
