import { Badge } from '@/components/ui/badge';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { formatShortDate } from '@/lib/dates';
import type { ShortageItem } from '@/api/types';

export function ShortagesTable({ shortages }: { shortages: ShortageItem[] }) {
  if (shortages.length === 0) {
    return <p className="text-sm text-muted-foreground">Không thiếu ca nào cho kỳ này.</p>;
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
              {/* Both shortage types leave the gate unable to operate — a
                  missing rank-and-file NV is just as critical as a missing
                  lead, so both render as destructive, not just "lead". */}
              <Badge variant="destructive">
                {s.shortage_type === 'lead' ? 'Thiếu lead' : 'Thiếu NV'}
              </Badge>
            </TableCell>
            <TableCell className="text-right font-semibold text-destructive">{s.missing}</TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}
