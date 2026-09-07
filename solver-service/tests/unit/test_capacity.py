from scheduler_api.domain.capacity import check_capacity
from scheduler_api.schemas.common import GateShiftRequirement, SolverConfig


def test_matches_notebook_sample_numbers():
    # Regression check against docs/memo_mvp_solver_coverage.md #1.6: with the real shift-hours
    # table (A/G 11h-13h, B/D 11h-12h) and 44h/week target, demand should be ~140 hours/day and
    # require ~22-23 employees minimum for a 28-day horizon.
    config = SolverConfig()  # defaults already match the notebook's real-data sample
    result = check_capacity(num_days=28, employee_count=21, config=config)

    assert result.demand_hours_per_day == 140
    assert result.demand_hours_total == 140 * 28
    assert result.target_hours_per_employee == 176
    assert 22.0 <= result.min_employees_required <= 23.0
    assert result.is_sufficient is False
    assert result.shortfall_ratio > 0


def test_sufficient_when_employee_count_covers_demand():
    config = SolverConfig(
        requirements={"X": {"sang": GateShiftRequirement(nv=1), "dem": GateShiftRequirement(nv=0)}},
        shift_hours={"X": {"sang": 8, "dem": 8}},
        lead_gates=set(),
        target_hours_per_week=56,  # -> 8h/day/employee target
    )
    # demand = 8h/day * 7 days = 56h; target/employee for 7 days = 56h -> exactly 1 employee needed
    result = check_capacity(num_days=7, employee_count=1, config=config)

    assert result.min_employees_required == 1.0
    assert result.is_sufficient is True
    assert result.shortfall_ratio == 0.0


def test_zero_days_does_not_divide_by_zero():
    config = SolverConfig()
    result = check_capacity(num_days=0, employee_count=0, config=config)

    assert result.demand_hours_total == 0
    assert result.min_employees_required == 0.0
    assert result.is_sufficient is True
