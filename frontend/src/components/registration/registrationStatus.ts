import type { AvailabilityMap, Employee } from '@/api/types'

export type DayStatus = 'unset' | 'sang' | 'dem' | 'both' | 'off' | 'leave'

export const STATUS_META: Record<DayStatus, { label: string; shortLabel: string; className: string }> = {
  unset: {
    label: 'Chưa đăng ký',
    shortLabel: '—',
    className: 'border border-dashed border-border bg-transparent text-muted-foreground',
  },
  sang: { label: 'Sáng', shortLabel: 'Sáng', className: 'bg-status-sang text-status-sang-foreground' },
  dem: { label: 'Đêm', shortLabel: 'Đêm', className: 'bg-status-dem text-status-dem-foreground' },
  both: {
    label: 'Cả hai',
    shortLabel: 'Cả hai',
    className: 'bg-gradient-to-r from-status-sang to-status-dem text-foreground',
  },
  off: { label: 'Không làm (N)', shortLabel: 'N', className: 'bg-status-off text-status-off-foreground' },
  leave: { label: 'Nghỉ phép', shortLabel: 'Nghỉ', className: 'bg-status-leave text-status-leave-foreground' },
}

export const SELECTABLE_STATUSES: DayStatus[] = ['sang', 'dem', 'both', 'off', 'leave']

export function computeStatus(employee: Employee, availability: AvailabilityMap, date: string): DayStatus {
  if (employee.leave_days?.includes(date)) return 'leave'
  const avail = availability[employee.employee_id]?.[date]
  if (!avail) return 'unset'
  if (avail.sang && avail.dem) return 'both'
  if (avail.sang) return 'sang'
  if (avail.dem) return 'dem'
  return 'off'
}

/** key = `${employeeId}|${date}`, matching the shape used by ScheduleTable's overrideKey. */
export function cellKey(employeeId: string, date: string): string {
  return `${employeeId}|${date}`
}
