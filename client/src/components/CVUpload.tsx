import { useQueryClient } from '@tanstack/react-query'
import { useState, type ChangeEvent } from 'react'
import { api } from '../lib/api'
import { profileQueryKey } from '../lib/useProfile'

type Props = {
  label: string
}

// CVUpload sends a PDF or DOCX as soon as the candidate picks it.
export function CVUpload({ label }: Props) {
  const queryClient = useQueryClient()
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  async function upload(e: ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0]
    e.target.value = '' // so picking the same file again still uploads
    if (!file) {
      return
    }
    setBusy(true)
    setError('')
    try {
      queryClient.setQueryData(profileQueryKey, await api.uploadCV(file))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Something went wrong; try again')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="flex flex-col gap-2">
      <label htmlFor="cv-file" className="text-sm font-medium text-gray-700">
        {label}
      </label>
      <input id="cv-file" type="file" accept=".pdf,.docx" disabled={busy} onChange={upload} className="text-sm" />
      <p className="text-xs text-gray-500">PDF or DOCX, up to 5 MB. A LinkedIn PDF export works too.</p>
      {error && (
        <p role="alert" className="rounded border border-red-300 bg-red-50 p-3 text-red-800">
          {error}
        </p>
      )}
    </div>
  )
}
