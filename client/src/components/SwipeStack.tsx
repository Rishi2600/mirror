import { animate, motion, useMotionValue, useReducedMotion, useTransform, type PanInfo } from 'motion/react'
import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react'
import { UNDO_MS, useSwipeQueue, type Direction } from '../lib/useSwipeQueue'
import { useVisibleTime } from '../lib/useVisibleTime'

// A swipe counts when the card is dragged this far, or flung this fast.
const DISTANCE = 120
const SPEED = 600

type Props<T> = {
  items: T[]
  getKey: (item: T) => string
  render: (item: T) => ReactNode
  // Called when a swipe leaves the undo window (it is final from then on).
  // dwellMs is how long the card was on screen. Resolving to false means the
  // swipe was refused, and the card comes back.
  onSend: (item: T, direction: Direction, dwellMs: number) => void | Promise<boolean>
  // False stops likes (skips still work).
  canLike?: boolean
  emptyMessage: string
}

// SwipeStack shows one card at a time, with the next two waiting behind it.
// Drag or fling the top card right to like it, left to skip it; the
// buttons and the arrow keys do the same.
export function SwipeStack<T>({ items, getKey, render, onSend, canLike = true, emptyMessage }: Props<T>) {
  const [swiped, setSwiped] = useState<Set<string>>(() => new Set())
  const [leaving, setLeaving] = useState<Direction | null>(null)
  const queue = useSwipeQueue<{ card: T; dwellMs: number }>(async (s) => {
    const accepted = await onSend(s.item.card, s.direction, s.item.dwellMs)
    if (accepted === false) {
      bringBack(s.item.card)
    }
  })

  const remaining = items.filter((item) => !swiped.has(getKey(item)))
  const top = remaining[0]
  const timeOnCard = useVisibleTime(top && getKey(top))

  function decide(direction: Direction) {
    if (!top) {
      return
    }
    setLeaving(null)
    setSwiped((prev) => new Set(prev).add(getKey(top)))
    queue.swipe({ card: top, dwellMs: timeOnCard() }, direction)
  }

  function bringBack(card: T) {
    setSwiped((prev) => {
      const next = new Set(prev)
      next.delete(getKey(card))
      return next
    })
  }

  function undo() {
    const swipe = queue.undo()
    if (swipe) {
      bringBack(swipe.item.card)
    }
  }

  // The arrow keys swipe; Backspace undoes.
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement) {
        return
      }
      if (e.key === 'ArrowRight' && canLike) setLeaving('right')
      if (e.key === 'ArrowLeft') setLeaving('left')
      if (e.key === 'Backspace') undo()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  })

  return (
    <div className="flex flex-col gap-4">
      <div className="relative h-[70vh] max-h-[680px] min-h-[420px]">
        {remaining.length === 0 && (
          <div className="flex h-full items-center justify-center rounded-lg border border-dashed border-gray-300 p-6 text-center text-gray-600">
            {emptyMessage}
          </div>
        )}
        {/* The next two cards wait behind; drawing them now also loads their images. */}
        {remaining
          .slice(1, 3)
          .reverse()
          .map((item, i, behind) => (
            <div
              key={getKey(item)}
              aria-hidden="true"
              className="absolute inset-0 overflow-hidden rounded-lg bg-white"
              style={{ transform: `translateY(${(behind.length - i) * 10}px) scale(${1 - (behind.length - i) * 0.04})` }}
            >
              {render(item)}
            </div>
          ))}
        {top && (
          <SwipeCard key={getKey(top)} leaving={leaving} canLike={canLike} onDecide={decide}>
            {render(top)}
          </SwipeCard>
        )}
      </div>

      <div className="flex items-center justify-center gap-4">
        <button
          type="button"
          onClick={() => setLeaving('left')}
          disabled={!top}
          aria-label="Skip"
          className="flex h-14 w-14 items-center justify-center rounded-full border border-gray-300 text-gray-700 disabled:opacity-40"
        >
          <svg viewBox="0 0 24 24" className="h-6 w-6" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round">
            <path d="M6 6l12 12M18 6L6 18" />
          </svg>
        </button>
        <button
          type="button"
          onClick={undo}
          disabled={!queue.waiting}
          aria-label="Undo"
          className="rounded-full border border-gray-300 px-4 py-2 text-sm disabled:opacity-40"
        >
          Undo
        </button>
        <button
          type="button"
          onClick={() => setLeaving('right')}
          disabled={!top || !canLike}
          aria-label="Like"
          className="flex h-14 w-14 items-center justify-center rounded-full bg-green-600 text-white disabled:opacity-40"
        >
          <svg viewBox="0 0 24 24" className="h-6 w-6" fill="currentColor">
            <path d="M12 21s-7.5-4.6-9.6-9.2C1 8.6 3 5 6.5 5c2 0 3.6 1.1 4.5 2.6.3.5.7.5 1 0C12.9 6.1 14.5 5 16.5 5 20 5 22 8.6 20.6 11.8 18.5 16.4 12 21 12 21z" />
          </svg>
        </button>
      </div>
      <p className="text-center text-xs text-gray-500">
        {queue.waiting
          ? `${queue.waiting.direction === 'right' ? 'Liked' : 'Skipped'}. You can undo for ${UNDO_MS / 1000} seconds.`
          : 'Swipe right to like, left to skip. Arrow keys work too.'}
      </p>
    </div>
  )
}

type CardProps = {
  children: ReactNode
  // Set by the buttons and keys: the card flies out that way.
  leaving: Direction | null
  canLike: boolean
  onDecide: (direction: Direction) => void
}

// SwipeCard is the draggable top card. It tilts and shows LIKE or SKIP as
// it moves, flies out past the edge on a swipe, or springs back.
function SwipeCard({ children, leaving, canLike, onDecide }: CardProps) {
  const x = useMotionValue(0)
  const rotate = useTransform(x, [-300, 300], [-12, 12])
  const likeOpacity = useTransform(x, [30, DISTANCE], [0, 1])
  const skipOpacity = useTransform(x, [-DISTANCE, -30], [1, 0])
  const reduceMotion = useReducedMotion()
  // The latest onDecide, so flyOut stays the same function between renders.
  const onDecideRef = useRef(onDecide)
  useEffect(() => {
    onDecideRef.current = onDecide
  })

  const flyOut = useCallback(
    (direction: Direction) => {
      const edge = (window.innerWidth + 200) * (direction === 'right' ? 1 : -1)
      animate(x, edge, { duration: reduceMotion ? 0 : 0.25, ease: 'easeIn' }).then(() => onDecideRef.current(direction))
    },
    [x, reduceMotion],
  )

  useEffect(() => {
    if (leaving) {
      flyOut(leaving)
    }
  }, [leaving, flyOut])

  function onDragEnd(_: PointerEvent | MouseEvent | TouchEvent, info: PanInfo) {
    if (canLike && (info.offset.x > DISTANCE || info.velocity.x > SPEED)) {
      flyOut('right')
    } else if (info.offset.x < -DISTANCE || info.velocity.x < -SPEED) {
      flyOut('left')
    } else {
      animate(x, 0, { type: 'spring', stiffness: 400, damping: 30 })
    }
  }

  return (
    <motion.div
      data-testid="swipe-card"
      drag="x"
      dragMomentum={false}
      onDragEnd={onDragEnd}
      style={{ x, rotate, touchAction: 'pan-y' }}
      className="absolute inset-0 cursor-grab overflow-y-auto rounded-lg bg-white active:cursor-grabbing"
    >
      <motion.span
        style={{ opacity: likeOpacity }}
        className="pointer-events-none absolute left-4 top-4 z-10 rotate-[-12deg] rounded border-4 border-green-600 px-2 text-2xl font-bold text-green-600"
      >
        LIKE
      </motion.span>
      <motion.span
        style={{ opacity: skipOpacity }}
        className="pointer-events-none absolute right-4 top-4 z-10 rotate-12 rounded border-4 border-red-600 px-2 text-2xl font-bold text-red-600"
      >
        SKIP
      </motion.span>
      {children}
    </motion.div>
  )
}
