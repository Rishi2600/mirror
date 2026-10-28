import { useState } from 'react'
import type { Seniority } from '../../api/types'
import { Field, inputClass } from '../../components/form'
import { toList, toText } from '../../lib/lists'
import { StepNav } from './StepNav'
import type { StepProps } from './types'

const seniorities: Seniority[] = ['intern', 'junior', 'mid', 'senior', 'lead', 'principal']

// Step 4: experience level, skills and summary.
export function SkillsStep({ profile, busy, onBack, onNext }: StepProps) {
  const [years, setYears] = useState(profile.cv.years_experience)
  const [seniority, setSeniority] = useState(profile.cv.seniority)
  const [skills, setSkills] = useState(toText(profile.cv.skills))
  const [summary, setSummary] = useState(profile.cv.summary)

  return (
    <div className="flex flex-col gap-4">
      <div className="grid grid-cols-2 gap-3">
        <Field label="Years of experience">
          <input type="number" min={0} value={years} onChange={(e) => setYears(Number(e.target.value))} className={inputClass} />
        </Field>
        <Field label="Seniority">
          <select value={seniority} onChange={(e) => setSeniority(e.target.value as Seniority | '')} className={inputClass}>
            <option value="">Not sure</option>
            {seniorities.map((s) => (
              <option key={s} value={s}>
                {s}
              </option>
            ))}
          </select>
        </Field>
      </div>
      <Field label="Skills (comma separated)">
        <input value={skills} onChange={(e) => setSkills(e.target.value)} className={inputClass} />
      </Field>
      <Field label="Summary">
        <textarea rows={5} value={summary} onChange={(e) => setSummary(e.target.value)} className={inputClass} />
      </Field>
      <p className="text-xs text-gray-500">Emails, phone numbers and links are removed from your card until you match.</p>

      <StepNav
        busy={busy}
        onBack={onBack}
        onNext={() => onNext({ cv: { years_experience: years, seniority, skills: toList(skills), summary } })}
      />
    </div>
  )
}
