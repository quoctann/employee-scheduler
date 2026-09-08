import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import * as api from './endpoints'
import type { SetAvailabilityParams, SetLeaveDayParams, UnapproveParams, UpdateGateShiftRequirementParams } from './endpoints'
import type { LockedAssignment, ShiftType, SolverConfig } from './types'
import type { CreateEmployeeParams, UpdateEmployeeParams } from './endpoints'

// `from`/`to` (YYYY-MM-DD) scope the availability window to whatever the
// caller is actually displaying — without them the backend defaults to
// [today, today+27], so a registration view browsing a week that starts
// before today (e.g. the current week when today isn't Monday) or a past
// week would write successfully but never see the write reflected back.
export function useEmployees(from?: string, to?: string) {
  return useQuery({
    queryKey: ['employees', from ?? null, to ?? null],
    queryFn: () => api.fetchEmployees(false, from, to),
  })
}

export function useAllEmployees() {
  return useQuery({ queryKey: ['employees', 'all'], queryFn: () => api.fetchEmployees(true) })
}

export function useConfig() {
  return useQuery({ queryKey: ['config'], queryFn: api.fetchConfig })
}

export function useUpdateGateShiftRequirement() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ gateCode, shiftType, params }: { gateCode: string; shiftType: ShiftType; params: UpdateGateShiftRequirementParams }) =>
      api.updateGateShiftRequirement(gateCode, shiftType, params),
    onSuccess: (config: SolverConfig) => {
      queryClient.setQueryData(['config'], config)
    },
  })
}

export function useRenameGate() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ gateCode, newCode }: { gateCode: string; newCode: string }) => api.renameGate(gateCode, newCode),
    onSuccess: (config: SolverConfig) => {
      queryClient.setQueryData(['config'], config)
    },
  })
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
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (assignments: LockedAssignment[]) => api.approveAssignments(assignments),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['schedule', 'approved'] }),
  })
}

export function useUnapprove() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (params: UnapproveParams) => api.unapproveAssignments(params),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['schedule', 'approved'] }),
  })
}

// startDate/numDays come from the currently displayed solve result, so this
// naturally stays disabled (no query) until a result exists.
export function useApprovedAssignments(startDate: string, numDays: number) {
  return useQuery({
    queryKey: ['schedule', 'approved', startDate, numDays],
    queryFn: () => api.fetchApprovedAssignments(startDate, numDays),
    enabled: Boolean(startDate) && numDays > 0,
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

function useEmployeeMutation<TVariables>(mutationFn: (variables: TVariables) => Promise<unknown>) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn,
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['employees'] })
    },
  })
}

export function useCreateEmployee() {
  return useEmployeeMutation((params: CreateEmployeeParams) => api.createEmployee(params))
}

export function useUpdateEmployee() {
  return useEmployeeMutation(({ employeeId, params }: { employeeId: string; params: UpdateEmployeeParams }) =>
    api.updateEmployee(employeeId, params),
  )
}

export function useDeactivateEmployee() {
  return useEmployeeMutation((employeeId: string) => api.deactivateEmployee(employeeId))
}

export function useRestoreEmployee() {
  return useEmployeeMutation((employeeId: string) => api.restoreEmployee(employeeId))
}
