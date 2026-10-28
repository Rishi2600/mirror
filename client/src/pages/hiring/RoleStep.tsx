import { useState } from 'react'
import type { Seniority } from '../../api/types'
import { Field, inputClass } from '../../components/form'
import { toList, toText } from '../../lib/lists'
import { StepNav } from '../profile/StepNav'
import type { HiringStepProps } from './types'

const seniorities: Seniority[] = ['intern', 'junior', 'mid', 'senior', 'lead', 'principal']

// Step 2: what the role is.
export function RoleStep({ card, busy, onBack, onNext }: HiringStepProps) {
  const [title, setTitle] = useState(card.title)
  const [companyName, setCompanyName] = useState(card.company_name)
  const [seniority, setSeniority] = useState(card.seniority)
  const [skills, setSkills] = useState(toText(card.skills))
  const [summary, setSummary] = useState(card.summary)

  return (
    <div className="flex flex-col gap-4">
      <Field label="Job title">
        <input value={title} onChange={(e) => setTitle(e.target.value)} className={inputClass} />
      </Field>
      <Field label="Company (optional)">
        <input value={companyName} onChange={(e) => setCompanyName(e.target.value)} className={inputClass} />
      </Field>
      <Field label="Seniority">
        <select value={seniority} onChange={(e) => setSeniority(e.target.value as Seniority | '')} className={inputClass}>
          <option value="">Not set</option>
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
      <Field label="Short description">
        <textarea rows={4} value={summary} onChange={(e) => setSummary(e.target.value)} className={inputClass} />
      </Field>
      <p className="text-xs text-gray-500">Emails, phone numbers and links are removed from the card until you match.</p>
      <StepNav
        busy={busy}
        onBack={onBack}
        onNext={() => onNext({ title, company_name: companyName, seniority, skills: toList(skills), summary })}
      />
    </div>
  )
}
