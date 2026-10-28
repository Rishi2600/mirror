import type { ReactNode } from 'react'

export const inputClass = 'w-full rounded border border-gray-300 px-3 py-2'

// Field is a label above its input.
export function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <label className="flex flex-col gap-1 text-sm text-gray-700">
      {label}
      {children}
    </label>
  )
}

export function ErrorMessage({ message }: { message: string }) {
  if (!message) {
    return null
  }
  return (
    <p role="alert" className="rounded border border-red-300 bg-red-50 p-3 text-red-800">
      {message}
    </p>
  )
}
