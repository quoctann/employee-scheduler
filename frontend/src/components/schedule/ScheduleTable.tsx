import { useMemo, useState } from 'react'
import { createColumnHelper, flexRender, getCoreRowModel, useReactTable } from '@tanstack/react-table'
import { CircleCheckIcon } from 'lucide-react'
import { toast } from 'sonner'
import { RoleBadge } from '@/components/RoleBadge'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { dateRange, formatShortDate } from '@/lib/dates'
import { cn } from '@/lib/utils'
import type { Employee, Role, ScheduleEntry, ShiftHours, ShiftType } from '@/api/types'

export interface CellAssignment {
  gate: string
  shift: ShiftType
}

/** key = `${employeeId}|${date}`; a present entry with value `null` is an explicit manual "Off". */
export type CellOverrides = Map<string, CellAssignment | null>

export function overrideKey(employeeId: string, date: string): string {
  return `${employeeId}|${date}`
}

/** The manual override for a cell wins over the solver's original value —
 *  but only when one was actually staged (`Map.has`, not `??`): a staged
 *  "Off" override is `null`, which `??` would silently fall back to
 *  `original` and undo. Centralized here so every reader agrees. */
export function resolveCell(
  overrides: CellOverrides,
  employeeId: string,
  date: string,
  original: CellAssignment | null,
): CellAssignment | null {
  const key = overrideKey(employeeId, date)
  return overrides.has(key) ? (overrides.get(key) ?? null) : original
}

/** Priority for what a cell's value is when there's no *live* override:
 *  the server's approved state if this cell has one, else `fallback` (the
 *  raw solver output). Shared by the grid's display and by ScheduleView's
 *  approve-submission logic — they must agree on this, or re-approving a
 *  cell with no current override (e.g. right after a page reload resets
 *  `overrides`) would silently revert it to its pre-approval value instead
 *  of re-confirming what's already approved. */
export function resolveApprovedBaseline(
  approvedAssignments: Map<string, CellAssignment | null>,
  key: string,
  fallback: CellAssignment | null,
): CellAssignment | null {
  return approvedAssignments.has(key) ? (approvedAssignments.get(key) ?? null) : fallback
}

interface ScheduleRow {
  employeeId: string
  name: string
  role: Role
  cells: Record<string, CellAssignment>
}

interface ScheduleTableProps {
  employees: Employee[]
  schedule: ScheduleEntry[]
  startDate: string
  numDays: number
  shiftHours: ShiftHours
  overrides: CellOverrides
  onEditCell: (employeeId: string, date: string, assignment: CellAssignment | null) => void
  /** keys are `${employeeId}|${date}` (see `overrideKey`); value is `null` for
   *  an approved "Off". This is the server-confirmed baseline a cell falls
   *  back to once it's not in `overrides` anymore (e.g. after a page
   *  reload) — without it, an approved manual edit would silently revert to
   *  the original solver output once the in-memory override is gone. Also
   *  drives the "approved" checkmark (any cell present here is approved). */
  approvedAssignments: Map<string, CellAssignment | null>
  /** dates (YYYY-MM-DD) that have at least one shortage somewhere in that
   *  day's gates — shortages are per gate/shift/date, not per employee, so
   *  this highlights the whole date column rather than a specific cell. */
  shortageDates: Set<string>
}

const columnHelper = createColumnHelper<ScheduleRow>()

/** Pivot: rows = roster employees, columns = each date in the horizon, cell = "Gate-duration" or OFF.
 *  Every cell is click-to-edit: a manager can override the solver's assignment (different gate/shift,
 *  or Off), tracked in `overrides` and highlighted until the edited schedule is approved. */
export function ScheduleTable({
  employees,
  schedule,
  startDate,
  numDays,
  shiftHours,
  overrides,
  onEditCell,
  approvedAssignments,
  shortageDates,
}: ScheduleTableProps) {
  const dates = useMemo(() => dateRange(startDate, numDays), [startDate, numDays])

  const rows = useMemo<ScheduleRow[]>(() => {
    const byEmployee = new Map<string, Record<string, CellAssignment>>()
    for (const entry of schedule) {
      const cells = byEmployee.get(entry.employee_id) ?? {}
      // Keep gate/shift as separate fields — joining into one string and
      // splitting it back would silently mis-parse any gate code that
      // itself contains a "-".
      cells[entry.date] = { gate: entry.gate, shift: entry.shift }
      byEmployee.set(entry.employee_id, cells)
    }
    return employees
      .slice()
      .sort((a, b) => a.employee_id.localeCompare(b.employee_id))
      .map((e) => ({
        employeeId: e.employee_id,
        name: e.name,
        role: e.role,
        cells: byEmployee.get(e.employee_id) ?? {},
      }))
  }, [employees, schedule])

  const columns = useMemo(
    () => [
      columnHelper.accessor('name', {
        header: 'Nhân viên',
        cell: (info) => (
          <div className="flex flex-col">
            <span className="font-medium">{info.getValue()}</span>
            <RoleBadge role={info.row.original.role} className="text-xs text-muted-foreground" />
          </div>
        ),
      }),
      ...dates.map((date) =>
        columnHelper.display({
          id: date,
          header: () => (
            <span className={cn(shortageDates.has(date) && 'font-semibold text-destructive')}>
              {formatShortDate(date)}
            </span>
          ),
          cell: (info) => {
            const employeeId = info.row.original.employeeId
            const key = overrideKey(employeeId, date)
            const original = info.row.original.cells[date] ?? null
            // approvedAssignments (server truth) wins over the raw solver
            // output as the baseline `overrides` layers on top of — see the
            // prop doc above for why (an approved edit must survive
            // `overrides` being wiped, e.g. by a page reload).
            const baseline = resolveApprovedBaseline(approvedAssignments, key, original)
            const effective = resolveCell(overrides, employeeId, date, baseline)
            return (
              <EditableCell
                value={effective}
                shiftHours={shiftHours}
                edited={overrides.has(key)}
                approved={approvedAssignments.has(key)}
                onSave={(assignment) => onEditCell(employeeId, date, assignment)}
              />
            )
          },
        }),
      ),
    ],
    [dates, overrides, onEditCell, shiftHours, approvedAssignments, shortageDates],
  )

  const table = useReactTable({ data: rows, columns, getCoreRowModel: getCoreRowModel() })

  return (
    <Table>
      <TableHeader>
        {table.getHeaderGroups().map((headerGroup) => (
          <TableRow key={headerGroup.id}>
            {headerGroup.headers.map((header) => (
              <TableHead
                key={header.id}
                className={cn(shortageDates.has(header.id) && 'border-b-2 border-destructive bg-destructive/5')}
              >
                {flexRender(header.column.columnDef.header, header.getContext())}
              </TableHead>
            ))}
          </TableRow>
        ))}
      </TableHeader>
      <TableBody>
        {table.getRowModel().rows.map((row) => (
          <TableRow key={row.id}>
            {row.getVisibleCells().map((cell) => (
              <TableCell key={cell.id}>{flexRender(cell.column.columnDef.cell, cell.getContext())}</TableCell>
            ))}
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}

/** Picks `preferred` if the gate actually offers it, otherwise falls back to
 *  whichever shift the gate does offer — so the Ca dropdown never lands on a
 *  combination that isn't in the config. */
function pickValidShift(shiftHours: ShiftHours, gate: string, preferred: ShiftType): ShiftType {
  if (shiftHours[gate]?.[preferred] !== undefined) return preferred
  return (['sang', 'dem'] as const).find((candidate) => shiftHours[gate]?.[candidate] !== undefined) ?? preferred
}

function EditableCell({
  value,
  shiftHours,
  edited,
  approved,
  onSave,
}: {
  value: CellAssignment | null
  shiftHours: ShiftHours
  edited: boolean
  approved: boolean
  onSave: (assignment: CellAssignment | null) => void
}) {
  const gates = Object.keys(shiftHours).sort()
  const [open, setOpen] = useState(false)
  // Default an "Off" cell (no gate yet) to the first configured gate rather
  // than '' — a <select> with no option matching its value falls back to
  // displaying the first <option> anyway, so leaving state at '' meant the
  // dropdown *looked* like a gate was picked while `shifts` (derived from the
  // real, empty `gate` state) stayed empty and Ca couldn't be chosen until
  // the user picked a genuinely different gate to trigger the onChange reset.
  const [gate, setGate] = useState(() => value?.gate ?? gates[0] ?? '')
  const [shift, setShift] = useState<ShiftType>(() => pickValidShift(shiftHours, value?.gate ?? gates[0] ?? '', value?.shift ?? 'sang'))
  const shifts = (['sang', 'dem'] as const).filter((candidate) => shiftHours[gate]?.[candidate] !== undefined)
  const hours = value ? shiftHours[value.gate]?.[value.shift] : undefined

  function handleOpenChange(next: boolean) {
    if (next) {
      const nextGate = value?.gate ?? gates[0] ?? ''
      setGate(nextGate)
      setShift(pickValidShift(shiftHours, nextGate, value?.shift ?? 'sang'))
    }
    setOpen(next)
  }

  function handleSave() {
    if (shiftHours[gate]?.[shift] === undefined) {
      toast.error('Chọn một cổng và ca có trong cấu hình trước khi lưu')
      return
    }
    onSave({ gate, shift })
    setOpen(false)
  }

  function handleOff() {
    onSave(null)
    setOpen(false)
  }

  return (
    <Popover open={open} onOpenChange={handleOpenChange}>
      <PopoverTrigger asChild>
        <button
          type="button"
          aria-label={
            (value ? `${value.gate}, ca ${value.shift === 'dem' ? 'đêm' : 'sáng'}, ${hours ?? 'chưa có'} giờ` : 'Off') +
            (approved && !edited ? ', đã duyệt' : '')
          }
          className={cn(
            'relative rounded-md ring-offset-1 transition-shadow hover:shadow-sm',
            edited && 'ring-2 ring-status-edited ring-offset-background',
          )}
        >
          {value ? (
            <Badge
              className={cn(
                'border-transparent',
                value.shift === 'dem'
                  ? 'bg-status-dem text-status-dem-foreground'
                  : 'bg-status-sang text-status-sang-foreground',
              )}
            >
              {value.gate}-{hours ?? '?'}
            </Badge>
          ) : (
            <span className="px-1 text-xs text-muted-foreground">OFF</span>
          )}
          {/* Suppressed while `edited`: a staged-but-unsaved edit is about to
              override whatever was approved, so showing both at once would
              claim a confirmed state that's no longer true. */}
          {approved && !edited && (
            <CircleCheckIcon className="absolute -top-1.5 -right-1.5 size-3.5 rounded-full bg-background text-status-approved" />
          )}
        </button>
      </PopoverTrigger>
      <PopoverContent className="w-56">
        <div className="flex flex-col gap-2">
          <div className="grid gap-1.5">
            <label className="text-xs font-medium text-muted-foreground">Cổng</label>
            <select
              value={gate}
              onChange={(e) => {
                const nextGate = e.target.value
                setGate(nextGate)
                if (shiftHours[nextGate]?.[shift] === undefined) {
                  setShift((['sang', 'dem'] as const).find((candidate) => shiftHours[nextGate]?.[candidate] !== undefined) ?? 'sang')
                }
              }}
              className="h-9 rounded-md border border-input bg-transparent px-3 text-sm"
            >
              {gate && !shiftHours[gate] && <option value={gate}>{gate} (không còn cấu hình)</option>}
              {gates.map((gateCode) => <option key={gateCode} value={gateCode}>{gateCode}</option>)}
            </select>
          </div>
          <div className="grid gap-1.5">
            <label className="text-xs font-medium text-muted-foreground">Ca</label>
            <select
              value={shift}
              onChange={(e) => setShift(e.target.value as ShiftType)}
              className="h-9 rounded-md border border-input bg-transparent px-3 text-sm"
            >
              {shift && !shifts.includes(shift) && <option value={shift}>{shift === 'dem' ? 'Đêm' : 'Sáng'} (không còn cấu hình)</option>}
              {shifts.map((shiftType) => <option key={shiftType} value={shiftType}>{shiftType === 'dem' ? 'Đêm' : 'Sáng'}</option>)}
            </select>
          </div>
          <div className="flex justify-between gap-2 pt-1">
            <Button type="button" variant="outline" size="sm" onClick={handleOff}>
              Off
            </Button>
            <Button type="button" size="sm" onClick={handleSave} disabled={shiftHours[gate]?.[shift] === undefined}>
              Lưu
            </Button>
          </div>
        </div>
      </PopoverContent>
    </Popover>
  )
}
