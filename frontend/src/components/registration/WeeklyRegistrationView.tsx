import { useMemo, useState } from 'react';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { useEmployees, useSetAvailability, useSetLeaveDay } from '@/api/hooks';
import { errorMessage } from '@/lib/errors';
import { addWeeks, dateRange, formatShortDate, startOfWeek, todayISO } from '@/lib/dates';
import { cn } from '@/lib/utils';
import { RegistrationTable } from './RegistrationTable';
import { cellKey, STATUS_META, type DayStatus } from './registrationStatus';

export function WeeklyRegistrationView() {
  const [weekStart, setWeekStart] = useState(() => startOfWeek(todayISO()));
  const [search, setSearch] = useState('');
  const [pendingKeys, setPendingKeys] = useState<Set<string>>(new Set());

  const days = useMemo(() => dateRange(weekStart, 7), [weekStart]);
  // Scope the fetch to the visible week, not the backend's today-anchored
  // default — otherwise days before today (e.g. Monday of the current week
  // when today isn't Monday) or any past week would never show what was
  // just written.
  const employeesQuery = useEmployees(days[0], days[6]);
  const setAvailability = useSetAvailability();
  const setLeaveDay = useSetLeaveDay();

  const employees = employeesQuery.data?.employees ?? [];
  const availability = employeesQuery.data?.availability ?? {};

  const filteredEmployees = useMemo(() => {
    const q = search.trim().toLowerCase();
    if (!q) return employees;
    return employees.filter(
      (e) => e.name.toLowerCase().includes(q) || e.employee_id.toLowerCase().includes(q),
    );
  }, [employees, search]);

  async function applyStatus(employeeId: string, date: string, status: DayStatus) {
    const key = cellKey(employeeId, date);
    setPendingKeys((prev) => new Set(prev).add(key));
    try {
      if (status === 'leave') {
        await setLeaveDay.mutateAsync({ employeeId, params: { date, on_leave: true } });
        await setAvailability.mutateAsync({
          employeeId,
          params: { date, sang: false, dem: false },
        });
      } else {
        await setLeaveDay.mutateAsync({ employeeId, params: { date, on_leave: false } });
        await setAvailability.mutateAsync({
          employeeId,
          params: {
            date,
            sang: status === 'sang' || status === 'both',
            dem: status === 'dem' || status === 'both',
          },
        });
      }
      toast.success(`Đã đăng ký ${STATUS_META[status].label.toLowerCase()} — ${date}`);
    } catch (err) {
      toast.error(errorMessage(err));
    } finally {
      setPendingKeys((prev) => {
        const next = new Set(prev);
        next.delete(key);
        return next;
      });
    }
  }

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle>Đăng ký lịch tuần</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex flex-wrap items-end gap-4">
            <div className="grid gap-1.5">
              <label htmlFor="registration-search" className="text-sm font-medium">
                Tìm nhân viên
              </label>
              <Input
                id="registration-search"
                placeholder="Tên hoặc mã nhân viên..."
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                className="w-56"
              />
            </div>

            <div className="flex items-center gap-2">
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={() => setWeekStart((w) => addWeeks(w, -1))}
              >
                ← Tuần trước
              </Button>
              <span className="text-sm font-medium">
                Tuần {formatShortDate(days[0])} – {formatShortDate(days[6])}
              </span>
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={() => setWeekStart((w) => addWeeks(w, 1))}
              >
                Tuần sau →
              </Button>
            </div>
          </div>

          <div className="flex flex-wrap gap-3 border-t pt-3">
            {(Object.keys(STATUS_META) as DayStatus[]).map((status) => (
              <span
                key={status}
                className="flex items-center gap-1.5 text-xs text-muted-foreground"
              >
                <span
                  className={cn('inline-block size-3 rounded-full', STATUS_META[status].className)}
                />
                {STATUS_META[status].label}
              </span>
            ))}
          </div>
        </CardContent>
      </Card>

      {employeesQuery.isLoading && (
        <p className="text-sm text-muted-foreground">Đang tải danh sách nhân viên...</p>
      )}
      {employeesQuery.isError && (
        <p className="text-sm text-destructive">
          Không tải được nhân viên: {errorMessage(employeesQuery.error)}
        </p>
      )}

      {filteredEmployees.length > 0 && (
        <Card>
          <CardContent className="pt-6">
            <RegistrationTable
              employees={filteredEmployees}
              availability={availability}
              days={days}
              pendingKeys={pendingKeys}
              onSelectStatus={applyStatus}
            />
          </CardContent>
        </Card>
      )}

      {!employeesQuery.isLoading && filteredEmployees.length === 0 && (
        <p className="text-sm text-muted-foreground">
          Không tìm thấy nhân viên nào khớp "{search}".
        </p>
      )}
    </div>
  );
}
