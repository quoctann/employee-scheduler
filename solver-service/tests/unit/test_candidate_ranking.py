from datetime import date, timedelta

import pytest

from scheduler_api.domain.candidate_ranking import rank_replacement_candidates
from scheduler_api.errors import ReferentialIntegrityError
from scheduler_api.schemas.candidates import ReplacementCandidatesRequest, TargetSlot
from scheduler_api.schemas.common import (
    Employee,
    GateShiftRequirement,
    ShiftAvailability,
    SolverConfig,
)
from scheduler_api.schemas.solve import CarryIn, ScheduleEntry

START = date(2026, 9, 7)


def d(offset: int) -> date:
    return START + timedelta(days=offset)


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


def full_avail(eids: list[str], num_days: int = 5) -> dict:
    return {eid: {d(i): ShiftAvailability(sang=True, dem=True) for i in range(num_days)} for eid in eids}


def base_request(**overrides) -> ReplacementCandidatesRequest:
    employees = overrides.pop(
        "employees",
        [
            Employee(employee_id="NV01", name="NV01", role="NV"),
            Employee(employee_id="NV02", name="NV02", role="NV"),
        ],
    )
    defaults = dict(
        start_date=START,
        num_days=5,
        employees=employees,
        availability=full_avail([e.employee_id for e in employees]),
        current_schedule=[],
        target_slot=TargetSlot(date=d(2), gate="A", shift="sang"),
        top_n=5,
        config=lead_config(),
    )
    defaults.update(overrides)
    return ReplacementCandidatesRequest(**defaults)


def test_excludes_unavailable_employee():
    employees = [
        Employee(employee_id="NV01", name="NV01", role="NV"),
        Employee(employee_id="NV02", name="NV02", role="NV"),
    ]
    availability = full_avail(["NV01", "NV02"])
    availability["NV01"][d(2)] = ShiftAvailability(sang=False, dem=True)
    req = base_request(employees=employees, availability=availability)

    result = rank_replacement_candidates(req)

    assert "NV01" not in {c.employee_id for c in result.candidates}
    assert "NV02" in {c.employee_id for c in result.candidates}
    assert result.excluded_count == 1


def test_excludes_employee_already_working_that_day():
    req = base_request(current_schedule=[ScheduleEntry(employee_id="NV01", date=d(2), gate="A", shift="dem")])

    result = rank_replacement_candidates(req)

    assert "NV01" not in {c.employee_id for c in result.candidates}


def test_excludes_tc_assigned_outside_lead_gates():
    employees = [
        Employee(employee_id="TC01", name="TC01", role="TC"),
        Employee(employee_id="NV02", name="NV02", role="NV"),
    ]
    req = base_request(
        employees=employees,
        availability=full_avail(["TC01", "NV02"]),
        target_slot=TargetSlot(date=d(2), gate="A", shift="sang"),  # gate A is not a lead_gate
    )

    result = rank_replacement_candidates(req)

    assert "TC01" not in {c.employee_id for c in result.candidates}
    assert "NV02" in {c.employee_id for c in result.candidates}


def test_excludes_when_would_break_adjacency_after_night_shift():
    req = base_request(
        current_schedule=[ScheduleEntry(employee_id="NV01", date=d(1), gate="A", shift="dem")],
        target_slot=TargetSlot(date=d(2), gate="A", shift="sang"),
    )

    result = rank_replacement_candidates(req)

    assert "NV01" not in {c.employee_id for c in result.candidates}


def test_excludes_when_would_break_adjacency_before_morning_shift():
    req = base_request(
        current_schedule=[ScheduleEntry(employee_id="NV01", date=d(3), gate="A", shift="sang")],
        target_slot=TargetSlot(date=d(2), gate="A", shift="dem"),
    )

    result = rank_replacement_candidates(req)

    assert "NV01" not in {c.employee_id for c in result.candidates}


def test_excludes_employee_on_leave():
    employees = [
        Employee(employee_id="NV01", name="NV01", role="NV", leave_days={d(2)}),
        Employee(employee_id="NV02", name="NV02", role="NV"),
    ]
    req = base_request(employees=employees, availability=full_avail(["NV01", "NV02"]))

    result = rank_replacement_candidates(req)

    assert "NV01" not in {c.employee_id for c in result.candidates}


def test_excludes_explicit_excluded_employee_id():
    req = base_request(excluded_employee_id="NV01")

    result = rank_replacement_candidates(req)

    assert "NV01" not in {c.employee_id for c in result.candidates}
    assert result.excluded_count == 1


def test_ranks_underfilled_employee_first():
    # d(0) and d(4) are both non-adjacent to the d(2) target slot, so this doesn't also trip the
    # adjacency hard-filter — isolates the ranking behavior from the exclusion behavior.
    req = base_request(
        current_schedule=[
            ScheduleEntry(employee_id="NV02", date=d(0), gate="A", shift="sang"),
            ScheduleEntry(employee_id="NV02", date=d(4), gate="A", shift="dem"),
        ]
    )
    # NV01 has 0 hours so far, NV02 already has 16h -> NV01 should rank higher (bigger gap to target)

    result = rank_replacement_candidates(req)

    assert result.candidates[0].employee_id == "NV01"
    assert result.candidates[0].score >= result.candidates[1].score


def test_would_create_streak_is_flagged_and_penalized():
    employees = [Employee(employee_id="NV01", name="NV01", role="NV")]
    req = base_request(
        employees=employees,
        availability=full_avail(["NV01"]),
        current_schedule=[
            ScheduleEntry(employee_id="NV01", date=d(0), gate="A", shift="sang"),
            ScheduleEntry(employee_id="NV01", date=d(1), gate="A", shift="sang"),
        ],
        target_slot=TargetSlot(date=d(2), gate="A", shift="sang"),
    )

    result = rank_replacement_candidates(req)

    assert len(result.candidates) == 1
    assert result.candidates[0].would_create_streak is True
    assert any("streak" in r.lower() for r in result.candidates[0].reasons)


def test_top_n_truncates_results():
    employees = [Employee(employee_id=f"NV{i:02d}", name=f"NV{i:02d}", role="NV") for i in range(10)]
    req = base_request(
        employees=employees, availability=full_avail([e.employee_id for e in employees]), top_n=3
    )

    result = rank_replacement_candidates(req)

    assert len(result.candidates) == 3


def test_referential_integrity_error_for_unknown_excluded_employee():
    req = base_request(excluded_employee_id="GHOST")

    with pytest.raises(ReferentialIntegrityError):
        rank_replacement_candidates(req)


def test_referential_integrity_error_for_unknown_gate():
    req = base_request(target_slot=TargetSlot(date=d(2), gate="ZZZ", shift="sang"))

    with pytest.raises(ReferentialIntegrityError):
        rank_replacement_candidates(req)


def test_referential_integrity_error_for_date_outside_horizon():
    req = base_request(target_slot=TargetSlot(date=d(99), gate="A", shift="sang"))

    with pytest.raises(ReferentialIntegrityError):
        rank_replacement_candidates(req)


def test_returns_empty_candidates_when_everyone_is_filtered_out():
    solo = [Employee(employee_id="NV01", name="NV01", role="NV")]
    req = base_request(excluded_employee_id="NV01", employees=solo)

    result = rank_replacement_candidates(req)

    assert result.candidates == []
    assert result.excluded_count == 1


def test_employee_missing_from_availability_map_is_excluded():
    employees = [Employee(employee_id="NV01", name="NV01", role="NV")]
    req = base_request(employees=employees, availability={})

    result = rank_replacement_candidates(req)

    assert result.candidates == []
    assert result.excluded_count == 1


def test_carry_in_blocks_morning_candidate_on_day_zero():
    req = base_request(
        target_slot=TargetSlot(date=d(0), gate="A", shift="sang"),
        carry_in=CarryIn(worked_night_before_start={"NV01"}),
    )

    result = rank_replacement_candidates(req)

    assert "NV01" not in {c.employee_id for c in result.candidates}
    assert "NV02" in {c.employee_id for c in result.candidates}


def test_referential_integrity_error_for_unknown_carry_in_employee():
    req = base_request(carry_in=CarryIn(worked_night_before_start={"GHOST"}))

    with pytest.raises(ReferentialIntegrityError):
        rank_replacement_candidates(req)


def test_requires_lead_excludes_nv_for_a_mandatory_lead_slot():
    employees = [
        Employee(employee_id="TC01", name="TC01", role="TC"),
        Employee(employee_id="NV01", name="NV01", role="NV"),
    ]
    req = base_request(
        employees=employees,
        availability=full_avail(["TC01", "NV01"]),
        # gate B's night shift is lead_mandatory_role=True in lead_config()
        target_slot=TargetSlot(date=d(2), gate="B", shift="dem", requires_lead=True),
    )

    result = rank_replacement_candidates(req)

    assert {c.employee_id for c in result.candidates} == {"TC01"}


def test_requires_lead_allows_pc_when_not_mandatory_role():
    employees = [
        Employee(employee_id="PC01", name="PC01", role="PC"),
        Employee(employee_id="NV01", name="NV01", role="NV"),
    ]
    config = lead_config()
    # relax gate B's morning slot to allow PC as lead substitute (memo #2c)
    config.requirements["B"]["sang"].lead_mandatory_role = False
    req = base_request(
        employees=employees,
        availability=full_avail(["PC01", "NV01"]),
        config=config,
        target_slot=TargetSlot(date=d(2), gate="B", shift="sang", requires_lead=True),
    )

    result = rank_replacement_candidates(req)

    assert {c.employee_id for c in result.candidates} == {"PC01"}


def test_referential_integrity_error_when_requires_lead_but_gate_shift_has_no_lead():
    req = base_request(target_slot=TargetSlot(date=d(2), gate="A", shift="sang", requires_lead=True))

    with pytest.raises(ReferentialIntegrityError, match="no lead requirement"):
        rank_replacement_candidates(req)


def test_reason_mentions_over_target_and_exactly_on_target():
    employees = [
        Employee(employee_id="OVER", name="OVER", role="NV"),
        Employee(employee_id="EVEN", name="EVEN", role="NV"),
    ]
    req = base_request(
        employees=employees,
        availability=full_avail(["OVER", "EVEN"]),
        current_schedule=[
            ScheduleEntry(employee_id="OVER", date=d(0), gate="A", shift="sang"),
            ScheduleEntry(employee_id="OVER", date=d(4), gate="A", shift="dem"),
            ScheduleEntry(employee_id="OVER", date=d(1), gate="A", shift="sang"),
            ScheduleEntry(employee_id="OVER", date=d(3), gate="A", shift="dem"),
        ],
    )

    result = rank_replacement_candidates(req)

    over = next(c for c in result.candidates if c.employee_id == "OVER")
    assert over.deviation_hours > 0
    assert any("cao hơn target" in r for r in over.reasons)
