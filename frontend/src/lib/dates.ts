/** Inclusive range of "YYYY-MM-DD" strings, `numDays` long, starting at `startDate`. */
export function dateRange(startDate: string, numDays: number): string[] {
  if (!Number.isFinite(numDays) || numDays <= 0) {
    return [];
  }
  const start = new Date(`${startDate}T00:00:00Z`);
  return Array.from({ length: Math.floor(numDays) }, (_, i) => {
    const d = new Date(start);
    d.setUTCDate(d.getUTCDate() + i);
    return d.toISOString().slice(0, 10);
  });
}

/** "2026-09-07" -> "07/09" for compact table headers. */
export function formatShortDate(isoDate: string): string {
  const [, month, day] = isoDate.split('-');
  return `${day}/${month}`;
}

/** Today as "YYYY-MM-DD" in the local timezone (for default form values). */
export function todayISO(): string {
  const now = new Date();
  const offsetMs = now.getTimezoneOffset() * 60_000;
  return new Date(now.getTime() - offsetMs).toISOString().slice(0, 10);
}

/** The Monday ("YYYY-MM-DD") of the week containing `isoDate`. */
export function startOfWeek(isoDate: string): string {
  const d = new Date(`${isoDate}T00:00:00Z`);
  const dayIndex = (d.getUTCDay() + 6) % 7; // 0 = Monday ... 6 = Sunday
  d.setUTCDate(d.getUTCDate() - dayIndex);
  return d.toISOString().slice(0, 10);
}

/** `isoDate` shifted by `weeks` * 7 days (negative to go back). */
export function addWeeks(isoDate: string, weeks: number): string {
  const d = new Date(`${isoDate}T00:00:00Z`);
  d.setUTCDate(d.getUTCDate() + weeks * 7);
  return d.toISOString().slice(0, 10);
}

/** "2026-09-07" -> "Th 2, 07/09" for weekday-aware headers (Mon-based, VN labels). */
export function formatWeekdayDate(isoDate: string): string {
  const d = new Date(`${isoDate}T00:00:00Z`);
  const weekdayLabels = ['CN', 'Th 2', 'Th 3', 'Th 4', 'Th 5', 'Th 6', 'Th 7'];
  return `${weekdayLabels[d.getUTCDay()]}, ${formatShortDate(isoDate)}`;
}
