import { Link, Navigate } from 'react-router'
import { CVUpload } from '../components/CVUpload'
import { ProfileForm } from '../components/ProfileForm'
import { useMe } from '../lib/useMe'
import { useProfile } from '../lib/useProfile'

// ProfilePage: upload a CV, wait while it is read, then edit the profile.
export function ProfilePage() {
  const me = useMe()
  const profile = useProfile()

  if (me.data?.role !== 'candidate') {
    return <Navigate to="/" replace />
  }

  const p = profile.data
  return (
    <main className="mx-auto flex min-h-screen max-w-xl flex-col gap-6 p-6">
      <div className="flex items-center justify-between">
        <h1 className="text-3xl font-semibold">Your profile</h1>
        <Link to="/" className="text-sm text-blue-700 underline">
          Home
        </Link>
      </div>

      {profile.isPending && <p className="text-gray-600">Loading...</p>}
      {profile.isError && (
        <p role="alert" className="rounded border border-red-300 bg-red-50 p-3 text-red-800">
          Could not load your profile. Reload the page to try again.
        </p>
      )}

      {profile.isSuccess && !p && (
        <>
          <p className="text-gray-700">Upload your CV and we will fill in your profile for you.</p>
          <CVUpload label="Your CV" />
        </>
      )}

      {p?.parse_status === 'pending' && (
        <p data-testid="parsing" className="rounded border border-gray-200 p-4 text-gray-700">
          Reading your CV... this usually takes a few seconds.
        </p>
      )}

      {p?.parse_status === 'failed' && (
        <>
          <p role="alert" className="rounded border border-red-300 bg-red-50 p-3 text-red-800">
            {p.parse_error}
          </p>
          <CVUpload label="Try another CV" />
        </>
      )}

      {p?.parse_status === 'done' && (
        <>
          <ProfileForm profile={p} />
          <CVUpload label="Upload a different CV" />
        </>
      )}
    </main>
  )
}
