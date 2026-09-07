from __future__ import annotations

from pydantic import BaseModel, Field

from scheduler_api.schemas.common import SolverConfig


class CapacityCheckRequest(BaseModel):
    num_days: int = Field(ge=1, le=180)
    employee_count: int = Field(ge=0)
    config: SolverConfig = Field(default_factory=SolverConfig)


class CapacityCheckResult(BaseModel):
    demand_hours_per_day: int
    demand_hours_total: int
    target_hours_per_employee: int
    min_employees_required: float
    employee_count: int
    is_sufficient: bool
    shortfall_ratio: float
