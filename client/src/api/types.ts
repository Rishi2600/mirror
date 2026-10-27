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
