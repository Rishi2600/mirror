import type { JobView } from '../api/types'

function payRange(view: JobView) {
  const n = (v: number) => v.toLocaleString()
  if (view.pay_min > 0 && view.pay_max > 0) {
    return `${n(view.pay_min)} - ${n(view.pay_max)} ${view.pay_currency} a year`
  }
  if (view.pay_min > 0) {
    return `from ${n(view.pay_min)} ${view.pay_currency} a year`
  }
  if (view.pay_max > 0) {
    return `up to ${n(view.pay_max)} ${view.pay_currency} a year`
  }
  return ''
}

// JobCard is a hiring card as candidates see it. The employer's preview uses
// it too, so both always look the same.
export function JobCard({ view }: { view: JobView }) {
  const facts = [view.location, view.work_mode, view.seniority].filter(Boolean)
  const pay = payRange(view)

  return (
    <article data-testid="job-card" className="flex flex-col gap-4 rounded-lg border border-gray-200 p-4 shadow-sm">
      <header className="flex items-center gap-4">
        {view.image_url ? (
          <img src={view.image_url} alt={view.company_name || view.title} className="h-20 w-20 rounded-lg object-cover" />
        ) : (
          <div className="flex h-20 w-20 items-center justify-center rounded-lg bg-gray-200 text-2xl font-semibold text-gray-500">
            {(view.company_name || view.title).charAt(0)}
          </div>
        )}
        <div>
          <h2 className="text-xl font-semibold">{view.title}</h2>
          {view.company_name && <p className="text-gray-700">{view.company_name}</p>}
          {view.employer_first_name && <p className="text-sm text-gray-500">Posted by {view.employer_first_name}</p>}
        </div>
      </header>

      {facts.length > 0 && <p className="text-sm text-gray-700">{facts.join(' · ')}</p>}
      {pay && <p className="text-sm font-medium">{pay}</p>}

      {view.skills.length > 0 && (
        <ul className="flex flex-wrap gap-2">
          {view.skills.map((skill) => (
            <li key={skill} className="rounded-full bg-gray-100 px-3 py-1 text-sm">
              {skill}
            </li>
          ))}
        </ul>
      )}

      {view.summary && <p className="text-sm text-gray-800">{view.summary}</p>}
      {view.visa_sponsorship && <p className="text-sm text-green-700">Visa sponsorship available</p>}
    </article>
  )
}
