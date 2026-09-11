import { useCallback, useMemo, useState, type FormEvent } from 'react'
import { CircleCheckIcon } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Dialog, DialogClose, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  useApprove,
  useApprovedAssignments,
  useConfig,
  useEmployees,
  useLatestSchedule,
  useSolve,
  useUnapprove,
} from '@/api/hooks'
import * as api from '@/api/endpoints'
import { errorMessage } from '@/lib/errors'
import { addWeeks, dateRange, formatShortDate, startOfWeek, todayISO } from '@/lib/dates'
import type { LockedAssignment, ScheduleEntry } from '@/api/types'
import { EmployeeSummaryTable } from './EmployeeSummaryTable'
import {
  overrideKey,
  resolveApprovedBaseline,
  resolveCell,
  ScheduleTable,
  type CellAssignment,
  type CellOverrides,
} from './ScheduleTable'
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
  const [solveDialogOpen, setSolveDialogOpen] = useState(false)
  const [solveMode, setSolveMode] = useState<'unapproved_only' | 'all'>('unapproved_only')
  const [unapproveDialogOpen, setUnapproveDialogOpen] = useState(false)

  const employeesQuery = useEmployees()
  const configQuery = useConfig()
  const latestQuery = useLatestSchedule()
  const solve = useSolve()
  const approve = useApprove()
  const unapprove = useUnapprove()

  const result = latestQuery.data?.found ? latestQuery.data.result : undefined

  const approvedQuery = useApprovedAssignments(result?.start_date ?? '', result?.num_days ?? 0)
  // What's actually confirmed on the server, by cell — the durable baseline
  // both the grid's display AND handleApprove's submission fall back to for
  // any cell not currently in `overrides` (see resolveApprovedBaseline).
  // `overrides` only holds *staged, not-yet-submitted* edits and is cleared
  // right after a successful approve; without this map, that clearing (or a
  // plain page reload, which wipes `overrides` too) would make the next
  // approve fall back to `result.schedule` — the solver's original,
  // immutable output — silently reverting any already-approved edit back to
  // its pre-edit value. Also drives the "approved" checkmark.
  //
  // Trade-off: if a solve later runs and downgrades this cell's lock because
  // the employee's availability/leave changed (sanitizeLockedAssignments,
  // schedule_service.go), `result.schedule` reflects that correction but this
  // map still holds the now-stale approved value until the manager
  // re-approves or unapproves — a narrow, pre-existing edge case, not a
  // regression this introduces.
  const approvedAssignments = useMemo(() => {
    const map = new Map<string, CellAssignment | null>()
    for (const a of approvedQuery.data?.assignments ?? []) {
      map.set(overrideKey(a.employee_id, a.date), a.off ? null : { gate: a.gate!, shift: a.shift! })
    }
    return map
  }, [approvedQuery.data])
  const shortageDates = useMemo(() => new Set((result?.shortages ?? []).map((s) => s.date)), [result])

  // A fresh solve result replaces whatever manual overrides were staged on
  // top of the previous one — they applied to a schedule that's now gone.
  // Resetting during render (rather than in an effect) avoids an extra
  // render pass; see https://react.dev/learn/you-might-not-need-an-effect#adjusting-some-state-when-a-prop-changes.
  if (result?.run_id !== overridesRunID) {
    setOverridesRunID(result?.run_id)
    setOverrides(new Map())
  }

  // The Start Date / Num Days inputs no longer submit directly — "Xếp lịch"
  // opens the mode dialog instead, so Enter in an input must not trigger a
  // solve behind the manager's back.
  function handleDateFormSubmit(e: FormEvent) {
    e.preventDefault()
  }

  function runSolve(ignoreApproved: boolean) {
    solve.mutate(
      { start_date: startDate, num_days: numDays, time_limit_s: 30, ignore_approved: ignoreApproved },
      {
        onSuccess: (r) => {
          setStartDate(r.start_date)
          setNumDays(r.num_days)
          toast.success(`Solve xong: ${r.status} — ${r.shortages.length} vị trí thiếu`)
        },
        onError: (err) => toast.error(errorMessage(err)),
      },
    )
    setSolveDialogOpen(false)
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
    // For a cell with no live override, fall back to what's already approved
    // (resolveApprovedBaseline) rather than straight to the solver's raw
    // output — otherwise re-approving after `overrides` has been cleared
    // (which happens right below on success, and on every page reload)
    // would silently revert every previously-approved edit back to its
    // pre-edit value. Manual overrides staged in the grid still take
    // precedence over both, so a manager's click-to-edit is what actually
    // gets locked in.
    const dates = dateRange(result.start_date, result.num_days)
    const assignments: LockedAssignment[] = employeesQuery.data.employees.flatMap((employee) =>
      dates.map((date): LockedAssignment => {
        const key = overrideKey(employee.employee_id, date)
        const baseline = resolveApprovedBaseline(approvedAssignments, key, worked.get(key) ?? null)
        const shift = resolveCell(overrides, employee.employee_id, date, baseline)
        return shift
          ? { employee_id: employee.employee_id, date, gate: shift.gate, shift: shift.shift, off: false }
          : { employee_id: employee.employee_id, date, gate: null, shift: null, off: true }
      }),
    )

    approve.mutate(assignments, {
      onSuccess: (res) => {
        toast.success(`Đã duyệt ${res.approved_count} ô — sẽ được giữ nguyên ở lần solve sau`)
        // Safe to clear now: any cell just submitted is about to reappear in
        // `approvedAssignments` (refetched by useApprove's onSuccess), which
        // is exactly what handleApprove and the grid both fall back to for a
        // cell with no live override — so nothing is lost by clearing.
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
          <CardTitle>Chạy trình xếp lịch</CardTitle>
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
          <form onSubmit={handleDateFormSubmit} className="flex flex-wrap items-end gap-4">
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
            <Button type="button" disabled={solve.isPending} onClick={() => setSolveDialogOpen(true)}>
              {solve.isPending ? 'Đang chạy...' : 'Xếp lịch'}
            </Button>
            {result && (
              <Button type="button" variant="outline" onClick={handleApprove} disabled={approve.isPending}>
                {approve.isPending ? 'Đang duyệt...' : `Phê duyệt lịch này${overrides.size > 0 ? ` (${overrides.size} ô đã sửa)` : ''}`}
              </Button>
            )}
            {result && (
              <Button
                type="button"
                variant="outline"
                onClick={() => setUnapproveDialogOpen(true)}
                disabled={unapprove.isPending}
              >
                {unapprove.isPending ? 'Đang hủy...' : 'Hủy phê duyệt'}
              </Button>
            )}
            {result && (
              <Button type="button" variant="outline" asChild>
                <a href={api.scheduleExportUrl()}>Xuất Excel</a>
              </Button>
            )}
          </form>
        </CardContent>
      </Card>

      <Dialog open={solveDialogOpen} onOpenChange={setSolveDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Xếp lịch</DialogTitle>
            <DialogDescription>
              Chọn cách xếp lịch cho khung {formatShortDate(startDate)} — {numDays} ngày.
            </DialogDescription>
          </DialogHeader>
          <div className="grid gap-3">
            <label className="flex items-start gap-2 text-sm">
              <input
                type="radio"
                name="solve-mode"
                className="mt-1"
                checked={solveMode === 'unapproved_only'}
                onChange={() => setSolveMode('unapproved_only')}
              />
              <span>
                <span className="font-medium">Chỉ xếp các ô chưa duyệt</span>
                <br />
                <span className="text-xs text-muted-foreground">Giữ nguyên các ô đã phê duyệt, chỉ xếp lại phần còn lại.</span>
              </span>
            </label>
            <label className="flex items-start gap-2 text-sm">
              <input
                type="radio"
                name="solve-mode"
                className="mt-1"
                checked={solveMode === 'all'}
                onChange={() => setSolveMode('all')}
              />
              <span>
                <span className="font-medium">Xếp lại toàn bộ</span>
                <br />
                <span className="text-xs text-muted-foreground">
                  Bỏ qua các ô đã duyệt cho lần chạy này. Chỉ áp dụng cho kết quả lần này — dữ liệu đã duyệt trong hệ
                  thống không đổi trừ khi bấm Phê duyệt lại.
                </span>
              </span>
            </label>
          </div>
          <DialogFooter>
            <DialogClose asChild>
              <Button type="button" variant="outline">
                Hủy
              </Button>
            </DialogClose>
            <Button type="button" onClick={() => runSolve(solveMode === 'all')} disabled={solve.isPending}>
              {solve.isPending ? 'Đang chạy...' : 'Xếp lịch'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={unapproveDialogOpen} onOpenChange={setUnapproveDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Hủy phê duyệt</DialogTitle>
            <DialogDescription>
              {result &&
                `Toàn bộ ${result.num_days} ngày đã duyệt kể từ ${formatShortDate(result.start_date)} sẽ bị hủy. Lần xếp lịch tiếp theo (kể cả chế độ "chỉ xếp chưa duyệt") sẽ xếp lại từ đầu cho khung này.`}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <DialogClose asChild>
              <Button type="button" variant="outline">
                Đóng
              </Button>
            </DialogClose>
            <Button
              type="button"
              variant="destructive"
              disabled={unapprove.isPending}
              onClick={() => {
                if (!result) return
                unapprove.mutate(
                  { start_date: result.start_date, num_days: result.num_days },
                  {
                    onSuccess: (res) => {
                      toast.success(`Đã hủy phê duyệt ${res.unapproved_count} ô`)
                      setUnapproveDialogOpen(false)
                    },
                    onError: (err) => toast.error(errorMessage(err)),
                  },
                )
              }}
            >
              {unapprove.isPending ? 'Đang hủy...' : 'Xác nhận hủy phê duyệt'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {employeesQuery.isLoading && <p className="text-sm text-muted-foreground">Đang tải danh sách nhân viên...</p>}
       {employeesQuery.isError && (
        <p className="text-sm text-destructive">Không tải được nhân viên: {errorMessage(employeesQuery.error)}</p>
       )}
       {configQuery.isError && <p className="text-sm text-destructive">Không tải được cấu hình cổng/ca: {errorMessage(configQuery.error)}</p>}

      {result && employeesQuery.data ? (
        <>
          <Card>
            <CardHeader>
              <CardTitle>
                Lịch xếp ca - {result.status} ({result.wall_time_s.toFixed(2)}s)
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
                <span className="flex items-center gap-1.5">
                  <CircleCheckIcon className="size-3.5 text-status-approved" /> Đã duyệt
                </span>
                <span className="flex items-center gap-1.5">
                  <span className="inline-block size-3 rounded border-b-2 border-destructive bg-destructive/5" /> Ngày thiếu ca
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
                shiftHours={configQuery.data?.shift_hours ?? {}}
                overrides={overrides}
                onEditCell={handleEditCell}
                approvedAssignments={approvedAssignments}
                shortageDates={shortageDates}
              />
            </CardContent>
          </Card>

          <Card
            className={
              result.shortages.length > 0 ? 'ring-2 ring-destructive/60 bg-destructive/5' : undefined
            }
          >
            <CardHeader>
              <CardTitle className={result.shortages.length > 0 ? 'text-destructive' : undefined}>
                {result.shortages.length > 0 ? '⚠ ' : ''}
                Thiếu ca ({result.shortages.length})
              </CardTitle>
              {result.shortages.length > 0 && (
                <p className="text-sm text-destructive">
                  Thiếu nhân sự khiến cổng không đủ người để hoạt động — cần xử lý trước khi phê duyệt lịch.
                </p>
              )}
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
