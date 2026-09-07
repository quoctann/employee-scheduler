// Mirrors the Go backend's JSON contract exactly (which itself mirrors
// solver-service's schemas) — see backend/internal/core/domain.

export type Role = 'NV' | 'TC' | 'PC'
export type ShiftType = 'sang' | 'dem'
export type ShortageType = 'staff' | 'lead'
export type SolveStatus = 'OPTIMAL' | 'FEASIBLE' | 'INFEASIBLE' | 'UNKNOWN' | 'MODEL_INVALID'

export interface Employee {
  employee_id: string
  name: string
  role: Role
  leave_days?: string[]
}

export interface ShiftAvailability {
  sang: boolean
  dem: boolean
}

export type AvailabilityMap = Record<string, Record<string, ShiftAvailability>>

export interface ScheduleEntry {
  employee_id: string
  date: string
  gate: string
  shift: ShiftType
}

export interface ShortageItem {
  date: string
  gate: string
  shift: ShiftType
  shortage_type: ShortageType
  missing: number
}

export interface EmployeeSummary {
  employee_id: string
  role: Role
  leave_days: number
  target_hours: number
  actual_hours: number
  deviation_hours: number
  actual_shifts: number
}

export interface SolveResult {
  run_id?: number
  start_date: string
  num_days: number
  status: SolveStatus
  objective_value: number | null
  wall_time_s: number
  schedule: ScheduleEntry[]
  shortages: ShortageItem[]
  employee_summary: EmployeeSummary[]
}

export interface CapacityCheckResult {
  demand_hours_per_day: number
  demand_hours_total: number
  target_hours_per_employee: number
  min_employees_required: number
  employee_count: number
  is_sufficient: boolean
  shortfall_ratio: number
}

export interface TargetSlot {
  date: string
  gate: string
  shift: ShiftType
  requires_lead: boolean
}

export interface CandidateItem {
  employee_id: string
  role: Role
  score: number
  target_hours: number
  actual_hours_so_far: number
  deviation_hours: number
  would_create_streak: boolean
  reasons: string[]
}

export interface ReplacementCandidatesResult {
  target_slot: TargetSlot
  candidates: CandidateItem[]
  excluded_count: number
}

export interface LockedAssignment {
  employee_id: string
  date: string
  gate: string | null
  shift: ShiftType | null
  off: boolean
}

export interface EmployeesResponse {
  employees: Employee[]
  availability: AvailabilityMap
}

export interface LatestScheduleResponse {
  found: boolean
  result?: SolveResult
}

export interface ApproveResponse {
  approved_count: number
}

export interface AckResponse {
  ok: boolean
}
