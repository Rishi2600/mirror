// Response types of the server's API, written to match the Go handlers.

export interface Health {
  status: 'ok'
  version: string
  // RFC 3339 time
  time: string
}
