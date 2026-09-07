from __future__ import annotations

from datetime import date
from typing import Literal

from pydantic import BaseModel, Field, model_validator

from scheduler_api.schemas.common import (
    SHIFT_TYPES,
    AvailabilityMap,
    Employee,
    Role,
    ShiftType,
    SolverConfig,
)

SolveStatus = Literal["OPTIMAL", "FEASIBLE", "INFEASIBLE", "UNKNOWN", "MODEL_INVALID"]
ShortageType = Literal["staff", "lead"]

MAX_EMPLOYEES = 2000
MAX_LOCKED_ASSIGNMENTS = 50_000
# len(employees) * num_days * len(gates) * len(shift_types) CP-SAT boolean variables get built
# before solving even starts (domain/solver.py) — this bounds worst-case model-construction cost
# regardless of how the individual field caps above are combined.
MAX_MODEL_VARIABLES = 2_000_000


class LockedAssignment(BaseModel):
    employee_id: str
    date: date
    gate: str | None = None
    shift: ShiftType | None = None
    off: bool = False

    @model_validator(mode="after")
    def _validate_shape(self) -> LockedAssignment:
        if self.off:
            if self.gate is not None or self.shift is not None:
                raise ValueError("locked assignment with off=true must not set gate/shift")
        else:
            if self.gate is None or self.shift is None:
                raise ValueError("locked assignment must set both gate and shift when off=false")
        return self


class CarryIn(BaseModel):
    worked_night_before_start: set[str] = Field(default_factory=set)


class SolveRequest(BaseModel):
    start_date: date
    num_days: int = Field(ge=1, le=180)
    employees: list[Employee] = Field(max_length=MAX_EMPLOYEES)
    availability: AvailabilityMap = Field(max_length=MAX_EMPLOYEES)
    locked_assignments: list[LockedAssignment] = Field(
        default_factory=list, max_length=MAX_LOCKED_ASSIGNMENTS
    )
    carry_in: CarryIn = Field(default_factory=CarryIn)
    config: SolverConfig = Field(default_factory=SolverConfig)
    time_limit_s: int = Field(default=30, ge=1, le=300)

    @model_validator(mode="after")
    def _bound_model_size(self) -> SolveRequest:
        total_vars = len(self.employees) * self.num_days * len(self.config.gates) * len(SHIFT_TYPES)
        if total_vars > MAX_MODEL_VARIABLES:
            raise ValueError(
                f"employees x num_days x gates x shift_types = {total_vars} exceeds the "
                f"{MAX_MODEL_VARIABLES} solver-variable ceiling; reduce the horizon, employee "
                f"count, or number of gates"
            )
        return self


class ScheduleEntry(BaseModel):
    employee_id: str
    date: date
    gate: str
    shift: ShiftType


class ShortageItem(BaseModel):
    date: date
    gate: str
    shift: ShiftType
    shortage_type: ShortageType
    missing: int


class EmployeeSummary(BaseModel):
    employee_id: str
    role: Role
    leave_days: int
    target_hours: int
    actual_hours: int
    deviation_hours: int
    actual_shifts: int


class SolveResult(BaseModel):
    status: SolveStatus
    objective_value: float | None
    wall_time_s: float
    schedule: list[ScheduleEntry]
    shortages: list[ShortageItem]
    employee_summary: list[EmployeeSummary]
