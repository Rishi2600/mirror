import { useQueryClient } from '@tanstack/react-query'
import { useState, type ChangeEvent } from 'react'
import { ErrorMessage, Field, inputClass } from '../../components/form'
import { api } from '../../lib/api'
import { hiringCardQueryKey, hiringCardsQueryKey } from '../../lib/useHiringCards'
import { meQueryKey, useMe } from '../../lib/useMe'
import { StepNav } from '../profile/StepNav'
import type { HiringStepProps } from './types'

// Step 4: the employer's first name and the card's image (a logo or a photo).
export function YouStep({ card, busy, onBack, onNext }: HiringStepProps) {
  const me = useMe()
  const queryClient = useQueryClient()
  const [firstName, setFirstName] = useState(me.data?.first_name ?? '')
  const [error, setError] = useState('')
  const [working, setWorking] = useState(false)

  async function uploadImage(e: ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0]
    e.target.value = ''
    if (!file) {
      return
    }
    setWorking(true)
    setError('')
    try {
      queryClient.setQueryData(hiringCardQueryKey(card.id), await api.uploadHiringCardImage(card.id, file))
      queryClient.invalidateQueries({ queryKey: hiringCardsQueryKey })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Something went wrong; try again')
    } finally {
      setWorking(false)
    }
  }

  // The first name belongs to the employer, not the card, so it is saved
  // on its own before moving on.
  async function saveName() {
    setWorking(true)
    setError('')
    try {
      queryClient.setQueryData(meQueryKey, await api.updateMe(firstName))
      onNext()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Something went wrong; try again')
    } finally {
      setWorking(false)
    }
  }

  return (
    <div className="flex flex-col gap-4">
      <Field label="Your first name">
        <input value={firstName} onChange={(e) => setFirstName(e.target.value)} className={inputClass} />
      </Field>
      <p className="text-xs text-gray-500">Candidates see "Posted by" your first name. Your email and phone stay hidden until you match.</p>

      <div className="flex items-center gap-4">
        {card.image_url ? (
          <img src={card.image_url} alt="Card image" className="h-24 w-24 rounded-lg object-cover" />
        ) : (
          <div className="h-24 w-24 rounded-lg bg-gray-200" />
        )}
        <label className="flex flex-col gap-1 text-sm text-gray-700">
          {card.image_url ? 'Change the image' : 'Add a company logo or your photo (optional)'}
          <input id="card-image" type="file" accept="image/jpeg,image/png,image/webp" disabled={working} onChange={uploadImage} />
        </label>
      </div>
      <ErrorMessage message={error} />

      <StepNav busy={busy || working} onBack={onBack} onNext={saveName} nextLabel="Preview the card" />
    </div>
  )
}
