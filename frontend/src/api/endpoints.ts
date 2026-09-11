import { BASE_URL, deleteJSON, getJSON, postJSON, putJSON } from './client'
import type {
  AckResponse,
  ApproveResponse,
  CapacityCheckResult,
  EmployeesResponse,
  GateShiftRequirement,
  LatestScheduleResponse,
  ListApprovedResponse,
  LockedAssignment,
  Employee,
  ReplacementCandidatesResult,
  ShiftType,
  SolveResult,
  Role,
  SolverConfig,
  TargetSlot,
  UnapproveResponse,
} from './types'

export function fetchEmployees(includeInactive = false, from?: string, to?: string) {
  const params = new URLSearchParams()
  if (includeInactive) params.set('include_inactive', 'true')
  if (from) params.set('from', from)
  if (to) params.set('to', to)
  const query = params.toString()
  return getJSON<EmployeesResponse>(`/api/v1/employees${query ? `?${query}` : ''}`)
}

export function fetchConfig() {
  return getJSON<SolverConfig>('/api/v1/config')
}

export interface UpdateGateShiftRequirementParams extends GateShiftRequirement {
  shift_hours: number
}

export function updateGateShiftRequirement(gateCode: string, shiftType: ShiftType, params: UpdateGateShiftRequirementParams) {
  return putJSON<SolverConfig>(`/api/v1/config/gates/${encodeURIComponent(gateCode)}/shifts/${encodeURIComponent(shiftType)}`, params)
}

export function renameGate(gateCode: string, newCode: string) {
  return putJSON<SolverConfig>(`/api/v1/config/gates/${encodeURIComponent(gateCode)}/rename`, { new_code: newCode })
}

export interface CreateEmployeeParams {
  employee_id: string
  name: string
  role: Role
}

export interface UpdateEmployeeParams {
  name: string
  role: Role
}

export function createEmployee(params: CreateEmployeeParams) {
  return postJSON<Employee>('/api/v1/employees', params)
}

export function updateEmployee(employeeId: string, params: UpdateEmployeeParams) {
  return putJSON<Employee>(`/api/v1/employees/${encodeURIComponent(employeeId)}`, params)
}

export function deactivateEmployee(employeeId: string) {
  return deleteJSON<Employee>(`/api/v1/employees/${encodeURIComponent(employeeId)}`)
}

export function restoreEmployee(employeeId: string) {
  return postJSON<Employee>(`/api/v1/employees/${encodeURIComponent(employeeId)}/restore`, {})
}

export function fetchLatestSchedule() {
  return getJSON<LatestScheduleResponse>('/api/v1/schedule/latest')
}

export interface SolveParams {
  start_date: string
  num_days: number
  time_limit_s?: number
  ignore_approved?: boolean
}

export function solveSchedule(params: SolveParams) {
  return postJSON<SolveResult>('/api/v1/schedule/solve', params)
}

export function approveAssignments(assignments: LockedAssignment[]) {
  return postJSON<ApproveResponse>('/api/v1/schedule/approve', { assignments })
}

export interface UnapproveParams {
  start_date: string
  num_days: number
}

export function unapproveAssignments(params: UnapproveParams) {
  return postJSON<UnapproveResponse>('/api/v1/schedule/unapprove', params)
}

export function fetchApprovedAssignments(startDate: string, numDays: number) {
  const params = new URLSearchParams({ start_date: startDate, num_days: String(numDays) })
  return getJSON<ListApprovedResponse>(`/api/v1/schedule/approved?${params.toString()}`)
}

// Not a fetch+blob download: the endpoint has no auth, so the browser can
// just navigate to it directly and let Content-Disposition drive the save.
export function scheduleExportUrl() {
  return `${BASE_URL}/api/v1/schedule/export`
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
