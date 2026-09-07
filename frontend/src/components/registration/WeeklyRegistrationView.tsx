import { useMemo, useState } from 'react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { useEmployees, useSetAvailability, useSetLeaveDay } from '@/api/hooks'
import { errorMessage } from '@/lib/errors'
import { addWeeks, dateRange, formatShortDate, formatWeekdayDate, startOfWeek, todayISO } from '@/lib/dates'
import type { AvailabilityMap, Employee } from '@/api/types'
import { cn } from '@/lib/utils'

type DayStatus = 'unset' | 'sang' | 'dem' | 'both' | 'off' | 'leave'

const STATUS_META: Record<DayStatus, { label: string; shortLabel: string; className: string }> = {
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

const SELECTABLE_STATUSES: DayStatus[] = ['sang', 'dem', 'both', 'off', 'leave']

function computeStatus(employee: Employee, availability: AvailabilityMap, date: string): DayStatus {
  if (employee.leave_days?.includes(date)) return 'leave'
  const avail = availability[employee.employee_id]?.[date]
  if (!avail) return 'unset'
  if (avail.sang && avail.dem) return 'both'
  if (avail.sang) return 'sang'
  if (avail.dem) return 'dem'
  return 'off'
}

export function WeeklyRegistrationView() {
  const employeesQuery = useEmployees()
  const setAvailability = useSetAvailability()
  const setLeaveDay = useSetLeaveDay()

  const [employeeId, setEmployeeId] = useState<string>('')
  const [weekStart, setWeekStart] = useState(() => startOfWeek(todayISO()))
  const [openDate, setOpenDate] = useState<string | null>(null)

  const employees = employeesQuery.data?.employees ?? []
  const activeEmployeeId = employeeId || employees[0]?.employee_id || ''
  const activeEmployee = employees.find((e) => e.employee_id === activeEmployeeId)
  const availability = employeesQuery.data?.availability ?? {}
  const days = useMemo(() => dateRange(weekStart, 7), [weekStart])

  const isPending = setAvailability.isPending || setLeaveDay.isPending

  async function applyStatus(date: string, status: DayStatus) {
    if (!activeEmployee) return
    try {
      if (status === 'leave') {
        await setLeaveDay.mutateAsync({ employeeId: activeEmployee.employee_id, params: { date, on_leave: true } })
        await setAvailability.mutateAsync({
          employeeId: activeEmployee.employee_id,
          params: { date, sang: false, dem: false },
        })
      } else {
        await setLeaveDay.mutateAsync({ employeeId: activeEmployee.employee_id, params: { date, on_leave: false } })
        await setAvailability.mutateAsync({
          employeeId: activeEmployee.employee_id,
          params: { date, sang: status === 'sang' || status === 'both', dem: status === 'dem' || status === 'both' },
        })
      }
      toast.success(`Đã đăng ký ${STATUS_META[status].label.toLowerCase()} — ${date}`)
    } catch (err) {
      toast.error(errorMessage(err))
    }
  }

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle>Đăng ký lịch tuần</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex flex-wrap items-end gap-4">
            <div className="grid gap-1.5">
              <label htmlFor="registration-employee" className="text-sm font-medium">
                Nhân viên
              </label>
              <select
                id="registration-employee"
                value={activeEmployeeId}
                onChange={(e) => setEmployeeId(e.target.value)}
                className="h-9 w-56 rounded-md border border-input bg-transparent px-3 text-sm"
              >
                {employees.map((e) => (
                  <option key={e.employee_id} value={e.employee_id}>
                    {e.employee_id} — {e.name}
                  </option>
                ))}
              </select>
            </div>

            <div className="flex items-center gap-2">
              <Button type="button" variant="outline" size="sm" onClick={() => setWeekStart((w) => addWeeks(w, -1))}>
                ← Tuần trước
              </Button>
              <span className="text-sm font-medium">
                Tuần {formatShortDate(days[0])} – {formatShortDate(days[6])}
              </span>
              <Button type="button" variant="outline" size="sm" onClick={() => setWeekStart((w) => addWeeks(w, 1))}>
                Tuần sau →
              </Button>
            </div>
          </div>

          <div className="flex flex-wrap gap-3 border-t pt-3">
            {(Object.keys(STATUS_META) as DayStatus[]).map((status) => (
              <span key={status} className="flex items-center gap-1.5 text-xs text-muted-foreground">
                <span className={cn('inline-block size-3 rounded-full', STATUS_META[status].className)} />
                {STATUS_META[status].label}
              </span>
            ))}
          </div>
        </CardContent>
      </Card>

      {employeesQuery.isLoading && <p className="text-sm text-muted-foreground">Đang tải danh sách nhân viên...</p>}
      {employeesQuery.isError && (
        <p className="text-sm text-destructive">Không tải được nhân viên: {errorMessage(employeesQuery.error)}</p>
      )}

      {activeEmployee && (
        <Card>
          <CardHeader>
            <CardTitle>
              {activeEmployee.name} <span className="font-normal text-muted-foreground">({activeEmployee.role})</span>
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="grid grid-cols-7 gap-3">
              {days.map((date) => {
                const status = computeStatus(activeEmployee, availability, date)
                const meta = STATUS_META[status]
                return (
                  <Popover key={date} open={openDate === date} onOpenChange={(o) => setOpenDate(o ? date : null)}>
                    <PopoverTrigger asChild>
                      <button
                        type="button"
                        disabled={isPending}
                        className="flex flex-col items-stretch gap-1.5 rounded-xl p-1 text-left transition-transform hover:-translate-y-0.5 disabled:opacity-60"
                      >
                        <span className="text-xs font-medium text-muted-foreground">{formatWeekdayDate(date)}</span>
                        <span
                          className={cn(
                            'flex h-12 items-center justify-center rounded-lg text-sm font-semibold shadow-sm',
                            meta.className,
                          )}
                        >
                          {meta.shortLabel}
                        </span>
                      </button>
                    </PopoverTrigger>
                    <PopoverContent className="w-56">
                      <p className="mb-2 text-xs font-medium text-muted-foreground">{formatWeekdayDate(date)}</p>
                      <div className="flex flex-col gap-1">
                        {SELECTABLE_STATUSES.map((s) => (
                          <Button
                            key={s}
                            type="button"
                            variant={status === s ? 'default' : 'outline'}
                            size="sm"
                            className="justify-start"
                            disabled={isPending}
                            onClick={() => {
                              // Close immediately so a second click can't fire
                              // an overlapping request against the same day
                              // while the first PUT is still in flight.
                              setOpenDate(null)
                              void applyStatus(date, s)
                            }}
                          >
                            {STATUS_META[s].label}
                          </Button>
                        ))}
                      </div>
                    </PopoverContent>
                  </Popover>
                )
              })}
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  )
}
