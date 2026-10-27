import { useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { api } from '../lib/api'
import { meQueryKey } from '../lib/useMe'

const choiceClass = 'w-full rounded border border-gray-300 p-4 text-left hover:border-gray-900 disabled:opacity-50'

// ChooseRolePage asks a new user, once, whether they are hiring or job hunting.
export function ChooseRolePage() {
  const queryClient = useQueryClient()
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  async function choose(role: 'candidate' | 'employer') {
    setBusy(true)
    setError('')
    try {
      const user = await api.setRole(role)
      // The route guard sends the user on once the role is set.
      queryClient.setQueryData(meQueryKey, user)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Something went wrong; try again')
      setBusy(false)
    }
  }

  return (
    <main className="mx-auto flex min-h-screen max-w-sm flex-col justify-center gap-4 p-6">
      <h1 className="text-3xl font-semibold">How will you use Mirror?</h1>
      <p className="text-sm text-gray-700">You pick this once.</p>
      <button type="button" disabled={busy} onClick={() => choose('candidate')} className={choiceClass}>
        <span className="block font-medium">I'm looking for a job</span>
        <span className="text-sm text-gray-600">Swipe on roles that fit you</span>
      </button>
      <button type="button" disabled={busy} onClick={() => choose('employer')} className={choiceClass}>
        <span className="block font-medium">I'm hiring</span>
        <span className="text-sm text-gray-600">Post roles and swipe on candidates</span>
      </button>
      {error && (
        <p role="alert" className="rounded border border-red-300 bg-red-50 p-3 text-red-800">
          {error}
        </p>
      )}
    </main>
  )
}
