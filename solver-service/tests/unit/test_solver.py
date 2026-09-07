from datetime import date, timedelta

import pytest

from scheduler_api.domain.solver import solve_schedule
from scheduler_api.errors import ReferentialIntegrityError
from scheduler_api.schemas.common import (
    Employee,
    GateShiftRequirement,
    ShiftAvailability,
    SolverConfig,
    SolverWeights,
)
from scheduler_api.schemas.solve import CarryIn, LockedAssignment

START = date(2026, 9, 7)


def days(n: int) -> list[date]:
    return [START + timedelta(days=i) for i in range(n)]


def tiny_config(**overrides) -> SolverConfig:
    defaults = dict(
        requirements={"X": {"sang": GateShiftRequirement(nv=1), "dem": GateShiftRequirement(nv=1)}},
        shift_hours={"X": {"sang": 8, "dem": 8}},
        lead_gates=set(),
        target_hours_per_week=40,
        weights=SolverWeights(streak_penalty_weight=5, streak_length=3),
    )
    defaults.update(overrides)
    return SolverConfig(**defaults)


def lead_config() -> SolverConfig:
    return SolverConfig(
        requirements={
            "A": {"sang": GateShiftRequirement(nv=1), "dem": GateShiftRequirement(nv=1)},
            "B": {
                "sang": GateShiftRequirement(nv=1, lead=1, lead_mandatory_role=True),
                "dem": GateShiftRequirement(nv=1, lead=1, lead_mandatory_role=True),
            },
        },
        shift_hours={"A": {"sang": 8, "dem": 8}, "B": {"sang": 8, "dem": 8}},
        lead_gates={"B"},
        target_hours_per_week=40,
    )


def avail(employee_ids, day_list, sang=True, dem=True) -> dict:
    return {eid: {d: ShiftAvailability(sang=sang, dem=dem) for d in day_list} for eid in employee_ids}


def test_availability_and_one_shift_per_day_are_enforced():
    # NV01 is only available for 'sang', never 'dem' — and can only take one shift a day anyway,
    # so both constraints get exercised by a single-employee, both-shifts-demanded scenario.
    d = days(1)
    employees = [Employee(employee_id="NV01", name="NV01", role="NV")]
    availability = {"NV01": {d[0]: ShiftAvailability(sang=True, dem=False)}}

    result = solve_schedule(
        start_date=START,
        num_days=1,
        employees=employees,
        availability=availability,
        config=tiny_config(),
        time_limit_s=5,
    )

    assert result.status in ("OPTIMAL", "FEASIBLE")
    dem_entries = [e for e in result.schedule if e.shift == "dem"]
    assert dem_entries == []
    sang_entries = [e for e in result.schedule if e.shift == "sang"]
    assert len(sang_entries) == 1
    assert sang_entries[0].employee_id == "NV01"
    dem_shortage = [s for s in result.shortages if s.shift == "dem"]
    assert dem_shortage and dem_shortage[0].missing == 1


def test_tc_only_works_lead_gates():
    d = days(2)
    employees = [
        Employee(employee_id="TC01", name="TC01", role="TC"),
        Employee(employee_id="NV01", name="NV01", role="NV"),
    ]
    availability = avail(["TC01", "NV01"], d)

    result = solve_schedule(
        start_date=START,
        num_days=2,
        employees=employees,
        availability=availability,
        config=lead_config(),
        time_limit_s=5,
    )

    tc_entries = [e for e in result.schedule if e.employee_id == "TC01"]
    assert tc_entries, "TC01 should have been assigned somewhere (only gate B needs a lead)"
    assert all(e.gate == "B" for e in tc_entries)


def test_no_morning_right_after_night_for_same_employee():
    d = days(2)
    # nv=1 for both shifts on both days, single employee -> the solver would want to cover
    # day0-dem AND day1-sang (only they can), but the adjacency rule must block that combination.
    config = tiny_config()
    employees = [Employee(employee_id="NV01", name="NV01", role="NV")]
    availability = avail(["NV01"], d)

    result = solve_schedule(
        start_date=START,
        num_days=2,
        employees=employees,
        availability=availability,
        config=config,
        time_limit_s=5,
    )

    worked_day0_dem = any(e.date == d[0] and e.shift == "dem" for e in result.schedule)
    worked_day1_sang = any(e.date == d[1] and e.shift == "sang" for e in result.schedule)
    assert not (worked_day0_dem and worked_day1_sang)


def test_carry_in_blocks_morning_on_first_day():
    d = days(1)
    employees = [Employee(employee_id="NV01", name="NV01", role="NV")]
    availability = avail(["NV01"], d)
    config = tiny_config(
        requirements={"X": {"sang": GateShiftRequirement(nv=1), "dem": GateShiftRequirement(nv=0)}}
    )

    result = solve_schedule(
        start_date=START,
        num_days=1,
        employees=employees,
        availability=availability,
        config=config,
        carry_in=CarryIn(worked_night_before_start={"NV01"}),
        time_limit_s=5,
    )

    assert result.schedule == []
    assert any(s.shift == "sang" and s.missing == 1 for s in result.shortages)


def test_locked_assignments_are_pinned():
    d = days(1)
    employees = [
        Employee(employee_id="NV01", name="NV01", role="NV"),
        Employee(employee_id="NV02", name="NV02", role="NV"),
    ]
    availability = avail(["NV01", "NV02"], d)
    locked = [
        LockedAssignment(employee_id="NV01", date=d[0], off=True),
        LockedAssignment(employee_id="NV02", date=d[0], gate="X", shift="sang"),
    ]

    result = solve_schedule(
        start_date=START,
        num_days=1,
        employees=employees,
        availability=availability,
        config=tiny_config(),
        locked_assignments=locked,
        time_limit_s=5,
    )

    assert not any(e.employee_id == "NV01" for e in result.schedule)
    assert any(e.employee_id == "NV02" and e.gate == "X" and e.shift == "sang" for e in result.schedule)


def test_shortfall_slack_instead_of_infeasible_when_no_employees():
    result = solve_schedule(
        start_date=START,
        num_days=1,
        employees=[],
        availability={},
        config=tiny_config(),
        time_limit_s=5,
    )

    assert result.status in ("OPTIMAL", "FEASIBLE")
    assert result.schedule == []
    assert {(s.shift, s.missing) for s in result.shortages} == {("sang", 1), ("dem", 1)}


def test_effort_target_hours_pro_rated_for_leave_days():
    d = days(7)
    employees = [Employee(employee_id="NV01", name="NV01", role="NV", leave_days={d[0], d[1], d[2]})]
    availability = avail(["NV01"], d)
    config = tiny_config(
        requirements={"X": {"sang": GateShiftRequirement(nv=0), "dem": GateShiftRequirement(nv=0)}},
        target_hours_per_week=42,
    )

    result = solve_schedule(
        start_date=START,
        num_days=7,
        employees=employees,
        availability=availability,
        config=config,
        time_limit_s=5,
    )

    # base_target_hours_horizon = round(42 * 7/7) = 42; available_days = 4/7
    expected_target = round(42 * 4 / 7)
    summary = result.employee_summary[0]
    assert summary.leave_days == 3
    assert summary.target_hours == expected_target


def test_streak_penalty_avoids_three_consecutive_same_shift_type():
    # Only 'sang' is demanded (1 slot/day, 6 days) so the adjacency rule (c) can't force a
    # streak by itself — with 2 employees and a target that exactly matches a 3-shift-each
    # split, the streak penalty is the only thing that should break the tie away from
    # clustering (e.g. NV01 taking 3 days in a row) toward alternating.
    d = days(6)
    employees = [
        Employee(employee_id="NV01", name="NV01", role="NV"),
        Employee(employee_id="NV02", name="NV02", role="NV"),
    ]
    availability = avail(["NV01", "NV02"], d)
    config = tiny_config(
        requirements={"X": {"sang": GateShiftRequirement(nv=1), "dem": GateShiftRequirement(nv=0)}},
        target_hours_per_week=28,  # -> 24h horizon target = exactly 3 shifts of 8h each
    )

    result = solve_schedule(
        start_date=START,
        num_days=6,
        employees=employees,
        availability=availability,
        config=config,
        time_limit_s=10,
    )

    by_emp: dict[str, dict[date, str]] = {"NV01": {}, "NV02": {}}
    for e in result.schedule:
        by_emp[e.employee_id][e.date] = e.shift

    for eid, sched in by_emp.items():
        for i in range(len(d) - 2):
            window = [sched.get(d[i + k]) for k in range(3)]
            assert not (window[0] == window[1] == window[2] and window[0] is not None), (
                f"{eid} has a 3-day same-shift streak: {window} at day {i}"
            )


def test_referential_integrity_error_for_unknown_locked_employee():
    d = days(1)
    employees = [Employee(employee_id="NV01", name="NV01", role="NV")]
    availability = avail(["NV01"], d)
    locked = [LockedAssignment(employee_id="GHOST", date=d[0], off=True)]

    with pytest.raises(ReferentialIntegrityError):
        solve_schedule(
            start_date=START,
            num_days=1,
            employees=employees,
            availability=availability,
            config=tiny_config(),
            locked_assignments=locked,
        )


def test_referential_integrity_error_for_unknown_carry_in_employee():
    d = days(1)
    employees = [Employee(employee_id="NV01", name="NV01", role="NV")]
    availability = avail(["NV01"], d)

    with pytest.raises(ReferentialIntegrityError):
        solve_schedule(
            start_date=START,
            num_days=1,
            employees=employees,
            availability=availability,
            config=tiny_config(),
            carry_in=CarryIn(worked_night_before_start={"GHOST"}),
        )


def test_locked_assignment_conflicting_with_leave_day_is_rejected_clearly():
    # Regression: previously this fell through to CP-SAT as two contradictory hard constraints
    # on the same variable, producing a bare status=INFEASIBLE that looked like a genuine
    # staffing shortage. It should instead be caught and explained at validation time.
    d = days(1)
    employees = [Employee(employee_id="NV01", name="NV01", role="NV", leave_days={d[0]})]
    availability = avail(["NV01"], d)
    locked = [LockedAssignment(employee_id="NV01", date=d[0], gate="X", shift="sang")]

    with pytest.raises(ReferentialIntegrityError, match="leave"):
        solve_schedule(
            start_date=START,
            num_days=1,
            employees=employees,
            availability=availability,
            config=tiny_config(),
            locked_assignments=locked,
        )


def test_locked_assignment_conflicting_with_unavailability_is_rejected_clearly():
    d = days(1)
    employees = [Employee(employee_id="NV01", name="NV01", role="NV")]
    availability = {"NV01": {d[0]: ShiftAvailability(sang=False, dem=True)}}
    locked = [LockedAssignment(employee_id="NV01", date=d[0], gate="X", shift="sang")]

    with pytest.raises(ReferentialIntegrityError, match="not registered available"):
        solve_schedule(
            start_date=START,
            num_days=1,
            employees=employees,
            availability=availability,
            config=tiny_config(),
            locked_assignments=locked,
        )


def test_locked_assignment_of_tc_outside_lead_gates_is_rejected_clearly():
    d = days(1)
    employees = [Employee(employee_id="TC01", name="TC01", role="TC")]
    availability = avail(["TC01"], d)
    locked = [LockedAssignment(employee_id="TC01", date=d[0], gate="X", shift="sang")]

    with pytest.raises(ReferentialIntegrityError, match="lead_gates"):
        solve_schedule(
            start_date=START,
            num_days=1,
            employees=employees,
            availability=availability,
            config=tiny_config(),  # lead_gates=set(), so gate X is never a lead gate
            locked_assignments=locked,
        )


def test_pc_can_satisfy_a_non_mandatory_lead_slot_but_nv_cannot():
    d = days(1)
    config = SolverConfig(
        requirements={
            "B": {
                "sang": GateShiftRequirement(nv=0, lead=1, lead_mandatory_role=False),
                "dem": GateShiftRequirement(nv=0),
            }
        },
        shift_hours={"B": {"sang": 8, "dem": 8}},
        lead_gates={"B"},
        # 0 so the balance term never rewards assigning someone beyond what staffing actually
        # requires — with a nonzero target, the solver can find it cheaper overall to also staff
        # NV01 just to shrink NV01's own balance deviation, which would make this assertion
        # about *eligibility* (not about whether extra staffing helps) unreliable.
        target_hours_per_week=0,
    )
    employees = [
        Employee(employee_id="PC01", name="PC01", role="PC"),
        Employee(employee_id="NV01", name="NV01", role="NV"),
    ]
    availability = avail(["PC01", "NV01"], d)

    result = solve_schedule(
        start_date=START,
        num_days=1,
        employees=employees,
        availability=availability,
        config=config,
        time_limit_s=5,
    )

    assert result.status in ("OPTIMAL", "FEASIBLE")
    assert any(e.employee_id == "PC01" for e in result.schedule)
    assert not any(e.employee_id == "NV01" for e in result.schedule)
    assert result.shortages == []


def test_high_target_hours_with_small_shift_hours_stays_feasible():
    # Regression: the balance deviation variable's domain used to be sized only from
    # num_days * max_shift_hours, which could sit below an independently-configured high
    # target_hours_per_week and make the whole model spuriously INFEASIBLE.
    d = days(3)
    config = tiny_config(
        requirements={"X": {"sang": GateShiftRequirement(nv=0), "dem": GateShiftRequirement(nv=0)}},
        shift_hours={"X": {"sang": 1, "dem": 1}},
        target_hours_per_week=168,
    )
    employees = [Employee(employee_id="NV01", name="NV01", role="NV")]
    availability = avail(["NV01"], d)

    result = solve_schedule(
        start_date=START,
        num_days=3,
        employees=employees,
        availability=availability,
        config=config,
        time_limit_s=5,
    )

    assert result.status in ("OPTIMAL", "FEASIBLE")
