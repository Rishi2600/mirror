import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router'
import type { Contact } from '../api/types'
import { CandidateCard } from '../components/CandidateCard'
import { Chat } from '../components/chat/Chat'
import { ErrorMessage } from '../components/form'
import { JobCard } from '../components/JobCard'
import { api } from '../lib/api'
import { useMe } from '../lib/useMe'

// MatchPage is one match: the chat, and below it the other side's card.
// The employer also sees the candidate's contact details and CV. Either
// side can end the match.
export function MatchPage() {
  const { id = '' } = useParams()
  const me = useMe()
  const match = useQuery({ queryKey: ['match', id], queryFn: () => api.getMatch(id), retry: false })
  const m = match.isError ? undefined : match.data
  const otherName = m?.job ? m.job.employer_first_name || 'the employer' : (m?.candidate?.name ?? '')
  const title = m?.job ? `${m.job.title}${m.job.company_name ? `, ${m.job.company_name}` : ''}` : (m?.hiring_card_title ?? '')

  return (
    <main className="mx-auto flex min-h-screen max-w-xl flex-col gap-6 p-6">
      <div className="flex items-center justify-between gap-3">
        <div>
          <h1 className="text-2xl font-semibold">{m ? otherName : 'Match'}</h1>
          {m && <p className="text-sm text-gray-600">{title}</p>}
        </div>
        <Link to="/matches" className="text-sm text-blue-700 underline">
          All matches
        </Link>
      </div>

      {match.isPending && <p className="text-gray-600">Loading the match...</p>}
      {match.isError && <ErrorMessage message="This match does not exist or has ended." />}
      {m && me.data && <Chat matchId={id} myId={me.data.id} otherName={otherName} />}
      {m?.contact && <ContactDetails contact={m.contact} />}
      {m?.candidate && <CandidateCard card={m.candidate} />}
      {m?.job && <JobCard view={m.job} />}
      {m && <Unmatch id={id} employer={!!m.candidate} />}
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
function Unmatch({ id, employer }: { id: string; employer: boolean }) {
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
      <p>
        {employer
          ? 'You will lose the contact details, the CV and the chat.'
          : 'The employer will no longer see your contact details or CV.'}{' '}
        The chat closes for both of you. This cannot be undone.
      </p>
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
