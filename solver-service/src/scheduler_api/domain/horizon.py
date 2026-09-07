"""Shared helpers used by both solver.py and candidate_ranking.py, so the two agree on the same
date <-> index mapping and the same availability lookup semantics.
"""

from __future__ import annotations

from datetime import date, timedelta

from scheduler_api.schemas.common import AvailabilityMap, ShiftType


def build_days(start_date: date, num_days: int) -> list[date]:
    return [start_date + timedelta(days=i) for i in range(num_days)]


def build_day_index(days: list[date]) -> dict[date, int]:
    return {d: i for i, d in enumerate(days)}


def is_available(availability: AvailabilityMap, eid: str, day: date, shift: ShiftType) -> bool:
    day_avail = availability.get(eid, {}).get(day)
    if day_avail is None:
        return False
    return day_avail.sang if shift == "sang" else day_avail.dem
