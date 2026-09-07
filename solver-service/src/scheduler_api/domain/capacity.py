"""Pre-solve capacity check — ported from docs/shift_scheduler_mvp.ipynb (cell 6).

Pure arithmetic (no CP-SAT), so callers can sanity-check demand vs. supply before spending time
on a full solve, or surface a "cần thêm nhân sự" warning in the UI.
"""

from __future__ import annotations

from scheduler_api.schemas.capacity import CapacityCheckResult
from scheduler_api.schemas.common import SHIFT_TYPES, SolverConfig


def check_capacity(*, num_days: int, employee_count: int, config: SolverConfig) -> CapacityCheckResult:
    demand_hours_per_day = sum(
        config.requirements[g][s].total_needed * config.shift_hours[g][s]
        for g in config.gates
        for s in SHIFT_TYPES
    )
    demand_hours_total = demand_hours_per_day * num_days
    target_hours_per_employee = round(config.target_hours_per_week * num_days / 7) if num_days else 0

    if target_hours_per_employee > 0:
        min_employees_required = demand_hours_total / target_hours_per_employee
    else:
        min_employees_required = 0.0

    is_sufficient = employee_count >= min_employees_required
    shortfall_ratio = (
        max(0.0, (min_employees_required - employee_count) / min_employees_required)
        if min_employees_required > 0
        else 0.0
    )

    return CapacityCheckResult(
        demand_hours_per_day=demand_hours_per_day,
        demand_hours_total=demand_hours_total,
        target_hours_per_employee=target_hours_per_employee,
        min_employees_required=round(min_employees_required, 2),
        employee_count=employee_count,
        is_sufficient=is_sufficient,
        shortfall_ratio=round(shortfall_ratio, 4),
    )
