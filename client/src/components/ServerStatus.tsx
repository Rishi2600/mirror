import type { Health } from '../api/types'

type Props = {
  health?: Health
  isLoading: boolean
  isError: boolean
}

// ServerStatus shows the API's health: its status and version, or a clear
// message when the API cannot be reached.
export function ServerStatus({ health, isLoading, isError }: Props) {
  if (isLoading) {
    return <p className="text-gray-600">Checking the server...</p>
  }
  if (isError || !health) {
    return (
      <p role="alert" className="rounded border border-red-300 bg-red-50 p-3 text-red-800">
        The API is unreachable. Start the server and reload this page.
      </p>
    )
  }
  return (
    <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1 rounded border border-gray-200 p-3">
      <dt className="text-gray-600">Status</dt>
      <dd data-testid="server-status" className="font-medium text-green-700">
        {health.status}
      </dd>
      <dt className="text-gray-600">Version</dt>
      <dd data-testid="server-version" className="font-medium">
        {health.version}
      </dd>
    </dl>
  )
}
