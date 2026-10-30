import type { Likes } from '../api/types'
import { formatTime } from '../lib/format'

// LikesNote tells the candidate where they stand: likes left, a pause, the
// budget running out. A rule that affects them is never silent.
export function LikesNote({ likes }: { likes: Likes }) {
  return (
    <div className="flex flex-col gap-2">
      <Status likes={likes} />
      {likes.low_score && (
        <p className="rounded border border-amber-300 bg-amber-50 p-3 text-sm text-amber-900">
          Your likes count for more when you choose: like the jobs that fit you best, and reply to your matches.
        </p>
      )}
    </div>
  )
}

function Status({ likes }: { likes: Likes }) {
  if (likes.paused_until) {
    return (
      <p role="status" className="rounded border border-amber-300 bg-amber-50 p-3 text-sm text-amber-900">
        Likes are paused until {formatTime(likes.paused_until)}. Take a moment with each card: employers notice careful
        likes. You can still skip.
      </p>
    )
  }
  if (likes.left === 0) {
    return (
      <p role="status" className="rounded border border-amber-300 bg-amber-50 p-3 text-sm text-amber-900">
        You have used today's {likes.daily_budget} likes. More at midnight. You can still skip.
      </p>
    )
  }
  if (likes.left <= 3) {
    return (
      <p className="text-sm text-amber-800">
        {likes.left} {likes.left === 1 ? 'like' : 'likes'} left today. They reset at midnight.
      </p>
    )
  }
  return (
    <p className="text-sm text-gray-600">
      {likes.left} of {likes.daily_budget} likes left today
    </p>
  )
}
