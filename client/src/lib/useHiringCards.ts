import { useQuery } from '@tanstack/react-query'
import { api } from './api'

export const hiringCardsQueryKey = ['hiring-cards']
export const hiringCardQueryKey = (id: string) => ['hiring-cards', id]

// useHiringCards returns the employer's hiring cards, newest first.
export function useHiringCards() {
  return useQuery({ queryKey: hiringCardsQueryKey, queryFn: api.listHiringCards })
}

export function useHiringCard(id: string) {
  return useQuery({ queryKey: hiringCardQueryKey(id), queryFn: () => api.getHiringCard(id), retry: false })
}
