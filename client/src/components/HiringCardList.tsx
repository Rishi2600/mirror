import { Link } from 'react-router'
import { useHiringCards } from '../lib/useHiringCards'
import { ErrorMessage } from './form'

const statusLabels = { draft: 'Draft', live: 'Live', closed: 'Closed' }
const statusClasses = {
  draft: 'bg-amber-100 text-amber-900',
  live: 'bg-green-100 text-green-800',
  closed: 'bg-gray-200 text-gray-700',
}

// HiringCardList is the employer's home: their hiring cards and a way to
// create another. Each card will open its own candidate timeline.
export function HiringCardList() {
  const cards = useHiringCards()

  if (cards.isPending) {
    return <p className="text-gray-600">Loading your cards...</p>
  }
  if (cards.isError) {
    return <ErrorMessage message="Could not load your cards. Reload the page to try again." />
  }

  return (
    <section className="flex flex-col gap-3">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold">Your hiring cards</h2>
        <Link to="/hiring/new" className="rounded bg-gray-900 px-3 py-2 text-sm font-medium text-white">
          Create a new card
        </Link>
      </div>
      {cards.data.length === 0 && <p className="text-gray-600">No cards yet. Create one for each role you are hiring for.</p>}
      <ul className="flex flex-col gap-2">
        {cards.data.map((card) => (
          <li key={card.id}>
            <Link
              to={`/hiring/${card.id}`}
              data-testid="hiring-card-item"
              className="flex items-center gap-3 rounded border border-gray-200 p-3 hover:border-gray-900"
            >
              {card.image_url ? (
                <img src={card.image_url} alt="" className="h-12 w-12 rounded object-cover" />
              ) : (
                <div className="h-12 w-12 rounded bg-gray-200" />
              )}
              <div className="flex-1">
                <p className="font-medium">{card.title || 'Untitled role'}</p>
                {card.company_name && <p className="text-sm text-gray-600">{card.company_name}</p>}
              </div>
              <span className={`rounded-full px-2 py-1 text-xs font-medium ${statusClasses[card.status]}`}>
                {statusLabels[card.status]}
              </span>
            </Link>
          </li>
        ))}
      </ul>
    </section>
  )
}
