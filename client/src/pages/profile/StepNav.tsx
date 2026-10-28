type Props = {
  busy: boolean
  onBack?: () => void
  onNext: () => void
  nextLabel?: string
  nextDisabled?: boolean
}

export function StepNav({ busy, onBack, onNext, nextLabel = 'Next', nextDisabled = false }: Props) {
  return (
    <div className="flex justify-between gap-3">
      {onBack ? (
        <button type="button" onClick={onBack} disabled={busy} className="rounded border border-gray-300 px-4 py-2">
          Back
        </button>
      ) : (
        <span />
      )}
      <button
        type="button"
        onClick={onNext}
        disabled={busy || nextDisabled}
        className="rounded bg-gray-900 px-4 py-2 font-medium text-white disabled:opacity-50"
      >
        {nextLabel}
      </button>
    </div>
  )
}
