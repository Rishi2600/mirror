// Response types of the server's API, written to match the Go handlers.

export interface Health {
  status: 'ok'
  version: string
  // RFC 3339 time
  time: string
}

export type Role = 'candidate' | 'employer' | 'admin'

// Matches UserResponse in server/internal/httpserver/auth_handlers.go.
export interface User {
  id: string
  email: string
  // Empty until the user picks candidate or employer.
  role: Role | ''
}

// Matches profiles.CV in server/internal/profiles/profiles.go.
export interface CV {
  full_name: string
  email: string
  phone: string
  headline: string
  summary: string
  current_title: string
  current_employer: string
  location: string
  years_experience: number
  seniority: Seniority | ''
  skills: string[]
  experience: Experience[]
  education: Education[]
}

export type Seniority = 'intern' | 'junior' | 'mid' | 'senior' | 'lead' | 'principal'

export interface Experience {
  title: string
  company: string
  start: string
  end: string
  summary: string
}

export interface Education {
  degree: string
  institution: string
  year: string
}

export type WorkMode = 'onsite' | 'hybrid' | 'remote' | 'any'

export interface Preferences {
  role_types: string[]
  locations: string[]
  work_mode: WorkMode
  // Minimum yearly pay in pay_currency; 0 means not set.
  min_pay: number
  pay_currency: string
  needs_visa: boolean
  // Companies that must never see the card.
  hidden_from: string[]
}

// Matches profiles.CardSettings: what the card shows.
export interface CardSettings {
  employer_display: 'name' | 'description' | 'hidden'
  employer_description: string
  show_pay: boolean
}

// Matches ProfileResponse in server/internal/httpserver/profile_handlers.go.
export interface Profile {
  // pending: the CV is being read.
  parse_status: 'pending' | 'done' | 'failed'
  parse_error: string
  cv: CV
  preferences: Preferences
  card_settings: CardSettings
  // Empty until a photo is uploaded.
  photo_url: string
  // True once the card is finished (and live for employers).
  onboarded: boolean
  updated_at: string
}

// Matches profiles.Card: what employers see before a match.
export interface Card {
  name: string
  photo_url: string
  headline: string
  location: string
  current_title: string
  current_employer: string
  years_experience: number
  seniority: Seniority | ''
  skills: string[]
  summary: string
  experience: Experience[]
  education: Education[]
  work_mode: WorkMode
  role_types: string[]
  // 0 when the candidate hides it.
  min_pay: number
  pay_currency: string
}

// Matches CardResponse in server/internal/httpserver/card_handlers.go.
export interface CardPreview {
  card: Card
  // Contact details taken out of the card's text.
  removed: { emails: number; phones: number; links: number }
  // What the card still needs before it can be finished.
  missing: string[]
  onboarded: boolean
}
