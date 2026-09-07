import { useMemo, useState } from 'react'
import { createColumnHelper, flexRender, getCoreRowModel, useReactTable } from '@tanstack/react-table'
import { toast } from 'sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { dateRange, formatShortDate } from '@/lib/dates'
import { cn } from '@/lib/utils'
import type { Employee, ScheduleEntry, ShiftType } from '@/api/types'

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

interface ScheduleRow {
  employeeId: string
  name: string
  role: string
  cells: Record<string, CellAssignment>
}

interface ScheduleTableProps {
  employees: Employee[]
  schedule: ScheduleEntry[]
  startDate: string
  numDays: number
  overrides: CellOverrides
  onEditCell: (employeeId: string, date: string, assignment: CellAssignment | null) => void
}

const columnHelper = createColumnHelper<ScheduleRow>()

/** Pivot: rows = roster employees, columns = each date in the horizon, cell = "Gate·shift" or OFF.
 *  Every cell is click-to-edit: a manager can override the solver's assignment (different gate/shift,
 *  or Off), tracked in `overrides` and highlighted until the edited schedule is approved. */
export function ScheduleTable({ employees, schedule, startDate, numDays, overrides, onEditCell }: ScheduleTableProps) {
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
            <span className="text-xs text-muted-foreground">{info.row.original.role}</span>
          </div>
        ),
      }),
      ...dates.map((date) =>
        columnHelper.display({
          id: date,
          header: formatShortDate(date),
          cell: (info) => {
            const employeeId = info.row.original.employeeId
            const original = info.row.original.cells[date] ?? null
            const effective = resolveCell(overrides, employeeId, date, original)
            return (
              <EditableCell
                value={effective}
                edited={overrides.has(overrideKey(employeeId, date))}
                onSave={(assignment) => onEditCell(employeeId, date, assignment)}
              />
            )
          },
        }),
      ),
    ],
    [dates, overrides, onEditCell],
  )

  const table = useReactTable({ data: rows, columns, getCoreRowModel: getCoreRowModel() })

  return (
    <Table>
      <TableHeader>
        {table.getHeaderGroups().map((headerGroup) => (
          <TableRow key={headerGroup.id}>
            {headerGroup.headers.map((header) => (
              <TableHead key={header.id}>{flexRender(header.column.columnDef.header, header.getContext())}</TableHead>
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

function EditableCell({
  value,
  edited,
  onSave,
}: {
  value: CellAssignment | null
  edited: boolean
  onSave: (assignment: CellAssignment | null) => void
}) {
  const [open, setOpen] = useState(false)
  const [gate, setGate] = useState(value?.gate ?? '')
  const [shift, setShift] = useState<ShiftType>(value?.shift ?? 'sang')

  function handleOpenChange(next: boolean) {
    if (next) {
      setGate(value?.gate ?? '')
      setShift(value?.shift ?? 'sang')
    }
    setOpen(next)
  }

  function handleSave() {
    if (!gate.trim()) {
      toast.error('Cần nhập mã cổng trước khi lưu')
      return
    }
    onSave({ gate: gate.trim().toUpperCase(), shift })
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
          className={cn(
            'rounded-md ring-offset-1 transition-shadow hover:shadow-sm',
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
              {value.gate}·{value.shift === 'dem' ? 'đêm' : 'sáng'}
            </Badge>
          ) : (
            <span className="px-1 text-xs text-muted-foreground">OFF</span>
          )}
        </button>
      </PopoverTrigger>
      <PopoverContent className="w-56">
        <div className="flex flex-col gap-2">
          <div className="grid gap-1.5">
            <label className="text-xs font-medium text-muted-foreground">Cổng</label>
            <Input value={gate} onChange={(e) => setGate(e.target.value.toUpperCase())} placeholder="vd: A" />
          </div>
          <div className="grid gap-1.5">
            <label className="text-xs font-medium text-muted-foreground">Ca</label>
            <select
              value={shift}
              onChange={(e) => setShift(e.target.value as ShiftType)}
              className="h-9 rounded-md border border-input bg-transparent px-3 text-sm"
            >
              <option value="sang">Sáng</option>
              <option value="dem">Đêm</option>
            </select>
          </div>
          <div className="flex justify-between gap-2 pt-1">
            <Button type="button" variant="outline" size="sm" onClick={handleOff}>
              Off
            </Button>
            <Button type="button" size="sm" onClick={handleSave}>
              Lưu
            </Button>
          </div>
        </div>
      </PopoverContent>
    </Popover>
  )
}
