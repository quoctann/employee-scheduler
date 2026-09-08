import { useMemo, useState } from 'react'
import { createColumnHelper, flexRender, getCoreRowModel, useReactTable } from '@tanstack/react-table'
import { RoleBadge } from '@/components/RoleBadge'
import { Button } from '@/components/ui/button'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { formatWeekdayDate } from '@/lib/dates'
import { cn } from '@/lib/utils'
import type { AvailabilityMap, Employee } from '@/api/types'
import { cellKey, computeStatus, SELECTABLE_STATUSES, STATUS_META, type DayStatus } from './registrationStatus'

interface RegistrationRow {
  employee: Employee
}

interface RegistrationTableProps {
  employees: Employee[]
  availability: AvailabilityMap
  days: string[]
  pendingKeys: Set<string>
  onSelectStatus: (employeeId: string, date: string, status: DayStatus) => void
}

const columnHelper = createColumnHelper<RegistrationRow>()

/** Pivot: rows = every employee, columns = each day of the visible week, cell = click-to-edit
 *  registration status. Replaces the old single-employee-at-a-time picker so a manager can see
 *  and edit everyone's registration for the week at a glance. */
export function RegistrationTable({ employees, availability, days, pendingKeys, onSelectStatus }: RegistrationTableProps) {
  const rows = useMemo<RegistrationRow[]>(
    () =>
      employees
        .slice()
        .sort((a, b) => a.employee_id.localeCompare(b.employee_id))
        .map((employee) => ({ employee })),
    [employees],
  )

  const columns = useMemo(
    () => [
      columnHelper.display({
        id: 'employee',
        header: 'Nhân viên',
        cell: (info) => {
          const { employee } = info.row.original
          return (
            <div className="flex flex-col">
              <span className="font-medium">{employee.name}</span>
              <span className="text-xs text-muted-foreground">
                {employee.employee_id} · <RoleBadge role={employee.role} />
              </span>
            </div>
          )
        },
      }),
      ...days.map((date) =>
        columnHelper.display({
          id: date,
          header: formatWeekdayDate(date),
          cell: (info) => {
            const { employee } = info.row.original
            const status = computeStatus(employee, availability, date)
            return (
              <RegistrationCell
                status={status}
                pending={pendingKeys.has(cellKey(employee.employee_id, date))}
                onSelect={(next) => onSelectStatus(employee.employee_id, date, next)}
              />
            )
          },
        }),
      ),
    ],
    [days, availability, pendingKeys, onSelectStatus],
  )

  const table = useReactTable({ data: rows, columns, getCoreRowModel: getCoreRowModel() })

  return (
    <Table>
      <TableHeader>
        {table.getHeaderGroups().map((headerGroup) => (
          <TableRow key={headerGroup.id}>
            {headerGroup.headers.map((header, index) => (
              <TableHead
                key={header.id}
                className={cn(index === 0 && 'sticky left-0 z-10 bg-background')}
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
            {row.getVisibleCells().map((cell, index) => (
              <TableCell key={cell.id} className={cn(index === 0 && 'sticky left-0 z-10 bg-background')}>
                {flexRender(cell.column.columnDef.cell, cell.getContext())}
              </TableCell>
            ))}
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}

function RegistrationCell({
  status,
  pending,
  onSelect,
}: {
  status: DayStatus
  pending: boolean
  onSelect: (status: DayStatus) => void
}) {
  const [open, setOpen] = useState(false)
  const meta = STATUS_META[status]

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          type="button"
          disabled={pending}
          className={cn(
            'flex h-8 w-16 items-center justify-center rounded-lg text-xs font-semibold shadow-sm transition-transform hover:-translate-y-0.5 disabled:opacity-60',
            meta.className,
          )}
        >
          {meta.shortLabel}
        </button>
      </PopoverTrigger>
      <PopoverContent className="w-48">
        <div className="flex flex-col gap-1">
          {SELECTABLE_STATUSES.map((s) => (
            <Button
              key={s}
              type="button"
              variant={status === s ? 'default' : 'outline'}
              size="sm"
              className="justify-start"
              disabled={pending}
              onClick={() => {
                // Close immediately so a second click can't fire an
                // overlapping request against the same cell while the
                // first PUT is still in flight.
                setOpen(false)
                onSelect(s)
              }}
            >
              {STATUS_META[s].label}
            </Button>
          ))}
        </div>
      </PopoverContent>
    </Popover>
  )
}
