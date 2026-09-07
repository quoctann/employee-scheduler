import { useCallback, useState, type FormEvent } from 'react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useApprove, useEmployees, useLatestSchedule, useSolve } from '@/api/hooks'
import { errorMessage } from '@/lib/errors'
import { addWeeks, dateRange, startOfWeek, todayISO } from '@/lib/dates'
import type { LockedAssignment, ScheduleEntry } from '@/api/types'
import { EmployeeSummaryTable } from './EmployeeSummaryTable'
import { overrideKey, resolveCell, ScheduleTable, type CellAssignment, type CellOverrides } from './ScheduleTable'
import { ShortagesTable } from './ShortagesTable'

const MIN_DAYS = 1
const MAX_DAYS = 180
const WEEK_DAYS = 7

function clampNumDays(raw: string): number {
  const n = Math.floor(Number(raw))
  if (!Number.isFinite(n)) return MIN_DAYS
  return Math.min(MAX_DAYS, Math.max(MIN_DAYS, n))
}

export function ScheduleView() {
  const [startDate, setStartDate] = useState(() => startOfWeek(todayISO()))
  const [numDays, setNumDays] = useState(WEEK_DAYS)
  const [overrides, setOverrides] = useState<CellOverrides>(new Map())
  const [overridesRunID, setOverridesRunID] = useState<number | undefined>(undefined)

  const employeesQuery = useEmployees()
  const latestQuery = useLatestSchedule()
  const solve = useSolve()
  const approve = useApprove()

  const result = latestQuery.data?.found ? latestQuery.data.result : undefined

  // A fresh solve result replaces whatever manual overrides were staged on
  // top of the previous one — they applied to a schedule that's now gone.
  // Resetting during render (rather than in an effect) avoids an extra
  // render pass; see https://react.dev/learn/you-might-not-need-an-effect#adjusting-some-state-when-a-prop-changes.
  if (result?.run_id !== overridesRunID) {
    setOverridesRunID(result?.run_id)
    setOverrides(new Map())
  }

  function handleSolve(e: FormEvent) {
    e.preventDefault()
    solve.mutate(
      { start_date: startDate, num_days: numDays, time_limit_s: 30 },
      {
        onSuccess: (r) => {
          setStartDate(r.start_date)
          setNumDays(r.num_days)
          toast.success(`Solve xong: ${r.status} — ${r.shortages.length} vị trí thiếu`)
        },
        onError: (err) => toast.error(errorMessage(err)),
      },
    )
  }

  const handleEditCell = useCallback((employeeId: string, date: string, assignment: CellAssignment | null) => {
    setOverrides((prev) => {
      const next = new Map(prev)
      next.set(overrideKey(employeeId, date), assignment)
      return next
    })
  }, [])

  function handleApprove() {
    if (!result || !employeesQuery.data) return

    const worked = new Map<string, CellAssignment>()
    for (const entry of result.schedule) {
      worked.set(overrideKey(entry.employee_id, entry.date), { gate: entry.gate, shift: entry.shift })
    }

    // Approve the *whole* horizon, not just worked shifts — a day with no
    // schedule entry for an employee is an implicit day off, and locking
    // only the worked cells would leave those days unconfirmed (a later
    // solve could freely reassign them, defeating the point of approving).
    // Manual overrides staged in the grid take precedence over the
    // solver's own output, so a manager's click-to-edit is what actually
    // gets locked in.
    const dates = dateRange(result.start_date, result.num_days)
    const assignments: LockedAssignment[] = employeesQuery.data.employees.flatMap((employee) =>
      dates.map((date): LockedAssignment => {
        const shift = resolveCell(overrides, employee.employee_id, date, worked.get(overrideKey(employee.employee_id, date)) ?? null)
        return shift
          ? { employee_id: employee.employee_id, date, gate: shift.gate, shift: shift.shift, off: false }
          : { employee_id: employee.employee_id, date, gate: null, shift: null, off: true }
      }),
    )

    approve.mutate(assignments, {
      onSuccess: (res) => {
        toast.success(`Đã duyệt ${res.approved_count} ô — sẽ được giữ nguyên ở lần solve sau`)
        // The grid's "edited" ring means "manually staged, not yet locked" —
        // once approved, these cells are the confirmed truth, not a pending
        // edit, so the ring should clear even though `result` itself hasn't
        // changed (no new run_id to trigger the render-time reset above).
        setOverrides(new Map())
      },
      onError: (err) => toast.error(errorMessage(err)),
    })
  }

  const scheduleForTable: ScheduleEntry[] = result?.schedule ?? []

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle>Chạy solver</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="flex items-center gap-2">
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() => setStartDate((s) => addWeeks(s, -1))}
            >
              ← Tuần trước
            </Button>
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() => {
                setStartDate(startOfWeek(todayISO()))
                setNumDays(WEEK_DAYS)
              }}
            >
              Tuần này
            </Button>
            <Button type="button" variant="outline" size="sm" onClick={() => setStartDate((s) => addWeeks(s, 1))}>
              Tuần sau →
            </Button>
          </div>
          <form onSubmit={handleSolve} className="flex flex-wrap items-end gap-4">
            <div className="grid gap-1.5">
              <Label htmlFor="start-date">Ngày bắt đầu</Label>
              <Input
                id="start-date"
                type="date"
                value={startDate}
                onChange={(e) => setStartDate(e.target.value)}
                className="w-40"
              />
            </div>
            <div className="grid gap-1.5">
              <Label htmlFor="num-days">Số ngày</Label>
              <Input
                id="num-days"
                type="number"
                min={MIN_DAYS}
                max={MAX_DAYS}
                value={numDays}
                onChange={(e) => setNumDays(clampNumDays(e.target.value))}
                className="w-24"
              />
            </div>
            <Button type="submit" disabled={solve.isPending}>
              {solve.isPending ? 'Đang chạy...' : 'Solve'}
            </Button>
            {result && (
              <Button type="button" variant="outline" onClick={handleApprove} disabled={approve.isPending}>
                {approve.isPending ? 'Đang duyệt...' : `Approve lịch này${overrides.size > 0 ? ` (${overrides.size} ô đã sửa)` : ''}`}
              </Button>
            )}
          </form>
        </CardContent>
      </Card>

      {employeesQuery.isLoading && <p className="text-sm text-muted-foreground">Đang tải danh sách nhân viên...</p>}
      {employeesQuery.isError && (
        <p className="text-sm text-destructive">Không tải được nhân viên: {errorMessage(employeesQuery.error)}</p>
      )}

      {result && employeesQuery.data ? (
        <>
          <Card>
            <CardHeader>
              <CardTitle>
                Lịch xếp ca — {result.status} ({result.wall_time_s.toFixed(2)}s)
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-3">
              <p className="flex flex-wrap items-center gap-3 text-xs text-muted-foreground">
                <span className="flex items-center gap-1.5">
                  <span className="inline-block size-3 rounded bg-status-sang" /> Sáng
                </span>
                <span className="flex items-center gap-1.5">
                  <span className="inline-block size-3 rounded bg-status-dem" /> Đêm
                </span>
                <span className="flex items-center gap-1.5">
                  <span className="inline-block size-3 rounded ring-2 ring-status-edited" /> Đã sửa tay (chưa lưu)
                </span>
                <span>Bấm vào một ô để sửa cổng/ca hoặc đặt Off.</span>
              </p>
              <ScheduleTable
                // Remounts (discarding any open popover's local form state)
                // whenever a fresh solve replaces the underlying schedule —
                // otherwise a cell left open across a re-solve could save
                // stale gate/shift values the user can no longer see.
                key={`${result.run_id ?? ''}|${result.start_date}|${result.num_days}`}
                employees={employeesQuery.data.employees}
                schedule={scheduleForTable}
                startDate={result.start_date}
                numDays={result.num_days}
                overrides={overrides}
                onEditCell={handleEditCell}
              />
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>Thiếu ca ({result.shortages.length})</CardTitle>
            </CardHeader>
            <CardContent>
              <ShortagesTable shortages={result.shortages} />
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>Tổng hợp giờ làm</CardTitle>
            </CardHeader>
            <CardContent>
              <EmployeeSummaryTable summary={result.employee_summary} />
            </CardContent>
          </Card>
        </>
      ) : (
        !solve.isPending && <p className="text-sm text-muted-foreground">Chưa có lịch nào — bấm Solve để tạo lịch đầu tiên.</p>
      )}
    </div>
  )
}
