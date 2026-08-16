import { useQuery } from '@tanstack/react-query'
import { api } from '../api/api'
import { queryKeys } from '../api/queryKeys'

export function useMe() {
  return useQuery({
    queryKey: queryKeys.me,
    queryFn: api.me,
    retry: false,
    staleTime: 60_000,
  })
}
