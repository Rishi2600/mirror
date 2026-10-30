import { useQueryClient } from '@tanstack/react-query'
import { useState, type FormEvent } from 'react'
import { Navigate } from 'react-router'
import { api } from '../lib/api'
import { meQueryKey, useMe } from '../lib/useMe'

const inputClass = 'w-full rounded border border-gray-300 px-3 py-2'
const buttonClass = 'w-full rounded bg-gray-900 px-3 py-2 font-medium text-white disabled:opacity-50'

// SignInPage signs in with an emailed code: first the email, then the code.
export function SignInPage() {
  const me = useMe()
  const queryClient = useQueryClient()
  const [step, setStep] = useState<'email' | 'code'>('email')
  const [email, setEmail] = useState('')
  const [code, setCode] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  // Signed in (already, or just now in checkCode): go on to the app.
  if (me.data) {
    return <Navigate to={me.data.role ? '/' : '/choose-role'} replace />
  }

  async function run(action: () => Promise<void>) {
    setBusy(true)
    setError('')
    try {
      await action()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Something went wrong; try again')
    } finally {
      setBusy(false)
    }
  }

  function sendCode(e?: FormEvent) {
    e?.preventDefault()
    run(async () => {
      await api.requestCode(email)
      setStep('code')
    })
  }

  function checkCode(e: FormEvent) {
    e.preventDefault()
    run(async () => {
      // The browser's time zone: the daily like budget resets at local midnight.
      const user = await api.verifyCode(email, code, Intl.DateTimeFormat().resolvedOptions().timeZone)
      queryClient.setQueryData(meQueryKey, user)
    })
  }

  return (
    <main className="mx-auto flex min-h-screen max-w-sm flex-col justify-center gap-6 p-6">
      <h1 className="text-3xl font-semibold">Sign in to Mirror</h1>

      {step === 'email' ? (
        <form onSubmit={sendCode} className="flex flex-col gap-3">
          <label htmlFor="email" className="text-sm text-gray-700">
            Email
          </label>
          <input
            id="email"
            type="email"
            autoComplete="email"
            required
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            className={inputClass}
          />
          <button type="submit" disabled={busy} className={buttonClass}>
            Send me a code
          </button>
        </form>
      ) : (
        <form onSubmit={checkCode} className="flex flex-col gap-3">
          <p className="text-sm text-gray-700">
            We sent a 6-digit code to <strong>{email}</strong>. It expires in 10 minutes.
          </p>
          <label htmlFor="code" className="text-sm text-gray-700">
            Code
          </label>
          <input
            id="code"
            inputMode="numeric"
            autoComplete="one-time-code"
            maxLength={6}
            required
            value={code}
            onChange={(e) => setCode(e.target.value)}
            className={inputClass}
          />
          <button type="submit" disabled={busy} className={buttonClass}>
            Sign in
          </button>
          <div className="flex justify-between text-sm">
            <button type="button" onClick={() => setStep('email')} className="text-blue-700 underline">
              Use a different email
            </button>
            <button type="button" onClick={() => sendCode()} disabled={busy} className="text-blue-700 underline">
              Send a new code
            </button>
          </div>
        </form>
      )}

      {error && (
        <p role="alert" className="rounded border border-red-300 bg-red-50 p-3 text-red-800">
          {error}
        </p>
      )}
    </main>
  )
}
