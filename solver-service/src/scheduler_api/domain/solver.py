"""CP-SAT shift scheduler — ported from docs/shift_scheduler_mvp.ipynb (cell 10).

Business-rule -> constraint mapping (see memo_mvp_solver_coverage.md #1 and notebook cell 9):
  (a) each employee at most 1 shift/day, only if registered available; on leave -> never assigned
  (b) TC (trưởng ca) only works gates in config.lead_gates
  (c) no "sang" shift the day right after a "dem" shift the day before (incl. carry-in from the
      previous horizon, for rolling-horizon continuity)
  (d) locked assignments (already approved) are pinned
  (e) min-staffing per gate/shift + lead-role staffing, both with penalized slack instead of a
      hard infeasibility (-> shortage list for manual review)
  (f) effort balance in HOURS against a per-employee target (pro-rated for leave days)
  (g) soft penalty for `streak_length` consecutive days of the same shift type

Each constraint group is a small helper below, sharing a `_Context` of horizon/gate/employee
lookups so `solve_schedule` itself stays a short top-to-bottom pipeline.
"""

from __future__ import annotations

from dataclasses import dataclass
from datetime import date

from ortools.sat.python import cp_model

from scheduler_api.domain.horizon import build_day_index, build_days, is_available
from scheduler_api.errors import ReferentialIntegrityError
from scheduler_api.schemas.common import SHIFT_TYPES, AvailabilityMap, Employee, ShiftType, SolverConfig
from scheduler_api.schemas.solve import (
    CarryIn,
    EmployeeSummary,
    LockedAssignment,
    ScheduleEntry,
    ShortageItem,
    SolveResult,
)

_STATUS_NAMES = {
    cp_model.OPTIMAL: "OPTIMAL",
    cp_model.FEASIBLE: "FEASIBLE",
    cp_model.INFEASIBLE: "INFEASIBLE",
    cp_model.UNKNOWN: "UNKNOWN",
    cp_model.MODEL_INVALID: "MODEL_INVALID",
}

AssignVar = dict[tuple[str, int, str, ShiftType], cp_model.IntVar]
ShortfallVars = list[tuple[str, int, ShiftType, cp_model.IntVar]]


@dataclass
class _Context:
    days: list[date]
    day_index: dict[date, int]
    gates: list[str]
    eids: list[str]
    emp_by_id: dict[str, Employee]
    config: SolverConfig

    @property
    def num_days(self) -> int:
        return len(self.days)


def _validate_references(
    ctx: _Context,
    availability: AvailabilityMap,
    locked_assignments: list[LockedAssignment],
    carry_in: CarryIn,
) -> None:
    eids = set(ctx.eids)
    for la in locked_assignments:
        if la.employee_id not in eids:
            raise ReferentialIntegrityError(
                f"locked_assignments references unknown employee_id '{la.employee_id}'"
            )
        if la.date not in ctx.day_index:
            raise ReferentialIntegrityError(f"locked_assignments date {la.date} is outside the solve horizon")
        if la.off:
            continue
        if la.gate not in ctx.config.requirements:
            raise ReferentialIntegrityError(f"locked_assignments references unknown gate '{la.gate}'")
        emp = ctx.emp_by_id[la.employee_id]
        if la.date in emp.leave_days:
            raise ReferentialIntegrityError(
                f"locked_assignments assigns '{la.employee_id}' on {la.date}, but they are on leave that day"
            )
        if not is_available(availability, la.employee_id, la.date, la.shift):
            raise ReferentialIntegrityError(
                f"locked_assignments assigns '{la.employee_id}' to {la.gate}/{la.shift} on "
                f"{la.date}, but they are not registered available for that shift"
            )
        if emp.role == "TC" and la.gate not in ctx.config.lead_gates:
            raise ReferentialIntegrityError(
                f"locked_assignments assigns TC '{la.employee_id}' to gate '{la.gate}', which is "
                f"not in lead_gates"
            )
    unknown_carry_in = carry_in.worked_night_before_start - eids
    if unknown_carry_in:
        raise ReferentialIntegrityError(
            f"carry_in.worked_night_before_start references unknown employee_id(s) {sorted(unknown_carry_in)}"
        )


def _build_assignment_variables(model: cp_model.CpModel, ctx: _Context) -> AssignVar:
    assign: AssignVar = {}
    for e in ctx.eids:
        for d in range(ctx.num_days):
            for g in ctx.gates:
                for s in SHIFT_TYPES:
                    assign[e, d, g, s] = model.NewBoolVar(f"a_{e}_{d}_{g}_{s}")
    return assign


def _add_availability_and_leave_constraints(
    model: cp_model.CpModel, assign: AssignVar, ctx: _Context, availability: AvailabilityMap
) -> None:
    for e in ctx.eids:
        emp = ctx.emp_by_id[e]
        for d in range(ctx.num_days):
            model.Add(sum(assign[e, d, g, s] for g in ctx.gates for s in SHIFT_TYPES) <= 1)
            on_leave = ctx.days[d] in emp.leave_days
            for g in ctx.gates:
                for s in SHIFT_TYPES:
                    if on_leave or not is_available(availability, e, ctx.days[d], s):
                        model.Add(assign[e, d, g, s] == 0)


def _add_tc_gate_constraints(model: cp_model.CpModel, assign: AssignVar, ctx: _Context) -> None:
    for e in ctx.eids:
        if ctx.emp_by_id[e].role != "TC":
            continue
        for d in range(ctx.num_days):
            for g in ctx.gates:
                if g not in ctx.config.lead_gates:
                    for s in SHIFT_TYPES:
                        model.Add(assign[e, d, g, s] == 0)


def _add_adjacency_constraints(
    model: cp_model.CpModel, assign: AssignVar, ctx: _Context, carry_in: CarryIn
) -> None:
    for e in ctx.eids:
        for d in range(ctx.num_days - 1):
            night_today = sum(assign[e, d, g, "dem"] for g in ctx.gates)
            morning_next = sum(assign[e, d + 1, g, "sang"] for g in ctx.gates)
            model.Add(night_today + morning_next <= 1)
        if e in carry_in.worked_night_before_start and ctx.num_days > 0:
            for g in ctx.gates:
                model.Add(assign[e, 0, g, "sang"] == 0)


def _add_locked_constraints(
    model: cp_model.CpModel, assign: AssignVar, ctx: _Context, locked_assignments: list[LockedAssignment]
) -> None:
    for la in locked_assignments:
        d = ctx.day_index[la.date]
        for g in ctx.gates:
            for s in SHIFT_TYPES:
                target = 1 if (not la.off and la.gate == g and la.shift == s) else 0
                model.Add(assign[la.employee_id, d, g, s] == target)


def _add_staffing_constraints(
    model: cp_model.CpModel, assign: AssignVar, ctx: _Context
) -> tuple[ShortfallVars, ShortfallVars]:
    shortfall_vars: ShortfallVars = []
    lead_shortfall_vars: ShortfallVars = []
    for d in range(ctx.num_days):
        for g in ctx.gates:
            for s in SHIFT_TYPES:
                req = ctx.config.requirements[g][s]
                people_here = [assign[e, d, g, s] for e in ctx.eids]
                if req.total_needed > 0:
                    short = model.NewIntVar(0, req.total_needed, f"short_{g}_{d}_{s}")
                    model.Add(sum(people_here) + short >= req.total_needed)
                    shortfall_vars.append((g, d, s, short))
                else:
                    # nobody should be scheduled to a gate/shift with zero declared headcount —
                    # otherwise the balance term alone could make the solver invent phantom shifts
                    model.Add(sum(people_here) == 0)
                if g in ctx.config.lead_gates and req.lead > 0:
                    lead_people = [
                        assign[e, d, g, s] for e in ctx.eids if ctx.emp_by_id[e].role in ("TC", "PC")
                    ]
                    strict_tc = [assign[e, d, g, s] for e in ctx.eids if ctx.emp_by_id[e].role == "TC"]
                    lead_short = model.NewIntVar(0, req.lead, f"leadshort_{g}_{d}_{s}")
                    if req.lead_mandatory_role:
                        model.Add(sum(strict_tc) + lead_short >= req.lead)
                    else:
                        model.Add(sum(lead_people) + lead_short >= req.lead)
                    lead_shortfall_vars.append((g, d, s, lead_short))
    return shortfall_vars, lead_shortfall_vars


def _add_balance_constraints(
    model: cp_model.CpModel, assign: AssignVar, ctx: _Context
) -> tuple[list[cp_model.IntVar], dict[str, int]]:
    base_target_hours_horizon = round(ctx.config.target_hours_per_week * ctx.num_days / 7)
    max_shift_hours = max(ctx.config.shift_hours[g][s] for g in ctx.gates for s in SHIFT_TYPES)

    target_hours_by_emp: dict[str, int] = {}
    deviation_vars = []
    for e in ctx.eids:
        emp = ctx.emp_by_id[e]
        available_days = sum(1 for d in range(ctx.num_days) if ctx.days[d] not in emp.leave_days)
        target_hours = round(base_target_hours_horizon * available_days / ctx.num_days) if ctx.num_days else 0
        target_hours_by_emp[e] = target_hours
        total_hours = sum(
            ctx.config.shift_hours[g][s] * assign[e, d, g, s]
            for d in range(ctx.num_days)
            for g in ctx.gates
            for s in SHIFT_TYPES
        )
        # clamp defensively against target_hours: with independently-configurable
        # target_hours_per_week and shift_hours, the "natural" ceiling (num_days * max_shift_hours)
        # could otherwise sit below an unusually high target, which would make the model spuriously
        # INFEASIBLE instead of just failing to fully close the balance gap.
        max_possible_hours = max(ctx.num_days * max_shift_hours, target_hours)
        dev = model.NewIntVar(0, max_possible_hours, f"dev_{e}")
        model.AddAbsEquality(dev, total_hours - target_hours)
        deviation_vars.append(dev)
    return deviation_vars, target_hours_by_emp


def _add_streak_penalty(model: cp_model.CpModel, assign: AssignVar, ctx: _Context) -> list[cp_model.IntVar]:
    streak_len = ctx.config.weights.streak_length
    streak_vars = []
    for e in ctx.eids:
        for d in range(max(0, ctx.num_days - streak_len + 1)):
            for stype in SHIFT_TYPES:
                worked = [model.NewBoolVar(f"w_{e}_{d}_{k}_{stype}") for k in range(streak_len)]
                for k in range(streak_len):
                    model.Add(sum(assign[e, d + k, g, stype] for g in ctx.gates) == worked[k])
                streak = model.NewBoolVar(f"streak_{e}_{d}_{stype}")
                model.AddBoolAnd(worked).OnlyEnforceIf(streak)
                model.AddBoolOr([w.Not() for w in worked]).OnlyEnforceIf(streak.Not())
                streak_vars.append(streak)
    return streak_vars


def _set_objective(
    model: cp_model.CpModel,
    config: SolverConfig,
    shortfall_vars: ShortfallVars,
    lead_shortfall_vars: ShortfallVars,
    deviation_vars: list[cp_model.IntVar],
    streak_vars: list[cp_model.IntVar],
) -> None:
    weights = config.weights
    model.Minimize(
        weights.shortfall_penalty * sum(v for *_, v in shortfall_vars)
        + weights.lead_shortfall_penalty * sum(v for *_, v in lead_shortfall_vars)
        + weights.balance_penalty_weight * sum(deviation_vars)
        + weights.streak_penalty_weight * sum(streak_vars)
    )


def _extract_result(
    solver: cp_model.CpSolver,
    status: int,
    assign: AssignVar,
    ctx: _Context,
    shortfall_vars: ShortfallVars,
    lead_shortfall_vars: ShortfallVars,
    target_hours_by_emp: dict[str, int],
) -> SolveResult:
    has_solution = status in (cp_model.OPTIMAL, cp_model.FEASIBLE)

    schedule: list[ScheduleEntry] = []
    hours_by_emp: dict[str, int] = dict.fromkeys(ctx.eids, 0)
    shifts_by_emp: dict[str, int] = dict.fromkeys(ctx.eids, 0)
    if has_solution:
        for e in ctx.eids:
            for d in range(ctx.num_days):
                for g in ctx.gates:
                    for s in SHIFT_TYPES:
                        if solver.Value(assign[e, d, g, s]):
                            schedule.append(ScheduleEntry(employee_id=e, date=ctx.days[d], gate=g, shift=s))
                            hours_by_emp[e] += ctx.config.shift_hours[g][s]
                            shifts_by_emp[e] += 1

    shortages: list[ShortageItem] = []
    if has_solution:
        for g, d, s, var in shortfall_vars:
            if solver.Value(var) > 0:
                shortages.append(
                    ShortageItem(
                        date=ctx.days[d], gate=g, shift=s, shortage_type="staff", missing=solver.Value(var)
                    )
                )
        for g, d, s, var in lead_shortfall_vars:
            if solver.Value(var) > 0:
                shortages.append(
                    ShortageItem(
                        date=ctx.days[d], gate=g, shift=s, shortage_type="lead", missing=solver.Value(var)
                    )
                )

    employee_summary = [
        EmployeeSummary(
            employee_id=e,
            role=ctx.emp_by_id[e].role,
            leave_days=len(ctx.emp_by_id[e].leave_days),
            target_hours=target_hours_by_emp[e],
            actual_hours=hours_by_emp[e],
            deviation_hours=hours_by_emp[e] - target_hours_by_emp[e],
            actual_shifts=shifts_by_emp[e],
        )
        for e in ctx.eids
    ]

    return SolveResult(
        status=_STATUS_NAMES.get(status, "UNKNOWN"),
        objective_value=solver.ObjectiveValue() if has_solution else None,
        wall_time_s=round(solver.WallTime(), 2),
        schedule=schedule,
        shortages=shortages,
        employee_summary=employee_summary,
    )


def solve_schedule(
    *,
    start_date: date,
    num_days: int,
    employees: list[Employee],
    availability: AvailabilityMap,
    config: SolverConfig,
    locked_assignments: list[LockedAssignment] | None = None,
    carry_in: CarryIn | None = None,
    time_limit_s: int = 30,
    num_search_workers: int = 4,
) -> SolveResult:
    locked_assignments = locked_assignments or []
    carry_in = carry_in or CarryIn()

    days = build_days(start_date, num_days)
    ctx = _Context(
        days=days,
        day_index=build_day_index(days),
        gates=config.gates,
        eids=[e.employee_id for e in employees],
        emp_by_id={e.employee_id: e for e in employees},
        config=config,
    )
    _validate_references(ctx, availability, locked_assignments, carry_in)

    model = cp_model.CpModel()
    assign = _build_assignment_variables(model, ctx)

    _add_availability_and_leave_constraints(model, assign, ctx, availability)
    _add_tc_gate_constraints(model, assign, ctx)
    _add_adjacency_constraints(model, assign, ctx, carry_in)
    _add_locked_constraints(model, assign, ctx, locked_assignments)
    shortfall_vars, lead_shortfall_vars = _add_staffing_constraints(model, assign, ctx)
    deviation_vars, target_hours_by_emp = _add_balance_constraints(model, assign, ctx)
    streak_vars = _add_streak_penalty(model, assign, ctx)
    _set_objective(model, config, shortfall_vars, lead_shortfall_vars, deviation_vars, streak_vars)

    solver = cp_model.CpSolver()
    solver.parameters.max_time_in_seconds = time_limit_s
    solver.parameters.num_search_workers = num_search_workers
    status = solver.Solve(model)

    return _extract_result(
        solver, status, assign, ctx, shortfall_vars, lead_shortfall_vars, target_hours_by_emp
    )
