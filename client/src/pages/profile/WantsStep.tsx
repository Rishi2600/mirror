import { useState } from 'react'
import type { Preferences, WorkMode } from '../../api/types'
import { Field, inputClass } from '../../components/form'
import { toList, toText } from '../../lib/lists'
import { StepNav } from './StepNav'
import type { StepProps } from './types'

const workModes: WorkMode[] = ['any', 'remote', 'hybrid', 'onsite']

// Step 5: what the candidate is looking for, and who must not see the card.
export function WantsStep({ profile, busy, onBack, onNext }: StepProps) {
  const [prefs, setPrefs] = useState<Preferences>(profile.preferences)
  const [roleTypes, setRoleTypes] = useState(toText(profile.preferences.role_types))
  const [locations, setLocations] = useState(toText(profile.preferences.locations))
  const [hiddenFrom, setHiddenFrom] = useState(toText(profile.preferences.hidden_from))
  const [showPay, setShowPay] = useState(profile.card_settings.show_pay)

  const edit = (changes: Partial<Preferences>) => setPrefs({ ...prefs, ...changes })

  return (
    <div className="flex flex-col gap-4">
      <Field label="Role types (comma separated)">
        <input value={roleTypes} onChange={(e) => setRoleTypes(e.target.value)} placeholder="Backend, Platform" className={inputClass} />
      </Field>
      <Field label="Locations (comma separated)">
        <input value={locations} onChange={(e) => setLocations(e.target.value)} placeholder="Berlin, Remote EU" className={inputClass} />
      </Field>
      <Field label="Work mode">
        <select value={prefs.work_mode} onChange={(e) => edit({ work_mode: e.target.value as WorkMode })} className={inputClass}>
          {workModes.map((m) => (
            <option key={m} value={m}>
              {m}
            </option>
          ))}
        </select>
      </Field>
      <div className="grid grid-cols-2 gap-3">
        <Field label="Minimum yearly pay (0 = not set)">
          <input type="number" min={0} value={prefs.min_pay} onChange={(e) => edit({ min_pay: Number(e.target.value) })} className={inputClass} />
        </Field>
        <Field label="Currency">
          <input maxLength={3} value={prefs.pay_currency} onChange={(e) => edit({ pay_currency: e.target.value.toUpperCase() })} className={inputClass} />
        </Field>
      </div>
      <label className="flex items-center gap-2 text-sm text-gray-700">
        <input type="checkbox" checked={showPay} onChange={(e) => setShowPay(e.target.checked)} />
        Show my minimum pay on my card
      </label>
      <label className="flex items-center gap-2 text-sm text-gray-700">
        <input type="checkbox" checked={prefs.needs_visa} onChange={(e) => edit({ needs_visa: e.target.checked })} />
        I need visa sponsorship
      </label>
      <Field label="Hide my card from these companies (comma separated)">
        <input value={hiddenFrom} onChange={(e) => setHiddenFrom(e.target.value)} className={inputClass} />
      </Field>

      <StepNav
        busy={busy}
        onBack={onBack}
        nextLabel="Preview my card"
        onNext={() =>
          onNext({
            preferences: { ...prefs, role_types: toList(roleTypes), locations: toList(locations), hidden_from: toList(hiddenFrom) },
            card_settings: { show_pay: showPay },
          })
        }
      />
    </div>
  )
}
