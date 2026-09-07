import { type FormEvent, useState } from 'react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useCapacityCheck } from '@/api/hooks'
import { errorMessage } from '@/lib/errors'

const MIN_DAYS = 1
const MAX_DAYS = 180

function clampNumDays(raw: string): number {
  const n = Math.floor(Number(raw))
  if (!Number.isFinite(n)) return MIN_DAYS
  return Math.min(MAX_DAYS, Math.max(MIN_DAYS, n))
}

/** Keeps "" (meaning "use the current roster size") but rejects negative numbers. */
function clampEmployeeCount(raw: string): string {
  if (raw === '') return ''
  const n = Math.floor(Number(raw))
  if (!Number.isFinite(n) || n < 0) return '0'
  return String(n)
}

export function CapacityCheckPanel() {
  const [numDays, setNumDays] = useState(28)
  const [employeeCount, setEmployeeCount] = useState('')
  const check = useCapacityCheck()

  function handleCheck(e: FormEvent) {
    e.preventDefault()
    check.mutate(
      { num_days: numDays, employee_count: employeeCount === '' ? undefined : Number(employeeCount) },
      { onError: (err) => toast.error(errorMessage(err)) },
    )
  }

  const result = check.data

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle>Capacity check</CardTitle>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleCheck} className="flex flex-wrap items-end gap-4">
            <div className="grid gap-1.5">
              <Label htmlFor="cap-num-days">Số ngày</Label>
              <Input
                id="cap-num-days"
                type="number"
                min={MIN_DAYS}
                max={MAX_DAYS}
                value={numDays}
                onChange={(e) => setNumDays(clampNumDays(e.target.value))}
                className="w-24"
              />
            </div>
            <div className="grid gap-1.5">
              <Label htmlFor="cap-employee-count">Số NV (để trống = dùng số hiện có)</Label>
              <Input
                id="cap-employee-count"
                type="number"
                min={0}
                placeholder="21"
                value={employeeCount}
                onChange={(e) => setEmployeeCount(clampEmployeeCount(e.target.value))}
                className="w-48"
              />
            </div>
            <Button type="submit" disabled={check.isPending}>
              {check.isPending ? 'Đang kiểm tra...' : 'Check'}
            </Button>
          </form>
        </CardContent>
      </Card>

      {result && (
        <Card>
          <CardHeader>
            <CardTitle className={result.is_sufficient ? '' : 'text-destructive'}>
              {result.is_sufficient ? 'Đủ nhân lực' : 'Thiếu nhân lực'}
            </CardTitle>
          </CardHeader>
          <CardContent className="grid grid-cols-2 gap-4 sm:grid-cols-3">
            <Stat label="Giờ-người/ngày cần" value={result.demand_hours_per_day} />
            <Stat label="Tổng giờ-người cần" value={result.demand_hours_total} />
            <Stat label="Giờ mục tiêu/NV" value={result.target_hours_per_employee} />
            <Stat label="Số NV cần tối thiểu" value={result.min_employees_required.toFixed(2)} />
            <Stat label="Số NV hiện có" value={result.employee_count} />
            <Stat label="Tỷ lệ thiếu" value={`${(result.shortfall_ratio * 100).toFixed(1)}%`} />
          </CardContent>
        </Card>
      )}
    </div>
  )
}

function Stat({ label, value }: { label: string; value: string | number }) {
  return (
    <div className="rounded-md border p-3">
      <p className="text-xs text-muted-foreground">{label}</p>
      <p className="text-lg font-semibold">{value}</p>
    </div>
  )
}
