import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect } from 'react'
import { api } from './api'

export const selectivityKey = ['selectivity']

// useLikes returns the candidate's likes today, and refreshes when a pause
// ends or the budget resets.
export function useLikes() {
  const queryClient = useQueryClient()
  const query = useQuery({ queryKey: selectivityKey, queryFn: api.getSelectivity })
  const likes = query.data?.likes
  const blockedUntil = likes?.paused_until ?? (likes?.left === 0 ? likes.resets_at : null)

  useEffect(() => {
    if (!blockedUntil) {
      return
    }
    const wait = new Date(blockedUntil).getTime() - Date.now() + 1000
    const timer = setTimeout(() => queryClient.invalidateQueries({ queryKey: selectivityKey }), wait)
    return () => clearTimeout(timer)
  }, [blockedUntil, queryClient])

  return likes
}
