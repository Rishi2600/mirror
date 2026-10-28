import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router'
import type { Contact } from '../api/types'
import { CandidateCard } from '../components/CandidateCard'
import { ErrorMessage } from '../components/form'
import { JobCard } from '../components/JobCard'
import { api } from '../lib/api'

// MatchPage shows one match. The employer sees the candidate's contact
// details and CV; the candidate sees the job and can unmatch.
export function MatchPage() {
  const { id = '' } = useParams()
  const match = useQuery({ queryKey: ['match', id], queryFn: () => api.getMatch(id), retry: false })

  return (
    <main className="mx-auto flex min-h-screen max-w-xl flex-col gap-6 p-6">
      <div className="flex items-center justify-between">
        <h1 className="text-3xl font-semibold">Match</h1>
        <Link to="/matches" className="text-sm text-blue-700 underline">
          All matches
        </Link>
      </div>

      {match.isPending && <p className="text-gray-600">Loading the match...</p>}
      {match.isError && <ErrorMessage message="This match does not exist or has ended." />}
      {match.data?.contact && <ContactDetails contact={match.data.contact} />}
      {match.data?.candidate && <CandidateCard card={match.data.candidate} />}
      {match.data?.job && (
        <>
          <JobCard view={match.data.job} />
          <p className="text-gray-600">You and {match.data.job.employer_first_name || 'the employer'} both said yes. Chat opens here soon.</p>
          <Unmatch id={id} />
        </>
      )}
    </main>
  )
}

function ContactDetails({ contact }: { contact: Contact }) {
  return (
    <section data-testid="contact" className="flex flex-col gap-1 rounded-lg border border-green-300 bg-green-50 p-4">
      <h2 className="text-lg font-semibold">{contact.full_name}</h2>
      <a href={`mailto:${contact.email}`} className="text-blue-700 underline">
        {contact.email}
      </a>
      {contact.phone && <p>{contact.phone}</p>}
      {contact.cv_url && (
        <a href={contact.cv_url} className="mt-2 self-start rounded bg-gray-900 px-3 py-2 text-sm font-medium text-white">
          Download CV
        </a>
      )}
    </section>
  )
}

// Unmatch asks once more before ending the match.
function Unmatch({ id }: { id: string }) {
  const [confirming, setConfirming] = useState(false)
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const unmatch = useMutation({
    mutationFn: () => api.unmatch(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['matches'] })
      navigate('/matches', { replace: true })
    },
  })

  if (!confirming) {
    return (
      <button type="button" onClick={() => setConfirming(true)} className="self-start text-sm text-red-700 underline">
        Unmatch
      </button>
    )
  }
  return (
    <div className="flex flex-col gap-3 rounded-lg border border-red-300 bg-red-50 p-4">
      <p>The employer will no longer see your contact details or CV. This cannot be undone.</p>
      <div className="flex gap-3">
        <button
          type="button"
          onClick={() => unmatch.mutate()}
          disabled={unmatch.isPending}
          className="rounded bg-red-700 px-3 py-2 text-sm font-medium text-white disabled:opacity-50"
        >
          Yes, unmatch
        </button>
        <button type="button" onClick={() => setConfirming(false)} className="rounded border border-gray-300 px-3 py-2 text-sm">
          Keep the match
        </button>
      </div>
      <ErrorMessage message={unmatch.isError ? 'Could not unmatch. Try again.' : ''} />
    </div>
  )
}
