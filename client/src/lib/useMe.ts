import { useQuery } from '@tanstack/react-query'
import { api } from './api'

export const meQueryKey = ['me']

// useMe returns the signed-in user; data is null when nobody is signed in.
export function useMe() {
  return useQuery({ queryKey: meQueryKey, queryFn: api.getMe, retry: false })
}
