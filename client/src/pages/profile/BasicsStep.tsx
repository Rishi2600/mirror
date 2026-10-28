import { useQueryClient } from '@tanstack/react-query'
import { useState, type ChangeEvent } from 'react'
import { ErrorMessage, Field, inputClass } from '../../components/form'
import { api } from '../../lib/api'
import { profileQueryKey } from '../../lib/useProfile'
import { StepNav } from './StepNav'
import type { StepProps } from './types'

// Step 2: photo, name, headline and city.
export function BasicsStep({ profile, busy, onBack, onNext }: StepProps) {
  const queryClient = useQueryClient()
  const [fullName, setFullName] = useState(profile.cv.full_name)
  const [headline, setHeadline] = useState(profile.cv.headline)
  const [location, setLocation] = useState(profile.cv.location)
  const [photoError, setPhotoError] = useState('')
  const [uploading, setUploading] = useState(false)

  async function uploadPhoto(e: ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0]
    e.target.value = ''
    if (!file) {
      return
    }
    setUploading(true)
    setPhotoError('')
    try {
      queryClient.setQueryData(profileQueryKey, await api.uploadPhoto(file))
    } catch (err) {
      setPhotoError(err instanceof Error ? err.message : 'Something went wrong; try again')
    } finally {
      setUploading(false)
    }
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center gap-4">
        {profile.photo_url ? (
          <img src={profile.photo_url} alt="Your photo" className="h-24 w-24 rounded-full object-cover" />
        ) : (
          <div className="h-24 w-24 rounded-full bg-gray-200" />
        )}
        <label className="flex flex-col gap-1 text-sm text-gray-700">
          {profile.photo_url ? 'Change your photo' : 'Add a photo'}
          <input id="photo-file" type="file" accept="image/jpeg,image/png,image/webp" disabled={uploading} onChange={uploadPhoto} />
          <span className="text-xs text-gray-500">JPEG, PNG or WebP. We remove the photo's location data.</span>
        </label>
      </div>
      <ErrorMessage message={photoError} />

      <Field label="Full name">
        <input value={fullName} onChange={(e) => setFullName(e.target.value)} className={inputClass} />
      </Field>
      <Field label="Headline">
        <input
          value={headline}
          onChange={(e) => setHeadline(e.target.value)}
          placeholder="Senior backend engineer"
          className={inputClass}
        />
      </Field>
      <Field label="City">
        <input value={location} onChange={(e) => setLocation(e.target.value)} className={inputClass} />
      </Field>

      <StepNav
        busy={busy || uploading}
        onBack={onBack}
        onNext={() => onNext({ cv: { full_name: fullName, headline, location } })}
      />
    </div>
  )
}
