import { type FormEvent, useState } from 'react'
import { toast } from 'sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { useCandidates } from '@/api/hooks'
import { errorMessage } from '@/lib/errors'
import { todayISO } from '@/lib/dates'
import type { ShiftType } from '@/api/types'

export function CandidatesPanel() {
  const [date, setDate] = useState(todayISO())
  const [gate, setGate] = useState('A')
  const [shift, setShift] = useState<ShiftType>('sang')
  const [requiresLead, setRequiresLead] = useState(false)
  const [excludedEmployeeId, setExcludedEmployeeId] = useState('')

  const candidates = useCandidates()

  function handleSearch(e: FormEvent) {
    e.preventDefault()
    candidates.mutate(
      {
        target_slot: { date, gate, shift, requires_lead: requiresLead },
        excluded_employee_id: excludedEmployeeId || undefined,
        top_n: 5,
      },
      { onError: (err) => toast.error(errorMessage(err)) },
    )
  }

  const result = candidates.data

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle>Tìm người thay thế</CardTitle>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSearch} className="flex flex-wrap items-end gap-4">
            <div className="grid gap-1.5">
              <Label htmlFor="cand-date">Ngày</Label>
              <Input id="cand-date" type="date" value={date} onChange={(e) => setDate(e.target.value)} className="w-40" />
            </div>
            <div className="grid gap-1.5">
              <Label htmlFor="cand-gate">Cổng</Label>
              <Input
                id="cand-gate"
                value={gate}
                onChange={(e) => setGate(e.target.value.toUpperCase())}
                className="w-20"
              />
            </div>
            <div className="grid gap-1.5">
              <Label htmlFor="cand-shift">Ca</Label>
              <select
                id="cand-shift"
                value={shift}
                onChange={(e) => setShift(e.target.value as ShiftType)}
                className="h-9 rounded-md border border-input bg-transparent px-3 text-sm"
              >
                <option value="sang">Sáng</option>
                <option value="dem">Đêm</option>
              </select>
            </div>
            <div className="grid gap-1.5">
              <Label htmlFor="cand-excluded">Loại trừ NV (id)</Label>
              <Input
                id="cand-excluded"
                value={excludedEmployeeId}
                onChange={(e) => setExcludedEmployeeId(e.target.value)}
                placeholder="ví dụ: NV08"
                className="w-36"
              />
            </div>
            <label className="flex items-center gap-2 text-sm">
              <input type="checkbox" checked={requiresLead} onChange={(e) => setRequiresLead(e.target.checked)} />
              Cần lead
            </label>
            <Button type="submit" disabled={candidates.isPending}>
              {candidates.isPending ? 'Đang tìm...' : 'Tìm ứng viên'}
            </Button>
          </form>
        </CardContent>
      </Card>

      {result && (
        <Card>
          <CardHeader>
            <CardTitle>
              Ứng viên cho {result.target_slot.gate}-{result.target_slot.shift} ngày {result.target_slot.date}
              {result.excluded_count > 0 && (
                <span className="ml-2 text-sm font-normal text-muted-foreground">
                  (đã loại {result.excluded_count} người không đủ điều kiện)
                </span>
              )}
            </CardTitle>
          </CardHeader>
          <CardContent>
            {result.candidates.length === 0 ? (
              <p className="text-sm text-muted-foreground">Không có ứng viên phù hợp.</p>
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Nhân viên</TableHead>
                    <TableHead>Vai trò</TableHead>
                    <TableHead className="text-right">Điểm</TableHead>
                    <TableHead className="text-right">Chênh lệch giờ</TableHead>
                    <TableHead>Lý do</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {result.candidates.map((c) => (
                    <TableRow key={c.employee_id}>
                      <TableCell className="font-medium">{c.employee_id}</TableCell>
                      <TableCell>{c.role}</TableCell>
                      <TableCell className="text-right">{c.score.toFixed(2)}</TableCell>
                      <TableCell className="text-right">{c.deviation_hours}</TableCell>
                      <TableCell>
                        <div className="flex flex-wrap gap-1">
                          {c.reasons.map((reason, i) => (
                            <Badge key={`${c.employee_id}-reason-${i}`} variant="outline">
                              {reason}
                            </Badge>
                          ))}
                          {c.would_create_streak && <Badge variant="destructive">tạo chuỗi ca liên tiếp</Badge>}
                        </div>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
          </CardContent>
        </Card>
      )}
    </div>
  )
}
