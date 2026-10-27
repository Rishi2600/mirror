import { useQueryClient } from '@tanstack/react-query'
import { useState, type FormEvent, type ReactNode } from 'react'
import type { CV, Preferences, Profile, Seniority, WorkMode } from '../api/types'
import { api } from '../lib/api'
import { profileQueryKey } from '../lib/useProfile'

const seniorities: Seniority[] = ['intern', 'junior', 'mid', 'senior', 'lead', 'principal']
const workModes: WorkMode[] = ['any', 'remote', 'hybrid', 'onsite']
const inputClass = 'w-full rounded border border-gray-300 px-3 py-2'

// Lists are edited as comma-separated text.
const toText = (items: string[]) => items.join(', ')
const toList = (text: string) => text.split(',').map((s) => s.trim()).filter(Boolean)

function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <label className="flex flex-col gap-1 text-sm text-gray-700">
      {label}
      {children}
    </label>
  )
}

// ProfileForm edits what the AI read from the CV, and the job preferences.
export function ProfileForm({ profile }: { profile: Profile }) {
  const queryClient = useQueryClient()
  const [cv, setCV] = useState<CV>(profile.cv)
  const [prefs, setPrefs] = useState<Preferences>(profile.preferences)
  const [skills, setSkills] = useState(toText(profile.cv.skills))
  const [roleTypes, setRoleTypes] = useState(toText(profile.preferences.role_types))
  const [locations, setLocations] = useState(toText(profile.preferences.locations))
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const editCV = (changes: Partial<CV>) => setCV({ ...cv, ...changes })
  const editPrefs = (changes: Partial<Preferences>) => setPrefs({ ...prefs, ...changes })

  async function save(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    setMessage('')
    setError('')
    try {
      const saved = await api.updateProfile(
        { ...cv, skills: toList(skills) },
        { ...prefs, role_types: toList(roleTypes), locations: toList(locations) },
      )
      queryClient.setQueryData(profileQueryKey, saved)
      setMessage('Saved')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Something went wrong; try again')
    } finally {
      setBusy(false)
    }
  }

  return (
    <form onSubmit={save} className="flex flex-col gap-6">
      <section className="flex flex-col gap-3">
        <h2 className="text-lg font-semibold">From your CV</h2>
        <p className="text-sm text-gray-600">We read this from your CV. Fix anything that is wrong.</p>
        <Field label="Full name">
          <input value={cv.full_name} onChange={(e) => editCV({ full_name: e.target.value })} className={inputClass} />
        </Field>
        <Field label="Headline">
          <input value={cv.headline} onChange={(e) => editCV({ headline: e.target.value })} className={inputClass} />
        </Field>
        <div className="grid grid-cols-2 gap-3">
          <Field label="Current title">
            <input value={cv.current_title} onChange={(e) => editCV({ current_title: e.target.value })} className={inputClass} />
          </Field>
          <Field label="Current employer">
            <input value={cv.current_employer} onChange={(e) => editCV({ current_employer: e.target.value })} className={inputClass} />
          </Field>
          <Field label="Location">
            <input value={cv.location} onChange={(e) => editCV({ location: e.target.value })} className={inputClass} />
          </Field>
          <Field label="Years of experience">
            <input
              type="number"
              min={0}
              value={cv.years_experience}
              onChange={(e) => editCV({ years_experience: Number(e.target.value) })}
              className={inputClass}
            />
          </Field>
        </div>
        <Field label="Seniority">
          <select value={cv.seniority} onChange={(e) => editCV({ seniority: e.target.value as Seniority | '' })} className={inputClass}>
            <option value="">Not sure</option>
            {seniorities.map((s) => (
              <option key={s} value={s}>
                {s}
              </option>
            ))}
          </select>
        </Field>
        <Field label="Skills (comma separated)">
          <input value={skills} onChange={(e) => setSkills(e.target.value)} className={inputClass} />
        </Field>
        <Field label="Summary">
          <textarea rows={4} value={cv.summary} onChange={(e) => editCV({ summary: e.target.value })} className={inputClass} />
        </Field>

        {cv.experience.length > 0 && (
          <div className="text-sm">
            <h3 className="font-medium">Experience</h3>
            <ul className="mt-1 flex flex-col gap-2">
              {cv.experience.map((job, i) => (
                <li key={i}>
                  <span className="font-medium">{job.title}</span>, {job.company} ({job.start} - {job.end})
                  {job.summary && <p className="text-gray-600">{job.summary}</p>}
                </li>
              ))}
            </ul>
          </div>
        )}
        {cv.education.length > 0 && (
          <div className="text-sm">
            <h3 className="font-medium">Education</h3>
            <ul className="mt-1">
              {cv.education.map((ed, i) => (
                <li key={i}>
                  {ed.degree}, {ed.institution} {ed.year && `(${ed.year})`}
                </li>
              ))}
            </ul>
          </div>
        )}
      </section>

      <section className="flex flex-col gap-3">
        <h2 className="text-lg font-semibold">What you are looking for</h2>
        <Field label="Role types (comma separated)">
          <input value={roleTypes} onChange={(e) => setRoleTypes(e.target.value)} placeholder="Backend, Platform" className={inputClass} />
        </Field>
        <Field label="Locations (comma separated)">
          <input value={locations} onChange={(e) => setLocations(e.target.value)} placeholder="Berlin, Remote EU" className={inputClass} />
        </Field>
        <Field label="Work mode">
          <select value={prefs.work_mode} onChange={(e) => editPrefs({ work_mode: e.target.value as WorkMode })} className={inputClass}>
            {workModes.map((m) => (
              <option key={m} value={m}>
                {m}
              </option>
            ))}
          </select>
        </Field>
        <div className="grid grid-cols-2 gap-3">
          <Field label="Minimum yearly pay (0 = not set)">
            <input
              type="number"
              min={0}
              value={prefs.min_pay}
              onChange={(e) => editPrefs({ min_pay: Number(e.target.value) })}
              className={inputClass}
            />
          </Field>
          <Field label="Currency">
            <input
              maxLength={3}
              value={prefs.pay_currency}
              onChange={(e) => editPrefs({ pay_currency: e.target.value.toUpperCase() })}
              className={inputClass}
            />
          </Field>
        </div>
        <label className="flex items-center gap-2 text-sm text-gray-700">
          <input type="checkbox" checked={prefs.needs_visa} onChange={(e) => editPrefs({ needs_visa: e.target.checked })} />
          I need visa sponsorship
        </label>
      </section>

      <button type="submit" disabled={busy} className="rounded bg-gray-900 px-3 py-2 font-medium text-white disabled:opacity-50">
        Save profile
      </button>
      {message && <p className="text-sm text-green-700">{message}</p>}
      {error && (
        <p role="alert" className="rounded border border-red-300 bg-red-50 p-3 text-red-800">
          {error}
        </p>
      )}
    </form>
  )
}
