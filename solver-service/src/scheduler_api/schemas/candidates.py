from __future__ import annotations

from datetime import date

from pydantic import BaseModel, Field

from scheduler_api.schemas.common import AvailabilityMap, Employee, Role, ShiftType, SolverConfig
from scheduler_api.schemas.solve import MAX_EMPLOYEES, MAX_LOCKED_ASSIGNMENTS, CarryIn, ScheduleEntry


class TargetSlot(BaseModel):
    date: date
    gate: str
    shift: ShiftType
    # True when the person being replaced was specifically covering this gate/shift's lead
    # requirement (config.requirements[gate][shift].lead > 0) — narrows candidates to those
    # eligible for the lead slot (TC only if lead_mandatory_role, else TC or PC) instead of any
    # available body. Leave False for an ordinary NV slot.
    requires_lead: bool = False


class ReplacementCandidatesRequest(BaseModel):
    start_date: date
    num_days: int = Field(ge=1, le=180)
    employees: list[Employee] = Field(max_length=MAX_EMPLOYEES)
    availability: AvailabilityMap = Field(max_length=MAX_EMPLOYEES)
    current_schedule: list[ScheduleEntry] = Field(default_factory=list, max_length=MAX_LOCKED_ASSIGNMENTS)
    # mirrors SolveRequest.carry_in: who worked the night shift on the day before start_date, so
    # a candidate for a day-0 'sang' slot isn't suggested in violation of the adjacency rule the
    # solver itself enforces (docs/memo_mvp_solver_coverage.md #1.6 rolling-horizon continuity).
    carry_in: CarryIn = Field(default_factory=CarryIn)
    target_slot: TargetSlot
    excluded_employee_id: str | None = None
    top_n: int = Field(default=5, ge=1, le=50)
    config: SolverConfig = Field(default_factory=SolverConfig)


class CandidateItem(BaseModel):
    employee_id: str
    role: Role
    score: float
    target_hours: int
    actual_hours_so_far: int
    deviation_hours: int
    would_create_streak: bool
    reasons: list[str]


class ReplacementCandidatesResult(BaseModel):
    target_slot: TargetSlot
    candidates: list[CandidateItem]
    excluded_count: int
