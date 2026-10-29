import { useState } from 'react'
import type { ChatFile, ChatMessage } from '../../api/types'
import { formatSize, formatTime } from '../../lib/format'

type Props = {
  message: ChatMessage
  mine: boolean
  seen: boolean
  onDelete: () => void
  // Called when an image has loaded and made the message taller.
  onImageLoad: () => void
}

// MessageBubble is one message: text, an image, or a document card. Your
// own messages can be deleted, after a confirm.
export function MessageBubble({ message, mine, seen, onDelete, onImageLoad }: Props) {
  const [confirming, setConfirming] = useState(false)
  const side = mine ? 'items-end' : 'items-start'
  const colors = mine ? 'bg-blue-600 text-white' : 'bg-gray-100 text-gray-900'

  if (message.deleted) {
    return (
      <div className={`flex flex-col ${side}`}>
        <p className="rounded-2xl border border-gray-200 px-3 py-2 text-sm text-gray-500 italic">Message deleted</p>
      </div>
    )
  }
  return (
    <div data-testid="message" className={`flex flex-col gap-1 ${side}`}>
      <div className={`flex max-w-[80%] flex-col gap-2 rounded-2xl px-3 py-2 ${colors}`}>
        {message.file && <Attachment file={message.file} mine={mine} onImageLoad={onImageLoad} />}
        {message.body && <p className="break-words whitespace-pre-wrap">{message.body}</p>}
      </div>
      <div className="flex gap-2 text-xs text-gray-500">
        <span>{formatTime(message.created_at)}</span>
        {seen && <span>Seen</span>}
        {mine && !confirming && (
          <button type="button" onClick={() => setConfirming(true)} className="underline">
            Delete
          </button>
        )}
        {confirming && (
          <>
            <span>Delete for both of you?</span>
            <button type="button" onClick={onDelete} className="font-medium text-red-700 underline">
              Yes
            </button>
            <button type="button" onClick={() => setConfirming(false)} className="underline">
              No
            </button>
          </>
        )}
      </div>
    </div>
  )
}

function Attachment({ file, mine, onImageLoad }: { file: ChatFile; mine: boolean; onImageLoad: () => void }) {
  if (file.is_image) {
    return (
      <a href={file.url} target="_blank" rel="noreferrer">
        <img src={file.url} alt={file.name} loading="lazy" onLoad={onImageLoad} className="max-h-64 rounded-lg" />
      </a>
    )
  }
  return (
    <a
      href={file.url}
      className={`flex items-center gap-3 rounded-lg p-2 ${mine ? 'bg-blue-700' : 'bg-white'}`}
      aria-label={`Download ${file.name}`}
    >
      <span className="rounded bg-gray-800 px-2 py-1 text-xs font-semibold text-white">
        {file.content_type === 'application/pdf' ? 'PDF' : 'DOCX'}
      </span>
      <span className="flex flex-col">
        <span className="font-medium break-all">{file.name}</span>
        <span className="text-xs opacity-75">{formatSize(file.size)}</span>
      </span>
    </a>
  )
}
