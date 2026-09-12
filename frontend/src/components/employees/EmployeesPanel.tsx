import { useState, type FormEvent } from 'react';
import { toast } from 'sonner';
import { ApiError } from '@/api/client';
import {
  useAllEmployees,
  useCreateEmployee,
  useDeactivateEmployee,
  useRestoreEmployee,
  useUpdateEmployee,
} from '@/api/hooks';
import type { Employee, Role } from '@/api/types';
import { errorMessage } from '@/lib/errors';
import { RoleBadge } from '@/components/RoleBadge';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';

const roles: Role[] = ['NV', 'TC', 'PC'];

interface EmployeeForm {
  employeeID: string;
  name: string;
  role: Role;
}

const emptyForm: EmployeeForm = { employeeID: '', name: '', role: 'NV' };

export function EmployeesPanel() {
  const employeesQuery = useAllEmployees();
  const create = useCreateEmployee();
  const update = useUpdateEmployee();
  const deactivate = useDeactivateEmployee();
  const restore = useRestoreEmployee();
  const [createOpen, setCreateOpen] = useState(false);
  const [editEmployee, setEditEmployee] = useState<Employee | null>(null);
  const [form, setForm] = useState<EmployeeForm>(emptyForm);

  function openCreate() {
    setForm(emptyForm);
    setCreateOpen(true);
  }

  function openEdit(employee: Employee) {
    setForm({ employeeID: employee.employee_id, name: employee.name, role: employee.role });
    setEditEmployee(employee);
  }

  function handleCreate(event: FormEvent) {
    event.preventDefault();
    create.mutate(
      { employee_id: form.employeeID, name: form.name, role: form.role },
      {
        onSuccess: () => {
          toast.success('Đã thêm nhân viên');
          setCreateOpen(false);
        },
        onError: (error) => {
          toast.error(
            error instanceof ApiError && error.status === 409
              ? 'Mã nhân viên đã tồn tại. Hãy khôi phục bản ghi cũ nếu cần.'
              : errorMessage(error),
          );
        },
      },
    );
  }

  function handleUpdate(event: FormEvent) {
    event.preventDefault();
    if (!editEmployee) return;
    update.mutate(
      { employeeId: editEmployee.employee_id, params: { name: form.name, role: form.role } },
      {
        onSuccess: () => {
          toast.success('Đã cập nhật nhân viên');
          setEditEmployee(null);
        },
        onError: (error) => toast.error(errorMessage(error)),
      },
    );
  }

  function handleDeactivate(employee: Employee) {
    if (
      !window.confirm(
        `Ngưng hoạt động ${employee.name} (${employee.employee_id})? Lịch sử và đăng ký vẫn được giữ lại.`,
      )
    )
      return;
    deactivate.mutate(employee.employee_id, {
      onSuccess: () => toast.success(`Đã ngưng hoạt động ${employee.employee_id}`),
      onError: (error) => toast.error(errorMessage(error)),
    });
  }

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader className="flex-row items-center justify-between gap-4">
          <div>
            <CardTitle>Quản lý nhân viên</CardTitle>
            <p className="mt-1 text-sm text-muted-foreground">
              Thêm, sửa, ngưng hoạt động hoặc khôi phục dữ liệu demo.
            </p>
          </div>
          <Button type="button" onClick={openCreate}>
            Thêm nhân viên
          </Button>
        </CardHeader>
        <CardContent>
          {employeesQuery.isLoading && (
            <p className="text-sm text-muted-foreground">Đang tải nhân viên...</p>
          )}
          {employeesQuery.isError && (
            <p className="text-sm text-destructive">
              Không tải được nhân viên: {errorMessage(employeesQuery.error)}
            </p>
          )}
          {employeesQuery.data && (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Mã</TableHead>
                  <TableHead>Họ tên</TableHead>
                  <TableHead>Vai trò</TableHead>
                  <TableHead>Trạng thái</TableHead>
                  <TableHead className="text-right">Thao tác</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {employeesQuery.data.employees.map((employee) => (
                  <TableRow
                    key={employee.employee_id}
                    className={!employee.active ? 'opacity-60' : undefined}
                  >
                    <TableCell className="font-medium">{employee.employee_id}</TableCell>
                    <TableCell>{employee.name}</TableCell>
                    <TableCell>
                      <RoleBadge role={employee.role} />
                    </TableCell>
                    <TableCell>
                      <Badge variant={employee.active ? 'outline' : 'secondary'}>
                        {employee.active ? 'Đang hoạt động' : 'Đã ngưng'}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-right">
                      <div className="flex justify-end gap-2">
                        <Button
                          type="button"
                          size="sm"
                          variant="outline"
                          onClick={() => openEdit(employee)}
                        >
                          Sửa
                        </Button>
                        {employee.active ? (
                          <Button
                            type="button"
                            size="sm"
                            variant="destructive"
                            disabled={deactivate.isPending}
                            onClick={() => handleDeactivate(employee)}
                          >
                            Ngưng hoạt động
                          </Button>
                        ) : (
                          <Button
                            type="button"
                            size="sm"
                            disabled={restore.isPending}
                            onClick={() =>
                              restore.mutate(employee.employee_id, {
                                onSuccess: () =>
                                  toast.success(`Đã khôi phục ${employee.employee_id}`),
                                onError: (error) => toast.error(errorMessage(error)),
                              })
                            }
                          >
                            Khôi phục
                          </Button>
                        )}
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent>
          <form onSubmit={handleCreate} className="grid gap-4">
            <DialogHeader>
              <DialogTitle>Thêm nhân viên</DialogTitle>
              <DialogDescription>Mã nhân viên là cố định sau khi tạo.</DialogDescription>
            </DialogHeader>
            <EmployeeFormFields form={form} onChange={setForm} />
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setCreateOpen(false)}>
                Hủy
              </Button>
              <Button type="submit" disabled={create.isPending}>
                {create.isPending ? 'Đang thêm...' : 'Thêm'}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <Dialog open={editEmployee !== null} onOpenChange={(open) => !open && setEditEmployee(null)}>
        <DialogContent>
          <form onSubmit={handleUpdate} className="grid gap-4">
            <DialogHeader>
              <DialogTitle>Sửa nhân viên</DialogTitle>
              <DialogDescription>Mã nhân viên không thể thay đổi.</DialogDescription>
            </DialogHeader>
            <EmployeeFormFields form={form} onChange={setForm} employeeIDReadOnly />
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setEditEmployee(null)}>
                Hủy
              </Button>
              <Button type="submit" disabled={update.isPending}>
                {update.isPending ? 'Đang lưu...' : 'Lưu thay đổi'}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </div>
  );
}

function EmployeeFormFields({
  form,
  onChange,
  employeeIDReadOnly = false,
}: {
  form: EmployeeForm;
  onChange: (form: EmployeeForm) => void;
  employeeIDReadOnly?: boolean;
}) {
  return (
    <div className="grid gap-4">
      <div className="grid gap-1.5">
        <Label htmlFor="employee-id">Mã nhân viên</Label>
        <Input
          id="employee-id"
          value={form.employeeID}
          readOnly={employeeIDReadOnly}
          onChange={(event) => onChange({ ...form, employeeID: event.target.value.toUpperCase() })}
          required
        />
      </div>
      <div className="grid gap-1.5">
        <Label htmlFor="employee-name">Họ tên</Label>
        <Input
          id="employee-name"
          value={form.name}
          onChange={(event) => onChange({ ...form, name: event.target.value })}
          required
        />
      </div>
      <div className="grid gap-1.5">
        <Label htmlFor="employee-role">Vai trò</Label>
        <select
          id="employee-role"
          value={form.role}
          onChange={(event) => onChange({ ...form, role: event.target.value as Role })}
          className="h-9 rounded-md border border-input bg-transparent px-3 text-sm"
        >
          {roles.map((role) => (
            <option key={role} value={role}>
              {role}
            </option>
          ))}
        </select>
      </div>
    </div>
  );
}
