import { StarIcon } from 'lucide-react';
import { cn } from '@/lib/utils';
import type { Role } from '@/api/types';

/** Role rendered as plain text, with a small star marking TC (trưởng ca —
 *  shift lead) specifically. NV (staff) and PC (phó ca — deputy lead) render
 *  as text only; only TC gets a visual marker, by design. */
export function RoleBadge({ role, className }: { role: Role; className?: string }) {
  return (
    <span className={cn('inline-flex items-center gap-1', className)}>
      {role}
      {role === 'TC' && (
        <StarIcon className="size-3 text-amber-500" fill="currentColor" aria-hidden="true" />
      )}
    </span>
  );
}
