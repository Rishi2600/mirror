import { useRef, useState, type KeyboardEvent } from 'react'
import { formatSize } from '../../lib/format'

const maxBody = 4000
const maxFileBytes = 10 * 1024 * 1024
const typingEveryMs = 2000

type Props = {
  onSend: (body: string, file?: File) => void
  onTyping: () => void
}

// Composer is the message box: Enter sends, Shift+Enter starts a new line.
// One file can go with each message.
export function Composer({ onSend, onTyping }: Props) {
  const [text, setText] = useState('')
  const [file, setFile] = useState<File | undefined>()
  const [error, setError] = useState('')
  const fileInput = useRef<HTMLInputElement>(null)
  const lastTyping = useRef(0)

  const tooLong = text.length > maxBody
  const canSend = (text.trim() !== '' || file !== undefined) && !tooLong

  function send() {
    if (!canSend) {
      return
    }
    onSend(text.trim(), file)
    setText('')
    setFile(undefined)
  }

  function onKeyDown(e: KeyboardEvent<HTMLTextAreaElement>) {
    if (e.key === 'Enter' && !e.shiftKey && !e.nativeEvent.isComposing) {
      e.preventDefault()
      send()
    }
  }

  function onChange(value: string) {
    setText(value)
    if (Date.now() - lastTyping.current > typingEveryMs) {
      lastTyping.current = Date.now()
      onTyping()
    }
  }

  function pick(picked: File | undefined) {
    setError('')
    if (picked && picked.size > maxFileBytes) {
      setError('Files must be 10 MB or smaller.')
      picked = undefined
    }
    setFile(picked)
    if (fileInput.current) {
      fileInput.current.value = '' // so picking the same file again still fires
    }
  }

  return (
    <div className="flex flex-col gap-2 border-t border-gray-200 pt-3">
      {file && (
        <div className="flex items-center justify-between rounded bg-gray-100 px-3 py-2 text-sm">
          <span className="break-all">
            {file.name} ({formatSize(file.size)})
          </span>
          <button type="button" onClick={() => setFile(undefined)} className="underline">
            Remove
          </button>
        </div>
      )}
      {error && (
        <p role="alert" className="text-sm text-red-700">
          {error}
        </p>
      )}
      <div className="flex items-end gap-2">
        <button
          type="button"
          onClick={() => fileInput.current?.click()}
          className="rounded border border-gray-300 px-3 py-2 text-sm"
        >
          Attach
        </button>
        <input
          ref={fileInput}
          type="file"
          accept="image/jpeg,image/png,image/webp,application/pdf,.docx"
          className="hidden"
          aria-label="Attach a file"
          onChange={(e) => pick(e.target.files?.[0])}
        />
        <textarea
          value={text}
          onChange={(e) => onChange(e.target.value)}
          onKeyDown={onKeyDown}
          rows={2}
          placeholder="Write a message"
          aria-label="Message"
          className="min-w-0 flex-1 resize-none rounded border border-gray-300 px-3 py-2"
        />
        <button
          type="button"
          onClick={send}
          disabled={!canSend}
          className="rounded bg-blue-600 px-4 py-2 font-medium text-white disabled:opacity-50"
        >
          Send
        </button>
      </div>
      {text.length > maxBody - 500 && (
        <p className={`text-right text-xs ${tooLong ? 'text-red-700' : 'text-gray-500'}`}>
          {text.length} / {maxBody}
        </p>
      )}
    </div>
  )
}
