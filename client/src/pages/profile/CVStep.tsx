import type { Profile } from '../../api/types'
import { CVUpload } from '../../components/CVUpload'
import { StepNav } from './StepNav'

type Props = {
  profile: Profile | null
  busy: boolean
  onNext: () => void
}

// Step 1: the CV fills in most of the next steps.
export function CVStep({ profile, busy, onNext }: Props) {
  const status = profile?.parse_status
  return (
    <div className="flex flex-col gap-4">
      {!profile && <p className="text-gray-700">Upload your CV and we will fill in most of your card for you.</p>}

      {status === 'pending' && (
        <p data-testid="parsing" className="rounded border border-gray-200 p-4 text-gray-700">
          Reading your CV... this usually takes a few seconds.
        </p>
      )}
      {status === 'failed' && (
        <p role="alert" className="rounded border border-red-300 bg-red-50 p-3 text-red-800">
          {profile?.parse_error}
        </p>
      )}
      {status === 'done' && <p className="text-gray-700">We have read your CV. Check what we found in the next steps.</p>}

      {status !== 'pending' && <CVUpload label={profile ? 'Upload a different CV' : 'Your CV'} />}
      <StepNav busy={busy} onNext={onNext} nextDisabled={status !== 'done'} />
    </div>
  )
}
