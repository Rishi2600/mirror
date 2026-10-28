import { useState } from 'react'
import type { HiringFields } from '../../api/types'
import { Field, inputClass } from '../../components/form'
import { StepNav } from '../profile/StepNav'
import { fieldsOf, type HiringStepProps } from './types'

// Step 3: where the job is, and what it pays.
export function WherePayStep({ card, busy, onBack, onNext }: HiringStepProps) {
  const [fields, setFields] = useState<HiringFields>(fieldsOf(card))
  const edit = (changes: Partial<HiringFields>) => setFields({ ...fields, ...changes })

  return (
    <div className="flex flex-col gap-4">
      <Field label="Location">
        <input value={fields.location} onChange={(e) => edit({ location: e.target.value })} placeholder="Berlin" className={inputClass} />
      </Field>
      <Field label="Work mode">
        <select
          value={fields.work_mode}
          onChange={(e) => edit({ work_mode: e.target.value as HiringFields['work_mode'] })}
          className={inputClass}
        >
          <option value="">Choose one</option>
          <option value="remote">remote</option>
          <option value="hybrid">hybrid</option>
          <option value="onsite">onsite</option>
        </select>
      </Field>
      <div className="grid grid-cols-3 gap-3">
        <Field label="Pay from (yearly)">
          <input type="number" min={0} value={fields.pay_min} onChange={(e) => edit({ pay_min: Number(e.target.value) })} className={inputClass} />
        </Field>
        <Field label="Pay up to">
          <input type="number" min={0} value={fields.pay_max} onChange={(e) => edit({ pay_max: Number(e.target.value) })} className={inputClass} />
        </Field>
        <Field label="Currency">
          <input maxLength={3} value={fields.pay_currency} onChange={(e) => edit({ pay_currency: e.target.value.toUpperCase() })} className={inputClass} />
        </Field>
      </div>
      <p className="text-xs text-gray-500">Leave pay at 0 if you prefer not to show it.</p>
      <label className="flex items-center gap-2 text-sm text-gray-700">
        <input type="checkbox" checked={fields.visa_sponsorship} onChange={(e) => edit({ visa_sponsorship: e.target.checked })} />
        We sponsor visas for this role
      </label>
      <StepNav busy={busy} onBack={onBack} onNext={() => onNext(fields)} />
    </div>
  )
}
