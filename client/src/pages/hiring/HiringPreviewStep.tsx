import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { Link } from 'react-router'
import type { HiringCard } from '../../api/types'
import { ErrorMessage } from '../../components/form'
import { JobCard } from '../../components/JobCard'
import { api } from '../../lib/api'
import { hiringCardQueryKey, hiringCardsQueryKey } from '../../lib/useHiringCards'
import { StepNav } from '../profile/StepNav'

type Props = {
  card: HiringCard
  onBack: () => void
}

// Step 5: the card exactly as candidates see it, and its status.
export function HiringPreviewStep({ card, onBack }: Props) {
  const queryClient = useQueryClient()
  // Always fetched fresh, so it shows the latest saved steps.
  const preview = useQuery({ queryKey: ['hiring-preview', card.id], queryFn: () => api.previewHiringCard(card.id), gcTime: 0 })
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  async function setStatus(status: 'live' | 'closed') {
    setBusy(true)
    setError('')
    try {
      queryClient.setQueryData(hiringCardQueryKey(card.id), await api.setHiringCardStatus(card.id, status))
      queryClient.invalidateQueries({ queryKey: hiringCardsQueryKey })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Something went wrong; try again')
    } finally {
      setBusy(false)
    }
  }

  if (preview.isPending) {
    return <p className="text-gray-600">Loading the card...</p>
  }
  if (preview.isError) {
    return <ErrorMessage message="Could not load the card. Reload the page to try again." />
  }
  const { view, removed, missing } = preview.data
  const removedCount = removed.emails + removed.phones + removed.links

  return (
    <div className="flex flex-col gap-4">
      <p className="text-sm text-gray-700">This is exactly what candidates see.</p>
      <JobCard view={view} />
      {removedCount > 0 && (
        <p className="rounded border border-amber-300 bg-amber-50 p-3 text-sm text-amber-900">
          We removed {removedCount} contact detail{removedCount === 1 ? '' : 's'} (emails, phones or links) from the card. Candidates
          reach you through Mirror when you match.
        </p>
      )}
      {missing.length > 0 && <ErrorMessage message={`Before this card can go live, add ${missing.join(', ')}.`} />}
      <ErrorMessage message={error} />

      {card.status === 'live' ? (
        <div className="flex items-center justify-between gap-3">
          <div className="flex flex-col gap-1">
            <p data-testid="card-live" className="text-green-800">
              Live: fitting candidates see this card.
            </p>
            <Link to={`/hiring/${card.id}/timeline`} className="text-sm text-blue-700 underline">
              See fitting candidates
            </Link>
          </div>
          <button type="button" disabled={busy} onClick={() => setStatus('closed')} className="rounded border border-gray-300 px-4 py-2">
            Close card
          </button>
        </div>
      ) : (
        <>
          {card.status === 'closed' && (
            <p data-testid="card-closed" className="text-gray-700">
              Closed: candidates no longer see this card.
            </p>
          )}
          <StepNav
            busy={busy}
            onBack={onBack}
            onNext={() => setStatus('live')}
            nextLabel={card.status === 'closed' ? 'Open again' : 'Go live'}
            nextDisabled={missing.length > 0}
          />
        </>
      )}
    </div>
  )
}
