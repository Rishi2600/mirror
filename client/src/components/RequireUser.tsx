import { Navigate, Outlet } from 'react-router'
import { useMe } from '../lib/useMe'

type Props = {
  // true: the page needs a role (users without one go to /choose-role).
  // false: the page is for users who have not picked a role yet.
  needsRole: boolean
}

// RequireUser shows its child routes only to a signed-in user.
export function RequireUser({ needsRole }: Props) {
  const me = useMe()

  if (me.isPending) {
    return <p className="p-6 text-gray-600">Loading...</p>
  }
  if (me.isError) {
    return (
      <p role="alert" className="m-6 rounded border border-red-300 bg-red-50 p-3 text-red-800">
        The API is unreachable. Start the server and reload this page.
      </p>
    )
  }
  if (!me.data) {
    return <Navigate to="/sign-in" replace />
  }
  if (needsRole && !me.data.role) {
    return <Navigate to="/choose-role" replace />
  }
  if (!needsRole && me.data.role) {
    return <Navigate to="/" replace />
  }
  return <Outlet />
}
