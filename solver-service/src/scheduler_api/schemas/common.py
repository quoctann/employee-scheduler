"""Shared domain schemas: employees, availability, solver config.

These are used directly by the domain layer (solver/capacity/candidate_ranking) as well as
the API request/response schemas — there is no separate internal dataclass layer, since the
Pydantic models already are the right shape for both validation and computation.
"""

from __future__ import annotations

from datetime import date
from typing import Literal

from pydantic import BaseModel, Field, model_validator

ShiftType = Literal["sang", "dem"]
Role = Literal["NV", "TC", "PC"]

SHIFT_TYPES: tuple[ShiftType, ...] = ("sang", "dem")


class Employee(BaseModel):
    employee_id: str = Field(max_length=100)
    name: str = Field(max_length=200)
    role: Role
    leave_days: set[date] = Field(default_factory=set, max_length=366)


class ShiftAvailability(BaseModel):
    sang: bool = False
    dem: bool = False


# employee_id -> date -> ShiftAvailability
AvailabilityMap = dict[str, dict[date, ShiftAvailability]]


class GateShiftRequirement(BaseModel):
    nv: int = Field(default=0, ge=0, le=1000)
    lead: int = Field(default=0, ge=0, le=1000)
    lead_mandatory_role: bool = False

    @property
    def total_needed(self) -> int:
        return self.nv + self.lead


class SolverWeights(BaseModel):
    shortfall_penalty: int = Field(default=1000, ge=0, le=1_000_000)
    lead_shortfall_penalty: int = Field(default=800, ge=0, le=1_000_000)
    balance_penalty_weight: int = Field(default=1, ge=0, le=1_000_000)
    streak_penalty_weight: int = Field(default=2, ge=0, le=1_000_000)
    streak_length: int = Field(default=3, ge=1, le=60)


def _default_requirements() -> dict[str, dict[ShiftType, GateShiftRequirement]]:
    return {
        "A": {"sang": GateShiftRequirement(nv=1), "dem": GateShiftRequirement(nv=1)},
        "B": {
            "sang": GateShiftRequirement(nv=2, lead=1, lead_mandatory_role=False),
            "dem": GateShiftRequirement(nv=1, lead=1, lead_mandatory_role=True),
        },
        "G": {"sang": GateShiftRequirement(nv=1), "dem": GateShiftRequirement(nv=1)},
        "D": {"sang": GateShiftRequirement(nv=1), "dem": GateShiftRequirement(nv=2)},
    }


def _default_shift_hours() -> dict[str, dict[ShiftType, int]]:
    return {
        "A": {"sang": 11, "dem": 13},
        "B": {"sang": 11, "dem": 12},
        "G": {"sang": 11, "dem": 13},
        "D": {"sang": 11, "dem": 12},
    }


MAX_GATES = 100


class SolverConfig(BaseModel):
    requirements: dict[str, dict[ShiftType, GateShiftRequirement]] = Field(
        default_factory=_default_requirements, max_length=MAX_GATES
    )
    shift_hours: dict[str, dict[ShiftType, int]] = Field(
        default_factory=_default_shift_hours, max_length=MAX_GATES
    )
    lead_gates: set[str] = Field(default_factory=lambda: {"B"}, max_length=MAX_GATES)
    target_hours_per_week: int = Field(default=44, ge=0, le=168)
    weights: SolverWeights = Field(default_factory=SolverWeights)

    @property
    def gates(self) -> list[str]:
        return list(self.requirements.keys())

    @model_validator(mode="after")
    def _validate_consistency(self) -> SolverConfig:
        req_gates = set(self.requirements.keys())
        hours_gates = set(self.shift_hours.keys())
        if req_gates != hours_gates:
            raise ValueError(
                f"requirements gates {sorted(req_gates)} must match shift_hours gates {sorted(hours_gates)}"
            )
        if not self.lead_gates.issubset(req_gates):
            raise ValueError(
                f"lead_gates {sorted(self.lead_gates)} must be a subset of gates {sorted(req_gates)}"
            )
        for gate, per_shift in self.requirements.items():
            missing = set(SHIFT_TYPES) - set(per_shift.keys())
            if missing:
                raise ValueError(f"gate '{gate}' is missing requirements for shifts {missing}")
        for gate, per_shift in self.shift_hours.items():
            missing = set(SHIFT_TYPES) - set(per_shift.keys())
            if missing:
                raise ValueError(f"gate '{gate}' is missing shift_hours for shifts {missing}")
            for shift, hours in per_shift.items():
                if not (1 <= hours <= 24):
                    raise ValueError(f"gate '{gate}' shift '{shift}' shift_hours={hours} must be in [1, 24]")
        for gate, per_shift in self.requirements.items():
            for shift, req in per_shift.items():
                if req.lead > 0 and gate not in self.lead_gates:
                    raise ValueError(
                        f"gate '{gate}' shift '{shift}' requires lead slots but is not in lead_gates"
                    )
        return self
