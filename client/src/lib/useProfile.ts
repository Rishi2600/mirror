import { useQuery } from '@tanstack/react-query'
import { api } from './api'

export const profileQueryKey = ['profile']

// useProfile returns the candidate's profile (null before the first upload).
// While the CV is being read, it checks again every 2 seconds.
export function useProfile() {
  return useQuery({
    queryKey: profileQueryKey,
    queryFn: api.getProfile,
    retry: false,
    refetchInterval: (query) => (query.state.data?.parse_status === 'pending' ? 2000 : false),
  })
}
