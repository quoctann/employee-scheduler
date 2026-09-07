import { getJSON, postJSON, putJSON } from './client'
import type {
  AckResponse,
  ApproveResponse,
  CapacityCheckResult,
  EmployeesResponse,
  LatestScheduleResponse,
  LockedAssignment,
  ReplacementCandidatesResult,
  SolveResult,
  TargetSlot,
} from './types'

export function fetchEmployees() {
  return getJSON<EmployeesResponse>('/api/v1/employees')
}

export function fetchLatestSchedule() {
  return getJSON<LatestScheduleResponse>('/api/v1/schedule/latest')
}

export interface SolveParams {
  start_date: string
  num_days: number
  time_limit_s?: number
}

export function solveSchedule(params: SolveParams) {
  return postJSON<SolveResult>('/api/v1/schedule/solve', params)
}

export function approveAssignments(assignments: LockedAssignment[]) {
  return postJSON<ApproveResponse>('/api/v1/schedule/approve', { assignments })
}

export interface CapacityCheckParams {
  num_days: number
  employee_count?: number
}

export function checkCapacity(params: CapacityCheckParams) {
  return postJSON<CapacityCheckResult>('/api/v1/capacity-check', params)
}

export interface CandidatesParams {
  target_slot: TargetSlot
  excluded_employee_id?: string
  top_n?: number
}

export function fetchCandidates(params: CandidatesParams) {
  return postJSON<ReplacementCandidatesResult>('/api/v1/candidates', params)
}

export interface SetAvailabilityParams {
  date: string
  sang: boolean
  dem: boolean
}

export function setAvailability(employeeId: string, params: SetAvailabilityParams) {
  return putJSON<AckResponse>(`/api/v1/employees/${employeeId}/availability`, params)
}

export interface SetLeaveDayParams {
  date: string
  on_leave: boolean
}

export function setLeaveDay(employeeId: string, params: SetLeaveDayParams) {
  return putJSON<AckResponse>(`/api/v1/employees/${employeeId}/leave`, params)
}
