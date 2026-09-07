import { Badge } from '@/components/ui/badge'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { formatShortDate } from '@/lib/dates'
import type { ShortageItem } from '@/api/types'

export function ShortagesTable({ shortages }: { shortages: ShortageItem[] }) {
  if (shortages.length === 0) {
    return <p className="text-sm text-muted-foreground">Không thiếu ca nào cho kỳ này.</p>
  }

  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>Ngày</TableHead>
          <TableHead>Cổng</TableHead>
          <TableHead>Ca</TableHead>
          <TableHead>Loại</TableHead>
          <TableHead className="text-right">Thiếu</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {shortages.map((s, i) => (
          <TableRow key={`${s.date}-${s.gate}-${s.shift}-${s.shortage_type}-${i}`}>
            <TableCell>{formatShortDate(s.date)}</TableCell>
            <TableCell>{s.gate}</TableCell>
            <TableCell>{s.shift === 'dem' ? 'Đêm' : 'Sáng'}</TableCell>
            <TableCell>
              <Badge variant={s.shortage_type === 'lead' ? 'destructive' : 'outline'}>
                {s.shortage_type === 'lead' ? 'Thiếu lead' : 'Thiếu NV'}
              </Badge>
            </TableCell>
            <TableCell className="text-right font-medium">{s.missing}</TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}
