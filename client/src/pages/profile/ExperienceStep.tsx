import { useState } from 'react'
import type { CardSettings, Experience } from '../../api/types'
import { Field, inputClass } from '../../components/form'
import { StepNav } from './StepNav'
import type { StepProps } from './types'

const emptyJob: Experience = { title: '', company: '', start: '', end: '', summary: '' }

// Step 3: current role, past jobs, and how the current employer is shown.
export function ExperienceStep({ profile, busy, onBack, onNext }: StepProps) {
  const [currentTitle, setCurrentTitle] = useState(profile.cv.current_title)
  const [currentEmployer, setCurrentEmployer] = useState(profile.cv.current_employer)
  const [jobs, setJobs] = useState<Experience[]>(profile.cv.experience)
  const [settings, setSettings] = useState<CardSettings>(profile.card_settings)

  const editJob = (i: number, changes: Partial<Experience>) =>
    setJobs(jobs.map((job, j) => (j === i ? { ...job, ...changes } : job)))

  return (
    <div className="flex flex-col gap-4">
      <div className="grid grid-cols-2 gap-3">
        <Field label="Current title">
          <input value={currentTitle} onChange={(e) => setCurrentTitle(e.target.value)} className={inputClass} />
        </Field>
        <Field label="Current employer">
          <input value={currentEmployer} onChange={(e) => setCurrentEmployer(e.target.value)} className={inputClass} />
        </Field>
      </div>

      <fieldset className="flex flex-col gap-2 rounded border border-gray-200 p-3 text-sm">
        <legend className="px-1 font-medium">Show my current employer as</legend>
        <label className="flex items-center gap-2">
          <input
            type="radio"
            checked={settings.employer_display === 'name'}
            onChange={() => setSettings({ ...settings, employer_display: 'name' })}
          />
          Its name{currentEmployer && `: ${currentEmployer}`}
        </label>
        <label className="flex items-center gap-2">
          <input
            type="radio"
            checked={settings.employer_display === 'description'}
            onChange={() => setSettings({ ...settings, employer_display: 'description' })}
          />
          A description
        </label>
        {settings.employer_display === 'description' && (
          <input
            aria-label="Employer description"
            value={settings.employer_description}
            onChange={(e) => setSettings({ ...settings, employer_description: e.target.value })}
            placeholder="a payments company"
            className={inputClass}
          />
        )}
        <label className="flex items-center gap-2">
          <input
            type="radio"
            checked={settings.employer_display === 'hidden'}
            onChange={() => setSettings({ ...settings, employer_display: 'hidden' })}
          />
          Don't show it
        </label>
        {settings.employer_display !== 'name' && (
          <p className="text-xs text-gray-500">Check that your summary and job descriptions don't name it either.</p>
        )}
      </fieldset>

      <section className="flex flex-col gap-3">
        <h3 className="font-medium">Jobs</h3>
        {jobs.map((job, i) => (
          <div key={i} className="flex flex-col gap-2 rounded border border-gray-200 p-3">
            <div className="grid grid-cols-2 gap-2">
              <input aria-label="Job title" value={job.title} onChange={(e) => editJob(i, { title: e.target.value })} placeholder="Title" className={inputClass} />
              <input aria-label="Company" value={job.company} onChange={(e) => editJob(i, { company: e.target.value })} placeholder="Company" className={inputClass} />
              <input aria-label="Start" value={job.start} onChange={(e) => editJob(i, { start: e.target.value })} placeholder="Start" className={inputClass} />
              <input aria-label="End" value={job.end} onChange={(e) => editJob(i, { end: e.target.value })} placeholder="End (or present)" className={inputClass} />
            </div>
            <textarea
              aria-label="What you did"
              rows={2}
              value={job.summary}
              onChange={(e) => editJob(i, { summary: e.target.value })}
              placeholder="What you did"
              className={inputClass}
            />
            <button type="button" onClick={() => setJobs(jobs.filter((_, j) => j !== i))} className="self-end text-sm text-red-700 underline">
              Remove this job
            </button>
          </div>
        ))}
        <button type="button" onClick={() => setJobs([...jobs, emptyJob])} className="self-start text-sm text-blue-700 underline">
          Add a job
        </button>
      </section>

      <StepNav
        busy={busy}
        onBack={onBack}
        onNext={() =>
          onNext({
            cv: { current_title: currentTitle, current_employer: currentEmployer, experience: jobs },
            card_settings: settings,
          })
        }
      />
    </div>
  )
}
