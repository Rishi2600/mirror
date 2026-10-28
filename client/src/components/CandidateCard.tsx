import type { Card } from '../api/types'

// CandidateCard is the card employers see before a match. The candidate's
// preview uses it too, so both always look the same.
export function CandidateCard({ card }: { card: Card }) {
  const facts = [
    card.years_experience > 0 && `${card.years_experience} years`,
    card.seniority,
    card.work_mode !== 'any' && card.work_mode,
  ].filter(Boolean)

  return (
    <article data-testid="candidate-card" className="flex flex-col gap-4 rounded-lg border border-gray-200 p-4 shadow-sm">
      <header className="flex items-center gap-4">
        {card.photo_url ? (
          <img src={card.photo_url} alt={card.name} className="h-20 w-20 rounded-full object-cover" />
        ) : (
          <div className="h-20 w-20 rounded-full bg-gray-200" />
        )}
        <div>
          <h2 className="text-xl font-semibold">{card.name}</h2>
          <p className="text-gray-700">{card.headline}</p>
          {card.location && <p className="text-sm text-gray-500">{card.location}</p>}
        </div>
      </header>

      <p className="text-sm text-gray-700">
        {facts.join(' · ')}
        {card.current_employer && <> · Now at {card.current_employer}</>}
      </p>

      {card.skills.length > 0 && (
        <ul className="flex flex-wrap gap-2">
          {card.skills.map((skill) => (
            <li key={skill} className="rounded-full bg-gray-100 px-3 py-1 text-sm">
              {skill}
            </li>
          ))}
        </ul>
      )}

      {card.summary && <p className="text-sm text-gray-800">{card.summary}</p>}

      {card.experience.length > 0 && (
        <section className="text-sm">
          <h3 className="font-medium">Experience</h3>
          <ul className="mt-1 flex flex-col gap-2">
            {card.experience.map((job, i) => (
              <li key={i}>
                <span className="font-medium">{job.title}</span>
                {job.company && <>, {job.company}</>}
                <span className="text-gray-500">
                  {' '}
                  ({job.start} - {job.end})
                </span>
                {job.summary && <p className="text-gray-600">{job.summary}</p>}
              </li>
            ))}
          </ul>
        </section>
      )}

      {card.education.length > 0 && (
        <section className="text-sm">
          <h3 className="font-medium">Education</h3>
          {card.education.map((ed, i) => (
            <p key={i}>
              {ed.degree}, {ed.institution} {ed.year && `(${ed.year})`}
            </p>
          ))}
        </section>
      )}

      {(card.min_pay > 0 || card.role_types.length > 0) && (
        <p className="text-sm text-gray-700">
          <span className="font-medium">Looking for: </span>
          {[card.role_types.join(', '), card.min_pay > 0 && `from ${card.min_pay.toLocaleString()} ${card.pay_currency} a year`]
            .filter(Boolean)
            .join(' · ')}
        </p>
      )}
    </article>
  )
}
