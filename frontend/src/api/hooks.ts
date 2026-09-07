import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import * as api from './endpoints'
import type { SetAvailabilityParams, SetLeaveDayParams } from './endpoints'
import type { LockedAssignment } from './types'

export function useEmployees() {
  return useQuery({ queryKey: ['employees'], queryFn: api.fetchEmployees })
}

export function useLatestSchedule() {
  return useQuery({ queryKey: ['schedule', 'latest'], queryFn: api.fetchLatestSchedule })
}

export function useSolve() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: api.solveSchedule,
    onSuccess: (result) => {
      queryClient.setQueryData(['schedule', 'latest'], { found: true, result })
    },
  })
}

export function useApprove() {
  return useMutation({
    mutationFn: (assignments: LockedAssignment[]) => api.approveAssignments(assignments),
  })
}

export function useCapacityCheck() {
  return useMutation({ mutationFn: api.checkCapacity })
}

export function useCandidates() {
  return useMutation({ mutationFn: api.fetchCandidates })
}

export function useSetAvailability() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ employeeId, params }: { employeeId: string; params: SetAvailabilityParams }) =>
      api.setAvailability(employeeId, params),
    // onSettled, not onSuccess: if this call fails after its sibling (leave
    // vs. availability) already succeeded, the day is left in a genuinely
    // partial server state — the grid must still refetch to show that,
    // rather than keep displaying the pre-click status.
    onSettled: () => queryClient.invalidateQueries({ queryKey: ['employees'] }),
  })
}

export function useSetLeaveDay() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ employeeId, params }: { employeeId: string; params: SetLeaveDayParams }) =>
      api.setLeaveDay(employeeId, params),
    // onSettled, not onSuccess: if this call fails after its sibling (leave
    // vs. availability) already succeeded, the day is left in a genuinely
    // partial server state — the grid must still refetch to show that,
    // rather than keep displaying the pre-click status.
    onSettled: () => queryClient.invalidateQueries({ queryKey: ['employees'] }),
  })
}
