import { useQuery } from '@tanstack/react-query'
import { ServerStatus } from '../components/ServerStatus'
import { api } from '../lib/api'

async function fetchHealth() {
  const { data, response } = await api.GET('/v1/healthz')
  if (!data) {
    throw new Error(`API answered ${response.status}`)
  }
  return data
}

export function HomePage() {
  const health = useQuery({ queryKey: ['health'], queryFn: fetchHealth, retry: false })

  return (
    <main className="mx-auto flex min-h-screen max-w-md flex-col justify-center gap-6 p-6">
      <h1 className="text-3xl font-semibold">Mirror</h1>
      <ServerStatus
        health={health.data}
        isLoading={health.isPending}
        isError={health.isError}
      />
    </main>
  )
}
