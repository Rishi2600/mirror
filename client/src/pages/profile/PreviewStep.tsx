import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { CandidateCard } from '../../components/CandidateCard'
import { ErrorMessage } from '../../components/form'
import { api } from '../../lib/api'
import { profileQueryKey } from '../../lib/useProfile'
import { StepNav } from './StepNav'

type Props = {
  onBack: () => void
}

function plural(n: number, word: string) {
  return `${n} ${word}${n === 1 ? '' : 's'}`
}

// Step 6: the card exactly as employers see it. Finishing makes it live.
export function PreviewStep({ onBack }: Props) {
  const queryClient = useQueryClient()
  // Always fetched fresh, so it shows the latest saved steps.
  const preview = useQuery({ queryKey: ['card'], queryFn: api.getCard, gcTime: 0 })
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  async function finish() {
    setBusy(true)
    setError('')
    try {
      queryClient.setQueryData(profileQueryKey, await api.finishCard())
      await preview.refetch()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Something went wrong; try again')
    } finally {
      setBusy(false)
    }
  }

  if (preview.isPending) {
    return <p className="text-gray-600">Loading your card...</p>
  }
  if (preview.isError) {
    return <ErrorMessage message="Could not load your card. Reload the page to try again." />
  }

  const { card, removed, missing, onboarded } = preview.data
  const removedParts = [
    removed.emails > 0 && plural(removed.emails, 'email address'),
    removed.phones > 0 && plural(removed.phones, 'phone number'),
    removed.links > 0 && plural(removed.links, 'link'),
  ].filter(Boolean)

  return (
    <div className="flex flex-col gap-4">
      <p className="text-sm text-gray-700">This is exactly what employers see.</p>
      <CandidateCard card={card} />
      <p className="text-sm text-gray-600">Hidden until you match: your email, phone, links and CV.</p>
      {removedParts.length > 0 && (
        <p data-testid="removed" className="rounded border border-amber-300 bg-amber-50 p-3 text-sm text-amber-900">
          We removed {removedParts.join(', ')} from your card's text. Employers get your contact details when you match.
        </p>
      )}
      {missing.length > 0 && <ErrorMessage message={`Before you finish, add ${missing.join(', ')}.`} />}
      <ErrorMessage message={error} />

      {onboarded ? (
        <p data-testid="live" className="rounded border border-green-300 bg-green-50 p-3 text-green-800">
          Your card is live. Changes you make show up for employers at once.
        </p>
      ) : (
        <StepNav busy={busy} onBack={onBack} onNext={finish} nextLabel="Finish" nextDisabled={missing.length > 0} />
      )}
    </div>
  )
}
