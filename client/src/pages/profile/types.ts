import type { CardSettings, CV, Preferences, Profile } from '../../api/types'

// ProfileChanges are the fields one step edits; the builder merges them
// into the profile and saves.
export type ProfileChanges = {
  cv?: Partial<CV>
  preferences?: Partial<Preferences>
  card_settings?: Partial<CardSettings>
}

export type StepProps = {
  profile: Profile
  busy: boolean
  onBack: () => void
  // Saves the changes (if any), then moves to the next step.
  onNext: (changes?: ProfileChanges) => void
}
