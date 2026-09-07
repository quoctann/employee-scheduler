"""Top-N replacement-candidate ranking — implements memo item #9b.

Deliberately NOT a CP-SAT re-solve: this is a fast, explainable scoring pass over the employee
list so a manager can pick a replacement by hand instead of the solver deciding unilaterally.
Default ranking criteria (proposed here per backlog note #2, open to tuning once reviewed):
  1. hard-filter out anyone who structurally can't take the slot (unavailable, double-booked,
     breaks the "no sang right after dem" adjacency rule — including carry-in from the previous
     horizon at day 0 — TC working outside lead_gates, or not eligible for a lead-mandatory slot)
  2. rank the rest by how far *below* their target hours they currently are (helps rebalance
     effort), penalizing candidates who would create a `streak_length`-day same-shift streak
"""

from __future__ import annotations

from collections import defaultdict
from dataclasses import dataclass
from datetime import date, timedelta

from scheduler_api.domain.horizon import build_day_index, build_days, is_available
from scheduler_api.errors import ReferentialIntegrityError
from scheduler_api.schemas.candidates import (
    CandidateItem,
    ReplacementCandidatesRequest,
    ReplacementCandidatesResult,
    TargetSlot,
)
from scheduler_api.schemas.common import Employee, Role
from scheduler_api.schemas.solve import ScheduleEntry

_STREAK_PENALTY = 0.15

ScheduleByEmpDate = dict[tuple[str, date], ScheduleEntry]


@dataclass
class _CandidateDraft:
    employee_id: str
    role: Role
    target_hours: int
    actual_hours_so_far: int
    would_create_streak: bool

    @property
    def deviation_hours(self) -> int:
        return self.actual_hours_so_far - self.target_hours


def _validate_references(req: ReplacementCandidatesRequest, day_index: dict[date, int]) -> None:
    eids = {e.employee_id for e in req.employees}
    if req.excluded_employee_id is not None and req.excluded_employee_id not in eids:
        raise ReferentialIntegrityError(
            f"excluded_employee_id '{req.excluded_employee_id}' not found in employees"
        )
    unknown_carry_in = req.carry_in.worked_night_before_start - eids
    if unknown_carry_in:
        raise ReferentialIntegrityError(
            f"carry_in.worked_night_before_start references unknown employee_id(s) {sorted(unknown_carry_in)}"
        )
    if req.target_slot.gate not in req.config.requirements:
        raise ReferentialIntegrityError(f"target_slot references unknown gate '{req.target_slot.gate}'")
    if req.target_slot.date not in day_index:
        raise ReferentialIntegrityError("target_slot.date is outside the given horizon")
    if req.target_slot.requires_lead:
        slot_req = req.config.requirements[req.target_slot.gate][req.target_slot.shift]
        if slot_req.lead == 0:
            raise ReferentialIntegrityError(
                f"target_slot.requires_lead=true but {req.target_slot.gate}/{req.target_slot.shift} "
                f"has no lead requirement configured"
            )


def _would_create_streak(
    entries: list[ScheduleEntry], slot_date: date, slot_shift: str, streak_length: int
) -> bool:
    shift_by_date: dict[date, str] = {e.date: e.shift for e in entries}
    shift_by_date[slot_date] = slot_shift
    for offset in range(streak_length):
        window_start = slot_date - timedelta(days=offset)
        window = [window_start + timedelta(days=i) for i in range(streak_length)]
        if all(shift_by_date.get(d) == slot_shift for d in window):
            return True
    return False


def _is_eligible(
    req: ReplacementCandidatesRequest,
    emp: Employee,
    slot: TargetSlot,
    day_index: dict[date, int],
    schedule_by_emp_date: ScheduleByEmpDate,
) -> bool:
    eid = emp.employee_id
    if eid == req.excluded_employee_id:
        return False
    if slot.date in emp.leave_days:
        return False
    if not is_available(req.availability, eid, slot.date, slot.shift):
        return False
    if (eid, slot.date) in schedule_by_emp_date:
        return False
    if emp.role == "TC" and slot.gate not in req.config.lead_gates:
        return False
    if slot.requires_lead:
        slot_req = req.config.requirements[slot.gate][slot.shift]
        eligible_roles = {"TC"} if slot_req.lead_mandatory_role else {"TC", "PC"}
        if emp.role not in eligible_roles:
            return False
    if slot.shift == "sang":
        if day_index[slot.date] == 0 and eid in req.carry_in.worked_night_before_start:
            return False
        prev_entry = schedule_by_emp_date.get((eid, slot.date - timedelta(days=1)))
        if prev_entry is not None and prev_entry.shift == "dem":
            return False
    else:
        next_entry = schedule_by_emp_date.get((eid, slot.date + timedelta(days=1)))
        if next_entry is not None and next_entry.shift == "sang":
            return False
    return True


def _draft_candidate(
    req: ReplacementCandidatesRequest,
    emp: Employee,
    slot: TargetSlot,
    days: list[date],
    base_target_hours_horizon: int,
    schedule_by_emp: dict[str, list[ScheduleEntry]],
) -> _CandidateDraft:
    available_days = sum(1 for d in days if d not in emp.leave_days)
    target_hours = round(base_target_hours_horizon * available_days / req.num_days) if req.num_days else 0
    entries = schedule_by_emp.get(emp.employee_id, [])
    actual_hours_so_far = sum(req.config.shift_hours[e.gate][e.shift] for e in entries)
    would_create_streak = _would_create_streak(
        entries, slot.date, slot.shift, req.config.weights.streak_length
    )
    return _CandidateDraft(
        employee_id=emp.employee_id,
        role=emp.role,
        target_hours=target_hours,
        actual_hours_so_far=actual_hours_so_far,
        would_create_streak=would_create_streak,
    )


def _score_candidates(
    drafts: list[_CandidateDraft], streak_length: int, slot: TargetSlot
) -> list[CandidateItem]:
    gaps = [d.target_hours - d.actual_hours_so_far for d in drafts]
    min_gap, max_gap = min(gaps), max(gaps)
    span = max(max_gap - min_gap, 1)

    scored: list[CandidateItem] = []
    for draft, gap in zip(drafts, gaps, strict=True):
        base_score = (gap - min_gap) / span
        penalty = _STREAK_PENALTY if draft.would_create_streak else 0.0
        score = max(0.0, min(1.0, base_score - penalty))

        reasons = []
        if gap > 0:
            reasons.append(f"Đang thấp hơn target {gap}h — nên ưu tiên bù")
        elif gap < 0:
            reasons.append(f"Đang cao hơn target {abs(gap)}h")
        else:
            reasons.append("Đang đúng target giờ")
        reasons.append(
            f"Sẽ tạo streak {streak_length} ca liên tiếp cùng loại — cân nhắc"
            if draft.would_create_streak
            else f"Không tạo streak {streak_length} ca liên tiếp cùng loại"
        )
        reasons.append(f"Đã đăng ký sẵn sàng ca {slot.shift} ngày {slot.date.isoformat()}")

        scored.append(
            CandidateItem(
                employee_id=draft.employee_id,
                role=draft.role,
                score=round(score, 4),
                target_hours=draft.target_hours,
                actual_hours_so_far=draft.actual_hours_so_far,
                deviation_hours=draft.deviation_hours,
                would_create_streak=draft.would_create_streak,
                reasons=reasons,
            )
        )
    scored.sort(key=lambda c: c.score, reverse=True)
    return scored


def rank_replacement_candidates(
    req: ReplacementCandidatesRequest,
) -> ReplacementCandidatesResult:
    days = build_days(req.start_date, req.num_days)
    day_index = build_day_index(days)
    _validate_references(req, day_index)

    slot = req.target_slot
    base_target_hours_horizon = round(req.config.target_hours_per_week * req.num_days / 7)

    schedule_by_emp_date: ScheduleByEmpDate = {(e.employee_id, e.date): e for e in req.current_schedule}
    schedule_by_emp: dict[str, list[ScheduleEntry]] = defaultdict(list)
    for e in req.current_schedule:
        schedule_by_emp[e.employee_id].append(e)

    drafts: list[_CandidateDraft] = []
    excluded_count = 0
    for emp in req.employees:
        if not _is_eligible(req, emp, slot, day_index, schedule_by_emp_date):
            excluded_count += 1
            continue
        drafts.append(_draft_candidate(req, emp, slot, days, base_target_hours_horizon, schedule_by_emp))

    if not drafts:
        return ReplacementCandidatesResult(target_slot=slot, candidates=[], excluded_count=excluded_count)

    scored = _score_candidates(drafts, req.config.weights.streak_length, slot)
    return ReplacementCandidatesResult(
        target_slot=slot, candidates=scored[: req.top_n], excluded_count=excluded_count
    )
