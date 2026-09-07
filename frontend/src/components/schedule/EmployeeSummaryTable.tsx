import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import type { EmployeeSummary } from '@/api/types'

export function EmployeeSummaryTable({ summary }: { summary: EmployeeSummary[] }) {
  const sorted = summary.slice().sort((a, b) => a.employee_id.localeCompare(b.employee_id))

  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>Nhân viên</TableHead>
          <TableHead>Vai trò</TableHead>
          <TableHead className="text-right">Mục tiêu (h)</TableHead>
          <TableHead className="text-right">Thực tế (h)</TableHead>
          <TableHead className="text-right">Chênh lệch (h)</TableHead>
          <TableHead className="text-right">Số ca</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {sorted.map((s) => (
          <TableRow key={s.employee_id}>
            <TableCell className="font-medium">{s.employee_id}</TableCell>
            <TableCell>{s.role}</TableCell>
            <TableCell className="text-right">{s.target_hours}</TableCell>
            <TableCell className="text-right">{s.actual_hours}</TableCell>
            <TableCell className={`text-right ${s.deviation_hours < 0 ? 'text-destructive' : ''}`}>
              {s.deviation_hours > 0 ? '+' : ''}
              {s.deviation_hours}
            </TableCell>
            <TableCell className="text-right">{s.actual_shifts}</TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}
