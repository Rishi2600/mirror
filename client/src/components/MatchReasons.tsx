// MatchReasons shows how well a card fits, and why ("why this card").
export function MatchReasons({ score, reasons }: { score: number; reasons: string[] }) {
  const percent = Math.round(score * 100)
  return (
    <div className="flex flex-col gap-2 px-1">
      <div className="flex items-center gap-2 text-sm">
        <span className="font-medium">{percent}% fit</span>
        <div className="h-2 flex-1 rounded bg-gray-200" aria-hidden="true">
          <div className="h-2 rounded bg-green-600" style={{ width: `${percent}%` }} />
        </div>
      </div>
      {reasons.length > 0 && (
        <ul className="flex flex-wrap gap-2 text-xs" aria-label="Why this card">
          {reasons.map((reason) => (
            <li key={reason} className="rounded-full border border-green-300 bg-green-50 px-2 py-1 text-green-900">
              {reason}
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
