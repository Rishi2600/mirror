import { useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { Link, Navigate } from 'react-router'
import type { Profile } from '../api/types'
import { ErrorMessage } from '../components/form'
import { api } from '../lib/api'
import { useMe } from '../lib/useMe'
import { profileQueryKey, useProfile } from '../lib/useProfile'
import { BasicsStep } from './profile/BasicsStep'
import { CVStep } from './profile/CVStep'
import { ExperienceStep } from './profile/ExperienceStep'
import { PreviewStep } from './profile/PreviewStep'
import { SkillsStep } from './profile/SkillsStep'
import type { ProfileChanges } from './profile/types'
import { WantsStep } from './profile/WantsStep'

const stepNames = ['Your CV', 'The basics', 'Experience', 'Skills and summary', 'What you want', 'Preview']
const previewStep = stepNames.length - 1

// Where a candidate lands: the preview once the card is live, else the
// first step that still needs them.
function startStep(p: Profile | null) {
  if (p?.onboarded) {
    return previewStep
  }
  return p?.parse_status === 'done' ? 1 : 0
}

// ProfilePage loads the candidate's profile, then shows the card builder.
export function ProfilePage() {
  const me = useMe()
  const profile = useProfile()

  if (me.data?.role !== 'candidate') {
    return <Navigate to="/" replace />
  }
  if (profile.isPending) {
    return <p className="p-6 text-gray-600">Loading...</p>
  }
  if (profile.isError) {
    return <ErrorMessage message="Could not load your profile. Reload the page to try again." />
  }
  return <CardBuilder profile={profile.data} />
}

// CardBuilder builds the card step by step. The start step is picked once,
// when the page opens; after that only the candidate moves between steps
// (so the page never jumps when the CV finishes reading).
function CardBuilder({ profile: p }: { profile: Profile | null }) {
  const queryClient = useQueryClient()
  const [current, setCurrent] = useState(() => startStep(p))
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const canJump = p?.parse_status === 'done'
  const go = (to: number) => {
    setError('')
    setCurrent(to)
  }

  async function next(changes?: ProfileChanges) {
    if (!changes || !p) {
      go(current + 1)
      return
    }
    setBusy(true)
    setError('')
    try {
      const saved = await api.updateProfile(
        { ...p.cv, ...changes.cv },
        { ...p.preferences, ...changes.preferences },
        { ...p.card_settings, ...changes.card_settings },
      )
      queryClient.setQueryData(profileQueryKey, saved)
      go(current + 1)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Something went wrong; try again')
    } finally {
      setBusy(false)
    }
  }

  const stepProps = { busy, onBack: () => go(current - 1), onNext: next }
  return (
    <main className="mx-auto flex min-h-screen max-w-xl flex-col gap-6 p-6">
      <div className="flex items-center justify-between">
        <h1 className="text-3xl font-semibold">Your card</h1>
        <Link to="/" className="text-sm text-blue-700 underline">
          Home
        </Link>
      </div>

      <nav aria-label="Steps" className="flex gap-1">
        {stepNames.map((name, i) => (
          <button
            key={name}
            type="button"
            aria-label={name}
            aria-current={i === current ? 'step' : undefined}
            disabled={!canJump}
            onClick={() => go(i)}
            className={`h-2 flex-1 rounded ${i <= current ? 'bg-gray-900' : 'bg-gray-200'}`}
          />
        ))}
      </nav>
      <h2 data-testid="step-title" className="text-lg font-semibold">
        Step {current + 1} of {stepNames.length}: {stepNames[current]}
      </h2>

      {current === 0 && <CVStep profile={p} busy={busy} onNext={() => go(1)} />}
      {p && current === 1 && <BasicsStep profile={p} {...stepProps} />}
      {p && current === 2 && <ExperienceStep profile={p} {...stepProps} />}
      {p && current === 3 && <SkillsStep profile={p} {...stepProps} />}
      {p && current === 4 && <WantsStep profile={p} {...stepProps} />}
      {p && current === previewStep && <PreviewStep onBack={() => go(current - 1)} />}
      <ErrorMessage message={error} />
    </main>
  )
}
