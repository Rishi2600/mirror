import { useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { Link, Navigate, useParams } from 'react-router'
import type { HiringCard, HiringFields } from '../api/types'
import { ErrorMessage } from '../components/form'
import { api } from '../lib/api'
import { hiringCardQueryKey, hiringCardsQueryKey, useHiringCard } from '../lib/useHiringCards'
import { useMe } from '../lib/useMe'
import { HiringPreviewStep } from './hiring/HiringPreviewStep'
import { RoleStep } from './hiring/RoleStep'
import { fieldsOf } from './hiring/types'
import { WherePayStep } from './hiring/WherePayStep'
import { YouStep } from './hiring/YouStep'

// Step 1 (the job description) is NewHiringCardPage; this page has 2 to 5.
const stepNames = ['The role', 'Where and pay', 'You and the image', 'Preview']
const previewStep = stepNames.length - 1

// HiringCardPage loads one of the employer's cards, then shows its builder.
export function HiringCardPage() {
  const { id = '' } = useParams()
  const me = useMe()
  const card = useHiringCard(id)

  if (me.data?.role !== 'employer') {
    return <Navigate to="/" replace />
  }
  if (card.isPending) {
    return <p className="p-6 text-gray-600">Loading...</p>
  }
  if (card.isError) {
    return <ErrorMessage message="This hiring card does not exist, or is not yours." />
  }
  return <HiringBuilder card={card.data} />
}

// HiringBuilder: a draft starts at the first step; a live or closed card
// opens on its preview. Like the candidate builder, the start step is
// picked once.
function HiringBuilder({ card }: { card: HiringCard }) {
  const queryClient = useQueryClient()
  const [current, setCurrent] = useState(() => (card.status === 'draft' ? 0 : previewStep))
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const go = (to: number) => {
    setError('')
    setCurrent(to)
  }

  async function next(changes?: Partial<HiringFields>) {
    if (!changes) {
      go(current + 1)
      return
    }
    setBusy(true)
    setError('')
    try {
      const saved = await api.updateHiringCard(card.id, { ...fieldsOf(card), ...changes })
      queryClient.setQueryData(hiringCardQueryKey(card.id), saved)
      queryClient.invalidateQueries({ queryKey: hiringCardsQueryKey })
      go(current + 1)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Something went wrong; try again')
    } finally {
      setBusy(false)
    }
  }

  const stepProps = { card, busy, onBack: () => go(current - 1), onNext: next }
  return (
    <main className="mx-auto flex min-h-screen max-w-xl flex-col gap-6 p-6">
      <div className="flex items-center justify-between">
        <h1 className="text-3xl font-semibold">{card.title || 'Hiring card'}</h1>
        <Link to="/" className="text-sm text-blue-700 underline">
          Your cards
        </Link>
      </div>

      <nav aria-label="Steps" className="flex gap-1">
        {stepNames.map((name, i) => (
          <button
            key={name}
            type="button"
            aria-label={name}
            aria-current={i === current ? 'step' : undefined}
            onClick={() => go(i)}
            className={`h-2 flex-1 rounded ${i <= current ? 'bg-gray-900' : 'bg-gray-200'}`}
          />
        ))}
      </nav>
      <h2 data-testid="step-title" className="text-lg font-semibold">
        Step {current + 2} of 5: {stepNames[current]}
      </h2>

      {current === 0 && <RoleStep {...stepProps} />}
      {current === 1 && <WherePayStep {...stepProps} />}
      {current === 2 && <YouStep {...stepProps} />}
      {current === previewStep && <HiringPreviewStep card={card} onBack={() => go(current - 1)} />}
      <ErrorMessage message={error} />
    </main>
  )
}
