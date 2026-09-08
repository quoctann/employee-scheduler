import { useState, type FormEvent } from 'react'
import { toast } from 'sonner'
import { useConfig, useRenameGate, useUpdateGateShiftRequirement } from '@/api/hooks'
import type { ShiftType } from '@/api/types'
import { errorMessage } from '@/lib/errors'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

const shiftLabel: Record<ShiftType, string> = { sang: 'Sáng', dem: 'Đêm' }

interface RequirementForm {
  gate: string
  shift: ShiftType
  nv: number
  lead: number
  leadMandatoryRole: boolean
  shiftHours: number
}

export function ConfigPanel() {
  const configQuery = useConfig()
  const update = useUpdateGateShiftRequirement()
  const rename = useRenameGate()
  const [editing, setEditing] = useState<RequirementForm | null>(null)
  const [renamingGate, setRenamingGate] = useState<string | null>(null)
  const [newGateCode, setNewGateCode] = useState('')

  function openRename(gate: string) {
    setRenamingGate(gate)
    setNewGateCode(gate)
  }

  function handleRename(event: FormEvent) {
    event.preventDefault()
    if (!renamingGate) return
    const trimmed = newGateCode.trim()
    rename.mutate(
      { gateCode: renamingGate, newCode: trimmed },
      {
        onSuccess: () => {
          toast.success(`Đã đổi tên cổng ${renamingGate} thành ${trimmed}`)
          setRenamingGate(null)
        },
        onError: (error) => toast.error(errorMessage(error)),
      },
    )
  }

  function openEdit(gate: string, shift: ShiftType) {
    const config = configQuery.data
    if (!config) return
    const requirement = config.requirements[gate]?.[shift]
    setEditing({
      gate,
      shift,
      nv: requirement?.nv ?? 0,
      lead: requirement?.lead ?? 0,
      leadMandatoryRole: requirement?.lead_mandatory_role ?? false,
      shiftHours: config.shift_hours[gate]?.[shift] ?? 8,
    })
  }

  function handleSave(event: FormEvent) {
    event.preventDefault()
    if (!editing) return
    update.mutate(
      {
        gateCode: editing.gate,
        shiftType: editing.shift,
        params: { nv: editing.nv, lead: editing.lead, lead_mandatory_role: editing.leadMandatoryRole, shift_hours: editing.shiftHours },
      },
      {
        onSuccess: () => {
          toast.success(`Đã cập nhật cấu hình cổng ${editing.gate} - ca ${shiftLabel[editing.shift]}`)
          setEditing(null)
        },
        onError: (error) => toast.error(errorMessage(error)),
      },
    )
  }

  const config = configQuery.data
  const gates = config ? Object.keys(config.requirements).sort() : []
  const leadGates = new Set(config?.lead_gates ?? [])

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle>Đổi tên cổng</CardTitle>
          <p className="mt-1 text-sm text-muted-foreground">
            Mã cổng mới sẽ áp dụng cho toàn bộ cấu hình, lịch đã duyệt và lịch sử xếp lịch của cổng này.
          </p>
        </CardHeader>
        <CardContent>
          {configQuery.isLoading && <p className="text-sm text-muted-foreground">Đang tải cấu hình...</p>}
          {configQuery.isError && <p className="text-sm text-destructive">Không tải được cấu hình: {errorMessage(configQuery.error)}</p>}
          {config && (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Cổng</TableHead>
                  <TableHead className="text-right">Thao tác</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {gates.map((gate) => (
                  <TableRow key={gate}>
                    <TableCell className="font-medium">
                      {gate}
                      {leadGates.has(gate) && (
                        <Badge variant="outline" className="ml-2">
                          Cổng cần trưởng ca
                        </Badge>
                      )}
                    </TableCell>
                    <TableCell className="text-right">
                      <Button type="button" size="sm" variant="outline" onClick={() => openRename(gate)}>
                        Đổi tên
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Cấu hình cổng &amp; trưởng ca</CardTitle>
          <p className="mt-1 text-sm text-muted-foreground">
            Với mỗi cổng và ca trực, hệ thống xếp lịch cần biết cần bao nhiêu nhân viên (NV) và bao nhiêu
            trưởng ca (Lead), và liệu vị trí trưởng ca đó có bắt buộc phải do nhân viên vai trò Tổ trưởng
            (TC) đảm nhiệm hay không. Sửa các giá trị bên dưới để thay đổi yêu cầu nhân sự cho lần xếp lịch
            (Solve) tiếp theo — dùng để demo/test cấu hình trước khi áp dụng cho lịch thật.
          </p>
        </CardHeader>
        <CardContent>
          {configQuery.isLoading && <p className="text-sm text-muted-foreground">Đang tải cấu hình...</p>}
          {configQuery.isError && <p className="text-sm text-destructive">Không tải được cấu hình: {errorMessage(configQuery.error)}</p>}
          {config && (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Cổng</TableHead>
                  <TableHead>Ca</TableHead>
                  <TableHead className="text-right">Số nhân viên (NV)</TableHead>
                  <TableHead className="text-right">Số trưởng ca (TC)</TableHead>
                  <TableHead>Bắt buộc vai trò TC</TableHead>
                  <TableHead className="text-right">Giờ/ca</TableHead>
                  <TableHead className="text-right">Thao tác</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {gates.map((gate) => {
                  const shiftsForGate = (['sang', 'dem'] as const).filter(
                    (shift) => config.requirements[gate]?.[shift] !== undefined,
                  )
                  return shiftsForGate.map((shift) => {
                    const requirement = config.requirements[gate][shift]!
                    return (
                      <TableRow key={`${gate}-${shift}`}>
                        <TableCell className="font-medium">
                          {gate}
                          {leadGates.has(gate) && (
                            <Badge variant="outline" className="ml-2">
                              Cổng cần trưởng ca
                            </Badge>
                          )}
                        </TableCell>
                        <TableCell>{shiftLabel[shift]}</TableCell>
                        <TableCell className="text-right">{requirement.nv}</TableCell>
                        <TableCell className="text-right">{requirement.lead}</TableCell>
                        <TableCell>
                          <Badge variant={requirement.lead_mandatory_role ? 'destructive' : 'outline'}>
                            {requirement.lead_mandatory_role ? 'Bắt buộc TC' : 'Không bắt buộc'}
                          </Badge>
                        </TableCell>
                        <TableCell className="text-right">{config.shift_hours[gate]?.[shift] ?? '—'}</TableCell>
                        <TableCell className="text-right">
                          <Button type="button" size="sm" variant="outline" onClick={() => openEdit(gate, shift)}>
                            Sửa
                          </Button>
                        </TableCell>
                      </TableRow>
                    )
                  })
                })}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      <Dialog open={editing !== null} onOpenChange={(open) => !open && setEditing(null)}>
        <DialogContent>
          {editing && (
            <form onSubmit={handleSave} className="grid gap-4">
              <DialogHeader>
                <DialogTitle>
                  Sửa yêu cầu cổng {editing.gate} - ca {shiftLabel[editing.shift]}
                </DialogTitle>
                <DialogDescription>
                  Số NV và số trưởng ca là số lượng tối thiểu cần xếp cho ca này. Bật &quot;Bắt buộc vai trò TC&quot;
                  nếu vị trí trưởng ca chỉ được giao cho nhân viên có vai trò Tổ trưởng (TC), không thể là NV
                  thường đứng thay.
                </DialogDescription>
              </DialogHeader>
              <div className="grid grid-cols-2 gap-4">
                <div className="grid gap-1.5">
                  <Label htmlFor="req-nv">Số NV</Label>
                  <Input
                    id="req-nv"
                    type="number"
                    min={0}
                    max={1000}
                    value={editing.nv}
                    onChange={(event) => setEditing({ ...editing, nv: Number(event.target.value) })}
                    required
                  />
                </div>
                <div className="grid gap-1.5">
                  <Label htmlFor="req-lead">Số trưởng ca (Lead)</Label>
                  <Input
                    id="req-lead"
                    type="number"
                    min={0}
                    max={1000}
                    value={editing.lead}
                    onChange={(event) => setEditing({ ...editing, lead: Number(event.target.value) })}
                    required
                  />
                </div>
              </div>
              <div className="grid gap-1.5">
                <Label htmlFor="req-hours">Giờ/ca</Label>
                <Input
                  id="req-hours"
                  type="number"
                  min={1}
                  max={24}
                  value={editing.shiftHours}
                  onChange={(event) => setEditing({ ...editing, shiftHours: Number(event.target.value) })}
                  required
                />
              </div>
              <label className="flex items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  checked={editing.leadMandatoryRole}
                  onChange={(event) => setEditing({ ...editing, leadMandatoryRole: event.target.checked })}
                />
                Bắt buộc vai trò TC cho vị trí trưởng ca
              </label>
              <DialogFooter>
                <Button type="button" variant="outline" onClick={() => setEditing(null)}>
                  Hủy
                </Button>
                <Button type="submit" disabled={update.isPending}>
                  {update.isPending ? 'Đang lưu...' : 'Lưu thay đổi'}
                </Button>
              </DialogFooter>
            </form>
          )}
        </DialogContent>
      </Dialog>

      <Dialog open={renamingGate !== null} onOpenChange={(open) => !open && setRenamingGate(null)}>
        <DialogContent>
          {renamingGate && (
            <form onSubmit={handleRename} className="grid gap-4">
              <DialogHeader>
                <DialogTitle>Đổi tên cổng {renamingGate}</DialogTitle>
                <DialogDescription>
                  Mã cổng mới sẽ áp dụng cho toàn bộ cấu hình, lịch đã duyệt và lịch sử xếp lịch của cổng này.
                </DialogDescription>
              </DialogHeader>
              <div className="grid gap-1.5">
                <Label htmlFor="gate-new-code">Mã cổng mới</Label>
                <Input
                  id="gate-new-code"
                  value={newGateCode}
                  onChange={(event) => setNewGateCode(event.target.value)}
                  maxLength={20}
                  required
                  autoFocus
                />
              </div>
              <DialogFooter>
                <Button type="button" variant="outline" onClick={() => setRenamingGate(null)}>
                  Hủy
                </Button>
                <Button type="submit" disabled={rename.isPending || newGateCode.trim() === ''}>
                  {rename.isPending ? 'Đang đổi tên...' : 'Đổi tên'}
                </Button>
              </DialogFooter>
            </form>
          )}
        </DialogContent>
      </Dialog>
    </div>
  )
}
