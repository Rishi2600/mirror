import type { HiringCard, HiringFields } from '../../api/types'

export type HiringStepProps = {
  card: HiringCard
  busy: boolean
  onBack: () => void
  // Saves the changes (if any), then moves to the next step.
  onNext: (changes?: Partial<HiringFields>) => void
}

// fieldsOf picks the editable fields out of a card.
export function fieldsOf(card: HiringCard): HiringFields {
  return {
    title: card.title,
    company_name: card.company_name,
    location: card.location,
    work_mode: card.work_mode,
    seniority: card.seniority,
    skills: card.skills,
    summary: card.summary,
    pay_min: card.pay_min,
    pay_max: card.pay_max,
    pay_currency: card.pay_currency,
    visa_sponsorship: card.visa_sponsorship,
  }
}
