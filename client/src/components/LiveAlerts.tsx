import { useEffect, useState } from 'react'
import { Link, useLocation } from 'react-router'
import { useLive } from '../lib/live'
import { useMe } from '../lib/useMe'

const showMs = 6000

type Alert = { matchId: string; title: string; text: string }

// LiveAlerts shows a short notice for a new match or message, unless that
// chat is already on screen.
export function LiveAlerts() {
  const live = useLive()
  const me = useMe()
  const { pathname } = useLocation()
  const [alert, setAlert] = useState<Alert | null>(null)
  const myId = me.data?.id

  useEffect(
    () =>
      live.subscribe((e) => {
        if (pathname === `/matches/${e.match_id}`) {
          return
        }
        if (e.type === 'match') {
          setAlert({ matchId: e.match_id, title: "It's a match!", text: 'Someone you liked likes you back.' })
        }
        if (e.type === 'message' && e.message && e.message.sender_id !== myId) {
          const text = e.message.body || (e.message.file ? `Sent ${e.message.file.name}` : '')
          setAlert({ matchId: e.match_id, title: 'New message', text })
        }
      }),
    [live, pathname, myId],
  )

  useEffect(() => {
    if (!alert) {
      return
    }
    const timer = setTimeout(() => setAlert(null), showMs)
    return () => clearTimeout(timer)
  }, [alert])

  if (!alert) {
    return null
  }
  return (
    <div role="status" className="fixed top-4 right-4 left-4 z-50 mx-auto max-w-sm rounded-lg bg-gray-900 p-4 text-white shadow-lg">
      <Link to={`/matches/${alert.matchId}`} onClick={() => setAlert(null)} className="block">
        <p className="font-semibold">{alert.title}</p>
        <p className="truncate text-sm text-gray-300">{alert.text}</p>
      </Link>
    </div>
  )
}
