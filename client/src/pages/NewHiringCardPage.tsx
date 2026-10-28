import { useQueryClient } from '@tanstack/react-query'
import { useState, type FormEvent } from 'react'
import { Link, Navigate, useNavigate } from 'react-router'
import { ErrorMessage, inputClass } from '../components/form'
import { api } from '../lib/api'
import { hiringCardQueryKey, hiringCardsQueryKey } from '../lib/useHiringCards'
import { useMe } from '../lib/useMe'

// NewHiringCardPage: step 1 of a hiring card. The AI reads the pasted job
// description and fills in the next steps.
export function NewHiringCardPage() {
  const me = useMe()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [description, setDescription] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  if (me.data?.role !== 'employer') {
    return <Navigate to="/" replace />
  }

  async function create(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    setError('')
    try {
      const card = await api.createHiringCard(description)
      queryClient.setQueryData(hiringCardQueryKey(card.id), card)
      queryClient.invalidateQueries({ queryKey: hiringCardsQueryKey })
      navigate(`/hiring/${card.id}`)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Something went wrong; try again')
      setBusy(false)
    }
  }

  return (
    <main className="mx-auto flex min-h-screen max-w-xl flex-col gap-6 p-6">
      <div className="flex items-center justify-between">
        <h1 className="text-3xl font-semibold">New hiring card</h1>
        <Link to="/" className="text-sm text-blue-700 underline">
          Home
        </Link>
      </div>
      <h2 className="text-lg font-semibold">Step 1 of 5: The job description</h2>
      <form onSubmit={create} className="flex flex-col gap-3">
        <label htmlFor="description" className="text-sm text-gray-700">
          Paste the job description. We fill in the card for you; you can change everything next.
        </label>
        <textarea
          id="description"
          rows={12}
          required
          value={description}
          onChange={(e) => setDescription(e.target.value)}
          className={inputClass}
        />
        <button type="submit" disabled={busy} className="rounded bg-gray-900 px-4 py-2 font-medium text-white disabled:opacity-50">
          {busy ? 'Reading the description...' : 'Next'}
        </button>
      </form>
      <ErrorMessage message={error} />
    </main>
  )
}
