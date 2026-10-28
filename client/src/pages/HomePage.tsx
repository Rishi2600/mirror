import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Link } from 'react-router'
import { HiringCardList } from '../components/HiringCardList'
import { ServerStatus } from '../components/ServerStatus'
import { api } from '../lib/api'
import { meQueryKey, useMe } from '../lib/useMe'

const roleLabels = { candidate: 'Candidate', employer: 'Employer', admin: 'Admin' }

export function HomePage() {
  const me = useMe()
  const health = useQuery({ queryKey: ['health'], queryFn: api.getHealth, retry: false })
  const queryClient = useQueryClient()

  async function logout() {
    await api.logout()
    // The route guard sends the user to /sign-in.
    queryClient.setQueryData(meQueryKey, null)
  }

  const user = me.data
  return (
    <main className="mx-auto flex min-h-screen max-w-md flex-col justify-center gap-6 p-6">
      <h1 className="text-3xl font-semibold">Mirror</h1>
      {user && user.role && (
        <section className="flex items-center justify-between rounded border border-gray-200 p-3">
          <div>
            <p data-testid="user-email" className="font-medium">
              {user.email}
            </p>
            <p data-testid="user-role" className="text-sm text-gray-600">
              {roleLabels[user.role]}
            </p>
          </div>
          <button type="button" onClick={logout} className="rounded border border-gray-300 px-3 py-1 text-sm">
            Log out
          </button>
        </section>
      )}
      {user?.role === 'candidate' && (
        <div className="flex flex-col gap-2">
          <Link to="/timeline" className="rounded bg-gray-900 px-3 py-2 text-center font-medium text-white">
            Your timeline
          </Link>
          <Link to="/profile" className="rounded border border-gray-300 px-3 py-2 text-center font-medium">
            Your card
          </Link>
          <Link to="/matches" className="rounded border border-gray-300 px-3 py-2 text-center font-medium">
            Your matches
          </Link>
        </div>
      )}
      {user?.role === 'employer' && (
        <>
          <Link to="/matches" className="rounded border border-gray-300 px-3 py-2 text-center font-medium">
            Your matches
          </Link>
          <HiringCardList />
        </>
      )}
      <ServerStatus health={health.data} isLoading={health.isPending} isError={health.isError} />
    </main>
  )
}
